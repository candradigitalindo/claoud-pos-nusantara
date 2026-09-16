package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// ── Bukti foto ──────────────────────────────────────────────────────────────

// SaveHandoverPhoto menyimpan satu bukti foto dan mengantrekannya untuk
// dicadangkan ke email. Statusnya 'pending' sampai penjadwal berhasil mengirim.
func SaveHandoverPhoto(ex assetExecer, moment, desk, refType, refID, refNumber, photoURL, notes, actor string) {
	if strings.TrimSpace(photoURL) == "" {
		return
	}
	ex.Exec(`
		INSERT INTO handover_photos (id, moment, desk, ref_type, ref_id, ref_number, photo_url,
			notes, actor, backup_status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending',(now() AT TIME ZONE 'UTC'))`,
		NewULID(), moment, desk, refType, refID, refNumber, photoURL, notes, actor)
}

func ListHandoverPhotos(refType, refID string) ([]models.HandoverPhoto, error) {
	rows, err := database.DB.Query(`
		SELECT id, moment, COALESCE(desk, ''), ref_type, ref_id, COALESCE(ref_number, ''),
		       photo_url, COALESCE(notes, ''), COALESCE(actor, ''), backup_status,
		       COALESCE(TO_CHAR(backup_at, 'YYYY-MM-DD HH24:MI'), ''), COALESCE(backup_error, ''),
		       COALESCE(drive_file_id, ''), COALESCE(drive_url, ''),
		       TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI')
		FROM handover_photos WHERE ref_type = $1 AND ref_id = $2
		ORDER BY created_at`, refType, refID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.HandoverPhoto, 0)
	for rows.Next() {
		var p models.HandoverPhoto
		if err := rows.Scan(&p.ID, &p.Moment, &p.Desk, &p.RefType, &p.RefID, &p.RefNumber,
			&p.PhotoURL, &p.Notes, &p.Actor, &p.BackupStatus, &p.BackupAt, &p.BackupError,
			&p.DriveFileID, &p.DriveURL, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Serah terima aset ke PIC pengaju ────────────────────────────────────────

func generateHandoverNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("SRT%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "asset_handovers", "handover_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// CreateAssetHandover menyerahkan aset dari bagian Aset kepada PIC yang
// mengajukan pembeliannya. Barang tidak berhenti di bagian aset: sampai
// dokumen ini terbit, tidak ada yang bertanggung jawab atas fisiknya.
func CreateAssetHandover(req models.AssetHandoverRequest, actor string, outletScope []string) (*models.AssetHandover, error) {
	if strings.TrimSpace(req.PicName) == "" {
		return nil, Invalid("nama PIC penerima wajib diisi")
	}
	if len(req.Items) == 0 {
		return nil, Invalid("minimal satu aset diserahkan")
	}
	if strings.TrimSpace(req.PhotoURL) == "" {
		return nil, Invalid("foto serah terima wajib diunggah — tanpa bukti, penyerahan tidak bisa dibuktikan")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	number, err := generateHandoverNumber(tx)
	if err != nil {
		return nil, err
	}
	outletID := req.OutletID
	id := NewULID()

	for _, it := range req.Items {
		var name, aOutlet, aPic string
		var qty int
		var deleted bool
		if err := tx.QueryRow(`SELECT name, outlet_id, COALESCE(pic_name,''), quantity, is_deleted
			FROM assets WHERE id = $1`, it.AssetID).Scan(&name, &aOutlet, &aPic, &qty, &deleted); err != nil {
			return nil, Invalid("aset tidak ditemukan")
		}
		if deleted {
			return nil, Invalid("%s sudah dihapus dari daftar", name)
		}
		if !outletInScope(aOutlet, outletScope) {
			return nil, Invalid("%s berada di outlet di luar akses Anda", name)
		}
		if outletID == "" {
			outletID = aOutlet
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO asset_handovers (id, handover_number, purchase_request_id, outlet_id, pic_name,
			pic_position, location, notes, handed_by, created_at)
		VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7,$8,$9,(now() AT TIME ZONE 'UTC'))`,
		id, number, req.PurchaseRequestID, outletID, req.PicName, req.PicPosition,
		req.Location, req.Notes, actor); err != nil {
		return nil, err
	}

	for _, it := range req.Items {
		qty := it.Qty
		if qty <= 0 {
			qty = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO asset_handover_items (id, handover_id, asset_id, qty)
			VALUES ($1,$2,$3,$4)`, NewULID(), id, it.AssetID, qty); err != nil {
			return nil, err
		}
		// Penanggung jawab dan lokasi aset ikut berpindah ke PIC penerima.
		var before string
		tx.QueryRow(`SELECT COALESCE(pic_name,'') FROM assets WHERE id=$1`, it.AssetID).Scan(&before)
		if _, err := tx.Exec(`UPDATE assets SET pic_name=$1,
			location = CASE WHEN $2 <> '' THEN $2 ELSE location END,
			updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`,
			req.PicName, req.Location, it.AssetID); err != nil {
			return nil, err
		}
		writeAssetMovement(tx, models.AssetMovement{
			AssetID: it.AssetID, Type: "kondisi", Qty: 0,
			ToLocation: req.Location, RefType: "asset_handover", RefID: id, RefNumber: number,
			Notes: fmt.Sprintf("Diserahkan ke %s (sebelumnya: %s)", req.PicName, orDash(before)),
			Actor: actor,
		})
	}

	SaveHandoverPhoto(tx, "distribusi", "perlengkapan", "asset_handover", id, number,
		req.PhotoURL, "Serah terima aset ke "+req.PicName, actor)

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetHandover(id, outletScope)
}

func GetAssetHandover(id string, outletScope []string) (*models.AssetHandover, error) {
	var h models.AssetHandover
	var prID, outletID sql.NullString
	err := database.DB.QueryRow(`
		SELECT h.id, h.handover_number, h.purchase_request_id, COALESCE(pr.request_number, ''),
		       h.outlet_id, COALESCE(o.name, ''), h.pic_name, COALESCE(h.pic_position, ''),
		       COALESCE(h.location, ''), COALESCE(h.notes, ''), COALESCE(h.handed_by, ''),
		       TO_CHAR(h.created_at, 'YYYY-MM-DD HH24:MI')
		FROM asset_handovers h
		LEFT JOIN outlets o ON o.id = h.outlet_id
		LEFT JOIN purchase_requests pr ON pr.id = h.purchase_request_id
		WHERE h.id = $1`, id).
		Scan(&h.ID, &h.HandoverNumber, &prID, &h.RequestNumber, &outletID, &h.OutletName,
			&h.PicName, &h.PicPosition, &h.Location, &h.Notes, &h.HandedBy, &h.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("dokumen serah terima tidak ditemukan")
	}
	h.PurchaseRequestID, h.OutletID = prID.String, outletID.String
	if !outletInScope(h.OutletID, outletScope) {
		return nil, fmt.Errorf("dokumen di luar akses Anda")
	}
	rows, err := database.DB.Query(`
		SELECT i.id, i.asset_id, COALESCE(a.asset_no,''), COALESCE(a.name,''), COALESCE(a.unit,''), i.qty
		FROM asset_handover_items i LEFT JOIN assets a ON a.id = i.asset_id
		WHERE i.handover_id = $1 ORDER BY a.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it models.AssetHandoverItem
		if err := rows.Scan(&it.ID, &it.AssetID, &it.AssetNo, &it.AssetName, &it.Unit, &it.Qty); err != nil {
			return nil, err
		}
		h.Items = append(h.Items, it)
	}
	h.Photos, _ = ListHandoverPhotos("asset_handover", id)
	return &h, nil
}

func ListAssetHandovers(outletScope []string) ([]models.AssetHandover, error) {
	q := `
		SELECT h.id, h.handover_number, COALESCE(h.purchase_request_id, ''), COALESCE(pr.request_number, ''),
		       COALESCE(h.outlet_id, ''), COALESCE(o.name, ''), h.pic_name, COALESCE(h.pic_position, ''),
		       COALESCE(h.location, ''), COALESCE(h.notes, ''), COALESCE(h.handed_by, ''),
		       TO_CHAR(h.created_at, 'YYYY-MM-DD HH24:MI')
		FROM asset_handovers h
		LEFT JOIN outlets o ON o.id = h.outlet_id
		LEFT JOIN purchase_requests pr ON pr.id = h.purchase_request_id`
	args := []interface{}{}
	if outletScope != nil {
		q += ` WHERE h.outlet_id = ANY($1::text[])`
		args = append(args, pqStringArray(outletScope))
	}
	q += ` ORDER BY h.created_at DESC LIMIT 200`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetHandover, 0)
	for rows.Next() {
		var h models.AssetHandover
		if err := rows.Scan(&h.ID, &h.HandoverNumber, &h.PurchaseRequestID, &h.RequestNumber,
			&h.OutletID, &h.OutletName, &h.PicName, &h.PicPosition, &h.Location, &h.Notes,
			&h.HandedBy, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListAssetsAwaitingHandover — aset yang sudah diterima bagian aset tapi belum
// diserahkan ke siapa pun. Inilah daftar pekerjaan distribusi bagian aset.
func ListAssetsAwaitingHandover(outletScope []string) ([]models.Asset, error) {
	q := fmt.Sprintf(`
		SELECT %s FROM assets a %s
		WHERE a.is_deleted = false AND COALESCE(a.status,'aktif') = 'aktif'
		  AND COALESCE(a.pic_name, '') = ''
		  AND NOT EXISTS (SELECT 1 FROM asset_handover_items hi WHERE hi.asset_id = a.id)`,
		assetSelectCols, assetJoins)
	args := []interface{}{}
	if outletScope != nil {
		q += ` AND a.outlet_id = ANY($1::text[])`
		args = append(args, pqStringArray(outletScope))
	}
	q += ` ORDER BY a.created_at DESC LIMIT 200`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Asset, 0)
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
