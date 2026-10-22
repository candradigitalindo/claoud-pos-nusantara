package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"
	"log"
	"mime"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func documentError(c *fiber.Ctx, err error) error {
	var invalid services.ValidationError
	code, message := 500, "Gagal memproses dokumen. Silakan coba lagi."
	switch {
	case errors.As(err, &invalid):
		code, message = 400, invalid.Error()
	case errors.Is(err, services.ErrDocumentNumberExists):
		code, message = 409, "Nomor dokumen sudah digunakan. Gunakan nomor lain."
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, os.ErrNotExist):
		code, message = 404, "Dokumen atau berkas tidak ditemukan."
	default:
		log.Printf("documents: %v", err)
	}
	return c.Status(code).JSON(models.APIResponse{Success: false, Error: message})
}

func ListDocuments(c *fiber.Ctx) error {
	page, limit := getPagination(c)
	if limit > 100 {
		limit = 100
	}
	docs, total, err := services.ListDocuments(c.Query("search"), page, limit)
	if err != nil {
		return documentError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"items": docs, "total": total, "page": page, "limit": limit}})
}

func UploadDocument(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return documentError(c, services.Invalid("Pilih berkas dokumen terlebih dahulu."))
	}
	actor, _ := c.Locals("admin_username").(string)
	doc, err := services.CreateDocument(c.FormValue("document_number"), c.FormValue("about"), file, actor)
	if err != nil {
		return documentError(c, err)
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: doc})
}

func DownloadDocument(c *fiber.Ctx) error {
	doc, file, err := services.OpenDocumentDownload(c.Params("id"))
	if err != nil {
		return documentError(c, err)
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": doc.FileName}))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", "no-store")
	c.Set("X-Download-Count", strconv.FormatInt(doc.DownloadCount, 10))
	return c.SendStream(file, int(doc.FileSize))
}
