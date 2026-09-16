package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListAssetCategories(c *fiber.Ctx) error {
	list, err := services.ListAssetCategories(c.Query("include_inactive") == "1")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat kategori: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func CreateAssetCategory(c *fiber.Ctx) error {
	var req models.AssetCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	req.IsActive = true
	cat, err := services.CreateAssetCategory(req)
	if err != nil {
		return invalidOr(c, err, "Gagal menambah kategori: ")
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: cat})
}

func UpdateAssetCategory(c *fiber.Ctx) error {
	var req models.AssetCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	cat, err := services.UpdateAssetCategory(c.Params("id"), req)
	if err != nil {
		return invalidOr(c, err, "Gagal menyimpan kategori: ")
	}
	return c.JSON(models.APIResponse{Success: true, Data: cat})
}

func DeleteAssetCategory(c *fiber.Ctx) error {
	if err := services.DeleteAssetCategory(c.Params("id")); err != nil {
		return invalidOr(c, err, "Gagal menghapus kategori: ")
	}
	return c.JSON(models.APIResponse{Success: true})
}
