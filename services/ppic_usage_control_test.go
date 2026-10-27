package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

const usageDay = "2026-10-20"
const usageAt = "2026-10-20 05:00:00"

// These fixtures represent persisted inventory evidence, including legacy and
// incomplete records. Tests never derive expected consumption using a recipe.
// The schema is isolated and dropped after each test, preserving immutable
// policy-history constraints without ever deleting their audit rows directly.
func usageReportDB(t *testing.T) usagePolicyFixture {
	t.Helper()
	f := usagePolicyDB(t)
	for _, q := range []string{
		`CREATE FUNCTION tz_day_start(d DATE) RETURNS TIMESTAMP LANGUAGE SQL IMMUTABLE AS $$ SELECT d::timestamp - INTERVAL '7 hours' $$`,
		`CREATE TABLE stock_ledger(item_id CHAR(26),warehouse_id CHAR(26),qty_base NUMERIC(15,4),PRIMARY KEY(item_id,warehouse_id))`,
		`CREATE TABLE stock_movements(id CHAR(26) PRIMARY KEY,item_id CHAR(26),warehouse_id CHAR(26),movement_type TEXT,ref_type TEXT,ref_id TEXT,qty_base NUMERIC(15,4),cost_per_base NUMERIC(15,2),pos_product_local_id TEXT DEFAULT '',created_at TIMESTAMP)`,
		`CREATE TABLE stock_wastes(id CHAR(26) PRIMARY KEY,item_id CHAR(26),warehouse_id CHAR(26),qty_base NUMERIC(15,4),total_cost NUMERIC(15,2),reason TEXT,void_event_id CHAR(26),source_movement_id CHAR(26),created_at TIMESTAMP)`,
		`CREATE TABLE cloud_orders(id CHAR(26) PRIMARY KEY,local_id TEXT,outlet_id CHAR(26),items JSONB,payment_info JSONB,is_holding BOOL DEFAULT false,UNIQUE(outlet_id,local_id))`,
		`CREATE TABLE cloud_transactions(id CHAR(26) PRIMARY KEY,outlet_id CHAR(26),order_id TEXT,items JSONB,created_at TIMESTAMP,synced_at TIMESTAMP)`,
		`CREATE TABLE pos_void_stock_events(id CHAR(26) PRIMARY KEY,outlet_id CHAR(26),order_id TEXT,source_type TEXT)`,
		`CREATE TABLE stock_opnames(id CHAR(26) PRIMARY KEY,warehouse_id CHAR(26),opname_number TEXT,status TEXT,approved_at TIMESTAMP)`,
		`CREATE TABLE stock_opname_items(opname_id CHAR(26),item_id CHAR(26),qty_system_base NUMERIC(15,4),qty_counted_base NUMERIC(15,4))`,
		`CREATE TABLE work_orders(id CHAR(26) PRIMARY KEY,warehouse_id CHAR(26),status TEXT,qty_planned_base NUMERIC(15,4),qty_actual_base NUMERIC(15,4),finished_at TIMESTAMP)`,
		`CREATE TABLE work_order_materials(wo_id CHAR(26),item_id CHAR(26),qty_plan_base NUMERIC(15,4),qty_actual_base NUMERIC(15,4),UNIQUE(wo_id,item_id))`,
		`CREATE TABLE stock_breakdowns(id CHAR(26) PRIMARY KEY,warehouse_id CHAR(26),input_item_id CHAR(26),input_qty_base NUMERIC(15,4))`,
		`CREATE TABLE stock_deduction_failures(id CHAR(26) PRIMARY KEY,outlet_id CHAR(26),status TEXT,created_at TIMESTAMP)`,
		`CREATE TABLE pos_stock_deductions(outlet_id CHAR(26),transaction_id VARCHAR(50),product_local_id VARCHAR(100),qty NUMERIC(15,4),PRIMARY KEY(outlet_id,transaction_id,product_local_id))`,
		`CREATE TABLE product_recipes(id CHAR(26),product_id CHAR(26),item_id CHAR(26),qty_base NUMERIC(15,4))`,
	} {
		usageSQL(t, q)
	}
	return f
}

func usageSQL(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	if _, err := database.DB.Exec(q, args...); err != nil {
		t.Fatalf("fixture: %v; SQL: %s", err, q)
	}
}
func usageLedger(t *testing.T, f usagePolicyFixture, wh string, qty float64) {
	t.Helper()
	usageSQL(t, `INSERT INTO stock_ledger VALUES($1,$2,$3)`, f.item, wh, qty)
}
func usagePolicy(t *testing.T, f usagePolicyFixture, usage, waste float64) {
	t.Helper()
	if _, err := SavePpicUsagePolicy(usagePolicyInput(f, usage, waste, 0), nil, "PPIC uji"); err != nil {
		t.Fatal(err)
	}
}
func usageMovement(t *testing.T, f usagePolicyFixture, kind, ref, id, at string, qty float64) string {
	t.Helper()
	movement := NewULID()
	usageSQL(t, `INSERT INTO stock_movements(id,item_id,warehouse_id,movement_type,ref_type,ref_id,qty_base,cost_per_base,pos_product_local_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,10,'P',$8::timestamp)`, movement, f.item, f.own, kind, ref, id, qty, at)
	return movement
}
func usagePayment(t *testing.T, f usagePolicyFixture, order, created, synced string) string {
	t.Helper()
	id := NewULID()
	usageSQL(t, `INSERT INTO cloud_transactions VALUES($1,$2,$3,'[{"product_id":"P","quantity":1}]',$4::timestamp,$5::timestamp)`, id, f.outlet, order, created, synced)
	return id
}
func usageWaste(t *testing.T, f usagePolicyFixture, reason, movement, at string, qty float64) string {
	t.Helper()
	id := NewULID()
	usageSQL(t, `INSERT INTO stock_wastes(id,item_id,warehouse_id,qty_base,total_cost,reason,source_movement_id,created_at) VALUES($1,$2,$3,$4::numeric,$4::numeric*10,$5,NULLIF($6,'')::char(26),$7::timestamp)`, id, f.item, f.own, qty, reason, movement, at)
	return id
}
func usageCount(t *testing.T, f usagePolicyFixture, at string) {
	t.Helper()
	id := NewULID()
	usageSQL(t, `INSERT INTO stock_opnames VALUES($1,$2,'OP-UJI','approved',$3::timestamp)`, id, f.own, at)
	usageSQL(t, `INSERT INTO stock_opname_items VALUES($1,$2,1000,1)`, id, f.item)
}
func usageReport(t *testing.T, f usagePolicyFixture) *models.PpicUsageControlReport {
	t.Helper()
	r, err := GetPpicUsageControl(usageDay, usageDay, f.own, "", "", nil, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func usageOnlyRow(t *testing.T, f usagePolicyFixture) models.PpicUsageControlRow {
	t.Helper()
	r := usageReport(t, f)
	if len(r.Rows) != 1 {
		t.Fatalf("expected one row: %+v", r)
	}
	return r.Rows[0]
}

func TestPpicUsageControlLimitsAndStatus(t *testing.T) {
	policy := func(u, w float64) *models.PpicUsagePolicy {
		return &models.PpicUsagePolicy{UsageTolerancePct: u, WasteTolerancePct: w}
	}
	for _, tt := range []struct {
		name   string
		row    models.PpicUsageControlRow
		status string
	}{
		{"unset idle", models.PpicUsageControlRow{}, "unset"},
		{"explicit zero idle", models.PpicUsageControlRow{Policy: policy(0, 0)}, "no_activity"},
		{"no denominator", models.PpicUsageControlRow{WasteQty: 2, Policy: policy(0, 0)}, "no_basis"},
		{"count required", models.PpicUsageControlRow{SaleQty: 10, Policy: policy(0, 0)}, "needs_count"},
		{"zero exact", models.PpicUsageControlRow{SaleQty: 10, HasStocktake: true, Policy: policy(0, 0)}, "within_limit"},
		{"at both limits", models.PpicUsageControlRow{SaleQty: 10, AdjustmentShortageQty: 1, WasteQty: 0.5, HasStocktake: true, Policy: policy(10, 5)}, "within_limit"},
		{"strict tiny waste", models.PpicUsageControlRow{SaleQty: 10, WasteQty: 0.0001, HasStocktake: true, Policy: policy(0, 0)}, "over_limit"},
		{"surplus never offsets shortage", models.PpicUsageControlRow{SaleQty: 10, StocktakeShortageQty: 2, StocktakeSurplusQty: 100, HasStocktake: true, Policy: policy(0, 0)}, "over_limit"},
		{"incomplete within boundary", models.PpicUsageControlRow{SaleQty: 10, HasStocktake: true, Policy: policy(0, 0), Flags: []string{"pending_deductions"}}, "data_incomplete"},
		{"known breach remains visible", models.PpicUsageControlRow{SaleQty: 10, WasteQty: 1, Policy: policy(0, 0), Flags: []string{"pending_deductions"}}, "over_limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			finishPpicUsageRow(&tt.row)
			if tt.row.Status != tt.status {
				t.Fatalf("status %s want %s: %+v", tt.row.Status, tt.status, tt.row)
			}
			if tt.row.ReferenceQty == 0 && (tt.row.WastePct != nil || tt.row.UsageVariancePct != nil || tt.row.UsageLimitQty != nil || tt.row.WasteLimitQty != nil) {
				t.Fatal("zero denominator invented a ratio or limit")
			}
		})
	}
}

func TestPpicUsageControlScopePaginationExport(t *testing.T) {
	f := usageReportDB(t)
	for _, wh := range []string{f.own, f.other, f.central} {
		usageLedger(t, f, wh, 20)
	}
	// A malformed central warehouse with an outlet ID still must not leak.
	usageSQL(t, `UPDATE warehouses SET outlet_id=$1 WHERE id=$2`, f.outlet, f.central)
	for _, tt := range []struct {
		scope []string
		count int
	}{{nil, 3}, {[]string{f.outlet}, 1}, {[]string{}, 0}} {
		r, err := GetPpicUsageControl(usageDay, usageDay, "", "", "", tt.scope, 1, 1)
		if err != nil || r.TotalRows != tt.count || r.Summary.ItemsCount != tt.count || r.Summary.UnsetCount != tt.count {
			t.Fatalf("scope report: %+v %v", r, err)
		}
		if len(r.Rows) > 1 {
			t.Fatal("pagination ignored")
		}
	}
	hugePage, err := GetPpicUsageControl(usageDay, usageDay, "", "", "", nil, int(^uint(0)>>1), 200)
	if err != nil || len(hugePage.Rows) != 0 || hugePage.TotalRows != 3 {
		t.Fatalf("huge page: %+v %v", hugePage, err)
	}
	if _, err := GetPpicUsageControl(usageDay, usageDay, f.central, "", "", []string{f.outlet}, 1, 0); !errors.Is(err, ErrPpicUsageAccess) {
		t.Fatalf("central direct access: %v", err)
	}
	if _, err := ExportPpicUsageControl(usageDay, usageDay, f.other, "", "", []string{f.outlet}); !errors.Is(err, ErrPpicUsageAccess) {
		t.Fatalf("export direct access: %v", err)
	}
	report, err := GetPpicUsageControl(usageDay, usageDay, "", "Bahan A", "unset", nil, 2, 1)
	if err != nil || report.TotalRows != 3 || len(report.Rows) != 1 {
		t.Fatalf("page two: %+v %v", report, err)
	}
	for _, tt := range []struct {
		scope []string
		count int
	}{{nil, 3}, {[]string{f.outlet}, 1}, {[]string{}, 0}} {
		data, err := ExportPpicUsageControl(usageDay, usageDay, "", "Bahan A", "unset", tt.scope)
		if err != nil {
			t.Fatal(err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		lines, err := book.GetRows("Kontrol Pemakaian")
		book.Close()
		if err != nil {
			t.Fatal(err)
		}
		found, count := false, 0
		for _, line := range lines {
			if len(line) > 0 && line[0] == "Gudang" {
				found = true
				continue
			}
			if found && len(line) > 3 && line[3] == "Bahan A" {
				count++
				if len(line) > 24 && line[24] != "" {
					t.Fatalf("unset limit exported as value: %v", line[24])
				}
			}
		}
		if !found || count != tt.count {
			t.Fatalf("export rows %d want %d", count, tt.count)
		}
	}
}

func TestPpicUsageControlPaidWasteVoidQuarantineAndNonNetting(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 800)
	usagePolicy(t, f, 20, 50)
	sale := usagePayment(t, f, "paid", usageAt, usageAt)
	usageMovement(t, f, "sale", "transaction", sale, usageAt, -10)
	// Unpaid and titip orders with large quantities never enter the baseline.
	usageSQL(t, `INSERT INTO cloud_orders(id,local_id,outlet_id,items,is_holding) VALUES($1,'unpaid',$3,'[{"qty":900}]',false),($2,'titip',$3,'[{"qty":800}]',true)`, NewULID(), NewULID(), f.outlet)
	usagePayment(t, f, "titip", usageAt, usageAt)
	paidVoid := usagePayment(t, f, "void", usageAt, usageAt)
	voidMove := usageMovement(t, f, "waste", "transaction", paidVoid, usageAt, -2)
	usageWaste(t, f, "void", voidMove, usageAt, 2)
	usageSQL(t, `INSERT INTO pos_void_stock_events VALUES($1,$2,'void','order')`, NewULID(), f.outlet)
	normalWaste := usageWaste(t, f, "spoiled", "", usageAt, 1)
	usageMovement(t, f, "waste", "stock_waste", normalWaste, usageAt, -1)
	usageMovement(t, f, "quality_hold", "stock_quality", NewULID(), usageAt, -3)
	usageWaste(t, f, "quality_disposal", "", usageAt, 3) // no second stock movement
	usageMovement(t, f, "transfer_out", "stock_transfer", NewULID(), usageAt, -50)
	usageMovement(t, f, "return_out", "supplier_return", NewULID(), usageAt, -4)
	usageMovement(t, f, "adjustment", "stock_opname", NewULID(), usageAt, -2)
	usageMovement(t, f, "adjustment", "stock_opname", NewULID(), usageAt, 100)
	usageMovement(t, f, "adjustment", "set_stock", NewULID(), usageAt, -1)
	usageMovement(t, f, "adjustment", "set_stock", NewULID(), usageAt, 200)
	usageCount(t, f, usageAt)
	r := usageOnlyRow(t, f)
	if r.SaleQty != 10 || r.ReferenceQty != 10 || r.EvaluatedUsageQty != 13 || r.ExcessUsageQty != 3 || r.StocktakeSurplusQty != 100 || r.TransferOutQty != 50 || r.QualityHoldQty != 3 {
		t.Fatalf("consumption/netting: %+v", r)
	}
	if r.WasteQty != 6 || r.VoidWasteQty != 2 || r.WasteValue != 60 || r.ShortageValue != 30 || r.UndocumentedWasteQty != 0 || r.WastePct == nil || *r.WastePct != 60 {
		t.Fatalf("waste duplicate or missed QC: %+v", r)
	}
	if r.Status != "over_limit" || !r.HasStocktake || usageHasFlag(r, "sales_without_stock_basis") || usageHasFlag(r, "waste_document_missing") {
		t.Fatalf("status/count: %+v", r)
	}
	summary := usageReport(t, f).Summary
	if summary.WasteValue != 60 || summary.ShortageValue != 30 {
		t.Fatalf("financial summary counts sale/production again: %+v", summary)
	}
}

func TestPpicUsageControlPostedDateAndHistoricalSnapshots(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 50)
	usagePolicy(t, f, 0, 100)
	txn := usagePayment(t, f, "late-offline", "2026-09-01 05:00:00", usageAt)
	usageMovement(t, f, "sale", "transaction", txn, usageAt, -10)
	usageMovement(t, f, "sale", "transaction", txn, "2026-10-19 16:59:59", -99)
	usageMovement(t, f, "sale", "transaction", txn, "2026-10-19 17:00:00", -2)
	usageMovement(t, f, "sale", "transaction", txn, "2026-10-20 17:00:00", -77)
	usageCount(t, f, "2026-09-01 05:00:00")
	usageSQL(t, `INSERT INTO product_recipes VALUES($1,$2,$3,999)`, NewULID(), NewULID(), f.item)
	r := usageOnlyRow(t, f)
	if !strings.HasSuffix(r.StocktakeAt, "Z") || r.StocktakeAt != "2026-09-01T05:00:00Z" || r.Policy == nil || !strings.HasSuffix(r.Policy.UpdatedAt, "Z") {
		t.Fatalf("timestamps must carry UTC zone: %+v", r)
	}
	if r.SaleQty != 12 || r.ReferenceQty != 12 || r.HasStocktake || r.StocktakeAt == "" || r.Status != "needs_count" {
		t.Fatalf("posting timezone or old count: %+v", r)
	}
	usageSQL(t, `UPDATE product_recipes SET qty_base=1234`)
	after := usageOnlyRow(t, f)
	if after.ReferenceQty != r.ReferenceQty {
		t.Fatal("current recipe changed historical usage")
	}
	previous, err := GetPpicUsageControl("2026-09-01", "2026-09-01", f.own, "", "", nil, 1, 0)
	if err != nil || previous.Rows[0].SaleQty != 0 {
		t.Fatalf("offline sale counted by original date: %+v %v", previous, err)
	}
	// A later void changes the old sale classification but waste belongs to its
	// own event date, and is not counted again as an orphan movement.
	old := usageMovement(t, f, "waste", "transaction", txn, "2026-09-01 05:00:00", -3)
	usageWaste(t, f, "void", old, usageAt, 3)
	now := usageOnlyRow(t, f)
	if now.WasteQty != 3 || now.UndocumentedWasteQty != 0 {
		t.Fatalf("late void: %+v", now)
	}
}

func TestPpicUsageControlProductionSnapshotsAndMissingMovements(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 50)
	usagePolicy(t, f, 10, 10)
	usageCount(t, f, usageAt)
	wo := NewULID()
	usageSQL(t, `INSERT INTO work_orders VALUES($1,$2,'done',10,5,$3::timestamp)`, wo, f.own, usageAt)
	usageSQL(t, `INSERT INTO work_order_materials VALUES($1,$2,20,12)`, wo, f.item)
	usageMovement(t, f, "production_out", "work_order", wo, usageAt, -12)
	usageMovement(t, f, "production_out", "production", "", usageAt, -4)
	breakdown := NewULID()
	usageSQL(t, `INSERT INTO stock_breakdowns VALUES($1,$2,$3,3)`, breakdown, f.own, f.item)
	usageMovement(t, f, "breakdown_out", "stock_breakdown", breakdown, usageAt, -3)
	r := usageOnlyRow(t, f)
	if r.ProductionReferenceQty != 14 || r.ProductionQty != 16 || r.ReferenceQty != 17 || r.EvaluatedUsageQty != 19 || r.Status != "over_limit" {
		t.Fatalf("WO output scaling: %+v", r)
	}
	for _, flag := range []string{"production_plan_snapshot", "production_documented_basis", "breakdown_documented_basis"} {
		if !usageHasFlag(r, flag) {
			t.Fatalf("basis missing: %s %+v", flag, r)
		}
	}
	// The WO says material was used, but its movement is missing. The reference
	// must not turn the artificially low recorded actual into a green result.
	usageSQL(t, `DELETE FROM stock_movements WHERE ref_id=$1`, wo)
	r = usageOnlyRow(t, f)
	if r.Status != "data_incomplete" || !usageHasFlag(r, "production_movement_missing") {
		t.Fatalf("missing WO material movement looks healthy: %+v", r)
	}
}

func TestPpicUsageControlUnknownSaleAndUndocumentedWaste(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 10)
	usagePolicy(t, f, 100, 100)
	usageCount(t, f, usageAt)
	usageMovement(t, f, "sale", "transaction", NewULID(), usageAt, -4) // no payment
	usageMovement(t, f, "expired", "manual", NewULID(), usageAt, -2)
	usageWaste(t, f, "quality_disposal", "", usageAt, 3)
	r := usageOnlyRow(t, f)
	if r.ReferenceQty != 0 || r.SaleQty != 0 || r.OtherUsageQty != 4 || r.WasteQty != 5 || r.UndocumentedWasteQty != 2 || r.WasteValue != 50 || r.Status != "data_incomplete" {
		t.Fatalf("unknown consumption or orphan waste hidden: %+v", r)
	}
	if r.UsageLimitQty != nil || r.WasteLimitQty != nil || r.WastePct != nil {
		t.Fatalf("zero reference got invented ratios: %+v", r)
	}
}

func TestPpicUsageControlPendingAndMissingReceiptBasis(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 50)
	usagePolicy(t, f, 100, 100)
	usageCount(t, f, usageAt)
	txn := usagePayment(t, f, "paid", usageAt, usageAt)
	usageMovement(t, f, "sale", "transaction", txn, usageAt, -10)
	usagePayment(t, f, "missing", "2026-09-01 05:00:00", usageAt) // offline receipt without material snapshot
	for _, status := range []string{"pending", "resolved", "cancelled"} {
		usageSQL(t, `INSERT INTO stock_deduction_failures VALUES($1,$2,$3,$4::timestamp)`, NewULID(), f.outlet, status, usageAt)
	}
	extra := NewULID()
	usageSQL(t, `INSERT INTO warehouses(id,name,type,outlet_id) VALUES($1,'Second warehouse','outlet',$2)`, extra, f.outlet)
	usageLedger(t, f, extra, 20)
	r, err := GetPpicUsageControl(usageDay, usageDay, "", "", "", []string{f.outlet}, 1, 0)
	if err != nil || r.Summary.PendingDeductions != 1 || len(r.Rows) != 2 {
		t.Fatalf("pending duplicated per warehouse: %+v %v", r, err)
	}
	for _, row := range r.Rows {
		if row.Status != "data_incomplete" || !usageHasFlag(row, "pending_deductions") || !usageHasFlag(row, "sales_without_stock_basis") {
			t.Fatalf("missing basis hidden: %+v", row)
		}
	}
	// Search and status filters must apply identically to detail and export,
	// while the pending counter intentionally remains warehouse-wide.
	filtered, err := GetPpicUsageControl(usageDay, usageDay, "", "does not exist", "", []string{f.outlet}, 1, 0)
	if err != nil || filtered.TotalRows != 0 || filtered.Summary.ItemsCount != 0 || filtered.Summary.PendingDeductions != 1 {
		t.Fatalf("filtered summary: %+v %v", filtered, err)
	}
}

func TestPpicUsageControlZeroBasisPolicyOnlyAndValidation(t *testing.T) {
	f := usageReportDB(t)
	usagePolicy(t, f, 0, 0) // policy-only rows are visible without a ledger
	r := usageOnlyRow(t, f)
	if r.Status != "no_activity" {
		t.Fatalf("policy-only row: %+v", r)
	}
	usageWaste(t, f, "spoilage", "", usageAt, 5)
	r = usageOnlyRow(t, f)
	if r.Status != "no_basis" || r.WastePct != nil || r.WasteLimitQty != nil {
		t.Fatalf("waste without baseline: %+v", r)
	}
	for _, tt := range [][3]string{{"bad", usageDay, ""}, {usageDay, "2026-10-19", ""}, {"2025-01-01", usageDay, ""}, {usageDay, usageDay, "unknown"}} {
		if _, err := GetPpicUsageControl(tt[0], tt[1], "", "", tt[2], nil, 1, 0); err == nil {
			t.Fatal("invalid report input accepted", tt)
		}
	}
	empty, err := GetPpicUsageControl(usageDay, usageDay, "", "", "", []string{}, 1, 0)
	if err != nil || empty.Rows == nil || empty.TotalRows != 0 {
		t.Fatalf("empty scope: %+v %v", empty, err)
	}
	data, err := ExportPpicUsageControl(usageDay, usageDay, f.own, "", "no_basis", nil)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	lines, _ := book.GetRows("Kontrol Pemakaian")
	for _, line := range lines {
		if len(line) > 3 && line[3] == "Bahan A" {
			if line[24] != "" || line[25] != "" || line[26] != "" {
				t.Fatalf("zero baseline Excel invented limits: %v", line)
			}
			return
		}
	}
	t.Fatal("no_basis export omitted row", fmt.Sprint(strings.Join(empty.Notes, " ")))
}

func TestPpicUsageControlSuccessfulNonStockReceiptIsNotMissing(t *testing.T) {
	f := usageReportDB(t)
	usageLedger(t, f, f.own, 20)
	usagePolicy(t, f, 0, 0)
	usageCount(t, f, usageAt)
	sale := usagePayment(t, f, "ingredient-paid", usageAt, usageAt)
	usageMovement(t, f, "sale", "transaction", sale, usageAt, -10)
	nonStock := usagePayment(t, f, "non-stock-paid", usageAt, usageAt)
	// resolvePOSProduct can intentionally return no materials; its successful
	// receipt preserves that decision even if today's product recipe changes.
	usageSQL(t, `INSERT INTO pos_stock_deductions VALUES($1,$2,'P',1)`, f.outlet, nonStock)
	usageSQL(t, `INSERT INTO product_recipes VALUES($1,$2,$3,999)`, NewULID(), NewULID(), f.item)
	r := usageOnlyRow(t, f)
	if r.ReferenceQty != 10 || r.Status != "within_limit" || usageHasFlag(r, "sales_without_stock_basis") {
		t.Fatalf("successful no-material receipt flagged missing: %+v", r)
	}
	usageSQL(t, `DELETE FROM pos_stock_deductions WHERE transaction_id=$1`, nonStock)
	r = usageOnlyRow(t, f)
	if r.Status != "data_incomplete" || !usageHasFlag(r, "sales_without_stock_basis") {
		t.Fatalf("truly missing product coverage hidden: %+v", r)
	}
}

// This integration case uses the application's complete migrated schema and
// real payment/void writers, supplementing the isolated historical fixtures.
func TestPpicUsageControlPOSWorkflowSnapshots(t *testing.T) {
	f := newPOSVoidFixture(t)
	var today string
	if err := database.DB.QueryRow(`SELECT tz_today()::text`).Scan(&today); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveOrder(f.outlet, f.order); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTransaction(f.outlet, f.payment); err != nil {
		t.Fatal(err)
	}
	get := func() models.PpicUsageControlRow {
		t.Helper()
		report, err := GetPpicUsageControl(today, today, f.wh, "", "", nil, 1, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range report.Rows {
			if r.ItemID == f.itemID {
				return r
			}
		}
		t.Fatalf("missing material row: %+v", report)
		return models.PpicUsageControlRow{}
	}
	before := get()
	if before.SaleQty != 18 || before.WasteQty != 0 || usageHasFlag(before, "sales_without_stock_basis") {
		t.Fatalf("real payment snapshot: %+v", before)
	}
	usageSQL(t, `UPDATE recipe_items SET qty_base=90 WHERE recipe_master_id=$1`, f.product.RecipeMasterID)
	after := get()
	if after.ReferenceQty != 18 {
		t.Fatalf("current recipe rewrote real payment: %+v", after)
	}
	void := f.void()
	void.PaymentInfo.VoidedAt = time.Now().UTC().Format(time.RFC3339Nano)
	void.UpdatedAt = void.PaymentInfo.VoidedAt
	if _, err := SaveOrder(f.outlet, void); err != nil {
		t.Fatal(err)
	}
	after = get()
	if after.SaleQty != 0 || after.WasteQty != 18 || after.VoidWasteQty != 18 || after.WasteValue != 36 || after.UndocumentedWasteQty != 0 {
		t.Fatalf("real paid void double-counted or recomputed: %+v", after)
	}
}
