package services

import (
	"cloud-pos/database"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"
)

// ProcurementStockCatalog exposes quantities without inventory valuation. Each
// balance belongs to one accessible, active central warehouse; balances are
// never combined across warehouses. LastPurchasePrice comes from historical
// purchase/GRN documents to prefill HPS on raw-material requests.
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
	Code               string             `json:"code"`
	BaseUnit           string             `json:"base_unit"`
	PurchaseRatio      float64            `json:"purchase_ratio"`
	StockByWarehouse   map[string]float64 `json:"stock_by_warehouse"`
	LastPurchasePrice  float64            `json:"last_purchase_price,omitempty"`
	LastPurchaseDate   string             `json:"last_purchase_date,omitempty"`
	LastPurchaseSource string             `json:"last_purchase_source,omitempty"`
}

func ListProcurementStockCatalog(outletScope []string) (*ProcurementStockCatalog, error) {
	out := &ProcurementStockCatalog{
		Items: make([]ProcurementStockItem, 0), Warehouses: make([]ProcurementStockWarehouse, 0),
	}
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
		       CASE WHEN LOWER(BTRIM(COALESCE(NULLIF(dist_unit, ''), base_unit, ''))) = LOWER(BTRIM(COALESCE(base_unit, '')))
		            THEN 1 ELSE COALESCE(NULLIF(dist_ratio, 0), 1) END,
		       COALESCE(dist_unit, '')
		FROM stock_items WHERE TRIM(name) <> '' ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]int)
	byName := make(map[string]int)
	distUnits := make([]string, 0)
	for rows.Next() {
		item := ProcurementStockItem{StockByWarehouse: make(map[string]float64)}
		var distUnit string
		if err := rows.Scan(&item.ID, &item.Name, &item.Unit, &item.IsActive, &item.Code, &item.BaseUnit, &item.PurchaseRatio, &distUnit); err != nil {
			rows.Close()
			return nil, err
		}
		idx := len(out.Items)
		byID[item.ID] = idx
		byName[strings.ToLower(strings.TrimSpace(item.Name))] = idx
		distUnits = append(distUnits, strings.TrimSpace(distUnit))
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
	for rows.Next() {
		var itemID, warehouseID string
		var quantity float64
		if err := rows.Scan(&itemID, &warehouseID, &quantity); err != nil {
			rows.Close()
			return nil, err
		}
		if index, ok := byID[itemID]; ok {
			out.Items[index].StockByWarehouse[warehouseID] = quantity
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// 1. Harga beli terakhir dari Penerimaan Barang (GRN).
	grRows, grErr := database.DB.Query(`
		SELECT DISTINCT ON (gi.item_id)
		       gi.item_id,
		       gi.cost_per_base,
		       COALESCE(gi.qty_dist, 0),
		       COALESCE(gi.subtotal, 0),
		       COALESCE(gi.unit_used, ''),
		       TO_CHAR(tz_date(gr.received_at), 'YYYY-MM-DD')
		FROM goods_receipt_items gi
		JOIN goods_receipts gr ON gr.id = gi.receipt_id
		WHERE gi.cost_per_base > 0
		ORDER BY gi.item_id, gr.received_at DESC, gi.id DESC`)
	if grErr == nil {
		for grRows.Next() {
			var itemID, unitUsed, dt string
			var costPerBase, qtyDist, subtotal float64
			if err := grRows.Scan(&itemID, &costPerBase, &qtyDist, &subtotal, &unitUsed, &dt); err == nil {
				if idx, ok := byID[itemID]; ok {
					ratio := out.Items[idx].PurchaseRatio
					if ratio <= 0 {
						ratio = 1
					}
					price := math.Round(costPerBase * ratio)
					if qtyDist > 0 && subtotal > 0 && distUnits[idx] != "" && strings.EqualFold(strings.TrimSpace(unitUsed), distUnits[idx]) {
						price = math.Round(subtotal / qtyDist)
					}
					if price > 0 {
						out.Items[idx].LastPurchasePrice = price
						out.Items[idx].LastPurchaseDate = dt
						out.Items[idx].LastPurchaseSource = "GRN"
					}
				}
			}
		}
		grRows.Close()
	}

	// 2. Fallback dari riwayat pengadaan (final_price, lalu hps_price) untuk item yang belum pernah lewat GRN.
	prRows, prErr := database.DB.Query(`
		SELECT DISTINCT ON (LOWER(BTRIM(sub->>'name')))
		       LOWER(BTRIM(sub->>'name')) AS item_key,
		       COALESCE(
		         NULLIF(CASE WHEN (sub->>'final_price') ~ '^[0-9]+(\.[0-9]+)?$' THEN (sub->>'final_price')::numeric ELSE 0 END, 0),
		         NULLIF(CASE WHEN (sub->>'hps_price')   ~ '^[0-9]+(\.[0-9]+)?$' THEN (sub->>'hps_price')::numeric   ELSE 0 END, 0),
		         0
		       )::float8 AS last_price,
		       TO_CHAR(tz_date(pr.created_at), 'YYYY-MM-DD') AS dt
		FROM purchase_requests pr
		CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(pr.items) = 'array' THEN pr.items ELSE '[]'::jsonb END) AS grp
		CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(grp->'items') = 'array' THEN grp->'items' ELSE '[]'::jsonb END) AS sub
		WHERE pr.status NOT IN ('rejected', 'cancelled')
		  AND BTRIM(COALESCE(sub->>'name', '')) <> ''
		  AND (
		    (CASE WHEN (sub->>'final_price') ~ '^[0-9]+(\.[0-9]+)?$' THEN (sub->>'final_price')::numeric ELSE 0 END) > 0
		    OR (CASE WHEN (sub->>'hps_price') ~ '^[0-9]+(\.[0-9]+)?$' THEN (sub->>'hps_price')::numeric ELSE 0 END) > 0
		  )
		ORDER BY LOWER(BTRIM(sub->>'name')), pr.created_at DESC, pr.id DESC`)
	if prErr == nil {
		for prRows.Next() {
			var key, dt string
			var price float64
			if err := prRows.Scan(&key, &price, &dt); err == nil {
				if idx, ok := byName[key]; ok && out.Items[idx].LastPurchasePrice <= 0 && price > 0 {
					out.Items[idx].LastPurchasePrice = math.Round(price)
					out.Items[idx].LastPurchaseDate = dt
					out.Items[idx].LastPurchaseSource = "Pengadaan"
				}
			}
		}
		prRows.Close()
	}

	out.AsOf = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}
