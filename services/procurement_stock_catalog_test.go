package services

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/database"
)

func TestProcurementStockCatalogKeepsMainWarehouseBalancesSeparate(t *testing.T) {
	requireDB(t)
	itemID := recvStockItem(t, "Bahan katalog pengadaan "+NewULID())
	first, second, inactive := recvWarehouse(t), recvWarehouse(t), recvWarehouse(t)
	if _, err := database.DB.Exec(`UPDATE warehouses SET is_active = false WHERE id = $1`, inactive); err != nil {
		t.Fatal(err)
	}
	for warehouseID, quantity := range map[string]float64{first: 8000, second: 23000, inactive: 99000} {
		if _, err := database.DB.Exec(`INSERT INTO stock_ledger (id, item_id, warehouse_id, qty_base, avg_cost) VALUES ($1,$2,$3,$4,125)`, NewULID(), itemID, warehouseID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := ListProcurementStockCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	var found *ProcurementStockItem
	for i := range catalog.Items {
		if catalog.Items[i].ID == itemID {
			found = &catalog.Items[i]
			break
		}
	}
	if found == nil || found.StockByWarehouse[first] != 8000 || found.StockByWarehouse[second] != 23000 {
		t.Fatalf("separate warehouse balances missing: %+v", found)
	}
	if _, exists := found.StockByWarehouse[inactive]; exists {
		t.Fatal("inactive central warehouse balance exposed")
	}
	if found.Unit != "kg" || found.BaseUnit != "gr" || found.PurchaseRatio != 1000 {
		t.Fatalf("purchase-unit conversion missing: %+v", found)
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "avg_cost") || strings.Contains(string(data), "total_value") {
		t.Fatal("procurement quantity lookup exposed inventory valuation")
	}
	scoped, err := ListProcurementStockCatalog([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.Warehouses) != 0 {
		t.Fatal("empty outlet scope exposed central warehouses")
	}
	for _, item := range scoped.Items {
		if len(item.StockByWarehouse) != 0 {
			t.Fatal("empty outlet scope exposed central warehouse balances")
		}
	}
}
