package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"testing"
)

func reservationClearingFixture(t *testing.T, settled string) (string, *models.Reservation, string) {
	t.Helper()
	outlet := newPOSRestoreOutlet(t)
	reservation := newTestReservation(t, outlet, 100000)
	t.Cleanup(func() { cleanupReservation(reservation.ID) })
	if _, err := AddReservationPayment(reservation.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 100000, Method: "cash", PaidAt: "2026-09-30",
	}, "Kasir", true, nil); err != nil {
		t.Fatal(err)
	}
	transactionID := NewULID()
	if err := SettleReservation(outlet, reservation.ID, transactionID, "Kasir"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`UPDATE reservations SET settled_at=$2::timestamp WHERE id=$1`, reservation.ID, settled); err != nil {
		t.Fatal(err)
	}
	return outlet, reservation, transactionID
}

func reservationClearingLedger(t *testing.T, from, to, outlet string) models.GeneralLedgerAccount {
	t.Helper()
	report, err := GetGeneralLedger(from, to, outlet, "2-350", []string{outlet})
	if err != nil {
		t.Fatal(err)
	}
	account := glAccount(report, "2-350")
	if account == nil {
		t.Fatal("reservation clearing account missing")
	}
	return *account
}

func TestFinanceReservationClearingDailyLedgerIsAdditive(t *testing.T) {
	for _, tc := range []struct {
		name, settled, usedAt, closingDay, usageDay string
	}{
		{"later sync", "2026-10-02 05:00:00", "2026-10-05T05:00:00Z", "2026-10-02", "2026-10-05"},
		{"Jakarta midnight", "2026-10-01 16:59:59", "2026-10-01T17:00:00Z", "2026-10-01", "2026-10-02"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outlet, _, transactionID := reservationClearingFixture(t, tc.settled)
			if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: transactionID,
				OutletCode: "DP", TransactionDate: tc.usedAt, TotalAmount: 200000, PaymentMethod: "mixed",
				Payments: []models.PaymentLine{
					{PaymentMethod: "reservasi_dp", Amount: 100000, CreatedAt: tc.usedAt},
					{PaymentMethod: "cash", Amount: 100000, CreatedAt: tc.usedAt},
				},
			}); err != nil {
				t.Fatal(err)
			}
			closed := reservationClearingLedger(t, tc.closingDay, tc.closingDay, outlet)
			used := reservationClearingLedger(t, tc.usageDay, tc.usageDay, outlet)
			month := reservationClearingLedger(t, "2026-10-01", "2026-10-31", outlet)
			financeNear(t, closed.Balance, 100000, "clearing created on reservation settlement")
			financeNear(t, used.Balance, -100000, "clearing reversed on actual POS usage date")
			financeNear(t, month.Balance, 0, "clearing resolved by month end")
			financeNear(t, month.TotalDebit, closed.TotalDebit+used.TotalDebit, "daily debits add to monthly debits")
			financeNear(t, month.TotalCredit, closed.TotalCredit+used.TotalCredit, "daily credits add to monthly credits")
			if len(closed.Entries) != 1 || closed.Entries[0].Date != tc.closingDay || len(used.Entries) != 1 || used.Entries[0].Date != tc.usageDay {
				t.Fatalf("clearing has unstable source dates: closed=%+v used=%+v", closed.Entries, used.Entries)
			}
		})
	}
}

func TestFinanceReservationClearingTracksLaterDepositsRefundsAndPOSVoid(t *testing.T) {
	outlet, reservation, transactionID := reservationClearingFixture(t, "2026-10-02 05:00:00")
	// Legacy validated records can arrive with later payment dates. They must
	// change clearing on those dates without rewriting the settlement event.
	for _, payment := range []struct {
		kind, at string
		amount   float64
	}{{"dp", "2026-10-03 05:00:00", 50000}, {"refund", "2026-10-04 05:00:00", 20000}} {
		if _, err := database.DB.Exec(`INSERT INTO reservation_payments
			(id,reservation_id,outlet_id,type,amount,method,paid_at,status)
			VALUES($1,$2,$3,$4,$5,'cash',$6::timestamp,'validated')`, NewULID(), reservation.ID, outlet, payment.kind, payment.amount, payment.at); err != nil {
			t.Fatal(err)
		}
	}
	order := balanceHistoryOrder(t, outlet, NewULID(), "2026-10-04T05:00:00Z", 200000)
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: transactionID, OrderID: order.LocalID,
		OutletCode: "DP", TransactionDate: "2026-10-05T05:00:00Z", TotalAmount: 200000, PaymentMethod: "mixed",
		Payments: []models.PaymentLine{
			{PaymentMethod: "reservasi_dp", Amount: 100000, CreatedAt: "2026-10-05T05:00:00Z"},
			{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-10-05T05:00:00Z"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	order.Version, order.UpdatedAt = 2, "2026-10-06T05:00:00Z"
	order.PaymentInfo.VoidedAt, order.PaymentInfo.VoidedBy, order.PaymentInfo.VoidReason = order.UpdatedAt, "Supervisor", "Batal reservasi POS"
	if _, err := SaveOrder(outlet, order); err != nil {
		t.Fatal(err)
	}
	var dailyDebit, dailyCredit float64
	for day, want := range map[string]float64{
		"2026-10-02": 100000, "2026-10-03": 50000, "2026-10-04": -20000,
		"2026-10-05": -100000, "2026-10-06": 100000,
	} {
		account := reservationClearingLedger(t, day, day, outlet)
		financeNear(t, account.Balance, want, "clearing delta on "+day)
		dailyDebit += account.TotalDebit
		dailyCredit += account.TotalCredit
	}
	month := reservationClearingLedger(t, "2026-10-01", "2026-10-31", outlet)
	financeNear(t, month.TotalDebit, dailyDebit, "later source changes retain daily debit additivity")
	financeNear(t, month.TotalCredit, dailyCredit, "later source changes retain daily credit additivity")
	financeNear(t, month.Balance, 130000, "remaining validated deposit after POS void")
}

func TestFinanceReservationClearingMethodCorrectionKeepsEarlierPending(t *testing.T) {
	outlet, _, transactionID := reservationClearingFixture(t, "2026-10-01 05:00:00")
	payment := models.PushTransactionRequest{LocalID: transactionID, OutletCode: "DP",
		TransactionDate: "2026-10-02T05:00:00Z", TotalAmount: 100000, PaymentMethod: "cash", Version: 1,
		Payments: []models.PaymentLine{{PaymentMethod: "cash", Amount: 100000, CreatedAt: "2026-10-02T05:00:00Z"}}}
	id, err := SaveTransaction(outlet, payment)
	if err != nil {
		t.Fatal(err)
	}
	before := reservationClearingLedger(t, "2026-10-01", "2026-10-03", outlet)
	financeNear(t, before.Balance, 100000, "DP still pending after an actual cash payment")
	payment.Version, payment.PaymentMethod, payment.Payments[0].PaymentMethod = 2, "reservasi_dp", "reservasi_dp"
	if _, err := SaveTransaction(outlet, payment); err != nil {
		t.Fatal(err)
	}
	// The POS did not supply a correction clock. Freeze only the observation
	// date of the new financial revision to make the temporal contract explicit.
	if _, err := database.DB.Exec(`UPDATE finance_pos_snapshots SET observed_at='2026-11-02 05:00:00'
		WHERE id=(SELECT MAX(id) FROM finance_pos_snapshots WHERE outlet_id=$1 AND entity_kind='transaction' AND source_id=$2)`, outlet, id); err != nil {
		t.Fatal(err)
	}
	financeNear(t, reservationClearingLedger(t, "2026-10-01", "2026-10-03", outlet).Balance, 100000,
		"later method correction must not retroactively clear earlier DP")
	financeNear(t, reservationClearingLedger(t, "2026-10-02", "2026-10-02", outlet).Balance, 0,
		"original cash payment creates no DP clearing event")
	financeNear(t, reservationClearingLedger(t, "2026-11-02", "2026-11-02", outlet).Balance, -100000,
		"DP clearing reverses on method correction observation date")
	financeNear(t, reservationClearingLedger(t, "2026-10-01", "2026-11-02", outlet).Balance, 0,
		"corrected DP fully cleared")
}
