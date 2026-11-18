package models

import "time"

type PushAnalyticsRequest struct {
	OutletID   string      `json:"outlet_id"`
	OutletCode string      `json:"outlet_code"`
	Date       string      `json:"date"`
	Summary    interface{} `json:"summary"`
}

type CloudAnalytics struct {
	ID         string    `json:"id"`
	OutletID   string    `json:"outlet_id"`
	OutletCode string    `json:"outlet_code"`
	Date       string    `json:"date"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PushCashierShiftRequest struct {
	LocalID         string  `json:"local_id"`
	OpenedBy        string  `json:"opened_by"`
	OpenedAt        string  `json:"opened_at"`
	OpeningCash     float64 `json:"opening_cash"`
	ClosedAt        string  `json:"closed_at"`
	ClosedBy        string  `json:"closed_by"`
	ClosingCash     float64 `json:"closing_cash"`
	ClosingCard     float64 `json:"closing_card"`
	ClosingQris     float64 `json:"closing_qris"`
	ClosingTransfer float64 `json:"closing_transfer"`
	CarryOverCash   float64 `json:"carry_over_cash"`
	PreviousShiftID string      `json:"previous_shift_id"`
	HandoverTo      string      `json:"handover_to"`
	Status          string      `json:"status"`
	Notes           string      `json:"notes"`
	Report          interface{} `json:"report"` // jumlah transaksi per metode + kas masuk/keluar
}

type CloudCashierShift struct {
	ID              string     `json:"id"`
	LocalID         string     `json:"local_id"`
	OutletID        string     `json:"outlet_id"`
	OpenedBy        string     `json:"opened_by"`
	OpenedAt        time.Time  `json:"opened_at"`
	OpeningCash     float64    `json:"opening_cash"`
	ClosedAt        *time.Time `json:"closed_at"`
	ClosedBy        string     `json:"closed_by"`
	ClosingCash     float64    `json:"closing_cash"`
	ClosingCard     float64    `json:"closing_card"`
	ClosingQris     float64    `json:"closing_qris"`
	ClosingTransfer float64    `json:"closing_transfer"`
	CarryOverCash   float64    `json:"carry_over_cash"`
	PreviousShiftID string     `json:"previous_shift_id"`
	HandoverTo      string     `json:"handover_to"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	SyncedAt        time.Time  `json:"synced_at"`
}

type PushPrinterRequest struct {
	LocalID     string `json:"local_id"`
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address"`
	Port        int    `json:"port"`
	PrinterType string `json:"printer_type"`
	PaperSize   string `json:"paper_size"`
	IsActive    bool   `json:"is_active"`
	IsDeleted   bool   `json:"is_deleted"`
}

type CloudPrinter struct {
	ID          string    `json:"id"`
	LocalID     string    `json:"local_id"`
	OutletID    string    `json:"outlet_id"`
	Name        string    `json:"name"`
	IPAddress   string    `json:"ip_address"`
	Port        int       `json:"port"`
	PrinterType string    `json:"printer_type"`
	PaperSize   string    `json:"paper_size"`
	IsActive    bool      `json:"is_active"`
	IsDeleted   bool      `json:"is_deleted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SyncedAt    time.Time `json:"synced_at"`
}

type PushCashMovementRequest struct {
	LocalID         string  `json:"local_id"`
	ShiftID         string  `json:"shift_id"`
	MovementType    string  `json:"movement_type"`
	Amount          float64 `json:"amount"`
	CounterpartName string  `json:"counterpart_name"`
	Note            string  `json:"note"`
	CreatedAt       string  `json:"created_at"`
	// CashPayoutID: kas keluar ini adalah penyerahan kas untuk tugas
	// "Bayar Pengadaan" (Kas Kasir Outlet). Kosong untuk kas keluar biasa.
	CashPayoutID string `json:"cash_payout_id,omitempty"`
}

type CloudCashMovement struct {
	ID              string    `json:"id"`
	LocalID         string    `json:"local_id"`
	OutletID        string    `json:"outlet_id"`
	ShiftID         string    `json:"shift_id"`
	MovementType    string    `json:"movement_type"`
	Amount          float64   `json:"amount"`
	CounterpartName string    `json:"counterpart_name"`
	Note            string    `json:"note"`
	CreatedAt       time.Time `json:"created_at"`
	SyncedAt        time.Time `json:"synced_at"`
}
