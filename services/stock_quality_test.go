package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"sync"
	"testing"
	"time"
)

func qualityActor(name string, scope []string, perms ...string) StockQualityAccess {
	a := StockQualityAccess{ActorID: name, ActorName: name, Scope: scope, Permissions: map[string]bool{"stockquality.view": true}}
	for _, p := range perms {
		a.Permissions[p] = true
	}
	return a
}
func qualityPhotoBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func qualityPhoto(t *testing.T, wh string, a StockQualityAccess) string {
	t.Helper()
	photo, err := CreateStockEvidence(wh, qualityPhotoBytes(t), a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p, _ := UploadFilePath(photo.URL); os.Remove(p) })
	return photo.ID
}
func qualityCleanup(t *testing.T, warehouses ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, wh := range warehouses {
			execScratch(`DELETE FROM stock_quality_cases WHERE warehouse_id=$1 OR return_warehouse_id=$1`, wh)
			execScratch(`DELETE FROM stock_transfer_receipts WHERE transfer_id IN(SELECT id FROM stock_transfers WHERE from_warehouse_id=$1 OR to_warehouse_id=$1)`, wh)
			execScratch(`DELETE FROM stock_evidence WHERE warehouse_id=$1`, wh)
			execScratch(`DELETE FROM stock_wastes WHERE warehouse_id=$1`, wh)
		}
	})
}
func qualityReport(t *testing.T, f *bdFixture, item, reason string, qty float64, unit string, a StockQualityAccess) string {
	t.Helper()
	batches, err := StockQualityBatches(f.wh, item, a)
	if err != nil || len(batches) == 0 {
		t.Fatalf("batches: %v %+v", err, batches)
	}
	id, err := CreateStockQuality(models.StockQualityRequest{WarehouseID: f.wh, ItemID: item, BatchID: batches[0].ID, Qty: qty, UnitUsed: unit, Reason: reason, Notes: "Kondisi tidak sesuai", EvidenceID: qualityPhoto(t, f.wh, a)}, a)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func qualityRead(t *testing.T, id string) *models.StockQualityCase {
	t.Helper()
	c, err := GetStockQuality(id, qualityActor("reader", nil, "*"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func qualityDo(t *testing.T, id, action string, a StockQualityAccess, edit func(*models.StockQualityAction)) {
	t.Helper()
	c := qualityRead(t, id)
	r := models.StockQualityAction{Action: action, Version: c.Version, Notes: "Bukti tindakan telah diperiksa"}
	wh := c.WarehouseID
	if action == "receive_return" {
		wh = c.ReturnWarehouseID
	}
	if action != "approve" {
		r.EvidenceID = qualityPhoto(t, wh, a)
	}
	if edit != nil {
		edit(&r)
	}
	if err := ActStockQuality(id, r, a); err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}

func TestStockEvidenceStampAndScope(t *testing.T) {
	data := qualityPhotoBytes(t)
	now := time.Date(2026, 10, 21, 12, 30, 0, 0, time.UTC)
	photo, err := stampStockEvidence(data, "01STAMPEVIDENCE012345678900", "operator", now)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(photo))
	if err != nil || img.Bounds().Dy() != 110 || img.Bounds().Dx() != 720 {
		t.Fatalf("foto timestamp: %v %v", img, err)
	}
	if bytes.Equal(data, photo) {
		t.Fatal("foto tidak diberi cap server")
	}
	if _, err := stampStockEvidence([]byte("%PDF-surat"), "id", "op", now); err == nil {
		t.Fatal("PDF diterima sebagai foto")
	}
	requireDB(t)
	f := newBreakdownFixture(t)
	qualityCleanup(t, f.wh)
	a := qualityActor("outlet", []string{f.outlet}, "stockquality.outlet.report")
	other := qualityActor("other", []string{}, "stockquality.outlet.report")
	if _, err := CreateStockEvidence(f.wh, data, other); !errors.Is(err, ErrStockQualityAccess) {
		t.Fatalf("scope foto: %v", err)
	}
	id := qualityPhoto(t, f.wh, a)
	var actor, at string
	if err := database.DB.QueryRow(`SELECT actor_id,recorded_at::text FROM stock_evidence WHERE id=$1`, id).Scan(&actor, &at); err != nil || actor != a.ActorID || at == "" {
		t.Fatalf("audit foto: %v %s %s", err, actor, at)
	}
}

func TestStockQualityBatchApprovalAndDisposal(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	item := f.item(t, "Susu", "botol", "dus", 12)
	qualityCleanup(t, f.wh)
	f.stockIn(t, item, 12, 2000, "2025-01-01")
	f.stockIn(t, item, 24, 3000, "2027-01-01")
	reporter := qualityActor("pelapor", []string{f.outlet}, "stockquality.outlet.report")
	manager := qualityActor("supervisor", []string{f.outlet}, "stockquality.outlet.approve")
	executor := qualityActor("pelaksana", []string{f.outlet}, "stockquality.outlet.execute")
	batches, err := StockQualityBatches(f.wh, item, reporter)
	if err != nil {
		t.Fatal(err)
	}
	req := models.StockQualityRequest{WarehouseID: f.wh, ItemID: item, BatchID: batches[1].ID, Qty: 1, UnitUsed: "dus", Reason: "spoiled", Notes: "Segel bocor", EvidenceID: qualityPhoto(t, f.wh, reporter)}
	if _, err := CreateStockQuality(req, manager); !errors.Is(err, ErrStockQualityAccess) {
		t.Fatalf("lapor tanpa izin: %v", err)
	}
	id, err := CreateStockQuality(req, reporter)
	if err != nil {
		t.Fatal(err)
	}
	c := qualityRead(t, id)
	if c.CostPerBase != 3000 || c.QtyBase != 12 || c.ExpiryDate != "2027-01-01" {
		t.Fatalf("batch terpilih: %+v", c)
	}
	if qty, _ := f.ledger(t, item); qty != 24 {
		t.Fatalf("karantina: %v", qty)
	}
	if _, err := CreateStockQuality(req, reporter); err == nil {
		t.Fatal("bukti dapat dipakai ulang")
	}
	self := reporter
	self.Permissions["stockquality.outlet.approve"] = true
	if err := ActStockQuality(id, models.StockQualityAction{Action: "approve", Version: 1, Disposition: "dispose", Notes: "sendiri"}, self); err == nil {
		t.Fatal("pelapor menyetujui sendiri")
	}
	qualityDo(t, id, "approve", manager, func(r *models.StockQualityAction) { r.Disposition = "dispose" })
	if err := ActStockQuality(id, models.StockQualityAction{Action: "execute", Version: 2, Notes: "tanpa foto"}, executor); err == nil {
		t.Fatal("pelaksanaan tanpa foto")
	}
	qualityDo(t, id, "execute", executor, nil)
	if qty, _ := f.ledger(t, item); qty != 24 {
		t.Fatalf("stok dipotong dua kali: %v", qty)
	}
	var loss float64
	if err := database.DB.QueryRow(`SELECT SUM(total_cost) FROM stock_wastes WHERE warehouse_id=$1`, f.wh).Scan(&loss); err != nil || loss != 36000 {
		t.Fatalf("nilai pemusnahan: %v %v", loss, err)
	}
	c = qualityRead(t, id)
	if c.Status != "disposed" || len(c.Events) != 3 || c.Events[0].Evidence == nil || c.Events[2].Evidence == nil || c.Events[0].Reference != req.BatchID {
		t.Fatalf("jejak audit: %+v", c)
	}
}

func TestStockQualityReleaseScopeAndEvidenceOwnership(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	item := f.item(t, "Cleo", "botol", "dus", 24)
	qualityCleanup(t, f.wh)
	f.stockIn(t, item, 24, 2000, "2028-01-01")
	reporter := qualityActor("qc", []string{f.outlet}, "stockquality.outlet.report")
	manager := qualityActor("kepala", []string{f.outlet}, "stockquality.outlet.approve")
	id := qualityReport(t, f, item, "reject", 4, "botol", reporter)
	if _, err := GetStockQuality(id, qualityActor("asing", []string{})); !errors.Is(err, ErrStockQualityAccess) {
		t.Fatalf("baca lintas outlet: %v", err)
	}
	r := models.StockQualityAction{Action: "release", Version: 1, Notes: "hasil cek layak", EvidenceID: qualityPhoto(t, f.wh, reporter)}
	if err := ActStockQuality(id, r, manager); err == nil {
		t.Fatal("menggunakan foto akun lain")
	}
	qualityDo(t, id, "release", manager, nil)
	if qty, _ := f.ledger(t, item); qty != 24 {
		t.Fatalf("stok pelepasan: %v", qty)
	}
	var exp string
	if err := database.DB.QueryRow(`SELECT expiry_date::text FROM stock_batches WHERE ref_id=$1 AND ref_type='stock_quality'`, id).Scan(&exp); err != nil || exp != "2028-01-01" {
		t.Fatalf("tanggal batch hilang: %v %s", err, exp)
	}
}

func qualityVendor(t *testing.T) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO vendors(id,name) VALUES($1,'Supplier Uji Mutu')`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM vendors WHERE id=$1`, id) })
	return id
}

func TestStockQualitySupplierReplacementAndRefund(t *testing.T) {
	for _, settlement := range []string{"replacement", "refund"} {
		t.Run(settlement, func(t *testing.T) {
			outlet, central, item := unitWarehouses(t)
			qualityCleanup(t, outlet.wh, central.wh)
			vendor := qualityVendor(t)
			central.stockIn(t, item, 48, 2000, "")
			qc := qualityActor("qc-induk", nil, "stockquality.central.report", "stockquality.central.receive")
			manager := qualityActor("kepala-induk", nil, "stockquality.central.approve")
			executor := qualityActor("logistik", nil, "stockquality.central.execute")
			finance := qualityActor("finance", nil, "stockquality.refund")
			id := qualityReport(t, central, item, "return", 1, "dus", qc)
			qualityDo(t, id, "approve", manager, func(r *models.StockQualityAction) {
				r.Disposition = "return_supplier"
				r.VendorID = vendor
				r.Reference = "INV-123"
				r.Settlement = settlement
			})
			qualityDo(t, id, "execute", executor, nil)
			a := qc
			if settlement == "refund" {
				a = finance
			}
			for _, qty := range []float64{10, 14} {
				qualityDo(t, id, settlement, a, func(r *models.StockQualityAction) {
					r.QtyBase = qty
					r.Reference = "Bukti supplier"
					r.Amount = qty * 1900
				})
			}
			c := qualityRead(t, id)
			if c.Status != "settled" || c.SettledQtyBase != 24 {
				t.Fatalf("penyelesaian: %+v", c)
			}
			want := 24.0
			if settlement == "replacement" {
				want = 48
			}
			if qty, _ := central.ledger(t, item); qty != want {
				t.Fatalf("stok penyelesaian: %v, mau %v", qty, want)
			}
			if settlement == "refund" && c.RefundAmount != 45600 {
				t.Fatalf("refund: %v", c.RefundAmount)
			}
			if err := ActStockQuality(id, models.StockQualityAction{Action: settlement, Version: c.Version, Notes: "ulang", QtyBase: 24, Amount: 48000, Reference: "ulang", EvidenceID: qualityPhoto(t, central.wh, a)}, a); err == nil {
				t.Fatal("retur selesai dapat diselesaikan ulang")
			}
		})
	}
}

func TestStockInspectionPartialRejectAndReturn(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	qualityCleanup(t, outlet.wh, central.wh)
	central.stockIn(t, item, 48, 2000, "2028-02-01")
	doc := unitTransfer(t, central.wh, outlet.wh, item, "dus", 2)
	receiver := qualityActor("penerima", []string{outlet.outlet}, "stocktransfers.receive", "stocktransfers.reject", "stockquality.outlet.report")
	request := models.StockInspectionRequest{Version: 0, EvidenceID: qualityPhoto(t, outlet.wh, receiver), Items: []models.StockInspectionItem{{ItemID: item, Accepted: 20, Rejected: 4, Reason: "Bocor"}}}
	if err := ReceiveStockTransfer(doc.ID, request, receiver); err != nil {
		t.Fatal(err)
	}
	if err := ReceiveStockTransfer(doc.ID, request, receiver); err == nil {
		t.Fatal("penerimaan diposting dua kali")
	}
	doc, err := GetStockTransfer(doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Status != "partial_received" || doc.Items[0].AcceptedQtyBase != 20 || doc.Items[0].RejectedQtyBase != 4 || len(doc.Receipts) != 1 {
		t.Fatalf("parsial: %+v", doc)
	}
	if qty, _ := outlet.ledger(t, item); qty != 20 {
		t.Fatalf("reject masuk stok tersedia: %v", qty)
	}
	var waste int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_wastes WHERE item_id=$1`, item).Scan(&waste)
	if waste != 0 {
		t.Fatal("reject dianggap hilang")
	}
	request.Version = 1
	request.EvidenceID = qualityPhoto(t, outlet.wh, receiver)
	request.Items = []models.StockInspectionItem{{ItemID: item, Accepted: 24}}
	if err = ReceiveStockTransfer(doc.ID, request, receiver); err != nil {
		t.Fatal(err)
	}
	if qty, _ := outlet.ledger(t, item); qty != 44 {
		t.Fatalf("penerimaan berikutnya: %v", qty)
	}
	cases, _, err := ListStockQuality(outlet.wh, "", "", 1, 20, receiver)
	if err != nil || len(cases) != 1 {
		t.Fatalf("kasus reject: %v %+v", err, cases)
	}
	id := cases[0].ID
	manager := qualityActor("supervisor-outlet", []string{outlet.outlet}, "stockquality.outlet.approve")
	executor := qualityActor("pelaksana-outlet", []string{outlet.outlet}, "stockquality.outlet.execute")
	centralQC := qualityActor("qc-induk", nil, "stockquality.central.receive")
	qualityDo(t, id, "approve", manager, func(r *models.StockQualityAction) {
		r.Disposition = "return_warehouse"
		r.ReturnWarehouseID = central.wh
	})
	qualityDo(t, id, "execute", executor, nil)
	qualityDo(t, id, "receive_return", centralQC, nil)
	if qty, _ := central.ledger(t, item); qty != 0 {
		t.Fatalf("reject otomatis tersedia di induk: %v", qty)
	}
	cases, _, err = ListStockQuality(central.wh, "", "", 1, 20, centralQC)
	if err != nil || len(cases) != 1 || cases[0].ParentCaseID != id || cases[0].QtyBase != 4 || cases[0].CostPerBase != 2000 {
		t.Fatalf("karantina induk: %v %+v", err, cases)
	}
	if cases[0].ExpiryDate != "2028-02-01" {
		t.Fatal("tanggal ED pengiriman tidak dibawa")
	}
}

func TestStockInspectionConcurrentAndLimits(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	qualityCleanup(t, outlet.wh, central.wh)
	central.stockIn(t, item, 24, 2000, "")
	doc := unitTransfer(t, central.wh, outlet.wh, item, "dus", 1)
	a := qualityActor("receive", []string{outlet.outlet}, "stocktransfers.receive")
	centralOnly := qualityActor("qc-central-only", nil, "stockquality.central.receive")
	if err := StockQualityWarehouseAccess(outlet.wh, centralOnly, "transfer_receive"); !errors.Is(err, ErrStockQualityAccess) {
		t.Fatalf("role penerima induk dapat menerima atas nama outlet: %v", err)
	}
	req := models.StockInspectionRequest{Version: 0, EvidenceID: qualityPhoto(t, outlet.wh, a), Items: []models.StockInspectionItem{{ItemID: item, Accepted: 25}}}
	if err := ReceiveStockTransfer(doc.ID, req, a); err == nil {
		t.Fatal("over-receipt")
	}
	req.Items[0].Accepted = 20
	req.Items[0].Rejected = 4
	req.Items[0].Reason = "Bocor"
	if err := ReceiveStockTransfer(doc.ID, req, a); err == nil {
		t.Fatal("reject tanpa izin")
	}
	req.Items[0].Accepted = 24
	req.Items[0].Rejected = 0
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- ReceiveStockTransfer(doc.ID, req, a) }()
	}
	wg.Wait()
	close(results)
	passed := 0
	for err := range results {
		if err == nil {
			passed++
		}
	}
	if passed != 1 {
		t.Fatalf("konkurensi: %d diterima", passed)
	}
	if qty, _ := outlet.ledger(t, item); qty != 24 {
		t.Fatalf("stok ganda: %v", qty)
	}
}
