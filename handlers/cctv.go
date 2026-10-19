package handlers

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"

	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Monitoring & kelola kamera (admin, scoped) ──────────────────────────────

// GetCameras — daftar kamera semua outlet dalam scope (opsional ?outlet_id=).
func GetCameras(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	cams, err := services.ListCameras(outletID, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat kamera"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cams})
}

// GetOutletCameras — daftar kamera satu outlet.
func GetOutletCameras(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	cams, err := services.ListCamerasByOutlet(outletID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat kamera"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: cams})
}

// CreateCamera — tambah kamera pada outlet (cameras.create).
func CreateCamera(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	var req models.CreateCameraRequest
	if len(c.Body()) > 48<<10 || c.BodyParser(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
	}
	if req.SourceTicket != "" {
		admin, _ := c.Locals("admin_id").(string)
		source, err := services.ResolveCCTVCloudTicket(req.SourceTicket, admin, outletID)
		if err != nil {
			return cctvError(c, err)
		}
		req.SourceURL = &source
	}
	cam, err := services.CreateCamera(outletID, req)
	if err != nil {
		return cctvError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: cam})
}

// UpdateCamera — ubah konfigurasi kamera (cameras.update).
func UpdateCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	var req models.UpdateCameraRequest
	if len(c.Body()) > 48<<10 || c.BodyParser(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
	}
	if req.SourceTicket != "" {
		outlet, err := services.GetCameraOutletID(id)
		if err != nil {
			return cctvError(c, err)
		}
		admin, _ := c.Locals("admin_id").(string)
		source, err := services.ResolveCCTVCloudTicket(req.SourceTicket, admin, outlet)
		if err != nil {
			return cctvError(c, err)
		}
		req.SourceURL = &source
	}
	cam, err := services.UpdateCamera(id, req)
	if err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: cam, Message: "Kamera berhasil diperbarui"})
}

// ToggleCamera — aktif/nonaktif (cameras.update).
func ToggleCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	cam, err := services.ToggleCamera(id)
	if err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: cam})
}

// DeleteCamera — hapus kamera (cameras.delete).
func DeleteCamera(c *fiber.Ctx) error {
	id := c.Params("camId")
	if !cameraInScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
	}
	if err := services.DeleteCamera(id); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Kamera berhasil dihapus"})
}

// ── Streaming (live + playback) ─────────────────────────────────────────────

// StartCameraStream — mulai live view. Mendaftarkan stream ke go2rtc, menerbitkan
// cookie stream token (untuk relay /cctv/), dan mengembalikan URL HLS.
func StartCameraStream(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("camId")
		if !cameraInScope(c, id) {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
		}
		if err := setCCTVCookie(c); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Streaming CCTV memerlukan sesi login admin yang masih berlaku"})
		}
		resp, err := services.StartLive(id)
		if err != nil {
			return cctvError(c, err)
		}
		return c.JSON(models.APIResponse{Success: true, Data: resp})
	}
}

// StartCameraPlayback — mulai putar ulang rekaman DVR untuk rentang waktu (transient).
func StartCameraPlayback(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("camId")
		if !cameraInScope(c, id) {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses kamera tidak diizinkan"})
		}
		var req models.PlaybackRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format permintaan tidak valid"})
		}
		if _, err := services.BuildPlaybackRTSP(models.OutletCamera{}, "", req.Start, req.End); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
		}
		if err := setCCTVCookie(c); err != nil {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Streaming CCTV memerlukan sesi login admin yang masih berlaku"})
		}
		resp, err := services.StartPlayback(id, req)
		if err != nil {
			return cctvError(c, err)
		}
		return c.JSON(models.APIResponse{Success: true, Data: resp})
	}
}

// StopCameraStream — hentikan & bersihkan stream playback transient (?name=).
func StopCameraStream(c *fiber.Ctx) error {
	id := c.Params("camId")
	// Periksa kecocokan nama sebelum melakukan panggilan ke relay.
	streamCamera, err := services.CCTVStreamCameraID(c.Query("name"))
	if err != nil || streamCamera != id {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	if !cameraInScope(c, id) {
		return cctvError(c, services.ErrCCTVAccessDenied)
	}
	if err := services.StopStream(id, c.Query("name")); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}

// CCTVAuthz — subrequest nginx auth_request untuk melindungi relay /cctv/.
// Divalidasi via cookie stream token (browser mengirimnya otomatis pada tiap
// request media/segmen). Registrasi di luar grup AdminAuth karena request media
// tidak membawa header Authorization.
func CCTVAuthz(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u, err := url.ParseRequestURI(c.Get("X-Original-URI"))
		if err != nil || !strings.HasPrefix(u.Path, "/cctv/") {
			return cctvError(c, services.ErrCCTVAccessDenied)
		}
		if _, err := services.CCTVMediaSource(c.Cookies("cctv_token"), strings.TrimPrefix(u.Path, "/cctv"), u.RawQuery); err != nil {
			return cctvError(c, err)
		}
		return c.SendStatus(fiber.StatusOK)
	}
}

// CCTVRelay melindungi seluruh playlist/segmen; tidak ada proxy umum ke API
// go2rtc. Kredensial dan body galat upstream tidak pernah diteruskan.
func CCTVRelay(c *fiber.Ctx) error {
	body, contentType, err := services.RelayCCTVMedia(c.Cookies("cctv_token"), strings.TrimPrefix(c.Path(), "/cctv"), string(c.Context().QueryArgs().QueryString()))
	if err != nil {
		return cctvError(c, err)
	}
	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(body)
}

func RefreshCCTVSession(c *fiber.Ctx) error {
	if err := setCCTVCookie(c); err != nil {
		return cctvError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true})
}

func ClearCCTVSession(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{Name: "cctv_token", Path: "/cctv", Expires: time.Unix(1, 0), MaxAge: -1, HTTPOnly: true, Secure: c.Protocol() == "https", SameSite: "Lax"})
	return c.JSON(models.APIResponse{Success: true})
}

func cctvError(c *fiber.Ctx, err error) error {
	status, message := fiber.StatusBadGateway, "Stream belum tersedia; periksa koneksi kamera dan layanan relay"
	switch {
	case errors.Is(err, services.ErrAuthUnavailable):
		status, message = fiber.StatusServiceUnavailable, services.ErrAuthUnavailable.Error()
	case errors.Is(err, services.ErrAuthTokenInvalid), errors.Is(err, services.ErrAuthAccountInactive), errors.Is(err, services.ErrAuthSessionRevoked):
		status, message = fiber.StatusUnauthorized, "Sesi CCTV sudah tidak berlaku; buka ulang kamera"
	case errors.Is(err, services.ErrCCTVAccessDenied):
		status, message = fiber.StatusForbidden, services.ErrCCTVAccessDenied.Error()
	case errors.Is(err, services.ErrCameraInactive):
		status, message = fiber.StatusForbidden, services.ErrCameraInactive.Error()
	case errors.Is(err, services.ErrCameraValidation):
		status, message = fiber.StatusBadRequest, err.Error()
	case errors.Is(err, services.ErrPlaybackUnsupported):
		status, message = fiber.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, services.ErrCCTVGatewayOffline):
		status, message = fiber.StatusFailedDependency, err.Error()
	case errors.Is(err, services.ErrCCTVStreamLimit):
		status, message = fiber.StatusTooManyRequests, err.Error()
	case errors.Is(err, sql.ErrNoRows):
		status, message = fiber.StatusNotFound, "Kamera tidak ditemukan"
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(status).JSON(models.APIResponse{Success: false, Error: message})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// cameraInScope memuat outlet pemilik kamera lalu memvalidasi scope admin.
func cameraInScope(c *fiber.Ctx, camID string) bool {
	outletID, err := services.GetCameraOutletID(camID)
	if err == sql.ErrNoRows {
		return true // biarkan service mengembalikan "tidak ditemukan"
	}
	if err != nil {
		return false
	}
	return validateOutletAccess(c, outletID)
}

// setCCTVCookie menyetel cookie stream token (Path=/cctv, HttpOnly) berumur pendek.
func setCCTVCookie(c *fiber.Ctx) error {
	tok, err := services.IssueCCTVToken(strings.TrimPrefix(c.Get("Authorization"), "Bearer "))
	if err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     "cctv_token",
		Value:    tok,
		Path:     "/cctv",
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: "Lax",
		MaxAge:   3600,
	})
	return nil
}
