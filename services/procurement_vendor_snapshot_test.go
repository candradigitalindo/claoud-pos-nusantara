package services

// Rekening vendor pada pembayaran (keputusan owner 14 Okt 2026): rekening
// tujuan dipotret server dari data vendor saat pembayaran dicatat, transaksi
// lama tetap menunjuk rekening lama walau vendor mengganti nomor rekeningnya,
// perubahan rekening vendor dicatat (siapa, kapan, lama → baru), dan
// pengajuan baru bisa ditagihkan setelah memilih vendor terdaftar.

import (
	"errors"
	"strings"
	"testing"

	"cloud-pos/models"
)

func snapshotVendor(t *testing.T, name, bank, number, holder string) *models.Vendor {
	t.Helper()
	v, err := CreateVendor(models.CreateVendorRequest{Name: name, BankName: bank, AccountNumber: number, AccountHolder: holder})
	if err != nil {
		t.Fatalf("vendor uji: %v", err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM vendors WHERE id = $1`, v.ID) })
	return v
}

// snapshotBill: pengajuan jasa sampai 'payment_requested' ke vendor vendorID.
func snapshotBill(t *testing.T, item string, price float64, vendorID string) *models.PurchaseRequest {
	t.Helper()
	group := func(final float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: item, Qty: 1, Unit: "kali", HpsPrice: price, FinalPrice: final},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji rekening", Items: group(0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)
	verifyAndApprove(t, pr.ID)
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: group(price), VendorID: &vendorID}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Fatalf("ajukan bayar: %v", err)
	}
	return pr
}

func TestPaymentSnapshotsVendorAccount(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	v := snapshotVendor(t, "CV Rekening Uji "+sfx, "BCA", "1110001110", "CV Rekening Uji")
	pr := snapshotBill(t, "Zzz-Uji-Rekening-"+sfx, 1000000, v.ID)

	// Rekening ketikan pembayar diabaikan: yang tercatat rekening vendor.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAmount: 400000,
		PaymentAccountDest: "BRI 9999999999 a.n. Orang Lain",
	}); err != nil {
		t.Fatalf("bayar 1: %v", err)
	}
	h, err := GetPaymentHistories(pr.ID)
	if err != nil || len(h) != 1 {
		t.Fatalf("histori: %v (%d baris)", err, len(h))
	}
	if h[0].PaymentAccountDest != "BCA 1110001110 a.n. CV Rekening Uji" || h[0].DestAccountNumber != "1110001110" ||
		h[0].DestManual || h[0].VendorID != v.ID || h[0].VendorName != v.Name {
		t.Errorf("potret pembayaran 1 = %+v, mau rekening BCA 1110001110 dari data vendor (bukan ketikan pembayar)", h[0])
	}

	// Vendor mengganti rekening: perubahan dicatat beserta pelakunya.
	upd := models.UpdateVendorRequest{Name: v.Name, BankName: "Mandiri", AccountNumber: "2220002220", AccountHolder: "CV Rekening Uji"}
	got, err := UpdateVendor(v.ID, upd, Actor{Name: "Admin Uji"})
	if err != nil {
		t.Fatalf("ganti rekening: %v", err)
	}
	if len(got.BankChanges) != 1 {
		t.Fatalf("riwayat rekening = %d baris, mau 1", len(got.BankChanges))
	}
	c := got.BankChanges[0]
	if c.OldBankName != "BCA" || c.OldAccountNumber != "1110001110" || c.NewBankName != "Mandiri" ||
		c.NewAccountNumber != "2220002220" || c.ChangedBy != "Admin Uji" {
		t.Errorf("riwayat rekening = %+v, mau BCA 1110001110 → Mandiri 2220002220 oleh Admin Uji", c)
	}
	// Ubah data lain tanpa menyentuh rekening: tidak menambah riwayat.
	upd.Phone = "0812000111"
	if again, err := UpdateVendor(v.ID, upd, Actor{Name: "Admin Uji"}); err != nil || len(again.BankChanges) != 1 {
		t.Errorf("ubah telepon menambah riwayat rekening: %v (%d baris)", err, len(again.BankChanges))
	}

	// Pelunasan memakai rekening baru; pembayaran lama tetap rekening lama.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"}); err != nil {
		t.Fatalf("bayar 2: %v", err)
	}
	h, _ = GetPaymentHistories(pr.ID)
	if len(h) != 2 {
		t.Fatalf("histori = %d baris, mau 2", len(h))
	}
	if h[0].PaymentAccountDest != "BCA 1110001110 a.n. CV Rekening Uji" {
		t.Errorf("pembayaran lama berubah menjadi %q setelah vendor ganti rekening", h[0].PaymentAccountDest)
	}
	if h[1].PaymentAccountDest != "Mandiri 2220002220 a.n. CV Rekening Uji" || h[1].DestBankName != "Mandiri" {
		t.Errorf("pembayaran baru = %q, mau rekening Mandiri yang berlaku saat dibayar", h[1].PaymentAccountDest)
	}
}

// Vendor tanpa rekening terdaftar (mis. belanja tunai di pasar): pembayar wajib
// menulis rekening/cara bayarnya, dan pembayaran itu ditandai manual.
func TestPayVendorWithoutAccountIsManual(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	v := snapshotVendor(t, "Pasar Uji "+sfx, "", "", "")
	pr := snapshotBill(t, "Zzz-Uji-Tunai-"+sfx, 250000, v.ID)

	_, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"})
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "belum punya rekening terdaftar") {
		t.Fatalf("bayar vendor tanpa rekening tanpa keterangan: %v, mau ValidationError", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "pay", ActorName: "keuangan", PaymentAccountDest: "Tunai",
	}); err != nil {
		t.Fatalf("bayar tunai: %v", err)
	}
	h, _ := GetPaymentHistories(pr.ID)
	if len(h) != 1 || h[0].PaymentAccountDest != "Tunai" || !h[0].DestManual || h[0].VendorName != v.Name {
		t.Errorf("histori tunai = %+v, mau Tunai bertanda manual", h)
	}
}

// Pengajuan baru bisa ditagihkan setelah memilih vendor terdaftar; pecahan
// tanpa vendor terdaftar tidak ikut diajukan bersama induknya.
func TestRequestPaymentRequiresRegisteredVendor(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	a, b := "Zzz-Uji-Vendor-A-"+sfx, "Zzz-Uji-Vendor-B-"+sfx
	items := func(fa, fb float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: a, Qty: 1, Unit: "kali", HpsPrice: 100000, FinalPrice: fa},
			{Name: b, Qty: 1, Unit: "kali", HpsPrice: 200000, FinalPrice: fb},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji vendor", Items: items(0, 0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)
	verifyAndApprove(t, pr.ID)

	// Vendor nama bebas (lewat API, tanpa vendor terdaftar) → tidak bisa ditagihkan.
	free := "CV Nama Bebas " + sfx
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items(100000, 200000), VendorName: &free}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"})
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "belum memilih vendor dari daftar vendor") {
		t.Fatalf("ajukan bayar tanpa vendor terdaftar: %v, mau ValidationError", err)
	}

	// Pecah: A ke vendor nama bebas, B ke vendor terdaftar. Induk diajukan →
	// hanya pecahan B yang ikut.
	vb := snapshotVendor(t, "CV Terdaftar "+sfx, "BNI", "3330003330", "CV Terdaftar")
	split := func(vendorID, vendorName, item string) *models.PurchaseRequest {
		t.Helper()
		m, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
			VendorID: vendorID, VendorName: vendorName,
			Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{{Name: item}}}},
		})
		if err != nil {
			t.Fatalf("split %s: %v", item, err)
		}
		for i := range m.Children {
			if m.Children[i].Items[0].Items[0].Name == item {
				return &m.Children[i]
			}
		}
		t.Fatalf("pecahan %s tidak ditemukan", item)
		return nil
	}
	ca := split("", free, a)
	cb := split(vb.ID, vb.Name, b)
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Fatalf("ajukan bayar induk: %v", err)
	}
	if st, _ := recvStatus(t, cb.ID); st != "payment_requested" {
		t.Errorf("pecahan bervendor terdaftar = %s, mau payment_requested", st)
	}
	if st, _ := recvStatus(t, ca.ID); st != "approved" {
		t.Errorf("pecahan tanpa vendor terdaftar = %s, mau tetap approved (diajukan sendiri setelah memilih vendor)", st)
	}
	// Setelah memilih vendor terdaftar, pecahan A bisa diajukan sendiri.
	va := snapshotVendor(t, "CV Pilihan "+sfx, "BRI", "4440004440", "CV Pilihan")
	child, _ := GetPurchaseRequest(ca.ID)
	if _, err := UpdatePurchaseItems(ca.ID, models.UpdatePurchaseItemsInput{Items: child.Items, VendorID: &va.ID}); err != nil {
		t.Fatalf("pilih vendor pecahan A: %v", err)
	}
	if _, err := UpdatePurchaseStatus(ca.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Errorf("ajukan bayar pecahan A setelah memilih vendor: %v", err)
	}
}
