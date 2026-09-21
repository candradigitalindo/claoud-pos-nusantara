package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ── WhatsApp ────────────────────────────────────────────────────────────────

func waErr(c *fiber.Ctx, err error, fallback string) error {
	var ve services.ValidationError
	if errors.As(err, &ve) {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: ve.Msg})
	}
	msg := fallback
	if err != nil {
		msg = fallback + ": " + err.Error()
	}
	return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: msg})
}

// WAStatus: status gateway (koneksi, QR) + ringkasan antrean.
func WAStatus(c *fiber.Ctx) error {
	force := c.Query("force") == "1"
	st := models.WAStatus{
		Enabled: services.GetWASettings().Enabled,
		Gateway: services.WAGatewayStatus(force),
		Queue:   services.WAQueueStats(),
	}
	return c.JSON(models.APIResponse{Success: true, Data: st})
}

func WALogin(c *fiber.Ctx) error {
	st, err := services.WAGatewayLogin()
	if err != nil {
		return waErr(c, err, "Gagal memulai pemasangan")
	}
	return c.JSON(models.APIResponse{Success: true, Data: st})
}

func WALogout(c *fiber.Ctx) error {
	if err := services.WAGatewayLogout(); err != nil {
		return waErr(c, err, "Gagal melepas nomor")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Nomor WhatsApp dilepas dari gateway"})
}

func WAGroups(c *fiber.Ctx) error {
	list, err := services.WAGatewayGroups()
	if err != nil {
		return waErr(c, err, "Gagal membaca daftar grup")
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func WASendTest(c *fiber.Ctx) error {
	var body struct {
		To      string `json:"to"`
		Message string `json:"message"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	m, err := services.SendWATest(body.To, body.Message, assetActor(c))
	if err != nil {
		return waErr(c, err, "Pesan uji gagal terkirim")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Pesan uji terkirim", Data: m})
}

func WAGetSettings(c *fiber.Ctx) error {
	s := services.GetWASettings()
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"settings": s,
		"events":   services.WAEventCatalog(s),
	}})
}

func WAUpdateSettings(c *fiber.Ctx) error {
	var in models.WASettings
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if err := services.UpdateWASettings(in); err != nil {
		return waErr(c, err, "Gagal menyimpan pengaturan")
	}
	s := services.GetWASettings()
	return c.JSON(models.APIResponse{Success: true, Message: "Pengaturan WhatsApp disimpan", Data: fiber.Map{
		"settings": s, "events": services.WAEventCatalog(s),
	}})
}

func WAPreviewTemplate(c *fiber.Ctx) error {
	var body struct {
		Event    string `json:"event"`
		Template string `json:"template"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"preview": services.WARenderPreview(body.Event, body.Template)}})
}

// ── Penerima ──

func WAListRecipients(c *fiber.Ctx) error {
	list, err := services.ListWARecipients()
	if err != nil {
		return waErr(c, err, "Gagal memuat penerima")
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func WACreateRecipient(c *fiber.Ctx) error {
	var r models.WARecipient
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	out, err := services.CreateWARecipient(r)
	if err != nil {
		return waErr(c, err, "Gagal menambah penerima")
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Message: "Penerima ditambahkan", Data: out})
}

func WAUpdateRecipient(c *fiber.Ctx) error {
	var r models.WARecipient
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	out, err := services.UpdateWARecipient(c.Params("id"), r)
	if err != nil {
		return waErr(c, err, "Gagal mengubah penerima")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Penerima disimpan", Data: out})
}

func WADeleteRecipient(c *fiber.Ctx) error {
	if err := services.DeleteWARecipient(c.Params("id")); err != nil {
		return waErr(c, err, "Gagal menghapus penerima")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Penerima dihapus"})
}

// ── Log pesan ──

func WAListMessages(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListWAMessages(c.Query("status"), c.Query("kind"), c.Query("broadcast_id"), c.Query("q"), page, limit)
	if err != nil {
		return waErr(c, err, "Gagal memuat log pesan")
	}
	totalPages := (total + limit - 1) / limit
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func WARetryMessage(c *fiber.Ctx) error {
	if err := services.RetryWAMessage(c.Params("id")); err != nil {
		return waErr(c, err, "Gagal mengulang pesan")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Pesan dimasukkan kembali ke antrean"})
}

func WACancelMessage(c *fiber.Ctx) error {
	if err := services.CancelWAMessage(c.Params("id")); err != nil {
		return waErr(c, err, "Gagal membatalkan pesan")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Pesan dibatalkan"})
}

func WARetryFailed(c *fiber.Ctx) error {
	n, err := services.RetryFailedWAMessages()
	if err != nil {
		return waErr(c, err, "Gagal mengulang pesan")
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Mengulang " + itoa(n) + " pesan gagal", Data: fiber.Map{"requeued": n}})
}

// ── Broadcast ──

func WAListBroadcasts(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListWABroadcasts(page, limit)
	if err != nil {
		return waErr(c, err, "Gagal memuat broadcast")
	}
	totalPages := (total + limit - 1) / limit
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func WAPreviewBroadcast(c *fiber.Ctx) error {
	var req models.WABroadcastRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if req.OutletID != "" && !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	p, err := services.WAPreviewBroadcast(req, getOutletScope(c))
	if err != nil {
		return waErr(c, err, "Gagal menghitung penerima")
	}
	return c.JSON(models.APIResponse{Success: true, Data: p})
}

func WACreateBroadcast(c *fiber.Ctx) error {
	var req models.WABroadcastRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if req.OutletID != "" && !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	b, err := services.CreateWABroadcast(req, assetActor(c), getOutletScope(c))
	if err != nil {
		return waErr(c, err, "Gagal membuat broadcast")
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true,
		Message: "Broadcast diantrekan ke " + itoa(b.Total) + " penerima", Data: b})
}

func WAGetBroadcast(c *fiber.Ctx) error {
	b, err := services.GetWABroadcast(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: b})
}

func WACancelBroadcast(c *fiber.Ctx) error {
	n, err := services.CancelWABroadcast(strings.TrimSpace(c.Params("id")))
	if err != nil {
		return waErr(c, err, "Gagal membatalkan broadcast")
	}
	return c.JSON(models.APIResponse{Success: true, Message: itoa(n) + " pesan yang belum terkirim dibatalkan"})
}

// WARecipientUsers: akun pengguna + event yang akan diterimanya (perutean posisi).
func WARecipientUsers(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: services.WARecipientUsers()})
}

// WARoleOptions: daftar role untuk pilihan penimpa posisi per event.
func WARoleOptions(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: services.WARoleOptions()})
}
