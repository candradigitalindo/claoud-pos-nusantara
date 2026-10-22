package database

// RunPOSVoidMigrations adds receipts for stock consumption and void events.
// No historical void is replayed: stock is processed only by incoming POS sync.
func RunPOSVoidMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		// Support the existing master/custom recipe schema without replacing a
		// richer recipe view when one is already installed.
		`DO $$ BEGIN
			IF to_regclass('public.product_recipe_materials') IS NULL THEN
				EXECUTE $view$CREATE VIEW product_recipe_materials AS
					SELECT x.product_id,x.item_id,SUM(x.qty_base) AS qty_base FROM (
						SELECT p.id AS product_id,r.item_id,r.qty_base
						FROM cloud_products p JOIN recipe_items r ON r.recipe_master_id=p.recipe_master_id
						WHERE p.stock_type='recipe'
						UNION ALL
						SELECT p.id AS product_id,r.item_id,r.qty_base
						FROM cloud_products p JOIN product_recipes r ON r.product_id=p.id
						WHERE p.stock_type='recipe' AND p.recipe_master_id IS NULL
					) x GROUP BY x.product_id,x.item_id$view$;
			END IF;
		END $$`,
		`ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS pos_product_local_id VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE order_item_voids DROP CONSTRAINT IF EXISTS order_item_voids_outlet_id_local_id_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_order_item_void_event ON order_item_voids(outlet_id,local_id,voided_at)`,
		`CREATE TABLE IF NOT EXISTS pos_stock_deductions (
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			transaction_id VARCHAR(50) NOT NULL, product_local_id VARCHAR(100) NOT NULL,
			qty NUMERIC(15,4) NOT NULL CHECK(qty>0),
			created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			PRIMARY KEY(outlet_id,transaction_id,product_local_id)
		)`,
		`CREATE TABLE IF NOT EXISTS pos_void_stock_events (
			id CHAR(26) PRIMARY KEY, outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			order_id VARCHAR(50) NOT NULL, source_type VARCHAR(20) NOT NULL,
			source_id VARCHAR(50) NOT NULL, voided_at TIMESTAMP NOT NULL,
			actor TEXT NOT NULL, reason TEXT NOT NULL, items JSONB NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id,source_type,source_id,voided_at)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_pos_full_order_void ON pos_void_stock_events(outlet_id,order_id) WHERE source_type='order'`,
		`ALTER TABLE stock_wastes ADD COLUMN IF NOT EXISTS void_event_id CHAR(26) REFERENCES pos_void_stock_events(id)`,
		`ALTER TABLE stock_wastes ADD COLUMN IF NOT EXISTS source_movement_id CHAR(26)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_waste_source_movement ON stock_wastes(source_movement_id) WHERE source_movement_id IS NOT NULL`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
