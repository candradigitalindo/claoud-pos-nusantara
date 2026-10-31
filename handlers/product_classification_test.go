package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProductClassificationHandlerRejectsUnsafeRequestsBeforeDatabase(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{"allowed"})
		c.Locals("admin_username", "session actor")
		return c.Next()
	})
	app.Get("/classification", PreviewProductClassification)
	app.Post("/classification", ApplyProductClassification)
	for _, method := range []string{"GET", "POST"} {
		if code, _ := doJSON(t, app, method, "/classification?outlet_id=foreign", models.ApplyProductClassificationRequest{}); code != 403 {
			t.Fatalf("foreign outlet %s: status=%d", method, code)
		}
	}
	item := models.ApplyProductClassificationItem{ProductID: services.NewULID(), Fingerprint: strings.Repeat("a", 64), SuggestedKind: "food"}
	for _, body := range []interface{}{
		models.ApplyProductClassificationRequest{},
		models.ApplyProductClassificationRequest{Items: []models.ApplyProductClassificationItem{item, item}},
		map[string]interface{}{"items": []models.ApplyProductClassificationItem{item}, "actor": "forged"},
		map[string]interface{}{"items": []map[string]string{{"product_id": item.ProductID, "fingerprint": item.Fingerprint, "suggested_kind": "invalid"}}},
		models.ApplyProductClassificationRequest{Items: make([]models.ApplyProductClassificationItem, services.MaxProductClassificationApply+1)},
	} {
		if code, _ := doJSON(t, app, "POST", "/classification", body); code != 400 {
			t.Fatalf("unsafe body status=%d want400", code)
		}
	}
	if code, _ := doJSON(t, app, "POST", "/classification", strings.Repeat("x", 256*1024)); code != 413 {
		t.Fatalf("oversized body status=%d", code)
	}
}
