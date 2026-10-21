package handlers

import (
	"encoding/json"

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

// ── Katalog stok global (T-20) ──
//
// Definisi item katalog (kode, nama, satuan, kategori, aktif/nonaktif) berlaku
// untuk SEMUA gudang dan menentukan jenis belanja serta meja penerimaan
// pengadaan. Katalog itu milik Gudang Induk: hanya pengguna yang berhak atas
// Gudang Induk — role bercakupan semua outlet, karena Gudang Induk tidak
// terikat outlet mana pun — yang boleh membuat, mengubah, menonaktifkan, atau
// menghapusnya. Role per outlet hanya boleh mengubah kolom PER GUDANG (stok
// minimum, saldo) untuk gudang outletnya sendiri. Dulu cek cakupan hanya
// berjalan bila warehouse_id diisi, dan toggle tidak dicek sama sekali.

const catalogDeniedMsg = "Katalog stok global hanya dikelola Gudang Induk (role dengan cakupan semua outlet). Dari outlet, Anda hanya bisa mengubah stok minimum dan saldo gudang outlet Anda."

// catalogManager: pemanggil berhak atas definisi katalog global.
func catalogManager(c *fiber.Ctx) bool { return getOutletScope(c) == nil }

// sentJSONKeys: kunci JSON tingkat atas yang benar-benar dikirim (nil bila
// badan bukan objek JSON — perlakuan lama: semua dianggap dikirim).
func sentJSONKeys(body []byte) map[string]bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys
}

// CreateStockItem menangani penambahan item stok baru ke dalam sistem.
func CreateStockItem(c *fiber.Ctx) error {
	var req models.StockItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
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
	req.Present = sentJSONKeys(c.Body())
	scope := getOutletScope(c)
	if !catalogManager(c) {
		// Role per outlet: hanya kolom per gudang untuk gudang OUTLET dalam
		// cakupannya — service tidak menyentuh katalog dari konteks outlet.
		// Tanpa gudang (konteks katalog) atau dengan Gudang Induk → ditolak.
		if req.WarehouseID == "" || !services.WarehouseInOutletScope(req.WarehouseID, scope) {
			return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
		}
	} else if req.WarehouseID != "" && !services.WarehouseMutableInScope(req.WarehouseID, scope) {
		// Konteks per-gudang (min-stock / set_stock) hanya untuk gudang dalam scope user.
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
// Menonaktifkan item katalog mengubah jenis belanja pengajuan baru, jadi
// tunduk pada aturan katalog global.
func ToggleStockItemActive(c *fiber.Ctx) error {
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.ToggleStockItemActive(c.Params("id"), actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "status diperbarui"})
}

// DeleteStockItem menghapus data item stok dari sistem.
func DeleteStockItem(c *fiber.Ctx) error {
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
	}
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

// Kategori item stok adalah bagian katalog global (dipakai semua gudang),
// jadi tunduk pada aturan yang sama dengan item katalog: hanya role bercakupan
// semua outlet yang boleh membuat, mengubah, atau menghapusnya.

func CreateStockItemCategory(c *fiber.Ctx) error {
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
	}
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
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
	}
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
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: catalogDeniedMsg})
	}
	if err := services.DeleteStockItemCategory(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "kategori dihapus"})
}

// ── Warehouses ────────────────────────────────────────────────

// ListWarehouses mengambil daftar semua gudang yang terdaftar.
// ?include_central=1 (opt-in, hanya form Transfer Stok): role per outlet ikut
// melihat Gudang Induk aktif supaya bisa memilihnya sebagai gudang asal —
// meminta stok ke Gudang Induk. Tanpa parameter, daftar tetap gudang outletnya.
func ListWarehouses(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	wtype := c.Query("type")
	includeCentral := c.Query("include_central") == "1" || c.Query("include_central") == "true"
	whs, total, err := services.ListWarehousesScoped(wtype, getOutletScope(c), includeCentral, page, limit)
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
	if req.MovType == "waste" || req.MovType == "spoiled" || req.MovType == "expired" {
		return c.Status(400).JSON(models.APIResponse{Error: "Barang rusak/expired/spoil wajib melalui laporan karantina dengan bukti foto dan persetujuan"})
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

// ── Permintaan outlet ke Gudang Induk ──
//
// WarehouseMutableInScope menganggap Gudang Induk (tanpa outlet) dalam cakupan
// siapa pun — itulah yang membuat outlet bisa MEMINTA stok: membuat transfer
// Gudang Induk → gudang outletnya. Tetapi tahap yang dijalankan atas nama
// Gudang Induk (menyetujui dan mengirim permintaan itu, membatalkannya setelah
// disetujui, menerima barang di Gudang Induk) milik role bercakupan semua
// outlet — Gudang Induk tidak terikat outlet mana pun, sama seperti katalog
// global (T-20). Tanpa aturan ini role outlet yang kebetulan memegang
// stocktransfers.approve/update bisa menyetujui dan mengirim permintaannya
// sendiri. Penerimaan di gudang outlet tetap oleh outlet.

// centralTransferStageDenied: pesan penolakan bila pemanggil ber-scope outlet
// menjalankan tahap atas nama Gudang Induk; "" = boleh.
func centralTransferStageDenied(c *fiber.Ctx, t *models.StockTransfer, status string) string {
	if getOutletScope(c) == nil {
		return ""
	}
	switch status {
	case "approved", "sent":
		if t.FromWarehouseType == "central" {
			return "Permintaan ke Gudang Induk disetujui dan dikirim oleh Gudang Induk (role dengan cakupan semua outlet). Outlet Anda mengonfirmasi penerimaan saat barang tiba."
		}
	case "cancelled":
		if t.FromWarehouseType == "central" && t.Status != "draft" {
			return "Permintaan yang sudah disetujui Gudang Induk hanya bisa dibatalkan oleh Gudang Induk."
		}
	case "received":
		if t.ToWarehouseType == "central" {
			return "Penerimaan di Gudang Induk dicatat oleh Gudang Induk (role dengan cakupan semua outlet)."
		}
	}
	return ""
}

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
	// Role per outlet: minimal satu sisi gudang OUTLET dalam cakupannya.
	// Meminta dari Gudang Induk (Gudang Induk → gudang outletnya) boleh;
	// Gudang Induk → outlet lain sudah tertolak di atas, dan Gudang Induk ↔
	// Gudang Induk adalah urusan Gudang Induk sendiri.
	if scope != nil && !services.WarehouseInOutletScope(req.FromWarehouseID, scope) && !services.WarehouseInOutletScope(req.ToWarehouseID, scope) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses gudang tidak diizinkan: salah satu gudang harus gudang outlet Anda"})
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
	scope := getOutletScope(c)
	if !services.TransferInScope(c.Params("id"), scope) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses transfer tidak diizinkan"})
	}
	// Izin per tahap, sama seperti mutasi aset: setujui dan terima adalah
	// wewenang yang berbeda dari kirim/batalkan. Satu izin untuk semuanya
	// membuat satu orang bisa menyetujui, mengirim, dan menerima sendiri.
	perm := map[string]string{
		"approved":  "stocktransfers.approve",
		"sent":      "stocktransfers.update",
		"cancelled": "stocktransfers.update",
		"received":  "stocktransfers.receive",
	}[body.Status]
	if perm == "" {
		return c.Status(400).JSON(models.APIResponse{Error: "status tidak dikenal"})
	}
	if !roleHasPermission(c, perm) {
		return c.Status(403).JSON(models.APIResponse{Error: "Anda tidak punya izin untuk tahap ini (" + perm + ")"})
	}
	// Arah: yang melepas barang adalah gudang asal, yang menerima gudang
	// tujuan. TransferInScope lolos bila salah satu sisi dalam scope, jadi
	// tanpa cek ini pengguna outlet bisa menekan "kirim" atas nama gudang lain.
	t, err := services.GetStockTransfer(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: "transfer tidak ditemukan"})
	}
	switch body.Status {
	case "received":
		if !services.WarehouseMutableInScope(t.ToWarehouseID, scope) {
			return c.Status(403).JSON(models.APIResponse{Error: "hanya gudang tujuan yang bisa menerima transfer ini"})
		}
	default:
		if !services.WarehouseMutableInScope(t.FromWarehouseID, scope) {
			return c.Status(403).JSON(models.APIResponse{Error: "hanya gudang asal yang bisa melakukan ini"})
		}
	}
	if msg := centralTransferStageDenied(c, t, body.Status); msg != "" {
		return c.Status(403).JSON(models.APIResponse{Error: msg})
	}
	if body.Status == "received" {
		return c.Status(400).JSON(models.APIResponse{Error: "Gunakan pemeriksaan penerimaan dengan foto bertimestamp, jumlah diterima, dan jumlah reject"})
	}
	// Outlet membatalkan permintaannya ke Gudang Induk hanya selagi draft;
	// syarat itu diperiksa ulang di bawah kunci baris oleh service.
	fromStatus := ""
	if scope != nil && body.Status == "cancelled" && t.FromWarehouseType == "central" {
		fromStatus = "draft"
	}
	actor, _ := c.Locals("admin_username").(string)
	t, err = services.UpdateTransferStatusFrom(c.Params("id"), body.Status, body.PhotoURL, actor, fromStatus)
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
	scope := getOutletScope(c)
	if !services.TransferInScope(transferID, scope) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses transfer tidak diizinkan"})
	}
	// Mengisi jumlah yang sampai adalah bagian dari MENERIMA: izin dan arahnya
	// mengikuti tahap 'received'.
	if !roleHasPermission(c, "stocktransfers.receive") {
		return c.Status(403).JSON(models.APIResponse{Error: "Anda tidak punya izin menerima transfer (stocktransfers.receive)"})
	}
	t, err := services.GetStockTransfer(transferID)
	if err != nil || !services.WarehouseMutableInScope(t.ToWarehouseID, scope) {
		return c.Status(403).JSON(models.APIResponse{Error: "hanya gudang tujuan yang bisa mengisi jumlah diterima"})
	}
	if msg := centralTransferStageDenied(c, t, "received"); msg != "" {
		return c.Status(403).JSON(models.APIResponse{Error: msg})
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
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "Outlet di luar akses Anda"})
	}
	rms, err := services.ListRecipeMastersScoped(outletID, search, getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rms})
}

func GetRecipeMaster(c *fiber.Ctx) error {
	if !services.RecipeMasterInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "Akses resep tidak diizinkan"})
	}
	rm, err := services.GetRecipeMaster(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rm})
}

func CreateRecipeMaster(c *fiber.Ctx) error {
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: "Master resep bersama dikelola F&B dengan cakupan semua outlet"})
	}
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
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: "Master resep bersama dikelola F&B dengan cakupan semua outlet"})
	}
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
	if !catalogManager(c) {
		return c.Status(403).JSON(models.APIResponse{Error: "Master resep bersama dikelola F&B dengan cakupan semua outlet"})
	}
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

// ListStockCatalogNames — nama + satuan katalog stok untuk saran di form
// pengadaan bahan baku. Tanpa qty/biaya, jadi boleh dibuka pemegang izin
// pengadaan bahan baku yang tidak punya izin gudang.
func ListStockCatalogNames(c *fiber.Ctx) error {
	list, err := services.ListStockCatalogNames()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: "Gagal memuat katalog stok: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
