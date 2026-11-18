package services

import (
	"cloud-pos/database"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// ProcurementStockCatalog exposes quantities without inventory valuation. Each
// balance belongs to one accessible, active central warehouse; balances are
// never combined across warehouses.
type ProcurementStockCatalog struct {
	Items      []ProcurementStockItem      `json:"items"`
	Warehouses []ProcurementStockWarehouse `json:"warehouses"`
	AsOf       string                      `json:"as_of"`
}

type ProcurementStockWarehouse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProcurementStockItem struct {
	StockCatalogName
	Code             string             `json:"code"`
	BaseUnit         string             `json:"base_unit"`
	PurchaseRatio    float64            `json:"purchase_ratio"`
	StockByWarehouse map[string]float64 `json:"stock_by_warehouse"`
}

func ListProcurementStockCatalog(outletScope []string) (*ProcurementStockCatalog, error) {
	out := &ProcurementStockCatalog{
		Items: make([]ProcurementStockItem, 0), Warehouses: make([]ProcurementStockWarehouse, 0),
	}
	// Match the regular warehouse list: scoped roles can only see warehouses
	// belonging to their outlets. The transfer-only include_central exception
	// does not grant access to central balances here.
	where := "w.type = 'central' AND w.is_active = true"
	var args []interface{}
	if outletScope != nil {
		where += " AND w.outlet_id = ANY($1)"
		args = append(args, pq.Array(outletScope))
	}
	rows, err := database.DB.Query(`SELECT w.id, w.name FROM warehouses w WHERE `+where+` ORDER BY w.name, w.id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var warehouse ProcurementStockWarehouse
		if err := rows.Scan(&warehouse.ID, &warehouse.Name); err != nil {
			rows.Close()
			return nil, err
		}
		out.Warehouses = append(out.Warehouses, warehouse)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}

	rows, err = database.DB.Query(`
		SELECT id, TRIM(name),
		       COALESCE(NULLIF(dist_unit_label, ''), NULLIF(dist_unit, ''), base_unit, ''),
		       COALESCE(is_active, true), COALESCE(code, ''), COALESCE(base_unit, ''),
		       CASE WHEN COALESCE(NULLIF(dist_unit_label, ''), NULLIF(dist_unit, ''), base_unit, '') = base_unit
		            THEN 1 ELSE COALESCE(NULLIF(dist_ratio, 0), 1) END
		FROM stock_items WHERE TRIM(name) <> '' ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]int)
	for rows.Next() {
		item := ProcurementStockItem{StockByWarehouse: make(map[string]float64)}
		if err := rows.Scan(&item.ID, &item.Name, &item.Unit, &item.IsActive, &item.Code, &item.BaseUnit, &item.PurchaseRatio); err != nil {
			rows.Close()
			return nil, err
		}
		byID[item.ID] = len(out.Items)
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = database.DB.Query(fmt.Sprintf(`
		SELECT sl.item_id, sl.warehouse_id, sl.qty_base
		FROM stock_ledger sl JOIN warehouses w ON w.id = sl.warehouse_id
		WHERE %s`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID, warehouseID string
		var quantity float64
		if err := rows.Scan(&itemID, &warehouseID, &quantity); err != nil {
			return nil, err
		}
		if index, ok := byID[itemID]; ok {
			out.Items[index].StockByWarehouse[warehouseID] = quantity
		}
	}
	out.AsOf = time.Now().UTC().Format(time.RFC3339)
	return out, rows.Err()
}
