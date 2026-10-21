package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

type financeSource struct {
	ID, At, OutletID, WorkUnitID, Reference, Description, Notes, Actor, ProofURL, Counterparty, Method, Period, Status, Kind, Link, Subtype string
	Amount, Extra                                                                                                                           float64
	Items                                                                                                                                   []models.PurchaseRequestItem
}

// SQL uses Go field names for JSON decoding (Postgres folds unquoted aliases
// to lower case, which encoding/json matches case-insensitively).
func financeSourceTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02"} {
		if t, e := time.Parse(layout, s); e == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("waktu sumber keuangan tidak valid: %q", s)
}

func financePair(op *models.FinanceOperation, debit, credit string, amount float64) {
	amount = round2(amount)
	if amount < 0 {
		debit, credit = credit, debit
		amount = -amount
	}
	if amount == 0 {
		return
	}
	d, _ := financeAccount(debit)
	c, _ := financeAccount(credit)
	op.Postings = append(op.Postings, models.FinancePosting{AccountCode: debit, AccountName: d.Name, Debit: amount}, models.FinancePosting{AccountCode: credit, AccountName: c.Name, Credit: amount})
}

func loadFinanceJournal(from, to, outlet string, scope []string) ([]models.FinanceOperation, error) {
	// Read the configured timezone once per report, not once per source row.
	location := GetTimezoneLocation()
	if outlet != "" {
		if !financeInScope(outlet, scope) {
			return nil, ErrFinanceAccess
		}
		scope = []string{outlet}
	}
	args := []interface{}{from, to}
	scopeSQL := ""
	if scope != nil {
		args = append(args, pq.Array(scope))
		scopeSQL = ` AND q.outletid=ANY($3::text[])`
	}
	ops := []models.FinanceOperation{}
	read := func(source, query string, post func(*models.FinanceOperation, financeSource)) error {
		rows, err := database.DB.Query(`SELECT row_to_json(q) FROM (`+query+`) q WHERE q.at>=tz_day_start($1::date) AND q.at<tz_day_start($2::date+1)`+scopeSQL+` ORDER BY q.at,q.id`, args...)
		if err != nil {
			return fmt.Errorf("keuangan %s: %w", source, err)
		}
		defer rows.Close()
		for rows.Next() {
			var raw []byte
			var r financeSource
			if err = rows.Scan(&raw); err != nil {
				return err
			}
			if err = json.Unmarshal(raw, &r); err != nil {
				return err
			}
			at, e := financeSourceTime(r.At)
			if e != nil {
				return e
			}
			op := models.FinanceOperation{ID: source + ":" + r.ID, Source: source, SourceID: r.ID, At: at, Date: at.In(location).Format("2006-01-02"), OutletID: strings.TrimSpace(r.OutletID), WorkUnitID: strings.TrimSpace(r.WorkUnitID), Reference: r.Reference, Description: r.Description, Notes: r.Notes, Actor: r.Actor, Counterparty: r.Counterparty, ProofURL: r.ProofURL, Method: r.Method, Period: r.Period, Status: "posted", Postings: []models.FinancePosting{}}
			post(&op, r)
			if op.Status == "skip" {
				continue
			}
			for _, p := range op.Postings {
				if p.AccountCode == glAccCash {
					op.CashIn += p.Debit
					op.CashOut += p.Credit
				}
			}
			ops = append(ops, op)
		}
		return rows.Err()
	}
	// Sales and their PB1 are taken from the same non-void source as the sales
	// report. Reservation DP is offset below, never received a second time.
	if err := read("sale", `SELECT t.id,t.created_at at,t.outlet_id outletid,t.local_id reference,
		'Penjualan: '||COALESCE(t.cashier_name,'Kasir')||' - '||COALESCE(t.payment_method,'') description,
		t.cashier_name actor,t.orderer_name counterparty,t.payment_method method,t.total_amount amount,t.tax_amount extra,
		CASE WHEN EXISTS(SELECT 1 FROM stock_movements sm WHERE sm.ref_type='transaction' AND sm.ref_id=t.id::text AND sm.movement_type='sale') THEN 'costed' ELSE '' END link
		FROM cloud_transactions t WHERE true`+txNotVoided("t"), func(o *models.FinanceOperation, r financeSource) {
		o.CashCategory = "sales"
		financePair(o, glAccCash, glAccRevenue, r.Amount)
		financePair(o, glAccTaxExpense, glAccTaxPayable, r.Extra)
		if r.Link == "" {
			o.ReviewNote = "Belum ada pemakaian stok tertaut pada penjualan ini; periksa kelengkapan HPP produk."
		}
	}); err != nil {
		return nil, err
	}
	if err := read("pos_cash", `SELECT m.id,m.created_at at,m.outlet_id outletid,m.local_id reference,
		CASE WHEN lower(m.movement_type) IN ('masuk','in','income','pemasukan') THEN 'Kas Masuk: ' ELSE 'Pengeluaran: ' END||m.note description,
		m.note||COALESCE(E'\nKlasifikasi: '||c.notes||' ('||c.actor||')','') notes,m.counterpart_name counterparty,m.movement_type kind,m.amount,c.account_code subtype
		FROM cloud_cash_movements m LEFT JOIN LATERAL (SELECT account_code,notes,actor FROM finance_cash_classifications WHERE movement_id=m.id ORDER BY created_at DESC,id DESC LIMIT 1) c ON true`, func(o *models.FinanceOperation, r financeSource) {
		if meta, ok := financeAccount(r.Subtype); ok {
			o.CashCategory = meta.CashCategory
			if statusIn(strings.ToLower(r.Kind), []string{"masuk", "in", "income", "pemasukan"}) {
				financePair(o, glAccCash, meta.Code, r.Amount)
			} else {
				financePair(o, meta.Code, glAccCash, r.Amount)
			}
			return
		}
		if statusIn(strings.ToLower(r.Kind), []string{"masuk", "in", "income", "pemasukan"}) {
			o.CashCategory = "other"
			financePair(o, glAccCash, glAccOtherIncome, r.Amount)
		} else {
			o.CashCategory = "opex"
			financePair(o, glAccOpex, glAccCash, r.Amount)
		}
		o.ReviewNote = "Akun umum dari POS; periksa uraian kas dan klasifikasinya."
	}); err != nil {
		return nil, err
	}
	if err := read("procurement_accrual", `SELECT pr.id,COALESCE(pr.approved_at,pr.created_at) at,pr.outlet_id outletid,pr.work_unit_id workunitid,
		pr.request_number reference,pr.request_number||' · '||COALESCE(wu.name,'-')||' — '||pr.requested_by description,
		pr.requested_by actor,pr.vendor_name counterparty,pr.notes notes,pr.total_final amount,`+procurementClassExpr("pr")+` kind
		FROM purchase_requests pr LEFT JOIN work_units wu ON wu.id=pr.work_unit_id
		WHERE `+prCommittedCond("pr")+` AND `+countableCond("pr")+` AND pr.total_final>0`, func(o *models.FinanceOperation, r financeSource) {
		acc, label := glAccountForClass(r.Kind)
		if r.Kind == "bahan" {
			acc = "1-350"
			label = "Pengadaan Bahan"
		}
		o.Description = label + ": " + o.Description
		financePair(o, acc, glAccPayable, r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("procurement_payment", `SELECT ph.id,ph.created_at at,pr.outlet_id outletid,pr.work_unit_id workunitid,
		pr.request_number reference,'Bayar: '||pr.request_number||' · '||COALESCE(wu.name,'-')||' — '||pr.requested_by description,
		ph.payment_notes notes,ph.paid_by actor,ph.payment_proof proofurl,ph.vendor_name counterparty,ph.amount,`+procurementClassExpr("pr")+` kind
		FROM `+procurementCashOutFrom+` LEFT JOIN work_units wu ON wu.id=pr.work_unit_id WHERE `+countableCond("pr"), func(o *models.FinanceOperation, r financeSource) {
		o.ProofURL = ""
		for _, part := range strings.Split(r.ProofURL, " | ") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if ValidateUploadURL(part) == nil {
				o.ProofURL = part
			} else {
				o.Notes = strings.TrimSpace(o.Notes + "\nReferensi pembayaran: " + part)
			}
		}
		o.CashCategory = map[string]string{"bahan": "cogs", "jasa": "service", "modal": "capex", "projek": "project"}[r.Kind]
		financePair(o, glAccPayable, glAccCash, r.Amount)
	}); err != nil {
		return nil, err
	}
	// Consumables accepted as habis are expensed at receipt, rather than kept
	// indefinitely in equipment/project assets. The original payment stays put.
	if err := read("procurement_consumed", `SELECT d.id,d.created_at at,pr.outlet_id outletid,pr.work_unit_id workunitid,
		pr.request_number reference,'Barang habis pakai: '||pr.request_number description,d.reason notes,d.actor,
		d.qty amount,d.pr_item_key link,pr.items,`+procurementClassExpr("pr")+` kind
		FROM pr_receiving_decisions d JOIN purchase_requests pr ON pr.id=d.purchase_request_id
		WHERE d.destination='habis' AND `+prCommittedCond("pr")+` AND `+countableCond("pr"), func(o *models.FinanceOperation, r financeSource) {
		account, _ := glAccountForClass(r.Kind)
		if r.Kind == "bahan" {
			account = "1-350"
		}
		matched := false
		_ = forEachPRLine(r.Items, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			if key == r.Link {
				matched = true
				o.Description += " · " + sub.Name
				financePair(o, glAccOpex, account, r.Amount*effectiveUnitPrice(sub))
			}
			return nil
		})
		if !matched {
			o.ReviewNote = "Rincian barang habis pakai tidak ditemukan pada dokumen pengadaan; periksa nilai penerimaan."
		}
	}); err != nil {
		return nil, err
	}
	if err := read("receivable", `SELECT o.id,o.created_at at,o.outlet_id outletid,o.local_id reference,
		'Piutang: '||COALESCE(ot.name,o.outlet_code,'-')||' - '||COALESCE(o.customer_name,'Pelanggan') description,
		o.customer_name counterparty,o.total_amount amount FROM cloud_orders o LEFT JOIN outlets ot ON ot.id=o.outlet_id
		WHERE COALESCE(o.payment_info->>'payment_status','unpaid')<>'paid' AND NULLIF(o.payment_info->>'voided_at','') IS NULL AND COALESCE(o.is_holding,false)=false`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, glAccReceivable, glAccRevenue, r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("reservation_payment", `SELECT p.id,p.paid_at at,p.outlet_id outletid,rv.id reference,
		'Uang muka reservasi ('||p.type||'): '||rv.customer_name description,p.notes,p.submitted_by actor,p.proof_url proofurl,
		rv.customer_name counterparty,p.method,p.type kind,p.amount FROM reservation_payments p JOIN reservations rv ON rv.id=p.reservation_id WHERE p.status='validated'`, func(o *models.FinanceOperation, r financeSource) {
		o.CashCategory = "deposit"
		if r.Kind == "refund" {
			financePair(o, glAccCustomerDep, glAccCash, r.Amount)
		} else {
			financePair(o, glAccCash, glAccCustomerDep, r.Amount)
		}
	}); err != nil {
		return nil, err
	}
	if err := read("reservation_forfeit", `SELECT rv.id,rv.cancelled_at at,rv.outlet_id outletid,rv.id reference,
		'Uang muka hangus: '||rv.customer_name description,rv.customer_name counterparty,
		COALESCE((SELECT SUM(CASE WHEN p.type='refund' THEN -p.amount ELSE p.amount END) FROM reservation_payments p WHERE p.reservation_id=rv.id AND p.status='validated'),0) amount
		FROM reservations rv WHERE rv.status='cancelled' AND rv.cancel_disposition='hangus'`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, glAccCustomerDep, glAccOtherIncome, r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("reservation_usage", `SELECT transaction_payments.id,transaction_payments.created_at at,transaction_payments.outlet_id outletid,t.local_id reference,
		'Pemakaian uang muka: '||COALESCE(NULLIF(t.orderer_name,''),t.cashier_name,'Kasir') description,transaction_payments.amount
		FROM transaction_payments JOIN cloud_transactions t ON t.id=transaction_payments.transaction_id
		WHERE transaction_payments.payment_method='`+ReservationDpMethod+`'`+payNotVoided, func(o *models.FinanceOperation, r financeSource) {
		o.CashCategory = "sales"
		financePair(o, glAccCustomerDep, glAccCash, r.Amount)
	}); err != nil {
		return nil, err
	}
	// Closing a reservation may arrive before the POS transaction sync. Keep
	// the unmatched DP as a liability in a clearing account, never as income or
	// cash. Once the matching payment arrives it is handled by usage above.
	if err := read("reservation_pending", `SELECT rv.id,rv.settled_at at,rv.outlet_id outletid,rv.pos_transaction_id reference,
		'DP menunggu sinkronisasi POS: '||rv.customer_name description,rv.customer_name counterparty,
		GREATEST(0,COALESCE((SELECT SUM(CASE WHEN p.type='refund' THEN -p.amount ELSE p.amount END) FROM reservation_payments p WHERE p.reservation_id=rv.id AND p.status='validated'),0)
		-COALESCE((SELECT SUM(tp.amount) FROM transaction_payments tp JOIN cloud_transactions t ON t.id=tp.transaction_id WHERE t.outlet_id=rv.outlet_id AND (t.id::text=rv.pos_transaction_id OR t.local_id=rv.pos_transaction_id) AND tp.payment_method='`+ReservationDpMethod+`'),0)) amount
		FROM reservations rv WHERE rv.status='done'`, func(o *models.FinanceOperation, r financeSource) {
		if r.Amount <= 0 {
			o.Status = "skip"
			return
		}
		financePair(o, glAccCustomerDep, "2-350", r.Amount)
		o.ReviewNote = "Reservasi selesai, tetapi rincian pemakaian DP di transaksi POS belum lengkap. Periksa sinkronisasi; jangan catat sebagai pemasukan baru."
	}); err != nil {
		return nil, err
	}
	// Every physical stock movement has exactly one financial counterpart.
	// Purchase / production / transfers are assets, not expense or cash flow.
	if err := read("stock", `SELECT sm.id,sm.created_at at,w.outlet_id outletid,sm.ref_number reference,
		sm.movement_type||': '||si.name||' · '||w.name description,sm.notes,sm.created_by actor,
		sm.qty_base*sm.cost_per_base amount,sm.movement_type kind,sm.ref_type subtype,
		COALESCE(gr.purchase_request_id,'') link
		FROM stock_movements sm JOIN warehouses w ON w.id=sm.warehouse_id JOIN stock_items si ON si.id=sm.item_id
		LEFT JOIN goods_receipts gr ON sm.ref_type='goods_receipt' AND gr.id::text=sm.ref_id`, func(o *models.FinanceOperation, r financeSource) {
		counter := "4-230"
		switch r.Kind {
		case "sale":
			counter = glAccCOGS
		case "purchase_in":
			counter = "2-150"
			if r.Link != "" {
				counter = "1-350"
			}
		case "transfer_out":
			counter = "1-310"
		case "transfer_in":
			counter = "3-200"
		case "production_out", "production_in", "breakdown_out", "breakdown_in":
			counter = "1-330"
		case "quality_hold", "quality_release", "supplier_replacement":
			counter = "1-320"
		case "waste", "spoiled", "expired":
			counter = "5-380"
		case "adjustment", "return_in":
			if r.Subtype == "opening" {
				counter = "3-100"
			} else if r.Amount < 0 {
				counter = "5-380"
			}
		default:
			o.ReviewNote = "Jenis mutasi belum memiliki pemetaan khusus; periksa klasifikasi."
			if r.Amount < 0 {
				counter = "5-380"
			}
		}
		financePair(o, "1-300", counter, r.Amount)
		if r.Amount == 0 {
			o.ReviewNote = "Nilai persediaan belum tersedia; lengkapi harga pokok di sumber."
		}
	}); err != nil {
		return nil, err
	}
	// Disposing a quarantine does not deduct saleable stock again. Refunds
	// settle the quarantined asset; only a difference is income or expense.
	if err := read("stock_quality", `SELECT e.id,(e.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,c.number reference,
		e.action||': '||c.number||' · '||si.name description,e.notes,e.actor_name actor,se.url proofurl,c.vendor_name counterparty,
		CASE WHEN e.action='refund' THEN e.amount ELSE c.qty_base*c.cost_per_base END amount,
		e.qty_base*c.cost_per_base extra,e.action kind,c.disposition subtype,c.id link
		FROM stock_quality_events e JOIN stock_quality_cases c ON c.id=e.case_id JOIN warehouses w ON w.id=c.warehouse_id
		JOIN stock_items si ON si.id=c.item_id LEFT JOIN stock_evidence se ON se.id=e.evidence_id
		WHERE e.action='refund' OR (e.action='execute' AND c.disposition='dispose')`, func(o *models.FinanceOperation, r financeSource) {
		if r.Kind == "refund" {
			o.CashCategory = "supplier_refund"
			financePair(o, glAccCash, "1-320", r.Amount)
			if r.Extra > r.Amount {
				financePair(o, "5-390", "1-320", r.Extra-r.Amount)
			} else {
				financePair(o, "1-320", "4-220", r.Amount-r.Extra)
			}
		} else {
			financePair(o, "5-380", "1-320", r.Amount)
		}
	}); err != nil {
		return nil, err
	}
	// A rejected transfer enters quarantine directly, without a saleable stock
	// movement. Consume the in-transit account once, on the receipt event.
	if err := read("stock_reject", `SELECT c.id,(c.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,c.number reference,
		'Reject penerimaan: '||c.number description,c.notes,c.created_name actor,c.qty_base*c.cost_per_base amount
		FROM stock_quality_cases c JOIN warehouses w ON w.id=c.warehouse_id WHERE c.transfer_id IS NOT NULL AND c.parent_case_id IS NULL`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "1-320", "3-200", r.Amount)
	}); err != nil {
		return nil, err
	}
	// Clear the sender's in-transit asset only as the recipient accepts/rejects it.
	if err := read("stock_transfer_settlement", `SELECT sm.id,sm.created_at at,w.outlet_id outletid,st.transfer_number reference,
		'Penyelesaian pengiriman: '||st.transfer_number description,sm.notes,sm.created_by actor,sm.qty_base*sm.cost_per_base amount
		FROM stock_movements sm JOIN stock_transfers st ON st.id::text=sm.ref_id JOIN warehouses w ON w.id=st.from_warehouse_id
		WHERE sm.ref_type='stock_transfer' AND sm.movement_type='transfer_in'
		UNION ALL SELECT c.id,(c.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,st.transfer_number reference,
		'Penerimaan reject: '||c.number description,c.notes,c.created_name actor,c.qty_base*c.cost_per_base amount
		FROM stock_quality_cases c JOIN stock_transfers st ON st.id=c.transfer_id JOIN warehouses w ON w.id=st.from_warehouse_id
		WHERE c.parent_case_id IS NULL`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "3-200", "1-310", r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("stock_return_dispatch", `SELECT e.id,(e.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,c.number reference,
		'Kirim retur outlet: '||c.number description,e.notes,e.actor_name actor,c.qty_base*c.cost_per_base amount
		FROM stock_quality_events e JOIN stock_quality_cases c ON c.id=e.case_id JOIN warehouses w ON w.id=c.warehouse_id
		WHERE e.action='execute' AND c.disposition='return_warehouse'`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "1-310", "1-320", r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("stock_return_receipt", `SELECT e.id,(e.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,c.number reference,
		'Terima retur outlet: '||c.number description,e.notes,e.actor_name actor,c.qty_base*c.cost_per_base amount
		FROM stock_quality_events e JOIN stock_quality_cases c ON c.id=e.case_id JOIN warehouses w ON w.id=c.return_warehouse_id
		WHERE e.action='receive_return'`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "1-320", "3-200", r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("stock_return_settlement", `SELECT e.id,(e.created_at AT TIME ZONE 'UTC') at,w.outlet_id outletid,c.number reference,
		'Retur diterima induk: '||c.number description,e.notes,e.actor_name actor,c.qty_base*c.cost_per_base amount
		FROM stock_quality_events e JOIN stock_quality_cases c ON c.id=e.case_id JOIN warehouses w ON w.id=c.warehouse_id
		WHERE e.action='receive_return'`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "3-200", "1-310", r.Amount)
	}); err != nil {
		return nil, err
	}
	// Maintenance linked to procurement is already accrued there. An unlinked
	// completed job creates a payable; a cost field alone is not proof of cash.
	if err := read("maintenance", `SELECT m.id,m.maintenance_date::timestamp at,a.outlet_id outletid,m.wo_number reference,
		'Perawatan: '||a.name description,m.description notes,m.performed_by actor,m.attachment_url proofurl,m.cost amount
		FROM asset_maintenances m JOIN assets a ON a.id=m.asset_id WHERE m.status='selesai' AND m.purchase_request_id IS NULL AND m.cost>0`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "5-360", "2-150", r.Amount)
		o.ReviewNote = "Biaya perawatan diakui sebagai hutang. Catat pelunasannya pada akun Hutang Operasional Lainnya jika sudah dibayar."
	}); err != nil {
		return nil, err
	}
	// A manually registered asset has no cash-payment document. Carry its
	// acquisition value against opening capital; procurement assets are already
	// posted by their purchase request and are deliberately excluded here.
	if err := read("asset_opening", `SELECT a.id,COALESCE(a.purchase_date::timestamp,a.created_at) at,a.outlet_id outletid,a.code reference,
		'Aset awal: '||a.name description,a.notes,
		a.purchase_price*(a.quantity+COALESCE((SELECT SUM(d.qty) FROM asset_disposals d WHERE d.asset_id=a.id AND d.status='approved'),0)) amount
		FROM assets a WHERE a.purchase_request_id IS NULL AND a.is_deleted=false`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, glAccFixedAsset, "3-100", r.Amount)
	}); err != nil {
		return nil, err
	}
	if err := read("asset_disposal", `SELECT d.id,d.approved_at at,a.outlet_id outletid,d.disposal_number reference,
		'Pelepasan aset: '||a.name description,d.reason notes,d.approved_by actor,d.attachment_url proofurl,d.proceeds amount,d.book_value extra
		FROM asset_disposals d JOIN assets a ON a.id=d.asset_id WHERE d.status='approved'`, func(o *models.FinanceOperation, r financeSource) {
		financePair(o, "1-250", glAccFixedAsset, r.Amount)
		if r.Extra > r.Amount {
			financePair(o, "5-390", glAccFixedAsset, r.Extra-r.Amount)
		} else {
			financePair(o, glAccFixedAsset, "4-220", r.Amount-r.Extra)
		}
		if r.Amount > 0 {
			o.ReviewNote = "Hasil penjualan aset dicatat sebagai piutang sampai penerimaannya dicatat pada akun Piutang Lainnya / Penjualan Aset."
		}
	}); err != nil {
		return nil, err
	}
	// Project spending is capitalized when the equipment is actually registered.
	// Two sides keep the allocation correct even when a project supplies another outlet.
	for _, side := range []struct{ source, outlet, debit, credit string }{
		{"project_asset_release", "pr.outlet_id", "3-200", glAccProjectWIP},
		{"project_asset_receipt", "a.outlet_id", glAccFixedAsset, "3-200"},
	} {
		if err := read(side.source, `SELECT a.id,a.created_at at,`+side.outlet+` outletid,pr.request_number reference,
			'Kapitalisasi projek: '||a.name description,a.notes,
			a.purchase_price*(a.quantity+COALESCE((SELECT SUM(d.qty) FROM asset_disposals d WHERE d.asset_id=a.id AND d.status='approved'),0)) amount
			FROM assets a JOIN purchase_requests pr ON pr.id=a.purchase_request_id
			WHERE a.is_deleted=false AND `+prCommittedCond("pr")+` AND `+countableCond("pr")+` AND `+procurementClassExpr("pr")+`='projek'`, func(o *models.FinanceOperation, r financeSource) {
			financePair(o, side.debit, side.credit, r.Amount)
		}); err != nil {
			return nil, err
		}
	}
	if err := read("manual", `SELECT f.id,f.occurred_at at,f.outlet_id outletid,f.work_unit_id workunitid,f.number reference,f.description,f.notes,f.created_by actor,
		f.proof_url proofurl,f.counterparty,f.method,f.period,f.status,f.amount,f.account_code kind,f.direction subtype
		FROM finance_entries f WHERE f.posted_at IS NOT NULL`, func(o *models.FinanceOperation, r financeSource) {
		meta, _ := financeAccount(r.Kind)
		o.CashCategory = meta.CashCategory
		if r.Subtype == "in" {
			financePair(o, glAccCash, r.Kind, r.Amount)
		} else {
			financePair(o, r.Kind, glAccCash, r.Amount)
		}
	}); err != nil {
		return nil, err
	}
	if err := read("manual_reversal", `SELECT f.id,f.voided_at at,f.outlet_id outletid,f.work_unit_id workunitid,f.number reference,
		'Pembalikan: '||f.description description,f.void_reason notes,f.voided_by actor,f.proof_url proofurl,f.counterparty,f.method,f.period,
		f.amount,f.account_code kind,f.direction subtype FROM finance_entries f WHERE f.status='voided' AND f.posted_at IS NOT NULL`, func(o *models.FinanceOperation, r financeSource) {
		meta, _ := financeAccount(r.Kind)
		o.CashCategory = meta.CashCategory
		if r.Subtype == "in" {
			financePair(o, r.Kind, glAccCash, r.Amount)
		} else {
			financePair(o, glAccCash, r.Kind, r.Amount)
		}
	}); err != nil {
		return nil, err
	}
	// Depreciation keeps the established asset calculation, split per outlet
	// and period, so all report breakdowns include exactly the same expense.
	f, ef := time.Parse("2006-01-02", from)
	t, et := time.Parse("2006-01-02", to)
	if ef != nil || et != nil || t.Before(f) {
		return nil, Invalid("rentang tanggal tidak valid")
	}
	// Avoid iterating empty decades for an as-of balance report.
	var firstAsset *time.Time
	if err := database.DB.QueryRow(`SELECT MIN(purchase_date) FROM assets WHERE is_deleted=false`).Scan(&firstAsset); err != nil {
		return nil, err
	}
	if firstAsset == nil {
		f = t.AddDate(0, 1, 0)
	} else if f.Before(*firstAsset) {
		f = *firstAsset
	}
	for cur := time.Date(f.Year(), f.Month(), 1, 0, 0, 0, 0, time.UTC); !cur.After(t); cur = cur.AddDate(0, 1, 0) {
		start, end := cur, cur.AddDate(0, 1, -1)
		if start.Before(f) {
			start = f
		}
		if end.After(t) {
			end = t
		}
		_, by, err := depreciationForPeriod(start.Format("2006-01-02"), end.Format("2006-01-02"), scope)
		if err != nil {
			return nil, err
		}
		for oid, amount := range by {
			if round2(amount) == 0 {
				continue
			}
			at := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, location)
			o := models.FinanceOperation{ID: "depreciation:" + oid + ":" + cur.Format("2006-01"), Source: "depreciation", At: at, Date: end.Format("2006-01-02"), OutletID: oid, Description: "Penyusutan aset " + cur.Format("2006-01"), Status: "posted", Postings: []models.FinancePosting{}}
			financePair(&o, glAccDepreciation, glAccFixedAsset, amount)
			ops = append(ops, o)
		}
	}
	// Resolve display names once. Names do not affect amounts or access.
	names := map[string]string{}
	wuNames := map[string]string{}
	rows, err := database.DB.Query(`SELECT id,name FROM outlets`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		names[strings.TrimSpace(id)] = name
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = database.DB.Query(`SELECT id,name FROM work_units`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		wuNames[strings.TrimSpace(id)] = name
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range ops {
		ops[i].OutletName = names[ops[i].OutletID]
		if ops[i].OutletID == "" {
			ops[i].OutletName = "Pusat"
		}
		ops[i].WorkUnitName = wuNames[ops[i].WorkUnitID]
		var debit, credit float64
		for _, p := range ops[i].Postings {
			debit += p.Debit
			credit += p.Credit
			if _, ok := financeAccount(p.AccountCode); !ok {
				return nil, fmt.Errorf("akun %s belum terdaftar", p.AccountCode)
			}
		}
		if math.Abs(debit-credit) > .001 {
			return nil, fmt.Errorf("jurnal %s tidak seimbang", ops[i].ID)
		}
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].At.Equal(ops[j].At) {
			return ops[i].ID < ops[j].ID
		}
		return ops[i].At.Before(ops[j].At)
	})
	return ops, nil
}
