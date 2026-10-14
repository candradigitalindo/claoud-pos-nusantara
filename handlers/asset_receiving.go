package handlers

import (
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// roleHasPermission memeriksa izin di DALAM handler — dipakai saat satu aksi
// boleh berjalan dengan kemampuan berbeda-beda, bukan ditolak sepenuhnya.
// Aturan pencocokannya sama dengan middleware.RequirePermission.
func roleHasPermission(c *fiber.Ctx, permission string) bool {
	role, _ := c.Locals("admin_role").(string)
	if role == "superadmin" {
		return true
	}
	perms, err := services.GetRolePermissions(role)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// receiveAccess menghitung wewenang serah terima atas pengajuan :id (T-10,
// T-17). Route menerima pemegang izin buat pengajuan jenis apa pun, bagian
// Aset (assets.create), dan petugas gudang (stockledger.adjust); keputusan
// akhirnya PER BARIS di service:
//
//   - pemegang izin buat pengajuan untuk jenis ini → semua baris;
//   - assets.create → baris peralatan (aset / habis pakai / material);
//   - stockledger.adjust → baris dapur/stok, langsung ke buku stok.
//
// Dulu hanya pemegang submit yang bisa membuka dialognya, sehingga bagian
// perlengkapan dan teknisi kena 403 walau menunya tampil. Dokumen di luar
// cakupan — termasuk pengajuan pusat tanpa outlet/unit bagi role per outlet —
// ditolak di sini (T-43). Bila ditolak, respons sudah ditulis.
func receiveAccess(c *fiber.Ctx) (services.ReceiveAccess, bool, error) {
	id := c.Params("id")
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return services.ReceiveAccess{}, false, ferr
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return services.ReceiveAccess{}, false, permissionLoadError(c)
	}
	ra := services.ReceiveAccess{
		Submit: acc.can(reqType, "submit"),
		Asset:  acc.has("assets.create"),
		Stock:  acc.has("stockledger.adjust"),
	}
	switch {
	case reqType == services.RequestTypeJasa:
		return ra, false, c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pengajuan jasa diselesaikan lewat tombol Selesai, bukan Serah Terima barang.",
		})
	case reqType == services.RequestTypeBahan && !ra.Submit && !ra.Stock:
		// Seluruh baris bahan baku masuk buku stok: izin tambah aset saja
		// tidak berarti apa-apa di sini.
		return ra, false, purchaseForbidden(c, "Bahan baku diterima petugas gudang (izin penyesuaian stok) atau pengaju pengadaan bahan baku.")
	case !ra.Any():
		return ra, false, purchaseForbidden(c, "Anda tidak memiliki akses menerima barang pengadaan "+services.RequestTypeLabel(reqType)+".")
	}
	if !validatePurchaseRequestScope(c, id) {
		return ra, false, c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	return ra, true, nil
}

// GetReceivingDraft menyusun isi dialog Serah Terima: tiap sub-item pengajuan
// beserta usulan tujuannya, jenis barisnya (meja), dan sisa yang belum dicatat.
func GetReceivingDraft(c *fiber.Ctx) error {
	if _, ok, err := receiveAccess(c); !ok {
		return err
	}
	d, err := services.BuildReceivingDraft(c.Params("id"), getOutletScope(c))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
	}
	return c.JSON(models.APIResponse{Success: true, Data: d})
}

// ReceiveGoods menjalankan penerimaan: aset + stok gudang + status pengajuan,
// dalam satu transaksi. Penerima (received_by, pencatat aset/GRN) = akun yang
// login, bukan nama dari body (T-7).
func ReceiveGoods(c *fiber.Ctx) error {
	access, ok, err := receiveAccess(c)
	if !ok {
		return err
	}
	var req models.ReceiveGoodsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: "Format data tidak valid"})
	}
	// Tanpa hak gudang, baris dapur tidak dibuang melainkan ditunda ke antrean
	// gudang — barangnya memang sudah datang secara fisik.
	res, err := services.ReceiveGoods(c.Params("id"), req, purchaseActor(c), access, getOutletScope(c))
	if err != nil {
		return purchaseServiceError(c, err)
	}
	return c.JSON(models.APIResponse{Success: true, Data: res, Message: res.Message})
}

// ListIncompleteReceipts — laporan "Pengadaan diterima, barang belum lengkap".
func ListIncompleteReceipts(c *fiber.Ctx) error {
	// Bawaan: hanya perlengkapan — bahan dapur bukan urusan tim aset.
	// "?kind=semua" membuka saringan; nilai kosong TIDAK bisa dipakai untuk itu
	// karena Fiber mengembalikan nilai bawaan untuk query yang kosong.
	kind := c.Query("kind", "perlengkapan")
	if kind == "semua" {
		kind = ""
	}
	list, err := services.ListIncompleteReceipts(getOutletScope(c), kind)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat laporan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}

// ListReceivingQueue — antrean serah terima milik satu meja.
//
// Bagian Aset membuka antrean "perlengkapan", Gudang Induk membuka "dapur".
// Masing-masing hanya melihat pekerjaannya sendiri.
func ListReceivingQueue(c *fiber.Ctx) error {
	kind := c.Query("kind", "perlengkapan")
	if kind == "semua" {
		kind = ""
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	// Meja Aset/Gudang menerima semua jenis. Selain itu (hanya izin
	// pengadaan), antrean dibatasi ke jenis yang boleh dilihat: pengaju
	// barang/jasa tidak melihat bahan baku, dan sebaliknya.
	var allowedTypes []string // nil = tanpa batas jenis
	if !acc.has("assets.view") && !acc.has("stockledger.adjust") {
		allowedTypes = acc.viewableTypes()
	}
	list, err := services.ListReceivingQueue(kind, getOutletScope(c), allowedTypes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat antrean penerimaan: " + err.Error(),
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: list})
}
