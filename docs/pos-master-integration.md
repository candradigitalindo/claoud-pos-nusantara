# Acuan master Cloud POS dan Flutter

Dua repositori tetap dipisahkan: `candradigitalindo/claoud-pos-nusantara` untuk
Cloud API/dashboard, dan `candradigitalindo/app-pos-nusantara` untuk aplikasi
Flutter. Cabang `master` pada keduanya menjadi hasil konsolidasi. Penggabungan
kode tidak menjalankan deployment atau migrasi pada database produksi.

## Asal kode

| Bagian | Acuan sebelum konsolidasi | Yang dipertahankan |
| --- | --- | --- |
| Cloud saat ini | `claoud-pos-nusantara`, `0762c6a` | Aset/perlengkapan, pengadaan, keuangan, F&B, stok, PPIC, reservasi, CCTV, dan hak akses terbaru |
| Cloud pendamping | `cloud-nusantara/main`, `0a4fc9d` | Pesanan online, kontrak QRIS, add-on, biaya tambahan, dan pengujian terkait |
| Flutter Nusantara | `app-pos-nusantara/main`, `8c36e15` | Reservasi dan DP tervalidasi |
| Flutter kasir/station | `app-pos-resto/feat/kasir-station-parity`, `57a5070` | Laci kasir, dapur station, aksi kasir yang sama, serta pembaruan layar langsung |

Riwayat Cloud digabungkan dengan merge Git. Implementasi aset lama dari repo
pendamping tidak menggantikan alur aset/pengadaan yang lebih baru. Flutter
digabungkan tiga arah menggunakan subtree `app-pos-flutter` pada `bf1071d`
sebagai dasar, sehingga fitur reservasi dan fitur station tetap tersedia.
Repo/cabang asal dipertahankan sebagai referensi.

## Perbaikan integrasi

- ID lokal disimpan terpisah dari ID cloud yang unik. Registri tambahan
  `pos_entity_ids` mencatat pemetaan per outlet dan jenis data, termasuk bila
  pembayaran/mutasi datang sebelum order/shift induknya. ID cloud lama tidak
  ditulis ulang dan tidak ada pemindahan data antar-outlet.
- Flutter menyimpan `order_items.product_id` melalui migrasi SQLite v14 dan
  mengirimkannya bersama item order/transaksi. Data lama tanpa ID hanya dapat
  dicocokkan dengan satu produk yang namanya cocok dalam outlet yang sama;
  data ambigu ditolak untuk diperiksa, bukan dipilih secara acak.
- Baris pembayaran mempertahankan waktu pembayarannya sendiri. Penghitungan
  fallback shift memakai waktu buka/tutup shift, bukan waktu sinkronisasi.
- Setiap kejadian void/titipan memiliki identitas antrean tersendiri. Balasan
  batch membawa `event_id`; balasan ambigu tidak menghapus antrean lain.
- Add-on dan biaya tambahan disimpan oleh server. Jenis batch yang belum
  didukung dikembalikan sebagai kegagalan, sehingga tidak dianggap tersimpan.
- DP reservasi diperbarui berdasarkan selisih tervalidasi, tanpa mencatat ulang
  DP lama. Reservasi yang order lokalnya sudah lunas tidak dibuka ulang.
  Rincian pembayaran hasil restore yang belum lengkap harus diperiksa sebelum
  pembaruan DP. Kegagalan penutupan reservasi membuat transaksi tetap perlu
  dicoba ulang tanpa menggandakan pembayaran yang sudah disimpan.
- Menu publik mengirim ID cloud untuk reservasi dan `product_local_id` untuk
  POS/pesanan online; kedua identitas tidak dipertukarkan.

Tidak ada perhitungan ulang stok atau keuangan historis secara otomatis.
Data produksi yang sudah mengalami benturan ID atau kehilangan rincian
memerlukan rekonsiliasi tersendiri setelah penyebab operasionalnya dipastikan.

## QRIS dan batas pemeriksaan

Penyedia QRIS nyata belum diimplementasikan. Tanpa konfigurasi, layanan
pembayaran otomatis nonaktif. Simulasi hanya aktif jika **kedua** nilai ini
diatur secara eksplisit:

```text
PAYMENT_GATEWAY_PROVIDER=mock
PAYMENT_GATEWAY_ALLOW_MOCK=true
```

Antarmuka menandainya sebagai simulasi; tidak ada uang yang diterima melalui
adapter ini. Pengujian memeriksa pembatasan outlet/provider dan transaksi atomik
antara status pembayaran dan status pesanan online.

Snapshot order baru menyertakan rincian pembayaran, biaya, pilihan add-on,
identitas item, dan reservasi agar order yang masih berjalan dapat dipulihkan.
Data historis yang belum pernah mengirim rincian tersebut tetap memerlukan
sinkronisasi dari perangkat asal atau rekonsiliasi. Sistem tidak mengarang
metode pembayaran, waktu, maupun biaya yang hilang.
Pemulihan lengkap baru tersedia setelah backend ini dan APK baru digunakan
serta order aktif disinkronkan ulang. Saat rilis terpisah, backend dipasang
lebih dahulu agar snapshot dari APK baru tidak dibuang oleh server lama.

Pengaturan printer/dapur dari cloud serta penyedia pembayaran nyata masih
merupakan pekerjaan lanjutan. Pembaruan DP lokal dilakukan
saat reservasi dibuka/dipilih kembali. Pengujian otomatis tidak menggantikan
uji tablet, cetak printer, laci uang, dan pembayaran nyata sebelum rilis APK.

## Perbaikan lanjutan sinkronisasi dan pemulihan

- Flutter membaca `payment_info` baik sebagai objek maupun teks JSON seperti
  respons Cloud. Snapshot `payment_info.restore.version = 1` membawa baris
  pembayaran dengan ID/metode/waktu asli, biaya dengan `sync_id` portabel,
  reservasi, dan metadata order. Pilihan add-on, catatan, kategori, serta ID
  item turut dipertahankan; harga item sudah mencakup add-on.
- Restore hanya mengisi entitas yang belum ada. Order/shift lokal, transaksi
  lunas, dan pekerjaan yang masih menunggu sinkronisasi tidak ditimpa.
  Meja tagihan yang dipulihkan ditandai terisi; meja bernama valid yang belum
  ada dibuat. Benturan dengan tagihan lokal lain membatalkan restore.
  Respons tidak lengkap atau jumlah yang tidak cocok membatalkan transaksi
  restore secara utuh. Saldo pembayaran lama tanpa rincian tetap dipertahankan,
  tetapi pelunasan/penambahan DP ditahan sampai rinciannya diperiksa.
- Pada database operasional kosong, master biaya aktif/nonaktif dari Cloud
  dipulihkan berdasarkan `sync_id`; biaya bawaan yang tidak ada di Cloud
  dinonaktifkan agar tidak terhitung dua kali. Pada kasir yang sudah beroperasi,
  definisi biaya lokal dipertahankan dan referensi yang hilang ditolak.
- Cloud mengembalikan referensi shift lama sebagai ID lokal milik outlet yang
  sama, termasuk mutasi kas yang masih merujuk ID lokal. Jika sebuah ID dapat
  berarti dua hal, ID canonical diprioritaskan. Tidak ada penulisan ulang data
  produksi. Order yang sudah memiliki transaksi selesai dikeluarkan dari restore;
  kiriman order lama tidak menurunkan status lunas atau mengganti snapshot baru.
- Penarikan menu memvalidasi seluruh halaman sebelum mengubah database lokal.
  Halaman berulang/terpotong, respons tidak valid, dan kegagalan jaringan tidak
  menghapus menu lokal atau memajukan checkpoint. Full pull dan incremental
  pull diantrekan; checkpoint dipisahkan per URL/outlet dan berasal dari server.
  Setelah full pull, perubahan diputar ulang satu kali dari awal agar perubahan
  selama pengambilan halaman tidak terlewat.
- Penggantian outbox, mutasi kas, serta buka/tutup/serah-terima shift dilakukan
  dalam transaksi SQLite bersama antreannya. Mutasi kas lama tanpa antrean
  dimasukkan kembali dengan identitas dan waktu asli. Monitor rekonsiliasi
  membaca status pekerjaan terbaru, sehingga keberhasilan lama tidak menutupi
  pekerjaan baru yang masih pending atau gagal.
- Snapshot order dan antreannya dibuat dalam satu transaksi. Backfill
  membandingkan revisi order terbaru dengan payload antrean terakhir, sehingga
  pembayaran sebagian atau perubahan item yang ter-commit sebelum aplikasi
  tertutup tetap dikirim ulang. Pembaruan status dapur ikut menaikkan revisi
  order, walaupun status keseluruhan pesanan belum berubah.

## Menjalankan uji kelayakan terisolasi

Prasyarat: Docker, Git, Bash, Python 3, ruang disk untuk Flutter SDK dan cache.
Jalankan dari checkout Cloud:

```bash
scripts/test-pos-integration.sh \
  --flutter ../app-pos-nusantara \
  --artifacts /path/baru/hasil-uji-master
```

Runner membuat snapshot sumber tanpa `.env`, jaringan Docker **internal** baru,
PostgreSQL sementara dengan nama database berakhiran `_test`, dan data sintetis.
Tidak ada port yang diterbitkan ke host, mount volume produksi, atau akses API
produksi. Kontainer dibatasi CPU/memorinya dan dibersihkan berdasarkan ID/label
run sendiri. Artefak pemeriksaan disimpan setelah kontainer dihentikan.

Pemeriksaan mencakup build Go dan Vue, seluruh tes Go dengan database nyata,
tes Flutter, serta E2E Flutter melawan API uji untuk pesanan online, reservasi,
restore, dan siklus unggah/ganti/hapus foto produk.
Tes wajib yang terlewati tidak dianggap lulus. Gunakan `--phase backend`,
`--phase frontend`, atau `--phase flutter` untuk mengulang bagian yang gagal;
`--offline` memakai cache yang sudah disiapkan. Lihat `--help` untuk lokasi SDK
dan opsi artefak. Hasil mesin tersedia dalam `summary.json` dan `result.txt`.

## Hasil pemeriksaan konsolidasi — 31 Oktober 2026

| Pemeriksaan | Hasil |
| --- | --- |
| Build API Go | Lulus |
| Seluruh tes Go dengan PostgreSQL sintetis | 446 lulus, 3 dilewati |
| Build dashboard Vue/Vite | Lulus; ada peringatan ukuran chunk |
| Analyzer Flutter 3.41.9 / Dart 3.11.5 | 0 error, 0 warning; 21 informasi `avoid_print` pada utilitas simulasi |
| Seluruh tes Flutter dengan API sintetis | 74 lulus, 0 dilewati |
| E2E pesanan online dan reservasi sampai pelunasan | Lulus, termasuk simulasi QRIS |

Tiga tes Go yang dilewati adalah `TestCCTVGo2rtcHLSIntegration` dan
`TestScrapeLive` yang memerlukan layanan eksternal, serta
`TestKesimpulanStabilTerhadapRentangMinggu` yang memerlukan riwayat statistik
memadai. Tes integrasi POS wajib benar-benar dijalankan.

Log dan ringkasan pemeriksaan pada VPS tersimpan di
`/home/candra/projects/pos-integration-staging/artifacts/master-backend`,
`master-frontend`, dan `master-flutter`. Pemeriksaan memakai database baru
dan data sintetis. Hasil ini memvalidasi konsolidasi kode; belum merupakan
persetujuan rilis APK atau deployment produksi.

## Hasil perbaikan lanjutan — 31 Oktober 2026

- Seluruh tes Go pada PostgreSQL sintetis: **451 lulus, 3 dilewati** dengan
  alasan yang sama seperti pemeriksaan konsolidasi di atas.
- Seluruh tes Flutter: **115 lulus, 0 dilewati**, termasuk tiga alur E2E wajib:
  pesanan online, reservasi, serta POS → API → restore SQLite pada outlet uji
  tersendiri. Uji restore membandingkan ID item/pembayaran, nominal, metode,
  waktu asli, reservasi, dan biaya, lalu mengulang restore untuk memeriksa
  idempotensi. Kegagalan tulis outbox, halaman menu terpotong, benturan data
  lokal, dan pemulihan revisi setelah aplikasi tertutup juga diuji.
- Analyzer Flutter: **0 error, 0 warning**; 21 informasi `avoid_print` lama
  pada utilitas simulasi. Pemeriksaan whitespace dan sintaks runner lulus.

Artefak berada di
`/home/candra/projects/pos-integration-staging/artifacts/restore-backend-final`
dan `restore-flutter-final`. Tidak ada perubahan UI Vue pada tahap ini.
Kontainer/database uji dibersihkan setelah pengujian; production dan APK yang
sedang digunakan tidak diubah oleh pemeriksaan ini.

## Sinkronisasi foto produk Cloud → Flutter

- Foto tetap dikelola melalui unggah/hapus produk di dashboard Cloud. Push
  perubahan produk dari POS tidak mengambil alih atau mengosongkan foto Cloud.
  Penyimpanan form edit produk lama juga tidak menimpa URL yang sudah diganti
  atau dihapus oleh sinkronisasi di SQLite.
  Full pull dan incremental pull mengirim `photo_url`; string kosong berarti
  foto dihapus. Penggantian foto menghasilkan URL baru agar cache lama tidak
  dipakai untuk gambar baru. File lama dibersihkan setelah perubahan database
  berhasil; kegagalan penulisan mempertahankan foto sebelumnya.
  Jalur unduh foto membaca berkas terkini sehingga URL yang telah dihapus
  langsung mengembalikan 404, tanpa sisa cache file terbuka dari server.
- SQLite Flutter v15 menambah kolom foto tanpa menghapus produk/transaksi
  sebelumnya. Cursor sinkronisasi diberi versi `photos-v1` per URL/outlet
  agar aplikasi yang diperbarui menarik foto yang sudah ada sebelum upgrade.
  Respons server lama yang tidak memiliki field foto mempertahankan nilai lokal.
- Foto tampil pada daftar produk, kasir, waiter, dan station. URL relatif
  `/uploads/...` diubah menjadi URL pada host Cloud yang dikonfigurasi, termasuk
  konfigurasi yang berakhiran `/api/v1`. Foto kosong/gagal diunduh memakai
  placeholder; daftar menu tetap dapat digunakan.
  Menu yang sedang terbuka memperbarui foto setelah sinkronisasi, termasuk
  penghapusan, tanpa mengubah harga atau jumlah barang dalam keranjang aktif.
  Gambar pada tile di-decode sesuai ukuran tampilan dengan batas 768 piksel
  agar gambar kamera beresolusi besar tidak memenuhi memori tablet.
- Cache berkas lokal bertahan setelah aplikasi ditutup, dibatasi 100 MiB/256
  gambar dengan batas 10 MiB per gambar serta validasi format/resolusi. Unduhan
  berjalan di latar belakang dan dicoba lagi pada siklus sinkron berikutnya.
  Foto yang sudah tersimpan dapat tampil tanpa internet; foto yang belum
  diunduh atau sudah dikeluarkan dari cache memerlukan koneksi kembali.
- Station dapat mengambil foto yang tersimpan di Main POS melalui route LAN
  dengan sesi yang sama seperti akses menu. URL versi lama, produk terhapus,
  atau sesi tidak valid ditolak. Kredensial API/sesi tidak dikirim ke URL gambar
  publik. Koneksi internet tidak diperlukan bila foto tersedia di cache Main POS
  atau station dan jaringan lokal tetap terhubung.

Backend dan aplikasi Flutter baru perlu dirilis untuk mengaktifkan alur ini
di perangkat operasional, dengan backend dipasang lebih dahulu. Perubahan
sumber dan pengujian terisolasi tidak memperbarui container production maupun
APK yang sedang digunakan.

## Hasil uji sinkronisasi foto — 31 Oktober 2026

- Build API dan seluruh tes Go dengan PostgreSQL sintetis: **456 lulus,
  3 dilewati**, dengan alasan eksternal/data yang sama seperti sebelumnya.
- Seluruh tes Flutter: **142 lulus, 0 dilewati**. Empat E2E wajib benar-benar
  dijalankan: pesanan online, reservasi, restore, dan foto produk melalui API.
- Analyzer Flutter: **0 error, 0 warning**; 21 informasi `avoid_print` lama.
  Pemeriksaan whitespace dan sintaks runner juga lulus.
- Siklus foto diuji dari login admin sintetis, unggah, full pull, edit produk
  dari POS, ganti foto, incremental pull, cache setelah restart tanpa HTTP,
  hapus foto, sampai penolakan file bukan gambar. URL lama langsung 404.
  Uji terpisah mencakup batas cache/berkas, cache LAN dengan sesi, unduhan
  lambat tanpa menahan menu, upgrade database/cursor, pembaruan widget,
  serta keranjang kasir/waiter/station yang tetap utuh.

Artefak akhir berada di
`/home/candra/projects/pos-integration-staging/artifacts/photo-backend-final`
dan `photo-flutter-verified`. Manifest `source-verification.json` memastikan
103 berkas Dart pada snapshot uji sama dengan sumber akhir. Semua container,
database, dan jaringan uji dibersihkan. Production tidak dideploy, APK belum
dibangun/dirilis, dan pengujian tablet fisik tetap diperlukan sebelum rilis.

## Jenis produk dan analisis katalog

Halaman Produk Cloud menyediakan jenis **Makanan**, **Minuman**, **Aktivitas**,
dan **Lainnya**, termasuk filter serta penanda pada daftar global dan per outlet.
Jenis produk terpisah dari kategori POS. Form edit yang hanya mengubah nama,
harga, atau kategori tidak mengirim ulang jenis lama, sehingga perubahan jenis
terbaru dari F&B atau analisis tetap dipertahankan.

Jenis memakai profil F&B yang sama, dengan fallback ke resep utama jika belum
ada profil. Migrasi `RunFnbMigrations` memperluas jenis profil dan menyediakan
view `product_menu_kinds`; startup backend gagal bila migrasi ini gagal.
Aktivitas dan Lainnya tidak masuk perhitungan costing/dashboard F&B. Pengaturan target HPP,
kemasan, dan overhead yang sudah tersimpan tetap dipertahankan. Klasifikasi
resep serta pengembangan menu F&B tetap Makanan/Minuman.

Tombol **Analisis jenis** membaca seluruh produk yang belum dihapus dalam
cakupan akses pengguna, bukan hanya halaman daftar yang terbuka. Laporan
menampilkan jenis saat ini, saran, alasan, dan produk yang perlu ditinjau.
Analisis menggunakan aturan nama/kategori/resep yang dapat diperiksa. Sesuai
ketentuan pemilik, produk yang belum masuk tiga jenis utama, termasuk nama
tidak dikenal dan paket campuran, dikelompokkan sebagai **Lainnya** (`other`).
Alasan menyebutkan ketentuan ini bersama bukti katalog yang mendasarinya;
penetapan Lainnya tidak menyatakan isi paket sudah dapat dipastikan.
Jenis yang sudah ditentukan, termasuk Lainnya, tidak ditimpa. Nilai jenis lama
yang tidak valid tetap perlu ditinjau. Nilai kosong/Belum ditentukan berbeda
dari Lainnya dan tetap tersedia untuk form serta filter.

Sesuai ketentuan pemilik, analisis outlet **ATV Overland** mengelompokkan semua
produk yang belum memiliki jenis sebagai Aktivitas, termasuk barang pendukung.
Aturan memeriksa nama outlet lengkap (abaikan kapitalisasi/spasi berlebih),
bukan kategori ATV atau potongan nama outlet lain. Jenis eksplisit tetap
dipertahankan sesuai kontrak analisis; aturan ini bukan pembatas form edit.
Perubahan nama outlet setelah pratinjau membatalkan penerapan dengan 409.

Pratinjau memerlukan `products.view`; penerapan memerlukan `products.update`
dan konfirmasi pilihan di dialog. Maksimal 1.000 saran siap diterapkan
diterapkan dalam satu transaksi. Server memeriksa ulang sumber, fingerprint,
jenis, serta akses outlet; perubahan katalog membatalkan seluruh pilihan
dengan respons 409. Pelaku tercatat dari sesi login, bukan kiriman browser.
Perubahan profil menaikkan revisi produk untuk incremental sync. API full pull
dan incremental pull mengirim `menu_kind`; POS lama tidak dapat mengosongkan
jenis Cloud melalui push produk. Penyimpanan/tampilan jenis di Flutter belum
ditambahkan pada tahap Cloud ini.

Perubahan/penghapusan resep juga menerbitkan revisi untuk produk yang jenis
efektifnya berubah, termasuk perpindahan ke resep berikutnya. Profil eksplisit
tetap berlaku. Penguncian mengikuti urutan produk lalu resep; penautan baru
yang bertabrakan membatalkan perubahan dengan 409 agar dapat dicoba kembali.

Analisis tanpa database tersedia melalui CLI yang menerima array JSON
`ProductClassificationInput` dari stdin:

```bash
go run ./cmd/classify-products < katalog.json > analisis.json
go run ./cmd/classify-products -format csv < katalog.json > analisis.csv
```

CLI hanya menghasilkan laporan. CSV/JSON katalog operasional disimpan sebagai
artefak privat di luar Git; penerapan tetap menggunakan pratinjau terbaru dari
API. Pengujian menggunakan database sintetis dan respons API browser sintetis,
tanpa menulis katalog produksi.

```bash
scripts/test-pos-integration.sh --phase backend --offline \
  --artifacts /path/baru/uji-jenis-backend
scripts/test-product-kinds-browser.sh --offline \
  --playwright-modules /path/node_modules \
  --artifacts /path/baru/uji-jenis-browser
```

Runner browser membutuhkan `playwright` dan `playwright-core` versi 1.47.2
serta image `mcr.microsoft.com/playwright:v1.47.2-jammy`. Runner membangun Vue
dan menguji jenis produk, pemilihan resep, serta costing F&B pada container
tanpa jaringan eksternal/port host. Dependensi `npm ci` memakai cache ketika
`--offline`; tanpa opsi itu hanya tahap persiapan dependensi yang berjejaring.

Analisis snapshot katalog pada 31 Oktober 2026 mencakup **883 produk yang belum
dihapus di 11 outlet**; 234 produk yang sudah dihapus tidak disertakan.
Setelah ketentuan ATV Overland dan Lainnya, hasilnya **442 usulan Makanan,
336 Minuman, 11 Aktivitas, dan 94 Lainnya**. Seluruh 883 produk mendapat usulan
jenis; tidak ada yang tersisa untuk ditinjau pada snapshot ini. Keempat produk ATV Overland
seluruhnya mendapat usulan Aktivitas.
Ini merupakan usulan berdasarkan bukti katalog, belum penerapan ke database
produksi. Laporan seluruh baris, alasan, dan ringkasan per outlet tersimpan di
`/home/candra/projects/pos-integration-staging/artifacts/product-kinds-analysis/`
sebagai `analysis-final.csv`, `analysis-final.json`, dan `analysis-summary.json`.
Pengambilan sumber memakai koneksi PostgreSQL read-only; CLI tidak terhubung
ke database. Tidak ada deployment produksi pada tahap ini.

Hasil pengujian akhir jenis produk:

- Build API dan tes Go: **539 lulus, 3 dilewati** dengan alasan eksternal/data
  yang sama seperti pemeriksaan sebelumnya. Tes wajib klasifikasi, akses
  outlet, pelestarian biaya, migrasi ulang, perubahan/penghapusan resep,
  sinkronisasi, serta konkurensi seluruhnya dijalankan dan lulus.
- Build Vue dan seluruh skenario browser jenis produk, pemilihan resep, serta
  costing F&B lulus. Cakupan mencakup 883 baris sintetis, ponsel, akses lihat
  saja, konflik 409, dan batas 1.000 pilihan dalam satu penerapan.
- Sintaks runner dan pemeriksaan whitespace lulus. Peringatan build Vue yang
  tersisa berkaitan dengan ukuran chunk.

Artefak akhir: `product-kinds-backend-final` dan
`product-kinds-browser-deps-fixed` di direktori artefak yang sama. Manifest
backend memastikan 318 berkas sumber Go/modul sama dengan snapshot yang diuji.
Seluruh container dan jaringan uji telah dibersihkan. Katalog produksi dan
container operasional tetap tidak diubah.

Penyesuaian ATV Overland diuji ulang pada database sintetis: build API lulus,
**541 tes Go lulus, 3 dilewati** dengan alasan yang sama. Tes baru mencakup
seluruh produk outlet, batas cakupan nama, penerapan serta incremental sync,
dan pembatalan seluruh pilihan setelah outlet berganti nama. Artefak tersimpan
di `atv-overland-backend`. Laporan sebelumnya dicadangkan dengan nama
`analysis-before-atv-overland.*`; hasil untuk 879 produk outlet lain tetap.
Tidak ada perubahan UI atau deployment produksi pada penyesuaian ini.

Penambahan Lainnya diuji pada database dan respons browser sintetis: build API,
**543 tes Go lulus, 3 dilewati**, build Vue, serta seluruh skenario browser
jenis produk, resep, dan costing F&B lulus. Pengujian mencakup tambah/edit dan
filter Lainnya, penerapan massal dengan batas akses, biaya dan kategori tetap,
migrasi ulang, full/incremental sync, serta pengecualian dari costing F&B.
Artefak berada di `other-kind-backend` dan `other-kind-browser`; sumber Go
akhir cocok dengan 318 berkas pada snapshot uji. Laporan sebelum penyesuaian
tersimpan sebagai `analysis-before-other.*`; 789 jenis sebelumnya tetap,
sedangkan 94 produk sisanya mendapat usulan Lainnya. Seluruh sumber daya uji
dibersihkan dan produksi tidak dideploy.

## Tutup kasir dan laporan per hari

Audit menemukan bahwa laporan tutup kasir memakai rentang sejak shift dibuka,
sehingga shift yang masih terbuka melewati pergantian tanggal menggabungkan
beberapa hari. Menutup aplikasi atau menyelesaikan sesi station tidak menutup
shift laci. Kas fisik juga mencakup modal awal dan mutasi kas, sehingga berbeda
dari penjualan pada hari itu. Transaksi kemarin tidak boleh dihilangkan dari
rekonsiliasi dengan sekadar memotong total shift ke tengah malam.

Perbaikan memisahkan rincian harian dari total seluruh shift. Laporan membawa
rentang waktu yang jelas dan rincian `by_day` menurut tanggal lokal perangkat,
sementara total penutupan dan kas serah-terima tetap mencakup ledger shift.
Dialog menampilkan tanggal lengkap, penanda pergantian hari, dan menunggu
penutupan berhasil. Identitas shift pada dialog tidak boleh berpindah ke shift
baru saat ada serah-terima dari perangkat lain. Kegagalan cetak setelah
penutupan tersimpan tidak dianggap sebagai kegagalan menutup kasir.

Cloud menerima penutupan shift sebagai keadaan akhir. Kiriman buka lama dan
pengulangan penutupan mengembalikan ID yang sama tanpa menimpa waktu, nominal,
atau laporan penutupan yang telah diterima. Data lama dengan status buka tetapi
memiliki `closed_at` tidak ikut dipulihkan, beserta mutasi kas milik shift itu.
Flutter menolak snapshot pemulihan baru yang bertentangan dengan bukti
penutupan, tanpa mengarang tanggal buka atau mengubah shift lokal yang ada.
Shift yang lebih lama atau bertumpang tindih dengan riwayat lokal juga ditolak;
shift berikutnya yang dibuka tepat saat shift sebelumnya ditutup tetap valid.
Tidak ada pembaruan massal riwayat produksi; koreksi nominal historis tetap
memerlukan rekonsiliasi tersendiri.

Validasi pada lingkungan terisolasi dengan data sintetis:

- Build API dan **546 tes Go lulus, 3 dilewati** karena prasyarat eksternal/data
  yang sama dengan pengujian sebelumnya. Ketiga tes integritas penutupan baru
  wajib dijalankan dan semuanya lulus.
- **171 tes Flutter lulus**, termasuk integrasi pesanan, reservasi, pemulihan,
  dan foto produk. Skenario baru mencakup pergantian tanggal dan tengah malam,
  cetak ulang, mutasi kas lama, identitas shift yang berubah, kegagalan simpan,
  printer gagal setelah penutupan tersimpan, serta pemulihan shift lama.
- Analyzer Flutter tidak menemukan error atau warning. Tiga saran gaya baru
  dirapikan dan diverifikasi ulang; 21 info `avoid_print` pada utilitas uji lama
  tetap ada.

Artefak ada di `shift-close-backend`, `shift-close-flutter`, dan
`shift-close-precheck` pada direktori artefak staging. Pemeriksaan ini belum
mencakup APK rilis, pemasangan pada perangkat kasir, atau printer fisik.
Tidak ada deployment ataupun perubahan data produksi.

## Rincian menu pada struk tutup kasir

Struk tutup kasir dan serah-terima menambahkan bagian `MENU TERJUAL`, berisi
nama menu dari pesanan, jumlah unit, subtotal jumlah per jenis, dan total unit.
Urutan jenis selalu **Makanan, Minuman, Aktivitas, Lainnya**. Jenis kosong atau
tidak dikenal masuk Lainnya. Nama panjang dibungkus untuk kertas 58 mm/32 kolom
dan 80 mm/48 kolom. Laporan lama yang belum mempunyai rincian menu tetap bisa
dicetak tanpa menyatakan bahwa jumlah menunya nol.
Karakter di luar encoding satu byte printer ditampilkan sebagai `?` pada nama
menu, agar tidak berubah menjadi byte perintah printer; nama asli tetap tersimpan.

Jumlah dihitung dari item pesanan yang transaksinya selesai dalam rentang
`[opened_at, closed_at)` shift. Pembayaran parsial atau multi-metode tidak
menggandakan jumlah item. Pesanan belum lunas, titipan, dan transaksi batal
tidak dihitung; transaksi selesai yang digratiskan tetap mencatat jumlah menu.
Jumlah menu memakai waktu pelunasan, sedangkan rincian pembayaran memakai
waktu tiap pembayaran. Pada shift lintas tanggal, bagian ini mencakup seluruh
periode shift yang tercetak di kepala struk.

Flutter memakai skema SQLite v16 untuk menyimpan `products.menu_kind` dan
snapshot `order_items.menu_kind`. Jenis mengikuti katalog Cloud; edit produk
lokal tidak menimpa jenis dari Cloud. Setelah upgrade, checkpoint katalog
diulang sekali agar produk yang tidak berubah juga memperoleh jenisnya.
Pesanan baru menyimpan jenis saat item dibuat, termasuk saat item dipisah atau
dipindahkan. Snapshot jenis dipertahankan saat sinkronisasi dan pemulihan.
Item lama tanpa snapshot pada shift terbuka memakai jenis katalog yang
tersedia, lalu dibekukan saat penutupan berhasil. Item pada shift yang sudah
tertutup sebelum fitur ini dan tidak punya snapshot masuk Lainnya; sistem
tidak menebak jenis historis dari nama menu.

Payload laporan Cloud menambahkan `menu_items_by_kind` dan `menu_quantity`.
Tidak ada perubahan perhitungan nominal penjualan atau saldo laci. Pengujian
memakai data sintetis di staging; penerapan jenis katalog ke data produksi,
build APK rilis, pemasangan perangkat, dan deployment tetap langkah terpisah.
Cetak ulang lokal tetap membutuhkan detail transaksi yang masih tersimpan;
setelah detail dihapus oleh retensi, rincian aslinya tersedia pada payload
laporan shift di Cloud, bukan hasil rekonstruksi lokal.

Validasi fitur rincian menu:

- Build API serta **547 tes Go lulus, 3 dilewati** dengan alasan eksternal/data
  yang sama. Tes baru memeriksa empat jenis menu melewati sync dan restore;
  retry penutupan juga mempertahankan rincian menu pada laporan Cloud.
- **203 tes Flutter lulus** pada suite lengkap, termasuk integrasi API nyata
  untuk pemulihan jenis item. Setelah perapian karakter printer, **19 tes cetak
  dan penutupan lulus ulang**, termasuk satu regresi Unicode tambahan.
- Analyzer suite lengkap tidak menemukan error atau warning; 21 info
  `avoid_print` pada utilitas lama tetap ada. Analyzer ulang pada dua berkas
  cetak yang disempurnakan tidak menemukan masalah. Sintaks runner dan whitespace
  bersih. Migrasi SQLite, replay katalog, batas shift, pembayaran terpisah,
  pemecahan/penggabungan item, transaksi gratis/batal, dan kegagalan penutupan
  memakai database sintetis.

Artefak: `shift-menu-backend`, `shift-menu-flutter`, `shift-menu-precheck.log`,
`shift-menu-report-precheck.log`, `shift-menu-final-print.log`, dan
`shift-menu-final-analyze.log` serta `shift-menu-final-verification.json`
di direktori artefak staging. Uji format
memeriksa byte ESC/POS untuk dua lebar kertas; printer fisik dan APK rilis belum
diuji atau dipasang. Tidak ada deployment produksi.

## Rilis produksi — 31 Oktober 2026

**Status terbaru ini menggantikan keterangan “belum dideploy” pada catatan
historis di atas.** Cloud API dan dashboard Vue dari commit `79b10ba` telah
dibangun dan dipasang di [pos.nbp.co.id](https://pos.nbp.co.id) pada **11:17 WIB**.
Homepage, ping API, dan aset frontend melalui HTTPS publik berhasil diperiksa.
Sumber Flutter untuk rilis pendamping adalah commit `2472e19`.

Sebelum pemasangan, database, uploads, dan dokumen dicadangkan serta image
sebelumnya disimpan untuk rollback. Backup database dipulihkan pada lingkungan
Docker terisolasi tanpa koneksi layanan produksi. Image baru berhasil menjalankan
migrasi dan boot dua kali; image lama juga berhasil boot pada salinan database
yang sudah dimigrasikan dan diklasifikasikan. Pada setiap tahap, jumlah baris
dan total nominal ledger keuangan, harga/stok produk, serta kebijakan PPIC tetap
sama. Seluruh sumber daya rehearsal telah dibersihkan. Migrasi produksi kemudian
berhasil membuat tabel integrasi, view jenis produk, dan constraint yang diperlukan.

Klasifikasi **883 produk aktif** diterapkan ke produksi pada **11:19 WIB**:
**442 Makanan, 336 Minuman, 11 Aktivitas, dan 94 Lainnya**. Keempat produk
**ATV Overland** masuk Aktivitas. Pemeriksaan endpoint katalog/sinkronisasi
memverifikasi jenis seluruh **883 produk di 11 outlet**.

Validasi sumber rilis tetap mengacu pada **547 tes Go lulus, 3 dilewati**,
**203 tes Flutter lulus**, serta **19 tes cetak dan penutupan lulus ulang**
yang dijelaskan sebelumnya. Bukti deployment, backup, rehearsal, dan pemeriksaan
sinkronisasi disimpan secara privat di
`/home/candra/projects/pos-integration-staging/releases/20261031-79b10ba/`,
termasuk `release-status.json`, `rehearsal/summary.json`, dan
`production-catalog-sync-check.json`.

APK universal **v10.18** berhasil dibangun dari `2472e19` dan dipublikasikan
pada **11:45 WIB**: [unduh APK](https://pos.nbp.co.id/uploads/apps/app-universal-release-v10.18.apk).
Paket berukuran **64.178.671 byte** mencakup `armeabi-v7a`, `arm64-v8a`, dan
`x86_64`, dengan minimum Android 7.0/API 24. Identitas aplikasi tetap
`com.candradigital.pos_resto`, versionName `1.2.0`, dan versionCode meningkat
dari `88512350` ke `93421153`. Tanda tangan cocok dengan APK sebelumnya;
paket rilis tidak debuggable, integritas ZIP dan alignment 16 KB lulus.
Seluruh 220 berkas sumber terlacak dibandingkan dengan commit rilis; satu-satunya
penyesuaian adalah batas resource Gradle pada salinan build terisolasi.
Unduhan HTTPS publik sudah diverifikasi ukuran, MIME, dan SHA-256:
`0c75448f46fb9df9298195b42bf19cb1f8ca959d57298664dbd59ecb41ebd17d`.

Artefak dan audit APK tersimpan secara privat di
`/home/candra/projects/pos-integration-staging/releases/20261031-2472e19/artifacts/`.
Build pertama melewati batas memori kontainer; pengulangan dengan Kotlin
in-process dan batas heap lebih kecil berhasil tanpa mengganggu produksi.
Pasang APK sebagai pembaruan aplikasi yang ada, lalu sinkronkan katalog Cloud
agar jenis produk tersedia pada tablet. Sesuai pilihan pengguna, distribusi
hanya melalui tautan unduh; pemasangan perangkat dan pengujian printer fisik
belum dilakukan.
