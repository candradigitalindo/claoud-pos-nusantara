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
