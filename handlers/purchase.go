package handlers

import (
	"errors"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ListPurchaseRequests(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	workUnitID := c.Query("work_unit_id", "")
	status := c.Query("status", "")
	requestType := c.Query("type", "")
	projectID := c.Query("project_id", "")
	parentID := c.Query("parent_id", "")
	excludeMasters := c.Query("exclude_masters", "") == "true"
	search := strings.TrimSpace(c.Query("search", ""))
	page, limit := getPagination(c)
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)

	result, err := services.ListPurchaseRequests(outletID, workUnitID, status, requestType, projectID, parentID, excludeMasters, search, scopeIDs, wuScopeIDs, page, limit)
	if err != nil {
		log.Printf("ListPurchaseRequests error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat data pengajuan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func CreatePurchaseRequest(c *fiber.Ctx) error {
	var input models.CreatePurchaseRequestInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if input.RequestedBy == "" || len(input.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "requested_by dan items wajib diisi.",
		})
	}

	if input.RequestType != "barang" && input.RequestType != "jasa" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "request_type harus 'barang' atau 'jasa'.",
		})
	}

	for _, item := range input.Items {
		if item.Name == "" || len(item.Items) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Setiap pengadaan harus memiliki nama dan minimal 1 item.",
			})
		}
		for _, sub := range item.Items {
			if sub.Name == "" || sub.Qty <= 0 || sub.HpsPrice < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
					Success: false, Error: "Setiap item harus memiliki nama, qty > 0, dan harga HPS >= 0.",
				})
			}
		}
	}

	// Validate scope BEFORE creating
	if input.OutletID != "" && !validateOutletAccess(c, input.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet tidak dalam scope Anda"})
	}
	if input.WorkUnitID != "" && !validateWorkUnitAccess(c, input.WorkUnitID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Unit kerja tidak dalam scope Anda"})
	}

	result, err := services.CreatePurchaseRequest(input)
	if err != nil {
		// Kesalahan isi permintaan ditampilkan apa adanya — pengaju perlu tahu
		// apa yang salah. Kegagalan sistem tetap disembunyikan.
		var ve services.ValidationError
		if errors.As(err, &ve) {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: ve.Error()})
		}
		log.Printf("CreatePurchaseRequest error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat pengajuan.",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: result})
}

func GetPurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	result, err := services.GetPurchaseRequest(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat detail: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func SplitPurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.SplitPurchaseRequestInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if input.VendorName == "" || len(input.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "vendor_name dan items wajib diisi.",
		})
	}

	// Check permission for purchasing action
	roleName, _ := c.Locals("admin_role").(string)
	if roleName != "superadmin" {
		perms, err := services.GetRolePermissions(roleName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memuat permission.",
			})
		}
		hasPermission := false
		for _, p := range perms {
			if p == "procurement.requests.purchasing" {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false, Error: "Anda tidak memiliki akses untuk melakukan split vendor.",
			})
		}
	}

	result, err := services.SplitPurchaseRequest(id, input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func UpdatePurchaseStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.UpdatePurchaseStatusInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if input.Action == "" || input.ActorName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "action dan actor_name wajib diisi.",
		})
	}

	// Per-action permission check
	var requiredPerm string
	switch strings.ToLower(input.Action) {
	case "approve", "reject":
		requiredPerm = "procurement.requests.approve"
	case "request_payment":
		requiredPerm = "procurement.requests.purchasing"
	case "pay":
		// Mencairkan uang butuh izin sendiri. Sebelumnya cukup
		// finance.payments.view — izin membaca halaman Pembayaran — sehingga
		// siapa pun yang boleh melihat daftar tagihan juga boleh membayarnya.
		requiredPerm = "finance.payments.pay"
	case "receive", "cancel":
		requiredPerm = "procurement.requests.submit"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Action tidak valid.",
		})
	}

	// Bypass superadmin disamakan dengan handler lain di file ini; tanpa itu
	// izin per-aksi bergantung pada berhasilnya seeding role_permissions.
	roleName, _ := c.Locals("admin_role").(string)
	if roleName != "superadmin" {
		perms, err := services.GetRolePermissions(roleName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memuat permission.",
			})
		}
		hasPermission := false
		for _, p := range perms {
			if p == requiredPerm {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false, Error: "Anda tidak memiliki akses untuk aksi ini.",
			})
		}
	}

	result, err := services.UpdatePurchaseStatus(id, input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func UpdatePurchaseItems(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.UpdatePurchaseItemsInput
	if err := c.BodyParser(&input); err != nil {
		log.Printf("[UpdatePurchaseItems] BodyParser error for id=%s: %v", id, err)
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if len(input.Items) == 0 {
		log.Printf("[UpdatePurchaseItems] Empty items for id=%s, body=%s", id, string(c.Body()))
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Items tidak boleh kosong.",
		})
	}

	// Check current status to determine required permission
	var currentStatus string
	if err := database.DB.QueryRow("SELECT status FROM purchase_requests WHERE id = $1", id).Scan(&currentStatus); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pengajuan tidak ditemukan.",
		})
	}

	var requiredPerm string
	switch currentStatus {
	case "pending":
		requiredPerm = "procurement.requests.submit" // pengaju edits qty
	case "approved":
		requiredPerm = "procurement.requests.purchasing" // purchasing fills prices
	default:
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Tidak bisa edit item pada status ini.",
		})
	}

	roleName, _ := c.Locals("admin_role").(string)
	if roleName != "superadmin" {
		perms, err := services.GetRolePermissions(roleName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memuat permission.",
			})
		}
		hasPermission := false
		for _, p := range perms {
			if p == requiredPerm {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false, Error: "Anda tidak memiliki akses untuk aksi ini.",
			})
		}
	}

	result, err := services.UpdatePurchaseItems(id, input)
	if err != nil {
		log.Printf("[UpdatePurchaseItems] ERROR for id=%s: %v", id, err)
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func DeletePurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	roleName, _ := c.Locals("admin_role").(string)
	isAdmin := roleName == "superadmin" || roleName == "admin"

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	if err := services.DeletePurchaseRequest(id, isAdmin); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Message: "Pengajuan berhasil dihapus."})
}

func GetProcurementDashboard(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if outletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Outlet ID wajib diisi.",
		})
	}
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	result, err := services.GetProcurementDashboard(outletID, nil, nil)
	if err != nil {
		log.Printf("GetProcurementDashboard error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard pengadaan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetProcurementDashboardGlobal(c *fiber.Ctx) error {
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)
	result, err := services.GetProcurementDashboard("", scopeIDs, wuScopeIDs)
	if err != nil {
		log.Printf("GetProcurementDashboardGlobal error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard pengadaan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetPaymentStats(c *fiber.Ctx) error {
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)
	result, err := services.GetPaymentStats(scopeIDs, wuScopeIDs)
	if err != nil {
		log.Printf("GetPaymentStats error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat statistik pembayaran.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetPaymentHistories(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	histories, err := services.GetPaymentHistories(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori pembayaran.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: histories})
}
