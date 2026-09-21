// wa-gateway — jembatan WhatsApp gratis untuk Cloud POS.
//
// Memakai protokol WhatsApp Web multi-device lewat pustaka whatsmeow: satu
// nomor WhatsApp biasa dipasangkan dengan memindai QR (seperti WhatsApp Web),
// lalu API utama mengirim pesan lewat HTTP ke layanan ini. Tidak ada biaya
// per pesan dan tidak ada akun Meta Business — tetapi ini jalur TIDAK RESMI:
// WhatsApp bisa memblokir nomor yang mengirim terlalu banyak/terlalu cepat.
// Pengaturan jeda dan batas harian ada di API utama, bukan di sini.
//
// Layanan ini sengaja bodoh: tidak tahu apa isi pesannya, tidak menyimpan
// antrean. Sesi (kunci enkripsi perangkat) disimpan di Postgres yang sama
// dengan aplikasi supaya ikut ter-backup harian dan tidak hilang saat
// container dibuat ulang.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("PORT", "4100")
	token := env("WA_GATEWAY_TOKEN", "")
	dsn := env("WA_DB_DSN", "")
	if token == "" {
		log.Fatal("WA_GATEWAY_TOKEN wajib diisi")
	}
	if dsn == "" {
		log.Fatal("WA_DB_DSN wajib diisi (contoh: postgres://user:pass@db:5432/cloud_pos?sslmode=disable)")
	}

	gw, err := newGateway(dsn)
	if err != nil {
		log.Fatalf("Gagal menyiapkan gateway: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(gw, token),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("wa-gateway siap di :%s (state=%s)", port, gw.snapshot().State)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("wa-gateway berhenti...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	gw.shutdown()
}
