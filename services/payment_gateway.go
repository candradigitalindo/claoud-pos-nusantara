package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// ── Kontrak penyedia pembayaran ──────────────────────────────────────────────
//
// Seluruh alur QRIS di sistem ini (tagihan, polling status, webhook, pencatatan
// pembayaran) tidak tahu-menahu soal penyedia tertentu. Menambah Midtrans atau
// Xendit cukup dengan mengimplementasikan PaymentGateway lalu mendaftarkannya
// di RegisterGateway — tidak ada satu pun handler atau tabel yang perlu berubah.

// QRISCharge adalah permintaan tagihan QRIS yang dikirim ke penyedia.
type QRISCharge struct {
	// ChargeID adalah id milik kita, dipakai sebagai referensi order di sisi
	// penyedia agar webhook bisa dipetakan balik tanpa menyimpan peta terpisah.
	ChargeID    string
	OutletID    string
	Amount      float64
	Description string
}

// QRISResult adalah jawaban penyedia atas permintaan tagihan.
type QRISResult struct {
	// ProviderRef = id transaksi di sisi penyedia (untuk rekonsiliasi manual).
	ProviderRef string
	// QRString = payload QRIS mentah; POS yang menggambar QR-nya sendiri
	// sehingga tidak bergantung pada URL gambar milik penyedia.
	QRString  string
	ExpiresAt time.Time
}

// Status tagihan. Sengaja sedikit: yang dibutuhkan kasir hanyalah "sudah
// dibayar atau belum", sisanya menghentikan polling.
const (
	QRISPending = "pending"
	QRISPaid    = "paid"
	QRISExpired = "expired"
	QRISFailed  = "failed"
)

// PaymentGateway adalah antarmuka yang harus dipenuhi setiap penyedia QRIS.
type PaymentGateway interface {
	// Name mengembalikan pengenal penyedia, ikut disimpan di baris tagihan.
	Name() string

	// CreateQRIS membuat tagihan dan mengembalikan payload QR-nya.
	CreateQRIS(charge QRISCharge) (*QRISResult, error)

	// CheckStatus menanyakan status terkini ke penyedia. Dipakai sebagai
	// jaring pengaman bila webhook tidak pernah datang.
	CheckStatus(providerRef string) (string, error)

	// VerifyWebhook memastikan callback benar-benar berasal dari penyedia,
	// lalu mengembalikan (chargeID, status). Error = tolak callback.
	VerifyWebhook(body []byte, headers map[string]string) (string, string, error)
}

var (
	gatewayMu       sync.RWMutex
	gateways        = map[string]PaymentGateway{}
	activeGatewayID string
)

// RegisterGateway mendaftarkan implementasi penyedia. Dipanggil dari init()
// masing-masing adapter.
func RegisterGateway(g PaymentGateway) {
	gatewayMu.Lock()
	defer gatewayMu.Unlock()
	gateways[g.Name()] = g
}

// InitPaymentGateway memilih penyedia aktif dari env PAYMENT_GATEWAY_PROVIDER.
// An unset/unknown provider disables QRIS. Simulation additionally requires an
// explicit PAYMENT_GATEWAY_ALLOW_MOCK=true; it never collects real payments.
func InitPaymentGateway() {
	want := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_GATEWAY_PROVIDER")))
	gatewayMu.Lock()
	defer gatewayMu.Unlock()
	activeGatewayID = ""
	if want == "" || want == "disabled" {
		return
	}
	if want == "mock" && !strings.EqualFold(strings.TrimSpace(os.Getenv("PAYMENT_GATEWAY_ALLOW_MOCK")), "true") {
		log.Printf("QRIS simulation disabled: PAYMENT_GATEWAY_ALLOW_MOCK must be true")
		return
	}
	if _, ok := gateways[want]; !ok {
		log.Printf("Payment gateway %q tidak terdaftar — QRIS dinonaktifkan", want)
		return
	}
	activeGatewayID = want
	log.Printf("Payment gateway aktif: %s", want)
}

// MockSimulationEnabled reports the deliberate simulation configuration.
func MockSimulationEnabled() bool {
	gatewayMu.RLock()
	defer gatewayMu.RUnlock()
	return activeGatewayID == "mock" && strings.EqualFold(strings.TrimSpace(os.Getenv("PAYMENT_GATEWAY_ALLOW_MOCK")), "true")
}

// ActiveGateway mengembalikan penyedia yang sedang dipakai.
func ActiveGateway() (PaymentGateway, error) {
	gatewayMu.RLock()
	defer gatewayMu.RUnlock()
	g, ok := gateways[activeGatewayID]
	if !ok {
		return nil, fmt.Errorf("payment gateway belum dikonfigurasi")
	}
	return g, nil
}

// GatewayByName dipakai endpoint webhook: callback harus diverifikasi oleh
// adapter penyedia yang mengirimnya, bukan oleh penyedia yang sedang aktif
// (tagihan lama dari penyedia sebelumnya masih boleh diselesaikan).
func GatewayByName(name string) (PaymentGateway, error) {
	gatewayMu.RLock()
	defer gatewayMu.RUnlock()
	g, ok := gateways[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("penyedia %q tidak dikenal", name)
	}
	return g, nil
}

// ── Adapter tiruan ───────────────────────────────────────────────────────────

// mockGateway memenuhi seluruh kontrak tanpa memanggil jaringan, sehingga alur
// kasir bisa dijalankan end-to-end sebelum akun penyedia asli tersedia.
// Tagihan TIDAK pernah berpindah status sendiri: pembayaran disimulasikan lewat
// endpoint simulate-paid atau webhook, persis seperti penyedia asli.
type mockGateway struct{}

func (mockGateway) Name() string { return "mock" }

func (mockGateway) CreateQRIS(charge QRISCharge) (*QRISResult, error) {
	if charge.Amount <= 0 {
		return nil, fmt.Errorf("nominal tagihan harus lebih dari 0")
	}
	// Payload berformat mirip QRIS asli agar pemindai di POS tetap bisa
	// menggambarnya, tetapi jelas bertanda MOCK supaya tidak pernah tertukar
	// dengan kode produksi.
	qr := fmt.Sprintf("00020101021226MOCK-QRIS-%s5204000053033605802ID5405%.0f6304",
		charge.ChargeID, charge.Amount)
	return &QRISResult{
		ProviderRef: "mock_" + charge.ChargeID,
		QRString:    qr,
		ExpiresAt:   time.Now().UTC().Add(15 * time.Minute),
	}, nil
}

func (mockGateway) CheckStatus(providerRef string) (string, error) {
	// Status sebenarnya dipegang tabel qris_charges; adapter tiruan tidak
	// menyimpan apa pun, jadi ia tidak pernah mendahului keadaan lokal.
	return QRISPending, nil
}

func (mockGateway) VerifyWebhook(body []byte, headers map[string]string) (string, string, error) {
	// Tanda tangan HMAC-SHA256 atas body, kunci = QRIS_WEBHOOK_SECRET.
	// Bentuknya sengaja sama dengan yang dipakai penyedia asli agar penanganan
	// webhook tidak perlu berubah saat adapter ditukar.
	secret := os.Getenv("QRIS_WEBHOOK_SECRET")
	if secret == "" {
		return "", "", fmt.Errorf("QRIS_WEBHOOK_SECRET belum diset")
	}
	got := headers["x-signature"]
	if got == "" {
		return "", "", fmt.Errorf("tanda tangan tidak ada")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(got)), []byte(want)) {
		return "", "", fmt.Errorf("tanda tangan tidak cocok")
	}

	var payload struct {
		ChargeID string `json:"charge_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("payload tidak valid: %w", err)
	}
	if payload.ChargeID == "" {
		return "", "", fmt.Errorf("charge_id tidak ada di payload")
	}
	return payload.ChargeID, normalizeGatewayStatus(payload.Status), nil
}

func init() { RegisterGateway(mockGateway{}) }

// normalizeGatewayStatus memetakan beragam istilah penyedia ke status internal.
// Apa pun yang tidak dikenali dianggap masih menunggu — jangan pernah menebak
// "lunas" dari status asing.
func normalizeGatewayStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "paid", "settlement", "success", "succeeded", "completed", "capture":
		return QRISPaid
	case "expire", "expired":
		return QRISExpired
	case "deny", "denied", "cancel", "cancelled", "failure", "failed":
		return QRISFailed
	default:
		return QRISPending
	}
}

// newChargeID membuat pengenal tagihan acak 26 karakter hex-ish yang aman
// dipakai sebagai referensi order di sisi penyedia.
func newChargeID() string {
	b := make([]byte, 13)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%026d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(b)
}
