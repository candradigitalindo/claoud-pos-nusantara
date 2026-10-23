package database

// Documents are a shared library. View/download access is seeded to all roles.
func RunDocumentMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id CHAR(26) PRIMARY KEY,
			document_number VARCHAR(120) NOT NULL CHECK(btrim(document_number)<>''),
			about VARCHAR(1000) NOT NULL CHECK(btrim(about)<>''),
			file_name VARCHAR(255) NOT NULL, content_type VARCHAR(150) NOT NULL,
			file_size BIGINT NOT NULL CHECK(file_size>0 AND file_size<=20971520),
			uploaded_by VARCHAR(100) NOT NULL,
			uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			download_count BIGINT NOT NULL DEFAULT 0 CHECK(download_count>=0)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_number ON documents(lower(document_number))`,
		`CREATE INDEX IF NOT EXISTS idx_documents_uploaded ON documents(uploaded_at DESC,id DESC)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	for _, permission := range []string{"documents.view", "documents.create", "documents.download"} {
		if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES('admin',$1),('superadmin',$1) ON CONFLICT DO NOTHING`, permission); err != nil {
			return err
		}
	}
	// Seed existing roles once, including custom roles without any permissions.
	// Later permission changes in the role editor must survive server restarts.
	seed, err := tx.Exec(`INSERT INTO app_settings(key,value) VALUES('mig_documents_all_roles','done') ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if n, err := seed.RowsAffected(); err != nil {
		return err
	} else if n > 0 {
		if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission)
			SELECT roles.name, p.permission FROM roles
			CROSS JOIN (VALUES ('documents.view'), ('documents.download')) AS p(permission)
			ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
