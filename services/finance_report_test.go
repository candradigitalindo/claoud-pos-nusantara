package services

// Uji laporan keuangan atas alur pengadaan & aset: kelas belanja (belanja
// modal keluar dari HPP), akrual buku besar tanpa dobel beban untuk pembelian
// tempo/cicilan lintas periode, dan penyusutan periode yang cocok dengan
// rumus modul aset. Butuh Postgres; tanpa koneksi di-skip (lihat TestMain).

import (
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// scratchOutlet mengembalikan id outlet untuk pengujian: outlet pertama yang
// ada, atau outlet sementara yang dibuat khusus lalu dihapus lewat cleanup.
func scratchOutlet(t *testing.T) (string, func()) {
	t.Helper()
	var id string
	if err := database.DB.QueryRow(`SELECT id FROM outlets ORDER BY created_at LIMIT 1`).Scan(&id); err == nil && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), func() {}
	}
	id = NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, 'Outlet Uji', $3)`,
		id, "UJI-"+id[len(id)-6:], "key-"+id); err != nil {
		t.Fatalf("outlet uji: %v", err)
	}
	return id, func() { execScratch("DELETE FROM outlets WHERE id = $1", id) }
}

func glAccount(rep *models.GeneralLedgerResponse, code string) *models.GeneralLedgerAccount {
	for i := range rep.Accounts {
		if rep.Accounts[i].Code == code {
			return &rep.Accounts[i]
		}
	}
	return nil
}

// sumEntries menjumlahkan debit & kredit entri sebuah akun yang deskripsinya
// memuat penanda — supaya data lain di database tidak ikut terhitung.
func sumEntries(acc *models.GeneralLedgerAccount, marker string) (debit, credit float64) {
	if acc == nil {
		return 0, 0
	}
	for _, e := range acc.Entries {
		if strings.Contains(e.Description, marker) {
			debit += e.Debit
			credit += e.Credit
		}
	}
	return debit, credit
}

// Pembelian tempo/cicilan lintas periode tidak boleh mendebit beban dua kali:
// beban diakui SEKALI saat disetujui, pembayaran hanya mengurangi hutang.
func TestGeneralLedgerAccrualNoDoubleCount(t *testing.T) {
	requireDB(t)
	marker := "gl-uji-" + NewULID()[20:]
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: marker, VendorName: "CV Uji GL",
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis",
			Items: []models.PurchaseSubItem{{Name: "Servis genset", Qty: 1, Unit: "kali", HpsPrice: 100000, FinalPrice: 100000}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer execScratch("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	must := func(action string, amount float64) {
		t.Helper()
		if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: action, ActorName: "uji", PaymentAmount: amount}); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	must("approve", 0)
	must("request_payment", 0)
	must("pay", 40000)
	must("pay", 60000)

	// Pindahkan tanggal: disetujui & cicilan pertama di Januari, pelunasan Februari.
	execScratch(`UPDATE purchase_requests SET approved_at = '2026-01-15 03:00:00', created_at = '2026-01-15 03:00:00' WHERE id = $1`, pr.ID)
	execScratch(`UPDATE payment_histories SET created_at = '2026-01-20 03:00:00' WHERE purchase_request_id = $1 AND amount = 40000`, pr.ID)
	execScratch(`UPDATE payment_histories SET created_at = '2026-02-10 03:00:00' WHERE purchase_request_id = $1 AND amount = 60000`, pr.ID)

	jan, err := GetGeneralLedger("2026-01-01", "2026-01-31", "", "", nil)
	if err != nil {
		t.Fatalf("GL jan: %v", err)
	}
	feb, err := GetGeneralLedger("2026-02-01", "2026-02-28", "", "", nil)
	if err != nil {
		t.Fatalf("GL feb: %v", err)
	}

	if d, _ := sumEntries(glAccount(jan, "5-200"), marker); d != 100000 {
		t.Errorf("Jan beban jasa = %.0f, mau 100000 (diakui sekali saat disetujui)", d)
	}
	if d, c := sumEntries(glAccount(jan, "2-100"), marker); c != 100000 || d != 40000 {
		t.Errorf("Jan hutang: kredit=%.0f debit=%.0f, mau 100000/40000", c, d)
	}
	if _, c := sumEntries(glAccount(jan, "1-100"), marker); c != 40000 {
		t.Errorf("Jan kas keluar = %.0f, mau 40000", c)
	}
	if d, _ := sumEntries(glAccount(feb, "5-200"), marker); d != 0 {
		t.Errorf("Feb beban jasa = %.0f, mau 0 — pelunasan tidak boleh mendebit beban lagi", d)
	}
	if d, _ := sumEntries(glAccount(feb, "2-100"), marker); d != 60000 {
		t.Errorf("Feb hutang debit = %.0f, mau 60000", d)
	}
}

// Belanja peralatan (goods_kind perlengkapan) keluar dari HPP: tercatat sebagai
// Belanja Modal di Arus Kas dan Laba/Rugi, bukan sebagai beban.
func TestCapexLeavesCOGS(t *testing.T) {
	requireDB(t)
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	plBefore, err := GetProfitLossReport(today, today, "", nil)
	if err != nil {
		t.Fatalf("P&L sebelum: %v", err)
	}
	cfBefore, err := GetCashFlowReport(today, today, "", nil)
	if err != nil {
		t.Fatalf("arus kas sebelum: %v", err)
	}

	pr := newReceivableGoodsPR(t, 50000) // 2 × 50.000, nama di luar katalog → perlengkapan
	defer cleanupReceivablePR(pr.ID)
	if k, _ := ClassifyPurchaseItems(pr.Items); k != "perlengkapan" {
		t.Fatalf("goods_kind = %q, mau perlengkapan", k)
	}
	for _, action := range []string{"request_payment", "pay"} {
		if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: action, ActorName: "uji"}); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}

	plAfter, _ := GetProfitLossReport(today, today, "", nil)
	cfAfter, _ := GetCashFlowReport(today, today, "", nil)
	if d := plAfter.Summary.COGS - plBefore.Summary.COGS; d != 0 {
		t.Errorf("HPP bertambah %.0f oleh belanja peralatan — seharusnya 0", d)
	}
	if d := plAfter.Summary.CapexPayments - plBefore.Summary.CapexPayments; d != 100000 {
		t.Errorf("belanja modal P&L bertambah %.0f, mau 100000", d)
	}
	if d := cfAfter.Summary.CapexPayments - cfBefore.Summary.CapexPayments; d != 100000 {
		t.Errorf("belanja modal arus kas bertambah %.0f, mau 100000", d)
	}
	if d := cfAfter.Summary.NetCashFlow - cfBefore.Summary.NetCashFlow; d != -100000 {
		t.Errorf("arus kas bersih berubah %.0f, mau -100000 (uangnya tetap keluar)", d)
	}
}

// Penyusutan periode = akumulasi(akhir) − akumulasi(awal − 1 hari), rumus
// garis lurus modul aset: 1.200.000 / 12 bulan = 100.000 per bulan.
func TestDepreciationForPeriod(t *testing.T) {
	requireDB(t)
	outletID, cleanupOutlet := scratchOutlet(t)
	defer cleanupOutlet()

	marBefore, _, _ := depreciationForPeriod("2026-03-01", "2026-03-31", []string{outletID})
	yearBefore, _, _ := depreciationForPeriod("2026-01-01", "2026-12-31", []string{outletID})
	balBefore, err := GetBalanceReport("2026-03-01", "2026-03-31", "", nil)
	if err != nil {
		t.Fatalf("neraca sebelum: %v", err)
	}

	id := NewULID()
	if _, err := database.DB.Exec(`
		INSERT INTO assets (id, outlet_id, name, quantity, unit, purchase_date, purchase_price,
			useful_life_months, residual_value, is_deleted, status)
		VALUES ($1, $2, 'Uji Penyusutan', 1, 'unit', '2026-01-15', 1200000, 12, 0, false, 'aktif')`,
		id, outletID); err != nil {
		t.Fatalf("insert aset: %v", err)
	}
	defer execScratch("DELETE FROM assets WHERE id = $1", id)

	mar, _, err := depreciationForPeriod("2026-03-01", "2026-03-31", []string{outletID})
	if err != nil {
		t.Fatalf("penyusutan maret: %v", err)
	}
	if d := mar - marBefore; d != 100000 {
		t.Errorf("penyusutan Maret = %.0f, mau 100000", d)
	}
	year, _, _ := depreciationForPeriod("2026-01-01", "2026-12-31", []string{outletID})
	if d := year - yearBefore; d != 1100000 {
		t.Errorf("penyusutan 2026 = %.0f, mau 1100000 (11 bulan penuh sejak 15 Jan)", d)
	}
	// Setelah umur habis tidak ada penyusutan lagi.
	if over, _, _ := depreciationForPeriod("2028-01-01", "2028-12-31", []string{outletID}); over != 0 {
		t.Errorf("penyusutan 2028 = %.0f, mau 0 — umur ekonomis sudah habis", over)
	}
	// Neraca per 31 Mar 2026: nilai buku = 1.200.000 − 2 × 100.000.
	balAfter, err := GetBalanceReport("2026-03-01", "2026-03-31", "", nil)
	if err != nil {
		t.Fatalf("neraca sesudah: %v", err)
	}
	if d := balAfter.FixedAssets - balBefore.FixedAssets; d != 1000000 {
		t.Errorf("aset tetap Neraca bertambah %.0f, mau 1000000", d)
	}
	if d := balAfter.TotalAssets - balBefore.TotalAssets; d != 1000000 {
		t.Errorf("total aset Neraca bertambah %.0f, mau 1000000", d)
	}
}

// Ekspor Excel keempat laporan harus tetap jalan setelah kolom-kolom baru
// ditambahkan — kesalahan huruf kolom tidak ketahuan oleh kompilator.
func TestFinanceExcelExportsRun(t *testing.T) {
	requireDB(t)
	from, to := "2026-01-01", "2026-03-31"
	for name, fn := range map[string]func() ([]byte, string, error){
		"arus kas":   func() ([]byte, string, error) { return BuildCashFlowReportExcel(from, to, "", nil) },
		"laba rugi":  func() ([]byte, string, error) { return BuildProfitLossReportExcel(from, to, "", nil) },
		"neraca":     func() ([]byte, string, error) { return BuildBalanceReportExcel(from, to, "", nil) },
		"buku besar": func() ([]byte, string, error) { return BuildGeneralLedgerExcel(from, to, "", "", nil) },
	} {
		b, fname, err := fn()
		if err != nil || len(b) == 0 || fname == "" {
			t.Errorf("ekspor %s: err=%v bytes=%d nama=%q", name, err, len(b), fname)
		}
	}
}
