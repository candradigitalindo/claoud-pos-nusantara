# Perlengkapan & Aset — Pendataan, Manajemen, Mutasi, Perawatan & Penerimaan dari Pengadaan

| | |
|---|---|
| **Status** | **Fase 1 TAYANG** (15 Sep 2026) · **Fase 2, 3 & 4 SELESAI, belum dirilis** (16 Sep 2026) — F2: mutasi antar outlet (`MTA-…`, draft→pending→approved→sent→received, pemecahan/penggabungan baris, berita acara). F3: work order perawatan (`WOM-…`, dijadwalkan→berjalan→selesai), daftar jatuh tempo, penjadwal harian, interval per kategori, tautan ke Pengadaan Jasa. F4: dialog Serah Terima pengadaan → aset/stok, penghapusan (`DSP-…`), opname (`OPA-…`), lima laporan + export Excel. F5: material projek (terima → pakai → sisa) + panel material & aset di halaman Projek. **Seluruh fase selesai.** |
| **Tanggal** | 15 September 2026 |
| **Modul terdampak** | Perlengkapan (`assets`), Pengadaan (`purchase_requests`), Gudang (GRN), Projek, Keuangan (penyusutan & biaya perawatan), Sidebar/UI, Permission, Database |
| **Referensi** | `docs/kontrak-waktu-transaksi-utc.md`, `docs/pengembangan-sidebar-ppic.md` (pola dokumen & pola transfer stok) |
| **Berkas kunci** | [models/asset.go](../models/asset.go), [services/asset.go](../services/asset.go), [handlers/asset.go](../handlers/asset.go), [ui/src/pages/Assets.vue](../ui/src/pages/Assets.vue), [routes/routes.go](../routes/routes.go) |

---

## 1. Latar Belakang

### 1.1 Yang sudah ada hari ini

| Lapisan | Kondisi |
|---|---|
| Tabel | `assets` (outlet_id, code, name, category, quantity, unit, condition, location, purchase_date, purchase_price, notes, soft delete) dan `asset_maintenances` (tanggal, tipe, deskripsi, biaya, pelaksana, kondisi setelah, jadwal berikutnya) — [database/migrations.go:677](../database/migrations.go#L677) |
| API | 8 endpoint: CRUD aset + list/tambah/hapus catatan perawatan — [routes/routes.go:276](../routes/routes.go#L276) |
| Hak akses | 4 kunci: `assets.view`, `assets.create`, `assets.update`, `assets.delete` |
| UI | Satu halaman `/perlengkapan` di grup sidebar **Master Data**: tabel + filter outlet/kondisi/pencarian, modal tambah-edit, modal riwayat perawatan |
| Batasan data | Sudah ter-scope per outlet lewat `getOutletScope` / `validateOutletAccess` |

Artinya: **pendataan dasar dan catatan perawatan sudah jalan.** Yang belum ada adalah
semua hal yang membuat aset bisa dikelola sebagai aset, bukan sekadar daftar.

### 1.2 Enam celah yang ditutup dokumen ini

1. **Pendataan belum terstandar.** Kode/tag diketik bebas (boleh kosong, boleh kembar),
   tidak ada nomor seri, merk/tipe, garansi, penanggung jawab, foto, atau sumber perolehan.
   Tidak ada audit fisik (opname aset), jadi selisih catatan-vs-lapangan tidak pernah ketahuan.
2. **Tidak ada siklus hidup.** Satu aset hanya punya `condition`. Tidak ada status
   (aktif / dipinjam / dalam perbaikan / transit / dihapus), tidak ada penyusutan, tidak ada
   nilai buku, tidak ada penghapusan (disposal) yang tercatat — aset rusak berat hanya
   bisa dihapus lewat soft delete, tanpa berita acara dan tanpa jejak.
3. **Tidak ada pemindahan antar outlet.** Satu-satunya cara memindahkan meja dari Outlet A
   ke Outlet B adalah mengedit `outlet_id`. Tidak ada persetujuan, tidak ada status kirim/terima,
   tidak ada bukti serah terima, dan **tidak ada jejak sama sekali** — riwayat lokasi aset hilang.
4. **Perawatan hanya berupa catatan mundur.** `next_due_date` sudah disimpan tapi tidak pernah
   dibaca: tak ada daftar jatuh tempo, tak ada pengingat, tak ada status pengerjaan, tak ada
   kaitan ke Pengadaan Jasa saat perbaikan dikerjakan vendor.
5. **Barang habis pakai projek tidak berwujud data.** Modul Projek hanya memegang RAB
   dan rekap uang yang dihitung dari `purchase_requests` ([models/project.go](../models/project.go));
   tidak ada satu pun tabel yang mencatat **barang apa yang datang ke lokasi projek**.
   Semen, cat, keramik, dan kabel senilai puluhan juta hanya berupa angka `paid` —
   tidak ada catatan berapa yang dipakai, berapa yang tersisa saat projek selesai, dan
   ke mana sisanya pergi. Inilah titik kebocoran yang paling sulit dibuktikan.
6. **Pengadaan dan Aset terputus.** Aksi **Serah Terima** pada pengajuan pengadaan barang
   ([services/purchase.go:869](../services/purchase.go#L869)) hanya mengubah `status` menjadi
   `received` dan mengisi `received_by/received_at`. AC, kulkas, dan kursi yang baru dibeli
   **tidak otomatis menjadi baris aset** — petugas harus mengetik ulang seluruhnya, atau
   (yang lebih sering terjadi) tidak mengetiknya sama sekali.

### 1.3 Prinsip rancangan

- **Menyambung, bukan memparalelkan.** Mutasi aset meniru pola `stock_transfers`
  (draft → pending → sent → received) yang sudah dipahami tim; penerimaan aset menempel pada
  aksi `receive` pengadaan yang sudah ada, bukan alur baru di samping.
- **Setiap perubahan aset meninggalkan jejak.** Satu buku besar aset (`asset_movements`)
  mencatat pendataan, mutasi, perubahan kondisi, perawatan, opname, dan penghapusan.
- **Hak akses granular per submenu**, konsisten dengan pola `AllPermissions`.
- **Angka uang bisa diaudit**: nilai buku dihitung dari kolom yang tersimpan, bukan dari
  tabel turunan yang bisa basi.

---

## 2. Ruang Lingkup & Batasan Istilah

| Istilah | Definisi di sistem ini | Tabel |
|---|---|---|
| **Aset / Perlengkapan** | Barang **tidak habis pakai** yang dipakai berulang untuk operasional: meja, kursi, AC, kulkas, mesin kopi, POS, CCTV, kendaraan, perkakas. | `assets` |
| **Stok / bahan** | Barang **habis pakai** yang masuk resep atau dijual: bahan baku, kemasan, minuman. | `stock_items` + `stock_ledger` |
| **Material projek** | Barang **habis pakai yang dikonsumsi sebuah projek** pembangunan/renovasi: semen, cat, keramik, kabel, pipa, paku. Bukan aset (habis), bukan stok dapur (tidak masuk resep, tidak dijual). | `project_materials` (baru) |
| **Produk** | Yang dijual di kasir. | `products` |

**Aturan pemilahan saat barang datang** (dipakai di Bagian 8 — seluruh ambang di bawah
adalah usulan, angka final ditetapkan manajemen dan disimpan di `app_settings`):

> 1. Barang yang **ada di katalog `stock_items`** → **stok gudang** (GRN), **berapa pun
>    harganya**. Ini diperiksa lebih dulu dari ambang harga: daging premium, keju impor,
>    dan bahan mahal lain adalah urusan gudang/dapur, bukan aset — lihat kotak di bawah.
> 2. Sisanya, bila pengajuannya **terikat sebuah projek** (`project_id` terisi) →
>    **material projek**.
> 3. Sisanya, bila **umur pakainya > 12 bulan** **dan** harga satuannya **≥ Rp 500.000**
>    → **aset**.
> 4. Selebihnya → **habis pakai langsung**: hanya menjadi biaya, tidak dicatat sebagai barang.
>
> Barang murah tapi awet dan penting (mis. perkakas dapur Rp 150.000) boleh dinaikkan
> manual menjadi aset dengan mencentang “Catat sebagai aset” — ambang hanya usulan default,
> bukan larangan. Urutan di atas juga hanya **usulan yang terisi otomatis** di dialog
> penerimaan; petugas selalu bisa memindahkan baris ke tujuan lain.

> **Batas tanggung jawab: tim aset hanya menerima data barang perlengkapan, bukan barang
> dapur.** Karena itu urutan di atas memeriksa katalog stok **sebelum** ambang harga. Kalau
> ambang harga menang duluan, daging wagyu Rp 2.500.000/kg akan diusulkan menjadi aset dan
> mendarat di meja tim aset — padahal barang itu habis dimasak minggu itu juga. Konsekuensinya
> di seluruh modul:
>
> - **Dialog Serah Terima** memisahkan barisnya menjadi dua kelompok berlabel
>   *"Perlengkapan & aset"* (dicatat tim aset) dan *"Barang dapur / gudang"*, sehingga siapa
>   mengerjakan apa terlihat sebelum diklik.
> - **Laporan "Pengadaan Belum Lengkap"** di modul aset hanya memuat baris perlengkapan.
>   Bahan dapur yang belum masuk buku stok adalah pekerjaan gudang, dan menampilkannya di
>   sini hanya membuat tim aset menagih pekerjaan orang lain. Parameter `?kind=semua`
>   membuka pandangan lintas tim bila memang dibutuhkan.
> - **Daftar aset tidak pernah bisa berisi bahan dapur** secara struktural: aset hidup di
>   tabel `assets`, bahan di `stock_items` + `stock_ledger` (Bagian 2).

**Di luar lingkup dokumen ini:** aset tak berwujud (lisensi, sewa), penyusutan fiskal
menurut aturan pajak (yang dipakai di sini penyusutan garis lurus untuk manajemen internal),
dan integrasi ke jurnal akuntansi eksternal.

---

## 3. Arsitektur Data

Semua tabel mengikuti konvensi repo: id `CHAR(26)` ULID (`services.NewULID()`),
timestamp `TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')`, penghapusan lewat `is_deleted`,
penomoran dokumen lewat `nextDocNumber(tx, table, column, prefix)`
([services/warehouse.go:1092](../services/warehouse.go#L1092)).

### 3.1 `assets` — kolom tambahan

Kolom lama tidak diubah; semua tambahan `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`
sehingga data existing tetap valid.

```sql
ALTER TABLE assets
  ADD COLUMN IF NOT EXISTS asset_no        VARCHAR(40) DEFAULT '',   -- nomor resmi, unik, dibuat sistem
  ADD COLUMN IF NOT EXISTS tracking_mode   VARCHAR(10)  DEFAULT 'massal', -- tunggal | massal
  ADD COLUMN IF NOT EXISTS serial_number   VARCHAR(80)  DEFAULT '',
  ADD COLUMN IF NOT EXISTS brand           VARCHAR(80)  DEFAULT '',
  ADD COLUMN IF NOT EXISTS model           VARCHAR(80)  DEFAULT '',
  ADD COLUMN IF NOT EXISTS status          VARCHAR(20)  DEFAULT 'aktif',
  ADD COLUMN IF NOT EXISTS pic_name        VARCHAR(100) DEFAULT '',  -- penanggung jawab
  ADD COLUMN IF NOT EXISTS work_unit_id    CHAR(26),                 -- unit kerja pemakai (opsional)
  ADD COLUMN IF NOT EXISTS acquisition_src VARCHAR(20)  DEFAULT 'manual', -- manual|pengadaan|hibah|opname|mutasi
  ADD COLUMN IF NOT EXISTS purchase_request_id CHAR(26),             -- asal pengadaan
  ADD COLUMN IF NOT EXISTS pr_item_key     VARCHAR(120) DEFAULT '',  -- kunci idempotensi baris PR
  ADD COLUMN IF NOT EXISTS warranty_until  DATE,
  ADD COLUMN IF NOT EXISTS useful_life_months INT       DEFAULT 0,   -- 0 = tidak disusutkan
  ADD COLUMN IF NOT EXISTS residual_value  DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS photo_url       TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS disposed_at     TIMESTAMP;

CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_asset_no
  ON assets(asset_no) WHERE asset_no <> '' AND is_deleted = false;
CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status);
CREATE INDEX IF NOT EXISTS idx_assets_pr     ON assets(purchase_request_id);
```

**`tracking_mode` — dua cara sebuah baris mewakili barang:**

| Mode | Arti | Contoh | Aturan |
|---|---|---|---|
| `tunggal` | Satu baris = satu unit fisik, `quantity` dipaksa 1 | AC, kulkas, motor, mesin kopi | Wajib punya `asset_no`; `serial_number` sangat dianjurkan; perawatan & garansi per unit |
| `massal` | Satu baris = sekumpulan unit identik, `quantity` > 1 | 40 kursi, 12 meja, 25 gelas kaca | `asset_no` mewakili kelompok; mutasi sebagian memecah baris (lihat 6.4) |

Aset baru dari pengadaan bernilai tinggi otomatis diusulkan `tunggal`; qty > 1 pada mode
`tunggal` menghasilkan **N baris terpisah**, masing-masing `asset_no` sendiri.

**`status` vs `condition` — dua sumbu berbeda, jangan dicampur:**

| Sumbu | Nilai | Makna |
|---|---|---|
| `condition` (fisik) | `baik`, `rusak_ringan`, `rusak_berat` | Bagaimana kondisi barangnya |
| `status` (administratif) | `aktif`, `dipinjam`, `perbaikan`, `transit`, `tidak_aktif`, `dihapus` | Di mana posisinya dalam siklus hidup |

Nilai lama `condition = 'perbaikan'` (dipakai UI sekarang) dimigrasikan sekali jalan menjadi
`status = 'perbaikan'` dengan `condition` diturunkan ke `rusak_ringan`; nilai `perbaikan`
dihapus dari daftar pilihan kondisi di UI.

### 3.2 `asset_movements` — buku besar aset

Satu tabel append-only yang menjawab “barang ini pernah ke mana dan kenapa”.
Tidak pernah di-UPDATE, tidak pernah di-DELETE.

```sql
CREATE TABLE IF NOT EXISTS asset_movements (
  id           CHAR(26) PRIMARY KEY,
  asset_id     CHAR(26) NOT NULL REFERENCES assets(id),
  type         VARCHAR(24) NOT NULL,  -- pendataan|penerimaan|mutasi_keluar|mutasi_masuk|
                                      -- kondisi|perawatan|opname|penghapusan|pemecahan
  qty          INT DEFAULT 0,         -- + masuk / - keluar (mode massal)
  from_outlet_id CHAR(26),
  to_outlet_id   CHAR(26),
  from_location  VARCHAR(150) DEFAULT '',
  to_location    VARCHAR(150) DEFAULT '',
  condition_before VARCHAR(20) DEFAULT '',
  condition_after  VARCHAR(20) DEFAULT '',
  ref_type     VARCHAR(24) DEFAULT '', -- asset_transfer|purchase_request|maintenance|disposal|opname
  ref_id       CHAR(26),
  ref_number   VARCHAR(40) DEFAULT '',
  amount       DECIMAL(15,2) DEFAULT 0, -- biaya/nilai yang menyertai (perawatan, perolehan)
  notes        TEXT DEFAULT '',
  actor        VARCHAR(100) DEFAULT '',
  created_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);
CREATE INDEX IF NOT EXISTS idx_asset_mov_asset ON asset_movements(asset_id, created_at);
CREATE INDEX IF NOT EXISTS idx_asset_mov_ref   ON asset_movements(ref_type, ref_id);
```

### 3.3 `asset_transfers` + `asset_transfer_items` — mutasi antar outlet

Meniru `stock_transfers` supaya alur dan istilahnya sama dengan transfer stok.

```sql
CREATE TABLE IF NOT EXISTS asset_transfers (
  id              CHAR(26) PRIMARY KEY,
  transfer_number VARCHAR(30) NOT NULL UNIQUE,   -- MTA260915001
  from_outlet_id  CHAR(26) NOT NULL REFERENCES outlets(id),
  to_outlet_id    CHAR(26) NOT NULL REFERENCES outlets(id),
  status          VARCHAR(20) NOT NULL DEFAULT 'draft',
  reason          VARCHAR(40)  DEFAULT '',  -- relokasi|pinjam|perbaikan|penyeimbangan|lainnya
  expected_return DATE,                     -- diisi bila reason = pinjam/perbaikan
  notes           TEXT DEFAULT '',
  created_by      VARCHAR(100) DEFAULT '',
  approved_by     VARCHAR(100), approved_at TIMESTAMP,
  sent_by         VARCHAR(100), sent_at     TIMESTAMP,
  received_by     VARCHAR(100), received_at TIMESTAMP,
  rejected_reason TEXT DEFAULT '',
  created_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
  updated_at      TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);

CREATE TABLE IF NOT EXISTS asset_transfer_items (
  id            CHAR(26) PRIMARY KEY,
  transfer_id   CHAR(26) NOT NULL REFERENCES asset_transfers(id) ON DELETE CASCADE,
  asset_id      CHAR(26) NOT NULL REFERENCES assets(id),
  qty           INT NOT NULL DEFAULT 1,
  received_qty  INT,                        -- NULL sebelum diterima
  condition_sent  VARCHAR(20) DEFAULT '',
  condition_recv  VARCHAR(20) DEFAULT '',
  target_asset_id CHAR(26),                 -- baris tujuan hasil pemecahan/penggabungan
  notes         TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_asset_tr_status ON asset_transfers(status);
CREATE INDEX IF NOT EXISTS idx_asset_tr_outlets ON asset_transfers(from_outlet_id, to_outlet_id);
```

### 3.4 `asset_maintenances` — kolom tambahan

```sql
ALTER TABLE asset_maintenances
  ADD COLUMN IF NOT EXISTS wo_number   VARCHAR(30) DEFAULT '',   -- WOM260915001
  ADD COLUMN IF NOT EXISTS status      VARCHAR(20) DEFAULT 'selesai', -- dijadwalkan|berjalan|selesai|batal
  ADD COLUMN IF NOT EXISTS scheduled_date DATE,
  ADD COLUMN IF NOT EXISTS vendor_id   CHAR(26),
  ADD COLUMN IF NOT EXISTS purchase_request_id CHAR(26),  -- bila perbaikan lewat Pengadaan Jasa
  ADD COLUMN IF NOT EXISTS downtime_hours DECIMAL(10,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS attachment_url TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_asset_mtc_due
  ON asset_maintenances(next_due_date) WHERE next_due_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_asset_mtc_status ON asset_maintenances(status);
```

Catatan kompatibilitas: baris lama tidak punya status → default `'selesai'` benar secara
historis, karena selama ini form hanya dipakai mencatat pekerjaan yang sudah terjadi.

### 3.5 `asset_disposals` — penghapusan aset

```sql
CREATE TABLE IF NOT EXISTS asset_disposals (
  id             CHAR(26) PRIMARY KEY,
  disposal_number VARCHAR(30) NOT NULL UNIQUE,  -- DSP260915001
  asset_id       CHAR(26) NOT NULL REFERENCES assets(id),
  qty            INT NOT NULL DEFAULT 1,
  method         VARCHAR(20) NOT NULL,   -- dijual|dihibahkan|dimusnahkan|hilang|tukar_tambah
  reason         TEXT NOT NULL,
  proceeds       DECIMAL(15,2) DEFAULT 0, -- hasil penjualan bila dijual
  book_value     DECIMAL(15,2) DEFAULT 0, -- nilai buku saat dihapus (snapshot)
  status         VARCHAR(20) DEFAULT 'pending', -- pending|approved|rejected
  requested_by   VARCHAR(100) DEFAULT '',
  approved_by    VARCHAR(100), approved_at TIMESTAMP,
  rejected_reason TEXT DEFAULT '',
  attachment_url TEXT DEFAULT '',
  created_at     TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);
```

`book_value` sengaja di-snapshot: nilai buku dihitung on-the-fly dari `purchase_date`
(Bagian 5.3), sehingga tanpa snapshot angkanya ikut berubah setiap hari dan berita acara
lama jadi tidak cocok dengan dokumen cetaknya.

### 3.6 Opname aset (audit fisik)

```sql
CREATE TABLE IF NOT EXISTS asset_opname_sessions (
  id           CHAR(26) PRIMARY KEY,
  opname_number VARCHAR(30) NOT NULL UNIQUE,   -- OPA260915001
  outlet_id    CHAR(26) NOT NULL REFERENCES outlets(id),
  status       VARCHAR(20) DEFAULT 'berjalan', -- berjalan|selesai|batal
  notes        TEXT DEFAULT '',
  created_by   VARCHAR(100) DEFAULT '',
  approved_by  VARCHAR(100), approved_at TIMESTAMP,
  created_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);

CREATE TABLE IF NOT EXISTS asset_opname_items (
  id          CHAR(26) PRIMARY KEY,
  session_id  CHAR(26) NOT NULL REFERENCES asset_opname_sessions(id) ON DELETE CASCADE,
  asset_id    CHAR(26) REFERENCES assets(id),  -- NULL = temuan barang tak terdaftar
  system_qty  INT DEFAULT 0,
  counted_qty INT DEFAULT 0,
  condition_found VARCHAR(20) DEFAULT '',
  found_name  VARCHAR(150) DEFAULT '',  -- diisi bila asset_id NULL
  location    VARCHAR(150) DEFAULT '',
  notes       TEXT DEFAULT ''
);
```

### 3.7 Penomoran dokumen

Semua memakai `nextDocNumber` (advisory lock per tabel, aman dari balapan) dan
`GetTimezoneLocation()` supaya tanggal pada nomor mengikuti zona waktu aplikasi,
bukan UTC.

| Dokumen | Format | Contoh |
|---|---|---|
| Nomor aset | `AST-{kode outlet}-{YYMM}-{seq 3}` | `AST-NHC01-2609-007` |
| Mutasi aset | `MTA{YYMMDD}{seq 3}` | `MTA260915001` |
| Work order perawatan | `WOM{YYMMDD}{seq 3}` | `WOM260915002` |
| Penghapusan | `DSP{YYMMDD}{seq 3}` | `DSP260915001` |
| Opname aset | `OPA{YYMMDD}{seq 3}` | `OPA260915001` |

> `nextDocNumber` mencocokkan pola `'^' || prefix || '(\d+)$'`, jadi **prefix harus berakhir
> tepat sebelum bagian angka** — untuk nomor aset prefix-nya `AST-NHC01-2609-`.

### 3.8 `project_materials` — material habis pakai milik projek

Tabel ini adalah **buku stok lokasi projek**: barang yang sudah dibayar dan sudah datang,
tapi belum tentu sudah terpakai. Sengaja **terpisah dari `stock_items`/`stock_ledger`**,
karena semen dan keramik bukan bahan F&B — memasukkannya ke katalog stok akan mengotori
resep, HPP, MRP, dan seluruh laporan PPIC yang dihitung dari katalog itu.

```sql
CREATE TABLE IF NOT EXISTS project_materials (
  id           CHAR(26) PRIMARY KEY,
  project_id   CHAR(26) NOT NULL REFERENCES projects(id),
  purchase_request_id CHAR(26),
  pr_item_key  VARCHAR(120) DEFAULT '',      -- kunci idempotensi, sama seperti aset (8.4)
  name         VARCHAR(150) NOT NULL,
  unit         VARCHAR(20)  DEFAULT '',
  qty_received DECIMAL(15,4) NOT NULL DEFAULT 0,
  qty_used     DECIMAL(15,4) NOT NULL DEFAULT 0,
  qty_returned DECIMAL(15,4) NOT NULL DEFAULT 0,  -- sisa yang ditarik ke gudang/outlet
  qty_wasted   DECIMAL(15,4) NOT NULL DEFAULT 0,  -- rusak/hilang, wajib beralasan
  unit_cost    DECIMAL(15,2) DEFAULT 0,
  location     VARCHAR(150) DEFAULT '',      -- titik simpan di lokasi projek
  received_at  TIMESTAMP,
  received_by  VARCHAR(100) DEFAULT '',
  notes        TEXT DEFAULT '',
  created_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC'),
  updated_at   TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_materials_pr_item
  ON project_materials(purchase_request_id, pr_item_key)
  WHERE purchase_request_id IS NOT NULL AND pr_item_key <> '';
CREATE INDEX IF NOT EXISTS idx_project_materials_project ON project_materials(project_id);

CREATE TABLE IF NOT EXISTS project_material_logs (
  id          CHAR(26) PRIMARY KEY,
  material_id CHAR(26) NOT NULL REFERENCES project_materials(id) ON DELETE CASCADE,
  type        VARCHAR(20) NOT NULL,   -- penerimaan|pemakaian|pengembalian|susut|koreksi
  qty         DECIMAL(15,4) NOT NULL,
  ref_type    VARCHAR(24) DEFAULT '', -- purchase_request|goods_receipt|asset|disposal
  ref_id      CHAR(26),
  ref_number  VARCHAR(40) DEFAULT '',
  notes       TEXT DEFAULT '',
  actor       VARCHAR(100) DEFAULT '',
  created_at  TIMESTAMP DEFAULT (now() AT TIME ZONE 'UTC')
);
CREATE INDEX IF NOT EXISTS idx_project_material_logs_mat
  ON project_material_logs(material_id, created_at);
```

Identitas yang selalu berlaku — dijaga di service, bukan hanya di UI:

```
qty_received  =  Σ log 'penerimaan'
qty_received  =  qty_used + qty_returned + qty_wasted + sisa
```

`sisa` tidak disimpan sebagai kolom (agar tidak bisa basi), melainkan dihitung.
`project_material_logs` bersifat append-only seperti `asset_movements`: koreksi dilakukan
dengan menambah baris bertipe `koreksi`, bukan mengubah baris lama.

---

## 4. Pendataan Aset

### 4.1 Empat pintu masuk data

| Sumber | `acquisition_src` | Pemicu | Bagian |
|---|---|---|---|
| Input manual | `manual` | Petugas menambah lewat halaman Perlengkapan | 4.2 |
| Penerimaan pengadaan | `pengadaan` | Aksi **Serah Terima** pada pengajuan pengadaan barang | 8 |
| Temuan opname | `opname` | Barang ada di lapangan tapi tidak ada di sistem | 4.5 |
| Mutasi masuk | `mutasi` | Baris baru hasil mutasi dari outlet lain | 6.4 |

Apa pun pintunya, hasilnya satu: baris `assets` + satu baris `asset_movements`
bertipe `pendataan`/`penerimaan`/`mutasi_masuk`.

### 4.2 Field: wajib, dianjurkan, opsional

| Kelompok | Field | Aturan |
|---|---|---|
| **Wajib** | `name`, `outlet_id`, `category`, `quantity`, `unit`, `condition`, `status` | `name` & `outlet_id` sudah divalidasi hari ini ([services/asset.go](../services/asset.go)); sisanya punya default |
| **Wajib bila `tracking_mode = tunggal`** | `asset_no` (dibuat sistem), `quantity = 1` | Ditolak bila qty > 1 |
| **Dianjurkan** | `serial_number`, `brand`, `model`, `purchase_date`, `purchase_price`, `location`, `pic_name` | Diberi tanda “lengkapi data” di UI; ikut skor kelengkapan (4.6) |
| **Opsional** | `warranty_until`, `useful_life_months`, `residual_value`, `work_unit_id`, `photo_url`, `notes` | `useful_life_months` kosong → dipakai default per kategori (5.3) |
| **Terkunci sistem** | `asset_no`, `acquisition_src`, `purchase_request_id`, `pr_item_key`, `disposed_at` | Tidak bisa diubah lewat form |

Field lama `code` tetap dipertahankan sebagai **kode/tag internal bebas** (mis. nomor
stiker lama yang sudah tertempel), terpisah dari `asset_no` yang dibuat sistem.

### 4.3 Label & QR

Setiap aset bisa dicetak labelnya dari halaman detail:
`asset_no` + nama + outlet + QR berisi URL `/perlengkapan/{id}`. Tujuannya satu — petugas
opname dan teknisi cukup memindai, tidak mengetik. Pencetakan memakai layout HTML print
(ukuran 50×25 mm dan 70×40 mm), tanpa pustaka eksternal selain generator QR yang sudah
dipakai di UI.

Pemindaian QR di halaman opname langsung menandai baris terhitung — ini yang membuat
opname 300 kursi selesai dalam satu sesi, bukan satu sore.

### 4.4 Foto & lampiran

Foto aset diunggah lewat endpoint `POST /admin/upload` yang sudah ada
([handlers/upload.go](../handlers/upload.go), tersimpan di `/uploads/YYYY-MM/`).
Izin endpoint itu saat ini terbatas pada `finance.payments.view` / `procurement.requests.view`
([routes/routes.go:261](../routes/routes.go#L261)), jadi `assets.create` dan `assets.update`
**harus ditambahkan** ke daftar `RequireAnyPermission`-nya, kalau tidak petugas aset akan
kena 403 saat mengunggah foto.

### 4.5 Opname aset (audit fisik)

Alur per outlet:

```
Buat sesi (pilih outlet, opsional filter kategori/lokasi)
   → sistem membekukan daftar aset outlet itu sebagai baris opname (system_qty)
   → petugas menghitung: scan QR / centang manual → counted_qty + condition_found
   → barang tak terdaftar dicatat sebagai baris temuan (asset_id NULL + found_name)
   → Selesai → tampilkan selisih → Setujui (butuh assets.opname.approve)
   → sistem menerapkan: qty disesuaikan, kondisi diperbarui,
     temuan menjadi aset baru (acquisition_src = 'opname'),
     aset hilang menjadi usulan penghapusan (method = 'hilang', status = 'pending')
```

Setiap penerapan menulis `asset_movements` bertipe `opname` dengan `ref_type = 'opname'`,
sehingga selisih bisa ditelusuri ke sesi mana yang menyebabkannya.

**KPI**: akurasi pendataan aset = `1 − (jumlah baris selisih ÷ jumlah baris diperiksa)`,
target ≥ 98%. Opname dijadwalkan minimal **dua kali setahun per outlet**; outlet yang
belum diopname > 6 bulan muncul sebagai peringatan di dashboard.

### 4.6 Skor kelengkapan data

Dashboard menampilkan persentase aset yang punya `asset_no`, `purchase_date`,
`purchase_price`, `location`, dan `pic_name`. Ini yang mendorong data lama dilengkapi:
tanpa `purchase_price` dan `purchase_date`, seluruh laporan penyusutan di Bagian 5.3
tidak bisa menghitung apa pun untuk aset tersebut.

---

## 5. Manajemen Aset (Siklus Hidup)

### 5.1 Status dan perpindahannya

```
                     ┌──────────────┐
      pendataan  ───▶│    aktif     │◀──────────────┐
                     └──────┬───────┘               │
          ┌─────────────────┼─────────────────┐     │
          ▼                 ▼                 ▼     │
   ┌────────────┐    ┌────────────┐    ┌──────────┐ │
   │  dipinjam  │    │ perbaikan  │    │ transit  │ │ (mutasi diterima)
   └─────┬──────┘    └─────┬──────┘    └────┬─────┘ │
         │ kembali         │ selesai        └───────┘
         └─────────────────┴──────▶ aktif
                           │ tidak bisa diperbaiki
                           ▼
                    ┌──────────────┐   disetujui   ┌──────────┐
                    │ tidak_aktif  │──────────────▶│ dihapus  │
                    └──────────────┘  (disposal)   └──────────┘
```

| Status | Kapan | Siapa yang mengubah |
|---|---|---|
| `aktif` | Dipakai normal | default |
| `dipinjam` | Dipinjam outlet/unit lain sementara (`reason = pinjam` pada mutasi, ada `expected_return`) | sistem, lewat mutasi |
| `perbaikan` | Ada work order perawatan berstatus `berjalan` | sistem, lewat modul perawatan (sejak Fase 3 status ini **tidak lagi bisa diisi manual** — cara menandai barang sedang diperbaiki adalah membuat work order lalu menekan "Mulai") |
| `transit` | Mutasi sudah `sent` tapi belum `received` | sistem, lewat mutasi |
| `tidak_aktif` | Tidak dipakai, disimpan di gudang | manual (`assets.update`) |
| `dihapus` | Penghapusan disetujui | sistem, lewat disposal |

Aturan penting: **status yang dikendalikan sistem tidak boleh diubah manual.** Form edit
menonaktifkan pilihan `transit`, `perbaikan`, dan `dihapus`; keluar dari status itu hanya
lewat dokumen yang menyebabkannya. Tanpa aturan ini, aset bisa “pulang” dari transit
tanpa pernah diterima, dan dua outlet mengklaim barang yang sama.

### 5.2 Penanggung jawab, lokasi, dan unit kerja

- `location` = ruang/area fisik dalam outlet (“Dapur”, “Ruang Meeting Lt.2”).
- `pic_name` = nama penanggung jawab. Perubahan PIC dan lokasi **selalu** menulis
  `asset_movements` bertipe `kondisi` dengan `from_location`/`to_location` terisi,
  sehingga pertanyaan “siapa yang terakhir pegang” punya jawaban.
- `work_unit_id` opsional, mengikat aset ke unit kerja untuk kebutuhan laporan
  dan batasan akses (Bagian 9).

### 5.3 Penyusutan & nilai buku

Metode: **garis lurus bulanan**, dihitung saat query — tidak ada tabel jurnal penyusutan
yang perlu di-posting tiap bulan (dan karenanya tidak ada risiko lupa posting).

```
penyusutan_per_bulan = (purchase_price − residual_value) / useful_life_months
bulan_berjalan       = selisih bulan penuh antara purchase_date dan tanggal laporan
akumulasi            = MIN(penyusutan_per_bulan × bulan_berjalan,
                           purchase_price − residual_value)
nilai_buku           = purchase_price − akumulasi        (minimum = residual_value)
```

Aset dengan `useful_life_months = 0` dianggap **tidak disusutkan**; nilai bukunya tetap
`purchase_price`. Aset berstatus `dihapus` keluar dari perhitungan sejak `disposed_at`.

Default umur ekonomis per kategori (disimpan di `app_settings`, bisa diubah manajemen):

| Kategori | Bulan | Kategori | Bulan |
|---|---|---|---|
| Elektronik / IT | 36 | Peralatan dapur | 60 |
| Mebel (meja, kursi, rak) | 60 | Mesin (kopi, es, pendingin) | 60 |
| Kendaraan | 96 | Perkakas & lain-lain | 48 |

Dalam SQL, `bulan_berjalan` dihitung sepenuhnya di sisi database supaya ikut zona waktu
aplikasi dan tidak bergeser oleh zona browser:

```sql
GREATEST(0, (DATE_PART('year', AGE(CURRENT_DATE, a.purchase_date)) * 12
           + DATE_PART('month', AGE(CURRENT_DATE, a.purchase_date))))::int AS months_elapsed
```

### 5.4 Penghapusan (disposal)

```
Ajukan penghapusan (assets.disposal.create)
  → status pengajuan 'pending', aset TIDAK berubah dulu
  → Setujui (assets.disposal.approve)
     → aset: status='dihapus', disposed_at=NOW(), quantity dikurangi qty yang dihapus
     → snapshot book_value disimpan di dokumen
     → asset_movements: type='penghapusan', amount = book_value, ref = DSP…
  → Tolak → status 'rejected' + alasan, aset tetap apa adanya
```

Aset mode `massal` boleh dihapus sebagian (mis. 5 dari 40 kursi): `quantity` berkurang,
baris aset tetap hidup. Aset `tunggal` selalu dihapus utuh.

Soft delete lama (`is_deleted = true`) **tidak lagi dipakai untuk aset yang benar-benar
sudah tidak ada** — itu untuk salah input. Barang yang rusak/dijual/hilang harus lewat
disposal, supaya nilainya tercatat dan laporan aset tidak diam-diam menyusut tanpa sebab.

---

## 6. Pemindahan Aset Antar Outlet

### 6.1 Alur status

Sama persis dengan pola `stock_transfers` yang sudah dipakai Gudang, agar tim tidak perlu
mempelajari alur kedua:

```
draft ──ajukan──▶ pending ──setujui──▶ approved ──kirim──▶ sent ──terima──▶ received
  │                  │                     │                  │
  └──batal──▶ cancelled                    └──tolak──▶ rejected
                                                            (barang kembali ke asal)
```

| Aksi | Dari status | Ke status | Izin | Efek |
|---|---|---|---|---|
| simpan draft | — | `draft` | `assets.transfer.create` | Belum ada efek ke aset |
| ajukan | `draft` | `pending` | `assets.transfer.create` | — |
| setujui | `pending` | `approved` | `assets.transfer.approve` | — |
| tolak | `pending` | `rejected` | `assets.transfer.approve` | — |
| kirim | `approved` | `sent` | `assets.transfer.create` | Aset asal → `status='transit'`, qty dikurangi (massal), `asset_movements: mutasi_keluar` |
| terima | `sent` | `received` | `assets.transfer.receive` | Aset tiba di outlet tujuan, `asset_movements: mutasi_masuk` |
| batal | `draft`, `pending` | `cancelled` | `assets.transfer.create` | — |

**Persetujuan bisa dilewati** bila manajemen memilih begitu: setting
`asset_transfer_require_approval` di `app_settings`; bila `false`, aksi “ajukan” langsung
menghasilkan `approved`. Default: `true` untuk mutasi antar outlet, karena inilah satu-satunya
titik di mana aset berpindah pemilik anggaran.

### 6.2 Aturan siapa boleh apa

- **Outlet asal** membuat, mengirim, dan membatalkan.
- **Outlet tujuan** menerima. Pengguna yang scope-nya hanya outlet tujuan **tidak boleh**
  mengirim, dan sebaliknya.
- Pengguna ber-scope `specific` hanya melihat mutasi yang `from_outlet_id` **atau**
  `to_outlet_id`-nya ada dalam scope-nya — kalau tidak, mutasi yang menuju outletnya
  tidak akan pernah terlihat untuk diterima.
- `validateOutletAccess` dipakai untuk `from_outlet_id` saat membuat dokumen, dan untuk
  `to_outlet_id` saat menerima.

### 6.3 Yang boleh dimutasi

| Kondisi aset | Boleh dimutasi? |
|---|---|
| `status = 'aktif'` atau `'tidak_aktif'` | Ya |
| `status = 'transit'` | Tidak — sudah ada di mutasi lain yang berjalan |
| `status = 'perbaikan'` | Tidak, kecuali `reason = 'perbaikan'` (dikirim ke bengkel/outlet lain untuk diperbaiki) |
| `status = 'dihapus'` | Tidak |
| `condition = 'rusak_berat'` | Boleh, tapi UI memberi peringatan — sering ini disposal yang disamarkan jadi mutasi |

### 6.4 Pemecahan baris untuk mode massal

Ini bagian paling rawan, jadi aturannya dikunci:

**Saat `kirim` (sent):**
- `qty` yang dikirim < `quantity` aset asal → `quantity` asal dikurangi `qty`.
  Baris asal tetap hidup di outlet asal.
- `qty` = seluruh `quantity` → seluruh baris masuk transit, `status='transit'`.

**Saat `terima` (received):**
1. Cari baris aset di outlet tujuan yang **sepadan**: sama `name`, `category`, `unit`,
   `tracking_mode='massal'`, `condition` sama, dan belum dihapus.
   - Ketemu → `quantity` baris itu ditambah `received_qty` (penggabungan).
   - Tidak ketemu → buat baris baru di outlet tujuan (`acquisition_src='mutasi'`,
     `purchase_date`/`purchase_price`/`useful_life_months` **diwarisi dari asal** supaya
     penyusutan tidak ter-reset menjadi barang baru).
2. `target_asset_id` pada baris mutasi diisi id baris tujuan — inilah yang menyambung
   riwayat lama dengan baris baru.
3. Aset mode `tunggal` **tidak pernah digabung**: baris yang sama hanya berpindah
   `outlet_id`, `asset_no` dan riwayat perawatannya ikut, `status` kembali ke `aktif`.

**Penerimaan sebagian** (`received_qty < qty`): selisihnya otomatis dikembalikan ke baris
asal dan dicatat sebagai `asset_movements` bertipe `mutasi_masuk` di outlet asal dengan
catatan “selisih penerimaan”. Dokumen tetap `received`, tapi ditandai **selisih** di daftar
sehingga bisa ditindaklanjuti.

Seluruh langkah `kirim` dan `terima` berjalan dalam **satu transaksi database**
(`tx.Begin` … `tx.Commit`, pola `applyMovement` di [services/warehouse.go](../services/warehouse.go)):
aset tidak boleh pernah berada di dua outlet sekaligus, atau lenyap dari keduanya.

### 6.5 Dokumen serah terima

Halaman detail mutasi bisa mencetak **Berita Acara Serah Terima Aset**: nomor MTA, outlet
asal & tujuan, daftar aset (nomor, nama, seri, kondisi kirim/terima), alasan, dan dua kolom
tanda tangan. Ini dokumen yang selama ini dibuat manual di luar sistem.

---

## 7. Perawatan Barang (Maintenance)

### 7.1 Dua jenis pekerjaan

| Jenis | `type` | Dipicu oleh | Ciri |
|---|---|---|---|
| **Preventif** | `rutin`, `inspeksi` | Jadwal (`next_due_date`) | Direncanakan, biaya kecil, mencegah kerusakan |
| **Korektif** | `perbaikan`, `penggantian` | Kerusakan | Tidak terjadwal, biaya besar, ada downtime |

Keempat nilai `type` di atas sudah dipakai UI hari ini
([ui/src/pages/Assets.vue:256](../ui/src/pages/Assets.vue#L256)) dan tidak diubah.

### 7.2 Siklus work order

```
dijadwalkan ──mulai──▶ berjalan ──selesai──▶ selesai
     │                    │
     └──────batal─────────┴──▶ batal
```

- Masuk `berjalan` → aset `status='perbaikan'` (dan aset tidak bisa dimutasi, 6.3).
- Masuk `selesai` → `condition_after` diterapkan ke aset (perilaku ini **sudah ada**
  di [services/asset.go](../services/asset.go) `AddAssetMaintenance`), `status` kembali
  `aktif`, `next_due_date` dipakai membuat jadwal berikutnya, dan
  `asset_movements` bertipe `perawatan` ditulis dengan `amount = cost`.
- Pencatatan mundur (pekerjaan yang sudah terjadi) tetap didukung: form lama langsung
  menghasilkan work order berstatus `selesai`, persis seperti sekarang.

### 7.3 Jadwal & pengingat

`next_due_date` yang selama ini tersimpan tanpa pernah dibaca sekarang menjadi sumber:

- **Daftar Jatuh Tempo** — halaman berisi work order `dijadwalkan` dengan
  `scheduled_date ≤ hari ini + 30 hari`, dikelompokkan: terlambat / minggu ini / bulan ini.
- **Penjadwal harian** — job di `services` (pola `ppic_scheduler.go` / `social_scheduler.go`)
  berjalan sekali sehari pada zona waktu aplikasi, membuat work order `dijadwalkan` untuk
  perawatan yang `next_due_date`-nya jatuh dalam 7 hari ke depan, dan menandai yang lewat
  tempo sebagai **terlambat** di dashboard.
- **Template interval per kategori** (`app_settings`): AC 3 bulan, mesin kopi 1 bulan,
  kulkas/chiller 6 bulan, genset 6 bulan, kendaraan 6 bulan. Dipakai mengisi `next_due_date`
  otomatis saat work order diselesaikan, tetap bisa ditimpa manual.

### 7.4 Perawatan yang dikerjakan vendor → Pengadaan Jasa

Bila perbaikan memerlukan pembayaran vendor, work order bisa **membuat pengajuan Pengadaan
Jasa** (`request_type = 'jasa'`) langsung dari halaman perawatan:

```
Work order (butuh vendor) ──▶ draft PR jasa (nama pengadaan = "Perbaikan {nama aset}")
                              ──▶ alur pengadaan normal: approve → bayar
                              ──▶ saat PR 'received', cost work order terisi dari total_final
```

`asset_maintenances.purchase_request_id` menyimpan tautannya, sehingga biaya perawatan
di laporan aset dan biaya pengadaan jasa di laporan keuangan **berasal dari satu angka**,
bukan dua entri yang diketik terpisah (masalah yang sama dengan yang sudah diperbaiki
pada commit “Sinkronkan angka pengadaan antara keuangan dan laporan”).

### 7.5 KPI perawatan

| KPI | Rumus | Target usulan |
|---|---|---|
| Kepatuhan jadwal | WO preventif selesai tepat waktu ÷ WO preventif jatuh tempo | ≥ 90% |
| Rasio preventif : korektif | jumlah WO per jenis | ≥ 70 : 30 |
| Biaya perawatan per aset | Σ `cost` per aset per tahun | — |
| **Rasio biaya terhadap nilai perolehan** | Σ biaya perawatan seumur hidup ÷ `purchase_price` | > 50% → **usulkan ganti**, bukan perbaiki lagi |
| Downtime | Σ `downtime_hours` per aset per bulan | — |

Rasio 50% itulah yang mengubah perawatan dari catatan biaya menjadi alat keputusan:
saat kulkas yang dibeli Rp 8 juta sudah menghabiskan Rp 4 juta perbaikan, sistem yang
mengusulkan penggantian, bukan ingatan orang.

---

## 8. Penerimaan Barang dari Pengadaan

### 8.1 Alur sekarang dan di mana putusnya

```
pending ─approve─▶ approved ─ajukan bayar─▶ payment_requested ─bayar─▶ paid/partial
                                                                          │
                                                                 “Serah Terima”
                                                                          ▼
                                                                      received   ← BERHENTI DI SINI
```

Aksi `receive` ([services/purchase.go:869](../services/purchase.go#L869)) hanya menjalankan:

```sql
UPDATE purchase_requests SET status='received', received_by=$2, received_at=$3 WHERE id=$4
```

Tidak ada baris aset yang lahir, tidak ada stok yang bertambah. Modul GRN
(`goods_receipts`) memang sudah bisa menautkan `purchase_request_id`, tapi dibuat dari
halaman Gudang secara terpisah dan hanya untuk `stock_items` — barang modal tidak punya
tempat di sana.

### 8.1b Dua meja serah terima

Pengadaan barang tidak diterima di satu tempat. Prosesnya terbagi dua, dan tiap bagian
punya meja serah terimanya sendiri:

| Jenis belanja | Diterima di | Yang mencatat | Wujud datanya |
|---|---|---|---|
| **Barang dapur untuk outlet** | **Gudang Induk** | Tim gudang (`stockledger.adjust`) | Baris GRN → `stock_ledger` |
| **Belanja peralatan** | **Bagian Aset** | Tim aset (`assets.create`) | Baris `assets` bernomor |
| **Material projek** (semen, cat, keramik) | **Bagian Aset** | Tim aset (`assets.create`) | Baris `project_materials`; sisanya menjadi aset (§8.9) |

**Satu pengajuan tidak boleh mencampur keduanya.** Aturan ini ditegakkan **sejak pengajuan
dibuat**, bukan saat diterima: `ClassifyPurchaseItems` memeriksa tiap sub-item terhadap
katalog stok dan menolak dokumen campuran dengan pesan yang menyebut barang mana yang
bentrok. Dokumen campuran memaksa satu berkas diantarkan ke dua meja — dan di situlah barang
tercecer. Jenis yang lolos disimpan di kolom `purchase_requests.goods_kind`
(`dapur` | `perlengkapan`), diwarisi oleh pecahan per vendor, dan dipakai menyaring antrean.
Pengadaan **jasa** tidak terkena aturan ini.

Konsekuensinya pada rancangan:

1. **Dua antrean terpisah, satu halaman.** `GET /admin/receiving-queue?kind=perlengkapan|dapur`
   memberi tiap meja daftar pekerjaannya sendiri. Halaman **Penerimaan Peralatan** ada di grup
   Perlengkapan, **Penerimaan dari Pengadaan** di grup Gudang; keduanya memakai komponen yang
   sama, dibedakan oleh `receivingKind` pada meta rute.
2. **Dialog difilter per meja.** Tim aset tidak melihat baris bahan dapur, dan sebaliknya.
   Dari halaman Pengadaan (petugas pengadaan) dialog tetap menampilkan keduanya, dikelompokkan
   dengan judul yang menyebut siapa mengerjakan apa.
3. **Peringatan dini di form pengajuan.** Halaman Pengadaan Barang memuat nama katalog stok
   dan menandai campuran sambil diketik, lengkap dengan penjelasan meja masing-masing —
   penolakan server menjadi jaring pengaman, bukan kejutan.
4. **Status pengajuan hanya maju ke `received` setelah SELURUH barisnya diterima** — bukan
   setelah salah satu meja selesai. Tanpa aturan ini, tim aset yang mencatat kompor akan
   menutup dokumen sementara daging di gudang belum diterima siapa pun, dan bagian keuangan
   membaca "barang sudah datang" untuk belanja yang separuhnya masih di jalan.

Satu dokumen tetap satu dokumen: kedua meja bekerja pada pengajuan yang sama, dan sisa
pekerjaan meja lain terlihat di pesan hasil (“1 baris belum diterima — dokumen tetap terbuka”).

### 8.1c Penerimaan dan pembayaran adalah dua dimensi

Ada vendor yang mengirim dulu baru menagih (tempo), ada yang minta dibayar di muka.
Kolom `status` lama mencampur keduanya — `received` diperlakukan sebagai kelanjutan `paid` —
sehingga **barang yang datang sebelum dibayar tidak punya tempat dicatat**, dan petugas
terpaksa menunda serah terima sampai keuangan membayar. Itu justru menghilangkan jejak
barang yang fisiknya sudah ada di outlet.

Kolom `purchase_requests.receipt_status` (`''` | `partial` | `received`) memisahkan keduanya:

| Urutan | Yang terjadi | Status dokumen |
|---|---|---|
| **Barang dulu (tempo)** | Disetujui → barang diterima → `receipt_status='received'`, `status` **tetap** `approved`/`payment_requested` | Masih tagihan; begitu dibayar lunas, `status` **otomatis** menjadi `received` tanpa perlu menekan Serah Terima lagi |
| **Bayar dulu** | Disetujui → dibayar (`paid`) → barang diterima | `status` menjadi `received` seperti biasa |
| **Sebagian** | Diterima sebagian kapan pun | `receipt_status='partial'`, dokumen tetap terbuka di antrean |

Penerimaan kini sah sejak dokumen **disetujui** — yang menghalangi hanyalah pengajuan yang
belum disetujui atau sudah mati, bukan keadaan pembayarannya.

**Di bagian Pembayaran**, tiap baris membawa penanda *“Barang sudah diterima / diterima
sebagian / belum diterima”* berdampingan dengan status pembayarannya. Membayar barang yang
belum datang adalah risiko yang berbeda dengan melunasi barang yang sudah ada di outlet,
dan sebelumnya perbedaan itu tidak terlihat sama sekali dari layar pembayaran. Sebaliknya,
**antrean penerimaan** menampilkan keadaan pembayaran (“Belum dibayar (tempo)” / “Dibayar
sebagian” / “Sudah dibayar”), supaya petugas yang menerima tahu ia sedang menerima barang
yang belum lunas.

### 8.1d Distribusi lanjutan dan bukti foto

Barang tidak berhenti di meja penerimaan. Masing-masing meja meneruskannya:

| Meja | Meneruskan ke | Dokumennya |
|---|---|---|
| **Gudang Induk** | Gudang outlet | Transfer Stok yang sudah ada (`TRF…`) |
| **Bagian Aset** | **PIC yang mengajukan pembelian** | Serah Terima Aset (`SRT…`), tabel `asset_handovers` |

**Dua momen wajib berfoto**, keduanya ditolak bila fotonya kosong:

1. **Saat barang diterima dari tim purchasing** — foto diminta di dialog Serah Terima.
2. **Saat barang didistribusikan** — foto diminta saat menyerahkan aset ke PIC, dan saat
   gudang induk menekan "Kirim" pada transfer stok.

Tanpa foto, “barang sudah saya serahkan” dan “saya belum menerima apa pun” tidak bisa
dibedakan ketika keduanya saling mengklaim. Pengambilannya memakai
`capture="environment"` sehingga kamera belakang ponsel langsung terbuka — dipakai sambil
memegang barangnya, bukan mengunggah berkas dari galeri.

Serah terima aset ke PIC juga **memindahkan penanggung jawabnya**: `assets.pic_name` dan
lokasi ikut diperbarui, dan satu baris buku besar aset ditulis (“Diserahkan ke … (sebelumnya: …)”).
Halaman **Distribusi ke PIC** menampilkan daftar aset yang sudah diterima tapi belum
diserahkan — itulah daftar pekerjaan distribusi bagian aset.

### 8.1e Cadangan bukti foto ke Google Drive

Foto serah terima harus bertahan lebih lama dari server ini: folder `uploads` bisa hilang
karena disk penuh, kontainer dibuat ulang, atau salah hapus. Setiap foto disalin ke
**Google Drive** memakai **remote rclone `gdrive:` yang sudah ada** — remote yang sama
dengan cron cadangan database harian ([scripts/backup-db.sh](../scripts/backup-db.sh)).

- Tidak ada kredensial baru: akses memakai `rclone.conf` milik server, dipasang **read-only**
  ke container lewat `docker-compose.yml`, lalu **disalin sekali** ke lokasi yang bisa ditulis.
  Salinan itu perlu karena rclone menyegarkan token OAuth dan menulisnya kembali; dengan
  berkas read-only, tiap pemanggilan memuntahkan `Failed to save config … device or resource
  busy` ke stderr — dan pesan itu sempat ikut tersimpan sebagai "tautan" sebelum diperbaiki.
- Penjadwal berjalan **tiap 15 menit**, maksimal 20 foto per putaran. Kegagalan ditandai
  `failed` beserta pesan aslinya dan bisa diulang dari halaman Pengaturan.
- Folder tujuan: `gdrive:cloud-pos-photos/<YYYY-MM>/`, mengikuti susunan folder di server.
  Bisa diubah lewat setelan `photo_drive_remote`.
- `rclone link` sekaligus memasang izin **"siapa saja yang punya link"** pada berkasnya.

**Menampilkan foto langsung dari Drive.** Foto yang sudah tersalin dipanggil dari Drive
memakai endpoint `https://drive.google.com/thumbnail?id=<id>&sz=w1200`, dengan berkas lokal
sebagai cadangan (`@error` pada tag `<img>`). Bentuk `uc?export=view` sengaja tidak dipakai
sebagai pilihan utama: Google kerap mengalihkannya dan gagal dimuat pada tag `<img>`,
terutama saat banyak foto dimuat sekaligus. Diuji sungguhan ke Drive: berkas terunggah,
tautan publiknya membalas **HTTP 200 `image/png` tanpa login**.

> **Keputusan pemilik sistem, 16 Sep 2026: foto dijadikan publik.** Konsekuensinya —
> siapa pun yang memegang tautannya bisa membuka fotonya, termasuk di luar perusahaan,
> tanpa login. Ini diterima sebagai ganti kemudahan menampilkan bukti langsung di layar.
> Bila suatu saat berubah, jalur amannya adalah endpoint proxy di backend (server yang
> menarik berkas dari Drive dan mengalirkannya ke browser), bukan membuka izin Drive.

### 8.2 Alur baru: satu dialog di titik Serah Terima

Aksi `receive` tetap ada dan tetap mengubah status. Yang ditambahkan adalah **dialog
penerimaan** sebelum status berubah, yang membagi setiap baris belanja ke tujuannya:

```
         ┌────────────────────────────────────────────────────┐
         │  Dialog "Serah Terima Pengadaan {no PR}"           │
         │  Setiap sub-item dipilih tujuannya:                │
         ├────────────────────────────────────────────────────┤
Sub-item ┤ ⦿ Aset             → baris assets                  │──▶ asset_movements: penerimaan
         │ ○ Material projek   → project_materials            │──▶ buku material lokasi projek
         │                       (hanya bila PR punya projek) │
         │ ○ Stok gudang       → GoodsReceipt (GRN)           │──▶ stock_ledger + batch
         │ ○ Habis pakai       → tidak dicatat sebagai        │
         │                       barang, hanya biaya          │
         └────────────────────────────────────────────────────┘
                              │
                              ▼  satu transaksi
             purchase_requests.status = 'received'
```

Dialog ini dipakai dari tiga tempat: antrean bagian Aset (hanya baris perlengkapan), antrean
Gudang Induk (hanya baris dapur), dan halaman Pengadaan (keduanya, dikelompokkan).

Usulan tujuan diisi otomatis oleh urutan aturan di Bagian 2, petugas tinggal mengoreksi.
Pilihan **Material projek** hanya muncul bila pengajuan terikat `project_id`, dan menjadi
usulan default untuk seluruh baris non-aset pada pengajuan projek. Dialog menolak disimpan
bila ada baris yang belum dipilih tujuannya — diam bukan jawaban yang aman untuk barang
senilai belasan juta.

Pilihan **Habis pakai** sengaja dibiarkan ada, tapi dibatasi: untuk baris bernilai
**≥ Rp 1.000.000** (ambang di `app_settings`) pilihan itu memerlukan alasan tertulis yang
ikut tersimpan di `notes` PR. Tanpa rem ini, seluruh rancangan bisa dilewati hanya dengan
menandai semuanya “habis pakai”.

### 8.3 Pemetaan field: baris PR → aset / material projek

Struktur `purchase_requests.items` adalah JSONB: daftar **entri pengadaan**
(`PurchaseRequestItem`: `name`, `hps_total`, `final_total`) yang masing-masing berisi
**sub-item** (`PurchaseSubItem`: `name`, `qty`, `unit`, `final_price`, `final_subtotal`)
— lihat [models/purchase.go](../models/purchase.go).

| Field aset | Diisi dari | Catatan |
|---|---|---|
| `name` | `PurchaseSubItem.name` | Bisa disunting di dialog |
| `quantity` | `PurchaseSubItem.qty` | Boleh dikurangi bila datang sebagian (8.5) |
| `unit` | `PurchaseSubItem.unit` | |
| `purchase_price` | `final_price` (per satuan) | Bila `final_price` 0 → pakai `hps_price`; bila keduanya 0, dialog menandai “harga belum diisi” |
| `purchase_date` | `purchase_requests.received_at` (tanggal, zona aplikasi) | Bukan `created_at` PR — tanggal barang tiba yang menjadi awal penyusutan |
| `outlet_id` | `purchase_requests.outlet_id` | Bila PR dibuat untuk unit kerja tanpa outlet, petugas memilih outlet tujuan di dialog |
| `work_unit_id` | `purchase_requests.work_unit_id` | |
| `category` | Dipilih di dialog | Kategori menentukan default umur ekonomis (5.3) |
| `useful_life_months` | Default kategori | Bisa ditimpa |
| `condition` | `baik` | |
| `status` | `aktif` | |
| `acquisition_src` | `'pengadaan'` | Terkunci |
| `purchase_request_id` | id PR (anak, bukan master — lihat 8.4) | Terkunci |
| `pr_item_key` | `normalize(entri.name) + '|' + normalize(subitem.name) + '#' + urutan kemunculan` | Kunci idempotensi |
| `asset_no` | Dibuat sistem | `AST-{kode outlet}-{YYMM}-{seq}` |
| `tracking_mode` | `tunggal` bila qty=1 atau harga satuan ≥ Rp 5 juta; selain itu `massal` | Pilihan `tunggal` dengan qty>1 menghasilkan N baris |

Nilai `vendor_name` dan `invoice_number` PR disalin ke `notes` aset
(“Vendor: … · Invoice: …”) agar jejak pembelian ikut terbawa tanpa menambah kolom baru.

Untuk baris bertujuan **Material projek**, pemetaannya jauh lebih pendek — material tidak
punya identitas per unit, tidak disusutkan, dan tidak bernomor:

| Field `project_materials` | Diisi dari | Catatan |
|---|---|---|
| `project_id` | `purchase_requests.project_id` | Terkunci. Tujuan “Material projek” memang hanya muncul bila kolom ini terisi |
| `purchase_request_id` | id PR (anak, bukan master) | Terkunci |
| `pr_item_key` | Rumus yang sama dengan aset | Kunci idempotensi **dan** kunci penggabungan penerimaan bertahap (8.5) |
| `name`, `unit` | `PurchaseSubItem.name`, `.unit` | Nama bisa disunting di dialog |
| `qty_received` | `PurchaseSubItem.qty` | Boleh dikurangi bila datang sebagian; penerimaan berikutnya **menambah** kolom ini, bukan membuat baris baru |
| `unit_cost` | `final_price` (per satuan) | Jatuh ke `hps_price` bila final belum diisi, sama seperti aset |
| `received_at` / `received_by` | Waktu aksi + nama petugas | Zona waktu aplikasi |
| `location` | Diisi di dialog (opsional) | Titik simpan di lokasi projek |
| `qty_used`, `qty_returned`, `qty_wasted` | 0 | Hanya berubah lewat log (8.8, 8.9) |

Satu baris `project_material_logs` bertipe `penerimaan` ditulis setiap kali `qty_received`
bertambah — termasuk pada penerimaan pertama. Tanpa itu, kiriman semen tahap kedua hanya
tampak sebagai angka yang membesar tanpa jejak siapa dan kapan.

### 8.4 Idempotensi — pertahanan terhadap pencatatan ganda

Ini risiko terbesar alur ini: satu klik ganda, satu PR yang dibuka dua petugas, atau satu
dokumen yang statusnya sempat dikembalikan → aset atau material kembar, dan nilai aset perusahaan
menggelembung tanpa ada barangnya.

Tiga lapis pertahanan:

1. **Indeks unik parsial.**
   ```sql
   CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_pr_item
     ON assets(purchase_request_id, pr_item_key)
     WHERE purchase_request_id IS NOT NULL AND pr_item_key <> '' AND is_deleted = false;
   ```
   Penyisipan kedua gagal di level database, bukan hanya di level aplikasi.
2. **Penjaga status.** Pembuatan aset hanya boleh dari PR berstatus `paid` atau `partial`
   yang sedang berpindah ke `received` — sama dengan `validTransitions["receive"]` yang ada.
   PR yang sudah `received` menolak dialog kedua kali, kecuali lewat menu **Lengkapi
   Penerimaan** (8.5) yang memang melewati baris yang sudah pernah dibuat asetnya.
3. **Satu transaksi.** Pembuatan aset, GRN, penulisan `asset_movements`, dan
   `UPDATE purchase_requests` berada dalam satu `tx`. Gagal di tengah → tidak ada yang
   tersisa separuh jalan.

**PR yang dipecah per vendor** (`split_status = 'master'` + anak-anak): aset dibuat dari
**dokumen anak** yang memegang item, bukan dari master. Master yang itemnya sudah habis
dipecah adalah cangkang kosong (`fullySplitMasterCond` di
[services/purchase.go:41](../services/purchase.go#L41)) — mencatat aset dari master berarti
mencatat barang yang sama dua kali. Karena aksi `receive` pada master melakukan cascade ke
anak-anaknya, dialog penerimaan pada master menampilkan item **gabungan seluruh anak**,
dan setiap aset yang lahir tetap menyimpan `purchase_request_id` anaknya.

### 8.5 Penerimaan sebagian & penerimaan susulan

Status PR bersifat tunggal (`received` atau belum), jadi penerimaan sebagian tidak diwakili
status, melainkan **selisih antara qty PR dan qty yang sudah berwujud data** — di ketiga
tujuan sekaligus:

- Dialog boleh mencatat qty lebih kecil dari qty PR.
- Sisanya muncul di laporan **“Pengadaan diterima, barang belum lengkap”**:
  ```
  baris PR (qty)
    −  Σ quantity aset      dengan pr_item_key sama
    −  Σ qty_received material projek  dengan pr_item_key sama
    −  Σ qty GRN terkait
    >  0
  ```
  Material projek **wajib ikut dikurangkan**; kalau tidak, semen yang sudah diterima akan
  selamanya nongkrong di laporan ini dan laporan itu berhenti dipercaya orang.
- Menu **Lengkapi Penerimaan** pada PR `received` membuka kembali dialog hanya untuk sisa itu.

Inilah bentuk **penerimaan bertahap** yang khas projek: satu baris PR “Semen 100 sak” bisa
datang tiga kali. Penerimaan kedua dan ketiga tidak membuat baris `project_materials` baru —
indeks unik `(purchase_request_id, pr_item_key)` menolaknya — melainkan **menambah
`qty_received` pada baris yang sama** plus satu log `penerimaan`. Jadi satu baris PR tetap
berpasangan dengan satu baris material, betapa pun banyak kali barangnya dikirim.

Laporan ini adalah alat rekonsiliasi utama modul: selama ada baris di sana, ada barang yang
sudah dibayar tapi belum jelas keberadaannya.

### 8.6 Baris yang masuk gudang, bukan aset

Untuk tujuan **Stok gudang**, dialog memanggil `services.CreateGoodsReceipt` yang sudah ada
([services/goods_receipt.go](../services/goods_receipt.go)) dengan `purchase_request_id`
terisi — jadi GRN, batch FIFO, dan `stock_ledger` mengikuti jalur yang sudah teruji, dan
kolom `purchase_request_id` pada `goods_receipts` akhirnya benar-benar dipakai.

Dua syarat yang harus dipenuhi, kalau tidak baris itu ditunda:

1. Sub-item harus dipetakan ke sebuah `stock_items` (pencarian dengan saran berdasarkan
   kemiripan nama; boleh membuat item baru bila pengguna punya izin katalog).
2. Pembuatan GRN memerlukan izin `stockledger.adjust`
   ([routes/routes.go:361](../routes/routes.go#L361)). Petugas pengadaan umumnya tidak
   memilikinya. Bila izin tidak ada, baris ditandai **“menunggu penerimaan gudang”** dan
   muncul di antrean halaman Gudang — status PR tetap boleh menjadi `received`, karena
   barangnya memang sudah diterima secara fisik.

### 8.7 Projek pembangunan/renovasi: tiga nasib satu belanja

**Sisi input sudah selesai, tidak ada yang perlu dibangun ulang.** Pengadaan **Barang**
dan **Jasa** sama-sama sudah punya pemilih projek — `SearchSelect` yang terikat
`form.project_id` di [PurchaseGoods.vue:132](../ui/src/pages/PurchaseGoods.vue#L132) dan
[PurchaseServices.vue:132](../ui/src/pages/PurchaseServices.vue#L132) — lengkap dengan
deep-link `?project_id=` dari tombol belanja di halaman detail projek
([ProjectDetail.vue](../ui/src/pages/ProjectDetail.vue)), sehingga PR yang dibuat dari
sebuah projek sudah terisi projeknya sejak awal. Kolom `project_id` pada
`purchase_requests` pun sudah ada berikut indeksnya.

Yang belum ada murni di **sisi penerimaan**: setelah barang projek itu datang, tidak ada
tempat untuk mencatatnya. Modul Projek hari ini adalah payung anggaran murni — header RAB
+ rekap `Committed / Estimated / Paid / Outstanding` yang dihitung dari `purchase_requests`
([models/project.go](../models/project.go)). Tidak ada satu pun barang yang tercatat.

Belanja sebuah projek renovasi sebenarnya berakhir di tiga tempat berbeda, dan ketiganya
harus terlihat di halaman projek:

| Nasib | Contoh | Tercatat di | Nilainya |
|---|---|---|---|
| **Menjadi aset** | AC, kitchen set, meja kursi baru, CCTV | `assets` (`acquisition_src='pengadaan'`) | Masuk daftar aset, disusutkan |
| **Habis dikonsumsi projek** | Semen, cat, keramik, kabel, pipa, paku | `project_materials` | Biaya projek; sisa masih punya nilai |
| **Jasa** | Upah tukang, desain, instalasi | `purchase_requests` (`request_type='jasa'`) | Biaya projek, tidak berwujud barang |

Ketiganya sudah dibayar lewat alur pengadaan yang sama, jadi ketiganya muncul di rekap
serapan RAB yang sudah ada. Yang ditambahkan dokumen ini hanya satu: **yang berwujud barang
harus punya wujud data juga.**

Halaman detail projek mendapat dua panel baru: **“Aset yang dihasilkan projek ini”**
(dari `assets.purchase_request_id` milik PR projek tersebut) dan **“Material projek”**
(8.8) — jawaban atas pertanyaan “renovasi 300 juta itu jadi apa saja”.

### 8.8 Siklus material projek: terima → pakai → sisa

```
 Pengadaan (PR projek) ──Serah Terima──▶ project_materials
                                              │  qty_received
                    ┌─────────────────────────┼──────────────────────────┐
                    ▼                         ▼                          ▼
              pemakaian                   susut/rusak                  sisa
            (qty_used, log)             (qty_wasted, wajib            (dihitung)
                    │                     beralasan)                     │
                    │                                                    │
                    ▼                                                    ▼
          habis → tidak ada sisa                            penutupan projek (8.9)
```

**Siapa berwenang di titik mana.** Penerimaan material terjadi *di dalam* dialog Serah
Terima pengadaan, jadi ia diotorisasi oleh izin aksi itu — `procurement.requests.submit` —
bukan oleh `procurement.projects.manage`. Petugas pengadaan yang menerima barang tidak perlu
diberi hak kelola projek hanya untuk mencatat kedatangan. Sebaliknya, **pemakaian,
pengembalian, susut, dan penutupan** (8.8–8.9) menuntut `procurement.projects.manage`,
karena di situlah barang berpindah tangan dan nilainya menyusut. Keputusan B (15.1) berlaku
untuk siklus setelah barang diterima, bukan untuk penerimaannya.

**Pencatatan pemakaian** sengaja dibuat semurah mungkin, karena ini dikerjakan mandor di
lokasi lewat ponsel: satu layar berisi daftar material projek, isi angka terpakai, simpan.
Satu baris `project_material_logs` bertipe `pemakaian` per entri, `qty_used` bertambah.

Dua aturan pengaman:

- `qty_used + qty_returned + qty_wasted` **tidak boleh melampaui** `qty_received`. Pemakaian
  melebihi barang yang datang berarti ada penerimaan yang belum dicatat — sistem menolak dan
  mengarahkan ke dialog penerimaan, bukan diam-diam membuat saldo negatif.
- Pemakaian mundur (tanggal lampau) boleh, tapi tercatat siapa dan kapan menginputnya
  (`actor`, `created_at`) — yang menjadi selisih antara tanggal kejadian dan tanggal catat.

**Tanpa pencatatan pemakaian pun modul ini tetap berguna**: bila manajemen memilih tidak
membebani mandor dengan input harian, `qty_used` dibiarkan 0 dan seluruh material dianggap
tersisa sampai penutupan projek — di mana sisanya dihitung sekali lewat opname lokasi (8.9).
Yang tidak boleh adalah tidak mencatat **penerimaannya**, karena di situlah uang keluar.

### 8.9 Penutupan projek & serah terima sisa material

Projek berstatus `selesai` tidak boleh menyisakan barang tanpa tuan. Saat status projek
diubah menjadi `selesai`, sistem menjalankan **penutupan material**:

> **Aturan pokok: sisa projek harus didata dan menjadi ASET.** Barang yang masih ada
> wujudnya tidak boleh lenyap dari catatan hanya karena projeknya selesai. Tiga pilihan
> lain adalah pengecualian, bukan jalan setara.

```
Projek → Selesai
   │
   ├─ Ada material dengan sisa > 0?
   │     ya → tentukan nasib tiap sisa:
   │           ⦿ Jadi aset  (BAKU)   → baris assets di outlet projek, kategori
   │                                   "Sisa Material Projek", nilai = harga materialnya.
   │                                   Tersedia aksi massal "Catat semua sisa sebagai aset".
   │           ○ Kembali ke gudang   → pengecualian, hanya bila barangnya ada di katalog
   │                                   stok; lewat GRN, ref = nomor GRN
   │           ○ Susut / terbuang    → pengecualian, hanya yang benar-benar rusak/habis;
   │                                   wajib beralasan
   │           ○ Pindah projek       → pengecualian, dipakai projek lain yang berjalan
   │     tidak → lanjut
   │
   └─ Projek 'selesai' + ringkasan realisasi material dibekukan
```

Penutupan **tidak diblokir keras** bila masih ada sisa yang belum ditentukan: status projek
tetap bisa `selesai`, tapi projek itu muncul di laporan **“Material projek belum tuntas”**
sampai beres. Blokir keras hanya akan membuat orang berhenti mencatat material sama sekali —
yang justru mengembalikan keadaan ke hari ini.

**Realisasi vs RAB per material.** Karena `project_materials` menyimpan `qty_received` dan
`unit_cost`, halaman projek bisa membandingkan rencana (sub-item pada PR) dengan realisasi
per jenis material, bukan hanya total rupiah seperti sekarang. Selisih material
(`qty_wasted` + sisa tak berwujud) adalah angka yang selama ini tidak pernah bisa dihitung.

---

## 9. Hak Akses & Batasan Unit Kerja

### 9.1 Kunci izin

Ditambahkan ke `services.AllPermissions` ([services/auth.go:27](../services/auth.go#L27)),
yang saat ini hanya memuat empat kunci `assets.*`.

| Kunci | Untuk | Baru? |
|---|---|---|
| `assets.view` / `.create` / `.update` / `.delete` | Daftar & data induk aset | ada |
| `assets.transfer.view` | Melihat mutasi antar outlet | baru |
| `assets.transfer.create` | Membuat, mengajukan, mengirim, membatalkan mutasi | baru |
| `assets.transfer.approve` | Menyetujui / menolak mutasi | baru |
| `assets.transfer.receive` | Menerima mutasi di outlet tujuan | baru |
| `assets.maintenance.view` | Melihat work order & jadwal | baru |
| `assets.maintenance.create` | Membuat & menyelesaikan work order | baru |
| `assets.disposal.view` | Melihat daftar penghapusan | baru |
| `assets.disposal.create` | Mengajukan penghapusan | baru |
| `assets.disposal.approve` | Menyetujui penghapusan | baru |
| `assets.opname.view` / `.create` / `.approve` | Opname aset | baru |
| `assets.report.view` | Laporan aset & penyusutan | baru |

**Material projek memakai izin Projek yang sudah ada, bukan kunci baru.** Melihat material
= `procurement.projects.view`; mencatat pemakaian, pengembalian, susut, dan penutupan =
`procurement.projects.manage` ([routes/routes.go:245](../routes/routes.go#L245)).
**Penerimaannya sendiri** memakai `procurement.requests.submit`, karena terjadi di dalam
dialog Serah Terima pengadaan (8.8). Menambah
kunci berjenjang empat tingkat (`procurement.projects.materials.*`) hanya memperbesar risiko
migrasi izin tanpa memisahkan peran yang benar-benar berbeda — mandor dan PIC projek adalah
orang yang sama dalam praktiknya. Pengecualian: **mengembalikan sisa material ke gudang**
tetap menuntut `stockledger.adjust`, karena itu menambah stok sungguhan (8.9).

**Konsekuensi aturan prefix yang harus disadari:** `RequirePermission("X.view")` lolos bila
role punya kunci apa pun berawalan `X.` Karena semua kunci di atas berawalan `assets.`,
role yang hanya diberi `assets.transfer.receive` **otomatis bisa membuka daftar aset**.
Ini disengaja — orang tidak bisa menerima barang yang tidak boleh ia lihat — tapi jangan
sampai dikira kebocoran saat audit izin.

Pemberian izin untuk perawatan sengaja **tidak** memakai `assets.update` seperti sekarang
([routes/routes.go:282](../routes/routes.go#L282)), supaya teknisi bisa mencatat pekerjaan
tanpa ikut berhak mengubah harga perolehan dan outlet aset.

### 9.2 Migrasi izin — jebakan yang sudah pernah menggigit

Blok migrasi yang memecah izin lama menjadi granular **wajib ditaruh paling akhir**
di `RunMigrations` ([database/migrations.go](../database/migrations.go)), tepat sebelum
`return nil`. Migrasi izin di bagian tengah me-*re-seed* kunci kasar (`warehouse.view`,
`procurement.view`, dan seluruh kunci untuk superadmin) **pada setiap boot**; blok yang
ditaruh sebelum itu akan ditimpa balik tiap aplikasi menyala.

Kunci baru di sini **menambah**, tidak memecah `assets.view` yang lama, sehingga izin yang
sudah dimiliki role tidak berubah. Pemberian kunci baru ke role admin/manajer dilakukan
**sekali jalan** dengan penanda di `app_settings` (pola `mig_split_titipan_shiftrecon`),
supaya izin yang sengaja dicabut admin tidak dikembalikan setiap boot.

### 9.3 Batasan unit kerja (scope)

- Seluruh endpoint baru wajib memanggil `getOutletScope(c)` dan mengirimnya ke service,
  persis pola `assetScopeCond` yang sudah ada di [services/asset.go](../services/asset.go).
- Scope `specific` yang tidak menghasilkan outlet apa pun **harus** mengembalikan sentinel
  `__none__` (bukan `nil`) — `nil` berarti “semua outlet”, dan salah di sini membuka
  seluruh data perusahaan kepada role yang seharusnya tidak melihat apa-apa.
- Mutasi difilter dengan `from_outlet_id = ANY($n) OR to_outlet_id = ANY($n)` (lihat 6.2).
- Perubahan scope baru berlaku pada **login berikutnya** (ikut ter-bake di JWT).

---

### 9.4 Perubahan konkret di Role & Permission

Empat berkas harus disentuh bersamaan. Melewatkan salah satunya menghasilkan gejala yang
membingungkan: izin ada di database tapi tidak muncul di layar, atau muncul tapi tidak
bisa dicentang.

**a. [services/auth.go](../services/auth.go) — sumber kebenaran.** Tambahkan ke
`AllPermissions`, tepat setelah blok `assets.*` yang ada
([services/auth.go:27](../services/auth.go#L27)):

```go
"assets.view", "assets.create", "assets.update", "assets.delete",          // sudah ada
"assets.transfer.view", "assets.transfer.create",
"assets.transfer.approve", "assets.transfer.receive",
"assets.maintenance.view", "assets.maintenance.create",
"assets.disposal.view", "assets.disposal.create", "assets.disposal.approve",
"assets.opname.view", "assets.opname.create", "assets.opname.approve",
"assets.report.view",
```

Kunci `.view` untuk setiap sub-modul **wajib ada**, bukan hiasan — alasannya di poin (d).

**b. [database/migrations.go](../database/migrations.go) — pemberian awal, sekali jalan.**
Blok ini ditaruh **paling akhir** di `RunMigrations`, tepat sebelum `return nil`, dengan
penanda di `app_settings` (mis. `mig_assets_granular_v1`) mengikuti pola
`mig_split_titipan_shiftrecon`:

```
Bila penanda belum ada:
  superadmin           → seluruh kunci baru
  admin                → seluruh kunci baru
  role yang sudah punya assets.update → assets.maintenance.view + .create
                         (supaya petugas yang selama ini mencatat perawatan tidak
                          kehilangan akses saat izin perawatan dipisah dari assets.update)
  role yang sudah punya assets.view   → assets.transfer.view, assets.disposal.view,
                                        assets.opname.view, assets.report.view
  simpan penanda
```

Tanpa penanda, blok ini akan mengembalikan izin yang sengaja dicabut admin **setiap kali
aplikasi menyala** — persis jebakan yang sudah pernah terjadi di repo ini.

**c. [ui/src/components/PermissionMatrix.vue](../ui/src/components/PermissionMatrix.vue) —
editor izin.** Kolom CRUD-nya terkunci pada empat operasi (`ALL_OPS = ['view','create',
'update','delete']`), jadi `approve` dan `receive` **tidak bisa** memakai `type: 'crud'` —
harus `type: 'toggles'`, seperti yang sudah dilakukan kategori PPIC. Baris
`{ type: 'crud', module: 'assets', label: 'Perlengkapan & Perawatan' }` dipindah dari
kategori **Master Data** ke kategori baru, sejajar dengan pemindahan sidebar (11.1):

```js
{
  label: 'Perlengkapan', icon: IC.warehouse,
  items: [
    { type: 'crud', module: 'assets', label: 'Data Aset', icon: IC.warehouse },
    { type: 'toggles', label: 'Mutasi Antar Outlet', icon: IC.transfer, toggles: [
        { key: 'assets.transfer.view',    label: 'Lihat' },
        { key: 'assets.transfer.create',  label: 'Buat & Kirim' },
        { key: 'assets.transfer.approve', label: 'Setujui' },
        { key: 'assets.transfer.receive', label: 'Terima' },
    ]},
    { type: 'toggles', label: 'Perawatan', icon: IC.settings, toggles: [
        { key: 'assets.maintenance.view',   label: 'Lihat' },
        { key: 'assets.maintenance.create', label: 'Catat & Selesaikan' },
    ]},
    { type: 'toggles', label: 'Penghapusan Aset', icon: IC.waste, toggles: [
        { key: 'assets.disposal.view',    label: 'Lihat' },
        { key: 'assets.disposal.create',  label: 'Ajukan' },
        { key: 'assets.disposal.approve', label: 'Setujui' },
    ]},
    { type: 'toggles', label: 'Opname Aset', icon: IC.ledger, toggles: [
        { key: 'assets.opname.view',    label: 'Lihat' },
        { key: 'assets.opname.create',  label: 'Hitung' },
        { key: 'assets.opname.approve', label: 'Setujui Selisih' },
    ]},
    { type: 'single', key: 'assets.report.view', label: 'Laporan Aset', icon: IC.report },
  ],
},
```

Daftar di berkas ini **harus cocok** dengan `AllPermissions`: kunci yang ada di Go tapi
tidak ada di sini menjadi izin hantu — aktif di server, tak terlihat di layar.

**d. [ui/src/pages/Roles.vue](../ui/src/pages/Roles.vue) — halaman Role.** Dua hal:

1. **Daftar halaman tujuan setelah login** ([Roles.vue:369](../ui/src/pages/Roles.vue#L369)):
   entri `/perlengkapan` dipindah ke grup kategori baru dan ditambah enam halaman lain
   (Dashboard Aset, Mutasi, Perawatan, Opname, Penghapusan, Laporan Aset) dengan `perm`
   masing-masing, agar role yang hanya mengurus aset bisa mendarat langsung di halamannya.
2. **Perilaku cascade** ([Roles.vue:602](../ui/src/pages/Roles.vue#L602)) — tidak ada kode
   yang perlu diubah, tapi konsekuensinya harus dipahami:

| Aksi di layar | Yang terjadi | Akibat untuk modul ini |
|---|---|---|
| Menyalakan kunci non-`.view` | Sistem ikut menyalakan `<modul>.view`, dengan modul diambil dari **`lastIndexOf('.')`** | `assets.transfer.create` → ikut menyalakan `assets.transfer.view`. **Hanya berjalan bila kunci `.view` itu terdaftar di `AllPermissions`** — inilah sebabnya `assets.disposal.view` wajib ada meski tidak dipakai rute mana pun sebagai satu-satunya penjaga |
| Mematikan `assets.transfer.view` | Seluruh kunci berawalan `assets.transfer.` ikut mati | Sesuai harapan |
| **Mematikan `assets.view`** | Seluruh kunci berawalan `assets.` ikut mati — termasuk mutasi, perawatan, opname, penghapusan, laporan | Perilaku yang benar, tapi mengejutkan: satu klik mencabut seluruh modul. Beri konfirmasi di UI |

**e. Usulan preset role.** Bukan bagian dari kode, tapi ini yang akan ditanyakan pertama
kali saat modul menyala:

| Role | Kunci yang diberikan |
|---|---|
| **Superadmin** | Semua (bypass) |
| **Admin / Manajer Pusat** | Seluruh `assets.*` |
| **Manajer Area** | `assets.view`, `assets.update`, `assets.transfer.*` (termasuk approve), `assets.maintenance.*`, `assets.disposal.view/.approve`, `assets.opname.*`, `assets.report.view` |
| **Kepala Outlet** | `assets.view`, `assets.create`, `assets.update`, `assets.transfer.view/.create/.receive`, `assets.maintenance.view/.create`, `assets.disposal.create`, `assets.opname.view/.create` |
| **Teknisi / Maintenance** | `assets.maintenance.view`, `assets.maintenance.create` (cukup — aturan prefix memberinya akses baca daftar aset) |
| **Staf Gudang** | `assets.view`, `assets.transfer.view/.receive` |
| **PIC Projek / Mandor** | Tidak perlu kunci `assets.*` sama sekali; material projek memakai `procurement.projects.view/.manage` (15.1 keputusan B) |

Perubahan scope role tetap baru berlaku pada **login berikutnya** (9.3), termasuk untuk
seluruh kunci baru di atas.

---

## 10. API

### 10.1 Endpoint yang sudah ada (tidak berubah)

| Method | Path | Izin |
|---|---|---|
| GET | `/admin/assets` | `assets.view` |
| GET | `/admin/assets/:id` | `assets.view` |
| POST | `/admin/assets` | `assets.create` |
| PUT | `/admin/assets/:id` | `assets.update` |
| DELETE | `/admin/assets/:id` | `assets.delete` |
| GET | `/admin/assets/:id/maintenances` | `assets.view` → jadi `assets.maintenance.view` |
| POST | `/admin/assets/:id/maintenances` | `assets.update` → jadi `assets.maintenance.create` |
| DELETE | `/admin/assets/:id/maintenances/:mid` | `assets.update` → jadi `assets.maintenance.create` |

### 10.2 Endpoint baru

| Method | Path | Izin | Fungsi |
|---|---|---|---|
| GET | `/admin/assets/summary` | `assets.view` | Kartu dashboard: jumlah, nilai perolehan, nilai buku, per kondisi/status |
| GET | `/admin/assets/:id/movements` | `assets.view` | Buku besar satu aset |
| POST | `/admin/assets/:id/photo` | `assets.update` | Simpan URL foto hasil upload |
| GET | `/admin/asset-transfers` | `assets.transfer.view` | Daftar mutasi (filter status/outlet, paginasi) |
| GET | `/admin/asset-transfers/:id` | `assets.transfer.view` | Detail + item |
| POST | `/admin/asset-transfers` | `assets.transfer.create` | Buat draft |
| PUT | `/admin/asset-transfers/:id` | `assets.transfer.create` | Ubah draft |
| POST | `/admin/asset-transfers/:id/submit` | `assets.transfer.create` | draft → pending |
| POST | `/admin/asset-transfers/:id/approve` | `assets.transfer.approve` | pending → approved |
| POST | `/admin/asset-transfers/:id/reject` | `assets.transfer.approve` | pending → rejected |
| POST | `/admin/asset-transfers/:id/send` | `assets.transfer.create` | approved → sent |
| POST | `/admin/asset-transfers/:id/receive` | `assets.transfer.receive` | sent → received |
| POST | `/admin/asset-transfers/:id/cancel` | `assets.transfer.create` | → cancelled |
| GET | `/admin/asset-maintenances` | `assets.maintenance.view` | Lintas aset: filter status/jatuh tempo/outlet |
| GET | `/admin/asset-maintenances/due` | `assets.maintenance.view` | Daftar jatuh tempo (terlambat / 7 / 30 hari) |
| PUT | `/admin/asset-maintenances/:id` | `assets.maintenance.create` | Ubah status WO (mulai/selesai/batal) |
| POST | `/admin/asset-maintenances/:id/purchase-request` | `assets.maintenance.create` + `procurement.requests.submit` | Buat PR jasa dari WO |
| GET | `/admin/asset-disposals` | `assets.disposal.view` | Daftar penghapusan |
| POST | `/admin/asset-disposals` | `assets.disposal.create` | Ajukan penghapusan |
| POST | `/admin/asset-disposals/:id/approve` | `assets.disposal.approve` | Setujui / tolak |
| GET | `/admin/asset-opnames` | `assets.opname.view` | Daftar sesi |
| POST | `/admin/asset-opnames` | `assets.opname.create` | Buka sesi (membekukan daftar) |
| PUT | `/admin/asset-opnames/:id/items` | `assets.opname.create` | Simpan hasil hitung |
| POST | `/admin/asset-opnames/:id/approve` | `assets.opname.approve` | Terapkan selisih |
| GET | `/admin/purchase-requests/:id/receiving-draft` | `procurement.requests.submit` | Isi dialog Serah Terima (baris + usulan tujuan) |
| POST | `/admin/purchase-requests/:id/receive-goods` | `procurement.requests.submit` | Jalankan penerimaan: aset + GRN + status `received` |
| GET | `/admin/projects/:id/materials` | `procurement.projects.view` | Daftar material + sisa per projek |
| POST | `/admin/projects/:id/materials/:mid/usage` | `procurement.projects.manage` | Catat pemakaian (log append-only) |
| POST | `/admin/projects/:id/materials/:mid/settle` | `procurement.projects.manage` (+ `stockledger.adjust` bila ke gudang) | Tentukan nasib sisa: gudang / aset / susut / pindah projek |
| POST | `/admin/projects/:id/close-materials` | `procurement.projects.manage` | Penutupan material saat projek selesai |
| GET | `/admin/projects/:id/materials/export` | `procurement.projects.view` | Realisasi material per projek (Excel) |
| GET | `/admin/assets/reports/:type` | `assets.report.view` | `daftar`, `penyusutan`, `perawatan`, `mutasi`, `belum-lengkap` |
| GET | `/admin/assets/reports/:type/export` | `assets.report.view` | Export Excel (excelize) |

**Urutan pendaftaran rute — wajib diperhatikan.** Fiber mencocokkan rute sesuai urutan
pendaftaran, dan `/assets/:id` sudah terdaftar di
[routes/routes.go:277](../routes/routes.go#L277). Rute statis baru seperti
`/assets/summary` dan `/assets/reports/:type` **harus didaftarkan sebelum** `/assets/:id`,
kalau tidak permintaan itu akan ditangkap sebagai “ambil aset dengan id `summary`”
dan selalu menghasilkan 404. Karena itu mutasi memakai path terpisah
`/admin/asset-transfers`, bukan `/admin/assets/transfers`.

Semua respons memakai amplop `models.APIResponse{Success, Data, Error}` yang sudah baku.

---

## 11. UI

### 11.1 Sidebar

Satu halaman `/perlengkapan` di dalam **Master Data** tidak lagi cukup untuk enam layar.
Dibuat grup sidebar sendiri, disisipkan **setelah “Gudang”, sebelum “Pengaturan”**
(urutan `NAV_ITEMS_DATA` saat ini: Dashboard → Master Data → Produk → Penjualan →
Laporan → Pengadaan → PPIC → Pengguna → Gudang → Pengaturan → Log Akses), sehingga
seluruh urusan barang fisik berdampingan. Tautan lama di Master Data dihapus agar
tidak ada dua pintu ke halaman yang sama.

```
Perlengkapan
├── Dashboard Aset      /perlengkapan/dashboard   assets.view
├── Daftar Aset         /perlengkapan             assets.view
├── Mutasi Antar Outlet /perlengkapan/mutasi      assets.transfer.view
├── Perawatan           /perlengkapan/perawatan   assets.maintenance.view
├── Opname Aset         /perlengkapan/opname      assets.opname.view
├── Penghapusan         /perlengkapan/penghapusan assets.view
└── Laporan Aset        /perlengkapan/laporan     assets.report.view
```

Rute lama `/perlengkapan` **tetap** menjadi Daftar Aset, sehingga tautan yang sudah
tersimpan dan pengaturan “halaman tujuan setelah login” di
[ui/src/pages/Roles.vue:369](../ui/src/pages/Roles.vue#L369) tidak rusak. Entri baru
ditambahkan ke daftar itu dan ke `PermissionMatrix.vue` (kategori sidebar yang sama).

### 11.2 Halaman

| Halaman | Isi |
|---|---|
| **Dashboard Aset** | Kartu: total aset, nilai perolehan, nilai buku, aset rusak, WO terlambat, mutasi menunggu diterima. Grafik (ApexCharts, pola yang sudah dipakai): komposisi nilai per kategori, tren biaya perawatan 12 bulan, umur aset. |
| **Daftar Aset** | Halaman existing + kolom `asset_no`/status, filter status & kategori, aksi massal (cetak label, ajukan mutasi), tombol **Detail**. |
| **Detail Aset** | Ringkasan + nilai buku berjalan, tab: Riwayat Perawatan (existing), Riwayat Mutasi & Lokasi (`asset_movements`), Dokumen & Foto. |
| **Mutasi Antar Outlet** | Daftar per status (tab: Draft, Menunggu Persetujuan, Dikirim, Selesai) + form pilih aset (multi-pilih dari outlet asal) + halaman detail dengan tombol sesuai status dan cetak berita acara. |
| **Perawatan** | Dua tab: **Jatuh Tempo** (terlambat / 7 / 30 hari) dan **Riwayat**. Aksi: mulai, selesaikan, batalkan, buat PR jasa. |
| **Opname Aset** | Sesi berjalan dengan pemindai QR, daftar terhitung/belum, ringkasan selisih sebelum persetujuan. |
| **Penghapusan** | Daftar pengajuan + form + persetujuan, cetak berita acara penghapusan. |
| **Laporan Aset** | Lima laporan aset (Bagian 12) + export Excel. Dua laporan material projek tampil di halaman Projek, bukan di sini. |

### 11.3 Ketentuan tampilan yang berlaku di repo ini

- **Ikon = inline SVG bergaya lucide** (`stroke="currentColor"`, 12–16px). Jangan emoji —
  renderingnya berbeda antar sistem operasi.
- **Pembacaan respons API**: `apiClient` sudah membuka amplop untuk respons berpaginasi,
  tetapi **bukan** untuk respons non-paginasi — di sana `res.data` bernilai `undefined` dan
  data ada di `res` langsung. Ini kelas bug yang sudah tiga kali terulang di modul Gudang
  dan Resep; gunakan pembantu `asArray()` seperti di
  [ui/src/pages/Assets.vue:288](../ui/src/pages/Assets.vue#L288).
- Nilai uang diformat dengan pembantu format rupiah yang sudah ada, bukan `toLocaleString`
  lepas.
- Komponen dipakai ulang dari [ui/src/components/ui/](../ui/src/components/ui/)
  (`AppCard`, `AppTable`, `AppModal`, `AppButton`, `SearchSelect`, `AppPagination`, …).
  Modul ini **tidak** memperkenalkan komponen dasar baru.

### 11.4 Responsif — wajib, di semua ukuran layar

Modul ini dipakai di lapangan: mandor di lokasi projek, petugas opname di gudang, kepala
outlet yang menerima mutasi. Sebagian besar interaksinya terjadi di ponsel sambil berdiri
memegang barang. Karena itu tampilan mobile **bukan penyesuaian belakangan** — ia dirancang
lebih dulu, lalu dilebarkan ke desktop.

**Dasar teknis.** Tailwind **v4** tanpa berkas konfigurasi; breakpoint adalah bawaan v4
(`sm` 640px, `md` 768px, `lg` 1024px, `xl` 1280px, `2xl` 1536px), dan penambahan breakpoint
kustom dilakukan lewat blok `@theme` di [ui/src/style.css](../ui/src/style.css), bukan
dengan membuat `tailwind.config.js` baru. Meta viewport sudah benar di
[ui/index.html](../ui/index.html).

**Rentang yang harus dilayani: 320px hingga ≥ 1536px.** Tidak ada lebar minimum yang
“dimaafkan” — 320px (iPhone SE, Android entry level yang banyak dipakai staf) termasuk
yang diuji.

| Lebar | Perilaku yang diharapkan |
|---|---|
| **320–389px** | Satu kolom penuh. Kartu, bukan tabel. Tombol utama melebar penuh. Label disingkat, nilai tidak dipotong. Tidak ada scroll horizontal pada badan halaman |
| **390–639px** | Sama, dengan sedikit ruang napas: `dl` dua kolom (`grid-cols-[auto_1fr]`), aksi sekunder muncul |
| **640–1023px** (`sm`–`md`) | Tabel mulai tampil; filter berjajar; modal lebar sedang |
| **≥ 1024px** (`lg`+) | Tata letak penuh: filter satu baris, tabel seluruh kolom, panel detail berdampingan |

**Aturan yang tidak boleh dilanggar:**

1. **Tabel tidak pernah dipaksakan di layar sempit.** Pola baku repo ini: daftar kartu
   `<div class="sm:hidden">` + tabel `<div class="hidden sm:block">`, persis seperti
   [PurchaseServices.vue:40](../ui/src/pages/PurchaseServices.vue#L40). `AppTable` sendiri
   sudah membungkus dirinya dengan `overflow-x-auto`, tetapi itu **jaring pengaman, bukan
   desain** — tabel 9 kolom yang digeser-geser di ponsel tetap tidak terbaca.
2. **Tidak ada scroll horizontal pada badan halaman**, di lebar mana pun. Penyebab tersering
   di repo ini: teks panjang tanpa `min-w-0` pada anak flex, dan nama barang tanpa
   `break-words`. Keduanya wajib pada setiap kartu.
3. **Target sentuh ≥ 40×40px** dengan jarak antar tombol ≥ 8px. Ikon aksi 13px yang berdiri
   sendiri (pola lama di beberapa halaman) harus diberi padding agar area sentuhnya cukup.
4. **Tombol utama `w-full sm:w-auto`**; deretan aksi memakai `flex-col sm:flex-row`.
5. **Modal di ponsel tampil penuh layar** (`rounded-none sm:rounded-xl`), badan modal
   ber-scroll sendiri, tombol simpan/batal menempel di bawah — form aset dan form mutasi
   punya banyak field, dan tombol simpan tidak boleh “hilang di bawah lipatan”.
6. **Angka penting tetap terbaca**: nilai rupiah, qty, dan status tidak pernah ikut
   terpotong; badge status memakai `shrink-0`.
7. **Orientasi lanskap dan ponsel lipat** ikut diuji — lebar 640–840px sering jatuh di
   celah antara kartu dan tabel.

### 11.5 Informatif — layar harus menjawab, bukan sekadar menampilkan

Setiap layar modul ini wajib punya keempat keadaan berikut, bukan hanya keadaan “ada data”:

| Keadaan | Yang ditampilkan |
|---|---|
| **Memuat** | Kerangka (skeleton) atau `AppSpinner`, bukan layar kosong yang membuat orang menekan tombol dua kali |
| **Kosong** | Kalimat yang menjelaskan *mengapa* kosong dan *apa langkah berikutnya* (“Belum ada mutasi masuk untuk outlet ini. Mutasi akan muncul setelah outlet asal menekan Kirim.”) — bukan “Tidak ada data” |
| **Gagal** | `AppAlert` berisi pesan dari server + tombol coba lagi |
| **Terbatas izin** | Tombol yang tidak boleh dipakai **disembunyikan**, bukan dimatikan diam-diam; bila disabled, sertakan alasan lewat `title` (“Butuh izin Setujui Mutasi”) |

Selain itu, khusus modul aset:

- **Angka selalu disertai konteksnya.** “Sisa 12 sak” ditulis lengkap: `12 dari 100 sak ·
  terpakai 85 · susut 3`. Identitas `qty_received = qty_used + qty_returned + qty_wasted +
  sisa` (3.8) harus terbaca di layar, bukan hanya berlaku di database.
- **Status ditampilkan sebagai badge berwarna konsisten** di seluruh modul: `aktif` hijau,
  `transit` biru, `perbaikan` kuning, `tidak_aktif` abu, `dihapus` merah. Warna yang sama
  dipakai di daftar, detail, dan cetakan.
- **Setiap dokumen menampilkan jejaknya**: siapa membuat, menyetujui, mengirim, menerima,
  lengkap dengan waktu — pada mutasi ini bahkan menjadi isi utama halaman detail.
- **Peringatan ditampilkan di tempat kejadian**, bukan hanya di dashboard: aset
  `rusak_berat` yang hendak dimutasi (6.3), biaya perawatan yang melewati 50% nilai
  perolehan (7.5), dan material projek yang sisanya belum ditentukan (8.9).

### 11.6 Interaktif — cepat, dan tidak menghukum kesalahan

- **Umpan balik dalam 100ms.** Setiap aksi mengunci tombolnya sendiri (`saving`) dan
  memunculkan toast hasil. Tidak ada aksi yang “diam” lalu tiba-tiba daftar berubah.
- **Konfirmasi hanya untuk yang sulit dibatalkan**: kirim mutasi, setujui penghapusan,
  terapkan selisih opname, dan serah terima pengadaan. Sisanya langsung jalan — konfirmasi
  di mana-mana membuat orang menekan “Ya” tanpa membaca.
- **Filter dan pencarian di-debounce** (≈350ms, pola `debouncedLoad` yang sudah ada di
  [Assets.vue:306](../ui/src/pages/Assets.vue#L306)) dan **tersimpan di query string**,
  supaya tombol Back dan tautan yang dibagikan tetap menunjuk hasil yang sama.
- **Input lapangan seminimal mungkin.** Opname dan penerimaan material memakai pemindai QR
  dengan kamera belakang; angka memakai `inputmode="numeric"` agar papan tik ponsel langsung
  membuka angka; satuan ditampilkan menempel pada input.
- **Daftar panjang dipaginasi** (`AppPagination`) dengan ukuran halaman lebih kecil di
  ponsel; tidak ada halaman yang me-render 500 baris sekaligus.
- **Aksi massal** (cetak label, ajukan mutasi) memakai pilihan centang yang di ponsel
  memunculkan bilah aksi menempel di bawah layar berisi jumlah terpilih.
- **Draf tidak hilang.** Form mutasi dan sesi opname yang belum dikirim disimpan sebagai
  `draft` di server, bukan hanya di memori browser — sinyal di gudang sering putus.

### 11.7 Uji tampilan sebelum fase dinyatakan selesai

Tiap halaman baru diperiksa pada tujuh lebar: **320, 360, 390, 414, 768, 1024, 1440** — plus
satu kali lanskap di 740×360. Untuk setiap lebar:

1. Tidak ada scroll horizontal pada badan halaman.
2. Tidak ada teks terpotong, tumpang tindih, atau tombol yang keluar dari kartu.
3. Semua aksi utama terjangkau ibu jari tanpa mengubah posisi tangan.
4. Modal terbuka penuh dan tombol simpannya terlihat tanpa menggulir.
5. Keempat keadaan di 11.5 (memuat, kosong, gagal, terbatas izin) benar-benar muncul.

---

## 12. Laporan & Dashboard

| Laporan | Isi | Ekspor |
|---|---|---|
| **Daftar Aset (KIB)** | Seluruh aset per outlet/kategori/status: nomor, nama, seri, tanggal & harga perolehan, kondisi, lokasi, PIC | Excel |
| **Penyusutan & Nilai Buku** | Per aset: harga perolehan, umur ekonomis, akumulasi penyusutan, nilai buku per tanggal laporan; subtotal per kategori & outlet | Excel |
| **Biaya Perawatan** | Per aset/kategori/outlet per periode: jumlah WO, total biaya, downtime, rasio biaya terhadap nilai perolehan (penanda > 50%) | Excel |
| **Mutasi Aset** | Seluruh perpindahan dalam periode: dari–ke, nilai perolehan yang berpindah, status | Excel |
| **Pengadaan Belum Lengkap** | Baris PR `received` yang belum menjadi aset/GRN/material (rumus di 8.5) | Excel |
| **Realisasi Material Projek** | Per projek: material diterima (qty & nilai), terpakai, susut, sisa, nasib sisa; dibandingkan dengan rencana pada PR | Excel |
| **Material Projek Belum Tuntas** | Projek `selesai` yang masih punya sisa material tanpa nasib (8.9) | Excel |

Dashboard aset memakai `GET /admin/assets/summary`; grafiknya memakai ApexCharts seperti
modul lain.

**Aturan zona waktu (wajib).** Seluruh penyaringan dan pengelompokan tanggal dikonversi
**di dalam SQL** memakai zona waktu dari `app_settings`, bukan zona browser, dan nilai
batas dikirim tanpa sufiks `Z`. Jebakan yang sudah pernah terjadi: export Excel
mengonversi ulang tanggal yang sudah dikonversi, sehingga laporan berbeda dari layar.
Lihat `docs/kontrak-waktu-transaksi-utc.md`.

---

## 13. Rencana Implementasi

Tiap fase berdiri sendiri dan bisa dirilis tanpa menunggu fase berikutnya. **Standar
tampilan di 11.4–11.7 berlaku untuk setiap fase**, bukan fase perapian tersendiri di
belakang: sebuah halaman tidak dianggap selesai sebelum lolos uji tujuh lebar layar (11.7).

### Fase 1 — Pendataan yang layak (fondasi) — **SELESAI 15 Sep 2026**

Terimplementasi di [database/migrations.go](../database/migrations.go) (blok akhir `RunMigrations`),
[models/asset.go](../models/asset.go), [services/asset.go](../services/asset.go),
[handlers/asset.go](../handlers/asset.go), [routes/routes.go](../routes/routes.go),
[ui/src/utils/assets.js](../ui/src/utils/assets.js), [ui/src/pages/Assets.vue](../ui/src/pages/Assets.vue),
dan [ui/src/pages/AssetDetail.vue](../ui/src/pages/AssetDetail.vue). Catatan pelaksanaan:
penyusutan dihitung lewat `LEFT JOIN LATERAL` pada setiap query aset; label QR memakai
paket `qrcode` yang di-*import* dinamis agar tidak membebani bundel utama; endpoint
`POST /admin/upload` kini juga menerima `assets.create`/`assets.update`.

- Migrasi kolom `assets` (3.1) + tabel `asset_movements` (3.2).
- Penomoran `asset_no`, backfill untuk aset lama (per outlet, urut `created_at`).
- Migrasi `condition = 'perbaikan'` → `status = 'perbaikan'`.
- Form tambah/edit diperluas; halaman **Detail Aset** + tab riwayat.
- Penulisan `asset_movements` pada create/update/hapus.
- Cetak label + QR.
- Tambah `assets.create`/`assets.update` ke izin endpoint upload (4.4).

### Fase 2 — Mutasi antar outlet — **SELESAI 15 Sep 2026**

Terimplementasi di [models/asset_transfer.go](../models/asset_transfer.go),
[services/asset_transfer.go](../services/asset_transfer.go),
[handlers/asset_transfer.go](../handlers/asset_transfer.go),
[ui/src/pages/AssetTransfers.vue](../ui/src/pages/AssetTransfers.vue), dan
[ui/src/api/assetTransfers.js](../ui/src/api/assetTransfers.js). Catatan pelaksanaan:

- Baris aset massal yang **habis seluruhnya** setelah diterima di tujuan di-*soft delete*
  di outlet asal (bukan disisakan berjumlah 0), sehingga tidak menyampah di daftar sementara
  seluruh riwayatnya tetap utuh di `asset_movements`.
- Endpoint `GET /admin/asset-transfers/available` hanya menawarkan aset yang sah dipilih —
  bukan transit, bukan terhapus, dan belum terikat dokumen lain — supaya kesalahan ketahuan
  sebelum dokumen dibuat, bukan sesudah.
- Sidebar kini punya grup **Perlengkapan** sendiri (Daftar Aset + Mutasi Antar Outlet),
  entri lama di Master Data dilepas.
- Izin `assets.transfer.view` diturunkan sekali jalan ke pemegang `assets.view`; wewenang
  **memindahkan** aset tidak diturunkan otomatis — terlalu besar untuk diberikan diam-diam.

- Tabel `asset_transfers` + `asset_transfer_items`, nomor `MTA…`.
- Service dengan transaksi penuh: submit/approve/send/receive/cancel + pemecahan &
  penggabungan baris (6.4).
- Endpoint + halaman Mutasi + berita acara.
- Izin `assets.transfer.*` — empat berkas sekaligus sesuai daftar periksa di 9.4 —
  dan penyaringan scope dua arah.

### Fase 3 — Perawatan penuh — **SELESAI 16 Sep 2026**

Terimplementasi di [services/asset_maintenance.go](../services/asset_maintenance.go),
[handlers/asset_maintenance.go](../handlers/asset_maintenance.go), dan
[ui/src/pages/AssetMaintenance.vue](../ui/src/pages/AssetMaintenance.vue). Catatan pelaksanaan:

- **Interval kategori disimpan sebagai daftar berurut, bukan map.** Urutan iterasi map di Go
  acak, sehingga satu unit AC (cocok dengan "AC" 3 bulan *dan* "Elektronik" 12 bulan) akan
  dijadwalkan berbeda-beda tiap kali dihitung. Pola pendek (≤3 huruf, seperti "ac") dicocokkan
  sebagai kata utuh supaya tidak ikut cocok pada "rack" atau "vacuum". Dikunci oleh
  [services/asset_maintenance_test.go](../services/asset_maintenance_test.go).
- **Perhitungan jatuh tempo memakai zona waktu aplikasi, bukan `CURRENT_DATE` Postgres.**
  Dengan server database ber-UTC, setiap pukul 17.00–24.00 WIB tanggalnya masih kemarin —
  pekerjaan yang jatuh tempo hari ini akan tampil "besok" dan yang terlambat tampil belum
  terlambat. Ekspresinya `(now() AT TIME ZONE '<zona>')::date`, **satu** konversi: melewatkan
  UTC dua kali justru menggeser ke arah sebaliknya.
- **Aset yang sedang dikerjakan tidak bisa dimutasi**, kecuali alasan mutasinya "dikirim untuk
  diperbaiki" (§6.3). Aturan ini dipakai bersama oleh pemeriksaan penyimpanan dokumen *dan*
  daftar pilihan aset, supaya tidak ada pilihan yang menipu.
- Menutup work order rutin otomatis menerbitkan siklus berikutnya; penjadwal harian (~03.00
  zona aplikasi) mengurus jadwal lama yang belum punya work order, dan idempoten.

- Kolom tambahan `asset_maintenances`, nomor `WOM…`, siklus status.
- Halaman Perawatan lintas aset + daftar jatuh tempo.
- Penjadwal harian + template interval per kategori.
- Tautan ke Pengadaan Jasa (7.4).
- Pemisahan izin `assets.maintenance.*` dari `assets.update`, termasuk pemberian sekali
  jalan bagi role yang selama ini mencatat perawatan (9.4b) agar tidak ada yang kehilangan akses.

### Fase 4 — Pengadaan → Aset, opname, penghapusan, laporan — **SELESAI 16 Sep 2026**

Terimplementasi di [services/asset_receiving.go](../services/asset_receiving.go),
[services/asset_disposal.go](../services/asset_disposal.go),
[services/asset_opname.go](../services/asset_opname.go),
[services/asset_report.go](../services/asset_report.go),
[ui/src/components/ReceivingDialog.vue](../ui/src/components/ReceivingDialog.vue),
[ui/src/pages/AssetDisposals.vue](../ui/src/pages/AssetDisposals.vue),
[ui/src/pages/AssetOpname.vue](../ui/src/pages/AssetOpname.vue), dan
[ui/src/pages/AssetReports.vue](../ui/src/pages/AssetReports.vue). Catatan pelaksanaan —
empat hal menyimpang dari rancangan awal, semuanya karena pengujian:

1. **`CreateGoodsReceipt` dipecah menjadi `CreateGoodsReceiptTx`** yang menerima transaksi
   pemanggil. Tanpa itu, penerimaan aset dan penerimaan stok berjalan di dua transaksi
   terpisah yang bisa putus di tengah — persis yang dilarang §8.4.
2. **Harga per satuan beli ≠ harga per satuan dasar.** Pengajuan menyimpan harga per kg,
   sedangkan GRN menyimpan harga per gram. Tanpa pembagian `dist_ratio`, gula 5 kg seharga
   Rp 70.000 masuk buku stok sebagai **Rp 70.000.000**. Ditemukan oleh uji terima #13.
3. **Tabel baru `pr_receiving_decisions`** (di luar rancangan awal). Rumus rekonsiliasi §8.5
   hanya mengurangi aset dan GRN, sehingga baris yang *sengaja* ditandai "habis pakai" akan
   muncul di laporan selamanya dan laporan itu berhenti dipercaya. Sekarang setiap keputusan
   punya jejak: siapa memutuskan apa atas baris belanja mana. Kolom `pr_item_key` juga
   ditambahkan ke `goods_receipt_items` supaya penautannya eksak, bukan tebak-tebakan nama.
4. **Penghapusan tidak memakai `is_deleted`.** Versi pertama menyembunyikan aset sepenuhnya —
   termasuk buku besarnya, yang justru menjadi tujuan berita acara. Sekarang barisnya tetap
   ada dengan `status='dihapus'` + `disposed_at`; yang menyembunyikannya dari daftar harian
   adalah filter status, dan riwayat serta nilai bukunya tetap bisa dibuka.

Catatan lain: aset yang **hilang saat opname tidak langsung dihapus**, melainkan menjadi
usulan penghapusan berstatus `pending` — kehilangan barang perlu persetujuan, bukan sekadar
hasil hitungan. Laporan memakai path `/admin/asset-reports/:type` (terpisah dari `/assets/:id`).

- Dialog Serah Terima + `receive-goods` (Bagian 8) dengan indeks unik idempotensi.
- Tabel & halaman opname aset, penghapusan.
- Lima laporan + export Excel + dashboard aset.

### Fase 5 — Material projek — **SELESAI 16 Sep 2026**

Terimplementasi di [services/project_material.go](../services/project_material.go),
[handlers/project_material.go](../handlers/project_material.go),
[ui/src/components/ProjectMaterialPanel.vue](../ui/src/components/ProjectMaterialPanel.vue),
dan panel baru di [ui/src/pages/ProjectDetail.vue](../ui/src/pages/ProjectDetail.vue).
Catatan pelaksanaan:

- **Barang mahal tetap menjadi aset meski dibeli untuk projek.** Urutan usulan di dialog:
  katalog stok → ambang harga (aset) → terikat projek (material) → habis pakai. AC dan
  kitchen set hasil renovasi adalah aset outlet, bukan material yang habis dikonsumsi.
- **Identitas `qty_received = terpakai + dikembalikan + susut + sisa` ditampilkan di layar**,
  lengkap dengan bilah kemajuan — bukan hanya berlaku diam-diam di database (§11.5).
- **Empat nasib sisa** (gudang / aset / susut / pindah projek) berjalan dalam satu transaksi
  masing-masing. Pengembalian ke gudang memakai `CreateGoodsReceiptTx` dan membagi
  `dist_ratio` seperti penerimaan pengadaan — pelajaran dari bug harga per satuan di Fase 4.
- **Susut wajib beralasan**; pemakaian melebihi sisa ditolak dengan pesan yang mengarahkan
  ke penerimaan yang belum dicatat.
- **Penutupan projek tidak diblokir** saat masih ada sisa — laporan "Material Projek Belum
  Tuntas" yang menagih. Blokir keras hanya membuat orang berhenti mencatat material.
- Aset hasil projek ditelusuri lewat catatan `"Pengadaan: <nomor>"` pada aset, sehingga
  panel "Aset yang Dihasilkan Projek Ini" tidak memerlukan kolom tambahan.
- **Material projek diterima di bagian Aset**, bukan di meja tersendiri: barangnya bukan
  barang katalog stok, jadi pengajuan projek jatuh ke antrean Penerimaan Peralatan.
- **Sisa material menjadi aset sebagai jalur baku**: outlet diwarisi dari projeknya,
  kategori terisi "Sisa Material Projek", nilai perolehan = harga material. Tanpa perlu
  mengisi apa pun, satu klik cukup.

### Fase 5 — Material projek (rencana awal)

- Tabel `project_materials` + `project_material_logs` (3.8).
- Tujuan **Material projek** pada dialog Serah Terima (8.2), memakai kunci idempotensi yang sama.
- Panel material + panel “Aset yang dihasilkan projek ini” di halaman detail projek.
- Layar pencatatan pemakaian yang ringan untuk ponsel.
- Penutupan material saat projek `selesai` (8.9) + dua laporan material.

Fase ini bergantung pada Fase 4 (dialog Serah Terima) tetapi tidak pada Fase 2–3, jadi bisa
didahulukan bila renovasi outlet sedang berjalan dan uangnya besar.

**Catatan build:** aplikasi dibangun lewat Docker (host tanpa Go terpasang).
Setiap perubahan UI perlu rebuild image nginx/app agar benar-benar terkirim; `excelize`
dipatok pada v2.10.0. Rinciannya di catatan build repo.

---

## 14. Uji Terima

Daftar yang harus lulus sebelum tiap fase dinyatakan selesai.

**Pendataan**
1. Aset baru mode `tunggal` dengan qty 3 → ditolak atau dipecah menjadi 3 baris ber-`asset_no` sendiri.
2. `asset_no` tidak pernah kembar, termasuk saat dua petugas menyimpan bersamaan.
3. Mengubah lokasi/PIC menghasilkan satu baris `asset_movements`.

**Mutasi**
4. Kirim 10 dari 40 kursi → outlet asal tersisa 30, dokumen `sent`, tidak ada baris baru di tujuan.
5. Terima 10 kursi → digabung ke baris sepadan di tujuan; `purchase_date` & harga perolehan diwarisi, bukan hari ini.
6. Terima 8 dari 10 → 2 kembali ke outlet asal, dokumen ditandai selisih.
7. Aset `transit` tidak muncul sebagai pilihan pada mutasi lain.
8. Pengguna ber-scope hanya outlet tujuan: bisa menerima, **tidak** bisa mengirim.
9. Kegagalan di tengah proses terima (mis. koneksi putus) tidak menyisakan aset di dua tempat.

**Perawatan**
10. WO `berjalan` membuat aset berstatus `perbaikan` dan menutup kemungkinan mutasi.
11. WO selesai dengan `condition_after` mengubah kondisi aset (perilaku existing tetap jalan).
12. Perawatan dengan `next_due_date` muncul di daftar jatuh tempo pada tanggal yang benar menurut zona aplikasi.

**Pengadaan → aset**
13. PR berisi 1 AC + 5 kg gula: AC menjadi aset, gula menjadi GRN, status PR `received`.
14. Klik “Serah Terima” dua kali → hanya satu aset yang lahir (indeks unik menolak yang kedua).
15. PR yang dipecah ke 2 vendor → aset menunjuk PR anak, tidak ada aset dari master.
16. Menerima 2 dari 5 unit → 3 sisanya muncul di laporan “Pengadaan Belum Lengkap”; **Lengkapi Penerimaan** hanya menawarkan 3.
17. Petugas tanpa `stockledger.adjust` tetap bisa menyelesaikan serah terima; baris gudang masuk antrean.

**Material projek**
17b. PR projek berisi 50 sak semen + 1 AC: semen masuk `project_materials`, AC menjadi aset,
    keduanya tertaut ke projek yang sama.
17c. Mencatat pemakaian 60 dari 50 sak → ditolak, dengan pesan yang mengarahkan ke penerimaan.
17c-2. Baris PR “Semen 100 sak” diterima tiga tahap (40 + 40 + 20) → tetap **satu** baris
    `project_materials` dengan `qty_received` 100 dan **tiga** log `penerimaan`; baris itu
    hilang dari laporan “Pengadaan diterima, barang belum lengkap” tepat setelah tahap ketiga.
17c-3. Petugas pengadaan tanpa `procurement.projects.manage` tetap bisa menyelesaikan
    penerimaan material, tapi tidak bisa mencatat pemakaian.
17d. Sisa 8 sak dikembalikan ke gudang → GRN terbentuk, `qty_returned` = 8, sisa = 0,
    dan stok gudang bertambah persis 8.
17e. Projek diubah ke `selesai` dengan sisa belum ditentukan → tetap bisa `selesai`,
    tapi muncul di laporan “Material Projek Belum Tuntas”.
17f. `qty_received` selalu sama dengan `qty_used + qty_returned + qty_wasted + sisa`
    untuk setiap baris, sesudah rangkaian operasi apa pun.
17g. Material projek tidak pernah muncul di katalog `stock_items`, laporan PPIC, atau
    perhitungan HPP.

**Tampilan (berlaku untuk setiap halaman baru)**
17h. Pada lebar 320px, tidak ada halaman modul ini yang menghasilkan scroll horizontal.
17i. Daftar aset, mutasi, perawatan, opname, dan material tampil sebagai kartu di bawah
    `sm`, dan sebagai tabel mulai `sm` — bukan tabel yang digeser-geser.
17j. Modal tambah aset dan form mutasi terbuka penuh layar di ponsel dengan tombol simpan
    terlihat tanpa menggulir.
17k. Nama barang sepanjang 80 karakter tidak merusak tata letak kartu mana pun.
17l. Keempat keadaan layar (memuat, kosong, gagal, terbatas izin) muncul benar di setiap
    halaman — termasuk kalimat kosong yang menjelaskan langkah berikutnya.
17m. Aksi yang tidak diizinkan role tersembunyi, bukan tampil lalu menolak saat ditekan.

**Laporan & izin**
18. Nilai buku aset berumur lebih dari umur ekonomis = `residual_value`, tidak negatif.
19. Angka di layar dan di file Excel identik untuk periode yang sama.
20. Role ber-scope `specific` tanpa outlet sama sekali tidak melihat satu aset pun (sentinel `__none__`).
21. Setelah dua kali restart aplikasi, izin yang dicabut admin tetap tercabut.

---

## 15. Keputusan

### 15.1 Sudah ditetapkan — 15 September 2026

Dua keputusan arsitektur di bawah sudah disetujui dan **tidak dibuka lagi** saat
implementasi. Keduanya bukan setelan `app_settings`; mengubahnya berarti mengubah rancangan.

| # | Keputusan | Alasan |
|---|---|---|
| A | **Material projek tidak masuk katalog `stock_items`.** Disimpan di tabelnya sendiri (`project_materials`, 3.8). | Semen, cat, dan keramik bukan bahan F&B. Memasukkannya ke katalog stok akan mengotori resep, HPP, MRP, dan seluruh laporan PPIC yang dihitung dari katalog itu. Pengembalian sisa ke gudang tetap lewat GRN, jadi barang yang memang layak jadi stok tetap bisa masuk (8.9). |
| B | **Material projek memakai izin `procurement.projects.view` / `.manage` yang sudah ada**, bukan kunci izin baru. | Mandor dan PIC projek adalah orang yang sama dalam praktik; kunci berjenjang empat tingkat hanya menambah risiko migrasi izin tanpa memisahkan peran yang berbeda. Pengecualian tetap berlaku: mengembalikan sisa ke gudang menuntut `stockledger.adjust` karena menambah stok sungguhan (9.1). |

### 15.2 Masih perlu ditetapkan manajemen

Angka-angka di bawah dipakai sebagai default dalam dokumen ini; semuanya disimpan di
`app_settings` sehingga bisa diubah tanpa rilis ulang.

| # | Pertanyaan | Usulan default |
|---|---|---|
| 1 | Ambang nilai sebuah barang dicatat sebagai aset | Rp 500.000 dan umur > 12 bulan |
| 2 | Umur ekonomis per kategori | Tabel di 5.3 |
| 3 | Mutasi antar outlet perlu persetujuan? | Ya |
| 4 | Siapa yang menyetujui mutasi & penghapusan | Manajer area / pusat |
| 5 | Frekuensi opname aset | 2× setahun per outlet |
| 6 | Ambang “ganti, jangan perbaiki lagi” | Biaya perawatan kumulatif > 50% harga perolehan |
| 7 | Nilai residu | Rp 0 (disusutkan penuh) |
| 7b | Material projek: dicatat qty-nya atau cukup biaya? | Dicatat qty penerimaannya (wajib); pencatatan pemakaian harian opsional per projek |
| 7c | Ambang nilai baris yang boleh ditandai “habis pakai” tanpa alasan tertulis | < Rp 1.000.000 |
| 7d | Nasib default sisa material saat projek selesai | Dikembalikan ke gudang bila ada di katalog stok; selain itu ditentukan manual |
| 8 | Aset lama tanpa harga/tanggal perolehan | Diinventarisasi dengan nilai Rp 0 dan ditandai “data belum lengkap”, bukan ditaksir |

---

## 16. Ringkasan Perubahan Kode

| Berkas | Perubahan |
|---|---|
| [database/migrations.go](../database/migrations.go) | ALTER `assets` & `asset_maintenances`; tabel `asset_movements`, `asset_transfers`, `asset_transfer_items`, `asset_disposals`, `asset_opname_sessions`, `asset_opname_items`; indeks unik idempotensi; blok izin **di akhir** fungsi |
| [models/asset.go](../models/asset.go) | Field baru pada `Asset`/`AssetMaintenance`; tipe `AssetTransfer`, `AssetMovement`, `AssetDisposal`, `AssetOpname*`, `AssetReceivingDraft` |
| [services/asset.go](../services/asset.go) | Penomoran, penulisan movement, penyusutan; berkas baru `asset_transfer.go`, `asset_maintenance.go`, `asset_opname.go`, `asset_report.go` |
| [services/purchase.go](../services/purchase.go) | Aksi `receive` memanggil penerimaan barang (aset + GRN) dalam satu transaksi |
| [services/goods_receipt.go](../services/goods_receipt.go) | Dipakai ulang apa adanya untuk baris bertujuan gudang **dan** untuk pengembalian sisa material projek |
| [models/project.go](../models/project.go), [services/project.go](../services/project.go), [handlers/project.go](../handlers/project.go) | Tipe & service material projek (`project_materials`, log, penutupan), panel aset & material di detail projek |
| [ui/src/pages/ProjectDetail.vue](../ui/src/pages/ProjectDetail.vue) | Panel “Material projek” + “Aset yang dihasilkan projek ini”, layar pemakaian & penutupan |
| [handlers/asset.go](../handlers/asset.go) | Handler untuk seluruh endpoint baru, semuanya melewatkan `getOutletScope` |
| [routes/routes.go](../routes/routes.go) | Rute baru; **rute statis didaftarkan sebelum `/assets/:id`** |
| [services/auth.go](../services/auth.go) | 13 kunci izin baru di `AllPermissions` (daftar lengkap di 9.4a) |
| [ui/src/layouts/DashboardLayout.vue](../ui/src/layouts/DashboardLayout.vue), [ui/src/router/index.js](../ui/src/router/index.js) | Grup sidebar “Perlengkapan” + tujuh rute baru (11.1) |
| [ui/src/pages/Roles.vue](../ui/src/pages/Roles.vue), [ui/src/components/PermissionMatrix.vue](../ui/src/components/PermissionMatrix.vue) | Kategori izin “Perlengkapan” (CRUD + 4 kelompok toggle), daftar halaman tujuan login — rinci di 9.4c–d |
| `ui/src/pages/` | `AssetDashboard.vue`, `AssetDetail.vue`, `AssetTransfers.vue`, `AssetMaintenance.vue`, `AssetOpname.vue`, `AssetDisposals.vue`, `AssetReports.vue` + perluasan `Assets.vue` |
| [ui/src/api/assets.js](../ui/src/api/assets.js) | Fungsi API baru mengikuti pola yang ada |
