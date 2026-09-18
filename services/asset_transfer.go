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

// Alur status mutasi aset — sama dengan pola stock_transfers.
//
//	draft ──ajukan──▶ pending ──setujui──▶ approved ──kirim──▶ sent ──terima──▶ received
//	  │                  │                     │
//	  └──batal──▶ cancelled                    └──tolak──▶ rejected
var assetTransferFlow = map[string]map[string]string{
	"submit":  {"draft": "pending"},
	"approve": {"pending": "approved"},
	"reject":  {"pending": "rejected"},
	"send":    {"approved": "sent"},
	"receive": {"sent": "received"},
	"cancel":  {"draft": "cancelled", "pending": "cancelled", "approved": "cancelled"},
}

// Status aset yang boleh ikut dimutasi. 'transit' tidak ada di sini: aset yang
// sudah berada di dokumen mutasi lain tidak boleh dikirim dua kali.
//
// 'perbaikan' hanya sah bila alasan mutasinya memang mengirim barang untuk
// diperbaiki — di luar itu, aset yang sedang dikerjakan teknisi tidak boleh
// tiba-tiba berangkat ke outlet lain (docs/perlengkapan-aset.md §6.3).
var assetTransferable = map[string]bool{"aktif": true, "tidak_aktif": true}

func assetStatusTransferable(status, reason string) bool {
	if assetTransferable[status] {
		return true
	}
	return status == "perbaikan" && reason == "perbaikan"
}

func generateAssetTransferNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("MTA%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "asset_transfers", "transfer_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// assetTransferScopeCond membatasi dokumen pada outlet yang boleh dilihat.
// Sengaja DUA ARAH: dokumen yang MENUJU outlet kita harus terlihat, kalau tidak
// barang kiriman tidak akan pernah bisa diterima.
func assetTransferScopeCond(outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	return fmt.Sprintf(" AND (t.from_outlet_id = ANY($%d::text[]) OR t.to_outlet_id = ANY($%d::text[]))", idx, idx),
		[]interface{}{pq.Array(outletIDs)}
}

func outletInScope(outletID string, scope []string) bool {
	if scope == nil {
		return true
	}
	for _, id := range scope {
		if id == outletID {
			return true
		}
	}
	return false
}

const assetTransferCols = `
	t.id, t.transfer_number, t.from_outlet_id, COALESCE(fo.name, ''), t.to_outlet_id, COALESCE(t_o.name, ''),
	t.status, COALESCE(t.reason, ''), COALESCE(TO_CHAR(t.expected_return, 'YYYY-MM-DD'), ''),
	COALESCE(t.notes, ''), COALESCE(t.photo_url, ''), COALESCE(t.rejected_reason, ''), COALESCE(t.created_by, ''),
	COALESCE(t.approved_by, ''), COALESCE(TO_CHAR(t.approved_at, 'YYYY-MM-DD HH24:MI'), ''),
	COALESCE(t.sent_by, ''), COALESCE(TO_CHAR(t.sent_at, 'YYYY-MM-DD HH24:MI'), ''),
	COALESCE(t.received_by, ''), COALESCE(TO_CHAR(t.received_at, 'YYYY-MM-DD HH24:MI'), ''),
	COALESCE(agg.item_count, 0)::int, COALESCE(agg.total_qty, 0)::int, COALESCE(agg.shortfall, false),
	TO_CHAR(t.created_at, 'YYYY-MM-DD HH24:MI'), TO_CHAR(t.updated_at, 'YYYY-MM-DD HH24:MI')`

const assetTransferJoins = `
	LEFT JOIN outlets fo  ON fo.id  = t.from_outlet_id
	LEFT JOIN outlets t_o ON t_o.id = t.to_outlet_id
	LEFT JOIN (
		SELECT transfer_id, COUNT(*) AS item_count, SUM(qty) AS total_qty,
		       BOOL_OR(received_qty IS NOT NULL AND received_qty < qty) AS shortfall
		FROM asset_transfer_items GROUP BY transfer_id
	) agg ON agg.transfer_id = t.id`

func scanAssetTransfer(sc interface{ Scan(...interface{}) error }) (models.AssetTransfer, error) {
	var t models.AssetTransfer
	err := sc.Scan(&t.ID, &t.TransferNumber, &t.FromOutletID, &t.FromOutletName,
		&t.ToOutletID, &t.ToOutletName, &t.Status, &t.Reason, &t.ExpectedReturn,
		&t.Notes, &t.PhotoURL, &t.RejectedReason, &t.CreatedBy, &t.ApprovedBy, &t.ApprovedAt,
		&t.SentBy, &t.SentAt, &t.ReceivedBy, &t.ReceivedAt,
		&t.ItemCount, &t.TotalQty, &t.HasShortfall, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func ListAssetTransfers(status, outletID string, outletScope []string) ([]models.AssetTransfer, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if status != "" {
		conds = append(conds, fmt.Sprintf("t.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("(t.from_outlet_id = $%d OR t.to_outlet_id = $%d)", idx, idx))
		args = append(args, outletID)
		idx++
	}
	scopeCond, scopeArgs := assetTransferScopeCond(outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
	}
	q := fmt.Sprintf(`SELECT %s FROM asset_transfers t %s WHERE %s ORDER BY t.created_at DESC LIMIT 200`,
		assetTransferCols, assetTransferJoins, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetTransfer, 0)
	for rows.Next() {
		t, err := scanAssetTransfer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func GetAssetTransfer(id string, outletScope []string) (*models.AssetTransfer, error) {
	scopeCond, scopeArgs := assetTransferScopeCond(outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	q := fmt.Sprintf(`SELECT %s FROM asset_transfers t %s WHERE t.id = $1%s`,
		assetTransferCols, assetTransferJoins, scopeCond)
	t, err := scanAssetTransfer(database.DB.QueryRow(q, args...))
	if err != nil {
		return nil, err
	}
	items, err := listAssetTransferItems(id)
	if err != nil {
		return nil, err
	}
	t.Items = items
	return &t, nil
}

func listAssetTransferItems(transferID string) ([]models.AssetTransferItem, error) {
	rows, err := database.DB.Query(`
		SELECT i.id, i.asset_id, COALESCE(a.asset_no, ''), COALESCE(a.name, ''), COALESCE(a.unit, ''),
		       COALESCE(a.tracking_mode, 'massal'), i.qty, i.received_qty,
		       COALESCE(i.condition_sent, ''), COALESCE(i.condition_recv, ''),
		       COALESCE(i.target_asset_id, ''), COALESCE(a.quantity, 0), COALESCE(i.notes, '')
		FROM asset_transfer_items i
		LEFT JOIN assets a ON a.id = i.asset_id
		WHERE i.transfer_id = $1
		ORDER BY a.name`, transferID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetTransferItem, 0)
	for rows.Next() {
		var it models.AssetTransferItem
		if err := rows.Scan(&it.ID, &it.AssetID, &it.AssetNo, &it.AssetName, &it.Unit,
			&it.TrackingMode, &it.Qty, &it.ReceivedQty, &it.ConditionSent, &it.ConditionRecv,
			&it.TargetAssetID, &it.AvailableQty, &it.Notes); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ── Pembuatan & perubahan draft ─────────────────────────────────────────────

func validateTransferInput(req models.AssetTransferRequest) error {
	if req.FromOutletID == "" || req.ToOutletID == "" {
		return fmt.Errorf("outlet asal dan tujuan wajib dipilih")
	}
	if req.FromOutletID == req.ToOutletID {
		return fmt.Errorf("outlet asal dan tujuan tidak boleh sama")
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("minimal satu aset dipilih")
	}
	seen := map[string]bool{}
	for _, it := range req.Items {
		if it.AssetID == "" {
			return fmt.Errorf("aset tidak valid")
		}
		if seen[it.AssetID] {
			return fmt.Errorf("aset yang sama dipilih dua kali")
		}
		seen[it.AssetID] = true
		if it.Qty <= 0 {
			return fmt.Errorf("jumlah yang dimutasi harus lebih dari 0")
		}
	}
	return nil
}

// checkTransferableAsset memastikan aset boleh ikut dokumen ini: milik outlet
// asal, jumlahnya cukup, statusnya sah, dan belum terikat mutasi lain yang
// masih berjalan.
func checkTransferableAsset(q interface {
	QueryRow(string, ...interface{}) *sql.Row
}, assetID, fromOutletID string, qty int, excludeTransferID, reason string) (string, error) {
	var name, status, cond string
	var available int
	var deleted bool
	var outletID string
	err := q.QueryRow(`SELECT name, COALESCE(status,'aktif'), condition, quantity, is_deleted, outlet_id
		FROM assets WHERE id = $1`, assetID).Scan(&name, &status, &cond, &available, &deleted, &outletID)
	if err != nil {
		return "", fmt.Errorf("aset tidak ditemukan")
	}
	if deleted {
		return name, fmt.Errorf("%s sudah dihapus dari daftar", name)
	}
	if outletID != fromOutletID {
		return name, fmt.Errorf("%s bukan milik outlet asal", name)
	}
	if !assetStatusTransferable(status, reason) {
		if status == "perbaikan" {
			return name, fmt.Errorf("%s sedang dikerjakan teknisi — pilih alasan \"Dikirim untuk diperbaiki\" bila memang hendak dikirim ke bengkel", name)
		}
		return name, fmt.Errorf("%s sedang berstatus '%s' dan tidak bisa dimutasi", name, status)
	}
	if qty > available {
		return name, fmt.Errorf("%s hanya tersedia %d unit", name, available)
	}
	var busy int
	if err := q.QueryRow(`
		SELECT COUNT(*) FROM asset_transfer_items i
		JOIN asset_transfers t ON t.id = i.transfer_id
		WHERE i.asset_id = $1 AND t.status IN ('draft','pending','approved','sent') AND t.id <> $2`,
		assetID, excludeTransferID).Scan(&busy); err == nil && busy > 0 {
		return name, fmt.Errorf("%s sudah tercantum di dokumen mutasi lain yang belum selesai", name)
	}
	return name, nil
}

func CreateAssetTransfer(req models.AssetTransferRequest, actor string, outletScope []string) (*models.AssetTransfer, error) {
	if err := validateTransferInput(req); err != nil {
		return nil, err
	}
	// Yang membuat dokumen adalah outlet ASAL — itulah yang melepas barang.
	if !outletInScope(req.FromOutletID, outletScope) {
		return nil, fmt.Errorf("outlet asal di luar akses Anda")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	number, err := generateAssetTransferNumber(tx)
	if err != nil {
		return nil, err
	}
	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO asset_transfers (id, transfer_number, from_outlet_id, to_outlet_id, status,
			reason, expected_return, notes, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'draft',$5,$6,$7,$8,(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
		id, number, req.FromOutletID, req.ToOutletID, req.Reason,
		nullableDate(req.ExpectedReturn), req.Notes, actor); err != nil {
		return nil, err
	}
	for _, it := range req.Items {
		if _, err := checkTransferableAsset(tx, it.AssetID, req.FromOutletID, it.Qty, id, req.Reason); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
			INSERT INTO asset_transfer_items (id, transfer_id, asset_id, qty, notes)
			VALUES ($1,$2,$3,$4,$5)`, NewULID(), id, it.AssetID, it.Qty, it.Notes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetTransfer(id, outletScope)
}

func UpdateAssetTransfer(id string, req models.AssetTransferRequest, outletScope []string) (*models.AssetTransfer, error) {
	before, err := GetAssetTransfer(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("dokumen mutasi tidak ditemukan")
	}
	if before.Status != "draft" {
		return nil, fmt.Errorf("hanya draft yang bisa diubah; dokumen ini berstatus '%s'", before.Status)
	}
	req.FromOutletID = before.FromOutletID // asal terkunci setelah dokumen terbit
	if err := validateTransferInput(req); err != nil {
		return nil, err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE asset_transfers SET to_outlet_id=$1, reason=$2, expected_return=$3, notes=$4,
			updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$5`,
		req.ToOutletID, req.Reason, nullableDate(req.ExpectedReturn), req.Notes, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM asset_transfer_items WHERE transfer_id = $1`, id); err != nil {
		return nil, err
	}
	for _, it := range req.Items {
		if _, err := checkTransferableAsset(tx, it.AssetID, before.FromOutletID, it.Qty, id, req.Reason); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`
			INSERT INTO asset_transfer_items (id, transfer_id, asset_id, qty, notes)
			VALUES ($1,$2,$3,$4,$5)`, NewULID(), id, it.AssetID, it.Qty, it.Notes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetTransfer(id, outletScope)
}

func DeleteAssetTransfer(id string, outletScope []string) error {
	before, err := GetAssetTransfer(id, outletScope)
	if err != nil {
		return fmt.Errorf("dokumen mutasi tidak ditemukan")
	}
	if before.Status != "draft" {
		return fmt.Errorf("hanya draft yang bisa dihapus")
	}
	_, err = database.DB.Exec(`DELETE FROM asset_transfers WHERE id = $1`, id)
	return err
}

// ── Transisi status + efek ke aset ──────────────────────────────────────────

// AssetTransferAction menjalankan satu aksi alur. Seluruh efek ke aset terjadi
// di dalam SATU transaksi: aset tidak boleh pernah berada di dua outlet
// sekaligus, atau lenyap dari keduanya.
func AssetTransferAction(id, action string, req models.AssetTransferActionRequest, actor string, outletScope []string) (*models.AssetTransfer, error) {
	flow, ok := assetTransferFlow[action]
	if !ok {
		return nil, fmt.Errorf("aksi '%s' tidak dikenal", action)
	}
	before, err := GetAssetTransfer(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("dokumen mutasi tidak ditemukan")
	}
	newStatus, ok := flow[before.Status]
	if !ok {
		return nil, fmt.Errorf("tidak bisa %s dari status '%s'", action, before.Status)
	}
	// Siapa boleh apa: yang melepas barang adalah outlet asal, yang menerima
	// adalah outlet tujuan. Tanpa pemisahan ini, satu orang bisa mengirim
	// sekaligus menerima — dan selisih tidak akan pernah ketahuan.
	switch action {
	case "send", "cancel", "submit":
		if !outletInScope(before.FromOutletID, outletScope) {
			return nil, fmt.Errorf("hanya outlet asal yang bisa melakukan ini")
		}
	case "receive":
		if !outletInScope(before.ToOutletID, outletScope) {
			return nil, fmt.Errorf("hanya outlet tujuan yang bisa menerima mutasi ini")
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	switch action {
	case "send":
		// Foto saat barang berangkat wajib, sama seperti transfer stok dan serah
		// terima ke PIC: tanpa itu, selisih di tujuan tidak bisa ditelusuri.
		if strings.TrimSpace(req.PhotoURL) == "" {
			return nil, Invalid("foto barang saat dikirim wajib diunggah — tanpa bukti, selisih di outlet tujuan tidak bisa ditelusuri")
		}
		if err := applyTransferSend(tx, before, actor); err != nil {
			return nil, err
		}
		_, err = tx.Exec(`UPDATE asset_transfers SET status='sent', sent_by=$1, sent_at=(now() AT TIME ZONE 'UTC'),
			photo_url=$3, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, id, req.PhotoURL)
		if err == nil {
			SaveHandoverPhoto(tx, "distribusi", "perlengkapan", "asset_transfer", id, before.TransferNumber,
				req.PhotoURL, "Mutasi aset "+before.FromOutletName+" → "+before.ToOutletName, actor)
		}
	case "receive":
		// Yang mengirim bukan yang menerima — dijaga di sini, bukan hanya lewat
		// pemisahan izin: dua izin bisa saja jatuh ke satu orang.
		if before.SentBy != "" && before.SentBy == actor {
			return nil, Invalid("pengirim tidak boleh menerima mutasi yang sama — penerimaan dicatat petugas outlet tujuan")
		}
		if err := applyTransferReceive(tx, before, req, actor); err != nil {
			return nil, err
		}
		_, err = tx.Exec(`UPDATE asset_transfers SET status='received', received_by=$1, received_at=(now() AT TIME ZONE 'UTC'),
			updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, id)
	case "approve":
		_, err = tx.Exec(`UPDATE asset_transfers SET status='approved', approved_by=$1, approved_at=(now() AT TIME ZONE 'UTC'),
			updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, id)
	case "reject":
		if strings.TrimSpace(req.Reason) == "" {
			return nil, fmt.Errorf("alasan penolakan wajib diisi")
		}
		_, err = tx.Exec(`UPDATE asset_transfers SET status='rejected', rejected_reason=$1, approved_by=$2,
			approved_at=(now() AT TIME ZONE 'UTC'), updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`,
			req.Reason, actor, id)
	default: // submit, cancel
		_, err = tx.Exec(`UPDATE asset_transfers SET status=$1, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`,
			newStatus, id)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAssetTransfer(id, outletScope)
}

// applyTransferSend melepas barang dari outlet asal.
//
//   - aset tunggal: barisnya sendiri yang berangkat → status 'transit'
//   - aset massal : jumlah di outlet asal berkurang; bila habis, barisnya
//     ikut 'transit' supaya tidak bisa dipilih dokumen lain
func applyTransferSend(tx *sql.Tx, t *models.AssetTransfer, actor string) error {
	for _, it := range t.Items {
		name, err := checkTransferableAsset(tx, it.AssetID, t.FromOutletID, it.Qty, t.ID, t.Reason)
		if err != nil {
			return err
		}
		var cond, mode string
		var quantity int
		if err := tx.QueryRow(`SELECT condition, COALESCE(tracking_mode,'massal'), quantity FROM assets WHERE id=$1`,
			it.AssetID).Scan(&cond, &mode, &quantity); err != nil {
			return err
		}
		if mode == "tunggal" {
			if _, err := tx.Exec(`UPDATE assets SET status='transit', updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`,
				it.AssetID); err != nil {
				return err
			}
		} else {
			sisa := quantity - it.Qty
			status := "aktif"
			if sisa == 0 {
				status = "transit"
			}
			if _, err := tx.Exec(`UPDATE assets SET quantity=$1, status=$2, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`,
				sisa, status, it.AssetID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`UPDATE asset_transfer_items SET condition_sent=$1 WHERE id=$2`, cond, it.ID); err != nil {
			return err
		}
		writeAssetMovement(tx, models.AssetMovement{
			AssetID: it.AssetID, Type: "mutasi_keluar", Qty: -it.Qty,
			FromOutletID: t.FromOutletID, ToOutletID: t.ToOutletID,
			ConditionBefore: cond, ConditionAfter: cond,
			RefType: "asset_transfer", RefID: t.ID, RefNumber: t.TransferNumber,
			Notes: fmt.Sprintf("Dikirim ke %s (%s)", t.ToOutletName, name), Actor: actor,
		})
	}
	return nil
}

// applyTransferReceive menempatkan barang di outlet tujuan.
//
// Aset tunggal berpindah utuh beserta nomor dan riwayatnya. Aset massal
// digabung ke baris sepadan bila ada — dengan tanggal & harga perolehan
// DIWARISI dari baris asal, supaya penyusutannya tidak ter-reset menjadi
// barang baru. Kekurangan penerimaan dikembalikan ke outlet asal, bukan
// dianggap hilang diam-diam.
func applyTransferReceive(tx *sql.Tx, t *models.AssetTransfer, req models.AssetTransferActionRequest, actor string) error {
	recvByItem := map[string]models.AssetTransferReceiptLine{}
	for _, line := range req.Items {
		recvByItem[line.ItemID] = line
	}

	for _, it := range t.Items {
		recv := it.Qty
		condRecv := it.ConditionSent
		if line, ok := recvByItem[it.ID]; ok {
			recv = line.ReceivedQty
			if strings.TrimSpace(line.ConditionRecv) != "" {
				condRecv = line.ConditionRecv
			}
		}
		if recv < 0 {
			recv = 0
		}
		if recv > it.Qty {
			return fmt.Errorf("%s: jumlah diterima (%d) melebihi yang dikirim (%d)", it.AssetName, recv, it.Qty)
		}
		if condRecv != "" && !assetValidCondition[condRecv] {
			return fmt.Errorf("kondisi '%s' tidak dikenal", condRecv)
		}

		var mode, srcCond string
		var srcQty int
		var purchaseDate sql.NullString
		var price, residual float64
		var life int
		var category, unit, name, code, brand, model, serial string
		if err := tx.QueryRow(`
			SELECT COALESCE(tracking_mode,'massal'), condition, quantity,
			       TO_CHAR(purchase_date,'YYYY-MM-DD'), purchase_price, COALESCE(residual_value,0),
			       COALESCE(useful_life_months,0), category, unit, name, code,
			       COALESCE(brand,''), COALESCE(model,''), COALESCE(serial_number,'')
			FROM assets WHERE id=$1`, it.AssetID).
			Scan(&mode, &srcCond, &srcQty, &purchaseDate, &price, &residual, &life,
				&category, &unit, &name, &code, &brand, &model, &serial); err != nil {
			return err
		}
		if condRecv == "" {
			condRecv = srcCond
		}

		targetID := ""
		if mode == "tunggal" {
			if recv == 0 {
				// Tidak jadi diterima: barang kembali menjadi milik outlet asal.
				if _, err := tx.Exec(`UPDATE assets SET status='aktif', updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`,
					it.AssetID); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(`
					UPDATE assets SET outlet_id=$1, status='aktif', condition=$2, location='',
						updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`,
					t.ToOutletID, condRecv, it.AssetID); err != nil {
					return err
				}
				targetID = it.AssetID
			}
		} else if recv > 0 {
			// Cari baris sepadan di outlet tujuan untuk digabung.
			var existing string
			err := tx.QueryRow(`
				SELECT id FROM assets
				WHERE outlet_id=$1 AND is_deleted=false AND COALESCE(tracking_mode,'massal')='massal'
				  AND lower(name)=lower($2) AND lower(COALESCE(category,''))=lower($3)
				  AND lower(COALESCE(unit,''))=lower($4) AND condition=$5
				  AND COALESCE(status,'aktif') IN ('aktif','tidak_aktif')
				ORDER BY created_at LIMIT 1`,
				t.ToOutletID, name, category, unit, condRecv).Scan(&existing)
			if err == nil && existing != "" {
				if _, err := tx.Exec(`UPDATE assets SET quantity = quantity + $1, updated_at=(now() AT TIME ZONE 'UTC')
					WHERE id=$2`, recv, existing); err != nil {
					return err
				}
				targetID = existing
			} else {
				newID := NewULID()
				assetNo, err := generateAssetNo(tx, t.ToOutletID)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(`
					INSERT INTO assets (id, asset_no, outlet_id, code, name, category, quantity, unit,
						tracking_mode, serial_number, brand, model, condition, status, location,
						acquisition_src, purchase_date, purchase_price, useful_life_months, residual_value,
						notes, is_deleted, created_at, updated_at)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'massal',$9,$10,$11,$12,'aktif','', 'mutasi',
						NULLIF($13,'')::date,$14,$15,$16,$17,false,(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
					newID, assetNo, t.ToOutletID, code, name, category, recv, unit,
					serial, brand, model, condRecv, purchaseDate.String, price, life, residual,
					fmt.Sprintf("Mutasi dari %s (%s)", t.FromOutletName, t.TransferNumber)); err != nil {
					return err
				}
				targetID = newID
			}
		}

		// Sisa yang tidak sampai kembali menjadi milik outlet asal.
		shortfall := it.Qty - recv
		if mode != "tunggal" {
			if shortfall > 0 {
				if _, err := tx.Exec(`UPDATE assets SET quantity = quantity + $1, updated_at=(now() AT TIME ZONE 'UTC')
					WHERE id=$2`, shortfall, it.AssetID); err != nil {
					return err
				}
				writeAssetMovement(tx, models.AssetMovement{
					AssetID: it.AssetID, Type: "mutasi_masuk", Qty: shortfall,
					ToOutletID: t.FromOutletID, ConditionAfter: srcCond,
					RefType: "asset_transfer", RefID: t.ID, RefNumber: t.TransferNumber,
					Notes: fmt.Sprintf("Selisih penerimaan: %d dari %d unit kembali ke asal", shortfall, it.Qty),
					Actor: actor,
				})
			}
			// Baris asal yang habis seluruhnya tidak lagi berada di outlet asal.
			var left int
			if err := tx.QueryRow(`SELECT quantity FROM assets WHERE id=$1`, it.AssetID).Scan(&left); err != nil {
				return err
			}
			if left <= 0 {
				if _, err := tx.Exec(`UPDATE assets SET is_deleted=true, status='aktif',
					updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$1`, it.AssetID); err != nil {
					return err
				}
			} else if _, err := tx.Exec(`UPDATE assets SET status='aktif', updated_at=(now() AT TIME ZONE 'UTC')
				WHERE id=$1 AND COALESCE(status,'aktif')='transit'`, it.AssetID); err != nil {
				return err
			}
		}

		if recv > 0 && targetID != "" {
			writeAssetMovement(tx, models.AssetMovement{
				AssetID: targetID, Type: "mutasi_masuk", Qty: recv,
				FromOutletID: t.FromOutletID, ToOutletID: t.ToOutletID,
				ConditionBefore: it.ConditionSent, ConditionAfter: condRecv,
				RefType: "asset_transfer", RefID: t.ID, RefNumber: t.TransferNumber,
				Notes: fmt.Sprintf("Diterima dari %s (%s)", t.FromOutletName, t.TransferNumber), Actor: actor,
			})
		}

		if _, err := tx.Exec(`UPDATE asset_transfer_items SET received_qty=$1, condition_recv=$2,
			target_asset_id=NULLIF($3,'') WHERE id=$4`, recv, condRecv, targetID, it.ID); err != nil {
			return err
		}
	}
	return nil
}

// ListTransferableAssets memberi daftar aset yang sah dipilih untuk mutasi dari
// sebuah outlet: bukan transit, bukan terhapus, dan belum terikat dokumen lain.
func ListTransferableAssets(outletID, reason string, outletScope []string) ([]models.Asset, error) {
	if !outletInScope(outletID, outletScope) {
		return nil, fmt.Errorf("outlet di luar akses Anda")
	}
	// Aset yang sedang dikerjakan hanya ditawarkan bila memang hendak dikirim
	// untuk diperbaiki — daftar pilihan mengikuti aturan yang sama dengan
	// pemeriksaan saat dokumen disimpan, supaya tidak ada pilihan yang menipu.
	allowed := []string{"aktif", "tidak_aktif"}
	if reason == "perbaikan" {
		allowed = append(allowed, "perbaikan")
	}
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT %s FROM assets a %s
		WHERE a.outlet_id = $1 AND a.is_deleted = false
		  AND COALESCE(a.status,'aktif') = ANY($2::text[])
		  AND a.quantity > 0
		  AND NOT EXISTS (
			SELECT 1 FROM asset_transfer_items i JOIN asset_transfers t2 ON t2.id = i.transfer_id
			WHERE i.asset_id = a.id AND t2.status IN ('draft','pending','approved','sent'))
		ORDER BY a.name`, assetSelectCols, assetJoins), outletID, pq.Array(allowed))
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
