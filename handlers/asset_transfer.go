package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func ListAssetTransfers(c *fiber.Ctx) error {
	list, err := services.ListAssetTransfers(c.Query("status"), c.Query("outlet_id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat mutasi: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// ListTransferableAssets memberi pilihan aset untuk form mutasi — hanya yang
// sah dipilih, supaya kesalahan ketahuan sebelum dokumen dibuat, bukan sesudah.
func ListTransferableAssets(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "outlet_id wajib diisi"})
	}
	list, err := services.ListTransferableAssets(outletID, c.Query("reason"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func GetAssetTransfer(c *fiber.Ctx) error {
	t, err := services.GetAssetTransfer(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Dokumen mutasi tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t})
}

func CreateAssetTransfer(c *fiber.Ctx) error {
	var req models.AssetTransferRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	t, err := services.CreateAssetTransfer(req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: t})
}

func UpdateAssetTransfer(c *fiber.Ctx) error {
	var req models.AssetTransferRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	t, err := services.UpdateAssetTransfer(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t})
}

func DeleteAssetTransfer(c *fiber.Ctx) error {
	if err := services.DeleteAssetTransfer(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// assetTransferAction dipakai bersama oleh seluruh aksi alur; izinnya dibedakan
// di tingkat rute, bukan di sini.
func assetTransferAction(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.AssetTransferActionRequest
		// Aksi tanpa badan pesan (submit/approve/cancel) tetap sah.
		_ = c.BodyParser(&req)
		t, err := services.AssetTransferAction(c.Params("id"), action, req, assetActor(c), getOutletScope(c))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
		}
		return c.JSON(models.APIResponse{Success: true, Data: t})
	}
}

var (
	SubmitAssetTransfer  = assetTransferAction("submit")
	ApproveAssetTransfer = assetTransferAction("approve")
	RejectAssetTransfer  = assetTransferAction("reject")
	SendAssetTransfer    = assetTransferAction("send")
	ReceiveAssetTransfer = assetTransferAction("receive")
	CancelAssetTransfer  = assetTransferAction("cancel")
)
