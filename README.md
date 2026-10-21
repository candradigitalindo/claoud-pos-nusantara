# Nusantara POS - Cloud API

Backend cloud server untuk sinkronisasi data multi-outlet POS.

## Tech Stack
- **Go** + **Fiber v2** (HTTP framework)
- **PostgreSQL** (database)
- **JWT** untuk admin auth
- **API Key** untuk outlet auth

## Quick Start

### 1. Setup PostgreSQL
```bash
createdb cloud_pos
```

### 2. Configure
```bash
cp .env.example .env
# Edit .env sesuai konfigurasi database
```

### 3. Run
```bash
make deps  # Download dependencies
make run   # Run server
```

### 4. Build
```bash
make build        # Build binary
make build-linux  # Build untuk Linux (production)
```

## API Endpoints

### Public
| Method | Endpoint | Keterangan |
|--------|----------|------------|
| GET | `/api/v1/ping` | Health check |

### Outlet API (Auth: Bearer API_KEY)
| Method | Endpoint | Keterangan |
|--------|----------|------------|
| POST | `/api/v1/outlets/:id/sync/batch` | Batch sync data |
| POST | `/api/v1/outlets/:id/orders` | Push order |
| POST | `/api/v1/outlets/:id/transactions` | Push transaction |
| POST | `/api/v1/outlets/:id/products` | Push product |
| POST | `/api/v1/outlets/:id/analytics/daily` | Push analytics |
| GET | `/api/v1/outlets/:id/updates?since=` | Pull updates |
| GET | `/api/v1/outlets/:id/orders` | List orders |
| GET | `/api/v1/outlets/:id/transactions` | List transactions |
| GET | `/api/v1/outlets/:id/products` | List products |
| GET | `/api/v1/outlets/:id/analytics` | Get analytics |
| GET | `/api/v1/outlets/:id/conflicts` | Get conflicts |
| POST | `/api/v1/outlets/:id/conflicts/:cid/resolve` | Resolve conflict |
| GET | `/api/v1/outlets/:id/sync/logs` | Sync logs |

### Admin API (Auth: Bearer ADMIN_TOKEN)
| Method | Endpoint | Keterangan |
|--------|----------|------------|
| GET | `/api/v1/admin/dashboard` | Dashboard stats |
| POST | `/api/v1/admin/outlets` | Create outlet |
| GET | `/api/v1/admin/outlets` | List outlets |
| GET | `/api/v1/admin/outlets/:id` | Get outlet detail |
| POST | `/api/v1/admin/outlets/:id/regenerate-key` | Regenerate API key |
| PUT | `/api/v1/admin/outlets/:id/toggle` | Activate/deactivate outlet |

## Flow Sinkronisasi

```
POS Outlet (pos-app.exe)
    │
    ├─ Push: Orders, Transactions, Products ──→ Cloud API ──→ PostgreSQL
    │
    └─ Pull: GET /updates?since=... ←── Cloud API ←── PostgreSQL
```

## Resep produk (opsional)

Di **Produk & Kategori → Tambah/Edit Produk**, pilih **Resep (opsional)**.
Pilihan **Tanpa resep** tetap memungkinkan produk dijual. Master resep yang
dipilih berlaku untuk satu produk terjual; bahan dipotong melalui sinkronisasi
penjualan ketika transaksi membawa `product_id` dan outlet memiliki gudang aktif.
Resep bisa diganti atau dilepas dari formulir yang sama.

API `POST /admin/products` dan `PUT /admin/products/:id` menerima
`recipe_master_id`. Pada PUT, field yang tidak dikirim atau bernilai `null`
mempertahankan pengaturan stok; string kosong melepasnya. Sinkronisasi produk
dari POS juga mempertahankan tautan yang diatur di cloud.

`GET /admin/products/recipe-options?outlet_id=...` menyediakan ID dan nama resep
untuk pemegang izin membuat/mengubah produk. Pilihan dibatasi ke resep publik
atau resep rahasia yang diizinkan untuk outlet tujuan, dengan komposisi valid.
Penautan diperiksa kembali di backend saat disimpan.

## Dua satuan stok per barang

Atur di **Item Stok → pilih Gudang Induk → Tambah/Edit Bahan**:
`Satuan Dasar / Pemakaian Outlet`, `Satuan Kemasan / Gudang Induk`, dan isi
kemasan. Contoh Cleo: dasar `botol`, kemasan `dus`, isi `24`. Setiap barang
memiliki konversinya sendiri, termasuk rasio pecahan (misalnya botol ke liter).

Saldo disimpan dalam satuan dasar. Daftar item dan buku stok menampilkan satuan
kemasan di gudang induk, satuan dasar di outlet, serta jumlah ekuivalennya.
Input saldo/minimum/HPP, penyesuaian, penerimaan, transfer, dan pembuangan bisa
memakai salah satu satuan. Transfer 2 dus menjadi 48 botol di outlet;
pembuangan 1 botol hanya mengurangi 1 botol. Penerimaan seharga Rp48.000/dus
menyimpan HPP Rp2.000/botol. Resep tetap memakai satuan dasar bahan.

API transfer, GRN, dan waste menerima `unit_used` bersama `qty_dist` (jumlah
dalam satuan terpilih). Tanpa `unit_used`, input mengikuti satuan distribusi
seperti klien lama. GRN menerima `cost_per_unit`; jika tidak dikirim, tetap
memakai `cost_per_base`. API saldo dan penyesuaian tetap memakai jumlah/HPP
satuan dasar. Transfer menyimpan hasil konversi saat dibuat sehingga perubahan
isi kemasan tidak menghitung ulang dokumen yang sudah ada. Pengaturan ini
menggunakan master barang yang sudah tersedia dan tidak mengubah saldo lama.

## Reject, retur supplier, dan barang kedaluwarsa

Menu **Gudang → Reject, Retur & Kedaluwarsa** memisahkan stok siap pakai dari
karantina. Laporkan reject, expired, spoil/basi, rusak, atau retur supplier
dengan memilih **batch barang**, jumlah dan satuannya, alasan, serta foto.
Konversi dus/botol mengikuti pengaturan barang. Batch yang dipilih menentukan
biaya dan tanggal kedaluwarsa; jumlahnya langsung dikeluarkan dari stok siap
pakai satu kali dan ditelusuri dalam dokumen karantina.

Alur: laporan → keputusan supervisor → pelaksanaan → penyelesaian.
Pelapor tidak bisa menyetujui laporannya sendiri. Supervisor dapat memilih
pemusnahan, retur ke supplier (induk), atau retur ke induk (outlet). Pelepasan
karantina memerlukan pemeriksaan/foto baru; batch yang sudah kedaluwarsa tidak
bisa dilepas. Pemusnahan masuk riwayat waste tanpa pemotongan stok kedua.
Retur outlet yang tiba di induk menjadi karantina baru, bukan otomatis stok
layak pakai. Tautan antar dokumen, petugas, waktu, dan foto tersimpan di riwayat.

Retur supplier membutuhkan supplier aktif, referensi invoice/surat jalan, dan
pilihan penyelesaian **barang pengganti** atau **refund**. Penerimaan pengganti
menambah stok barang yang sama sesuai jumlah aktual dan biaya barang retur;
tanggal kedaluwarsa baru dicatat. Finance mencatat jumlah barang yang
diselesaikan, nominal dana yang benar-benar diterima, referensi transaksi,
dan foto bukti. Keduanya bisa dicatat bertahap; dokumen selesai setelah seluruh
jumlah retur diselesaikan. Catatan refund ini adalah register penyelesaian
retur, belum membuat jurnal kas/bank otomatis pada laporan keuangan.

**Penerimaan transfer** kini mengisi jumlah diterima dan reject per barang
dalam satu pemeriksaan berfoto. Sisa yang belum datang tetap dalam perjalanan
dan dapat diterima berikutnya. Reject masuk karantina dengan nilai biaya
pengiriman; tidak otomatis dicatat sebagai kehilangan. Versi dokumen dan
kunci transaksi mencegah penerimaan atau penyelesaian ganda. Perencanaan MRP
menghitung hanya sisa transfer yang masih dalam perjalanan.

Foto melalui `/admin/stock-evidence` divalidasi sebagai JPG/PNG/WebP (maksimal
5 MB/25 megapiksel), diberi cap **waktu pencatatan server UTC** dan pengunggah,
serta disimpan dengan hash SHA-256. Timestamp tidak mengklaim waktu pengambilan
dari EXIF/kamera. Bukti terikat gudang dan akun pengunggah, berlaku 24 jam
untuk satu pengajuan/tindakan, dan tidak bisa dipakai ulang untuk dokumen lain.
Keputusan administratif memakai foto laporan awal; tindakan fisik dan
pencatatan refund memerlukan foto baru. Endpoint waste lama dan penyesuaian
langsung bertipe waste/expired/spoiled diarahkan ke alur karantina ini.

Migrasi membuat role berikut **sekali**, tanpa mengganti role akun yang ada:

| Role | Cakupan dan tahap |
|---|---|
| `gudang_induk_qc` | Induk: laporan karantina; menerima transfer, retur outlet, dan barang pengganti |
| `gudang_induk_supervisor` | Induk: keputusan karantina/retur/pemusnahan; persetujuan transfer |
| `gudang_induk_retur` | Induk: pelaksanaan retur/pemusnahan; pengiriman transfer |
| `finance_refund_supplier` | Mencatat refund supplier yang diterima, tanpa mutasi stok |
| `gudang_outlet_penerima` | Outlet: penerimaan penuh/sebagian, reject, dan laporan karantina |
| `gudang_outlet_supervisor` | Outlet: keputusan retur/pemusnahan/pelepasan |
| `gudang_outlet_pelaksana` | Outlet: mengirim retur dan melaksanakan pemusnahan |

Role outlet dibuat dengan cakupan **unit kerja tertentu yang masih kosong**;
pemilik harus menentukan outlet/unit kerjanya lewat menu Role sebelum role
tersebut digunakan. Role induk dan finance memakai cakupan semua outlet untuk
akses pusat, tetapi hak mutasinya dibatasi tahap masing-masing. Permission
`stockquality.*` dan `stocktransfers.reject` tersedia di matriks Role. Data
karantina baru tidak mengubah catatan waste/transfer historis. Laporan stok
siap pakai mengecualikan barang karantina; nilai kasus terlihat pada detailnya.

## Divisi F & B

Sidebar **F & B** mengelompokkan Dashboard, Pengembangan Produk, HPP Menu,
dan Resep & SOP. Resep yang sebelumnya berada di Gudang memakai data yang
sama; `/recipes` diarahkan ke `/fnb/recipes`.

| Tanggung jawab | Dukungan aplikasi |
|---|---|
| Pengembangan menu | Ide → uji coba → review → disetujui → diluncurkan; konsep, hasil uji, harga target, referensi resep dan produk POS |
| Standarisasi | Resep makanan/minuman, takaran bahan, langkah SOP, durasi, akses resep rahasia per outlet |
| Pengendalian biaya | HPP bahan + kemasan + overhead per porsi; target HPP yang dapat diatur, estimasi margin kontribusi dan harga minimum |
| Evaluasi rutin | Jumlah menu makanan/minuman, kelengkapan biaya, menu melewati target, serta tahap pengembangan |

Dasar pemetaan jobdesk: [Food & Beverage Director — Marriott Careers](https://careers.marriott.com/food-beverage-director/job/P1-7121094-0)
menjelaskan koordinasi operasi, kualitas layanan, anggaran dan kinerja divisi;
[Food & Beverage Cost Controller — Accor](https://careers.accor.com/job/food-and-beverage-cost-controller-in-ibis-new-delhi-aerocity-new-delhi-india-jid-109148)
mencakup standard recipe costing, analisis penjualan, laporan biaya dan kontrol persediaan.
Pemetaan menu di atas disesuaikan untuk aplikasi ini. Penilaian sensori dan SOP
dicatat sebagai hasil uji; modul ini tidak menggantikan pelatihan atau audit mutu lapangan.

Kategori F&B disimpan terpisah dari kategori POS sehingga sinkronisasi tidak
menimpanya. Produk mengikuti kategori resep bila belum memiliki pengaturan
F&B sendiri; data lama lainnya ditampilkan sebagai **Belum dikategorikan**.
Produk tetap boleh berdiri sendiri tanpa resep. Tautan resep/produk dalam
pengembangan hanya referensi; tidak otomatis menerbitkan produk atau mengganti
harga/resep POS. Setelah disetujui, isi pengembangan terkunci; kembalikan ke uji
coba untuk revisi. Perubahan bersamaan diperiksa melalui versi dokumen.

HPP adalah biaya standar **terkini**, bukan HPP penjualan historis atau laba
bersih. Biaya bahan mengikuti valuasi gudang outlet; gudang induk menjadi
fallback. Bahan setengah jadi tanpa valuasi dihitung dari komposisinya.
Harga bahan nol/tidak tersedia ditandai **Harga bahan belum lengkap**, bukan
ditampilkan sebagai margin 100%. Target awal 35% dapat diubah per menu dan
bukan standar wajib industri. Overhead/kemasan diisi per porsi oleh pengelola.

Saat server mulai, `RunFnbMigrations` menambahkan tabel pengembangan/profil HPP
dan kategori resep tanpa menghapus data lama. Role awal `fnb` dibuat sekali
dengan cakupan semua outlet dan halaman awal `/fnb/dashboard`; tidak ada akun
yang otomatis dipindahkan ke role ini. Izin baru `fnb.dashboard.view`,
`fnb.development.view/create/update/approve`, `fnb.costing.view/update` tersedia
di menu Role. Izin `recipes.*` tetap digunakan. Pengguna bercakupan outlet hanya
melihat resep publik/yang dibagikan dan data outletnya; pengubahan master resep
bersama dilakukan oleh pengelola dengan cakupan semua outlet. Role `fnb` tidak
mendapat izin penyesuaian stok, transfer stok, pembayaran, atau pengelolaan pengguna.
Izin `recipes.create` juga mengikuti akses produksi bahan setengah jadi yang
sudah ada pada sistem.

## CCTV outlet

Halaman **Pengaturan → CCTV** mengelompokkan kamera menurut nama outlet,
menyediakan kelola kamera, tes koneksi, live view, dan playback bila perangkat
mendukungnya. Akses mengikuti izin `cameras.*` dan scope outlet akun login.
**Pantau outlet** membuka hingga empat kamera sekaligus; kamera dari outlet
berbeda juga dapat ditambahkan melalui **Tambah ke monitor**. Pencarian
mencakup nama kamera, nama outlet, dan merek. Pemutar mencoba menyambung ulang
dua kali jika koneksi gagal dan menyediakan tombol coba lagi.
Tayangan menggunakan HLS; gunakan video H.264 agar kompatibel dengan browser.
Cloud menyimpan konfigurasi terenkripsi; rekaman tetap di DVR/SD card kamera.

Compose menyertakan go2rtc internal tanpa port publik. Nginx meneruskan
`/cctv/` ke Go untuk memeriksa sesi admin, izin, outlet, dan sesi HLS pada
setiap permintaan. Konfigurasi runtime relay berada di RAM (tmpfs); registrasi
stream memakai PATCH agar URL kamera tidak ditulis ke konfigurasi di disk.
Lihat [API go2rtc](https://github.com/AlexxIT/go2rtc/blob/v1.9.14/internal/streams/api.go).

Pilihan koneksi:

- Preset RTSP: Hikvision/HiLook, Dahua/Imou, Tapo dan Reolink pada model yang
  menyediakan protokol tersebut; DVRIP untuk perangkat iCSee/XMEye yang cocok.
- ONVIF dan RTSP manual untuk merek lain, termasuk HK/HKM, V380, Yoosee dan kamera
  China lainnya **jika modelnya menyediakan protokol lokal**. Nama merek dan
  aplikasi tidak menjamin kompatibilitas. Tidak ada fallback ke Hikvision.
- Tuya Smart: login akun, pilih region dan kamera dari dashboard, lalu simpan
  pada outlet. Koneksi memakai cloud vendor tanpa penghubung/VPN. Konektor
  go2rtc ini tidak mendukung login Smart Life. URL sumber hanya dipindahkan
  melalui tiket terenkripsi yang terikat admin/outlet selama 10 menit.
- URL konektor vendor: `tapo://`, `xiaomi://`, `bubble://`, `tuya://` sesuai
  dokumentasi go2rtc dan dukungan model. `exec:`, `ffmpeg:`, `file:` dan
  argumen fragment ditolak. UID/QR aplikasi Hik-Connect, EZVIZ, V380 atau
  Yoosee bukan URL stream universal; model cloud tertutup memerlukan API/SDK
  vendor. Integrasi UID layanan-layanan tersebut belum tersedia.

Untuk perangkat outlet yang memakai **Hik-Connect, HKM-Smartview, Dahua dan
EZVIZ**, pilih koneksi berikut:

| Perangkat / aplikasi | Koneksi yang tersedia di sistem |
| --- | --- |
| Hikvision / Hik-Connect | Preset Hikvision; isi IP dan akun kamera atau DVR/NVR, lalu pilih channel. Akun perangkat berbeda dari akun Hik-Connect. |
| HKM / HKM-Smartview | Pilih merek HKM, lalu ONVIF atau RTSP sesuai model. Untuk kamera melalui DVR/NVR, gunakan alamat dan akun perekamnya. UID Smartview belum didukung langsung. |
| Dahua | Preset Dahua; isi IP dan akun kamera atau DVR/NVR, lalu pilih channel. Login cloud/P2P belum diimplementasikan. |
| EZVIZ | RTSP manual pada model yang mendukung. Untuk C6N/TY1/TY2, [panduan resmi EZVIZ](https://m-support.ezviz.com/faq/article/How-to-set-up-C6N-TY1-TY2-as-a-webcam) mencantumkan `rtsp://admin:PASSWORD_LOKAL@IP_KAMERA:554/ch1/main`. Gunakan password lokal saat ini; kode verifikasi pada label berlaku bila belum diganti. Model lain perlu URL sesuai dokumentasinya. |

Semua koneksi lokal di atas dapat melalui penghubung outlet tanpa VPN.
Akses cloud langsung adalah integrasi terpisah: [Hik-Connect OpenAPI](https://tpp.hikvision.com/products/HC-Integration)
memerlukan pengajuan API key dan persetujuan akses developer; EZVIZ menyediakan
[Open Platform/SDK](https://irusopen.ezviz.com/developer). Login cloud kedua
layanan tersebut belum diimplementasikan. Untuk HKM-Smartview, protokol dan
dukungan cloud perlu dipastikan dari model/DVR serta dokumentasi vendornya;
nama aplikasi saja tidak menentukan jenis koneksi.

Untuk kamera lokal, jalur yang disediakan tanpa VPN adalah **penghubung
outlet**, satu komputer Docker yang selalu menyala dan dapat menjangkau
IP kamera. Buat token di **Koneksi outlet**, unduh `cctv-gateway.env`, lalu
jalankan dari salinan proyek ini pada komputer outlet:

```bash
docker compose --env-file cctv-gateway.env -f docker-compose.cctv-gateway.yml up -d --build
```

Penghubung hanya membuat koneksi HTTPS keluar ke cloud dan tidak membuka
port masuk. Kamera pada outlet berbeda boleh memakai IP lokal yang sama.
Token tersimpan sebagai hash di cloud, terikat satu outlet, dan dapat diganti
dari dashboard. Setelah token diganti, unduh konfigurasi baru lalu buat ulang
container penghubung. Gunakan URL HTTPS publik cloud, tanpa `/api/v1`.
HTTP hanya diizinkan agen untuk uji localhost.

Jalur **Langsung** juga tersedia bila server sudah dapat menjangkau alamat
perangkat atau layanan vendor. Tidak ada kewajiban VPN. ONVIF menggunakan IP
yang dimasukkan operator; pencarian broadcast otomatis belum disediakan.
Gunakan **Cari profil ONVIF** pada formulir untuk memilih channel/kualitas
yang benar pada DVR/NVR. Perangkat dengan satu profil dapat dipilih otomatis;
perangkat dengan beberapa profil memerlukan pemilihan eksplisit, karena
urutan profil tidak selalu mengikuti nomor channel. Kredensial URL profil
dipindahkan ke formulir hanya dalam tiket terenkripsi yang terikat admin/outlet.
**Tes sebelum simpan** menguji video dari formulir tanpa menulis konfigurasi;
saat mengedit, field rahasia kosong memakai kredensial tersimpan. Kotak hapus
tersedia untuk password, URL sumber ONVIF khusus, dan URL rekaman tersimpan.
Untuk RTSP manual, isi URL pengganti karena koneksi selalu memerlukan URL.

Pilihan playback diaktifkan untuk preset Hikvision/Dahua atau URL rekaman
RTSP khusus. URL rekaman dapat memakai `{start}`, `{end}` (ISO basic),
`{start_iso}`, `{end_iso}`, `{start_unix}`, `{end_unix}`. Dukungan live tidak
otomatis berarti rekaman SD card dapat diakses. Pilihan jam mengikuti **zona
waktu kamera**, default `Asia/Jakarta` untuk konfigurasi lama. Hikvision dan
placeholder `{start}`/`{end}` dikonversi ke UTC; Dahua serta `{start_iso}`/
`{end_iso}` memakai waktu lokal perangkat, sedangkan `{start_unix}`/
`{end_unix}` memakai Unix timestamp. Atur zona waktu perangkat WITA/WIT pada
formulir kamera. Jam DVR harus sesuai dengan zona yang dipilih.

Setiap penonton mendapat sesi relay sendiri. Menutup pemutar/monitor melepaskan
sesi tersebut tanpa menghentikan penonton lain. Sesi idle lebih dari tiga menit
dibersihkan otomatis; playback dibatasi paling lama satu jam. Perubahan,
penonaktifan, dan penghapusan kamera mencabut sesi lama seketika, lalu cleaner
mencoba menghapus stream dari relay asal, maksimal tiga upaya jika relay tidak
dapat dijangkau. Setelah itu sesi tetap ditolak oleh backend. Server membatasi
256 sesi bersamaan.
Dashboard memperbarui status penghubung setiap 20 detik selama halaman terlihat.
**Cabut akses penghubung** menghapus token outlet; koneksi berikutnya memerlukan
token baru. Penggantian token juga mencabut sesi lama milik penghubung itu.

Simpan `CAMERA_ENC_KEY` secara tetap jika diisi; nilai kosong memakai turunan
`JWT_SECRET`. Perubahan kunci tanpa migrasi membuat kredensial lama tidak
terbaca. Streaming memerlukan login admin; token statis hanya untuk API
konfigurasi. Layanan belum menyediakan SDK cloud universal semua merek.

Tes unit: `go test ./...`. Tes migrasi/CRUD dan penghubung HTTPS memakai
Postgres uji kosong, `DB_NAME` wajib berakhiran `_test`:

```bash
go test ./services -run TestCCTVMultiVendorAndOutboundGatewayDB -v
```

## Konfigurasi di POS App

Setelah cloud API running, setting di POS app (`.env`):
```
SYNC_ENABLED=true
CLOUD_API_URL=https://cloud-api.example.com
CLOUD_API_KEY=<api_key dari admin>
OUTLET_ID=<outlet_id>
OUTLET_CODE=JKT-001
WEBHOOK_SECRET=shared-secret
```
