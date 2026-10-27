PANDUAN OPERASIONAL GUDANG — CLOUD POS

Artefak utama: ../Panduan-Operasional-Gudang-Cloud-POS.pdf
Versi aplikasi: 67092f2. Edisi: 27 Oktober 2026, Asia/Jakarta.
Screenshot diambil dari aplikasi aktual di browser Chromium, menggunakan data
simulasi pada database terisolasi. Halaman daftar ditampilkan dalam viewport;
sebagian screenshot formulir dipotong ke panel dialog agar lebih terbaca.
Data bisnis produksi tidak diubah saat penyusunan panduan.

Sumber naskah: front-pages.json, rule-pages.json, guide-pages.json, end-pages.json.
Sumber aturan: audit-aturan.json dan referensi file:baris pada bab aturan.
Bangun HTML: python3 build-book.py
Cetak PDF: node render-pdf.cjs (memerlukan Playwright dan Chromium).
PLAYWRIGHT_MODULE dapat menunjuk modul Playwright yang sudah tersedia.
DOC_OUTPUT dapat digunakan untuk memilih direktori artefak HTML/PDF.

Perbarui aturan dan screenshot bersama bila aplikasi berubah.
Gambar sumber tersedia di ../screenshots; daftar halaman dan checksum gambar
tersedia di ../manifest.json; hasil pemeriksaan PDF pada ../verifikasi.json.
Audit tampilan tambahan terdapat pada audit-tampilan.txt.
Tidak ada akun, token, atau kredensial database di sumber dokumentasi ini.

REVISI 3 — GUDANG MENGELOLA BAHAN BAKU
Gudang mengelola bahan baku dan persediaannya, tanpa membuat atau mengubah resep.
Tutorial pusat resep F&B, resep bahan setengah jadi dan pengaturan pola pemotongan
dikeluarkan dari panduan Gudang. Alur transaksi stok tetap dijelaskan menggunakan
formula yang telah ditetapkan F&B. Dokumen berisi 69 halaman dan 45 screenshot.

Screenshot pada revisi ini menggunakan aset demo terisolasi dari revisi sebelumnya;
tidak ada pengambilan screenshot baru. Tiga screenshot editor F&B tetap disimpan
sebagai arsip, tetapi tidak dirujuk HTML, PDF atau manifest revisi 3:
48-fnb-recipe-center.png, 09-sfg-recipe.png, 10-breakdown-pattern.png.

Audit-aturan dan audit-tampilan merupakan dasar audit awal; revisi-fnb.json
menjelaskan perubahan atas dasar tersebut. Hasil validasi artefak revisi 3
tersimpan di ../verifikasi.json. Klaim pengujian aplikasi dari revisi sebelumnya
tidak dianggap sebagai pengujian ulang untuk perubahan dokumen ini.

Status publikasi: published. PDF revisi 3 diterbitkan pada 27 Oktober 2026.
URL unduhan revisi 3 terverifikasi (parameter revisi menghindari cache lama):
https://pos.nbp.co.id/uploads/panduan/Panduan-Operasional-Gudang-Cloud-POS.pdf?rev=3
Dua URL lama berikut sudah diganti di origin dengan PDF revisi 3 yang sama:
https://pos.nbp.co.id/uploads/panduan/Panduan-Operasional-Gudang-Cloud-POS-2026-10-27.pdf
https://pos.nbp.co.id/uploads/panduan/Panduan-Operasional-Gudang-Cloud-POS-Revisi-FNB-2026-10-27.pdf
Ketiga file di origin telah diperiksa dan identik. Cache CDN pada URL lama dapat
menyajikan edisi sebelumnya sampai masa cache berakhir (hingga 4 jam).
URL unduhan revisi 3: HTTP 200, application/pdf, SHA256 sama dengan PDF lokal.
Hasil akses URL dan checksum unduhan tersimpan pada ../verifikasi.json.
Pembaruan PDF tidak menerapkan perubahan aplikasi ke produksi. Perubahan aplikasi
belum diterapkan ke produksi saat artefak revisi ini disiapkan.
