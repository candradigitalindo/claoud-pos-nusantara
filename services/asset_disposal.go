package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

var disposalValidMethod = map[string]bool{
	"dijual": true, "dihibahkan": true, "dimusnahkan": true, "hilang": true, "tukar_tambah": true,
}

func generateDisposalNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("DSP%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "asset_disposals", "disposal_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

const disposalCols = `
	d.id, d.disposal_number, d.asset_id, COALESCE(a.name, ''), COALESCE(a.asset_no, ''),
	COALESCE(a.outlet_id, ''), COALESCE(o.name, ''), d.qty, d.method, d.reason,
	d.proceeds, d.book_value, d.status, COALESCE(d.requested_by, ''), COALESCE(d.approved_by, ''),
	COALESCE(TO_CHAR(d.approved_at, 'YYYY-MM-DD HH24:MI'), ''), COALESCE(d.rejected_reason, ''),
	COALESCE(d.attachment_url, ''), TO_CHAR(d.created_at, 'YYYY-MM-DD HH24:MI')`

const disposalJoins = `
	LEFT JOIN assets a  ON a.id = d.asset_id
	LEFT JOIN outlets o ON o.id = a.outlet_id`

func scanDisposal(sc interface{ Scan(...interface{}) error }) (models.AssetDisposal, error) {
	var d models.AssetDisposal
	err := sc.Scan(&d.ID, &d.DisposalNumber, &d.AssetID, &d.AssetName, &d.AssetNo,
		&d.OutletID, &d.OutletName, &d.Qty, &d.Method, &d.Reason, &d.Proceeds, &d.BookValue,
		&d.Status, &d.RequestedBy, &d.ApprovedBy, &d.ApprovedAt, &d.RejectedReason,
		&d.AttachmentURL, &d.CreatedAt)
	return d, err
}

func ListAssetDisposals(status string, outletScope []string) ([]models.AssetDisposal, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if status != "" {
		conds = append(conds, fmt.Sprintf("d.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("a.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}
	q := fmt.Sprintf(`SELECT %s FROM asset_disposals d %s WHERE %s ORDER BY d.created_at DESC LIMIT 300`,
		disposalCols, disposalJoins, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetDisposal, 0)
	for rows.Next() {
		d, err := scanDisposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func GetAssetDisposal(id string, outletScope []string) (*models.AssetDisposal, error) {
	cond := "d.id = $1"
	args := []interface{}{id}
	if outletScope != nil {
		cond += " AND a.outlet_id = ANY($2::text[])"
		args = append(args, pq.Array(outletScope))
	}
	d, err := scanDisposal(database.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM asset_disposals d %s WHERE %s`, disposalCols, disposalJoins, cond), args...))
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateAssetDisposal mengajukan penghapusan. Aset BELUM berubah — pengajuan
// hanya menahan niat; yang mengubah data adalah persetujuan.
func CreateAssetDisposal(req models.AssetDisposalRequest, actor string, outletScope []string) (*models.AssetDisposal, error) {
	asset, err := GetAsset(req.AssetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if !disposalValidMethod[req.Method] {
		return nil, fmt.Errorf("cara penghapusan '%s' tidak dikenal", req.Method)
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("alasan penghapusan wajib diisi")
	}
	if req.Qty <= 0 {
		req.Qty = asset.Quantity
	}
	if req.Qty > asset.Quantity {
		return nil, fmt.Errorf("%s hanya tersisa %d unit", asset.Name, asset.Quantity)
	}
	if asset.Status == "transit" {
		return nil, fmt.Errorf("aset sedang dalam perjalanan mutasi")
	}
	var open int
	database.DB.QueryRow(`SELECT COUNT(*) FROM asset_disposals WHERE asset_id=$1 AND status='pending'`,
		req.AssetID).Scan(&open)
	if open > 0 {
		return nil, fmt.Errorf("sudah ada pengajuan penghapusan yang menunggu persetujuan untuk aset ini")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	number, err := generateDisposalNumber(tx)
	if err != nil {
		return nil, err
	}
	id := NewULID()
	// Nilai buku di-snapshot saat pengajuan: dihitung on-the-fly, jadi tanpa
	// snapshot angkanya berubah tiap hari dan berita acara lama tidak cocok
	// lagi dengan dokumen cetaknya.
	bookValue := asset.BookValue * float64(req.Qty)
	if _, err := tx.Exec(`
		INSERT INTO asset_disposals (id, disposal_number, asset_id, qty, method, reason,
			proceeds, book_value, status, requested_by, attachment_url, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',$9,$10,(now() AT TIME ZONE 'UTC'))`,
		id, number, req.AssetID, req.Qty, req.Method, req.Reason,
		req.Proceeds, bookValue, actor, req.AttachmentURL); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetDisposal(id, outletScope)
}

// ApproveAssetDisposal menyetujui atau menolak pengajuan. Persetujuan inilah
// yang benar-benar mengeluarkan barang dari daftar, beserta jejak nilainya.
func ApproveAssetDisposal(id string, approve bool, reason, actor string, outletScope []string) (*models.AssetDisposal, error) {
	d, err := GetAssetDisposal(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("pengajuan penghapusan tidak ditemukan")
	}
	if d.Status != "pending" {
		return nil, fmt.Errorf("pengajuan ini sudah %s", d.Status)
	}
	if !approve && strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("alasan penolakan wajib diisi")
	}
	asset, err := GetAsset(d.AssetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if !approve {
		if _, err := tx.Exec(`UPDATE asset_disposals SET status='rejected', rejected_reason=$1,
			approved_by=$2, approved_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`, reason, actor, id); err != nil {
			return nil, err
		}
	} else {
		if d.Qty > asset.Quantity {
			return nil, fmt.Errorf("jumlah yang dihapus melebihi sisa aset (%d)", asset.Quantity)
		}
		sisa := asset.Quantity - d.Qty
		if sisa <= 0 {
			// Habis. Sengaja TIDAK memakai is_deleted: baris tetap ada agar
			// berita acara, nilai buku saat dihapus, dan seluruh riwayatnya
			// masih bisa dibuka — itulah gunanya dokumen penghapusan. Yang
			// menyembunyikannya dari daftar harian adalah status 'dihapus'.
			if _, err := tx.Exec(`UPDATE assets SET quantity=0, status='dihapus',
				disposed_at=(now() AT TIME ZONE 'UTC'), updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`,
				d.AssetID); err != nil {
				return nil, err
			}
		} else if _, err := tx.Exec(`UPDATE assets SET quantity=$1, updated_at=(now() AT TIME ZONE 'UTC')
			WHERE id=$2`, sisa, d.AssetID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE asset_disposals SET status='approved', approved_by=$1,
			approved_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, id); err != nil {
			return nil, err
		}
		writeAssetMovement(tx, models.AssetMovement{
			AssetID: d.AssetID, Type: "penghapusan", Qty: -d.Qty,
			FromOutletID: asset.OutletID, FromLocation: asset.Location,
			ConditionBefore: asset.Condition, Amount: d.BookValue,
			RefType: "disposal", RefID: d.ID, RefNumber: d.DisposalNumber,
			Notes: fmt.Sprintf("Penghapusan (%s): %s", d.Method, d.Reason), Actor: actor,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetDisposal(id, outletScope)
}
