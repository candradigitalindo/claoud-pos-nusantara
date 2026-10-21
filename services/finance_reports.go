package services

import (
	"cloud-pos/models"
	"sort"
	"strings"
)

func financeLedger(from, to, outlet, filter string, scope []string) (*models.GeneralLedgerResponse, error) {
	if filter != "" {
		if _, ok := financeAccount(filter); !ok {
			return nil, Invalid("akun tidak dikenal")
		}
	}
	ops, err := loadFinanceJournal(from, to, outlet, scope)
	if err != nil {
		return nil, err
	}
	rep := &models.GeneralLedgerResponse{DateFrom: from, DateTo: to, AccountAll: filter == "", Accounts: []models.GeneralLedgerAccount{}}
	for _, meta := range FinanceAccounts {
		if filter != "" && filter != meta.Code {
			continue
		}
		a := models.GeneralLedgerAccount{Code: meta.Code, Name: meta.Name, Group: meta.Group, Entries: []models.GeneralLedgerEntry{}}
		for _, o := range ops {
			for _, p := range o.Postings {
				if p.AccountCode != meta.Code {
					continue
				}
				a.TotalDebit += p.Debit
				a.TotalCredit += p.Credit
				if meta.Group == "aset" || meta.Group == "beban" {
					a.Balance += p.Debit - p.Credit
				} else {
					a.Balance += p.Credit - p.Debit
				}
				a.Entries = append(a.Entries, models.GeneralLedgerEntry{Date: o.Date, Description: o.Description, Debit: p.Debit, Credit: p.Credit, Balance: round2(a.Balance), Source: o.Source, SourceID: o.SourceID, Reference: o.Reference, OutletName: o.OutletName, Notes: o.Notes, ProofURL: o.ProofURL})
			}
		}
		a.TotalDebit = round2(a.TotalDebit)
		a.TotalCredit = round2(a.TotalCredit)
		a.Balance = round2(a.Balance)
		rep.Accounts = append(rep.Accounts, a)
		if a.Code == glAccCash {
			rep.Summary.CashBalance = a.Balance
		}
		if a.Group == "pendapatan" {
			rep.Summary.TotalRevenue += a.Balance
		}
		if a.Group == "beban" {
			rep.Summary.TotalExpense += a.Balance
		}
	}
	rep.Summary.TotalRevenue = round2(rep.Summary.TotalRevenue)
	rep.Summary.TotalExpense = round2(rep.Summary.TotalExpense)
	return rep, nil
}

func financeCashFlow(from, to, outlet string, scope []string) (*models.CashFlowResponse, error) {
	ops, err := loadFinanceJournal(from, to, outlet, scope)
	if err != nil {
		return nil, err
	}
	rep := &models.CashFlowResponse{Daily: []models.CashFlowRow{}}
	by := map[string]*models.CashFlowRow{}
	for _, o := range ops {
		if o.CashIn == 0 && o.CashOut == 0 {
			continue
		}
		r := by[o.Date]
		if r == nil {
			r = &models.CashFlowRow{Date: o.Date}
			by[o.Date] = r
		}
		net := round2(o.CashIn - o.CashOut)
		switch o.CashCategory {
		case "sales":
			r.SalesReceipts += net
		case "cogs":
			r.COGSPayments -= net
		case "service":
			r.ServicePayments -= net
		case "opex":
			r.OpexPayments -= net
		case "capex":
			r.CapexPayments -= net
		case "project":
			r.ProjectPayments -= net
		case "deposit":
			r.DepositReceipts += o.CashIn
			r.DepositRefunds += o.CashOut
		case "supplier_refund":
			r.SupplierRefunds += net
		case "financing":
			r.FinancingReceipts += o.CashIn
			r.FinancingPayments += o.CashOut
		default:
			r.OtherReceipts += net
		}
		r.NetCashFlow += net
	}
	s := &rep.Summary
	for _, r := range by {
		r.NetCashFlow = round2(r.NetCashFlow)
		rep.Daily = append(rep.Daily, *r)
		s.SalesReceipts += r.SalesReceipts
		s.OtherReceipts += r.OtherReceipts
		s.DepositReceipts += r.DepositReceipts
		s.COGSPayments += r.COGSPayments
		s.ServicePayments += r.ServicePayments
		s.OpexPayments += r.OpexPayments
		s.CapexPayments += r.CapexPayments
		s.ProjectPayments += r.ProjectPayments
		s.DepositRefunds += r.DepositRefunds
		s.SupplierRefunds += r.SupplierRefunds
		s.FinancingReceipts += r.FinancingReceipts
		s.FinancingPayments += r.FinancingPayments
	}
	s.TotalReceipts = round2(s.SalesReceipts + s.OtherReceipts + s.DepositReceipts + s.SupplierRefunds + s.FinancingReceipts)
	s.TotalPayments = round2(s.COGSPayments + s.ServicePayments + s.OpexPayments + s.CapexPayments + s.ProjectPayments + s.DepositRefunds + s.FinancingPayments)
	s.NetCashFlow = round2(s.TotalReceipts - s.TotalPayments)
	sort.Slice(rep.Daily, func(i, j int) bool { return rep.Daily[i].Date > rep.Daily[j].Date })
	return rep, nil
}

func financeProfitLoss(from, to, outlet string, scope []string) (*models.ProfitLossResponse, error) {
	ops, err := loadFinanceJournal(from, to, outlet, scope)
	if err != nil {
		return nil, err
	}
	rep := &models.ProfitLossResponse{Daily: []models.ProfitLossRow{}, ByOutlet: []models.ProfitLossOutletRow{}, Accounts: []models.FinanceAccountTotal{}}
	balances := map[string]float64{}
	daily := map[string]*models.ProfitLossRow{}
	byOutlet := map[string]*models.ProfitLossOutletRow{}
	s := &rep.Summary
	for _, o := range ops {
		if o.ReviewNote != "" {
			rep.ReviewCount++
			if o.Source == "sale" {
				rep.UncostedSales++
			}
		}
		if o.CashCategory == "capex" {
			s.CapexPayments += o.CashOut - o.CashIn
		}
		if o.CashCategory == "project" {
			s.ProjectPayments += o.CashOut - o.CashIn
		}
		for _, p := range o.Postings {
			meta, _ := financeAccount(p.AccountCode)
			if meta.Group != "pendapatan" && meta.Group != "beban" {
				continue
			}
			v := round2(p.Debit - p.Credit)
			if meta.Group == "pendapatan" {
				v = -v
			}
			balances[p.AccountCode] += v
			d := daily[o.Date]
			if d == nil {
				d = &models.ProfitLossRow{Date: o.Date}
				daily[o.Date] = d
			}
			b := byOutlet[o.OutletID]
			if b == nil {
				b = &models.ProfitLossOutletRow{OutletID: o.OutletID, OutletName: o.OutletName}
				byOutlet[o.OutletID] = b
			}
			if meta.Group == "pendapatan" {
				d.Revenue += v
				b.Revenue += v
			} else if p.AccountCode == glAccCOGS {
				d.COGS += v
				b.COGS += v
			} else {
				d.OperatingExpense += v
				if p.AccountCode == glAccDepreciation {
					b.Depreciation += v
				} else {
					b.OperatingExpense += v
				}
			}
			if o.Source == "reservation_forfeit" && p.AccountCode == glAccOtherIncome {
				s.ForfeitedDeposits += v
			}
		}
	}
	for _, meta := range FinanceAccounts {
		v := round2(balances[meta.Code])
		if v == 0 {
			continue
		}
		if meta.Group != "pendapatan" && meta.Group != "beban" {
			continue
		}
		rep.Accounts = append(rep.Accounts, models.FinanceAccountTotal{FinanceAccount: meta, Amount: v})
		if meta.Group == "pendapatan" {
			if meta.Code == glAccRevenue {
				s.SalesRevenue += v
			} else {
				s.OtherIncome += v
			}
			continue
		}
		switch meta.Code {
		case glAccCOGS:
			s.COGS += v
		case glAccServiceExp:
			s.ServiceExpense += v
		case glAccDepreciation:
			s.DepreciationExpense += v
		case glAccTaxExpense:
			s.TaxExpense += v
		default:
			s.OperatingExpense += v
		}
	}
	s.TotalRevenue = round2(s.SalesRevenue + s.OtherIncome)
	s.GrossProfit = round2(s.SalesRevenue - s.COGS)
	s.TotalOpex = round2(s.ServiceExpense + s.OperatingExpense + s.DepreciationExpense)
	s.OperatingProfit = round2(s.GrossProfit + s.OtherIncome - s.TotalOpex)
	s.NetProfit = round2(s.OperatingProfit - s.TaxExpense)
	if s.SalesRevenue != 0 {
		s.GrossMargin = round2(s.GrossProfit / s.SalesRevenue * 100)
	}
	if s.TotalRevenue != 0 {
		s.NetMargin = round2(s.NetProfit / s.TotalRevenue * 100)
	}
	for _, d := range daily {
		d.Revenue = round2(d.Revenue)
		d.COGS = round2(d.COGS)
		d.OperatingExpense = round2(d.OperatingExpense)
		d.GrossProfit = round2(d.Revenue - d.COGS)
		d.NetProfit = round2(d.GrossProfit - d.OperatingExpense)
		rep.Daily = append(rep.Daily, *d)
	}
	for _, b := range byOutlet {
		b.Revenue = round2(b.Revenue)
		b.COGS = round2(b.COGS)
		b.OperatingExpense = round2(b.OperatingExpense)
		b.Depreciation = round2(b.Depreciation)
		b.NetProfit = round2(b.Revenue - b.COGS - b.OperatingExpense - b.Depreciation)
		rep.ByOutlet = append(rep.ByOutlet, *b)
	}
	sort.Slice(rep.Daily, func(i, j int) bool { return rep.Daily[i].Date > rep.Daily[j].Date })
	sort.Slice(rep.ByOutlet, func(i, j int) bool { return rep.ByOutlet[i].OutletName < rep.ByOutlet[j].OutletName })
	return rep, nil
}

func ListFinanceOperations(from, to, outlet, source, direction, account, search string, page, limit int, scope []string) (*models.FinanceOperationsResponse, error) {
	ops, err := loadFinanceJournal(from, to, outlet, scope)
	if err != nil {
		return nil, err
	}
	rep := &models.FinanceOperationsResponse{Rows: []models.FinanceOperation{}}
	search = strings.ToLower(strings.TrimSpace(search))
	filtered := []models.FinanceOperation{}
	for i := len(ops) - 1; i >= 0; i-- {
		o := ops[i]
		if source == "review" && o.ReviewNote == "" {
			continue
		}
		if source != "" && source != "review" && o.Source != source {
			continue
		}
		if direction == "in" && o.CashIn == 0 || direction == "out" && o.CashOut == 0 || direction == "noncash" && (o.CashIn != 0 || o.CashOut != 0) {
			continue
		}
		if account != "" {
			found := false
			for _, p := range o.Postings {
				if p.AccountCode == account {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if search != "" && !strings.Contains(strings.ToLower(o.Reference+" "+o.Description+" "+o.Notes+" "+o.Counterparty+" "+o.OutletName+" "+o.WorkUnitName), search) {
			continue
		}
		filtered = append(filtered, o)
		rep.CashIn += o.CashIn
		rep.CashOut += o.CashOut
		if o.ReviewNote != "" {
			rep.ReviewCount++
		}
	}
	rep.Total = len(filtered)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	start := (page - 1) * limit
	if start < len(filtered) {
		end := start + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		rep.Rows = filtered[start:end]
	}
	rep.CashIn = round2(rep.CashIn)
	rep.CashOut = round2(rep.CashOut)
	return rep, nil
}

func financeBalance(from, to, outlet string, scope []string) (*models.BalanceResponse, error) {
	// A balance is cumulative to the closing date. The selected start date is
	// only retained for display; changing it must not erase opening balances.
	ops, err := loadFinanceJournal("1900-01-01", to, outlet, scope)
	if err != nil {
		return nil, err
	}
	balances := map[string]float64{}
	by := map[string]map[string]float64{}
	names := map[string]string{}
	for _, o := range ops {
		if by[o.OutletID] == nil {
			by[o.OutletID] = map[string]float64{}
		}
		names[o.OutletID] = o.OutletName
		for _, p := range o.Postings {
			m, _ := financeAccount(p.AccountCode)
			v := p.Debit - p.Credit
			if m.Group != "aset" && m.Group != "beban" {
				v = -v
			}
			balances[p.AccountCode] += v
			by[o.OutletID][p.AccountCode] += v
		}
	}
	makeRow := func(b map[string]float64) models.BalanceOutletRow {
		r := models.BalanceOutletRow{CashAndEquivalents: round2(b[glAccCash]), Receivables: round2(b[glAccReceivable] + b["1-250"]), FixedAssets: round2(b[glAccFixedAsset]), ProjectsInProgress: round2(b[glAccProjectWIP]), AccountsPayable: round2(b[glAccPayable] + b["2-150"] + b["2-350"]), CustomerDeposits: round2(b[glAccCustomerDep]), TaxPayable: round2(b[glAccTaxPayable]), FinancingLiabilities: round2(b["2-400"])}
		for _, code := range []string{"1-300", "1-310", "1-320", "1-330", "1-350"} {
			r.Inventory += b[code]
		}
		r.Inventory = round2(r.Inventory)
		r.TotalAssets = round2(r.CashAndEquivalents + r.Receivables + r.FixedAssets + r.ProjectsInProgress + r.Inventory)
		r.TotalLiabilities = round2(r.AccountsPayable + r.CustomerDeposits + r.TaxPayable + r.FinancingLiabilities)
		r.TotalEquity = round2(r.TotalAssets - r.TotalLiabilities)
		r.TotalRevenue = round2(b[glAccRevenue])
		r.UnpaidAmount = r.Receivables
		for _, m := range FinanceAccounts {
			if m.Group == "pendapatan" && m.Code != glAccRevenue {
				r.TotalCashIn += b[m.Code]
			}
			if m.Group == "beban" {
				r.TotalExpense += b[m.Code]
			}
		}
		return r
	}
	r := makeRow(balances)
	rep := &models.BalanceResponse{DateFrom: from, DateTo: to, CashAndEquivalents: r.CashAndEquivalents, Receivables: r.Receivables, Inventory: r.Inventory, FixedAssets: r.FixedAssets, ProjectsInProgress: r.ProjectsInProgress, TotalAssets: r.TotalAssets, AccountsPayable: r.AccountsPayable, CustomerDeposits: r.CustomerDeposits, TaxPayable: r.TaxPayable, FinancingLiabilities: r.FinancingLiabilities, TotalLiabilities: r.TotalLiabilities, TotalEquity: r.TotalEquity, TotalRevenue: r.TotalRevenue, TotalCashIn: r.TotalCashIn, TotalExpense: r.TotalExpense, UnpaidAmount: r.UnpaidAmount, Outlets: []models.BalanceOutletRow{}, Accounts: []models.FinanceAccountTotal{}}
	for oid, b := range by {
		r := makeRow(b)
		r.OutletID = oid
		r.OutletName = names[oid]
		rep.Outlets = append(rep.Outlets, r)
	}
	for _, m := range FinanceAccounts {
		if m.Group == "beban" || m.Group == "pendapatan" {
			continue
		}
		rep.Accounts = append(rep.Accounts, models.FinanceAccountTotal{FinanceAccount: m, Amount: round2(balances[m.Code])})
	}
	sort.Slice(rep.Outlets, func(i, j int) bool { return rep.Outlets[i].OutletName < rep.Outlets[j].OutletName })
	return rep, nil
}
