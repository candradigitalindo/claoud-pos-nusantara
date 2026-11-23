package services

import (
	"errors"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Pengajuan jasa langsung menunggu persetujuan tanpa melalui verifikasi,
// mengikuti pola bahan baku: lahir 'verified', aksi 'verify' ditolak, pengaju
// masih boleh mengubah isi sampai disetujui, dan notifikasi WA langsung ke
// pemegang izin procurement.requests.approve_services.
func TestJasaSkipsVerificationSpec(t *testing.T) {
	if !SkipsVerification(RequestTypeJasa) {
		t.Fatal("SkipsVerification(jasa) harus true")
	}
	spec := waSpec("purchase.service_submitted")
	if spec == nil || len(spec.Permissions) != 1 || spec.Permissions[0] != "procurement.requests.approve_services" || spec.Extra != "pengaju" {
		t.Fatalf("event WA pengajuan jasa harus ke pemegang procurement.requests.approve_services + pengaju: %+v", spec)
	}
}

func TestJasaSkipsVerification(t *testing.T) {
	requireDB(t)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "tester",
		Items: []models.PurchaseRequestItem{{
			Name:  "Perawatan AC",
			Items: []models.PurchaseSubItem{{Name: "Cuci AC Uji Skip Verif", Qty: 2, Unit: "unit", HpsPrice: 100000}},
		}},
	})
	if err != nil {
		t.Fatalf("create jasa: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)
	})
	if pr.Status != "verified" {
		t.Fatalf("status awal jasa = %q, mau verified (Menunggu Persetujuan)", pr.Status)
	}

	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "verify", ActorName: "v", Version: pr.Version})
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("verifikasi jasa: %v, mau ValidationError", err)
	}

	// Pengaju masih boleh mengubah isi selama menunggu persetujuan.
	items := clonePurchaseItems(pr.Items)
	items[0].Items[0].Qty = 4
	edited, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items, Version: pr.Version})
	if err != nil {
		t.Fatalf("ubah isi jasa saat menunggu persetujuan: %v", err)
	}
	if edited.Status != "verified" || edited.Items[0].Items[0].Qty != 4 || edited.TotalHps != 400000 {
		t.Errorf("setelah diubah: status=%s qty=%v hps=%v, mau verified / 4 / 400000", edited.Status, edited.Items[0].Items[0].Qty, edited.TotalHps)
	}

	// Penyetuju jasa langsung menyetujui dengan versi terbaru (versi lama ditolak).
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Penyetuju Jasa", Version: pr.Version}); err == nil {
		t.Error("persetujuan memakai versi lama berhasil — harus ditolak 409")
	}
	got, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Penyetuju Jasa", Version: edited.Version})
	if err != nil || got.Status != "approved" {
		t.Fatalf("setujui jasa: %v (status %v)", err, got)
	}
}
