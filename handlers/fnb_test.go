package handlers

import (
	"cloud-pos/models"
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

func TestFnbCostOnlyAccessRedactsAllComposition(t *testing.T) {
	snapshot := &models.RecipeCostSnapshot{Ingredients: []models.RecipeCostIngredient{{ItemID: "secret"}}}
	rows := []models.FnbCostRow{{Ingredients: snapshot.Ingredients, PriceComparisons: []models.FnbPriceComparison{{ItemID: "secret"}}, Recipes: []models.FnbRecipeCost{{RecipeCost: models.RecipeCost{InitialCost: snapshot, CostIngredients: snapshot.Ingredients}}}}}
	redactFnbComposition(rows)
	if len(rows[0].Ingredients) > 0 || len(rows[0].PriceComparisons) > 0 || len(rows[0].Recipes[0].CostIngredients) > 0 || len(rows[0].Recipes[0].InitialCost.Ingredients) > 0 {
		t.Fatal("cost-only response leaked composition")
	}
	if len(snapshot.Ingredients) != 1 {
		t.Fatal("redaction mutated shared snapshot")
	}
}

func TestFnbOutletPolicyHandlersRejectOutsideScope(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		return c.Next()
	})
	app.Get("/outlets/:id/policy", GetFnbOutletPolicy)
	app.Put("/outlets/:id/policy", SaveFnbOutletPolicy)
	for _, method := range []string{"GET", "PUT"} {
		code, _ := doJSON(t, app, method, "/outlets/other/policy", map[string]any{"target_cost_pct": 40})
		if code != 403 {
			t.Fatalf("%s scope: %d", method, code)
		}
	}
}
