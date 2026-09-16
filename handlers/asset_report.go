package handlers

import (
	"fmt"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// GetAssetSummary — kartu ringkasan untuk halaman Laporan Aset.
func GetAssetSummary(c *fiber.Ctx) error {
	s, err := services.GetAssetSummary(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat ringkasan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

// GetAssetDashboard — satu layar ringkas untuk pengelola aset.
func GetAssetDashboard(c *fiber.Ctx) error {
	d, err := services.GetAssetDashboard(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func GetAssetReport(c *fiber.Ctx) error {
	rep, err := services.BuildAssetReport(c.Params("type"), c.Query("from"), c.Query("to"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rep})
}

// ExportAssetReport mengunduh laporan sebagai Excel. Zona waktu & filternya
// sama persis dengan yang tampil di layar, supaya angka di file dan di layar
// tidak pernah berbeda.
func ExportAssetReport(c *fiber.Ctx) error {
	actor, _ := c.Locals("admin_username").(string)
	buf, filename, err := services.ExportAssetReport(c.Params("type"), c.Query("from"), c.Query("to"), actor, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}
