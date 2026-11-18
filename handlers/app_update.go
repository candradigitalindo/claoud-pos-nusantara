package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud-pos/internal/apkinfo"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Paket aplikasi kasir. Hanya APK dengan paket ini yang ditawarkan sebagai
// pembaruan ke tablet — file lain di halaman App POS (ZIP, EXE, APK lain)
// tidak pernah terpasang lewat pembaruan dalam-aplikasi.
const posPackageName = "com.candradigital.pos_resto"

type apkMeta struct {
	Package     string
	VersionCode int64
	VersionName string
	SHA256      string
}

type cachedAPKMeta struct {
	size    int64
	modTime time.Time
	meta    apkMeta
	ok      bool
}

var (
	// Dipegang selama membaca APK: setelah restart, banyak tablet yang memeriksa
	// bersamaan tidak membaca ulang ratusan MB secara paralel — yang pertama
	// menghitung, sisanya memakai cache.
	apkMetaMu    sync.Mutex
	apkMetaCache = map[string]cachedAPKMeta{}

	releaseLabelRe = regexp.MustCompile(`(?i)-v(\d+(?:\.\d+)*)\.[a-z0-9]+$`)
)

// apkMetaFor membaca identitas & SHA-256 APK (di-cache per nama+ukuran+waktu).
// ok=false untuk berkas non-APK atau APK yang tidak terbaca.
func apkMetaFor(name string, info os.FileInfo) (apkMeta, bool) {
	if strings.ToLower(filepath.Ext(name)) != ".apk" {
		return apkMeta{}, false
	}
	apkMetaMu.Lock()
	defer apkMetaMu.Unlock()
	if c, hit := apkMetaCache[name]; hit && c.size == info.Size() && c.modTime.Equal(info.ModTime()) {
		return c.meta, c.ok
	}
	meta, ok := readAPKMeta(filepath.Join(appFilesDir, name))
	apkMetaCache[name] = cachedAPKMeta{size: info.Size(), modTime: info.ModTime(), meta: meta, ok: ok}
	return meta, ok
}

func readAPKMeta(path string) (apkMeta, bool) {
	info, err := apkinfo.ReadFile(path)
	if err != nil {
		return apkMeta{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return apkMeta{}, false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return apkMeta{}, false
	}
	return apkMeta{
		Package:     info.Package,
		VersionCode: info.VersionCode,
		VersionName: info.VersionName,
		SHA256:      hex.EncodeToString(h.Sum(nil)),
	}, true
}

func forgetAPKMeta(name string) {
	apkMetaMu.Lock()
	delete(apkMetaCache, name)
	apkMetaMu.Unlock()
}

// releaseLabel: "v10.18" dari nama berkas berversi (app-...-v10.18.apk).
func releaseLabel(name string) string {
	if m := releaseLabelRe.FindStringSubmatch(name); m != nil {
		return "v" + m[1]
	}
	return ""
}

func newAppFileInfo(name string, info os.FileInfo) appFileInfo {
	f := appFileInfo{
		Name:       name,
		URL:        "/uploads/apps/" + name,
		Size:       info.Size(),
		UploadedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		Label:      releaseLabel(name),
	}
	if meta, ok := apkMetaFor(name, info); ok {
		f.Package = meta.Package
		f.VersionCode = meta.VersionCode
		f.VersionName = meta.VersionName
		f.SHA256 = meta.SHA256
	}
	return f
}

// listAppFileInfos — semua berkas distribusi (terbaru dulu) beserta identitas
// APK-nya. Folder yang belum ada = daftar kosong.
func listAppFileInfos() ([]appFileInfo, error) {
	entries, err := os.ReadDir(appFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []appFileInfo{}, nil
		}
		return nil, err
	}
	files := make([]appFileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, newAppFileInfo(e.Name(), info))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].UploadedAt > files[j].UploadedAt })
	return files, nil
}

// latestPosRelease — APK POS dengan versionCode tertinggi (bukan sekadar yang
// terakhir diunggah: mengunggah ulang build lama tidak boleh menurunkan versi
// tablet).
func latestPosRelease(files []appFileInfo) *appFileInfo {
	var best *appFileInfo
	for i := range files {
		f := &files[i]
		if f.Package != posPackageName || f.VersionCode <= 0 {
			continue
		}
		if best == nil || f.VersionCode > best.VersionCode ||
			(f.VersionCode == best.VersionCode && f.UploadedAt > best.UploadedAt) {
			best = f
		}
	}
	return best
}

// appVersionCode: "1.2.0+93421153" (app_version heartbeat) → 93421153.
func appVersionCode(v string) int64 {
	if i := strings.LastIndex(v, "+"); i >= 0 {
		if n, err := strconv.ParseInt(strings.TrimSpace(v[i+1:]), 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// PosAppUpdate — PUBLIK: versi APK POS terbaru untuk pembaruan dalam-aplikasi.
// `?current=<versionCode>` (opsional) menambahkan label versi yang terpasang.
// Tidak membuka data baru: APK-nya memang sudah bisa diunduh publik dari
// /uploads/apps. Tanpa auth supaya tablet station (tanpa API key) juga bisa.
func PosAppUpdate(c *fiber.Ctx) error {
	files, err := listAppFileInfos()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal membaca daftar aplikasi."})
	}
	latest := latestPosRelease(files)
	current, _ := strconv.ParseInt(strings.TrimSpace(c.Query("current")), 10, 64)
	currentLabel := ""
	if current > 0 {
		for _, f := range files {
			if f.Package == posPackageName && f.VersionCode == current {
				currentLabel = f.Label
				break
			}
		}
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"latest":           latest,
		"current_label":    currentLabel,
		"update_available": latest != nil && latest.VersionCode > current,
	}})
}

type appDeviceVersion struct {
	OutletID    string `json:"outlet_id"`
	OutletName  string `json:"outlet_name"`
	HasData     bool   `json:"has_data"`
	Status      string `json:"status"`
	AppVersion  string `json:"app_version"`
	VersionCode int64  `json:"version_code"`
	Label       string `json:"label"`
	UpToDate    bool   `json:"up_to_date"`
	Model       string `json:"model"`
	OS          string `json:"os"`
	LastSeen    string `json:"last_seen"`
}

// AppFileDevices — versi aplikasi di tablet tiap outlet (dari heartbeat)
// dibanding APK POS terbaru. Admin, mengikuti cakupan outlet akun.
func AppFileDevices(c *fiber.Ctx) error {
	files, err := listAppFileInfos()
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal membaca daftar aplikasi."})
	}
	latest := latestPosRelease(files)
	labels := map[int64]string{}
	for _, f := range files {
		if f.Package == posPackageName && f.VersionCode > 0 && f.Label != "" {
			labels[f.VersionCode] = f.Label
		}
	}
	report, err := services.GetDeviceMonitor("", getOutletScope(c))
	if err != nil {
		return c.Status(500).JSON(models.APIResponse{Success: false, Error: "Gagal memuat perangkat: " + err.Error()})
	}
	devices := make([]appDeviceVersion, 0, len(report.Devices))
	for _, d := range report.Devices {
		code := appVersionCode(d.AppVersion)
		devices = append(devices, appDeviceVersion{
			OutletID:    d.OutletID,
			OutletName:  d.OutletName,
			HasData:     d.HasData,
			Status:      d.Status,
			AppVersion:  d.AppVersion,
			VersionCode: code,
			Label:       labels[code],
			UpToDate:    latest != nil && code > 0 && code >= latest.VersionCode,
			Model:       d.Model,
			OS:          d.OS,
			LastSeen:    d.ReceivedAt,
		})
	}
	return c.JSON(models.APIResponse{Success: true, Data: fiber.Map{
		"latest":  latest,
		"devices": devices,
	}})
}
