package handlers

// Stream SSE /admin/events memakai validasi sesi yang sama dengan AdminAuth
// (T-55). Tanpa database: cookie CCTV (JWT bertanda tangan secret yang sama)
// tidak membuka stream. Dengan Postgres uji (DB_NAME berakhiran _test): token
// sah membuka stream, stream yang sedang terbuka diputus begitu akun
// dinonaktifkan, dan membuka ulang dengan token itu ditolak.

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

const sesiRahasiaSSE = "rahasia-jwt-uji-sse-0123456789abcdef"

func TestEventsMenolakTokenCCTV(t *testing.T) {
	services.InitCCTV("", sesiRahasiaSSE, "")
	tok, err := services.IssueCCTVToken("01ADMIN000SUPERADMIN00001")
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/events", Events(&config.Config{JWTSecret: sesiRahasiaSSE}))
	resp, err := app.Test(httptest.NewRequest("GET", "/events?token="+url.QueryEscape(tok), nil), 15000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("cookie CCTV membuka stream SSE: status %d, mau 401", resp.StatusCode)
	}
}

func TestEventsSesiDicabutMemutusStreamDB(t *testing.T) {
	if !strings.HasSuffix(os.Getenv("DB_NAME"), "_test") || database.DB == nil || database.DB.Ping() != nil {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
	if err := database.RunAuthMigrations(); err != nil {
		t.Fatalf("migrasi auth: %v", err)
	}
	services.InvalidateAllAdminSessions()

	suf := strings.ToLower(services.NewULID()[16:])
	role, username, password := "ujv_"+suf, "ujisse_"+suf, "rahasia-uji-123"
	if _, err := services.CreateRole(models.CreateRoleRequest{Name: role, Description: "uji sesi SSE", ScopeType: "all", Permissions: []string{"dashboard"}}); err != nil {
		t.Fatalf("buat role uji: %v", err)
	}
	admin, err := services.CreateAdmin(models.CreateAdminRequest{Username: username, Password: password, Name: "Uji SSE", Role: role})
	if err != nil {
		t.Fatalf("buat akun uji: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM cloud_admins WHERE id = $1`, admin.ID)
		database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role)
		database.DB.Exec(`DELETE FROM roles WHERE name = $1`, role)
		services.InvalidateAllAdminSessions()
	})
	login, err := services.AdminLogin(models.AdminLoginRequest{Username: username, Password: password}, sesiRahasiaSSE)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Ping (dan pemeriksaan ulang sesi) dipercepat agar tes tidak menunggu 25 detik.
	asliJeda := eventsPingInterval
	eventsPingInterval = 50 * time.Millisecond
	t.Cleanup(func() { eventsPingInterval = asliJeda })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/events", Events(&config.Config{JWTSecret: sesiRahasiaSSE}))
	go app.Listener(ln)
	t.Cleanup(func() { app.ShutdownWithTimeout(2 * time.Second) })
	alamat := "http://" + ln.Addr().String() + "/events?token=" + url.QueryEscape(login.Token)

	resp, err := http.Get(alamat)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token sah: status %d, mau 200", resp.StatusCode)
	}
	baris, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.HasPrefix(baris, ": connected") {
		t.Fatalf("baris pertama stream = %q (err %v)", baris, err)
	}

	// Akun dinonaktifkan saat stream terbuka → server memutus stream.
	if _, err := services.ToggleAdminActive(admin.ID); err != nil {
		t.Fatal(err)
	}
	selesai := make(chan struct{})
	go func() {
		io.Copy(io.Discard, resp.Body)
		close(selesai)
	}()
	select {
	case <-selesai:
	case <-time.After(5 * time.Second):
		t.Fatal("stream SSE tidak diputus setelah akun dinonaktifkan")
	}

	// Membuka ulang dengan token yang sama ditolak.
	resp2, err := http.Get(alamat)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token akun nonaktif membuka ulang stream: status %d, mau 401", resp2.StatusCode)
	}
}
