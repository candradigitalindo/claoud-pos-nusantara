package routes

import (
	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProductClassificationRoutesRequireUpdatePermissionToApply(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	viewer, _ := ruteAkun(t, "cv"+suffix, []string{"products.view"})
	updater, _ := ruteAkun(t, "cu"+suffix, []string{"products.update"})
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	path := "/api/v1/admin/products/classification"
	for _, test := range []struct {
		method, token string
		status        int
	}{
		{"GET", viewer, 200}, {"POST", viewer, 403},
		// Existing RBAC makes products.update imply products.view.
		{"GET", updater, 200}, {"POST", updater, 400},
		{"GET", "", 401}, {"POST", "", 401},
	} {
		if code, result := rutePanggil(t, app, test.method, path, test.token, models.ApplyProductClassificationRequest{}); code != test.status {
			t.Fatalf("%s status=%d want=%d response=%s", test.method, code, test.status, result.Error)
		}
	}
}
