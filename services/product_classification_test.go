package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type classificationFixture struct {
	outlet, foreign string
	recipes         []string
}

func newClassificationFixture(t *testing.T) *classificationFixture {
	t.Helper()
	requireDB(t)
	f := &classificationFixture{outlet: NewULID(), foreign: NewULID()}
	for _, outlet := range []string{f.outlet, f.foreign} {
		if _, err := database.DB.Exec(`INSERT INTO outlets(id,code,name,api_key) VALUES($1,$2,'Classification fixture',$3)`,
			outlet, "CLASS"+outlet[12:], "classification-"+outlet); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, outlet := range []string{f.outlet, f.foreign} {
			if _, err := database.DB.Exec(`DELETE FROM cloud_products WHERE outlet_id=$1`, outlet); err != nil {
				t.Error(err)
			}
			if _, err := database.DB.Exec(`DELETE FROM outlets WHERE id=$1`, outlet); err != nil {
				t.Error(err)
			}
		}
		for _, recipe := range f.recipes {
			if _, err := database.DB.Exec(`DELETE FROM recipe_masters WHERE id=$1`, recipe); err != nil {
				t.Error(err)
			}
		}
	})
	return f
}

func (f *classificationFixture) product(t *testing.T, outlet, name string) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO cloud_products(id,local_id,outlet_id,name,category_name,price,updated_at)
		VALUES($1,$2,$3,$4,'',10000,'2020-01-01T00:00:00Z')`, id, id, outlet, name); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *classificationFixture) recipe(t *testing.T, product, kind string) string {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO recipe_masters(id,name,menu_kind) VALUES($1,'Private recipe fixture',$2)`, id, kind); err != nil {
		t.Fatal(err)
	}
	f.recipes = append(f.recipes, id)
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='recipe',recipe_master_id=$1 WHERE id=$2`, id, product); err != nil {
		t.Fatal(err)
	}
	return id
}

func classificationSelection(t *testing.T, outlet string, scope []string, ids ...string) []models.ApplyProductClassificationItem {
	t.Helper()
	report, err := PreviewProductClassification(outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	rows := make(map[string]models.ProductClassificationRow)
	for _, row := range report.Rows {
		rows[row.ProductID] = row
	}
	items := []models.ApplyProductClassificationItem{}
	for _, id := range ids {
		row, found := rows[id]
		if !found {
			t.Fatalf("preview did not include %s", id)
		}
		items = append(items, models.ApplyProductClassificationItem{ProductID: id, Fingerprint: row.Fingerprint, SuggestedKind: row.SuggestedKind})
	}
	return items
}

func TestProductClassificationPreviewScopedCompleteAndConservative(t *testing.T) {
	f := newClassificationFixture(t)
	for i := 0; i < 105; i++ {
		f.product(t, f.outlet, fmt.Sprintf("Espresso %d", i))
	}
	manual := f.product(t, f.outlet, "Kopi dengan jenis manual")
	if _, err := database.DB.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind) VALUES($1,'activity')`, manual); err != nil {
		t.Fatal(err)
	}
	linked := f.product(t, f.outlet, "Kopi dengan resep")
	f.recipe(t, linked, "food")
	f.product(t, f.outlet, "Custom Payment")
	deleted := f.product(t, f.outlet, "Nasi dihapus")
	if _, err := database.DB.Exec(`UPDATE cloud_products SET is_deleted=true WHERE id=$1`, deleted); err != nil {
		t.Fatal(err)
	}
	f.product(t, f.foreign, "Kopi outlet lain")
	report, err := PreviewProductClassification("", []string{f.outlet})
	if err != nil {
		t.Fatal(err)
	}
	if report.Totals != (models.ProductClassificationTotals{Total: 108, Existing: 2, Suggested: 106, Food: 1, Beverage: 105, Activity: 1, Other: 1}) || len(report.Rows) != 108 {
		t.Fatalf("preview truncated or incorrectly classified: %+v rows=%d", report.Totals, len(report.Rows))
	}
	for _, row := range report.Rows {
		if row.OutletID != f.outlet || row.ProductID == deleted {
			t.Fatalf("scope/deletion leak: %+v", row)
		}
	}
	if _, err := PreviewProductClassification(f.foreign, []string{f.outlet}); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("explicit foreign outlet: %v", err)
	}
	empty, err := PreviewProductClassification("", []string{})
	if err != nil || len(empty.Rows) != 0 || empty.Rows == nil {
		t.Fatalf("empty scope must return []: %+v %v", empty, err)
	}
}

func TestProductClassificationApplyPreservesCostsAndAudit(t *testing.T) {
	f := newClassificationFixture(t)
	food := f.product(t, f.outlet, "Nasi Goreng")
	beverage := f.product(t, f.outlet, "Espresso")
	if _, err := database.DB.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by)
		VALUES($1,'',22.5,1200,350,'previous actor')`, food); err != nil {
		t.Fatal(err)
	}
	selection := classificationSelection(t, f.outlet, nil, food, beverage)
	applied, err := ApplyProductClassification(f.outlet, []string{f.outlet}, "classifier tester", selection)
	if err != nil || applied != 2 {
		t.Fatalf("apply=%d err=%v", applied, err)
	}
	var kind, actor string
	var target, packaging, overhead float64
	var updated time.Time
	var version int
	if err := database.DB.QueryRow(`SELECT f.menu_kind,f.target_cost_pct,f.packaging_cost,f.overhead_cost,f.updated_by,p.updated_at,p.version
		FROM fnb_product_profiles f JOIN cloud_products p ON p.id=f.product_id WHERE p.id=$1`, food).
		Scan(&kind, &target, &packaging, &overhead, &actor, &updated, &version); err != nil {
		t.Fatal(err)
	}
	if kind != "food" || target != 22.5 || packaging != 1200 || overhead != 350 || actor != "classifier tester" || updated.Year() <= 2020 || version != 2 {
		t.Fatalf("classification changed costs/lost audit: %s %.2f %.2f %.2f %s %v %d", kind, target, packaging, overhead, actor, updated, version)
	}
	var conflict ConflictError
	if _, err := ApplyProductClassification(f.outlet, nil, "retry", selection); !errors.As(err, &conflict) {
		t.Fatalf("stale retry must not rewrite existing labels: %v", err)
	}
}

func TestProductClassificationApplyRejectsTamperingAndStaleAtomically(t *testing.T) {
	f := newClassificationFixture(t)
	one := f.product(t, f.outlet, "Nasi Goreng")
	two := f.product(t, f.outlet, "Espresso")
	foreign := f.product(t, f.foreign, "ATV")
	selection := classificationSelection(t, f.outlet, nil, one, two)
	assertUnchanged := func() {
		t.Helper()
		var count int
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM fnb_product_profiles WHERE product_id IN ($1,$2)`, one, two).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed selection partially changed profiles: count=%d err=%v", count, err)
		}
	}
	for _, tamper := range []func([]models.ApplyProductClassificationItem){
		func(items []models.ApplyProductClassificationItem) { items[1].SuggestedKind = "activity" },
		func(items []models.ApplyProductClassificationItem) { items[1].Fingerprint = strings.Repeat("0", 64) },
	} {
		items := append([]models.ApplyProductClassificationItem{}, selection...)
		tamper(items)
		var conflict ConflictError
		if _, err := ApplyProductClassification(f.outlet, nil, "tester", items); !errors.As(err, &conflict) {
			t.Fatalf("tampered suggestion accepted: %v", err)
		}
		assertUnchanged()
	}
	var invalid ValidationError
	if _, err := ApplyProductClassification(f.outlet, nil, "tester", []models.ApplyProductClassificationItem{selection[0], selection[0]}); !errors.As(err, &invalid) {
		t.Fatalf("duplicate selection accepted: %v", err)
	}
	foreignSelection := classificationSelection(t, f.foreign, nil, foreign)
	if _, err := ApplyProductClassification("", []string{f.outlet}, "tester", append(selection, foreignSelection...)); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("foreign row accepted: %v", err)
	}
	assertUnchanged()
	if _, err := database.DB.Exec(`UPDATE cloud_products SET name='Latte baru',updated_at=NOW() WHERE id=$1`, two); err != nil {
		t.Fatal(err)
	}
	var conflict ConflictError
	if _, err := ApplyProductClassification(f.outlet, nil, "tester", selection); !errors.As(err, &conflict) {
		t.Fatalf("stale name accepted: %v", err)
	}
	assertUnchanged()
	// Recipe metadata also participates, even if the product timestamp did not change.
	recipe := f.recipe(t, one, "")
	selection = classificationSelection(t, f.outlet, nil, one)
	if _, err := database.DB.Exec(`UPDATE recipe_masters SET menu_kind='beverage' WHERE id=$1`, recipe); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyProductClassification(f.outlet, nil, "tester", selection); !errors.As(err, &conflict) {
		t.Fatalf("stale recipe accepted: %v", err)
	}
	assertUnchanged()
}

func TestProductClassificationConcurrentApplyHasSingleWinner(t *testing.T) {
	f := newClassificationFixture(t)
	id := f.product(t, f.outlet, "Espresso")
	selection := classificationSelection(t, f.outlet, nil, id)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := ApplyProductClassification(f.outlet, nil, "parallel tester", selection)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		var conflict ConflictError
		if err == nil {
			succeeded++
		} else if errors.As(err, &conflict) {
			conflicted++
		} else {
			t.Errorf("unexpected concurrent failure: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent apply winners=%d conflicts=%d", succeeded, conflicted)
	}
	var version int
	if err := database.DB.QueryRow(`SELECT version FROM cloud_products WHERE id=$1`, id).Scan(&version); err != nil || version != 2 {
		t.Fatalf("concurrent apply wrote more than once: version=%d err=%v", version, err)
	}
}

func TestProductClassificationATVOverlandPolicyApplyAndRename(t *testing.T) {
	f := newClassificationFixture(t)
	if _, err := database.DB.Exec(`UPDATE outlets SET name='ATV Overland' WHERE id=$1`, f.outlet); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, name := range []string{"Dokumentasi", "GOLD ATV", "Jas Hujan", "SILVER ATV"} {
		ids = append(ids, f.product(t, f.outlet, name))
	}
	foreign := f.product(t, f.foreign, "Jas Hujan")
	selection := classificationSelection(t, f.outlet, []string{f.outlet}, ids...)
	for _, item := range selection {
		if item.SuggestedKind != "activity" {
			t.Fatalf("outlet item omitted from activity selection: %+v", item)
		}
	}
	// Outlet identity is part of the evidence. A rename after preview must
	// reject the entire batch, including otherwise recognizable ATV items.
	if _, err := database.DB.Exec(`UPDATE outlets SET name='ATV Overland Cafe' WHERE id=$1`, f.outlet); err != nil {
		t.Fatal(err)
	}
	var conflict ConflictError
	if _, err := ApplyProductClassification(f.outlet, []string{f.outlet}, "tester", selection); !errors.As(err, &conflict) {
		t.Fatalf("outlet rename accepted stale policy: %v", err)
	}
	var written int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM fnb_product_profiles f JOIN cloud_products p ON p.id=f.product_id WHERE p.outlet_id=$1`, f.outlet).Scan(&written); err != nil || written != 0 {
		t.Fatalf("stale policy partially applied: %d %v", written, err)
	}
	if _, err := database.DB.Exec(`UPDATE outlets SET name='ATV Overland' WHERE id=$1`, f.outlet); err != nil {
		t.Fatal(err)
	}
	selection = classificationSelection(t, f.outlet, []string{f.outlet}, ids...)
	if n, err := ApplyProductClassification(f.outlet, []string{f.outlet}, "owner policy", selection); err != nil || n != 4 {
		t.Fatalf("apply policy: %d %v", n, err)
	}
	updates, err := GetUpdatesSince(f.outlet, "2021-01-01T00:00:00Z")
	if err != nil || len(updates.Products) != 4 {
		t.Fatalf("policy updates missing: %+v %v", updates, err)
	}
	for _, product := range updates.Products {
		if product.MenuKind != "activity" || product.Version != 2 {
			t.Fatalf("policy did not publish activity: %+v", product)
		}
	}
	report, err := PreviewProductClassification("", []string{f.outlet, f.foreign})
	if err != nil || report.Totals != (models.ProductClassificationTotals{Total: 5, Existing: 4, Suggested: 1, Activity: 4, Other: 1}) {
		t.Fatalf("policy scope/retry result: %+v %v", report.Totals, err)
	}
	for _, row := range report.Rows {
		if row.ProductID == foreign && (row.CurrentKind != "" || row.Status != "suggested" || row.SuggestedKind != "other") {
			t.Fatalf("policy changed another outlet: %+v", row)
		}
	}
}

func TestProductClassificationOtherFallbackPreservesScopeCostsAndSync(t *testing.T) {
	f := newClassificationFixture(t)
	id := f.product(t, f.outlet, "Paket Nasi + Ice Tea")
	foreign := f.product(t, f.foreign, "Nama belum dikenal")
	recipe := f.recipe(t, id, "beverage")
	if _, err := database.DB.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by)
		VALUES($1,'',22.5,1200,350,'previous actor')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`UPDATE cloud_products SET category_name='Paket asli' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	selection := classificationSelection(t, f.outlet, []string{f.outlet}, id)
	if selection[0].SuggestedKind != "other" {
		t.Fatalf("unclassified product not grouped into Lainnya: %+v", selection)
	}
	foreignSelection := classificationSelection(t, f.foreign, nil, foreign)
	if _, err := ApplyProductClassification("", []string{f.outlet}, "owner policy", append(selection, foreignSelection...)); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("Lainnya fallback bypassed outlet scope: %v", err)
	}
	var kind string
	if err := database.DB.QueryRow(`SELECT menu_kind FROM fnb_product_profiles WHERE product_id=$1`, id).Scan(&kind); err != nil || kind != "" {
		t.Fatalf("foreign selection partially applied: %q %v", kind, err)
	}
	if n, err := ApplyProductClassification(f.outlet, []string{f.outlet}, "owner policy", selection); err != nil || n != 1 {
		t.Fatalf("apply Lainnya=%d err=%v", n, err)
	}
	var actor string
	var target, packaging, overhead float64
	if err := database.DB.QueryRow(`SELECT menu_kind,target_cost_pct,packaging_cost,overhead_cost,updated_by FROM fnb_product_profiles WHERE product_id=$1`, id).
		Scan(&kind, &target, &packaging, &overhead, &actor); err != nil {
		t.Fatal(err)
	}
	if kind != "other" || target != 22.5 || packaging != 1200 || overhead != 350 || actor != "owner policy" {
		t.Fatalf("Lainnya lost profile data/audit: %s %.2f %.2f %.2f %s", kind, target, packaging, overhead, actor)
	}
	products, _, err := GetProducts(f.outlet, 1, 100)
	if err != nil || len(products) != 1 {
		t.Fatalf("full sync: %+v %v", products, err)
	}
	p := products[0]
	if p.MenuKind != "other" || p.Price != 10000 || p.CategoryName != "Paket asli" || p.StockType != "recipe" || p.RecipeMasterID != recipe || p.Version != 2 {
		t.Fatalf("Lainnya changed unrelated product fields: %+v", p)
	}
	updates, err := GetUpdatesSince(f.outlet, "2021-01-01T00:00:00Z")
	if err != nil || len(updates.Products) != 1 || updates.Products[0].MenuKind != "other" || updates.Products[0].Version != 2 {
		t.Fatalf("incremental sync lost Lainnya: %+v %v", updates, err)
	}
	report, err := PreviewProductClassification(f.outlet, []string{f.outlet})
	if err != nil || report.Totals != (models.ProductClassificationTotals{Total: 1, Existing: 1, Other: 1}) {
		t.Fatalf("applied Lainnya must be preserved on next preview: %+v %v", report.Totals, err)
	}
}
