package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// PublicCreateOrder — POST /public/outlets/:slug/orders
// Dipanggil halaman pemesanan QR tamu. Tanpa auth, dibatasi rate limiter publik.
func PublicCreateOrder(c *fiber.Ctx) error {
	var req models.PublicOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	order, err := services.CreatePublicOrder(c.Params("slug"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"id":           order.ID,
		"status":       order.Status,
		"table_number": order.TableNumber,
		"subtotal":     order.Subtotal,
		"charges":      order.ChargeLines,
		"total_amount": order.TotalAmount,
		"payment":      order.Payment,
	}})
}

// PublicOrderStatus — GET /public/outlets/:slug/orders/:orderId
// Tamu memantau pesanannya tanpa login.
func PublicOrderStatus(c *fiber.Ctx) error {
	info, err := services.OnlineOrderStatus(c.Params("slug"), c.Params("orderId"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: info})
}

// PublicSimulateOrderPaid — POST /public/outlets/:slug/orders/:orderId/simulate-paid
// Alat uji; ditolak server kecuali penyedia pembayaran mock sedang aktif.
func PublicSimulateOrderPaid(c *fiber.Ctx) error {
	if err := services.SimulateOnlineOrderPaid(c.Params("slug"), c.Params("orderId")); err != nil {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"paid": true}})
}

// GetPendingOnlineOrders — GET /outlets/:outletId/online-orders
// Di-polling POS untuk menarik pesanan tamu yang belum diproses.
func GetPendingOnlineOrders(c *fiber.Ctx) error {
	orders, err := services.PendingOnlineOrders(c.Params("outletId"))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: orders})
}

// ClaimOnlineOrder — POST /outlets/:outletId/online-orders/:orderId/claim
// Hanya satu perangkat yang bisa menang; yang kalah mendapat 409 dan harus
// melewati pesanan itu supaya tidak ada order meja dobel.
func ClaimOnlineOrder(c *fiber.Ctx) error {
	var body struct {
		DeviceID string `json:"device_id"`
	}
	_ = c.BodyParser(&body)
	if body.DeviceID == "" {
		return c.Status(400).JSON(models.APIResponse{
			Success: false, Error: "device_id wajib diisi",
		})
	}

	ok, err := services.ClaimOnlineOrder(c.Params("outletId"), c.Params("orderId"), body.DeviceID)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	if !ok {
		return c.Status(409).JSON(models.APIResponse{
			Success: false, Error: "Pesanan sudah diambil perangkat lain",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"claimed": true}})
}

// ConfirmOnlineOrder — POST /outlets/:outletId/online-orders/:orderId/confirm
// POS memanggil ini SETELAH order meja berhasil dibuat.
func ConfirmOnlineOrder(c *fiber.Ctx) error {
	var body struct {
		LocalOrderID string `json:"local_order_id"`
	}
	_ = c.BodyParser(&body)
	if err := services.ConfirmOnlineOrder(c.Params("outletId"), c.Params("orderId"), body.LocalOrderID); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"confirmed": true}})
}

// RejectOnlineOrder — POST /outlets/:outletId/online-orders/:orderId/reject
func RejectOnlineOrder(c *fiber.Ctx) error {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&body)
	if err := services.RejectOnlineOrder(c.Params("outletId"), c.Params("orderId"), body.Reason); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"rejected": true}})
}
