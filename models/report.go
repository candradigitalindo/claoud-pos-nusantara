package models

type SalesReportRow struct {
	Date              string  `json:"date"`
	TotalTransactions int     `json:"total_transactions"`
	TotalPax          int     `json:"total_pax"`
	TotalRevenue      float64 `json:"total_revenue"`
	CashRevenue       float64 `json:"cash_revenue"`
	QrisRevenue       float64 `json:"qris_revenue"`
	CardRevenue       float64 `json:"card_revenue"`
	TransferRevenue   float64 `json:"transfer_revenue"`
}

type SalesReportOutlet struct {
	OutletID          string  `json:"outlet_id"`
	OutletName        string  `json:"outlet_name"`
	TotalTransactions int     `json:"total_transactions"`
	TotalRevenue      float64 `json:"total_revenue"`
	UnpaidOrders      int     `json:"unpaid_orders"`
	UnpaidAmount      float64 `json:"unpaid_amount"`
}

type SalesReportSummary struct {
	TotalTransactions int     `json:"total_transactions"`
	TotalRevenue      float64 `json:"total_revenue"`
	AvgPerTransaction float64 `json:"avg_per_transaction"`
	CashRevenue       float64 `json:"cash_revenue"`
	QrisRevenue       float64 `json:"qris_revenue"`
	CardRevenue       float64 `json:"card_revenue"`
	TransferRevenue   float64 `json:"transfer_revenue"`
	UnpaidOrders      int     `json:"unpaid_orders"`
	UnpaidAmount      float64 `json:"unpaid_amount"`
}

type SalesReportResponse struct {
	Summary      SalesReportSummary       `json:"summary"`
	Daily        []SalesReportRow         `json:"daily"`
	ByOutlet     []SalesReportOutlet      `json:"by_outlet"`
	Transactions []SalesReportTransaction `json:"transactions"`
	Page         int                      `json:"page"`
	Limit        int                      `json:"limit"`
	Total        int                      `json:"total"`
	TotalPages   int                      `json:"total_pages"`
}

type SalesReportTransaction struct {
	ID            string  `json:"id"`
	OutletName    string  `json:"outlet_name"`
	OutletCode    string  `json:"outlet_code"`
	TotalAmount   float64 `json:"total_amount"`
	PaymentMethod string  `json:"payment_method"`
	CashierName   string  `json:"cashier_name"`
	OrdererName   string  `json:"orderer_name"`
	Pax           int     `json:"pax"`
	Items         string  `json:"items"`
	CreatedAt     string  `json:"created_at"`
}

type UnpaidOrderRow struct {
	ID           string  `json:"id"`
	OutletName   string  `json:"outlet_name"`
	OutletCode   string  `json:"outlet_code"`
	TableNumber  string  `json:"table_number"`
	CustomerName string  `json:"customer_name"`
	Pax          int     `json:"pax"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
	Items        string  `json:"items"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type UnpaidOrdersResponse struct {
	TotalUnpaid int              `json:"total_unpaid"`
	TotalAmount float64          `json:"total_amount"`
	Orders      []UnpaidOrderRow `json:"orders"`
	Page        int              `json:"page"`
	Limit       int              `json:"limit"`
	Total       int              `json:"total"`
	TotalPages  int              `json:"total_pages"`
}

type ProductSalesRow struct {
	OutletName   string  `json:"outlet_name"`
	ProductName  string  `json:"product_name"`
	CategoryName string  `json:"category_name"`
	TotalQty     int     `json:"total_qty"`
	TotalRevenue float64 `json:"total_revenue"`
}

type ProductSalesResponse struct {
	DateFrom     string            `json:"date_from"`
	DateTo       string            `json:"date_to"`
	Total        int               `json:"total"` // jumlah baris (produk × outlet) keseluruhan
	Page         int               `json:"page"`
	Limit        int               `json:"limit"`
	TotalQty     int               `json:"total_qty"`     // grand total qty semua halaman
	TotalRevenue float64           `json:"total_revenue"` // grand total pendapatan semua halaman
	Items        []ProductSalesRow `json:"items"`
}

type TaxReportSummary struct {
	TotalTransactions int     `json:"total_transactions"`
	GrossRevenue      float64 `json:"gross_revenue"`
	TaxAmount         float64 `json:"tax_amount"`
	NetRevenue        float64 `json:"net_revenue"`
	TaxRate           float64 `json:"tax_rate"`
}

type TaxReportRow struct {
	Date              string  `json:"date"`
	TotalTransactions int     `json:"total_transactions"`
	GrossRevenue      float64 `json:"gross_revenue"`
	TaxAmount         float64 `json:"tax_amount"`
	NetRevenue        float64 `json:"net_revenue"`
}

type TaxOutletRow struct {
	OutletID     string  `json:"outlet_id"`
	OutletName   string  `json:"outlet_name"`
	GrossRevenue float64 `json:"gross_revenue"`
	TaxAmount    float64 `json:"tax_amount"`
	NetRevenue   float64 `json:"net_revenue"`
	TaxRate      float64 `json:"tax_rate"`    // tarif terkonfigurasi outlet ini (%)
	TaxEnabled   bool    `json:"tax_enabled"` // status pajak outlet ini
}

type TaxReportResponse struct {
	Summary  TaxReportSummary `json:"summary"`
	Daily    []TaxReportRow   `json:"daily"`
	ByOutlet []TaxOutletRow   `json:"by_outlet"`
}

type CashFlowSummary struct {
	// Penerimaan Operasi
	SalesReceipts float64 `json:"sales_receipts"`
	OtherReceipts float64 `json:"other_receipts"`
	TotalReceipts float64 `json:"total_receipts"`
	// Pengeluaran Operasi
	COGSPayments    float64 `json:"cogs_payments"`
	ServicePayments float64 `json:"service_payments"`
	OpexPayments    float64 `json:"opex_payments"`
	TotalPayments   float64 `json:"total_payments"`
	// Arus Kas Bersih
	NetCashFlow float64 `json:"net_cash_flow"`
}

type CashFlowRow struct {
	Date            string  `json:"date"`
	SalesReceipts   float64 `json:"sales_receipts"`
	OtherReceipts   float64 `json:"other_receipts"`
	COGSPayments    float64 `json:"cogs_payments"`
	ServicePayments float64 `json:"service_payments"`
	OpexPayments    float64 `json:"opex_payments"`
	NetCashFlow     float64 `json:"net_cash_flow"`
}

type CashFlowResponse struct {
	Summary CashFlowSummary `json:"summary"`
	Daily   []CashFlowRow   `json:"daily"`
}

type BalanceOutletRow struct {
	OutletID   string `json:"outlet_id"`
	OutletName string `json:"outlet_name"`
	// Aset
	CashAndEquivalents float64 `json:"cash_and_equivalents"`
	Receivables        float64 `json:"receivables"`
	TotalAssets        float64 `json:"total_assets"`
	// Kewajiban
	AccountsPayable  float64 `json:"accounts_payable"`
	TaxPayable       float64 `json:"tax_payable"`
	TotalLiabilities float64 `json:"total_liabilities"`
	// Ekuitas
	TotalEquity float64 `json:"total_equity"`
	// Detail
	TotalRevenue float64 `json:"total_revenue"`
	TotalCashIn  float64 `json:"total_cash_in"`
	TotalExpense float64 `json:"total_expense"`
	UnpaidAmount float64 `json:"unpaid_amount"`
}

type BalanceResponse struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	// Aset
	CashAndEquivalents float64 `json:"cash_and_equivalents"`
	Receivables        float64 `json:"receivables"`
	TotalAssets        float64 `json:"total_assets"`
	// Kewajiban
	AccountsPayable  float64 `json:"accounts_payable"`
	TaxPayable       float64 `json:"tax_payable"`
	TotalLiabilities float64 `json:"total_liabilities"`
	// Ekuitas
	TotalEquity float64 `json:"total_equity"`
	// Detail
	TotalRevenue float64            `json:"total_revenue"`
	TotalCashIn  float64            `json:"total_cash_in"`
	TotalExpense float64            `json:"total_expense"`
	UnpaidAmount float64            `json:"unpaid_amount"`
	Outlets      []BalanceOutletRow `json:"outlets"`
}

// ── Laba Rugi F&B ───────────────────────────────────────────

type ProfitLossSummary struct {
	// Pendapatan
	SalesRevenue float64 `json:"sales_revenue"`
	OtherIncome  float64 `json:"other_income"`
	TotalRevenue float64 `json:"total_revenue"`
	// HPP
	COGS        float64 `json:"cogs"`
	GrossProfit float64 `json:"gross_profit"`
	GrossMargin float64 `json:"gross_margin"`
	// Beban Operasional
	ServiceExpense   float64 `json:"service_expense"`
	OperatingExpense float64 `json:"operating_expense"`
	TotalOpex        float64 `json:"total_opex"`
	OperatingProfit  float64 `json:"operating_profit"`
	// Pajak
	TaxExpense float64 `json:"tax_expense"`
	// Laba Bersih
	NetProfit float64 `json:"net_profit"`
	NetMargin float64 `json:"net_margin"`
}

type ProfitLossRow struct {
	Date             string  `json:"date"`
	Revenue          float64 `json:"revenue"`
	COGS             float64 `json:"cogs"`
	GrossProfit      float64 `json:"gross_profit"`
	OperatingExpense float64 `json:"operating_expense"`
	NetProfit        float64 `json:"net_profit"`
}

type ProfitLossOutletRow struct {
	OutletID         string  `json:"outlet_id"`
	OutletName       string  `json:"outlet_name"`
	Revenue          float64 `json:"revenue"`
	COGS             float64 `json:"cogs"`
	OperatingExpense float64 `json:"operating_expense"`
	NetProfit        float64 `json:"net_profit"`
}

type ProfitLossResponse struct {
	Summary  ProfitLossSummary     `json:"summary"`
	Daily    []ProfitLossRow       `json:"daily"`
	ByOutlet []ProfitLossOutletRow `json:"by_outlet"`
}

// ── Buku Besar (General Ledger) ─────────────────────────────

type GeneralLedgerAccount struct {
	Code        string               `json:"code"`
	Name        string               `json:"name"`
	Group       string               `json:"group"` // aset, kewajiban, ekuitas, pendapatan, beban
	TotalDebit  float64              `json:"total_debit"`
	TotalCredit float64              `json:"total_credit"`
	Balance     float64              `json:"balance"`
	Entries     []GeneralLedgerEntry `json:"entries"`
}

type GeneralLedgerEntry struct {
	Date        string  `json:"date"`
	Description string  `json:"description"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	Balance     float64 `json:"balance"`
}

type GeneralLedgerSummary struct {
	CashBalance  float64 `json:"cash_balance"`
	TotalRevenue float64 `json:"total_revenue"`
	TotalExpense float64 `json:"total_expense"`
}

type GeneralLedgerResponse struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	// AccountAll = false saat laporan difilter ke satu akun; ringkasan
	// (kas/pendapatan/beban) hanya lengkap kalau semua akun ikut dihitung.
	AccountAll bool                   `json:"account_all"`
	Summary    GeneralLedgerSummary   `json:"summary"`
	Accounts   []GeneralLedgerAccount `json:"accounts"`
}
