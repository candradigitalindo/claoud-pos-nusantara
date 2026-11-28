package services

import (
	"math"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestStockItemSameUnitRatioValidation(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	// Reject same base_unit & dist_unit with dist_ratio > 1
	if _, err := CreateStockItem(models.StockItemRequest{
		Name:      "Uji Garpu Rasio Salah " + NewULID()[20:],
		BaseUnit:  "Pack",
		DistUnit:  "Pack",
		DistRatio: 100,
	}, "uji"); err == nil {
		t.Fatal("expected error when dist_unit == base_unit and dist_ratio == 100")
	}

	// Accept valid distinct units with ratio 24
	item, err := CreateStockItem(models.StockItemRequest{
		Name:      "Uji Bir Valid " + NewULID()[20:],
		BaseUnit:  "Botol",
		DistUnit:  "Dus",
		DistRatio: 24,
	}, "uji")
	if err != nil {
		t.Fatalf("CreateStockItem valid: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, item.ID)
	})

	// Reject update making dist_unit == base_unit while keeping dist_ratio == 24
	if _, err := UpdateStockItem(item.ID, models.StockItemRequest{
		DistUnit: "botol",
		Present:  map[string]bool{"dist_unit": true},
	}, "uji"); err == nil {
		t.Fatal("expected error when updating dist_unit to match base_unit with dist_ratio=24")
	}
}

func TestWarehouseDashboardUnitsUTCCostAndScope(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	outletA := NewULID()
	outletB := NewULID()
	whCentral := NewULID()
	whA := NewULID()
	whB := NewULID()
	itemID := NewULID()
	inactiveID := NewULID()

	for _, o := range []struct{ id, code, name, slug string }{
		{outletA, "UOA" + outletA[21:], "Uji Outlet A", "uji-oa-" + outletA[20:]},
		{outletB, "UOB" + outletB[21:], "Uji Outlet B", "uji-ob-" + outletB[20:]},
	} {
		if _, err := database.DB.Exec(
			`INSERT INTO outlets (id, code, name, slug, api_key, is_active) VALUES ($1,$2,$3,$4,$5,true)`,
			o.id, o.code, o.name, o.slug, "key-"+o.id,
		); err != nil {
			t.Fatalf("insert outlet: %v", err)
		}
	}
	if _, err := database.DB.Exec(
		`INSERT INTO warehouses (id, code, name, type, outlet_id, is_active) VALUES
		 ($1,$2,'Gudang Induk Uji','central',NULL,true),
		 ($3,$4,'Gudang Outlet A Uji','outlet',$5,true),
		 ($6,$7,'Gudang Outlet B Uji','outlet',$8,true)`,
		whCentral, "WHC"+whCentral[22:],
		whA, "WHA"+whA[22:], outletA,
		whB, "WHB"+whB[22:], outletB,
	); err != nil {
		t.Fatalf("insert warehouses: %v", err)
	}
	if _, err := database.DB.Exec(
		`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, min_stock, is_active) VALUES
		 ($1,$2,'Bir Uji Dashboard','Botol','Dus',24,10,true),
		 ($3,$4,'Item Nonaktif Uji','Pcs','Pcs',1,10,false)`,
		itemID, "BIR"+itemID[20:],
		inactiveID, "INA"+inactiveID[20:],
	); err != nil {
		t.Fatalf("insert stock_items: %v", err)
	}

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM goods_receipt_items WHERE item_id IN ($1,$2)`, itemID, inactiveID)
		database.DB.Exec(`DELETE FROM goods_receipts WHERE warehouse_id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM stock_transfer_items WHERE item_id IN ($1,$2)`, itemID, inactiveID)
		database.DB.Exec(`DELETE FROM stock_transfers WHERE from_warehouse_id IN ($1,$2,$3) OR to_warehouse_id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM stock_items WHERE id IN ($1,$2)`, itemID, inactiveID)
		database.DB.Exec(`DELETE FROM warehouses WHERE id IN ($1,$2,$3)`, whCentral, whA, whB)
		database.DB.Exec(`DELETE FROM outlets WHERE id IN ($1,$2)`, outletA, outletB)
	})

	// 1. GoodsReceipt 1 Dus (24 Botol) @ 480.000/Dus = 20.000/Botol in Central
	priceDus := 480000.0
	gr, err := CreateGoodsReceipt(models.GoodsReceiptRequest{
		WarehouseID: whCentral,
		VendorName:  "Vendor Uji",
		Items: []models.GoodsReceiptItemReq{{
			ItemID:      itemID,
			QtyDist:     1,
			UnitUsed:    "dus", // case-insensitive check
			CostPerUnit: &priceDus,
		}},
	}, "uji")
	if err != nil {
		t.Fatalf("CreateGoodsReceipt: %v", err)
	}
	if gr.Items[0].QtyBase != 24 || gr.Items[0].UnitUsed != "Dus" || math.Abs(gr.Items[0].CostPerBase-20000) > 0.01 {
		t.Fatalf("unexpected GRN conversion: %+v", gr.Items[0])
	}

	// 2. Verify Dashboard recent movement carries BaseUnit="Botol", UnitUsed="Dus", QtyDist=1, QtyBase=24, and UTC Z timestamp
	dashAll, err := GetWarehouseDashboard(nil)
	if err != nil {
		t.Fatalf("GetWarehouseDashboard(nil): %v", err)
	}
	var found *models.StockMovement
	for i := range dashAll.RecentMovements {
		if dashAll.RecentMovements[i].ItemID == itemID && dashAll.RecentMovements[i].WarehouseID == whCentral {
			found = &dashAll.RecentMovements[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected recent movement for Bir Uji Dashboard")
	}
	if found.QtyBase != 24 || found.QtyDist != 1 || found.UnitUsed != "Dus" || found.BaseUnit != "Botol" || found.DistUnit != "Dus" || found.DistRatio != 24 {
		t.Fatalf("dashboard movement unit metadata mismatch: %+v", found)
	}
	if !strings.HasSuffix(found.CreatedAt, "Z") {
		t.Fatalf("dashboard movement CreatedAt must be RFC3339 UTC ending in Z, got %q", found.CreatedAt)
	}
	if _, err := time.Parse(time.RFC3339, found.CreatedAt); err != nil {
		t.Fatalf("dashboard movement CreatedAt invalid RFC3339 %q: %v", found.CreatedAt, err)
	}

	// 3. CreateAdjustment in Dus with CostPerBase=0 must preserve UnitUsed="Dus", QtyDist=1, and inherit avg_cost=20000 (not 0!)
	if err := CreateAdjustment(models.AdjustmentRequest{
		ItemID:      itemID,
		WarehouseID: whCentral,
		QtyBase:     24,
		UnitUsed:    "Dus",
		CostPerBase: 0,
		MovType:     "adjustment",
		Notes:       "koreksi masuk 1 dus",
	}, "uji"); err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
	var adjQtyBase, adjQtyDist, adjCost, avgAfter float64
	var adjUnit string
	if err := database.DB.QueryRow(`
		SELECT qty_base, qty_dist, unit_used, cost_per_base
		FROM stock_movements WHERE item_id=$1 AND warehouse_id=$2 AND movement_type='adjustment'
		ORDER BY created_at DESC LIMIT 1`, itemID, whCentral).Scan(&adjQtyBase, &adjQtyDist, &adjUnit, &adjCost); err != nil {
		t.Fatalf("query adj movement: %v", err)
	}
	if adjQtyBase != 24 || adjQtyDist != 1 || adjUnit != "Dus" || math.Abs(adjCost-20000) > 0.01 {
		t.Fatalf("adjustment mismatch: qtyBase=%v qtyDist=%v unit=%q cost=%v", adjQtyBase, adjQtyDist, adjUnit, adjCost)
	}
	if err := database.DB.QueryRow(`SELECT avg_cost FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2`, itemID, whCentral).Scan(&avgAfter); err != nil {
		t.Fatalf("query ledger avg_cost: %v", err)
	}
	if math.Abs(avgAfter-20000) > 0.01 {
		t.Fatalf("avg_cost should stay 20000 after zero-cost positive adjustment, got %v", avgAfter)
	}

	// 4. Seed inactive low stock in whA + exact-at-min stock (10 == min 10) in whA + transfer central -> whB
	if _, err := database.DB.Exec(
		`INSERT INTO stock_ledger (id, item_id, warehouse_id, qty_base, avg_cost, min_stock) VALUES
		 ($1,$2,$3,10,20000,10),
		 ($4,$5,$3,1,1000,10)`,
		NewULID(), itemID, whA, NewULID(), inactiveID,
	); err != nil {
		t.Fatalf("seed ledger whA: %v", err)
	}
	if _, err := CreateStockTransfer(models.StockTransferRequest{
		FromWarehouseID: whCentral,
		ToWarehouseID:   whB,
		Items:           []models.StockTransferItemReq{{ItemID: itemID, QtyDist: 1, UnitUsed: "Dus"}},
	}, "uji"); err != nil {
		t.Fatalf("CreateStockTransfer central->B: %v", err)
	}

	// Outlet A dashboard must NOT leak Central value/movements, must NOT leak Central->B transfer,
	// must NOT count inactive item as low stock, and must NOT count qty==min (10==10) as low stock (< rule)
	dashA, err := GetWarehouseDashboard([]string{outletA})
	if err != nil {
		t.Fatalf("GetWarehouseDashboard(outletA): %v", err)
	}
	if dashA.TotalWarehouses != 1 {
		t.Fatalf("outletA TotalWarehouses: got %d, want 1", dashA.TotalWarehouses)
	}
	if dashA.PendingTransfers != 0 || len(dashA.PendingTransferList) != 0 {
		t.Fatalf("outletA leaked central->B transfer: count=%d list=%d", dashA.PendingTransfers, len(dashA.PendingTransferList))
	}
	if dashA.LowStockCount != 0 || len(dashA.LowStockItems) != 0 {
		t.Fatalf("outletA should have 0 low stock (inactive ignored, 10==10 not < 10): count=%d list=%+v", dashA.LowStockCount, dashA.LowStockItems)
	}
	if math.Abs(dashA.TotalStockValue-200000) > 0.01 {
		t.Fatalf("outletA TotalStockValue should only be active item in whA (10*20000=200000), got %v", dashA.TotalStockValue)
	}
	for _, m := range dashA.RecentMovements {
		if m.WarehouseID == whCentral {
			t.Fatalf("outletA leaked central warehouse movement: %+v", m)
		}
	}
}

func TestWarehouseDashboardExpiring30DaysAndWarehouseFilter(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	wh1 := NewULID()
	wh2 := NewULID()
	item1 := NewULID()
	item2 := NewULID()
	bExpired := NewULID()
	bCrit := NewULID()
	bWarn := NewULID()
	bFar := NewULID()
	bZero := NewULID()

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_batches WHERE id IN ($1,$2,$3,$4,$5)`, bExpired, bCrit, bWarn, bFar, bZero)
		database.DB.Exec(`DELETE FROM stock_items WHERE id IN ($1,$2)`, item1, item2)
		database.DB.Exec(`DELETE FROM warehouses WHERE id IN ($1,$2)`, wh1, wh2)
	})

	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id, is_active) VALUES
		($1, $2, 'Gudang Exp Uji 1', 'central', NULL, true),
		($3, $4, 'Gudang Exp Uji 2', 'central', NULL, true)`,
		wh1, "GE1-"+wh1[20:], wh2, "GE2-"+wh2[20:]); err != nil {
		t.Fatalf("insert warehouses: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, is_active) VALUES
		($1, $2, 'Susu Exp Uji', 'Botol', 'Dus', 12, true),
		($3, $4, 'Keju Exp Uji', 'Pack', 'Pack', 1, true)`,
		item1, "SE1-"+item1[20:], item2, "KE2-"+item2[20:]); err != nil {
		t.Fatalf("insert items: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO stock_batches (id, item_id, warehouse_id, qty_base, cost_per_base, expiry_date, ref_type) VALUES
		($1, $6, $8, 12, 10000, tz_today() - 3, 'goods_receipt'),
		($2, $6, $8, 24, 10000, tz_today() + 5, 'goods_receipt'),
		($3, $7, $9, 5,  20000, tz_today() + 20, 'goods_receipt'),
		($4, $7, $8, 10, 20000, tz_today() + 45, 'goods_receipt'),
		($5, $7, $8, 0,  20000, tz_today() + 2, 'goods_receipt')`,
		bExpired, bCrit, bWarn, bFar, bZero, item1, item2, wh1, wh2); err != nil {
		t.Fatalf("insert batches: %v", err)
	}

	// Filter khusus wh1: hanya bExpired (-3 hari) + bCrit (+5 hari), tidak termasuk bWarn (wh2), bFar (+45 hari), bZero (qty 0).
	d1, err := GetWarehouseDashboardFiltered(nil, wh1)
	if err != nil {
		t.Fatalf("GetWarehouseDashboardFiltered wh1: %v", err)
	}
	if d1.ExpiredCount != 1 || d1.Expiring7dCount != 1 || d1.Expiring30dCount != 2 {
		t.Fatalf("wh1 expiry counts mismatch: expired=%d exp7=%d exp30=%d", d1.ExpiredCount, d1.Expiring7dCount, d1.Expiring30dCount)
	}
	if math.Abs(d1.Expiring30dValue-360000) > 0.01 {
		t.Fatalf("wh1 expiring30d value: got %.2f, want 360000", d1.Expiring30dValue)
	}
	if len(d1.ExpiringItems) != 2 || d1.ExpiringItems[0].Status != "expired" || d1.ExpiringItems[1].Status != "critical" {
		t.Fatalf("wh1 expiring items unexpected: %+v", d1.ExpiringItems)
	}
	// WarehouseStocks harus tetap memuat wh2 agar filter gudang di UI tetap lengkap.
	foundWh2 := false
	for _, ws := range d1.WarehouseStocks {
		if ws.WarehouseID == wh2 {
			foundWh2 = true
			if ws.Expiring30dCount != 1 {
				t.Fatalf("wh2 Expiring30dCount in WarehouseStocks: got %d, want 1", ws.Expiring30dCount)
			}
		}
	}
	if !foundWh2 {
		t.Fatalf("WarehouseStocks should keep listing all accessible warehouses when filtering by wh1")
	}

	// Filter khusus wh2: hanya bWarn (+20 hari, status=warning)
	d2, err := GetWarehouseDashboardFiltered(nil, wh2)
	if err != nil {
		t.Fatalf("GetWarehouseDashboardFiltered wh2: %v", err)
	}
	if d2.ExpiredCount != 0 || d2.Expiring7dCount != 0 || d2.Expiring30dCount != 1 || len(d2.ExpiringItems) != 1 || d2.ExpiringItems[0].Status != "warning" {
		t.Fatalf("wh2 expiring unexpected: count=%d items=%+v", d2.Expiring30dCount, d2.ExpiringItems)
	}
}
