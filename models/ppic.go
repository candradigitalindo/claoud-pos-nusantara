package models

// Monitoring kedaluwarsa dan bukti hitung fisik persediaan.

type ExpiryBatchRow struct {
	BatchID       string  `json:"batch_id"`
	ItemID        string  `json:"item_id"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	Category      string  `json:"category"`
	BaseUnit      string  `json:"base_unit"`
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	QtyBase       float64 `json:"qty_base"`
	CostPerBase   float64 `json:"cost_per_base"`
	Value         float64 `json:"value"`
	ExpiryDate    string  `json:"expiry_date"`
	DaysLeft      int     `json:"days_left"`
	Bucket        string  `json:"bucket"` // expired | d3 | d7 | d30 | safe
	RefType       string  `json:"ref_type"`
	CreatedAt     string  `json:"created_at"`
	AckAt         string  `json:"ack_at"`
	AckBy         string  `json:"ack_by"`
	AckNote       string  `json:"ack_note"`
}

type ExpiryBucketSummary struct {
	Bucket string  `json:"bucket"`
	Count  int     `json:"count"`
	Value  float64 `json:"value"`
}

type ExpiryResponse struct {
	Summary       []ExpiryBucketSummary `json:"summary"`
	NoExpiryCount int                   `json:"no_expiry_count"` // batch qty>0 tanpa tanggal expiry
	Data          []ExpiryBatchRow      `json:"data"`
	Total         int                   `json:"total"`
}

type ExpiryAckRequest struct {
	Note string `json:"note"`
}

// ── Stock Opname (Cycle Count) ────────────────────────────────

type StockOpname struct {
	ID            string  `json:"id"`
	OpnameNumber  string  `json:"opname_number"`
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseType string  `json:"warehouse_type"`
	OutletName    string  `json:"outlet_name"`
	Category      string  `json:"category"` // kosong = semua kategori
	Status        string  `json:"status"`   // counting | review | approved | cancelled
	Notes         string  `json:"notes"`
	CreatedBy     string  `json:"created_by"`
	ApprovedBy    string  `json:"approved_by"`
	ApprovedAt    *string `json:"approved_at"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`

	ItemsTotal   int     `json:"items_total"`
	ItemsCounted int     `json:"items_counted"`
	ItemsDiff    int     `json:"items_diff"`
	DiffValue    float64 `json:"diff_value"`
	AccuracyPct  float64 `json:"accuracy_pct"` // IRA: item tanpa selisih / item dihitung

	Items []StockOpnameItem `json:"items,omitempty"`
}

type StockOpnameItem struct {
	ID             string   `json:"id"`
	ItemID         string   `json:"item_id"`
	ItemCode       string   `json:"item_code"`
	ItemName       string   `json:"item_name"`
	Category       string   `json:"category"`
	BaseUnit       string   `json:"base_unit"`
	DistUnit       string   `json:"dist_unit"`
	DistRatio      float64  `json:"dist_ratio"`
	DistUnitLabel  string   `json:"dist_unit_label"`
	QtySystemBase  float64  `json:"qty_system_base"`
	QtyCountedBase *float64 `json:"qty_counted_base"`
	CostPerBase    float64  `json:"cost_per_base"`
	DiffBase       float64  `json:"diff_base"`
	DiffValue      float64  `json:"diff_value"`
	Reason         string   `json:"reason"`
}

type StockOpnameCreateRequest struct {
	WarehouseID string `json:"warehouse_id"`
	Category    string `json:"category"` // opsional: batasi per kategori (ABC cycle count)
	Notes       string `json:"notes"`
}

type OpnameCountItem struct {
	ItemID         string   `json:"item_id"`
	QtyCountedBase *float64 `json:"qty_counted_base"`
	Reason         string   `json:"reason"`
}

type OpnameCountRequest struct {
	Items []OpnameCountItem `json:"items"`
}
