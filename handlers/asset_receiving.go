package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// roleHasPermission memeriksa izin di DALAM handler — dipakai saat satu aksi
// boleh berjalan dengan kemampuan berbeda-beda, bukan ditolak sepenuhnya.
// Aturan pencocokannya sama dengan middleware.RequirePermission.
func roleHasPermission(c *fiber.Ctx, permission string) bool {
	role, _ := c.Locals("admin_role").(string)
	if role == "superadmin" {
		return true
	}
	perms, err := services.GetRolePermissions(role)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// GetReceivingDraft menyusun isi dialog Serah Terima: tiap sub-item pengajuan
// beserta usulan tujuannya dan sisa yang belum dicatat.
func GetReceivingDraft(c *fiber.Ctx) error {
	d, err := services.BuildReceivingDraft(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

// ReceiveGoods menjalankan penerimaan: aset + stok gudang + status pengajuan,
// dalam satu transaksi.
func ReceiveGoods(c *fiber.Ctx) error {
	var req models.ReceiveGoodsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	// Petugas pengadaan umumnya tidak berhak menambah stok. Barisnya tidak
	// dibuang, melainkan ditunda ke antrean gudang — barangnya memang sudah
	// datang secara fisik.
	canStock := roleHasPermission(c, "stockledger.adjust")
	res, err := services.ReceiveGoods(c.Params("id"), req, assetActor(c), canStock, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: res, Message: res.Message})
}

// ListIncompleteReceipts — laporan "Pengadaan diterima, barang belum lengkap".
func ListIncompleteReceipts(c *fiber.Ctx) error {
	// Bawaan: hanya perlengkapan — bahan dapur bukan urusan tim aset.
	// "?kind=semua" membuka saringan; nilai kosong TIDAK bisa dipakai untuk itu
	// karena Fiber mengembalikan nilai bawaan untuk query yang kosong.
	kind := c.Query("kind", "perlengkapan")
	if kind == "semua" {
		kind = ""
	}
	list, err := services.ListIncompleteReceipts(getOutletScope(c), kind)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat laporan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// ListReceivingQueue — antrean serah terima milik satu meja.
//
// Bagian Aset membuka antrean "perlengkapan", Gudang Induk membuka "dapur".
// Masing-masing hanya melihat pekerjaannya sendiri.
func ListReceivingQueue(c *fiber.Ctx) error {
	kind := c.Query("kind", "perlengkapan")
	if kind == "semua" {
		kind = ""
	}
	list, err := services.ListReceivingQueue(kind, getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat antrean penerimaan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
