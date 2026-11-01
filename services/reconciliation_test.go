package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"strings"
	"testing"
	"time"
)

func newReconciliationOutlet(t *testing.T) string {
	t.Helper()
	outlet := newPOSRestoreOutlet(t)
	// Run before the restore fixture removes the outlet and its shifts.
	t.Cleanup(func() {
		if _, err := database.DB.Exec(`DELETE FROM shift_revenue_adjustments WHERE outlet_id=$1`, outlet); err != nil {
			t.Error(err)
		}
	})
	return outlet
}

func reconciliationShift(t *testing.T, outlet, openedAt, closedAt string, report interface{}) string {
	t.Helper()
	id, err := SaveCashierShift(outlet, models.PushCashierShiftRequest{
		LocalID: NewULID(), OpenedBy: "Kasir rekonsiliasi", ClosedBy: "Kasir rekonsiliasi",
		OpenedAt: openedAt, ClosedAt: closedAt, Status: "closed", Report: report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func reconciliationSale(t *testing.T, outlet, at string, amount float64, payments ...models.PaymentLine) string {
	t.Helper()
	method := "cash"
	if len(payments) > 1 {
		method = "mixed"
	}
	id, err := SaveTransaction(outlet, models.PushTransactionRequest{
		LocalID: NewULID(), OutletCode: "RECON", CashierName: "Kasir rekonsiliasi",
		TransactionDate: at, Subtotal: amount, TotalAmount: amount, PaymentMethod: method,
		CashAmount: amount, Payments: payments, Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func reconciliationRow(t *testing.T, report *models.ShiftReconReport, shiftID string) models.ShiftReconRow {
	t.Helper()
	for _, row := range report.Shifts {
		if row.ShiftID == shiftID {
			return row
		}
	}
	t.Fatalf("shift %s missing from reconciliation: %+v", shiftID, report.Shifts)
	return models.ShiftReconRow{}
}

func TestShiftReconciliationUsesEventTimesAndDateFilter(t *testing.T) {
	outlet := newReconciliationOutlet(t)
	id := reconciliationShift(t, outlet, "2026-10-21T01:00:00Z", "2026-10-21T09:00:00Z",
		map[string]interface{}{"sales_total": 100000, "sales_count": 1})
	// A closed offline shift can arrive days after all of its sales occurred.
	// Keep the actual sync boundary deterministic and different from the event.
	if _, err := database.DB.Exec(`UPDATE cloud_cashier_shifts SET created_at='2026-10-28 01:00:00',updated_at='2026-10-28 01:00:00' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	reconciliationSale(t, outlet, "2026-10-21T05:00:00Z", 100000)
	report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, []string{outlet})
	if err != nil {
		t.Fatal(err)
	}
	row := reconciliationRow(t, report, id)
	if len(report.Shifts) != 1 || row.CloudSales != 100000 || row.CloudCount != 1 || row.Diff != 0 || row.Status != "balanced" {
		t.Fatalf("fully synced offline shift incorrectly differs: %+v", report)
	}
	if !strings.HasPrefix(row.OpenedAt, "2026-10-21 ") || !strings.HasPrefix(row.ClosedAt, "2026-10-21 ") || row.OpenedAt == row.ClosedAt {
		t.Fatalf("displayed receipt times rather than event times: %+v", row)
	}
	receivedDay, err := GetShiftReconciliation("2026-10-28", "2026-10-28", outlet, nil)
	if err != nil || len(receivedDay.Shifts) != 0 {
		t.Fatalf("offline shift leaked into receipt-date filter: %+v %v", receivedDay, err)
	}
	if err := ApplyShiftAdjustment(id, "test admin", []string{outlet}); err == nil {
		t.Fatal("balanced offline shift accepted an adjustment")
	}
}

func TestShiftReconciliationAllocatesPartialPaymentsToTheirShift(t *testing.T) {
	outlet := newReconciliationOutlet(t)
	first := reconciliationShift(t, outlet, "2026-10-21T06:00:00Z", "2026-10-21T07:00:00Z",
		map[string]interface{}{"sales_total": 3000, "sales_count": 0})
	second := reconciliationShift(t, outlet, "2026-10-21T07:00:00Z", "2026-10-21T08:00:00Z",
		map[string]interface{}{"sales_total": 7000, "sales_count": 1})
	reconciliationSale(t, outlet, "2026-10-21T07:30:00Z", 10000,
		models.PaymentLine{PaymentMethod: "cash", Amount: 3000, CreatedAt: "2026-10-21T13:05:00+07:00"},
		models.PaymentLine{PaymentMethod: "qris", Amount: 7000, CreatedAt: "2026-10-21T07:30:00Z"})
	report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]float64{first: 3000, second: 7000} {
		row := reconciliationRow(t, report, id)
		wantCount := 0
		if id == second {
			wantCount = 1
		}
		if row.CloudSales != want || row.CloudCount != wantCount || row.Diff != 0 || row.Status != "balanced" {
			t.Fatalf("partial payment assigned to final transaction shift: %+v; want %v", row, want)
		}
		if err := ApplyShiftAdjustment(id, "test admin", nil); err == nil {
			t.Fatalf("fully synced partial-payment shift %s accepted adjustment", id)
		}
	}
}

func TestShiftReconciliationLegacyAndFreeTransactions(t *testing.T) {
	outlet := newReconciliationOutlet(t)
	id := reconciliationShift(t, outlet, "2026-10-21T06:00:00Z", "2026-10-21T07:00:00Z",
		map[string]interface{}{"sales_total": 10000, "sales_count": 3})
	legacySale := reconciliationSale(t, outlet, "2026-10-21T06:05:00Z", 10000)
	// Historical headers can predate transaction_payments. Revenue must still
	// be counted once without duplicating newer transactions with payment rows.
	if _, err := database.DB.Exec(`DELETE FROM transaction_payments WHERE transaction_id=$1 AND outlet_id=$2`, legacySale, outlet); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{
		LocalID: NewULID(), OutletCode: "RECON", TransactionDate: "2026-10-21T06:15:00Z",
		TotalAmount: 50000, Subtotal: 50000, PaymentMethod: "compliment", Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	reconciliationSale(t, outlet, "2026-10-21T06:25:00Z", 0)
	report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := reconciliationRow(t, report, id)
	if row.CloudSales != 10000 || row.CloudCount != 3 || row.Status != "balanced" {
		t.Fatalf("legacy or free transaction miscounted: %+v", row)
	}
}

func TestShiftReconciliationClosingBoundaryIsExclusive(t *testing.T) {
	outlet := newReconciliationOutlet(t)
	first := reconciliationShift(t, outlet, "2026-10-21T06:00:00Z", "2026-10-21T07:00:00Z",
		map[string]interface{}{"sales_total": 0, "sales_count": 0})
	second := reconciliationShift(t, outlet, "2026-10-21T07:00:00Z", "2026-10-21T08:00:00Z",
		map[string]interface{}{"sales_total": 10000, "sales_count": 1})
	reconciliationSale(t, outlet, "2026-10-21T07:00:00Z", 10000,
		models.PaymentLine{PaymentMethod: "cash", Amount: 10000, CreatedAt: "2026-10-21T07:00:00Z"})
	report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]float64{first: 0, second: 10000} {
		row := reconciliationRow(t, report, id)
		if row.CloudSales != want || row.Diff != 0 || row.Status != "balanced" {
			t.Fatalf("closing-boundary payment counted in wrong shift: %+v; want %v", row, want)
		}
	}
}

func TestShiftReconciliationOutletScopeAndAdjustmentLifecycle(t *testing.T) {
	outlet, other := newReconciliationOutlet(t), newReconciliationOutlet(t)
	id := reconciliationShift(t, outlet, "2026-10-21T06:00:00Z", "2026-10-21T07:00:00Z",
		map[string]interface{}{"sales_total": 10000, "sales_count": 1})
	for _, scope := range [][]string{{other}, {}} {
		report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, scope)
		if err != nil || len(report.Shifts) != 0 {
			t.Fatalf("explicit outlet bypassed scope %v: %+v %v", scope, report, err)
		}
		if err := ApplyShiftAdjustment(id, "outside admin", scope); err == nil {
			t.Fatalf("adjustment accepted outside scope %v", scope)
		}
	}
	if err := ApplyShiftAdjustment(id, "test admin", []string{outlet}); err != nil {
		t.Fatal(err)
	}
	if err := ApplyShiftAdjustment(id, "test admin", nil); err == nil {
		t.Fatal("duplicate active adjustment accepted")
	}
	report, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	row := reconciliationRow(t, report, id)
	if !row.Adjusted || row.AdjustmentAmount != 10000 || row.AdjustmentID == 0 {
		t.Fatalf("applied adjustment missing: %+v", row)
	}
	for _, scope := range [][]string{{other}, {}} {
		if err := RevertShiftAdjustment(row.AdjustmentID, "outside admin", scope); err == nil {
			t.Fatalf("revert accepted outside scope %v", scope)
		}
	}
	var active int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM shift_revenue_adjustments WHERE id=$1 AND status='applied'`, row.AdjustmentID).Scan(&active); err != nil || active != 1 {
		t.Fatalf("denied revert changed active adjustment: %d %v", active, err)
	}
	if err := RevertShiftAdjustment(row.AdjustmentID, "test admin", []string{outlet}); err != nil {
		t.Fatal(err)
	}
	if err := RevertShiftAdjustment(row.AdjustmentID, "test admin", nil); err == nil {
		t.Fatal("already reverted adjustment accepted")
	}
	var syntheticCount int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1 AND payment_method='adjustment'`, outlet).Scan(&syntheticCount); err != nil || syntheticCount != 0 {
		t.Fatalf("revert left synthetic revenue: %d %v", syntheticCount, err)
	}
	// A reverted adjustment must not permanently prevent a new correction.
	if err := ApplyShiftAdjustment(id, "test admin", []string{outlet}); err != nil {
		t.Fatalf("reapply after revert: %v", err)
	}
}

func TestShiftAdjustmentRequiresValidCashierReport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report interface{}
	}{
		{name: "missing", report: nil},
		{name: "empty", report: map[string]interface{}{}},
		{name: "missing sales total", report: map[string]interface{}{"sales_count": 1}},
		{name: "non numeric sales total", report: map[string]interface{}{"sales_total": "10000"}},
		{name: "null sales total", report: map[string]interface{}{"sales_total": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outlet := newReconciliationOutlet(t)
			id := reconciliationShift(t, outlet, "2026-10-21T06:00:00Z", "2026-10-21T07:00:00Z", tc.report)
			if err := ApplyShiftAdjustment(id, "test admin", nil); err == nil {
				t.Fatalf("invalid cashier report accepted: %#v", tc.report)
			}
			if _, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil); err == nil {
				t.Fatalf("invalid report silently shown as zero cashier sales: %#v", tc.report)
			}
			var adjustments, revenue int
			if err := database.DB.QueryRow(`SELECT COUNT(*) FROM shift_revenue_adjustments WHERE outlet_id=$1`, outlet).Scan(&adjustments); err != nil {
				t.Fatal(err)
			}
			if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1`, outlet).Scan(&revenue); err != nil {
				t.Fatal(err)
			}
			if adjustments != 0 || revenue != 0 {
				t.Fatalf("invalid reports changed revenue: %d adjustments, %d transactions", adjustments, revenue)
			}
		})
	}
}

func TestShiftAdjustmentRejectsInvalidEventWindow(t *testing.T) {
	outlet := newReconciliationOutlet(t)
	id := reconciliationShift(t, outlet, "2026-10-21T07:00:00Z", "2026-10-21T06:00:00Z",
		map[string]interface{}{"sales_total": 10000, "sales_count": 1})
	if err := ApplyShiftAdjustment(id, "test admin", nil); err == nil {
		t.Fatal("shift with closing before opening accepted an adjustment")
	}
	if _, err := GetShiftReconciliation("2026-10-21", "2026-10-21", outlet, nil); err == nil {
		t.Fatal("invalid event window silently compared as zero cloud sales")
	}
}

func TestShiftReconciliationPropagatesSalesQueryErrors(t *testing.T) {
	requireDB(t)
	tx, err := database.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// PostgreSQL aborts only this transaction. Other fixtures and the shared
	// schema remain usable throughout the failure test.
	if _, err := tx.Exec(`SELECT 1/0`); err == nil {
		t.Fatal("expected isolated transaction failure")
	}
	opened := time.Date(2026, time.October, 21, 6, 0, 0, 0, time.UTC)
	if _, _, err := cloudSalesInWindow(tx, "unused-outlet", opened, opened.Add(time.Hour)); err == nil {
		t.Fatal("failed sales query silently treated as zero sales")
	}
}
