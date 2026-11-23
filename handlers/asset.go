package handlers

import (
	"fmt"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// assetActor mengambil nama pengguna untuk dicatat di buku besar aset.
func assetActor(c *fiber.Ctx) string {
	actor, _ := c.Locals("admin_username").(string)
	return actor
}

// assetPageSize — layar Perlengkapan menampilkan 100 aset per halaman.
const assetPageSize = 100

// assetListFilter membaca saringan daftar Perlengkapan dari query string.
// Cakupan outlet selalu dari role pengguna, tidak pernah dari parameter.
func assetListFilter(c *fiber.Ctx) services.AssetListFilter {
	return services.AssetListFilter{
		OutletID:    c.Query("outlet_id"),
		Search:      c.Query("search"),
		Condition:   c.Query("condition"),
		Status:      c.Query("status"),
		OutletScope: getOutletScope(c),
	}
}

// assetListPage = PaginatedResponse + ringkasan seluruh hasil saringan.
type assetListPage struct {
	models.PaginatedResponse
	Summary services.AssetListSummary `json:"summary"`
}

// ListAssets: dengan ?page= mengembalikan satu halaman (bawaan 100 baris) plus
// ringkasan seluruh hasil saringan untuk kartu di atas daftar. Tanpa ?page=
// tetap mengembalikan daftar utuh seperti semula — pemilih aset di halaman
// Penghapusan Aset dan Detail Projek membutuhkan semua baris.
func ListAssets(c *fiber.Ctx) error {
	f := assetListFilter(c)
	if c.Query("page") == "" {
		assets, err := services.ListAssets(f)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
				Success: false, Error: "Gagal memuat aset: " + err.Error(),
			})
		}
		return c.JSON(models.APIResponse{Success: true, Data: assets})
	}

	limit := c.QueryInt("limit", assetPageSize)
	if limit < 1 || limit > 1000 {
		limit = assetPageSize
	}
	assets, summary, page, err := services.ListAssetsPage(f, c.QueryInt("page", 1), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat aset: " + err.Error(),
		})
	}
	totalPages := (summary.Count + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	return c.JSON(assetListPage{
		PaginatedResponse: models.PaginatedResponse{
			Success: true, Data: assets, Page: page, Limit: limit,
			Total: summary.Count, TotalPages: totalPages,
		},
		Summary: summary,
	})
}

// ExportAssets mengunduh daftar Perlengkapan sebagai Excel. Saringannya sama
// dengan layar, tapi TANPA paginasi: berkas selalu memuat semua aset yang
// lolos saringan, bukan hanya 100 baris di halaman yang sedang dibuka.
func ExportAssets(c *fiber.Ctx) error {
	buf, filename, err := services.ExportAssetList(assetListFilter(c), assetActor(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal menyiapkan Excel: " + err.Error(),
		})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func GetAsset(c *fiber.Ctx) error {
	a, err := services.GetAsset(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Aset tidak ditemukan",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func CreateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if !validateOutletAccess(c, req.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	a, err := services.CreateAsset(req, assetActor(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: a})
}

func UpdateAsset(c *fiber.Ctx) error {
	var req models.AssetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	a, err := services.UpdateAsset(c.Params("id"), req, assetActor(c), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: a})
}

func DeleteAsset(c *fiber.Ctx) error {
	if err := services.DeleteAsset(c.Params("id"), assetActor(c), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

func ListAssetMaintenances(c *fiber.Ctx) error {
	// Scope guard: ensure the asset is visible before exposing its history.
	if _, err := services.GetAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Aset tidak ditemukan"})
	}
	list, err := services.ListAssetMaintenances(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func DeleteAssetMaintenance(c *fiber.Ctx) error {
	if err := services.DeleteAssetMaintenance(c.Params("id"), c.Params("mid"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true})
}

// ListAssetMovements mengembalikan buku besar satu aset: pendataan, perubahan
// kondisi/lokasi, perawatan, dan penghapusan — berurut dari yang terbaru.
func ListAssetMovements(c *fiber.Ctx) error {
	// Scope guard: pastikan asetnya terlihat sebelum riwayatnya dibuka.
	if _, err := services.GetAsset(c.Params("id"), getOutletScope(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Aset tidak ditemukan"})
	}
	list, err := services.ListAssetMovements(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat riwayat: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
