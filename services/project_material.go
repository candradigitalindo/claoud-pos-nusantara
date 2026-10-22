package services

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Identitas yang selalu dijaga:
//
//	qty_received = qty_used + qty_returned + qty_wasted + sisa
//	qty_received = Σ log 'penerimaan'
//
// Sisa tidak disimpan sebagai kolom agar tidak pernah basi; log bersifat
// append-only, koreksi dilakukan dengan menambah baris bertipe 'koreksi'.

const projectMaterialCols = `
	m.id, m.project_id, COALESCE(p.name, ''), COALESCE(m.purchase_request_id, ''),
	COALESCE(pr.request_number, ''), m.name, COALESCE(m.unit, ''),
	m.qty_received, m.qty_used, m.qty_returned, m.qty_wasted, COALESCE(m.unit_cost, 0),
	COALESCE(m.location, ''), COALESCE(TO_CHAR(m.received_at, 'YYYY-MM-DD'), ''),
	COALESCE(m.received_by, ''), COALESCE(m.notes, '')`

const projectMaterialJoins = `
	LEFT JOIN projects p ON p.id = m.project_id
	LEFT JOIN purchase_requests pr ON pr.id = m.purchase_request_id`

func scanProjectMaterial(sc interface{ Scan(...interface{}) error }) (models.ProjectMaterial, error) {
	var m models.ProjectMaterial
	err := sc.Scan(&m.ID, &m.ProjectID, &m.ProjectName, &m.PurchaseRequestID, &m.RequestNumber,
		&m.Name, &m.Unit, &m.QtyReceived, &m.QtyUsed, &m.QtyReturned, &m.QtyWasted,
		&m.UnitCost, &m.Location, &m.ReceivedAt, &m.ReceivedBy, &m.Notes)
	m.Remaining = m.QtyReceived - m.QtyUsed - m.QtyReturned - m.QtyWasted
	m.ValueReceived = m.QtyReceived * m.UnitCost
	m.ValueRemaining = m.Remaining * m.UnitCost
	return m, err
}

func ListProjectMaterials(projectID string) ([]models.ProjectMaterial, error) {
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT %s FROM project_materials m %s
		WHERE m.project_id = $1 ORDER BY m.name`, projectMaterialCols, projectMaterialJoins), projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProjectMaterial, 0)
	for rows.Next() {
		m, err := scanProjectMaterial(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func GetProjectMaterial(id string) (*models.ProjectMaterial, error) {
	m, err := scanProjectMaterial(database.DB.QueryRow(fmt.Sprintf(
		`SELECT %s FROM project_materials m %s WHERE m.id = $1`,
		projectMaterialCols, projectMaterialJoins), id))
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func GetProjectMaterialSummary(projectID string) (*models.ProjectMaterialSummary, error) {
	var s models.ProjectMaterialSummary
	err := database.DB.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(qty_received * unit_cost), 0),
		       COALESCE(SUM(qty_used * unit_cost), 0),
		       COALESCE(SUM(qty_wasted * unit_cost), 0),
		       COALESCE(SUM((qty_received - qty_used - qty_returned - qty_wasted) * unit_cost), 0),
		       COUNT(*) FILTER (WHERE qty_received - qty_used - qty_returned - qty_wasted > 0)
		FROM project_materials WHERE project_id = $1`, projectID).
		Scan(&s.Lines, &s.ValueReceived, &s.ValueUsed, &s.ValueWasted, &s.ValueRemaining, &s.UnsettledLines)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func ListProjectMaterialLogs(materialID string) ([]models.ProjectMaterialLog, error) {
	rows, err := database.DB.Query(`
		SELECT id, material_id, type, qty, COALESCE(ref_type, ''), COALESCE(ref_number, ''),
		       COALESCE(notes, ''), COALESCE(actor, ''), TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI'), COALESCE(evidence_url,'')
		FROM project_material_logs WHERE material_id = $1
		ORDER BY created_at DESC, id DESC`, materialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProjectMaterialLog, 0)
	for rows.Next() {
		var l models.ProjectMaterialLog
		if err := rows.Scan(&l.ID, &l.MaterialID, &l.Type, &l.Qty, &l.RefType, &l.RefNumber,
			&l.Notes, &l.Actor, &l.CreatedAt, &l.EvidenceURL); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func writeMaterialLog(ex assetExecer, materialID, logType string, qty float64, refType, refID, refNumber, notes, actor string, evidence ...string) error {
	photo := ""
	if len(evidence) > 0 {
		photo = evidence[0]
	}
	_, err := ex.Exec(`
		INSERT INTO project_material_logs (id, material_id, type, qty, ref_type, ref_id, ref_number, notes, actor, evidence_url, created_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,(now() AT TIME ZONE 'UTC'))`,
		NewULID(), materialID, logType, qty, refType, refID, refNumber, notes, actor, photo)
	return err
}

// upsertProjectMaterialTx mencatat penerimaan material. Penerimaan BERTAHAP
// (semen 100 sak datang tiga kali) menambah qty_received pada baris yang sama,
// bukan membuat baris baru — indeks unik (purchase_request_id, pr_item_key)
// yang menjaganya.
func upsertProjectMaterialTx(tx *sql.Tx, projectID, prID, prItemKey, name, unit string,
	qty, unitCost float64, location, actor string) (string, error) {
	if projectID == "" {
		return "", Invalid("%s: pengajuan ini tidak terikat projek, tujuan material tidak tersedia", name)
	}
	var id string
	err := tx.QueryRow(`
		SELECT id FROM project_materials
		WHERE purchase_request_id = $1 AND pr_item_key = $2`, prID, prItemKey).Scan(&id)
	if err == nil && id != "" {
		if _, err := tx.Exec(`
			UPDATE project_materials SET qty_received = qty_received + $1, unit_cost = $2,
				updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $3`, qty, unitCost, id); err != nil {
			return "", err
		}
	} else {
		id = NewULID()
		if _, err := tx.Exec(`
			INSERT INTO project_materials (id, project_id, purchase_request_id, pr_item_key, name, unit,
				qty_received, unit_cost, location, received_at, received_by, created_at, updated_at)
			VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,(now() AT TIME ZONE 'UTC'),$10,
				(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
			id, projectID, prID, prItemKey, name, unit, qty, unitCost, location, actor); err != nil {
			return "", err
		}
	}
	if err := writeMaterialLog(tx, id, "penerimaan", qty, "purchase_request", prID, "", "Diterima di lokasi projek", actor); err != nil {
		return "", err
	}
	return id, nil
}

// RecordMaterialUsage retains a clear error for older clients. Every new
// consumption must have a usage report and independent verification.
func RecordMaterialUsage(id string, req models.ProjectMaterialUsageRequest, actor string) (*models.ProjectMaterial, error) {
	return nil, Invalid("catat pemakaian melalui Perlengkapan → Penggunaan Barang Projek, lengkap dengan bukti dan verifikasi")
}

// SettleMaterialRemainder menentukan nasib sisa material.
//
// ATURAN POKOK: sisa projek harus didata dan menjadi ASET. Barang yang masih
// ada wujudnya tidak boleh lenyap dari catatan hanya karena projeknya selesai.
// Tiga pilihan lain adalah pengecualian, bukan jalan setara:
//
//	aset   → BAKU. Sisa yang masih ada wujudnya dicatat sebagai aset outlet
//	gudang → hanya untuk barang yang memang ada di katalog stok
//	susut  → hanya untuk yang benar-benar rusak/habis, wajib beralasan
//	pindah → dipakai projek lain yang sedang berjalan
func SettleMaterialRemainder(id string, req models.ProjectMaterialSettleRequest, actor string, canStock bool, outletScope []string) (*models.ProjectMaterial, error) {
	if req.Action == "susut" {
		return nil, Invalid("catat susut melalui laporan Penggunaan Barang Projek agar bukti dan verifikasinya lengkap")
	}
	if photo, err := validateMaterialEvidence(req.PhotoURL); err != nil {
		return nil, err
	} else if !photo {
		return nil, Invalid("bukti penyelesaian sisa harus berupa foto")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var projectID string
	if err = tx.QueryRow(`SELECT project_id FROM project_materials WHERE id=$1`, id).Scan(&projectID); err != nil {
		return nil, err
	}
	// Transfers lock both projects in deterministic order.
	ids := []string{projectID}
	if req.Action == "pindah" && req.TargetProjectID != "" && req.TargetProjectID != projectID {
		ids = append(ids, req.TargetProjectID)
	}
	sort.Strings(ids)
	for _, pid := range ids {
		if err = lockMaterialProject(tx, pid); err != nil {
			return nil, err
		}
	}
	m, err := scanProjectMaterial(tx.QueryRow(fmt.Sprintf(`SELECT %s FROM project_materials m %s WHERE m.id=$1 FOR UPDATE OF m`, projectMaterialCols, projectMaterialJoins), id))
	if err != nil {
		return nil, err
	}
	qty := req.Qty
	if !validMaterialQty(qty) {
		return nil, Invalid("jumlah harus positif dan maksimal 4 angka desimal")
	}
	if qty > m.Remaining {
		return nil, Invalid("%s: jumlah melebihi sisa %.4f %s", m.Name, m.Remaining, m.Unit)
	}
	if req.Action == "aset" && math.Trunc(qty) != qty {
		return nil, Invalid("jumlah aset harus bulat; sisa pecahan dapat dikembalikan ke gudang atau dipindah projek")
	}
	switch req.Action {
	case "gudang":
		if !canStock {
			return nil, Invalid("Anda tidak berhak menambah stok gudang — mintakan ke tim gudang")
		}
		if !WarehouseMutableInScope(req.WarehouseID, outletScope) {
			return nil, Invalid("gudang di luar akses Anda")
		}
		if req.StockItemID == "" || req.WarehouseID == "" {
			return nil, Invalid("item stok dan gudang wajib dipilih")
		}
		// Harga material tersimpan per satuan beli; GRN menyimpan per satuan dasar.
		var distRatio float64
		if err := tx.QueryRow(`SELECT COALESCE(NULLIF(dist_ratio, 0), 1) FROM stock_items WHERE id = $1`,
			req.StockItemID).Scan(&distRatio); err != nil {
			return nil, Invalid("item stok tidak ditemukan")
		}
		grnID, err := CreateGoodsReceiptTx(tx, models.GoodsReceiptRequest{
			WarehouseID: req.WarehouseID,
			VendorName:  "Sisa projek " + m.ProjectName,
			Notes:       fmt.Sprintf("Pengembalian sisa material projek: %s", m.Name),
			Items: []models.GoodsReceiptItemReq{{
				ItemID: req.StockItemID, QtyDist: qty, CostPerBase: m.UnitCost / distRatio,
			}},
		}, actor)
		if err != nil {
			return nil, err
		}
		var grnNumber string
		tx.QueryRow(`SELECT grn_number FROM goods_receipts WHERE id = $1`, grnID).Scan(&grnNumber)
		if _, err := tx.Exec(`UPDATE project_materials SET qty_returned = qty_returned + $1,
			updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $2`, qty, id); err != nil {
			return nil, err
		}
		if err := writeMaterialLog(tx, id, "pengembalian", qty, "goods_receipt", grnID, grnNumber,
			"Dikembalikan ke gudang. "+req.Notes, actor, req.PhotoURL); err != nil {
			return nil, err
		}

	case "aset":
		outletID := req.OutletID
		if outletID == "" {
			// Bawaan: outlet milik projeknya sendiri — sisa renovasi sebuah
			// outlet menjadi aset outlet itu.
			tx.QueryRow(`SELECT COALESCE(outlet_id, '') FROM projects WHERE id = $1`, m.ProjectID).Scan(&outletID)
		}
		if outletID == "" {
			return nil, Invalid("outlet tujuan aset wajib dipilih — projek ini tidak terikat outlet")
		}
		if !outletInScope(outletID, outletScope) {
			return nil, Invalid("outlet di luar akses Anda")
		}
		category, cerr := resolveAssetCategory(req.Category)
		if cerr != nil {
			return nil, cerr
		}
		assetID, err := insertReceivedAsset(tx, receivedAsset{
			OutletID: outletID, Name: m.Name, Category: category,
			Qty: int(qty), Unit: m.Unit, TrackingMode: "massal",
			Price: m.UnitCost, PRNumber: "sisa projek " + m.ProjectName,
			ProjectID: m.ProjectID,
		}, actor)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE project_materials SET qty_returned = qty_returned + $1,
			updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $2`, qty, id); err != nil {
			return nil, err
		}
		if err := writeMaterialLog(tx, id, "pengembalian", qty, "asset", assetID, "",
			"Sisa dicatat sebagai aset. "+req.Notes, actor, req.PhotoURL); err != nil {
			return nil, err
		}

	case "pindah":
		if req.TargetProjectID == "" || req.TargetProjectID == m.ProjectID {
			return nil, Invalid("pilih projek tujuan yang berbeda")
		}
		var targetName string
		if err := tx.QueryRow(`SELECT name FROM projects WHERE id = $1`, req.TargetProjectID).Scan(&targetName); err != nil {
			return nil, Invalid("projek tujuan tidak ditemukan")
		}
		// Baris baru di projek tujuan; tanpa tautan pengajuan karena barangnya
		// tidak dibeli oleh projek itu.
		newID := NewULID()
		if _, err := tx.Exec(`
			INSERT INTO project_materials (id, project_id, pr_item_key, name, unit, qty_received,
				unit_cost, location, received_at, received_by, notes, created_at, updated_at)
			VALUES ($1,$2,'',$3,$4,$5,$6,'',(now() AT TIME ZONE 'UTC'),$7,$8,
				(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
			newID, req.TargetProjectID, m.Name, m.Unit, qty, m.UnitCost, actor,
			"Pindahan sisa dari projek "+m.ProjectName); err != nil {
			return nil, err
		}
		if err := writeMaterialLog(tx, newID, "penerimaan", qty, "project", m.ProjectID, "",
			"Pindahan dari projek "+m.ProjectName, actor, req.PhotoURL); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE project_materials SET qty_returned = qty_returned + $1,
			updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $2`, qty, id); err != nil {
			return nil, err
		}
		if err := writeMaterialLog(tx, id, "pengembalian", qty, "project", req.TargetProjectID, "",
			"Dipindahkan ke projek "+targetName+". "+req.Notes, actor, req.PhotoURL); err != nil {
			return nil, err
		}

	case "":
		return nil, Invalid("nasib sisa wajib dipilih — bawaannya dicatat sebagai aset")

	default:
		return nil, Invalid("nasib sisa '%s' tidak dikenal", req.Action)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetProjectMaterial(id)
}

// UnsettledProjectRow — satu projek yang sudah selesai tapi materialnya belum tuntas.
type UnsettledProjectRow struct {
	ProjectID      string  `json:"project_id"`
	ProjectNumber  string  `json:"project_number"`
	ProjectName    string  `json:"project_name"`
	Status         string  `json:"status"`
	OutletName     string  `json:"outlet_name"`
	Lines          int     `json:"lines"`
	ValueRemaining float64 `json:"value_remaining"`
}

// ListUnsettledProjectMaterials — laporan "Material Projek Belum Tuntas".
//
// Daftar ini tetap tersedia untuk rekonsiliasi data lama. Penutupan projek
// sekarang memeriksa saldo material dan laporan yang belum diverifikasi.
//
// outletIDs/wuIDs = scope role (nil = semua): projek di luar cakupan tidak
// ikut tampil, sama dengan daftar projek (T-49).
func ListUnsettledProjectMaterials(outletIDs, wuIDs []string) ([]UnsettledProjectRow, error) {
	scopeCond, scopeArgs := prScopeCond("p", outletIDs, wuIDs, 1)
	rows, err := database.DB.Query(`
		SELECT p.id, COALESCE(p.project_number, ''), p.name, p.status, COALESCE(o.name, ''),
		       COUNT(*), COALESCE(SUM((m.qty_received - m.qty_used - m.qty_returned - m.qty_wasted) * m.unit_cost), 0)
		FROM project_materials m
		JOIN projects p ON p.id = m.project_id
		LEFT JOIN outlets o ON o.id = p.outlet_id
		WHERE (m.qty_received - m.qty_used - m.qty_returned - m.qty_wasted) > 0
		  AND p.status = 'selesai'`+scopeCond+`
		GROUP BY p.id, p.project_number, p.name, p.status, o.name
		ORDER BY 7 DESC`, scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UnsettledProjectRow, 0)
	for rows.Next() {
		var r UnsettledProjectRow
		if err := rows.Scan(&r.ProjectID, &r.ProjectNumber, &r.ProjectName, &r.Status,
			&r.OutletName, &r.Lines, &r.ValueRemaining); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
