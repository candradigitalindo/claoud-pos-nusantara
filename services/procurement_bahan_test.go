package services

// Uji alur Pengadaan Bahan Baku (request_type 'bahan') yang butuh Postgres.
// TestMain + requireDB ada di purchase_flow_test.go (paket yang sama).

import (
	"errors"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Nama bahan sengaja tidak ada di katalog stok: pengajuan barang biasa akan
// mengklasifikasikannya 'perlengkapan', pengajuan bahan baku tetap 'dapur'.
const bahanUjiName = "Tepung Uji Bahan Baku zq7"

func createBahanUji(t *testing.T) *models.PurchaseRequest {
	t.Helper()
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeBahan, RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "Kebutuhan Dapur",
			Items: []models.PurchaseSubItem{{Name: bahanUjiName, Qty: 3, Unit: "kg", HpsPrice: 12000}},
		}},
	})
	if err != nil {
		t.Fatalf("create bahan: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)
	})
	return pr
}

// Bahan baku tanpa verifikasi: pengajuan Kepala Gudang Induk langsung
// menunggu persetujuan Manager Logistik (keputusan owner 14 Okt 2026).
func TestBahanCreatedAsDapurAwaitingApproval(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)
	if pr.Status != "verified" {
		t.Errorf("status = %q, mau verified (Menunggu Persetujuan, tanpa verifikasi)", pr.Status)
	}
	if pr.RequestType != RequestTypeBahan {
		t.Errorf("request_type = %q, mau bahan", pr.RequestType)
	}
	var kind string
	database.DB.QueryRow("SELECT COALESCE(goods_kind,'') FROM purchase_requests WHERE id = $1", pr.ID).Scan(&kind)
	if kind != "dapur" {
		t.Errorf("goods_kind = %q, mau dapur", kind)
	}
}

func TestBahanRejectsProject(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeBahan, RequestedBy: "tester", ProjectID: "01PROJEKUJIBAHANBAKU000000",
		Items: []models.PurchaseRequestItem{{
			Name:  "Kebutuhan Dapur",
			Items: []models.PurchaseSubItem{{Name: bahanUjiName, Qty: 1, Unit: "kg", HpsPrice: 1000}},
		}},
	})
	if err == nil {
		database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)
		t.Fatal("pengajuan bahan bertaut projek seharusnya ditolak")
	}
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Msg != "Pengadaan bahan baku tidak terkait projek" {
		t.Errorf("error = %v, mau ValidationError 'Pengadaan bahan baku tidak terkait projek'", err)
	}
}

func TestListPurchaseRequestsAllowedTypesHidesBahan(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)

	hidden, err := ListPurchaseRequests("", "", "", "", "", "all", false, pr.RequestNumber, nil, nil,
		[]string{RequestTypeBarang, RequestTypeJasa}, 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, r := range hidden.Requests {
		if r.ID == pr.ID {
			t.Fatal("pengajuan bahan muncul padahal jenis yang diizinkan hanya barang/jasa")
		}
	}

	shown, err := ListPurchaseRequests("", "", "", "", "", "all", false, pr.RequestNumber, nil, nil,
		[]string{RequestTypeBahan}, 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, r := range shown.Requests {
		if r.ID == pr.ID {
			found = true
		}
	}
	if !found {
		t.Error("pengajuan bahan harus muncul bila jenis bahan diizinkan")
	}

	none, err := ListPurchaseRequests("", "", "", "", "", "all", false, pr.RequestNumber, nil, nil,
		[]string{}, 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if none.Total != 0 {
		t.Errorf("tanpa jenis yang diizinkan, total = %d, mau 0", none.Total)
	}
}

func TestLineKindOfBahanIsDapur(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)
	key := prItemKey("Kebutuhan Dapur", bahanUjiName, 0)
	got, err := lineKindOf(database.DB, pr.ID, RequestTypeBahan, "", key, models.PurchaseSubItem{Name: bahanUjiName})
	if err != nil || got != "dapur" {
		t.Errorf("lineKindOf baris bahan = %q (%v), mau dapur", got, err)
	}
	// Jenis baris dibekukan di item saat dibuat (T-19).
	if k := pr.Items[0].Items[0].GoodsKind; k != "dapur" {
		t.Errorf("goods_kind baris bahan tersimpan = %q, mau dapur", k)
	}
}

// Edit item pengajuan bahan tetap menolak rujukan baris RAB, sama seperti saat
// dibuat: bahan baku tidak pernah terkait projek.
func TestBahanUpdateItemsRejectsRabItem(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)
	_, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name: "Kebutuhan Dapur",
			Items: []models.PurchaseSubItem{{Name: bahanUjiName, Qty: 3, Unit: "kg", HpsPrice: 12000,
				RabItemID: "01RABUJIBAHANBAKU000000000"}},
		}},
	})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, mau ValidationError rujukan RAB", err)
	}
}

// Bahan baku hanya diterima ke stok gudang; tujuan aset/habis pakai ditolak,
// dan antrean serah terima menyembunyikannya dari yang hanya boleh barang/jasa.
func TestBahanReceiveOnlyToStockAndQueueFiltered(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)
	if _, err := database.DB.Exec("UPDATE purchase_requests SET status = 'approved' WHERE id = $1", pr.ID); err != nil {
		t.Fatalf("set approved: %v", err)
	}
	draft, err := BuildReceivingDraft(pr.ID, nil)
	if err != nil || len(draft.Lines) != 1 {
		t.Fatalf("draft: %v", err)
	}
	if draft.RequestType != RequestTypeBahan {
		t.Errorf("draft.request_type = %q, mau bahan", draft.RequestType)
	}
	for _, dest := range []string{"aset", "habis"} {
		_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{
			PhotoURL: "/uploads/uji.jpg", OutletID: "x",
			Lines: []models.ReceiveGoodsLine{{PRItemKey: draft.Lines[0].PRItemKey, Destination: dest, Qty: 1, Reason: "langsung dipakai uji"}},
		}, Actor{Name: "petugas"}, ReceiveAccess{Submit: true, Stock: true}, nil)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("tujuan %q: error = %v, mau ValidationError", dest, err)
		}
	}

	inQueue := func(allowed []string) bool {
		rows, err := ListReceivingQueue("", nil, allowed)
		if err != nil {
			t.Fatalf("queue: %v", err)
		}
		for _, r := range rows {
			if r.PurchaseRequestID == pr.ID {
				return true
			}
		}
		return false
	}
	if inQueue([]string{RequestTypeBarang, RequestTypeJasa}) {
		t.Error("pengajuan bahan muncul di antrean padahal jenis yang diizinkan hanya barang/jasa")
	}
	if !inQueue([]string{RequestTypeBahan}) || !inQueue(nil) {
		t.Error("pengajuan bahan harus muncul bila jenis bahan diizinkan / tanpa batas")
	}
}

// Alur bahan baku tanpa verifikasi (keputusan owner 14 Okt 2026): Kepala
// Gudang Induk mengajukan → Manager Logistik menyetujui. Pengajuan lahir
// 'verified' (Menunggu Persetujuan), aksi verifikasi ditolak, pengaju masih
// boleh mengubah isi sampai disetujui, dan WA pengajuannya langsung ke
// pemegang izin persetujuan bahan baku.
func TestBahanSkipsVerification(t *testing.T) {
	requireDB(t)
	pr := createBahanUji(t)

	_, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "verify", ActorName: "v", Version: pr.Version})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("verifikasi bahan baku: %v, mau ValidationError", err)
	}

	// Pengaju mengubah qty selama menunggu persetujuan.
	items := clonePurchaseItems(pr.Items)
	items[0].Items[0].Qty = 5
	edited, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items, Version: pr.Version})
	if err != nil {
		t.Fatalf("ubah isi bahan baku yang menunggu persetujuan: %v", err)
	}
	if edited.Status != "verified" || edited.Items[0].Items[0].Qty != 5 || edited.TotalHps != 60000 {
		t.Errorf("setelah diubah: status=%s qty=%v hps=%v, mau verified / 5 / 60000", edited.Status, edited.Items[0].Items[0].Qty, edited.TotalHps)
	}
	// Penyetuju memutus atas isi terbaru; versi lama ditolak.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Manager Logistik", Version: pr.Version}); err == nil {
		t.Error("persetujuan memakai versi sebelum diubah berhasil — penyetuju harus memuat ulang isi terbaru")
	}
	got, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Manager Logistik", Version: edited.Version})
	if err != nil || got.Status != "approved" {
		t.Fatalf("setujui bahan baku: %v (status %v)", err, got)
	}

	spec := waSpec("purchase.rawmaterial_submitted")
	if spec == nil || len(spec.Permissions) != 1 || spec.Permissions[0] != "procurement.rawmaterials.approve" {
		t.Errorf("event WA pengajuan bahan baku harus ke pemegang procurement.rawmaterials.approve: %+v", spec)
	}
}

// Barang yang sudah diverifikasi tetap terkunci isinya.
func TestBarangLockedAfterVerification(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	ver, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	var ve ValidationError
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: ver.Items}); !errors.As(err, &ve) {
		t.Errorf("ubah isi barang yang sudah diverifikasi: %v, mau ValidationError", err)
	}
}
