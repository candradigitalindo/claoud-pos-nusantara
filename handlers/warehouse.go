package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Stock Items ───────────────────────────────────────────────

// ListStockItems mengambil daftar item stok (bahan baku/barang) dengan dukungan pencarian dan filter status.
func ListStockItems(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	search := c.Query("search")
	activeOnly := c.Query("active_only") == "true"
	warehouseID := c.Query("warehouse_id")
	items, total, err := services.ListStockItems(search, activeOnly, warehouseID, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: items, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}

// GetStockItem mengambil detail informasi satu item stok berdasarkan ID.
func GetStockItem(c *fiber.Ctx) error {
	item, err := services.GetStockItem(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: item})
}

// CreateStockItem menangani penambahan item stok baru ke dalam sistem.
func CreateStockItem(c *fiber.Ctx) error {
	var req models.StockItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	// Konteks per-gudang (min-stock / stok awal) hanya untuk gudang dalam scope user.
	if req.WarehouseID != "" && !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	item, err := services.CreateStockItem(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: item})
}

// UpdateStockItem memperbarui informasi item stok yang sudah ada.
func UpdateStockItem(c *fiber.Ctx) error {
	var req models.StockItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	// Konteks per-gudang (min-stock / set_stock) hanya untuk gudang dalam scope user.
	if req.WarehouseID != "" && !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	item, err := services.UpdateStockItem(c.Params("id"), req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: item})
}

// ToggleStockItemActive mengubah status aktif/nonaktif dari sebuah item stok.
func ToggleStockItemActive(c *fiber.Ctx) error {
	if err := services.ToggleStockItemActive(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "status diperbarui"})
}

// DeleteStockItem menghapus data item stok dari sistem.
func DeleteStockItem(c *fiber.Ctx) error {
	if err := services.DeleteStockItem(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "bahan baku dihapus"})
}

// ── Stock Item Categories ─────────────────────────────────────

func ListStockItemCategories(c *fiber.Ctx) error {
	cats, err := services.ListStockItemCategories()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cats})
}

func CreateStockItemCategory(c *fiber.Ctx) error {
	var req models.StockItemCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	cat, err := services.CreateStockItemCategory(req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: cat})
}

func UpdateStockItemCategory(c *fiber.Ctx) error {
	var req models.StockItemCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	cat, err := services.UpdateStockItemCategory(c.Params("id"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cat})
}

func DeleteStockItemCategory(c *fiber.Ctx) error {
	if err := services.DeleteStockItemCategory(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "kategori dihapus"})
}

// ── Warehouses ────────────────────────────────────────────────

// ListWarehouses mengambil daftar semua gudang yang terdaftar.
func ListWarehouses(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	wtype := c.Query("type")
	whs, total, err := services.ListWarehouses(wtype, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: whs, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}

// GetWarehouse mengambil detail informasi satu gudang berdasarkan ID.
func GetWarehouse(c *fiber.Ctx) error {
	wh, err := services.GetWarehouse(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	if !validateOutletAccess(c, wh.OutletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses ditolak: gudang ini bukan milik outlet Anda"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: wh})
}

// CreateWarehouse menangani pembuatan data gudang baru.
func CreateWarehouse(c *fiber.Ctx) error {
	var req models.WarehouseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	wh, err := services.CreateWarehouse(req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: wh})
}

// UpdateWarehouse memperbarui informasi data gudang yang sudah ada.
func UpdateWarehouse(c *fiber.Ctx) error {
	var req models.WarehouseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	wh, err := services.UpdateWarehouse(c.Params("id"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: wh})
}

// DeleteWarehouse menghapus data gudang dari sistem.
func DeleteWarehouse(c *fiber.Ctx) error {
	if err := services.DeleteWarehouse(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "gudang dihapus"})
}

// ── Stock Ledger ──────────────────────────────────────────────

// GetStockLedger mengambil data saldo stok (buku stok) berdasarkan gudang dan item.
func GetStockLedger(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	warehouseID := c.Query("warehouse_id")
	itemID := c.Query("item_id")
	search := c.Query("search")
	lowOnly := c.Query("low_stock") == "true"
	resp, err := services.GetStockLedger(warehouseID, itemID, search, lowOnly, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (resp.Total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(fiber.Map{
		"success":           true,
		"data":              resp.Data,
		"page":              page,
		"limit":             limit,
		"total":             resp.Total,
		"total_pages":       totalPages,
		"low_stock_count":   resp.LowStockCount,
		"total_asset_value": resp.TotalAssetValue,
	})
}

// ── Stock Movements ───────────────────────────────────────────

// GetStockMovements mengambil riwayat pergerakan stok (masuk/keluar/penyesuaian).
func GetStockMovements(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	warehouseID := c.Query("warehouse_id")
	itemID := c.Query("item_id")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	rows, total, err := services.GetStockMovements(warehouseID, itemID, dateFrom, dateTo, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: rows, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}

// CreateAdjustment menangani pencatatan penyesuaian stok manual.
func CreateAdjustment(c *fiber.Ctx) error {
	var req models.AdjustmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.CreateAdjustment(req, actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "pergerakan stok berhasil dicatat"})
}

// ── Stock Transfers ───────────────────────────────────────────

// ListStockTransfers mengambil daftar pengiriman stok antar gudang.
func ListStockTransfers(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	status := c.Query("status")
	warehouseID := c.Query("warehouse_id")
	transfers, total, err := services.ListStockTransfers(status, warehouseID, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: transfers, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}

// GetStockTransfer mengambil detail informasi pengiriman stok antar gudang.
func GetStockTransfer(c *fiber.Ctx) error {
	if !services.TransferInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses transfer tidak diizinkan"})
	}
	t, err := services.GetStockTransfer(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t})
}

// CreateStockTransfer menangani pembuatan pengajuan pengiriman stok baru antar gudang.
func CreateStockTransfer(c *fiber.Ctx) error {
	var req models.StockTransferRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	scope := getOutletScope(c)
	if !services.WarehouseMutableInScope(req.FromWarehouseID, scope) || !services.WarehouseMutableInScope(req.ToWarehouseID, scope) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	t, err := services.CreateStockTransfer(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: t})
}

// UpdateTransferStatus memperbarui status pengiriman stok (misal: dikirim, diterima, dibatalkan).
func UpdateTransferStatus(c *fiber.Ctx) error {
	var body struct {
		Status string `json:"status"`
		// PhotoURL: bukti barang saat dikirim dari gudang induk ke gudang outlet.
		PhotoURL string `json:"photo_url"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.TransferInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses transfer tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	t, err := services.UpdateTransferStatusWithPhoto(c.Params("id"), body.Status, body.PhotoURL, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: t})
}

// UpdateReceivedQty menangani pembaruan kuantitas barang yang benar-benar diterima pada saat pengiriman stok.
func UpdateReceivedQty(c *fiber.Ctx) error {
	transferID := c.Params("id")
	itemID := c.Params("itemId")
	var body struct {
		ReceivedQtyBase float64 `json:"received_qty_base"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.TransferInScope(transferID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses transfer tidak diizinkan"})
	}
	if err := services.UpdateTransferReceivedQty(transferID, itemID, body.ReceivedQtyBase); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "qty penerimaan diperbarui"})
}

// ── Product Recipes ───────────────────────────────────────────

// GetProductRecipes mengambil daftar resep (kebutuhan bahan baku) untuk sebuah produk.
func GetProductRecipes(c *fiber.Ctx) error {
	recipes, err := services.GetProductRecipes(c.Params("productId"))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: recipes})
}

// SaveProductRecipes menyimpan atau memperbarui daftar resep untuk sebuah produk.
func SaveProductRecipes(c *fiber.Ctx) error {
	var req models.ProductRecipeRequest
	req.ProductID = c.Params("productId")
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if err := services.SaveProductRecipes(req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "resep produk disimpan"})
}

// ── Semi-Finished Good (SFG) Recipes & Production ────────────

func GetStockItemRecipes(c *fiber.Ctx) error {
	recipes, err := services.GetStockItemRecipes(c.Params("id"))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: recipes})
}

func SaveStockItemRecipes(c *fiber.Ctx) error {
	var req models.StockItemRecipeRequest
	req.ParentItemID = c.Params("id")
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if err := services.SaveStockItemRecipes(req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "resep bahan baku disimpan"})
}

func ProduceStockItem(c *fiber.Ctx) error {
	var req models.ProduceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	// Item yang diproduksi mengikuti path (/stock-items/:id/produce), bukan body.
	req.ItemID = c.Params("id")
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.ProduceStockItem(req, actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "produksi bahan baku berhasil"})
}

// ── Master Recipe System ─────────────────────────────────────

func ListRecipeMasters(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	search := c.Query("search")
	rms, err := services.ListRecipeMasters(outletID, search)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rms})
}

func GetRecipeMaster(c *fiber.Ctx) error {
	rm, err := services.GetRecipeMaster(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rm})
}

func CreateRecipeMaster(c *fiber.Ctx) error {
	var req models.RecipeMasterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	id, err := services.SaveRecipeMaster("", req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: id})
}

func UpdateRecipeMaster(c *fiber.Ctx) error {
	var req models.RecipeMasterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	id, err := services.SaveRecipeMaster(c.Params("id"), req)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: id})
}

func DeleteRecipeMaster(c *fiber.Ctx) error {
	if err := services.DeleteRecipeMaster(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "resep master dihapus"})
}

// ── Stock Waste ───────────────────────────────────────────

func ListStockWastes(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	warehouseID := c.Query("warehouse_id")
	search := c.Query("search")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	wastes, total, err := services.GetStockWastes(warehouseID, search, dateFrom, dateTo, getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{
		Success: true, Data: wastes, Page: page, Limit: limit,
		Total: total, TotalPages: totalPages,
	})
}

func CreateStockWaste(c *fiber.Ctx) error {
	var req models.StockWasteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.CreateStockWaste(req, actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "pembuangan stok berhasil dicatat"})
}


func GetWarehouseDashboard(c *fiber.Ctx) error {
	scopeIDs := getOutletScope(c)
	data, err := services.GetWarehouseDashboard(scopeIDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard gudang: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

// ── Penerimaan Barang (Goods Receipt / GRN) ───────────────────

// CreateGoodsReceipt mencatat penerimaan barang ke gudang (stok masuk).
func CreateGoodsReceipt(c *fiber.Ctx) error {
	var req models.GoodsReceiptRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan"})
	}
	actor, _ := c.Locals("admin_username").(string)
	g, err := services.CreateGoodsReceipt(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: g, Message: "Penerimaan barang berhasil dicatat"})
}

// ListGoodsReceipts mengambil daftar dokumen penerimaan barang (scoped).
func ListGoodsReceipts(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListGoodsReceipts(c.Query("warehouse_id"), getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

// GetGoodsReceipt mengambil detail satu dokumen penerimaan barang.
func GetGoodsReceipt(c *fiber.Ctx) error {
	if !services.GoodsReceiptInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses penerimaan tidak diizinkan"})
	}
	g, err := services.GetGoodsReceipt(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: "Penerimaan tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: g})
}
