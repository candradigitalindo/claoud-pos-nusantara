package database

// RunCCTVMigrations mempertahankan konfigurasi lama (Hikvision/Dahua, direct)
// dan menambah sumber multi-vendor serta penghubung outlet tanpa VPN.
func RunCCTVMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`ALTER TABLE outlet_cameras ADD COLUMN IF NOT EXISTS connection_type VARCHAR(20) NOT NULL DEFAULT ''`,
		`ALTER TABLE outlet_cameras ADD COLUMN IF NOT EXISTS access_mode VARCHAR(20) NOT NULL DEFAULT 'direct'`,
		`ALTER TABLE outlet_cameras ADD COLUMN IF NOT EXISTS source_url_enc TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outlet_cameras ADD COLUMN IF NOT EXISTS playback_url_enc TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE outlet_cameras ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta'`,
		`CREATE TABLE IF NOT EXISTS outlet_cctv_gateways (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL UNIQUE REFERENCES outlets(id) ON DELETE CASCADE,
			token_hash CHAR(64) NOT NULL UNIQUE,
			last_seen TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
