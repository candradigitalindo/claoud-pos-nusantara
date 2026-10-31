package handlers

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

var productPhotoDir = "uploads/products"

var productPhotoExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

// ServeProductPhoto opens each requested image afresh. Fiber's static handler
// caches open file descriptors, which can otherwise serve a removed photo for
// several seconds after replacement or deletion.
func ServeProductPhoto(c *fiber.Ctx) error {
	name := c.Params("name")
	ext := strings.ToLower(filepath.Ext(name))
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\%`) || !productPhotoExt[ext] {
		return c.SendStatus(fiber.StatusNotFound)
	}
	path := filepath.Join(productPhotoDir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return c.SendStatus(fiber.StatusNotFound)
	}
	file, err := os.Open(path)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Type(strings.TrimPrefix(ext, "."))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	// SendStream transfers ownership of the file to fasthttp, which closes it
	// after writing the response (including HEAD requests).
	return c.SendStream(file, int(info.Size()))
}

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
	tx, err := database.DB.Begin()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui produk"})
	}
	defer tx.Rollback()
	var previous, outletID string
	err = tx.QueryRow(`SELECT COALESCE(photo_url,''),outlet_id FROM cloud_products
		WHERE id=$1 AND is_deleted=false FOR UPDATE`, id).Scan(&previous, &outletID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Produk tidak ditemukan"})
	}
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui produk"})
	}
	if !validateOutletAccess(c, strings.TrimSpace(outletID)) {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}

	if err := os.MkdirAll(productPhotoDir, 0755); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan folder"})
	}
	name := id + "-" + services.NewULID() + ext
	path := filepath.Join(productPhotoDir, name)
	if err := c.SaveFile(file, path); err != nil {
		os.Remove(path)
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyimpan foto"})
	}
	url := "/uploads/products/" + name

	_, err = tx.Exec(`UPDATE cloud_products SET photo_url = $1, updated_at = NOW() WHERE id = $2`, url, id)
	if err != nil {
		os.Remove(path)
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui produk"})
	}
	if err := tx.Commit(); err != nil {
		// A disconnected COMMIT has an unknown outcome. Keep the new file so
		// a successfully committed URL cannot become a broken image.
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memperbarui produk"})
	}
	removeProductPhoto(previous, id)
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
	tx, err := database.DB.Begin()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus foto"})
	}
	defer tx.Rollback()
	var cur, outletID string
	err = tx.QueryRow(`SELECT COALESCE(photo_url,''),outlet_id FROM cloud_products
		WHERE id=$1 AND is_deleted=false FOR UPDATE`, id).Scan(&cur, &outletID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: "Produk tidak ditemukan"})
	}
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus foto"})
	}
	if !validateOutletAccess(c, strings.TrimSpace(outletID)) {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: "Outlet di luar akses Anda"})
	}
	if _, err := tx.Exec(`UPDATE cloud_products SET photo_url='', updated_at=NOW() WHERE id=$1`, id); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus foto"})
	}
	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus foto"})
	}
	removeProductPhoto(cur, id)
	return c.JSON(models.APIResponse{Success: true})
}

// Only files generated for this product can be removed. Never follow a stored
// URL into another upload directory or remove another product's photo.
func removeProductPhoto(raw, productID string) {
	const prefix = "/uploads/products/"
	if !strings.HasPrefix(raw, prefix) {
		return
	}
	name := strings.TrimPrefix(raw, prefix)
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) || !strings.HasPrefix(name, productID+"-") {
		return
	}
	os.Remove(filepath.Join(productPhotoDir, name))
}
