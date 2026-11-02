package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func balancePeriodExportValues(r models.BalanceOutletRow) []float64 {
	return []float64{r.CashAndEquivalents, r.Receivables, r.Inventory, r.FixedAssets, r.ProjectsInProgress,
		r.TotalAssets, r.AccountsPayable, r.CustomerDeposits, r.TaxPayable, r.FinancingLiabilities,
		r.TotalLiabilities, r.TotalEquity, r.TotalRevenue, r.TotalCashIn, r.TotalExpense, r.UnpaidAmount}
}

func balancePeriodExportNumber(t *testing.T, got string, want float64, label string) {
	t.Helper()
	n, err := strconv.ParseFloat(got, 64)
	if err != nil {
		t.Fatalf("%s: not a numeric Excel cell: %q (%v)", label, got, err)
	}
	financeNear(t, n, want, label)
}

func balancePeriodExportFixture(t *testing.T) string {
	t.Helper()
	outlet := newPOSRestoreOutlet(t)
	opening := balanceHistoryOrder(t, outlet, NewULID(), "2026-09-30T05:00:00Z", 100000)
	balanceHistoryPayment(t, outlet, opening, "2026-09-30T06:00:00Z",
		models.PaymentLine{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-09-30T06:00:00Z"})
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-02T05:00:00Z", 50000)
	balanceHistoryPayment(t, outlet, order, "2026-10-03T05:00:00Z",
		models.PaymentLine{PaymentMethod: "cash", Amount: 20000, CreatedAt: "2026-10-03T05:00:00Z"})
	shift := NewULID()
	if _, err := SaveCashierShift(outlet, models.PushCashierShiftRequest{LocalID: shift, OpenedBy: "Uji export",
		OpenedAt: "2026-10-03T00:00:00Z", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: NewULID(), ShiftID: shift,
		MovementType: "out", Amount: 5000, CounterpartName: "Operasional uji", Note: "Pengeluaran periode",
		CreatedAt: "2026-10-03T06:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	other := newPOSRestoreOutlet(t)
	hidden := balanceHistoryOrder(t, other, NewULID(), "2026-10-02T05:00:00Z", 999999)
	balanceHistoryPayment(t, other, hidden, "2026-10-02T06:00:00Z",
		models.PaymentLine{PaymentMethod: "cash", Amount: 999999, CreatedAt: "2026-10-02T06:00:00Z"})
	return outlet
}

func TestBalancePeriodExcelMatchesAPIForBothDatesAndOutletScope(t *testing.T) {
	outlet := balancePeriodExportFixture(t)
	readSources := func() string {
		t.Helper()
		var snapshot string
		query := "SELECT jsonb_build_object(" +
			"'orders',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM cloud_orders t WHERE outlet_id=$1)," +
			"'transactions',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM cloud_transactions t WHERE outlet_id=$1)," +
			"'payments',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM transaction_payments t WHERE outlet_id=$1)," +
			"'cash',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM cloud_cash_movements t WHERE outlet_id=$1)," +
			"'history',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM finance_pos_snapshots t WHERE outlet_id=$1))::text"
		if err := database.DB.QueryRow(query, outlet).Scan(&snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	before := readSources()
	for _, from := range []string{"2026-10-01", "2026-10-03"} {
		t.Run(from, func(t *testing.T) {
			report, err := GetBalanceReport(from, "2026-10-03", "", []string{outlet})
			if err != nil {
				t.Fatal(err)
			}
			data, filename, err := BuildBalanceReportExcel(from, "2026-10-03", "", []string{outlet})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(filename, "_"+from+"_2026-10-03.xlsx") || strings.Contains(filename, "_per_") {
				t.Fatalf("filename does not identify both applied dates: %s", filename)
			}
			book, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			defer book.Close()
			rows, err := book.GetRows("Neraca", excelize.Options{RawCellValue: true})
			if err != nil {
				t.Fatal(err)
			}
			kv := map[string]string{}
			comparison := 0
			for i, row := range rows {
				if len(row) > 1 {
					kv[row[0]] = row[1]
				}
				if len(row) > 0 && row[0] == "Komponen" {
					comparison = i + 1
					if len(row) != 4 || !strings.Contains(row[1], "Sebelum") || !strings.Contains(row[2], "Mutasi") || !strings.Contains(row[3], "3 Okt 2026") {
						t.Fatalf("opening/period/closing column labels unclear: %+v", row)
					}
				}
			}
			fromLabel := "1 Okt 2026"
			if from == "2026-10-03" {
				fromLabel = "3 Okt 2026"
			}
			if kv["Tanggal Mulai"] != fromLabel || kv["Tanggal Akhir"] != "3 Okt 2026" ||
				kv["Periode"] != fromLabel+" s.d. 3 Okt 2026" {
				t.Fatalf("selected range missing from workbook: %+v", kv)
			}
			for label, want := range map[string]float64{
				"Penerimaan Kas Periode":       report.Period.CashReceipts,
				"Pengeluaran Kas Periode":      report.Period.CashPayments,
				"Arus Kas Bersih Periode":      report.Period.CashAndEquivalents,
				"Perubahan Piutang Usaha":      report.Period.Receivables,
				"Perubahan Total Aset":         report.Period.TotalAssets,
				"Pendapatan Transaksi Periode": report.Period.TotalRevenue,
				"Pendapatan Lainnya Periode":   report.Period.TotalCashIn,
				"Beban Periode":                report.Period.TotalExpense,
				"Selisih Neraca Penutup":       report.BalanceDifference,
				"Catatan Pemeriksaan Periode":  float64(report.Period.ReviewCount),
				"Catatan Pemeriksaan Penutup":  float64(report.ReviewCount),
			} {
				balancePeriodExportNumber(t, kv[label], want, label)
			}
			// Establish that primary values exclude opening cash rather than
			// merely reproducing the closing position returned by the API.
			balancePeriodExportNumber(t, kv["Penerimaan Kas Periode"], 20000, "period gross receipts")
			balancePeriodExportNumber(t, kv["Pengeluaran Kas Periode"], 5000, "period gross payments")
			balancePeriodExportNumber(t, kv["Arus Kas Bersih Periode"], 15000, "period net excludes opening cash")
			if comparison == 0 {
				t.Fatal("opening/period/closing comparison missing")
			}
			closing := models.BalanceOutletRow{CashAndEquivalents: report.CashAndEquivalents, Receivables: report.Receivables,
				Inventory: report.Inventory, FixedAssets: report.FixedAssets, ProjectsInProgress: report.ProjectsInProgress,
				TotalAssets: report.TotalAssets, AccountsPayable: report.AccountsPayable, CustomerDeposits: report.CustomerDeposits,
				TaxPayable: report.TaxPayable, FinancingLiabilities: report.FinancingLiabilities, TotalLiabilities: report.TotalLiabilities,
				TotalEquity: report.TotalEquity, TotalRevenue: report.TotalRevenue, TotalCashIn: report.TotalCashIn,
				TotalExpense: report.TotalExpense, UnpaidAmount: report.UnpaidAmount}
			for col, section := range []models.BalanceOutletRow{report.Opening.BalanceOutletRow, report.Period.BalanceOutletRow, closing} {
				for i, want := range balancePeriodExportValues(section) {
					row := rows[comparison+i]
					balancePeriodExportNumber(t, row[col+1], want, row[0])
				}
			}
			for _, section := range []struct {
				sheet string
				rows  []models.BalanceOutletRow
			}{{"Outlet Awal", report.Opening.Outlets}, {"Outlet Periode", report.Period.Outlets}, {"Outlet Akhir", report.Outlets}} {
				exportRows, err := book.GetRows(section.sheet, excelize.Options{RawCellValue: true})
				if err != nil {
					t.Fatal(err)
				}
				if len(section.rows) != 1 || section.rows[0].OutletID != outlet {
					t.Fatalf("fixture scoped report contains an unexpected outlet: %+v", section.rows)
				}
				got := exportRows[4]
				if got[0] != section.rows[0].OutletName || exportRows[5][0] != "TOTAL" {
					t.Fatalf("outlet rows or total missing in %s: %+v", section.sheet, exportRows)
				}
				for i, want := range balancePeriodExportValues(section.rows[0]) {
					balancePeriodExportNumber(t, got[i+1], want, section.sheet+" outlet")
					balancePeriodExportNumber(t, exportRows[5][i+1], want, section.sheet+" total")
				}
			}
			accountRows, err := book.GetRows("Rincian Akun", excelize.Options{RawCellValue: true})
			if err != nil {
				t.Fatal(err)
			}
			byCode := map[string][]float64{}
			for index, accounts := range [][]models.FinanceAccountTotal{report.Opening.Accounts, report.Period.Accounts, report.Accounts} {
				for _, account := range accounts {
					if byCode[account.Code] == nil {
						byCode[account.Code] = make([]float64, 3)
					}
					byCode[account.Code][index] = account.Amount
				}
			}
			if len(accountRows)-1 != len(byCode) {
				t.Fatal("account comparison omitted or duplicated an account")
			}
			for _, row := range accountRows[1:] {
				for i, want := range byCode[row[0]] {
					balancePeriodExportNumber(t, row[i+3], want, "account "+row[0])
				}
			}
		})
	}
	if after := readSources(); after != before {
		t.Fatal("reading API/export changed the original financial evidence")
	}
}

func TestBalancePeriodExcelEmptyRangeKeepsOpeningAndClosingSeparate(t *testing.T) {
	outlet := balancePeriodExportFixture(t)
	data, _, err := BuildBalanceReportExcel("2026-10-04", "2026-10-05", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("Neraca", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if len(row) > 1 && (row[0] == "Penerimaan Kas Periode" || row[0] == "Pengeluaran Kas Periode" || row[0] == "Arus Kas Bersih Periode") {
			balancePeriodExportNumber(t, row[1], 0, row[0])
		}
		if len(row) == 4 && row[0] == "Kas & Setara Kas" {
			balancePeriodExportNumber(t, row[1], 115000, "opening cash before empty range")
			balancePeriodExportNumber(t, row[2], 0, "empty range cash movement")
			balancePeriodExportNumber(t, row[3], 115000, "closing cash after empty range")
		}
	}
	periodRows, err := book.GetRows("Outlet Periode", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	if periodRows[4][0] != "TOTAL" {
		t.Fatalf("empty period must have only a zero total: %+v", periodRows)
	}
	balancePeriodExportNumber(t, periodRows[4][1], 0, "empty outlet period total")
}
