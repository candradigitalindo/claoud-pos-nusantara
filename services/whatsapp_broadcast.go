package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// ── Broadcast WhatsApp ──────────────────────────────────────────────────────
//
// Kampanye kirim massal: ke pelanggan (master pelanggan dari order kasir,
// disaring outlet/kunjungan), ke daftar penerima internal, atau ke daftar
// nomor manual. Setiap tujuan menjadi satu baris wa_messages berjenis
// 'broadcast', dan pekerja pengirim yang sama mengirimnya dengan jeda +
// jam tenang + batas harian. Progres dihitung dari status baris-baris itu.

const waBroadcastMaxTargets = 5000

// waParseManual: "0812…, Nama" per baris → target ternormalisasi, dedup.
func waParseManual(raw string) []models.WATarget {
	seen := map[string]bool{}
	out := []models.WATarget{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		num, name := line, ""
		for _, sep := range []string{",", ";", "\t", " - "} {
			if i := strings.Index(line, sep); i > 0 {
				num, name = strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len(sep):])
				break
			}
		}
		t := WANormalizeTarget(num)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, models.WATarget{Target: t, Name: name})
	}
	return out
}

// WABroadcastAudience menyelesaikan daftar tujuan sebuah broadcast.
func WABroadcastAudience(req models.WABroadcastRequest, scope []string) ([]models.WATarget, error) {
	switch req.Audience {
	case "customers":
		return waCustomerAudience(req, scope)
	case "recipients":
		list, err := ListWARecipients()
		if err != nil {
			return nil, err
		}
		out := []models.WATarget{}
		for _, r := range list {
			if r.IsActive {
				out = append(out, models.WATarget{Target: r.Target, Name: r.Name})
			}
		}
		return out, nil
	case "manual":
		return waParseManual(req.Manual), nil
	}
	return nil, Invalid("audiens harus 'customers', 'recipients', atau 'manual'")
}

func waCustomerAudience(req models.WABroadcastRequest, scope []string) ([]models.WATarget, error) {
	args := []interface{}{}
	idx := 1
	join := "o.customer_id = c.id"
	if scope != nil {
		join += fmt.Sprintf(" AND o.outlet_id = ANY($%d::text[])", idx)
		args = append(args, pq.Array(scope))
		idx++
	}
	if req.OutletID != "" {
		join += fmt.Sprintf(" AND o.outlet_id = $%d", idx)
		args = append(args, req.OutletID)
		idx++
	}
	having := []string{}
	minVisits := req.MinVisits
	if minVisits < 1 {
		minVisits = 1
	}
	having = append(having, fmt.Sprintf("COUNT(o.id) >= $%d", idx))
	args = append(args, minVisits)
	idx++
	if req.LastVisitDays > 0 {
		having = append(having, fmt.Sprintf("MAX(o.created_at) >= (now() AT TIME ZONE 'UTC') - ($%d * interval '1 day')", idx))
		args = append(args, req.LastVisitDays)
		idx++
	}
	if req.NotVisitedDays > 0 {
		having = append(having, fmt.Sprintf("MAX(o.created_at) < (now() AT TIME ZONE 'UTC') - ($%d * interval '1 day')", idx))
		args = append(args, req.NotVisitedDays)
		idx++
	}
	q := fmt.Sprintf(`SELECT c.name, c.phone FROM customers c JOIN cloud_orders o ON %s
		WHERE c.phone <> ''
		GROUP BY c.id, c.name, c.phone
		HAVING %s
		ORDER BY MAX(o.created_at) DESC LIMIT %d`, join, strings.Join(having, " AND "), waBroadcastMaxTargets)
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	out := []models.WATarget{}
	for rows.Next() {
		var name, phone string
		if rows.Scan(&name, &phone) != nil {
			continue
		}
		t := WANormalizeTarget(phone)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, models.WATarget{Target: t, Name: strings.TrimSpace(name)})
	}
	return out, rows.Err()
}

func WAPreviewBroadcast(req models.WABroadcastRequest, scope []string) (*models.WAAudiencePreview, error) {
	list, err := WABroadcastAudience(req, scope)
	if err != nil {
		return nil, err
	}
	sample := list
	if len(sample) > 8 {
		sample = sample[:8]
	}
	return &models.WAAudiencePreview{Total: len(list), Sample: sample}, nil
}

func waBroadcastFilter(req models.WABroadcastRequest) map[string]interface{} {
	f := map[string]interface{}{}
	if req.Audience == "customers" {
		f["outlet_id"] = req.OutletID
		f["min_visits"] = req.MinVisits
		f["last_visit_days"] = req.LastVisitDays
		f["not_visited_days"] = req.NotVisitedDays
	}
	if req.Audience == "manual" {
		f["manual_lines"] = len(waParseManual(req.Manual))
	}
	if req.SendAt != "" {
		f["send_at"] = req.SendAt
	}
	return f
}

func CreateWABroadcast(req models.WABroadcastRequest, actor string, scope []string) (*models.WABroadcast, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" {
		return nil, Invalid("judul broadcast wajib diisi")
	}
	if req.Body == "" {
		return nil, Invalid("isi pesan wajib diisi")
	}
	if req.ImageURL != "" && !strings.HasPrefix(req.ImageURL, "http://") && !strings.HasPrefix(req.ImageURL, "https://") {
		return nil, Invalid("URL gambar harus diawali http:// atau https://")
	}
	sched := time.Now().UTC()
	if s := strings.TrimSpace(req.SendAt); s != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04", s, GetTimezoneLocation())
		if err != nil {
			return nil, Invalid("format jadwal kirim harus YYYY-MM-DD HH:MM")
		}
		sched = t.UTC()
	}
	targets, err := WABroadcastAudience(req, scope)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, Invalid("tidak ada penerima yang cocok dengan pilihan audiens")
	}
	audience := "customer"
	if req.Audience == "recipients" {
		audience = "internal"
	}
	filterJSON, _ := json.Marshal(waBroadcastFilter(req))

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	id := NewULID()
	if _, err := tx.Exec(`INSERT INTO wa_broadcasts (id, title, body, image_url, audience, filter, total, status, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'queued',$8,(now() AT TIME ZONE 'UTC'))`,
		id, req.Title, req.Body, req.ImageURL, req.Audience, string(filterJSON), len(targets), actor); err != nil {
		return nil, err
	}
	stmt, err := tx.Prepare(`INSERT INTO wa_messages (id, target, target_name, audience, kind, broadcast_id, body, image_url, status, scheduled_at, created_at)
		VALUES ($1,$2,$3,$4,'broadcast',$5,$6,$7,'pending',$8,(now() AT TIME ZONE 'UTC'))`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	for _, t := range targets {
		name := strings.TrimSpace(t.Name)
		body := strings.ReplaceAll(req.Body, "{name}", waOr(name, "Kak"))
		if _, err := stmt.Exec(NewULID(), t.Target, name, audience, id, body, req.ImageURL, sched); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetWABroadcast(id)
}

func waBroadcastCols() string {
	return fmt.Sprintf(`b.id, b.title, b.body, b.image_url, b.audience, b.filter::text, b.total, b.status, b.created_by,
		to_char((b.created_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'),
		COALESCE(to_char((b.finished_at AT TIME ZONE 'UTC') AT TIME ZONE %[1]s, 'YYYY-MM-DD HH24:MI'), ''),
		(SELECT COUNT(*) FROM wa_messages m WHERE m.broadcast_id = b.id AND m.status = 'sent'),
		(SELECT COUNT(*) FROM wa_messages m WHERE m.broadcast_id = b.id AND m.status = 'failed'),
		(SELECT COUNT(*) FROM wa_messages m WHERE m.broadcast_id = b.id AND m.status = 'pending')`, tzExpr)
}

func scanWABroadcast(sc interface{ Scan(...interface{}) error }) (models.WABroadcast, error) {
	var b models.WABroadcast
	var filter string
	err := sc.Scan(&b.ID, &b.Title, &b.Body, &b.ImageURL, &b.Audience, &filter, &b.Total, &b.Status, &b.CreatedBy,
		&b.CreatedAt, &b.FinishedAt, &b.Sent, &b.Failed, &b.Pending)
	if err != nil {
		return b, err
	}
	b.ID = strings.TrimSpace(b.ID)
	b.Filter = map[string]interface{}{}
	json.Unmarshal([]byte(filter), &b.Filter)
	if b.Status == "queued" && b.Pending == 0 {
		b.Status = "done"
		database.DB.Exec(`UPDATE wa_broadcasts SET status='done', finished_at=COALESCE(finished_at,(now() AT TIME ZONE 'UTC')) WHERE id=$1 AND status='queued'`, b.ID)
	}
	return b, nil
}

func ListWABroadcasts(page, limit int) ([]models.WABroadcast, int, error) {
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM wa_broadcasts`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(`SELECT `+waBroadcastCols()+` FROM wa_broadcasts b ORDER BY b.created_at DESC LIMIT $1 OFFSET $2`,
		limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]models.WABroadcast, 0)
	for rows.Next() {
		b, err := scanWABroadcast(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

func GetWABroadcast(id string) (*models.WABroadcast, error) {
	row := database.DB.QueryRow(`SELECT `+waBroadcastCols()+` FROM wa_broadcasts b WHERE b.id = $1`, id)
	b, err := scanWABroadcast(row)
	if err != nil {
		return nil, fmt.Errorf("broadcast tidak ditemukan")
	}
	return &b, nil
}

func CancelWABroadcast(id string) (int, error) {
	res, err := database.DB.Exec(`UPDATE wa_messages SET status='cancelled', last_error='broadcast dibatalkan'
		WHERE broadcast_id=$1 AND status='pending'`, id)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	database.DB.Exec(`UPDATE wa_broadcasts SET status='cancelled', finished_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, id)
	return int(n), nil
}
