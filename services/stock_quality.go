package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ErrStockQualityAccess = errors.New("Anda tidak berwenang melakukan tahap ini pada gudang tersebut")

type StockQualityAccess struct {
	ActorID, ActorName string
	Scope              []string
	Permissions        map[string]bool
}

func (a StockQualityAccess) Has(p string) bool {
	if a.Permissions[p] || a.Permissions["*"] {
		return true
	}
	if strings.HasSuffix(p, ".view") {
		prefix := strings.TrimSuffix(p, ".view") + "."
		for key, allowed := range a.Permissions {
			if allowed && strings.HasPrefix(key, prefix) {
				return true
			}
		}
	}
	return false
}

type qualityQueryer interface {
	QueryRow(string, ...interface{}) *sql.Row
}

func qualityWarehouse(q qualityQueryer, id string, a StockQualityAccess, stage string) (string, error) {
	var kind, oid string
	var active bool
	if err := q.QueryRow(`SELECT type,COALESCE(outlet_id,''),is_active FROM warehouses WHERE id=$1`, id).Scan(&kind, &oid, &active); err != nil {
		return "", err
	}
	if !active && stage != "view" {
		return "", Invalid("gudang sudah tidak aktif")
	}
	if a.Scope != nil {
		found := false
		for _, s := range a.Scope {
			if s == oid && kind == "outlet" {
				found = true
			}
		}
		if !found {
			return "", ErrStockQualityAccess
		}
	}
	perm := "stockquality." + kind + "." + stage
	if stage == "view" {
		perm = "stockquality.view"
	}
	if stage == "refund" {
		perm = "stockquality.refund"
	}
	if stage == "transfer_receive" {
		perm = "stocktransfers.receive"
		if a.Has("stockquality." + kind + ".receive") {
			return kind, nil
		}
	}
	if !a.Has(perm) {
		return "", ErrStockQualityAccess
	}
	return kind, nil
}

func StockQualityWarehouseAccess(id string, a StockQualityAccess, stage string) error {
	_, err := qualityWarehouse(database.DB, id, a, stage)
	return err
}

func claimStockEvidence(tx *sql.Tx, id, warehouse, ref string, a StockQualityAccess) error {
	if id == "" {
		return Invalid("foto bukti dengan timestamp server wajib diunggah")
	}
	res, err := tx.Exec(`UPDATE stock_evidence SET used_ref=$1 WHERE id=$2 AND warehouse_id=$3 AND actor_id=$4 AND used_ref='' AND recorded_at>NOW()-INTERVAL '24 hours'`, ref, id, warehouse, a.ActorID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Invalid("foto tidak sah, sudah digunakan, berbeda gudang/pengunggah, atau lebih dari 24 jam; unggah foto baru")
	}
	return nil
}

func qualityEvent(tx *sql.Tx, id, action, notes, evidence, reference string, qty, amount float64, a StockQualityAccess) error {
	_, err := tx.Exec(`INSERT INTO stock_quality_events(id,case_id,action,actor_id,actor_name,notes,evidence_id,qty_base,amount,reference) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, NewULID(), id, action, a.ActorID, a.ActorName, notes, nilIfEmpty(evidence), qty, amount, reference)
	return err
}

func insertQualityCase(tx *sql.Tx, c *models.StockQualityCase, a StockQualityAccess) error {
	c.ID = NewULID()
	c.Number = "QC-" + time.Now().In(GetTimezoneLocation()).Format("20060102") + "-" + c.ID[18:]
	_, err := tx.Exec(`INSERT INTO stock_quality_cases(id,number,warehouse_id,item_id,qty_base,qty_input,unit_used,cost_per_base,expiry_date,reason,notes,return_warehouse_id,transfer_id,parent_case_id,created_by,created_name)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, c.ID, c.Number, c.WarehouseID, c.ItemID, c.QtyBase, c.QtyInput, c.UnitUsed, c.CostPerBase, nilIfEmpty(c.ExpiryDate), c.Reason, c.Notes, nilIfEmpty(c.ReturnWarehouseID), nilIfEmpty(c.TransferID), nilIfEmpty(c.ParentCaseID), a.ActorID, a.ActorName)
	return err
}

// Quarantine one identified batch, preserving that batch's valuation and expiry.
// Available stock is deducted once here; disposal/return never deduct it again.
func CreateStockQuality(req models.StockQualityRequest, a StockQualityAccess) (string, error) {
	if !map[string]bool{"reject": true, "expired": true, "spoiled": true, "damaged": true, "return": true}[req.Reason] || strings.TrimSpace(req.Notes) == "" || len(req.Notes) > 10000 {
		return "", Invalid("alasan dan catatan kejadian wajib diisi")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	kind, err := qualityWarehouse(tx, req.WarehouseID, a, "report")
	if err != nil {
		return "", err
	}
	if kind != "central" && req.Reason == "return" {
		return "", Invalid("retur supplier hanya dari Gudang Induk; outlet melaporkan reject/rusak")
	}
	var base, dist string
	var ratio float64
	if err = tx.QueryRow(`SELECT base_unit,dist_unit,dist_ratio FROM stock_items WHERE id=$1 AND is_active`, req.ItemID).Scan(&base, &dist, &ratio); err != nil {
		return "", err
	}
	qty, unit, err := stockQuantityToBase(req.Qty, req.UnitUsed, base, dist, ratio)
	if err != nil {
		return "", err
	}
	qty = round4(qty)
	if qty <= 0 {
		return "", Invalid("jumlah terlalu kecil")
	}
	var ledgerQty float64
	if err = tx.QueryRow(`SELECT qty_base FROM stock_ledger WHERE warehouse_id=$1 AND item_id=$2 FOR UPDATE`, req.WarehouseID, req.ItemID).Scan(&ledgerQty); err != nil {
		return "", Invalid("stok barang tidak tersedia")
	}
	var batchQty, cost float64
	var expiry string
	if err = tx.QueryRow(`SELECT qty_base,cost_per_base,COALESCE(expiry_date::text,'') FROM stock_batches WHERE id=$1 AND item_id=$2 AND warehouse_id=$3 FOR UPDATE`, req.BatchID, req.ItemID, req.WarehouseID).Scan(&batchQty, &cost, &expiry); err != nil {
		return "", Invalid("pilih batch stok yang tersedia di gudang ini")
	}
	if qty > batchQty || qty > ledgerQty {
		return "", Invalid("jumlah melebihi stok tersedia dalam batch")
	}
	c := models.StockQualityCase{WarehouseID: req.WarehouseID, ItemID: req.ItemID, QtyBase: qty, QtyInput: req.Qty, UnitUsed: unit, CostPerBase: cost, ExpiryDate: expiry, Reason: req.Reason, Notes: strings.TrimSpace(req.Notes)}
	if err = insertQualityCase(tx, &c, a); err != nil {
		return "", err
	}
	if err = claimStockEvidence(tx, req.EvidenceID, req.WarehouseID, c.ID, a); err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE stock_batches SET qty_base=qty_base-$1 WHERE id=$2`, qty, req.BatchID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(`UPDATE stock_ledger SET qty_base=qty_base-$1,updated_at=NOW() WHERE item_id=$2 AND warehouse_id=$3`, qty, req.ItemID, req.WarehouseID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(`INSERT INTO stock_movements(id,item_id,warehouse_id,movement_type,qty_base,qty_dist,unit_used,cost_per_base,balance_after,ref_id,ref_type,ref_number,notes,created_by,expiry_date)
		VALUES($1,$2,$3,'quality_hold',$4,$5,$6,$7,$8,$9,'stock_quality',$10,$11,$12,$13)`, NewULID(), req.ItemID, req.WarehouseID, -qty, -req.Qty, unit, cost, ledgerQty-qty, c.ID, c.Number, req.Notes, a.ActorName, nilIfEmpty(expiry)); err != nil {
		return "", err
	}
	if err = qualityEvent(tx, c.ID, "reported", req.Notes, req.EvidenceID, req.BatchID, qty, 0, a); err != nil {
		return "", err
	}
	return c.ID, tx.Commit()
}

const qualitySelect = `SELECT c.id,c.number,c.warehouse_id,w.name,w.type,c.item_id,i.name,i.base_unit,c.qty_base,c.qty_input,c.unit_used,c.cost_per_base,
	COALESCE(c.expiry_date::text,''),c.reason,c.notes,c.status,c.disposition,c.settlement,COALESCE(c.vendor_id,''),c.vendor_name,c.supplier_reference,
	COALESCE(c.return_warehouse_id,''),COALESCE(rw.name,''),COALESCE(c.transfer_id,''),COALESCE(c.parent_case_id,''),c.settled_qty_base,c.refund_amount,
	c.created_by,c.created_name,c.approved_by,c.version,c.created_at::text
	FROM stock_quality_cases c JOIN warehouses w ON w.id=c.warehouse_id JOIN stock_items i ON i.id=c.item_id LEFT JOIN warehouses rw ON rw.id=c.return_warehouse_id `

type qualityScanner interface{ Scan(...interface{}) error }

func scanQuality(row qualityScanner) (models.StockQualityCase, error) {
	var c models.StockQualityCase
	err := row.Scan(&c.ID, &c.Number, &c.WarehouseID, &c.WarehouseName, &c.WarehouseType, &c.ItemID, &c.ItemName, &c.BaseUnit, &c.QtyBase, &c.QtyInput, &c.UnitUsed, &c.CostPerBase, &c.ExpiryDate, &c.Reason, &c.Notes, &c.Status, &c.Disposition, &c.Settlement, &c.VendorID, &c.VendorName, &c.SupplierReference, &c.ReturnWarehouseID, &c.ReturnWarehouseName, &c.TransferID, &c.ParentCaseID, &c.SettledQtyBase, &c.RefundAmount, &c.CreatedBy, &c.CreatedName, &c.ApprovedBy, &c.Version, &c.CreatedAt)
	return c, err
}

func ListStockQuality(warehouse, status, search string, page, limit int, a StockQualityAccess) ([]models.StockQualityCase, int, error) {
	if !a.Has("stockquality.view") {
		return nil, 0, ErrStockQualityAccess
	}
	if warehouse != "" {
		if _, err := qualityWarehouse(database.DB, warehouse, a, "view"); err != nil {
			return nil, 0, err
		}
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	where := ` WHERE ($1 OR w.type='outlet' AND w.outlet_id=ANY($2)) AND ($3='' OR c.warehouse_id=$3) AND ($4='' OR c.status=$4) AND ($5='' OR c.number ILIKE $6 OR i.name ILIKE $6 OR c.vendor_name ILIKE $6)`
	args := []interface{}{a.Scope == nil, pq.Array(a.Scope), warehouse, status, search, "%" + search + "%"}
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_quality_cases c JOIN warehouses w ON w.id=c.warehouse_id JOIN stock_items i ON i.id=c.item_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := database.DB.Query(qualitySelect+where+` ORDER BY c.created_at DESC,c.id DESC LIMIT $7 OFFSET $8`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []models.StockQualityCase{}
	for rows.Next() {
		c, err := scanQuality(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func GetStockQuality(id string, a StockQualityAccess) (*models.StockQualityCase, error) {
	c, err := scanQuality(database.DB.QueryRow(qualitySelect+` WHERE c.id=$1`, id))
	if err != nil {
		return nil, err
	}
	if _, err = qualityWarehouse(database.DB, c.WarehouseID, a, "view"); err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT e.id,e.action,e.actor_name,e.notes,e.qty_base,e.amount,e.reference,e.created_at::text,
		COALESCE(p.id,''),COALESCE(p.url,''),COALESCE(p.recorded_at::text,''),COALESCE(p.actor_name,'') FROM stock_quality_events e LEFT JOIN stock_evidence p ON p.id=e.evidence_id WHERE e.case_id=$1 ORDER BY e.created_at,e.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c.Events = []models.StockQualityEvent{}
	for rows.Next() {
		var e models.StockQualityEvent
		var p models.StockEvidence
		if err := rows.Scan(&e.ID, &e.Action, &e.ActorName, &e.Notes, &e.QtyBase, &e.Amount, &e.Reference, &e.CreatedAt, &p.ID, &p.URL, &p.RecordedAt, &p.ActorName); err != nil {
			return nil, err
		}
		if p.ID != "" {
			e.Evidence = &p
		}
		c.Events = append(c.Events, e)
	}
	return &c, rows.Err()
}

type StockQualityBatch struct {
	ID         string  `json:"id"`
	QtyBase    float64 `json:"qty_base"`
	ExpiryDate string  `json:"expiry_date"`
	Reference  string  `json:"reference"`
}

func StockQualityBatches(warehouse, item string, a StockQualityAccess) ([]StockQualityBatch, error) {
	if _, err := qualityWarehouse(database.DB, warehouse, a, "report"); err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT id,qty_base,COALESCE(expiry_date::text,''),COALESCE(ref_type,'') FROM stock_batches WHERE warehouse_id=$1 AND item_id=$2 AND qty_base>0 ORDER BY expiry_date NULLS LAST,created_at,id`, warehouse, item)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StockQualityBatch{}
	for rows.Next() {
		var b StockQualityBatch
		if err := rows.Scan(&b.ID, &b.QtyBase, &b.ExpiryDate, &b.Reference); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func qualityExpiry(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return Invalid("tanggal kedaluwarsa tidak valid")
	}
	return nil
}

func qualityWaste(tx *sql.Tx, c models.StockQualityCase, a StockQualityAccess) error {
	num, err := generateWasteNumber(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO stock_wastes(id,waste_number,warehouse_id,item_id,qty_base,qty_dist,unit_used,cost_per_base,total_cost,reason,notes,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, NewULID(), num, c.WarehouseID, c.ItemID, c.QtyBase, c.QtyInput, c.UnitUsed, c.CostPerBase, c.CostPerBase*c.QtyBase, c.Reason, "Pemusnahan karantina "+c.Number+": "+c.Notes, a.ActorName)
	return err
}

func stockQualityNumberError(action, status string) error {
	return Invalid("tahap %s tidak dapat dijalankan pada status %s", action, status)
}

func ActStockQuality(id string, req models.StockQualityAction, a StockQualityAccess) error {
	if strings.TrimSpace(req.Notes) == "" || len(req.Notes) > 10000 || len(req.Reference) > 200 {
		return Invalid("catatan tindakan wajib diisi (maksimal 10000 karakter)")
	}
	if req.Action == "approve" {
		req.EvidenceID = ""
	}
	if req.Action != "refund" {
		req.Amount = 0
	}
	if req.Action != "refund" && req.Action != "replacement" {
		req.QtyBase = 0
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	c, err := scanQuality(tx.QueryRow(qualitySelect+` WHERE c.id=$1 FOR UPDATE OF c`, id))
	if err != nil {
		return err
	}
	if c.Version != req.Version {
		return Conflict("dokumen telah berubah; muat ulang sebelum melanjutkan")
	}
	stage := map[string]string{"approve": "approve", "release": "approve", "execute": "execute", "replacement": "receive", "refund": "refund", "receive_return": "receive"}[req.Action]
	if stage == "" {
		return Invalid("tindakan tidak dikenal")
	}
	warehouse := c.WarehouseID
	if req.Action == "receive_return" {
		warehouse = c.ReturnWarehouseID
	}
	if _, err = qualityWarehouse(tx, warehouse, a, stage); err != nil {
		return err
	}
	if req.Action != "approve" {
		if err = claimStockEvidence(tx, req.EvidenceID, warehouse, id, a); err != nil {
			return err
		}
	}
	status := c.Status
	switch req.Action {
	case "approve", "release":
		if c.Status != "pending" {
			return stockQualityNumberError(req.Action, c.Status)
		}
		if c.CreatedBy == a.ActorID {
			return Invalid("pelapor tidak boleh menyetujui laporannya sendiri")
		}
		if req.Action == "release" {
			if c.ExpiryDate != "" && c.ExpiryDate <= time.Now().In(GetTimezoneLocation()).Format("2006-01-02") {
				return Invalid("batch yang sudah kedaluwarsa tidak dapat dilepas menjadi stok siap pakai")
			}
			if err = applyMovement(tx, c.ItemID, c.WarehouseID, "quality_release", c.UnitUsed, id, "stock_quality", c.Number, req.Notes, a.ActorName, c.QtyBase, c.QtyInput, c.CostPerBase, c.ExpiryDate); err != nil {
				return err
			}
			status = "released"
		} else {
			switch req.Disposition {
			case "dispose":
			case "return_supplier":
				if c.WarehouseType != "central" {
					return Invalid("retur supplier hanya melalui Gudang Induk")
				}
				if req.Settlement != "replacement" && req.Settlement != "refund" {
					return Invalid("pilih penyelesaian barang pengganti atau refund")
				}
				if strings.TrimSpace(req.Reference) == "" {
					return Invalid("nomor invoice/surat jalan supplier wajib diisi")
				}
				if err = tx.QueryRow(`SELECT name FROM vendors WHERE id=$1 AND is_active`, req.VendorID).Scan(&c.VendorName); err != nil {
					return Invalid("pilih supplier aktif")
				}
				c.VendorID = req.VendorID
				c.SupplierReference = req.Reference
				c.Settlement = req.Settlement
			case "return_warehouse":
				if c.WarehouseType != "outlet" {
					return Invalid("retur ke induk hanya dari gudang outlet")
				}
				var kind string
				if err = tx.QueryRow(`SELECT type FROM warehouses WHERE id=$1 AND is_active`, req.ReturnWarehouseID).Scan(&kind); err != nil || kind != "central" {
					return Invalid("pilih gudang induk tujuan retur")
				}
				c.ReturnWarehouseID = req.ReturnWarehouseID
			default:
				return Invalid("pilih tindak lanjut karantina")
			}
			c.Disposition = req.Disposition
			status = "approved"
		}
		c.ApprovedBy = a.ActorID
	case "execute":
		if c.Status != "approved" {
			return stockQualityNumberError(req.Action, c.Status)
		}
		if c.Disposition == "dispose" {
			if err = qualityWaste(tx, c, a); err != nil {
				return err
			}
			status = "disposed"
		} else {
			status = "in_transit"
		}
	case "receive_return":
		if c.Status != "in_transit" || c.Disposition != "return_warehouse" {
			return stockQualityNumberError(req.Action, c.Status)
		}
		// Returned rejects are still quarantined at the central warehouse. They
		// do not become saleable stock merely because a return truck arrived.
		child := models.StockQualityCase{WarehouseID: c.ReturnWarehouseID, ItemID: c.ItemID, QtyBase: c.QtyBase, QtyInput: c.QtyBase, UnitUsed: c.BaseUnit, CostPerBase: c.CostPerBase, ExpiryDate: c.ExpiryDate, Reason: "reject", Notes: "Retur outlet " + c.Number + ": " + req.Notes, ParentCaseID: c.ID}
		if err = insertQualityCase(tx, &child, a); err != nil {
			return err
		}
		if err = qualityEvent(tx, child.ID, "reported", child.Notes, req.EvidenceID, c.Number, child.QtyBase, 0, a); err != nil {
			return err
		}
		status = "returned"
	case "replacement", "refund":
		if c.Status != "in_transit" || c.Disposition != "return_supplier" || c.Settlement != req.Action {
			return stockQualityNumberError(req.Action, c.Status)
		}
		if !fnbFinite(req.QtyBase) || req.QtyBase <= 0 || round4(req.QtyBase) <= 0 || round4(req.QtyBase) > round4(c.QtyBase-c.SettledQtyBase) {
			return Invalid("jumlah penyelesaian melebihi sisa retur atau tidak valid")
		}
		req.QtyBase = round4(req.QtyBase)
		if strings.TrimSpace(req.Reference) == "" {
			return Invalid("nomor bukti penyelesaian wajib diisi")
		}
		if req.Action == "replacement" {
			if err = qualityExpiry(req.ExpiryDate); err != nil {
				return err
			}
			if c.ExpiryDate != "" && req.ExpiryDate == "" {
				return Invalid("tanggal kedaluwarsa barang pengganti wajib diisi")
			}
			if req.ExpiryDate != "" && req.ExpiryDate <= time.Now().In(GetTimezoneLocation()).Format("2006-01-02") {
				return Invalid("barang pengganti harus belum kedaluwarsa")
			}
			if err = applyMovement(tx, c.ItemID, c.WarehouseID, "supplier_replacement", c.BaseUnit, id, "stock_quality", c.Number, req.Notes, a.ActorName, req.QtyBase, req.QtyBase, c.CostPerBase, req.ExpiryDate); err != nil {
				return err
			}
		} else {
			if !fnbFinite(req.Amount) || req.Amount <= 0 || req.Amount > 1e12 {
				return Invalid("nominal refund harus lebih dari nol dan valid")
			}
			c.RefundAmount += req.Amount
		}
		c.SettledQtyBase = round4(c.SettledQtyBase + req.QtyBase)
		if c.SettledQtyBase == c.QtyBase {
			status = "settled"
		}
	}
	if _, err = tx.Exec(`UPDATE stock_quality_cases SET status=$2,disposition=$3,settlement=$4,vendor_id=$5,vendor_name=$6,supplier_reference=$7,return_warehouse_id=$8,settled_qty_base=$9,refund_amount=$10,approved_by=$11,version=version+1,updated_at=NOW() WHERE id=$1`, id, status, c.Disposition, c.Settlement, nilIfEmpty(c.VendorID), c.VendorName, c.SupplierReference, nilIfEmpty(c.ReturnWarehouseID), c.SettledQtyBase, c.RefundAmount, c.ApprovedBy); err != nil {
		return err
	}
	if err = qualityEvent(tx, id, req.Action, req.Notes, req.EvidenceID, req.Reference, req.QtyBase, req.Amount, a); err != nil {
		return err
	}
	return tx.Commit()
}
