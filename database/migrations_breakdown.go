package database

import "fmt"

// RunBreakdownMigrations — skema Pemotongan: satu bahan dipotong/diolah
// menjadi beberapa hasil (mis. ayam utuh → paha, dada, sayap; daging sapi →
// has dalam, has luar, tetelan; ikan utuh → fillet, kepala; beras → nasi).
// Dipanggil main.go tepat setelah RunAuthMigrations; sengaja di berkas
// terpisah dari migrations.go. Tiap pernyataan idempoten sehingga aman
// dijalankan di setiap boot.
//
//   - stock_breakdown_patterns: POLA pemotongan per bahan (katalog global,
//     dikelola Gudang Induk). Satu bahan boleh punya beberapa pola bernama,
//     mis. ayam utuh "Per Bagian" dan "Potong 8"; nama unik per bahan tanpa
//     membedakan huruf besar/kecil.
//   - stock_item_breakdowns: baris hasil sebuah pola — hasil standar per 1
//     satuan distribusi bahan (dalam satuan dasar HASIL) dan bobot biayanya.
//     Satu item hasil boleh muncul di beberapa pola (unik per pola + hasil).
//   - stock_breakdowns + stock_breakdown_outputs: dokumen pemotongan
//     (POT-YYYYMMDD-NNN). Bahan keluar lewat movement breakdown_out (FIFO),
//     hasil masuk lewat breakdown_in ber-HPP hasil pembagian biaya aktual dan
//     mewarisi kedaluwarsa terdekat bahan yang terpotong. Pola, namanya,
//     hasil standar & bobot disalin ke dokumen supaya riwayat tidak berubah
//     saat pola diubah atau dihapus (pattern_id menjadi NULL, namanya tetap).
//     Susut/yield dibaca dari hasil aktual vs standar — limbah (tulang,
//     jeroan) tidak punya baris stok.
//
// Template versi awal (satu daftar hasil per bahan, tanpa pola) dipindahkan
// ke satu pola "Standar" per bahan; dokumen lamanya ikut ditautkan sekali.
func RunBreakdownMigrations() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS stock_breakdown_patterns (
			id CHAR(26) PRIMARY KEY,
			input_item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_by VARCHAR(100) NOT NULL DEFAULT '',
			updated_by VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_breakdown_patterns_name ON stock_breakdown_patterns (input_item_id, lower(name))`,

		`CREATE TABLE IF NOT EXISTS stock_item_breakdowns (
			id CHAR(26) PRIMARY KEY,
			pattern_id CHAR(26) REFERENCES stock_breakdown_patterns(id) ON DELETE CASCADE,
			input_item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			output_item_id CHAR(26) NOT NULL REFERENCES stock_items(id) ON DELETE CASCADE,
			qty_per_input DECIMAL(15,4) NOT NULL CHECK (qty_per_input > 0),
			cost_weight DECIMAL(15,4) NOT NULL DEFAULT 1 CHECK (cost_weight > 0),
			notes TEXT NOT NULL DEFAULT '',
			created_by VARCHAR(100) NOT NULL DEFAULT '',
			updated_by VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			updated_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
			CHECK (input_item_id <> output_item_id)
		)`,
		`ALTER TABLE stock_item_breakdowns ADD COLUMN IF NOT EXISTS pattern_id CHAR(26) REFERENCES stock_breakdown_patterns(id) ON DELETE CASCADE`,
		`CREATE INDEX IF NOT EXISTS idx_stock_item_breakdowns_output ON stock_item_breakdowns(output_item_id)`,
		// Template versi awal → pola "Standar" per bahan (id hex 26 karakter,
		// sah untuk CHAR(26) dan tidak bertabrakan dengan ULID aplikasi).
		`INSERT INTO stock_breakdown_patterns (id, input_item_id, name, notes, created_by, updated_by)
		 SELECT UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', '') FROM 1 FOR 26)), b.input_item_id, 'Standar',
		        'Dipindahkan dari template pemotongan versi awal', MIN(b.created_by), MIN(b.updated_by)
		 FROM stock_item_breakdowns b
		 WHERE b.pattern_id IS NULL
		   AND NOT EXISTS (SELECT 1 FROM stock_breakdown_patterns p
		                   WHERE p.input_item_id = b.input_item_id AND lower(p.name) = 'standar')
		 GROUP BY b.input_item_id`,
		`UPDATE stock_item_breakdowns b SET pattern_id = p.id
		 FROM stock_breakdown_patterns p
		 WHERE b.pattern_id IS NULL AND p.input_item_id = b.input_item_id AND lower(p.name) = 'standar'`,
		// Keunikan pindah dari (bahan, hasil) ke (pola, hasil).
		`ALTER TABLE stock_item_breakdowns DROP CONSTRAINT IF EXISTS stock_item_breakdowns_input_item_id_output_item_id_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_item_breakdowns_pattern_output ON stock_item_breakdowns (pattern_id, output_item_id)`,
		`ALTER TABLE stock_item_breakdowns ALTER COLUMN pattern_id SET NOT NULL`,

		`CREATE TABLE IF NOT EXISTS stock_breakdowns (
			id CHAR(26) PRIMARY KEY,
			breakdown_number VARCHAR(30) NOT NULL UNIQUE,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			input_item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			pattern_id CHAR(26) REFERENCES stock_breakdown_patterns(id) ON DELETE SET NULL,
			pattern_name VARCHAR(100) NOT NULL DEFAULT '',
			input_qty_dist DECIMAL(15,4) NOT NULL,
			input_qty_base DECIMAL(15,4) NOT NULL CHECK (input_qty_base > 0),
			input_unit VARCHAR(20) NOT NULL DEFAULT '',
			input_cost_total DECIMAL(15,2) NOT NULL DEFAULT 0,
			expiry_date DATE,
			notes TEXT NOT NULL DEFAULT '',
			created_by VARCHAR(100) NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
		)`,
		`ALTER TABLE stock_breakdowns ADD COLUMN IF NOT EXISTS pattern_id CHAR(26) REFERENCES stock_breakdown_patterns(id) ON DELETE SET NULL`,
		`ALTER TABLE stock_breakdowns ADD COLUMN IF NOT EXISTS pattern_name VARCHAR(100) NOT NULL DEFAULT ''`,
		`ALTER TABLE stock_breakdowns ADD COLUMN IF NOT EXISTS expiry_date DATE`,
		`CREATE INDEX IF NOT EXISTS idx_stock_breakdowns_wh ON stock_breakdowns(warehouse_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_breakdowns_input ON stock_breakdowns(input_item_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_breakdowns_pattern ON stock_breakdowns(pattern_id)`,

		`CREATE TABLE IF NOT EXISTS stock_breakdown_outputs (
			id CHAR(26) PRIMARY KEY,
			breakdown_id CHAR(26) NOT NULL REFERENCES stock_breakdowns(id) ON DELETE CASCADE,
			item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_base DECIMAL(15,4) NOT NULL CHECK (qty_base >= 0),
			std_qty_base DECIMAL(15,4) NOT NULL,
			qty_per_input DECIMAL(15,4) NOT NULL,
			cost_weight DECIMAL(15,4) NOT NULL,
			cost_per_base DECIMAL(15,4) NOT NULL DEFAULT 0,
			total_cost DECIMAL(15,2) NOT NULL DEFAULT 0,
			UNIQUE (breakdown_id, item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_breakdown_outputs_item ON stock_breakdown_outputs(item_id)`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("migrasi pemotongan: %w", err)
		}
	}

	// Dokumen versi awal (tanpa pola) ditautkan SEKALI ke pola "Standar"
	// bahannya. Penanda mencegah dokumen lama yang polanya tak ada tertaut ke
	// pola "Standar" buatan pengguna di kemudian hari.
	var done bool
	if err := DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM app_settings WHERE key = 'mig_breakdown_patterns_v1')`).Scan(&done); err != nil {
		return fmt.Errorf("migrasi pemotongan: %w", err)
	}
	if !done {
		if _, err := DB.Exec(`
			UPDATE stock_breakdowns sb SET pattern_id = p.id, pattern_name = p.name
			FROM stock_breakdown_patterns p
			WHERE sb.pattern_id IS NULL AND sb.pattern_name = ''
			  AND p.input_item_id = sb.input_item_id AND lower(p.name) = 'standar'`); err != nil {
			return fmt.Errorf("migrasi pemotongan: %w", err)
		}
		if _, err := DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('mig_breakdown_patterns_v1', 'done') ON CONFLICT (key) DO NOTHING`); err != nil {
			return fmt.Errorf("migrasi pemotongan: %w", err)
		}
	}
	return nil
}
