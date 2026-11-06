# API Data Terbuka — Produk, Penjualan Produk, Medsos

| | |
|---|---|
| **Untuk** | Pengembang/sistem lain yang menarik data outlet tanpa login |
| **Alamat dasar** | `https://pos.nbp.co.id/api/v1/open` |
| **Sumber kode** | `routes/routes.go` (grup `/open`), `handlers/public_data.go`, `services/public_data.go`, tes `routes/open_data_test.go` |

---

## Aturan umum

- **Tanpa auth.** Tidak perlu header `Authorization`. Siapa pun yang tahu alamatnya bisa membaca
  datanya, termasuk nilai penjualan.
- **Outlet dirujuk lewat slug** (mis. `sekar-ayu`), tidak peka huruf besar/kecil. Respons tidak
  memuat ID internal apa pun (outlet, produk, akun medsos). Foto produk tidak disertakan karena
  nama berkasnya mengandung ID.
- Hanya outlet **aktif** yang punya slug. Slug tak dikenal / outlet nonaktif → `404`.
- **Tanggal** `YYYY-MM-DD`, zona waktu aplikasi (WIB). Rentang maksimal **366 hari**. Format salah,
  `date_from > date_to`, atau rentang terlalu panjang → `400`.
- **Batas permintaan:** 120 per menit (dihitung bersama untuk semua pemanggil, karena aplikasi
  melihat semua lalu lintas datang dari nginx). Kelebihan → `429`. Limiter ini terpisah dari
  halaman pemesanan/reservasi pelanggan (`/api/v1/public`), jadi penarikan data tidak mengganggu
  pelanggan.
- Respons boleh di-cache 60 detik (`Cache-Control: public, max-age=60`).
- Bentuk respons: `{"success": true, "data": ...}` atau `{"success": false, "error": "..."}`.

## Endpoint

### `GET /outlets` — daftar outlet

```json
{"success":true,"data":[{"name":"ATV Overland","slug":"atv-overland"}, ...]}
```

### `GET /outlets/{slug}/products` — produk & harga outlet

Produk aktif (tidak dihapus), urut kategori lalu nama.

```json
{"success":true,"data":{
  "outlet":{"name":"Sekar Ayu","slug":"sekar-ayu"},
  "total":136,
  "products":[{"name":"KATSU CURRY","category":"JAPANESE FOOD","kind":"food","kind_label":"Makanan","price":60000}, ...]}}
```

- `category`: spasi berlebih dirapikan; kosong → `"Tidak Berkategori"`.
- `kind` / `kind_label`: `food`/Makanan, `beverage`/Minuman, `activity`/Aktivitas, `other`/Lainnya,
  `""`/Belum ditentukan — sama dengan halaman Produk.
- `price`: harga jual di POS, sebelum pajak dan diskon.

### `GET /outlets/{slug}/product-sales?date_from=&date_to=` — penjualan per produk

Bawaan tanpa parameter: hari ini. Urut nilai penjualan terbesar.

```json
{"success":true,"data":{
  "outlet":{"name":"Sekar Ayu","slug":"sekar-ayu"},
  "date_from":"2026-10-01","date_to":"2026-10-31",
  "total_qty":3388,"total_revenue":128394000,
  "items":[{"name":"KATSU CURRY","category":"JAPANESE FOOD","qty":145,"revenue":8700000}, ...]}}
```

Angkanya **sama persis dengan menu Laporan Penjualan Produk** di aplikasi (memakai fungsi yang
sama): basis tanggal order, order void/ditahan tidak dihitung, `revenue` = harga × qty sebelum
diskon, kompliment, dan pajak. Karena itu bisa sedikit berbeda dari omzet di Laporan Penjualan
(yang memakai tanggal bayar dan sudah dipotong diskon/kompliment, ditambah PB1).

### `GET /social?date_from=&date_to=` — Instagram & TikTok semua outlet

Bawaan tanpa parameter: 30 hari terakhir. Outlet tanpa akun tetap muncul dengan `accounts: []`.

```json
{"success":true,"data":{"date_from":"2026-10-08","date_to":"2026-11-06","outlets":[
  {"name":"Sekar Ayu","slug":"sekar-ayu","accounts":[
    {"platform":"instagram","username":"sekarayu_nusantara","profile_url":"https://www.instagram.com/sekarayu_nusantara/",
     "latest":{"date":"2026-11-06","followers":4448,"following":…,"posts":…,"likes":null,"approx":false},
     "followers_change":34,
     "daily":[{"date":"2026-10-08","followers":4414,…}, …]}]}]}}
```

- Sumber: potret harian penjadwal medsos (`docs/penjadwal-medsos.md`), satu per akun per hari.
- `latest`: pembacaan terakhir yang punya angka pengikut, **tidak** dibatasi rentang tanggal.
- `daily`: semua potret dalam rentang. Nilai bisa `null` bila platform tidak menyerahkan angka itu
  (mis. `likes` Instagram).
- `followers_change`: pengikut terakhir − pertama dalam rentang; `null` bila kurang dari dua
  pembacaan.
- `approx: true`: angka dibulatkan oleh platform (mis. "12,3 rb").
- Hanya akun medsos yang aktif.

## Contoh

```bash
curl https://pos.nbp.co.id/api/v1/open/outlets
curl https://pos.nbp.co.id/api/v1/open/outlets/kala-mont-view/products
curl "https://pos.nbp.co.id/api/v1/open/outlets/kala-mont-view/product-sales?date_from=2026-10-01&date_to=2026-10-31"
curl "https://pos.nbp.co.id/api/v1/open/social?date_from=2026-10-01&date_to=2026-10-31"
```
