package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Pemotongan (satu bahan → beberapa hasil) ──────────────────
//
// Pola pemotongan per bahan (bisa beberapa, mis. ayam "Per Bagian" dan
// "Potong 8") adalah definisi katalog global yang berlaku di semua gudang,
// jadi hanya dikelola role bercakupan semua outlet — aturan yang sama dengan
// katalog stok (T-20). Transaksinya dicatat di gudang tempat bahan dipotong:
// role per outlet hanya di gudang OUTLET-nya sendiri, Gudang Induk hanya role
// semua outlet (WarehouseInOutletScope tidak menghitung Gudang Induk).

const breakdownTemplateDeniedMsg = "Pola pemotongan termasuk katalog stok global — hanya dikelola Gudang Induk (role dengan cakupan semua outlet)."

// ListStockItemBreakdowns: bahan yang punya pola pemotongan aktif beserta
// pola-polanya, untuk pilihan bahan & pola di form Pemotongan.
func ListStockItemBreakdowns(c *fiber.Ctx) error {
	list, err := services.ListStockItemBreakdowns()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// GetStockItemBreakdown: semua pola pemotongan satu bahan (kosong bila belum
// diatur).
func GetStockItemBreakdown(c *fiber.Ctx) error {
	t, err := services.GetStockItemBreakdown(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t})
}

// SaveStockItemBreakdown menyimpan seluruh pola pemotongan satu bahan (ganti
// semua: pola ber-id diperbarui, baru dibuat, yang tidak dikirim dihapus).
func SaveStockItemBreakdown(c *fiber.Ctx) error {
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: breakdownTemplateDeniedMsg})
	}
	var req models.StockItemBreakdownRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	t, err := services.SaveStockItemBreakdown(c.Params("id"), req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t, Message: "pola pemotongan disimpan"})
}

// CreateStockBreakdown mencatat satu pemotongan (bahan keluar, hasil masuk).
func CreateStockBreakdown(c *fiber.Ctx) error {
	var req models.StockBreakdownRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseInOutletScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Pemotongan hanya bisa dicatat di gudang outlet Anda sendiri; pemotongan di Gudang Induk dicatat role dengan cakupan semua outlet."})
	}
	actor, _ := c.Locals("admin_username").(string)
	doc, err := services.CreateStockBreakdown(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: doc, Message: "pemotongan " + doc.BreakdownNumber + " dicatat"})
}

// ListStockBreakdowns: riwayat dokumen pemotongan (paginated, ikut cakupan outlet).
func ListStockBreakdowns(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListStockBreakdowns(c.Query("warehouse_id"), c.Query("item_id"), getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: list, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}
