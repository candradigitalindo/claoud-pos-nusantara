package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"cloud-pos/config"

	"github.com/gofiber/fiber/v2"
)

func TestCCTVStopRejectsForeignNameBeforeRelay(t *testing.T) {
	app := fiber.New()
	app.Post("/cameras/:camId/stop", StopCameraStream)
	for _, name := range []string{"", "cam_01ARZ3NDEKTSV4RRFFQ69G5FAW", "pb_01ARZ3NDEKTSV4RRFFQ69G5FAW_0123abcd", "rtsp://host/live"} {
		resp, err := app.Test(httptest.NewRequest("POST", "/cameras/01ARZ3NDEKTSV4RRFFQ69G5FAV/stop?name="+name, nil))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusForbidden {
			t.Fatalf("name=%q: status %d", name, resp.StatusCode)
		}
	}
}

func TestCCTVAuthzDoesNotAuthorizeWithoutMediaURI(t *testing.T) {
	app := fiber.New()
	app.Get("/authz", CCTVAuthz(&config.Config{}))
	resp, err := app.Test(httptest.NewRequest("GET", "/authz", nil))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("authz umum: status %d", resp.StatusCode)
	}
}

func TestCCTVRelayRejectsUnauthenticatedMediaAndInternalAPI(t *testing.T) {
	app := fiber.New()
	app.Get("/cctv/*", CCTVRelay)
	for path, status := range map[string]int{
		"/cctv/api/stream.m3u8?src=cam_01ARZ3NDEKTSV4RRFFQ69G5FAV": fiber.StatusUnauthorized,
		"/cctv/api/streams": fiber.StatusForbidden,
		"/cctv/api/config":  fiber.StatusForbidden,
	} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != status {
			t.Fatalf("%s: status %d, mau %d", path, resp.StatusCode, status)
		}
	}
}

func TestCCTVLogoutClearsMediaCookie(t *testing.T) {
	app := fiber.New()
	app.Post("/logout", ClearCCTVSession)
	resp, err := app.Test(httptest.NewRequest("POST", "/logout", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(cookie, "cctv_token=") || !strings.Contains(strings.ToLower(cookie), "path=/cctv") || !strings.Contains(strings.ToLower(cookie), "expires=") {
		t.Fatalf("cookie CCTV tidak dibersihkan: %s", cookie)
	}
}

func TestCCTVSetupOperationsRespectOutletScope(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		return c.Next()
	})
	app.Post("/outlets/:id/cameras/probe", ProbeCameraConfiguration)
	app.Post("/outlets/:id/cameras/onvif/profiles", DiscoverCameraONVIFProfiles)
	app.Delete("/outlets/:id/cctv-gateway", DeleteCCTVGateway)
	for _, test := range [][2]string{{"POST", "/outlets/other/cameras/probe"}, {"POST", "/outlets/other/cameras/onvif/profiles"}, {"DELETE", "/outlets/other/cctv-gateway"}} {
		response, err := app.Test(httptest.NewRequest(test[0], test[1], strings.NewReader(`{}`)))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatalf("%s: status %d", test[1], response.StatusCode)
		}
	}
}

func TestCCTVSetupRejectsOversizedCredentialsBeforeRelay(t *testing.T) {
	app := fiber.New()
	app.Post("/outlets/:id/cameras/probe", ProbeCameraConfiguration)
	app.Post("/outlets/:id/cameras/onvif/profiles", DiscoverCameraONVIFProfiles)
	for _, path := range []string{"/outlets/allowed/cameras/probe", "/outlets/allowed/cameras/onvif/profiles"} {
		request := httptest.NewRequest("POST", path, strings.NewReader(`{"source_url":"`+strings.Repeat("a", 50<<10)+`"}`))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("%s: status %d", path, response.StatusCode)
		}
	}
}
