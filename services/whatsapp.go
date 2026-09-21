package services

import (
	"bytes"
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

// ── WhatsApp: klien gateway, pengaturan, antrean, pekerja pengirim ──────────
//
// Arsitektur:
//   API utama ──HTTP──▶ wa-gateway (whatsmeow) ──▶ WhatsApp
//
// Pesan TIDAK pernah dikirim langsung dari alur bisnis. Setiap notifikasi
// masuk ke tabel wa_messages sebagai 'pending', lalu satu goroutine pekerja
// mengirimnya satu per satu dengan jeda acak (wa_gap_min_sec..wa_gap_max_sec),
// menghormati jam tenang untuk pesan ke pelanggan, dan batas harian. Alasan:
// jalur WhatsApp Web tidak resmi — nomor yang mengirim beruntun tanpa jeda
// berisiko diblokir, dan pemblokiran itu tidak bisa "diajukan banding".

var (
	waGatewayURL   string
	waGatewayToken string
	waHTTP         = &http.Client{Timeout: 120 * time.Second}

	waStatusMu   sync.Mutex
	waStatusAt   time.Time
	waStatusSnap models.WAGatewayStatus

	waWorkerOnce sync.Once
)

// WAPermanentError: gateway menyatakan pengiriman tidak akan berhasil walau
// diulang (nomor tidak terdaftar, JID salah, gambar tak valid).
type WAPermanentError struct{ Msg string }

func (e WAPermanentError) Error() string { return e.Msg }

func InitWhatsApp(cfg *config.Config) {
	waGatewayURL = strings.TrimRight(strings.TrimSpace(cfg.WAGatewayURL), "/")
	waGatewayToken = cfg.WAGatewayToken
	if waGatewayURL == "" {
		log.Printf("[WA] WA_GATEWAY_URL kosong — notifikasi WhatsApp nonaktif")
		return
	}
	waWorkerOnce.Do(func() {
		go waWorkerLoop()
		go waSchedulerLoop()
	})
	log.Printf("[WA] Pekerja pengirim aktif (gateway %s)", waGatewayURL)
}

func WAConfigured() bool { return waGatewayURL != "" }

// ── Klien gateway ───────────────────────────────────────────────────────────

type waGatewayResp struct {
	OK        bool            `json:"ok"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
	Permanent bool            `json:"permanent"`
}

func waCall(method, path string, body interface{}, out interface{}, timeout time.Duration) error {
	if waGatewayURL == "" {
		return fmt.Errorf("gateway WhatsApp belum dikonfigurasi (WA_GATEWAY_URL)")
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, waGatewayURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+waGatewayToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := waHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("gateway tidak terjangkau: %v", err)
	}
	defer resp.Body.Close()
	var gr waGatewayResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&gr); err != nil {
		return fmt.Errorf("respons gateway tidak valid (HTTP %d)", resp.StatusCode)
	}
	if !gr.OK {
		if gr.Permanent {
			return WAPermanentError{gr.Error}
		}
		if gr.Error == "" {
			gr.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return errors.New(gr.Error)
	}
	if out != nil && len(gr.Data) > 0 {
		return json.Unmarshal(gr.Data, out)
	}
	return nil
}

func waInvalidateStatus() {
	waStatusMu.Lock()
	waStatusAt = time.Time{}
	waStatusMu.Unlock()
}

// WAGatewayStatus membaca /status gateway; di-cache 5 detik supaya pekerja
// dan layar admin tidak membanjiri gateway.
func WAGatewayStatus(force bool) models.WAGatewayStatus {
	waStatusMu.Lock()
	defer waStatusMu.Unlock()
	if !force && time.Since(waStatusAt) < 5*time.Second {
		return waStatusSnap
	}
	s := models.WAGatewayStatus{Configured: waGatewayURL != ""}
	if !s.Configured {
		s.State = "unconfigured"
		s.Note = "WA_GATEWAY_URL belum diisi di compose/env"
	} else if err := waCall(http.MethodGet, "/status", nil, &s, 8*time.Second); err != nil {
		s.Reachable = false
		s.State = "unreachable"
		s.Error = err.Error()
		s.Note = "Layanan wa-gateway tidak merespons: " + err.Error()
	} else {
		s.Reachable = true
	}
	waStatusSnap, waStatusAt = s, time.Now()
	return s
}

func WAGatewayLogin() (models.WAGatewayStatus, error) {
	var s models.WAGatewayStatus
	err := waCall(http.MethodPost, "/login", nil, &s, 20*time.Second)
	waInvalidateStatus()
	if err != nil {
		return s, err
	}
	s.Configured, s.Reachable = true, true
	return s, nil
}

func WAGatewayLogout() error {
	err := waCall(http.MethodPost, "/logout", nil, nil, 30*time.Second)
	waInvalidateStatus()
	return err
}

func WAGatewayGroups() ([]models.WAGroup, error) {
	out := []models.WAGroup{}
	err := waCall(http.MethodGet, "/groups", nil, &out, 30*time.Second)
	return out, err
}

func waGatewaySend(target, body, imageURL string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	err := waCall(http.MethodPost, "/send", map[string]string{
		"to": target, "message": body, "image_url": imageURL,
	}, &out, 110*time.Second)
	return out.ID, err
}

// ── Nomor tujuan ────────────────────────────────────────────────────────────

// WANormalizeTarget: nomor HP → format internasional tanpa '+' (628…);
// JID grup (…@g.us) diteruskan apa adanya. Kosong bila tidak valid.
func WANormalizeTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "@") {
		return raw
	}
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	}
	if len(d) < 9 || len(d) > 16 {
		return ""
	}
	return d
}

// ── Pengaturan ──────────────────────────────────────────────────────────────

const waDefaultPublicBaseURL = "https://pos.nbp.co.id"

func waSettingInt(v string, def, min, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < min || n > max {
		return def
	}
	return n
}

func GetWASettings() models.WASettings {
	vals, _ := GetSettingsByKeys([]string{
		"wa_enabled", "wa_gap_min_sec", "wa_gap_max_sec", "wa_daily_limit",
		"wa_quiet_start", "wa_quiet_end", "wa_report_hour", "wa_reminder_hour",
		"wa_device_offline_min", "wa_public_base_url", "wa_events", "wa_templates", "wa_event_roles",
	})
	s := models.WASettings{
		Enabled:          vals["wa_enabled"] != "false",
		GapMinSec:        waSettingInt(vals["wa_gap_min_sec"], 4, 1, 300),
		GapMaxSec:        waSettingInt(vals["wa_gap_max_sec"], 9, 1, 600),
		DailyLimit:       waSettingInt(vals["wa_daily_limit"], 400, 1, 10000),
		QuietStart:       strings.TrimSpace(vals["wa_quiet_start"]),
		QuietEnd:         strings.TrimSpace(vals["wa_quiet_end"]),
		ReportHour:       waSettingInt(vals["wa_report_hour"], 22, 0, 23),
		ReminderHour:     waSettingInt(vals["wa_reminder_hour"], 10, 0, 23),
		DeviceOfflineMin: waSettingInt(vals["wa_device_offline_min"], 30, 5, 1440),
		PublicBaseURL:    strings.TrimRight(strings.TrimSpace(vals["wa_public_base_url"]), "/"),
		Events:           map[string]bool{},
		Templates:        map[string]string{},
		EventRoles:       map[string][]string{},
	}
	// Belum pernah disetel: jam tenang bawaan 21:30–07:00 untuk pesan ke pelanggan.
	// Nilai "-" berarti admin sengaja mematikannya.
	if s.QuietStart == "" && s.QuietEnd == "" {
		s.QuietStart, s.QuietEnd = "21:30", "07:00"
	}
	if s.QuietStart == "-" {
		s.QuietStart, s.QuietEnd = "", ""
	}
	if s.PublicBaseURL == "" {
		s.PublicBaseURL = waDefaultPublicBaseURL
	}
	if s.GapMaxSec < s.GapMinSec {
		s.GapMaxSec = s.GapMinSec
	}
	if v := vals["wa_events"]; v != "" {
		json.Unmarshal([]byte(v), &s.Events)
	}
	if v := vals["wa_templates"]; v != "" {
		json.Unmarshal([]byte(v), &s.Templates)
	}
	if v := vals["wa_event_roles"]; v != "" {
		json.Unmarshal([]byte(v), &s.EventRoles)
	}
	if s.EventRoles == nil {
		s.EventRoles = map[string][]string{}
	}
	for _, spec := range waEventSpecs {
		if _, ok := s.Events[spec.Key]; !ok {
			s.Events[spec.Key] = true
		}
	}
	return s
}

func waEventEnabled(s models.WASettings, key string) bool {
	v, ok := s.Events[key]
	return !ok || v
}

func waParseHHMM(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func UpdateWASettings(in models.WASettings) error {
	if in.GapMinSec < 1 || in.GapMinSec > 300 || in.GapMaxSec < in.GapMinSec || in.GapMaxSec > 600 {
		return Invalid("jeda antar pesan harus 1–300 detik dan maksimum ≥ minimum")
	}
	if in.DailyLimit < 1 || in.DailyLimit > 10000 {
		return Invalid("batas harian harus 1–10000 pesan")
	}
	if in.ReportHour < 0 || in.ReportHour > 23 || in.ReminderHour < 0 || in.ReminderHour > 23 {
		return Invalid("jam rekap/pengingat harus 0–23")
	}
	if in.DeviceOfflineMin < 5 || in.DeviceOfflineMin > 1440 {
		return Invalid("ambang perangkat offline harus 5–1440 menit")
	}
	qs, qe := strings.TrimSpace(in.QuietStart), strings.TrimSpace(in.QuietEnd)
	if (qs == "") != (qe == "") {
		return Invalid("jam tenang harus diisi keduanya (mulai & selesai) atau dikosongkan keduanya")
	}
	if qs != "" {
		if _, ok := waParseHHMM(qs); !ok {
			return Invalid("format jam tenang mulai harus HH:MM")
		}
		if _, ok := waParseHHMM(qe); !ok {
			return Invalid("format jam tenang selesai harus HH:MM")
		}
	}
	base := strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	if base != "" && !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return Invalid("alamat publik harus diawali http:// atau https://")
	}
	events := map[string]bool{}
	for _, spec := range waEventSpecs {
		if v, ok := in.Events[spec.Key]; ok {
			events[spec.Key] = v
		} else {
			events[spec.Key] = true
		}
	}
	templates := map[string]string{}
	for _, spec := range waEventSpecs {
		t := strings.TrimSpace(in.Templates[spec.Key])
		if t != "" && t != strings.TrimSpace(spec.Template) {
			templates[spec.Key] = t
		}
	}
	// Penimpa posisi: hanya simpan event yang daftarnya terisi.
	roleSet := map[string][]string{}
	for _, spec := range waEventSpecs {
		list := []string{}
		for _, r := range in.EventRoles[spec.Key] {
			if r = strings.TrimSpace(r); r != "" {
				list = append(list, r)
			}
		}
		if len(list) > 0 && spec.Audience == "internal" {
			roleSet[spec.Key] = list
		}
	}
	er, _ := json.Marshal(roleSet)
	ev, _ := json.Marshal(events)
	tp, _ := json.Marshal(templates)
	enabled := "true"
	if !in.Enabled {
		enabled = "false"
	}
	// Jam tenang kosong disimpan sebagai "-" agar bisa dibedakan dari "belum
	// pernah disetel" (yang memakai bawaan 21:30–07:00).
	if qs == "" {
		qs, qe = "-", "-"
	}
	return UpdateSettings(map[string]string{
		"wa_enabled":            enabled,
		"wa_gap_min_sec":        strconv.Itoa(in.GapMinSec),
		"wa_gap_max_sec":        strconv.Itoa(in.GapMaxSec),
		"wa_daily_limit":        strconv.Itoa(in.DailyLimit),
		"wa_quiet_start":        qs,
		"wa_quiet_end":          qe,
		"wa_report_hour":        strconv.Itoa(in.ReportHour),
		"wa_reminder_hour":      strconv.Itoa(in.ReminderHour),
		"wa_device_offline_min": strconv.Itoa(in.DeviceOfflineMin),
		"wa_public_base_url":    base,
		"wa_events":             string(ev),
		"wa_templates":          string(tp),
		"wa_event_roles":        string(er),
	})
}

// waQuietWindow mengembalikan (mulai, selesai) dalam menit sejak 00:00, atau
// ok=false bila jam tenang tidak dipakai.
func waQuietWindow(s models.WASettings) (int, int, bool) {
	if s.QuietStart == "-" || s.QuietStart == "" {
		return 0, 0, false
	}
	a, ok1 := waParseHHMM(s.QuietStart)
	b, ok2 := waParseHHMM(s.QuietEnd)
	if !ok1 || !ok2 || a == b {
		return 0, 0, false
	}
	return a, b, true
}

func waInQuiet(now time.Time, s models.WASettings) bool {
	a, b, ok := waQuietWindow(s)
	if !ok {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if a > b { // melewati tengah malam, mis. 21:30–07:00
		return m >= a || m < b
	}
	return m >= a && m < b
}

// waQuietEnd: waktu terdekat jam tenang berakhir (zona aplikasi).
func waQuietEnd(now time.Time, s models.WASettings) time.Time {
	_, b, ok := waQuietWindow(s)
	if !ok {
		return now
	}
	end := time.Date(now.Year(), now.Month(), now.Day(), b/60, b%60, 0, 0, now.Location())
	if !end.After(now) {
		end = end.Add(24 * time.Hour)
	}
	return end
}

// ── Antrean ─────────────────────────────────────────────────────────────────

// EnqueueWAMessage memasukkan satu pesan ke antrean. Untuk notifikasi
// ber-event, pasangan (event, ref_id, target) unik: pemanggilan ulang (retry
// sync device, cascade status) tidak menggandakan pesan. Mengembalikan "" bila
// pesan dianggap duplikat.
func EnqueueWAMessage(m models.WAMessage) (string, error) {
	target := WANormalizeTarget(m.Target)
	if target == "" {
		return "", Invalid("nomor tujuan tidak valid: %s", m.Target)
	}
	if strings.TrimSpace(m.Body) == "" && m.ImageURL == "" {
		return "", Invalid("isi pesan kosong")
	}
	if m.Audience == "" {
		m.Audience = "internal"
	}
	if m.Kind == "" {
		m.Kind = "notify"
	}
	sched := time.Now().UTC()
	if m.ScheduledAt != "" {
		if t, ok := parseTimeStrict(m.ScheduledAt); ok {
			sched = t.UTC()
		}
	}
	id := NewULID()
	res, err := database.DB.Exec(`
		INSERT INTO wa_messages (id, target, target_name, audience, kind, event, ref_id, broadcast_id,
			body, image_url, status, scheduled_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11,(now() AT TIME ZONE 'UTC'))
		ON CONFLICT DO NOTHING`,
		id, target, m.TargetName, m.Audience, m.Kind, m.Event, m.RefID, nilIfEmpty(m.BroadcastID),
		m.Body, m.ImageURL, sched)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", nil
	}
	return id, nil
}

func waMessageCols() string {
	return fmt.Sprintf(`id, target, target_name, audience, kind, event, ref_id, COALESCE(broadcast_id,''),
		body, image_url, status, attempts, last_error, wa_message_id,
		to_char((scheduled_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
		COALESCE(to_char((sent_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'), ''),
		to_char((created_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI')`, tzExpr)
}

func scanWAMessage(sc interface{ Scan(...interface{}) error }) (models.WAMessage, error) {
	var m models.WAMessage
	err := sc.Scan(&m.ID, &m.Target, &m.TargetName, &m.Audience, &m.Kind, &m.Event, &m.RefID, &m.BroadcastID,
		&m.Body, &m.ImageURL, &m.Status, &m.Attempts, &m.LastError, &m.WAMessageID,
		&m.ScheduledAt, &m.SentAt, &m.CreatedAt)
	m.ID = strings.TrimSpace(m.ID)
	m.BroadcastID = strings.TrimSpace(m.BroadcastID)
	return m, err
}

func ListWAMessages(status, kind, broadcastID, search string, page, limit int) ([]models.WAMessage, int, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	add := func(cond string, v interface{}) {
		conds = append(conds, fmt.Sprintf(cond, idx))
		args = append(args, v)
		idx++
	}
	if status != "" {
		add("status = $%d", status)
	}
	if kind != "" {
		add("kind = $%d", kind)
	}
	if broadcastID != "" {
		add("broadcast_id = $%d", broadcastID)
	}
	if s := strings.TrimSpace(search); s != "" {
		add("(target ILIKE $%[1]d OR target_name ILIKE $%[1]d OR body ILIKE $%[1]d OR event ILIKE $%[1]d)", "%"+s+"%")
	}
	where := strings.Join(conds, " AND ")
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM wa_messages WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`SELECT %s FROM wa_messages WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		waMessageCols(), where, idx, idx+1)
	args = append(args, limit, (page-1)*limit)
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]models.WAMessage, 0)
	for rows.Next() {
		m, err := scanWAMessage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func RetryWAMessage(id string) error {
	res, err := database.DB.Exec(`UPDATE wa_messages SET status='pending', attempts=0, last_error='',
		scheduled_at=(now() AT TIME ZONE 'UTC') WHERE id=$1 AND status IN ('failed','cancelled')`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Invalid("pesan tidak ditemukan atau tidak berstatus gagal/dibatalkan")
	}
	return nil
}

func CancelWAMessage(id string) error {
	res, err := database.DB.Exec(`UPDATE wa_messages SET status='cancelled', last_error='dibatalkan admin'
		WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Invalid("pesan tidak ditemukan atau sudah terkirim")
	}
	return nil
}

func RetryFailedWAMessages() (int, error) {
	res, err := database.DB.Exec(`UPDATE wa_messages SET status='pending', attempts=0, last_error='',
		scheduled_at=(now() AT TIME ZONE 'UTC') WHERE status='failed'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func waToday() string {
	return time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
}

func WAQueueStats() models.WAQueueStats {
	var st models.WAQueueStats
	today := waToday()
	database.DB.QueryRow(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'sent' AND sent_at >= tz_day_start($1::date)),
			COUNT(*) FILTER (WHERE status = 'failed' AND created_at >= tz_day_start($1::date)),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COALESCE(to_char((MAX(sent_at) AT TIME ZONE 'UTC') AT TIME ZONE `+tzExpr+`, 'YYYY-MM-DD HH24:MI'), '')
		FROM wa_messages`, today).Scan(&st.Pending, &st.SentToday, &st.FailedToday, &st.Failed, &st.LastSentAt)
	st.DailyLimit = GetWASettings().DailyLimit
	return st
}

// SendWATest mengirim langsung (tanpa antrean) supaya admin dapat umpan balik
// seketika; hasilnya tetap dicatat di log pesan.
func SendWATest(target, body, actor string) (models.WAMessage, error) {
	target = WANormalizeTarget(target)
	if target == "" {
		return models.WAMessage{}, Invalid("nomor tujuan tidak valid")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = "Pesan uji dari Cloud POS — gateway WhatsApp aktif."
	}
	id := NewULID()
	database.DB.Exec(`INSERT INTO wa_messages (id, target, target_name, audience, kind, body, status, created_at, scheduled_at)
		VALUES ($1,$2,$3,'internal','test',$4,'pending',(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
		id, target, "uji oleh "+actor, body)
	msgID, err := waGatewaySend(target, body, "")
	if err != nil {
		database.DB.Exec(`UPDATE wa_messages SET status='failed', attempts=1, last_error=$2 WHERE id=$1`, id, err.Error())
		return models.WAMessage{}, err
	}
	database.DB.Exec(`UPDATE wa_messages SET status='sent', attempts=1, wa_message_id=$2, sent_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, id, msgID)
	row := database.DB.QueryRow(`SELECT `+waMessageCols()+` FROM wa_messages WHERE id=$1`, id)
	return scanWAMessage(row)
}

// ── Penerima notifikasi internal ────────────────────────────────────────────

func waSplitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func scanWARecipient(sc interface{ Scan(...interface{}) error }) (models.WARecipient, error) {
	var r models.WARecipient
	var events, outlets string
	err := sc.Scan(&r.ID, &r.Name, &r.Target, &r.Kind, &events, &outlets, &r.IsActive, &r.Notes, &r.CreatedAt, &r.UpdatedAt)
	r.ID = strings.TrimSpace(r.ID)
	r.Events = waSplitList(events)
	r.OutletIDs = waSplitList(outlets)
	return r, err
}

func waRecipientCols() string {
	return fmt.Sprintf(`id, name, target, kind, events, outlet_ids, is_active, notes,
		to_char((created_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
		to_char((updated_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI')`, tzExpr)
}

func ListWARecipients() ([]models.WARecipient, error) {
	rows, err := database.DB.Query(`SELECT ` + waRecipientCols() + ` FROM wa_recipients ORDER BY name, target`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.WARecipient, 0)
	for rows.Next() {
		r, err := scanWARecipient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func validateWARecipient(r *models.WARecipient) error {
	r.Name = strings.TrimSpace(r.Name)
	r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
	if r.Kind == "" {
		r.Kind = "phone"
		if strings.HasSuffix(r.Target, "@g.us") {
			r.Kind = "group"
		}
	}
	if r.Kind != "phone" && r.Kind != "group" {
		return Invalid("jenis penerima harus 'phone' atau 'group'")
	}
	r.Target = WANormalizeTarget(r.Target)
	if r.Target == "" {
		return Invalid("nomor/JID tujuan tidak valid")
	}
	if r.Kind == "group" && !strings.HasSuffix(r.Target, "@g.us") {
		return Invalid("JID grup harus berakhiran @g.us (pilih dari daftar grup)")
	}
	if r.Kind == "phone" && strings.Contains(r.Target, "@") {
		return Invalid("nomor HP tidak boleh berisi '@'")
	}
	if r.Name == "" {
		r.Name = r.Target
	}
	clean := []string{}
	for _, e := range r.Events {
		e = strings.TrimSpace(e)
		if e == "*" {
			clean = []string{"*"}
			break
		}
		if spec := waSpec(e); spec != nil && spec.Audience == "internal" {
			clean = append(clean, e)
		}
	}
	if len(clean) == 0 {
		clean = []string{"*"}
	}
	r.Events = clean
	return nil
}

func CreateWARecipient(r models.WARecipient) (*models.WARecipient, error) {
	if err := validateWARecipient(&r); err != nil {
		return nil, err
	}
	id := NewULID()
	_, err := database.DB.Exec(`INSERT INTO wa_recipients (id, name, target, kind, events, outlet_ids, is_active, notes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
		id, r.Name, r.Target, r.Kind, strings.Join(r.Events, ","), strings.Join(r.OutletIDs, ","), r.IsActive, r.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "uq_wa_recipients_target") {
			return nil, Invalid("nomor/grup ini sudah terdaftar sebagai penerima")
		}
		return nil, err
	}
	return GetWARecipient(id)
}

func GetWARecipient(id string) (*models.WARecipient, error) {
	row := database.DB.QueryRow(`SELECT `+waRecipientCols()+` FROM wa_recipients WHERE id=$1`, id)
	r, err := scanWARecipient(row)
	if err != nil {
		return nil, fmt.Errorf("penerima tidak ditemukan")
	}
	return &r, nil
}

func UpdateWARecipient(id string, r models.WARecipient) (*models.WARecipient, error) {
	if err := validateWARecipient(&r); err != nil {
		return nil, err
	}
	res, err := database.DB.Exec(`UPDATE wa_recipients SET name=$2, target=$3, kind=$4, events=$5, outlet_ids=$6,
		is_active=$7, notes=$8, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`,
		id, r.Name, r.Target, r.Kind, strings.Join(r.Events, ","), strings.Join(r.OutletIDs, ","), r.IsActive, r.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "uq_wa_recipients_target") {
			return nil, Invalid("nomor/grup ini sudah terdaftar sebagai penerima lain")
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("penerima tidak ditemukan")
	}
	return GetWARecipient(id)
}

func DeleteWARecipient(id string) error {
	res, err := database.DB.Exec(`DELETE FROM wa_recipients WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("penerima tidak ditemukan")
	}
	return nil
}

// ── Pekerja pengirim ────────────────────────────────────────────────────────

func waWorkerLoop() {
	var lastState string
	for {
		d, state := waProcessNext()
		if state != "" && state != lastState {
			log.Printf("[WA] %s", state)
			lastState = state
		}
		time.Sleep(d)
	}
}

func waReschedule(id string, at time.Time, note string) {
	database.DB.Exec(`UPDATE wa_messages SET scheduled_at=$2, last_error=$3 WHERE id=$1 AND status='pending'`,
		id, at.UTC(), note)
}

// waProcessNext mengirim maksimal satu pesan lalu mengembalikan berapa lama
// pekerja harus diam sebelum mencoba lagi (+ pesan status untuk log).
func waProcessNext() (time.Duration, string) {
	s := GetWASettings()
	if !s.Enabled {
		return 20 * time.Second, "pengiriman dimatikan dari pengaturan"
	}
	// Notifikasi yang tertahan > 24 jam (gateway lama tidak tersambung) tidak
	// relevan lagi bila dikirim sekarang: batalkan, jangan bocorkan informasi
	// basi ke penerima.
	database.DB.Exec(`UPDATE wa_messages SET status='cancelled',
		last_error='kedaluwarsa: menunggu > 24 jam (gateway tidak tersambung?)'
		WHERE status='pending' AND kind='notify'
		  AND created_at < (now() AT TIME ZONE 'UTC') - interval '24 hours'`)

	var m models.WAMessage
	err := database.DB.QueryRow(`SELECT id, target, audience, kind, body, image_url, attempts
		FROM wa_messages WHERE status='pending' AND scheduled_at <= (now() AT TIME ZONE 'UTC')
		ORDER BY scheduled_at ASC, created_at ASC LIMIT 1`).
		Scan(&m.ID, &m.Target, &m.Audience, &m.Kind, &m.Body, &m.ImageURL, &m.Attempts)
	if err == sql.ErrNoRows {
		return 3 * time.Second, ""
	}
	if err != nil {
		return 10 * time.Second, "antrean tidak terbaca: " + err.Error()
	}
	m.ID = strings.TrimSpace(m.ID)

	loc := GetTimezoneLocation()
	now := time.Now().In(loc)
	if m.Audience == "customer" && waInQuiet(now, s) {
		end := waQuietEnd(now, s)
		waReschedule(m.ID, end, "ditunda: jam tenang sampai "+end.Format("02-01 15:04"))
		return 0, ""
	}
	var sentToday int
	database.DB.QueryRow(`SELECT COUNT(*) FROM wa_messages WHERE status='sent' AND sent_at >= tz_day_start($1::date)`,
		now.Format("2006-01-02")).Scan(&sentToday)
	if sentToday >= s.DailyLimit {
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 7, 0, 0, 0, loc)
		waReschedule(m.ID, next, fmt.Sprintf("ditunda: batas harian %d pesan tercapai", s.DailyLimit))
		return 0, fmt.Sprintf("batas harian %d tercapai, sisa antrean ditunda ke besok", s.DailyLimit)
	}

	st := WAGatewayStatus(false)
	if !st.Reachable {
		return 30 * time.Second, "gateway tidak terjangkau: " + st.Error
	}
	if !st.LoggedIn || !st.Connected {
		return 15 * time.Second, "gateway belum tersambung (" + st.State + "), antrean menunggu"
	}

	msgID, err := waGatewaySend(m.Target, m.Body, m.ImageURL)
	if err == nil {
		database.DB.Exec(`UPDATE wa_messages SET status='sent', attempts=attempts+1, last_error='',
			wa_message_id=$2, sent_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, m.ID, msgID)
		gap := s.GapMinSec
		if s.GapMaxSec > s.GapMinSec {
			gap += rand.Intn(s.GapMaxSec - s.GapMinSec + 1)
		}
		return time.Duration(gap) * time.Second, "mengirim (jeda " + strconv.Itoa(s.GapMinSec) + "–" + strconv.Itoa(s.GapMaxSec) + " dtk)"
	}
	var pe WAPermanentError
	attempts := m.Attempts + 1
	if errors.As(err, &pe) || attempts >= 3 {
		database.DB.Exec(`UPDATE wa_messages SET status='failed', attempts=$2, last_error=$3 WHERE id=$1`,
			m.ID, attempts, err.Error())
		return 2 * time.Second, "pesan gagal permanen: " + err.Error()
	}
	backoff := []time.Duration{3 * time.Minute, 15 * time.Minute}[attempts-1]
	database.DB.Exec(`UPDATE wa_messages SET attempts=$2, last_error=$3, scheduled_at=$4 WHERE id=$1`,
		m.ID, attempts, err.Error(), time.Now().UTC().Add(backoff))
	waInvalidateStatus()
	return 5 * time.Second, "kirim gagal, diulang nanti: " + err.Error()
}

// ── Penjadwal ───────────────────────────────────────────────────────────────

func waMarker(key string) string {
	v, _ := GetSetting(key)
	return v
}

func waSchedulerLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s := GetWASettings()
		if !s.Enabled {
			continue
		}
		loc := GetTimezoneLocation()
		now := time.Now().In(loc)
		day := now.Format("2006-01-02")
		if now.Hour() == s.ReportHour && waMarker("wa_last_daily_report") != day {
			UpdateSettings(map[string]string{"wa_last_daily_report": day})
			go SendDailySalesSummary(day)
		}
		if now.Hour() == s.ReminderHour && waMarker("wa_last_reminder") != day {
			UpdateSettings(map[string]string{"wa_last_reminder": day})
			go SendReservationReminders(now)
		}
		if now.Minute()%10 == 0 {
			go CheckDeviceOffline(s)
		}
	}
}

// waScopeArray membungkus scope outlet untuk query (nil = semua).
func waScopeArray(ids []string) interface{} {
	if ids == nil {
		return nil
	}
	return pq.Array(ids)
}
