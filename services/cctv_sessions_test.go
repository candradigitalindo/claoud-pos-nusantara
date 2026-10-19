package services

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"cloud-pos/models"
)

func TestCCTVIndependentViewersAndRelaySnapshot(t *testing.T) {
	token, _, _, camera, _ := cctvFixture(t)
	deleted := map[string]bool{}
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted[r.URL.Query().Get("src")] = true
		}
		if r.URL.Path == "/api/stream.m3u8" {
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=192000\nhls/playlist.m3u8?id=%s", r.URL.Query().Get("src"))
		}
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	first, err := registerCCTVStream(*camera, "rtsp://user:secret@camera/live", "live")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registerCCTVStream(*camera, "rtsp://user:secret@camera/live", "live")
	if err != nil || first == second {
		t.Fatal("penonton berbagi nama relay", err)
	}
	for _, name := range []string{first, second} {
		if _, _, err := RelayCCTVMedia(token, "/api/stream.m3u8", "src="+name); err != nil {
			t.Fatal(err)
		}
	}
	// Konfigurasi saat ini berubah ke gateway; stop tetap memakai relay asal.
	camera.AccessMode = "gateway"
	if err := StopStream(cameraA, first); err != nil {
		t.Fatal("stop memakai relay baru", err)
	}
	if !deleted[first] || deleted[second] {
		t.Fatal("stop menghentikan penonton lain")
	}
	if _, err := CCTVMediaSource(token, "/api/stream.m3u8", "src="+first); !errors.Is(err, ErrCCTVAccessDenied) {
		t.Fatal("stream yang sudah ditutup masih bisa dimulai", err)
	}
	if _, _, err := RelayCCTVMedia(token, "/api/hls/segment.ts", "id="+second+"&n=1"); err != nil {
		t.Fatal("penonton kedua terputus", err)
	}
	if err := StopStream(cameraA, first); err != nil {
		t.Fatal("stop tidak idempotent", err)
	}
}

func TestCCTVInvalidationDeniesMediaAndRetriesFailedCleanup(t *testing.T) {
	token, _, _, cam, _ := cctvFixture(t)
	failDelete := true
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && failDelete {
			w.WriteHeader(500)
		}
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	name, err := registerCCTVStream(*cam, "rtsp://camera/live", "live")
	if err != nil {
		t.Fatal(err)
	}
	invalidateCameraStreams(*cam)
	if _, err := CCTVMediaSource(token, "/api/stream.m3u8", "src="+name); !errors.Is(err, ErrCCTVAccessDenied) {
		t.Fatal("konfigurasi lama masih dapat dipakai", err)
	}
	sweepCCTVStreams(time.Now())
	cctvStreams.Lock()
	pending, ok := cctvStreams.sessions[name]
	cctvStreams.Unlock()
	if !ok || !pending.closing {
		t.Fatal("cleanup gagal tidak disimpan untuk retry")
	}
	failDelete = false
	sweepCCTVStreams(time.Now())
	cctvStreams.Lock()
	_, ok = cctvStreams.sessions[name]
	cctvStreams.Unlock()
	if ok {
		t.Fatal("cleanup tidak diulang")
	}
}

func TestCCTVCleanupExpiresIdleLiveAndAbsolutePlayback(t *testing.T) {
	cctvFixture(t)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	cam := models.OutletCamera{ID: cameraA}
	live, _ := registerCCTVStream(cam, "rtsp://camera/live", "live")
	playback, _ := registerCCTVStream(cam, "rtsp://camera/record", "playback")
	active, _ := registerCCTVStream(cam, "rtsp://camera/live", "live")
	cctvStreams.Lock()
	entry := cctvStreams.sessions[live]
	entry.used = time.Now().Add(-4 * time.Minute)
	cctvStreams.sessions[live] = entry
	entry = cctvStreams.sessions[playback]
	entry.created = time.Now().Add(-61 * time.Minute)
	cctvStreams.sessions[playback] = entry
	cctvStreams.Unlock()
	sweepCCTVStreams(time.Now())
	for _, name := range []string{live, playback} {
		if _, err := cameraForCCTVStream(name); !errors.Is(err, ErrCCTVAccessDenied) {
			t.Fatal("stream kedaluwarsa masih tersedia", err)
		}
	}
	if _, err := cameraForCCTVStream(active); err != nil {
		t.Fatal("stream aktif ikut dibersihkan", err)
	}
}

func TestCCTVFailedCleanupCannotOccupySlotsForever(t *testing.T) {
	cctvFixture(t)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(500)
		}
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	cam := models.OutletCamera{ID: cameraA}
	name, err := registerCCTVStream(cam, "rtsp://camera/live", "live")
	if err != nil {
		t.Fatal(err)
	}
	invalidateCameraStreams(cam)
	for i := 0; i < 3; i++ {
		sweepCCTVStreams(time.Now())
	}
	cctvStreams.Lock()
	remaining := len(cctvStreams.sessions)
	cctvStreams.Unlock()
	if remaining != 0 {
		t.Fatal("relay offline menahan slot selamanya")
	}
	if _, err := cameraForCCTVStream(name); err != ErrCCTVAccessDenied {
		t.Fatal("sesi yang ditutup dibuka kembali", err)
	}
}

func TestCCTVProbeDoesNotSaveAndAlwaysRemovesTemporaryStream(t *testing.T) {
	cctvFixture(t)
	calls := []string{}
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		if r.Method == http.MethodPatch && r.URL.Query().Get("src") != "rtsp://operator:secret@camera:554/Streaming/Channels/102" {
			t.Error("probe tidak memakai konfigurasi formulir")
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(500)
			fmt.Fprint(w, "rtsp://operator:secret@camera/live")
		}
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	password := "secret"
	err := ProbeCameraConfiguration("outlet-a", models.CameraProbeRequest{UpdateCameraRequest: models.UpdateCameraRequest{Brand: "hikvision", Host: "camera", ConnectionType: "hikvision", AccessMode: "direct", RTSPUsername: "operator", RTSPPassword: &password, Subtype: 1}})
	if !errors.Is(err, ErrCCTVRelayUnavailable) || strings.Contains(err.Error(), password) {
		t.Fatal("galat probe tidak disanitasi", err)
	}
	if strings.Join(calls, ",") != "PATCH,GET,DELETE" {
		t.Fatalf("stream probe tidak dibersihkan: %v", calls)
	}
}

func TestCCTVPlaybackUsesCameraTimezone(t *testing.T) {
	for _, test := range []struct{ kind, zone, expected string }{
		{"hikvision", "Asia/Jakarta", "starttime=20261019T030000Z"},
		{"hikvision", "Asia/Jayapura", "starttime=20261019T010000Z"},
		{"dahua", "Asia/Makassar", "starttime=2026_10_19_10_00_00"},
	} {
		cam := models.OutletCamera{Brand: test.kind, Timezone: test.zone, Host: "camera"}
		source, err := BuildPlaybackRTSP(cam, "", "2026-10-19T10:00", "2026-10-19T11:00")
		if err != nil || !strings.Contains(source, test.expected) {
			t.Fatalf("zona %s: %s %v", test.zone, source, err)
		}
	}
	cam := models.OutletCamera{ConnectionType: "rtsp", Timezone: "Asia/Jakarta"}
	source, err := BuildCameraPlayback(cam, "", "rtsp://camera/record?local={start_iso}&epoch={start_unix}", "2026-10-19T03:00:00Z", "2026-10-19T04:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(source)
	if u.Query().Get("local") != "2026-10-19T10:00:00" || u.Query().Get("epoch") != "1792378800" {
		t.Fatal("template tidak mengikuti zona/epoch", source)
	}
}
