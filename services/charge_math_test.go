package services

import (
	"math"
	"testing"

	"cloud-pos/models"
)

// Rumus biaya tambahan sisi cloud.
//
// sharedCases di bawah adalah KEMBARAN PERSIS dari tabel di
// `app-pos-flutter/test/charge_math_test.dart`. Cloud memakai rumus ini untuk
// menentukan nominal QRIS dinamis pesanan online, sementara POS memakai
// kembarannya untuk menagih di kasir. Kalau salah satu diubah tanpa yang lain,
// salah satu tabel gagal — itulah gunanya duplikasi yang disengaja ini.
//
// Bila menambah kasus di sini, TAMBAHKAN JUGA di berkas Dart-nya.

type chargeSpec struct {
	Type  string // "percentage" | "fixed"
	Value float64
}

type chargeCase struct {
	Name     string
	Subtotal float64
	Discount float64
	Charges  []chargeSpec
	Expected float64
}

var sharedCases = []chargeCase{
	{"tanpa biaya", 100000, 0, nil, 100000},

	{"pajak persentase saja (PB1 10%)", 100000, 0,
		[]chargeSpec{{"percentage", 10}}, 110000},

	{"biaya tetap saja", 100000, 0,
		[]chargeSpec{{"fixed", 5000}}, 105000},

	// Dua persentase dihitung dari SUBTOTAL, bukan berantai. Kalau berantai,
	// hasilnya 115500 — angka itulah yang ditolak kasus ini.
	{"pajak 10% + service 5% tidak berantai", 100000, 0,
		[]chargeSpec{{"percentage", 10}, {"percentage", 5}}, 115000},

	{"campuran persentase dan tetap", 100000, 0,
		[]chargeSpec{{"percentage", 10}, {"percentage", 5}, {"fixed", 2000}}, 117000},

	// Basis pengenaan = subtotal setelah diskon (DPP PB1/PPN).
	{"diskon menurunkan basis pajak", 200000, 50000,
		[]chargeSpec{{"percentage", 10}}, 165000},

	// Diskon 100% → basis 0 → tidak ada pajak DAN tidak ada biaya tetap.
	{"diskon penuh menihilkan seluruh biaya", 200000, 200000,
		[]chargeSpec{{"percentage", 10}, {"fixed", 5000}}, 0},

	{"subtotal nol", 0, 0,
		[]chargeSpec{{"percentage", 10}, {"fixed", 5000}}, 0},

	{"diskon melebihi subtotal", 50000, 80000,
		[]chargeSpec{{"percentage", 10}}, 0},

	// Pecahan: memastikan tidak ada pembulatan diam-diam di salah satu sisi.
	{"nominal berpecahan", 33333, 0,
		[]chargeSpec{{"percentage", 10}}, 36666.3},
}

func toCloudCharges(specs []chargeSpec) []models.CloudAdditionalCharge {
	out := make([]models.CloudAdditionalCharge, 0, len(specs))
	for i, s := range specs {
		out = append(out, models.CloudAdditionalCharge{
			LocalID:    string(rune('a' + i)),
			Name:       string(rune('a' + i)),
			ChargeType: s.Type,
			Value:      s.Value,
		})
	}
	return out
}

// basisSetelahDiskon mencerminkan ChargeMath.base di POS. Pesanan online tidak
// mengenal diskon manual, jadi CalculateOrderTotal sendiri menerima basis yang
// sudah bersih — perhitungan diskonnya diuji di sini agar tabelnya tetap bisa
// dibandingkan satu lawan satu dengan sisi Dart.
func basisSetelahDiskon(subtotal, discount float64) float64 {
	net := subtotal - discount
	if net < 0 {
		return 0
	}
	return net
}

func TestSharedChargeCases(t *testing.T) {
	for _, c := range sharedCases {
		t.Run(c.Name, func(t *testing.T) {
			base := basisSetelahDiskon(c.Subtotal, c.Discount)
			got, _ := CalculateOrderTotal(base, toCloudCharges(c.Charges))
			if math.Abs(got-c.Expected) > 0.0001 {
				t.Errorf("total = %v, mau %v\nBila kasus ini berubah, ubah juga charge_math_test.dart",
					got, c.Expected)
			}
		})
	}
}

func TestUrutanBiayaTidakMemengaruhiTotal(t *testing.T) {
	// POS membaca biaya tanpa ORDER BY (urut rowid), cloud mengurutkan per
	// nama. Itu aman HANYA selama biaya tidak berantai. Kalau suatu saat
	// rumusnya dibuat berantai, kasus ini gagal dan memaksa kedua sisi
	// disepakati ulang alih-alih diam-diam berbeda.
	const subtotal = 137500.0
	specs := []chargeSpec{{"percentage", 11}, {"fixed", 3000}, {"percentage", 6}}
	reversed := []chargeSpec{{"percentage", 6}, {"fixed", 3000}, {"percentage", 11}}

	maju, _ := CalculateOrderTotal(subtotal, toCloudCharges(specs))
	mundur, _ := CalculateOrderTotal(subtotal, toCloudCharges(reversed))
	if math.Abs(maju-mundur) > 0.0001 {
		t.Errorf("urutan mengubah total: %v vs %v", maju, mundur)
	}
}

func TestRincianBiayaMenjumlahKeTotal(t *testing.T) {
	// Rincian yang ditampilkan ke tamu harus benar-benar menjumlah ke nominal
	// yang ditagih. Kalau tidak, tamu melihat angka yang tak bisa ia cocokkan.
	const subtotal = 100000.0
	specs := []chargeSpec{{"percentage", 10}, {"percentage", 5}, {"fixed", 2000}}

	total, lines := CalculateOrderTotal(subtotal, toCloudCharges(specs))
	if len(lines) != len(specs) {
		t.Fatalf("rincian = %d baris, mau %d", len(lines), len(specs))
	}
	sum := subtotal
	for _, l := range lines {
		sum += l.Amount
	}
	if math.Abs(sum-total) > 0.0001 {
		t.Errorf("subtotal + rincian = %v, tapi total = %v", sum, total)
	}
}

func TestBiayaTetapNihilSaatBasisNol(t *testing.T) {
	// Bukan detail sepele: kalau biaya tetap tetap dikenakan, tagihan yang
	// sudah digratiskan masih menyisakan angka yang harus dibayar tamu.
	total, lines := CalculateOrderTotal(0, toCloudCharges([]chargeSpec{{"fixed", 5000}}))
	if total != 0 {
		t.Errorf("total = %v, mau 0", total)
	}
	if len(lines) != 1 || lines[0].Amount != 0 {
		t.Errorf("rincian biaya tetap = %+v, mau bernominal 0", lines)
	}
}
