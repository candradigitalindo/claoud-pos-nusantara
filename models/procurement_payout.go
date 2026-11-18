package models

// ProcurementCashPayout — tugas "Bayar Pengadaan dari Kas Kasir Outlet".
//
// Keuangan memilih sumber dana Kas Kasir Outlet → tugas 'pending' muncul di
// tablet kasir outlet itu (diambil langsung dari cloud, tidak lewat sinkron
// berkala) → kasir mengklaim saat online ('claimed', terkunci dari penarikan)
// → kas keluar dicatat di shift tablet → 'paid' (pembayaran pengajuan tercatat
// sekali, tertaut ke kas keluar itu). 'returned' = dikembalikan kasir dengan
// alasan; 'cancelled' = ditarik keuangan sebelum diproses.
type ProcurementCashPayout struct {
	ID                string  `json:"id"`
	PurchaseRequestID string  `json:"purchase_request_id"`
	RequestNumber     string  `json:"request_number"`
	RequestType       string  `json:"request_type"`
	ItemsSummary      string  `json:"items_summary"`
	VendorName        string  `json:"vendor_name"`
	OutletID          string  `json:"outlet_id"`
	OutletName        string  `json:"outlet_name"`
	Amount            float64 `json:"amount"`
	RecipientName     string  `json:"recipient_name"`
	Notes             string  `json:"notes"`
	Status            string  `json:"status"`
	AssignedBy        string  `json:"assigned_by"`
	AssignedAt        string  `json:"assigned_at"`
	MovementLocalID   string  `json:"movement_local_id,omitempty"`
	CashierName       string  `json:"cashier_name,omitempty"`
	AuthorizedBy      string  `json:"authorized_by,omitempty"`
	ClaimedAt         string  `json:"claimed_at,omitempty"`
	PaidAt            string  `json:"paid_at,omitempty"`
	ResolvedBy        string  `json:"resolved_by,omitempty"`
	ResolvedAt        string  `json:"resolved_at,omitempty"`
	ResolutionReason  string  `json:"resolution_reason,omitempty"`
}

// AssignCashPayoutInput — keuangan menetapkan sumber dana Kas Kasir Outlet.
type AssignCashPayoutInput struct {
	PurchaseRequestID string  `json:"purchase_request_id"`
	Amount            float64 `json:"amount"` // 0 = sisa tagihan
	RecipientName     string  `json:"recipient_name"`
	Notes             string  `json:"notes"`
	ActorName         string  `json:"-"`
	ActorID           string  `json:"-"`
}

// CashPayoutDeviceInput — aksi tablet kasir (klaim / selesai / lepas).
type CashPayoutDeviceInput struct {
	MovementLocalID string `json:"movement_local_id"`
	CashierName     string `json:"cashier_name"`
	AuthorizedBy    string `json:"authorized_by"`
	Reason          string `json:"reason"`
}

// CashPayoutResolveInput — keuangan menarik tugas yang belum diproses.
type CashPayoutResolveInput struct {
	Reason    string `json:"reason"`
	ActorName string `json:"-"`
}
