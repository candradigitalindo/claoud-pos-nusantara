package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"
)

// ── Stock Items ───────────────────────────────────────────────

// ListStockItems lists the shared catalog. When warehouseID is provided, each item
// also carries that warehouse's qty, min-stock and avg-cost (0 / not-present if the
// item has never been stocked there).
func ListStockItems(search string, activeOnly bool, warehouseID string, outletScope []string, page, limit int) ([]models.StockItem, int, error) {
	// Scoped users may only inspect a warehouse they can access (central or an
	// in-scope outlet). Otherwise drop the per-warehouse context (no cross-outlet leak).
	if outletScope != nil && warehouseID != "" {
		var ok bool
		database.DB.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND (type='central' OR outlet_id = ANY($2)))`,
			warehouseID, pq.Array(outletScope)).Scan(&ok)
		if !ok {
			warehouseID = ""
		}
	}

	// COUNT (no join needed)
	cond := "WHERE 1=1"
	cargs := []interface{}{}
	ci := 1
	if search != "" {
		cond += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d)", ci, ci+1)
		cargs = append(cargs, "%"+search+"%", "%"+search+"%")
		ci += 2
	}
	if activeOnly {
		cond += fmt.Sprintf(" AND si.is_active = $%d", ci)
		cargs = append(cargs, true)
		ci++
	}
	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM stock_items si "+cond, cargs...).Scan(&total)

	// Main query — $1 is the warehouse for the LEFT JOIN.
	args := []interface{}{warehouseID}
	i := 2
	// total_stock: scoped users only sum warehouses they can access (their outlets + central).
	totalExpr := "(SELECT COALESCE(SUM(qty_base), 0) FROM stock_ledger WHERE item_id = si.id)"
	if outletScope != nil {
		totalExpr = fmt.Sprintf(`(SELECT COALESCE(SUM(sl2.qty_base),0) FROM stock_ledger sl2
			JOIN warehouses w2 ON w2.id = sl2.warehouse_id
			WHERE sl2.item_id = si.id AND (w2.outlet_id = ANY($%d) OR w2.type='central'))`, i)
		args = append(args, pq.Array(outletScope))
		i++
	}
	where := "WHERE 1=1"
	if search != "" {
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d)", i, i+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		i += 2
	}
	if activeOnly {
		where += fmt.Sprintf(" AND si.is_active = $%d", i)
		args = append(args, true)
		i++
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	// avg_cost katalog dihitung dari ledger (rata-rata tertimbang semua gudang) —
	// kolom stock_items.avg_cost tidak pernah ditulis, selalu 0.
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT si.id, si.code, si.name, si.category, si.base_unit, si.dist_unit, si.dist_ratio, si.dist_unit_label,
		       (SELECT COALESCE(SUM(l.qty_base * l.avg_cost) / NULLIF(SUM(l.qty_base), 0), 0)
		        FROM stock_ledger l WHERE l.item_id = si.id AND l.qty_base > 0) AS avg_cost,
		       si.min_stock, si.notes, si.is_active,
		       TO_CHAR(si.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(si.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       %s AS total_stock,
		       COALESCE(sl.qty_base, 0), COALESCE(sl.min_stock, 0), COALESCE(sl.avg_cost, 0), (sl.item_id IS NOT NULL)
		FROM stock_items si
		LEFT JOIN stock_ledger sl ON sl.item_id = si.id AND sl.warehouse_id = $1
		%s ORDER BY si.name ASC LIMIT $%d OFFSET $%d
	`, totalExpr, where, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []models.StockItem{}
	for rows.Next() {
		var s models.StockItem
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Category, &s.BaseUnit, &s.DistUnit,
			&s.DistRatio, &s.DistUnitLabel, &s.AvgCost, &s.MinStock, &s.Notes, &s.IsActive,
			&s.CreatedAt, &s.UpdatedAt, &s.TotalStock,
			&s.WarehouseQty, &s.WarehouseMinStock, &s.WarehouseAvgCost, &s.InWarehouse); err != nil {
			return nil, 0, err
		}
		items = append(items, s)
	}
	return items, total, nil
}

func GetStockItem(id string) (*models.StockItem, error) {
	var s models.StockItem
	err := database.DB.QueryRow(`
		SELECT id, code, name, category, base_unit, dist_unit, dist_ratio, dist_unit_label,
		       (SELECT COALESCE(SUM(l.qty_base * l.avg_cost) / NULLIF(SUM(l.qty_base), 0), 0)
		        FROM stock_ledger l WHERE l.item_id = stock_items.id AND l.qty_base > 0) AS avg_cost,
		       min_stock, notes, is_active,
		       TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       (SELECT COALESCE(SUM(qty_base), 0) FROM stock_ledger WHERE item_id = stock_items.id) AS total_stock
		FROM stock_items WHERE id = $1`, id).Scan(
		&s.ID, &s.Code, &s.Name, &s.Category, &s.BaseUnit, &s.DistUnit,
		&s.DistRatio, &s.DistUnitLabel, &s.AvgCost, &s.MinStock, &s.Notes, &s.IsActive,
		&s.CreatedAt, &s.UpdatedAt, &s.TotalStock)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("bahan baku tidak ditemukan")
	}
	return &s, err
}

func CreateStockItem(req models.StockItemRequest, createdBy string) (*models.StockItem, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}
	if req.Code == "" {
		req.Code = generateStockItemCode(req.Name)
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.BaseUnit == "" {
		req.BaseUnit = "pcs"
	}
	if req.DistUnit == "" {
		req.DistUnit = req.BaseUnit
	}
	if req.DistRatio <= 0 {
		req.DistRatio = 1
	}
	id := NewULID()
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`
		INSERT INTO stock_items (id, code, name, category, base_unit, dist_unit, dist_ratio, dist_unit_label, min_stock, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id, req.Code, req.Name, req.Category, req.BaseUnit, req.DistUnit,
		req.DistRatio, req.DistUnitLabel, req.MinStock, req.Notes)
	if err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("kode bahan baku sudah digunakan")
		}
		return nil, err
	}

	// Per-warehouse: set this warehouse's min-stock and (optionally) opening balance.
	if req.WarehouseID != "" {
		if err := setWarehouseLedgerTx(tx, id, req.WarehouseID, req.WarehouseMinStock); err != nil {
			tx.Rollback()
			return nil, err
		}
		if req.OpeningStock > 0 {
			if err := applyMovement(tx, id, req.WarehouseID, "adjustment", req.BaseUnit, "", "opening", "", "Stok awal", createdBy, req.OpeningStock, 0, req.OpeningCost, ""); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockItem(id)
}

// setWarehouseLedgerTx ensures a ledger row exists for (item, warehouse) and stores
// its per-warehouse minimum stock.
func setWarehouseLedgerTx(tx *sql.Tx, itemID, warehouseID string, minStock float64) error {
	_, err := tx.Exec(`
		INSERT INTO stock_ledger (id, item_id, warehouse_id, qty_base, avg_cost, min_stock)
		VALUES ($1, $2, $3, 0, 0, $4)
		ON CONFLICT (item_id, warehouse_id) DO UPDATE SET min_stock = $4, updated_at = NOW()`,
		NewULID(), itemID, warehouseID, minStock)
	return err
}

func UpdateStockItem(id string, req models.StockItemRequest, createdBy string) (*models.StockItem, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}
	if req.Code == "" {
		req.Code = generateStockItemCode(req.Name)
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.DistRatio <= 0 {
		req.DistRatio = 1
	}

	// The shared catalog (units, distribution, name) may only be edited from a central
	// warehouse context. From an outlet, those fields are read-only — only the outlet's
	// own min-stock / balance can change.
	updateCatalog := true
	if req.WarehouseID != "" {
		var wtype string
		database.DB.QueryRow(`SELECT type FROM warehouses WHERE id=$1`, req.WarehouseID).Scan(&wtype)
		if wtype == "outlet" {
			updateCatalog = false
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}

	if updateCatalog {
		_, err = tx.Exec(`
			UPDATE stock_items SET code=$1, name=$2, category=$3, base_unit=$4, dist_unit=$5,
			  dist_ratio=$6, dist_unit_label=$7, min_stock=$8, notes=$9, updated_at=NOW()
			WHERE id=$10`,
			req.Code, req.Name, req.Category, req.BaseUnit, req.DistUnit,
			req.DistRatio, req.DistUnitLabel, req.MinStock, req.Notes, id)
		if err != nil {
			tx.Rollback()
			if strings.Contains(err.Error(), "unique") {
				return nil, fmt.Errorf("kode bahan baku sudah digunakan")
			}
			return nil, err
		}
	}

	// Per-warehouse: update min-stock and optionally adjust the balance to SetStock.
	if req.WarehouseID != "" {
		if err := setWarehouseLedgerTx(tx, id, req.WarehouseID, req.WarehouseMinStock); err != nil {
			tx.Rollback()
			return nil, err
		}
		if req.SetStock != nil {
			var curQty float64
			tx.QueryRow(`SELECT qty_base FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2`, id, req.WarehouseID).Scan(&curQty)
			delta := *req.SetStock - curQty
			if delta > 0.0001 || delta < -0.0001 {
				baseUnit := req.BaseUnit
				if baseUnit == "" {
					database.DB.QueryRow(`SELECT base_unit FROM stock_items WHERE id=$1`, id).Scan(&baseUnit)
				}
				if err := applyMovement(tx, id, req.WarehouseID, "adjustment", baseUnit, "", "set_stock", "", "Penyesuaian stok", createdBy, delta, 0, req.OpeningCost, ""); err != nil {
					tx.Rollback()
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockItem(id)
}

func ToggleStockItemActive(id string) error {
	_, err := database.DB.Exec(
		`UPDATE stock_items SET is_active = NOT is_active, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func DeleteStockItem(id string) error {
	// Check if referenced in ledger / movements
	var cnt int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_movements WHERE item_id = $1`, id).Scan(&cnt)
	if cnt > 0 {
		return fmt.Errorf("bahan baku sudah memiliki histori pergerakan, tidak dapat dihapus")
	}
	_, err := database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, id)
	return err
}
func generateStockItemCode(name string) string {
	words := strings.Fields(strings.TrimSpace(name))
	if len(words) == 0 {
		words = []string{"BK"}
	}
	var prefix string
	for _, w := range words {
		runes := []rune(w)
		if len(runes) > 0 {
			prefix += strings.ToUpper(string(runes[0]))
		}
	}
	var maxNum int
	_ = database.DB.QueryRow(
		`SELECT COALESCE(MAX(CAST(SUBSTRING(code FROM '^' || $1 || '(\d+)$') AS INTEGER)), 0)
		FROM stock_items WHERE code ~ ('^' || $1 || '\d+$')`,
		prefix,
	).Scan(&maxNum)
	return fmt.Sprintf("%s%d", prefix, maxNum+1)
}
// ── Stock Item Categories ─────────────────────────────────────

func ListStockItemCategories() ([]models.StockItemCategory, error) {
	rows, err := database.DB.Query(`
		SELECT id, name, notes,
		       TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_item_categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cats := []models.StockItemCategory{}
	for rows.Next() {
		var c models.StockItemCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Notes, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

func CreateStockItemCategory(req models.StockItemCategoryRequest) (*models.StockItemCategory, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("nama kategori tidak boleh kosong")
	}
	id := NewULID()
	_, err := database.DB.Exec(
		`INSERT INTO stock_item_categories (id, name, notes) VALUES ($1, $2, $3)`,
		id, strings.TrimSpace(req.Name), req.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("kategori dengan nama ini sudah ada")
		}
		return nil, err
	}
	return GetStockItemCategory(id)
}

func GetStockItemCategory(id string) (*models.StockItemCategory, error) {
	var c models.StockItemCategory
	err := database.DB.QueryRow(`
		SELECT id, name, notes,
		       TO_CHAR(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_item_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.Notes, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("kategori tidak ditemukan")
	}
	return &c, nil
}

func UpdateStockItemCategory(id string, req models.StockItemCategoryRequest) (*models.StockItemCategory, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("nama kategori tidak boleh kosong")
	}
	_, err := database.DB.Exec(
		`UPDATE stock_item_categories SET name=$1, notes=$2 WHERE id=$3`,
		strings.TrimSpace(req.Name), req.Notes, id)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("kategori dengan nama ini sudah ada")
		}
		return nil, err
	}
	return GetStockItemCategory(id)
}

func DeleteStockItemCategory(id string) error {
	var cnt int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_items WHERE category = (SELECT name FROM stock_item_categories WHERE id=$1)`, id).Scan(&cnt)
	if cnt > 0 {
		return fmt.Errorf("kategori sedang digunakan oleh %d bahan baku, tidak dapat dihapus", cnt)
	}
	_, err := database.DB.Exec(`DELETE FROM stock_item_categories WHERE id = $1`, id)
	return err
}

// ── Warehouses ────────────────────────────────────────────────

func ListWarehouses(warehouseType string, outletIDs []string, page, limit int) ([]models.Warehouse, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1
	if warehouseType != "" {
		where += fmt.Sprintf(" AND w.type = $%d", i)
		args = append(args, warehouseType)
		i++
	}
	if outletIDs != nil {
		where += fmt.Sprintf(" AND w.outlet_id = ANY($%d)", i)
		args = append(args, pq.Array(outletIDs))
		i++
	}
	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouses w "+where, args...).Scan(&total)
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT w.id, w.code, w.name, w.type, COALESCE(w.outlet_id,''), COALESCE(o.name,''),
		       w.is_active, w.notes, COALESCE(w.address,''),
		       TO_CHAR(w.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(w.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM warehouses w
		LEFT JOIN outlets o ON o.id = w.outlet_id
		%s ORDER BY w.type, w.name LIMIT $%d OFFSET $%d
	`, where, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	whs := []models.Warehouse{}
	for rows.Next() {
		var w models.Warehouse
		if err := rows.Scan(&w.ID, &w.Code, &w.Name, &w.Type, &w.OutletID, &w.OutletName,
			&w.IsActive, &w.Notes, &w.Address, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, err
		}

		whs = append(whs, w)
	}
	return whs, total, nil
}

func GetWarehouse(id string) (*models.Warehouse, error) {
	var w models.Warehouse
	err := database.DB.QueryRow(`
		SELECT w.id, w.code, w.name, w.type, COALESCE(w.outlet_id,''), COALESCE(o.name,''),
		       w.is_active, w.notes, COALESCE(w.address,''),
		       TO_CHAR(w.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(w.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM warehouses w LEFT JOIN outlets o ON o.id = w.outlet_id
		WHERE w.id = $1`, id).Scan(
		&w.ID, &w.Code, &w.Name, &w.Type, &w.OutletID, &w.OutletName,
		&w.IsActive, &w.Notes, &w.Address, &w.CreatedAt, &w.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("gudang tidak ditemukan")
	}
	return &w, err
}

func ensureOutletWarehouseUniqueness(outletID, excludeWarehouseID string) error {
	if outletID == "" {
		return nil
	}

	var exists bool
	err := database.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1
			FROM warehouses
			WHERE type = 'outlet'
			  AND outlet_id = $1
			  AND ($2 = '' OR id <> $2)
		)
	`, outletID, excludeWarehouseID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("setiap outlet hanya boleh memiliki satu gudang outlet")
	}
	return nil
}

func isOutletWarehouseUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "idx_warehouses_unique_outlet_warehouse")
}

// CreateOutletWarehouse membuat gudang outlet secara otomatis saat outlet dibuat.
// Idempotent — tidak error jika gudang sudah ada.
func CreateOutletWarehouse(outletID, outletName, outletCode string) error {
	// Skip jika sudah ada
	var exists bool
	database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM warehouses WHERE outlet_id=$1 AND type='outlet')`, outletID).Scan(&exists)
	if exists {
		return nil
	}
	code := strings.ToUpper(outletCode) + "-WH"
	name := "Gudang " + outletName
	id := NewULID()
	_, err := database.DB.Exec(`
		INSERT INTO warehouses (id, code, name, type, outlet_id, notes, address)
		VALUES ($1,$2,$3,'outlet',$4,'','')
		ON CONFLICT DO NOTHING`,
		id, code, name, outletID)
	return err
}

func CreateWarehouse(req models.WarehouseRequest) (*models.Warehouse, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.OutletID = strings.TrimSpace(req.OutletID)
	if req.Code == "" || req.Name == "" {
		return nil, fmt.Errorf("kode dan nama gudang wajib diisi")
	}
	if req.Type != "central" && req.Type != "outlet" {
		return nil, fmt.Errorf("tipe gudang harus 'central' atau 'outlet'")
	}
	if req.Type == "outlet" && req.OutletID == "" {
		return nil, fmt.Errorf("gudang outlet wajib memilih outlet")
	}
	if req.Type == "central" {
		req.OutletID = ""
	}
	if req.OutletID != "" {
		var exists bool
		if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM outlets WHERE id = $1)`, req.OutletID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("outlet tidak ditemukan")
		}
	}
	if req.Type == "outlet" {
		if err := ensureOutletWarehouseUniqueness(req.OutletID, ""); err != nil {
			return nil, err
		}
	}
	var outletID interface{} = nil
	if req.OutletID != "" {
		outletID = req.OutletID
	}
	id := NewULID()
	_, err := database.DB.Exec(`
		INSERT INTO warehouses (id, code, name, type, outlet_id, notes, address)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, req.Code, req.Name, req.Type, outletID, req.Notes, req.Address)

	if err != nil {
		if isOutletWarehouseUniqueViolation(err) {
			return nil, fmt.Errorf("setiap outlet hanya boleh memiliki satu gudang outlet")
		}
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("kode gudang sudah digunakan")
		}
		return nil, err
	}
	return GetWarehouse(id)
}

func UpdateWarehouse(id string, req models.WarehouseRequest) (*models.Warehouse, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.OutletID = strings.TrimSpace(req.OutletID)
	if req.Code == "" || req.Name == "" {
		return nil, fmt.Errorf("kode dan nama gudang wajib diisi")
	}
	if req.Type != "central" && req.Type != "outlet" {
		return nil, fmt.Errorf("tipe gudang harus 'central' atau 'outlet'")
	}
	if req.Type == "outlet" && req.OutletID == "" {
		return nil, fmt.Errorf("gudang outlet wajib memilih outlet")
	}
	if req.Type == "central" {
		req.OutletID = ""
	}
	if req.OutletID != "" {
		var exists bool
		if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM outlets WHERE id = $1)`, req.OutletID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("outlet tidak ditemukan")
		}
	}
	if req.Type == "outlet" {
		if err := ensureOutletWarehouseUniqueness(req.OutletID, id); err != nil {
			return nil, err
		}
	}
	var outletID interface{} = nil
	if req.OutletID != "" {
		outletID = req.OutletID
	}
	_, err := database.DB.Exec(`
		UPDATE warehouses SET code=$1, name=$2, type=$3, outlet_id=$4, notes=$5, address=$6, updated_at=NOW()
		WHERE id=$7`,
		req.Code, req.Name, req.Type, outletID, req.Notes, req.Address, id)

	if err != nil {
		if isOutletWarehouseUniqueViolation(err) {
			return nil, fmt.Errorf("setiap outlet hanya boleh memiliki satu gudang outlet")
		}
		if strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("kode gudang sudah digunakan")
		}
		return nil, err
	}
	return GetWarehouse(id)
}

func DeleteWarehouse(id string) error {
	var cnt int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_movements WHERE warehouse_id = $1`, id).Scan(&cnt)
	if cnt > 0 {
		return fmt.Errorf("gudang sudah memiliki histori stok, tidak dapat dihapus")
	}
	_, err := database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, id)
	return err
}

// ── Stock Ledger ──────────────────────────────────────────────

// effMinStock: ambang stok minimum efektif per baris ledger — pakai min_stock
// per-gudang bila diset (>0), selain itu jatuh ke min_stock katalog.
const effMinStock = "COALESCE(NULLIF(sl.min_stock, 0), si.min_stock)"

func GetStockLedger(warehouseID, itemID, search string, lowStockOnly bool, outletIDs []string, page, limit int) (*models.StockLedgerResponse, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1
	if warehouseID != "" {
		where += fmt.Sprintf(" AND sl.warehouse_id = $%d", i)
		args = append(args, warehouseID)
		i++
	}
	if itemID != "" {
		where += fmt.Sprintf(" AND sl.item_id = $%d", i)
		args = append(args, itemID)
		i++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d)", i, i+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		i += 2
	}
	if lowStockOnly {
		where += fmt.Sprintf(" AND %s > 0 AND sl.qty_base < %s", effMinStock, effMinStock)
	}
	if outletIDs != nil {
		where += fmt.Sprintf(" AND w.outlet_id = ANY($%d)", i)
		args = append(args, pq.Array(outletIDs))
		i++
	}

	resp := &models.StockLedgerResponse{
		Data: []models.StockLedgerRow{},
	}

	// Hitung total dan ringkasan
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE %s > 0 AND sl.qty_base < %s),
		       COALESCE(SUM(sl.qty_base * sl.avg_cost), 0)
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id
		JOIN warehouses w ON w.id = sl.warehouse_id %s`, effMinStock, effMinStock, where), args...).Scan(&resp.Total, &resp.LowStockCount, &resp.TotalAssetValue)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT sl.item_id, si.code, si.name, si.base_unit, si.dist_unit, si.dist_ratio, si.dist_unit_label,
		       sl.warehouse_id, w.name, w.type,
		       sl.qty_base,
		       ROUND(sl.qty_base / NULLIF(si.dist_ratio, 0)::numeric, 4)::float8 AS qty_dist,
		       sl.avg_cost,
		       ROUND(sl.qty_base * sl.avg_cost, 2)::float8 AS stock_value,
		       %s AS min_stock,
		       (%s > 0 AND sl.qty_base < %s) AS is_low
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id
		JOIN warehouses w ON w.id = sl.warehouse_id
		%s ORDER BY w.type, si.name LIMIT $%d OFFSET $%d
	`, effMinStock, effMinStock, effMinStock, where, i, i+1), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r models.StockLedgerRow
		if err := rows.Scan(&r.ItemID, &r.ItemCode, &r.ItemName, &r.BaseUnit, &r.DistUnit,
			&r.DistRatio, &r.DistUnitLabel, &r.WarehouseID, &r.WarehouseName, &r.WarehouseType,
			&r.QtyBase, &r.QtyDist, &r.AvgCost, &r.StockValue, &r.MinStock, &r.IsLow); err != nil {
			return nil, err
		}
		resp.Data = append(resp.Data, r)
	}
	return resp, nil
}

// ── Stock Movements ───────────────────────────────────────────

func GetStockMovements(warehouseID, itemID string, dateFrom, dateTo string, outletIDs []string, page, limit int) ([]models.StockMovement, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1
	if warehouseID != "" {
		where += fmt.Sprintf(" AND sm.warehouse_id = $%d", i)
		args = append(args, warehouseID)
		i++
	}
	if itemID != "" {
		where += fmt.Sprintf(" AND sm.item_id = $%d", i)
		args = append(args, itemID)
		i++
	}
	// Batas hari mengikuti timezone aplikasi (kolom tersimpan UTC); tz_day_start
	// dipakai sebagai konstanta di sisi kanan agar index created_at tetap terpakai.
	if dateFrom != "" {
		where += fmt.Sprintf(" AND sm.created_at >= tz_day_start($%d::date)", i)
		args = append(args, dateFrom)
		i++
	}
	if dateTo != "" {
		where += fmt.Sprintf(" AND sm.created_at < tz_day_start($%d::date + 1)", i)
		args = append(args, dateTo)
		i++
	}
	if outletIDs != nil {
		where += fmt.Sprintf(" AND sm.warehouse_id IN (SELECT id FROM warehouses WHERE outlet_id = ANY($%d))", i)
		args = append(args, pq.Array(outletIDs))
		i++
	}

	var total int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_movements sm `+where, args...).Scan(&total)
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT sm.id, sm.item_id, si.name, sm.warehouse_id, w.name,
		       sm.movement_type, sm.qty_base, COALESCE(sm.qty_dist,0), sm.unit_used,
		       sm.cost_per_base, sm.balance_after,
		       COALESCE(sm.ref_id,''), sm.ref_type, sm.ref_number,
		       sm.notes, sm.created_by,
		       TO_CHAR(sm.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(sm.expiry_date, 'YYYY-MM-DD')
		FROM stock_movements sm
		JOIN stock_items si ON si.id = sm.item_id
		JOIN warehouses w ON w.id = sm.warehouse_id
		%s ORDER BY sm.created_at DESC LIMIT $%d OFFSET $%d
	`, where, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []models.StockMovement{}
	for rows.Next() {
		var m models.StockMovement
		var expDate sql.NullString
		if err := rows.Scan(&m.ID, &m.ItemID, &m.ItemName, &m.WarehouseID, &m.WarehouseName,
			&m.MovementType, &m.QtyBase, &m.QtyDist, &m.UnitUsed,
			&m.CostPerBase, &m.BalanceAfter, &m.RefID, &m.RefType, &m.RefNumber,
			&m.Notes, &m.CreatedBy, &m.CreatedAt, &expDate); err != nil {
			return nil, 0, err
		}
		if expDate.Valid {
			m.ExpiryDate = &expDate.String
		}
		result = append(result, m)
	}
	return result, total, nil
}

// applyMovement updates stock_ledger and handles batch-based FIFO tracking.
// qty is in base_unit; positive = stock in, negative = stock out.
func applyMovement(tx *sql.Tx, itemID, warehouseID, movType, unitUsed, refID, refType, refNumber, notes, createdBy string, qtyBase, qtyDist, costPerBase float64, expiryDate string) error {
	// Ensure ledger row exists
	_, err := tx.Exec(`
		INSERT INTO stock_ledger (id, item_id, warehouse_id, qty_base, avg_cost)
		VALUES ($1, $2, $3, 0, 0) ON CONFLICT (item_id, warehouse_id) DO NOTHING`,
		NewULID(), itemID, warehouseID)
	if err != nil {
		return err
	}

	// Lock ledger row
	var curQty, curAvgCost float64
	err = tx.QueryRow(`SELECT qty_base, avg_cost FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2 FOR UPDATE`, itemID, warehouseID).Scan(&curQty, &curAvgCost)
	if err != nil {
		return err
	}

	newQty := curQty + qtyBase
	newAvgCost := curAvgCost
	actualCostPerBase := costPerBase

	if qtyBase > 0 {
		// Stock IN: Create new batch
		var expiry interface{} = nil
		if expiryDate != "" {
			expiry = expiryDate
		}
		_, err = tx.Exec(`
			INSERT INTO stock_batches (id, item_id, warehouse_id, qty_base, cost_per_base, expiry_date, ref_id, ref_type)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			NewULID(), itemID, warehouseID, qtyBase, costPerBase, expiry, refID, refType)
		if err != nil {
			return fmt.Errorf("failed to create stock batch: %w", err)
		}

		// Update moving average (for reference)
		if newQty > 0 && costPerBase > 0 {
			newAvgCost = (curQty*curAvgCost + qtyBase*costPerBase) / newQty
		}
	} else if qtyBase < 0 {
		// Stock OUT: FIFO deduction
		absQty := -qtyBase
		totalDeductedCost := 0.0
		remainingToDeduct := absQty

		// FEFO: batch ber-expiry terdekat dipotong lebih dulu (batch tanpa expiry
		// paling akhir), baru urut umur — expiry dicatat per batch dan dimonitor
		// PPIC, jadi konsumsi harus mengikutinya. Tie-break id (ULID ~ waktu).
		rows, err := tx.Query(`
			SELECT id, qty_base, cost_per_base
			FROM stock_batches
			WHERE item_id=$1 AND warehouse_id=$2 AND qty_base > 0
			ORDER BY (expiry_date IS NULL), expiry_date ASC, created_at ASC, id ASC`, itemID, warehouseID)
		if err != nil {
			return err
		}
		
		type batchUpdate struct {
			id  string
			qty float64
		}
		var updates []batchUpdate

		for rows.Next() && remainingToDeduct > 0 {
			var bid string
			var bqty, bcost float64
			if err := rows.Scan(&bid, &bqty, &bcost); err != nil {
				rows.Close()
				return err
			}

			deduct := bqty
			if deduct > remainingToDeduct {
				deduct = remainingToDeduct
			}

			updates = append(updates, batchUpdate{id: bid, qty: bqty - deduct})
			totalDeductedCost += deduct * bcost
			remainingToDeduct -= deduct
		}
		rows.Close()

		if remainingToDeduct > 0.0001 { // allow small float epsilon
			return fmt.Errorf("stok tidak mencukupi untuk FIFO (kurang %.4f)", remainingToDeduct)
		}

		// Execute batch updates
		for _, up := range updates {
			if up.qty <= 0 {
				_, err = tx.Exec(`DELETE FROM stock_batches WHERE id=$1`, up.id)
			} else {
				_, err = tx.Exec(`UPDATE stock_batches SET qty_base=$1 WHERE id=$2`, up.qty, up.id)
			}
			if err != nil {
				return err
			}
		}

		// Calculate actual cost for the movement record
		actualCostPerBase = totalDeductedCost / absQty
	}

	_, err = tx.Exec(`UPDATE stock_ledger SET qty_base=$1, avg_cost=$2, updated_at=NOW() WHERE item_id=$3 AND warehouse_id=$4`,
		newQty, newAvgCost, itemID, warehouseID)
	if err != nil {
		return err
	}

	var refIDArg interface{} = nil
	if refID != "" {
		refIDArg = refID
	}
	var expDateArg interface{} = nil
	if expiryDate != "" {
		expDateArg = expiryDate
	}

	_, err = tx.Exec(`
		INSERT INTO stock_movements (id, item_id, warehouse_id, movement_type, qty_base, qty_dist, unit_used, cost_per_base, balance_after, ref_id, ref_type, ref_number, notes, created_by, expiry_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		NewULID(), itemID, warehouseID, movType, qtyBase, qtyDist, unitUsed, actualCostPerBase, newQty, refIDArg, refType, refNumber, notes, createdBy, expDateArg)
	return err
}

// CreateAdjustment records a manual stock movement.
func CreateAdjustment(req models.AdjustmentRequest, createdBy string) error {
	allowed := map[string]bool{
		"purchase_in": true, "adjustment": true, "waste": true, 
		"return_in": true, "spoiled": true, "expired": true,
	}
	if !allowed[req.MovType] {
		return fmt.Errorf("tipe movement tidak valid")
	}
	if req.QtyBase == 0 {
		return fmt.Errorf("qty tidak boleh 0")
	}
	
	// For stock-out types, ensure negative
	outTypes := map[string]bool{"waste": true, "spoiled": true, "expired": true}
	if outTypes[req.MovType] && req.QtyBase > 0 {
		req.QtyBase = -req.QtyBase
	}

	// Get item for unit info
	item, err := GetStockItem(req.ItemID)
	if err != nil {
		return err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtyDist := req.QtyBase / item.DistRatio

	if err := applyMovement(tx, req.ItemID, req.WarehouseID, req.MovType, item.BaseUnit,
		"", "manual", "", req.Notes, createdBy, req.QtyBase, qtyDist, req.CostPerBase, req.ExpiryDate); err != nil {
		return err
	}
	return tx.Commit()
}

// DeductStockByRecipe deducts raw materials from a warehouse based on the products sold in a transaction.
func DeductStockByRecipe(outletID string, productLocalID string, qtySold float64, refID, refNumber string) error {
	// 1. Find the warehouse for this outlet
	var warehouseID string
	err := database.DB.QueryRow(`SELECT id FROM warehouses WHERE outlet_id = $1 AND type = 'outlet' AND is_active = true`, outletID).Scan(&warehouseID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	// 2. Get product info (type and link)
	var stockType string
	var linkedItemID sql.NullString
	var recipeMasterID sql.NullString
	var productCloudID string
	var productName string
	err = database.DB.QueryRow(`SELECT id, name, stock_type, linked_stock_item_id, recipe_master_id FROM cloud_products WHERE outlet_id = $1 AND local_id = $2`, outletID, productLocalID).Scan(&productCloudID, &productName, &stockType, &linkedItemID, &recipeMasterID)
	if err != nil {
		return nil // Skip if product not found
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if stockType == "single" && linkedItemID.Valid {
		// ... existing code for single ...
		var itemID = linkedItemID.String
		var baseUnit string
		if err := tx.QueryRow(`SELECT base_unit FROM stock_items WHERE id = $1`, itemID).Scan(&baseUnit); err != nil {
			return fmt.Errorf("bahan baku %s tidak ditemukan: %w", itemID, err)
		}
		
		if err := applyMovement(tx, itemID, warehouseID, "sale", baseUnit,
			refID, "transaction", refNumber, "Sale of single product", "system", -qtySold, -qtySold, 0, ""); err != nil {
			return fmt.Errorf("failed to deduct direct stock for %s: %w", productName, err)
		}
	} else if stockType == "recipe" {
		// --- CASE B: RECIPE PRODUCT ---
		var rows *sql.Rows
		var err error

		if recipeMasterID.Valid && recipeMasterID.String != "" {
			// USE MASTER RECIPE
			rows, err = tx.Query(`
				SELECT ri.item_id, ri.qty_base, si.name, si.base_unit, si.dist_ratio
				FROM recipe_items ri
				JOIN stock_items si ON si.id = ri.item_id
				WHERE ri.recipe_master_id = $1`, recipeMasterID.String)
		} else {
			// USE CUSTOM PRODUCT RECIPE (legacy/fallback)
			rows, err = tx.Query(`
				SELECT pr.item_id, pr.qty_base, si.name, si.base_unit, si.dist_ratio
				FROM product_recipes pr
				JOIN stock_items si ON si.id = pr.item_id
				WHERE pr.product_id = $1`, productCloudID)
		}

		if err != nil {
			return err
		}

		// Baca semua bahan resep ke slice DULU sebelum memanggil applyMovement.
		// lib/pq tidak mengizinkan query lain pada tx selama result set `rows`
		// masih terbuka di koneksi yang sama.
		type recipeRow struct {
			id, name, unit  string
			qtyBase, ratio  float64
		}
		var recipeRows []recipeRow
		for rows.Next() {
			var rr recipeRow
			if err := rows.Scan(&rr.id, &rr.qtyBase, &rr.name, &rr.unit, &rr.ratio); err != nil {
				rows.Close()
				return fmt.Errorf("gagal scan bahan resep: %w", err)
			}
			recipeRows = append(recipeRows, rr)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, rr := range recipeRows {
			totalQty := rr.qtyBase * qtySold
			distQty := totalQty
			if rr.ratio > 0 {
				distQty = totalQty / rr.ratio
			}
			if err := applyMovement(tx, rr.id, warehouseID, "sale", rr.unit,
				refID, "transaction", refNumber, "Recipe deduction from sale", "system", -totalQty, -distQty, 0, ""); err != nil {
				return fmt.Errorf("failed recipe deduction for %s: %w", rr.name, err)
			}
		}
	}

	return tx.Commit()
}

// RetryFailedStockDeductions mencoba ulang deduksi stok penjualan yang gagal
// (tercatat di stock_deduction_failures, mis. karena batch FIFO habis saat sync).
// Dipanggil berkala oleh scheduler; berhenti mencoba setelah 20 attempt agar
// baris bermasalah permanen tidak di-loop selamanya (tetap terlihat di tabel).
func RetryFailedStockDeductions() {
	rows, err := database.DB.Query(`
		SELECT id, outlet_id, transaction_id, product_local_id, qty
		FROM stock_deduction_failures
		WHERE status = 'pending' AND attempts < 20
		ORDER BY created_at ASC LIMIT 100`)
	if err != nil {
		return
	}
	type failRow struct {
		id, outletID, txID, productID string
		qty                           float64
	}
	var fails []failRow
	for rows.Next() {
		var f failRow
		if err := rows.Scan(&f.id, &f.outletID, &f.txID, &f.productID, &f.qty); err == nil {
			fails = append(fails, f)
		}
	}
	rows.Close()

	resolved := 0
	for _, f := range fails {
		if derr := DeductStockByRecipe(f.outletID, f.productID, f.qty, f.txID, f.txID); derr != nil {
			database.DB.Exec(`
				UPDATE stock_deduction_failures
				SET attempts = attempts + 1, last_error = $2, last_attempt_at = NOW() AT TIME ZONE 'UTC'
				WHERE id = $1`, f.id, derr.Error())
		} else {
			database.DB.Exec(`
				UPDATE stock_deduction_failures
				SET status = 'resolved', attempts = attempts + 1, last_attempt_at = NOW() AT TIME ZONE 'UTC'
				WHERE id = $1`, f.id)
			resolved++
		}
	}
	if resolved > 0 {
		log.Printf("[stock] retry deduksi: %d dari %d antrian berhasil", resolved, len(fails))
	}
}

// ── Stock Transfers ───────────────────────────────────────────

// nextDocNumber menghasilkan nomor dokumen berurutan "<prefix><seq>" secara aman
// terhadap request bersamaan: advisory lock per-prefix menserialisasi pembacaan
// MAX (COUNT(*)+1 lama bisa bentrok dengan unique constraint saat dua dokumen
// dibuat berbarengan). Lock lepas otomatis saat tx commit/rollback.
func nextDocNumber(tx *sql.Tx, table, column, prefix string) (int, error) {
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, "docnum:"+table); err != nil {
		return 0, err
	}
	var seq int
	err := tx.QueryRow(fmt.Sprintf(
		`SELECT COALESCE(MAX(CAST(SUBSTRING(%s FROM '^' || $1 || '(\d+)$') AS INT)), 0) + 1
		 FROM %s WHERE %s LIKE $1 || '%%'`, column, table, column), prefix).Scan(&seq)
	return seq, err
}

func generateTransferNumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("TRF%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "stock_transfers", "transfer_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

func ListStockTransfers(status, warehouseID string, outletIDs []string, page, limit int) ([]models.StockTransfer, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1
	if status != "" {
		where += fmt.Sprintf(" AND st.status = $%d", i)
		args = append(args, status)
		i++
	}
	if warehouseID != "" {
		where += fmt.Sprintf(" AND (st.from_warehouse_id = $%d OR st.to_warehouse_id = $%d)", i, i+1)
		args = append(args, warehouseID, warehouseID)
		i += 2
	}
	if outletIDs != nil {
		where += fmt.Sprintf(" AND (st.from_warehouse_id IN (SELECT id FROM warehouses WHERE outlet_id = ANY($%d)) OR st.to_warehouse_id IN (SELECT id FROM warehouses WHERE outlet_id = ANY($%d)))", i, i+1)
		args = append(args, pq.Array(outletIDs), pq.Array(outletIDs))
		i += 2
	}

	var total int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_transfers st `+where, args...).Scan(&total)
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT st.id, st.transfer_number, st.from_warehouse_id, fw.name, st.to_warehouse_id, tw.name,
		       st.status, st.notes,
		       COALESCE(st.approved_by,''), COALESCE(TO_CHAR(st.approved_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       COALESCE(st.sent_by,''), COALESCE(TO_CHAR(st.sent_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       COALESCE(st.received_by,''), COALESCE(TO_CHAR(st.received_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       st.created_by,
		       TO_CHAR(st.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(st.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_transfers st
		JOIN warehouses fw ON fw.id = st.from_warehouse_id
		JOIN warehouses tw ON tw.id = st.to_warehouse_id
		%s ORDER BY st.created_at DESC LIMIT $%d OFFSET $%d
	`, where, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []models.StockTransfer{}
	for rows.Next() {
		var t models.StockTransfer
		t.Items = []models.StockTransferItem{}
		if err := rows.Scan(&t.ID, &t.TransferNumber, &t.FromWarehouseID, &t.FromWarehouse,
			&t.ToWarehouseID, &t.ToWarehouse, &t.Status, &t.Notes,
			&t.ApprovedBy, &t.ApprovedAt, &t.SentBy, &t.SentAt,
			&t.ReceivedBy, &t.ReceivedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, t)
	}
	return result, total, nil
}

// WarehouseMutableInScope reports whether the scoped user may MUTATE stock in this
// warehouse: gudang pusat (outlet_id NULL) atau gudang milik outlet dalam scope.
// nil = all. Dipakai untuk menutup mutasi lintas-outlet lewat warehouse_id di body.
func WarehouseMutableInScope(warehouseID string, outletIDs []string) bool {
	if outletIDs == nil {
		return true
	}
	var cnt int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM warehouses WHERE id = $1 AND (outlet_id IS NULL OR outlet_id = ANY($2))`,
		warehouseID, pq.Array(outletIDs)).Scan(&cnt)
	return cnt > 0
}

// TransferInScope reports whether a transfer involves a warehouse the scoped user
// may access (its from- or to-warehouse belongs to an in-scope outlet). nil = all.
func TransferInScope(transferID string, outletIDs []string) bool {
	if outletIDs == nil {
		return true
	}
	var cnt int
	database.DB.QueryRow(`
		SELECT COUNT(*) FROM stock_transfers st
		WHERE st.id = $1 AND (
			st.from_warehouse_id IN (SELECT id FROM warehouses WHERE outlet_id = ANY($2)) OR
			st.to_warehouse_id   IN (SELECT id FROM warehouses WHERE outlet_id = ANY($2)))`,
		transferID, pq.Array(outletIDs)).Scan(&cnt)
	return cnt > 0
}

func GetStockTransfer(id string) (*models.StockTransfer, error) {
	var t models.StockTransfer
	err := database.DB.QueryRow(`
		SELECT st.id, st.transfer_number, st.from_warehouse_id, fw.name, st.to_warehouse_id, tw.name,
		       st.status, st.notes, COALESCE(st.photo_url,''),
		       COALESCE(st.approved_by,''), COALESCE(TO_CHAR(st.approved_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       COALESCE(st.sent_by,''), COALESCE(TO_CHAR(st.sent_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       COALESCE(st.received_by,''), COALESCE(TO_CHAR(st.received_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		       st.created_by,
		       TO_CHAR(st.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       TO_CHAR(st.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_transfers st
		JOIN warehouses fw ON fw.id = st.from_warehouse_id
		JOIN warehouses tw ON tw.id = st.to_warehouse_id
		WHERE st.id = $1`, id).Scan(
		&t.ID, &t.TransferNumber, &t.FromWarehouseID, &t.FromWarehouse,
		&t.ToWarehouseID, &t.ToWarehouse, &t.Status, &t.Notes, &t.PhotoURL,
		&t.ApprovedBy, &t.ApprovedAt, &t.SentBy, &t.SentAt,
		&t.ReceivedBy, &t.ReceivedAt, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transfer tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	// Load items
	rows, err := database.DB.Query(`
		SELECT ti.id, ti.transfer_id, ti.item_id, si.code, si.name,
		       si.base_unit, si.dist_unit, si.dist_ratio, si.dist_unit_label,
		       ti.qty_dist, ti.qty_base, ti.unit_used,
		       ti.received_qty_base, ti.notes
		FROM stock_transfer_items ti
		JOIN stock_items si ON si.id = ti.item_id
		WHERE ti.transfer_id = $1 ORDER BY si.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t.Items = []models.StockTransferItem{}
	for rows.Next() {
		var item models.StockTransferItem
		if err := rows.Scan(&item.ID, &item.TransferID, &item.ItemID, &item.ItemCode, &item.ItemName,
			&item.BaseUnit, &item.DistUnit, &item.DistRatio, &item.DistUnitLabel,
			&item.QtyDist, &item.QtyBase, &item.UnitUsed, &item.ReceivedQtyBase, &item.Notes); err != nil {
			return nil, err
		}
		t.Items = append(t.Items, item)
	}
	return &t, nil
}

func CreateStockTransfer(req models.StockTransferRequest, createdBy string) (*models.StockTransfer, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("minimal 1 item dalam transfer")
	}
	if req.FromWarehouseID == req.ToWarehouseID {
		return nil, fmt.Errorf("gudang asal dan tujuan tidak boleh sama")
	}

	id := NewULID()
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	num, err := generateTransferNumber(tx)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(`
		INSERT INTO stock_transfers (id, transfer_number, from_warehouse_id, to_warehouse_id, status, notes, created_by)
		VALUES ($1,$2,$3,$4,'draft',$5,$6)`,
		id, num, req.FromWarehouseID, req.ToWarehouseID, req.Notes, createdBy)
	if err != nil {
		return nil, err
	}

	for index, it := range req.Items {
		if strings.TrimSpace(it.ItemID) == "" {
			return nil, fmt.Errorf("item transfer baris %d wajib dipilih", index+1)
		}
		if it.QtyDist <= 0 {
			return nil, fmt.Errorf("qty transfer pada baris %d harus lebih dari 0", index+1)
		}
		item, err := GetStockItem(it.ItemID)
		if err != nil {
			return nil, fmt.Errorf("bahan baku %s tidak ditemukan", it.ItemID)
		}
		qtyBase := it.QtyDist * item.DistRatio
		tiID := NewULID()
		_, err = tx.Exec(`
			INSERT INTO stock_transfer_items (id, transfer_id, item_id, qty_dist, qty_base, unit_used, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			tiID, id, it.ItemID, it.QtyDist, qtyBase, item.DistUnit, it.Notes)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockTransfer(id)
}

func UpdateTransferReceivedQty(transferID, itemID string, receivedQtyBase float64) error {
	var status string
	err := database.DB.QueryRow(`SELECT status FROM stock_transfers WHERE id = $1`, transferID).Scan(&status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("transfer tidak ditemukan")
	}
	if err != nil {
		return err
	}
	if status != "sent" {
		return fmt.Errorf("qty penerimaan hanya dapat diubah saat status transfer 'sent'")
	}

	var shippedQty float64
	err = database.DB.QueryRow(`SELECT qty_base FROM stock_transfer_items WHERE transfer_id = $1 AND item_id = $2`, transferID, itemID).Scan(&shippedQty)
	if err == sql.ErrNoRows {
		return fmt.Errorf("item transfer tidak ditemukan")
	}
	if err != nil {
		return err
	}
	if receivedQtyBase < 0 {
		return fmt.Errorf("qty penerimaan tidak boleh negatif")
	}
	if receivedQtyBase > shippedQty {
		return fmt.Errorf("qty penerimaan tidak boleh melebihi qty dikirim")
	}

	_, err = database.DB.Exec(
		`UPDATE stock_transfer_items SET received_qty_base=$1 WHERE transfer_id=$2 AND item_id=$3`,
		receivedQtyBase, transferID, itemID)
	return err
}

// UpdateTransferStatus memperbarui status transfer stok dan melakukan pergerakan stok pada tahap yang sesuai.
// UpdateTransferStatusWithPhoto membungkus UpdateTransferStatus dengan bukti
// foto pengiriman: distribusi gudang induk → gudang outlet wajib berfoto sama
// seperti penerimaan dari purchasing.
func UpdateTransferStatusWithPhoto(id, newStatus, photoURL, actor string) (*models.StockTransfer, error) {
	if newStatus == "sent" && strings.TrimSpace(photoURL) == "" {
		return nil, Invalid("foto barang saat dikirim wajib diunggah")
	}
	t, err := UpdateTransferStatus(id, newStatus, actor)
	if err != nil {
		return nil, err
	}
	if newStatus == "sent" {
		database.DB.Exec(`UPDATE stock_transfers SET photo_url=$1 WHERE id=$2`, photoURL, id)
		SaveHandoverPhoto(database.DB, "distribusi", "dapur", "stock_transfer", id, t.TransferNumber,
			photoURL, "Distribusi gudang induk → "+t.ToWarehouse, actor)
	}
	return t, nil
}

func UpdateTransferStatus(id, newStatus, actor string) (*models.StockTransfer, error) {
	transfer, err := GetStockTransfer(id)
	if err != nil {
		return nil, err
	}

	validFlow := map[string]string{
		"draft":    "approved",
		"approved": "sent",
		"sent":     "received",
	}
	cancelable := map[string]bool{"draft": true, "approved": true}

	if newStatus == "cancelled" {
		if !cancelable[transfer.Status] {
			return nil, fmt.Errorf("transfer status '%s' tidak dapat dibatalkan", transfer.Status)
		}
	} else {
		if validFlow[transfer.Status] != newStatus {
			return nil, fmt.Errorf("transisi status tidak valid: %s → %s", transfer.Status, newStatus)
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Kunci baris & validasi ulang di dalam transaksi. Cek di atas berjalan di luar
	// lock, jadi dua request bersamaan (double-click / retry) bisa sama-sama lolos
	// dan menerapkan mutasi stok dua kali tanpa guard ini.
	var curStatus string
	if err := tx.QueryRow(`SELECT status FROM stock_transfers WHERE id=$1 FOR UPDATE`, id).Scan(&curStatus); err != nil {
		return nil, err
	}
	if newStatus == "cancelled" {
		if !cancelable[curStatus] {
			return nil, fmt.Errorf("transfer status '%s' tidak dapat dibatalkan", curStatus)
		}
	} else if validFlow[curStatus] != newStatus {
		return nil, fmt.Errorf("transisi status tidak valid: %s → %s", curStatus, newStatus)
	}

	now := time.Now()
	switch newStatus {
	case "approved":
		_, err = tx.Exec(`UPDATE stock_transfers SET status='approved', approved_by=$1, approved_at=$2, updated_at=NOW() WHERE id=$3`, actor, now, id)
	case "sent":
		// Potong stok dari gudang asal
		for _, it := range transfer.Items {
			var availableQty float64
			if err := tx.QueryRow(`
				SELECT COALESCE((
					SELECT qty_base FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2
				), 0)
			`, it.ItemID, transfer.FromWarehouseID).Scan(&availableQty); err != nil {
				return nil, err
			}
			if availableQty < it.QtyBase {
				return nil, fmt.Errorf("stok %s di gudang asal tidak mencukupi", it.ItemName)
			}
			if err := applyMovement(tx, it.ItemID, transfer.FromWarehouseID, "transfer_out",
				it.DistUnit, id, "stock_transfer", transfer.TransferNumber, "",
				actor, -it.QtyBase, -it.QtyDist, 0, ""); err != nil {
				return nil, fmt.Errorf("gagal kurangi stok %s: %w", it.ItemName, err)
			}
		}
		_, err = tx.Exec(`UPDATE stock_transfers SET status='sent', sent_by=$1, sent_at=$2, updated_at=NOW() WHERE id=$3`, actor, now, id)
	case "received":
		// Yang mengirim bukan yang menerima — prinsip yang sama dengan mutasi
		// aset. Tanpa ini, selisih kirim vs terima tidak akan pernah ketahuan.
		if transfer.SentBy != "" && transfer.SentBy == actor {
			return nil, Invalid("pengirim tidak boleh menerima transfer yang sama — penerimaan dicatat oleh petugas gudang tujuan")
		}
		// Tambah stok ke gudang tujuan
		for _, it := range transfer.Items {
			// Nilai barang masuk = cost FIFO yang benar-benar terpotong saat "sent"
			// (tercatat di movement transfer_out). avg_cost live gudang asal bisa
			// sudah berubah oleh pembelian baru di antara kirim dan terima.
			var avgCost float64
			err := tx.QueryRow(`
				SELECT cost_per_base FROM stock_movements
				WHERE ref_id=$1 AND ref_type='stock_transfer' AND item_id=$2 AND movement_type='transfer_out'
				ORDER BY created_at DESC LIMIT 1`, id, it.ItemID).Scan(&avgCost)
			if err != nil {
				// Fallback transfer lama (dikirim sebelum cost tercatat): avg_cost gudang asal.
				if err := tx.QueryRow(`SELECT avg_cost FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2`, it.ItemID, transfer.FromWarehouseID).Scan(&avgCost); err != nil {
					return nil, fmt.Errorf("gagal baca cost dari gudang asal: %w", err)
				}
			}
			
			recvQty := it.QtyBase
			if it.ReceivedQtyBase != nil {
				recvQty = *it.ReceivedQtyBase
				if recvQty < 0 {
					return nil, fmt.Errorf("qty diterima untuk %s tidak boleh negatif", it.ItemName)
				}
				if recvQty > it.QtyBase {
					return nil, fmt.Errorf("qty diterima untuk %s tidak boleh melebihi qty dikirim", it.ItemName)
				}
			}

			// Selisih kirim vs terima: stok sudah terpotong penuh di gudang asal saat
			// "sent", jadi kekurangannya dicatat sebagai waste 'lost' (dokumen, tanpa
			// movement tambahan) supaya penyusutan terlihat di laporan — sebelumnya
			// selisih ini hilang tanpa jejak.
			if shortfall := it.QtyBase - recvQty; shortfall > 0.0001 {
				ratio := it.DistRatio
				if ratio <= 0 {
					ratio = 1
				}
				wasteNum, werr := generateWasteNumber(tx)
				if werr != nil {
					return nil, werr
				}
				if _, werr := tx.Exec(`
					INSERT INTO stock_wastes (id, waste_number, warehouse_id, item_id, qty_base, qty_dist,
						unit_used, cost_per_base, total_cost, reason, notes, created_by)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'lost',$10,$11)`,
					NewULID(), wasteNum, transfer.FromWarehouseID, it.ItemID, shortfall, shortfall/ratio,
					it.DistUnit, avgCost, avgCost*shortfall,
					fmt.Sprintf("Selisih transfer %s: dikirim %.2f, diterima %.2f", transfer.TransferNumber, it.QtyBase, recvQty),
					actor); werr != nil {
					return nil, fmt.Errorf("gagal catat selisih %s: %w", it.ItemName, werr)
				}
			}

			if recvQty == 0 {
				continue
			}
			recvDist := recvQty / it.DistRatio
			if err := applyMovement(tx, it.ItemID, transfer.ToWarehouseID, "transfer_in",
				it.DistUnit, id, "stock_transfer", transfer.TransferNumber, "",
				actor, recvQty, recvDist, avgCost, ""); err != nil {
				return nil, fmt.Errorf("gagal tambah stok %s: %w", it.ItemName, err)
			}
		}
		_, err = tx.Exec(`UPDATE stock_transfers SET status='received', received_by=$1, received_at=$2, updated_at=NOW() WHERE id=$3`, actor, now, id)
	case "cancelled":
		_, err = tx.Exec(`UPDATE stock_transfers SET status='cancelled', updated_at=NOW() WHERE id=$1`, id)
	}

	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetStockTransfer(id)
}

// ── Product Recipes ───────────────────────────────────────────

func GetProductRecipes(productID string) ([]models.ProductRecipe, error) {
	rows, err := database.DB.Query(`
		SELECT pr.id, pr.product_id, COALESCE(cp.name,''), pr.item_id, si.code, si.name,
		       si.dist_unit, si.dist_unit_label, si.base_unit,
		       pr.qty_dist, pr.qty_base, pr.unit_used, pr.notes
		FROM product_recipes pr
		JOIN stock_items si ON si.id = pr.item_id
		LEFT JOIN cloud_products cp ON cp.id = pr.product_id
		WHERE pr.product_id = $1 ORDER BY si.name`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.ProductRecipe{}
	for rows.Next() {
		var r models.ProductRecipe
		if err := rows.Scan(&r.ID, &r.ProductID, &r.ProductName, &r.ItemID, &r.ItemCode, &r.ItemName,
			&r.DistUnit, &r.DistUnitLabel, &r.BaseUnit,
			&r.QtyDist, &r.QtyBase, &r.UnitUsed, &r.Notes); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func SaveProductRecipes(req models.ProductRecipeRequest) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove existing
	_, err = tx.Exec(`DELETE FROM product_recipes WHERE product_id = $1`, req.ProductID)
	if err != nil {
		return err
	}

	for _, it := range req.Items {
		item, err := GetStockItem(it.ItemID)
		if err != nil {
			return fmt.Errorf("bahan baku %s tidak ditemukan", it.ItemID)
		}
		qtyBase := it.QtyDist * item.DistRatio
		_, err = tx.Exec(`
			INSERT INTO product_recipes (id, product_id, item_id, qty_dist, qty_base, unit_used, notes, visibility)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			NewULID(), req.ProductID, it.ItemID, it.QtyDist, qtyBase, item.DistUnit, it.Notes, req.Visibility)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Master Recipe System ─────────────────────────────────────

func ListRecipeMasters(outletID string, search string) ([]models.RecipeMaster, error) {
	conds := []string{"(r.visibility = 'public' OR r.id IN (SELECT recipe_master_id FROM recipe_outlet_access WHERE outlet_id = $1))"}
	args := []interface{}{outletID}
	
	if search != "" {
		conds = append(conds, fmt.Sprintf("r.name ILIKE $%d", len(args)+1))
		args = append(args, "%"+search+"%")
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.name, r.description, r.visibility, r.instructions, r.total_time, r.created_at, r.updated_at
		FROM recipe_masters r
		WHERE %s ORDER BY r.name ASC`, strings.Join(conds, " AND "))

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []models.RecipeMaster{}
	for rows.Next() {
		var rm models.RecipeMaster
		if err := rows.Scan(&rm.ID, &rm.Name, &rm.Description, &rm.Visibility, &rm.Instructions, &rm.TotalTime, &rm.CreatedAt, &rm.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, rm)
	}
	return result, nil
}

func GetRecipeMaster(id string) (models.RecipeMaster, error) {
	var rm models.RecipeMaster
	err := database.DB.QueryRow(`SELECT id, name, description, visibility, instructions, total_time, created_at, updated_at FROM recipe_masters WHERE id=$1`, id).Scan(
		&rm.ID, &rm.Name, &rm.Description, &rm.Visibility, &rm.Instructions, &rm.TotalTime, &rm.CreatedAt, &rm.UpdatedAt)
	if err != nil {
		return rm, err
	}

	// Items
	rows, err := database.DB.Query(`
		SELECT ri.id, ri.item_id, si.name, si.code, ri.qty_base, si.base_unit, ri.notes
		FROM recipe_items ri
		JOIN stock_items si ON si.id = ri.item_id
		WHERE ri.recipe_master_id = $1`, id)
	if err == nil {
		defer rows.Close()
	for rows.Next() {
		var ri models.RecipeItem
		if err := rows.Scan(&ri.ID, &ri.ItemID, &ri.ItemName, &ri.ItemCode, &ri.QtyBase, &ri.Unit, &ri.Notes); err != nil {
			rows.Close()
			return rm, err
		}
		rm.Items = append(rm.Items, ri)
	}
	}

	// Outlet Access
	oaRows, oaErr := database.DB.Query(`SELECT outlet_id FROM recipe_outlet_access WHERE recipe_master_id=$1`, id)
	if oaErr == nil {
		defer oaRows.Close()
		for oaRows.Next() {
			var oid string
			if err := oaRows.Scan(&oid); err != nil {
				return rm, err
			}
			rm.OutletIDs = append(rm.OutletIDs, oid)
		}
	}

	return rm, nil
}

func SaveRecipeMaster(id string, req models.RecipeMasterRequest) (string, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if id == "" {
		id = NewULID()
		_, err = tx.Exec(`INSERT INTO recipe_masters (id, name, description, visibility, instructions, total_time) VALUES ($1,$2,$3,$4,$5,$6)`,
			id, req.Name, req.Description, req.Visibility, req.Instructions, req.TotalTime)
	} else {
		_, err = tx.Exec(`UPDATE recipe_masters SET name=$1, description=$2, visibility=$3, instructions=$4, total_time=$5, updated_at=NOW() WHERE id=$6`,
			req.Name, req.Description, req.Visibility, req.Instructions, req.TotalTime, id)
	}
	if err != nil {
		return "", err
	}

	// Update Items
	_, _ = tx.Exec(`DELETE FROM recipe_items WHERE recipe_master_id = $1`, id)
	for _, it := range req.Items {
		_, err = tx.Exec(`INSERT INTO recipe_items (id, recipe_master_id, item_id, qty_base, notes) VALUES ($1,$2,$3,$4,$5)`,
			NewULID(), id, it.ItemID, it.QtyBase, it.Notes)
		if err != nil {
			return "", err
		}
	}

	// Update Access
	_, _ = tx.Exec(`DELETE FROM recipe_outlet_access WHERE recipe_master_id = $1`, id)
	if req.Visibility == "secret" {
		for _, oid := range req.OutletIDs {
			_, err = tx.Exec(`INSERT INTO recipe_outlet_access (recipe_master_id, outlet_id) VALUES ($1,$2)`, id, oid)
			if err != nil {
				return "", err
			}
		}
	}

	err = tx.Commit()
	return id, err
}

func DeleteRecipeMaster(id string) error {
	_, err := database.DB.Exec(`DELETE FROM recipe_masters WHERE id = $1`, id)
	return err
}


func GetStockItemRecipes(parentID string) ([]models.StockItemRecipe, error) {
	rows, err := database.DB.Query(`
		SELECT r.id, r.parent_item_id, r.child_item_id, si.name, si.code, r.qty_base, si.base_unit, r.notes, r.visibility
		FROM stock_item_recipes r
		JOIN stock_items si ON si.id = r.child_item_id
		WHERE r.parent_item_id = $1 ORDER BY si.name`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.StockItemRecipe{}
	for rows.Next() {
		var r models.StockItemRecipe
		if err := rows.Scan(&r.ID, &r.ParentItemID, &r.ChildItemID, &r.ChildName, &r.ChildCode, &r.QtyBase, &r.Unit, &r.Notes, &r.Visibility); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func SaveStockItemRecipes(req models.StockItemRecipeRequest) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`DELETE FROM stock_item_recipes WHERE parent_item_id = $1`, req.ParentItemID)
	if err != nil {
		return err
	}
	for _, it := range req.Items {
		_, err = tx.Exec(`
			INSERT INTO stock_item_recipes (id, parent_item_id, child_item_id, qty_base, notes, visibility)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			NewULID(), req.ParentItemID, it.ItemID, it.QtyBase, it.Notes, req.Visibility)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ProduceStockItem creates a semi-finished good from raw components.
func ProduceStockItem(req models.ProduceRequest, createdBy string) error {
	if req.QtyProduce <= 0 {
		return fmt.Errorf("kuantitas produksi harus lebih dari 0")
	}

	recipes, err := GetStockItemRecipes(req.ItemID)
	if err != nil || len(recipes) == 0 {
		return fmt.Errorf("item ini tidak memiliki resep, definisikan resep terlebih dahulu")
	}

	itemToProduce, err := GetStockItem(req.ItemID)
	if err != nil {
		return err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var totalCostOfProduction float64

	// Generate unique production reference for this entire production run
	prodRef := fmt.Sprintf("PROD-%s-%s", req.ItemID[:min(len(req.ItemID), 8)], time.Now().Format("150405.000"))

	// 1. Deduct all components (FIFO) and calculate total cost
	for _, r := range recipes {
		qtyToDeduct := r.QtyBase * req.QtyProduce
		
		// Lock and check stock first (FOR UPDATE untuk cegah race condition)
		var available float64
		if err := tx.QueryRow(`SELECT qty_base FROM stock_ledger WHERE item_id=$1 AND warehouse_id=$2 FOR UPDATE`, r.ChildItemID, req.WarehouseID).Scan(&available); err != nil {
			return fmt.Errorf("gagal membaca stok %s: %w", r.ChildName, err)
		}
		if available < qtyToDeduct {
			return fmt.Errorf("stok %s tidak mencukupi (butuh %.2f, ada %.2f)", r.ChildName, qtyToDeduct, available)
		}

		// Deduction (Movement Type: 'production_out')

		err := applyMovement(tx, r.ChildItemID, req.WarehouseID, "production_out", r.Unit, 
			"", "production", prodRef, "Used to produce "+itemToProduce.Name, createdBy, -qtyToDeduct, -qtyToDeduct, 0, "")
		if err != nil {
			return err
		}

		// Get the actual cost from the movement record we just created
		// applyMovement's FIFO deduction calculates actualCostPerBase from old batches
		var movementCost float64
		err = tx.QueryRow(`
			SELECT cost_per_base * ABS(qty_base) 
			FROM stock_movements 
			WHERE item_id=$1 AND warehouse_id=$2 AND ref_type='production' AND ref_number=$3
		`, r.ChildItemID, req.WarehouseID, prodRef).Scan(&movementCost)
		if err != nil {
			return fmt.Errorf("gagal membaca biaya aktual untuk %s: %w", r.ChildName, err)
		}
		totalCostOfProduction += movementCost


	}

	// 2. Add the Produced Item (SFG) with calculated HPP
	costPerUnit := totalCostOfProduction / req.QtyProduce
	err = applyMovement(tx, req.ItemID, req.WarehouseID, "production_in", itemToProduce.BaseUnit,
		"", "production", prodRef, "Produced from components", createdBy, req.QtyProduce, req.QtyProduce, costPerUnit, "")

	if err != nil {
		return err
	}

	return tx.Commit()
}

// ── Stock Waste ───────────────────────────────────────────

func generateWasteNumber(tx *sql.Tx) (string, error) {
	prefix := "WST-" + time.Now().In(GetTimezoneLocation()).Format("20060102") + "-"
	seq, err := nextDocNumber(tx, "stock_wastes", "waste_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, seq), nil
}

func GetStockWastes(warehouseID, search, dateFrom, dateTo string, outletIDs []string, page, limit int) ([]models.StockWaste, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	i := 1

	if warehouseID != "" {
		where += fmt.Sprintf(" AND sw.warehouse_id = $%d", i)
		args = append(args, warehouseID)
		i++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (si.name ILIKE $%d OR si.code ILIKE $%d OR sw.waste_number ILIKE $%d)", i, i+1, i+2)
		args = append(args, "%"+search+"%", "%"+search+"%", "%"+search+"%")
		i += 3
	}
	// Batas hari mengikuti timezone aplikasi (kolom tersimpan UTC).
	if dateFrom != "" {
		where += fmt.Sprintf(" AND sw.created_at >= tz_day_start($%d::date)", i)
		args = append(args, dateFrom)
		i++
	}
	if dateTo != "" {
		where += fmt.Sprintf(" AND sw.created_at < tz_day_start($%d::date + 1)", i)
		args = append(args, dateTo)
		i++
	}
	if outletIDs != nil {
		where += fmt.Sprintf(" AND sw.warehouse_id IN (SELECT id FROM warehouses WHERE outlet_id = ANY($%d))", i)
		args = append(args, pq.Array(outletIDs))
		i++
	}

	var total int
	err := database.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM stock_wastes sw
		JOIN stock_items si ON sw.item_id = si.id
		`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	args = append(args, limit, offset)

	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT sw.id, sw.waste_number, sw.warehouse_id, w.name as warehouse_name,
		       sw.item_id, si.name as item_name, si.code as item_code,
		       sw.qty_base, sw.qty_dist, sw.unit_used, si.base_unit, si.dist_unit,
		       sw.cost_per_base, sw.total_cost, sw.reason, sw.notes, sw.created_by,
		       TO_CHAR(sw.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_wastes sw
		JOIN warehouses w ON sw.warehouse_id = w.id
		JOIN stock_items si ON sw.item_id = si.id
		%s
		ORDER BY sw.created_at DESC LIMIT $%d OFFSET $%d
	`, where, i, i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var wastes []models.StockWaste
	for rows.Next() {
		var w models.StockWaste
		err := rows.Scan(&w.ID, &w.WasteNumber, &w.WarehouseID, &w.WarehouseName,
			&w.ItemID, &w.ItemName, &w.ItemCode,
			&w.QtyBase, &w.QtyDist, &w.UnitUsed, &w.BaseUnit, &w.DistUnit,
			&w.CostPerBase, &w.TotalCost, &w.Reason, &w.Notes, &w.CreatedBy, &w.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		wastes = append(wastes, w)
	}

	return wastes, total, nil
}

func CreateStockWaste(req models.StockWasteRequest, actor string) error {
	item, err := GetStockItem(req.ItemID)
	if err != nil {
		return err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtyBase := req.QtyDist * item.DistRatio
	wasteID := NewULID()
	wasteNum, err := generateWasteNumber(tx)
	if err != nil {
		return err
	}

	// 1. Apply Movement (deduct stock)
	movType := "waste"
	if req.Reason == "expired" {
		movType = "expired"
	} else if req.Reason == "spoiled" {
		movType = "spoiled"
	}

	err = applyMovement(tx, req.ItemID, req.WarehouseID, movType, req.UnitUsed,
		wasteID, "stock_waste", wasteNum, req.Notes, actor, -qtyBase, -req.QtyDist, 0, "")
	if err != nil {
		return err
	}

	// 2. Get actual cost from the movement record (applyMovement calculates this via FIFO)
	var costPerBase float64
	err = tx.QueryRow(`
		SELECT cost_per_base 
		FROM stock_movements 
		WHERE ref_id = $1 AND ref_type = 'stock_waste'
	`, wasteID).Scan(&costPerBase)
	if err != nil {
		return err
	}

	totalCost := costPerBase * qtyBase

	// 3. Record in stock_wastes
	_, err = tx.Exec(`
		INSERT INTO stock_wastes (
			id, waste_number, warehouse_id, item_id, qty_base, qty_dist, 
			unit_used, cost_per_base, total_cost, reason, notes, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, wasteID, wasteNum, req.WarehouseID, req.ItemID, qtyBase, req.QtyDist,
		req.UnitUsed, costPerBase, totalCost, req.Reason, req.Notes, actor)
	if err != nil {
		return err
	}

	return tx.Commit()
}



// GetWarehouseDashboard returns summary stats for the warehouse dashboard.
func GetWarehouseDashboard(outletIDs []string) (*models.WarehouseDashboardStats, error) {
	stats := &models.WarehouseDashboardStats{}

	// Scope helper: warehouse filter clause and args
	wFilter := ""   // filter on table aliased as w (warehouses)
	oFilter := ""   // filter on outlet_id column directly
	var wArgs []interface{}
	if outletIDs != nil {
		wFilter = " AND (w.outlet_id = ANY($1) OR w.type = 'central')"
		oFilter = " AND (outlet_id = ANY($1) OR type = 'central')"
		wArgs = append(wArgs, pq.Array(outletIDs))
	}

	run := func(q string, args []interface{}, dest ...interface{}) {
		if args != nil {
			database.DB.QueryRow(q, args...).Scan(dest...)
		} else {
			database.DB.QueryRow(q).Scan(dest...)
		}
	}

	// ── 1. Ringkasan gudang ──────────────────────────────────
	run(fmt.Sprintf(`SELECT COUNT(*), COUNT(CASE WHEN type='central' THEN 1 END), COUNT(CASE WHEN type='outlet' THEN 1 END)
		FROM warehouses w WHERE is_active = true%s`, wFilter), wArgs,
		&stats.TotalWarehouses, &stats.CentralWarehouses, &stats.OutletWarehouses)

	// ── 2. Total item bahan baku ─────────────────────────────
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_items WHERE is_active = true`).Scan(&stats.TotalItems)

	// ── 3. Item dengan stok (per warehouse scope) ────────────
	// items_with_stock: item yang qty_base > 0 di gudang scope
	// low_stock: item qty_base <= min_stock (dan min_stock > 0) di setidaknya satu gudang scope
	// out_of_stock: item yang TOTAL qty_base = 0 di semua gudang scope (pernah punya stok)
	// total_stock_value: sum(qty_base * avg_cost) semua gudang scope
	run(fmt.Sprintf(`
		SELECT
			COUNT(DISTINCT CASE WHEN sl.qty_base > 0 THEN sl.item_id END),
			COUNT(DISTINCT CASE WHEN sl.qty_base <= %s AND %s > 0 AND sl.qty_base >= 0 THEN sl.item_id END),
			COUNT(DISTINCT CASE WHEN sl.qty_base <= 0 THEN sl.item_id END),
			COALESCE(SUM(CASE WHEN sl.qty_base > 0 THEN sl.qty_base * sl.avg_cost ELSE 0 END), 0)
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id
		JOIN warehouses w ON w.id = sl.warehouse_id
		WHERE w.is_active = true%s`, effMinStock, effMinStock, wFilter), wArgs,
		&stats.ItemsWithStock, &stats.LowStockCount, &stats.OutOfStockCount, &stats.TotalStockValue)

	// ── 4. Transfer pending ──────────────────────────────────
	run(fmt.Sprintf(`
		SELECT COUNT(*) FROM stock_transfers st
		WHERE st.status NOT IN ('received','cancelled')
		AND (st.from_warehouse_id IN (SELECT id FROM warehouses WHERE is_active=true%s)
		  OR st.to_warehouse_id   IN (SELECT id FROM warehouses WHERE is_active=true%s))`,
		oFilter, oFilter), wArgs, &stats.PendingTransfers)

	// ── 5. Pergerakan hari ini dan 7 hari ───────────────────
	// tz_date/tz_today: kolom tersimpan UTC; `created_at AT TIME ZONE 'Asia/Jakarta'`
	// polos arah konversinya terbalik (geser -7 jam, bukan +7).
	run(fmt.Sprintf(`
		SELECT
			COUNT(CASE WHEN tz_date(sm.created_at) = tz_today() THEN 1 END),
			COUNT(CASE WHEN sm.created_at >= NOW() - INTERVAL '7 days' THEN 1 END)
		FROM stock_movements sm
		JOIN warehouses w ON w.id = sm.warehouse_id
		WHERE w.is_active = true%s`, wFilter), wArgs,
		&stats.MovementsToday, &stats.Movements7d)

	// ── 6. Stok per gudang ───────────────────────────────────
	wsQ := fmt.Sprintf(`
		SELECT w.id, w.name, w.type, COALESCE(o.name,'—'),
			COUNT(DISTINCT CASE WHEN sl.qty_base > 0 THEN sl.item_id END),
			COALESCE(SUM(CASE WHEN sl.qty_base > 0 THEN sl.qty_base * sl.avg_cost ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN sl.qty_base <= %s AND %s > 0 AND sl.qty_base >= 0 THEN sl.item_id END)
		FROM warehouses w
		LEFT JOIN outlets o ON o.id = w.outlet_id
		LEFT JOIN stock_ledger sl ON sl.warehouse_id = w.id
		LEFT JOIN stock_items si ON si.id = sl.item_id
		WHERE w.is_active = true%s
		GROUP BY w.id, w.name, w.type, o.name
		ORDER BY w.type DESC, SUM(COALESCE(CASE WHEN sl.qty_base > 0 THEN sl.qty_base * sl.avg_cost ELSE 0 END, 0)) DESC`,
		effMinStock, effMinStock, wFilter)
	stats.WarehouseStocks = []models.WarehouseStockSummary{}
	wsRows, wsErr := func() (*sql.Rows, error) {
		if wArgs != nil {
			return database.DB.Query(wsQ, wArgs...)
		}
		return database.DB.Query(wsQ)
	}()
	if wsErr == nil {
		defer wsRows.Close()
		for wsRows.Next() {
			var r models.WarehouseStockSummary
			if err := wsRows.Scan(&r.WarehouseID, &r.WarehouseName, &r.WarehouseType, &r.OutletName,
				&r.TotalItems, &r.TotalValue, &r.LowStockCount); err == nil {
				stats.WarehouseStocks = append(stats.WarehouseStocks, r)
			}
		}
	}

	// ── 7. Item stok rendah / habis (maks 10) ───────────────
	lsQ := fmt.Sprintf(`
		SELECT sl.item_id, si.code, si.name, si.base_unit, si.dist_unit, si.dist_ratio, si.dist_unit_label,
			w.id, w.name, w.type,
			sl.qty_base,
			ROUND(CASE WHEN si.dist_ratio > 0 THEN sl.qty_base / si.dist_ratio ELSE sl.qty_base END, 4),
			sl.avg_cost, sl.qty_base * sl.avg_cost, %s, true
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id
		JOIN warehouses w ON w.id = sl.warehouse_id
		WHERE w.is_active = true AND %s > 0 AND sl.qty_base <= %s%s
		ORDER BY sl.qty_base ASC LIMIT 10`, effMinStock, effMinStock, effMinStock, wFilter)
	stats.LowStockItems = []models.StockLedgerRow{}
	lsRows, lsErr := func() (*sql.Rows, error) {
		if wArgs != nil {
			return database.DB.Query(lsQ, wArgs...)
		}
		return database.DB.Query(lsQ)
	}()
	if lsErr == nil {
		defer lsRows.Close()
		for lsRows.Next() {
			var r models.StockLedgerRow
			if err := lsRows.Scan(&r.ItemID, &r.ItemCode, &r.ItemName, &r.BaseUnit, &r.DistUnit, &r.DistRatio, &r.DistUnitLabel,
				&r.WarehouseID, &r.WarehouseName, &r.WarehouseType,
				&r.QtyBase, &r.QtyDist, &r.AvgCost, &r.StockValue, &r.MinStock, &r.IsLow); err == nil {
				stats.LowStockItems = append(stats.LowStockItems, r)
			}
		}
	}

	// ── 8. Pergerakan terkini (maks 10) ─────────────────────
	rmQ := fmt.Sprintf(`
		SELECT sm.id, sm.item_id, si.name, sm.warehouse_id, w.name,
			sm.movement_type, sm.qty_base, sm.qty_dist, sm.unit_used,
			sm.cost_per_base, sm.balance_after,
			COALESCE(sm.ref_id::text,''), COALESCE(sm.ref_type,''), COALESCE(sm.ref_number,''),
			sm.expiry_date::text, COALESCE(sm.notes,''), COALESCE(sm.created_by,''),
			sm.created_at::text
		FROM stock_movements sm
		JOIN stock_items si ON si.id = sm.item_id
		JOIN warehouses w ON w.id = sm.warehouse_id
		WHERE w.is_active = true%s
		ORDER BY sm.created_at DESC LIMIT 10`, wFilter)
	stats.RecentMovements = []models.StockMovement{}
	rmRows, rmErr := func() (*sql.Rows, error) {
		if wArgs != nil {
			return database.DB.Query(rmQ, wArgs...)
		}
		return database.DB.Query(rmQ)
	}()
	if rmErr == nil {
		defer rmRows.Close()
		for rmRows.Next() {
			var r models.StockMovement
			if err := rmRows.Scan(&r.ID, &r.ItemID, &r.ItemName, &r.WarehouseID, &r.WarehouseName,
				&r.MovementType, &r.QtyBase, &r.QtyDist, &r.UnitUsed,
				&r.CostPerBase, &r.BalanceAfter,
				&r.RefID, &r.RefType, &r.RefNumber,
				&r.ExpiryDate, &r.Notes, &r.CreatedBy, &r.CreatedAt); err == nil {
				stats.RecentMovements = append(stats.RecentMovements, r)
			}
		}
	}

	// ── 9. Trend pergerakan 14 hari (FIX: subquery scope, bukan LEFT JOIN) ──
	// Bug sebelumnya: filter scope di LEFT JOIN tidak efektif memfilter data
	whSubQ := "SELECT id FROM warehouses WHERE is_active = true"
	var trendArgs []interface{}
	if outletIDs != nil {
		whSubQ += " AND (outlet_id = ANY($1) OR type = 'central')"
		trendArgs = []interface{}{pq.Array(outletIDs)}
	}
	trendQ := fmt.Sprintf(`
		SELECT TO_CHAR(d.dt, 'YYYY-MM-DD'),
			COUNT(CASE WHEN sm.qty_base > 0 THEN 1 END)::int,
			COUNT(CASE WHEN sm.qty_base < 0 THEN 1 END)::int
		FROM generate_series(tz_today() - 13, tz_today(), '1 day'::interval) d(dt)
		LEFT JOIN stock_movements sm
			ON tz_date(sm.created_at) = d.dt
			AND sm.warehouse_id IN (%s)
		GROUP BY d.dt ORDER BY d.dt`, whSubQ)
	stats.MovementTrend = []models.DailyMovementPoint{}
	trRows, trErr := func() (*sql.Rows, error) {
		if trendArgs != nil {
			return database.DB.Query(trendQ, trendArgs...)
		}
		return database.DB.Query(trendQ)
	}()
	if trErr == nil {
		defer trRows.Close()
		for trRows.Next() {
			var p models.DailyMovementPoint
			if err := trRows.Scan(&p.Date, &p.In, &p.Out); err == nil {
				stats.MovementTrend = append(stats.MovementTrend, p)
			}
		}
	}

	// ── 10. Transfer pending list (maks 5) ───────────────────
	// Ikut scope outlet: user ber-scope hanya melihat transfer yang menyentuh
	// gudang pusat atau gudang outlet-nya (konsisten dengan blok lain di atas).
	stats.PendingTransferList = []models.StockTransfer{}
	ptScope := ""
	var ptArgs []interface{}
	if outletIDs != nil {
		ptScope = ` AND (fw.outlet_id = ANY($1) OR fw.type = 'central' OR tw.outlet_id = ANY($1) OR tw.type = 'central')`
		ptArgs = append(ptArgs, pq.Array(outletIDs))
	}
	ptRows, ptErr := database.DB.Query(`
		SELECT st.id, st.transfer_number,
			st.from_warehouse_id, fw.name,
			st.to_warehouse_id, tw.name,
			st.status, COALESCE(st.notes,''), COALESCE(st.created_by,''),
			COALESCE(st.approved_by,''), COALESCE(st.sent_by,''), COALESCE(st.received_by,''),
			COALESCE(st.approved_at::text,''), COALESCE(st.sent_at::text,''), COALESCE(st.received_at::text,''),
			st.created_at::text, st.updated_at::text
		FROM stock_transfers st
		JOIN warehouses fw ON fw.id = st.from_warehouse_id
		JOIN warehouses tw ON tw.id = st.to_warehouse_id
		WHERE st.status NOT IN ('received','cancelled')`+ptScope+`
		ORDER BY st.created_at DESC LIMIT 5`, ptArgs...)
	if ptErr == nil {
		defer ptRows.Close()
		for ptRows.Next() {
			var r models.StockTransfer
			if err := ptRows.Scan(&r.ID, &r.TransferNumber,
				&r.FromWarehouseID, &r.FromWarehouse,
				&r.ToWarehouseID, &r.ToWarehouse,
				&r.Status, &r.Notes, &r.CreatedBy,
				&r.ApprovedBy, &r.SentBy, &r.ReceivedBy,
				&r.ApprovedAt, &r.SentAt, &r.ReceivedAt,
				&r.CreatedAt, &r.UpdatedAt); err == nil {
				stats.PendingTransferList = append(stats.PendingTransferList, r)
			}
		}
	}

	return stats, nil
}
