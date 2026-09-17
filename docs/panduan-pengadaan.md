# Ketentuan & Panduan Penggunaan — Modul Pengadaan

| | |
|---|---|
| **Untuk** | Seluruh peran yang terlibat dalam pengadaan barang dan jasa |
| **Isi** | Alur resmi, ketentuan tiap tahap, dan langkah pemakaian aplikasinya |
| **Dokumen terkait** | Penerimaan aset & distribusi: `docs/panduan-perlengkapan.md` · Rancangan modul aset: `docs/perlengkapan-aset.md` |
| **Berlaku sejak** | 17 September 2026 |

---

## 1. Alur Resmi Pengadaan

```
  ┌────────────┐   ┌──────────┐   ┌────────────┐   ┌─────────────┐   ┌──────────────┐
  │ PENGAJUAN  │──▶│ APPROVE  │──▶│ PEMBELIAN  │──▶│ PENERIMAAN  │──▶│  DISTRIBUSI  │
  └────────────┘   └──────────┘   └────────────┘   └─────────────┘   └──────────────┘
   Manager Outlet   Admin / GM /    Tim              Logistik /        ke Outlet /
   / Head Store     Direktur        Purchasing       Asset Officer     PIC (serah terima)

   HPS + sumber     Pending          Isi harga        Barang dapur →    Gudang → gudang
   HPS wajib        Setujui          final & vendor,  Gudang Induk      outlet
   terisi           Tolak + alasan   ajukan bayar     Peralatan →       Aset → PIC
                                                      Bagian Aset       pengaju
```

**Satu pengajuan tidak boleh mencampur barang dapur dan peralatan.** Serah terimanya berada di
dua tempat berbeda, dan sistem menolak pengajuan campuran sejak dibuat.

---

## 2. Tahap 1 — Pengajuan

**Siapa:** Manager Outlet / Head Store (izin `procurement.requests.submit`)
**Menu:** Pengadaan → **Pengadaan Barang** atau **Pengadaan Jasa** → **+ Buat Pengajuan**

### Ketentuan

1. Satu pengajuan berisi satu atau beberapa **Nama Pengadaan**, masing-masing berisi rincian item.
2. **Setiap item wajib punya HPS (Harga Perkiraan Sendiri) dan Sumber HPS.** HPS tanpa sumber
   adalah angka yang tidak bisa diperiksa siapa pun.
3. Pengajuan yang terkait projek renovasi/pembangunan **wajib memilih projeknya**, agar
   belanjanya masuk ke serapan RAB projek itu.
4. Barang dapur dan peralatan **diajukan terpisah**.

### Sumber HPS — apa yang ditulis

| Bentuk | Contoh |
|---|---|
| Harga toko lewat telepon | `Toko Jaya Abadi, telp 0812-3456-7890 (17 Sep)` |
| Tautan marketplace | `https://shopee.co.id/Kompor-Gas-4-Tungku-i.123.456789` |
| Penawaran vendor | `Penawaran CV Sinar Abadi No. 042/IX/2026` |
| Harga pembelian sebelumnya | `Pembelian terakhir 12 Jul 2026, PR 120726003` |

Tautan cukup ditempel apa adanya — di layar akan tampil sebagai nama situsnya saja
(`shopee.co.id…`) dan bisa diklik oleh penyetuju.

> **Hindari:** "perkiraan", "kira-kira segitu", atau dikosongkan. Penyetuju tidak punya cara
> memeriksanya, dan pengajuan berisiko dikembalikan.

### Langkah

1. Pilih unit kerja/outlet pengaju dan (bila ada) projeknya.
2. Isi **Nama Pengadaan**, misalnya "Peralatan Dapur Outlet Pusat".
3. Untuk tiap item: nama, qty, satuan, **HPS satuan**, lalu **Sumber HPS** pada baris di bawahnya.
4. Tambah item atau tambah Nama Pengadaan lain bila perlu.
5. Periksa total HPS di bagian bawah, lalu **Simpan**.

Status pengajuan menjadi **Pending**.

---

## 3. Tahap 2 — Approve

**Siapa:** Admin / GM / Direktur (izin `procurement.requests.approve`)
**Menu:** Pengadaan → Pengadaan Barang/Jasa → buka pengajuan berstatus Pending

### Tiga keputusan

| Keputusan | Akibatnya | Ketentuan |
|---|---|---|
| **Pending** | Dibiarkan; pengajuan tetap menunggu | Dipakai bila masih perlu keterangan dari pengaju |
| **Setujui** | Status menjadi Disetujui, lanjut ke Purchasing | Pastikan HPS wajar dan sumbernya bisa diperiksa |
| **Tolak** | Status menjadi Ditolak, alur berhenti | **Alasan wajib diisi** — ditolak sistem bila kosong |

### Yang diperiksa sebelum menyetujui

1. **Kebutuhannya nyata** — bukan barang yang sudah ada di outlet lain dan bisa dimutasi.
2. **HPS masuk akal** dan **sumbernya bisa dibuka/dihubungi**.
3. **Jumlahnya wajar** terhadap kebutuhan operasional.
4. Untuk belanja projek: **RAB projek masih cukup** (terlihat di halaman Projek).

### Ketentuan penolakan

Alasan penolakan **wajib**, dan dijaga di sisi server — bukan sekadar peringatan di layar.
Tulis alasan yang bisa ditindaklanjuti:

| Kurang baik | Lebih baik |
|---|---|
| "Tidak disetujui" | "HPS di atas harga pasar; lampirkan pembanding dari dua toko" |
| "Belum perlu" | "Outlet Dua punya unit menganggur — ajukan mutasi, bukan pembelian" |
| "Data kurang" | "Sumber HPS kosong pada item Rak Stainless" |

Pengajuan yang ditolak **tidak bisa dilanjutkan**. Pengaju membuat pengajuan baru setelah
memperbaiki.

---

## 4. Tahap 3 — Pembelian

**Siapa:** Tim Purchasing (izin `procurement.requests.purchasing`), pembayaran oleh Keuangan
(izin `finance.payments.view`)

### 4.1 Mengisi harga final & vendor

1. Buka pengajuan berstatus **Disetujui** → **Isi Harga (Purchasing)**.
2. Isi **harga final** tiap item — harga yang benar-benar disepakati vendor.
3. Isi **vendor** dan **nomor invoice**.
4. Simpan. Selisih HPS dan harga final terlihat langsung di layar.

**Bila satu pengajuan dibeli dari beberapa vendor**, pakai **Pecah per Vendor**: item dipindahkan
ke dokumen anak dengan vendornya masing-masing. Pembayaran lalu dilakukan per vendor.

### 4.2 Mengajukan pembayaran

Setelah harga final terisi, tekan **Ajukan Pembayaran**. Status menjadi **Menunggu Pembayaran**
dan muncul di menu **Pembayaran**.

### 4.3 Pembayaran

Bagian Keuangan membuka **Pembayaran**, memilih rekening sumber & tujuan, mengunggah bukti
transfer, lalu menyimpan. Pembayaran boleh **sebagian** — sisa tagihannya tetap tercatat.

Di layar Pembayaran, tiap baris menampilkan **dua keadaan sekaligus**: status pembayaran dan
penanda *Barang sudah/belum diterima*. Membayar barang yang belum datang adalah keputusan yang
berbeda dengan melunasi barang yang sudah ada di outlet — keduanya sah, tapi harus disadari.

---

## 5. Tahap 4 — Penerimaan

**Siapa:** Logistik/Gudang Induk (barang dapur) atau Asset Officer (peralatan)
**Menu:** Gudang → **Penerimaan dari Pengadaan**, atau Perlengkapan → **Penerimaan Peralatan**

### Ketentuan

1. **Barang boleh diterima sebelum maupun sesudah dibayar.** Pembelian tempo tetap dicatat pada
   hari barangnya datang.
2. **Foto barang saat diterima wajib.** Sistem menolak penerimaan tanpa foto.
3. Barang yang datang bertahap dicatat **sebagian**, sisanya tetap muncul di antrean.
4. Status pengajuan menjadi **Diterima** hanya setelah **seluruh** barisnya diterima **dan**
   pembayarannya lunas.

### Langkah singkat

1. Buka antrean sesuai bagian Anda, klik **Terima Barang**.
2. Ambil foto barangnya.
3. Isi jumlah yang benar-benar datang, tentukan tujuan tiap baris (aset / material projek /
   stok gudang / habis pakai).
4. Simpan.

Rincian per tujuan ada di `docs/panduan-perlengkapan.md` bagian 2.

---

## 6. Tahap 5 — Distribusi (Serah Terima)

Barang tidak berhenti di meja penerimaan.

| Jenis barang | Diteruskan ke | Caranya |
|---|---|---|
| **Barang dapur** | Gudang outlet | Gudang → Transfer Stok, kirim ke gudang outlet (**wajib berfoto**) |
| **Peralatan/aset** | **PIC yang mengajukan pembelian** | Perlengkapan → Distribusi ke PIC (**wajib berfoto**) |
| **Material projek** | Dipakai di lokasi projek | Dicatat pemakaiannya di halaman Projek |

Serah terima aset memindahkan **penanggung jawab** barang ke penerimanya. Sejak saat itu, PIC
tersebut yang bertanggung jawab atas fisiknya.

---

## 7. Ketentuan Umum

1. **Setiap perpindahan barang berfoto.** Dua momen wajib: saat diterima dari purchasing, dan
   saat diserahkan ke tujuan akhirnya. Foto otomatis dicadangkan ke Google Drive perusahaan.
2. **Yang mengajukan bukan yang menyetujui.** Pemisahan ini ditegakkan hak akses.
3. **Angka tidak diketik dua kali.** Perbaikan aset yang memerlukan vendor diajukan langsung dari
   work order perawatan, sehingga biayanya cukup diisi sekali.
4. **Dokumen tidak dihapus untuk menutupi kesalahan.** Pengajuan yang keliru dibatalkan
   (statusnya tercatat), bukan dihapus.
5. **Pengajuan yang sudah dibayar tidak bisa diubah itemnya.** Perbaikan hanya mungkin sebelum
   pembayaran diajukan.

---

## 8. Tabel Status

| Status | Artinya | Siapa yang bergerak berikutnya |
|---|---|---|
| **Pending** | Menunggu keputusan | Admin / GM / Direktur |
| **Disetujui** | Boleh dibelanjakan | Tim Purchasing |
| **Menunggu Pembayaran** | Harga final terisi, menunggu dibayar | Keuangan |
| **Dibayar Sebagian** | Sebagian tagihan lunas | Keuangan (sisa) |
| **Dibayar** | Tagihan lunas | Penerimaan |
| **Diterima** | Seluruh barang diterima **dan** lunas | Distribusi |
| **Ditolak** | Ditolak beserta alasannya | Pengaju (ajukan ulang bila masih perlu) |
| **Dibatalkan** | Dibatalkan sebelum dibayar | — |

Penanda **Barang sudah/belum diterima** berdiri sendiri dari status di atas, karena barang bisa
datang sebelum maupun sesudah dibayar.

---

## 9. Hak Akses per Peran

| Peran | Hak akses |
|---|---|
| **Manager Outlet / Head Store** | `procurement.requests.view`, `procurement.requests.submit` |
| **Admin / GM / Direktur** | ditambah `procurement.requests.approve` |
| **Tim Purchasing** | `procurement.requests.view`, `procurement.requests.purchasing` |
| **Keuangan** | `finance.payments.view` |
| **Logistik / Gudang** | `stockledger.adjust`, `stocktransfers.*` |
| **Asset Officer** | `assets.create`, `assets.update`, `procurement.requests.submit` |
| **PIC Projek** | `procurement.projects.view`, `procurement.projects.manage` |

Bila sebuah tombol tidak muncul, hak aksesnya belum diberikan — minta admin membukanya lewat
menu **Role**.

---

## 10. Pertanyaan yang Sering Muncul

**Pengajuan ditolak, apakah bisa diperbaiki lalu diajukan lagi?**
Dokumen yang ditolak berhenti di situ agar jejak keputusannya utuh. Buat pengajuan baru setelah
memperbaiki hal yang disebut pada alasan penolakan.

**Barang sudah datang tapi belum dibayar — dicatat sekarang atau nanti?**
Catat sekarang, pada hari barangnya datang. Status tagihan tidak berubah; yang berubah hanya
penanda barang diterima.

**Satu pengajuan berisi kompor dan beras, kenapa ditolak?**
Karena serah terimanya di dua tempat. Pisahkan menjadi dua pengajuan.

**HPS meleset jauh dari harga final, apakah masalah?**
Bukan pelanggaran, tapi menjadi bahan evaluasi. Sumber HPS yang jelas membuat selisih itu bisa
dijelaskan — misalnya harga naik sejak penawaran diambil.

**Bagaimana bila barang datang tidak lengkap?**
Terima sebagian sesuai yang datang. Sisanya tetap muncul di antrean penerimaan sampai lengkap,
dan terpantau di laporan "Pengadaan Belum Lengkap".

---

*Panduan ini mengikuti alur yang berjalan di aplikasi per 17 September 2026. Bila alurnya
berubah, dokumen ini ikut ditinjau.*
