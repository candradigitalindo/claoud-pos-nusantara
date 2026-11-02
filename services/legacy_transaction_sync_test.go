package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"reflect"
	"testing"
)

func newLegacyTransactionFixture(t *testing.T) *posVoidFixture {
	t.Helper()
	requirePOSIdentity(t)
	return newPOSVoidFixture(t)
}

func legacyTransactionRequest(f *posVoidFixture) models.PushTransactionRequest {
	pay := f.payment
	pay.Items = append([]models.TransactionItem(nil), f.payment.Items...)
	pay.Items[0].ProductID = ""
	pay.Items[0].MenuKind = "food"
	pay.Items[0].WaiterName = "Petugas lama"
	pay.PaymentMethod = "mixed"
	pay.Payments = []models.PaymentLine{
		{PaymentMethod: "cash", Amount: 6000, CreatedAt: "2026-10-21T06:04:00Z"},
		{PaymentMethod: "qris", Amount: 4000, CreatedAt: "2026-10-21T06:05:00Z"},
	}
	return pay
}

func duplicateLegacyProductName(t *testing.T, f *posVoidFixture) models.CloudProduct {
	t.Helper()
	other := f.productRecipeFixture.product(t, "")
	if _, err := database.DB.Exec(`UPDATE cloud_products SET name=$1 WHERE id=$2`, f.product.Name, other.ID); err != nil {
		t.Fatal(err)
	}
	return other
}

func readLegacyTransactionItems(t *testing.T, outlet, localID string) (string, []models.TransactionItem) {
	t.Helper()
	var id, raw string
	var amount float64
	if err := database.DB.QueryRow(`SELECT TRIM(id),total_amount,items::text FROM cloud_transactions WHERE outlet_id=$1 AND local_id=$2`, outlet, localID).Scan(&id, &amount, &raw); err != nil {
		t.Fatal(err)
	}
	if amount != 10000 {
		t.Fatalf("legacy transaction amount changed: %v", amount)
	}
	var items []models.TransactionItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatal(err)
	}
	var count int
	var paid float64
	if err := database.DB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(amount),0) FROM transaction_payments WHERE outlet_id=$1 AND transaction_id=$2`, outlet, id).Scan(&count, &paid); err != nil {
		t.Fatal(err)
	}
	if count != 2 || paid != 10000 {
		t.Fatalf("legacy payment details missing or duplicated: count=%d amount=%v", count, paid)
	}
	return id, items
}

func assertLegacyTransactionStock(t *testing.T, f *posVoidFixture, want float64) {
	t.Helper()
	stock, _ := f.ledger(t, f.itemID)
	if !bdNear(stock, want) {
		t.Fatalf("legacy transaction stock=%v; want %v", stock, want)
	}
}

func assertLegacyIdentityTask(t *testing.T, outlet, transactionID, status string, original []models.TransactionItem) string {
	t.Helper()
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_deduction_failures WHERE outlet_id=$1 AND transaction_id=$2 AND failure_kind='identity'`, outlet, transactionID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("identity work missing or duplicated: count=%d %v", count, err)
	}
	var id, key, gotStatus, raw string
	if err := database.DB.QueryRow(`SELECT id,product_local_id,status,item_snapshot::text FROM stock_deduction_failures WHERE outlet_id=$1 AND transaction_id=$2 AND failure_kind='identity'`, outlet, transactionID).Scan(&id, &key, &gotStatus, &raw); err != nil {
		t.Fatal(err)
	}
	if key != "__legacy_identity__" || gotStatus != status {
		t.Fatalf("identity task key=%q status=%q; want reserved key and %q", key, gotStatus, status)
	}
	var snapshot []models.TransactionItem
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, original) {
		t.Fatalf("original unresolved snapshot changed: %+v; want %+v", snapshot, original)
	}
	return id
}

func TestPOSLegacyTransactionDuplicateNamesPreservesMoneyAndSnapshot(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	pay := legacyTransactionRequest(f)
	var stableID string
	for attempt := 0; attempt < 2; attempt++ {
		id, err := SaveTransaction(f.outlet, pay)
		if err != nil {
			t.Fatalf("ambiguous legacy item blocked monetary sync: %v", err)
		}
		if attempt > 0 && id != stableID {
			t.Fatalf("retry changed transaction identity: %s -> %s", stableID, id)
		}
		stableID = id
		_, items := readLegacyTransactionItems(t, f.outlet, pay.LocalID)
		if len(items) != 1 || items[0] != pay.Items[0] {
			t.Fatalf("unresolved item snapshot changed: %+v; want %+v", items, pay.Items)
		}
		assertLegacyTransactionStock(t, f, 100)
		assertLegacyIdentityTask(t, f.outlet, id, "pending", pay.Items)
	}
	var receipts int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=$2`, f.outlet, stableID).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("ambiguous item guessed a product receipt: %d %v", receipts, err)
	}
}

func TestPOSLegacyTransactionBatchAcknowledgesAmbiguousOldPayload(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	legacy := legacyTransactionRequest(f)
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	// Exercise the older HTTP JSON contract rather than a pre-enriched Go
	// request: top-level id is normalized, and line IDs/product IDs are absent.
	payload["id"] = payload["local_id"]
	delete(payload, "local_id")
	for _, raw := range payload["items"].([]interface{}) {
		item := raw.(map[string]interface{})
		delete(item, "id")
		delete(item, "product_id")
	}
	request := models.BatchSyncRequest{OutletID: f.outlet, OutletCode: "LEGACY",
		Items: []models.BatchSyncItem{{EntityType: "transaction", Operation: "create", Data: payload}}}
	var stableID string
	for attempt := 0; attempt < 2; attempt++ {
		response := ProcessBatchSync(f.outlet, request)
		if response.Processed != 1 || response.Success != 1 || response.Failed != 0 || len(response.Results) != 1 {
			t.Fatalf("old payment was not acknowledged to the outbox: %+v", response)
		}
		result := response.Results[0]
		if result.Status != "success" || result.LocalID != legacy.LocalID || result.CloudID == "" || result.Error != "" {
			t.Fatalf("invalid per-item payment acknowledgement: %+v", result)
		}
		if stableID != "" && result.CloudID != stableID {
			t.Fatalf("batch retry changed identity: %s -> %s", stableID, result.CloudID)
		}
		stableID = result.CloudID
		readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
		assertLegacyIdentityTask(t, f.outlet, stableID, "pending", legacy.Items)
		assertLegacyTransactionStock(t, f, 100)
	}
}

func TestPOSLegacyTransactionUniqueNameWithBrokenRecipeKeepsPayment(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	if _, err := database.DB.Exec(`DELETE FROM recipe_items WHERE recipe_master_id=$1`, f.product.RecipeMasterID); err != nil {
		t.Fatal(err)
	}
	pay := legacyTransactionRequest(f)
	id, err := SaveTransaction(f.outlet, pay)
	if err != nil {
		t.Fatalf("recipe configuration blocked monetary sync: %v", err)
	}
	_, items := readLegacyTransactionItems(t, f.outlet, pay.LocalID)
	if len(items) != 1 || items[0].ProductID != f.product.LocalID {
		t.Fatalf("unique catalog identity depends on recipe validity: %+v", items)
	}
	var failures int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_deduction_failures WHERE outlet_id=$1 AND transaction_id=$2 AND product_local_id=$3 AND status='pending'`, f.outlet, id, f.product.LocalID).Scan(&failures); err != nil || failures != 1 {
		t.Fatalf("known product stock failure not retained: %d %v", failures, err)
	}
	assertLegacyTransactionStock(t, f, 100)
}

func TestPOSLegacyTransactionRecoversExactOrderItemIdentity(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	itemID := NewULID()
	f.order.Items[0].ID = itemID
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	pay := legacyTransactionRequest(f)
	pay.Items[0].ID = itemID
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := SaveTransaction(f.outlet, pay); err != nil {
			t.Fatal(err)
		}
		_, items := readLegacyTransactionItems(t, f.outlet, pay.LocalID)
		if len(items) != 1 || items[0].ProductID != f.product.LocalID {
			t.Fatalf("exact order item identity not recovered: %+v", items)
		}
		assertLegacyTransactionStock(t, f, 82)
	}
}

func TestPOSLegacyTransactionDoesNotRecoverFromOtherOutlet(t *testing.T) {
	a, b := newLegacyTransactionFixture(t), newLegacyTransactionFixture(t)
	itemID := NewULID()
	a.order.Items[0].ID = itemID
	if _, err := SaveOrder(a.outlet, a.order); err != nil {
		t.Fatal(err)
	}
	pay := legacyTransactionRequest(b)
	pay.OrderID = a.order.LocalID
	pay.Items[0].ID = itemID
	pay.Items[0].ProductName = a.product.Name
	if _, err := SaveTransaction(b.outlet, pay); err != nil {
		t.Fatalf("unmatched legacy identity blocked payment: %v", err)
	}
	_, items := readLegacyTransactionItems(t, b.outlet, pay.LocalID)
	if len(items) != 1 || items[0].ProductID != "" {
		t.Fatalf("identity recovered across outlets: %+v", items)
	}
	id, _ := readLegacyTransactionItems(t, b.outlet, pay.LocalID)
	assertLegacyIdentityTask(t, b.outlet, id, "pending", pay.Items)
	assertLegacyTransactionStock(t, a, 100)
	assertLegacyTransactionStock(t, b, 100)
}

func TestPOSLegacyTransactionUpgradeAggregatesProductOnce(t *testing.T) {
	for _, knownFirst := range []bool{false, true} {
		name := "two unresolved lines"
		if knownFirst {
			name = "known and unresolved line"
		}
		t.Run(name, func(t *testing.T) {
			f := newLegacyTransactionFixture(t)
			duplicateLegacyProductName(t, f)
			legacy := legacyTransactionRequest(f)
			legacy.Items = []models.TransactionItem{
				{ID: NewULID(), ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
				{ID: NewULID(), ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
			}
			if knownFirst {
				legacy.Items[0].ProductID = f.product.LocalID
			}
			var stableID string
			for attempt := 0; attempt < 2; attempt++ {
				id, err := SaveTransaction(f.outlet, legacy)
				if err != nil {
					t.Fatalf("legacy payment with unresolved lines rejected: %v", err)
				}
				if stableID != "" && id != stableID {
					t.Fatalf("legacy retry changed identity: %s -> %s", stableID, id)
				}
				stableID = id
				assertLegacyTransactionStock(t, f, 100)
				assertLegacyIdentityTask(t, f.outlet, id, "pending", legacy.Items)
			}
			modern := legacy
			modern.Items = append([]models.TransactionItem(nil), legacy.Items...)
			for i := range modern.Items {
				modern.Items[i].ProductID = f.product.LocalID
			}
			// A later explicit payload resolves the original lines. A delayed old
			// retry must preserve that recovery and the same per-product receipt.
			for _, retry := range []models.PushTransactionRequest{modern, modern, legacy} {
				id, err := SaveTransaction(f.outlet, retry)
				if err != nil || id != stableID {
					t.Fatalf("upgrade/retry changed transaction: %s %v; want %s", id, err, stableID)
				}
				_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
				if len(items) != 2 || items[0].ProductID != f.product.LocalID || items[1].ProductID != f.product.LocalID {
					t.Fatalf("recovered identities lost on replay: %+v", items)
				}
				assertLegacyTransactionStock(t, f, 64)
				assertLegacyIdentityTask(t, f.outlet, id, "resolved", legacy.Items)
			}
			var receipts int
			var qty float64
			if err := database.DB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(qty),0) FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=$2`, f.outlet, stableID).Scan(&receipts, &qty); err != nil || receipts != 1 || qty != 2 {
				t.Fatalf("recovery did not aggregate final product quantity: receipts=%d qty=%v %v", receipts, qty, err)
			}
		})
	}
}

func TestPOSLegacyTransactionOldReplayRetainsModernLineIdentity(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	legacy := legacyTransactionRequest(f)
	legacy.Items[0].ID = ""
	id, err := SaveTransaction(f.outlet, legacy)
	if err != nil {
		t.Fatal(err)
	}
	modern := legacy
	modern.Items = append([]models.TransactionItem(nil), legacy.Items...)
	modern.Items[0].ID = NewULID()
	modern.Items[0].ProductID = f.product.LocalID
	if _, err := SaveTransaction(f.outlet, modern); err != nil {
		t.Fatal(err)
	}
	assertLegacyIdentityTask(t, f.outlet, id, "resolved", legacy.Items)
	if _, err := SaveTransaction(f.outlet, legacy); err != nil {
		t.Fatalf("delayed old replay blocked payment: %v", err)
	}
	_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
	if len(items) != 1 || items[0].ProductID != f.product.LocalID {
		t.Fatalf("old replay erased the modern product identity: %+v", items)
	}
	assertLegacyIdentityTask(t, f.outlet, id, "resolved", legacy.Items)
	assertLegacyTransactionStock(t, f, 82)
}

func TestPOSLegacyTransactionOldReplayPreservesDistinctIdenticalProducts(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	other := duplicateLegacyProductName(t, f)
	legacy := legacyTransactionRequest(f)
	legacy.Items = []models.TransactionItem{
		{ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
		{ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
	}
	modern := legacy
	modern.Items = append([]models.TransactionItem(nil), legacy.Items...)
	modern.Items[0].ID, modern.Items[0].ProductID = NewULID(), f.product.LocalID
	modern.Items[1].ID, modern.Items[1].ProductID = NewULID(), other.LocalID
	id, err := SaveTransaction(f.outlet, modern)
	if err != nil {
		t.Fatal(err)
	}
	// Both commercial lines are identical, but their accepted products differ.
	// The old payload can confirm the complete sale without assigning IDs to
	// either individual line or erasing the richer modern snapshot.
	for attempt := 0; attempt < 2; attempt++ {
		gotID, err := SaveTransaction(f.outlet, legacy)
		if err != nil || gotID != id {
			t.Fatalf("old replay changed transaction: %s %v; want %s", gotID, err, id)
		}
		_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
		if !reflect.DeepEqual(items, modern.Items) {
			t.Fatalf("old replay erased distinct accepted product identities: %+v", items)
		}
		assertLegacyTransactionStock(t, f, 82)
	}
	var pending, receipts int
	var qty float64
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_deduction_failures WHERE outlet_id=$1 AND transaction_id=$2 AND failure_kind='identity' AND status='pending'`, f.outlet, id).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(qty),0) FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=$2`, f.outlet, id).Scan(&receipts, &qty); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || receipts != 2 || qty != 2 {
		t.Fatalf("old replay reopened work or changed receipts: pending=%d receipts=%d qty=%v", pending, receipts, qty)
	}
}

func TestPOSLegacyTransactionIdenticalLinesDoNotBorrowKnownIdentity(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	legacy := legacyTransactionRequest(f)
	legacy.Items = []models.TransactionItem{
		{ProductID: f.product.LocalID, ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
		{ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
	}
	id, err := SaveTransaction(f.outlet, legacy)
	if err != nil {
		t.Fatal(err)
	}
	failureID := assertLegacyIdentityTask(t, f.outlet, id, "pending", legacy.Items)
	if err := retryPOSStockDeduction(failureID, f.outlet, id, "__legacy_identity__", 1); err == nil {
		t.Fatal("worker guessed unknown line identity from an identical known line")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := SaveTransaction(f.outlet, legacy); err != nil {
			t.Fatal(err)
		}
		_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
		if len(items) != 2 || items[0].ProductID != f.product.LocalID || items[1].ProductID != "" {
			t.Fatalf("unknown commercial duplicate borrowed the known line's identity: %+v", items)
		}
		assertLegacyIdentityTask(t, f.outlet, id, "pending", legacy.Items)
		assertLegacyTransactionStock(t, f, 100)
	}
}

func TestPOSLegacyTransactionEmptyReplayPreservesIdentityWorkAndDetails(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	legacy := legacyTransactionRequest(f)
	id, err := SaveTransaction(f.outlet, legacy)
	if err != nil {
		t.Fatal(err)
	}
	empty := legacy
	empty.Items = nil
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := SaveTransaction(f.outlet, empty); err != nil {
			t.Fatal(err)
		}
		_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
		if !reflect.DeepEqual(items, legacy.Items) {
			t.Fatalf("empty legacy retry erased original sale details: %+v", items)
		}
		assertLegacyIdentityTask(t, f.outlet, id, "pending", legacy.Items)
		assertLegacyTransactionStock(t, f, 100)
	}
	modern := legacy
	modern.Items = append([]models.TransactionItem(nil), legacy.Items...)
	modern.Items[0].ProductID = f.product.LocalID
	if _, err := SaveTransaction(f.outlet, modern); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, empty); err != nil {
		t.Fatal(err)
	}
	_, items := readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
	if !reflect.DeepEqual(items, modern.Items) {
		t.Fatalf("empty retry erased recovered details: %+v", items)
	}
	assertLegacyIdentityTask(t, f.outlet, id, "resolved", legacy.Items)
	assertLegacyTransactionStock(t, f, 82)
}

func TestPOSLegacyTransactionIdentityWorkerAggregatesSnapshot(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicate := duplicateLegacyProductName(t, f)
	pay := legacyTransactionRequest(f)
	// Actual older APKs omit both product and line IDs. Preserve both lines,
	// even when their commercial snapshots are identical.
	pay.Items = []models.TransactionItem{
		{ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
		{ProductName: f.product.Name, Quantity: 1, Price: 5000, Subtotal: 5000},
	}
	id, err := SaveTransaction(f.outlet, pay)
	if err != nil {
		t.Fatal(err)
	}
	failure := assertLegacyIdentityTask(t, f.outlet, id, "pending", pay.Items)
	if _, err := database.DB.Exec(`UPDATE cloud_products SET name='Different legacy product' WHERE id=$1`, duplicate.ID); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := retryPOSStockDeduction(failure, f.outlet, id, "__legacy_identity__", 1); err != nil {
			t.Fatalf("identity worker could not resolve the complete snapshot: %v", err)
		}
		_, items := readLegacyTransactionItems(t, f.outlet, pay.LocalID)
		if len(items) != 2 || items[0].ProductID != f.product.LocalID || items[1].ProductID != f.product.LocalID {
			t.Fatalf("identity worker lost repeated legacy items: %+v", items)
		}
		assertLegacyIdentityTask(t, f.outlet, id, "resolved", pay.Items)
		assertLegacyTransactionStock(t, f, 64)
	}
}

func TestPOSLegacyTransactionIdentityGatesStaleStockRetry(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	pay := legacyTransactionRequest(f)
	id, err := SaveTransaction(f.outlet, pay)
	if err != nil {
		t.Fatal(err)
	}
	assertLegacyIdentityTask(t, f.outlet, id, "pending", pay.Items)
	// Simulate a stock task selected before a newer legacy snapshot exposed
	// unresolved identities. The locked recheck must wait rather than deduct.
	failureID := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO stock_deduction_failures
		(id,outlet_id,transaction_id,product_local_id,qty,last_error,failure_kind)
		VALUES($1,$2,$3,$4,1,'Historical stock retry','stock')`, failureID, f.outlet, id, f.product.LocalID); err != nil {
		t.Fatal(err)
	}
	if err := retryPOSStockDeduction(failureID, f.outlet, id, f.product.LocalID, 1); err != nil {
		t.Fatalf("waiting for identity should not report a stock error: %v", err)
	}
	assertLegacyTransactionStock(t, f, 100)
	var status string
	var attempts, receipts int
	if err := database.DB.QueryRow(`SELECT status,attempts FROM stock_deduction_failures WHERE id=$1`, failureID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=$2`, f.outlet, id).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || attempts != 0 || receipts != 0 {
		t.Fatalf("gated task changed state: status=%s attempts=%d receipts=%d", status, attempts, receipts)
	}
}

func TestPOSLegacyTransactionVoidWaitsForIdentityWithoutLosingMoney(t *testing.T) {
	f := newLegacyTransactionFixture(t)
	duplicateLegacyProductName(t, f)
	f.order.Items[0].ProductID = ""
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	legacy := legacyTransactionRequest(f)
	id, err := SaveTransaction(f.outlet, legacy)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := SaveOrder(f.outlet, f.void()); err == nil {
			t.Fatal("void silently discarded unresolved stock identity")
		}
		readLegacyTransactionItems(t, f.outlet, legacy.LocalID)
		assertLegacyIdentityTask(t, f.outlet, id, "pending", legacy.Items)
		assertLegacyTransactionStock(t, f, 100)
		var voids, waste int
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM pos_void_stock_events WHERE outlet_id=$1`, f.outlet).Scan(&voids); err != nil {
			t.Fatal(err)
		}
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_wastes WHERE warehouse_id=$1`, f.wh).Scan(&waste); err != nil {
			t.Fatal(err)
		}
		if voids != 0 || waste != 0 {
			t.Fatalf("failed void partially changed stock audit: events=%d waste=%d", voids, waste)
		}
	}
	modern := legacy
	modern.Items = append([]models.TransactionItem(nil), legacy.Items...)
	modern.Items[0].ProductID = f.product.LocalID
	if _, err := SaveTransaction(f.outlet, modern); err != nil {
		t.Fatal(err)
	}
	assertLegacyIdentityTask(t, f.outlet, id, "resolved", legacy.Items)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := SaveOrder(f.outlet, f.void()); err != nil {
			t.Fatalf("resolved legacy transaction cannot be voided: %v", err)
		}
	}
	f.assertWaste(t, 82, 18, 36, 1)
	var transactions, payments int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, f.outlet, id).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM transaction_payments WHERE outlet_id=$1 AND transaction_id=$2`, f.outlet, id).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if transactions != 0 || payments != 0 {
		t.Fatalf("resolved void retained payment: transactions=%d payments=%d", transactions, payments)
	}
}
