package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Endpoint data terbuka /api/v1/open — tanpa auth, outlet dirujuk lewat slug.

// openMaxRangeDays membatasi rentang tanggal supaya satu permintaan publik
// tidak bisa memaksa database memindai bertahun-tahun data.
const openMaxRangeDays = 366

func openJSON(c *fiber.Ctx, data interface{}) error {
	c.Set("Cache-Control", "public, max-age=60")
	return c.JSON(models.APIResponse{Success: true, Data: data})
}

func openError(c *fiber.Ctx, err error, what string) error {
	if errors.Is(err, services.ErrOpenOutletNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: "Outlet tidak ditemukan"})
	}
	log.Printf("open data %s: %v", what, err)
	return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memuat data."})
}

// openDateRange membaca date_from/date_to (YYYY-MM-DD, zona waktu aplikasi).
// Bila kosong: date_to = hari ini, date_from = date_to − (defaultDays−1).
func openDateRange(c *fiber.Ctx, defaultDays int) (string, string, error) {
	loc := services.GetTimezoneLocation()
	dateTo := c.Query("date_to", time.Now().In(loc).Format("2006-01-02"))
	to, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		return "", "", fmt.Errorf("format tanggal tidak valid, gunakan YYYY-MM-DD")
	}
	dateFrom := c.Query("date_from", to.AddDate(0, 0, -(defaultDays-1)).Format("2006-01-02"))
	from, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		return "", "", fmt.Errorf("format tanggal tidak valid, gunakan YYYY-MM-DD")
	}
	if from.After(to) {
		return "", "", fmt.Errorf("date_from tidak boleh lebih besar dari date_to")
	}
	if int(to.Sub(from).Hours()/24)+1 > openMaxRangeDays {
		return "", "", fmt.Errorf("rentang tanggal maksimal %d hari", openMaxRangeDays)
	}
	return dateFrom, dateTo, nil
}

func badRange(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
}

// GET /api/v1/open/outlets
func OpenListOutlets(c *fiber.Ctx) error {
	list, err := services.ListOpenOutlets()
	if err != nil {
		return openError(c, err, "outlets")
	}
	return openJSON(c, list)
}

// GET /api/v1/open/outlets/:slug/products
func OpenOutletProducts(c *fiber.Ctx) error {
	res, err := services.GetOpenProducts(c.Params("slug"))
	if err != nil {
		return openError(c, err, "products")
	}
	return openJSON(c, res)
}

// GET /api/v1/open/outlets/:slug/product-sales?date_from=&date_to= (bawaan: hari ini)
func OpenOutletProductSales(c *fiber.Ctx) error {
	from, to, err := openDateRange(c, 1)
	if err != nil {
		return badRange(c, err)
	}
	res, err := services.GetOpenProductSales(c.Params("slug"), from, to)
	if err != nil {
		return openError(c, err, "product-sales")
	}
	return openJSON(c, res)
}

// GET /api/v1/open/social?date_from=&date_to= (bawaan: 30 hari terakhir)
func OpenSocial(c *fiber.Ctx) error {
	from, to, err := openDateRange(c, 30)
	if err != nil {
		return badRange(c, err)
	}
	res, err := services.GetOpenSocial(from, to)
	if err != nil {
		return openError(c, err, "social")
	}
	return openJSON(c, res)
}
