package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ── PPIC Dashboard ────────────────────────────────────────────

// GetPpicDashboard mengembalikan KPI, alert feed, dan data grafik dashboard PPIC.
func GetPpicDashboard(c *fiber.Ctx) error {
	stats, err := services.GetPpicDashboard(getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: stats})
}

// ── Par Level & ROP ───────────────────────────────────────────

// ListPlanningParams menampilkan parameter perencanaan per item untuk satu gudang.
func ListPlanningParams(c *fiber.Ctx) error {
	warehouseID := c.Query("warehouse_id")
	if !services.WarehouseMutableInScope(warehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	page, limit := getPagination(c)
	resp, err := services.ListPlanningParams(warehouseID, c.Query("search"), c.Query("category"),
		c.Query("below_only") == "true", page, limit)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: resp})
}

// UpsertPlanningParams menyimpan parameter perencanaan (bulk).
func UpsertPlanningParams(c *fiber.Ctx) error {
	var req models.PlanningParamsUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	scope := getOutletScope(c)
	for _, r := range req.Rows {
		if !services.WarehouseMutableInScope(r.WarehouseID, scope) {
			return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
		}
	}
	actor, _ := c.Locals("admin_username").(string)
	n, err := services.UpsertPlanningParams(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"saved": n}})
}

// AutoFillPlanningParams menghitung saran dari histori pemakaian dan menyimpannya
// untuk seluruh item ber-pemakaian di satu gudang.
func AutoFillPlanningParams(c *fiber.Ctx) error {
	var req struct {
		WarehouseID string `json:"warehouse_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	n, err := services.AutoFillPlanningParams(req.WarehouseID, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"saved": n}})
}

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

// ── Demand Forecast (Fase 2) ──────────────────────────────────

// ListForecasts mengembalikan matriks forecast produk × tanggal.
// Default rentang: hari ini s.d. 6 hari ke depan (horizon standar).
func ListForecasts(c *fiber.Ctx) error {
	loc := services.GetTimezoneLocation()
	today := time.Now().In(loc)
	dateFrom := c.Query("date_from", today.Format("2006-01-02"))
	dateTo := c.Query("date_to", today.AddDate(0, 0, 6).Format("2006-01-02"))
	if validDate(dateFrom) == "" || validDate(dateTo) == "" || dateFrom > dateTo {
		return c.Status(400).JSON(models.APIResponse{Error: "rentang tanggal tidak valid (YYYY-MM-DD)"})
	}
	outletID := c.Query("outlet_id")
	scope := getOutletScope(c)
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "akses outlet tidak diizinkan"})
	}
	page, limit := getPagination(c)
	resp, err := services.ListForecasts(outletID, c.Query("search"), c.Query("category"), dateFrom, dateTo, scope, page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: resp})
}

// GetForecastSalesHistory — histori penjualan harian 35 hari satu produk
// (bahan rumus WMA), untuk baris expand di halaman Demand Forecast.
func GetForecastSalesHistory(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID == "" || !validateOutletAccess(c, outletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "akses outlet tidak diizinkan"})
	}
	points, err := services.GetForecastSalesHistory(outletID, c.Query("product"))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: points})
}

// UpdateForecasts menyimpan penyesuaian manual (bulk).
func UpdateForecasts(c *fiber.Ctx) error {
	var req models.ForecastUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	for _, r := range req.Rows {
		if !validateOutletAccess(c, r.OutletID) {
			return c.Status(403).JSON(models.APIResponse{Error: "akses outlet tidak diizinkan"})
		}
	}
	actor, _ := c.Locals("admin_username").(string)
	n, err := services.UpdateForecasts(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"saved": n}})
}

// GenerateForecasts menjalankan WMA per hari-dalam-minggu untuk horizon ke depan.
func GenerateForecasts(c *fiber.Ctx) error {
	var req models.ForecastGenerateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	// Isi aktual dulu supaya evaluasi ikut segar, lalu generate dalam scope user.
	services.FillForecastActuals()
	n, err := services.GenerateForecasts(getOutletScope(c), req.HorizonDays, actor)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"generated": n}})
}

// ── MRP (Fase 2) ──────────────────────────────────────────────

func RunMrp(c *fiber.Ctx) error {
	var req models.MrpRunRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	run, err := services.RunMrp(req, actor, getOutletScope(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: run})
}

func ListMrpRuns(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListMrpRuns(getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func GetMrpRun(c *fiber.Ctx) error {
	if !services.MrpRunInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "run MRP di luar scope akses Anda"})
	}
	run, err := services.GetMrpRun(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: run})
}

func CreateMrpPurchaseRequest(c *fiber.Ctx) error {
	if !services.MrpRunInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "run MRP di luar scope akses Anda"})
	}
	var req models.MrpExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	// Pengaju = akun yang login (id + nama tampilan), sama dengan form (T-7).
	pr, err := services.CreateMrpPurchaseRequest(c.Params("id"), req, purchaseActor(c))
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: pr})
}

func CreateMrpTransfer(c *fiber.Ctx) error {
	if !services.MrpRunInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "run MRP di luar scope akses Anda"})
	}
	var req models.MrpExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	transfers, err := services.CreateMrpTransfer(c.Params("id"), req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: transfers})
}

// ── Rencana Produksi & Work Order (Fase 3) ────────────────────

func ListProductionPlans(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListProductionPlans(c.Query("warehouse_id"), c.Query("status"), getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func GetProductionPlan(c *fiber.Ctx) error {
	if !services.ProductionPlanInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "rencana di luar scope akses Anda"})
	}
	plan, err := services.GetProductionPlan(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: plan})
}

func CreateProductionPlan(c *fiber.Ctx) error {
	var req models.ProductionPlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	plan, err := services.CreateProductionPlan(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: plan})
}

func ApproveProductionPlan(c *fiber.Ctx) error {
	if !services.ProductionPlanInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "rencana di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.ApproveProductionPlan(c.Params("id"), actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "rencana disetujui"})
}

func ReleaseProductionPlan(c *fiber.Ctx) error {
	if !services.ProductionPlanInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "rencana di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	plan, err := services.ReleaseProductionPlan(c.Params("id"), actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: plan, Message: "rencana di-release — work order dibuat"})
}

func CancelProductionPlan(c *fiber.Ctx) error {
	if !services.ProductionPlanInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "rencana di luar scope akses Anda"})
	}
	if err := services.CancelProductionPlan(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "rencana dibatalkan"})
}

func ListProducibleItems(c *fiber.Ctx) error {
	warehouseID := c.Query("warehouse_id")
	if !services.WarehouseMutableInScope(warehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	list, err := services.ListProducibleItems(warehouseID)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func ListWorkOrders(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	list, total, err := services.ListWorkOrders(c.Query("warehouse_id"), c.Query("status"), getOutletScope(c), page, limit)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(models.PaginatedResponse{Success: true, Data: list, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func GetWorkOrder(c *fiber.Ctx) error {
	if !services.WorkOrderInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "WO di luar scope akses Anda"})
	}
	wo, err := services.GetWorkOrder(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: wo})
}

func CreateWorkOrder(c *fiber.Ctx) error {
	var req models.WorkOrderCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	if !services.WarehouseMutableInScope(req.WarehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	wo, err := services.CreateWorkOrder(req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: wo})
}

func StartWorkOrder(c *fiber.Ctx) error {
	if !services.WorkOrderInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "WO di luar scope akses Anda"})
	}
	actor, _ := c.Locals("admin_username").(string)
	if err := services.StartWorkOrder(c.Params("id"), actor); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "WO dimulai"})
}

func FinishWorkOrder(c *fiber.Ctx) error {
	if !services.WorkOrderInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "WO di luar scope akses Anda"})
	}
	var req models.WorkOrderFinishRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	wo, err := services.FinishWorkOrder(c.Params("id"), req, actor)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: wo, Message: "produksi diposting ke stok"})
}

func CancelWorkOrder(c *fiber.Ctx) error {
	if !services.WorkOrderInScope(c.Params("id"), getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "WO di luar scope akses Anda"})
	}
	if err := services.CancelWorkOrder(c.Params("id")); err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "WO dibatalkan"})
}

// ── Laporan PPIC (Fase 3) ─────────────────────────────────────

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

func GetPpicReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := ppicReportRange(c)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	warehouseID := c.Query("warehouse_id")
	if warehouseID != "" && !services.WarehouseMutableInScope(warehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "akses outlet tidak diizinkan"})
	}
	scope := getOutletScope(c)

	// Ringkasan/statistik dihitung dari data penuh di service; baris tabel
	// dikirim per halaman supaya payload & render browser ringan.
	page, limit := getPagination(c)

	var data interface{}
	switch c.Params("tab") {
	case "sold":
		var rep *models.PpicSoldReport
		rep, err = services.GetPpicSoldReport(dateFrom, dateTo, outletID, c.Query("search"), scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "hpp":
		idealPct, _ := strconv.ParseFloat(c.Query("ideal_pct", "35"), 64)
		var rep *models.PpicHppReport
		rep, err = services.GetPpicHppReport(outletID, idealPct, scope)
		if rep != nil {
			if s := strings.ToLower(c.Query("search")); s != "" {
				filtered := rep.Rows[:0:0]
				for _, r := range rep.Rows {
					if strings.Contains(strings.ToLower(r.ProductName), s) || strings.Contains(strings.ToLower(r.Category), s) {
						filtered = append(filtered, r)
					}
				}
				rep.Rows = filtered
			}
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "variance":
		var rep *models.PpicVarianceReport
		rep, err = services.GetPpicVarianceReport(dateFrom, dateTo, warehouseID, scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "production":
		var rep *models.PpicProductionReport
		rep, err = services.GetPpicProductionReport(dateFrom, dateTo, warehouseID, scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "forecast":
		var rep *models.PpicForecastAccReport
		rep, err = services.GetPpicForecastAccReport(dateFrom, dateTo, outletID, scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "otif":
		var rep *models.PpicOtifReport
		rep, err = services.GetPpicOtifReport(dateFrom, dateTo, scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	case "distribution":
		var rep *models.PpicDistributionReport
		rep, err = services.GetPpicDistributionReport(dateFrom, dateTo, outletID, c.Query("search"), scope)
		if rep != nil {
			rep.Rows, rep.TotalRows = pagePpicRows(rep.Rows, page, limit)
		}
		data = rep
	default:
		return c.Status(400).JSON(models.APIResponse{Error: "tab laporan tidak dikenal"})
	}
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

// pagePpicRows memotong baris laporan ke satu halaman; mengembalikan potongan
// dan jumlah total baris (untuk kontrol paginasi di UI).
func pagePpicRows[T any](rows []T, page, limit int) ([]T, int) {
	total := len(rows)
	start := (page - 1) * limit
	if start >= total {
		return []T{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total
}

// ExportPpicReport mengunduh satu tab Laporan PPIC sebagai Excel — filter &
// scope identik dengan tampilan layar.
func ExportPpicReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := ppicReportRange(c)
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Error: err.Error()})
	}
	warehouseID := c.Query("warehouse_id")
	if warehouseID != "" && !services.WarehouseMutableInScope(warehouseID, getOutletScope(c)) {
		return c.Status(403).JSON(models.APIResponse{Error: "gudang di luar scope akses Anda"})
	}
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(403).JSON(models.APIResponse{Error: "akses outlet tidak diizinkan"})
	}
	idealPct, _ := strconv.ParseFloat(c.Query("ideal_pct", "35"), 64)
	actor, _ := c.Locals("admin_username").(string)
	data, filename, err := services.BuildPpicReportExcel(c.Params("tab"), dateFrom, dateTo, warehouseID, outletID, idealPct, actor, getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Error: "Gagal membuat file Excel: " + err.Error()})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	return c.Send(data)
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
