package models

type StockEvidence struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	RecordedAt string `json:"recorded_at"`
	ActorName  string `json:"actor_name"`
}

type StockQualityCase struct {
	ID                  string              `json:"id"`
	Number              string              `json:"number"`
	WarehouseID         string              `json:"warehouse_id"`
	WarehouseName       string              `json:"warehouse_name"`
	WarehouseType       string              `json:"warehouse_type"`
	ItemID              string              `json:"item_id"`
	ItemName            string              `json:"item_name"`
	BaseUnit            string              `json:"base_unit"`
	QtyBase             float64             `json:"qty_base"`
	QtyInput            float64             `json:"qty_input"`
	UnitUsed            string              `json:"unit_used"`
	CostPerBase         float64             `json:"cost_per_base"`
	ExpiryDate          string              `json:"expiry_date"`
	Reason              string              `json:"reason"`
	Notes               string              `json:"notes"`
	Status              string              `json:"status"`
	Disposition         string              `json:"disposition"`
	Settlement          string              `json:"settlement"`
	VendorID            string              `json:"vendor_id"`
	VendorName          string              `json:"vendor_name"`
	SupplierReference   string              `json:"supplier_reference"`
	ReturnWarehouseID   string              `json:"return_warehouse_id"`
	ReturnWarehouseName string              `json:"return_warehouse_name"`
	TransferID          string              `json:"transfer_id"`
	ParentCaseID        string              `json:"parent_case_id"`
	SettledQtyBase      float64             `json:"settled_qty_base"`
	RefundAmount        float64             `json:"refund_amount"`
	CreatedBy           string              `json:"created_by"`
	CreatedName         string              `json:"created_name"`
	ApprovedBy          string              `json:"approved_by"`
	Version             int                 `json:"version"`
	CreatedAt           string              `json:"created_at"`
	Events              []StockQualityEvent `json:"events,omitempty"`
}
type StockQualityEvent struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	ActorName string         `json:"actor_name"`
	Notes     string         `json:"notes"`
	QtyBase   float64        `json:"qty_base"`
	Amount    float64        `json:"amount"`
	Reference string         `json:"reference"`
	CreatedAt string         `json:"created_at"`
	Evidence  *StockEvidence `json:"evidence,omitempty"`
}
type StockQualityRequest struct {
	WarehouseID string  `json:"warehouse_id"`
	ItemID      string  `json:"item_id"`
	BatchID     string  `json:"batch_id"`
	Qty         float64 `json:"qty"`
	UnitUsed    string  `json:"unit_used"`
	Reason      string  `json:"reason"`
	Notes       string  `json:"notes"`
	EvidenceID  string  `json:"evidence_id"`
}
type StockQualityAction struct {
	Action            string  `json:"action"`
	Version           int     `json:"version"`
	Notes             string  `json:"notes"`
	EvidenceID        string  `json:"evidence_id"`
	Disposition       string  `json:"disposition"`
	Settlement        string  `json:"settlement"`
	VendorID          string  `json:"vendor_id"`
	Reference         string  `json:"reference"`
	ReturnWarehouseID string  `json:"return_warehouse_id"`
	QtyBase           float64 `json:"qty_base"`
	Amount            float64 `json:"amount"`
	ExpiryDate        string  `json:"expiry_date"`
}
type StockInspectionItem struct {
	ItemID   string  `json:"item_id"`
	Accepted float64 `json:"accepted_qty_base"`
	Rejected float64 `json:"rejected_qty_base"`
	Reason   string  `json:"reason"`
}
type StockInspectionRequest struct {
	Version    int                   `json:"version"`
	EvidenceID string                `json:"evidence_id"`
	Notes      string                `json:"notes"`
	Items      []StockInspectionItem `json:"items"`
}
type StockTransferReceipt struct {
	ID        string                `json:"id"`
	ActorName string                `json:"actor_name"`
	Notes     string                `json:"notes"`
	CreatedAt string                `json:"created_at"`
	Evidence  StockEvidence         `json:"evidence"`
	Items     []StockInspectionItem `json:"items"`
}
