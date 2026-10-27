package services

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

var ppicUsageFlagLabels = map[string]string{
	"production_movement_missing":  "Mutasi bahan produksi tidak sesuai dengan snapshot realisasi WO",
	"policy_unset":                 "Batas belum ditetapkan",
	"no_reference":                 "Tidak ada acuan pemakaian",
	"stocktake_missing":            "Belum ada opname disetujui dalam periode",
	"stocktake_shortage":           "Kekurangan fisik dari koreksi opname",
	"stocktake_surplus_not_offset": "Kelebihan opname tidak menutup kekurangan",
	"manual_adjustment_shortage":   "Koreksi stok negatif di luar opname",
	"void_waste_included":          "Waste sudah termasuk void",
	"usage_over_limit":             "Pemakaian tercatat melebihi batas",
	"waste_over_limit":             "Waste melebihi batas",
	"sale_basis_missing":           "Pengeluaran berlabel penjualan tanpa bukti pembayaran yang valid",
	"pending_deductions":           "Masih ada deduksi tertunda di gudang ini",
	"sales_without_stock_basis":    "Ada transaksi berbayar tanpa snapshot bahan di gudang ini",
	"production_snapshot_missing":  "Snapshot rencana produksi tidak lengkap",
	"production_documented_basis":  "Produksi langsung: acuan sama dengan pengeluaran tercatat, tanpa standar independen",
	"production_plan_snapshot":     "Acuan produksi memakai snapshot rencana WO yang disesuaikan hasil aktual",
	"breakdown_snapshot_missing":   "Dokumen pemotongan tidak cocok dengan pengeluaran",
	"breakdown_documented_basis":   "Pemotongan: acuan memakai bahan masuk tercatat, belum menilai kewajaran hasil",
	"waste_document_missing":       "Pengeluaran waste tidak memiliki dokumen waste yang sesuai",
	"returns_excluded":             "Retur tidak dimasukkan dalam pemakaian",
	"other_usage_review":           "Pengeluaran lain perlu ditinjau",
}

func usageFlagsText(flags []string) string {
	texts := make([]string, 0, len(flags))
	for _, flag := range flags {
		label := ppicUsageFlagLabels[flag]
		if label == "" {
			label = flag
		}
		texts = append(texts, label)
	}
	return strings.Join(texts, "; ")
}

// ExportPpicUsageControl intentionally reuses the report's scope, status,
// search and summary before pagination. Empty limits remain blank, not zero.
func ExportPpicUsageControl(dateFrom, dateTo, warehouseID, search, status string, scope []string) ([]byte, error) {
	report, err := GetPpicUsageControl(dateFrom, dateTo, warehouseID, search, status, scope, 1, 0)
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Kontrol Pemakaian"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	st, err := buildExportStyles(f)
	if err != nil {
		return nil, err
	}
	header := ppicWriteHeader(f, sheet, st, "KONTROL PEMAKAIAN BAHAN — PPIC", dateFrom+" s.d. "+dateTo,
		fmt.Sprintf("Gudang: %s | Pencarian: %s | Status: %s | Cakupan sesuai hak akses", warehouseID, search, usageStatusLabel(status)), "PPIC", [][2]string{
			{"Baris bahan/gudang", fmt.Sprint(report.Summary.ItemsCount)},
			{"Melebihi batas", fmt.Sprint(report.Summary.OverLimitCount)},
			{"Batas belum ditetapkan", fmt.Sprint(report.Summary.UnsetCount)},
			{"Belum ada opname dalam periode", fmt.Sprint(report.Summary.NeedsCount)},
			{"Belum ada acuan", fmt.Sprint(report.Summary.NoBasisCount)},
			{"Nilai waste", rpID(report.Summary.WasteValue)},
			{"Nilai koreksi stok negatif", rpID(report.Summary.ShortageValue)},
			{"Deduksi tertunda (seluruh gudang terpilih)", fmt.Sprint(report.Summary.PendingDeductions)},
		})
	for _, note := range report.Notes {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", header), note)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", header), fmt.Sprintf("A%d", header), st.note)
		header++
	}
	header++
	columns := []string{"Gudang", "Outlet", "Kode bahan", "Bahan", "Kategori", "Satuan dasar", "Stok saat laporan dibuka", "Penjualan terbayar (snapshot)", "Produksi aktual", "Acuan produksi", "Pemotongan", "Pengeluaran lain", "Kekurangan opname", "Kelebihan opname (tidak dinet)", "Koreksi negatif lain", "Transfer keluar (dikecualikan)", "Karantina (dikecualikan)", "Acuan pemakaian", "Pemakaian dinilai", "Kelebihan pemakaian", "Waste termasuk void", "Dari waste: void", "Nilai waste (Rp)", "Nilai koreksi negatif (Rp)", "Batas pemakaian (qty)", "Batas waste (qty)", "Waste / acuan (%)", "Selisih pemakaian (%)", "Toleransi pemakaian (%)", "Toleransi waste (%)", "Versi batas saat ini", "Alasan batas", "Pencatat batas", "Batas diperbarui", "Opname terakhir disetujui", "Nomor opname", "Ada opname dalam periode", "Status", "Catatan dasar perhitungan", "Dari waste: belum terdokumentasi"}
	ppicHeaderRowAt(f, sheet, header, columns, st.header)
	f.SetRowHeight(sheet, header, 60)
	nullable := func(value *float64) interface{} {
		if value == nil {
			return ""
		}
		return *value
	}
	for i, r := range report.Rows {
		var usage, waste, version interface{} = "", "", ""
		reason, actor, at := "", "", ""
		if r.Policy != nil {
			usage = r.Policy.UsageTolerancePct
			waste = r.Policy.WasteTolerancePct
			version = r.Policy.Version
			reason = r.Policy.Reason
			actor = r.Policy.UpdatedBy
			at = r.Policy.UpdatedAt
		}
		counted := "Belum"
		if r.HasStocktake {
			counted = "Ya"
		}
		values := []interface{}{r.WarehouseName, r.OutletName, r.ItemCode, r.ItemName, r.Category, r.BaseUnit, r.StockQty, r.SaleQty, r.ProductionQty, r.ProductionReferenceQty, r.BreakdownQty, r.OtherUsageQty, r.StocktakeShortageQty, r.StocktakeSurplusQty, r.AdjustmentShortageQty, r.TransferOutQty, r.QualityHoldQty, r.ReferenceQty, r.EvaluatedUsageQty, r.ExcessUsageQty, r.WasteQty, r.VoidWasteQty, r.WasteValue, r.ShortageValue, nullable(r.UsageLimitQty), nullable(r.WasteLimitQty), nullable(r.WastePct), nullable(r.UsageVariancePct), usage, waste, version, reason, actor, at, r.StocktakeAt, r.StocktakeNumber, counted, usageStatusLabel(r.Status), usageFlagsText(r.Flags), r.UndocumentedWasteQty}
		row := header + i + 1
		start, _ := excelize.CoordinatesToCellName(1, row)
		if err := f.SetSheetRow(sheet, start, &values); err != nil {
			return nil, err
		}
		end, _ := excelize.CoordinatesToCellName(len(columns), row)
		f.SetCellStyle(sheet, start, end, st.text)
		f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("AD%d", row), st.num)
		f.SetCellStyle(sheet, fmt.Sprintf("W%d", row), fmt.Sprintf("X%d", row), st.money)
	}
	last, _ := excelize.ColumnNumberToName(len(columns))
	f.SetColWidth(sheet, "A", last, 20)
	f.SetColWidth(sheet, "D", "D", 30)
	f.SetColWidth(sheet, "AF", "AF", 45)
	f.SetColWidth(sheet, "AM", "AM", 80)
	ppicAutoFilter(f, sheet, header, header+len(report.Rows), len(columns))
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
