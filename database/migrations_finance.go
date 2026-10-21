package database

import "fmt"

var FinancePermissions = []string{"finance.entries.view", "finance.entries.create", "finance.entries.post", "finance.entries.void"}

func RunFinanceMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS finance_entries (
			id CHAR(26) PRIMARY KEY, number VARCHAR(50) UNIQUE NOT NULL, request_key VARCHAR(100) UNIQUE NOT NULL,
			direction VARCHAR(8) NOT NULL CHECK(direction IN ('in','out')), account_code VARCHAR(16) NOT NULL,
			outlet_id CHAR(26) REFERENCES outlets(id), work_unit_id CHAR(26) REFERENCES work_units(id),
			amount NUMERIC(18,2) NOT NULL CHECK(amount>0 AND amount<=1000000000000), occurred_at TIMESTAMP NOT NULL,
			period VARCHAR(7) NOT NULL DEFAULT '', counterparty VARCHAR(200) NOT NULL,
			description VARCHAR(300) NOT NULL, notes TEXT NOT NULL DEFAULT '', reference VARCHAR(200) NOT NULL DEFAULT '',
			method VARCHAR(20) NOT NULL, bank_account_id CHAR(26) REFERENCES bank_accounts(id), bank_label TEXT NOT NULL DEFAULT '',
			proof_url TEXT NOT NULL DEFAULT '', status VARCHAR(12) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','posted','voided')),
			version INTEGER NOT NULL DEFAULT 1, created_by VARCHAR(100) NOT NULL, created_actor_id VARCHAR(100) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'), posted_by VARCHAR(100) NOT NULL DEFAULT '', posted_at TIMESTAMP,
			voided_by VARCHAR(100) NOT NULL DEFAULT '', voided_at TIMESTAMP, void_reason TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_finance_entries_date ON finance_entries(occurred_at,outlet_id)`,
		`ALTER TABLE finance_entries ADD COLUMN IF NOT EXISTS request_fingerprint VARCHAR(64) NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS finance_cash_classifications (
			id CHAR(26) PRIMARY KEY, movement_id CHAR(26) NOT NULL REFERENCES cloud_cash_movements(id),
			account_code VARCHAR(16) NOT NULL, notes TEXT NOT NULL, actor VARCHAR(100) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_finance_cash_classification ON finance_cash_classifications(movement_id,created_at DESC,id DESC)`,
		`CREATE TABLE IF NOT EXISTS finance_entry_events (
			id CHAR(26) PRIMARY KEY, entry_id CHAR(26) NOT NULL REFERENCES finance_entries(id),
			action VARCHAR(20) NOT NULL, actor VARCHAR(100) NOT NULL, notes TEXT NOT NULL DEFAULT '',
			snapshot JSONB NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
	} {
		if _, err = tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrasi keuangan: %w", err)
		}
	}
	for _, role := range []string{"admin", "superadmin"} {
		for _, p := range FinancePermissions {
			if _, err = tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, p); err != nil {
				return err
			}
		}
	}
	for _, role := range []struct {
		name, description string
		perms             []string
	}{
		{"finance_pencatat", "Keuangan: lihat operasional dan buat draf pemasukan/pengeluaran", []string{"finance.entries.view", "finance.entries.create"}},
		{"finance_supervisor", "Keuangan: periksa, bukukan, dan balik transaksi dengan jejak audit", FinancePermissions},
	} {
		res, err := tx.Exec(`INSERT INTO roles(name,description,is_system,scope_type,redirect_to) VALUES($1,$2,false,'all','/finance-operations') ON CONFLICT DO NOTHING`, role.name, role.description)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		for _, p := range append(append([]string{}, role.perms...), "reports.ledger.view", "reports.cashflow.view", "reports.pnl.view", "reports.balance.view") {
			if _, err = tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role.name, p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
