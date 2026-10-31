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

Halaman Produk Cloud menyediakan jenis **Makanan**, **Minuman**, dan
**Aktivitas**, termasuk filter serta penanda pada daftar global dan per outlet.
Jenis produk terpisah dari kategori POS. Form edit yang hanya mengubah nama,
harga, atau kategori tidak mengirim ulang jenis lama, sehingga perubahan jenis
terbaru dari F&B atau analisis tetap dipertahankan.

Jenis memakai profil F&B yang sama, dengan fallback ke resep utama jika belum
ada profil. Migrasi `RunFnbMigrations` memperluas jenis profil dan menyediakan
view `product_menu_kinds`; startup backend gagal bila migrasi ini gagal.
Aktivitas tidak masuk perhitungan costing/dashboard F&B. Pengaturan target HPP,
kemasan, dan overhead yang sudah tersimpan tetap dipertahankan. Klasifikasi
resep serta pengembangan menu F&B tetap Makanan/Minuman.

Tombol **Analisis jenis** membaca seluruh produk yang belum dihapus dalam
cakupan akses pengguna, bukan hanya halaman daftar yang terbuka. Laporan
menampilkan jenis saat ini, saran, alasan, dan produk yang perlu ditinjau.
Analisis menggunakan aturan nama/kategori/resep yang dapat diperiksa; nama
tidak dikenal, barang di luar tiga jenis, serta paket campuran tidak dipaksa
menjadi Aktivitas. Jenis yang sudah ditentukan tidak ditimpa.

Sesuai ketentuan pemilik, analisis outlet **ATV Overland** mengelompokkan semua
produk yang belum memiliki jenis sebagai Aktivitas, termasuk barang pendukung.
Aturan memeriksa nama outlet lengkap (abaikan kapitalisasi/spasi berlebih),
bukan kategori ATV atau potongan nama outlet lain. Jenis eksplisit tetap
dipertahankan sesuai kontrak analisis; aturan ini bukan pembatas form edit.
Perubahan nama outlet setelah pratinjau membatalkan penerapan dengan 409.

Pratinjau memerlukan `products.view`; penerapan memerlukan `products.update`
dan konfirmasi pilihan di dialog. Maksimal 1.000 saran berkeyakinan tinggi
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
Setelah ketentuan ATV Overland, hasilnya **442 usulan Makanan, 336 Minuman,
11 Aktivitas**, serta **94 perlu ditinjau**. Keempat produk ATV Overland
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
