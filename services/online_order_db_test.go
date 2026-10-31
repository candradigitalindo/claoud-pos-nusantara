package services

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Uji jalur pesanan online yang kebenarannya dijamin SQL, bukan kode Go:
// klaim yang atomik, webhook yang idempoten, dan promosi pesanan setelah lunas.
// Ketiganya tidak bisa dibuktikan tanpa database sungguhan — menjalankannya di
// SQLite akan menguji query yang berbeda dari yang dipakai produksi.
//
// Jalankan dengan Postgres tersedia:
//
//	DB_NAME=cloud_pos_test go test ./services/ -run TestDB
//
// Tanpa itu seluruh berkas ini di-skip, bukan gagal — supaya `go test ./...`
// tetap hijau di mesin yang tidak memasang Postgres.

var (
	onlineDBOnce  sync.Once
	onlineDBError error
)

// Use the existing suite connection guarded by DB_NAME ending in _test.
// Never replace database.DB with a connection from a second environment.
func setupDB(t *testing.T) {
	t.Helper()
	requireDB(t)
	onlineDBOnce.Do(func() { onlineDBError = database.RunMigrations() })
	if onlineDBError != nil {
		t.Fatalf("online order test migrations: %v", onlineDBError)
	}
}

// seedOutlet membuat outlet uji yang bersih beserta shift terbuka, satu menu,
// dan satu biaya pajak — bentuk minimum agar pesanan online bisa dibuat.
func produkID(outletID string) string { return "PRD" + outletID[len(outletID)-10:] }

func seedOutlet(t *testing.T) (outletID, slug string) {
	t.Helper()
	outletID = NewULID()
	suffix := outletID[len(outletID)-10:]
	slug = "uji-" + suffix

	_, err := database.DB.Exec(
		`INSERT INTO outlets (id, name, code, slug, api_key, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())`,
		outletID, "Outlet Uji", "UJI"+suffix, slug, "key-"+outletID,
	)
	if err != nil {
		t.Fatalf("seed outlet: %v", err)
	}

	// Shift terbuka — tanpa ini CreatePublicOrder menolak pesanan.
	shiftID := NewULID()
	if _, err := database.DB.Exec(
		`INSERT INTO cloud_cashier_shifts (id, local_id, outlet_id, opened_by, opened_at, opening_cash, status)
		VALUES ($1, $2, $3, 'Kasir Uji', NOW(), 0, 'open')`,
		shiftID, shiftID, outletID,
	); err != nil {
		t.Fatalf("seed shift: %v", err)
	}

	if _, err := database.DB.Exec(
		`INSERT INTO cloud_products (id, local_id, outlet_id, name, price, updated_at)
		VALUES ($1, $2, $3, 'Nasi Goreng', 25000, NOW())`,
		"PRD"+suffix, "PRD"+suffix, outletID,
	); err != nil {
		t.Fatalf("seed produk: %v", err)
	}

	chargeID := NewULID()
	if _, err := database.DB.Exec(
		`INSERT INTO cloud_additional_charges (id, local_id, outlet_id, name, charge_type, value, is_active)
		VALUES ($1, $2, $3, 'Pajak Restoran (PB1)', 'percentage', 10, true)`,
		chargeID, chargeID, outletID,
	); err != nil {
		t.Fatalf("seed biaya: %v", err)
	}

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM online_orders WHERE outlet_id = $1`, outletID)
		database.DB.Exec(`DELETE FROM qris_charges WHERE outlet_id = $1`, outletID)
		database.DB.Exec(`DELETE FROM cloud_additional_charges WHERE outlet_id = $1`, outletID)
		database.DB.Exec(`DELETE FROM cloud_products WHERE outlet_id = $1`, outletID)
		database.DB.Exec(`DELETE FROM cloud_cashier_shifts WHERE outlet_id = $1`, outletID)
		database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outletID)
	})
	return outletID, slug
}

func pesanSatuNasiGoreng(t *testing.T, outletID, slug string) *models.OnlineOrder {
	t.Helper()
	enableMockGateway(t)
	order, err := CreatePublicOrder(slug, models.PublicOrderRequest{
		TableNumber: "A5",
		Items: []models.OnlineOrderItem{
			{ProductLocalID: produkID(outletID), Qty: 2},
		},
	})
	if err != nil {
		t.Fatalf("buat pesanan: %v", err)
	}
	return order
}

func statusPesanan(t *testing.T, id string) (status string, paid float64) {
	t.Helper()
	if err := database.DB.QueryRow(
		`SELECT status, paid_amount FROM online_orders WHERE id = $1`, id,
	).Scan(&status, &paid); err != nil {
		t.Fatalf("baca status: %v", err)
	}
	return
}

func TestDBTotalPesananIkutBiayaOutlet(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)

	// 2 × 25.000 = 50.000, PB1 10% = 5.000 → 55.000.
	if order.Subtotal != 50000 {
		t.Errorf("subtotal = %v, mau 50000", order.Subtotal)
	}
	if order.TotalAmount != 55000 {
		t.Errorf("total = %v, mau 55000", order.TotalAmount)
	}
	// Nominal QRIS HARUS sama dengan total tagihan — inilah inti "QRIS dinamis".
	if order.Payment == nil || order.Payment.Amount != order.TotalAmount {
		t.Fatalf("nominal QRIS tidak sama dengan tagihan: %+v", order.Payment)
	}
}

func TestDBPesananBelumDibayarTidakTerlihatPOS(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)

	status, _ := statusPesanan(t, order.ID)
	if status != models.OnlineOrderAwaitingPayment {
		t.Errorf("status awal = %q, mau %q", status, models.OnlineOrderAwaitingPayment)
	}

	// Dapur tidak boleh memasak pesanan yang belum dibayar.
	pending, err := PendingOnlineOrders(outletID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("POS melihat %d pesanan yang belum dibayar", len(pending))
	}
}

func TestDBPesananMunculSetelahLunas(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
		t.Fatal(err)
	}

	status, paid := statusPesanan(t, order.ID)
	if status != models.OnlineOrderNew {
		t.Errorf("status = %q, mau %q", status, models.OnlineOrderNew)
	}
	// Nominal yang dicatat adalah yang benar-benar diterima penyedia.
	if paid != 55000 {
		t.Errorf("paid_amount = %v, mau 55000", paid)
	}

	pending, err := PendingOnlineOrders(outletID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("POS melihat %d pesanan, mau 1", len(pending))
	}
	if pending[0].PaidAmount != 55000 || pending[0].TotalAmount != 55000 {
		t.Errorf("nominal yang dikirim ke POS salah: %+v", pending[0])
	}
}

func TestDBWebhookGandaTidakMelunaskanDuaKali(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)

	// Penyedia mengulang kiriman callback — hal yang lumrah terjadi.
	for i := 0; i < 5; i++ {
		if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
			t.Fatalf("callback ke-%d: %v", i+1, err)
		}
	}

	var n int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM qris_charges WHERE id = $1 AND status = 'paid'`,
		order.Payment.ChargeID,
	).Scan(&n)
	if n != 1 {
		t.Errorf("baris tagihan lunas = %d, mau 1", n)
	}

	_, paid := statusPesanan(t, order.ID)
	if paid != 55000 {
		t.Errorf("paid_amount = %v setelah 5 callback, mau tetap 55000", paid)
	}
}

func TestDBCallbackGagalTerlambatTidakMembatalkanYangSudahLunas(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
		t.Fatal(err)
	}
	// Callback "expired" yang datang terlambat TIDAK boleh menurunkan status
	// tagihan yang uangnya sudah masuk.
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISExpired); err != nil {
		t.Fatal(err)
	}

	var status string
	database.DB.QueryRow(
		`SELECT status FROM qris_charges WHERE id = $1`, order.Payment.ChargeID,
	).Scan(&status)
	if status != QRISPaid {
		t.Errorf("status tagihan = %q setelah callback terlambat, mau %q", status, QRISPaid)
	}
}

func TestDBKlaimAtomikHanyaSatuPerangkatMenang(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
		t.Fatal(err)
	}

	// Sepuluh perangkat menarik pesanan yang sama pada saat bersamaan.
	// Kalau lebih dari satu menang, tamu mendapat dua order meja dan dua
	// tiket dapur untuk satu pesanan.
	const perangkat = 10
	var wg sync.WaitGroup
	menang := make([]bool, perangkat)
	for i := 0; i < perangkat; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := ClaimOnlineOrder(outletID, order.ID, fmt.Sprintf("device-%d", idx))
			if err == nil && ok {
				menang[idx] = true
			}
		}(i)
	}
	wg.Wait()

	total := 0
	for _, m := range menang {
		if m {
			total++
		}
	}
	if total != 1 {
		t.Errorf("%d perangkat memenangkan klaim, mau tepat 1", total)
	}
}

func TestDBKlaimUlangPerangkatSamaDiizinkan(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
		t.Fatal(err)
	}

	// Percobaan ulang setelah jaringan putus tidak boleh buntu.
	for i := 0; i < 3; i++ {
		ok, err := ClaimOnlineOrder(outletID, order.ID, "device-A")
		if err != nil || !ok {
			t.Fatalf("klaim ulang ke-%d ditolak (ok=%v err=%v)", i+1, ok, err)
		}
	}
	// Perangkat lain tetap tidak boleh menyerobot klaim yang masih segar.
	if ok, _ := ClaimOnlineOrder(outletID, order.ID, "device-B"); ok {
		t.Error("perangkat lain berhasil menyerobot klaim yang masih berlaku")
	}
}

func TestDBKonfirmasiMenutupSiklus(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	order := pesanSatuNasiGoreng(t, outletID, slug)
	if err := MarkQRISChargePaid(order.Payment.ChargeID, QRISPaid); err != nil {
		t.Fatal(err)
	}
	if ok, err := ClaimOnlineOrder(outletID, order.ID, "device-A"); err != nil || !ok {
		t.Fatalf("klaim gagal: ok=%v err=%v", ok, err)
	}
	if err := ConfirmOnlineOrder(outletID, order.ID, "ORDERLOKAL1"); err != nil {
		t.Fatal(err)
	}

	status, _ := statusPesanan(t, order.ID)
	if status != models.OnlineOrderConfirmed {
		t.Errorf("status = %q, mau %q", status, models.OnlineOrderConfirmed)
	}

	// Pesanan yang sudah jadi order meja TIDAK boleh ditarik lagi — kalau
	// tidak, tamu mendapat pesanan kedua yang tidak pernah ia pesan.
	pending, err := PendingOnlineOrders(outletID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("pesanan terkonfirmasi masih ditawarkan ke POS: %d", len(pending))
	}
}

func TestDBTagihanPendingDipakaiUlangBukanDitagihDuaKali(t *testing.T) {
	setupDB(t)
	outletID, _ := seedOutlet(t)
	enableMockGateway(t)

	a, err := CreateQRISCharge(outletID, "ORDER1", 55000, "Meja A5")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateQRISCharge(outletID, "ORDER1", 55000, "Meja A5")
	if err != nil {
		t.Fatal(err)
	}
	if a.ChargeID != b.ChargeID {
		t.Errorf("terbit dua tagihan untuk order & nominal sama: %s vs %s", a.ChargeID, b.ChargeID)
	}

	// Nominal berubah (tamu menambah item) → QR baru, bukan yang lama.
	c, err := CreateQRISCharge(outletID, "ORDER1", 70000, "Meja A5")
	if err != nil {
		t.Fatal(err)
	}
	if c.ChargeID == a.ChargeID {
		t.Error("nominal berubah tapi QR lama dipakai ulang")
	}
}

func TestDBPesananDitolakSaatShiftTutup(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)

	// Tutup shift-nya: restoran tidak sedang beroperasi.
	if _, err := database.DB.Exec(
		`UPDATE cloud_cashier_shifts SET closed_at = NOW(), status = 'closed' WHERE outlet_id = $1`,
		outletID,
	); err != nil {
		t.Fatal(err)
	}

	enableMockGateway(t)
	_, err := CreatePublicOrder(slug, models.PublicOrderRequest{
		TableNumber: "A5",
		Items: []models.OnlineOrderItem{
			{ProductLocalID: produkID(outletID), Qty: 1},
		},
	})
	if err == nil {
		t.Fatal("pesanan diterima walau tidak ada shift terbuka")
	}

	// Tidak boleh ada tagihan maupun pesanan yang tertinggal menggantung.
	var n int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM online_orders WHERE outlet_id = $1`, outletID).Scan(&n)
	if n != 0 {
		t.Errorf("%d pesanan tertinggal padahal ditolak", n)
	}
}

func TestDBHargaKirimanTamuDiabaikan(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)
	enableMockGateway(t)

	// Perangkat tamu mengaku menunya bernama lain dan berharga 1 rupiah.
	order, err := CreatePublicOrder(slug, models.PublicOrderRequest{
		TableNumber: "A5",
		Items: []models.OnlineOrderItem{{
			ProductLocalID: produkID(outletID),
			ProductName:    "Gratisan",
			Qty:            2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.TotalAmount != 55000 {
		t.Errorf("total = %v — harga kiriman tamu terpakai, mau 55000", order.TotalAmount)
	}
	if order.Items[0].ProductName != "Nasi Goreng" {
		t.Errorf("nama menu = %q, mau dibaca dari master", order.Items[0].ProductName)
	}
}

func TestDBAddonAsingDitolak(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)
	enableMockGateway(t)

	// Add-on yang tidak terdaftar pada menu itu tidak boleh diterima —
	// kalau lolos, tamu bisa menyisipkan tambahan berharga negatif.
	_, err := CreatePublicOrder(slug, models.PublicOrderRequest{
		TableNumber: "A5",
		Items: []models.OnlineOrderItem{{
			ProductLocalID: produkID(outletID),
			Qty:            1,
			Addons:         []models.OnlineOrderAddon{{ID: "ADDON-PALSU", Price: -99999}},
		}},
	})
	if err == nil {
		t.Fatal("add-on asing diterima")
	}
}

func TestDBMenuLuarOutletDitolak(t *testing.T) {
	setupDB(t)
	_, slug := seedOutlet(t)
	outletLain, _ := seedOutlet(t)
	enableMockGateway(t)

	// Menu milik outlet lain tidak boleh bisa dipesan lewat slug outlet ini.
	_, err := CreatePublicOrder(slug, models.PublicOrderRequest{
		TableNumber: "A5",
		Items: []models.OnlineOrderItem{
			{ProductLocalID: produkID(outletLain), Qty: 1},
		},
	})
	if err == nil {
		t.Fatal("menu milik outlet lain diterima")
	}
}

var _ = sql.ErrNoRows // menjaga import tetap terpakai bila tes dipangkas
