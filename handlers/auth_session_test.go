package handlers

// Handler autentikasi (T-56): galat DB saat login = 503 dengan pesan umum —
// bukan 401 berisi galat mentah, dan tidak pernah menerbitkan token — serta
// /me/permissions menampilkan scope hasil validasi sesi (selain "all" berarti
// specific; tidak pernah jatuh ke "all").

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/database"

	"github.com/gofiber/fiber/v2"
)

func TestAdminLoginGalatDB503(t *testing.T) {
	putus, err := sql.Open("postgres", "host=db-uji-dinonaktifkan.invalid port=1 user=uji dbname=uji_test sslmode=disable connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	asli := database.DB
	database.DB = putus
	t.Cleanup(func() {
		database.DB = asli
		putus.Close()
	})

	app := fiber.New()
	app.Post("/login", AdminLogin(&config.Config{JWTSecret: "rahasia-jwt-uji-login-0123456789"}))
	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"siapa","password":"apa-saja"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 15000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("login saat DB putus: status %d, mau 503 (body %s)", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "invalid") || strings.Contains(string(body), "dial") || strings.Contains(string(body), "token\":") {
		t.Fatalf("respons login membocorkan galat mentah / token: %s", body)
	}
}

func TestGetMyPermissionsMemakaiScopeSesiDB(t *testing.T) {
	if !strings.HasSuffix(os.Getenv("DB_NAME"), "_test") || database.DB == nil || database.DB.Ping() != nil {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
	kasus := []struct {
		nama    string
		locals  map[string]interface{}
		scope   string
		outlets []string // nil = harus null (scope all)
	}{
		{"scope all", map[string]interface{}{"admin_scope": "all"}, "all", nil},
		{"scope specific", map[string]interface{}{"admin_scope": "specific", "admin_outlet_ids": []string{"O1", "O2"}}, "specific", []string{"O1", "O2"}},
		{"specific tanpa outlet", map[string]interface{}{"admin_scope": "specific"}, "specific", []string{}},
		{"scope tidak terpasang → tertutup", map[string]interface{}{}, "specific", []string{}},
	}
	for _, k := range kasus {
		app := fiber.New()
		app.Get("/me", func(c *fiber.Ctx) error {
			c.Locals("admin_role", "manager")
			for kunci, v := range k.locals {
				c.Locals(kunci, v)
			}
			return c.Next()
		}, GetMyPermissions)
		resp, err := app.Test(httptest.NewRequest("GET", "/me", nil), 15000)
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Data struct {
				ScopeType      string    `json:"scope_type"`
				ScopeOutletIDs *[]string `json:"scope_outlet_ids"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: status %d, decode %v", k.nama, resp.StatusCode, err)
		}
		if out.Data.ScopeType != k.scope {
			t.Errorf("%s: scope_type = %q, mau %q", k.nama, out.Data.ScopeType, k.scope)
		}
		switch {
		case k.outlets == nil && out.Data.ScopeOutletIDs != nil:
			t.Errorf("%s: scope_outlet_ids = %v, mau null", k.nama, *out.Data.ScopeOutletIDs)
		case k.outlets != nil && (out.Data.ScopeOutletIDs == nil || strings.Join(*out.Data.ScopeOutletIDs, ",") != strings.Join(k.outlets, ",")):
			t.Errorf("%s: scope_outlet_ids = %v, mau %v", k.nama, out.Data.ScopeOutletIDs, k.outlets)
		}
	}
}
