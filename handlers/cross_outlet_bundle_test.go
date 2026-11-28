package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestCrossOutletBundleSaveRejectsScopedRole(t *testing.T) {
	app := fiber.New()
	app.Put("/products/:id/bundle-components", func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"OUTLET-ATV"})
		return c.Next()
	}, SaveProductBundleComponents)

	req := httptest.NewRequest("PUT", "/products/PROD-1/bundle-components", strings.NewReader(`{"components":[]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for outlet-scoped role editing cross-outlet bundles, got %d", resp.StatusCode)
	}
}

func TestManualBundleClaimRejectsScopedRole(t *testing.T) {
	app := fiber.New()
	app.Post("/cross-outlet-bundles/claims", func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"OUTLET-ATV"})
		return c.Next()
	}, CreateManualBundleClaim)

	req := httptest.NewRequest("POST", "/cross-outlet-bundles/claims", strings.NewReader(`{"component_id":"COMP-1","source_qty":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for scoped role creating manual bundle claim, got %d", resp.StatusCode)
	}
}
