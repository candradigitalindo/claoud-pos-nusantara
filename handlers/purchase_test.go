package handlers

// Uji handler pengadaan: identitas pelaku SELALU dari sesi (bukan dari body),
// unit kerja wajib untuk pengajuan manual, izin tolak bergantung tahap, dan
// version basi → 409. Bagian yang butuh database di-skip tanpa Postgres uji
// (DB_NAME wajib berakhiran _test).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

var handlerDBReady bool

func TestMain(m *testing.M) {
	cfg := config.Load()
	// Hanya database uji (berakhiran _test) yang boleh disentuh; selain itu
	// diarahkan ke alamat yang pasti gagal (lihat services TestMain).
	if !strings.HasSuffix(cfg.DBName, "_test") {
		cfg.DBHost, cfg.DBPort = "db-uji-dinonaktifkan.invalid", "1"
	}
	if err := database.Connect(cfg); err == nil {
		handlerDBReady = true
	}
	os.Exit(m.Run())
}

func requireHandlerDB(t *testing.T) {
	t.Helper()
	if !handlerDBReady {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
}

// purchaseTestApp memasang handler pengadaan dengan sesi tiruan (pengganti
// middleware AdminAuth).
func purchaseTestApp(locals map[string]interface{}) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		for k, v := range locals {
			c.Locals(k, v)
		}
		return c.Next()
	})
	app.Post("/pr", CreatePurchaseRequest)
	app.Put("/pr/:id/status", UpdatePurchaseStatus)
	app.Put("/pr/:id/items", UpdatePurchaseItems)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, url string, body interface{}) (int, models.APIResponse) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	data, _ := io.ReadAll(resp.Body)
	var out models.APIResponse
	json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

var testItemsBody = []map[string]interface{}{{
	"name": "Servis", "items": []map[string]interface{}{{"name": "Cuci AC uji handler", "qty": 1, "unit": "unit", "hps_price": 150000}},
}}

// ── Tanpa database ─────────────────────────────────────────────────────────

func TestCreatePurchaseRequiresWorkUnit(t *testing.T) {
	app := purchaseTestApp(map[string]interface{}{
		"admin_id": "static", "admin_username": "admin", "admin_role": "superadmin", "admin_scope": "all",
	})
	code, res := doJSON(t, app, "POST", "/pr", map[string]interface{}{
		"request_type": "jasa", "requested_by": "Siapa Saja", "items": testItemsBody,
	})
	if code != fiber.StatusBadRequest || !strings.Contains(res.Error, "work_unit_id") {
		t.Errorf("tanpa unit kerja: %d %q, mau 400 menyebut work_unit_id", code, res.Error)
	}
}

func TestCreatePurchaseRejectsWorkUnitOutOfScope(t *testing.T) {
	app := purchaseTestApp(map[string]interface{}{
		"admin_id": "static", "admin_username": "admin", "admin_role": "superadmin",
		"admin_scope": "specific", "admin_outlet_ids": []string{"01KOUTLETSAYA0000000000000"},
		"admin_work_unit_ids": []string{"01KUNITSAYA000000000000000"},
	})
	code, _ := doJSON(t, app, "POST", "/pr", map[string]interface{}{
		"request_type": "jasa", "work_unit_id": "01KUNITORANGLAIN0000000000", "items": testItemsBody,
	})
	if code != fiber.StatusForbidden {
		t.Errorf("unit di luar cakupan: status %d, mau 403", code)
	}
	code, _ = doJSON(t, app, "POST", "/pr", map[string]interface{}{
		"request_type": "jasa", "work_unit_id": "01KUNITSAYA000000000000000",
		"outlet_id": "01KOUTLETORANGLAIN00000000", "items": testItemsBody,
	})
	if code != fiber.StatusForbidden {
		t.Errorf("outlet di luar cakupan: status %d, mau 403", code)
	}
}

// ── Dengan database ────────────────────────────────────────────────────────

func newHandlerTestUnit(t *testing.T) string {
	t.Helper()
	unit := services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO work_units (id, name, created_at, updated_at) VALUES ($1, 'Unit Uji Handler', NOW(), NOW())`, unit); err != nil {
		t.Fatalf("unit uji: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM purchase_requests WHERE work_unit_id = $1`, unit)
		database.DB.Exec(`DELETE FROM work_units WHERE id = $1`, unit)
	})
	return unit
}

func prColumns(t *testing.T, id string) map[string]string {
	t.Helper()
	var reqBy, verBy, appBy, rejBy string
	var reqByID, verByID, appByID, rejByID *string
	if err := database.DB.QueryRow(`
		SELECT requested_by, COALESCE(verified_by,''), COALESCE(approved_by,''), COALESCE(rejected_by,''),
		       requested_by_id, verified_by_id, approved_by_id, rejected_by_id
		FROM purchase_requests WHERE id = $1`, id).
		Scan(&reqBy, &verBy, &appBy, &rejBy, &reqByID, &verByID, &appByID, &rejByID); err != nil {
		t.Fatalf("baca pengajuan: %v", err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	return map[string]string{
		"requested_by": reqBy, "verified_by": verBy, "approved_by": appBy, "rejected_by": rejBy,
		"requested_by_id": deref(reqByID), "verified_by_id": deref(verByID),
		"approved_by_id": deref(appByID), "rejected_by_id": deref(rejByID),
	}
}

func TestPurchaseIdentityFromSessionNotBody(t *testing.T) {
	requireHandlerDB(t)
	adminID := services.NewULID()
	username := "uji_sesi_" + strings.ToLower(adminID[20:])
	if _, err := database.DB.Exec(`INSERT INTO cloud_admins (id, username, password_hash, name, role) VALUES ($1, $2, 'x', 'Penyetuju Uji Sesi', 'superadmin')`,
		adminID, username); err != nil {
		t.Fatalf("akun uji: %v", err)
	}
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM cloud_admins WHERE id = $1`, adminID) })
	unit := newHandlerTestUnit(t)

	app := purchaseTestApp(map[string]interface{}{
		"admin_id": adminID, "admin_username": username, "admin_role": "superadmin", "admin_scope": "all",
	})

	code, res := doJSON(t, app, "POST", "/pr", map[string]interface{}{
		"request_type": "jasa", "requested_by": "Orang Lain", "work_unit_id": unit, "items": testItemsBody,
	})
	if code != fiber.StatusCreated {
		t.Fatalf("create: %d %q", code, res.Error)
	}
	raw, _ := json.Marshal(res.Data)
	var pr models.PurchaseRequest
	json.Unmarshal(raw, &pr)
	if cols := prColumns(t, pr.ID); cols["requested_by"] != "Penyetuju Uji Sesi" || cols["requested_by_id"] != adminID {
		t.Errorf("pengaju tersimpan = %q / %q, mau dari sesi (bukan 'Orang Lain')", cols["requested_by"], cols["requested_by_id"])
	}

	// Version basi → 409.
	code, res = doJSON(t, app, "PUT", "/pr/"+pr.ID+"/status", map[string]interface{}{
		"action": "verify", "version": "2001-01-01T00:00:00Z",
	})
	if code != fiber.StatusConflict || res.Error != "Pengajuan berubah, muat ulang" {
		t.Errorf("verify version basi: %d %q, mau 409 'Pengajuan berubah, muat ulang'", code, res.Error)
	}

	cur, _ := services.GetPurchaseRequest(pr.ID)
	code, res = doJSON(t, app, "PUT", "/pr/"+pr.ID+"/status", map[string]interface{}{
		"action": "verify", "actor_name": "Palsu", "version": cur.Version,
		"rejected_items": []interface{}{},
	})
	if code != fiber.StatusOK {
		t.Fatalf("verify: %d %q", code, res.Error)
	}
	code, res = doJSON(t, app, "PUT", "/pr/"+pr.ID+"/status", map[string]interface{}{
		"action": "approve", "actor_name": "Super Admin",
	})
	if code != fiber.StatusOK {
		t.Fatalf("approve: %d %q", code, res.Error)
	}
	cols := prColumns(t, pr.ID)
	if cols["verified_by"] != "Penyetuju Uji Sesi" || cols["verified_by_id"] != adminID {
		t.Errorf("verifikator tersimpan = %q / %q, mau dari sesi (bukan 'Palsu')", cols["verified_by"], cols["verified_by_id"])
	}
	if cols["approved_by"] != "Penyetuju Uji Sesi" || cols["approved_by_id"] != adminID {
		t.Errorf("penyetuju tersimpan = %q / %q, mau dari sesi (bukan 'Super Admin')", cols["approved_by"], cols["approved_by_id"])
	}

	// Edit approved selain harga → 400 (isi terkunci, T-6).
	code, _ = doJSON(t, app, "PUT", "/pr/"+pr.ID+"/items", map[string]interface{}{
		"items": []map[string]interface{}{{"name": "Servis", "items": []map[string]interface{}{
			{"name": "Cuci AC uji handler", "qty": 5, "unit": "unit", "hps_price": 150000}}}},
	})
	if code != fiber.StatusBadRequest {
		t.Errorf("ubah qty setelah approve: status %d, mau 400", code)
	}
}

func TestRejectPermissionDependsOnStage(t *testing.T) {
	requireHandlerDB(t)
	role := "uji_verif_" + strings.ToLower(services.NewULID()[18:])
	for _, p := range []string{"procurement.requests.view", "procurement.requests.verify"} {
		if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`, role, p); err != nil {
			t.Fatalf("izin uji: %v", err)
		}
	}
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role) })
	unit := newHandlerTestUnit(t)

	mk := func() *models.PurchaseRequest {
		pr, err := services.CreatePurchaseRequest(models.CreatePurchaseRequestInput{
			RequestType: "jasa", RequestedBy: "pengaju", WorkUnitID: unit,
			Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 1, HpsPrice: 1000}}}},
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return pr
	}
	// Pelaku tanpa baris cloud_admins: nama jatuh ke username sesi.
	app := purchaseTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "verifikator_uji", "admin_role": role, "admin_scope": "all",
	})

	pending := mk()
	if code, _ := doJSON(t, app, "PUT", "/pr/"+pending.ID+"/status", map[string]interface{}{"action": "approve"}); code != fiber.StatusForbidden {
		t.Errorf("verifikator menyetujui: status %d, mau 403", code)
	}
	code, res := doJSON(t, app, "PUT", "/pr/"+pending.ID+"/status", map[string]interface{}{
		"action": "reject", "rejected_reason": "Tersedia di outlet lain",
	})
	if code != fiber.StatusOK {
		t.Fatalf("verifikator menolak pending: %d %q", code, res.Error)
	}
	if cols := prColumns(t, pending.ID); cols["rejected_by"] != "verifikator_uji" || cols["approved_by"] != "" {
		t.Errorf("penolak = %q, approved_by = %q; mau verifikator_uji dan kosong", cols["rejected_by"], cols["approved_by"])
	}

	verified := mk()
	if code, res := doJSON(t, app, "PUT", "/pr/"+verified.ID+"/status", map[string]interface{}{
		"action": "verify", "version": verified.Version,
	}); code != fiber.StatusOK {
		t.Fatalf("verify: %d %q", code, res.Error)
	}
	if code, _ := doJSON(t, app, "PUT", "/pr/"+verified.ID+"/status", map[string]interface{}{
		"action": "reject", "rejected_reason": "Anggaran habis",
	}); code != fiber.StatusForbidden {
		t.Errorf("verifikator menolak dokumen verified (tahap penyetuju): status %d, mau 403", code)
	}
	// Pending + verify izin: edit item tetap butuh submit.
	if code, _ := doJSON(t, app, "PUT", "/pr/"+mk().ID+"/items", map[string]interface{}{"items": testItemsBody}); code != fiber.StatusForbidden {
		t.Errorf("verifikator mengubah isi pengajuan: status %d, mau 403", code)
	}
}
