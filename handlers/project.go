package handlers

import (
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// projectForbidden: projek di luar cakupan role pemanggil (T-49).
func projectForbidden(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
		Success: false, Error: "Akses projek tidak diizinkan",
	})
}

// requireProjectOwner: role ber-scope unit kerja wajib menautkan projek ke
// unit/outlet dalam cakupannya — projek tanpa unit lenyap dari daftarnya
// sendiri dan jatuh menjadi projek pusat (T-49).
func requireProjectOwner(c *fiber.Ctx, outletID, workUnitID string) error {
	if getWorkUnitScope(c) == nil && getOutletScope(c) == nil {
		return nil
	}
	if strings.TrimSpace(outletID) == "" && strings.TrimSpace(workUnitID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pilih unit kerja projek — projek tanpa unit kerja tidak akan terlihat oleh role Anda",
		})
	}
	return nil
}

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
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
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
	if err := requireProjectOwner(c, req.OutletID, req.WorkUnitID); err != nil {
		return err
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
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
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
	if err := requireProjectOwner(c, req.OutletID, req.WorkUnitID); err != nil {
		return err
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
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
	if err := services.DeleteProject(c.Params("id")); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": true}})
}

// ── RAB per baris ─────────────────────────────────────────────────────────

// GetProjectRab dipakai halaman projek dan form pengajuan (untuk memilih baris
// RAB beserta sisanya), jadi izinnya seluas daftar projek.
func GetProjectRab(c *fiber.Ctx) error {
	// Detail & RAB projek hanya untuk projek dalam cakupan (T-24): dulu id
	// projek outlet lain cukup untuk membaca barisnya lalu menyerapnya.
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
	res, err := services.GetProjectRab(c.Params("id"), getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: res})
}

func SaveProjectRab(c *fiber.Ctx) error {
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
	var req models.SaveProjectRabRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	items, err := services.SaveProjectRab(c.Params("id"), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: items})
}

func SetProjectRab(c *fiber.Ctx) error {
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
	actor, _ := c.Locals("admin_username").(string)
	p, err := services.SetProjectRab(c.Params("id"), actor)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func ReopenProjectRab(c *fiber.Ctx) error {
	if !validateProjectScope(c, c.Params("id")) {
		return projectForbidden(c)
	}
	p, err := services.ReopenProjectRab(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}
