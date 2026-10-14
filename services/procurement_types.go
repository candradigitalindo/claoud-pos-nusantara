package services

import (
	"cloud-pos/database"
	"strings"
)

// ── Jenis pengajuan pengadaan & izin per jenis ─────────────────────────────
//
// Satu tabel purchase_requests melayani tiga jenis pengajuan dengan alur status
// yang sama persis (pending → approved → payment_requested → partial/paid →
// received, pecah per vendor, HPS wajib). Yang berbeda hanyalah SIAPA yang
// boleh menyentuhnya:
//
//	barang, jasa → procurement.requests.*
//	bahan        → procurement.rawmaterials.*
//
// Bahan baku dipisah karena orang yang mengajukan dan menyetujuinya berbeda:
// PPIC/gudang yang merencanakan kebutuhan dapur, bukan pengaju peralatan atau
// jasa. finance.payments.* tetap berlaku untuk SEMUA jenis — bagian keuangan
// membayar tagihan apa pun jenisnya.
//
// Seluruh pemeriksaan izin pengadaan per jenis WAJIB lewat helper di sini,
// supaya aksi baru (mis. langkah verifikasi) cukup ditambahkan sebagai satu
// nama aksi dan otomatis berlaku untuk semua jenis.

const (
	RequestTypeBarang = "barang"
	RequestTypeJasa   = "jasa"
	RequestTypeBahan  = "bahan"
)

// RequestTypes: seluruh jenis pengajuan yang sah, urutan tampilan.
var RequestTypes = []string{RequestTypeBarang, RequestTypeJasa, RequestTypeBahan}

// IsValidRequestType melaporkan apakah t jenis pengajuan yang dikenal.
func IsValidRequestType(t string) bool {
	for _, rt := range RequestTypes {
		if t == rt {
			return true
		}
	}
	return false
}

// RequestTypeLabel: label jenis untuk layar, Excel, dan pesan WhatsApp.
func RequestTypeLabel(t string) string {
	switch t {
	case RequestTypeBarang:
		return "Barang"
	case RequestTypeJasa:
		return "Jasa"
	case RequestTypeBahan:
		return "Bahan Baku"
	}
	return t
}

// procurementPermModule: awalan kunci izin yang mengatur jenis pengajuan ini.
func procurementPermModule(requestType string) string {
	if requestType == RequestTypeBahan {
		return "procurement.rawmaterials"
	}
	return "procurement.requests"
}

// ProcurementPermission memetakan (jenis, aksi) ke kunci izin.
// Aksi: view, submit, verify, approve, purchasing.
func ProcurementPermission(requestType, action string) string {
	return procurementPermModule(requestType) + "." + action
}

// MapProcurementPermission menerjemahkan kunci procurement.requests.* ke
// padanannya untuk jenis pengajuan ini. Kunci lain dikembalikan apa adanya
// (finance.payments.pay tetap finance.payments.pay). Dipakai perutean
// notifikasi yang mendaftarkan izin penerimanya dalam bentuk barang/jasa.
func MapProcurementPermission(perm, requestType string) string {
	const base = "procurement.requests."
	if !strings.HasPrefix(perm, base) {
		return perm
	}
	return ProcurementPermission(requestType, strings.TrimPrefix(perm, base))
}

// permitsKey meniru aturan middleware.RequirePermission: cocok persis, atau
// untuk "<modul>.view" cukup memegang izin apa pun berawalan "<modul>.".
func permitsKey(perms []string, required string) bool {
	module := ""
	if strings.HasSuffix(required, ".view") {
		module = strings.TrimSuffix(required, ".view") + "."
	}
	for _, p := range perms {
		if p == required || (module != "" && strings.HasPrefix(p, module)) {
			return true
		}
	}
	return false
}

// HasPermission: perms memenuhi kunci izin required (aturan prefix .view
// berlaku). Superadmin ditangani pemanggil.
func HasPermission(perms []string, required string) bool {
	return permitsKey(perms, required)
}

// PermitsProcurement: perms boleh menjalankan aksi atas pengajuan berjenis ini.
func PermitsProcurement(perms []string, requestType, action string) bool {
	return permitsKey(perms, ProcurementPermission(requestType, action))
}

// ViewableRequestTypes: jenis pengajuan yang boleh dilihat pemegang perms.
// Selalu non-nil; kosong berarti tidak ada jenis yang boleh dilihat.
func ViewableRequestTypes(perms []string) []string {
	out := []string{}
	for _, rt := range RequestTypes {
		if PermitsProcurement(perms, rt, "view") {
			out = append(out, rt)
		}
	}
	return out
}

// PurchaseRequestTypeOf membaca jenis sebuah pengajuan.
func PurchaseRequestTypeOf(id string) (string, error) {
	var t string
	err := database.DB.QueryRow(`SELECT request_type FROM purchase_requests WHERE id = $1`, id).Scan(&t)
	return t, err
}

// PurchaseRequestTypeAndStatus membaca jenis dan status sebuah pengajuan —
// izin aksi tolak bergantung tahapnya (pending → verify, verified → approve).
func PurchaseRequestTypeAndStatus(id string) (string, string, error) {
	var t, s string
	err := database.DB.QueryRow(`SELECT request_type, status FROM purchase_requests WHERE id = $1`, id).Scan(&t, &s)
	return t, s, err
}

// PurchaseActionPermission: aksi izin pengadaan untuk sebuah aksi status pada
// tahap tertentu. Kosong = bukan izin pengadaan per jenis (pembayaran memakai
// finance.payments.pay) atau aksi tidak dikenal (ok = false).
//
//	verify          → verify
//	approve         → approve
//	reject          → verify saat pending (verifikator), approve selain itu
//	request_payment → purchasing
//	receive, cancel → submit
func PurchaseActionPermission(action, status string) (procAction string, ok bool) {
	switch action {
	case "verify":
		return "verify", true
	case "approve":
		return "approve", true
	case "reject":
		if status == "pending" {
			return "verify", true
		}
		return "approve", true
	case "request_payment":
		return "purchasing", true
	case "receive", "cancel":
		return "submit", true
	case "pay":
		return "", true
	}
	return "", false
}
