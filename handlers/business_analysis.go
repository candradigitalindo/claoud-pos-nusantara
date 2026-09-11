package handlers

import (
	"fmt"
	"log"
	"strconv"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// GetBusinessAnalysis melayani halaman Laporan → Analisa Bisnis.
//
// Sengaja TIDAK menerima filter outlet maupun scope: RGI adalah ukuran relatif
// terhadap populasi, jadi menyaring sebagian outlet menghasilkan angka yang
// salah, bukan angka yang lebih sempit. Karena itu halaman ini dibatasi ke
// peran yang boleh melihat seluruh grup.
func GetBusinessAnalysis(c *fiber.Ctx) error {
	weeks, err := strconv.Atoi(c.Query("weeks", "12"))
	if err != nil || weeks <= 0 {
		weeks = 12
	}

	report, err := services.GetBusinessAnalysis(weeks)
	if err != nil {
		log.Printf("GetBusinessAnalysis error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat analisa bisnis.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: report})
}

// ExportBusinessAnalysisExcel mengunduh Analisa Bisnis sebagai file Excel
// lengkap dengan grafik native (bukan gambar), memakai rentang minggu yang
// sama dengan tampilan halaman.
func ExportBusinessAnalysisExcel(c *fiber.Ctx) error {
	weeks, err := strconv.Atoi(c.Query("weeks", "12"))
	if err != nil || weeks <= 0 {
		weeks = 12
	}

	data, filename, err := services.BuildBusinessAnalysisExcel(weeks)
	if err != nil {
		log.Printf("ExportBusinessAnalysisExcel error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat file Excel analisa bisnis.",
		})
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(data)
}
