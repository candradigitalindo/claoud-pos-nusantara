package database

import "fmt"

// RunProcurementUrgencyMigrations: tingkat kebutuhan pengajuan (rutin /
// prioritas / urgent) dan Pembelian Darurat (Nov 2026, keputusan owner 19 Nov:
// urgent ≤ Rp500.000 boleh dibeli dulu dari kas laci outlet, disahkan
// penyetuju sesudahnya). Seluruhnya aditif: dokumen lama menjadi 'rutin' dan
// bukan darurat.
func RunProcurementUrgencyMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS urgency VARCHAR(20) NOT NULL DEFAULT 'rutin'`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_category VARCHAR(40) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_reason TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_recipient VARCHAR(200) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_proof_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_proof_total NUMERIC(15,2)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_proof_notes TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_proof_by VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS emergency_proof_at TIMESTAMP`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS ratification_status VARCHAR(20) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS ratified_by VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS ratified_by_id CHAR(26)`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS ratified_at TIMESTAMP`,
		`ALTER TABLE purchase_requests ADD COLUMN IF NOT EXISTS ratification_notes TEXT NOT NULL DEFAULT ''`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_purchase_requests_urgency') THEN
				ALTER TABLE purchase_requests ADD CONSTRAINT chk_purchase_requests_urgency
					CHECK (urgency IN ('rutin','prioritas','urgent'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_purchase_requests_ratification') THEN
				ALTER TABLE purchase_requests ADD CONSTRAINT chk_purchase_requests_ratification
					CHECK (ratification_status IN ('','pending','ratified','rejected')
					   AND (emergency OR ratification_status = ''));
			END IF;
		END $$`,
		// Pemakaian bulanan per outlet (plafon) dan antrean pengesahan.
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_emergency ON purchase_requests(outlet_id, created_at) WHERE emergency`,
		`CREATE INDEX IF NOT EXISTS idx_purchase_requests_ratification ON purchase_requests(ratification_status) WHERE ratification_status = 'pending'`,
		// Batas bawaan: Rp500.000 per pembelian, tanpa plafon bulanan.
		`INSERT INTO app_settings (key, value) VALUES ('procurement_emergency_max', '500000') ON CONFLICT (key) DO NOTHING`,
		`INSERT INTO app_settings (key, value) VALUES ('procurement_emergency_monthly_cap', '0') ON CONFLICT (key) DO NOTHING`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrasi tingkat kebutuhan pengadaan: %w", err)
		}
	}
	// Izin Pembelian Darurat diberikan SEKALI ke role yang hari ini boleh
	// membuat pengajuan barang/jasa (manajer outlet), supaya bisa langsung
	// dipakai. Pencabutan per role lewat halaman Role bertahan: penanda di
	// app_settings di-commit bersama pemberiannya.
	res, err := tx.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_procurement_emergency_perm', 'done') ON CONFLICT (key) DO NOTHING`)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n > 0 {
		if _, err := tx.Exec(`INSERT INTO role_permissions (role, permission)
			SELECT DISTINCT role, 'procurement.requests.emergency' FROM role_permissions
			WHERE permission = 'procurement.requests.submit'
			ON CONFLICT DO NOTHING`); err != nil {
			return fmt.Errorf("izin Pembelian Darurat: %w", err)
		}
	}
	return tx.Commit()
}
