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
		`ALTER TABLE recipe_masters ADD COLUMN IF NOT EXISTS initial_cost_snapshot JSONB`,
		`CREATE TABLE IF NOT EXISTS fnb_outlet_policies (
			outlet_id CHAR(26) PRIMARY KEY REFERENCES outlets(id) ON DELETE CASCADE,
			target_cost_pct NUMERIC(8,4) NOT NULL CHECK(target_cost_pct>0 AND target_cost_pct<=100 AND target_cost_pct::text NOT IN ('NaN','Infinity','-Infinity')),
			updated_by VARCHAR(100) NOT NULL DEFAULT '',updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS product_recipe_links (
			product_id CHAR(26) NOT NULL REFERENCES cloud_products(id) ON DELETE CASCADE ON UPDATE CASCADE,
			recipe_master_id CHAR(26) NOT NULL REFERENCES recipe_masters(id) ON DELETE CASCADE,
			multiplier NUMERIC(15,4) NOT NULL CHECK(multiplier > 0 AND multiplier::text NOT IN ('NaN','Infinity','-Infinity')),
			position INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY(product_id,recipe_master_id)
		)`,
		// Keep legacy single-recipe products readable without inventing historical prices.
		`CREATE OR REPLACE VIEW product_recipe_sources AS
			SELECT l.product_id,l.recipe_master_id,l.multiplier,l.position
			FROM product_recipe_links l JOIN cloud_products p ON p.id=l.product_id WHERE p.stock_type='recipe'
			UNION ALL
			SELECT p.id,p.recipe_master_id,1::numeric,0 FROM cloud_products p
			WHERE p.stock_type='recipe' AND p.recipe_master_id IS NOT NULL
			AND NOT EXISTS(SELECT 1 FROM product_recipe_links l WHERE l.product_id=p.id)`,
		`CREATE OR REPLACE VIEW product_recipe_materials AS
			SELECT x.product_id,x.item_id,SUM(x.qty_base) AS qty_base FROM (
				SELECT s.product_id,i.item_id,i.qty_base*s.multiplier AS qty_base
				FROM product_recipe_sources s JOIN recipe_items i ON i.recipe_master_id=s.recipe_master_id
				UNION ALL
				SELECT r.product_id,r.item_id,r.qty_base FROM product_recipes r JOIN cloud_products p ON p.id=r.product_id
				WHERE p.stock_type='recipe' AND NOT EXISTS(SELECT 1 FROM product_recipe_sources s WHERE s.product_id=p.id)
			) x GROUP BY x.product_id,x.item_id`,
		`CREATE TABLE IF NOT EXISTS fnb_product_profiles (
			product_id CHAR(26) PRIMARY KEY REFERENCES cloud_products(id) ON DELETE CASCADE,
			menu_kind VARCHAR(16) NOT NULL DEFAULT '' CHECK(menu_kind IN ('','food','beverage')),
			target_cost_pct NUMERIC(8,4) NOT NULL DEFAULT 35 CHECK(target_cost_pct > 0 AND target_cost_pct <= 100),
			packaging_cost NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK(packaging_cost >= 0),
			overhead_cost NUMERIC(18,4) NOT NULL DEFAULT 0 CHECK(overhead_cost >= 0),
			updated_by VARCHAR(100) NOT NULL DEFAULT '', updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		// Product classification also covers non-F&B activities. Recipe and
		// development constraints deliberately remain food/beverage-only.
		`ALTER TABLE fnb_product_profiles DROP CONSTRAINT IF EXISTS fnb_product_profiles_menu_kind_check`,
		`ALTER TABLE fnb_product_profiles ADD CONSTRAINT fnb_product_profiles_menu_kind_check
			CHECK(menu_kind IN ('','food','beverage','activity'))`,
		`CREATE OR REPLACE VIEW product_menu_kinds AS
			SELECT p.id AS product_id,COALESCE(f.menu_kind,r.menu_kind,'') AS menu_kind,
				CASE WHEN f.product_id IS NOT NULL THEN 'profile'
					WHEN r.recipe_master_id IS NOT NULL THEN 'recipe' ELSE '' END AS menu_kind_source
			FROM cloud_products p LEFT JOIN fnb_product_profiles f ON f.product_id=p.id
			LEFT JOIN LATERAL (
				SELECT rm.id AS recipe_master_id,rm.menu_kind
				FROM product_recipe_sources ps JOIN recipe_masters rm ON rm.id=ps.recipe_master_id
				WHERE ps.product_id=p.id ORDER BY ps.position,rm.id LIMIT 1
			) r ON true`,
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
