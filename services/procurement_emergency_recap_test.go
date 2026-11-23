package services

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

// Rekap Pembelian Darurat untuk audit Keuangan: tahap tiap dokumen, selisih
// kas − struk, dan Kas Keluar tablet di luar tugas Bayar Pengadaan.
func TestEmergencyRecapStagesDifferencesAndOtherCash(t *testing.T) {
	requireDB(t)
	outlet, unit := emergencyTestUnit(t)
	now := time.Now().UTC().Format(time.RFC3339)
	mk := func(price float64) *models.PurchaseRequest {
		pr, err := CreateEmergencyPurchase(emergencyInput(unit, price))
		if err != nil {
			t.Fatalf("buat %v: %v", price, err)
		}
		return pr
	}
	// handOver: klaim + kas keluar tersinkron (tertaut tugas) + selesai.
	handOver := func(prID string) {
		task := openPayoutFor(t, outlet, prID)
		m := movementID()
		if _, err := ClaimCashPayout(outlet, task.ID, device(m)); err != nil {
			t.Fatalf("klaim: %v", err)
		}
		if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: m, MovementType: "out", Amount: task.Amount,
			CounterpartName: task.RecipientName, Note: "Bayar Pengadaan", CreatedAt: now, CashPayoutID: task.ID}); err != nil {
			t.Fatalf("kas keluar tertaut: %v", err)
		}
		if _, err := CompleteCashPayout(outlet, task.ID, m); err != nil {
			t.Fatalf("selesai: %v", err)
		}
	}
	proof := func(prID string, total float64) {
		if _, err := SubmitEmergencyProof(prID, models.EmergencyProofInput{ProofURL: "/uploads/struk-rekap.jpg", ProofTotal: total, ActorName: "Manajer Uji"}); err != nil {
			t.Fatalf("struk: %v", err)
		}
	}

	waitingCash := mk(100000)
	ratified := mk(200000)
	handOver(ratified.ID)
	proof(ratified.ID, 180000)
	if _, err := DecideEmergencyPurchase(ratified.ID, models.EmergencyDecisionInput{Decision: "ratify", ActorName: "Owner Uji"}); err != nil {
		t.Fatalf("sahkan: %v", err)
	}
	waitingProof := mk(150000)
	handOver(waitingProof.ID)
	rejected := mk(120000)
	handOver(rejected.ID)
	proof(rejected.ID, 120000)
	if _, err := DecideEmergencyPurchase(rejected.ID, models.EmergencyDecisionInput{Decision: "reject", Notes: "harga terlalu mahal", ActorName: "Owner Uji"}); err != nil {
		t.Fatalf("tolak: %v", err)
	}
	cancelled := mk(90000)
	if _, err := CancelEmergencyPurchase(cancelled.ID, models.EmergencyDecisionInput{Notes: "genset hidup lagi", ActorName: "Manajer Uji"}); err != nil {
		t.Fatalf("batal: %v", err)
	}
	waitingRatification := mk(50000)
	handOver(waitingRatification.ID)
	proof(waitingRatification.ID, 50000)
	// Kas Keluar biasa (tidak tertaut tugas) & Kas Masuk: hanya yang keluar dihitung.
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: movementID(), MovementType: "out", Amount: 75000,
		CounterpartName: "Solar genset", Note: "solar 30 liter", CreatedAt: now}); err != nil {
		t.Fatalf("kas keluar biasa: %v", err)
	}
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: movementID(), MovementType: "in", Amount: 20000,
		CounterpartName: "Kembalian darurat", Note: "kembalian struk", CreatedAt: now}); err != nil {
		t.Fatalf("kas masuk: %v", err)
	}

	recap, err := GetEmergencyRecap("", outlet, nil, nil)
	if err != nil {
		t.Fatalf("rekap: %v", err)
	}
	tot := recap.Totals
	check := func(name string, got models.EmergencyRecapBucket, n int, amount float64) {
		t.Helper()
		if got.Count != n || cents(got.Amount) != cents(amount) {
			t.Errorf("%s: dapat %d/%v, mau %d/%v", name, got.Count, got.Amount, n, amount)
		}
	}
	check("dicatat", tot.Recorded, 5, 620000)
	check("menunggu kas", tot.WaitingCash, 1, 100000)
	check("menunggu struk", tot.WaitingProof, 1, 150000)
	check("menunggu pengesahan", tot.WaitingRatification, 1, 50000)
	check("disahkan", tot.Ratified, 1, 200000)
	check("ditolak", tot.Rejected, 1, 120000)
	check("batal", tot.Cancelled, 1, 90000)
	check("kas keluar lain", tot.OtherCashOut, 1, 75000)
	if cents(tot.ProofDifference) != cents(20000) {
		t.Errorf("selisih struk: %v", tot.ProofDifference)
	}
	if len(recap.Outlets) != 1 || recap.Outlets[0].OutletID != outlet || recap.Outlets[0].Recorded.Count != 5 {
		t.Fatalf("per outlet: %+v", recap.Outlets)
	}
	if len(recap.Rows) != 6 || recap.Settings.MaxAmount != 500000 || recap.Month == "" || recap.MonthLabel == "" {
		t.Fatalf("rincian: %d baris, %+v", len(recap.Rows), recap)
	}
	byID := map[string]models.EmergencyRecapRow{}
	for _, r := range recap.Rows {
		byID[r.ID] = r
	}
	r := byID[ratified.ID]
	if r.Stage != EmergencyStageRatified || r.Cashier != "Kasir Uji" || r.AuthorizedBy != "Manajer Uji" ||
		r.Difference == nil || *r.Difference != 20000 || r.RatifiedBy != "Owner Uji" || !strings.Contains(r.Items, "Servis genset") {
		t.Fatalf("baris disahkan: %+v", r)
	}
	if c := byID[cancelled.ID]; c.Stage != EmergencyStageCancelled || c.CancelReason != "genset hidup lagi" {
		t.Fatalf("baris batal: %+v", c)
	}
	if w := byID[waitingCash.ID]; w.Stage != EmergencyStageWaitingCash || w.Difference != nil || w.PaidAt != nil {
		t.Fatalf("baris menunggu kas: %+v", w)
	}
	if rj := byID[rejected.ID]; rj.Stage != EmergencyStageRejected || rj.RatificationNotes != "harga terlalu mahal" {
		t.Fatalf("baris ditolak: %+v", rj)
	}

	// Cakupan: outlet di luar cakupan tidak terbaca; bulan lain kosong.
	if none, err := GetEmergencyRecap("", "", []string{"__none__"}, nil); err != nil || len(none.Rows) != 0 || len(none.Outlets) != 0 {
		t.Fatalf("di luar cakupan harus kosong: %+v %v", none, err)
	}
	if scoped, err := GetEmergencyRecap("", "", []string{outlet}, nil); err != nil || scoped.Totals.Recorded.Count != 5 {
		t.Fatalf("cakupan outlet: %+v %v", scoped, err)
	}
	if past, err := GetEmergencyRecap("2020-01", outlet, nil, nil); err != nil || len(past.Rows) != 0 || past.MonthLabel != "Januari 2020" {
		t.Fatalf("bulan lain: %+v %v", past, err)
	}
	if _, err := GetEmergencyRecap("11/2026", outlet, nil, nil); err == nil {
		t.Fatal("format bulan salah harus ditolak")
	}

	// Excel rekap: tiga sheet, enam baris rincian.
	data, name, err := BuildEmergencyRecapExcel("", outlet, nil, nil)
	if err != nil || !strings.HasPrefix(name, "Rekap-Pembelian-Darurat_") {
		t.Fatalf("excel rekap: %v %s", err, name)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("buka excel: %v", err)
	}
	defer f.Close()
	if sheets := f.GetSheetList(); strings.Join(sheets, "|") != "Ringkasan|Rincian|Cara Membaca" {
		t.Fatalf("sheet: %v", sheets)
	}
	if v, _ := f.GetCellValue("Rincian", "A7"); v != "6" {
		t.Fatalf("baris rincian ke-6: %q", v)
	}
	if v, _ := f.GetCellValue("Rincian", "A8"); v != "" {
		t.Fatalf("tidak boleh ada baris ke-7: %q", v)
	}

	// Ekspor Pembayaran: saringan darurat + kolom baru di belakang.
	pay, _, err := BuildProcurementPaymentsExcelExt("", "", "", "", []string{outlet}, nil, PurchaseListExtra{Emergency: true})
	if err != nil {
		t.Fatalf("ekspor pembayaran: %v", err)
	}
	pf, err := excelize.OpenReader(bytes.NewReader(pay))
	if err != nil {
		t.Fatalf("buka ekspor: %v", err)
	}
	defer pf.Close()
	for col, want := range map[string]string{"S1": "Catatan Pembayaran", "T1": "Tingkat", "U1": "Tahap Darurat", "W1": "Total Struk", "X1": "Selisih Kas−Struk"} {
		if v, _ := pf.GetCellValue("Daftar Pembayaran", col); v != want {
			t.Fatalf("header %s: %q, mau %q", col, v, want)
		}
	}
	rows, _ := pf.GetRows("Daftar Pembayaran")
	dataRows := 0
	foundDiff := false
	for _, row := range rows[1:] {
		if len(row) > 1 && row[0] != "" && row[0] != "TOTAL" && !strings.HasPrefix(row[0], "Catatan") {
			dataRows++
			if len(row) > 23 && row[1] == ratified.RequestNumber {
				foundDiff = row[19] == "Darurat" && row[20] == "Disahkan" && strings.Contains(row[23], "20")
			}
		}
	}
	// Dibatalkan tidak berkomitmen → tidak ada di halaman/ekspor Pembayaran.
	if dataRows != 5 || !foundDiff {
		t.Fatalf("ekspor darurat: %d baris, selisih ditemukan=%v", dataRows, foundDiff)
	}
	pend, _, err := BuildProcurementPaymentsExcelExt("", "", "", "", []string{outlet}, nil, PurchaseListExtra{Ratification: "pending"})
	if err != nil {
		t.Fatalf("ekspor menunggu pengesahan: %v", err)
	}
	ppf, _ := excelize.OpenReader(bytes.NewReader(pend))
	defer ppf.Close()
	prow, _ := ppf.GetRows("Daftar Pembayaran")
	n := 0
	for _, row := range prow[1:] {
		if len(row) > 1 && row[0] != "" && row[0] != "TOTAL" && !strings.HasPrefix(row[0], "Catatan") {
			n++
		}
	}
	if n != 3 { // menunggu kas, menunggu struk, menunggu pengesahan
		t.Fatalf("menunggu pengesahan: %d baris", n)
	}
}
