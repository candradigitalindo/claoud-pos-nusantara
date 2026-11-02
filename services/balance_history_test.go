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

func balanceHistoryOrder(t *testing.T, outlet, localID, at string, amount float64) models.PushOrderRequest {
	t.Helper()
	order := models.PushOrderRequest{LocalID: localID, OutletCode: "BALANCE", CustomerName: "Pelanggan riwayat",
		Status: "served", TotalAmount: amount, Version: 1, CreatedAt: at, UpdatedAt: at,
		PaymentInfo: models.PaymentInfo{PaymentStatus: "unpaid"}}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	return order
}

func balanceHistoryPayment(t *testing.T, outlet string, order models.PushOrderRequest, at string, payments ...models.PaymentLine) string {
	t.Helper()
	id, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "BALANCE",
		OrderID: order.LocalID, TransactionDate: at, Subtotal: order.TotalAmount, TotalAmount: order.TotalAmount,
		PaymentMethod: "mixed", Payments: payments, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func balanceHistoryReport(t *testing.T, outlet, asOf string) *models.BalanceResponse {
	t.Helper()
	report, err := GetBalanceReport(asOf, asOf, outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func assertBalanceHistory(t *testing.T, report *models.BalanceResponse, cash, receivable, revenue float64) {
	t.Helper()
	financeNear(t, report.CashAndEquivalents, cash, "cash as of closing date")
	financeNear(t, report.Receivables, receivable, "receivable as of closing date")
	financeNear(t, report.TotalRevenue, revenue, "order revenue recognized once")
	financeNear(t, report.BalanceDifference, 0, "balanced source journal")
}

func TestBalanceHistoryLaterPaymentKeepsEarlierReceivable(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 0, 100000, 100000)
	order.Version = 2
	order.UpdatedAt = "2026-10-05T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 100000, CreatedAt: order.UpdatedAt})
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 0, 100000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-05"), 100000, 0, 100000)
	// The start date is accepted for older clients but does not reset balances.
	wide, err := GetBalanceReport("2026-01-01", "2026-10-05", outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, wide, 100000, 0, 100000)
}

func TestBalanceHistoryPartialPaymentsFollowTheirOwnDates(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.Version = 2
	order.UpdatedAt = "2026-10-05T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 40000, CreatedAt: "2026-10-02T05:00:00Z"},
		models.PaymentLine{PaymentMethod: "qris", Amount: 60000, CreatedAt: order.UpdatedAt})
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-01"), 0, 100000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 40000, 60000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-05"), 100000, 0, 100000)
}

func TestBalanceHistoryFutureVoidDoesNotEraseEarlierPayment(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.Version = 2
	order.UpdatedAt = "2026-10-02T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 100000, CreatedAt: order.UpdatedAt})
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
	order.Version = 3
	order.UpdatedAt = "2026-10-08T05:00:00Z"
	order.PaymentInfo.VoidedAt = order.UpdatedAt
	order.PaymentInfo.VoidedBy = "Supervisor"
	order.PaymentInfo.VoidReason = "Pembatalan kemudian"
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
}

func TestBalanceHistoryFutureOrderMutationKeepsEarlierAmount(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.TotalAmount, order.Version, order.UpdatedAt = 150000, 2, "2026-10-05T05:00:00Z"
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 0, 100000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-06"), 0, 150000, 150000)
}

func TestBalanceHistoryPaymentCutoffUsesJakartaMidnight(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	originalTimezone, err := GetTimezone()
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateSettings(map[string]string{"timezone": "Asia/Jakarta"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := UpdateSettings(map[string]string{"timezone": originalTimezone}); err != nil {
			t.Error(err)
		}
	})
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.Version = 2
	order.UpdatedAt = "2026-10-02T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 40000, CreatedAt: "2026-10-01T16:59:59Z"},
		models.PaymentLine{PaymentMethod: "qris", Amount: 60000, CreatedAt: "2026-10-01T17:00:00Z"})
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-01"), 40000, 60000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-02"), 100000, 0, 100000)
}

func TestBalanceHistoryLegacyOrderIDsRemainOutletScoped(t *testing.T) {
	a, b := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	localID := NewULID()
	balanceHistoryOrder(t, a, localID, "2026-10-01T05:00:00Z", 100000)
	balanceHistoryOrder(t, b, localID, "2026-10-01T05:00:00Z", 200000)
	var canonicalB string
	if err := database.DB.QueryRow(`SELECT TRIM(id) FROM cloud_orders WHERE outlet_id=$1 AND local_id=$2`, b, localID).Scan(&canonicalB); err != nil {
		t.Fatal(err)
	}
	if canonicalB == localID {
		t.Fatal("fixture did not exercise a cross-outlet local-ID collision")
	}
	for _, reference := range []string{canonicalB, localID} {
		transactionID := NewULID()
		if _, err := database.DB.Exec(`INSERT INTO cloud_transactions(id,local_id,outlet_id,outlet_code,order_id,total_amount,payment_method,created_at)
			VALUES($1::text,$1::text,$2,'BALANCE',$3,200000,'cash','2026-10-02 05:00:00')`, transactionID, b, reference); err != nil {
			t.Fatal(err)
		}
		assertBalanceHistory(t, balanceHistoryReport(t, b, "2026-10-03"), 200000, 0, 200000)
		assertBalanceHistory(t, balanceHistoryReport(t, a, "2026-10-03"), 0, 100000, 100000)
		if _, err := database.DB.Exec(`DELETE FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, b, transactionID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBalanceHistoryLegacySaleWithoutOrderUsesHeaderOnce(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO cloud_transactions(id,local_id,outlet_id,outlet_code,total_amount,payment_method,created_at)
		VALUES($1::text,$1::text,$2,'BALANCE',75000,'cash','2026-10-02 05:00:00')`, id, outlet); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-01"), 0, 0, 0)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 75000, 0, 75000)
}

func TestBalanceHistoryCrossShiftRefundIsNotCountedTwice(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.Version, order.UpdatedAt = 2, "2026-10-02T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	balanceHistoryPayment(t, outlet, order, order.UpdatedAt,
		models.PaymentLine{PaymentMethod: "cash", Amount: 100000, CreatedAt: order.UpdatedAt})
	shiftLocalID := NewULID()
	if _, err := SaveCashierShift(outlet, models.PushCashierShiftRequest{LocalID: shiftLocalID,
		OpenedBy: "Kasir berikutnya", OpenedAt: "2026-10-08T01:00:00Z", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	voidAt := "2026-10-08T05:00:00Z"
	// Match the original Flutter payload's uppercase eight-character order prefix.
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: NewULID(), ShiftID: shiftLocalID,
		MovementType: "out", Amount: 100000, CounterpartName: "Refund void (shift sebelumnya)",
		Note:      "Void order #" + strings.ToUpper(order.LocalID[:8]) + " — dibayar tunai sebelum shift ini dibuka",
		CreatedAt: voidAt}); err != nil {
		t.Fatal(err)
	}
	order.Version, order.UpdatedAt = 3, voidAt
	order.PaymentInfo.VoidedAt, order.PaymentInfo.VoidedBy, order.PaymentInfo.VoidReason = voidAt, "Supervisor", "Refund lintas shift"
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-08"), 0, 0, 0)
}

func TestBalanceHistoryRetriesAndReportReadsPreserveSourceFacts(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	order.Version, order.UpdatedAt = 2, "2026-10-02T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	payment := models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "BALANCE", OrderID: order.LocalID,
		TransactionDate: order.UpdatedAt, Subtotal: 100000, TotalAmount: 100000, PaymentMethod: "cash", Version: 1,
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 100000, CreatedAt: order.UpdatedAt}}}
	id, err := SaveTransaction(outlet, payment)
	if err != nil {
		t.Fatal(err)
	}
	countSnapshots := func() int {
		t.Helper()
		var count int
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM finance_pos_snapshots WHERE outlet_id=$1`, outlet).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	count := countSnapshots()
	if count == 0 {
		t.Fatal("POS writes did not capture accounting snapshots")
	}
	for retry := 0; retry < 2; retry++ {
		if _, err := SaveOrder(outlet, order); err != nil {
			t.Fatal(err)
		}
		if got, err := SaveTransaction(outlet, payment); err != nil || got != id {
			t.Fatalf("payment retry changed identity: %s %v", got, err)
		}
	}
	if got := countSnapshots(); got != count {
		t.Fatalf("identical retries duplicated financial snapshots: %d -> %d", count, got)
	}
	readHeader := func() string {
		t.Helper()
		var snapshot string
		if err := database.DB.QueryRow(`SELECT to_jsonb(t)::text FROM cloud_transactions t WHERE outlet_id=$1 AND id=$2`, outlet, id).Scan(&snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	header := readHeader()
	report := balanceHistoryReport(t, outlet, "2026-10-03")
	assertBalanceHistory(t, report, 100000, 0, 100000)
	capital := 0.0
	for _, account := range report.Accounts {
		if account.Group == "ekuitas" {
			capital += account.Amount
		}
	}
	financeNear(t, report.TotalEquity, capital+report.TotalRevenue+report.TotalCashIn-report.TotalExpense, "capital plus accumulated operating result")
	financeNear(t, report.TotalEquity, 100000, "nonzero accumulated equity")
	if _, _, err := BuildBalanceReportExcel("2026-01-01", "2026-10-03", outlet, nil); err != nil {
		t.Fatal(err)
	}
	if got := countSnapshots(); got != count {
		t.Fatalf("report/export wrote financial history: %d -> %d", count, got)
	}
	if got := readHeader(); got != header {
		t.Fatalf("historical report rewrote current transaction:\nbefore %s\nafter %s", header, got)
	}
}

func TestBalanceHistoryReservationDepositIsAppliedWithoutDuplicatingCash(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	reservation := newTestReservation(t, outlet, 100000)
	t.Cleanup(func() { cleanupReservation(reservation.ID) })
	if _, err := AddReservationPayment(reservation.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 100000, Method: "cash", PaidAt: "2026-10-01",
	}, "Kasir", true, nil); err != nil {
		t.Fatal(err)
	}
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-02T05:00:00Z", 200000)
	order.Version, order.UpdatedAt = 2, "2026-10-03T05:00:00Z"
	order.PaymentInfo = models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 200000, PaidAt: order.UpdatedAt}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "BALANCE",
		OrderID: order.LocalID, ReservationID: reservation.ID, TransactionDate: order.UpdatedAt,
		Subtotal: 200000, TotalAmount: 200000, PaymentMethod: "mixed", Version: 1,
		Payments: []models.PaymentLine{
			{PaymentMethod: "reservasi_dp", Amount: 100000, CreatedAt: "2026-10-02T05:00:00Z"},
			{PaymentMethod: "cash", Amount: 100000, CreatedAt: order.UpdatedAt},
		},
	}); err != nil {
		t.Fatal(err)
	}
	before := balanceHistoryReport(t, outlet, "2026-10-01")
	assertBalanceHistory(t, before, 100000, 0, 0)
	financeNear(t, before.CustomerDeposits, 100000, "deposit remains a liability before POS usage")
	partial := balanceHistoryReport(t, outlet, "2026-10-02")
	assertBalanceHistory(t, partial, 100000, 100000, 200000)
	financeNear(t, partial.CustomerDeposits, 0, "deposit consumed without receiving its cash again")
	paid := balanceHistoryReport(t, outlet, "2026-10-03")
	assertBalanceHistory(t, paid, 200000, 0, 200000)
	financeNear(t, paid.CustomerDeposits, 0, "settled deposit liability")
}

func TestBalanceHistoryExcelUsesClosingDateAndMatchesJSON(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	report := balanceHistoryReport(t, outlet, "2026-10-03")
	data, filename, err := BuildBalanceReportExcel("2026-01-01", "2026-10-03", outlet, []string{outlet})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filename, "_per_2026-10-03.xlsx") || strings.Contains(filename, "_sd_") {
		t.Fatalf("balance filename describes a flow interval: %s", filename)
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
	values := map[string]string{}
	for _, row := range rows {
		if len(row) > 1 {
			values[row[0]] = row[1]
		}
	}
	if values["Posisi per"] != "3 Okt 2026" || values["Periode"] != "" {
		t.Fatalf("Excel does not identify its as-of date: %+v", values)
	}
	for label, want := range map[string]float64{
		"Kas & Setara Kas (saldo buku per tanggal posisi)": report.CashAndEquivalents,
		"Sisa Piutang Usaha (per tanggal posisi)":          report.Receivables,
		"Total Aset":                   report.TotalAssets,
		"Total Kewajiban":              report.TotalLiabilities,
		"Ekuitas termasuk hasil usaha": report.TotalEquity,
		"Pendapatan Transaksi":         report.TotalRevenue,
	} {
		got, err := strconv.ParseFloat(values[label], 64)
		if err != nil || got != want {
			t.Fatalf("Excel %q=%q (%v); JSON=%v", label, values[label], err, want)
		}
	}
}
