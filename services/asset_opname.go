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

// Namanya dibedakan dari generateOpnameNumber milik PPIC (opname STOK): dua
// modul berbeda, dua deret nomor berbeda — OPA untuk aset, OPN untuk stok.
func generateAssetOpnameNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("OPA%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "asset_opname_sessions", "opname_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

const opnameCols = `
	s.id, s.opname_number, s.outlet_id, COALESCE(o.name, ''), s.status, COALESCE(s.notes, ''),
	COALESCE(s.created_by, ''), COALESCE(s.approved_by, ''),
	COALESCE(TO_CHAR(s.approved_at, 'YYYY-MM-DD HH24:MI'), ''),
	TO_CHAR(s.created_at, 'YYYY-MM-DD HH24:MI'),
	COALESCE(agg.item_count, 0)::int, COALESCE(agg.counted, 0)::int, COALESCE(agg.diffs, 0)::int`

const opnameJoins = `
	LEFT JOIN outlets o ON o.id = s.outlet_id
	LEFT JOIN (
		SELECT session_id, COUNT(*) AS item_count,
		       COUNT(counted_qty) AS counted,
		       COUNT(*) FILTER (WHERE counted_qty IS NOT NULL AND counted_qty <> system_qty) AS diffs
		FROM asset_opname_items GROUP BY session_id
	) agg ON agg.session_id = s.id`

func scanOpname(sc interface{ Scan(...interface{}) error }) (models.AssetOpnameSession, error) {
	var s models.AssetOpnameSession
	err := sc.Scan(&s.ID, &s.OpnameNumber, &s.OutletID, &s.OutletName, &s.Status, &s.Notes,
		&s.CreatedBy, &s.ApprovedBy, &s.ApprovedAt, &s.CreatedAt,
		&s.ItemCount, &s.CountedCount, &s.DiffCount)
	if s.CountedCount > 0 {
		s.Accuracy = 1 - float64(s.DiffCount)/float64(s.CountedCount)
	}
	return s, err
}

func ListAssetOpnames(status string, outletScope []string) ([]models.AssetOpnameSession, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if status != "" {
		conds = append(conds, fmt.Sprintf("s.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("s.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}
	q := fmt.Sprintf(`SELECT %s FROM asset_opname_sessions s %s WHERE %s ORDER BY s.created_at DESC LIMIT 200`,
		opnameCols, opnameJoins, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetOpnameSession, 0)
	for rows.Next() {
		s, err := scanOpname(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func GetAssetOpname(id string, outletScope []string) (*models.AssetOpnameSession, error) {
	cond := "s.id = $1"
	args := []interface{}{id}
	if outletScope != nil {
		cond += " AND s.outlet_id = ANY($2::text[])"
		args = append(args, pq.Array(outletScope))
	}
	sess, err := scanOpname(database.DB.QueryRow(
		fmt.Sprintf(`SELECT %s FROM asset_opname_sessions s %s WHERE %s`, opnameCols, opnameJoins, cond), args...))
	if err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`
		SELECT i.id, COALESCE(i.asset_id, ''), COALESCE(a.name, ''), COALESCE(a.asset_no, ''),
		       COALESCE(a.unit, ''), i.system_qty, i.counted_qty, COALESCE(a.condition, ''),
		       COALESCE(i.condition_found, ''), COALESCE(i.found_name, ''), COALESCE(i.location, ''),
		       COALESCE(i.notes, '')
		FROM asset_opname_items i
		LEFT JOIN assets a ON a.id = i.asset_id
		WHERE i.session_id = $1
		ORDER BY COALESCE(a.name, i.found_name)`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it models.AssetOpnameItem
		if err := rows.Scan(&it.ID, &it.AssetID, &it.AssetName, &it.AssetNo, &it.Unit,
			&it.SystemQty, &it.CountedQty, &it.ConditionNow, &it.ConditionFound,
			&it.FoundName, &it.Location, &it.Notes); err != nil {
			return nil, err
		}
		if it.CountedQty != nil {
			it.Diff = *it.CountedQty - it.SystemQty
		}
		if it.AssetName == "" {
			it.AssetName = it.FoundName
		}
		sess.Items = append(sess.Items, it)
	}
	return &sess, rows.Err()
}

// CreateAssetOpname membuka sesi dan MEMBEKUKAN daftar aset outlet itu sebagai
// baris hitungan. Aset yang sedang transit tidak diikutkan: barangnya memang
// tidak ada di tempat, dan menghitungnya sebagai hilang akan keliru.
func CreateAssetOpname(req models.AssetOpnameCreateRequest, actor string, outletScope []string) (*models.AssetOpnameSession, error) {
	if req.OutletID == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	if !outletInScope(req.OutletID, outletScope) {
		return nil, fmt.Errorf("outlet di luar akses Anda")
	}
	var open int
	database.DB.QueryRow(`SELECT COUNT(*) FROM asset_opname_sessions WHERE outlet_id=$1 AND status='berjalan'`,
		req.OutletID).Scan(&open)
	if open > 0 {
		return nil, fmt.Errorf("masih ada sesi opname berjalan di outlet ini — selesaikan dulu")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	number, err := generateAssetOpnameNumber(tx)
	if err != nil {
		return nil, err
	}
	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO asset_opname_sessions (id, opname_number, outlet_id, status, notes, created_by, created_at)
		VALUES ($1,$2,$3,'berjalan',$4,$5,(now() AT TIME ZONE 'UTC'))`,
		id, number, req.OutletID, req.Notes, actor); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`
		INSERT INTO asset_opname_items (id, session_id, asset_id, system_qty)
		SELECT UPPER(SUBSTRING(REPLACE(gen_random_uuid()::text, '-', '') FROM 1 FOR 26)), $1, a.id, a.quantity
		FROM assets a
		WHERE a.outlet_id = $2 AND a.is_deleted = false
		  AND COALESCE(a.status, 'aktif') <> 'transit'`, id, req.OutletID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetOpname(id, outletScope)
}

// SaveOpnameCount menyimpan hasil hitung. Bisa dipanggil berkali-kali selama
// sesi berjalan — petugas menghitung bertahap, bukan sekali duduk.
func SaveOpnameCount(id string, req models.AssetOpnameCountRequest, outletScope []string) (*models.AssetOpnameSession, error) {
	sess, err := GetAssetOpname(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("sesi opname tidak ditemukan")
	}
	if sess.Status != "berjalan" {
		return nil, fmt.Errorf("sesi ini sudah %s", sess.Status)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, l := range req.Lines {
		if l.ConditionFound != "" && !assetValidCondition[l.ConditionFound] {
			return nil, fmt.Errorf("kondisi '%s' tidak dikenal", l.ConditionFound)
		}
		if l.ItemID != "" {
			if _, err := tx.Exec(`
				UPDATE asset_opname_items SET counted_qty=$1, condition_found=$2, notes=$3
				WHERE id=$4 AND session_id=$5`, l.CountedQty, l.ConditionFound, l.Notes, l.ItemID, id); err != nil {
				return nil, err
			}
			continue
		}
		// Temuan barang tak terdaftar.
		if strings.TrimSpace(l.FoundName) == "" {
			return nil, fmt.Errorf("nama barang temuan wajib diisi")
		}
		qty := 1
		if l.CountedQty != nil {
			qty = *l.CountedQty
		}
		if _, err := tx.Exec(`
			INSERT INTO asset_opname_items (id, session_id, asset_id, system_qty, counted_qty,
				condition_found, found_name, location, notes)
			VALUES ($1,$2,NULL,0,$3,$4,$5,$6,$7)`,
			NewULID(), id, qty, l.ConditionFound, l.FoundName, l.Location, l.Notes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetOpname(id, outletScope)
}

// ApproveAssetOpname menerapkan selisih:
//   - jumlah disesuaikan ke hasil hitungan, kondisi diperbarui
//   - temuan barang tak terdaftar menjadi aset baru (acquisition_src 'opname')
//   - aset yang hilang seluruhnya menjadi USULAN penghapusan, bukan langsung
//     dihapus — kehilangan barang perlu persetujuan, bukan sekadar hitungan
func ApproveAssetOpname(id, actor string, outletScope []string) (*models.AssetOpnameSession, error) {
	sess, err := GetAssetOpname(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("sesi opname tidak ditemukan")
	}
	if sess.Status != "berjalan" {
		return nil, fmt.Errorf("sesi ini sudah %s", sess.Status)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, it := range sess.Items {
		if it.CountedQty == nil {
			continue // belum dihitung: jangan diubah apa pun
		}
		counted := *it.CountedQty

		if it.AssetID == "" {
			if counted <= 0 {
				continue
			}
			assetNo, err := generateAssetNo(tx, sess.OutletID)
			if err != nil {
				return nil, err
			}
			cond := it.ConditionFound
			if cond == "" {
				cond = "baik"
			}
			newID := NewULID()
			if _, err := tx.Exec(`
				INSERT INTO assets (id, asset_no, outlet_id, code, name, category, quantity, unit,
					tracking_mode, condition, status, location, acquisition_src, notes,
					is_deleted, created_at, updated_at)
				VALUES ($1,$2,$3,'',$4,'',$5,'unit','massal',$6,'aktif',$7,'opname',$8,
					false,(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
				newID, assetNo, sess.OutletID, it.FoundName, counted, cond, it.Location,
				"Temuan opname "+sess.OpnameNumber); err != nil {
				return nil, err
			}
			writeAssetMovement(tx, models.AssetMovement{
				AssetID: newID, Type: "opname", Qty: counted, ToOutletID: sess.OutletID,
				ToLocation: it.Location, ConditionAfter: cond,
				RefType: "opname", RefID: sess.ID, RefNumber: sess.OpnameNumber,
				Notes: "Barang ditemukan saat opname, belum pernah terdata", Actor: actor,
			})
			continue
		}

		if it.ConditionFound != "" && it.ConditionFound != it.ConditionNow {
			if _, err := tx.Exec(`UPDATE assets SET condition=$1, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`,
				it.ConditionFound, it.AssetID); err != nil {
				return nil, err
			}
		}
		if counted == it.SystemQty {
			continue
		}
		if counted > 0 {
			if _, err := tx.Exec(`UPDATE assets SET quantity=$1, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`,
				counted, it.AssetID); err != nil {
				return nil, err
			}
		}
		writeAssetMovement(tx, models.AssetMovement{
			AssetID: it.AssetID, Type: "opname", Qty: counted - it.SystemQty,
			ToOutletID: sess.OutletID, ConditionBefore: it.ConditionNow, ConditionAfter: it.ConditionFound,
			RefType: "opname", RefID: sess.ID, RefNumber: sess.OpnameNumber,
			Notes: fmt.Sprintf("Selisih opname: sistem %d, terhitung %d", it.SystemQty, counted), Actor: actor,
		})
		// Hilang seluruhnya → usulan penghapusan, menunggu persetujuan.
		if counted <= 0 {
			number, err := generateDisposalNumber(tx)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(`
				INSERT INTO asset_disposals (id, disposal_number, asset_id, qty, method, reason,
					book_value, status, requested_by, created_at)
				VALUES ($1,$2,$3,$4,'hilang',$5,0,'pending',$6,(now() AT TIME ZONE 'UTC'))`,
				NewULID(), number, it.AssetID, it.SystemQty,
				"Tidak ditemukan saat opname "+sess.OpnameNumber, actor); err != nil {
				return nil, err
			}
		}
	}

	if _, err := tx.Exec(`UPDATE asset_opname_sessions SET status='selesai', approved_by=$1,
		approved_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetOpname(id, outletScope)
}
