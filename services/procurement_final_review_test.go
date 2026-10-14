package services

// Uji regresi peninjauan akhir (14 Okt 2026) atas perubahan pengadaan
// beberapa agen: harga final & invoice hanya dari pembelian (T-39), status
// induk diturunkan ulang setelah pecah vendor, dan sisa tagihan dokumen
// 'received' yang belum lunas di ekspor Excel Pembayaran (T-12/T-76).

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

func reviewSuffix() string { return strings.ToLower(NewULID()[18:]) }

// reviewCleanup menghapus satu keluarga pengajuan beserta histori bayarnya.
func reviewCleanup(t *testing.T, id string) {
	t.Helper()
	t.Cleanup(func() {
		execScratch(`DELETE FROM payment_histories WHERE purchase_request_id IN (SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)`, id)
		execScratch(`DELETE FROM purchase_requests WHERE parent_id = $1`, id)
		execScratch(`DELETE FROM purchase_requests WHERE id = $1`, id)
	})
}

// T-39: pengaju tidak bisa mengisi harga final, subtotal final, maupun nomor
// invoice — saat membuat maupun saat mengubah pengajuan pending. Hanya
// pembelian yang mengisinya setelah pengajuan disetujui.
func TestRequesterFinalPriceAndInvoiceIgnored(t *testing.T) {
	requireDB(t)
	name := "Zzz-Uji-T39-Servis-" + reviewSuffix()
	items := func(final, subtotal float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", FinalTotal: subtotal, Items: []models.PurchaseSubItem{
			{Name: name, Qty: 2, Unit: "kali", HpsPrice: 100000, FinalPrice: final, FinalSubtotal: subtotal},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji T-39", Items: items(90000, 180000),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)

	noFinal := func(stage string, got *models.PurchaseRequest) {
		t.Helper()
		sub := got.Items[0].Items[0]
		if sub.FinalPrice != 0 || sub.FinalSubtotal != 0 || got.Items[0].FinalTotal != 0 || got.TotalFinal != 0 {
			t.Errorf("%s: harga final dari pengaju tersimpan (final_price=%v final_subtotal=%v final_total=%v total_final=%v) — mau semuanya 0",
				stage, sub.FinalPrice, sub.FinalSubtotal, got.Items[0].FinalTotal, got.TotalFinal)
		}
		if got.TotalHps != 200000 || got.TotalAmount != 200000 {
			t.Errorf("%s: total_hps=%v total_amount=%v, mau 200000 (nilai pengajuan = HPS)", stage, got.TotalHps, got.TotalAmount)
		}
		if got.InvoiceNumber != "" {
			t.Errorf("%s: nomor invoice dari pengaju tersimpan (%q) — mau kosong", stage, got.InvoiceNumber)
		}
	}
	noFinal("buat", pr)

	invPengaju := "INV-DARI-PENGAJU"
	edited, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: items(80000, 160000), InvoiceNumber: &invPengaju,
	})
	if err != nil {
		t.Fatalf("ubah pending: %v", err)
	}
	noFinal("ubah pending", edited)

	// Setelah disetujui, pembelian yang mengisi harga final dan invoice.
	verifyAndApprove(t, pr.ID)
	invBeli := "INV-PEMBELIAN-T39"
	priced, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: items(95000, 0), InvoiceNumber: &invBeli,
	})
	if err != nil {
		t.Fatalf("isi harga oleh pembelian: %v", err)
	}
	if priced.TotalFinal != 190000 || priced.Items[0].Items[0].FinalSubtotal != 190000 || priced.InvoiceNumber != invBeli {
		t.Errorf("pembelian: total_final=%v subtotal=%v invoice=%q, mau 190000/190000/%s",
			priced.TotalFinal, priced.Items[0].Items[0].FinalSubtotal, priced.InvoiceNumber, invBeli)
	}
}

// Pecah vendor yang menghabiskan sisa item induk mengubah keluarga dokumen:
// status induk wajib diturunkan ulang saat itu juga, bukan menunggu peristiwa
// berikutnya. Dulu induk tetap 'approved' walau pecahan lain sudah lunas.
func TestSplitRederivesMasterStatus(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	a, b := "Zzz-Uji-Pecah-A-"+sfx, "Zzz-Uji-Pecah-B-"+sfx
	group := func(fa, fb float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: a, Qty: 1, Unit: "kali", HpsPrice: 100000, FinalPrice: fa},
			{Name: b, Qty: 1, Unit: "kali", HpsPrice: 200000, FinalPrice: fb},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji pecah", Items: group(0, 0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)
	verifyAndApprove(t, pr.ID)
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: group(100000, 200000), VendorID: strPtr(testVendorID(t))}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}

	split := func(vendor, item string) string {
		t.Helper()
		m, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
			VendorID: testVendorNamed(t, vendor), VendorName: vendor, Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{{Name: item}}}},
		})
		if err != nil {
			t.Fatalf("split %s: %v", item, err)
		}
		for _, ch := range m.Children {
			if ch.VendorName == vendor {
				return ch.ID
			}
		}
		t.Fatalf("pecahan %s tidak ditemukan", vendor)
		return ""
	}
	childA := split("CV Pecah A", a)
	if _, err := UpdatePurchaseStatus(childA, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"}); err != nil {
		t.Fatalf("ajukan bayar A: %v", err)
	}
	if _, err := UpdatePurchaseStatus(childA, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"}); err != nil {
		t.Fatalf("bayar A: %v", err)
	}
	if st, _ := recvStatus(t, pr.ID); st != "approved" {
		t.Fatalf("induk masih memegang item B: status %q, mau approved", st)
	}

	// Sisa item terakhir dipecah → induk tinggal cangkang dan statusnya
	// mengikuti keluarga: A lunas, B belum → dibayar sebagian.
	split("CV Pecah B", b)
	if st, _ := recvStatus(t, pr.ID); st != "partial" {
		t.Errorf("status induk setelah sisa item terakhir dipecah = %q, mau partial (A lunas, B belum diajukan)", st)
	}
}

// Dokumen yang barangnya sudah diterima tetapi baru dibayar sebagian tetap
// bisa dilunasi (T-12) — ekspor Excel Pembayaran wajib menampilkan sisanya
// sebagai sisa tagihan, sama seperti halaman Pembayaran dan Neraca.
func TestPaymentsExportCountsReceivedRemainder(t *testing.T) {
	requireDB(t)
	name := "Zzz-Uji-Ekspor-Sisa-" + reviewSuffix()
	group := func(final float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: name, Qty: 1, Unit: "kali", HpsPrice: 1000000, FinalPrice: final},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji ekspor", Items: group(0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)
	verifyAndApprove(t, pr.ID)
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: group(1000000), VendorID: strPtr(testVendorID(t))}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	for _, in := range []models.UpdatePurchaseStatusInput{
		{Action: "request_payment", ActorName: "pembelian"},
		{Action: "pay", ActorName: "keuangan", PaymentAmount: 400000},
		{Action: "receive", ActorName: "pengaju"},
	} {
		if _, err := UpdatePurchaseStatus(pr.ID, in); err != nil {
			t.Fatalf("%s: %v", in.Action, err)
		}
	}
	got, err := GetPurchaseRequest(pr.ID)
	if err != nil || got.Status != "received" || got.PaidAmount != 400000 {
		t.Fatalf("keadaan uji: status=%v paid=%v err=%v, mau received / 400000", got.Status, got.PaidAmount, err)
	}

	data, _, err := BuildProcurementPaymentsExcel("", "", "", got.RequestNumber, nil, nil)
	if err != nil {
		t.Fatalf("ekspor: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("baca ekspor: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Daftar Pembayaran")
	if err != nil {
		t.Fatalf("sheet daftar: %v", err)
	}
	found := false
	for _, r := range rows[1:] {
		if len(r) > 11 && r[1] == got.RequestNumber {
			found = true
			if cell := strings.ReplaceAll(strings.ReplaceAll(r[11], ".", ""), ",", ""); !strings.Contains(cell, "600000") {
				t.Errorf("Sisa Tagihan dokumen received yang baru dibayar 400.000 dari 1.000.000 = %q, mau 600000", r[11])
			}
		}
	}
	if !found {
		t.Fatalf("baris %s tidak ada di ekspor (%d baris)", got.RequestNumber, len(rows))
	}
	summary, _ := f.GetRows("Ringkasan")
	ok := false
	for _, r := range summary {
		if len(r) > 1 && strings.HasPrefix(r[0], "Sisa Hutang") {
			ok = strings.Contains(strings.ReplaceAll(strings.ReplaceAll(r[1], ".", ""), ",", ""), fmt.Sprint(600000))
		}
	}
	if !ok {
		t.Errorf("ringkasan Sisa Hutang tidak memuat sisa 600000 dokumen received")
	}
}

// T-43 (sisa): pengajuan pusat (tanpa outlet maupun unit kerja, mis. MRP
// Gudang Induk) hanya boleh dibuka role bercakupan semua outlet, tetapi WA-nya
// dulu dikirim dengan outlet kosong — artinya "semua role", termasuk role per
// outlet yang memegang izin verifikasi/persetujuan namun tidak bisa membuka
// dokumennya. Isi pengajuan pusat bocor ke luar cakupan.
func TestWACentralPurchaseSkipsOutletScopedRoles(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	outlet, unit, admin := NewULID(), NewULID(), NewULID()
	role := "ujiwa_" + sfx
	phone := "0812" + fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
	for _, q := range []struct {
		sql  string
		args []interface{}
	}{
		{`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`, []interface{}{outlet, "WA" + sfx[:6], "Outlet Uji WA " + sfx, "key-wa-" + outlet}},
		{`INSERT INTO work_units (id, name, outlet_id) VALUES ($1, $2, $3)`, []interface{}{unit, "Unit Uji WA " + sfx, outlet}},
		{`INSERT INTO roles (name, description, scope_type) VALUES ($1, 'uji', 'specific')`, []interface{}{role}},
		{`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1, $2)`, []interface{}{role, unit}},
		{`INSERT INTO role_permissions (role, permission) VALUES ($1, 'procurement.requests.verify')`, []interface{}{role}},
		{`INSERT INTO cloud_admins (id, username, password_hash, name, role, wa_phone, wa_notify, is_active)
		  VALUES ($1, $2, 'x', $3, $4, $5, true, true)`, []interface{}{admin, "uji_wa_" + sfx, "Verifikator Outlet " + sfx, role, phone}},
	} {
		if _, err := database.DB.Exec(q.sql, q.args...); err != nil {
			t.Fatalf("data uji: %v", err)
		}
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_admins WHERE id = $1`, admin)
		execScratch(`DELETE FROM role_permissions WHERE role = $1`, role)
		execScratch(`DELETE FROM role_work_unit_scope WHERE role = $1`, role)
		execScratch(`DELETE FROM roles WHERE name = $1`, role)
		execScratch(`DELETE FROM work_units WHERE id = $1`, unit)
		execScratch(`DELETE FROM outlets WHERE id = $1`, outlet)
	})
	receives := func(outletID string) bool {
		for _, r := range waRecipientsForMapped("purchase.submitted", outletID, nil) {
			if strings.TrimSpace(r.ID) == admin {
				return true
			}
		}
		return false
	}
	// Bukti perilaku lama: outlet kosong menjangkau role per outlet.
	if !receives("") {
		t.Fatal("prasyarat uji: outlet kosong seharusnya menjangkau semua role (perilaku lama)")
	}
	central := &models.PurchaseRequest{RequestType: RequestTypeBarang}
	if receives(waPurchaseOutlet(central)) {
		t.Error("role per outlet menerima WA pengajuan pusat yang tidak bisa ia buka (T-43)")
	}
	own := &models.PurchaseRequest{RequestType: RequestTypeBarang, OutletID: &outlet, WorkUnitID: &unit}
	if !receives(waPurchaseOutlet(own)) {
		t.Error("role per outlet tidak lagi menerima WA pengajuan outletnya sendiri")
	}
}
