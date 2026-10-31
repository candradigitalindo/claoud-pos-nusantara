package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
)

func productKindPtr(kind string) *string { return &kind }

func changeProductKind(t *testing.T, p models.CloudProduct, kind *string) {
	t.Helper()
	if err := AdminUpdateProduct(p.ID, models.AdminUpdateProductRequest{
		Name: p.Name, Code: p.Code, Price: p.Price, MenuKind: kind,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProductKindEffectiveViewAndScopedFilters(t *testing.T) {
	f := newProductRecipeFixture(t)
	other := newProductRecipeFixture(t)
	foodRecipe := f.recipe(t, "public", false, 1, false)
	beverageRecipe := f.recipe(t, "public", false, 1, false)
	for id, kind := range map[string]string{foodRecipe: "food", beverageRecipe: "beverage"} {
		if _, err := database.DB.Exec(`UPDATE recipe_masters SET menu_kind=$2 WHERE id=$1`, id, kind); err != nil {
			t.Fatal(err)
		}
	}
	food := f.product(t, foodRecipe)
	links := []models.ProductRecipeLink{{RecipeMasterID: beverageRecipe, Multiplier: 1}, {RecipeMasterID: foodRecipe, Multiplier: 1}}
	beverage, err := AdminCreateProduct(models.AdminCreateProductRequest{
		OutletID: f.outlet, Name: "Menu campuran " + NewULID(), Price: 12000, Recipes: &links,
	})
	if err != nil {
		t.Fatal(err)
	}
	if food.MenuKind != "food" || beverage.MenuKind != "beverage" {
		t.Fatalf("create response lost first-recipe classification: food=%q beverage=%q", food.MenuKind, beverage.MenuKind)
	}
	unknown := f.product(t, foodRecipe)
	changeProductKind(t, unknown, productKindPtr(""))
	activity := f.product(t, "")
	changeProductKind(t, activity, productKindPtr("activity"))
	foreign := other.product(t, "")
	changeProductKind(t, foreign, productKindPtr("activity"))

	for _, want := range []struct{ id, kind, source string }{
		{food.ID, "food", "recipe"}, {beverage.ID, "beverage", "recipe"},
		{unknown.ID, "", "profile"}, {activity.ID, "activity", "profile"},
	} {
		var kind, source string
		if err := database.DB.QueryRow(`SELECT menu_kind,menu_kind_source FROM product_menu_kinds WHERE product_id=$1`, want.id).Scan(&kind, &source); err != nil {
			t.Fatal(err)
		}
		if kind != want.kind || source != want.source {
			t.Fatalf("effective kind: %s got %q/%q want %q/%q", want.id, kind, source, want.kind, want.source)
		}
	}
	for filter, id := range map[string]string{"food": food.ID, "beverage": beverage.ID, "unclassified": unknown.ID, "activity": activity.ID} {
		rows, count, err := GetAllProducts("", "", []string{f.outlet}, 1, 100, filter)
		if err != nil || count != 1 || len(rows) != 1 || rows[0].ID != id {
			t.Fatalf("filter %s scope mismatch: rows=%+v total=%d err=%v", filter, rows, count, err)
		}
	}
	rows, count, err := GetAllProducts(f.outlet, "", nil, 1, 100)
	if err != nil || count != 4 || len(rows) != 4 {
		t.Fatalf("legacy call signature: %+v %d %v", rows, count, err)
	}
	rows, count, err = GetAllProducts("", "", []string{}, 1, 100, "activity")
	if err != nil || count != 0 || len(rows) != 0 {
		t.Fatalf("empty scope leaked: %+v %d %v", rows, count, err)
	}
	if _, _, err := GetAllProducts(f.outlet, "", nil, 1, 100, "service"); !errors.Is(err, ErrProductMenuKind) {
		t.Fatalf("invalid filter: %v", err)
	}
	posRows, _, err := GetProducts(f.outlet, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, row := range posRows {
		got[row.ID] = row.MenuKind
	}
	if got[activity.ID] != "activity" || got[food.ID] != "food" || got[beverage.ID] != "beverage" {
		t.Fatalf("POS list missing effective kind: %+v", got)
	}
}

func TestProductKindEditsPreserveFnbCostsAndIgnorePOSClassification(t *testing.T) {
	f := newProductRecipeFixture(t)
	p := f.product(t, "")
	profile := models.FnbProfile{MenuKind: "food", TargetCostPct: 28, PackagingCost: 500, OverheadCost: 700}
	if err := SaveFnbProfile(p.ID, profile, []string{f.outlet}, "Cost owner"); err != nil {
		t.Fatal(err)
	}
	changeProductKind(t, p, productKindPtr("activity"))
	changeProductKind(t, p, nil)
	var push models.PushProductRequest
	if err := json.Unmarshal([]byte(`{"menu_kind":"beverage"}`), &push); err != nil {
		t.Fatal(err)
	}
	push.LocalID, push.Name, push.Code, push.Price, push.Version = p.LocalID, p.Name, p.Code, p.Price, 99
	if _, err := SaveProduct(f.outlet, push); err != nil {
		t.Fatal(err)
	}
	var kind, actor string
	var target, packaging, overhead float64
	if err := database.DB.QueryRow(`SELECT menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by FROM fnb_product_profiles WHERE product_id=$1`, p.ID).
		Scan(&kind, &target, &packaging, &overhead, &actor); err != nil {
		t.Fatal(err)
	}
	if kind != "activity" || target != 28 || packaging != 500 || overhead != 700 || actor != "Cost owner" {
		t.Fatalf("product/POS edit changed cost setup: %s %v %v %v %s", kind, target, packaging, overhead, actor)
	}
	bad := "other"
	if err := AdminUpdateProduct(p.ID, models.AdminUpdateProductRequest{Name: "Should not save", MenuKind: &bad}); !errors.Is(err, ErrProductMenuKind) {
		t.Fatalf("invalid kind accepted: %v", err)
	}
	var name string
	if err := database.DB.QueryRow(`SELECT name FROM cloud_products WHERE id=$1`, p.ID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != p.Name {
		t.Fatalf("invalid edit partially committed: %s", name)
	}
	created, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "Aktivitas " + NewULID(), Price: 50000, MenuKind: productKindPtr("activity")})
	if err != nil || created.MenuKind != "activity" {
		t.Fatalf("create activity: %+v %v", created, err)
	}
	var duplicateColumns int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='cloud_products' AND column_name='menu_kind'`).Scan(&duplicateColumns); err != nil {
		t.Fatal(err)
	}
	if duplicateColumns != 0 {
		t.Fatal("classification duplicated outside shared F&B profile")
	}
}

func TestProductKindFnbEditPublishesUpdatesAndActivityIsOutsideCosting(t *testing.T) {
	f := newProductRecipeFixture(t)
	activity := f.product(t, "")
	unknown := f.product(t, "")
	changeProductKind(t, activity, productKindPtr("activity"))
	costs, err := GetFnbCosting(f.outlet, []string{f.outlet})
	if err != nil || len(costs) != 1 || costs[0].ProductID != unknown.ID {
		t.Fatalf("activity included in costing: %+v %v", costs, err)
	}
	overview, err := GetFnbOverview(f.outlet, []string{f.outlet})
	if err != nil || overview.ProductCount != 1 || overview.Unclassified != 1 {
		t.Fatalf("activity inflated dashboard: %+v %v", overview, err)
	}
	if _, err := database.DB.Exec(`UPDATE cloud_products SET updated_at='2000-01-01' WHERE id=$1`, activity.ID); err != nil {
		t.Fatal(err)
	}
	if err := SaveFnbProfile(activity.ID, models.FnbProfile{MenuKind: "beverage", TargetCostPct: 30}, []string{f.outlet}, "F&B"); err != nil {
		t.Fatal(err)
	}
	updates, err := GetUpdatesSince(f.outlet, "2001-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range updates.Products {
		if row.CloudID == activity.ID {
			found = true
			if row.MenuKind != "beverage" {
				t.Fatalf("stale classification in incremental sync: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("F&B classification edit did not advance product cursor")
	}
	rows, _, err := GetAllProducts(f.outlet, "", nil, 1, 100, "beverage")
	if err != nil || len(rows) != 1 || rows[0].ID != activity.ID {
		t.Fatalf("shared kind not visible in product list: %+v %v", rows, err)
	}
	if err := SaveFnbProfile(activity.ID, models.FnbProfile{MenuKind: "activity", TargetCostPct: 30}, nil, "F&B"); err == nil {
		t.Fatal("F&B cost editor accepted activity")
	}
	if _, err := SaveRecipeMaster("", models.RecipeMasterRequest{Name: "Invalid activity recipe", Visibility: "public", MenuKind: productKindPtr("activity"), Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 1}}}); err == nil {
		t.Fatal("recipe accepted activity")
	}
	if _, err := SaveFnbDevelopment("", models.FnbDevelopment{OutletID: f.outlet, Name: "Invalid activity development", MenuKind: "activity", TargetCostPct: 35}, []string{f.outlet}, "F&B"); err == nil {
		t.Fatal("development accepted activity")
	}
}

func TestProductKindMigrationIsIdempotentAndKeepsExistingProfile(t *testing.T) {
	f := newProductRecipeFixture(t)
	p := f.product(t, "")
	if err := SaveFnbProfile(p.ID, models.FnbProfile{MenuKind: "food", TargetCostPct: 26, PackagingCost: 10}, nil, "Owner"); err != nil {
		t.Fatal(err)
	}
	changeProductKind(t, p, productKindPtr("activity"))
	for i := 0; i < 2; i++ {
		if err := database.RunFnbMigrations(); err != nil {
			t.Fatal(err)
		}
	}
	var kind string
	var target, packaging float64
	var updated time.Time
	if err := database.DB.QueryRow(`SELECT menu_kind,target_cost_pct,packaging_cost,updated_at FROM fnb_product_profiles WHERE product_id=$1`, p.ID).Scan(&kind, &target, &packaging, &updated); err != nil {
		t.Fatal(err)
	}
	if kind != "activity" || target != 26 || packaging != 10 || updated.IsZero() {
		t.Fatalf("migration changed profile: %s %.2f %.2f %v", kind, target, packaging, updated)
	}
}

func TestProductKindRecipeFallbackChangesPublishUpdates(t *testing.T) {
	f := newProductRecipeFixture(t)
	first := f.recipe(t, "public", false, 1, false)
	next := f.recipe(t, "public", false, 1, false)
	if _, err := database.DB.Exec(`UPDATE recipe_masters SET menu_kind='food' WHERE id IN ($1,$2)`, first, next); err != nil {
		t.Fatal(err)
	}
	createLinked := func(recipes ...string) models.CloudProduct {
		t.Helper()
		links := []models.ProductRecipeLink{}
		for _, id := range recipes {
			links = append(links, models.ProductRecipeLink{RecipeMasterID: id, Multiplier: 1})
		}
		p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "Fallback " + NewULID(), Price: 12345, CategoryName: "Kategori tetap", Recipes: &links})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	linked := createLinked(first, next)
	legacy := createLinked(first)
	if _, err := database.DB.Exec(`DELETE FROM product_recipe_links WHERE product_id=$1`, legacy.ID); err != nil {
		t.Fatal(err)
	}
	secondary := createLinked(next, first)
	overridden := createLinked(first)
	empty := createLinked(first)
	for id, kind := range map[string]string{overridden.ID: "activity", empty.ID: ""} {
		if _, err := database.DB.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by)
			VALUES($1,$2,27,200,300,'Cost owner')`, id, kind); err != nil {
			t.Fatal(err)
		}
	}
	resetCursor := func() {
		t.Helper()
		if _, err := database.DB.Exec(`UPDATE cloud_products SET updated_at='2000-01-01' WHERE outlet_id=$1`, f.outlet); err != nil {
			t.Fatal(err)
		}
	}
	assertState := func(version int, linkedKind, legacyKind string, wantUpdateCount int) {
		t.Helper()
		products, _, err := GetProducts(f.outlet, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range products {
			wantKind, wantVersion := "food", 1
			switch p.ID {
			case linked.ID:
				wantKind, wantVersion = linkedKind, version
			case legacy.ID:
				wantKind, wantVersion = legacyKind, version
			case overridden.ID:
				wantKind = "activity"
			case empty.ID:
				wantKind = ""
			case secondary.ID:
			default:
				t.Fatalf("unexpected product %s", p.ID)
			}
			if p.MenuKind != wantKind || p.Version != wantVersion || p.Price != 12345 || p.CategoryName != "Kategori tetap" {
				t.Fatalf("recipe propagation changed unrelated data or wrong revision: %+v want kind=%q version=%d", p, wantKind, wantVersion)
			}
		}
		updates, err := GetUpdatesSince(f.outlet, "2001-01-01T00:00:00Z")
		if err != nil || len(updates.Products) != wantUpdateCount {
			t.Fatalf("incremental propagation: %+v %v", updates, err)
		}
		for _, update := range updates.Products {
			wantKind := linkedKind
			if update.CloudID == legacy.ID {
				wantKind = legacyKind
			} else if update.CloudID != linked.ID {
				t.Fatalf("unchanged effective kind unexpectedly published: %+v", update)
			}
			if update.MenuKind != wantKind || update.Version != version {
				t.Fatalf("incremental/full pull disagree: %+v", update)
			}
		}
	}
	resetCursor()
	req := models.RecipeMasterRequest{Name: "Resep diperbarui", Visibility: "public", MenuKind: productKindPtr("beverage"), Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 1}}}
	// A rejected item change must roll back the recipe kind and all revisions.
	bad := req
	bad.Items = []models.RecipeItemReq{{ItemID: NewULID(), QtyBase: 1}}
	if _, err := SaveRecipeMaster(first, bad); err == nil {
		t.Fatal("invalid recipe item unexpectedly saved")
	}
	assertState(1, "food", "food", 0)
	if _, err := SaveRecipeMaster(first, req); err != nil {
		t.Fatal(err)
	}
	assertState(2, "beverage", "beverage", 2)
	resetCursor()
	if _, err := SaveRecipeMaster(first, req); err != nil {
		t.Fatal(err)
	}
	assertState(2, "beverage", "beverage", 0)
	if err := DeleteRecipeMaster(first); err != nil {
		t.Fatal(err)
	}
	assertState(3, "food", "", 2)
	for id, wantKind := range map[string]string{overridden.ID: "activity", empty.ID: ""} {
		var kind, actor string
		var target, packaging, overhead float64
		if err := database.DB.QueryRow(`SELECT menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by FROM fnb_product_profiles WHERE product_id=$1`, id).
			Scan(&kind, &target, &packaging, &overhead, &actor); err != nil {
			t.Fatal(err)
		}
		if kind != wantKind || target != 27 || packaging != 200 || overhead != 300 || actor != "Cost owner" {
			t.Fatalf("recipe lifecycle changed explicit profile: %q %v %v %v %q", kind, target, packaging, overhead, actor)
		}
	}
}

func TestProductKindRecipeConcurrentLinkRequiresRetry(t *testing.T) {
	f := newProductRecipeFixture(t)
	recipe := f.recipe(t, "public", false, 1, false)
	existing := f.product(t, recipe)
	joining := f.product(t, "")
	writer, err := database.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	// Simulate a product editor which validated the recipe, then commits its
	// new link while the recipe updater waits for that SHARE lock.
	if _, err := writer.Exec(`SELECT id FROM recipe_masters WHERE id=$1 FOR SHARE`, recipe); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(`UPDATE cloud_products SET stock_type='recipe',recipe_master_id=$1 WHERE id=$2`, recipe, joining.ID); err != nil {
		t.Fatal(err)
	}
	req := models.RecipeMasterRequest{Name: "Concurrent recipe", Visibility: "public", MenuKind: productKindPtr("beverage"), Items: []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 1}}}
	done := make(chan error, 1)
	go func() { _, err := SaveRecipeMaster(recipe, req); done <- err }()
	consumed := false
	defer func() {
		writer.Rollback()
		if !consumed {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("recipe updater did not stop")
			}
		}
	}()
	locked := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		probe, err := database.DB.Begin()
		if err != nil {
			t.Fatal(err)
		}
		_, err = probe.Exec(`SELECT id FROM cloud_products WHERE id=$1 FOR UPDATE NOWAIT`, existing.ID)
		probe.Rollback()
		var pg *pq.Error
		if errors.As(err, &pg) && pg.Code == "55P03" {
			locked = true
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !locked {
		t.Fatal("recipe updater never acquired its product lock")
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		consumed = true
		var conflict ConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("newly linked product was silently missed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recipe/product locking deadlocked")
	}
	var kind string
	if err := database.DB.QueryRow(`SELECT menu_kind FROM recipe_masters WHERE id=$1`, recipe).Scan(&kind); err != nil || kind != "" {
		t.Fatalf("conflict partially committed recipe: %q %v", kind, err)
	}
	if _, err := SaveRecipeMaster(recipe, req); err != nil {
		t.Fatal(err)
	}
	rows, _, err := GetProducts(f.outlet, 1, 100)
	if err != nil || len(rows) != 2 {
		t.Fatalf("read after retry: %+v %v", rows, err)
	}
	for _, p := range rows {
		if p.MenuKind != "beverage" || p.Version != 2 {
			t.Fatalf("retry did not propagate to both products: %+v", p)
		}
	}
}
