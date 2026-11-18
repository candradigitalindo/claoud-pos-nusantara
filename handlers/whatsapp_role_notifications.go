package handlers

import (
	"encoding/json"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// WAUpdateRoleNotifications changes one role's subscription without overwriting
// another role's preferences or global sending/template settings.
func WAUpdateRoleNotifications(c *fiber.Ctx) error {
	var policy models.WARoleNotificationPolicy
	body := strings.TrimSpace(string(c.Body()))
	if body == "" || body == "null" || !strings.HasPrefix(body, "{") {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Isi pengaturan notifikasi role tidak valid"})
	}
	if err := json.Unmarshal(c.Body(), &policy); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format pengaturan notifikasi role tidak valid"})
	}
	if err := services.UpdateWARoleNotification(c.Params("role"), &policy); err != nil {
		return waErr(c, err, "Gagal menyimpan notifikasi role")
	}
	return waRoleNotificationResponse(c, "Notifikasi role disimpan")
}

func WAResetRoleNotifications(c *fiber.Ctx) error {
	if err := services.UpdateWARoleNotification(c.Params("role"), nil); err != nil {
		return waErr(c, err, "Gagal mengembalikan notifikasi role")
	}
	return waRoleNotificationResponse(c, "Notifikasi role dikembalikan ke aturan otomatis")
}

func waRoleNotificationResponse(c *fiber.Ctx, message string) error {
	return c.JSON(models.APIResponse{Success: true, Message: message, Data: fiber.Map{
		"settings": services.GetWASettings(), "roles": services.WARoleOptions(),
	}})
}
