# Panduan Penggunaan — Modul Perlengkapan

| | |
|---|---|
| **Untuk** | Petugas Perlengkapan dan siapa pun yang membuka menu Perlengkapan |
| **Isi** | Langkah demi langkah tiap pekerjaan, aturan yang ditolak sistem, dan letak angka KPI |
| **Pasangan dokumen** | Tanggung jawab & penilaian: `docs/jobdesk-petugas-perlengkapan.md` |
| **Diperbarui** | 16 September 2026 |

---

## Peta Menu

Seluruh pekerjaan ada di grup **Perlengkapan** di sisi kiri:

| Menu | Dipakai untuk |
|---|---|
| **Dashboard Aset** | Melihat nilai aset dan daftar pekerjaan yang menunggu |
| **Daftar Aset** | Mencari, melengkapi data, mencetak label, dan **impor massal lewat Excel** |
| **Penerimaan Peralatan** | Menerima barang dari tim purchasing |
| **Distribusi ke PIC** | Menyerahkan aset ke orang yang mengajukan pembeliannya |
| **Mutasi Antar Outlet** | Memindahkan aset ke outlet lain |
| **Perawatan** | Jadwal dan riwayat perawatan seluruh aset |
| **Opname Aset** | Menghitung fisik aset per outlet |
| **Penghapusan** | Mengeluarkan aset yang dijual, rusak, atau hilang |
| **Laporan Aset** | Lima laporan + unduh Excel |
| **Kategori Aset** | Daftar kategori beserta umur ekonomis dan interval perawatannya |

Nomor dokumen yang akan Anda temui: `AST-…` (aset), `MTA-…` (mutasi), `WOM-…` (perawatan),
`OPA-…` (opname), `DSP-…` (penghapusan), `SRT-…` (serah terima ke PIC).

---

## 1. Memulai Hari — Dashboard Aset

1. Buka **Perlengkapan → Dashboard Aset**.
2. Lihat baris **Perlu Ditindak**. Kartu berwarna kuning berarti ada pekerjaan; klik kartunya
   untuk langsung menuju halamannya.
3. Kerjakan berurutan: perawatan terlambat → mutasi menunggu diterima → aset belum diserahkan.

Kartu **Biaya Rawat Tertinggi** di bagian bawah menandai aset yang biayanya sudah melewati 50%
harga belinya. Aset seperti itu diusulkan diganti, bukan diperbaiki lagi.

---

## 2. Menerima Barang dari Purchasing

**Siapkan dulu:** barangnya ada di depan Anda, dan ponsel siap memotret.

1. Buka **Penerimaan Peralatan**. Daftar berisi pengadaan yang menunggu diterima, lengkap dengan
   keterangan pembayarannya.
2. Klik **Terima Barang** pada pengajuan yang barangnya datang.
3. **Ambil foto barangnya** di kotak paling atas. Tekan tombol foto — kamera belakang ponsel
   langsung terbuka.
4. Periksa tiap baris. Isi **jumlah** yang benar-benar datang (boleh lebih sedikit dari yang
   dibeli bila datang bertahap).
5. Pilih **tujuan** tiap baris:
   - **Aset** — barang tidak habis pakai. Isi kategori, mode pencatatan, dan lokasi.
   - **Material projek** — hanya muncul bila pengajuannya terikat projek.
   - **Habis pakai** — hanya untuk barang yang langsung habis. Bernilai ≥ Rp 1.000.000 wajib
     diberi alasan.
6. Klik **Catat Penerimaan**.

**Mode pencatatan aset:**

| Mode | Dipakai untuk | Akibatnya |
|---|---|---|
| **Tunggal** | Barang mahal atau bernomor seri (AC, kulkas, mesin kopi) | Tiap unit dapat nomor sendiri; qty 3 menghasilkan 3 baris aset |
| **Massal** | Barang seragam berjumlah banyak (kursi, gelas) | Satu baris berisi banyak unit |

**Yang akan ditolak sistem:**

| Pesan | Artinya |
|---|---|
| "foto barang saat diterima wajib diunggah" | Belum memotret. Foto adalah satu-satunya bukti barang benar-benar sampai |
| "jumlah melebihi sisa yang belum dicatat" | Baris itu sudah pernah dicatat sebelumnya — periksa dulu riwayatnya |
| "kategori … tidak terdaftar" | Kategori harus dipilih dari daftar; tambahkan dulu di menu Kategori Aset |
| "pengajuan berstatus … belum bisa diserahterimakan" | Pengajuannya belum disetujui |

**Barang belum dibayar boleh diterima.** Kolom pembayaran di daftar antrean menunjukkan
keadaannya; status tagihan tidak berubah sampai keuangan membayar.

**Barang dapur bukan urusan Anda.** Bila di dialog muncul kelompok "Barang dapur / gudang",
biarkan — itu antrean Gudang Induk.

---

## 3. Melengkapi Data Aset & Mencetak Label

1. Buka **Daftar Aset**, klik nama asetnya untuk membuka detail, atau klik **Edit** untuk
   mengubah.
2. Lengkapi: kategori, merk, tipe, **nomor seri**, lokasi, **penanggung jawab**, tanggal & harga
   perolehan, garansi.
3. Umur ekonomis terisi otomatis dari kategorinya; ubah hanya bila memang berbeda.
4. Mencetak label: centang aset yang akan dicetak di daftar, lalu pilih **Cetak Label 50×25**
   atau **70×40**. Label berisi nomor aset dan QR yang menuju halaman detailnya.

> Bila jendela cetak tidak muncul, izinkan pop-up untuk situs ini di browser.

Halaman detail aset menampilkan **nilai buku berjalan**, riwayat perawatan, dan riwayat
perpindahan — termasuk siapa yang terakhir memegangnya.

---

## 3b. Mendata Banyak Aset Sekaligus lewat Excel

Untuk pendataan awal atau aset lama yang jumlahnya ratusan, mengetik satu per satu lewat form
terlalu lama. Pakai jalur Excel.

**Langkah:**

1. Buka **Daftar Aset** → tombol **Impor Excel**.
2. Klik **Unduh Template**. Templatenya berisi empat lembar:
   - **Data Aset** — tempat Anda mengetik, hanya berisi judul kolom
   - **Panduan** — arti tiap kolom dan aturan pengisiannya
   - **Referensi** — daftar kategori dan kode outlet yang berlaku
   - **Contoh Pengisian** — dua contoh baris, sengaja dipisah agar tidak ikut terkirim
3. Isi lembar **Data Aset** mulai baris ke-2. Kolom Kode Outlet, Kategori, Mode, dan Kondisi
   punya daftar pilihan — klik selnya lalu pilih.
4. Simpan sebagai `.xlsx`, kembali ke aplikasi, klik **Pilih Berkas**.
5. Sistem **memeriksa dulu** dan menampilkan tiga angka: baris terbaca, siap disimpan, dan perlu
   diperbaiki. Baris bermasalah disebut **nomor barisnya** beserta alasannya.
6. Perbaiki baris yang bermasalah di Excel lalu unggah ulang, atau langsung tekan
   **Simpan N Baris** untuk menyimpan yang sudah benar.

**Yang perlu diketahui:**

| Hal | Perilakunya |
|---|---|
| Baris bermasalah | Tidak menggagalkan berkas. Yang benar tetap tersimpan, yang salah dilaporkan |
| Mode **Tunggal** dengan jumlah 3 | Menghasilkan **3 aset terpisah**, masing-masing bernomor sendiri |
| Mode **Massal** dengan jumlah 40 | Menghasilkan **1 baris aset** berisi 40 unit |
| Harga `Rp 35.000.000` atau `450.000` | Diterima; pemisah ribuan dan awalan Rp dibersihkan sendiri |
| Tanggal `15/02/2026` | Diterima, sama dengan `2026-02-15` |
| Umur ekonomis kosong | Diisi otomatis dari kategorinya |
| Nomor aset | **Tidak perlu diisi** — dibuat sistem |
| Nomor seri kembar dalam satu berkas | Ditolak, disebut baris berapa kembarannya |

**Pesan yang sering muncul:**

| Pesan | Perbaikannya |
|---|---|
| "kode outlet … tidak dikenal atau di luar akses Anda" | Salin persis dari sheet Referensi |
| "kategori … tidak terdaftar" | Tambahkan dulu di menu Kategori Aset, atau kosongkan |
| "jumlah harus angka bulat lebih dari 0" | Periksa sel yang kosong atau berisi teks |
| "format tanggal tidak dikenali" | Tulis `YYYY-MM-DD` |
| "kolom … tidak ditemukan" | Judul kolom terhapus/berubah — unduh template baru |

> Jangan mengubah judul kolom di baris 1. Sistem mencocokkan kolom lewat judulnya, bukan
> posisinya — jadi menambah kolom bantu sendiri di sebelah kanan tidak masalah, tetapi
> mengganti judul akan membuat kolomnya tidak terbaca.

---

## 4. Menyerahkan Aset ke PIC

1. Buka **Distribusi ke PIC**. Bagian atas berisi **Menunggu Diserahkan** — aset yang sudah
   diterima tapi belum ada penanggung jawabnya.
2. Klik **+ Serahkan Aset**.
3. Isi nama PIC penerima, jabatan, dan lokasi penempatan.
4. Centang aset yang diserahkan.
5. **Ambil foto serah terima.**
6. Klik **Serahkan**.

Setelah itu penanggung jawab dan lokasi aset berpindah otomatis, dan riwayat asetnya mencatat
"Diserahkan ke …".

---

## 5. Memindahkan Aset ke Outlet Lain

1. Buka **Mutasi Antar Outlet** → **+ Buat Mutasi**.
2. Pilih outlet asal dan tujuan, lalu alasan pemindahannya.
3. Centang aset yang dipindah dan isi jumlahnya (untuk aset massal bisa sebagian).
4. Simpan sebagai draft, lalu **Ajukan**.
5. Setelah disetujui atasan, tekan **Kirim Barang** dan lampirkan fotonya.
6. Petugas outlet tujuan membuka dokumen yang sama dan menekan **Terima Barang**, mengisi jumlah
   yang benar-benar sampai.
7. Cetak **Berita Acara Serah Terima** dari dokumen yang sudah selesai.

**Yang perlu diketahui:**

- Aset yang sedang dikerjakan teknisi tidak bisa dimutasi, kecuali alasannya "Dikirim untuk
  diperbaiki".
- Aset yang sudah tercantum di dokumen mutasi lain tidak akan muncul sebagai pilihan.
- Bila yang diterima lebih sedikit dari yang dikirim, sisanya otomatis kembali ke outlet asal
  dan dokumen ditandai **Selisih** — isi apa adanya, jangan dibulatkan.

---

## 6. Perawatan

### 6.1 Mengerjakan perawatan terjadwal

1. Buka **Perawatan**. Tab **Terlambat** dan **Jatuh Tempo 7 Hari** adalah antrean kerja.
2. Klik pekerjaannya → **Mulai Kerjakan**. Aset akan ditandai sedang diperbaiki dan tidak bisa
   dimutasi selama itu.
3. Setelah selesai, isi **Penutupan Pekerjaan**: tanggal, biaya, pelaksana, lama berhenti, dan
   kondisi setelah dikerjakan.
4. Klik **Selesai**. Jadwal berikutnya terbit otomatis sesuai interval kategorinya.

### 6.2 Mencatat perbaikan yang sudah terjadi

Dari halaman **detail aset** → tab Perawatan → **+ Catat Perawatan**. Isi apa adanya; dokumen
langsung berstatus selesai.

### 6.3 Perbaikan yang memerlukan vendor berbayar

Pada work order yang belum selesai, klik **Ajukan Pengadaan Jasa**. Pengajuan jasa terbentuk dan
tertaut ke work order ini — biayanya cukup diketik sekali.

---

## 7. Opname Aset (Audit Fisik)

1. Buka **Opname Aset** → **+ Buka Sesi Opname**, pilih outlet.
2. Daftar aset outlet itu dibekukan sebagai baris hitungan. Aset yang sedang dalam perjalanan
   mutasi tidak diikutkan.
3. Hitung fisik, isi kolom hitungan tiap baris. Bisa disimpan bertahap — tidak harus sekali
   duduk.
4. Barang yang ada di lapangan tapi tak ada di daftar: buka **Catat barang yang tidak ada di
   daftar**, isi nama, jumlah, dan lokasinya.
5. Klik **Simpan Hitungan** untuk menyimpan sementara.
6. Setelah semua terhitung, atasan menekan **Terapkan Selisih**.

**Yang terjadi saat selisih diterapkan:**

| Keadaan | Hasilnya |
|---|---|
| Jumlah berbeda | Jumlah aset disesuaikan dengan hitungan |
| Kondisi berbeda | Kondisi aset diperbarui |
| Barang temuan | Menjadi aset baru bersumber "opname" |
| Barang tidak ditemukan | Menjadi **usulan penghapusan**, menunggu persetujuan — tidak langsung dihapus |

---

## 8. Penghapusan Aset

1. Buka **Penghapusan** → **+ Ajukan Penghapusan**.
2. Pilih asetnya, jumlah, dan caranya: dijual, dihibahkan, dimusnahkan, hilang, atau tukar
   tambah.
3. Isi alasan yang jelas. Bila dijual, isi hasil penjualannya.
4. Atasan menyetujui atau menolak.

Nilai buku saat diajukan **dibekukan** di dokumen, sehingga berita acara tetap cocok meski
penyusutan terus berjalan.

> **Tombol Hapus di Daftar Aset bukan untuk ini.** Tombol itu hanya untuk salah input. Barang
> yang benar-benar sudah tidak ada harus lewat dokumen penghapusan.

---

## 9. Material Projek

1. Material diterima lewat **Penerimaan Peralatan** seperti biasa, dengan tujuan **Material
   projek**.
2. Pemakaian dan sisanya dikelola di halaman **Projek → detail projek → panel Material Projek**.
3. Mencatat pemakaian: klik **Catat Pemakaian**, isi jumlah yang dipakai.
4. Saat projek ditutup, sisa material **didata menjadi aset**: klik **Catat semua sisa sebagai
   aset**, atau tentukan satu per satu lewat **Tentukan Sisa**.

Pilihan lain (kembali ke gudang, susut, pindah projek) adalah pengecualian — susut wajib
beralasan.

---

## 10. Kategori Aset

Kategori menentukan **umur ekonomis** dan **interval perawatan** bawaan setiap aset baru, jadi
isinya berpengaruh langsung ke nilai buku dan jadwal perawatan.

1. Buka **Kategori Aset**.
2. **+ Tambah Kategori** untuk yang belum ada: isi nama, umur ekonomis (bulan), interval
   perawatan (bulan), dan catatan.
3. Kategori yang tidak dipakai lagi sebaiknya **dinonaktifkan**, bukan dihapus.

Mengganti nama kategori otomatis memperbarui seluruh aset yang memakainya.

---

## 11. Laporan & Letak Angka KPI

Menu **Laporan Aset** berisi lima laporan; masing-masing bisa diunduh sebagai Excel lewat tombol
**Export Excel**.

| KPI (lihat dokumen uraian tugas) | Letak angkanya |
|---|---|
| Kepatuhan jadwal perawatan | Perawatan → tab Riwayat & Jatuh Tempo |
| Perawatan terlambat | Dashboard Aset → kartu "Perawatan terlambat" |
| Akurasi data aset | Opname Aset → persentase akurasi tiap sesi |
| Kelengkapan data aset | Laporan Aset → Daftar Aset (kolom kosong terlihat) |
| Ketepatan pencatatan penerimaan | Laporan Aset → Pengadaan Belum Lengkap |
| Kecepatan penyerahan ke pemakai | Distribusi ke PIC → daftar "Menunggu Diserahkan" |
| Kelengkapan bukti foto | Pengaturan → Cadangan Bukti Foto |
| Selisih perpindahan | Mutasi Antar Outlet → penanda "Selisih" |
| Rasio preventif : korektif | Perawatan → kartu ringkasan |
| Aset boros belum ditindak | Dashboard Aset → Biaya Rawat Tertinggi |

---

## 12. Bukti Foto & Cadangannya

Setiap foto penerimaan dan penyerahan otomatis disalin ke Google Drive perusahaan dalam 15
menit. Statusnya terlihat di bawah tiap foto:

| Keterangan | Artinya |
|---|---|
| ✓ tersalin ke Drive | Aman; salinannya ada di luar server |
| … menunggu disalin | Normal, tunggu sebentar |
| ! gagal disalin | Laporkan ke admin sistem — buktinya baru ada di satu tempat |

Halaman **Pengaturan → Cadangan Bukti Foto** menampilkan jumlah yang menunggu dan gagal, serta
tombol **coba lagi**.

---

## 13. Bekerja dari Ponsel

Seluruh halaman bisa dipakai dari ponsel, dan memang dirancang begitu — penerimaan, opname, dan
penyerahan dikerjakan sambil memegang barangnya.

- Daftar berubah menjadi kartu; tombol utama melebar penuh agar mudah ditekan satu tangan.
- Tombol foto langsung membuka kamera belakang.
- Isian angka memunculkan papan tik angka.

---

## 14. Hak Akses

Bila sebuah menu tidak muncul atau tombolnya tidak ada, hak aksesnya belum diberikan. Minta
atasan membukanya lewat menu **Role**.

| Hak akses | Untuk |
|---|---|
| `assets.dashboard.view` | Dashboard Aset |
| `assets.view` / `.create` / `.update` | Daftar Aset, Kategori Aset, melengkapi data |
| `assets.transfer.view` / `.create` / `.receive` | Mutasi antar outlet |
| `assets.maintenance.view` / `.create` | Perawatan |
| `assets.opname.view` / `.create` | Opname |
| `assets.disposal.view` / `.create` | Penghapusan |
| `assets.report.view` | Laporan Aset |
| `procurement.requests.submit` | Menyelesaikan serah terima pengadaan |

Hak **menyetujui** (`.approve`) dan **menghapus** (`assets.delete`) sengaja dipegang atasan.

---

*Panduan ini mengikuti tampilan aplikasi per 16 September 2026. Rincian alur dan alasan
rancangannya ada di `docs/perlengkapan-aset.md`.*
