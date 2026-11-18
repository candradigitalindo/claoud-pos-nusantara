package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWAEventCatalogExplainsDataScope(t *testing.T) {
	for _, event := range WAEventCatalog(models.WASettings{}) {
		if event.ScopeKind != "outlet" && event.ScopeKind != "global" && event.ScopeKind != "mixed" {
			t.Errorf("%s: unknown scope %q", event.Key, event.ScopeKind)
		}
		if strings.TrimSpace(event.ScopeDescription) == "" {
			t.Errorf("%s: scope explanation is empty", event.Key)
		}
		wantDigest := event.Key == "sales.daily" || event.Key == "ppic.alert" || event.Key == "asset.maintenance_due"
		if event.IsDigest != wantDigest {
			t.Errorf("%s: is_digest = %v, want %v", event.Key, event.IsDigest, wantDigest)
		}
	}
	for _, key := range []string{"sales.daily", "ppic.alert", "asset.maintenance_due", "purchase.approved"} {
		if kind, _ := waEventScope(key); kind != "mixed" {
			t.Errorf("%s scope = %s, want mixed", key, kind)
		}
	}
	for _, key := range []string{"reservation.created", "customer.reservation_created", "cashier_shift.closed", "device.offline"} {
		if kind, _ := waEventScope(key); kind != "outlet" {
			t.Errorf("%s scope = %s, want outlet", key, kind)
		}
	}
}

func TestWADailySalesPayloadCountsOnlyRecipientOutlets(t *testing.T) {
	rows := []waOutletSales{
		{ID: "a", Name: "Outlet Sendiri", Revenue: 100000, Count: 2, UnpaidCount: 1, UnpaidAmount: 30000, VoidCount: 1},
		{ID: "b", Name: "Outlet Lain", Revenue: 900000, Count: 9, UnpaidCount: 4, UnpaidAmount: 80000, VoidCount: 3},
	}
	for _, scope := range [][]string{{"a"}, {"a", "a"}} {
		vars, ok := waDailySalesVars("2026-11-10", rows, scope)
		if !ok || vars["total_revenue"] != waRp(100000) || vars["total_count"] != "2" || vars["avg"] != waRp(50000) || vars["unpaid_count"] != "1" || vars["unpaid_amount"] != waRp(30000) || vars["void_count"] != "1" || strings.Contains(vars["outlets"], "Outlet Lain") {
			t.Fatalf("own digest contains wrong totals or foreign data: %+v", vars)
		}
	}
	manualVars, ok := waDailySalesVars("2026-11-10", rows, []string{"b"})
	if !ok || manualVars["total_count"] != "9" || strings.Contains(manualVars["outlets"], "Outlet Sendiri") {
		t.Errorf("manual outlet filter ignored: %+v", manualVars)
	}
	global, ok := waDailySalesVars("2026-11-10", rows, nil)
	if !ok || global["total_revenue"] != waRp(1000000) || global["total_count"] != "11" || global["unpaid_count"] != "5" {
		t.Errorf("global digest incorrect: %+v", global)
	}
	if _, ok := waDailySalesVars("2026-11-10", rows, []string{"__none__"}); ok {
		t.Error("empty effective scope receives sales digest")
	}
}

func TestWAPpicPayloadScopesCountsAndExcludesCentralInventory(t *testing.T) {
	groups := []waExpiryAlertGroup{
		{outletID: "a", expired: 12, expiring: 2, lines: []waExpiryAlertLine{{item: "Bahan Sendiri", warehouse: "Gudang Sendiri", expiry: "2026-11-09", qty: 5, unit: "kg"}}},
		{outletID: "b", expired: 20, expiring: 3, lines: []waExpiryAlertLine{{item: "Bahan Lain", warehouse: "Gudang Lain", expiry: "2026-11-08"}}},
		{expired: 30, expiring: 4, lines: []waExpiryAlertLine{{item: "Bahan Pusat", warehouse: "Gudang Induk", expiry: "2026-11-07"}}},
	}
	vars, ok := waPpicScopedVars("2026-11-10", groups, []string{"a"})
	if !ok || vars["expired_count"] != "12" || vars["expiring_count"] != "2" || !strings.Contains(vars["expiring_list"], "Bahan Sendiri") || strings.Contains(vars["expiring_list"], "Bahan Lain") || strings.Contains(vars["expiring_list"], "Pusat") {
		t.Fatalf("scoped PPIC digest contains foreign/global data: %+v", vars)
	}
	global, ok := waPpicScopedVars("2026-11-10", groups, nil)
	if !ok || global["expired_count"] != "62" || global["expiring_count"] != "9" || !strings.Contains(global["expiring_list"], "Bahan Pusat") {
		t.Errorf("global PPIC digest incomplete: %+v", global)
	}
	if _, ok := waPpicScopedVars("2026-11-10", groups, []string{"__none__"}); ok {
		t.Error("empty effective scope receives PPIC digest")
	}
	if waDigestCoversOutlet([]string{"a", ""}, "") {
		t.Error("outlet-filtered recipient can see central inventory")
	}
}

func TestWAMaintenancePayloadScopesCountAndDetails(t *testing.T) {
	rows := []WAMaintenanceDue{
		{WONumber: "WO-A", AssetName: "Aset Sendiri", OutletID: "a", DueDate: "2026-11-10"},
		{WONumber: "WO-B", AssetName: "Aset Lain", OutletID: "b", DueDate: "2026-11-10"},
		{WONumber: "WO-P", AssetName: "Aset Pusat", DueDate: "2026-11-10"},
	}
	for _, tc := range []struct {
		scope                     []string
		count, contains, excludes string
	}{
		{[]string{"a"}, "1", "Aset Sendiri", "Aset Lain"},
		{[]string{"b"}, "1", "Aset Lain", "Aset Sendiri"},
		{nil, "3", "Aset Pusat", ""},
	} {
		vars, ok := waMaintenanceVars("2026-11-10", rows, tc.scope)
		if !ok || vars["count"] != tc.count || !strings.Contains(vars["list"], tc.contains) || (tc.excludes != "" && strings.Contains(vars["list"], tc.excludes)) || (len(tc.scope) > 0 && strings.Contains(vars["list"], "Aset Pusat")) {
			t.Errorf("maintenance scope %v leaked count/details: %+v", tc.scope, vars)
		}
	}
	if _, ok := waMaintenanceVars("2026-11-10", rows, []string{"__none__"}); ok {
		t.Error("empty effective scope receives maintenance digest")
	}
}

func TestWAPurchaseScopeDistinguishesWorkUnitFromCentral(t *testing.T) {
	unit, blank, outlet := "unit-pusat", " ", "outlet-a"
	for _, request := range []*models.PurchaseRequest{
		{}, {OutletID: &blank}, {OutletID: &blank, WorkUnitID: &blank},
	} {
		if got := waPurchaseOutlet(request); got != waCentralOutlet {
			t.Errorf("central purchase scope = %q", got)
		}
	}
	for _, request := range []*models.PurchaseRequest{
		{WorkUnitID: &unit}, {OutletID: &blank, WorkUnitID: &unit},
	} {
		if got := waPurchaseOutlet(request); got != waWorkUnitPrefix+unit {
			t.Errorf("work-unit purchase scope = %q, want unit marker", got)
		}
	}
	if got := waPurchaseOutlet(&models.PurchaseRequest{OutletID: &outlet, WorkUnitID: &unit}); got != outlet {
		t.Errorf("outlet purchase scope = %q", got)
	}
}

func TestWAPpicAlertSQLCountsBeforeLimitingRecipientDetails(t *testing.T) {
	requireDB(t)
	outletA, outletB := recvOutlet(t), recvOutlet(t)
	warehouseA, warehouseB, central := recvWarehouse(t), recvWarehouse(t), recvWarehouse(t)
	for _, row := range []struct{ warehouse, outlet string }{{warehouseA, outletA}, {warehouseB, outletB}} {
		if _, err := database.DB.Exec(`UPDATE warehouses SET type='outlet', outlet_id=$1 WHERE id=$2`, row.outlet, row.warehouse); err != nil {
			t.Fatal(err)
		}
	}
	name := "Bahan Scope WA " + NewULID()
	item := recvStockItem(t, name)
	batch := func(warehouse string, days int, ack bool) {
		t.Helper()
		if _, err := database.DB.Exec(`INSERT INTO stock_batches (id, warehouse_id, item_id, qty_base, cost_per_base, expiry_date, ppic_ack_at)
			VALUES ($1,$2,$3,5,100,CURRENT_DATE + $4::int,CASE WHEN $5 THEN NOW() ELSE NULL END)`, NewULID(), warehouse, item, days, ack); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 12; i++ {
		batch(warehouseA, -1, false)
	}
	batch(warehouseA, 1, false)
	batch(warehouseA, -1, true) // acknowledged alerts must not reappear
	batch(warehouseB, -1, false)
	batch(central, -1, false)
	groups, err := waPpicAlertGroups()
	if err != nil {
		t.Fatal(err)
	}
	vars, ok := waPpicScopedVars("2026-11-10", groups, []string{outletA})
	if !ok || vars["expired_count"] != "12" || vars["expiring_count"] != "1" || strings.Count(vars["expiring_list"], "• ") != 10 {
		t.Fatalf("query counts were capped or scoped incorrectly: %+v", vars)
	}
	foreign, ok := waPpicScopedVars("2026-11-10", groups, []string{outletB})
	if !ok || foreign["expired_count"] != "1" || foreign["expiring_count"] != "0" {
		t.Fatalf("foreign scope inherited global totals: %+v", foreign)
	}
}

func TestWAScopedDigestQueuesOnlyManualRecipientScopeAndDeduplicates(t *testing.T) {
	requireDB(t)
	oldURL := waGatewayURL
	waGatewayURL = "http://notification-test.invalid" // no worker is started; enqueue only
	t.Cleanup(func() { waGatewayURL = oldURL })
	settings := map[string]string{"wa_enabled": "true", "wa_events": `{}`, "wa_event_roles": `{}`, "wa_role_notifications": `{}`,
		"wa_templates": `{"sales.daily":"{outlets}\nTotal: {total_revenue}"}`}
	for key, value := range settings {
		key := key
		old, err := GetSetting(key)
		if err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		existed := err == nil
		t.Cleanup(func() {
			if existed {
				UpdateSettings(map[string]string{key: old})
			} else {
				execScratch(`DELETE FROM app_settings WHERE key=$1`, key)
			}
		})
		if err := UpdateSettings(map[string]string{key: value}); err != nil {
			t.Fatal(err)
		}
	}
	phone := "62813" + fmt.Sprintf("%010d", time.Now().UnixNano()%10000000000)
	recipient, err := CreateWARecipient(models.WARecipient{Name: "Penerima scoped uji " + NewULID(), Target: phone, IsActive: true,
		Events: []string{"sales.daily"}, OutletIDs: []string{"outlet-a"}})
	if err != nil {
		t.Fatal(err)
	}
	ref := "digest-scope-" + NewULID()
	t.Cleanup(func() {
		execScratch(`DELETE FROM wa_messages WHERE event='sales.daily' AND ref_id=$1`, ref)
		execScratch(`DELETE FROM wa_recipients WHERE id=$1`, recipient.ID)
	})
	rows := []waOutletSales{{ID: "outlet-a", Name: "Outlet Sendiri", Count: 2, Revenue: 20000}, {ID: "outlet-b", Name: "Outlet Rahasia", Count: 20, Revenue: 900000}}
	for i := 0; i < 2; i++ {
		notifyWAScopedDigest("sales.daily", ref, func(scope []string) (map[string]string, bool) { return waDailySalesVars("2026-11-10", rows, scope) })
	}
	var count int
	var body string
	if err := database.DB.QueryRow(`SELECT COUNT(*), COALESCE(MAX(body),'') FROM wa_messages WHERE event='sales.daily' AND ref_id=$1 AND target=$2`, ref, recipient.Target).Scan(&count, &body); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(body, "Outlet Sendiri") || strings.Contains(body, "Outlet Rahasia") || strings.Contains(body, waRp(920000)) {
		t.Fatalf("manual digest leaked global data or duplicated: count=%d body=%s", count, body)
	}
}
