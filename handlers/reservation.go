package handlers

import (
	"path/filepath"
	"strconv"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Admin ───────────────────────────────────────────────────

func ListReservations(c *fiber.Ctx) error {
	data, err := services.ListReservations(
		c.Query("outlet_id"), c.Query("status"), c.Query("date_from"), c.Query("date_to"), getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat reservasi: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

func GetReservation(c *fiber.Ctx) error {
	r, err := services.GetReservation(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Reservasi tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func CreateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.CreateReservation(req, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: r})
}

func UpdateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.UpdateReservation(c.Params("id"), req, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

func UpdateReservationStatus(c *fiber.Ctx) error {
	var body models.ReservationStatusRequest
	if err := c.BodyParser(&body); err != nil || body.Status == "" {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Status tidak valid"})
	}
	r, err := services.UpdateReservationStatus(c.Params("id"), body.Status, body.Disposition, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: r})
}

// ── Pembayaran (uang muka) ──────────────────────────────────────────────────

func ListReservationPayments(c *fiber.Ctx) error {
	if _, err := services.GetReservation(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Reservasi tidak ditemukan"})
	}
	list, err := services.ListReservationPayments(c.Params("id"))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// AddReservationPayment: admin mencatat uang yang ia lihat sendiri → langsung sah.
func AddReservationPayment(c *fiber.Ctx) error {
	var req models.ReservationPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	p, err := services.AddReservationPayment(c.Params("id"), req, assetActor(c), true, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: p})
}

func ValidateReservationPayment(c *fiber.Ctx) error {
	p, err := services.ValidateReservationPayment(c.Params("id"), c.Params("pid"), assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func RejectReservationPayment(c *fiber.Ctx) error {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&body)
	p, err := services.RejectReservationPayment(c.Params("id"), c.Params("pid"), body.Reason, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func GetReservationSettings(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: services.GetReservationSettings()})
}

func UpdateReservationSettings(c *fiber.Ctx) error {
	var body struct {
		DpPercent float64 `json:"dp_percent"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if err := services.SetReservationDpPercent(body.DpPercent); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: services.GetReservationSettings()})
}

// ── App POS (auth API key outlet) ───────────────────────────────────────────

func OutletListReservations(c *fiber.Ctx) error {
	outletID, _ := c.Locals("outlet_id").(string)
	list, err := services.ListOutletReservations(outletID, c.Query("date"))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func OutletSettleReservation(c *fiber.Ctx) error {
	outletID, _ := c.Locals("outlet_id").(string)
	var body struct {
		TransactionLocalID string `json:"transaction_local_id"`
		CashierName        string `json:"cashier_name"`
	}
	_ = c.BodyParser(&body)
	if err := services.SettleReservation(outletID, c.Params("rid"), body.TransactionLocalID, body.CashierName); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

func DeleteReservation(c *fiber.Ctx) error {
	if err := services.DeleteReservation(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// ── Public (no auth) ────────────────────────────────────────

func PublicGetMenu(c *fiber.Ctx) error {
	menu, err := services.GetPublicMenu(c.Params("slug"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Outlet tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: menu})
}

// PublicGetReservation: status untuk pelanggan (link cek status / halaman sukses).
func PublicGetReservation(c *fiber.Ctx) error {
	s, err := services.PublicReservationStatus(c.Params("slug"), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Reservasi tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

// PublicSubmitReservationPayment: pelanggan mengunggah bukti transfer
// (multipart: file, type, amount, bank_account_id, paid_at, notes). Hanya
// gambar, maksimal 5MB; lahir 'pending' sampai divalidasi admin.
func PublicSubmitReservationPayment(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Foto bukti transfer wajib diunggah"})
	}
	if file.Size > 5*1024*1024 {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Ukuran foto maksimal 5MB"})
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}[ext] || !sniffContentAllowed(file, "image/") {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Bukti harus berupa gambar (JPG, PNG, atau WebP)"})
	}
	amount, _ := strconv.ParseFloat(strings.ReplaceAll(c.FormValue("amount"), ".", ""), 64)
	url, err := storeUpload(c, file, ext)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	p, err := services.PublicSubmitReservationPayment(c.Params("slug"), c.Params("id"), models.ReservationPaymentRequest{
		Type: c.FormValue("type"), Amount: amount, Method: c.FormValue("method"),
		BankAccountID: c.FormValue("bank_account_id"), ProofURL: url,
		PaidAt: c.FormValue("paid_at"), Notes: c.FormValue("notes"),
	})
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: p,
		Message: "Bukti diterima. Kami validasi dulu, lalu reservasi Anda dikonfirmasi."})
}

func PublicCreateReservation(c *fiber.Ctx) error {
	var req models.ReservationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	r, err := services.CreatePublicReservation(c.Params("slug"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"id": r.ID, "customer_name": r.CustomerName, "status": r.Status,
		"reservation_date": r.ReservationDate, "reservation_time": r.ReservationTime,
		"total": r.Total, "down_payment": r.DownPayment, "remaining": r.Remaining,
	}})
}
