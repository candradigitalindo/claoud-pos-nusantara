package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// reconShiftJSON — bagian report shift yang dipakai rekonsiliasi.
type reconShiftJSON struct {
	SalesTotal float64 `json:"sales_total"`
	SalesCount int     `json:"sales_count"`
	ByCashier  []struct {
		Cashier string `json:"cashier"`
	} `json:"by_cashier"`
}

type reconQueryer interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func parseReconReport(raw string) (reconShiftJSON, error) {
	var r reconShiftJSON
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return r, fmt.Errorf("laporan tutup kasir tidak valid: %w", err)
	}
	sales, ok := fields["sales_total"]
	if !ok || string(sales) == "null" {
		return r, fmt.Errorf("laporan tutup kasir belum memiliki sales_total")
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, fmt.Errorf("laporan tutup kasir tidak valid: %w", err)
	}
	if math.IsNaN(r.SalesTotal) || math.IsInf(r.SalesTotal, 0) || r.SalesTotal < 0 {
		return r, fmt.Errorf("total penjualan kasir tidak valid")
	}
	return r, nil
}

// Nominal mengikuti waktu tiap pembayaran, termasuk DP pada shift sebelumnya.
// Header lama tanpa rincian pembayaran memakai total dan waktu transaksi sebagai
// cadangan. Jumlah transaksi, seperti laporan POS, mengikuti waktu pelunasan
// (termasuk transaksi gratis), bukan jumlah pembayaran. Akhir jendela eksklusif.
func cloudSalesInWindow(q reconQueryer, outletID string, buka, tutup time.Time) (float64, int, error) {
	if tutup.Before(buka) {
		return 0, 0, fmt.Errorf("waktu tutup shift mendahului waktu buka")
	}
	var total float64
	var count int
	err := q.QueryRow(`
		WITH sales AS (
			SELECT tp.amount
			FROM transaction_payments tp
			JOIN cloud_transactions t ON t.id = tp.transaction_id AND t.outlet_id = tp.outlet_id
			WHERE tp.outlet_id = $1 AND tp.created_at >= $2 AND tp.created_at < $3
			  AND lower(btrim(COALESCE(t.payment_method,''))) NOT IN ('compliment','adjustment')
			  AND lower(btrim(tp.payment_method)) NOT IN ('compliment','adjustment')
			UNION ALL
			SELECT t.total_amount
			FROM cloud_transactions t
			WHERE t.outlet_id = $1 AND t.created_at >= $2 AND t.created_at < $3
			  AND lower(btrim(COALESCE(t.payment_method,''))) NOT IN ('compliment','adjustment')
			  AND NOT EXISTS (SELECT 1 FROM transaction_payments tp
				WHERE tp.transaction_id = t.id AND tp.outlet_id = t.outlet_id)
		)
		SELECT COALESCE(SUM(amount),0), (
			SELECT COUNT(*) FROM cloud_transactions t
			WHERE t.outlet_id = $1 AND t.created_at >= $2 AND t.created_at < $3
			  AND lower(btrim(COALESCE(t.payment_method,''))) <> 'adjustment'
		) FROM sales`, strings.TrimSpace(outletID), buka, tutup,
	).Scan(&total, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal menghitung penjualan cloud: %w", err)
	}
	return round2(total), count, nil
}

// GetShiftReconciliation membandingkan penjualan versi kasir (report shift) dengan
// yang masuk cloud, untuk tiap shift TERTUTUP pada rentang tanggal (scoped).
func GetShiftReconciliation(dateFrom, dateTo, outletID string, outletScope []string) (*models.ShiftReconReport, error) {
	conds := []string{"s.status = 'closed'"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("s.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("s.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("COALESCE(s.opened_at,s.created_at) >= tz_day_start($%d::date)", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("COALESCE(s.opened_at,s.created_at) < tz_day_start($%d::date + 1)", idx))
		args = append(args, dateTo)
		idx++
	}

	q := fmt.Sprintf(`
		SELECT TRIM(s.id), TRIM(s.outlet_id), COALESCE(o.name, s.outlet_id),
			to_char((COALESCE(s.opened_at,s.created_at) AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
			to_char((COALESCE(s.closed_at,s.updated_at) AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
			COALESCE(s.opened_at,s.created_at), COALESCE(s.closed_at,s.updated_at),
			COALESCE(s.report::text, '{}')
		FROM cloud_cashier_shifts s
		LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE %[2]s
		ORDER BY COALESCE(s.opened_at,s.created_at) DESC`, tzExpr, strings.Join(conds, " AND "))

	// Semua angka dan penyesuaian berasal dari snapshot yang sama, walaupun
	// sinkronisasi baru masuk di tengah proses penyusunan laporan.
	tx, err := database.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &models.ShiftReconReport{Shifts: []models.ShiftReconRow{}}
	type pending struct {
		row         models.ShiftReconRow
		buka, tutup time.Time
	}
	var list []pending
	shiftIDs := []string{}
	for rows.Next() {
		var r models.ShiftReconRow
		var buka, tutup time.Time
		var reportJSON string
		if err := rows.Scan(&r.ShiftID, &r.OutletID, &r.OutletName,
			&r.OpenedAt, &r.ClosedAt, &buka, &tutup, &reportJSON); err != nil {
			return nil, err
		}
		rj, err := parseReconReport(reportJSON)
		if err != nil {
			return nil, fmt.Errorf("shift %s: %w", r.ShiftID, err)
		}
		r.CashierSales = round2(rj.SalesTotal)
		r.CashierCount = rj.SalesCount
		names := make([]string, 0, len(rj.ByCashier))
		for _, c := range rj.ByCashier {
			if strings.TrimSpace(c.Cashier) != "" {
				names = append(names, c.Cashier)
			}
		}
		r.Cashiers = strings.Join(names, ", ")
		list = append(list, pending{r, buka, tutup})
		shiftIDs = append(shiftIDs, r.ShiftID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Penyesuaian aktif per shift.
	adjByShift := map[string]struct {
		id     int64
		amount float64
	}{}
	if len(shiftIDs) > 0 {
		arows, aerr := tx.Query(`
			SELECT TRIM(shift_id), id, adjustment_amount
			FROM shift_revenue_adjustments
			WHERE status='applied' AND TRIM(shift_id) = ANY($1::text[])`, pq.Array(shiftIDs))
		if aerr != nil {
			return nil, aerr
		}
		defer arows.Close()
		for arows.Next() {
			var sid string
			var id int64
			var amt float64
			if err := arows.Scan(&sid, &id, &amt); err != nil {
				return nil, err
			}
			adjByShift[sid] = struct {
				id     int64
				amount float64
			}{id, amt}
		}
		if err := arows.Err(); err != nil {
			return nil, err
		}
		arows.Close()
	}

	for _, p := range list {
		r := p.row
		r.CloudSales, r.CloudCount, err = cloudSalesInWindow(tx, r.OutletID, p.buka, p.tutup)
		if err != nil {
			return nil, fmt.Errorf("shift %s: %w", r.ShiftID, err)
		}
		r.Diff = round2(r.CashierSales - r.CloudSales)
		if a, ok := adjByShift[r.ShiftID]; ok {
			r.Adjusted = true
			r.AdjustmentID = a.id
			r.AdjustmentAmount = a.amount
		}
		switch {
		case math.Abs(r.Diff) < 1:
			r.Status = "balanced"
			report.Summary.BalancedCount++
		case r.Diff > 0:
			r.Status = "short"
			report.Summary.ShortCount++
			report.Summary.ShortTotal = round2(report.Summary.ShortTotal + r.Diff)
		default:
			r.Status = "over"
			report.Summary.OverCount++
			report.Summary.OverTotal = round2(report.Summary.OverTotal + r.Diff)
		}
		if r.Adjusted {
			report.Summary.AdjustedCount++
			report.Summary.AdjustedTotal = round2(report.Summary.AdjustedTotal + r.AdjustmentAmount)
		}
		report.Summary.TotalShifts++
		report.Shifts = append(report.Shifts, r)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

// ApplyShiftAdjustment menyamakan pendapatan cloud dengan versi kasir untuk satu
// shift: menambahkan satu transaksi penyesuaian (payment_method='adjustment')
// sebesar selisih, dan mencatatnya di shift_revenue_adjustments agar bisa dibatalkan.
func ApplyShiftAdjustment(shiftID, adminName string, outletScope []string) error {
	shiftID = strings.TrimSpace(shiftID)
	var outletID string
	if err := database.DB.QueryRow(`SELECT TRIM(outlet_id) FROM cloud_cashier_shifts WHERE id=$1`, shiftID).Scan(&outletID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("shift tidak ditemukan")
		}
		return err
	}
	if !reconOutletAllowed(outletID, outletScope) {
		return fmt.Errorf("akses outlet tidak diizinkan")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Sinkronisasi pembayaran, void, apply dan revert harus berurutan per
	// outlet; hitung selisih setelah memegang lock yang sama dengan POS.
	if err := lockPOSStock(tx, outletID); err != nil {
		return err
	}

	var outletCode, reportJSON, status string
	var buka, tutup time.Time
	err = tx.QueryRow(`
		SELECT TRIM(s.outlet_id), COALESCE(o.code,''), COALESCE(s.report::text,'{}'), s.status,
			COALESCE(s.opened_at,s.created_at), COALESCE(s.closed_at,s.updated_at)
		FROM cloud_cashier_shifts s LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE s.id = $1 FOR UPDATE OF s`, shiftID).
		Scan(&outletID, &outletCode, &reportJSON, &status, &buka, &tutup)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("shift tidak ditemukan")
		}
		return err
	}
	if status != "closed" {
		return fmt.Errorf("hanya shift tertutup yang bisa disesuaikan")
	}
	if !reconOutletAllowed(outletID, outletScope) {
		return fmt.Errorf("akses outlet tidak diizinkan")
	}

	rj, err := parseReconReport(reportJSON)
	if err != nil {
		return err
	}
	cloudSales, _, err := cloudSalesInWindow(tx, outletID, buka, tutup)
	if err != nil {
		return err
	}
	diff := round2(rj.SalesTotal - cloudSales)
	if diff < 1 {
		return fmt.Errorf("tidak ada kekurangan untuk disesuaikan (cloud sudah ≥ versi kasir)")
	}

	// Cegah dobel penyesuaian aktif.
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied')`, shiftID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("shift ini sudah punya penyesuaian aktif")
	}

	// Transaksi penyesuaian: dicatat pada hari (buka) shift agar dashboard-per-hari
	// mengikuti versi kasir. Tanpa order & tanpa transaction_payments, sehingga tak
	// mengotori kartu Metode Pembayaran.
	txnID := NewULID()
	localID := "ADJ-" + shiftID
	if _, err := tx.Exec(`
		INSERT INTO cloud_transactions (id, local_id, outlet_id, outlet_code, order_id,
			subtotal, total_amount, tax_amount, payment_method, cash_amount, change_amount,
			cashier_name, items, created_at, synced_at)
		VALUES ($1,$2,$3,$4,'',$5,$5,0,'adjustment',0,0,$6,'[]',$7, NOW())`,
		txnID, localID, outletID, outletCode, diff, "Penyesuaian (versi kasir)", buka,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO shift_revenue_adjustments
			(outlet_id, shift_id, shift_date, cashier_sales, cloud_sales, adjustment_amount,
			 txn_id, txn_local_id, note, status, created_by)
		VALUES ($1,$2, tz_date($3), $4,$5,$6, $7,$8,$9,'applied',$10)`,
		outletID, shiftID, buka, round2(rj.SalesTotal), cloudSales, diff,
		txnID, localID,
		fmt.Sprintf("Ikuti versi kasir: +%s (kasir %s vs cloud %s)", fmtRp(diff), fmtRp(rj.SalesTotal), fmtRp(cloudSales)),
		adminName,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// RevertShiftAdjustment membatalkan penyesuaian: menghapus transaksi penyesuaian
// dan menandai catatan sebagai reverted (nilai kembali ke data asli cloud).
func RevertShiftAdjustment(adjID int64, adminName string, outletScope []string) error {
	var outletID string
	err := database.DB.QueryRow(`SELECT TRIM(outlet_id) FROM shift_revenue_adjustments WHERE id=$1`, adjID).Scan(&outletID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("penyesuaian tidak ditemukan")
		}
		return err
	}
	if !reconOutletAllowed(outletID, outletScope) {
		return fmt.Errorf("akses outlet tidak diizinkan")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockPOSStock(tx, outletID); err != nil {
		return err
	}
	var txnID, localID, status string
	err = tx.QueryRow(`SELECT TRIM(outlet_id), COALESCE(TRIM(txn_id),''), COALESCE(txn_local_id,''), status
		FROM shift_revenue_adjustments WHERE id=$1 FOR UPDATE`, adjID).Scan(&outletID, &txnID, &localID, &status)
	if err != nil {
		return err
	}
	if !reconOutletAllowed(outletID, outletScope) {
		return fmt.Errorf("akses outlet tidak diizinkan")
	}
	if status != "applied" {
		return fmt.Errorf("penyesuaian sudah dibatalkan")
	}
	if err := clearReconPayments(tx, outletID, txnID, localID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM cloud_transactions
		WHERE outlet_id=$1 AND (id=$2 OR local_id=$3)
		AND lower(btrim(payment_method))='adjustment'`, outletID, txnID, localID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE shift_revenue_adjustments SET status='reverted', reverted_by=$2, reverted_at=now() AT TIME ZONE 'UTC' WHERE id=$1`, adjID, adminName); err != nil {
		return err
	}
	return tx.Commit()
}

func reconOutletAllowed(outletID string, scope []string) bool {
	if scope == nil {
		return true
	}
	for _, id := range scope {
		if id == outletID {
			return true
		}
	}
	return false
}

// Migrasi lama dapat membuat rincian pembayaran untuk transaksi penyesuaian.
// Bersihkan hanya rincian milik transaksi sintetis yang bersangkutan.
func clearReconPayments(tx *sql.Tx, outletID, txnID, localID string) error {
	_, err := tx.Exec(`DELETE FROM transaction_payments
		WHERE outlet_id=$1 AND transaction_id IN (
			SELECT id FROM cloud_transactions WHERE outlet_id=$1
			AND (id=$2 OR local_id=$3) AND lower(btrim(payment_method))='adjustment'
		)`, outletID, txnID, localID)
	return err
}

// Dipanggil di transaksi yang sama dengan pembayaran/void dan dengan lock POS
// outlet dipegang. Penyesuaian hanya berkurang ketika data asli menyusul; void
// atau koreksi berikutnya tidak boleh menciptakan pendapatan sintetis baru.
// cashier_sales/cloud_sales tetap menyimpan angka saat persetujuan awal.
func refreshShiftAdjustments(tx *sql.Tx, outletID string) error {
	rows, err := tx.Query(`SELECT a.id, TRIM(a.shift_id), COALESCE(TRIM(a.txn_id),''),
		COALESCE(a.txn_local_id,''), a.cashier_sales, a.adjustment_amount,
		COALESCE(s.opened_at,s.created_at), COALESCE(s.closed_at,s.updated_at)
		FROM shift_revenue_adjustments a
		JOIN cloud_cashier_shifts s ON s.id=a.shift_id AND s.outlet_id=a.outlet_id
		WHERE a.outlet_id=$1 AND a.status='applied' AND a.adjustment_amount>0
		ORDER BY a.id FOR UPDATE OF a`, outletID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type adjustment struct {
		id                           int64
		shiftID, txnID, localID      string
		cashierSales, previousAmount float64
		buka, tutup                  time.Time
	}
	var adjustments []adjustment
	for rows.Next() {
		var a adjustment
		if err := rows.Scan(&a.id, &a.shiftID, &a.txnID, &a.localID,
			&a.cashierSales, &a.previousAmount, &a.buka, &a.tutup); err != nil {
			return err
		}
		adjustments = append(adjustments, a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, a := range adjustments {
		cloudSales, _, err := cloudSalesInWindow(tx, outletID, a.buka, a.tutup)
		if err != nil {
			return fmt.Errorf("penyesuaian shift %s: %w", a.shiftID, err)
		}
		amount := round2(math.Min(a.previousAmount, math.Max(0, a.cashierSales-cloudSales)))
		if amount == a.previousAmount {
			continue
		}
		if err := clearReconPayments(tx, outletID, a.txnID, a.localID); err != nil {
			return err
		}
		if amount == 0 {
			if _, err := tx.Exec(`DELETE FROM cloud_transactions WHERE outlet_id=$1
				AND (id=$2 OR local_id=$3) AND lower(btrim(payment_method))='adjustment'`, outletID, a.txnID, a.localID); err != nil {
				return err
			}
		} else {
			res, err := tx.Exec(`UPDATE cloud_transactions SET total_amount=$4, subtotal=$4, synced_at=NOW()
				WHERE outlet_id=$1 AND (id=$2 OR local_id=$3)
				AND lower(btrim(payment_method))='adjustment'`, outletID, a.txnID, a.localID, amount)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("transaksi penyesuaian shift %s tidak ditemukan atau tidak unik", a.shiftID)
			}
		}
		if _, err := tx.Exec(`UPDATE shift_revenue_adjustments SET adjustment_amount=$2 WHERE id=$1`, a.id, amount); err != nil {
			return err
		}
	}
	return nil
}

// fmtRp — format rupiah ringkas untuk catatan.
func fmtRp(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	// sisipkan pemisah ribuan
	n := len(s)
	if n <= 3 {
		return "Rp" + s
	}
	var b strings.Builder
	b.WriteString("Rp")
	pre := n % 3
	if pre > 0 {
		b.WriteString(s[:pre])
		if n > pre {
			b.WriteString(".")
		}
	}
	for i := pre; i < n; i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < n {
			b.WriteString(".")
		}
	}
	return b.String()
}
