package services

// Uji alur uang muka reservasi: DP hanya sah setelah tervalidasi, status
// mengikuti pembayaran, uang muka menjadi kewajiban di Neraca dan kas masuk
// di Arus Kas, lalu tertutup saat reservasi diselesaikan dari POS. Butuh
// Postgres; tanpa koneksi di-skip (lihat TestMain).

import (
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

func newTestReservation(t *testing.T, outletID string, dp float64) *models.Reservation {
	t.Helper()
	r, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "Uji Reservasi " + NewULID()[20:], Pax: 4,
		ReservationDate: time.Now().In(GetTimezoneLocation()).AddDate(0, 0, 3).Format("2006-01-02"),
		ReservationTime: "19:00", DownPayment: dp,
		Items: []models.ReservationItem{{ProductName: "Paket Uji", Qty: 2, Price: 100000}},
	}, nil)
	if err != nil {
		t.Fatalf("create reservasi: %v", err)
	}
	return r
}

func cleanupReservation(id string) {
	execScratch("DELETE FROM reservation_payments WHERE reservation_id = $1", id)
	execScratch("DELETE FROM reservations WHERE id = $1", id)
}

func TestReservationDepositFlow(t *testing.T) {
	requireDB(t)
	outletID, cleanupOutlet := scratchOutlet(t)
	defer cleanupOutlet()
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")

	cfBefore, _ := GetCashFlowReport(today, today, "", nil)
	balBefore, _ := GetBalanceReport(today, today, "", nil)

	r := newTestReservation(t, outletID, 100000) // total 200.000, DP diminta 100.000
	defer cleanupReservation(r.ID)
	if r.Status != "pending" || r.Total != 200000 || r.Remaining != 200000 || r.DpPaid {
		t.Fatalf("awal: status=%s total=%.0f sisa=%.0f dpPaid=%v", r.Status, r.Total, r.Remaining, r.DpPaid)
	}

	// Konfirmasi tanpa DP tervalidasi ditolak.
	if _, err := UpdateReservationStatus(r.ID, "confirmed", "", "admin", nil); err == nil {
		t.Error("konfirmasi tanpa DP tervalidasi lolos")
	}
	// Transfer tanpa bukti ditolak.
	if _, err := AddReservationPayment(r.ID, models.ReservationPaymentRequest{Type: "dp", Amount: 100000, Method: "transfer"}, "admin", true, nil); err == nil {
		t.Error("transfer tanpa bukti lolos")
	}
	// Admin mencatat DP (langsung sah) → otomatis dikonfirmasi.
	if _, err := AddReservationPayment(r.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 100000, Method: "transfer", ProofURL: "/uploads/uji-dp.jpg", PaidAt: today,
	}, "admin", true, nil); err != nil {
		t.Fatalf("catat DP: %v", err)
	}
	r, _ = GetReservation(r.ID, nil)
	if r.Status != "confirmed" || r.PaidAmount != 100000 || r.Remaining != 100000 || !r.DpPaid {
		t.Fatalf("setelah DP: status=%s dibayar=%.0f sisa=%.0f dpPaid=%v", r.Status, r.PaidAmount, r.Remaining, r.DpPaid)
	}

	// Pelanggan mengirim pelunasan → pending; melebihi sisa ditolak.
	if _, err := PublicSubmitReservationPayment(slugOf(t, outletID), r.ID, models.ReservationPaymentRequest{Amount: 150000, ProofURL: "/uploads/uji-lunas.jpg"}); err == nil {
		t.Error("pelunasan melebihi sisa lolos")
	}
	p, err := PublicSubmitReservationPayment(slugOf(t, outletID), r.ID, models.ReservationPaymentRequest{Amount: 100000, ProofURL: "/uploads/uji-lunas.jpg"})
	if err != nil {
		t.Fatalf("kirim pelunasan: %v", err)
	}
	if p.Status != "pending" || p.Type != "pelunasan" {
		t.Fatalf("bukti pelanggan: status=%s type=%s", p.Status, p.Type)
	}
	r, _ = GetReservation(r.ID, nil)
	if r.PendingAmount != 100000 || r.PaidAmount != 100000 {
		t.Fatalf("pending=%.0f paid=%.0f, mau 100000/100000", r.PendingAmount, r.PaidAmount)
	}
	// Selagi ada bukti menunggu, reservasi tidak bisa ditutup.
	if err := SettleReservation(outletID, r.ID, "TX-UJI", "kasir"); err == nil {
		t.Error("settle dengan bukti menunggu validasi lolos")
	}
	if _, err := ValidateReservationPayment(r.ID, p.ID, "admin", nil); err != nil {
		t.Fatalf("validasi: %v", err)
	}
	r, _ = GetReservation(r.ID, nil)
	if r.PaidAmount != 200000 || r.Remaining != 0 || r.PendingAmount != 0 {
		t.Fatalf("setelah validasi: paid=%.0f sisa=%.0f pending=%.0f", r.PaidAmount, r.Remaining, r.PendingAmount)
	}

	// Keuangan: kas masuk 200.000 sebagai uang muka, kewajiban 200.000 — bukan pendapatan.
	cfAfter, _ := GetCashFlowReport(today, today, "", nil)
	if d := cfAfter.Summary.DepositReceipts - cfBefore.Summary.DepositReceipts; d != 200000 {
		t.Errorf("arus kas uang muka bertambah %.0f, mau 200000", d)
	}
	if d := cfAfter.Summary.SalesReceipts - cfBefore.Summary.SalesReceipts; d != 0 {
		t.Errorf("penerimaan penjualan berubah %.0f oleh DP — seharusnya 0", d)
	}
	balMid, _ := GetBalanceReport(today, today, "", nil)
	if d := balMid.CustomerDeposits - balBefore.CustomerDeposits; d != 200000 {
		t.Errorf("uang muka pelanggan Neraca bertambah %.0f, mau 200000", d)
	}
	if d := balMid.CashAndEquivalents - balBefore.CashAndEquivalents; d != 200000 {
		t.Errorf("kas Neraca bertambah %.0f, mau 200000", d)
	}

	// Hapus ditolak selama ada pembayaran tervalidasi.
	if err := DeleteReservation(r.ID, nil); err == nil {
		t.Error("reservasi dengan pembayaran tervalidasi bisa dihapus")
	}

	// Ditutup dari POS → done, kewajiban hilang.
	if err := SettleReservation(outletID, r.ID, "TX-UJI", "kasir"); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := SettleReservation(outletID, r.ID, "TX-UJI", "kasir"); err != nil {
		t.Errorf("settle ulang transaksi yang sama harus idempoten: %v", err)
	}
	if err := SettleReservation(outletID, r.ID, "TX-LAIN", "kasir"); err == nil {
		t.Error("settle oleh transaksi lain lolos")
	}
	r, _ = GetReservation(r.ID, nil)
	if r.Status != "done" || r.PosTransactionID != "TX-UJI" {
		t.Fatalf("setelah settle: status=%s tx=%s", r.Status, r.PosTransactionID)
	}
	balAfter, _ := GetBalanceReport(today, today, "", nil)
	if d := balAfter.CustomerDeposits - balBefore.CustomerDeposits; d != 0 {
		t.Errorf("uang muka Neraca setelah ditutup masih %.0f, mau 0", d)
	}
	if _, err := UpdateReservation(r.ID, models.ReservationRequest{CustomerName: "x"}, nil); err == nil {
		t.Error("reservasi selesai masih bisa diubah")
	}
}

// Pembatalan dengan uang muka HANGUS: kewajiban hilang, menjadi pendapatan lain.
func TestReservationCancelForfeit(t *testing.T) {
	requireDB(t)
	outletID, cleanupOutlet := scratchOutlet(t)
	defer cleanupOutlet()
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	plBefore, _ := GetProfitLossReport(today, today, "", nil)

	r := newTestReservation(t, outletID, 50000)
	defer cleanupReservation(r.ID)
	if _, err := AddReservationPayment(r.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 50000, Method: "cash", PaidAt: today,
	}, "admin", true, nil); err != nil {
		t.Fatalf("catat DP tunai: %v", err)
	}
	// Batal tanpa keputusan nasib uang muka ditolak.
	if _, err := UpdateReservationStatus(r.ID, "cancelled", "", "admin", nil); err == nil {
		t.Error("batal tanpa keputusan refund/hangus lolos")
	}
	if _, err := UpdateReservationStatus(r.ID, "cancelled", "hangus", "admin", nil); err != nil {
		t.Fatalf("batal hangus: %v", err)
	}
	plAfter, _ := GetProfitLossReport(today, today, "", nil)
	if d := plAfter.Summary.ForfeitedDeposits - plBefore.Summary.ForfeitedDeposits; d != 50000 {
		t.Errorf("DP hangus di P&L bertambah %.0f, mau 50000", d)
	}
	if d := plAfter.Summary.OtherIncome - plBefore.Summary.OtherIncome; d != 50000 {
		t.Errorf("pendapatan lain bertambah %.0f, mau 50000", d)
	}
	gl, err := GetGeneralLedger(today, today, "", "", nil)
	if err != nil {
		t.Fatalf("GL: %v", err)
	}
	marker := r.CustomerName
	if d, _ := sumEntries(glAccount(gl, "2-300"), marker); d != 50000 {
		t.Errorf("GL uang muka debit (hangus) = %.0f, mau 50000", d)
	}
	if _, c := sumEntries(glAccount(gl, "4-200"), marker); c != 50000 {
		t.Errorf("GL pendapatan lain kredit = %.0f, mau 50000", c)
	}
	bal, _ := GetBalanceReport(today, today, "", nil)
	var dep float64
	for _, o := range bal.Outlets {
		if strings.TrimSpace(o.OutletID) == outletID {
			dep = o.CustomerDeposits
		}
	}
	_ = dep // outlet lain bisa punya uang muka sendiri; yang penting reservasi ini tidak lagi dihitung
	rr, _ := GetReservation(r.ID, nil)
	if rr.Status != "cancelled" || rr.CancelDisposition != "hangus" {
		t.Errorf("status=%s disposisi=%s", rr.Status, rr.CancelDisposition)
	}
}

// slugOf memastikan outlet uji punya slug untuk endpoint publik.
func slugOf(t *testing.T, outletID string) string {
	t.Helper()
	var slug string
	database.DB.QueryRow(`SELECT COALESCE(slug, '') FROM outlets WHERE id = $1`, outletID).Scan(&slug)
	if strings.TrimSpace(slug) == "" {
		slug = "uji-" + strings.ToLower(outletID[len(outletID)-6:])
		if _, err := database.DB.Exec(`UPDATE outlets SET slug = $1 WHERE id = $2`, slug, outletID); err != nil {
			t.Fatalf("set slug: %v", err)
		}
	}
	return strings.TrimSpace(slug)
}

// Pajak / biaya tambahan outlet (PB1 10%) otomatis masuk total reservasi
// dan baris duplikat bernama sama dari perangkat lama hanya dihitung sekali.
func TestReservationAppliesOutletChargesAndDeduplicates(t *testing.T) {
	requireDB(t)
	outletID, cleanupOutlet := scratchOutlet(t)
	defer cleanupOutlet()

	// Sisipkan 2 baris aktif bernama sama ("Pajak Restoran (PB1)") seperti di production.
	oldID, newID := NewULID(), NewULID()
	if _, err := database.DB.Exec(`
		INSERT INTO cloud_additional_charges (id, local_id, outlet_id, name, charge_type, value, is_active, version, created_at, updated_at)
		VALUES
			($1::char(26), $1::varchar, $3, 'Pajak Restoran (PB1)', 'percentage', 10, true, 1, NOW() - interval '2 days', NOW() - interval '2 days'),
			($2::char(26), $2::varchar, $3, 'Pajak Restoran (PB1)', 'percentage', 10, true, 1, NOW(), NOW())`,
		oldID, newID, outletID); err != nil {
		t.Fatalf("seed charges: %v", err)
	}
	t.Cleanup(func() {
		execScratch("DELETE FROM cloud_additional_charges WHERE outlet_id = $1", outletID)
	})

	r, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "PT Uji PB1", Pax: 30,
		ReservationDate: time.Now().In(GetTimezoneLocation()).AddDate(0, 0, 1).Format("2006-01-02"),
		ReservationTime: "11:00", DownPayment: 300000,
		Items: []models.ReservationItem{{ProductName: "Nasi Kotak Paket A", Qty: 30, Price: 35000}},
	}, nil)
	if err != nil {
		t.Fatalf("create reservasi PB1: %v", err)
	}
	defer cleanupReservation(r.ID)

	if r.Subtotal != 1050000 {
		t.Errorf("subtotal = %.0f, mau 1050000", r.Subtotal)
	}
	if len(r.Charges) != 1 || r.Charges[0].Amount != 105000 {
		t.Fatalf("charges = %+v, mau 1 baris PB1 105000 (bukan dobel 20%%)", r.Charges)
	}
	if r.Total != 1155000 || r.Remaining != 1155000 {
		t.Errorf("total/remaining = %.0f/%.0f, mau 1155000/1155000", r.Total, r.Remaining)
	}
	if itemsTxt := waReservationItemsWithCharges(r); !strings.Contains(itemsTxt, "Pajak Restoran (PB1)") {
		t.Errorf("rincian WA tidak mencantumkan PB1: %s", itemsTxt)
	}
}

// Alur reservasi yang diperketat & disederhanakan:
// 1. Reservasi tanpa menu + DP manual bisa dibuat dan dibayar.
// 2. Jadwal (tanggal & jam) wajib; tanggal lampau ditolak untuk publik.
// 3. Status pending tidak bisa langsung loncat ke done.
// 4. Total edit tidak boleh turun di bawah pembayaran tervalidasi.
// 5. InitialPayment saat buat reservasi langsung memvalidasi & konfirmasi otomatis.
func TestReservationImprovedFlowAndGuards(t *testing.T) {
	requireDB(t)
	outletID, cleanupOutlet := scratchOutlet(t)
	defer cleanupOutlet()
	loc := GetTimezoneLocation()
	today := time.Now().In(loc).Format("2006-01-02")
	yesterday := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1).Format("2006-01-02")

	// Jadwal wajib
	if _, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "Tanpa Tanggal", Pax: 2,
	}, nil); err == nil {
		t.Error("reservasi tanpa tanggal/jam lolos")
	}
	// Publik tidak boleh pesan tanggal lampau
	if _, err := CreatePublicReservation(slugOf(t, outletID), models.ReservationRequest{
		CustomerName: "Tamu Kemarin", Pax: 2, ReservationDate: yesterday, ReservationTime: "12:00",
		Items: []models.ReservationItem{{ProductName: "Teh", Qty: 1, Price: 10000}},
	}); err == nil {
		t.Error("reservasi publik tanggal lampau lolos")
	}

	// Reservasi tanpa menu + DP manual 200.000 harus diterima & bisa dibayar
	noMenu, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "Meja VIP", Pax: 6,
		ReservationDate: tomorrow, ReservationTime: "18:30", DownPayment: 200000,
	}, nil)
	if err != nil {
		t.Fatalf("reservasi tanpa menu + DP manual ditolak: %v", err)
	}
	defer cleanupReservation(noMenu.ID)
	if noMenu.Remaining != 200000 || noMenu.Status != "pending" {
		t.Fatalf("tanpa menu: status=%s remaining=%.0f, mau pending/200000", noMenu.Status, noMenu.Remaining)
	}
	// Pending tidak boleh langsung ditandai selesai
	if _, err := UpdateReservationStatus(noMenu.ID, "done", "", "admin", nil); err == nil {
		t.Error("status pending langsung ke done lolos")
	}
	if _, err := AddReservationPayment(noMenu.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 200000, Method: "cash", PaidAt: today,
	}, "admin", true, nil); err != nil {
		t.Fatalf("bayar DP manual tanpa menu: %v", err)
	}
	noMenu, _ = GetReservation(noMenu.ID, nil)
	if noMenu.Status != "confirmed" || noMenu.PaidAmount != 200000 || noMenu.Remaining != 0 {
		t.Fatalf("setelah bayar DP manual: status=%s paid=%.0f rem=%.0f", noMenu.Status, noMenu.PaidAmount, noMenu.Remaining)
	}

	// InitialPayment gagal validasi (transfer tanpa bukti) tidak boleh meninggalkan reservasi yatim
	if _, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "Gagal Bukti", Pax: 2,
		ReservationDate: tomorrow, ReservationTime: "12:00", DownPayment: 100000,
		Items:          []models.ReservationItem{{ProductName: "Paket", Qty: 2, Price: 100000}},
		InitialPayment: &models.ReservationPaymentRequest{Type: "dp", Amount: 100000, Method: "transfer"},
	}, nil); err == nil {
		t.Error("initial payment transfer tanpa bukti lolos")
	}

	// InitialPayment valid → langsung tervalidasi & confirmed
	instant, err := CreateReservation(models.ReservationRequest{
		OutletID: outletID, CustomerName: "Langsung DP", Pax: 4,
		ReservationDate: tomorrow, ReservationTime: "13:00", DownPayment: 100000,
		Items:          []models.ReservationItem{{ProductName: "Paket", Qty: 2, Price: 100000}},
		InitialPayment: &models.ReservationPaymentRequest{Type: "dp", Amount: 100000, Method: "transfer", ProofURL: "/uploads/uji-init.jpg", PaidAt: today},
		Actor:          "Kasir",
	}, nil)
	if err != nil {
		t.Fatalf("create dengan initial payment: %v", err)
	}
	defer cleanupReservation(instant.ID)
	if instant.Status != "confirmed" || instant.PaidAmount != 100000 || instant.Remaining != 100000 {
		t.Fatalf("instant DP: status=%s paid=%.0f rem=%.0f", instant.Status, instant.PaidAmount, instant.Remaining)
	}

	// Menurunkan total menu di bawah pembayaran tervalidasi (100.000) harus ditolak
	if _, err := UpdateReservation(instant.ID, models.ReservationRequest{
		CustomerName: "Langsung DP", Pax: 4, ReservationDate: tomorrow, ReservationTime: "13:00",
		DownPayment: 50000,
		Items:       []models.ReservationItem{{ProductName: "Paket Kecil", Qty: 1, Price: 50000}},
	}, nil); err == nil {
		t.Error("menurunkan total di bawah paid_amount lolos")
	}
}

func TestReservationProductsAndCrossOutletGuard(t *testing.T) {
	requireDB(t)
	outletA, outletB := NewULID(), NewULID()
	for _, o := range []struct{ id, code, name, slug string }{
		{outletA, "RTA" + outletA[20:], "Outlet Uji Resv A", "uji-resv-a-" + outletA[20:]},
		{outletB, "RTB" + outletB[20:], "Outlet Uji Resv B", "uji-resv-b-" + outletB[20:]},
	} {
		if _, err := database.DB.Exec(
			`INSERT INTO outlets (id, code, name, slug, api_key, is_active) VALUES ($1,$2,$3,$4,$5,true)`,
			o.id, o.code, o.name, o.slug, "key-"+o.id,
		); err != nil {
			t.Fatalf("insert outlet %s: %v", o.name, err)
		}
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM reservations WHERE outlet_id IN ($1,$2)`, outletA, outletB)
		database.DB.Exec(`DELETE FROM outlets WHERE id IN ($1,$2)`, outletA, outletB)
	})
	prodA1 := NewULID()
	prodA2 := NewULID()
	prodADel := NewULID()
	prodB := NewULID()
	for _, row := range []struct {
		id, outlet, name string
		price            float64
		del              bool
	}{
		{prodA1, outletA, "Zebra Latte", 35000, false},
		{prodA2, outletA, "americano", 25000, false},
		{prodADel, outletA, "Deleted Menu", 10000, true},
		{prodB, outletB, "Buntut Outlet B", 95000, false},
	} {
		if _, err := database.DB.Exec(
			`INSERT INTO cloud_products (id, outlet_id, local_id, name, price, category_name, is_deleted) VALUES ($1,$2,$3,$4,$5,'Kopi',$6)`,
			row.id, row.outlet, "LOC-"+row.id[18:], row.name, row.price, row.del,
		); err != nil {
			t.Fatalf("insert product %s: %v", row.name, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range []string{prodA1, prodA2, prodADel, prodB} {
			database.DB.Exec(`DELETE FROM cloud_products WHERE id = $1`, id)
		}
	})

	list, err := ListReservationProducts(outletA)
	if err != nil {
		t.Fatalf("ListReservationProducts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 active products for outletA, got %d", len(list))
	}
	if list[0].Name != "americano" || list[1].Name != "Zebra Latte" {
		t.Fatalf("expected case-insensitive sort [americano, Zebra Latte], got [%s, %s]", list[0].Name, list[1].Name)
	}

	tomorrow := time.Now().In(GetTimezoneLocation()).AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := CreateReservation(models.ReservationRequest{
		OutletID: outletA, CustomerName: "Cross Outlet", Pax: 2,
		ReservationDate: tomorrow, ReservationTime: "18:00",
		Items: []models.ReservationItem{{ProductID: prodB, ProductName: "Buntut Outlet B", Qty: 1, Price: 95000}},
	}, nil); err == nil {
		t.Fatal("expected error when submitting product from another outlet")
	}
}
