package models

// Penerimaan Barang (Goods Receipt / GRN) — dokumen stok masuk ke gudang.

// GoodsReceiptItemReq — satu baris pada form penerimaan.
type GoodsReceiptItemReq struct {
	ItemID      string  `json:"item_id"`
	QtyDist     float64 `json:"qty_dist"`      // jumlah dalam satuan beli/distribusi
	CostPerBase float64 `json:"cost_per_base"` // harga per satuan dasar
	ExpiryDate  string  `json:"expiry_date"`   // opsional, YYYY-MM-DD
}

// GoodsReceiptRequest — payload buat GRN.
type GoodsReceiptRequest struct {
	WarehouseID       string                `json:"warehouse_id"`
	VendorName        string                `json:"vendor_name"`
	PORef             string                `json:"po_ref"`
	PurchaseRequestID string                `json:"purchase_request_id"`
	Notes             string                `json:"notes"`
	Items             []GoodsReceiptItemReq `json:"items"`
}

// GoodsReceiptItem — baris tersimpan.
type GoodsReceiptItem struct {
	ID          string  `json:"id"`
	ItemID      string  `json:"item_id"`
	ItemName    string  `json:"item_name"`
	ItemCode    string  `json:"item_code,omitempty"`
	QtyBase     float64 `json:"qty_base"`
	QtyDist     float64 `json:"qty_dist"`
	UnitUsed    string  `json:"unit_used"`
	BaseUnit    string  `json:"base_unit,omitempty"`
	CostPerBase float64 `json:"cost_per_base"`
	Subtotal    float64 `json:"subtotal"`
	ExpiryDate  string  `json:"expiry_date,omitempty"`
}

// GoodsReceipt — dokumen penerimaan.
type GoodsReceipt struct {
	ID            string `json:"id"`
	GRNNumber     string `json:"grn_number"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	VendorName    string `json:"vendor_name"`
	PORef         string `json:"po_ref"`
	// Tautan ke pengajuan pengadaan. Kolomnya sudah lama ada dan ikut tersimpan,
	// tapi tidak pernah dibaca balik sehingga dokumen GRN terputus dari alur
	// pengadaan yang memicunya.
	PurchaseRequestID     string             `json:"purchase_request_id,omitempty"`
	PurchaseRequestNumber string             `json:"purchase_request_number,omitempty"`
	Notes                 string             `json:"notes"`
	TotalCost             float64            `json:"total_cost"`
	ItemCount             int                `json:"item_count"`
	ReceivedBy            string             `json:"received_by"`
	ReceivedAt            string             `json:"received_at"`
	Items                 []GoodsReceiptItem `json:"items,omitempty"`
}
