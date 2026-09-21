# WhatsApp — Notifikasi Otomatis & Broadcast

| | |
|---|---|
| **Untuk** | Pemilik/admin yang mengelola notifikasi, dan siapa pun yang merawat server |
| **Berlaku sejak** | 21 September 2026 |
| **Sumber kode** | `wa-gateway/` (layanan gateway), `services/whatsapp*.go`, `handlers/whatsapp.go`, `ui/src/pages/whatsapp/` |

---

## 1. Apa ini dan apa batasnya

Cloud POS bisa mengirim pesan WhatsApp **tanpa biaya per pesan dan tanpa akun WhatsApp Business API**.
Caranya: satu nomor WhatsApp biasa dipasangkan lewat QR — persis seperti WhatsApp Web — ke layanan
kecil bernama `wa-gateway` yang berjalan di VPS ini (pustaka [whatsmeow](https://github.com/tulir/whatsmeow)).

Konsekuensinya harus dipahami sejak awal:

- **Ini jalur tidak resmi.** WhatsApp bisa memblokir nomor yang berperilaku seperti spammer
  (kirim beruntun ke banyak nomor asing, banyak yang membalas "laporkan"). Blokir tidak bisa
  diajukan banding. Karena itu:
  - Pakai **nomor khusus toko**, bukan nomor pribadi pemilik.
  - Pakai nomor yang sudah "hangat" (pernah dipakai chat normal beberapa minggu), bukan kartu baru.
  - Biarkan **jeda antar pesan** (bawaan 4–9 detik acak), **batas harian** (bawaan 400), dan
    **jam tenang** untuk pelanggan (bawaan 21:30–07:00) tetap hidup.
  - Broadcast ke pelanggan sebaiknya hanya ke yang pernah bertransaksi (memang begitu sumbernya) dan
    tidak setiap hari.
- Gateway **satu arah**: pesan masuk tidak diproses (hanya tercatat di log container).
- Bila pustaka whatsmeow kedaluwarsa (WhatsApp mengubah protokol), status gateway menjadi
  *Bermasalah: versi pustaka kedaluwarsa* — solusinya membangun ulang image `wa` dengan versi terbaru
  (`go get go.mau.fi/whatsmeow@latest` di folder `wa-gateway/`).

## 2. Arsitektur

```
alur bisnis (reservasi, tutup kasir, pengadaan, PPIC, aset, heartbeat)
        │  notifyWA(event, …)            ── services/whatsapp_events.go
        ▼
   wa_messages (antrean, status pending)  ── Postgres
        │  satu pekerja, satu per satu, jeda acak, jam tenang, batas harian
        ▼
   wa-gateway  ─── HTTP internal (token) ───▶  WhatsApp
   (container `wa`, whatsmeow, sesi di tabel whatsmeow_* pada DB yang sama)
```

- Pesan **tidak pernah** dikirim langsung dari alur bisnis. Semua masuk antrean dulu; kalau gateway
  sedang mati, antrean menunggu. Notifikasi yang menunggu lebih dari 24 jam dibatalkan otomatis
  (informasi basi tidak dikirim).
- Satu notifikasi per (event, dokumen, tujuan): retry sync dari app POS atau cascade status tidak
  menggandakan pesan.
- Sesi WhatsApp tersimpan di database → ikut **backup harian** dan bertahan saat container dibuat ulang.

## 3. Menyiapkan pertama kali

1. `docker compose build wa app nginx && docker compose up -d wa app nginx`.
   Token internal `WA_GATEWAY_TOKEN` sudah ada di `.env` (dibuat acak saat fitur ini dipasang).
2. Buka menu **WhatsApp → Koneksi & Notifikasi**, tab **Koneksi**, klik **Tampilkan QR**.
3. Di HP dengan nomor toko: WhatsApp → ⋮ → **Perangkat tertaut** → **Tautkan perangkat** → pindai.
4. Setelah status *Tersambung*, kirim **pesan uji** ke nomor sendiri.
5. Isi **nomor WhatsApp tiap pengguna** di menu **Pengguna → Ubah** (lihat §3a). Inilah penerima
   notifikasi internal; tab **Penerima** memperlihatkan siapa akan menerima event apa.
6. Tab **Notifikasi & Template**: matikan event yang tidak diinginkan, ubah template bila perlu,
   dan bila aturan otomatis tidak cocok, timpa **Posisi penerima** per event.

### 3a. Penerima internal mengikuti POSISI, bukan daftar bebas

Notifikasi internal **tidak** dikirim ke semua orang. Sebuah pengguna menerima sebuah event hanya bila:

1. **Role-nya berkaitan** dengan event itu. Aturan otomatisnya memakai hak akses role:

   | Event | Role yang otomatis menerima |
   |---|---|
   | Reservasi baru / bukti masuk / terkonfirmasi / batal | yang punya akses Reservasi |
   | Laporan tutup kasir | yang punya akses Laporan Shift Kasir |
   | Rekap penjualan harian | yang punya akses Laporan Penjualan |
   | Pengajuan pengadaan baru | yang punya izin Persetujuan Pengadaan |
   | Pengajuan disetujui / dibayar | Purchasing + **pengaju** (dicocokkan nama akun) |
   | Pengajuan ditolak | **pengaju** saja |
   | Permintaan pembayaran | yang punya izin Pencairan Pembayaran (keuangan) |
   | Peringatan stok | Dashboard PPIC / Kedaluwarsa / Dashboard Gudang |
   | WO perawatan aset | Perawatan Aset |
   | Perangkat kasir offline | Monitoring Perangkat |

   Superadmin menerima semua event internal. Aturan ini bisa **ditimpa per event** di tab
   Notifikasi → *Posisi penerima* (centang role; daftar yang dicentang menggantikan aturan otomatis).
2. **Outletnya cocok.** Role dengan *Batasan Unit Kerja* "specific" hanya menerima event outletnya
   sendiri (manajer Sekar Ayu tidak menerima tutup kasir Kala). Event yang tidak terkait outlet
   (peringatan stok gudang, pengadaan pusat) tetap dikirim.
3. Akun aktif, nomor WhatsApp terisi, dan opsi *Terima notifikasi WhatsApp* menyala (menu Pengguna → Ubah).

**Penerima tambahan** (tab Penerima, bagian bawah) dipakai untuk grup WhatsApp manajemen atau
nomor yang bukan akun pengguna. Untuk grup, nomor gateway harus anggota grup; pilih lewat *Muat grup*.
Penerima tambahan pun sebaiknya dibatasi event dan outletnya, bukan "semua".

## 4. Notifikasi yang tersedia

| Event | Ke siapa | Dipicu oleh |
|---|---|---|
| Reservasi baru | internal | reservasi dari halaman publik / admin |
| Bukti pembayaran masuk | internal | pelanggan unggah bukti (perlu validasi) |
| Reservasi terkonfirmasi | internal + pelanggan | DP tervalidasi (otomatis/manual) |
| Reservasi dibatalkan | internal + pelanggan | admin membatalkan |
| Ke pelanggan: reservasi diterima | pelanggan | berisi DP, rekening aktif, tautan status |
| Ke pelanggan: bukti ditolak | pelanggan | admin menolak bukti, dengan alasan |
| Ke pelanggan: pengingat H-1 | pelanggan | terjadwal, jam pengingat (bawaan 10:00), reservasi terkonfirmasi |
| Laporan tutup kasir | internal | shift ditutup di app POS & tersinkron |
| Rekap penjualan harian | internal | terjadwal, jam rekap (bawaan 22:00); penerima berfilter outlet hanya melihat outletnya |
| Pengajuan baru / disetujui / ditolak / minta bayar / dibayar | internal | alur pengadaan |
| Peringatan stok (kedaluwarsa & ROP) | internal | evaluasi PPIC harian (~02:00) |
| WO perawatan aset terbit | internal | penjadwal perawatan (~03:00) |
| Perangkat kasir tidak melapor | internal | heartbeat berhenti > ambang menit (satu per outlet per hari) |

Tautan status reservasi memakai **Alamat publik** di pengaturan (bawaan `https://pos.nbp.co.id`).
Notifikasi ke pelanggan hanya terkirim bila nomor HP pada reservasi valid.

## 5. Broadcast

Menu **WhatsApp → Broadcast**. Audiens:

- **Pelanggan**: master pelanggan (dari order kasir yang punya nomor HP), disaring outlet, minimal
  kunjungan, "datang dalam N hari", atau "tidak datang ≥ N hari" (kampanye ajak kembali). Dibatasi
  scope outlet admin yang membuatnya.
- **Penerima internal**: semua penerima aktif.
- **Daftar manual**: tempel nomor, satu per baris, boleh `nomor, nama`.

`{name}` di pesan diganti nama penerima. Bisa menyertakan gambar (URL JPEG/PNG ≤ 6 MB) dan dijadwalkan.
Klik **Hitung penerima** dulu untuk melihat jumlah dan perkiraan durasi. Broadcast yang berjalan bisa
dibatalkan (sisa yang belum terkirim).

## 6. Log & pemulihan

- **WhatsApp → Log Pesan**: semua pesan dengan status *Menunggu / Terkirim / Gagal / Dibatalkan*,
  alasan gagal, dan tombol *Ulangi* / *Batalkan*. Gagal permanen (nomor tidak terdaftar di WhatsApp)
  tidak diulang otomatis; gagal sementara diulang 2× (3 dan 15 menit).
- Nomor **keluar dari Perangkat Tertaut** di HP → status *Belum dipasang*; pindai QR lagi. Antrean tetap aman.
- Container `wa` mati → status *Gateway mati*; antrean menunggu. `docker compose logs wa` untuk melihat sebabnya.
- Ganti nomor: tab Koneksi → **Lepas nomor (logout)** → pindai QR dengan nomor baru.

## 7. Hak akses

`whatsapp.view` (lihat status & log), `whatsapp.manage` (koneksi, penerima, pengaturan, template,
ulangi/batalkan pesan), `whatsapp.broadcast` (kirim broadcast). Saat fitur dipasang, ketiganya
diberikan ke role admin & superadmin; role lain diatur di halaman Role. Mengisi nomor WhatsApp
pengguna memakai izin pengguna (`users.update`), bukan izin WhatsApp.
