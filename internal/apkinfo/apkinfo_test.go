package apkinfo

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// Manifest biner diambil dari APK rilis POS (dibangun Flutter/aapt2), versi
// & package-nya sudah dicocokkan dengan `aapt dump badging`.
func TestParseManifestFromReleasedPOS(t *testing.T) {
	cases := []struct {
		file string
		code int64
	}{
		{"pos-v10.18-AndroidManifest.bin", 93421153},
		{"pos-v10.17-AndroidManifest.bin", 88512350},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(filepath.Join("testdata", tc.file))
		if err != nil {
			t.Fatal(err)
		}
		info, err := ParseManifest(data)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if info.Package != "com.candradigital.pos_resto" || info.VersionCode != tc.code || info.VersionName != "1.2.0" {
			t.Fatalf("%s: got %+v", tc.file, info)
		}
	}
}

func TestReadFileFromAPKArchive(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("testdata", "pos-v10.18-AndroidManifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.apk")
	writeZip(t, path, map[string][]byte{"classes.dex": {1, 2, 3}, "AndroidManifest.xml": manifest})
	info, err := ReadFile(path)
	if err != nil || info.VersionCode != 93421153 {
		t.Fatalf("got %+v, %v", info, err)
	}

	missing := filepath.Join(t.TempDir(), "kosong.apk")
	writeZip(t, missing, map[string][]byte{"classes.dex": {1}})
	if _, err := ReadFile(missing); err == nil {
		t.Fatal("APK tanpa manifest harus ditolak")
	}
	notZip := filepath.Join(t.TempDir(), "bukan.apk")
	if err := os.WriteFile(notZip, []byte("bukan zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(notZip); err == nil {
		t.Fatal("berkas non-zip harus ditolak")
	}
}

// Data rusak/terpotong tidak boleh membuat server panik.
func TestParseManifestRejectsCorruptInputWithoutPanic(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "pos-v10.18-AndroidManifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 7, 8, 40, 300, len(data) / 2, len(data) - 1} {
		_, _ = ParseManifest(data[:n])
	}
	corrupt := append([]byte(nil), data...)
	for i := 8; i < len(corrupt); i += 97 {
		corrupt[i] ^= 0xff
	}
	_, _ = ParseManifest(corrupt)
	if _, err := ParseManifest([]byte("PK\x03\x04 bukan manifest")); err == nil {
		t.Fatal("format salah harus ditolak")
	}
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
