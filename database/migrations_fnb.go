package database

import "fmt"

// RunFnbMigrations preserves existing recipes and POS product categories.
func RunFnbMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`UPDATE roles SET redirect_to='/fnb/recipes' WHERE redirect_to='/recipes'`,
		`ALTER TABLE recipe_masters ADD COLUMN IF NOT EXISTS menu_kind VARCHAR(16) NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS fnb_product_profiles (
			product_id CHAR(26) PRIMARY KEY REFERENCES cloud_products(id) ON DELETE CASCADE,
			menu_kind VARCHAR(16) NOT NULL DEFAULT '' CHECK(menu_kind IN ('','food','beverage')),
			target_cost_pct NUMERIC(8,4) NOT NULL DEFAULT 35 CHECK(target_cost_pct > 0 AND target_cost_pct <= 100),
			packaging_cost NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK(packaging_cost >= 0),
			overhead_cost NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK(overhead_cost >= 0),
			updated_by VARCHAR(100) NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS fnb_developments (
			id CHAR(26) PRIMARY KEY, outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			name VARCHAR(200) NOT NULL, menu_kind VARCHAR(16) NOT NULL CHECK(menu_kind IN ('food','beverage')),
			status VARCHAR(16) NOT NULL DEFAULT 'idea' CHECK(status IN ('idea','trial','review','approved','launched','archived')),
			brief TEXT NOT NULL DEFAULT '', trial_notes TEXT NOT NULL DEFAULT '',
			target_price NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK(target_price >= 0),
			target_cost_pct NUMERIC(8,4) NOT NULL DEFAULT 35 CHECK(target_cost_pct > 0 AND target_cost_pct <= 100),
			recipe_master_id CHAR(26) REFERENCES recipe_masters(id) ON DELETE SET NULL,
			product_id CHAR(26) REFERENCES cloud_products(id) ON DELETE SET NULL,
			version INTEGER NOT NULL DEFAULT 1, created_by VARCHAR(100) NOT NULL DEFAULT '',
			updated_by VARCHAR(100) NOT NULL DEFAULT '', approved_by VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fnb_developments_outlet_status ON fnb_developments(outlet_id,status)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrasi F&B: %w", err)
		}
	}
	perms := []string{"fnb.dashboard.view", "fnb.development.view", "fnb.development.create", "fnb.development.update", "fnb.development.approve", "fnb.costing.view", "fnb.costing.update"}
	for _, role := range []string{"admin", "superadmin"} {
		for _, p := range perms {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, p); err != nil {
				return err
			}
		}
	}
	// Seed once: subsequent boots must respect the owner's changes to this role.
	res, err := tx.Exec(`INSERT INTO roles(name,description,is_system,scope_type,redirect_to)
		VALUES('fnb','Divisi F&B: pengembangan menu, resep, dan pengendalian HPP',false,'all','/fnb/dashboard') ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		for _, p := range append(perms, "recipes.view", "recipes.create", "recipes.update", "recipes.delete", "products.view", "products.create", "products.update") {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES('fnb',$1) ON CONFLICT DO NOTHING`, p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
