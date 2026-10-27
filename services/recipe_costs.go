package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"github.com/lib/pq"
	"time"
)

type recipeCostQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
}
type costMaterial struct {
	id  string
	qty float64
}
type costIngredient struct{ name, unit string }
type recipeCostCatalog struct {
	infos      map[string]costIngredient
	wip        map[string][]costMaterial
	costs      map[string]float64
	quantities map[string]float64
	purchases  map[string]recipePurchasePrice
}

type recipePurchasePrice struct {
	cost float64
	at   string
}

type recipeCostDefinition struct {
	materials []costMaterial
	initial   *models.RecipeCostSnapshot
}

func loadRecipeCostDefinitions(q recipeCostQuerier, ids []string) (map[string]recipeCostDefinition, error) {
	result := map[string]recipeCostDefinition{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := q.Query(`SELECT r.id,r.initial_cost_snapshot,i.item_id,i.qty_base FROM recipe_masters r
		LEFT JOIN recipe_items i ON i.recipe_master_id=r.id WHERE r.id=ANY($1) ORDER BY r.id,i.id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var snapshot []byte
		var item sql.NullString
		var qty sql.NullFloat64
		if err := rows.Scan(&id, &snapshot, &item, &qty); err != nil {
			return nil, err
		}
		d, exists := result[id]
		if !exists && len(snapshot) > 0 {
			if err := json.Unmarshal(snapshot, &d.initial); err != nil {
				return nil, err
			}
		}
		if item.Valid && qty.Valid {
			d.materials = append(d.materials, costMaterial{item.String, qty.Float64})
		}
		result[id] = d
	}
	return result, rows.Err()
}

func (c *recipeCostCatalog) recipeCost(d recipeCostDefinition, outlet string) models.RecipeCost {
	r := models.RecipeCost{InitialCost: d.initial, CostIngredients: []models.RecipeCostIngredient{}}
	total, complete := 0.0, len(d.materials) > 0
	for _, m := range d.materials {
		v, ok := c.unitCost(m.id, outlet, map[string]bool{})
		if !ok || m.qty <= 0 || !fnbFinite(m.qty) {
			complete = false
		}
		inf := c.infos[m.id]
		line := models.RecipeCostIngredient{ItemID: m.id, ItemName: inf.name, Unit: inf.unit, QtyBase: m.qty, CostPerBase: round4(v), Cost: round4(v * m.qty), IsWip: len(c.wip[m.id]) > 0}
		r.CostIngredients = append(r.CostIngredients, line)
		total += line.Cost
	}
	if complete {
		v := round4(total)
		r.CurrentCost = &v
	}
	return r
}

// Capture once, in the creation transaction. Central valuation is the shared
// recipe reference; missing prices remain unknown, never fabricated as zero.
func captureInitialRecipeCost(tx *sql.Tx, id string) error {
	c, err := loadRecipeCostCatalog(tx, []string{})
	if err != nil {
		return err
	}
	defs, err := loadRecipeCostDefinitions(tx, []string{id})
	if err != nil {
		return err
	}
	cost := c.recipeCost(defs[id], "")
	snapshot := models.RecipeCostSnapshot{CapturedAt: time.Now().UTC().Format(time.RFC3339), Source: "central", Total: cost.CurrentCost, Ingredients: cost.CostIngredients}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE recipe_masters SET initial_cost_snapshot=$2 WHERE id=$1 AND initial_cost_snapshot IS NULL`, id, string(data))
	return err
}

func getRecipeCost(id, outlet string) (models.RecipeCost, error) {
	c, err := loadRecipeCostCatalog(database.DB, []string{outlet})
	if err != nil {
		return models.RecipeCost{}, err
	}
	d, err := loadRecipeCostDefinitions(database.DB, []string{id})
	if err != nil {
		return models.RecipeCost{}, err
	}
	return c.recipeCost(d[id], outlet), nil
}

func loadRecipeCostCatalog(q recipeCostQuerier, filter []string) (*recipeCostCatalog, error) {
	// Load shared ingredient definitions; valuations below never read another outlet.
	infos := map[string]costIngredient{}
	rows, err := q.Query(`SELECT id,name,base_unit FROM stock_items`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var inf costIngredient
		if err := rows.Scan(&id, &inf.name, &inf.unit); err != nil {
			rows.Close()
			return nil, err
		}
		infos[id] = inf
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	wip := map[string][]costMaterial{}
	rows, err = q.Query(`SELECT parent_item_id,child_item_id,qty_base FROM stock_item_recipes`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var m costMaterial
		if err := rows.Scan(&id, &m.id, &m.qty); err != nil {
			rows.Close()
			return nil, err
		}
		wip[id] = append(wip[id], m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	costs := map[string]float64{}
	quantities := map[string]float64{}
	// Remaining batches are the current inventory value. Older ledger averages
	// may predate FIFO consumption; only fall back when legacy batches are incomplete.
	rows, err = q.Query(`SELECT item_id,outlet_id,
		COALESCE(SUM(CASE WHEN qty_base>0 THEN qty_base*unit_cost END)/NULLIF(SUM(CASE WHEN qty_base>0 THEN qty_base END),0),MAX(unit_cost),0),SUM(qty_base)
		FROM (SELECT l.item_id,COALESCE(w.outlet_id,'') outlet_id,l.qty_base,
			CASE WHEN b.qty>0 AND ABS(b.qty-l.qty_base)<0.0001 THEN b.value/b.qty ELSE l.avg_cost END unit_cost
			FROM stock_ledger l JOIN warehouses w ON w.id=l.warehouse_id
			LEFT JOIN LATERAL(SELECT SUM(qty_base) qty,SUM(qty_base*cost_per_base) value FROM stock_batches
				WHERE item_id=l.item_id AND warehouse_id=l.warehouse_id AND qty_base>0) b ON true
			WHERE w.is_active AND (w.type='central' OR $1 OR w.outlet_id=ANY($2))) x GROUP BY item_id,outlet_id`, filter == nil, pq.Array(filter))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, oid string
		var cost, qty float64
		if err := rows.Scan(&id, &oid, &cost, &qty); err != nil {
			rows.Close()
			return nil, err
		}
		costs[id+"|"+oid] = cost
		quantities[id+"|"+oid] = qty
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	purchases := map[string]recipePurchasePrice{}
	rows, err = q.Query(`SELECT DISTINCT ON(sm.item_id) sm.item_id,sm.cost_per_base,TO_CHAR(sm.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM stock_movements sm JOIN warehouses w ON w.id=sm.warehouse_id
		WHERE w.is_active AND w.type='central' AND sm.movement_type='purchase_in' AND sm.qty_base>0
		ORDER BY sm.item_id,sm.created_at DESC,sm.id DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var price recipePurchasePrice
		if err := rows.Scan(&id, &price.cost, &price.at); err != nil {
			rows.Close()
			return nil, err
		}
		purchases[id] = price
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return &recipeCostCatalog{infos: infos, wip: wip, costs: costs, quantities: quantities, purchases: purchases}, nil
}

// A planning estimate never revalues outlet stock or historical sales.
func (c *recipeCostCatalog) projectedUnitCost(id string, visiting map[string]bool) (float64, bool, string, string) {
	if _, ok := c.infos[id]; !ok {
		return 0, false, "missing", ""
	}
	if p, ok := c.purchases[id]; ok {
		return p.cost, p.cost > 0 && fnbFinite(p.cost), "central_purchase", p.at
	}
	if len(c.wip[id]) > 0 {
		if visiting[id] || len(visiting) >= 20 {
			return 0, false, "recipe_estimate", ""
		}
		visiting[id] = true
		defer delete(visiting, id)
		total, complete := 0.0, true
		for _, m := range c.wip[id] {
			v, ok, _, _ := c.projectedUnitCost(m.id, visiting)
			if !ok || m.qty <= 0 || !fnbFinite(m.qty) {
				complete = false
			}
			total += v * m.qty
		}
		return total, complete && fnbFinite(total), "recipe_estimate", ""
	}
	v := c.costs[id+"|"]
	return v, v > 0 && fnbFinite(v), "central_average", ""
}

func (c *recipeCostCatalog) unitCost(id, oid string, visiting map[string]bool) (float64, bool) {
	infos, costs, wip := c.infos, c.costs, c.wip
	if _, ok := infos[id]; !ok {
		return 0, false
	}
	if v := costs[id+"|"+oid]; v > 0 && fnbFinite(v) {
		return v, true
	}
	if v := costs[id+"|"]; v > 0 && fnbFinite(v) {
		return v, true
	}
	if visiting[id] || len(visiting) >= 20 || len(wip[id]) == 0 {
		return 0, false
	}
	visiting[id] = true
	defer delete(visiting, id)
	total := 0.0
	complete := true
	for _, m := range wip[id] {
		v, ok := c.unitCost(m.id, oid, visiting)
		if !ok || m.qty <= 0 || !fnbFinite(m.qty) {
			complete = false
		}
		total += v * m.qty
	}
	return total, complete && fnbFinite(total)
}
