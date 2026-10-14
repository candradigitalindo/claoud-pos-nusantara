package config

// Uji aturan token admin statis (T-1). Tanpa database.

import (
	"strings"
	"testing"
)

const tokenKuatUji = "3f9c1a7e5b2d4c6e8a0b1d3f5e7c9a2b4d6f8e0a1c3e5b7d9f2a4c6e8b0d1f3a"

func TestStaticAdminTokenProblem(t *testing.T) {
	lemah := map[string]string{
		"":                                      "kosong",
		"   ":                                   "kosong",
		"admin-secret-token":                    "bawaan",
		"ADMIN-SECRET-TOKEN":                    "bawaan",
		"admin-secret-token-admin-secret-token": "bawaan",
		"change-me-in-production-0123456789abc": "bawaan",
		"shared-secret-between-cloud-and-pos":   "bawaan",
		"3f9c1a7e5b2d4c6e8a0b1d3f5e7c9a2":       "pendek", // 31 karakter
		strings.Repeat("a", 64):                 "seragam",
		strings.Repeat("1212", 10):              "seragam",
	}
	for tok, jenis := range lemah {
		c := &Config{AdminToken: tok}
		if c.StaticAdminTokenProblem() == "" {
			t.Errorf("%q (%s): mau nonaktif, ternyata aktif", tok, jenis)
		}
		if c.StaticAdminTokenMatches(tok) {
			t.Errorf("%q (%s): token lemah tidak boleh diterima", tok, jenis)
		}
	}

	c := &Config{AdminToken: tokenKuatUji}
	if p := c.StaticAdminTokenProblem(); p != "" {
		t.Fatalf("token kuat dianggap bermasalah: %s", p)
	}
	if !c.StaticAdminTokenMatches(tokenKuatUji) {
		t.Error("token kuat yang sama persis harus diterima")
	}
	for _, salah := range []string{"", tokenKuatUji[:63], tokenKuatUji + "0", strings.ToUpper(tokenKuatUji), " " + tokenKuatUji, "admin-secret-token"} {
		if c.StaticAdminTokenMatches(salah) {
			t.Errorf("token %q tidak boleh cocok", salah)
		}
	}
}

// Tanpa ADMIN_TOKEN di lingkungan, tidak ada nilai bawaan yang diam-diam aktif.
func TestLoadTanpaAdminTokenBawaan(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "")
	c := Load()
	if c.AdminToken != "" {
		t.Fatalf("AdminToken bawaan = %q, mau kosong", c.AdminToken)
	}
	if c.StaticAdminTokenMatches("admin-secret-token") {
		t.Fatal("nilai bawaan lama masih diterima")
	}
}

// JWT_SECRET kosong (jatuh ke nilai bawaan) atau masih nilai contoh dari repo
// membuat siapa pun bisa menandatangani JWT sendiri — termasuk untuk akun
// superadmin bawaan yang id-nya tercantum di seeder. Nilai semacam itu
// ditandai supaya aplikasi memperingatkan saat start. Nilainya sengaja TIDAK
// diganti otomatis: kunci enkripsi kredensial kamera diturunkan dari
// JWT_SECRET bila CAMERA_ENC_KEY kosong, jadi rahasia acak per proses akan
// membuat kredensial RTSP tersimpan tidak bisa dibuka setelah restart.
func TestJWTSecretProblem(t *testing.T) {
	for _, lemah := range []string{"", "   ", "change-me-in-production", "your-cloud-jwt-secret-change-in-production", "rahasia-pendek"} {
		if p := (&Config{JWTSecret: lemah}).JWTSecretProblem(); p == "" {
			t.Errorf("JWT_SECRET %q tidak ditandai lemah", lemah)
		}
	}
	if p := (&Config{JWTSecret: tokenKuatUji}).JWTSecretProblem(); p != "" {
		t.Errorf("JWT_SECRET kuat ditandai bermasalah: %s", p)
	}
	t.Setenv("JWT_SECRET", "")
	if c := Load(); c.JWTSecretProblem() == "" {
		t.Error("tanpa JWT_SECRET, nilai bawaan tidak ditandai lemah")
	}
	t.Setenv("JWT_SECRET", tokenKuatUji)
	if c := Load(); c.JWTSecret != tokenKuatUji {
		t.Errorf("JWT_SECRET yang diisi diubah menjadi %q", c.JWTSecret)
	}
}
