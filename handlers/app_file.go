package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud-pos/models"

	"github.com/gofiber/fiber/v2"
)

const appFilesDir = "uploads/apps"

var (
	appFileNameRe   = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	appAllowedExt   = map[string]bool{".apk": true, ".aab": true, ".ipa": true, ".zip": true, ".exe": true, ".dmg": true}
	maxAppFileBytes = int64(200 * 1024 * 1024) // 200MB
	versionSuffixRe = regexp.MustCompile(`-v(\d+)(?:\.(\d+))?$`) // suffix versi di nama dasar: -v10 / -v10.2
)

func sanitizeAppFileName(name string) string {
	name = filepath.Base(name)                  // strip any path
	name = strings.ReplaceAll(name, " ", "-")   // no spaces in URL
	name = appFileNameRe.ReplaceAllString(name, "_")
	name = strings.Trim(name, ".-_")
	if name == "" {
		name = "file"
	}
	return name
}

// versionedAppFileName menamai file upload dengan versi berurutan major.minor:
// setelah `-v10` upload berikutnya menjadi `-v10.1`, lalu `-v10.2`, dst — build
// baru tidak menimpa yang lama dan versinya tampak dari nama file/URL.
// Naik MAJOR dilakukan eksplisit: beri nama file upload dengan versi yang lebih
// tinggi (mis. `app-v11.apk`) dan versi itu dipakai apa adanya; minor berikutnya
// melanjutkan dari sana (v11.1, v11.2, …).
func versionedAppFileName(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	// Versi eksplisit di nama upload (bila ada), lalu buang dari base.
	expMaj, expMin, hasExplicit := 0, 0, false
	if m := versionSuffixRe.FindStringSubmatch(base); m != nil {
		expMaj, _ = strconv.Atoi(m[1])
		if m[2] != "" {
			expMin, _ = strconv.Atoi(m[2])
		}
		hasExplicit = true
		base = versionSuffixRe.ReplaceAllString(base, "")
	}
	if base == "" {
		base = "file"
	}

	// Versi tertinggi yang sudah ada untuk nama dasar ini (dukung -vN dan -vN.M).
	maxMaj, maxMin, found := 0, 0, false
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `-v(\d+)(?:\.(\d+))?` + regexp.QuoteMeta(ext) + `$`)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			m := re.FindStringSubmatch(e.Name())
			if m == nil {
				continue
			}
			maj, _ := strconv.Atoi(m[1])
			min := 0
			if m[2] != "" {
				min, _ = strconv.Atoi(m[2])
			}
			if !found || maj > maxMaj || (maj == maxMaj && min > maxMin) {
				maxMaj, maxMin, found = maj, min, true
			}
		}
	}

	// Versi eksplisit yang lebih tinggi dari semua yang ada → rilis baru, pakai apa adanya.
	if hasExplicit && (!found || expMaj > maxMaj || (expMaj == maxMaj && expMin > maxMin)) {
		if expMin > 0 {
			return fmt.Sprintf("%s-v%d.%d%s", base, expMaj, expMin, ext)
		}
		return fmt.Sprintf("%s-v%d%s", base, expMaj, ext)
	}
	if !found {
		return fmt.Sprintf("%s-v1%s", base, ext)
	}
	// Lanjutan berurutan: v10 → v10.1 → v10.2 …
	return fmt.Sprintf("%s-v%d.%d%s", base, maxMaj, maxMin+1, ext)
}

type appFileInfo struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Size       int64  `json:"size"`
	UploadedAt string `json:"uploaded_at"`
	// Label versi dari nama berkas (mis. "v10.18").
	Label string `json:"label,omitempty"`
	// Identitas APK dibaca dari AndroidManifest (kosong untuk non-APK).
	Package     string `json:"package,omitempty"`
	VersionCode int64  `json:"version_code,omitempty"`
	VersionName string `json:"version_name,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// ListAppFiles lists distributable app files (APK, etc.) with their download URLs.
func ListAppFiles(c *fiber.Ctx) error {
	files, err := listAppFileInfos()
	if err != nil {
		return c.JSON(models.APIResponse{Success: true, Data: []appFileInfo{}})
	}
	return c.JSON(models.APIResponse{Success: true, Data: files})
}

// UploadAppFile saves an uploaded app file and returns its download URL.
func UploadAppFile(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "File tidak ditemukan."})
	}
	if file.Size > maxAppFileBytes {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Ukuran file maksimal 200MB."})
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !appAllowedExt[ext] {
		return c.Status(400).JSON(models.APIResponse{Success: false, Error: "Format tidak didukung. Gunakan APK, AAB, IPA, ZIP, EXE, atau DMG."})
	}

	if err := os.MkdirAll(appFilesDir, 0755); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyiapkan folder."})
	}

	name := versionedAppFileName(appFilesDir, sanitizeAppFileName(file.Filename))
	savePath := filepath.Join(appFilesDir, name)
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menyimpan file."})
	}

	// Baca versi APK sekarang juga (sekaligus mengisi cache) agar tablet yang
	// memeriksa pembaruan langsung mendapat versi baru tanpa jeda.
	data := appFileInfo{
		Name:       name,
		URL:        "/uploads/apps/" + name,
		Size:       file.Size,
		UploadedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Label:      releaseLabel(name),
	}
	if info, err := os.Stat(savePath); err == nil {
		data = newAppFileInfo(name, info)
	}
	return c.Status(201).JSON(models.APIResponse{Success: true, Data: data})
}

// DownloadAppFile serves an app file with the correct MIME type and a download
// disposition, so an APK is saved as ".apk" (not ".zip", which the generic ZIP
// content-sniffer would otherwise produce). Public — the URL is shareable.
func DownloadAppFile(c *fiber.Ctx) error {
	name := sanitizeAppFileName(c.Params("name"))
	path := filepath.Join(appFilesDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return c.Status(404).SendString("File tidak ditemukan")
	}

	ctype := "application/octet-stream"
	if strings.ToLower(filepath.Ext(name)) == ".apk" {
		ctype = "application/vnd.android.package-archive"
	}

	f, err := os.Open(path)
	if err != nil {
		return c.Status(500).SendString("Gagal membuka file")
	}
	c.Set("Content-Type", ctype)
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Set("Cache-Control", "public, max-age=300")
	return c.SendStream(f, int(info.Size())) // SendStream closes the file when done
}

// DeleteAppFile removes an app file by name.
func DeleteAppFile(c *fiber.Ctx) error {
	name := sanitizeAppFileName(c.Params("name"))
	path := filepath.Join(appFilesDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return c.Status(404).JSON(models.APIResponse{Success: false, Error: "File tidak ditemukan."})
		}
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal menghapus file."})
	}
	forgetAPKMeta(name)
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{"deleted": name}})
}

