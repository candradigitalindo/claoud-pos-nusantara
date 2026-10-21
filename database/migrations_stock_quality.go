package database

import "fmt"

// StockQualityPermissions separates location and stage; no stock mutation is
// implied by a view permission. Existing roles are never silently reassigned.
var StockQualityPermissions = []string{
	"stockquality.view", "stockquality.central.report", "stockquality.central.approve",
	"stockquality.central.execute", "stockquality.central.receive", "stockquality.refund",
	"stockquality.outlet.receive", "stockquality.outlet.report", "stockquality.outlet.approve", "stockquality.outlet.execute",
	"stocktransfers.reject",
}

func RunStockQualityMigrations() error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS stock_evidence (
			id CHAR(26) PRIMARY KEY, warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id),
			url TEXT NOT NULL, sha256 VARCHAR(64) NOT NULL, actor_id VARCHAR(100) NOT NULL,
			actor_name VARCHAR(100) NOT NULL, recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			used_ref VARCHAR(100) NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS stock_quality_cases (
			id CHAR(26) PRIMARY KEY, number VARCHAR(50) UNIQUE NOT NULL,
			warehouse_id CHAR(26) NOT NULL REFERENCES warehouses(id), item_id CHAR(26) NOT NULL REFERENCES stock_items(id),
			qty_base NUMERIC(18,4) NOT NULL CHECK(qty_base>0), qty_input NUMERIC(18,4) NOT NULL CHECK(qty_input>0),
			unit_used VARCHAR(20) NOT NULL, cost_per_base NUMERIC(18,6) NOT NULL, expiry_date DATE,
			reason VARCHAR(20) NOT NULL CHECK(reason IN ('reject','expired','spoiled','damaged','return')),
			notes TEXT NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'pending',
			disposition VARCHAR(24) NOT NULL DEFAULT '', settlement VARCHAR(20) NOT NULL DEFAULT '',
			vendor_id CHAR(26), vendor_name VARCHAR(200) NOT NULL DEFAULT '', supplier_reference VARCHAR(200) NOT NULL DEFAULT '',
			return_warehouse_id CHAR(26) REFERENCES warehouses(id), transfer_id CHAR(26), parent_case_id CHAR(26),
			settled_qty_base NUMERIC(18,4) NOT NULL DEFAULT 0, refund_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
			created_by VARCHAR(100) NOT NULL, created_name VARCHAR(100) NOT NULL,
			approved_by VARCHAR(100) NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK(settled_qty_base>=0 AND settled_qty_base<=qty_base)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_quality_warehouse_status ON stock_quality_cases(warehouse_id,status)`,
		`CREATE TABLE IF NOT EXISTS stock_quality_events (
			id CHAR(26) PRIMARY KEY, case_id CHAR(26) NOT NULL REFERENCES stock_quality_cases(id) ON DELETE CASCADE,
			action VARCHAR(30) NOT NULL, actor_id VARCHAR(100) NOT NULL, actor_name VARCHAR(100) NOT NULL,
			notes TEXT NOT NULL DEFAULT '', evidence_id CHAR(26) REFERENCES stock_evidence(id),
			qty_base NUMERIC(18,4) NOT NULL DEFAULT 0, amount NUMERIC(18,2) NOT NULL DEFAULT 0,
			reference VARCHAR(200) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE stock_transfers ADD COLUMN IF NOT EXISTS receipt_version INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE stock_transfer_items ADD COLUMN IF NOT EXISTS accepted_qty_base NUMERIC(18,4) NOT NULL DEFAULT 0`,
		`ALTER TABLE stock_transfer_items ADD COLUMN IF NOT EXISTS rejected_qty_base NUMERIC(18,4) NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS stock_transfer_receipts (
			id CHAR(26) PRIMARY KEY, transfer_id CHAR(26) NOT NULL REFERENCES stock_transfers(id) ON DELETE CASCADE,
			evidence_id CHAR(26) NOT NULL REFERENCES stock_evidence(id), actor_id VARCHAR(100) NOT NULL,
			actor_name VARCHAR(100) NOT NULL, notes TEXT NOT NULL DEFAULT '',
			items JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrasi mutu stok: %w", err)
		}
	}
	for _, role := range []string{"admin", "superadmin"} {
		for _, perm := range StockQualityPermissions {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, perm); err != nil {
				return err
			}
		}
	}
	roles := []struct {
		name, description, scope string
		perms                    []string
	}{
		{"gudang_induk_qc", "Gudang Induk: laporkan reject/retur/expired/spoil dan terima barang pengganti/retur outlet", "all", []string{"stockquality.central.report", "stockquality.central.receive", "stocktransfers.view", "stocktransfers.reject"}},
		{"gudang_induk_supervisor", "Gudang Induk: keputusan karantina, retur supplier, dan pemusnahan", "all", []string{"stockquality.central.approve", "stocktransfers.view", "stocktransfers.approve"}},
		{"gudang_induk_retur", "Gudang Induk: kirim retur supplier dan laksanakan pemusnahan yang disetujui", "all", []string{"stockquality.central.execute", "stocktransfers.view", "stocktransfers.update"}},
		{"finance_refund_supplier", "Finance: catat pengembalian dana supplier beserta bukti transaksi", "all", []string{"stockquality.refund"}},
		{"gudang_outlet_penerima", "Outlet: terima penuh/sebagian, reject, dan laporkan stok bermasalah", "specific", []string{"stockquality.outlet.report", "stockquality.outlet.receive", "stocktransfers.view", "stocktransfers.reject"}},
		{"gudang_outlet_supervisor", "Outlet: setujui retur/pemusnahan atau pelepasan karantina", "specific", []string{"stockquality.outlet.approve"}},
		{"gudang_outlet_pelaksana", "Outlet: kirim retur ke induk dan laksanakan pemusnahan yang disetujui", "specific", []string{"stockquality.outlet.execute"}},
	}
	for _, role := range roles {
		res, err := tx.Exec(`INSERT INTO roles(name,description,is_system,scope_type,redirect_to) VALUES($1,$2,false,$3,'/stock-quality') ON CONFLICT DO NOTHING`, role.name, role.description, role.scope)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		for _, p := range append([]string{"stockquality.view", "stockitems.view", "stockledger.view", "stockwastes.view"}, role.perms...) {
			if _, err := tx.Exec(`INSERT INTO role_permissions(role,permission) VALUES($1,$2) ON CONFLICT DO NOTHING`, role.name, p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
