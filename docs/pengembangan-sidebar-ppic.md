# PPIC — Kontrol Bahan Baku

Diperbarui 27 Oktober 2026. PPIC mengendalikan pemakaian bahan di gudang,
menentukan batas wajar, dan menindaklanjuti selisih untuk mengurangi kebocoran.
F&B mengelola resep; Gudang mencatat transaksi persediaan.

## Menu aktif

| Menu | Rute | Izin |
|---|---|---|
| Kontrol Pemakaian Bahan | `/ppic/usage-control` | `ppic.control.view`, `.update`, `.export` |
| Stock Opname | `/ppic/opname` | `ppic.opname.view`, `.create`, `.approve` |
| Monitor Kedaluwarsa | `/ppic/expiry` | `ppic.expiry.view`, `.ack` |

Dashboard lama, Par Level & ROP, Demand Forecast, MRP, Rencana Produksi,
Work Order, dan Laporan PPIC lama dihapus dari UI, API, serta daftar izin.
Proses forecast otomatis dan peringatan ROP juga dihentikan. Tautan lama
beralih ke halaman PPIC yang masih diizinkan; pengguna tanpa izin modul yang
tersisa mendapat halaman akses ditolak. Perubahan ini tidak memberikan izin
baru kepada role yang hanya memiliki izin lama.

Data transaksi, opname, work order, dan rincian bahan historis tetap disimpan
agar pencatatan stok dan kontrol pemakaian dapat ditelusuri. Helper perhitungan
bersama tetap digunakan oleh costing F&B dan operasi Gudang. Pemulihan
pemotongan stok POS yang gagal tetap berjalan.

## Kontrol pemakaian dan waste

PPIC mengatur dua toleransi terpisah per bahan–gudang: pemakaian dan waste.
Tidak ada nilai bawaan. **Batas belum ditetapkan** berbeda dari **0%**, yang
berarti kebijakan tanpa toleransi. Setiap perubahan wajib mempunyai alasan;
riwayat menyimpan nilai, pengubah, waktu, dan nomor revisi.

- Acuan penjualan memakai catatan bahan transaksi berbayar, bukan resep terbaru.
  Acuan work order historis memakai rincian standar yang tersimpan dan disesuaikan
  dengan hasil aktual. Produksi langsung dan pemotongan tanpa standar pembanding
  independen diberi keterangan.
- Batas pemakaian = acuan × (1 + toleransi pemakaian/100).
  Batas waste = acuan × toleransi waste/100. Tanpa acuan, batas kuantitas dan rasio
  tidak dihitung; hasil tidak otomatis dinyatakan wajar.
- Pemakaian dinilai dari penjualan, pengolahan, pengeluaran lain, serta koreksi
  stok negatif. Kelebihan stok tidak menghapus kekurangan; bahan dan gudang
  diperiksa masing-masing.
- Waste termasuk void yang menjadi waste dan pemusnahan karantina, dihitung sekali.
  Pengeluaran waste tanpa dokumen tetap masuk total dengan penanda bukti belum
  lengkap. Titipan, transfer, dan karantina bukan konsumsi.
- Pemotongan stok POS otomatis bukan bukti penggunaan fisik di dapur. Status
  **Dalam batas tercatat** tetap perlu dibaca bersama hasil opname dan bukti waste.
- Periode mengikuti pencatatan stok/waste dan persetujuan opname. Batas selalu
  memakai kebijakan aktif sekarang, termasuk ketika melihat periode lampau.
- Ekspor Excel mengikuti filter dan cakupan akses, tanpa batas halaman tabel.

Akun bercakupan outlet hanya dapat mengakses kebijakan dan laporan kontrol
untuk gudang outletnya. Kebijakan gudang pusat memerlukan cakupan semua outlet.
Izin kontrol tidak memberikan hak membuat resep atau melakukan mutasi stok.

## Alur harian

1. Buka Kontrol Pemakaian Bahan, pilih gudang/periode, dan lengkapi batas yang kosong.
2. Periksa status **Melebihi batas** dan **Data perlu dilengkapi** serta rinciannya.
3. Cocokkan dengan resep F&B, dokumen sumber, bukti waste, dan hitungan fisik.
4. Lakukan Stock Opname sesuai kewenangan: hitung → review → persetujuan.
   Penyimpanan hitungan belum memposting stok; persetujuan memposting penyesuaian.
5. Tinjau Monitor Kedaluwarsa. Catat penanganan setelah tindakan yang sesuai;
   tanda ditindak tidak otomatis membuang atau memindahkan barang.
6. Perbaiki kesalahan melalui dokumen sumber. Catat alasan bila batas diubah.

## API dan pemeliharaan

Semua rute di bawah `/api/v1/admin/ppic`:

| Rute | Kegunaan |
|---|---|
| `GET /lookups` | Pilihan gudang/bahan untuk ketiga fungsi PPIC |
| `GET /usage-control` | Laporan kontrol dan ringkasan |
| `GET /usage-control/lookups` | Pilihan bahan/gudang untuk kebijakan |
| `PUT /usage-control/policy` | Menyimpan batas dengan pemeriksaan nomor revisi |
| `GET /usage-control/policy/history` | Riwayat batas bahan/gudang |
| `GET /usage-control/export` | Ekspor Excel |
| `GET, POST /opnames` | Daftar dan pembuatan sesi opname |
| `GET /opnames/:id` | Rincian opname |
| `PUT /opnames/:id/items` | Simpan hitungan |
| `POST /opnames/:id/submit, approve, cancel` | Tahap sesi opname |
| `GET /expiry` | Monitor kedaluwarsa |
| `POST /expiry/:batchId/ack` | Catat penanganan batch |

Migrasi fokus PPIC membersihkan izin/tujuan login yang sudah dihapus tanpa
menghapus data histori atau mengaktifkan kembali izin aktif yang pernah dicabut.
Peringatan kedaluwarsa dan pemulihan pemotongan stok POS tetap berjalan; tidak
ada lagi pekerjaan terjadwal untuk forecast maupun ROP.
