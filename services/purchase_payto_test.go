package services

// Tujuan pembayaran diisi bagian Pembelian (keputusan owner 19 Nov 2026):
// Keuangan hanya membaca tujuan itu, memilih sumber dana, dan mengembalikan
// detail yang salah ke Pembelian. Rekening transfer tetap selalu rekening
// vendor terdaftar (aturan 14 Okt 2026).

import (
	"errors"
	"strings"
	"testing"

	"cloud-pos/models"
)

func payToOf(t *testing.T, id string) models.PurchasePayTo {
	t.Helper()
	pr, err := GetPurchaseRequest(id)
	if err != nil {
		t.Fatalf("muat %s: %v", id, err)
	}
	return pr.PayTo
}

func versionOf(t *testing.T, id string) string {
	t.Helper()
	pr, err := GetPurchaseRequest(id)
	if err != nil {
		t.Fatalf("muat %s: %v", id, err)
	}
	return pr.Version
}

func wantInvalid(t *testing.T, err error, contains, what string) {
	t.Helper()
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, contains) {
		t.Fatalf("%s: %v, mau ValidationError berisi %q", what, err, contains)
	}
}

func wantConflict(t *testing.T, err error, what string) {
	t.Helper()
	var ce ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("%s: %v, mau ConflictError (409)", what, err)
	}
}

func TestPayToRequestPaymentRules(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	withAcct := snapshotVendor(t, "CV Berekening "+sfx, "BCA", "5550005550", "CV Berekening")
	noAcct := snapshotVendor(t, "Warung Tanpa Rekening "+sfx, "", "", "")

	// Vendor berekening: tunai ditolak (rekening terdaftar wajib dipakai);
	// tanpa memilih metode = rekening terdaftar, catatan ikut tersimpan.
	a := snapshotPriced(t, "Zzz-PayTo-A-"+sfx, 300000, withAcct.ID)
	_, err := UpdatePurchaseStatus(a.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToManual, Text: "Tunai"}})
	wantInvalid(t, err, "punya rekening terdaftar", "tunai untuk vendor berekening")
	if _, err := UpdatePurchaseStatus(a.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "Budi Pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Note: " Bayar sebelum tanggal 25 "}}); err != nil {
		t.Fatalf("ajukan bayar A: %v", err)
	}
	p := payToOf(t, a.ID)
	if p.Method != PayToVendorAccount || p.Effective != PayToVendorAccount || !p.Ready ||
		p.AccountNumber != "5550005550" || p.BankName != "BCA" || p.AccountHolder != "CV Berekening" ||
		p.Note != "Bayar sebelum tanggal 25" || p.SetBy != "Budi Pembelian" || p.SetAt == nil {
		t.Errorf("pay_to A = %+v, mau rekening terdaftar BCA 5550005550 + catatan + pengisi", p)
	}

	// Vendor tanpa rekening: wajib memilih cara bayar; VA diperiksa.
	b := snapshotPriced(t, "Zzz-PayTo-B-"+sfx, 200000, noAcct.ID)
	_, err = UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian"})
	wantInvalid(t, err, "belum punya rekening terdaftar", "tanpa cara bayar")
	_, err = UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToVendorAccount}})
	wantInvalid(t, err, "belum punya rekening terdaftar", "rekening terdaftar untuk vendor tanpa rekening")
	_, err = UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: "Shopee", OrderNumber: "SO-1", BankName: "BCA", AccountNumber: "12ab"}})
	wantInvalid(t, err, "angka", "VA bukan angka")
	_, err = UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: "transfer_lain", Text: "BRI 123 a.n. Siapa Saja"}})
	wantInvalid(t, err, "tidak valid", "metode tak dikenal")
	if _, err := UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: " Shopee ", OrderNumber: " SO-2611-001 ",
			BankName: "BCA", AccountNumber: " 3901234567890 ", AccountHolder: "Shopee"}}); err != nil {
		t.Fatalf("ajukan bayar VA: %v", err)
	}
	p = payToOf(t, b.ID)
	if p.Effective != PayToMarketplaceVA || !p.Ready || p.Marketplace != "Shopee" || p.OrderNumber != "SO-2611-001" ||
		p.AccountNumber != "3901234567890" || p.BankName != "BCA" || p.AccountHolder != "Shopee" {
		t.Errorf("pay_to B = %+v, mau VA Shopee SO-2611-001 BCA 3901234567890", p)
	}
}

func TestPayToReturnAndFixFlow(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	shop := snapshotVendor(t, "Toko Online "+sfx, "", "", "")
	pr := snapshotBill(t, "Zzz-PayTo-Ret-"+sfx, 400000, shop.ID, &models.PurchasePaymentDetailsInput{
		Method: PayToMarketplaceVA, Marketplace: "Tokopedia", OrderNumber: "INV/1", BankName: "Mandiri", AccountNumber: "8800111"})

	// Alasan wajib; versi lama ditolak 409.
	_, err := ReturnPurchasePaymentDetails(pr.ID, models.PurchasePaymentReturnInput{Reason: "x", Version: versionOf(t, pr.ID), ActorName: "Keuangan"})
	wantInvalid(t, err, "alasan", "kembalikan tanpa alasan")
	_, err = ReturnPurchasePaymentDetails(pr.ID, models.PurchasePaymentReturnInput{Reason: "VA sudah kedaluwarsa", Version: "2020-01-01T00:00:00Z", ActorName: "Keuangan"})
	wantConflict(t, err, "kembalikan dengan versi lama")

	if _, err := ReturnPurchasePaymentDetails(pr.ID, models.PurchasePaymentReturnInput{
		Reason: " VA sudah kedaluwarsa ", Version: versionOf(t, pr.ID), ActorName: "Sari Keuangan"}); err != nil {
		t.Fatalf("kembalikan: %v", err)
	}
	p := payToOf(t, pr.ID)
	if p.Ready || p.ReturnedAt == nil || p.ReturnReason != "VA sudah kedaluwarsa" || p.ReturnedBy != "Sari Keuangan" ||
		!strings.Contains(p.Problem, "dikembalikan ke Pembelian") {
		t.Errorf("setelah dikembalikan pay_to = %+v", p)
	}
	_, err = ReturnPurchasePaymentDetails(pr.ID, models.PurchasePaymentReturnInput{Reason: "VA sudah kedaluwarsa", Version: versionOf(t, pr.ID), ActorName: "Keuangan"})
	wantInvalid(t, err, "sudah dikembalikan", "kembalikan dua kali")
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"})
	wantInvalid(t, err, "dikembalikan ke Pembelian", "bayar saat dikembalikan")

	// Pembelian memperbaiki (VA baru): pengembalian terhapus, bisa dibayar.
	_, err = UpdatePurchasePaymentDetails(pr.ID, models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: "Tokopedia",
		OrderNumber: "INV/2", BankName: "Mandiri", AccountNumber: "8800222", ActorName: "pembelian"})
	wantInvalid(t, err, "version wajib", "ubah tanpa versi")
	if _, err := UpdatePurchasePaymentDetails(pr.ID, models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: "Tokopedia",
		OrderNumber: "INV/2", BankName: "Mandiri", AccountNumber: "8800222", Version: versionOf(t, pr.ID), ActorName: "pembelian"}); err != nil {
		t.Fatalf("perbaiki: %v", err)
	}
	p = payToOf(t, pr.ID)
	if !p.Ready || p.ReturnedAt != nil || p.ReturnReason != "" || p.ReturnedBy != "" || p.AccountNumber != "8800222" || p.OrderNumber != "INV/2" {
		t.Errorf("setelah diperbaiki pay_to = %+v", p)
	}

	// Form Bayar yang dibuka sebelum Pembelian mengubah detail → 409; yang
	// tercatat selalu isian Pembelian, bukan ketikan Keuangan.
	opened := versionOf(t, pr.ID)
	if _, err := UpdatePurchasePaymentDetails(pr.ID, models.PurchasePaymentDetailsInput{
		Method: PayToManual, Text: "Tunai saat barang diantar", Version: opened, ActorName: "pembelian"}); err != nil {
		t.Fatalf("ganti ke tunai: %v", err)
	}
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan", Version: opened})
	wantConflict(t, err, "bayar dengan versi sebelum detail diubah")
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan",
		Version: versionOf(t, pr.ID), PaymentMethod: "marketplace_va", PaymentAccountNumber: "123", PaymentAccountDest: "BRI 1 a.n. X"}); err != nil {
		t.Fatalf("bayar: %v", err)
	}
	h, _ := GetPaymentHistories(pr.ID)
	if len(h) != 1 || h[0].PaymentMethod != PayToManual || h[0].PaymentAccountDest != "Tunai saat barang diantar" || !h[0].DestManual {
		t.Errorf("histori = %+v, mau tunai isian Pembelian", h)
	}
	// Lunas → detail tidak bisa diubah atau dikembalikan lagi.
	_, err = UpdatePurchasePaymentDetails(pr.ID, models.PurchasePaymentDetailsInput{Method: PayToManual, Text: "Tunai", Version: versionOf(t, pr.ID)})
	wantInvalid(t, err, "menunggu pembayaran", "ubah setelah lunas")
	_, err = ReturnPurchasePaymentDetails(pr.ID, models.PurchasePaymentReturnInput{Reason: "salah tujuan", Version: versionOf(t, pr.ID)})
	wantInvalid(t, err, "menunggu pembayaran", "kembalikan setelah lunas")
}

func TestPayToOutletCashGuards(t *testing.T) {
	requireDB(t)
	outlet := payoutTestOutlet(t)
	prID := payablePR(t, outlet, 90000) // vendor berekening → rekening terdaftar

	// Selama tugas Kas Kasir Outlet terbuka, detail terkunci.
	po, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"})
	if err != nil {
		t.Fatalf("tetapkan tugas: %v", err)
	}
	_, err = UpdatePurchasePaymentDetails(prID, models.PurchasePaymentDetailsInput{Method: PayToVendorAccount, Version: versionOf(t, prID)})
	wantInvalid(t, err, "Kas Kasir Outlet", "ubah detail saat tugas terbuka")
	_, err = ReturnPurchasePaymentDetails(prID, models.PurchasePaymentReturnInput{Reason: "rekening salah", Version: versionOf(t, prID)})
	wantInvalid(t, err, "Kas Kasir Outlet", "kembalikan saat tugas terbuka")
	if _, err := CancelCashPayout(po.ID, models.CashPayoutResolveInput{Reason: "uji", ActorName: "Keuangan"}); err != nil {
		t.Fatalf("tarik tugas: %v", err)
	}

	// Dikembalikan ke Pembelian → tugas kas tidak bisa ditetapkan.
	if _, err := ReturnPurchasePaymentDetails(prID, models.PurchasePaymentReturnInput{
		Reason: "rekening salah", Version: versionOf(t, prID), ActorName: "Keuangan"}); err != nil {
		t.Fatalf("kembalikan: %v", err)
	}
	_, err = AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"})
	wantInvalid(t, err, "dikembalikan ke Pembelian", "tugas kas saat dikembalikan")

	// VA marketplace → hanya transfer Kantor Pusat.
	if _, err := UpdatePurchasePaymentDetails(prID, models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: "Shopee",
		OrderNumber: "SO-9", BankName: "BNI", AccountNumber: "9881234", Version: versionOf(t, prID), ActorName: "pembelian"}); err != nil {
		t.Fatalf("ganti ke VA: %v", err)
	}
	_, err = AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"})
	wantInvalid(t, err, "Virtual Account Marketplace", "tugas kas untuk VA")

	// Kembali ke rekening terdaftar → tugas kas bisa ditetapkan lagi.
	if _, err := UpdatePurchasePaymentDetails(prID, models.PurchasePaymentDetailsInput{Method: PayToVendorAccount,
		Version: versionOf(t, prID), ActorName: "pembelian"}); err != nil {
		t.Fatalf("kembali ke rekening: %v", err)
	}
	if _, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: prID, ActorName: "Keuangan"}); err != nil {
		t.Fatalf("tetapkan tugas lagi: %v", err)
	}
}

func TestPayToLegacyRequests(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	withAcct := snapshotVendor(t, "CV Lama "+sfx, "BRI", "7770007770", "CV Lama")
	noAcct := snapshotVendor(t, "Pasar Lama "+sfx, "", "", "")
	a := snapshotBill(t, "Zzz-PayTo-LamaA-"+sfx, 120000, withAcct.ID)
	b := snapshotBill(t, "Zzz-PayTo-LamaB-"+sfx, 80000, noAcct.ID, &models.PurchasePaymentDetailsInput{Method: PayToManual, Text: "Tunai"})
	// Tiru pengajuan yang sudah menunggu bayar sebelum rilis: isian kosong.
	execScratch(`UPDATE purchase_requests SET pay_to_method='', pay_to_text='', pay_to_set_by='', pay_to_set_at=NULL WHERE id IN ($1, $2)`, a.ID, b.ID)

	if p := payToOf(t, a.ID); !p.Ready || p.Method != "" || p.Effective != PayToVendorAccount || p.AccountNumber != "7770007770" {
		t.Errorf("data lama vendor berekening: %+v, mau otomatis rekening terdaftar", p)
	}
	if p := payToOf(t, b.ID); p.Ready || !strings.Contains(p.Problem, "belum mengisi detail pembayaran") {
		t.Errorf("data lama vendor tanpa rekening: %+v, mau belum bisa dibayar", p)
	}
	_, err := UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"})
	wantInvalid(t, err, "belum mengisi detail pembayaran", "bayar data lama tanpa rekening")
	if _, err := UpdatePurchaseStatus(a.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"}); err != nil {
		t.Fatalf("bayar data lama berekening: %v", err)
	}
	if h, _ := GetPaymentHistories(a.ID); len(h) != 1 || h[0].DestAccountNumber != "7770007770" || h[0].PaymentMethod != PayToVendorAccount {
		t.Errorf("histori data lama = %+v, mau rekening terdaftar", h)
	}
	if _, err := UpdatePurchasePaymentDetails(b.ID, models.PurchasePaymentDetailsInput{Method: PayToManual, Text: "Tunai di toko",
		Version: versionOf(t, b.ID), ActorName: "pembelian"}); err != nil {
		t.Fatalf("lengkapi data lama: %v", err)
	}
	if _, err := UpdatePurchaseStatus(b.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan"}); err != nil {
		t.Fatalf("bayar setelah dilengkapi: %v", err)
	}
}

// Induk dipecah per vendor: detail di dialog milik induk (vendornya sendiri);
// pecahan bervendor-rekening ikut memakai rekening terdaftarnya, pecahan yang
// butuh VA/tunai diajukan sendiri.
func TestPayToSplitChildren(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	a, b, c := "Zzz-PayTo-SplitA-"+sfx, "Zzz-PayTo-SplitB-"+sfx, "Zzz-PayTo-SplitC-"+sfx
	items := func(fa, fb, fc float64) []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: a, Qty: 1, Unit: "kali", HpsPrice: 100000, FinalPrice: fa},
			{Name: b, Qty: 1, Unit: "kali", HpsPrice: 200000, FinalPrice: fb},
			{Name: c, Qty: 1, Unit: "kali", HpsPrice: 300000, FinalPrice: fc},
		}}}
	}
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "pengaju uji pecahan", Items: items(0, 0, 0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	reviewCleanup(t, pr.ID)
	verifyAndApprove(t, pr.ID)
	vm := snapshotVendor(t, "Marketplace Induk "+sfx, "", "", "")
	vb := snapshotVendor(t, "CV Pecahan B "+sfx, "BNI", "6660006660", "CV Pecahan B")
	vc := snapshotVendor(t, "Pasar Pecahan C "+sfx, "", "", "")
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items(100000, 200000, 300000), VendorID: &vm.ID}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	split := func(v *models.Vendor, item string) string {
		t.Helper()
		m, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
			VendorID: v.ID, VendorName: v.Name,
			Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{{Name: item}}}},
		})
		if err != nil {
			t.Fatalf("split %s: %v", item, err)
		}
		for i := range m.Children {
			if m.Children[i].Items[0].Items[0].Name == item {
				return m.Children[i].ID
			}
		}
		t.Fatalf("pecahan %s tidak ditemukan", item)
		return ""
	}
	cb, cc := split(vb, b), split(vc, c)

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToMarketplaceVA, Marketplace: "Shopee", OrderNumber: "SO-INDUK",
			BankName: "BCA", AccountNumber: "3900001"}}); err != nil {
		t.Fatalf("ajukan bayar induk: %v", err)
	}
	if p := payToOf(t, pr.ID); p.Effective != PayToMarketplaceVA || p.OrderNumber != "SO-INDUK" {
		t.Errorf("pay_to induk = %+v, mau VA induk", p)
	}
	if st, _ := recvStatus(t, cb); st != "payment_requested" {
		t.Errorf("pecahan B (vendor berekening) = %s, mau payment_requested", st)
	}
	if p := payToOf(t, cb); p.Method != PayToVendorAccount || p.AccountNumber != "6660006660" || p.OrderNumber != "" {
		t.Errorf("pay_to pecahan B = %+v, mau rekening terdaftar vendornya (bukan VA induk)", p)
	}
	if st, _ := recvStatus(t, cc); st != "approved" {
		t.Errorf("pecahan C (vendor tanpa rekening) = %s, mau tetap approved", st)
	}
	if _, err := UpdatePurchaseStatus(cc, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "pembelian",
		PayTo: &models.PurchasePaymentDetailsInput{Method: PayToManual, Text: "Tunai di pasar"}}); err != nil {
		t.Fatalf("ajukan bayar pecahan C sendiri: %v", err)
	}
	if p := payToOf(t, cc); p.Effective != PayToManual || p.Text != "Tunai di pasar" || !p.Ready {
		t.Errorf("pay_to pecahan C = %+v", p)
	}
}
