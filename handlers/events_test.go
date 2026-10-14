package handlers

// Stream SSE memakai aturan token statis yang sama dengan AdminAuth (T-1):
// nilai bawaan lama dan ADMIN_TOKEN kosong tidak membuka stream. Hanya jalur
// penolakan yang diuji — jalur diterima membuka stream tanpa akhir.

import (
	"net/http/httptest"
	"testing"

	"cloud-pos/config"

	"github.com/gofiber/fiber/v2"
)

func TestEventsMenolakTokenStatisLemah(t *testing.T) {
	for _, adminToken := range []string{"admin-secret-token", ""} {
		app := fiber.New()
		app.Get("/events", Events(&config.Config{AdminToken: adminToken, JWTSecret: "rahasia-jwt-uji-yang-panjang-0123456789"}))
		for _, kirim := range []string{"admin-secret-token", "", "bukan-jwt"} {
			resp, err := app.Test(httptest.NewRequest("GET", "/events?token="+kirim, nil))
			if err != nil {
				t.Fatalf("ADMIN_TOKEN=%q token=%q: %v", adminToken, kirim, err)
			}
			if resp.StatusCode != fiber.StatusUnauthorized {
				t.Errorf("ADMIN_TOKEN=%q token=%q: status = %d, mau 401", adminToken, kirim, resp.StatusCode)
			}
		}
	}
}
