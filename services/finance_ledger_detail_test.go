package services

import (
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func TestLedgerDetailHelpers(t *testing.T) {
	for s, want := range map[string]bool{
		"01M69EHZCPYMAPX8Z2MS2WKWHB": true, "01m69ehzcpymapx8z2ms2wkwhb": false, "191126004": false,
		"5b0c8a2e-1f2d-4c3b-9a8e-0123456789ab": true, "PAYROLL-SEP": false, "01M69EHZCPYMAPX8Z2MS2WKWHI": false,
	} {
		if got := isOpaqueID(s); got != want {
			t.Errorf("isOpaqueID(%q) = %v", s, got)
		}
	}
	if got := posTableLabel("9"); got != "Meja 9" {
		t.Errorf("meja angka: %q", got)
	}
	if got := posTableLabel("TERAS 1"); got != "TERAS 1" {
		t.Errorf("meja nama: %q", got)
	}
	if got := staffLabel("Kasir", "Kasir") + "|" + staffLabel("oleh", "Kasir Bawah") + "|" + staffLabel("Kasir", "Zia"); got != "Kasir|Kasir Bawah|Kasir Zia" {
		t.Errorf("label staf: %q", got)
	}
	if got := posReceiptNo("01m7ssp8abcdef"); got != "TRX-01M7SSP8" {
		t.Errorf("nomor struk: %q", got)
	}
	if got := ledgerMethodLabel("cash") + "|" + ledgerMethodLabel("qris") + "|" + ledgerMethodLabel(ReservationDpMethod); got != "Tunai|QRIS|Uang muka reservasi" {
		t.Errorf("metode: %q", got)
	}
	e := models.GeneralLedgerEntry{Description: "Pengeluaran: solar genset", Notes: "solar genset", OutletName: "Kala", Reference: "01M69EHZCPYMAPX8Z2MS2WKWHB", Detail: "Pak Leo"}
	if got := ledgerEntryText(e); got != "Pengeluaran: solar genset\nKala · Pak Leo" {
		t.Errorf("teks Excel: %q", got)
	}
}

func TestLedgerDetailsReadableForPOSAndProcurement(t *testing.T) {
	requireDB(t)
	outlet := payoutTestOutlet(t)
	orderID, orderLocal := NewULID(), NewULID()
	txID, txLocal := NewULID(), NewULID()
	if _, err := database.DB.Exec(`INSERT INTO cloud_orders (id, local_id, outlet_id, outlet_code, total_amount, table_number, customer_name, created_by)
		VALUES ($1, $2, $3, 'KO', 55000, '9', 'Araaa', 'Zia')`, orderID, orderLocal, outlet); err != nil {
		t.Fatalf("order uji: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO cloud_transactions (id, local_id, outlet_id, outlet_code, total_amount, cashier_name, payment_method, order_id)
		VALUES ($1, $2, $3, 'KO', 55000, 'Zia', 'qris', $4)`, txID, txLocal, outlet, orderLocal); err != nil {
		t.Fatalf("transaksi uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_transactions WHERE id = $1`, txID)
		execScratch(`DELETE FROM cloud_orders WHERE id = $1`, orderID)
	})
	ops := []models.FinanceOperation{
		{Source: "sale", SourceID: txID, Reference: txLocal, Description: "Pembayaran POS: " + txLocal, Actor: "Zia", Method: "qris"},
		{Source: "receivable", SourceID: orderID, Reference: orderLocal, Description: "Pengakuan piutang POS: " + orderLocal},
		{Source: "procurement_payment", Reference: "191126004", Description: "Bayar: 191126004 · Healing — Nissa", Counterparty: "Vendor tes", Actor: "Cindy"},
		{Source: "pos_cash", Reference: NewULID(), Description: "Pengeluaran: penggajian orang taman", Counterparty: "Ibu Yuli"},
		{Source: "finance_entry", Reference: "PAYROLL-SEP", Description: "Gaji September", Counterparty: "Tim Outlet", Method: "cash"},
		{Source: "depreciation", Description: "Penyusutan aset 2026-11"},
	}
	details, descs := financeLedgerDetails(ops)
	receipt := "TRX-" + strings.ToUpper(orderLocal[:8])
	want := []struct{ detail, desc string }{
		{receipt + " · Meja 9 · Araaa · Kasir Zia · QRIS", "Pembayaran POS"},
		{receipt + " · Meja 9 · Araaa · oleh Zia", "Pengakuan piutang POS"},
		{"Vendor tes · dibayar Cindy", "Bayar: 191126004 · Healing — Nissa"},
		{"Ibu Yuli", "Pengeluaran: penggajian orang taman"},
		{"PAYROLL-SEP · Tim Outlet · Tunai", "Gaji September"},
		{"", "Penyusutan aset 2026-11"},
	}
	for i, w := range want {
		if details[i] != w.detail || descs[i] != w.desc {
			t.Errorf("op %d (%s): dapat (%q, %q), mau (%q, %q)", i, ops[i].Source, details[i], descs[i], w.detail, w.desc)
		}
	}
}
