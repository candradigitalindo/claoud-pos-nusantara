package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// Layar Perlengkapan memuat 100 aset per halaman (?page=), sedangkan pemilih
// aset di halaman lain tetap menerima daftar utuh (tanpa ?page=), dan Unduh
// Excel selalu memuat semua aset yang lolos saringan.
func TestAssetListPagedLegacyAndExport(t *testing.T) {
	requireRuteDB(t)
	sfx := strings.ToLower(services.NewULID()[20:])
	viewer, _ := ruteAkun(t, sfx+"v", []string{"assets.view"})
	// Izin apa pun di modul assets.* membuka assets.view (lihat RequirePermission),
	// jadi akun "ditolak" memakai izin dari modul lain.
	denied, _ := ruteAkun(t, sfx+"n", []string{"procurement.projects.manage"})

	outlet := services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		outlet, "AL"+sfx, "Outlet Uji Daftar Aset "+sfx, "key-daftar-aset-"+outlet); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ruteExec(`DELETE FROM outlets WHERE id = $1`, outlet) })
	t.Cleanup(func() { ruteExec(`DELETE FROM assets WHERE outlet_id = $1`, outlet) })
	for i := 0; i < 120; i++ {
		if _, err := database.DB.Exec(`INSERT INTO assets (id, outlet_id, name, quantity, purchase_price) VALUES ($1, $2, $3, 2, 5000)`,
			services.NewULID(), outlet, fmt.Sprintf("Meja Uji %03d", i)); err != nil {
			t.Fatal(err)
		}
	}

	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	get := func(path, token string) (int, []byte, string) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return resp.StatusCode, buf.Bytes(), resp.Header.Get("Content-Type")
	}
	base := "/api/v1/admin/assets?outlet_id=" + outlet

	// Tanpa ?page=: array utuh seperti semula.
	code, body, _ := get(base, viewer)
	var legacy struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &legacy); code != 200 || err != nil || len(legacy.Data) != 120 {
		t.Fatalf("daftar utuh: %d, %d baris, err %v: %s", code, len(legacy.Data), err, body)
	}

	type paged struct {
		Data       []map[string]interface{}  `json:"data"`
		Page       int                       `json:"page"`
		Limit      int                       `json:"limit"`
		Total      int                       `json:"total"`
		TotalPages int                       `json:"total_pages"`
		Summary    services.AssetListSummary `json:"summary"`
	}
	for _, tc := range []struct {
		query                   string
		page, limit, rows, last int
	}{
		{"&page=1", 1, 100, 100, 2}, // bawaan 100 per halaman
		{"&page=2&limit=100", 2, 100, 20, 2},
		{"&page=7&limit=100", 2, 100, 20, 2}, // di luar jangkauan → halaman terakhir
	} {
		code, body, _ := get(base+tc.query, viewer)
		var p paged
		if err := json.Unmarshal(body, &p); code != 200 || err != nil {
			t.Fatalf("%s: %d, err %v: %s", tc.query, code, err, body)
		}
		if p.Page != tc.page || p.Limit != tc.limit || len(p.Data) != tc.rows || p.Total != 120 || p.TotalPages != tc.last {
			t.Fatalf("%s: halaman %d/%d limit %d, %d baris dari %d", tc.query, p.Page, p.TotalPages, p.Limit, len(p.Data), p.Total)
		}
		// Ringkasan mencakup semua 120 aset, bukan hanya halaman ini.
		if p.Summary.Count != 120 || p.Summary.Units != 240 || p.Summary.Acquisition != 120*2*5000 {
			t.Fatalf("%s: ringkasan %+v", tc.query, p.Summary)
		}
	}

	code, body, ctype := get("/api/v1/admin/asset-export?outlet_id="+outlet, viewer)
	if code != 200 || !strings.Contains(ctype, "spreadsheetml") {
		t.Fatalf("ekspor: %d %s: %.200s", code, ctype, body)
	}
	x, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	rows, _ := x.GetRows("Perlengkapan")
	if want := 5 + 120 + 1; len(rows) != want { // 5 baris kepala + 120 aset + TOTAL
		t.Fatalf("ekspor berisi %d baris, harusnya %d (semua aset, tanpa paginasi)", len(rows), want)
	}

	for _, path := range []string{base + "&page=1", "/api/v1/admin/asset-export?outlet_id=" + outlet} {
		if code, _, _ := get(path, denied); code != 403 {
			t.Fatalf("%s tanpa assets.view: %d, harusnya 403", path, code)
		}
	}
}
