package services

// Uji regresi temuan uji integrasi UI 14 Okt 2026 (backend).

import (
	"strings"
	"sync"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Dokumen biasa yang SEMUA itemnya ditolak verifikator ber-items kosong dengan
// split_status NULL. fullySplitMasterCond dulu bernilai NULL untuk baris itu,
// sehingga "NOT cangkang" ikut membuangnya: dokumen ditolak hilang dari
// dashboard, daftar Pembayaran, dan daftar projek.
func TestFullyRejectedRequestIsNotAShell(t *testing.T) {
	requireDB(t)
	before, err := GetProcurementDashboard("", nil, nil)
	if err != nil {
		t.Fatalf("dashboard awal: %v", err)
	}
	pr := createVerifPR(t, []models.PurchaseRequestItem{{Name: "Peralatan", Items: []models.PurchaseSubItem{
		{Name: verifOddPrefix + "Ditolak-Semua-" + reviewSuffix(), Qty: 1, Unit: "unit", HpsPrice: 250000},
	}}})
	got, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version,
		models.VerifyRejectedItemInput{Group: 0, Item: 0, Reason: "Barang masih ada di outlet lain"}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != "rejected" || len(got.Items) != 0 {
		t.Fatalf("keadaan uji: status=%s items=%d, mau rejected tanpa item", got.Status, len(got.Items))
	}

	var kept bool
	if err := database.DB.QueryRow(`SELECT (NOT `+fullySplitMasterCond("")+`) IS TRUE FROM purchase_requests WHERE id = $1`,
		pr.ID).Scan(&kept); err != nil {
		t.Fatalf("cek kondisi: %v", err)
	}
	if !kept {
		t.Errorf("dokumen yang semua itemnya ditolak dianggap cangkang pecahan — ikut terbuang dari semua penjumlahan")
	}

	after, err := GetProcurementDashboard("", nil, nil)
	if err != nil {
		t.Fatalf("dashboard akhir: %v", err)
	}
	if after.TotalRequests != before.TotalRequests+1 {
		t.Errorf("total_requests %d → %d, mau bertambah 1", before.TotalRequests, after.TotalRequests)
	}
	if after.StatusBreakdown.Rejected != before.StatusBreakdown.Rejected+1 {
		t.Errorf("rejected %d → %d, mau bertambah 1", before.StatusBreakdown.Rejected, after.StatusBreakdown.Rejected)
	}
}

// Nomor pengajuan dulu MAX()+1 dari dokumen yang tersisa: menghapus dokumen
// terakhir hari itu membuat pengajuan berikutnya memakai ulang nomornya, dan
// dua pengajuan bersamaan bisa memperoleh nomor yang sama.
func TestRequestNumberNotReusedAfterDelete(t *testing.T) {
	requireDB(t)
	create := func() *models.PurchaseRequest {
		t.Helper()
		pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
			RequestType: RequestTypeJasa, RequestedBy: "pengaju uji nomor",
			Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
				{Name: "Zzz-Uji-Nomor-" + reviewSuffix(), Qty: 1, Unit: "kali", HpsPrice: 10000},
			}}},
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		reviewCleanup(t, pr.ID)
		return pr
	}
	first := create()
	if err := DeletePurchaseRequest(first.ID, true); err != nil {
		t.Fatalf("hapus: %v", err)
	}
	second := create()
	if second.RequestNumber == first.RequestNumber {
		t.Errorf("nomor %s dipakai ulang setelah dokumennya dihapus", first.RequestNumber)
	}

	const n = 8
	nums := make(chan string, n)
	var wg sync.WaitGroup
	now := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := generateRequestNumber(now)
			if err != nil {
				t.Errorf("nomor: %v", err)
				return
			}
			nums <- s
		}()
	}
	wg.Wait()
	close(nums)
	seen := map[string]bool{}
	for s := range nums {
		if seen[s] {
			t.Errorf("nomor kembar %s dari permintaan bersamaan", s)
		}
		seen[s] = true
	}
}

// Serah terima campuran aset + habis pakai dulu hanya berbunyi "1 aset
// dicatat" — baris habis pakainya tidak disebut sama sekali.
func TestReceiveMessageMentionsExpenseLines(t *testing.T) {
	requireDB(t)
	outlet := recvOutlet(t)
	sfx := recvSuffix()
	kursi, lap := "Kursi Uji Pesan "+sfx, "Lap Uji Pesan "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Perabot", Items: []models.PurchaseSubItem{
		{Name: kursi, Qty: 1, Unit: "unit", HpsPrice: 300000, FinalPrice: 300000},
		{Name: lap, Qty: 4, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	d := recvDraft(t, pr.ID)
	res, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, OutletID: outlet, Lines: []models.ReceiveGoodsLine{
		{PRItemKey: recvLine(t, d, kursi, "").PRItemKey, Destination: "aset", Qty: 1, TrackingMode: "tunggal"},
		{PRItemKey: recvLine(t, d, lap, "").PRItemKey, Destination: "habis", Qty: 4, Reason: "dipakai langsung untuk kebersihan dapur"},
	}}, recvActor, recvAll, nil)
	if err != nil {
		t.Fatalf("terima: %v", err)
	}
	if res.AssetsCreated != 1 || res.ExpenseLines != 1 {
		t.Errorf("assets_created=%d expense_lines=%d, mau 1 dan 1", res.AssetsCreated, res.ExpenseLines)
	}
	if !strings.Contains(res.Message, "1 aset dicatat") || !strings.Contains(res.Message, "1 baris dicatat sebagai habis pakai") {
		t.Errorf("pesan serah terima = %q, mau menyebut aset dan baris habis pakai", res.Message)
	}
}
