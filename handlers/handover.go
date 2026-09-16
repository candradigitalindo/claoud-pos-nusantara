package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// ListAssetsAwaitingHandover — daftar pekerjaan distribusi bagian aset: barang
// yang sudah diterima tapi belum diserahkan ke PIC mana pun.
func ListAssetsAwaitingHandover(c *fiber.Ctx) error {
	list, err := services.ListAssetsAwaitingHandover(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat daftar: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func ListAssetHandovers(c *fiber.Ctx) error {
	list, err := services.ListAssetHandovers(getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat serah terima: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

func GetAssetHandover(c *fiber.Ctx) error {
	h, err := services.GetAssetHandover(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: h})
}

func CreateAssetHandover(c *fiber.Ctx) error {
	var req models.AssetHandoverRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	h, err := services.CreateAssetHandover(req, assetActor(c), getOutletScope(c))
	if err != nil {
		return invalidOr(c, err, "Gagal menyerahkan aset: ")
	}
	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: h})
}

// ── Cadangan foto ───────────────────────────────────────────────────────────

func GetPhotoBackupStatus(c *fiber.Ctx) error {
	s, err := services.GetPhotoBackupStatus()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat status cadangan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: s})
}

// RetryPhotoBackup mengembalikan foto yang gagal dikirim ke antrean.
func RetryPhotoBackup(c *fiber.Ctx) error {
	n, err := services.RetryFailedPhotoBackups()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal mengulang: " + err.Error()})
	}
	return c.JSON(models.APIResponse{
		Success: true,
		Message: "Mengulang pengiriman " + itoa(n) + " foto",
		Data:    fiber.Map{"requeued": n},
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ── Setelan cadangan foto ───────────────────────────────────────────────────

// GetPhotoBackupSettings — hanya lokasi folder Drive. Kredensialnya milik
// rclone di server (dipasang read-only lewat compose), jadi tidak ada kata
// sandi yang perlu disimpan atau ditampilkan di sini.
func GetPhotoBackupSettings(c *fiber.Ctx) error {
	vals, err := services.GetSettingsByKeys([]string{"photo_drive_remote"})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat setelan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"photo_drive_remote": vals["photo_drive_remote"],
	}})
}

func UpdatePhotoBackupSettings(c *fiber.Ctx) error {
	var body struct {
		Remote string `json:"photo_drive_remote"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	if err := services.UpdateSettings(map[string]string{"photo_drive_remote": body.Remote}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal menyimpan setelan: " + err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Message: "Lokasi cadangan foto disimpan"})
}
