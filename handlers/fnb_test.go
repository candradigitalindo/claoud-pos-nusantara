package handlers

import (
	"github.com/gofiber/fiber/v2"
	"testing"
)

func TestFnbRejectsOutsideScopeBeforeDataAccess(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		return c.Next()
	})
	app.Get("/dashboard", FnbOverview)
	app.Get("/costing", FnbCosting)
	app.Get("/development", ListFnbDevelopments)
	app.Get("/options", FnbDevelopmentOptions)
	app.Post("/recipes", CreateRecipeMaster)
	app.Put("/recipes/:id", UpdateRecipeMaster)
	app.Delete("/recipes/:id", DeleteRecipeMaster)
	for _, test := range []struct{ method, url string }{{"GET", "/dashboard?outlet_id=other"}, {"GET", "/costing?outlet_id=other"}, {"GET", "/development?outlet_id=other"}, {"GET", "/options?outlet_id=other"}, {"POST", "/recipes"}, {"PUT", "/recipes/one"}, {"DELETE", "/recipes/one"}} {
		code, _ := doJSON(t, app, test.method, test.url, nil)
		if code != 403 {
			t.Errorf("%s %s: %d", test.method, test.url, code)
		}
	}
}
