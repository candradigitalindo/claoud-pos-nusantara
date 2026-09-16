package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListAssetDisposals(c *fiber.Ctx) error {
	list, err := services.ListAssetDisposals(c.Query("status"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat penghapusan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func CreateAssetDisposal(c *fiber.Ctx) error {
	var req models.AssetDisposalRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	d, err := services.CreateAssetDisposal(req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: d})
}

func ApproveAssetDisposal(c *fiber.Ctx) error {
	var req models.AssetDisposalApproveRequest
	_ = c.BodyParser(&req)
	d, err := services.ApproveAssetDisposal(c.Params("id"), req.Approve, req.Reason, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

// ── Opname ──────────────────────────────────────────────────────────────────

func ListAssetOpnames(c *fiber.Ctx) error {
	list, err := services.ListAssetOpnames(c.Query("status"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat opname: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func GetAssetOpname(c *fiber.Ctx) error {
	s, err := services.GetAssetOpname(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Sesi opname tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

func CreateAssetOpname(c *fiber.Ctx) error {
	var req models.AssetOpnameCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	s, err := services.CreateAssetOpname(req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: s})
}

func SaveAssetOpnameCount(c *fiber.Ctx) error {
	var req models.AssetOpnameCountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	s, err := services.SaveOpnameCount(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

func ApproveAssetOpname(c *fiber.Ctx) error {
	s, err := services.ApproveAssetOpname(c.Params("id"), assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}
