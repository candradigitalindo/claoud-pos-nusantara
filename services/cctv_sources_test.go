package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"cloud-pos/internal/cctvrelay"
	"cloud-pos/models"
)

func TestCCTVMultiVendorSources(t *testing.T) {
	for _, test := range []struct {
		kind, want string
		port       int
	}{
		{"hikvision", "rtsp://operator:p%40ss@192.168.1.10:554/Streaming/Channels/102", 554},
		{"dahua", "rtsp://operator:p%40ss@192.168.1.10:554/cam/realmonitor?channel=1&subtype=1", 554},
		{"tapo", "rtsp://operator:p%40ss@192.168.1.10:554/stream2", 554},
		{"reolink", "rtsp://operator:p%40ss@192.168.1.10:554/h264Preview_01_sub", 554},
		{"onvif", "onvif://operator:p%40ss@192.168.1.10:80?subtype=1", 80},
		{"dvrip", "dvrip://operator:p%40ss@192.168.1.10:34567?channel=0&subtype=1", 34567},
	} {
		t.Run(test.kind, func(t *testing.T) {
			cam := models.OutletCamera{Name: "Kasir", Brand: "HK", ConnectionType: test.kind, AccessMode: "gateway", Host: "192.168.1.10", Channel: 1, Subtype: 1, RTSPUsername: "operator"}
			if err := validateCameraConfig(&cam, "", ""); err != nil {
				t.Fatal(err)
			}
			actual, err := BuildCameraSource(cam, "p@ss", "")
			if err != nil || actual != test.want {
				t.Fatalf("sumber: %s (%v), mau %s", actual, err, test.want)
			}
		})
	}
	cam := models.OutletCamera{Name: "China", Brand: "v380", Host: "192.168.1.10"}
	if err := validateCameraConfig(&cam, "", ""); !errors.Is(err, ErrCameraValidation) {
		t.Fatal("merek tak dikenal masih diasumsikan Hikvision")
	}
}

func TestCCTVCustomURLsAndPlaybackCapabilities(t *testing.T) {
	for _, raw := range []string{"exec:cat /etc/passwd", "ffmpeg:rtsp://camera/live", "rtsp://camera/live#input=file", "file:///etc/passwd"} {
		if validateCameraSourceURL(raw, "rtsp") == nil {
			t.Fatal("protokol berbahaya diterima")
		}
	}
	source := "rtsp://user:secret@camera/live"
	cam := models.OutletCamera{Name: "China", Brand: "yoosee", ConnectionType: "rtsp", AccessMode: "gateway", Channel: 1}
	if err := validateCameraConfig(&cam, source, ""); err != nil {
		t.Fatal(err)
	}
	if actual, err := BuildCameraSource(cam, "", source); err != nil || actual != source {
		t.Fatal("sumber RTSP manual berubah")
	}
	if _, err := BuildCameraPlayback(cam, "", "", "2026-10-19T10:00", "2026-10-19T11:00"); !errors.Is(err, ErrPlaybackUnsupported) {
		t.Fatal("playback merek generik diasumsikan Hikvision")
	}
	actual, err := BuildCameraPlayback(cam, "", "rtsp://camera/record?start={start}&end={end}", "2026-10-19T10:00", "2026-10-19T11:00")
	if err != nil || !strings.Contains(actual, "start=20261019T030000Z") {
		t.Fatalf("template rekaman: %s %v", actual, err)
	}
}

func TestCCTVCloudDiscoveryTicketsProtectCredentials(t *testing.T) {
	cctvFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tuya" || r.URL.Query().Get("email") != "owner@example.com" {
			t.Error("permintaan cloud salah")
		}
		fmt.Fprint(w, `{"sources":[{"name":"Kamera toko","url":"tuya://protect-eu.ismartlife.me?device_id=dev1&email=owner%40example.com&password=secret"}]}`)
	}))
	defer server.Close()
	go2rtcBaseURL = server.URL
	devices, err := DiscoverTuyaCameras("admin-a", "outlet-a", "protect-eu.ismartlife.me", "owner@example.com", "secret")
	if err != nil || len(devices) != 1 {
		t.Fatalf("discovery: %v", err)
	}
	encoded, _ := json.Marshal(devices)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "owner@example.com") {
		t.Fatal("kredensial dikembalikan ke UI")
	}
	src, err := ResolveCCTVCloudTicket(devices[0].Ticket, "admin-a", "outlet-a")
	if err != nil || !strings.Contains(src, "password=secret") {
		t.Fatal("tiket tidak dapat digunakan admin/outlet pemilik")
	}
	for _, pair := range [][2]string{{"admin-b", "outlet-a"}, {"admin-a", "outlet-b"}} {
		if _, err := ResolveCCTVCloudTicket(devices[0].Ticket, pair[0], pair[1]); err == nil {
			t.Fatal("tiket dapat dipakai admin/outlet lain")
		}
	}
	if _, err := ResolveCCTVCloudTicket(devices[0].Ticket+"bad", "admin-a", "outlet-a"); err == nil {
		t.Fatal("tiket rusak diterima")
	}
	data, _ := json.Marshal(cctvCloudTicket{Source: src, Admin: "admin-a", Outlet: "outlet-a", Expires: time.Now().Add(-time.Second).Unix()})
	expired, _ := encryptSecret(string(data))
	if _, err := ResolveCCTVCloudTicket(expired, "admin-a", "outlet-a"); err == nil {
		t.Fatal("tiket kedaluwarsa diterima")
	}
}

func TestCCTVGatewayJobsCannotCrossOutlets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		response, err := requestCCTVGateway(ctx, "outlet-a:key", cctvrelay.Request{Method: "GET", Path: "/api/stream.m3u8", Query: url.Values{"src": {"cam_" + cameraA}}.Encode()})
		if err == nil && string(response.Body) != "ok" {
			err = errors.New("body tidak cocok")
		}
		done <- err
	}()
	jobs := PollCCTVGateway(ctx, "outlet-a:key")
	if len(jobs) != 1 {
		t.Fatal("job tidak diterima")
	}
	result := cctvrelay.Response{ID: jobs[0].ID, Status: 200, Body: []byte("ok")}
	if CompleteCCTVGateway("outlet-b:key", result) {
		t.Fatal("outlet lain dapat menyelesaikan job")
	}
	if !CompleteCCTVGateway("outlet-a:key", result) {
		t.Fatal("outlet pemilik ditolak")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if CompleteCCTVGateway("outlet-a:key", result) {
		t.Fatal("hasil job dapat diulang")
	}
}

func TestCCTVONVIFFallsBackToAvailableProfile(t *testing.T) {
	cctvFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/onvif" {
			t.Error("endpoint bukan ONVIF")
		}
		if strings.Contains(r.URL.Query().Get("src"), "subtype=") {
			t.Error("profil harus dicari tanpa subtype")
		}
		fmt.Fprint(w, `{"sources":[{"url":"onvif://operator:secret@camera:80?subtype=OnlyProfile"},{"url":"onvif://operator:secret@camera:80?subtype=OnlyProfile&snapshot"}]}`)
	}))
	defer server.Close()
	go2rtcBaseURL = server.URL
	cam := models.OutletCamera{ConnectionType: "onvif", Host: "camera", Port: 80, Subtype: 1, RTSPUsername: "operator"}
	src, err := resolveCameraSource(cam, "secret", "")
	if err != nil || !strings.Contains(src, "subtype=OnlyProfile") || strings.Contains(src, "snapshot") {
		t.Fatalf("profil ONVIF: %s %v", src, err)
	}
}

func TestCCTVONVIFProfileSelectionProtectsCredentialsAndAvoidsWrongChannel(t *testing.T) {
	cctvFixture(t)
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"sources":[{"name":"Kasir utama","url":"onvif://operator:secret@camera:80?subtype=Camera1Main"},{"name":"Teras hemat","url":"onvif://operator:secret@camera:80?subtype=Camera2Sub"},{"name":"Snapshot","url":"onvif://operator:secret@camera:80?subtype=Camera1Main&snapshot"},{"name":"Perangkat asing","url":"onvif://operator:secret@other-camera:80?subtype=Other"}]}`)
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	password := "secret"
	req := models.CameraProbeRequest{UpdateCameraRequest: models.UpdateCameraRequest{Brand: "HKM", ConnectionType: "onvif", AccessMode: "direct", Host: "camera", Port: 80, RTSPUsername: "operator", RTSPPassword: &password}}
	profiles, err := DiscoverCameraONVIFProfiles("admin-a", "outlet-a", req)
	if err != nil || len(profiles) != 2 || profiles[1].Name != "Teras hemat" {
		t.Fatalf("profil salah: %d %v", len(profiles), err)
	}
	encoded, _ := json.Marshal(profiles)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "onvif://") {
		t.Fatal("URL/kredensial profil bocor ke browser")
	}
	source, err := ResolveCCTVCloudTicket(profiles[1].Ticket, "admin-a", "outlet-a")
	if err != nil || !strings.Contains(source, "subtype=Camera2Sub") {
		t.Fatal("profil terpilih tidak sesuai", err)
	}
	if _, err := ResolveCCTVCloudTicket(profiles[1].Ticket, "admin-a", "outlet-b"); err != ErrCCTVAccessDenied {
		t.Fatal("tiket profil dapat dipindah ke outlet lain")
	}
	cam := models.OutletCamera{ConnectionType: "onvif", Host: "camera", Port: 80, Subtype: 1, RTSPUsername: "operator"}
	if _, err := resolveCameraSource(cam, password, ""); !errors.Is(err, ErrCameraValidation) {
		t.Fatal("DVR multi-profil masih dipilih menurut urutan channel", err)
	}
	if selected, err := resolveCameraSource(cam, password, source); err != nil || selected != source {
		t.Fatal("profil terpilih diganti secara otomatis", err)
	}
}
