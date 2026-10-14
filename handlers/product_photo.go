package handlers

import (
	"os"
	"path/filepath"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

const productPhotoDir = "uploads/products"

var productPhotoExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

// UploadProductPhoto stores a product image and saves its URL on the product so
// prospective reservation customers can see the menu photos.
func UploadProductPhoto(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validateRowOutletAccess(c, "cloud_products", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "File tidak ada"})
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !productPhotoExt[ext] {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format foto harus JPG, PNG, WEBP, atau GIF"})
	}
	if file.Size > 10*1024*1024 {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Ukuran foto maksimal 10MB"})
	}
	if !sniffContentAllowed(file, "image/") {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Isi file bukan gambar"})
	}

	if err := os.MkdirAll(productPhotoDir, 0755); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan folder"})
	}
	name := id + "-" + services.NewULID() + ext
	if err := c.SaveFile(file, filepath.Join(productPhotoDir, name)); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyimpan foto"})
	}
	url := "/uploads/products/" + name

	res, err := database.DB.Exec(`UPDATE cloud_products SET photo_url = $1, updated_at = NOW() WHERE id = $2 AND is_deleted = false`, url, id)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui produk"})
	}
	if n, _ := res.RowsAffected(); n == 0 {
		os.Remove(filepath.Join(productPhotoDir, name))
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Produk tidak ditemukan"})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"photo_url": url}})
}

// DeleteProductPhoto clears a product's photo.
func DeleteProductPhoto(c *fiber.Ctx) error {
	id := c.Params("id")
	// Cakupan outlet sama dengan UploadProductPhoto: tanpa cek ini pengguna
	// ber-scope outlet bisa menghapus foto produk outlet lain.
	if !validateRowOutletAccess(c, "cloud_products", id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	var cur string
	database.DB.QueryRow(`SELECT COALESCE(photo_url,'') FROM cloud_products WHERE id=$1`, id).Scan(&cur)
	if _, err := database.DB.Exec(`UPDATE cloud_products SET photo_url='', updated_at=NOW() WHERE id=$1`, id); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus foto"})
	}
	// Berkas di disk hanya dihapus bila URL-nya terkurung di folder uploads
	// (T-2); nilai lain cukup dikosongkan dari produk.
	if cur != "" {
		if p, err := services.UploadFilePath(cur); err == nil {
			os.Remove(p)
		}
	}
	return c.JSON(models.APIResponse{Success: true})
}
