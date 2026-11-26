package services

import (
	"fmt"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestListStockItemsCaseInsensitiveOrderPaginationAndWarehouseQty(t *testing.T) {
	requireDB(t)

	prefix := "UJI-SORT-" + NewULID()[:8] + "-"
	whID := NewULID()
	if _, err := database.DB.Exec(
		`INSERT INTO warehouses (id, code, name, type, is_active) VALUES ($1, $2, $3, 'central', true)`,
		whID, "WH-"+NewULID()[:6], "Gudang Uji Sort "+prefix,
	); err != nil {
		t.Fatalf("insert warehouse: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id = $1`, whID)
		database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id = $1`, whID)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id = $1`, whID)
		database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, whID)
	})

	// Mix uppercase and lowercase names under the same searchable prefix.
	// With case-sensitive ORDER BY name ASC, lowercase "alpha" would sort AFTER "Zebra".
	names := []string{
		prefix + "Zebra",
		prefix + "alpha",
		prefix + "Beta",
		prefix + "soju lychee",
	}
	ids := make([]string, len(names))
	for i, nm := range names {
		id := NewULID()
		ids[i] = id
		if _, err := database.DB.Exec(
			`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, is_active) VALUES ($1,$2,$3,'Botol','Dus',20,true)`,
			id, fmt.Sprintf("TS%d%s", i, NewULID()[:5]), nm,
		); err != nil {
			t.Fatalf("insert stock_item %s: %v", nm, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			database.DB.Exec(`DELETE FROM stock_movements WHERE item_id = $1`, id)
			database.DB.Exec(`DELETE FROM stock_batches WHERE item_id = $1`, id)
			database.DB.Exec(`DELETE FROM stock_ledger WHERE item_id = $1`, id)
			database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, id)
		}
	})

	// Add stock for "soju lychee" in whID
	if err := CreateAdjustment(models.AdjustmentRequest{
		ItemID: ids[3], WarehouseID: whID, QtyBase: 40, CostPerBase: 50000, MovType: "purchase_in",
	}, "uji"); err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}

	// Paginate 2 per page across 2 pages; verify case-insensitive order and zero overlap/gaps.
	p1, total1, err := ListStockItems(prefix, true, whID, nil, 1, 2)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	p2, total2, err := ListStockItems(prefix, true, whID, nil, 2, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if total1 != 4 || total2 != 4 {
		t.Fatalf("expected total=4, got %d and %d", total1, total2)
	}
	all := append(p1, p2...)
	if len(all) != 4 {
		t.Fatalf("expected 4 items across pages, got %d", len(all))
	}
	wantOrder := []string{
		prefix + "alpha",
		prefix + "Beta",
		prefix + "soju lychee",
		prefix + "Zebra",
	}
	for i, want := range wantOrder {
		if all[i].Name != want {
			t.Fatalf("index %d: got %q, want %q", i, all[i].Name, want)
		}
	}
	if all[2].WarehouseQty != 40 {
		t.Fatalf("soju lychee warehouse_qty: got %v, want 40", all[2].WarehouseQty)
	}
	if all[0].WarehouseQty != 0 {
		t.Fatalf("alpha warehouse_qty: got %v, want 0", all[0].WarehouseQty)
	}
}
