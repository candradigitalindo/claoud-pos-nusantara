package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"strings"
	"testing"
)

func TestBalanceLegacyComplimentHeaderReversesUndatedPaidOrder(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
		TotalAmount: 100000, Version: 1, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
		PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", Method: "compliment"}}
	// Older clients marked the order paid but omitted the optional compliment
	// timestamp. The dated transaction header is the authoritative free-payment proof.
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	id, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
		OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
		PaymentMethod: "compliment", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	var persisted float64
	if err := database.DB.QueryRow(`SELECT total_amount FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, outlet, id).Scan(&persisted); err != nil || persisted != 0 {
		t.Fatalf("fixture did not exercise normalized compliment header: %v %v", persisted, err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-01"), 0, 100000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-02"), 0, 0, 0)
}

func TestBalanceLegacyPaidOrderWithoutPaymentClockNeedsSourceReview(t *testing.T) {
	for _, state := range []string{"uncaptured legacy paid", "first snapshot already paid", "archived unpaid invoice"} {
		t.Run(state, func(t *testing.T) {
			outlet := newPOSRestoreOutlet(t)
			order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
				TotalAmount: 100000, Version: 2, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
				PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, Method: "cash"}}
			if state == "uncaptured legacy paid" {
				if _, err := database.DB.Exec(`INSERT INTO cloud_orders
					(id,local_id,outlet_id,outlet_code,status,total_amount,payment_info,created_at,updated_at)
					VALUES($1::text,$1::text,$2,'LEGACY','served',100000,
					'{"payment_status":"paid","paid_amount":100000,"method":"cash"}',
					'2026-10-01 05:00:00','2026-10-02 05:00:00')`, order.LocalID, outlet); err != nil {
					t.Fatal(err)
				}
			} else {
				if state == "archived unpaid invoice" {
					balanceHistoryOrder(t, outlet, order.LocalID, order.CreatedAt, order.TotalAmount)
				}
				if _, err := SaveOrder(outlet, order); err != nil {
					t.Fatal(err)
				}
			}
			report := balanceHistoryReport(t, outlet, "2026-10-03")
			if state == "archived unpaid invoice" {
				assertBalanceHistory(t, report, 0, 100000, 100000)
			} else {
				// A current paid flag is not evidence of a collectible receivable
				// or of a payment on any particular historical date.
				assertBalanceHistory(t, report, 0, 0, 0)
			}
			if report.ReviewCount == 0 {
				t.Fatal("missing payment date/details were presented without source-review qualification")
			}
		})
	}
}

func TestBalanceLegacyCorroboratedSingleMethodHeaderCompletesPaymentGap(t *testing.T) {
	for _, method := range []string{"cash", "qris"} {
		t.Run(method, func(t *testing.T) {
			outlet := newPOSRestoreOutlet(t)
			order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
				TotalAmount: 100000, Version: 1, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
				PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, Method: method}}
			if _, err := SaveOrder(outlet, order); err != nil {
				t.Fatal(err)
			}
			id, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
				OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
				PaymentMethod: method, Version: 1,
				Payments: []models.PaymentLine{{PaymentMethod: method, Amount: 40000, CreatedAt: "2026-10-01T06:00:00Z"}}})
			if err != nil {
				t.Fatal(err)
			}
			var paymentAmount float64
			if err := database.DB.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM transaction_payments WHERE outlet_id=$1 AND transaction_id=$2`, outlet, id).Scan(&paymentAmount); err != nil || paymentAmount != 40000 {
				t.Fatalf("fixture did not retain the normalized evidence gap: %v %v", paymentAmount, err)
			}
			// The dated line remains on its source day; the independently
			// corroborated final-header remainder belongs to completion day.
			earlier := balanceHistoryReport(t, outlet, "2026-10-01")
			assertBalanceHistory(t, earlier, 40000, 60000, 100000)
			complete := balanceHistoryReport(t, outlet, "2026-10-02")
			assertBalanceHistory(t, complete, 100000, 0, 100000)
			if complete.ReviewCount == 0 {
				t.Fatal("supplemented legacy payment gap lacks source-review qualification")
			}
		})
	}
}

func TestBalanceLegacyUncorroboratedHeaderDoesNotFillPaymentGap(t *testing.T) {
	for _, tc := range []struct {
		name, headerMethod, lineMethod, paidAt, updatedAt string
		archivedUnpaid                                    bool
	}{
		{name: "mixed header", headerMethod: "mixed", lineMethod: "cash", updatedAt: "2026-10-02T05:00:00Z"},
		{name: "different line method", headerMethod: "cash", lineMethod: "qris", updatedAt: "2026-10-02T05:00:00Z"},
		{name: "explicit later payment", headerMethod: "cash", lineMethod: "cash", paidAt: "2026-10-05T05:00:00Z", updatedAt: "2026-10-02T05:00:00Z"},
		{name: "archived later settlement", headerMethod: "cash", lineMethod: "cash", updatedAt: "2026-10-05T05:00:00Z", archivedUnpaid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outlet := newPOSRestoreOutlet(t)
			order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
				TotalAmount: 100000, Version: 2, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: tc.updatedAt,
				PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, Method: tc.headerMethod, PaidAt: tc.paidAt}}
			if tc.archivedUnpaid {
				balanceHistoryOrder(t, outlet, order.LocalID, order.CreatedAt, order.TotalAmount)
			}
			if _, err := SaveOrder(outlet, order); err != nil {
				t.Fatal(err)
			}
			if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
				OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
				PaymentMethod: tc.headerMethod, Version: 1,
				Payments: []models.PaymentLine{{PaymentMethod: tc.lineMethod, Amount: 40000, CreatedAt: "2026-10-02T05:00:00Z"}}}); err != nil {
				t.Fatal(err)
			}
			// Only the actual dated receipt is known by Oct3. A mixed/mismatched
			// method or evidence of later settlement cannot establish the other 60k.
			assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 40000, 60000, 100000)
		})
	}
}

func TestBalanceLegacyExplicitComplimentDateDoesNotErasePriorInvoice(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
		TotalAmount: 100000, Version: 1, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
		PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", Method: "compliment", ComplimentedAt: "2026-10-02T05:00:00Z"}}
	// This is the first captured order state; the explicit date still proves
	// the invoice existed before the later complimentary settlement.
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
		OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
		PaymentMethod: "compliment", Version: 1}); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-01"), 0, 100000, 100000)
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-02"), 0, 0, 0)
}

func TestBalanceLegacyZeroInvoiceUsesVerifiedPositiveSaleHeader(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
		TotalAmount: 0, Version: 1, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
		PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", Method: "cash", PaidAmount: 100000, PaidAt: "2026-10-02T05:00:00Z"}}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
		OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
		PaymentMethod: "cash", Version: 1,
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-10-02T05:00:00Z"}}}); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
}

func TestBalanceLegacyCorroboratedGapAndCrossShiftRefundCancelOnce(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	order := models.PushOrderRequest{LocalID: NewULID(), OutletCode: "LEGACY", Status: "served",
		TotalAmount: 100000, Version: 1, CreatedAt: "2026-10-01T05:00:00Z", UpdatedAt: "2026-10-02T05:00:00Z",
		PaymentInfo: models.PaymentInfo{PaymentStatus: "paid", PaidAmount: 100000, Method: "cash"}}
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "LEGACY",
		OrderID: order.LocalID, TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000,
		PaymentMethod: "cash", Version: 1,
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 40000, CreatedAt: "2026-10-01T06:00:00Z"}}}); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-03"), 100000, 0, 100000)
	shiftID := NewULID()
	if _, err := SaveCashierShift(outlet, models.PushCashierShiftRequest{LocalID: shiftID,
		OpenedBy: "Kasir berikutnya", OpenedAt: "2026-10-08T01:00:00Z", Status: "open"}); err != nil {
		t.Fatal(err)
	}
	voidAt := "2026-10-08T05:00:00Z"
	if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: NewULID(), ShiftID: shiftID,
		MovementType: "out", Amount: 100000, CounterpartName: "Refund void (shift sebelumnya)",
		Note:      "Void order #" + strings.ToUpper(order.LocalID[:8]) + " — dibayar tunai sebelum shift ini dibuka",
		CreatedAt: voidAt}); err != nil {
		t.Fatal(err)
	}
	order.Version, order.UpdatedAt = 2, voidAt
	order.PaymentInfo.VoidedAt, order.PaymentInfo.VoidedBy, order.PaymentInfo.VoidReason = voidAt, "Supervisor", "Refund pembayaran lama"
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	assertBalanceHistory(t, balanceHistoryReport(t, outlet, "2026-10-08"), 0, 0, 0)
}
