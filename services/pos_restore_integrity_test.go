package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"reflect"
	"testing"
)

func newPOSRestoreOutlet(t *testing.T) string {
	t.Helper()
	requirePOSIdentity(t)
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets(id,code,name,api_key) VALUES($1,$2,'Restore fixture',$3)`, id, "RST"+id[10:], "restore-test-"+id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, table := range []string{"transaction_payments", "cloud_transactions", "cloud_orders", "cloud_cash_movements", "cloud_cashier_shifts", "cloud_additional_charges", "sync_logs", "pos_entity_ids"} {
			if _, err := database.DB.Exec(`DELETE FROM `+table+` WHERE outlet_id=$1`, id); err != nil {
				t.Error(err)
			}
		}
		if _, err := database.DB.Exec(`DELETE FROM outlets WHERE id=$1`, id); err != nil {
			t.Error(err)
		}
	})
	return id
}

func restoreOrderRequest(id string) models.PushOrderRequest {
	return models.PushOrderRequest{LocalID: id, OutletCode: "RESTORE", Status: "served", TotalAmount: 100,
		Version: 100, CreatedAt: "2026-10-21T06:00:00Z", UpdatedAt: "2026-10-21T06:01:00Z",
		PaymentInfo: models.PaymentInfo{PaymentStatus: "unpaid"}}
}

func assertJSONValueEqual(t *testing.T, want, got []byte) {
	t.Helper()
	var a, b interface{}
	if err := json.Unmarshal(want, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("JSON value changed:\nwant %s\ngot %s", want, got)
	}
}

func TestPOSRestoreSnapshotAndChargeMastersRoundTrip(t *testing.T) {
	outlet, other := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	for _, at := range []string{outlet, other} {
		for i, id := range []string{"restore-pb1", "restore-packaging"} {
			_, err := SaveAdditionalCharge(at, models.PushAdditionalChargeRequest{LocalID: id, Name: id,
				ChargeType: "percentage", Value: 10, IsActive: 1 - i, Version: 1})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	// Simulate the actual JSON decode/typed persistence boundary. Unknown
	// snapshot fields must survive even though the Go server does not use them.
	var req models.PushOrderRequest
	if err := json.Unmarshal([]byte(`{
		"local_id":"restore-snapshot","outlet_code":"RESTORE","status":"served",
		"total_amount":110,"version":100,"created_at":"2026-10-21T06:00:00Z","updated_at":"2026-10-21T06:01:00Z",
		"created_by":"Kasir","orderer_name":"Pelayan","is_holding":true,
		"items":[{"id":"item-1","product_id":"product-1","category_id":"category-1","product_name":"Nasi",
		"qty":1,"price":100,"subtotal":100,"notes":"Tanpa cabai","addons":"[{\"id\":\"addon-1\",\"name\":\"Keju\",\"price\":10}]",
		"waiter_name":"Pelayan","is_additional":1,"created_at":"2026-10-21T06:00:01Z","updated_at":"2026-10-21T06:00:02Z"}],
		"payment_info":{"paid_amount":40,"payment_status":"partial","complimented_at":"2026-10-21T06:00:03Z","compliment_by":"Supervisor","compliment_reason":"Item contoh",
		"restore":{"version":1,"payments":[{"id":"pay-1","amount":40,"payment_method":"cash","payment_note":"DP","created_by":"Kasir","created_at":"2026-10-21T06:00:04Z"}],
		"charges":[{"sync_id":"restore-pb1","is_auto":true,"name":"PB1","charge_type":"percentage","value":10,"applied_amount":10,"created_at":"2026-10-21T06:00:00Z","updated_at":"2026-10-21T06:00:00Z"}],
		"reservation_id":"reservation-local","customer_phone":"0812345","customer_id":"customer-local","is_holding":1,"is_merged":0,"merged_from":null,
		"future_metadata":{"keep":[1,true,"text"]}}}}`), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrder(outlet, req); err != nil {
		t.Fatal(err)
	}
	restored, err := GetRestoreData(outlet)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.UnpaidOrders) != 1 || len(restored.AdditionalCharges) != 2 {
		t.Fatalf("restore missing snapshot/masters: %+v", restored)
	}
	order := restored.UnpaidOrders[0]
	var items []models.OrderItem
	var payment models.PaymentInfo
	if err := json.Unmarshal([]byte(order.Items), &items); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(order.PaymentInfo), &payment); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, req.Items) || order.CreatedBy != "Kasir" || order.OrdererName != "Pelayan" || !order.IsHolding {
		t.Fatalf("restore metadata changed: %+v %+v", order, items)
	}
	if payment.PaidAmount != 40 || payment.ComplimentedAt != req.PaymentInfo.ComplimentedAt || payment.ComplimentBy != "Supervisor" || payment.ComplimentReason != "Item contoh" {
		t.Fatalf("financial metadata changed: %+v", payment)
	}
	assertJSONValueEqual(t, req.PaymentInfo.Restore, payment.Restore)
	masters := map[string]models.RestoreAdditionalCharge{}
	for _, c := range restored.AdditionalCharges {
		masters[c.LocalID] = c
	}
	if !masters["restore-pb1"].IsActive || masters["restore-packaging"].IsActive || masters["restore-pb1"].Value != 10 {
		t.Fatalf("active/inactive charge masters changed: %+v", masters)
	}
	// Future client versions are opaque to this server, not silently stripped.
	req.UpdatedAt = "2026-10-21T06:02:00Z"
	req.PaymentInfo.Restore = json.RawMessage(`{"version":99,"unknown":{"precision":"keep-me"}}`)
	if _, err := SaveOrder(outlet, req); err != nil {
		t.Fatal(err)
	}
	restored, err = GetRestoreData(outlet)
	if err != nil || len(restored.UnpaidOrders) != 1 {
		t.Fatalf("future snapshot: %+v %v", restored, err)
	}
	if err := json.Unmarshal([]byte(restored.UnpaidOrders[0].PaymentInfo), &payment); err != nil {
		t.Fatal(err)
	}
	assertJSONValueEqual(t, req.PaymentInfo.Restore, payment.Restore)
}

func TestPOSRestoreStaleSnapshotCannotLosePaymentState(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	req := restoreOrderRequest(NewULID())
	req.UpdatedAt = "2026-10-21T06:01:00.800Z"
	req.PaymentInfo = models.PaymentInfo{PaymentStatus: "partial", PaidAmount: 40,
		Restore: json.RawMessage(`{"version":1,"payments":[{"id":"pay","amount":40}],"charges":[]}`)}
	if _, err := SaveOrder(outlet, req); err != nil {
		t.Fatal(err)
	}
	stale := req
	stale.UpdatedAt, stale.Version, stale.TotalAmount = "2026-10-21T06:01:00.100Z", 101, 900
	stale.PaymentInfo = models.PaymentInfo{PaymentStatus: "unpaid"}
	for _, version := range []int{101, 100} {
		stale.Version = version
		if _, err := SaveOrder(outlet, stale); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.UnpaidOrders) != 1 || restored.UnpaidOrders[0].TotalAmount != 100 {
		t.Fatalf("stale order replaced current snapshot: %+v %v", restored, err)
	}
	var payment models.PaymentInfo
	if err := json.Unmarshal([]byte(restored.UnpaidOrders[0].PaymentInfo), &payment); err != nil {
		t.Fatal(err)
	}
	assertJSONValueEqual(t, req.PaymentInfo.Restore, payment.Restore)
	// A later source mutation may have a lower enqueue version (another clock).
	req.UpdatedAt, req.Version = "2026-10-21T06:02:00Z", 99
	req.PaymentInfo.PaymentStatus, req.PaymentInfo.PaidAmount = "paid", 100
	if _, err := SaveOrder(outlet, req); err != nil {
		t.Fatal(err)
	}
	stale.Version, stale.UpdatedAt = 0, "2026-10-22T06:00:00Z"
	if _, err := SaveOrder(outlet, stale); err != nil {
		t.Fatal(err)
	}
	restored, err = GetRestoreData(outlet)
	if err != nil || len(restored.UnpaidOrders) != 0 {
		t.Fatalf("paid order reopened: %+v %v", restored, err)
	}
	// Legacy clients without versions still update an ordinary unpaid order.
	legacy := restoreOrderRequest(NewULID())
	legacy.Version, legacy.UpdatedAt = 0, ""
	for _, total := range []float64{100, 150} {
		legacy.TotalAmount = total
		if _, err := SaveOrder(outlet, legacy); err != nil {
			t.Fatal(err)
		}
	}
	var total float64
	if err := database.DB.QueryRow(`SELECT total_amount FROM cloud_orders WHERE outlet_id=$1 AND local_id=$2`, outlet, legacy.LocalID).Scan(&total); err != nil || total != 150 {
		t.Fatalf("legacy update rejected: %v %v", total, err)
	}
}

func TestPOSRestoreCompletedTransactionsAndCanonicalOrderPrecedence(t *testing.T) {
	outlet, other := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	local := NewULID()
	if _, err := SaveTransaction(outlet, models.PushTransactionRequest{LocalID: NewULID(), OrderID: local,
		OutletCode: "RESTORE", TotalAmount: 100, PaymentMethod: "cash", CreatedAt: "2026-10-21T06:02:00Z"}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []string{outlet, other} {
		if _, err := SaveOrder(at, restoreOrderRequest(local)); err != nil {
			t.Fatal(err)
		}
	}
	insertOrder := func(id, local, status string) {
		t.Helper()
		if _, err := database.DB.Exec(`INSERT INTO cloud_orders(id,local_id,outlet_id,outlet_code,status,total_amount,payment_info)
			VALUES($1,$2,$3,'RESTORE',$4,100,'{"payment_status":"unpaid"}')`, id, local, outlet, status); err != nil {
			t.Fatal(err)
		}
	}
	insertTransaction := func(ref string) {
		t.Helper()
		id := NewULID()
		if _, err := database.DB.Exec(`INSERT INTO cloud_transactions(id,local_id,outlet_id,outlet_code,order_id,total_amount)
			VALUES($1,$2,$3,'RESTORE',$4,100)`, id, id, outlet, ref); err != nil {
			t.Fatal(err)
		}
	}
	canonical, alias, legacy := NewULID(), NewULID(), NewULID()
	insertOrder(canonical, NewULID(), "served")
	insertOrder(alias, canonical, "served") // local ID collides with another canonical ID
	insertTransaction(canonical)
	insertOrder(NewULID(), legacy, "served")
	insertTransaction(legacy) // old transactions reference only the local ID
	insertOrder(NewULID(), NewULID(), "completed")
	insertOrder(NewULID(), NewULID(), "voided")
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.UnpaidOrders) != 1 || restored.UnpaidOrders[0].ID != alias {
		t.Fatalf("paid/legacy/alias filtering wrong: %+v %v", restored, err)
	}
	restored, err = GetRestoreData(other)
	if err != nil || len(restored.UnpaidOrders) != 1 || restored.UnpaidOrders[0].LocalID != local {
		t.Fatalf("another outlet's transaction hid unpaid order: %+v %v", restored, err)
	}
}

func TestPOSRestoreLegacyShiftReferencesStayScopedAndCanonicalFirst(t *testing.T) {
	outlet, other := newPOSRestoreOutlet(t), newPOSRestoreOutlet(t)
	openID, openLocal := NewULID(), NewULID()
	closedID, closedLocal, aliasID := NewULID(), NewULID(), NewULID()
	insertShift := func(at, id, local, status, previous string) {
		t.Helper()
		if _, err := database.DB.Exec(`INSERT INTO cloud_cashier_shifts(id,local_id,outlet_id,opened_by,opened_at,opening_cash,status,previous_shift_id)
			VALUES($1,$2,$3,'Kasir','2026-10-21 06:00:00',0,$4,$5)`, id, local, at, status, previous); err != nil {
			t.Fatal(err)
		}
	}
	insertShift(outlet, closedID, closedLocal, "closed", "")
	insertShift(outlet, aliasID, closedID, "open", openLocal)
	insertShift(outlet, openID, openLocal, "open", closedID)
	insertShift(other, NewULID(), openLocal, "open", closedID)
	insertMovement := func(at, ref string) string {
		t.Helper()
		id := NewULID()
		if _, err := database.DB.Exec(`INSERT INTO cloud_cash_movements(id,local_id,outlet_id,shift_id,movement_type,amount,created_at)
			VALUES($1,$2,$3,$4,'in',10,'2026-10-21 06:05:00')`, id, id, at, ref); err != nil {
			t.Fatal(err)
		}
		return id
	}
	want := map[string]bool{insertMovement(outlet, openID): true, insertMovement(outlet, openLocal): true}
	insertMovement(outlet, closedID) // canonical closed beats another open shift's local ID
	insertMovement(other, openLocal)
	insertMovement(other, openID) // foreign canonical ID is not a parent in this outlet
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.OpenShifts) != 2 || len(restored.CashMovements) != 2 {
		t.Fatalf("legacy movement restore: %+v %v", restored, err)
	}
	for _, s := range restored.OpenShifts {
		if s.ID == openID && s.PreviousShiftID != closedLocal {
			t.Fatalf("canonical previous shift not translated: %+v", s)
		}
		if s.ID == aliasID && s.PreviousShiftID != openLocal {
			t.Fatalf("legacy local previous shift changed: %+v", s)
		}
	}
	for _, m := range restored.CashMovements {
		if !want[m.ID] || m.ShiftID != openLocal {
			t.Fatalf("wrong movement/parent: %+v", m)
		}
	}
	restored, err = GetRestoreData(other)
	if err != nil || len(restored.CashMovements) != 1 || restored.OpenShifts[0].PreviousShiftID != closedID {
		t.Fatalf("cross-outlet reference resolved: %+v %v", restored, err)
	}
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM pos_entity_ids WHERE outlet_id IN($1,$2)`, outlet, other).Scan(&count); err != nil || count != 0 {
		t.Fatalf("restore rewrote legacy identities: %d %v", count, err)
	}
}
