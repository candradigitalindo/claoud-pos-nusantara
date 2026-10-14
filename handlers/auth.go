package handlers

import (
	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"math"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func AdminLogin(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.AdminLoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Invalid request body",
			})
		}

		if req.Username == "" || req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Username dan password wajib diisi",
			})
		}

		ip := clientIP(c)
		ua := c.Get("User-Agent")

		result, err := services.AdminLogin(req, cfg.JWTSecret)
		if err != nil {
			services.RecordLogin(req.Username, "", "failed", ip, ua)
			// Galat DB saat membaca akun/izin/scope: login ditolak (T-56) dengan
			// 503 & pesan umum — detail galat hanya di log server.
			if errors.Is(err, services.ErrAuthUnavailable) {
				return c.Status(fiber.StatusServiceUnavailable).JSON(models.APIResponse{
					Success: false, Error: "Login belum dapat diproses, coba lagi sebentar lagi",
				})
			}
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
				Success: false, Error: err.Error(),
			})
		}

		services.RecordLogin(result.Admin.Username, result.Admin.Role, "success", ip, ua)

		return c.JSON(models.APIResponse{
			Success: true,
			Data:    result,
			Message: "Login berhasil",
		})
	}
}

func AdminGetProducts(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	search := c.Query("search")
	page, limit := getPagination(c)
	scopeIDs := getOutletScope(c)

	products, total, err := services.GetAllProducts(outletID, search, scopeIDs, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil data produk",
		})
	}
	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"items":       products,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func AdminGetCategories(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	search := c.Query("search")
	page, limit := getPagination(c)
	scopeIDs := getOutletScope(c)

	cats, total, err := services.GetAllCategories(outletID, search, scopeIDs, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil data kategori",
		})
	}
	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"items":       cats,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

func AdminCreateProduct(c *fiber.Ctx) error {
	var req models.AdminCreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Invalid request body"})
	}
	if req.OutletID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "outlet_id dan name wajib diisi"})
	}
	if !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	p, err := services.AdminCreateProduct(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: p})
}

func AdminUpdateProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	var req models.AdminUpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "name wajib diisi"})
	}
	if !validateRowOutletAccess(c, "cloud_products", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	if err := services.AdminUpdateProduct(id, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}

func AdminDeleteProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validateRowOutletAccess(c, "cloud_products", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	if err := services.AdminDeleteProduct(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}

func AdminCreateCategory(c *fiber.Ctx) error {
	var req models.AdminCreateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Invalid request body"})
	}
	if req.OutletID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "outlet_id dan name wajib diisi"})
	}
	if !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	cat, err := services.AdminCreateCategory(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: cat})
}

func AdminUpdateCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	var req models.AdminUpdateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "name wajib diisi"})
	}
	if !validateRowOutletAccess(c, "cloud_categories", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	if err := services.AdminUpdateCategory(id, req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}

func AdminDeleteCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validateRowOutletAccess(c, "cloud_categories", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	if err := services.AdminDeleteCategory(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"id": id}})
}

func GetAdmins(c *fiber.Ctx) error {
	admins, err := services.GetAdmins()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil data admin",
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Data: admins,
	})
}

func CreateAdmin(c *fiber.Ctx) error {
	var req models.CreateAdminRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	admin, err := services.CreateAdmin(req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "username, password, dan name wajib diisi" ||
			err.Error() == "password minimal 6 karakter" {
			status = fiber.StatusBadRequest
		}
		if strings.Contains(err.Error(), "sudah digunakan") {
			status = fiber.StatusConflict
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{
		Success: true,
		Data:    admin,
		Message: "Admin berhasil dibuat",
	})
}

// guardTargetAdmin memblokir modifikasi akun superadmin oleh non-superadmin,
// mengembalikan response 403 bila ditolak (nil bila lolos).
func guardTargetAdmin(c *fiber.Ctx, adminID string) error {
	callerRole, _ := c.Locals("admin_role").(string)
	if err := services.GuardTargetAdmin(adminID, callerRole); err != nil {
		status := fiber.StatusForbidden
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return nil
}

func UpdateAdminRole(c *fiber.Ctx) error {
	adminID := c.Params("id")
	var req models.UpdateAdminRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}
	if resp := guardTargetAdmin(c, adminID); resp != nil {
		return resp
	}

	admin, err := services.UpdateAdminRole(adminID, req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "admin tidak ditemukan" {
			status = fiber.StatusNotFound
		}
		if strings.Contains(err.Error(), "tidak valid") {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true,
		Data:    admin,
		Message: "Role berhasil diubah",
	})
}

func UpdateAdmin(c *fiber.Ctx) error {
	adminID := c.Params("id")
	var req models.UpdateAdminRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}
	if resp := guardTargetAdmin(c, adminID); resp != nil {
		return resp
	}

	admin, err := services.UpdateAdmin(adminID, req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		if strings.Contains(err.Error(), "wajib") || strings.Contains(err.Error(), "tidak valid") {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Data: admin, Message: "Admin berhasil diperbarui",
	})
}

func ResetAdminPassword(c *fiber.Ctx) error {
	adminID := c.Params("id")
	var req models.ResetAdminPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}
	if resp := guardTargetAdmin(c, adminID); resp != nil {
		return resp
	}

	if err := services.ResetAdminPassword(adminID, req); err != nil {
		status := fiber.StatusBadRequest
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Message: "Password berhasil direset",
	})
}

func ToggleAdminActive(c *fiber.Ctx) error {
	adminID := c.Params("id")
	if resp := guardTargetAdmin(c, adminID); resp != nil {
		return resp
	}
	admin, err := services.ToggleAdminActive(adminID)
	if err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	msg := "User diaktifkan"
	if !admin.IsActive {
		msg = "User dinonaktifkan"
	}
	return c.JSON(models.APIResponse{
		Success: true, Data: admin, Message: msg,
	})
}

func DeleteAdmin(c *fiber.Ctx) error {
	adminID := c.Params("id")
	if resp := guardTargetAdmin(c, adminID); resp != nil {
		return resp
	}
	if err := services.DeleteAdmin(adminID); err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Message: "Admin berhasil dihapus",
	})
}

func GetAllRolePermissions(c *fiber.Ctx) error {
	perms, err := services.GetAllRolePermissions()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil data permissions",
		})
	}

	scopes, _ := services.GetAllRoleScopes()

	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"permissions":     perms,
			"all_permissions": services.AllPermissions,
			"scopes":          scopes,
		},
	})
}

func UpdateRolePermissions(c *fiber.Ctx) error {
	role := c.Params("role")
	var req models.UpdateRolePermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	if err := services.UpdateRolePermissions(role, req.Permissions); err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak valid") {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true,
		Message: "Permissions berhasil diperbarui",
	})
}

func GetMyPermissions(c *fiber.Ctx) error {
	role, _ := c.Locals("admin_role").(string)

	if role == "superadmin" {
		return c.JSON(models.APIResponse{
			Success: true,
			Data: fiber.Map{
				"permissions":      services.AllPermissions,
				"scope_type":       "all",
				"scope_outlet_ids": nil,
			},
		})
	}

	perms, err := services.GetRolePermissions(role)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil permissions",
		})
	}

	// Scope = hasil validasi sesi di middleware (dibaca dari DB, gagal-
	// tertutup) — tidak dihitung ulang, agar yang ditampilkan selalu sama
	// dengan yang ditegakkan. Selain "all" berarti specific.
	scopeType, _ := c.Locals("admin_scope").(string)
	var scopeOutletIDs []string
	if scopeType != "all" {
		scopeType = "specific"
		scopeOutletIDs, _ = c.Locals("admin_outlet_ids").([]string)
		if scopeOutletIDs == nil {
			scopeOutletIDs = []string{}
		}
	}

	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"permissions":      perms,
			"scope_type":       scopeType,
			"scope_outlet_ids": scopeOutletIDs,
		},
	})
}

func ListRoles(c *fiber.Ctx) error {
	roles, err := services.ListRoles()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengambil data role",
		})
	}

	perms, _ := services.GetAllRolePermissions()
	scopes, _ := services.GetAllRoleScopes()

	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"roles":           roles,
			"permissions":     perms,
			"all_permissions": services.AllPermissions,
			"scopes":          scopes,
		},
	})
}

func CreateRole(c *fiber.Ctx) error {
	var req models.CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	role, err := services.CreateRole(req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "wajib") || strings.Contains(err.Error(), "tidak valid") || strings.Contains(err.Error(), "tidak diizinkan") {
			status = fiber.StatusBadRequest
		}
		if strings.Contains(err.Error(), "sudah ada") {
			status = fiber.StatusConflict
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{
		Success: true, Data: role, Message: "Role berhasil dibuat",
	})
}

func DeleteRole(c *fiber.Ctx) error {
	name := c.Params("name")
	if err := services.DeleteRole(name); err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		if strings.Contains(err.Error(), "tidak dapat") || strings.Contains(err.Error(), "bawaan") {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Message: "Role berhasil dihapus",
	})
}

func UpdateRole(c *fiber.Ctx) error {
	name := c.Params("name")
	var req models.UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	role, err := services.UpdateRole(name, req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak ditemukan") {
			status = fiber.StatusNotFound
		}
		if strings.Contains(err.Error(), "wajib") || strings.Contains(err.Error(), "tidak dapat") || strings.Contains(err.Error(), "bawaan") {
			status = fiber.StatusBadRequest
		}
		if strings.Contains(err.Error(), "sudah ada") {
			status = fiber.StatusConflict
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Data: role, Message: "Role berhasil diperbarui",
	})
}

func UpdateRoleScope(c *fiber.Ctx) error {
	role := c.Params("role")
	var req models.UpdateRoleScopeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	if err := services.UpdateRoleScope(role, req); err != nil {
		status := fiber.StatusInternalServerError
		if strings.Contains(err.Error(), "tidak") {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Message: "Scope role berhasil diperbarui",
	})
}

func ChangePassword(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(string)
	if adminID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
			Success: false, Error: "Unauthorized",
		})
	}

	var req models.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Password lama dan baru wajib diisi",
		})
	}

	if len(req.NewPassword) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Password baru minimal 6 karakter",
		})
	}

	if err := services.ChangePassword(adminID, req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Message: "Password berhasil diubah",
	})
}

func UpdateProfile(c *fiber.Ctx) error {
	adminID, _ := c.Locals("admin_id").(string)
	if adminID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{
			Success: false, Error: "Unauthorized",
		})
	}

	var req models.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Invalid request body",
		})
	}

	admin, err := services.UpdateProfile(adminID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{
		Success: true, Data: admin, Message: "Profil berhasil diperbarui",
	})
}
