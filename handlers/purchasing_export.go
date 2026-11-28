package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ExportPurchasing uses the request list's view permission and location scopes,
// including its per-type separation between equipment/services and materials.
func ExportPurchasing(c *fiber.Ctx) error {
	outletID := strings.TrimSpace(c.Query("outlet_id"))
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return purchaseForbidden(c, "Akses outlet tidak diizinkan")
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	requestType := strings.TrimSpace(c.Query("type"))
	var allowedTypes []string
	if requestType != "" {
		if requestType == services.PurchasingGroupBarangJasa {
			if !acc.can(services.RequestTypeBarang, "view") {
				return purchaseForbidden(c, "Anda tidak memiliki akses melihat pengadaan Barang & Jasa.")
			}
		} else {
			if !services.IsValidRequestType(requestType) {
				return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "type harus 'barang', 'jasa', 'bahan', atau 'barang_jasa'."})
			}
			if !acc.can(requestType, "view") {
				return purchaseForbidden(c, "Anda tidak memiliki akses melihat pengadaan "+services.RequestTypeLabel(requestType)+".")
			}
		}
	} else {
		allowedTypes = acc.viewableTypes()
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && status != services.PurchaseStatusCommittedFilter && !services.IsValidPurchaseStatus(status) {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Status pengadaan tidak valid."})
	}
	data, filename, err := services.BuildPurchasingExcel(outletID, strings.TrimSpace(c.Query("work_unit_id")), status,
		requestType, strings.TrimSpace(c.Query("project_id")), strings.TrimSpace(c.Query("parent_id")), strings.TrimSpace(c.Query("search")),
		getOutletScope(c), getWorkUnitScope(c), allowedTypes)
	if err != nil {
		var invalid services.ValidationError
		if errors.As(err, &invalid) {
			return purchaseServiceError(c, err)
		}
		log.Printf("ExportPurchasing error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal membuat file Excel purchasing per vendor."})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}
