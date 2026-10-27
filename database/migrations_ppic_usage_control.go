package database

import (
	"database/sql"
	"fmt"
)

// RunPpicUsageControlMigrations creates explicit limits without inventing any
// tolerances. History records survive catalog deletion and cannot be rewritten.
func RunPpicUsageControlMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS ppic_usage_policies (
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
			usage_tolerance_pct NUMERIC(8,4) NOT NULL CHECK(usage_tolerance_pct BETWEEN 0 AND 100 AND usage_tolerance_pct::text NOT IN ('NaN','Infinity','-Infinity')),
			waste_tolerance_pct NUMERIC(8,4) NOT NULL CHECK(waste_tolerance_pct BETWEEN 0 AND 100 AND waste_tolerance_pct::text NOT IN ('NaN','Infinity','-Infinity')),
			version INTEGER NOT NULL CHECK(version > 0),
			reason TEXT NOT NULL CHECK(length(trim(reason)) > 0),
			updated_by VARCHAR(100) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY(item_id,warehouse_id)
		)`,
		`CREATE TABLE IF NOT EXISTS ppic_usage_policy_history (
			item_id CHAR(26) NOT NULL, warehouse_id CHAR(26) NOT NULL,
			usage_tolerance_pct NUMERIC(8,4) NOT NULL CHECK(usage_tolerance_pct BETWEEN 0 AND 100 AND usage_tolerance_pct::text NOT IN ('NaN','Infinity','-Infinity')),
			waste_tolerance_pct NUMERIC(8,4) NOT NULL CHECK(waste_tolerance_pct BETWEEN 0 AND 100 AND waste_tolerance_pct::text NOT IN ('NaN','Infinity','-Infinity')),
			version INTEGER NOT NULL CHECK(version > 0),
			reason TEXT NOT NULL CHECK(length(trim(reason)) > 0),
			updated_by VARCHAR(100) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY(item_id,warehouse_id,version)
		)`,
		`CREATE OR REPLACE FUNCTION ppic_usage_history_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'riwayat batas pemakaian PPIC tidak dapat diubah atau dihapus'; END; $$`,
		`DO $$ BEGIN
			IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='ppic_usage_history_immutable' AND tgrelid='ppic_usage_policy_history'::regclass) THEN
				CREATE TRIGGER ppic_usage_history_immutable BEFORE UPDATE OR DELETE ON ppic_usage_policy_history
				FOR EACH ROW EXECUTE FUNCTION ppic_usage_history_immutable();
			END IF;
		END $$`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrasi kontrol pemakaian PPIC: %w", err)
		}
	}
	// The marker is committed together with the permission changes. Subsequent
	// starts must preserve any permission revocation made in the role editor.
	res, err := tx.Exec(`INSERT INTO app_settings(key,value) VALUES('mig_ppic_usage_control_permissions','done') ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n > 0 {
		roleSeed, err := tx.Exec(`INSERT INTO roles(name,description,is_system,scope_type,redirect_to)
			SELECT 'ppic','PPIC: pengendalian pemakaian bahan baku dan batas waste',false,'all','/ppic/usage-control'
			WHERE NOT EXISTS(SELECT 1 FROM roles WHERE lower(name)='ppic') ON CONFLICT DO NOTHING`)
		if err != nil {
			return err
		}
		// A newly created role follows the shared document-library policy even
		// when the one-time document migration ran before this role existed.
		if created, err := roleSeed.RowsAffected(); err != nil {
			return err
		} else if created > 0 {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission)
				VALUES('ppic','documents.view'),('ppic','documents.download'),
				('ppic','ppic.opname.view'),('ppic','ppic.expiry.view') ON CONFLICT DO NOTHING`); err != nil {
				return err
			}
		}
		for _, permission := range []string{"ppic.control.view", "ppic.control.update", "ppic.control.export"} {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission)
				SELECT target.role,$1 FROM (
					SELECT name AS role FROM roles WHERE lower(name)='ppic'
					UNION SELECT 'admin' UNION SELECT 'superadmin'
				) target ON CONFLICT DO NOTHING`, permission); err != nil {
				return err
			}
		}
	}
	if err := repairPpicUsageRoleCase(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// The original seed only recognized lowercase "ppic". Repair skipped legacy
// case variants once, without modifying their names, scopes or existing rights.
func repairPpicUsageRoleCase(tx *sql.Tx) error {
	seed, err := tx.Exec(`INSERT INTO app_settings(key,value) VALUES('mig_ppic_usage_control_role_case','done') ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	count, err := seed.RowsAffected()
	if err != nil || count == 0 {
		return err
	}
	// A partial control grant may be deliberate; never expand that selection.
	rows, err := tx.Query(`SELECT r.name FROM roles r WHERE lower(r.name)='ppic' AND r.name<>'ppic'
		AND NOT EXISTS(SELECT 1 FROM role_permissions p WHERE p.role=r.name AND p.permission LIKE 'ppic.control.%')
		ORDER BY r.name FOR UPDATE OF r`)
	if err != nil {
		return err
	}
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return err
		}
		roles = append(roles, role)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, role := range roles {
		for _, permission := range []string{"ppic.control.view", "ppic.control.update", "ppic.control.export"} {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, permission); err != nil {
				return err
			}
		}
		// Retain unrelated custom landing pages. Only the old operational PPIC
		// destinations are moved to the newly available control screen.
		if _, err := tx.Exec(`UPDATE roles SET redirect_to='/ppic/usage-control' WHERE name=$1
			AND split_part(split_part(redirect_to,'?',1),'#',1) IN (
				'/ppic/dashboard','/ppic/planning-params','/ppic/forecast','/ppic/mrp',
				'/ppic/production-plans','/ppic/work-orders','/ppic/reports','/ppic/opname','/ppic/expiry','/403')`, role); err != nil {
			return err
		}
	}
	return nil
}
