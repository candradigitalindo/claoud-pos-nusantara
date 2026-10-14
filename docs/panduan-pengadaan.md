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
  ┌───────────┐  ┌────────────┐  ┌──────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐
  │ PENGAJUAN │─▶│ VERIFIKASI │─▶│ APPROVE  │─▶│ PEMBELIAN  │─▶│ PENERIMAAN │─▶│ DISTRIBUSI │
  └───────────┘  └────────────┘  └──────────┘  └────────────┘  └────────────┘  └────────────┘
   Manager        Bagian          Penyetuju     Pembelian      Gudang Induk /  ke Outlet /
   Outlet /       Perlengkapan    (owner)       Barang &       Bagian Aset     PIC (serah
   Head Store                                   Perlengkapan /                 terima)
                                                Bahan Baku
   HPS + sumber   Cek barang di   Setujui /     Isi harga,     Barang dapur →  Gudang → gudang
   HPS wajib      outlet lain;    Tolak +       vendor dari    Gudang Induk    outlet
   terisi         tolak per item  alasan        daftar, ajukan Peralatan →     Aset → PIC
                  bila masih ada                bayar          Bagian Aset     pengaju
```

Siapa yang memverifikasi, menyetujui, dan membeli **diatur lewat menu Role** — tidak ada akun
yang ditetapkan di sistem (lihat §9). **Pengadaan Bahan Baku tidak melewati Verifikasi**:
pengajuan Kepala Gudang Induk langsung menunggu persetujuan Manager Logistik (lihat §12).

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
3. Pengajuan yang terkait projek renovasi/pembangunan **wajib memilih projeknya** dan, pada tiap
   item, **baris RAB yang diserapnya**. Projek yang RAB-nya belum ditetapkan tidak menerima
   pengajuan (lihat §11).
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

Status pengajuan menjadi **Menunggu Verifikasi** (bahan baku: langsung **Menunggu Persetujuan**,
§12). Selama status itu, pengaju masih bisa mengubah isinya; sejak diverifikasi, isi pengajuan
terkunci.

---

## 3. Tahap 2 — Verifikasi & Approve

Setiap pengajuan melewati dua pemeriksaan berurutan: **verifikasi** oleh bagian perlengkapan,
lalu **persetujuan** oleh penyetuju. Siapa yang boleh melakukan masing-masing diatur di menu
**Role** — klik **Verifikasi** atau **Approval** pada role yang dimaksud (§9).

### 3.1 Verifikasi — mencegah pengadaan ganda (barang & jasa)

**Siapa:** role berizin Verifikasi (`procurement.requests.verify`), umumnya Bagian Perlengkapan.
Bahan baku tidak melalui tahap ini.
**Menu:** Pengadaan → buka pengajuan berstatus **Menunggu Verifikasi**

1. Periksa setiap item: apakah barangnya **masih ada atau menganggur di outlet lain**.
2. Bila masih ada, **tolak item itu** dengan alasan, misalnya "Blender menganggur di outlet
   Semesta — dimutasi". Pemindahannya dikonfirmasi ke bagian perlengkapan lewat Mutasi Aset
   atau Transfer Stok, bukan dibeli baru.
3. Item yang lolos diteruskan dengan menekan **Verifikasi**. Status menjadi **Menunggu
   Persetujuan**.
4. Bila **semua** item ditolak, pengajuan otomatis berstatus **Ditolak**.

Item yang ditolak tidak ikut dihitung dan tidak bisa dibeli. Daftar beserta alasannya tetap
terlihat pada pengajuan ("Item ditolak saat verifikasi").

Bila dua orang membuka pengajuan yang sama dan salah satunya sudah mengubahnya, yang lain
mendapat pesan **"Pengajuan berubah, muat ulang"** — keputusan selalu diambil atas isi terbaru.

### 3.2 Approve

**Siapa:** role berizin Approval (`procurement.requests.approve`; bahan baku:
`procurement.rawmaterials.approve`, mis. Manager Logistik) — penyetuju
**Menu:** Pengadaan → buka pengajuan berstatus **Menunggu Persetujuan**

| Keputusan | Akibatnya | Ketentuan |
|---|---|---|
| **Biarkan** | Pengajuan tetap Menunggu Persetujuan | Dipakai bila masih perlu keterangan dari pengaju |
| **Setujui** | Status menjadi Disetujui, lanjut ke Pembelian | Pastikan HPS wajar dan sumbernya bisa diperiksa |
| **Tolak** | Status menjadi Ditolak, alur berhenti | **Alasan wajib diisi** — ditolak sistem bila kosong |

### Yang diperiksa sebelum menyetujui

1. **Hasil verifikasi** — item mana yang ditolak dan alasannya.
2. **HPS masuk akal** dan **sumbernya bisa dibuka/dihubungi**.
3. **Jumlahnya wajar** terhadap kebutuhan operasional.
4. Untuk belanja projek: **anggaran baris RAB baru terpakai saat pengajuan disetujui** (§11).
   Bila persetujuan ini melampaui sisa baris RAB, panel persetujuan menampilkan peringatan
   beserta sisanya. Persetujuan tetap bisa dilakukan; bila perlu, revisi RAB dulu.

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

**Siapa:** Pembelian Barang & Perlengkapan (izin `procurement.requests.purchasing`, untuk barang
dan jasa) atau Pembelian Bahan Baku (izin `procurement.rawmaterials.purchasing`); pembayaran oleh
Keuangan (izin `finance.payments.pay`)

### 4.1 Mengisi harga final & vendor

1. Buka pengajuan berstatus **Disetujui** → **Isi Harga (Purchasing)**.
2. Isi **harga final** tiap item — harga yang benar-benar disepakati vendor.
3. **Pilih vendor dari daftar vendor** (wajib) dan isi **nomor invoice**.
4. Simpan. Selisih HPS dan harga final terlihat langsung di layar.

Vendor yang belum terdaftar didaftarkan dulu di menu **Vendor** beserta rekeningnya. Vendor yang
dibayar tunai (mis. belanja di pasar) cukup didaftarkan tanpa rekening.

**Struk pembelian.** Satu pengajuan bisa dibeli dalam beberapa transaksi di vendornya, masing-
masing dengan struknya — mis. struk A tanggal 1 senilai Rp 19 jt dan struk B senilai Rp 20 jt.
Catat setiap struk di detail pengajuan (bagian **Struk Pembelian**): nomor, tanggal, total, dan
**foto struk (wajib)**. Bagian pembelian maupun petugas logistik yang membawa struknya boleh
mencatat. Layar membandingkan total struk dengan total harga final; struk yang belum dibayar boleh
dihapus.

Harga final dan nomor invoice **hanya** diisi di tahap ini, pada pengajuan yang sudah disetujui.
Harga final atau invoice yang dikirim pengaju saat membuat atau mengubah pengajuan (pending)
diabaikan sistem — nilai pengajuan sampai tahap persetujuan selalu HPS.

**Bila satu pengajuan dibeli dari beberapa vendor**, pakai **Pecah per Vendor**: item dipindahkan
ke dokumen anak dengan vendornya masing-masing. Pembayaran lalu dilakukan per vendor.

### 4.2 Mengajukan pembayaran

Setelah harga final semua item terisi dan vendor dipilih dari daftar, tekan **Ajukan
Pembayaran**. Status menjadi **Menunggu Pembayaran** dan muncul di menu **Pembayaran**.
Pengajuan tanpa vendor terdaftar tidak bisa diajukan pembayarannya.

### 4.3 Pembayaran

Bagian Keuangan membuka **Pembayaran**, memilih rekening sumber (rekening perusahaan),
mengunggah bukti transfer, lalu menyimpan. Pembayaran boleh **sebagian** (cicil) dan boleh
**mundur** — barang boleh diterima lebih dulu, tagihannya tetap tercatat sampai lunas. Halaman
Pembayaran hanya memuat pengajuan yang sudah disetujui.

**Bayar per struk.** Bila pengajuan punya struk, form bayar menampilkan pilihan **Struk yang
Dibayar** beserta sisanya; jumlah yang dikosongkan berarti melunasi sisa struk itu. Tanpa memilih
struk (mis. bayar sekaligus beberapa pengajuan), pembayaran dibagi ke struk yang belum lunas,
terlama dulu. Riwayat pembayaran mencatat struk mana yang dibayar.

**Rekening tujuan diambil otomatis dari data vendor** dan tidak bisa diketik ulang di form bayar.
Rekening itu **disimpan permanen pada setiap pembayaran**:

- Bila vendor kemudian mengganti nomor rekeningnya, **pembayaran lama tetap tercatat dengan
  rekening lama**; pembayaran berikutnya memakai rekening baru.
- Setiap perubahan rekening vendor dicatat — siapa, kapan, rekening lama → baru — dan terlihat
  di halaman detail vendor. Selama 30 hari setelah perubahan, form bayar menampilkan peringatan
  beserta rekening sebelumnya. **Konfirmasi ke vendor bahwa perubahan itu sah sebelum
  mentransfer.**
- Vendor tanpa rekening terdaftar: tulis cara bayarnya (mis. "Tunai"). Pembayaran itu tercatat
  dengan tanda **manual**.

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
   pembayarannya lunas. Baris **stok gudang** yang dicatat oleh petugas tanpa hak gudang
   (misalnya dari halaman Pengadaan) **belum dihitung diterima**: baris itu berpindah ke antrean
   Gudang Induk dan dokumen tetap terbuka sampai gudang mencatatnya.
5. Barang dapur diterima di **Gudang Induk** dulu, lalu diteruskan ke outlet lewat Transfer
   Stok. Dialog penerimaan menampilkan gudang mana yang membutuhkannya (dari MRP atau dari outlet
   pengaju), tetapi tujuan penerimaannya tetap Gudang Induk.

### Pengambilan oleh petugas logistik

Barang bisa **diantar vendor** langsung atau **diambil petugas logistik** — dipilih sesuai
keadaan, tidak perlu ditetapkan di awal.

1. Petugas logistik (izin **Pengambilan Barang**) membuka pengajuan yang sudah disetujui →
   **Catat Pengambilan**: **foto barang saat diambil (wajib)** dan catatan (berat/jumlah koli,
   struk yang dibawa). Status pengambilan menjadi **Dalam pengantaran**.
2. Saat barang tiba, petugas penerimaan memilih **Diantar oleh** di dialog Serah Terima: vendor
   langsung, atau pengambilan petugas logistik yang sedang dalam pengantaran. Pengambilan itu
   tercatat **sudah diterima** beserta penerima dan waktunya.

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
2. **Yang mengajukan bukan yang memverifikasi atau menyetujui.** Pemisahan ini diatur lewat hak
   akses: jangan berikan izin Verifikasi atau Approval kepada role pengaju.
3. **Angka tidak diketik dua kali.** Perbaikan aset yang memerlukan vendor diajukan langsung dari
   work order perawatan, sehingga biayanya cukup diisi sekali.
4. **Dokumen tidak dihapus untuk menutupi kesalahan.** Pengajuan yang keliru dibatalkan
   (statusnya tercatat), bukan dihapus.
5. **Pengajuan yang sudah dibayar tidak bisa diubah itemnya.** Perbaikan hanya mungkin sebelum
   pembayaran diajukan.
6. **Pengajuan yang barangnya sudah mulai diterima dikunci.** Begitu ada aset, stok, atau
   material projek yang lahir darinya, pengajuan itu tidak bisa dibatalkan, diubah itemnya, atau
   dihapus, meski belum dibayar (pembelian tempo). Koreksi dilakukan lewat dokumen penghapusan
   aset atau koreksi stok, bukan dengan membatalkan pengajuannya.
7. **GRN manual bukan untuk barang pengadaan.** Menu Gudang → Penerimaan Barang menolak nomor
   pengajuan yang masih hidup; barang pengadaan dicatat lewat Penerimaan dari Pengadaan supaya
   pengajuannya ikut tertutup.

---

## 8. Tabel Status

| Status | Artinya | Siapa yang bergerak berikutnya |
|---|---|---|
| **Menunggu Verifikasi** | Barang/jasa baru diajukan; menunggu pemeriksaan ketersediaan di outlet lain | Pemegang izin Verifikasi |
| **Menunggu Persetujuan** | Sudah diverifikasi (bahan baku: baru diajukan, tanpa verifikasi) | Pemegang izin Approval |
| **Disetujui** | Boleh dibelanjakan | Pembelian |
| **Menunggu Pembayaran** | Harga final terisi, menunggu dibayar | Keuangan |
| **Dibayar Sebagian** | Sebagian tagihan lunas | Keuangan (sisa) |
| **Dibayar** | Tagihan lunas | Penerimaan |
| **Diterima** | Seluruh barang diterima **dan** lunas | Distribusi |
| **Ditolak** | Ditolak penyetuju beserta alasannya, atau semua itemnya ditolak saat verifikasi | Pengaju (ajukan ulang bila masih perlu) |
| **Dibatalkan** | Dibatalkan sebelum dibayar dan sebelum ada barang yang diterima | — |

Penanda **Barang sudah/belum diterima** berdiri sendiri dari status di atas, karena barang bisa
datang sebelum maupun sesudah dibayar.

---

## 9. Hak Akses per Peran

| Peran | Hak akses |
|---|---|
| **Manager Outlet / Head Store** | `procurement.requests.view`, `procurement.requests.submit` |
| **Bagian Perlengkapan (verifikator)** | `procurement.requests.view`, `procurement.requests.verify` |
| **Penyetuju (owner)** | `procurement.requests.approve` |
| **Petugas Logistik** | `procurement.requests.pickup` dan/atau `procurement.rawmaterials.pickup` (Pengambilan Barang) |
| **Pembelian Barang & Perlengkapan** | `procurement.requests.view`, `procurement.requests.purchasing` |
| **Keuangan** | `finance.payments.view`, `finance.payments.pay` |
| **Logistik / Gudang** | `stockledger.adjust`, `stocktransfers.*` |
| **Asset Officer** | `assets.create`, `assets.update`, `procurement.requests.submit` |
| **PIC Projek** | `procurement.projects.view`, `procurement.projects.manage` (susun & tetapkan RAB) |
| **Kepala Gudang Induk** | `procurement.rawmaterials.view`, `procurement.rawmaterials.submit` (Pengadaan Bahan Baku, lihat §12) |
| **Manager Logistik** | `procurement.rawmaterials.view`, `procurement.rawmaterials.approve` |
| **PPIC / Perencana Bahan** | `procurement.rawmaterials.view`, `procurement.rawmaterials.submit` (draft dari MRP) |
| **Pembelian Bahan Baku** | `procurement.rawmaterials.view`, `procurement.rawmaterials.purchasing` |

Di menu **Role**, izin ini tampil sebagai pilihan **Lihat · Buat/Hapus · Verifikasi · Approval ·
Pembelian · Pengambilan Barang** pada baris *Pengadaan Barang, Perlengkapan & Jasa*, dan **Lihat ·
Buat/Hapus · Approval · Pembelian · Pengambilan Barang** pada *Pengadaan Bahan Baku*. Satu
izin boleh diberikan ke beberapa role sekaligus. Perubahan berlaku begitu disimpan; pengguna role
itu cukup memuat ulang halaman. Cakupan outlet role ikut berlaku: role bercakupan outlet tertentu
hanya bisa memverifikasi/menyetujui pengajuan outlet itu, jadi bagian perlengkapan yang memeriksa
ke outlet lain sebaiknya bercakupan semua outlet.

Izin `procurement.requests.*` hanya mengatur pengajuan **Barang** dan **Jasa**. Pengajuan
**Bahan Baku** diatur `procurement.rawmaterials.*` (lihat, ajukan, setujui, pembelian,
pengambilan) — tanpa verifikasi. Izin Keuangan (`finance.payments.view`, `finance.payments.pay`)
berlaku untuk semua jenis, dan Dashboard Pengadaan menampilkan semua jenis. Izin bahan baku **tidak**
disalin dari izin pengadaan barang: orangnya berbeda, jadi diberikan per role lewat menu **Role**
(saat dipasang, hanya admin, superadmin, dan role PPIC — lihat & ajukan — yang memilikinya).

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

**Barang yang diajukan ternyata masih ada di outlet lain — bagaimana?**
Verifikator menolak item itu saat verifikasi dengan alasannya, lalu pemindahannya dikonfirmasi ke
bagian perlengkapan (Mutasi Aset / Transfer Stok). Item lain di pengajuan yang sama tetap lanjut
ke persetujuan.

**Vendor mengganti nomor rekening — apakah pembayaran lama ikut berubah?**
Tidak. Rekening disimpan pada setiap pembayaran saat dicatat. Pembayaran berikutnya memakai
rekening baru, dan perubahannya tercatat di detail vendor.

---

*Panduan ini mengikuti alur yang berjalan di aplikasi per 14 Oktober 2026. Bila alurnya
berubah, dokumen ini ikut ditinjau.*

---

## 11. Belanja Projek — RAB per Baris

Projek pembangunan/renovasi tidak lagi memegang RAB sebagai satu angka. RAB disusun per baris
pekerjaan, ditetapkan, dan setiap item belanja menunjuk baris yang ia serap — sehingga yang
terlihat bukan hanya "sisa RAB", melainkan **pos mana yang sudah habis dan pos mana yang belum
tersentuh**.

### Alur

```
  ┌──────────────┐   ┌──────────────┐   ┌────────────────┐   ┌──────────────────────┐
  │ BUAT PROJEK  │──▶│  SUSUN RAB   │──▶│ TETAPKAN RAB   │──▶│  BELANJA TAHAP       │
  └──────────────┘   └──────────────┘   └────────────────┘   └──────────────────────┘
   nama, unit,        bagian pekerjaan   baris dikunci,       pengajuan barang/jasa
   PIC, jadwal        → uraian, volume   versi naik,          biasa; tiap item
   (status draft)     × harga satuan     projek → berjalan    memilih baris RAB
```

| Langkah | Siapa | Menu |
|---|---|---|
| Buat projek | PIC Projek (`procurement.projects.manage`) | Pengadaan → Projek → **+ Buat Projek** |
| Susun RAB | PIC Projek | Detail projek → **Susun RAB** |
| Tetapkan RAB | PIC Projek | Detail projek → **Tetapkan RAB** |
| Belanja tahap | Pengaju (`procurement.requests.submit`) | Detail projek → **+ Belanja Tahap Baru**, atau form pengadaan biasa dengan projek dipilih |

### Ketentuan RAB

1. Satu baris = **uraian + jenis + volume × harga satuan**. Bagian pekerjaan (Pekerjaan
   Persiapan, Sipil, Mekanikal & Elektrikal, dst.) hanya pengelompokan.
2. **Jenis baris** menentukan pengajuan mana yang boleh menyerapnya: `barang` hanya oleh
   pengajuan barang, `jasa` hanya oleh pengajuan jasa, `umum` oleh keduanya (mis. "Tak terduga").
3. Total RAB = jumlah seluruh baris. Angka ini yang tampil sebagai RAB projek di daftar, neraca
   (Projek Berjalan), dan rekap serapan.
4. **Belanja tahap hanya bisa dibuat setelah RAB ditetapkan.** Pengajuan untuk projek yang
   RAB-nya masih draft atau sedang direvisi ditolak sistem.
5. Mengubah RAB yang sudah ditetapkan: **Buka Revisi** → ubah baris → **Tetapkan** lagi (versi
   naik). Baris yang sudah dipakai pengajuan tidak bisa dihapus, hanya diubah volume/harganya.
6. Projek lama yang RAB-nya satu angka otomatis mendapat satu baris `umum` senilai angka itu
   (sudah ditetapkan), sehingga belanjanya tetap berjalan.

### Ketentuan item belanja projek

1. Pada form pengajuan, setelah projek dipilih, setiap item punya pilihan **Baris RAB**. Memilih
   baris mengisi nama (bila kosong), satuan, HPS satuan, dan Sumber HPS (`RAB PRJ-… v1: …`).
2. Item boleh dibiarkan **di luar RAB** bila memang tidak ada posnya. Nilainya tetap membebani
   projek, tetapi tampil terpisah sebagai "Belanja di luar RAB" — bahan evaluasi, bukan
   pelanggaran.
3. Item yang nilainya **melebihi sisa baris** ditandai merah di form. Sistem tidak memblokir,
   karena harga bisa naik; penyetuju yang memutuskan (lihat §3.2).
4. **Anggaran baru terpakai setelah pengajuan disetujui.** Pengajuan yang masih Menunggu
   Verifikasi atau Menunggu Persetujuan tidak mengurangi Sisa RAB; nilainya ditampilkan terpisah
   sebagai *menunggu persetujuan*. Karena pengajuan yang sedang antre belum memotong sisa baris,
   panel persetujuan memperingatkan penyetuju bila persetujuannya melampaui sisa itu.
5. Pecahan vendor dan pengisian harga final tetap membawa rujukan baris RAB-nya. Angka
   **Terpakai** memakai harga final begitu diisi, HPS selama belum.
6. Pembayaran dicatat per dokumen, jadi angka **Terbayar per baris** adalah alokasi proporsional
   dari pembayaran dokumen ke item-itemnya.

---

## 12. Pengadaan Bahan Baku

Bahan dapur (tepung, gula, daging, sayur, bumbu) diajukan lewat jenis pengadaan sendiri,
terpisah dari Pengadaan Barang. Orang yang merencanakan dan menyetujui kebutuhan bahan berbeda
dengan pengaju peralatan, jadi hak aksesnya pun dipisah.

| | |
|---|---|
| **Siapa** | Kepala Gudang Induk (mengajukan), Manager Logistik (menyetujui), Pembelian Bahan Baku, Petugas Logistik, PPIC (draft MRP) |
| **Menu** | Pengadaan → **Bahan Baku** |
| **Izin** | `procurement.rawmaterials.view` · `.submit` · `.approve` · `.purchasing` · `.pickup` (lihat §9) |

### Alur

```
  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐  ┌────────────┐
  │ PENGAJUAN     │─▶│ APPROVE      │─▶│ PEMBELIAN    │─▶│ PEMBAYARAN  │─▶│ PENGAMBILAN  │─▶│ PENERIMAAN │
  └───────────────┘  └──────────────┘  └──────────────┘  └─────────────┘  └──────────────┘  └────────────┘
   Kepala Gudang      Manager           Pembelian         Keuangan:         Petugas logistik  Gudang Induk
   Induk / PPIC       Logistik          Bahan Baku:       mundur, cicil,    (atau diantar     (stok masuk,
                                        harga, vendor,    per struk         vendor langsung)  foto wajib)
                                        struk
```

Pembayaran dan pengambilan tidak harus berurutan: barang boleh diambil/diterima dulu lalu dibayar
belakangan (bayar mundur), atau dibayar dulu baru diambil.

### Ketentuan

1. **Tanpa verifikasi.** Pengajuan bahan baku langsung berstatus **Menunggu Persetujuan** dan
   pengajunya masih bisa mengubah isinya sampai disetujui. Selebihnya sama dengan Pengadaan
   Barang: Approve / Tolak dengan alasan → isi harga final, pilih vendor, catat struk, pecah per
   vendor bila perlu, ajukan bayar → Keuangan membayar (per struk, cicil, atau mundur) → barang
   diambil petugas logistik atau diantar vendor → diterima Gudang Induk.
2. **Seluruh barisnya diterima di Gudang Induk** (Gudang → **Penerimaan dari Pengadaan**),
   termasuk bahan yang belum terdaftar di katalog stok. Pengajuan bahan baku tidak pernah
   masuk antrean Penerimaan Peralatan dan tidak menjadi aset.
3. **Tidak untuk projek.** Pengajuan bahan baku tidak bisa ditautkan ke projek; material projek
   diajukan lewat Pengadaan Barang/Jasa dengan baris RAB (§11).
4. **Draft dari MRP masuk ke sini.** Tombol buat PR di lembar kerja MRP (PPIC) menghasilkan
   Pengadaan Bahan Baku berstatus Menunggu Persetujuan, lalu diproses seperti pengajuan lain.
5. **Outlet tidak mengajukan bahan baku ke pengadaan.** Outlet meminta bahan ke Gudang Induk
   lewat **Transfer Stok** (gudang asal: Gudang Induk); Gudang Induk menyetujui dan mengirimnya.
6. Di laporan keuangan, belanja bahan baku dicatat sebagai **HPP bahan baku**, bukan belanja
   modal.
7. Larangan mencampur barang dapur dan peralatan (§1) tidak berlaku di sini karena seluruh
   isinya memang bahan dapur. Peralatan dapur (kompor, freezer) tetap diajukan lewat Pengadaan
   Barang.
