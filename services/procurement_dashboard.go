package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"log"

	"github.com/lib/pq"
)

// GetProcurementDashboard returns procurement statistics.
// If outletID is empty, returns global stats (filtered by scope if applicable).
func GetProcurementDashboard(outletID string, scopeIDs []string, wuScopeIDs []string) (*models.ProcurementDashboardResponse, error) {
	resp := &models.ProcurementDashboardResponse{
		WorkUnits:    make([]models.ProcurementWorkUnitStat, 0),
		MonthlyTrend: make([]models.ProcurementMonthlyTrend, 0),
	}

	// Build scope WHERE clause
	var scopeClause, scopeClauseJoin string
	var args []interface{}
	idx := 1

	if outletID != "" {
		scopeClause = fmt.Sprintf("WHERE outlet_id = $%d", idx)
		scopeClauseJoin = fmt.Sprintf("WHERE pr.outlet_id = $%d", idx)
		args = append(args, outletID)
		idx++
	} else if scopeIDs != nil || wuScopeIDs != nil {
		// Scoped user without specific outlet filter
		conditions := []string{}
		if scopeIDs != nil && len(scopeIDs) > 0 {
			conditions = append(conditions, fmt.Sprintf("outlet_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(scopeIDs))
			idx++
		}
		if wuScopeIDs != nil && len(wuScopeIDs) > 0 {
			conditions = append(conditions, fmt.Sprintf("work_unit_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(wuScopeIDs))
			idx++
		}
		if len(conditions) > 0 {
			combined := conditions[0]
			for _, c := range conditions[1:] {
				combined += " OR " + c
			}
			scopeClause = "WHERE (" + combined + ")"
			// rebuild with pr. prefix for JOIN queries, using same arg indices
			conditionsJoin := []string{}
			argIdx := 1
			if scopeIDs != nil && len(scopeIDs) > 0 {
				conditionsJoin = append(conditionsJoin, fmt.Sprintf("pr.outlet_id = ANY($%d::text[])", argIdx))
				argIdx++
			}
			if wuScopeIDs != nil && len(wuScopeIDs) > 0 {
				conditionsJoin = append(conditionsJoin, fmt.Sprintf("pr.work_unit_id = ANY($%d::text[])", argIdx))
			}
			if len(conditionsJoin) > 0 {
				combinedJoin := conditionsJoin[0]
				for _, c := range conditionsJoin[1:] {
					combinedJoin += " OR " + c
				}
				scopeClauseJoin = "WHERE (" + combinedJoin + ")"
			}
		} else {
			// Scoped but no outlets/work units — return empty
			return resp, nil
		}
	}
	// else: no scope (all access) — no WHERE clause

	// Master yang seluruh itemnya sudah dipecah tinggal cangkang kosong. Dulu
	// hanya blok Hutang Usaha yang menyaringnya, sehingga ringkasan, split tipe,
	// per unit kerja, dan tren menghitung master bernilai penuh sekaligus semua
	// pecahannya — setiap pengadaan yang di-split terhitung dua kali.
	excludeShell := "NOT " + fullySplitMasterCond("")
	excludeShellJoin := "NOT " + fullySplitMasterCond("pr")
	andWhere := func(clause, extra string) string {
		if clause == "" {
			return "WHERE " + extra
		}
		return clause + " AND " + extra
	}

	// ── 1) Summary counts & amounts by status ──
	summaryQ := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(total_final), 0) AS total_amount,
			COUNT(*) FILTER (WHERE status = 'pending')           AS pending,
			COUNT(*) FILTER (WHERE status = 'approved')          AS approved,
			COUNT(*) FILTER (WHERE status = 'payment_requested') AS payment_requested,
			COUNT(*) FILTER (WHERE status = 'paid')              AS paid,
			COUNT(*) FILTER (WHERE status = 'received')          AS received,
			COUNT(*) FILTER (WHERE status = 'rejected')          AS rejected,
			COUNT(*) FILTER (WHERE status = 'cancelled')         AS cancelled
		FROM purchase_requests
		%s`, andWhere(scopeClause, excludeShell))

	err := database.DB.QueryRow(summaryQ, args...).Scan(
		&resp.TotalRequests, &resp.TotalAmount,
		&resp.StatusBreakdown.Pending, &resp.StatusBreakdown.Approved,
		&resp.StatusBreakdown.PaymentRequested,
		&resp.StatusBreakdown.Paid, &resp.StatusBreakdown.Received,
		&resp.StatusBreakdown.Rejected, &resp.StatusBreakdown.Cancelled,
	)
	if err != nil {
		return nil, fmt.Errorf("procurement dashboard summary: %w", err)
	}

	// ── 2) Type split (barang vs jasa) ──
	typeQ := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE request_type = 'barang') AS barang,
			COALESCE(SUM(total_final) FILTER (WHERE request_type = 'barang'), 0) AS barang_total,
			COUNT(*) FILTER (WHERE request_type = 'jasa') AS jasa,
			COALESCE(SUM(total_final) FILTER (WHERE request_type = 'jasa'), 0) AS jasa_total
		FROM purchase_requests
		%s`, andWhere(scopeClause, excludeShell))

	err = database.DB.QueryRow(typeQ, args...).Scan(
		&resp.TypeSplit.Barang, &resp.TypeSplit.BarangTotal,
		&resp.TypeSplit.Jasa, &resp.TypeSplit.JasaTotal,
	)
	if err != nil {
		return nil, fmt.Errorf("procurement dashboard type split: %w", err)
	}

	// ── 3) Per work-unit stats ──
	wuQ := fmt.Sprintf(`
		SELECT
			COALESCE(pr.work_unit_id, ''),
			COALESCE(wu.name, 'Tanpa Unit Kerja'),
			COUNT(*) AS total,
			COALESCE(SUM(pr.total_final), 0) AS total_amount,
			COUNT(*) FILTER (WHERE pr.status = 'pending')           AS pending,
			COUNT(*) FILTER (WHERE pr.status = 'approved')          AS approved,
			COUNT(*) FILTER (WHERE pr.status = 'payment_requested') AS payment_requested,
			COUNT(*) FILTER (WHERE pr.status = 'paid')              AS paid,
			COUNT(*) FILTER (WHERE pr.status = 'received')          AS received,
			COUNT(*) FILTER (WHERE pr.status = 'rejected')          AS rejected,
			COUNT(*) FILTER (WHERE pr.status = 'cancelled')         AS cancelled
		FROM purchase_requests pr
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		%s
		GROUP BY pr.work_unit_id, wu.name
		ORDER BY total_amount DESC`, andWhere(scopeClauseJoin, excludeShellJoin))

	rows, err := database.DB.Query(wuQ, args...)
	if err != nil {
		return nil, fmt.Errorf("procurement dashboard work units: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s models.ProcurementWorkUnitStat
		if err := rows.Scan(
			&s.WorkUnitID, &s.WorkUnitName,
			&s.Total, &s.TotalAmount,
			&s.Pending, &s.Approved, &s.PaymentRequested, &s.Paid, &s.Received,
			&s.Rejected, &s.Cancelled,
		); err != nil {
			log.Printf("procurement dashboard scan work unit: %v", err)
			continue
		}
		resp.WorkUnits = append(resp.WorkUnits, s)
	}

	// ── 4) Accounts Payable (Hutang Usaha) ──
	// Sisa kewajiban dihitung dari (total_final - paid_amount) untuk semua
	// status yang masih berjalan, bukan dari daftar nama status. Dengan begitu
	// pengajuan yang sudah diterima tapi belum lunas tetap tercatat sebagai
	// hutang, bukan menghilang begitu statusnya berubah jadi 'received'.
	apWhere := andWhere(scopeClause, excludeShell+" AND "+outstandingCond)
	apQ := fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(total_final - paid_amount), 0)
		FROM purchase_requests %s`, apWhere)
	err = database.DB.QueryRow(apQ, args...).Scan(&resp.AccountsPayable.Count, &resp.AccountsPayable.TotalAmount)
	if err != nil {
		return nil, fmt.Errorf("procurement dashboard accounts payable: %w", err)
	}

	// ── 5) Monthly trend (last 12 months) ──
	trendExtra := "created_at >= DATE_TRUNC('month', NOW()) - INTERVAL '11 months'"
	trendWhere := andWhere(scopeClause, excludeShell+" AND "+trendExtra)
	trendQ := fmt.Sprintf(`
		SELECT
			TO_CHAR(created_at, 'YYYY-MM') AS month,
			COUNT(*) AS count,
			COALESCE(SUM(total_final), 0) AS total_amount
		FROM purchase_requests
		%s
		GROUP BY TO_CHAR(created_at, 'YYYY-MM')
		ORDER BY month`, trendWhere)

	trendRows, err := database.DB.Query(trendQ, args...)
	if err != nil {
		return nil, fmt.Errorf("procurement dashboard trend: %w", err)
	}
	defer trendRows.Close()

	for trendRows.Next() {
		var t models.ProcurementMonthlyTrend
		if err := trendRows.Scan(&t.Month, &t.Count, &t.TotalAmount); err != nil {
			log.Printf("procurement dashboard scan trend: %v", err)
			continue
		}
		resp.MonthlyTrend = append(resp.MonthlyTrend, t)
	}

	return resp, nil
}

// GetPaymentStats returns aggregated stats for the procurement payments page.
func GetPaymentStats(scopeIDs []string, wuScopeIDs []string) (*models.PaymentStatsResponse, error) {
	resp := &models.PaymentStatsResponse{}

	var scopeClause string
	var args []interface{}
	idx := 1

	if scopeIDs != nil || wuScopeIDs != nil {
		conditions := []string{}
		if scopeIDs != nil && len(scopeIDs) > 0 {
			conditions = append(conditions, fmt.Sprintf("outlet_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(scopeIDs))
			idx++
		}
		if wuScopeIDs != nil && len(wuScopeIDs) > 0 {
			conditions = append(conditions, fmt.Sprintf("work_unit_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(wuScopeIDs))
			idx++
		}
		if len(conditions) > 0 {
			combined := conditions[0]
			for _, c := range conditions[1:] {
				combined += " OR " + c
			}
			scopeClause = "WHERE (" + combined + ") AND "
		} else {
			return resp, nil
		}
	} else {
		scopeClause = "WHERE "
	}

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'payment_requested') AS waiting,
			COALESCE(SUM(total_final) FILTER (WHERE status = 'payment_requested'), 0) AS total_waiting,
			COUNT(*) FILTER (WHERE status IN ('paid', 'partial')) AS paid,
			-- uang yang benar-benar keluar; menjumlah total_final untuk baris
			-- 'partial' akan melaporkan tagihan penuh sebagai sudah dibayar.
			COALESCE(SUM(paid_amount), 0) AS total_paid,
			COUNT(*) FILTER (WHERE status = 'received') AS received,
			COUNT(*) FILTER (WHERE `+outstandingCond+`) AS ap_count,
			COALESCE(SUM(total_final - paid_amount) FILTER (WHERE `+outstandingCond+`), 0) AS ap_total
		FROM purchase_requests
		%s NOT `+fullySplitMasterCond("")+`
	`, scopeClause)

	var apCount int
	var apTotal float64
	err := database.DB.QueryRow(q, args...).Scan(
		&resp.Waiting, &resp.TotalWaiting,
		&resp.Paid, &resp.TotalPaid,
		&resp.Received,
		&apCount, &apTotal,
	)
	if err != nil {
		return nil, fmt.Errorf("payment stats: %w", err)
	}
	resp.AccountsPayable = models.AccountsPayable{Count: apCount, TotalAmount: apTotal}

	return resp, nil
}
