package handlers

import (
	"cloud-pos/database"
	"cloud-pos/services"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestPurchasingExportRejectsForeignTypeAndOutlet(t *testing.T) {
	requireHandlerDB(t)
	role := "export_equipment_" + strings.ToLower(services.NewULID()[18:])
	if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, 'procurement.requests.view')`, role); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role) })
	for _, tc := range []struct {
		name, role, path, scope string
		outlets                 []string
	}{
		{name: "equipment viewer cannot export raw materials", role: role, scope: "all", path: "/export?type=bahan"},
		{name: "explicit foreign outlet is forbidden", role: "superadmin", scope: "specific", outlets: []string{"mine"}, path: "/export?outlet_id=another-outlet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				c.Locals("admin_role", tc.role)
				c.Locals("admin_scope", tc.scope)
				c.Locals("admin_outlet_ids", tc.outlets)
				return c.Next()
			})
			app.Get("/export", ExportPurchasing)
			res, err := app.Test(httptest.NewRequest("GET", tc.path, nil), -1)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != fiber.StatusForbidden {
				t.Errorf("status = %d, want 403", res.StatusCode)
			}
		})
	}
}
