package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ppicUsageStatuses = map[string]bool{"": true, "over_limit": true, "data_incomplete": true, "unset": true, "no_basis": true, "needs_count": true, "within_limit": true, "no_activity": true}

// Every quantity is a base-unit quantity for one item in one warehouse. In
// particular, automatic POS deductions are a recorded baseline, not an
// independent measurement of what the kitchen physically used.
const ppicUsageScopeCTE = `WITH wh AS (
 SELECT w.id,w.name,w.type,w.outlet_id,COALESCE(o.name,'Gudang Induk') AS outlet_name
 FROM warehouses w LEFT JOIN outlets o ON o.id=w.outlet_id
 WHERE w.is_active AND ($3='' OR w.id=$3) AND ($4 OR (w.type='outlet' AND w.outlet_id=ANY($5)))
)`

func usageFlag(r *models.PpicUsageControlRow, flag string) {
	for _, f := range r.Flags {
		if f == flag {
			return
		}
	}
	r.Flags = append(r.Flags, flag)
}

func usageHasFlag(r models.PpicUsageControlRow, flag string) bool {
	for _, f := range r.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

func finishPpicUsageRow(r *models.PpicUsageControlRow) {
	r.ReferenceQty = round4(r.SaleQty + r.ProductionReferenceQty + r.BreakdownQty)
	r.EvaluatedUsageQty = round4(r.SaleQty + r.ProductionQty + r.BreakdownQty + r.OtherUsageQty + r.StocktakeShortageQty + r.AdjustmentShortageQty)
	r.ExcessUsageQty = round4(math.Max(0, r.EvaluatedUsageQty-r.ReferenceQty))
	if r.ReferenceQty > 0 {
		variance := round4((r.EvaluatedUsageQty - r.ReferenceQty) / r.ReferenceQty * 100)
		waste := round4(r.WasteQty / r.ReferenceQty * 100)
		r.UsageVariancePct = &variance
		r.WastePct = &waste
		if r.Policy != nil {
			usageLimit := round4(r.ReferenceQty * (1 + r.Policy.UsageTolerancePct/100))
			wasteLimit := round4(r.ReferenceQty * r.Policy.WasteTolerancePct / 100)
			r.UsageLimitQty = &usageLimit
			r.WasteLimitQty = &wasteLimit
		}
	}
	if r.Policy == nil {
		usageFlag(r, "policy_unset")
	}
	if r.ReferenceQty <= 0 {
		usageFlag(r, "no_reference")
	}
	if !r.HasStocktake {
		usageFlag(r, "stocktake_missing")
	}
	if r.StocktakeShortageQty > 0 {
		usageFlag(r, "stocktake_shortage")
	}
	if r.StocktakeSurplusQty > 0 {
		usageFlag(r, "stocktake_surplus_not_offset")
	}
	if r.AdjustmentShortageQty > 0 {
		usageFlag(r, "manual_adjustment_shortage")
	}
	if r.VoidWasteQty > 0 {
		usageFlag(r, "void_waste_included")
	}
	usageOver := r.UsageLimitQty != nil && r.EvaluatedUsageQty > *r.UsageLimitQty+0.00005
	wasteOver := r.WasteLimitQty != nil && r.WasteQty > *r.WasteLimitQty+0.00005
	if usageOver {
		usageFlag(r, "usage_over_limit")
	}
	if wasteOver {
		usageFlag(r, "waste_over_limit")
	}
	incomplete := false
	for _, f := range []string{"sale_basis_missing", "pending_deductions", "sales_without_stock_basis", "production_snapshot_missing", "production_movement_missing", "breakdown_snapshot_missing", "waste_document_missing"} {
		incomplete = incomplete || usageHasFlag(*r, f)
	}
	switch {
	case usageOver || wasteOver:
		r.Status = "over_limit"
	case incomplete:
		r.Status = "data_incomplete"
	case r.Policy == nil:
		r.Status = "unset"
	case r.EvaluatedUsageQty == 0 && r.WasteQty == 0 && r.ReferenceQty == 0 && r.StocktakeSurplusQty == 0:
		r.Status = "no_activity"
	case r.ReferenceQty <= 0:
		r.Status = "no_basis"
	case !r.HasStocktake:
		r.Status = "needs_count"
	default:
		r.Status = "within_limit"
	}
}

// GetPpicUsageControl reads one consistent inventory snapshot. Dates select
// posted stock movements/waste events, not the original offline POS sale date.
// limit=0 is the unpaginated, identically filtered export path.
func GetPpicUsageControl(dateFrom, dateTo, warehouseID, search, status string, scope []string, page, limit int) (*models.PpicUsageControlReport, error) {
	from, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		return nil, Invalid("tanggal awal tidak valid")
	}
	to, err := time.Parse("2006-01-02", dateTo)
	if err != nil || to.Before(from) || to.Sub(from) > 365*24*time.Hour {
		return nil, Invalid("periode harus berurutan dan maksimal 366 hari")
	}
	if !ppicUsageStatuses[status] {
		return nil, Invalid("status kontrol pemakaian tidak dikenal")
	}
	rep := &models.PpicUsageControlReport{DateFrom: dateFrom, DateTo: dateTo, Rows: []models.PpicUsageControlRow{}, Notes: []string{
		"Acuan penjualan memakai snapshot mutasi bahan pada transaksi pembayaran; resep saat ini tidak menghitung ulang histori. Pengurangan otomatis ini bukan bukti pemakaian fisik dapur.",
		"Periode mengikuti waktu pencatatan mutasi stok dan kejadian waste. Sinkronisasi POS terlambat dapat masuk periode berbeda dari tanggal penjualan; void dapat mereklasifikasi mutasi lama.",
		"Batas memakai kebijakan PPIC yang berlaku saat laporan dibuka. Riwayat kebijakan tersimpan; batas lama tidak diterapkan otomatis pada periode lampau.",
		"Pemakaian dinilai = penjualan + produksi + pemotongan + pengeluaran lain + kekurangan opname + koreksi stok negatif. Transfer, karantina, retur dan waste tidak ditambahkan sebagai pemakaian.",
		"Waste memakai dokumen waste satu kali, termasuk void dan pemusnahan karantina. Mutasi waste tanpa dokumen tetap dihitung terpisah sebagai waste belum terdokumentasi. Persen waste memakai acuan pemakaian sebagai pembagi.",
		"Opname adalah bukti hitung yang telah disetujui dalam periode, bukan pemantauan fisik terus-menerus. Selisih memakai penyesuaian yang diposting saat approval dan dapat mencakup kejadian sebelum periode.",
		"Kelebihan stok tidak menutup kekurangan. Nilai ringkasan hanya waste dan koreksi negatif; biaya produksi dan penjualan tidak dijumlahkan karena dapat menghitung bahan yang sama dua kali.",
	}}
	if warehouseID != "" {
		if err := ValidatePpicUsageWarehouse(warehouseID, scope); err != nil {
			return nil, err
		}
	}
	if scope != nil && len(scope) == 0 {
		return rep, nil
	}
	tx, err := database.DB.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	args := []interface{}{dateFrom, dateTo, warehouseID, scope == nil, pq.Array(scope)}
	rows, err := tx.Query(ppicUsageScopeCTE+`, keys AS (
  SELECT l.item_id,l.warehouse_id FROM stock_ledger l JOIN wh ON wh.id=l.warehouse_id
  UNION SELECT p.item_id,p.warehouse_id FROM ppic_usage_policies p JOIN wh ON wh.id=p.warehouse_id
  UNION SELECT m.item_id,m.warehouse_id FROM stock_movements m JOIN wh ON wh.id=m.warehouse_id WHERE m.created_at>=tz_day_start($1::date) AND m.created_at<tz_day_start($2::date+1)
  UNION SELECT s.item_id,s.warehouse_id FROM stock_wastes s JOIN wh ON wh.id=s.warehouse_id WHERE s.created_at>=tz_day_start($1::date) AND s.created_at<tz_day_start($2::date+1)
  UNION SELECT i.item_id,o.warehouse_id FROM stock_opname_items i JOIN stock_opnames o ON o.id=i.opname_id JOIN wh ON wh.id=o.warehouse_id WHERE o.status='approved' AND i.qty_counted_base IS NOT NULL AND o.approved_at>=tz_day_start($1::date) AND o.approved_at<tz_day_start($2::date+1)
  UNION SELECT m.item_id,o.warehouse_id FROM work_order_materials m JOIN work_orders o ON o.id=m.wo_id JOIN wh ON wh.id=o.warehouse_id WHERE o.status='done' AND o.finished_at>=tz_day_start($1::date) AND o.finished_at<tz_day_start($2::date+1)
 ) SELECT i.id,i.code,i.name,COALESCE(i.category,''),i.base_unit,wh.id,wh.name,wh.type,wh.outlet_name,COALESCE(l.qty_base,0),
  COALESCE(p.version,0),COALESCE(p.usage_tolerance_pct,0),COALESCE(p.waste_tolerance_pct,0),COALESCE(p.reason,''),COALESCE(p.updated_by,''),p.updated_at,
  counted.approved_at,COALESCE(counted.opname_number,''),COALESCE(counted.approved_at>=tz_day_start($1::date),false)
 FROM keys k JOIN stock_items i ON i.id=k.item_id JOIN wh ON wh.id=k.warehouse_id
 LEFT JOIN stock_ledger l ON l.item_id=k.item_id AND l.warehouse_id=k.warehouse_id
 LEFT JOIN ppic_usage_policies p ON p.item_id=k.item_id AND p.warehouse_id=k.warehouse_id
 LEFT JOIN LATERAL (SELECT o.approved_at,o.opname_number FROM stock_opname_items oi JOIN stock_opnames o ON o.id=oi.opname_id WHERE oi.item_id=k.item_id AND o.warehouse_id=k.warehouse_id AND o.status='approved' AND oi.qty_counted_base IS NOT NULL AND o.approved_at<tz_day_start($2::date+1) ORDER BY o.approved_at DESC,o.id DESC LIMIT 1) counted ON true`, args...)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*models.PpicUsageControlRow{}
	key := func(item, warehouse string) string {
		return strings.TrimSpace(item) + ":" + strings.TrimSpace(warehouse)
	}
	for rows.Next() {
		r := &models.PpicUsageControlRow{Flags: []string{}}
		p := &models.PpicUsagePolicy{}
		var updated, counted sql.NullTime
		if err = rows.Scan(&r.ItemID, &r.ItemCode, &r.ItemName, &r.Category, &r.BaseUnit, &r.WarehouseID, &r.WarehouseName, &r.WarehouseType, &r.OutletName, &r.StockQty, &p.Version, &p.UsageTolerancePct, &p.WasteTolerancePct, &p.Reason, &p.UpdatedBy, &updated, &counted, &r.StocktakeNumber, &r.HasStocktake); err != nil {
			rows.Close()
			return nil, err
		}
		if updated.Valid {
			p.UpdatedAt = updated.Time.UTC().Format(time.RFC3339Nano)
		}
		if counted.Valid {
			r.StocktakeAt = counted.Time.UTC().Format(time.RFC3339Nano)
		}
		if p.Version > 0 {
			p.ItemID = r.ItemID
			p.WarehouseID = r.WarehouseID
			r.Policy = p
		}
		byKey[key(r.ItemID, r.WarehouseID)] = r
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Sale movements must belong to an actual payment from this warehouse's
	// outlet. An order without payment, parked order or completed void is never
	// promoted into the sales baseline just because a movement says "sale".
	rows, err = tx.Query(ppicUsageScopeCTE+` SELECT m.item_id,m.warehouse_id,m.movement_type,COALESCE(m.ref_type,''),m.qty_base,m.cost_per_base,
  (t.id IS NOT NULL AND NOT COALESCE(o.is_holding,false) AND NULLIF(o.payment_info->>'voided_at','') IS NULL AND NOT EXISTS(SELECT 1 FROM pos_void_stock_events v WHERE v.outlet_id=t.outlet_id AND v.order_id=t.order_id AND v.source_type='order')) AS valid_sale,
  (wo.id IS NOT NULL AND wo.status='done' AND wo.qty_planned_base>0 AND wo.qty_actual_base>0 AND wm.item_id IS NOT NULL AND wo.finished_at>=tz_day_start($1::date) AND wo.finished_at<tz_day_start($2::date+1)) AS valid_wo,
  (b.id IS NOT NULL AND ABS(b.input_qty_base+m.qty_base)<0.0001) AS valid_breakdown,
  (EXISTS(SELECT 1 FROM stock_wastes sw WHERE sw.source_movement_id=m.id OR (m.ref_type='stock_waste' AND sw.id::text=m.ref_id))) AS waste_document
 FROM stock_movements m JOIN wh ON wh.id=m.warehouse_id
 LEFT JOIN cloud_transactions t ON m.ref_type='transaction' AND t.id::text=m.ref_id AND t.outlet_id=wh.outlet_id
 LEFT JOIN cloud_orders o ON o.outlet_id=t.outlet_id AND o.local_id=t.order_id
 LEFT JOIN work_orders wo ON m.ref_type='work_order' AND wo.id::text=m.ref_id AND wo.warehouse_id=m.warehouse_id
 LEFT JOIN work_order_materials wm ON wm.wo_id=wo.id AND wm.item_id=m.item_id
 LEFT JOIN stock_breakdowns b ON m.ref_type='stock_breakdown' AND b.id::text=m.ref_id AND b.warehouse_id=m.warehouse_id AND b.input_item_id=m.item_id
 WHERE m.created_at>=tz_day_start($1::date) AND m.created_at<tz_day_start($2::date+1) AND m.qty_base<>0`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item, warehouse, kind, ref string
		var qty, cost float64
		var validSale, validWO, validBreakdown, wasteDocument bool
		if err = rows.Scan(&item, &warehouse, &kind, &ref, &qty, &cost, &validSale, &validWO, &validBreakdown, &wasteDocument); err != nil {
			rows.Close()
			return nil, err
		}
		r := byKey[key(item, warehouse)]
		if r == nil {
			continue
		}
		if qty > 0 {
			if kind == "adjustment" && ref == "stock_opname" {
				r.StocktakeSurplusQty += qty
			}
			continue
		}
		amount := -qty
		switch kind {
		case "sale":
			if validSale {
				r.SaleQty += amount
			} else {
				r.OtherUsageQty += amount
				usageFlag(r, "sale_basis_missing")
			}
		case "production_out":
			r.ProductionQty += amount
			if ref != "work_order" {
				r.ProductionReferenceQty += amount
				usageFlag(r, "production_documented_basis")
			} else if !validWO {
				r.ProductionReferenceQty += amount
				usageFlag(r, "production_snapshot_missing")
			}
		case "breakdown_out":
			r.BreakdownQty += amount
			usageFlag(r, "breakdown_documented_basis")
			if !validBreakdown {
				usageFlag(r, "breakdown_snapshot_missing")
			}
		case "adjustment":
			if ref == "stock_opname" {
				r.StocktakeShortageQty += amount
			} else {
				r.AdjustmentShortageQty += amount
			}
			r.ShortageValue += amount * cost
		case "transfer_out":
			r.TransferOutQty += amount
		case "quality_hold":
			r.QualityHoldQty += amount
		case "waste", "expired", "spoiled":
			if !wasteDocument {
				r.UndocumentedWasteQty += amount
				r.WasteQty += amount
				r.WasteValue += amount * cost
				usageFlag(r, "waste_document_missing")
			}
		case "return_out", "supplier_return", "return_supplier", "return_warehouse":
			usageFlag(r, "returns_excluded")
		default:
			r.OtherUsageQty += amount
			usageFlag(r, "other_usage_review")
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = tx.Query(ppicUsageScopeCTE+` SELECT m.item_id,o.warehouse_id,SUM(m.qty_plan_base*o.qty_actual_base/o.qty_planned_base),
 BOOL_OR(m.qty_actual_base IS NULL OR ABS(COALESCE((SELECT SUM(-sm.qty_base) FROM stock_movements sm WHERE sm.ref_type='work_order' AND sm.ref_id=o.id::text AND sm.item_id=m.item_id AND sm.warehouse_id=o.warehouse_id AND sm.movement_type='production_out' AND sm.qty_base<0),0)-COALESCE(m.qty_actual_base,0))>0.00005)
 FROM work_order_materials m JOIN work_orders o ON o.id=m.wo_id JOIN wh ON wh.id=o.warehouse_id
 WHERE o.status='done' AND o.qty_planned_base>0 AND o.qty_actual_base>0 AND o.finished_at>=tz_day_start($1::date) AND o.finished_at<tz_day_start($2::date+1)
 GROUP BY m.item_id,o.warehouse_id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item, warehouse string
		var qty float64
		var incomplete bool
		if err = rows.Scan(&item, &warehouse, &qty, &incomplete); err != nil {
			rows.Close()
			return nil, err
		}
		if r := byKey[key(item, warehouse)]; r != nil {
			r.ProductionReferenceQty += qty
			usageFlag(r, "production_plan_snapshot")
			if incomplete {
				usageFlag(r, "production_movement_missing")
			}
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = tx.Query(ppicUsageScopeCTE+` SELECT s.item_id,s.warehouse_id,SUM(s.qty_base),SUM(CASE WHEN s.reason='void' OR s.void_event_id IS NOT NULL THEN s.qty_base ELSE 0 END),SUM(s.total_cost)
 FROM stock_wastes s JOIN wh ON wh.id=s.warehouse_id WHERE s.created_at>=tz_day_start($1::date) AND s.created_at<tz_day_start($2::date+1) GROUP BY s.item_id,s.warehouse_id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item, warehouse string
		var qty, voidQty, value float64
		if err = rows.Scan(&item, &warehouse, &qty, &voidQty, &value); err != nil {
			rows.Close()
			return nil, err
		}
		if r := byKey[key(item, warehouse)]; r != nil {
			r.WasteQty += qty
			r.VoidWasteQty = voidQty
			r.WasteValue += value
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Failures cannot be expanded using today's recipe without inventing a
	// historical material baseline. Keep them as a warehouse data-quality gap.
	rows, err = tx.Query(ppicUsageScopeCTE+` SELECT wh.id,f.id FROM wh JOIN stock_deduction_failures f ON f.outlet_id=wh.outlet_id
 WHERE f.status='pending' AND f.created_at>=tz_day_start($1::date) AND f.created_at<tz_day_start($2::date+1)`, args...)
	if err != nil {
		return nil, err
	}
	pending := map[string]bool{}
	pendingIDs := map[string]bool{}
	for rows.Next() {
		var wh string
		var id string
		if err = rows.Scan(&wh, &id); err != nil {
			rows.Close()
			return nil, err
		}
		pending[wh] = true
		pendingIDs[id] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = tx.Query(ppicUsageScopeCTE+` SELECT DISTINCT wh.id FROM wh JOIN cloud_transactions t ON t.outlet_id=wh.outlet_id
 CROSS JOIN LATERAL jsonb_array_elements(COALESCE(t.items,'[]'::jsonb)) item
 LEFT JOIN cloud_orders o ON o.outlet_id=t.outlet_id AND o.local_id=t.order_id
 WHERE t.synced_at>=tz_day_start($1::date) AND t.synced_at<tz_day_start($2::date+1)
 AND NOT COALESCE(o.is_holding,false) AND NULLIF(o.payment_info->>'voided_at','') IS NULL
 AND NOT EXISTS(SELECT 1 FROM pos_void_stock_events v WHERE v.outlet_id=t.outlet_id AND v.order_id=t.order_id AND v.source_type='order')
 AND (CASE WHEN jsonb_typeof(item->'quantity')='number' THEN (item->>'quantity')::numeric ELSE 0 END)>0
 AND NOT EXISTS(SELECT 1 FROM pos_stock_deductions d WHERE d.outlet_id=t.outlet_id AND d.transaction_id=t.id::text AND d.product_local_id=COALESCE(item->>'product_id',''))
 AND NOT EXISTS(SELECT 1 FROM stock_movements m JOIN warehouses mw ON mw.id=m.warehouse_id WHERE mw.outlet_id=wh.outlet_id AND m.ref_type='transaction' AND m.ref_id=t.id::text AND m.qty_base<0 AND m.movement_type IN ('sale','waste') AND (m.pos_product_local_id=COALESCE(item->>'product_id','') OR m.pos_product_local_id=''))`, args...)
	if err != nil {
		return nil, err
	}
	rep.Summary.PendingDeductions = len(pendingIDs)
	missing := map[string]bool{}
	for rows.Next() {
		var wh string
		if err = rows.Scan(&wh); err != nil {
			rows.Close()
			return nil, err
		}
		missing[wh] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	search = strings.ToLower(strings.TrimSpace(search))
	for _, r := range byKey {
		if pending[r.WarehouseID] {
			usageFlag(r, "pending_deductions")
		}
		if missing[r.WarehouseID] {
			usageFlag(r, "sales_without_stock_basis")
		}
		finishPpicUsageRow(r)
		if search != "" && !strings.Contains(strings.ToLower(r.ItemName+" "+r.ItemCode+" "+r.WarehouseName+" "+r.OutletName), search) {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		rep.Rows = append(rep.Rows, *r)
		rep.Summary.ItemsCount++
		if r.Status == "over_limit" {
			rep.Summary.OverLimitCount++
		}
		if r.Policy == nil {
			rep.Summary.UnsetCount++
		}
		if !r.HasStocktake {
			rep.Summary.NeedsCount++
		}
		if r.ReferenceQty <= 0 {
			rep.Summary.NoBasisCount++
		}
		rep.Summary.WasteValue += r.WasteValue
		rep.Summary.ShortageValue += r.ShortageValue
	}
	rank := map[string]int{"over_limit": 0, "data_incomplete": 1, "unset": 2, "no_basis": 3, "needs_count": 4, "within_limit": 5, "no_activity": 6}
	sort.Slice(rep.Rows, func(i, j int) bool {
		a, b := rep.Rows[i], rep.Rows[j]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		if a.WarehouseName != b.WarehouseName {
			return a.WarehouseName < b.WarehouseName
		}
		if a.ItemName != b.ItemName {
			return a.ItemName < b.ItemName
		}
		return a.ItemID < b.ItemID
	})
	rep.TotalRows = len(rep.Rows)
	if limit > 0 {
		if page < 1 {
			page = 1
		}
		// Compare before multiplication: a very large page must return no rows,
		// rather than overflowing to a negative slice index.
		if page-1 > len(rep.Rows)/limit {
			rep.Rows = []models.PpicUsageControlRow{}
		} else {
			start := (page - 1) * limit
			if start >= len(rep.Rows) {
				rep.Rows = []models.PpicUsageControlRow{}
			} else {
				end := len(rep.Rows)
				if limit < len(rep.Rows)-start {
					end = start + limit
				}
				rep.Rows = rep.Rows[start:end]
			}
		}
	}

	rep.Summary.WasteValue = round2(rep.Summary.WasteValue)
	rep.Summary.ShortageValue = round2(rep.Summary.ShortageValue)
	return rep, nil
}

func usageStatusLabel(status string) string {
	switch status {
	case "over_limit":
		return "Melebihi batas"
	case "data_incomplete":
		return "Data belum lengkap"
	case "unset":
		return "Batas belum ditetapkan"
	case "no_basis":
		return "Belum ada acuan"
	case "needs_count":
		return "Perlu opname"
	case "within_limit":
		return "Dalam batas tercatat"
	case "no_activity":
		return "Tidak ada aktivitas"
	default:
		return fmt.Sprintf("%s", status)
	}
}
