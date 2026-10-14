package middleware

// Uji jalur token admin statis di AdminAuth (T-1). Tanpa database: token yang
// ditolak jatuh ke validasi JWT, yang gagal tanpa menyentuh DB.

import (
	"io"
	"net/http/httptest"
	"testing"

	"cloud-pos/config"

	"github.com/gofiber/fiber/v2"
)

const tokenKuatUji = "3f9c1a7e5b2d4c6e8a0b1d3f5e7c9a2b4d6f8e0a1c3e5b7d9f2a4c6e8b0d1f3a"

func TestAdminAuthTokenStatis(t *testing.T) {
	kasus := []struct {
		nama, adminToken, kirim string
		status                  int
	}{
		{"nilai bawaan lama ditolak", "admin-secret-token", "admin-secret-token", 401},
		{"ADMIN_TOKEN kosong menutup jalur statis", "", "token-apa-pun", 401},
		{"token pendek ditolak", "pendek-sekali", "pendek-sekali", 401},
		{"token kuat diterima sebagai superadmin", tokenKuatUji, tokenKuatUji, 200},
		{"token kuat yang salah ditolak", tokenKuatUji, tokenKuatUji[:63] + "0", 401},
	}
	for _, k := range kasus {
		cfg := &config.Config{AdminToken: k.adminToken, JWTSecret: "rahasia-jwt-uji-yang-panjang-0123456789"}
		app := fiber.New()
		app.Get("/x", AdminAuth(cfg), func(c *fiber.Ctx) error {
			role, _ := c.Locals("admin_role").(string)
			return c.SendString(role)
		})
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("Authorization", "Bearer "+k.kirim)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s: %v", k.nama, err)
		}
		if resp.StatusCode != k.status {
			t.Errorf("%s: status = %d, mau %d", k.nama, resp.StatusCode, k.status)
			continue
		}
		if k.status == 200 {
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "superadmin" {
				t.Errorf("%s: role = %q, mau superadmin", k.nama, body)
			}
		}
	}
}
