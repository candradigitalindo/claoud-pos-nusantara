package services

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

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
