\set ON_ERROR_STOP on

-- Synthetic fixture only. The runner creates a fresh *_test database in tmpfs.
DO $$ BEGIN
  IF current_database() NOT LIKE '%\_test' ESCAPE '\' THEN
    RAISE EXCEPTION 'Integration fixtures require a database ending in _test';
  END IF;
END $$;

INSERT INTO outlets (id, name, code, slug, api_key, is_active, created_at, updated_at)
VALUES ('01E2ETESTOUTLET00000000001', 'Outlet E2E', 'E2E', 'e2e-uji',
        'e2e-api-key', true, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC');

-- Independent recovery fixture: its POS uploads and restores its own catalog,
-- shifts and payment snapshots without sharing operational data with other E2E.
INSERT INTO outlets (id, name, code, slug, api_key, is_active, created_at, updated_at)
VALUES ('01E2ERESTOREOUTLET00000001', 'Outlet Restore E2E', 'E2ER', 'e2e-restore',
        'e2e-restore-key', true, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC');

INSERT INTO cloud_cashier_shifts (id, local_id, outlet_id, opened_by, opened_at, opening_cash, status)
VALUES ('01E2ETESTSHIFT00000000001A', '01E2ETESTSHIFT00000000001A',
        '01E2ETESTOUTLET00000000001', 'Kasir E2E', now() AT TIME ZONE 'UTC', 0, 'open');

INSERT INTO cloud_products (id, local_id, outlet_id, name, price, updated_at)
VALUES ('01E2ETESTPRODUK00000000001', '01E2ETESTPRODUK00000000001',
        '01E2ETESTOUTLET00000000001', 'Nasi Goreng', 25000, now() AT TIME ZONE 'UTC');

INSERT INTO cloud_product_addons (id, local_id, outlet_id, product_local_id, name, price, is_active)
VALUES ('01E2ETESTADDON00000000001A', '01E2ETESTADDON00000000001A',
        '01E2ETESTOUTLET00000000001', '01E2ETESTPRODUK00000000001', 'Extra keju', 5000, true);

INSERT INTO cloud_additional_charges (id, local_id, outlet_id, name, charge_type, value, is_active)
VALUES ('01E2ETESTCHARGE00000000001', '01E2ETESTCHARGE00000000001',
        '01E2ETESTOUTLET00000000001', 'Pajak Restoran (PB1)', 'percentage', 10, true);
