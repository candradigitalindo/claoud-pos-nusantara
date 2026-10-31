package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"testing"
)

func TestPOSMenuKindsSurviveOrderRestoreAndTransactionSync(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	t.Cleanup(func() {
		// The outlet fixture removes orders/transactions afterwards. Products
		// have no foreign key from those immutable item JSON snapshots.
		if _, err := database.DB.Exec(`DELETE FROM cloud_products WHERE outlet_id=$1`, outlet); err != nil {
			t.Error(err)
		}
	})
	order := restoreOrderRequest(NewULID())
	transaction := models.PushTransactionRequest{LocalID: NewULID(), OrderID: order.LocalID,
		TotalAmount: 100, Subtotal: 100, CashAmount: 100, PaymentMethod: "cash",
		TransactionDate: "2026-10-21T06:02:00Z", Version: 1}
	for i, kind := range []string{"food", "beverage", "activity", "other"} {
		product, err := AdminCreateProduct(models.AdminCreateProductRequest{
			OutletID: outlet, Name: "Menu " + kind, Price: 10, MenuKind: &kind})
		if err != nil {
			t.Fatal(err)
		}
		itemID := NewULID()
		order.Items = append(order.Items, models.OrderItem{ID: itemID, ProductID: product.LocalID,
			ProductName: product.Name, MenuKind: kind, Qty: i + 1, Price: 10, Subtotal: float64((i + 1) * 10)})
		transaction.Items = append(transaction.Items, models.TransactionItem{ID: itemID, ProductID: product.LocalID,
			ProductName: product.Name, MenuKind: kind, Quantity: i + 1, Price: 10, Subtotal: float64((i + 1) * 10)})
	}
	// Exercise the same typed JSON boundary used by the HTTP sync handler.
	orderJSON, _ := json.Marshal(order)
	var incomingOrder models.PushOrderRequest
	if err := json.Unmarshal(orderJSON, &incomingOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrder(outlet, incomingOrder); err != nil {
		t.Fatal(err)
	}
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.UnpaidOrders) != 1 {
		t.Fatalf("restore missing order: %+v %v", restored, err)
	}
	wantItems, _ := json.Marshal(order.Items)
	assertJSONValueEqual(t, wantItems, []byte(restored.UnpaidOrders[0].Items))

	transactionJSON, _ := json.Marshal(transaction)
	var incomingTransaction models.PushTransactionRequest
	if err := json.Unmarshal(transactionJSON, &incomingTransaction); err != nil {
		t.Fatal(err)
	}
	id, err := SaveTransaction(outlet, incomingTransaction)
	if err != nil {
		t.Fatal(err)
	}
	// A subsequent catalog reclassification must not change sold-item snapshots.
	if _, err := database.DB.Exec(`UPDATE fnb_product_profiles SET menu_kind='other'
		WHERE product_id IN (SELECT id FROM cloud_products WHERE outlet_id=$1)`, outlet); err != nil {
		t.Fatal(err)
	}
	var items string
	if err := database.DB.QueryRow(`SELECT items::text FROM cloud_transactions WHERE id=$1`, id).Scan(&items); err != nil {
		t.Fatal(err)
	}
	wantItems, _ = json.Marshal(transaction.Items)
	assertJSONValueEqual(t, wantItems, []byte(items))
}
