package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// Uji lapisan penyedia pembayaran yang tidak menyentuh database.
//
// Yang TIDAK tercakup di sini: idempotensi tingkat baris pada MarkQRISChargePaid
// (dijaga klausa `WHERE status = 'pending'` + pemeriksaan RowsAffected) dan
// promosi pesanan online setelah lunas. Keduanya butuh Postgres sungguhan.

func hmacSign(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestNormalizeGatewayStatus(t *testing.T) {
	// Penyedia yang berbeda memakai istilah berbeda untuk hal yang sama.
	lunas := []string{"paid", "settlement", "success", "succeeded", "completed", "capture", "PAID", " Settlement "}
	for _, s := range lunas {
		if got := normalizeGatewayStatus(s); got != QRISPaid {
			t.Errorf("normalizeGatewayStatus(%q) = %q, mau %q", s, got, QRISPaid)
		}
	}

	gagal := []string{"deny", "cancelled", "failure", "failed"}
	for _, s := range gagal {
		if got := normalizeGatewayStatus(s); got != QRISFailed {
			t.Errorf("normalizeGatewayStatus(%q) = %q, mau %q", s, got, QRISFailed)
		}
	}

	if got := normalizeGatewayStatus("expired"); got != QRISExpired {
		t.Errorf("expired = %q, mau %q", got, QRISExpired)
	}

	// INTI KEAMANANNYA: status asing TIDAK BOLEH pernah ditebak sebagai lunas.
	// Kalau ini longgar, penyedia yang mengirim istilah tak dikenal bisa
	// membuat pesanan dianggap terbayar padahal uangnya belum masuk.
	asing := []string{"", "pending", "authorized", "on_hold", "settlement_pending", "lunas", "xyz"}
	for _, s := range asing {
		if got := normalizeGatewayStatus(s); got == QRISPaid {
			t.Errorf("normalizeGatewayStatus(%q) = %q — status asing tidak boleh dianggap lunas", s, got)
		}
	}
}

func TestMockVerifyWebhookMenolakTandaTanganPalsu(t *testing.T) {
	const secret = "rahasia-uji"
	t.Setenv("QRIS_WEBHOOK_SECRET", secret)

	body := []byte(`{"charge_id":"abc123","status":"settlement"}`)
	gw := mockGateway{}

	t.Run("tanda tangan sah diterima", func(t *testing.T) {
		id, status, err := gw.VerifyWebhook(body, map[string]string{
			"x-signature": hmacSign(t, secret, body),
		})
		if err != nil {
			t.Fatalf("callback sah ditolak: %v", err)
		}
		if id != "abc123" {
			t.Errorf("charge_id = %q, mau %q", id, "abc123")
		}
		if status != QRISPaid {
			t.Errorf("status = %q, mau %q", status, QRISPaid)
		}
	})

	t.Run("tanda tangan salah ditolak", func(t *testing.T) {
		if _, _, err := gw.VerifyWebhook(body, map[string]string{
			"x-signature": hmacSign(t, "kunci-lain", body),
		}); err == nil {
			t.Error("callback dengan kunci salah diterima — seharusnya ditolak")
		}
	})

	t.Run("body diubah setelah ditandatangani ditolak", func(t *testing.T) {
		// Skenario nyata: penyerang menaikkan nominal atau menukar charge_id
		// pada callback yang tanda tangannya dicuri dari kiriman lain.
		sig := hmacSign(t, secret, body)
		diubah := []byte(`{"charge_id":"pesanan-orang-lain","status":"settlement"}`)
		if _, _, err := gw.VerifyWebhook(diubah, map[string]string{
			"x-signature": sig,
		}); err == nil {
			t.Error("body yang diubah diterima — seharusnya ditolak")
		}
	})

	t.Run("tanpa tanda tangan ditolak", func(t *testing.T) {
		if _, _, err := gw.VerifyWebhook(body, map[string]string{}); err == nil {
			t.Error("callback tanpa tanda tangan diterima — seharusnya ditolak")
		}
	})

	t.Run("charge_id kosong ditolak", func(t *testing.T) {
		kosong := []byte(`{"status":"settlement"}`)
		if _, _, err := gw.VerifyWebhook(kosong, map[string]string{
			"x-signature": hmacSign(t, secret, kosong),
		}); err == nil {
			t.Error("callback tanpa charge_id diterima — seharusnya ditolak")
		}
	})
}

func TestMockVerifyWebhookTanpaSecretSelaluDitolak(t *testing.T) {
	// Deploy yang lupa mengisi QRIS_WEBHOOK_SECRET tidak boleh berubah menjadi
	// endpoint terbuka yang menerima callback dari siapa pun.
	t.Setenv("QRIS_WEBHOOK_SECRET", "")
	body := []byte(`{"charge_id":"abc","status":"paid"}`)
	if _, _, err := (mockGateway{}).VerifyWebhook(body, map[string]string{
		"x-signature": "apa-saja",
	}); err == nil {
		t.Error("callback diterima walau secret belum diset")
	}
}

func TestMarkQRISChargePaidMengabaikanStatusBelumFinal(t *testing.T) {
	// Status yang belum final harus keluar SEBELUM menyentuh database. Diuji
	// tanpa koneksi DB: kalau suatu saat penjagaan ini hilang, tes ini panik
	// karena database.DB masih nil — kegagalan yang keras, bukan senyap.
	if err := MarkQRISChargePaid("charge-apa-pun", QRISPending); err != nil {
		t.Errorf("status pending menghasilkan error: %v", err)
	}
	if err := MarkQRISChargePaid("charge-apa-pun", "status-asing"); err != nil {
		t.Errorf("status asing menghasilkan error: %v", err)
	}
}

func TestMockCreateQRISMenolakNominalTidakValid(t *testing.T) {
	gw := mockGateway{}
	for _, amount := range []float64{0, -1, -50000} {
		if _, err := gw.CreateQRIS(QRISCharge{ChargeID: "x", Amount: amount}); err == nil {
			t.Errorf("nominal %v diterima — seharusnya ditolak", amount)
		}
	}
}

func TestChargeIDUnikDanTidakKosong(t *testing.T) {
	// charge_id dipakai sebagai referensi order di sisi penyedia dan sebagai
	// primary key. Bentrokan berarti dua pesanan berbagi satu tagihan.
	seen := map[string]bool{}
	for i := 0; i < 5000; i++ {
		id := newChargeID()
		if id == "" {
			t.Fatal("charge id kosong")
		}
		if seen[id] {
			t.Fatalf("charge id bentrok pada iterasi %d: %s", i, id)
		}
		seen[id] = true
	}
}

// Simulation is opt-in and test cleanup restores the previous active provider.
func enableMockGateway(t *testing.T) {
	t.Helper()
	gatewayMu.RLock()
	previous := activeGatewayID
	gatewayMu.RUnlock()
	t.Cleanup(func() {
		gatewayMu.Lock()
		activeGatewayID = previous
		gatewayMu.Unlock()
	})
	t.Setenv("PAYMENT_GATEWAY_PROVIDER", "mock")
	t.Setenv("PAYMENT_GATEWAY_ALLOW_MOCK", "true")
	InitPaymentGateway()
}

func TestPaymentGatewayConfigurationFailsClosed(t *testing.T) {
	enableMockGateway(t)
	for _, tc := range []struct{ provider, allow string }{
		{"", "true"}, {"disabled", "true"}, {"unknown-provider", "true"}, {"mock", ""}, {"mock", "false"},
	} {
		t.Run(tc.provider+"/"+tc.allow, func(t *testing.T) {
			t.Setenv("PAYMENT_GATEWAY_PROVIDER", tc.provider)
			t.Setenv("PAYMENT_GATEWAY_ALLOW_MOCK", tc.allow)
			InitPaymentGateway()
			if _, err := ActiveGateway(); err == nil {
				t.Fatal("unconfigured QRIS must not activate a mock provider")
			}
			if MockSimulationEnabled() {
				t.Fatal("simulation unexpectedly enabled")
			}
		})
	}
	InitPaymentGateway()
	if !MockSimulationEnabled() {
		t.Fatal("explicit mock configuration must support simulation")
	}
}
