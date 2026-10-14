package services

// Uji perbaikan penerimaan barang, pecah vendor, pembayaran, dan katalog stok
// (temuan T-5, T-12…T-19, T-28, T-29, T-31, T-32, T-34…T-37, T-44, T-45, T-57,
// T-62, T-67, T-80, T-81 — laporan Tinjauan Pengadaan 14 Okt 2026).
// Tes tanpa requireDB berjalan di mana saja; sisanya butuh Postgres uji
// (TestMain + requireDB di purchase_flow_test.go).

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

// ── Tanpa database ──────────────────────────────────────────────────────────

func TestForEachPRLineNormalizesOccurrence(t *testing.T) {
	items := []models.PurchaseRequestItem{{Name: "Perabot", Items: []models.PurchaseSubItem{
		{Name: "Kursi"}, {Name: "kursi "}, {Name: "Meja"}, {Name: "KURSI"},
	}}}
	keys := []string{}
	forEachPRLine(items, func(_ models.PurchaseRequestItem, _ models.PurchaseSubItem, key string) error {
		keys = append(keys, key)
		return nil
	})
	want := []string{"perabot|kursi#0", "perabot|kursi#1", "perabot|meja#0", "perabot|kursi#2"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("kunci = %v, mau %v (nama kembar beda huruf/spasi tetap unik per dokumen, T-29)", keys, want)
	}
	if prItemKey("Perabot", "Kursi", 1) != keys[1] {
		t.Error("prItemKey tidak sama dengan rumus forEachPRLine")
	}
}

func TestReceivingLineIDDistinguishesSources(t *testing.T) {
	if got := receivingLineID("A", "A", "x#0"); got != "x#0" {
		t.Errorf("baris milik dokumen sendiri = %q, mau kunci apa adanya", got)
	}
	if got := receivingLineID("A", "B", "x#0"); got != "B:x#0" {
		t.Errorf("baris pecahan = %q, mau berawalan id pecahan", got)
	}
}

func TestReceiveAccessPerLine(t *testing.T) {
	cases := []struct {
		acc  ReceiveAccess
		kind string
		want bool
	}{
		{ReceiveAccess{Submit: true}, "dapur", true},
		{ReceiveAccess{Submit: true}, "perlengkapan", true},
		{ReceiveAccess{Asset: true}, "perlengkapan", true},
		{ReceiveAccess{Asset: true}, "projek", true},
		{ReceiveAccess{Asset: true}, "dapur", false},
		{ReceiveAccess{Stock: true}, "dapur", true},
		{ReceiveAccess{Stock: true}, "habis", false},
		{ReceiveAccess{}, "dapur", false},
	}
	for _, c := range cases {
		if got := c.acc.canHandle(c.kind); got != c.want {
			t.Errorf("%+v.canHandle(%q) = %v, mau %v", c.acc, c.kind, got, c.want)
		}
	}
	if (ReceiveAccess{}).Any() {
		t.Error("tanpa wewenang apa pun dianggap berwenang")
	}
}

func TestValidateReceivingDestination(t *testing.T) {
	dapur := models.ReceivingLine{Name: "Gula", Kind: "dapur"}
	alat := models.ReceivingLine{Name: "Mesin Kopi", Kind: "perlengkapan"}
	cases := []struct {
		line          models.ReceivingLine
		reqType, proj string
		dest          string
		ok            bool
	}{
		{dapur, RequestTypeBarang, "", "stok", true},
		{dapur, RequestTypeBarang, "", "aset", false},
		{dapur, RequestTypeBarang, "", "habis", false},
		{alat, RequestTypeBarang, "", "aset", true},
		{alat, RequestTypeBarang, "", "habis", true},
		{alat, RequestTypeBarang, "", "stok", false}, // T-80: peralatan tidak pindah ke meja dapur
		{alat, RequestTypeBarang, "", "material", false},
		{alat, RequestTypeBarang, "PRJ", "material", true},
		{alat, RequestTypeBarang, "", "entah", false},
		{alat, RequestTypeBahan, "", "aset", false}, // bahan baku hanya ke stok
		{alat, RequestTypeBahan, "", "stok", true},
	}
	for _, c := range cases {
		err := validateReceivingDestination(c.line, c.reqType, c.proj, c.dest)
		if (err == nil) != c.ok {
			t.Errorf("%s/%s/%s → %q: err=%v, mau ok=%v", c.line.Kind, c.reqType, c.proj, c.dest, err, c.ok)
		}
	}
}

func TestDeriveMasterStatus(t *testing.T) {
	m := func(status string, total, paid float64, receipt string) familyMember {
		return familyMember{Status: status, TotalFinal: total, PaidAmount: paid, ReceiptStatus: receipt}
	}
	own := func(total, paid float64, receipt string) *familyMember {
		f := m("", total, paid, receipt)
		return &f
	}
	cases := []struct {
		name     string
		cur      string
		own      *familyMember
		children []familyMember
		want     string
	}{
		{"induk batal tidak dihidupkan pembayaran pecahan (T-14)", "cancelled", nil,
			[]familyMember{m("paid", 100, 100, "")}, "cancelled"},
		{"pecahan batal tidak ikut dihitung (T-36)", "partial", nil,
			[]familyMember{m("paid", 100, 100, ""), m("cancelled", 50, 0, "")}, "paid"},
		{"induk melunasi sisanya tapi pecahan belum dibayar → partial (T-36)", "paid", own(100, 100, ""),
			[]familyMember{m("payment_requested", 50, 0, "")}, "partial"},
		{"semua pecahan diterima → induk received (T-36)", "paid", nil,
			[]familyMember{m("received", 100, 100, "received"), m("received", 50, 50, "received")}, "received"},
		{"bagian induk belum diajukan tetap approved", "approved", own(100, 0, ""),
			[]familyMember{m("paid", 50, 50, "")}, "approved"},
		{"cangkang dengan pecahan belum diajukan tetap approved (T-16)", "approved", nil,
			[]familyMember{m("payment_requested", 100, 0, ""), m("approved", 0, 0, "")}, "approved"},
		{"semua pecahan di keuangan → payment_requested", "approved", nil,
			[]familyMember{m("payment_requested", 100, 0, ""), m("payment_requested", 50, 0, "")}, "payment_requested"},
		{"bagian induk sudah diajukan, pecahan lain belum berharga", "payment_requested", own(100, 0, ""),
			[]familyMember{m("approved", 0, 0, "")}, "payment_requested"},
		{"induk + pecahan lunas & lengkap → received", "paid", own(100, 100, "received"),
			[]familyMember{m("received", 50, 50, "received")}, "received"},
		{"barang induk lengkap tapi belum dibayar sama sekali → bukan received", "payment_requested", own(100, 0, "received"),
			[]familyMember{m("received", 50, 50, "received")}, "partial"},
		{"cangkang tanpa pecahan hidup dibiarkan", "approved", nil,
			[]familyMember{m("cancelled", 50, 0, "")}, "approved"},
	}
	for _, c := range cases {
		if got := deriveMasterStatus(c.cur, c.own, c.children); got != c.want {
			t.Errorf("%s: %q, mau %q", c.name, got, c.want)
		}
	}
}

func TestAggregateReceiptStatus(t *testing.T) {
	own := &familyMember{ReceiptStatus: "received"}
	if got := aggregateReceiptStatus(own, []familyMember{{Status: "paid", ReceiptStatus: ""}}); got != "partial" {
		t.Errorf("induk lengkap + pecahan belum = %q, mau partial", got)
	}
	if got := aggregateReceiptStatus(nil, []familyMember{{Status: "paid", ReceiptStatus: "received"}, {Status: "cancelled"}}); got != "received" {
		t.Errorf("pecahan batal ikut menahan status penerimaan: %q", got)
	}
	if got := aggregateReceiptStatus(nil, nil); got != "" {
		t.Errorf("tanpa anggota = %q, mau kosong", got)
	}
}

func TestSplitVendorBreakdownSingleMessage(t *testing.T) {
	pr := &models.PurchaseRequest{
		RequestNumber: "1410260001", Status: "payment_requested", VendorName: "CV Induk",
		TotalFinal: 600, TotalAmount: 600,
		Items: []models.PurchaseRequestItem{{Name: "Sisa"}},
		Children: []models.PurchaseRequest{
			{RequestNumber: "1410260002", VendorName: "CV A", Status: "payment_requested", TotalFinal: 300, TotalAmount: 300},
			{RequestNumber: "1410260003", VendorName: "CV Batal", Status: "cancelled", TotalFinal: 200, TotalAmount: 200},
		},
	}
	vendor, total := waSplitVendorBreakdown(pr)
	if total != 400 {
		t.Errorf("total = %v, mau 400 (sisa induk 100 + CV A 300; pecahan batal tidak dijumlahkan)", total)
	}
	if !strings.Contains(vendor, "CV A (No. 1410260002)") || strings.Contains(vendor, "CV Batal") || !strings.Contains(vendor, "sisa induk") {
		t.Errorf("rincian vendor tidak sesuai:\n%s", vendor)
	}
	vars := waPurchaseVars(pr)
	if vars["vendor"] != vendor {
		t.Error("pesan induk tidak memakai rincian per vendor (T-62)")
	}
}

func TestValidateReceivableItemText(t *testing.T) {
	long := strings.Repeat("a", maxGoodsItemNameLen+1)
	items := []models.PurchaseRequestItem{{Name: "Grup", Items: []models.PurchaseSubItem{{Name: long, Qty: 1, Unit: "pcs"}}}}
	if err := validateReceivableItemText(items, RequestTypeBarang); err == nil {
		t.Error("nama barang lebih dari 150 huruf diterima — serah terimanya kelak gagal (T-80)")
	}
	if err := validateReceivableItemText(items, RequestTypeJasa); err != nil {
		t.Errorf("jasa tidak perlu batas nama barang: %v", err)
	}
	items[0].Items[0] = models.PurchaseSubItem{Name: "Kursi", Unit: strings.Repeat("u", maxGoodsUnitLen+1)}
	if err := validateReceivableItemText(items, RequestTypeBarang); err == nil {
		t.Error("satuan lebih dari 20 huruf diterima")
	}
}

// ── Dengan database ─────────────────────────────────────────────────────────

var recvActor = Actor{Name: "Petugas Uji Terima"}

var recvAll = ReceiveAccess{Submit: true, Asset: true, Stock: true}

const recvPhoto = "/uploads/uji-terima.jpg"

func recvSuffix() string { return strings.ToLower(NewULID()[14:]) }

// recvCleanupFamily menghapus pengajuan beserta pecahan dan semua catatan
// penerimaannya.
func recvCleanupFamily(id string) {
	fam := `(SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)`
	execScratch(`DELETE FROM pr_receiving_decisions WHERE purchase_request_id IN `+fam, id)
	execScratch(`DELETE FROM handover_photos WHERE ref_id IN `+fam, id)
	execScratch(`DELETE FROM asset_movements WHERE asset_id IN (SELECT id FROM assets WHERE purchase_request_id IN `+fam+`)`, id)
	execScratch(`DELETE FROM assets WHERE purchase_request_id IN `+fam, id)
	execScratch(`DELETE FROM goods_receipts WHERE purchase_request_id IN `+fam, id)
	execScratch(`DELETE FROM project_materials WHERE purchase_request_id IN `+fam, id)
	execScratch(`DELETE FROM purchase_requests WHERE parent_id = $1`, id)
	execScratch(`DELETE FROM purchase_requests WHERE id = $1`, id)
}

// recvApprovedPR membuat pengajuan, meloloskannya sampai approved, dan
// mendaftarkan pembersihannya.
func recvApprovedPR(t *testing.T, reqType string, items []models.PurchaseRequestItem) *models.PurchaseRequest {
	t.Helper()
	pr, err := createPricedPR(models.CreatePurchaseRequestInput{
		RequestType: reqType, RequestedBy: "pengaju uji", VendorName: "CV Uji Terima", Items: items,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { recvCleanupFamily(pr.ID) })
	verifyAndApprove(t, pr.ID)
	got, err := GetPurchaseRequest(pr.ID)
	if err != nil {
		t.Fatalf("muat: %v", err)
	}
	return got
}

func recvOutlet(t *testing.T) string {
	t.Helper()
	id := NewULID()
	sfx := recvSuffix()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		id, "RT"+sfx[:8], "Outlet Uji Terima "+sfx, "key-terima-"+id); err != nil {
		t.Fatalf("outlet uji: %v", err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM outlets WHERE id = $1`, id) })
	return id
}

func recvStockItem(t *testing.T, name string) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio)
		VALUES ($1, $2, $3, 'gr', 'kg', 1000)`, id, "UJT"+strings.ToUpper(recvSuffix()), name); err != nil {
		t.Fatalf("katalog uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM goods_receipt_items WHERE item_id = $1`, id)
		execScratch(`DELETE FROM stock_batches WHERE item_id = $1`, id)
		execScratch(`DELETE FROM stock_movements WHERE item_id = $1`, id)
		execScratch(`DELETE FROM stock_ledger WHERE item_id = $1`, id)
		execScratch(`DELETE FROM stock_items WHERE id = $1`, id)
	})
	return id
}

func recvWarehouse(t *testing.T) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type) VALUES ($1, $2, 'Gudang Uji Terima', 'central')`,
		id, "WT"+strings.ToUpper(recvSuffix())); err != nil {
		t.Fatalf("gudang uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM goods_receipts WHERE warehouse_id = $1`, id)
		execScratch(`DELETE FROM stock_batches WHERE warehouse_id = $1`, id)
		execScratch(`DELETE FROM stock_movements WHERE warehouse_id = $1`, id)
		execScratch(`DELETE FROM stock_ledger WHERE warehouse_id = $1`, id)
		execScratch(`DELETE FROM warehouses WHERE id = $1`, id)
	})
	return id
}

func recvDraft(t *testing.T, id string) *models.ReceivingDraft {
	t.Helper()
	d, err := BuildReceivingDraft(id, nil)
	if err != nil {
		t.Fatalf("draft %s: %v", id, err)
	}
	return d
}

// recvLine mencari baris draf menurut nama (dan dokumen sumber bila diisi).
func recvLine(t *testing.T, d *models.ReceivingDraft, name, source string) models.ReceivingLine {
	t.Helper()
	for _, l := range d.Lines {
		if l.Name == name && (source == "" || l.SourcePRID == source) {
			return l
		}
	}
	t.Fatalf("baris %q (sumber %q) tidak ada di draf %s", name, source, d.RequestNumber)
	return models.ReceivingLine{}
}

func recvStatus(t *testing.T, id string) (status, receipt string) {
	t.Helper()
	if err := database.DB.QueryRow(`SELECT status, COALESCE(receipt_status, '') FROM purchase_requests WHERE id = $1`, id).
		Scan(&status, &receipt); err != nil {
		t.Fatalf("status %s: %v", id, err)
	}
	return
}

func recvSplit(t *testing.T, parentID, vendor, group string, names ...string) *models.PurchaseRequest {
	t.Helper()
	subs := []models.PurchaseSubItem{}
	for _, n := range names {
		subs = append(subs, models.PurchaseSubItem{Name: n})
	}
	master, err := SplitPurchaseRequest(parentID, models.SplitPurchaseRequestInput{
		VendorID: testVendorNamed(t, vendor), VendorName: vendor, Items: []models.PurchaseRequestItem{{Name: group, Items: subs}},
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	for i := range master.Children {
		if master.Children[i].VendorName == vendor {
			return &master.Children[i]
		}
	}
	t.Fatalf("pecahan %s tidak ditemukan", vendor)
	return nil
}

func mustStatus(t *testing.T, id, action string, amount float64) {
	t.Helper()
	if _, err := UpdatePurchaseStatus(id, models.UpdatePurchaseStatusInput{
		Action: action, ActorName: "uji", PaymentAmount: amount, RejectedReason: "alasan uji",
	}); err != nil {
		t.Fatalf("%s %s: %v", action, id, err)
	}
}

// T-5: baris pecahan yang diterima lewat dokumen induk tercatat pada
// pecahannya, jadi tidak bisa diterima lagi lewat jalur mana pun.
func TestReceiveSplitChildViaMasterCountsOnce(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	gula, garam := "Gula Uji Terima "+sfx, "Garam Uji Terima "+sfx
	stockID := recvStockItem(t, gula)
	wh := recvWarehouse(t)
	pr := recvApprovedPR(t, RequestTypeBahan, []models.PurchaseRequestItem{{Name: "Dapur", Items: []models.PurchaseSubItem{
		{Name: gula, Qty: 5, Unit: "kg", HpsPrice: 14000, FinalPrice: 14000},
		{Name: garam, Qty: 2, Unit: "kg", HpsPrice: 9000, FinalPrice: 9000},
	}}})
	child := recvSplit(t, pr.ID, "CV Garam", "Dapur", garam)

	d := recvDraft(t, pr.ID)
	line := recvLine(t, d, garam, child.ID)
	if line.PRItemKey == line.ItemKey {
		t.Errorf("baris pecahan lewat induk berkunci %q — mau berawalan id pecahan", line.PRItemKey)
	}
	res, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: line.PRItemKey, Destination: "stok", Qty: 2, StockItemID: stockID, WarehouseID: wh,
	}}}, recvActor, recvAll, nil)
	if err != nil {
		t.Fatalf("terima lewat induk: %v", err)
	}
	if len(res.GRNNumbers) != 1 {
		t.Fatalf("GRN terbit %v, mau 1", res.GRNNumbers)
	}
	var grnPR string
	database.DB.QueryRow(`SELECT purchase_request_id FROM goods_receipts WHERE grn_number = $1`, res.GRNNumbers[0]).Scan(&grnPR)
	if strings.TrimSpace(grnPR) != child.ID {
		t.Errorf("GRN ditautkan ke %q, mau pecahannya %q (T-5)", grnPR, child.ID)
	}
	// Dari jalur pecahan maupun induk, baris itu sudah habis.
	if l := recvLine(t, recvDraft(t, child.ID), garam, ""); l.Remaining != 0 || l.Recorded != 2 {
		t.Errorf("draf pecahan: recorded=%d remaining=%d, mau 2/0", l.Recorded, l.Remaining)
	}
	if l := recvLine(t, recvDraft(t, pr.ID), garam, child.ID); l.Remaining != 0 {
		t.Errorf("draf induk: remaining=%d, mau 0", l.Remaining)
	}
	for _, via := range []string{pr.ID, child.ID} {
		l := recvLine(t, recvDraft(t, via), garam, child.ID)
		if _, err := ReceiveGoods(via, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
			PRItemKey: l.PRItemKey, Destination: "stok", Qty: 2, StockItemID: stockID, WarehouseID: wh,
		}}}, recvActor, recvAll, nil); err == nil {
			t.Errorf("stok garam bisa dicatat lagi lewat %s (T-5)", via)
		}
	}
	if _, r := recvStatus(t, child.ID); r != "received" {
		t.Errorf("receipt_status pecahan = %q, mau received", r)
	}
	if _, r := recvStatus(t, pr.ID); r != "partial" {
		t.Errorf("receipt_status induk = %q, mau partial (gula belum diterima)", r)
	}
}

// T-29: baris bernama kembar di induk & pecahan tidak saling menimpa, dan
// satu baris tidak boleh dikirim dua kali dalam satu kiriman.
func TestReceiveTwinNamesAcrossMasterAndChild(t *testing.T) {
	requireDB(t)
	name := "Kursi Uji Kembar " + recvSuffix()
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Perabot", Items: []models.PurchaseSubItem{
		{Name: name, Qty: 2, Unit: "pcs", HpsPrice: 100000, FinalPrice: 100000},
		{Name: name, Qty: 3, Unit: "pcs", HpsPrice: 100000, FinalPrice: 100000},
	}}})
	child := recvSplit(t, pr.ID, "CV Kursi", "Perabot", name)

	d := recvDraft(t, pr.ID)
	own := recvLine(t, d, name, pr.ID)
	ch := recvLine(t, d, name, child.ID)
	if own.ItemKey != ch.ItemKey {
		t.Fatalf("skenario uji tidak membentuk kunci kembar: %q vs %q", own.ItemKey, ch.ItemKey)
	}
	if own.PRItemKey == ch.PRItemKey {
		t.Fatalf("identitas baris draf bertabrakan: %q", own.PRItemKey)
	}
	dup := models.ReceiveGoodsLine{PRItemKey: own.PRItemKey, Destination: "habis", Qty: 1, Reason: "dipakai langsung di ruang tamu"}
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{dup, dup}},
		recvActor, recvAll, nil); err == nil {
		t.Error("baris yang sama dua kali dalam satu kiriman diterima (T-29)")
	}
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: own.PRItemKey, Destination: "habis", Qty: 3, Reason: "dipakai langsung di ruang tamu",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima sisa induk: %v", err)
	}
	if l := recvLine(t, recvDraft(t, pr.ID), name, child.ID); l.Remaining != 2 {
		t.Errorf("baris pecahan ikut tertutup: remaining=%d, mau 2", l.Remaining)
	}
	if _, r := recvStatus(t, child.ID); r != "" {
		t.Errorf("receipt_status pecahan = %q, mau kosong (barangnya belum datang)", r)
	}
	if _, r := recvStatus(t, pr.ID); r != "partial" {
		t.Errorf("receipt_status induk = %q, mau partial", r)
	}
	// Pasangan (source_pr_id, item_key) juga diterima.
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		SourcePRID: child.ID, PRItemKey: ch.ItemKey, Destination: "habis", Qty: 2, Reason: "dipakai langsung di ruang tamu",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima baris pecahan lewat pasangan sumber: %v", err)
	}
	if _, r := recvStatus(t, pr.ID); r != "received" {
		t.Errorf("receipt_status induk setelah semua diterima = %q, mau received", r)
	}
}

// T-18: aset bertahap — massal maupun tunggal qty 1 — tidak bertabrakan
// dengan indeks unik, dan kunci berakhiran nomor tidak salah hitung (T-80).
func TestStagedAssetReceiptNoKeyClash(t *testing.T) {
	requireDB(t)
	outlet := recvOutlet(t)
	sfx := recvSuffix()
	meja, lemari := "Meja Uji Bertahap "+sfx, "Lemari Uji Tunggal "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Perabot", Items: []models.PurchaseSubItem{
		{Name: meja, Qty: 5, Unit: "unit", HpsPrice: 600000, FinalPrice: 600000},
		{Name: lemari, Qty: 2, Unit: "unit", HpsPrice: 6000000, FinalPrice: 6000000},
	}}})
	recv := func(name, mode string, qty int) error {
		l := recvLine(t, recvDraft(t, pr.ID), name, "")
		_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, OutletID: outlet, Lines: []models.ReceiveGoodsLine{{
			PRItemKey: l.PRItemKey, Destination: "aset", Qty: qty, TrackingMode: mode,
		}}}, recvActor, recvAll, nil)
		return err
	}
	for _, step := range []struct {
		name, mode string
		qty        int
	}{{meja, "massal", 3}, {meja, "massal", 2}, {lemari, "tunggal", 1}, {lemari, "tunggal", 1}} {
		if err := recv(step.name, step.mode, step.qty); err != nil {
			t.Fatalf("terima %s %s %d: %v (T-18)", step.name, step.mode, step.qty, err)
		}
	}
	d := recvDraft(t, pr.ID)
	for _, n := range []string{meja, lemari} {
		if l := recvLine(t, d, n, ""); l.Remaining != 0 {
			t.Errorf("%s: remaining=%d, mau 0", n, l.Remaining)
		}
	}
	var keys []string
	rows, _ := database.DB.Query(`SELECT pr_item_key FROM assets WHERE purchase_request_id = $1 ORDER BY pr_item_key`, pr.ID)
	for rows.Next() {
		var k string
		rows.Scan(&k)
		keys = append(keys, k)
	}
	rows.Close()
	if len(keys) != 4 {
		t.Fatalf("baris aset = %v, mau 4", keys)
	}
	for _, k := range keys {
		if !strings.Contains(k, "#0/") {
			t.Errorf("kunci aset %q tanpa sufiks nomor", k)
		}
	}
	if _, r := recvStatus(t, pr.ID); r != "received" {
		t.Errorf("receipt_status = %q, mau received", r)
	}
}

// T-80: kunci "#1" tidak ikut menghitung aset baris "#10".
func TestRecordedQtyExactSuffix(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Uji", Items: []models.PurchaseSubItem{
		{Name: "Zzz Kunci " + recvSuffix(), Qty: 1, Unit: "pcs", HpsPrice: 1000},
	}}})
	outlet := recvOutlet(t)
	for _, k := range []string{"uji|x#1/1", "uji|x#10/1", "uji|x#1/abc"} {
		if _, err := database.DB.Exec(`INSERT INTO assets (id, outlet_id, name, quantity, purchase_request_id, pr_item_key)
			VALUES ($1, $2, 'Aset Uji Kunci', 1, $3, $4)`, NewULID(), outlet, pr.ID, k); err != nil {
			t.Fatalf("aset uji: %v", err)
		}
	}
	got, err := recordedQty(database.DB, pr.ID, "uji|x#1")
	if err != nil || got != 1 {
		t.Errorf("recordedQty(#1) = %d (%v), mau 1 — aset #10 dan sufiks bukan angka tidak dihitung", got, err)
	}
}

// T-17/T-10/T-80: wewenang dan tujuan diperiksa per baris.
func TestReceiveLineAuthorityAndDestination(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	alat := "Mesin Kopi Uji " + sfx
	barang := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: alat, Qty: 1, Unit: "unit", HpsPrice: 300000, FinalPrice: 300000},
	}}})
	bahan := recvApprovedPR(t, RequestTypeBahan, []models.PurchaseRequestItem{{Name: "Dapur", Items: []models.PurchaseSubItem{
		{Name: "Tepung Uji " + sfx, Qty: 1, Unit: "kg", HpsPrice: 9000},
	}}})
	alatLine := recvLine(t, recvDraft(t, barang.ID), alat, "")
	if alatLine.Kind == "dapur" {
		t.Fatalf("baris peralatan terbaca dapur")
	}
	try := func(id string, l models.ReceiveGoodsLine, acc ReceiveAccess) error {
		_, err := ReceiveGoods(id, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{l}}, recvActor, acc, nil)
		return err
	}
	ok := "langsung dipakai di bar depan"
	if err := try(barang.ID, models.ReceiveGoodsLine{PRItemKey: alatLine.PRItemKey, Destination: "habis", Qty: 1, Reason: ok},
		ReceiveAccess{Stock: true}); err == nil || !strings.Contains(err.Error(), "bagian Aset") {
		t.Errorf("petugas gudang mencatat baris peralatan: %v", err)
	}
	if err := try(barang.ID, models.ReceiveGoodsLine{PRItemKey: alatLine.PRItemKey, Destination: "stok", Qty: 1},
		ReceiveAccess{Submit: true}); err == nil || !strings.Contains(err.Error(), "stok gudang") {
		t.Errorf("peralatan diterima sebagai stok (T-80): %v", err)
	}
	if err := try(barang.ID, models.ReceiveGoodsLine{PRItemKey: alatLine.PRItemKey, Destination: "habis", Qty: 1, Reason: "rusak"},
		ReceiveAccess{Asset: true}); err == nil || !strings.Contains(err.Error(), "minimal 10 huruf") {
		t.Errorf("habis pakai beralasan pendek diterima (T-17): %v", err)
	}
	if err := try(barang.ID, models.ReceiveGoodsLine{PRItemKey: alatLine.PRItemKey, Destination: "habis", Qty: 1, Reason: ok},
		ReceiveAccess{}); err == nil {
		t.Error("tanpa wewenang apa pun bisa menerima barang")
	}
	tepung := recvDraft(t, bahan.ID).Lines[0]
	if err := try(bahan.ID, models.ReceiveGoodsLine{PRItemKey: tepung.PRItemKey, Destination: "stok", Qty: 1},
		ReceiveAccess{Asset: true}); err == nil || !strings.Contains(err.Error(), "petugas gudang") {
		t.Errorf("bagian Aset mencatat baris dapur: %v", err)
	}
	// Bagian Aset (tanpa izin pengadaan) boleh mencatat baris peralatan.
	if err := try(barang.ID, models.ReceiveGoodsLine{PRItemKey: alatLine.PRItemKey, Destination: "habis", Qty: 1, Reason: ok},
		ReceiveAccess{Asset: true}); err != nil {
		t.Errorf("bagian Aset ditolak mencatat peralatan: %v", err)
	}
	var by string
	database.DB.QueryRow(`SELECT COALESCE(received_by, '') FROM purchase_requests WHERE id = $1`, barang.ID).Scan(&by)
	if by != recvActor.Name {
		t.Errorf("received_by = %q, mau pelaku sesi %q", by, recvActor.Name)
	}
}

// T-45: kiriman tanpa barang ditolak dan tidak mengunci dokumen.
func TestReceiveEmptySubmissionRejected(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: "Rak Uji Kosong " + recvSuffix(), Qty: 2, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
	}}})
	l := recvDraft(t, pr.ID).Lines[0]
	_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "habis", Qty: 0,
	}}}, recvActor, recvAll, nil)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("kiriman tanpa barang: err=%v, mau ValidationError", err)
	}
	if _, r := recvStatus(t, pr.ID); r != "" {
		t.Errorf("receipt_status = %q, mau tetap kosong", r)
	}
	if purchaseHasReceipts(database.DB, pr.ID) {
		t.Error("dokumen terkunci padahal tidak ada barang diterima")
	}
}

// T-28: dua penerimaan bersamaan atas sisa yang sama hanya satu yang tercatat.
func TestConcurrentReceiveRecordsOnce(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: "Kipas Uji Serentak " + recvSuffix(), Qty: 3, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
	}}})
	l := recvDraft(t, pr.ID).Lines[0]
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
				PRItemKey: l.PRItemKey, Destination: "habis", Qty: 3, Reason: "dibagikan langsung ke dapur",
			}}}, recvActor, recvAll, nil)
		}(i)
	}
	close(start)
	wg.Wait()
	okCount := 0
	for _, e := range errs {
		if e == nil {
			okCount++
		}
	}
	got, err := recordedQty(database.DB, pr.ID, l.ItemKey)
	if okCount != 1 || err != nil || got != 3 {
		t.Errorf("sukses=%d tercatat=%d (%v); mau tepat 1 sukses dan 3 unit (T-28)", okCount, got, err)
	}
}

// T-12/T-81: dibayar sebagian lalu diterima → sisa tetap bisa dilunasi.
func TestPayRemainderAfterReceived(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: "Blender Uji Tempo " + recvSuffix(), Qty: 1, Unit: "unit", HpsPrice: 200000, FinalPrice: 200000},
	}}})
	mustStatus(t, pr.ID, "request_payment", 0)
	mustStatus(t, pr.ID, "pay", 80000)
	l := recvDraft(t, pr.ID).Lines[0]
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "habis", Qty: 1, Reason: "langsung dipakai di dapur",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima: %v", err)
	}
	if s, _ := recvStatus(t, pr.ID); s != "received" {
		t.Fatalf("status = %q, mau received", s)
	}
	pay := func(amount float64) error {
		_, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "keuangan", PaymentAmount: amount})
		return err
	}
	if err := pay(-5); err == nil {
		t.Error("nilai bayar negatif diterima (T-81)")
	}
	if err := pay(150000); err == nil {
		t.Error("bayar melebihi sisa diterima")
	}
	if err := pay(120000); err != nil {
		t.Fatalf("pelunasan sisa dari received ditolak (T-12): %v", err)
	}
	got, _ := GetPurchaseRequest(pr.ID)
	if got.Status != "received" || got.PaidAmount != 200000 {
		t.Errorf("status=%q terbayar=%v; mau received / 200000", got.Status, got.PaidAmount)
	}
	if err := pay(0); err == nil {
		t.Error("dokumen lunas masih bisa dibayar")
	}
}

// T-13: barang tempo diterima sebelum harga final → harga tetap bisa diisi
// (struktur terkunci), nilai aset ikut disesuaikan, lalu bisa diajukan bayar.
func TestTempoPriceAfterReceiptThenPay(t *testing.T) {
	requireDB(t)
	outlet := recvOutlet(t)
	name := "Freezer Uji Tempo " + recvSuffix()
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: name, Qty: 1, Unit: "unit", HpsPrice: 700000},
	}}})
	l := recvDraft(t, pr.ID).Lines[0]
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, OutletID: outlet, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "aset", Qty: 1, UnitPrice: 99000000, // harga klien diabaikan (T-38)
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima: %v", err)
	}
	assetPrice := func() float64 {
		var p float64
		database.DB.QueryRow(`SELECT purchase_price FROM assets WHERE purchase_request_id = $1`, pr.ID).Scan(&p)
		return p
	}
	if p := assetPrice(); p != 700000 {
		t.Fatalf("nilai aset = %v, mau HPS 700000 (harga kiriman klien diabaikan)", p)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err == nil {
		t.Error("ajukan bayar tanpa harga final diterima")
	}
	cur, _ := GetPurchaseRequest(pr.ID)
	priced := clonePurchaseItems(cur.Items)
	priced[0].Items[0].FinalPrice = 750000
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: priced, VendorID: strPtr(testVendorID(t))}); err != nil {
		t.Fatalf("isi harga final setelah barang diterima ditolak (T-13): %v", err)
	}
	if p := assetPrice(); p != 750000 {
		t.Errorf("nilai aset setelah harga final = %v, mau 750000", p)
	}
	bad := clonePurchaseItems(priced)
	bad[0].Items[0].Qty = 2
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: bad}); err == nil {
		t.Error("qty bisa diubah setelah barang diterima")
	}
	mustStatus(t, pr.ID, "request_payment", 0)
	mustStatus(t, pr.ID, "pay", 0)
	if s, _ := recvStatus(t, pr.ID); s != "received" {
		t.Errorf("status setelah lunas = %q, mau received (barang sudah diterima)", s)
	}
}

// T-14: induk tidak bisa dibatalkan selama ada pecahan di keuangan, dan
// pembayaran pecahan tidak menghidupkan induk yang batal.
func TestCancelMasterBlockedByChildInFinance(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b := "Kompor Uji A "+sfx, "Kompor Uji B "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
		{Name: b, Qty: 1, Unit: "unit", HpsPrice: 200000, FinalPrice: 200000},
	}}})
	child := recvSplit(t, pr.ID, "CV Kompor B", "Alat", b)
	mustStatus(t, child.ID, "request_payment", 0)

	_, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "cancel", ActorName: "pengaju"})
	if err == nil || !strings.Contains(err.Error(), child.RequestNumber) {
		t.Fatalf("batal induk dengan pecahan di keuangan: err=%v, mau ditolak menyebut %s (T-14)", err, child.RequestNumber)
	}
	if s, _ := recvStatus(t, child.ID); s != "payment_requested" {
		t.Errorf("status pecahan berubah jadi %q", s)
	}
	// Data lama: induk sudah batal sementara pecahannya di keuangan.
	execScratch(`UPDATE purchase_requests SET status = 'cancelled' WHERE id = $1`, pr.ID)
	mustStatus(t, child.ID, "pay", 0)
	if s, _ := recvStatus(t, pr.ID); s != "cancelled" {
		t.Errorf("pembayaran pecahan menghidupkan induk batal menjadi %q (T-14)", s)
	}
}

// T-37/T-15/T-36: semua item wajib berharga sebelum ditagihkan, dan status
// induk dihitung dari keluarganya.
func TestRequestPaymentNeedsPricesAndMasterFollowsFamily(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b := "Rak Uji Harga A "+sfx, "Rak Uji Harga B "+sfx
	single := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
		{Name: b, Qty: 1, Unit: "unit", HpsPrice: 50000},
	}}})
	_, err := UpdatePurchaseStatus(single.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"})
	if err == nil || !strings.Contains(err.Error(), b) {
		t.Errorf("ajukan bayar dengan item tanpa harga: err=%v, mau ditolak menyebut %q (T-37)", err, b)
	}

	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
		{Name: b, Qty: 1, Unit: "unit", HpsPrice: 50000},
	}}})
	child := recvSplit(t, pr.ID, "CV Rak A", "Alat", a)
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err == nil ||
		!strings.Contains(err.Error(), "sisa item induk") {
		t.Errorf("induk bersisa item tanpa harga lolos diajukan: %v (T-15)", err)
	}
	if s, _ := recvStatus(t, child.ID); s != "approved" {
		t.Errorf("penolakan induk tetap memindahkan pecahan ke %q", s)
	}
	cur, _ := GetPurchaseRequest(pr.ID)
	own := clonePurchaseItems(cur.Items)
	own[0].Items[0].FinalPrice = 60000
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: own, VendorID: strPtr(testVendorID(t))}); err != nil {
		t.Fatalf("isi harga sisa induk: %v", err)
	}
	mustStatus(t, pr.ID, "request_payment", 0)
	if s, _ := recvStatus(t, child.ID); s != "payment_requested" {
		t.Errorf("pecahan berharga tidak ikut diajukan: %q", s)
	}
	mustStatus(t, pr.ID, "pay", 0)
	if s, _ := recvStatus(t, pr.ID); s != "partial" {
		t.Errorf("induk lunas bagiannya sendiri tapi pecahan belum dibayar: status %q, mau partial (T-36)", s)
	}
	mustStatus(t, child.ID, "pay", 0)
	if s, _ := recvStatus(t, pr.ID); s != "paid" {
		t.Errorf("seluruh keluarga lunas: status induk %q, mau paid", s)
	}
	d := recvDraft(t, pr.ID)
	lines := []models.ReceiveGoodsLine{}
	for _, l := range d.Lines {
		lines = append(lines, models.ReceiveGoodsLine{PRItemKey: l.PRItemKey, Destination: "habis", Qty: l.Remaining, Reason: "langsung dipakai di gudang"})
	}
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: lines}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima semua: %v", err)
	}
	for _, id := range []string{pr.ID, child.ID} {
		if s, r := recvStatus(t, id); s != "received" || r != "received" {
			t.Errorf("%s: status=%q receipt=%q, mau received/received (T-36)", id, s, r)
		}
	}
}

// T-16: pecahan tanpa harga dilewati saat induk diajukan, lalu diajukan
// sendiri setelah berharga — induk mengikuti keluarganya.
func TestUnpricedChildRequestedLater(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b := "Lampu Uji A "+sfx, "Lampu Uji B "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "unit", HpsPrice: 100000, FinalPrice: 100000},
		{Name: b, Qty: 1, Unit: "unit", HpsPrice: 50000},
	}}})
	ca := recvSplit(t, pr.ID, "CV Lampu A", "Alat", a)
	cb := recvSplit(t, pr.ID, "CV Lampu B", "Alat", b)
	mustStatus(t, pr.ID, "request_payment", 0)
	if s, _ := recvStatus(t, ca.ID); s != "payment_requested" {
		t.Errorf("pecahan berharga: %q, mau payment_requested", s)
	}
	if s, _ := recvStatus(t, cb.ID); s != "approved" {
		t.Errorf("pecahan tanpa harga: %q, mau tetap approved", s)
	}
	if s, _ := recvStatus(t, pr.ID); s != "approved" {
		t.Errorf("induk cangkang: %q, mau approved selama ada pecahan belum diajukan", s)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"}); err == nil {
		t.Error("ajukan ulang induk tanpa pecahan siap diterima diam-diam")
	}
	cur, _ := GetPurchaseRequest(cb.ID)
	priced := clonePurchaseItems(cur.Items)
	priced[0].Items[0].FinalPrice = 55000
	if _, err := UpdatePurchaseItems(cb.ID, models.UpdatePurchaseItemsInput{Items: priced}); err != nil {
		t.Fatalf("isi harga pecahan: %v", err)
	}
	mustStatus(t, cb.ID, "request_payment", 0)
	if s, _ := recvStatus(t, pr.ID); s != "payment_requested" {
		t.Errorf("induk setelah semua pecahan diajukan: %q, mau payment_requested", s)
	}
}

// T-31: pecah vendor setelah ada penerimaan ditolak.
func TestSplitAfterReceiptRejected(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b := "Gelas Uji A "+sfx, "Gelas Uji B "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 2, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
		{Name: b, Qty: 2, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	l := recvLine(t, recvDraft(t, pr.ID), a, "")
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "habis", Qty: 2, Reason: "langsung dipakai di bar",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima: %v", err)
	}
	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Gelas", Items: []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{{Name: a}}}},
	}); err == nil {
		t.Error("barang yang sudah diterima bisa dipecah ke vendor lain (T-31)")
	}
}

// T-32/T-34: induk & pecahan dihapus bersama, dan pecahan tidak bisa dihapus
// bila induknya sudah diproses keuangan.
func TestDeleteSplitFamilyRules(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b := "Piring Uji A "+sfx, "Piring Uji B "+sfx
	items := func() []models.PurchaseRequestItem {
		return []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
			{Name: a, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
			{Name: b, Qty: 1, Unit: "pcs", HpsPrice: 20000, FinalPrice: 20000},
		}}}
	}
	// Induk batal dengan pecahan yang (data lama) masih hidup → hapus ditolak.
	pr := recvApprovedPR(t, RequestTypeBarang, items())
	child := recvSplit(t, pr.ID, "CV Piring", "Alat", b)
	execScratch(`UPDATE purchase_requests SET status = 'cancelled' WHERE id = $1`, pr.ID)
	if err := DeletePurchaseRequest(pr.ID, false); err == nil || !strings.Contains(err.Error(), child.RequestNumber) {
		t.Errorf("hapus induk dengan pecahan hidup: err=%v, mau ditolak menyebut %s (T-32)", err, child.RequestNumber)
	}
	// Batal lewat alur (cascade) lalu hapus → pecahan ikut terhapus, tidak yatim.
	pr2 := recvApprovedPR(t, RequestTypeBarang, items())
	child2 := recvSplit(t, pr2.ID, "CV Piring", "Alat", b)
	mustStatus(t, pr2.ID, "cancel", 0)
	if err := DeletePurchaseRequest(pr2.ID, false); err != nil {
		t.Fatalf("hapus induk batal: %v", err)
	}
	var left int
	database.DB.QueryRow(`SELECT COUNT(*) FROM purchase_requests WHERE id = $1`, child2.ID).Scan(&left)
	if left != 0 {
		t.Error("pecahan yatim tertinggal setelah induk dihapus (T-32)")
	}
	// Induk sudah lunas → pecahan batal tidak bisa dihapus (item kembali ke
	// dokumen yang tidak bisa dibayar lagi).
	pr3 := recvApprovedPR(t, RequestTypeBarang, items())
	c3a := recvSplit(t, pr3.ID, "CV Piring A", "Alat", a)
	c3b := recvSplit(t, pr3.ID, "CV Piring B", "Alat", b)
	mustStatus(t, c3a.ID, "request_payment", 0)
	mustStatus(t, c3a.ID, "pay", 0)
	mustStatus(t, c3b.ID, "cancel", 0)
	if s, _ := recvStatus(t, pr3.ID); s != "paid" {
		t.Fatalf("induk = %q, mau paid (satu-satunya pecahan hidup lunas)", s)
	}
	if err := DeletePurchaseRequest(c3b.ID, false); err == nil {
		t.Error("pecahan batal bisa dihapus padahal induknya sudah lunas (T-34)")
	}
}

// T-35: tombol status "receive" hanya untuk jasa.
func TestReceiveActionOnlyForJasa(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: "Sendok Uji " + recvSuffix(), Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	mustStatus(t, pr.ID, "request_payment", 0)
	mustStatus(t, pr.ID, "pay", 0)
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "receive", ActorName: "pengaju"}); err == nil {
		t.Error("pengajuan barang ditutup 'received' lewat tombol status tanpa serah terima (T-35)")
	}
}

// T-19: jenis baris dibekukan saat dibuat; katalog yang berubah sesudahnya
// tidak memindahkan meja penerimaan.
func TestLineKindFrozenAtCreation(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	beras := "Beras Uji Beku " + sfx
	stockID := recvStockItem(t, beras)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Dapur", Items: []models.PurchaseSubItem{
		{Name: "  " + strings.ToUpper(beras) + " ", Qty: 1, Unit: "sak", HpsPrice: 600000, FinalPrice: 600000},
	}}})
	if k := pr.Items[0].Items[0].GoodsKind; k != "dapur" {
		t.Fatalf("goods_kind baris = %q, mau dapur (nama dicocokkan tanpa beda huruf/spasi)", k)
	}
	execScratch(`UPDATE stock_items SET is_active = false, name = name || ' lama' WHERE id = $1`, stockID)
	if l := recvDraft(t, pr.ID).Lines[0]; l.Kind != "dapur" || l.Destination != "stok" {
		t.Errorf("katalog dinonaktifkan memindahkan baris ke %s/%s (T-19)", l.Kind, l.Destination)
	}
	alat := "Oven Uji Beku " + sfx
	pr2 := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: alat, Qty: 1, Unit: "unit", HpsPrice: 900000, FinalPrice: 900000},
	}}})
	recvStockItem(t, alat) // nama yang sama baru masuk katalog sesudahnya
	if l := recvDraft(t, pr2.ID).Lines[0]; l.Kind == "dapur" {
		t.Error("baris peralatan pindah ke meja dapur karena katalog berubah (T-19)")
	}
}

// T-57: satu work order tidak bisa melahirkan dua PR; tautan ke PR yang
// ditolak tidak menghalangi pengajuan ulang, dan dilepas saat PR dihapus.
func TestMaintenancePurchaseRequestIdempotent(t *testing.T) {
	requireDB(t)
	outlet := recvOutlet(t)
	assetID, woID := NewULID(), NewULID()
	if _, err := database.DB.Exec(`INSERT INTO assets (id, outlet_id, name, quantity) VALUES ($1, $2, 'AC Uji WO', 1)`, assetID, outlet); err != nil {
		t.Fatalf("aset uji: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO asset_maintenances (id, asset_id, type, description, cost, status, wo_number, scheduled_date)
		VALUES ($1, $2, 'perbaikan', 'Ganti kompresor uji', 750000, 'dijadwalkan', $3, CURRENT_DATE)`,
		woID, assetID, "WO-UJI-"+strings.ToUpper(recvSuffix())); err != nil {
		t.Fatalf("WO uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM purchase_requests WHERE notes LIKE $1`, "%Ganti kompresor uji%")
		execScratch(`DELETE FROM purchase_requests WHERE id IN (SELECT purchase_request_id FROM asset_maintenances WHERE id = $1)`, woID)
		execScratch(`DELETE FROM asset_maintenances WHERE id = $1`, woID)
		execScratch(`DELETE FROM assets WHERE id = $1`, assetID)
	})
	t.Cleanup(func() { execScratch(`DELETE FROM purchase_requests WHERE outlet_id = $1`, outlet) })

	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]*models.PurchaseRequest, 3)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], _ = CreateMaintenancePurchaseRequest(woID, Actor{Name: "Teknisi Uji"}, nil)
		}(i)
	}
	close(start)
	wg.Wait()
	created := 0
	var first *models.PurchaseRequest
	for _, r := range results {
		if r != nil {
			created++
			first = r
		}
	}
	var total int
	database.DB.QueryRow(`SELECT COUNT(*) FROM purchase_requests WHERE outlet_id = $1`, outlet).Scan(&total)
	if created != 1 || total != 1 {
		t.Fatalf("PR dari satu WO: %d berhasil, %d tersimpan; mau tepat 1 (T-57)", created, total)
	}
	if _, err := UpdatePurchaseStatus(first.ID, models.UpdatePurchaseStatusInput{Action: "reject", ActorName: "verifikator", RejectedReason: "Vendor belum jelas"}); err != nil {
		t.Fatalf("tolak: %v", err)
	}
	again, err := CreateMaintenancePurchaseRequest(woID, Actor{Name: "Teknisi Uji"}, nil)
	if err != nil {
		t.Fatalf("ajukan ulang setelah PR ditolak: %v (T-57)", err)
	}
	if err := DeletePurchaseRequest(again.ID, false); err != nil {
		t.Fatalf("hapus PR pending: %v", err)
	}
	var link *string
	database.DB.QueryRow(`SELECT purchase_request_id FROM asset_maintenances WHERE id = $1`, woID).Scan(&link)
	if link != nil {
		t.Errorf("tautan WO ke PR yang dihapus tidak dilepas: %q", *link)
	}
}

// T-44: GRN manual tidak boleh membawa kunci baris pengajuan.
func TestManualReceiptRejectsPRItemKey(t *testing.T) {
	requireDB(t)
	_, err := CreateGoodsReceipt(models.GoodsReceiptRequest{
		WarehouseID: "tidak-dipakai",
		Items:       []models.GoodsReceiptItemReq{{ItemID: "x", QtyDist: 1, PRItemKey: "dapur|gula#0"}},
	}, "gudang")
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(err.Error(), "Penerimaan dari Pengadaan") {
		t.Errorf("GRN manual berkunci baris pengajuan: %v (T-44)", err)
	}
}

// T-67: vendor pecahan diambil dari daftar vendor; vendor nonaktif ditolak.
func TestSplitVendorResolvedFromVendorTable(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	active, inactive := NewULID(), NewULID()
	for _, v := range []struct {
		id, name string
		on       bool
	}{{active, "CV Resmi Uji " + sfx, true}, {inactive, "CV Mati Uji " + sfx, false}} {
		if _, err := database.DB.Exec(`INSERT INTO vendors (id, name, is_active) VALUES ($1, $2, $3)`, v.id, v.name, v.on); err != nil {
			t.Fatalf("vendor uji: %v", err)
		}
	}
	t.Cleanup(func() { execScratch(`DELETE FROM vendors WHERE id IN ($1, $2)`, active, inactive) })
	a, b := "Talenan Uji A "+sfx, "Talenan Uji B "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
		{Name: b, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	split := func(vendorID, item string) (*models.PurchaseRequest, error) {
		return SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
			VendorID: vendorID, VendorName: "Nama Palsu",
			Items: []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{{Name: item}}}},
		})
	}
	if _, err := split(inactive, a); err == nil {
		t.Error("pecah ke vendor nonaktif diterima")
	}
	m, err := split(active, a)
	if err != nil {
		t.Fatalf("pecah ke vendor aktif: %v", err)
	}
	if got := m.Children[0].VendorName; got != "CV Resmi Uji "+sfx {
		t.Errorf("nama vendor pecahan = %q, mau dari tabel vendors (T-67)", got)
	}
}

// T-80: nama grup/item panjang (kunci > 120 huruf) tetap bisa diterima.
func TestLongItemNamesReceivable(t *testing.T) {
	requireDB(t)
	group := "Perlengkapan " + strings.Repeat("panjang ", 18)
	item := "Barang " + strings.Repeat("panjang ", 15) + recvSuffix()
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: group, Items: []models.PurchaseSubItem{
		{Name: item, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	l := recvDraft(t, pr.ID).Lines[0]
	if len(l.ItemKey) <= 120 {
		t.Fatalf("kunci uji hanya %d karakter — tidak menguji batas lama", len(l.ItemKey))
	}
	if _, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "habis", Qty: 1, Reason: "langsung dipakai di ruang tamu",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Errorf("baris berkunci panjang gagal diterima: %v (T-80)", err)
	}
}

// T-20: PUT parsial katalog tidak mengosongkan kolom yang tidak dikirim, dan
// pengubahnya tercatat.
func TestStockItemPartialUpdateKeepsFields(t *testing.T) {
	requireDB(t)
	item, err := CreateStockItem(models.StockItemRequest{
		Name: "Kecap Uji Katalog " + recvSuffix(), Category: "Bumbu", BaseUnit: "ml", DistUnit: "botol",
		DistRatio: 600, DistUnitLabel: "botol 600ml", MinStock: 12, Notes: "catatan uji",
	}, "gudang_induk")
	if err != nil {
		t.Fatalf("buat item: %v", err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM stock_items WHERE id = $1`, item.ID) })
	got, err := UpdateStockItem(item.ID, models.StockItemRequest{Name: item.Name + " Baru", Present: map[string]bool{"name": true}}, "ppic")
	if err != nil {
		t.Fatalf("ubah nama: %v", err)
	}
	if got.Category != "Bumbu" || got.BaseUnit != "ml" || got.DistUnit != "botol" || got.DistRatio != 600 ||
		got.DistUnitLabel != "botol 600ml" || got.MinStock != 12 || got.Notes != "catatan uji" || got.Code != item.Code {
		t.Errorf("PUT parsial mengosongkan kolom lain: %+v (T-20)", got)
	}
	if got.CreatedBy != "gudang_induk" || got.UpdatedBy != "ppic" {
		t.Errorf("jejak pengubah = %q/%q, mau gudang_induk/ppic", got.CreatedBy, got.UpdatedBy)
	}
	if err := ToggleStockItemActive(item.ID, "admin_uji"); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	after, _ := GetStockItem(item.ID)
	if after.IsActive || after.UpdatedBy != "admin_uji" {
		t.Errorf("toggle: aktif=%v oleh=%q", after.IsActive, after.UpdatedBy)
	}
	if err := ToggleStockItemActive(NewULID(), "admin_uji"); err == nil {
		t.Error("toggle item yang tidak ada dianggap berhasil")
	}
}

// T-5/T-36: pecahan lunas diterima lewat antrean (dokumennya sendiri) →
// induk cangkang ikut 'received'.
func TestChildReceivedViaQueueClosesShellMaster(t *testing.T) {
	requireDB(t)
	name := "Wajan Uji Antrean " + recvSuffix()
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: name, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	child := recvSplit(t, pr.ID, "CV Wajan", "Alat", name)
	mustStatus(t, pr.ID, "request_payment", 0)
	mustStatus(t, child.ID, "pay", 0)
	if s, _ := recvStatus(t, pr.ID); s != "paid" {
		t.Fatalf("induk cangkang = %q, mau paid", s)
	}
	l := recvDraft(t, child.ID).Lines[0]
	if _, err := ReceiveGoods(child.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: l.PRItemKey, Destination: "habis", Qty: 1, Reason: "langsung dipakai di dapur",
	}}}, recvActor, recvAll, nil); err != nil {
		t.Fatalf("terima pecahan: %v", err)
	}
	if s, r := recvStatus(t, pr.ID); s != "received" || r != "received" {
		t.Errorf("induk setelah pecahan diterima: %s/%s, mau received/received (T-36)", s, r)
	}
	if rows, err := ListReceivingQueue("", nil, nil); err == nil {
		for _, r := range rows {
			if r.PurchaseRequestID == pr.ID || r.PurchaseRequestID == child.ID {
				t.Errorf("dokumen %s masih di antrean setelah lengkap", r.RequestNumber)
			}
		}
	}
}

// T-28/T-26: aksi bersamaan pada satu keluarga (serah terima lewat induk,
// ajukan bayar pecahan, ubah harga pecahan, hapus pecahan lain) tidak saling
// mengunci — semuanya mengunci induk lebih dulu, baru pecahan.
func TestFamilyConcurrentActionsNoDeadlock(t *testing.T) {
	requireDB(t)
	sfx := recvSuffix()
	a, b, c := "Panci Uji Serentak A "+sfx, "Panci Uji Serentak B "+sfx, "Panci Uji Serentak C "+sfx
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: a, Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
		{Name: b, Qty: 1, Unit: "pcs", HpsPrice: 20000, FinalPrice: 20000},
		{Name: c, Qty: 1, Unit: "pcs", HpsPrice: 30000, FinalPrice: 30000},
	}}})
	cb := recvSplit(t, pr.ID, "CV Panci B", "Alat", b)
	cc := recvSplit(t, pr.ID, "CV Panci C", "Alat", c)
	mustStatus(t, cc.ID, "cancel", 0)
	own := recvLine(t, recvDraft(t, pr.ID), a, pr.ID)
	cbItems := clonePurchaseItems(cb.Items)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 4)
	run := func(i int, f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f()
		}()
	}
	run(0, func() error {
		_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
			PRItemKey: own.PRItemKey, Destination: "habis", Qty: 1, Reason: "langsung dipakai di dapur",
		}}}, recvActor, recvAll, nil)
		return err
	})
	run(1, func() error {
		_, err := UpdatePurchaseStatus(cb.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "purchasing"})
		return err
	})
	run(2, func() error {
		_, err := UpdatePurchaseItems(cb.ID, models.UpdatePurchaseItemsInput{Items: cbItems})
		return err
	})
	run(3, func() error { return DeletePurchaseRequest(cc.ID, false) })
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil && strings.Contains(err.Error(), "deadlock") {
			t.Errorf("aksi %d saling mengunci: %v", i, err)
		}
	}
	if errs[0] != nil {
		t.Errorf("serah terima lewat induk gagal: %v", errs[0])
	}
}

// Jaga-jaga: pesan kesalahan kiriman tak dikenal menyebut baris & cara muat ulang.
func TestReceiveUnknownLine(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBarang, []models.PurchaseRequestItem{{Name: "Alat", Items: []models.PurchaseSubItem{
		{Name: "Gunting Uji " + recvSuffix(), Qty: 1, Unit: "pcs", HpsPrice: 10000, FinalPrice: 10000},
	}}})
	_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
		PRItemKey: "tidak|ada#0", Destination: "habis", Qty: 1, Reason: fmt.Sprintf("alasan %s", "panjang sekali"),
	}}}, recvActor, recvAll, nil)
	if err == nil || !strings.Contains(err.Error(), "muat ulang") {
		t.Errorf("baris tak dikenal: %v", err)
	}
}
