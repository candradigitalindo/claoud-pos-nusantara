package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// txNotVoided mengembalikan kondisi AND yang mengecualikan transaksi yang order-nya
// sudah di-void (agar tidak terhitung sebagai penjualan). ref = referensi tabel
// cloud_transactions di query (mis. "cloud_transactions" atau "t").
func txNotVoided(ref string) string {
	return ` AND NOT EXISTS (SELECT 1 FROM cloud_orders vo WHERE vo.id = ` + ref +
		`.order_id AND NULLIF(vo.payment_info->>'voided_at','') IS NOT NULL)`
}

// payNotVoided mengecualikan baris transaction_payments yang transaksinya ter-void.
const payNotVoided = ` AND NOT EXISTS (SELECT 1 FROM cloud_transactions vt JOIN cloud_orders vo ON vo.id = vt.order_id WHERE vt.id = transaction_payments.transaction_id AND NULLIF(vo.payment_info->>'voided_at','') IS NOT NULL)`

// paymentMethodTotals menjumlahkan nominal per metode pembayaran dari
// transaction_payments untuk rentang tanggal & scope outlet tertentu. Inilah
// sumber rekap per-metode (bukan kolom payment_method header yang bisa 'mixed').
func paymentMethodTotals(dateFrom, dateTo string, filterIDs []string) (map[string]float64, error) {
	q := `SELECT payment_method, COALESCE(SUM(amount), 0)
		FROM transaction_payments
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + payNotVoided
	args := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		q += ` AND outlet_id = ANY($3::text[])`
		args = append(args, pq.Array(filterIDs))
	}
	q += ` GROUP BY payment_method`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64)
	for rows.Next() {
		var method string
		var amt float64
		if err := rows.Scan(&method, &amt); err != nil {
			return nil, err
		}
		out[method] = amt
	}
	return out, rows.Err()
}

func GetSalesReport(dateFrom, dateTo, outletID string, scopeIDs []string, page, limit int) (*models.SalesReportResponse, error) {
	report := &models.SalesReportResponse{
		Page:  page,
		Limit: limit,
	}

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	// Count/total/avg di level transaksi (header).
	summaryQuery := `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(total_amount), 0),
			COALESCE(AVG(total_amount), 0)
		FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + txNotVoided("cloud_transactions")

	args := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		summaryQuery += ` AND outlet_id = ANY($3::text[])`
		args = append(args, pq.Array(filterIDs))
	}

	err := database.DB.QueryRow(summaryQuery, args...).Scan(
		&report.Summary.TotalTransactions,
		&report.Summary.TotalRevenue,
		&report.Summary.AvgPerTransaction,
	)
	if err != nil {
		return nil, fmt.Errorf("sales report summary query failed: %w", err)
	}

	// Rincian per metode dari transaction_payments (header bisa bernilai 'mixed').
	if methodTotals, mErr := paymentMethodTotals(dateFrom, dateTo, filterIDs); mErr != nil {
		log.Printf("Sales report payment-method query warning: %v", mErr)
	} else {
		report.Summary.CashRevenue = methodTotals["cash"]
		report.Summary.QrisRevenue = methodTotals["qris"]
		report.Summary.CardRevenue = methodTotals["card"]
		report.Summary.TransferRevenue = methodTotals["transfer"]
	}

	unpaidQuery := `
		SELECT COUNT(*)::int, COALESCE(SUM(total_amount), 0)
		FROM cloud_orders
		WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid')
		  AND NULLIF(payment_info->>'voided_at','') IS NULL AND COALESCE(is_holding,false) = false
		  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`
	unpaidArgs := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		unpaidQuery += ` AND outlet_id = ANY($3::text[])`
		unpaidArgs = append(unpaidArgs, pq.Array(filterIDs))
	}
	if err := database.DB.QueryRow(unpaidQuery, unpaidArgs...).Scan(
		&report.Summary.UnpaidOrders,
		&report.Summary.UnpaidAmount,
	); err != nil {
		log.Printf("Sales report unpaid query warning: %v", err)
	}

	// Total transaksi, omzet, & jumlah tamu (pax) per hari. Pax di-resolve dari
	// order tertaut (cloud_transactions.order_id → cloud_orders.pax).
	dailyQuery := `
		SELECT
			TO_CHAR(tz_date(created_at), 'YYYY-MM-DD') AS date,
			COUNT(*)::int,
			COALESCE(SUM(total_amount), 0),
			COALESCE(SUM(COALESCE((SELECT o2.pax FROM cloud_orders o2 WHERE o2.id = cloud_transactions.order_id LIMIT 1), 0)), 0)::int
		FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + txNotVoided("cloud_transactions")

	dailyArgs := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		dailyQuery += ` AND outlet_id = ANY($3::text[])`
		dailyArgs = append(dailyArgs, pq.Array(filterIDs))
	}
	dailyQuery += ` GROUP BY tz_date(created_at) ORDER BY tz_date(created_at) DESC`

	dailyRows, err := database.DB.Query(dailyQuery, dailyArgs...)
	if err != nil {
		return nil, fmt.Errorf("sales report daily query failed: %w", err)
	}
	defer dailyRows.Close()

	report.Daily = []models.SalesReportRow{}
	idxByDate := map[string]int{}
	for dailyRows.Next() {
		var row models.SalesReportRow
		if err := dailyRows.Scan(&row.Date, &row.TotalTransactions, &row.TotalRevenue, &row.TotalPax); err != nil {
			return nil, fmt.Errorf("sales report daily scan failed: %w", err)
		}
		idxByDate[row.Date] = len(report.Daily)
		report.Daily = append(report.Daily, row)
	}

	// Rincian per metode per hari dari transaction_payments (header bisa 'mixed').
	methodDailyQuery := `
		SELECT TO_CHAR(tz_date(created_at), 'YYYY-MM-DD') AS date, payment_method, COALESCE(SUM(amount), 0)
		FROM transaction_payments
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + payNotVoided
	if filterIDs != nil {
		methodDailyQuery += ` AND outlet_id = ANY($3::text[])`
	}
	methodDailyQuery += ` GROUP BY tz_date(created_at), payment_method`
	if mRows, mErr := database.DB.Query(methodDailyQuery, dailyArgs...); mErr != nil {
		log.Printf("Sales report daily payment-method query warning: %v", mErr)
	} else {
		defer mRows.Close()
		for mRows.Next() {
			var date, method string
			var amt float64
			if err := mRows.Scan(&date, &method, &amt); err != nil {
				continue
			}
			i, ok := idxByDate[date]
			if !ok {
				continue
			}
			switch method {
			case "cash":
				report.Daily[i].CashRevenue = amt
			case "qris":
				report.Daily[i].QrisRevenue = amt
			case "card":
				report.Daily[i].CardRevenue = amt
			case "transfer":
				report.Daily[i].TransferRevenue = amt
			}
		}
	}

	outletQuery := `
		SELECT
			t.outlet_id,
			COALESCE(o.name, t.outlet_code),
			COUNT(*)::int,
			COALESCE(SUM(t.total_amount), 0),
			COALESCE(uq.cnt, 0)::int,
			COALESCE(uq.amt, 0)
		FROM cloud_transactions t
		LEFT JOIN outlets o ON o.id = t.outlet_id
		LEFT JOIN (
			SELECT outlet_id, COUNT(*) AS cnt, SUM(total_amount) AS amt
			FROM cloud_orders
			-- Belum bayar dilihat dari payment_info->>'payment_status' (bukan kolom
			-- status yang menyimpan status dapur: cooking/served/completed), agar
			-- definisinya sama dengan ringkasan dan tidak salah kolom.
			WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid')
			  AND NULLIF(payment_info->>'voided_at','') IS NULL AND COALESCE(is_holding,false) = false
			  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) uq ON uq.outlet_id = t.outlet_id
		WHERE t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($2::date + 1)` + txNotVoided("t")

	outletArgs := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		outletQuery += ` AND t.outlet_id = ANY($3::text[])`
		outletArgs = append(outletArgs, pq.Array(filterIDs))
	}
	outletQuery += ` GROUP BY t.outlet_id, o.name, t.outlet_code, uq.cnt, uq.amt ORDER BY SUM(t.total_amount) DESC`

	outletRows, err := database.DB.Query(outletQuery, outletArgs...)
	if err != nil {
		return nil, fmt.Errorf("sales report outlet query failed: %w", err)
	}
	defer outletRows.Close()

	report.ByOutlet = []models.SalesReportOutlet{}
	for outletRows.Next() {
		var row models.SalesReportOutlet
		if err := outletRows.Scan(&row.OutletID, &row.OutletName, &row.TotalTransactions, &row.TotalRevenue, &row.UnpaidOrders, &row.UnpaidAmount); err != nil {
			return nil, fmt.Errorf("sales report outlet scan failed: %w", err)
		}
		report.ByOutlet = append(report.ByOutlet, row)
	}

	countQuery := `SELECT COUNT(*) FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + txNotVoided("cloud_transactions")
	countArgs := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		countQuery += ` AND outlet_id = ANY($3::text[])`
		countArgs = append(countArgs, pq.Array(filterIDs))
	}

	var total int
	if err := database.DB.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, fmt.Errorf("sales report count query failed: %w", err)
	}
	report.Total = total
	report.TotalPages = int(math.Ceil(float64(total) / float64(limit)))

	offset := (page - 1) * limit
	txQuery := `
		SELECT
			t.id,
			COALESCE(o.name, t.outlet_code),
			t.outlet_code,
			t.total_amount,
			t.payment_method,
			-- Nama kasir: app sering mengirim cashier_name kosong, jadi resolve dari
			-- shift kasir yang menaungi waktu transaksi (opened_by) sebagai fallback.
			-- Pakai s.created_at (= waktu shift dibuka sebenarnya); opened_at bisa
			-- ter-overwrite jadi closed_at saat shift ditutup.
			COALESCE(NULLIF(t.cashier_name, ''),
				(SELECT NULLIF(s.opened_by, '') FROM cloud_cashier_shifts s
				 WHERE s.outlet_id = t.outlet_id AND s.created_at <= t.created_at
				 ORDER BY s.created_at DESC LIMIT 1),
				'') AS cashier_name,
			COALESCE(t.orderer_name, ''),
			COALESCE((SELECT o2.pax FROM cloud_orders o2 WHERE o2.id = t.order_id LIMIT 1), 0) AS pax,
			t.items,
			TO_CHAR(t.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM cloud_transactions t
		LEFT JOIN outlets o ON o.id = t.outlet_id
		WHERE t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($2::date + 1)` + txNotVoided("t")

	txArgs := []interface{}{dateFrom, dateTo}
	paramIdx := 3
	if filterIDs != nil {
		txQuery += fmt.Sprintf(` AND t.outlet_id = ANY($%d::text[])`, paramIdx)
		txArgs = append(txArgs, pq.Array(filterIDs))
		paramIdx++
	}
	txQuery += fmt.Sprintf(` ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`, paramIdx, paramIdx+1)
	txArgs = append(txArgs, limit, offset)

	txRows, err := database.DB.Query(txQuery, txArgs...)
	if err != nil {
		return nil, fmt.Errorf("sales report transactions query failed: %w", err)
	}
	defer txRows.Close()

	transactions := []models.SalesReportTransaction{}
	for txRows.Next() {
		var t models.SalesReportTransaction
		if err := txRows.Scan(&t.ID, &t.OutletName, &t.OutletCode, &t.TotalAmount,
			&t.PaymentMethod, &t.CashierName, &t.OrdererName, &t.Pax, &t.Items, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("sales report transaction scan failed: %w", err)
		}
		transactions = append(transactions, t)
	}

	report.Transactions = transactions
	return report, nil
}

func GetUnpaidOrders(outletID, status, dateFrom, dateTo string, scopeIDs []string, page, limit int) (*models.UnpaidOrdersResponse, error) {
	report := &models.UnpaidOrdersResponse{
		Page:  page,
		Limit: limit,
	}

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	where := `WHERE COALESCE(o.payment_info->>'payment_status','unpaid') NOT IN ('paid') AND NULLIF(o.payment_info->>'voided_at','') IS NULL AND COALESCE(o.is_holding,false) = false`
	args := []interface{}{}
	paramIdx := 1

	if filterIDs != nil {
		where += fmt.Sprintf(` AND o.outlet_id = ANY($%d::text[])`, paramIdx)
		args = append(args, pq.Array(filterIDs))
		paramIdx++
	}
	if status != "" {
		where += fmt.Sprintf(` AND o.status = $%d`, paramIdx)
		args = append(args, status)
		paramIdx++
	}
	// Filter tanggal opsional agar detail modal konsisten dengan kartu ringkasan
	// SalesReport yang sudah date-filtered.
	if dateFrom != "" {
		where += fmt.Sprintf(` AND o.created_at >= tz_day_start($%d::date)`, paramIdx)
		args = append(args, dateFrom)
		paramIdx++
	}
	if dateTo != "" {
		where += fmt.Sprintf(` AND o.created_at < tz_day_start($%d::date + 1)`, paramIdx)
		args = append(args, dateTo)
		paramIdx++
	}

	sumQ := `SELECT COUNT(*)::int, COALESCE(SUM(o.total_amount), 0) FROM cloud_orders o ` + where
	if err := database.DB.QueryRow(sumQ, args...).Scan(&report.TotalUnpaid, &report.TotalAmount); err != nil {
		return nil, fmt.Errorf("unpaid orders summary failed: %w", err)
	}

	report.Total = report.TotalUnpaid
	report.TotalPages = int(math.Ceil(float64(report.Total) / float64(limit)))

	offset := (page - 1) * limit
	listQ := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(ot.name, o.outlet_code),
			o.outlet_code,
			COALESCE(o.table_number, ''),
			COALESCE(o.customer_name, ''),
			o.pax,
			o.total_amount,
			o.status,
			COALESCE(o.items::text, '[]'),
			TO_CHAR(o.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			TO_CHAR(o.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM cloud_orders o
		LEFT JOIN outlets ot ON ot.id = o.outlet_id
		%s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d`, where, paramIdx, paramIdx+1)

	listArgs := append(args, limit, offset)
	rows, err := database.DB.Query(listQ, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("unpaid orders list failed: %w", err)
	}
	defer rows.Close()

	report.Orders = []models.UnpaidOrderRow{}
	for rows.Next() {
		var r models.UnpaidOrderRow
		if err := rows.Scan(&r.ID, &r.OutletName, &r.OutletCode, &r.TableNumber,
			&r.CustomerName, &r.Pax, &r.TotalAmount, &r.Status, &r.Items,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("unpaid orders scan failed: %w", err)
		}
		report.Orders = append(report.Orders, r)
	}

	return report, nil
}

func GetProductSalesReport(dateFrom, dateTo, outletID, sortBy, sortDir string, scopeIDs []string, page, limit int) (*models.ProductSalesResponse, error) {
	conds := []string{
		"o.created_at >= tz_day_start($1::date)", "o.created_at < tz_day_start($2::date + 1)",
		// Order yang di-void bukan penjualan.
		"NULLIF(o.payment_info->>'voided_at','') IS NULL AND COALESCE(o.is_holding,false) = false",
	}
	args := []interface{}{dateFrom, dateTo}
	idx := 3

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	if filterIDs != nil {
		conds = append(conds, fmt.Sprintf("o.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(filterIDs))
		idx++
	}
	whereSQL := "WHERE " + strings.Join(conds, " AND ")

	// Kategori: item order sering mengirim 'category'/'category_name' kosong, jadi
	// di-resolve dari cloud_products (cocokkan nama produk dalam outlet yang sama)
	// agar kolom Kategori tidak selalu "Tidak Berkategori".
	// Baris di-grup per (outlet, produk, kategori); grand total dihitung lewat
	// window function supaya kartu ringkasan & % kontribusi tetap mencakup SEMUA
	// halaman meski daftarnya dipaginasi.
	baseSub := fmt.Sprintf(`
		SELECT
			COALESCE(ot.name, o.outlet_code, '') AS outlet_name,
			COALESCE(NULLIF(item->>'product_name', ''), 'Unknown') AS product_name,
			COALESCE(
				NULLIF(item->>'category_name', ''),
				NULLIF(item->>'category', ''),
				(SELECT p.category_name FROM cloud_products p
				 WHERE p.outlet_id = o.outlet_id
				   AND p.name = item->>'product_name'
				   AND COALESCE(p.category_name, '') <> ''
				   AND COALESCE(p.is_deleted, false) = false
				 LIMIT 1),
				'Tidak Berkategori'
			) AS category_name,
			COALESCE((item->>'qty')::int, 0) AS qty,
			COALESCE((item->>'subtotal')::float8, COALESCE((item->>'price')::float8, 0) * COALESCE((item->>'qty')::int, 0)) AS revenue
		FROM cloud_orders o
		LEFT JOIN outlets ot ON ot.id = o.outlet_id,
			jsonb_array_elements(COALESCE(o.items, '[]'::jsonb)) AS item
		%s`, whereSQL)

	// Whitelist kolom & arah urut (cegah SQL injection — nilai disuntik literal).
	orderCol := "total_revenue"
	if sortBy == "qty" {
		orderCol = "total_qty"
	}
	orderDir := "DESC"
	if strings.EqualFold(sortDir, "asc") {
		orderDir = "ASC"
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(`
		SELECT outlet_name, product_name, category_name, total_qty, total_revenue,
			COUNT(*) OVER ()          AS grand_rows,
			SUM(total_qty) OVER ()    AS grand_qty,
			SUM(total_revenue) OVER () AS grand_revenue
		FROM (
			SELECT outlet_name, product_name, category_name,
				SUM(qty) AS total_qty,
				SUM(revenue) AS total_revenue
			FROM (%s) sub
			GROUP BY outlet_name, product_name, category_name
		) agg
		ORDER BY %s %s, total_revenue DESC
		LIMIT $%d OFFSET $%d`, baseSub, orderCol, orderDir, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &models.ProductSalesResponse{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Page:     page,
		Limit:    limit,
		Items:    make([]models.ProductSalesRow, 0),
	}
	for rows.Next() {
		var r models.ProductSalesRow
		if err := rows.Scan(&r.OutletName, &r.ProductName, &r.CategoryName, &r.TotalQty, &r.TotalRevenue,
			&resp.Total, &resp.TotalQty, &resp.TotalRevenue); err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 && page > 1 {
		// Halaman melewati akhir data: total tetap perlu diisi untuk pagination UI.
		database.DB.QueryRow(fmt.Sprintf(`
			SELECT COUNT(*), COALESCE(SUM(total_qty),0), COALESCE(SUM(total_revenue),0) FROM (
				SELECT outlet_name, product_name, category_name,
					SUM(qty) AS total_qty, SUM(revenue) AS total_revenue
				FROM (%s) sub GROUP BY outlet_name, product_name, category_name) agg`, baseSub),
			args[:len(args)-2]...).Scan(&resp.Total, &resp.TotalQty, &resp.TotalRevenue)
	}

	return resp, nil
}

func GetTaxReport(dateFrom, dateTo, outletID string, scopeIDs []string) (*models.TaxReportResponse, error) {
	// Pajak kini per-outlet & dihitung oleh App POS per transaksi, jadi laporan
	// memakai nilai pajak nyata yang tersimpan (cloud_transactions.tax_amount),
	// bukan estimasi dari satu tarif global.

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	effRate := func(tax, net float64) float64 {
		if net <= 0 {
			return 0
		}
		return round2(tax / net * 100)
	}

	args := []interface{}{dateFrom, dateTo}
	outletFilter := ""
	if filterIDs != nil {
		outletFilter = " AND outlet_id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}

	var totalTx int
	var gross, tax float64
	if err := database.DB.QueryRow(
		`SELECT COUNT(*)::int, COALESCE(SUM(total_amount), 0), COALESCE(SUM(tax_amount), 0)
		 FROM cloud_transactions
		 WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`+outletFilter,
		args...,
	).Scan(&totalTx, &gross, &tax); err != nil {
		return nil, err
	}

	gross = round2(gross)
	tax = round2(tax)
	summary := models.TaxReportSummary{
		TotalTransactions: totalTx,
		GrossRevenue:      gross,
		TaxAmount:         tax,
		NetRevenue:        round2(gross - tax),
		TaxRate:           effRate(tax, gross-tax), // tarif efektif gabungan
	}

	dailyRows, err := database.DB.Query(
		`SELECT TO_CHAR(tz_date(created_at), 'YYYY-MM-DD'), COUNT(*)::int,
			COALESCE(SUM(total_amount), 0), COALESCE(SUM(tax_amount), 0)
		 FROM cloud_transactions
		 WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`+outletFilter+`
		 GROUP BY tz_date(created_at) ORDER BY tz_date(created_at) DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer dailyRows.Close()

	daily := make([]models.TaxReportRow, 0)
	for dailyRows.Next() {
		var r models.TaxReportRow
		var g, t float64
		if err := dailyRows.Scan(&r.Date, &r.TotalTransactions, &g, &t); err != nil {
			return nil, err
		}
		r.GrossRevenue = round2(g)
		r.TaxAmount = round2(t)
		r.NetRevenue = round2(g - t)
		daily = append(daily, r)
	}
	if err := dailyRows.Err(); err != nil {
		return nil, err
	}

	outletArgs := []interface{}{dateFrom, dateTo}
	outletFilter2 := ""
	if filterIDs != nil {
		outletFilter2 = " AND t.outlet_id = ANY($3::text[])"
		outletArgs = append(outletArgs, pq.Array(filterIDs))
	}
	outletRows, err := database.DB.Query(
		`SELECT t.outlet_id, COALESCE(o.name, t.outlet_code),
			COALESCE(SUM(t.total_amount), 0), COALESCE(SUM(t.tax_amount), 0),
			COALESCE(o.tax_rate, 0), COALESCE(o.tax_enabled, false)
		 FROM cloud_transactions t
		 LEFT JOIN outlets o ON o.id = t.outlet_id
		 WHERE t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($2::date + 1)`+outletFilter2+`
		 GROUP BY t.outlet_id, o.name, t.outlet_code, o.tax_rate, o.tax_enabled
		 ORDER BY SUM(t.total_amount) DESC`,
		outletArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer outletRows.Close()

	byOutlet := make([]models.TaxOutletRow, 0)
	for outletRows.Next() {
		var r models.TaxOutletRow
		var g, t float64
		if err := outletRows.Scan(&r.OutletID, &r.OutletName, &g, &t, &r.TaxRate, &r.TaxEnabled); err != nil {
			return nil, err
		}
		r.GrossRevenue = round2(g)
		r.TaxAmount = round2(t)
		r.NetRevenue = round2(g - t)
		byOutlet = append(byOutlet, r)
	}
	if err := outletRows.Err(); err != nil {
		return nil, err
	}

	return &models.TaxReportResponse{Summary: summary, Daily: daily, ByOutlet: byOutlet}, nil
}

func GetCashFlowReport(dateFrom, dateTo, outletID string, scopeIDs []string) (*models.CashFlowResponse, error) {
	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	args := []interface{}{dateFrom, dateTo}
	outletFilter := ""
	prOutletFilter := ""
	if filterIDs != nil {
		outletFilter = " AND outlet_id = ANY($3::text[])"
		prOutletFilter = " AND pr.outlet_id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}

	cls := procurementClassExpr("pr")
	query := `
		SELECT TO_CHAR(date, 'YYYY-MM-DD') AS date,
			SUM(sales_receipts) AS sales_receipts,
			SUM(other_receipts) AS other_receipts,
			SUM(cogs_payments)  AS cogs_payments,
			SUM(svc_payments)   AS svc_payments,
			SUM(opex_payments)  AS opex_payments,
			SUM(capex_payments) AS capex_payments,
			SUM(proj_payments)  AS proj_payments,
			SUM(dep_receipts)   AS dep_receipts,
			SUM(dep_refunds)    AS dep_refunds
		FROM (
			-- Penerimaan Penjualan
			SELECT tz_date(created_at) AS date,
				total_amount AS sales_receipts,
				0::float8 AS other_receipts,
				0::float8 AS cogs_payments,
				0::float8 AS svc_payments,
				0::float8 AS opex_payments,
				0::float8 AS capex_payments,
				0::float8 AS proj_payments,
				0::float8 AS dep_receipts,
				0::float8 AS dep_refunds
			FROM cloud_transactions
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + outletFilter + `

			UNION ALL

			-- Kas Masuk = Penerimaan Lainnya, Kas Keluar = Beban Operasional
			SELECT tz_date(created_at) AS date,
				0::float8,
				CASE WHEN LOWER(movement_type) IN ('masuk','in','income','pemasukan') THEN amount ELSE 0 END,
				0::float8,
				0::float8,
				CASE WHEN LOWER(movement_type) NOT IN ('masuk','in','income','pemasukan') THEN amount ELSE 0 END,
				0::float8, 0::float8, 0::float8, 0::float8
			FROM cloud_cash_movements
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + outletFilter + `

			UNION ALL

			-- Pengadaan dipilah per kelas (procurementClassExpr): bahan = HPP,
			-- jasa = Beban Jasa, modal = Belanja Modal, projek = Belanja Projek.
			-- Satu baris per PEMBAYARAN (lihat procurement_finance.go): cicilan
			-- harus jatuh pada tanggal uangnya benar-benar keluar.
			SELECT tz_date(ph.created_at) AS date,
				0::float8, 0::float8,
				CASE WHEN ` + cls + ` = 'bahan'  THEN ph.amount ELSE 0 END,
				CASE WHEN ` + cls + ` = 'jasa'   THEN ph.amount ELSE 0 END,
				0::float8,
				CASE WHEN ` + cls + ` = 'modal'  THEN ph.amount ELSE 0 END,
				CASE WHEN ` + cls + ` = 'projek' THEN ph.amount ELSE 0 END,
				0::float8, 0::float8
			FROM ` + procurementCashOutFrom + `
			` + procurementCashOutWhere(prOutletFilter) + `

			UNION ALL

			-- Uang muka reservasi tervalidasi pada tanggal dibayar: DP/pelunasan
			-- masuk, refund keluar. Kewajiban (bukan pendapatan), tapi kasnya nyata.
			SELECT tz_date(p.paid_at) AS date,
				0::float8, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8,
				CASE WHEN p.type <> 'refund' THEN p.amount ELSE 0 END,
				CASE WHEN p.type = 'refund' THEN p.amount ELSE 0 END
			FROM reservation_payments p
			WHERE p.status = 'validated'
			  AND p.paid_at >= tz_day_start($1::date) AND p.paid_at < tz_day_start($2::date + 1)` +
		strings.Replace(prOutletFilter, "pr.outlet_id", "p.outlet_id", 1) + `

			UNION ALL

			-- Bagian transaksi POS yang dibayar dari uang muka: uangnya sudah masuk
			-- saat DP, jadi dikurangkan dari penerimaan penjualan hari kunjungan.
			SELECT tz_date(created_at) AS date,
				-amount, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8, 0::float8
			FROM transaction_payments
			WHERE payment_method = '` + ReservationDpMethod + `'
			  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + payNotVoided + outletFilter + `
		) sub
		GROUP BY date
		ORDER BY date DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	daily := make([]models.CashFlowRow, 0)
	var sumSales, sumOther, sumCOGS, sumSvc, sumOpex, sumCapex, sumProj, sumDep, sumRefund float64
	for rows.Next() {
		var r models.CashFlowRow
		if err := rows.Scan(&r.Date, &r.SalesReceipts, &r.OtherReceipts,
			&r.COGSPayments, &r.ServicePayments, &r.OpexPayments,
			&r.CapexPayments, &r.ProjectPayments, &r.DepositReceipts, &r.DepositRefunds); err != nil {
			return nil, err
		}
		r.NetCashFlow = (r.SalesReceipts + r.OtherReceipts + r.DepositReceipts) -
			(r.COGSPayments + r.ServicePayments + r.OpexPayments + r.CapexPayments + r.ProjectPayments + r.DepositRefunds)
		daily = append(daily, r)
		sumSales += r.SalesReceipts
		sumOther += r.OtherReceipts
		sumCOGS += r.COGSPayments
		sumSvc += r.ServicePayments
		sumOpex += r.OpexPayments
		sumCapex += r.CapexPayments
		sumProj += r.ProjectPayments
		sumDep += r.DepositReceipts
		sumRefund += r.DepositRefunds
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalReceipts := sumSales + sumOther + sumDep
	totalPayments := sumCOGS + sumSvc + sumOpex + sumCapex + sumProj + sumRefund

	return &models.CashFlowResponse{
		Summary: models.CashFlowSummary{
			SalesReceipts:   sumSales,
			OtherReceipts:   sumOther,
			DepositReceipts: sumDep,
			TotalReceipts:   totalReceipts,
			COGSPayments:    sumCOGS,
			ServicePayments: sumSvc,
			OpexPayments:    sumOpex,
			CapexPayments:   sumCapex,
			ProjectPayments: sumProj,
			DepositRefunds:  sumRefund,
			TotalPayments:   totalPayments,
			NetCashFlow:     totalReceipts - totalPayments,
		},
		Daily: daily,
	}, nil
}

// assetBookValueExprAt: nilai buku satu baris aset (per unit × jumlah) pada
// tanggal parameter, memakai rumus garis lurus yang sama dengan modul aset
// (assetJoins di asset.go): akumulasi = min(basis/umur × bulan berjalan, basis).
func assetBookValueExprAt(dateParam string) string {
	return fmt.Sprintf(`GREATEST(a.purchase_price - LEAST(
		CASE WHEN COALESCE(a.useful_life_months, 0) > 0
		     THEN (a.purchase_price - COALESCE(a.residual_value, 0)) / a.useful_life_months
		          * GREATEST(0, (DATE_PART('year', AGE(%[1]s::date, a.purchase_date)) * 12
		                        + DATE_PART('month', AGE(%[1]s::date, a.purchase_date)))::int)
		     ELSE 0 END,
		GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0)), 0) * a.quantity`, dateParam)
}

// depreciationForPeriod menghitung beban penyusutan garis lurus seluruh aset
// aktif untuk rentang tanggal, per outlet. Beban periode = akumulasi(dateTo)
// − akumulasi(dateFrom − 1 hari), sehingga totalnya selalu cocok dengan
// nilai buku yang ditampilkan modul aset — tidak ada rumus kedua.
func depreciationForPeriod(dateFrom, dateTo string, filterIDs []string) (float64, map[string]float64, error) {
	args := []interface{}{dateFrom, dateTo}
	scope := ""
	if filterIDs != nil {
		scope = " AND a.outlet_id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}
	rows, err := database.DB.Query(`
		SELECT COALESCE(x.outlet_id, ''),
		       COALESCE(SUM((LEAST(x.months_to, x.life) - LEAST(x.months_from, x.life)) * x.base / x.life * x.quantity), 0)
		FROM (
			SELECT a.outlet_id, a.quantity, a.useful_life_months AS life,
			       GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0) AS base,
			       GREATEST(0, (DATE_PART('year', AGE($2::date, a.purchase_date)) * 12
			                   + DATE_PART('month', AGE($2::date, a.purchase_date)))::int) AS months_to,
			       GREATEST(0, (DATE_PART('year', AGE($1::date - 1, a.purchase_date)) * 12
			                   + DATE_PART('month', AGE($1::date - 1, a.purchase_date)))::int) AS months_from
			FROM assets a
			WHERE a.is_deleted = false AND COALESCE(a.status, 'aktif') <> 'dihapus'
			  AND a.purchase_date IS NOT NULL AND COALESCE(a.useful_life_months, 0) > 0`+scope+`
		) x
		GROUP BY 1`, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	byOutlet := map[string]float64{}
	var total float64
	for rows.Next() {
		var outlet string
		var amt float64
		if err := rows.Scan(&outlet, &amt); err != nil {
			return 0, nil, err
		}
		byOutlet[strings.TrimSpace(outlet)] = amt
		total += amt
	}
	return total, byOutlet, rows.Err()
}

// taxTotalsByOutlet menjumlahkan tax_amount riil per outlet untuk periode terpilih.
// Menggantikan estimasi revenue × tarif global agar Neraca/P&L/Buku Besar selalu
// rekonsil dengan Laporan Pajak (yang sudah per-transaksi/per-outlet).
func taxTotalsByOutlet(dateFrom, dateTo string, filterIDs []string) (map[string]float64, float64) {
	q := `SELECT outlet_id, COALESCE(SUM(tax_amount),0)
		FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`
	args := []interface{}{dateFrom, dateTo}
	if filterIDs != nil {
		q += ` AND outlet_id = ANY($3::text[])`
		args = append(args, pq.Array(filterIDs))
	}
	q += ` GROUP BY outlet_id`

	byOutlet := map[string]float64{}
	var total float64
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return byOutlet, 0
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var v float64
		if rows.Scan(&id, &v) == nil {
			byOutlet[strings.TrimSpace(id)] = v
			total += v
		}
	}
	return byOutlet, total
}

func GetBalanceReport(dateFrom, dateTo, outletID string, scopeIDs []string) (*models.BalanceResponse, error) {
	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	taxByOutlet, taxTotal := taxTotalsByOutlet(dateFrom, dateTo, filterIDs)

	args := []interface{}{dateFrom, dateTo}
	outletWhere := ""
	if filterIDs != nil {
		outletWhere = " AND o.id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}

	outletRows, err := database.DB.Query(`
		SELECT
			o.id,
			o.name,
			COALESCE(t.total_revenue, 0),
			COALESCE(mv.total_cash_in, 0),
			COALESCE(mv.total_expense, 0),
			COALESCE(uq.unpaid_amount, 0),
			COALESCE(pr.total_procurement, 0),
			COALESCE(ap.accounts_payable, 0),
			COALESCE(inv.inventory, 0),
			COALESCE(fa.fixed_assets, 0),
			COALESCE(pj.projects, 0),
			COALESCE(cd.deposits, 0),
			COALESCE(ca.cash_adj, 0)
		FROM outlets o
		LEFT JOIN (
			SELECT outlet_id, SUM(total_amount) AS total_revenue
			FROM cloud_transactions
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id,
				SUM(CASE WHEN LOWER(movement_type) IN ('masuk','in','income','pemasukan') THEN amount ELSE 0 END) AS total_cash_in,
				SUM(CASE WHEN LOWER(movement_type) NOT IN ('masuk','in','income','pemasukan') THEN amount ELSE 0 END) AS total_expense
			FROM cloud_cash_movements
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) mv ON mv.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, SUM(total_amount) AS unpaid_amount
			FROM cloud_orders
			WHERE COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid')
			  AND NULLIF(payment_info->>'voided_at','') IS NULL AND COALESCE(is_holding,false) = false
			  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) uq ON uq.outlet_id = o.id
		LEFT JOIN (
			SELECT pr.outlet_id, SUM(ph.amount) AS total_procurement
			FROM `+procurementCashOutFrom+`
			`+procurementCashOutWhere("")+`
			GROUP BY pr.outlet_id
		) pr ON pr.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, SUM(`+payableExpr("")+`) AS accounts_payable
			FROM purchase_requests
			WHERE `+outstandingCond+`
			  AND `+countableCond("")+`
			GROUP BY outlet_id
		) ap ON ap.outlet_id = o.id
		-- Persediaan: nilai buku stok gudang outlet saat ini (gudang induk tanpa
		-- outlet dijumlahkan ke total di bawah).
		LEFT JOIN (
			SELECT w.outlet_id, SUM(sl.qty_base * sl.avg_cost) AS inventory
			FROM stock_ledger sl JOIN warehouses w ON w.id = sl.warehouse_id
			WHERE w.outlet_id IS NOT NULL
			GROUP BY w.outlet_id
		) inv ON inv.outlet_id = o.id
		-- Aset tetap: nilai buku per tanggal akhir periode, rumus modul aset.
		LEFT JOIN (
			SELECT a.outlet_id, SUM(`+assetBookValueExprAt("$2")+`) AS fixed_assets
			FROM assets a
			WHERE a.is_deleted = false AND COALESCE(a.status, 'aktif') <> 'dihapus'
			  AND (a.purchase_date IS NULL OR a.purchase_date <= $2::date)
			GROUP BY a.outlet_id
		) fa ON fa.outlet_id = o.id
		-- Projek berjalan: belanja projek yang sudah dibayar dan belum berwujud
		-- aset, untuk projek yang belum selesai.
		LEFT JOIN (
			SELECT p.outlet_id, GREATEST(SUM(p.paid_amount) - COALESCE(SUM(av.value), 0), 0) AS projects
			FROM (
				SELECT pr.id, pr.outlet_id, COALESCE(pr.paid_amount, 0) AS paid_amount
				FROM purchase_requests pr
				JOIN projects pj ON pj.id = pr.project_id
				WHERE pj.status NOT IN ('selesai','batal')
				  AND pr.status NOT IN ('pending','rejected','cancelled')
				  AND `+countableCond("pr")+`
			) p
			LEFT JOIN (
				SELECT purchase_request_id, SUM(purchase_price * quantity) AS value
				FROM assets WHERE is_deleted = false AND purchase_request_id IS NOT NULL
				GROUP BY purchase_request_id
			) av ON av.purchase_request_id = p.id
			GROUP BY p.outlet_id
		) pj ON pj.outlet_id = o.id
		-- Uang muka pelanggan: kewajiban atas reservasi yang belum ditutup (atau
		-- dibatalkan dengan keputusan refund tapi refundnya belum dicatat).
		LEFT JOIN (
			SELECT p.outlet_id, SUM(CASE WHEN p.type = 'refund' THEN -p.amount ELSE p.amount END) AS deposits
			FROM reservation_payments p JOIN reservations rv ON rv.id = p.reservation_id
			WHERE p.status = 'validated'
			  AND (rv.status IN ('pending','confirmed') OR (rv.status = 'cancelled' AND rv.cancel_disposition = 'refund'))
			GROUP BY p.outlet_id
		) cd ON cd.outlet_id = o.id
		-- Penyesuaian kas periode: uang muka masuk − refund − bagian transaksi
		-- yang dibayar dari uang muka (uangnya sudah dihitung saat DP).
		LEFT JOIN (
			SELECT outlet_id, SUM(v) AS cash_adj FROM (
				SELECT p.outlet_id, CASE WHEN p.type = 'refund' THEN -p.amount ELSE p.amount END AS v
				FROM reservation_payments p
				WHERE p.status = 'validated' AND p.paid_at >= tz_day_start($1::date) AND p.paid_at < tz_day_start($2::date + 1)
				UNION ALL
				SELECT tp.outlet_id, -tp.amount FROM transaction_payments tp
				WHERE tp.payment_method = '`+ReservationDpMethod+`'
				  AND tp.created_at >= tz_day_start($1::date) AND tp.created_at < tz_day_start($2::date + 1)
			) x GROUP BY outlet_id
		) ca ON ca.outlet_id = o.id
		WHERE o.is_active = true
		  AND (t.outlet_id IS NOT NULL OR mv.outlet_id IS NOT NULL OR uq.outlet_id IS NOT NULL OR pr.outlet_id IS NOT NULL OR ap.outlet_id IS NOT NULL
		       OR inv.outlet_id IS NOT NULL OR fa.outlet_id IS NOT NULL OR pj.outlet_id IS NOT NULL OR cd.outlet_id IS NOT NULL)`+outletWhere+`
		ORDER BY COALESCE(t.total_revenue, 0) DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer outletRows.Close()

	outlets := make([]models.BalanceOutletRow, 0)
	var sumRev, sumCashIn, sumExp, sumUnpaid, sumAP, sumInv, sumFA, sumPJ, sumDep, sumCashAdj float64
	for outletRows.Next() {
		var r models.BalanceOutletRow
		var procurement, cashAdj float64
		if err := outletRows.Scan(&r.OutletID, &r.OutletName, &r.TotalRevenue, &r.TotalCashIn, &r.TotalExpense, &r.UnpaidAmount, &procurement, &r.AccountsPayable,
			&r.Inventory, &r.FixedAssets, &r.ProjectsInProgress, &r.CustomerDeposits, &cashAdj); err != nil {
			return nil, err
		}
		r.TotalExpense += procurement
		r.CustomerDeposits = math.Max(round2(r.CustomerDeposits), 0)
		// Aset
		r.CashAndEquivalents = round2(r.TotalRevenue + r.TotalCashIn - r.TotalExpense + cashAdj)
		r.Receivables = r.UnpaidAmount
		r.Inventory = round2(r.Inventory)
		r.FixedAssets = round2(r.FixedAssets)
		r.ProjectsInProgress = round2(r.ProjectsInProgress)
		r.TotalAssets = round2(r.CashAndEquivalents + r.Receivables + r.Inventory + r.FixedAssets + r.ProjectsInProgress)
		// Kewajiban
		r.TaxPayable = round2(taxByOutlet[strings.TrimSpace(r.OutletID)])
		r.TotalLiabilities = round2(r.TaxPayable + r.AccountsPayable + r.CustomerDeposits)
		// Ekuitas = Aset - Kewajiban
		r.TotalEquity = round2(r.TotalAssets - r.TotalLiabilities)

		outlets = append(outlets, r)
		sumRev += r.TotalRevenue
		sumCashIn += r.TotalCashIn
		sumExp += r.TotalExpense
		sumUnpaid += r.UnpaidAmount
		sumAP += r.AccountsPayable
		sumInv += r.Inventory
		sumFA += r.FixedAssets
		sumPJ += r.ProjectsInProgress
		sumDep += r.CustomerDeposits
		sumCashAdj += cashAdj
	}
	if err := outletRows.Err(); err != nil {
		return nil, err
	}

	// Add procurement with no outlet (outlet_id IS NULL) to global expense
	var nullOutletProcurement float64
	nullPrArgs := []interface{}{dateFrom, dateTo}
	nullPrQuery := `SELECT COALESCE(SUM(ph.amount), 0)
		FROM ` + procurementCashOutFrom + `
		` + procurementCashOutWhere(" AND pr.outlet_id IS NULL")
	if filterIDs == nil {
		if err := database.DB.QueryRow(nullPrQuery, nullPrArgs...).Scan(&nullOutletProcurement); err != nil {
			return nil, err
		}
		sumExp += nullOutletProcurement
	}

	// Accounts payable with no outlet
	var nullOutletAP float64
	if filterIDs == nil {
		database.DB.QueryRow(`SELECT COALESCE(SUM(`+payableExpr("")+`), 0)
			FROM purchase_requests WHERE `+outstandingCond+` AND outlet_id IS NULL AND `+countableCond("")).Scan(&nullOutletAP)
		sumAP += nullOutletAP

		// Persediaan gudang induk (tanpa outlet) dan projek tanpa outlet hanya
		// masuk ke total gabungan — tidak ada baris outlet yang memilikinya.
		var centralInv, nullProjects float64
		database.DB.QueryRow(`SELECT COALESCE(SUM(sl.qty_base * sl.avg_cost), 0)
			FROM stock_ledger sl JOIN warehouses w ON w.id = sl.warehouse_id WHERE w.outlet_id IS NULL`).Scan(&centralInv)
		database.DB.QueryRow(`
			SELECT GREATEST(COALESCE(SUM(p.paid_amount), 0) - COALESCE(SUM(av.value), 0), 0)
			FROM (
				SELECT pr.id, COALESCE(pr.paid_amount, 0) AS paid_amount
				FROM purchase_requests pr JOIN projects pj ON pj.id = pr.project_id
				WHERE pj.status NOT IN ('selesai','batal') AND pr.outlet_id IS NULL
				  AND pr.status NOT IN ('pending','rejected','cancelled') AND `+countableCond("pr")+`
			) p
			LEFT JOIN (
				SELECT purchase_request_id, SUM(purchase_price * quantity) AS value
				FROM assets WHERE is_deleted = false AND purchase_request_id IS NOT NULL
				GROUP BY purchase_request_id
			) av ON av.purchase_request_id = p.id`).Scan(&nullProjects)
		sumInv += centralInv
		sumPJ += nullProjects
	}

	// Total accounting
	cash := round2(sumRev + sumCashIn - sumExp + sumCashAdj)
	receivables := sumUnpaid
	inventory := round2(sumInv)
	fixedAssets := round2(sumFA)
	projects := round2(sumPJ)
	totalAssets := round2(cash + receivables + inventory + fixedAssets + projects)
	taxPayable := round2(taxTotal)
	accountsPayable := round2(sumAP)
	customerDeposits := round2(sumDep)
	totalLiabilities := round2(taxPayable + accountsPayable + customerDeposits)
	totalEquity := round2(totalAssets - totalLiabilities)

	return &models.BalanceResponse{
		DateFrom:           dateFrom,
		DateTo:             dateTo,
		CashAndEquivalents: cash,
		Receivables:        receivables,
		Inventory:          inventory,
		FixedAssets:        fixedAssets,
		ProjectsInProgress: projects,
		TotalAssets:        totalAssets,
		AccountsPayable:    accountsPayable,
		CustomerDeposits:   customerDeposits,
		TaxPayable:         taxPayable,
		TotalLiabilities:   totalLiabilities,
		TotalEquity:        totalEquity,
		TotalRevenue:       sumRev,
		TotalCashIn:        sumCashIn,
		TotalExpense:       sumExp,
		UnpaidAmount:       sumUnpaid,
		Outlets:            outlets,
	}, nil
}

func GetProfitLossReport(dateFrom, dateTo, outletID string, scopeIDs []string) (*models.ProfitLossResponse, error) {
	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }

	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	args := []interface{}{dateFrom, dateTo}
	outletFilter := ""
	// Arus kas keluar pengadaan dibaca lewat payment_histories yang di-join ke
	// purchase_requests, jadi butuh varian filter yang ber-alias.
	prOutletFilter := ""
	if filterIDs != nil {
		outletFilter = " AND outlet_id = ANY($3::text[])"
		prOutletFilter = " AND pr.outlet_id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}
	// Hanya kelas 'bahan' yang menjadi HPP dan 'jasa' yang menjadi beban jasa.
	// Belanja modal dan projek bukan beban: keduanya menjadi aset (lihat Neraca)
	// dan masuk Laba/Rugi lewat penyusutan.
	cls := procurementClassExpr("pr")

	// ── Daily breakdown ──
	dailyRows, err := database.DB.Query(`
		SELECT TO_CHAR(date, 'YYYY-MM-DD') AS date,
			SUM(revenue) AS revenue,
			SUM(cogs)    AS cogs,
			SUM(opex)    AS opex
		FROM (
			SELECT tz_date(created_at) AS date, total_amount AS revenue, 0::float8 AS cogs, 0::float8 AS opex
			FROM cloud_transactions
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`+outletFilter+`

			UNION ALL

			SELECT tz_date(created_at) AS date, 0::float8, 0::float8, amount
			FROM cloud_cash_movements
			WHERE LOWER(movement_type) NOT IN ('masuk','in','income','pemasukan')
			  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`+outletFilter+`

			UNION ALL

			SELECT tz_date(ph.created_at) AS date, 0::float8,
				CASE WHEN `+cls+` = 'bahan' THEN ph.amount ELSE 0 END,
				CASE WHEN `+cls+` = 'jasa'  THEN ph.amount ELSE 0 END
			FROM `+procurementCashOutFrom+`
			`+procurementCashOutWhere(prOutletFilter)+`
		) sub
		GROUP BY date
		ORDER BY date DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer dailyRows.Close()

	daily := make([]models.ProfitLossRow, 0)
	for dailyRows.Next() {
		var r models.ProfitLossRow
		var opex float64
		if err := dailyRows.Scan(&r.Date, &r.Revenue, &r.COGS, &opex); err != nil {
			return nil, err
		}
		r.GrossProfit = r.Revenue - r.COGS
		r.OperatingExpense = opex
		r.NetProfit = r.GrossProfit - opex
		daily = append(daily, r)
	}
	if err := dailyRows.Err(); err != nil {
		return nil, err
	}

	// ── Summary totals ──
	var salesRevenue, otherIncome, totalCOGS, serviceExpense, operatingExpense float64
	var capexPayments, projectPayments float64

	// Sales revenue
	revArgs := []interface{}{dateFrom, dateTo}
	revQ := `SELECT COALESCE(SUM(total_amount), 0) FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`
	if filterIDs != nil {
		revQ += ` AND outlet_id = ANY($3::text[])`
		revArgs = append(revArgs, pq.Array(filterIDs))
	}
	database.DB.QueryRow(revQ, revArgs...).Scan(&salesRevenue)

	// Other income (kas masuk)
	oiArgs := []interface{}{dateFrom, dateTo}
	oiQ := `SELECT COALESCE(SUM(amount), 0) FROM cloud_cash_movements
		WHERE LOWER(movement_type) IN ('masuk','in','income','pemasukan')
		  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`
	if filterIDs != nil {
		oiQ += ` AND outlet_id = ANY($3::text[])`
		oiArgs = append(oiArgs, pq.Array(filterIDs))
	}
	database.DB.QueryRow(oiQ, oiArgs...).Scan(&otherIncome)

	// Uang muka reservasi yang HANGUS (dibatalkan dengan keputusan 'hangus')
	// menjadi pendapatan lain pada tanggal pembatalan — satu-satunya jalan uang
	// muka masuk P&L tanpa transaksi POS.
	var forfeited float64
	ffArgs := []interface{}{dateFrom, dateTo}
	ffQ := `SELECT COALESCE(SUM(s.v), 0) FROM (
		SELECT rv.id, SUM(CASE WHEN p.type = 'refund' THEN -p.amount ELSE p.amount END) AS v
		FROM reservations rv JOIN reservation_payments p ON p.reservation_id = rv.id AND p.status = 'validated'
		WHERE rv.status = 'cancelled' AND rv.cancel_disposition = 'hangus'
		  AND rv.cancelled_at >= tz_day_start($1::date) AND rv.cancelled_at < tz_day_start($2::date + 1)`
	if filterIDs != nil {
		ffQ += ` AND rv.outlet_id = ANY($3::text[])`
		ffArgs = append(ffArgs, pq.Array(filterIDs))
	}
	ffQ += ` GROUP BY rv.id) s`
	database.DB.QueryRow(ffQ, ffArgs...).Scan(&forfeited)
	otherIncome += forfeited

	// COGS (purchase barang) and Service expense (purchase jasa)
	prArgs := []interface{}{dateFrom, dateTo}
	prQ := `SELECT
		COALESCE(SUM(CASE WHEN ` + cls + ` = 'bahan'  THEN ph.amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN ` + cls + ` = 'jasa'   THEN ph.amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN ` + cls + ` = 'modal'  THEN ph.amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN ` + cls + ` = 'projek' THEN ph.amount ELSE 0 END), 0)
	FROM ` + procurementCashOutFrom + `
	` + procurementCashOutWhere("")
	if filterIDs != nil {
		prQ += ` AND pr.outlet_id = ANY($3::text[])`
		prArgs = append(prArgs, pq.Array(filterIDs))
	}
	database.DB.QueryRow(prQ, prArgs...).Scan(&totalCOGS, &serviceExpense, &capexPayments, &projectPayments)

	// Operating expense (kas keluar)
	opArgs := []interface{}{dateFrom, dateTo}
	opQ := `SELECT COALESCE(SUM(amount), 0) FROM cloud_cash_movements
		WHERE LOWER(movement_type) NOT IN ('masuk','in','income','pemasukan')
		  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)`
	if filterIDs != nil {
		opQ += ` AND outlet_id = ANY($3::text[])`
		opArgs = append(opArgs, pq.Array(filterIDs))
	}
	database.DB.QueryRow(opQ, opArgs...).Scan(&operatingExpense)

	// Penyusutan aset periode ini — satu-satunya jalan belanja modal ke P&L.
	depreciation, depByOutlet, err := depreciationForPeriod(dateFrom, dateTo, filterIDs)
	if err != nil {
		return nil, err
	}
	depreciation = round2(depreciation)

	totalRevenue := salesRevenue + otherIncome
	grossProfit := salesRevenue - totalCOGS
	totalOpex := serviceExpense + operatingExpense + depreciation
	operatingProfit := grossProfit + otherIncome - totalOpex
	_, pnlTax := taxTotalsByOutlet(dateFrom, dateTo, filterIDs)
	taxExpense := round2(pnlTax)
	netProfit := operatingProfit - taxExpense

	grossMargin := 0.0
	if salesRevenue > 0 {
		grossMargin = math.Round(grossProfit/salesRevenue*10000) / 100
	}
	netMargin := 0.0
	if totalRevenue > 0 {
		netMargin = math.Round(netProfit/totalRevenue*10000) / 100
	}

	// ── Per-outlet breakdown ──
	outletArgs := []interface{}{dateFrom, dateTo}
	outletFilter2 := ""
	if filterIDs != nil {
		outletFilter2 = " AND o.id = ANY($3::text[])"
		outletArgs = append(outletArgs, pq.Array(filterIDs))
	}
	outletQRows, err := database.DB.Query(`
		SELECT
			o.id,
			o.name,
			COALESCE(t.revenue, 0),
			COALESCE(pr_b.cogs, 0),
			COALESCE(mv.expense, 0) + COALESCE(pr_j.svc, 0)
		FROM outlets o
		LEFT JOIN (
			SELECT outlet_id, SUM(total_amount) AS revenue
			FROM cloud_transactions
			WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) t ON t.outlet_id = o.id
		LEFT JOIN (
			SELECT pr.outlet_id, SUM(ph.amount) AS cogs
			FROM `+procurementCashOutFrom+`
			`+procurementCashOutWhere(" AND "+cls+" = 'bahan'")+`
			GROUP BY pr.outlet_id
		) pr_b ON pr_b.outlet_id = o.id
		LEFT JOIN (
			SELECT pr.outlet_id, SUM(ph.amount) AS svc
			FROM `+procurementCashOutFrom+`
			`+procurementCashOutWhere(" AND "+cls+" = 'jasa'")+`
			GROUP BY pr.outlet_id
		) pr_j ON pr_j.outlet_id = o.id
		LEFT JOIN (
			SELECT outlet_id, SUM(amount) AS expense
			FROM cloud_cash_movements
			WHERE LOWER(movement_type) NOT IN ('masuk','in','income','pemasukan')
			  AND created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)
			GROUP BY outlet_id
		) mv ON mv.outlet_id = o.id
		WHERE o.is_active = true
		  AND (t.outlet_id IS NOT NULL OR mv.outlet_id IS NOT NULL OR pr_b.outlet_id IS NOT NULL OR pr_j.outlet_id IS NOT NULL)`+outletFilter2+`
		ORDER BY COALESCE(t.revenue, 0) DESC`,
		outletArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer outletQRows.Close()

	byOutlet := make([]models.ProfitLossOutletRow, 0)
	for outletQRows.Next() {
		var r models.ProfitLossOutletRow
		if err := outletQRows.Scan(&r.OutletID, &r.OutletName, &r.Revenue, &r.COGS, &r.OperatingExpense); err != nil {
			return nil, err
		}
		r.Depreciation = round2(depByOutlet[strings.TrimSpace(r.OutletID)])
		r.NetProfit = r.Revenue - r.COGS - r.OperatingExpense - r.Depreciation
		byOutlet = append(byOutlet, r)
	}
	if err := outletQRows.Err(); err != nil {
		return nil, err
	}

	return &models.ProfitLossResponse{
		Summary: models.ProfitLossSummary{
			SalesRevenue:     round2(salesRevenue),
			OtherIncome:      round2(otherIncome),
			ForfeitedDeposits: round2(forfeited),
			TotalRevenue:     round2(totalRevenue),
			COGS:             round2(totalCOGS),
			GrossProfit:      round2(grossProfit),
			GrossMargin:      grossMargin,
			ServiceExpense:   round2(serviceExpense),
			OperatingExpense: round2(operatingExpense),
			DepreciationExpense: depreciation,
			TotalOpex:        round2(totalOpex),
			OperatingProfit:  round2(operatingProfit),
			CapexPayments:    round2(capexPayments),
			ProjectPayments:  round2(projectPayments),
			TaxExpense:       taxExpense,
			NetProfit:        round2(netProfit),
			NetMargin:        netMargin,
		},
		Daily:    daily,
		ByOutlet: byOutlet,
	}, nil
}

// ── Buku Besar (General Ledger) ─────────────────────────────

// Kode akun COA F&B yang dipakai buku besar.
const (
	glAccCash         = "1-100"
	glAccReceivable   = "1-200"
	glAccFixedAsset   = "1-400" // belanja modal; dikredit oleh penyusutan (metode neto)
	glAccProjectWIP   = "1-450" // belanja projek yang belum berwujud aset
	glAccPayable      = "2-100"
	glAccTaxPayable   = "2-200"
	glAccCustomerDep  = "2-300" // uang muka reservasi: kewajiban sampai ditutup di POS
	glAccRevenue      = "4-100"
	glAccOtherIncome  = "4-200"
	glAccCOGS         = "5-100"
	glAccServiceExp   = "5-200"
	glAccOpex         = "5-300"
	glAccDepreciation = "5-400"
	glAccTaxExpense   = "6-100"
)

type glAccountMeta struct {
	Code  string
	Name  string
	Group string
}

// glAccounts menentukan urutan tampil sekaligus nama & kelompok tiap akun.
var glAccounts = []glAccountMeta{
	{glAccCash, "Kas & Setara Kas", "aset"},
	{glAccReceivable, "Piutang Usaha", "aset"},
	{glAccFixedAsset, "Aset Tetap (Peralatan)", "aset"},
	{glAccProjectWIP, "Projek Berjalan", "aset"},
	{glAccPayable, "Hutang Usaha", "kewajiban"},
	{glAccTaxPayable, "Hutang Pajak Restoran", "kewajiban"},
	{glAccCustomerDep, "Uang Muka Pelanggan", "kewajiban"},
	{glAccRevenue, "Pendapatan Penjualan", "pendapatan"},
	{glAccOtherIncome, "Pendapatan Lainnya", "pendapatan"},
	{glAccCOGS, "HPP - Bahan Baku", "beban"},
	{glAccServiceExp, "Beban Jasa & Layanan", "beban"},
	{glAccOpex, "Beban Operasional", "beban"},
	{glAccDepreciation, "Beban Penyusutan Aset", "beban"},
	{glAccTaxExpense, "Beban Pajak Restoran", "beban"},
}

// glAccountForClass memetakan kelas belanja (procurementClassExpr) ke akun
// yang didebit saat pengadaan diakui.
func glAccountForClass(cls string) (account, label string) {
	switch cls {
	case "jasa":
		return glAccServiceExp, "Beban Jasa"
	case "modal":
		return glAccFixedAsset, "Aset Tetap"
	case "projek":
		return glAccProjectWIP, "Projek"
	}
	return glAccCOGS, "HPP"
}

func GetGeneralLedger(dateFrom, dateTo, outletID, accountFilter string, scopeIDs []string) (*models.GeneralLedgerResponse, error) {
	// Normalisasi filter outlet: outlet_id eksplisit menang, selain itu pakai
	// scope role. scopeIDs nil = boleh semua outlet.
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	type rawEntry struct {
		At          time.Time // waktu asli, dipakai mengurutkan saldo berjalan
		Date        string
		Description string
		Debit       float64
		Credit      float64
	}

	accountEntries := make(map[string][]rawEntry, len(glAccounts))

	// wanted: akun ikut ditampilkan? add: catat entri (dibuang kalau akunnya
	// tidak diminta, sehingga tidak ada alokasi sia-sia saat difilter).
	wanted := func(code string) bool { return accountFilter == "" || accountFilter == code }
	add := func(code string, at time.Time, date, desc string, debit, credit float64) {
		if !wanted(code) {
			return
		}
		accountEntries[code] = append(accountEntries[code], rawEntry{
			At: at, Date: date, Description: desc, Debit: debit, Credit: credit,
		})
	}
	// needQ: lewati query yang tak satu pun akunnya diminta.
	needQ := func(accounts ...string) bool {
		for _, a := range accounts {
			if wanted(a) {
				return true
			}
		}
		return false
	}

	// eachRow menutup rows tepat setelah query selesai — penting karena
	// fungsi ini menembak 6 query berurutan dalam satu request.
	eachRow := func(label, query string, qargs []interface{}, scan func(*sql.Rows) error) error {
		rows, err := database.DB.Query(query, qargs...)
		if err != nil {
			return fmt.Errorf("general ledger: %s query failed: %w", label, err)
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return fmt.Errorf("general ledger: %s scan failed: %w", label, err)
			}
		}
		return rows.Err()
	}

	// Filter outlet dipasang sebagai $3 untuk query yang tabelnya tanpa alias.
	args := []interface{}{dateFrom, dateTo}
	outletFilter := ""
	if filterIDs != nil {
		outletFilter = " AND outlet_id = ANY($3::text[])"
		args = append(args, pq.Array(filterIDs))
	}
	// Versi beralias (pr., o.) untuk query yang tabelnya di-JOIN.
	aliasFilter := func(alias string) string {
		if filterIDs == nil {
			return ""
		}
		return " AND " + alias + ".outlet_id = ANY($3::text[])"
	}

	// 1) Pendapatan Penjualan — dari cloud_transactions (transaksi ter-void
	//    dikecualikan agar angkanya sama dengan Laporan Penjualan).
	if needQ(glAccCash, glAccRevenue) {
		q := `
		SELECT
			created_at,
			TO_CHAR(tz_date(created_at), 'YYYY-MM-DD') AS date,
			COALESCE(cashier_name, 'Kasir') || ' - ' || COALESCE(payment_method, '') AS description,
			COALESCE(total_amount, 0)
		FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` +
			outletFilter + txNotVoided("cloud_transactions") + `
		ORDER BY created_at`

		if err := eachRow("revenue", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &amount); err != nil {
				return err
			}
			desc = "Penjualan: " + desc
			add(glAccCash, at, date, desc, amount, 0)
			add(glAccRevenue, at, date, desc, 0, amount)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 2) Kas Masuk & Pengeluaran Operasional — dari cloud_cash_movements
	if needQ(glAccCash, glAccOtherIncome, glAccOpex) {
		q := `
		SELECT
			created_at,
			TO_CHAR(tz_date(created_at), 'YYYY-MM-DD') AS date,
			COALESCE(NULLIF(note, ''), movement_type) AS description,
			movement_type,
			COALESCE(amount, 0)
		FROM cloud_cash_movements
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` + outletFilter + `
		ORDER BY created_at`

		if err := eachRow("cash movement", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc, mvType string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &mvType, &amount); err != nil {
				return err
			}
			switch strings.ToLower(mvType) {
			case "masuk", "in", "income", "pemasukan":
				desc = "Kas Masuk: " + desc
				add(glAccCash, at, date, desc, amount, 0)
				add(glAccOtherIncome, at, date, desc, 0, amount)
			default:
				desc = "Pengeluaran: " + desc
				add(glAccOpex, at, date, desc, amount, 0)
				add(glAccCash, at, date, desc, 0, amount)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 3) Pengakuan belanja pengadaan — AKRUAL per dokumen pada tanggal
	//    disetujui: Dr (HPP | Beban Jasa | Aset Tetap | Projek Berjalan) /
	//    Cr Hutang Usaha sebesar harga final. Pembayaran (4) lalu mengurangi
	//    hutang, bukan mendebit beban lagi.
	//
	//    Skema lama mendebit beban DUA KALI untuk pembelian tempo/cicilan
	//    lintas periode: sekali sebagai "sisa hutang" (disaring per tanggal
	//    dibuat), sekali lagi saat dibayar — dan hutangnya tidak pernah dibalik.
	//    Pengajuan 10 juta dicicil 4 + 6 lintas bulan menghasilkan HPP 16 juta.
	if needQ(glAccPayable, glAccCOGS, glAccServiceExp, glAccFixedAsset, glAccProjectWIP) {
		q := `
		SELECT
			COALESCE(pr.approved_at, pr.created_at) AS at,
			TO_CHAR(tz_date(COALESCE(pr.approved_at, pr.created_at)), 'YYYY-MM-DD') AS date,
			` + procurementClassExpr("pr") + `,
			pr.request_number || ' · ' || COALESCE(wu.name, '-') || ' — ' || pr.requested_by AS description,
			pr.total_final
		FROM purchase_requests pr
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		WHERE pr.status NOT IN ('pending','rejected','cancelled') AND pr.total_final > 0
		  AND ` + countableCond("pr") + `
		  AND COALESCE(pr.approved_at, pr.created_at) >= tz_day_start($1::date)
		  AND COALESCE(pr.approved_at, pr.created_at) < tz_day_start($2::date + 1)` +
			aliasFilter("pr") + `
		ORDER BY 1`

		if err := eachRow("procurement accrual", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, cls, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &cls, &desc, &amount); err != nil {
				return err
			}
			account, label := glAccountForClass(cls)
			add(account, at, date, label+": "+desc, amount, 0)
			add(glAccPayable, at, date, label+": "+desc, 0, amount)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 4) Pembayaran pengadaan — Dr Hutang Usaha / Cr Kas, satu baris per
	//    PEMBAYARAN supaya cicilan jatuh pada tanggal uangnya benar-benar keluar.
	if needQ(glAccCash, glAccPayable) {
		q := `
		SELECT
			ph.created_at,
			TO_CHAR(tz_date(ph.created_at), 'YYYY-MM-DD') AS date,
			pr.request_number || ' · ' || COALESCE(wu.name, '-') || ' — ' || pr.requested_by AS description,
			ph.amount
		FROM ` + procurementCashOutFrom + `
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		` + procurementCashOutWhere(aliasFilter("pr")) + `
		ORDER BY ph.created_at`

		if err := eachRow("procurement payment", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &amount); err != nil {
				return err
			}
			add(glAccPayable, at, date, "Bayar: "+desc, amount, 0)
			add(glAccCash, at, date, "Bayar: "+desc, 0, amount)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 5) Piutang Usaha — cloud_orders yang belum lunas (snapshot saat ini)
	if needQ(glAccReceivable, glAccRevenue) {
		q := `
		SELECT
			o.created_at,
			TO_CHAR(tz_date(o.created_at), 'YYYY-MM-DD') AS date,
			COALESCE(ot.name, o.outlet_code, '-') || ' - ' || COALESCE(o.customer_name, 'Pelanggan') AS description,
			COALESCE(o.total_amount, 0)
		FROM cloud_orders o
		LEFT JOIN outlets ot ON ot.id = o.outlet_id
		WHERE COALESCE(o.payment_info->>'payment_status','unpaid') <> 'paid'
		  AND NULLIF(o.payment_info->>'voided_at','') IS NULL AND COALESCE(o.is_holding,false) = false
		  AND o.created_at >= tz_day_start($1::date) AND o.created_at < tz_day_start($2::date + 1)` +
			aliasFilter("o") + `
		ORDER BY o.created_at`

		if err := eachRow("receivables", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &amount); err != nil {
				return err
			}
			desc = "Piutang: " + desc
			add(glAccReceivable, at, date, desc, amount, 0)
			add(glAccRevenue, at, date, desc, 0, amount)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 6) Pajak Restoran (PB1) — pakai tax_amount riil per transaksi (per-outlet,
	// termasuk outlet yang pajaknya nonaktif/tarif beda), bukan revenue × tarif
	// global. Dicatat sekali per hari, memakai transaksi terakhir hari itu
	// sebagai waktu agar urutannya jatuh di akhir hari.
	if needQ(glAccTaxExpense, glAccTaxPayable) {
		q := `
		SELECT MAX(created_at) AS at,
		       TO_CHAR(tz_date(created_at), 'YYYY-MM-DD') AS date,
		       COALESCE(SUM(tax_amount), 0)
		FROM cloud_transactions
		WHERE created_at >= tz_day_start($1::date) AND created_at < tz_day_start($2::date + 1)` +
			outletFilter + txNotVoided("cloud_transactions") + `
		GROUP BY tz_date(created_at)
		ORDER BY tz_date(created_at)`

		if err := eachRow("tax", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date string
			var dayTax float64
			if err := rows.Scan(&at, &date, &dayTax); err != nil {
				return err
			}
			if taxAmt := round2(dayTax); taxAmt > 0 {
				add(glAccTaxExpense, at, date, "Pajak Restoran (PB1)", taxAmt, 0)
				add(glAccTaxPayable, at, date, "Pajak Restoran (PB1)", 0, taxAmt)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 6b) Uang muka reservasi.
	//     Masuk (dp/pelunasan tervalidasi): Dr Kas / Cr Uang Muka Pelanggan.
	//     Refund: Dr Uang Muka / Cr Kas. Hangus: Dr Uang Muka / Cr Pendapatan
	//     Lainnya. Dipakai di POS (baris 'reservasi_dp'): Dr Uang Muka / Cr Kas,
	//     mengimbangi Dr Kas penuh dari posting penjualan.
	if needQ(glAccCash, glAccCustomerDep, glAccOtherIncome) {
		q := `
		SELECT p.paid_at, TO_CHAR(tz_date(p.paid_at), 'YYYY-MM-DD') AS date, p.type,
			COALESCE(o.name, '-') || ' - ' || rv.customer_name AS description, p.amount
		FROM reservation_payments p
		JOIN reservations rv ON rv.id = p.reservation_id
		LEFT JOIN outlets o ON o.id = rv.outlet_id
		WHERE p.status = 'validated'
		  AND p.paid_at >= tz_day_start($1::date) AND p.paid_at < tz_day_start($2::date + 1)` +
			aliasFilter("p") + `
		ORDER BY p.paid_at`
		if err := eachRow("reservation deposits", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, typ, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &typ, &desc, &amount); err != nil {
				return err
			}
			if typ == "refund" {
				add(glAccCustomerDep, at, date, "Refund uang muka: "+desc, amount, 0)
				add(glAccCash, at, date, "Refund uang muka: "+desc, 0, amount)
			} else {
				add(glAccCash, at, date, "Uang muka reservasi ("+typ+"): "+desc, amount, 0)
				add(glAccCustomerDep, at, date, "Uang muka reservasi ("+typ+"): "+desc, 0, amount)
			}
			return nil
		}); err != nil {
			return nil, err
		}

		q = `
		SELECT rv.cancelled_at, TO_CHAR(tz_date(rv.cancelled_at), 'YYYY-MM-DD') AS date,
			COALESCE(o.name, '-') || ' - ' || rv.customer_name AS description,
			COALESCE((SELECT SUM(CASE WHEN p.type = 'refund' THEN -p.amount ELSE p.amount END)
			          FROM reservation_payments p WHERE p.reservation_id = rv.id AND p.status = 'validated'), 0)
		FROM reservations rv LEFT JOIN outlets o ON o.id = rv.outlet_id
		WHERE rv.status = 'cancelled' AND rv.cancel_disposition = 'hangus'
		  AND rv.cancelled_at >= tz_day_start($1::date) AND rv.cancelled_at < tz_day_start($2::date + 1)` +
			aliasFilter("rv") + `
		ORDER BY rv.cancelled_at`
		if err := eachRow("forfeited deposits", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &amount); err != nil {
				return err
			}
			if amount > 0 {
				add(glAccCustomerDep, at, date, "Uang muka hangus: "+desc, amount, 0)
				add(glAccOtherIncome, at, date, "Uang muka hangus: "+desc, 0, amount)
			}
			return nil
		}); err != nil {
			return nil, err
		}

		q = `
		SELECT transaction_payments.created_at, TO_CHAR(tz_date(transaction_payments.created_at), 'YYYY-MM-DD') AS date,
			COALESCE(NULLIF(t.orderer_name, ''), t.cashier_name, 'Kasir') || ' - ' || t.local_id AS description,
			transaction_payments.amount
		FROM transaction_payments
		JOIN cloud_transactions t ON t.id = transaction_payments.transaction_id
		WHERE transaction_payments.payment_method = '` + ReservationDpMethod + `'
		  AND transaction_payments.created_at >= tz_day_start($1::date) AND transaction_payments.created_at < tz_day_start($2::date + 1)` +
			payNotVoided + aliasFilter("transaction_payments") + `
		ORDER BY transaction_payments.created_at`
		if err := eachRow("deposit usage", q, args, func(rows *sql.Rows) error {
			var at time.Time
			var date, desc string
			var amount float64
			if err := rows.Scan(&at, &date, &desc, &amount); err != nil {
				return err
			}
			add(glAccCustomerDep, at, date, "Pemakaian uang muka: "+desc, amount, 0)
			add(glAccCash, at, date, "Pemakaian uang muka: "+desc, 0, amount)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// 7) Penyusutan aset — satu jurnal per bulan kalender dalam rentang,
	//    Dr Beban Penyusutan / Cr Aset Tetap (metode neto), rumus modul aset.
	if needQ(glAccDepreciation, glAccFixedAsset) {
		from, errF := time.Parse("2006-01-02", dateFrom)
		to, errT := time.Parse("2006-01-02", dateTo)
		if errF == nil && errT == nil && !to.Before(from) {
			loc := GetTimezoneLocation()
			for cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); !cur.After(to); cur = cur.AddDate(0, 1, 0) {
				mStart, mEnd := cur, cur.AddDate(0, 1, -1)
				if mStart.Before(from) {
					mStart = from
				}
				if mEnd.After(to) {
					mEnd = to
				}
				amt, _, err := depreciationForPeriod(mStart.Format("2006-01-02"), mEnd.Format("2006-01-02"), filterIDs)
				if err != nil {
					return nil, fmt.Errorf("general ledger: depreciation: %w", err)
				}
				if amt = round2(amt); amt > 0 {
					at := time.Date(mEnd.Year(), mEnd.Month(), mEnd.Day(), 23, 59, 59, 0, loc)
					desc := "Penyusutan aset " + cur.Format("2006-01")
					add(glAccDepreciation, at, mEnd.Format("2006-01-02"), desc, amt, 0)
					add(glAccFixedAsset, at, mEnd.Format("2006-01-02"), desc, 0, amt)
				}
			}
		}
	}

	// ── Susun akun dari entri ──
	accounts := make([]models.GeneralLedgerAccount, 0, len(glAccounts))
	balanceOf := make(map[string]float64, len(glAccounts))

	for _, meta := range glAccounts {
		if !wanted(meta.Code) {
			continue
		}
		raw := accountEntries[meta.Code]

		// Satu akun bisa disuplai beberapa query (mis. Kas dari penjualan,
		// kas masuk, dan pengadaan), jadi urutkan ulang secara kronologis.
		// Harus stabil: tanpa itu entri berwaktu sama diacak dan kolom saldo
		// berjalan berubah-ubah tiap request.
		sort.SliceStable(raw, func(i, j int) bool { return raw[i].At.Before(raw[j].At) })

		var acctDebit, acctCredit, balance float64
		isDebitNormal := meta.Group == "aset" || meta.Group == "beban"
		ledgerEntries := make([]models.GeneralLedgerEntry, 0, len(raw))

		for _, e := range raw {
			acctDebit += e.Debit
			acctCredit += e.Credit
			if isDebitNormal {
				balance += e.Debit - e.Credit
			} else {
				balance += e.Credit - e.Debit
			}
			ledgerEntries = append(ledgerEntries, models.GeneralLedgerEntry{
				Date:        e.Date,
				Description: e.Description,
				Debit:       round2(e.Debit),
				Credit:      round2(e.Credit),
				Balance:     round2(balance),
			})
		}

		balance = round2(balance)
		accounts = append(accounts, models.GeneralLedgerAccount{
			Code:        meta.Code,
			Name:        meta.Name,
			Group:       meta.Group,
			TotalDebit:  round2(acctDebit),
			TotalCredit: round2(acctCredit),
			Balance:     balance,
			Entries:     ledgerEntries,
		})
		balanceOf[meta.Code] = balance
	}

	return &models.GeneralLedgerResponse{
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		AccountAll: accountFilter == "",
		Summary: models.GeneralLedgerSummary{
			CashBalance:  balanceOf[glAccCash],
			TotalRevenue: round2(balanceOf[glAccRevenue] + balanceOf[glAccOtherIncome]),
			TotalExpense: round2(balanceOf[glAccCOGS] + balanceOf[glAccServiceExp] + balanceOf[glAccOpex] + balanceOf[glAccDepreciation] + balanceOf[glAccTaxExpense]),
		},
		Accounts: accounts,
	}, nil
}

func GetVoidReport(dateFrom, dateTo, outletID string, scopeIDs []string, page, limit int) (*models.VoidReport, error) {
	offset := (page - 1) * limit

	// Order dianggap void hanya jika voided_at ada DAN bukan empty string.
	// NULLIF mencegah error cast "" → timestamptz.
	conds := []string{"NULLIF(o.payment_info->>'voided_at','') IS NOT NULL"}
	args := []interface{}{}
	idx := 1

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("o.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	} else if scopeIDs != nil {
		conds = append(conds, fmt.Sprintf("o.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(scopeIDs))
		idx++
	}

	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("NULLIF(o.payment_info->>'voided_at','')::timestamptz >= $%d::timestamptz", idx))
		args = append(args, dateFrom+" 00:00:00")
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("NULLIF(o.payment_info->>'voided_at','')::timestamptz <= $%d::timestamptz", idx))
		args = append(args, dateTo+" 23:59:59")
		idx++
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	var totalAmount float64
	err := database.DB.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*), COALESCE(SUM(o.total_amount),0)
		FROM cloud_orders o %s`, where,
	), args...).Scan(&total, &totalAmount)
	if err != nil {
		return nil, err
	}

	dataArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(
		`SELECT o.id,
			COALESCE(out.name,''),
			COALESCE(o.table_number,''),
			COALESCE(o.customer_name,''),
			o.total_amount,
			COALESCE(o.items::text,'[]'),
			COALESCE(o.payment_info->>'voided_at',''),
			COALESCE(o.payment_info->>'voided_by',''),
			COALESCE(o.payment_info->>'void_reason',''),
			o.created_at::text
		FROM cloud_orders o
		LEFT JOIN outlets out ON out.id = o.outlet_id
		%s
		ORDER BY (o.payment_info->>'voided_at') DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1,
	), dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make([]models.VoidOrderRow, 0)
	for rows.Next() {
		var r models.VoidOrderRow
		if err := rows.Scan(&r.ID, &r.OutletName, &r.TableNumber, &r.CustomerName,
			&r.TotalAmount, &r.Items, &r.VoidedAt, &r.VoidedBy, &r.VoidReason, &r.CreatedAt); err != nil {
			return nil, err
		}
		data = append(data, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ── Void item (hapus item dari order belum bayar) ──────────────────────
	// Audit terpisah dari void transaksi: order tetap jalan, hanya item yang
	// dihapus. Ditampilkan sebagai tab sendiri di laporan void.
	iconds := []string{"v.voided_at IS NOT NULL"}
	iargs := []interface{}{}
	iidx := 1

	if outletID != "" {
		iconds = append(iconds, fmt.Sprintf("v.outlet_id = $%d", iidx))
		iargs = append(iargs, outletID)
		iidx++
	} else if scopeIDs != nil {
		iconds = append(iconds, fmt.Sprintf("v.outlet_id = ANY($%d::text[])", iidx))
		iargs = append(iargs, pq.Array(scopeIDs))
		iidx++
	}
	if dateFrom != "" {
		iconds = append(iconds, fmt.Sprintf("v.voided_at >= $%d::timestamptz", iidx))
		iargs = append(iargs, dateFrom+" 00:00:00")
		iidx++
	}
	if dateTo != "" {
		iconds = append(iconds, fmt.Sprintf("v.voided_at <= $%d::timestamptz", iidx))
		iargs = append(iargs, dateTo+" 23:59:59")
		iidx++
	}

	iwhere := "WHERE " + strings.Join(iconds, " AND ")

	var itemTotal int
	var itemAmount float64
	if err := database.DB.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*), COALESCE(SUM(v.subtotal),0)
		FROM order_item_voids v %s`, iwhere,
	), iargs...).Scan(&itemTotal, &itemAmount); err != nil {
		return nil, err
	}

	itemArgs := append(append([]interface{}{}, iargs...), limit, offset)
	irows, err := database.DB.Query(fmt.Sprintf(
		`SELECT v.id,
			COALESCE(out.name,''),
			COALESCE(v.order_id,''),
			COALESCE(v.table_number,''),
			COALESCE(v.product_name,''),
			v.qty, v.price, v.subtotal,
			COALESCE(v.waiter_name,''),
			COALESCE(v.voided_by,''),
			COALESCE(v.void_reason,''),
			COALESCE(to_char(v.voided_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')
		FROM order_item_voids v
		LEFT JOIN outlets out ON out.id = v.outlet_id
		%s
		ORDER BY v.voided_at DESC
		LIMIT $%d OFFSET $%d`, iwhere, iidx, iidx+1,
	), itemArgs...)
	if err != nil {
		return nil, err
	}
	defer irows.Close()

	items := make([]models.VoidItemRow, 0)
	for irows.Next() {
		var r models.VoidItemRow
		if err := irows.Scan(&r.ID, &r.OutletName, &r.OrderID, &r.TableNumber,
			&r.ProductName, &r.Qty, &r.Price, &r.Subtotal,
			&r.WaiterName, &r.VoidedBy, &r.VoidReason, &r.VoidedAt); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if err := irows.Err(); err != nil {
		return nil, err
	}

	return &models.VoidReport{
		Summary: models.VoidReportSummary{
			TotalVoided: total, TotalAmount: totalAmount,
			ItemVoided: itemTotal, ItemAmount: itemAmount,
		},
		Data:       data,
		Items:      items,
		Total:      total,
		ItemsTotal: itemTotal,
		Page:       page,
		Limit:      limit,
	}, nil
}

// GetDiscountReport aggregates order discounts and complimentary-item value.
// Reads optional fields the Flutter POS app may send in the order JSON:
//   payment_info.discount  → bill discount (total_amount is already net)
//   items[].discount       → per-line discount
//   items[].is_complimentary → free item (its subtotal counts as compliment value)
func GetDiscountReport(dateFrom, dateTo, outletID string, scopeIDs []string, page, limit int) (*models.DiscountReport, error) {
	offset := (page - 1) * limit

	conds := []string{"NULLIF(o.payment_info->>'voided_at','') IS NULL AND COALESCE(o.is_holding,false) = false"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("o.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	} else if scopeIDs != nil {
		conds = append(conds, fmt.Sprintf("o.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(scopeIDs))
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("o.created_at >= tz_day_start($%d::date)", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("o.created_at < tz_day_start($%d::date + 1)", idx))
		args = append(args, dateTo)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	// CTE computes per-order discount & compliment from the JSON.
	base := `
		WITH items_arr AS (
			SELECT o.id, o.outlet_id, o.customer_name, o.created_at, o.total_amount,
			       COALESCE(NULLIF(o.payment_info->>'discount','')::numeric, 0) AS order_discount,
			       CASE WHEN jsonb_typeof(o.items)='array' THEN o.items ELSE '[]'::jsonb END AS items
			FROM cloud_orders o ` + where + `
		),
		agg AS (
			SELECT b.id, b.outlet_id, b.customer_name, b.created_at, b.total_amount, b.order_discount,
			       COALESCE((SELECT SUM(COALESCE(NULLIF(it->>'discount','')::numeric,0))
			                 FROM jsonb_array_elements(b.items) it),0) AS item_discount,
			       COALESCE((SELECT SUM(COALESCE(NULLIF(it->>'subtotal','')::numeric,
			                              COALESCE(NULLIF(it->>'price','')::numeric,0)*COALESCE(NULLIF(it->>'qty','')::numeric,0)))
			                 FROM jsonb_array_elements(b.items) it
			                 WHERE lower(COALESCE(it->>'is_complimentary','')) IN ('true','1','t','yes')),0) AS compliment
			FROM items_arr b
		),
		rows AS (
			SELECT a.id, COALESCE(out.name,'') AS outlet_name, a.customer_name,
			       a.created_at, a.total_amount,
			       (a.order_discount + a.item_discount) AS discount, a.compliment
			FROM agg a LEFT JOIN outlets out ON out.id = a.outlet_id
			WHERE (a.order_discount + a.item_discount) > 0 OR a.compliment > 0
		)`

	// Summary
	var sum models.DiscountReportSummary
	if err := database.DB.QueryRow(base+`
		SELECT COUNT(*), COALESCE(SUM(total_amount),0), COALESCE(SUM(discount),0), COALESCE(SUM(compliment),0)
		FROM rows`, args...).Scan(&sum.TotalOrders, &sum.Net, &sum.Discount, &sum.Compliment); err != nil {
		return nil, err
	}
	sum.Gross = sum.Net + sum.Discount + sum.Compliment

	// Rows — created_at dikirim sebagai waktu lokal zona aplikasi (app_settings)
	// tanpa penanda zona, agar tampilan tidak bergantung zona waktu browser.
	const tz = "COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta')"
	dataArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := database.DB.Query(base+fmt.Sprintf(`
		SELECT id, outlet_name, customer_name, total_amount, discount, compliment,
		       TO_CHAR((created_at AT TIME ZONE 'UTC') AT TIME ZONE %s, 'YYYY-MM-DD"T"HH24:MI:SS')
		FROM rows ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, tz, idx, idx+1), dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make([]models.DiscountReportRow, 0)
	for rows.Next() {
		var r models.DiscountReportRow
		if err := rows.Scan(&r.ID, &r.OutletName, &r.CustomerName, &r.Net, &r.Discount, &r.Compliment, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Gross = r.Net + r.Discount + r.Compliment
		data = append(data, r)
	}

	return &models.DiscountReport{
		Summary: sum,
		Data:    data,
		Total:   sum.TotalOrders,
		Page:    page,
		Limit:   limit,
	}, nil
}

// GetTitipanReport — laporan Meja Titipan: log audit titip/tarik (per rentang
// tanggal) + ringkasan barang yang masih menggantung (order is_holding aktif).
func GetTitipanReport(dateFrom, dateTo, outletID string, scopeIDs []string, page, limit int) (*models.TitipanReport, error) {
	offset := (page - 1) * limit
	const tz = "COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta')"

	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("l.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	} else if scopeIDs != nil {
		conds = append(conds, fmt.Sprintf("l.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(scopeIDs))
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("tz_date(l.performed_at) >= $%d::date", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("tz_date(l.performed_at) <= $%d::date", idx))
		args = append(args, dateTo)
		idx++
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var sum models.TitipanReportSummary
	var total int
	if err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE action='park'), COALESCE(SUM(subtotal) FILTER (WHERE action='park'),0),
			COUNT(*) FILTER (WHERE action='pull'), COALESCE(SUM(subtotal) FILTER (WHERE action='pull'),0)
		FROM order_item_titipan_logs l %s`, where), args...).
		Scan(&total, &sum.ParkCount, &sum.ParkAmount, &sum.PullCount, &sum.PullAmount); err != nil {
		return nil, err
	}

	// Barang yang MASIH menggantung: order is_holding aktif (belum bayar, tidak void).
	holdConds := []string{"is_holding = true",
		"COALESCE(payment_info->>'payment_status','unpaid') NOT IN ('paid')",
		"NULLIF(payment_info->>'voided_at','') IS NULL"}
	holdArgs := []interface{}{}
	hidx := 1
	if outletID != "" {
		holdConds = append(holdConds, fmt.Sprintf("outlet_id = $%d", hidx))
		holdArgs = append(holdArgs, outletID)
		hidx++
	} else if scopeIDs != nil {
		holdConds = append(holdConds, fmt.Sprintf("outlet_id = ANY($%d::text[])", hidx))
		holdArgs = append(holdArgs, pq.Array(scopeIDs))
		hidx++
	}
	database.DB.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*), COALESCE(SUM(total_amount),0) FROM cloud_orders WHERE %s`,
		strings.Join(holdConds, " AND ")), holdArgs...).Scan(&sum.HoldingCount, &sum.HoldingValue)

	dataArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT l.id::text, COALESCE(o.name,''), l.local_id, l.action, l.product_name,
			l.qty, l.price, l.subtotal, l.source_table, l.target_table, l.performed_by,
			COALESCE(to_char((l.performed_at AT TIME ZONE 'UTC') AT TIME ZONE %s, 'YYYY-MM-DD HH24:MI'),'')
		FROM order_item_titipan_logs l
		LEFT JOIN outlets o ON o.id = l.outlet_id
		%s
		ORDER BY l.performed_at DESC NULLS LAST
		LIMIT $%d OFFSET $%d`, tz, where, idx, idx+1), dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make([]models.TitipanLogRow, 0)
	for rows.Next() {
		var r models.TitipanLogRow
		if err := rows.Scan(&r.ID, &r.OutletName, &r.LocalID, &r.Action, &r.ProductName,
			&r.Qty, &r.Price, &r.Subtotal, &r.SourceTable, &r.TargetTable, &r.PerformedBy, &r.PerformedAt); err != nil {
			return nil, err
		}
		data = append(data, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.TitipanReport{Summary: sum, Data: data, Total: total, Page: page, Limit: limit}, nil
}
