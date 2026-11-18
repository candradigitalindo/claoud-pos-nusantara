package services

import (
	"strings"
	"unicode/utf8"

	"cloud-pos/models"
)

// resolvePaymentDestination preserves the vendor snapshot unless the payer
// explicitly selects a marketplace VA. A VA is supplied for every payment,
// including installments; it is never written back to the vendor master.
func resolvePaymentDestination(d paymentDest, input models.UpdatePurchaseStatusInput) (paymentDest, error) {
	d.vendorName = strings.TrimSpace(d.vendorName)
	d.bank, d.number, d.holder = strings.TrimSpace(d.bank), strings.TrimSpace(d.number), strings.TrimSpace(d.holder)
	switch strings.TrimSpace(input.PaymentMethod) {
	case "marketplace_va":
		d.method = "marketplace_va"
		d.marketplace = strings.TrimSpace(input.PaymentMarketplace)
		d.orderNumber = strings.TrimSpace(input.PaymentOrderNumber)
		d.bank = strings.TrimSpace(input.PaymentBankName)
		d.number = strings.TrimSpace(input.PaymentAccountNumber)
		d.holder = strings.TrimSpace(input.PaymentAccountHolder)
		if d.marketplace == "" || d.orderNumber == "" || d.bank == "" || d.number == "" {
			return paymentDest{}, Invalid("marketplace, nomor pesanan, bank, dan nomor virtual account wajib diisi untuk pembayaran marketplace")
		}
		if utf8.RuneCountInString(d.marketplace) > 100 || utf8.RuneCountInString(d.orderNumber) > 100 ||
			utf8.RuneCountInString(d.bank) > 100 || len(d.number) > 50 || utf8.RuneCountInString(d.holder) > 200 {
			return paymentDest{}, Invalid("data pembayaran marketplace terlalu panjang")
		}
		for _, c := range d.number {
			if c < '0' || c > '9' {
				return paymentDest{}, Invalid("nomor virtual account harus berisi angka")
			}
		}
		d.text = "VA " + formatAccountDest(d.bank, d.number, d.holder)
		d.manual = true
	case "", "vendor_account":
		d.method = "vendor_account"
		if d.number != "" {
			d.text = formatAccountDest(d.bank, d.number, d.holder)
			return d, nil
		}
		d.method, d.manual = "manual", true
		d.bank, d.holder = "", ""
		d.text = strings.TrimSpace(input.PaymentAccountDest)
		if d.text == "" {
			return paymentDest{}, Invalid("vendor %s belum punya rekening terdaftar — isi rekening tujuan atau cara bayarnya (mis. Tunai), atau pilih Virtual Account Marketplace", d.vendorName)
		}
	case "outlet_cash":
		// Tunai dari laci kasir outlet lewat tugas "Bayar Pengadaan". Hanya
		// jalur tugas (CashPayoutID tidak bisa diisi dari body) yang boleh
		// memakai metode ini — Kantor Pusat tidak bisa mencatatnya manual.
		if strings.TrimSpace(input.CashPayoutID) == "" {
			return paymentDest{}, Invalid("metode Kas Kasir Outlet hanya bisa lewat tugas penyerahan kas di tablet kasir")
		}
		d.method, d.manual = "outlet_cash", true
		d.bank, d.number, d.holder = "", "", ""
		d.text = strings.TrimSpace(input.PaymentAccountDest)
		if d.text == "" {
			d.text = "Tunai — Kas Kasir Outlet"
		}
	default:
		return paymentDest{}, Invalid("metode pembayaran tidak valid")
	}
	if utf8.RuneCountInString(d.text) > 300 {
		return paymentDest{}, Invalid("rekening tujuan / cara bayar maksimal 300 karakter")
	}
	return d, nil
}
