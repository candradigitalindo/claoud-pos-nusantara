package services

import (
	"cloud-pos/database"
	"testing"
)

func ppicCaseSQL(t *testing.T, query string, args ...interface{}) {
	t.Helper()
	if _, err := database.DB.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func ppicCasePermissionCount(t *testing.T, role, pattern string) int {
	t.Helper()
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role=$1 AND permission LIKE $2`, role, pattern).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPpicUsageRoleCaseInitialSeedUsesExistingName(t *testing.T) {
	for _, role := range []string{"PPIC", "Ppic"} {
		t.Run(role, func(t *testing.T) {
			usagePolicyDB(t)
			// Reproduce a database before the first usage-control migration.
			ppicCaseSQL(t, `DELETE FROM role_permissions WHERE role='ppic'`)
			ppicCaseSQL(t, `DELETE FROM roles WHERE name='ppic'`)
			ppicCaseSQL(t, `DELETE FROM app_settings WHERE key IN ('mig_ppic_usage_control_permissions','mig_ppic_usage_control_role_case')`)
			ppicCaseSQL(t, `INSERT INTO roles(name,description,is_system,scope_type,redirect_to) VALUES($1,'PPIC operasional lama',false,'specific','/ppic/dashboard')`, role)
			ppicCaseSQL(t, `INSERT INTO role_permissions(role,permission) VALUES($1,'ppic.opname.approve'),($1,'documents.view'),($1,'procurement.rawmaterials.submit')`, role)
			if err := database.RunPpicUsageControlMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := database.RunPpicFocusMigrations(); err != nil {
				t.Fatal(err)
			}
			var names int
			if err := database.DB.QueryRow(`SELECT COUNT(*) FROM roles WHERE lower(name)='ppic'`).Scan(&names); err != nil || names != 1 {
				t.Fatalf("created duplicate role: %d %v", names, err)
			}
			if got := ppicCasePermissionCount(t, role, "ppic.control.%"); got != 3 {
				t.Fatalf("existing %s control permissions: %d", role, got)
			}
			if got := ppicCasePermissionCount(t, "ppic", "%"); got != 0 {
				t.Fatalf("orphan lowercase grants: %d", got)
			}
			if got := ppicCasePermissionCount(t, role, "%"); got != 6 {
				t.Fatalf("existing rights expanded beyond control: %d", got)
			}
			var description, scope, redirect string
			if err := database.DB.QueryRow(`SELECT description,scope_type,redirect_to FROM roles WHERE name=$1`, role).Scan(&description, &scope, &redirect); err != nil || description != "PPIC operasional lama" || scope != "specific" || redirect != "/ppic/usage-control" {
				t.Fatalf("role identity/scope/landing: %s %s %s %v", description, scope, redirect, err)
			}
			ppicCaseSQL(t, `DELETE FROM role_permissions WHERE role=$1 AND permission='ppic.control.update'`, role)
			if err := database.RunPpicUsageControlMigrations(); err != nil {
				t.Fatal(err)
			}
			if got := ppicCasePermissionCount(t, role, "ppic.control.update"); got != 0 {
				t.Fatal("restart restored explicit case-variant revocation")
			}
		})
	}
}

func TestPpicUsageRoleCaseRepairIsNarrowAndOneTime(t *testing.T) {
	f := usagePolicyDB(t)
	// The original marker and generated lowercase role already exist.
	ppicCaseSQL(t, `DELETE FROM app_settings WHERE key='mig_ppic_usage_control_role_case'`)
	ppicCaseSQL(t, `INSERT INTO roles(name,description,is_system,scope_type,redirect_to) VALUES
		('PPIC','Pengawas bahan baku',false,'specific','/ppic/opname'),
		('Ppic','Akses sengaja terbatas',false,'specific','/ppic/expiry'),
		('pPiC','Tujuan khusus',false,'all','/documents'),
		('ppic_assistant','Role berbeda',false,'all','/ppic/opname')`)
	ppicCaseSQL(t, `INSERT INTO role_permissions(role,permission) VALUES
		('PPIC','ppic.opname.approve'),('PPIC','ppic.expiry.ack'),('PPIC','documents.view'),('PPIC','procurement.rawmaterials.submit'),
		('Ppic','ppic.control.view'),('ppic_assistant','ppic.opname.view')`)
	ppicCaseSQL(t, `CREATE TABLE role_work_unit_scope(role TEXT,work_unit_id TEXT)`)
	ppicCaseSQL(t, `INSERT INTO role_work_unit_scope VALUES('PPIC','existing-work-unit')`)
	ppicCaseSQL(t, `CREATE TABLE cloud_admins(role TEXT)`)
	ppicCaseSQL(t, `INSERT INTO cloud_admins VALUES('PPIC')`)
	ppicCaseSQL(t, `DELETE FROM role_permissions WHERE (role='ppic' AND permission='ppic.control.update') OR (role='admin' AND permission='ppic.control.export')`)
	if _, err := SavePpicUsagePolicy(usagePolicyInput(f, 2, 1, 0), nil, "PPIC"); err != nil {
		t.Fatal(err)
	}
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	for role, want := range map[string]int{"PPIC": 3, "Ppic": 1, "pPiC": 3, "ppic_assistant": 0, "ppic": 2, "admin": 2} {
		if got := ppicCasePermissionCount(t, role, "ppic.control.%"); got != want {
			t.Fatalf("repair grants %s: got %d want %d", role, got, want)
		}
	}
	if got := ppicCasePermissionCount(t, "PPIC", "%"); got != 7 {
		t.Fatalf("existing PPIC rights changed beyond three control grants: %d", got)
	}
	for role, want := range map[string]string{"PPIC": "/ppic/usage-control", "Ppic": "/ppic/expiry", "pPiC": "/documents"} {
		var redirect string
		if err := database.DB.QueryRow(`SELECT redirect_to FROM roles WHERE name=$1`, role).Scan(&redirect); err != nil || redirect != want {
			t.Fatalf("repair landing %s: %s want %s (%v)", role, redirect, want, err)
		}
	}
	var scope, description, workUnit, actorRole string
	if err := database.DB.QueryRow(`SELECT scope_type,description FROM roles WHERE name='PPIC'`).Scan(&scope, &description); err != nil || scope != "specific" || description != "Pengawas bahan baku" {
		t.Fatalf("scope/identity changed: %s %s %v", scope, description, err)
	}
	if err := database.DB.QueryRow(`SELECT work_unit_id FROM role_work_unit_scope WHERE role='PPIC'`).Scan(&workUnit); err != nil || workUnit != "existing-work-unit" {
		t.Fatalf("scope members changed: %s %v", workUnit, err)
	}
	if err := database.DB.QueryRow(`SELECT role FROM cloud_admins`).Scan(&actorRole); err != nil || actorRole != "PPIC" {
		t.Fatalf("account role renamed: %s %v", actorRole, err)
	}
	ppicCaseSQL(t, `DELETE FROM role_permissions WHERE role='PPIC' AND permission='ppic.control.update'`)
	ppicCaseSQL(t, `UPDATE roles SET redirect_to='/ppic/expiry' WHERE name='PPIC'`)
	if err := database.RunPpicUsageControlMigrations(); err != nil {
		t.Fatal(err)
	}
	if got := ppicCasePermissionCount(t, "PPIC", "ppic.control.update"); got != 0 {
		t.Fatal("repair reran and restored revoked permission")
	}
	var redirect string
	if err := database.DB.QueryRow(`SELECT redirect_to FROM roles WHERE name='PPIC'`).Scan(&redirect); err != nil || redirect != "/ppic/expiry" {
		t.Fatalf("repair reran and changed chosen landing: %s %v", redirect, err)
	}
	history, err := ListPpicUsagePolicyHistory(f.item, f.own, nil)
	if err != nil || len(history) != 1 || history[0].UpdatedBy != "PPIC" {
		t.Fatalf("policy history changed: %+v %v", history, err)
	}
	var roleCount int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM roles WHERE lower(name)='ppic'`).Scan(&roleCount); err != nil || roleCount != 4 {
		t.Fatalf("repair deleted or merged roles: %d %v", roleCount, err)
	}
}
