package middleware

// Uji validasi sesi JWT di AdminAuth (T-55, T-56).
//
// Tanpa database: token jenis lain/rusak ditolak 401 sebelum menyentuh DB, dan
// galat DB saat memeriksa sesi = 503 (gagal-tertutup — bukan diterima dengan
// scope "all"). Dengan Postgres uji (DB_NAME berakhiran _test): token tercabut
// setelah password diganti, akun nonaktif ditolak, token lama tanpa klaim
// versi diterima selama versi akun 0, scope dibaca dari DB (bukan dari token),
// dan galat DB saat membaca scope = 503.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
)

const sesiRahasiaMW = "rahasia-jwt-uji-middleware-0123456789"

var sesiMWDBReady bool

func TestMain(m *testing.M) {
	cfg := config.Load()
	// Hanya database uji (berakhiran _test) yang boleh disentuh; selain itu
	// diarahkan ke alamat yang pasti gagal: database.DB terisi tetapi setiap
	// query galat — persis kondisi "DB tidak bisa dihubungi".
	if !strings.HasSuffix(cfg.DBName, "_test") {
		cfg.DBHost, cfg.DBPort = "db-uji-dinonaktifkan.invalid", "1"
	}
	if err := database.Connect(cfg); err == nil {
		if err := database.RunAuthMigrations(); err == nil {
			sesiMWDBReady = true
		}
	}
	os.Exit(m.Run())
}

func sesiAppMW() *fiber.App {
	app := fiber.New()
	app.Get("/x", AdminAuth(&config.Config{JWTSecret: sesiRahasiaMW}), func(c *fiber.Ctx) error {
		id, _ := c.Locals("admin_id").(string)
		role, _ := c.Locals("admin_role").(string)
		scope, _ := c.Locals("admin_scope").(string)
		outlets, _ := c.Locals("admin_outlet_ids").([]string)
		return c.SendString(fmt.Sprintf("%s|%s|%s|%s", id, role, scope, strings.Join(outlets, ",")))
	})
	return app
}

func sesiPanggilMW(t *testing.T, app *fiber.App, token string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, 15000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func sesiTandaTanganMW(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// sesiKlaimMW: bentuk token sebelum perbaikan (tanpa typ & ver) bila ver < 0,
// selain itu bentuk baru.
func sesiKlaimMW(sub, role string, ver int) jwt.MapClaims {
	now := time.Now().UTC()
	c := jwt.MapClaims{
		"sub": sub, "username": "uji", "name": "Uji", "role": role, "scope": "all",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	if ver >= 0 {
		c["typ"] = "admin"
		c["ver"] = ver
	}
	return c
}

// Token yang bukan sesi admin ditolak di tahap token (pesan "token tidak
// valid"), tidak sampai ke pencocokan akun.
func TestAdminAuthMenolakTokenBukanSesiAdmin(t *testing.T) {
	services.InitCCTV("", sesiRahasiaMW, "")
	cctv, err := services.IssueCCTVToken(sesiTandaTanganMW(t, sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", 0), sesiRahasiaMW))
	if err != nil {
		t.Fatal(err)
	}
	kedaluwarsa := sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", 0)
	kedaluwarsa["exp"] = time.Now().Add(-time.Minute).Unix()
	tanpaExp := sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", 0)
	delete(tanpaExp, "exp")
	verRusak := sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", 0)
	verRusak["ver"] = "nol"

	kasus := map[string]string{
		"cookie CCTV (typ cctv, secret sama)": cctv,
		"secret lain":                         sesiTandaTanganMW(t, sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", 0), "secret-lain-yang-juga-panjang-0123"),
		"kedaluwarsa":                         sesiTandaTanganMW(t, kedaluwarsa, sesiRahasiaMW),
		"tanpa exp":                           sesiTandaTanganMW(t, tanpaExp, sesiRahasiaMW),
		"klaim ver bukan angka":               sesiTandaTanganMW(t, verRusak, sesiRahasiaMW),
	}
	app := sesiAppMW()
	for nama, tok := range kasus {
		status, body := sesiPanggilMW(t, app, tok)
		if status != fiber.StatusUnauthorized || !strings.Contains(body, "token tidak valid") {
			t.Errorf("%s: status=%d body=%s, mau 401 token tidak valid", nama, status, body)
		}
	}
}

// DB tidak bisa dihubungi saat memeriksa sesi → 503, handler tidak jalan.
// Dulu tidak ada pemeriksaan DB sama sekali; tanpa gagal-tertutup, galat
// seperti ini mudah jatuh ke "terima dengan scope all".
func TestAdminAuthGalatDBMenolak503(t *testing.T) {
	putus, err := sql.Open("postgres", "host=db-uji-dinonaktifkan.invalid port=1 user=uji dbname=uji_test sslmode=disable connect_timeout=2")
	if err != nil {
		t.Fatal(err)
	}
	asli := database.DB
	database.DB = putus
	services.InvalidateAllAdminSessions()
	t.Cleanup(func() {
		database.DB = asli
		putus.Close()
		services.InvalidateAllAdminSessions()
	})

	app := sesiAppMW()
	for nama, ver := range map[string]int{"token baru": 0, "token lama tanpa ver": -1} {
		status, body := sesiPanggilMW(t, app, sesiTandaTanganMW(t, sesiKlaimMW("01ADMIN000SUPERADMIN00001", "superadmin", ver), sesiRahasiaMW))
		if status != fiber.StatusServiceUnavailable || strings.Contains(body, "superadmin") {
			t.Errorf("%s: status=%d body=%s, mau 503 tanpa masuk handler", nama, status, body)
		}
	}
}

// ── Dengan Postgres uji ─────────────────────────────────────────────────────

func sesiButuhDBMW(t *testing.T) {
	t.Helper()
	if !sesiMWDBReady {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
	services.InvalidateAllAdminSessions()
}

type sesiAkunMW struct{ ID, Username, Password, Role string }

func sesiBuatAkunMW(t *testing.T, scopeType string) sesiAkunMW {
	t.Helper()
	suf := strings.ToLower(services.NewULID()[16:])
	a := sesiAkunMW{Username: "ujimw_" + suf, Password: "rahasia-uji-123", Role: "ujm_" + suf}
	if _, err := services.CreateRole(models.CreateRoleRequest{
		Name: a.Role, Description: "uji sesi middleware", ScopeType: scopeType, Permissions: []string{"dashboard"},
	}); err != nil {
		t.Fatalf("buat role uji: %v", err)
	}
	admin, err := services.CreateAdmin(models.CreateAdminRequest{Username: a.Username, Password: a.Password, Name: "Uji MW", Role: a.Role})
	if err != nil {
		t.Fatalf("buat akun uji: %v", err)
	}
	a.ID = admin.ID
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM cloud_admins WHERE id = $1`, a.ID)
		database.DB.Exec(`DELETE FROM role_work_unit_scope WHERE role = $1`, a.Role)
		database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, a.Role)
		database.DB.Exec(`DELETE FROM roles WHERE name = $1`, a.Role)
		services.InvalidateAllAdminSessions()
	})
	return a
}

func sesiLoginMW(t *testing.T, username, password string) string {
	t.Helper()
	res, err := services.AdminLogin(models.AdminLoginRequest{Username: username, Password: password}, sesiRahasiaMW)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return res.Token
}

func sesiHarusMW(t *testing.T, app *fiber.App, langkah, token string, mauStatus int, mauIsi string) {
	t.Helper()
	status, body := sesiPanggilMW(t, app, token)
	if status != mauStatus || !strings.Contains(body, mauIsi) {
		t.Fatalf("%s: status=%d body=%s, mau %d berisi %q", langkah, status, body, mauStatus, mauIsi)
	}
}

func TestAdminAuthSesiDicabutDB(t *testing.T) {
	sesiButuhDBMW(t)
	a := sesiBuatAkunMW(t, "all")
	app := sesiAppMW()
	baru := sesiLoginMW(t, a.Username, a.Password)
	lama := sesiTandaTanganMW(t, sesiKlaimMW(a.ID, a.Role, -1), sesiRahasiaMW)
	sukses := a.ID + "|" + a.Role + "|all|"

	sesiHarusMW(t, app, "token login", baru, 200, sukses)
	sesiHarusMW(t, app, "token lama tanpa ver, versi akun 0", lama, 200, sukses)

	if err := services.ChangePassword(a.ID, models.ChangePasswordRequest{CurrentPassword: a.Password, NewPassword: "rahasia-baru-456"}); err != nil {
		t.Fatal(err)
	}
	sesiHarusMW(t, app, "token login setelah password diganti", baru, 401, "sesi sudah tidak berlaku")
	sesiHarusMW(t, app, "token lama setelah password diganti", lama, 401, "sesi sudah tidak berlaku")

	kedua := sesiLoginMW(t, a.Username, "rahasia-baru-456")
	sesiHarusMW(t, app, "login ulang dengan password baru", kedua, 200, sukses)

	if _, err := services.ToggleAdminActive(a.ID); err != nil {
		t.Fatal(err)
	}
	sesiHarusMW(t, app, "akun dinonaktifkan", kedua, 401, "akun tidak aktif")
}

// Scope ditegakkan dari DB, bukan dari klaim token: token yang terbit dengan
// scope "all" (mis. korban T-56 sebelum deploy) langsung mengikuti scope role
// yang tersimpan; galat DB saat membaca scope = 503.
func TestAdminAuthScopeDariDBDanGagalTertutupDB(t *testing.T) {
	sesiButuhDBMW(t)
	a := sesiBuatAkunMW(t, "all")
	app := sesiAppMW()
	tok := sesiLoginMW(t, a.Username, a.Password)
	sesiHarusMW(t, app, "scope all", tok, 200, "|all|")

	// Unit kerja + outlet uji; scope role diubah lewat SQL langsung (tanpa
	// menaikkan versi) untuk menunjukkan bahwa klaim scope di token diabaikan.
	suf := strings.ToUpper(services.NewULID()[16:])
	outletID, wuID := services.NewULID(), services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		outletID, "UJ"+suf, "Outlet Uji Sesi "+suf, "kunci-uji-sesi-"+suf); err != nil {
		t.Fatalf("buat outlet uji: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM role_work_unit_scope WHERE work_unit_id = $1`, wuID)
		database.DB.Exec(`DELETE FROM work_units WHERE id = $1`, wuID)
		database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outletID)
	})
	if _, err := database.DB.Exec(`INSERT INTO work_units (id, outlet_id, name) VALUES ($1, $2, $3)`, wuID, outletID, "Unit Uji "+suf); err != nil {
		t.Fatalf("buat unit kerja uji: %v", err)
	}
	if _, err := database.DB.Exec(`UPDATE roles SET scope_type = 'specific' WHERE name = $1`, a.Role); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1, $2)`, a.Role, wuID); err != nil {
		t.Fatal(err)
	}
	services.InvalidateAllAdminSessions() // sama dengan menunggu cache kedaluwarsa
	sesiHarusMW(t, app, "token ber-klaim all, role kini specific", tok, 200, "|specific|"+outletID)

	// Galat DB tepat di langkah baca outlet scope → 503, bukan scope all.
	dasar, err := pq.NewConnector(config.Load().DSN())
	if err != nil {
		t.Fatal(err)
	}
	gagal := sql.OpenDB(sesiConnectorGagalMW{dasar: dasar, pola: "role_work_unit_scope"})
	asli := database.DB
	database.DB = gagal
	services.InvalidateAllAdminSessions()
	t.Cleanup(func() {
		database.DB = asli
		gagal.Close()
		services.InvalidateAllAdminSessions()
	})
	sesiHarusMW(t, app, "galat DB saat baca scope", tok, 503, "tidak dapat diperiksa")
}

// sesiConnectorGagalMW: koneksi Postgres sungguhan yang menolak query berisi
// pola tertentu (lihat juga services/auth_session_test.go).
type sesiConnectorGagalMW struct {
	dasar *pq.Connector
	pola  string
}

func (k sesiConnectorGagalMW) Connect(ctx context.Context) (driver.Conn, error) {
	c, err := k.dasar.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return sesiConnGagalMW{Conn: c, pola: k.pola}, nil
}

func (k sesiConnectorGagalMW) Driver() driver.Driver { return k.dasar.Driver() }

type sesiConnGagalMW struct {
	driver.Conn
	pola string
}

func (c sesiConnGagalMW) Prepare(q string) (driver.Stmt, error) {
	if strings.Contains(q, c.pola) {
		return nil, fmt.Errorf("galat DB uji pada query berisi %q", c.pola)
	}
	return c.Conn.Prepare(q)
}
