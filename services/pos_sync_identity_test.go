package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func requirePOSIdentity(t *testing.T) {
	t.Helper()
	requireDB(t)
	if err := database.RunPOSIdentityMigrations(); err != nil {
		t.Fatal(err)
	}
}

func TestPOSIdentityChildBeforeParentAndOutletIsolation(t *testing.T) {
	requirePOSIdentity(t)
	a, b := newPOSVoidFixture(t), newPOSVoidFixture(t)
	b.order.LocalID = a.order.LocalID
	b.payment.LocalID, b.payment.OrderID = a.payment.LocalID, a.order.LocalID
	// Both payments arrive before their parents. The second outlet must not
	// take the first outlet's identity, stock, customer, payments, or void state.
	ta, err := SaveTransaction(a.outlet, a.payment)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := SaveTransaction(b.outlet, b.payment)
	if err != nil {
		t.Fatal(err)
	}
	ob, err := SaveOrder(b.outlet, b.order)
	if err != nil {
		t.Fatal(err)
	}
	oa, err := SaveOrder(a.outlet, a.order)
	if err != nil {
		t.Fatal(err)
	}
	if oa != a.order.LocalID || ta != a.payment.LocalID || oa == ob || ta == tb || len(ob) != 26 || len(tb) != 26 {
		t.Fatalf("identities: orders %q/%q transactions %q/%q", oa, ob, ta, tb)
	}
	for _, f := range []*posVoidFixture{a, b} {
		for n := 0; n < 2; n++ {
			if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
				t.Fatal(err)
			}
		}
		stock, _ := f.ledger(t, f.itemID)
		if !bdNear(stock, 82) {
			t.Fatalf("retry changed stock: %v", stock)
		}
		var link, parentOutlet string
		if err := database.DB.QueryRow(`SELECT t.order_id,o.outlet_id FROM cloud_transactions t
			JOIN cloud_orders o ON o.id=t.order_id WHERE t.outlet_id=$1`, f.outlet).Scan(&link, &parentOutlet); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(parentOutlet) != f.outlet {
			t.Fatalf("cross-outlet parent link: %s", link)
		}
	}
	if _, err := SaveOrder(b.outlet, b.void()); err != nil {
		t.Fatal(err)
	}
	b.assertWaste(t, 82, 18, 36, 1)
	var remaining int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1`, a.outlet).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("other outlet's payment was removed: %d %v", remaining, err)
	}
	r, err := GetSalesReport("2026-10-21", "2026-10-21", a.outlet, nil, 1, 20)
	if err != nil || r.Summary.TotalTransactions != 1 || !bdNear(r.Summary.TotalRevenue, 10000) {
		t.Fatalf("other outlet report changed: %+v %v", r, err)
	}
}

func TestPOSIdentityConcurrentAllocationAndLegacyCanonicalID(t *testing.T) {
	requirePOSIdentity(t)
	a, b := newPOSVoidFixture(t), newPOSVoidFixture(t)
	b.order.LocalID = a.order.LocalID
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for n := 0; n < 12; n++ {
		f := a
		if n%2 != 0 {
			f = b
		}
		wg.Add(1)
		go func(f *posVoidFixture) {
			defer wg.Done()
			_, err := SaveOrder(f.outlet, f.order)
			errs <- err
		}(f)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count, distinct int
	if err := database.DB.QueryRow(`SELECT COUNT(*),COUNT(DISTINCT id) FROM cloud_orders WHERE local_id=$1`, a.order.LocalID).Scan(&count, &distinct); err != nil || count != 2 || distinct != 2 {
		t.Fatalf("allocation duplicated/lost rows: %d/%d %v", count, distinct, err)
	}
	// Simulate a pre-registry row whose canonical ID already differs from its
	// local ID. Adoption must never overwrite that ID or break later children.
	canonical, local := NewULID(), NewULID()
	if _, err := database.DB.Exec(`INSERT INTO cloud_orders(id,local_id,outlet_id,outlet_code,total_amount)
		VALUES($1,$2,$3,'TEST',10000)`, canonical, local, a.outlet); err != nil {
		t.Fatal(err)
	}
	req := a.order
	req.LocalID = local
	id, err := SaveOrder(a.outlet, req)
	if err != nil || id != canonical {
		t.Fatalf("legacy canonical changed: %s %v", id, err)
	}
	pay := a.payment
	pay.LocalID, pay.OrderID = NewULID(), local
	if _, err := SaveTransaction(a.outlet, pay); err != nil {
		t.Fatal(err)
	}
	var linked string
	if err := database.DB.QueryRow(`SELECT order_id FROM cloud_transactions WHERE outlet_id=$1 AND local_id=$2`, a.outlet, pay.LocalID).Scan(&linked); err != nil || strings.TrimSpace(linked) != canonical {
		t.Fatalf("legacy parent link: %q %v", linked, err)
	}
	// Older rows may still reference the local ID. Voiding that parent must
	// reuse the original consumption rather than deducting a second recipe.
	if _, err := database.DB.Exec(`UPDATE cloud_transactions SET order_id=$1 WHERE outlet_id=$2 AND local_id=$3`, local, a.outlet, pay.LocalID); err != nil {
		t.Fatal(err)
	}
	req.PaymentInfo = a.void().PaymentInfo
	if _, err := SaveOrder(a.outlet, req); err != nil {
		t.Fatal(err)
	}
	a.assertWaste(t, 82, 18, 36, 1)
	report, err := GetSalesReport("2026-10-21", "2026-10-21", a.outlet, nil, 1, 20)
	if err != nil || report.Summary.TotalTransactions != 0 || report.Summary.TotalRevenue != 0 {
		t.Fatalf("legacy void still counted: %+v %v", report, err)
	}
}

func TestPOSShiftIdentityMovementBeforeParentAndRestore(t *testing.T) {
	requirePOSIdentity(t)
	a, b := newBreakdownFixture(t), newBreakdownFixture(t)
	for _, f := range []*bdFixture{a, b} {
		outlet := f.outlet
		t.Cleanup(func() {
			execScratch(`DELETE FROM cloud_cash_movements WHERE outlet_id=$1`, outlet)
			execScratch(`DELETE FROM cloud_cashier_shifts WHERE outlet_id=$1`, outlet)
			execScratch(`DELETE FROM sync_logs WHERE outlet_id=$1`, outlet)
		})
	}
	local, movement := NewULID(), NewULID()
	ids := map[string]string{}
	for _, f := range []*bdFixture{a, b} {
		m := models.PushCashMovementRequest{LocalID: movement, ShiftID: local, MovementType: "in", Amount: 5000, CreatedAt: "2026-10-21T06:02:00Z"}
		for n := 0; n < 2; n++ {
			if _, err := SaveCashMovement(f.outlet, m); err != nil {
				t.Fatal(err)
			}
		}
		shift := models.PushCashierShiftRequest{LocalID: local, OpenedBy: "Kasir", OpenedAt: "2026-10-21T06:00:00Z", Status: "open"}
		id, err := SaveCashierShift(f.outlet, shift)
		if err != nil {
			t.Fatal(err)
		}
		ids[f.outlet] = id
		var parent string
		var count int
		if err := database.DB.QueryRow(`SELECT MIN(shift_id),COUNT(*) FROM cloud_cash_movements WHERE outlet_id=$1`, f.outlet).Scan(&parent, &count); err != nil || strings.TrimSpace(parent) != id || count != 1 {
			t.Fatalf("movement parent/count: %q/%d %v", parent, count, err)
		}
		restored, err := GetRestoreData(f.outlet)
		if err != nil {
			t.Fatal(err)
		}
		if len(restored.OpenShifts) != 1 || restored.OpenShifts[0].LocalID != local || len(restored.CashMovements) != 1 || restored.CashMovements[0].ShiftID != local {
			t.Fatalf("restore must use local identities: %+v", restored)
		}
	}
	if ids[a.outlet] != local || ids[b.outlet] == local || len(ids[b.outlet]) != 26 {
		t.Fatalf("shift collision: %+v", ids)
	}
}

func TestPOSLegacySaleProductAndPaymentShiftTimes(t *testing.T) {
	requirePOSIdentity(t)
	f := newPOSVoidFixture(t)
	t.Cleanup(func() { execScratch(`DELETE FROM cloud_cashier_shifts WHERE outlet_id=$1`, f.outlet) })
	shiftIDs := []string{NewULID(), NewULID()}
	for i, id := range shiftIDs {
		shift := models.PushCashierShiftRequest{LocalID: id, OpenedBy: "Kasir", ClosedBy: "Kasir", Status: "closed",
			OpenedAt: fmt.Sprintf("2026-10-21T0%d:00:00Z", 6+i), ClosedAt: fmt.Sprintf("2026-10-21T0%d:00:00Z", 7+i)}
		if _, err := SaveCashierShift(f.outlet, shift); err != nil {
			t.Fatal(err)
		}
		// An incomplete close retry must not move opened_at to cloud receipt time.
		shift.OpenedAt = ""
		if _, err := SaveCashierShift(f.outlet, shift); err != nil {
			t.Fatal(err)
		}
	}
	pay := f.payment
	pay.Items = append([]models.TransactionItem(nil), pay.Items...)
	pay.Items[0].ProductID = ""
	pay.CreatedAt = "2026-10-21T07:30:00Z"
	pay.Payments = []models.PaymentLine{
		{PaymentMethod: "cash", Amount: 3000, CreatedAt: "2026-10-21T13:05:00+07:00"},
		{PaymentMethod: "qris", Amount: 7000, CreatedAt: "2026-10-21T07:30:00Z"},
	}
	for i := 0; i < 2; i++ {
		if _, err := SaveTransaction(f.outlet, pay); err != nil {
			t.Fatal(err)
		}
	}
	stock, _ := f.ledger(t, f.itemID)
	if !bdNear(stock, 82) {
		t.Fatalf("legacy sale stock skipped/doubled: %v", stock)
	}
	var firstAt, lastAt string
	if err := database.DB.QueryRow(`SELECT MIN(created_at)::text,MAX(created_at)::text FROM transaction_payments WHERE outlet_id=$1`, f.outlet).Scan(&firstAt, &lastAt); err != nil || firstAt != "2026-10-21 06:05:00" || lastAt != "2026-10-21 07:30:00" {
		t.Fatalf("payment times: %q/%q %v", firstAt, lastAt, err)
	}
	report, err := GetCashierShiftReport(f.outlet, "closed", "2026-10-21", "2026-10-21", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Shifts) != 2 {
		t.Fatalf("receipt date displaced shifts: %+v", report.Shifts)
	}
	for _, shift := range report.Shifts {
		want := 3000.0
		if strings.TrimSpace(shift.ID) == shiftIDs[1] {
			want = 7000
		}
		if !bdNear(shift.SalesTotal, want) || shift.SalesCount != 1 || shift.SalesSource != "cloud" {
			t.Fatalf("partial payment assigned to wrong shift: %+v want %v", shift, want)
		}
	}
	// No product guessing when two exact names exist in the same outlet.
	other := f.productRecipeFixture.product(t, "")
	if _, err := database.DB.Exec(`UPDATE cloud_products SET name=$1 WHERE id=$2`, f.product.Name, other.ID); err != nil {
		t.Fatal(err)
	}
	pay.LocalID = NewULID()
	if _, err := SaveTransaction(f.outlet, pay); err == nil {
		t.Fatal("ambiguous product must fail and remain retryable")
	}
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1 AND local_id=$2`, f.outlet, pay.LocalID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("ambiguous sale partially committed: %d %v", n, err)
	}
}

func TestPOSRetryWithoutTimestampKeepsPersistedTime(t *testing.T) {
	requirePOSIdentity(t)
	f := newPOSVoidFixture(t)
	pay := models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "TEST", TotalAmount: 1000, PaymentMethod: "cash"}
	id, err := SaveTransaction(f.outlet, pay)
	if err != nil {
		t.Fatal(err)
	}
	var original string
	if err := database.DB.QueryRow(`SELECT created_at::text FROM cloud_transactions WHERE id=$1`, id).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, pay); err != nil {
		t.Fatal(err)
	}
	var replay string
	if err := database.DB.QueryRow(`SELECT created_at::text FROM transaction_payments WHERE transaction_id=$1`, id).Scan(&replay); err != nil || replay != original {
		t.Fatalf("timestamp-less replay moved payment time: %q -> %q: %v", original, replay, err)
	}
}

func TestPOSReservationSettlementFailureRetriesWithoutDuplicateSale(t *testing.T) {
	requirePOSIdentity(t)
	f := newPOSVoidFixture(t)
	r, err := CreateReservation(models.ReservationRequest{
		OutletID: f.outlet, CustomerName: "Uji retry reservasi", Pax: 1,
		ReservationDate: "2026-10-21", ReservationTime: "13:00", DownPayment: 5000,
		Items: []models.ReservationItem{{ProductName: f.product.Name, Qty: 1, Price: 10000}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupReservation(r.ID) })
	proof, err := AddReservationPayment(r.ID, models.ReservationPaymentRequest{
		Type: "dp", Amount: 5000, Method: "transfer", ProofURL: "/uploads/uji-pending.jpg",
	}, "pelanggan", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	pay := f.payment
	pay.ReservationID = r.ID
	if _, err := SaveTransaction(f.outlet, pay); err == nil || !strings.Contains(err.Error(), "tersimpan") {
		t.Fatalf("pending proof must retain outbox retry: %v", err)
	}
	// Cash was paid at the POS; the outstanding transfer proof is rejected.
	if _, err := RejectReservationPayment(r.ID, proof.ID, "bukti duplikat", "admin", nil); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if _, err := SaveTransaction(f.outlet, pay); err != nil {
			t.Fatal(err)
		}
	}
	reservation, err := GetReservation(r.ID, nil)
	if err != nil || reservation.Status != "done" || reservation.PosTransactionID != pay.LocalID {
		t.Fatalf("reservation retry did not settle: %+v %v", reservation, err)
	}
	var rows int
	var paid float64
	if err := database.DB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(amount),0) FROM transaction_payments WHERE outlet_id=$1`, f.outlet).Scan(&rows, &paid); err != nil || rows != 1 || !bdNear(paid, 10000) {
		t.Fatalf("settlement retry duplicated payment: %d %v %v", rows, paid, err)
	}
	stock, _ := f.ledger(t, f.itemID)
	if !bdNear(stock, 82) {
		t.Fatalf("settlement retry duplicated stock consumption: %v", stock)
	}
}
