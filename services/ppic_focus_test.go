package services

import (
	"cloud-pos/database"
	"encoding/json"
	"strings"
	"testing"
)

func TestPpicFocusPermissionCatalog(t *testing.T) {
	for _, permission := range AllPermissions {
		for _, module := range []string{"dashboard", "planning", "forecast", "mrp", "production", "workorders", "reports"} {
			if strings.HasPrefix(permission, "ppic."+module+".") {
				t.Fatalf("retired permission remains assignable: %s", permission)
			}
		}
	}
	if err := UpdateRolePermissions("reader", []string{"ppic.forecast.manage"}); err == nil {
		t.Fatal("retired forecast permission accepted")
	}
}

func TestPpicFocusMigrationRedirectsWithoutGrantingAndKeepsHistory(t *testing.T) {
	f := usagePolicyDB(t)
	if _, err := SavePpicUsagePolicy(usagePolicyInput(f, 3, 1, 0), nil, "ppic"); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"demand_forecasts", "mrp_runs", "production_plans", "work_orders", "work_order_materials"} {
		exec(`CREATE TABLE ` + table + `(evidence TEXT)`)
		exec(`INSERT INTO ` + table + ` VALUES('historical snapshot')`)
	}
	for _, row := range []struct{ name, landing, permission string }{
		{"legacy-only", "/ppic/forecast", "ppic.forecast.manage"},
		{"legacy-control", "/ppic/dashboard", "ppic.control.update"},
		{"legacy-count", "/ppic/work-orders", "ppic.opname.approve"},
		{"legacy-expiry", "/ppic/reports?tab=hpp", "ppic.expiry.ack"},
		{"other-module", "/fnb/costing", "fnb.costing.view"},
	} {
		exec(`INSERT INTO roles(name,redirect_to) VALUES($1,$2)`, row.name, row.landing)
		exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2)`, row.name, row.permission)
		exec(`INSERT INTO role_permissions(role,permission) VALUES($1,'ppic.planning.update')`, row.name)
	}
	// Existing PPIC/admin revocations must survive this cleanup and restarts.
	exec(`DELETE FROM role_permissions WHERE role IN ('admin','ppic') AND permission IN ('ppic.control.update','ppic.opname.view','ppic.expiry.view')`)
	for i := 0; i < 2; i++ {
		if err := database.RunPpicFocusMigrations(); err != nil {
			t.Fatal(err)
		}
		if err := database.RunPpicUsageControlMigrations(); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{
		"legacy-only": "/403", "legacy-control": "/ppic/usage-control", "legacy-count": "/ppic/opname", "legacy-expiry": "/ppic/expiry", "other-module": "/fnb/costing",
	} {
		var got string
		if err := database.DB.QueryRow(`SELECT redirect_to FROM roles WHERE name=$1`, name).Scan(&got); err != nil || got != want {
			t.Fatalf("landing %s: %s want %s (%v)", name, got, want, err)
		}
	}
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE permission LIKE 'ppic.planning.%' OR permission LIKE 'ppic.forecast.%' OR permission='ppic.dashboard.view' OR (role='legacy-only') OR (role IN ('admin','ppic') AND permission IN ('ppic.control.update','ppic.opname.view','ppic.expiry.view'))`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("retired/restored/granted permissions: %d %v", count, err)
	}
	for _, table := range []string{"demand_forecasts", "mrp_runs", "production_plans", "work_orders", "work_order_materials"} {
		var evidence string
		if err := database.DB.QueryRow(`SELECT evidence FROM ` + table).Scan(&evidence); err != nil || evidence != "historical snapshot" {
			t.Fatalf("lost %s evidence: %s %v", table, evidence, err)
		}
	}
	history, err := ListPpicUsagePolicyHistory(f.item, f.own, nil)
	if err != nil || len(history) != 1 || history[0].Version != 1 || history[0].UsageTolerancePct != 3 {
		t.Fatalf("usage audit changed: %+v %v", history, err)
	}
}

func TestPpicFocusOnlyNewRoleGetsRetainedReaders(t *testing.T) {
	usagePolicyDB(t)
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role='ppic' AND permission IN ('ppic.opname.view','ppic.expiry.view')`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("new PPIC role retained readers: %d %v", count, err)
	}
	// A pre-existing PPIC role may have explicitly declined these permissions.
	if _, err := database.DB.Exec(`DELETE FROM app_settings WHERE key='mig_ppic_usage_control_permissions'; DELETE FROM role_permissions WHERE role='ppic' AND permission IN ('ppic.opname.view','ppic.expiry.view')`); err != nil {
		t.Fatal(err)
	}
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role='ppic' AND permission IN ('ppic.opname.view','ppic.expiry.view')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("existing role readers restored: %d %v", count, err)
	}
}

func TestPpicFocusMigratesOnlyExactBuiltinWATemplate(t *testing.T) {
	usagePolicyDB(t)
	const previous = `⚠️ *Peringatan Stok {date}*
Batch kedaluwarsa: *{expired_count}* · ≤3 hari: *{expiring_count}* · Di bawah ROP: *{rop_count}*

Segera kedaluwarsa:
{expiring_list}

Di bawah titik pesan ulang:
{rop_list}`
	for _, tc := range []struct {
		name, template string
		remove         bool
	}{
		{"old builtin", previous, true},
		{"custom text", previous + "\nHubungi pengawas gudang.", false},
		{"already current", waSpec("ppic.alert").Template, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved, _ := json.Marshal(map[string]string{"ppic.alert": tc.template, "sales.daily": "Rekap khusus {date}"})
			if _, err := database.DB.Exec(`INSERT INTO app_settings(key,value) VALUES('wa_templates',$1) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, string(saved)); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := database.RunPpicFocusMigrations(); err != nil {
					t.Fatal(err)
				}
			}
			var raw string
			if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key='wa_templates'`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			if got["sales.daily"] != "Rekap khusus {date}" {
				t.Fatal("another event's custom template changed")
			}
			if tc.remove {
				if _, exists := got["ppic.alert"]; exists {
					t.Fatal("old builtin still overrides new expiry default")
				}
			} else if got["ppic.alert"] != tc.template || raw != string(saved) {
				t.Fatal("custom/current template setting changed")
			}
		})
	}
	// Preserve malformed historical configuration rather than destroying it.
	if _, err := database.DB.Exec(`UPDATE app_settings SET value='legacy malformed json' WHERE key='wa_templates'`); err != nil {
		t.Fatal(err)
	}
	if err := database.RunPpicFocusMigrations(); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := database.DB.QueryRow(`SELECT value FROM app_settings WHERE key='wa_templates'`).Scan(&raw); err != nil || raw != "legacy malformed json" {
		t.Fatalf("malformed template was overwritten: %q %v", raw, err)
	}
}

func TestPpicFocusWATemplatesNeverClaimRetiredROPWasMeasured(t *testing.T) {
	vars := waPpicExpiryVars("2026-10-27", 2, 3, []string{"Bahan A"})
	const custom = "Batch expired: {expired_count}. Di bawah ROP: {rop_count}\n{rop_list}"
	for _, values := range []map[string]string{vars, waSampleVars(waSpec("ppic.alert"))} {
		body := waRender(custom, values)
		if strings.Count(body, "tidak dievaluasi (fitur ROP dinonaktifkan)") != 2 || strings.Contains(body, "Di bawah ROP: 0") || strings.Contains(body, "{rop_") {
			t.Fatalf("retired ROP misleading in custom template: %s", body)
		}
	}
	body := waRender(waSpec("ppic.alert").Template, vars)
	if strings.Contains(body, "ROP") || !strings.Contains(body, "Batch kedaluwarsa: *2*") || !strings.Contains(body, "Bahan A") {
		t.Fatalf("builtin should only describe expiry: %s", body)
	}
}
