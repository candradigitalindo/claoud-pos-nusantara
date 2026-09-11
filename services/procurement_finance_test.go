package services

// Uji sinkronisasi angka pengadaan antara halaman Pembayaran (keuangan) dan
// laporan keuangan. Tiap tes di sini mengunci satu ketidakcocokan nyata yang
// pernah ada:
//
//   - cicilan seluruhnya jatuh di tanggal pembayaran TERAKHIR, sehingga kas
//     bulan sebelumnya kosong dan laporan tidak cocok dengan mutasi rekening;
//   - "hutang usaha" punya tiga rumus berbeda (Pembayaran / Neraca / Buku
//     Besar) sehingga tidak ada dua halaman yang menampilkan angka sama;
//   - pembayaran atas sisa item pada master yang sudah dipecah sebagian
//     dibuang seluruhnya oleh filter "split_status IS NULL".
//
// TestMain + requireDB ada di purchase_flow_test.go (paket yang sama).

import (
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// payDay memundurkan tanggal sebuah baris payment_histories supaya bisa
// menguji cicilan lintas hari tanpa menunggu waktu nyata.
func backdatePayment(t *testing.T, prID string, amount float64, to time.Time) {
	t.Helper()
	res, err := database.DB.Exec(`
		UPDATE payment_histories SET created_at = $1
		WHERE id = (SELECT id FROM payment_histories
		            WHERE purchase_request_id = $2 AND amount = $3
		            ORDER BY created_at LIMIT 1)`, to, prID, amount)
	if err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("backdate: %d baris terpengaruh, mau 1", n)
	}
}

func approvedPR(t *testing.T, total float64) *models.PurchaseRequest {
	t.Helper()
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Semen", Qty: 1, Unit: "sak", HpsPrice: total, FinalPrice: total}},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	for _, act := range []string{"approve", "request_payment"} {
		if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: act, ActorName: "tester"}); err != nil {
			t.Fatalf("%s: %v", act, err)
		}
	}
	return pr
}

func cashOutOn(t *testing.T, day string) float64 {
	t.Helper()
	rep, err := GetCashFlowReport(day, day, "", nil)
	if err != nil {
		t.Fatalf("cash flow %s: %v", day, err)
	}
	var sum float64
	for _, r := range rep.Daily {
		sum += r.COGSPayments + r.ServicePayments
	}
	return sum
}

// Cicilan harus jatuh pada tanggal uangnya keluar, bukan menumpuk di tanggal
// cicilan terakhir.
func TestCashFlowSplitsInstallmentsByPaymentDate(t *testing.T) {
	requireDB(t)
	pr := approvedPR(t, 1000000)
	defer database.DB.Exec("DELETE FROM payment_histories WHERE purchase_request_id = $1", pr.ID)
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	for _, amt := range []float64{400000, 600000} {
		if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
			Action: "pay", ActorName: "keuangan", PaymentAmount: amt,
		}); err != nil {
			t.Fatalf("bayar %v: %v", amt, err)
		}
	}

	// Cicilan pertama dipindah ke 40 hari lalu (beda bulan), kedua ke 5 hari lalu.
	now := time.Now().UTC()
	first, second := now.AddDate(0, 0, -40), now.AddDate(0, 0, -5)
	backdatePayment(t, pr.ID, 400000, first)
	backdatePayment(t, pr.ID, 600000, second)

	dayFirst := first.Format("2006-01-02")
	daySecond := second.Format("2006-01-02")

	if got := cashOutOn(t, dayFirst); got != 400000 {
		t.Errorf("kas keluar %s = %v, mau 400000 (cicilan pertama)", dayFirst, got)
	}
	if got := cashOutOn(t, daySecond); got != 600000 {
		t.Errorf("kas keluar %s = %v, mau 600000 — bukan 1000000; seluruh tagihan tidak boleh jatuh di tanggal cicilan terakhir", daySecond, got)
	}
}

// Hutang usaha di Neraca harus sama persis dengan yang dilihat keuangan di
// halaman Pembayaran.
func TestAccountsPayableMatchesPaymentsPage(t *testing.T) {
	requireDB(t)

	base, err := GetPaymentStats(nil, nil)
	if err != nil {
		t.Fatalf("payment stats awal: %v", err)
	}

	// Tagihan yang sudah di meja keuangan tapi belum dibayar sepeser pun.
	// Status 'payment_requested' inilah yang dulu hilang dari Neraca.
	waiting := approvedPR(t, 750000)
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", waiting.ID)

	// Tagihan yang baru dicicil sebagian.
	partial := approvedPR(t, 500000)
	defer database.DB.Exec("DELETE FROM payment_histories WHERE purchase_request_id = $1", partial.ID)
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", partial.ID)
	if _, err := UpdatePurchaseStatus(partial.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 200000,
	}); err != nil {
		t.Fatalf("bayar sebagian: %v", err)
	}

	stats, err := GetPaymentStats(nil, nil)
	if err != nil {
		t.Fatalf("payment stats: %v", err)
	}
	added := stats.AccountsPayable.TotalAmount - base.AccountsPayable.TotalAmount
	if added != 1050000 {
		t.Fatalf("hutang bertambah %v di halaman Pembayaran, mau 1050000 (750000 + sisa 300000)", added)
	}

	// Neraca memakai rentang tanggal, tapi hutang usaha adalah saldo berjalan.
	today := time.Now().UTC().Format("2006-01-02")
	bal, err := GetBalanceReport(today, today, "", nil)
	if err != nil {
		t.Fatalf("neraca: %v", err)
	}
	if bal.AccountsPayable != stats.AccountsPayable.TotalAmount {
		t.Errorf("hutang usaha Neraca = %v, halaman Pembayaran = %v — kedua halaman wajib menampilkan angka yang sama",
			bal.AccountsPayable, stats.AccountsPayable.TotalAmount)
	}
}

// Master yang dipecah SEBAGIAN masih memegang sisa item dan boleh dibayar.
// Pembayaran itu dulu dibuang laporan oleh filter "split_status IS NULL".
func TestPartiallySplitMasterPaymentReachesCashFlow(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name: "Material",
			Items: []models.PurchaseSubItem{
				{Name: "Semen", Qty: 1, Unit: "sak", HpsPrice: 300000, FinalPrice: 300000},
				{Name: "Pasir", Qty: 1, Unit: "truk", HpsPrice: 200000, FinalPrice: 200000},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM payment_histories WHERE purchase_request_id IN (SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)", pr.ID)
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "tester"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// Pasir pindah ke vendor lain; Semen 300.000 tetap di master.
	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Pasir",
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Pasir", Qty: 1, HpsPrice: 200000, FinalPrice: 200000}},
		}},
	}); err != nil {
		t.Fatalf("split: %v", err)
	}

	before := cashOutOn(t, time.Now().UTC().Format("2006-01-02"))

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err != nil {
		t.Fatalf("request_payment: %v", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 300000,
	}); err != nil {
		t.Fatalf("bayar master: %v", err)
	}

	after := cashOutOn(t, time.Now().UTC().Format("2006-01-02"))
	if after-before != 300000 {
		t.Errorf("kas keluar bertambah %v, mau 300000 — pembayaran atas sisa item master tidak boleh hilang dari laporan", after-before)
	}
}
