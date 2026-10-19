package services

import (
	"cloud-pos/models"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrCCTVAccessDenied     = errors.New("akses stream kamera tidak diizinkan")
	ErrCameraInactive       = errors.New("kamera nonaktif")
	ErrCCTVRelayUnavailable = errors.New("stream belum tersedia; periksa koneksi kamera dan layanan relay")
	cctvCameraLoader        = GetCamera
	cctvPermissionLoader    = GetRolePermissions
)

// Scope dan izin dibaca kembali pada setiap permintaan media, termasuk segmen
// HLS. Cookie tidak pernah menjadi izin global ke seluruh API go2rtc.
func authorizeCCTVStream(token, name string) (*AdminSession, error) {
	sess, err := authenticateCCTVToken(token)
	if err != nil {
		return nil, err
	}
	id, err := CCTVStreamCameraID(name)
	if err != nil {
		return nil, err
	}
	cam, err := cctvCameraLoader(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCCTVAccessDenied
	}
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	if !cam.IsActive {
		return nil, ErrCameraInactive
	}
	if sess.ScopeType != "all" {
		allowed := false
		for _, outlet := range sess.OutletIDs {
			allowed = allowed || outlet == cam.OutletID
		}
		if !allowed {
			return nil, ErrCCTVAccessDenied
		}
	}
	if sess.Role != "superadmin" {
		perms, err := cctvPermissionLoader(sess.Role)
		if err != nil {
			return nil, ErrAuthUnavailable
		}
		allowed := false
		for _, perm := range perms {
			// Sama dengan RequirePermission("cameras.view").
			allowed = allowed || strings.HasPrefix(perm, "cameras.")
		}
		if !allowed {
			return nil, ErrCCTVAccessDenied
		}
	}
	return sess, nil
}

type cctvHLSSession struct {
	stream string
	admin  string
	used   time.Time
}

var cctvHLS = struct {
	sync.Mutex
	sessions map[string]cctvHLSSession
}{sessions: make(map[string]cctvHLSSession)}

const cctvHLSIdleTTL = 2 * time.Minute

var cctvMediaTypes = map[string]string{
	"/api/stream.m3u8":       "application/vnd.apple.mpegurl",
	"/api/hls/playlist.m3u8": "application/vnd.apple.mpegurl",
	"/api/hls/segment.ts":    "video/mp2t",
	"/api/hls/init.mp4":      "video/mp4",
	"/api/hls/segment.m4s":   "video/iso.segment",
}

// CCTVMediaSource memetakan URI media ke stream milik admin. ID HLS dari
// playlist go2rtc diikat ke stream + admin, sehingga menebak ID sesi outlet
// lain tidak dapat melewati validasi scope melalui endpoint segmen.
func CCTVMediaSource(token, path, rawQuery string) (string, error) {
	if _, ok := cctvMediaTypes[path]; !ok {
		return "", ErrCCTVAccessDenied
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", ErrCCTVAccessDenied
	}
	for key, values := range query {
		if len(values) != 1 || (key != "src" && key != "id" && key != "n" && key != "mp4") {
			return "", ErrCCTVAccessDenied
		}
	}
	if path == "/api/stream.m3u8" {
		if query.Has("id") || query.Has("n") {
			return "", ErrCCTVAccessDenied
		}
		name := query.Get("src")
		_, err := authorizeCCTVStream(token, name)
		if err == nil {
			_, err = cameraForCCTVStream(name)
		}
		return name, err
	}
	if query.Has("src") || query.Has("mp4") {
		return "", ErrCCTVAccessDenied
	}
	sess, err := authenticateCCTVToken(token)
	if err != nil {
		return "", err
	}
	cctvHLS.Lock()
	hls, ok := cctvHLS.sessions[query.Get("id")]
	if ok && hls.admin == sess.ID && time.Since(hls.used) < cctvHLSIdleTTL {
		hls.used = time.Now()
		cctvHLS.sessions[query.Get("id")] = hls
	} else {
		ok = false
	}
	cctvHLS.Unlock()
	if !ok {
		return "", ErrCCTVAccessDenied
	}
	_, err = authorizeCCTVStream(token, hls.stream)
	if err == nil {
		_, err = cameraForCCTVStream(hls.stream)
	}
	return hls.stream, err
}

// RelayCCTVMedia hanya meneruskan lima endpoint HLS, tanpa cookie/header
// admin ke go2rtc. Tidak ada akses browser ke daftar sumber, kredensial,
// konfigurasi, pembuatan stream, atau URL RTSP mentah.
func RelayCCTVMedia(token, path, rawQuery string) ([]byte, string, error) {
	name, err := CCTVMediaSource(token, path, rawQuery)
	if err != nil {
		return nil, "", err
	}
	query, _ := url.ParseQuery(rawQuery)
	cam, err := cameraForCCTVStream(name)
	if err != nil {
		return nil, "", err
	}
	resp, err := cameraRelayRequest(*cam, http.MethodGet, path, query)
	if err != nil {
		return nil, "", err
	}
	if resp.Status != http.StatusOK {
		return nil, "", ErrCCTVRelayUnavailable
	}
	body := resp.Body
	if path == "/api/stream.m3u8" {
		if err := bindCCTVHLS(token, name, body); err != nil {
			return nil, "", err
		}
	}
	return body, cctvMediaTypes[path], nil
}

func cctvCameraLoaderName(name string) (*models.OutletCamera, error) {
	id, err := CCTVStreamCameraID(name)
	if err != nil {
		return nil, err
	}
	return cctvCameraLoader(id)
}

func bindCCTVHLS(token, name string, body []byte) error {
	sess, err := authenticateCCTVToken(token)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil || u.IsAbs() || u.Host != "" || u.Path != "hls/playlist.m3u8" || u.Query().Get("id") == "" {
			return ErrCCTVRelayUnavailable
		}
		now := time.Now()
		cctvHLS.Lock()
		for id, entry := range cctvHLS.sessions {
			if now.Sub(entry.used) >= cctvHLSIdleTTL {
				delete(cctvHLS.sessions, id)
			}
		}
		if len(cctvHLS.sessions) >= 4096 {
			cctvHLS.Unlock()
			return fmt.Errorf("%w: terlalu banyak sesi", ErrCCTVRelayUnavailable)
		}
		cctvHLS.sessions[u.Query().Get("id")] = cctvHLSSession{stream: name, admin: sess.ID, used: now}
		cctvHLS.Unlock()
		return nil
	}
	return ErrCCTVRelayUnavailable
}
