package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

// AssetSummary — angka ringkasan untuk kartu di halaman Laporan Aset.
type AssetSummary struct {
	AssetCount      int     `json:"asset_count"`
	UnitCount       int     `json:"unit_count"`
	AcquisitionCost float64 `json:"acquisition_cost"`
	BookValue       float64 `json:"book_value"`
	Depreciation    float64 `json:"depreciation"`
	Damaged         int     `json:"damaged"`
	UnderRepair     int     `json:"under_repair"`
	InTransit       int     `json:"in_transit"`
	DisposedValue   float64 `json:"disposed_value"`
	MaintenanceCost float64 `json:"maintenance_cost_year"`
	IncompleteLines int     `json:"incomplete_lines"`
}

func GetAssetSummary(outletScope []string) (*AssetSummary, error) {
	scope := ""
	args := []interface{}{}
	if outletScope != nil {
		scope = " AND a.outlet_id = ANY($1::text[])"
		args = append(args, pq.Array(outletScope))
	}
	var s AssetSummary
	// Nilai perolehan dan nilai buku dikali jumlah unit: harga tersimpan per unit.
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(a.quantity), 0),
		       COALESCE(SUM(a.purchase_price * a.quantity), 0),
		       COALESCE(SUM((a.purchase_price - COALESCE(dep.accum, 0)) * a.quantity), 0),
		       COALESCE(SUM(COALESCE(dep.accum, 0) * a.quantity), 0),
		       COUNT(*) FILTER (WHERE a.condition <> 'baik'),
		       COUNT(*) FILTER (WHERE COALESCE(a.status,'aktif') = 'perbaikan'),
		       COUNT(*) FILTER (WHERE COALESCE(a.status,'aktif') = 'transit')
		FROM assets a %s
		WHERE a.is_deleted = false AND COALESCE(a.status,'aktif') <> 'dihapus'%s`,
		assetJoins, scope), args...).
		Scan(&s.AssetCount, &s.UnitCount, &s.AcquisitionCost, &s.BookValue, &s.Depreciation,
			&s.Damaged, &s.UnderRepair, &s.InTransit)
	if err != nil {
		return nil, err
	}
	database.DB.QueryRow(fmt.Sprintf(`
		SELECT COALESCE(SUM(d.book_value), 0) FROM asset_disposals d
		JOIN assets a ON a.id = d.asset_id
		WHERE d.status = 'approved'%s`, scope), args...).Scan(&s.DisposedValue)
	database.DB.QueryRow(fmt.Sprintf(`
		SELECT COALESCE(SUM(m.cost), 0) FROM asset_maintenances m
		JOIN assets a ON a.id = m.asset_id
		WHERE m.status = 'selesai' AND m.maintenance_date >= date_trunc('year', %s)%s`,
		appTodayExpr(), scope), args...).Scan(&s.MaintenanceCost)
	if rows, err := ListIncompleteReceipts(outletScope, "perlengkapan"); err == nil {
		s.IncompleteLines = len(rows)
	}
	return &s, nil
}

// AssetReportRow adalah satu baris laporan — bentuknya sengaja seragam untuk
// kelima jenis laporan supaya tabel dan export memakai jalur yang sama.
type AssetReportRow struct {
	Cells []string `json:"cells"`
}

type AssetReport struct {
	Type    string           `json:"type"`
	Title   string           `json:"title"`
	Headers []string         `json:"headers"`
	Rows    []AssetReportRow `json:"rows"`
	Totals  map[string]float64 `json:"totals,omitempty"`
}

var assetReportTitles = map[string]string{
	"daftar":       "Daftar Aset (KIB)",
	"penyusutan":   "Penyusutan & Nilai Buku",
	"perawatan":    "Biaya Perawatan",
	"mutasi":       "Mutasi Aset",
	"belum-lengkap": "Pengadaan Diterima, Barang Belum Lengkap",
}

func money(v float64) string { return fmt.Sprintf("%.0f", v) }

// BuildAssetReport menyusun salah satu dari lima laporan aset.
func BuildAssetReport(reportType, from, to string, outletScope []string) (*AssetReport, error) {
	title, ok := assetReportTitles[reportType]
	if !ok {
		return nil, fmt.Errorf("jenis laporan '%s' tidak dikenal", reportType)
	}
	rep := &AssetReport{Type: reportType, Title: title, Rows: []AssetReportRow{}, Totals: map[string]float64{}}
	scope := ""
	args := []interface{}{}
	idx := 1
	if outletScope != nil {
		scope = fmt.Sprintf(" AND a.outlet_id = ANY($%d::text[])", idx)
		args = append(args, pq.Array(outletScope))
		idx++
	}

	switch reportType {
	case "daftar":
		rep.Headers = []string{"Nomor Aset", "Nama", "Kategori", "Outlet", "Lokasi", "Penanggung Jawab",
			"Jumlah", "Satuan", "Kondisi", "Status", "Tgl Perolehan", "Harga Perolehan"}
		rows, err := database.DB.Query(fmt.Sprintf(`
			SELECT COALESCE(a.asset_no,''), a.name, COALESCE(a.category,''), COALESCE(o.name,''),
			       COALESCE(a.location,''), COALESCE(a.pic_name,''), a.quantity, a.unit,
			       a.condition, COALESCE(a.status,'aktif'),
			       COALESCE(TO_CHAR(a.purchase_date,'YYYY-MM-DD'),''), a.purchase_price
			FROM assets a LEFT JOIN outlets o ON o.id = a.outlet_id
			WHERE a.is_deleted = false AND COALESCE(a.status,'aktif') <> 'dihapus'%s
			ORDER BY o.name, a.name`, scope), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var no, name, cat, outlet, loc, pic, unit, cond, status, pdate string
			var qty int
			var price float64
			if err := rows.Scan(&no, &name, &cat, &outlet, &loc, &pic, &qty, &unit, &cond, &status, &pdate, &price); err != nil {
				return nil, err
			}
			rep.Totals["nilai_perolehan"] += price * float64(qty)
			rep.Rows = append(rep.Rows, AssetReportRow{Cells: []string{no, name, cat, outlet, loc, pic,
				fmt.Sprintf("%d", qty), unit, cond, status, pdate, money(price)}})
		}

	case "penyusutan":
		rep.Headers = []string{"Nomor Aset", "Nama", "Outlet", "Tgl Perolehan", "Harga Perolehan",
			"Umur (bln)", "Bulan Berjalan", "Akumulasi Penyusutan", "Nilai Buku"}
		rows, err := database.DB.Query(fmt.Sprintf(`
			SELECT COALESCE(a.asset_no,''), a.name, COALESCE(o.name,''),
			       COALESCE(TO_CHAR(a.purchase_date,'YYYY-MM-DD'),''), a.purchase_price * a.quantity,
			       COALESCE(a.useful_life_months,0), COALESCE(dep.months,0)::int,
			       COALESCE(dep.accum,0) * a.quantity,
			       (a.purchase_price - COALESCE(dep.accum,0)) * a.quantity
			FROM assets a %s
			WHERE a.is_deleted = false AND COALESCE(a.status,'aktif') <> 'dihapus'%s
			ORDER BY (a.purchase_price * a.quantity) DESC`, assetJoins, scope), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var no, name, outlet, pdate string
			var price, accum, book float64
			var life, months int
			if err := rows.Scan(&no, &name, &outlet, &pdate, &price, &life, &months, &accum, &book); err != nil {
				return nil, err
			}
			rep.Totals["perolehan"] += price
			rep.Totals["penyusutan"] += accum
			rep.Totals["nilai_buku"] += book
			rep.Rows = append(rep.Rows, AssetReportRow{Cells: []string{no, name, outlet, pdate, money(price),
				fmt.Sprintf("%d", life), fmt.Sprintf("%d", months), money(accum), money(book)}})
		}

	case "perawatan":
		rep.Headers = []string{"Nomor WO", "Tanggal", "Aset", "Outlet", "Jenis", "Pekerjaan",
			"Pelaksana", "Biaya", "Downtime (jam)"}
		cond := ""
		if from != "" {
			cond += fmt.Sprintf(" AND m.maintenance_date >= $%d::date", idx)
			args = append(args, from)
			idx++
		}
		if to != "" {
			cond += fmt.Sprintf(" AND m.maintenance_date <= $%d::date", idx)
			args = append(args, to)
			idx++
		}
		rows, err := database.DB.Query(fmt.Sprintf(`
			SELECT COALESCE(m.wo_number,''), TO_CHAR(m.maintenance_date,'YYYY-MM-DD'), COALESCE(a.name,''),
			       COALESCE(o.name,''), m.type, m.description, COALESCE(m.performed_by,''),
			       m.cost, COALESCE(m.downtime_hours,0)
			FROM asset_maintenances m
			LEFT JOIN assets a ON a.id = m.asset_id
			LEFT JOIN outlets o ON o.id = a.outlet_id
			WHERE COALESCE(m.status,'selesai') = 'selesai'%s%s
			ORDER BY m.maintenance_date DESC`, scope, cond), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var wo, date, asset, outlet, mtype, desc, by string
			var cost, downtime float64
			if err := rows.Scan(&wo, &date, &asset, &outlet, &mtype, &desc, &by, &cost, &downtime); err != nil {
				return nil, err
			}
			rep.Totals["biaya"] += cost
			rep.Totals["downtime"] += downtime
			rep.Rows = append(rep.Rows, AssetReportRow{Cells: []string{wo, date, asset, outlet, mtype, desc, by,
				money(cost), fmt.Sprintf("%.1f", downtime)}})
		}

	case "mutasi":
		rep.Headers = []string{"Nomor", "Tanggal Kirim", "Dari", "Ke", "Aset", "Dikirim", "Diterima", "Status"}
		rows, err := database.DB.Query(fmt.Sprintf(`
			SELECT t.transfer_number, COALESCE(TO_CHAR(t.sent_at,'YYYY-MM-DD'),''),
			       COALESCE(fo.name,''), COALESCE(t_o.name,''), COALESCE(a.name,''),
			       i.qty, COALESCE(i.received_qty, 0), t.status
			FROM asset_transfer_items i
			JOIN asset_transfers t ON t.id = i.transfer_id
			LEFT JOIN assets a ON a.id = i.asset_id
			LEFT JOIN outlets fo ON fo.id = t.from_outlet_id
			LEFT JOIN outlets t_o ON t_o.id = t.to_outlet_id
			WHERE t.status IN ('sent','received')%s
			ORDER BY t.sent_at DESC NULLS LAST`,
			strings.Replace(scope, "a.outlet_id", "COALESCE(t.from_outlet_id, t.to_outlet_id)", 1)), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var num, sent, fromO, toO, asset, status string
			var qty, recv int
			if err := rows.Scan(&num, &sent, &fromO, &toO, &asset, &qty, &recv, &status); err != nil {
				return nil, err
			}
			rep.Rows = append(rep.Rows, AssetReportRow{Cells: []string{num, sent, fromO, toO, asset,
				fmt.Sprintf("%d", qty), fmt.Sprintf("%d", recv), status}})
		}

	case "belum-lengkap":
		rep.Headers = []string{"Nomor Pengadaan", "Diterima", "Outlet", "Vendor", "Entri", "Barang",
			"Dibeli", "Tercatat", "Kurang", "Nilai Kurang"}
		// Laporan di modul aset hanya memuat perlengkapan — bahan dapur yang
		// belum masuk buku stok adalah pekerjaan gudang, bukan tim aset.
		list, err := ListIncompleteReceipts(outletScope, "perlengkapan")
		if err != nil {
			return nil, err
		}
		for _, r := range list {
			rep.Totals["nilai_kurang"] += r.MissingValue
			rep.Rows = append(rep.Rows, AssetReportRow{Cells: []string{r.RequestNumber, r.ReceivedAt,
				r.OutletName, r.VendorName, r.EntryName, r.Name, fmt.Sprintf("%d", r.Qty),
				fmt.Sprintf("%d", r.Recorded), fmt.Sprintf("%d", r.Missing), money(r.MissingValue)}})
		}
	}
	return rep, nil
}

// ExportAssetReport membungkus laporan menjadi berkas Excel.
func ExportAssetReport(reportType, from, to, actor string, outletScope []string) (*bytes.Buffer, string, error) {
	rep, err := BuildAssetReport(reportType, from, to, outletScope)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := "Laporan"
	f.SetSheetName("Sheet1", sheet)

	now := time.Now().In(GetTimezoneLocation())
	f.SetCellValue(sheet, "A1", rep.Title)
	periode := "Seluruh data"
	if from != "" || to != "" {
		periode = strings.TrimSpace(from + " s/d " + to)
	}
	f.SetCellValue(sheet, "A2", "Periode  : "+periode)
	f.SetCellValue(sheet, "A3", fmt.Sprintf("Dicetak  : %s oleh %s · Sumber: Cloud POS — Modul Perlengkapan",
		now.Format("2006-01-02 15:04"), actor))

	headRow := 5
	for i, h := range rep.Headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headRow)
		f.SetCellValue(sheet, cell, h)
	}
	for r, row := range rep.Rows {
		for c, v := range row.Cells {
			cell, _ := excelize.CoordinatesToCellName(c+1, headRow+1+r)
			// Angka ditulis sebagai angka supaya bisa dijumlah di Excel.
			var num float64
			if _, err := fmt.Sscanf(v, "%f", &num); err == nil && v != "" && isNumeric(v) {
				f.SetCellValue(sheet, cell, num)
			} else {
				f.SetCellValue(sheet, cell, v)
			}
		}
	}
	if len(rep.Totals) > 0 {
		row := headRow + len(rep.Rows) + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "TOTAL")
		i := 0
		for k, v := range rep.Totals {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row+1+i), k)
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row+1+i), v)
			i++
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("Laporan-Aset_%s_%s.xlsx", reportType, now.Format("20060102-1504"))
	return buf, filename, nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if (r == '-' || r == '+') && i == 0 {
			continue
		}
		if r == '.' {
			continue
		}
		return false
	}
	return true
}

var _ = models.Asset{}

// ── Dashboard aset ──────────────────────────────────────────────────────────

// AssetDashboard adalah satu layar yang menjawab pertanyaan harian pengelola
// aset: berapa nilainya, apa yang perlu ditindak hari ini, dan ke mana uang
// perawatan pergi.
type AssetDashboard struct {
	Summary      *AssetSummary        `json:"summary"`
	ByCategory   []AssetCategoryValue `json:"by_category"`
	ByStatus     []AssetCountBucket   `json:"by_status"`
	ByCondition  []AssetCountBucket   `json:"by_condition"`
	ByOutlet     []AssetCategoryValue `json:"by_outlet"`
	MaintTrend   []AssetMonthValue    `json:"maintenance_trend"`
	AgeBuckets   []AssetCountBucket   `json:"age_buckets"`
	Actions      AssetActionCounts    `json:"actions"`
	TopMaintenance []AssetTopCost     `json:"top_maintenance"`
}

type AssetCategoryValue struct {
	Label       string  `json:"label"`
	Count       int     `json:"count"`
	Units       int     `json:"units"`
	Acquisition float64 `json:"acquisition"`
	BookValue   float64 `json:"book_value"`
}

type AssetCountBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type AssetMonthValue struct {
	Month string  `json:"month"`
	Cost  float64 `json:"cost"`
	Count int     `json:"count"`
}

// AssetTopCost menyorot aset yang biaya perawatannya sudah mendekati atau
// melewati harga belinya — kandidat diganti, bukan diperbaiki lagi (§7.5).
type AssetTopCost struct {
	AssetID     string  `json:"asset_id"`
	AssetNo     string  `json:"asset_no"`
	Name        string  `json:"name"`
	OutletName  string  `json:"outlet_name"`
	Acquisition float64 `json:"acquisition"`
	MaintCost   float64 `json:"maint_cost"`
	Ratio       float64 `json:"ratio"` // persen
}

// AssetActionCounts adalah pekerjaan yang menunggu — angka yang seharusnya nol.
type AssetActionCounts struct {
	MaintenanceOverdue  int `json:"maintenance_overdue"`
	MaintenanceDueWeek  int `json:"maintenance_due_week"`
	TransfersToReceive  int `json:"transfers_to_receive"`
	TransfersToApprove  int `json:"transfers_to_approve"`
	DisposalsPending    int `json:"disposals_pending"`
	OpnameRunning       int `json:"opname_running"`
	AwaitingHandover    int `json:"awaiting_handover"`
	IncompleteReceipts  int `json:"incomplete_receipts"`
	PhotosPendingBackup int `json:"photos_pending_backup"`
}

func GetAssetDashboard(outletScope []string) (*AssetDashboard, error) {
	d := &AssetDashboard{
		ByCategory: []AssetCategoryValue{}, ByStatus: []AssetCountBucket{},
		ByCondition: []AssetCountBucket{}, ByOutlet: []AssetCategoryValue{},
		MaintTrend: []AssetMonthValue{}, AgeBuckets: []AssetCountBucket{},
		TopMaintenance: []AssetTopCost{},
	}
	sum, err := GetAssetSummary(outletScope)
	if err != nil {
		return nil, err
	}
	d.Summary = sum

	scope, args := "", []interface{}{}
	if outletScope != nil {
		scope = " AND a.outlet_id = ANY($1::text[])"
		args = append(args, pq.Array(outletScope))
	}
	live := `a.is_deleted = false AND COALESCE(a.status,'aktif') <> 'dihapus'`

	// Komposisi nilai per kategori — ke mana uang aset tertanam.
	if rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT COALESCE(NULLIF(a.category,''), 'Tanpa kategori'), COUNT(*), COALESCE(SUM(a.quantity),0),
		       COALESCE(SUM(a.purchase_price * a.quantity),0),
		       COALESCE(SUM((a.purchase_price - COALESCE(dep.accum,0)) * a.quantity),0)
		FROM assets a %s WHERE %s%s
		GROUP BY 1 ORDER BY 4 DESC LIMIT 12`, assetJoins, live, scope), args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var c AssetCategoryValue
			if rows.Scan(&c.Label, &c.Count, &c.Units, &c.Acquisition, &c.BookValue) == nil {
				d.ByCategory = append(d.ByCategory, c)
			}
		}
	}

	// Sebaran per outlet.
	if rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT COALESCE(NULLIF(o.name,''), 'Tanpa outlet'), COUNT(*), COALESCE(SUM(a.quantity),0),
		       COALESCE(SUM(a.purchase_price * a.quantity),0),
		       COALESCE(SUM((a.purchase_price - COALESCE(dep.accum,0)) * a.quantity),0)
		FROM assets a %s WHERE %s%s
		GROUP BY 1 ORDER BY 4 DESC LIMIT 12`, assetJoins, live, scope), args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var c AssetCategoryValue
			if rows.Scan(&c.Label, &c.Count, &c.Units, &c.Acquisition, &c.BookValue) == nil {
				d.ByOutlet = append(d.ByOutlet, c)
			}
		}
	}

	// Status & kondisi.
	for _, q := range []struct {
		expr string
		dest *[]AssetCountBucket
	}{
		{"COALESCE(a.status,'aktif')", &d.ByStatus},
		{"a.condition", &d.ByCondition},
	} {
		if rows, err := database.DB.Query(fmt.Sprintf(`
			SELECT %s, COUNT(*) FROM assets a WHERE %s%s GROUP BY 1 ORDER BY 2 DESC`,
			q.expr, live, scope), args...); err == nil {
			for rows.Next() {
				var b AssetCountBucket
				if rows.Scan(&b.Label, &b.Count) == nil {
					*q.dest = append(*q.dest, b)
				}
			}
			rows.Close()
		}
	}

	// Umur aset — menunjukkan gelombang penggantian yang akan datang.
	if rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT CASE
		         WHEN a.purchase_date IS NULL THEN 'Tanpa tanggal'
		         WHEN a.purchase_date > %[1]s - INTERVAL '1 year'  THEN '< 1 tahun'
		         WHEN a.purchase_date > %[1]s - INTERVAL '3 years' THEN '1–3 tahun'
		         WHEN a.purchase_date > %[1]s - INTERVAL '5 years' THEN '3–5 tahun'
		         ELSE '> 5 tahun' END,
		       COUNT(*)
		FROM assets a WHERE %[2]s%[3]s GROUP BY 1`, appTodayExpr(), live, scope), args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var b AssetCountBucket
			if rows.Scan(&b.Label, &b.Count) == nil {
				d.AgeBuckets = append(d.AgeBuckets, b)
			}
		}
	}

	// Tren biaya perawatan 12 bulan terakhir, termasuk bulan tanpa biaya —
	// deret yang bolong membuat grafiknya berbohong.
	mScope := strings.Replace(scope, "$1", "$1", 1)
	if rows, err := database.DB.Query(fmt.Sprintf(`
		WITH bulan AS (
			SELECT TO_CHAR(generate_series(
				date_trunc('month', %[1]s) - INTERVAL '11 months',
				date_trunc('month', %[1]s), INTERVAL '1 month'), 'YYYY-MM') AS m
		)
		SELECT bulan.m,
		       COALESCE(SUM(mt.cost), 0),
		       COUNT(mt.id)
		FROM bulan
		LEFT JOIN asset_maintenances mt ON TO_CHAR(mt.maintenance_date, 'YYYY-MM') = bulan.m
			AND COALESCE(mt.status,'selesai') = 'selesai'
		LEFT JOIN assets a ON a.id = mt.asset_id
		WHERE mt.id IS NULL OR (a.is_deleted = false%[2]s)
		GROUP BY bulan.m ORDER BY bulan.m`, appTodayExpr(), mScope), args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var v AssetMonthValue
			if rows.Scan(&v.Month, &v.Cost, &v.Count) == nil {
				d.MaintTrend = append(d.MaintTrend, v)
			}
		}
	}

	// Aset dengan biaya perawatan terbesar dibanding harga belinya.
	if rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT a.id, COALESCE(a.asset_no,''), a.name, COALESCE(o.name,''),
		       a.purchase_price * a.quantity, COALESCE(mc.total, 0),
		       CASE WHEN a.purchase_price * a.quantity > 0
		            THEN COALESCE(mc.total,0) / (a.purchase_price * a.quantity) * 100 ELSE 0 END
		FROM assets a
		LEFT JOIN outlets o ON o.id = a.outlet_id
		JOIN (SELECT asset_id, SUM(cost) AS total FROM asset_maintenances
		      WHERE COALESCE(status,'selesai')='selesai' GROUP BY asset_id) mc ON mc.asset_id = a.id
		WHERE %s%s AND mc.total > 0
		ORDER BY 7 DESC LIMIT 5`, live, scope), args...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var t AssetTopCost
			if rows.Scan(&t.AssetID, &t.AssetNo, &t.Name, &t.OutletName,
				&t.Acquisition, &t.MaintCost, &t.Ratio) == nil {
				d.TopMaintenance = append(d.TopMaintenance, t)
			}
		}
	}

	// Pekerjaan yang menunggu.
	today := appTodayExpr()
	database.DB.QueryRow(fmt.Sprintf(`
		SELECT
		  (SELECT COUNT(*) FROM asset_maintenances m JOIN assets a ON a.id=m.asset_id
		     WHERE m.status='dijadwalkan' AND m.scheduled_date < %[1]s AND a.is_deleted=false%[2]s),
		  (SELECT COUNT(*) FROM asset_maintenances m JOIN assets a ON a.id=m.asset_id
		     WHERE m.status='dijadwalkan' AND m.scheduled_date BETWEEN %[1]s AND %[1]s + 7 AND a.is_deleted=false%[2]s),
		  (SELECT COUNT(*) FROM asset_transfers t WHERE t.status='sent'),
		  (SELECT COUNT(*) FROM asset_transfers t WHERE t.status='pending'),
		  (SELECT COUNT(*) FROM asset_disposals WHERE status='pending'),
		  (SELECT COUNT(*) FROM asset_opname_sessions WHERE status='berjalan'),
		  (SELECT COUNT(*) FROM assets a WHERE a.is_deleted=false AND COALESCE(a.status,'aktif')='aktif'
		     AND COALESCE(a.pic_name,'')='' 
		     AND NOT EXISTS (SELECT 1 FROM asset_handover_items hi WHERE hi.asset_id=a.id)%[2]s),
		  (SELECT COUNT(*) FROM handover_photos WHERE backup_status='pending')`,
		today, scope), args...).
		Scan(&d.Actions.MaintenanceOverdue, &d.Actions.MaintenanceDueWeek,
			&d.Actions.TransfersToReceive, &d.Actions.TransfersToApprove,
			&d.Actions.DisposalsPending, &d.Actions.OpnameRunning,
			&d.Actions.AwaitingHandover, &d.Actions.PhotosPendingBackup)
	d.Actions.IncompleteReceipts = sum.IncompleteLines

	return d, nil
}
