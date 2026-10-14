package services

// Uji langkah VERIFIKASI pengadaan (pending → verified → approved) beserta
// pagar-pagarnya: tolak per item, version compare-and-set, approve hanya dari
// verified, isi terkunci setelah diverifikasi/disetujui, status berkomitmen
// (verified belum hutang), dan identitas pelaku yang disimpan sebagai akun.
//
// Bagian atas tanpa database; bagian bawah butuh Postgres (requireDB —
// TestMain ada di purchase_flow_test.go).

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// ── Tanpa database ─────────────────────────────────────────────────────────

func TestValidTransitionsVerificationFlow(t *testing.T) {
	kasus := []struct {
		action, from, to string // to "" = tidak boleh
	}{
		{"verify", "pending", "verified"},
		{"verify", "verified", ""},
		{"verify", "approved", ""},
		{"approve", "verified", "approved"},
		{"approve", "pending", ""}, // approve langsung dari pending ditolak (T-3)
		{"approve", "approved", ""},
		{"reject", "pending", "rejected"},
		{"reject", "verified", "rejected"},
		{"reject", "approved", ""},
		{"cancel", "pending", "cancelled"},
		{"cancel", "verified", "cancelled"},
		{"cancel", "approved", "cancelled"},
		{"cancel", "payment_requested", ""},
		{"request_payment", "approved", "payment_requested"},
		{"request_payment", "verified", ""},
	}
	for _, k := range kasus {
		got, ok := validTransitions[k.action][k.from]
		if k.to == "" {
			if ok {
				t.Errorf("%s dari %s seharusnya ditolak, dapat → %s", k.action, k.from, got)
			}
			continue
		}
		if !ok || got != k.to {
			t.Errorf("%s dari %s = %q (ok=%v), mau %q", k.action, k.from, got, ok, k.to)
		}
	}
}

func TestPurchaseActionPermissionByStage(t *testing.T) {
	kasus := []struct {
		action, status, want string
		known                bool
	}{
		{"verify", "pending", "verify", true},
		{"approve", "verified", "approve", true},
		// Tolak bergantung tahap.
		{"reject", "pending", "verify", true},
		{"reject", "verified", "approve", true},
		{"reject", "approved", "approve", true},
		{"request_payment", "approved", "purchasing", true},
		{"cancel", "verified", "submit", true},
		{"receive", "paid", "submit", true},
		{"pay", "payment_requested", "", true}, // finance.payments.pay
		{"hapus", "pending", "", false},
	}
	for _, k := range kasus {
		got, known := PurchaseActionPermission(k.action, k.status)
		if got != k.want || known != k.known {
			t.Errorf("%s @%s = (%q,%v), mau (%q,%v)", k.action, k.status, got, known, k.want, k.known)
		}
	}
}

func TestVerifyPermissionPerRequestType(t *testing.T) {
	if got := ProcurementPermission(RequestTypeBarang, "verify"); got != "procurement.requests.verify" {
		t.Errorf("barang verify = %q", got)
	}
	if got := ProcurementPermission(RequestTypeJasa, "verify"); got != "procurement.requests.verify" {
		t.Errorf("jasa verify = %q", got)
	}
	if got := ProcurementPermission(RequestTypeBahan, "verify"); got != "procurement.rawmaterials.verify" {
		t.Errorf("bahan verify = %q", got)
	}
	verifikator := []string{"procurement.requests.view", "procurement.requests.verify"}
	if !PermitsProcurement(verifikator, RequestTypeBarang, "verify") {
		t.Error("pemegang procurement.requests.verify harus bisa memverifikasi barang")
	}
	if PermitsProcurement(verifikator, RequestTypeBarang, "approve") {
		t.Error("verifikator tidak boleh otomatis bisa menyetujui")
	}
	if PermitsProcurement(verifikator, RequestTypeBahan, "verify") {
		t.Error("verifikator barang/jasa tidak boleh memverifikasi bahan baku")
	}
	// Kunci verify berprefix .requests. sehingga ikut membuka .view.
	if !PermitsProcurement([]string{"procurement.requests.verify"}, RequestTypeJasa, "view") {
		t.Error("procurement.requests.verify harus ikut membuka view (aturan prefix)")
	}
	has := func(key string) bool {
		for _, p := range AllPermissions {
			if p == key {
				return true
			}
		}
		return false
	}
	// Bahan baku tanpa tahap verifikasi (keputusan owner 14 Okt 2026): tidak
	// ada kunci verifikasi bahan baku yang bisa diberikan lewat halaman Role.
	if !has("procurement.requests.verify") {
		t.Error("procurement.requests.verify belum ada di AllPermissions")
	}
	if has("procurement.rawmaterials.verify") {
		t.Error("procurement.rawmaterials.verify masih ada di AllPermissions — bahan baku tidak melalui verifikasi")
	}
	for _, key := range []string{"procurement.requests.pickup", "procurement.rawmaterials.pickup"} {
		if !has(key) {
			t.Errorf("%s belum ada di AllPermissions", key)
		}
	}
}

func TestNormalizeVerifyRejections(t *testing.T) {
	ok := []models.VerifyRejectedItemInput{
		{Group: 0, Item: 2, Reason: "  Tersedia di outlet Kala — konfirmasi ke bagian perlengkapan  "},
		{Group: 1, Item: 0, Reason: "Ádá é"}, // 5 huruf (karakter multibyte dihitung per huruf)
	}
	got, err := normalizeVerifyRejections(ok)
	if err != nil {
		t.Fatalf("payload sah ditolak: %v", err)
	}
	if got[0].Reason != "Tersedia di outlet Kala — konfirmasi ke bagian perlengkapan" {
		t.Errorf("alasan tidak di-trim: %q", got[0].Reason)
	}
	if len(got) != 2 || got[1].Group != 1 || got[1].Item != 0 {
		t.Errorf("hasil normalisasi = %+v", got)
	}
	if out, err := normalizeVerifyRejections(nil); err != nil || len(out) != 0 {
		t.Errorf("tanpa item ditolak harus sah: %v %v", out, err)
	}

	salah := map[string][]models.VerifyRejectedItemInput{
		"indeks group negatif": {{Group: -1, Item: 0, Reason: "alasan cukup"}},
		"indeks item negatif":  {{Group: 0, Item: -1, Reason: "alasan cukup"}},
		"duplikat":             {{Group: 0, Item: 1, Reason: "alasan cukup"}, {Group: 0, Item: 1, Reason: "alasan lain lagi"}},
		"alasan kosong":        {{Group: 0, Item: 0, Reason: "   "}},
		"alasan 4 huruf":       {{Group: 0, Item: 0, Reason: " abcd "}},
	}
	for nama, in := range salah {
		_, err := normalizeVerifyRejections(in)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: error = %v, mau ValidationError", nama, err)
		}
	}
}

func verifyTestItems() []models.PurchaseRequestItem {
	return []models.PurchaseRequestItem{
		{Name: "Peralatan Dapur", Items: []models.PurchaseSubItem{
			{Name: "Kursi", Qty: 2, Unit: "unit", HpsPrice: 350000, HpsSource: "Toko A"},
			{Name: "Meja", Qty: 1, Unit: "unit", HpsPrice: 900000, HpsSource: "Toko B", RabItemID: "RAB1"},
		}},
		{Name: "Kebersihan", Items: []models.PurchaseSubItem{
			{Name: "Sapu", Qty: 3, Unit: "pcs", HpsPrice: 25000},
		}},
	}
}

func TestSplitRejectedItems(t *testing.T) {
	at := time.Date(2026, 10, 14, 1, 2, 3, 456, time.UTC)
	actor := Actor{ID: "01KVERIFIKATORUJI000000000", Name: "Petugas Perlengkapan"}
	rej := []verifyRejection{
		{Group: 1, Item: 0, Reason: "Masih ada di gudang"},
		{Group: 0, Item: 1, Reason: "Tersedia di outlet Kala"},
	}
	remaining, rejected, err := splitRejectedItems(verifyTestItems(), rej, actor, at)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	// Group "Kebersihan" habis → dibuang; "Peralatan Dapur" tinggal Kursi.
	if len(remaining) != 1 || remaining[0].Name != "Peralatan Dapur" || len(remaining[0].Items) != 1 || remaining[0].Items[0].Name != "Kursi" {
		t.Fatalf("sisa = %+v", remaining)
	}
	if len(rejected) != 2 {
		t.Fatalf("jumlah ditolak = %d, mau 2", len(rejected))
	}
	// Urutan mengikuti isi tersimpan, bukan urutan kiriman.
	meja := rejected[0]
	if meja.Name != "Meja" || meja.Group != "Peralatan Dapur" || meja.Qty != 1 || meja.Unit != "unit" ||
		meja.HpsPrice != 900000 || meja.HpsSource != "Toko B" || meja.RabItemID != "RAB1" ||
		meja.Reason != "Tersedia di outlet Kala" || meja.RejectedBy != actor.Name || meja.RejectedByID != actor.ID ||
		meja.RejectedAt != "2026-10-14T01:02:03Z" || meja.Stage != "verifikasi" {
		t.Errorf("baris ditolak tidak lengkap: %+v", meja)
	}
	if rejected[1].Name != "Sapu" || rejected[1].Group != "Kebersihan" {
		t.Errorf("baris ditolak kedua = %+v", rejected[1])
	}

	// Semua ditolak → tidak ada sisa.
	all := []verifyRejection{{0, 0, "alasan satu"}, {0, 1, "alasan dua"}, {1, 0, "alasan tiga"}}
	if rem, rj, err := splitRejectedItems(verifyTestItems(), all, actor, at); err != nil || len(rem) != 0 || len(rj) != 3 {
		t.Errorf("semua ditolak: sisa=%d ditolak=%d err=%v", len(rem), len(rj), err)
	}

	// Indeks di luar isi tersimpan → 400.
	for _, r := range []verifyRejection{{Group: 2, Item: 0, Reason: "tidak ada"}, {Group: 0, Item: 2, Reason: "tidak ada"}} {
		_, _, err := splitRejectedItems(verifyTestItems(), []verifyRejection{r}, actor, at)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("indeks (%d,%d): error = %v, mau ValidationError", r.Group, r.Item, err)
		}
	}
}

func TestApplyApprovedPriceEdit(t *testing.T) {
	stored := verifyTestItems()

	// Hanya harga final yang berubah → diterima; field lain diambil dari isi
	// tersimpan, bukan dari kiriman.
	in := verifyTestItems()
	in[0].Items[0].FinalPrice = 340000
	in[0].Items[1].FinalPrice = 880000
	in[0].Items[0].HpsSubtotal = 1 // angka turunan kiriman diabaikan
	out, err := applyApprovedPriceEdit(stored, in)
	if err != nil {
		t.Fatalf("isi harga final ditolak: %v", err)
	}
	if out[0].Items[0].FinalPrice != 340000 || out[0].Items[1].FinalPrice != 880000 || out[1].Items[0].FinalPrice != 0 {
		t.Errorf("harga final tidak terpasang: %+v", out)
	}
	if out[0].Items[1].RabItemID != "RAB1" || out[0].Items[0].HpsSource != "Toko A" {
		t.Errorf("field tersimpan hilang: %+v", out[0].Items)
	}

	ubah := map[string]func(items []models.PurchaseRequestItem) []models.PurchaseRequestItem{
		"qty": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem { x[0].Items[0].Qty = 20; return x },
		"nama item": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[0].Items[0].Name = "Laptop"
			return x
		},
		"satuan": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[1].Items[0].Unit = "lusin"
			return x
		},
		"HPS": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[0].Items[1].HpsPrice = 1
			return x
		},
		"sumber HPS": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[0].Items[0].HpsSource = "lain"
			return x
		},
		"baris RAB": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[0].Items[1].RabItemID = "RAB2"
			return x
		},
		"nama pengadaan": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem { x[1].Name = "Lain"; return x },
		"baris baru": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[1].Items = append(x[1].Items, models.PurchaseSubItem{Name: "Pel", Qty: 1, HpsPrice: 1})
			return x
		},
		"baris dihapus": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			x[0].Items = x[0].Items[:1]
			return x
		},
		"pengadaan baru": func(x []models.PurchaseRequestItem) []models.PurchaseRequestItem {
			return append(x, models.PurchaseRequestItem{Name: "Baru", Items: []models.PurchaseSubItem{{Name: "X", Qty: 1}}})
		},
	}
	for nama, f := range ubah {
		_, err := applyApprovedPriceEdit(stored, f(verifyTestItems()))
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: error = %v, mau ValidationError", nama, err)
			continue
		}
		if !strings.Contains(ve.Msg, "terkunci") {
			t.Errorf("%s: pesan tidak jelas: %q", nama, ve.Msg)
		}
	}
}

func TestVersionMatches(t *testing.T) {
	stored := time.Date(2026, 10, 14, 3, 4, 5, 123456000, time.UTC)
	v := purchaseVersion(stored)
	if v != "2026-10-14T03:04:05.123456Z" {
		t.Errorf("version = %q", v)
	}
	if !versionMatches(v, stored) {
		t.Error("version yang sama harus cocok")
	}
	// Zona berbeda, instan sama → cocok.
	if !versionMatches(stored.In(time.FixedZone("WIB", 7*3600)).Format(time.RFC3339Nano), stored) {
		t.Error("instan sama beda zona harus cocok")
	}
	if versionMatches(purchaseVersion(stored.Add(time.Microsecond)), stored) {
		t.Error("beda 1 mikrodetik harus dianggap berubah")
	}
	if versionMatches("bukan-waktu", stored) || versionMatches("", stored) {
		t.Error("version rusak/kosong tidak boleh cocok")
	}
	// Tanpa pecahan detik.
	noFrac := time.Date(2026, 10, 14, 3, 4, 5, 0, time.UTC)
	if !versionMatches("2026-10-14T03:04:05Z", noFrac) {
		t.Error("RFC3339 tanpa pecahan detik harus cocok")
	}
}

func TestValidActorID(t *testing.T) {
	kasus := map[string]string{
		"01KABCDEFGHJKMNPQRSTVWXYZ0":   "01KABCDEFGHJKMNPQRSTVWXYZ0",
		" 01KABCDEFGHJKMNPQRSTVWXYZ0 ": "01KABCDEFGHJKMNPQRSTVWXYZ0",
		// Akun superadmin bawaan (database/seeder.go) ber-id 25 karakter dan
		// dipakai sebagai "sub" JWT — wajib tetap tercatat sebagai akun.
		"01ADMIN000SUPERADMIN00001":   "01ADMIN000SUPERADMIN00001",
		"01ADMIN000SUPERADMIN00001 ":  "01ADMIN000SUPERADMIN00001",
		"static":                      "",
		"":                            "",
		"01kabcdefghjkmnpqrstvwxyz0":  "",
		"01KABCDEFGHJKMNPQRSTVWXYZ01": "", // 27 karakter
		"01K-ABCDEF":                  "",
	}
	for in, want := range kasus {
		if got := validActorID(in); got != want {
			t.Errorf("validActorID(%q) = %q, mau %q", in, got, want)
		}
	}
	// Token statis tidak menyentuh database.
	if a := ResolveActor("static", "admin"); a.ID != "" || a.Name != "admin" {
		t.Errorf("token statis = %+v, mau {ID:\"\" Name:admin}", a)
	}
}

func TestCommittedStatusHelpers(t *testing.T) {
	cond := prCommittedCond("pr")
	for _, s := range []string{"approved", "payment_requested", "partial", "paid", "received"} {
		if !strings.Contains(cond, "'"+s+"'") || !IsCommittedPurchaseStatus(s) {
			t.Errorf("status %s harus berkomitmen (%s)", s, cond)
		}
	}
	for _, s := range []string{"pending", "verified", "rejected", "cancelled"} {
		if strings.Contains(cond, "'"+s+"'") || IsCommittedPurchaseStatus(s) {
			t.Errorf("status %s tidak boleh dihitung belanja berjalan (%s)", s, cond)
		}
	}
	if !strings.HasPrefix(cond, "pr.status IN (") {
		t.Errorf("kondisi harus daftar putih ber-alias: %s", cond)
	}
	for _, s := range []string{"pending", "verified", "approved", "received"} {
		if !isLivePurchaseStatus(s) {
			t.Errorf("%s harus termasuk hidup", s)
		}
	}
	for _, s := range []string{"rejected", "cancelled", "returned", ""} {
		if isLivePurchaseStatus(s) {
			t.Errorf("%q tidak boleh termasuk hidup", s)
		}
	}
	if !IsValidPurchaseStatus("verified") || IsValidPurchaseStatus("priced") {
		t.Error("daftar status sah harus memuat verified dan menolak status asing")
	}
	// Hutang hanya dari status berkomitmen.
	if oc := outstandingCondFor(""); !strings.Contains(oc, "status IN (") || strings.Contains(oc, "'verified'") || strings.Contains(oc, "NOT IN") {
		t.Errorf("outstandingCond harus daftar putih berkomitmen: %s", oc)
	}
}

func TestPurchaseStatusLabelVerification(t *testing.T) {
	if got := purchaseStatusLabel("pending"); got != "Menunggu Verifikasi" {
		t.Errorf("label pending = %q", got)
	}
	if got := purchaseStatusLabel("verified"); got != "Menunggu Persetujuan" {
		t.Errorf("label verified = %q", got)
	}
}

func TestWAVerificationEvents(t *testing.T) {
	sub := waSpec("purchase.submitted")
	if sub == nil || len(sub.Permissions) != 1 || sub.Permissions[0] != "procurement.requests.verify" || sub.Extra != "pengaju" ||
		!strings.Contains(sub.Template, "Menunggu verifikasi") {
		t.Errorf("purchase.submitted harus ke pemegang verify + pengaju (tanda terima) dengan teks 'Menunggu verifikasi': %+v", sub)
	}
	ver := waSpec("purchase.verified")
	if ver == nil {
		t.Fatal("event purchase.verified belum ada")
	}
	if len(ver.Permissions) != 1 || ver.Permissions[0] != "procurement.requests.approve" || ver.Extra != "pengaju" ||
		!strings.Contains(ver.Template, "Menunggu persetujuan") || !strings.Contains(ver.Template, "{rejected_list}") {
		t.Errorf("purchase.verified harus ke pemegang approve + pengaju, memuat item ditolak: %+v", ver)
	}
	// Bahan baku tanpa verifikasi: hanya kunci verifikasi barang/jasa.
	if waPermissionLabels["procurement.requests.verify"] == "" {
		t.Error("label izin procurement.requests.verify belum ada di peta label WA")
	}
	if _, ok := waPermissionLabels["procurement.rawmaterials.verify"]; ok {
		t.Error("label izin procurement.rawmaterials.verify masih ada — kuncinya sudah dihapus")
	}

	pr := &models.PurchaseRequest{RejectedItems: []models.RejectedPurchaseItem{
		{Name: "Kursi", Qty: 2, Unit: "unit", Reason: "Tersedia di outlet Kala", Stage: "verifikasi"},
	}}
	n, list := waRejectedItemsVars(pr)
	if n != "1" || !strings.Contains(list, "Kursi ×2 unit — Tersedia di outlet Kala") {
		t.Errorf("rejected vars = %q / %q", n, list)
	}
	if n, list := waRejectedItemsVars(&models.PurchaseRequest{}); n != "0" || list != "" {
		t.Errorf("tanpa item ditolak = %q / %q, mau 0 / kosong", n, list)
	}
}

func TestWAExtraMatchesByID(t *testing.T) {
	a := waAdminRow{ID: "01KPENGAJUUJI0000000000000", Name: "Manager ATV"}
	if !waExtraMatches(waExtraIDPrefix+a.ID, a) {
		t.Error("id akun harus cocok")
	}
	if waExtraMatches(waExtraIDPrefix+"01KLAIN00000000000000000000", a) {
		t.Error("id akun lain tidak boleh cocok")
	}
	if !waExtraMatches("manager atv", a) {
		t.Error("nama (data lama) tetap cocok tanpa membedakan huruf besar")
	}
	if waExtraMatches("", a) || waExtraMatches(waExtraIDPrefix, a) {
		t.Error("entri kosong tidak boleh cocok")
	}
}

// ── Dengan database ────────────────────────────────────────────────────────

const (
	verifActorID   = "01KVERIFIKATORUJI000000000"
	approverID     = "01KPENYETUJUUJI00000000000"
	requesterID    = "01KPENGAJUUJI0000000000000"
	verifOddPrefix = "Zzz-Uji-Verif-"
)

func createVerifPR(t *testing.T, items []models.PurchaseRequestItem) *models.PurchaseRequest {
	t.Helper()
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeBarang, RequestedBy: "Pengaju Uji", RequestedByID: requesterID,
		Items: items,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		execScratch("DELETE FROM payment_histories WHERE purchase_request_id IN (SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)", pr.ID)
		execScratch("DELETE FROM purchase_requests WHERE parent_id = $1", pr.ID)
		execScratch("DELETE FROM purchase_requests WHERE id = $1", pr.ID)
	})
	return pr
}

func verifItems() []models.PurchaseRequestItem {
	return []models.PurchaseRequestItem{
		{Name: "Peralatan", Items: []models.PurchaseSubItem{
			{Name: verifOddPrefix + "Kursi", Qty: 2, Unit: "unit", HpsPrice: 100000, FinalPrice: 90000},
			{Name: verifOddPrefix + "Meja", Qty: 1, Unit: "unit", HpsPrice: 50000},
		}},
		{Name: "Lain-lain", Items: []models.PurchaseSubItem{
			{Name: verifOddPrefix + "Rak", Qty: 3, Unit: "pcs", HpsPrice: 10000},
		}},
	}
}

func verifyInput(version string, rej ...models.VerifyRejectedItemInput) models.UpdatePurchaseStatusInput {
	return models.UpdatePurchaseStatusInput{
		Action: "verify", ActorName: "Verifikator Uji", ActorID: verifActorID,
		Version: version, Notes: "Cek stok outlet lain", RejectedItems: rej,
	}
}

func TestVerifyRejectOneItemThenApprove(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	if pr.Status != "pending" || pr.Version == "" || pr.RejectedItems == nil || len(pr.RejectedItems) != 0 {
		t.Fatalf("pengajuan baru: status=%s version=%q rejected=%v", pr.Status, pr.Version, pr.RejectedItems)
	}
	if pr.RequestedByID == nil || *pr.RequestedByID != requesterID {
		t.Errorf("requested_by_id = %v, mau %s", pr.RequestedByID, requesterID)
	}

	got, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version,
		models.VerifyRejectedItemInput{Group: 0, Item: 1, Reason: "Tersedia di outlet Kala — konfirmasi ke bagian perlengkapan"},
		models.VerifyRejectedItemInput{Group: 1, Item: 0, Reason: "Masih ada di gudang"},
	))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != "verified" {
		t.Fatalf("status = %q, mau verified", got.Status)
	}
	if len(got.Items) != 1 || len(got.Items[0].Items) != 1 || got.Items[0].Items[0].Name != verifOddPrefix+"Kursi" {
		t.Errorf("sisa item = %+v, mau hanya Kursi (group Lain-lain dibuang)", got.Items)
	}
	// Total dihitung ulang dari sisa. Harga final kiriman pengaju (90.000 untuk
	// Kursi) diabaikan sejak dibuat — diisi pembelian setelah disetujui (T-39).
	if got.TotalHps != 200000 || got.TotalFinal != 0 || got.TotalAmount != 200000 {
		t.Errorf("total = hps %v / final %v / amount %v, mau 200000 / 0 / 200000", got.TotalHps, got.TotalFinal, got.TotalAmount)
	}
	if len(got.RejectedItems) != 2 {
		t.Fatalf("rejected_items = %+v, mau 2", got.RejectedItems)
	}
	ri := got.RejectedItems[0]
	if ri.Group != "Peralatan" || ri.Name != verifOddPrefix+"Meja" || ri.Stage != "verifikasi" ||
		ri.RejectedBy != "Verifikator Uji" || ri.RejectedByID != verifActorID || ri.RejectedAt == "" ||
		!strings.HasPrefix(ri.Reason, "Tersedia di outlet Kala") {
		t.Errorf("baris ditolak = %+v", ri)
	}
	if got.VerifiedBy != "Verifikator Uji" || got.VerifiedByID == nil || *got.VerifiedByID != verifActorID ||
		got.VerifiedAt == nil || got.VerificationNotes != "Cek stok outlet lain" {
		t.Errorf("jejak verifikasi: by=%q id=%v at=%v notes=%q", got.VerifiedBy, got.VerifiedByID, got.VerifiedAt, got.VerificationNotes)
	}
	if got.Version == pr.Version {
		t.Error("version harus berubah setelah verifikasi")
	}

	// Verifikasi kedua tidak mungkin (sudah bukan pending).
	if _, err := UpdatePurchaseStatus(pr.ID, verifyInput(got.Version)); err == nil {
		t.Error("verifikasi ulang atas pengajuan verified diterima")
	}

	// Approve dari verified, pelaku tersimpan sebagai akun.
	app, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "approve", ActorName: "Owner Uji", ActorID: approverID, Version: got.Version,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if app.Status != "approved" || app.ApprovedBy == nil || *app.ApprovedBy != "Owner Uji" ||
		app.ApprovedByID == nil || *app.ApprovedByID != approverID || app.ApprovedAt == nil {
		t.Errorf("approve: status=%s by=%v id=%v", app.Status, app.ApprovedBy, app.ApprovedByID)
	}
	if len(app.RejectedItems) != 2 {
		t.Errorf("rejected_items hilang setelah approve: %+v", app.RejectedItems)
	}
}

func TestVerifyAllItemsRejectedMeansRejected(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, []models.PurchaseRequestItem{{Name: "Peralatan", Items: []models.PurchaseSubItem{
		{Name: verifOddPrefix + "Blender", Qty: 1, Unit: "unit", HpsPrice: 750000},
	}}})
	got, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version,
		models.VerifyRejectedItemInput{Group: 0, Item: 0, Reason: "Blender menganggur di outlet Semesta"}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != "rejected" {
		t.Fatalf("status = %q, mau rejected", got.Status)
	}
	if got.RejectedReason == nil || *got.RejectedReason != "Semua item ditolak saat verifikasi" {
		t.Errorf("rejected_reason = %v", got.RejectedReason)
	}
	if got.RejectedBy != "Verifikator Uji" || got.RejectedAt == nil {
		t.Errorf("penolak = %q at %v", got.RejectedBy, got.RejectedAt)
	}
	if len(got.Items) != 0 || got.TotalHps != 0 || got.TotalAmount != 0 || len(got.RejectedItems) != 1 {
		t.Errorf("isi setelah semua ditolak: items=%d hps=%v amount=%v rejected=%d", len(got.Items), got.TotalHps, got.TotalAmount, len(got.RejectedItems))
	}
	var rejectedByID, approvedBy *string
	database.DB.QueryRow("SELECT rejected_by_id, approved_by FROM purchase_requests WHERE id = $1", pr.ID).Scan(&rejectedByID, &approvedBy)
	if rejectedByID == nil || *rejectedByID != verifActorID {
		t.Errorf("rejected_by_id = %v, mau %s", rejectedByID, verifActorID)
	}
	if approvedBy != nil {
		t.Errorf("penolakan tidak boleh ditulis ke approved_by (T-73), dapat %q", *approvedBy)
	}
}

func TestVerifyStaleVersionAndMissingVersion(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())

	// Tanpa version → 400.
	_, err := UpdatePurchaseStatus(pr.ID, verifyInput(""))
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("verify tanpa version: error = %v, mau ValidationError", err)
	}

	// Pengaju mengubah qty setelah verifikator membuka dokumen.
	time.Sleep(2 * time.Millisecond)
	items := verifItems()
	items[0].Items[0].Qty = 5
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); err != nil {
		t.Fatalf("edit pending: %v", err)
	}
	_, err = UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version))
	var ce ConflictError
	if !errors.As(err, &ce) || ce.Msg != "Pengajuan berubah, muat ulang" {
		t.Fatalf("version basi: error = %v, mau ConflictError 'Pengajuan berubah, muat ulang'", err)
	}
	if st, _ := GetPurchaseRequest(pr.ID); st.Status != "pending" {
		t.Errorf("status setelah konflik = %q, mau tetap pending", st.Status)
	}

	// Version opsional pada aksi lain tetap diperiksa bila dikirim.
	fresh, _ := GetPurchaseRequest(pr.ID)
	if _, err := UpdatePurchaseStatus(pr.ID, verifyInput(fresh.Version)); err != nil {
		t.Fatalf("verify dengan version baru: %v", err)
	}
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "x", Version: fresh.Version})
	if !errors.As(err, &ce) {
		t.Errorf("approve dengan version basi: error = %v, mau ConflictError", err)
	}

	// Status yang dipakai handler memilih izin sudah lewat → 409.
	_, err = UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "reject", ActorName: "x", RejectedReason: "alasan", ExpectedStatus: "pending"})
	if !errors.As(err, &ce) {
		t.Errorf("ExpectedStatus basi: error = %v, mau ConflictError", err)
	}
}

func TestApproveFromPendingRejected(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Owner"}); err == nil {
		t.Fatal("approve langsung dari pending diterima — verifikasi terlewati")
	}
	if st, _ := GetPurchaseRequest(pr.ID); st.Status != "pending" || st.ApprovedBy != nil {
		t.Errorf("status = %q approved_by = %v, mau pending tanpa penyetuju", st.Status, st.ApprovedBy)
	}
	// Tolak tanpa alasan → 400, termasuk dari verified.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "reject", ActorName: "Verif", RejectedReason: "  "}); err == nil {
		t.Error("tolak tanpa alasan diterima")
	}
}

func TestEditLockedAfterVerifyAndOnlyPriceAfterApprove(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	ver, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// verified: isi terkunci.
	_, err = UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: ver.Items})
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "terkunci") {
		t.Errorf("edit saat verified: error = %v, mau ValidationError 'terkunci'", err)
	}

	// Batal tetap boleh dari verified — diuji di dokumen terpisah; di sini lanjut approve.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Owner"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var kindBefore string
	database.DB.QueryRow("SELECT COALESCE(goods_kind,'') FROM purchase_requests WHERE id = $1", pr.ID).Scan(&kindBefore)

	// approved: qty berubah → ditolak.
	items := verifItems()
	items[0].Items[0].Qty = 100
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); !errors.As(err, &ve) {
		t.Errorf("ubah qty setelah approve: error = %v, mau ValidationError", err)
	}
	// approved: baris baru → ditolak.
	items = verifItems()
	items[1].Items = append(items[1].Items, models.PurchaseSubItem{Name: "Laptop", Qty: 1, Unit: "unit", HpsPrice: 9000000})
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items}); !errors.As(err, &ve) {
		t.Errorf("tambah baris setelah approve: error = %v, mau ValidationError", err)
	}

	// approved: hanya harga final + vendor + invoice → diterima.
	items = verifItems()
	items[0].Items[0].FinalPrice = 95000
	items[0].Items[1].FinalPrice = 45000
	items[1].Items[0].FinalPrice = 9000
	vendor, invoice := "CV Uji Verifikasi", "INV-VERIF-1"
	got, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: items, VendorName: &vendor, InvoiceNumber: &invoice})
	if err != nil {
		t.Fatalf("isi harga final: %v", err)
	}
	if got.TotalFinal != 2*95000+45000+3*9000 || got.TotalHps != 200000+50000+30000 {
		t.Errorf("total setelah harga final: final=%v hps=%v", got.TotalFinal, got.TotalHps)
	}
	if got.VendorName != vendor || got.InvoiceNumber != invoice || got.Status != "approved" {
		t.Errorf("vendor=%q invoice=%q status=%q", got.VendorName, got.InvoiceNumber, got.Status)
	}
	var kindAfter string
	database.DB.QueryRow("SELECT COALESCE(goods_kind,'') FROM purchase_requests WHERE id = $1", pr.ID).Scan(&kindAfter)
	if kindAfter != kindBefore {
		t.Errorf("goods_kind berubah oleh isi harga (%q → %q) — jenis belanja terkunci setelah disetujui", kindBefore, kindAfter)
	}
}

func TestCancelFromVerified(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	if _, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	got, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "cancel", ActorName: "Pengaju"})
	if err != nil || got.Status != "cancelled" {
		t.Fatalf("batal dari verified: status=%v err=%v", got, err)
	}
}

// Pengajuan yang baru diverifikasi (harga final sudah terisi) belum disetujui
// owner: tidak boleh muncul sebagai hutang, akrual Buku Besar, maupun belanja
// vendor. Begitu disetujui baru ikut terhitung.
//
// Sejak T-39 harga final tidak bisa diisi sebelum disetujui lewat API, jadi
// keadaan "verified bernilai final" disiapkan langsung di database (data lama
// / jalur lain) — filter laporan tetap wajib menolaknya.
func TestVerifiedNotCountedAsPayableOrSpend(t *testing.T) {
	requireDB(t)
	marker := "verif-hutang-" + NewULID()[20:]
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: marker,
		Items: []models.PurchaseRequestItem{{Name: "Servis", Items: []models.PurchaseSubItem{
			{Name: "Servis genset uji", Qty: 1, Unit: "kali", HpsPrice: 400000, FinalPrice: 400000},
		}}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM purchase_requests WHERE id = $1", pr.ID) })

	base, err := GetPaymentStats(nil, nil)
	if err != nil {
		t.Fatalf("payment stats awal: %v", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v, _ := GetPurchaseRequest(pr.ID); v.TotalFinal != 0 {
		t.Errorf("harga final pengaju terbawa sampai verified: total_final=%v, mau 0 (T-39)", v.TotalFinal)
	}
	execScratch(`UPDATE purchase_requests SET total_final = 400000, total_amount = 400000,
		items = jsonb_set(items, '{0,items,0,final_price}', '400000'::jsonb) WHERE id = $1`, pr.ID)
	afterVerify, _ := GetPaymentStats(nil, nil)
	if d := afterVerify.AccountsPayable.TotalAmount - base.AccountsPayable.TotalAmount; d != 0 {
		t.Errorf("hutang bertambah %v oleh pengajuan yang baru diverifikasi — mau 0", d)
	}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	gl, err := GetGeneralLedger(today, today, "", "", nil)
	if err != nil {
		t.Fatalf("GL: %v", err)
	}
	if _, c := sumEntries(glAccount(gl, "2-100"), marker); c != 0 {
		t.Errorf("akrual hutang Buku Besar = %v untuk pengajuan verified — mau 0", c)
	}
	dash, err := GetProcurementDashboard("", nil, nil)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dash.StatusBreakdown.Verified < 1 {
		t.Errorf("dashboard verified = %d, mau ≥ 1 (Menunggu Persetujuan)", dash.StatusBreakdown.Verified)
	}

	// Setelah disetujui → masuk hutang dan akrual.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Owner"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	afterApprove, _ := GetPaymentStats(nil, nil)
	if d := afterApprove.AccountsPayable.TotalAmount - base.AccountsPayable.TotalAmount; d != 400000 {
		t.Errorf("hutang setelah disetujui bertambah %v, mau 400000", d)
	}
	gl, _ = GetGeneralLedger(today, today, "", "", nil)
	if _, c := sumEntries(glAccount(gl, "2-100"), marker); c != 400000 {
		t.Errorf("akrual hutang setelah disetujui = %v, mau 400000", c)
	}
	bal, err := GetBalanceReport(today, today, "", nil)
	if err != nil {
		t.Fatalf("neraca: %v", err)
	}
	if bal.AccountsPayable != afterApprove.AccountsPayable.TotalAmount {
		t.Errorf("hutang Neraca %v ≠ halaman Pembayaran %v", bal.AccountsPayable, afterApprove.AccountsPayable.TotalAmount)
	}
}

// Pecah vendor hanya dari approved, tanpa bersarang, dan pecahan membawa
// jejak verifikasi/persetujuan serta jenis belanja induknya.
func TestSplitOnlyFromApprovedCarriesTrail(t *testing.T) {
	requireDB(t)
	pr := createVerifPR(t, verifItems())
	splitIn := models.SplitPurchaseRequestInput{VendorName: "CV Pecahan", Items: []models.PurchaseRequestItem{{
		Name: "Lain-lain", Items: []models.PurchaseSubItem{{Name: verifOddPrefix + "Rak", Qty: 3}},
	}}}
	if _, err := SplitPurchaseRequest(pr.ID, splitIn); err == nil {
		t.Fatal("split dari pending diterima")
	}
	if _, err := UpdatePurchaseStatus(pr.ID, verifyInput(pr.Version)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := SplitPurchaseRequest(pr.ID, splitIn); err == nil {
		t.Fatal("split dari verified diterima")
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Owner Uji", ActorID: approverID}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	master, err := SplitPurchaseRequest(pr.ID, splitIn)
	if err != nil {
		t.Fatalf("split dari approved: %v", err)
	}
	if len(master.Children) != 1 {
		t.Fatalf("pecahan = %d", len(master.Children))
	}
	ch := master.Children[0]
	if ch.Status != "approved" || ch.ApprovedBy == nil || *ch.ApprovedBy != "Owner Uji" || ch.ApprovedByID == nil ||
		ch.VerifiedBy != "Verifikator Uji" || ch.RequestedByID == nil || *ch.RequestedByID != requesterID {
		t.Errorf("pecahan tanpa jejak: status=%s approved_by=%v verified_by=%q requested_by_id=%v",
			ch.Status, ch.ApprovedBy, ch.VerifiedBy, ch.RequestedByID)
	}
	var parentKind, childKind string
	database.DB.QueryRow("SELECT COALESCE(goods_kind,'') FROM purchase_requests WHERE id = $1", pr.ID).Scan(&parentKind)
	database.DB.QueryRow("SELECT COALESCE(goods_kind,'') FROM purchase_requests WHERE id = $1", ch.ID).Scan(&childKind)
	if parentKind == "" || childKind != parentKind {
		t.Errorf("goods_kind pecahan = %q, induk = %q — harus sama (T-30)", childKind, parentKind)
	}
	if _, err := SplitPurchaseRequest(ch.ID, models.SplitPurchaseRequestInput{VendorName: "CV Cucu", Items: ch.Items}); err == nil {
		t.Error("pecahan berhasil dipecah lagi (bersarang)")
	}
}

// Unit kerja menentukan outlet: kombinasi outlet A + unit B ditolak (T-9).
func TestCreateDerivesOutletFromWorkUnit(t *testing.T) {
	requireDB(t)
	rnd := NewULID() // 26 karakter
	outletA, outletB := "01KOUTLETUJIA"+rnd[13:], "01KOUTLETUJIB"+rnd[13:]
	unitA := "01KUNITUJIAA" + rnd[12:]
	for i, o := range []string{outletA, outletB} {
		if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
			o, fmt.Sprintf("UV%d%s", i, rnd[18:]), "Outlet Uji "+rnd[22:], "key-"+o); err != nil {
			t.Fatalf("outlet uji: %v", err)
		}
	}
	if _, err := database.DB.Exec(`INSERT INTO work_units (id, outlet_id, name, created_at, updated_at) VALUES ($1, $2, 'Unit Uji A', NOW(), NOW())`, unitA, outletA); err != nil {
		t.Fatalf("unit uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch("DELETE FROM purchase_requests WHERE work_unit_id = $1 OR outlet_id IN ($2, $3)", unitA, outletA, outletB)
		execScratch("DELETE FROM work_units WHERE id = $1 OR outlet_id IN ($2, $3)", unitA, outletA, outletB)
		execScratch("DELETE FROM outlets WHERE id IN ($1, $2)", outletA, outletB)
	})

	in := models.CreatePurchaseRequestInput{
		RequestType: RequestTypeJasa, RequestedBy: "Pengaju", WorkUnitID: unitA,
		Items: []models.PurchaseRequestItem{{Name: "Jasa", Items: []models.PurchaseSubItem{{Name: "Cuci AC", Qty: 1, HpsPrice: 1000}}}},
	}
	pr, err := CreatePurchaseRequest(in)
	if err != nil {
		t.Fatalf("create dengan unit: %v", err)
	}
	if pr.OutletID == nil || *pr.OutletID != outletA {
		t.Errorf("outlet = %v, mau diturunkan dari unit (%s)", pr.OutletID, outletA)
	}

	in.OutletID = outletB
	_, err = CreatePurchaseRequest(in)
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "outlet tidak sesuai") {
		t.Errorf("outlet B + unit A: error = %v, mau ValidationError 'outlet tidak sesuai'", err)
	}

	in.OutletID, in.WorkUnitID = "", "01KUNITTIDAKADA00000000000"
	if _, err := CreatePurchaseRequest(in); !errors.As(err, &ve) {
		t.Errorf("unit tidak ada: error = %v, mau ValidationError", err)
	}

	// Jalur otomatis (outlet saja): unitnya diturunkan dari outlet.
	in.OutletID, in.WorkUnitID = outletA, ""
	auto, err := CreatePurchaseRequest(in)
	if err != nil {
		t.Fatalf("create outlet saja: %v", err)
	}
	if auto.WorkUnitID == nil || *auto.WorkUnitID != unitA {
		t.Errorf("unit jalur otomatis = %v, mau %s", auto.WorkUnitID, unitA)
	}
}

// Belanja projek wajib dari unit pemilik projek (T-24), dan serapan RAB per
// baris dihitung dari semua pengajuan projek, bukan hanya yang terlihat (T-25).
func TestProjectOwnerAndAbsorptionIgnoreViewerScope(t *testing.T) {
	requireDB(t)
	rnd := NewULID() // 26 karakter
	outletA, outletB := "01KOUTLETPRJA"+rnd[13:], "01KOUTLETPRJB"+rnd[13:]
	unitA, unitB := "01KUNITPRJAA"+rnd[12:], "01KUNITPRJBB"+rnd[12:]
	for i, o := range []string{outletA, outletB} {
		if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
			o, fmt.Sprintf("PJ%d%s", i, rnd[18:]), "Outlet Projek "+rnd[22:], "key-"+o); err != nil {
			t.Fatalf("outlet uji: %v", err)
		}
	}
	for u, o := range map[string]string{unitA: outletA, unitB: outletB} {
		if _, err := database.DB.Exec(`INSERT INTO work_units (id, outlet_id, name, created_at, updated_at) VALUES ($1, $2, 'Unit Projek', NOW(), NOW())`, u, o); err != nil {
			t.Fatalf("unit uji: %v", err)
		}
	}
	proj, err := CreateProject(models.CreateProjectRequest{Name: "Projek Uji Scope", Budget: 1000000, Status: "berjalan", WorkUnitID: unitA}, "tester")
	if err != nil {
		t.Fatalf("projek: %v", err)
	}
	t.Cleanup(func() {
		execScratch("DELETE FROM purchase_requests WHERE project_id = $1", proj.ID)
		execScratch("DELETE FROM project_rab_items WHERE project_id = $1", proj.ID)
		execScratch("DELETE FROM projects WHERE id = $1", proj.ID)
		execScratch("DELETE FROM work_units WHERE id IN ($1, $2)", unitA, unitB)
		execScratch("DELETE FROM outlets WHERE id IN ($1, $2)", outletA, outletB)
	})
	rab, err := GetProjectRab(proj.ID, nil, nil)
	if err != nil || len(rab.Items) != 1 {
		t.Fatalf("rab: %v", err)
	}
	line := rab.Items[0].ID
	mk := func(unit string) (*models.PurchaseRequest, error) {
		return CreatePurchaseRequest(models.CreatePurchaseRequestInput{
			RequestType: RequestTypeBarang, RequestedBy: "Pengaju", WorkUnitID: unit, ProjectID: proj.ID,
			Items: []models.PurchaseRequestItem{{Name: "Material", Items: []models.PurchaseSubItem{
				{Name: verifOddPrefix + "Keramik", Qty: 2, Unit: "dus", HpsPrice: 100000, RabItemID: line},
			}}},
		})
	}
	// Unit outlet lain tidak boleh menyerap RAB projek ini.
	_, err = mk(unitB)
	var ve ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "milik unit kerja lain") {
		t.Errorf("pengajuan dari unit lain: error = %v, mau ValidationError", err)
	}
	own, err := mk(unitA)
	if err != nil {
		t.Fatalf("pengajuan dari unit projek: %v", err)
	}
	// Anggaran baru terpakai saat disetujui (keputusan owner 14 Okt 2026).
	verifyAndApprove(t, own.ID)
	// Penonton ber-scope outlet B tetap melihat serapan yang sama.
	viewer, err := GetProjectRab(proj.ID, []string{outletB}, []string{unitB})
	if err != nil {
		t.Fatalf("rab penonton: %v", err)
	}
	if viewer.Items[0].Committed != 200000 {
		t.Errorf("serapan baris bagi penonton lain = %v, mau 200000 (dari semua pengajuan)", viewer.Items[0].Committed)
	}
}

// Akun superadmin bawaan ber-id 25 karakter (database/seeder.go) dan id itu
// menjadi "sub" JWT: pelakunya tetap tercatat sebagai akun — nama dari
// cloud_admins, id tersimpan, dan terbaca tanpa spasi pengisi CHAR(26).
func TestActorWith25CharAccountID(t *testing.T) {
	requireDB(t)
	id := "01ADMINUJI00PENYETUJU0001" // 25 karakter
	if _, err := database.DB.Exec(`INSERT INTO cloud_admins (id, username, password_hash, name, role)
		VALUES ($1, 'uji_owner_25', 'x', 'Owner Uji 25', 'superadmin') ON CONFLICT (id) DO NOTHING`, id); err != nil {
		t.Fatalf("akun uji: %v", err)
	}
	t.Cleanup(func() { execScratch("DELETE FROM cloud_admins WHERE id = $1", id) })

	a := ResolveActor(id, "uji_owner_25")
	if a.ID != id || a.Name != "Owner Uji 25" {
		t.Fatalf("ResolveActor = %+v, mau id %s nama 'Owner Uji 25'", a, id)
	}
	pr := createVerifPR(t, verifItems())
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "verify", ActorName: a.Name, ActorID: a.ID, Version: pr.Version,
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	got, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: a.Name, ActorID: a.ID})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got.ApprovedByID == nil || *got.ApprovedByID != id || got.VerifiedByID == nil || *got.VerifiedByID != id {
		t.Errorf("approved_by_id = %v, verified_by_id = %v, mau %q tanpa spasi", got.ApprovedByID, got.VerifiedByID, id)
	}
}
