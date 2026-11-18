package services

import (
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Bayar pengadaan dari Kas Kasir Outlet: satu uang keluar → satu pembayaran
// pengajuan → satu catatan kas di pembukuan.

func payoutTestOutlet(t *testing.T) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		id, "KO-"+id[len(id)-6:], "Outlet Kas "+id[len(id)-4:], "key-"+id); err != nil {
		t.Fatalf("outlet uji: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM outlets WHERE id = $1", id) })
	return id
}

// payablePR: pengajuan jasa milik outlet, sudah ditagihkan (payment_requested).
func payablePR(t *testing.T, outletID string, price float64) string {
	t.Helper()
	pr, err := createPricedPR(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: "tester", VendorName: "Vendor Uji Bersama", OutletID: outletID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis mesin kopi",
			Items: []models.PurchaseSubItem{{Name: "Jasa servis", Qty: 1, Unit: "kali", HpsPrice: price, FinalPrice: price}},
		}},
	})
	if err != nil {
		t.Fatalf("buat pengajuan: %v", err)
	}
	verifyAndApprove(t, pr.ID)
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Fatalf("ajukan pembayaran: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_cash_movements WHERE cash_payout_id IN (SELECT id FROM procurement_cash_payouts WHERE purchase_request_id=$1)`, pr.ID)
		execScratch(`DELETE FROM payment_histories WHERE purchase_request_id = $1`, pr.ID)
		execScratch(`DELETE FROM procurement_cash_payouts WHERE purchase_request_id = $1`, pr.ID)
		execScratch(`DELETE FROM purchase_requests WHERE id = $1`, pr.ID)
	})
	return pr.ID
}

func movementID() string { return "MV" + NewULID() }

func device(movement string) models.CashPayoutDeviceInput {
	return models.CashPayoutDeviceInput{MovementLocalID: movement, CashierName: "Kasir Uji", AuthorizedBy: "Manajer Uji"}
}

func TestOutletCashPayoutFullFlowPaysOnceAndBooksOnce(t *testing.T) {
	requireDB(t)
	outlet := payoutTestOutlet(t)
	other := payoutTestOutlet(t)
	prID := payablePR(t, outlet, 250000)

	p, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"})
	if err != nil {
		t.Fatalf("tetapkan kas outlet: %v", err)
	}
	if p.Status != "pending" || p.Amount != 250000 || p.RecipientName != "Vendor Uji Bersama" {
		t.Fatalf("tugas: %+v", p)
	}
	// Selama tugas terbuka, Kantor Pusat tidak bisa membayar pengajuan yang sama.
	if _, err := UpdatePurchaseStatus(prID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "Keuangan"}); err == nil ||
		!strings.Contains(err.Error(), "Kas Kasir Outlet") {
		t.Fatalf("bayar transfer harus ditolak selama tugas terbuka, got %v", err)
	}
	if _, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"}); err == nil {
		t.Fatal("tugas kedua untuk pengajuan yang sama harus ditolak")
	}
	// Hanya tablet outlet pemilik yang melihat & bisa memproses tugas.
	if list, _ := ListOutletCashPayouts(other); len(list) != 0 {
		t.Fatalf("outlet lain tidak boleh melihat tugas: %+v", list)
	}
	if list, _ := ListOutletCashPayouts(outlet); len(list) != 1 || list[0].ItemsSummary != "Servis mesin kopi" {
		t.Fatalf("daftar tablet: %+v", list)
	}
	m1 := movementID()
	if _, err := ClaimCashPayout(other, p.ID, device(m1)); err == nil {
		t.Fatal("outlet lain tidak boleh mengklaim")
	}
	if _, err := ClaimCashPayout(outlet, p.ID, models.CashPayoutDeviceInput{MovementLocalID: m1, CashierName: "Kasir Uji"}); err == nil {
		t.Fatal("klaim tanpa PIN manajer harus ditolak")
	}
	claimed, err := ClaimCashPayout(outlet, p.ID, device(m1))
	if err != nil || claimed.Status != "claimed" {
		t.Fatalf("klaim: %+v %v", claimed, err)
	}
	if again, err := ClaimCashPayout(outlet, p.ID, device(m1)); err != nil || again.Status != "claimed" {
		t.Fatalf("klaim ulang (balasan hilang) harus idempoten: %v", err)
	}
	if _, err := ClaimCashPayout(outlet, p.ID, device(movementID())); err == nil {
		t.Fatal("klaim dengan kas keluar lain harus ditolak")
	}
	if _, err := CancelCashPayout(p.ID, models.CashPayoutResolveInput{ActorName: "Keuangan"}); err == nil {
		t.Fatal("tugas yang sedang diproses kasir tidak boleh ditarik")
	}

	// Kas keluar tablet sampai lewat sinkron → tugas tuntas (jalur tahan-putus).
	paidAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{
		LocalID: m1, MovementType: "out", Amount: 250000, CounterpartName: "Vendor Uji Bersama",
		Note: "Bayar pengadaan", CreatedAt: paidAt, CashPayoutID: p.ID,
	}); err != nil {
		t.Fatalf("sinkron kas keluar: %v", err)
	}
	done, err := GetCashPayout(p.ID)
	if err != nil || done.Status != "paid" {
		t.Fatalf("tugas harus lunas setelah kas keluar tersinkron: %+v %v", done, err)
	}
	// Tablet menyelesaikan secara online setelahnya: idempoten, tidak dobel.
	if again, err := CompleteCashPayout(outlet, p.ID, m1); err != nil || again.Status != "paid" {
		t.Fatalf("selesai ulang harus idempoten: %v", err)
	}
	pr, _ := GetPurchaseRequest(prID)
	if pr.Status != "paid" || pr.PaidAmount != 250000 {
		t.Fatalf("pengajuan: status=%s paid=%v", pr.Status, pr.PaidAmount)
	}
	var payments int
	var method, payoutRef string
	database.DB.QueryRow(`SELECT COUNT(*), MAX(payment_method), MAX(COALESCE(cash_payout_id,'')) FROM payment_histories WHERE purchase_request_id=$1`, prID).
		Scan(&payments, &method, &payoutRef)
	if payments != 1 || method != "outlet_cash" || strings.TrimSpace(payoutRef) != p.ID {
		t.Fatalf("pembayaran: n=%d metode=%s tugas=%s", payments, method, payoutRef)
	}
	var linked string
	database.DB.QueryRow(`SELECT COALESCE(cash_payout_id,'') FROM cloud_cash_movements WHERE outlet_id=$1 AND local_id=$2`, outlet, m1).Scan(&linked)
	if strings.TrimSpace(linked) != p.ID {
		t.Fatalf("kas keluar harus tertaut ke tugas, got %q", linked)
	}

	// Pembukuan: uang keluar tercatat SEKALI (sebagai pelunasan hutang).
	day := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	ops, err := loadFinanceJournal(day, day, outlet, nil)
	if err != nil {
		t.Fatalf("jurnal: %v", err)
	}
	var cashOut float64
	posCash := 0
	for _, op := range ops {
		cashOut += op.CashOut
		if op.Source == "pos_cash" {
			posCash++
		}
	}
	if posCash != 0 || cashOut != 250000 {
		t.Fatalf("kas keluar dobel di pembukuan: pos_cash=%d kas keluar=%v", posCash, cashOut)
	}
	if err := ClassifyFinanceCash(mustMovementCloudID(t, outlet, m1), "5-300", "uji", FinanceAccess{Actor: "Keuangan", Permissions: map[string]bool{"finance.entries.post": true}}); err == nil {
		t.Fatal("penyerahan kas pengadaan tidak boleh diklasifikasi ulang manual")
	}
}

func mustMovementCloudID(t *testing.T, outlet, local string) string {
	t.Helper()
	var id string
	if err := database.DB.QueryRow(`SELECT id FROM cloud_cash_movements WHERE outlet_id=$1 AND local_id=$2`, outlet, local).Scan(&id); err != nil {
		t.Fatalf("kas keluar: %v", err)
	}
	return id
}

func TestOutletCashPayoutGuardsAndReturns(t *testing.T) {
	requireDB(t)
	outlet := payoutTestOutlet(t)

	// Pengajuan tanpa outlet hanya bisa dibayar Kantor Pusat.
	noOutlet := payablePR(t, "", 50000)
	if _, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: noOutlet, ActorName: "Keuangan"}); err == nil {
		t.Fatal("pengajuan tanpa outlet tidak boleh memakai kas outlet")
	}

	prID := payablePR(t, outlet, 100000)
	if _, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, Amount: 150000, ActorName: "Keuangan"}); err == nil {
		t.Fatal("nominal melebihi sisa tagihan harus ditolak")
	}
	p, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, Amount: 40000, RecipientName: "Budi (staf)", ActorName: "Keuangan"})
	if err != nil {
		t.Fatalf("tetapkan sebagian: %v", err)
	}
	if _, err := ReturnCashPayout(outlet, p.ID, models.CashPayoutDeviceInput{CashierName: "Kasir"}); err == nil {
		t.Fatal("pengembalian tanpa alasan harus ditolak")
	}
	ret, err := ReturnCashPayout(outlet, p.ID, models.CashPayoutDeviceInput{CashierName: "Kasir", Reason: "Harga di toko Rp45.000"})
	if err != nil || ret.Status != "returned" || ret.ResolutionReason != "Harga di toko Rp45.000" {
		t.Fatalf("kembalikan: %+v %v", ret, err)
	}
	if pr, _ := GetPurchaseRequest(prID); pr.CashPayout == nil || pr.CashPayout.Status != "returned" {
		t.Fatalf("halaman pembayaran harus melihat tugas yang dikembalikan: %+v", pr.CashPayout)
	}

	// Tetapkan ulang → klaim → kas keluar gagal tercatat → lepas → tarik.
	p2, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, Amount: 45000, ActorName: "Keuangan"})
	if err != nil {
		t.Fatalf("tetapkan ulang: %v", err)
	}
	m := movementID()
	if _, err := ClaimCashPayout(outlet, p2.ID, device(m)); err != nil {
		t.Fatalf("klaim: %v", err)
	}
	if rel, err := ReleaseCashPayout(outlet, p2.ID, m); err != nil || rel.Status != "pending" {
		t.Fatalf("lepas klaim: %+v %v", rel, err)
	}
	// Kas keluar yang mengaku tugas tanpa klaim tidak ditautkan & tidak membayar.
	stray := movementID()
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: stray, MovementType: "out", Amount: 45000, Note: "x", CashPayoutID: p2.ID}); err != nil {
		t.Fatalf("sinkron: %v", err)
	}
	var linked string
	database.DB.QueryRow(`SELECT COALESCE(cash_payout_id,'') FROM cloud_cash_movements WHERE outlet_id=$1 AND local_id=$2`, outlet, stray).Scan(&linked)
	if strings.TrimSpace(linked) != "" {
		t.Fatal("kas keluar tanpa klaim tidak boleh tertaut ke tugas")
	}
	execScratch(`DELETE FROM cloud_cash_movements WHERE outlet_id=$1 AND local_id=$2`, outlet, stray)
	if cur, _ := GetCashPayout(p2.ID); cur.Status != "pending" {
		t.Fatalf("tugas tidak boleh berubah oleh kas keluar liar: %s", cur.Status)
	}
	// Klaim lagi, kas keluar tercatat → tidak bisa dilepas lagi.
	m2 := movementID()
	if _, err := ClaimCashPayout(outlet, p2.ID, device(m2)); err != nil {
		t.Fatalf("klaim ulang: %v", err)
	}
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: m2, MovementType: "out", Amount: 45000, Note: "Bayar", CashPayoutID: p2.ID}); err != nil {
		t.Fatalf("sinkron kas keluar: %v", err)
	}
	if _, err := ReleaseCashPayout(outlet, p2.ID, m2); err == nil {
		t.Fatal("klaim yang kas keluarnya sudah tercatat tidak boleh dilepas")
	}
	pr, _ := GetPurchaseRequest(prID)
	if pr.Status != "partial" || pr.PaidAmount != 45000 || pr.CashPayout != nil {
		t.Fatalf("pembayaran sebagian: status=%s paid=%v payout=%+v", pr.Status, pr.PaidAmount, pr.CashPayout)
	}

	// Tugas berikutnya yang belum diproses bisa ditarik keuangan.
	p3, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"})
	if err != nil || p3.Amount != 55000 {
		t.Fatalf("tetapkan sisa: %+v %v", p3, err)
	}
	if c, err := CancelCashPayout(p3.ID, models.CashPayoutResolveInput{ActorName: "Keuangan", Reason: "Dibayar transfer"}); err != nil || c.Status != "cancelled" {
		t.Fatalf("tarik: %+v %v", c, err)
	}
	if _, err := ClaimCashPayout(outlet, p3.ID, device(movementID())); err == nil {
		t.Fatal("tugas yang ditarik tidak boleh diklaim")
	}
	// Setelah ditarik, Kantor Pusat boleh membayar sisanya.
	if _, err := UpdatePurchaseStatus(prID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "Keuangan"}); err != nil {
		t.Fatalf("bayar transfer setelah tugas ditarik: %v", err)
	}
}
