PANDUAN OPERASIONAL GUDANG — CLOUD POS

Artefak utama: ../Panduan-Operasional-Gudang-Cloud-POS.pdf
Revisi 4: kontrol bahan, opname dan kedaluwarsa. Edisi: 27 Oktober 2026, Asia/Jakarta.
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

REVISI 4 — FOKUS TIGA FUNGSI PPIC
PPIC menyisakan Kontrol Pemakaian Bahan, Stock Opname dan Monitor Kedaluwarsa.
Menu perencanaan/produksi/laporan lama dihapus. Data historis tetap disimpan.
Gudang tetap mengelola persediaan; resep ditetapkan F&B. Instruksi WO lama
pada bab produksi diganti dengan prosedur pencatatan dan pemeriksaan histori.

Screenshot memakai aset demo revisi sebelumnya. Label Buang pada screenshot
kedaluwarsa kini menjadi Lihat Waste dan hanya muncul sesuai izin akun.
Tidak ada pengambilan screenshot baru pada revisi dokumen ini.

CATATAN PENERBITAN
Hasil pemeriksaan artefak, checksum PDF, status publikasi dan pemeriksaan
unduhan server dicatat pada ../verifikasi.json. Status penerbitan diperbarui
setelah berkas dan aplikasi diperiksa pada server. Naskah PDF berisi rincian
revisi dan prosedur operasional, tanpa status deployment sementara.
