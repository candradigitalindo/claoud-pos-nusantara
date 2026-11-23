package services

import (
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Tujuan pembayaran pengadaan diisi bagian Pembelian (keputusan owner 19 Nov
// 2026): Pembelian yang memesan ke vendor/marketplace dan tahu cara bayarnya.
// Keuangan hanya membaca tujuan itu (dengan tombol salin), memilih sumber dana
// & rekening asal, lalu mencatat transfernya; bila tujuannya salah, Keuangan
// mengembalikannya ke Pembelian dengan alasan.
//
// Aturan 14 Okt 2026 tetap berlaku: transfer selalu ke rekening vendor
// TERDAFTAR yang dibaca dari data vendor saat dibayar — tidak pernah diketik
// bebas. "Tunai / cara bayar lain" hanya untuk vendor yang belum punya
// rekening terdaftar; VA marketplace milik satu pesanan, bukan milik vendor.

const (
	PayToVendorAccount = "vendor_account"
	PayToMarketplaceVA = "marketplace_va"
	PayToManual        = "manual"
)

// payToStored: kolom pay_to_* seperti tersimpan.
type payToStored struct {
	method, marketplace, orderNumber, bank, number, holder, text, note string
	setBy, returnedBy, returnReason                                    string
	setAt, returnedAt                                                  *time.Time
}

// vendorAccount: rekening terdaftar vendor saat ini.
type vendorAccount struct{ bank, number, holder string }

func (v vendorAccount) registered() bool { return strings.TrimSpace(v.number) != "" }

// buildPayTo menurunkan tujuan yang berlaku dari isian Pembelian dan rekening
// vendor saat ini. Satu-satunya rumus — dipakai tampilan (GET) dan pembayaran,
// sehingga yang dibayar Keuangan selalu yang ia lihat.
func buildPayTo(st payToStored, vendorID string, v vendorAccount) models.PurchasePayTo {
	p := models.PurchasePayTo{
		Method: st.method, Marketplace: st.marketplace, OrderNumber: st.orderNumber,
		Text: st.text, Note: st.note, SetBy: st.setBy, SetAt: fmtTimePtr(st.setAt),
		ReturnedAt: fmtTimePtr(st.returnedAt), ReturnedBy: st.returnedBy, ReturnReason: st.returnReason,
		VendorBankName: v.bank, VendorAccountNumber: v.number, VendorAccountHolder: v.holder,
	}
	switch st.method {
	case PayToMarketplaceVA:
		p.Effective = PayToMarketplaceVA
		p.BankName, p.AccountNumber, p.AccountHolder = st.bank, st.number, st.holder
	case PayToManual:
		p.Effective = PayToManual
	case PayToVendorAccount, "":
		// Data lama tanpa isian: rekening vendor terdaftar bila ada.
		if v.registered() {
			p.Effective = PayToVendorAccount
			p.BankName, p.AccountNumber, p.AccountHolder = v.bank, v.number, v.holder
		} else if strings.TrimSpace(st.bank) != "" && strings.TrimSpace(st.number) != "" {
			// Pembayaran manual di luar pengajuan dengan rekening penerima langsung.
			p.Effective = PayToVendorAccount
			p.BankName, p.AccountNumber, p.AccountHolder = st.bank, st.number, st.holder
		}
	}
	switch {
	case st.returnedAt != nil:
		p.Problem = "detail pembayaran dikembalikan ke Pembelian: " + st.returnReason
	case st.method == PayToManual && strings.TrimSpace(st.text) != "":
		// Pembayaran tunai/kas kasir yang sudah memiliki tujuan jelas (termasuk
		// yang diarahkan langsung oleh penyetuju ke Kas Kasir Outlet).
	case (p.Effective == PayToVendorAccount || p.Effective == PayToMarketplaceVA) && strings.TrimSpace(p.BankName) != "" && strings.TrimSpace(p.AccountNumber) != "":
		// Rekening bank atau VA tujuan sudah lengkap.
	case strings.TrimSpace(vendorID) == "":
		p.Problem = "pengajuan ini belum memilih vendor dari daftar vendor"
	case p.Effective == "" && st.method == PayToVendorAccount:
		p.Problem = "rekening terdaftar vendor ini kosong — Pembelian perlu memperbarui detail pembayaran atau data vendor"
	case p.Effective == "":
		p.Problem = "Pembelian belum mengisi detail pembayaran (vendor belum punya rekening terdaftar)"
	}
	p.Ready = p.Problem == ""
	return p
}

// resolvePayToInput memeriksa dan merapikan isian Pembelian terhadap rekening
// vendor terdaftar saat ini. nil / metode kosong = rekening vendor terdaftar.
func resolvePayToInput(vendorName string, v vendorAccount, in *models.PurchasePaymentDetailsInput) (payToStored, error) {
	var d models.PurchasePaymentDetailsInput
	if in != nil {
		d = *in
	}
	method := strings.TrimSpace(d.Method)
	note := strings.TrimSpace(d.Note)
	if utf8.RuneCountInString(note) > 1000 {
		return payToStored{}, Invalid("catatan untuk Keuangan maksimal 1000 karakter")
	}
	if method == "" {
		if !v.registered() {
			return payToStored{}, Invalid("vendor %s belum punya rekening terdaftar — pilih cara bayarnya: Virtual Account Marketplace atau Tunai / cara bayar lain", vendorName)
		}
		method = PayToVendorAccount
	}
	st := payToStored{method: method, note: note}
	switch method {
	case PayToVendorAccount:
		if !v.registered() {
			return payToStored{}, Invalid("vendor %s belum punya rekening terdaftar — lengkapi rekeningnya di data Vendor, atau pilih Virtual Account Marketplace / Tunai", vendorName)
		}
	case PayToMarketplaceVA:
		dest, err := resolvePaymentDestination(paymentDest{vendorName: vendorName}, models.UpdatePurchaseStatusInput{
			PaymentMethod: PayToMarketplaceVA, PaymentMarketplace: d.Marketplace, PaymentOrderNumber: d.OrderNumber,
			PaymentBankName: d.BankName, PaymentAccountNumber: d.AccountNumber, PaymentAccountHolder: d.AccountHolder,
		})
		if err != nil {
			return payToStored{}, err
		}
		st.marketplace, st.orderNumber = dest.marketplace, dest.orderNumber
		st.bank, st.number, st.holder = dest.bank, dest.number, dest.holder
	case PayToManual:
		if v.registered() {
			return payToStored{}, Invalid("vendor %s punya rekening terdaftar — pembayarannya memakai rekening itu. Bila harus tunai, Keuangan bisa memilih sumber dana Kas Kasir Outlet", vendorName)
		}
		st.text = strings.TrimSpace(d.Text)
		if st.text == "" {
			return payToStored{}, Invalid("tulis cara bayarnya, mis. \"Tunai saat barang diantar\"")
		}
		if utf8.RuneCountInString(st.text) > 300 {
			return payToStored{}, Invalid("cara bayar maksimal 300 karakter")
		}
	default:
		return payToStored{}, Invalid("cara bayar tidak valid")
	}
	return st, nil
}

// prVendorAccountTx: vendor terdaftar pengajuan beserta rekeningnya saat ini
// (dikunci FOR SHARE supaya tidak berganti di tengah transaksi). vendorID ""
// bila pengajuan belum memilih vendor.
func prVendorAccountTx(tx *sql.Tx, prID string) (vendorID, vendorName string, acct vendorAccount, err error) {
	if err = tx.QueryRow(`SELECT COALESCE(vendor_id, ''), COALESCE(vendor_name, '') FROM purchase_requests WHERE id = $1`, prID).Scan(&vendorID, &vendorName); err != nil {
		return "", "", vendorAccount{}, err
	}
	vendorID = strings.TrimSpace(vendorID)
	vendorName = strings.TrimSpace(vendorName)
	if vendorID == "" {
		return "", vendorName, vendorAccount{}, nil
	}
	err = tx.QueryRow(`
		SELECT name, COALESCE(bank_name, ''), COALESCE(account_number, ''), COALESCE(account_holder, '')
		FROM vendors WHERE id = $1 FOR SHARE`, vendorID).Scan(&vendorName, &acct.bank, &acct.number, &acct.holder)
	if err == sql.ErrNoRows {
		return vendorID, "", vendorAccount{}, Invalid("vendor pengajuan ini sudah tidak ada — pilih vendor dari daftar dulu")
	}
	return vendorID, vendorName, acct, err
}

// loadPayToTx membaca isian tujuan pembayaran (baris sudah dikunci pemanggil).
func loadPayToTx(tx *sql.Tx, prID string) (payToStored, error) {
	var st payToStored
	err := tx.QueryRow(`
		SELECT pay_to_method, pay_to_marketplace, pay_to_order_number, pay_to_bank_name,
		       pay_to_account_number, pay_to_account_holder, pay_to_text, pay_to_note,
		       pay_to_set_by, pay_to_set_at, pay_to_returned_at, pay_to_returned_by, pay_to_return_reason
		FROM purchase_requests WHERE id = $1`, prID).Scan(
		&st.method, &st.marketplace, &st.orderNumber, &st.bank,
		&st.number, &st.holder, &st.text, &st.note,
		&st.setBy, &st.setAt, &st.returnedAt, &st.returnedBy, &st.returnReason)
	return st, err
}

// payToFromInputTx: isian Pembelian yang sudah diperiksa terhadap vendor
// pengajuan ini.
func payToFromInputTx(tx *sql.Tx, prID string, in *models.PurchasePaymentDetailsInput) (payToStored, error) {
	vendorID, vendorName, acct, err := prVendorAccountTx(tx, prID)
	if err != nil {
		return payToStored{}, err
	}
	if vendorID == "" {
		return payToStored{}, Invalid("pengajuan ini belum memilih vendor dari daftar vendor — pilih vendornya lewat Isi Harga dulu")
	}
	return resolvePayToInput(vendorName, acct, in)
}

// payToDestTx: tujuan pembayaran Kantor Pusat untuk pengajuan yang sudah
// dikunci pemanggil — dari isian Pembelian (atau rekening vendor terdaftar
// untuk data lama), TIDAK pernah dari isian Keuangan.
func payToDestTx(tx *sql.Tx, prID string) (paymentDest, error) {
	st, err := loadPayToTx(tx, prID)
	if err != nil {
		return paymentDest{}, err
	}
	vendorID, vendorName, acct, err := prVendorAccountTx(tx, prID)
	if err != nil {
		return paymentDest{}, err
	}
	p := buildPayTo(st, vendorID, acct)
	if !p.Ready {
		return paymentDest{}, Invalid("belum bisa dibayar: %s", p.Problem)
	}
	d := paymentDest{vendorID: vendorID, vendorName: vendorName}
	switch p.Effective {
	case PayToVendorAccount:
		d.method = PayToVendorAccount
		d.bank, d.number, d.holder = p.BankName, p.AccountNumber, p.AccountHolder
		d.text = formatAccountDest(d.bank, d.number, d.holder)
	case PayToMarketplaceVA:
		d.method, d.manual = PayToMarketplaceVA, true
		d.marketplace, d.orderNumber = p.Marketplace, p.OrderNumber
		d.bank, d.number, d.holder = p.BankName, p.AccountNumber, p.AccountHolder
		d.text = "VA " + formatAccountDest(d.bank, d.number, d.holder)
	case PayToManual:
		d.method, d.manual = PayToManual, true
		d.text = p.Text
	}
	return d, nil
}

// payToForPRTx: tujuan yang berlaku untuk satu pengajuan (dalam transaksi).
func payToForPRTx(tx *sql.Tx, prID string) (models.PurchasePayTo, error) {
	st, err := loadPayToTx(tx, prID)
	if err != nil {
		return models.PurchasePayTo{}, err
	}
	vendorID, _, acct, err := prVendorAccountTx(tx, prID)
	if err != nil {
		return models.PurchasePayTo{}, err
	}
	return buildPayTo(st, vendorID, acct), nil
}

// payToEditableTx: detail pembayaran hanya bisa diubah / dikembalikan selama
// pengajuan menunggu pembayaran, masih bertagihan sendiri, dan tidak sedang
// dibayar dari Kas Kasir Outlet.
func payToEditableTx(tx *sql.Tx, cur *lockedPurchase) error {
	if _, ok := validTransitions["pay"][cur.Status]; !ok {
		return Invalid("detail pembayaran hanya bisa diubah saat pengajuan menunggu pembayaran")
	}
	if cur.SplitStatus == "master" && !hasOwnPurchaseItems(cur.ItemsJSON) {
		return Invalid("pengajuan induk ini tidak punya tagihan sendiri — detail pembayaran diatur di pecahannya")
	}
	var totalFinal, paid float64
	var emergency bool
	if err := tx.QueryRow(`SELECT COALESCE(total_final, 0), COALESCE(paid_amount, 0), emergency FROM purchase_requests WHERE id = $1`,
		cur.ID).Scan(&totalFinal, &paid, &emergency); err != nil {
		return err
	}
	if emergency {
		return Invalid("Pembelian Darurat dibayar tunai dari Kas Kasir Outlet lewat tablet kasir — tidak memakai detail pembayaran")
	}
	if cents(totalFinal)-cents(paid) <= 0 {
		return Invalid("tagihan pengajuan ini sudah lunas")
	}
	var open bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM procurement_cash_payouts
		WHERE purchase_request_id = $1 AND status IN ('pending','claimed'))`, cur.ID).Scan(&open); err != nil {
		return err
	}
	if open {
		return Invalid("pengajuan ini sedang dibayar dari Kas Kasir Outlet — minta Keuangan menarik tugasnya dulu")
	}
	return nil
}

// UpdatePurchasePaymentDetails: bagian Pembelian mengubah tujuan pembayaran
// selama tagihan belum lunas (mis. VA kedaluwarsa, cicilan dengan VA baru),
// sekaligus menjawab pengembalian dari Keuangan. Version wajib: Keuangan yang
// membuka form Bayar sebelum perubahan ini akan ditolak 409 saat konfirmasi.
func UpdatePurchasePaymentDetails(id string, in models.PurchasePaymentDetailsInput) (*models.PurchaseRequest, error) {
	if strings.TrimSpace(in.Version) == "" {
		return nil, Invalid("version wajib dikirim — muat ulang pengajuan lalu coba lagi")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cur, err := lockPurchaseTx(tx, id)
	if err != nil {
		return nil, Invalid("pengajuan tidak ditemukan")
	}
	if !versionMatches(in.Version, cur.UpdatedAt) {
		return nil, Conflict(purchaseChangedMsg)
	}
	if err := payToEditableTx(tx, cur); err != nil {
		return nil, err
	}
	st, err := payToFromInputTx(tx, id, &in)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`
		UPDATE purchase_requests SET pay_to_method=$1, pay_to_marketplace=$2, pay_to_order_number=$3,
		       pay_to_bank_name=$4, pay_to_account_number=$5, pay_to_account_holder=$6, pay_to_text=$7, pay_to_note=$8,
		       pay_to_set_by=$9, pay_to_set_by_id=$10, pay_to_set_at=$11,
		       pay_to_returned_at=NULL, pay_to_returned_by='', pay_to_returned_by_id=NULL, pay_to_return_reason='',
		       updated_at=$11
		WHERE id=$12`,
		st.method, st.marketplace, st.orderNumber, st.bank, st.number, st.holder, st.text, st.note,
		in.ActorName, nilIfEmpty(validActorID(in.ActorID)), now, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	pr, err := GetPurchaseRequest(id)
	if err == nil {
		// Keuangan diberi tahu tagihan ini siap (lagi) diproses.
		go NotifyPurchaseEvent(pr, "payment_details", in.ActorName, "", 0)
	}
	return pr, err
}

// ReturnPurchasePaymentDetails: Keuangan mengembalikan detail pembayaran yang
// salah/kurang ke Pembelian dengan alasan. Pembayaran Kantor Pusat dan tugas
// Kas Kasir Outlet terkunci sampai Pembelian memperbaikinya.
func ReturnPurchasePaymentDetails(id string, in models.PurchasePaymentReturnInput) (*models.PurchaseRequest, error) {
	reason := strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(reason) < VerifyReasonMinLen {
		return nil, Invalid("alasan pengembalian wajib diisi (minimal %d huruf) supaya Pembelian tahu apa yang diperbaiki", VerifyReasonMinLen)
	}
	if utf8.RuneCountInString(reason) > 1000 {
		return nil, Invalid("alasan pengembalian maksimal 1000 karakter")
	}
	if strings.TrimSpace(in.Version) == "" {
		return nil, Invalid("version wajib dikirim — muat ulang pengajuan lalu coba lagi")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cur, err := lockPurchaseTx(tx, id)
	if err != nil {
		return nil, Invalid("pengajuan tidak ditemukan")
	}
	if !versionMatches(in.Version, cur.UpdatedAt) {
		return nil, Conflict(purchaseChangedMsg)
	}
	if err := payToEditableTx(tx, cur); err != nil {
		return nil, err
	}
	var returned bool
	if err := tx.QueryRow(`SELECT pay_to_returned_at IS NOT NULL FROM purchase_requests WHERE id = $1`, id).Scan(&returned); err != nil {
		return nil, err
	}
	if returned {
		return nil, Invalid("detail pembayaran pengajuan ini sudah dikembalikan ke Pembelian")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`
		UPDATE purchase_requests SET pay_to_returned_at=$1, pay_to_returned_by=$2, pay_to_returned_by_id=$3,
		       pay_to_return_reason=$4, updated_at=$1
		WHERE id=$5`,
		now, in.ActorName, nilIfEmpty(validActorID(in.ActorID)), reason, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	pr, err := GetPurchaseRequest(id)
	if err == nil {
		go NotifyPurchaseEvent(pr, "payment_returned", in.ActorName, reason, 0)
	}
	return pr, err
}
