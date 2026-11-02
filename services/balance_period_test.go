package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func balancePeriodOperation(date, outlet, debit, credit string, amount float64, review string) models.FinanceOperation {
	op := models.FinanceOperation{Date: date, OutletID: outlet, OutletName: outlet, ReviewNote: review}
	financePair(&op, debit, credit, amount)
	return op
}

func assertBalancePeriodAddsToClosing(t *testing.T, report *models.BalanceResponse) {
	t.Helper()
	opening := reflect.ValueOf(report.Opening.BalanceOutletRow)
	period := reflect.ValueOf(report.Period.BalanceOutletRow)
	closing := reflect.ValueOf(*report)
	for i := 0; i < opening.NumField(); i++ {
		if opening.Field(i).Kind() != reflect.Float64 {
			continue
		}
		name := opening.Type().Field(i).Name
		financeNear(t, opening.Field(i).Float()+period.Field(i).Float(), closing.FieldByName(name).Float(), name+" opening + period = closing")
	}
	for i, account := range report.Accounts {
		financeNear(t, report.Opening.Accounts[i].Amount+report.Period.Accounts[i].Amount, account.Amount, account.Code+" opening + period = closing")
	}
	if report.Opening.ReviewCount+report.Period.ReviewCount != report.ReviewCount {
		t.Fatal("review notes did not stay within their own date sections")
	}
	for _, section := range []models.BalanceSection{report.Opening, report.Period} {
		financeNear(t, section.CashReceipts-section.CashPayments, section.CashAndEquivalents, "cash receipts less payments = section movement")
	}
}

func TestBalancePeriodTodayExcludesOpeningCash(t *testing.T) {
	ops := []models.FinanceOperation{
		balancePeriodOperation("2026-10-31", "A", glAccCash, "3-100", 1000000000, ""),
		balancePeriodOperation("2026-10-31", "A", glAccReceivable, glAccRevenue, 100000, "Riwayat awal"),
		balancePeriodOperation("2026-11-02", "A", glAccCash, glAccReceivable, 40000, "HPP belum tertaut"),
		balancePeriodOperation("2026-11-02", "A", glAccOpex, glAccCash, 10000, ""),
		balancePeriodOperation("2026-11-02", "A", "", "", 0, "Bukti perlu ditinjau"),
		balancePeriodOperation("2026-11-03", "A", glAccCash, glAccRevenue, 999999, "Masa depan"),
	}
	today := buildFinanceBalance("2026-11-02", "2026-11-02", ops)
	financeNear(t, today.Opening.CashAndEquivalents, 1000000000, "opening cash stays separate")
	financeNear(t, today.Period.CashAndEquivalents, 30000, "today net cash excludes all opening receipts")
	financeNear(t, today.Period.CashReceipts, 40000, "today receipts")
	financeNear(t, today.Period.CashPayments, 10000, "today payments")
	financeNear(t, today.Period.Receivables, -40000, "today receivable reduction")
	financeNear(t, today.Period.TotalRevenue, 0, "collecting an older invoice is not new revenue")
	financeNear(t, today.Period.TotalExpense, 10000, "today expense")
	financeNear(t, today.CashAndEquivalents, 1000030000, "legacy top-level cash remains closing")
	if today.Opening.ReviewCount != 1 || today.Period.ReviewCount != 2 || today.ReviewCount != 3 {
		t.Fatalf("review note dates: opening=%d period=%d closing=%d", today.Opening.ReviewCount, today.Period.ReviewCount, today.ReviewCount)
	}
	assertBalancePeriodAddsToClosing(t, today)

	rangeReport := buildFinanceBalance("2026-10-31", "2026-11-02", ops)
	financeNear(t, rangeReport.Period.CashAndEquivalents, 1000030000, "wider period includes earlier cash")
	financeNear(t, rangeReport.Period.TotalRevenue, 100000, "wider period includes invoice creation")
	financeNear(t, rangeReport.CashAndEquivalents, today.CashAndEquivalents, "same closing date preserves closing cash")
	assertBalancePeriodAddsToClosing(t, rangeReport)
}

func TestBalancePeriodEmptyDayDoesNotRepeatOpeningReviewNotes(t *testing.T) {
	ops := []models.FinanceOperation{
		balancePeriodOperation("2026-10-31", "A", glAccCash, glAccRevenue, 120000, "Riwayat lama"),
		balancePeriodOperation("2026-10-31", "A", glAccOpex, glAccCash, 20000, "Klasifikasi lama"),
	}
	report := buildFinanceBalance("2026-11-02", "2026-11-02", ops)
	financeNear(t, report.Opening.CashAndEquivalents, 100000, "empty-day opening")
	financeNear(t, report.Period.CashAndEquivalents, 0, "empty-day movement")
	financeNear(t, report.Period.TotalRevenue, 0, "empty-day revenue")
	financeNear(t, report.Period.TotalExpense, 0, "empty-day expense")
	if report.Period.ReviewCount != 0 || len(report.Period.Outlets) != 0 || report.Period.Outlets == nil {
		t.Fatalf("empty period contains historical rows or warnings: %+v", report.Period)
	}
	for _, account := range report.Period.Accounts {
		financeNear(t, account.Amount, 0, "empty-day account "+account.Code)
	}
	assertBalancePeriodAddsToClosing(t, report)
}

func TestBalancePeriodGrossCashUsesEveryPosting(t *testing.T) {
	// A correction can contain receipts and payments within one operation.
	// Netting the operation first would incorrectly report 100/0, not 120/20.
	op := models.FinanceOperation{Date: "2026-11-02", OutletID: "A", OutletName: "A",
		Postings: []models.FinancePosting{
			{AccountCode: glAccCash, Debit: 120}, {AccountCode: glAccRevenue, Credit: 120},
			{AccountCode: glAccOpex, Debit: 20}, {AccountCode: glAccCash, Credit: 20},
		}}
	report := buildFinanceBalance("2026-11-02", "2026-11-02", []models.FinanceOperation{op})
	financeNear(t, report.Period.CashReceipts, 120, "gross cash receipts")
	financeNear(t, report.Period.CashPayments, 20, "gross cash payments")
	financeNear(t, report.Period.CashAndEquivalents, 100, "net cash")
	financeNear(t, report.Period.BalanceDifference, 0, "period equation")
	financeNear(t, report.Period.Outlets[0].CashAndEquivalents, 100, "outlet movement")
	assertBalancePeriodAddsToClosing(t, report)
	encoded, err := json.Marshal(report.Period)
	if err != nil {
		t.Fatal(err)
	}
	var section map[string]interface{}
	if err := json.Unmarshal(encoded, &section); err != nil {
		t.Fatal(err)
	}
	if section["cash_and_equivalents"] != float64(100) || section["cash_receipts"] != float64(120) || section["cash_payments"] != float64(20) {
		t.Fatalf("period monetary fields are not flat JSON: %s", encoded)
	}
}

func TestBalancePeriodStoredPartialPaymentCrossesSelectedDates(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-31T05:00:00Z", 100000)
	order.Version, order.UpdatedAt = 2, "2026-11-02T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 40000, CreatedAt: "2026-11-01T05:00:00Z"},
		models.PaymentLine{PaymentMethod: "qris", Amount: 60000, CreatedAt: order.UpdatedAt})
	report, err := GetBalanceReport("2026-11-02", "2026-11-02", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, report.Opening.CashAndEquivalents, 40000, "earlier partial receipt stays opening")
	financeNear(t, report.Opening.Receivables, 60000, "opening unpaid portion")
	financeNear(t, report.Period.CashReceipts, 60000, "selected-day QRIS receipt")
	financeNear(t, report.Period.Receivables, -60000, "selected-day receivable settlement")
	financeNear(t, report.Period.TotalRevenue, 0, "later settlement does not repeat old invoice")
	assertBalanceHistory(t, report, 100000, 0, 100000)
	assertBalancePeriodAddsToClosing(t, report)

	wide, err := GetBalanceReport("2026-10-31", "2026-11-02", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, wide.Period.CashReceipts, 100000, "range includes both receipts")
	financeNear(t, wide.Period.TotalRevenue, 100000, "range includes original invoice")
	assertBalancePeriodAddsToClosing(t, wide)
}

func TestBalancePeriodKeepsOpeningAndMovementOutletScoped(t *testing.T) {
	a, b := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	for _, fixture := range []struct {
		outlet string
		amount float64
	}{{a, 100000}, {b, 200000}} {
		order := balanceHistoryOrder(t, fixture.outlet, NewULID(), "2026-11-01T05:00:00Z", fixture.amount)
		order.Version, order.UpdatedAt = 2, "2026-11-02T05:00:00Z"
		order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: fixture.amount, PaidAt: order.UpdatedAt}
		if _, err := SaveOrder(fixture.outlet, order); err != nil {
			t.Fatal(err)
		}
		balanceHistoryPayment(t, fixture.outlet, order, order.UpdatedAt,
			models.PaymentLine{PaymentMethod: "cash", Amount: fixture.amount, CreatedAt: order.UpdatedAt})
	}
	report, err := GetBalanceReport("2026-11-02", "2026-11-02", "", []string{a})
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, report.Opening.Receivables, 100000, "scope-limited opening")
	financeNear(t, report.Period.CashReceipts, 100000, "scope-limited receipts")
	for _, rows := range [][]models.BalanceOutletRow{report.Opening.Outlets, report.Period.Outlets, report.Outlets} {
		if len(rows) != 1 || rows[0].OutletID != a {
			t.Fatalf("section leaked another outlet: %+v", rows)
		}
	}
	assertBalancePeriodAddsToClosing(t, report)
	if _, err := GetBalanceReport("2026-11-02", "2026-11-02", b, []string{a}); !errors.Is(err, ErrFinanceAccess) {
		t.Fatalf("out-of-scope outlet accepted: %v", err)
	}
	empty, err := GetBalanceReport("2026-11-02", "2026-11-02", "", []string{})
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, empty.Opening.TotalAssets, 0, "empty scope opening")
	financeNear(t, empty.Period.TotalAssets, 0, "empty scope period")
	financeNear(t, empty.TotalAssets, 0, "empty scope closing")
}

func balancePeriodAsset(t *testing.T, outlet, purchased string, price float64, life int) {
	t.Helper()
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO assets
		(id,outlet_id,name,quantity,unit,purchase_date,purchase_price,useful_life_months,residual_value,is_deleted,status)
		VALUES($1,$2,'Aset uji periode',1,'unit',$3,$4,$5,0,false,'aktif')`, id, outlet, purchased, price, life); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := database.DB.Exec(`DELETE FROM assets WHERE id=$1`, id); err != nil {
			t.Error(err)
		}
	})
}

func TestBalancePeriodDepreciationUsesSelectedStart(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	balancePeriodAsset(t, outlet, "2026-01-15", 1200000, 12)
	for _, tc := range []struct {
		name, from, to, previous string
		opening, movement        float64
	}{
		{"anniversary before period", "2026-03-20", "2026-03-25", "2026-03-19", 200000, 0},
		{"anniversary within period", "2026-03-10", "2026-03-20", "2026-03-09", 100000, 100000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := GetBalanceReport(tc.from, tc.to, outlet, nil)
			if err != nil {
				t.Fatal(err)
			}
			prior, err := GetBalanceReport(tc.previous, tc.previous, outlet, nil)
			if err != nil {
				t.Fatal(err)
			}
			financeNear(t, report.Opening.TotalExpense, tc.opening, "depreciation already earned before selected start")
			financeNear(t, report.Period.TotalExpense, tc.movement, "depreciation earned within selected interval")
			financeNear(t, report.Period.FixedAssets, -tc.movement, "period book-value movement")
			financeNear(t, report.Opening.FixedAssets, prior.FixedAssets, "opening book value matches previous closing")
			financeNear(t, report.Opening.TotalExpense, prior.TotalExpense, "opening expense matches previous closing")

			// The boundary affects attribution only. Other reports retain their
			// original default monthly journal and identical closing amounts.
			legacyOps, err := loadFinanceJournal("1900-01-01", tc.to, outlet, nil)
			if err != nil {
				t.Fatal(err)
			}
			legacy := buildFinanceBalance(tc.from, tc.to, legacyOps)
			financeNear(t, report.FixedAssets, legacy.FixedAssets, "unchanged closing fixed assets")
			financeNear(t, report.TotalExpense, legacy.TotalExpense, "unchanged closing expense")
			financeNear(t, report.TotalEquity, legacy.TotalEquity, "unchanged closing equity")
			assertBalancePeriodAddsToClosing(t, report)
		})
	}
}

func TestBalancePeriodDepreciationPreservesRoundedMonthlyClosing(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	// Each asset depreciates by 1/3 per completed month. In March one
	// anniversary precedes the selected start and the other follows it:
	// the existing 0.67 monthly total must split into 0.33 + 0.34.
	balancePeriodAsset(t, outlet, "2026-01-05", 1, 3)
	balancePeriodAsset(t, outlet, "2026-01-25", 1, 3)
	report, err := GetBalanceReport("2026-03-20", "2026-03-31", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := GetBalanceReport("2026-03-19", "2026-03-19", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, report.Opening.TotalExpense, 1, "opening rounded depreciation")
	financeNear(t, report.Period.TotalExpense, .34, "period includes monthly rounding remainder")
	financeNear(t, report.TotalExpense, 1.34, "closing preserves two rounded monthly totals")
	financeNear(t, report.Opening.FixedAssets, prior.FixedAssets, "rounded opening matches previous closing")
	legacyOps, err := loadFinanceJournal("1900-01-01", "2026-03-31", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacy := buildFinanceBalance("2026-03-20", "2026-03-31", legacyOps)
	financeNear(t, report.FixedAssets, legacy.FixedAssets, "rounded closing book value is unchanged")
	financeNear(t, report.TotalExpense, legacy.TotalExpense, "rounded closing expense is unchanged")
	assertBalancePeriodAddsToClosing(t, report)
}
