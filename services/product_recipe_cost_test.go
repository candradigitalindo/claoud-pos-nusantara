package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
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

func TestProductRecipeMultipleCostingAndUsageControl(t *testing.T) {
	f := newProductRecipeFixture(t)
	r1, r2 := costTestRecipe(t, f.itemID, 2), costTestRecipe(t, f.itemID, 3)
	links := []models.ProductRecipeLink{{RecipeMasterID: r1, Multiplier: 1}, {RecipeMasterID: r2, Multiplier: .5}}
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "Menu gabungan", Recipes: &links})
	if err != nil {
		t.Fatal(err)
	}
	f.stockIn(t, f.itemID, 100, 1000, "")
	costing, err := GetFnbCosting(f.outlet, []string{f.outlet})
	if err != nil || len(costing) != 1 || len(costing[0].Ingredients) != 1 || costing[0].Ingredients[0].QtyBase != 3.5 {
		t.Fatalf("F&B combined ingredients: %+v %v", costing, err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM pos_stock_deductions WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM stock_deduction_failures WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM transaction_payments WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM cloud_transactions WHERE outlet_id=$1`, f.outlet)
	})
	if _, err := SaveTransaction(f.outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "COSTTEST", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), TotalAmount: 100000, PaymentMethod: "cash", Items: []models.TransactionItem{{ProductID: p.LocalID, ProductName: p.Name, Quantity: 4, Price: 25000, Subtotal: 100000}}}); err != nil {
		t.Fatal(err)
	}
	var day string
	if err := database.DB.QueryRow(`SELECT tz_today()::text`).Scan(&day); err != nil {
		t.Fatal(err)
	}
	report, err := GetPpicUsageControl(day, day, f.wh, "", "", []string{f.outlet}, 1, 0)
	if err != nil || len(report.Rows) != 1 || report.Rows[0].SaleQty != 14 || report.Rows[0].ReferenceQty != 14 {
		t.Fatalf("usage from paid combined recipe snapshot: %+v %v", report, err)
	}
}
