package routes

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRecipeAuthoringSeparatedFromWarehouseProduction(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	warehouse, _ := ruteAkun(t, "wh"+suffix, []string{"stockitems.view", "stockitems.update", "stockledger.view", "stockledger.adjust"})
	viewer, _ := ruteAkun(t, "rv"+suffix, []string{"recipes.view", "stockledger.view"})
	author, _ := ruteAkun(t, "ra"+suffix, []string{"recipes.view", "recipes.create", "recipes.update", "recipes.delete"})
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	wh, err := services.CreateWarehouse(models.WarehouseRequest{Code: "FR" + suffix, Name: "Gudang tes produksi", Type: "central", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	var items []string
	t.Cleanup(func() {
		for _, id := range items {
			ruteExec(`DELETE FROM stock_item_recipes WHERE parent_item_id=$1 OR child_item_id=$1`, id)
			ruteExec(`DELETE FROM stock_movements WHERE item_id=$1`, id)
			ruteExec(`DELETE FROM stock_batches WHERE item_id=$1`, id)
			ruteExec(`DELETE FROM stock_ledger WHERE item_id=$1`, id)
		}
		for _, id := range items {
			ruteExec(`DELETE FROM stock_items WHERE id=$1`, id)
		}
		ruteExec(`DELETE FROM warehouses WHERE id=$1`, wh.ID)
	})
	for _, name := range []string{"Bahan " + suffix, "Hasil " + suffix} {
		item, err := services.CreateStockItem(models.StockItemRequest{Name: name, BaseUnit: "g", DistUnit: "g", DistRatio: 1}, "test")
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, item.ID)
	}
	raw, result := items[0], items[1]
	base := "/api/v1/admin"
	formulaURL := base + "/stock-items/" + result + "/recipes"
	produceURL := base + "/stock-items/" + result + "/produce"
	formula := models.StockItemRecipeRequest{
		// The URL is authoritative; a body-supplied parent cannot retarget writes.
		ParentItemID: raw, Visibility: "public",
		Items: []models.StockItemRecipeItem{{ItemID: raw, QtyBase: 2}},
	}
	if code, res := rutePanggil(t, app, "PUT", formulaURL, author, formula); code != fiber.StatusOK {
		t.Fatalf("F&B author saves SFG: %d %s", code, res.Error)
	}
	rows, err := services.GetStockItemRecipes(result)
	if err != nil || len(rows) != 1 || rows[0].ChildItemID != raw {
		t.Fatalf("path target formula: %+v %v", rows, err)
	}
	for _, token := range []string{warehouse, viewer} {
		for _, action := range []struct{ method, url string }{
			{"PUT", formulaURL}, {"PUT", base + "/products/product/recipes"},
			{"PUT", base + "/stock-items/" + raw + "/breakdown"},
			{"POST", base + "/recipe-masters"}, {"PUT", base + "/recipe-masters/recipe"}, {"DELETE", base + "/recipe-masters/recipe"},
		} {
			if code, _ := rutePanggil(t, app, action.method, action.url, token, formula); code != fiber.StatusForbidden {
				t.Errorf("role without recipe write %s %s: %d", action.method, action.url, code)
			}
		}
	}
	if code, res := rutePanggil(t, app, "GET", formulaURL, viewer, nil); code != fiber.StatusOK {
		t.Fatalf("recipe view: %d %s", code, res.Error)
	}
	produce := models.ProduceRequest{WarehouseID: wh.ID, QtyProduce: 3}
	for _, token := range []string{viewer, author} {
		if code, _ := rutePanggil(t, app, "POST", produceURL, token, produce); code != fiber.StatusForbidden {
			t.Fatalf("recipe permission alone must not post stock: %d", code)
		}
	}
	if err := services.CreateAdjustment(models.AdjustmentRequest{WarehouseID: wh.ID, ItemID: raw, MovType: "purchase_in", QtyBase: 100, CostPerBase: 5}, "test"); err != nil {
		t.Fatal(err)
	}
	if code, res := rutePanggil(t, app, "POST", produceURL, warehouse, produce); code != fiber.StatusOK {
		t.Fatalf("warehouse-only production: %d %s", code, res.Error)
	}
	for item, want := range map[string]float64{raw: 94, result: 3} {
		var qty float64
		if err := database.DB.QueryRow(`SELECT qty_base FROM stock_ledger WHERE warehouse_id=$1 AND item_id=$2`, wh.ID, item).Scan(&qty); err != nil || qty != want {
			t.Errorf("stock %s: %v want %v (%v)", item, qty, want, err)
		}
	}
	rows, err = services.GetStockItemRecipes(result)
	if err != nil || len(rows) != 1 || rows[0].QtyBase != 2 {
		t.Fatalf("denied writes/production changed formula: %+v %v", rows, err)
	}
}

func TestLegacyWarehouseMigrationDoesNotGrantRecipeAccess(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	for _, explicit := range []bool{false, true} {
		role := "fwm" + suffix
		if explicit {
			role += "f"
		}
		perms := []string{"warehouse.view", "warehouse.create", "warehouse.update", "warehouse.delete"}
		if explicit {
			perms = append(perms, "recipes.view", "recipes.create", "recipes.update", "recipes.delete")
		}
		if _, err := services.CreateRole(models.CreateRoleRequest{Name: role, ScopeType: "all"}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ruteExec(`DELETE FROM role_permissions WHERE role=$1`, role)
			ruteExec(`DELETE FROM roles WHERE name=$1`, role)
		})
		// Historical coarse permissions are no longer accepted by the role
		// editor, so seed their old stored representation for migration coverage.
		for _, permission := range perms {
			if _, err := database.DB.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2)`, role, permission); err != nil {
				t.Fatal(err)
			}
		}
		if err := database.RunMigrations(); err != nil {
			t.Fatal(err)
		}
		for _, permission := range []string{"recipes.view", "recipes.create", "recipes.update", "recipes.delete"} {
			var got bool
			if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role=$1 AND permission=$2)`, role, permission).Scan(&got); err != nil || got != explicit {
				t.Errorf("explicit=%v %s: got %v (%v)", explicit, permission, got, err)
			}
		}
		var adjust bool
		if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM role_permissions WHERE role=$1 AND permission='stockledger.adjust')`, role).Scan(&adjust); err != nil || !adjust {
			t.Fatalf("warehouse execution permission lost: %v %v", adjust, err)
		}
	}
}
