package database

// RunPOSIdentityMigrations reserves outlet-local identities before either the
// entity or a referring child arrives. Existing business rows are not rewritten.
func RunPOSIdentityMigrations() error {
	_, err := DB.Exec(`CREATE TABLE IF NOT EXISTS pos_entity_ids (
		entity_type VARCHAR(30) NOT NULL,
		outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
		local_id VARCHAR(50) NOT NULL,
		cloud_id CHAR(26) NOT NULL,
		PRIMARY KEY(entity_type, outlet_id, local_id),
		UNIQUE(entity_type, cloud_id)
	)`)
	return err
}
