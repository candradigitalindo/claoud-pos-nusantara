package handlers

import (
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/middleware"
	"cloud-pos/models"
	"cloud-pos/services"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestPpicUsageHandlerRejectsInvalidFiltersAndMasksInternalError(t *testing.T) {
	app := fiber.New()
	app.Get("/report", GetPpicUsageControl)
	app.Get("/export", ExportPpicUsageControl)
	app.Get("/error", func(c *fiber.Ctx) error { return ppicUsageError(c, errors.New("private database error")) })
	for _, url := range []string{"/report?date_from=not-a-date", "/report?date_from=2026-02-30", "/report?date_from=2026-10-20&date_to=2026-10-19", "/report?status=leak", "/export?status=unknown"} {
		if code, _ := doJSON(t, app, "GET", url, nil); code != 400 {
			t.Fatalf("invalid %s: %d", url, code)
		}
	}
	if code, res := doJSON(t, app, "GET", "/error", nil); code != 500 || strings.Contains(res.Error, "private") {
		t.Fatalf("internal error exposed: %d %+v", code, res)
	}
}

func TestPpicUsageHandlersPermissionsScopeActorAndConflict(t *testing.T) {
	requireHandlerDB(t)
	original := database.DB
	schema := fmt.Sprintf("ppic_policy_handlers_%d", time.Now().UnixNano())
	if _, err := original.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", config.Load().DSN()+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
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
		`INSERT INTO roles(name) VALUES('admin'),('reader'),('editor'),('warehouse'),('counter'),('expiry')`,
		`INSERT INTO role_permissions(role,permission) VALUES('reader','ppic.control.view'),('editor','ppic.control.update'),('warehouse','stockitems.update'),('counter','ppic.opname.create'),('expiry','ppic.expiry.view')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	own, other, central, item, outlet, otherOutlet := services.NewULID(), services.NewULID(), services.NewULID(), services.NewULID(), services.NewULID(), services.NewULID()
	if _, err := db.Exec(`INSERT INTO outlets(id,name) VALUES($1,'A'),($2,'B')`, outlet, otherOutlet); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO warehouses(id,name,type,outlet_id) VALUES($1,'A','outlet',$4),($2,'B','outlet',$5),($3,'Pusat','central',NULL)`, own, other, central, outlet, otherOutlet); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock_items(id,code,name,category,base_unit) VALUES($1,'BA','Bahan A','Bahan','g')`, item); err != nil {
		t.Fatal(err)
	}
	newApp := func(role string, scope []string) *fiber.App {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("admin_role", role)
			c.Locals("admin_username", "ppic-session")
			c.Locals("admin_scope", "all")
			if scope != nil {
				c.Locals("admin_scope", "specific")
				c.Locals("admin_outlet_ids", scope)
			}
			return c.Next()
		})
		app.Get("/lookups", middleware.RequirePermission("ppic.control.view"), GetPpicUsageLookups)
		app.Get("/shared-lookups", middleware.RequireAnyPermission("ppic.control.view", "ppic.opname.view", "ppic.expiry.view"), GetPpicUsageLookups)
		app.Get("/history", middleware.RequirePermission("ppic.control.view"), GetPpicUsagePolicyHistory)
		app.Put("/policy", middleware.RequirePermission("ppic.control.update"), SavePpicUsagePolicy)
		app.Get("/export", middleware.RequirePermission("ppic.control.export"), ExportPpicUsageControl)
		return app
	}
	body := map[string]interface{}{"item_id": item, "warehouse_id": own, "usage_tolerance_pct": 0, "waste_tolerance_pct": 2, "version": 0, "reason": "hasil evaluasi", "updated_by": "forged"}
	reader := newApp("reader", []string{outlet})
	if code, res := doJSON(t, reader, "GET", "/lookups", nil); code != 200 {
		t.Fatalf("PPIC-only lookup: %d %s", code, res.Error)
	}
	if code, _ := doJSON(t, reader, "PUT", "/policy", body); code != 403 {
		t.Fatalf("reader mutated policy: %d", code)
	}
	if code, _ := doJSON(t, reader, "GET", "/export", nil); code != 403 {
		t.Fatalf("reader exported without permission: %d", code)
	}
	warehouse := newApp("warehouse", nil)
	if code, _ := doJSON(t, warehouse, "GET", "/lookups", nil); code != 403 {
		t.Fatalf("warehouse received PPIC permission: %d", code)
	}
	if code, _ := doJSON(t, warehouse, "GET", "/shared-lookups", nil); code != 403 {
		t.Fatalf("warehouse received retained PPIC lookup permission: %d", code)
	}
	for _, role := range []string{"counter", "expiry"} {
		app := newApp(role, []string{outlet})
		code, res := doJSON(t, app, "GET", "/shared-lookups", nil)
		var lookups models.PpicUsageLookups
		decodeData(t, res, &lookups)
		if code != 200 || len(lookups.Warehouses) != 1 || lookups.Warehouses[0].ID != own || len(lookups.Items) != 1 {
			t.Fatalf("retained %s lookup without Gudang: %d %+v", role, code, lookups)
		}
		if code, _ := doJSON(t, app, "GET", "/lookups", nil); code != 403 {
			t.Fatalf("retained %s granted control access: %d", role, code)
		}
	}
	editor := newApp("editor", []string{outlet})
	if code, res := doJSON(t, editor, "PUT", "/policy", body); code != 200 {
		t.Fatalf("save: %d %s", code, res.Error)
	}
	if code, _ := doJSON(t, editor, "PUT", "/policy", body); code != 409 {
		t.Fatalf("stale create: %d", code)
	}
	code, res := doJSON(t, reader, "GET", "/history?item_id="+item+"&warehouse_id="+own, nil)
	var history []models.PpicUsagePolicy
	decodeData(t, res, &history)
	if code != 200 || len(history) != 1 || history[0].UpdatedBy != "ppic-session" || history[0].UsageTolerancePct != 0 {
		t.Fatalf("history actor/zero: %d %+v", code, history)
	}
	for _, wh := range []string{other, central} {
		body["warehouse_id"] = wh
		if code, _ := doJSON(t, editor, "PUT", "/policy", body); code != 403 {
			t.Fatalf("write scope: %d", code)
		}
		if code, _ := doJSON(t, reader, "GET", "/history?item_id="+item+"&warehouse_id="+wh, nil); code != 403 {
			t.Fatalf("history scope: %d", code)
		}
		if code, _ := doJSON(t, reader, "GET", "/lookups?warehouse_id="+wh, nil); code != 403 {
			t.Fatalf("lookup scope: %d", code)
		}
	}
	body["warehouse_id"] = own
	empty := newApp("editor", []string{})
	if code, _ := doJSON(t, empty, "PUT", "/policy", body); code != 403 {
		t.Fatalf("empty scope write: %d", code)
	}
	code, res = doJSON(t, newApp("reader", []string{}), "GET", "/lookups", nil)
	var lookups models.PpicUsageLookups
	decodeData(t, res, &lookups)
	if code != 200 || len(lookups.Warehouses) != 0 || len(lookups.Items) != 0 {
		t.Fatalf("empty scope lookup: %d %+v", code, lookups)
	}
	body["warehouse_id"] = services.NewULID()
	if code, _ := doJSON(t, newApp("editor", nil), "PUT", "/policy", body); code != 404 {
		t.Fatalf("missing warehouse: %d", code)
	}
	body["warehouse_id"] = own
	delete(body, "usage_tolerance_pct")
	if code, _ := doJSON(t, editor, "PUT", "/policy", body); code != 400 {
		t.Fatalf("omitted limit accepted: %d", code)
	}
}
