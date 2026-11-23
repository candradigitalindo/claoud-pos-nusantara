package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ── Pembelian Darurat ──────────────────────────────────────────────────────
//
// Barang/jasa urgent ≤ batas (bawaan Rp500.000) dibeli dulu dari kas laci
// outlet lewat tugas Bayar Pengadaan di tablet kasir (PIN Manager/SVP), lalu
// disahkan pemegang procurement.requests.approve. Izin kasar dicek di rute;
// di sini cakupan unit kerja/outlet dan jenis dokumennya.

func emergencyServiceError(c *fiber.Ctx, err error, what string) error {
	var ve services.ValidationError
	var ce services.ConflictError
	switch {
	case errors.As(err, &ce):
		return c.Status(fiber.StatusConflict).JSON(models.APIResponse{Success: false, Error: ce.Error()})
	case errors.As(err, &ve):
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: ve.Error()})
	}
	log.Printf("%s error: %v", what, err)
	return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{Success: false, Error: "Gagal memproses Pembelian Darurat."})
}

// GetEmergencyContext: batas, pemakaian bulan berjalan, dan kesiapan tablet
// kasir untuk unit kerja outlet yang dipilih.
func GetEmergencyContext(c *fiber.Ctx) error {
	wu := strings.TrimSpace(c.Query("work_unit_id", ""))
	if wu == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Pilih unit kerja outlet."})
	}
	if !validateWorkUnitAccess(c, wu) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Unit kerja tidak dalam scope Anda"})
	}
	ctx, err := services.GetEmergencyContext(wu)
	if err != nil {
		return emergencyServiceError(c, err, "GetEmergencyContext")
	}
	return c.JSON(models.APIResponse{Success: true, Data: ctx})
}

// CreateEmergencyPurchase mencatat Pembelian Darurat dan mengirim tugas kas
// ke tablet kasir outlet.
func CreateEmergencyPurchase(c *fiber.Ctx) error {
	var in models.CreateEmergencyPurchaseInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	in.WorkUnitID = strings.TrimSpace(in.WorkUnitID)
	if in.WorkUnitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Pilih unit kerja outlet."})
	}
	if !validateWorkUnitAccess(c, in.WorkUnitID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Unit kerja tidak dalam scope Anda"})
	}
	// Pengaju = akun yang login (T-7).
	actor := purchaseActor(c)
	in.RequestedBy, in.RequestedByID = actor.Name, actor.ID
	pr, err := services.CreateEmergencyPurchase(in)
	if err != nil {
		return emergencyServiceError(c, err, "CreateEmergencyPurchase")
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: pr})
}

// emergencyDocOrFail: dokumen dalam cakupan pemanggil dan memang Pembelian
// Darurat. Bila tidak, respons sudah ditulis.
func emergencyDocOrFail(c *fiber.Ctx, id string) (bool, error) {
	if !validatePurchaseRequestScope(c, id) {
		return false, c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	if _, found, ferr := purchaseTypeOrFail(c, id); !found {
		return false, ferr
	}
	if !services.IsEmergencyPurchase(id) {
		return false, c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Pengajuan ini bukan Pembelian Darurat."})
	}
	return true, nil
}

func emergencyDecisionBody(c *fiber.Ctx) (models.EmergencyDecisionInput, bool, error) {
	var in models.EmergencyDecisionInput
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&in); err != nil {
			return in, false, c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
		}
	}
	actor := purchaseActor(c)
	in.ActorName, in.ActorID = actor.Name, actor.ID
	return in, true, nil
}

// SubmitEmergencyProof: pengaju mengunggah foto struk & total belanja.
func SubmitEmergencyProof(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := emergencyDocOrFail(c, id); !ok {
		return err
	}
	var in models.EmergencyProofInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	in.ActorName = purchaseActor(c).Name
	pr, err := services.SubmitEmergencyProof(id, in)
	if err != nil {
		return emergencyServiceError(c, err, "SubmitEmergencyProof")
	}
	return c.JSON(models.APIResponse{Success: true, Data: pr})
}

// DecideEmergencyPurchase: penyetuju mengesahkan atau menolak.
func DecideEmergencyPurchase(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := emergencyDocOrFail(c, id); !ok {
		return err
	}
	in, ok, err := emergencyDecisionBody(c)
	if !ok {
		return err
	}
	pr, err := services.DecideEmergencyPurchase(id, in)
	if err != nil {
		return emergencyServiceError(c, err, "DecideEmergencyPurchase")
	}
	return c.JSON(models.APIResponse{Success: true, Data: pr})
}

// CancelEmergencyPurchase: dibatalkan sebelum kas diserahkan.
func CancelEmergencyPurchase(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := emergencyDocOrFail(c, id); !ok {
		return err
	}
	in, ok, err := emergencyDecisionBody(c)
	if !ok {
		return err
	}
	pr, err := services.CancelEmergencyPurchase(id, in)
	if err != nil {
		return emergencyServiceError(c, err, "CancelEmergencyPurchase")
	}
	return c.JSON(models.APIResponse{Success: true, Data: pr})
}

// ResendEmergencyPayout: kirim ulang tugas kas ke tablet kasir.
func ResendEmergencyPayout(c *fiber.Ctx) error {
	id := c.Params("id")
	if ok, err := emergencyDocOrFail(c, id); !ok {
		return err
	}
	in, ok, err := emergencyDecisionBody(c)
	if !ok {
		return err
	}
	pr, err := services.ResendEmergencyPayout(id, in)
	if err != nil {
		return emergencyServiceError(c, err, "ResendEmergencyPayout")
	}
	return c.JSON(models.APIResponse{Success: true, Data: pr})
}

// GetEmergencySettings: batas per pembelian & plafon bulanan.
func GetEmergencySettings(c *fiber.Ctx) error {
	return c.JSON(models.APIResponse{Success: true, Data: services.GetEmergencySettings()})
}

// UpdateEmergencySettings: hanya penyetuju barang/jasa.
func UpdateEmergencySettings(c *fiber.Ctx) error {
	var in models.EmergencySettings
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid."})
	}
	s, err := services.UpdateEmergencySettings(in)
	if err != nil {
		return emergencyServiceError(c, err, "UpdateEmergencySettings")
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

// GetEmergencyRecap: rekap bulanan Pembelian Darurat untuk audit Keuangan
// (?month=YYYY-MM, ?outlet_id opsional). Cakupan outlet/unit kerja role
// dipaksakan di service.
func GetEmergencyRecap(c *fiber.Ctx) error {
	outletID := strings.TrimSpace(c.Query("outlet_id", ""))
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	recap, err := services.GetEmergencyRecap(c.Query("month", ""), outletID, getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return emergencyServiceError(c, err, "GetEmergencyRecap")
	}
	return c.JSON(models.APIResponse{Success: true, Data: recap})
}

// ExportEmergencyRecap: rekap yang sama sebagai Excel.
func ExportEmergencyRecap(c *fiber.Ctx) error {
	outletID := strings.TrimSpace(c.Query("outlet_id", ""))
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	data, filename, err := services.BuildEmergencyRecapExcel(c.Query("month", ""), outletID, getOutletScope(c), getWorkUnitScope(c))
	if err != nil {
		return emergencyServiceError(c, err, "ExportEmergencyRecap")
	}
	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
	return c.Send(data)
}
