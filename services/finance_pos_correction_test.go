package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"testing"
	"time"
)

func correctionTime(day string) time.Time {
	return financePOSTime(day + "T05:00:00Z")
}

func correctionState(method, day string, amount float64, actual bool) financePOSState {
	s := financePOSState{ID: "transaction", LocalID: "local", OutletID: "outlet", Amount: amount,
		Method: method, CreatedAt: "2026-10-02T05:00:00Z", Costed: true}
	if actual {
		s.Payments = []financePOSPayment{{Method: method, Amount: amount, At: day + "T05:00:00Z"}}
	}
	return s
}

func correctionOperations(revisions []financePOSRevision, voided time.Time, creditFor func(financePOSState, time.Time) string) []models.FinanceOperation {
	var operations []models.FinanceOperation
	financePOSEmitPayments(&financePOSEntity{Kind: "transaction", Revisions: revisions}, voided,
		func(s financePOSState) ([]financePOSPayment, bool) { return financePOSPayments(s), len(s.Payments) > 0 }, creditFor,
		func(at time.Time, source, id string, s financePOSState, debit, credit string, amount float64, category, review string) {
			operation := models.FinanceOperation{At: at, Source: source, ReviewNote: review}
			financePair(&operation, debit, credit, amount)
			operations = append(operations, operation)
		})
	return operations
}

func correctionDebitNet(operations []models.FinanceOperation, account, day string) float64 {
	var amount float64
	for _, operation := range operations {
		if operation.At.After(correctionTime(day).Add(time.Hour)) {
			continue
		}
		for _, posting := range operation.Postings {
			if posting.AccountCode == account {
				amount += posting.Debit - posting.Credit
			}
		}
	}
	return round2(amount)
}

func correctionRevenue(financePOSState, time.Time) string { return glAccRevenue }

func TestFinancePOSPaymentMethodCorrectionPreservesKnownPast(t *testing.T) {
	initial := correctionState("cash", "2026-10-02", 100, true)
	corrected := correctionState(ReservationDpMethod, "2026-10-02", 100, true)
	ops := correctionOperations([]financePOSRevision{
		{State: initial, Observed: correctionTime("2026-10-02")},
		{State: corrected, Observed: correctionTime("2026-10-05")},
	}, time.Time{}, correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-03"), 100, "known earlier cash")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-03"), 0, "deposit not retroactively consumed")
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 0, "cash reclassified once")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-06"), 100, "deposit consumed at correction time")
	financeNear(t, correctionDebitNet(ops, glAccRevenue, "2026-10-06"), -100, "no duplicate revenue")
}

func TestFinancePOSPaymentDateCorrectionDoesNotRetimeCash(t *testing.T) {
	ops := correctionOperations([]financePOSRevision{
		{State: correctionState("cash", "2026-10-02", 100, true), Observed: correctionTime("2026-10-02")},
		{State: correctionState("cash", "2026-10-04", 100, true), Observed: correctionTime("2026-10-05")},
	}, time.Time{}, correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-03"), 100, "payment date correction retains known past")
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 100, "payment date correction does not receive cash again")
	var reviewed bool
	for _, operation := range ops {
		if operation.At.Equal(correctionTime("2026-10-05")) && operation.ReviewNote != "" {
			reviewed = true
		}
	}
	if !reviewed {
		t.Fatal("source date correction has no dated review evidence")
	}
}

func TestFinancePOSPaymentReallocationKeepsTotalReceipts(t *testing.T) {
	initial := correctionState("mixed", "2026-10-02", 100, true)
	initial.Payments = []financePOSPayment{{Method: "cash", Amount: 70, At: initial.CreatedAt}, {Method: ReservationDpMethod, Amount: 30, At: initial.CreatedAt}}
	corrected := initial
	corrected.Payments = []financePOSPayment{{Method: "cash", Amount: 20, At: initial.CreatedAt}, {Method: ReservationDpMethod, Amount: 80, At: initial.CreatedAt}}
	ops := correctionOperations([]financePOSRevision{{State: initial}, {State: corrected, Observed: correctionTime("2026-10-05")}}, time.Time{}, correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-03"), 70, "original cash allocation")
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 20, "corrected cash allocation")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-06"), 80, "corrected deposit allocation")
	financeNear(t, correctionDebitNet(ops, glAccRevenue, "2026-10-06"), -100, "reallocation preserves sale total")
}

func TestFinancePOSHeaderDepositDeltaDoesNotInventCash(t *testing.T) {
	ops := correctionOperations([]financePOSRevision{
		{State: correctionState(ReservationDpMethod, "2026-10-02", 100, false)},
		{State: correctionState(ReservationDpMethod, "2026-10-02", 150, false), Observed: correctionTime("2026-10-05")},
	}, time.Time{}, correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 0, "DP header correction is noncash")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-03"), 100, "original deposit before correction")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-06"), 150, "deposit-only correction")
}

func TestFinancePOSCorrectionAfterVoidDoesNotResurrectReceipt(t *testing.T) {
	ops := correctionOperations([]financePOSRevision{
		{State: correctionState("cash", "2026-10-02", 100, true)},
		{State: correctionState(ReservationDpMethod, "2026-10-02", 100, true), Observed: correctionTime("2026-10-05")},
	}, correctionTime("2026-10-04"), correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-03"), 100, "cash before void")
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 0, "void remains final")
	financeNear(t, correctionDebitNet(ops, glAccCustomerDep, "2026-10-06"), 0, "late source change does not invent DP reversal")
	var review bool
	for _, operation := range ops {
		review = review || operation.ReviewNote != ""
	}
	if !review {
		t.Fatal("post-void correction needs review")
	}
}

func TestFinancePOSAttachingOrderReclassifiesWithoutAnotherReceipt(t *testing.T) {
	initial := correctionState("cash", "2026-10-02", 100, true)
	attached := initial
	attached.OrderID = "order"
	ops := correctionOperations([]financePOSRevision{{State: initial}, {State: attached, Observed: correctionTime("2026-10-05")}}, time.Time{},
		func(s financePOSState, _ time.Time) string {
			if s.OrderID != "" {
				return glAccReceivable
			}
			return glAccRevenue
		})
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-06"), 100, "attaching order does not receive money again")
	financeNear(t, correctionDebitNet(ops, glAccRevenue, "2026-10-03"), -100, "prior orphan source preserved")
	financeNear(t, correctionDebitNet(ops, glAccRevenue, "2026-10-06"), 0, "order invoice becomes the only revenue")
	financeNear(t, correctionDebitNet(ops, glAccReceivable, "2026-10-06"), -100, "receipt clears linked invoice")
}

func TestFinancePOSInitialPaymentAfterVoidCannotCreateEarlierRefund(t *testing.T) {
	ops := correctionOperations([]financePOSRevision{{State: correctionState("cash", "2026-10-10", 100, true)}}, correctionTime("2026-10-08"), correctionRevenue)
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-09"), 0, "no refund before an inconsistent future receipt")
	financeNear(t, correctionDebitNet(ops, glAccCash, "2026-10-11"), 0, "inconsistent payment remains unposted")
	if len(ops) == 0 || ops[0].ReviewNote == "" {
		t.Fatal("inconsistent void/payment dates need review evidence")
	}
}

func TestFinancePOSStoredPaymentCorrectionKeepsFirstSnapshotAndRetry(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-01T05:00:00Z", 100000)
	request := models.PushTransactionRequest{LocalID: NewULID(), OrderID: order.LocalID, OutletCode: "CORRECTION",
		TotalAmount: 100000, Subtotal: 100000, PaymentMethod: "cash", TransactionDate: "2026-10-02T05:00:00Z",
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-10-02T05:00:00Z"}}}
	id, err := SaveTransaction(outlet, request)
	if err != nil {
		t.Fatal(err)
	}
	request.PaymentMethod = ReservationDpMethod
	request.Payments[0].PaymentMethod = ReservationDpMethod
	for retry := 0; retry < 2; retry++ {
		if _, err := SaveTransaction(outlet, request); err != nil {
			t.Fatal(err)
		}
	}
	var snapshots int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM finance_pos_snapshots WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2`, outlet, id).Scan(&snapshots); err != nil || snapshots != 2 {
		t.Fatalf("payment correction/retry snapshots=%d err=%v; want original and correction only", snapshots, err)
	}
	if _, err := database.DB.Exec(`UPDATE finance_pos_snapshots SET observed_at='2026-10-05 05:00:00'
		WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2
		AND id>(SELECT MIN(id) FROM finance_pos_snapshots WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2)`, outlet, id); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
	after := balanceHistoryReport(t, outlet, "2026-10-06")
	assertBalanceHistory(t, after, 0, 0, 100000)
	financeNear(t, after.CustomerDeposits, -100000, "DP consumed at recorded correction time")
}

func TestFinancePOSStoredOrphanAttachmentClearsScopedInvoiceWithoutMoreCash(t *testing.T) {
	a, b := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	localID := NewULID()
	balanceHistoryOrder(t, a, localID, "2026-10-01T05:00:00Z", 50000)
	order := balanceHistoryOrder(t, b, localID, "2026-10-01T05:00:00Z", 100000)
	request := models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "CORRECTION",
		TotalAmount: 100000, Subtotal: 100000, PaymentMethod: "cash", TransactionDate: "2026-10-02T05:00:00Z",
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-10-02T05:00:00Z"}}}
	id, err := SaveTransaction(b, request)
	if err != nil {
		t.Fatal(err)
	}
	request.OrderID = order.LocalID
	for retry := 0; retry < 2; retry++ {
		if _, err := SaveTransaction(b, request); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.DB.Exec(`UPDATE finance_pos_snapshots SET observed_at='2026-10-05 05:00:00'
		WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2
		AND id>(SELECT MIN(id) FROM finance_pos_snapshots WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2)`, b, id); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, b, "2026-10-03"), 100000, 100000, 200000)
	assertBalanceHistory(t, balanceHistoryReport(t, b, "2026-10-06"), 100000, 0, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, a, "2026-10-06"), 0, 50000, 50000)
}
