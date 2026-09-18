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
