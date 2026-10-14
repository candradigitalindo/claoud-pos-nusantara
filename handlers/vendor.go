package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListVendors(c *fiber.Ctx) error {
	activeOnly := c.Query("active") == "true"
	vendors, err := services.ListVendors(activeOnly, getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat vendor: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: vendors})
}

func GetVendor(c *fiber.Ctx) error {
	id := c.Params("id")
	v, err := services.GetVendor(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Vendor tidak ditemukan",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: v})
}

func CreateVendor(c *fiber.Ctx) error {
	var req models.CreateVendorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	v, err := services.CreateVendor(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: v})
}

func UpdateVendor(c *fiber.Ctx) error {
	id := c.Params("id")
	var req models.UpdateVendorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	v, err := services.UpdateVendor(id, req, purchaseActor(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: v})
}

func ToggleVendorActive(c *fiber.Ctx) error {
	id := c.Params("id")
	v, err := services.ToggleVendorActive(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: v})
}

func DeleteVendor(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := services.DeleteVendor(id); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true})
}

func GetVendorDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	detail, err := services.GetVendorDetail(id, getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Vendor tidak ditemukan",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: detail})
}

func ListVendorPurchases(c *fiber.Ctx) error {
	id := c.Params("id")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	data, err := services.ListVendorPurchases(id, getOutletScope(c), getWorkUnitScope(c), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat data: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}
