package models

// ── PPIC Dashboard ────────────────────────────────────────────

type PpicDashboardStats struct {
	// KPI
	TotalStockValue  float64 `json:"total_stock_value"`
	BelowRopCount    int     `json:"below_rop_count"`
	OutOfStockCount  int     `json:"out_of_stock_count"`
	CoverageDays     float64 `json:"coverage_days"`      // DOI agregat: nilai stok / rata-rata pemakaian harian (28 hari)
	UsageValue30d    float64 `json:"usage_value_30d"`    // nilai pemakaian (stok keluar) 30 hari
	TurnoverRatio30d float64 `json:"turnover_ratio_30d"` // pemakaian 30 hari / nilai stok saat ini (aproksimasi ITR)
	ExpiredCount     int     `json:"expired_count"`
	ExpiredValue     float64 `json:"expired_value"`
	Expiring7dCount  int     `json:"expiring_7d_count"`
	Expiring7dValue  float64 `json:"expiring_7d_value"`
	WasteValue30d    float64 `json:"waste_value_30d"`
	WastePct30d      float64 `json:"waste_pct_30d"` // nilai waste / nilai pemakaian × 100
	DeadStockCount   int     `json:"dead_stock_count"`
	DeadStockValue   float64 `json:"dead_stock_value"`
	// Fase 2: akurasi forecast 7 hari terakhir (100 − MAPE); EvalRows 0 = belum ada data
	ForecastAccuracy7d float64 `json:"forecast_accuracy_7d"`
	ForecastEvalRows   int     `json:"forecast_eval_rows"`

	LastOpname *PpicOpnameSummary `json:"last_opname"`

	Alerts         []PpicAlert           `json:"alerts"`
	UsageTrend     []PpicDailyValuePoint `json:"usage_trend"`     // nilai masuk vs keluar per hari, 30 hari
	TopWaste       []PpicWasteItem       `json:"top_waste"`       // top item waste by nilai, 30 hari
	ExpiryCalendar []PpicExpiryPoint     `json:"expiry_calendar"` // nilai batch yang akan kedaluwarsa per hari, 30 hari ke depan
	WarehouseRows  []PpicWarehouseRow    `json:"warehouse_rows"`
}

type PpicAlert struct {
	Type          string  `json:"type"`     // expired | expiring | stockout | below_rop | dead_stock
	Severity      string  `json:"severity"` // red | amber | gray
	ItemID        string  `json:"item_id"`
	ItemName      string  `json:"item_name"`
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	Qty           float64 `json:"qty"`
	Unit          string  `json:"unit"`
	Value         float64 `json:"value"`
	Date          string  `json:"date"` // tanggal expiry / info tambahan
	Message       string  `json:"message"`
}

type PpicDailyValuePoint struct {
	Date     string  `json:"date"`
	InValue  float64 `json:"in_value"`
	OutValue float64 `json:"out_value"`
}

type PpicWasteItem struct {
	ItemID   string  `json:"item_id"`
	ItemName string  `json:"item_name"`
	BaseUnit string  `json:"base_unit"`
	TotalQty float64 `json:"total_qty"`
	Value    float64 `json:"value"`
}

type PpicExpiryPoint struct {
	Date  string  `json:"date"`
	Count int     `json:"count"`
	Value float64 `json:"value"`
}

type PpicWarehouseRow struct {
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseType string  `json:"warehouse_type"`
	OutletName    string  `json:"outlet_name"`
	StockValue    float64 `json:"stock_value"`
	BelowRopCount int     `json:"below_rop_count"`
	ExpiringValue float64 `json:"expiring_value"` // nilai batch ≤7 hari (termasuk yang sudah lewat)
}

type PpicOpnameSummary struct {
	ID            string  `json:"id"`
	OpnameNumber  string  `json:"opname_number"`
	WarehouseName string  `json:"warehouse_name"`
	Status        string  `json:"status"`
	ItemsTotal    int     `json:"items_total"`
	ItemsCounted  int     `json:"items_counted"`
	ItemsDiff     int     `json:"items_diff"`
	AccuracyPct   float64 `json:"accuracy_pct"`
	CreatedAt     string  `json:"created_at"`
}

// ── Par Level & ROP (item_planning_params) ────────────────────

type PlanningParamRow struct {
	ItemID        string  `json:"item_id"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	Category      string  `json:"category"`
	BaseUnit      string  `json:"base_unit"`
	DistUnit      string  `json:"dist_unit"`
	DistRatio     float64 `json:"dist_ratio"`
	DistUnitLabel string  `json:"dist_unit_label"`
	WarehouseID   string  `json:"warehouse_id"`
	QtyBase       float64 `json:"qty_base"`
	AvgCost       float64 `json:"avg_cost"`
	MinStock      float64 `json:"min_stock"` // per-warehouse (stock_ledger.min_stock), fallback lama
	AvgDailyUsage float64 `json:"avg_daily_usage"`
	StdDailyUsage float64 `json:"std_daily_usage"`
	// Parameter tersimpan
	HasParams    bool    `json:"has_params"`
	LeadTimeDays int     `json:"lead_time_days"`
	SafetyStock  float64 `json:"safety_stock"`
	ReorderPoint float64 `json:"reorder_point"`
	ParLevel     float64 `json:"par_level"`
	Moq          float64 `json:"moq"`
	// Saran sistem (endpoint suggest)
	SugSafetyStock  float64 `json:"sug_safety_stock"`
	SugReorderPoint float64 `json:"sug_reorder_point"`
	SugParLevel     float64 `json:"sug_par_level"`
	IsBelowRop      bool    `json:"is_below_rop"`
}

type PlanningParamsResponse struct {
	Data  []PlanningParamRow `json:"data"`
	Total int                `json:"total"`
}

type PlanningParamUpsert struct {
	ItemID       string  `json:"item_id"`
	WarehouseID  string  `json:"warehouse_id"`
	LeadTimeDays int     `json:"lead_time_days"`
	SafetyStock  float64 `json:"safety_stock"`
	ReorderPoint float64 `json:"reorder_point"`
	ParLevel     float64 `json:"par_level"`
	Moq          float64 `json:"moq"`
}

type PlanningParamsUpdateRequest struct {
	Rows []PlanningParamUpsert `json:"rows"`
}

// ── Monitor Kedaluwarsa (FEFO) ────────────────────────────────

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

// ── Demand Forecast (Fase 2) ──────────────────────────────────

type ForecastCell struct {
	Date      string   `json:"date"`
	QtySystem float64  `json:"qty_system"`
	QtyManual *float64 `json:"qty_manual"`
	QtyActual *float64 `json:"qty_actual"`
	EventNote string   `json:"event_note"`
}

type ForecastRow struct {
	OutletID    string         `json:"outlet_id"`
	OutletName  string         `json:"outlet_name"`
	ProductName string         `json:"product_name"`
	Category    string         `json:"category"`
	Cells       []ForecastCell `json:"cells"`
	// Evaluasi 14 hari terakhir (baris dengan aktual > 0)
	AccuracyPct float64 `json:"accuracy_pct"`
	EvalRows    int     `json:"eval_rows"`
}

type ForecastResponse struct {
	Dates       []string      `json:"dates"`
	Rows        []ForecastRow `json:"rows"`
	Total       int           `json:"total"`      // total produk (untuk paginasi)
	Categories  []string      `json:"categories"` // kategori produk dalam scope (untuk filter)
	Accuracy14d float64       `json:"accuracy_14d"`
	EvalRows14d int           `json:"eval_rows_14d"`
}

type ForecastHistoryPoint struct {
	Date string  `json:"date"`
	Qty  float64 `json:"qty"`
}

type ForecastUpdateRow struct {
	OutletID    string   `json:"outlet_id"`
	ProductName string   `json:"product_name"`
	Date        string   `json:"date"`
	QtyManual   *float64 `json:"qty_manual"` // null = kembali ke angka sistem
	EventNote   string   `json:"event_note"`
}

type ForecastUpdateRequest struct {
	Rows []ForecastUpdateRow `json:"rows"`
}

type ForecastGenerateRequest struct {
	HorizonDays int `json:"horizon_days"` // default 7
}

// ── MRP — Rencana Kebutuhan Bahan (Fase 2) ────────────────────

type MrpRunRequest struct {
	WarehouseID string `json:"warehouse_id"`
	HorizonDays int    `json:"horizon_days"` // default 7
	IncludePar  bool   `json:"include_par"`  // sertakan isi-ulang par level
}

type MrpRun struct {
	ID               string  `json:"id"`
	RunNumber        string  `json:"run_number"`
	WarehouseID      string  `json:"warehouse_id"`
	WarehouseName    string  `json:"warehouse_name"`
	WarehouseType    string  `json:"warehouse_type"`
	HorizonDays      int     `json:"horizon_days"`
	IncludePar       bool    `json:"include_par"`
	Status           string  `json:"status"`
	NoRecipeCount    int     `json:"no_recipe_count"`
	NoRecipeProducts string  `json:"no_recipe_products"` // contoh nama, dipotong
	Notes            string  `json:"notes"`
	CreatedBy        string  `json:"created_by"`
	CreatedAt        string  `json:"created_at"`
	ForecastQty      float64 `json:"forecast_qty"` // total porsi forecast yang diledakkan

	Items []MrpRunItem `json:"items,omitempty"`
}

type MrpRunItem struct {
	ID               string  `json:"id"`
	ItemID           string  `json:"item_id"`
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	Category         string  `json:"category"`
	BaseUnit         string  `json:"base_unit"`
	DistUnit         string  `json:"dist_unit"`
	DistRatio        float64 `json:"dist_ratio"`
	DistUnitLabel    string  `json:"dist_unit_label"`
	GrossReq         float64 `json:"gross_req"`
	OnHand           float64 `json:"on_hand"`
	OnOrder          float64 `json:"on_order"`
	InTransit        float64 `json:"in_transit"`
	SafetyStock      float64 `json:"safety_stock"`
	ParLevel         float64 `json:"par_level"`
	Moq              float64 `json:"moq"`
	NetReq           float64 `json:"net_req"`
	Suggestion       string  `json:"suggestion"` // purchase | transfer | produce | none
	QtySuggestedBase float64 `json:"qty_suggested_base"`
	QtySuggestedDist float64 `json:"qty_suggested_dist"`
	SourceWarehouse  string  `json:"source_warehouse_id"`
	SourceWhName     string  `json:"source_warehouse_name"`
	LastVendor       string  `json:"last_vendor"`
	LastPriceDist    float64 `json:"last_price_dist"`
	ActionRefType    string  `json:"action_ref_type"`
	ActionRefID      string  `json:"action_ref_id"`
	ActionRefNumber  string  `json:"action_ref_number"`
}

type MrpExecuteRequest struct {
	ItemIDs []string `json:"item_ids"`
	Notes   string   `json:"notes"`
}

// ── Rencana Produksi / MPS + Work Order (Fase 3) ──────────────

type ProductionPlan struct {
	ID            string  `json:"id"`
	PlanNumber    string  `json:"plan_number"`
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	PlanDate      string  `json:"plan_date"`
	Status        string  `json:"status"` // draft | approved | released | closed | cancelled
	Notes         string  `json:"notes"`
	CreatedBy     string  `json:"created_by"`
	ApprovedBy    string  `json:"approved_by"`
	ApprovedAt    *string `json:"approved_at"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	ItemCount     int     `json:"item_count"`
	WoTotal       int     `json:"wo_total"`
	WoDone        int     `json:"wo_done"`

	Items []ProductionPlanItem `json:"items,omitempty"`
}

type ProductionPlanItem struct {
	ID              string  `json:"id"`
	ItemID          string  `json:"item_id"`
	ItemCode        string  `json:"item_code"`
	ItemName        string  `json:"item_name"`
	BaseUnit        string  `json:"base_unit"`
	QtyPlannedBase  float64 `json:"qty_planned_base"`
	QtyForecastBase float64 `json:"qty_forecast_base"` // kolom bantu: kebutuhan menurut MRP/forecast
	OnHand          float64 `json:"on_hand"`
	ParLevel        float64 `json:"par_level"`
	Notes           string  `json:"notes"`
	WoID            string  `json:"wo_id"`
	WoNumber        string  `json:"wo_number"`
	WoStatus        string  `json:"wo_status"`
}

type ProductionPlanItemReq struct {
	ItemID         string  `json:"item_id"`
	QtyPlannedBase float64 `json:"qty_planned_base"`
	Notes          string  `json:"notes"`
}

type ProductionPlanRequest struct {
	WarehouseID string                  `json:"warehouse_id"`
	PlanDate    string                  `json:"plan_date"` // YYYY-MM-DD
	Notes       string                  `json:"notes"`
	Items       []ProductionPlanItemReq `json:"items"`
}

type WorkOrder struct {
	ID             string   `json:"id"`
	WoNumber       string   `json:"wo_number"`
	PlanID         string   `json:"plan_id"`
	PlanNumber     string   `json:"plan_number"`
	WarehouseID    string   `json:"warehouse_id"`
	WarehouseName  string   `json:"warehouse_name"`
	ItemID         string   `json:"item_id"`
	ItemCode       string   `json:"item_code"`
	ItemName       string   `json:"item_name"`
	BaseUnit       string   `json:"base_unit"`
	ShelfLifeDays  int      `json:"shelf_life_days"`
	QtyPlannedBase float64  `json:"qty_planned_base"`
	QtyActualBase  *float64 `json:"qty_actual_base"`
	YieldPct       float64  `json:"yield_pct"`
	Status         string   `json:"status"` // planned | in_progress | done | cancelled
	StartedAt      *string  `json:"started_at"`
	FinishedAt     *string  `json:"finished_at"`
	ExecutedBy     string   `json:"executed_by"`
	Notes          string   `json:"notes"`
	CreatedBy      string   `json:"created_by"`
	CreatedAt      string   `json:"created_at"`
	CostTotal      float64  `json:"cost_total"`
	CostPerUnit    float64  `json:"cost_per_unit"`
	ExpiryDate     string   `json:"expiry_date"` // batch hasil produksi

	Materials []WorkOrderMaterial `json:"materials,omitempty"`
}

type WorkOrderMaterial struct {
	ID            string   `json:"id"`
	ItemID        string   `json:"item_id"`
	ItemCode      string   `json:"item_code"`
	ItemName      string   `json:"item_name"`
	BaseUnit      string   `json:"base_unit"`
	QtyPlanBase   float64  `json:"qty_plan_base"`
	QtyActualBase *float64 `json:"qty_actual_base"`
	CostPerBase   float64  `json:"cost_per_base"` // estimasi saat dibuat; aktual terisi saat selesai
	OnHand        float64  `json:"on_hand"`
	VariancePct   float64  `json:"variance_pct"` // (aktual − plan) / plan × 100
}

type WorkOrderCreateRequest struct {
	WarehouseID    string  `json:"warehouse_id"`
	ItemID         string  `json:"item_id"`
	QtyPlannedBase float64 `json:"qty_planned_base"`
	Notes          string  `json:"notes"`
}

type WorkOrderFinishMaterial struct {
	ItemID        string  `json:"item_id"`
	QtyActualBase float64 `json:"qty_actual_base"`
}

type WorkOrderFinishRequest struct {
	QtyActualBase float64                   `json:"qty_actual_base"`
	Materials     []WorkOrderFinishMaterial `json:"materials"` // kosong = pakai resep × qty aktual
	Notes         string                    `json:"notes"`
}

// ── Laporan PPIC (Fase 3) ─────────────────────────────────────

type PpicVarianceRow struct {
	ItemID      string  `json:"item_id"`
	ItemCode    string  `json:"item_code"`
	ItemName    string  `json:"item_name"`
	Category    string  `json:"category"`
	BaseUnit    string  `json:"base_unit"`
	TheoQty     float64 `json:"theo_qty"`
	TheoValue   float64 `json:"theo_value"`
	ActQty      float64 `json:"act_qty"`
	ActValue    float64 `json:"act_value"`
	DiffQty     float64 `json:"diff_qty"`
	DiffValue   float64 `json:"diff_value"`
	VariancePct float64 `json:"variance_pct"` // (aktual − teoretis) / teoretis × 100
	IsOver      bool    `json:"is_over"`      // di atas ambang (default 5%)
}

type PpicVarianceReport struct {
	DateFrom          string            `json:"date_from"`
	DateTo            string            `json:"date_to"`
	Rows              []PpicVarianceRow `json:"rows"`
	TotalRows         int               `json:"total_rows"`
	TheoValue         float64           `json:"theo_value"`
	ActValue          float64           `json:"act_value"`
	VariancePct       float64           `json:"variance_pct"`
	Revenue           float64           `json:"revenue"`
	FoodCostPct       float64           `json:"food_cost_pct"`
	QtySold           float64           `json:"qty_sold"`
	QtySoldWithRecipe float64           `json:"qty_sold_with_recipe"`
	RecipeCoveragePct float64           `json:"recipe_coverage_pct"`
	Representative    bool              `json:"representative"` // coverage ≥ 80%
}

type PpicProductionRow struct {
	WoNumber      string  `json:"wo_number"`
	PlanNumber    string  `json:"plan_number"`
	WarehouseName string  `json:"warehouse_name"`
	ItemName      string  `json:"item_name"`
	BaseUnit      string  `json:"base_unit"`
	QtyPlanned    float64 `json:"qty_planned"`
	QtyActual     float64 `json:"qty_actual"`
	YieldPct      float64 `json:"yield_pct"`
	CostTotal     float64 `json:"cost_total"`
	CostPerUnit   float64 `json:"cost_per_unit"`
	MatVariance   float64 `json:"mat_variance_value"` // nilai (aktual − plan) bahan
	FinishedAt    string  `json:"finished_at"`
	ExecutedBy    string  `json:"executed_by"`
}

type PpicProductionReport struct {
	Rows          []PpicProductionRow `json:"rows"`
	TotalRows     int                 `json:"total_rows"`
	WoDone        int                 `json:"wo_done"`
	AvgYieldPct   float64             `json:"avg_yield_pct"`
	TotalCost     float64             `json:"total_cost"`
	TotalMatVar   float64             `json:"total_mat_variance"`
	PlanAdherence float64             `json:"plan_adherence_pct"` // Σ aktual / Σ rencana
}

type PpicForecastAccRow struct {
	OutletName  string  `json:"outlet_name"`
	ProductName string  `json:"product_name"`
	EvalRows    int     `json:"eval_rows"`
	AccuracyPct float64 `json:"accuracy_pct"`
	BiasPct     float64 `json:"bias_pct"` // rata-rata (forecast − aktual)/aktual; + = over-forecast
}

type PpicForecastAccReport struct {
	Rows        []PpicForecastAccRow `json:"rows"`
	TotalRows   int                  `json:"total_rows"`
	AccuracyPct float64              `json:"accuracy_pct"`
	EvalRows    int                  `json:"eval_rows"`
}

type PpicOtifRow struct {
	VendorName         string  `json:"vendor_name"`
	TotalPR            int     `json:"total_pr"`
	Received           int     `json:"received"`
	OnTime             int     `json:"on_time"`
	Late               int     `json:"late"`
	OutstandingOverdue int     `json:"outstanding_overdue"`
	OtifPct            float64 `json:"otif_pct"`
	AvgLeadDays        float64 `json:"avg_lead_days"`
}

type PpicOtifReport struct {
	Rows      []PpicOtifRow `json:"rows"`
	TotalRows int           `json:"total_rows"`
	TotalPR   int           `json:"total_pr"`
	OtifPct   float64       `json:"otif_pct"`
	Note      string        `json:"note"`
}

// ── Laporan Distribusi Induk → Outlet ─────────────────────────
// Matriks per item: qty dikirim dari gudang induk ke tiap outlet dalam periode,
// pemakaian outlet (jual/waste/lainnya), dan stok kini — untuk tracking moving
// item saat stock opname dan membandingkan kiriman vs stok semua outlet.

type PpicDistOutletCol struct {
	OutletID      string `json:"outlet_id"`
	OutletName    string `json:"outlet_name"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
}

type PpicDistCell struct {
	Delivered     float64 `json:"delivered"`      // qty_base diterima dari induk dalam periode
	DeliveryCount int     `json:"delivery_count"` // jumlah baris kiriman
	LastDelivery  string  `json:"last_delivery"`  // waktu kiriman terakhir
	SaleOut       float64 `json:"sale_out"`       // terpakai penjualan (qty_base, positif)
	WasteOut      float64 `json:"waste_out"`      // waste/spoiled/expired
	OtherOut      float64 `json:"other_out"`      // keluar lain (produksi, transfer keluar, koreksi minus)
	OtherIn       float64 `json:"other_in"`       // masuk selain dari induk (GRN, produksi, koreksi plus)
	CurrentQty    float64 `json:"current_qty"`    // stok kini gudang outlet
}

type PpicDistRow struct {
	ItemID         string                  `json:"item_id"`
	ItemCode       string                  `json:"item_code"`
	ItemName       string                  `json:"item_name"`
	Category       string                  `json:"category"`
	BaseUnit       string                  `json:"base_unit"`
	CentralQty     float64                 `json:"central_qty"`      // stok kini gudang induk
	TotalDelivered float64                 `json:"total_delivered"`  // Σ kiriman semua outlet
	TotalOutletQty float64                 `json:"total_outlet_qty"` // Σ stok kini semua outlet
	Cells          map[string]PpicDistCell `json:"cells"`            // key = outlet_id
}

type PpicDistributionReport struct {
	DateFrom            string              `json:"date_from"`
	DateTo              string              `json:"date_to"`
	Outlets             []PpicDistOutletCol `json:"outlets"`
	Rows                []PpicDistRow       `json:"rows"`
	TotalRows           int                 `json:"total_rows"`
	TotalDeliveredValue float64             `json:"total_delivered_value"` // Σ qty × cost kiriman
	DeliveryDocs        int                 `json:"delivery_docs"`         // dokumen transfer unik
	ItemsDelivered      int                 `json:"items_delivered"`       // item dengan kiriman > 0
}

// ── HPP Menu / Costing Card (Fase 3) ──────────────────────────

type PpicHppIngredient struct {
	ItemID      string  `json:"item_id"`
	ItemName    string  `json:"item_name"`
	IsWip       bool    `json:"is_wip"` // punya resep internal (barang setengah jadi)
	QtyBase     float64 `json:"qty_base"`
	Unit        string  `json:"unit"`
	CostPerBase float64 `json:"cost_per_base"`
	Cost        float64 `json:"cost"`
}

type PpicHppRow struct {
	TargetCostPct    float64             `json:"target_cost_pct"`
	CostStatus       string              `json:"cost_status"`
	CostBasis        string              `json:"cost_basis"`
	PackagingCost    float64             `json:"packaging_cost"`
	OverheadCost     float64             `json:"overhead_cost"`
	ProjectedHpp     *float64            `json:"projected_hpp"`
	ProjectedCostPct *float64            `json:"projected_cost_pct"`
	ProjectedIsOver  bool                `json:"projected_is_over"`
	ProductID        string              `json:"product_id"`
	OutletID         string              `json:"outlet_id"`
	OutletName       string              `json:"outlet_name"`
	ProductName      string              `json:"product_name"`
	Category         string              `json:"category"`
	Price            float64             `json:"price"`
	Hpp              float64             `json:"hpp"`
	HppPct           float64             `json:"hpp_pct"`    // HPP / harga jual × 100
	Margin           float64             `json:"margin"`     // harga − HPP
	MarginPct        float64             `json:"margin_pct"` // margin / harga × 100
	IdealCost        float64             `json:"ideal_cost"` // harga × ambang ideal
	IsOver           bool                `json:"is_over"`    // HPP% di atas ambang
	HasRecipe        bool                `json:"has_recipe"`
	Ingredients      []PpicHppIngredient `json:"ingredients,omitempty"`
}

type PpicHppReport struct {
	IdealPct       float64      `json:"ideal_pct"`
	Rows           []PpicHppRow `json:"rows"`
	TotalRows      int          `json:"total_rows"`
	ProductCount   int          `json:"product_count"`
	WithRecipe     int          `json:"with_recipe"`
	OverCount      int          `json:"over_count"`
	AvgHppPct      float64      `json:"avg_hpp_pct"` // rata-rata menu ber-resep & ber-harga
	NoRecipeSample string       `json:"no_recipe_sample"`
}

// ── Produk Terjual (qty-only, untuk PPIC) ─────────────────────

type PpicSoldRow struct {
	OutletName  string  `json:"outlet_name"`
	ProductName string  `json:"product_name"`
	Category    string  `json:"category"`
	Qty         float64 `json:"qty"`
	AvgPerDay   float64 `json:"avg_per_day"`
	SharePct    float64 `json:"share_pct"` // kontribusi terhadap total qty
	HasRecipe   bool    `json:"has_recipe"`
}

type PpicSoldReport struct {
	DateFrom     string        `json:"date_from"`
	DateTo       string        `json:"date_to"`
	Days         int           `json:"days"`
	Rows         []PpicSoldRow `json:"rows"`
	TotalRows    int           `json:"total_rows"`
	TotalQty     float64       `json:"total_qty"`
	ProductCount int           `json:"product_count"`
	NoRecipeQty  float64       `json:"no_recipe_qty"` // qty terjual dari produk tanpa resep
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
