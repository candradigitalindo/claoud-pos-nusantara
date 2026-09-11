package handlers

import (
	"log"
	"strconv"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ── Kinerja Markom ───────────────────────────────────────────────────────────
//
// Halaman ini memasok angka Instagram/TikTok yang dipakai Analisa Bisnis. Yang
// dilihat (social.view) dipisah dari yang mengubah (social.manage): mendaftarkan
// akun, mengetik tambalan mingguan, dan menarik paksa dari halaman publik adalah
// pekerjaan Markom — pembaca laporan tidak perlu bisa menyentuhnya.

func ListSocialAccounts(c *fiber.Ctx) error {
	accounts, err := services.ListSocialAccounts()
	if err != nil {
		log.Printf("ListSocialAccounts error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat daftar akun medsos.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: accounts})
}

func CreateSocialAccount(c *fiber.Ctx) error {
	var req models.SocialAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Data yang dikirim tidak bisa dibaca.",
		})
	}
	acc, err := services.CreateSocialAccount(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: acc})
}

func UpdateSocialAccount(c *fiber.Ctx) error {
	var req models.SocialAccountRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Data yang dikirim tidak bisa dibaca.",
		})
	}
	acc, err := services.UpdateSocialAccount(c.Params("id"), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: acc})
}

func DeleteSocialAccount(c *fiber.Ctx) error {
	if err := services.DeleteSocialAccount(c.Params("id")); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": true}})
}

// ScrapeSocialAccounts menarik angka sekarang juga, tanpa menunggu jadwal.
//
// Berjalan sinkron meski bisa memakan menit: jeda antar-akun disengaja supaya
// alamat IP server tidak diblokir, dan orang yang menekan tombolnya justru
// perlu melihat akun mana yang gagal. Menjalankannya di latar akan mengubah
// kegagalan jadi senyap — persis keadaan yang seluruh modul ini hindari.
func ScrapeSocialAccounts(c *fiber.Ctx) error {
	var req struct {
		IDs []string `json:"ids"`
	}
	// Badan kosong = tarik semua akun aktif; bukan galat.
	_ = c.BodyParser(&req)

	res, err := services.ScrapeSocialAccounts(req.IDs, false)
	if err != nil {
		// Putaran yang bentrok bukan kerusakan server: orang menekan tombolnya
		// dua kali, atau menekannya saat penjadwal sedang bekerja. Jawaban 409
		// membuat halaman bisa membedakannya dari galat sungguhan.
		if strings.Contains(err.Error(), "sedang berjalan") {
			return c.Status(fiber.StatusConflict).JSON(models.APIResponse{
				Success: false, Error: err.Error(),
			})
		}
		log.Printf("ScrapeSocialAccounts error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal menjalankan penarikan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: res})
}

func GetSocialWeekly(c *fiber.Ctx) error {
	weeks, err := strconv.Atoi(c.Query("weeks", "12"))
	if err != nil || weeks <= 0 {
		weeks = 12
	}
	rows, err := services.GetSocialWeekly(weeks)
	if err != nil {
		log.Printf("GetSocialWeekly error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat ringkasan mingguan medsos.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: rows})
}

func ListSocialManualWeeks(c *fiber.Ctx) error {
	list, err := services.ListSocialManualWeeks(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat isian manual.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func UpsertSocialManualWeek(c *fiber.Ctx) error {
	var req models.SocialManualWeek
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Data yang dikirim tidak bisa dibaca.",
		})
	}
	req.AccountID = c.Params("id")

	actor, _ := c.Locals("admin_username").(string)
	if err := services.UpsertSocialManualWeek(req, actor); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"saved": true}})
}
