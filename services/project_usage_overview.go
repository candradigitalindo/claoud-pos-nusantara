package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
)

type UsageProject struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ProjectNumber string `json:"project_number"`
	Status        string `json:"status"`
}

func ListUsageProjects(outlets, units []string) ([]UsageProject, error) {
	where, args := prScopeCond("p", outlets, units, 1)
	rows, err := database.DB.Query(`SELECT p.id,p.name,COALESCE(p.project_number,''),p.status FROM projects p WHERE true`+where+` ORDER BY p.created_at DESC,p.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []UsageProject{}
	for rows.Next() {
		var p UsageProject
		if err = rows.Scan(&p.ID, &p.Name, &p.ProjectNumber, &p.Status); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

type ProjectUsageAsset struct {
	ID        string   `json:"id"`
	Number    string   `json:"number"`
	Name      string   `json:"name"`
	Qty       int      `json:"qty"`
	Unit      string   `json:"unit"`
	Location  string   `json:"location"`
	PIC       string   `json:"pic"`
	Condition string   `json:"condition"`
	Evidence  []string `json:"evidence"`
}

type ProjectUsageOverview struct {
	Project        UsageProject                   `json:"project"`
	Materials      []models.ProjectMaterial       `json:"materials"`
	Summary        *models.ProjectMaterialSummary `json:"summary"`
	Assets         []ProjectUsageAsset            `json:"assets"`
	PendingReports int                            `json:"pending_reports"`
}

func GetProjectUsageOverview(id string) (*ProjectUsageOverview, error) {
	d := &ProjectUsageOverview{Assets: []ProjectUsageAsset{}}
	if err := database.DB.QueryRow(`SELECT id,name,COALESCE(project_number,''),status FROM projects WHERE id=$1`, id).Scan(&d.Project.ID, &d.Project.Name, &d.Project.ProjectNumber, &d.Project.Status); err != nil {
		return nil, err
	}
	var err error
	if d.Materials, err = ListProjectMaterials(id); err != nil {
		return nil, err
	}
	if d.Summary, err = GetProjectMaterialSummary(id); err != nil {
		return nil, err
	}
	if err = database.DB.QueryRow(`SELECT COUNT(*) FROM project_usage_reports WHERE project_id=$1 AND status NOT IN ('verified','cancelled')`, id).Scan(&d.PendingReports); err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT a.id,COALESCE(a.asset_no,''),a.name,a.quantity,COALESCE(a.unit,''),COALESCE(a.location,''),COALESCE(a.pic_name,''),a.condition,
		COALESCE((SELECT jsonb_agg(DISTINCT hp.photo_url) FROM asset_handover_items hi JOIN handover_photos hp ON hp.ref_type='asset_handover' AND hp.ref_id=hi.handover_id WHERE hi.asset_id=a.id),'[]')
		FROM assets a WHERE a.project_id=$1 AND a.is_deleted=false ORDER BY a.name,a.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a ProjectUsageAsset
		var raw []byte
		if err = rows.Scan(&a.ID, &a.Number, &a.Name, &a.Qty, &a.Unit, &a.Location, &a.PIC, &a.Condition, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &a.Evidence); err != nil {
			return nil, err
		}
		d.Assets = append(d.Assets, a)
	}
	return d, rows.Err()
}

// Called while holding the project row lock, shared with receipts, usage and
// settlement. Changing a project status cannot skip material accountability.
func checkProjectMaterialClosureTx(tx *sql.Tx, id string) error {
	checks := []struct{ query, message string }{
		{`SELECT COUNT(*) FROM project_materials WHERE project_id=$1 AND qty_received<>qty_used+qty_returned+qty_wasted`, "masih ada %d jenis material bersisa; tentukan pengembalian, pemindahan, aset, atau susutnya"},
		{`SELECT COUNT(*) FROM project_usage_reports WHERE project_id=$1 AND status NOT IN ('verified','cancelled')`, "masih ada %d laporan penggunaan/susut yang belum diverifikasi"},
		{`SELECT COUNT(*) FROM purchase_requests WHERE project_id=$1 AND request_type='barang' AND status NOT IN ('cancelled','rejected') AND COALESCE(receipt_status,'')<>'received' AND NOT ` + fullySplitMasterCond(""), "masih ada %d pengadaan barang yang belum diterima lengkap"},
		{`SELECT COUNT(*) FROM assets WHERE project_id=$1 AND is_deleted=false AND COALESCE(status,'aktif')<>'dihapus' AND COALESCE(pic_name,'')=''`, "masih ada %d aset tanpa PIC; selesaikan distribusi aset"},
	}
	for _, c := range checks {
		var n int
		if err := tx.QueryRow(c.query, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return Invalid("projek belum dapat ditutup: %s", fmt.Sprintf(c.message, n))
		}
	}
	return nil
}
