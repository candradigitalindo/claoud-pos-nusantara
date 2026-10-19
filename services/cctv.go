package services

// CCTV — integrasi kamera CCTV outlet secara RELAY-ONLY (zero-retention).
//
// Prinsip:
//   - Cloud TIDAK PERNAH menyimpan video. go2rtc me-relay aliran
//     dari perangkat outlet ke browser (HLS in-memory). Rekaman tetap di DVR.
//   - Perangkat lokal dijangkau melalui penghubung HTTPS keluar dari outlet,
//     tanpa VPN. Sumber cloud vendor dapat dijangkau langsung oleh relay cloud.
//   - Kredensial RTSP disimpan terenkripsi (AES-256-GCM) dan hanya dipakai server-side;
//     dikirim ke go2rtc via API internal (tidak pernah ke browser).
//
// Alur: handler membangun URL RTSP (live/playback) → daftarkan stream ke go2rtc via
// API internal → browser menonton lewat nginx /cctv/ (dilindungi cookie stream token).

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
)

var (
	cctvEncKey    []byte // 32-byte AES key (turunan SHA-256)
	cctvSecret    string // secret untuk stream token (HS256)
	go2rtcBaseURL string
	cctvHTTP      = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
)

// InitCCTV menyiapkan kunci enkripsi, secret token, dan alamat go2rtc. Dipanggil
// sekali dari main setelah config di-load. encKeyRaw boleh kosong — kunci diturunkan
// dari jwtSecret agar tetap berfungsi tanpa env tambahan (bisa dioverride via CAMERA_ENC_KEY).
func InitCCTV(encKeyRaw, jwtSecret, go2rtcURL string) {
	seed := encKeyRaw
	if seed == "" {
		seed = "cctv:" + jwtSecret
	}
	sum := sha256.Sum256([]byte(seed))
	cctvEncKey = sum[:]
	cctvSecret = jwtSecret
	go2rtcBaseURL = strings.TrimRight(go2rtcURL, "/")
	if go2rtcBaseURL == "" {
		go2rtcBaseURL = "http://go2rtc:1984"
	}
}

// ── Enkripsi kredensial RTSP (AES-256-GCM) ──────────────────────────────────

func encryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(cctvEncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decryptSecret(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(cctvEncKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext terlalu pendek")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// ── Stream token (cookie berumur pendek untuk akses relay /cctv/) ───────────

// IssueCCTVToken menurunkan cookie media dari JWT login, tanpa menyalin token
// admin ke browser. Cookie mengikuti versi/role sesi asal dan tidak boleh
// berumur lebih panjang daripada sesi tersebut. Token statis tidak diterima.
func IssueCCTVToken(adminJWT string) (string, error) {
	original, err := ValidateJWT(adminJWT, cctvSecret)
	if err != nil {
		return "", ErrAuthTokenInvalid
	}
	tc, err := parseAdminTokenClaims(original)
	if err != nil || tc.role == "" {
		return "", ErrAuthTokenInvalid
	}
	expires, err := original.GetExpirationTime()
	if err != nil || expires == nil {
		return "", ErrAuthTokenInvalid
	}
	deadline := time.Now().UTC().Add(time.Hour)
	if expires.Time.Before(deadline) {
		deadline = expires.Time
	}
	claims := jwt.MapClaims{
		"typ":  "cctv",
		"sub":  tc.sub,
		"role": tc.role,
		"ver":  tc.version,
		"iat":  time.Now().UTC().Unix(),
		"exp":  deadline.Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cctvSecret))
}

// ValidateCCTVToken memvalidasi cookie stream token.
func ValidateCCTVToken(tok string) error {
	_, err := authenticateCCTVToken(tok)
	return err
}

func authenticateCCTVToken(tok string) (*AdminSession, error) {
	parsed, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		return []byte(cctvSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, ErrAuthTokenInvalid
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrAuthTokenInvalid
	}
	if t, _ := claims["typ"].(string); t != "cctv" {
		return nil, ErrAuthTokenInvalid
	}
	// Cookie lama tanpa versi/role ditolak; bukan sesi yang dapat dicabut.
	if _, ok := claims["ver"]; !ok {
		return nil, ErrAuthTokenInvalid
	}
	claims["typ"] = adminTokenType
	tc, err := parseAdminTokenClaims(claims)
	if err != nil || tc.role == "" {
		return nil, ErrAuthTokenInvalid
	}
	return authenticateAdminClaims(tc)
}

// ── Pembangun URL RTSP (Hikvision / Dahua) ──────────────────────────────────

func normBrand(b string) string {
	b = strings.ToLower(strings.TrimSpace(b))
	if strings.HasPrefix(b, "dahua") {
		return "dahua"
	}
	return "hikvision"
}

func camPort(p int) int {
	if p <= 0 {
		return 554
	}
	return p
}

func rtspBase(cam models.OutletCamera, pass string) *url.URL {
	u := &url.URL{Scheme: "rtsp", Host: net.JoinHostPort(cam.Host, strconv.Itoa(camPort(cam.Port)))}
	if cam.RTSPUsername != "" {
		u.User = url.UserPassword(cam.RTSPUsername, pass)
	}
	return u
}

// BuildLiveRTSP menyusun URL RTSP live sub-stream (default) sesuai merk.
//
//	Hikvision: /Streaming/Channels/<ch><stream>  (ch1 sub = 102, main = 101)
//	Dahua:     /cam/realmonitor?channel=<ch>&subtype=<0|1>
func BuildLiveRTSP(cam models.OutletCamera, pass string) string {
	u := rtspBase(cam, pass)
	ch := cam.Channel
	if ch <= 0 {
		ch = 1
	}
	if normBrand(cam.Brand) == "dahua" {
		st := 1
		if cam.Subtype == 0 {
			st = 0
		}
		u.Path = "/cam/realmonitor"
		u.RawQuery = fmt.Sprintf("channel=%d&subtype=%d", ch, st)
		return u.String()
	}
	// Hikvision
	streamNo := 2 // sub-stream
	if cam.Subtype == 0 {
		streamNo = 1 // main-stream
	}
	u.Path = fmt.Sprintf("/Streaming/Channels/%d", ch*100+streamNo)
	return u.String()
}

// BuildPlaybackRTSP menyusun URL RTSP putar-ulang dari rekaman DVR untuk rentang waktu.
// Aliran playback bersifat transient — cloud tidak menyimpan apa pun.
//
//	Hikvision: /Streaming/tracks/<ch>01?starttime=YYYYMMDDThhmmssZ&endtime=...
//	Dahua:     /cam/playback?channel=<ch>&subtype=0&starttime=YYYY_MM_DD_hh_mm_ss&endtime=...
func BuildPlaybackRTSP(cam models.OutletCamera, pass, start, end string) (string, error) {
	st, err := parseCameraPlaybackTime(cam, start)
	if err != nil {
		return "", fmt.Errorf("waktu mulai tidak valid")
	}
	et, err := parseCameraPlaybackTime(cam, end)
	if err != nil {
		return "", fmt.Errorf("waktu selesai tidak valid")
	}
	if !et.After(st) {
		return "", fmt.Errorf("waktu selesai harus setelah waktu mulai")
	}
	u := rtspBase(cam, pass)
	ch := cam.Channel
	if ch <= 0 {
		ch = 1
	}
	if normBrand(cam.Brand) == "dahua" {
		u.Path = "/cam/playback"
		u.RawQuery = fmt.Sprintf("channel=%d&subtype=0&starttime=%s&endtime=%s",
			ch, st.Format("2006_01_02_15_04_05"), et.Format("2006_01_02_15_04_05"))
		return u.String(), nil
	}
	// Hikvision — starttime/endtime dalam format ISO basic + 'Z'.
	u.Path = fmt.Sprintf("/Streaming/tracks/%d", ch*100+1)
	u.RawQuery = fmt.Sprintf("starttime=%s&endtime=%s",
		st.UTC().Format("20060102T150405Z"), et.UTC().Format("20060102T150405Z"))
	return u.String(), nil
}

func parseCameraPlaybackTime(cam models.OutletCamera, s string) (time.Time, error) {
	tz := cam.Timezone
	if tz == "" {
		tz = "Asia/Jakarta"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: zona waktu kamera tidak valid", ErrCameraValidation)
	}
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), nil
	}
	for _, f := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("format waktu tidak dikenal")
}

// ── Klien go2rtc (API internal, di jaringan compose — bukan ke outlet) ───────

// Go2rtcPublish mendaftarkan/memperbarui satu named-stream di go2rtc (idempotent).
// go2rtc menghubungkan RTSP secara on-demand (hanya saat ada penonton) dan tidak
// menulis apa pun ke disk (relay in-memory).
func Go2rtcPublish(name, src string) error {
	q := url.Values{"name": {name}, "src": {src}}
	// PATCH mengubah sumber di RAM. PUT juga menulis URL berkredensial ke
	// konfigurasi go2rtc; kredensial relay tidak boleh dipersistenkan.
	req, err := http.NewRequest(http.MethodPatch, go2rtcBaseURL+"/api/streams?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := cctvHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("go2rtc tidak dapat dihubungi")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("go2rtc gagal daftar stream (%d)", resp.StatusCode)
	}
	return nil
}

// Go2rtcDelete menghapus named-stream (dipakai untuk membersihkan aliran playback transient).
func Go2rtcDelete(name string) error {
	q := url.Values{"src": {name}}
	req, err := http.NewRequest(http.MethodDelete, go2rtcBaseURL+"/api/streams?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := cctvHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("go2rtc gagal menghentikan stream (%d)", resp.StatusCode)
	}
	return nil
}

// ── CRUD kamera (config saja) ───────────────────────────────────────────────

func scanCameras(rows *sql.Rows) ([]models.OutletCamera, error) {
	defer rows.Close()
	out := make([]models.OutletCamera, 0)
	for rows.Next() {
		var c models.OutletCamera
		var pwdEnc, outletName string
		if err := rows.Scan(&c.ID, &c.OutletID, &outletName, &c.Name, &c.Brand, &c.Host, &c.Port,
			&c.Channel, &c.Subtype, &c.RTSPUsername, &pwdEnc, &c.IsActive, &c.SortOrder,
			&c.CreatedAt, &c.UpdatedAt, &c.ConnectionType, &c.AccessMode, &c.HasSourceURL, &c.HasPlaybackURL, &c.Timezone); err != nil {
			return nil, err
		}
		c.ID = strings.TrimSpace(c.ID)
		c.OutletID = strings.TrimSpace(c.OutletID)
		c.OutletName = strings.TrimSpace(outletName)
		c.HasPassword = pwdEnc != ""
		c.ConnectionType = cameraConnection(c)
		c.SupportsPlayback = c.ConnectionType == "hikvision" || c.ConnectionType == "dahua" || c.HasPlaybackURL
		out = append(out, c)
	}
	return out, rows.Err()
}

const cameraCols = `c.id, c.outlet_id, COALESCE(o.name,''), c.name, c.brand, c.host, c.port,
	c.channel, c.subtype, c.rtsp_username, c.rtsp_password_enc, c.is_active, c.sort_order,
	c.created_at, c.updated_at, c.connection_type, c.access_mode, c.source_url_enc <> '', c.playback_url_enc <> '', c.timezone`

// ListCameras mengembalikan kamera semua outlet dalam scope (opsional filter satu outlet).
func ListCameras(outletID string, scope []string) ([]models.OutletCamera, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("c.outlet_id = $%d", idx))
		args = append(args, strings.TrimSpace(outletID))
		idx++
	}
	if scope != nil {
		conds = append(conds, fmt.Sprintf("c.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(scope))
		idx++
	}
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id
		WHERE %s ORDER BY o.name ASC, c.sort_order ASC, c.created_at ASC`,
		cameraCols, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	return scanCameras(rows)
}

// ListCamerasByOutlet mengembalikan kamera satu outlet.
func ListCamerasByOutlet(outletID string) ([]models.OutletCamera, error) {
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id
		WHERE c.outlet_id = $1 ORDER BY c.sort_order ASC, c.created_at ASC`, cameraCols)
	rows, err := database.DB.Query(q, strings.TrimSpace(outletID))
	if err != nil {
		return nil, err
	}
	return scanCameras(rows)
}

// GetCamera mengembalikan satu kamera (tanpa password).
func GetCamera(id string) (*models.OutletCamera, error) {
	q := fmt.Sprintf(`SELECT %s FROM outlet_cameras c
		LEFT JOIN outlets o ON o.id = c.outlet_id WHERE c.id = $1`, cameraCols)
	rows, err := database.DB.Query(q, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	list, err := scanCameras(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sql.ErrNoRows
	}
	return &list[0], nil
}

// GetCameraOutletID mengembalikan outlet_id pemilik kamera (untuk validasi scope).
func GetCameraOutletID(id string) (string, error) {
	var outletID string
	err := database.DB.QueryRow(`SELECT outlet_id FROM outlet_cameras WHERE id = $1`, strings.TrimSpace(id)).Scan(&outletID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(outletID), nil
}

// getCameraSecret mengembalikan kamera + password RTSP terdekripsi (server-side only).
func getCameraConnectionSecrets(id string, requireActive bool) (*models.OutletCamera, string, string, string, error) {
	cam, err := cctvCameraLoader(id)
	if err != nil {
		return nil, "", "", "", err
	}
	if requireActive && !cam.IsActive {
		return nil, "", "", "", ErrCameraInactive
	}
	var enc, sourceEnc, playbackEnc string
	if err := database.DB.QueryRow(`SELECT rtsp_password_enc, source_url_enc, playback_url_enc FROM outlet_cameras WHERE id=$1`, strings.TrimSpace(id)).Scan(&enc, &sourceEnc, &playbackEnc); err != nil {
		return nil, "", "", "", err
	}
	pass, err := decryptSecret(enc)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("gagal membuka kredensial kamera")
	}
	source, err := decryptSecret(sourceEnc)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("gagal membuka sumber kamera")
	}
	playback, err := decryptSecret(playbackEnc)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("gagal membuka sumber rekaman")
	}
	return cam, pass, source, playback, nil
}

func CreateCamera(outletID string, req models.CreateCameraRequest) (*models.OutletCamera, error) {
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	cam := models.OutletCamera{ID: NewULID(), OutletID: strings.TrimSpace(outletID), Name: req.Name, Brand: req.Brand,
		ConnectionType: req.ConnectionType, AccessMode: req.AccessMode, Host: req.Host, Port: req.Port, Channel: req.Channel, Subtype: req.Subtype,
		RTSPUsername: strings.TrimSpace(req.RTSPUsername), IsActive: active, SortOrder: req.SortOrder, Timezone: req.Timezone}
	source, playback := "", ""
	if req.SourceURL != nil {
		source = strings.TrimSpace(*req.SourceURL)
	}
	if req.PlaybackURL != nil {
		playback = strings.TrimSpace(*req.PlaybackURL)
	}
	if err := validateCameraConfig(&cam, source, playback); err != nil {
		return nil, err
	}
	pwdEnc, err := encryptSecret(req.RTSPPassword)
	if err != nil {
		return nil, err
	}
	sourceEnc, err := encryptSecret(source)
	if err != nil {
		return nil, err
	}
	playbackEnc, err := encryptSecret(playback)
	if err != nil {
		return nil, err
	}
	_, err = database.DB.Exec(`INSERT INTO outlet_cameras
 (id,outlet_id,name,brand,host,port,channel,subtype,rtsp_username,rtsp_password_enc,is_active,sort_order,connection_type,access_mode,source_url_enc,playback_url_enc,timezone)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		cam.ID, cam.OutletID, cam.Name, cam.Brand, cam.Host, cam.Port, cam.Channel, cam.Subtype, cam.RTSPUsername, pwdEnc, cam.IsActive, cam.SortOrder, cam.ConnectionType, cam.AccessMode, sourceEnc, playbackEnc, cam.Timezone)
	if err != nil {
		return nil, err
	}
	return GetCamera(cam.ID)
}

func prepareCameraUpdate(current *models.OutletCamera, req models.UpdateCameraRequest, password, source, playback string) (models.OutletCamera, string, string, string, error) {
	cam := *current
	if req.ConnectionType != "" && req.ConnectionType != current.ConnectionType && req.SourceURL == nil {
		source = ""
	}
	cam.Name, cam.Brand, cam.Host, cam.Port, cam.Channel, cam.Subtype, cam.RTSPUsername, cam.SortOrder = req.Name, req.Brand, req.Host, req.Port, req.Channel, req.Subtype, strings.TrimSpace(req.RTSPUsername), req.SortOrder
	if req.ConnectionType != "" {
		cam.ConnectionType = req.ConnectionType
	}
	if req.Timezone != "" {
		cam.Timezone = req.Timezone
	}
	if req.AccessMode != "" {
		cam.AccessMode = req.AccessMode
	}
	if req.IsActive != nil {
		cam.IsActive = *req.IsActive
	}
	if req.RTSPPassword != nil {
		password = *req.RTSPPassword
	}
	if req.SourceURL != nil {
		source = strings.TrimSpace(*req.SourceURL)
	}
	if req.PlaybackURL != nil {
		playback = strings.TrimSpace(*req.PlaybackURL)
	}
	if err := validateCameraConfig(&cam, source, playback); err != nil {
		return cam, "", "", "", err
	}
	return cam, password, source, playback, nil
}

func UpdateCamera(id string, req models.UpdateCameraRequest) (*models.OutletCamera, error) {
	unlock := lockCCTVCamera(id)
	defer unlock()
	current, password, source, playback, err := getCameraConnectionSecrets(id, false)
	if err != nil {
		return nil, err
	}
	cam, password, source, playback, err := prepareCameraUpdate(current, req, password, source, playback)
	if err != nil {
		return nil, err
	}
	pwdEnc, err := encryptSecret(password)
	if err != nil {
		return nil, err
	}
	sourceEnc, err := encryptSecret(source)
	if err != nil {
		return nil, err
	}
	playbackEnc, err := encryptSecret(playback)
	if err != nil {
		return nil, err
	}
	_, err = database.DB.Exec(`UPDATE outlet_cameras SET name=$1,brand=$2,host=$3,port=$4,channel=$5,subtype=$6,rtsp_username=$7,rtsp_password_enc=$8,is_active=$9,sort_order=$10,connection_type=$11,access_mode=$12,source_url_enc=$13,playback_url_enc=$14,timezone=$15,updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$16`,
		cam.Name, cam.Brand, cam.Host, cam.Port, cam.Channel, cam.Subtype, cam.RTSPUsername, pwdEnc, cam.IsActive, cam.SortOrder, cam.ConnectionType, cam.AccessMode, sourceEnc, playbackEnc, cam.Timezone, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	invalidateCameraStreams(*current)
	return GetCamera(id)
}

func ToggleCamera(id string) (*models.OutletCamera, error) {
	unlock := lockCCTVCamera(id)
	defer unlock()
	current, err := GetCamera(id)
	if err != nil {
		return nil, err
	}
	res, err := database.DB.Exec(`UPDATE outlet_cameras SET is_active = NOT is_active, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, sql.ErrNoRows
	}
	invalidateCameraStreams(*current)
	return GetCamera(id)
}

func DeleteCamera(id string) error {
	unlock := lockCCTVCamera(id)
	defer unlock()
	current, err := GetCamera(id)
	if err != nil {
		return err
	}
	res, err := database.DB.Exec(`DELETE FROM outlet_cameras WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	invalidateCameraStreams(*current)
	return nil
}

// StartLive membangun stream live di go2rtc dan mengembalikan info stream.
func StartLive(id string) (*models.StreamResponse, error) {
	unlock := lockCCTVCamera(id)
	defer unlock()
	cam, pass, source, _, err := getCameraConnectionSecrets(id, true)
	if err != nil {
		return nil, err
	}
	src, err := resolveCameraSource(*cam, pass, source)
	if err != nil {
		return nil, err
	}
	name, err := registerCCTVStream(*cam, src, "live")
	if err != nil {
		return nil, err
	}
	return streamResp(name, "live"), nil
}

// StartPlayback membangun stream putar-ulang transient di go2rtc.
func StartPlayback(id string, req models.PlaybackRequest) (*models.StreamResponse, error) {
	unlock := lockCCTVCamera(id)
	defer unlock()
	cam, pass, _, playback, err := getCameraConnectionSecrets(id, true)
	if err != nil {
		return nil, err
	}
	src, err := BuildCameraPlayback(*cam, pass, playback, req.Start, req.End)
	if err != nil {
		return nil, err
	}
	name, err := registerCCTVStream(*cam, src, "playback")
	if err != nil {
		return nil, err
	}
	return streamResp(name, "playback"), nil
}

// CCTVStreamCameraID hanya menerima nama terbitan backend, bukan URL RTSP,
// sumber sembarang, atau prefix parsial milik kamera lain.
func CCTVStreamCameraID(name string) (string, error) {
	var id, suffix string
	if strings.HasPrefix(name, "cam_") && (len(name) == 30 || len(name) == 39 && name[30] == '_') {
		id = name[4:30]
		if len(name) == 39 {
			suffix = name[31:]
		}
	} else if strings.HasPrefix(name, "pb_") && len(name) == 38 && name[29] == '_' {
		id, suffix = name[3:29], name[30:]
	}
	for _, ch := range suffix {
		if !strings.ContainsRune("0123456789abcdefghjkmnpqrstvwxyz", ch) {
			return "", ErrCCTVAccessDenied
		}
	}
	if _, err := ulid.ParseStrict(id); err != nil || id != strings.ToUpper(id) {
		return "", ErrCCTVAccessDenied
	}
	return id, nil
}

func streamResp(name, mode string) *models.StreamResponse {
	return &models.StreamResponse{
		Name:   name,
		Mode:   mode,
		HLSURL: "/cctv/api/stream.m3u8?src=" + url.QueryEscape(name),
	}
}
