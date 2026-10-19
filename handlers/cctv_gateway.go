package handlers

import (
	"context"
	"strings"
	"time"

	"cloud-pos/internal/cctvrelay"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func GetCCTVOptions(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"brands": services.CCTVBrands, "connections": services.CCTVConnections, "tuya_regions": services.TuyaRegions}})
}

func GetCCTVGateways(c *fiber.Ctx) error {
	gateways, err := services.ListCCTVGateways(getOutletScope(c))
	if err != nil {
		return cctvError(c, services.ErrAuthUnavailable)
	}
	return c.JSON(models.APIResponse{Success: true, Data: gateways})
}

func DeleteCCTVGateway(c *fiber.Ctx) error {
	outlet := c.Params("id")
	if !validateOutletAccess(c, outlet) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	if err := services.DeleteCCTVGateway(outlet); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Akses penghubung outlet dicabut"})
}

func ProbeCameraConfiguration(c *fiber.Ctx) error {
	outlet := c.Params("id")
	if !validateOutletAccess(c, outlet) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	var req models.CameraProbeRequest
	if len(c.Body()) > 48<<10 || c.BodyParser(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Konfigurasi kamera tidak valid"})
	}
	if req.SourceTicket != "" {
		admin, _ := c.Locals("admin_id").(string)
		source, err := services.ResolveCCTVCloudTicket(req.SourceTicket, admin, outlet)
		if err != nil {
			return cctvError(c, err)
		}
		req.SourceURL = &source
	}
	if err := services.ProbeCameraConfiguration(outlet, req); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"connected": true}, Message: "Koneksi video berhasil; konfigurasi belum disimpan"})
}

func DiscoverCameraONVIFProfiles(c *fiber.Ctx) error {
	outlet := c.Params("id")
	if !validateOutletAccess(c, outlet) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	var req models.CameraProbeRequest
	if len(c.Body()) > 48<<10 || c.BodyParser(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Konfigurasi ONVIF tidak valid"})
	}
	admin, _ := c.Locals("admin_id").(string)
	if req.SourceTicket != "" {
		source, err := services.ResolveCCTVCloudTicket(req.SourceTicket, admin, outlet)
		if err != nil {
			return cctvError(c, err)
		}
		req.SourceURL = &source
	}
	profiles, err := services.DiscoverCameraONVIFProfiles(admin, outlet, req)
	if err != nil {
		return cctvError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: profiles})
}

func CreateCCTVGateway(rotate bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		outlet := c.Params("id")
		if !validateOutletAccess(c, outlet) {
			return cctvError(c, services.ErrCCTVAccessDenied)
		}
		token, err := services.CreateCCTVGateway(outlet, rotate)
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(models.APIResponse{Success: false, Error: "Penghubung tidak dapat dibuat; periksa outlet atau gunakan penggantian token untuk penghubung yang sudah ada"})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"outlet_id": outlet, "token": token}})
	}
}

func CCTVGatewayAuth(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return cctvError(c, services.ErrAuthTokenInvalid)
	}
	key, err := services.AuthenticateCCTVGateway(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		return cctvError(c, err)
	}
	c.Locals("cctv_gateway_key", key)
	return c.Next()
}

func PollCCTVGateway(c *fiber.Ctx) error {
	key, _ := c.Locals("cctv_gateway_key").(string)
	ctx, cancel := context.WithTimeout(c.UserContext(), 16*time.Second)
	defer cancel()
	jobs := services.PollCCTVGateway(ctx, key)
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"jobs": jobs})
}

func CompleteCCTVGateway(c *fiber.Ctx) error {
	if len(c.Body()) > cctvrelay.MaxBody*2 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var response cctvrelay.Response
	if err := c.BodyParser(&response); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	key, _ := c.Locals("cctv_gateway_key").(string)
	if !services.CompleteCCTVGateway(key, response) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.JSON(fiber.Map{"success": true})
}

func DiscoverTuyaCameras(c *fiber.Ctx) error {
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	outlet := c.Params("id")
	if !validateOutletAccess(c, outlet) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	var req struct {
		Region   string `json:"region"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if c.BodyParser(&req) != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	admin, _ := c.Locals("admin_id").(string)
	devices, err := services.DiscoverTuyaCameras(admin, outlet, req.Region, req.Email, req.Password)
	if err != nil {
		return cctvError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: devices})
}

func TestCameraConnection(c *fiber.Ctx) error {
	if !cameraInScope(c, c.Params("camId")) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	if err := services.TestCameraConnection(c.Params("camId")); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"connected": true, "checked_at": time.Now().UTC()}, Message: "Kamera berhasil dihubungkan"})
}
