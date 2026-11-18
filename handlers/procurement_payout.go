package handlers

import (
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Keuangan (admin) ─────────────────────────────────────────────────────────

// AssignCashPayout — sumber dana "Kas Kasir Outlet": buat tugas penyerahan kas
// untuk tablet kasir outlet pengajuan.
func AssignCashPayout(c *fiber.Ctx) error {
	var in models.AssignCashPayoutInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Data tidak valid"})
	}
	in.PurchaseRequestID = strings.TrimSpace(in.PurchaseRequestID)
	if !validatePurchaseRequestScope(c, in.PurchaseRequestID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	actor := purchaseActor(c)
	in.ActorName, in.ActorID = actor.Name, actor.ID
	p, err := services.AssignCashPayout(in)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: p})
}

// CancelCashPayout — tarik tugas yang belum diproses kasir.
func CancelCashPayout(c *fiber.Ctx) error {
	p, err := services.GetCashPayout(strings.TrimSpace(c.Params("id")))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Tugas tidak ditemukan"})
	}
	if !validatePurchaseRequestScope(c, p.PurchaseRequestID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	var in models.CashPayoutResolveInput
	_ = c.BodyParser(&in)
	in.ActorName = purchaseActor(c).Name
	out, err := services.CancelCashPayout(p.ID, in)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: out})
}

// ── Tablet kasir (API key outlet) ────────────────────────────────────────────

func outletFromCtx(c *fiber.Ctx) string {
	id, _ := c.Locals("outlet_id").(string)
	return strings.TrimSpace(id)
}

func cashPayoutDeviceInput(c *fiber.Ctx) (models.CashPayoutDeviceInput, error) {
	var in models.CashPayoutDeviceInput
	err := c.BodyParser(&in)
	return in, err
}

// ListOutletCashPayouts — tugas "Bayar Pengadaan" outlet ini, diambil langsung
// oleh tablet saat dibutuhkan (bukan lewat sinkron berkala).
func ListOutletCashPayouts(c *fiber.Ctx) error {
	list, err := services.ListOutletCashPayouts(outletFromCtx(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat tugas bayar pengadaan"})
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func cashPayoutDeviceAction(c *fiber.Ctx, act func(outletID, id string, in models.CashPayoutDeviceInput) (*models.ProcurementCashPayout, error)) error {
	in, err := cashPayoutDeviceInput(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Data tidak valid"})
	}
	p, err := act(outletFromCtx(c), strings.TrimSpace(c.Params("id")), in)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func ClaimCashPayout(c *fiber.Ctx) error {
	return cashPayoutDeviceAction(c, services.ClaimCashPayout)
}

func CompleteCashPayout(c *fiber.Ctx) error {
	return cashPayoutDeviceAction(c, func(outletID, id string, in models.CashPayoutDeviceInput) (*models.ProcurementCashPayout, error) {
		return services.CompleteCashPayout(outletID, id, in.MovementLocalID)
	})
}

func ReturnCashPayout(c *fiber.Ctx) error {
	return cashPayoutDeviceAction(c, services.ReturnCashPayout)
}

func ReleaseCashPayout(c *fiber.Ctx) error {
	return cashPayoutDeviceAction(c, func(outletID, id string, in models.CashPayoutDeviceInput) (*models.ProcurementCashPayout, error) {
		return services.ReleaseCashPayout(outletID, id, in.MovementLocalID)
	})
}
