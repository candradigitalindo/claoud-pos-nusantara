package models

// Policies are explicit per item/warehouse. A nil policy means no limit has
// been agreed; zero is a valid, deliberately strict tolerance.
type PpicUsagePolicy struct {
	ItemID            string  `json:"item_id"`
	WarehouseID       string  `json:"warehouse_id"`
	UsageTolerancePct float64 `json:"usage_tolerance_pct"`
	WasteTolerancePct float64 `json:"waste_tolerance_pct"`
	Version           int     `json:"version"`
	Reason            string  `json:"reason"`
	UpdatedBy         string  `json:"updated_by"`
	UpdatedAt         string  `json:"updated_at"`
}

type PpicUsagePolicyInput struct {
	ItemID            string   `json:"item_id"`
	WarehouseID       string   `json:"warehouse_id"`
	UsageTolerancePct *float64 `json:"usage_tolerance_pct"`
	WasteTolerancePct *float64 `json:"waste_tolerance_pct"`
	Version           int      `json:"version"`
	Reason            string   `json:"reason"`
}

type PpicUsageWarehouse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
}

type PpicUsageItem struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Category string `json:"category"`
	BaseUnit string `json:"base_unit"`
}

type PpicUsageLookups struct {
	Warehouses []PpicUsageWarehouse `json:"warehouses"`
	Items      []PpicUsageItem      `json:"items"`
}

type PpicUsageControlRow struct {
	ItemID                 string           `json:"item_id"`
	ItemCode               string           `json:"item_code"`
	ItemName               string           `json:"item_name"`
	Category               string           `json:"category"`
	BaseUnit               string           `json:"base_unit"`
	WarehouseID            string           `json:"warehouse_id"`
	WarehouseName          string           `json:"warehouse_name"`
	WarehouseType          string           `json:"warehouse_type"`
	OutletName             string           `json:"outlet_name"`
	StockQty               float64          `json:"stock_qty"`
	SaleQty                float64          `json:"sale_qty"`
	ProductionQty          float64          `json:"production_qty"`
	ProductionReferenceQty float64          `json:"production_reference_qty"`
	BreakdownQty           float64          `json:"breakdown_qty"`
	OtherUsageQty          float64          `json:"other_usage_qty"`
	StocktakeShortageQty   float64          `json:"stocktake_shortage_qty"`
	StocktakeSurplusQty    float64          `json:"stocktake_surplus_qty"`
	AdjustmentShortageQty  float64          `json:"adjustment_shortage_qty"`
	TransferOutQty         float64          `json:"transfer_out_qty"`
	QualityHoldQty         float64          `json:"quality_hold_qty"`
	ReferenceQty           float64          `json:"reference_qty"`
	EvaluatedUsageQty      float64          `json:"evaluated_usage_qty"`
	ExcessUsageQty         float64          `json:"excess_usage_qty"`
	WasteQty               float64          `json:"waste_qty"`
	VoidWasteQty           float64          `json:"void_waste_qty"`
	UndocumentedWasteQty   float64          `json:"undocumented_waste_qty"`
	WasteValue             float64          `json:"waste_value"`
	ShortageValue          float64          `json:"shortage_value"`
	UsageLimitQty          *float64         `json:"usage_limit_qty"`
	WasteLimitQty          *float64         `json:"waste_limit_qty"`
	WastePct               *float64         `json:"waste_pct"`
	UsageVariancePct       *float64         `json:"usage_variance_pct"`
	Policy                 *PpicUsagePolicy `json:"policy"`
	StocktakeAt            string           `json:"stocktake_at"`
	StocktakeNumber        string           `json:"stocktake_number"`
	HasStocktake           bool             `json:"has_stocktake"`
	Status                 string           `json:"status"`
	Flags                  []string         `json:"flags"`
}

type PpicUsageControlSummary struct {
	ItemsCount        int     `json:"items_count"`
	OverLimitCount    int     `json:"over_limit_count"`
	UnsetCount        int     `json:"unset_count"`
	NeedsCount        int     `json:"needs_count"`
	NoBasisCount      int     `json:"no_basis_count"`
	WasteValue        float64 `json:"waste_value"`
	ShortageValue     float64 `json:"shortage_value"`
	PendingDeductions int     `json:"pending_deductions"`
}

type PpicUsageControlReport struct {
	DateFrom  string                  `json:"date_from"`
	DateTo    string                  `json:"date_to"`
	Rows      []PpicUsageControlRow   `json:"rows"`
	TotalRows int                     `json:"total_rows"`
	Summary   PpicUsageControlSummary `json:"summary"`
	Notes     []string                `json:"notes"`
}
