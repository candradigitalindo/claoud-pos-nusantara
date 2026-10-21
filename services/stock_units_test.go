package services

import (
	"math"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestStockUnitConversion(t *testing.T) {
	for _, tc := range []struct {
		name, base, pack, selected string
		ratio, qty, want           float64
	}{
		{"dus ke botol", "botol", "dus", "dus", 24, 2, 48},
		{"botol tetap botol", "botol", "dus", "botol", 24, 3, 3},
		{"klien lama memakai kemasan", "botol", "dus", "", 24, 2, 48},
		{"kemasan berbeda per barang", "botol", "dus", "dus", 12, 2, 24},
		{"kg ke gram", "gram", "kg", "kg", 1000, 0.25, 250},
		{"isi pecahan", "liter", "botol", "botol", 0.6, 2, 1.2},
		{"tanpa kemasan", "pcs", "", "pcs", 1, 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := stockQuantityToBase(tc.qty, tc.selected, tc.base, tc.pack, tc.ratio)
			if err != nil || math.Abs(got-tc.want) > 0.0001 {
				t.Fatalf("konversi = %v, %v; mau %v", got, err, tc.want)
			}
		})
	}
	for _, qty := range []float64{-1, 0, math.NaN(), math.Inf(1)} {
		if _, _, err := stockQuantityToBase(qty, "botol", "botol", "dus", 24); err == nil {
			t.Fatalf("jumlah tidak valid diterima: %v", qty)
		}
	}
	if _, _, err := stockQuantityToBase(1, "kg", "botol", "dus", 24); err == nil {
		t.Fatal("satuan asing diterima")
	}
}

// Inventory is shared in base units; warehouses choose how quantities are entered.
func unitWarehouses(t *testing.T) (outlet, central *bdFixture, item string) {
	t.Helper()
	requireDB(t)
	outlet, central = newBreakdownFixture(t), newBreakdownFixture(t)
	if _, err := database.DB.Exec(`UPDATE warehouses SET type='central', outlet_id=NULL WHERE id=$1`, central.wh); err != nil {
		t.Fatal(err)
	}
	item = outlet.item(t, "Cleo", "botol", "dus", 24)
	t.Cleanup(func() {
		execScratch(`DELETE FROM goods_receipt_items WHERE receipt_id IN (SELECT id FROM goods_receipts WHERE warehouse_id IN ($1,$2))`, outlet.wh, central.wh)
		execScratch(`DELETE FROM goods_receipts WHERE warehouse_id IN ($1,$2)`, outlet.wh, central.wh)
		execScratch(`DELETE FROM stock_transfer_items WHERE transfer_id IN (SELECT id FROM stock_transfers WHERE from_warehouse_id IN ($1,$2))`, outlet.wh, central.wh)
		execScratch(`DELETE FROM stock_transfers WHERE from_warehouse_id IN ($1,$2)`, outlet.wh, central.wh)
		execScratch(`DELETE FROM stock_wastes WHERE item_id=$1`, item)
		// Central's fixture is cleaned first; remove its ledgers before its warehouse.
		execScratch(`DELETE FROM stock_batches WHERE warehouse_id=$1`, central.wh)
		execScratch(`DELETE FROM stock_movements WHERE warehouse_id=$1`, central.wh)
		execScratch(`DELETE FROM stock_ledger WHERE warehouse_id=$1`, central.wh)
	})
	return
}

func unitTransfer(t *testing.T, from, to, item, unit string, qty float64) *models.StockTransfer {
	t.Helper()
	doc, err := CreateStockTransfer(models.StockTransferRequest{
		FromWarehouseID: from, ToWarehouseID: to,
		Items: []models.StockTransferItemReq{{ItemID: item, QtyDist: qty, UnitUsed: unit}},
	}, "uji-pengirim")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = UpdateTransferStatus(doc.ID, "approved", "uji-pemberi-persetujuan"); err != nil {
		t.Fatal(err)
	}
	if _, err = UpdateTransferStatusWithPhoto(doc.ID, "sent", "/uploads/uji.jpg", "uji-pengirim"); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestStockUnitsWarehouseLifecycle(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	price := 48000.0
	receipt, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh,
		Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 5, UnitUsed: "dus", CostPerUnit: &price}},
	}, "uji")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Items[0].QtyBase != 120 || receipt.Items[0].CostPerBase != 2000 || receipt.TotalCost != 240000 {
		t.Fatalf("penerimaan 5 dus @48000: %+v", receipt)
	}
	doc := unitTransfer(t, central.wh, outlet.wh, item, "dus", 2)
	if _, err := UpdateTransferStatus(doc.ID, "received", "uji-penerima"); err != nil {
		t.Fatal(err)
	}
	if qty, cost := outlet.ledger(t, item); qty != 48 || cost != 2000 {
		t.Fatalf("outlet: %v @%v", qty, cost)
	}
	if qty, _ := central.ledger(t, item); qty != 72 {
		t.Fatalf("pusat: %v", qty)
	}

	err = CreateStockWaste(models.StockWasteRequest{WarehouseID: outlet.wh, ItemID: item, QtyDist: 1, UnitUsed: "botol", Reason: "damaged"}, "uji")
	if err != nil {
		t.Fatal(err)
	}
	var wasteQty, wasteCost float64
	if err := database.DB.QueryRow(`SELECT qty_base, total_cost FROM stock_wastes WHERE item_id=$1`, item).Scan(&wasteQty, &wasteCost); err != nil {
		t.Fatal(err)
	}
	if wasteQty != 1 || wasteCost != 2000 {
		t.Fatalf("waste 1 botol: %v, biaya %v", wasteQty, wasteCost)
	}
	if qty, _ := outlet.ledger(t, item); qty != 47 {
		t.Fatalf("setelah waste 1 botol: %v", qty)
	}

	// Transfer back loose bottles, without multiplying by the case size.
	doc = unitTransfer(t, outlet.wh, central.wh, item, "botol", 3)
	if doc.Items[0].QtyBase != 3 || doc.Items[0].UnitUsed != "botol" {
		t.Fatalf("transfer botol: %+v", doc.Items)
	}
	if _, err := UpdateTransferStatus(doc.ID, "received", "uji-penerima"); err != nil {
		t.Fatal(err)
	}
	if qty, _ := outlet.ledger(t, item); qty != 44 {
		t.Fatalf("sisa outlet: %v", qty)
	}
	if qty, cost := central.ledger(t, item); qty != 75 || cost != 2000 {
		t.Fatalf("sisa pusat: %v @%v", qty, cost)
	}
	if err := CreateStockWaste(models.StockWasteRequest{WarehouseID: central.wh, ItemID: item, QtyDist: 1, UnitUsed: "dus", Reason: "damaged"}, "uji"); err != nil {
		t.Fatal(err)
	}
	if qty, _ := central.ledger(t, item); qty != 51 {
		t.Fatalf("waste 1 dus: %v", qty)
	}

	price = 2000
	if _, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh,
		Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 1, UnitUsed: "botol", CostPerUnit: &price}},
	}, "uji"); err != nil {
		t.Fatal(err)
	}
	if qty, cost := central.ledger(t, item); qty != 52 || cost != 2000 {
		t.Fatalf("penerimaan botol: %v @%v", qty, cost)
	}
}

func TestStockUnitsTransferKeepsOriginalConversion(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	central.stockIn(t, item, 100, 2000, "")
	doc := unitTransfer(t, central.wh, outlet.wh, item, "dus", 2)
	if _, err := database.DB.Exec(`UPDATE stock_items SET dist_ratio=12 WHERE id=$1`, item); err != nil {
		t.Fatal(err)
	}
	if err := UpdateTransferReceivedQty(doc.ID, item, 36); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateTransferStatus(doc.ID, "received", "uji-penerima"); err != nil {
		t.Fatal(err)
	}
	var qtyBase, qtyEntered float64
	var unit string
	if err := database.DB.QueryRow(`SELECT qty_base, qty_dist, unit_used FROM stock_movements WHERE ref_id=$1 AND movement_type='transfer_in'`, doc.ID).Scan(&qtyBase, &qtyEntered, &unit); err != nil {
		t.Fatal(err)
	}
	if qtyBase != 36 || qtyEntered != 1.5 || unit != "dus" {
		t.Fatalf("penerimaan harus memakai isi 24: %v/%v %s", qtyBase, qtyEntered, unit)
	}
	if err := database.DB.QueryRow(`SELECT qty_base, qty_dist, unit_used FROM stock_wastes WHERE item_id=$1`, item).Scan(&qtyBase, &qtyEntered, &unit); err != nil {
		t.Fatal(err)
	}
	if qtyBase != 12 || qtyEntered != 0.5 || unit != "dus" {
		t.Fatalf("selisih harus memakai isi 24: %v/%v %s", qtyBase, qtyEntered, unit)
	}
	// Legacy clients omit unit_used, so their input still means distribution units.
	legacy, err := CreateStockTransfer(models.StockTransferRequest{FromWarehouseID: central.wh, ToWarehouseID: outlet.wh,
		Items: []models.StockTransferItemReq{{ItemID: item, QtyDist: 2}},
	}, "uji")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Items[0].QtyBase != 24 || legacy.Items[0].UnitUsed != "dus" {
		t.Fatalf("klien lama: %+v", legacy.Items)
	}
}

func TestStockUnitsLegacyReceiptAndInvalidUnits(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	receipt, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh,
		Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 2, CostPerBase: 2000}},
	}, "uji")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TotalCost != 96000 || receipt.Items[0].QtyBase != 48 {
		t.Fatalf("GRN klien lama: %+v", receipt)
	}
	if _, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh,
		Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 2, UnitUsed: "kg", CostPerBase: 2000}},
	}, "uji"); err == nil {
		t.Fatal("GRN satuan asing diterima")
	}
	if _, err := CreateStockTransfer(models.StockTransferRequest{FromWarehouseID: central.wh, ToWarehouseID: outlet.wh,
		Items: []models.StockTransferItemReq{{ItemID: item, QtyDist: 2, UnitUsed: "kg"}},
	}, "uji"); err == nil {
		t.Fatal("transfer satuan asing diterima")
	}
	if err := CreateStockWaste(models.StockWasteRequest{WarehouseID: central.wh, ItemID: item, QtyDist: 1, UnitUsed: "kg", Reason: "damaged"}, "uji"); err == nil {
		t.Fatal("waste satuan asing diterima")
	}
	if qty, _ := central.ledger(t, item); qty != 48 {
		t.Fatalf("input ditolak mengubah stok: %v", qty)
	}
	if count := central.count(t, "goods_receipts"); count != 1 {
		t.Fatalf("GRN invalid tidak rollback: %d", count)
	}
	zero := 0.0
	receipt, err = CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh,
		Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 1, UnitUsed: "botol", CostPerBase: 9000, CostPerUnit: &zero}},
	}, "uji")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TotalCost != 0 {
		t.Fatalf("harga per unit nol harus mengalahkan fallback: %v", receipt.TotalCost)
	}
}
