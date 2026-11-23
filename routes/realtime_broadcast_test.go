package routes

import (
	"cloud-pos/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestBroadcastAfter(t *testing.T) {
	app := fiber.New()
	app.Post("/ok", broadcastAfter(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
	}, "procurement", "procurement_payment"))
	app.Post("/outlets/:outletId/act", broadcastAfter(func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	}, "reservation"))
	app.Post("/bad", broadcastAfter(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid"})
	}, "procurement"))
	app.Post("/err", broadcastAfter(func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusInternalServerError, "boom")
	}, "procurement"))

	ch := services.EventsHub.Subscribe()
	defer services.EventsHub.Unsubscribe(ch)

	// 1) 201 Created harus memancarkan kedua event secara berurutan
	req := httptest.NewRequest(http.MethodPost, "/ok", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test /ok failed: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	gotTypes := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			var ev struct {
				Type     string `json:"type"`
				OutletID string `json:"outlet_id"`
			}
			if err := json.Unmarshal([]byte(msg), &ev); err != nil {
				t.Fatalf("invalid json payload %q: %v", msg, err)
			}
			gotTypes = append(gotTypes, ev.Type)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
	if gotTypes[0] != "procurement" || gotTypes[1] != "procurement_payment" {
		t.Fatalf("unexpected events: %v", gotTypes)
	}

	// 2) Rute dengan :outletId membawa outlet_id di event
	reqOutlet := httptest.NewRequest(http.MethodPost, "/outlets/out-123/act", nil)
	if _, err := app.Test(reqOutlet); err != nil {
		t.Fatalf("app.Test /outlets/out-123/act failed: %v", err)
	}
	select {
	case msg := <-ch:
		var ev struct {
			Type     string `json:"type"`
			OutletID string `json:"outlet_id"`
		}
		if err := json.Unmarshal([]byte(msg), &ev); err != nil {
			t.Fatalf("invalid json payload: %v", err)
		}
		if ev.Type != "reservation" || ev.OutletID != "out-123" {
			t.Fatalf("unexpected outlet event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for outlet event")
	}

	// 3) 400 BadRequest & error handler TIDAK boleh memancarkan event
	for _, url := range []string{"/bad", "/err"} {
		if _, err := app.Test(httptest.NewRequest(http.MethodPost, url, nil)); err != nil {
			t.Fatalf("app.Test %s failed: %v", url, err)
		}
	}
	select {
	case msg := <-ch:
		t.Fatalf("expected no broadcast on error status, got %s", msg)
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}
