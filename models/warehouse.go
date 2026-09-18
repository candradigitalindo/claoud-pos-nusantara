package models

// ── Stock Item ────────────────────────────────────────────────

type StockItem struct {
	ID            string  `json:"id" db:"id"`
	Code          string  `json:"code" db:"code"`
	Name          string  `json:"name" db:"name"`
	Category      string  `json:"category" db:"category"`
	BaseUnit      string  `json:"base_unit" db:"base_unit"`
	DistUnit      string  `json:"dist_unit" db:"dist_unit"`
	DistRatio     float64 `json:"dist_ratio" db:"dist_ratio"`
	DistUnitLabel string  `json:"dist_unit_label" db:"dist_unit_label"`
	MinStock      float64 `json:"min_stock" db:"min_stock"`
	TotalStock    float64 `json:"total_stock"`
	AvgCost       float64 `json:"avg_cost"`
	// Per-warehouse context (populated when a warehouse_id is supplied to list/get)
	WarehouseQty      float64 `json:"warehouse_qty"`
	WarehouseMinStock float64 `json:"warehouse_min_stock"`
	WarehouseAvgCost  float64 `json:"warehouse_avg_cost"`
	InWarehouse       bool    `json:"in_warehouse"`
	RecipeMasterID string `json:"recipe_master_id"`
	IsActive      bool    `json:"is_active" db:"is_active"`
	Notes         string  `json:"notes" db:"notes"`
	CreatedBy     string  `json:"created_by" db:"created_by"`
	CreatedAt     string  `json:"created_at" db:"created_at"`
	UpdatedAt     string  `json:"updated_at" db:"updated_at"`
}

type StockItemRequest struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	BaseUnit      string  `json:"base_unit"`
	DistUnit      string  `json:"dist_unit"`
	DistRatio     float64 `json:"dist_ratio"`
	DistUnitLabel string  `json:"dist_unit_label"`
	MinStock      float64 `json:"min_stock"`
	Notes         string  `json:"notes"`
	// Per-warehouse context. When WarehouseID is set, the per-warehouse minimum
	// stock is saved; OpeningStock seeds an opening movement on create; SetStock
	// (edit) adjusts the warehouse balance to the given absolute quantity.
	WarehouseID       string   `json:"warehouse_id"`
	WarehouseMinStock float64  `json:"warehouse_min_stock"`
	OpeningStock      float64  `json:"opening_stock"`
	OpeningCost       float64  `json:"opening_cost"`
	SetStock          *float64 `json:"set_stock"`
}

type StockItemCategory struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

type StockItemCategoryRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

// ── Warehouse ─────────────────────────────────────────────────

type Warehouse struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"` // central | outlet
	OutletID     string `json:"outlet_id"`
	OutletName   string `json:"outlet_name"`
	Address      string `json:"address"`
	IsActive     bool   `json:"is_active"`
	Notes        string `json:"notes"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type WarehouseRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	OutletID string `json:"outlet_id"`
	Address  string `json:"address"`
	IsActive bool   `json:"is_active"`
	Notes    string `json:"notes"`
}

// ── Stock Ledger & Movement ───────────────────────────────────

type StockLedgerResponse struct {
	Data             []StockLedgerRow `json:"data"`
	Total            int              `json:"total"`
	LowStockCount    int              `json:"low_stock_count"`
	TotalAssetValue  float64          `json:"total_asset_value"`
}

type StockLedgerRow struct {
	ItemID        string  `json:"item_id"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	BaseUnit      string  `json:"base_unit"`
	DistUnit      string  `json:"dist_unit"`
	DistRatio     float64 `json:"dist_ratio"`
	DistUnitLabel string  `json:"dist_unit_label"`
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseType string  `json:"warehouse_type"`
	QtyBase       float64 `json:"qty_base"`
	QtyDist       float64 `json:"qty_dist"`
	AvgCost       float64 `json:"avg_cost"`
	StockValue    float64 `json:"stock_value"`
	MinStock      float64 `json:"min_stock"`
	IsLow         bool    `json:"is_low"`
}

type StockMovement struct {
	ID           string  `json:"id"`
	ItemID       string  `json:"item_id"`
	ItemName     string  `json:"item_name"`
	WarehouseID  string  `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	MovementType string  `json:"movement_type"`
	QtyBase      float64 `json:"qty_base"`
	QtyDist      float64 `json:"qty_dist"`
	UnitUsed     string  `json:"unit_used"`
	CostPerBase  float64 `json:"cost_per_base"`
	BalanceAfter float64 `json:"balance_after"`
	RefID        string  `json:"ref_id"`
	RefType      string  `json:"ref_type"`
	RefNumber    string  `json:"ref_number"`
	ExpiryDate   *string `json:"expiry_date"`
	Notes        string  `json:"notes"`
	CreatedBy    string  `json:"created_by"`
	CreatedAt    string  `json:"created_at"`
}

type AdjustmentRequest struct {
	ItemID      string  `json:"item_id"`
	WarehouseID string  `json:"warehouse_id"`
	QtyBase     float64 `json:"qty_base"`
	CostPerBase float64 `json:"cost_per_base"`
	MovType     string  `json:"movement_type"` // adjustment | waste | spoiled | expired | purchase_in | return_in
	ExpiryDate  string  `json:"expiry_date"`  // YYYY-MM-DD
	Notes       string  `json:"notes"`
}

// ── Stock Batches (FIFO Support) ──────────────────────────────

type StockBatch struct {
	ID           string    `json:"id"`
	ItemID       string    `json:"item_id"`
	WarehouseID  string    `json:"warehouse_id"`
	QtyBase      float64   `json:"qty_base"`
	CostPerBase  float64   `json:"cost_per_base"`
	ExpiryDate   *string   `json:"expiry_date"`
	RefID        string    `json:"ref_id"`
	RefType      string    `json:"ref_type"`
	CreatedAt    string    `json:"created_at"`
}

// ── Stock Transfer ────────────────────────────────────────────

type StockTransfer struct {
	ID              string              `json:"id"`
	TransferNumber  string              `json:"transfer_number"`
	FromWarehouseID string              `json:"from_warehouse_id"`
	FromWarehouse   string              `json:"from_warehouse_name"`
	ToWarehouseID   string              `json:"to_warehouse_id"`
	ToWarehouse     string              `json:"to_warehouse_name"`
	Status          string              `json:"status"` // draft | pending | sent | received | cancelled
	Notes           string              `json:"notes"`
	// PhotoURL: bukti barang saat dikirim dari gudang asal (wajib saat 'sent').
	PhotoURL        string              `json:"photo_url"`
	Items           []StockTransferItem `json:"items"`
	CreatedBy       string              `json:"created_by"`
	ApprovedBy      string              `json:"approved_by"`
	ApprovedAt      string              `json:"approved_at"`
	SentBy          string              `json:"sent_by"`
	SentAt          *string             `json:"sent_at"`
	ReceivedBy      string              `json:"received_by"`
	ReceivedAt      *string             `json:"received_at"`
	CreatedAt       string              `json:"created_at"`
	UpdatedAt       string              `json:"updated_at"`
}

type StockTransferItem struct {
	ID              string   `json:"id"`
	TransferID      string   `json:"transfer_id"`
	ItemID          string   `json:"item_id"`
	ItemCode        string   `json:"item_code"`
	ItemName        string   `json:"item_name"`
	BaseUnit        string   `json:"base_unit"`
	DistUnit        string   `json:"dist_unit"`
	DistRatio       float64  `json:"dist_ratio"`
	DistUnitLabel   string   `json:"dist_unit_label"`
	QtyBase         float64  `json:"qty_base"`
	QtyDist         float64  `json:"qty_dist"`
	UnitUsed        string   `json:"unit_used"`
	ReceivedQtyBase *float64 `json:"received_qty_base"`
	Notes           string   `json:"notes"`
}

type StockTransferRequest struct {
	FromWarehouseID string                   `json:"from_warehouse_id"`
	ToWarehouseID   string                   `json:"to_warehouse_id"`
	Notes           string                   `json:"notes"`
	Items           []StockTransferItemReq   `json:"items"`
}

type StockTransferItemReq struct {
	ItemID  string  `json:"item_id"`
	QtyDist float64 `json:"qty_dist"`
	Notes   string  `json:"notes"`
}

// ── Product Recipe ────────────────────────────────────────────

type ProductRecipe struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	ProductName   string  `json:"product_name"`
	ItemID        string  `json:"item_id"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	DistUnit      string  `json:"dist_unit"`
	DistUnitLabel string  `json:"dist_unit_label"`
	BaseUnit      string  `json:"base_unit"`
	QtyDist       float64 `json:"qty_dist"`
	QtyBase       float64 `json:"qty_base"`
	UnitUsed      string  `json:"unit_used"`
	Visibility    string  `json:"visibility"`
	Notes         string  `json:"notes"`
}

type ProductRecipeRequest struct {
	ProductID  string              `json:"product_id"`
	Visibility string              `json:"visibility"` // public | secret
	Items      []ProductRecipeItem `json:"items"`
}

type ProductRecipeItem struct {
	ItemID     string  `json:"item_id"`
	QtyDist    float64 `json:"qty_dist"`
	Visibility string  `json:"visibility"`
	Notes      string  `json:"notes"`
}

// ── Semi-Finished Good Recipe ─────────────────────────────────

type StockItemRecipe struct {
	ID           string  `json:"id"`
	ParentItemID string  `json:"parent_item_id"`
	ChildItemID  string  `json:"child_item_id"`
	ChildName    string  `json:"child_name"`
	ChildCode    string  `json:"child_code"`
	QtyBase      float64 `json:"qty_base"`
	Unit         string  `json:"unit"`
	Visibility   string  `json:"visibility"`
	Notes        string  `json:"notes"`
}

type StockItemRecipeRequest struct {
	ParentItemID string                 `json:"parent_item_id"`
	Visibility   string                 `json:"visibility"` // public | secret
	Items        []StockItemRecipeItem  `json:"items"`
}

type StockItemRecipeItem struct {
	ItemID  string  `json:"item_id"`
	QtyBase float64 `json:"qty_base"`
	Notes   string  `json:"notes"`
}

type ProduceRequest struct {
	ItemID      string  `json:"item_id"` // Item yang diproduksi
	WarehouseID string  `json:"warehouse_id"`
	QtyProduce  float64 `json:"qty_produce"`
	Notes       string  `json:"notes"`
}

// ── Master Recipe System ──────────────────────────────────────

type RecipeMaster struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Visibility  string            `json:"visibility"` // public | secret
	Instructions string           `json:"instructions"` // JSON string of steps
	TotalTime   int               `json:"total_time"`
	Items       []RecipeItem      `json:"items"`
	OutletIDs   []string          `json:"outlet_ids"` // For secret access
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

type RecipeItem struct {
	ID        string  `json:"id"`
	ItemID    string  `json:"item_id"`
	ItemName  string  `json:"item_name"`
	ItemCode  string  `json:"item_code"`
	QtyBase   float64 `json:"qty_base"`
	Unit      string  `json:"unit"`
	Notes     string  `json:"notes"`
}

type RecipeMasterRequest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Visibility   string            `json:"visibility"`
	Instructions string            `json:"instructions"`
	TotalTime    int               `json:"total_time"`
	Items        []RecipeItemReq   `json:"items"`
	OutletIDs    []string          `json:"outlet_ids"`
}

type RecipeItemReq struct {
	ItemID  string  `json:"item_id"`
	QtyBase float64 `json:"qty_base"`
	Notes   string  `json:"notes"`
}

// ── Stock Waste ───────────────────────────────────────────

type StockWaste struct {
	ID            string  `json:"id" db:"id"`
	WasteNumber   string  `json:"waste_number" db:"waste_number"`
	WarehouseID   string  `json:"warehouse_id" db:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name" db:"warehouse_name"`
	ItemID        string  `json:"item_id" db:"item_id"`
	ItemName      string  `json:"item_name" db:"item_name"`
	ItemCode      string  `json:"item_code" db:"item_code"`
	QtyBase       float64 `json:"qty_base" db:"qty_base"`
	QtyDist       float64 `json:"qty_dist" db:"qty_dist"`
	UnitUsed      string  `json:"unit_used" db:"unit_used"`
	BaseUnit      string  `json:"base_unit" db:"base_unit"`
	DistUnit      string  `json:"dist_unit" db:"dist_unit"`
	CostPerBase   float64 `json:"cost_per_base" db:"cost_per_base"`
	TotalCost     float64 `json:"total_cost" db:"total_cost"`
	Reason        string  `json:"reason" db:"reason"` // damaged | expired | lost | other
	Notes         string  `json:"notes" db:"notes"`
	CreatedBy     string  `json:"created_by" db:"created_by"`
	CreatedAt     string  `json:"created_at" db:"created_at"`
}

type StockWasteRequest struct {
	WarehouseID string  `json:"warehouse_id"`
	ItemID      string  `json:"item_id"`
	QtyDist     float64 `json:"qty_dist"` // User inputs in dist unit
	UnitUsed    string  `json:"unit_used"`
	Reason      string  `json:"reason"`
	Notes       string  `json:"notes"`
}

// ── Warehouse Dashboard ───────────────────────────────────────

type WarehouseDashboardStats struct {
	TotalWarehouses   int                    `json:"total_warehouses"`
	CentralWarehouses int                    `json:"central_warehouses"`
	OutletWarehouses  int                    `json:"outlet_warehouses"`
	TotalItems        int                    `json:"total_items"`
	ItemsWithStock    int                    `json:"items_with_stock"`
	LowStockCount     int                    `json:"low_stock_count"`
	OutOfStockCount   int                    `json:"out_of_stock_count"`
	TotalStockValue   float64                `json:"total_stock_value"`
	PendingTransfers  int                    `json:"pending_transfers"`
	MovementsToday    int                    `json:"movements_today"`
	Movements7d       int                    `json:"movements_7d"`
	WarehouseStocks   []WarehouseStockSummary `json:"warehouse_stocks"`
	LowStockItems     []StockLedgerRow       `json:"low_stock_items"`
	RecentMovements   []StockMovement        `json:"recent_movements"`
	PendingTransferList []StockTransfer      `json:"pending_transfer_list"`
	MovementTrend     []DailyMovementPoint   `json:"movement_trend"`
}

type WarehouseStockSummary struct {
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	WarehouseType string  `json:"warehouse_type"`
	OutletName    string  `json:"outlet_name"`
	TotalItems    int     `json:"total_items"`
	TotalValue    float64 `json:"total_value"`
	LowStockCount int     `json:"low_stock_count"`
}

type DailyMovementPoint struct {
	Date  string `json:"date"`
	In    int    `json:"in"`
	Out   int    `json:"out"`
}
