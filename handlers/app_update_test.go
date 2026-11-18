package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/gofiber/fiber/v2"
)

// Diresolusi saat paket dimuat (cwd = handlers/), sebelum tes berpindah folder.
var apkFixtureDir = func() string {
	dir, _ := filepath.Abs(filepath.Join("..", "internal", "apkinfo", "testdata"))
	return dir
}()

func manifestFixture(t *testing.T, version string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(apkFixtureDir, "pos-"+version+"-AndroidManifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Ganti nama paket di string pool (panjang sama) — APK lain di halaman App POS.
func withPackage(t *testing.T, manifest []byte, from, to string) []byte {
	t.Helper()
	enc16 := func(s string) []byte {
		var b bytes.Buffer
		for _, u := range utf16.Encode([]rune(s)) {
			b.WriteByte(byte(u))
			b.WriteByte(byte(u >> 8))
		}
		return b.Bytes()
	}
	for _, pair := range [][2][]byte{{[]byte(from), []byte(to)}, {enc16(from), enc16(to)}} {
		if bytes.Contains(manifest, pair[0]) {
			return bytes.ReplaceAll(manifest, pair[0], pair[1])
		}
	}
	t.Fatalf("paket %q tidak ditemukan di manifest", from)
	return nil
}

func writeAPK(t *testing.T, name string, manifest []byte, mod time.Time) string {
	t.Helper()
	path := filepath.Join(appFilesDir, name)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for entry, body := range map[string][]byte{"AndroidManifest.xml": manifest, "classes.dex": []byte(name)} {
		w, err := zw.Create(entry)
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
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

type posUpdateBody struct {
	Success bool `json:"success"`
	Data    struct {
		Latest *struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Size        int64  `json:"size"`
			Label       string `json:"label"`
			Package     string `json:"package"`
			VersionCode int64  `json:"version_code"`
			VersionName string `json:"version_name"`
			SHA256      string `json:"sha256"`
		} `json:"latest"`
		CurrentLabel    string `json:"current_label"`
		UpdateAvailable bool   `json:"update_available"`
	} `json:"data"`
}

func getPosUpdate(t *testing.T, query string) posUpdateBody {
	t.Helper()
	app := fiber.New()
	app.Get("/api/v1/app-update/pos", PosAppUpdate)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/app-update/pos"+query, nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d, cache %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	var body posUpdateBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPosAppUpdateOffersHighestPOSVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(appFilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	apkMetaCache = map[string]cachedAPKMeta{}
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	sum18 := writeAPK(t, "app-universal-release-v10.18.apk", manifestFixture(t, "v10.18"), base)
	// Build lama diunggah ulang BELAKANGAN: tetap bukan versi terbaru.
	writeAPK(t, "app-arm64-v8a-release-v10.17.apk", manifestFixture(t, "v10.17"), base.Add(48*time.Hour))
	// APK aplikasi lain (paket berbeda) & berkas non-APK tidak pernah ditawarkan.
	writeAPK(t, "aplikasi-lain-v99.apk",
		withPackage(t, manifestFixture(t, "v10.18"), "com.candradigital.pos_resto", "com.candradigital.pos_xxxxx"),
		base.Add(72*time.Hour))
	if err := os.WriteFile(filepath.Join(appFilesDir, "panduan-v1.zip"), []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	body := getPosUpdate(t, "?current=88512350")
	latest := body.Data.Latest
	if !body.Success || latest == nil {
		t.Fatalf("respons kosong: %+v", body)
	}
	if latest.Name != "app-universal-release-v10.18.apk" || latest.VersionCode != 93421153 ||
		latest.Label != "v10.18" || latest.Package != posPackageName || latest.SHA256 != sum18 ||
		latest.URL != "/uploads/apps/app-universal-release-v10.18.apk" || latest.VersionName != "1.2.0" {
		t.Fatalf("latest salah: %+v", latest)
	}
	if !body.Data.UpdateAvailable || body.Data.CurrentLabel != "v10.17" {
		t.Fatalf("status pembaruan salah: %+v", body.Data)
	}

	upToDate := getPosUpdate(t, "?current=93421153")
	if upToDate.Data.UpdateAvailable || upToDate.Data.CurrentLabel != "v10.18" {
		t.Fatalf("tablet terbaru tidak boleh ditawari pembaruan: %+v", upToDate.Data)
	}
}

func TestPosAppUpdateWithoutReleasesReturnsNull(t *testing.T) {
	t.Chdir(t.TempDir())
	apkMetaCache = map[string]cachedAPKMeta{}
	body := getPosUpdate(t, "")
	if !body.Success || body.Data.Latest != nil || body.Data.UpdateAvailable {
		t.Fatalf("tanpa APK harus null: %+v", body)
	}
}

func TestAPKMetaCacheFollowsFileChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(appFilesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	apkMetaCache = map[string]cachedAPKMeta{}
	name := "app-release-v10.20.apk"
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	writeAPK(t, name, manifestFixture(t, "v10.17"), base)
	files, _ := listAppFileInfos()
	if files[0].VersionCode != 88512350 {
		t.Fatalf("got %+v", files[0])
	}
	// Berkas diganti isinya (nama sama) → cache tidak boleh basi.
	writeAPK(t, name, manifestFixture(t, "v10.18"), base.Add(time.Hour))
	files, _ = listAppFileInfos()
	if files[0].VersionCode != 93421153 {
		t.Fatalf("cache basi: %+v", files[0])
	}
}

func TestAppVersionCodeFromHeartbeat(t *testing.T) {
	for in, want := range map[string]int64{"1.2.0+93421153": 93421153, "1.2.0": 0, "": 0, "1.2.0+x": 0} {
		if got := appVersionCode(in); got != want {
			t.Fatalf("%q → %d, want %d", in, got, want)
		}
	}
}
