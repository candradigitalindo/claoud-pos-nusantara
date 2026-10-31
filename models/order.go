package models

import (
	"encoding/json"
	"time"
)

type OrderItem struct {
	ID           string  `json:"id,omitempty"`
	ProductID    string  `json:"product_id,omitempty"`
	ProductName  string  `json:"product_name"`
	MenuKind     string  `json:"menu_kind,omitempty"` // POS item snapshot; keep on restore
	Category     string  `json:"category"`
	CategoryID   string  `json:"category_id,omitempty"`
	Qty          int     `json:"qty"`
	Price        float64 `json:"price"`
	Subtotal     float64 `json:"subtotal"`
	Destination  string  `json:"destination"`
	Status       string  `json:"status"`
	WaiterName   string  `json:"waiter_name,omitempty"` // nama pemesan item (sumber pengelompokan struk)
	Notes        string  `json:"notes,omitempty"`
	Addons       string  `json:"addons,omitempty"` // JSON string from POS, preserved as a snapshot
	IsAdditional int     `json:"is_additional,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
	// Diskon & komplimen (dikirim dari aplikasi POS Flutter, opsional):
	Discount        float64 `json:"discount,omitempty"`         // diskon nominal untuk baris ini
	IsComplimentary bool    `json:"is_complimentary,omitempty"` // true = item gratis/komplimen
}

type PaymentInfo struct {
	Method           string  `json:"method"`
	Amount           float64 `json:"amount"`
	PaidAmount       float64 `json:"paid_amount"`
	PaymentStatus    string  `json:"payment_status"`
	PaidAt           string  `json:"paid_at"`
	Discount         float64 `json:"discount,omitempty"`      // diskon nominal pada bill (total_amount sudah net)
	DiscountNote     string  `json:"discount_note,omitempty"` // alasan/label diskon (opsional)
	VoidedAt         string  `json:"voided_at,omitempty"`
	VoidedBy         string  `json:"voided_by,omitempty"`
	VoidReason       string  `json:"void_reason,omitempty"`
	ComplimentedAt   string  `json:"complimented_at,omitempty"`
	ComplimentBy     string  `json:"compliment_by,omitempty"`
	ComplimentReason string  `json:"compliment_reason,omitempty"`
	// Versioned POS recovery data. Keep unknown fields/versions intact so the
	// originating client can validate payments, charges and local metadata.
	Restore json.RawMessage `json:"restore,omitempty"`
}

// ── Laporan Diskon & Komplimen ──────────────────────────────
type DiscountReportRow struct {
	ID           string  `json:"id"`
	OutletName   string  `json:"outlet_name"`
	CustomerName string  `json:"customer_name"`
	Net          float64 `json:"net"`        // total_amount (dibayar, sudah net diskon)
	Discount     float64 `json:"discount"`   // total diskon (bill + per item)
	Compliment   float64 `json:"compliment"` // nilai item komplimen
	Gross        float64 `json:"gross"`      // net + diskon + komplimen
	CreatedAt    string  `json:"created_at"`
}

type DiscountReportSummary struct {
	TotalOrders int     `json:"total_orders"`
	Net         float64 `json:"net"`
	Discount    float64 `json:"discount"`
	Compliment  float64 `json:"compliment"`
	Gross       float64 `json:"gross"`
}

type DiscountReport struct {
	Summary DiscountReportSummary `json:"summary"`
	Data    []DiscountReportRow   `json:"data"`
	Total   int                   `json:"total"`
	Page    int                   `json:"page"`
	Limit   int                   `json:"limit"`
}

type PushOrderRequest struct {
	LocalID       string      `json:"local_id"`
	OutletID      string      `json:"outlet_id"`
	OutletCode    string      `json:"outlet_code"`
	TableNumber   string      `json:"table_number"`
	CustomerName  string      `json:"customer_name"`
	CustomerPhone string      `json:"customer_phone"` // no. HP pelanggan (opsional)
	OrdererName   string      `json:"orderer_name"`   // label "Pemesan" gabungan (seperti struk)
	CreatedBy     string      `json:"created_by"`     // pembuat order (1 orang)
	Pax           int         `json:"pax"`
	TotalAmount   float64     `json:"total_amount"`
	Status        string      `json:"status"`
	Items         []OrderItem `json:"items"`
	PaymentInfo   PaymentInfo `json:"payment_info"`
	Version       int         `json:"version"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	IsHolding     bool        `json:"is_holding"` // true = order Meja Titipan (parked check), bukan tamu aktif
}

// PushOrderItemTitipanRequest = payload entityType "order_item_titipan" dari app POS:
// audit event titip (park) / tarik (pull) satu item Meja Titipan. Selalu operation
// "create" (event log append-only), bukan mutasi state.
type PushOrderItemTitipanRequest struct {
	LocalID     string  `json:"local_id"` // order_items.id
	Action      string  `json:"action"`   // "park" | "pull"
	ProductName string  `json:"product_name"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
	CategoryID  string  `json:"category_id"`
	SourceTable string  `json:"source_table"`
	TargetTable string  `json:"target_table"`
	By          string  `json:"by"` // authorizer (park) / kasir (pull)
	At          string  `json:"at"` // ISO-8601 UTC
}

// ── Laporan Titipan (Meja Titipan) ──────────────────────────
type TitipanLogRow struct {
	ID          string  `json:"id"`
	OutletName  string  `json:"outlet_name"`
	LocalID     string  `json:"local_id"`
	Action      string  `json:"action"` // park | pull
	ProductName string  `json:"product_name"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
	SourceTable string  `json:"source_table"`
	TargetTable string  `json:"target_table"`
	PerformedBy string  `json:"performed_by"`
	PerformedAt string  `json:"performed_at"`
}

type TitipanReportSummary struct {
	ParkCount    int     `json:"park_count"`    // event titip
	ParkAmount   float64 `json:"park_amount"`   // nilai barang dititip
	PullCount    int     `json:"pull_count"`    // event tarik
	PullAmount   float64 `json:"pull_amount"`   // nilai barang ditarik
	HoldingCount int     `json:"holding_count"` // order titipan aktif (is_holding, belum bayar)
	HoldingValue float64 `json:"holding_value"` // nilai barang yang masih menggantung
}

type TitipanReport struct {
	Summary TitipanReportSummary `json:"summary"`
	Data    []TitipanLogRow      `json:"data"`
	Total   int                  `json:"total"`
	Page    int                  `json:"page"`
	Limit   int                  `json:"limit"`
}

// PushOrderItemVoidRequest = payload entityType "order_item_void" dari app POS:
// audit penghapusan satu item dari order yang belum dibayar (di-gate PIN Manager/SVP).
type PushOrderItemVoidRequest struct {
	ProductID   string  `json:"product_id,omitempty"`
	LocalID     string  `json:"local_id"` // id item yang dihapus
	OrderID     string  `json:"order_id"`
	TableNumber string  `json:"table_number"`
	ProductName string  `json:"product_name"`
	CategoryID  string  `json:"category_id"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
	WaiterName  string  `json:"waiter_name"`
	VoidedBy    string  `json:"voided_by"`
	VoidReason  string  `json:"void_reason"`
	VoidedAt    string  `json:"voided_at"`
}

type VoidItemRow struct {
	ID          string  `json:"id"`
	OutletName  string  `json:"outlet_name"`
	OrderID     string  `json:"order_id"`
	TableNumber string  `json:"table_number"`
	ProductName string  `json:"product_name"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
	WaiterName  string  `json:"waiter_name"`
	VoidedBy    string  `json:"voided_by"`
	VoidReason  string  `json:"void_reason"`
	VoidedAt    string  `json:"voided_at"`
}

type VoidOrderRow struct {
	ID           string  `json:"id"`
	OutletName   string  `json:"outlet_name"`
	TableNumber  string  `json:"table_number"`
	CustomerName string  `json:"customer_name"`
	TotalAmount  float64 `json:"total_amount"`
	Items        string  `json:"items"`
	VoidedAt     string  `json:"voided_at"`
	VoidedBy     string  `json:"voided_by"`
	VoidReason   string  `json:"void_reason"`
	CreatedAt    string  `json:"created_at"`
}

type VoidReportSummary struct {
	TotalVoided int     `json:"total_voided"` // jumlah void transaksi (order penuh)
	TotalAmount float64 `json:"total_amount"`
	ItemVoided  int     `json:"item_voided"` // jumlah void item (hapus item, order jalan terus)
	ItemAmount  float64 `json:"item_amount"`
}

type VoidReport struct {
	Summary    VoidReportSummary `json:"summary"`
	Data       []VoidOrderRow    `json:"data"`
	Items      []VoidItemRow     `json:"items"`
	Total      int               `json:"total"`
	ItemsTotal int               `json:"items_total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
}

type CloudOrder struct {
	ID            string    `json:"id"`
	LocalID       string    `json:"local_id"`
	OutletID      string    `json:"outlet_id"`
	OutletCode    string    `json:"outlet_code"`
	TableNumber   string    `json:"table_number"`
	CustomerName  string    `json:"customer_name"`
	CustomerPhone string    `json:"customer_phone"`
	CreatedBy     string    `json:"created_by"`
	OrdererName   string    `json:"orderer_name"`
	IsHolding     bool      `json:"is_holding"`
	Pax           int       `json:"pax"`
	TotalAmount   float64   `json:"total_amount"`
	Status        string    `json:"status"`
	Items         string    `json:"items"`
	PaymentInfo   string    `json:"payment_info"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	SyncedAt      time.Time `json:"synced_at"`
}
