package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

var (
	ErrPpicUsageAccess   = errors.New("gudang di luar cakupan akses Anda")
	ErrPpicUsageConflict = errors.New("batas pemakaian sudah berubah; muat ulang sebelum menyimpan")
	ErrPpicUsageNotFound = errors.New("bahan atau gudang aktif tidak ditemukan")
)

type ppicUsageQuerier interface {
	QueryRow(string, ...interface{}) *sql.Row
}

// ValidatePpicUsageWarehouse deliberately does not share the legacy warehouse
// helper: an outlet-scoped account cannot inspect or change central limits.
func ValidatePpicUsageWarehouse(warehouseID string, scope []string) error {
	return validatePpicUsageWarehouse(database.DB, warehouseID, scope, false)
}

func validatePpicUsageWarehouse(q ppicUsageQuerier, warehouseID string, scope []string, lock bool) error {
	if strings.TrimSpace(warehouseID) == "" {
		return Invalid("pilih gudang terlebih dahulu")
	}
	if scope != nil && len(scope) == 0 {
		return ErrPpicUsageAccess
	}
	query := `SELECT type,COALESCE(outlet_id,'') FROM warehouses WHERE id=$1 AND is_active`
	if lock {
		query += ` FOR SHARE`
	}
	var kind, outlet string
	if err := q.QueryRow(query, warehouseID).Scan(&kind, &outlet); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPpicUsageNotFound
		}
		return err
	}
	if scope == nil {
		return nil
	}
	if kind == "outlet" {
		for _, id := range scope {
			if strings.TrimSpace(outlet) == id {
				return nil
			}
		}
	}
	return ErrPpicUsageAccess
}

func validatePpicUsageItem(q ppicUsageQuerier, itemID string, lock bool) error {
	if strings.TrimSpace(itemID) == "" {
		return Invalid("pilih bahan terlebih dahulu")
	}
	query := `SELECT id FROM stock_items WHERE id=$1 AND is_active`
	if lock {
		query += ` FOR SHARE`
	}
	var found string
	if err := q.QueryRow(query, itemID).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPpicUsageNotFound
		}
		return err
	}
	return nil
}

func GetPpicUsageLookups(warehouseID string, scope []string) (*models.PpicUsageLookups, error) {
	r := &models.PpicUsageLookups{Warehouses: []models.PpicUsageWarehouse{}, Items: []models.PpicUsageItem{}}
	if warehouseID != "" {
		if err := ValidatePpicUsageWarehouse(warehouseID, scope); err != nil {
			return nil, err
		}
	}
	rows, err := database.DB.Query(`SELECT w.id,w.name,w.type,COALESCE(w.outlet_id,''),COALESCE(o.name,'')
		FROM warehouses w LEFT JOIN outlets o ON o.id=w.outlet_id
		WHERE w.is_active AND ($1 OR (w.type='outlet' AND w.outlet_id=ANY($2)))
		AND ($3='' OR w.id=$3) ORDER BY w.type,w.name,w.id`, scope == nil, pq.Array(scope), warehouseID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var w models.PpicUsageWarehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.OutletID, &w.OutletName); err != nil {
			rows.Close()
			return nil, err
		}
		r.Warehouses = append(r.Warehouses, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(r.Warehouses) == 0 {
		return r, nil
	}
	// Catalog names/units are shared metadata, without stock, prices or recipes.
	items, err := database.DB.Query(`SELECT id,code,name,COALESCE(category,''),base_unit FROM stock_items WHERE is_active ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var item models.PpicUsageItem
		if err := items.Scan(&item.ID, &item.Code, &item.Name, &item.Category, &item.BaseUnit); err != nil {
			return nil, err
		}
		r.Items = append(r.Items, item)
	}
	return r, items.Err()
}

func SavePpicUsagePolicy(req models.PpicUsagePolicyInput, scope []string, actor string) (*models.PpicUsagePolicy, error) {
	req.ItemID, req.WarehouseID, req.Reason = strings.TrimSpace(req.ItemID), strings.TrimSpace(req.WarehouseID), strings.TrimSpace(req.Reason)
	actor = strings.TrimSpace(actor)
	if req.ItemID == "" || req.WarehouseID == "" {
		return nil, Invalid("bahan dan gudang wajib diisi")
	}
	if req.UsageTolerancePct == nil || req.WasteTolerancePct == nil {
		return nil, Invalid("batas pemakaian dan batas waste wajib diisi; nol berarti tanpa toleransi")
	}
	for _, value := range []float64{*req.UsageTolerancePct, *req.WasteTolerancePct} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
			return nil, Invalid("batas persentase harus antara 0 dan 100")
		}
	}
	if req.Version < 0 || req.Version >= 2147483647 {
		return nil, Invalid("versi batas pemakaian tidak valid")
	}
	if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 1000 {
		return nil, Invalid("alasan penetapan/perubahan wajib diisi, maksimal 1000 karakter")
	}
	if actor == "" || utf8.RuneCountInString(actor) > 100 {
		return nil, Invalid("pencatat batas pemakaian tidak valid")
	}
	if scope != nil && len(scope) == 0 {
		return nil, ErrPpicUsageAccess
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := validatePpicUsageWarehouse(tx, req.WarehouseID, scope, true); err != nil {
		return nil, err
	}
	if err := validatePpicUsageItem(tx, req.ItemID, true); err != nil {
		return nil, err
	}
	var row *sql.Row
	columns := `item_id,warehouse_id,usage_tolerance_pct,waste_tolerance_pct,version,reason,updated_by,updated_at`
	if req.Version == 0 {
		row = tx.QueryRow(`INSERT INTO ppic_usage_policies(`+columns+`) VALUES($1,$2,$3,$4,1,$5,$6,NOW())
			ON CONFLICT(item_id,warehouse_id) DO NOTHING RETURNING `+columns,
			req.ItemID, req.WarehouseID, *req.UsageTolerancePct, *req.WasteTolerancePct, req.Reason, actor)
	} else {
		row = tx.QueryRow(`UPDATE ppic_usage_policies SET usage_tolerance_pct=$3,waste_tolerance_pct=$4,
			version=version+1,reason=$5,updated_by=$6,updated_at=NOW()
			WHERE item_id=$1 AND warehouse_id=$2 AND version=$7 RETURNING `+columns,
			req.ItemID, req.WarehouseID, *req.UsageTolerancePct, *req.WasteTolerancePct, req.Reason, actor, req.Version)
	}
	policy, err := scanPpicUsagePolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPpicUsageConflict
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO ppic_usage_policy_history(`+columns+`) SELECT `+columns+`
		FROM ppic_usage_policies WHERE item_id=$1 AND warehouse_id=$2`, req.ItemID, req.WarehouseID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return policy, nil
}

type ppicUsageScanner interface{ Scan(...interface{}) error }

func scanPpicUsagePolicy(row ppicUsageScanner) (*models.PpicUsagePolicy, error) {
	var p models.PpicUsagePolicy
	var at time.Time
	if err := row.Scan(&p.ItemID, &p.WarehouseID, &p.UsageTolerancePct, &p.WasteTolerancePct, &p.Version, &p.Reason, &p.UpdatedBy, &at); err != nil {
		return nil, err
	}
	p.UpdatedAt = at.UTC().Format(time.RFC3339Nano)
	return &p, nil
}

func ListPpicUsagePolicyHistory(itemID, warehouseID string, scope []string) ([]models.PpicUsagePolicy, error) {
	if err := ValidatePpicUsageWarehouse(warehouseID, scope); err != nil {
		return nil, err
	}
	if err := validatePpicUsageItem(database.DB, itemID, false); err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT item_id,warehouse_id,usage_tolerance_pct,waste_tolerance_pct,version,reason,updated_by,updated_at
		FROM ppic_usage_policy_history WHERE item_id=$1 AND warehouse_id=$2 ORDER BY version DESC`, itemID, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.PpicUsagePolicy{}
	for rows.Next() {
		p, err := scanPpicUsagePolicy(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}
	return result, rows.Err()
}
