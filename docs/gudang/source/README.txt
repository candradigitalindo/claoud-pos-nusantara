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

REVISI 3 LOKAL — GUDANG MENGELOLA BAHAN BAKU
Gudang mengelola bahan baku dan persediaannya, tanpa membuat atau mengubah resep.
Tutorial pusat resep F&B, resep bahan setengah jadi dan pengaturan pola pemotongan
dikeluarkan dari panduan Gudang. Alur transaksi stok tetap dijelaskan menggunakan
formula yang telah ditetapkan F&B. Dokumen berisi 69 halaman dan 45 screenshot.

Screenshot pada revisi ini menggunakan aset demo terisolasi dari revisi sebelumnya;
tidak ada pengambilan screenshot baru. Tiga screenshot editor F&B tetap disimpan
sebagai arsip, tetapi tidak dirujuk HTML, PDF atau manifest revisi 3:
48-fnb-recipe-center.png, 09-sfg-recipe.png, 10-breakdown-pattern.png.

Audit-aturan dan audit-tampilan merupakan dasar audit awal; revisi-fnb.json
menjelaskan perubahan atas dasar tersebut. Hasil validasi artefak lokal revisi 3
tersimpan di ../verifikasi.json. Klaim pengujian aplikasi dari revisi sebelumnya
tidak dianggap sebagai pengujian ulang untuk perubahan dokumen ini.

PDF revisi 3 hanya diperbarui secara lokal, belum diterbitkan atau diunggah.
URL PDF revisi 2 yang pernah dicatat menunjuk edisi lama dan bukan PDF lokal ini.
Perubahan aplikasi pada workspace belum dinyatakan diterapkan ke produksi.
