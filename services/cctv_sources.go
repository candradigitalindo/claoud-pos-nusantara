package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud-pos/models"
)

var ErrCameraValidation = errors.New("konfigurasi kamera tidak valid")
var ErrCCTVStreamLimit = errors.New("batas tayangan CCTV tercapai")
var ErrPlaybackUnsupported = errors.New("perangkat ini belum menyediakan koneksi rekaman; tambahkan URL playback yang didukung perangkat")

type CCTVConnectionOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Port     int    `json:"port"`
	NeedsURL bool   `json:"needs_url"`
	Help     string `json:"help"`
}

var CCTVConnections = []CCTVConnectionOption{
	{"hikvision", "Hikvision / HiLook", 554, false, "Untuk perangkat Hikvision/HiLook, termasuk yang memakai Hik-Connect. Isi IP dan akun perangkat, bukan akun aplikasi."},
	{"dahua", "Dahua / Imou (RTSP)", 554, false, "Memerlukan RTSP pada model perangkat. Isi IP kamera atau DVR/NVR dan akun perangkat, bukan akun aplikasi."},
	{"tapo", "TP-Link Tapo (RTSP)", 554, false, "Gunakan akun kamera yang dibuat di aplikasi Tapo, bukan akun login aplikasi."},
	{"reolink", "Reolink (RTSP)", 554, false, "Aktifkan RTSP pada model Reolink yang mendukung."},
	{"onvif", "ONVIF — berbagai merek", 80, false, "Aktifkan ONVIF, lalu cari profil untuk memilih kamera/channel dan kualitas yang tersedia pada perangkat."},
	{"rtsp", "RTSP manual — berbagai merek", 554, true, "Masukkan URL RTSP dari dokumentasi model kamera."},
	{"dvrip", "DVRIP — iCSee / XMEye yang kompatibel", 34567, false, "Channel di formulir dimulai dari 1; sistem mengubahnya ke nomor channel DVRIP."},
	{"tuya", "Tuya Smart — cloud tanpa penghubung", 0, true, "Hubungkan akun Tuya Smart untuk memilih kamera, atau masukkan URL konektor Tuya yang valid."},
	{"vendor", "Konektor vendor lainnya", 0, true, "URL konektor go2rtc: tapo://, xiaomi://, bubble:// atau tuya://. UID aplikasi saja tidak cukup."},
}

var CCTVBrands = []string{"Hikvision", "HiLook", "HK", "HKM", "Dahua", "Imou", "EZVIZ", "TP-Link Tapo", "Reolink", "Xiaomi", "Tuya", "iCSee", "XMEye", "V380", "Yoosee", "EseeCloud", "Lainnya"}

func cameraConnection(cam models.OutletCamera) string {
	if cam.ConnectionType != "" {
		return strings.ToLower(strings.TrimSpace(cam.ConnectionType))
	}
	switch strings.ToLower(strings.TrimSpace(cam.Brand)) {
	case "", "hikvision", "hilook":
		return "hikvision"
	case "dahua":
		return "dahua"
	}
	return ""
}

func connectionOption(kind string) (CCTVConnectionOption, bool) {
	for _, option := range CCTVConnections {
		if option.ID == kind {
			return option, true
		}
	}
	return CCTVConnectionOption{}, false
}

func validateCameraConfig(cam *models.OutletCamera, source, playback string) error {
	cam.Name, cam.Host, cam.Brand = strings.TrimSpace(cam.Name), strings.TrimSpace(cam.Host), strings.ToLower(strings.TrimSpace(cam.Brand))
	if cam.Name == "" || len(cam.Name) > 100 {
		return fmt.Errorf("%w: nama wajib diisi, maksimal 100 karakter", ErrCameraValidation)
	}
	if cam.Brand == "" {
		cam.Brand = "hikvision"
	}
	if len(cam.Brand) > 20 {
		return fmt.Errorf("%w: nama merek terlalu panjang", ErrCameraValidation)
	}
	if len(cam.RTSPUsername) > 80 || cam.SortOrder < 0 || cam.SortOrder > 100000 {
		return fmt.Errorf("%w: username maksimal 80 karakter dan urutan harus 0–100000", ErrCameraValidation)
	}
	if cam.Timezone == "" {
		cam.Timezone = "Asia/Jakarta"
	}
	if len(cam.Timezone) > 64 {
		return fmt.Errorf("%w: zona waktu kamera tidak valid", ErrCameraValidation)
	}
	if _, err := time.LoadLocation(cam.Timezone); err != nil {
		return fmt.Errorf("%w: zona waktu kamera tidak valid", ErrCameraValidation)
	}
	cam.ConnectionType = cameraConnection(*cam)
	option, ok := connectionOption(cam.ConnectionType)
	if !ok {
		return fmt.Errorf("%w: pilih jenis koneksi sesuai model kamera", ErrCameraValidation)
	}
	if cam.AccessMode == "" {
		cam.AccessMode = "direct"
	} // kompatibilitas konfigurasi lama
	if cam.AccessMode != "direct" && cam.AccessMode != "gateway" {
		return fmt.Errorf("%w: jalur koneksi tidak dikenal", ErrCameraValidation)
	}
	if cam.Channel <= 0 {
		cam.Channel = 1
	}
	if cam.Channel > 256 || cam.Subtype < 0 || cam.Subtype > 1 {
		return fmt.Errorf("%w: channel atau kualitas stream tidak valid", ErrCameraValidation)
	}
	if cam.Port == 0 {
		cam.Port = option.Port
	}
	if cam.Port < 0 || cam.Port > 65535 {
		return fmt.Errorf("%w: port harus antara 1 dan 65535", ErrCameraValidation)
	}
	if option.NeedsURL || source != "" {
		if err := validateCameraSourceURL(source, cam.ConnectionType); err != nil {
			return err
		}
	} else if !validCameraHost(cam.Host) {
		return fmt.Errorf("%w: isi IP atau hostname perangkat tanpa skema/port/path", ErrCameraValidation)
	}
	if playback != "" {
		sample := strings.NewReplacer("{start}", "20261019T100000Z", "{end}", "20261019T110000Z", "{start_iso}", "2026-10-19T10:00:00", "{end_iso}", "2026-10-19T11:00:00", "{start_unix}", "1792396800", "{end_unix}", "1792400400").Replace(playback)
		if err := validateCameraSourceURL(sample, "rtsp"); err != nil {
			return fmt.Errorf("%w: URL playback harus berupa RTSP yang valid", ErrCameraValidation)
		}
	}
	return nil
}

func validCameraHost(host string) bool {
	if host == "" || len(host) > 120 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

func validateCameraSourceURL(raw, kind string) error {
	if len(raw) > 8192 {
		return fmt.Errorf("%w: URL sumber terlalu panjang", ErrCameraValidation)
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\x00") {
		return fmt.Errorf("%w: URL sumber tidak valid", ErrCameraValidation)
	}
	allowed := false
	switch kind {
	case "rtsp":
		allowed = u.Scheme == "rtsp" || u.Scheme == "rtsps"
	case "onvif":
		allowed = u.Scheme == "onvif"
	case "tuya":
		allowed = u.Scheme == "tuya" && validTuyaHost(u.Hostname())
	case "vendor":
		allowed = u.Scheme == "tapo" || u.Scheme == "xiaomi" || u.Scheme == "bubble" || u.Scheme == "tuya" && validTuyaHost(u.Hostname())
	default:
		allowed = u.Scheme == "rtsp" || u.Scheme == "rtsps"
	}
	if !allowed {
		return fmt.Errorf("%w: protokol URL tidak sesuai jenis koneksi", ErrCameraValidation)
	}
	return nil
}

func BuildCameraSource(cam models.OutletCamera, password, source string) (string, error) {
	kind := cameraConnection(cam)
	option, ok := connectionOption(kind)
	if !ok {
		return "", fmt.Errorf("%w: jenis koneksi tidak dikenal", ErrCameraValidation)
	}
	if source != "" {
		if err := validateCameraSourceURL(source, kind); err != nil {
			return "", err
		}
		return source, nil
	}
	if option.NeedsURL {
		return "", fmt.Errorf("%w: URL sumber wajib diisi", ErrCameraValidation)
	}
	port := cam.Port
	if port <= 0 {
		port = option.Port
	}
	u := &url.URL{Scheme: "rtsp", Host: net.JoinHostPort(cam.Host, strconv.Itoa(port))}
	if cam.RTSPUsername != "" {
		u.User = url.UserPassword(cam.RTSPUsername, password)
	}
	switch kind {
	case "hikvision", "dahua":
		cam.Brand = kind
		return BuildLiveRTSP(cam, password), nil
	case "tapo":
		u.Path = "/stream1"
		if cam.Subtype == 1 {
			u.Path = "/stream2"
		}
	case "reolink":
		quality := "main"
		if cam.Subtype == 1 {
			quality = "sub"
		}
		u.Path = fmt.Sprintf("/h264Preview_%02d_%s", cam.Channel, quality)
	case "onvif":
		u.Scheme = "onvif"
		u.RawQuery = fmt.Sprintf("subtype=%d", cam.Subtype)
	case "dvrip":
		u.Scheme = "dvrip"
		u.RawQuery = fmt.Sprintf("channel=%d&subtype=%d", max(cam.Channel-1, 0), cam.Subtype)
	}
	return u.String(), nil
}

func BuildCameraPlayback(cam models.OutletCamera, password, template, start, end string) (string, error) {
	if template == "" {
		kind := cameraConnection(cam)
		if kind != "hikvision" && kind != "dahua" {
			return "", ErrPlaybackUnsupported
		}
		cam.Brand = kind
		return BuildPlaybackRTSP(cam, password, start, end)
	}
	st, err := parseCameraPlaybackTime(cam, start)
	if err != nil {
		return "", fmt.Errorf("%w: waktu mulai tidak valid", ErrCameraValidation)
	}
	et, err := parseCameraPlaybackTime(cam, end)
	if err != nil || !et.After(st) {
		return "", fmt.Errorf("%w: waktu selesai harus setelah waktu mulai", ErrCameraValidation)
	}
	src := strings.NewReplacer("{start}", st.UTC().Format("20060102T150405Z"), "{end}", et.UTC().Format("20060102T150405Z"), "{start_iso}", st.Format("2006-01-02T15:04:05"), "{end_iso}", et.Format("2006-01-02T15:04:05"), "{start_unix}", strconv.FormatInt(st.Unix(), 10), "{end_unix}", strconv.FormatInt(et.Unix(), 10)).Replace(template)
	if err := validateCameraSourceURL(src, "rtsp"); err != nil {
		return "", err
	}
	return src, nil
}

// resolveCameraSource memilih profil ONVIF yang tersedia. Kamera murah dengan
// satu profil tetap dapat dipakai saat operator memilih sub-stream default.
// URL profil berkredensial tidak pernah dikirim ke browser.
func resolveCameraSource(cam models.OutletCamera, password, source string) (string, error) {
	src, err := BuildCameraSource(cam, password, source)
	if err != nil {
		return "", err
	}
	if cameraConnection(cam) != "onvif" || source != "" {
		return src, nil
	}
	profiles, err := listCameraONVIFProfiles(cam, src)
	if err != nil {
		return "", err
	}
	if len(profiles) != 1 {
		return "", fmt.Errorf("%w: perangkat memiliki beberapa profil ONVIF; ubah kamera, cari profil, lalu pilih kamera/channel yang ingin ditonton", ErrCameraValidation)
	}
	return profiles[0].URL, nil
}

type cameraONVIFProfile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func listCameraONVIFProfiles(cam models.OutletCamera, src string) ([]cameraONVIFProfile, error) {
	base, _ := url.Parse(src)
	query := base.Query()
	query.Del("subtype")
	query.Del("snapshot")
	base.RawQuery = query.Encode()
	response, err := cameraRelayRequest(cam, http.MethodGet, "/api/onvif", url.Values{"src": {base.String()}})
	if err != nil {
		return nil, err
	}
	if response.Status != http.StatusOK {
		return nil, ErrCCTVRelayUnavailable
	}
	var profiles struct {
		Sources []cameraONVIFProfile `json:"sources"`
	}
	if json.Unmarshal(response.Body, &profiles) != nil {
		return nil, ErrCCTVRelayUnavailable
	}
	valid := []cameraONVIFProfile{}
	for _, profile := range profiles.Sources {
		u, err := url.Parse(profile.URL)
		if err == nil && u.Host == base.Host && u.Scheme == "onvif" && !u.Query().Has("snapshot") && validateCameraSourceURL(profile.URL, "onvif") == nil {
			valid = append(valid, profile)
		}
	}
	if len(valid) == 0 {
		return nil, ErrCCTVRelayUnavailable
	}
	return valid, nil
}

func DiscoverCameraONVIFProfiles(adminID, outletID string, req models.CameraProbeRequest) ([]CCTVCloudDevice, error) {
	cam, password, source, err := prepareCameraProbe(outletID, req)
	if err != nil {
		return nil, err
	}
	if cameraConnection(cam) != "onvif" {
		return nil, fmt.Errorf("%w: pilih koneksi ONVIF untuk mencari profil", ErrCameraValidation)
	}
	src, err := BuildCameraSource(cam, password, source)
	if err != nil {
		return nil, err
	}
	profiles, err := listCameraONVIFProfiles(cam, src)
	if err != nil {
		return nil, err
	}
	items := []CCTVCloudDevice{}
	for index, profile := range profiles {
		ticket, err := issueCCTVSourceTicket(profile.URL, "onvif", adminID, outletID)
		if err != nil {
			return nil, err
		}
		name := profile.Name
		if name == "" {
			name = fmt.Sprintf("Profil %d", index+1)
		}
		items = append(items, CCTVCloudDevice{ID: strconv.Itoa(index), Name: name, Ticket: ticket})
	}
	return items, nil
}
