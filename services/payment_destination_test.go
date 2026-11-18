package services

import (
	"strings"
	"testing"

	"cloud-pos/models"
)

func TestResolvePaymentDestination(t *testing.T) {
	vendor := paymentDest{vendorID: "vendor", vendorName: "Toko Uji", bank: "BCA", number: "111222", holder: "Toko Uji"}
	va := models.UpdatePurchaseStatusInput{
		PaymentMethod: "marketplace_va", PaymentMarketplace: " Shopee ", PaymentOrderNumber: " ORDER-001 ",
		PaymentBankName: " Mandiri ", PaymentAccountNumber: " 001234567890 ", PaymentAccountHolder: " Shopee Checkout ",
	}
	t.Run("explicit VA overrides registered account for this payment", func(t *testing.T) {
		d, err := resolvePaymentDestination(vendor, va)
		if err != nil {
			t.Fatal(err)
		}
		if d.method != "marketplace_va" || d.marketplace != "Shopee" || d.orderNumber != "ORDER-001" ||
			d.bank != "Mandiri" || d.number != "001234567890" || d.holder != "Shopee Checkout" ||
			d.text != "VA Mandiri 001234567890 a.n. Shopee Checkout" || !d.manual || d.vendorID != vendor.vendorID {
			t.Fatalf("wrong VA snapshot: %+v", d)
		}
		if vendor.number != "111222" || vendor.bank != "BCA" {
			t.Fatal("VA altered registered vendor destination")
		}
	})
	t.Run("legacy transfer ignores typed account", func(t *testing.T) {
		d, err := resolvePaymentDestination(vendor, models.UpdatePurchaseStatusInput{PaymentAccountDest: "Rekening lain", PaymentAccountNumber: "999"})
		if err != nil || d.method != "vendor_account" || d.text != "BCA 111222 a.n. Toko Uji" || d.manual {
			t.Fatalf("standard vendor snapshot: %+v, %v", d, err)
		}
	})
	t.Run("cash remains available without registered account", func(t *testing.T) {
		d, err := resolvePaymentDestination(paymentDest{vendorID: "vendor", vendorName: "Pasar"}, models.UpdatePurchaseStatusInput{PaymentAccountDest: " Tunai "})
		if err != nil || d.method != "manual" || d.text != "Tunai" || !d.manual {
			t.Fatalf("cash snapshot: %+v, %v", d, err)
		}
	})
	t.Run("each VA is independent", func(t *testing.T) {
		first, err := resolvePaymentDestination(vendor, va)
		if err != nil {
			t.Fatal(err)
		}
		va.PaymentAccountNumber, va.PaymentOrderNumber = "00000999", "ORDER-002"
		second, err := resolvePaymentDestination(vendor, va)
		if err != nil || first.number != "001234567890" || first.orderNumber != "ORDER-001" || second.number != "00000999" || second.orderNumber != "ORDER-002" {
			t.Fatalf("snapshots: first=%+v second=%+v err=%v", first, second, err)
		}
	})
}

func TestResolvePaymentDestinationRejectsIncompleteOrInvalidVA(t *testing.T) {
	valid := models.UpdatePurchaseStatusInput{
		PaymentMethod: "marketplace_va", PaymentMarketplace: "Tokopedia", PaymentOrderNumber: "INV/001",
		PaymentBankName: "BNI", PaymentAccountNumber: "000123456789",
	}
	cases := map[string]func(*models.UpdatePurchaseStatusInput){
		"marketplace required": func(in *models.UpdatePurchaseStatusInput) { in.PaymentMarketplace = " " },
		"order required":       func(in *models.UpdatePurchaseStatusInput) { in.PaymentOrderNumber = "" },
		"bank required":        func(in *models.UpdatePurchaseStatusInput) { in.PaymentBankName = "" },
		"VA required":          func(in *models.UpdatePurchaseStatusInput) { in.PaymentAccountNumber = "" },
		"VA digits only":       func(in *models.UpdatePurchaseStatusInput) { in.PaymentAccountNumber = "0012invalid" },
		"VA length":            func(in *models.UpdatePurchaseStatusInput) { in.PaymentAccountNumber = strings.Repeat("1", 51) },
		"marketplace length":   func(in *models.UpdatePurchaseStatusInput) { in.PaymentMarketplace = strings.Repeat("x", 101) },
		"order length":         func(in *models.UpdatePurchaseStatusInput) { in.PaymentOrderNumber = strings.Repeat("x", 101) },
		"destination length": func(in *models.UpdatePurchaseStatusInput) {
			in.PaymentBankName, in.PaymentAccountHolder = strings.Repeat("b", 100), strings.Repeat("h", 200)
		},
		"unknown method": func(in *models.UpdatePurchaseStatusInput) { in.PaymentMethod = "unknown" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := valid
			mutate(&in)
			if _, err := resolvePaymentDestination(paymentDest{}, in); err == nil {
				t.Fatal("expected invalid destination to be rejected")
			}
		})
	}
	if _, err := resolvePaymentDestination(paymentDest{vendorName: "Pasar"}, models.UpdatePurchaseStatusInput{}); err == nil {
		t.Fatal("manual payment without destination was accepted")
	}
}

func TestMarketplacePaymentSnapshotsPerTransactionWithoutChangingVendor(t *testing.T) {
	requireDB(t)
	sfx := reviewSuffix()
	v := snapshotVendor(t, "Marketplace Toko "+sfx, "BCA", "111222333", "Toko Uji")
	pr := snapshotBill(t, "Zzz-Uji-VA-"+sfx, 1000000, v.ID)
	for i, va := range []string{"001234567890", "009876543210"} {
		_, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
			Action: "pay", ActorName: "keuangan", PaymentAmount: 500000, PaymentMethod: "marketplace_va",
			PaymentMarketplace: "Tokopedia", PaymentOrderNumber: "ORDER-" + va,
			PaymentBankName: "Mandiri", PaymentAccountNumber: va, PaymentAccountHolder: "Tokopedia",
		})
		if err != nil {
			t.Fatalf("payment %d: %v", i+1, err)
		}
	}
	h, err := GetPaymentHistories(pr.ID)
	if err != nil || len(h) != 2 {
		t.Fatalf("histories: %+v, %v", h, err)
	}
	for i, va := range []string{"001234567890", "009876543210"} {
		if h[i].PaymentMethod != "marketplace_va" || h[i].PaymentMarketplace != "Tokopedia" ||
			h[i].PaymentOrderNumber != "ORDER-"+va || h[i].DestAccountNumber != va || h[i].VendorID != v.ID {
			t.Errorf("snapshot %d: %+v", i, h[i])
		}
	}
	registered, err := GetVendor(v.ID)
	if err != nil || registered.AccountNumber != "111222333" || registered.BankName != "BCA" {
		t.Fatalf("registered account changed: %+v, %v", registered, err)
	}
}
