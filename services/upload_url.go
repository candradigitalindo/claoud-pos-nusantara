package services

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ── URL berkas unggahan ─────────────────────────────────────────────────────
//
// Bukti foto & berkas (serah terima aset, penerimaan pengadaan, kirim mutasi
// aset, distribusi stok, bukti bayar pengadaan, bukti DP reservasi) dikirim
// klien sebagai URL hasil POST /admin/upload: "/uploads/YYYY-MM/<ULID>.<ext>".
// Dulu server hanya mengecek nilainya tidak kosong, padahal foto serah terima
// kelak dibaca dari disk lalu disalin ke Google Drive dengan tautan publik —
// "/uploads/../../root/.config/rclone/rclone.conf" membuat berkas server di
// luar folder unggahan ikut terbit (temuan T-2, 14 Okt 2026). Karena itu:
//   - setiap titik masuk memvalidasi URL dengan ValidateUploadURL, dan
//   - setiap operasi disk memetakan URL lewat UploadFilePath/resolveUploadFile,
//     yang menjamin path tetap di dalam folder uploads.

// UploadsDir: folder unggahan, relatif terhadap direktori kerja aplikasi —
// sama dengan yang ditulis handler unggah dan disajikan app.Static("/uploads").
const UploadsDir = "uploads"

const uploadURLPrefix = "/" + UploadsDir + "/"

// maxUploadURLLen: URL hasil unggah ±45 karakter; batas ini sekadar pagar.
const maxUploadURLLen = 255

// uploadExts: ekstensi yang diterima handler unggah bukti (POST /admin/upload),
// satu-satunya pintu lahirnya URL bukti. Handler memakai daftar yang sama.
var uploadExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".pdf": true}

// UploadExtAllowed: ekstensi (bertitik, huruf besar/kecil) boleh diunggah sebagai bukti.
func UploadExtAllowed(ext string) bool { return uploadExts[strings.ToLower(ext)] }

// checkUploadURLPath memeriksa bentuk URL unggahan tanpa melihat ekstensi.
func checkUploadURLPath(raw string) error {
	if raw == "" {
		return Invalid("alamat berkas kosong")
	}
	if len(raw) > maxUploadURLLen {
		return Invalid("alamat berkas terlalu panjang")
	}
	if !strings.HasPrefix(raw, uploadURLPrefix) {
		return Invalid("harus berkas yang diunggah lewat aplikasi (alamat diawali /uploads/)")
	}
	// Daftar putih karakter. Sekaligus menolak karakter kontrol, backslash,
	// spasi, '%' (".." tersandi), '?', '#', ':' (skema URL), dan non-ASCII.
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		ok := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' ||
			ch == '-' || ch == '_' || ch == '.' || ch == '/'
		if !ok {
			return Invalid("alamat berkas memuat karakter yang tidak diizinkan")
		}
	}
	rel := raw[len(uploadURLPrefix):]
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return Invalid("alamat berkas tidak boleh memuat segmen kosong, '.', atau '..'")
		}
	}
	// Pagar terakhir: setelah Clean, path tetap di bawah folder uploads.
	if clean := filepath.Clean(filepath.FromSlash(rel)); !filepath.IsLocal(clean) || filepath.ToSlash(clean) != rel {
		return Invalid("alamat berkas keluar dari folder unggahan")
	}
	return nil
}

// ValidateUploadURL memastikan raw adalah URL berkas hasil unggahan aplikasi:
// berawalan "/uploads/", hanya huruf/angka ASCII, '-', '_', '.', '/', tanpa
// segmen kosong/'.'/'..', berekstensi JPG/PNG/WebP/PDF, dan hasil
// filepath.Clean-nya tetap di dalam folder uploads. URL penuh (http://…), path
// absolut lain, dan path relatif ditolak. Galatnya ValidationError (pesan aman
// ditampilkan apa adanya). Keberadaan berkas tidak diperiksa di sini.
func ValidateUploadURL(raw string) error {
	if err := checkUploadURLPath(raw); err != nil {
		return err
	}
	if !UploadExtAllowed(path.Ext(raw)) {
		return Invalid("jenis berkas harus JPG, PNG, WebP, atau PDF")
	}
	return nil
}

// validateUploadField = ValidateUploadURL dengan nama isian di pesannya, mis.
// "foto serah terima tidak sah: harus berkas yang diunggah lewat aplikasi …".
func validateUploadField(label, raw string) error {
	if err := ValidateUploadURL(raw); err != nil {
		return Invalid("%s tidak sah: %s", label, err.Error())
	}
	return nil
}

// ValidatePaymentProof memeriksa payment_proof pembayaran pengadaan. Isinya
// gabungan "<no. referensi> | <URL bukti>" (lihat ProcurementPayments.vue dan
// splitPaymentProof), keduanya opsional. Bagian berupa path (diawali '/' atau
// '\') wajib URL unggahan yang sah; tautan luar ("://") ditolak.
func ValidatePaymentProof(proof string) error {
	for _, part := range strings.Split(proof, " | ") {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
		case strings.HasPrefix(part, "/") || strings.HasPrefix(part, `\`):
			if err := validateUploadField("berkas bukti pembayaran", part); err != nil {
				return err
			}
		case strings.Contains(part, "://"):
			return Invalid("bukti pembayaran harus berkas yang diunggah lewat aplikasi, bukan tautan luar")
		}
	}
	return nil
}

// UploadFilePath memetakan URL unggahan ke path absolut di disk yang dijamin
// berada di dalam folder uploads absolut. Ekstensi tidak diperiksa (foto produk
// boleh GIF); penerima bukti memakai ValidateUploadURL.
func UploadFilePath(raw string) (string, error) {
	return uploadFilePathIn(UploadsDir, raw)
}

func uploadFilePathIn(dir, raw string) (string, error) {
	if err := checkUploadURLPath(raw); err != nil {
		return "", err
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.New("folder unggahan tidak bisa ditentukan")
	}
	p := filepath.Join(root, filepath.FromSlash(raw[len(uploadURLPrefix):]))
	if !pathInsideDir(root, p) {
		return "", Invalid("alamat berkas keluar dari folder unggahan")
	}
	return p, nil
}

// pathInsideDir: p (absolut & bersih) berada di bawah dir, bukan dir itu sendiri.
func pathInsideDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

// resolveUploadFile menyiapkan berkas unggahan untuk dibaca dari disk (mis.
// disalin ke Drive): URL wajib sah (ValidateUploadURL), target wajib berkas
// biasa (folder, perangkat, dan symlink ditolak), dan setelah semua symlink
// diurai path-nya tetap di dalam folder uploads (yang juga diurai). Yang
// dikembalikan path absolut hasil urai — itulah yang boleh disalin.
func resolveUploadFile(raw string) (string, error) {
	return resolveUploadFileIn(UploadsDir, raw)
}

func resolveUploadFileIn(dir, raw string) (string, error) {
	if err := ValidateUploadURL(raw); err != nil {
		return "", err
	}
	p, err := uploadFilePathIn(dir, raw)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", errors.New("berkas tidak ditemukan di server")
		}
		return "", errors.New("berkas tidak bisa dibaca")
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("bukan berkas biasa (folder atau symlink ditolak)")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.New("folder unggahan tidak bisa ditentukan")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("folder unggahan tidak bisa dibaca")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errors.New("berkas tidak bisa dibaca")
	}
	if !pathInsideDir(realRoot, real) {
		return "", errors.New("berkas berada di luar folder unggahan")
	}
	return real, nil
}
