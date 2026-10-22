package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func costTestRecipe(t *testing.T, item string, qty float64) string {
	t.Helper()
	id, err := SaveRecipeMaster("", models.RecipeMasterRequest{Name: "HPP " + NewULID(), Visibility: "public", Items: []models.RecipeItemReq{{ItemID: item, QtyBase: qty}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM recipe_masters WHERE id=$1`, id) })
	return id
}

func TestProductRecipeMultipleCostsSnapshotsAndFIFO(t *testing.T) {
	f := newProductRecipeFixture(t)
	central := newBreakdownFixture(t)
	if _, err := database.DB.Exec(`UPDATE warehouses SET type='central',outlet_id=NULL WHERE id=$1`, central.wh); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, table := range []string{"stock_batches", "stock_movements", "stock_ledger"} {
			execScratch(`DELETE FROM `+table+` WHERE warehouse_id=$1`, central.wh)
		}
	})
	central.stockIn(t, f.itemID, 100, 10000, "")
	f.stockIn(t, f.itemID, 100, 10000, "")
	r1, r2 := costTestRecipe(t, f.itemID, 2), costTestRecipe(t, f.itemID, 1)
	links := []models.ProductRecipeLink{{RecipeMasterID: r1, Multiplier: .5}, {RecipeMasterID: r2, Multiplier: 1.5}}
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "Menu gabungan", Price: 50000, Recipes: &links})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Recipes) != 2 {
		t.Fatalf("create lost recipes: %+v", p)
	}
	check := func(current, reference float64) {
		t.Helper()
		rows, err := GetFnbCosting(f.outlet, []string{f.outlet})
		if err != nil || len(rows) != 1 {
			t.Fatalf("costing: %v %+v", err, rows)
		}
		r := rows[0]
		if r.CostStatus != "ready" || len(r.Recipes) != 2 || len(r.Ingredients) != 1 || !bdNear(r.MaterialCost, current) || r.ReferenceMaterialCost == nil || !bdNear(*r.ReferenceMaterialCost, reference) {
			t.Fatalf("costs: %+v", r)
		}
	}
	check(25000, 25000) // (2 × .5 + 1 × 1.5) × Rp10.000.
	initial, err := GetRecipeMaster(r1)
	if err != nil {
		t.Fatal(err)
	}
	if initial.InitialCost == nil || initial.InitialCost.Total == nil || *initial.InitialCost.Total != 20000 {
		t.Fatalf("initial: %+v", initial.RecipeCost)
	}
	// Outlet purchase price rises, central falls; current costing follows the outlet.
	f.stockIn(t, f.itemID, 100, 12000, "")
	central.stockIn(t, f.itemID, 100, 8000, "")
	check(27500, 25000)
	detail, err := GetProductRecipeDetail(f.outlet, r1)
	if err != nil || detail.CurrentCost == nil || *detail.CurrentCost != 22000 || !reflect.DeepEqual(detail.InitialCost, initial.InitialCost) {
		t.Fatalf("picker: %+v %v", detail, err)
	}
	centralDetail, err := GetRecipeMaster(r1)
	if err != nil || centralDetail.CurrentCost == nil || *centralDetail.CurrentCost != 18000 {
		t.Fatalf("central: %+v %v", centralDetail, err)
	}
	// Shared ingredients produce a single movement. FIFO crosses two batches.
	ref := NewULID()
	if err := DeductStockByRecipe(f.outlet, p.LocalID, 50, ref, "uji gabungan"); err != nil {
		t.Fatal(err)
	}
	var count int
	var qty, value float64
	if err := database.DB.QueryRow(`SELECT COUNT(*),SUM(-qty_base),SUM(-qty_base*cost_per_base) FROM stock_movements WHERE ref_id=$1`, ref).Scan(&count, &qty, &value); err != nil {
		t.Fatal(err)
	}
	if count != 1 || qty != 125 || math.Abs(value-1300000) > .01 {
		t.Fatalf("movement: %d %v %v", count, qty, value)
	}
	f.stockIn(t, f.itemID, 75, 5000, "")
	var after float64
	if err := database.DB.QueryRow(`SELECT -qty_base*cost_per_base FROM stock_movements WHERE ref_id=$1`, ref).Scan(&after); err != nil || after != value {
		t.Fatalf("historical cost changed: %v %v", after, err)
	}
	// Editing quantities changes the current composition, never the creation reference.
	if _, err := SaveRecipeMaster(r1, models.RecipeMasterRequest{Name: "Resep diubah", Visibility: "public", Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 4}}}); err != nil {
		t.Fatal(err)
	}
	afterEdit, err := GetRecipeMaster(r1)
	if err != nil || !reflect.DeepEqual(afterEdit.InitialCost, initial.InitialCost) || afterEdit.CurrentCost == nil || *afterEdit.CurrentCost != 36000 {
		t.Fatalf("immutable snapshot: %+v %v", afterEdit, err)
	}
	check(29750, 25000) // remaining batches average Rp8.500; 4 × .5 + 1 × 1.5.
}

func TestProductRecipeMultipleValidationCompatibilityAndScope(t *testing.T) {
	f := newProductRecipeFixture(t)
	r1, r2 := f.recipe(t, "public", false, 2, false), f.recipe(t, "secret", true, 3, false)
	denied := f.recipe(t, "secret", false, 1, false)
	p := f.product(t, r1)
	links := []models.ProductRecipeLink{{RecipeMasterID: r1, Multiplier: 1}, {RecipeMasterID: r2, Multiplier: .5}}
	update := func(ls *[]models.ProductRecipeLink, legacy *string) error {
		return AdminUpdateProduct(p.ID, models.AdminUpdateProductRequest{Name: p.Name, Code: p.Code, Price: p.Price, Recipes: ls, RecipeMasterID: legacy})
	}
	read := func() []models.ProductRecipeLink {
		t.Helper()
		ps, _, err := GetProducts(f.outlet, 1, 100)
		if err != nil || len(ps) != 1 {
			t.Fatalf("read: %v %+v", err, ps)
		}
		return ps[0].Recipes
	}
	if err := update(&links, nil); err != nil {
		t.Fatal(err)
	}
	before := read()
	for _, invalid := range [][]models.ProductRecipeLink{
		{{RecipeMasterID: r1, Multiplier: 1}, {RecipeMasterID: r1, Multiplier: 1}},
		{{RecipeMasterID: r1, Multiplier: 0}}, {{RecipeMasterID: r1, Multiplier: -1}},
		{{RecipeMasterID: r1, Multiplier: math.NaN()}}, {{RecipeMasterID: r1, Multiplier: .000001}},
		{{RecipeMasterID: r1, Multiplier: 1}, {RecipeMasterID: denied, Multiplier: 1}},
	} {
		if err := update(&invalid, nil); !errors.Is(err, ErrProductRecipe) {
			t.Fatalf("invalid accepted: %+v %v", invalid, err)
		}
		if !reflect.DeepEqual(read(), before) {
			t.Fatal("failed update changed links")
		}
	}
	if err := update(nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveProduct(f.outlet, models.PushProductRequest{LocalID: p.LocalID, Name: p.Name, Code: p.Code, Price: p.Price, Version: 4}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read(), before) {
		t.Fatal("omitted links or POS sync changed links")
	}
	if _, err := database.DB.Exec(`DELETE FROM recipe_outlet_access WHERE recipe_master_id=$1`, r2); err != nil {
		t.Fatal(err)
	}
	rows, err := GetFnbCosting(f.outlet, []string{f.outlet})
	if err != nil || len(rows) != 1 || rows[0].CostStatus != "restricted" || len(rows[0].Recipes) != 0 || rows[0].ReferenceMaterialCost != nil {
		t.Fatalf("second secret recipe leaked: %v %+v", err, rows)
	}
	if err := update(nil, &r1); err != nil {
		t.Fatal(err)
	}
	if got := read(); len(got) != 1 || got[0].RecipeMasterID != r1 {
		t.Fatal("legacy explicit replacement retained old links")
	}
	// Existing databases can still hold a singular link with no junction rows.
	if _, err := database.DB.Exec(`DELETE FROM product_recipe_links WHERE product_id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if got := read(); len(got) != 1 || got[0].Multiplier != 1 {
		t.Fatal("legacy single-recipe fallback lost")
	}
	empty := []models.ProductRecipeLink{}
	if err := update(&empty, nil); err != nil {
		t.Fatal(err)
	}
	if len(read()) != 0 {
		t.Fatal("unlink retained recipes")
	}
}

func TestRecipeCostMissingAndHistoricalReference(t *testing.T) {
	f := newProductRecipeFixture(t)
	r := costTestRecipe(t, f.itemID, 2)
	before, err := GetRecipeMaster(r)
	if err != nil || before.InitialCost == nil || before.InitialCost.Total != nil || before.CurrentCost != nil {
		t.Fatalf("missing cost treated as free: %+v %v", before, err)
	}
	f.stockIn(t, f.itemID, 10, 3000, "")
	d, err := GetProductRecipeDetail(f.outlet, r)
	if err != nil || d.CurrentCost == nil || *d.CurrentCost != 6000 || d.InitialCost.Total != nil {
		t.Fatalf("missing initial price fabricated: %+v %v", d, err)
	}
	if _, err := database.DB.Exec(`UPDATE recipe_masters SET initial_cost_snapshot=NULL WHERE id=$1`, r); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveRecipeMaster(r, models.RecipeMasterRequest{Name: "Resep lama", Visibility: "public", Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 3}}}); err != nil {
		t.Fatal(err)
	}
	d, err = GetProductRecipeDetail(f.outlet, r)
	if err != nil || d.InitialCost != nil || d.CurrentCost == nil || *d.CurrentCost != 9000 {
		t.Fatalf("historical cost backfilled: %+v %v", d, err)
	}
}

func TestRecipeCostUsesBasePriceFromBoxReceipt(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	price := 48000.0
	if _, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: central.wh, Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 1, UnitUsed: "dus", CostPerUnit: &price}}}, "uji"); err != nil {
		t.Fatal(err)
	}
	r := costTestRecipe(t, item, 2)
	d, err := GetProductRecipeDetail(outlet.outlet, r)
	if err != nil || d.CurrentCost == nil || *d.CurrentCost != 4000 || d.InitialCost == nil || d.InitialCost.Total == nil || *d.InitialCost.Total != 4000 {
		t.Fatalf("box price was not split: %+v %v", d, err)
	}
	if len(d.CostIngredients) != 1 || d.CostIngredients[0].CostPerBase != 2000 || d.CostIngredients[0].Unit != "botol" {
		t.Fatalf("unit price: %+v", d.CostIngredients)
	}
}

func TestProductRecipeMultiplePpicConsumers(t *testing.T) {
	f := newProductRecipeFixture(t)
	r1, r2 := costTestRecipe(t, f.itemID, 2), costTestRecipe(t, f.itemID, 3)
	links := []models.ProductRecipeLink{{RecipeMasterID: r1, Multiplier: 1}, {RecipeMasterID: r2, Multiplier: .5}}
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "PPIC gabungan", Recipes: &links})
	if err != nil {
		t.Fatal(err)
	}
	f.stockIn(t, f.itemID, 100, 1000, "")
	hpp, err := GetPpicHppReport(f.outlet, 35, []string{f.outlet})
	if err != nil || len(hpp.Rows) != 1 || len(hpp.Rows[0].Ingredients) != 1 || hpp.Rows[0].Ingredients[0].QtyBase != 3.5 {
		t.Fatalf("PPIC HPP: %+v %v", hpp, err)
	}
	if _, err := database.DB.Exec(`INSERT INTO demand_forecasts(id,outlet_id,product_name,forecast_date,qty_system) VALUES($1,$2,$3,CURRENT_DATE,4)`, NewULID(), f.outlet, p.Name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM mrp_run_items WHERE run_id IN (SELECT id FROM mrp_runs WHERE warehouse_id=$1)`, f.wh)
		execScratch(`DELETE FROM mrp_runs WHERE warehouse_id=$1`, f.wh)
	})
	mrp, err := RunMrp(models.MrpRunRequest{WarehouseID: f.wh, HorizonDays: 1}, "uji", []string{f.outlet})
	if err != nil || len(mrp.Items) != 1 || mrp.Items[0].GrossReq != 14 {
		t.Fatalf("MRP: %+v %v", mrp, err)
	}
	order := NewULID()
	items, _ := json.Marshal([]map[string]any{{"product_name": p.Name, "qty": 4}})
	if _, err := database.DB.Exec(`INSERT INTO cloud_orders(id,local_id,outlet_id,outlet_code,items,total_amount,created_at) VALUES($1::char(26),$1::text,$2,'HPPTEST',$3,100000,NOW() AT TIME ZONE 'UTC')`, order, f.outlet, string(items)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM cloud_orders WHERE id=$1`, order) })
	var day string
	if err := database.DB.QueryRow(`SELECT tz_today()::text`).Scan(&day); err != nil {
		t.Fatal(err)
	}
	variance, err := GetPpicVarianceReport(day, day, f.wh, []string{f.outlet})
	if err != nil || len(variance.Rows) != 1 || variance.Rows[0].TheoQty != 14 || variance.QtySoldWithRecipe != 4 {
		t.Fatalf("variance: %+v %v", variance, err)
	}
	sold, err := GetPpicSoldReport(day, day, f.outlet, "", []string{f.outlet})
	if err != nil || len(sold.Rows) != 1 || !sold.Rows[0].HasRecipe {
		t.Fatalf("coverage: %+v %v", sold, err)
	}
}
