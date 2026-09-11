package models

// PurchaseSubItem represents a single item inside a procurement entry.
type PurchaseSubItem struct {
	Name          string  `json:"name"`
	Qty           int     `json:"qty"`
	Unit          string  `json:"unit"`
	HpsPrice      float64 `json:"hps_price"`
	HpsSubtotal   float64 `json:"hps_subtotal"`
	FinalPrice    float64 `json:"final_price"`
	FinalSubtotal float64 `json:"final_subtotal"`
}

// PurchaseRequestItem represents a procurement entry ("Nama Pengadaan") with aggregated totals.
type PurchaseRequestItem struct {
	Name       string            `json:"name"`
	HpsTotal   float64           `json:"hps_total"`
	FinalTotal float64           `json:"final_total"`
	Items      []PurchaseSubItem `json:"items"`
}

// PurchaseRequest represents a purchase request from an outlet.
type PurchaseRequest struct {
	ID                   string                `json:"id"`
	RequestNumber        string                `json:"request_number"`
	OutletID             *string               `json:"outlet_id"`
	OutletName           *string               `json:"outlet_name,omitempty"`
	WorkUnitID           *string               `json:"work_unit_id"`
	WorkUnitName         string                `json:"work_unit_name,omitempty"`
	RequestType          string                `json:"request_type"`
	RequestedBy          string                `json:"requested_by"`
	VendorID             *string               `json:"vendor_id"`
	VendorName           string                `json:"vendor_name"`
	Status               string                `json:"status"`
	Items                []PurchaseRequestItem `json:"items"`
	TotalAmount          float64               `json:"total_amount"`
	TotalHps             float64               `json:"total_hps"`
	TotalFinal           float64               `json:"total_final"`
	Notes                string                `json:"notes"`
	InvoiceNumber        string                `json:"invoice_number"`
	ApprovedBy           *string               `json:"approved_by"`
	ApprovedAt           *string               `json:"approved_at"`
	RejectedReason       *string               `json:"rejected_reason"`
	PaidBy               *string               `json:"paid_by"`
	PaidAt               *string               `json:"paid_at"`
	PaymentProof         *string               `json:"payment_proof"`
	PaymentAccountDest   string                `json:"payment_account_dest"`
	PaymentAccountSource string                `json:"payment_account_source"`
	PaymentNotes         string                `json:"payment_notes"`
	PaidAmount           float64               `json:"paid_amount"`
	ReceivedBy           *string               `json:"received_by"`
	ReceivedAt           *string               `json:"received_at"`
	ProjectID            *string               `json:"project_id"`
	ProjectName          string                `json:"project_name,omitempty"`
	ParentID             *string               `json:"parent_id"`
	ParentNumber         string                `json:"parent_number,omitempty"`
	SplitStatus          *string               `json:"split_status"` // 'master' or nil
	Children             []PurchaseRequest     `json:"children,omitempty"`
	CreatedAt            string                `json:"created_at"`
	UpdatedAt            string                `json:"updated_at"`
}

// SplitPurchaseRequestInput is the payload for splitting a PR into a new vendor request.
type SplitPurchaseRequestInput struct {
	VendorID   string                `json:"vendor_id" validate:"required"`
	VendorName string                `json:"vendor_name" validate:"required"`
	Items      []PurchaseRequestItem `json:"items" validate:"required"`
}

// CreatePurchaseRequestInput is the payload for creating a purchase request.
type CreatePurchaseRequestInput struct {
	OutletID    string                `json:"outlet_id"`
	WorkUnitID  string                `json:"work_unit_id"`
	RequestType string                `json:"request_type" validate:"required"`
	RequestedBy string                `json:"requested_by" validate:"required"`
	VendorID    string                `json:"vendor_id"`
	VendorName  string                `json:"vendor_name"`
	// ProjectID opsional: mengikat pengajuan ini sebagai satu belanja tahap
	// dari sebuah projek pembangunan/renovasi.
	ProjectID string                `json:"project_id"`
	Items     []PurchaseRequestItem `json:"items" validate:"required"`
	Notes       string                `json:"notes"`
}

// UpdatePurchaseStatusInput is the payload for advancing purchase request status.
type UpdatePurchaseStatusInput struct {
	Action               string  `json:"action" validate:"required"` // approve, reject, request_payment, pay, receive, cancel
	ActorName            string  `json:"actor_name" validate:"required"`
	RejectedReason       string  `json:"rejected_reason,omitempty"`
	PaymentProof         string  `json:"payment_proof,omitempty"`
	PaymentAccountDest   string  `json:"payment_account_dest,omitempty"`
	PaymentAccountSource string  `json:"payment_account_source,omitempty"`
	PaymentNotes         string  `json:"payment_notes,omitempty"`
	PaymentAmount        float64 `json:"payment_amount,omitempty"`
}

// UpdatePurchaseItemsInput is the payload for updating items (e.g. setting final prices).
//
// Vendor dan nomor invoice sengaja berupa pointer: field yang tidak dikirim
// (nil) berarti "biarkan apa adanya", sedangkan string kosong berarti pengguna
// memang mengosongkannya. Sebelumnya bertipe string biasa, sehingga pengaju
// yang sekadar mengubah qty diam-diam menghapus vendor dan nomor invoice.
type UpdatePurchaseItemsInput struct {
	Items         []PurchaseRequestItem `json:"items" validate:"required"`
	VendorID      *string               `json:"vendor_id,omitempty"`
	VendorName    *string               `json:"vendor_name,omitempty"`
	InvoiceNumber *string               `json:"invoice_number,omitempty"`
}

// PurchaseRequestListResponse wraps paginated purchase requests.
type PurchaseRequestListResponse struct {
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
	Total      int               `json:"total"`
	TotalPages int               `json:"total_pages"`
	Requests   []PurchaseRequest `json:"requests"`
}

// PaymentHistory represents a single payment entry for a purchase request.
type PaymentHistory struct {
	ID                   string  `json:"id"`
	PurchaseRequestID    string  `json:"purchase_request_id"`
	Amount               float64 `json:"amount"`
	PaymentProof         string  `json:"payment_proof"`
	PaymentAccountDest   string  `json:"payment_account_dest"`
	PaymentAccountSource string  `json:"payment_account_source"`
	PaymentNotes         string  `json:"payment_notes"`
	PaidBy               string  `json:"paid_by"`
	CreatedAt            string  `json:"created_at"`
}
