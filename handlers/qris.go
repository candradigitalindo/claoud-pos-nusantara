package handlers

import (
	"os"
	"strings"

	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

type createQRISChargeRequest struct {
	OrderLocalID string  `json:"order_local_id"`
	Amount       float64 `json:"amount"`
	Description  string  `json:"description"`
}

// CreateQRISCharge — POST /outlets/:outletId/qris/charges
// Dipanggil POS saat kasir memilih metode QRIS.
func CreateQRISCharge(c *fiber.Ctx) error {
	var req createQRISChargeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid",
		})
	}
	if req.Amount <= 0 {
		return c.Status(400).JSON(models.APIResponse{
			Success: false, Error: "Nominal tagihan harus lebih dari 0",
		})
	}

	charge, err := services.CreateQRISCharge(
		c.Params("outletId"), req.OrderLocalID, req.Amount, req.Description,
	)
	if err != nil {
		return c.Status(502).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: charge})
}

// GetQRISCharge — GET /outlets/:outletId/qris/charges/:chargeId
// Di-polling POS sambil menunggu tamu membayar.
func GetQRISCharge(c *fiber.Ctx) error {
	charge, err := services.GetQRISCharge(c.Params("outletId"), c.Params("chargeId"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: charge})
}

// SimulateQRISPaid — POST /outlets/:outletId/qris/charges/:chargeId/simulate-paid
//
// Alat uji: menandai tagihan lunas tanpa penyedia asli. HANYA tersedia saat
// penyedia aktif adalah "mock", supaya tidak mungkin ada jalur di produksi yang
// bisa melunasi tagihan tanpa uang benar-benar masuk.
func SimulateQRISPaid(c *fiber.Ctx) error {
	if !services.MockSimulationEnabled() {
		return c.Status(403).JSON(models.APIResponse{
			Success: false,
			Error:   "Simulasi hanya tersedia saat penyedia pembayaran mock aktif",
		})
	}
	if err := services.SimulateQRISChargePaid(c.Params("outletId"), c.Params("chargeId")); err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	charge, err := services.GetQRISCharge(c.Params("outletId"), c.Params("chargeId"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: charge})
}

// QRISWebhook — POST /webhooks/qris/:provider
//
// Callback penyedia. Tidak memakai AuthOutlet: keasliannya dibuktikan tanda
// tangan yang diverifikasi adapter penyedia yang bersangkutan, bukan API key
// outlet. Verifikasi memakai adapter SESUAI :provider (bukan penyedia aktif)
// agar tagihan lama dari penyedia sebelumnya tetap bisa diselesaikan.
func QRISWebhook(c *fiber.Ctx) error {
	gw, err := services.GatewayByName(c.Params("provider"))
	if err != nil {
		return c.Status(404).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	if gw.Name() == "mock" && !services.MockSimulationEnabled() {
		return c.Status(403).JSON(models.APIResponse{Success: false, Error: "Simulasi QRIS tidak aktif"})
	}

	// Header dinormalkan ke huruf kecil agar adapter tidak perlu menebak
	// kapitalisasi yang dipakai masing-masing penyedia.
	headers := map[string]string{}
	c.Request().Header.VisitAll(func(k, v []byte) {
		headers[strings.ToLower(string(k))] = string(v)
	})

	chargeID, status, err := gw.VerifyWebhook(c.Body(), headers)
	if err != nil {
		// 401 tanpa detail: jangan beri tahu pengirim tak dikenal bagian mana
		// dari verifikasi yang gagal.
		return c.Status(401).JSON(models.APIResponse{
			Success: false, Error: "Callback ditolak",
		})
	}

	if err := services.MarkQRISChargePaidForProvider(gw.Name(), chargeID, status); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"received": true}})
}

// GetPaymentGatewayInfo — GET /outlets/:outletId/qris/info
// POS memakai ini untuk tahu apakah QRIS terintegrasi tersedia; bila penyedia
// masih mock, kasir diberi tahu bahwa pembayaran perlu dikonfirmasi manual.
func GetPaymentGatewayInfo(c *fiber.Ctx) error {
	gw, err := services.ActiveGateway()
	if err != nil {
		return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
			"enabled": false, "provider": "",
		}})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"enabled":       true,
		"provider":      gw.Name(),
		"is_mock":       gw.Name() == "mock",
		"webhook_ready": os.Getenv("QRIS_WEBHOOK_SECRET") != "",
	}})
}
