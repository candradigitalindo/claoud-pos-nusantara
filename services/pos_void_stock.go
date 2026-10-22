package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Every POS stock writer uses this transaction-scoped lock, including retry.
// Outlet serialization also covers a payment arriving before its order exists.
func lockPOSStock(tx *sql.Tx, outletID string) error {
	_, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "pos-stock:"+outletID)
	return err
}

type posStockItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"product_name"`
	Qty       float64 `json:"qty"`
}

type posMaterial struct {
	id, unit string
	qty      float64
}

type posProduct struct {
	id, name  string
	materials []posMaterial
}

// Older POS versions send only a name. An exact, unique match within the same
// outlet is supported; ambiguous or missing products must be retried, not guessed.
func resolvePOSProduct(tx *sql.Tx, outletID string, item posStockItem) (posProduct, error) {
	var p posProduct
	var cloudID, stockType string
	var linked sql.NullString
	rows, err := tx.Query(`SELECT id,local_id,name,stock_type,linked_stock_item_id FROM cloud_products
		WHERE outlet_id=$1 AND (($2<>'' AND local_id=$2)
		OR ($2='' AND lower(btrim(name))=lower(btrim($3)) AND NOT is_deleted))
		ORDER BY id LIMIT 2 FOR SHARE`, outletID, strings.TrimSpace(item.ProductID), strings.TrimSpace(item.Name))
	if err != nil {
		return p, err
	}
	n := 0
	for rows.Next() {
		if err = rows.Scan(&cloudID, &p.id, &p.name, &stockType, &linked); err != nil {
			rows.Close()
			return p, err
		}
		n++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	if n != 1 {
		return p, fmt.Errorf("produk void/stok %q (%s) harus cocok dengan satu produk outlet; ditemukan %d", item.Name, item.ProductID, n)
	}
	switch stockType {
	case "none", "":
		return p, nil
	case "single":
		if !linked.Valid {
			return p, fmt.Errorf("tautan stok produk %s belum lengkap", p.name)
		}
		var unit string
		if err = tx.QueryRow(`SELECT base_unit FROM stock_items WHERE id=$1`, linked.String).Scan(&unit); err != nil {
			return p, err
		}
		p.materials = []posMaterial{{linked.String, unit, 1}}
	case "recipe":
		rows, err = tx.Query(`SELECT m.item_id,s.base_unit,m.qty_base FROM product_recipe_materials m
			JOIN stock_items s ON s.id=m.item_id WHERE m.product_id=$1 ORDER BY m.item_id`, cloudID)
		if err != nil {
			return p, err
		}
		for rows.Next() {
			var m posMaterial
			if err = rows.Scan(&m.id, &m.unit, &m.qty); err != nil {
				rows.Close()
				return p, err
			}
			p.materials = append(p.materials, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return p, err
		}
		if len(p.materials) == 0 {
			return p, fmt.Errorf("bahan resep %s belum lengkap", p.name)
		}
	default:
		return p, fmt.Errorf("jenis stok produk %s tidak dikenal", p.name)
	}
	for _, m := range p.materials {
		if m.qty <= 0 || math.IsNaN(m.qty) || math.IsInf(m.qty, 0) {
			return p, fmt.Errorf("jumlah bahan resep %s tidak valid", p.name)
		}
	}
	return p, nil
}

// Returns movement IDs; FIFO amounts and costs are captured before writing waste.
func consumePOSProduct(tx *sql.Tx, outletID string, p posProduct, qty float64, refID, refType, refNumber, actor, notes, movement string) ([]string, error) {
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) {
		return nil, fmt.Errorf("jumlah produk harus positif")
	}
	if len(p.materials) == 0 {
		return nil, nil
	}
	var wh string
	if err := tx.QueryRow(`SELECT id FROM warehouses WHERE outlet_id=$1 AND type='outlet' AND is_active`, outletID).Scan(&wh); err != nil {
		return nil, fmt.Errorf("gudang aktif untuk stok produk %s: %w", p.name, err)
	}
	ids := []string{}
	for _, m := range p.materials {
		amount := round4(m.qty * qty)
		if amount <= 0 {
			return nil, fmt.Errorf("pemakaian bahan %s terlalu kecil", p.name)
		}
		out, err := applyMovementValued(tx, m.id, wh, movement, m.unit, refID, refType, refNumber, notes, actor, -amount, -amount, 0, "")
		if err != nil {
			return nil, fmt.Errorf("pemakaian bahan %s: %w", p.name, err)
		}
		if _, err = tx.Exec(`UPDATE stock_movements SET pos_product_local_id=$2 WHERE id=$1`, out.id, p.id); err != nil {
			return nil, err
		}
		ids = append(ids, out.id)
	}
	return ids, nil
}

// The receipt is written in the same transaction as every ingredient. Products
// sharing an ingredient and a retried failed deduction remain independent.
func deductPOSProduct(tx *sql.Tx, outletID, productID string, qty float64, txnID, number string) error {
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) {
		return fmt.Errorf("jumlah produk harus positif")
	}
	var previous float64
	err := tx.QueryRow(`SELECT qty FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=$2 AND product_local_id=$3`, outletID, txnID, productID).Scan(&previous)
	if err == nil {
		if math.Abs(previous-qty) > 0.0001 {
			return fmt.Errorf("jumlah produk transaksi yang sudah memotong stok berubah")
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	// Pre-migration sales lack a per-product receipt. Preserve their deductions;
	// only a recorded failed product is eligible for another attempt.
	var legacy, failed bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM stock_movements sm JOIN warehouses w ON w.id=sm.warehouse_id
		WHERE w.outlet_id=$1 AND sm.ref_type='transaction' AND sm.ref_id=$2 AND sm.pos_product_local_id=''),
		EXISTS(SELECT 1 FROM stock_deduction_failures WHERE outlet_id=$1 AND transaction_id=$2 AND product_local_id=$3 AND status='pending')`, outletID, txnID, productID).Scan(&legacy, &failed); err != nil {
		return err
	}
	if legacy && !failed {
		return nil
	}
	p, err := resolvePOSProduct(tx, outletID, posStockItem{ProductID: productID})
	if err != nil {
		return err
	}
	if _, err = consumePOSProduct(tx, outletID, p, qty, txnID, "transaction", number, "system", "Recipe deduction from sale", "sale"); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO pos_stock_deductions(outlet_id,transaction_id,product_local_id,qty) VALUES($1,$2,$3,$4)`, outletID, txnID, productID, qty)
	return err
}

type posVoidEvent struct {
	id, outletID, orderID, sourceType, sourceID, actor, reason string
	at                                                         time.Time
	items                                                      []posStockItem
}

func claimPOSVoid(tx *sql.Tx, e *posVoidEvent) (bool, error) {
	if e.sourceID == "" || e.orderID == "" {
		return false, fmt.Errorf("ID order dan event void wajib diisi")
	}
	data, err := json.Marshal(e.items)
	if err != nil {
		return false, err
	}
	e.id = NewULID()
	res, err := tx.Exec(`INSERT INTO pos_void_stock_events(id,outlet_id,order_id,source_type,source_id,voided_at,actor,reason,items)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, e.id, e.outletID, e.orderID, e.sourceType, e.sourceID, e.at, e.actor, e.reason, string(data))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Reclassify the original physical consumption without touching ledger/batches.
// The unique movement link makes the waste register and inventory agree.
func recordPOSVoidWaste(tx *sql.Tx, e posVoidEvent, movementID string) error {
	// Use this transaction's connection: acquiring a second pooled connection
	// while other requests wait for the outlet lock can exhaust the pool.
	var timezone string
	if err := tx.QueryRow(`SELECT COALESCE((SELECT value FROM app_settings WHERE key='timezone'),'Asia/Jakarta')`).Scan(&timezone); err != nil {
		return err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.FixedZone("WIB", 7*60*60)
	}
	prefix := "WST-" + e.at.In(location).Format("20060102") + "-"
	seq, err := nextDocNumber(tx, "stock_wastes", "waste_number", prefix)
	if err != nil {
		return err
	}
	number := fmt.Sprintf("%s%04d", prefix, seq)
	note := fmt.Sprintf("Waste akibat pembatalan; order %s; %s %s; %s", e.orderID, e.sourceType, e.sourceID, e.reason)
	_, err = tx.Exec(`INSERT INTO stock_wastes(id,waste_number,warehouse_id,item_id,qty_base,qty_dist,unit_used,cost_per_base,total_cost,reason,notes,created_by,created_at,void_event_id,source_movement_id)
		SELECT $1,$2,sm.warehouse_id,sm.item_id,-sm.qty_base,-sm.qty_base,s.base_unit,sm.cost_per_base,-sm.qty_base*sm.cost_per_base,
		'void',$3,$4,$5,$6,sm.id FROM stock_movements sm JOIN stock_items s ON s.id=sm.item_id
		WHERE sm.id=$7 AND sm.qty_base<0 ON CONFLICT(source_movement_id) WHERE source_movement_id IS NOT NULL DO NOTHING`, NewULID(), number, note, e.actor, e.at, e.id, movementID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE stock_movements SET movement_type='waste',notes=concat_ws('; ',NULLIF(notes,''),$2::text) WHERE id=$1 AND movement_type='sale'`, movementID, note)
	return err
}

func consumePOSVoidItems(tx *sql.Tx, e posVoidEvent, items []posStockItem, consumed map[string]float64) ([]string, error) {
	products := map[string]posProduct{}
	amounts := map[string]float64{}
	for _, item := range items {
		if item.Qty <= 0 || math.IsNaN(item.Qty) || math.IsInf(item.Qty, 0) {
			return nil, fmt.Errorf("jumlah item void harus positif")
		}
		// A paid product's original recipe has already been consumed. Do not
		// require its current recipe/name to reconstruct that historical usage.
		if item.ProductID != "" && consumed[item.ProductID] >= item.Qty {
			consumed[item.ProductID] -= item.Qty
			continue
		}
		p, err := resolvePOSProduct(tx, e.outletID, item)
		if err != nil {
			return nil, err
		}
		products[p.id] = p
		amounts[p.id] += item.Qty
	}
	keys := make([]string, 0, len(amounts))
	for key := range amounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	movements := []string{}
	for _, key := range keys {
		qty := amounts[key] - consumed[key]
		if qty <= 0.0001 {
			continue
		}
		ids, err := consumePOSProduct(tx, e.outletID, products[key], qty, e.id, "pos_void", e.orderID, e.actor, "Waste akibat pembatalan: "+e.reason, "waste")
		if err != nil {
			return nil, err
		}
		movements = append(movements, ids...)
	}
	return movements, nil
}

func applyOrderVoidStock(tx *sql.Tx, outletID, orderID string, req models.PushOrderRequest) error {
	at, ok := parseTimeStrict(req.PaymentInfo.VoidedAt)
	if !ok {
		return fmt.Errorf("waktu void tidak valid")
	}
	e := posVoidEvent{outletID: outletID, orderID: orderID, sourceType: "order", sourceID: orderID, actor: req.PaymentInfo.VoidedBy, reason: req.PaymentInfo.VoidReason, at: at.UTC(), items: []posStockItem{}}
	for _, i := range req.Items {
		e.items = append(e.items, posStockItem{i.ProductID, i.ProductName, float64(i.Qty)})
	}
	fresh, err := claimPOSVoid(tx, &e)
	if err != nil || !fresh {
		return err
	}
	rows, err := tx.Query(`SELECT id,COALESCE(items,'[]') FROM cloud_transactions WHERE outlet_id=$1 AND btrim(order_id)=$2 ORDER BY id`, outletID, orderID)
	if err != nil {
		return err
	}
	txnIDs := []string{}
	paidItems := []posStockItem{}
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		txnIDs = append(txnIDs, strings.TrimSpace(id))
		var items []models.TransactionItem
		if err = json.Unmarshal(raw, &items); err != nil {
			rows.Close()
			return err
		}
		for _, i := range items {
			paidItems = append(paidItems, posStockItem{i.ProductID, i.ProductName, float64(i.Quantity)})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	consumed := map[string]float64{}
	rows, err = tx.Query(`SELECT product_local_id,SUM(qty) FROM pos_stock_deductions WHERE outlet_id=$1 AND transaction_id=ANY($2) GROUP BY product_local_id`, outletID, pq.Array(txnIDs))
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var qty float64
		if err = rows.Scan(&id, &qty); err != nil {
			rows.Close()
			return err
		}
		consumed[id] = qty
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.Query(`SELECT sm.id,sm.pos_product_local_id FROM stock_movements sm JOIN warehouses w ON w.id=sm.warehouse_id
		WHERE w.outlet_id=$1 AND sm.ref_type='transaction' AND sm.ref_id=ANY($2) AND sm.movement_type='sale' ORDER BY sm.id`, outletID, pq.Array(txnIDs))
	if err != nil {
		return err
	}
	ids := []string{}
	legacy := false
	for rows.Next() {
		var id, product string
		if err = rows.Scan(&id, &product); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
		legacy = legacy || product == ""
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}

	items := e.items
	if len(items) == 0 {
		items = paidItems
	}
	// Recover the product ID from the original payment snapshot for old order
	// payloads that only carry names, even after a recipe or product rename.
	paidNames := map[string]string{}
	for _, item := range paidItems {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if previous, exists := paidNames[name]; exists && previous != item.ProductID {
			paidNames[name] = ""
		} else if !exists {
			paidNames[name] = item.ProductID
		}
	}
	for i := range items {
		if items[i].ProductID == "" {
			items[i].ProductID = paidNames[strings.ToLower(strings.TrimSpace(items[i].Name))]
		}
	}
	if legacy {
		// Historical movements are authoritative; never re-expand an old recipe
		// just because it has changed. Recover only explicitly failed products.
		items = []posStockItem{}
		consumed = map[string]float64{}
		rows, err = tx.Query(`SELECT f.product_local_id,SUM(f.qty) FROM stock_deduction_failures f
			WHERE f.outlet_id=$1 AND f.transaction_id=ANY($2) AND f.status='pending'
			AND NOT EXISTS(SELECT 1 FROM pos_stock_deductions d WHERE d.outlet_id=f.outlet_id AND d.transaction_id=f.transaction_id AND d.product_local_id=f.product_local_id)
			GROUP BY f.product_local_id`, outletID, pq.Array(txnIDs))
		if err != nil {
			return err
		}
		for rows.Next() {
			var i posStockItem
			if err = rows.Scan(&i.ProductID, &i.Qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, i)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	newMovements, err := consumePOSVoidItems(tx, e, items, consumed)
	if err != nil {
		return err
	}
	// Take all inventory locks before the shared waste-number lock.
	for _, id := range append(ids, newMovements...) {
		if err = recordPOSVoidWaste(tx, e, id); err != nil {
			return err
		}
	}

	if _, err = tx.Exec(`UPDATE stock_deduction_failures SET status='cancelled',last_error='Dialihkan ke waste akibat pembatalan' WHERE outlet_id=$1 AND transaction_id=ANY($2) AND status='pending'`, outletID, pq.Array(txnIDs)); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM transaction_payments WHERE outlet_id=$1 AND transaction_id=ANY($2)`, outletID, pq.Array(txnIDs)); err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM cloud_transactions WHERE outlet_id=$1 AND id::text=ANY($2)`, outletID, pq.Array(txnIDs))
	return err
}

func applyItemVoidStock(tx *sql.Tx, outletID string, req models.PushOrderItemVoidRequest) error {
	at, ok := parseTimeStrict(req.VoidedAt)
	if !ok {
		return fmt.Errorf("waktu void item tidak valid")
	}
	e := posVoidEvent{outletID: outletID, orderID: req.OrderID, sourceType: "order_item", sourceID: req.LocalID, actor: req.VoidedBy, reason: req.VoidReason, at: at.UTC(), items: []posStockItem{{req.ProductID, req.ProductName, req.Qty}}}
	fresh, err := claimPOSVoid(tx, &e)
	if err != nil || !fresh {
		return err
	}
	ids, err := consumePOSVoidItems(tx, e, e.items, map[string]float64{})
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = recordPOSVoidWaste(tx, e, id); err != nil {
			return err
		}
	}
	return nil
}

// Cancel orphaned retries and recheck the receipt while holding the same lock as
// payment/void. A scheduler snapshot taken just before a void cannot deduct later.
func retryPOSStockDeduction(id, outletID, txnID, productID string, qty float64) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockPOSStock(tx, outletID); err != nil {
		return err
	}
	var pending, exists bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM stock_deduction_failures WHERE id=$1 AND status='pending'),
		EXISTS(SELECT 1 FROM cloud_transactions t WHERE t.outlet_id=$2 AND t.id=$3 AND NOT EXISTS(SELECT 1 FROM cloud_orders o WHERE o.outlet_id=t.outlet_id AND o.local_id=t.order_id AND NULLIF(o.payment_info->>'voided_at','') IS NOT NULL))`, id, outletID, txnID).Scan(&pending, &exists); err != nil {
		return err
	}
	if !pending {
		return nil
	}
	if !exists {
		_, err = tx.Exec(`UPDATE stock_deduction_failures SET status='cancelled',last_error='Transaksi tidak aktif atau sudah void' WHERE id=$1`, id)
	} else {
		if err = deductPOSProduct(tx, outletID, productID, qty, txnID, txnID); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE stock_deduction_failures SET status='resolved',attempts=attempts+1,last_attempt_at=NOW() AT TIME ZONE 'UTC' WHERE id=$1`, id)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
