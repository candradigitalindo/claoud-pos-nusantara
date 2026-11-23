package services

import (
	"strings"
	"unicode"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// ── Rincian entri Buku Besar ───────────────────────────────────────────────
//
// Keterangan jurnal POS lama berbentuk "Pembayaran POS: 01M69EH…" dan
// referensinya ID yang sama — tidak terbaca manusia. Buku Besar kini memberi
// satu baris rincian ringkas yang bisa dicocokkan dengan struk & orang:
//
//	POS        TRX-01M7SSP8 · Meja 9 · Araaa · Kasir Zia · QRIS
//	pengadaan  Vendor tes · dibayar Cindy
//	kas keluar Ibu Yuli
//
// Nomor struk = "TRX-" + 8 karakter awal ID order, sama dengan yang dicetak
// tablet (ReceiptBuilder). ID mentah tetap ada di Reference untuk audit.

// financePOSLedgerSources: sumber jurnal yang berasal dari order/transaksi POS.
var financePOSLedgerSources = map[string]bool{
	"sale": true, "receivable": true, "sale_tax": true, "sale_compliment": true,
	"sale_void": true, "sale_refund": true, "sale_tax_void": true, "sale_correction": true,
	"sale_tax_correction": true, "sale_link_correction": true, "pos_review": true,
}

type posLedgerInfo struct{ receipt, table, customer, staff string }

type posOrderRow struct{ id, localID, table, customer, creator string }

// isOpaqueID: ULID (26 huruf Crockford) atau UUID — tidak bermakna di layar.
func isOpaqueID(s string) bool {
	s = strings.TrimSpace(s)
	switch len(s) {
	case 26:
		for _, c := range s {
			if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z') || strings.ContainsRune("ILOU", c) {
				return false
			}
		}
		return true
	case 36:
		for i, c := range s {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				if c != '-' {
					return false
				}
				continue
			}
			if !unicode.Is(unicode.ASCII_Hex_Digit, c) {
				return false
			}
		}
		return true
	}
	return false
}

// posReceiptNo: nomor struk seperti dicetak tablet ("TRX-01M7SSP8").
func posReceiptNo(orderLocalID string) string {
	id := strings.TrimSpace(orderLocalID)
	if len(id) < 8 {
		return ""
	}
	return "TRX-" + strings.ToUpper(id[:8])
}

// posTableLabel: "9" → "Meja 9"; nama area ("TERAS 1", "IN 02") apa adanya.
func posTableLabel(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	for _, c := range t {
		if c < '0' || c > '9' {
			return t
		}
	}
	return "Meja " + t
}

// ledgerMethodLabel: label metode bayar singkat.
func ledgerMethodLabel(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "":
		return ""
	case "cash", "tunai":
		return "Tunai"
	case "qris":
		return "QRIS"
	case "card", "debit", "credit":
		return "Kartu"
	case "transfer":
		return "Transfer"
	case ReservationDpMethod:
		return "Uang muka reservasi"
	case "compliment":
		return "Komplimen"
	case "mixed":
		return "Campuran"
	case "outlet_cash":
		return "Kas Kasir Outlet"
	case "marketplace_va":
		return "VA Marketplace"
	case "vendor_account":
		return "Transfer rekening vendor"
	case "adjustment":
		return "Penyesuaian"
	}
	return paymentMethodLabel(m)
}

// staffLabel: "Kasir Zia" / "oleh Zia"; nama akun yang sudah berupa peran
// ("Kasir", "Kasir Bawah") tidak diberi awalan lagi.
func staffLabel(prefix, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(name), "kasir") {
		return name
	}
	return prefix + " " + name
}

func joinDetail(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// loadPOSLedgerInfo membaca nomor struk, meja, pelanggan, dan kasir untuk ID
// order/transaksi POS. Tiga kueri ber-indeks untuk seluruh periode.
func loadPOSLedgerInfo(ids []string) map[string]posLedgerInfo {
	out := map[string]posLedgerInfo{}
	if len(ids) == 0 {
		return out
	}
	orders := map[string]posOrderRow{} // kunci: id dan local_id
	readOrders := func(query string, arg []string) {
		rows, err := database.DB.Query(query, pq.Array(arg))
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var o posOrderRow
			if rows.Scan(&o.id, &o.localID, &o.table, &o.customer, &o.creator) == nil {
				o.id = strings.TrimSpace(o.id)
				orders[o.id] = o
				if o.localID != "" {
					orders[o.localID] = o
				}
			}
		}
	}
	const orderCols = `SELECT TRIM(id), COALESCE(local_id, ''), COALESCE(table_number, ''), COALESCE(customer_name, ''),
		COALESCE(NULLIF(created_by, ''), orderer_name, '') FROM cloud_orders`
	readOrders(orderCols+` WHERE id = ANY($1::text[])`, ids)

	type txRow struct{ cashier, orderRef string }
	txs := map[string]txRow{}
	missing := []string{}
	if rows, err := database.DB.Query(`SELECT TRIM(id), COALESCE(cashier_name, ''), COALESCE(order_id, '')
		FROM cloud_transactions WHERE id = ANY($1::text[])`, pq.Array(ids)); err == nil {
		for rows.Next() {
			var id string
			var t txRow
			if rows.Scan(&id, &t.cashier, &t.orderRef) == nil {
				txs[strings.TrimSpace(id)] = t
				if ref := strings.TrimSpace(t.orderRef); ref != "" {
					if _, ok := orders[ref]; !ok {
						missing = append(missing, ref)
					}
				}
			}
		}
		rows.Close()
	}
	if len(missing) > 0 {
		readOrders(orderCols+` WHERE local_id = ANY($1::text[]) OR id = ANY($1::text[])`, missing)
	}

	for _, id := range ids {
		if t, ok := txs[id]; ok {
			o := orders[strings.TrimSpace(t.orderRef)]
			info := posLedgerInfo{receipt: posReceiptNo(o.localID), table: o.table, customer: o.customer}
			info.staff = staffLabel("Kasir", t.cashier)
			out[id] = info
			continue
		}
		if o, ok := orders[id]; ok {
			info := posLedgerInfo{receipt: posReceiptNo(o.localID), table: o.table, customer: o.customer}
			info.staff = staffLabel("oleh", o.creator)
			out[id] = info
		}
	}
	return out
}

// financeLedgerDetails: rincian ringkas & keterangan rapi per operasi (indeks
// sejajar dengan ops). Keterangan selain POS tidak diubah.
func financeLedgerDetails(ops []models.FinanceOperation) ([]string, []string) {
	details := make([]string, len(ops))
	descs := make([]string, len(ops))
	ids := []string{}
	seen := map[string]bool{}
	for _, o := range ops {
		if id := strings.TrimSpace(o.SourceID); financePOSLedgerSources[o.Source] && id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	pos := loadPOSLedgerInfo(ids)
	for i, o := range ops {
		desc := o.Description
		ref := strings.TrimSpace(o.Reference)
		if financePOSLedgerSources[o.Source] {
			if ref != "" && strings.HasSuffix(desc, ": "+ref) {
				desc = strings.TrimSuffix(desc, ": "+ref)
			}
			info := pos[strings.TrimSpace(o.SourceID)]
			customer := info.customer
			if customer == "" {
				customer = o.Counterparty
			}
			staff := info.staff
			if staff == "" {
				staff = staffLabel("Kasir", o.Actor)
			}
			details[i] = joinDetail(info.receipt, posTableLabel(info.table), customer, staff, ledgerMethodLabel(o.Method))
			descs[i] = desc
			continue
		}
		refPart := ""
		if ref != "" && !isOpaqueID(ref) && !strings.Contains(desc, ref) {
			refPart = ref
		}
		party := strings.TrimSpace(o.Counterparty)
		if party != "" && strings.Contains(desc, party) {
			party = ""
		}
		actor := ""
		if o.Source == "procurement_payment" && strings.TrimSpace(o.Actor) != "" {
			actor = "dibayar " + strings.TrimSpace(o.Actor)
		}
		details[i] = joinDetail(refPart, party, actor, ledgerMethodLabel(o.Method))
		descs[i] = desc
	}
	return details, descs
}

// ledgerEntryText: teks Keterangan Excel Buku Besar — keterangan, lalu
// "outlet · rincian", lalu catatan bila tidak mengulang keterangan.
func ledgerEntryText(e models.GeneralLedgerEntry) string {
	detail := e.Detail
	if detail == "" && e.Reference != "" && !isOpaqueID(e.Reference) && !strings.Contains(e.Description, e.Reference) {
		detail = e.Reference
	}
	text := e.Description
	if meta := joinDetail(e.OutletName, detail); meta != "" {
		text += "\n" + meta
	}
	if n := strings.TrimSpace(e.Notes); n != "" && !strings.Contains(e.Description, n) {
		text += "\n" + n
	}
	return text
}
