package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/middleware"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func roleNotificationsTestApp(role string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("admin_role", role); return c.Next() })
	app.Get("/admin/roles/notification-options", middleware.RequirePermission("roles.view"), GetRoleNotificationOptions)
	app.Put("/admin/roles/:role/notifications", middleware.RequirePermission("roles.update"), UpdateRoleNotifications)
	app.Delete("/admin/roles/:role/notifications", middleware.RequirePermission("roles.update"), ResetRoleNotifications)
	return app
}

func roleNotificationRequest(t *testing.T, app *fiber.App, method, path, body string) (int, json.RawMessage) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("invalid response %s: %v", raw, err)
	}
	return res.StatusCode, response.Data
}

func assertSafeRoleNotificationData(t *testing.T, raw []byte) {
	t.Helper()
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"events", "roles", "settings"}) {
		t.Fatalf("unexpected role options fields: %v", keys)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data["settings"], &settings); err != nil {
		t.Fatal(err)
	}
	keys = []string{}
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"enabled", "event_roles", "events", "role_notifications"}) {
		t.Fatalf("role endpoint exposed unrelated settings: %v", keys)
	}
	var events []map[string]json.RawMessage
	if err := json.Unmarshal(data["events"], &events); err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("internal notification options are missing")
	}
	for _, event := range events {
		if string(event["audience"]) != `"internal"` {
			t.Fatalf("customer event exposed: %s", event["key"])
		}
		for _, forbidden := range []string{"template", "default_template", "placeholders"} {
			if _, exists := event[forbidden]; exists {
				t.Fatalf("event %s exposed %s", event["key"], forbidden)
			}
		}
	}
	for _, secret := range []string{"INTERNAL_TEMPLATE_SECRET", "CUSTOMER_TEMPLATE_SECRET", "private-notification.example", "customer.reservation_created"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("role response leaked %q", secret)
		}
	}
}

func TestRoleNotificationPayloadOmitsTemplatesAndCustomerSettings(t *testing.T) {
	policy := models.WARoleNotificationPolicy{Enabled: true, OutletMode: "role", Events: map[string]string{"reservation.created": "off", "customer.reservation_created": "off"}}
	s := models.WASettings{
		Enabled: true, PublicBaseURL: "https://private-notification.example", GapMinSec: 7,
		Events:            map[string]bool{"reservation.created": false, "customer.reservation_created": false},
		Templates:         map[string]string{"reservation.created": "INTERNAL_TEMPLATE_SECRET", "customer.reservation_created": "CUSTOMER_TEMPLATE_SECRET"},
		EventRoles:        map[string][]string{"reservation.created": {"test"}, "customer.reservation_created": {"test"}},
		RoleNotifications: map[string]models.WARoleNotificationPolicy{"test": policy},
	}
	raw, err := json.Marshal(roleNotificationData(s, []models.WARoleOption{{Name: "test", Policy: policy}}))
	if err != nil {
		t.Fatal(err)
	}
	assertSafeRoleNotificationData(t, raw)
	if policy.Events["customer.reservation_created"] != "off" || s.Templates["reservation.created"] != "INTERNAL_TEMPLATE_SECRET" {
		t.Fatal("sanitizing response changed source settings")
	}
}

func TestRoleNotificationsRejectMalformedPolicyWithoutDatabase(t *testing.T) {
	app := roleNotificationsTestApp("superadmin")
	for _, body := range []string{"", "null", "[]", `{"enabled":"yes"}`, "{"} {
		if code, _ := roleNotificationRequest(t, app, "PUT", "/admin/roles/test/notifications", body); code != fiber.StatusBadRequest {
			t.Errorf("malformed body %q: status %d", body, code)
		}
	}
}

func TestRoleNotificationsPermissionsSanitizationAndTargetOnlyUpdates(t *testing.T) {
	requireHandlerDB(t)
	suffix := strings.ToLower(services.NewULID()[18:])
	viewer, editor, waOnly, target, requester := "rnv"+suffix, "rne"+suffix, "rnw"+suffix, "rnt"+suffix, "rnr"+suffix
	permissions := map[string][]string{viewer: {"roles.view"}, editor: {"roles.update"}, waOnly: {"whatsapp.manage"}, target: {"cashier_shifts.view"}, requester: {"procurement.requests.submit"}}
	for role, perms := range permissions {
		if _, err := database.DB.Exec(`INSERT INTO roles (name, description, scope_type) VALUES ($1,'Role notification HTTP test','specific')`, role); err != nil {
			t.Fatal(err)
		}
		for _, permission := range perms {
			if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1,$2)`, role, permission); err != nil {
				t.Fatal(err)
			}
		}
		role := role
		t.Cleanup(func() {
			database.DB.Exec(`DELETE FROM role_permissions WHERE role=$1`, role)
			database.DB.Exec(`DELETE FROM roles WHERE name=$1`, role)
		})
	}
	settings := map[string]string{
		"wa_role_notifications": `{"untouched":{"enabled":true,"outlet_mode":"role","include_global":false,"outlet_ids":[],"events":{}}}`,
		"wa_templates":          `{"reservation.created":"INTERNAL_TEMPLATE_SECRET","customer.reservation_created":"CUSTOMER_TEMPLATE_SECRET"}`,
		"wa_events":             `{"reservation.created":true,"customer.reservation_created":false}`,
		"wa_event_roles":        `{}`, "wa_public_base_url": "https://private-notification.example", "wa_gap_min_sec": "7",
	}
	for key, value := range settings {
		var original string
		err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key=$1`, key).Scan(&original)
		if err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		key, original, existed := key, original, err == nil
		t.Cleanup(func() {
			if existed {
				database.DB.Exec(`INSERT INTO app_settings (key,value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, original)
			} else {
				database.DB.Exec(`DELETE FROM app_settings WHERE key=$1`, key)
			}
		})
		if _, err := database.DB.Exec(`INSERT INTO app_settings (key,value) VALUES ($1,$2) ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, value); err != nil {
			t.Fatal(err)
		}
	}
	policy := `{"enabled":true,"outlet_mode":"role","outlet_ids":[],"include_global":false,"events":{"cashier_shift.closed":"on"}}`
	path := "/admin/roles/" + target + "/notifications"
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{viewer, "GET", "/admin/roles/notification-options", "", 200},
		{viewer, "PUT", path, policy, 403},
		{viewer, "DELETE", path, "", 403},
		{waOnly, "GET", "/admin/roles/notification-options", "", 403},
		{waOnly, "PUT", path, policy, 403},
		{editor, "GET", "/admin/roles/notification-options", "", 200},
		{editor, "PUT", path, policy, 200},
		{editor, "PUT", path, `{"enabled":true,"outlet_mode":"role","include_global":true}`, 400},
		{editor, "PUT", path, `{"enabled":true,"outlet_mode":"selected","outlet_ids":["outside-role-or-inactive"]}`, 400},
		{editor, "PUT", "/admin/roles/role-does-not-exist/notifications", policy, 400},
		{editor, "DELETE", "/admin/roles/role-does-not-exist/notifications", "", 400},
		{editor, "DELETE", path, "", 200},
	} {
		code, data := roleNotificationRequest(t, roleNotificationsTestApp(tc.role), tc.method, tc.path, tc.body)
		if code != tc.want {
			t.Fatalf("%s %s as %s: %d, want %d", tc.method, tc.path, tc.role, code, tc.want)
		}
		if code == 200 {
			assertSafeRoleNotificationData(t, data)
			var response struct {
				Roles []models.WARoleOption `json:"roles"`
			}
			if err := json.Unmarshal(data, &response); err != nil {
				t.Fatal(err)
			}
			foundCashier, foundRequester := false, false
			for _, role := range response.Roles {
				if role.Name == target {
					foundCashier = true
					if len(role.PersonalEvents) != 0 {
						t.Fatal("cashier-only role received unrelated purchase personal metadata")
					}
				}
				if role.Name == requester {
					foundRequester = true
					if len(role.PersonalEvents) == 0 {
						t.Fatal("requester lost own-document metadata")
					}
				}
			}
			if !foundCashier || !foundRequester {
				t.Fatal("role metadata missing target/requester")
			}
		}
		var raw string
		if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key='wa_role_notifications'`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var policies map[string]models.WARoleNotificationPolicy
		if err := json.Unmarshal([]byte(raw), &policies); err != nil {
			t.Fatal(err)
		}
		if !policies["untouched"].Enabled {
			t.Fatal("saving one role clobbered another role's policy")
		}
		if tc.role == editor && tc.method == "PUT" && code == 200 && policies[target].Events["cashier_shift.closed"] != "on" {
			t.Fatal("roles.update without whatsapp.manage could not save target policy")
		}
		if tc.role == editor && tc.method == "DELETE" && code == 200 {
			if _, exists := policies[target]; exists {
				t.Fatal("target policy did not reset")
			}
		}
	}
	for key, expected := range settings {
		if key == "wa_role_notifications" {
			continue
		}
		var actual string
		if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key=$1`, key).Scan(&actual); err != nil || actual != expected {
			t.Fatalf("role endpoint changed unrelated setting %s", key)
		}
	}
}
