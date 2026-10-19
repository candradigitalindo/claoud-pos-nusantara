package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/lib/pq"
)

func CreateOutlet(req models.CreateOutletRequest) (*models.Outlet, error) {
	apiKey := GenerateAPIKey()
	outlet := &models.Outlet{}
	id := NewULID()

	err := database.DB.QueryRow(
		`INSERT INTO outlets (id, code, name, address, phone, api_key, webhook_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, code, name, address, COALESCE(phone,''), api_key, COALESCE(webhook_url,''), is_active, created_at, updated_at`,
		id, req.Code, req.Name, req.Address, req.Phone, apiKey, req.WebhookURL,
	).Scan(&outlet.ID, &outlet.Code, &outlet.Name, &outlet.Address,
		&outlet.Phone, &outlet.APIKey, &outlet.WebhookURL, &outlet.IsActive,
		&outlet.CreatedAt, &outlet.UpdatedAt)

	if err != nil {
		return nil, err
	}

	// Auto-create work unit for this outlet (jangan diam-diam: catat bila gagal,
	// karena kegagalan tersembunyi sempat membuat outlet tanpa unit kerja).
	if e := CreateWorkUnitForOutlet(outlet.ID, outlet.Name); e != nil {
		log.Printf("auto-create work unit gagal untuk outlet %s (%s): %v", outlet.Name, outlet.ID, e)
	}

	// Auto-create gudang outlet
	if e := CreateOutletWarehouse(outlet.ID, outlet.Name, outlet.Code); e != nil {
		log.Printf("auto-create gudang gagal untuk outlet %s (%s): %v", outlet.Name, outlet.ID, e)
	}

	return outlet, nil
}

func GetOutlets() ([]models.Outlet, error) {
	return GetOutletsScoped(nil)
}

// GetOutletsScoped returns outlets filtered by the given IDs. Pass nil for all outlets.
func GetOutletsScoped(outletIDs []string) ([]models.Outlet, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if outletIDs != nil {
		rows, err = database.DB.Query(
			`SELECT id, code, COALESCE(slug,''), name, COALESCE(address,''), COALESCE(phone,''), COALESCE(webhook_url,''), is_active, created_at, updated_at
			FROM outlets WHERE id = ANY($1) ORDER BY created_at DESC`,
			pq.Array(outletIDs),
		)
	} else {
		rows, err = database.DB.Query(
			`SELECT id, code, COALESCE(slug,''), name, COALESCE(address,''), COALESCE(phone,''), COALESCE(webhook_url,''), is_active, created_at, updated_at
			FROM outlets ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	outlets := make([]models.Outlet, 0)
	for rows.Next() {
		var o models.Outlet
		if err := rows.Scan(&o.ID, &o.Code, &o.Slug, &o.Name, &o.Address,
			&o.Phone, &o.WebhookURL, &o.IsActive, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		outlets = append(outlets, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outlets, nil
}

// GetMyScopeOutlets returns the outlets accessible to the current user based on scope.
// For "all" scope (both nil), returns all outlets.
// For "specific" scope, returns outlets matching outletIDs OR linked via work unit IDs.
func GetMyScopeOutlets(outletIDs []string, wuIDs []string) ([]models.Outlet, error) {
	if outletIDs == nil && wuIDs == nil {
		return GetOutletsScoped(nil) // all scope
	}

	// Collect outlet IDs from direct scope + work unit→outlet mapping
	idSet := make(map[string]bool)
	for _, id := range outletIDs {
		idSet[id] = true
	}

	// Resolve work unit → outlet_id
	if len(wuIDs) > 0 {
		rows, err := database.DB.Query(
			`SELECT DISTINCT outlet_id FROM work_units WHERE id = ANY($1) AND outlet_id IS NOT NULL`,
			pq.Array(wuIDs),
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var oid string
				if rows.Scan(&oid) == nil && oid != "" {
					idSet[oid] = true
				}
			}
		}
	}

	if len(idSet) == 0 {
		return []models.Outlet{}, nil
	}

	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return GetOutletsScoped(ids)
}

func GetOutlet(id string) (*models.Outlet, error) {
	o := &models.Outlet{}
	err := database.DB.QueryRow(
		`SELECT TRIM(id), code, COALESCE(slug,''), name, COALESCE(address,''), COALESCE(phone,''), COALESCE(api_key,''), COALESCE(webhook_url,''), is_active, created_at, updated_at
		FROM outlets WHERE TRIM(id) = $1`, strings.TrimSpace(id),
	).Scan(&o.ID, &o.Code, &o.Slug, &o.Name, &o.Address,
		&o.Phone, &o.APIKey, &o.WebhookURL, &o.IsActive, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func UpdateOutlet(id string, req models.UpdateOutletRequest) (*models.Outlet, error) {
	o := &models.Outlet{}
	err := database.DB.QueryRow(
		`UPDATE outlets SET name = $1, address = $2, phone = $3, webhook_url = $4, updated_at = NOW()
		WHERE TRIM(id) = $5
		RETURNING id, code, name, COALESCE(address,''), COALESCE(phone,''), COALESCE(api_key,''), COALESCE(webhook_url,''), is_active, created_at, updated_at`,
		req.Name, req.Address, req.Phone, req.WebhookURL, strings.TrimSpace(id),
	).Scan(&o.ID, &o.Code, &o.Name, &o.Address,
		&o.Phone, &o.APIKey, &o.WebhookURL, &o.IsActive, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func RegenerateAPIKey(id string) (string, error) {
	newKey := GenerateAPIKey()
	result, err := database.DB.Exec(
		`UPDATE outlets SET api_key = $1, updated_at = NOW() WHERE id = $2`,
		newKey, id,
	)
	if err != nil {
		return "", err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return "", fmt.Errorf("outlet not found")
	}
	return newKey, nil
}

func ToggleOutlet(id string) (*models.Outlet, error) {
	o := &models.Outlet{}
	err := database.DB.QueryRow(
		`UPDATE outlets SET is_active = NOT is_active, updated_at = NOW()
		WHERE id = $1
		RETURNING id, code, name, COALESCE(address,''), COALESCE(phone,''), COALESCE(webhook_url,''), is_active, created_at, updated_at`,
		id,
	).Scan(&o.ID, &o.Code, &o.Name, &o.Address,
		&o.Phone, &o.WebhookURL, &o.IsActive, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func DeleteOutlet(id string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// asset_maintenances tidak punya kolom outlet_id — bersihkan via aset outlet-nya
	// sebelum barisnya sendiri ikut terhapus di loop bawah.
	if _, err := tx.Exec(`DELETE FROM asset_maintenances WHERE asset_id IN (SELECT id FROM assets WHERE outlet_id = $1)`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("failed to delete asset maintenances: %w", err)
	}

	relatedTables := []string{
		"outlet_cameras",
		"outlet_cctv_gateways",
		"cloud_cash_movements",
		"cloud_cashier_shifts",
		"sync_conflicts",
		"sync_logs",
		"cloud_analytics",
		"cloud_printers",
		"transaction_payments",
		"order_item_voids",
		"cloud_orders",
		"cloud_transactions",
		"cloud_products",
		"cloud_categories",
		"assets",
		"reservations",
	}
	for _, table := range relatedTables {
		_, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE outlet_id = $1", table), strings.TrimSpace(id))
		if err != nil {
			return fmt.Errorf("failed to delete from %s: %w", table, err)
		}
	}

	result, err := tx.Exec("DELETE FROM outlets WHERE TRIM(id) = $1", strings.TrimSpace(id))
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("outlet not found")
	}
	return tx.Commit()
}
