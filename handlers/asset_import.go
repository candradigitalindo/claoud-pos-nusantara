package handlers

import (
	"fmt"
	"io"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// DownloadAssetImportTemplate — template Excel berisi contoh, panduan, dan
// daftar referensi yang diambil dari data nyata (kategori & outlet yang benar),
// supaya staf tidak mengisi sesuatu yang nanti ditolak.
func DownloadAssetImportTemplate(c *fiber.Ctx) error {
	buf, filename, err := services.BuildAssetImportTemplate(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal menyiapkan template: " + err.Error()})
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

// ImportAssets memeriksa (mode=preview) atau menyimpan (mode=commit) isi berkas.
//
// Pemeriksaan dan penyimpanan dipisah dengan sengaja: berkas berisi ratusan
// baris tidak boleh langsung masuk, karena satu kolom salah ketik akan
// melahirkan ratusan aset keliru yang harus dibersihkan satu per satu.
func ImportAssets(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Berkas tidak ditemukan"})
	}
	if file.Size > 5*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Ukuran berkas maksimal 5MB"})
	}
	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Berkas tidak bisa dibuka"})
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Berkas tidak bisa dibaca"})
	}

	commit := c.Query("mode") == "commit"
	res, err := services.ImportAssetsFromExcel(content, commit, assetActor(c), getOutletScope(c))
	if err != nil {
		return invalidOr(c, err, "Gagal memproses berkas: ")
	}
	return c.JSON(models.APIResponse{Success: true, Data: res, Message: res.Message})
}
