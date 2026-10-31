package routes

import (
	"cloud-pos/config"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// The integration branch used to lack POS online/QRIS APIs while the remote
// branch lacked newer reservations, stock and finance. Both must survive merge.
func TestPOSMergedRoutesPreserveBothFeatureFamilies(t *testing.T) {
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	paths := map[string]bool{}
	for _, route := range app.GetRoutes() {
		paths[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"POST /api/v1/public/outlets/:slug/orders",
		"GET /api/v1/public/outlets/:slug/reservations/:id",
		"POST /api/v1/public/outlets/:slug/reservations/:id/payments",
		"GET /api/v1/outlets/:outletId/online-orders",
		"POST /api/v1/outlets/:outletId/qris/charges",
		"GET /api/v1/outlets/:outletId/reservations",
		"POST /api/v1/outlets/:outletId/sync/batch",
		"GET /api/v1/admin/ppic/usage-control",
		"POST /api/v1/admin/asset-transfers/:id/receive",
	} {
		if !paths[route] {
			t.Errorf("merge dropped required route %s", route)
		}
	}
	for _, path := range []string{
		"/api/v1/outlets/test-outlet/online-orders",
		"/api/v1/outlets/test-outlet/qris/info",
		"/api/v1/outlets/test-outlet/reservations",
	} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("outlet API bypassed auth: %s = %d", path, response.StatusCode)
		}
	}
}
