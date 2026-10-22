package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// ── Helper format & tata letak ────────────────────────────────

// groupID menyisipkan pemisah ribuan gaya Indonesia (1.234.567).
func groupID(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	if neg {
		return "-" + out
	}
	return out
}

func rpID(v float64) string {
	return "Rp " + groupID(int64(v+0.5*sign(v)))
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

func qtyID(v float64) string {
	if v == float64(int64(v)) {
		return groupID(int64(v))
	}
	return fmt.Sprintf("%s,%02d", groupID(int64(v)), int64(v*100+0.5)%100)
}

// ppicLookupName mengambil nama outlet/gudang untuk baris filter di header.
func ppicLookupName(table, id string) string {
	if id == "" {
		return ""
	}
	var name string
	database.DB.QueryRow(`SELECT name FROM `+table+` WHERE id = $1`, id).Scan(&name)
	return name
}

// ppicWriteHeader menulis blok identitas + RINGKASAN, mengembalikan nomor baris
// tempat header tabel harus ditulis.
func ppicWriteHeader(f *excelize.File, sheet string, st exportStyles, title, period, filterLine, actor string, summary [][2]string) int {
	now := time.Now().In(GetTimezoneLocation())
	f.SetCellValue(sheet, "A1", title)
	f.SetCellStyle(sheet, "A1", "A1", st.title)
	f.SetCellValue(sheet, "A2", "Periode  : "+period)
	f.SetCellValue(sheet, "A3", "Filter   : "+filterLine)
	f.SetCellValue(sheet, "A4", fmt.Sprintf("Dicetak  : %s oleh %s · Sumber: Cloud POS — Modul PPIC", fmtTimeID(now), actor))
	f.SetCellStyle(sheet, "A2", "A4", st.label)

	row := 6
	if len(summary) > 0 {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "RINGKASAN")
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
		for _, kv := range summary {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row), kv[0])
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), kv[1])
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
			row++
		}
		row++ // baris kosong pemisah
	}
	return row
}

// ppicHeaderRowAt menulis header kolom tabel pada baris tertentu + freeze pane
// + auto-filter Excel (kolom bisa disortir/difilter oleh penerima file).
func ppicHeaderRowAt(f *excelize.File, sheet string, hrow int, headers []string, styleID int) {
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, hrow)
		f.SetCellValue(sheet, cell, h)
	}
	first, _ := excelize.CoordinatesToCellName(1, hrow)
	last, _ := excelize.CoordinatesToCellName(len(headers), hrow)
	f.SetCellStyle(sheet, first, last, styleID)
	f.SetRowHeight(sheet, hrow, 22)
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: hrow, TopLeftCell: fmt.Sprintf("A%d", hrow+1), ActivePane: "bottomLeft"})
}

func ppicAutoFilter(f *excelize.File, sheet string, hrow, lastRow, cols int) {
	first, _ := excelize.CoordinatesToCellName(1, hrow)
	last, _ := excelize.CoordinatesToCellName(cols, lastRow)
	f.AutoFilter(sheet, first+":"+last, nil)
}

// BuildPpicReportExcel membuat file Excel untuk satu tab Laporan PPIC.
// tab: sold | hpp | variance | production | forecast | otif.
// Filter & scope identik dengan tampilan layar; ringkasan di atas tabel
// menyalin kartu-kartu yang tampil di halaman.
func BuildPpicReportExcel(tab, dateFrom, dateTo, warehouseID, outletID string, idealPct float64, actor string, outletScope []string) ([]byte, string, error) {
	f := excelize.NewFile()
	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}
	sheet := "Laporan"
	f.SetSheetName("Sheet1", sheet)

	// Style tambahan: penanda baris/sel bermasalah.
	border := []excelize.Border{
		{Type: "left", Color: "D1D5DB", Style: 1}, {Type: "right", Color: "D1D5DB", Style: 1},
		{Type: "top", Color: "D1D5DB", Style: 1}, {Type: "bottom", Color: "D1D5DB", Style: 1},
	}
	pctFmt := "0.0%"
	badPct, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "DC2626"}, CustomNumFmt: &pctFmt, Border: border,
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FEF2F2"}}})
	badTxt, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "DC2626"}, Border: border,
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FEF2F2"}}})
	badNum, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "DC2626"}, Border: border,
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FEF2F2"}}})

	period := fmt.Sprintf("%s s.d. %s", fmtDateID(dateFrom), fmtDateID(dateTo))
	outletName := ppicLookupName("outlets", outletID)
	warehouseName := ppicLookupName("warehouses", warehouseID)
	orDefault := func(s, def string) string {
		if s == "" {
			return def
		}
		return s
	}

	var title, slug string
	switch tab {

	// ─────────────────────────────────────────────────────────
	case "sold":
		title, slug = "Laporan PPIC — Produk Terjual (Qty, tanpa nominal)", "Produk-Terjual"
		rep, err := GetPpicSoldReport(dateFrom, dateTo, outletID, "", outletScope)
		if err != nil {
			return nil, "", err
		}
		avgDay := 0.0
		if rep.Days > 0 {
			avgDay = rep.TotalQty / float64(rep.Days)
		}
		noRecipePct := 0.0
		if rep.TotalQty > 0 {
			noRecipePct = rep.NoRecipeQty / rep.TotalQty * 100
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period,
			"Outlet: "+orDefault(outletName, "Semua (sesuai hak akses)"), actor, [][2]string{
				{"Total terjual", qtyID(rep.TotalQty) + " porsi dalam " + fmt.Sprintf("%d", rep.Days) + " hari"},
				{"Produk berbeda terjual", fmt.Sprintf("%d produk", rep.ProductCount)},
				{"Rata-rata per hari", qtyID(avgDay) + " porsi"},
				{"Terjual dari produk TANPA resep", fmt.Sprintf("%s porsi (%.1f%%) — tidak terhitung di MRP/variance", qtyID(rep.NoRecipeQty), noRecipePct)},
			})
		headers := []string{"No", "Produk", "Kategori", "Outlet", "Qty Terjual", "Rata-rata/Hari", "Kontribusi %", "Punya Resep"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for i, r := range rep.Rows {
			hasRecipe := "Ya"
			if !r.HasRecipe {
				hasRecipe = "BELUM"
			}
			f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
				i + 1, r.ProductName, r.Category, r.OutletName, r.Qty, r.AvgPerDay, r.SharePct / 100, hasRecipe})
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row), st.text)
			f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), st.pct)
			if r.HasRecipe {
				f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.text)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), badTxt)
			}
			row++
		}
		f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{"", "TOTAL", "", "", rep.TotalQty, avgDay, 1.0, ""})
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row), st.numTot)
		f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), st.pctTot)
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "A", 5)
		f.SetColWidth(sheet, "B", "B", 34)
		f.SetColWidth(sheet, "C", "D", 20)
		f.SetColWidth(sheet, "E", "H", 14)

	// ─────────────────────────────────────────────────────────
	case "hpp":
		title, slug = "Laporan PPIC — HPP Menu (Costing Card)", "HPP-Menu"
		rep, err := GetPpicHppReport(outletID, idealPct, outletScope)
		if err != nil {
			return nil, "", err
		}
		hrow := ppicWriteHeader(f, sheet, st, title, fmt.Sprintf("per %s (kondisi biaya saat ini)", fmtDateID(time.Now().In(GetTimezoneLocation()).Format("2006-01-02"))),
			"Outlet: "+orDefault(outletName, "Semua (sesuai hak akses)")+" · Batas HPP mengikuti kebijakan masing-masing outlet/menu", actor, [][2]string{
				{"Jumlah menu", fmt.Sprintf("%d menu, %d di antaranya ber-resep", rep.ProductCount, rep.WithRecipe)},
				{"Rata-rata HPP", fmt.Sprintf("%.1f%% dari harga jual", rep.AvgHppPct)},
				{"Menu mencapai batas", fmt.Sprintf("%d menu — perlu tinjau harga/porsi/resep", rep.OverCount)},
				{"Basis biaya", "Stok outlet + kemasan + overhead; sama dengan F&B. Proyeksi induk untuk perencanaan, bukan HPP penjualan aktual."},
				{"Menu tanpa resep", orDefault(rep.NoRecipeSample, "—")},
			})
		headers := []string{"Menu", "Outlet", "Kategori", "Bahan / Komponen", "Qty", "Satuan", "Harga/Satuan", "Cost", "HPP Total", "Harga Jual", "HPP %", "Margin", "Batas Biaya", "Batas %", "HPP Proyeksi", "Proyeksi %", "Status Biaya", "Kemasan", "Overhead"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for _, r := range rep.Rows {
			first := true
			ingredients := r.Ingredients
			if len(ingredients) == 0 {
				ingredients = []models.PpicHppIngredient{{}}
			}
			for _, ing := range ingredients {
				vals := []interface{}{"", "", "", ing.ItemName, ing.QtyBase, ing.Unit, ing.CostPerBase, ing.Cost, "", "", "", "", "", "", "", "", "", "", ""}
				if first {
					vals[0], vals[1], vals[2] = r.ProductName, r.OutletName, r.Category
					vals[9], vals[12], vals[13], vals[16], vals[17], vals[18] = r.Price, r.IdealCost, r.TargetCostPct/100, r.CostStatus, r.PackagingCost, r.OverheadCost
					if r.CostStatus == "ready" {
						vals[8] = r.Hpp
						if r.Price > 0 {
							vals[10], vals[11] = r.HppPct/100, r.Margin
						}
					}
					if r.ProjectedHpp != nil {
						vals[14] = *r.ProjectedHpp
					}
					if r.ProjectedCostPct != nil {
						vals[15] = *r.ProjectedCostPct / 100
					}
				}
				if ing.IsWip {
					vals[3] = "WIP — " + ing.ItemName
				}
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &vals)
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), st.text)
				f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), st.num)
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), st.text)
				f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("J%d", row), st.money)
				if first && r.IsOver {
					f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("K%d", row), badPct)
				} else {
					f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("K%d", row), st.pct)
				}
				f.SetCellStyle(sheet, fmt.Sprintf("L%d", row), fmt.Sprintf("M%d", row), st.money)
				f.SetCellStyle(sheet, fmt.Sprintf("N%d", row), fmt.Sprintf("N%d", row), st.pct)
				f.SetCellStyle(sheet, fmt.Sprintf("O%d", row), fmt.Sprintf("O%d", row), st.money)
				f.SetCellStyle(sheet, fmt.Sprintf("P%d", row), fmt.Sprintf("P%d", row), st.pct)
				f.SetCellStyle(sheet, fmt.Sprintf("R%d", row), fmt.Sprintf("S%d", row), st.money)
				first = false
				row++
			}
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row+1), "HPP % merah = mencapai batas outlet/menu. Proyeksi tidak mengubah stok dan HPP transaksi. Baris tanpa nama menu = lanjutan rincian bahan menu di atasnya.")
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row+1), fmt.Sprintf("A%d", row+1), st.note)
		f.SetColWidth(sheet, "A", "A", 30)
		f.SetColWidth(sheet, "B", "C", 16)
		f.SetColWidth(sheet, "D", "D", 30)
		f.SetColWidth(sheet, "E", "M", 13)
		f.SetColWidth(sheet, "N", "S", 18)

	// ─────────────────────────────────────────────────────────
	case "variance":
		title, slug = "Laporan PPIC — Variance Pemakaian", "Variance-Pemakaian"
		rep, err := GetPpicVarianceReport(dateFrom, dateTo, warehouseID, outletScope)
		if err != nil {
			return nil, "", err
		}
		summary := [][2]string{
			{"Pemakaian teoretis", rpID(rep.TheoValue) + " (penjualan × resep + produksi × resep)"},
			{"Pemakaian aktual", rpID(rep.ActValue) + " (buku stok, tanpa transfer)"},
			{"Variance", fmt.Sprintf("%+.1f%% — target < ±5%%", rep.VariancePct)},
			{"Food cost aktual", fmt.Sprintf("%.1f%% dari omzet %s (biaya aktual periode laporan)", rep.FoodCostPct, rpID(rep.Revenue))},
			{"Dasar teoretis", "Simulasi memakai resep saat laporan dibuka. Perubahan resep tidak mengubah pemakaian aktual."},
			{"Coverage resep", fmt.Sprintf("%.1f%% dari %s porsi terjual", rep.RecipeCoveragePct, qtyID(rep.QtySold))},
		}
		if !rep.Representative {
			summary = append(summary, [2]string{"PERHATIAN", "Coverage resep < 80% — angka variance BELUM representatif"})
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period,
			"Gudang: "+orDefault(warehouseName, "Semua (sesuai hak akses)"), actor, summary)
		headers := []string{"Kode", "Item", "Kategori", "Satuan", "Qty Teoretis", "Nilai Teoretis", "Qty Aktual", "Nilai Aktual", "Selisih Qty", "Selisih Nilai", "Variance %"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for _, r := range rep.Rows {
			f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
				r.ItemCode, r.ItemName, r.Category, r.BaseUnit,
				r.TheoQty, r.TheoValue, r.ActQty, r.ActValue,
				r.DiffQty, r.DiffValue, r.VariancePct / 100})
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), st.text)
			f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), st.money)
			f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.money)
			f.SetCellStyle(sheet, fmt.Sprintf("I%d", row), fmt.Sprintf("I%d", row), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("J%d", row), fmt.Sprintf("J%d", row), st.money)
			if r.IsOver {
				f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("K%d", row), badPct)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("K%d", row), st.pct)
			}
			row++
		}
		f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
			"", "TOTAL", "", "", "", rep.TheoValue, "", rep.ActValue, "", rep.ActValue - rep.TheoValue, rep.VariancePct / 100})
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("J%d", row), st.moneyTot)
		f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("K%d", row), st.pctTot)
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "A", 12)
		f.SetColWidth(sheet, "B", "B", 32)
		f.SetColWidth(sheet, "C", "K", 14)

	// ─────────────────────────────────────────────────────────
	case "production":
		title, slug = "Laporan PPIC — Produksi & Yield", "Produksi-Yield"
		rep, err := GetPpicProductionReport(dateFrom, dateTo, warehouseID, outletScope)
		if err != nil {
			return nil, "", err
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period,
			"Gudang: "+orDefault(warehouseName, "Semua (sesuai hak akses)"), actor, [][2]string{
				{"Work order selesai", fmt.Sprintf("%d WO", rep.WoDone)},
				{"Rata-rata yield", fmt.Sprintf("%.1f%% — target ≥ 90%%", rep.AvgYieldPct)},
				{"Plan adherence", fmt.Sprintf("%.1f%% (Σ hasil aktual / Σ rencana)", rep.PlanAdherence)},
				{"Total HPP produksi", rpID(rep.TotalCost)},
				{"Total variance bahan", rpID(rep.TotalMatVar) + " (aktual − rencana, dihargai)"},
			})
		headers := []string{"Nomor WO", "Rencana", "Gudang", "Item", "Satuan", "Qty Rencana", "Qty Aktual", "Yield %", "HPP Total", "HPP/Unit", "Variance Bahan (Rp)", "Selesai", "Oleh"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for _, r := range rep.Rows {
			finished := r.FinishedAt
			if len(finished) >= 10 {
				finished = fmtDateID(finished[:10])
			}
			f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
				r.WoNumber, r.PlanNumber, r.WarehouseName, r.ItemName, r.BaseUnit,
				r.QtyPlanned, r.QtyActual, r.YieldPct / 100, r.CostTotal, r.CostPerUnit, r.MatVariance,
				finished, r.ExecutedBy})
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), st.text)
			f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("G%d", row), st.num)
			if r.YieldPct < 90 {
				f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), badPct)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.pct)
			}
			f.SetCellStyle(sheet, fmt.Sprintf("I%d", row), fmt.Sprintf("K%d", row), st.money)
			f.SetCellStyle(sheet, fmt.Sprintf("L%d", row), fmt.Sprintf("M%d", row), st.text)
			row++
		}
		f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
			"", "TOTAL", "", "", "", "", "", rep.AvgYieldPct / 100, rep.TotalCost, "", rep.TotalMatVar, "", ""})
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), st.numTot)
		f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.pctTot)
		f.SetCellStyle(sheet, fmt.Sprintf("I%d", row), fmt.Sprintf("M%d", row), st.moneyTot)
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "D", 18)
		f.SetColWidth(sheet, "E", "M", 13)

	// ─────────────────────────────────────────────────────────
	case "forecast":
		title, slug = "Laporan PPIC — Akurasi Forecast", "Akurasi-Forecast"
		rep, err := GetPpicForecastAccReport(dateFrom, dateTo, outletID, outletScope)
		if err != nil {
			return nil, "", err
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period,
			"Outlet: "+orDefault(outletName, "Semua (sesuai hak akses)"), actor, [][2]string{
				{"Akurasi agregat", fmt.Sprintf("%.1f%% (100 − MAPE) — target ≥ 80%%", rep.AccuracyPct)},
				{"Titik evaluasi", fmt.Sprintf("%d hari-produk dengan penjualan aktual > 0", rep.EvalRows)},
				{"Cara baca bias", "Bias + = forecast KELEBIHAN (over), bias − = KEKURANGAN (under)"},
			})
		headers := []string{"Outlet", "Produk", "Titik Evaluasi", "Akurasi %", "Bias %"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for _, r := range rep.Rows {
			f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
				r.OutletName, r.ProductName, r.EvalRows, r.AccuracyPct / 100, r.BiasPct / 100})
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), st.text)
			f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), st.num)
			if r.AccuracyPct < 80 {
				f.SetCellStyle(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("D%d", row), badPct)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("D%d", row), st.pct)
			}
			f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), st.pct)
			row++
		}
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "B", 28)
		f.SetColWidth(sheet, "C", "E", 18)

	// ─────────────────────────────────────────────────────────
	case "otif":
		title, slug = "Laporan PPIC — Kinerja Vendor (OTIF)", "OTIF-Vendor"
		rep, err := GetPpicOtifReport(dateFrom, dateTo, outletScope)
		if err != nil {
			return nil, "", err
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period, "Semua vendor dalam scope", actor, [][2]string{
			{"OTIF agregat", fmt.Sprintf("%.1f%% (tepat waktu / diterima) — target ≥ 90%%", rep.OtifPct)},
			{"Total PR ber-tanggal-butuh", fmt.Sprintf("%d PR", rep.TotalPR)},
			{"Catatan metode", rep.Note},
		})
		headers := []string{"Vendor", "Total PR", "Diterima", "Tepat Waktu", "Terlambat", "Overdue Belum Datang", "OTIF %", "Lead Time Rata² (hari)"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		for _, r := range rep.Rows {
			f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
				r.VendorName, r.TotalPR, r.Received, r.OnTime, r.Late, r.OutstandingOverdue, r.OtifPct / 100, r.AvgLeadDays})
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.text)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row), st.num)
			if r.Late > 0 {
				f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), badNum)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), st.num)
			}
			if r.OutstandingOverdue > 0 {
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), badNum)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), st.num)
			}
			if r.Received > 0 && r.OtifPct < 90 {
				f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), badPct)
			} else {
				f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), st.pct)
			}
			f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.num)
			row++
		}
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "A", 30)
		f.SetColWidth(sheet, "B", "H", 17)

	// ─────────────────────────────────────────────────────────
	case "distribution":
		title, slug = "Laporan PPIC — Distribusi Gudang Induk → Outlet", "Distribusi-Outlet"
		rep, err := GetPpicDistributionReport(dateFrom, dateTo, outletID, "", outletScope)
		if err != nil {
			return nil, "", err
		}
		hrow := ppicWriteHeader(f, sheet, st, title, period,
			"Outlet: "+orDefault(outletName, "Semua (sesuai hak akses)"), actor, [][2]string{
				{"Nilai barang dikirim", rpID(rep.TotalDeliveredValue)},
				{"Dokumen kiriman", fmt.Sprintf("%d transfer diterima", rep.DeliveryDocs)},
				{"Item bergerak", fmt.Sprintf("%d item dikirim dari induk", rep.ItemsDelivered)},
				{"Outlet tujuan", fmt.Sprintf("%d outlet", len(rep.Outlets))},
			})
		headers := []string{"No", "Item", "Kode", "Satuan", "Outlet", "Dikirim (Qty)", "Kiriman", "Kiriman Terakhir",
			"Terjual", "Waste", "Keluar Lain", "Masuk Lain", "Stok Outlet Kini", "Stok Induk Kini"}
		ppicHeaderRowAt(f, sheet, hrow, headers, st.header)
		row := hrow + 1
		no := 0
		for _, r := range rep.Rows {
			for _, o := range rep.Outlets {
				c, ok := r.Cells[o.OutletID]
				if !ok {
					continue
				}
				no++
				last := ""
				if c.LastDelivery != "" {
					last = fmtDateID(c.LastDelivery[:10])
				}
				f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &[]interface{}{
					no, r.ItemName, r.ItemCode, r.BaseUnit, o.OutletName,
					c.Delivered, c.DeliveryCount, last,
					c.SaleOut, c.WasteOut, c.OtherOut, c.OtherIn, c.CurrentQty, r.CentralQty})
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.num)
				f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("E%d", row), st.text)
				f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("G%d", row), st.num)
				f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), st.text)
				f.SetCellStyle(sheet, fmt.Sprintf("I%d", row), fmt.Sprintf("N%d", row), st.num)
				if c.WasteOut > 0 {
					f.SetCellStyle(sheet, fmt.Sprintf("J%d", row), fmt.Sprintf("J%d", row), badNum)
				}
				row++
			}
		}
		ppicAutoFilter(f, sheet, hrow, row-1, len(headers))
		f.SetColWidth(sheet, "A", "A", 5)
		f.SetColWidth(sheet, "B", "B", 30)
		f.SetColWidth(sheet, "C", "E", 16)
		f.SetColWidth(sheet, "F", "N", 14)

	default:
		return nil, "", fmt.Errorf("tab laporan tidak dikenal: %s", tab)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("Laporan-PPIC_%s_%s_sd_%s.xlsx", slug, dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}
