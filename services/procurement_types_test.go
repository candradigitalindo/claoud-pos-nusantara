package services

// Uji tanpa database untuk izin pengadaan per jenis pengajuan (barang/jasa
// → procurement.requests.*, bahan baku → procurement.rawmaterials.*).

import (
	"reflect"
	"strings"
	"testing"
)

func TestProcurementPermissionMapping(t *testing.T) {
	cases := []struct{ rt, action, want string }{
		{"barang", "view", "procurement.requests.view"},
		{"jasa", "approve", "procurement.requests.approve"},
		{"bahan", "submit", "procurement.rawmaterials.submit"},
		{"bahan", "purchasing", "procurement.rawmaterials.purchasing"},
		// Aksi baru (mis. verifikasi) cukup ditambahkan sebagai nama aksi.
		{"bahan", "verify", "procurement.rawmaterials.verify"},
		{"barang", "verify", "procurement.requests.verify"},
	}
	for _, c := range cases {
		if got := ProcurementPermission(c.rt, c.action); got != c.want {
			t.Errorf("ProcurementPermission(%q,%q) = %q, mau %q", c.rt, c.action, got, c.want)
		}
	}
}

func TestMapProcurementPermission(t *testing.T) {
	if got := MapProcurementPermission("procurement.requests.approve", "bahan"); got != "procurement.rawmaterials.approve" {
		t.Errorf("approve bahan = %q", got)
	}
	if got := MapProcurementPermission("procurement.requests.approve", "barang"); got != "procurement.requests.approve" {
		t.Errorf("approve barang = %q", got)
	}
	// Izin di luar pengadaan per jenis tidak diterjemahkan.
	if got := MapProcurementPermission("finance.payments.pay", "bahan"); got != "finance.payments.pay" {
		t.Errorf("finance.payments.pay bahan = %q", got)
	}
}

func TestPermitsProcurementPrefixView(t *testing.T) {
	// Memegang aksi apa pun di modul berarti boleh melihat modul itu.
	onlySubmitBahan := []string{"procurement.rawmaterials.submit"}
	if !PermitsProcurement(onlySubmitBahan, "bahan", "view") {
		t.Error("rawmaterials.submit harus memenuhi syarat lihat bahan")
	}
	if PermitsProcurement(onlySubmitBahan, "barang", "view") {
		t.Error("izin bahan tidak boleh membuka pengadaan barang")
	}
	if PermitsProcurement(onlySubmitBahan, "bahan", "approve") {
		t.Error("submit tidak boleh memenuhi approve")
	}
	// Prefix hanya untuk aksi view.
	if PermitsProcurement([]string{"procurement.requests.view"}, "jasa", "submit") {
		t.Error("view tidak boleh memenuhi submit")
	}
	onlyBarang := []string{"procurement.requests.approve"}
	if PermitsProcurement(onlyBarang, "bahan", "approve") {
		t.Error("procurement.requests.* tidak lagi mengatur pengajuan bahan")
	}
	if !PermitsProcurement(onlyBarang, "barang", "view") || !PermitsProcurement(onlyBarang, "jasa", "approve") {
		t.Error("procurement.requests.approve harus berlaku untuk barang & jasa")
	}
	// Awalan yang mirip tapi bukan modul yang sama tidak dihitung.
	if PermitsProcurement([]string{"procurement.requestsx.view"}, "barang", "view") {
		t.Error("awalan harus diakhiri titik")
	}
}

func TestViewableRequestTypes(t *testing.T) {
	cases := []struct {
		perms []string
		want  []string
	}{
		{nil, []string{}},
		{[]string{"finance.payments.view"}, []string{}},
		{[]string{"procurement.requests.view"}, []string{"barang", "jasa"}},
		{[]string{"procurement.rawmaterials.purchasing"}, []string{"bahan"}},
		{[]string{"procurement.requests.submit", "procurement.rawmaterials.view"}, []string{"barang", "jasa", "bahan"}},
	}
	for _, c := range cases {
		got := ViewableRequestTypes(c.perms)
		if got == nil {
			t.Fatalf("ViewableRequestTypes(%v) = nil, harus slice non-nil", c.perms)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ViewableRequestTypes(%v) = %v, mau %v", c.perms, got, c.want)
		}
	}
}

func TestIsValidRequestTypeAndLabel(t *testing.T) {
	for _, rt := range []string{"barang", "jasa", "bahan"} {
		if !IsValidRequestType(rt) {
			t.Errorf("%q harus sah", rt)
		}
	}
	for _, rt := range []string{"", "Bahan", "modal", "bahan baku"} {
		if IsValidRequestType(rt) {
			t.Errorf("%q tidak boleh sah", rt)
		}
	}
	if RequestTypeLabel("bahan") != "Bahan Baku" || RequestTypeLabel("barang") != "Barang" || RequestTypeLabel("jasa") != "Jasa" {
		t.Error("label jenis tidak sesuai")
	}
}

func TestProcurementClassExprBahanBranch(t *testing.T) {
	expr := procurementClassExpr("pr")
	branch := "WHEN pr.request_type = 'bahan' THEN 'bahan'"
	i := strings.Index(expr, branch)
	if i < 0 {
		t.Fatalf("procurementClassExpr tidak memuat cabang bahan:\n%s", expr)
	}
	// Projek tetap menang lebih dulu; bahan harus diperiksa sebelum
	// goods_kind supaya tidak pernah jatuh ke kelas 'modal'.
	if p := strings.Index(expr, "project_id IS NOT NULL"); p < 0 || p > i {
		t.Error("cabang projek harus mendahului cabang bahan")
	}
	if g := strings.Index(expr, "goods_kind"); g < 0 || g < i {
		t.Error("cabang bahan harus mendahului goods_kind")
	}
}
