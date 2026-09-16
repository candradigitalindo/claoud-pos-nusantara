package handlers

import (
	"errors"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// invalidOr mengubah error validasi menjadi 400 berisi pesannya, sisanya 500.
func invalidOr(c *fiber.Ctx, err error, fallback string) error {
	var ve services.ValidationError
	if errors.As(err, &ve) {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: ve.Error()})
	}
	return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: fallback + err.Error()})
}

func ListProjectMaterials(c *fiber.Ctx) error {
	list, err := services.ListProjectMaterials(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat material projek: " + err.Error()})
	}
	sum, _ := services.GetProjectMaterialSummary(c.Params("id"))
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"materials": list, "summary": sum}})
}

func ListProjectMaterialLogs(c *fiber.Ctx) error {
	list, err := services.ListProjectMaterialLogs(c.Params("mid"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat material: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func RecordProjectMaterialUsage(c *fiber.Ctx) error {
	var req models.ProjectMaterialUsageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	m, err := services.RecordMaterialUsage(c.Params("mid"), req, assetActor(c))
	if err != nil {
		return invalidOr(c, err, "Gagal mencatat pemakaian: ")
	}
	return c.JSON(models.APIResponse{Success: true, Data: m})
}

// SettleProjectMaterial menentukan nasib sisa material saat projek ditutup.
func SettleProjectMaterial(c *fiber.Ctx) error {
	var req models.ProjectMaterialSettleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	// Mengembalikan sisa ke gudang berarti menambah stok sungguhan — izinnya
	// terpisah dari izin kelola projek (keputusan B, docs §15.1).
	canStock := roleHasPermission(c, "stockledger.adjust")
	m, err := services.SettleMaterialRemainder(c.Params("mid"), req, assetActor(c), canStock, getOutletScope(c))
	if err != nil {
		return invalidOr(c, err, "Gagal menentukan sisa: ")
	}
	return c.JSON(models.APIResponse{Success: true, Data: m})
}

func ListUnsettledProjectMaterials(c *fiber.Ctx) error {
	list, err := services.ListUnsettledProjectMaterials()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat laporan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
