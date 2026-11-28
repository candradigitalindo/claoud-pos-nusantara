package services

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestLogisticsDeliveryCrossPRManifestQuotaAndTwoDeskReceive(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	outletID := NewULID()
	whCentral := NewULID()
	itemID := NewULID()
	prBahan := NewULID()
	prAlat := NewULID()
	prCancel := NewULID()
	var delID, delCancelID string

	t.Cleanup(func() {
		if delID != "" {
			database.DB.Exec(`DELETE FROM handover_photos WHERE ref_id = $1`, delID)
			database.DB.Exec(`DELETE FROM logistics_deliveries WHERE id = $1`, delID)
		}
		if delCancelID != "" {
			database.DB.Exec(`DELETE FROM logistics_deliveries WHERE id = $1`, delCancelID)
		}
		for _, pid := range []string{prBahan, prAlat, prCancel} {
			database.DB.Exec(`DELETE FROM handover_photos WHERE ref_id = $1`, pid)
			database.DB.Exec(`DELETE FROM pr_receiving_decisions WHERE purchase_request_id = $1`, pid)
			database.DB.Exec(`DELETE FROM assets WHERE purchase_request_id = $1`, pid)
			database.DB.Exec(`DELETE FROM goods_receipt_items WHERE receipt_id IN (SELECT id FROM goods_receipts WHERE purchase_request_id = $1)`, pid)
			database.DB.Exec(`DELETE FROM goods_receipts WHERE purchase_request_id = $1`, pid)
			database.DB.Exec(`DELETE FROM purchase_requests WHERE id = $1`, pid)
		}
		database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id = $1`, whCentral)
		database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id = $1`, whCentral)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id = $1`, whCentral)
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, itemID)
		database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, whCentral)
		database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outletID)
	})

	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, slug, api_key, is_active) VALUES ($1, $2, 'Outlet Logistik Uji', $3, $4, true)`,
		outletID, "LG"+outletID[20:], "outlet-lg-"+outletID[20:], "key-"+outletID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id, is_active) VALUES ($1, $2, 'Gudang Induk Log Uji', 'central', NULL, true)`,
		whCentral, "GL"+whCentral[20:]); err != nil {
		t.Fatal(err)
	}
	susuName := "Susu Logistik Uji " + itemID[20:]
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, is_active) VALUES ($1, $2, $3, 'Botol', 'Dus', 12, true)`,
		itemID, "SL"+itemID[20:], susuName); err != nil {
		t.Fatal(err)
	}

	itemsBahan, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  "Belanja Bahan",
		Items: []models.PurchaseSubItem{{Name: susuName, Qty: 10, Unit: "Dus", HpsPrice: 120000, FinalPrice: 120000}},
	}})
	itemsAlat, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  "Belanja Alat",
		Items: []models.PurchaseSubItem{{Name: "Blender Uji " + prAlat[20:], Qty: 2, Unit: "unit", HpsPrice: 2500000, FinalPrice: 2500000}},
	}})
	itemsCancel, _ := json.Marshal([]models.PurchaseRequestItem{{
		Name:  "Belanja Cancel",
		Items: []models.PurchaseSubItem{{Name: "Rak Uji " + prCancel[20:], Qty: 5, Unit: "unit", HpsPrice: 300000, FinalPrice: 300000}},
	}})

	if _, err := database.DB.Exec(`INSERT INTO purchase_requests (id, request_number, outlet_id, request_type, goods_kind, status, items, total_amount, total_hps, total_final, vendor_name, requested_by) VALUES
		($1, $2, $3, 'barang', 'dapur', 'paid', $4::jsonb, 1200000, 1200000, 1200000, 'PT Susu Jaya', 'gudang'),
		($5, $6, $3, 'barang', 'perlengkapan', 'paid', $7::jsonb, 5000000, 5000000, 5000000, 'CV Alat Dapur', 'outlet'),
		($8, $9, $3, 'barang', 'perlengkapan', 'approved', $10::jsonb, 1500000, 1500000, 1500000, 'CV Rak', 'outlet')`,
		prBahan, "PR-LB-"+prBahan[20:], outletID, string(itemsBahan),
		prAlat, "PR-LA-"+prAlat[20:], string(itemsAlat),
		prCancel, "PR-LC-"+prCancel[20:], string(itemsCancel),
	); err != nil {
		t.Fatal(err)
	}

	// 1. Check dispatchable items include both PRs
	disp, err := ListDispatchablePRItems(nil, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("ListDispatchablePRItems: %v", err)
	}
	var keyBahan, keyAlat, keyCancel string
	for _, it := range disp {
		if it.PurchaseRequestID == prBahan {
			keyBahan = it.PRItemKey
			if it.QtyAvailable != 10 {
				t.Fatalf("expected prBahan QtyAvailable=10, got %d", it.QtyAvailable)
			}
		}
		if it.PurchaseRequestID == prAlat {
			keyAlat = it.PRItemKey
			if it.QtyAvailable != 2 {
				t.Fatalf("expected prAlat QtyAvailable=2, got %d", it.QtyAvailable)
			}
		}
		if it.PurchaseRequestID == prCancel {
			keyCancel = it.PRItemKey
		}
	}
	if keyBahan == "" || keyAlat == "" || keyCancel == "" {
		t.Fatalf("missing dispatchable keys: bahan=%q alat=%q cancel=%q", keyBahan, keyAlat, keyCancel)
	}

	// 2. Create 1 combined Surat Jalan carrying 4 Dus from PR-Bahan + 2 unit from PR-Alat
	del, err := CreateLogisticsDelivery(models.CreateLogisticsDeliveryInput{
		DispatchMode:           "logistics",
		DispatchNow:            true,
		CourierName:            "Budi Logistik",
		VehicleInfo:            "Granmax B 1234 XY",
		OriginLabel:            "Purchasing",
		DestinationType:        "warehouse",
		DestinationWarehouseID: whCentral,
		PickupPhotoURLs:        []string{"/uploads/2026-11/pk1.jpg"},
		Items: []models.LogisticsDeliveryItemInput{
			{PurchaseRequestID: prBahan, PRItemKey: keyBahan, QtyPlanned: 4},
			{PurchaseRequestID: prAlat, PRItemKey: keyAlat, QtyPlanned: 2},
		},
	}, Actor{Name: "Purchasing"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateLogisticsDelivery: %v", err)
	}
	delID = del.ID
	if !strings.HasPrefix(del.DeliveryNumber, "SJ-") || del.Status != "in_transit" || len(del.Items) != 2 {
		t.Fatalf("unexpected created delivery: %+v", del)
	}

	// 2b. Verify ListPurchaseRequests batch-attaches Deliveries so Purchasing list shows SJ badge live
	prList, err := ListPurchaseRequests("", "", "", "barang", "", "", false, "PR-LB-"+prBahan[20:], nil, nil, nil, 1, 20)
	if err != nil || len(prList.Requests) != 1 || len(prList.Requests[0].Deliveries) != 1 || prList.Requests[0].Deliveries[0].DeliveryNumber != del.DeliveryNumber {
		t.Fatalf("expected ListPurchaseRequests to attach delivery %s, err=%v got=%+v", del.DeliveryNumber, err, prList)
	}

	// 3. Anti-collision: direct vendor draft on prBahan must show InDelivery=4, AvailableDirect=6
	draft, err := BuildReceivingDraft(prBahan, nil)
	if err != nil {
		t.Fatalf("BuildReceivingDraft: %v", err)
	}
	if len(draft.Lines) != 1 || draft.Lines[0].InDelivery != 4 || draft.Lines[0].AvailableDirect != 6 {
		t.Fatalf("expected InDelivery=4 AvailableDirect=6, got %+v", draft.Lines)
	}

	// Attempting direct receive qty=7 MUST FAIL
	gudangAccess := ReceiveAccess{Stock: true}
	if _, err := ReceiveGoods(prBahan, models.ReceiveGoodsRequest{
		PhotoURLs: []string{"/uploads/2026-11/vnd1.jpg"},
		Desk:      "dapur",
		Lines: []models.ReceiveGoodsLine{{
			PRItemKey:   draft.Lines[0].PRItemKey,
			Destination: "stok",
			Qty:         7,
			StockItemID: itemID,
			WarehouseID: whCentral,
			UnitUsed:    "Dus",
		}},
	}, Actor{Name: "Kepala Gudang"}, gudangAccess, nil); err == nil {
		t.Fatal("expected anti-collision error when receiving 7 direct while 4 are in active delivery")
	}

	// Receiving 6 direct vendor MUST SUCCEED
	if _, err := ReceiveGoods(prBahan, models.ReceiveGoodsRequest{
		PhotoURLs: []string{"/uploads/2026-11/vnd1.jpg"},
		Desk:      "dapur",
		Lines: []models.ReceiveGoodsLine{{
			PRItemKey:   draft.Lines[0].PRItemKey,
			Destination: "stok",
			Qty:         6,
			StockItemID: itemID,
			WarehouseID: whCentral,
			UnitUsed:    "Dus",
		}},
	}, Actor{Name: "Kepala Gudang"}, gudangAccess, nil); err != nil {
		t.Fatalf("direct vendor receive 6 Dus: %v", err)
	}

	// 4. Edit/delete/replace draft receive photos BEFORE final received
	pDraft1 := []string{"/uploads/2026-11/r1.jpg", "/uploads/2026-11/r2.jpg"}
	del, err = UpdateLogisticsDeliveryPhotos(delID, models.UpdateLogisticsPhotosInput{ReceivePhotoURLs: &pDraft1}, false, true, nil)
	if err != nil || len(del.ReceivePhotoURLs) != 2 {
		t.Fatalf("save draft receive photos: err=%v urls=%+v", err, del)
	}
	// Delete r1.jpg, replace with r3.jpg
	pDraft2 := []string{"/uploads/2026-11/r2.jpg", "/uploads/2026-11/r3.jpg"}
	del, err = UpdateLogisticsDeliveryPhotos(delID, models.UpdateLogisticsPhotosInput{ReceivePhotoURLs: &pDraft2}, false, true, nil)
	if err != nil || len(del.ReceivePhotoURLs) != 2 || del.ReceivePhotoURLs[0] != "/uploads/2026-11/r2.jpg" || del.ReceivePhotoURLs[1] != "/uploads/2026-11/r3.jpg" {
		t.Fatalf("replace draft receive photos failed: err=%v urls=%+v", err, del.ReceivePhotoURLs)
	}

	var diBahanID, diAlatID string
	for _, it := range del.Items {
		if it.PurchaseRequestID == prBahan {
			diBahanID = it.ID
		}
		if it.PurchaseRequestID == prAlat {
			diAlatID = it.ID
		}
	}

	// 5. Desk 1 (Gudang Induk) receives 4 Dus bahan baku from SJ → SJ becomes partial_received!
	res1, err := ReceiveLogisticsDelivery(delID, models.ReceiveLogisticsDeliveryInput{
		Desk: "dapur",
		// Omit PhotoURLs to verify fallback to saved draft ReceivePhotoURLs!
		Lines: []models.ReceiveLogisticsLineInput{{
			DeliveryItemID: diBahanID,
			Qty:            4,
			Destination:    "stok",
			StockItemID:    itemID,
			WarehouseID:    whCentral,
			UnitUsed:       "Dus",
		}},
	}, Actor{Name: "Kepala Gudang"}, gudangAccess, nil)
	if err != nil {
		t.Fatalf("Desk 1 ReceiveLogisticsDelivery: %v", err)
	}
	if res1.DeliveryStatus != "partial_received" || res1.RemainingItems != 1 || res1.GRNNumber == "" {
		t.Fatalf("expected partial_received + GRN, got %+v", res1)
	}

	// 6. Desk 2 (Bagian Aset) receives 2 unit Blender from the SAME SJ → SJ becomes received!
	asetAccess := ReceiveAccess{Asset: true}
	res2, err := ReceiveLogisticsDelivery(delID, models.ReceiveLogisticsDeliveryInput{
		Desk:      "perlengkapan",
		PhotoURLs: []string{"/uploads/2026-11/r4.jpg"},
		Lines: []models.ReceiveLogisticsLineInput{{
			DeliveryItemID: diAlatID,
			Qty:            2,
			Destination:    "aset",
			TrackingMode:   "tunggal",
			Location:       "Dapur Pusat",
		}},
	}, Actor{Name: "Tim Aset"}, asetAccess, nil)
	if err != nil {
		t.Fatalf("Desk 2 ReceiveLogisticsDelivery: %v", err)
	}
	if res2.DeliveryStatus != "received" || res2.RemainingItems != 0 || res2.AssetsCreated != 2 {
		t.Fatalf("expected received + 2 assets, got %+v", res2)
	}

	// 7. After final received, photo modification MUST BE REJECTED
	pAfter := []string{"/uploads/2026-11/hack.jpg"}
	if _, err := UpdateLogisticsDeliveryPhotos(delID, models.UpdateLogisticsPhotosInput{ReceivePhotoURLs: &pAfter}, true, true, nil); err == nil {
		t.Fatal("expected photo edit after received to be rejected")
	}

	// 8. Cancel delivery releases reserved quota back to PR
	delCancel, err := CreateLogisticsDelivery(models.CreateLogisticsDeliveryInput{
		DispatchMode: "logistics",
		DispatchNow:  true,
		CourierName:  "Kurir Uji",
		Items:        []models.LogisticsDeliveryItemInput{{PurchaseRequestID: prCancel, PRItemKey: keyCancel, QtyPlanned: 5}},
	}, Actor{Name: "Purchasing"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateLogisticsDelivery cancel test: %v", err)
	}
	delCancelID = delCancel.ID
	dBefore, _ := BuildReceivingDraft(prCancel, nil)
	if dBefore.Lines[0].AvailableDirect != 0 {
		t.Fatalf("expected AvailableDirect=0 while in_transit, got %d", dBefore.Lines[0].AvailableDirect)
	}
	if _, err := UpdateLogisticsDeliveryStatus(delCancelID, models.UpdateLogisticsStatusInput{Status: "cancelled", CancelReason: "batal"}, Actor{Name: "Purchasing"}, nil); err != nil {
		t.Fatalf("cancel delivery: %v", err)
	}
	dAfter, _ := BuildReceivingDraft(prCancel, nil)
	if dAfter.Lines[0].AvailableDirect != 5 {
		t.Fatalf("expected AvailableDirect=5 after cancel, got %d", dAfter.Lines[0].AvailableDirect)
	}
}
