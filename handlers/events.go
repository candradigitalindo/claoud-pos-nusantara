package handlers

import (
	"bufio"
	"fmt"
	"time"

	"cloud-pos/config"
	"cloud-pos/middleware"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// eventsPingInterval: jeda ping keep-alive SSE, sekaligus jeda pemeriksaan
// ulang sesi pada stream yang sedang terbuka. Variabel agar tes bisa memendekkan.
var eventsPingInterval = 25 * time.Second

// Events — stream SSE untuk update realtime dashboard admin.
// EventSource di browser tidak bisa mengirim header Authorization, jadi token
// dikirim via query (?token=) dan divalidasi manual dengan logika yang sama
// seperti middleware AdminAuth: token statis lewat helper yang sama, JWT lewat
// services.AuthenticateAdminJWT (akun aktif, versi token, role — T-55).
func Events(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Query("token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.APIResponse{Success: false, Error: "Unauthorized"})
		}
		static := cfg.StaticAdminTokenMatches(token)
		if !static {
			if _, err := services.AuthenticateAdminJWT(token, cfg.JWTSecret); err != nil {
				return middleware.RejectAdminSession(c, err)
			}
		}
		jwtSecret := cfg.JWTSecret

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		// Matikan buffering nginx untuk response ini agar event terkirim seketika.
		c.Set("X-Accel-Buffering", "no")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			sub := services.EventsHub.Subscribe()
			defer services.EventsHub.Unsubscribe(sub)

			// Ping berkala menjaga koneksi tetap hidup melewati proxy_read_timeout nginx (60s).
			ticker := time.NewTicker(eventsPingInterval)
			defer ticker.Stop()

			fmt.Fprintf(w, ": connected\n\n")
			if err := w.Flush(); err != nil {
				return
			}
			for {
				select {
				case msg, ok := <-sub:
					if !ok {
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", msg)
					if err := w.Flush(); err != nil {
						return
					}
				case <-ticker.C:
					// Stream bisa hidup berjam-jam: sesi diperiksa ulang tiap ping
					// (memakai cache sesi). Akun dinonaktifkan, password diganti,
					// role berubah, atau token kedaluwarsa → stream diputus;
					// EventSource menyambung ulang dan ditolak di atas.
					if !static {
						if _, err := services.AuthenticateAdminJWT(token, jwtSecret); err != nil {
							return
						}
					}
					fmt.Fprintf(w, ": ping\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		}))
		return nil
	}
}
