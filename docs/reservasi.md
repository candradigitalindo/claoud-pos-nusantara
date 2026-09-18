# Reservasi & Uang Muka — Ketentuan dan Cara Kerja

| | |
|---|---|
| **Untuk** | Admin/kasir yang mengelola reservasi, keuangan, dan tim app POS |
| **Berlaku sejak** | 19 September 2026 |
| **Dokumen terkait** | Laporan keuangan: `docs/perlengkapan-aset.md` §12b · Sumber kode: `services/reservation.go`, `services/reservation_payment.go` |

---

## 1. Prinsip

**Uang muka (DP) adalah kewajiban kepada pelanggan, bukan pendapatan.** Pendapatan tetap diakui
saat transaksi POS terjadi pada hari kunjungan, sesuai kebijakan penjualan yang berlaku. Karena
itu:

- Kolom **DP yang diminta** pada reservasi hanyalah angka yang ditagihkan.
- Uang yang benar-benar masuk dicatat sebagai **dokumen pembayaran** (`reservation_payments`)
  berjenis DP, pelunasan, atau refund — dengan nominal, metode, rekening tujuan, bukti, tanggal,
  dan siapa yang memvalidasi.
- **Status reservasi mengikuti pembayaran**, bukan dropdown bebas.

```
 pending ──DP tervalidasi (otomatis)──▶ confirmed ──transaksi POS / admin──▶ done
    │                                       │
    └──────────── cancelled ────────────────┘   wajib pilih: refund | hangus (bila sudah ada uang masuk)
```

`done` dan `cancelled` bersifat final. Reservasi yang sudah punya pembayaran tervalidasi
**tidak bisa dihapus**, hanya dibatalkan.

## 2. Alur pelanggan (halaman publik `/r/<slug>`)

1. Pelanggan mengisi data dan memilih menu. DP yang diminta dihitung otomatis dari kebijakan
   **persen DP** (pengaturan di halaman Reservasi, bawaan 50%).
2. Setelah kirim, halaman menampilkan **tautan cek status** (`/r/<slug>?id=…`), rekening tujuan
   (dari master Rekening Bank yang aktif), dan formulir **unggah bukti transfer**.
3. Bukti dari pelanggan lahir berstatus **menunggu validasi**. Nominalnya tidak boleh melebihi
   sisa yang belum dibayar.
4. Admin memvalidasi atau menolak (alasan wajib, dibaca pelanggan di tautan status).
5. Begitu DP yang diminta terpenuhi oleh pembayaran tervalidasi, reservasi **otomatis
   Dikonfirmasi**.

## 3. Alur admin (menu Reservasi)

- **Catat Pembayaran**: untuk uang yang admin lihat sendiri (transfer masuk di rekening, tunai
  di kasir). Langsung tervalidasi atas nama admin. Bukti wajib untuk transfer/QRIS.
- **Validasi / Tolak** bukti kiriman pelanggan.
- **Konfirmasi** manual hanya untuk reservasi tanpa DP; yang ber-DP terkonfirmasi otomatis.
- **Batalkan**: bila sudah ada uang masuk, pilih **refund** (lalu catat refundnya sebagai
  pembayaran berjenis refund, dengan bukti) atau **hangus** (menjadi Pendapatan Lainnya).
- **Selesai**: normalnya ditutup otomatis oleh transaksi POS; tombol manual disediakan untuk
  keadaan darurat.

Reservasi selesai/batal dikunci: data dan menunya tidak bisa diubah.

## 4. App POS (yang perlu dibangun di sisi Flutter)

Endpoint sudah tersedia dengan auth API key outlet:

| Endpoint | Guna |
|---|---|
| `GET /api/v1/outlets/:id/reservations?date=YYYY-MM-DD` | Reservasi hari itu (pending/confirmed) beserta item, total, dibayar, sisa |
| `POST /api/v1/outlets/:id/reservations/:rid/settle` `{transaction_local_id}` | Menutup reservasi dari transaksi tertentu |
| `reservation_id` pada payload transaksi (`POST …/transactions`, batch sync) | Menutup otomatis saat transaksi tersinkron |

Aturan pemakaian uang muka di POS: buka order dari reservasi, lalu kirim uang mukanya sebagai
**satu baris `payments[]` bermetode `reservasi_dp`** sebesar `paid_amount`, sisanya dibayar
dengan metode apa pun. Transaksi tetap dikirim **utuh** (total penuh) — itulah pendapatan hari
kunjungan. Server mengecualikan baris `reservasi_dp` dari penerimaan kas hari itu karena uangnya
sudah masuk saat DP.

Reservasi tidak bisa ditutup selagi masih ada bukti pembayaran yang menunggu validasi.

## 5. Yang terjadi di laporan keuangan

| Peristiwa | Arus Kas | Laba/Rugi | Neraca | Buku Besar |
|---|---|---|---|---|
| DP/pelunasan tervalidasi | Penerimaan **Uang Muka Reservasi** (tanggal bayar) | — | Kas ↑, **Uang Muka Pelanggan** ↑ | Dr Kas / Cr Uang Muka Pelanggan (2-300) |
| Refund tervalidasi | Pengeluaran **Refund Uang Muka** | — | Kas ↓, Uang Muka ↓ | Dr Uang Muka / Cr Kas |
| Dibatalkan, **hangus** | — | **Pendapatan Lainnya** ↑ (tanggal batal) | Uang Muka ↓ | Dr Uang Muka / Cr Pendapatan Lainnya |
| Transaksi POS dengan baris `reservasi_dp` | Penjualan dikurangi bagian `reservasi_dp` | Pendapatan penuh (seperti transaksi biasa) | Uang Muka ↓ | Dr Uang Muka / Cr Kas (mengimbangi posting penjualan) |

Uang Muka Pelanggan di Neraca = seluruh pembayaran tervalidasi (dp + pelunasan − refund) atas
reservasi berstatus pending/confirmed, ditambah reservasi batal-refund yang refundnya belum
dicatat.

## 6. Hak akses

| Izin | Untuk |
|---|---|
| `reservations.view` | Melihat reservasi, riwayat pembayaran, kebijakan DP |
| `reservations.create` | Membuat reservasi |
| `reservations.update` | Mencatat/validasi/menolak pembayaran, mengubah status, mengubah persen DP, mengunggah bukti |
| `reservations.delete` | Menghapus reservasi yang belum punya pembayaran tervalidasi |

Halaman publik tidak memerlukan login; dibatasi 30 permintaan per menit per IP, bukti hanya
gambar maksimal 5MB.
