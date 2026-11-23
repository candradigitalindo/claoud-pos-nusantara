# Lingkungan Demo & Skema Rilis ke Produksi

| | |
|---|---|
| **Untuk** | Pemilik sistem, pengembang, dan pelatih pengguna |
| **Isi** | Apa itu `demo.nbp.co.id`, cara memakainya untuk latihan dan uji, serta alur resmi menaikkan perubahan dari demo ke produksi |
| **Perkakas** | `deploy/demo.sh` (lingkungan demo) · `deploy/release.sh` (build, promosi, rollback) |
| **Berlaku sejak** | 23 November 2026 |

---

## 1. Dua lingkungan, satu server

| | **Produksi** | **Demo** |
|---|---|---|
| Alamat | https://pos.nbp.co.id | https://demo.nbp.co.id |
| Data | Data asli, terus berjalan | Salinan produksi yang **disanitasi**, diambil saat reset |
| Dipakai untuk | Operasional harian | Latihan pengguna baru · uji fitur sebelum rilis · gladi migrasi |
| Tablet POS | Semua outlet | Hanya tablet latihan (mode *Development*) |
| WhatsApp, medsos, AI, Google Drive, CCTV | Aktif | **Mati** — demo tidak punya akses internet |
| Tanda di layar | — | Bingkai oranye, label **DEMO** di tepi atas, judul tab `[DEMO] …` |
| Boleh rusak? | Tidak | Boleh — tinggal `deploy/demo.sh reset` |

Apa pun yang dikerjakan di demo **tidak pernah menyentuh produksi**: basis data, berkas
unggahan, dokumen, dan rahasianya terpisah. Sebaliknya, data yang dimasukkan di demo
**hilang** setiap kali demo di-reset.

## 2. Cara kerjanya

```
                         Cloudflare (HTTPS)
                                │ HTTP :80
                     ┌──────────▼───────────┐
                     │  nginx PRODUKSI      │  memilih tujuan dari nama domain
                     └───┬──────────────┬───┘
       Host: pos.nbp.co.id              │ Host: demo.nbp.co.id
                         │              │ jaringan docker "nbp-edge" (internal)
            ┌────────────▼───┐    ┌─────▼──────────────────────────────────────┐
            │ app produksi   │    │ web demo ─▶ app demo ─▶ db demo            │
            │ db, wa, ...    │    │            analytics demo                  │
            └────────────────┘    │ jaringan "internal": TANPA akses internet  │
                                  └────────────────────────────────────────────┘
                                    proyek compose: cloud-pos-demo
```

Jaminan isolasi:

- **Tanpa internet.** Container demo hanya tersambung ke jaringan docker `internal`.
  Demo secara fisik tidak bisa mengirim WhatsApp, menarik Instagram/TikTok, memanggil AI,
  mengunggah ke Google Drive, atau menjangkau kamera — walau pengaturannya menyala.
- **Rahasia sendiri.** JWT, webhook, token admin, dan kunci kamera demo dibuat acak saat
  pemasangan. Token login produksi tidak berlaku di demo dan sebaliknya.
- **Tidak ada port terbuka.** Satu-satunya pintu masuk demo adalah nginx produksi.
- **Dibatasi sumber dayanya.** Memori & CPU tiap container demo dipatok (total ±2 GB
  maksimal), supaya demo tidak pernah merebut sumber daya produksi.
- **Gagal dengan aman.** Bila demo mati, `demo.nbp.co.id` menampilkan halaman
  "Lingkungan demo sedang tidak aktif"; produksi tidak terpengaruh.

## 3. Apa yang berbeda di data demo

Setiap reset menyalin basis data produksi lalu menjalankan `deploy/demo/sanitize.sql`:

| Data | Di demo |
|---|---|
| Kunci API outlet | Diganti semua (`pos_demo_…`). Kunci produksi tidak berlaku di demo; kunci demo tidak berlaku di produksi |
| Sesi WhatsApp, antrean & riwayat pesan WA | Tidak disalin sama sekali |
| Pengaturan WhatsApp | Dimatikan; tautan di pesan menunjuk ke demo; nomor WA staf dikosongkan |
| Nomor HP pelanggan | Disamarkan (`08000…`), konsisten antar tabel — nomor yang sama tetap sama |
| Akun medsos | Penarikan otomatis dimatikan |
| CCTV | Kredensial, alamat kamera, dan token penghubung dihapus |
| Antrean cadangan foto ke Google Drive | Ditutup |
| Berkas APK (`uploads/apps`) | Tidak disalin — tablet latihan tidak pernah ditawari pembaruan dari demo |
| Sesi login | Semua gugur; login ulang |
| Akun, role, menu, transaksi, laporan, dokumen, foto | Sama dengan produksi saat salinan diambil |

## 4. Memakai demo untuk latihan

**Login.** Akun dan sandi di demo sama dengan produksi *pada saat salinan terakhir diambil*,
dengan hak akses yang sama. Untuk peserta yang belum punya akun produksi, buat **akun
latihan** yang otomatis dibuat ulang setiap reset:

1. Salin `deploy/demo/training-users.example.csv` ke
   `../pos-integration-staging/demo/training-users.csv`.
2. Isi `username,nama,role,password` (role harus ada di halaman Role; nama role maksimal
   20 karakter — lihat catatan di §8).
3. Terapkan sekarang: `deploy/demo.sh training-users` (berikutnya ikut otomatis saat reset).

Berkas CSV itu berisi sandi: biarkan di folder keadaan demo (izin 600), jangan di repo.

**Tablet POS latihan.** Di App POS: Pengaturan → Cloud → lingkungan **Development**,
URL `https://demo.nbp.co.id`, lalu isi Outlet ID dan kunci dari `deploy/demo.sh outlet-keys`.
Transaksi latihan masuk ke demo, tidak ke produksi. Kembalikan tablet ke **Production**
setelah selesai, dan jangan memakai tablet operasional untuk latihan.

**Yang tidak berfungsi di demo:** kirim WhatsApp, tarik medsos, narasi AI Analisa Bisnis
(memakai kalimat templat), cadangan foto ke Drive, CCTV, pembaruan APK, dan QRIS. Untuk
latihan alur QRIS, aktifkan simulasi di `demo.env` (`DEMO_PAYMENT_GATEWAY_PROVIDER=mock`,
`DEMO_PAYMENT_GATEWAY_ALLOW_MOCK=true`) lalu `deploy/demo.sh start`.

**Ingat:** data demo bisa dilihat siapa pun yang punya akun. Beri akun latihan role yang
sesuai tugasnya, sama seperti di produksi.

## 5. Skema rilis: dari kode sampai produksi

```
  ┌────────────┐   ┌──────────────────────┐   ┌──────────────┐   ┌─────────────┐
  │ 1. BUILD   │──▶│ 2. DEMO + data segar │──▶│ 3. UJI (UAT) │──▶│ 4. PROMOSI  │
  │ image rilis│   │ migrasi digladi di   │   │ di demo.nbp  │   │ image SAMA  │
  │ sekali     │   │ salinan produksi     │   │ .co.id       │   │ ke produksi │
  └────────────┘   └──────────────────────┘   └──────┬───────┘   └──────┬──────┘
                                                     │ gagal            │ gagal sehat
                                                     ▼                  ▼
                                              perbaiki → build    rollback otomatis
                                              ulang (rilis baru)  ke image lama
```

Aturan:

1. **Image yang naik ke produksi adalah image yang sama persis dengan yang diuji di demo.**
   Tidak ada build ulang di antara keduanya.
2. **Commit dulu.** Rilis dari perubahan yang belum di-commit ditolak promosi (kecuali
   `--allow-dirty`), supaya kode produksi selalu bisa dilacak ke satu commit.
3. **Uji dengan data segar** (`--reset`), supaya migrasi basis data sudah pernah dijalankan
   pada salinan produksi terbaru sebelum menyentuh produksi.
4. **Produksi tidak pernah tanpa jaring pengaman:** setiap promosi membuat cadangan basis
   data dan menandai image lama; bila produksi tidak sehat, image lama dinaikkan kembali
   otomatis.

### Langkah demi langkah

```bash
# 1. Build (± 5 menit). Bawaan: isi working tree; atau --ref <commit>.
deploy/release.sh build rekap-darurat
#    → rilis 20261124-091500-rekap-darurat, image cloud-pos-{app,nginx}:rekap-darurat-20261124-091500

# 2. Jalankan di demo dengan data produksi segar (± 1–2 menit)
deploy/demo.sh deploy 20261124-091500-rekap-darurat --reset

# 3. Uji di https://demo.nbp.co.id (daftar periksa di bawah)

# 4. Naikkan ke produksi — diminta mengetik nama rilis sebagai konfirmasi
deploy/release.sh promote 20261124-091500-rekap-darurat

# Bila ada masalah setelahnya
deploy/release.sh rollback 20261124-091500-rekap-darurat

# Posisi semua rilis (mana yang di demo, mana yang di produksi)
deploy/release.sh list
```

`promote` mengerjakan: cadangan `pg_dump` produksi ke folder rilis → tag image lama
`before-<rilis>` → ganti container `app` + `nginx` (gangguan beberapa detik; tablet
menyinkron ulang otomatis) → cek kesehatan (ping, tanpa restart, tanpa panic, `index.html`
baru) → tag `latest` ke image rilis → catat `deployment.json`. Gagal cek kesehatan =
otomatis kembali ke image lama.

`rollback` hanya mengembalikan image; basis data tidak dikembalikan karena migrasi di
aplikasi ini aditif. Perintah pemulihan basis data dicetak bila memang dibutuhkan.

### Daftar periksa uji di demo

- [ ] `deploy/demo.sh deploy … --reset` selesai **sehat** (tanpa panic, tanpa restart).
- [ ] Login dengan beberapa role yang terdampak perubahan (mis. pengaju, verifikator,
      keuangan) — menu dan tombol sesuai izin.
- [ ] Alur yang diubah dicoba dari awal sampai akhir dengan data nyata salinan.
- [ ] Laporan yang terdampak (L/R, arus kas, neraca, laporan penjualan) masuk akal
      dibanding produksi pada tanggal salinan.
- [ ] Bila menyentuh sinkron POS: tablet latihan dicoba transaksi, tutup kasir, sinkron.
- [ ] Tampilan ponsel (lebar ± 390 px) untuk halaman yang diubah.
- [ ] `deploy/demo.sh logs app` bersih dari galat baru.

### Jalur darurat (hotfix)

Untuk perbaikan kritis yang tidak bisa menunggu: tetap `build` → `deploy … --reset` →
uji singkat → `promote`. Bila demo sendiri bermasalah, `promote … --skip-demo` tersedia,
dan `--allow-dirty` untuk perubahan yang belum di-commit — keduanya tercatat di
`deployment.json` dan wajib di-commit/diuji ulang sesudahnya.

## 6. Pemasangan & perawatan (admin server)

**Pemasangan pertama** (sudah dilakukan 23 Nov 2026):

```bash
deploy/demo.sh init        # rahasia, jaringan nbp-edge, rilis dasar = image produksi, data, nyalakan
deploy/demo.sh route-on    # sambungkan demo.nbp.co.id di nginx produksi (reload, tanpa downtime)
```

Lalu di Cloudflare → DNS zona `nbp.co.id`: tambah **CNAME `demo` → `pos.nbp.co.id`,
Proxied** (awan oranye). SSL ikut pengaturan zona yang sudah ada. **Urutan penting:**
sebelum `route-on`, nginx produksi menjawab nama domain apa pun sebagai produksi, jadi
DNS yang dibuat lebih dulu akan membuat `demo.nbp.co.id` menampilkan PRODUKSI. Periksa
dengan `deploy/demo.sh route-check`.

Blok `demo.nbp.co.id` tersimpan di `nginx/default.http-only.conf`, jadi image nginx
rilis berikutnya membawanya secara permanen; `route-on` hanya memasangnya ke container
yang sedang berjalan.

**Lokasi berkas** (di luar repo, karena berisi rahasia & cadangan):

| Jalur | Isi |
|---|---|
| `../pos-integration-staging/demo/demo.env` | Rahasia demo (izin 600) |
| `../pos-integration-staging/demo/state.env` | Rilis yang sedang berjalan di demo |
| `../pos-integration-staging/demo/status.json` | Waktu salinan data & deploy terakhir |
| `../pos-integration-staging/demo/snapshots/` | 2 salinan `pg_dump` produksi terakhir |
| `../pos-integration-staging/demo/training-users.csv` | Akun latihan (opsional) |
| `../pos-integration-staging/releases/<rilis>/` | `build.json`, `demo.json`, `deployment.json`, `compose.release.json`, `compose.rollback.json`, `db-before.dump`, log |

**Reset otomatis mingguan (opsional)** — mis. Senin 04.00 WIB, sebelum jam kerja:

```
0 4 * * 1  cd /home/candra/projects/cloud-pos && deploy/demo.sh reset >> ../pos-integration-staging/demo/reset.log 2>&1
```

Reset juga mengganti kunci API outlet demo — tablet latihan perlu kunci baru sesudahnya.

**Perawatan disk.** Snapshot demo dibatasi 2 terakhir. Setiap promosi menyimpan cadangan
basis data (± 25 MB) di folder rilisnya; hapus folder & image rilis lama secara berkala
(sisakan rilis produksi dan satu `before-…` terbaru).

**Server baru / pemulihan bencana.** `docker-compose.yml` produksi memakai jaringan
eksternal `nbp-edge`; buat sekali sebelum `docker compose up`:
`docker network create --internal nbp-edge`.

**Mematikan demo:** `deploy/demo.sh stop` (data tetap; domain menampilkan halaman
"demo tidak aktif"). **Menghapus total:** `deploy/demo.sh destroy`.

## 7. Ringkasan perintah

| Perintah | Fungsi |
|---|---|
| `deploy/demo.sh status` | Rilis demo, umur data, kesehatan, status domain |
| `deploy/demo.sh reset` | Segarkan data demo dari produksi (± 1 menit) |
| `deploy/demo.sh deploy <rilis> [--reset]` | Jalankan rilis tertentu di demo |
| `deploy/demo.sh training-users` | Terapkan akun latihan dari CSV |
| `deploy/demo.sh outlet-keys` | Outlet ID & kunci API demo untuk tablet latihan |
| `deploy/demo.sh logs [app\|web\|db]` | Log container demo |
| `deploy/demo.sh route-check` | Uji jalur `pos.` dan `demo.nbp.co.id` |
| `deploy/demo.sh start` / `stop` | Nyalakan / matikan demo |
| `deploy/release.sh build <nama> [--ref <commit>]` | Bangun rilis kandidat |
| `deploy/release.sh list` / `show <rilis>` | Daftar & rincian rilis |
| `deploy/release.sh promote <rilis>` | Naikkan ke produksi |
| `deploy/release.sh rollback <rilis>` | Kembalikan produksi ke image sebelumnya |

## 8. Catatan

- Layanan `wa` (gateway WhatsApp) dan `analytics` produksi tidak ikut skema rilis ini
  (jarang berubah); perbarui seperti biasa dengan `docker compose build <layanan>` lalu
  `docker compose up -d <layanan>`.
- Temuan saat uji pertama di demo (23 Nov 2026): kolom `cloud_admins.role` hanya
  `varchar(20)`, sehingga 7 role bernama panjang (mis. "verifikator pengadaan",
  "penyetuju barang, jasa & proyek") tidak bisa diberikan ke akun mana pun
  (`value too long for type character varying(20)`).
