package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"testing"
)

type productRecipeFixture struct {
	*bdFixture
	itemID string
}

func newProductRecipeFixture(t *testing.T) *productRecipeFixture {
	t.Helper()
	requireDB(t)
	f := &productRecipeFixture{bdFixture: newBreakdownFixture(t)}
	f.itemID = f.item(t, "Bahan resep produk", "g", "g", 1)
	t.Cleanup(func() {
		execScratch(`DELETE FROM sync_logs WHERE outlet_id = $1`, f.outlet)
		execScratch(`DELETE FROM cloud_products WHERE outlet_id = $1`, f.outlet)
	})
	return f
}

func (f *productRecipeFixture) recipe(t *testing.T, visibility string, access bool, qty float64, empty bool) string {
	t.Helper()
	req := models.RecipeMasterRequest{Name: "Resep " + NewULID(), Visibility: visibility}
	if access {
		req.OutletIDs = []string{f.outlet}
	}
	// Build a valid recipe first; the invalid fixtures simulate historical rows.
	req.Items = []models.RecipeItemReq{{ItemID: f.itemID, QtyBase: 1}}
	id, err := SaveRecipeMaster("", req)
	if err != nil {
		t.Fatal(err)
	}
	if empty {
		_, err = database.DB.Exec(`DELETE FROM recipe_items WHERE recipe_master_id=$1`, id)
	} else {
		_, err = database.DB.Exec(`UPDATE recipe_items SET qty_base=$2 WHERE recipe_master_id=$1`, id, qty)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM recipe_masters WHERE id = $1`, id) })
	return id
}

func (f *productRecipeFixture) product(t *testing.T, recipeID string) models.CloudProduct {
	t.Helper()
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{
		OutletID: f.outlet, Name: "Produk " + NewULID(), Code: NewULID()[10:], Price: 10000, RecipeMasterID: recipeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProductRecipeOptionalLifecycle(t *testing.T) {
	f := newProductRecipeFixture(t)
	r1 := f.recipe(t, "public", false, 10, false)
	r2 := f.recipe(t, "secret", true, 5, false)
	f.stockIn(t, f.itemID, 100, 2, "")
	p := f.product(t, "")
	if p.StockType != "none" || p.RecipeMasterID != "" {
		t.Fatalf("produk tanpa resep: %+v", p)
	}
	checkStock := func(want float64) {
		t.Helper()
		got, _ := f.ledger(t, f.itemID)
		if !bdNear(got, want) {
			t.Fatalf("stok %.4f, mau %.4f", got, want)
		}
	}
	sell := func(qty float64) {
		t.Helper()
		if err := DeductStockByRecipe(f.outlet, p.LocalID, qty, NewULID(), "uji resep"); err != nil {
			t.Fatal(err)
		}
	}
	update := func(recipeID *string) {
		t.Helper()
		if err := AdminUpdateProduct(p.ID, models.AdminUpdateProductRequest{Name: p.Name, Code: p.Code, Price: p.Price, RecipeMasterID: recipeID}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() models.CloudProduct {
		t.Helper()
		products, _, err := GetAllProducts(f.outlet, p.Code, nil, 1, 100)
		if err != nil || len(products) != 1 {
			t.Fatalf("baca produk: %v, %#v", err, products)
		}
		return products[0]
	}
	sell(2)
	checkStock(100)
	update(&r1)
	if got := read(); got.StockType != "recipe" || got.RecipeMasterID != r1 || got.RecipeMasterName == "" {
		t.Fatalf("tautan tidak tersimpan: %+v", got)
	}
	sell(2)
	checkStock(80)
	// Klien lama yang hanya mengubah nama/harga tidak boleh melepas tautan.
	update(nil)
	if got := read(); got.RecipeMasterID != r1 {
		t.Fatal("update tanpa field resep menghapus tautan")
	}
	// Sinkronisasi POS juga mempertahankan pengaturan resep dari cloud.
	if _, err := SaveProduct(f.outlet, models.PushProductRequest{LocalID: p.LocalID, Name: p.Name, Code: p.Code, Price: p.Price, Version: 3}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got.RecipeMasterID != r1 {
		t.Fatal("sinkronisasi POS menghapus tautan")
	}
	update(&r2)
	sell(3)
	checkStock(65)
	empty := ""
	update(&empty)
	if got := read(); got.StockType != "none" || got.RecipeMasterID != "" {
		t.Fatalf("gagal melepas resep: %+v", got)
	}
	sell(3)
	checkStock(65)
	// Konfigurasi stok langsung yang sudah ada juga dipertahankan oleh klien lama.
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='single', linked_stock_item_id=$1 WHERE id=$2`, f.itemID, p.ID); err != nil {
		t.Fatal(err)
	}
	update(nil)
	if got := read(); got.StockType != "single" {
		t.Fatal("update biasa mengganti stok langsung")
	}
	update(&r1)
	var linked string
	if err := database.DB.QueryRow(`SELECT COALESCE(linked_stock_item_id,'') FROM cloud_products WHERE id=$1`, p.ID).Scan(&linked); err != nil || linked != "" {
		t.Fatalf("tautan stok langsung harus dilepas saat memilih resep: %q %v", linked, err)
	}
	created := f.product(t, r2)
	if created.StockType != "recipe" || created.RecipeMasterID != r2 || created.RecipeMasterName == "" {
		t.Fatalf("buat produk dengan resep: %+v", created)
	}
	products, _, err := GetProducts(f.outlet, 1, 100)
	if err != nil || len(products) != 2 || products[0].RecipeMasterName == "" || products[1].RecipeMasterName == "" {
		t.Fatalf("daftar outlet tidak membawa tautan: %v, %+v", err, products)
	}
}

func TestProductRecipeAvailability(t *testing.T) {
	f := newProductRecipeFixture(t)
	public := f.recipe(t, "public", false, 10, false)
	secret := f.recipe(t, "secret", true, 10, false)
	denied := f.recipe(t, "secret", false, 10, false)
	empty := f.recipe(t, "public", false, 0, true)
	negative := f.recipe(t, "public", false, -1, false)
	options, err := ListProductRecipeOptions(f.outlet)
	if err != nil {
		t.Fatal(err)
	}
	available := map[string]bool{}
	for _, r := range options {
		available[r.ID] = true
	}
	if !available[public] || !available[secret] || available[denied] || available[empty] || available[negative] {
		t.Fatalf("filter pilihan resep tidak sesuai: %+v", options)
	}
	p := f.product(t, public)
	for _, id := range []string{denied, empty, negative, NewULID()} {
		_, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outlet, Name: "Ditolak", Code: NewULID()[10:], RecipeMasterID: id})
		if !errors.Is(err, ErrProductRecipe) {
			t.Fatalf("buat dengan resep tidak valid: %v", err)
		}
		err = AdminUpdateProduct(p.ID, models.AdminUpdateProductRequest{Name: "Tidak boleh berubah", Code: p.Code, RecipeMasterID: &id})
		if !errors.Is(err, ErrProductRecipe) {
			t.Fatalf("ubah dengan resep tidak valid: %v", err)
		}
	}
	var name, recipeID string
	if err := database.DB.QueryRow(`SELECT name, recipe_master_id FROM cloud_products WHERE id=$1`, p.ID).Scan(&name, &recipeID); err != nil || name != p.Name || recipeID != public {
		t.Fatalf("penolakan harus atomik: %q %q %v", name, recipeID, err)
	}
}
