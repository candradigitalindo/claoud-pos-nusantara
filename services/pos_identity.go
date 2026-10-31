package services

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/oklog/ulid/v2"
)

// reservePOSID keeps existing canonical IDs and the historical local-ID value
// for the first outlet. A colliding outlet receives a stable 26-character ID.
// Reserving references as well as parents makes child-before-parent sync safe.
// Callers commit this reservation in the same transaction as the business row.
func reservePOSID(tx *sql.Tx, entity, outletID, localID string) (string, error) {
	tables := map[string]string{
		"order": "cloud_orders", "transaction": "cloud_transactions",
		"cashier_shift": "cloud_cashier_shifts", "cash_movement": "cloud_cash_movements",
		"printer": "cloud_printers",
		"product": "cloud_products", "category": "cloud_categories",
		"product_addon": "cloud_product_addons", "additional_charge": "cloud_additional_charges",
	}
	table, ok := tables[entity]
	if !ok {
		return "", fmt.Errorf("jenis identitas POS tidak dikenal: %s", entity)
	}
	localID = strings.TrimSpace(localID)
	if localID == "" || utf8.RuneCountInString(localID) > 50 {
		return "", fmt.Errorf("local_id %s harus terisi dan maksimal 50 karakter", entity)
	}
	var id string
	lookup := func() error {
		return tx.QueryRow(`SELECT cloud_id FROM pos_entity_ids WHERE entity_type=$1 AND outlet_id=$2 AND local_id=$3`, entity, outletID, localID).Scan(&id)
	}
	if err := lookup(); err == nil {
		return strings.TrimSpace(id), nil
	} else if err != sql.ErrNoRows {
		return "", err
	}
	// Only first allocations take this lock. It also covers a caller-supplied
	// local ID that happens to equal another outlet's generated canonical ID.
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "pos-identity:"+entity); err != nil {
		return "", err
	}
	if err := lookup(); err == nil {
		return strings.TrimSpace(id), nil
	} else if err != sql.ErrNoRows {
		return "", err
	}
	// Adopt an existing canonical ID without changing rows or their references.
	err := tx.QueryRow(`SELECT id FROM `+table+` WHERE outlet_id=$1 AND local_id=$2`, outletID, localID).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if err == sql.ErrNoRows {
		id = localID
		for salt := 0; ; salt++ {
			if salt > 0 || utf8.RuneCountInString(id) > 26 {
				digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d", entity, outletID, localID, salt)))
				var value ulid.ULID
				copy(value[:], digest[:16])
				id = value.String()
			}
			var occupied bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id=$1)
				OR EXISTS(SELECT 1 FROM pos_entity_ids WHERE entity_type=$2 AND cloud_id=$1)`, id, entity).Scan(&occupied); err != nil {
				return "", err
			}
			if !occupied {
				break
			}
		}
	}
	id = strings.TrimSpace(id)
	_, err = tx.Exec(`INSERT INTO pos_entity_ids(entity_type,outlet_id,local_id,cloud_id) VALUES($1,$2,$3,$4)`, entity, outletID, localID, id)
	return id, err
}
