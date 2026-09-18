package services

// Uji regresi alur pengadaan barang & jasa. Tiap tes di sini mengunci satu bug
// nyata yang pernah ada: split vendor yang tidak memindahkan item (dan karena
// itu memungkinkan dobel bayar serta dobel hitung), pencocokan item berdasarkan
// nama yang ikut membawa baris kembar, penghapusan pengajuan yang sudah ada
// pembayarannya, endpoint update item tanpa validasi, dan vendor yang terhapus
// diam-diam saat qty diubah.
//
// Butuh Postgres. Tanpa koneksi database seluruh paket ini di-skip, bukan gagal,
// supaya `go test ./...` tetap bisa dijalankan di mesin tanpa database.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
)

var dbReady bool

func TestMain(m *testing.M) {
	if err := database.Connect(config.Load()); err == nil {
		if err := database.DB.Ping(); err == nil {
			dbReady = true
		}
	}
	os.Exit(m.Run())
}

// requireDB melewati tes yang menyentuh database bila tidak ada koneksi.
func requireDB(t *testing.T) {
	t.Helper()
	if !dbReady {
		t.Skip("butuh Postgres; set DB_HOST/DB_USER/DB_PASSWORD/DB_NAME untuk menjalankannya")
	}
}

func execScratch(q string, args ...interface{}) { database.DB.Exec(q, args...) }

// Skenario penuh: buat PR 3 item, split 1 item ke vendor lain, lalu periksa
// bahwa item sisa tetap di master dan tidak ada nilai yang tergandakan.
func TestSplitMovesItemsOutOfMaster(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name: "Sembako",
			Items: []models.PurchaseSubItem{
				{Name: "Beras", Qty: 1, Unit: "sak", HpsPrice: 100000, FinalPrice: 100000},
				{Name: "Gula", Qty: 1, Unit: "kg", HpsPrice: 200000, FinalPrice: 200000},
				{Name: "Beras", Qty: 2, Unit: "sak", HpsPrice: 50000, FinalPrice: 50000},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	if pr.TotalFinal != 400000 {
		t.Fatalf("total awal = %v, mau 400000", pr.TotalFinal)
	}

	// Split SATU baris "Beras" saja (ada dua baris bernama sama).
	master, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Lain",
		Items: []models.PurchaseRequestItem{{
			Name:  "Sembako",
			Items: []models.PurchaseSubItem{{Name: "Beras", Qty: 1, HpsPrice: 100000, FinalPrice: 100000}},
		}},
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if len(master.Children) != 1 {
		t.Fatalf("jumlah pecahan = %d, mau 1", len(master.Children))
	}
	child := master.Children[0]
	if child.TotalFinal != 100000 {
		t.Errorf("total pecahan = %v, mau 100000 (hanya satu baris Beras yang ikut)", child.TotalFinal)
	}

	// Master harus menyisakan Gula + Beras kedua = 200000 + 100000.
	var ownFinal float64
	var itemsJSON []byte
	database.DB.QueryRow("SELECT total_final, items FROM purchase_requests WHERE id=$1", pr.ID).Scan(&ownFinal, &itemsJSON)
	if ownFinal != 300000 {
		t.Errorf("sisa milik master = %v, mau 300000", ownFinal)
	}
	var remaining []models.PurchaseRequestItem
	json.Unmarshal(itemsJSON, &remaining)
	if len(remaining) != 1 || len(remaining[0].Items) != 2 {
		t.Errorf("sisa item master = %+v, mau 1 grup berisi 2 item", remaining)
	}

	// Total dokumen (tampilan) = sisa + pecahan = nilai asli, tanpa kehilangan.
	if master.TotalFinal != 400000 {
		t.Errorf("total dokumen master = %v, mau 400000", master.TotalFinal)
	}
}

// Master yang seluruh itemnya sudah dipecah tidak boleh bisa dibayar.
func TestFullySplitMasterCannotBePaid(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "ATK",
			Items: []models.PurchaseSubItem{{Name: "Kertas", Qty: 1, Unit: "rim", HpsPrice: 50000, FinalPrice: 50000}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "atasan"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Lain",
		Items: []models.PurchaseRequestItem{{
			Name:  "ATK",
			Items: []models.PurchaseSubItem{{Name: "Kertas", Qty: 1, HpsPrice: 50000, FinalPrice: 50000}},
		}},
	}); err != nil {
		t.Fatalf("split: %v", err)
	}

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err != nil {
		t.Fatalf("request_payment: %v", err)
	}
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan", PaymentAmount: 50000})
	if err == nil {
		t.Fatal("master kosong berhasil dibayar — ini jalur dobel bayar yang harusnya tertutup")
	}
	t.Logf("ditolak dengan benar: %v", err)
}

// Menghapus pecahan mengembalikan itemnya ke induk, bukan menghilangkannya.
func TestDeletingChildReturnsItemsToParent(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name: "Dapur",
			Items: []models.PurchaseSubItem{
				{Name: "Panci", Qty: 1, Unit: "pcs", HpsPrice: 75000},
				{Name: "Wajan", Qty: 1, Unit: "pcs", HpsPrice: 25000},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	master, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Lain",
		Items: []models.PurchaseRequestItem{{
			Name:  "Dapur",
			Items: []models.PurchaseSubItem{{Name: "Panci", Qty: 1, HpsPrice: 75000}},
		}},
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if err := DeletePurchaseRequest(master.Children[0].ID, true); err != nil {
		t.Fatalf("delete child: %v", err)
	}

	back, err := GetPurchaseRequest(pr.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if back.TotalHps != 100000 {
		t.Errorf("total induk setelah pecahan dihapus = %v, mau 100000", back.TotalHps)
	}
	if back.SplitStatus != nil {
		t.Errorf("split_status = %v, mau nil karena tidak ada pecahan tersisa", *back.SplitStatus)
	}
	if len(back.Items) != 1 || len(back.Items[0].Items) != 2 {
		t.Errorf("item induk = %+v, mau kembali utuh 2 item", back.Items)
	}
}

// Pembayaran sebagian tidak boleh terhapus, dan qty nol ditolak.
func TestGuards(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: "tester", VendorName: "CV Jasa",
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis AC",
			Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 2, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	// Qty nol ditolak lewat jalur update (dulu lolos tanpa validasi).
	_, err = UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis AC",
			Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 0, HpsPrice: 100000}},
		}},
	})
	if err == nil {
		t.Error("qty 0 diterima — validasi update tidak jalan")
	}

	// Update tanpa mengirim vendor tidak boleh menghapus vendor.
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis AC",
			Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 3, HpsPrice: 100000, FinalPrice: 100000}},
		}},
	}); err != nil {
		t.Fatalf("update qty: %v", err)
	}
	after, _ := GetPurchaseRequest(pr.ID)
	if after.VendorName != "CV Jasa" {
		t.Errorf("vendor = %q, mau tetap %q", after.VendorName, "CV Jasa")
	}

	// Sebaliknya, vendor yang memang dikirim tetap harus tersimpan — jangan
	// sampai perbaikan di atas malah membuat form isi harga tidak berfungsi.
	newVendor := "CV Ganti"
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name:  "Servis AC",
			Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 3, HpsPrice: 100000, FinalPrice: 100000}},
		}},
		VendorName:    &newVendor,
		InvoiceNumber: strPtr("INV-001"),
	}); err != nil {
		t.Fatalf("update vendor: %v", err)
	}
	changed, _ := GetPurchaseRequest(pr.ID)
	if changed.VendorName != newVendor || changed.InvoiceNumber != "INV-001" {
		t.Errorf("vendor=%q invoice=%q, mau %q / INV-001", changed.VendorName, changed.InvoiceNumber, newVendor)
	}

	// Bayar sebagian, lalu coba hapus sebagai admin.
	UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "atasan"})
	UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"})
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 100000,
	}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	paid, _ := GetPurchaseRequest(pr.ID)
	if paid.Status != "partial" {
		t.Fatalf("status = %q, mau partial", paid.Status)
	}
	if err := DeletePurchaseRequest(pr.ID, true); err == nil {
		t.Error("pengajuan yang sudah dibayar sebagian terhapus berikut histori pembayarannya")
	}
	// 'partial' kini bisa diserahterimakan (dulu buntu).
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "receive", ActorName: "pengaju"}); err != nil {
		t.Errorf("receive dari partial ditolak: %v", err)
	}
}

// Query yang SQL-nya diubah harus benar-benar jalan di Postgres, bukan cuma
// lolos kompilasi Go.
func TestProcurementQueriesRun(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "Bahan Baku",
			Items: []models.PurchaseSubItem{{Name: "Tepung Cakra", Qty: 5, Unit: "sak", HpsPrice: 10000}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID) })

	if _, err := GetProcurementDashboard("", nil, nil); err != nil {
		t.Errorf("dashboard global: %v", err)
	}
	if _, err := GetProcurementDashboard("", []string{"x"}, []string{"y"}); err != nil {
		t.Errorf("dashboard scoped: %v", err)
	}
	if _, err := GetPaymentStats(nil, nil); err != nil {
		t.Errorf("payment stats: %v", err)
	}
	if _, err := GetPaymentStats([]string{"x"}, nil); err != nil {
		t.Errorf("payment stats scoped: %v", err)
	}
	if _, _, err := BuildProcurementPaymentsExcel("", "", "", "", nil, nil); err != nil {
		t.Errorf("export excel: %v", err)
	}

	// Pencarian harus menemukan lewat nama SUB-item, bukan hanya nama grup.
	for _, q := range []string{"Tepung Cakra", "Bahan Baku", pr.RequestNumber} {
		res, err := ListPurchaseRequests("", "", "", "", "", "all", true, q, nil, nil, 1, 20)
		if err != nil {
			t.Fatalf("list search %q: %v", q, err)
		}
		found := false
		for _, r := range res.Requests {
			if r.ID == pr.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("pencarian %q tidak menemukan pengajuan", q)
		}
	}
}

// Pengadaan yang di-split habis tidak boleh terhitung dua kali di dashboard:
// dulu master (nilai penuh) + pecahan (nilai penuh) sama-sama dijumlahkan.
func TestDashboardDoesNotDoubleCountSplit(t *testing.T) {
	requireDB(t)
	before, err := GetProcurementDashboard("", nil, nil)
	if err != nil {
		t.Fatalf("dashboard awal: %v", err)
	}

	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "Elektronik",
			Items: []models.PurchaseSubItem{{Name: "Kipas", Qty: 1, Unit: "pcs", HpsPrice: 300000, FinalPrice: 300000}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID) })

	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Lain",
		Items: []models.PurchaseRequestItem{{
			Name:  "Elektronik",
			Items: []models.PurchaseSubItem{{Name: "Kipas", Qty: 1, HpsPrice: 300000, FinalPrice: 300000}},
		}},
	}); err != nil {
		t.Fatalf("split: %v", err)
	}

	after, err := GetProcurementDashboard("", nil, nil)
	if err != nil {
		t.Fatalf("dashboard akhir: %v", err)
	}

	deltaAmount := after.TotalAmount - before.TotalAmount
	deltaCount := after.TotalRequests - before.TotalRequests
	if deltaAmount != 300000 {
		t.Errorf("kenaikan nilai = %v, mau 300000 (bukan 600000 alias dobel)", deltaAmount)
	}
	if deltaCount != 1 {
		t.Errorf("kenaikan jumlah pengajuan = %d, mau 1 (master kosong tidak ikut dihitung)", deltaCount)
	}
	t.Logf("delta nilai=%v jumlah=%d", deltaAmount, deltaCount)
}

// Master yang hanya di-split sebagian tetap memegang sisa item, dan sisa itu
// harus tetap muncul di halaman Pembayaran serta bisa dilunasi. Dulu sisa ini
// yatim: master disembunyikan total, jadi nilainya tidak pernah bisa ditagih.
func TestPartiallySplitMasterRemainsPayable(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name: "Gudang",
			Items: []models.PurchaseSubItem{
				{Name: "Rak", Qty: 1, Unit: "pcs", HpsPrice: 400000, FinalPrice: 400000},
				{Name: "Palet", Qty: 1, Unit: "pcs", HpsPrice: 600000, FinalPrice: 600000},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID) })

	UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "atasan"})
	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Palet",
		Items: []models.PurchaseRequestItem{{
			Name:  "Gudang",
			Items: []models.PurchaseSubItem{{Name: "Palet", Qty: 1, HpsPrice: 600000, FinalPrice: 600000}},
		}},
	}); err != nil {
		t.Fatalf("split: %v", err)
	}

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err != nil {
		t.Fatalf("request_payment: %v", err)
	}

	// Halaman Pembayaran (exclude_masters) harus tetap memuat master ini.
	list, err := ListPurchaseRequests("", "", "", "", "", "all", true, "", nil, nil, 1, 200)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var masterRow *models.PurchaseRequest
	for i := range list.Requests {
		if list.Requests[i].ID == pr.ID {
			masterRow = &list.Requests[i]
		}
	}
	if masterRow == nil {
		t.Fatal("master dengan sisa item hilang dari halaman Pembayaran — sisanya tidak akan pernah bisa ditagih")
	}
	if masterRow.TotalFinal != 400000 {
		t.Errorf("nominal tagihan master = %v, mau 400000 (hanya sisanya, bukan nilai penuh)", masterRow.TotalFinal)
	}

	// Dan sisa itu bisa dilunasi.
	paid, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 400000,
	})
	if err != nil {
		t.Fatalf("bayar sisa master: %v", err)
	}
	if paid.PaidAmount != 400000 {
		t.Errorf("terbayar = %v, mau 400000", paid.PaidAmount)
	}
	// Melebihi sisa harus ditolak, bukan diterima diam-diam.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 1,
	}); err == nil {
		t.Error("pembayaran melebihi tagihan diterima")
	}
}

// strPtr dipakai dari services/asset_import.go — deklarasi kedua di sini
// membuat paket tes tidak terkompilasi.

// ── Kunci dokumen & antrean penerimaan ──────────────────────────────────────

// Nama barang yang pasti tidak ada di katalog stok, supaya barisnya jatuh ke
// meja Perlengkapan dan tidak tergantung isi database.
const testOddItem = "Zzz-Uji-Barang-Perlengkapan-9f3a"

func newReceivableGoodsPR(t *testing.T, price float64) *models.PurchaseRequest {
	t.Helper()
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", VendorName: "CV Uji",
		Items: []models.PurchaseRequestItem{{
			Name:  "Uji Penerimaan",
			Items: []models.PurchaseSubItem{{Name: testOddItem, Qty: 2, Unit: "pcs", HpsPrice: price, FinalPrice: price}},
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "atasan"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return pr
}

func cleanupReceivablePR(id string) {
	execScratch("DELETE FROM pr_receiving_decisions WHERE purchase_request_id = $1", id)
	execScratch("DELETE FROM handover_photos WHERE ref_id = $1", id)
	execScratch("UPDATE purchase_requests SET receipt_status = '' WHERE id = $1", id)
	execScratch("DELETE FROM purchase_requests WHERE id = $1", id)
}

// Pembelian tempo: barang diterima saat status masih 'approved'. Sejak itu
// dokumen dikunci — tidak bisa dibatalkan, itemnya diubah, atau dihapus.
func TestReceivedRequestIsLocked(t *testing.T) {
	requireDB(t)
	pr := newReceivableGoodsPR(t, 50000)
	defer cleanupReceivablePR(pr.ID)

	draft, err := BuildReceivingDraft(pr.ID, nil)
	if err != nil || len(draft.Lines) != 1 {
		t.Fatalf("draft: %v (%d baris)", err, len(draft.Lines))
	}
	res, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{
		PhotoURL: "/uploads/uji.jpg",
		Lines:    []models.ReceiveGoodsLine{{PRItemKey: draft.Lines[0].PRItemKey, Destination: "habis", Qty: 2, Reason: "uji"}},
	}, "petugas", false, nil)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if res.ReceiptStatus != "received" {
		t.Fatalf("receipt_status = %q, mau received", res.ReceiptStatus)
	}
	got, _ := GetPurchaseRequest(pr.ID)
	if got.Status != "approved" {
		t.Fatalf("status = %q, mau tetap approved (belum dibayar)", got.Status)
	}

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "cancel", ActorName: "pengaju"}); err == nil {
		t.Error("pengajuan yang barangnya sudah diterima bisa dibatalkan")
	}
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: pr.Items}); err == nil {
		t.Error("item pengajuan yang barangnya sudah diterima bisa diubah")
	}
	if err := DeletePurchaseRequest(pr.ID, true); err == nil {
		t.Error("pengajuan yang barangnya sudah diterima bisa dihapus admin")
	}
}

// Baris stok yang ditunda (petugas tanpa hak gudang) TIDAK dihitung selesai:
// dokumen tetap terbuka, dan barisnya berpindah ke antrean Gudang Induk
// meski namanya tidak ada di katalog stok.
func TestQueuedStockLineStaysOpen(t *testing.T) {
	requireDB(t)
	pr := newReceivableGoodsPR(t, 50000)
	defer cleanupReceivablePR(pr.ID)

	draft, _ := BuildReceivingDraft(pr.ID, nil)
	key := draft.Lines[0].PRItemKey
	res, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{
		PhotoURL: "/uploads/uji.jpg",
		Lines:    []models.ReceiveGoodsLine{{PRItemKey: key, Destination: "stok", Qty: 2}},
	}, "petugas-aset", false, nil)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if res.StockLinesQueued != 1 || res.OutstandingLines != 1 || res.ReceiptStatus != "partial" {
		t.Fatalf("queued=%d outstanding=%d receipt=%q; mau 1/1/partial", res.StockLinesQueued, res.OutstandingLines, res.ReceiptStatus)
	}

	inQueue := func(kind string) bool {
		rows, err := ListReceivingQueue(kind, nil)
		if err != nil {
			t.Fatalf("queue %s: %v", kind, err)
		}
		for _, r := range rows {
			if r.PurchaseRequestID == pr.ID {
				return true
			}
		}
		return false
	}
	if !inQueue("dapur") {
		t.Error("baris stok yang ditunda tidak muncul di antrean Gudang Induk")
	}
	if inQueue("perlengkapan") {
		t.Error("baris stok yang ditunda masih nongkrong di antrean Perlengkapan")
	}
	again, _ := BuildReceivingDraft(pr.ID, nil)
	if again.Lines[0].Kind != "dapur" || again.Lines[0].Destination != "stok" || again.Lines[0].Remaining != 2 {
		t.Errorf("draft ulang: kind=%q dest=%q remaining=%d; mau dapur/stok/2",
			again.Lines[0].Kind, again.Lines[0].Destination, again.Lines[0].Remaining)
	}
}

// GRN manual yang menyebut nomor pengajuan yang masih hidup ditolak — jalur
// itu tidak menulis pr_item_key, jadi pengajuannya tidak akan pernah tertutup.
func TestManualReceiptRejectsLivePurchase(t *testing.T) {
	requireDB(t)
	pr := newReceivableGoodsPR(t, 50000)
	defer cleanupReceivablePR(pr.ID)

	_, err := CreateGoodsReceipt(models.GoodsReceiptRequest{
		WarehouseID: "tidak-dipakai", PORef: pr.RequestNumber,
		Items: []models.GoodsReceiptItemReq{{ItemID: "x", QtyDist: 1}},
	}, "gudang")
	if err == nil || !strings.Contains(err.Error(), pr.RequestNumber) {
		t.Errorf("GRN manual untuk pengajuan hidup tidak ditolak dengan menyebut nomornya: %v", err)
	}
	_, err = CreateGoodsReceipt(models.GoodsReceiptRequest{
		WarehouseID: "tidak-dipakai", PurchaseRequestID: pr.ID,
		Items: []models.GoodsReceiptItemReq{{ItemID: "x", QtyDist: 1}},
	}, "gudang")
	if err == nil || !strings.Contains(err.Error(), "Penerimaan dari Pengadaan") {
		t.Errorf("GRN bertaut pengajuan tanpa pr_item_key tidak ditolak: %v", err)
	}
}
