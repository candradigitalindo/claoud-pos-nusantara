package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ExportSalesReport mengunduh laporan penjualan sebagai file Excel (.xlsx).
// Filter tanggal & outlet identik dengan GetSalesReport, termasuk pembatasan
// scope outlet berdasarkan role admin — user hanya bisa mengekspor data
// outlet yang memang boleh ia lihat.
func ExportSalesReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	scopeIDs := getOutletScope(c)

	data, filename, err := services.BuildSalesReportExcel(dateFrom, dateTo, outletID, scopeIDs)
	if err != nil {
		log.Printf("ExportSalesReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan penjualan.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportDiscountReport mengunduh laporan diskon & komplimen sebagai file Excel
// (.xlsx). Filter (tanggal, outlet) identik dengan GetDiscountReport, termasuk
// pembatasan scope outlet berdasarkan role.
func ExportDiscountReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildDiscountReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportDiscountReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan diskon.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportVoidReport mengunduh laporan void (void transaksi + void item) sebagai
// file Excel (.xlsx). Filter (tanggal, outlet) identik dengan GetVoidReport,
// termasuk pembatasan scope outlet berdasarkan role. Data titipan tidak ikut
// karena memakai permission terpisah.
func ExportVoidReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildVoidReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportVoidReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan void.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportTaxReport mengunduh laporan pajak restoran (PB1) sebagai file Excel
// (.xlsx). Filter (tanggal, outlet) identik dengan GetTaxReport, termasuk
// pembatasan scope outlet berdasarkan role.
func ExportTaxReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildTaxReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportTaxReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan pajak.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportBalanceReport mengunduh laporan neraca sebagai file Excel (.xlsx).
// Filter identik dengan GetBalanceReport, termasuk pembatasan scope outlet
// berdasarkan role.
func ExportBalanceReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildBalanceReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportBalanceReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan neraca.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportProfitLossReport mengunduh laporan laba rugi F&B sebagai file Excel
// (.xlsx). Filter (tanggal, outlet) identik dengan GetProfitLossReport,
// termasuk pembatasan scope outlet berdasarkan role.
func ExportProfitLossReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildProfitLossReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportProfitLossReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan laba rugi.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportCashFlowReport mengunduh laporan arus kas sebagai file Excel (.xlsx).
// Filter (tanggal, outlet) identik dengan GetCashFlowReport, termasuk
// pembatasan scope outlet berdasarkan role.
func ExportCashFlowReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildCashFlowReportExcel(dateFrom, dateTo, outletID, getOutletScope(c))
	if err != nil {
		log.Printf("ExportCashFlowReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan arus kas.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportGeneralLedger mengunduh buku besar sebagai file Excel (.xlsx). Filter
// (tanggal, outlet, akun) identik dengan GetGeneralLedger, termasuk pembatasan
// scope outlet berdasarkan role.
func ExportGeneralLedger(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildGeneralLedgerExcel(
		dateFrom, dateTo, outletID, c.Query("account"), getOutletScope(c))
	if err != nil {
		log.Printf("ExportGeneralLedger error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel buku besar.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportProcurementPayments mengunduh daftar pembayaran pengadaan sebagai file
// Excel (.xlsx). Filter (status, tipe, projek, kata kunci) identik dengan
// halaman Pembayaran — kalau ada satu filter yang tidak ikut, isi file tidak
// sama dengan yang dilihat pengguna di layar. Scope outlet/unit kerja per role
// dipaksakan di service.
func ExportProcurementPayments(c *fiber.Ctx) error {
	data, filename, err := services.BuildProcurementPaymentsExcel(
		c.Query("status"), c.Query("type"), c.Query("project_id"), strings.TrimSpace(c.Query("search")),
		getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		log.Printf("ExportProcurementPayments error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel pembayaran pengadaan.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportCashierShiftReport mengunduh laporan shift kasir sebagai file Excel
// (.xlsx). Filter (outlet, status, rentang tanggal — boleh kosong) identik
// dengan GetCashierShiftReport, termasuk pembatasan scope outlet per role.
func ExportCashierShiftReport(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	data, filename, err := services.BuildCashierShiftReportExcel(
		outletID, c.Query("status"), c.Query("date_from"), c.Query("date_to"), getOutletScope(c))
	if err != nil {
		log.Printf("ExportCashierShiftReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan shift kasir.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}

// ExportProductSalesReport mengunduh laporan penjualan per produk sebagai file
// Excel (.xlsx). Filter (tanggal, outlet, urutan) identik dengan
// GetProductSalesReport, termasuk pembatasan scope outlet berdasarkan role.
func ExportProductSalesReport(c *fiber.Ctx) error {
	dateFrom, dateTo, err := getDateRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	scopeIDs := getOutletScope(c)
	sortBy := c.Query("sort", "revenue") // revenue | qty
	sortDir := c.Query("dir", "desc")    // desc | asc

	data, filename, err := services.BuildProductSalesReportExcel(dateFrom, dateTo, outletID, sortBy, sortDir, scopeIDs)
	if err != nil {
		log.Printf("ExportProductSalesReport error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel laporan penjualan produk.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}
