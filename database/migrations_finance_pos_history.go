package database

// These are factual source snapshots, captured atomically with POS writes.
// There is deliberately no production backfill and no foreign key to the
// mutable order/transaction rows: a void must not erase financial history.
func RunFinancePOSHistoryMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS finance_pos_snapshots (
			id BIGSERIAL PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			entity_kind VARCHAR(16) NOT NULL CHECK(entity_kind IN ('order','transaction')),
			source_id VARCHAR(50) NOT NULL,
			source_created_at TIMESTAMP NOT NULL,
			source_updated_at TIMESTAMP,
			voided_at TIMESTAMP,
			observed_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			financial_hash VARCHAR(64) NOT NULL,
			payload JSONB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_finance_pos_snapshot_source
			ON finance_pos_snapshots(outlet_id,entity_kind,source_id,id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_finance_pos_snapshot_void
			ON finance_pos_snapshots(outlet_id,voided_at) WHERE entity_kind='transaction' AND voided_at IS NOT NULL`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
