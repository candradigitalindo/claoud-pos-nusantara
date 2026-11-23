package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

// Label tampilan — sama dengan ui/src/utils/assets.js, supaya berkas Excel
// memakai istilah yang sama dengan layar, bukan kode internal.
var assetConditionLabels = map[string]string{
	"baik": "Baik", "rusak_ringan": "Rusak Ringan", "rusak_berat": "Rusak Berat",
}

var assetStatusLabels = map[string]string{
	"aktif": "Aktif", "dipinjam": "Dipinjam", "perbaikan": "Dalam Perbaikan",
	"transit": "Dalam Perjalanan", "tidak_aktif": "Tidak Dipakai", "dihapus": "Dihapus",
}

var assetTrackingLabels = map[string]string{"massal": "Massal", "tunggal": "Tunggal"}

func assetLabel(labels map[string]string, key string) string {
	if v, ok := labels[key]; ok {
		return v
	}
	return key
}

type assetExportKind int

const (
	assetColText assetExportKind = iota
	assetColNumber
	assetColMoney
)

// assetExportColumn: satu kolom berkas. total != nil = kolom ikut dijumlah di
// baris TOTAL.
type assetExportColumn struct {
	header string
	width  float64
	kind   assetExportKind
	value  func(a models.Asset) interface{}
	total  func(a models.Asset) float64
}

// assetQtyForValue: harga tersimpan per unit; jumlah 0 dihitung 1 unit —
// rumus yang sama dengan kartu ringkasan (SummarizeAssets).
func assetQtyForValue(a models.Asset) float64 {
	if a.Quantity == 0 {
		return 1
	}
	return float64(a.Quantity)
}

// Kolom pertama s/d Nama dibekukan saat digulir ke kanan (lihat assetExportFrozenCols).
var assetExportColumns = []assetExportColumn{
	{header: "No", width: 6, kind: assetColNumber},
	{header: "Nomor Aset", width: 20, value: func(a models.Asset) interface{} { return a.AssetNo }},
	{header: "Nama Perlengkapan", width: 32, value: func(a models.Asset) interface{} { return a.Name }},
	{header: "Kode / Tag", width: 14, value: func(a models.Asset) interface{} { return a.Code }},
	{header: "Kategori", width: 18, value: func(a models.Asset) interface{} { return a.Category }},
	{header: "Outlet", width: 22, value: func(a models.Asset) interface{} { return a.OutletName }},
	{header: "Lokasi / Ruang", width: 20, value: func(a models.Asset) interface{} { return a.Location }},
	{header: "Penanggung Jawab", width: 18, value: func(a models.Asset) interface{} { return a.PicName }},
	{header: "Merk", width: 14, value: func(a models.Asset) interface{} { return a.Brand }},
	{header: "Tipe / Model", width: 14, value: func(a models.Asset) interface{} { return a.Model }},
	{header: "Nomor Seri", width: 18, value: func(a models.Asset) interface{} { return a.SerialNumber }},
	{header: "Mode", width: 10, value: func(a models.Asset) interface{} { return assetLabel(assetTrackingLabels, a.TrackingMode) }},
	{header: "Jumlah", width: 9, kind: assetColNumber,
		value: func(a models.Asset) interface{} { return a.Quantity },
		total: func(a models.Asset) float64 { return float64(a.Quantity) }},
	{header: "Satuan", width: 9, value: func(a models.Asset) interface{} { return a.Unit }},
	{header: "Kondisi", width: 13, value: func(a models.Asset) interface{} { return assetLabel(assetConditionLabels, a.Condition) }},
	{header: "Status", width: 17, value: func(a models.Asset) interface{} { return assetLabel(assetStatusLabels, a.Status) }},
	{header: "Tgl Perolehan", width: 13, value: func(a models.Asset) interface{} { return a.PurchaseDate }},
	{header: "Harga Perolehan / Unit", width: 16, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.PurchasePrice }},
	{header: "Umur Ekonomis (bln)", width: 11, kind: assetColNumber,
		value: func(a models.Asset) interface{} { return a.UsefulLifeMonths }},
	{header: "Nilai Residu / Unit", width: 14, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.ResidualValue }},
	{header: "Akumulasi Penyusutan / Unit", width: 16, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.Depreciation }},
	{header: "Nilai Buku / Unit", width: 16, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.BookValue }},
	{header: "Total Nilai Perolehan", width: 18, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.PurchasePrice * assetQtyForValue(a) },
		total: func(a models.Asset) float64 { return a.PurchasePrice * assetQtyForValue(a) }},
	{header: "Total Nilai Buku", width: 18, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.BookValue * assetQtyForValue(a) },
		total: func(a models.Asset) float64 { return a.BookValue * assetQtyForValue(a) }},
	{header: "Garansi s/d", width: 12, value: func(a models.Asset) interface{} { return a.WarrantyUntil }},
	{header: "Jml Perawatan", width: 10, kind: assetColNumber,
		value: func(a models.Asset) interface{} { return a.MaintenanceCount },
		total: func(a models.Asset) float64 { return float64(a.MaintenanceCount) }},
	{header: "Perawatan Terakhir", width: 13, value: func(a models.Asset) interface{} { return a.LastMaintenance }},
	{header: "Total Biaya Perawatan", width: 16, kind: assetColMoney,
		value: func(a models.Asset) interface{} { return a.MaintenanceCost },
		total: func(a models.Asset) float64 { return a.MaintenanceCost }},
	{header: "Catatan", width: 30, value: func(a models.Asset) interface{} { return a.Notes }},
}

const assetExportFrozenCols = 3 // No, Nomor Aset, Nama Perlengkapan

// describeAssetFilter menuliskan saringan yang dipakai, supaya berkas yang
// beredar tetap jelas isinya walau dibuka tanpa melihat layarnya.
func describeAssetFilter(f AssetListFilter) string {
	parts := []string{}
	switch {
	case f.OutletID != "":
		name := ""
		database.DB.QueryRow(`SELECT name FROM outlets WHERE id = $1`, f.OutletID).Scan(&name)
		if name == "" {
			name = f.OutletID
		}
		parts = append(parts, "Outlet "+name)
	case f.OutletScope != nil:
		parts = append(parts, "Semua outlet dalam akses Anda")
	default:
		parts = append(parts, "Semua outlet")
	}
	if f.Status != "" {
		parts = append(parts, "Status "+assetLabel(assetStatusLabels, f.Status))
	} else {
		parts = append(parts, "Semua status kecuali Dihapus")
	}
	if f.Condition != "" {
		parts = append(parts, "Kondisi "+assetLabel(assetConditionLabels, f.Condition))
	}
	if f.Search != "" {
		parts = append(parts, fmt.Sprintf("Kata kunci %q", f.Search))
	}
	return strings.Join(parts, " · ")
}

// ExportAssetList menyusun Excel daftar Perlengkapan: SEMUA aset yang lolos
// saringan — tanpa batas 100 baris per halaman di layar — dengan urutan yang
// sama dengan layar. Baris TOTAL memakai rumus yang sama dengan kartu
// ringkasan, jadi angkanya cocok dengan yang tampil di halaman.
func ExportAssetList(f AssetListFilter, actor string) (*bytes.Buffer, string, error) {
	assets, err := ListAssets(f)
	if err != nil {
		return nil, "", err
	}

	x := excelize.NewFile()
	defer x.Close()
	sheet := "Perlengkapan"
	if err := x.SetSheetName("Sheet1", sheet); err != nil {
		return nil, "", err
	}

	border := []excelize.Border{
		{Type: "left", Color: "D1D5DB", Style: 1},
		{Type: "right", Color: "D1D5DB", Style: 1},
		{Type: "top", Color: "D1D5DB", Style: 1},
		{Type: "bottom", Color: "D1D5DB", Style: 1},
	}
	numFmt := "#,##0"
	totalFill := excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ECFDF5"}}
	styles := map[string]*excelize.Style{
		"title":  {Font: &excelize.Font{Bold: true, Size: 14}},
		"info":   {Font: &excelize.Font{Color: "374151"}},
		"header": {Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"059669"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}, Border: border},
		"text":   {Border: border, Alignment: &excelize.Alignment{Vertical: "top"}},
		"num":    {Border: border, CustomNumFmt: &numFmt, Alignment: &excelize.Alignment{Vertical: "top"}},
		"label":  {Font: &excelize.Font{Bold: true}, Fill: totalFill, Border: border},
		"total":  {Font: &excelize.Font{Bold: true}, Fill: totalFill, Border: border, CustomNumFmt: &numFmt},
		"empty":  {Font: &excelize.Font{Italic: true, Color: "6B7280"}},
	}
	st := map[string]int{}
	for k, s := range styles {
		id, err := x.NewStyle(s)
		if err != nil {
			return nil, "", err
		}
		st[k] = id
	}

	now := time.Now().In(GetTimezoneLocation())
	if actor == "" {
		actor = "-"
	}
	x.SetCellValue(sheet, "A1", "Daftar Perlengkapan")
	x.SetCellStyle(sheet, "A1", "A1", st["title"])
	x.SetCellValue(sheet, "A2", "Saringan : "+describeAssetFilter(f))
	x.SetCellValue(sheet, "A3", fmt.Sprintf("Dicetak  : %s oleh %s · Sumber: Cloud POS — Modul Perlengkapan",
		now.Format("2006-01-02 15:04"), actor))
	x.SetCellStyle(sheet, "A2", "A3", st["info"])

	const headRow = 5
	lastCol, _ := excelize.ColumnNumberToName(len(assetExportColumns))
	for i, col := range assetExportColumns {
		name, _ := excelize.ColumnNumberToName(i + 1)
		x.SetCellValue(sheet, fmt.Sprintf("%s%d", name, headRow), col.header)
		x.SetColWidth(sheet, name, name, col.width)
	}
	x.SetCellStyle(sheet, fmt.Sprintf("A%d", headRow), fmt.Sprintf("%s%d", lastCol, headRow), st["header"])
	x.SetRowHeight(sheet, headRow, 32)

	firstData := headRow + 1
	totals := make([]float64, len(assetExportColumns))
	for r, a := range assets {
		row := make([]interface{}, len(assetExportColumns))
		for i, col := range assetExportColumns {
			if col.value == nil {
				row[i] = r + 1 // kolom No
			} else {
				row[i] = col.value(a)
			}
			if col.total != nil {
				totals[i] += col.total(a)
			}
		}
		if err := x.SetSheetRow(sheet, fmt.Sprintf("A%d", firstData+r), &row); err != nil {
			return nil, "", err
		}
	}

	if len(assets) == 0 {
		x.SetCellValue(sheet, fmt.Sprintf("A%d", firstData), "Tidak ada perlengkapan yang cocok dengan saringan ini.")
		x.SetCellStyle(sheet, fmt.Sprintf("A%d", firstData), fmt.Sprintf("A%d", firstData), st["empty"])
	} else {
		lastData := firstData + len(assets) - 1
		for i, col := range assetExportColumns {
			name, _ := excelize.ColumnNumberToName(i + 1)
			style := st["text"]
			if col.kind != assetColText {
				style = st["num"]
			}
			x.SetCellStyle(sheet, fmt.Sprintf("%s%d", name, firstData), fmt.Sprintf("%s%d", name, lastData), style)
		}

		totalRow := lastData + 1
		for i, col := range assetExportColumns {
			name, _ := excelize.ColumnNumberToName(i + 1)
			cell := fmt.Sprintf("%s%d", name, totalRow)
			if col.total != nil {
				x.SetCellValue(sheet, cell, totals[i])
				x.SetCellStyle(sheet, cell, cell, st["total"])
			} else {
				x.SetCellStyle(sheet, cell, cell, st["label"])
			}
		}
		x.SetCellValue(sheet, fmt.Sprintf("B%d", totalRow), fmt.Sprintf("TOTAL — %d aset", len(assets)))

		// Saringan Excel hanya atas baris data; baris TOTAL di luar jangkauannya.
		if err := x.AutoFilter(sheet, fmt.Sprintf("A%d:%s%d", headRow, lastCol, lastData), nil); err != nil {
			return nil, "", err
		}
	}

	frozenCol, _ := excelize.ColumnNumberToName(assetExportFrozenCols + 1)
	x.SetPanes(sheet, &excelize.Panes{
		Freeze: true, XSplit: assetExportFrozenCols, YSplit: headRow,
		TopLeftCell: fmt.Sprintf("%s%d", frozenCol, firstData), ActivePane: "bottomRight",
	})

	buf, err := x.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf, fmt.Sprintf("Daftar-Perlengkapan_%s.xlsx", now.Format("20060102-1504")), nil
}
