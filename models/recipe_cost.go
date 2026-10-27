package models

// RecipeCostIngredient is shared by recipe snapshots and F&B costing.
type RecipeCostIngredient struct {
	ItemID      string  `json:"item_id"`
	ItemName    string  `json:"item_name"`
	IsWip       bool    `json:"is_wip"` // punya resep internal (barang setengah jadi)
	QtyBase     float64 `json:"qty_base"`
	Unit        string  `json:"unit"`
	CostPerBase float64 `json:"cost_per_base"`
	Cost        float64 `json:"cost"`
}
