package services

import (
	"cloud-pos/database"
	"cloud-pos/internal/cctvrelay"
	"cloud-pos/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func prepareCCTVTestSchema(t *testing.T) {
	t.Helper()
	requireDB(t)
	// Fixture tabel lama pada Postgres uji; TestMain menolak database produksi.
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS outlets(id CHAR(26) PRIMARY KEY,name TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS outlet_cameras(id CHAR(26) PRIMARY KEY,outlet_id CHAR(26) REFERENCES outlets(id),name TEXT,brand TEXT,host TEXT,port INT,channel INT,subtype INT,rtsp_username TEXT,rtsp_password_enc TEXT,is_active BOOLEAN,sort_order INT,created_at TIMESTAMP DEFAULT NOW(),updated_at TIMESTAMP DEFAULT NOW())`,
	} {
		if _, err := database.DB.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := database.RunCCTVMigrations(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCCTVMultiVendorAndOutboundGatewayDB(t *testing.T) {
	prepareCCTVTestSchema(t)
	outletA, outletB := NewULID(), NewULID()
	for _, id := range []string{outletA, outletB} {
		if _, err := database.DB.Exec(`INSERT INTO outlets(id,name) VALUES($1,'Outlet uji')`, id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM outlet_cameras WHERE outlet_id IN($1,$2)`, outletA, outletB)
		database.DB.Exec(`DELETE FROM outlets WHERE id IN($1,$2)`, outletA, outletB)
	})
	token, _, scope, _, _ := cctvFixture(t)
	scope.OutletIDs = []string{outletA}
	InvalidateAllAdminSessions()
	cctvCameraLoader = GetCamera
	source := "rtsp://operator:secret-camera@192.168.1.10/vendor/path"
	cam, err := CreateCamera(outletA, models.CreateCameraRequest{Name: "Kamera generik", Brand: "v380", ConnectionType: "rtsp", AccessMode: "gateway", SourceURL: &source, Channel: 1, Subtype: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !cam.HasSourceURL || cam.SupportsPlayback || cam.AccessMode != "gateway" {
		t.Fatalf("kapabilitas salah: %+v", cam)
	}
	var stored string
	database.DB.QueryRow(`SELECT source_url_enc FROM outlet_cameras WHERE id=$1`, cam.ID).Scan(&stored)
	if strings.Contains(stored, "secret-camera") || stored == source {
		t.Fatal("URL kamera tidak terenkripsi")
	}
	data, _ := json.Marshal(cam)
	if strings.Contains(string(data), "secret-camera") {
		t.Fatal("URL kamera bocor ke JSON")
	}
	inactive := false
	updated, err := UpdateCamera(cam.ID, models.UpdateCameraRequest{Name: "Ganti nama", Brand: cam.Brand, ConnectionType: cam.ConnectionType, Channel: 1, Subtype: 1, IsActive: &inactive})
	if err != nil || updated.IsActive || !updated.HasSourceURL {
		t.Fatalf("edit sumber: %+v %v", updated, err)
	}
	updated, err = UpdateCamera(cam.ID, models.UpdateCameraRequest{Name: "Nama kedua", Brand: cam.Brand, ConnectionType: cam.ConnectionType, Channel: 1, Subtype: 1})
	if err != nil || updated.IsActive {
		t.Fatal("edit mengaktifkan kamera nonaktif", err)
	}
	if _, err := ToggleCamera(cam.ID); err != nil {
		t.Fatal(err)
	}
	gatewayToken, err := CreateCCTVGateway(outletA, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCCTVGateway(outletB, false); err != nil {
		t.Fatal(err)
	}
	gateways, err := ListCCTVGateways([]string{outletA})
	if err != nil || len(gateways) != 1 {
		t.Fatal("scope gateway salah", err)
	}
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/streams":
			if r.Method == http.MethodPatch && r.URL.Query().Get("src") != source {
				t.Error("sumber kamera berubah")
			}
			fmt.Fprint(w, `{}`)
		case "/api/stream.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=192000\nhls/playlist.m3u8?id=edgeSession")
		case "/api/hls/segment.ts":
			w.Write([]byte{0x47, 0x40, 0x11, 0x10})
		default:
			w.WriteHeader(404)
		}
	}))
	defer relay.Close()
	cloud := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := AuthenticateCCTVGateway(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/cctv/gateway/poll":
			json.NewEncoder(w).Encode(map[string]any{"jobs": PollCCTVGateway(r.Context(), key)})
		case "/api/v1/cctv/gateway/result":
			var response cctvrelay.Response
			if json.NewDecoder(r.Body).Decode(&response) != nil || !CompleteCCTVGateway(key, response) {
				w.WriteHeader(404)
				return
			}
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer cloud.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := cctvrelay.Client{CloudURL: cloud.URL, Token: gatewayToken, RelayURL: relay.URL, HTTP: cloud.Client()}
	go client.Run(ctx, nil)
	for i := 0; i < 50; i++ {
		if _, err := loadCCTVGateway(outletA); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stream, err := StartLive(cam.ID)
	if err != nil {
		t.Fatal("start HTTPS keluar", err)
	}
	if _, _, err := RelayCCTVMedia(token, "/api/stream.m3u8", "src="+stream.Name); err != nil {
		t.Fatal("playlist HTTPS keluar", err)
	}
	body, _, err := RelayCCTVMedia(token, "/api/hls/segment.ts", "id=edgeSession&n=0")
	if err != nil || len(body) != 4 {
		t.Fatal("segmen HTTPS keluar", err)
	}
	if err := StopStream(cam.ID, stream.Name); err != nil {
		t.Fatal("stop HTTPS keluar", err)
	}
	if _, _, err := RelayCCTVMedia(token, "/api/hls/segment.ts", "id=edgeSession&n=1"); err == nil {
		t.Fatal("HLS masih diizinkan setelah stop")
	}
	cancel()
	rotated, err := CreateCCTVGateway(outletA, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateCCTVGateway(gatewayToken); err == nil {
		t.Fatal("token lama masih diterima")
	}
	if _, err := AuthenticateCCTVGateway(rotated); err != nil {
		t.Fatal("token baru ditolak", err)
	}
	if err := DeleteCCTVGateway(outletA); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthenticateCCTVGateway(rotated); err == nil {
		t.Fatal("token penghubung yang dicabut masih diterima")
	}
}

func TestCCTVProbeStoredSecretsAndConfigurationCleanupDB(t *testing.T) {
	prepareCCTVTestSchema(t)
	outlet := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets(id,name) VALUES($1,'Outlet konfigurasi uji')`, outlet); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM outlet_cameras WHERE outlet_id=$1`, outlet)
		database.DB.Exec(`DELETE FROM outlets WHERE id=$1`, outlet)
	})
	cctvFixture(t)
	cctvCameraLoader = GetCamera
	source := "onvif://operator:stored-source-secret@camera:80?subtype=Profile1"
	playback := "rtsp://operator:record-secret@camera/record?start={start}"
	cam, err := CreateCamera(outlet, models.CreateCameraRequest{Name: "Kamera uji", Brand: "HKM", ConnectionType: "onvif", AccessMode: "direct", SourceURL: &source, PlaybackURL: &playback, RTSPPassword: "stored-password", Timezone: "Asia/Jayapura"})
	if err != nil {
		t.Fatal(err)
	}
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" && strings.HasPrefix(r.URL.Query().Get("name"), "check_") && r.URL.Query().Get("src") != source {
			t.Error("probe tidak memakai sumber terenkripsi tersimpan")
		}
		fmt.Fprint(w, `{}`)
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	req := models.CameraProbeRequest{CameraID: cam.ID, UpdateCameraRequest: models.UpdateCameraRequest{Name: cam.Name, Brand: cam.Brand, ConnectionType: cam.ConnectionType, AccessMode: cam.AccessMode}}
	if err := ProbeCameraConfiguration(outlet, req); err != nil {
		t.Fatal("probe tersimpan", err)
	}
	if err := ProbeCameraConfiguration(NewULID(), req); err != ErrCCTVAccessDenied {
		t.Fatal("probe dapat mengambil kredensial outlet lain", err)
	}
	unchanged, err := GetCamera(cam.ID)
	if err != nil || unchanged.UpdatedAt != cam.UpdatedAt || !unchanged.HasSourceURL || !unchanged.HasPassword || unchanged.Timezone != "Asia/Jayapura" {
		t.Fatal("probe mengubah konfigurasi", err)
	}
	empty := ""
	updated, err := UpdateCamera(cam.ID, models.UpdateCameraRequest{Name: cam.Name, Brand: cam.Brand, ConnectionType: "hikvision", Host: "camera", RTSPPassword: &empty, PlaybackURL: &empty})
	if err != nil || updated.HasSourceURL || updated.HasPassword || updated.HasPlaybackURL || updated.Timezone != "Asia/Jayapura" {
		t.Fatal("perubahan protokol/penghapusan rahasia gagal", err)
	}
	first, err := StartLive(cam.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := StartLive(cam.ID)
	if err != nil || first.Name == second.Name {
		t.Fatal("sesi live tidak terpisah", err)
	}
	if err := DeleteCamera(cam.ID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{first.Name, second.Name} {
		if _, err := cameraForCCTVStream(name); err != ErrCCTVAccessDenied {
			t.Fatal("kamera yang dihapus masih punya stream aktif", err)
		}
	}
	sweepCCTVStreams(time.Now())
	cctvStreams.Lock()
	left := len(cctvStreams.sessions)
	cctvStreams.Unlock()
	if left != 0 {
		t.Fatal("relay kamera terhapus tidak dibersihkan")
	}
}
