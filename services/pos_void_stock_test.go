package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"strings"
	"sync"
	"testing"
)

type posVoidFixture struct {
	*productRecipeFixture
	product models.CloudProduct
	order   models.PushOrderRequest
	payment models.PushTransactionRequest
}

func newPOSVoidFixture(t *testing.T) *posVoidFixture {
	t.Helper()
	f := &posVoidFixture{productRecipeFixture: newProductRecipeFixture(t)}
	r := f.recipe(t, "public", false, 18, false)
	f.product = f.productRecipeFixture.product(t, r)
	f.stockIn(t, f.itemID, 100, 2, "")
	f.order = models.PushOrderRequest{LocalID: NewULID(), OutletCode: "TEST", Pax: 1, Status: "cooking", TotalAmount: 10000,
		CreatedAt: "2026-10-21T06:00:00Z", UpdatedAt: "2026-10-21T06:00:00Z",
		Items:       []models.OrderItem{{ProductID: f.product.LocalID, ProductName: f.product.Name, Qty: 1, Price: 10000, Subtotal: 10000}},
		PaymentInfo: models.PaymentInfo{PaymentStatus: "unpaid"}}
	f.payment = models.PushTransactionRequest{LocalID: NewULID(), OutletCode: "TEST", OrderID: f.order.LocalID, TotalAmount: 10000,
		PaymentMethod: "cash", CreatedAt: "2026-10-21T06:05:00Z", CashierName: "Kasir",
		Items: []models.TransactionItem{{ProductID: f.product.LocalID, ProductName: f.product.Name, Quantity: 1, Price: 10000, Subtotal: 10000}}}
	t.Cleanup(func() {
		execScratch(`DELETE FROM stock_wastes WHERE warehouse_id=$1`, f.wh)
		execScratch(`DELETE FROM pos_void_stock_events WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM pos_stock_deductions WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM stock_deduction_failures WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM order_item_voids WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM transaction_payments WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM cloud_transactions WHERE outlet_id=$1`, f.outlet)
		execScratch(`DELETE FROM cloud_orders WHERE outlet_id=$1`, f.outlet)
	})
	return f
}

func (f *posVoidFixture) void() models.PushOrderRequest {
	r := f.order
	r.PaymentInfo = models.PaymentInfo{PaymentStatus: "unpaid", VoidedAt: "2026-10-21T06:10:00Z", VoidedBy: "Supervisor", VoidReason: "Pelanggan membatalkan"}
	r.UpdatedAt = r.PaymentInfo.VoidedAt
	return r
}

func (f *posVoidFixture) assertWaste(t *testing.T, stock, qty, cost float64, n int) {
	t.Helper()
	got, _ := f.ledger(t, f.itemID)
	if !bdNear(got, stock) {
		t.Fatalf("stok %v, ingin %v", got, stock)
	}
	var count int
	var totalQty, totalCost float64
	if err := database.DB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(qty_base),0),COALESCE(SUM(total_cost),0) FROM stock_wastes WHERE warehouse_id=$1 AND reason='void'`, f.wh).Scan(&count, &totalQty, &totalCost); err != nil {
		t.Fatal(err)
	}
	if count != n || !bdNear(totalQty, qty) || !bdNear(totalCost, cost) {
		t.Fatalf("waste count=%d qty=%v cost=%v, ingin %d %v %v", count, totalQty, totalCost, n, qty, cost)
	}
	var saleQty, wasteQty float64
	if err := database.DB.QueryRow(`SELECT COALESCE(SUM(-qty_base) FILTER(WHERE movement_type='sale'),0),COALESCE(SUM(-qty_base) FILTER(WHERE movement_type='waste'),0) FROM stock_movements WHERE warehouse_id=$1 AND item_id=$2`, f.wh, f.itemID).Scan(&saleQty, &wasteQty); err != nil {
		t.Fatal(err)
	}
	if saleQty != 0 || !bdNear(wasteQty, qty) {
		t.Fatalf("mutasi sale=%v waste=%v", saleQty, wasteQty)
	}
}

func TestPOSVoidUnpaidDeductsRecipeAndRetriesOnce(t *testing.T) {
	f := newPOSVoidFixture(t)
	req := f.void()
	// Legacy clients only send names; matching must stay within the outlet.
	req.Items[0].ProductID = ""
	for i := 0; i < 2; i++ {
		if _, err := SaveOrder(f.outlet, req); err != nil {
			t.Fatal(err)
		}
	}
	f.assertWaste(t, 82, 18, 36, 1)
	var actor, notes, at string
	if err := database.DB.QueryRow(`SELECT created_by,notes,created_at::text FROM stock_wastes WHERE warehouse_id=$1`, f.wh).Scan(&actor, &notes, &at); err != nil {
		t.Fatal(err)
	}
	if actor != "Supervisor" || !strings.Contains(notes, f.order.LocalID) || at != "2026-10-21 06:10:00" {
		t.Fatalf("jejak waste: %s %s %s", actor, notes, at)
	}
}

func TestPOSVoidCustomRecipeWithoutMaster(t *testing.T) {
	f := newPOSVoidFixture(t)
	p := f.productRecipeFixture.product(t, "")
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='recipe' WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(`INSERT INTO product_recipes(id,product_id,item_id,qty_dist,qty_base,unit_used)
		VALUES($1,$2,$3,7,7,'g')`, NewULID(), p.ID, f.itemID); err != nil {
		t.Fatal(err)
	}
	req := f.void()
	req.Items = []models.OrderItem{{ProductID: p.LocalID, ProductName: p.Name, Qty: 2}}
	if _, err := SaveOrder(f.outlet, req); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 86, 14, 28, 1)
}

func TestPOSVoidPaidKeepsOriginalConsumptionAndCost(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	// Changing the recipe and current inventory cost cannot rewrite the sale's
	// ingredient quantities/FIFO cost when that payment is voided later.
	execScratch(`UPDATE recipe_items SET qty_base=90 WHERE recipe_master_id=$1`, f.product.RecipeMasterID)
	execScratch(`UPDATE cloud_products SET name='Nama baru' WHERE id=$1`, f.product.ID)
	f.order.Items[0].ProductID = ""
	f.stockIn(t, f.itemID, 10, 20, "")
	for i := 0; i < 2; i++ {
		if _, err := SaveOrder(f.outlet, f.void()); err != nil {
			t.Fatal(err)
		}
	}
	f.assertWaste(t, 92, 18, 36, 1)
	// A stale order and delayed payment must not resurrect the void or stock.
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 92, 18, 36, 1)
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id=$1`, f.outlet).Scan(&n); err != nil || n != 0 {
		t.Fatalf("transaksi void muncul lagi: %d %v", n, err)
	}
}

func TestPOSVoidInsufficientStockRollsBackAndCanRetry(t *testing.T) {
	f := newPOSVoidFixture(t)
	f.order.Items[0].Qty = 10
	if _, err := SaveOrder(f.outlet, f.void()); err == nil {
		t.Fatal("stok tidak cukup harus gagal")
	}
	f.assertWaste(t, 100, 0, 0, 0)
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM pos_void_stock_events WHERE outlet_id=$1`, f.outlet).Scan(&n)
	if n != 0 {
		t.Fatal("event yang gagal tidak boleh dianggap selesai")
	}
	f.stockIn(t, f.itemID, 100, 2, "")
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 20, 180, 360, 1)
}

func TestPOSVoidItemPartialEventsAndRemainingOrder(t *testing.T) {
	f := newPOSVoidFixture(t)
	req := models.PushOrderItemVoidRequest{LocalID: NewULID(), OrderID: f.order.LocalID, ProductID: f.product.LocalID, ProductName: f.product.Name, Qty: 1, VoidedBy: "Supervisor", VoidReason: "Batal satu", VoidedAt: "2026-10-21T06:02:00Z"}
	for i := 0; i < 2; i++ {
		if _, err := SaveOrderItemVoid(f.outlet, req); err != nil {
			t.Fatal(err)
		}
	}
	req.VoidedAt = "2026-10-21T13:02:00+07:00"
	if _, err := SaveOrderItemVoid(f.outlet, req); err != nil {
		t.Fatal(err)
	}
	// The same order item can be partly voided a second time; the timestamp
	// identifies a distinct event, while retries retain their original timestamp.
	req.VoidedAt = "2026-10-21T06:03:00Z"
	if _, err := SaveOrderItemVoid(f.outlet, req); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 46, 54, 108, 3)
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM order_item_voids WHERE outlet_id=$1`, f.outlet).Scan(&n)
	if n != 2 {
		t.Fatalf("riwayat void parsial %d", n)
	}
}

func TestPOSVoidConcurrentPaymentAndVoid(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var err error
			if i%2 == 0 {
				_, err = SaveTransaction(f.outlet, f.payment)
			} else {
				_, err = SaveOrder(f.outlet, f.void())
			}
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.assertWaste(t, 82, 18, 36, 1)
}

func TestPOSVoidCancelsFailedDeductionAndStaleRetry(t *testing.T) {
	f := newPOSVoidFixture(t)
	f.payment.Items[0].Quantity = 10
	f.order.Items[0].Qty = 10
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	var failure string
	if err := database.DB.QueryRow(`SELECT id FROM stock_deduction_failures WHERE outlet_id=$1 AND status='pending'`, f.outlet).Scan(&failure); err != nil {
		t.Fatal(err)
	}
	f.stockIn(t, f.itemID, 100, 2, "")
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	if err := retryPOSStockDeduction(failure, f.outlet, f.payment.LocalID, f.product.LocalID, 10); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 20, 180, 360, 1)
	var status string
	database.DB.QueryRow(`SELECT status FROM stock_deduction_failures WHERE id=$1`, failure).Scan(&status)
	if status != "cancelled" {
		t.Fatalf("retry belum dibatalkan: %s", status)
	}
}

func TestPOSVoidLegacySaleReclassifiesWithoutRecipeRecalculation(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	execScratch(`DELETE FROM pos_stock_deductions WHERE outlet_id=$1`, f.outlet)
	execScratch(`UPDATE stock_movements SET pos_product_local_id='' WHERE ref_type='transaction' AND ref_id=$1`, f.payment.LocalID)
	// Historical recipe may no longer be present. The original movement remains
	// enough evidence to register the loss without deducting anything else.
	execScratch(`DELETE FROM recipe_items WHERE recipe_master_id=$1`, f.product.RecipeMasterID)
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 82, 18, 36, 1)
}

func TestPOSVoidUnknownOrAmbiguousProductDoesNotGuess(t *testing.T) {
	f := newPOSVoidFixture(t)
	req := f.void()
	req.Items = []models.OrderItem{{ProductName: "Produk tidak dikenal", Qty: 1}}
	if _, err := SaveOrder(f.outlet, req); err == nil {
		t.Fatal("produk tidak dikenal harus gagal")
	}
	f.assertWaste(t, 100, 0, 0, 0)
	other := f.productRecipeFixture.product(t, "")
	execScratch(`UPDATE cloud_products SET name=$2 WHERE id=$1`, other.ID, f.product.Name)
	req.Items[0].ProductName = f.product.Name
	if _, err := SaveOrder(f.outlet, req); err == nil {
		t.Fatal("nama ambigu harus gagal")
	}
	f.assertWaste(t, 100, 0, 0, 0)
}

func TestPOSVoidPartialDeductionSharedIngredient(t *testing.T) {
	f := newPOSVoidFixture(t)
	r := f.recipe(t, "public", false, 90, false)
	p := f.productRecipeFixture.product(t, r)
	f.payment.Items = append(f.payment.Items, models.TransactionItem{ProductID: p.LocalID, ProductName: p.Name, Quantity: 1})
	f.order.Items = append(f.order.Items, models.OrderItem{ProductID: p.LocalID, ProductName: p.Name, Qty: 1})
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	// One product succeeds; the other runs out of the shared ingredient.
	f.stockIn(t, f.itemID, 100, 2, "")
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 92, 108, 216, 2)
}

func TestPOSVoidSingleStockAndUntrackedProduct(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := database.DB.Exec(`UPDATE cloud_products SET stock_type='single',linked_stock_item_id=$2 WHERE id=$1`, f.product.ID, f.itemID); err != nil {
		t.Fatal(err)
	}
	f.order.Items[0].Qty = 2
	untracked := f.productRecipeFixture.product(t, "")
	f.order.Items = append(f.order.Items, models.OrderItem{ProductID: untracked.LocalID, ProductName: untracked.Name, Qty: 3})
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 98, 2, 4, 1)
}

func TestPOSVoidDoesNotUseAnotherOutletsProduct(t *testing.T) {
	f := newPOSVoidFixture(t)
	other := newPOSVoidFixture(t)
	req := f.void()
	req.Items = []models.OrderItem{{ProductID: other.product.LocalID, ProductName: other.product.Name, Qty: 1}}
	if _, err := SaveOrder(f.outlet, req); err == nil {
		t.Fatal("produk outlet lain harus ditolak")
	}
	f.assertWaste(t, 100, 0, 0, 0)
	other.assertWaste(t, 100, 0, 0, 0)
}

func TestPOSVoidReportsLossWithoutSalesCOGS(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	report, err := ListFinanceOperations("2026-10-20", "2026-10-23", f.outlet, "stock", "", "5-380", "", 1, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 1 {
		t.Fatalf("operasi kerugian %d, ingin 1", report.Total)
	}
	var loss, inventory float64
	for _, p := range report.Rows[0].Postings {
		if p.AccountCode == "5-380" {
			loss += p.Debit
		}
		if p.AccountCode == "1-300" {
			inventory += p.Credit
		}
		if p.AccountCode == glAccCOGS {
			t.Fatal("void masih masuk HPP penjualan")
		}
	}
	if !bdNear(loss, 36) || !bdNear(inventory, 36) {
		t.Fatalf("kerugian %v, persediaan %v", loss, inventory)
	}
}

func TestPOSVoidMigrationsPreserveCompletedEvents(t *testing.T) {
	f := newPOSVoidFixture(t)
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := database.RunPOSVoidMigrations(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SaveOrder(f.outlet, f.void()); err != nil {
		t.Fatal(err)
	}
	f.assertWaste(t, 82, 18, 36, 1)
}
