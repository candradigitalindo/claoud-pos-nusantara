package handlers

import (
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Pemotongan: template = katalog global (hanya role semua outlet); transaksi
// role per outlet hanya di gudang outletnya sendiri, Gudang Induk hanya role
// semua outlet.
func TestStockBreakdownScopeRules(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, central := stockFlowUsers(t, f, "recipes.update", "recipes.view", "stockledger.adjust", "stockledger.view")

	// Item bahan = f.item (gr, distribusi kg); satu hasil potongan.
	sfx := strings.ToLower(services.NewULID()[16:])
	part := services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio) VALUES ($1, $2, $3, 'gr', 'gr', 1)`,
		part, "BRP"+strings.ToUpper(sfx), "Beras Pecah Uji "+sfx); err != nil {
		t.Fatalf("item hasil: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_breakdown_outputs WHERE item_id = $1`, part)
		database.DB.Exec(`DELETE FROM stock_breakdown_patterns WHERE input_item_id = $1`, f.item)
		database.DB.Exec(`DELETE FROM stock_item_breakdowns WHERE input_item_id = $1 OR output_item_id = $1`, f.item)
		database.DB.Exec(`DELETE FROM stock_batches WHERE item_id = $1`, part)
		database.DB.Exec(`DELETE FROM stock_movements WHERE item_id = $1`, part)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE item_id = $1`, part)
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, part)
	})
	template := map[string]interface{}{"patterns": []map[string]interface{}{{
		"name": "Pecah", "outputs": []map[string]interface{}{{"output_item_id": part, "qty_per_input": 900, "cost_weight": 1}},
	}}}
	if code, _ := doJSON(t, outlet, "PUT", "/stock-items/"+f.item+"/breakdown", template); code != fiber.StatusForbidden {
		t.Errorf("role outlet menyimpan template pemotongan: %d, mau 403", code)
	}
	if code, res := doJSON(t, central, "PUT", "/stock-items/"+f.item+"/breakdown", template); code != fiber.StatusOK {
		t.Fatalf("Gudang Induk menyimpan template: %d %q", code, res.Error)
	}

	for _, w := range []string{f.whA, f.whB} {
		if err := services.CreateAdjustment(models.AdjustmentRequest{ItemID: f.item, WarehouseID: w, QtyBase: 2000, CostPerBase: 10, MovType: "purchase_in"}, "uji"); err != nil {
			t.Fatalf("stok: %v", err)
		}
	}
	body := func(wh string) map[string]interface{} {
		return map[string]interface{}{"warehouse_id": wh, "input_item_id": f.item, "input_qty_dist": 1,
			"outputs": []map[string]interface{}{{"item_id": part, "qty_base": 850}}}
	}
	code, res := doJSON(t, outlet, "POST", "/stock-breakdowns", body(f.whA))
	if code != fiber.StatusCreated {
		t.Fatalf("outlet memotong di gudangnya sendiri: %d %q", code, res.Error)
	}
	var doc models.StockBreakdown
	decodeData(t, res, &doc)
	if doc.WarehouseID != f.whA || doc.PatternName != "Pecah" || len(doc.Outputs) != 1 || doc.Outputs[0].QtyBase != 850 || !strings.HasPrefix(doc.BreakdownNumber, "POT-") {
		t.Errorf("dokumen: %+v", doc)
	}
	if code, _ := doJSON(t, outlet, "POST", "/stock-breakdowns", body(f.whB)); code != fiber.StatusForbidden {
		t.Errorf("outlet memotong di gudang outlet lain: %d, mau 403", code)
	}
	if code, _ := doJSON(t, outlet, "POST", "/stock-breakdowns", body(f.whC)); code != fiber.StatusForbidden {
		t.Errorf("outlet memotong di Gudang Induk: %d, mau 403", code)
	}
	if code, res := doJSON(t, central, "POST", "/stock-breakdowns", body(f.whC)); code != fiber.StatusCreated {
		t.Errorf("Gudang Induk memotong di Gudang Induk: %d %q", code, res.Error)
	}
	if got := ledgerQty(part, f.whB); got != 0 {
		t.Errorf("stok hasil di gudang outlet B %.2f, mau 0", got)
	}

	// Riwayat role outlet: hanya dokumen gudang outletnya.
	code, res = doJSON(t, outlet, "GET", "/stock-breakdowns?limit=100", nil)
	if code != fiber.StatusOK {
		t.Fatalf("riwayat outlet: %d %q", code, res.Error)
	}
	var list []models.StockBreakdown
	decodeData(t, res, &list)
	seen := map[string]bool{}
	for _, d := range list {
		seen[d.WarehouseID] = true
		if d.WarehouseID != f.whA {
			t.Errorf("riwayat outlet memuat dokumen gudang %s", d.WarehouseName)
		}
	}
	if !seen[f.whA] {
		t.Errorf("riwayat outlet tidak memuat dokumen gudangnya")
	}
	code, res = doJSON(t, outlet, "GET", "/stock-breakdowns?warehouse_id="+f.whC, nil)
	decodeData(t, res, &list)
	if code != fiber.StatusOK || len(list) != 0 {
		t.Errorf("riwayat outlet untuk Gudang Induk: %d, %d dokumen, mau 0", code, len(list))
	}
}
