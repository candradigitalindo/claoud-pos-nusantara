package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"sync"
	"testing"
)

type shiftAdjustmentFixture struct {
	outlet string
	shift  string
}

func newShiftAdjustmentFixture(t *testing.T) shiftAdjustmentFixture {
	t.Helper()
	f := shiftAdjustmentFixture{outlet: newPOSRestoreOutlet(t)}
	t.Cleanup(func() {
		if _, err := database.DB.Exec(`DELETE FROM pos_void_stock_events WHERE outlet_id=$1`, f.outlet); err != nil {
			t.Error(err)
		}
		if _, err := database.DB.Exec(`DELETE FROM shift_revenue_adjustments WHERE outlet_id=$1`, f.outlet); err != nil {
			t.Error(err)
		}
	})
	var err error
	f.shift, err = SaveCashierShift(f.outlet, models.PushCashierShiftRequest{
		LocalID: NewULID(), OpenedBy: "Kasir rekonsiliasi", ClosedBy: "Kasir rekonsiliasi",
		OpenedAt: "2026-10-21T02:00:00Z", ClosedAt: "2026-10-21T15:00:00Z", Status: "closed",
		Report: map[string]any{"sales_total": 1000000, "sales_count": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, reconciliationPayment(800000, "2026-10-21T06:00:00Z")); err != nil {
		t.Fatal(err)
	}
	return f
}

func reconciliationPayment(amount float64, at string) models.PushTransactionRequest {
	return models.PushTransactionRequest{
		LocalID: NewULID(), Subtotal: amount, TotalAmount: amount,
		PaymentMethod: "cash", CashAmount: amount, TransactionDate: at,
		CashierName: "Kasir rekonsiliasi", Version: 1,
	}
}

func (f shiftAdjustmentFixture) assertRevenue(t *testing.T, revenue, adjustment float64) {
	t.Helper()
	var gotRevenue, gotAdjustment float64
	if err := database.DB.QueryRow(`SELECT COALESCE(SUM(total_amount),0),
		COALESCE(SUM(total_amount) FILTER (WHERE payment_method='adjustment'),0)
		FROM cloud_transactions WHERE outlet_id=$1`, f.outlet).Scan(&gotRevenue, &gotAdjustment); err != nil {
		t.Fatal(err)
	}
	if gotRevenue != revenue || gotAdjustment != adjustment {
		t.Fatalf("revenue=%v adjustment=%v; want %v and %v", gotRevenue, gotAdjustment, revenue, adjustment)
	}
}

func (f shiftAdjustmentFixture) activeAdjustment(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := database.DB.QueryRow(`SELECT id FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestShiftAdjustmentShrinksOnLateSyncAndRetry(t *testing.T) {
	f := newShiftAdjustmentFixture(t)
	if err := ApplyShiftAdjustment(f.shift, "Supervisor", nil); err != nil {
		t.Fatal(err)
	}
	adjustmentID := f.activeAdjustment(t)
	f.assertRevenue(t, 1000000, 200000)
	late := reconciliationPayment(100000, "2026-10-21T08:00:00Z")
	for retry := 0; retry < 2; retry++ {
		if _, err := SaveTransaction(f.outlet, late); err != nil {
			t.Fatal(err)
		}
		f.assertRevenue(t, 1000000, 100000)
	}
	if _, err := SaveTransaction(f.outlet, reconciliationPayment(100000, "2026-10-21T09:00:00Z")); err != nil {
		t.Fatal(err)
	}
	f.assertRevenue(t, 1000000, 0)
	var originalCashier, originalCloud, effective float64
	if err := database.DB.QueryRow(`SELECT cashier_sales, cloud_sales, adjustment_amount FROM shift_revenue_adjustments WHERE id=$1`, adjustmentID).
		Scan(&originalCashier, &originalCloud, &effective); err != nil {
		t.Fatal(err)
	}
	if originalCashier != 1000000 || originalCloud != 800000 || effective != 0 {
		t.Fatalf("approval evidence changed: cashier=%v cloud=%v effective=%v", originalCashier, originalCloud, effective)
	}
	if err := RevertShiftAdjustment(adjustmentID, "Supervisor", nil); err != nil {
		t.Fatal(err)
	}
	f.assertRevenue(t, 1000000, 0)
	var status string
	if err := database.DB.QueryRow(`SELECT status FROM shift_revenue_adjustments WHERE id=$1`, adjustmentID).Scan(&status); err != nil || status != "reverted" {
		t.Fatalf("adjustment history lost: status=%q err=%v", status, err)
	}
}

func TestShiftAdjustmentLateSyncFromAnotherWindowDoesNotConsumeAdjustment(t *testing.T) {
	f := newShiftAdjustmentFixture(t)
	if err := ApplyShiftAdjustment(f.shift, "Supervisor", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, reconciliationPayment(50000, "2026-10-22T06:00:00Z")); err != nil {
		t.Fatal(err)
	}
	f.assertRevenue(t, 1050000, 200000)
}

func TestShiftAdjustmentDoesNotRestoreAdjustmentAfterVoid(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		amount float64
	}{
		{name: "partial", amount: 100000},
		{name: "fully_synchronized", amount: 200000},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newShiftAdjustmentFixture(t)
			if err := ApplyShiftAdjustment(f.shift, "Supervisor", nil); err != nil {
				t.Fatal(err)
			}
			order := models.PushOrderRequest{
				LocalID: NewULID(), Status: "served", TotalAmount: scenario.amount,
				CreatedAt: "2026-10-21T09:00:00Z", UpdatedAt: "2026-10-21T09:00:00Z",
				PaymentInfo: models.PaymentInfo{PaymentStatus: "paid"},
			}
			if _, err := SaveOrder(f.outlet, order); err != nil {
				t.Fatal(err)
			}
			late := reconciliationPayment(scenario.amount, "2026-10-21T09:00:00Z")
			late.OrderID = order.LocalID
			if _, err := SaveTransaction(f.outlet, late); err != nil {
				t.Fatal(err)
			}
			wantAdjustment := 200000 - scenario.amount
			f.assertRevenue(t, 1000000, wantAdjustment)
			order.UpdatedAt = "2026-10-21T09:10:00Z"
			order.PaymentInfo.VoidedAt = order.UpdatedAt
			order.PaymentInfo.VoidedBy = "Supervisor"
			order.PaymentInfo.VoidReason = "Pembatalan setelah sinkron"
			if _, err := SaveOrder(f.outlet, order); err != nil {
				t.Fatal(err)
			}
			f.assertRevenue(t, 800000+wantAdjustment, wantAdjustment)
			if _, err := SaveTransaction(f.outlet, reconciliationPayment(50000, "2026-10-22T06:00:00Z")); err != nil {
				t.Fatal(err)
			}
			f.assertRevenue(t, 850000+wantAdjustment, wantAdjustment)
			var amount float64
			if err := database.DB.QueryRow(`SELECT adjustment_amount FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&amount); err != nil || amount != wantAdjustment {
				t.Fatalf("void recreated adjustment: amount=%v err=%v", amount, err)
			}
		})
	}
}

func TestShiftAdjustmentConcurrentApplyAndLateSync(t *testing.T) {
	f := newShiftAdjustmentFixture(t)
	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errors <- ApplyShiftAdjustment(f.shift, "Supervisor", nil)
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := SaveTransaction(f.outlet, reconciliationPayment(100000, "2026-10-21T08:00:00Z"))
		errors <- err
	}()
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.assertRevenue(t, 1000000, 100000)
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&count); err != nil || count != 1 {
		t.Fatalf("active adjustments=%d err=%v", count, err)
	}
}

func TestShiftAdjustmentConcurrentRevertAndLateSync(t *testing.T) {
	f := newShiftAdjustmentFixture(t)
	if err := ApplyShiftAdjustment(f.shift, "Supervisor", nil); err != nil {
		t.Fatal(err)
	}
	adjustmentID := f.activeAdjustment(t)
	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		errors <- RevertShiftAdjustment(adjustmentID, "Supervisor", nil)
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := SaveTransaction(f.outlet, reconciliationPayment(100000, "2026-10-21T08:00:00Z"))
		errors <- err
	}()
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.assertRevenue(t, 900000, 0)
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&count); err != nil || count != 0 {
		t.Fatalf("active adjustments=%d err=%v", count, err)
	}
}

func TestShiftAdjustmentRefreshRollsBackWithPayment(t *testing.T) {
	f := newShiftAdjustmentFixture(t)
	if err := ApplyShiftAdjustment(f.shift, "Supervisor", nil); err != nil {
		t.Fatal(err)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := lockPOSStock(tx, f.outlet); err != nil {
		t.Fatal(err)
	}
	paymentID := NewULID()
	if _, err := tx.Exec(`INSERT INTO cloud_transactions
		(id,local_id,outlet_id,outlet_code,total_amount,payment_method,created_at)
		VALUES($1,$2,$3,'TEST',100000,'cash','2026-10-21 08:00:00')`, paymentID, paymentID, f.outlet); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO transaction_payments(transaction_id,outlet_id,payment_method,amount,created_at)
		VALUES($1,$2,'cash',100000,'2026-10-21 08:00:00')`, paymentID, f.outlet); err != nil {
		t.Fatal(err)
	}
	if err := refreshShiftAdjustments(tx, f.outlet); err != nil {
		t.Fatal(err)
	}
	var effective float64
	if err := tx.QueryRow(`SELECT adjustment_amount FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&effective); err != nil || effective != 100000 {
		t.Fatalf("adjustment not refreshed inside payment transaction: effective=%v err=%v", effective, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	f.assertRevenue(t, 1000000, 200000)
	if err := database.DB.QueryRow(`SELECT adjustment_amount FROM shift_revenue_adjustments WHERE shift_id=$1 AND status='applied'`, f.shift).Scan(&effective); err != nil || effective != 200000 {
		t.Fatalf("rolled-back sync changed adjustment: effective=%v err=%v", effective, err)
	}
}
