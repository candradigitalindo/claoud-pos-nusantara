package services

// Uji regresi: vendor yang dikirim saat MEMBUAT pengajuan diperiksa seperti
// saat pecah vendor & isi harga (T-67). Ditemukan saat uji E2E 14 Okt 2026:
// vendor_id yang tidak ada gagal di foreign key (HTTP 500), vendor nonaktif
// diterima, dan nama vendor bebas tersimpan berdampingan dengan id vendor lain.

import (
	"errors"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestCreatePurchaseVendorResolvedFromVendorTable(t *testing.T) {
	requireDB(t)
	sfx := strings.ToLower(NewULID()[18:])
	active, inactive := NewULID(), NewULID()
	for _, v := range []struct {
		id, name string
		on       bool
	}{{active, "CV Resmi Buat Uji " + sfx, true}, {inactive, "CV Mati Buat Uji " + sfx, false}} {
		if _, err := database.DB.Exec(`INSERT INTO vendors (id, name, is_active) VALUES ($1, $2, $3)`, v.id, v.name, v.on); err != nil {
			t.Fatalf("vendor uji: %v", err)
		}
	}
	created := []string{}
	t.Cleanup(func() {
		for _, id := range created {
			execScratch(`DELETE FROM purchase_requests WHERE id = $1`, id)
		}
		execScratch(`DELETE FROM vendors WHERE id IN ($1, $2)`, active, inactive)
	})
	create := func(vendorID, vendorName string) (*models.PurchaseRequest, error) {
		pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
			RequestType: RequestTypeJasa, RequestedBy: "Pengaju Uji Vendor",
			VendorID: vendorID, VendorName: vendorName,
			Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
				{Name: "Servis Uji Vendor " + sfx, Qty: 1, Unit: "paket", HpsPrice: 100000, HpsSource: "uji"},
			}}},
		})
		if pr != nil {
			created = append(created, pr.ID)
		}
		return pr, err
	}

	var ve ValidationError
	if _, err := create("01KVENDORTIDAKADA000000000", "Nama Bebas"); !errors.As(err, &ve) {
		t.Errorf("vendor tidak ada: err = %v, mau ValidationError (bukan galat foreign key → HTTP 500)", err)
	}
	if _, err := create(inactive, "Nama Bebas"); !errors.As(err, &ve) {
		t.Errorf("vendor nonaktif: err = %v, mau ValidationError", err)
	}
	pr, err := create(active, "Nama Palsu Pilihan Pengaju")
	if err != nil {
		t.Fatalf("vendor aktif: %v", err)
	}
	if pr.VendorID == nil || strings.TrimSpace(*pr.VendorID) != active || pr.VendorName != "CV Resmi Buat Uji "+sfx {
		t.Errorf("vendor tersimpan = %v / %q, mau %s / nama dari tabel vendors (T-67)", pr.VendorID, pr.VendorName, active)
	}
	// Tanpa vendor_id, nama bebas tetap diterima seperti sebelumnya.
	pr, err = create("", "Toko Langganan "+sfx)
	if err != nil || pr.VendorID != nil || pr.VendorName != "Toko Langganan "+sfx {
		t.Errorf("tanpa vendor_id: err=%v vendor=%v/%q", err, pr.VendorID, pr.VendorName)
	}
}
