# Penjadwal Penarikan Medsos (Kinerja Markom)

Modul **Laporan → Kinerja Markom** menarik angka Instagram dan TikTok tiap
outlet, lalu memasoknya ke grafik medsos di **Analisa Bisnis**.

## Bukan cron sistem

Penarikannya TIDAK dipasang di crontab server. Ia goroutine di dalam proses
aplikasi — `services/social_scheduler.go`, dinyalakan dari `main.go` lewat
`services.StartSocialScheduler()`.

Alasannya:

- ikut hidup-mati bersama aplikasinya, jadi tidak ada penjadwal yatim yang
  masih menembak setelah aplikasinya dimatikan;
- memakai zona waktu aplikasi (`app_settings.timezone`) dan koneksi database
  yang sama, bukan zona waktu OS yang bisa berbeda;
- pindah server cukup memindahkan compose-nya, tidak ada entri OS yang
  tertinggal.

Crontab server sendiri hanya memuat pekerjaan lain:

```
0 0 * * *  scripts/backup-db.sh
0 1 * * 0  docker builder prune
```

## Kapan ia menyala

Jendela **03:00–05:39 waktu aplikasi**, dan menitnya BERPINDAH tiap hari —
diturunkan dari tanggalnya (`socialMenitTarget`, hash FNV atas string tanggal).

```
12 Sep → 03:59    13 Sep → 03:40    14 Sep → 03:13    15 Sep → 05:34
```

Stabil sepanjang hari itu (restart tidak menggeser jadwal), tetapi berbeda
antar hari. Penarikan yang jatuh pada menit yang sama persis tiap pagi selama
berbulan-bulan adalah tanda tangan yang terlalu rapi — justru keteraturan itu
yang membuat sebuah alamat IP dikenali sebagai robot.

Pemeriksaan tiap 8 menit, berjalan **sekali per hari** (`lastRunDay`).

### Kejar ketinggalan

Tiga menit setelah boot, penjadwal memeriksa apakah masih ada akun terjadwal
yang belum punya potret hari ini. Kalau ada DAN jam target sudah lewat, ia
menarik saat itu juga. Yang ditembak hanya akun yang kurang — restart sore hari
tidak menembaki ulang seluruh daftar.

## Rem yang menjaga alamat IP

Lihat `services/social.go` (`ScrapeSocialAccounts`) dan
`services/social_scraper.go`:

| Rem | Perilaku |
|---|---|
| Jeda antar akun | terjadwal 45–120 dtk; diminta orang 5–13 dtk |
| Urutan akun | `ORDER BY random()` tiap putaran |
| Satu putaran saja | mutex + flag atomik; permintaan bentrok dibalas HTTP 409 |
| Akun yang ditolak | diistirahatkan 6 jam (atau sesuai `Retry-After`) |
| Penolakan beruntun | putaran dihentikan setelah 3 kali, sisanya ditinggal |
| Sudah punya potret | dilewati pada putaran terjadwal |

Tombol **Tarik** di halaman bukan untuk mengumpulkan, melainkan memastikan
tautannya benar: maksimal 5 akun sekali tekan (`socialBatasManual`).

## Kalau angkanya berhenti masuk

1. Buka **Laporan → Kinerja Markom** — kolom status tiap akun menunjukkan
   Lancar / Gagal terakhir / Mandek / Isian manual, lengkap dengan pesan galat
   di tooltip.
2. Periksa log: `docker logs cloud-pos-app-1 | grep '\[Medsos\]'`
3. Uji satu akun tanpa menyentuh produksi:
   ```
   SOCIAL_LIVE_TEST=tiktok:namaakun go test ./services/ -run TestScrapeLive -v
   ```
4. Kalau seluruhnya ditolak, isi `SOCIAL_PROXY_URL` di `.env` lalu
   `docker compose up -d app`. Penarikan akan keluar lewat alamat itu.

## Yang tidak akan pernah didapat

**Jangkauan/impresi Instagram** tidak ada di halaman publik — angka itu hanya
hidup di layar Insights pemegang akun. Satu-satunya sumbernya isian manual
mingguan di halaman Kinerja Markom, dan itu bukan tambalan sementara.

Halaman profil TikTok juga tidak menyerahkan daftar videonya, hanya penghitung
akun. Karena itu konten dan interaksi mingguan diturunkan dari SELISIH
penghitung antar minggu — sah sebagai ukuran aliran, hanya tanpa jumlah tonton.

Akibatnya sumbu jangkauan di Analisa Bisnis memakai dasar "interaksi", dan
dasar yang dipakai selalu disebutkan di baris outletnya.

## Mematikan penarikan untuk satu akun

Kolom `social_accounts.auto_fetch`. Akun yang dimatikan tidak ikut jadwal
harian dan **tidak pernah ditandai mandek** — ia cuma belum diisi. Ini penting:
tanpa pemisahan itu, satu akun yang selalu gagal cukup untuk mengeluarkan
seluruh outletnya dari grafik medsos, termasuk platform lain yang sehat.

Saklarnya ada di modal Tambah/Ubah Akun.
