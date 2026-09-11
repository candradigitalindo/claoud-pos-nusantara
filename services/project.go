package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Projek = payung tipis di atas pengadaan. Semua angka serapan dihitung ulang
// dari purchase_requests, tidak pernah disimpan di baris projek — jadi tidak
// ada kemungkinan rekap dan dokumen sumbernya berbeda.

var projectStatuses = map[string]bool{
	"draft": true, "berjalan": true, "selesai": true, "batal": true,
}

// projectPRFilter memilih purchase_requests yang boleh ikut dijumlahkan untuk
// sebuah projek. Master yang seluruh itemnya sudah dipecah ke vendor adalah
// cangkang bernilai 0 yang nilainya sudah pindah ke pecahannya — ikut
// menjumlahkannya berarti menghitung dua kali. Filter ini WAJIB sama dengan
// yang dipakai GetProjectDetail saat mengambil daftar belanjanya.
var projectPRFilter = "project_id = $1 AND NOT " + fullySplitMasterCond("")

// committedCond = pengajuan yang masih hidup (belum ditolak/dibatalkan).
const committedCond = "status NOT IN ('rejected','cancelled')"

func generateProjectNumber(t time.Time) (string, error) {
	prefix := "PRJ-" + t.Format("0601") + "-" // PRJ-YYMM-
	var maxSeq int
	err := database.DB.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTRING(project_number FROM 10) AS INTEGER)), 0) FROM projects WHERE project_number LIKE $1",
		prefix+"%",
	).Scan(&maxSeq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1), nil
}

// summarySelect adalah potongan SELECT rekap yang dipakai baik oleh daftar
// maupun detail, supaya definisi "komitmen / terbayar / sisa hutang" hanya ada
// di satu tempat. Selalu dipakai atas purchase_requests tanpa alias, sama
// seperti outstandingCond.
func summarySelect() string {
	// total_amount = harga final bila sudah diisi, selain itu HPS (lihat
	// amountOf di purchase.go). Komitmen sengaja memakai kolom itu supaya
	// pengajuan yang sudah disetujui tapi belum diberi harga tetap memakan
	// RAB — kalau memakai total_final, sisa RAB terlihat lega padahal
	// belanjanya sudah antre.
	//
	// Hutang (outstanding) tetap berbasis total_final: estimasi HPS bukan
	// kewajiban kepada siapa pun.
	return fmt.Sprintf(`
		COUNT(*) FILTER (WHERE %[1]s) AS request_count,
		COALESCE(SUM(total_amount) FILTER (WHERE %[1]s), 0) AS committed,
		COALESCE(SUM(total_amount) FILTER (WHERE %[1]s AND total_final = 0), 0) AS estimated,
		COALESCE(SUM(paid_amount), 0) AS paid,
		COALESCE(SUM(total_final - paid_amount) FILTER (WHERE %[2]s), 0) AS outstanding`,
		committedCond, outstandingCond)
}

// fillDerived mengisi angka turunan yang tidak datang dari SQL.
func fillDerived(p *models.Project) {
	p.RemainingBudget = p.Budget - p.Committed
	if p.Budget > 0 {
		p.AbsorbedPct = p.Committed / p.Budget * 100
	}
	p.OverBudget = p.Budget > 0 && p.Committed > p.Budget
}

func dateStr(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02")
}

// ListProjects mengembalikan projek beserta rekapnya.
//
// outletIDs/wuIDs berasal dari scope role. Reuse prScopeCond (lihat vendor.go):
// tabel projects punya kolom outlet_id & work_unit_id yang sama bentuknya,
// jadi kondisi scope-nya identik.
func ListProjects(status, search string, outletIDs, wuIDs []string) ([]models.Project, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if status != "" {
		where += fmt.Sprintf(" AND p.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.project_number ILIKE $%d OR p.pic ILIKE $%d)", idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}
	scopeCond, scopeArgs := prScopeCond("p", outletIDs, wuIDs, idx)
	where += scopeCond
	args = append(args, scopeArgs...)

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT p.id, p.project_number, p.name, p.outlet_id, COALESCE(o.name,''),
		       p.work_unit_id, COALESCE(wu.name,''), p.pic, p.budget,
		       p.start_date, p.target_date, p.status, p.notes, p.created_by,
		       p.created_at, p.updated_at,
		       COALESCE(s.request_count, 0), COALESCE(s.committed, 0), COALESCE(s.estimated, 0),
		       COALESCE(s.paid, 0), COALESCE(s.outstanding, 0)
		FROM projects p
		LEFT JOIN outlets o ON o.id = p.outlet_id
		LEFT JOIN work_units wu ON wu.id = p.work_unit_id
		LEFT JOIN (
			SELECT project_id, %s
			FROM purchase_requests
			WHERE project_id IS NOT NULL AND NOT %s
			GROUP BY project_id
		) s ON s.project_id = p.id
		%s
		ORDER BY p.created_at DESC
	`, summarySelect(), fullySplitMasterCond(""), where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	for rows.Next() {
		var p models.Project
		var start, target sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.ProjectNumber, &p.Name, &p.OutletID, &p.OutletName,
			&p.WorkUnitID, &p.WorkUnitName, &p.PIC, &p.Budget,
			&start, &target, &p.Status, &p.Notes, &p.CreatedBy,
			&p.CreatedAt, &p.UpdatedAt,
			&p.RequestCount, &p.Committed, &p.Estimated, &p.Paid, &p.Outstanding,
		); err != nil {
			return nil, err
		}
		p.StartDate, p.TargetDate = dateStr(start), dateStr(target)
		fillDerived(&p)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProject mengambil satu projek beserta rekapnya (tanpa daftar belanja).
func GetProject(id string) (*models.Project, error) {
	var p models.Project
	var start, target sql.NullTime
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT p.id, p.project_number, p.name, p.outlet_id, COALESCE(o.name,''),
		       p.work_unit_id, COALESCE(wu.name,''), p.pic, p.budget,
		       p.start_date, p.target_date, p.status, p.notes, p.created_by,
		       p.created_at, p.updated_at,
		       COALESCE(s.request_count, 0), COALESCE(s.committed, 0), COALESCE(s.estimated, 0),
		       COALESCE(s.paid, 0), COALESCE(s.outstanding, 0)
		FROM projects p
		LEFT JOIN outlets o ON o.id = p.outlet_id
		LEFT JOIN work_units wu ON wu.id = p.work_unit_id
		LEFT JOIN (
			SELECT %s FROM purchase_requests WHERE %s
		) s ON TRUE
		WHERE p.id = $1
	`, summarySelect(), projectPRFilter), id).Scan(
		&p.ID, &p.ProjectNumber, &p.Name, &p.OutletID, &p.OutletName,
		&p.WorkUnitID, &p.WorkUnitName, &p.PIC, &p.Budget,
		&start, &target, &p.Status, &p.Notes, &p.CreatedBy,
		&p.CreatedAt, &p.UpdatedAt,
		&p.RequestCount, &p.Committed, &p.Estimated, &p.Paid, &p.Outstanding,
	)
	if err != nil {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	p.StartDate, p.TargetDate = dateStr(start), dateStr(target)
	fillDerived(&p)
	return &p, nil
}

// GetProjectDetail = header + rekap + daftar belanja tahap.
//
// parentID "all" + excludeMasters: pecahan vendor ikut ditampilkan sebagai
// dokumen sendiri, sedangkan master yang sudah habis dipecah disembunyikan —
// persis himpunan yang dijumlahkan projectPRFilter.
func GetProjectDetail(id string, outletIDs, wuIDs []string) (*models.ProjectDetailResponse, error) {
	p, err := GetProject(id)
	if err != nil {
		return nil, err
	}
	list, err := ListPurchaseRequests("", "", "", "", id, "all", true, "", outletIDs, wuIDs, 1, 500)
	if err != nil {
		return nil, err
	}
	return &models.ProjectDetailResponse{Project: *p, Requests: list.Requests}, nil
}

// resolveProjectOutlet mengisi outlet_id dari unit kerja bila pengguna hanya
// memilih unit kerja. Tanpa ini, projek milik sebuah outlet tidak akan terlihat
// oleh role yang scope-nya dibatasi per OUTLET (lihat prScopeCond).
func resolveProjectOutlet(outletID, workUnitID string) string {
	if outletID != "" || workUnitID == "" {
		return outletID
	}
	var derived sql.NullString
	database.DB.QueryRow("SELECT outlet_id FROM work_units WHERE id = $1", workUnitID).Scan(&derived)
	if derived.Valid {
		return derived.String
	}
	return ""
}

func validateProjectInput(name string, budget float64, status string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("nama projek wajib diisi")
	}
	if budget < 0 {
		return "", fmt.Errorf("RAB tidak boleh negatif")
	}
	if status == "" {
		status = "berjalan"
	}
	if !projectStatuses[status] {
		return "", fmt.Errorf("status projek '%s' tidak valid", status)
	}
	return status, nil
}

func CreateProject(req models.CreateProjectRequest, createdBy string) (*models.Project, error) {
	status, err := validateProjectInput(req.Name, req.Budget, req.Status)
	if err != nil {
		return nil, err
	}

	id := NewULID()
	now := time.Now().UTC()

	// Nomor dihitung dari MAX() tanpa lock, jadi dua projek bersamaan bisa
	// mendapat nomor yang sama — unique index menolaknya dan kita ulang.
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		number, nerr := generateProjectNumber(now)
		if nerr != nil {
			return nil, fmt.Errorf("gagal generate nomor projek: %w", nerr)
		}
		_, lastErr = database.DB.Exec(`
			INSERT INTO projects (id, project_number, name, outlet_id, work_unit_id, pic, budget,
			                      start_date, target_date, status, notes, created_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)
		`, id, number, strings.TrimSpace(req.Name), nilIfEmpty(resolveProjectOutlet(req.OutletID, req.WorkUnitID)), nilIfEmpty(req.WorkUnitID),
			req.PIC, req.Budget, nilIfEmpty(req.StartDate), nilIfEmpty(req.TargetDate),
			status, req.Notes, createdBy, now)
		if lastErr == nil {
			return GetProject(id)
		}
		if !strings.Contains(lastErr.Error(), "uq_projects_number") {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func UpdateProject(id string, req models.UpdateProjectRequest) (*models.Project, error) {
	status, err := validateProjectInput(req.Name, req.Budget, req.Status)
	if err != nil {
		return nil, err
	}
	res, err := database.DB.Exec(`
		UPDATE projects SET name=$1, outlet_id=$2, work_unit_id=$3, pic=$4, budget=$5,
		       start_date=$6, target_date=$7, status=$8, notes=$9, updated_at=$10
		WHERE id=$11
	`, strings.TrimSpace(req.Name), nilIfEmpty(resolveProjectOutlet(req.OutletID, req.WorkUnitID)), nilIfEmpty(req.WorkUnitID),
		req.PIC, req.Budget, nilIfEmpty(req.StartDate), nilIfEmpty(req.TargetDate),
		status, req.Notes, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("projek tidak ditemukan")
	}
	return GetProject(id)
}

// DeleteProject menolak menghapus projek yang masih memayungi pengajuan.
// FK-nya ON DELETE SET NULL, jadi menghapus diam-diam akan melepas pengajuan
// dari projeknya tanpa jejak — lebih baik pengguna melepasnya sendiri dulu.
func DeleteProject(id string) error {
	var used int
	if err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM purchase_requests WHERE project_id = $1", id,
	).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("projek masih memiliki %d pengajuan pengadaan — lepaskan atau hapus pengajuannya dulu", used)
	}
	res, err := database.DB.Exec("DELETE FROM projects WHERE id = $1", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("projek tidak ditemukan")
	}
	return nil
}
