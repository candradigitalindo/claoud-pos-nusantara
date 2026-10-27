package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ppicUsageError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	switch {
	case errors.Is(err, services.ErrPpicUsageAccess):
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Error: err.Error()})
	case errors.Is(err, services.ErrPpicUsageNotFound):
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Error: err.Error()})
	case errors.Is(err, services.ErrPpicUsageConflict):
		return c.Status(fiber.StatusConflict).JSON(models.APIResponse{Error: err.Error()})
	case errors.As(err, &invalid):
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Error: err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Error: "Gagal memproses kontrol pemakaian bahan"})
	}
}

func ppicUsageFilters(c *fiber.Ctx) (string, string, string, string, string, error) {
	from, to, err := ppicReportRange(c)
	if err != nil {
		return "", "", "", "", "", services.Invalid("%s", err)
	}
	status := strings.TrimSpace(c.Query("status"))
	switch status {
	case "", "unset", "no_basis", "over_limit", "needs_count", "within_limit", "no_activity", "data_incomplete":
	default:
		return "", "", "", "", "", services.Invalid("status kontrol pemakaian tidak dikenal")
	}
	return from, to, strings.TrimSpace(c.Query("warehouse_id")), strings.TrimSpace(c.Query("search")), status, nil
}

func GetPpicUsageControl(c *fiber.Ctx) error {
	from, to, warehouse, search, status, err := ppicUsageFilters(c)
	if err != nil {
		return ppicUsageError(c, err)
	}
	page, limit := getPagination(c)
	report, err := services.GetPpicUsageControl(from, to, warehouse, search, status, getOutletScope(c), page, limit)
	if err != nil {
		return ppicUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: report})
}

func GetPpicUsageLookups(c *fiber.Ctx) error {
	lookups, err := services.GetPpicUsageLookups(strings.TrimSpace(c.Query("warehouse_id")), getOutletScope(c))
	if err != nil {
		return ppicUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: lookups})
}

func SavePpicUsagePolicy(c *fiber.Ctx) error {
	var input models.PpicUsagePolicyInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Error: "body tidak valid"})
	}
	actor, _ := c.Locals("admin_username").(string)
	policy, err := services.SavePpicUsagePolicy(input, getOutletScope(c), actor)
	if err != nil {
		return ppicUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: policy, Message: "batas pemakaian dan waste tersimpan"})
}

func GetPpicUsagePolicyHistory(c *fiber.Ctx) error {
	history, err := services.ListPpicUsagePolicyHistory(strings.TrimSpace(c.Query("item_id")), strings.TrimSpace(c.Query("warehouse_id")), getOutletScope(c))
	if err != nil {
		return ppicUsageError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: history})
}

func ExportPpicUsageControl(c *fiber.Ctx) error {
	from, to, warehouse, search, status, err := ppicUsageFilters(c)
	if err != nil {
		return ppicUsageError(c, err)
	}
	data, err := services.ExportPpicUsageControl(from, to, warehouse, search, status, getOutletScope(c))
	if err != nil {
		return ppicUsageError(c, err)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="Kontrol-Pemakaian-PPIC-%s-%s.xlsx"`, from, to))
	return c.Send(data)
}
