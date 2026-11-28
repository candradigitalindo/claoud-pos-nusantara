package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func GetProductBundleComponents(c *fiber.Ctx) error {
	items, err := services.ListProductBundleComponents(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func SaveProductBundleComponents(c *fiber.Ctx) error {
	if getOutletScope(c) != nil {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: "Pengaturan bundling lintas outlet hanya dapat diubah oleh role dengan cakupan semua outlet"})
	}
	var req models.SaveProductBundleComponentsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	items, err := services.SaveProductBundleComponents(c.Params("id"), req.Components)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func ListAllBundleComponents(c *fiber.Ctx) error {
	items, err := services.ListAllBundleComponents(c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat aturan bundling: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func ListBundlePartnerOutlets(c *fiber.Ctx) error {
	outlets, err := services.ListBundlePartnerOutlets()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat daftar outlet: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: outlets})
}

func ListBundlePartnerProducts(c *fiber.Ctx) error {
	items, err := services.ListBundlePartnerProducts(c.Query("outlet_id"))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func ListCrossOutletBundleClaims(c *fiber.Ctx) error {
	resp, err := services.ListCrossOutletBundleClaims(models.CrossOutletBundleFilter{
		OutletID:  c.Query("outlet_id"),
		Direction: c.Query("direction"),
		Status:    c.Query("status"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		Search:    c.Query("search"),
	}, getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat klaim bundling: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: resp})
}

func CreateManualBundleClaim(c *fiber.Ctx) error {
	if getOutletScope(c) != nil {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: "Pembuatan kupon bundling manual hanya untuk role dengan cakupan semua outlet"})
	}
	var req models.ManualCrossOutletBundleClaimRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	claim, err := services.CreateManualBundleClaim(req, assetActor(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: claim})
}

func ClaimCrossOutletBundle(c *fiber.Ctx) error {
	var req models.ClaimCrossOutletBundleRequest
	_ = c.BodyParser(&req)
	claim, err := services.ClaimCrossOutletBundle(c.Params("id"), req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: claim, Message: "Penyerahan bundling berhasil dicatat"})
}

func ExpireCrossOutletBundleClaim(c *fiber.Ctx) error {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&body)
	claim, err := services.ExpireCrossOutletBundleClaim(c.Params("id"), body.Reason, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: claim, Message: "Kupon bundling ditandai kedaluwarsa"})
}
