package services

import (
	"database/sql"

	"cloud-pos/models"
)

// purchaseQuantitiesChanged compares the effective quantities after the
// approved-edit merge has verified the request structure and original qty.
func purchaseQuantitiesChanged(stored, updated []models.PurchaseRequestItem) bool {
	for g := range stored {
		for i := range stored[g].Items {
			if stored[g].Items[i].PurchaseQuantity() != updated[g].Items[i].PurchaseQuantity() {
				return true
			}
		}
	}
	return false
}

func ensurePurchaseQuantitiesEditable(tx *sql.Tx, id string, hasReceipts bool) error {
	// Include pending warehouse decisions and propagate query errors, so a
	// failed receipt lookup cannot unlock quantities already received.
	ownReceipts, err := docHasOwnReceipts(tx, id)
	if err != nil {
		return err
	}
	if hasReceipts || ownReceipts {
		return Invalid("jumlah beli aktual tidak bisa diubah setelah barang mulai diterima")
	}
	var hasFinancialRecords bool
	err = tx.QueryRow(`SELECT
		EXISTS(SELECT 1 FROM payment_histories WHERE purchase_request_id = $1)
		OR EXISTS(SELECT 1 FROM purchase_struks WHERE purchase_request_id = $1)
		OR EXISTS(SELECT 1 FROM purchase_requests WHERE id = $1 AND COALESCE(paid_amount, 0) > 0)`, id).
		Scan(&hasFinancialRecords)
	if err != nil {
		return err
	}
	if hasFinancialRecords {
		return Invalid("jumlah beli aktual tidak bisa diubah setelah struk atau pembayaran dicatat; tetapkan jumlah aktual sebelum mencatat struk")
	}
	return nil
}
