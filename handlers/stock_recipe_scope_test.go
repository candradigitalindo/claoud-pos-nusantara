package handlers

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestOutletCannotAuthorSharedRecipes(t *testing.T) {
	// Scope must reject before parsing or touching the database, even if a
	// role has recipe-write permission at the router.
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"own-outlet"})
		return c.Next()
	})
	app.Put("/products/:id/recipes", SaveProductRecipes)
	app.Put("/stock-items/:id/recipes", SaveStockItemRecipes)
	app.Put("/stock-items/:id/breakdown", SaveStockItemBreakdown)
	app.Post("/recipe-masters", CreateRecipeMaster)
	app.Put("/recipe-masters/:id", UpdateRecipeMaster)
	app.Delete("/recipe-masters/:id", DeleteRecipeMaster)
	for _, tc := range []struct{ method, path string }{
		{"PUT", "/products/p/recipes"}, {"PUT", "/stock-items/s/recipes"},
		{"PUT", "/stock-items/s/breakdown"}, {"POST", "/recipe-masters"},
		{"PUT", "/recipe-masters/r"}, {"DELETE", "/recipe-masters/r"},
	} {
		if code, _ := doJSON(t, app, tc.method, tc.path, map[string]interface{}{}); code != fiber.StatusForbidden {
			t.Errorf("%s %s: %d, want 403", tc.method, tc.path, code)
		}
	}
}

func TestStockProductionRestrictedToOwnOutlet(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	result := services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO stock_items(id,code,name,base_unit,dist_unit,dist_ratio) VALUES($1,$2,'Hasil uji produksi','ml','ml',1)`, result, result); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_item_recipes WHERE parent_item_id=$1`, result)
		database.DB.Exec(`DELETE FROM stock_movements WHERE item_id=$1`, result)
		database.DB.Exec(`DELETE FROM stock_batches WHERE item_id=$1`, result)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE item_id=$1`, result)
		database.DB.Exec(`DELETE FROM stock_items WHERE id=$1`, result)
	})
	if err := services.SaveStockItemRecipes(models.StockItemRecipeRequest{ParentItemID: result, Items: []models.StockItemRecipeItem{{ItemID: f.item, QtyBase: 2}}}); err != nil {
		t.Fatal(err)
	}
	if err := services.CreateAdjustment(models.AdjustmentRequest{ItemID: f.item, WarehouseID: f.whA, QtyBase: 100, CostPerBase: 12, MovType: "purchase_in"}, "uji"); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{f.outletA})
		c.Locals("admin_username", "gudang_a")
		return c.Next()
	})
	app.Post("/stock-items/:id/produce", ProduceStockItem)
	for _, wh := range []string{f.whB, f.whC, f.whC2} {
		if code, _ := doJSON(t, app, "POST", "/stock-items/"+result+"/produce", models.ProduceRequest{WarehouseID: wh, QtyProduce: 3}); code != fiber.StatusForbidden {
			t.Fatalf("production outside outlet %s: %d", wh, code)
		}
	}
	if code, res := doJSON(t, app, "POST", "/stock-items/"+result+"/produce", models.ProduceRequest{WarehouseID: f.whA, QtyProduce: 3}); code != fiber.StatusOK {
		t.Fatalf("own outlet production: %d %s", code, res.Error)
	}
	if got := ledgerQty(f.item, f.whA); got != 94 {
		t.Fatalf("ingredients: %v, want 94", got)
	}
	if got := ledgerQty(result, f.whA); got != 3 {
		t.Fatalf("output: %v, want 3", got)
	}
}

func TestProductRecipeTargetUsesURL(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	var ids []string
	t.Cleanup(func() {
		for _, id := range ids {
			database.DB.Exec(`DELETE FROM product_recipes WHERE product_id=$1`, id)
			database.DB.Exec(`DELETE FROM cloud_products WHERE id=$1`, id)
		}
		database.DB.Exec(`DELETE FROM sync_logs WHERE outlet_id=$1`, f.outletA)
	})
	for _, name := range []string{"Produk target", "Produk lain"} {
		p, err := services.AdminCreateProduct(models.AdminCreateProductRequest{OutletID: f.outletA, Name: name, Code: services.NewULID()[10:], Price: 1000})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, p.ID)
	}
	app := fiber.New()
	app.Put("/products/:id/recipes", SaveProductRecipes)
	app.Get("/products/:id/recipes", GetProductRecipes)
	url := "/products/" + ids[0] + "/recipes"
	request := models.ProductRecipeRequest{ProductID: ids[1], Visibility: "public", Items: []models.ProductRecipeItem{{ItemID: f.item, QtyDist: 1}}}
	if code, res := doJSON(t, app, "PUT", url, request); code != fiber.StatusOK {
		t.Fatalf("save legacy product formula: %d %s", code, res.Error)
	}
	code, res := doJSON(t, app, "GET", url, nil)
	var rows []models.ProductRecipe
	decodeData(t, res, &rows)
	if code != fiber.StatusOK || len(rows) != 1 || rows[0].ProductID != ids[0] || rows[0].QtyBase != 1000 {
		t.Fatalf("read URL target: %d %+v", code, rows)
	}
	other, err := services.GetProductRecipes(ids[1])
	if err != nil || len(other) != 0 {
		t.Fatalf("body target unexpectedly changed: %+v %v", other, err)
	}
}
