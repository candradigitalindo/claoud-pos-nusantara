# Petugas Perlengkapan — Uraian Tugas & Indikator Kinerja

| | |
|---|---|
| **Posisi** | Petugas Perlengkapan (Asset Officer) |
| **Bagian** | Perlengkapan & Aset |
| **Melapor kepada** | Manajer Area / Manajer Pusat |
| **Berhubungan erat dengan** | Tim Purchasing, Gudang Induk, Kepala Outlet, PIC pengaju, Teknisi & vendor, Keuangan |
| **Alat kerja** | Cloud POS — menu **Perlengkapan** (Dashboard Aset, Daftar Aset, Penerimaan Peralatan, Distribusi ke PIC, Mutasi Antar Outlet, Perawatan, Opname Aset, Penghapusan, Laporan Aset, Kategori Aset) |
| **Berlaku sejak** | 16 September 2026 |

---

## 1. Ringkasan Peran

Petugas Perlengkapan bertanggung jawab agar **setiap barang tidak habis pakai milik perusahaan
punya wujud data yang benar** — sejak barangnya diterima dari tim purchasing, dipakai
sehari-hari, dirawat, dipindahkan, sampai akhirnya dihapus.

Ukuran keberhasilan peran ini sederhana: **apa yang tercatat di sistem sama dengan apa yang ada
di lapangan**, dan setiap perpindahan barang bisa dibuktikan tanpa mengandalkan ingatan orang.

Peran ini **tidak mengurus barang dapur**. Bahan baku, kemasan, dan barang habis pakai yang
masuk katalog stok adalah urusan Gudang Induk. Batas ini tegas dan sudah dijaga sistem:
pengajuan pengadaan tidak boleh mencampur barang dapur dan peralatan.

---

## 2. Tanggung Jawab Utama

### 2.1 Menerima peralatan dari tim purchasing

Membuka **Penerimaan Peralatan**, mencocokkan fisik barang dengan rincian pengajuan, lalu
mencatat penerimaannya.

- **Wajib memotret barang saat diterima.** Sistem menolak penerimaan tanpa foto. Foto diambil
  di tempat serah terima, bukan diunggah dari galeri.
- Menentukan tujuan tiap baris: **Aset** (dicatat bernomor), **Material projek**, atau
  **Habis pakai**. Barang bernilai ≥ Rp 1.000.000 yang ditandai habis pakai wajib beralasan.
- Boleh menerima barang yang **belum dibayar** (pembelian tempo) — status tagihannya tidak
  berubah, dan itu memang benar.
- Menerima sebagian bila barang datang bertahap; sisanya tetap tercatat menunggu.

### 2.2 Mendata aset dengan lengkap

Setiap aset yang lahir mendapat nomor otomatis (`AST-…`). Tugas petugas adalah melengkapi yang
tidak bisa ditebak sistem:

- Kategori (dipilih dari **Kategori Aset**, bukan diketik bebas), merk, tipe, **nomor seri**
- Lokasi penempatan dan **penanggung jawab**
- Tanggal & harga perolehan, masa garansi
- Mencetak dan menempelkan **label QR** pada barangnya

Aset mahal atau bernomor seri dicatat sebagai **tunggal** (satu baris satu unit); barang
seragam berjumlah banyak seperti kursi dicatat sebagai **massal**.

### 2.3 Mendistribusikan ke PIC pengaju

Barang tidak berhenti di bagian aset. Lewat **Distribusi ke PIC**, petugas menyerahkan aset
kepada orang yang mengajukan pembeliannya.

- **Wajib berfoto saat serah terima.**
- Penanggung jawab dan lokasi aset otomatis berpindah ke penerima.
- Daftar "Menunggu Diserahkan" adalah antrean pekerjaan harian — idealnya kosong.

### 2.4 Mengurus mutasi antar outlet

Membuat dokumen **Mutasi Antar Outlet** (`MTA-…`) saat aset pindah tempat: ajukan → (disetujui
manajer) → kirim → diterima outlet tujuan.

- Memeriksa kondisi barang saat kirim dan saat terima; selisih dicatat apa adanya.
- Mencetak **Berita Acara Serah Terima** untuk ditandatangani kedua pihak.
- Aset yang sedang diperbaiki tidak boleh dimutasi, kecuali memang dikirim ke bengkel.

### 2.5 Menjalankan perawatan

Halaman **Perawatan** memegang jadwal dan riwayat seluruh aset.

- Mengerjakan daftar **jatuh tempo** sebelum lewat tanggal: mulai → selesaikan work order
  (`WOM-…`), isi biaya, pelaksana, lama berhenti, dan kondisi setelah dikerjakan.
- Jadwal berikutnya terbit otomatis dari interval kategori; petugas menyesuaikan bila perlu.
- Bila perbaikan memerlukan vendor berbayar, **mengajukan Pengadaan Jasa langsung dari work
  order** agar biayanya tidak diketik dua kali di dua modul.
- Mengusulkan penggantian bila biaya perawatan sebuah aset sudah melewati **50%** harga
  belinya — memperbaiki terus barang seperti itu lebih mahal daripada menggantinya.

### 2.6 Opname aset (audit fisik)

Minimal **dua kali setahun per outlet**, atau sewaktu-waktu diminta manajemen.

- Membuka sesi opname (`OPA-…`), menghitung fisik, mencatat kondisi yang ditemukan.
- Barang yang ada di lapangan tapi tak terdata dicatat sebagai temuan → menjadi aset baru.
- Barang yang tidak ditemukan **tidak langsung dihapus**, melainkan menjadi usulan penghapusan
  yang menunggu persetujuan.

### 2.7 Penghapusan aset

Mengajukan **Penghapusan** (`DSP-…`) untuk barang yang dijual, dimusnahkan, dihibahkan, atau
hilang — lengkap dengan alasan dan (bila dijual) hasil penjualannya. Nilai bukunya dibekukan di
dokumen agar berita acara tetap cocok meski penyusutan terus berjalan.

> Menghapus data lewat tombol Hapus di Daftar Aset **hanya untuk salah input**. Barang yang
> benar-benar sudah tidak ada harus lewat dokumen penghapusan, supaya nilainya tercatat dan
> laporan aset tidak menyusut tanpa sebab.

### 2.8 Material projek

Untuk belanja projek renovasi/pembangunan: mencatat penerimaan material (semen, cat, keramik),
mencatat pemakaiannya, dan **memastikan sisanya didata menjadi aset** saat projek ditutup.

### 2.9 Menjaga kebersihan data & laporan

- Memeriksa **Dashboard Aset** setiap pagi dan mengerjakan kartu "Perlu Ditindak".
- Menindaklanjuti laporan **Pengadaan Belum Lengkap** — selama ada baris di sana, ada barang
  yang sudah dibayar tapi belum jelas keberadaannya.
- Memastikan seluruh bukti foto berhasil tercadangkan (status pada dokumen dan di Pengaturan).
- Menyiapkan laporan aset bulanan untuk manajemen.

---

## 3. Irama Kerja

| Frekuensi | Yang dikerjakan |
|---|---|
| **Harian** | Buka Dashboard Aset; kerjakan penerimaan yang masuk antrean; serahkan aset ke PIC; mulai/selesaikan work order yang jatuh tempo; terima mutasi masuk |
| **Mingguan** | Kosongkan antrean "Menunggu Diserahkan"; periksa laporan Pengadaan Belum Lengkap; pastikan tidak ada foto gagal tercadangkan; lengkapi data aset yang belum penuh |
| **Bulanan** | Tutup laporan biaya perawatan; usulkan penggantian untuk aset yang melewati ambang 50%; periksa aset tanpa penanggung jawab; laporan aset ke manajemen |
| **Semesteran** | Opname aset per outlet; rekonsiliasi selisih; usulkan penghapusan untuk barang hilang/rusak berat |
| **Insidental** | Mutasi antar outlet; perbaikan darurat; penerimaan material projek; penutupan material saat projek selesai |

---

## 4. Batas Wewenang

| Boleh diputuskan sendiri | Perlu persetujuan atasan |
|---|---|
| Mencatat penerimaan barang dan membuat aset | **Menyetujui** mutasi antar outlet |
| Menyerahkan aset ke PIC pengaju | **Menyetujui** penghapusan aset |
| Membuat & menyelesaikan work order perawatan | **Menerapkan** selisih opname |
| Mengajukan mutasi, penghapusan, dan opname | Mengubah ambang kebijakan (nilai kapitalisasi, umur ekonomis, interval perawatan) |
| Melengkapi data aset, lokasi, penanggung jawab | Mengubah data outlet atau harga perolehan aset yang sudah ditetapkan |

Pemisahan ini juga ditegakkan sistem lewat hak akses: petugas memegang izin **membuat dan
menjalankan**, atasan memegang izin **menyetujui**. Seorang petugas tidak bisa mengirim sekaligus
menerima mutasinya sendiri.

---

## 5. Indikator Kinerja (KPI)

Semua angka di bawah bisa dibaca langsung dari sistem — tidak ada yang perlu dihitung manual.

### 5.1 KPI Utama

| # | Indikator | Rumus | Target | Sumber data | Periode |
|---|---|---|---|---|---|
| 1 | **Kepatuhan jadwal perawatan** | WO preventif selesai tepat waktu ÷ WO preventif jatuh tempo | **≥ 90%** | Perawatan → tab Riwayat & Jatuh Tempo | Bulanan |
| 2 | **Perawatan terlambat** | Jumlah WO berstatus dijadwalkan yang lewat tanggal | **0** | Dashboard Aset → kartu "Perawatan terlambat" | Mingguan |
| 3 | **Akurasi data aset (IRA)** | 1 − (baris selisih ÷ baris diperiksa saat opname) | **≥ 98%** | Opname Aset → akurasi sesi | Per opname |
| 4 | **Kelengkapan data aset** | Aset yang punya nomor, tgl & harga perolehan, lokasi, dan penanggung jawab ÷ total aset | **≥ 95%** | Daftar Aset & Laporan Daftar Aset | Bulanan |
| 5 | **Ketepatan pencatatan penerimaan** | Baris pengadaan diterima yang sudah berwujud data ÷ seluruh baris diterima | **100%** dalam 1 hari kerja | Laporan "Pengadaan Belum Lengkap" | Mingguan |
| 6 | **Kecepatan distribusi ke PIC** | Rata-rata hari dari aset diterima sampai diserahkan ke PIC | **≤ 2 hari kerja** | Distribusi ke PIC → "Menunggu Diserahkan" | Mingguan |
| 7 | **Kelengkapan bukti foto** | Foto penerimaan & distribusi yang berhasil tercadangkan ÷ seluruh foto | **100%** | Pengaturan → Cadangan Bukti Foto | Mingguan |
| 8 | **Selisih mutasi** | Jumlah dokumen mutasi yang diterima lebih sedikit dari yang dikirim | **0** | Mutasi Antar Outlet → penanda "Selisih" | Bulanan |

### 5.2 KPI Pendukung

| # | Indikator | Rumus | Target | Periode |
|---|---|---|---|---|
| 9 | **Rasio preventif : korektif** | Jumlah WO rutin+inspeksi : perbaikan+penggantian | **≥ 70 : 30** | Triwulan |
| 10 | **Aset melewati ambang ganti** | Aset dengan biaya rawat kumulatif > 50% harga beli **yang belum diusulkan diganti** | **0** | Bulanan |
| 11 | **Ketepatan waktu opname** | Outlet yang diopname sesuai jadwal ÷ seluruh outlet | **100%** | Semester |
| 12 | **Penyelesaian usulan penghapusan** | Usulan diproses ≤ 7 hari sejak diajukan | **≥ 90%** | Bulanan |
| 13 | **Sisa material projek tertangani** | Projek selesai tanpa sisa material yang belum ditentukan nasibnya | **100%** | Per projek |
| 14 | **Aset tanpa penanggung jawab** | Jumlah aset aktif yang `pic_name`-nya kosong | **0** | Bulanan |

### 5.3 Cara Menilai

- **Sangat baik** — seluruh KPI Utama tercapai, tidak ada temuan berulang pada opname.
- **Baik** — maksimal satu KPI Utama meleset tipis, disertai penjelasan dan tindak lanjut.
- **Perlu perbaikan** — dua KPI Utama atau lebih meleset, atau ada barang hilang yang tidak
  bisa dijelaskan.

Penilaian dilakukan **bulanan** oleh atasan langsung, memakai angka apa adanya dari sistem.

---

## 6. Yang Tidak Boleh Terjadi

Hal-hal berikut dianggap kegagalan serius, bukan sekadar KPI yang meleset:

1. **Menerima barang tanpa memotretnya**, atau memotret sesuatu yang bukan barangnya.
2. **Menyerahkan aset tanpa dokumen serah terima** — barang berpindah tangan tanpa penanggung
   jawab yang jelas.
3. **Mencatat penerimaan barang yang belum datang**, dengan alasan apa pun.
4. **Menghapus data aset yang barangnya benar-benar hilang** lewat tombol Hapus, bukan lewat
   dokumen penghapusan — ini menghilangkan nilai dari laporan tanpa jejak.
5. **Membiarkan hasil opname tidak diterapkan**, sehingga catatan dan lapangan terus berbeda.
6. **Menandai barang mahal sebagai "habis pakai"** untuk menghindari pencatatan aset.

---

## 7. Kebutuhan Akses Sistem

Role **Petugas Perlengkapan** minimal memegang hak akses berikut (diatur di menu Role):

| Hak akses | Untuk |
|---|---|
| `assets.dashboard.view` | Membuka Dashboard Aset |
| `assets.view`, `assets.create`, `assets.update` | Mendata dan melengkapi aset, mengelola kategori |
| `assets.transfer.view`, `.create`, `.receive` | Mengajukan, mengirim, dan menerima mutasi |
| `assets.maintenance.view`, `.create` | Menjadwalkan dan menyelesaikan perawatan |
| `assets.opname.view`, `.create` | Menghitung opname |
| `assets.disposal.view`, `.create` | Mengajukan penghapusan |
| `assets.report.view` | Membuka laporan aset |
| `procurement.requests.submit` | Menyelesaikan serah terima pengadaan |

**Sengaja tidak diberikan** kepada petugas: `assets.transfer.approve`, `assets.disposal.approve`,
`assets.opname.approve`, dan `assets.delete`. Keempatnya milik atasan — yang menjalankan dan yang
menyetujui tidak boleh orang yang sama.

Petugas yang juga mengurus material projek memerlukan tambahan `procurement.projects.view` dan
`procurement.projects.manage`.

---

## 8. Perlengkapan Kerja

- Ponsel berkamera dengan akses Cloud POS (dipakai memotret di lapangan dan menghitung opname)
- Printer label untuk stiker QR aset
- Akses ke seluruh area outlet dan gudang yang menjadi tanggung jawabnya

---

*Dokumen ini mengikuti alur yang berjalan di Cloud POS modul Perlengkapan. Bila alur sistem
berubah, uraian tugas dan KPI di sini ikut ditinjau. Rincian teknis alurnya ada di
`docs/perlengkapan-aset.md`.*
