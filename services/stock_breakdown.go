package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

// ── Pemotongan (satu bahan → beberapa hasil) ──────────────────
//
// Kebalikan Produksi (ProduceStockItem: banyak bahan → satu hasil dengan rasio
// tetap). Satu bahan dipotong/diolah menjadi beberapa hasil yang dipakai menu
// berbeda — mis. ayam utuh → paha, dada, sayap; daging sapi → has dalam, has
// luar, tetelan; ikan utuh → fillet, kepala; beras → nasi. Jumlah hasilnya
// tidak pernah persis sama, jadi stok dan HPP mengikuti hasil AKTUAL.
//
// Satu bahan boleh punya beberapa POLA bernama (mis. ayam "Per Bagian" vs
// "Potong 8"; daging sapi "Steak" vs "Rendang"). Pola = katalog global yang
// dikelola Gudang Induk: hasil standar per 1 satuan distribusi bahan (dalam
// satuan dasar hasil — berat maupun potongan) sebagai pembanding susut, plus
// bobot biaya untuk membagi biaya bahan. Hasil sebuah pola boleh menjadi bahan
// pola lain (bertingkat, mis. daging sapi → has dalam → steak 200 g).
//
// Movement: bahan keluar 'breakdown_out' (FIFO), hasil masuk 'breakdown_in';
// ref_type 'stock_breakdown', ref_number nomor dokumen POT-YYYYMMDD-NNN.

// breakdownLabelUnit: label satuan distribusi untuk pesan — label kemasan bila
// ada, selain itu kode satuannya.
func breakdownLabelUnit(label, unit string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return unit
}

// ── Pola (template) ───────────────────────────────────────────

// GetStockItemBreakdown: semua pola pemotongan satu bahan (aktif maupun
// nonaktif) beserta hasilnya. Patterns kosong bila belum diatur.
func GetStockItemBreakdown(inputItemID string) (*models.StockItemBreakdownTemplate, error) {
	t := &models.StockItemBreakdownTemplate{Patterns: []models.StockBreakdownPattern{}}
	err := database.DB.QueryRow(`
		SELECT id, code, name, base_unit, dist_unit, COALESCE(dist_unit_label, ''),
		       COALESCE(NULLIF(dist_ratio, 0), 1), is_active
		FROM stock_items WHERE id = $1`, inputItemID).Scan(
		&t.InputItemID, &t.InputItemCode, &t.InputItemName, &t.InputBaseUnit, &t.InputDistUnit,
		&t.InputDistUnitLabel, &t.InputDistRatio, &t.InputIsActive)
	if err == sql.ErrNoRows {
		return nil, Invalid("bahan baku tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	all, err := loadBreakdownTemplates(inputItemID, false)
	if err != nil {
		return nil, err
	}
	if len(all) == 1 {
		t.Patterns = all[0].Patterns
	}
	return t, nil
}

// ListStockItemBreakdowns: bahan yang punya pola pemotongan AKTIF (yang sudah
// berisi hasil) beserta pola-pola aktifnya — sumber pilihan di form Pemotongan.
func ListStockItemBreakdowns() ([]models.StockItemBreakdownTemplate, error) {
	all, err := loadBreakdownTemplates("", true)
	if err != nil {
		return nil, err
	}
	out := make([]models.StockItemBreakdownTemplate, 0, len(all))
	for _, t := range all {
		usable := make([]models.StockBreakdownPattern, 0, len(t.Patterns))
		for _, p := range t.Patterns {
			if len(p.Outputs) > 0 {
				usable = append(usable, p)
			}
		}
		if len(usable) > 0 {
			t.Patterns = usable
			out = append(out, t)
		}
	}
	return out, nil
}

// loadBreakdownTemplates mengelompokkan pola & baris hasil per bahan;
// inputItemID kosong = semua bahan, activeOnly = hanya pola aktif. Urutan pola
// mengikuti urutan pembuatannya.
func loadBreakdownTemplates(inputItemID string, activeOnly bool) ([]models.StockItemBreakdownTemplate, error) {
	q := `
		SELECT p.input_item_id, si.code, si.name, si.base_unit, si.dist_unit, COALESCE(si.dist_unit_label, ''),
		       COALESCE(NULLIF(si.dist_ratio, 0), 1), si.is_active,
		       p.id, p.name, p.notes, p.is_active, p.updated_by,
		       TO_CHAR(p.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       COALESCE(b.id::text, ''), COALESCE(b.output_item_id::text, ''), COALESCE(so.code, ''), COALESCE(so.name, ''),
		       COALESCE(so.base_unit, ''), COALESCE(so.is_active, false),
		       COALESCE(b.qty_per_input, 0), COALESCE(b.cost_weight, 0), COALESCE(b.notes, '')
		FROM stock_breakdown_patterns p
		JOIN stock_items si ON si.id = p.input_item_id
		LEFT JOIN stock_item_breakdowns b ON b.pattern_id = p.id
		LEFT JOIN stock_items so ON so.id = b.output_item_id
		WHERE 1=1`
	args := []interface{}{}
	if inputItemID != "" {
		args = append(args, inputItemID)
		q += ` AND p.input_item_id = $1`
	}
	if activeOnly {
		q += ` AND p.is_active`
	}
	q += ` ORDER BY si.name, si.id, p.created_at, p.id, so.name`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.StockItemBreakdownTemplate{}
	for rows.Next() {
		var t models.StockItemBreakdownTemplate
		var p models.StockBreakdownPattern
		var l models.StockItemBreakdownLine
		if err := rows.Scan(&t.InputItemID, &t.InputItemCode, &t.InputItemName, &t.InputBaseUnit, &t.InputDistUnit,
			&t.InputDistUnitLabel, &t.InputDistRatio, &t.InputIsActive,
			&p.ID, &p.Name, &p.Notes, &p.IsActive, &p.UpdatedBy, &p.UpdatedAt,
			&l.ID, &l.OutputItemID, &l.OutputItemCode, &l.OutputItemName, &l.OutputBaseUnit, &l.OutputIsActive,
			&l.QtyPerInput, &l.CostWeight, &l.Notes); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].InputItemID != t.InputItemID {
			t.Patterns = []models.StockBreakdownPattern{}
			out = append(out, t)
		}
		cur := &out[len(out)-1]
		if n := len(cur.Patterns); n == 0 || cur.Patterns[n-1].ID != p.ID {
			p.Outputs = []models.StockItemBreakdownLine{}
			cur.Patterns = append(cur.Patterns, p)
		}
		if l.ID != "" {
			pat := &cur.Patterns[len(cur.Patterns)-1]
			pat.Outputs = append(pat.Outputs, l)
		}
	}
	return out, rows.Err()
}

// SaveStockItemBreakdown menyimpan SELURUH pola satu bahan (ganti semua): pola
// ber-id diperbarui di tempat (id-nya tetap, jadi dokumen lama tetap tertaut),
// pola tanpa id dibuat, pola yang tidak dikirim dihapus beserta hasilnya.
// Dokumen pemotongan lama tidak berubah: nama pola, hasil standar & bobot
// sudah tersalin ke dokumennya.
func SaveStockItemBreakdown(inputItemID string, req models.StockItemBreakdownRequest, actor string) (*models.StockItemBreakdownTemplate, error) {
	input, err := GetStockItem(inputItemID)
	if err != nil {
		return nil, Invalid("bahan baku tidak ditemukan")
	}
	names := map[string]bool{}
	for pi := range req.Patterns {
		p := &req.Patterns[pi]
		p.ID = strings.TrimSpace(p.ID)
		p.Name = strings.TrimSpace(p.Name)
		p.Notes = strings.TrimSpace(p.Notes)
		if p.Name == "" {
			return nil, Invalid("nama pola ke-%d wajib diisi", pi+1)
		}
		if utf8.RuneCountInString(p.Name) > 100 {
			return nil, Invalid("nama pola \"%s\" terlalu panjang (maks. 100 karakter)", p.Name)
		}
		key := strings.ToLower(p.Name)
		if names[key] {
			return nil, Invalid("nama pola \"%s\" dipakai lebih dari sekali", p.Name)
		}
		names[key] = true
		if len(p.Outputs) == 0 {
			return nil, Invalid("pola \"%s\" belum punya hasil", p.Name)
		}
		seen := map[string]bool{}
		for i := range p.Outputs {
			l := &p.Outputs[i]
			l.OutputItemID = strings.TrimSpace(l.OutputItemID)
			if l.OutputItemID == "" {
				return nil, Invalid("pola \"%s\": item hasil baris %d wajib dipilih", p.Name, i+1)
			}
			if l.OutputItemID == inputItemID {
				return nil, Invalid("pola \"%s\": %s tidak bisa menjadi hasil pemotongannya sendiri", p.Name, input.Name)
			}
			if seen[l.OutputItemID] {
				return nil, Invalid("pola \"%s\": item hasil baris %d dipilih lebih dari sekali", p.Name, i+1)
			}
			seen[l.OutputItemID] = true
			if l.QtyPerInput <= 0 {
				return nil, Invalid("pola \"%s\": hasil standar baris %d harus lebih dari 0", p.Name, i+1)
			}
			if l.CostWeight != nil && *l.CostWeight <= 0 {
				return nil, Invalid("pola \"%s\": bobot biaya baris %d harus lebih dari 0", p.Name, i+1)
			}
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Kunci item bahan: dua penyimpanan bersamaan atas pola bahan yang sama
	// berurutan, bukan saling menimpa sebagian.
	if _, err := tx.Exec(`SELECT id FROM stock_items WHERE id = $1 FOR UPDATE`, inputItemID); err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	rows, err := tx.Query(`SELECT id FROM stock_breakdown_patterns WHERE input_item_id = $1`, inputItemID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		existing[strings.TrimSpace(id)] = true
	}
	rows.Close()
	keep := map[string]bool{}
	for _, p := range req.Patterns {
		if p.ID == "" {
			continue
		}
		if !existing[p.ID] {
			return nil, Invalid("pola \"%s\" tidak ditemukan — muat ulang lalu simpan lagi", p.Name)
		}
		keep[p.ID] = true
	}
	// 1. Pola yang tidak dikirim dihapus (baris hasilnya ikut; dokumen lama
	//    kehilangan tautan pattern_id tetapi tetap menyimpan nama polanya).
	for id := range existing {
		if !keep[id] {
			if _, err := tx.Exec(`DELETE FROM stock_breakdown_patterns WHERE id = $1`, id); err != nil {
				return nil, err
			}
		}
	}
	// 2. Nama sementara untuk pola yang dipertahankan, supaya saling tukar nama
	//    tidak menabrak indeks unik (bahan, nama) di tengah penyimpanan.
	for id := range keep {
		if _, err := tx.Exec(`UPDATE stock_breakdown_patterns SET name = '~' || id WHERE id = $1`, id); err != nil {
			return nil, err
		}
	}
	// 3. Simpan pola dan ganti baris hasilnya.
	for _, p := range req.Patterns {
		active := p.IsActive == nil || *p.IsActive
		id := p.ID
		if id == "" {
			id = NewULID()
			if _, err := tx.Exec(`
				INSERT INTO stock_breakdown_patterns (id, input_item_id, name, notes, is_active, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, $6)`, id, inputItemID, p.Name, p.Notes, active, actor); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.Exec(`
				UPDATE stock_breakdown_patterns
				SET name = $2, notes = $3, is_active = $4, updated_by = $5, updated_at = (now() AT TIME ZONE 'UTC')
				WHERE id = $1`, id, p.Name, p.Notes, active, actor); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(`DELETE FROM stock_item_breakdowns WHERE pattern_id = $1`, id); err != nil {
				return nil, err
			}
		}
		for i, l := range p.Outputs {
			var exists bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM stock_items WHERE id = $1)`, l.OutputItemID).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				return nil, Invalid("pola \"%s\": item hasil baris %d tidak ditemukan", p.Name, i+1)
			}
			weight := 1.0
			if l.CostWeight != nil {
				weight = *l.CostWeight
			}
			if _, err := tx.Exec(`
				INSERT INTO stock_item_breakdowns (id, pattern_id, input_item_id, output_item_id, qty_per_input, cost_weight, notes, created_by, updated_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
				NewULID(), id, inputItemID, l.OutputItemID, l.QtyPerInput, weight, strings.TrimSpace(l.Notes), actor); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockItemBreakdown(inputItemID)
}

// pickBreakdownPattern: pola yang dipakai sebuah pemotongan. patternID wajib
// bila bahan punya lebih dari satu pola aktif; kosong = satu-satunya pola aktif.
func pickBreakdownPattern(t *models.StockItemBreakdownTemplate, patternID string) (*models.StockBreakdownPattern, error) {
	if patternID != "" {
		for i := range t.Patterns {
			p := &t.Patterns[i]
			if p.ID != patternID {
				continue
			}
			if !p.IsActive {
				return nil, Invalid("pola pemotongan \"%s\" sedang nonaktif", p.Name)
			}
			if len(p.Outputs) == 0 {
				return nil, Invalid("pola pemotongan \"%s\" belum punya hasil", p.Name)
			}
			return p, nil
		}
		return nil, Invalid("pola pemotongan tidak ditemukan untuk %s", t.InputItemName)
	}
	var active []*models.StockBreakdownPattern
	for i := range t.Patterns {
		if t.Patterns[i].IsActive && len(t.Patterns[i].Outputs) > 0 {
			active = append(active, &t.Patterns[i])
		}
	}
	switch len(active) {
	case 0:
		return nil, Invalid("%s belum punya pola pemotongan aktif — atur dulu di Item Stok › Atur Pemotongan (Gudang Induk)", t.InputItemName)
	case 1:
		return active[0], nil
	}
	return nil, Invalid("%s punya %d pola pemotongan aktif — pilih pola yang dipakai", t.InputItemName, len(active))
}

// ── Pembagian biaya ───────────────────────────────────────────

// breakdownShare: satu hasil dalam pembagian biaya pemotongan.
type breakdownShare struct {
	qty         float64 // hasil aktual, satuan dasar hasil
	qtyPerInput float64 // hasil standar per 1 satuan distribusi bahan
	weight      float64 // bobot biaya
}

// allocateBreakdownCost membagi biaya bahan ke hasil sebanding
// qty × (bobot ÷ hasil standar). bobot÷standar = nilai relatif per satuan
// hasil, jadi pada hasil standar porsi biaya tiap baris = bobotnya. Bila hasil
// aktual kurang dari standar, biaya bahan yang sama terbagi ke qty yang lebih
// sedikit → HPP per satuan semua hasil naik: susut ikut terbebankan ke hasil,
// tidak hilang dari pembukuan (hasil di atas standar sebaliknya menurunkannya).
// Total per baris dibulatkan ke sen; sisa pembulatan masuk baris bernilai
// terbesar sehingga Σ total = biaya bahan.
func allocateBreakdownCost(total float64, lines []breakdownShare) (perUnit, totals []float64) {
	perUnit = make([]float64, len(lines))
	totals = make([]float64, len(lines))
	sum := 0.0
	for _, l := range lines {
		if l.qty > 0 && l.qtyPerInput > 0 {
			sum += l.qty * l.weight / l.qtyPerInput
		}
	}
	total = round2(total)
	if sum <= 0 || total <= 0 {
		return perUnit, totals
	}
	big, acc := -1, 0.0
	for i, l := range lines {
		if l.qty <= 0 || l.qtyPerInput <= 0 {
			continue
		}
		totals[i] = round2(total * (l.qty * l.weight / l.qtyPerInput) / sum)
		acc += totals[i]
		if big < 0 || totals[i] > totals[big] {
			big = i
		}
	}
	totals[big] = round2(totals[big] + total - acc)
	for i, l := range lines {
		if l.qty > 0 {
			perUnit[i] = totals[i] / l.qty
		}
	}
	return perUnit, totals
}

// ── Dokumen pemotongan ────────────────────────────────────────

func generateBreakdownNumber(tx *sql.Tx) (string, error) {
	prefix := "POT-" + time.Now().In(GetTimezoneLocation()).Format("20060102") + "-"
	seq, err := nextDocNumber(tx, "stock_breakdowns", "breakdown_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// CreateStockBreakdown mencatat satu pemotongan dalam SATU transaksi: kunci &
// cek stok bahan → bahan keluar FIFO (breakdown_out) → biaya FIFO aktualnya
// dibagi ke hasil pola terpilih (allocateBreakdownCost) → tiap hasil masuk
// dengan HPP-nya sendiri (breakdown_in) dan mewarisi kedaluwarsa terdekat
// batch bahan yang terpotong → dokumen + baris hasil (aktual, standar, HPP).
// Cakupan gudang diputus handler (role per outlet hanya gudang outletnya).
func CreateStockBreakdown(req models.StockBreakdownRequest, actor string) (*models.StockBreakdown, error) {
	req.WarehouseID = strings.TrimSpace(req.WarehouseID)
	req.InputItemID = strings.TrimSpace(req.InputItemID)
	if req.WarehouseID == "" {
		return nil, Invalid("gudang wajib dipilih")
	}
	if req.InputItemID == "" {
		return nil, Invalid("bahan yang dipotong wajib dipilih")
	}
	if req.InputQtyDist <= 0 {
		return nil, Invalid("jumlah bahan yang dipakai harus lebih dari 0")
	}

	var whName string
	var whActive bool
	err := database.DB.QueryRow(`SELECT name, is_active FROM warehouses WHERE id = $1`, req.WarehouseID).Scan(&whName, &whActive)
	if err == sql.ErrNoRows {
		return nil, Invalid("gudang tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if !whActive {
		return nil, Invalid("%s tidak aktif", whName)
	}

	tmpl, err := GetStockItemBreakdown(req.InputItemID)
	if err != nil {
		return nil, err
	}
	pattern, err := pickBreakdownPattern(tmpl, strings.TrimSpace(req.PatternID))
	if err != nil {
		return nil, err
	}
	// Hanya hasil pola terpilih yang dihitung — hasil pola lain tidak ikut
	// sebagai baris 0 / susut.
	lines := pattern.Outputs
	inPattern := map[string]int{}
	for i, o := range lines {
		inPattern[o.OutputItemID] = i
	}

	// Hasil aktual per baris pola; baris yang tidak dikirim = 0 tetapi tetap
	// tercatat di dokumen supaya susutnya terlihat (yield 0%).
	actual := make([]float64, len(lines))
	sent := map[string]bool{}
	anyPositive := false
	for _, o := range req.Outputs {
		o.ItemID = strings.TrimSpace(o.ItemID)
		idx, ok := inPattern[o.ItemID]
		if !ok {
			name := o.ItemID
			if it, err := GetStockItem(o.ItemID); err == nil {
				name = it.Name
			}
			return nil, Invalid("hasil \"%s\" tidak ada di pola pemotongan \"%s\" %s", name, pattern.Name, tmpl.InputItemName)
		}
		if sent[o.ItemID] {
			return nil, Invalid("hasil %s dikirim lebih dari sekali", lines[idx].OutputItemName)
		}
		sent[o.ItemID] = true
		if o.QtyBase < 0 {
			return nil, Invalid("hasil %s tidak boleh negatif", lines[idx].OutputItemName)
		}
		actual[idx] = o.QtyBase
		if o.QtyBase > 0 {
			anyPositive = true
		}
	}
	if !anyPositive {
		return nil, Invalid("isi minimal satu hasil potongan yang lebih dari 0")
	}

	qtyBase := req.InputQtyDist * tmpl.InputDistRatio
	distLabel := breakdownLabelUnit(tmpl.InputDistUnitLabel, tmpl.InputDistUnit)

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id := NewULID()
	number, err := generateBreakdownNumber(tx)
	if err != nil {
		return nil, err
	}

	// Kunci & cek stok bahan (FOR UPDATE: cegah dua pemotongan/penjualan
	// bersamaan menghabiskan stok yang sama). Belum pernah ada stok = 0.
	var available float64
	err = tx.QueryRow(`SELECT qty_base FROM stock_ledger WHERE item_id = $1 AND warehouse_id = $2 FOR UPDATE`,
		req.InputItemID, req.WarehouseID).Scan(&available)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if available+1e-9 < qtyBase {
		return nil, Invalid("stok %s di %s tidak mencukupi (butuh %s %s, ada %s %s)",
			tmpl.InputItemName, whName, fmtQty(qtyBase), tmpl.InputBaseUnit, fmtQty(available), tmpl.InputBaseUnit)
	}

	outNames := make([]string, 0, len(lines))
	for i, o := range lines {
		if actual[i] > 0 {
			outNames = append(outNames, o.OutputItemName)
		}
	}
	userNotes := strings.TrimSpace(req.Notes)
	withNotes := func(s string) string {
		if userNotes == "" {
			return s
		}
		return s + " — " + userNotes
	}

	// Bahan keluar: biaya FIFO yang benar-benar terpotong menjadi biaya hasil,
	// kedaluwarsa terdekat batch yang terpotong diwarisi batch hasil.
	consumed, err := applyMovementValued(tx, req.InputItemID, req.WarehouseID, "breakdown_out", tmpl.InputBaseUnit,
		id, "stock_breakdown", number, withNotes(fmt.Sprintf("Dipotong (%s) menjadi %s", pattern.Name, strings.Join(outNames, ", "))), actor,
		-qtyBase, -req.InputQtyDist, 0, "")
	if err != nil {
		return nil, fmt.Errorf("gagal mengurangi stok %s: %w", tmpl.InputItemName, err)
	}

	shares := make([]breakdownShare, len(lines))
	for i, o := range lines {
		shares[i] = breakdownShare{qty: actual[i], qtyPerInput: o.QtyPerInput, weight: o.CostWeight}
	}
	perUnit, totals := allocateBreakdownCost(consumed.value, shares)

	var expiry interface{}
	if consumed.earliestExpiry != "" {
		expiry = consumed.earliestExpiry
	}
	if _, err := tx.Exec(`
		INSERT INTO stock_breakdowns (id, breakdown_number, warehouse_id, input_item_id, pattern_id, pattern_name,
			input_qty_dist, input_qty_base, input_unit, input_cost_total, expiry_date, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		id, number, req.WarehouseID, req.InputItemID, pattern.ID, pattern.Name,
		req.InputQtyDist, qtyBase, distLabel, round2(consumed.value), expiry, userNotes, actor); err != nil {
		return nil, err
	}

	for i, o := range lines {
		if actual[i] > 0 {
			var ratio float64
			if err := tx.QueryRow(`SELECT COALESCE(NULLIF(dist_ratio, 0), 1) FROM stock_items WHERE id = $1`, o.OutputItemID).Scan(&ratio); err != nil {
				return nil, err
			}
			if err := applyMovement(tx, o.OutputItemID, req.WarehouseID, "breakdown_in", o.OutputBaseUnit,
				id, "stock_breakdown", number, withNotes(fmt.Sprintf("Hasil pemotongan %s (%s)", tmpl.InputItemName, pattern.Name)), actor,
				actual[i], actual[i]/ratio, perUnit[i], consumed.earliestExpiry); err != nil {
				return nil, fmt.Errorf("gagal menambah stok %s: %w", o.OutputItemName, err)
			}
		}
		if _, err := tx.Exec(`
			INSERT INTO stock_breakdown_outputs (id, breakdown_id, item_id, qty_base, std_qty_base,
				qty_per_input, cost_weight, cost_per_base, total_cost)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			NewULID(), id, o.OutputItemID, actual[i], req.InputQtyDist*o.QtyPerInput,
			o.QtyPerInput, o.CostWeight, round4(perUnit[i]), totals[i]); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockBreakdown(id)
}

// fmtQty: angka qty untuk pesan galat — tanpa nol berlebih (4000, 2,5).
func fmtQty(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}

const breakdownHeaderSelect = `
	SELECT sb.id, sb.breakdown_number, sb.warehouse_id, w.name, w.type,
	       sb.input_item_id, si.code, si.name, si.base_unit, sb.input_unit,
	       COALESCE(sb.pattern_id::text, ''), sb.pattern_name, COALESCE(TO_CHAR(sb.expiry_date, 'YYYY-MM-DD'), ''),
	       sb.input_qty_dist, sb.input_qty_base, sb.input_cost_total, sb.notes, sb.created_by,
	       TO_CHAR(sb.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
	FROM stock_breakdowns sb
	JOIN warehouses w ON w.id = sb.warehouse_id
	JOIN stock_items si ON si.id = sb.input_item_id`

func scanBreakdownHeader(sc interface{ Scan(...interface{}) error }) (models.StockBreakdown, error) {
	var b models.StockBreakdown
	err := sc.Scan(&b.ID, &b.BreakdownNumber, &b.WarehouseID, &b.WarehouseName, &b.WarehouseType,
		&b.InputItemID, &b.InputItemCode, &b.InputItemName, &b.InputBaseUnit, &b.InputUnit,
		&b.PatternID, &b.PatternName, &b.ExpiryDate,
		&b.InputQtyDist, &b.InputQtyBase, &b.InputCostTotal, &b.Notes, &b.CreatedBy, &b.CreatedAt)
	b.PatternID = strings.TrimSpace(b.PatternID)
	b.Outputs = []models.StockBreakdownOutput{}
	if b.InputQtyBase > 0 {
		b.InputCostPerBase = b.InputCostTotal / b.InputQtyBase
	}
	return b, err
}

// fillBreakdownOutputs memuat baris hasil dokumen-dokumen sekaligus lalu
// menghitung yield per baris & per dokumen (aktual vs standar pola).
func fillBreakdownOutputs(docs []models.StockBreakdown) error {
	if len(docs) == 0 {
		return nil
	}
	ids := make([]string, len(docs))
	pos := map[string]int{}
	for i, d := range docs {
		ids[i] = d.ID
		pos[d.ID] = i
	}
	rows, err := database.DB.Query(`
		SELECT o.breakdown_id, o.id, o.item_id, si.code, si.name, si.base_unit,
		       o.qty_base, o.std_qty_base, o.qty_per_input, o.cost_weight, o.cost_per_base, o.total_cost
		FROM stock_breakdown_outputs o
		JOIN stock_items si ON si.id = o.item_id
		WHERE o.breakdown_id = ANY($1)
		ORDER BY si.name`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var docID string
		var o models.StockBreakdownOutput
		if err := rows.Scan(&docID, &o.ID, &o.ItemID, &o.ItemCode, &o.ItemName, &o.BaseUnit,
			&o.QtyBase, &o.StdQtyBase, &o.QtyPerInput, &o.CostWeight, &o.CostPerBase, &o.TotalCost); err != nil {
			return err
		}
		if o.StdQtyBase > 0 {
			o.YieldPct = round2(o.QtyBase / o.StdQtyBase * 100)
		}
		d := &docs[pos[docID]]
		d.Outputs = append(d.Outputs, o)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Yield dokumen ditimbang bobot biaya: Σ aktual×bobot/standar ÷ Σ standar×bobot/standar.
	// Susut biaya = biaya bahan × (1 − yield) — porsi biaya yang tidak menjadi hasil.
	for i := range docs {
		var act, std float64
		for _, o := range docs[i].Outputs {
			if o.QtyPerInput > 0 {
				act += o.QtyBase * o.CostWeight / o.QtyPerInput
				std += o.StdQtyBase * o.CostWeight / o.QtyPerInput
			}
		}
		if std > 0 {
			docs[i].YieldPct = round2(act / std * 100)
			docs[i].ShrinkCost = round2(docs[i].InputCostTotal * (1 - act/std))
		}
	}
	return nil
}

// GetStockBreakdown: satu dokumen pemotongan beserta baris hasilnya.
func GetStockBreakdown(id string) (*models.StockBreakdown, error) {
	b, err := scanBreakdownHeader(database.DB.QueryRow(breakdownHeaderSelect+` WHERE sb.id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, Invalid("dokumen pemotongan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	docs := []models.StockBreakdown{b}
	if err := fillBreakdownOutputs(docs); err != nil {
		return nil, err
	}
	return &docs[0], nil
}

// ListStockBreakdowns: riwayat dokumen pemotongan, terbaru dulu. itemID
// mencocokkan bahan maupun hasil. outletScope != nil → hanya gudang OUTLET
// dalam cakupan (pemotongan di Gudang Induk milik role semua outlet).
func ListStockBreakdowns(warehouseID, itemID string, outletScope []string, page, limit int) ([]models.StockBreakdown, int, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	if warehouseID != "" {
		args = append(args, warehouseID)
		where += fmt.Sprintf(" AND sb.warehouse_id = $%d", len(args))
	}
	if itemID != "" {
		args = append(args, itemID)
		where += fmt.Sprintf(` AND (sb.input_item_id = $%d OR EXISTS (
			SELECT 1 FROM stock_breakdown_outputs o WHERE o.breakdown_id = sb.id AND o.item_id = $%d))`, len(args), len(args))
	}
	if outletScope != nil {
		args = append(args, pq.Array(outletScope))
		where += fmt.Sprintf(" AND sb.warehouse_id IN (SELECT id FROM warehouses WHERE type = 'outlet' AND outlet_id = ANY($%d))", len(args))
	}
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_breakdowns sb`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := database.DB.Query(breakdownHeaderSelect+where+
		fmt.Sprintf(" ORDER BY sb.created_at DESC, sb.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	docs := []models.StockBreakdown{}
	for rows.Next() {
		b, err := scanBreakdownHeader(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		docs = append(docs, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := fillBreakdownOutputs(docs); err != nil {
		return nil, 0, err
	}
	return docs, total, nil
}
