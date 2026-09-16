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

// assetScopeCond restricts assets to the scoped outlets. nil scope = all outlets.
// A sentinel like {"__none__"} naturally matches nothing.
func assetScopeCond(alias string, outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	col := "outlet_id"
	if alias != "" {
		col = alias + ".outlet_id"
	}
	return fmt.Sprintf(" AND %s = ANY($%d::text[])", col, idx), []interface{}{pq.Array(outletIDs)}
}

func nullableDate(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// assetExecer menerima *sql.DB maupun *sql.Tx, supaya penulisan buku besar bisa
// ikut transaksi pemanggilnya tanpa menduplikasi fungsi.
type assetExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// Status yang hanya boleh berubah lewat dokumen. Membiarkannya diubah lewat
// form edit berarti aset bisa "pulang" dari transit tanpa pernah diterima, dan
// dua outlet mengklaim barang yang sama.
//
// 'perbaikan' masuk daftar ini sejak Fase 3: work order perawatanlah yang
// menyalakan dan mematikannya. Menandai aset sedang diperbaiki sekarang
// dilakukan dengan membuat work order lalu menekan "Mulai" — bukan dengan
// mengubah status di form, yang membuat status dan pekerjaan bisa berbeda.
var assetSystemStatus = map[string]bool{"transit": true, "perbaikan": true, "dihapus": true}

var assetValidStatus = map[string]bool{
	"aktif": true, "dipinjam": true, "perbaikan": true,
	"transit": true, "tidak_aktif": true, "dihapus": true,
}

var assetValidCondition = map[string]bool{"baik": true, "rusak_ringan": true, "rusak_berat": true}

// Kolom aset + turunan penyusutan garis lurus. Penyusutan sengaja dihitung saat
// query, bukan disimpan: tidak ada jurnal bulanan yang bisa lupa di-posting, dan
// nilai buku tidak pernah basi.
const assetSelectCols = `
	a.id, COALESCE(a.asset_no, ''), a.outlet_id, COALESCE(o.name, ''), a.code, a.name, a.category,
	a.quantity, a.unit, COALESCE(a.tracking_mode, 'massal'), COALESCE(a.serial_number, ''),
	COALESCE(a.brand, ''), COALESCE(a.model, ''), a.condition, COALESCE(a.status, 'aktif'),
	a.location, COALESCE(a.pic_name, ''), COALESCE(a.acquisition_src, 'manual'),
	COALESCE(TO_CHAR(a.purchase_date, 'YYYY-MM-DD'), ''), a.purchase_price,
	COALESCE(TO_CHAR(a.warranty_until, 'YYYY-MM-DD'), ''), COALESCE(a.useful_life_months, 0),
	COALESCE(a.residual_value, 0), COALESCE(a.photo_url, ''), a.notes,
	COALESCE(m.cnt, 0)::int, COALESCE(m.last_date, ''), COALESCE(m.total_cost, 0),
	COALESCE(dep.months, 0)::int, COALESCE(dep.accum, 0), a.purchase_price - COALESCE(dep.accum, 0),
	a.created_at, a.updated_at`

// assetJoins menyediakan agregat perawatan + hitungan penyusutan untuk kolom di atas.
const assetJoins = `
	LEFT JOIN outlets o ON o.id = a.outlet_id
	LEFT JOIN (
		SELECT asset_id, COUNT(*) AS cnt,
		       TO_CHAR(MAX(maintenance_date), 'YYYY-MM-DD') AS last_date,
		       SUM(cost) AS total_cost
		FROM asset_maintenances GROUP BY asset_id
	) m ON m.asset_id = a.id
	LEFT JOIN LATERAL (
		SELECT months, LEAST(
			CASE WHEN COALESCE(a.useful_life_months, 0) > 0
			     THEN (a.purchase_price - COALESCE(a.residual_value, 0)) / a.useful_life_months * months
			     ELSE 0 END,
			GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0)
		) AS accum
		FROM (
			SELECT CASE WHEN a.purchase_date IS NULL THEN 0 ELSE GREATEST(0,
				(DATE_PART('year', AGE(CURRENT_DATE, a.purchase_date)) * 12
				 + DATE_PART('month', AGE(CURRENT_DATE, a.purchase_date)))::int) END AS months
		) t
	) dep ON true`

func scanAsset(sc interface{ Scan(...interface{}) error }) (models.Asset, error) {
	var a models.Asset
	err := sc.Scan(&a.ID, &a.AssetNo, &a.OutletID, &a.OutletName, &a.Code, &a.Name, &a.Category,
		&a.Quantity, &a.Unit, &a.TrackingMode, &a.SerialNumber, &a.Brand, &a.Model,
		&a.Condition, &a.Status, &a.Location, &a.PicName, &a.AcquisitionSrc,
		&a.PurchaseDate, &a.PurchasePrice, &a.WarrantyUntil, &a.UsefulLifeMonths,
		&a.ResidualValue, &a.PhotoURL, &a.Notes,
		&a.MaintenanceCount, &a.LastMaintenance, &a.MaintenanceCost,
		&a.MonthsElapsed, &a.Depreciation, &a.BookValue,
		&a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// ── Penomoran aset ──────────────────────────────────────────────────────────

// generateAssetNo membuat nomor AST-<kode outlet>-<YYMM>-<urut 3 digit>.
// Prefix sengaja berakhir tepat sebelum bagian angka, karena nextDocNumber
// mencocokkan pola '^' || prefix || '(\d+)$'.
func generateAssetNo(tx *sql.Tx, outletID string) (string, error) {
	var code string
	if err := tx.QueryRow(`SELECT COALESCE(NULLIF(code, ''), 'XXX') FROM outlets WHERE id = $1`, outletID).Scan(&code); err != nil {
		code = "XXX"
	}
	prefix := fmt.Sprintf("AST-%s-%s-", code, time.Now().In(GetTimezoneLocation()).Format("0601"))
	seq, err := nextDocNumber(tx, "assets", "asset_no", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// ── Buku besar aset ─────────────────────────────────────────────────────────

func writeAssetMovement(ex assetExecer, m models.AssetMovement) {
	if m.ID == "" {
		m.ID = NewULID()
	}
	// Buku besar tidak boleh menggagalkan operasi utama: kegagalan di sini
	// dicatat sebagai baris yang hilang, bukan sebagai aset yang batal tersimpan.
	ex.Exec(`
		INSERT INTO asset_movements (id, asset_id, type, qty, from_outlet_id, to_outlet_id,
			from_location, to_location, condition_before, condition_after,
			ref_type, ref_id, ref_number, amount, notes, actor, created_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,$11,NULLIF($12,''),$13,$14,$15,$16,
			(now() AT TIME ZONE 'UTC'))`,
		m.ID, m.AssetID, m.Type, m.Qty, m.FromOutletID, m.ToOutletID,
		m.FromLocation, m.ToLocation, m.ConditionBefore, m.ConditionAfter,
		m.RefType, m.RefID, m.RefNumber, m.Amount, m.Notes, m.Actor)
}

func ListAssetMovements(assetID string) ([]models.AssetMovement, error) {
	rows, err := database.DB.Query(`
		SELECT mv.id, mv.asset_id, mv.type, COALESCE(mv.qty, 0),
		       COALESCE(mv.from_outlet_id, ''), COALESCE(mv.to_outlet_id, ''),
		       COALESCE(fo.name, ''), COALESCE(t_o.name, ''),
		       COALESCE(mv.from_location, ''), COALESCE(mv.to_location, ''),
		       COALESCE(mv.condition_before, ''), COALESCE(mv.condition_after, ''),
		       COALESCE(mv.ref_type, ''), COALESCE(mv.ref_id, ''), COALESCE(mv.ref_number, ''),
		       COALESCE(mv.amount, 0), COALESCE(mv.notes, ''), COALESCE(mv.actor, ''), mv.created_at
		FROM asset_movements mv
		LEFT JOIN outlets fo  ON fo.id  = mv.from_outlet_id
		LEFT JOIN outlets t_o ON t_o.id = mv.to_outlet_id
		WHERE mv.asset_id = $1
		ORDER BY mv.created_at DESC, mv.id DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetMovement, 0)
	for rows.Next() {
		var m models.AssetMovement
		if err := rows.Scan(&m.ID, &m.AssetID, &m.Type, &m.Qty,
			&m.FromOutletID, &m.ToOutletID, &m.FromOutletName, &m.ToOutletName,
			&m.FromLocation, &m.ToLocation, &m.ConditionBefore, &m.ConditionAfter,
			&m.RefType, &m.RefID, &m.RefNumber, &m.Amount, &m.Notes, &m.Actor, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── Daftar & detail ─────────────────────────────────────────────────────────

// ListAssets returns assets (optionally filtered by outlet/search/condition/status)
// with maintenance aggregates and straight-line depreciation per asset.
func ListAssets(outletID, search, condition, status string, outletScope []string) ([]models.Asset, error) {
	conds := []string{"a.is_deleted = false"}
	args := []interface{}{}
	idx := 1

	if outletID != "" {
		conds = append(conds, fmt.Sprintf("a.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf(
			"(a.name ILIKE $%d OR a.code ILIKE $%d OR a.category ILIKE $%d OR a.asset_no ILIKE $%d OR a.serial_number ILIKE $%d)",
			idx, idx, idx, idx, idx))
		args = append(args, "%"+search+"%")
		idx++
	}
	if condition != "" {
		conds = append(conds, fmt.Sprintf("a.condition = $%d", idx))
		args = append(args, condition)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("COALESCE(a.status, 'aktif') = $%d", idx))
		args = append(args, status)
		idx++
	} else {
		// Aset yang sudah dihapus lewat berita acara tidak ikut daftar harian,
		// tapi barisnya tetap ada supaya riwayat dan nilai bukunya bisa dibuka.
		conds = append(conds, "COALESCE(a.status, 'aktif') <> 'dihapus'")
	}
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, idx)
	if scopeCond != "" {
		conds = append(conds, strings.TrimPrefix(scopeCond, " AND "))
		args = append(args, scopeArgs...)
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM assets a %s WHERE %s ORDER BY a.created_at DESC`,
		assetSelectCols, assetJoins, strings.Join(conds, " AND "))

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

func GetAsset(id string, outletScope []string) (*models.Asset, error) {
	scopeCond, scopeArgs := assetScopeCond("a", outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	q := fmt.Sprintf(`SELECT %s FROM assets a %s WHERE a.id = $1 AND a.is_deleted = false%s`,
		assetSelectCols, assetJoins, scopeCond)
	a, err := scanAsset(database.DB.QueryRow(q, args...))
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ── Tulis ───────────────────────────────────────────────────────────────────

// normalizeAssetInput menerapkan default dan menolak kombinasi yang tidak sah.
func normalizeAssetInput(req *models.AssetRequest, isCreate bool) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("nama aset wajib diisi")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.Unit == "" {
		req.Unit = "unit"
	}
	if req.Condition == "" {
		req.Condition = "baik"
	}
	// 'perbaikan' dulu berupa kondisi; sekarang ia status milik work order.
	// Klien lama yang masih mengirim kondisi itu tetap dilayani: kondisi fisiknya
	// diturunkan, tapi statusnya TIDAK dipasang — status perbaikan hanya sah bila
	// ada work order yang benar-benar berjalan (Fase 3).
	if req.Condition == "perbaikan" {
		req.Condition = "rusak_ringan"
	}
	if !assetValidCondition[req.Condition] {
		return fmt.Errorf("kondisi '%s' tidak dikenal", req.Condition)
	}
	if req.TrackingMode == "" {
		req.TrackingMode = "massal"
	}
	if req.TrackingMode != "tunggal" && req.TrackingMode != "massal" {
		return fmt.Errorf("mode pencatatan '%s' tidak dikenal", req.TrackingMode)
	}
	if req.TrackingMode == "tunggal" && req.Quantity != 1 {
		return fmt.Errorf("aset bernomor tunggal hanya boleh berjumlah 1 — pecah menjadi beberapa aset atau pilih mode massal")
	}
	if req.Status == "" {
		req.Status = "aktif"
	}
	if !assetValidStatus[req.Status] {
		return fmt.Errorf("status '%s' tidak dikenal", req.Status)
	}
	if isCreate && assetSystemStatus[req.Status] {
		return fmt.Errorf("status '%s' hanya bisa terjadi lewat dokumen (mutasi/perawatan/penghapusan)", req.Status)
	}
	// Kategori wajib terdaftar di master — teks bebas membuat "Elektronik",
	// "elektronik", dan "Elektronic" jadi tiga kelompok di dashboard & laporan.
	official, err := resolveAssetCategory(req.Category)
	if err != nil {
		return err
	}
	req.Category = official
	// Umur ekonomis bawaan diambil dari kategorinya bila tidak diisi.
	if req.UsefulLifeMonths == 0 && official != "" {
		if life, _ := categoryDefaults(official); life > 0 {
			req.UsefulLifeMonths = life
		}
	}
	if req.UsefulLifeMonths < 0 {
		req.UsefulLifeMonths = 0
	}
	if req.ResidualValue < 0 {
		req.ResidualValue = 0
	}
	if req.ResidualValue > req.PurchasePrice {
		return fmt.Errorf("nilai residu tidak boleh melebihi harga perolehan")
	}
	return nil
}

func CreateAsset(req models.AssetRequest, actor string) (*models.Asset, error) {
	if req.OutletID == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	if err := normalizeAssetInput(&req, true); err != nil {
		return nil, err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	assetNo, err := generateAssetNo(tx, req.OutletID)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat nomor aset: %w", err)
	}

	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO assets (id, asset_no, outlet_id, code, name, category, quantity, unit,
			tracking_mode, serial_number, brand, model, condition, status, location, pic_name,
			acquisition_src, purchase_date, purchase_price, warranty_until, useful_life_months,
			residual_value, photo_url, notes, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'manual',$17,$18,$19,$20,$21,$22,$23,
			false, (now() AT TIME ZONE 'UTC'), (now() AT TIME ZONE 'UTC'))`,
		id, assetNo, req.OutletID, req.Code, req.Name, req.Category, req.Quantity, req.Unit,
		req.TrackingMode, req.SerialNumber, req.Brand, req.Model, req.Condition, req.Status,
		req.Location, req.PicName, nullableDate(req.PurchaseDate), req.PurchasePrice,
		nullableDate(req.WarrantyUntil), req.UsefulLifeMonths, req.ResidualValue,
		req.PhotoURL, req.Notes); err != nil {
		return nil, err
	}

	writeAssetMovement(tx, models.AssetMovement{
		AssetID: id, Type: "pendataan", Qty: req.Quantity,
		ToOutletID: req.OutletID, ToLocation: req.Location,
		ConditionAfter: req.Condition, Amount: req.PurchasePrice * float64(req.Quantity),
		Notes: "Pendataan aset baru", Actor: actor,
	})

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetAsset(id, nil)
}

func UpdateAsset(id string, req models.AssetRequest, actor string, outletScope []string) (*models.Asset, error) {
	before, err := GetAsset(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if err := normalizeAssetInput(&req, false); err != nil {
		return nil, err
	}
	// Status yang dikendalikan sistem hanya boleh dipertahankan, tidak diubah
	// manual — dan tidak bisa dipasang manual oleh form.
	if req.Status != before.Status && assetSystemStatus[req.Status] {
		return nil, fmt.Errorf("status '%s' hanya bisa terjadi lewat dokumen (mutasi/perawatan/penghapusan)", req.Status)
	}
	if assetSystemStatus[before.Status] && req.Status != before.Status {
		return nil, fmt.Errorf("aset sedang berstatus '%s'; selesaikan dokumennya dulu", before.Status)
	}

	// 20 kolom + $21 = id, sehingga scope mulai di $22.
	scopeCond, scopeArgs := assetScopeCond("", outletScope, 22)
	args := append([]interface{}{
		req.Code, req.Name, req.Category, req.Quantity, req.Unit, req.TrackingMode,
		req.SerialNumber, req.Brand, req.Model, req.Condition, req.Status, req.Location,
		req.PicName, nullableDate(req.PurchaseDate), req.PurchasePrice,
		nullableDate(req.WarrantyUntil), req.UsefulLifeMonths, req.ResidualValue, req.PhotoURL,
		req.Notes, id,
	}, scopeArgs...)
	// $21 = id, $22.. = scope. Catatan: outlet_id sengaja TIDAK ikut diubah —
	// perpindahan outlet hanya sah lewat dokumen mutasi (Fase 2).
	res, err := database.DB.Exec(fmt.Sprintf(`
		UPDATE assets SET code=$1, name=$2, category=$3, quantity=$4, unit=$5, tracking_mode=$6,
			serial_number=$7, brand=$8, model=$9, condition=$10, status=$11, location=$12,
			pic_name=$13, purchase_date=$14, purchase_price=$15, warranty_until=$16,
			useful_life_months=$17, residual_value=$18, photo_url=$19, notes=$20,
			updated_at=(now() AT TIME ZONE 'UTC')
		WHERE id=$21 AND is_deleted=false%s`, scopeCond), args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}

	// Satu baris buku besar bila ada yang berpindah/berubah wujud. Perubahan
	// kosmetik (nama, catatan) tidak perlu meninggalkan jejak.
	if before.Location != req.Location || before.Condition != req.Condition ||
		before.Status != req.Status || before.PicName != req.PicName || before.Quantity != req.Quantity {
		notes := []string{}
		if before.Location != req.Location {
			notes = append(notes, fmt.Sprintf("lokasi: %s → %s", orDash(before.Location), orDash(req.Location)))
		}
		if before.PicName != req.PicName {
			notes = append(notes, fmt.Sprintf("penanggung jawab: %s → %s", orDash(before.PicName), orDash(req.PicName)))
		}
		if before.Status != req.Status {
			notes = append(notes, fmt.Sprintf("status: %s → %s", before.Status, req.Status))
		}
		if before.Quantity != req.Quantity {
			notes = append(notes, fmt.Sprintf("jumlah: %d → %d", before.Quantity, req.Quantity))
		}
		writeAssetMovement(database.DB, models.AssetMovement{
			AssetID: id, Type: "kondisi", Qty: req.Quantity - before.Quantity,
			FromLocation: before.Location, ToLocation: req.Location,
			ConditionBefore: before.Condition, ConditionAfter: req.Condition,
			Notes: strings.Join(notes, " · "), Actor: actor,
		})
	}
	return GetAsset(id, outletScope)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func DeleteAsset(id, actor string, outletScope []string) error {
	before, err := GetAsset(id, outletScope)
	if err != nil {
		return fmt.Errorf("aset tidak ditemukan")
	}
	scopeCond, scopeArgs := assetScopeCond("", outletScope, 2)
	args := append([]interface{}{id}, scopeArgs...)
	res, err := database.DB.Exec(fmt.Sprintf(
		`UPDATE assets SET is_deleted=true, updated_at=(now() AT TIME ZONE 'UTC')
		 WHERE id=$1 AND is_deleted=false%s`, scopeCond), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("aset tidak ditemukan")
	}
	// Ini penghapusan DATA (salah input), bukan penghapusan aset secara fisik —
	// penghapusan fisik punya berita acara sendiri di modul Penghapusan (Fase 4).
	writeAssetMovement(database.DB, models.AssetMovement{
		AssetID: id, Type: "penghapusan", Qty: -before.Quantity,
		FromOutletID: before.OutletID, FromLocation: before.Location,
		ConditionBefore: before.Condition,
		Notes:           "Data aset dihapus dari daftar", Actor: actor,
	})
	return nil
}
