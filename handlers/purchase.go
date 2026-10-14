package handlers

import (
	"errors"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// ── Izin pengadaan per jenis pengajuan ─────────────────────────────────────
//
// Route hanya menyaring "punya izin pengadaan jenis apa pun"
// (procurement.requests.X ATAU procurement.rawmaterials.X). Keputusan akhir
// diambil di sini setelah jenis pengajuannya diketahui: barang/jasa diatur
// procurement.requests.*, bahan baku diatur procurement.rawmaterials.*
// (services.ProcurementPermission). Aksi baru cukup ditambahkan sebagai nama
// aksi — mis. "verify" — tanpa menulis ulang cek izin di tiap handler.

// purchaseAccess: izin role pemanggil, dimuat sekali per request.
type purchaseAccess struct {
	superadmin bool
	perms      []string
}

func loadPurchaseAccess(c *fiber.Ctx) (purchaseAccess, error) {
	role, _ := c.Locals("admin_role").(string)
	// Bypass superadmin disamakan dengan middleware; tanpa itu izin per-aksi
	// bergantung pada berhasilnya seeding role_permissions.
	if role == "superadmin" {
		return purchaseAccess{superadmin: true}, nil
	}
	perms, err := services.GetRolePermissions(role)
	return purchaseAccess{perms: perms}, err
}

// can: boleh menjalankan aksi pengadaan atas pengajuan berjenis requestType.
func (a purchaseAccess) can(requestType, action string) bool {
	return a.superadmin || services.PermitsProcurement(a.perms, requestType, action)
}

// has: memegang kunci izin di luar pengadaan per jenis (mis. finance.payments.pay).
func (a purchaseAccess) has(perm string) bool {
	return a.superadmin || services.HasPermission(a.perms, perm)
}

// viewableTypes: jenis yang boleh dilihat; nil = tanpa batas (superadmin).
func (a purchaseAccess) viewableTypes() []string {
	if a.superadmin {
		return nil
	}
	return services.ViewableRequestTypes(a.perms)
}

func permissionLoadError(c *fiber.Ctx) error {
	return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
		Success: false, Error: "Gagal memuat permission.",
	})
}

func purchaseForbidden(c *fiber.Ctx, msg string) error {
	if msg == "" {
		msg = "Anda tidak memiliki akses untuk aksi ini."
	}
	return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: msg})
}

// requirePurchasePerm memeriksa aksi pengadaan untuk satu jenis pengajuan.
// Bila ditolak, respons sudah ditulis: pemanggil cukup `return err`.
func requirePurchasePerm(c *fiber.Ctx, requestType, action, deniedMsg string) (bool, error) {
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return false, permissionLoadError(c)
	}
	if !acc.can(requestType, action) {
		return false, purchaseForbidden(c, deniedMsg)
	}
	return true, nil
}

// purchaseActor: pelaku aksi pengadaan dari sesi login (middleware) — id akun
// dan nama tampilannya. Nilai actor_name / requested_by di body tidak pernah
// dipakai (T-7): dulu siapa pun bisa tercatat sebagai penyetuju owner.
func purchaseActor(c *fiber.Ctx) services.Actor {
	id, _ := c.Locals("admin_id").(string)
	username, _ := c.Locals("admin_username").(string)
	return services.ResolveActor(id, username)
}

// purchaseServiceError: galat layanan pengadaan → respons. Dokumen yang
// berubah sejak dibuka (version/status beda) = 409 supaya layar memuat ulang;
// selebihnya 400 dengan pesannya.
func purchaseServiceError(c *fiber.Ctx, err error) error {
	var ce services.ConflictError
	if errors.As(err, &ce) {
		return c.Status(fiber.StatusConflict).JSON(models.APIResponse{Success: false, Error: ce.Error()})
	}
	return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: err.Error()})
}

// purchaseTypeOrFail membaca jenis pengajuan; bila tidak ada, respons 404
// sudah ditulis.
func purchaseTypeOrFail(c *fiber.Ctx, id string) (string, bool, error) {
	t, err := services.PurchaseRequestTypeOf(id)
	if err != nil {
		return "", false, c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Pengajuan tidak ditemukan.",
		})
	}
	return t, true, nil
}

func ListPurchaseRequests(c *fiber.Ctx) error {
	outletID := c.Query("outlet_id", "")
	if outletID != "" && !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}
	workUnitID := c.Query("work_unit_id", "")
	status := c.Query("status", "")
	requestType := c.Query("type", "")
	projectID := c.Query("project_id", "")
	parentID := c.Query("parent_id", "")
	excludeMasters := c.Query("exclude_masters", "") == "true"
	// Halaman Pembayaran (exclude_masters) hanya memuat tagihan: dokumen yang
	// sudah disetujui owner. Pilihan "Semua" dulu ikut memuat pengajuan yang
	// menunggu verifikasi/persetujuan, ditolak, dan batal dari semua jenis
	// (bernominal Rp 0) — kini berarti semua status berkomitmen.
	if excludeMasters {
		if status == "" {
			status = services.PurchaseStatusCommittedFilter
		} else if !services.IsCommittedPurchaseStatus(status) {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Halaman Pembayaran hanya memuat pengajuan yang sudah disetujui.",
			})
		}
	}
	search := strings.TrimSpace(c.Query("search", ""))
	page, limit := getPagination(c)
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)

	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	// Halaman Pembayaran (exclude_masters) menagih SEMUA jenis pengajuan:
	// pemegang finance.payments.view melihat seluruhnya, termasuk bahan baku.
	financeAll := excludeMasters && acc.has("finance.payments.view")
	var allowedTypes []string // nil = tanpa batas jenis
	if requestType != "" {
		if !services.IsValidRequestType(requestType) {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "type harus 'barang', 'jasa', atau 'bahan'.",
			})
		}
		if !financeAll && !acc.can(requestType, "view") {
			return purchaseForbidden(c, "Anda tidak memiliki akses melihat pengadaan "+services.RequestTypeLabel(requestType)+".")
		}
	} else if !financeAll {
		allowedTypes = acc.viewableTypes()
	}

	result, err := services.ListPurchaseRequests(outletID, workUnitID, status, requestType, projectID, parentID, excludeMasters, search, scopeIDs, wuScopeIDs, allowedTypes, page, limit)
	if err != nil {
		log.Printf("ListPurchaseRequests error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat data pengajuan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func CreatePurchaseRequest(c *fiber.Ctx) error {
	var input models.CreatePurchaseRequestInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if len(input.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "items wajib diisi.",
		})
	}

	if !services.IsValidRequestType(input.RequestType) {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "request_type harus 'barang', 'jasa', atau 'bahan'.",
		})
	}
	if ok, err := requirePurchasePerm(c, input.RequestType, "submit",
		"Anda tidak memiliki akses mengajukan pengadaan "+services.RequestTypeLabel(input.RequestType)+"."); !ok {
		return err
	}
	if input.RequestType == services.RequestTypeBahan && strings.TrimSpace(input.ProjectID) != "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pengadaan bahan baku tidak terkait projek",
		})
	}

	for _, item := range input.Items {
		if item.Name == "" || len(item.Items) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Setiap pengadaan harus memiliki nama dan minimal 1 item.",
			})
		}
		for _, sub := range item.Items {
			if sub.Name == "" || sub.Qty <= 0 || sub.HpsPrice < 0 {
				return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
					Success: false, Error: "Setiap item harus memiliki nama, qty > 0, dan harga HPS >= 0.",
				})
			}
		}
	}

	// Pengajuan manual WAJIB atas nama satu unit kerja dalam cakupan
	// pengaju; outlet diturunkan dari unit itu di service (T-9). Dulu form
	// mengirim outlet & unit kosong dan server menerimanya, sehingga lahir
	// pengajuan yatim yang tidak bisa dilihat, diubah, atau diterima
	// pengajunya sendiri.
	input.WorkUnitID = strings.TrimSpace(input.WorkUnitID)
	input.OutletID = strings.TrimSpace(input.OutletID)
	if input.WorkUnitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pilih unit kerja pengaju (work_unit_id wajib).",
		})
	}
	if !validateWorkUnitAccess(c, input.WorkUnitID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Unit kerja tidak dalam scope Anda"})
	}
	if input.OutletID != "" && !validateOutletAccess(c, input.OutletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Outlet tidak dalam scope Anda"})
	}
	// Belanja projek hanya untuk projek dalam cakupan pengaju (T-24).
	if pid := strings.TrimSpace(input.ProjectID); pid != "" && !validateProjectScope(c, pid) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Projek di luar cakupan Anda"})
	}

	// Pengaju = akun yang login, bukan isi requested_by (T-7).
	actor := purchaseActor(c)
	input.RequestedBy, input.RequestedByID = actor.Name, actor.ID

	result, err := services.CreatePurchaseRequest(input)
	if err != nil {
		// Kesalahan isi permintaan ditampilkan apa adanya — pengaju perlu tahu
		// apa yang salah. Kegagalan sistem tetap disembunyikan.
		var ve services.ValidationError
		if errors.As(err, &ve) {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{Success: false, Error: ve.Error()})
		}
		log.Printf("CreatePurchaseRequest error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal membuat pengajuan.",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.APIResponse{Success: true, Data: result})
}

func GetPurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return ferr
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	// Detail juga dibuka dari halaman Pembayaran, yang berlaku untuk semua jenis.
	if !acc.can(reqType, "view") && !acc.has("finance.payments.view") {
		return purchaseForbidden(c, "Anda tidak memiliki akses melihat pengadaan "+services.RequestTypeLabel(reqType)+".")
	}

	result, err := services.GetPurchaseRequest(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat detail: " + err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func SplitPurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.SplitPurchaseRequestInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if input.VendorName == "" || len(input.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "vendor_name dan items wajib diisi.",
		})
	}

	// Split vendor = pekerjaan purchasing untuk jenis pengajuan ini.
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return ferr
	}
	if ok, err := requirePurchasePerm(c, reqType, "purchasing", "Anda tidak memiliki akses untuk melakukan split vendor."); !ok {
		return err
	}

	result, err := services.SplitPurchaseRequest(id, input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func UpdatePurchaseStatus(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.UpdatePurchaseStatusInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Action == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "action wajib diisi.",
		})
	}

	reqType, status, err := services.PurchaseRequestTypeAndStatus(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(models.APIResponse{
			Success: false, Error: "Pengajuan tidak ditemukan.",
		})
	}
	if input.Action == "verify" && reqType == services.RequestTypeBahan {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pengadaan bahan baku tidak melalui verifikasi — pengajuannya langsung menunggu persetujuan.",
		})
	}

	// Per-action permission check. Aksi pengadaan dipetakan ke izin jenis
	// pengajuannya (barang/jasa → procurement.requests.*, bahan →
	// procurement.rawmaterials.*); pembayaran berlaku untuk semua jenis.
	// Tolak bergantung tahap: saat pending = keputusan verifikator (verify),
	// saat verified = keputusan penyetuju (approve).
	procAction, known := services.PurchaseActionPermission(input.Action, status)
	if !known {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Action tidak valid.",
		})
	}
	acc, err := loadPurchaseAccess(c)
	if err != nil {
		return permissionLoadError(c)
	}
	allowed := false
	if procAction != "" {
		allowed = acc.can(reqType, procAction)
	} else {
		// Mencairkan uang butuh izin sendiri. Sebelumnya cukup
		// finance.payments.view — izin membaca halaman Pembayaran — sehingga
		// siapa pun yang boleh melihat daftar tagihan juga boleh membayarnya.
		allowed = acc.has("finance.payments.pay")
	}
	if !allowed {
		return purchaseForbidden(c, "")
	}

	// Pelaku dari sesi login; actor_name kiriman UI lama diabaikan (T-7).
	actor := purchaseActor(c)
	input.ActorName, input.ActorID = actor.Name, actor.ID
	// Izin di atas diputus atas tahap ini; bila dokumen sudah pindah tahap
	// sebelum dikunci service, aksinya ditolak 409.
	input.ExpectedStatus = status

	result, err := services.UpdatePurchaseStatus(id, input)
	if err != nil {
		return purchaseServiceError(c, err)
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func UpdatePurchaseItems(c *fiber.Ctx) error {
	id := c.Params("id")

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}

	var input models.UpdatePurchaseItemsInput
	if err := c.BodyParser(&input); err != nil {
		log.Printf("[UpdatePurchaseItems] BodyParser error for id=%s: %v", id, err)
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Format data tidak valid.",
		})
	}

	if len(input.Items) == 0 {
		log.Printf("[UpdatePurchaseItems] Empty items for id=%s, body=%s", id, string(c.Body()))
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Items tidak boleh kosong.",
		})
	}

	// Check current status to determine required permission
	var currentStatus, reqType string
	if err := database.DB.QueryRow("SELECT status, request_type FROM purchase_requests WHERE id = $1", id).Scan(&currentStatus, &reqType); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Pengajuan tidak ditemukan.",
		})
	}

	var action string
	switch currentStatus {
	case "pending":
		action = "submit" // pengaju edits qty
	case "approved":
		// Purchasing hanya mengisi harga final, vendor, dan invoice — service
		// menolak perubahan isi lain (T-6).
		action = "purchasing"
	case "verified":
		// Bahan baku tidak melalui verifikasi: lahir 'verified' (Menunggu
		// Persetujuan) dan pengajunya masih boleh mengubah isi sampai
		// disetujui. Barang & jasa yang sudah diverifikasi terkunci.
		if reqType != services.RequestTypeBahan {
			return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
				Success: false, Error: "Pengajuan sudah diverifikasi dan menunggu persetujuan — isinya terkunci.",
			})
		}
		action = "submit"
	default:
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Tidak bisa edit item pada status ini.",
		})
	}

	if ok, err := requirePurchasePerm(c, reqType, action, ""); !ok {
		return err
	}
	input.ExpectedStatus = currentStatus

	result, err := services.UpdatePurchaseItems(id, input)
	if err != nil {
		log.Printf("[UpdatePurchaseItems] ERROR for id=%s: %v", id, err)
		return purchaseServiceError(c, err)
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func DeletePurchaseRequest(c *fiber.Ctx) error {
	id := c.Params("id")
	roleName, _ := c.Locals("admin_role").(string)
	isAdmin := roleName == "superadmin" || roleName == "admin"

	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	reqType, found, ferr := purchaseTypeOrFail(c, id)
	if !found {
		return ferr
	}
	if ok, err := requirePurchasePerm(c, reqType, "submit", ""); !ok {
		return err
	}

	if err := services.DeletePurchaseRequest(id, isAdmin); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: err.Error(),
		})
	}

	return c.JSON(models.APIResponse{Success: true, Message: "Pengajuan berhasil dihapus."})
}

func GetProcurementDashboard(c *fiber.Ctx) error {
	outletID := c.Params("id")
	if outletID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.APIResponse{
			Success: false, Error: "Outlet ID wajib diisi.",
		})
	}
	if !validateOutletAccess(c, outletID) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses outlet tidak diizinkan"})
	}

	result, err := services.GetProcurementDashboard(outletID, nil, nil)
	if err != nil {
		log.Printf("GetProcurementDashboard error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard pengadaan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetProcurementDashboardGlobal(c *fiber.Ctx) error {
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)
	result, err := services.GetProcurementDashboard("", scopeIDs, wuScopeIDs)
	if err != nil {
		log.Printf("GetProcurementDashboardGlobal error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat dashboard pengadaan.",
		})
	}

	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetPaymentStats(c *fiber.Ctx) error {
	scopeIDs := getOutletScope(c)
	wuScopeIDs := getWorkUnitScope(c)
	result, err := services.GetPaymentStats(scopeIDs, wuScopeIDs)
	if err != nil {
		log.Printf("GetPaymentStats error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat statistik pembayaran.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: result})
}

func GetPaymentHistories(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validatePurchaseRequestScope(c, id) {
		return c.Status(fiber.StatusForbidden).JSON(models.APIResponse{Success: false, Error: "Akses pengajuan tidak diizinkan"})
	}
	histories, err := services.GetPaymentHistories(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.APIResponse{
			Success: false, Error: "Gagal memuat histori pembayaran.",
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: histories})
}
