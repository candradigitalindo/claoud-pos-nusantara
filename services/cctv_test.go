package services

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud-pos/models"

	"github.com/golang-jwt/jwt/v5"
)

const cameraA = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const cameraB = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

func cctvFixture(t *testing.T) (string, *adminAccountState, *RoleScope, *models.OutletCamera, *[]string) {
	t.Helper()
	key, secret, base := cctvEncKey, cctvSecret, go2rtcBaseURL
	oldCamera, oldPermission := cctvCameraLoader, cctvPermissionLoader
	InitCCTV("uji", sesiRahasiaUji, "http://relay.invalid")
	account := &adminAccountState{Found: true, Active: true, Role: "manager", Version: 1}
	scope := &RoleScope{Type: "specific", OutletIDs: []string{"outlet-a"}}
	cam := &models.OutletCamera{ID: cameraA, OutletID: "outlet-a", IsActive: true}
	perms := &[]string{"cameras.view"}
	sesiPasangTiruan(t, func(string) (adminAccountState, error) { return *account, nil }, func(string) (RoleScope, error) { return *scope, nil })
	cctvCameraLoader = func(id string) (*models.OutletCamera, error) {
		copy := *cam
		copy.ID = id
		if id == cameraB {
			copy.OutletID = "outlet-b"
		}
		return &copy, nil
	}
	cctvPermissionLoader = func(string) ([]string, error) { return *perms, nil }
	cctvHLS.Lock()
	cctvHLS.sessions = make(map[string]cctvHLSSession)
	cctvHLS.Unlock()
	cctvStreams.Lock()
	oldStreams := cctvStreams.sessions
	cctvStreams.sessions = make(map[string]cctvStreamSession)
	cctvStreams.Unlock()
	t.Cleanup(func() {
		cctvStreams.Lock()
		cctvStreams.sessions = oldStreams
		cctvStreams.Unlock()
		cctvEncKey, cctvSecret, go2rtcBaseURL = key, secret, base
		cctvCameraLoader, cctvPermissionLoader = oldCamera, oldPermission
	})
	admin := sesiTandaTangan(t, sesiKlaimBaru("admin-a", "manager", 1), sesiRahasiaUji)
	token, err := IssueCCTVToken(admin)
	if err != nil {
		t.Fatal(err)
	}
	return token, account, scope, cam, perms
}

func TestCCTVRelayScopeAndRevocation(t *testing.T) {
	token, account, scope, cam, perms := cctvFixture(t)
	if _, err := authorizeCCTVStream(token, "cam_"+cameraA); err != nil {
		t.Fatal(err)
	}
	if _, err := authorizeCCTVStream(token, "cam_"+cameraB); !errors.Is(err, ErrCCTVAccessDenied) {
		t.Fatalf("outlet lain: %v", err)
	}
	*perms = nil
	if _, err := authorizeCCTVStream(token, "cam_"+cameraA); !errors.Is(err, ErrCCTVAccessDenied) {
		t.Fatalf("izin dicabut: %v", err)
	}
	*perms = []string{"cameras.view"}
	cam.IsActive = false
	if _, err := authorizeCCTVStream(token, "cam_"+cameraA); !errors.Is(err, ErrCameraInactive) {
		t.Fatalf("kamera nonaktif: %v", err)
	}
	cam.IsActive = true
	scope.OutletIDs = nil
	InvalidateAllAdminSessions()
	if _, err := authorizeCCTVStream(token, "cam_"+cameraA); !errors.Is(err, ErrCCTVAccessDenied) {
		t.Fatalf("scope dicabut: %v", err)
	}
	scope.OutletIDs = []string{"outlet-a"}
	for _, mutate := range []func(){
		func() { account.Active = false },
		func() { account.Active = true; account.Version++ },
		func() { account.Version = 1; account.Role = "other" },
		func() { account.Role = "manager"; account.Found = false },
	} {
		mutate()
		InvalidateAllAdminSessions()
		if err := ValidateCCTVToken(token); err == nil {
			t.Fatal("sesi tercabut masih membuka CCTV")
		}
	}
}

func TestCCTVHLSCannotReuseAnotherSession(t *testing.T) {
	token, _, _, _, _ := cctvFixture(t)
	calls := 0
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("identitas browser diteruskan ke go2rtc")
		}
		if r.URL.Path == "/api/stream.m3u8" {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=192000\nhls/playlist.m3u8?id=sessionA")
		} else {
			fmt.Fprint(w, "segment")
		}
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	if _, _, err := RelayCCTVMedia(token, "/api/stream.m3u8", "src=cam_"+cameraA); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RelayCCTVMedia(token, "/api/hls/segment.ts", "id=sessionA&n=1"); err != nil {
		t.Fatal(err)
	}
	other, err := IssueCCTVToken(sesiTandaTangan(t, sesiKlaimBaru("admin-b", "manager", 1), sesiRahasiaUji))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"id=sessionA", "id=guessed", "id=sessionA&src=cam_" + cameraA} {
		if _, _, err := RelayCCTVMedia(other, "/api/hls/segment.ts", query); !errors.Is(err, ErrCCTVAccessDenied) {
			t.Fatalf("sesi asing %s: %v", query, err)
		}
	}
	for _, request := range [][2]string{
		{"/api/streams", "src=cam_" + cameraA},
		{"/api/config", ""},
		{"/api/webrtc", "src=cam_" + cameraA + "&dst=other"},
		{"/api/stream.m3u8", "src=rtsp://user:password@host/live"},
		{"/api/stream.m3u8", "src=cam_" + cameraA + "&src=cam_" + cameraB},
		{"/api/stream.m3u8", "src=cam_" + cameraA + "&name=other"},
	} {
		if _, _, err := RelayCCTVMedia(token, request[0], request[1]); err == nil {
			t.Fatalf("endpoint/sumber terlarang: %v", request)
		}
	}
	if calls != 2 {
		t.Fatalf("permintaan tanpa izin mencapai relay: %d panggilan", calls)
	}
}

func TestCCTVStopMustMatchCameraAndReportRelayFailure(t *testing.T) {
	cctvFixture(t)
	calls := 0
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "rtsp://admin:secret@host/live")
	}))
	defer relay.Close()
	go2rtcBaseURL = relay.URL
	for _, name := range []string{"cam_" + cameraB, "pb_" + cameraB + "_0123abcd", "rtsp://host/live", "", "cam_" + cameraA + "extra"} {
		if err := StopStream(cameraA, name); !errors.Is(err, ErrCCTVAccessDenied) {
			t.Fatalf("stop %q: %v", name, err)
		}
	}
	if calls != 0 {
		t.Fatal("stop kamera lain mencapai relay")
	}
	name := "pb_" + cameraA + "_0123abcd"
	cctvStreams.Lock()
	cctvStreams.sessions[name] = cctvStreamSession{camera: models.OutletCamera{ID: cameraA}}
	cctvStreams.Unlock()
	if err := StopStream(cameraA, name); err == nil {
		t.Fatal("kegagalan relay dilaporkan sukses")
	}
	if err := Go2rtcPublish("cam_"+cameraA, "rtsp://admin:secret@host/live"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("galat bocor/tidak dilaporkan: %v", err)
	}
}

func TestCCTVInactiveCameraCannotStartPlayback(t *testing.T) {
	_, _, _, cam, _ := cctvFixture(t)
	cam.IsActive = false
	_, err := StartPlayback(cameraA, models.PlaybackRequest{Start: "2026-10-19T10:00:00", End: "2026-10-19T10:05:00"})
	if !errors.Is(err, ErrCameraInactive) {
		t.Fatalf("playback kamera nonaktif: %v", err)
	}
}

func TestCCTVTokenRequiresLoginAndKeepsOriginalExpiry(t *testing.T) {
	cctvFixture(t)
	claims := sesiKlaimBaru("admin-a", "manager", 1)
	claims["exp"] = time.Now().Add(time.Minute).Unix()
	token, err := IssueCCTVToken(sesiTandaTangan(t, claims, sesiRahasiaUji))
	if err != nil {
		t.Fatal(err)
	}
	relayClaims, err := ValidateJWT(token, sesiRahasiaUji)
	if err != nil {
		t.Fatal(err)
	}
	if relayClaims["exp"] != float64(claims["exp"].(int64)) {
		t.Fatal("cookie melampaui umur sesi login")
	}
	if _, err := IssueCCTVToken(token); err == nil {
		t.Fatal("cookie CCTV dapat menerbitkan sesi baru")
	}
	delete(relayClaims, "exp")
	if err := ValidateCCTVToken(sesiTandaTangan(t, relayClaims, sesiRahasiaUji)); err == nil {
		t.Fatal("cookie tanpa batas waktu diterima")
	}
	if _, err := IssueCCTVToken("admin-a"); err == nil {
		t.Fatal("ID akun diterima sebagai bukti login")
	}
	legacy := jwt.MapClaims{"typ": "cctv", "sub": "admin-a", "exp": time.Now().Add(time.Hour).Unix()}
	if err := ValidateCCTVToken(sesiTandaTangan(t, legacy, sesiRahasiaUji)); err == nil {
		t.Fatal("cookie lama tanpa sesi diterima")
	}
}

// Opt-in: server go2rtc uji dengan named-stream test-source (video sintetis).
// Tidak menghubungi DVR atau database produksi.
func TestCCTVGo2rtcHLSIntegration(t *testing.T) {
	base := os.Getenv("CCTV_TEST_GO2RTC_URL")
	if base == "" {
		t.Skip("set CCTV_TEST_GO2RTC_URL untuk relay uji dengan stream test-source")
	}
	token, _, _, cam, _ := cctvFixture(t)
	go2rtcBaseURL = base
	name, err := registerCCTVStream(*cam, "test-source", "live")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = StopStream(cameraA, name) })
	if _, _, err := RelayCCTVMedia(token, "/api/stream.m3u8", "src="+name); err != nil {
		t.Fatal(err)
	}
	cctvHLS.Lock()
	id := ""
	for key, session := range cctvHLS.sessions {
		if session.stream == name {
			id = key
		}
	}
	cctvHLS.Unlock()
	if id == "" {
		t.Fatal("playlist tidak menghasilkan sesi terikat")
	}
	if body, _, err := RelayCCTVMedia(token, "/api/hls/playlist.m3u8", "id="+id); err != nil || !strings.Contains(string(body), "segment.ts") {
		t.Fatalf("playlist HLS: %s %v", body, err)
	}
	if body, mediaType, err := RelayCCTVMedia(token, "/api/hls/segment.ts", "id="+id+"&n=0"); err != nil || len(body) == 0 || mediaType != "video/mp2t" {
		t.Fatalf("segmen HLS: %d byte, %s, %v", len(body), mediaType, err)
	}
}
