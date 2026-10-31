package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProductKindRejectsInvalidFilterAndCreationBeforeDatabaseAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		return c.Next()
	})
	app.Get("/products", AdminGetProducts)
	app.Post("/products", AdminCreateProduct)
	for _, tc := range []struct {
		method, path string
		body         map[string]any
		want         int
	}{
		{"GET", "/products?menu_kind=service", nil, 400},
		{"GET", "/products?outlet_id=other&menu_kind=activity", nil, 403},
		{"GET", "/products?outlet_id=other&menu_kind=other", nil, 403},
		{"POST", "/products", map[string]any{"outlet_id": "allowed", "name": "Uji", "menu_kind": "service"}, 400},
		{"POST", "/products", map[string]any{"outlet_id": "other", "name": "Uji", "menu_kind": "activity"}, 403},
		{"POST", "/products", map[string]any{"outlet_id": "other", "name": "Uji", "menu_kind": "other"}, 403},
	} {
		status, _ := doJSON(t, app, tc.method, tc.path, tc.body)
		if status != tc.want {
			t.Fatalf("%s %s: status %d, want %d", tc.method, tc.path, status, tc.want)
		}
	}
}
