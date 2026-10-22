package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProductRecipeOptionsRequireOutletScope(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		return c.Next()
	})
	app.Get("/products/recipe-options", AdminProductRecipeOptions)
	app.Get("/products/recipe-options/:id", AdminProductRecipeDetail)
	for _, test := range []struct {
		url    string
		status int
	}{
		{"/products/recipe-options", 400},
		{"/products/recipe-options?outlet_id=other", 403},
		{"/products/recipe-options/recipe-one", 400},
		{"/products/recipe-options/recipe-one?outlet_id=other", 403},
	} {
		code, _ := doJSON(t, app, "GET", test.url, nil)
		if code != test.status {
			t.Errorf("%s: %d, mau %d", test.url, code, test.status)
		}
	}
}

func TestProductRecipeOptionsEmptyScopeDenied(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		return c.Next()
	})
	app.Get("/products/recipe-options", AdminProductRecipeOptions)
	app.Get("/products/recipe-options/:id", AdminProductRecipeDetail)
	for _, path := range []string{"/products/recipe-options?outlet_id=any", "/products/recipe-options/recipe-one?outlet_id=any"} {
		code, _ := doJSON(t, app, "GET", path, nil)
		if code != fiber.StatusForbidden {
			t.Fatalf("scope kosong %s: %d, mau 403", path, code)
		}
	}
}
