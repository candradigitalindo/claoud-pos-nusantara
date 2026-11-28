package services

import (
	"math"
	"sync"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

var bundleTestMigOnce sync.Once
var bundleTestMigErr error

func ensureBundleTestMigrations(t *testing.T) {
	t.Helper()
	bundleTestMigOnce.Do(func() {
		bundleTestMigErr = database.RunMigrations()
	})
	if bundleTestMigErr != nil {
		t.Fatalf("RunMigrations: %v", bundleTestMigErr)
	}
}

func TestCrossOutletBundleEndToEndStockAndAccounting(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	atvOutlet := NewULID()
	gdOutlet := NewULID()
	gdWh := NewULID()
	atvProd := NewULID()
	gdDonatProd := NewULID()
	donatStockItem := NewULID()

	for _, o := range []struct{ id, code, name, slug string }{
		{atvOutlet, "ATV" + atvOutlet[21:], "Uji ATV Overland", "uji-atv-" + atvOutlet[20:]},
		{gdOutlet, "GD" + gdOutlet[22:], "Uji Gadis Desa", "uji-gd-" + gdOutlet[20:]},
	} {
		if _, err := database.DB.Exec(
			`INSERT INTO outlets (id, code, name, slug, api_key, is_active) VALUES ($1,$2,$3,$4,$5,true)`,
			o.id, o.code, o.name, o.slug, "key-"+o.id,
		); err != nil {
			t.Fatalf("insert outlet: %v", err)
		}
	}
	if _, err := database.DB.Exec(
		`INSERT INTO warehouses (id, code, name, type, outlet_id, is_active) VALUES ($1,$2,'Gudang Uji Gadis Desa','outlet',$3,true)`,
		gdWh, "WHGD"+gdWh[22:], gdOutlet,
	); err != nil {
		t.Fatalf("insert warehouse: %v", err)
	}
	if _, err := database.DB.Exec(
		`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio, is_active) VALUES ($1,$2,'Donat Kentang Uji','pcs','pack',10,true)`,
		donatStockItem, "DN"+donatStockItem[20:],
	); err != nil {
		t.Fatalf("insert stock_item: %v", err)
	}
	if err := CreateAdjustment(models.AdjustmentRequest{
		ItemID: donatStockItem, WarehouseID: gdWh, QtyBase: 20, CostPerBase: 8000, MovType: "purchase_in",
	}, "uji"); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
	if _, err := database.DB.Exec(
		`INSERT INTO cloud_products (id, outlet_id, local_id, name, price, category_name, stock_type, is_deleted) VALUES
		 ($1,$2,$3,'GOLD ATV',300000,'ATV','none',false),
		 ($4,$5,$6,'DONAT KAMPUNG GD',45000,'SNACK','recipe',false)`,
		atvProd, atvOutlet, "loc-"+atvProd, gdDonatProd, gdOutlet, "loc-"+gdDonatProd,
	); err != nil {
		t.Fatalf("insert products: %v", err)
	}
	if _, err := database.DB.Exec(
		`INSERT INTO product_recipes (id, product_id, item_id, qty_dist, qty_base, unit_used) VALUES ($1,$2,$3,1,1,'pcs')`,
		NewULID(), gdDonatProd, donatStockItem,
	); err != nil {
		t.Fatalf("insert product_recipe: %v", err)
	}

	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM cross_outlet_bundle_claims WHERE source_outlet_id = $1 OR partner_outlet_id = $2`, atvOutlet, gdOutlet)
		database.DB.Exec(`DELETE FROM product_bundle_components WHERE source_outlet_id = $1`, atvOutlet)
		database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id = $1`, gdWh)
		database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id = $1`, gdWh)
		database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id = $1`, gdWh)
		database.DB.Exec(`DELETE FROM product_recipes WHERE product_id = $1`, gdDonatProd)
		database.DB.Exec(`DELETE FROM transaction_payments WHERE outlet_id IN ($1,$2)`, atvOutlet, gdOutlet)
		database.DB.Exec(`DELETE FROM cloud_transactions WHERE outlet_id IN ($1,$2)`, atvOutlet, gdOutlet)
		database.DB.Exec(`DELETE FROM finance_pos_revisions WHERE outlet_id IN ($1,$2)`, atvOutlet, gdOutlet)
		database.DB.Exec(`DELETE FROM pos_stock_deductions WHERE outlet_id IN ($1,$2)`, atvOutlet, gdOutlet)
		database.DB.Exec(`DELETE FROM cloud_products WHERE id IN ($1,$2)`, atvProd, gdDonatProd)
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, donatStockItem)
		database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, gdWh)
		database.DB.Exec(`DELETE FROM outlets WHERE id IN ($1,$2)`, atvOutlet, gdOutlet)
	})

	comps, err := SaveProductBundleComponents(atvProd, []models.SaveBundleComponentItem{{
		PartnerOutletID:  gdOutlet,
		PartnerProductID: gdDonatProd,
		QtyPerBundle:     2,
		AllocationAmount: 63636,
		ValidDays:        1,
		Notes:            "2 donat ambil di Gadis Desa",
	}})
	if err != nil {
		t.Fatalf("SaveProductBundleComponents: %v", err)
	}
	if len(comps) != 1 || !comps[0].PartnerHasRecipe {
		t.Fatalf("expected 1 component with PartnerHasRecipe=true, got %+v", comps)
	}

	if _, err := SaveProductBundleComponents(atvProd, []models.SaveBundleComponentItem{{
		PartnerOutletID:  gdOutlet,
		PartnerProductID: gdDonatProd,
		QtyPerBundle:     2,
		AllocationAmount: 350000,
		ValidDays:        1,
	}}); err == nil {
		t.Fatal("expected error when allocation exceeds source price")
	}

	now := time.Now().In(GetTimezoneLocation())
	today := now.Format("2006-01-02")
	txLocal := "TX-ATV-" + NewULID()[18:]
	txReq := models.PushTransactionRequest{
		LocalID:       txLocal,
		OutletCode:    "ATV",
		Subtotal:      272727,
		TaxAmount:     27273,
		TotalAmount:   300000,
		PaymentMethod: "qris",
		CashierName:   "Windi",
		OrdererName:   "Tamu Budi",
		CreatedAt:     now.UTC().Format(time.RFC3339),
		Items: []models.TransactionItem{{
			ProductID:   atvProd,
			ProductName: "GOLD ATV",
			Quantity:    1,
			Price:       300000,
			Subtotal:    300000,
		}},
	}
	if _, err := SaveTransaction(atvOutlet, txReq); err != nil {
		t.Fatalf("SaveTransaction: %v", err)
	}
	if _, err := SaveTransaction(atvOutlet, txReq); err != nil {
		t.Fatalf("SaveTransaction retry: %v", err)
	}

	gdList, err := ListCrossOutletBundleClaims(models.CrossOutletBundleFilter{Direction: "inbound"}, []string{gdOutlet})
	if err != nil {
		t.Fatalf("ListCrossOutletBundleClaims: %v", err)
	}
	if len(gdList.Claims) != 1 {
		t.Fatalf("expected 1 claim for Gadis Desa, got %d", len(gdList.Claims))
	}
	claim := gdList.Claims[0]
	if claim.Status != "pending" || claim.PartnerQty != 2 || claim.AllocationAmount != 63636 || !claim.CanClaim {
		t.Fatalf("unexpected pending claim: %+v", claim)
	}

	if _, err := ClaimCrossOutletBundle(claim.ID, models.ClaimCrossOutletBundleRequest{}, "Windi", []string{atvOutlet}); err == nil {
		t.Fatal("expected error when ATV manager attempts to claim Gadis Desa ticket")
	}

	claimed, err := ClaimCrossOutletBundle(claim.ID, models.ClaimCrossOutletBundleRequest{Notes: "Diserahkan ke Tamu Budi"}, "Sania", []string{gdOutlet})
	if err != nil {
		t.Fatalf("ClaimCrossOutletBundle: %v", err)
	}
	if claimed.Status != "claimed" || !claimed.StockDeducted || claimed.ClaimedQty != 2 {
		t.Fatalf("unexpected claimed result: %+v", claimed)
	}
	if _, err := ClaimCrossOutletBundle(claim.ID, models.ClaimCrossOutletBundleRequest{}, "Sania", []string{gdOutlet}); err == nil {
		t.Fatal("expected error on duplicate claim")
	}

	var qtyAfter float64
	if err := database.DB.QueryRow(`SELECT qty_base FROM stock_ledger WHERE warehouse_id = $1 AND item_id = $2`, gdWh, donatStockItem).Scan(&qtyAfter); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if math.Abs(qtyAfter-18) > 0.0001 {
		t.Fatalf("Gadis Desa stock after claiming 2 donuts: got %v, want 18", qtyAfter)
	}

	pnlATV, err := financeProfitLoss(today, today, atvOutlet, []string{atvOutlet, gdOutlet})
	if err != nil {
		t.Fatalf("PnL ATV: %v", err)
	}
	if math.Abs(pnlATV.Summary.SalesRevenue-(300000-63636)) > 0.01 {
		t.Fatalf("ATV net sales revenue: got %.2f, want %.2f", pnlATV.Summary.SalesRevenue, 300000.0-63636.0)
	}

	pnlGD, err := financeProfitLoss(today, today, gdOutlet, []string{atvOutlet, gdOutlet})
	if err != nil {
		t.Fatalf("PnL GD: %v", err)
	}
	if math.Abs(pnlGD.Summary.SalesRevenue-63636) > 0.01 {
		t.Fatalf("Gadis Desa bundle allocation revenue: got %.2f, want 63636.00", pnlGD.Summary.SalesRevenue)
	}
	if math.Abs(pnlGD.Summary.COGS-16000) > 0.01 {
		t.Fatalf("Gadis Desa COGS (2 x 8000): got %.2f, want 16000.00", pnlGD.Summary.COGS)
	}

	pnlAll, err := financeProfitLoss(today, today, "", []string{atvOutlet, gdOutlet})
	if err != nil {
		t.Fatalf("PnL Consolidated: %v", err)
	}
	if math.Abs(pnlAll.Summary.SalesRevenue-300000) > 0.01 {
		t.Fatalf("Consolidated sales revenue must stay 300000 (no double counting!), got %.2f", pnlAll.Summary.SalesRevenue)
	}
	if math.Abs(pnlAll.Summary.COGS-16000) > 0.01 {
		t.Fatalf("Consolidated COGS: got %.2f, want 16000.00", pnlAll.Summary.COGS)
	}
	if math.Abs(pnlAll.Summary.TaxExpense-27273) > 0.01 {
		t.Fatalf("Consolidated PB1 must stay 27273 (1x, no double tax!), got %.2f", pnlAll.Summary.TaxExpense)
	}
	if math.Abs(pnlGD.Summary.TaxExpense) > 0.01 {
		t.Fatalf("Gadis Desa must have 0 PB1 on bundle claim, got %.2f", pnlGD.Summary.TaxExpense)
	}

	for _, tc := range []struct{ label, outlet string }{
		{"ATV", atvOutlet},
		{"GadisDesa", gdOutlet},
		{"Consolidated", ""},
	} {
		bal, err := financeBalance(today, today, tc.outlet, []string{atvOutlet, gdOutlet})
		if err != nil {
			t.Fatalf("financeBalance %s: %v", tc.label, err)
		}
		if math.Abs(bal.BalanceDifference) > 0.01 {
			t.Fatalf("Balance Sheet %s not balanced: diff=%.2f assets=%.2f liab=%.2f",
				tc.label, bal.BalanceDifference, bal.TotalAssets, bal.TotalLiabilities)
		}
	}
}
