package services

import (
	"cloud-pos/database"
	"log"
	"time"
)

// StartPpicScheduler menjalankan evaluasi alert PPIC harian (sekitar pukul 02:00
// zona waktu aplikasi): menghitung batch kedaluwarsa, lalu
// broadcast SSE "ppic_alert" untuk pembaruan monitor. Evaluasi
// juga dijalankan sekali saat boot agar alert langsung terisi.
func StartPpicScheduler() {
	go func() {
		evaluatePpicAlerts()
		RetryFailedStockDeductions()
		lastRunDay := ""
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			// Tiap tick: coba ulang deduksi stok penjualan yang tadi gagal
			// (stok mungkin sudah diisi lewat GRN/transfer/produksi).
			RetryFailedStockDeductions()
			loc := GetTimezoneLocation()
			now := time.Now().In(loc)
			day := now.Format("2006-01-02")
			if now.Hour() == 2 && day != lastRunDay {
				lastRunDay = day
				evaluatePpicAlerts()
			}
		}
	}()
}

func evaluatePpicAlerts() {
	var expired, expiring int
	database.DB.QueryRow(`
		SELECT
			COUNT(CASE WHEN sb.expiry_date < CURRENT_DATE THEN 1 END),
			COUNT(CASE WHEN sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 3 THEN 1 END)
		FROM stock_batches sb
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL
		AND sb.ppic_ack_at IS NULL AND w.is_active = true`).Scan(&expired, &expiring)
	log.Printf("[PPIC] Evaluasi kedaluwarsa: %d batch expired, %d batch ≤3 hari", expired, expiring)
	if expired > 0 || expiring > 0 {
		BroadcastSync("ppic_alert", "")
		go NotifyPpicAlert(expired, expiring)
	}
}
