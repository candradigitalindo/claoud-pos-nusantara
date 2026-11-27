package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestReservationSettingsRejectsScopedRole(t *testing.T) {
	app := fiber.New()
	app.Put("/reservations/settings", func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"OUTLET-1"})
		return c.Next()
	}, UpdateReservationSettings)

	req := httptest.NewRequest("PUT", "/reservations/settings", strings.NewReader(`{"dp_percent":25}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for scoped outlet role changing global DP settings, got %d", resp.StatusCode)
	}
}

func TestListReservationProductsValidatesOutletAndScope(t *testing.T) {
	app := fiber.New()
	app.Get("/reservations/products", func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"OUTLET-1"})
		return c.Next()
	}, ListReservationProducts)

	// Missing outlet_id → 400
	req1 := httptest.NewRequest("GET", "/reservations/products", nil)
	resp1, err := app.Test(req1, 5000)
	if err != nil {
		t.Fatalf("req1: %v", err)
	}
	if resp1.StatusCode != 400 {
		t.Fatalf("expected 400 for missing outlet_id, got %d", resp1.StatusCode)
	}

	// Out-of-scope outlet_id → 403
	req2 := httptest.NewRequest("GET", "/reservations/products?outlet_id=OUTLET-OTHER", nil)
	resp2, err := app.Test(req2, 5000)
	if err != nil {
		t.Fatalf("req2: %v", err)
	}
	if resp2.StatusCode != 403 {
		t.Fatalf("expected 403 for out-of-scope outlet_id, got %d", resp2.StatusCode)
	}
}
