package models

// ProductBundleComponent mendefinisikan komponen produk dari outlet mitra yang
// menjadi bagian dari produk bundling di outlet sumber (mis. GOLD ATV di ATV Overland
// menyertakan DONAT KAMPUNG GD di Gadis Desa).
type ProductBundleComponent struct {
	ID                 string  `json:"id"`
	SourceOutletID     string  `json:"source_outlet_id"`
	SourceOutletName   string  `json:"source_outlet_name,omitempty"`
	SourceProductID    string  `json:"source_product_id"`
	SourceProductName  string  `json:"source_product_name,omitempty"`
	SourceProductPrice float64 `json:"source_product_price,omitempty"`
	PartnerOutletID    string  `json:"partner_outlet_id"`
	PartnerOutletName  string  `json:"partner_outlet_name,omitempty"`
	PartnerProductID   string  `json:"partner_product_id"`
	PartnerProductName string  `json:"partner_product_name,omitempty"`
	PartnerProductPrice float64 `json:"partner_product_price,omitempty"`
	PartnerHasRecipe   bool    `json:"partner_has_recipe"`
	QtyPerBundle       float64 `json:"qty_per_bundle"`
	AllocationAmount   float64 `json:"allocation_amount"`
	ValidDays          int     `json:"valid_days"`
	Notes              string  `json:"notes"`
	IsActive           bool    `json:"is_active"`
	CreatedAt          string  `json:"created_at,omitempty"`
	UpdatedAt          string  `json:"updated_at,omitempty"`
}

type SaveBundleComponentItem struct {
	PartnerOutletID  string  `json:"partner_outlet_id"`
	PartnerProductID string  `json:"partner_product_id"`
	QtyPerBundle     float64 `json:"qty_per_bundle"`
	AllocationAmount float64 `json:"allocation_amount"`
	ValidDays        int     `json:"valid_days"`
	Notes            string  `json:"notes"`
}

type SaveProductBundleComponentsRequest struct {
	Components []SaveBundleComponentItem `json:"components"`
}

type BundlePartnerProduct struct {
	ID           string  `json:"id"`
	LocalID      string  `json:"local_id"`
	OutletID     string  `json:"outlet_id"`
	Name         string  `json:"name"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
	HasRecipe    bool    `json:"has_recipe"`
}

type CrossOutletBundleClaim struct {
	ID                       string  `json:"id"`
	ClaimCode                string  `json:"claim_code"`
	ComponentID              string  `json:"component_id"`
	SourceOutletID           string  `json:"source_outlet_id"`
	SourceOutletName         string  `json:"source_outlet_name"`
	SourceTransactionID      string  `json:"source_transaction_id"`
	SourceTransactionLocalID string  `json:"source_transaction_local_id"`
	SourceOrderID            string  `json:"source_order_id"`
	SourceProductID          string  `json:"source_product_id"`
	SourceProductName        string  `json:"source_product_name"`
	SourceQty                float64 `json:"source_qty"`
	PartnerOutletID          string  `json:"partner_outlet_id"`
	PartnerOutletName        string  `json:"partner_outlet_name"`
	PartnerProductID         string  `json:"partner_product_id"`
	PartnerProductLocalID    string  `json:"partner_product_local_id"`
	PartnerProductName       string  `json:"partner_product_name"`
	PartnerQty               float64 `json:"partner_qty"`
	ClaimedQty               float64 `json:"claimed_qty"`
	AllocationAmount         float64 `json:"allocation_amount"`
	Status                   string  `json:"status"` // pending | claimed | expired | voided
	CustomerName             string  `json:"customer_name"`
	CashierName              string  `json:"cashier_name"`
	SaleAt                   string  `json:"sale_at"`
	ExpiresAt                string  `json:"expires_at"`
	ClaimedAt                string  `json:"claimed_at"`
	ClaimedBy                string  `json:"claimed_by"`
	StockDeducted            bool    `json:"stock_deducted"`
	StockDeductionNote       string  `json:"stock_deduction_note"`
	VoidedAt                 string  `json:"voided_at"`
	VoidReason               string  `json:"void_reason"`
	Notes                    string  `json:"notes"`
	CanClaim                 bool    `json:"can_claim"`
}

type CrossOutletBundleSummary struct {
	TotalClaims       int     `json:"total_claims"`
	PendingCount      int     `json:"pending_count"`
	PendingAllocation float64 `json:"pending_allocation"`
	ClaimedCount      int     `json:"claimed_count"`
	ClaimedAllocation float64 `json:"claimed_allocation"`
	ExpiredCount      int     `json:"expired_count"`
	VoidedCount       int     `json:"voided_count"`
}

type CrossOutletBundleReconciliationRow struct {
	SourceOutletID    string  `json:"source_outlet_id"`
	SourceOutletName  string  `json:"source_outlet_name"`
	PartnerOutletID   string  `json:"partner_outlet_id"`
	PartnerOutletName string  `json:"partner_outlet_name"`
	SourceProductName string  `json:"source_product_name"`
	PartnerProductName string `json:"partner_product_name"`
	TotalBundlesSold  float64 `json:"total_bundles_sold"`
	TotalPartnerQty   float64 `json:"total_partner_qty"`
	ClaimedPartnerQty float64 `json:"claimed_partner_qty"`
	PendingPartnerQty float64 `json:"pending_partner_qty"`
	ExpiredPartnerQty float64 `json:"expired_partner_qty"`
	VoidedPartnerQty  float64 `json:"voided_partner_qty"`
	ClaimedAllocation float64 `json:"claimed_allocation"`
	PendingAllocation float64 `json:"pending_allocation"`
}

type CrossOutletBundleFilter struct {
	OutletID  string
	Direction string // "" | "inbound" | "outbound"
	Status    string
	DateFrom  string
	DateTo    string
	Search    string
}

type CrossOutletBundleListResponse struct {
	Claims         []CrossOutletBundleClaim             `json:"claims"`
	Summary        CrossOutletBundleSummary             `json:"summary"`
	Reconciliation []CrossOutletBundleReconciliationRow `json:"reconciliation"`
}

type ClaimCrossOutletBundleRequest struct {
	Notes string `json:"notes"`
}

type ManualCrossOutletBundleClaimRequest struct {
	ComponentID  string  `json:"component_id"`
	SourceQty    float64 `json:"source_qty"`
	CustomerName string  `json:"customer_name"`
	Notes        string  `json:"notes"`
}
