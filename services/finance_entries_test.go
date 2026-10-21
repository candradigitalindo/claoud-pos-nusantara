package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

func TestFinanceDisposedAssetKeepsHistory(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	id, disposal := NewULID(), NewULID()
	if _, err := database.DB.Exec(`INSERT INTO assets(id,outlet_id,name,quantity,purchase_date,purchase_price,useful_life_months,residual_value,status) VALUES($1,$2,'Aset riwayat',0,'2026-01-15',1200000,12,0,'dihapus')`, id, f.outlet); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM asset_disposals WHERE id=$1`, disposal)
		execScratch(`DELETE FROM assets WHERE id=$1`, id)
	})
	if _, err := database.DB.Exec(`INSERT INTO asset_disposals(id,disposal_number,asset_id,qty,method,reason,proceeds,book_value,status,created_at,approved_at) VALUES($1,$3,$2,1,'jual','uji',650000,700000,'approved','2026-06-20 03:00','2026-06-21 03:00')`, disposal, id, "UJI-"+disposal); err != nil {
		t.Fatal(err)
	}
	mar, err := GetBalanceReport("2026-03-01", "2026-03-31", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, mar.FixedAssets, 1000000, "historical asset after disposal")
	jun, err := GetBalanceReport("2026-06-01", "2026-06-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, jun.FixedAssets, 0, "disposed asset cleared")
	cf, err := GetCashFlowReport("2026-06-01", "2026-06-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, cf.Summary.TotalReceipts, 0, "disposal proceeds need payment confirmation")
	dep, _, err := depreciationForPeriod("2026-07-01", "2026-12-31", []string{f.outlet})
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, dep, 0, "no depreciation after disposal")
}

func TestFinanceConsumablesBecomeExpenseOnce(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	pr := newReceivableGoodsPR(t, 50000)
	t.Cleanup(func() { cleanupReceivablePR(pr.ID) })
	for _, action := range []string{"request_payment", "pay"} {
		if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: action, ActorName: "uji"}); err != nil {
			t.Fatal(err)
		}
	}
	execScratch(`UPDATE purchase_requests SET outlet_id=$2,approved_at='2026-09-10 03:00' WHERE id=$1`, pr.ID, f.outlet)
	execScratch(`UPDATE payment_histories SET created_at='2026-09-11 03:00',payment_proof='REF-UJI | /uploads/2026-09/bukti.png' WHERE purchase_request_id=$1`, pr.ID)
	key := prItemKey(pr.Items[0].Name, pr.Items[0].Items[0].Name, 0)
	if _, err := database.DB.Exec(`INSERT INTO pr_receiving_decisions(id,purchase_request_id,pr_item_key,destination,qty,reason,actor,created_at) VALUES($1,$2,$3,'habis',2,'Dipakai operasional','uji','2026-09-12 03:00')`, NewULID(), pr.ID, key); err != nil {
		t.Fatal(err)
	}
	gl, err := GetGeneralLedger("2026-09-01", "2026-09-30", f.outlet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, glAccount(gl, "5-300").Balance, 100000, "consumable expense")
	financeNear(t, glAccount(gl, "1-400").Balance, 0, "not equipment")
	financeNear(t, glAccount(gl, "1-100").Balance, -100000, "payment occurs once")
	payments, err := ListFinanceOperations("2026-09-01", "2026-09-30", f.outlet, "procurement_payment", "", "", "", 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(payments.Rows) != 1 || payments.Rows[0].ProofURL != "/uploads/2026-09/bukti.png" {
		t.Fatalf("procurement proof: %+v", payments)
	}
}

func TestFinanceProjectCapitalizationAcrossOutlets(t *testing.T) {
	requireDB(t)
	origin, destination := newBreakdownFixture(t), newBreakdownFixture(t)
	project := newTestProject(t, 2000000)
	t.Cleanup(func() { execScratch(`DELETE FROM projects WHERE id=$1`, project.ID) })
	pr, err := createPricedPR(models.CreatePurchaseRequestInput{RequestType: "barang", RequestedBy: "uji", ProjectID: project.ID, Items: []models.PurchaseRequestItem{{Name: "Peralatan", Items: []models.PurchaseSubItem{{Name: "Mixer uji", Qty: 1, Unit: "unit", HpsPrice: 1200000, FinalPrice: 1200000}}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupReceivablePR(pr.ID) })
	verifyAndApprove(t, pr.ID)
	execScratch(`UPDATE purchase_requests SET outlet_id=$2,approved_at='2026-09-10 03:00' WHERE id=$1`, pr.ID, origin.outlet)
	id := NewULID()
	if _, err = database.DB.Exec(`INSERT INTO assets(id,outlet_id,name,quantity,purchase_date,purchase_price,useful_life_months,residual_value,status,purchase_request_id,created_at) VALUES($1,$2,'Mixer projek',1,'2026-09-15',1200000,12,0,'aktif',$3,'2026-09-15 03:00')`, id, destination.outlet, pr.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM assets WHERE id=$1`, id) })
	for _, f := range []*bdFixture{origin, destination} {
		bal, e := GetBalanceReport("2026-09-01", "2026-09-30", f.outlet, nil)
		if e != nil {
			t.Fatal(e)
		}
		financeNear(t, bal.ProjectsInProgress, 0, "project moved to equipment")
		want := 0.0
		if f.outlet == destination.outlet {
			want = 1200000
		}
		financeNear(t, bal.FixedAssets, want, "equipment at receiving outlet")
		financeNear(t, bal.CashAndEquivalents, 0, "capitalization is noncash")
	}
}

func financeTestActor() FinanceAccess {
	return FinanceAccess{ActorID: "finance-test", Actor: "Finance Uji", Permissions: map[string]bool{"*": true}}
}
func financeTestDraft(t *testing.T, outlet string, amount float64) *models.FinanceEntry {
	t.Helper()
	r, err := CreateFinanceEntry(models.FinanceEntryInput{RequestKey: NewULID(), Direction: "out", AccountCode: "5-310", OutletID: outlet, Amount: amount, OccurredAt: "2026-09-15", Period: "2026-09", Counterparty: "Tim Outlet", Description: "Gaji September", Notes: "Total gaji divisi, 8 staf", Reference: "PAYROLL-SEP", Method: "cash"}, financeTestActor())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM finance_entry_events WHERE entry_id=$1`, r.ID)
		execScratch(`DELETE FROM finance_entries WHERE id=$1`, r.ID)
	})
	return r
}
func financeNear(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > .005 {
		t.Fatalf("%s: got %.2f want %.2f", label, got, want)
	}
}

func TestFinanceSalaryPostingReversalAndReports(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	a := financeTestActor()
	d := financeTestDraft(t, f.outlet, 8000000)
	cf, err := GetCashFlowReport("2026-09-01", "2026-09-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, cf.Summary.TotalPayments, 0, "draft excluded")
	if _, err = ActFinanceEntry(d.ID, "post", "", d.Version, a); err == nil {
		t.Fatal("posting without proof accepted")
	}
	r := d.FinanceEntryInput
	r.ProofURL = "/uploads/2026-09/payroll.png"
	d, err = UpdateFinanceEntry(d.ID, r, a)
	if err != nil {
		t.Fatal(err)
	}
	staff := a
	staff.Permissions = map[string]bool{"finance.entries.create": true}
	if _, err = ActFinanceEntry(d.ID, "post", "", d.Version, staff); !errors.Is(err, ErrFinanceAccess) {
		t.Fatalf("staff post: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := ActFinanceEntry(d.ID, "post", "", d.Version, a); results <- e }()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else {
			var conflict ConflictError
			if !errors.As(e, &conflict) {
				t.Fatal(e)
			}
		}
	}
	if success != 1 {
		t.Fatalf("posting succeeded %d times", success)
	}
	d, err = GetFinanceEntry(d.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.History) != 3 {
		t.Fatalf("audit events: %d", len(d.History))
	}
	if _, err = UpdateFinanceEntry(d.ID, d.FinanceEntryInput, a); err == nil {
		t.Fatal("posted entry editable")
	}
	gl, err := GetGeneralLedger("2026-09-01", "2026-09-30", f.outlet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, glAccount(gl, "5-310").Balance, 8000000, "salary account")
	financeNear(t, gl.Summary.CashBalance, -8000000, "cash ledger")
	cf, err = GetCashFlowReport("2026-09-01", "2026-09-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, cf.Summary.NetCashFlow, gl.Summary.CashBalance, "cash matches GL")
	pl, err := GetProfitLossReport("2026-09-01", "2026-09-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.NetProfit, -8000000, "salary expense")
	financeNear(t, pl.Daily[0].NetProfit, pl.Summary.NetProfit, "daily totals")
	financeNear(t, pl.ByOutlet[0].NetProfit, pl.Summary.NetProfit, "outlet totals")
	bal, err := GetBalanceReport("2026-09-01", "2026-09-30", f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, bal.CashAndEquivalents, -8000000, "balance cash")
	if _, err = ActFinanceEntry(d.ID, "void", "", d.Version, a); err == nil {
		t.Fatal("void without reason accepted")
	}
	if _, err = ActFinanceEntry(d.ID, "void", "Koreksi total gaji", d.Version, a); err != nil {
		t.Fatal(err)
	}
	cf, _ = GetCashFlowReport("2026-09-01", "2026-09-30", f.outlet, nil)
	financeNear(t, cf.Summary.NetCashFlow, -8000000, "historical period unchanged")
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	bal, err = GetBalanceReport(today, today, f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, bal.CashAndEquivalents, 0, "reversal corrected cumulative balance")
	ops, err := ListFinanceOperations("2026-09-01", today, f.outlet, "", "", "5-310", "", 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ops.Total != 2 {
		t.Fatalf("source and reversal: %d", ops.Total)
	}
}

func TestFinanceEntryValidationIdempotencyAndScope(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	a := financeTestActor()
	d := financeTestDraft(t, f.outlet, 100)
	copy, err := CreateFinanceEntry(d.FinanceEntryInput, a)
	if err != nil || copy.ID != d.ID {
		t.Fatalf("idempotency: %v %+v", err, copy)
	}
	changed := d.FinanceEntryInput
	changed.Amount++
	if _, err = CreateFinanceEntry(changed, a); err == nil {
		t.Fatal("reused key with different amount accepted")
	}
	restricted := a
	restricted.Outlets = []string{}
	if _, err = GetFinanceEntry(d.ID, restricted); !errors.Is(err, ErrFinanceAccess) {
		t.Fatalf("read scope: %v", err)
	}
	if _, err = ActFinanceEntry(d.ID, "void", "uji", d.Version, restricted); !errors.Is(err, ErrFinanceAccess) {
		t.Fatalf("write scope: %v", err)
	}
	if _, err = GetCashFlowReport("2026-09-01", "2026-09-30", f.outlet, []string{}); !errors.Is(err, ErrFinanceAccess) {
		t.Fatalf("report scope: %v", err)
	}
	for _, edit := range []func(*models.FinanceEntryInput){func(r *models.FinanceEntryInput) { r.Amount = math.NaN() }, func(r *models.FinanceEntryInput) { r.Amount = -1 }, func(r *models.FinanceEntryInput) { r.Period = "" }, func(r *models.FinanceEntryInput) { r.AccountCode = "4-200" }, func(r *models.FinanceEntryInput) { r.ProofURL = "https://example.com/proof" }} {
		r := d.FinanceEntryInput
		r.RequestKey = NewULID()
		edit(&r)
		if _, err = CreateFinanceEntry(r, a); err == nil {
			t.Fatalf("invalid entry accepted: %+v", r)
		}
	}
}

func TestFinanceSupplierRefundAppearsOnceWithLoss(t *testing.T) {
	requireDB(t)
	outlet, central, item := unitWarehouses(t)
	qualityCleanup(t, outlet.wh, central.wh)
	central.stockIn(t, item, 24, 2000, "")
	vendor := qualityVendor(t)
	qc := qualityActor("qc", nil, "*")
	manager := qualityActor("supervisor", nil, "*")
	exec := qualityActor("logistik", nil, "*")
	finance := qualityActor("finance", nil, "*")
	id := qualityReport(t, central, item, "return", 1, "dus", qc)
	qualityDo(t, id, "approve", manager, func(r *models.StockQualityAction) {
		r.Disposition = "return_supplier"
		r.VendorID = vendor
		r.Reference = "INV-TEST"
		r.Settlement = "refund"
	})
	qualityDo(t, id, "execute", exec, nil)
	qualityDo(t, id, "refund", finance, func(r *models.StockQualityAction) { r.QtyBase = 24; r.Amount = 45600; r.Reference = "REFUND-TEST" })
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	ops, err := ListFinanceOperations(today, today, "", "stock_quality", "", "", qualityRead(t, id).Number, 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ops.Total != 1 {
		t.Fatalf("refund count %d", ops.Total)
	}
	financeNear(t, ops.CashIn, 45600, "supplier refund cash")
	var loss float64
	for _, p := range ops.Rows[0].Postings {
		if p.AccountCode == "5-390" {
			loss += p.Debit - p.Credit
		}
	}
	financeNear(t, loss, 2400, "refund difference")
	if ops.Rows[0].ProofURL == "" || ops.Rows[0].Notes == "" {
		t.Fatal("missing source evidence/notes")
	}
}

func TestFinanceSaleVoidAndStockConsumption(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	item := f.item(t, "Bahan Uji", "pcs", "pcs", 1)
	f.stockIn(t, item, 10, 5000, "")
	tx, err := database.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = applyMovement(tx, item, f.wh, "sale", "pcs", "sale-test", "transaction", "SALE-TEST", "dua porsi", "kasir", -2, -2, 0, ""); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	orderID, txID := NewULID(), NewULID()
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_transactions WHERE id=$1`, txID)
		execScratch(`DELETE FROM cloud_orders WHERE id=$1`, orderID)
	})
	if _, err = database.DB.Exec(`INSERT INTO cloud_orders(id,local_id,outlet_id,outlet_code,total_amount,payment_info) VALUES($1::text,$1::text,$2,'UJI',20000,'{"payment_status":"paid","voided_at":"2026-10-21T00:00:00Z"}')`, orderID, f.outlet); err != nil {
		t.Fatal(err)
	}
	if _, err = database.DB.Exec(`INSERT INTO cloud_transactions(id,local_id,outlet_id,outlet_code,order_id,total_amount,tax_amount) VALUES($1::text,$1::text,$2,'UJI',$3,20000,2000)`, txID, f.outlet, orderID); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	cf, err := GetCashFlowReport(today, today, f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, cf.Summary.SalesReceipts, 0, "void cash excluded")
	pl, err := GetProfitLossReport(today, today, f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.SalesRevenue, 0, "void revenue excluded")
	financeNear(t, pl.Summary.COGS, 10000, "actual stock consumed")
	bal, err := GetBalanceReport(today, today, f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, bal.Inventory, 40000, "remaining inventory")
	execScratch(`UPDATE cloud_orders SET payment_info='{"payment_status":"paid"}' WHERE id=$1`, orderID)
	pl, err = GetProfitLossReport(today, today, f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pl.UncostedSales != 1 {
		t.Fatalf("unlinked sale warning: %d", pl.UncostedSales)
	}
	review, err := ListFinanceOperations(today, today, f.outlet, "review", "", "", "", 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if review.Total < 1 {
		t.Fatal("unlinked sale missing from review filter")
	}
	for _, row := range review.Rows {
		if row.ReviewNote == "" {
			t.Fatal("review filter included a complete record")
		}
	}
}

func TestFinanceCashClassificationKeepsOneSource(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	id := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO cloud_cash_movements(id,local_id,outlet_id,shift_id,movement_type,amount,note) VALUES($1::text,$1::text,$2,'uji','out',150000,'Gaji harian')`, id, f.outlet); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM finance_cash_classifications WHERE movement_id=$1`, id)
		execScratch(`DELETE FROM cloud_cash_movements WHERE id=$1`, id)
	})
	a := financeTestActor()
	if err := ClassifyFinanceCash(id, "5-310", "Gaji tim outlet", a); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	ops, err := ListFinanceOperations(today, today, f.outlet, "pos_cash", "", "5-310", "", 1, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ops.Total != 1 || ops.Rows[0].ReviewNote != "" {
		t.Fatalf("classification: %+v", ops)
	}
	financeNear(t, ops.CashOut, 150000, "classified expense")
	gl, err := GetGeneralLedger(today, today, f.outlet, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, glAccount(gl, "5-310").Balance, 150000, "salary account after classification")
	financeNear(t, glAccount(gl, "5-300").Balance, 0, "old generic expense removed")
	if err = ClassifyFinanceCash(id, "4-200", "salah arah", a); err == nil {
		t.Fatal("income account accepted for expense")
	}
}

func TestFinanceTransferAndRejectRemainAssets(t *testing.T) {
	requireDB(t)
	outlet, central, item := unitWarehouses(t)
	qualityCleanup(t, outlet.wh, central.wh)
	central.stockIn(t, item, 48, 2000, "")
	doc := unitTransfer(t, central.wh, outlet.wh, item, "dus", 2)
	receiver := qualityActor("penerima", []string{outlet.outlet}, "stocktransfers.receive", "stocktransfers.reject", "stockquality.outlet.report")
	if err := ReceiveStockTransfer(doc.ID, models.StockInspectionRequest{Version: 0, EvidenceID: qualityPhoto(t, outlet.wh, receiver), Items: []models.StockInspectionItem{{ItemID: item, Accepted: 20, Rejected: 4, Reason: "Bocor"}}}, receiver); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	bal, err := GetBalanceReport(today, today, outlet.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, bal.Inventory, 48000, "outlet accepted plus quarantine")
	pl, err := GetProfitLossReport(today, today, outlet.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.NetProfit, 0, "transfer and quarantine not expense")
	cf, err := GetCashFlowReport(today, today, outlet.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, cf.Summary.NetCashFlow, 0, "transfer not cash")
}
