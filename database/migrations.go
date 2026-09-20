package database

import (
	"log"
	"regexp"
	"strings"
)

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugNonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// backfillOutletSlugs assigns a unique slug to every outlet missing one,
// derived from its name (collisions get -2, -3, … or fall back to the code).
func backfillOutletSlugs() {
	rows, err := DB.Query(`SELECT id, name, code FROM outlets WHERE COALESCE(slug,'') = ''`)
	if err != nil {
		log.Printf("Backfill outlet slugs skipped: %v", err)
		return
	}
	type o struct{ id, name, code string }
	var list []o
	for rows.Next() {
		var x o
		if err := rows.Scan(&x.id, &x.name, &x.code); err == nil {
			list = append(list, x)
		}
	}
	rows.Close()

	taken := map[string]bool{}
	if r2, err := DB.Query(`SELECT slug FROM outlets WHERE COALESCE(slug,'') <> ''`); err == nil {
		for r2.Next() {
			var s string
			r2.Scan(&s)
			taken[s] = true
		}
		r2.Close()
	}

	for _, x := range list {
		base := slugify(x.name)
		if base == "" {
			base = slugify(x.code)
		}
		if base == "" {
			base = "outlet"
		}
		slug := base
		for i := 2; taken[slug]; i++ {
			slug = base + "-" + strings.ToLower(x.code)
			if taken[slug] {
				slug = base + "-" + itoa(i)
			}
		}
		taken[slug] = true
		DB.Exec(`UPDATE outlets SET slug = $1 WHERE id = $2`, slug, x.id)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func RunMigrations() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS outlets (
			id CHAR(26) PRIMARY KEY,
			code VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			address TEXT,
			api_key VARCHAR(100) UNIQUE NOT NULL,
			webhook_url TEXT,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,

		`CREATE TABLE IF NOT EXISTS cloud_orders (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			outlet_code VARCHAR(20) NOT NULL,
			table_number VARCHAR(20),
			customer_name VARCHAR(100),
			pax INTEGER DEFAULT 1,
			total_amount DECIMAL(15,2) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			items JSONB,
			payment_info JSONB,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		`CREATE TABLE IF NOT EXISTS cloud_transactions (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			outlet_code VARCHAR(20) NOT NULL,
			order_id VARCHAR(50),
			total_amount DECIMAL(15,2) NOT NULL,
			payment_method VARCHAR(30),
			cash_amount DECIMAL(15,2) DEFAULT 0,
			change_amount DECIMAL(15,2) DEFAULT 0,
			cashier_name VARCHAR(100),
			items JSONB,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		`CREATE TABLE IF NOT EXISTS cloud_products (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			name VARCHAR(200) NOT NULL,
			category_id VARCHAR(50),
			category_name VARCHAR(100),
			price DECIMAL(15,2) NOT NULL,
			stock INTEGER DEFAULT 0,
			destination VARCHAR(50),
			is_deleted BOOLEAN DEFAULT false,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		`CREATE TABLE IF NOT EXISTS cloud_categories (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50),
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			name VARCHAR(100) NOT NULL,
			is_deleted BOOLEAN DEFAULT false,
			version INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id),
			UNIQUE(outlet_id, name)
		)`,

		`CREATE TABLE IF NOT EXISTS cloud_analytics (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			outlet_code VARCHAR(20) NOT NULL,
			date DATE NOT NULL,
			summary JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, date)
		)`,

		`CREATE TABLE IF NOT EXISTS sync_logs (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			action VARCHAR(50) NOT NULL,
			entity_type VARCHAR(30),
			entity_count INTEGER DEFAULT 0,
			status VARCHAR(20) DEFAULT 'success',
			error_message TEXT,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,

		`CREATE TABLE IF NOT EXISTS sync_conflicts (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			entity_type VARCHAR(30) NOT NULL,
			entity_local_id VARCHAR(50),
			entity_cloud_id CHAR(26),
			conflict_field VARCHAR(50),
			cloud_value TEXT,
			local_value TEXT,
			cloud_version INTEGER,
			local_version INTEGER,
			resolution VARCHAR(20),
			resolved_by VARCHAR(100),
			resolved_at TIMESTAMP,
			notes TEXT,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,

		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_outlet ON cloud_orders(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_local ON cloud_orders(outlet_id, local_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_updated ON cloud_orders(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_transactions_outlet ON cloud_transactions(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_transactions_local ON cloud_transactions(outlet_id, local_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_products_outlet ON cloud_products(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_products_local ON cloud_products(outlet_id, local_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_products_updated ON cloud_products(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_categories_outlet ON cloud_categories(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_analytics_outlet ON cloud_analytics(outlet_id, date)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_logs_outlet ON sync_logs(outlet_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_created ON cloud_orders(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_transactions_created ON cloud_transactions(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_payment_status ON cloud_orders((payment_info->>'payment_status'))`,

		// Cashier shifts table
		`CREATE TABLE IF NOT EXISTS cloud_cashier_shifts (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			opened_by VARCHAR(100) NOT NULL,
			opened_at TIMESTAMP NOT NULL,
			opening_cash DECIMAL(15,2) NOT NULL DEFAULT 0,
			closed_at TIMESTAMP,
			closed_by VARCHAR(100),
			closing_cash DECIMAL(15,2),
			closing_card DECIMAL(15,2),
			closing_qris DECIMAL(15,2),
			closing_transfer DECIMAL(15,2),
			carry_over_cash DECIMAL(15,2),
			previous_shift_id VARCHAR(50),
			handover_to VARCHAR(100),
			status VARCHAR(20) NOT NULL DEFAULT 'open',
			notes TEXT,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		// Cash movements table
		`CREATE TABLE IF NOT EXISTS cloud_cash_movements (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			shift_id VARCHAR(50) NOT NULL,
			movement_type VARCHAR(10) NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			counterpart_name VARCHAR(200) NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_cloud_cashier_shifts_outlet ON cloud_cashier_shifts(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_cashier_shifts_status ON cloud_cashier_shifts(outlet_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_cashier_shifts_opened ON cloud_cashier_shifts(opened_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_cash_movements_outlet ON cloud_cash_movements(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_cash_movements_shift ON cloud_cash_movements(shift_id)`,

		// Admin users table
		`CREATE TABLE IF NOT EXISTS cloud_admins (
			id CHAR(26) PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(100) NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'admin',
			is_active BOOLEAN DEFAULT true,
			last_login_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,

		// Printers per outlet
		`CREATE TABLE IF NOT EXISTS cloud_printers (
			id CHAR(26) PRIMARY KEY,
			local_id VARCHAR(50) NOT NULL,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			name VARCHAR(100) NOT NULL,
			ip_address VARCHAR(50) NOT NULL,
			port INTEGER NOT NULL DEFAULT 9100,
			printer_type VARCHAR(20) NOT NULL,
			paper_size VARCHAR(10) NOT NULL DEFAULT '80mm',
			is_active BOOLEAN NOT NULL DEFAULT true,
			is_deleted BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, local_id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_cloud_printers_outlet ON cloud_printers(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_printers_active ON cloud_printers(outlet_id, is_active)`,

		// Add phone column to outlets (idempotent)
		`ALTER TABLE outlets ADD COLUMN IF NOT EXISTS phone VARCHAR(50) DEFAULT ''`,
	}

	for i, m := range migrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Migration %d failed: %v", i+1, err)
			return err
		}
	}

	// Additive migrations — safe for existing databases
	additiveMigrations := []string{
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS items JSONB`,
		`ALTER TABLE cloud_categories ADD COLUMN IF NOT EXISTS code_prefix VARCHAR(10) DEFAULT ''`,
		`ALTER TABLE cloud_categories ADD COLUMN IF NOT EXISTS printer_id VARCHAR(50) DEFAULT NULL`,
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS code VARCHAR(50) DEFAULT NULL`,
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`,
		// Unique product code per outlet — exclude soft-deleted rows so a deleted
		// product no longer blocks reusing its code (matches generateProductCode,
		// which only looks at is_deleted=false). Recreate to add the is_deleted filter.
		`DROP INDEX IF EXISTS idx_cloud_products_code`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cloud_products_code ON cloud_products(outlet_id, code) WHERE code IS NOT NULL AND code != '' AND is_deleted = false`,
	}

	for _, m := range additiveMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Additive migration skipped: %v", err)
		}
	}

	// Role permissions table
	rolePermMigrations := []string{
		`CREATE TABLE IF NOT EXISTS role_permissions (
			id SERIAL PRIMARY KEY,
			role VARCHAR(50) NOT NULL,
			permission VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(role, permission)
		)`,
		// Seed default permissions for admin (full access)
		`INSERT INTO role_permissions (role, permission)
		VALUES
			('admin', 'dashboard'),
			('admin', 'outlets'),
			('admin', 'products'),
			('admin', 'reports'),
			('admin', 'users'),
			('admin', 'settings.view'),
			('admin', 'settings.manage')
		ON CONFLICT (role, permission) DO NOTHING`,
		// Seed default permissions for manager (no user management)
		`INSERT INTO role_permissions (role, permission)
		VALUES
			('manager', 'dashboard'),
			('manager', 'outlets'),
			('manager', 'products'),
			('manager', 'reports')
		ON CONFLICT (role, permission) DO NOTHING`,
		// Seed default permissions for viewer (read-only)
		`INSERT INTO role_permissions (role, permission)
		VALUES
			('viewer', 'dashboard'),
			('viewer', 'reports')
		ON CONFLICT (role, permission) DO NOTHING`,
	}

	for _, m := range rolePermMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Role permissions migration skipped: %v", err)
		}
	}

	// Roles table (custom roles registry)
	rolesMigrations := []string{
		`CREATE TABLE IF NOT EXISTS roles (
			name VARCHAR(50) PRIMARY KEY,
			description VARCHAR(255) NOT NULL DEFAULT '',
			is_system BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		// Seed default roles (only admin is system role)
		`INSERT INTO roles (name, description, is_system) VALUES
			('admin', 'Full akses ke semua fitur', true),
			('manager', 'Akses tanpa manajemen pengguna', false),
			('viewer', 'Hanya dashboard dan laporan', false)
		ON CONFLICT (name) DO NOTHING`,
		// Ensure only admin is system role
		`UPDATE roles SET is_system = false WHERE name IN ('manager', 'viewer') AND is_system = true`,
		// Widen role_permissions.role column to allow custom role names
		`ALTER TABLE role_permissions ALTER COLUMN role TYPE VARCHAR(50)`,
		// Add redirect_to column for post-login redirect per role
		`ALTER TABLE roles ADD COLUMN IF NOT EXISTS redirect_to VARCHAR(100) NOT NULL DEFAULT '/'`,
		// Set default redirect_to for seeded roles
		`UPDATE roles SET redirect_to = '/' WHERE name = 'admin' AND redirect_to = '/'`,
		`UPDATE roles SET redirect_to = '/manager-dashboard' WHERE name = 'manager' AND redirect_to IN ('/', '/outlets')`,
		`UPDATE roles SET redirect_to = '/sales-report' WHERE name = 'viewer' AND redirect_to = '/'`,
		// Seed dashboard.manager permission for admin and manager roles
		`INSERT INTO role_permissions (role, permission) VALUES ('admin', 'dashboard.manager') ON CONFLICT (role, permission) DO NOTHING`,
		`INSERT INTO role_permissions (role, permission) VALUES ('manager', 'dashboard.manager') ON CONFLICT (role, permission) DO NOTHING`,
		// Migrate old-style permissions (outlets, products) to new format (module.view)
		`UPDATE role_permissions SET permission = 'outlets.view' WHERE permission = 'outlets'`,
		`UPDATE role_permissions SET permission = 'products.view' WHERE permission = 'products'`,
		`UPDATE role_permissions SET permission = 'users.view' WHERE permission = 'users'`,
		// Seed procurement.work_units for admin and manager roles
		`INSERT INTO role_permissions (role, permission) VALUES ('admin', 'procurement.work_units') ON CONFLICT (role, permission) DO NOTHING`,
		`INSERT INTO role_permissions (role, permission) VALUES ('manager', 'procurement.work_units') ON CONFLICT (role, permission) DO NOTHING`,
	}

	for _, m := range rolesMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Roles migration skipped: %v", err)
		}
	}

	// Purchase requests table
	purchaseMigrations := []string{
		`CREATE TABLE IF NOT EXISTS purchase_requests (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id),
			request_type VARCHAR(10) NOT NULL DEFAULT 'barang',
			requested_by VARCHAR(100) NOT NULL,
			vendor_name VARCHAR(200) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			items JSONB NOT NULL DEFAULT '[]',
			total_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
			total_hps DECIMAL(15,2) NOT NULL DEFAULT 0,
			total_final DECIMAL(15,2) NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			approved_by VARCHAR(100),
			approved_at TIMESTAMP,
			rejected_reason TEXT,
			paid_by VARCHAR(100),
			paid_at TIMESTAMP,
			payment_proof TEXT,
			received_by VARCHAR(100),
			received_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_outlet ON purchase_requests(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_status ON purchase_requests(status)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_created ON purchase_requests(created_at)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS request_type VARCHAR(10) NOT NULL DEFAULT 'barang'`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS vendor_name VARCHAR(200) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS total_hps DECIMAL(15,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS total_final DECIMAL(15,2) NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_type ON purchase_requests(request_type)`,
		// Allow purchase requests without outlet (standalone work units)
		`ALTER TABLE purchase_requests ALTER COLUMN outlet_id DROP NOT NULL`,
		// Payment detail fields
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS payment_account_dest VARCHAR(300) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS payment_account_source VARCHAR(300) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS payment_notes TEXT NOT NULL DEFAULT ''`,
		// Split purchase requests support
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS parent_id CHAR(26) REFERENCES purchase_requests(id) ON DELETE SET NULL`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS split_status VARCHAR(20) DEFAULT NULL`, // 'master' or NULL
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_parent ON purchase_requests(parent_id)`,
		// Request number for sequential numbering (ddmmYYnnn)
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS request_number VARCHAR(20) NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_number ON purchase_requests(request_number)`,
		// Cascade: sync children status to match their master's status (for existing data)
		// Only cascade to children that are still in an earlier/equal stage — never downgrade paid/partial/received
		`UPDATE purchase_requests c
		 SET status = p.status, updated_at = NOW()
		 FROM purchase_requests p
		 WHERE c.parent_id = p.id
		   AND p.split_status = 'master'
		   AND c.status != p.status
		   AND p.status IN ('approved', 'payment_requested', 'cancelled')
		   AND c.status NOT IN ('partial', 'paid', 'received')`,
	}

	for _, m := range purchaseMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Purchase requests migration skipped: %v", err)
		}
	}

	// Work units table — each outlet is auto-registered as a work unit
	workUnitMigrations := []string{
		`CREATE TABLE IF NOT EXISTS work_units (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			admin_id CHAR(26) REFERENCES cloud_admins(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_units_outlet ON work_units(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_work_units_admin ON work_units(admin_id)`,
		// Seed work units for existing outlets that don't have one yet
		`INSERT INTO work_units (id, outlet_id, name, created_at, updated_at)
		SELECT
			UPPER(LPAD(TO_HEX((EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT), 12, '0') ||
			LPAD(TO_HEX((RANDOM() * 2147483647)::INT), 8, '0') ||
			LPAD(TO_HEX((RANDOM() * 65535)::INT), 4, '0') ||
			'00'),
			o.id, o.name, NOW(), NOW()
		FROM outlets o
		WHERE NOT EXISTS (SELECT 1 FROM work_units wu WHERE wu.outlet_id = o.id)`,
		// Add work_unit_id column to purchase_requests
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS work_unit_id CHAR(26) REFERENCES work_units(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_work_unit ON purchase_requests(work_unit_id)`,
		// Allow standalone work units (not linked to an outlet)
		`ALTER TABLE work_units ALTER COLUMN outlet_id DROP NOT NULL`,
		`ALTER TABLE work_units DROP CONSTRAINT IF EXISTS work_units_outlet_id_key`,
		`DROP INDEX IF EXISTS work_units_outlet_id_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_work_units_outlet_unique ON work_units(outlet_id) WHERE outlet_id IS NOT NULL`,
	}

	for _, m := range workUnitMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Work units migration skipped: %v", err)
		}
	}

	// Migrate old permission keys to granular view/manage format
	permMigrations := map[string][]string{
		"outlets":     {"outlets.view", "outlets.manage"},
		"products":    {"products.view", "products.manage"},
		"procurement": {"procurement.view", "procurement.submit", "procurement.approve", "procurement.purchasing", "procurement.finance"},
		"users":       {"users.view", "users.manage"},
	}
	for old, newPerms := range permMigrations {
		var count int
		DB.QueryRow("SELECT COUNT(*) FROM role_permissions WHERE permission = $1", old).Scan(&count)
		if count > 0 {
			rows, err := DB.Query("SELECT role FROM role_permissions WHERE permission = $1", old)
			if err == nil {
				var roles []string
				for rows.Next() {
					var r string
					rows.Scan(&r)
					roles = append(roles, r)
				}
				rows.Close()
				for _, r := range roles {
					for _, np := range newPerms {
						DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, np)
					}
				}
				DB.Exec("DELETE FROM role_permissions WHERE permission = $1", old)
			}
			log.Printf("Migrated permission '%s' → %v for %d roles", old, newPerms, count)
		}
	}

	// Migrate procurement.manage → 3 granular permissions
	{
		var count int
		DB.QueryRow("SELECT COUNT(*) FROM role_permissions WHERE permission = 'procurement.manage'").Scan(&count)
		if count > 0 {
			rows, err := DB.Query("SELECT role FROM role_permissions WHERE permission = 'procurement.manage'")
			if err == nil {
				var roles []string
				for rows.Next() {
					var r string
					rows.Scan(&r)
					roles = append(roles, r)
				}
				rows.Close()
				for _, r := range roles {
					DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, "procurement.submit")
					DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, "procurement.approve")
					DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, "procurement.purchasing")
					DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, "procurement.finance")
				}
				DB.Exec("DELETE FROM role_permissions WHERE permission = 'procurement.manage'")
			}
			log.Printf("Migrated 'procurement.manage' → submit+approve+purchasing+finance for %d roles", count)
		}
	}

	// Ensure roles that had procurement.finance also get procurement.purchasing (added later)
	{
		rows, err := DB.Query("SELECT DISTINCT role FROM role_permissions WHERE permission = 'procurement.finance'")
		if err == nil {
			var roles []string
			for rows.Next() {
				var r string
				rows.Scan(&r)
				roles = append(roles, r)
			}
			rows.Close()
			for _, r := range roles {
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, 'procurement.purchasing') ON CONFLICT DO NOTHING", r)
			}
		}
	}

	// Settings table (key-value store for app-wide settings)
	settingsMigrations := []string{
		`CREATE TABLE IF NOT EXISTS app_settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		// Seed default settings
		`INSERT INTO app_settings (key, value) VALUES
			('company_name', ''),
			('company_address', ''),
			('company_phone', ''),
			('company_email', ''),
			('company_tax_id', ''),
			('company_logo_url', ''),
			('timezone', 'Asia/Jakarta')
		ON CONFLICT (key) DO NOTHING`,
		// SQL function: convert a TIMESTAMP to DATE in the configured timezone.
		// Treats stored timestamps as UTC, converts to app timezone, then extracts date.
		`CREATE OR REPLACE FUNCTION tz_date(ts TIMESTAMP) RETURNS DATE AS $$
			SELECT ((ts AT TIME ZONE 'UTC') AT TIME ZONE COALESCE(
				(SELECT value FROM app_settings WHERE key = 'timezone'),
				'Asia/Jakarta'
			))::date;
		$$ LANGUAGE sql STABLE`,
		// SQL function: return today's date in the configured timezone.
		`CREATE OR REPLACE FUNCTION tz_today() RETURNS DATE AS $$
			SELECT (NOW() AT TIME ZONE COALESCE(
				(SELECT value FROM app_settings WHERE key = 'timezone'),
				'Asia/Jakarta'
			))::date;
		$$ LANGUAGE sql STABLE`,
		// SQL function: jam lokal (0-23) dari timestamp UTC, mengikuti timezone
		// aplikasi — untuk pola penjualan per jam (session DB berjalan di UTC,
		// jadi EXTRACT(HOUR ...) polos akan geser -7 jam dari WIB).
		`CREATE OR REPLACE FUNCTION tz_hour(ts TIMESTAMP) RETURNS INT AS $$
			SELECT EXTRACT(HOUR FROM (ts AT TIME ZONE 'UTC') AT TIME ZONE COALESCE(
				(SELECT value FROM app_settings WHERE key = 'timezone'),
				'Asia/Jakarta'
			))::int;
		$$ LANGUAGE sql STABLE`,
		// SQL function: awal hari lokal d sebagai wall-clock UTC (tipe kolom kita).
		// Dipakai di WHERE sebagai konstanta: "created_at >= tz_day_start($1)" bisa
		// pakai index, berbeda dengan "tz_date(created_at) >= $1" yang membungkus
		// kolom dan memaksa full scan.
		`CREATE OR REPLACE FUNCTION tz_day_start(d DATE) RETURNS TIMESTAMP AS $$
			SELECT ((d::timestamp) AT TIME ZONE COALESCE(
				(SELECT value FROM app_settings WHERE key = 'timezone'),
				'Asia/Jakarta'
			)) AT TIME ZONE 'UTC';
		$$ LANGUAGE sql STABLE`,
		// Tax settings
		`INSERT INTO app_settings (key, value) VALUES
			('tax_enabled', 'false'),
			('tax_rate', '10'),
			('tax_name', 'Pajak Restoran (PB1)')
		ON CONFLICT (key) DO NOTHING`,
		// Add tax_amount column to cloud_transactions
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS tax_amount DECIMAL(15,2) DEFAULT 0`,
		// Pemisahan penjualan vs pajak vs tambahan lainnya (DPP & charges)
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS subtotal DECIMAL(15,2) DEFAULT 0`,
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS other_charges_total DECIMAL(15,2) DEFAULT 0`,
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS charges JSONB`,
		// Laporan shift lengkap: jumlah transaksi per metode + kas masuk/keluar
		`ALTER TABLE cloud_cashier_shifts ADD COLUMN IF NOT EXISTS report JSONB`,
		// Rincian pembayaran multi-metode (Gabung Bayar / Split Bill). Header
		// cloud_transactions.payment_method bisa bernilai 'mixed'; rincian nyata
		// per metode disimpan di sini. outlet_id & created_at didenormalisasi dari
		// transaksi induk agar rekap per-metode tak perlu JOIN dan tanggalnya = tanggal transaksi.
		`CREATE TABLE IF NOT EXISTS transaction_payments (
			id              BIGSERIAL PRIMARY KEY,
			transaction_id  CHAR(26) NOT NULL,
			outlet_id       CHAR(26) NOT NULL DEFAULT '',
			payment_method  VARCHAR(20) NOT NULL,
			amount          DECIMAL(15,2) NOT NULL DEFAULT 0,
			payment_note    TEXT,
			created_at      TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transaction_payments_txid ON transaction_payments(transaction_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transaction_payments_method ON transaction_payments(payment_method)`,
		`CREATE INDEX IF NOT EXISTS idx_transaction_payments_outlet ON transaction_payments(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transaction_payments_created ON transaction_payments(created_at)`,
		// Manajemen aset (meja, kursi, dll) + histori perawatan per barang.
		`CREATE TABLE IF NOT EXISTS assets (
			id             CHAR(26) PRIMARY KEY,
			outlet_id      CHAR(26) NOT NULL,
			code           VARCHAR(50) DEFAULT '',
			name           VARCHAR(150) NOT NULL,
			category       VARCHAR(80) DEFAULT '',
			quantity       INT DEFAULT 1,
			unit           VARCHAR(20) DEFAULT 'unit',
			condition      VARCHAR(20) DEFAULT 'baik',
			location       VARCHAR(150) DEFAULT '',
			purchase_date  DATE,
			purchase_price DECIMAL(15,2) DEFAULT 0,
			notes          TEXT DEFAULT '',
			is_deleted     BOOLEAN DEFAULT false,
			created_at     TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at     TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_outlet ON assets(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_deleted ON assets(is_deleted)`,
		`CREATE TABLE IF NOT EXISTS asset_maintenances (
			id               CHAR(26) PRIMARY KEY,
			asset_id         CHAR(26) NOT NULL,
			maintenance_date DATE NOT NULL DEFAULT CURRENT_DATE,
			type             VARCHAR(30) DEFAULT 'rutin',
			description      TEXT NOT NULL,
			cost             DECIMAL(15,2) DEFAULT 0,
			performed_by     VARCHAR(120) DEFAULT '',
			condition_after  VARCHAR(20) DEFAULT '',
			next_due_date    DATE,
			created_at       TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_maintenances_asset ON asset_maintenances(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_maintenances_date ON asset_maintenances(maintenance_date)`,
		// Foto produk (agar calon reservasi bisa melihat menu).
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS photo_url VARCHAR(255) DEFAULT ''`,
		// Slug outlet untuk URL reservasi publik per outlet (/r/:slug).
		`ALTER TABLE outlets ADD COLUMN IF NOT EXISTS slug VARCHAR(80) DEFAULT ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_outlets_slug ON outlets(slug) WHERE slug <> ''`,
		// Reservasi (penjualan): pax, item dipesan, subtotal, DP, total, sisa, tanggal & jam.
		`CREATE TABLE IF NOT EXISTS reservations (
			id               CHAR(26) PRIMARY KEY,
			outlet_id        CHAR(26) NOT NULL,
			customer_name    VARCHAR(150) NOT NULL,
			customer_phone   VARCHAR(40) DEFAULT '',
			pax              INT DEFAULT 1,
			items            JSONB DEFAULT '[]',
			subtotal         DECIMAL(15,2) DEFAULT 0,
			down_payment     DECIMAL(15,2) DEFAULT 0,
			total            DECIMAL(15,2) DEFAULT 0,
			reservation_date DATE,
			reservation_time VARCHAR(5) DEFAULT '',
			status           VARCHAR(20) DEFAULT 'pending',
			notes            TEXT DEFAULT '',
			source           VARCHAR(20) DEFAULT 'admin',
			created_at       TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at       TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reservations_outlet ON reservations(outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reservations_date ON reservations(reservation_date)`,
		`CREATE INDEX IF NOT EXISTS idx_reservations_status ON reservations(status)`,
		// Nama kasir & pemesan (kini dikirim app POS). orderer_name = label "Pemesan"
		// gabungan seperti di struk; cashier_name sudah ada sebelumnya.
		`ALTER TABLE cloud_transactions ADD COLUMN IF NOT EXISTS orderer_name VARCHAR(150) DEFAULT ''`,
		`ALTER TABLE cloud_orders ADD COLUMN IF NOT EXISTS orderer_name VARCHAR(150) DEFAULT ''`,
		`ALTER TABLE cloud_orders ADD COLUMN IF NOT EXISTS created_by VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE cloud_orders ADD COLUMN IF NOT EXISTS customer_phone VARCHAR(40) DEFAULT ''`,
	}

	for _, m := range settingsMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Settings migration skipped: %v", err)
		}
	}

	// Backfill rincian pembayaran untuk transaksi lama (klien lama tanpa payments[]):
	// satu baris per transaksi dari payment_method + total_amount header. Idempoten —
	// hanya untuk transaksi yang belum punya baris di transaction_payments. Tanpa ini,
	// rekap per-metode (yang kini membaca dari transaction_payments) akan kehilangan data historis.
	if _, err := DB.Exec(`
		INSERT INTO transaction_payments (transaction_id, outlet_id, payment_method, amount, created_at)
		SELECT t.id, t.outlet_id, COALESCE(NULLIF(t.payment_method, ''), 'other'), t.total_amount, t.created_at
		FROM cloud_transactions t
		WHERE NOT EXISTS (SELECT 1 FROM transaction_payments tp WHERE tp.transaction_id = t.id)`); err != nil {
		log.Printf("Backfill transaction_payments skipped: %v", err)
	}

	// Backfill slug outlet (dari nama, dijamin unik) untuk URL reservasi publik.
	backfillOutletSlugs()

	// Grant settings permissions to all roles that have users.manage or users.update (admin-level roles)
	{
		rows, err := DB.Query("SELECT DISTINCT role FROM role_permissions WHERE permission IN ('users.manage', 'users.update')")
		if err == nil {
			var roles []string
			for rows.Next() {
				var r string
				rows.Scan(&r)
				roles = append(roles, r)
			}
			rows.Close()
			for _, r := range roles {
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, 'settings.view') ON CONFLICT DO NOTHING", r)
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, 'settings.update') ON CONFLICT DO NOTHING", r)
			}
		}
	}

	// Ensure superadmin has all CRUD permissions in DB
	allPerms := []string{
		"dashboard", "dashboard.manager",
		"outlets.view", "outlets.create", "outlets.update", "outlets.delete",
		"products.view", "products.create", "products.update", "products.delete",
		"reports",
		"procurement.view", "procurement.submit", "procurement.approve", "procurement.purchasing", "procurement.finance", "procurement.work_units", "procurement.vendors",
		"warehouse.view", "warehouse.create", "warehouse.update", "warehouse.delete",
		"users.view", "users.create", "users.update", "users.delete",
		"settings.view", "settings.update",
	}
	for _, p := range allPerms {
		DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('superadmin', $1) ON CONFLICT DO NOTHING", p)
	}

	// Vendors table
	vendorMigrations := []string{
		`CREATE TABLE IF NOT EXISTS vendors (
			id CHAR(26) PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			phone VARCHAR(50) DEFAULT '',
			email VARCHAR(200) DEFAULT '',
			address TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS bank_name VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS account_number VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE vendors ADD COLUMN IF NOT EXISTS account_holder VARCHAR(200) DEFAULT ''`,
	}
	for _, m := range vendorMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Vendor migration skipped: %v", err)
		}
	}

	// Seed procurement.vendors permission for admin/manager roles
	{
		rows, err := DB.Query("SELECT DISTINCT role FROM role_permissions WHERE permission = 'procurement.work_units'")
		if err == nil {
			for rows.Next() {
				var r string
				rows.Scan(&r)
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, 'procurement.vendors') ON CONFLICT DO NOTHING", r)
			}
			rows.Close()
		}
	}

	// Add vendor_id FK to purchase_requests
	vendorPurchaseMigrations := []string{
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS vendor_id CHAR(26) REFERENCES vendors(id) ON DELETE SET NULL`,
	}
	for _, m := range vendorPurchaseMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Vendor-purchase migration skipped: %v", err)
		}
	}

	// Add invoice_number to purchase_requests
	if _, err := DB.Exec(`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS invoice_number TEXT NOT NULL DEFAULT ''`); err != nil {
		log.Printf("Invoice number migration skipped: %v", err)
	}

	// Bank accounts table
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS bank_accounts (
		id CHAR(26) PRIMARY KEY,
		bank_name VARCHAR(100) NOT NULL,
		account_number VARCHAR(50) NOT NULL,
		account_holder VARCHAR(200) NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
		updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
	)`); err != nil {
		log.Printf("Bank accounts migration skipped: %v", err)
	}

	// Role scope — allow roles to be scoped to specific work units
	roleScopeMigrations := []string{
		`ALTER TABLE roles ADD COLUMN IF NOT EXISTS scope_type VARCHAR(10) NOT NULL DEFAULT 'all'`,
		`CREATE TABLE IF NOT EXISTS role_work_unit_scope (
			role VARCHAR(50) NOT NULL REFERENCES roles(name) ON DELETE CASCADE,
			work_unit_id CHAR(26) NOT NULL REFERENCES work_units(id) ON DELETE CASCADE,
			PRIMARY KEY (role, work_unit_id)
		)`,
		// Add ON UPDATE CASCADE so role renames propagate without FK violations
		`ALTER TABLE role_work_unit_scope DROP CONSTRAINT IF EXISTS role_work_unit_scope_role_fkey`,
		`ALTER TABLE role_work_unit_scope ADD CONSTRAINT role_work_unit_scope_role_fkey
			FOREIGN KEY (role) REFERENCES roles(name) ON DELETE CASCADE ON UPDATE CASCADE`,
	}
	for _, m := range roleScopeMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Role scope migration skipped: %v", err)
		}
	}

	log.Printf("Ran %d migrations successfully", len(migrations))

	// Payment histories — partial payment support
	paymentHistMigrations := []string{
		`CREATE TABLE IF NOT EXISTS payment_histories (
			id CHAR(26) PRIMARY KEY,
			purchase_request_id CHAR(26) NOT NULL REFERENCES purchase_requests(id) ON DELETE CASCADE,
			amount DECIMAL(15,2) NOT NULL,
			payment_proof TEXT NOT NULL DEFAULT '',
			payment_account_dest VARCHAR(300) NOT NULL DEFAULT '',
			payment_account_source VARCHAR(300) NOT NULL DEFAULT '',
			payment_notes TEXT NOT NULL DEFAULT '',
			paid_by VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_histories_pr ON payment_histories(purchase_request_id)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS paid_amount DECIMAL(15,2) NOT NULL DEFAULT 0`,
	}
	for _, m := range paymentHistMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Payment histories migration skipped: %v", err)
		}
	}

	// Warehouse / inventory module — referenced by services/warehouse.go but
	// previously created by hand on dev DBs (never had migration code).
	warehouseMigrations := []string{
		`CREATE TABLE IF NOT EXISTS stock_item_categories (
			id CHAR(26) PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS stock_items (
			id CHAR(26) PRIMARY KEY,
			code VARCHAR(50) NOT NULL UNIQUE,
			name VARCHAR(200) NOT NULL,
			category VARCHAR(100) DEFAULT '',
			base_unit VARCHAR(20) NOT NULL DEFAULT 'pcs',
			dist_unit VARCHAR(20) NOT NULL DEFAULT 'pcs',
			dist_ratio DECIMAL(15,4) NOT NULL DEFAULT 1,
			dist_unit_label VARCHAR(50) DEFAULT '',
			avg_cost DECIMAL(15,2) NOT NULL DEFAULT 0,
			min_stock DECIMAL(15,4) NOT NULL DEFAULT 0,
			notes TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS warehouses (
			id CHAR(26) PRIMARY KEY,
			code VARCHAR(50) NOT NULL UNIQUE,
			name VARCHAR(200) NOT NULL,
			type VARCHAR(10) NOT NULL DEFAULT 'central',
			outlet_id CHAR(26) REFERENCES outlets(id) ON DELETE SET NULL,
			address TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			notes TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_warehouses_unique_outlet_warehouse
			ON warehouses(outlet_id) WHERE type = 'outlet'`,
		`CREATE TABLE IF NOT EXISTS stock_ledger (
			id CHAR(26) PRIMARY KEY,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
			qty_base DECIMAL(15,4) NOT NULL DEFAULT 0,
			avg_cost DECIMAL(15,2) NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(item_id, warehouse_id)
		)`,
		// Per-warehouse minimum stock (reorder level) — central & each outlet set their own.
		`ALTER TABLE stock_ledger ADD COLUMN IF NOT EXISTS min_stock DECIMAL(15,4) NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS stock_batches (
			id CHAR(26) PRIMARY KEY,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
			qty_base DECIMAL(15,4) NOT NULL,
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			expiry_date DATE,
			ref_id VARCHAR(50) DEFAULT '',
			ref_type VARCHAR(30) DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_batches_fifo ON stock_batches(item_id, warehouse_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS stock_movements (
			id CHAR(26) PRIMARY KEY,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
			movement_type VARCHAR(30) NOT NULL,
			qty_base DECIMAL(15,4) NOT NULL,
			qty_dist DECIMAL(15,4),
			unit_used VARCHAR(20) DEFAULT '',
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			balance_after DECIMAL(15,4),
			ref_id VARCHAR(50),
			ref_type VARCHAR(30) DEFAULT '',
			ref_number VARCHAR(50) DEFAULT '',
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			expiry_date DATE,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_movements_item_wh ON stock_movements(item_id, warehouse_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_movements_ref ON stock_movements(ref_type, ref_number)`,
		`CREATE TABLE IF NOT EXISTS stock_transfers (
			id CHAR(26) PRIMARY KEY,
			transfer_number VARCHAR(30) NOT NULL UNIQUE,
			from_warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			to_warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			status VARCHAR(20) NOT NULL DEFAULT 'draft',
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			approved_by VARCHAR(100),
			approved_at TIMESTAMP,
			sent_by VARCHAR(100),
			sent_at TIMESTAMP,
			received_by VARCHAR(100),
			received_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS stock_transfer_items (
			id CHAR(26) PRIMARY KEY,
			transfer_id CHAR(26) NOT NULL REFERENCES stock_transfers(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_dist DECIMAL(15,4) NOT NULL,
			qty_base DECIMAL(15,4) NOT NULL,
			unit_used VARCHAR(20) DEFAULT '',
			received_qty_base DECIMAL(15,4),
			notes TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS product_recipes (
			id CHAR(26) PRIMARY KEY,
			product_id CHAR(26) NOT NULL,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			qty_dist DECIMAL(15,4) NOT NULL,
			qty_base DECIMAL(15,4) NOT NULL,
			unit_used VARCHAR(20) DEFAULT '',
			visibility VARCHAR(10) NOT NULL DEFAULT 'public',
			notes TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_product_recipes_product ON product_recipes(product_id)`,
		`CREATE TABLE IF NOT EXISTS recipe_masters (
			id CHAR(26) PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			description TEXT DEFAULT '',
			visibility VARCHAR(10) NOT NULL DEFAULT 'public',
			instructions TEXT DEFAULT '',
			total_time INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS recipe_items (
			id CHAR(26) PRIMARY KEY,
			recipe_master_id CHAR(26) NOT NULL REFERENCES recipe_masters(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			qty_base DECIMAL(15,4) NOT NULL,
			notes TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS recipe_outlet_access (
			recipe_master_id CHAR(26) NOT NULL REFERENCES recipe_masters(id) ON DELETE CASCADE,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			PRIMARY KEY (recipe_master_id, outlet_id)
		)`,
		`CREATE TABLE IF NOT EXISTS stock_item_recipes (
			id CHAR(26) PRIMARY KEY,
			parent_item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			child_item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			qty_base DECIMAL(15,4) NOT NULL,
			notes TEXT DEFAULT '',
			visibility VARCHAR(10) NOT NULL DEFAULT 'public'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_item_recipes_parent ON stock_item_recipes(parent_item_id)`,
		`CREATE TABLE IF NOT EXISTS stock_wastes (
			id CHAR(26) PRIMARY KEY,
			waste_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_base DECIMAL(15,4) NOT NULL,
			qty_dist DECIMAL(15,4),
			unit_used VARCHAR(20) DEFAULT '',
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			total_cost DECIMAL(15,2) NOT NULL DEFAULT 0,
			reason VARCHAR(20) DEFAULT '',
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		// Link cloud_products to the warehouse module (stock type, single-item link, master recipe)
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS stock_type VARCHAR(10) NOT NULL DEFAULT 'none'`,
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS linked_stock_item_id CHAR(26) REFERENCES stock_items(id) ON DELETE SET NULL`,
		`ALTER TABLE cloud_products ADD COLUMN IF NOT EXISTS recipe_master_id CHAR(26) REFERENCES recipe_masters(id) ON DELETE SET NULL`,
		// Penerimaan Barang (Goods Receipt / GRN) — dokumen stok masuk ke gudang,
		// opsional merujuk No. Pengadaan. Header + baris; tiap baris juga menghasilkan
		// satu stock_movement (purchase_in) + batch lewat applyMovement.
		`CREATE TABLE IF NOT EXISTS goods_receipts (
			id CHAR(26) PRIMARY KEY,
			grn_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			vendor_name VARCHAR(200) DEFAULT '',
			po_ref VARCHAR(50) DEFAULT '',
			purchase_request_id CHAR(26) DEFAULT NULL,
			notes TEXT DEFAULT '',
			total_cost DECIMAL(15,2) NOT NULL DEFAULT 0,
			received_by VARCHAR(100) DEFAULT '',
			received_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_goods_receipts_wh ON goods_receipts(warehouse_id, received_at DESC)`,
		`CREATE TABLE IF NOT EXISTS goods_receipt_items (
			id CHAR(26) PRIMARY KEY,
			receipt_id CHAR(26) NOT NULL REFERENCES goods_receipts(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			item_name VARCHAR(200) NOT NULL DEFAULT '',
			qty_base DECIMAL(15,4) NOT NULL,
			qty_dist DECIMAL(15,4) NOT NULL DEFAULT 0,
			unit_used VARCHAR(20) DEFAULT '',
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			subtotal DECIMAL(15,2) NOT NULL DEFAULT 0,
			expiry_date DATE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_goods_receipt_items_receipt ON goods_receipt_items(receipt_id)`,
	}
	for _, m := range warehouseMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Warehouse migration skipped: %v", err)
		}
	}

	// Seed missing permissions for existing roles
	{
		// Warehouse CRUD permissions — were previously seeded as warehouse.manage (now converted)
		warehousePerms := []string{"warehouse.view", "warehouse.create", "warehouse.update", "warehouse.delete"}
		warehouseRoles := []string{"superadmin", "admin", "manager"}
		for _, role := range warehouseRoles {
			for _, perm := range warehousePerms {
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", role, perm)
			}
		}

		// Procurement full permissions for admin
		procurementPerms := []string{"procurement.view", "procurement.submit", "procurement.approve", "procurement.purchasing", "procurement.finance"}
		for _, perm := range procurementPerms {
			DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('admin', $1) ON CONFLICT DO NOTHING", perm)
		}
		// Manager gets procurement.view so the menu shows
		DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('manager', 'procurement.view') ON CONFLICT DO NOTHING")
	}

	// Migrate .manage → granular CRUD permissions (one-time, idempotent)
	{
		manageToCRUD := map[string][]string{
			"outlets.manage":  {"outlets.create", "outlets.update", "outlets.delete"},
			"products.manage": {"products.create", "products.update", "products.delete"},
			"warehouse.manage": {"warehouse.create", "warehouse.update", "warehouse.delete"},
			"users.manage":    {"users.create", "users.update", "users.delete"},
			"settings.manage": {"settings.update"},
		}
		for oldPerm, newPerms := range manageToCRUD {
			var count int
			DB.QueryRow("SELECT COUNT(*) FROM role_permissions WHERE permission = $1", oldPerm).Scan(&count)
			if count > 0 {
				rows, err := DB.Query("SELECT role FROM role_permissions WHERE permission = $1", oldPerm)
				if err == nil {
					var roles []string
					for rows.Next() {
						var r string
						rows.Scan(&r)
						roles = append(roles, r)
					}
					rows.Close()
					for _, r := range roles {
						for _, np := range newPerms {
							DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, np)
						}
					}
					DB.Exec("DELETE FROM role_permissions WHERE permission = $1", oldPerm)
				}
				log.Printf("Migrated '%s' → %v for %d roles", oldPerm, newPerms, count)
			}
		}
	}

	// Ensure superadmin has all CRUD permissions (runs after conversion)
	{
		allPerms := []string{
			"dashboard", "dashboard.manager",
			"outlets.view", "outlets.create", "outlets.update", "outlets.delete",
			"products.view", "products.create", "products.update", "products.delete",
			"reports",
			"procurement.view", "procurement.submit", "procurement.approve", "procurement.purchasing", "procurement.finance", "procurement.work_units", "procurement.vendors",
			"warehouse.view", "warehouse.create", "warehouse.update", "warehouse.delete",
			"users.view", "users.create", "users.update", "users.delete",
			"settings.view", "settings.update",
		}
		for _, p := range allPerms {
			DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('superadmin', $1) ON CONFLICT DO NOTHING", p)
		}
	}

	// Access logs (login/access history — superadmin only)
	accessLogMigrations := []string{
		`CREATE TABLE IF NOT EXISTS access_logs (
			id CHAR(26) PRIMARY KEY,
			username VARCHAR(100) NOT NULL DEFAULT '',
			role VARCHAR(50) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'success',
			ip VARCHAR(64) NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			browser VARCHAR(80) NOT NULL DEFAULT '',
			os VARCHAR(80) NOT NULL DEFAULT '',
			device VARCHAR(40) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_access_logs_created ON access_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_access_logs_username ON access_logs(username)`,
	}
	for _, m := range accessLogMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Access logs migration skipped: %v", err)
		}
	}

	// ── Granular per-submenu permission migration (v2) ──────────────────────
	// Split coarse keys into per-submenu granular keys so existing roles keep
	// equivalent access. Replace-style: the old key is deleted after expansion.
	granularSplit := map[string][]string{
		"reports": {
			"reports.sales.view", "reports.product_sales.view", "reports.ledger.view",
			"reports.cashflow.view", "reports.pnl.view", "reports.balance.view",
			"reports.tax.view", "reports.void.view", "reports.discount.view",
		},
		"warehouse.view": {
			"warehouse_dashboard.view", "warehouses.view", "stockitems.view",
			"stocktransfers.view", "stockwastes.view", "stockledger.view", "recipes.view",
		},
		"warehouse.create": {
			"warehouses.create", "stockitems.create", "stocktransfers.create",
			"stockwastes.create", "stockledger.adjust", "recipes.create",
		},
		"warehouse.update": {"warehouses.update", "stockitems.update", "stocktransfers.update", "stocktransfers.approve", "stocktransfers.receive", "recipes.update"},
		"warehouse.delete": {"warehouses.delete", "stockitems.delete", "recipes.delete"},

		"procurement.view":       {"procurement.dashboard.view", "procurement.requests.view", "vendors.view"},
		"procurement.submit":     {"procurement.requests.submit"},
		"procurement.approve":    {"procurement.requests.approve"},
		"procurement.purchasing": {"procurement.requests.purchasing"},
		"procurement.finance":    {"finance.payments.view", "finance.bank.view", "finance.bank.create", "finance.bank.update", "finance.bank.delete"},
		"procurement.vendors":    {"vendors.create", "vendors.update", "vendors.delete"},
		"procurement.work_units": {"workunits.view", "workunits.create", "workunits.update", "workunits.delete"},

		"settings.view":   {"settings.company.view", "settings.timezone.view", "settings.tax.view"},
		"settings.update": {"settings.company.update", "settings.timezone.update", "settings.tax.update"},
		"settings.manage": {"settings.company.view", "settings.timezone.view", "settings.tax.view", "settings.company.update", "settings.timezone.update", "settings.tax.update"},

		"outlets.manage":  {"outlets.create", "outlets.update", "outlets.delete"},
		"products.manage": {"products.create", "products.update", "products.delete"},
		"users.manage":    {"users.create", "users.update", "users.delete", "roles.create", "roles.update", "roles.delete"},
	}
	for old, news := range granularSplit {
		var count int
		DB.QueryRow("SELECT COUNT(*) FROM role_permissions WHERE permission = $1", old).Scan(&count)
		if count == 0 {
			continue
		}
		rows, err := DB.Query("SELECT role FROM role_permissions WHERE permission = $1", old)
		if err != nil {
			continue
		}
		var rls []string
		for rows.Next() {
			var r string
			rows.Scan(&r)
			rls = append(rls, r)
		}
		rows.Close()
		for _, r := range rls {
			for _, np := range news {
				DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING", r, np)
			}
		}
		DB.Exec("DELETE FROM role_permissions WHERE permission = $1", old)
		log.Printf("Granular migration: '%s' → %d keys for %d roles", old, len(news), count)
	}

	// Additive: split 'users' module into users (admins) + roles. Keep users.* AND mirror to roles.*.
	for u, rl := range map[string]string{
		"users.view": "roles.view", "users.create": "roles.create",
		"users.update": "roles.update", "users.delete": "roles.delete",
	} {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT role, $2 FROM role_permissions WHERE permission = $1
			ON CONFLICT DO NOTHING`, u, rl)
	}

	// Ensure system 'admin' role always holds the full granular permission set.
	adminAll := []string{
		"dashboard",
		"outlets.view", "outlets.create", "outlets.update", "outlets.delete",
		"workunits.view", "workunits.create", "workunits.update", "workunits.delete",
		"appfiles.view", "appfiles.create", "appfiles.delete",
		"assets.view", "assets.create", "assets.update", "assets.delete",
		"products.view", "products.create", "products.update", "products.delete",
		"reservations.view", "reservations.create", "reservations.update", "reservations.delete",
		"reports.sales.view", "reports.product_sales.view", "reports.ledger.view",
		"reports.cashflow.view", "reports.pnl.view", "reports.balance.view",
		"reports.tax.view", "reports.void.view", "reports.titipan.view", "reports.discount.view",
		"cashier_shifts.view", "shift_reconciliation.view",
		"finance.payments.view", "finance.bank.view", "finance.bank.create", "finance.bank.update", "finance.bank.delete",
		"procurement.dashboard.view", "procurement.requests.view", "procurement.requests.submit",
		"procurement.requests.approve", "procurement.requests.purchasing",
		"vendors.view", "vendors.create", "vendors.update", "vendors.delete",
		"users.view", "users.create", "users.update", "users.delete",
		"roles.view", "roles.create", "roles.update", "roles.delete",
		"access_logs.view",
		"warehouse_dashboard.view",
		"warehouses.view", "warehouses.create", "warehouses.update", "warehouses.delete",
		"stockitems.view", "stockitems.create", "stockitems.update", "stockitems.delete",
		"stocktransfers.view", "stocktransfers.create", "stocktransfers.update",
		"stockwastes.view", "stockwastes.create",
		"stockledger.view", "stockledger.adjust",
		"recipes.view", "recipes.create", "recipes.update", "recipes.delete",
		"settings.company.view", "settings.company.update",
		"settings.timezone.view", "settings.timezone.update",
		"settings.tax.view", "settings.tax.update",
		"devices.view",
	}
	for _, p := range adminAll {
		DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('admin', $1) ON CONFLICT DO NOTHING", p)
	}

	// ── Heartbeat perangkat (App POS → Cloud) ───────────────────────────────
	// Snapshot terakhir per outlet (upsert) + histori ringkas untuk tren.
	deviceMigrations := []string{
		`CREATE TABLE IF NOT EXISTS device_heartbeats (
			outlet_id        CHAR(26) PRIMARY KEY,
			app_version      VARCHAR(50)  NOT NULL DEFAULT '',
			battery          INT          NOT NULL DEFAULT -1,
			battery_state    VARCHAR(20)  NOT NULL DEFAULT '',
			model            VARCHAR(120) NOT NULL DEFAULT '',
			os               VARCHAR(120) NOT NULL DEFAULT '',
			storage_total_mb BIGINT       NOT NULL DEFAULT 0,
			storage_free_mb  BIGINT       NOT NULL DEFAULT 0,
			network_online   BOOLEAN      NOT NULL DEFAULT false,
			pending_sync     INT          NOT NULL DEFAULT 0,
			last_sync_at     TIMESTAMP,
			printers         JSONB        NOT NULL DEFAULT '[]',
			reported_at      TIMESTAMP,
			received_at      TIMESTAMP    NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS device_heartbeat_logs (
			id              BIGSERIAL PRIMARY KEY,
			outlet_id       CHAR(26) NOT NULL,
			battery         INT      NOT NULL DEFAULT -1,
			battery_state   VARCHAR(20) NOT NULL DEFAULT '',
			network_online  BOOLEAN  NOT NULL DEFAULT false,
			pending_sync    INT      NOT NULL DEFAULT 0,
			printers_online INT      NOT NULL DEFAULT 0,
			printers_total  INT      NOT NULL DEFAULT 0,
			reported_at     TIMESTAMP,
			received_at     TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_device_hb_logs_outlet ON device_heartbeat_logs(outlet_id, received_at DESC)`,
		// Telemetri RAM & CPU perangkat (App POS Android). Semua nullable —
		// hanya terkirim di Android, dan field CPU bersifat best-effort (bisa
		// tidak ada saat heartbeat pertama / diblok SELinux).
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS ram_total_mb     INTEGER`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS ram_free_mb      INTEGER`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS ram_used_percent INTEGER`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS ram_low          BOOLEAN`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_cores        INTEGER`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_used_percent REAL`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_load_1m      REAL`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_load_5m      REAL`,
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_load_15m     REAL`,
		// Sumber angka CPU: 'system' (seluruh perangkat via /proc/stat) atau 'app'
		// (proses POS saja, fallback saat /proc/stat diblok SELinux).
		`ALTER TABLE device_heartbeats ADD COLUMN IF NOT EXISTS cpu_source VARCHAR(10)`,
	}
	for _, m := range deviceMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Device heartbeat migration skipped: %v", err)
		}
	}

	// ── Pajak per-outlet ────────────────────────────────────────────────────
	// Tiap outlet punya pengaturan pajaknya sendiri (bukan lagi satu nilai global).
	outletTaxMigrations := []string{
		`ALTER TABLE outlets ADD COLUMN IF NOT EXISTS tax_enabled BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE outlets ADD COLUMN IF NOT EXISTS tax_rate DECIMAL(5,2) NOT NULL DEFAULT 0`,
		`ALTER TABLE outlets ADD COLUMN IF NOT EXISTS tax_name VARCHAR(100) NOT NULL DEFAULT 'Pajak Restoran (PB1)'`,
	}
	for _, m := range outletTaxMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Outlet tax migration skipped: %v", err)
		}
	}
	// Backfill sekali: salin pajak global lama ke SEMUA outlet agar perilaku tetap
	// sama di hari pertama; setelah ini nilai global tak lagi dipakai untuk transaksi.
	{
		var seeded bool
		DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM app_settings WHERE key='tax_per_outlet_seeded')`).Scan(&seeded)
		if !seeded {
			var gEnabled, gRate, gName string
			DB.QueryRow(`SELECT COALESCE((SELECT value FROM app_settings WHERE key='tax_enabled'),'false')`).Scan(&gEnabled)
			DB.QueryRow(`SELECT COALESCE((SELECT value FROM app_settings WHERE key='tax_rate'),'0')`).Scan(&gRate)
			DB.QueryRow(`SELECT COALESCE(NULLIF((SELECT value FROM app_settings WHERE key='tax_name'),''),'Pajak Restoran (PB1)')`).Scan(&gName)
			if _, err := DB.Exec(
				`UPDATE outlets SET tax_enabled = $1, tax_rate = $2::numeric, tax_name = $3`,
				gEnabled == "true", gRate, gName,
			); err != nil {
				log.Printf("Outlet tax backfill skipped: %v", err)
			} else {
				DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('tax_per_outlet_seeded','true')
					ON CONFLICT (key) DO UPDATE SET value = 'true'`)
				log.Printf("Pajak per-outlet: backfill global → semua outlet selesai")
			}
		}
	}

	// ── Void item (hapus item dari order belum bayar) ──────────────────────
	// Audit terpisah dari void transaksi: app POS mengirim entityType
	// "order_item_void" saat satu item dihapus dari order (di-gate otorisasi
	// Manager/SVP). Order-nya sendiri tetap di-upsert via entityType "order".
	itemVoidMigrations := []string{
		`CREATE TABLE IF NOT EXISTS order_item_voids (
			id            CHAR(26) PRIMARY KEY,
			local_id      VARCHAR(50) NOT NULL,
			outlet_id     CHAR(26) NOT NULL REFERENCES outlets(id),
			order_id      VARCHAR(50) NOT NULL DEFAULT '',
			table_number  VARCHAR(50) NOT NULL DEFAULT '',
			product_name  VARCHAR(200) NOT NULL DEFAULT '',
			category_id   VARCHAR(50) NOT NULL DEFAULT '',
			qty           DECIMAL(10,2) NOT NULL DEFAULT 0,
			price         DECIMAL(15,2) NOT NULL DEFAULT 0,
			subtotal      DECIMAL(15,2) NOT NULL DEFAULT 0,
			waiter_name   VARCHAR(200) NOT NULL DEFAULT '',
			voided_by     VARCHAR(200) NOT NULL DEFAULT '',
			void_reason   TEXT NOT NULL DEFAULT '',
			voided_at     TIMESTAMP,
			created_at    TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			synced_at     TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE (outlet_id, local_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_item_voids_outlet ON order_item_voids(outlet_id, voided_at DESC)`,
	}
	for _, m := range itemVoidMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Order item void migration skipped: %v", err)
		}
	}

	// ── Index tambahan untuk pola WHERE yang sering dipakai laporan & sync ──
	// Tabel-tabel ini difilter per (outlet, tanggal) / status / vendor di hampir
	// semua laporan tapi sebelumnya hanya punya index satu-kolom (atau tidak ada).
	perfIndexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_cloud_transactions_outlet_created ON cloud_transactions(outlet_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_outlet_created ON cloud_orders(outlet_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_paid_at ON purchase_requests(paid_at)`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_vendor ON purchase_requests(vendor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_cash_movements_created ON cloud_cash_movements(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_categories_updated ON cloud_categories(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_transfers_status ON stock_transfers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_transfers_from_wh ON stock_transfers(from_warehouse_id)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_transfers_to_wh ON stock_transfers(to_warehouse_id)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_wastes_wh_created ON stock_wastes(warehouse_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_products_outlet_name ON cloud_products(outlet_id, name)`,
		// Nomor pengajuan pengadaan harus unik (generator MAX()+1 bisa balapan;
		// call site melakukan retry saat index ini menolak duplikat).
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_requests_number ON purchase_requests(request_number)`,
		// order_id dijoin dari cloud_transactions ke cloud_orders (pax dashboard,
		// cashier lookup laporan) — sebelumnya tanpa index → seq scan.
		`CREATE INDEX IF NOT EXISTS idx_cloud_transactions_order ON cloud_transactions(order_id)`,
	}
	for _, m := range perfIndexes {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Perf index migration skipped: %v", err)
		}
	}

	// ── Backfill sekali: normalisasi kembalian di transaction_payments ─────
	// App lama mengirim nominal tunai yang diserahkan pelanggan (termasuk
	// kembalian) sebagai baris pembayaran, menggelembungkan rekap tunai.
	// Kurangi kelebihan (sum(pembayaran) − total transaksi) dari baris tunai.
	// Save-path baru sudah menormalisasi, jadi cukup sekali untuk data lama.
	{
		var done bool
		DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM app_settings WHERE key='tp_change_normalized')`).Scan(&done)
		if !done {
			res, err := DB.Exec(`
				WITH ex AS (
					SELECT tp.transaction_id, SUM(tp.amount) - MAX(t.total_amount) AS excess
					FROM transaction_payments tp
					JOIN cloud_transactions t ON t.id = tp.transaction_id
					WHERE t.total_amount > 0
					GROUP BY tp.transaction_id
					HAVING SUM(tp.amount) > MAX(t.total_amount) + 0.01
				)
				UPDATE transaction_payments tp
				SET amount = GREATEST(0, tp.amount - ex.excess)
				FROM ex
				WHERE tp.transaction_id = ex.transaction_id
				  AND lower(tp.payment_method) = 'cash'
				  AND tp.id = (SELECT MIN(tp2.id) FROM transaction_payments tp2
					WHERE tp2.transaction_id = ex.transaction_id AND lower(tp2.payment_method) = 'cash')`)
			if err != nil {
				log.Printf("Backfill kembalian transaction_payments skipped: %v", err)
			} else {
				n, _ := res.RowsAffected()
				DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('tp_change_normalized','true')
					ON CONFLICT (key) DO UPDATE SET value = 'true'`)
				log.Printf("Backfill kembalian transaction_payments: %d baris dinormalisasi", n)
			}
		}
	}

	// Hak akses monitoring perangkat → superadmin, teknisi, dan semua role manager%.
	DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('superadmin', 'devices.view') ON CONFLICT DO NOTHING")
	DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ('teknisi', 'devices.view') ON CONFLICT DO NOTHING")
	if mrows, err := DB.Query("SELECT name FROM roles WHERE name = 'manager' OR name LIKE 'manager %'"); err == nil {
		var managerRoles []string
		for mrows.Next() {
			var n string
			if mrows.Scan(&n) == nil {
				managerRoles = append(managerRoles, n)
			}
		}
		mrows.Close()
		for _, r := range managerRoles {
			DB.Exec("INSERT INTO role_permissions (role, permission) VALUES ($1, 'devices.view') ON CONFLICT DO NOTHING", r)
		}
	}

	// ── Meja Titipan (parked check) ─────────────────────────────────────────
	// Order titipan (is_holding) = barang menggantung yang belum dibayar → harus
	// dikecualikan dari omzet/belum-lunas. Log titip/tarik (audit) disimpan
	// append-only untuk pengawasan "kebocoran".
	titipanMigrations := []string{
		`ALTER TABLE cloud_orders ADD COLUMN IF NOT EXISTS is_holding BOOLEAN NOT NULL DEFAULT false`,
		`CREATE TABLE IF NOT EXISTS order_item_titipan_logs (
			id            BIGSERIAL PRIMARY KEY,
			outlet_id     CHAR(26) NOT NULL,
			local_id      VARCHAR(50) NOT NULL,   -- order_items.id
			action        VARCHAR(10) NOT NULL,   -- 'park' | 'pull'
			product_name  TEXT    NOT NULL DEFAULT '',
			qty           NUMERIC NOT NULL DEFAULT 0,
			price         NUMERIC NOT NULL DEFAULT 0,
			subtotal      NUMERIC NOT NULL DEFAULT 0,
			category_id   VARCHAR(50) NOT NULL DEFAULT '',
			source_table  VARCHAR(50) NOT NULL DEFAULT '',
			target_table  VARCHAR(50) NOT NULL DEFAULT '',
			performed_by  TEXT NOT NULL DEFAULT '',
			performed_at  TIMESTAMP,
			created_at    TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		// Idempoten terhadap retry outbox: satu event (item, aksi, waktu) unik.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_titipan_event_unique
			ON order_item_titipan_logs(outlet_id, local_id, action, performed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_titipan_outlet ON order_item_titipan_logs(outlet_id, performed_at DESC)`,
	}
	for _, m := range titipanMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Titipan migration skipped: %v", err)
		}
	}

	// ── Rekonsiliasi shift: penyesuaian pendapatan "ikuti versi kasir" ──────
	// Catatan tersendiri untuk setiap penyesuaian, agar bisa dikembalikan bila
	// keliru. Nilai penyesuaian direalisasikan sebagai satu transaksi bertanda
	// payment_method='adjustment' (mengalir ke semua laporan), yang bisa dihapus
	// saat revert.
	reconMigrations := []string{
		`CREATE TABLE IF NOT EXISTS shift_revenue_adjustments (
			id                BIGSERIAL PRIMARY KEY,
			outlet_id         CHAR(26) NOT NULL,
			shift_id          CHAR(26) NOT NULL,
			shift_date        DATE,
			cashier_sales     NUMERIC NOT NULL DEFAULT 0,
			cloud_sales       NUMERIC NOT NULL DEFAULT 0,
			adjustment_amount NUMERIC NOT NULL DEFAULT 0,
			txn_id            CHAR(26),
			txn_local_id      VARCHAR(60),
			note              TEXT NOT NULL DEFAULT '',
			status            VARCHAR(12) NOT NULL DEFAULT 'applied', -- applied | reverted
			created_by        TEXT NOT NULL DEFAULT '',
			created_at        TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
			reverted_by       TEXT NOT NULL DEFAULT '',
			reverted_at       TIMESTAMP
		)`,
		// Hanya boleh ada satu penyesuaian AKTIF per shift.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_shift_adj_active ON shift_revenue_adjustments(shift_id) WHERE status='applied'`,
	}
	for _, m := range reconMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Reconciliation migration skipped: %v", err)
		}
	}

	// ── CCTV: konfigurasi kamera per outlet (RELAY-ONLY, zero-retention) ─────
	// Cloud hanya menyimpan CARA menjangkau DVR (host/channel/kredensial RTSP
	// terenkripsi). TIDAK ada footage/rekaman di cloud — video hanya di-relay
	// go2rtc dari DVR ke browser. Rekaman tetap 100% di DVR outlet.
	cctvMigrations := []string{
		`CREATE TABLE IF NOT EXISTS outlet_cameras (
			id                CHAR(26) PRIMARY KEY,
			outlet_id         CHAR(26) NOT NULL REFERENCES outlets(id),
			name              VARCHAR(100) NOT NULL,
			brand             VARCHAR(20) NOT NULL DEFAULT 'hikvision',
			host              VARCHAR(120) NOT NULL,
			port              INTEGER NOT NULL DEFAULT 554,
			channel           INTEGER NOT NULL DEFAULT 1,
			subtype           INTEGER NOT NULL DEFAULT 1,
			rtsp_username     VARCHAR(80) NOT NULL DEFAULT '',
			rtsp_password_enc TEXT NOT NULL DEFAULT '',
			is_active         BOOLEAN NOT NULL DEFAULT true,
			sort_order        INTEGER NOT NULL DEFAULT 0,
			created_at        TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at        TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outlet_cameras_outlet ON outlet_cameras(outlet_id, sort_order)`,
	}
	for _, m := range cctvMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("CCTV migration skipped: %v", err)
		}
	}

	// ── PPIC Fase 1: par level/ROP, monitor kedaluwarsa, stock opname ────────
	// Parameter perencanaan per item-gudang + sesi opname (snapshot blind count).
	// Batch mendapat kolom "ack" supaya alert kedaluwarsa bisa disenyapkan
	// setelah ditindak tanpa menghapus jejak batch-nya.
	ppicMigrations := []string{
		`CREATE TABLE IF NOT EXISTS item_planning_params (
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
			lead_time_days INTEGER NOT NULL DEFAULT 0,
			safety_stock DECIMAL(15,4) NOT NULL DEFAULT 0,
			reorder_point DECIMAL(15,4) NOT NULL DEFAULT 0,
			par_level DECIMAL(15,4) NOT NULL DEFAULT 0,
			moq DECIMAL(15,4) NOT NULL DEFAULT 0,
			updated_by VARCHAR(100) DEFAULT '',
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			PRIMARY KEY (item_id, warehouse_id)
		)`,
		`ALTER TABLE stock_items ADD COLUMN IF NOT EXISTS shelf_life_days INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE stock_items ADD COLUMN IF NOT EXISTS default_lead_time_days INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE stock_item_categories ADD COLUMN IF NOT EXISTS is_perishable BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE stock_batches ADD COLUMN IF NOT EXISTS ppic_ack_at TIMESTAMP`,
		`ALTER TABLE stock_batches ADD COLUMN IF NOT EXISTS ppic_ack_by VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE stock_batches ADD COLUMN IF NOT EXISTS ppic_ack_note TEXT DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_stock_batches_expiry ON stock_batches(expiry_date) WHERE expiry_date IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS stock_opnames (
			id CHAR(26) PRIMARY KEY,
			opname_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			category VARCHAR(100) DEFAULT '',
			status VARCHAR(12) NOT NULL DEFAULT 'counting',
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			approved_by VARCHAR(100) DEFAULT '',
			approved_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS stock_opname_items (
			id CHAR(26) PRIMARY KEY,
			opname_id CHAR(26) NOT NULL REFERENCES stock_opnames(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_system_base DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_counted_base DECIMAL(15,4),
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			reason TEXT DEFAULT '',
			UNIQUE(opname_id, item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_opnames_wh ON stock_opnames(warehouse_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_movements_wh_created ON stock_movements(warehouse_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_movements_item_wh_created ON stock_movements(item_id, warehouse_id, created_at)`,
		// Guard idempotensi deduksi stok penjualan: lookup EXISTS per (ref_type, ref_id).
		`CREATE INDEX IF NOT EXISTS idx_stock_movements_ref_id ON stock_movements(ref_type, ref_id) WHERE ref_id IS NOT NULL`,
		// Antrian deduksi stok penjualan yang gagal (mis. stok FIFO habis): dicatat
		// agar terlihat & bisa di-retry otomatis oleh scheduler, tidak cuma di log.
		`CREATE TABLE IF NOT EXISTS stock_deduction_failures (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL,
			transaction_id VARCHAR(50) NOT NULL,
			product_local_id VARCHAR(100) NOT NULL,
			qty DECIMAL(15,4) NOT NULL,
			last_error TEXT DEFAULT '',
			attempts INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			last_attempt_at TIMESTAMP,
			UNIQUE(transaction_id, product_local_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sdf_pending ON stock_deduction_failures(status, created_at) WHERE status = 'pending'`,
	}
	for _, m := range ppicMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("PPIC migration skipped: %v", err)
		}
	}

	// ── PPIC Fase 2: demand forecast + snapshot MRP ──────────────────────────
	// Forecast per (outlet, NAMA produk, tanggal) — produk dikenali per nama,
	// konsisten dengan laporan Penjualan Produk (payload order tidak selalu
	// membawa product_id). Hasil kalkulasi MRP disimpan sebagai snapshot agar
	// keputusan beli/transfer bisa diaudit; PR/transfer hasil MRP diberi jejak
	// origin + mrp_run_id.
	ppic2Migrations := []string{
		`CREATE TABLE IF NOT EXISTS demand_forecasts (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			product_name VARCHAR(200) NOT NULL,
			forecast_date DATE NOT NULL,
			qty_system DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_manual DECIMAL(15,4),
			qty_actual DECIMAL(15,4),
			method VARCHAR(20) NOT NULL DEFAULT 'wma_dow',
			event_note TEXT DEFAULT '',
			updated_by VARCHAR(100) DEFAULT '',
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			UNIQUE(outlet_id, product_name, forecast_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_demand_forecasts_date ON demand_forecasts(forecast_date)`,
		`CREATE TABLE IF NOT EXISTS mrp_runs (
			id CHAR(26) PRIMARY KEY,
			run_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			horizon_days INTEGER NOT NULL DEFAULT 7,
			include_par BOOLEAN NOT NULL DEFAULT true,
			status VARCHAR(12) NOT NULL DEFAULT 'open',
			no_recipe_count INTEGER NOT NULL DEFAULT 0,
			no_recipe_products TEXT DEFAULT '',
			forecast_qty DECIMAL(15,4) NOT NULL DEFAULT 0,
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS mrp_run_items (
			id CHAR(26) PRIMARY KEY,
			run_id CHAR(26) NOT NULL REFERENCES mrp_runs(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			gross_req DECIMAL(15,4) NOT NULL DEFAULT 0,
			on_hand DECIMAL(15,4) NOT NULL DEFAULT 0,
			on_order DECIMAL(15,4) NOT NULL DEFAULT 0,
			in_transit DECIMAL(15,4) NOT NULL DEFAULT 0,
			safety_stock DECIMAL(15,4) NOT NULL DEFAULT 0,
			par_level DECIMAL(15,4) NOT NULL DEFAULT 0,
			moq DECIMAL(15,4) NOT NULL DEFAULT 0,
			net_req DECIMAL(15,4) NOT NULL DEFAULT 0,
			suggestion VARCHAR(10) NOT NULL DEFAULT 'none',
			qty_suggested_base DECIMAL(15,4) NOT NULL DEFAULT 0,
			source_warehouse_id CHAR(26),
			last_vendor VARCHAR(200) DEFAULT '',
			last_price_dist DECIMAL(15,2) NOT NULL DEFAULT 0,
			action_ref_type VARCHAR(20) DEFAULT '',
			action_ref_id CHAR(26),
			action_ref_number VARCHAR(40) DEFAULT '',
			UNIQUE(run_id, item_id)
		)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS origin VARCHAR(20) NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS mrp_run_id CHAR(26)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS need_by_date DATE`,
		`ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS origin VARCHAR(20) NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS mrp_run_id CHAR(26)`,
	}
	for _, m := range ppic2Migrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("PPIC fase 2 migration skipped: %v", err)
		}
	}

	// ── PPIC Fase 3: rencana produksi (MPS) + work order ─────────────────────
	// WO memposting stok lewat mekanisme movement/batch existing (production_out/
	// production_in, ref_type='work_order'); hasil produksi menjadi batch baru
	// ber-HPP aktual + expiry dari shelf_life_days item.
	ppic3Migrations := []string{
		`CREATE TABLE IF NOT EXISTS production_plans (
			id CHAR(26) PRIMARY KEY,
			plan_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			plan_date DATE NOT NULL,
			status VARCHAR(12) NOT NULL DEFAULT 'draft',
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			approved_by VARCHAR(100) DEFAULT '',
			approved_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS production_plan_items (
			id CHAR(26) PRIMARY KEY,
			plan_id CHAR(26) NOT NULL REFERENCES production_plans(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_planned_base DECIMAL(15,4) NOT NULL,
			notes TEXT DEFAULT '',
			UNIQUE(plan_id, item_id)
		)`,
		`CREATE TABLE IF NOT EXISTS work_orders (
			id CHAR(26) PRIMARY KEY,
			wo_number VARCHAR(30) NOT NULL UNIQUE,
			plan_id CHAR(26) REFERENCES production_plans(id) ON DELETE SET NULL,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_planned_base DECIMAL(15,4) NOT NULL,
			qty_actual_base DECIMAL(15,4),
			yield_pct DECIMAL(8,2),
			status VARCHAR(12) NOT NULL DEFAULT 'planned',
			started_at TIMESTAMP,
			finished_at TIMESTAMP,
			executed_by VARCHAR(100) DEFAULT '',
			cost_total DECIMAL(15,2) NOT NULL DEFAULT 0,
			cost_per_unit DECIMAL(15,2) NOT NULL DEFAULT 0,
			expiry_date DATE,
			notes TEXT DEFAULT '',
			created_by VARCHAR(100) DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS work_order_materials (
			id CHAR(26) PRIMARY KEY,
			wo_id CHAR(26) NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_plan_base DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_actual_base DECIMAL(15,4),
			cost_per_base DECIMAL(15,2) NOT NULL DEFAULT 0,
			UNIQUE(wo_id, item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_orders_wh_status ON work_orders(warehouse_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_work_orders_finished ON work_orders(finished_at)`,
	}
	for _, m := range ppic3Migrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("PPIC fase 3 migration skipped: %v", err)
		}
	}

	// ── Pelanggan: master data otomatis dari cloud_orders ───────────────────
	// Identitas utama = nomor HP (dinormalisasi 08xx); order tanpa HP dicocokkan
	// per nama (lower). Histori kunjungan TIDAK diduplikasi: cloud_orders diberi
	// customer_id sehingga histori = query orders per pelanggan.
	customerMigrations := []string{
		`CREATE TABLE IF NOT EXISTS customers (
			id CHAR(26) PRIMARY KEY,
			name VARCHAR(150) NOT NULL DEFAULT '',
			phone VARCHAR(40) NOT NULL DEFAULT '',
			notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_customers_phone ON customers(phone) WHERE phone <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_customers_name_lower ON customers(LOWER(name))`,
		`ALTER TABLE cloud_orders ADD COLUMN IF NOT EXISTS customer_id CHAR(26)`,
		`CREATE INDEX IF NOT EXISTS idx_cloud_orders_customer ON cloud_orders(customer_id)`,
	}
	for _, m := range customerMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Customer migration skipped: %v", err)
		}
	}

	// ── Projek (pembangunan / renovasi) ─────────────────────────────────────
	// Payung tipis di atas pengadaan: projek hanya memegang RAB (satu angka
	// total) + identitas, sedangkan tiap belanja tahap tetap berupa
	// purchase_requests biasa dengan seluruh alur setujui/bayar/terima yang
	// sudah ada. Sengaja TIDAK memakai parent_id/split_status — kolom itu sudah
	// dipakai untuk pecahan vendor, dan cascade status-nya (lihat
	// services/purchase.go) akan salah kalau dipinjam untuk projek.
	//
	// project_id nullable: seluruh pengadaan lama tetap valid tanpa backfill.
	projectMigrations := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id CHAR(26) PRIMARY KEY,
			project_number VARCHAR(20) NOT NULL DEFAULT '',
			name VARCHAR(200) NOT NULL,
			outlet_id CHAR(26) REFERENCES outlets(id) ON DELETE SET NULL,
			work_unit_id CHAR(26) REFERENCES work_units(id) ON DELETE SET NULL,
			pic VARCHAR(150) NOT NULL DEFAULT '',
			budget DECIMAL(15,2) NOT NULL DEFAULT 0,
			start_date DATE,
			target_date DATE,
			status VARCHAR(20) NOT NULL DEFAULT 'berjalan',
			notes TEXT NOT NULL DEFAULT '',
			created_by VARCHAR(150) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_number ON projects(project_number) WHERE project_number <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_work_unit ON projects(work_unit_id)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS project_id CHAR(26) REFERENCES projects(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_project ON purchase_requests(project_id)`,
	}
	for _, m := range projectMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Project migration skipped: %v", err)
		}
	}

	// ── Split izin laporan: Titipan ⟂ Void, Rekonsiliasi ⟂ Shift Kasir ──────
	// One-shot (marker di app_settings): role yang punya izin lama otomatis
	// diberi izin baru SEKALI supaya tidak ada yang kehilangan akses saat
	// deploy; setelah itu pencabutan oleh admin tidak ditimpa saat boot
	// berikutnya. Izin lama TIDAK dihapus (masih dipakai tab Void / halaman
	// Shift Kasir), jadi blok split-then-delete biasa tidak bisa dipakai.
	// Wajib di akhir fungsi ini — re-seed di atas berjalan tiap boot.
	var splitDone int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_split_titipan_shiftrecon'").Scan(&splitDone)
	if splitDone == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT role, 'reports.titipan.view' FROM role_permissions WHERE permission = 'reports.void.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT role, 'shift_reconciliation.view' FROM role_permissions WHERE permission = 'cashier_shifts.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_split_titipan_shiftrecon', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission split: reports.titipan.view & shift_reconciliation.view di-backfill dari izin lama")
	}

	// ── Seed izin PPIC (one-shot, marker) ───────────────────────────────────
	// Role yang saat ini boleh melihat Dashboard Gudang otomatis mendapat semua
	// izin PPIC Fase 1 SEKALI saat deploy; setelah itu pencabutan oleh admin
	// tidak ditimpa saat boot. Wajib di akhir fungsi ini (lihat catatan blok
	// split di atas: re-seed coarse-key berjalan tiap boot).
	var ppicSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_ppic_phase1'").Scan(&ppicSeeded)
	if ppicSeeded == 0 {
		ppicPerms := []string{
			"ppic.dashboard.view",
			"ppic.planning.view", "ppic.planning.update",
			"ppic.expiry.view", "ppic.expiry.ack",
			"ppic.opname.view", "ppic.opname.create", "ppic.opname.approve",
		}
		for _, p := range ppicPerms {
			DB.Exec(`INSERT INTO role_permissions (role, permission)
				SELECT DISTINCT role, $1 FROM role_permissions WHERE permission = 'warehouse_dashboard.view'
				ON CONFLICT DO NOTHING`, p)
		}
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_ppic_phase1', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission PPIC Fase 1 di-seed ke role pemegang warehouse_dashboard.view")
	}

	// ── Seed izin PPIC Fase 2 (one-shot, marker) — lihat catatan blok Fase 1 ──
	var ppic2Seeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_ppic_phase2'").Scan(&ppic2Seeded)
	if ppic2Seeded == 0 {
		for _, p := range []string{
			"ppic.forecast.view", "ppic.forecast.manage",
			"ppic.mrp.view", "ppic.mrp.run", "ppic.mrp.execute",
		} {
			DB.Exec(`INSERT INTO role_permissions (role, permission)
				SELECT DISTINCT role, $1 FROM role_permissions WHERE permission = 'ppic.dashboard.view'
				ON CONFLICT DO NOTHING`, p)
		}
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_ppic_phase2', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission PPIC Fase 2 di-seed ke role pemegang ppic.dashboard.view")
	}

	// ── Seed izin PPIC Fase 3 (one-shot, marker) — lihat catatan blok Fase 1 ──
	var ppic3Seeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_ppic_phase3'").Scan(&ppic3Seeded)
	if ppic3Seeded == 0 {
		for _, p := range []string{
			"ppic.production.view", "ppic.production.create", "ppic.production.approve",
			"ppic.workorders.view", "ppic.workorders.create", "ppic.workorders.execute",
			"ppic.reports.view", "ppic.reports.export",
		} {
			DB.Exec(`INSERT INTO role_permissions (role, permission)
				SELECT DISTINCT role, $1 FROM role_permissions WHERE permission = 'ppic.dashboard.view'
				ON CONFLICT DO NOTHING`, p)
		}
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_ppic_phase3', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission PPIC Fase 3 di-seed ke role pemegang ppic.dashboard.view")
	}

	// ── Seed izin Pelanggan (one-shot, marker) — lihat catatan blok Fase 1 ──
	// Role yang boleh melihat Reservasi (modul Penjualan) otomatis mendapat
	// customers.view SEKALI saat deploy.
	var custSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_customers_perm'").Scan(&custSeeded)
	if custSeeded == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'customers.view' FROM role_permissions WHERE permission = 'reservations.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_customers_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission customers.view di-seed ke role pemegang reservations.view")
	}

	// ── Seed izin Analisa Bisnis (one-shot, marker) ─────────────────────────
	// Halaman ini menampilkan perbandingan antar-outlet (RGI), jadi TIDAK
	// diturunkan dari izin laporan lain: hanya role 'admin' yang diberi akses
	// saat deploy (superadmin lolos lewat bypass middleware). Pemberian ke role
	// lain dilakukan manual lewat halaman Role.
	var bizSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_business_analysis_perm'").Scan(&bizSeeded)
	if bizSeeded == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			VALUES ('admin', 'reports.business_analysis.view') ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_business_analysis_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission reports.business_analysis.view di-seed ke role admin")
	}


	// ── Seed izin Bayar Pengadaan (one-shot, marker) ────────────────────────
	// Aksi 'pay' dulu diguard oleh finance.payments.view, sehingga siapa pun
	// yang boleh MELIHAT halaman Pembayaran otomatis boleh MENCAIRKAN uang.
	// Izin baru finance.payments.pay diberikan sekali saat deploy ke role yang
	// hari ini sudah memegang finance.payments.view, supaya perilaku berjalan
	// tidak berubah mendadak; pencabutan per role dilakukan lewat halaman Role.
	var paySeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_finance_payments_pay'").Scan(&paySeeded)
	if paySeeded == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'finance.payments.pay' FROM role_permissions WHERE permission = 'finance.payments.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_finance_payments_pay', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission finance.payments.pay di-seed ke role pemegang finance.payments.view")
	}


	// ── Seed izin Projek (one-shot, marker) ─────────────────────────────────
	// Lihat: diturunkan ke role yang hari ini boleh melihat pengadaan, supaya
	// payung projeknya langsung terlihat oleh orang yang sama. Kelola (buat/
	// ubah/hapus projek + menetapkan RAB) sengaja TIDAK diturunkan otomatis —
	// itu keputusan anggaran, jadi hanya 'admin' yang diberi saat deploy dan
	// pemberian ke role lain dilakukan manual lewat halaman Role.
	var projSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_procurement_projects_perm'").Scan(&projSeeded)
	if projSeeded == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'procurement.projects.view' FROM role_permissions WHERE permission = 'procurement.requests.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			VALUES ('admin', 'procurement.projects.manage') ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_procurement_projects_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission Projek di-seed (view ke pemegang procurement.requests.view, manage ke admin)")
	}

	// ── Kinerja Markom (IG/TikTok) ──────────────────────────────────────────
	// Tiga tabel, karena tiga jenis angka yang umurnya berbeda:
	//
	//   social_snapshots  — cacah kumulatif (pengikut, jumlah post) per tanggal.
	//       Diambil harian. Pengikut hanya masuk akal sebagai potret, bukan
	//       sebagai penjumlahan; menjumlah pengikut lintas hari menghasilkan
	//       angka yang tidak berarti apa-apa.
	//   social_posts      — satu baris per konten, dengan angka terkininya.
	//       Dari sini jangkauan & interaksi MINGGUAN diturunkan, dengan konten
	//       dihitung pada minggu ia terbit.
	//   social_manual_weeks — tambalan yang diketik orang, per akun per minggu.
	//       Kolomnya nullable dan hanya yang terisi yang menimpa hasil scrape.
	//
	// Tabel ketiga bukan kenyamanan, melainkan syarat agar halamannya jujur:
	// jangkauan Instagram tidak pernah ada di halaman publik — angka itu hidup
	// di layar Insights pemegang akun. Tanpa jalur manual, kolom jangkauan IG
	// akan selamanya nol dan terbaca sebagai "tidak ada yang melihat".
	socialMigrations := []string{
		`CREATE TABLE IF NOT EXISTS social_accounts (
			id CHAR(26) PRIMARY KEY,
			outlet_id CHAR(26) NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
			platform VARCHAR(16) NOT NULL,
			username VARCHAR(120) NOT NULL,
			profile_url TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT true,
			last_scraped_at TIMESTAMP,
			last_ok_at TIMESTAMP,
			last_error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_social_accounts
			ON social_accounts(outlet_id, platform, lower(username))`,
		// auto_fetch memisahkan akun yang angkanya DIJEMPUT dari yang angkanya
		// DIANTAR orang. Pemisahan ini bukan kenyamanan: halaman profil publik
		// Instagram tidak menyerahkan satu angka pun ke server ini (HTTP 429,
		// lalu HTML tanpa angka), jadi akun IG yang terus dijadwalkan menarik
		// hanya menghasilkan barisan galat harian. Lebih buruk lagi, kegagalan
		// itu menandai outletnya "mandek" dan mengeluarkannya dari grafik —
		// satu akun IG cukup untuk mengosongkan outlet yang TikTok-nya sehat.
		`ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS auto_fetch BOOLEAN NOT NULL DEFAULT true`,
		`CREATE INDEX IF NOT EXISTS idx_social_accounts_outlet ON social_accounts(outlet_id)`,

		// captured_date = tanggal LOKAL (zona waktu aplikasi), bukan UTC. Scrape
		// yang jalan pukul 03:00 WIB terjadi pada 20:00 UTC hari sebelumnya; kalau
		// dikunci ke tanggal UTC, potret itu jatuh ke minggu bisnis yang salah
		// setiap kali — dan seluruh perbandingan mingguannya ikut meleset sehari.
		`CREATE TABLE IF NOT EXISTS social_snapshots (
			id CHAR(26) PRIMARY KEY,
			account_id CHAR(26) NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
			captured_date DATE NOT NULL,
			captured_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			followers BIGINT,
			following BIGINT,
			posts_count BIGINT,
			likes_total BIGINT,
			source VARCHAR(10) NOT NULL DEFAULT 'scrape',
			-- approx = angka yang sudah dibulatkan platform ("12,3 rb"). Ikut
			-- disimpan supaya selisih pengikut mingguan yang lahir dari
			-- pembulatan tidak dilaporkan sebagai pertumbuhan.
			approx BOOLEAN NOT NULL DEFAULT false
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_social_snapshots
			ON social_snapshots(account_id, captured_date)`,

		`CREATE TABLE IF NOT EXISTS social_posts (
			id CHAR(26) PRIMARY KEY,
			account_id CHAR(26) NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
			post_ref VARCHAR(64) NOT NULL,
			posted_at TIMESTAMP NOT NULL,
			permalink TEXT NOT NULL DEFAULT '',
			caption TEXT NOT NULL DEFAULT '',
			views BIGINT NOT NULL DEFAULT 0,
			likes BIGINT NOT NULL DEFAULT 0,
			comments BIGINT NOT NULL DEFAULT 0,
			shares BIGINT NOT NULL DEFAULT 0,
			fetched_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_social_posts ON social_posts(account_id, post_ref)`,
		`CREATE INDEX IF NOT EXISTS idx_social_posts_week ON social_posts(account_id, posted_at)`,

		`CREATE TABLE IF NOT EXISTS social_manual_weeks (
			account_id CHAR(26) NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
			week_start DATE NOT NULL,
			followers BIGINT,
			posts BIGINT,
			views BIGINT,
			engagement BIGINT,
			note TEXT NOT NULL DEFAULT '',
			updated_by VARCHAR(150) NOT NULL DEFAULT '',
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			PRIMARY KEY (account_id, week_start)
		)`,
	}
	for _, m := range socialMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Social migration skipped: %v", err)
		}
	}

	// ── Instagram: alihkan ke isian manual (one-shot, marker) ───────────────
	// Dijalankan sekali saat deploy, bukan sebagai aturan permanen di kode:
	// kalau nanti dipasang proxy yang tembus, akun IG tinggal dinyalakan lagi
	// per akun lewat halaman Kinerja Markom tanpa menyentuh migrasi ini.
	var igSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_social_ig_manual'").Scan(&igSeeded)
	if igSeeded == 0 {
		DB.Exec(`UPDATE social_accounts SET auto_fetch = false WHERE platform = 'instagram'`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_social_ig_manual', 'done') ON CONFLICT (key) DO NOTHING`)
	}

	// ── Instagram: nyalakan kembali penarikannya (one-shot, marker) ─────────
	// Migrasi di atas mematikannya ketika halaman publik IG belum terbukti bisa
	// dibaca dari server ini. Setelah ditemukan bahwa IG tetap menyerahkan
	// og:description kepada perayap pratinjau tautan, penarikannya sah lagi —
	// yang tetap tidak pernah publik hanyalah JANGKAUAN-nya.
	var igOn int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_social_ig_auto_on'").Scan(&igOn)
	if igOn == 0 {
		DB.Exec(`UPDATE social_accounts SET auto_fetch = true WHERE platform = 'instagram'`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_social_ig_auto_on', 'done') ON CONFLICT (key) DO NOTHING`)
	}

	// ── Seed izin Kinerja Markom (one-shot, marker) ─────────────────────────
	// Lihat: diturunkan ke role yang hari ini boleh membuka Analisa Bisnis —
	// grafik medsos menempel di halaman itu, jadi memisahkan izinnya hanya akan
	// menghasilkan halaman yang separuhnya kosong tanpa sebab yang terlihat.
	// Kelola (mendaftarkan akun, mengetik tambalan manual, menarik paksa) tidak
	// diturunkan otomatis: itu pekerjaan Markom, bukan pembaca laporan.
	var socSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_social_perm'").Scan(&socSeeded)
	if socSeeded == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'social.view' FROM role_permissions WHERE permission = 'reports.business_analysis.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			VALUES ('admin', 'social.view'), ('admin', 'social.manage') ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_social_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Permission Kinerja Markom di-seed (view ke pemegang reports.business_analysis.view, manage ke admin)")
	}

	// ── Perlengkapan/Aset Fase 1: pendataan yang layak ─────────────────────
	// Kolom tambahan + buku besar aset. DDL idempoten, aman dijalankan tiap boot.
	// Rancangan lengkap: docs/perlengkapan-aset.md
	assetPhase1 := []string{
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS asset_no VARCHAR(40) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS tracking_mode VARCHAR(10) DEFAULT 'massal'`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS serial_number VARCHAR(80) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS brand VARCHAR(80) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS model VARCHAR(80) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'aktif'`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS pic_name VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS work_unit_id CHAR(26)`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS acquisition_src VARCHAR(20) DEFAULT 'manual'`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS purchase_request_id CHAR(26)`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS pr_item_key VARCHAR(120) DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS warranty_until DATE`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS useful_life_months INT DEFAULT 0`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS residual_value DECIMAL(15,2) DEFAULT 0`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS photo_url TEXT DEFAULT ''`,
		`ALTER TABLE assets ADD COLUMN IF NOT EXISTS disposed_at TIMESTAMP`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_asset_no ON assets(asset_no) WHERE asset_no <> '' AND is_deleted = false`,
		`CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_assets_pr ON assets(purchase_request_id)`,
		`CREATE TABLE IF NOT EXISTS asset_movements (
			id               CHAR(26) PRIMARY KEY,
			asset_id         CHAR(26) NOT NULL REFERENCES assets(id),
			type             VARCHAR(24) NOT NULL,
			qty              INT DEFAULT 0,
			from_outlet_id   CHAR(26),
			to_outlet_id     CHAR(26),
			from_location    VARCHAR(150) DEFAULT '',
			to_location      VARCHAR(150) DEFAULT '',
			condition_before VARCHAR(20) DEFAULT '',
			condition_after  VARCHAR(20) DEFAULT '',
			ref_type         VARCHAR(24) DEFAULT '',
			ref_id           CHAR(26),
			ref_number       VARCHAR(40) DEFAULT '',
			amount           DECIMAL(15,2) DEFAULT 0,
			notes            TEXT DEFAULT '',
			actor            VARCHAR(100) DEFAULT '',
			created_at       TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_mov_asset ON asset_movements(asset_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_mov_ref ON asset_movements(ref_type, ref_id)`,
	}
	for _, m := range assetPhase1 {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-1 migration skipped: %v", err)
		}
	}

	// Backfill sekali jalan (marker): nomor aset, pemisahan status dari kondisi,
	// dan satu baris buku besar sebagai saldo awal tiap aset lama. Tanpa marker,
	// blok ini akan menomori ulang / menulis ganda setiap kali aplikasi menyala.
	var assetSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_fase1'").Scan(&assetSeeded)
	if assetSeeded == 0 {
		// Nomor aset: AST-<kode outlet>-<YYMM>-<urut 3 digit>, urut created_at
		// per outlet per bulan sehingga cocok dengan pola nextDocNumber.
		if _, err := DB.Exec(`
			WITH numbered AS (
				SELECT a.id,
				       'AST-' || COALESCE(NULLIF(o.code, ''), 'XXX') || '-' ||
				       TO_CHAR(a.created_at, 'YYMM') || '-' ||
				       LPAD((ROW_NUMBER() OVER (
				           PARTITION BY a.outlet_id, TO_CHAR(a.created_at, 'YYMM')
				           ORDER BY a.created_at, a.id))::text, 3, '0') AS no
				FROM assets a
				LEFT JOIN outlets o ON o.id = a.outlet_id
				WHERE COALESCE(a.asset_no, '') = ''
			)
			UPDATE assets SET asset_no = numbered.no
			FROM numbered WHERE assets.id = numbered.id`); err != nil {
			log.Printf("Asset backfill asset_no skipped: %v", err)
		}
		// Kondisi 'perbaikan' dulu dipakai sebagai status. Pindahkan ke kolom
		// status, kondisi fisiknya diturunkan ke rusak_ringan.
		DB.Exec(`UPDATE assets SET status = 'perbaikan', condition = 'rusak_ringan' WHERE condition = 'perbaikan'`)
		// Saldo awal buku besar aset — id hex 26 karakter, sah untuk CHAR(26)
		// dan tidak bertabrakan dengan ULID yang dibuat aplikasi.
		DB.Exec(`
			INSERT INTO asset_movements (id, asset_id, type, qty, to_outlet_id, to_location,
				condition_after, notes, actor, created_at)
			SELECT UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', '') FROM 1 FOR 26)),
			       a.id, 'pendataan', a.quantity, a.outlet_id, COALESCE(a.location, ''),
			       a.condition, 'Saldo awal pendataan (migrasi Fase 1)', '', a.created_at
			FROM assets a
			WHERE a.is_deleted = false
			  AND NOT EXISTS (SELECT 1 FROM asset_movements m WHERE m.asset_id = a.id)`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_fase1', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset Fase 1: nomor aset di-backfill, status dipisah dari kondisi, buku besar disiapkan")
	}

	// ── Perlengkapan/Aset Fase 2: mutasi antar outlet ──────────────────────
	// Alur & tabel meniru stock_transfers supaya istilahnya sama dengan
	// Transfer Stok yang sudah dipakai tim. Lihat docs/perlengkapan-aset.md §6.
	assetPhase2 := []string{
		`CREATE TABLE IF NOT EXISTS asset_transfers (
			id              CHAR(26) PRIMARY KEY,
			transfer_number VARCHAR(30) NOT NULL UNIQUE,
			from_outlet_id  CHAR(26) NOT NULL REFERENCES outlets(id),
			to_outlet_id    CHAR(26) NOT NULL REFERENCES outlets(id),
			status          VARCHAR(20) NOT NULL DEFAULT 'draft',
			reason          VARCHAR(40) DEFAULT '',
			expected_return DATE,
			notes           TEXT DEFAULT '',
			created_by      VARCHAR(100) DEFAULT '',
			approved_by     VARCHAR(100),
			approved_at     TIMESTAMP,
			sent_by         VARCHAR(100),
			sent_at         TIMESTAMP,
			received_by     VARCHAR(100),
			received_at     TIMESTAMP,
			rejected_reason TEXT DEFAULT '',
			created_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS asset_transfer_items (
			id              CHAR(26) PRIMARY KEY,
			transfer_id     CHAR(26) NOT NULL REFERENCES asset_transfers(id) ON DELETE CASCADE,
			asset_id        CHAR(26) NOT NULL REFERENCES assets(id),
			qty             INT NOT NULL DEFAULT 1,
			received_qty    INT,
			condition_sent  VARCHAR(20) DEFAULT '',
			condition_recv  VARCHAR(20) DEFAULT '',
			target_asset_id CHAR(26),
			notes           TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_tr_status ON asset_transfers(status)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_tr_outlets ON asset_transfers(from_outlet_id, to_outlet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_tr_items_tr ON asset_transfer_items(transfer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_tr_items_asset ON asset_transfer_items(asset_id)`,
	}
	for _, m := range assetPhase2 {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-2 migration skipped: %v", err)
		}
	}

	// Izin mutasi aset (one-shot, marker). Hanya MELIHAT yang diturunkan otomatis
	// ke pemegang assets.view — wewenang memindahkan aset antar outlet terlalu
	// besar untuk diberikan diam-diam; itu diberikan manual lewat halaman Role.
	var assetTrPerm int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_transfer_perm'").Scan(&assetTrPerm)
	if assetTrPerm == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'assets.transfer.view' FROM role_permissions WHERE permission = 'assets.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES
			('admin','assets.transfer.view'), ('admin','assets.transfer.create'),
			('admin','assets.transfer.approve'), ('admin','assets.transfer.receive'),
			('superadmin','assets.transfer.view'), ('superadmin','assets.transfer.create'),
			('superadmin','assets.transfer.approve'), ('superadmin','assets.transfer.receive')
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_transfer_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset Fase 2: izin mutasi di-seed (lihat ke pemegang assets.view, penuh ke admin)")
	}

	// ── Perlengkapan/Aset Fase 3: perawatan penuh ──────────────────────────
	// next_due_date sudah lama tersimpan tapi tidak pernah dibaca. Kolom di
	// bawah mengubah catatan mundur menjadi work order bersiklus hidup.
	assetPhase3 := []string{
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS wo_number VARCHAR(30) DEFAULT ''`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'selesai'`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS scheduled_date DATE`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS vendor_id CHAR(26)`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS purchase_request_id CHAR(26)`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS downtime_hours DECIMAL(10,2) DEFAULT 0`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS attachment_url TEXT DEFAULT ''`,
		`ALTER TABLE asset_maintenances ADD COLUMN IF NOT EXISTS created_by VARCHAR(100) DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_asset_mtc_due ON asset_maintenances(next_due_date) WHERE next_due_date IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_asset_mtc_status ON asset_maintenances(status)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_mtc_sched ON asset_maintenances(scheduled_date) WHERE scheduled_date IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_asset_mtc_wo ON asset_maintenances(wo_number) WHERE wo_number <> ''`,
	}
	for _, m := range assetPhase3 {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-3 migration skipped: %v", err)
		}
	}

	// Izin perawatan (one-shot, marker). Dipisah dari assets.update supaya teknisi
	// bisa mencatat pekerjaan tanpa ikut berhak mengubah harga perolehan dan
	// outlet aset — karena itu pemegang assets.update lama WAJIB diberi kunci
	// baru ini, kalau tidak mereka kehilangan akses yang selama ini dipakai.
	var assetMtcPerm int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_maintenance_perm'").Scan(&assetMtcPerm)
	if assetMtcPerm == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'assets.maintenance.view' FROM role_permissions WHERE permission = 'assets.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'assets.maintenance.create' FROM role_permissions WHERE permission = 'assets.update'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES
			('admin','assets.maintenance.view'), ('admin','assets.maintenance.create'),
			('superadmin','assets.maintenance.view'), ('superadmin','assets.maintenance.create')
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_maintenance_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset Fase 3: izin perawatan di-seed (create ke pemegang assets.update lama)")
	}

	// Nomor WO untuk catatan perawatan lama, sekali jalan: WOM<YYMMDD><urut>
	// berdasarkan tanggal pekerjaannya, supaya dokumen lama ikut bernomor.
	var assetWoSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_wo_backfill'").Scan(&assetWoSeeded)
	if assetWoSeeded == 0 {
		DB.Exec(`
			WITH numbered AS (
				SELECT id, 'WOM' || TO_CHAR(maintenance_date, 'YYMMDD') || LPAD((ROW_NUMBER() OVER (
					PARTITION BY maintenance_date ORDER BY created_at, id))::text, 3, '0') AS wo
				FROM asset_maintenances WHERE COALESCE(wo_number, '') = ''
			)
			UPDATE asset_maintenances SET wo_number = numbered.wo
			FROM numbered WHERE asset_maintenances.id = numbered.id`)
		DB.Exec(`UPDATE asset_maintenances SET status = 'selesai' WHERE COALESCE(status, '') = ''`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_wo_backfill', 'done') ON CONFLICT (key) DO NOTHING`)
	}

	// ── Perlengkapan/Aset Fase 4: penerimaan dari pengadaan ────────────────
	// Jejak keputusan per baris pengajuan. Tanpa ini, baris yang sengaja
	// ditandai "habis pakai" akan muncul selamanya di laporan barang yang
	// belum lengkap — dan laporan itu berhenti dipercaya orang.
	for _, m := range []string{
		`ALTER TABLE goods_receipt_items ADD COLUMN IF NOT EXISTS pr_item_key VARCHAR(120) DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS pr_receiving_decisions (
			id          CHAR(26) PRIMARY KEY,
			purchase_request_id CHAR(26) NOT NULL,
			pr_item_key VARCHAR(120) NOT NULL,
			destination VARCHAR(16) NOT NULL,
			qty         INT NOT NULL DEFAULT 0,
			reason      TEXT DEFAULT '',
			actor       VARCHAR(100) DEFAULT '',
			created_at  TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_decisions_pr ON pr_receiving_decisions(purchase_request_id, pr_item_key)`,
		`CREATE INDEX IF NOT EXISTS idx_grn_items_prkey ON goods_receipt_items(pr_item_key) WHERE pr_item_key <> ''`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-4 migration skipped: %v", err)
		}
	}

	// Indeks unik inilah pertahanan terakhir terhadap pencatatan ganda: klik
	// ganda, dua petugas membuka dokumen yang sama, atau status yang sempat
	// dikembalikan tidak boleh melahirkan aset kembar (§8.4).
	if _, err := DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_pr_item
		ON assets(purchase_request_id, pr_item_key)
		WHERE purchase_request_id IS NOT NULL AND pr_item_key <> '' AND is_deleted = false`); err != nil {
		log.Printf("Asset phase-4 migration skipped: %v", err)
	}

	// ── Perlengkapan/Aset Fase 4b: penghapusan & opname ────────────────────
	assetPhase4b := []string{
		`CREATE TABLE IF NOT EXISTS asset_disposals (
			id              CHAR(26) PRIMARY KEY,
			disposal_number VARCHAR(30) NOT NULL UNIQUE,
			asset_id        CHAR(26) NOT NULL REFERENCES assets(id),
			qty             INT NOT NULL DEFAULT 1,
			method          VARCHAR(20) NOT NULL,
			reason          TEXT NOT NULL,
			proceeds        DECIMAL(15,2) DEFAULT 0,
			book_value      DECIMAL(15,2) DEFAULT 0,
			status          VARCHAR(20) DEFAULT 'pending',
			requested_by    VARCHAR(100) DEFAULT '',
			approved_by     VARCHAR(100),
			approved_at     TIMESTAMP,
			rejected_reason TEXT DEFAULT '',
			attachment_url  TEXT DEFAULT '',
			created_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_disposals_status ON asset_disposals(status)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_disposals_asset ON asset_disposals(asset_id)`,
		`CREATE TABLE IF NOT EXISTS asset_opname_sessions (
			id            CHAR(26) PRIMARY KEY,
			opname_number VARCHAR(30) NOT NULL UNIQUE,
			outlet_id     CHAR(26) NOT NULL REFERENCES outlets(id),
			status        VARCHAR(20) DEFAULT 'berjalan',
			notes         TEXT DEFAULT '',
			created_by    VARCHAR(100) DEFAULT '',
			approved_by   VARCHAR(100),
			approved_at   TIMESTAMP,
			created_at    TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS asset_opname_items (
			id              CHAR(26) PRIMARY KEY,
			session_id      CHAR(26) NOT NULL REFERENCES asset_opname_sessions(id) ON DELETE CASCADE,
			asset_id        CHAR(26) REFERENCES assets(id),
			system_qty      INT DEFAULT 0,
			counted_qty     INT,
			condition_found VARCHAR(20) DEFAULT '',
			found_name      VARCHAR(150) DEFAULT '',
			location        VARCHAR(150) DEFAULT '',
			notes           TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_opname_items_sess ON asset_opname_items(session_id)`,
	}
	for _, m := range assetPhase4b {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-4b migration skipped: %v", err)
		}
	}

	// Izin penghapusan & opname (one-shot, marker). Mengajukan penghapusan
	// diturunkan ke pemegang assets.delete — merekalah yang selama ini berhak
	// mengeluarkan barang dari daftar. MENYETUJUI tidak diturunkan otomatis.
	var assetD4Perm int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_disposal_opname_perm'").Scan(&assetD4Perm)
	if assetD4Perm == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, p FROM role_permissions, LATERAL (VALUES ('assets.disposal.view'), ('assets.opname.view')) AS t(p)
			WHERE permission = 'assets.view' ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'assets.disposal.create' FROM role_permissions WHERE permission = 'assets.delete'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT r.role, p FROM (VALUES ('admin'), ('superadmin')) AS r(role),
			       LATERAL (VALUES ('assets.disposal.view'), ('assets.disposal.create'), ('assets.disposal.approve'),
			                       ('assets.opname.view'), ('assets.opname.create'), ('assets.opname.approve'),
			                       ('assets.report.view')) AS t(p)
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_disposal_opname_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset Fase 4: izin penghapusan, opname, dan laporan di-seed")
	}

	// Jenis belanja barang: "dapur" (diterima di Gudang Induk) atau
	// "perlengkapan" (diterima di bagian Aset). Satu pengajuan hanya boleh
	// salah satu — dokumen campuran memaksa satu berkas diantar ke dua meja.
	for _, m := range []string{
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS goods_kind VARCHAR(16) DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_pr_goods_kind ON purchase_requests(goods_kind) WHERE goods_kind <> ''`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Purchase goods_kind migration skipped: %v", err)
		}
	}

	// ── Perlengkapan/Aset Fase 5: material projek ──────────────────────────
	// Buku stok lokasi projek. Sengaja TERPISAH dari stock_items: semen dan
	// keramik bukan bahan F&B, dan memasukkannya ke katalog stok akan mengotori
	// resep, HPP, MRP, serta seluruh laporan PPIC (keputusan A, docs §15.1).
	assetPhase5 := []string{
		`CREATE TABLE IF NOT EXISTS project_materials (
			id           CHAR(26) PRIMARY KEY,
			project_id   CHAR(26) NOT NULL REFERENCES projects(id),
			purchase_request_id CHAR(26),
			pr_item_key  VARCHAR(120) DEFAULT '',
			name         VARCHAR(150) NOT NULL,
			unit         VARCHAR(20) DEFAULT '',
			qty_received DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_used     DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_returned DECIMAL(15,4) NOT NULL DEFAULT 0,
			qty_wasted   DECIMAL(15,4) NOT NULL DEFAULT 0,
			unit_cost    DECIMAL(15,2) DEFAULT 0,
			location     VARCHAR(150) DEFAULT '',
			received_at  TIMESTAMP,
			received_by  VARCHAR(100) DEFAULT '',
			notes        TEXT DEFAULT '',
			created_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_materials_pr_item
			ON project_materials(purchase_request_id, pr_item_key)
			WHERE purchase_request_id IS NOT NULL AND pr_item_key <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_project_materials_project ON project_materials(project_id)`,
		`CREATE TABLE IF NOT EXISTS project_material_logs (
			id          CHAR(26) PRIMARY KEY,
			material_id CHAR(26) NOT NULL REFERENCES project_materials(id) ON DELETE CASCADE,
			type        VARCHAR(20) NOT NULL,
			qty         DECIMAL(15,4) NOT NULL,
			ref_type    VARCHAR(24) DEFAULT '',
			ref_id      CHAR(26),
			ref_number  VARCHAR(40) DEFAULT '',
			notes       TEXT DEFAULT '',
			actor       VARCHAR(100) DEFAULT '',
			created_at  TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_material_logs_mat ON project_material_logs(material_id, created_at)`,
	}
	for _, m := range assetPhase5 {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset phase-5 migration skipped: %v", err)
		}
	}

	// Penerimaan barang dan pembayaran adalah DUA DIMENSI, bukan satu deret.
	// Ada vendor yang mengirim dulu baru ditagih (tempo), ada yang minta dibayar
	// di muka. Kolom status lama bercampur: 'received' diperlakukan sebagai
	// kelanjutan 'paid', sehingga barang yang datang sebelum dibayar tidak punya
	// tempat dicatat. receipt_status memisahkan keduanya.
	for _, m := range []string{
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS receipt_status VARCHAR(20) DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_pr_receipt_status ON purchase_requests(receipt_status) WHERE receipt_status <> ''`,
		// Dokumen lama berstatus 'received' jelas sudah diterima seluruhnya.
		`UPDATE purchase_requests SET receipt_status = 'received'
		 WHERE status = 'received' AND COALESCE(receipt_status, '') = ''`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Purchase receipt_status migration skipped: %v", err)
		}
	}

	// ── Fase 6: bukti foto serah terima & distribusi ───────────────────────
	// Dua momen wajib berfoto: saat barang diterima dari tim purchasing, dan
	// saat barang diteruskan ke tujuan akhirnya (gudang outlet / PIC pengaju).
	// Foto disalin ke email cadangan supaya tetap ada bila berkas di server
	// hilang — karena itu setiap baris punya status cadangannya sendiri.
	phase6 := []string{
		`CREATE TABLE IF NOT EXISTS handover_photos (
			id         CHAR(26) PRIMARY KEY,
			moment     VARCHAR(16) NOT NULL,      -- terima | distribusi
			desk       VARCHAR(16) NOT NULL DEFAULT '', -- perlengkapan | dapur
			ref_type   VARCHAR(24) NOT NULL,      -- purchase_request | asset_handover | stock_transfer
			ref_id     CHAR(26) NOT NULL,
			ref_number VARCHAR(40) DEFAULT '',
			photo_url  TEXT NOT NULL,
			notes      TEXT DEFAULT '',
			actor      VARCHAR(100) DEFAULT '',
			backup_status VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending|sent|failed|skipped
			backup_at  TIMESTAMP,
			backup_error TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_handover_photos_ref ON handover_photos(ref_type, ref_id)`,
		`CREATE INDEX IF NOT EXISTS idx_handover_photos_backup ON handover_photos(backup_status) WHERE backup_status = 'pending'`,

		// Serah terima aset ke PIC pengaju: barang tidak berhenti di bagian aset.
		`CREATE TABLE IF NOT EXISTS asset_handovers (
			id            CHAR(26) PRIMARY KEY,
			handover_number VARCHAR(30) NOT NULL UNIQUE,
			purchase_request_id CHAR(26),
			outlet_id     CHAR(26) REFERENCES outlets(id),
			pic_name      VARCHAR(100) NOT NULL,
			pic_position  VARCHAR(100) DEFAULT '',
			location      VARCHAR(150) DEFAULT '',
			notes         TEXT DEFAULT '',
			handed_by     VARCHAR(100) DEFAULT '',
			created_at    TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE TABLE IF NOT EXISTS asset_handover_items (
			id          CHAR(26) PRIMARY KEY,
			handover_id CHAR(26) NOT NULL REFERENCES asset_handovers(id) ON DELETE CASCADE,
			asset_id    CHAR(26) NOT NULL REFERENCES assets(id),
			qty         INT NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_handover_items_h ON asset_handover_items(handover_id)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_handovers_pr ON asset_handovers(purchase_request_id)`,

		// Foto saat barang dikirim dari gudang induk ke gudang outlet.
		`ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS photo_url TEXT DEFAULT ''`,
	}
	for _, m := range phase6 {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Handover photo migration skipped: %v", err)
		}
	}

	// Tautan Google Drive untuk bukti foto. Berkas dijadikan publik ("siapa
	// saja yang punya link") atas keputusan pemilik sistem, sehingga bisa
	// dipanggil langsung dari layar tanpa login.
	for _, m := range []string{
		`ALTER TABLE handover_photos ADD COLUMN IF NOT EXISTS drive_file_id VARCHAR(80) DEFAULT ''`,
		`ALTER TABLE handover_photos ADD COLUMN IF NOT EXISTS drive_url TEXT DEFAULT ''`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Drive photo migration skipped: %v", err)
		}
	}

	// Izin Dashboard Aset (one-shot, marker): diturunkan ke pemegang assets.view,
	// karena dashboard hanya merangkum data yang sudah boleh mereka lihat.
	var assetDashPerm int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_assets_dashboard_perm'").Scan(&assetDashPerm)
	if assetDashPerm == 0 {
		DB.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'assets.dashboard.view' FROM role_permissions WHERE permission = 'assets.view'
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES
			('admin','assets.dashboard.view'), ('superadmin','assets.dashboard.view')
			ON CONFLICT DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_assets_dashboard_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset: izin Dashboard Aset di-seed ke pemegang assets.view")
	}

	// ── Master kategori aset ───────────────────────────────────────────────
	// Sebelumnya kategori diketik bebas, sehingga "Elektronik", "elektronik",
	// dan "Elektronic" menjadi tiga kelompok berbeda di dashboard dan laporan.
	// Master ini sekaligus menjadi rumah bagi dua angka bawaan yang tadinya
	// terserak: umur ekonomis (dulu ditebak di sisi layar) dan interval
	// perawatan preventif (dulu daftar tetap di dalam kode Go).
	assetCategoryMigrations := []string{
		`CREATE TABLE IF NOT EXISTS asset_categories (
			id   CHAR(26) PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			useful_life_months INT DEFAULT 0,
			maintenance_interval_months INT DEFAULT 0,
			notes TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_asset_categories_active ON asset_categories(is_active)`,
	}
	for _, m := range assetCategoryMigrations {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Asset category migration skipped: %v", err)
		}
	}

	// Isi awal (one-shot, marker): daftar bawaan dari rancangan + kategori yang
	// terlanjur diketik pada aset yang sudah ada, supaya tidak ada data lama
	// yang kategorinya mendadak tidak dikenal.
	var assetCatSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_asset_categories'").Scan(&assetCatSeeded)
	if assetCatSeeded == 0 {
		DB.Exec(`
			INSERT INTO asset_categories (id, name, useful_life_months, maintenance_interval_months, notes)
			VALUES
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Elektronik & IT', 36, 12, 'Komputer, printer, POS, CCTV'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'AC & Pendingin', 36, 3,  'AC split, kulkas, chiller, freezer'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Mebel', 60, 0, 'Meja, kursi, rak, lemari'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Peralatan Dapur', 60, 6, 'Kompor, oven, peralatan masak'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Mesin Kopi & Bar', 60, 1, 'Mesin espresso, grinder, blender'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Genset & Kelistrikan', 96, 6, 'Genset, panel, instalasi'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Kendaraan', 96, 6, 'Motor, mobil operasional'),
			 (UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)), 'Perkakas & Lainnya', 48, 0, 'Perkakas, perlengkapan umum')
			ON CONFLICT (name) DO NOTHING`)
		DB.Exec(`
			INSERT INTO asset_categories (id, name, useful_life_months, notes)
			SELECT UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text,'-','') FROM 1 FOR 26)),
			       a.category, COALESCE(MAX(a.useful_life_months), 0), 'Dari data aset lama'
			FROM assets a
			WHERE COALESCE(a.category,'') <> ''
			  AND NOT EXISTS (SELECT 1 FROM asset_categories c WHERE lower(c.name) = lower(a.category))
			GROUP BY a.category
			ON CONFLICT (name) DO NOTHING`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_asset_categories', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Aset: master kategori di-seed")
	}

	// ── Kontrol transfer disamakan: mutasi aset berfoto, transfer stok berizin ──
	// Mutasi aset kini wajib berfoto saat kirim, sama seperti transfer stok dan
	// serah terima ke PIC.
	if _, err := DB.Exec(`ALTER TABLE asset_transfers ADD COLUMN IF NOT EXISTS photo_url TEXT DEFAULT ''`); err != nil {
		log.Printf("Asset transfer photo migration skipped: %v", err)
	}

	// Izin transfer stok dipecah: 'approve' dan 'receive' terpisah dari 'update'
	// (one-shot, marker — WAJIB di paling akhir, lihat catatan re-seed di atas).
	// Semua peran yang hari ini memegang 'update' — dan karena itu sudah bisa
	// menyetujui & menerima — mendapat keduanya, supaya tidak ada alur yang
	// mendadak terhenti. Pemisahan orangnya diatur admin lewat halaman Role.
	var stSplit int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_stocktransfers_split_perm'").Scan(&stSplit)
	if stSplit == 0 {
		for _, p := range []string{"stocktransfers.approve", "stocktransfers.receive"} {
			DB.Exec(`INSERT INTO role_permissions (role, permission)
				SELECT DISTINCT role, $1 FROM role_permissions WHERE permission = 'stocktransfers.update'
				ON CONFLICT DO NOTHING`, p)
			DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ('admin', $1), ('superadmin', $1)
				ON CONFLICT DO NOTHING`, p)
		}
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_stocktransfers_split_perm', 'done') ON CONFLICT (key) DO NOTHING`)
		log.Printf("Gudang: izin transfer stok dipecah (approve/receive diwariskan dari update)")
	}

	// ── Reservasi: uang muka tervalidasi ───────────────────────────────────
	// Sebelum ini DP hanya angka bebas di kolom down_payment: tanpa bukti,
	// tanpa tanggal, tanpa siapa yang memvalidasi, dan tidak pernah sampai ke
	// laporan keuangan. Sekarang down_payment = DP yang DIMINTA; uang yang
	// benar-benar masuk hidup di reservation_payments dan diperlakukan sebagai
	// kewajiban (Uang Muka Pelanggan) sampai reservasinya ditutup di POS.
	for _, m := range []string{
		`ALTER TABLE reservations ADD COLUMN IF NOT EXISTS cancel_disposition VARCHAR(10) DEFAULT ''`,
		`ALTER TABLE reservations ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMP`,
		`ALTER TABLE reservations ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMP`,
		`ALTER TABLE reservations ADD COLUMN IF NOT EXISTS settled_at TIMESTAMP`,
		// VARCHAR, bukan CHAR(26): id transaksi = local_id dari app POS (sampai 50
		// karakter), dan CHAR mem-padding nilai pendek dengan spasi.
		`ALTER TABLE reservations ADD COLUMN IF NOT EXISTS pos_transaction_id VARCHAR(50)`,
		`ALTER TABLE reservations ALTER COLUMN pos_transaction_id TYPE VARCHAR(50)`,
		`CREATE TABLE IF NOT EXISTS reservation_payments (
			id              CHAR(26) PRIMARY KEY,
			reservation_id  CHAR(26) NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
			outlet_id       CHAR(26) NOT NULL,
			type            VARCHAR(12) NOT NULL,
			amount          DECIMAL(15,2) NOT NULL,
			method          VARCHAR(20) DEFAULT 'transfer',
			bank_account_id CHAR(26),
			proof_url       TEXT DEFAULT '',
			paid_at         TIMESTAMP NOT NULL,
			status          VARCHAR(12) NOT NULL DEFAULT 'pending',
			submitted_by    VARCHAR(100) DEFAULT '',
			validated_by    VARCHAR(100) DEFAULT '',
			validated_at    TIMESTAMP,
			rejected_reason TEXT DEFAULT '',
			notes           TEXT DEFAULT '',
			created_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reservation_payments_resv ON reservation_payments(reservation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reservation_payments_paid ON reservation_payments(status, paid_at)`,
		`CREATE INDEX IF NOT EXISTS idx_reservations_cancelled ON reservations(cancelled_at) WHERE cancelled_at IS NOT NULL`,
		`INSERT INTO app_settings (key, value) VALUES ('reservation_dp_percent', '50') ON CONFLICT (key) DO NOTHING`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Reservation payments migration skipped: %v", err)
		}
	}

	// ── RAB projek per baris ──
	// projects.budget tetap ada sebagai Σ subtotal baris (disinkronkan
	// services), supaya neraca dan daftar tidak perlu join baru.
	for _, m := range []string{
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS rab_status VARCHAR(20) NOT NULL DEFAULT 'draft'`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS rab_version INT NOT NULL DEFAULT 0`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS rab_set_at TIMESTAMP`,
		`ALTER TABLE projects ADD COLUMN IF NOT EXISTS rab_set_by VARCHAR(150) NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS project_rab_items (
			id         CHAR(26) PRIMARY KEY,
			project_id CHAR(26) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			seq        INT NOT NULL DEFAULT 0,
			section    VARCHAR(150) NOT NULL DEFAULT '',
			name       VARCHAR(200) NOT NULL,
			kind       VARCHAR(10) NOT NULL DEFAULT 'barang',
			unit       VARCHAR(30) NOT NULL DEFAULT '',
			qty        DECIMAL(15,3) NOT NULL DEFAULT 1,
			unit_price DECIMAL(15,2) NOT NULL DEFAULT 0,
			subtotal   DECIMAL(15,2) NOT NULL DEFAULT 0,
			notes      TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_rab_items_project ON project_rab_items(project_id, seq)`,
	} {
		if _, err := DB.Exec(m); err != nil {
			log.Printf("Project RAB migration skipped: %v", err)
		}
	}
	// One-shot: projek lama yang RAB-nya masih satu angka mendapat satu baris
	// 'umum' senilai angka itu dan langsung berstatus ditetapkan, supaya
	// pengajuan untuk projek lama tidak mendadak ditolak. Projek ber-RAB 0 tetap
	// draft: memang belum punya anggaran.
	var rabSeeded int
	DB.QueryRow("SELECT COUNT(*) FROM app_settings WHERE key = 'mig_project_rab_legacy'").Scan(&rabSeeded)
	if rabSeeded == 0 {
		DB.Exec(`
			INSERT INTO project_rab_items (id, project_id, seq, section, name, kind, unit, qty, unit_price, subtotal, notes)
			SELECT UPPER(SUBSTR(MD5(p.id || clock_timestamp()::text), 1, 26)), p.id, 0, 'Anggaran',
			       'Anggaran projek (migrasi dari RAB satu angka)', 'umum', 'ls', 1, p.budget, p.budget, ''
			FROM projects p
			WHERE p.budget > 0 AND NOT EXISTS (SELECT 1 FROM project_rab_items r WHERE r.project_id = p.id)`)
		DB.Exec(`
			UPDATE projects SET rab_status = 'ditetapkan', rab_version = 1, rab_set_at = (now() AT TIME ZONE 'UTC'), rab_set_by = 'migrasi'
			WHERE budget > 0 AND rab_status = 'draft'`)
		DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_project_rab_legacy', 'done') ON CONFLICT (key) DO NOTHING`)
	}

	return nil
}
