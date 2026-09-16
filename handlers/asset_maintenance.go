package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ListMaintenances — daftar work order lintas aset (halaman Perawatan).
func ListMaintenances(c *fiber.Ctx) error {
	list, err := services.ListMaintenances(services.MaintenanceFilter{
		AssetID:  c.Query("asset_id"),
		OutletID: c.Query("outlet_id"),
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		DueScope: c.Query("due"),
		From:     c.Query("from"),
		To:       c.Query("to"),
		Search:   c.Query("search"),
	}, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat perawatan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func GetMaintenanceSummary(c *fiber.Ctx) error {
	s, err := services.GetMaintenanceSummary(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat ringkasan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

func GetMaintenance(c *fiber.Ctx) error {
	m, err := services.GetMaintenance(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Work order tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: m})
}

// AddAssetMaintenance tetap dipakai form lama di halaman detail aset.
func AddAssetMaintenance(c *fiber.Ctx) error {
	var req models.AssetMaintenanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	m, err := services.CreateMaintenance(c.Params("id"), req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: m})
}

func maintenanceAction(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.AssetMaintenanceCompleteRequest
		_ = c.BodyParser(&req) // aksi start/cancel tidak berbadan pesan
		m, err := services.MaintenanceAction(c.Params("id"), action, req, assetActor(c), getOutletScope(c))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
		}
		return c.JSON(models.APIResponse{Success: true, Data: m})
	}
}

var (
	StartMaintenance    = maintenanceAction("start")
	CompleteMaintenance = maintenanceAction("complete")
	CancelMaintenance   = maintenanceAction("cancel")
)

// CreateMaintenancePurchaseRequest menurunkan work order menjadi Pengadaan Jasa.
func CreateMaintenancePurchaseRequest(c *fiber.Ctx) error {
	actor := assetActor(c)
	pr, err := services.CreateMaintenancePurchaseRequest(c.Params("id"), actor, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: pr})
}
