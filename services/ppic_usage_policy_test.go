package services

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"
)

type usagePolicyFixture struct{ item, inactiveItem, own, other, central, inactive, outlet, otherOutlet string }

func usagePolicyDB(t *testing.T) usagePolicyFixture {
	t.Helper()
	requireDB(t)
	original := database.DB
	schema := fmt.Sprintf("ppic_policy_%d", time.Now().UnixNano())
	if _, err := original.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", config.Load().DSN()+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(8)
	database.DB = db
	t.Cleanup(func() {
		database.DB = original
		db.Close()
		if _, err := original.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	for _, q := range []string{
		`CREATE TABLE outlets(id CHAR(26) PRIMARY KEY,name TEXT)`,
		`CREATE TABLE warehouses(id CHAR(26) PRIMARY KEY,name TEXT,type TEXT,outlet_id CHAR(26),is_active BOOL NOT NULL DEFAULT true)`,
		`CREATE TABLE stock_items(id CHAR(26) PRIMARY KEY,code TEXT,name TEXT,category TEXT,base_unit TEXT,is_active BOOL NOT NULL DEFAULT true)`,
		`CREATE TABLE roles(name TEXT PRIMARY KEY,description TEXT,is_system BOOL,scope_type TEXT,redirect_to TEXT)`,
		`CREATE TABLE role_permissions(role TEXT,permission TEXT,UNIQUE(role,permission))`,
		`CREATE TABLE app_settings(key TEXT PRIMARY KEY,value TEXT)`,
		`INSERT INTO roles(name) VALUES('admin'),('superadmin'),('reader'),('gudang'),('fnb')`,
		`INSERT INTO role_permissions(role,permission) VALUES('reader','ppic.dashboard.view'),('gudang','stockitems.update'),('fnb','recipes.update')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	f := usagePolicyFixture{NewULID(), NewULID(), NewULID(), NewULID(), NewULID(), NewULID(), NewULID(), NewULID()}
	if _, err := db.Exec(`INSERT INTO outlets(id,name) VALUES($1,'Outlet A'),($2,'Outlet B')`, f.outlet, f.otherOutlet); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO warehouses(id,name,type,outlet_id,is_active) VALUES($1,'Outlet A','outlet',$5,true),($2,'Outlet B','outlet',$6,true),($3,'Induk','central',NULL,true),($4,'Nonaktif','outlet',$5,false)`, f.own, f.other, f.central, f.inactive, f.outlet, f.otherOutlet); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_items(id,code,name,category,base_unit,is_active) VALUES($1,'BA','Bahan A','Bahan','g',true),($2,'BN','Bahan nonaktif','Bahan','g',false)`, f.item, f.inactiveItem); err != nil {
		t.Fatal(err)
	}
	return f
}

func usagePolicyInput(f usagePolicyFixture, usage, waste float64, version int) models.PpicUsagePolicyInput {
	return models.PpicUsagePolicyInput{ItemID: f.item, WarehouseID: f.own, UsageTolerancePct: &usage, WasteTolerancePct: &waste, Version: version, Reason: " Standar PPIC hasil evaluasi "}
}

func TestPpicUsagePolicyExplicitLimitsHistoryAndConcurrentVersion(t *testing.T) {
	f := usagePolicyDB(t)
	history, err := ListPpicUsagePolicyHistory(f.item, f.own, []string{f.outlet})
	if err != nil || len(history) != 0 {
		t.Fatalf("no invented defaults: %+v %v", history, err)
	}
	p, err := SavePpicUsagePolicy(usagePolicyInput(f, 0, 0, 0), []string{f.outlet}, "ppic-a")
	if err != nil || p.Version != 1 || p.UsageTolerancePct != 0 || p.WasteTolerancePct != 0 || p.UpdatedBy != "ppic-a" || p.Reason != "Standar PPIC hasil evaluasi" || p.UpdatedAt == "" {
		t.Fatalf("explicit zero/save actor: %+v %v", p, err)
	}
	if _, err := SavePpicUsagePolicy(usagePolicyInput(f, 8, 2, 0), nil, "ppic-b"); !errors.Is(err, ErrPpicUsageConflict) {
		t.Fatalf("stale create: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := SavePpicUsagePolicy(usagePolicyInput(f, float64(2+i), 1, 1), nil, fmt.Sprintf("ppic-%d", i))
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrPpicUsageConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("concurrent saves: success=%d conflict=%d", ok, conflict)
	}
	history, err = ListPpicUsagePolicyHistory(f.item, f.own, nil)
	if err != nil || len(history) != 2 || history[0].Version != 2 || history[1].Version != 1 || history[1].UsageTolerancePct != 0 {
		t.Fatalf("history must preserve prior limit: %+v %v", history, err)
	}
	if _, err := database.DB.Exec(`UPDATE ppic_usage_policy_history SET reason='rewrite'`); err == nil {
		t.Fatal("history UPDATE was accepted")
	}
	if _, err := database.DB.Exec(`DELETE FROM ppic_usage_policy_history`); err == nil {
		t.Fatal("history DELETE was accepted")
	}
}

func TestPpicUsagePolicyScopeLookupAndValidation(t *testing.T) {
	f := usagePolicyDB(t)
	own := []string{f.outlet}
	for _, test := range []struct {
		wh    string
		scope []string
		want  error
	}{
		{f.own, own, nil}, {f.other, own, ErrPpicUsageAccess}, {f.central, own, ErrPpicUsageAccess},
		{f.own, []string{}, ErrPpicUsageAccess}, {f.inactive, nil, ErrPpicUsageNotFound}, {NewULID(), nil, ErrPpicUsageNotFound},
	} {
		if err := ValidatePpicUsageWarehouse(test.wh, test.scope); !errors.Is(err, test.want) {
			t.Fatalf("warehouse %s: %v want %v", test.wh, err, test.want)
		}
	}
	for _, test := range []struct {
		scope []string
		count int
	}{{nil, 3}, {own, 1}, {[]string{}, 0}} {
		look, err := GetPpicUsageLookups("", test.scope)
		if err != nil || len(look.Warehouses) != test.count {
			t.Fatalf("lookup: %+v %v", look, err)
		}
		if (test.count > 0 && len(look.Items) != 1) || (test.count == 0 && len(look.Items) != 0) {
			t.Fatalf("catalog scope/active: %+v", look)
		}
	}
	if _, err := GetPpicUsageLookups(f.other, own); !errors.Is(err, ErrPpicUsageAccess) {
		t.Fatalf("lookup outside scope: %v", err)
	}
	for _, wh := range []string{f.other, f.central} {
		req := usagePolicyInput(f, 2, 1, 0)
		req.WarehouseID = wh
		if _, err := SavePpicUsagePolicy(req, own, "ppic"); !errors.Is(err, ErrPpicUsageAccess) {
			t.Fatalf("write outside scope: %v", err)
		}
		if _, err := ListPpicUsagePolicyHistory(f.item, wh, own); !errors.Is(err, ErrPpicUsageAccess) {
			t.Fatalf("history outside scope: %v", err)
		}
	}
	for _, item := range []string{f.inactiveItem, NewULID()} {
		req := usagePolicyInput(f, 2, 1, 0)
		req.ItemID = item
		if _, err := SavePpicUsagePolicy(req, nil, "ppic"); !errors.Is(err, ErrPpicUsageNotFound) {
			t.Fatalf("unknown/inactive item: %v", err)
		}
	}
	bad := []models.PpicUsagePolicyInput{}
	for _, v := range []float64{-1, 101, math.NaN(), math.Inf(1), math.Inf(-1)} {
		bad = append(bad, usagePolicyInput(f, v, 1, 0), usagePolicyInput(f, 1, v, 0))
	}
	for _, which := range []string{"usage", "waste", "reason", "version"} {
		req := usagePolicyInput(f, 2, 1, 0)
		switch which {
		case "usage":
			req.UsageTolerancePct = nil
		case "waste":
			req.WasteTolerancePct = nil
		case "reason":
			req.Reason = "  "
		case "version":
			req.Version = -1
		}
		bad = append(bad, req)
	}
	for _, req := range bad {
		_, err := SavePpicUsagePolicy(req, nil, "ppic")
		var validation ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("invalid policy accepted: %+v %v", req, err)
		}
	}
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM ppic_usage_policies`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rejected requests left policies: %d %v", n, err)
	}
}

func TestPpicUsagePolicyMigrationKeepsLimitsHistoryAndRevocations(t *testing.T) {
	f := usagePolicyDB(t)
	if _, err := SavePpicUsagePolicy(usagePolicyInput(f, 2.5, 0, 0), nil, "ppic"); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"admin", "superadmin", "ppic"} {
		var n int
		if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role=$1 AND permission LIKE 'ppic.control.%'`, role).Scan(&n); err != nil || n != 3 {
			t.Fatalf("initial permissions %s: %d %v", role, n, err)
		}
	}
	var docs int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role='ppic' AND permission IN ('documents.view','documents.download')`).Scan(&docs); err != nil || docs != 2 {
		t.Fatalf("new PPIC role shared documents: %d %v", docs, err)
	}
	if _, err := database.DB.Exec(`DELETE FROM role_permissions WHERE role='ppic' AND permission IN ('ppic.control.update','documents.view')`); err != nil {
		t.Fatal(err)
	}
	before, err := ListPpicUsagePolicyHistory(f.item, f.own, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	after, err := ListPpicUsagePolicyHistory(f.item, f.own, nil)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("migration changed history: %+v %v", after, err)
	}
	var n int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE (role='ppic' AND permission IN ('ppic.control.update','documents.view')) OR (role IN ('reader','gudang','fnb') AND permission LIKE 'ppic.control.%')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("restored revocation/granted unrelated roles: %d %v", n, err)
	}
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM ppic_usage_policies`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("default policies added: %d %v", n, err)
	}
}
