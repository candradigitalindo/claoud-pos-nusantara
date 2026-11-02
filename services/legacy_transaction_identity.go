package services

import (
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// This key denotes one transaction-wide identity task, never a catalog product.
const legacyIdentityProductKey = "__legacy_identity__"

// Recover only from this transaction, an exact item in its same-outlet order,
// or a unique catalog name. Product identity does not depend on recipe validity.
// Missing/ambiguous identity is returned as pending work, not a payment error.
func enrichLegacyTransactionItems(tx *sql.Tx, outletID, txnID, orderID string, incoming []models.TransactionItem) ([]models.TransactionItem, string, error) {
	items := append([]models.TransactionItem(nil), incoming...)
	if len(items) == 0 {
		// Some legacy retries omit details. Keep an already accepted snapshot
		// rather than erasing its lines or resolving unfinished identity work.
		var raw string
		err := tx.QueryRow(`SELECT COALESCE(items::text,'[]') FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, outletID, txnID).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, "", err
		}
		var previous []models.TransactionItem
		if err == nil && json.Unmarshal([]byte(raw), &previous) == nil && len(previous) > 0 {
			return enrichLegacyTransactionItems(tx, outletID, txnID, orderID, previous)
		}
		return items, "", nil
	}
	needsIdentity := false
	for i := range items {
		items[i].ProductID = strings.TrimSpace(items[i].ProductID)
		if items[i].ProductID == legacyIdentityProductKey {
			items[i].ProductID = ""
		}
		if items[i].ProductID == "" && items[i].Quantity > 0 {
			needsIdentity = true
		}
	}
	if !needsIdentity {
		return items, "", nil
	}

	var previous []models.TransactionItem
	var raw string
	err := tx.QueryRow(`SELECT COALESCE(items::text,'[]') FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, outletID, txnID).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return nil, "", err
	}
	if err == nil {
		// An unusable optional snapshot cannot establish a product identity.
		if err := json.Unmarshal([]byte(raw), &previous); err != nil {
			previous = nil
		}
	}
	if legacyReplayMatchesPrevious(items, previous) {
		// Old retries can omit every line ID while modern snapshots contain
		// different products with identical prices/names. Preserve the complete
		// accepted snapshot instead of assigning those IDs to individual lines.
		return previous, "", nil
	}
	var orderItems []models.OrderItem
	if orderID != "" {
		err := tx.QueryRow(`SELECT COALESCE(items::text,'[]') FROM cloud_orders WHERE outlet_id=$1 AND id=$2`, outletID, orderID).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, "", err
		}
		if err == nil {
			if err := json.Unmarshal([]byte(raw), &orderItems); err != nil {
				orderItems = nil
			}
		}
	}

	var unresolved []string
	for i := range items {
		item := &items[i]
		item.ProductID = strings.TrimSpace(item.ProductID)
		if item.ProductID != "" || item.Quantity <= 0 {
			continue
		}
		// A delayed legacy retry must retain an already recovered identity.
		if id := previousTransactionProduct(*item, previous); id != "" {
			item.ProductID = id
			continue
		}
		if item.ID != "" {
			ids := map[string]bool{}
			for _, candidate := range orderItems {
				if candidate.ID == item.ID && strings.TrimSpace(candidate.ProductID) != "" {
					ids[strings.TrimSpace(candidate.ProductID)] = true
				}
			}
			if len(ids) == 1 {
				for id := range ids {
					localID, err := catalogIdentityByID(tx, outletID, id)
					if err != nil {
						return nil, "", err
					}
					item.ProductID = localID
				}
			}
		}
		if item.ProductID != "" {
			continue
		}
		rows, err := tx.Query(`SELECT local_id FROM cloud_products
			WHERE outlet_id=$1 AND lower(btrim(name))=lower(btrim($2)) AND NOT is_deleted
			ORDER BY id LIMIT 2 FOR SHARE`, outletID, item.ProductName)
		if err != nil {
			return nil, "", err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, "", err
			}
			ids = append(ids, strings.TrimSpace(id))
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, "", err
		}
		if len(ids) == 1 && ids[0] != "" && ids[0] != legacyIdentityProductKey {
			item.ProductID = ids[0]
		} else {
			unresolved = append(unresolved, fmt.Sprintf("item %d %q: identitas produk belum pasti (%d produk cocok)", i+1, item.ProductName, len(ids)))
		}
	}
	return items, strings.Join(unresolved, "; "), nil
}

func legacyReplayMatchesPrevious(items, previous []models.TransactionItem) bool {
	if len(items) == 0 || len(items) != len(previous) {
		return false
	}
	type commercial struct {
		name, waiter    string
		quantity        int
		price, subtotal float64
	}
	key := func(item models.TransactionItem) commercial {
		return commercial{item.ProductName, item.WaiterName, item.Quantity, item.Price, item.Subtotal}
	}
	counts := map[commercial]int{}
	for _, item := range previous {
		if strings.TrimSpace(item.ProductID) == "" || item.ProductID == legacyIdentityProductKey {
			return false
		}
		counts[key(item)]++
	}
	for _, item := range items {
		if item.ID != "" || item.ProductID != "" || counts[key(item)] == 0 {
			return false
		}
		counts[key(item)]--
	}
	return true
}

func previousTransactionProduct(item models.TransactionItem, previous []models.TransactionItem) string {
	ids := map[string]bool{}
	incomplete := false
	for _, old := range previous {
		match := item.ID != "" && old.ID == item.ID
		if item.ID == "" {
			// Exact commercial snapshot only; matching array positions is unsafe
			// when an old client reorders otherwise identical transaction lines.
			match = item.ProductName == old.ProductName && item.Quantity == old.Quantity &&
				item.Price == old.Price && item.Subtotal == old.Subtotal && item.WaiterName == old.WaiterName
		}
		if match {
			id := strings.TrimSpace(old.ProductID)
			if id == "" || id == legacyIdentityProductKey {
				incomplete = true
			} else {
				ids[id] = true
			}
		}
	}
	if !incomplete && len(ids) == 1 {
		for id := range ids {
			return id
		}
	}
	return ""
}

func catalogIdentityByID(tx *sql.Tx, outletID, productID string) (string, error) {
	rows, err := tx.Query(`SELECT local_id FROM cloud_products
		WHERE outlet_id=$1 AND (local_id=$2::text OR id::text=$2::text)
		ORDER BY id LIMIT 2 FOR SHARE`, outletID, productID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, strings.TrimSpace(id))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) == 1 && ids[0] != legacyIdentityProductKey {
		return ids[0], nil
	}
	return "", nil
}

func recordLegacyIdentityFailure(tx *sql.Tx, outletID, txnID, reason string, original []models.TransactionItem) error {
	snapshot, err := json.Marshal(original)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO stock_deduction_failures
		(id,outlet_id,transaction_id,product_local_id,qty,last_error,failure_kind,item_snapshot)
		VALUES($1,$2,$3,$4,1,$5,'identity',$6)
		ON CONFLICT(transaction_id,product_local_id) DO UPDATE SET
			last_error=EXCLUDED.last_error,status='pending',failure_kind='identity',
			item_snapshot=COALESCE(stock_deduction_failures.item_snapshot,EXCLUDED.item_snapshot),
			attempts=0,last_attempt_at=NULL`, NewULID(), outletID, txnID, legacyIdentityProductKey, reason, string(snapshot))
	return err
}

func resolveLegacyIdentityFailure(tx *sql.Tx, outletID, txnID string) error {
	_, err := tx.Exec(`UPDATE stock_deduction_failures SET status='resolved',last_error='',
		last_attempt_at=NOW() AT TIME ZONE 'UTC'
		WHERE outlet_id=$1 AND transaction_id=$2 AND failure_kind='identity' AND status='pending'`, outletID, txnID)
	return err
}

// All identities must be known before aggregate per-product receipts are written.
// Otherwise two deferred lines resolving to one product could deduct it twice.
func syncTransactionProductStock(tx *sql.Tx, outletID, txnID, localID string, items []models.TransactionItem) error {
	quantities := map[string]float64{}
	for _, item := range items {
		if item.ProductID == legacyIdentityProductKey {
			return fmt.Errorf("kunci antrean identitas bukan ID produk")
		}
		if item.ProductID != "" && item.Quantity > 0 {
			quantities[item.ProductID] += float64(item.Quantity)
		}
	}
	products := make([]string, 0, len(quantities))
	for product := range quantities {
		products = append(products, product)
	}
	sort.Strings(products)
	for _, product := range products {
		if _, err := tx.Exec(`SAVEPOINT pos_product_stock`); err != nil {
			return err
		}
		if derr := deductPOSProduct(tx, outletID, product, quantities[product], txnID, localID); derr != nil {
			if _, err := tx.Exec(`ROLLBACK TO SAVEPOINT pos_product_stock`); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO stock_deduction_failures(id,outlet_id,transaction_id,product_local_id,qty,last_error)
				VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(transaction_id,product_local_id)
				DO UPDATE SET last_error=EXCLUDED.last_error,qty=EXCLUDED.qty,status='pending'`, NewULID(), outletID, txnID, product, quantities[product], derr.Error()); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(`UPDATE stock_deduction_failures SET status='resolved'
				WHERE outlet_id=$1 AND transaction_id=$2 AND product_local_id=$3 AND failure_kind='stock' AND status='pending'`, outletID, txnID, product); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`RELEASE SAVEPOINT pos_product_stock`); err != nil {
			return err
		}
	}
	return nil
}

func retryLegacyTransactionIdentity(tx *sql.Tx, failureID, outletID, txnID string) error {
	var localID, orderID, raw string
	if err := tx.QueryRow(`SELECT local_id,COALESCE(order_id,''),COALESCE(items::text,'[]')
		FROM cloud_transactions WHERE outlet_id=$1 AND id=$2`, outletID, txnID).Scan(&localID, &orderID, &raw); err != nil {
		return err
	}
	var items []models.TransactionItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return err
	}
	items, reason, err := enrichLegacyTransactionItems(tx, outletID, txnID, orderID, items)
	if err != nil {
		return err
	}
	if reason != "" {
		return fmt.Errorf("%s", reason)
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE cloud_transactions SET items=$3 WHERE outlet_id=$1 AND id=$2`, outletID, txnID, string(encoded)); err != nil {
		return err
	}
	if err := syncTransactionProductStock(tx, outletID, txnID, localID, items); err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE stock_deduction_failures SET status='resolved',last_error='',
		attempts=attempts+1,last_attempt_at=NOW() AT TIME ZONE 'UTC'
		WHERE id=$1 AND outlet_id=$2 AND transaction_id=$3 AND failure_kind='identity'`, failureID, outletID, txnID)
	return err
}
