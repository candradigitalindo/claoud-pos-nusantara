package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"testing"
)

// Exercise the actual Flutter batch -> shared public menu -> online order path,
// including old products whose cloud ID differs from the POS local ID.
func TestDBFlutterMastersFeedOnlineOrdersWithoutChangingReservationIDs(t *testing.T) {
	setupDB(t)
	enableMockGateway(t)
	outletID, slug := seedOutlet(t)
	localProduct := produkID(outletID)
	cloudProduct := NewULID()
	if _, err := database.DB.Exec(`UPDATE cloud_products SET id=$1 WHERE outlet_id=$2 AND local_id=$3`, cloudProduct, outletID, localProduct); err != nil {
		t.Fatal(err)
	}
	addonID, chargeID := NewULID(), NewULID()
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM cloud_product_addons WHERE outlet_id=$1`, outletID) })
	request := models.BatchSyncRequest{Items: []models.BatchSyncItem{
		{EntityType: "product_addon", Operation: "upsert", Data: map[string]interface{}{
			"local_id": addonID, "product_local_id": localProduct, "name": "Keju",
			"price": 5000, "is_active": 1,
		}},
		{EntityType: "additional_charge", Operation: "upsert", Data: map[string]interface{}{
			"local_id": chargeID, "name": "Kemasan", "charge_type": "fixed", "value": 2000, "is_active": 1,
		}},
	}}
	for i := 0; i < 2; i++ {
		result := ProcessBatchSync(outletID, request)
		if result.Success != 2 || result.Failed != 0 {
			t.Fatalf("master sync retry %d: %+v", i, result)
		}
	}
	menu, err := GetPublicMenu(slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(menu.Categories) != 1 || len(menu.Categories[0].Products) != 1 {
		t.Fatalf("menu: %+v", menu)
	}
	product := menu.Categories[0].Products[0]
	if product.ID != cloudProduct || product.ProductLocalID != localProduct || len(product.Addons) != 1 {
		t.Fatalf("reservation ID and POS ID/addon must remain distinct: %+v", product)
	}
	order, err := CreatePublicOrder(slug, models.PublicOrderRequest{TableNumber: "A1", Items: []models.OnlineOrderItem{
		{ProductLocalID: product.ProductLocalID, Qty: 2, Addons: []models.OnlineOrderAddon{{ID: addonID}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if order.TotalAmount != 68000 || order.Payment == nil || order.Payment.Amount != 68000 {
		t.Fatalf("2*(25000+5000) + 10%% + 2000 must match QRIS: %+v", order)
	}
	removed := ProcessBatchSync(outletID, models.BatchSyncRequest{Items: []models.BatchSyncItem{
		{EntityType: "product_addon", Operation: "delete", Data: map[string]interface{}{"local_id": addonID}},
	}})
	if removed.Success != 1 {
		t.Fatalf("delete addon: %+v", removed)
	}
	menu, err = GetPublicMenu(slug)
	if err != nil || len(menu.Categories[0].Products[0].Addons) != 0 {
		t.Fatalf("deleted addon still offered: %+v %v", menu, err)
	}
}

func TestDBUnsupportedBatchEntityRemainsRetryableAndEchoesEventIdentity(t *testing.T) {
	setupDB(t)
	outletID, _ := seedOutlet(t)
	result := ProcessBatchSync(outletID, models.BatchSyncRequest{Items: []models.BatchSyncItem{
		{EntityType: "future_audit_event", Operation: "create", Data: map[string]interface{}{"local_id": "item-1", "event_id": "event-1"}},
	}})
	if result.Success != 0 || result.Failed != 1 || len(result.Results) != 1 {
		t.Fatalf("unknown entity acknowledged: %+v", result)
	}
	r := result.Results[0]
	if r.Status != "failed" || r.EventID != "event-1" || r.LocalID != "item-1" || r.EntityType != "future_audit_event" || r.CloudID != "" {
		t.Fatalf("ambiguous/lost event acknowledgement: %+v", r)
	}
}

func TestDBQRISSimulationAndCallbacksStayWithinOutletAndProvider(t *testing.T) {
	setupDB(t)
	enableMockGateway(t)
	outletID, _ := seedOutlet(t)
	otherOutlet, _ := seedOutlet(t)
	charge, err := CreateQRISCharge(outletID, "order-scope", 10000, "Uji")
	if err != nil {
		t.Fatal(err)
	}
	if err = SimulateQRISChargePaid(otherOutlet, charge.ChargeID); err == nil {
		t.Fatal("another outlet settled charge")
	}
	if err = MarkQRISChargePaidForProvider("another-provider", charge.ChargeID, QRISPaid); err == nil {
		t.Fatal("another provider settled charge")
	}
	state, err := GetQRISCharge(outletID, charge.ChargeID)
	if err != nil || state.Status != QRISPending {
		t.Fatalf("foreign request changed charge: %+v %v", state, err)
	}
	if _, err = database.DB.Exec(`UPDATE qris_charges SET provider='real-provider' WHERE id=$1`, charge.ChargeID); err != nil {
		t.Fatal(err)
	}
	if err = SimulateQRISChargePaid(outletID, charge.ChargeID); err == nil {
		t.Fatal("mock simulation settled a real-provider charge")
	}
	if _, err = database.DB.Exec(`UPDATE qris_charges SET provider='mock' WHERE id=$1`, charge.ChargeID); err != nil {
		t.Fatal(err)
	}
	if err = SimulateQRISChargePaid(outletID, charge.ChargeID); err != nil {
		t.Fatal(err)
	}
	state, err = GetQRISCharge(outletID, charge.ChargeID)
	if err != nil || state.Status != QRISPaid {
		t.Fatalf("explicit simulation failed: %+v %v", state, err)
	}
}

func TestDBQRISPaymentAndOnlinePromotionRollbackTogether(t *testing.T) {
	setupDB(t)
	outletID, slug := seedOutlet(t)
	order := pesanSatuNasiGoreng(t, outletID, slug)
	constraint := "qris_promotion_" + NewULID()
	if _, err := database.DB.Exec(`ALTER TABLE online_orders ADD CONSTRAINT ` + constraint + ` CHECK (id <> '` + order.ID + `' OR status <> 'new') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Exec(`ALTER TABLE online_orders DROP CONSTRAINT IF EXISTS ` + constraint) })
	if err := MarkQRISChargePaidForProvider("mock", order.Payment.ChargeID, QRISPaid); err == nil {
		t.Fatal("failed promotion was acknowledged")
	}
	charge, err := GetQRISCharge(outletID, order.Payment.ChargeID)
	if err != nil || charge.Status != QRISPending {
		t.Fatalf("charge escaped rollback: %+v %v", charge, err)
	}
	status, paid := statusPesanan(t, order.ID)
	if status != models.OnlineOrderAwaitingPayment || paid != 0 {
		t.Fatalf("order escaped rollback: %s %v", status, paid)
	}
	if _, err = database.DB.Exec(`ALTER TABLE online_orders DROP CONSTRAINT ` + constraint); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = MarkQRISChargePaidForProvider("mock", order.Payment.ChargeID, QRISPaid); err != nil {
			t.Fatal(err)
		}
	}
	status, paid = statusPesanan(t, order.ID)
	if status != models.OnlineOrderNew || paid != order.TotalAmount {
		t.Fatalf("retry failed: %s %v", status, paid)
	}
}
