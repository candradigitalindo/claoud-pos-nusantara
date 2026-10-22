package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrUsageConflict = errors.New("laporan sudah berubah; muat ulang sebelum melanjutkan")
var ErrUsageForbidden = errors.New("aksi ini tidak diizinkan untuk akun Anda")

const usageCols = `r.id,r.report_number,r.project_id,p.name,COALESCE(p.project_number,''),
	r.kind,TO_CHAR(r.usage_date,'YYYY-MM-DD'),r.location,r.work_description,r.performer,r.pic_name,
	r.items,r.posted_items,r.evidence,r.status,r.version,r.created_by,r.created_by_id,r.submitted_by_id,
	r.reviewed_by,r.review_note,r.submitted_at,r.reviewed_at,r.created_at,r.updated_at`

func scanProjectUsage(s interface{ Scan(...interface{}) error }) (*models.ProjectUsageReport, error) {
	r := &models.ProjectUsageReport{}
	var items, posted, evidence []byte
	err := s.Scan(&r.ID, &r.ReportNumber, &r.ProjectID, &r.ProjectName, &r.ProjectNumber, &r.Kind, &r.UsageDate,
		&r.Location, &r.WorkDescription, &r.Performer, &r.PICName, &items, &posted, &evidence, &r.Status, &r.Version,
		&r.CreatedBy, &r.CreatedByID, &r.SubmittedByID, &r.ReviewedBy, &r.ReviewNote, &r.SubmittedAt, &r.ReviewedAt, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(items, &r.Items); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(posted, &r.PostedItems); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(evidence, &r.Evidence); err != nil {
		return nil, err
	}
	return r, nil
}

func GetProjectUsage(id string) (*models.ProjectUsageReport, error) {
	r, err := scanProjectUsage(database.DB.QueryRow(`SELECT `+usageCols+` FROM project_usage_reports r JOIN projects p ON p.id=r.project_id WHERE r.id=$1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT action,actor,note,snapshot,created_at FROM project_usage_events WHERE report_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	r.Events = []models.ProjectUsageEvent{}
	for rows.Next() {
		var e models.ProjectUsageEvent
		var raw []byte
		if err = rows.Scan(&e.Action, &e.Actor, &e.Note, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &e.Snapshot); err != nil {
			return nil, err
		}
		r.Events = append(r.Events, e)
	}
	return r, rows.Err()
}

func ListProjectUsage(projectID, status, search, from, to string, outlets, units []string, page, limit int) ([]models.ProjectUsageReport, int, error) {
	where, args := prScopeCond("p", outlets, units, 1)
	for _, f := range []struct{ value, sql string }{{projectID, "r.project_id ="}, {status, "r.status ="}, {from, "r.usage_date >="}, {to, "r.usage_date <="}} {
		if f.value != "" {
			args = append(args, f.value)
			where += fmt.Sprintf(" AND %s $%d", f.sql, len(args))
		}
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (r.report_number ILIKE $%d OR p.name ILIKE $%d OR r.work_description ILIKE $%d)", n, n, n)
	}
	for _, date := range []string{from, to} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return nil, 0, Invalid("tanggal filter tidak valid")
			}
		}
	}
	base := ` FROM project_usage_reports r JOIN projects p ON p.id=r.project_id WHERE true` + where
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := database.DB.Query(`SELECT `+usageCols+base+fmt.Sprintf(` ORDER BY r.usage_date DESC,r.created_at DESC,r.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.ProjectUsageReport{}
	for rows.Next() {
		r, err := scanProjectUsage(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *r)
	}
	return out, total, rows.Err()
}

func usageActorID(a Actor) string {
	if a.ID != "" {
		return a.ID
	}
	return "static-admin"
}

// Every material mutation and project closure takes the project lock first.
func lockMaterialProject(tx *sql.Tx, id string) error {
	var status string
	if err := tx.QueryRow(`SELECT status FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return err
	}
	if status == "selesai" || status == "batal" {
		return Invalid("projek sudah %s; buka kembali sebelum mengubah material", status)
	}
	return nil
}

func validateUsageInput(in *models.ProjectUsageInput, complete bool) error {
	in.Location = strings.TrimSpace(in.Location)
	in.WorkDescription = strings.TrimSpace(in.WorkDescription)
	in.Performer = strings.TrimSpace(in.Performer)
	in.PICName = strings.TrimSpace(in.PICName)
	if in.Kind != "pemakaian" && in.Kind != "susut" {
		return Invalid("jenis laporan harus pemakaian atau susut")
	}
	d, err := time.ParseInLocation("2006-01-02", in.UsageDate, GetTimezoneLocation())
	if err != nil || d.After(time.Now().In(GetTimezoneLocation())) {
		return Invalid("tanggal penggunaan tidak valid atau belum terjadi")
	}
	for _, f := range []struct {
		value string
		max   int
	}{{in.Location, 200}, {in.WorkDescription, 3000}, {in.Performer, 150}, {in.PICName, 150}} {
		if utf8.RuneCountInString(f.value) > f.max {
			return Invalid("isian laporan terlalu panjang")
		}
	}
	if complete && (in.Location == "" || in.WorkDescription == "" || in.Performer == "" || in.PICName == "") {
		return Invalid("lokasi, pekerjaan/alasan, pelaksana, dan PIC wajib diisi")
	}
	if len(in.Items) == 0 || len(in.Items) > 100 {
		return Invalid("isi 1 sampai 100 barang dalam laporan")
	}
	seen := map[string]bool{}
	for _, it := range in.Items {
		if it.MaterialID == "" || seen[it.MaterialID] {
			return Invalid("barang kosong atau tercantum lebih dari sekali")
		}
		seen[it.MaterialID] = true
		if !validMaterialQty(it.Qty) {
			return Invalid("jumlah harus positif, maksimal 4 angka desimal, dan kurang dari 100 miliar")
		}
	}
	if in.Evidence == nil {
		in.Evidence = []string{}
	}
	if len(in.Evidence) > 10 {
		return Invalid("maksimal 10 bukti per laporan")
	}
	seen = map[string]bool{}
	hasPhoto := false
	for _, url := range in.Evidence {
		if seen[url] {
			return Invalid("bukti yang sama dicantumkan dua kali")
		}
		seen[url] = true
		photo, err := validateMaterialEvidence(url)
		if err != nil {
			return err
		}
		hasPhoto = hasPhoto || photo
	}
	if complete && !hasPhoto {
		return Invalid("minimal satu foto penggunaan/hasil pekerjaan atau kondisi barang wajib diunggah")
	}
	return nil
}

func validMaterialQty(q float64) bool {
	return !math.IsNaN(q) && !math.IsInf(q, 0) && q > 0 && q < 1e11 && math.Abs(q*1e4-math.Round(q*1e4)) < 0.001
}

func validateMaterialEvidence(url string) (bool, error) {
	p, err := resolveUploadFile(url)
	if err != nil {
		return false, Invalid("bukti tidak valid: %s", err)
	}
	f, err := os.Open(p)
	if err != nil {
		return false, Invalid("bukti tidak bisa dibaca")
	}
	defer f.Close()
	b := make([]byte, 512)
	n, _ := f.Read(b)
	typ := http.DetectContentType(b[:n])
	photo := typ == "image/jpeg" || typ == "image/png" || typ == "image/webp"
	if !photo && typ != "application/pdf" {
		return false, Invalid("bukti harus foto JPG, PNG, WebP, atau PDF")
	}
	return photo, nil
}

func usageItemsTx(tx *sql.Tx, projectID string, items []models.ProjectUsageItem) error {
	for i := range items {
		if err := tx.QueryRow(`SELECT m.name,m.unit,COALESCE(pr.request_number,'') FROM project_materials m LEFT JOIN purchase_requests pr ON pr.id=m.purchase_request_id WHERE m.id=$1 AND m.project_id=$2`, items[i].MaterialID, projectID).
			Scan(&items[i].Name, &items[i].Unit, &items[i].RequestNumber); err != nil {
			if err == sql.ErrNoRows {
				return Invalid("barang tidak ditemukan dalam projek ini")
			}
			return err
		}
	}
	return nil
}

func usageEventTx(tx *sql.Tx, r *models.ProjectUsageReport, action, note string, a Actor) error {
	data, err := json.Marshal(r.ProjectUsageInput)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO project_usage_events(id,report_id,action,actor,actor_id,note,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)`, NewULID(), r.ID, action, a.Name, usageActorID(a), note, string(data))
	return err
}

func SaveProjectUsage(id string, in models.ProjectUsageInput, a Actor) (*models.ProjectUsageReport, error) {
	if err := validateUsageInput(&in, false); err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockMaterialProject(tx, in.ProjectID); err != nil {
		return nil, err
	}
	var r *models.ProjectUsageReport
	if id != "" {
		r, err = scanProjectUsage(tx.QueryRow(`SELECT `+usageCols+` FROM project_usage_reports r JOIN projects p ON p.id=r.project_id WHERE r.id=$1 FOR UPDATE OF r`, id))
		if err != nil {
			return nil, err
		}
		if r.ProjectID != in.ProjectID {
			return nil, Invalid("projek laporan tidak boleh diganti")
		}
		if r.CreatedByID != usageActorID(a) {
			return nil, ErrUsageForbidden
		}
		if r.Version != in.Version {
			return nil, ErrUsageConflict
		}
		if r.Status != "draft" && r.Status != "revision" {
			return nil, Invalid("laporan hanya dapat diubah saat draft atau perlu perbaikan")
		}
		if r.Kind != in.Kind {
			return nil, Invalid("jenis laporan tidak boleh diganti")
		}
	}
	if err = usageItemsTx(tx, in.ProjectID, in.Items); err != nil {
		return nil, err
	}
	items, _ := json.Marshal(in.Items)
	evidence, _ := json.Marshal(in.Evidence)
	if id == "" {
		id = NewULID()
		prefix := "PBG" + time.Now().In(GetTimezoneLocation()).Format("060102")
		seq, e := nextDocNumber(tx, "project_usage_reports", "report_number", prefix)
		if e != nil {
			return nil, e
		}
		r = &models.ProjectUsageReport{ID: id, ReportNumber: fmt.Sprintf("%s%03d", prefix, seq)}
		_, err = tx.Exec(`INSERT INTO project_usage_reports(id,report_number,project_id,kind,usage_date,location,work_description,performer,pic_name,items,evidence,created_by,created_by_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, id, r.ReportNumber, in.ProjectID, in.Kind, in.UsageDate, in.Location, in.WorkDescription, in.Performer, in.PICName, string(items), string(evidence), a.Name, usageActorID(a))
		in.Version = 1
	} else {
		_, err = tx.Exec(`UPDATE project_usage_reports SET usage_date=$2,location=$3,work_description=$4,performer=$5,pic_name=$6,items=$7,evidence=$8,version=version+1,updated_at=NOW() WHERE id=$1`, id, in.UsageDate, in.Location, in.WorkDescription, in.Performer, in.PICName, string(items), string(evidence))
		in.Version++
	}
	if err != nil {
		return nil, err
	}
	r.ProjectUsageInput = in
	if err = usageEventTx(tx, r, "save", "", a); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return GetProjectUsage(id)
}

// Post only the difference from the last submission. Revision leaves the
// actual consumption recorded until a corrected report is submitted.
func postProjectUsageTx(tx *sql.Tx, r *models.ProjectUsageReport, next []models.ProjectUsageItem, a Actor) error {
	delta := map[string]float64{}
	for _, it := range r.PostedItems {
		delta[it.MaterialID] -= it.Qty
	}
	for _, it := range next {
		delta[it.MaterialID] += it.Qty
	}
	ids := []string{}
	for id := range delta {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	col := "qty_used"
	if r.Kind == "susut" {
		col = "qty_wasted"
	}
	for _, id := range ids {
		qty := math.Round(delta[id]*1e4) / 1e4
		if qty == 0 {
			continue
		}
		var name string
		var remaining, posted float64
		if err := tx.QueryRow(`SELECT name,qty_received-qty_used-qty_returned-qty_wasted,`+col+` FROM project_materials WHERE id=$1 AND project_id=$2 FOR UPDATE`, id, r.ProjectID).Scan(&name, &remaining, &posted); err != nil {
			return err
		}
		if qty > remaining+0.000001 || posted+qty < -0.000001 {
			return Invalid("%s: jumlah melebihi saldo material yang tersedia", name)
		}
		if _, err := tx.Exec(`UPDATE project_materials SET `+col+`=`+col+`+$1,updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, qty, id); err != nil {
			return err
		}
		typ := r.Kind
		if qty < 0 {
			typ = "koreksi"
		}
		if err := writeMaterialLog(tx, id, typ, qty, "project_usage", r.ID, r.ReportNumber, r.WorkDescription, a.Name); err != nil {
			return err
		}
	}
	return nil
}

func ActProjectUsage(id string, in models.ProjectUsageAction, a Actor, canVerify bool) (*models.ProjectUsageReport, error) {
	var projectID string
	if err := database.DB.QueryRow(`SELECT project_id FROM project_usage_reports WHERE id=$1`, id).Scan(&projectID); err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockMaterialProject(tx, projectID); err != nil {
		return nil, err
	}
	r, err := scanProjectUsage(tx.QueryRow(`SELECT `+usageCols+` FROM project_usage_reports r JOIN projects p ON p.id=r.project_id WHERE r.id=$1 FOR UPDATE OF r`, id))
	if err != nil {
		return nil, err
	}
	if r.Version != in.Version {
		return nil, ErrUsageConflict
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > 2000 {
		return nil, Invalid("catatan maksimal 2000 karakter")
	}
	switch in.Action {
	case "submit":
		if r.CreatedByID != usageActorID(a) {
			return nil, ErrUsageForbidden
		}
		if r.Status != "draft" && r.Status != "revision" {
			return nil, Invalid("laporan sudah diajukan atau ditutup")
		}
		if err = validateUsageInput(&r.ProjectUsageInput, true); err != nil {
			return nil, err
		}
		if err = postProjectUsageTx(tx, r, r.Items, a); err != nil {
			return nil, err
		}
		posted, _ := json.Marshal(r.Items)
		_, err = tx.Exec(`UPDATE project_usage_reports SET status='submitted',posted_items=$2,submitted_by_id=$3,submitted_at=NOW(),reviewed_by='',reviewed_at=NULL,review_note='',version=version+1,updated_at=NOW() WHERE id=$1`, id, string(posted), usageActorID(a))
	case "verify", "revise":
		if !canVerify || r.CreatedByID == usageActorID(a) || r.SubmittedByID == usageActorID(a) {
			return nil, ErrUsageForbidden
		}
		if r.Status != "submitted" {
			return nil, Invalid("hanya laporan yang diajukan dapat diperiksa")
		}
		status := "verified"
		if in.Action == "revise" {
			status = "revision"
			if note == "" {
				return nil, Invalid("alasan perbaikan wajib diisi")
			}
		}
		if in.Action == "verify" {
			if err = validateUsageInput(&r.ProjectUsageInput, true); err != nil {
				return nil, err
			}
		}
		_, err = tx.Exec(`UPDATE project_usage_reports SET status=$2,reviewed_by=$3,reviewed_at=NOW(),review_note=$4,version=version+1,updated_at=NOW() WHERE id=$1`, id, status, a.Name, note)
	case "cancel":
		if r.Status == "verified" || r.Status == "cancelled" {
			return nil, Invalid("laporan terverifikasi atau batal tidak dapat dibatalkan")
		}
		if r.CreatedByID != usageActorID(a) {
			return nil, ErrUsageForbidden
		}
		if note == "" {
			return nil, Invalid("alasan pembatalan wajib diisi")
		}
		// Submitted reports first go back through the independent reviewer.
		if r.Status == "submitted" {
			return nil, Invalid("minta pemeriksa mengembalikan laporan untuk perbaikan sebelum pembatalan")
		}
		if err = postProjectUsageTx(tx, r, nil, a); err != nil {
			return nil, err
		}
		_, err = tx.Exec(`UPDATE project_usage_reports SET status='cancelled',posted_items='[]',review_note=$2,version=version+1,updated_at=NOW() WHERE id=$1`, id, note)
	default:
		return nil, Invalid("aksi laporan tidak dikenal")
	}
	if err != nil {
		return nil, err
	}
	r.Version++
	if err = usageEventTx(tx, r, in.Action, note, a); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return GetProjectUsage(id)
}
