package services

// Konsep notifikasi pengadaan (owner, 15 Okt 2026): pengaju selalu dikabari
// setiap perkembangan pengajuannya (termasuk tanda terima saat diajukan),
// tiap tahap dikirim ke pemegang izin tahap itu, verifikator tahu hasil
// akhirnya, dan pengingat tertahan hanya ke pemegang tahap yang menahan.
// TestMain + requireDB ada di purchase_flow_test.go (paket yang sama).

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// waRoutingFixture: tiga akun ber-nomor WA dengan role bercakupan semua
// outlet — pengaju (tanpa izin apa pun), verifikator, keuangan.
type waRoutingFixture struct {
	requester, verifier, finance string // id akun
	cleanup                      func()
}

func newWARoutingFixture(t *testing.T) waRoutingFixture {
	t.Helper()
	// cloud_admins.role VARCHAR(20): nama role dibuat pendek.
	sfx := strings.ToLower(NewULID()[20:])
	mk := func(label string, perms ...string) string {
		role := "uw" + label + sfx
		if _, err := database.DB.Exec(`INSERT INTO roles (name, description, scope_type) VALUES ($1, 'uji', 'all')`, role); err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		for _, p := range perms {
			database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`, role, p)
		}
		id := NewULID()
		phone := "0813" + fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
		if _, err := database.DB.Exec(`INSERT INTO cloud_admins (id, username, password_hash, name, role, wa_phone, wa_notify, is_active)
			VALUES ($1, $2, 'x', $3, $4, $5, true, true)`, id, "uji_wr_"+label+sfx, "Akun "+label+" "+sfx, role, phone); err != nil {
			t.Fatalf("akun %s: %v", label, err)
		}
		t.Cleanup(func() {
			execScratch(`DELETE FROM cloud_admins WHERE id = $1`, id)
			execScratch(`DELETE FROM role_permissions WHERE role = $1`, role)
			execScratch(`DELETE FROM roles WHERE name = $1`, role)
		})
		return id
	}
	return waRoutingFixture{
		requester: mk("p"),
		verifier:  mk("v", "procurement.requests.view", "procurement.requests.verify"),
		finance:   mk("k", "finance.payments.view", "finance.payments.pay"),
	}
}

func waRecipientIDs(event, outletID string, extra ...string) map[string]bool {
	out := map[string]bool{}
	for _, r := range waRecipientsForMapped(event, outletID, nil, extra...) {
		out[strings.TrimSpace(r.ID)] = true
	}
	return out
}

func TestWAPurchaseRoutingConcept(t *testing.T) {
	requireDB(t)
	f := newWARoutingFixture(t)
	requester := waExtraIDPrefix + f.requester

	cases := []struct {
		event                     string
		wantReq, wantVer, wantKeu bool
	}{
		{"purchase.submitted", true, true, false},         // tanda terima + verifikator
		{"purchase.verified", true, false, false},         // pengaju + penyetuju (tidak ada di fixture)
		{"purchase.rejected", true, true, false},          // pengaju + verifikator tahu hasil akhir
		{"purchase.priced", true, false, false},           // pengaju
		{"purchase.payment_requested", true, false, true}, // pengaju + keuangan
		{"purchase.paid", true, false, true},              // pengaju + keuangan (+ pembelian)
		{"purchase.received", true, false, false},         // pengaju (+ pembelian)
	}
	for _, c := range cases {
		got := waRecipientIDs(c.event, "", requester)
		if got[f.requester] != c.wantReq || got[f.verifier] != c.wantVer || got[f.finance] != c.wantKeu {
			t.Errorf("%s: pengaju=%v verifikator=%v keuangan=%v, mau %v/%v/%v",
				c.event, got[f.requester], got[f.verifier], got[f.finance], c.wantReq, c.wantVer, c.wantKeu)
		}
	}
	// Pengaju dicocokkan lewat id akun: nama lain tidak ikut.
	if got := waRecipientIDs("purchase.priced", ""); got[f.requester] {
		t.Error("tanpa nama pengaju, akun tanpa izin ikut menerima purchase.priced")
	}
}

// Pengingat tertahan: hanya pemegang tahap yang menahan (+ pengaju).
func TestWAStaleReminderTargetsStageHolder(t *testing.T) {
	requireDB(t)
	f := newWARoutingFixture(t)
	requester := waExtraIDPrefix + f.requester
	mapTo := func(perm string) waPermMapper { return func(string) string { return perm } }

	got := map[string]bool{}
	for _, r := range waRecipientsForMapped("purchase.stale", "", mapTo("procurement.requests.verify"), requester) {
		got[strings.TrimSpace(r.ID)] = true
	}
	if !got[f.verifier] || !got[f.requester] || got[f.finance] {
		t.Errorf("tertahan di verifikasi: verifikator=%v pengaju=%v keuangan=%v, mau true/true/false", got[f.verifier], got[f.requester], got[f.finance])
	}
	got = map[string]bool{}
	for _, r := range waRecipientsForMapped("purchase.stale", "", mapTo("finance.payments.pay"), requester) {
		got[strings.TrimSpace(r.ID)] = true
	}
	if got[f.verifier] || !got[f.finance] {
		t.Errorf("tertahan di pembayaran: verifikator=%v keuangan=%v, mau false/true", got[f.verifier], got[f.finance])
	}
	if stage, ok := stalePurchaseStage["approved"]; !ok || stage.perm != "procurement.requests.purchasing" {
		t.Errorf("tahap approved tanpa harga harus ke pembelian: %+v", stage)
	}
}

// Semua event pengadaan terdaftar dan aktif secara bawaan.
func TestWAPurchaseEventsCatalog(t *testing.T) {
	for _, key := range []string{"purchase.submitted", "purchase.rawmaterial_submitted", "purchase.verified", "purchase.approved",
		"purchase.rejected", "purchase.priced", "purchase.payment_requested", "purchase.paid", "purchase.picked_up",
		"purchase.received", "purchase.stale"} {
		spec := waSpec(key)
		if spec == nil {
			t.Errorf("event %s belum ada", key)
			continue
		}
		if spec.Extra != "pengaju" && key != "purchase.stale" {
			t.Errorf("event %s tidak mengabari pengaju (Extra=%q)", key, spec.Extra)
		}
		if !waEventEnabled(models.WASettings{Events: map[string]bool{}}, key) {
			t.Errorf("event %s tidak aktif secara bawaan", key)
		}
	}
}

// Pengaju hanya menerima notifikasi pengajuannya sendiri: tidak ada event
// pengadaan yang dialamatkan ke izin Lihat/Buat (izin yang dipegang pengaju),
// dan role pengaju-saja ditolak sebagai penimpa penerima event pengadaan.
func TestWARequestersOnlyGetTheirOwnRequests(t *testing.T) {
	requesterPerms := []string{
		"procurement.requests.view", "procurement.requests.submit",
		"procurement.rawmaterials.view", "procurement.rawmaterials.submit",
		"procurement.dashboard.view", "procurement.projects.view", "procurement.projects.manage",
	}
	for _, spec := range waEventSpecs {
		if !strings.HasPrefix(spec.Key, "purchase.") {
			continue
		}
		for _, p := range spec.Permissions {
			for _, rp := range requesterPerms {
				if p == rp || waRoleHasPermission([]string{rp}, p) {
					t.Errorf("event %s dialamatkan ke izin pengaju %q — semua pengaju akan menerima pengajuan orang lain", spec.Key, p)
				}
			}
		}
	}
	// Role manajer outlet (Lihat + Buat) = pengaju-saja; verifikator bukan.
	mgr := waRoleInfo{perms: requesterPerms, scopeType: "specific"}
	if !WARequesterOnlyRole("manager uji", mgr) {
		t.Error("role Lihat/Buat harus dianggap pengaju-saja")
	}
	if WARequesterOnlyRole("verif uji", waRoleInfo{perms: []string{"procurement.requests.verify"}}) {
		t.Error("verifikator bukan pengaju-saja")
	}
	if WARequesterOnlyRole("superadmin", waRoleInfo{}) {
		t.Error("superadmin bukan pengaju-saja")
	}
}

// Dua pengaju di role yang sama: masing-masing hanya menerima pengajuannya.
func TestWARequesterDoesNotGetOtherRequesterEvents(t *testing.T) {
	requireDB(t)
	sfx := strings.ToLower(NewULID()[20:])
	role := "ujm" + sfx
	if _, err := database.DB.Exec(`INSERT INTO roles (name, description, scope_type) VALUES ($1, 'uji', 'all')`, role); err != nil {
		t.Fatalf("role: %v", err)
	}
	for _, p := range []string{"procurement.requests.view", "procurement.requests.submit"} {
		database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`, role, p)
	}
	ids := []string{NewULID(), NewULID()}
	for i, id := range ids {
		phone := fmt.Sprintf("0814%08d", (time.Now().UnixNano()+int64(i))%100000000)
		if _, err := database.DB.Exec(`INSERT INTO cloud_admins (id, username, password_hash, name, role, wa_phone, wa_notify, is_active)
			VALUES ($1, $2, 'x', $3, $4, $5, true, true)`, id, fmt.Sprintf("uji_m%d_%s", i, sfx), fmt.Sprintf("Manajer %d %s", i, sfx), role, phone); err != nil {
			t.Fatalf("akun: %v", err)
		}
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_admins WHERE role = $1`, role)
		execScratch(`DELETE FROM role_permissions WHERE role = $1`, role)
		execScratch(`DELETE FROM roles WHERE name = $1`, role)
	})
	for _, ev := range []string{"purchase.submitted", "purchase.verified", "purchase.approved", "purchase.rejected",
		"purchase.priced", "purchase.payment_requested", "purchase.paid", "purchase.picked_up", "purchase.received"} {
		got := waRecipientIDs(ev, "", waExtraIDPrefix+ids[0])
		if !got[ids[0]] {
			t.Errorf("%s: pengaju sendiri tidak menerima", ev)
		}
		if got[ids[1]] {
			t.Errorf("%s: pengaju lain di role yang sama ikut menerima", ev)
		}
	}
	// Penimpa penerima ke role pengaju-saja ditolak.
	if err := ValidateWAEventRoles(map[string][]string{"purchase.approved": {role}}); err == nil {
		t.Error("penimpa penerima ke role pengaju-saja diterima — semua manajer akan menerima pengajuan satu sama lain")
	}
	if err := ValidateWAEventRoles(map[string][]string{"ppic.alert": {role}}); err != nil {
		t.Errorf("penimpa event non-pengadaan ke role pengaju ditolak: %v", err)
	}
}
