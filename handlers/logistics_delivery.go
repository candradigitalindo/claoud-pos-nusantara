package handlers

import (
	"math"
	"strconv"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func isOutletScopedRequest(c *fiber.Ctx) bool {
	scope, _ := c.Locals("admin_scope").(string)
	return scope == "specific"
}

// logisticsViewGroups mengembalikan daftar kelompok Purchasing ('bahan_baku' / 'barang_jasa')
// yang boleh dilihat pengguna di modul Logistik. nil = kedua kelompok.
// Role ber-scope outlet spesifik tidak otomatis mendapat kelompok Bahan Baku hanya dari
// stockledger.adjust (izin itu dipakai untuk penyesuaian stok outlet, bukan bahan baku pusat).
func logisticsViewGroups(acc purchaseAccess, outletScoped bool) []string {
	if acc.superadmin || acc.has("logistics.deliveries.view") || acc.has("logistics.deliveries.manage") {
		return nil
	}
	groups := make([]string, 0, 2)
	if acc.has("logistics.rawmaterials.view") || acc.has("logistics.rawmaterials.manage") ||
		acc.has("procurement.rawmaterials.purchasing") ||
		(!outletScoped && acc.has("stockledger.adjust")) {
		groups = append(groups, services.PurchasingGroupBahanBaku)
	}
	if acc.has("logistics.goods.view") || acc.has("logistics.goods.manage") ||
		acc.has("assets.create") || acc.has("procurement.requests.purchasing") {
		groups = append(groups, services.PurchasingGroupBarangJasa)
	}
	if len(groups) == 2 {
		return nil
	}
	return groups
}

// logisticsManageGroups mengembalikan daftar kelompok Purchasing yang boleh dibuat/diubah/diberangkatkan pengguna.
func logisticsManageGroups(acc purchaseAccess) []string {
	if acc.superadmin || acc.has("logistics.deliveries.manage") {
		return nil
	}
	groups := make([]string, 0, 2)
	if acc.has("logistics.rawmaterials.manage") {
		groups = append(groups, services.PurchasingGroupBahanBaku)
	}
	if acc.has("logistics.goods.manage") {
		groups = append(groups, services.PurchasingGroupBarangJasa)
	}
	if len(groups) == 2 {
		return nil
	}
	return groups
}

func logisticsGroupsToTypes(groups []string) []string {
	if groups == nil {
		return nil
	}
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		switch g {
		case services.PurchasingGroupBahanBaku:
			out = append(out, services.RequestTypeBahan)
		case services.PurchasingGroupBarangJasa:
			out = append(out, services.RequestTypeBarang)
		}
	}
	return out
}

func groupInList(group string, allowed []string) bool {
	if allowed == nil {
		return true
	}
	for _, g := range allowed {
		if g == group {
			return true
		}
	}
	return false
}

func ListDispatchablePRItems(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	allowedTypes := logisticsGroupsToTypes(logisticsViewGroups(acc, isOutletScopedRequest(c)))
	reqType := strings.TrimSpace(c.Query("request_type"))
	if reqType == "" {
		reqType = strings.TrimSpace(c.Query("purchasing_group"))
	}
	list, err := services.ListDispatchablePRItems(
		getOutletScope(c),
		allowedTypes,
		c.Query("pr_id"),
		reqType,
		c.Query("line_kind"),
		c.Query("q"),
	)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func ListLogisticsDeliveries(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	purchasingGroup := strings.TrimSpace(c.Query("purchasing_group"))
	if purchasingGroup != "" && !services.IsValidPurchasingGroup(purchasingGroup) {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "purchasing_group harus 'bahan_baku' atau 'barang_jasa'."})
	}
	list, total, err := services.ListLogisticsDeliveriesFiltered(
		c.Query("status"),
		c.Query("dispatch_mode"),
		purchasingGroup,
		c.Query("date_from"),
		c.Query("date_to"),
		c.Query("pr_id"),
		c.Query("q"),
		getOutletScope(c),
		logisticsViewGroups(acc, isOutletScopedRequest(c)),
		page, limit,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(fiber.Map{
		"success":     true,
		"data":        list,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func GetLogisticsDelivery(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	id := c.Params("id")
	if err := services.CheckLogisticsDeliveryGroupAccess(id, logisticsViewGroups(acc, isOutletScopedRequest(c))); err != nil {
		return purchaseServiceError(c, err)
	}
	d, err := services.GetLogisticsDelivery(id, getOutletScope(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func CreateLogisticsDelivery(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	manageTypes := logisticsGroupsToTypes(logisticsManageGroups(acc))
	if manageTypes != nil && len(manageTypes) == 0 {
		return purchaseForbidden(c, "Anda tidak memiliki izin mengelola pengantaran logistik.")
	}
	var in models.CreateLogisticsDeliveryInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	d, err := services.CreateLogisticsDelivery(in, purchaseActor(c), getOutletScope(c), manageTypes)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: d})
}

func UpdateLogisticsDelivery(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	id := c.Params("id")
	manageGroups := logisticsManageGroups(acc)
	if err := services.CheckLogisticsDeliveryGroupAccess(id, manageGroups); err != nil {
		return purchaseServiceError(c, err)
	}
	var in models.UpdateLogisticsDeliveryInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	d, err := services.UpdateLogisticsDelivery(id, in, purchaseActor(c), getOutletScope(c), logisticsGroupsToTypes(manageGroups))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func UpdateLogisticsDeliveryStatus(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	id := c.Params("id")
	if err := services.CheckLogisticsDeliveryGroupAccess(id, logisticsManageGroups(acc)); err != nil {
		return purchaseServiceError(c, err)
	}
	var in models.UpdateLogisticsStatusInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	d, err := services.UpdateLogisticsDeliveryStatus(id, in, purchaseActor(c), getOutletScope(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func UpdateLogisticsDeliveryPhotos(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	id := c.Params("id")
	grp, err := services.GetLogisticsDeliveryGroup(id)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	canEditPickup := groupInList(grp, logisticsManageGroups(acc))
	canEditReceive := canEditPickup ||
		(grp == services.PurchasingGroupBahanBaku && (acc.has("stockledger.adjust") || acc.has("procurement.rawmaterials.submit"))) ||
		(grp == services.PurchasingGroupBarangJasa && (acc.has("assets.create") || acc.has("procurement.requests.submit")))
	var in models.UpdateLogisticsPhotosInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	d, err := services.UpdateLogisticsDeliveryPhotos(id, in, canEditPickup, canEditReceive, getOutletScope(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func ReceiveLogisticsDelivery(c *fiber.Ctx) error {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	id := c.Params("id")
	grp, err := services.GetLogisticsDeliveryGroup(id)
	if err != nil {
		return purchaseServiceError(c, err)
	}
	ra := services.ReceiveAccess{
		Submit: (grp == services.PurchasingGroupBahanBaku && acc.has("procurement.rawmaterials.submit")) ||
			(grp == services.PurchasingGroupBarangJasa && acc.has("procurement.requests.submit")),
		Asset: acc.has("assets.create"),
		Stock: acc.has("stockledger.adjust"),
	}
	if !ra.Any() {
		return purchaseForbidden(c, "Penerimaan final Surat Jalan dilakukan oleh Gudang Induk (izin penyesuaian stok) atau Bagian Aset (izin tambah aset).")
	}
	var req models.ReceiveLogisticsDeliveryInput
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	res, err := services.ReceiveLogisticsDelivery(id, req, purchaseActor(c), ra, getOutletScope(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: res, Message: res.Message})
}
