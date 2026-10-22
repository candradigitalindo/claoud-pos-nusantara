package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestFnbPriceTransferTimingPlanningAndHistoricalFinance(t *testing.T) {
	outlet, central, item := unitWarehouses(t)
	if _, err := database.DB.Exec(`UPDATE stock_items SET name='Cabe uji harga',base_unit='g',dist_unit='kg',dist_ratio=1000 WHERE id=$1`, item); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { execScratch(`DELETE FROM cloud_products WHERE outlet_id=$1`, outlet.outlet) })
	receive := func(warehouse string, price float64) {
		t.Helper()
		if _, err := CreateGoodsReceipt(models.GoodsReceiptRequest{WarehouseID: warehouse, Items: []models.GoodsReceiptItemReq{{ItemID: item, QtyDist: 5, UnitUsed: "kg", CostPerUnit: &price}}}, "uji-harga"); err != nil {
			t.Fatal(err)
		}
	}
	receive(outlet.wh, 10000)
	receive(central.wh, 20000)
	rid := costTestRecipe(t, item, 100)
	p, err := AdminCreateProduct(models.AdminCreateProductRequest{OutletID: outlet.outlet, Name: "Menu cabe", Price: 5000, RecipeMasterID: rid})
	if err != nil {
		t.Fatal(err)
	}
	scope := []string{outlet.outlet}
	if err := SaveFnbOutletPolicy(outlet.outlet, models.FnbOutletPolicy{TargetCostPct: 40}, scope, "uji"); err != nil {
		t.Fatal(err)
	}
	check := func(current, projection float64, currentAlert bool) {
		t.Helper()
		rows, err := GetFnbCosting(outlet.outlet, scope)
		if err != nil || len(rows) != 1 {
			t.Fatalf("F&B: %v %+v", err, rows)
		}
		r := rows[0]
		if r.CostStatus != "ready" || r.CostBasis != "outlet_stock" || !bdNear(r.TotalCost, current) || r.ProjectedTotalCost == nil || !bdNear(*r.ProjectedTotalCost, projection) || r.TargetCostPct != 40 || r.TargetSource != "outlet" || r.IsOver != currentAlert || r.ProjectedIsOver != (projection/5000*100 >= 40) {
			t.Fatalf("F&B prices: %+v", r)
		}
		ppic, err := GetPpicHppReport(outlet.outlet, 35, scope)
		if err != nil || len(ppic.Rows) != 1 || !bdNear(ppic.Rows[0].Hpp, current) || ppic.Rows[0].TargetCostPct != 40 || ppic.Rows[0].ProjectedHpp == nil || !bdNear(*ppic.Rows[0].ProjectedHpp, projection) {
			t.Fatalf("PPIC disagrees with F&B: %+v %v", ppic, err)
		}
	}
	check(1000, 2000, false) // central purchase changes the forecast, not outlet stock.
	doc := unitTransfer(t, central.wh, outlet.wh, item, "kg", 5)
	check(1000, 2000, false)   // shipped but not yet received.
	receive(central.wh, 30000) // purchase changes during transit.
	check(1000, 3000, false)
	if _, err := UpdateTransferStatus(doc.ID, "received", "uji-penerima"); err != nil {
		t.Fatal(err)
	}
	check(1500, 3000, false) // 5 kg @10k + 5 kg @20k, not the new central price @30k.
	if qty, avg := outlet.ledger(t, item); qty != 10000 || avg != 15 {
		t.Fatalf("received balance: %v @%v", qty, avg)
	}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	pl, err := GetProfitLossReport(today, today, outlet.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.COGS, 0, "purchase/transfer is inventory, not expense")
	ref := NewULID()
	if err := DeductStockByRecipe(outlet.outlet, p.LocalID, 50, ref, "HARGA-UJI"); err != nil {
		t.Fatal(err)
	}
	check(2000, 3000, true) // old batch exhausted; remaining stock is @20k/kg. 40% is inclusive.
	if qty, avg := outlet.ledger(t, item); qty != 5000 || avg != 20 {
		t.Fatalf("remaining batches: %v @%v", qty, avg)
	}
	pl, err = GetProfitLossReport(today, today, outlet.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.COGS, 50000, "50 portions consumed old batch @10k/kg")
	bal, err := GetBalanceReport(today, today, outlet.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, bal.Inventory, 100000, "5 kg @20k remaining")
	receive(central.wh, 8000)
	check(2000, 800, true) // Lower purchase prices only change the projection until new stock arrives.
	if _, err := SaveRecipeMaster(rid, models.RecipeMasterRequest{Name: "Cabe takaran baru", Visibility: "public", Items: []models.RecipeItemReq{{ItemID: item, QtyBase: 200}}}); err != nil {
		t.Fatal(err)
	}
	pl, err = GetProfitLossReport(today, today, outlet.outlet, scope)
	if err != nil {
		t.Fatal(err)
	}
	financeNear(t, pl.Summary.COGS, 50000, "recipe edits do not reprice historical COGS")
	data, _, err := BuildPpicReportExcel("hpp", today, today, "", outlet.outlet, 35, "uji", scope)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows(book.GetSheetName(0))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if len(row) > 16 && row[0] == p.Name {
			found = true
			pct, parseErr := strconv.ParseFloat(strings.TrimSuffix(row[13], "%"), 64)
			if parseErr != nil || pct != 40 {
				t.Fatalf("export threshold: %+v", row)
			}
			if row[16] != "ready" {
				t.Fatalf("export status: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("export omitted product")
	}
}

func TestFnbOutletPolicyScopeIsolationAndThreshold(t *testing.T) {
	f, other := newProductRecipeFixture(t), newProductRecipeFixture(t)
	rid := costTestRecipe(t, f.itemID, 1)
	p := f.product(t, rid)
	f.stockIn(t, f.itemID, 10, 4000, "")
	if err := SaveFnbProfile(p.ID, models.FnbProfile{TargetCostPct: 20}, nil, "uji"); err != nil {
		t.Fatal(err)
	}
	if err := SaveFnbOutletPolicy(f.outlet, models.FnbOutletPolicy{TargetCostPct: 40}, []string{f.outlet}, "uji"); err != nil {
		t.Fatal(err)
	}
	for _, pct := range []float64{0, -1, 101, math.NaN(), math.Inf(1)} {
		if err := SaveFnbOutletPolicy(f.outlet, models.FnbOutletPolicy{TargetCostPct: pct}, nil, "uji"); err == nil {
			t.Fatalf("invalid policy accepted: %v", pct)
		}
	}
	if err := SaveFnbOutletPolicy(other.outlet, models.FnbOutletPolicy{TargetCostPct: 60}, []string{f.outlet}, "uji"); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("policy scope bypass: %v", err)
	}
	if _, err := GetFnbOutletPolicy(other.outlet, []string{f.outlet}); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("policy read bypass: %v", err)
	}
	policy, err := GetFnbOutletPolicy(f.outlet, []string{f.outlet})
	if err != nil || !policy.Configured || policy.TargetCostPct != 40 {
		t.Fatalf("saved policy: %+v %v", policy, err)
	}
	untouched, err := GetFnbOutletPolicy(other.outlet, nil)
	if err != nil || untouched.Configured {
		t.Fatalf("other outlet changed: %+v %v", untouched, err)
	}
	rows, err := GetFnbCosting(f.outlet, nil)
	if err != nil || rows[0].TargetCostPct != 40 || !rows[0].IsOver {
		t.Fatalf("40%% boundary: %+v %v", rows, err)
	}
	if rows[0].ProjectedTotalCost != nil {
		t.Fatal("missing central price fabricated as a projection")
	}
	if _, err := GetPpicHppReport(other.outlet, 35, []string{f.outlet}); !errors.Is(err, ErrFnbScope) {
		t.Fatalf("PPIC scope bypass: %v", err)
	}
	if err := SaveFnbOutletPolicy(f.outlet, models.FnbOutletPolicy{TargetCostPct: 45}, nil, "uji"); err != nil {
		t.Fatal(err)
	}
	rows, err = GetFnbCosting(f.outlet, nil)
	if err != nil || rows[0].IsOver || rows[0].TargetCostPct != 45 {
		t.Fatalf("policy update not applied: %+v %v", rows, err)
	}
	// New products in this outlet inherit the policy, without their own profile.
	created := f.product(t, rid)
	rows, err = GetFnbCosting(f.outlet, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ProductID == created.ID && r.TargetCostPct != 45 {
			t.Fatal("new product did not inherit outlet limit")
		}
	}
}
