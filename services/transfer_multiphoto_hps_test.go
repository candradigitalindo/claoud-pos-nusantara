package services

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestNormalizeUploadURLs(t *testing.T) {
	urls, err := NormalizeUploadURLs("foto", "/uploads/2026-11/a.jpg", []string{
		"/uploads/2026-11/a.jpg",
		"/uploads/2026-11/b.jpg",
		"/uploads/2026-11/c.pdf",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(urls) != 3 || urls[0] != "/uploads/2026-11/a.jpg" || urls[2] != "/uploads/2026-11/c.pdf" {
		t.Fatalf("unexpected urls: %+v", urls)
	}

	many := make([]string, 11)
	for i := range many {
		many[i] = "/uploads/2026-11/f" + string(rune('a'+i)) + ".jpg"
	}
	if _, err := NormalizeUploadURLs("foto", "", many); err == nil {
		t.Fatal("expected error for >10 files")
	}

	if _, err := NormalizeUploadURLs("foto", "", []string{"https://evil.example/a.jpg"}); err == nil {
		t.Fatal("expected error for external url")
	}
}

func TestStockTransferDivisionManualDateAndMultiPhoto(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	outletID := NewULID()
	whCentral := NewULID()
	whOutlet := NewULID()
	itemID := NewULID()
	var ev1, ev2 string

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_transfer_receipt_items WHERE receipt_id IN (SELECT id FROM stock_transfer_receipts WHERE transfer_id IN (SELECT id FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $2))`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_transfer_receipts WHERE transfer_id IN (SELECT id FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM handover_photos WHERE ref_type = 'stock_transfer' AND ref_id IN (SELECT id FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_transfer_items WHERE transfer_id IN (SELECT id FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $2`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_evidence WHERE warehouse_id IN ($1, $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id IN ($1, $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id IN ($1, $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id IN ($1, $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, itemID)
		database.DB.Exec(`DELETE FROM warehouses WHERE id IN ($1, $2)`, whCentral, whOutlet)
		database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outletID)
	})

	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, slug, api_key, is_active) VALUES ($1, $2, 'Outlet Div Uji', $3, $4, true)`, outletID, "DV"+outletID[20:], "outlet-div-"+outletID[20:], "key-"+outletID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id, is_active) VALUES
		($1, $2, 'Gudang Induk Div Uji', 'central', NULL, true),
		($3, $4, 'Gudang Outlet Div Uji', 'outlet', $5, true)`,
		whCentral, "GC"+whCentral[20:], whOutlet, "GO"+whOutlet[20:], outletID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, min_stock, is_active)
		VALUES ($1, $2, 'Sirup Uji Divisi', 'Botol', 'Dus', 12, 1, true)`, itemID, "SY"+itemID[20:]); err != nil {
		t.Fatal(err)
	}
	if err := CreateAdjustment(models.AdjustmentRequest{
		ItemID:      itemID,
		WarehouseID: whCentral,
		QtyBase:     120,
		UnitUsed:    "Dus",
		CostPerBase:  15000,
		MovType:      "adjustment",
		Notes:        "seed FIFO",
	}, "admin"); err != nil {
		t.Fatalf("seed CreateAdjustment: %v", err)
	}

	// 1. Invalid division rejected
	if _, err := CreateStockTransfer(models.StockTransferRequest{
		FromWarehouseID: whCentral,
		ToWarehouseID:   whOutlet,
		TargetDivision:  "INVALID",
		Items:           []models.StockTransferItemReq{{ItemID: itemID, QtyDist: 1, UnitUsed: "Dus"}},
	}, "admin"); err == nil {
		t.Fatal("expected error for invalid division")
	}

	// 2. Create transfer with BAR + manual date 2026-11-20
	trBar, err := CreateStockTransfer(models.StockTransferRequest{
		FromWarehouseID: whCentral,
		ToWarehouseID:   whOutlet,
		TargetDivision:  "bar",
		TransferDate:    "2026-11-20",
		Notes:           "Rekap manual BAR",
		Items:           []models.StockTransferItemReq{{ItemID: itemID, QtyDist: 1, UnitUsed: "Dus"}},
	}, "admin")
	if err != nil {
		t.Fatalf("CreateStockTransfer BAR: %v", err)
	}
	if trBar.TargetDivision != "BAR" || trBar.TransferDate != "2026-11-20" {
		t.Fatalf("expected BAR + 2026-11-20, got div=%q date=%q", trBar.TargetDivision, trBar.TransferDate)
	}

	// 3. Create transfer with KITCHEN + default today date
	trKitchen, err := CreateStockTransfer(models.StockTransferRequest{
		FromWarehouseID: whCentral,
		ToWarehouseID:   whOutlet,
		TargetDivision:  "KITCHEN",
		Items:           []models.StockTransferItemReq{{ItemID: itemID, QtyDist: 1, UnitUsed: "Dus"}},
	}, "admin")
	if err != nil {
		t.Fatalf("CreateStockTransfer KITCHEN: %v", err)
	}
	todayWIB := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	if trKitchen.TargetDivision != "KITCHEN" || trKitchen.TransferDate != todayWIB {
		t.Fatalf("expected KITCHEN + %s, got div=%q date=%q", todayWIB, trKitchen.TargetDivision, trKitchen.TransferDate)
	}

	// 4. Filter by division and date range
	barList, barTotal, err := ListStockTransfersFiltered("", whOutlet, "BAR", "", "", nil, 1, 50)
	if err != nil {
		t.Fatalf("ListStockTransfersFiltered BAR: %v", err)
	}
	if barTotal != 1 || len(barList) != 1 || barList[0].ID != trBar.ID {
		t.Fatalf("expected only trBar for BAR filter, got total=%d len=%d", barTotal, len(barList))
	}

	dateList, dateTotal, err := ListStockTransfersFiltered("", whOutlet, "", "2026-11-19", "2026-11-21", nil, 1, 50)
	if err != nil {
		t.Fatalf("ListStockTransfersFiltered date: %v", err)
	}
	if dateTotal != 1 || len(dateList) != 1 || dateList[0].ID != trBar.ID {
		t.Fatalf("expected only trBar for 2026-11-19..21, got total=%d len=%d", dateTotal, len(dateList))
	}

	// 5. Approve & send with multiple photos
	if _, err := UpdateTransferStatus(trBar.ID, "approved", "admin"); err != nil {
		t.Fatalf("approve transfer: %v", err)
	}
	sent, err := UpdateTransferStatusFromPhotos(trBar.ID, "sent", "", []string{
		"/uploads/2026-11/kirim1.jpg",
		"/uploads/2026-11/kirim2.jpg",
	}, "admin", "approved")
	if err != nil {
		t.Fatalf("send transfer multi-photo: %v", err)
	}
	if sent.PhotoURL != "/uploads/2026-11/kirim1.jpg" || len(sent.PhotoURLs) != 2 {
		t.Fatalf("expected 2 send photo_urls, got primary=%q list=%+v", sent.PhotoURL, sent.PhotoURLs)
	}
	var hpCount int
	database.DB.QueryRow(`SELECT COUNT(*) FROM handover_photos WHERE ref_type='stock_transfer' AND ref_id=$1`, trBar.ID).Scan(&hpCount)
	if hpCount != 2 {
		t.Fatalf("expected 2 handover_photos for send, got %d", hpCount)
	}

	// 6. Receive with multiple timestamped evidence photos
	recvAcc := StockQualityAccess{
		ActorID:     NewULID(),
		ActorName:   "penerima",
		Permissions: map[string]bool{"*": true},
	}
	ev1 = qualityPhoto(t, whOutlet, recvAcc)
	ev2 = qualityPhoto(t, whOutlet, recvAcc)
	if err := ReceiveStockTransfer(trBar.ID, models.StockInspectionRequest{
		Version:     0,
		EvidenceID:  ev1,
		EvidenceIDs: []string{ev1, ev2},
		Notes:       "Semua diterima 2 foto",
		Items:       []models.StockInspectionItem{{ItemID: itemID, Accepted: 12}},
	}, recvAcc); err != nil {
		t.Fatalf("ReceiveStockTransfer multi-evidence: %v", err)
	}
	received, err := GetStockTransfer(trBar.ID)
	if err != nil {
		t.Fatalf("GetStockTransfer after receive: %v", err)
	}
	if len(received.Receipts) != 1 || len(received.Receipts[0].Evidences) != 2 {
		t.Fatalf("expected 1 receipt with 2 evidences, got %+v", received.Receipts)
	}
}

func TestProcurementMultiPhotoAndLastPurchasePriceHPS(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	whCentral := recvWarehouse(t)
	itemGRN := NewULID()
	itemPR := NewULID()
	grnOld := NewULID()
	grnNew := NewULID()
	prID := NewULID()
	prHistApproved := NewULID()
	prHistRejected := NewULID()

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM handover_photos WHERE ref_id IN ($1, $2, $3)`, prID, prHistApproved, prHistRejected)
		database.DB.Exec(`DELETE FROM purchase_struks WHERE purchase_request_id IN ($1, $2, $3)`, prID, prHistApproved, prHistRejected)
		database.DB.Exec(`DELETE FROM goods_receipt_items WHERE receipt_id IN ($1, $2)`, grnOld, grnNew)
		database.DB.Exec(`DELETE FROM goods_receipts WHERE id IN ($1, $2)`, grnOld, grnNew)
		database.DB.Exec(`DELETE FROM purchase_requests WHERE id IN ($1, $2, $3)`, prID, prHistApproved, prHistRejected)
		database.DB.Exec(`DELETE FROM stock_items WHERE id IN ($1, $2)`, itemGRN, itemPR)
	})

	grnName := "Susu UHT Uji HPS " + itemGRN[20:]
	prName := "Bubuk Matcha Uji HPS " + itemPR[20:]

	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, is_active) VALUES
		($1, $2, $3, 'Botol', 'Dus', 24, true),
		($4, $5, $6, 'gr', 'kg', 1000, true)`,
		itemGRN, "SH"+itemGRN[20:], grnName,
		itemPR, "BM"+itemPR[20:], prName); err != nil {
		t.Fatal(err)
	}

	if _, err := database.DB.Exec(`INSERT INTO goods_receipts (id, grn_number, warehouse_id, received_by, received_at, created_at) VALUES
		($1, $2, $3, 'uji', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
		($4, $5, $3, 'uji', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day')`,
		grnOld, "GRN-OLD-"+grnOld[20:], whCentral, grnNew, "GRN-NEW-"+grnNew[20:]); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO goods_receipt_items (id, receipt_id, item_id, qty_dist, unit_used, qty_base, cost_per_base, subtotal) VALUES
		($1, $2, $3, 1, 'Dus', 24, 18750, 450000),
		($4, $5, $3, 1, 'Dus', 24, 20000, 480000)`,
		NewULID(), grnOld, itemGRN, NewULID(), grnNew); err != nil {
		t.Fatal(err)
	}

	itemsApproved, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  "Bahan",
		Items: []models.PurchaseSubItem{{Name: prName, Qty: 2, Unit: "kg", HpsPrice: 60000, FinalPrice: 65000}},
	}})
	itemsRejected, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  "Bahan",
		Items: []models.PurchaseSubItem{{Name: prName, Qty: 2, Unit: "kg", HpsPrice: 999999, FinalPrice: 999999}},
	}})
	if _, err := database.DB.Exec(`INSERT INTO purchase_requests (id, request_number, request_type, status, items, total_amount, total_hps, total_final, requested_by, created_at) VALUES
		($1, $2, 'bahan', 'approved', $3::jsonb, 130000, 120000, 130000, 'uji', NOW() - INTERVAL '2 days'),
		($4, $5, 'bahan', 'rejected', $6::jsonb, 1999998, 1999998, 1999998, 'uji', NOW() - INTERVAL '1 hour')`,
		prHistApproved, "PR-HPS-OK-"+prHistApproved[20:], string(itemsApproved),
		prHistRejected, "PR-HPS-RJ-"+prHistRejected[20:], string(itemsRejected)); err != nil {
		t.Fatal(err)
	}

	cat, err := ListProcurementStockCatalog(nil)
	if err != nil {
		t.Fatalf("ListProcurementStockCatalog: %v", err)
	}
	raw, _ := json.Marshal(cat)
	if strings.Contains(string(raw), "avg_cost") || strings.Contains(string(raw), "total_value") {
		t.Fatal("catalog leaked avg_cost or total_value")
	}
	var gotGRN, gotPR *ProcurementStockItem
	for i := range cat.Items {
		if cat.Items[i].ID == itemGRN {
			gotGRN = &cat.Items[i]
		}
		if cat.Items[i].ID == itemPR {
			gotPR = &cat.Items[i]
		}
	}
	if gotGRN == nil || math.Abs(gotGRN.LastPurchasePrice-480000) > 1 || gotGRN.LastPurchaseSource != "GRN" {
		t.Fatalf("expected itemGRN last_purchase_price=480000 (GRN), got %+v", gotGRN)
	}
	if gotPR == nil || math.Abs(gotPR.LastPurchasePrice-65000) > 1 || gotPR.LastPurchaseSource != "Pengadaan" {
		t.Fatalf("expected itemPR last_purchase_price=65000 (Pengadaan), got %+v", gotPR)
	}

	if _, err := database.DB.Exec(`INSERT INTO purchase_requests (id, request_number, request_type, status, items, total_amount, total_hps, total_final, requested_by)
		VALUES ($1, $2, 'bahan', 'approved', $3::jsonb, 130000, 120000, 130000, 'uji')`,
		prID, "PR-MP-"+prID[20:], string(itemsApproved)); err != nil {
		t.Fatal(err)
	}
	invURLs := []string{"/uploads/2026-11/inv1.jpg", "/uploads/2026-11/inv2.pdf"}
	updated, err := UpdatePurchaseItems(prID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name:  "Bahan",
			Items: []models.PurchaseSubItem{{Name: prName, Qty: 2, Unit: "kg", HpsPrice: 60000, FinalPrice: 65000}},
		}},
		InvoiceURLs: &invURLs,
	})
	if err != nil {
		t.Fatalf("UpdatePurchaseItems multi invoice: %v", err)
	}
	if updated.InvoiceURL != "/uploads/2026-11/inv1.jpg" || len(updated.InvoiceURLs) != 2 {
		t.Fatalf("expected 2 invoice_urls, got primary=%q list=%+v", updated.InvoiceURL, updated.InvoiceURLs)
	}

	todayWIB := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	struk, err := AddPurchaseStruk(prID, models.AddPurchaseStrukInput{
		StrukNumber: "STR-MULTI-1",
		StrukDate:   todayWIB,
		Total:       130000,
		PhotoURLs: []string{
			"/uploads/2026-11/bon1.jpg",
			"/uploads/2026-11/bon2.jpg",
			"/uploads/2026-11/bon3.jpg",
		},
	}, Actor{Name: "purchasing"})
	if err != nil {
		t.Fatalf("AddPurchaseStruk multi-photo: %v", err)
	}
	if struk.PhotoURL != "/uploads/2026-11/bon1.jpg" || len(struk.PhotoURLs) != 3 {
		t.Fatalf("expected 3 struk photo_urls, got primary=%q list=%+v", struk.PhotoURL, struk.PhotoURLs)
	}
}

func TestSplitVendorMasterPreservesRealDisplayNames(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	masterID := NewULID()
	child1 := NewULID()
	child2 := NewULID()
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM purchase_requests WHERE id IN ($1, $2, $3)`, child1, child2, masterID)
	})

	realName := "PEMBELANJAAN MINGGUAN UJI " + masterID[20:]
	c1Items, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  realName,
		Items: []models.PurchaseSubItem{{Name: "Butter", Qty: 2, Unit: "Pack", HpsPrice: 40000}},
	}})
	c2Items, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  realName,
		Items: []models.PurchaseSubItem{{Name: "Susu", Qty: 1, Unit: "Dus", HpsPrice: 200000}},
	}})
	if _, err := database.DB.Exec(`INSERT INTO purchase_requests (id, request_number, request_type, status, split_status, items, total_amount, total_hps, total_final, requested_by)
		VALUES ($1, $2, 'bahan', 'approved', 'master', '[]'::jsonb, 0, 0, 0, 'uji')`,
		masterID, "PR-SM-"+masterID[20:]); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO purchase_requests (id, request_number, request_type, status, parent_id, vendor_name, items, total_amount, total_hps, total_final, requested_by) VALUES
		($1, $2, 'bahan', 'approved', $3, 'Vendor A', $4::jsonb, 80000, 80000, 0, 'uji'),
		($5, $6, 'bahan', 'approved', $3, 'Vendor B', $7::jsonb, 200000, 200000, 0, 'uji')`,
		child1, "PR-SC1-"+child1[20:], masterID, string(c1Items),
		child2, "PR-SC2-"+child2[20:], string(c2Items)); err != nil {
		t.Fatal(err)
	}

	res, err := ListPurchaseRequests("", "", "", "bahan", "", "", false, realName, nil, nil, nil, 1, 20)
	if err != nil {
		t.Fatalf("ListPurchaseRequests search realName: %v", err)
	}
	if len(res.Requests) != 1 || res.Requests[0].ID != masterID {
		t.Fatalf("expected masterPR via search=%q, got %d rows", realName, len(res.Requests))
	}
	got := res.Requests[0]
	if got.SplitCount != 2 || len(got.DisplayNames) != 1 || got.DisplayNames[0] != realName {
		t.Fatalf("expected SplitCount=2 DisplayNames=[%q], got count=%d names=%+v", realName, got.SplitCount, got.DisplayNames)
	}
}
