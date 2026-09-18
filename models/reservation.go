package models

import "time"

type ReservationItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Qty         int     `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
}

// Reservation — pesanan di muka dari pelanggan.
//
// Uang muka (DP) diperlakukan sebagai KEWAJIBAN kepada pelanggan, bukan
// pendapatan: pendapatan tetap diakui saat transaksi POS terjadi pada hari
// kunjungan (kebijakan pengakuan penjualan yang sudah berlaku). Karena itu
// DownPayment di sini adalah DP yang DIMINTA; uang yang benar-benar masuk
// hidup di reservation_payments dan harus tervalidasi lebih dulu.
type Reservation struct {
	ID              string            `json:"id"`
	OutletID        string            `json:"outlet_id"`
	OutletName      string            `json:"outlet_name"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	Pax             int               `json:"pax"`
	Items           []ReservationItem `json:"items"`
	Subtotal        float64           `json:"subtotal"`
	// DownPayment = DP yang diminta. PaidAmount = pembayaran TERVALIDASI
	// (dp + pelunasan − refund). PendingAmount = bukti yang masih menunggu
	// validasi. Remaining = Total − PaidAmount.
	DownPayment     float64           `json:"down_payment"`
	PaidAmount      float64           `json:"paid_amount"`
	PendingAmount   float64           `json:"pending_amount"`
	DpPaid          bool              `json:"dp_paid"`
	Total           float64           `json:"total"`
	Remaining       float64           `json:"remaining"`
	ReservationDate string            `json:"reservation_date"` // YYYY-MM-DD
	ReservationTime string            `json:"reservation_time"` // HH:MM
	// Status: pending (menunggu DP) → confirmed (DP tervalidasi) → done
	// (dilayani, ditutup POS/admin); cancelled dengan CancelDisposition
	// 'refund' (uang muka dikembalikan) atau 'hangus' (menjadi pendapatan lain).
	Status            string `json:"status"`
	CancelDisposition string `json:"cancel_disposition"`
	ConfirmedAt       string `json:"confirmed_at"`
	CancelledAt       string `json:"cancelled_at"`
	SettledAt         string `json:"settled_at"`
	PosTransactionID  string `json:"pos_transaction_id"`
	Notes           string            `json:"notes"`
	Source          string            `json:"source"` // admin | public
	Payments        []ReservationPayment `json:"payments,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ReservationPayment — satu bukti uang masuk/keluar atas sebuah reservasi.
// Dari pelanggan (halaman publik) lahir berstatus pending; admin yang
// memvalidasi. Dicatat admin sendiri langsung validated.
type ReservationPayment struct {
	ID             string  `json:"id"`
	ReservationID  string  `json:"reservation_id"`
	Type           string  `json:"type"`   // dp | pelunasan | refund
	Amount         float64 `json:"amount"`
	Method         string  `json:"method"` // transfer | cash | qris | lainnya
	BankAccountID  string  `json:"bank_account_id"`
	BankLabel      string  `json:"bank_label"`
	ProofURL       string  `json:"proof_url"`
	PaidAt         string  `json:"paid_at"` // YYYY-MM-DD HH:MM, zona aplikasi
	Status         string  `json:"status"`  // pending | validated | rejected
	SubmittedBy    string  `json:"submitted_by"`
	ValidatedBy    string  `json:"validated_by"`
	ValidatedAt    string  `json:"validated_at"`
	RejectedReason string  `json:"rejected_reason"`
	Notes          string  `json:"notes"`
	CreatedAt      string  `json:"created_at"`
}

type ReservationPaymentRequest struct {
	Type          string  `json:"type"`
	Amount        float64 `json:"amount"`
	Method        string  `json:"method"`
	BankAccountID string  `json:"bank_account_id"`
	ProofURL      string  `json:"proof_url"`
	PaidAt        string  `json:"paid_at"` // YYYY-MM-DD atau YYYY-MM-DD HH:MM (zona aplikasi); kosong = sekarang
	Notes         string  `json:"notes"`
}

type ReservationStatusRequest struct {
	Status      string `json:"status"`
	Disposition string `json:"disposition"` // refund | hangus — wajib saat membatalkan reservasi yang sudah ada uang masuk
}

// ReservationSettings — kebijakan DP + rekening tujuan untuk form pembayaran.
type ReservationSettings struct {
	DpPercent    float64       `json:"dp_percent"`
	BankAccounts []BankAccount `json:"bank_accounts"`
}

// PublicReservationStatus — tampilan status untuk pelanggan (tanpa auth):
// cukup untuk tahu berapa yang harus dibayar, ke mana, dan sudah sampai mana.
type PublicReservationStatus struct {
	ID              string               `json:"id"`
	OutletName      string               `json:"outlet_name"`
	CustomerName    string               `json:"customer_name"`
	ReservationDate string               `json:"reservation_date"`
	ReservationTime string               `json:"reservation_time"`
	Pax             int                  `json:"pax"`
	Items           []ReservationItem    `json:"items"`
	Total           float64              `json:"total"`
	DownPayment     float64              `json:"down_payment"`
	PaidAmount      float64              `json:"paid_amount"`
	PendingAmount   float64              `json:"pending_amount"`
	Remaining       float64              `json:"remaining"`
	Status          string               `json:"status"`
	DpPaid          bool                 `json:"dp_paid"`
	Payments        []ReservationPayment `json:"payments"`
	BankAccounts    []BankAccount        `json:"bank_accounts"`
}

// OutletReservation — bentuk ringkas untuk app POS: cukup untuk membuka
// order dari reservasi dan memakai uang mukanya sebagai baris pembayaran
// bermetode 'reservasi_dp'.
type OutletReservation struct {
	ID              string            `json:"id"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	Pax             int               `json:"pax"`
	ReservationDate string            `json:"reservation_date"`
	ReservationTime string            `json:"reservation_time"`
	Items           []ReservationItem `json:"items"`
	Total           float64           `json:"total"`
	PaidAmount      float64           `json:"paid_amount"`
	Remaining       float64           `json:"remaining"`
	Status          string            `json:"status"`
	Notes           string            `json:"notes"`
}

type ReservationRequest struct {
	OutletID        string            `json:"outlet_id"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	Pax             int               `json:"pax"`
	Items           []ReservationItem `json:"items"`
	DownPayment     float64           `json:"down_payment"`
	ReservationDate string            `json:"reservation_date"`
	ReservationTime string            `json:"reservation_time"`
	Status          string            `json:"status"`
	Notes           string            `json:"notes"`
}

// ── Public reservation page payloads ────────────────────────

type PublicProduct struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	PhotoURL string  `json:"photo_url"`
}

type PublicCategory struct {
	Name     string          `json:"name"`
	Products []PublicProduct `json:"products"`
}

type PublicMenu struct {
	OutletID   string           `json:"outlet_id"`
	OutletName string           `json:"outlet_name"`
	Slug       string           `json:"slug"`
	Categories []PublicCategory `json:"categories"`
}
