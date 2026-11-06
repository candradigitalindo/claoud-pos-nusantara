package routes

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// openGet memanggil endpoint terbuka tanpa header Authorization apa pun.
func openGet(t *testing.T, app *fiber.App, path string) (int, []byte, http.Header) {
	t.Helper()
	resp, data, err := documentRequest(app, http.MethodGet, path, "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, data, resp.Header
}

// assertTanpaID gagal bila ada kunci JSON "id" / "*_id" atau ID internal muncul di body.
func assertTanpaID(t *testing.T, body []byte, ids ...string) {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("body bukan JSON: %v\n%s", err, body)
	}
	var walk func(x interface{}, path string)
	walk = func(x interface{}, path string) {
		switch n := x.(type) {
		case map[string]interface{}:
			for k, child := range n {
				if k == "id" || strings.HasSuffix(k, "_id") {
					t.Errorf("respons memuat kunci ID %q di %s", k, path)
				}
				walk(child, path+"."+k)
			}
		case []interface{}:
			for _, child := range n {
				walk(child, path+"[]")
			}
		}
	}
	walk(v, "$")
	for _, id := range ids {
		if strings.Contains(string(body), strings.TrimSpace(id)) {
			t.Errorf("respons membocorkan ID internal %s", id)
		}
	}
}

func TestOpenDataEndpointsTanpaAuthDanTanpaID(t *testing.T) {
	requireRuteDB(t)
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	if err := database.RunFnbMigrations(); err != nil { // view product_menu_kinds
		t.Fatal(err)
	}
	suffix := strings.ToLower(services.NewULID()[18:])
	outletID, mati := services.NewULID(), services.NewULID()
	slug, slugMati := "uji-terbuka-"+suffix, "uji-mati-"+suffix
	for _, o := range []struct {
		id, name, code, slug string
		active             bool
	}{{outletID, "Outlet Terbuka " + suffix, "OT" + suffix, slug, true}, {mati, "Outlet Mati " + suffix, "OM" + suffix, slugMati, false}} {
		if _, err := database.DB.Exec(`INSERT INTO outlets (id,name,code,slug,api_key,is_active) VALUES ($1,$2,$3,$4,$5,$6)`,
			o.id, o.name, o.code, o.slug, "key-"+o.id, o.active); err != nil {
			t.Fatal(err)
		}
	}
	kopi, roti, hapus := services.NewULID(), services.NewULID(), services.NewULID()
	acc, accMati := services.NewULID(), services.NewULID()
	t.Cleanup(func() {
		ruteExec(`DELETE FROM social_snapshots WHERE account_id = ANY($1::text[])`, "{"+acc+","+accMati+"}")
		ruteExec(`DELETE FROM social_accounts WHERE outlet_id = $1`, outletID)
		ruteExec(`DELETE FROM cloud_orders WHERE outlet_id = $1`, outletID)
		ruteExec(`DELETE FROM fnb_product_profiles WHERE product_id = ANY($1::text[])`, "{"+kopi+","+roti+","+hapus+"}")
		ruteExec(`DELETE FROM cloud_products WHERE outlet_id = $1`, outletID)
		ruteExec(`DELETE FROM outlets WHERE id = ANY($1::text[])`, "{"+outletID+","+mati+"}")
	})
	mustExec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	for _, p := range []struct {
		id, name, cat string
		price         float64
		deleted       bool
	}{{kopi, "Kopi Uji", "  COFFEE  ", 15000, false}, {roti, "Roti Uji", "", 20000, false}, {hapus, "Produk Terhapus", "X", 1, true}} {
		mustExec(`INSERT INTO cloud_products (id,local_id,outlet_id,name,category_name,price,is_deleted) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			p.id, p.id, outletID, p.name, p.cat, p.price, p.deleted)
	}
	mustExec(`INSERT INTO fnb_product_profiles (product_id, menu_kind) VALUES ($1,'beverage')`, kopi)
	// 15 Okt 12.00 WIB = 05.00 UTC; 16 Okt di luar filter; order void tidak dihitung.
	order := func(created, items, payment string) {
		id := services.NewULID()
		mustExec(`INSERT INTO cloud_orders (id,local_id,outlet_id,outlet_code,total_amount,items,payment_info,created_at)
			VALUES ($1,$2,$3,'OT',0,$4::jsonb,$5::jsonb,$6::timestamp)`, id, id, outletID, items, payment, created)
	}
	paid := `{"payment_status":"paid"}`
	order("2026-10-15 05:00:00", `[{"product_name":"Kopi Uji","qty":2,"price":15000,"subtotal":30000}]`, paid)
	order("2026-10-15 06:00:00", `[{"product_name":"Roti Uji","qty":1,"price":20000,"subtotal":20000},{"product_name":"Kopi Uji","qty":1,"price":15000,"subtotal":15000}]`, paid)
	order("2026-10-16 05:00:00", `[{"product_name":"Kopi Uji","qty":9,"price":15000,"subtotal":135000}]`, paid)
	order("2026-10-15 07:00:00", `[{"product_name":"Kopi Uji","qty":5,"price":15000,"subtotal":75000}]`, `{"payment_status":"paid","voided_at":"2026-10-15T08:00:00Z"}`)

	mustExec(`INSERT INTO social_accounts (id,outlet_id,platform,username,profile_url,is_active) VALUES
		($1,$3,'instagram','uji_ig','https://www.instagram.com/uji_ig/',true),
		($2,$3,'tiktok','uji_mati','https://www.tiktok.com/@uji_mati',false)`, acc, accMati, outletID)
	for _, s := range []struct {
		date      string
		followers int64
	}{{"2026-10-01", 100}, {"2026-10-10", 110}, {"2026-10-20", 125}} {
		mustExec(`INSERT INTO social_snapshots (id,account_id,captured_date,followers,following,posts_count,likes_total)
			VALUES ($1,$2,$3::date,$4,7,30,NULL)`, services.NewULID(), acc, s.date, s.followers)
	}

	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	ids := []string{outletID, mati, kopi, roti, hapus, acc, accMati}

	// Daftar outlet: outlet aktif ada, outlet nonaktif tidak.
	code, body, hdr := openGet(t, app, "/api/v1/open/outlets")
	if code != 200 || !strings.Contains(string(body), `"slug":"`+slug+`"`) || strings.Contains(string(body), slugMati) {
		t.Fatalf("outlets %d %s", code, body)
	}
	if hdr.Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("Cache-Control = %q", hdr.Get("Cache-Control"))
	}
	assertTanpaID(t, body, ids...)

	// Produk: terhapus tidak ikut, kategori dirapikan, jenis berlabel. Slug tidak peka huruf besar.
	code, body, _ = openGet(t, app, "/api/v1/open/outlets/"+strings.ToUpper(slug)+"/products")
	if code != 200 {
		t.Fatalf("products %d %s", code, body)
	}
	assertTanpaID(t, body, ids...)
	var prod struct {
		Data struct {
			Outlet   struct{ Name, Slug string }
			Total    int
			Products []struct {
				Name, Category, Kind string
				KindLabel            string `json:"kind_label"`
				Price                float64
			}
		}
	}
	json.Unmarshal(body, &prod)
	if prod.Data.Outlet.Slug != slug || prod.Data.Total != 2 || len(prod.Data.Products) != 2 {
		t.Fatalf("produk tak sesuai: %s", body)
	}
	if p := prod.Data.Products[0]; p.Name != "Kopi Uji" || p.Category != "COFFEE" || p.Kind != "beverage" || p.KindLabel != "Minuman" || p.Price != 15000 {
		t.Errorf("kopi = %+v", p)
	}
	if p := prod.Data.Products[1]; p.Name != "Roti Uji" || p.Category != "Tidak Berkategori" || p.Kind != "" || p.KindLabel != "Belum ditentukan" {
		t.Errorf("roti = %+v", p)
	}

	// Penjualan produk 15 Okt saja: Kopi 3 (30rb+15rb), Roti 1; order 16 Okt & void tidak ikut.
	code, body, _ = openGet(t, app, "/api/v1/open/outlets/"+slug+"/product-sales?date_from=2026-10-15&date_to=2026-10-15")
	if code != 200 {
		t.Fatalf("product-sales %d %s", code, body)
	}
	assertTanpaID(t, body, ids...)
	var sales struct {
		Data struct {
			DateFrom     string `json:"date_from"`
			TotalQty     int    `json:"total_qty"`
			TotalRevenue float64 `json:"total_revenue"`
			Items        []struct {
				Name, Category string
				Qty            int
				Revenue        float64
			}
		}
	}
	json.Unmarshal(body, &sales)
	if sales.Data.DateFrom != "2026-10-15" || sales.Data.TotalQty != 4 || sales.Data.TotalRevenue != 65000 || len(sales.Data.Items) != 2 ||
		sales.Data.Items[0].Name != "Kopi Uji" || sales.Data.Items[0].Qty != 3 || sales.Data.Items[0].Revenue != 45000 ||
		sales.Data.Items[0].Category != "COFFEE" || sales.Data.Items[1].Category != "Tidak Berkategori" {
		t.Fatalf("penjualan tak sesuai: %s", body)
	}

	// Medsos: akun nonaktif tidak ikut; harian dalam rentang; latest tanpa batas rentang.
	code, body, _ = openGet(t, app, "/api/v1/open/social?date_from=2026-10-05&date_to=2026-10-15")
	if code != 200 {
		t.Fatalf("social %d %s", code, body)
	}
	assertTanpaID(t, body, ids...)
	var soc struct {
		Data struct {
			Outlets []struct {
				Slug     string
				Accounts []struct {
					Platform, Username string
					Latest             *struct {
						Date      string
						Followers int64
					}
					FollowersChange *int64 `json:"followers_change"`
					Daily           []struct {
						Date      string
						Followers *int64
						Likes     *int64
					}
				}
			}
		}
	}
	json.Unmarshal(body, &soc)
	found := false
	for _, o := range soc.Data.Outlets {
		if o.Slug != slug {
			continue
		}
		found = true
		if len(o.Accounts) != 1 || o.Accounts[0].Username != "uji_ig" {
			t.Fatalf("akun tak sesuai: %+v", o.Accounts)
		}
		a := o.Accounts[0]
		if a.Latest == nil || a.Latest.Date != "2026-10-20" || a.Latest.Followers != 125 {
			t.Errorf("latest = %+v", a.Latest)
		}
		if len(a.Daily) != 1 || a.Daily[0].Date != "2026-10-10" || a.Daily[0].Likes != nil {
			t.Errorf("daily = %+v", a.Daily)
		}
		if a.FollowersChange != nil {
			t.Errorf("satu pembacaan harus followers_change null, dapat %d", *a.FollowersChange)
		}
	}
	if !found {
		t.Fatalf("outlet uji tidak ada di medsos: %s", body)
	}
	code, body, _ = openGet(t, app, "/api/v1/open/social?date_from=2026-10-01&date_to=2026-10-31")
	if code != 200 || !strings.Contains(string(body), `"followers_change":25`) {
		t.Errorf("followers_change 01–31 Okt harus 25: %d %s", code, body)
	}

	// Slug tak dikenal / outlet nonaktif → 404; tanggal salah → 400.
	for _, p := range []string{"/api/v1/open/outlets/tidak-ada-" + suffix + "/products",
		"/api/v1/open/outlets/" + slugMati + "/products", "/api/v1/open/outlets/" + slugMati + "/product-sales"} {
		if code, body, _ := openGet(t, app, p); code != 404 {
			t.Errorf("%s = %d %s", p, code, body)
		}
	}
	for _, q := range []string{"date_from=15-10-2026&date_to=2026-10-15", "date_from=2026-10-16&date_to=2026-10-15",
		"date_from=2025-01-01&date_to=2026-10-15"} {
		if code, body, _ := openGet(t, app, "/api/v1/open/outlets/"+slug+"/product-sales?"+q); code != 400 {
			t.Errorf("product-sales?%s = %d %s", q, code, body)
		}
		if code, _, _ := openGet(t, app, "/api/v1/open/social?"+q); code != 400 {
			t.Errorf("social?%s = %d", q, code)
		}
	}
}

// Rute /open tidak boleh ikut limiter /public (30/menit) milik halaman pemesanan,
// dan sebaliknya jatah /public tidak terpakai oleh penarik data.
func TestOpenDataLimiterTerpisahDariPublic(t *testing.T) {
	requireRuteDB(t)
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	for i := 0; i < 40; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/open/outlets", nil), -1)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("permintaan ke-%d ke /open kena 429 — limiter /public ikut terpasang?", i+1)
		}
	}
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/public/outlets/tidak-ada/menu", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		t.Fatal("jatah /public habis karena permintaan /open")
	}
}
