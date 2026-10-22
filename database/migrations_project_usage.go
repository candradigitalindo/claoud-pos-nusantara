package database

// Usage reports keep proposed quantities separate from quantities already
// posted to the material ledger. Revising evidence never consumes stock twice.
func RunProjectUsageMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS project_id CHAR(26) REFERENCES projects(id)`,
		`UPDATE assets a SET project_id=pr.project_id FROM purchase_requests pr WHERE a.purchase_request_id=pr.id AND a.project_id IS NULL AND pr.project_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_assets_project ON assets(project_id) WHERE project_id IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS project_usage_reports (
			id CHAR(26) PRIMARY KEY, report_number VARCHAR(40) NOT NULL UNIQUE,
			project_id CHAR(26) NOT NULL REFERENCES projects(id),
			kind VARCHAR(20) NOT NULL CHECK(kind IN ('pemakaian','susut')),
			usage_date DATE NOT NULL, location VARCHAR(200) NOT NULL DEFAULT '',
			work_description TEXT NOT NULL DEFAULT '', performer VARCHAR(150) NOT NULL DEFAULT '',
			pic_name VARCHAR(150) NOT NULL DEFAULT '',
			items JSONB NOT NULL DEFAULT '[]', posted_items JSONB NOT NULL DEFAULT '[]',
			evidence JSONB NOT NULL DEFAULT '[]',
			status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','submitted','revision','verified','cancelled')),
			version BIGINT NOT NULL DEFAULT 1,
			created_by VARCHAR(150) NOT NULL, created_by_id VARCHAR(100) NOT NULL,
			submitted_by_id VARCHAR(100) NOT NULL DEFAULT '', reviewed_by VARCHAR(150) NOT NULL DEFAULT '',
			review_note TEXT NOT NULL DEFAULT '', submitted_at TIMESTAMPTZ, reviewed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_usage_project ON project_usage_reports(project_id,usage_date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_project_usage_status ON project_usage_reports(status,updated_at DESC)`,
		`CREATE TABLE IF NOT EXISTS project_usage_events (
			id CHAR(26) PRIMARY KEY, report_id CHAR(26) NOT NULL REFERENCES project_usage_reports(id) ON DELETE CASCADE,
			action VARCHAR(30) NOT NULL, actor VARCHAR(150) NOT NULL, actor_id VARCHAR(100) NOT NULL,
			note TEXT NOT NULL DEFAULT '', snapshot JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_usage_events ON project_usage_events(report_id,created_at,id)`,
		`ALTER TABLE project_material_logs ADD COLUMN IF NOT EXISTS evidence_url VARCHAR(255) NOT NULL DEFAULT ''`,
		// NOT VALID preserves older records for reconciliation while enforcing
		// the invariant for every subsequent insert/update.
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='project_material_balance_nonnegative' AND conrelid='project_materials'::regclass) THEN
			ALTER TABLE project_materials ADD CONSTRAINT project_material_balance_nonnegative CHECK (
				qty_received>=0 AND qty_used>=0 AND qty_returned>=0 AND qty_wasted>=0 AND
				qty_received>=qty_used+qty_returned+qty_wasted) NOT VALID;
		END IF; END $$`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	for _, permission := range []string{"assets.usage.view", "assets.usage.create", "assets.usage.verify"} {
		if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES('admin',$1),('superadmin',$1) ON CONFLICT DO NOTHING`, permission); err != nil {
			return err
		}
	}
	for _, permission := range []string{"assets.usage.view", "assets.usage.create"} {
		if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) SELECT name,$1 FROM roles WHERE name='perlengkapan' ON CONFLICT DO NOTHING`, permission); err != nil {
			return err
		}
	}
	return tx.Commit()
}
