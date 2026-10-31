package handlers

import (
	"bytes"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func productClassificationError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	var conflict services.ConflictError
	switch {
	case errors.Is(err, services.ErrFnbScope):
		return c.Status(403).JSON(models.APIResponse{Error: "Outlet di luar akses Anda"})
	case errors.As(err, &invalid):
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	case errors.As(err, &conflict):
		return c.Status(409).JSON(models.APIResponse{Error: err.Error()})
	default:
		return c.Status(500).JSON(models.APIResponse{Error: "Gagal memproses klasifikasi produk"})
	}
}

func PreviewProductClassification(c *fiber.Ctx) error {
	result, err := services.PreviewProductClassification(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return productClassificationError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func ApplyProductClassification(c *fiber.Ctx) error {
	outlet := c.Query("outlet_id")
	if outlet != "" && !validateOutletAccess(c, outlet) {
		return c.Status(403).JSON(models.APIResponse{Error: "Outlet di luar akses Anda"})
	}
	// A bounded strict JSON contract rejects actor overrides, unexpected kinds,
	// unknown fields, and trailing JSON before reaching the database.
	if len(c.Body()) > 256*1024 {
		return c.Status(413).JSON(models.APIResponse{Error: "Daftar klasifikasi terlalu besar"})
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	var request models.ApplyProductClassificationRequest
	if err := decoder.Decode(&request); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "Body klasifikasi tidak valid"})
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return c.Status(400).JSON(models.APIResponse{Error: "Body klasifikasi tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if strings.TrimSpace(actor) == "" {
		actor, _ = c.Locals("admin_id").(string)
	}
	applied, err := services.ApplyProductClassification(outlet, getOutletScope(c), actor, request.Items)
	if err != nil {
		return productClassificationError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"applied": applied}})
}
