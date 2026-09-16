package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// assetActor mengambil nama pengguna untuk dicatat di buku besar aset.
func assetActor(c *fiber.Ctx) string {
	actor, _ := c.Locals("admin_username").(string)
	return actor
}

func ListAssets(c *fiber.Ctx) error {
	assets, err := services.ListAssets(
		c.Query("outlet_id"), c.Query("search"), c.Query("condition"), c.Query("status"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat aset: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: assets})
}

func GetAsset(c *fiber.Ctx) error {
	a, err := services.GetAsset(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Aset tidak ditemukan",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func CreateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	a, err := services.CreateAsset(req, assetActor(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: a})
}

func UpdateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	a, err := services.UpdateAsset(c.Params("id"), req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func DeleteAsset(c *fiber.Ctx) error {
	if err := services.DeleteAsset(c.Params("id"), assetActor(c), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

func ListAssetMaintenances(c *fiber.Ctx) error {
	// Scope guard: ensure the asset is visible before exposing its history.
	if _, err := services.GetAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Aset tidak ditemukan"})
	}
	list, err := services.ListAssetMaintenances(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func DeleteAssetMaintenance(c *fiber.Ctx) error {
	if err := services.DeleteAssetMaintenance(c.Params("id"), c.Params("mid"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// ListAssetMovements mengembalikan buku besar satu aset: pendataan, perubahan
// kondisi/lokasi, perawatan, dan penghapusan — berurut dari yang terbaru.
func ListAssetMovements(c *fiber.Ctx) error {
	// Scope guard: pastikan asetnya terlihat sebelum riwayatnya dibuka.
	if _, err := services.GetAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Aset tidak ditemukan"})
	}
	list, err := services.ListAssetMovements(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
