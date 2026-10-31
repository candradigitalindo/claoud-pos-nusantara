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

Sinkronisasi foto, pengaturan printer/dapur dari cloud, serta penyedia
pembayaran nyata merupakan pekerjaan lanjutan. Pembaruan DP lokal dilakukan
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
tes Flutter, serta E2E Flutter melawan API uji untuk pesanan online dan reservasi.
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
