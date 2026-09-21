package services

import (
	"cloud-pos/database"
	"log"
	"time"
)

// StartPpicScheduler menjalankan evaluasi alert PPIC harian (sekitar pukul 02:00
// zona waktu aplikasi): menghitung batch kedaluwarsa & item di bawah ROP, lalu
// broadcast SSE "ppic_alert" supaya dashboard yang terbuka me-refresh. Evaluasi
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
				runNightlyForecastJob() // isi aktual kemarin + generate horizon 7 hari
				evaluatePpicAlerts()
			}
		}
	}()
}

func evaluatePpicAlerts() {
	var expired, expiring, belowRop int
	database.DB.QueryRow(`
		SELECT
			COUNT(CASE WHEN sb.expiry_date < CURRENT_DATE THEN 1 END),
			COUNT(CASE WHEN sb.expiry_date >= CURRENT_DATE AND sb.expiry_date < CURRENT_DATE + 3 THEN 1 END)
		FROM stock_batches sb
		JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL
		AND sb.ppic_ack_at IS NULL AND w.is_active = true`).Scan(&expired, &expiring)
	database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock) > 0
		AND sl.qty_base <= COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock)
		AND w.is_active = true`).Scan(&belowRop)

	log.Printf("[PPIC] Evaluasi alert: %d batch expired, %d batch ≤3 hari, %d item di bawah ROP", expired, expiring, belowRop)
	if expired > 0 || expiring > 0 || belowRop > 0 {
		BroadcastSync("ppic_alert", "")
		go NotifyPpicAlert(expired, expiring, belowRop)
	}
}
