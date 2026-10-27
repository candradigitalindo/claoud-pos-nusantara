package routes

import (
	"cloud-pos/config"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestPpicFocusRetiredRoutesAreUnavailableEvenForSuperadmin(t *testing.T) {
	const token = "ppic-retirement-route-test-strong-token-0123456789"
	app := fiber.New()
	Setup(app, &config.Config{AdminToken: token})
	for _, route := range []struct{ method, path string }{
		{"GET", "/ppic/dashboard"}, {"GET", "/ppic/planning-params"}, {"PUT", "/ppic/planning-params"}, {"POST", "/ppic/planning-params/auto-fill"},
		{"GET", "/ppic/forecasts"}, {"POST", "/ppic/forecasts/generate"}, {"PUT", "/ppic/forecasts"},
		{"POST", "/ppic/mrp/run"}, {"POST", "/ppic/mrp/runs/old/create-pr"}, {"POST", "/ppic/mrp/runs/old/create-transfer"},
		{"GET", "/ppic/producible-items"}, {"POST", "/ppic/production-plans"}, {"POST", "/ppic/production-plans/old/release"},
		{"GET", "/ppic/work-orders"}, {"POST", "/ppic/work-orders/old/start"}, {"POST", "/ppic/work-orders/old/finish"},
		{"GET", "/ppic/reports/hpp"}, {"GET", "/ppic/reports/variance/export"},
	} {
		req := httptest.NewRequest(route.method, "/api/v1/admin"+route.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 404 {
			t.Fatalf("retired %s %s still available: %d", route.method, route.path, response.StatusCode)
		}
	}
}
