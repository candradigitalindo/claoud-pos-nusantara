package main

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/routes"
	"cloud-pos/services"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.Load()

	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	if err := database.SeedAdmin(); err != nil {
		log.Printf("Admin seeder warning: %v", err)
	}

	// Pelanggan: proses order lama (pra-fitur) menjadi master pelanggan, one-shot.
	// Di goroutine agar boot tidak tertahan; upsert per-order idempotent.
	go services.BackfillCustomers()

	// CCTV relay: siapkan kunci enkripsi kredensial RTSP, secret stream token,
	// dan alamat media server go2rtc.
	services.InitCCTV(cfg.CameraEncKey, cfg.JWTSecret, cfg.Go2rtcURL)

	// PPIC: evaluasi alert harian (expired/ROP) + broadcast SSE ke dashboard.
	services.StartPpicScheduler()

	// Medsos: tarik angka IG/TikTok tiap dini hari untuk grafik Analisa Bisnis.
	services.StartSocialScheduler()

	// Terbitkan work order perawatan aset yang jatuh tempo (harian, ~03:00 WIB).
	services.StartAssetMaintenanceScheduler()

	// Cadangkan bukti foto serah terima ke email (tiap 15 menit).
	services.StartPhotoBackupScheduler()

	// WhatsApp: antrean pengiriman + penjadwal (rekap harian, pengingat reservasi,
	// perangkat offline). Tanpa WA_GATEWAY_URL fitur ini diam.
	services.InitWhatsApp(cfg)

	app := fiber.New(fiber.Config{
		AppName:        "Nusantara POS Cloud API v1.0.0",
		BodyLimit:      200 * 1024 * 1024,
		ServerHeader:   "NusantaraPOS-Cloud",
		UnescapePath:   true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Outlet-ID, X-Outlet-Code, X-Cloud-Signature",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	routes.Setup(app, cfg)

	// ── Auto-start Vite dev server when DEV_UI=true ─────────────
	var uiCmd *exec.Cmd
	if os.Getenv("DEV_UI") == "true" {
		uiCmd = startUIDevServer()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down Cloud API server...")
		if uiCmd != nil && uiCmd.Process != nil {
			log.Println("Stopping UI dev server...")
			uiCmd.Process.Kill()
		}
		app.Shutdown()
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Nusantara POS Cloud API running on http://localhost%s", addr)

	if lanIP := getLANIP(); lanIP != "" {
		log.Printf("LAN access: http://%s%s", lanIP, addr)
	}

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// startUIDevServer spawns `npm run dev` inside the ui/ directory.
// The Vite process is a child process and is killed when the API stops.
func startUIDevServer() *exec.Cmd {
	// Locate ui/ relative to the binary (or cwd when using go run)
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		exeDir = "."
	}
	uiDir := filepath.Join(exeDir, "ui")
	// Fallback: if running via `go run main.go` the binary lives in /tmp
	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		uiDir = "ui"
	}

	npmBin := "npm"
	if runtime.GOOS == "windows" {
		npmBin = "npm.cmd"
	}

	cmd := exec.Command(npmBin, "run", "dev")
	cmd.Dir = uiDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("[UI] Failed to start dev server: %v", err)
		return nil
	}
	log.Printf("[UI] Dev server starting in %s (pid %d)", uiDir, cmd.Process.Pid)
	return cmd
}

func getLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}
