package services

// Uji validator URL unggahan dan pengurungan path ke folder uploads (T-2).
// Tanpa database: semua kasus murni string atau berkas di folder sementara.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateUploadURL(t *testing.T) {
	sah := []string{
		"/uploads/a/b.jpg",
		"/uploads/2026-10/01JABCDEFGHJKMNPQRSTVWXYZ0.png",
		"/uploads/2026-10/01JABCDEFGHJKMNPQRSTVWXYZ0.JPEG",
		"/uploads/x.pdf",
		"/uploads/2026-10/bukti_1.webp",
	}
	for _, u := range sah {
		if err := ValidateUploadURL(u); err != nil {
			t.Errorf("%q: mau sah, dapat %v", u, err)
		}
	}

	tolak := []string{
		"",
		"../.env",
		"/uploads/../config",
		"/uploads/../.env",
		"/uploads/../../root/.config/rclone/rclone.conf",
		"/uploads/a/../../etc/passwd.jpg",
		"/uploads/../uploads/a.jpg",
		"/etc/passwd",
		"/app/uploads/a.jpg",
		"/root/.config/rclone/rclone.conf",
		"uploads/a.jpg",
		"./uploads/a.jpg",
		"http://evil.example/x.jpg",
		"https://pos.nbp.co.id/uploads/2026-10/x.jpg",
		"//evil.example/uploads/x.jpg",
		"javascript:alert(1)",
		"data:image/png;base64,AAAA",
		`/uploads/a\..\..\b.jpg`,
		"/uploads/a.jpg\x00",
		"/uploads/a.jpg\n",
		"/uploads/\x7fa.jpg",
		"/uploads/%2e%2e/x.jpg",
		"/uploads//a.jpg",
		"/uploads/./a.jpg",
		"/uploads/a/.",
		"/uploads/",
		"/uploads",
		"/uploadsX/a.jpg",
		"/uploads/a.jpg?x=1",
		"/uploads/a.jpg#x",
		" /uploads/a.jpg",
		"/uploads/a b.jpg",
		"/uploads/fóto.jpg",
		"/uploads/a.exe",
		"/uploads/a.jpg.sh",
		"/uploads/a",
		"/uploads/" + strings.Repeat("a", 300) + ".jpg",
	}
	for _, u := range tolak {
		err := ValidateUploadURL(u)
		if err == nil {
			t.Errorf("%q: mau ditolak, ternyata sah", u)
			continue
		}
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%q: galat harus ValidationError (pesan 400), dapat %T", u, err)
		}
	}
}

func TestValidateUploadFieldMenyebutIsian(t *testing.T) {
	err := validateUploadField("foto serah terima", "/uploads/../.env")
	if err == nil || !strings.HasPrefix(err.Error(), "foto serah terima tidak sah:") {
		t.Fatalf("pesan = %v, mau diawali nama isian", err)
	}
}

func TestValidatePaymentProof(t *testing.T) {
	sah := []string{
		"",
		"TRF-20260410-001",
		"TRF-20260410-001 | /uploads/2026-10/01JABCDEFGHJKMNPQRSTVWXYZ0.jpg",
		"/uploads/2026-10/01JABCDEFGHJKMNPQRSTVWXYZ0.pdf",
		"TRF/2026/001",
		"REF 1 | REF 2",
	}
	for _, p := range sah {
		if err := ValidatePaymentProof(p); err != nil {
			t.Errorf("%q: mau sah, dapat %v", p, err)
		}
	}
	tolak := []string{
		"TRF-1 | /uploads/../.env",
		"TRF-1 | /etc/passwd",
		"/uploads/../../root/.config/rclone/rclone.conf",
		"TRF-1 | http://evil.example/x.jpg",
		"https://evil.example/bukti.jpg",
		`TRF-1 | \uploads\a.jpg`,
		"TRF-1 | //evil.example/x.jpg",
	}
	for _, p := range tolak {
		if err := ValidatePaymentProof(p); err == nil {
			t.Errorf("%q: mau ditolak, ternyata sah", p)
		}
	}
}

func TestUploadFilePathTerkurung(t *testing.T) {
	root := t.TempDir()
	got, err := uploadFilePathIn(root, "/uploads/a/b.jpg")
	if err != nil {
		t.Fatalf("URL sah ditolak: %v", err)
	}
	if want := filepath.Join(root, "a", "b.jpg"); got != want {
		t.Errorf("path = %q, mau %q", got, want)
	}
	if !filepath.IsAbs(got) || !strings.HasPrefix(got, root+string(filepath.Separator)) {
		t.Errorf("path %q harus absolut dan di bawah %q", got, root)
	}
	// Foto produk boleh GIF: pemetaan path tidak memeriksa ekstensi bukti.
	if _, err := uploadFilePathIn(root, "/uploads/products/x.gif"); err != nil {
		t.Errorf("foto produk GIF ditolak: %v", err)
	}
	for _, u := range []string{"../.env", "/uploads/../config", "/uploads/../../x.jpg", "/etc/passwd", "http://evil.example/x.jpg", "/uploads/"} {
		if p, err := uploadFilePathIn(root, u); err == nil {
			t.Errorf("%q: mau ditolak, dapat path %q", u, p)
		}
	}
	// Bawaan memakai folder kerja aplikasi.
	if p, err := UploadFilePath("/uploads/2026-10/x.jpg"); err != nil || !strings.HasSuffix(p, filepath.Join(UploadsDir, "2026-10", "x.jpg")) {
		t.Errorf("UploadFilePath = %q, %v", p, err)
	}
}

func TestPathInsideDir(t *testing.T) {
	dir := filepath.Join(string(filepath.Separator), "app", "uploads")
	kasus := map[string]bool{
		filepath.Join(dir, "a.jpg"):            true,
		filepath.Join(dir, "2026-10", "a.jpg"): true,
		dir:                                    false,
		filepath.Join(dir, "..", ".env"):       false,
		filepath.Join(string(filepath.Separator), "app", "uploads-lain", "a.jpg"): false,
		filepath.Join(string(filepath.Separator), "etc", "passwd"):                false,
	}
	for p, want := range kasus {
		if got := pathInsideDir(dir, filepath.Clean(p)); got != want {
			t.Errorf("pathInsideDir(%q) = %v, mau %v", p, got, want)
		}
	}
}

// Susunan berkas sungguhan: berkas biasa, folder, symlink ke luar/dalam, dan
// folder uploads yang dirinya sendiri symlink.
func TestResolveUploadFile(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "uploads")
	outside := filepath.Join(base, "luar")
	mustMkdir(t, filepath.Join(root, "2026-10"))
	mustMkdir(t, filepath.Join(root, "folder.jpg"))
	mustMkdir(t, outside)
	mustWrite(t, filepath.Join(root, "2026-10", "ok.jpg"))
	mustWrite(t, filepath.Join(outside, "rahasia.jpg"))
	mustWrite(t, filepath.Join(base, ".env"))
	mustSymlink(t, filepath.Join(outside, "rahasia.jpg"), filepath.Join(root, "keluar.jpg"))
	mustSymlink(t, filepath.Join(root, "2026-10", "ok.jpg"), filepath.Join(root, "dalam.jpg"))
	mustSymlink(t, outside, filepath.Join(root, "sub"))

	got, err := resolveUploadFileIn(root, "/uploads/2026-10/ok.jpg")
	if err != nil {
		t.Fatalf("berkas sah ditolak: %v", err)
	}
	realRoot, _ := filepath.EvalSymlinks(root)
	if want := filepath.Join(realRoot, "2026-10", "ok.jpg"); got != want {
		t.Errorf("path = %q, mau %q", got, want)
	}

	for _, u := range []string{
		"/uploads/2026-10/tidak-ada.jpg", // tidak ada
		"/uploads/folder.jpg",            // folder
		"/uploads/keluar.jpg",            // symlink ke luar uploads
		"/uploads/dalam.jpg",             // symlink (meski ke dalam) tetap ditolak
		"/uploads/sub/rahasia.jpg",       // folder symlink ke luar uploads
		"/uploads/../luar/rahasia.jpg",   // traversal
		"/uploads/../.env",
		"../.env",
		filepath.Join(outside, "rahasia.jpg"), // path absolut lain
		"http://evil.example/x.jpg",
	} {
		if p, err := resolveUploadFileIn(root, u); err == nil {
			t.Errorf("%q: mau ditolak, dapat %q", u, p)
		}
	}

	// Folder uploads yang berupa symlink (mis. volume terpasang) tetap jalan.
	linkRoot := filepath.Join(base, "uploads-link")
	mustSymlink(t, root, linkRoot)
	if _, err := resolveUploadFileIn(linkRoot, "/uploads/2026-10/ok.jpg"); err != nil {
		t.Errorf("folder uploads bersymlink: %v", err)
	}
}

// uploadPhotoToDrive harus menolak sebelum rclone dipanggil.
func TestUploadPhotoToDriveMenolakDiLuarUploads(t *testing.T) {
	for _, u := range []string{"/uploads/../../root/.config/rclone/rclone.conf", "/../.env", "/etc/passwd", "/uploads/2026-10/tidak-ada-sama-sekali.jpg"} {
		_, _, err := uploadPhotoToDrive(u)
		if err == nil || !strings.Contains(err.Error(), "tidak disalin") {
			t.Errorf("%q: galat = %v, mau ditolak tanpa disalin", u, err)
		}
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("isi"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink tidak didukung di sini: %v", err)
	}
}
