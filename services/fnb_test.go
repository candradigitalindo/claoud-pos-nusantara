package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"testing"
)

func TestFnbCostingScopeCategoriesAndMissingPrices(t *testing.T) {
	f := newProductRecipeFixture(t)
	other := newProductRecipeFixture(t)
	rid := f.recipe(t, "public", false, 10, false)
	p := f.product(t, rid)
	standalone := f.product(t, "")
	f.stockIn(t, f.itemID, 100, 2, "")
	// Another outlet's very high cost must not inflate this outlet's HPP.
	other.stockIn(t, f.itemID, 100, 900, "")
	t.Cleanup(func() {
		execScratch(`DELETE FROM stock_batches WHERE item_id=$1 AND warehouse_id=$2`, f.itemID, other.wh)
		execScratch(`DELETE FROM stock_movements WHERE item_id=$1 AND warehouse_id=$2`, f.itemID, other.wh)
		execScratch(`DELETE FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2`, f.itemID, other.wh)
	})
	other.product(t, "")
	scope := []string{f.outlet}
	profile := models.FnbProfile{MenuKind: "food", TargetCostPct: 30, PackagingCost: 5, OverheadCost: 10}
	if err := SaveFnbProfile(p.ID, profile, scope, "tester"); err != nil {
		t.Fatal(err)
	}
	rows, err := GetFnbCosting(f.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("scope rows: %+v", rows)
	}
	for _, r := range rows {
		if r.ProductID == p.ID && (r.MenuKind != "food" || r.MaterialCost != 20 || r.TotalCost != 35 || r.CostStatus != "ready" || r.SuggestedPrice != 117) {
			t.Fatalf("HPP: %+v", r)
		}
		if r.ProductID == standalone.ID && r.CostStatus != "no_recipe" {
			t.Fatalf("produk mandiri: %+v", r)
		}
	}
	if _, err := GetFnbCosting(other.outlet, scope); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("bypass scope: %v", err)
	}
	if err := SaveFnbProfile(p.ID, profile, []string{other.outlet}, "tester"); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("profile scope: %v", err)
	}
	// Classification/cost settings survive POS sync.
	if _, err := SaveProduct(f.outlet, models.PushProductRequest{LocalID: p.LocalID, Name: p.Name, Price: p.Price, Version: 4}); err != nil {
		t.Fatal(err)
	}
	rows, err = GetFnbCosting(f.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(FilterFnbCosts(rows, "food", "")) != 1 {
		t.Fatal("kategori F&B hilang saat sinkronisasi")
	}
	if _, err := database.DB.Exec(`UPDATE stock_ledger SET avg_cost=0 WHERE item_id=$1 AND warehouse_id=$2`, f.itemID, f.wh); err != nil {
		t.Fatal(err)
	}
	rows, err = GetFnbCosting(f.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ProductID == p.ID && (r.CostStatus != "missing_cost" || r.Margin != 0) {
			t.Fatalf("harga kosong dianggap laba: %+v", r)
		}
	}
	ov, err := GetFnbOverview(f.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	if ov.ProductCount != 2 || ov.FoodCount != 1 || ov.Unclassified != 1 || ov.MissingCost != 1 || ov.NoRecipe != 1 {
		t.Fatalf("dashboard: %+v", ov)
	}
}

func TestFnbDevelopmentWorkflowAndScope(t *testing.T) {
	f := newProductRecipeFixture(t)
	p := f.product(t, "")
	scope := []string{f.outlet}
	d := models.FnbDevelopment{OutletID: f.outlet, Name: "Cleo kemasan", MenuKind: "beverage", TargetPrice: 5000, TargetCostPct: 35, TrialNotes: "Porsi 1 botol; evaluasi kemasan lulus", ProductID: p.ID}
	id, err := SaveFnbDevelopment("", d, scope, "tester")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM fnb_developments WHERE id=$1`, id) })
	read := func() models.FnbDevelopment {
		t.Helper()
		rows, err := ListFnbDevelopments(f.outlet, scope)
		if err != nil || len(rows) != 1 {
			t.Fatalf("read: %v %+v", err, rows)
		}
		return rows[0]
	}
	if read().Status != "idea" {
		t.Fatal("new item not idea")
	}
	if err := TransitionFnbDevelopment(id, "approved", 1, scope, "tester", true); err == nil {
		t.Fatal("melompati review")
	}
	if err := TransitionFnbDevelopment(id, "trial", 1, []string{}, "tester", false); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("scope: %v", err)
	}
	if err := TransitionFnbDevelopment(id, "trial", 1, scope, "tester", false); err != nil {
		t.Fatal(err)
	}
	if err := TransitionFnbDevelopment(id, "review", 1, scope, "tester", false); err == nil {
		t.Fatal("stale version diterima")
	}
	if err := TransitionFnbDevelopment(id, "review", 2, scope, "tester", false); err != nil {
		t.Fatal(err)
	}
	if err := TransitionFnbDevelopment(id, "approved", 3, scope, "tester", false); err == nil {
		t.Fatal("approval tanpa izin")
	}
	if err := TransitionFnbDevelopment(id, "approved", 3, scope, "manager", true); err != nil {
		t.Fatal(err)
	}
	d = read()
	d.Brief = "ubah sesudah approve"
	if _, err := SaveFnbDevelopment(id, d, scope, "tester"); err == nil {
		t.Fatal("isi disetujui bisa diubah")
	}
	if err := TransitionFnbDevelopment(id, "launched", 4, scope, "tester", false); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.Status != "launched" || got.ApprovedBy != "manager" {
		t.Fatalf("workflow: %+v", got)
	}
}

func TestFnbRecipeScopeAndValidation(t *testing.T) {
	f := newProductRecipeFixture(t)
	public := f.recipe(t, "public", false, 1, false)
	secret := f.recipe(t, "secret", true, 1, false)
	private := f.recipe(t, "secret", false, 1, false)
	if !RecipeMasterInScope(secret, []string{f.outlet}) || RecipeMasterInScope(private, []string{f.outlet}) {
		t.Fatal("akses detail resep rahasia salah")
	}
	rows, err := ListRecipeMastersScoped("", "", []string{f.outlet})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, r := range rows {
		found[r.ID] = true
	}
	if !found[public] || !found[secret] || found[private] {
		t.Fatalf("list scope: %+v", found)
	}
	rows, err = ListRecipeMastersScoped("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	found = map[string]bool{}
	for _, r := range rows {
		found[r.ID] = true
	}
	if !found[private] {
		t.Fatal("F&B pusat tidak melihat resep rahasia")
	}
	_, err = SaveRecipeMaster("", models.RecipeMasterRequest{Name: "Invalid", Visibility: "public", Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: -1}}})
	if err == nil {
		t.Fatal("resep negatif diterima")
	}
}

func TestFnbCostingSecretRecipes(t *testing.T) {
	f := newProductRecipeFixture(t)
	f.stockIn(t, f.itemID, 100, 2, "")
	private := f.recipe(t, "secret", false, 1, false)
	masterProduct := f.product(t, "")
	legacyProduct := f.product(t, "")
	// Simulate existing links whose recipe access has subsequently been revoked.
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='recipe',recipe_master_id=$2 WHERE id=$1`, masterProduct.ID, private); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='recipe' WHERE id=$1`, legacyProduct.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM product_recipes WHERE product_id=$1`, legacyProduct.ID) })
	// A mix of public and secret ingredients must not produce partial costing.
	for _, visibility := range []string{"public", "secret"} {
		if _, err := database.DB.Exec(`INSERT INTO product_recipes(id,product_id,item_id,qty_dist,qty_base,visibility) VALUES($1,$2,$3,1,1,$4)`, NewULID(), legacyProduct.ID, f.itemID, visibility); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := GetFnbCosting(f.outlet, []string{f.outlet})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("jumlah produk: %d", len(rows))
	}
	for _, row := range rows {
		if row.CostStatus != "restricted" || len(row.Ingredients) != 0 || row.TotalCost != 0 {
			t.Fatalf("resep rahasia terbuka untuk outlet: %+v", row)
		}
	}
	rows, err = GetFnbCosting(f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.CostStatus != "ready" || len(row.Ingredients) == 0 || row.TotalCost <= 0 {
			t.Fatalf("pengelola pusat tidak dapat menghitung resep: %+v", row)
		}
	}
}

func TestFnbReviewEditsRequireNewReview(t *testing.T) {
	f := newProductRecipeFixture(t)
	d := models.FnbDevelopment{OutletID: f.outlet, Name: "Uji menu", MenuKind: "food", TargetCostPct: 35}
	id, err := SaveFnbDevelopment("", d, nil, "tester")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM fnb_developments WHERE id=$1`, id) })
	if err := TransitionFnbDevelopment(id, "trial", 1, nil, "tester", false); err != nil {
		t.Fatal(err)
	}
	if err := TransitionFnbDevelopment(id, "review", 2, nil, "tester", false); err == nil {
		t.Fatal("review tanpa catatan/harga diterima")
	}
	d.Version = 2
	d.TargetPrice = 15000
	d.TrialNotes = "Rasa dan porsi sesuai"
	if _, err := SaveFnbDevelopment(id, d, nil, "tester"); err != nil {
		t.Fatal(err)
	}
	if err := TransitionFnbDevelopment(id, "review", 3, nil, "tester", false); err != nil {
		t.Fatal(err)
	}
	d.Version = 4
	d.TrialNotes = "Perubahan porsi perlu uji ulang"
	if _, err := SaveFnbDevelopment(id, d, nil, "tester"); err != nil {
		t.Fatal(err)
	}
	rows, err := ListFnbDevelopments(f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "trial" || rows[0].Version != 5 {
		t.Fatalf("revisi: %+v", rows)
	}
	if err := TransitionFnbDevelopment(id, "approved", 5, nil, "manager", true); err == nil {
		t.Fatal("revisi melewati review baru")
	}
}

func TestFnbCostingCentralFallbackAndWip(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	central.stockIn(t, item, 100, 2000, "")
	wip := outlet.item(t, "Minuman olahan", "porsi", "porsi", 1)
	if err := SaveStockItemRecipes(models.StockItemRecipeRequest{ParentItemID: wip, Visibility: "public", Items: []models.StockItemRecipeItem{{ItemID: item, QtyBase: 0.5}}}); err != nil {
		t.Fatal(err)
	}
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: outlet.outlet, Name: "Minuman Uji", Price: 3000})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_products WHERE id=$1`, p.ID)
		execScratch(`DELETE FROM stock_item_recipes WHERE parent_item_id=$1`, wip)
	})
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='single',linked_stock_item_id=$2 WHERE id=$1`, p.ID, wip); err != nil {
		t.Fatal(err)
	}
	rows, err := GetFnbCosting(outlet.outlet, []string{outlet.outlet})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TotalCost != 1000 || rows[0].CostStatus != "ready" {
		t.Fatalf("fallback/WIP: %+v", rows)
	}
	if _, err := database.DB.Exec(`UPDATE stock_item_recipes SET child_item_id=$1 WHERE parent_item_id=$1`, wip); err != nil {
		t.Fatal(err)
	}
	rows, err = GetFnbCosting(outlet.outlet, []string{outlet.outlet})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CostStatus != "missing_cost" {
		t.Fatalf("siklus WIP dianggap biaya lengkap: %+v", rows[0])
	}
}
