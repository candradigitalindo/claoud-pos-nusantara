package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RAB projek per baris.
//
// Sebelumnya projek hanya memegang satu angka total, sehingga "sisa RAB"
// tidak pernah bisa menjawab pertanyaan yang sebenarnya: pos mana yang sudah
// habis dan pos mana yang belum tersentuh. Sekarang RAB disusun per baris
// (bagian pekerjaan → uraian → volume × harga satuan), ditetapkan sekali, dan
// tiap item pengajuan menunjuk baris yang ia serap.
//
// Angka serapan per baris dihitung ulang dari purchase_requests — tidak ada
// kolom rekap di project_rab_items, jadi tidak mungkin basi.

var rabKinds = map[string]bool{"barang": true, "jasa": true, "umum": true}

// dbExecer dipenuhi *sql.DB maupun *sql.Tx, supaya helper di bawah bisa dipakai
// di dalam maupun di luar transaksi.
type dbExecer interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

const (
	RabDraft = "draft"
	RabSet   = "ditetapkan"
)

const rabItemCols = `id, project_id, seq, COALESCE(section,''), name, kind, COALESCE(unit,''),
	qty, unit_price, subtotal, COALESCE(notes,'')`

func scanRabItem(sc interface{ Scan(...interface{}) error }) (models.ProjectRabItem, error) {
	var it models.ProjectRabItem
	err := sc.Scan(&it.ID, &it.ProjectID, &it.Seq, &it.Section, &it.Name, &it.Kind, &it.Unit,
		&it.Qty, &it.UnitPrice, &it.Subtotal, &it.Notes)
	return it, err
}

// listRabItems membaca baris RAB apa adanya (tanpa serapan).
func listRabItems(q dbExecer, projectID string) ([]models.ProjectRabItem, error) {
	rows, err := q.Query(fmt.Sprintf(`SELECT %s FROM project_rab_items WHERE project_id = $1 ORDER BY seq, created_at`, rabItemCols), projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProjectRabItem, 0)
	for rows.Next() {
		it, err := scanRabItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// rabAbsorption adalah serapan satu baris RAB (atau "di luar RAB").
type rabAbsorption struct {
	committed, estimated, paid float64
	requests                   map[string]bool
}

// subItemAmount = nilai item yang membebani RAB: harga final bila sudah
// disepakati, selain itu HPS — sama seperti amountOf di tingkat dokumen.
func subItemAmount(s models.PurchaseSubItem) (amount float64, isEstimate bool) {
	if s.FinalPrice > 0 {
		return float64(s.Qty) * s.FinalPrice, false
	}
	return float64(s.Qty) * s.HpsPrice, true
}

// absorbByRabItem mengelompokkan serapan per rab_item_id dari daftar belanja
// projek. Dokumen yang ditolak/dibatalkan tidak membebani RAB. Pembayaran
// dicatat per dokumen, jadi dialokasikan ke item secara proporsional terhadap
// nilai item di dokumen itu.
//
// Kunci "" menampung item tanpa rujukan baris RAB; pemanggil memetakan rujukan
// ke baris yang sudah tidak ada ke kunci yang sama.
func absorbByRabItem(requests []models.PurchaseRequest, known map[string]bool) map[string]*rabAbsorption {
	out := map[string]*rabAbsorption{}
	get := func(k string) *rabAbsorption {
		if out[k] == nil {
			out[k] = &rabAbsorption{requests: map[string]bool{}}
		}
		return out[k]
	}
	for _, pr := range requests {
		if pr.Status == "rejected" || pr.Status == "cancelled" {
			continue
		}
		var docTotal float64
		for _, g := range pr.Items {
			for _, s := range g.Items {
				a, _ := subItemAmount(s)
				docTotal += a
			}
		}
		for _, g := range pr.Items {
			for _, s := range g.Items {
				key := s.RabItemID
				if key != "" && !known[key] {
					key = ""
				}
				a, est := subItemAmount(s)
				ab := get(key)
				ab.committed += a
				if est {
					ab.estimated += a
				}
				if docTotal > 0 && pr.PaidAmount > 0 {
					ab.paid += pr.PaidAmount * a / docTotal
				}
				ab.requests[pr.ID] = true
			}
		}
	}
	return out
}

// fillRabAbsorption mengisi serapan tiap baris dan mengembalikan bagian "di
// luar RAB" (committed, paid).
func fillRabAbsorption(items []models.ProjectRabItem, requests []models.PurchaseRequest) (offCommitted, offPaid float64) {
	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.ID] = true
	}
	abs := absorbByRabItem(requests, known)
	for i := range items {
		it := &items[i]
		if ab := abs[it.ID]; ab != nil {
			it.Committed = round2(ab.committed)
			it.Estimated = round2(ab.estimated)
			it.Paid = round2(ab.paid)
			it.RequestCount = len(ab.requests)
		}
		it.Remaining = round2(it.Subtotal - it.Committed)
		if it.Subtotal > 0 {
			it.AbsorbedPct = it.Committed / it.Subtotal * 100
		}
		it.OverBudget = it.Committed > it.Subtotal
	}
	if ab := abs[""]; ab != nil {
		return round2(ab.committed), round2(ab.paid)
	}
	return 0, 0
}

// GetProjectRab = baris RAB + serapannya, untuk halaman projek maupun form
// pengajuan (yang perlu tahu sisa tiap baris sebelum memilihnya).
func GetProjectRab(projectID string, outletIDs, wuIDs []string) (*models.ProjectRabResponse, error) {
	var res models.ProjectRabResponse
	err := database.DB.QueryRow(`SELECT id, project_number, status, rab_status, rab_version, budget FROM projects WHERE id = $1`, projectID).
		Scan(&res.ProjectID, &res.ProjectNumber, &res.ProjectStatus, &res.RabStatus, &res.RabVersion, &res.Total)
	if err != nil {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	items, err := listRabItems(database.DB, projectID)
	if err != nil {
		return nil, err
	}
	list, err := ListPurchaseRequests("", "", "", "", projectID, "all", true, "", outletIDs, wuIDs, 1, 500)
	if err != nil {
		return nil, err
	}
	fillRabAbsorption(items, list.Requests)
	res.Items = items
	return &res, nil
}

func validateRabInput(items []models.ProjectRabItemInput) error {
	if len(items) == 0 {
		return Invalid("RAB minimal berisi satu baris")
	}
	for i, it := range items {
		if strings.TrimSpace(it.Name) == "" {
			return Invalid("uraian pada baris RAB ke-%d wajib diisi", i+1)
		}
		if it.Kind == "" {
			items[i].Kind = "barang"
		} else if !rabKinds[it.Kind] {
			return Invalid("jenis baris RAB %q tidak dikenal (barang/jasa/umum)", it.Name)
		}
		if it.Qty <= 0 {
			return Invalid("volume baris RAB %q harus lebih dari 0", it.Name)
		}
		if it.UnitPrice < 0 {
			return Invalid("harga satuan baris RAB %q tidak boleh negatif", it.Name)
		}
	}
	return nil
}

// rabItemInUse: baris masih ditunjuk item pengajuan yang hidup. Baris seperti
// itu tidak boleh dihapus — serapannya akan jatuh ke "di luar RAB" dan jejak
// pos anggarannya hilang.
func rabItemInUse(q dbExecer, projectID, itemID string) bool {
	var used bool
	err := q.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM purchase_requests pr,
			     jsonb_array_elements(pr.items) g,
			     jsonb_array_elements(g->'items') s
			WHERE pr.project_id = $1 AND pr.status NOT IN ('rejected','cancelled')
			  AND s->>'rab_item_id' = $2)`, projectID, itemID).Scan(&used)
	return err == nil && used
}

// syncProjectBudget menyalin Σ subtotal ke projects.budget supaya rekap daftar
// dan neraca (yang membaca kolom budget) tetap benar tanpa join tambahan.
func syncProjectBudget(q dbExecer, projectID string) error {
	_, err := q.Exec(`
		UPDATE projects SET budget = COALESCE((SELECT SUM(subtotal) FROM project_rab_items WHERE project_id = $1), 0),
		       updated_at = $2 WHERE id = $1`, projectID, time.Now().UTC())
	return err
}

// SaveProjectRab mengganti susunan RAB. Hanya boleh saat RAB berstatus draft
// (baru atau sedang direvisi); RAB yang sudah ditetapkan harus dibuka dulu.
func SaveProjectRab(projectID string, req models.SaveProjectRabRequest) ([]models.ProjectRabItem, error) {
	var status, rabStatus string
	if err := database.DB.QueryRow(`SELECT status, rab_status FROM projects WHERE id = $1`, projectID).Scan(&status, &rabStatus); err != nil {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	if status == "selesai" || status == "batal" {
		return nil, Invalid("projek sudah %s; RAB tidak bisa diubah lagi", status)
	}
	if rabStatus == RabSet {
		return nil, Invalid("RAB sudah ditetapkan — buka revisi dulu sebelum mengubah barisnya")
	}
	if err := validateRabInput(req.Items); err != nil {
		return nil, err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existing, err := listRabItems(tx, projectID)
	if err != nil {
		return nil, err
	}
	existingIDs := map[string]bool{}
	for _, it := range existing {
		existingIDs[it.ID] = true
	}
	keep := map[string]bool{}
	now := time.Now().UTC()
	for seq, in := range req.Items {
		subtotal := round2(in.Qty * in.UnitPrice)
		name := strings.TrimSpace(in.Name)
		section := strings.TrimSpace(in.Section)
		if in.ID != "" && existingIDs[in.ID] {
			keep[in.ID] = true
			if _, err := tx.Exec(`
				UPDATE project_rab_items SET seq=$1, section=$2, name=$3, kind=$4, unit=$5, qty=$6,
				       unit_price=$7, subtotal=$8, notes=$9, updated_at=$10
				WHERE id=$11 AND project_id=$12`,
				seq, section, name, in.Kind, in.Unit, in.Qty, in.UnitPrice, subtotal, in.Notes, now, in.ID, projectID); err != nil {
				return nil, err
			}
			continue
		}
		id := NewULID()
		keep[id] = true
		if _, err := tx.Exec(`
			INSERT INTO project_rab_items (id, project_id, seq, section, name, kind, unit, qty, unit_price, subtotal, notes, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`,
			id, projectID, seq, section, name, in.Kind, in.Unit, in.Qty, in.UnitPrice, subtotal, in.Notes, now); err != nil {
			return nil, err
		}
	}
	for _, it := range existing {
		if keep[it.ID] {
			continue
		}
		if rabItemInUse(tx, projectID, it.ID) {
			return nil, Invalid("baris RAB %q sudah dipakai pengajuan pengadaan, tidak bisa dihapus — ubah volume/harganya saja", it.Name)
		}
		if _, err := tx.Exec(`DELETE FROM project_rab_items WHERE id = $1`, it.ID); err != nil {
			return nil, err
		}
	}
	if err := syncProjectBudget(tx, projectID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return listRabItems(database.DB, projectID)
}

// SetProjectRab menetapkan RAB: sejak ini baris dikunci, versi naik, dan
// pengajuan untuk projek ini boleh dibuat. Projek yang masih draft ikut
// menjadi berjalan.
func SetProjectRab(projectID, actor string) (*models.Project, error) {
	var status, rabStatus string
	var total sql.NullFloat64
	var count int
	err := database.DB.QueryRow(`
		SELECT p.status, p.rab_status,
		       (SELECT SUM(subtotal) FROM project_rab_items WHERE project_id = p.id),
		       (SELECT COUNT(*) FROM project_rab_items WHERE project_id = p.id)
		FROM projects p WHERE p.id = $1`, projectID).Scan(&status, &rabStatus, &total, &count)
	if err != nil {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	if status == "selesai" || status == "batal" {
		return nil, Invalid("projek sudah %s", status)
	}
	if rabStatus == RabSet {
		return nil, Invalid("RAB sudah ditetapkan")
	}
	if count == 0 || !total.Valid || total.Float64 <= 0 {
		return nil, Invalid("RAB belum berisi baris bernilai — susun dulu sebelum ditetapkan")
	}
	now := time.Now().UTC()
	if _, err := database.DB.Exec(`
		UPDATE projects SET rab_status = $1, rab_version = rab_version + 1, rab_set_at = $2, rab_set_by = $3,
		       budget = $4, status = CASE WHEN status = 'draft' THEN 'berjalan' ELSE status END, updated_at = $2
		WHERE id = $5`, RabSet, now, actor, round2(total.Float64), projectID); err != nil {
		return nil, err
	}
	return GetProject(projectID)
}

// ReopenProjectRab membuka RAB yang sudah ditetapkan untuk direvisi. Selama
// terbuka, pengajuan baru untuk projek ini ditolak (lihat validateProjectRab).
func ReopenProjectRab(projectID string) (*models.Project, error) {
	var status, rabStatus string
	if err := database.DB.QueryRow(`SELECT status, rab_status FROM projects WHERE id = $1`, projectID).Scan(&status, &rabStatus); err != nil {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	if status == "selesai" || status == "batal" {
		return nil, Invalid("projek sudah %s", status)
	}
	if rabStatus != RabSet {
		return nil, Invalid("RAB belum ditetapkan, tidak ada yang perlu dibuka")
	}
	if _, err := database.DB.Exec(`UPDATE projects SET rab_status = $1, updated_at = $2 WHERE id = $3`,
		RabDraft, time.Now().UTC(), projectID); err != nil {
		return nil, err
	}
	return GetProject(projectID)
}

// validateProjectRab dipanggil saat item pengajuan dibuat/diubah.
//
// requireSet: pengajuan BARU hanya boleh lahir bila RAB projek sudah
// ditetapkan. Perubahan item pada pengajuan lama tidak dikenai syarat itu
// (projek lama bisa saja belum punya RAB), tetapi rujukan barisnya tetap
// diperiksa: baris harus milik projek yang sama dan jenisnya cocok dengan
// jenis pengajuan.
func validateProjectRab(projectID, requestType string, items []models.PurchaseRequestItem, requireSet bool) error {
	if projectID == "" {
		for _, g := range items {
			for _, s := range g.Items {
				if s.RabItemID != "" {
					return Invalid("item %q menunjuk baris RAB, tetapi pengajuan tidak memilih projek", s.Name)
				}
			}
		}
		return nil
	}
	var status, rabStatus, number string
	if err := database.DB.QueryRow(`SELECT status, rab_status, project_number FROM projects WHERE id = $1`, projectID).
		Scan(&status, &rabStatus, &number); err != nil {
		return Invalid("projek tidak ditemukan")
	}
	if requireSet {
		if status == "selesai" || status == "batal" {
			return Invalid("projek %s sudah %s, tidak menerima pengajuan baru", number, status)
		}
		if rabStatus != RabSet {
			return Invalid("RAB projek %s belum ditetapkan — susun dan tetapkan RAB di halaman Projek dulu, baru ajukan belanjanya", number)
		}
	}
	lines, err := listRabItems(database.DB, projectID)
	if err != nil {
		return err
	}
	byID := make(map[string]models.ProjectRabItem, len(lines))
	for _, l := range lines {
		byID[l.ID] = l
	}
	for _, g := range items {
		for _, s := range g.Items {
			if s.RabItemID == "" {
				continue
			}
			line, ok := byID[s.RabItemID]
			if !ok {
				return Invalid("baris RAB untuk item %q tidak ditemukan pada projek %s", s.Name, number)
			}
			if line.Kind != "umum" && line.Kind != requestType {
				return Invalid("baris RAB %q berjenis %s, tidak bisa diserap pengajuan %s", line.Name, line.Kind, requestType)
			}
		}
	}
	return nil
}
