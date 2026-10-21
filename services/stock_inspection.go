package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"strings"
)

// ReceiveStockTransfer posts only the accepted quantities. Rejected quantities
// become valued quarantine cases; the remainder remains in transit for a later
// receipt. All quantities, evidence and audit entries commit together.
func ReceiveStockTransfer(id string, req models.StockInspectionRequest, a StockQualityAccess) error {
	if len(req.Items) == 0 || len(req.Items) > 500 || len(req.Notes) > 10000 {
		return Invalid("isi barang yang diperiksa")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status, from, to, sender, number string
	var version int
	if err = tx.QueryRow(`SELECT status,from_warehouse_id,to_warehouse_id,COALESCE(sent_by,''),transfer_number,receipt_version FROM stock_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&status, &from, &to, &sender, &number, &version); err != nil {
		return err
	}
	if status != "sent" && status != "partial_received" {
		return Invalid("transfer tidak sedang menunggu penerimaan")
	}
	if req.Version != version {
		return Conflict("penerimaan sudah berubah; muat ulang sebelum melanjutkan")
	}
	if sender == a.ActorName {
		return Invalid("pengirim tidak boleh menerima transfer yang sama")
	}
	if _, err = qualityWarehouse(tx, to, a, "transfer_receive"); err != nil {
		return err
	}
	receiptID := NewULID()
	if err = claimStockEvidence(tx, req.EvidenceID, to, receiptID, a); err != nil {
		return err
	}
	seen := map[string]bool{}
	processed := 0.0
	for _, it := range req.Items {
		if seen[it.ItemID] || it.ItemID == "" {
			return Invalid("baris barang penerimaan tidak boleh berulang")
		}
		seen[it.ItemID] = true
		if !fnbFinite(it.Accepted) || !fnbFinite(it.Rejected) || it.Accepted < 0 || it.Rejected < 0 {
			return Invalid("jumlah terima/reject harus valid dan tidak negatif")
		}
		accepted, rejected := round4(it.Accepted), round4(it.Rejected)
		if rejected > 0 && (!a.Has("stocktransfers.reject") || strings.TrimSpace(it.Reason) == "") {
			return Invalid("reject memerlukan izin reject dan alasan per barang")
		}
		if len(it.Reason) > 2000 {
			return Invalid("alasan reject terlalu panjang")
		}
		var qty, done, bad, qtyDist float64
		var unit, base string
		if err = tx.QueryRow(`SELECT t.qty_base,t.accepted_qty_base,t.rejected_qty_base,t.qty_dist,t.unit_used,i.base_unit FROM stock_transfer_items t JOIN stock_items i ON i.id=t.item_id WHERE transfer_id=$1 AND item_id=$2 FOR UPDATE OF t`, id, it.ItemID).Scan(&qty, &done, &bad, &qtyDist, &unit, &base); err != nil {
			return Invalid("barang tidak ditemukan pada transfer")
		}
		if accepted+rejected > round4(qty-done-bad) {
			return Invalid("jumlah diterima + reject melebihi sisa barang yang dikirim")
		}
		if accepted+rejected == 0 {
			continue
		}
		var cost float64
		var expiry sql.NullString
		if err = tx.QueryRow(`SELECT cost_per_base,expiry_date::text FROM stock_movements WHERE ref_id=$1 AND ref_type='stock_transfer' AND item_id=$2 AND warehouse_id=$3 AND movement_type='transfer_out' ORDER BY created_at DESC LIMIT 1`, id, it.ItemID, from).Scan(&cost, &expiry); err != nil {
			return Invalid("nilai pengiriman tidak ditemukan; periksa dokumen transfer")
		}
		if accepted > 0 {
			if err = applyMovement(tx, it.ItemID, to, "transfer_in", unit, id, "stock_transfer", number, req.Notes, a.ActorName, accepted, accepted*qtyDist/qty, cost, expiry.String); err != nil {
				return err
			}
		}
		if rejected > 0 {
			c := models.StockQualityCase{WarehouseID: to, ItemID: it.ItemID, QtyBase: rejected, QtyInput: rejected, UnitUsed: base, CostPerBase: cost, ExpiryDate: expiry.String, Reason: "reject", Notes: "Reject transfer " + number + ": " + it.Reason, TransferID: id, ReturnWarehouseID: from}
			if err = insertQualityCase(tx, &c, a); err != nil {
				return err
			}
			if err = qualityEvent(tx, c.ID, "reported", c.Notes, req.EvidenceID, number, rejected, 0, a); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`UPDATE stock_transfer_items SET accepted_qty_base=accepted_qty_base+$1,rejected_qty_base=rejected_qty_base+$2,received_qty_base=accepted_qty_base+$1 WHERE transfer_id=$3 AND item_id=$4`, accepted, rejected, id, it.ItemID); err != nil {
			return err
		}
		processed += accepted + rejected
	}
	if processed <= 0 {
		return Invalid("isi minimal satu jumlah diterima atau reject")
	}
	var pending, rejected float64
	if err = tx.QueryRow(`SELECT SUM(qty_base-accepted_qty_base-rejected_qty_base),SUM(rejected_qty_base) FROM stock_transfer_items WHERE transfer_id=$1`, id).Scan(&pending, &rejected); err != nil {
		return err
	}
	status = "partial_received"
	if round4(pending) == 0 {
		status = "received"
		if rejected > 0 {
			status = "received_rejected"
		}
	}
	if _, err = tx.Exec(`UPDATE stock_transfers SET status=$2,receipt_version=receipt_version+1,received_by=$3,received_at=NOW(),updated_at=NOW() WHERE id=$1`, id, status, a.ActorName); err != nil {
		return err
	}
	items, err := json.Marshal(req.Items)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO stock_transfer_receipts(id,transfer_id,evidence_id,actor_id,actor_name,notes,items) VALUES($1,$2,$3,$4,$5,$6,$7)`, receiptID, id, req.EvidenceID, a.ActorID, a.ActorName, req.Notes, items); err != nil {
		return err
	}
	return tx.Commit()
}

func loadStockTransferReceipts(t *models.StockTransfer) error {
	if err := database.DB.QueryRow(`SELECT receipt_version FROM stock_transfers WHERE id=$1`, t.ID).Scan(&t.ReceiptVersion); err != nil {
		return err
	}
	rows, err := database.DB.Query(`SELECT id,accepted_qty_base,rejected_qty_base FROM stock_transfer_items WHERE transfer_id=$1`, t.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var good, bad float64
		if err := rows.Scan(&id, &good, &bad); err != nil {
			rows.Close()
			return err
		}
		for i := range t.Items {
			if t.Items[i].ID == id {
				t.Items[i].AcceptedQtyBase = good
				t.Items[i].RejectedQtyBase = bad
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = database.DB.Query(`SELECT r.id,r.actor_name,r.notes,r.created_at::text,r.items,p.id,p.url,p.recorded_at::text,p.actor_name FROM stock_transfer_receipts r JOIN stock_evidence p ON p.id=r.evidence_id WHERE r.transfer_id=$1 ORDER BY r.created_at,r.id`, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	t.Receipts = []models.StockTransferReceipt{}
	for rows.Next() {
		var r models.StockTransferReceipt
		var data []byte
		if err := rows.Scan(&r.ID, &r.ActorName, &r.Notes, &r.CreatedAt, &data, &r.Evidence.ID, &r.Evidence.URL, &r.Evidence.RecordedAt, &r.Evidence.ActorName); err != nil {
			return err
		}
		if err := json.Unmarshal(data, &r.Items); err != nil {
			return err
		}
		t.Receipts = append(t.Receipts, r)
	}
	return rows.Err()
}
