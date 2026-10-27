package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"log"
	"sort"
	"strings"
)

// ── Perutean notifikasi internal berdasarkan POSISI ─────────────────────────
//
// Penerima internal utama adalah AKUN PENGGUNA yang punya nomor WhatsApp
// (cloud_admins.wa_phone). Sebuah event dikirim ke pengguna bila:
//
//  1. role-nya memegang salah satu izin yang terkait event itu (mis. laporan
//     tutup kasir → cashier_shifts.view; permintaan pembayaran → finance.payments.pay),
//     ATAU admin menimpa daftar role untuk event itu di pengaturan (wa_event_roles);
//  2. bila event terkait satu outlet, outlet itu masuk batasan unit kerja
//     role-nya (role ber-scope "specific" hanya menerima outletnya sendiri);
//  3. akunnya aktif dan opsi "terima notifikasi WA" menyala.
//
// Superadmin menerima semua event internal (kecuali daftar role penimpa
// diisi dan tidak menyebut superadmin). Orang tertentu bisa ditambahkan di
// luar aturan (mis. pengaju pengadaan dinotifikasi hasil pengajuannya sendiri)
// lewat extraNames — dicocokkan dengan nama akun, atau dengan id akun bila
// entrinya berbentuk "id:<ULID>" (lihat waExtraMatches).
//
// Daftar penerima tambahan (wa_recipients: grup WA, nomor di luar akun) tetap
// berlaku dan digabung di waRecipientsFor.

// waRoleHasPermission meniru aturan middleware.RequirePermission: cocok persis,
// atau untuk "<modul>.view" cukup memegang izin apa pun di modul itu.
func waRoleHasPermission(perms []string, required string) bool {
	for _, p := range perms {
		if p == required {
			return true
		}
	}
	if strings.HasSuffix(required, ".view") {
		module := strings.TrimSuffix(required, ".view")
		for _, p := range perms {
			if strings.HasPrefix(p, module+".") {
				return true
			}
		}
	}
	return false
}

type waRoleInfo struct {
	perms     []string
	scopeType string
	outletIDs []string
}

// waLoadRoles memuat izin + batasan outlet semua role sekali per perutean.
func waLoadRoles() map[string]waRoleInfo {
	out := map[string]waRoleInfo{}
	perms, err := GetAllRolePermissions()
	if err != nil {
		log.Printf("[WA] baca izin role: %v", err)
	}
	rows, err := database.DB.Query(`SELECT name FROM roles`)
	if err == nil {
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				st, ids := GetRoleScopeOutletIDs(name)
				out[name] = waRoleInfo{perms: perms[name], scopeType: st, outletIDs: ids}
			}
		}
		rows.Close()
	}
	out["superadmin"] = waRoleInfo{perms: AllPermissions, scopeType: "all"}
	return out
}

type waAdminRow struct {
	ID, Name, Username, Role, Phone string
	Notify, Active                  bool
}

func waLoadAdmins() []waAdminRow {
	rows, err := database.DB.Query(`SELECT id, name, username, role, COALESCE(wa_phone,''), COALESCE(wa_notify,true), is_active
		FROM cloud_admins ORDER BY name`)
	if err != nil {
		log.Printf("[WA] baca pengguna: %v", err)
		return nil
	}
	defer rows.Close()
	out := []waAdminRow{}
	for rows.Next() {
		var a waAdminRow
		if rows.Scan(&a.ID, &a.Name, &a.Username, &a.Role, &a.Phone, &a.Notify, &a.Active) == nil {
			a.ID = strings.TrimSpace(a.ID)
			a.Role = strings.TrimSpace(a.Role)
			out = append(out, a)
		}
	}
	return out
}

// waUserMatchesEvent: apakah role ini (tanpa memandang outlet) menjadi
// penerima event, mengikuti daftar penimpa bila ada.
//
// mapPerm (boleh nil) menerjemahkan izin pada spec sebelum dicocokkan —
// mis. event pengadaan untuk pengajuan bahan baku memetakan
// procurement.requests.approve ke procurement.rawmaterials.approve.
func waUserMatchesEvent(spec *waEventSpec, role string, info waRoleInfo, s models.WASettings, mapPerm waPermMapper) bool {
	if override := s.EventRoles[spec.Key]; len(override) > 0 {
		for _, r := range override {
			if strings.EqualFold(strings.TrimSpace(r), role) {
				return true
			}
		}
		return false
	}
	if role == "superadmin" {
		return true
	}
	for _, req := range spec.Permissions {
		if mapPerm != nil {
			req = mapPerm(req)
		}
		if waRoleHasPermission(info.perms, req) {
			return true
		}
	}
	return false
}

// waCentralOutlet: penanda perutean dokumen pusat (tanpa outlet & unit kerja)
// — hanya role bercakupan semua outlet yang tercakup (lihat waPurchaseOutlet).
// Bukan ULID, jadi tidak pernah cocok dengan outlet mana pun.
const waCentralOutlet = "__pusat__"

func waRoleCoversOutlet(info waRoleInfo, outletID string) bool {
	if outletID == "" || info.scopeType != "specific" {
		return true
	}
	for _, id := range info.outletIDs {
		if id == outletID {
			return true
		}
	}
	return false
}

// waExtraIDPrefix: penanda entri extraNames berupa id akun ("id:<ULID>"),
// bukan nama tampilan. Pengaju pengadaan dicocokkan lewat id-nya (T-7):
// nama bisa kembar atau berubah, dan dulu bisa diisi bebas dari body.
const waExtraIDPrefix = "id:"

// waExtraMatches: entri extraNames (nama akun, atau "id:<ULID>") menunjuk akun a.
func waExtraMatches(extra string, a waAdminRow) bool {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return false
	}
	if strings.HasPrefix(extra, waExtraIDPrefix) {
		id := strings.TrimSpace(strings.TrimPrefix(extra, waExtraIDPrefix))
		return id != "" && strings.EqualFold(id, strings.TrimSpace(a.ID))
	}
	return strings.EqualFold(extra, strings.TrimSpace(a.Name))
}

// waUserRecipients: pengguna ber-nomor yang berhak atas event ini + nama-nama
// tambahan (pengaju, dsb.). Dikembalikan sebagai WARecipient agar seragam
// dengan daftar tambahan; OutletIDs diisi dari batasan role (nil = semua).
func waUserRecipients(event, outletID string, s models.WASettings, mapPerm waPermMapper, extraNames []string) []models.WARecipient {
	spec := waSpec(event)
	if spec == nil || spec.Audience != "internal" {
		return nil
	}
	roles := waLoadRoles()
	out := []models.WARecipient{}
	for _, a := range waLoadAdmins() {
		if !a.Active || !a.Notify || a.Phone == "" {
			continue
		}
		info := roles[a.Role]
		named := false
		for _, n := range extraNames {
			if waExtraMatches(n, a) {
				named = true
				break
			}
		}
		if !named {
			if !waUserMatchesEvent(spec, a.Role, info, s, mapPerm) || !waRoleCoversOutlet(info, outletID) {
				continue
			}
		}
		var scope []string
		if info.scopeType == "specific" {
			scope = append([]string{}, info.outletIDs...)
			if len(scope) == 0 {
				scope = []string{"__none__"}
			}
		}
		out = append(out, models.WARecipient{
			ID: a.ID, Name: a.Name, Target: a.Phone, Kind: "phone", Events: []string{event},
			OutletIDs: scope, IsActive: true,
		})
	}
	return out
}

// waManualRecipients: daftar tambahan (grup / nomor di luar akun) yang
// berlangganan event ini dan tidak menyaring outletnya keluar.
func waManualRecipients(event, outletID string) []models.WARecipient {
	all, err := ListWARecipients()
	if err != nil {
		log.Printf("[WA] baca penerima tambahan: %v", err)
		return nil
	}
	out := []models.WARecipient{}
	for _, r := range all {
		if !r.IsActive {
			continue
		}
		sub := false
		for _, e := range r.Events {
			if e == "*" || e == event {
				sub = true
				break
			}
		}
		if !sub {
			continue
		}
		if outletID != "" && len(r.OutletIDs) > 0 {
			ok := false
			for _, o := range r.OutletIDs {
				if strings.TrimSpace(o) == outletID {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// waRecipientsFor menggabungkan penerima berbasis posisi (akun pengguna) dan
// penerima tambahan, tanpa nomor ganda.
func waRecipientsFor(event, outletID string, extraNames ...string) []models.WARecipient {
	return waRecipientsForMapped(event, outletID, nil, extraNames...)
}

// waPermMapper menerjemahkan kunci izin pada spec event sebelum dicocokkan
// dengan izin role. nil = apa adanya.
type waPermMapper func(perm string) string

// waBahanPermMapper: padanan izin untuk event pengajuan bahan baku.
func waBahanPermMapper(perm string) string {
	return MapProcurementPermission(perm, RequestTypeBahan)
}

func waRecipientsForMapped(event, outletID string, mapPerm waPermMapper, extraNames ...string) []models.WARecipient {
	s := GetWASettings()
	seen := map[string]bool{}
	out := []models.WARecipient{}
	for _, r := range append(waUserRecipients(event, outletID, s, mapPerm, extraNames), waManualRecipients(event, outletID)...) {
		t := WANormalizeTarget(r.Target)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		r.Target = t
		out = append(out, r)
	}
	return out
}

// ── Untuk layar Penerima ────────────────────────────────────────────────────

// WARecipientUsers: semua akun pengguna beserta event internal yang akan
// diterimanya (tanpa memandang outlet), supaya admin melihat "siapa dapat apa".
func WARecipientUsers() []models.WARecipientUser {
	s := GetWASettings()
	roles := waLoadRoles()
	outletNames := map[string]string{}
	if rows, err := database.DB.Query(`SELECT id, name FROM outlets`); err == nil {
		for rows.Next() {
			var id, name string
			if rows.Scan(&id, &name) == nil {
				outletNames[strings.TrimSpace(id)] = name
			}
		}
		rows.Close()
	}
	out := []models.WARecipientUser{}
	for _, a := range waLoadAdmins() {
		info := roles[a.Role]
		u := models.WARecipientUser{
			ID: a.ID, Name: a.Name, Username: a.Username, Role: a.Role, WAPhone: a.Phone,
			WANotify: a.Notify, IsActive: a.Active, ScopeType: info.scopeType, OutletNames: []string{}, Events: []string{},
		}
		if info.scopeType == "specific" {
			for _, id := range info.outletIDs {
				if n, ok := outletNames[id]; ok {
					u.OutletNames = append(u.OutletNames, n)
				}
			}
			sort.Strings(u.OutletNames)
		}
		if a.Active && a.Notify && a.Phone != "" {
			for i := range waEventSpecs {
				spec := &waEventSpecs[i]
				// Event pengadaan juga sampai ke pemegang izin bahan baku
				// (untuk pengajuan berjenis bahan), jadi keduanya dihitung.
				if spec.Audience == "internal" && (waUserMatchesEvent(spec, a.Role, info, s, nil) ||
					waUserMatchesEvent(spec, a.Role, info, s, waBahanPermMapper)) {
					u.Events = append(u.Events, spec.Key)
				}
			}
		}
		out = append(out, u)
	}
	return out
}

// WARoleOptions: daftar role untuk pilihan penimpa posisi per event.
func WARoleOptions() []models.WARoleOption {
	counts := map[string]int{}
	for _, a := range waLoadAdmins() {
		if a.Active && a.Notify && a.Phone != "" {
			counts[a.Role]++
		}
	}
	out := []models.WARoleOption{{Name: "superadmin", Description: "Pemilik sistem (semua akses)", ScopeType: "all", Users: counts["superadmin"]}}
	rows, err := database.DB.Query(`SELECT name, COALESCE(description,''), COALESCE(scope_type,'all') FROM roles ORDER BY name`)
	if err != nil {
		return out
	}
	defer rows.Close()
	infos := waLoadRoles()
	for rows.Next() {
		var r models.WARoleOption
		if rows.Scan(&r.Name, &r.Description, &r.ScopeType) == nil {
			r.Users = counts[r.Name]
			r.RequesterOnly = WARequesterOnlyRole(r.Name, infos[r.Name])
			out = append(out, r)
		}
	}
	return out
}

// waStageProcurementPerms: izin tahap pengadaan. Role yang memegang salah
// satunya boleh menjadi penimpa penerima event pengadaan; role yang hanya
// memegang Lihat/Buat (pengaju) tidak — setiap pengaju hanya dikabari tentang
// pengajuannya sendiri lewat jalur "pengaju", bukan semua pengajuan.
var waStageProcurementPerms = []string{
	"procurement.requests.verify", "procurement.requests.approve", "procurement.requests.purchasing", "procurement.requests.pickup",
	"procurement.rawmaterials.approve", "procurement.rawmaterials.purchasing", "procurement.rawmaterials.pickup",
	"finance.payments.pay", "stockledger.adjust", "assets.create",
}

// WARequesterOnlyRole: role yang untuk pengadaan hanya pengaju (tidak
// memegang izin tahap mana pun). superadmin bukan pengaju-saja.
func WARequesterOnlyRole(role string, info waRoleInfo) bool {
	if role == "superadmin" {
		return false
	}
	for _, p := range waStageProcurementPerms {
		if waRoleHasPermission(info.perms, p) {
			return false
		}
	}
	return true
}

// ValidateWAEventRoles menolak penimpa penerima yang membuat event pengadaan
// dikirim ke role pengaju-saja: semua pengaju di role itu akan menerima
// pengajuan satu sama lain.
func ValidateWAEventRoles(eventRoles map[string][]string) error {
	roles := waLoadRoles()
	for key, list := range eventRoles {
		if !strings.HasPrefix(key, "purchase.") {
			continue
		}
		for _, r := range list {
			r = strings.TrimSpace(r)
			info, ok := roles[r]
			if r == "" || (!ok && r != "superadmin") {
				continue
			}
			if WARequesterOnlyRole(r, info) {
				return Invalid("role %q hanya pengaju (tanpa izin verifikasi/approval/pembelian/pengambilan/bayar/gudang) — tidak bisa jadi penerima event %s: setiap pengaju otomatis dikabari tentang pengajuannya sendiri", r, key)
			}
		}
	}
	return nil
}

// waRouteDesc menjelaskan aturan otomatis sebuah event untuk layar pengaturan.
func waRouteDesc(spec *waEventSpec) string {
	if spec.Audience == "customer" {
		return "Nomor HP pelanggan pada reservasi"
	}
	labels := []string{}
	for _, p := range spec.Permissions {
		if l, ok := waPermissionLabels[p]; ok {
			labels = append(labels, l)
		} else {
			labels = append(labels, p)
		}
		// Padanan bahan baku (pengajuan berjenis 'bahan').
		if m := waBahanPermMapper(p); m != p {
			if l, ok := waPermissionLabels[m]; ok {
				labels = append(labels, l)
			}
		}
	}
	desc := "Superadmin"
	if len(labels) > 0 {
		desc += " + role dengan akses " + strings.Join(labels, " / ")
	}
	if spec.Extra != "" {
		desc += " + " + spec.Extra
	}
	return desc + " (dibatasi outlet role)"
}

var waPermissionLabels = map[string]string{
	"reservations.view":               "Reservasi",
	"cashier_shifts.view":             "Laporan Shift Kasir",
	"reports.sales.view":              "Laporan Penjualan",
	"procurement.requests.verify":     "Verifikasi Pengadaan",
	"procurement.requests.approve":    "Persetujuan Pengadaan",
	"procurement.requests.purchasing": "Purchasing (isi harga/beli)",
	"finance.payments.pay":            "Pencairan Pembayaran",
	"ppic.expiry.view":                "Kedaluwarsa PPIC",
	"warehouse_dashboard.view":        "Dashboard Gudang",
	"assets.maintenance.view":         "Perawatan Aset",
	"devices.view":                    "Monitoring Perangkat",

	// Padanan untuk pengajuan bahan baku (lihat MapProcurementPermission).
	"procurement.rawmaterials.approve":    "Persetujuan Pengadaan Bahan Baku",
	"procurement.rawmaterials.purchasing": "Purchasing Bahan Baku",
}
