package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"
)

func projectUsageError(c *fiber.Ctx, err error) error {
	code, message := 500, "Gagal memproses laporan penggunaan barang"
	var invalid services.ValidationError
	switch {
	case errors.Is(err, services.ErrUsageConflict):
		code, message = 409, err.Error()
	case errors.Is(err, services.ErrUsageForbidden):
		code, message = 403, "Aksi tidak diizinkan. Pencatat dan pemeriksa laporan harus berbeda."
	case errors.Is(err, sql.ErrNoRows):
		code, message = 404, "Laporan atau projek tidak ditemukan"
	case errors.As(err, &invalid):
		code, message = 400, err.Error()
	default:
		log.Printf("project usage: %v", err)
	}
	return c.Status(code).JSON(models.APIResponse{Success: false, Error: message})
}

func ListProjectUsage(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	items, total, err := services.ListProjectUsage(c.Query("project_id"), c.Query("status"), c.Query("search"), c.Query("from"), c.Query("to"), getOutletScope(c), getWorkUnitScope(c), page, limit)
	if err != nil {
		return projectUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"items": items, "total": total}})
}

func ListUsageProjects(c *fiber.Ctx) error {
	items, err := services.ListUsageProjects(getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return projectUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func ProjectUsageOverview(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validateProjectScope(c, id) {
		return projectForbidden(c)
	}
	data, err := services.GetProjectUsageOverview(id)
	if err != nil {
		return projectUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

func GetProjectUsage(c *fiber.Ctx) error {
	r, err := services.GetProjectUsage(c.Params("id"))
	if err != nil {
		return projectUsageError(c, err)
	}
	if !validateProjectScope(c, r.ProjectID) {
		return projectForbidden(c)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func SaveProjectUsage(c *fiber.Ctx) error {
	var in models.ProjectUsageInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format laporan tidak valid"})
	}
	if !validateProjectScope(c, in.ProjectID) {
		return projectForbidden(c)
	}
	if id := c.Params("id"); id != "" {
		r, err := services.GetProjectUsage(id)
		if err != nil {
			return projectUsageError(c, err)
		}
		if !validateProjectScope(c, r.ProjectID) {
			return projectForbidden(c)
		}
	}
	r, err := services.SaveProjectUsage(c.Params("id"), in, purchaseActor(c))
	if err != nil {
		return projectUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func ActProjectUsage(c *fiber.Ctx) error {
	r, err := services.GetProjectUsage(c.Params("id"))
	if err != nil {
		return projectUsageError(c, err)
	}
	if !validateProjectScope(c, r.ProjectID) {
		return projectForbidden(c)
	}
	var in models.ProjectUsageAction
	if err := c.BodyParser(&in); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format aksi tidak valid"})
	}
	permission := "assets.usage.create"
	if in.Action == "verify" || in.Action == "revise" {
		permission = "assets.usage.verify"
	}
	if !roleHasPermission(c, permission) {
		return projectUsageError(c, services.ErrUsageForbidden)
	}
	r, err = services.ActProjectUsage(r.ID, in, purchaseActor(c), roleHasPermission(c, "assets.usage.verify"))
	if err != nil {
		return projectUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}
