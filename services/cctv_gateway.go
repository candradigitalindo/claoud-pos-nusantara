package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"cloud-pos/database"
	"cloud-pos/internal/cctvrelay"
	"cloud-pos/models"

	"github.com/lib/pq"
)

var ErrCCTVGatewayOffline = errors.New("penghubung outlet belum tersambung; jalankan penghubung pada perangkat outlet atau pilih koneksi langsung/cloud vendor")

type CCTVGateway struct {
	ID         string     `json:"id"`
	OutletID   string     `json:"outlet_id"`
	OutletName string     `json:"outlet_name"`
	LastSeen   *time.Time `json:"last_seen"`
	Online     bool       `json:"online"`
	Key        string     `json:"-"`
}

func ListCCTVGateways(scope []string) ([]CCTVGateway, error) {
	args := []any{}
	filter := ""
	if scope != nil {
		filter = " WHERE g.outlet_id=ANY($1::text[])"
		args = append(args, pq.Array(scope))
	}
	rows, err := database.DB.Query(`SELECT g.id,g.outlet_id,o.name,g.last_seen FROM outlet_cctv_gateways g JOIN outlets o ON o.id=g.outlet_id`+filter+` ORDER BY o.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CCTVGateway{}
	for rows.Next() {
		var g CCTVGateway
		if err := rows.Scan(&g.ID, &g.OutletID, &g.OutletName, &g.LastSeen); err != nil {
			return nil, err
		}
		g.ID = strings.TrimSpace(g.ID)
		g.OutletID = strings.TrimSpace(g.OutletID)
		g.Online = g.LastSeen != nil && time.Since(*g.LastSeen) < 45*time.Second
		out = append(out, g)
	}
	return out, rows.Err()
}

func CreateCCTVGateway(outletID string, rotate bool) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	query := `INSERT INTO outlet_cctv_gateways(id,outlet_id,token_hash) VALUES($1,$2,$3)`
	if rotate {
		query += ` ON CONFLICT(outlet_id) DO UPDATE SET token_hash=EXCLUDED.token_hash,last_seen=NULL`
	}
	_, err := database.DB.Exec(query, NewULID(), outletID, hex.EncodeToString(hash[:]))
	if err != nil {
		return "", err
	}
	if rotate {
		invalidateGatewayStreams(outletID)
	}
	return token, nil
}

func DeleteCCTVGateway(outletID string) error {
	result, err := database.DB.Exec(`DELETE FROM outlet_cctv_gateways WHERE outlet_id=$1`, outletID)
	if err != nil {
		return ErrAuthUnavailable
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	invalidateGatewayStreams(outletID)
	return nil
}

func invalidateGatewayStreams(outletID string) {
	cctvStreams.Lock()
	cameras := map[string]models.OutletCamera{}
	for _, session := range cctvStreams.sessions {
		if session.camera.OutletID == outletID && session.camera.AccessMode == "gateway" {
			cameras[session.camera.ID] = session.camera
		}
	}
	cctvStreams.Unlock()
	for _, cam := range cameras {
		invalidateCameraStreams(cam)
	}
}

func AuthenticateCCTVGateway(token string) (string, error) {
	if len(token) != 64 {
		return "", ErrAuthTokenInvalid
	}
	hash := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(hash[:])
	var id string
	if err := database.DB.QueryRow(`UPDATE outlet_cctv_gateways SET last_seen=NOW() WHERE token_hash=$1 RETURNING id`, key).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrAuthTokenInvalid
		}
		return "", ErrAuthUnavailable
	}
	return strings.TrimSpace(id) + ":" + key, nil
}

func loadCCTVGateway(outlet string) (string, error) {
	var id, key string
	var seen *time.Time
	err := database.DB.QueryRow(`SELECT id,token_hash,last_seen FROM outlet_cctv_gateways WHERE outlet_id=$1`, outlet).Scan(&id, &key, &seen)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (seen == nil || time.Since(*seen) > 45*time.Second) {
		return "", ErrCCTVGatewayOffline
	}
	if err != nil {
		return "", ErrAuthUnavailable
	}
	return strings.TrimSpace(id) + ":" + key, nil
}

type gatewayPending struct {
	key    string
	result chan cctvrelay.Response
}
type gatewayQueue struct {
	jobs chan cctvrelay.Request
	used time.Time
}

var gatewayHub = struct {
	sync.Mutex
	queues  map[string]*gatewayQueue
	pending map[string]gatewayPending
}{queues: map[string]*gatewayQueue{}, pending: map[string]gatewayPending{}}

func gatewayJobs(key string) chan cctvrelay.Request {
	gatewayHub.Lock()
	defer gatewayHub.Unlock()
	now := time.Now()
	for k, q := range gatewayHub.queues {
		if now.Sub(q.used) > time.Minute {
			delete(gatewayHub.queues, k)
		}
	}
	q := gatewayHub.queues[key]
	if q == nil {
		q = &gatewayQueue{jobs: make(chan cctvrelay.Request, 32)}
		gatewayHub.queues[key] = q
	}
	q.used = now
	return q.jobs
}

func PollCCTVGateway(ctx context.Context, key string) []cctvrelay.Request {
	queue := gatewayJobs(key)
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case job := <-queue:
			gatewayHub.Lock()
			pending, ok := gatewayHub.pending[job.ID]
			gatewayHub.Unlock()
			if ok && pending.key == key {
				return []cctvrelay.Request{job}
			}
		case <-ctx.Done():
			return []cctvrelay.Request{}
		case <-timer.C:
			return []cctvrelay.Request{}
		}
	}
}

func CompleteCCTVGateway(key string, response cctvrelay.Response) bool {
	if len(response.Body) > cctvrelay.MaxBody || response.Status < 100 || response.Status > 599 {
		return false
	}
	gatewayHub.Lock()
	pending, ok := gatewayHub.pending[response.ID]
	if ok && pending.key == key {
		delete(gatewayHub.pending, response.ID)
	} else {
		ok = false
	}
	gatewayHub.Unlock()
	if ok {
		pending.result <- response
	}
	return ok
}

func requestCCTVGateway(ctx context.Context, key string, request cctvrelay.Request) (cctvrelay.Response, error) {
	if !cctvrelay.ValidRequest(request) {
		return cctvrelay.Response{}, ErrCCTVAccessDenied
	}
	request.ID = NewULID()
	queue := gatewayJobs(key)
	result := make(chan cctvrelay.Response, 1)
	gatewayHub.Lock()
	if len(gatewayHub.pending) >= 256 {
		gatewayHub.Unlock()
		return cctvrelay.Response{}, ErrCCTVGatewayOffline
	}
	gatewayHub.pending[request.ID] = gatewayPending{key: key, result: result}
	gatewayHub.Unlock()
	defer func() { gatewayHub.Lock(); delete(gatewayHub.pending, request.ID); gatewayHub.Unlock() }()
	select {
	case queue <- request:
	default:
		return cctvrelay.Response{}, ErrCCTVGatewayOffline
	}
	select {
	case response := <-result:
		return response, nil
	case <-ctx.Done():
		return cctvrelay.Response{}, ErrCCTVGatewayOffline
	}
}

// cameraRelayRequest memilih penghubung HTTPS outlet atau relay cloud.
func cameraRelayRequest(cam models.OutletCamera, method, path string, query url.Values) (cctvrelay.Response, error) {
	if cam.AccessMode == "gateway" {
		key, err := loadCCTVGateway(cam.OutletID)
		if err != nil {
			return cctvrelay.Response{}, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return requestCCTVGateway(ctx, key, cctvrelay.Request{Method: method, Path: path, Query: query.Encode()})
	}
	req, err := http.NewRequest(method, go2rtcBaseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return cctvrelay.Response{}, ErrCCTVRelayUnavailable
	}
	response, err := cctvHTTP.Do(req)
	if err != nil {
		return cctvrelay.Response{}, ErrCCTVRelayUnavailable
	}
	defer response.Body.Close()
	return readCCTVRelayResponse(response)
}

func readCCTVRelayResponse(response *http.Response) (cctvrelay.Response, error) {
	result := cctvrelay.Response{Status: response.StatusCode, ContentType: response.Header.Get("Content-Type")}
	if response.StatusCode >= 300 {
		return result, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, cctvrelay.MaxBody+1))
	if err != nil || len(body) > cctvrelay.MaxBody {
		return result, ErrCCTVRelayUnavailable
	}
	result.Body = body
	return result, nil
}

func publishCameraStream(cam models.OutletCamera, name, source string) error {
	if cam.AccessMode != "gateway" {
		return Go2rtcPublish(name, source)
	}
	response, err := cameraRelayRequest(cam, http.MethodPatch, "/api/streams", url.Values{"name": {name}, "src": {source}})
	if err != nil {
		return err
	}
	if response.Status >= 300 {
		return ErrCCTVRelayUnavailable
	}
	return nil
}

func deleteCameraStream(cam models.OutletCamera, name string) error {
	if cam.AccessMode != "gateway" {
		return Go2rtcDelete(name)
	}
	response, err := cameraRelayRequest(cam, http.MethodDelete, "/api/streams", url.Values{"src": {name}})
	if err != nil {
		return err
	}
	if response.Status >= 300 {
		return ErrCCTVRelayUnavailable
	}
	return nil
}

func TestCameraConnection(id string) error {
	cam, pass, source, _, err := getCameraConnectionSecrets(id, true)
	if err != nil {
		return err
	}
	return probeCameraConnection(*cam, pass, source)
}

// Probe tidak menulis kamera atau mengirim kembali URL/kredensial ke browser.
// Saat mengedit, field rahasia yang tidak dikirim tetap memakai nilai tersimpan.
func ProbeCameraConfiguration(outletID string, req models.CameraProbeRequest) error {
	cam, password, source, err := prepareCameraProbe(outletID, req)
	if err != nil {
		return err
	}
	return probeCameraConnection(cam, password, source)
}

func prepareCameraProbe(outletID string, req models.CameraProbeRequest) (models.OutletCamera, string, string, error) {
	cam := &models.OutletCamera{ID: NewULID(), OutletID: outletID}
	password, source, playback := "", "", ""
	if req.CameraID != "" {
		var err error
		cam, password, source, playback, err = getCameraConnectionSecrets(req.CameraID, false)
		if err != nil {
			return models.OutletCamera{}, "", "", err
		}
		if cam.OutletID != outletID {
			return models.OutletCamera{}, "", "", ErrCCTVAccessDenied
		}
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Tes kamera"
	}
	prepared, password, source, _, err := prepareCameraUpdate(cam, req.UpdateCameraRequest, password, source, playback)
	if err != nil {
		return models.OutletCamera{}, "", "", err
	}
	return prepared, password, source, nil
}

func probeCameraConnection(cam models.OutletCamera, pass, source string) error {
	src, err := resolveCameraSource(cam, pass, source)
	if err != nil {
		return err
	}
	name := "check_" + cam.ID + "_" + strings.ToLower(NewULID()[18:])
	if err := publishCameraStream(cam, name, src); err != nil {
		return err
	}
	defer deleteCameraStream(cam, name)
	response, err := cameraRelayRequest(cam, http.MethodGet, "/api/streams", url.Values{"src": {name}, "video": {""}})
	if err != nil {
		return err
	}
	if response.Status != http.StatusOK {
		return ErrCCTVRelayUnavailable
	}
	return nil
}
