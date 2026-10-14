package services

// Struk pembelian (dibayar per struk) dan pengambilan barang oleh petugas
// logistik — keputusan owner 14 Okt 2026. TestMain + requireDB ada di
// purchase_flow_test.go (paket yang sama).

import (
	"errors"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

const strukPhoto = "/uploads/uji-struk.jpg"

func strukToday() string { return time.Now().In(GetTimezoneLocation()).Format("2006-01-02") }

// strukBill: pengajuan bahan baku disetujui, berharga final 39 jt di vendor
// terdaftar — dua item senilai 19 jt dan 20 jt.
func strukBill(t *testing.T) *models.PurchaseRequest {
	t.Helper()
	sfx := reviewSuffix()
	items := func(a, b float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Kebutuhan Gudang Induk", Items: []models.PurchaseSubItem{
			{Name: "Ayam Utuh Uji Struk " + sfx, Qty: 1, Unit: "lot", HpsPrice: 19000000, FinalPrice: a},
			{Name: "Daging Uji Struk " + sfx, Qty: 1, Unit: "lot", HpsPrice: 20000000, FinalPrice: b},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeBahan, RequestedBy: "Kepala Gudang Induk", Items: items(0, 0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM payment_histories WHERE purchase_request_id = $1`, pr.ID)
		execScratch(`DELETE FROM purchase_struks WHERE purchase_request_id = $1`, pr.ID)
		execScratch(`DELETE FROM purchase_requests WHERE id = $1`, pr.ID)
	})
	verifyAndApprove(t, pr.ID)
	vid := testVendorID(t)
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items(19000000, 20000000), VendorID: &vid}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	got, _ := GetPurchaseRequest(pr.ID)
	return got
}

func addStruk(t *testing.T, prID, number string, total float64) *models.PurchaseStruk {
	t.Helper()
	s, err := AddPurchaseStruk(prID, models.AddPurchaseStrukInput{
		StrukNumber: number, StrukDate: strukToday(), Total: total, PhotoURL: strukPhoto,
	}, Actor{Name: "Pembelian Bahan Uji"})
	if err != nil {
		t.Fatalf("struk %s: %v", number, err)
	}
	return s
}

func payStruk(prID, strukID string, amount float64) error {
	_, err := UpdatePurchaseStatus(prID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: amount, StrukID: strukID,
	})
	return err
}

func TestPaymentPerStruk(t *testing.T) {
	requireDB(t)
	pr := strukBill(t)
	var ve ValidationError

	// Validasi struk.
	if _, err := AddPurchaseStruk(pr.ID, models.AddPurchaseStrukInput{StrukNumber: "X", StrukDate: strukToday(), Total: 1000}, Actor{}); !errors.As(err, &ve) {
		t.Errorf("struk tanpa foto: %v, mau ValidationError", err)
	}
	a := addStruk(t, pr.ID, "STRUK-A", 19000000)
	b := addStruk(t, pr.ID, "STRUK-B", 20000000)
	if _, err := AddPurchaseStruk(pr.ID, models.AddPurchaseStrukInput{StrukNumber: "struk-a", StrukDate: strukToday(), Total: 1, PhotoURL: strukPhoto}, Actor{}); !errors.As(err, &ve) {
		t.Errorf("nomor struk kembar: %v, mau ValidationError", err)
	}

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Fatalf("ajukan bayar: %v", err)
	}
	// Cicil struk B 5 jt; melebihi sisa struk ditolak.
	if err := payStruk(pr.ID, b.ID, 5000000); err != nil {
		t.Fatalf("bayar struk B: %v", err)
	}
	if err := payStruk(pr.ID, b.ID, 15000001); !errors.As(err, &ve) {
		t.Errorf("bayar melebihi sisa struk B: %v, mau ValidationError", err)
	}
	// Nominal kosong + struk A = lunasi struk A.
	if err := payStruk(pr.ID, a.ID, 0); err != nil {
		t.Fatalf("lunasi struk A: %v", err)
	}
	// Struk yang sudah dibayar tidak bisa dihapus.
	if err := DeletePurchaseStruk(pr.ID, a.ID); !errors.As(err, &ve) {
		t.Errorf("hapus struk yang sudah dibayar: %v, mau ValidationError", err)
	}
	// Tanpa memilih struk: sisa dibagi ke struk yang belum lunas (B 15 jt).
	if err := payStruk(pr.ID, "", 0); err != nil {
		t.Fatalf("lunasi sisa: %v", err)
	}

	got, _ := GetPurchaseRequest(pr.ID)
	if got.Status != "paid" || got.PaidAmount != 39000000 {
		t.Errorf("status=%s paid=%v, mau paid / 39000000", got.Status, got.PaidAmount)
	}
	// Tagihan lunas: struk baru tidak akan pernah bisa dibayar.
	if _, err := AddPurchaseStruk(pr.ID, models.AddPurchaseStrukInput{StrukNumber: "STRUK-LUNAS", StrukDate: strukToday(), Total: 1000, PhotoURL: strukPhoto}, Actor{}); !errors.As(err, &ve) {
		t.Errorf("struk baru pada tagihan lunas: %v, mau ValidationError", err)
	}
	byNumber := map[string]models.PurchaseStruk{}
	for _, s := range got.Struks {
		byNumber[s.StrukNumber] = s
	}
	if s := byNumber["STRUK-A"]; s.PaidAmount != 19000000 || s.Remaining != 0 {
		t.Errorf("struk A dibayar %v sisa %v, mau 19000000 / 0", s.PaidAmount, s.Remaining)
	}
	if s := byNumber["STRUK-B"]; s.PaidAmount != 20000000 || s.Remaining != 0 {
		t.Errorf("struk B dibayar %v sisa %v, mau 20000000 / 0", s.PaidAmount, s.Remaining)
	}
	h, _ := GetPaymentHistories(pr.ID)
	var numbers []string
	for _, x := range h {
		numbers = append(numbers, x.StrukNumber)
	}
	if strings.Join(numbers, ",") != "STRUK-B,STRUK-A,STRUK-B" {
		t.Errorf("histori per struk = %v, mau STRUK-B, STRUK-A, STRUK-B", numbers)
	}
}

// Struk yang belum dibayar boleh dihapus; struk baru dicatat setelah
// pengajuan disetujui.
func TestStrukRulesBeforeApprovalAndDelete(t *testing.T) {
	requireDB(t)
	pending := createBahanUji(t) // Menunggu Persetujuan
	var ve ValidationError
	if _, err := AddPurchaseStruk(pending.ID, models.AddPurchaseStrukInput{StrukNumber: "S-1", StrukDate: strukToday(), Total: 1000, PhotoURL: strukPhoto}, Actor{}); !errors.As(err, &ve) {
		t.Errorf("struk sebelum disetujui: %v, mau ValidationError", err)
	}
	pr := strukBill(t)
	tomorrow := time.Now().In(GetTimezoneLocation()).AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := AddPurchaseStruk(pr.ID, models.AddPurchaseStrukInput{StrukNumber: "BESOK", StrukDate: tomorrow, Total: 1000, PhotoURL: strukPhoto}, Actor{}); !errors.As(err, &ve) {
		t.Errorf("struk bertanggal besok: %v, mau ValidationError", err)
	}
	c := addStruk(t, pr.ID, "STRUK-C", 1000000)
	if err := DeletePurchaseStruk(pr.ID, c.ID); err != nil {
		t.Errorf("hapus struk yang belum dibayar: %v", err)
	}
	if got, _ := GetPurchaseRequest(pr.ID); len(got.Struks) != 0 {
		t.Errorf("struk tersisa %d, mau 0", len(got.Struks))
	}
}

// Petugas logistik mencatat pengambilan; saat Gudang Induk menerima barang
// yang diantarnya, pengambilan itu ditutup.
func TestPickupClosedOnReceipt(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	ayam := "Ayam Utuh Uji Ambil " + sfx
	stockID := recvStockItem(t, ayam)
	wh := recvWarehouse(t)
	pr := recvApprovedPR(t, RequestTypeBahan, []models.PurchaseRequestItem{{Name: "Dapur", Items: []models.PurchaseSubItem{
		{Name: ayam, Qty: 10, Unit: "kg", HpsPrice: 38000, FinalPrice: 38000},
	}}})
	t.Cleanup(func() { execScratch(`DELETE FROM purchase_pickups WHERE purchase_request_id = $1`, pr.ID) })

	var ve ValidationError
	if _, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{Notes: "tanpa foto"}, Actor{Name: "Logistik"}); !errors.As(err, &ve) {
		t.Errorf("pengambilan tanpa foto: %v, mau ValidationError", err)
	}
	p, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{PhotoURL: recvPhoto, Notes: "10 kg, 2 box"}, Actor{Name: "Budi Logistik"})
	if err != nil {
		t.Fatalf("catat pengambilan: %v", err)
	}
	d := recvDraft(t, pr.ID)
	if len(d.Pickups) != 1 || d.Pickups[0].ID != p.ID {
		t.Fatalf("draf serah terima: pengambilan terbuka %+v, mau 1", d.Pickups)
	}
	line := recvLine(t, d, ayam, "")
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, PickupID: p.ID, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: line.PRItemKey, Destination: "stok", Qty: 10, StockItemID: stockID, WarehouseID: wh,
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima dari logistik: %v", err)
	}
	if left := recvDraft(t, pr.ID).Pickups; len(left) != 0 {
		t.Errorf("pengambilan masih terbuka setelah diterima: %+v", left)
	}
	var note string
	database.DB.QueryRow(`SELECT notes FROM handover_photos WHERE ref_id = $1 AND moment = 'terima' ORDER BY created_at DESC LIMIT 1`, pr.ID).Scan(&note)
	if !strings.Contains(note, "petugas logistik Budi Logistik") {
		t.Errorf("catatan serah terima = %q, mau menyebut petugas logistik pengantarnya", note)
	}
	got, _ := GetPurchaseRequest(pr.ID)
	if len(got.Pickups) != 1 || got.Pickups[0].ReceivedAt == "" || got.Pickups[0].ReceivedBy != recvActor.Name {
		t.Errorf("pengambilan setelah diterima = %+v, mau tertutup oleh penerima", got.Pickups)
	}
	// Barang sudah diterima seluruhnya: tidak ada lagi yang bisa diambil.
	if _, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{PhotoURL: recvPhoto}, Actor{Name: "Logistik"}); !errors.As(err, &ve) {
		t.Errorf("pengambilan setelah semua diterima: %v, mau ValidationError", err)
	}
}

// Pengajuan jasa tidak punya barang untuk diambil.
func TestPickupRejectedForServices(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
		{Name: verifOddPrefix + "Servis AC", Qty: 1, Unit: "kali", HpsPrice: 150000},
	}}})
	execScratch(`UPDATE purchase_requests SET request_type = 'jasa' WHERE id = $1`, pr.ID)
	verifyAndApprove(t, pr.ID)
	var ve ValidationError
	if _, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{PhotoURL: recvPhoto}, Actor{Name: "Logistik"}); !errors.As(err, &ve) {
		t.Errorf("pengambilan untuk jasa: %v, mau ValidationError", err)
	}
}

// Pengambilan yang tidak dipilih pada penerimaan terakhir (diterima sebagai
// "diantar vendor") ditutup otomatis begitu dokumen diterima lengkap — dulu
// tertinggal "dalam pengantaran" selamanya.
func TestOpenPickupsClosedWhenFullyReceived(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	beras := "Beras Uji Ambil " + sfx
	stockID := recvStockItem(t, beras)
	wh := recvWarehouse(t)
	pr := recvApprovedPR(t, RequestTypeBahan, []models.PurchaseRequestItem{{Name: "Dapur", Items: []models.PurchaseSubItem{
		{Name: beras, Qty: 10, Unit: "kg", HpsPrice: 15000, FinalPrice: 15000},
	}}})
	t.Cleanup(func() { execScratch(`DELETE FROM purchase_pickups WHERE purchase_request_id = $1`, pr.ID) })
	p1, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{PhotoURL: recvPhoto, Notes: "angkut 1"}, Actor{Name: "Logistik Satu"})
	if err != nil {
		t.Fatalf("pengambilan 1: %v", err)
	}
	if _, err := AddPurchasePickup(pr.ID, models.AddPurchasePickupInput{PhotoURL: recvPhoto, Notes: "angkut 2"}, Actor{Name: "Logistik Dua"}); err != nil {
		t.Fatalf("pengambilan 2: %v", err)
	}
	receive := func(qty int, pickupID string) {
		t.Helper()
		line := recvLine(t, recvDraft(t, pr.ID), beras, "")
		if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, PickupID: pickupID, Lines: []models.ReceiveGoodsLine{{
			PRItemKey: line.PRItemKey, Destination: "stok", Qty: qty, StockItemID: stockID, WarehouseID: wh,
		}}}, recvActor, recvAll, nil); err != nil {
			t.Fatalf("terima %d: %v", qty, err)
		}
	}
	receive(4, p1.ID)
	if open := recvDraft(t, pr.ID).Pickups; len(open) != 1 {
		t.Fatalf("setelah terima sebagian: %d pengambilan terbuka, mau 1", len(open))
	}
	receive(6, "") // sisa diterima sebagai "diantar vendor"
	got, _ := GetPurchaseRequest(pr.ID)
	for _, p := range got.Pickups {
		if p.ReceivedAt == "" {
			t.Errorf("pengambilan %s masih dalam pengantaran setelah diterima lengkap", p.ID)
		}
		if p.ID != p1.ID && !strings.Contains(p.ReceivedBy, "penerimaan lengkap") {
			t.Errorf("pengambilan yang ditutup otomatis: received_by = %q, mau bertanda penerimaan lengkap", p.ReceivedBy)
		}
	}
}
