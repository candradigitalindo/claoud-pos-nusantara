package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ── Monitor Kedaluwarsa ───────────────────────────────────────

// GetExpiryMonitor mengembalikan ringkasan bucket + daftar batch ber-expiry.
func GetExpiryMonitor(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	resp, err := services.GetExpiryMonitor(c.Query("warehouse_id"), c.Query("bucket"), c.Query("search"),
		getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: resp})
}

// AckExpiryBatch menandai batch "sudah ditindak".
func AckExpiryBatch(c *fiber.Ctx) error {
	var req models.ExpiryAckRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.AckExpiryBatch(c.Params("batchId"), req.Note, actor, getOutletScope(c)); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "batch ditandai sudah ditindak"})
}

func ppicReportRange(c *fiber.Ctx) (string, string, error) {
	loc := services.GetTimezoneLocation()
	today := time.Now().In(loc)
	dateFrom := c.Query("date_from", today.AddDate(0, 0, -29).Format("2006-01-02"))
	dateTo := c.Query("date_to", today.Format("2006-01-02"))
	if validDate(dateFrom) == "" || validDate(dateTo) == "" || dateFrom > dateTo {
		return "", "", fmt.Errorf("rentang tanggal tidak valid (YYYY-MM-DD)")
	}
	return dateFrom, dateTo, nil
}

// pagePpicRows memotong baris laporan ke satu halaman; mengembalikan potongan
// dan jumlah total baris (untuk kontrol paginasi di UI).
func pagePpicRows[T any](rows []T, page, limit int) ([]T, int) {
	total := len(rows)
	if page < 1 {
		page = 1
	}
	if limit < 1 || page-1 > total/limit {
		return []T{}, total
	}
	start := (page - 1) * limit
	end := total
	if limit < total-start {
		end = start + limit
	}
	return rows[start:end], total
}

// ── Stock Opname ──────────────────────────────────────────────

func ListStockOpnames(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListStockOpnames(c.Query("warehouse_id"), c.Query("status"),
		getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func GetStockOpname(c *fiber.Ctx) error {
	if !services.OpnameInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "sesi opname di luar scope akses Anda"})
	}
	so, err := services.GetStockOpname(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: so})
}

func CreateStockOpname(c *fiber.Ctx) error {
	var req models.StockOpnameCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	so, err := services.CreateStockOpname(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: so})
}

func UpdateOpnameItems(c *fiber.Ctx) error {
	if !services.OpnameInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "sesi opname di luar scope akses Anda"})
	}
	var req models.OpnameCountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.UpdateOpnameCounts(c.Params("id"), req, actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "hitungan tersimpan"})
}

func SubmitStockOpname(c *fiber.Ctx) error {
	if !services.OpnameInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "sesi opname di luar scope akses Anda"})
	}
	if err := services.SubmitStockOpname(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "sesi diajukan untuk review"})
}

func ApproveStockOpname(c *fiber.Ctx) error {
	if !services.OpnameInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "sesi opname di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.ApproveStockOpname(c.Params("id"), actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "opname disetujui — penyesuaian stok diposting"})
}

func CancelStockOpname(c *fiber.Ctx) error {
	if !services.OpnameInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "sesi opname di luar scope akses Anda"})
	}
	if err := services.CancelStockOpname(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "sesi opname dibatalkan"})
}
