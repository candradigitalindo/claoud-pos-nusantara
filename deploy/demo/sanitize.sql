-- Sanitasi salinan produksi untuk lingkungan DEMO.
--
-- Dijalankan deploy/demo.sh reset SETELAH pg_restore dan SEBELUM app demo
-- menyala. Prinsipnya: demo = salinan utuh produksi, KECUALI apa pun yang
-- bisa menjangkau dunia nyata atau berlaku di produksi. Jaringan demo sudah
-- tanpa internet (docker --internal); berkas ini lapis pengaman kedua, supaya
-- demo tetap aman seandainya suatu hari jaringannya dibuka untuk uji integrasi.
--
-- Data yang SAMA SEKALI tidak disalin (pg_dump --exclude-table-data):
-- whatsmeow_* (sesi WhatsApp produksi — memakainya di tempat lain membuat
-- nomor produksi terputus), wa_messages, wa_gateway_events, wa_recipients,
-- wa_broadcasts. Berkas APK (uploads/apps) juga tidak disalin, sehingga tablet
-- latihan tidak pernah ditawari pembaruan dari demo.

\set ON_ERROR_STOP on
BEGIN;

-- 1. Kunci API outlet diganti semua: kunci produksi tidak berlaku di demo, dan
--    kunci yang tampil di layar demo tidak pernah berlaku di produksi. Awalan
--    pos_demo_ membuatnya mudah dikenali bila tertempel di tablet yang salah.
UPDATE outlets
   SET api_key = 'pos_demo_' || replace(gen_random_uuid()::text, '-', '')
                             || replace(gen_random_uuid()::text, '-', '');

-- 2. CCTV: kredensial, alamat kamera, dan token penghubung outlet tidak dibawa.
UPDATE outlet_cameras
   SET rtsp_username = '', rtsp_password_enc = '', source_url_enc = '',
       playback_url_enc = '', host = 'cctv-demo.invalid', is_active = false;
UPDATE outlet_cctv_gateways
   SET token_hash = encode(sha256(convert_to(gen_random_uuid()::text, 'UTF8')), 'hex');

-- 3. WhatsApp: dimatikan, tautan di pesan menunjuk ke demo, nomor staf kosong.
INSERT INTO app_settings (key, value) VALUES ('wa_enabled', 'false')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
INSERT INTO app_settings (key, value) VALUES ('wa_public_base_url', 'https://demo.nbp.co.id')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
UPDATE cloud_admins SET wa_phone = '' WHERE wa_phone <> '';

-- 4. Medsos: tidak ada penarikan otomatis (alamat IP server yang sama dipakai
--    produksi; penarikan ganda berisiko membuat IP diblokir Instagram/TikTok).
UPDATE social_accounts SET auto_fetch = false WHERE auto_fetch;

-- 5. Pencadangan foto ke Google Drive: antrean ditutup.
UPDATE handover_photos
   SET backup_status = 'failed', backup_error = 'demo: pencadangan Google Drive nonaktif'
 WHERE backup_status = 'pending';

-- 6. Nomor HP pelanggan disamarkan. Pemetaan per nilai asli: nomor yang sama
--    di tabel mana pun mendapat samaran yang sama (relasi tetap utuh), nomor
--    berbeda mendapat samaran berbeda (indeks unik customers.phone aman).
--    Awalan 08000 tidak dipakai operator seluler mana pun.
CREATE TEMP TABLE demo_phone_map ON COMMIT DROP AS
SELECT raw,
       '08000' || lpad(row_number() OVER (ORDER BY md5(raw))::text, 8, '0') AS masked
  FROM (
        SELECT phone          AS raw FROM customers    WHERE COALESCE(phone, '') <> ''
        UNION SELECT customer_phone FROM cloud_orders  WHERE COALESCE(customer_phone, '') <> ''
        UNION SELECT customer_phone FROM reservations  WHERE COALESCE(customer_phone, '') <> ''
        UNION SELECT customer_phone FROM online_orders WHERE COALESCE(customer_phone, '') <> ''
       ) s;

UPDATE customers t     SET phone = m.masked          FROM demo_phone_map m WHERE t.phone = m.raw;
UPDATE cloud_orders t  SET customer_phone = m.masked FROM demo_phone_map m WHERE t.customer_phone = m.raw;
UPDATE reservations t  SET customer_phone = m.masked FROM demo_phone_map m WHERE t.customer_phone = m.raw;
UPDATE online_orders t SET customer_phone = m.masked FROM demo_phone_map m WHERE t.customer_phone = m.raw;

-- 7. Semua sesi lama gugur (JWT demo juga ditandatangani rahasia sendiri).
UPDATE cloud_admins SET token_version = token_version + 1;

COMMIT;

-- Ringkasan untuk log reset.
SELECT 'outlet=' || (SELECT count(*) FROM outlets)
    || ' kunci_demo=' || (SELECT count(*) FROM outlets WHERE api_key LIKE 'pos\_demo\_%')
    || ' pelanggan=' || (SELECT count(*) FROM customers)
    || ' hp_disamarkan=' || (SELECT count(*) FROM customers WHERE phone LIKE '08000%')
    || ' order=' || (SELECT count(*) FROM cloud_orders)
    || ' akun=' || (SELECT count(*) FROM cloud_admins WHERE is_active)
    || ' wa_enabled=' || (SELECT value FROM app_settings WHERE key = 'wa_enabled')
    || ' medsos_otomatis=' || (SELECT count(*) FROM social_accounts WHERE auto_fetch)
    AS ringkasan;
