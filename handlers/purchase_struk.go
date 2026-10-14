package handlers

// Struk pembelian & pengambilan barang (lihat services/purchase_struk.go).
// Izin dipilih per jenis pengajuan seperti aksi pengadaan lain: struk dicatat
// bagian pembelian atau petugas logistik jenis itu, pengambilan hanya oleh
// petugas logistik (procurement.*.pickup).

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// strukAccess memeriksa scope dokumen dan izin struk (pembelian/pengambilan).
// Bila ditolak, respons sudah ditulis: pemanggil cukup `return err`.
func strukAccess(c *fiber.Ctx, id string) (bool, error) {
	if !validatePurchaseRequestScope(c, id) {
		return false, c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return false, ferr
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return false, permissionLoadError(c)
	}
	if !acc.can(reqType, "purchasing") && !acc.can(reqType, "pickup") {
		return false, purchaseForbidden(c, "Struk pembelian dicatat bagian pembelian atau petugas logistik pengadaan jenis ini.")
	}
	return true, nil
}

// AddPurchaseStruk mencatat struk pembelian (nomor, tanggal, total, foto).
func AddPurchaseStruk(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := strukAccess(c, id); !ok {
		return err
	}
	var in models.AddPurchaseStrukInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	s, err := services.AddPurchaseStruk(id, in, purchaseActor(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: s})
}

// DeletePurchaseStruk menghapus struk yang belum dibayar.
func DeletePurchaseStruk(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := strukAccess(c, id); !ok {
		return err
	}
	if err := services.DeletePurchaseStruk(id, c.Params("strukId")); err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Message: "struk dihapus"})
}

// AddPurchasePickup mencatat pengambilan barang oleh petugas logistik.
func AddPurchasePickup(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return ferr
	}
	if ok, err := requirePurchasePerm(c, reqType, "pickup", "Pengambilan barang dicatat petugas logistik (izin Pengambilan Barang) pengadaan jenis ini."); !ok {
		return err
	}
	var in models.AddPurchasePickupInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	p, err := services.AddPurchasePickup(id, in, purchaseActor(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: p})
}
