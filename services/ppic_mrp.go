package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"
)

func generateMrpNumber() string {
	now := time.Now()
	prefix := "MRP-" + now.Format("20060102") + "-"
	var seq int
	database.DB.QueryRow(`SELECT COUNT(*)+1 FROM mrp_runs WHERE run_number LIKE $1`, prefix+"%").Scan(&seq)
	return fmt.Sprintf("%s%03d", prefix, seq)
}

// RunMrp menghitung kebutuhan bahan untuk satu gudang tujuan dan menyimpannya
// sebagai snapshot (mrp_runs + mrp_run_items) supaya bisa diaudit.
//
// Alur: forecast (manual > sistem) horizon N hari → diledakkan lewat resep
// (produk single/recipe master/product_recipes, lalu level 2 resep barang
// setengah jadi) → kebutuhan kotor per bahan → dikurangi tersedia (on-hand +
// on-order PR + transfer in-transit) + buffer (par bila diaktifkan, selain itu
// safety stock) → kebutuhan bersih → saran: produksi (punya resep internal),
// transfer (gudang pusat cukup), atau beli (dibulatkan MOQ & satuan beli).
func RunMrp(req models.MrpRunRequest, actor string, outletScope []string) (*models.MrpRun, error) {
	if req.WarehouseID == "" {
		return nil, fmt.Errorf("warehouse_id wajib diisi")
	}
	horizon := req.HorizonDays
	if horizon <= 0 {
		horizon = 7
	}
	if horizon > 28 {
		horizon = 28
	}

	var whType, whName string
	var whOutletID sql.NullString
	if err := database.DB.QueryRow(`SELECT type, name, outlet_id FROM warehouses WHERE id = $1 AND is_active = true`,
		req.WarehouseID).Scan(&whType, &whName, &whOutletID); err != nil {
		return nil, fmt.Errorf("gudang tidak ditemukan")
	}

	// ── 1. Outlet sumber permintaan ──────────────────────────
	// Gudang outlet → forecast outlet itu; gudang pusat → agregat semua outlet
	// (dalam scope) karena pusat memasok semuanya.
	var outletIDs []string
	if whType == "outlet" && whOutletID.Valid {
		outletIDs = []string{strings.TrimSpace(whOutletID.String)}
	} else {
		q := `SELECT id FROM outlets WHERE is_active = true`
		var args []interface{}
		if outletScope != nil {
			q += ` AND id = ANY($1)`
			args = append(args, pq.Array(outletScope))
		}
		rows, err := database.DB.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				outletIDs = append(outletIDs, strings.TrimSpace(id))
			}
		}
		rows.Close()
	}
	if len(outletIDs) == 0 {
		return nil, fmt.Errorf("tidak ada outlet sumber permintaan")
	}

	// ── 2. Forecast per (outlet, produk) pada horizon ────────
	type prodKey struct{ outlet, name string }
	forecast := map[prodKey]float64{}
	var totalForecast float64
	fRows, err := database.DB.Query(fmt.Sprintf(`
		SELECT df.outlet_id, df.product_name, SUM(%s)
		FROM demand_forecasts df
		WHERE df.outlet_id = ANY($1) AND df.forecast_date >= CURRENT_DATE
		AND df.forecast_date < CURRENT_DATE + $2
		GROUP BY df.outlet_id, df.product_name
		HAVING SUM(%s) > 0`, forecastQtyExpr, forecastQtyExpr), pq.Array(outletIDs), horizon)
	if err != nil {
		return nil, err
	}
	for fRows.Next() {
		var k prodKey
		var qty float64
		if fRows.Scan(&k.outlet, &k.name, &qty) == nil {
			k.outlet = strings.TrimSpace(k.outlet)
			forecast[k] = qty
			totalForecast += qty
		}
	}
	fRows.Close()

	// ── 3. Resolusi produk → bahan (pola DeductStockByRecipe) ──
	type prodInfo struct {
		id, stockType, linkedItem, recipeMaster string
	}
	prodMap := map[prodKey]prodInfo{}
	pRows, err := database.DB.Query(`
		SELECT outlet_id, name, id, COALESCE(stock_type, ''), COALESCE(linked_stock_item_id, ''), COALESCE(recipe_master_id, '')
		FROM cloud_products
		WHERE outlet_id = ANY($1) AND COALESCE(is_deleted, false) = false`, pq.Array(outletIDs))
	if err != nil {
		return nil, err
	}
	for pRows.Next() {
		var k prodKey
		var pi prodInfo
		if pRows.Scan(&k.outlet, &k.name, &pi.id, &pi.stockType, &pi.linkedItem, &pi.recipeMaster) == nil {
			k.outlet = strings.TrimSpace(k.outlet)
			prodMap[k] = pi
		}
	}
	pRows.Close()

	// Muat resep yang dibutuhkan sekali jalan.
	masterIDs := []string{}
	legacyIDs := []string{}
	for k := range forecast {
		if pi, ok := prodMap[k]; ok {
			if pi.recipeMaster != "" {
				masterIDs = append(masterIDs, pi.recipeMaster)
			} else if pi.stockType == "recipe" {
				legacyIDs = append(legacyIDs, pi.id)
			}
		}
	}
	type mat struct {
		itemID  string
		qtyBase float64
	}
	masterRecipes := map[string][]mat{}
	if len(masterIDs) > 0 {
		rows, err := database.DB.Query(`SELECT recipe_master_id, item_id, qty_base FROM recipe_items WHERE recipe_master_id = ANY($1)`, pq.Array(masterIDs))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var mid string
			var m mat
			if rows.Scan(&mid, &m.itemID, &m.qtyBase) == nil {
				masterRecipes[mid] = append(masterRecipes[mid], m)
			}
		}
		rows.Close()
	}
	legacyRecipes := map[string][]mat{}
	if len(legacyIDs) > 0 {
		rows, err := database.DB.Query(`SELECT product_id, item_id, qty_base FROM product_recipes WHERE product_id = ANY($1)`, pq.Array(legacyIDs))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var pid string
			var m mat
			if rows.Scan(&pid, &m.itemID, &m.qtyBase) == nil {
				legacyRecipes[pid] = append(legacyRecipes[pid], m)
			}
		}
		rows.Close()
	}

	// Ledakan level 1: kebutuhan kotor per bahan (satuan dasar).
	gross := map[string]float64{}
	noRecipe := []string{}
	for k, qty := range forecast {
		pi, ok := prodMap[k]
		if !ok {
			noRecipe = append(noRecipe, k.name)
			continue
		}
		switch {
		case pi.stockType == "single" && pi.linkedItem != "":
			gross[strings.TrimSpace(pi.linkedItem)] += qty
		case pi.recipeMaster != "" && len(masterRecipes[pi.recipeMaster]) > 0:
			for _, m := range masterRecipes[pi.recipeMaster] {
				gross[m.itemID] += m.qtyBase * qty
			}
		case len(legacyRecipes[pi.id]) > 0:
			for _, m := range legacyRecipes[pi.id] {
				gross[m.itemID] += m.qtyBase * qty
			}
		default:
			noRecipe = append(noRecipe, k.name)
		}
	}

	// ── 4. Ketersediaan + level 2 (resep barang setengah jadi) ──
	itemIDs := keys(gross)
	avail, err := mrpAvailability(itemIDs, req.WarehouseID, whType, whOutletID)
	if err != nil {
		return nil, err
	}

	// Bahan yang punya resep internal: kekurangannya diledakkan ke bahan anak,
	// bahan itu sendiri disarankan PRODUKSI (bukan beli).
	producible := map[string][]mat{}
	if len(itemIDs) > 0 {
		rows, err := database.DB.Query(`SELECT parent_item_id, child_item_id, qty_base FROM stock_item_recipes WHERE parent_item_id = ANY($1)`, pq.Array(itemIDs))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var pid string
			var m mat
			if rows.Scan(&pid, &m.itemID, &m.qtyBase) == nil {
				producible[pid] = append(producible[pid], m)
			}
		}
		rows.Close()
	}
	for parent, children := range producible {
		a := avail[parent]
		shortfall := gross[parent] + a.safety - (a.onHand + a.onOrder + a.inTransit)
		if shortfall <= 0 {
			continue
		}
		for _, c := range children {
			gross[c.itemID] += c.qtyBase * shortfall
		}
	}

	// ── 5. Set item final (+ item par bila diminta) ──────────
	if req.IncludePar {
		rows, err := database.DB.Query(`SELECT item_id FROM item_planning_params WHERE warehouse_id = $1 AND par_level > 0`, req.WarehouseID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				id = strings.TrimSpace(id)
				if _, ok := gross[id]; !ok {
					gross[id] = 0
				}
			}
		}
		rows.Close()
	}
	itemIDs = keys(gross)
	if len(itemIDs) == 0 {
		return nil, fmt.Errorf("tidak ada kebutuhan bahan — forecast kosong atau resep belum terisi (generate forecast dulu di halaman Demand Forecast)")
	}
	avail, err = mrpAvailability(itemIDs, req.WarehouseID, whType, whOutletID)
	if err != nil {
		return nil, err
	}
	info, err := mrpItemInfo(itemIDs)
	if err != nil {
		return nil, err
	}
	central, err := mrpCentralStock(itemIDs, req.WarehouseID)
	if err != nil {
		return nil, err
	}
	lastBuy, err := mrpLastPurchase(itemIDs)
	if err != nil {
		return nil, err
	}

	// ── 6. Kebutuhan bersih + saran, simpan snapshot ─────────
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	runID := NewULID()
	runNumber := generateMrpNumber()
	noRecipeSample := strings.Join(uniqueHead(noRecipe, 12), ", ")
	if _, err := tx.Exec(`
		INSERT INTO mrp_runs (id, run_number, warehouse_id, horizon_days, include_par, status, no_recipe_count, no_recipe_products, forecast_qty, created_by)
		VALUES ($1, $2, $3, $4, $5, 'open', $6, $7, $8, $9)`,
		runID, runNumber, req.WarehouseID, horizon, req.IncludePar, len(uniqueHead(noRecipe, 100000)), noRecipeSample, totalForecast, actor); err != nil {
		return nil, err
	}

	const eps = 0.0001
	for _, itemID := range itemIDs {
		a := avail[itemID]
		inf, ok := info[itemID]
		if !ok {
			continue // item terhapus/nonaktif
		}
		buffer := a.safety
		if req.IncludePar && a.par > 0 {
			buffer = a.par
		}
		net := gross[itemID] + buffer - (a.onHand + a.onOrder + a.inTransit)
		if net < 0 {
			net = 0
		}

		suggestion := "none"
		qtyBase := 0.0
		sourceWh := ""
		if net > eps {
			cs := central[itemID]
			switch {
			case len(producible[itemID]) > 0:
				suggestion = "produce"
				qtyBase = net
			case whType == "outlet" && cs.qty >= net:
				suggestion = "transfer"
				qtyBase = math.Ceil(net*100) / 100
				sourceWh = cs.warehouseID
			default:
				suggestion = "purchase"
				ratio := inf.distRatio
				if ratio <= 0 {
					ratio = 1
				}
				qtyDist := math.Ceil(net / ratio)
				if a.moq > 0 && qtyDist < a.moq {
					qtyDist = math.Ceil(a.moq)
				}
				qtyBase = qtyDist * ratio
			}
		}

		var srcArg interface{}
		if sourceWh != "" {
			srcArg = sourceWh
		}
		lb := lastBuy[itemID]
		if _, err := tx.Exec(`
			INSERT INTO mrp_run_items (id, run_id, item_id, gross_req, on_hand, on_order, in_transit, safety_stock, par_level, moq, net_req, suggestion, qty_suggested_base, source_warehouse_id, last_vendor, last_price_dist)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			NewULID(), runID, itemID, round4(gross[itemID]), a.onHand, a.onOrder, a.inTransit,
			a.safety, a.par, a.moq, round4(net), suggestion, round4(qtyBase), srcArg, lb.vendor, lb.priceDist); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetMrpRun(runID)
}

type mrpAvail struct {
	onHand, onOrder, inTransit, safety, par, moq float64
}

// mrpAvailability memuat on-hand, on-order (PR aktif — dicocokkan per NAMA sub
// item karena PR bersifat teks bebas; estimasi, satuan dianggap satuan beli),
// in-transit (transfer 'sent' menuju gudang), dan parameter perencanaan.
func mrpAvailability(itemIDs []string, warehouseID, whType string, whOutletID sql.NullString) (map[string]mrpAvail, error) {
	res := map[string]mrpAvail{}
	if len(itemIDs) == 0 {
		return res, nil
	}

	rows, err := database.DB.Query(`
		SELECT si.id, COALESCE(sl.qty_base, 0),
			COALESCE(pp.safety_stock, 0), COALESCE(pp.par_level, 0), COALESCE(pp.moq, 0), si.dist_ratio, LOWER(TRIM(si.name))
		FROM stock_items si
		LEFT JOIN stock_ledger sl ON sl.item_id = si.id AND sl.warehouse_id = $2
		LEFT JOIN item_planning_params pp ON pp.item_id = si.id AND pp.warehouse_id = $2
		WHERE si.id = ANY($1)`, pq.Array(itemIDs), warehouseID)
	if err != nil {
		return nil, err
	}
	nameToID := map[string]string{}
	ratios := map[string]float64{}
	for rows.Next() {
		var id, lname string
		var a mrpAvail
		var ratio float64
		if rows.Scan(&id, &a.onHand, &a.safety, &a.par, &a.moq, &ratio, &lname) == nil {
			id = strings.TrimSpace(id)
			res[id] = a
			nameToID[lname] = id
			ratios[id] = ratio
		}
	}
	rows.Close()

	// In-transit: transfer berstatus 'sent' menuju gudang ini.
	tRows, err := database.DB.Query(`
		SELECT sti.item_id, COALESCE(SUM(sti.qty_base-sti.accepted_qty_base-sti.rejected_qty_base), 0)
		FROM stock_transfer_items sti
		JOIN stock_transfers st ON st.id = sti.transfer_id
		WHERE st.status IN ('sent','partial_received') AND st.to_warehouse_id = $2 AND sti.item_id = ANY($1)
		GROUP BY sti.item_id`, pq.Array(itemIDs), warehouseID)
	if err != nil {
		return nil, err
	}
	for tRows.Next() {
		var id string
		var qty float64
		if tRows.Scan(&id, &qty) == nil {
			id = strings.TrimSpace(id)
			a := res[id]
			a.inTransit = qty
			res[id] = a
		}
	}
	tRows.Close()

	// On-order: PR yang sudah DISETUJUI dan belum diterima, 60 hari terakhir.
	// Nama sub item dicocokkan ke nama item stok (estimasi). Daftar putih
	// status berkomitmen: pengajuan yang baru diverifikasi belum tentu
	// disetujui owner, jadi belum dihitung sebagai barang dalam pesanan (T-42).
	prCond := prCommittedCond("pr") + ` AND pr.received_at IS NULL
		AND pr.created_at >= NOW() - INTERVAL '60 days'`
	args := []interface{}{}
	if whType == "outlet" && whOutletID.Valid {
		prCond += ` AND pr.outlet_id = $1`
		args = append(args, strings.TrimSpace(whOutletID.String))
	}
	oRows, err := database.DB.Query(fmt.Sprintf(`
		SELECT LOWER(TRIM(sub->>'name')), SUM(COALESCE((sub->>'qty')::numeric, 0))
		FROM purchase_requests pr,
			jsonb_array_elements(COALESCE(pr.items, '[]'::jsonb)) grp,
			jsonb_array_elements(COALESCE(grp->'items', '[]'::jsonb)) sub
		WHERE %s
		GROUP BY 1`, prCond), args...)
	if err != nil {
		return nil, err
	}
	for oRows.Next() {
		var lname string
		var qtyDist float64
		if oRows.Scan(&lname, &qtyDist) == nil {
			if id, ok := nameToID[lname]; ok {
				a := res[id]
				ratio := ratios[id]
				if ratio <= 0 {
					ratio = 1
				}
				a.onOrder += qtyDist * ratio
				res[id] = a
			}
		}
	}
	oRows.Close()

	return res, nil
}

type mrpInfo struct{ distRatio float64 }

func mrpItemInfo(itemIDs []string) (map[string]mrpInfo, error) {
	res := map[string]mrpInfo{}
	rows, err := database.DB.Query(`SELECT id, dist_ratio FROM stock_items WHERE id = ANY($1) AND is_active = true`, pq.Array(itemIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var r float64
		if rows.Scan(&id, &r) == nil {
			res[strings.TrimSpace(id)] = mrpInfo{distRatio: r}
		}
	}
	return res, nil
}

type mrpCentral struct {
	warehouseID string
	qty         float64
}

// mrpCentralStock: gudang pusat dengan stok terbanyak per item (kandidat sumber transfer).
func mrpCentralStock(itemIDs []string, excludeWh string) (map[string]mrpCentral, error) {
	res := map[string]mrpCentral{}
	rows, err := database.DB.Query(`
		SELECT DISTINCT ON (sl.item_id) sl.item_id, w.id, sl.qty_base
		FROM stock_ledger sl
		JOIN warehouses w ON w.id = sl.warehouse_id AND w.type = 'central' AND w.is_active = true
		WHERE sl.item_id = ANY($1) AND sl.qty_base > 0 AND w.id <> $2
		ORDER BY sl.item_id, sl.qty_base DESC`, pq.Array(itemIDs), excludeWh)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var c mrpCentral
		if rows.Scan(&id, &c.warehouseID, &c.qty) == nil {
			res[strings.TrimSpace(id)] = c
		}
	}
	return res, nil
}

type mrpLastBuy struct {
	vendor    string
	priceDist float64
}

// mrpLastPurchase: vendor & harga terakhir per item dari penerimaan barang
// (goods_receipt_items ter-link item_id — sumber paling andal).
func mrpLastPurchase(itemIDs []string) (map[string]mrpLastBuy, error) {
	res := map[string]mrpLastBuy{}
	rows, err := database.DB.Query(`
		SELECT DISTINCT ON (gri.item_id) gri.item_id, COALESCE(gr.vendor_name, ''),
			gri.cost_per_base * si.dist_ratio
		FROM goods_receipt_items gri
		JOIN goods_receipts gr ON gr.id = gri.receipt_id
		JOIN stock_items si ON si.id = gri.item_id
		WHERE gri.item_id = ANY($1)
		ORDER BY gri.item_id, gr.received_at DESC`, pq.Array(itemIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var lb mrpLastBuy
		if rows.Scan(&id, &lb.vendor, &lb.priceDist) == nil {
			res[strings.TrimSpace(id)] = lb
		}
	}
	return res, nil
}

// ── Query run ─────────────────────────────────────────────────

func ListMrpRuns(outletIDs []string, page, limit int) ([]models.MrpRun, int, error) {
	scope, args := ppicWarehouseScope(outletIDs)
	base := ` FROM mrp_runs mr JOIN warehouses w ON w.id = mr.warehouse_id WHERE w.is_active = true` + scope

	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*)`+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT mr.id, mr.run_number, mr.warehouse_id, w.name, w.type, mr.horizon_days, mr.include_par,
			mr.status, mr.no_recipe_count, COALESCE(mr.no_recipe_products, ''), COALESCE(mr.notes, ''),
			COALESCE(mr.created_by, ''), mr.created_at::text, mr.forecast_qty
		%s ORDER BY mr.created_at DESC LIMIT %d OFFSET %d`, base, limit, (page-1)*limit), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []models.MrpRun{}
	for rows.Next() {
		var r models.MrpRun
		if err := rows.Scan(&r.ID, &r.RunNumber, &r.WarehouseID, &r.WarehouseName, &r.WarehouseType,
			&r.HorizonDays, &r.IncludePar, &r.Status, &r.NoRecipeCount, &r.NoRecipeProducts,
			&r.Notes, &r.CreatedBy, &r.CreatedAt, &r.ForecastQty); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, nil
}

func GetMrpRun(id string) (*models.MrpRun, error) {
	var r models.MrpRun
	err := database.DB.QueryRow(`
		SELECT mr.id, mr.run_number, mr.warehouse_id, w.name, w.type, mr.horizon_days, mr.include_par,
			mr.status, mr.no_recipe_count, COALESCE(mr.no_recipe_products, ''), COALESCE(mr.notes, ''),
			COALESCE(mr.created_by, ''), mr.created_at::text, mr.forecast_qty
		FROM mrp_runs mr JOIN warehouses w ON w.id = mr.warehouse_id
		WHERE mr.id = $1`, id).Scan(
		&r.ID, &r.RunNumber, &r.WarehouseID, &r.WarehouseName, &r.WarehouseType,
		&r.HorizonDays, &r.IncludePar, &r.Status, &r.NoRecipeCount, &r.NoRecipeProducts,
		&r.Notes, &r.CreatedBy, &r.CreatedAt, &r.ForecastQty)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run MRP tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.Query(`
		SELECT i.id, i.item_id, si.code, si.name, si.category, si.base_unit,
			si.dist_unit, si.dist_ratio, si.dist_unit_label,
			i.gross_req, i.on_hand, i.on_order, i.in_transit, i.safety_stock, i.par_level, i.moq,
			i.net_req, i.suggestion, i.qty_suggested_base,
			COALESCE(i.source_warehouse_id, ''), COALESCE(sw.name, ''),
			COALESCE(i.last_vendor, ''), i.last_price_dist,
			COALESCE(i.action_ref_type, ''), COALESCE(i.action_ref_id, ''), COALESCE(i.action_ref_number, '')
		FROM mrp_run_items i
		JOIN stock_items si ON si.id = i.item_id
		LEFT JOIN warehouses sw ON sw.id = i.source_warehouse_id
		WHERE i.run_id = $1
		ORDER BY CASE i.suggestion WHEN 'purchase' THEN 0 WHEN 'transfer' THEN 1 WHEN 'produce' THEN 2 ELSE 3 END, si.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	r.Items = []models.MrpRunItem{}
	for rows.Next() {
		var it models.MrpRunItem
		if err := rows.Scan(&it.ID, &it.ItemID, &it.ItemCode, &it.ItemName, &it.Category, &it.BaseUnit,
			&it.DistUnit, &it.DistRatio, &it.DistUnitLabel,
			&it.GrossReq, &it.OnHand, &it.OnOrder, &it.InTransit, &it.SafetyStock, &it.ParLevel, &it.Moq,
			&it.NetReq, &it.Suggestion, &it.QtySuggestedBase,
			&it.SourceWarehouse, &it.SourceWhName,
			&it.LastVendor, &it.LastPriceDist,
			&it.ActionRefType, &it.ActionRefID, &it.ActionRefNumber); err != nil {
			return nil, err
		}
		if it.DistRatio > 0 {
			it.QtySuggestedDist = round4(it.QtySuggestedBase / it.DistRatio)
		} else {
			it.QtySuggestedDist = it.QtySuggestedBase
		}
		r.Items = append(r.Items, it)
	}
	return &r, nil
}

func MrpRunInScope(id string, outletIDs []string) bool {
	var whID string
	if err := database.DB.QueryRow(`SELECT warehouse_id FROM mrp_runs WHERE id = $1`, id).Scan(&whID); err != nil {
		return false
	}
	return WarehouseMutableInScope(strings.TrimSpace(whID), outletIDs)
}

// ── Eksekusi: draft PR & draft transfer ───────────────────────

// CreateMrpPurchaseRequest membuat SATU draft PR (status awal alur pengadaan
// existing) dari baris saran 'purchase' yang dipilih. Vendor sengaja dikosongkan
// — diisi tim purchasing pada tahap "Isi Harga" seperti biasa.
func CreateMrpPurchaseRequest(runID string, req models.MrpExecuteRequest, actor Actor) (*models.PurchaseRequest, error) {
	run, err := GetMrpRun(runID)
	if err != nil {
		return nil, err
	}
	selected := pickMrpItems(run, req.ItemIDs, "purchase")
	if len(selected) == 0 {
		return nil, fmt.Errorf("tidak ada baris saran BELI yang dipilih (atau sudah dibuatkan PR)")
	}

	subs := make([]models.PurchaseSubItem, 0, len(selected))
	for _, it := range selected {
		unit := it.DistUnit
		if it.DistUnitLabel != "" {
			unit = it.DistUnitLabel
		}
		subs = append(subs, models.PurchaseSubItem{
			Name:     it.ItemName,
			Qty:      int(math.Ceil(it.QtySuggestedDist)),
			Unit:     unit,
			HpsPrice: it.LastPriceDist,
		})
	}

	var outletID string
	if run.WarehouseType == "outlet" {
		database.DB.QueryRow(`SELECT COALESCE(outlet_id, '') FROM warehouses WHERE id = $1`, run.WarehouseID).Scan(&outletID)
		outletID = strings.TrimSpace(outletID)
	}
	notes := fmt.Sprintf("Draft otomatis dari MRP %s (gudang %s, horizon %d hari).", run.RunNumber, run.WarehouseName, run.HorizonDays)
	if req.Notes != "" {
		notes += " " + req.Notes
	}
	// Kebutuhan bahan hasil MRP masuk Pengadaan Bahan Baku: diterima di
	// Gudang Induk dan diatur izin procurement.rawmaterials.*.
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		OutletID:      outletID,
		RequestType:   RequestTypeBahan,
		RequestedBy:   actor.Name,
		RequestedByID: actor.ID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Kebutuhan Bahan (MRP) " + run.RunNumber,
			Items: subs,
		}},
		Notes: notes,
	})
	if err != nil {
		return nil, err
	}

	database.DB.Exec(`UPDATE purchase_requests SET origin = 'mrp', mrp_run_id = $2, need_by_date = CURRENT_DATE + $3 WHERE id = $1`,
		pr.ID, runID, run.HorizonDays)
	markMrpItems(runID, selected, "purchase_request", pr.ID, pr.RequestNumber)
	return pr, nil
}

// CreateMrpTransfer membuat draft transfer stok per gudang sumber dari baris
// saran 'transfer' yang dipilih.
func CreateMrpTransfer(runID string, req models.MrpExecuteRequest, actor string) ([]models.StockTransfer, error) {
	run, err := GetMrpRun(runID)
	if err != nil {
		return nil, err
	}
	selected := pickMrpItems(run, req.ItemIDs, "transfer")
	if len(selected) == 0 {
		return nil, fmt.Errorf("tidak ada baris saran TRANSFER yang dipilih (atau sudah dibuatkan transfer)")
	}

	bySource := map[string][]models.MrpRunItem{}
	for _, it := range selected {
		if it.SourceWarehouse == "" {
			continue
		}
		bySource[it.SourceWarehouse] = append(bySource[it.SourceWarehouse], it)
	}

	created := []models.StockTransfer{}
	for source, items := range bySource {
		treq := models.StockTransferRequest{
			FromWarehouseID: source,
			ToWarehouseID:   run.WarehouseID,
			Notes:           fmt.Sprintf("Draft otomatis dari MRP %s. %s", run.RunNumber, req.Notes),
		}
		for _, it := range items {
			treq.Items = append(treq.Items, models.StockTransferItemReq{ItemID: it.ItemID, QtyDist: it.QtySuggestedDist})
		}
		st, err := CreateStockTransfer(treq, actor)
		if err != nil {
			return created, fmt.Errorf("transfer dari %s: %w", source, err)
		}
		database.DB.Exec(`UPDATE stock_transfers SET origin = 'mrp', mrp_run_id = $2 WHERE id = $1`, st.ID, runID)
		markMrpItems(runID, items, "stock_transfer", st.ID, st.TransferNumber)
		created = append(created, *st)
	}
	if len(created) == 0 {
		return nil, fmt.Errorf("baris transfer terpilih tidak punya gudang sumber")
	}
	return created, nil
}

func pickMrpItems(run *models.MrpRun, itemIDs []string, suggestion string) []models.MrpRunItem {
	want := map[string]bool{}
	for _, id := range itemIDs {
		want[id] = true
	}
	out := []models.MrpRunItem{}
	for _, it := range run.Items {
		if it.Suggestion != suggestion || it.ActionRefID != "" || it.QtySuggestedDist <= 0 {
			continue
		}
		if len(itemIDs) > 0 && !want[it.ItemID] {
			continue
		}
		out = append(out, it)
	}
	return out
}

func markMrpItems(runID string, items []models.MrpRunItem, refType, refID, refNumber string) {
	for _, it := range items {
		database.DB.Exec(`UPDATE mrp_run_items SET action_ref_type = $3, action_ref_id = $4, action_ref_number = $5 WHERE run_id = $1 AND item_id = $2`,
			runID, it.ItemID, refType, refID, refNumber)
	}
	database.DB.Exec(`UPDATE mrp_runs SET status = 'executed' WHERE id = $1`, runID)
}

// ── util kecil ────────────────────────────────────────────────

func keys(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func uniqueHead(list []string, n int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
			if len(out) >= n {
				break
			}
		}
	}
	return out
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
