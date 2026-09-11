package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListProjects(c *fiber.Ctx) error {
	status := c.Query("status", "")
	search := c.Query("search", "")
	projects, err := services.ListProjects(status, search, getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat projek: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: projects})
}

func GetProject(c *fiber.Ctx) error {
	detail, err := services.GetProjectDetail(c.Params("id"), getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: detail})
}

func CreateProject(c *fiber.Ctx) error {
	var req models.CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	if req.WorkUnitID != "" && !validateWorkUnitAccess(c, req.WorkUnitID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false, Error: "Akses unit kerja tidak diizinkan",
		})
	}
	if req.OutletID != "" && !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false, Error: "Akses outlet tidak diizinkan",
		})
	}
	actor, _ := c.Locals("admin_username").(string)
	p, err := services.CreateProject(req, actor)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: p})
}

func UpdateProject(c *fiber.Ctx) error {
	var req models.UpdateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	if req.WorkUnitID != "" && !validateWorkUnitAccess(c, req.WorkUnitID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false, Error: "Akses unit kerja tidak diizinkan",
		})
	}
	if req.OutletID != "" && !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false, Error: "Akses outlet tidak diizinkan",
		})
	}
	p, err := services.UpdateProject(c.Params("id"), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func DeleteProject(c *fiber.Ctx) error {
	if err := services.DeleteProject(c.Params("id")); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": true}})
}
