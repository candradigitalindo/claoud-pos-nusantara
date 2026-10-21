package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"log"
	"math"
	"strings"

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
	return financeCashFlow(dateFrom, dateTo, outletID, scopeIDs)
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
// untuk rentang tanggal, per outlet, termasuk riwayat unit yang telah dilepas.
// Beban periode = akumulasi(dateTo)
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
		       COALESCE(SUM((LEAST(LEAST(x.months_to, x.life) * x.base / x.life, x.cap)
		                    - LEAST(LEAST(x.months_from, x.life) * x.base / x.life, x.cap)) * x.quantity), 0)
		FROM (
			SELECT a.outlet_id, segment.quantity, segment.cap, a.useful_life_months AS life,
			       GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0) AS base,
			       GREATEST(0, (DATE_PART('year', AGE(LEAST($2::date,segment.stop_date), a.purchase_date)) * 12
			                   + DATE_PART('month', AGE(LEAST($2::date,segment.stop_date), a.purchase_date)))::int) AS months_to,
			       GREATEST(0, (DATE_PART('year', AGE(LEAST($1::date - 1,segment.stop_date), a.purchase_date)) * 12
			                   + DATE_PART('month', AGE(LEAST($1::date - 1,segment.stop_date), a.purchase_date)))::int) AS months_from
			FROM assets a
			CROSS JOIN LATERAL (
				SELECT a.quantity, 'infinity'::date stop_date,
				       GREATEST(a.purchase_price-COALESCE(a.residual_value,0),0) cap
				WHERE COALESCE(a.status,'aktif')<>'dihapus' AND a.quantity>0
				UNION ALL
				SELECT d.qty, tz_date(d.created_at),
				       GREATEST(a.purchase_price-d.book_value/d.qty,0)
				FROM asset_disposals d WHERE d.asset_id=a.id AND d.status='approved' AND d.qty>0
			) segment
			WHERE a.is_deleted = false
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
	return financeBalance(dateFrom, dateTo, outletID, scopeIDs)
}

func GetProfitLossReport(dateFrom, dateTo, outletID string, scopeIDs []string) (*models.ProfitLossResponse, error) {
	return financeProfitLoss(dateFrom, dateTo, outletID, scopeIDs)
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
	return financeLedger(dateFrom, dateTo, outletID, accountFilter, scopeIDs)
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
//
//	payment_info.discount  → bill discount (total_amount is already net)
//	items[].discount       → per-line discount
//	items[].is_complimentary → free item (its subtotal counts as compliment value)
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
