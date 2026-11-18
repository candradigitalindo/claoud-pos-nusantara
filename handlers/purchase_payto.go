package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// UpdatePurchasePaymentDetails: bagian Pembelian mengubah tujuan pembayaran
// (rekening vendor terdaftar / VA marketplace / tunai-cara lain) selama
// tagihan belum lunas, termasuk memperbaiki detail yang dikembalikan
// Keuangan. Izin = pembelian jenis pengajuannya (barang/jasa atau bahan).
func UpdatePurchasePaymentDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	var input models.PurchasePaymentDetailsInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	reqType, found, err := purchaseTypeOrFail(c, id)
	if !found {
		return err
	}
	if ok, err := requirePurchasePerm(c, reqType, "purchasing", "Hanya bagian pembelian yang bisa mengubah detail pembayaran."); !ok {
		return err
	}
	actor := purchaseActor(c)
	input.ActorName, input.ActorID = actor.Name, actor.ID
	result, err := services.UpdatePurchasePaymentDetails(id, input)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: result})
}

// ReturnPurchasePaymentDetails: Keuangan mengembalikan detail pembayaran yang
// salah/kurang ke Pembelian dengan alasan (izin finance.payments.pay di rute).
func ReturnPurchasePaymentDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	var input models.PurchasePaymentReturnInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	actor := purchaseActor(c)
	input.ActorName, input.ActorID = actor.Name, actor.ID
	result, err := services.ReturnPurchasePaymentDetails(id, input)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: result})
}
