package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/middleware"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func waRolePolicyTestApp(role string, protected bool) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_role", role)
		return c.Next()
	})
	if protected {
		app.Put("/roles/:role/notifications", middleware.RequirePermission("whatsapp.manage"), WAUpdateRoleNotifications)
		app.Delete("/roles/:role/notifications", middleware.RequirePermission("whatsapp.manage"), WAResetRoleNotifications)
	} else {
		app.Put("/roles/:role/notifications", WAUpdateRoleNotifications)
	}
	return app
}

func TestWARoleNotificationRejectsMalformedBody(t *testing.T) {
	app := waRolePolicyTestApp("superadmin", false)
	for _, body := range []string{"", "null", "[]", `{"enabled":"yes"}`, "{"} {
		req := httptest.NewRequest("PUT", "/roles/test/notifications", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != fiber.StatusBadRequest {
			t.Errorf("malformed body %q: status %d", body, res.StatusCode)
		}
	}
}

func TestWARoleNotificationEndpointRequiresManageAndChangesOnlyTarget(t *testing.T) {
	requireHandlerDB(t)
	suffix := strings.ToLower(services.NewULID()[18:])
	viewer, target := "wan_view_"+suffix, "wan_target_"+suffix
	if _, err := database.DB.Exec(`INSERT INTO roles (name,description,is_system) VALUES ($1,'Notification viewer',false),($2,'Notification target',false)`, viewer, target); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO role_permissions (role,permission) VALUES ($1,'whatsapp.view')`, viewer); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM role_permissions WHERE role=$1`, viewer)
		database.DB.Exec(`DELETE FROM roles WHERE name IN ($1,$2)`, viewer, target)
	})
	var original string
	err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key='wa_role_notifications'`).Scan(&original)
	existed := err == nil
	t.Cleanup(func() {
		if existed {
			database.DB.Exec(`UPDATE app_settings SET value=$1 WHERE key='wa_role_notifications'`, original)
		} else {
			database.DB.Exec(`DELETE FROM app_settings WHERE key='wa_role_notifications'`)
		}
	})
	if _, err := database.DB.Exec(`INSERT INTO app_settings (key,value) VALUES ('wa_role_notifications','{"untouched":{"enabled":true}}')
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`); err != nil {
		t.Fatal(err)
	}
	policy := `{"enabled":false,"outlet_mode":"role","outlet_ids":[],"include_global":false,"events":{"sales.daily":"off"}}`
	for _, tc := range []struct {
		caller, method, body string
		want                 int
	}{
		{viewer, "PUT", policy, 403},
		{viewer, "DELETE", "", 403},
		{"superadmin", "PUT", policy, 200},
		{"superadmin", "DELETE", "", 200},
	} {
		app := waRolePolicyTestApp(tc.caller, true)
		req := httptest.NewRequest(tc.method, "/roles/"+target+"/notifications", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Fatalf("%s as %s: status %d, want %d", tc.method, tc.caller, res.StatusCode, tc.want)
		}
		var value string
		if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key='wa_role_notifications'`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		var policies map[string]json.RawMessage
		if err := json.Unmarshal([]byte(value), &policies); err != nil || policies["untouched"] == nil {
			t.Fatalf("unrelated policy overwritten: %s, %v", value, err)
		}
		if tc.want == 200 && (policies[target] != nil) != (tc.method == "PUT") {
			t.Fatalf("target policy not updated/reset: %s", value)
		}
	}
}
