package middleware

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

func AuthOutlet() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
				Success: false,
				Error:   "Missing Authorization header",
			})
		}

		apiKey := strings.TrimPrefix(auth, "Bearer ")
		if apiKey == auth {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
				Success: false,
				Error:   "Invalid authorization format, use Bearer {api_key}",
			})
		}

		var outletID, outletCode, outletName string
		var isActive bool
		err := database.DB.QueryRow(
			"SELECT id, code, name, is_active FROM outlets WHERE api_key = $1",
			apiKey,
		).Scan(&outletID, &outletCode, &outletName, &isActive)

		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
				Success: false,
				Error:   "Invalid API key",
			})
		}

		// Trim whitespace from CHAR(26) columns
		outletID = strings.TrimSpace(outletID)
		outletCode = strings.TrimSpace(outletCode)
		outletName = strings.TrimSpace(outletName)

		if !isActive {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false,
				Error:   "Outlet is deactivated",
			})
		}

		// Validate URL param :outletId matches authenticated outlet (if present)
		if paramID := c.Params("outletId"); paramID != "" && paramID != outletID {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false,
				Error:   "API key does not match the requested outlet",
			})
		}

		c.Locals("outlet_id", outletID)
		c.Locals("outlet_code", outletCode)
		c.Locals("outlet_name", outletName)

		return c.Next()
	}
}

func AdminAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")

		if token == "" || token == auth {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
				Success: false,
				Error:   "Unauthorized: missing or invalid Authorization header",
			})
		}

		// 1) Token admin statis (kompatibilitas lama) — hanya bila ADMIN_TOKEN
		//    diisi nilai kuat; kosong/lemah = jalur ini mati (T-1).
		if cfg.StaticAdminTokenMatches(token) {
			c.Locals("admin_id", "static")
			c.Locals("admin_username", "admin")
			c.Locals("admin_role", "superadmin")
			c.Locals("admin_scope", "all")
			return c.Next()
		}

		// 2) JWT: tanda tangan & kedaluwarsa, lalu dicocokkan dengan keadaan
		//    akun di DB — masih ada, aktif, versi token & role sama — dan scope
		//    dibaca dari role di DB, bukan dari token (T-55, T-56). Cache 30 dtk.
		sess, err := services.AuthenticateAdminJWT(token, cfg.JWTSecret)
		if err != nil {
			return RejectAdminSession(c, err)
		}

		c.Locals("admin_id", sess.ID)
		c.Locals("admin_username", sess.Username)
		c.Locals("admin_role", sess.Role)
		c.Locals("admin_scope", sess.ScopeType)
		if sess.ScopeType == "specific" {
			c.Locals("admin_outlet_ids", sess.OutletIDs)
			c.Locals("admin_work_unit_ids", sess.WorkUnitIDs)
		}

		return c.Next()
	}
}

// RejectAdminSession menulis respons penolakan sesi admin; dipakai AdminAuth
// dan stream SSE agar aturannya sama. Sesi yang tidak bisa diperiksa (galat
// DB) = 503: tetap ditolak, tetapi UI tidak membuang sesi karena galat
// sementara. Token tidak sah, dicabut, atau akun nonaktif = 401 — UI
// membersihkan sesi dan kembali ke halaman login.
func RejectAdminSession(c *fiber.Ctx, err error) error {
	status, msg := fiber.StatusUnauthorized, "Unauthorized: token tidak valid atau sudah expired"
	switch {
	case errors.Is(err, services.ErrAuthUnavailable):
		status, msg = fiber.StatusServiceUnavailable, "Sesi tidak dapat diperiksa saat ini, coba lagi sebentar lagi"
	case errors.Is(err, services.ErrAuthSessionRevoked):
		msg = "Unauthorized: sesi sudah tidak berlaku, silakan login ulang"
	case errors.Is(err, services.ErrAuthAccountInactive):
		msg = "Unauthorized: akun tidak aktif"
	}
	return c.Status(status).JSON(models.APIResponse{Success: false, Error: msg})
}

// rateBucket menghitung request per klien dalam satu jendela waktu (fixed window).
type rateBucket struct {
	count       int
	windowStart time.Time
}

type rateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

// allow mengembalikan false bila key sudah melebihi max request dalam jendela window.
func (s *rateLimiterStore) allow(key string, max int, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	b, ok := s.buckets[key]
	if !ok || now.Sub(b.windowStart) >= window {
		// Sapu bucket kedaluwarsa saat map membesar agar memori tidak tumbuh tanpa batas.
		if len(s.buckets) > 10000 {
			for k, v := range s.buckets {
				if now.Sub(v.windowStart) >= window {
					delete(s.buckets, k)
				}
			}
		}
		s.buckets[key] = &rateBucket{count: 1, windowStart: now}
		return true
	}
	b.count++
	return b.count <= max
}

func newRateLimitHandler(name string, max int, window time.Duration) fiber.Handler {
	store := &rateLimiterStore{buckets: map[string]*rateBucket{}}
	return func(c *fiber.Ctx) error {
		if !store.allow(name+"|"+c.IP(), max, window) {
			return c.Status(fiber.StatusTooManyRequests).JSON(models.APIResponse{
				Success: false, Error: "Terlalu banyak permintaan, coba lagi sebentar lagi.",
			})
		}
		return c.Next()
	}
}

// RateLimiter membatasi request per IP untuk grup outlet API (sync device rutin,
// jadi batasnya longgar — pagar terhadap loop liar, bukan throttle lalu lintas normal).
func RateLimiter(cfg *config.Config) fiber.Handler {
	return newRateLimitHandler("outlet", 600, time.Minute)
}

// LoginRateLimiter membatasi percobaan login per IP (anti brute-force password).
func LoginRateLimiter() fiber.Handler {
	return newRateLimitHandler("login", 10, time.Minute)
}

// PublicRateLimiter membatasi endpoint publik tanpa auth (booking reservasi dll).
func PublicRateLimiter() fiber.Handler {
	return newRateLimitHandler("public", 30, time.Minute)
}

// RequirePermission checks if the authenticated admin's role has a specific permission.
// superadmin (static token) always passes.
// For ".view" permissions, having ".manage" of the same module also passes (manage implies view).
func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("admin_role").(string)

		// superadmin bypasses all permission checks
		if role == "superadmin" {
			return c.Next()
		}

		perms, err := services.GetRolePermissions(role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memeriksa permission",
			})
		}

		for _, p := range perms {
			if p == permission {
				return c.Next()
			}
			// Any sub-permission of a module implies .view access
			if strings.HasSuffix(permission, ".view") {
				module := strings.TrimSuffix(permission, ".view")
				if strings.HasPrefix(p, module+".") {
					return c.Next()
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false,
			Error:   "Akses ditolak: Anda tidak memiliki izin untuk fitur ini",
		})
	}
}

// RequireAnyPermission passes if the role holds ANY of the given permissions
// (same matching rules as RequirePermission). Used for resources reachable from
// multiple features — e.g. the vendor list is needed both by vendor management
// (vendors.view) and by procurement requesters (procurement.requests.*).
func RequireAnyPermission(permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("admin_role").(string)
		if role == "superadmin" {
			return c.Next()
		}

		perms, err := services.GetRolePermissions(role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memeriksa permission",
			})
		}

		for _, required := range permissions {
			for _, p := range perms {
				if p == required {
					return c.Next()
				}
				if strings.HasSuffix(required, ".view") {
					module := strings.TrimSuffix(required, ".view")
					if strings.HasPrefix(p, module+".") {
						return c.Next()
					}
				}
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
			Success: false,
			Error:   "Akses ditolak: Anda tidak memiliki izin untuk fitur ini",
		})
	}
}

// RequireSuperadmin allows only the superadmin role (master account).
func RequireSuperadmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("admin_role").(string)
		if role != "superadmin" {
			return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{
				Success: false, Error: "Akses ditolak: khusus superadmin",
			})
		}
		return c.Next()
	}
}
