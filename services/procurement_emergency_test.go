package services

import (
	"strings"
	"testing"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Tingkat kebutuhan & Pembelian Darurat (keputusan owner 19 Nov 2026).

func TestNormalizeUrgencyRules(t *testing.T) {
	loc := GetTimezoneLocation()
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, loc)
	cases := []struct {
		name, urgency, needBy, notes string
		wantErr                      string
		wantUrgency                  string
		wantDate                     string
	}{
		{name: "bawaan rutin", wantUrgency: "rutin"},
		{name: "rutin boleh bertanggal", urgency: "rutin", needBy: "2026-12-01", wantUrgency: "rutin", wantDate: "2026-12-01"},
		{name: "huruf besar", urgency: "PRIORITAS", needBy: "2026-11-21", wantUrgency: "prioritas", wantDate: "2026-11-21"},
		{name: "prioritas tanpa tanggal", urgency: "prioritas", wantErr: "tanggal dibutuhkan wajib"},
		{name: "urgent tanpa tanggal", urgency: "urgent", notes: "Genset mati total", wantErr: "tanggal dibutuhkan wajib"},
		{name: "tanggal lampau", urgency: "prioritas", needBy: "2026-11-18", wantErr: "sebelum hari ini"},
		{name: "format salah", urgency: "rutin", needBy: "19/11/2026", wantErr: "tidak valid"},
		{name: "urgent tanpa dampak", urgency: "urgent", needBy: "2026-11-19", notes: "cepat", wantErr: "menjelaskan dampaknya"},
		{name: "urgent terlalu jauh", urgency: "urgent", needBy: "2026-11-27", notes: "Genset mati total", wantErr: "paling lambat 7 hari"},
		{name: "urgent sah", urgency: "urgent", needBy: "2026-11-26", notes: "Genset mati total", wantUrgency: "urgent", wantDate: "2026-11-26"},
		{name: "jenis asing", urgency: "darurat", wantErr: "rutin, prioritas, atau urgent"},
	}
	for _, c := range cases {
		u, d, err := normalizeUrgency(c.urgency, c.needBy, c.notes, now)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: galat %q, dapat %v", c.name, c.wantErr, err)
			}
			continue
		}
		if err != nil || u != c.wantUrgency {
			t.Errorf("%s: dapat (%q, %v)", c.name, u, err)
			continue
		}
		got := ""
		if d != nil {
			got = *d
		}
		if got != c.wantDate {
			t.Errorf("%s: tanggal %q, dapat %q", c.name, c.wantDate, got)
		}
	}
}

func TestWAUrgencyTag(t *testing.T) {
	d := "2026-11-20"
	if got := waUrgencyTag(&models.PurchaseRequest{Urgency: "urgent", NeedByDate: &d}); !strings.Contains(got, "URGENT") || !strings.Contains(got, "20 Nov 2026") {
		t.Fatalf("urgent: %q", got)
	}
	if got := waUrgencyTag(&models.PurchaseRequest{Urgency: "prioritas"}); !strings.Contains(got, "PRIORITAS") {
		t.Fatalf("prioritas: %q", got)
	}
	if got := waUrgencyTag(&models.PurchaseRequest{Urgency: "rutin"}); got != "" {
		t.Fatalf("rutin harus kosong: %q", got)
	}
	if got := waUrgencyTag(&models.PurchaseRequest{Urgency: "urgent", Emergency: &models.PurchaseEmergency{}}); !strings.Contains(got, "DARURAT") {
		t.Fatalf("darurat: %q", got)
	}
	if got := waDuration(5 * time.Hour); got != "5 jam" {
		t.Fatalf("durasi jam: %q", got)
	}
	if got := waDuration(50 * time.Hour); got != "2 hari" {
		t.Fatalf("durasi hari: %q", got)
	}
}

// emergencyTestUnit: outlet + unit kerjanya, dibersihkan beserta seluruh
// pengajuan, tugas kas, pembayaran, dan kas keluar outlet itu.
func emergencyTestUnit(t *testing.T) (string, string) {
	t.Helper()
	outlet := payoutTestOutlet(t)
	unit := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO work_units (id, outlet_id, name, created_at, updated_at) VALUES ($1, $2, 'Unit Darurat', NOW(), NOW())`, unit, outlet); err != nil {
		t.Fatalf("unit uji: %v", err)
	}
	t.Cleanup(func() {
		execScratch(`DELETE FROM cloud_cash_movements WHERE outlet_id = $1`, outlet)
		execScratch(`DELETE FROM payment_histories WHERE purchase_request_id IN (SELECT id FROM purchase_requests WHERE outlet_id = $1)`, outlet)
		execScratch(`DELETE FROM procurement_cash_payouts WHERE outlet_id = $1`, outlet)
		execScratch(`DELETE FROM purchase_requests WHERE outlet_id = $1`, outlet)
		execScratch(`DELETE FROM device_heartbeats WHERE outlet_id = $1`, outlet)
		execScratch(`DELETE FROM work_units WHERE id = $1`, unit)
	})
	return outlet, unit
}

func emergencyInput(unit string, price float64) models.CreateEmergencyPurchaseInput {
	return models.CreateEmergencyPurchaseInput{
		WorkUnitID: unit, RequestType: "jasa", Category: "peralatan_rusak",
		Reason: "Genset mati, dapur tidak bisa beroperasi", ShopName: "Bengkel Jaya", RecipientName: "Rahmad",
		Items:       []models.EmergencyPurchaseItemInput{{Name: "Servis genset", Qty: 1, Unit: "kali", Price: price}},
		RequestedBy: "Manajer Uji",
	}
}

// openPayoutFor: tugas kas terbuka milik pengajuan ini di daftar tablet.
func openPayoutFor(t *testing.T, outlet, prID string) *models.ProcurementCashPayout {
	t.Helper()
	list, err := ListOutletCashPayouts(outlet)
	if err != nil {
		t.Fatalf("daftar tablet: %v", err)
	}
	for i := range list {
		if list[i].PurchaseRequestID == prID && (list[i].Status == "pending" || list[i].Status == "claimed") {
			return &list[i]
		}
	}
	t.Fatalf("tugas kas untuk %s tidak ada di tablet: %+v", prID, list)
	return nil
}

// handCash: kasir menyerahkan kas lewat tablet (klaim + kas keluar + selesai).
func handCash(t *testing.T, outlet, payoutID string) {
	t.Helper()
	m := movementID()
	if _, err := ClaimCashPayout(outlet, payoutID, device(m)); err != nil {
		t.Fatalf("klaim: %v", err)
	}
	if _, err := CompleteCashPayout(outlet, payoutID, m); err != nil {
		t.Fatalf("selesai: %v", err)
	}
}

func TestEmergencyPurchaseCashThenProofThenRatify(t *testing.T) {
	requireDB(t)
	outlet, unit := emergencyTestUnit(t)

	// Tablet masih v10.19 → peringatan; v10.20 → siap.
	if _, err := database.DB.Exec(`INSERT INTO device_heartbeats (outlet_id, app_version, received_at) VALUES ($1, '1.2.0+95008427', NOW() AT TIME ZONE 'UTC')`, outlet); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	ctx, err := GetEmergencyContext(unit)
	if err != nil || ctx.TabletReady || ctx.TabletNote == "" || ctx.MaxAmount != 500000 || ctx.OutletID != outlet {
		t.Fatalf("konteks tablet lama: %+v %v", ctx, err)
	}
	execScratch(`UPDATE device_heartbeats SET app_version = '1.2.0+95013423' WHERE outlet_id = $1`, outlet)
	if ctx, err = GetEmergencyContext(unit); err != nil || !ctx.TabletReady {
		t.Fatalf("konteks tablet v10.20: %+v %v", ctx, err)
	}

	pr, err := CreateEmergencyPurchase(emergencyInput(unit, 450000))
	if err != nil {
		t.Fatalf("buat Pembelian Darurat: %v", err)
	}
	if pr.Status != "payment_requested" || pr.Urgency != "urgent" || pr.Emergency == nil ||
		pr.Emergency.Ratification != "pending" || pr.TotalFinal != 450000 || pr.ApprovedAt == nil ||
		pr.Emergency.Recipient != "Rahmad" || pr.Emergency.CategoryLabel == "" || pr.NeedByDate == nil {
		t.Fatalf("pengajuan darurat: %+v / %+v", pr, pr.Emergency)
	}
	if pr.OutletID == nil || *pr.OutletID != outlet || pr.VendorName != "Bengkel Jaya" {
		t.Fatalf("outlet/toko: %+v", pr)
	}
	if !IsCommittedPurchaseStatus(pr.Status) {
		t.Fatal("Pembelian Darurat harus berkomitmen sejak dicatat (masuk hutang & pembukuan)")
	}
	task := openPayoutFor(t, outlet, pr.ID)
	if task.Status != "pending" || task.Amount != 450000 || task.RecipientName != "Rahmad" ||
		!strings.Contains(task.Notes, "PEMBELIAN DARURAT") || !strings.Contains(task.ItemsSummary, "Servis genset") {
		t.Fatalf("tugas kas tablet: %+v", task)
	}

	// Sebelum kas keluar: belum ada yang bisa dibuktikan atau disahkan.
	if _, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{ProofURL: "/uploads/struk-genset.jpg", ProofTotal: 450000}); err == nil ||
		!strings.Contains(err.Error(), "kas belum diserahkan") {
		t.Fatalf("bukti sebelum kas keluar harus ditolak: %v", err)
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "ratify", ActorName: "Owner"}); err == nil {
		t.Fatal("pengesahan sebelum kas keluar harus ditolak")
	}
	// Kantor Pusat tidak bisa membayar selama tugas kas terbuka.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "Keuangan"}); err == nil {
		t.Fatal("bayar Kantor Pusat harus ditolak")
	}

	handCash(t, outlet, task.ID)
	paid, _ := GetPurchaseRequest(pr.ID)
	if paid.Status != "paid" || paid.PaidAmount != 450000 {
		t.Fatalf("setelah kas diserahkan: status=%s dibayar=%v", paid.Status, paid.PaidAmount)
	}
	var n int
	var method string
	var vendorNull bool
	database.DB.QueryRow(`SELECT COUNT(*), MAX(payment_method), BOOL_AND(vendor_id IS NULL) FROM payment_histories WHERE purchase_request_id = $1`, pr.ID).
		Scan(&n, &method, &vendorNull)
	if n != 1 || method != "outlet_cash" || !vendorNull {
		t.Fatalf("pembayaran: n=%d metode=%s vendor_null=%v", n, method, vendorNull)
	}

	// Sudah keluar uang: tidak bisa dibatalkan lagi.
	if _, err := CancelEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Notes: "tidak jadi dibeli", ActorName: "Manajer Uji"}); err == nil {
		t.Fatal("pembatalan setelah kas keluar harus ditolak")
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "ratify", ActorName: "Owner"}); err == nil ||
		!strings.Contains(err.Error(), "foto struk") {
		t.Fatalf("pengesahan tanpa struk harus ditolak: %v", err)
	}
	if _, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{ProofURL: "https://contoh.com/a.jpg", ProofTotal: 450000}); err == nil {
		t.Fatal("tautan luar sebagai struk harus ditolak")
	}
	if _, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{ProofURL: "/uploads/struk-genset.jpg"}); err == nil {
		t.Fatal("struk tanpa total harus ditolak")
	}
	withProof, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{
		ProofURL: "/uploads/struk-genset.jpg", ProofTotal: 430000, Notes: "kembalian 20 rb ke laci", ActorName: "Manajer Uji",
	})
	if err != nil || withProof.Emergency.ProofURL == "" || withProof.Emergency.ProofTotal == nil || *withProof.Emergency.ProofTotal != 430000 {
		t.Fatalf("bukti belanja: %+v %v", withProof.Emergency, err)
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "reject", Notes: "no", ActorName: "Owner"}); err == nil {
		t.Fatal("tolak tanpa alasan cukup harus ditolak")
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "ratify", Version: "2020-01-01T00:00:00Z", ActorName: "Owner"}); err == nil {
		t.Fatal("versi usang harus 409")
	}
	done, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "ratify", Notes: "wajar", ActorName: "Owner", Version: withProof.Version})
	if err != nil || done.Emergency.Ratification != "ratified" || done.Emergency.RatifiedBy != "Owner" || done.Emergency.RatifiedAt == nil {
		t.Fatalf("pengesahan: %+v %v", done.Emergency, err)
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "reject", Notes: "berubah pikiran", ActorName: "Owner"}); err == nil {
		t.Fatal("keputusan kedua harus ditolak")
	}
	if _, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{ProofURL: "/uploads/lain.jpg", ProofTotal: 1}); err == nil {
		t.Fatal("bukti setelah disahkan tidak boleh diganti")
	}
	// Jasa diselesaikan seperti biasa.
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "receive", ActorName: "Manajer Uji"}); err != nil {
		t.Fatalf("selesai jasa: %v", err)
	}
}

func TestEmergencyPurchaseLimitAndMonthlyCap(t *testing.T) {
	requireDB(t)
	_, unit := emergencyTestUnit(t)
	t.Cleanup(func() {
		UpdateEmergencySettings(models.EmergencySettings{MaxAmount: 500000})
	})

	if _, err := CreateEmergencyPurchase(emergencyInput(unit, 500001)); err == nil || !strings.Contains(err.Error(), "melebihi batas") {
		t.Fatalf("di atas Rp500.000 harus ditolak: %v", err)
	}
	two := emergencyInput(unit, 0)
	two.Items = []models.EmergencyPurchaseItemInput{{Name: "Kabel", Qty: 3, Price: 100000}, {Name: "Saklar", Qty: 2, Price: 100001}}
	if _, err := CreateEmergencyPurchase(two); err == nil || !strings.Contains(err.Error(), "melebihi batas") {
		t.Fatalf("total gabungan di atas batas harus ditolak: %v", err)
	}
	for _, bad := range []func(*models.CreateEmergencyPurchaseInput){
		func(in *models.CreateEmergencyPurchaseInput) { in.RequestType = "bahan" },
		func(in *models.CreateEmergencyPurchaseInput) { in.Category = "asal" },
		func(in *models.CreateEmergencyPurchaseInput) { in.Reason = "rusak" },
		func(in *models.CreateEmergencyPurchaseInput) { in.RecipientName = " " },
		func(in *models.CreateEmergencyPurchaseInput) { in.Items = nil },
		func(in *models.CreateEmergencyPurchaseInput) { in.Items[0].Qty = 0 },
		func(in *models.CreateEmergencyPurchaseInput) { in.Items[0].Price = 0 },
		func(in *models.CreateEmergencyPurchaseInput) { in.WorkUnitID = "" },
	} {
		in := emergencyInput(unit, 1000)
		bad(&in)
		if _, err := CreateEmergencyPurchase(in); err == nil {
			t.Fatalf("isian tidak sah harus ditolak: %+v", in)
		}
	}

	first, err := CreateEmergencyPurchase(emergencyInput(unit, 500000))
	if err != nil || first.TotalFinal != 500000 {
		t.Fatalf("tepat Rp500.000 harus diterima: %v", err)
	}

	// Plafon bulanan: plafon < batas per pembelian tidak masuk akal.
	if _, err := UpdateEmergencySettings(models.EmergencySettings{MaxAmount: 500000, MonthlyCap: 100000}); err == nil {
		t.Fatal("plafon di bawah batas per pembelian harus ditolak")
	}
	s, err := UpdateEmergencySettings(models.EmergencySettings{MaxAmount: 500000, MonthlyCap: 600000})
	if err != nil || s.MonthlyCap != 600000 {
		t.Fatalf("simpan plafon: %+v %v", s, err)
	}
	ctx, err := GetEmergencyContext(unit)
	if err != nil || ctx.UsedThisMonth != 500000 || ctx.CountThisMonth != 1 || ctx.Remaining == nil || *ctx.Remaining != 100000 {
		t.Fatalf("pemakaian bulan ini: %+v %v", ctx, err)
	}
	if _, err := CreateEmergencyPurchase(emergencyInput(unit, 200000)); err == nil || !strings.Contains(err.Error(), "plafon") {
		t.Fatalf("melewati plafon bulanan harus ditolak: %v", err)
	}
	// Yang dibatalkan tidak menghabiskan plafon.
	if _, err := CancelEmergencyPurchase(first.ID, models.EmergencyDecisionInput{Notes: "salah input nominal", ActorName: "Manajer Uji"}); err != nil {
		t.Fatalf("batal: %v", err)
	}
	if _, err := CreateEmergencyPurchase(emergencyInput(unit, 200000)); err != nil {
		t.Fatalf("setelah dibatalkan plafon harus lega lagi: %v", err)
	}
}

func TestEmergencyPurchaseCancelReturnAndResend(t *testing.T) {
	requireDB(t)
	outlet, unit := emergencyTestUnit(t)

	pr, err := CreateEmergencyPurchase(emergencyInput(unit, 150000))
	if err != nil {
		t.Fatalf("buat: %v", err)
	}
	task := openPayoutFor(t, outlet, pr.ID)
	if _, err := CancelEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Notes: "x", ActorName: "Manajer Uji"}); err == nil {
		t.Fatal("batal tanpa alasan cukup harus ditolak")
	}
	// Kasir mengembalikan tugas (kas laci kurang) → kirim ulang.
	if _, err := ReturnCashPayout(outlet, task.ID, models.CashPayoutDeviceInput{Reason: "Kas laci tidak cukup", CashierName: "Kasir Uji"}); err != nil {
		t.Fatalf("kasir mengembalikan: %v", err)
	}
	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "pay", ActorName: "Keuangan"}); err == nil ||
		!strings.Contains(err.Error(), "tunai dari Kas Kasir Outlet") {
		t.Fatalf("transfer Kantor Pusat untuk Pembelian Darurat harus ditolak: %v", err)
	}
	if _, err := ResendEmergencyPayout(pr.ID, models.EmergencyDecisionInput{ActorName: "Manajer Uji"}); err != nil {
		t.Fatalf("kirim ulang: %v", err)
	}
	if _, err := ResendEmergencyPayout(pr.ID, models.EmergencyDecisionInput{ActorName: "Manajer Uji"}); err == nil {
		t.Fatal("kirim ulang saat tugas masih terbuka harus ditolak")
	}
	again := openPayoutFor(t, outlet, pr.ID)
	if again.ID == task.ID || again.Amount != 150000 || again.RecipientName != "Rahmad" {
		t.Fatalf("tugas baru: %+v", again)
	}
	// Keuangan juga bisa menetapkan ulang tanpa vendor terdaftar.
	if _, err := CancelCashPayout(again.ID, models.CashPayoutResolveInput{ActorName: "Keuangan", Reason: "ditarik"}); err != nil {
		t.Fatalf("tarik tugas: %v", err)
	}
	if p, err := AssignCashPayout(models.AssignCashPayoutInput{PurchaseRequestID: pr.ID, ActorName: "Keuangan"}); err != nil || p.RecipientName != "Rahmad" {
		t.Fatalf("keuangan menetapkan ulang: %+v %v", p, err)
	}

	// Sedang diproses kasir → tidak bisa dibatalkan.
	cur := openPayoutFor(t, outlet, pr.ID)
	if _, err := ClaimCashPayout(outlet, cur.ID, device(movementID())); err != nil {
		t.Fatalf("klaim: %v", err)
	}
	if _, err := CancelEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Notes: "tidak jadi dibeli", ActorName: "Manajer Uji"}); err == nil {
		t.Fatal("batal saat kasir memproses harus ditolak")
	}

	// Pembelian Darurat lain dibatalkan sebelum diproses: tugasnya ikut ditarik.
	other, err := CreateEmergencyPurchase(emergencyInput(unit, 90000))
	if err != nil {
		t.Fatalf("buat kedua: %v", err)
	}
	otherTask := openPayoutFor(t, outlet, other.ID)
	cancelled, err := CancelEmergencyPurchase(other.ID, models.EmergencyDecisionInput{Notes: "genset sudah hidup lagi", ActorName: "Manajer Uji"})
	if err != nil || cancelled.Status != "cancelled" || cancelled.Emergency == nil || cancelled.Emergency.Ratification != "" {
		t.Fatalf("batal: %+v %v", cancelled, err)
	}
	if p, _ := GetCashPayout(otherTask.ID); p == nil || p.Status != "cancelled" {
		t.Fatalf("tugas kas harus ikut ditarik: %+v", p)
	}
	if _, err := DecideEmergencyPurchase(other.ID, models.EmergencyDecisionInput{Decision: "ratify", ActorName: "Owner"}); err == nil {
		t.Fatal("yang dibatalkan tidak bisa disahkan")
	}
}

func TestPurchaseListUrgentFirstAndFilters(t *testing.T) {
	requireDB(t)
	outlet, unit := emergencyTestUnit(t)
	marker := "Urutan" + NewULID()[18:]
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	mk := func(name, urgency, notes string) *models.PurchaseRequest {
		in := models.CreatePurchaseRequestInput{
			RequestType: "jasa", RequestedBy: "tester", OutletID: outlet, Urgency: urgency, Notes: notes,
			Items: []models.PurchaseRequestItem{{Name: marker + " " + name,
				Items: []models.PurchaseSubItem{{Name: marker + " " + name, Qty: 1, Unit: "kali", HpsPrice: 1000}}}},
		}
		if urgency != "" && urgency != "rutin" {
			in.NeedByDate = today
		}
		pr, err := CreatePurchaseRequest(in)
		if err != nil {
			t.Fatalf("buat %s: %v", name, err)
		}
		return pr
	}
	urgent := mk("urgent", "urgent", "Pompa air mati, toilet tidak bisa dipakai")
	prior := mk("prioritas", "prioritas", "")
	rutin := mk("rutin", "", "")
	if rutin.Urgency != "rutin" || rutin.NeedByDate != nil || urgent.NeedByDate == nil || *urgent.NeedByDate != today {
		t.Fatalf("tingkat tersimpan: rutin=%+v urgent=%v", rutin.Urgency, urgent.NeedByDate)
	}
	emerg, err := CreateEmergencyPurchase(emergencyInput(unit, 50000))
	if err != nil {
		t.Fatalf("darurat: %v", err)
	}

	res, err := ListPurchaseRequestsExt("", "", "", "", "", "", false, marker, nil, nil, nil, 1, 20, PurchaseListExtra{UrgentFirst: true})
	if err != nil || len(res.Requests) != 3 {
		t.Fatalf("daftar: %v %+v", err, res)
	}
	got := []string{res.Requests[0].ID, res.Requests[1].ID, res.Requests[2].ID}
	want := []string{urgent.ID, prior.ID, rutin.ID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("urutan antrean: dapat %v, mau %v", got, want)
		}
	}
	plain, _ := ListPurchaseRequests("", "", "", "", "", "", false, marker, nil, nil, nil, 1, 20)
	if len(plain.Requests) != 3 || plain.Requests[0].ID != rutin.ID {
		t.Fatalf("tanpa UrgentFirst tetap terbaru dulu: %+v", plain.Requests)
	}
	only, _ := ListPurchaseRequestsExt("", "", "", "", "", "", false, marker, nil, nil, nil, 1, 20, PurchaseListExtra{Urgency: "urgent"})
	if len(only.Requests) != 1 || only.Requests[0].ID != urgent.ID {
		t.Fatalf("saring urgent: %+v", only.Requests)
	}
	pending, _ := ListPurchaseRequestsExt("", "", "", "", "", "", false, "", []string{outlet}, nil, nil, 1, 20, PurchaseListExtra{Ratification: "pending"})
	if len(pending.Requests) != 1 || pending.Requests[0].ID != emerg.ID || pending.Requests[0].Emergency == nil {
		t.Fatalf("saring menunggu pengesahan: %+v", pending.Requests)
	}
}

func TestStaleAndEmergencyReminderCandidates(t *testing.T) {
	requireDB(t)
	outlet, unit := emergencyTestUnit(t)
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	mk := func(urgency, notes string) string {
		in := models.CreatePurchaseRequestInput{
			RequestType: "jasa", RequestedBy: "tester", OutletID: outlet, Urgency: urgency, Notes: notes,
			Items: []models.PurchaseRequestItem{{Name: "Pengingat", Items: []models.PurchaseSubItem{{Name: "Jasa", Qty: 1, Unit: "kali", HpsPrice: 1000}}}},
		}
		if urgency != "rutin" {
			in.NeedByDate = today
		}
		pr, err := CreatePurchaseRequest(in)
		if err != nil {
			t.Fatalf("buat %s: %v", urgency, err)
		}
		return pr.ID
	}
	urgent, prior, rutin := mk("urgent", "Freezer mati, daging bisa rusak"), mk("prioritas", ""), mk("rutin", "")
	now := time.Now()
	// Semua tertahan 5 jam: hanya urgent yang lewat batasnya.
	execScratch(`UPDATE purchase_requests SET updated_at = $2 WHERE id = ANY(string_to_array($1, ','))`,
		urgent+","+prior+","+rutin, now.UTC().Add(-5*time.Hour))
	inList := func(list []stalePurchase, id string) bool {
		for _, s := range list {
			if s.id == id {
				return true
			}
		}
		return false
	}
	list, err := stalePurchaseCandidates(now)
	if err != nil {
		t.Fatalf("kandidat tertahan: %v", err)
	}
	if !inList(list, urgent) || inList(list, prior) || inList(list, rutin) {
		t.Fatalf("5 jam: hanya urgent yang diingatkan")
	}
	execScratch(`UPDATE purchase_requests SET updated_at = $2 WHERE id = ANY(string_to_array($1, ','))`,
		prior+","+rutin, now.UTC().Add(-30*time.Hour))
	if list, _ = stalePurchaseCandidates(now); !inList(list, prior) || inList(list, rutin) {
		t.Fatalf("30 jam: prioritas diingatkan, rutin belum")
	}

	// Pembelian Darurat tidak masuk pengingat biasa; punya pengingat sendiri.
	pr, err := CreateEmergencyPurchase(emergencyInput(unit, 75000))
	if err != nil {
		t.Fatalf("darurat: %v", err)
	}
	execScratch(`UPDATE purchase_requests SET updated_at = $2 WHERE id = $1`, pr.ID, now.UTC().Add(-72*time.Hour))
	if list, _ = stalePurchaseCandidates(now); inList(list, pr.ID) {
		t.Fatal("Pembelian Darurat tidak boleh masuk pengingat biasa")
	}
	followups := func() map[string]bool {
		out, err := emergencyFollowupCandidates(now)
		if err != nil {
			t.Fatalf("kandidat darurat: %v", err)
		}
		m := map[string]bool{}
		for _, f := range out {
			if f.id == pr.ID {
				m["ada"] = true
				m["tanpa_struk"] = f.noProof
			}
		}
		return m
	}
	if followups()["ada"] {
		t.Fatal("belum ada kas keluar: belum perlu diingatkan")
	}
	handCash(t, outlet, openPayoutFor(t, outlet, pr.ID).ID)
	execScratch(`UPDATE purchase_requests SET paid_at = $2 WHERE id = $1`, pr.ID, now.UTC().Add(-26*time.Hour))
	if f := followups(); !f["ada"] || !f["tanpa_struk"] {
		t.Fatalf("struk belum diunggah > 1 hari harus diingatkan: %v", f)
	}
	if _, err := SubmitEmergencyProof(pr.ID, models.EmergencyProofInput{ProofURL: "/uploads/struk.jpg", ProofTotal: 75000}); err != nil {
		t.Fatalf("bukti: %v", err)
	}
	if followups()["ada"] {
		t.Fatal("struk baru diunggah: penyetuju belum perlu diingatkan")
	}
	execScratch(`UPDATE purchase_requests SET emergency_proof_at = $2 WHERE id = $1`, pr.ID, now.UTC().Add(-26*time.Hour))
	if f := followups(); !f["ada"] || f["tanpa_struk"] {
		t.Fatalf("berstruk > 1 hari belum disahkan harus diingatkan ke penyetuju: %v", f)
	}
	if _, err := DecideEmergencyPurchase(pr.ID, models.EmergencyDecisionInput{Decision: "reject", Notes: "harga terlalu mahal", ActorName: "Owner"}); err != nil {
		t.Fatalf("tolak: %v", err)
	}
	if followups()["ada"] {
		t.Fatal("sudah diputus: tidak diingatkan lagi")
	}
}
