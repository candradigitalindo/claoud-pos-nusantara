package database

import (
	"database/sql"
	"encoding/json"
)

const retiredPpicAlertTemplate = `⚠️ *Peringatan Stok {date}*
Batch kedaluwarsa: *{expired_count}* · ≤3 hari: *{expiring_count}* · Di bawah ROP: *{rop_count}*

Segera kedaluwarsa:
{expiring_list}

Di bawah titik pesan ulang:
{rop_list}`

// RunPpicFocusMigrations retires access to removed PPIC functions while keeping
// every historical inventory/production document and retained permission.
func RunPpicFocusMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM role_permissions WHERE permission IN (
			'ppic.dashboard.view','ppic.planning.view','ppic.planning.update',
			'ppic.forecast.view','ppic.forecast.manage','ppic.mrp.view','ppic.mrp.run','ppic.mrp.execute',
			'ppic.production.view','ppic.production.create','ppic.production.approve',
			'ppic.workorders.view','ppic.workorders.create','ppic.workorders.execute',
			'ppic.reports.view','ppic.reports.export')`,
		// A removed landing page may only redirect to a feature this role can
		// already read. Subpermissions imply view, as in RequirePermission.
		`UPDATE roles r SET redirect_to=CASE
			WHEN r.name='superadmin' OR EXISTS(SELECT 1 FROM role_permissions p WHERE p.role=r.name AND p.permission IN ('ppic.control.view','ppic.control.update','ppic.control.export')) THEN '/ppic/usage-control'
			WHEN EXISTS(SELECT 1 FROM role_permissions p WHERE p.role=r.name AND p.permission IN ('ppic.opname.view','ppic.opname.create','ppic.opname.approve')) THEN '/ppic/opname'
			WHEN EXISTS(SELECT 1 FROM role_permissions p WHERE p.role=r.name AND p.permission IN ('ppic.expiry.view','ppic.expiry.ack')) THEN '/ppic/expiry'
			ELSE '/403' END
			WHERE split_part(split_part(r.redirect_to,'?',1),'#',1) IN (
			'/ppic/dashboard','/ppic/planning-params','/ppic/forecast','/ppic/mrp',
			'/ppic/production-plans','/ppic/work-orders','/ppic/reports')`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	if err := retirePpicAlertTemplate(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Overrides identical to the former builtin should follow the new default.
// Custom wording, other events and malformed legacy settings remain untouched.
func retirePpicAlertTemplate(tx *sql.Tx) error {
	var raw string
	err := tx.QueryRow(`SELECT value FROM app_settings WHERE key='wa_templates' FOR UPDATE`).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var templates map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &templates) != nil {
		return nil
	}
	var ppic string
	if json.Unmarshal(templates["ppic.alert"], &ppic) != nil || ppic != retiredPpicAlertTemplate {
		return nil
	}
	delete(templates, "ppic.alert")
	encoded, err := json.Marshal(templates)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE app_settings SET value=$1 WHERE key='wa_templates'`, string(encoded))
	return err
}
