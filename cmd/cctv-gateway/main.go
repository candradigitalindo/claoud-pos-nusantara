package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"cloud-pos/internal/cctvrelay"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := &cctvrelay.Client{CloudURL: os.Getenv("CCTV_CLOUD_URL"), Token: os.Getenv("CCTV_GATEWAY_TOKEN"), RelayURL: os.Getenv("CCTV_RELAY_URL")}
	if client.RelayURL == "" {
		client.RelayURL = "http://127.0.0.1:1984"
	}
	if err := client.Validate(); err != nil {
		log.Fatal(err)
	}
	if os.Getenv("CCTV_START_RELAY") == "1" {
		config, err := os.ReadFile("/etc/cctv/go2rtc.yaml")
		if err != nil {
			log.Fatal("konfigurasi relay outlet tidak tersedia")
		}
		if err := os.WriteFile("/tmp/cctv-go2rtc.yaml", config, 0600); err != nil {
			log.Fatal("konfigurasi runtime relay tidak dapat dibuat")
		}
		cmd := exec.CommandContext(ctx, "go2rtc", "-config", "/tmp/cctv-go2rtc.yaml")
		// Log relay dapat memuat kredensial perangkat; hanya status penghubung
		// yang dicatat. Galat koneksi ditampilkan melalui tes kamera di dashboard.
		if err := cmd.Start(); err != nil {
			log.Fatal("relay outlet tidak dapat dijalankan")
		}
		go func() { _ = cmd.Wait(); cancel() }()
	}
	log.Print("Penghubung CCTV aktif; menghubungi cloud melalui HTTPS")
	if err := client.Run(ctx, func(error) { log.Print("Koneksi cloud terputus/ditolak; mencoba lagi dalam 5 detik") }); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
