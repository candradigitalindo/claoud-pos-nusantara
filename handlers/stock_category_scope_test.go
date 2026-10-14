package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Kategori item stok bagian dari katalog global (T-20), tetapi endpoint-nya
// dulu hanya memeriksa izin stockitems.*: role per outlet bisa membuat,
// mengubah, dan menghapus kategori yang dipakai semua gudang, padahal item
// katalognya sendiri sudah ditolak 403 (ditemukan saat uji UI 14 Okt 2026).
func TestStockItemCategoryRequiresCentralScope(t *testing.T) {
	requireHandlerDB(t)
	sfx := strings.ToLower(services.NewULID()[16:])
	name := "Kategori Uji Cakupan " + sfx
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_item_categories WHERE name LIKE $1`, "Kategori Uji Cakupan %"+sfx)
	})
	role := testRole(t, "stockitems.create", "stockitems.update", "stockitems.delete", "stockitems.view")
	app := func(locals map[string]interface{}) *fiber.App {
		a := fiber.New()
		a.Use(func(c *fiber.Ctx) error {
			for k, v := range locals {
				c.Locals(k, v)
			}
			return c.Next()
		})
		a.Post("/stock-item-categories", CreateStockItemCategory)
		a.Put("/stock-item-categories/:id", UpdateStockItemCategory)
		a.Delete("/stock-item-categories/:id", DeleteStockItemCategory)
		return a
	}
	scoped := app(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "manajer_" + sfx, "admin_role": role,
		"admin_scope": "specific", "admin_outlet_ids": []string{services.NewULID()}, "admin_work_unit_ids": []string{},
	})
	central := app(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "gudang_induk", "admin_role": role, "admin_scope": "all",
	})

	if code, _ := doJSON(t, scoped, "POST", "/stock-item-categories", map[string]interface{}{"name": name}); code != fiber.StatusForbidden {
		t.Errorf("role per outlet membuat kategori: %d, mau 403", code)
	}
	code, res := doJSON(t, central, "POST", "/stock-item-categories", map[string]interface{}{"name": name})
	if code != fiber.StatusCreated {
		t.Fatalf("Gudang Induk membuat kategori: %d %q", code, res.Error)
	}
	raw, _ := json.Marshal(res.Data)
	var cat models.StockItemCategory
	json.Unmarshal(raw, &cat)
	if cat.ID == "" {
		t.Fatalf("kategori baru tanpa id: %s", raw)
	}
	if code, _ := doJSON(t, scoped, "PUT", "/stock-item-categories/"+cat.ID, map[string]interface{}{"name": "Ganti " + name}); code != fiber.StatusForbidden {
		t.Errorf("role per outlet mengubah kategori: %d, mau 403", code)
	}
	if code, _ := doJSON(t, scoped, "DELETE", "/stock-item-categories/"+cat.ID, nil); code != fiber.StatusForbidden {
		t.Errorf("role per outlet menghapus kategori: %d, mau 403", code)
	}
	if code, res := doJSON(t, central, "PUT", "/stock-item-categories/"+cat.ID, map[string]interface{}{"name": "Ganti " + name}); code != fiber.StatusOK {
		t.Errorf("Gudang Induk mengubah kategori: %d %q", code, res.Error)
	}
	if code, res := doJSON(t, central, "DELETE", "/stock-item-categories/"+cat.ID, nil); code != fiber.StatusOK {
		t.Errorf("Gudang Induk menghapus kategori: %d %q", code, res.Error)
	}
}
