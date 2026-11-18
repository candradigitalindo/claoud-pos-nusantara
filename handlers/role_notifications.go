package handlers

import (
	"encoding/json"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Role managers can configure internal subscriptions without reading gateway,
// customer, delivery-window, or message-template settings.
type roleNotificationSettings struct {
	Enabled           bool                                       `json:"enabled"`
	Events            map[string]bool                            `json:"events"`
	EventRoles        map[string][]string                        `json:"event_roles"`
	RoleNotifications map[string]models.WARoleNotificationPolicy `json:"role_notifications"`
}

type roleNotificationEvent struct {
	Key              string   `json:"key"`
	Label            string   `json:"label"`
	Group            string   `json:"group"`
	Audience         string   `json:"audience"`
	Description      string   `json:"description"`
	Enabled          bool     `json:"enabled"`
	Scheduled        bool     `json:"scheduled"`
	Permissions      []string `json:"permissions"`
	Roles            []string `json:"roles"`
	RouteDesc        string   `json:"route_desc"`
	ScopeKind        string   `json:"scope_kind"`
	ScopeDescription string   `json:"scope_description"`
	IsDigest         bool     `json:"is_digest"`
}

func roleNotificationData(settings models.WASettings, roles []models.WARoleOption) fiber.Map {
	catalog := services.WAEventCatalog(settings)
	allowed := map[string]bool{}
	events := make([]roleNotificationEvent, 0, len(catalog))
	clean := roleNotificationSettings{
		Enabled: settings.Enabled, Events: map[string]bool{}, EventRoles: map[string][]string{},
		RoleNotifications: map[string]models.WARoleNotificationPolicy{},
	}
	for _, event := range catalog {
		if event.Audience != "internal" {
			continue
		}
		allowed[event.Key] = true
		clean.Events[event.Key] = event.Enabled
		if len(settings.EventRoles[event.Key]) > 0 {
			clean.EventRoles[event.Key] = append([]string{}, settings.EventRoles[event.Key]...)
		}
		events = append(events, roleNotificationEvent{
			Key: event.Key, Label: event.Label, Group: event.Group, Audience: event.Audience,
			Description: event.Description, Enabled: event.Enabled, Scheduled: event.Scheduled,
			Permissions: event.Permissions, Roles: event.Roles, RouteDesc: event.RouteDesc,
			ScopeKind: event.ScopeKind, ScopeDescription: event.ScopeDescription, IsDigest: event.IsDigest,
		})
	}
	cleanPolicy := func(policy models.WARoleNotificationPolicy) models.WARoleNotificationPolicy {
		filtered := map[string]string{}
		for key, mode := range policy.Events {
			if allowed[key] && (mode == "auto" || mode == "on" || mode == "off") {
				filtered[key] = mode
			}
		}
		policy.Events = filtered
		return policy
	}
	for role, policy := range settings.RoleNotifications {
		clean.RoleNotifications[role] = cleanPolicy(policy)
	}
	safeRoles := make([]models.WARoleOption, len(roles))
	copy(safeRoles, roles)
	for i := range safeRoles {
		safeRoles[i].Policy = cleanPolicy(safeRoles[i].Policy)
	}
	return fiber.Map{"settings": clean, "roles": safeRoles, "events": events}
}

func roleNotificationResponse(c *fiber.Ctx, message string) error {
	return c.JSON(models.APIResponse{
		Success: true, Message: message,
		Data: roleNotificationData(services.GetWASettings(), services.WARoleOptions()),
	})
}

func GetRoleNotificationOptions(c *fiber.Ctx) error {
	return roleNotificationResponse(c, "")
}

func UpdateRoleNotifications(c *fiber.Ctx) error {
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
	return roleNotificationResponse(c, "Notifikasi role disimpan")
}

func ResetRoleNotifications(c *fiber.Ctx) error {
	if err := services.UpdateWARoleNotification(c.Params("role"), nil); err != nil {
		return waErr(c, err, "Gagal mengembalikan notifikasi role")
	}
	return roleNotificationResponse(c, "Notifikasi role dikembalikan ke aturan otomatis")
}
