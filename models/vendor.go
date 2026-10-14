package models

import "time"

// Vendor represents a supplier/vendor for procurement.
type Vendor struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Phone         string    `json:"phone"`
	Email         string    `json:"email"`
	Address       string    `json:"address"`
	Notes         string    `json:"notes"`
	BankName      string    `json:"bank_name"`
	AccountNumber string    `json:"account_number"`
	AccountHolder string    `json:"account_holder"`
	IsActive      bool      `json:"is_active"`
	UnpaidAmount  float64   `json:"unpaid_amount"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// BankChanges: riwayat perubahan rekening terbaru (paling baru dulu) —
	// hanya diisi GetVendor, untuk peringatan di form bayar dan halaman vendor.
	BankChanges []VendorBankChange `json:"bank_changes,omitempty"`
}

// VendorBankChange: satu perubahan rekening vendor — siapa, kapan, lama → baru.
// Pembayaran lama tidak terpengaruh (rekeningnya dipotret per pembayaran).
type VendorBankChange struct {
	ID               string    `json:"id"`
	OldBankName      string    `json:"old_bank_name"`
	OldAccountNumber string    `json:"old_account_number"`
	OldAccountHolder string    `json:"old_account_holder"`
	NewBankName      string    `json:"new_bank_name"`
	NewAccountNumber string    `json:"new_account_number"`
	NewAccountHolder string    `json:"new_account_holder"`
	ChangedBy        string    `json:"changed_by"`
	ChangedAt        time.Time `json:"changed_at"`
}

// CreateVendorRequest is the payload for creating a vendor.
type CreateVendorRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
	Notes         string `json:"notes"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

// VendorMonthlySpend represents monthly spending data for a vendor.
type VendorMonthlySpend struct {
	Month       string  `json:"month"`
	Count       int     `json:"count"`
	TotalAmount float64 `json:"total_amount"`
}

// VendorDetailResponse wraps vendor info with aggregated stats.
type VendorDetailResponse struct {
	Vendor        Vendor               `json:"vendor"`
	TotalSpent    float64              `json:"total_spent"`
	TotalPaid     float64              `json:"total_paid"`
	TotalDebt     float64              `json:"total_debt"`
	TotalPending  float64              `json:"total_pending"`
	PurchaseCount int                  `json:"purchase_count"`
	MonthlySpend  []VendorMonthlySpend `json:"monthly_spend"`
}

// UpdateVendorRequest is the payload for updating a vendor.
type UpdateVendorRequest struct {
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
	Notes         string `json:"notes"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}
