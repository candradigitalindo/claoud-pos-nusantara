package database

import "fmt"

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
			VALUES('ppic','PPIC: pengendalian pemakaian bahan baku dan batas waste',false,'all','/ppic/usage-control') ON CONFLICT DO NOTHING`)
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
			for _, role := range []string{"admin", "superadmin", "ppic"} {
				if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, permission); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
