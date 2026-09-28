package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

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

	block, _ := strconv.Atoi(c.Query("block", "4"))
	report, err := services.GetBusinessAnalysisWith(weeks, block)
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

	block, _ := strconv.Atoi(c.Query("block", "4"))
	data, filename, err := services.BuildBusinessAnalysisExcel(weeks, block)
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

// ── Kalender bisnis ──────────────────────────────────────────────────────────
// Hari libur, cuti bersama, libur sekolah, dan kejadian lokal yang dipakai
// Analisa Bisnis untuk menskalakan minggu berlibur ke minggu biasa. Karena ia
// ikut menentukan vonis, pembaca laporan boleh melihatnya; mengubahnya butuh
// reports.business_analysis.manage.

func ListBusinessCalendar(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		today := time.Now().In(services.GetTimezoneLocation())
		from = today.AddDate(0, 0, -180).Format("2006-01-02")
		to = today.AddDate(0, 0, 180).Format("2006-01-02")
	}
	list, err := services.ListBusinessCalendar(from, to)
	if err != nil {
		log.Printf("ListBusinessCalendar error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat kalender bisnis.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func UpsertBusinessCalendar(c *fiber.Ctx) error {
	var req models.BizCalendarDay
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Data yang dikirim tidak bisa dibaca.",
		})
	}
	actor, _ := c.Locals("admin_username").(string)
	d, err := services.UpsertBusinessCalendar(req, actor)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

func DeleteBusinessCalendar(c *fiber.Ctx) error {
	if err := services.DeleteBusinessCalendar(c.Params("day"), c.Query("kind")); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": true}})
}
