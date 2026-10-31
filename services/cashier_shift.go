package services

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

type shiftReportJSON struct {
	SalesTotal   float64                            `json:"sales_total"`
	SalesCount   int                                `json:"sales_count"`
	OpeningCash  float64                            `json:"opening_cash"`
	CashInTotal  float64                            `json:"cash_in_total"`
	CashOutTotal float64                            `json:"cash_out_total"`
	ExpectedCash float64                            `json:"expected_cash"`
	ByMethod     map[string]models.ShiftMethodTotal `json:"by_method"`
}

// GetCashierShiftReport mengembalikan daftar shift kasir (buka/serah-terima/tutup)
// dengan analisa balance kas (selisih = kas akhir − kas seharusnya) dan ringkasannya.
func GetCashierShiftReport(outletID, status, dateFrom, dateTo string, outletScope []string) (*models.CashierShiftReport, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("s.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("s.status = $%d", idx))
		args = append(args, status)
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
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("s.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}

	q := fmt.Sprintf(`
		SELECT s.id, s.outlet_id, COALESCE(o.name, s.outlet_id), s.opened_by,
			-- Gunakan waktu kejadian dari POS; waktu penerimaan hanya cadangan.
			to_char((COALESCE(s.opened_at,s.created_at) AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'), 'YYYY-MM-DD HH24:MI'),
			COALESCE(s.closed_by, ''),
			CASE WHEN s.status='closed' THEN to_char((COALESCE(s.closed_at,s.updated_at) AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'), 'YYYY-MM-DD HH24:MI') ELSE '' END,
			s.opening_cash, COALESCE(s.closing_cash,0), COALESCE(s.closing_card,0),
			COALESCE(s.closing_qris,0), COALESCE(s.closing_transfer,0), COALESCE(s.carry_over_cash,0),
			COALESCE(s.handover_to,''), s.status, COALESCE(s.notes,''),
			COALESCE(s.report::text, '{}'), s.created_at
		FROM cloud_cashier_shifts s
		LEFT JOIN outlets o ON o.id = s.outlet_id
		WHERE %s
		ORDER BY COALESCE(s.opened_at,s.created_at) DESC`, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	report := &models.CashierShiftReport{Shifts: []models.CashierShift{}}
	shiftIDs := []string{}
	for rows.Next() {
		var sh models.CashierShift
		var reportJSON string
		if err := rows.Scan(&sh.ID, &sh.OutletID, &sh.OutletName, &sh.OpenedBy,
			&sh.OpenedAt, &sh.ClosedBy, &sh.ClosedAt,
			&sh.OpeningCash, &sh.ClosingCash, &sh.ClosingCard, &sh.ClosingQris, &sh.ClosingTransfer,
			&sh.CarryOverCash, &sh.HandoverTo, &sh.Status, &sh.Notes, &reportJSON, &sh.CreatedAt); err != nil {
			return nil, err
		}
		var rj shiftReportJSON
		json.Unmarshal([]byte(reportJSON), &rj)
		sh.ByMethod = rj.ByMethod
		if sh.ByMethod == nil {
			sh.ByMethod = map[string]models.ShiftMethodTotal{}
		}
		sh.SalesTotal = rj.SalesTotal
		sh.SalesCount = rj.SalesCount
		sh.CashSales = sh.ByMethod["cash"].Total
		sh.SalesSource = "device"
		sh.Movements = []models.CashMovement{}
		report.Shifts = append(report.Shifts, sh)
		shiftIDs = append(shiftIDs, strings.TrimSpace(sh.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fallback: report tutup kasir dikirim device SAAT tutup — shift yang masih
	// berjalan atau yang tutupnya belum tersinkron tampil Rp 0 padahal transaksinya
	// sudah ada di cloud. Hitung dari transaction_payments dalam jendela shift
	// (buka → tutup / shift berikutnya / sekarang) untuk shift yang report-nya kosong.
	needCloud := []string{}
	for i := range report.Shifts {
		if report.Shifts[i].SalesTotal == 0 {
			needCloud = append(needCloud, strings.TrimSpace(report.Shifts[i].ID))
		}
	}
	if len(needCloud) > 0 {
		type cloudAgg struct {
			byMethod map[string]models.ShiftMethodTotal
			total    float64
			count    int
		}
		aggs := map[string]*cloudAgg{}
		crows, cErr := database.DB.Query(`
			WITH win AS (
				SELECT TRIM(s.id) AS sid, s.outlet_id, COALESCE(s.opened_at,s.created_at) AS t_from,
					LEAST(
						COALESCE((SELECT MIN(COALESCE(s2.opened_at,s2.created_at)) FROM cloud_cashier_shifts s2
							WHERE s2.outlet_id = s.outlet_id AND COALESCE(s2.opened_at,s2.created_at) > COALESCE(s.opened_at,s.created_at)),
							'infinity'::timestamp),
						CASE WHEN s.status = 'closed' THEN COALESCE(s.closed_at,s.updated_at) ELSE (NOW() AT TIME ZONE 'UTC') END
					) AS t_to
				FROM cloud_cashier_shifts s WHERE TRIM(s.id) = ANY($1::text[])
			)
			SELECT w.sid, COALESCE(tp.payment_method,'other'),
				COALESCE(SUM(tp.amount),0), COUNT(DISTINCT t.id)
			FROM win w
			JOIN transaction_payments tp ON tp.outlet_id = w.outlet_id
				AND tp.created_at >= w.t_from AND tp.created_at < w.t_to
			JOIN cloud_transactions t ON t.id = tp.transaction_id AND t.outlet_id = tp.outlet_id
			GROUP BY w.sid, tp.payment_method`, pq.Array(needCloud))
		if cErr == nil {
			defer crows.Close()
			for crows.Next() {
				var sid, method string
				var amt float64
				var cnt int
				if crows.Scan(&sid, &method, &amt, &cnt) == nil {
					a := aggs[sid]
					if a == nil {
						a = &cloudAgg{byMethod: map[string]models.ShiftMethodTotal{}}
						aggs[sid] = a
					}
					a.byMethod[method] = models.ShiftMethodTotal{Count: cnt, Total: amt}
					a.total += amt
				}
			}
		}
		// Jumlah transaksi dihitung distinct terpisah — split bill (2 metode)
		// tidak boleh terhitung dua kali.
		cntRows, cntErr := database.DB.Query(`
			WITH win AS (
				SELECT TRIM(s.id) AS sid, s.outlet_id, COALESCE(s.opened_at,s.created_at) AS t_from,
					LEAST(
						COALESCE((SELECT MIN(COALESCE(s2.opened_at,s2.created_at)) FROM cloud_cashier_shifts s2
							WHERE s2.outlet_id = s.outlet_id AND COALESCE(s2.opened_at,s2.created_at) > COALESCE(s.opened_at,s.created_at)),
							'infinity'::timestamp),
						CASE WHEN s.status = 'closed' THEN COALESCE(s.closed_at,s.updated_at) ELSE (NOW() AT TIME ZONE 'UTC') END
					) AS t_to
				FROM cloud_cashier_shifts s WHERE TRIM(s.id) = ANY($1::text[])
			)
			SELECT w.sid, COUNT(DISTINCT t.id)
			FROM win w
			JOIN transaction_payments tp ON tp.outlet_id = w.outlet_id
				AND tp.created_at >= w.t_from AND tp.created_at < w.t_to
			JOIN cloud_transactions t ON t.id = tp.transaction_id AND t.outlet_id = tp.outlet_id
			GROUP BY w.sid`, pq.Array(needCloud))
		if cntErr == nil {
			defer cntRows.Close()
			for cntRows.Next() {
				var sid string
				var cnt int
				if cntRows.Scan(&sid, &cnt) == nil {
					if a := aggs[sid]; a != nil {
						a.count = cnt
					}
				}
			}
		}
		for i := range report.Shifts {
			sh := &report.Shifts[i]
			if a := aggs[strings.TrimSpace(sh.ID)]; a != nil && sh.SalesTotal == 0 && a.total > 0 {
				sh.ByMethod = a.byMethod
				sh.SalesTotal = round2(a.total)
				sh.SalesCount = a.count
				sh.CashSales = a.byMethod["cash"].Total
				sh.SalesSource = "cloud"
			}
		}
	}

	// Kas masuk (pendapatan tambahan) & kas keluar (pengeluaran) dari rincian movement.
	movByShift := map[string][]models.CashMovement{}
	if len(shiftIDs) > 0 {
		mrows, mErr := database.DB.Query(`
			SELECT TRIM(shift_id), movement_type, amount, COALESCE(counterpart_name,''), COALESCE(note,''),
				COALESCE(to_char((created_at AT TIME ZONE 'UTC') AT TIME ZONE COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta'),'YYYY-MM-DD HH24:MI'),'')
			FROM cloud_cash_movements WHERE TRIM(shift_id) = ANY($1::text[])
			ORDER BY created_at ASC`, pq.Array(shiftIDs))
		if mErr == nil {
			defer mrows.Close()
			for mrows.Next() {
				var sid string
				var m models.CashMovement
				if mrows.Scan(&sid, &m.Type, &m.Amount, &m.CounterpartName, &m.Note, &m.CreatedAt) == nil {
					movByShift[sid] = append(movByShift[sid], m)
				}
			}
		}
	}

	// Finalisasi: kas masuk/keluar dari movement, kas seharusnya, selisih, ringkasan.
	for i := range report.Shifts {
		sh := &report.Shifts[i]
		movs := movByShift[strings.TrimSpace(sh.ID)]
		sh.Movements = movs
		for _, m := range movs {
			if m.Type == "in" {
				sh.CashIn += m.Amount
			} else if m.Type == "out" {
				sh.CashOut += m.Amount
			}
		}
		// Kas seharusnya = kas awal + penjualan tunai + kas masuk − kas keluar.
		sh.ExpectedCash = round2(sh.OpeningCash + sh.CashSales + sh.CashIn - sh.CashOut)
		if sh.Status == "closed" {
			sh.Variance = round2(sh.ClosingCash - sh.ExpectedCash)
			sh.Balanced = math.Abs(sh.Variance) < 0.5
		}

		report.Summary.TotalShifts++
		report.Summary.TotalSales += sh.SalesTotal
		report.Summary.TotalCashIn += sh.CashIn
		report.Summary.TotalCashOut += sh.CashOut
		if sh.Status == "closed" {
			report.Summary.ClosedShifts++
			report.Summary.TotalVariance = round2(report.Summary.TotalVariance + sh.Variance)
			if sh.Balanced {
				report.Summary.BalancedCount++
			} else {
				report.Summary.MissCount++
				if sh.Variance < 0 {
					report.Summary.ShortageTotal = round2(report.Summary.ShortageTotal + sh.Variance)
				} else {
					report.Summary.OverageTotal = round2(report.Summary.OverageTotal + sh.Variance)
				}
			}
		} else {
			report.Summary.OpenShifts++
		}
	}
	return report, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
