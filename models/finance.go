package models

import "time"

type FinanceAccount struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Group        string `json:"group"`
	Direction    string `json:"direction,omitempty"`
	CashCategory string `json:"cash_category,omitempty"`
}

type FinanceAccountTotal struct {
	FinanceAccount
	Amount float64 `json:"amount"`
}

type FinanceEntryInput struct {
	RequestKey    string  `json:"request_key"`
	Direction     string  `json:"direction"`
	AccountCode   string  `json:"account_code"`
	OutletID      string  `json:"outlet_id"`
	WorkUnitID    string  `json:"work_unit_id"`
	Amount        float64 `json:"amount"`
	OccurredAt    string  `json:"occurred_at"`
	Period        string  `json:"period"`
	Counterparty  string  `json:"counterparty"`
	Description   string  `json:"description"`
	Notes         string  `json:"notes"`
	Reference     string  `json:"reference"`
	Method        string  `json:"method"`
	BankAccountID string  `json:"bank_account_id"`
	ProofURL      string  `json:"proof_url"`
	Version       int     `json:"version"`
}

type FinanceEntry struct {
	FinanceEntryInput
	ID           string              `json:"id"`
	Number       string              `json:"number"`
	Status       string              `json:"status"`
	OutletName   string              `json:"outlet_name"`
	WorkUnitName string              `json:"work_unit_name"`
	BankLabel    string              `json:"bank_label"`
	CreatedBy    string              `json:"created_by"`
	CreatedAt    time.Time           `json:"created_at"`
	PostedBy     string              `json:"posted_by"`
	PostedAt     *time.Time          `json:"posted_at"`
	VoidedBy     string              `json:"voided_by"`
	VoidedAt     *time.Time          `json:"voided_at"`
	VoidReason   string              `json:"void_reason"`
	History      []FinanceEntryEvent `json:"history"`
}

type FinanceEntryEvent struct {
	Action string    `json:"action"`
	Actor  string    `json:"actor"`
	Notes  string    `json:"notes"`
	At     time.Time `json:"at"`
}

type FinancePosting struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
}

// One source event, with balanced postings. Reports and the operations screen
// consume the same events so an automatic transaction is never entered twice.
type FinanceOperation struct {
	ID           string           `json:"id"`
	Source       string           `json:"source"`
	SourceID     string           `json:"source_id"`
	Reference    string           `json:"reference"`
	At           time.Time        `json:"at"`
	Date         string           `json:"date"`
	OutletID     string           `json:"outlet_id"`
	OutletName   string           `json:"outlet_name"`
	WorkUnitID   string           `json:"work_unit_id"`
	WorkUnitName string           `json:"work_unit_name"`
	Description  string           `json:"description"`
	Notes        string           `json:"notes"`
	Actor        string           `json:"actor"`
	Counterparty string           `json:"counterparty"`
	ProofURL     string           `json:"proof_url"`
	Method       string           `json:"method"`
	Period       string           `json:"period"`
	Status       string           `json:"status"`
	ReviewNote   string           `json:"review_note"`
	CashCategory string           `json:"cash_category"`
	CashIn       float64          `json:"cash_in"`
	CashOut      float64          `json:"cash_out"`
	Postings     []FinancePosting `json:"postings"`
}

type FinanceOperationsResponse struct {
	Rows        []FinanceOperation `json:"rows"`
	Total       int                `json:"total"`
	CashIn      float64            `json:"cash_in"`
	CashOut     float64            `json:"cash_out"`
	ReviewCount int                `json:"review_count"`
}
