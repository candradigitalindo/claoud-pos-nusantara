package services

import (
	"bytes"
	"fmt"
	"time"

	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

// ── Ekspor Excel: Analisa Bisnis ────────────────────────────────────────────
//
// Isi file mengikuti persis apa yang tampil di halaman, dengan grafik NATIVE
// Excel (bukan gambar) supaya penerima bisa mengklik grafiknya, mengubah
// rentang, dan menelusuri angkanya sampai ke sel.

// bizPtrCell menulis nilai pointer float ke sel; nil ditulis sebagai sel kosong
// supaya tidak terbaca sebagai angka nol pada grafik.
func bizPtrCell(f *excelize.File, sheet, cell string, v *float64) {
	if v == nil {
		f.SetCellValue(sheet, cell, nil)
		return
	}
	f.SetCellValue(sheet, cell, *v)
}

func bizVerdictLabel(v string) string {
	switch v {
	case models.BizVerdictOutlet:
		return "MASALAHNYA DI OUTLET"
	case models.BizVerdictMarket:
		return "MASALAHNYA DI PASAR / PROMOSI"
	case models.BizVerdictBoth:
		return "PASAR TURUN & ADA OUTLET TERTINGGAL"
	case models.BizVerdictNormal:
		return "SEMUA WAJAR"
	default:
		return "DATA BELUM CUKUP"
	}
}

// bizWeekRange mengubah tanggal Senin menjadi label "6–12 Jul".
func bizWeekRange(weekStart string) string {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return weekStart
	}
	e := t.AddDate(0, 0, 6)
	if t.Month() == e.Month() {
		return fmt.Sprintf("%d–%d %s", t.Day(), e.Day(), monthShortID[e.Month()-1])
	}
	return fmt.Sprintf("%d %s–%d %s", t.Day(), monthShortID[t.Month()-1], e.Day(), monthShortID[e.Month()-1])
}

// BuildBusinessAnalysisExcel menyusun file .xlsx Analisa Bisnis untuk rentang
// minggu yang sama dengan tampilan halaman.
func BuildBusinessAnalysisExcel(weeks, block int) ([]byte, string, error) {
	rep, err := GetBusinessAnalysisWith(weeks, block)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}
	loc := GetTimezoneLocation()

	pctFmt := "0.0"
	stPct, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &pctFmt, Border: []excelize.Border{
		{Type: "left", Color: "D1D5DB", Style: 1}, {Type: "right", Color: "D1D5DB", Style: 1},
		{Type: "top", Color: "D1D5DB", Style: 1}, {Type: "bottom", Color: "D1D5DB", Style: 1},
	}})
	stWrap, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})

	// ══ Sheet 1: Ringkasan ══════════════════════════════════════════════════
	sum := "Ringkasan"
	f.SetSheetName("Sheet1", sum)
	f.SetColWidth(sum, "A", "A", 30)
	f.SetColWidth(sum, "B", "F", 18)

	f.SetCellValue(sum, "A1", "Analisa Bisnis — Outlet atau Pasar?")
	f.SetCellStyle(sum, "A1", "A1", st.title)
	f.SetCellValue(sum, "A2", fmt.Sprintf("Periode: %s s/d %s (%d minggu penuh)",
		fmtDateID(rep.PeriodFrom), fmtDateID(rep.PeriodTo), rep.WeeksCount))
	f.SetCellValue(sum, "A3", "Dibuat: "+fmtTimeID(time.Now().In(loc)))

	f.SetCellValue(sum, "A5", "KESIMPULAN")
	f.SetCellStyle(sum, "A5", "A5", st.section)
	f.SetCellValue(sum, "A6", bizVerdictLabel(rep.Verdict))
	f.SetCellStyle(sum, "A6", "A6", st.title)
	f.MergeCell(sum, "A7", "F10")
	f.SetCellValue(sum, "A7", rep.VerdictText)
	f.SetCellStyle(sum, "A7", "F10", stWrap)

	row := 12
	f.SetCellValue(sum, fmt.Sprintf("A%d", row), "ANGKA UTAMA")
	f.SetCellStyle(sum, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
	row++
	facts := [][2]interface{}{
		{"Gerak pasar setara kalender (ditimbang omzet)", rep.GroupGrowth4},
		{"Gerak pasar mentah", rep.GroupGrowthRaw4},
		{"Dijelaskan kalender (poin)", rep.CalendarEffect},
		{"Batas wajar pasar (±)", rep.MarketBand},
		{"Cara membandingkan", fmt.Sprintf("%d minggu terakhir lawan %d minggu sebelumnya (blok tetap)", rep.BlockWeeks, rep.BlockWeeks)},
		{"Outlet lebih baik", joinOrDash(rep.Leading)},
		{"Outlet tertinggal", joinOrDash(rep.Lagging)},
		{"Outlet perlu dipantau", joinOrDash(rep.Watch)},
		{"Outlet berdata lengkap", fmt.Sprintf("%d outlet", len(rep.PanelCodes))},
		{"Patokan tiap outlet", fmt.Sprintf("nilai tengah %d outlet pembanding", len(rep.PanelCodes))},
		{"Minggu tidak normal", joinOrDash(rep.GroupEvents)},
	}
	for _, fx := range facts {
		f.SetCellValue(sum, fmt.Sprintf("A%d", row), fx[0])
		switch v := fx[1].(type) {
		case *float64:
			if v != nil {
				f.SetCellValue(sum, fmt.Sprintf("B%d", row), *v)
				f.SetCellStyle(sum, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), stPct)
			}
		default:
			f.SetCellValue(sum, fmt.Sprintf("B%d", row), v)
		}
		row++
	}

	row += 2
	f.SetCellValue(sum, fmt.Sprintf("A%d", row), "CARA ANGKA INI DIHITUNG")
	f.SetCellStyle(sum, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
	row++
	for _, n := range rep.Notes {
		f.MergeCell(sum, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row+1))
		f.SetCellValue(sum, fmt.Sprintf("A%d", row), "• "+n)
		f.SetCellStyle(sum, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row+1), stWrap)
		f.SetRowHeight(sum, row, 15)
		row += 2
	}

	// ══ Sheet 2: Per Outlet ═════════════════════════════════════════════════
	det := "Per Outlet"
	f.NewSheet(det)
	headers := []string{"Outlet", "Kode", "Penjualan", "Transaksi", "Rata-rata/Struk",
		"Naik/Turun Setara Kalender (%)", "Outlet Lain (%)", "Selisih", "Batas Wajar",
		"Penjualan Blok Terakhir", "Penjualan Blok Sebelumnya", "Seharusnya", "Selisih (Rp)",
		"Kesimpulan", "Siapa yang Bertindak", "Penjelasan",
		"Dari Mana Perubahannya", "Yang Perlu Dilakukan",
		"Naik/Turun Mentah (%)", "Konsisten (minggu)", "Setara Kalender Blok Terakhir", "Setara Kalender Blok Sebelumnya",
		"Tren (%/minggu)", "Tren Nyata?", "Perkiraan 4 Minggu (Rp)", "Rentang Perkiraan", "Pengamatan Model"}
	setHeaderRow(f, det, headers, st.header)
	widths := []float64{26, 8, 16, 11, 15, 14, 16, 12, 12, 20, 20, 18, 16, 18, 18, 60, 60, 70, 14, 12, 20, 20, 12, 10, 20, 28, 90}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(det, col, col, w)
	}
	for i, o := range rep.Outlets {
		r := i + 2
		f.SetCellValue(det, fmt.Sprintf("A%d", r), o.Name)
		f.SetCellValue(det, fmt.Sprintf("B%d", r), o.Code)
		f.SetCellValue(det, fmt.Sprintf("C%d", r), o.Net)
		f.SetCellValue(det, fmt.Sprintf("D%d", r), o.Trx)
		f.SetCellValue(det, fmt.Sprintf("E%d", r), o.ATV)
		bizPtrCell(f, det, fmt.Sprintf("F%d", r), o.Growth4)
		bizPtrCell(f, det, fmt.Sprintf("G%d", r), o.PeerGrowth4)
		bizPtrCell(f, det, fmt.Sprintf("H%d", r), o.RGI4)
		bizPtrCell(f, det, fmt.Sprintf("I%d", r), o.Threshold)
		if o.RecentNet > 0 {
			f.SetCellValue(det, fmt.Sprintf("J%d", r), o.RecentNet)
			f.SetCellValue(det, fmt.Sprintf("K%d", r), o.PrevNet)
		}
		bizPtrCell(f, det, fmt.Sprintf("L%d", r), o.ExpectedNet)
		bizPtrCell(f, det, fmt.Sprintf("M%d", r), o.GapNet)
		f.SetCellValue(det, fmt.Sprintf("N%d", r), o.DiagnosisLabel)
		f.SetCellValue(det, fmt.Sprintf("O%d", r), o.Owner)
		f.SetCellValue(det, fmt.Sprintf("P%d", r), o.Note)
		f.SetCellValue(det, fmt.Sprintf("Q%d", r), o.Breakdown)
		f.SetCellValue(det, fmt.Sprintf("R%d", r), o.Advice)
		bizPtrCell(f, det, fmt.Sprintf("S%d", r), o.GrowthRaw4)
		if o.ConsistencyNeed > 0 {
			f.SetCellValue(det, fmt.Sprintf("T%d", r), fmt.Sprintf("%d dari %d", o.Consistency, rep.BlockWeeks))
		}
		if o.RecentAdj > 0 {
			f.SetCellValue(det, fmt.Sprintf("U%d", r), o.RecentAdj)
			f.SetCellValue(det, fmt.Sprintf("V%d", r), o.PrevAdj)
		}
		if o.Trend != nil {
			f.SetCellValue(det, fmt.Sprintf("W%d", r), o.Trend.SlopePct)
			nyata := "belum"
			if o.Trend.Significant {
				nyata = "ya"
			}
			f.SetCellValue(det, fmt.Sprintf("X%d", r), nyata)
		}
		if o.Forecast != nil {
			f.SetCellValue(det, fmt.Sprintf("Y%d", r), o.Forecast.TotalRaw)
			f.SetCellValue(det, fmt.Sprintf("Z%d", r), fmt.Sprintf("%s – %s", bizRupiah(o.Forecast.TotalLo), bizRupiah(o.Forecast.TotalHi)))
		}
		if len(o.ModelNotes) > 0 {
			f.SetCellValue(det, fmt.Sprintf("AA%d", r), joinOrDash(o.ModelNotes))
		}

		f.SetCellStyle(det, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.text)
		f.SetCellStyle(det, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), st.money)
		f.SetCellStyle(det, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), st.num)
		f.SetCellStyle(det, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.money)
		f.SetCellStyle(det, fmt.Sprintf("F%d", r), fmt.Sprintf("I%d", r), stPct)
		f.SetCellStyle(det, fmt.Sprintf("J%d", r), fmt.Sprintf("M%d", r), st.money)
		f.SetCellStyle(det, fmt.Sprintf("N%d", r), fmt.Sprintf("R%d", r), st.text)
		f.SetCellStyle(det, fmt.Sprintf("S%d", r), fmt.Sprintf("S%d", r), stPct)
		f.SetCellStyle(det, fmt.Sprintf("U%d", r), fmt.Sprintf("V%d", r), st.money)
		f.SetCellStyle(det, fmt.Sprintf("W%d", r), fmt.Sprintf("W%d", r), stPct)
		f.SetCellStyle(det, fmt.Sprintf("Y%d", r), fmt.Sprintf("Y%d", r), st.money)
		f.SetCellStyle(det, fmt.Sprintf("AA%d", r), fmt.Sprintf("AA%d", r), st.text)
	}
	lastDet := len(rep.Outlets) + 1

	// Grafik batang: selisih tiap outlet dengan rata-rata.
	if lastDet >= 2 {
		f.AddChart(det, "AC2", &excelize.Chart{
			Type: excelize.Bar,
			Series: []excelize.ChartSeries{{
				Name:       fmt.Sprintf("'%s'!$H$1", det),
				Categories: fmt.Sprintf("'%s'!$B$2:$B$%d", det, lastDet),
				Values:     fmt.Sprintf("'%s'!$H$2:$H$%d", det, lastDet),
			}},
			Title:     []excelize.RichTextRun{{Text: "Selisih Tiap Outlet dengan Outlet Lain"}},
			Legend:    excelize.ChartLegend{},
			Dimension: excelize.ChartDimension{Width: 620, Height: 380},
			XAxis:     excelize.ChartAxis{Title: []excelize.RichTextRun{{Text: "Outlet"}}},
			YAxis:     excelize.ChartAxis{Title: []excelize.RichTextRun{{Text: "← tertinggal   ·   lebih unggul →"}}},
		})
	}

	// ══ Sheet 3: Mingguan ═══════════════════════════════════════════════════
	wk := "Mingguan"
	f.NewSheet(wk)
	wkHeaders := []string{"Minggu", "Mulai", "Total Semua Outlet", "Total Setara Kalender", "Jumlah Outlet",
		"Naik/Turun Pasar Setara Kalender (%)", "Naik/Turun Pasar Mentah (%)", "Kalender"}
	for _, o := range rep.Outlets {
		wkHeaders = append(wkHeaders, o.Code)
	}
	setHeaderRow(f, wk, wkHeaders, st.header)
	f.SetColWidth(wk, "A", "A", 16)
	f.SetColWidth(wk, "B", "B", 12)
	f.SetColWidth(wk, "C", "G", 20)
	f.SetColWidth(wk, "H", "H", 40)
	const wkOutletCol = 9 // kolom pertama outlet (I)

	netByOutletWeek := map[string]map[string]float64{}
	for _, o := range rep.Outlets {
		m := map[string]float64{}
		for _, w := range o.Weeks {
			m[w.WeekStart] = w.Net
		}
		netByOutletWeek[o.Code] = m
	}
	for i, g := range rep.Group {
		r := i + 2
		f.SetCellValue(wk, fmt.Sprintf("A%d", r), bizWeekRange(g.WeekStart))
		f.SetCellValue(wk, fmt.Sprintf("B%d", r), fmtDateID(g.WeekStart))
		f.SetCellValue(wk, fmt.Sprintf("C%d", r), g.Net)
		f.SetCellValue(wk, fmt.Sprintf("D%d", r), g.NetAdj)
		f.SetCellValue(wk, fmt.Sprintf("E%d", r), g.OutletCount)
		bizPtrCell(f, wk, fmt.Sprintf("F%d", r), g.Growth)
		bizPtrCell(f, wk, fmt.Sprintf("G%d", r), g.GrowthRaw)
		f.SetCellValue(wk, fmt.Sprintf("H%d", r), g.Calendar)
		f.SetCellStyle(wk, fmt.Sprintf("C%d", r), fmt.Sprintf("D%d", r), st.money)
		f.SetCellStyle(wk, fmt.Sprintf("F%d", r), fmt.Sprintf("G%d", r), stPct)
		for j, o := range rep.Outlets {
			col, _ := excelize.ColumnNumberToName(wkOutletCol + j)
			if v, ok := netByOutletWeek[o.Code][g.WeekStart]; ok {
				f.SetCellValue(wk, fmt.Sprintf("%s%d", col, r), v)
			}
			f.SetCellStyle(wk, fmt.Sprintf("%s%d", col, r), fmt.Sprintf("%s%d", col, r), st.money)
		}
	}
	lastWk := len(rep.Group) + 1

	// Grafik kolom: naik-turun seluruh grup per minggu.
	if lastWk >= 2 {
		chartCol, _ := excelize.ColumnNumberToName(wkOutletCol + len(rep.Outlets) + 1)
		f.AddChart(wk, fmt.Sprintf("%s2", chartCol), &excelize.Chart{
			Type: excelize.Col,
			Series: []excelize.ChartSeries{{
				Name:       fmt.Sprintf("'%s'!$F$1", wk),
				Categories: fmt.Sprintf("'%s'!$A$2:$A$%d", wk, lastWk),
				Values:     fmt.Sprintf("'%s'!$F$2:$F$%d", wk, lastWk),
			}},
			Title:     []excelize.RichTextRun{{Text: "Naik-Turun Pasar per Minggu (setara kalender)"}},
			Legend:    excelize.ChartLegend{},
			Dimension: excelize.ChartDimension{Width: 700, Height: 360},
			YAxis:     excelize.ChartAxis{Title: []excelize.RichTextRun{{Text: "Persen dibanding minggu sebelumnya"}}},
		})
	}

	// ══ Sheet 4: Indeks ═════════════════════════════════════════════════════
	idx := "Perjalanan"
	f.NewSheet(idx)
	idxHeaders := []string{"Minggu", "PASAR"}
	for _, o := range rep.Outlets {
		idxHeaders = append(idxHeaders, o.Code)
	}
	setHeaderRow(f, idx, idxHeaders, st.header)
	f.SetColWidth(idx, "A", "A", 16)

	indexByOutletWeek := map[string]map[string]float64{}
	for _, o := range rep.Outlets {
		m := map[string]float64{}
		for _, w := range o.Weeks {
			if w.Index != nil {
				m[w.WeekStart] = *w.Index
			}
		}
		indexByOutletWeek[o.Code] = m
	}
	for i, g := range rep.Group {
		r := i + 2
		f.SetCellValue(idx, fmt.Sprintf("A%d", r), bizWeekRange(g.WeekStart))
		if g.Index != nil {
			f.SetCellValue(idx, fmt.Sprintf("B%d", r), *g.Index)
		}
		for j, o := range rep.Outlets {
			col, _ := excelize.ColumnNumberToName(3 + j)
			if v, ok := indexByOutletWeek[o.Code][g.WeekStart]; ok {
				f.SetCellValue(idx, fmt.Sprintf("%s%d", col, r), v)
			}
		}
	}

	// Grafik garis: perjalanan penjualan tiap outlet (minggu pertama = 100).
	if lastWk >= 2 {
		var series []excelize.ChartSeries
		for j := 0; j <= len(rep.Outlets); j++ {
			col, _ := excelize.ColumnNumberToName(2 + j)
			series = append(series, excelize.ChartSeries{
				Name:       fmt.Sprintf("'%s'!$%s$1", idx, col),
				Categories: fmt.Sprintf("'%s'!$A$2:$A$%d", idx, lastWk),
				Values:     fmt.Sprintf("'%s'!$%s$2:$%s$%d", idx, col, col, lastWk),
			})
		}
		chartCol, _ := excelize.ColumnNumberToName(3 + len(rep.Outlets) + 1)
		f.AddChart(idx, fmt.Sprintf("%s2", chartCol), &excelize.Chart{
			Type:      excelize.Line,
			Series:    series,
			Title:     []excelize.RichTextRun{{Text: "Perjalanan Penjualan Tiap Outlet (minggu pertama = 100)"}},
			Legend:    excelize.ChartLegend{Position: "bottom"},
			Dimension: excelize.ChartDimension{Width: 760, Height: 400},
			YAxis:     excelize.ChartAxis{Title: []excelize.RichTextRun{{Text: "Indeks"}}},
		})
	}

	sheetIdx, _ := f.GetSheetIndex(sum)
	f.SetActiveSheet(sheetIdx)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("analisa-bisnis-%s-sd-%s-blok%d.xlsx",
		slugFilename(rep.PeriodFrom), slugFilename(rep.PeriodTo), rep.BlockWeeks)
	return buf.Bytes(), name, nil
}

func joinOrDash(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	out := xs[0]
	for _, x := range xs[1:] {
		out += ", " + x
	}
	return out
}
