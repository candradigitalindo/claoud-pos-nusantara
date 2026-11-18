package services

import (
	"errors"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func quantityTestItems() []models.PurchaseRequestItem {
	return []models.PurchaseRequestItem{{Name: "Bahan", Items: []models.PurchaseSubItem{{
		Name: "Tepung", Qty: 10, Unit: "kotak", HpsPrice: 12000, HpsSource: "Toko",
	}}}}
}

func TestPurchasedQuantityKeepsApprovalAndRecalculatesFinal(t *testing.T) {
	stored := quantityTestItems()
	incoming := clonePurchaseItems(stored)
	qty := 8
	incoming[0].Items[0].PurchasedQty = &qty
	incoming[0].Items[0].FinalPrice = 11000
	updated, err := applyApprovedPriceEdit(stored, incoming)
	if err != nil {
		t.Fatal(err)
	}
	hps, final := recalcItems(updated)
	line := updated[0].Items[0]
	if line.Qty != 10 || line.PurchaseQuantity() != 8 || hps != 120000 || final != 88000 || line.FinalSubtotal != 88000 {
		t.Fatalf("quantity/totals = %+v / %v / %v", line, hps, final)
	}
	if stored[0].Items[0].PurchasedQty != nil || !purchaseQuantitiesChanged(stored, updated) {
		t.Fatal("approved request changed or actual quantity change was missed")
	}
	if amount, estimate := subItemAmount(line); amount != 88000 || estimate {
		t.Fatalf("RAB commitment = %v, estimate=%v", amount, estimate)
	}
	// Older clients may omit the new field when setting prices: do not reset
	// the eight boxes to the original ten.
	legacy := clonePurchaseItems(updated)
	legacy[0].Items[0].PurchasedQty = nil
	merged, err := applyApprovedPriceEdit(updated, legacy)
	if err != nil || merged[0].Items[0].PurchaseQuantity() != 8 || purchaseQuantitiesChanged(updated, merged) {
		t.Fatalf("legacy update reset purchase quantity: %+v / %v", merged, err)
	}
	copy := clonePurchaseItems(updated)
	*copy[0].Items[0].PurchasedQty = 7
	if updated[0].Items[0].PurchaseQuantity() != 8 {
		t.Fatal("clone shared purchased quantity pointer")
	}
}

func TestPurchasedQuantityRejectsOutsideApprovedQuantity(t *testing.T) {
	for _, qty := range []int{-1, 0, 11} {
		items := quantityTestItems()
		items[0].Items[0].PurchasedQty = &qty
		_, err := applyApprovedPriceEdit(quantityTestItems(), items)
		var invalid ValidationError
		if !errors.As(err, &invalid) {
			t.Errorf("purchased_qty=%d: expected validation error, got %v", qty, err)
		}
	}
	// The original approved quantity remains immutable.
	items := quantityTestItems()
	items[0].Items[0].Qty = 8
	if _, err := applyApprovedPriceEdit(quantityTestItems(), items); err == nil {
		t.Fatal("purchasing changed the approved quantity")
	}
}

func TestRequesterCannotSetPurchasedQuantity(t *testing.T) {
	items := quantityTestItems()
	qty := 8
	items[0].Items[0].PurchasedQty = &qty
	items[0].Items[0].FinalPrice = 10000
	clearFinalPrices(items)
	if line := items[0].Items[0]; line.PurchasedQty != nil || line.FinalPrice != 0 || line.PurchaseQuantity() != 10 {
		t.Fatalf("requester set purchasing fields: %+v", line)
	}
}

func TestPurchasedQuantityReceivingUsesActualWithoutPhantomRemainder(t *testing.T) {
	requireDB(t)
	wh := recvWarehouse(t)
	name := "Tepung Aktual " + recvSuffix()
	stockID := recvStockItem(t, name)
	pr := recvApprovedPR(t, RequestTypeBahan, []models.PurchaseRequestItem{{Name: "Bahan", Items: []models.PurchaseSubItem{{
		Name: name, Qty: 10, Unit: "kg", HpsPrice: 12000,
	}}}})
	qty := 8
	items := clonePurchaseItems(pr.Items)
	items[0].Items[0].PurchasedQty = &qty
	items[0].Items[0].FinalPrice = 11000
	priced, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items, Version: pr.Version, VendorID: strPtr(testVendorID(t))})
	if err != nil {
		t.Fatalf("set actual purchase: %v", err)
	}
	if priced.TotalHps != 120000 || priced.TotalFinal != 88000 || priced.Items[0].Items[0].Qty != 10 {
		t.Fatalf("incorrect requested/actual totals: %+v", priced)
	}
	draft := recvDraft(t, pr.ID)
	if line := draft.Lines[0]; line.Qty != 8 || line.Remaining != 8 || line.Subtotal != 88000 {
		t.Fatalf("draft follows original qty: %+v", line)
	}
	receive := func(qty int) error {
		_, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: recvPhoto, Lines: []models.ReceiveGoodsLine{{
			PRItemKey: draft.Lines[0].PRItemKey, Destination: "stok", Qty: qty, StockItemID: stockID, WarehouseID: wh,
		}}}, recvActor, recvAll, nil)
		return err
	}
	if err := receive(9); err == nil {
		t.Fatal("received more than the eight actually purchased")
	}
	if err := receive(8); err != nil {
		t.Fatalf("receive actual quantity: %v", err)
	}
	draft = recvDraft(t, pr.ID)
	current, err := GetPurchaseRequest(pr.ID)
	if err != nil || draft.Lines[0].Remaining != 0 || current.ReceiptStatus != "received" {
		t.Fatalf("phantom remainder after all eight received: %+v", draft)
	}
	var stock float64
	if err := database.DB.QueryRow(`SELECT COALESCE(SUM(qty_base),0) FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2`, stockID, wh).Scan(&stock); err != nil || stock != 8000 {
		t.Fatalf("stock = %v, err=%v, want 8000 gr", stock, err)
	}
	qty = 7
	items[0].Items[0].PurchasedQty = &qty
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); err == nil {
		t.Fatal("actual quantity changed after receipt")
	}
	// Price corrections still work after receipt without altering quantities.
	qty = 8
	items[0].Items[0].FinalPrice = 11500
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); err != nil {
		t.Fatalf("price edit after receipt blocked: %v", err)
	}
	mustStatus(t, pr.ID, "request_payment", 0)
	mustStatus(t, pr.ID, "pay", 0)
	current, err = GetPurchaseRequest(pr.ID)
	if err != nil || current.Status != "received" || current.PaidAmount != 92000 {
		t.Fatalf("bill should be eight boxes at 11500: %+v, %v", current, err)
	}
}

func TestPurchasedQuantityLockedAfterStruk(t *testing.T) {
	requireDB(t)
	pr := recvApprovedPR(t, RequestTypeBahan, quantityTestItems())
	if _, err := AddPurchaseStruk(pr.ID, models.AddPurchaseStrukInput{
		StrukNumber: "STRUK-AKTUAL", StrukDate: "2026-01-01", Total: 80000, PhotoURL: recvPhoto,
	}, recvActor); err != nil {
		t.Fatal(err)
	}
	items := clonePurchaseItems(pr.Items)
	qty := 8
	items[0].Items[0].PurchasedQty = &qty
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); err == nil {
		t.Fatal("actual quantity changed after struk recorded")
	}
}
