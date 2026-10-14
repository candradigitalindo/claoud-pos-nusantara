package handlers

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// storeUpload menyimpan berkas ke uploads/YYYY-MM/<ulid><ext> dan
// mengembalikan URL publiknya. Pemeriksaan ukuran/jenis dilakukan pemanggil.
//
// Nama berkas memakai services.NewULID (crypto/rand), bukan ulid.Make()
// (math/rand berbenih waktu): /uploads disajikan tanpa login, jadi nama yang
// tidak bisa ditebak adalah satu-satunya pelindung bukti (T-69, T-70).
func storeUpload(c *fiber.Ctx, file *multipart.FileHeader, ext string) (string, error) {
	datePath := time.Now().Format("2006-01")
	dir := filepath.Join(services.UploadsDir, datePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("gagal menyiapkan folder upload")
	}
	filename := fmt.Sprintf("%s%s", services.NewULID(), ext)
	if err := c.SaveFile(file, filepath.Join(dir, filename)); err != nil {
		return "", fmt.Errorf("gagal menyimpan file")
	}
	return fmt.Sprintf("/uploads/%s/%s", datePath, filename), nil
}

func UploadFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "File tidak ditemukan."})
	}

	// Max 5MB
	if file.Size > 5*1024*1024 {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Ukuran file maksimal 5MB."})
	}

	// Ekstensi yang diizinkan — daftar yang sama dengan validator URL bukti
	// (services.ValidateUploadURL), supaya berkas yang lolos unggah tidak
	// ditolak saat URL-nya dikirim sebagai bukti.
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !services.UploadExtAllowed(ext) {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format file tidak didukung. Gunakan JPG, PNG, PDF, atau WebP."})
	}
	if !sniffContentAllowed(file, "image/", "application/pdf") {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Isi file tidak sesuai formatnya."})
	}

	url, err := storeUpload(c, file, ext)
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{
		Success: true,
		Data: fiber.Map{
			"url":      url,
			"filename": file.Filename,
		},
	})
}
