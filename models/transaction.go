package models

import "time"

type TransactionItem struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
	WaiterName  string  `json:"waiter_name,omitempty"` // nama pemesan item (sumber pengelompokan struk)
}

type TransactionCharge struct {
	Name       string  `json:"name"`
	ChargeType string  `json:"charge_type"` // percentage | fixed
	Value      float64 `json:"value"`
	Amount     float64 `json:"amount"`
}

// PaymentLine adalah satu pembayaran dalam transaksi multi-metode
// (fitur Gabung Bayar / Split Bill di app POS).
type PaymentLine struct {
	PaymentMethod string  `json:"payment_method"` // cash | qris | card | transfer
	Amount        float64 `json:"amount"`
	PaymentNote   *string `json:"payment_note"`
	CreatedAt     string  `json:"created_at"`
}

type PushTransactionRequest struct {
	LocalID           string              `json:"local_id"`
	OutletID          string              `json:"outlet_id"`
	OutletCode        string              `json:"outlet_code"`
	OrderID           string              `json:"order_id"`
	Subtotal          float64             `json:"subtotal"`            // penjualan bersih (DPP)
	TaxAmount         float64             `json:"tax_amount"`          // pajak (charge persentase)
	OtherChargesTotal float64             `json:"other_charges_total"` // tambahan fixed
	Charges           []TransactionCharge `json:"charges"`             // rincian tiap tambahan
	TotalAmount       float64             `json:"total_amount"`        // grand total
	PaymentMethod     string              `json:"payment_method"`      // bisa "mixed" bila >1 metode
	CashAmount        float64             `json:"cash_amount"`         // total tunai sebenarnya
	ChangeAmount      float64             `json:"change_amount"`
	Payments          []PaymentLine       `json:"payments"`      // rincian per metode (Gabung Bayar / Split Bill)
	CashierName       string              `json:"cashier_name"`  // nama kasir pemroses bayar
	CreatedBy         string              `json:"created_by"`    // = cashier_name (fallback bila cashier_name kosong)
	OrdererName       string              `json:"orderer_name"`  // label "Pemesan" gabungan (seperti struk)
	// ReservationID opsional: transaksi ini menutup sebuah reservasi. Uang
	// mukanya dikirim sebagai baris payments[] bermetode 'reservasi_dp'.
	ReservationID     string              `json:"reservation_id"`
	Items             []TransactionItem   `json:"items"`
	Version           int                 `json:"version"`
	CreatedAt         string              `json:"created_at"`
	TransactionDate   string              `json:"transaction_date"` // waktu transaksi ASLI (utamakan ini; sync_timestamp hanya metadata)
}

type CloudTransaction struct {
	ID            string    `json:"id"`
	LocalID       string    `json:"local_id"`
	OutletID      string    `json:"outlet_id"`
	OutletCode    string    `json:"outlet_code"`
	OrderID           string    `json:"order_id"`
	Subtotal          float64   `json:"subtotal"`
	TotalAmount       float64   `json:"total_amount"`
	TaxAmount         float64   `json:"tax_amount"`
	OtherChargesTotal float64   `json:"other_charges_total"`
	Charges           string    `json:"charges"`
	PaymentMethod     string    `json:"payment_method"`
	CashAmount        float64   `json:"cash_amount"`
	ChangeAmount      float64   `json:"change_amount"`
	CashierName       string    `json:"cashier_name"`
	Items             string    `json:"items"`
	Version           int       `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	SyncedAt          time.Time `json:"synced_at"`
}
