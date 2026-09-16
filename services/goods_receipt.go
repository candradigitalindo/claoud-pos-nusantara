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

func generateGRNNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("GRN%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "goods_receipts", "grn_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// CreateGoodsReceipt mencatat penerimaan barang: header GRN + tiap baris menghasilkan
// stock-in (movement purchase_in + batch FIFO) via applyMovement, semua dalam satu transaksi.
func CreateGoodsReceipt(req models.GoodsReceiptRequest, actor string) (*models.GoodsReceipt, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	grnID, err := CreateGoodsReceiptTx(tx, req, actor)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetGoodsReceipt(grnID)
}

// CreateGoodsReceiptTx adalah isi CreateGoodsReceipt tanpa transaksinya sendiri,
// supaya penerimaan pengadaan (aset + stok gudang + status pengajuan) bisa
// berjalan sebagai SATU transaksi — bukan dua yang bisa putus di tengah.
func CreateGoodsReceiptTx(tx *sql.Tx, req models.GoodsReceiptRequest, actor string) (string, error) {
	if req.WarehouseID == "" {
		return "", fmt.Errorf("gudang wajib dipilih")
	}
	if len(req.Items) == 0 {
		return "", fmt.Errorf("minimal satu item diterima")
	}

	// Tanpa foreign key, id pengajuan yang salah ketik dulu tersimpan diam-diam
	// dan menghasilkan tautan yang menunjuk ke mana-mana.
	if strings.TrimSpace(req.PurchaseRequestID) != "" {
		var exists bool
		if err := database.DB.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM purchase_requests WHERE id = $1)`, req.PurchaseRequestID,
		).Scan(&exists); err != nil || !exists {
			return "", fmt.Errorf("pengajuan pengadaan yang ditautkan tidak ditemukan")
		}
	}

	grnID := NewULID()
	grnNumber, err := generateGRNNumber(tx)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()

	if _, err := tx.Exec(`
		INSERT INTO goods_receipts (id, grn_number, warehouse_id, vendor_name, po_ref, purchase_request_id, notes, received_by, received_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`,
		grnID, grnNumber, req.WarehouseID, req.VendorName, req.PORef, nilIfEmpty(req.PurchaseRequestID), req.Notes, actor, now,
	); err != nil {
		return "", fmt.Errorf("gagal membuat GRN: %w", err)
	}

	var totalCost float64
	for _, line := range req.Items {
		if line.ItemID == "" || line.QtyDist <= 0 {
			return "", fmt.Errorf("item dan qty (>0) wajib diisi tiap baris")
		}
		// Ambil master item untuk konversi satuan.
		var name, baseUnit, distUnit string
		var distRatio float64
		if err := tx.QueryRow(
			`SELECT name, base_unit, dist_unit, dist_ratio FROM stock_items WHERE id=$1`, line.ItemID,
		).Scan(&name, &baseUnit, &distUnit, &distRatio); err != nil {
			return "", fmt.Errorf("item stok tidak ditemukan: %w", err)
		}
		if distRatio <= 0 {
			distRatio = 1
		}
		qtyBase := line.QtyDist * distRatio
		subtotal := round2(qtyBase * line.CostPerBase)

		if err := applyMovement(tx, line.ItemID, req.WarehouseID, "purchase_in", distUnit,
			grnID, "goods_receipt", grnNumber, req.Notes, actor,
			qtyBase, line.QtyDist, line.CostPerBase, line.ExpiryDate); err != nil {
			return "", fmt.Errorf("gagal posting stok %s: %w", name, err)
		}

		var expiry interface{}
		if line.ExpiryDate != "" {
			expiry = line.ExpiryDate
		}
		if _, err := tx.Exec(`
			INSERT INTO goods_receipt_items (id, receipt_id, item_id, item_name, qty_base, qty_dist, unit_used, cost_per_base, subtotal, expiry_date, pr_item_key)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			NewULID(), grnID, line.ItemID, name, qtyBase, line.QtyDist, distUnit, line.CostPerBase, subtotal, expiry, line.PRItemKey,
		); err != nil {
			return "", fmt.Errorf("gagal simpan baris GRN: %w", err)
		}
		totalCost += subtotal
	}

	if _, err := tx.Exec(`UPDATE goods_receipts SET total_cost=$1 WHERE id=$2`, round2(totalCost), grnID); err != nil {
		return "", err
	}
	return grnID, nil
}

// GoodsReceiptInScope: gudang GRN berada dalam scope outlet user (pusat = boleh semua).
func GoodsReceiptInScope(id string, outletIDs []string) bool {
	if outletIDs == nil {
		return true
	}
	var cnt int
	database.DB.QueryRow(`
		SELECT COUNT(*) FROM goods_receipts g JOIN warehouses w ON w.id=g.warehouse_id
		WHERE g.id=$1 AND (w.outlet_id IS NULL OR w.outlet_id = ANY($2))`,
		id, pq.Array(outletIDs)).Scan(&cnt)
	return cnt > 0
}

func GetGoodsReceipt(id string) (*models.GoodsReceipt, error) {
	var g models.GoodsReceipt
	err := database.DB.QueryRow(`
		SELECT g.id, g.grn_number, g.warehouse_id, COALESCE(w.name,''), COALESCE(g.vendor_name,''),
			COALESCE(g.po_ref,''), COALESCE(g.purchase_request_id,''), COALESCE(pr.request_number,''),
			COALESCE(g.notes,''), g.total_cost, COALESCE(g.received_by,''),
			to_char(g.received_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM goods_receipts g
		JOIN warehouses w ON w.id=g.warehouse_id
		LEFT JOIN purchase_requests pr ON pr.id = g.purchase_request_id
		WHERE g.id=$1`, id,
	).Scan(&g.ID, &g.GRNNumber, &g.WarehouseID, &g.WarehouseName, &g.VendorName,
		&g.PORef, &g.PurchaseRequestID, &g.PurchaseRequestNumber,
		&g.Notes, &g.TotalCost, &g.ReceivedBy, &g.ReceivedAt)
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.Query(`
		SELECT gi.id, gi.item_id, gi.item_name, COALESCE(si.code,''), gi.qty_base, gi.qty_dist,
			COALESCE(gi.unit_used,''), COALESCE(si.base_unit,''), gi.cost_per_base, gi.subtotal,
			COALESCE(to_char(gi.expiry_date,'YYYY-MM-DD'),'')
		FROM goods_receipt_items gi LEFT JOIN stock_items si ON si.id=gi.item_id
		WHERE gi.receipt_id=$1 ORDER BY gi.item_name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	g.Items = []models.GoodsReceiptItem{}
	for rows.Next() {
		var it models.GoodsReceiptItem
		if err := rows.Scan(&it.ID, &it.ItemID, &it.ItemName, &it.ItemCode, &it.QtyBase, &it.QtyDist,
			&it.UnitUsed, &it.BaseUnit, &it.CostPerBase, &it.Subtotal, &it.ExpiryDate); err != nil {
			return nil, err
		}
		g.Items = append(g.Items, it)
	}
	g.ItemCount = len(g.Items)
	return &g, nil
}

func ListGoodsReceipts(warehouseID string, outletIDs []string, page, limit int) ([]models.GoodsReceipt, int, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if warehouseID != "" {
		conds = append(conds, fmt.Sprintf("g.warehouse_id=$%d", idx))
		args = append(args, warehouseID)
		idx++
	}
	if outletIDs != nil {
		conds = append(conds, fmt.Sprintf("(w.outlet_id IS NULL OR w.outlet_id = ANY($%d))", idx))
		args = append(args, pq.Array(outletIDs))
		idx++
	}
	where := strings.Join(conds, " AND ")

	var total int
	database.DB.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*) FROM goods_receipts g JOIN warehouses w ON w.id=g.warehouse_id WHERE %s`, where), args...).Scan(&total)

	offset := (page - 1) * limit
	q := fmt.Sprintf(`
		SELECT g.id, g.grn_number, g.warehouse_id, COALESCE(w.name,''), COALESCE(g.vendor_name,''),
			COALESCE(g.po_ref,''), COALESCE(pr.request_number,''), g.total_cost, COALESCE(g.received_by,''),
			to_char(g.received_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			(SELECT COUNT(*) FROM goods_receipt_items gi WHERE gi.receipt_id=g.id)
		FROM goods_receipts g
		JOIN warehouses w ON w.id=g.warehouse_id
		LEFT JOIN purchase_requests pr ON pr.id = g.purchase_request_id
		WHERE %s ORDER BY g.received_at DESC LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.GoodsReceipt{}
	for rows.Next() {
		var g models.GoodsReceipt
		if err := rows.Scan(&g.ID, &g.GRNNumber, &g.WarehouseID, &g.WarehouseName, &g.VendorName,
			&g.PORef, &g.PurchaseRequestNumber, &g.TotalCost, &g.ReceivedBy, &g.ReceivedAt, &g.ItemCount); err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, nil
}
