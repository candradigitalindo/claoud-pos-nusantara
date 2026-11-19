package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cloud-pos/database"
	"cloud-pos/models"
)

// ── Tingkat kebutuhan & Pembelian Darurat ──────────────────────────────────
//
// Keputusan owner 19 Nov 2026:
//
//	RUTIN      kebutuhan terencana — alur penuh seperti biasa.
//	PRIORITAS  dibutuhkan dalam beberapa hari — alur penuh, tanggal butuh
//	           wajib, tampil paling atas di antrean, pengingat lebih cepat.
//	URGENT     operasional berhenti / keselamatan / keamanan pangan:
//	           - ≤ batas (bawaan Rp500.000): PEMBELIAN DARURAT — dibeli dulu
//	             dari kas laci outlet lewat tugas Bayar Pengadaan di tablet
//	             (PIN Manager/SVP), struk diunggah sesudahnya, lalu disahkan
//	             pemegang procurement.requests.approve;
//	           - di atas batas: pengajuan URGENT biasa yang disetujui dulu.
//
// Pembelian Darurat lahir 'payment_requested' (komitmen sudah terjadi menurut
// kebijakan, approved_at = saat dicatat) sehingga jalur bayar, pembukuan, dan
// serah terima barang tetap memakai mesin yang sama dengan pengajuan biasa.
// Pengesahan berdiri sendiri (ratification_status) dan tidak membalik
// pembukuan: uangnya memang sudah keluar.

const (
	UrgencyRutin     = "rutin"
	UrgencyPrioritas = "prioritas"
	UrgencyUrgent    = "urgent"
)

// UrgencyLabel: label tingkat kebutuhan untuk layar & WhatsApp.
func UrgencyLabel(u string) string {
	switch u {
	case UrgencyPrioritas:
		return "Prioritas"
	case UrgencyUrgent:
		return "Urgent"
	}
	return "Rutin"
}

// urgentMaxLeadDays: kebutuhan URGENT paling lambat 7 hari lagi; selebihnya
// pilih PRIORITAS.
const urgentMaxLeadDays = 7

// urgentNotesMin: pengajuan URGENT wajib menjelaskan dampaknya.
const urgentNotesMin = 10

// normalizeUrgency memeriksa tingkat kebutuhan pengajuan baru. Hasil: urgensi
// yang sah dan tanggal butuh (YYYY-MM-DD) atau nil.
func normalizeUrgency(urgency, needBy, notes string, now time.Time) (string, *string, error) {
	u := strings.ToLower(strings.TrimSpace(urgency))
	if u == "" {
		u = UrgencyRutin
	}
	if u != UrgencyRutin && u != UrgencyPrioritas && u != UrgencyUrgent {
		return "", nil, Invalid("tingkat kebutuhan harus rutin, prioritas, atau urgent")
	}
	needBy = strings.TrimSpace(needBy)
	if needBy == "" {
		if u != UrgencyRutin {
			return "", nil, Invalid("tanggal dibutuhkan wajib diisi untuk pengajuan %s", strings.ToUpper(UrgencyLabel(u)))
		}
		return u, nil, nil
	}
	loc := GetTimezoneLocation()
	d, err := time.ParseInLocation("2006-01-02", needBy, loc)
	if err != nil {
		return "", nil, Invalid("tanggal dibutuhkan tidak valid (format YYYY-MM-DD)")
	}
	l := now.In(loc)
	today := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	if d.Before(today) {
		return "", nil, Invalid("tanggal dibutuhkan tidak boleh sebelum hari ini")
	}
	if u == UrgencyUrgent {
		if d.After(today.AddDate(0, 0, urgentMaxLeadDays)) {
			return "", nil, Invalid("pengajuan URGENT untuk kebutuhan paling lambat %d hari lagi — selebihnya pilih PRIORITAS", urgentMaxLeadDays)
		}
		if utf8.RuneCountInString(strings.TrimSpace(notes)) < urgentNotesMin {
			return "", nil, Invalid("pengajuan URGENT wajib menjelaskan dampaknya di Catatan (minimal %d huruf), mis. mesin mati dan operasional terhenti", urgentNotesMin)
		}
	}
	s := d.Format("2006-01-02")
	return u, &s, nil
}

// ── Kategori darurat ───────────────────────────────────────────────────────

type emergencyCategory struct{ Key, Label string }

// EmergencyCategories: kriteria Pembelian Darurat, urutan tampilan.
var EmergencyCategories = []emergencyCategory{
	{"peralatan_rusak", "Peralatan operasional rusak/mati"},
	{"bangunan", "Kerusakan bangunan/instalasi"},
	{"keselamatan", "Keselamatan & keamanan"},
	{"keamanan_pangan", "Keamanan pangan"},
	{"lainnya", "Lainnya"},
}

// EmergencyCategoryLabel: label kategori; "" bila tidak dikenal.
func EmergencyCategoryLabel(key string) string {
	for _, c := range EmergencyCategories {
		if c.Key == key {
			return c.Label
		}
	}
	return ""
}

// emergencyReasonMin: alasan darurat minimal (apa yang berhenti bila tidak
// dibeli sekarang).
const emergencyReasonMin = 10

// emergencyMaxLines: baris barang/jasa per Pembelian Darurat.
const emergencyMaxLines = 20

// ── Pengaturan batas ───────────────────────────────────────────────────────

const (
	settingEmergencyMax  = "procurement_emergency_max"
	settingEmergencyCap  = "procurement_emergency_monthly_cap"
	defaultEmergencyMax  = 500000
	emergencyMaxCeiling  = 10000000
	emergencyCapCeiling  = 1000000000
	emergencyApprovedTag = "Pembelian Darurat"
)

// GetEmergencySettings membaca batas Pembelian Darurat. Nilai rusak/kosong →
// bawaan (Rp500.000, tanpa plafon) supaya batas tidak pernah hilang diam-diam.
func GetEmergencySettings() models.EmergencySettings {
	vals, _ := GetSettingsByKeys([]string{settingEmergencyMax, settingEmergencyCap})
	s := models.EmergencySettings{MaxAmount: defaultEmergencyMax}
	if v, err := strconv.ParseFloat(strings.TrimSpace(vals[settingEmergencyMax]), 64); err == nil && v > 0 && v <= emergencyMaxCeiling {
		s.MaxAmount = math.Round(v)
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(vals[settingEmergencyCap]), 64); err == nil && v > 0 && v <= emergencyCapCeiling {
		s.MonthlyCap = math.Round(v)
	}
	return s
}

// UpdateEmergencySettings menyimpan batas baru (rupiah bulat).
func UpdateEmergencySettings(in models.EmergencySettings) (models.EmergencySettings, error) {
	maxAmount, monthlyCap := math.Round(in.MaxAmount), math.Round(in.MonthlyCap)
	if math.IsNaN(maxAmount) || maxAmount < 1 || maxAmount > emergencyMaxCeiling {
		return models.EmergencySettings{}, Invalid("batas per pembelian harus antara Rp1 dan %s", formatRupiahPlain(emergencyMaxCeiling*100))
	}
	if math.IsNaN(monthlyCap) || monthlyCap < 0 || monthlyCap > emergencyCapCeiling {
		return models.EmergencySettings{}, Invalid("plafon bulanan tidak valid")
	}
	if monthlyCap > 0 && monthlyCap < maxAmount {
		return models.EmergencySettings{}, Invalid("plafon bulanan harus 0 (tanpa plafon) atau minimal sebesar batas per pembelian")
	}
	if err := UpdateSettings(map[string]string{
		settingEmergencyMax: strconv.FormatFloat(maxAmount, 'f', 0, 64),
		settingEmergencyCap: strconv.FormatFloat(monthlyCap, 'f', 0, 64),
	}); err != nil {
		return models.EmergencySettings{}, err
	}
	return GetEmergencySettings(), nil
}

// emergencyMonthWindow: awal & akhir bulan berjalan menurut zona aplikasi,
// dalam UTC (kolom created_at disimpan UTC tanpa zona).
func emergencyMonthWindow(now time.Time) (time.Time, time.Time) {
	loc := GetTimezoneLocation()
	l := now.In(loc)
	start := time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc)
	return start.UTC(), start.AddDate(0, 1, 0).UTC()
}

// emergencyUsage: nilai & jumlah Pembelian Darurat outlet bulan berjalan yang
// tidak dibatalkan/ditolak.
func emergencyUsage(q rowQuerier, outletID string, now time.Time) (float64, int, error) {
	from, to := emergencyMonthWindow(now)
	var used float64
	var n int
	err := q.QueryRow(`
		SELECT COALESCE(SUM(total_final), 0), COUNT(*) FROM purchase_requests
		WHERE emergency AND outlet_id = $1 AND status NOT IN ('cancelled', 'rejected')
		  AND created_at >= $2 AND created_at < $3`, outletID, from, to).Scan(&used, &n)
	return used, n, err
}

// ── Kesiapan tablet ────────────────────────────────────────────────────────

// payoutMinVersionCode: App POS v10.20 — versi pertama yang punya menu Bayar
// Pengadaan (tugas Kas Kasir Outlet).
const payoutMinVersionCode = 95013423

// appVersionCode membaca versionCode dari app_version "1.2.0+95013423".
func appVersionCode(v string) int {
	i := strings.LastIndex(v, "+")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(v[i+1:]))
	if err != nil {
		return 0
	}
	return n
}

// outletTabletReady: tablet kasir outlet (heartbeat terakhir) sudah bisa
// memproses tugas Bayar Pengadaan.
func outletTabletReady(outletID string) (bool, string, string) {
	var version string
	var received sql.NullTime
	err := database.DB.QueryRow(`SELECT COALESCE(app_version, ''), received_at FROM device_heartbeats WHERE outlet_id = $1`, outletID).
		Scan(&version, &received)
	if err != nil {
		return false, "", "Belum ada laporan dari tablet kasir outlet ini — pastikan App POS v10.20 terpasang dan tablet online."
	}
	if appVersionCode(version) < payoutMinVersionCode {
		return false, version, "Tablet kasir outlet ini belum memakai App POS v10.20, jadi tugas kas belum akan muncul di sana. Perbarui tabletnya dulu."
	}
	if received.Valid && time.Since(received.Time) > 30*time.Minute {
		mins := int(time.Since(received.Time).Minutes())
		label := fmt.Sprintf("%d menit", mins)
		if mins >= 120 {
			label = fmt.Sprintf("%d jam", mins/60)
		}
		return true, version, "Tablet kasir terakhir melapor " + label + " lalu — tugas kas baru muncul saat tablet online dan layar kasir terbuka."
	}
	return true, version, ""
}

// ── Konteks form ───────────────────────────────────────────────────────────

// GetEmergencyContext: batas, pemakaian bulan berjalan, dan kesiapan tablet
// untuk outlet milik unit kerja ini.
func GetEmergencyContext(workUnitID string) (*models.EmergencyContext, error) {
	if strings.TrimSpace(workUnitID) == "" {
		return nil, Invalid("pilih unit kerja outlet")
	}
	outletID, _, err := resolvePurchaseOwner("", workUnitID)
	if err != nil {
		return nil, err
	}
	if outletID == "" {
		return nil, Invalid("Pembelian Darurat hanya untuk unit kerja outlet — kasnya diambil dari laci kasir outlet")
	}
	ctx := &models.EmergencyContext{EmergencySettings: GetEmergencySettings(), OutletID: outletID}
	var active bool
	if err := database.DB.QueryRow(`SELECT COALESCE(name, ''), is_active FROM outlets WHERE id = $1`, outletID).
		Scan(&ctx.OutletName, &active); err != nil {
		return nil, Invalid("outlet tidak ditemukan")
	}
	if !active {
		return nil, Invalid("outlet %s tidak aktif", ctx.OutletName)
	}
	if ctx.UsedThisMonth, ctx.CountThisMonth, err = emergencyUsage(database.DB, outletID, time.Now()); err != nil {
		return nil, err
	}
	if ctx.MonthlyCap > 0 {
		rem := math.Max(0, ctx.MonthlyCap-ctx.UsedThisMonth)
		ctx.Remaining = &rem
	}
	ctx.TabletReady, ctx.TabletVersion, ctx.TabletNote = outletTabletReady(outletID)
	return ctx, nil
}

// ── Pembuatan ──────────────────────────────────────────────────────────────

// emergencyGroupName: "DARURAT: Steam valve, Jasa pasang" — nama pengadaan
// yang juga tampil sebagai ringkasan di tablet kasir.
func emergencyGroupName(items []models.PurchaseSubItem) string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return truncateRunes("DARURAT: "+strings.Join(names, ", "), maxGoodsGroupNameLen)
}

// emergencyPayoutNotes: catatan tugas kas untuk kasir.
func emergencyPayoutNotes(categoryLabel, reason string) string {
	return truncateRunes("PEMBELIAN DARURAT · "+categoryLabel+": "+reason, 1000)
}

// CreateEmergencyPurchase mencatat Pembelian Darurat dan mengirim tugas kas
// ke tablet kasir outlet. Uang belum keluar sampai kasir menyerahkannya
// dengan PIN Manager/SVP.
func CreateEmergencyPurchase(in models.CreateEmergencyPurchaseInput) (*models.PurchaseRequest, error) {
	reqType := strings.TrimSpace(in.RequestType)
	if reqType != RequestTypeBarang && reqType != RequestTypeJasa {
		return nil, Invalid("Pembelian Darurat hanya untuk barang atau jasa")
	}
	category := strings.TrimSpace(in.Category)
	categoryLabel := EmergencyCategoryLabel(category)
	if categoryLabel == "" {
		return nil, Invalid("pilih kategori darurat")
	}
	reason, err := limitText(in.Reason, 1000, "alasan darurat")
	if err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(reason) < emergencyReasonMin {
		return nil, Invalid("jelaskan alasan daruratnya (minimal %d huruf): apa yang berhenti bila tidak dibeli sekarang", emergencyReasonMin)
	}
	recipient, err := limitText(in.RecipientName, 200, "nama penerima kas")
	if err != nil {
		return nil, err
	}
	if recipient == "" {
		return nil, Invalid("nama staf yang menerima kas wajib diisi")
	}
	shop, err := limitText(in.ShopName, 200, "nama toko/teknisi")
	if err != nil {
		return nil, err
	}
	notes, err := limitText(in.Notes, 1000, "catatan")
	if err != nil {
		return nil, err
	}
	if len(in.Items) == 0 {
		return nil, Invalid("isi minimal satu barang/jasa")
	}
	if len(in.Items) > emergencyMaxLines {
		return nil, Invalid("Pembelian Darurat maksimal %d baris", emergencyMaxLines)
	}
	source := shop
	if source == "" {
		source = "Harga toko/teknisi (Pembelian Darurat)"
	}
	subs := make([]models.PurchaseSubItem, 0, len(in.Items))
	for _, it := range in.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			return nil, Invalid("nama barang/jasa wajib diisi")
		}
		if it.Qty <= 0 {
			return nil, Invalid("jumlah %q harus lebih dari 0", name)
		}
		price := math.Round(it.Price)
		if price <= 0 || math.IsNaN(it.Price) || math.IsInf(it.Price, 0) {
			return nil, Invalid("harga %q harus lebih dari 0 — isi harga yang sudah dikonfirmasi ke toko/teknisi", name)
		}
		unit := strings.TrimSpace(it.Unit)
		if unit == "" {
			unit = "pcs"
			if reqType == RequestTypeJasa {
				unit = "kali"
			}
		}
		subs = append(subs, models.PurchaseSubItem{
			Name: name, Qty: it.Qty, Unit: unit,
			HpsPrice: price, HpsSource: source, FinalPrice: price,
		})
	}
	items := []models.PurchaseRequestItem{{Name: emergencyGroupName(subs), Items: subs}}
	if err := validateItems(items); err != nil {
		return nil, Invalid("%s", err.Error())
	}
	if err := validateReceivableItemText(items, reqType); err != nil {
		return nil, err
	}
	goodsKind, err := stampPurchaseItemKinds(items, reqType)
	if err != nil {
		return nil, err
	}
	totalHps, totalFinal := recalcItems(items)
	settings := GetEmergencySettings()
	if cents(totalFinal) > cents(settings.MaxAmount) {
		return nil, Invalid("total %s melebihi batas Pembelian Darurat %s — ajukan sebagai pengajuan URGENT biasa supaya disetujui dulu",
			formatRupiahPlain(cents(totalFinal)), formatRupiahPlain(cents(settings.MaxAmount)))
	}
	if strings.TrimSpace(in.WorkUnitID) == "" {
		return nil, Invalid("pilih unit kerja outlet")
	}
	outletID, workUnitID, err := resolvePurchaseOwner("", in.WorkUnitID)
	if err != nil {
		return nil, err
	}
	if outletID == "" {
		return nil, Invalid("Pembelian Darurat hanya untuk unit kerja outlet — kasnya diambil dari laci kasir outlet")
	}
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan item: %w", err)
	}

	id := NewULID()
	now := time.Now().UTC()
	today := now.In(GetTimezoneLocation()).Format("2006-01-02")
	approvedBy := truncateRunes(emergencyApprovedTag+" (kebijakan ≤ "+formatRupiahPlain(cents(settings.MaxAmount))+")", 100)
	actorID := nilIfEmpty(validActorID(in.RequestedByID))
	err = insertWithRequestNumber(now, func(reqNumber string) error {
		tx, err := database.DB.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		// Kunci baris outlet: dua Pembelian Darurat bersamaan dari outlet
		// yang sama tidak boleh sama-sama lolos plafon bulanan.
		var outletName string
		var active bool
		if err := tx.QueryRow(`SELECT COALESCE(name, ''), is_active FROM outlets WHERE id = $1 FOR UPDATE`, outletID).
			Scan(&outletName, &active); err != nil {
			return Invalid("outlet tidak ditemukan")
		}
		if !active {
			return Invalid("outlet %s tidak aktif", outletName)
		}
		if settings.MonthlyCap > 0 {
			used, _, err := emergencyUsage(tx, outletID, now)
			if err != nil {
				return err
			}
			if cents(used)+cents(totalFinal) > cents(settings.MonthlyCap) {
				left := cents(settings.MonthlyCap) - cents(used)
				if left < 0 {
					left = 0
				}
				return Invalid("plafon Pembelian Darurat %s bulan ini %s, sudah terpakai %s (sisa %s) — ajukan sebagai pengajuan URGENT biasa",
					outletName, formatRupiahPlain(cents(settings.MonthlyCap)), formatRupiahPlain(cents(used)), formatRupiahPlain(left))
			}
		}
		payTo := truncateRunes("Tunai dari Kas Kasir "+outletName+" — diserahkan kepada "+recipient, 300)
		if _, err := tx.Exec(`
			INSERT INTO purchase_requests (id, request_number, outlet_id, work_unit_id, request_type, requested_by, requested_by_id,
				vendor_name, status, items, total_amount, total_hps, total_final, notes, goods_kind,
				approved_by, approved_at, urgency, need_by_date, emergency, emergency_category, emergency_reason,
				emergency_recipient, ratification_status,
				pay_to_method, pay_to_text, pay_to_set_by, pay_to_set_by_id, pay_to_set_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'payment_requested', $9, $10, $10, $11, $12, $13,
				$14, $15, 'urgent', $16, TRUE, $17, $18, $19, 'pending',
				'manual', $20, $6, $7, $15, $15, $15)`,
			id, reqNumber, outletID, nilIfEmpty(workUnitID), reqType, in.RequestedBy, actorID,
			shop, itemsJSON, totalHps, totalFinal, notes, goodsKind,
			approvedBy, now, today, category, reason, recipient, payTo); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO procurement_cash_payouts
			(id, purchase_request_id, outlet_id, amount, recipient_name, notes, status, assigned_by, assigned_by_id, assigned_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9, $9)`,
			NewULID(), id, outletID, totalFinal, recipient, emergencyPayoutNotes(categoryLabel, reason),
			in.RequestedBy, actorID, now); err != nil {
			return err
		}
		return tx.Commit()
	})
	if err != nil {
		return nil, err
	}
	pr, err := GetPurchaseRequest(id)
	if err == nil {
		go NotifyPurchaseEvent(pr, "emergency_created", in.RequestedBy, "", totalFinal)
	}
	return pr, err
}

// ── Langkah sesudahnya ─────────────────────────────────────────────────────

// lockedEmergency: potret Pembelian Darurat yang dikunci.
type lockedEmergency struct {
	status, ratification, proofURL, outletID, category, reason, recipient string
	emergency                                                             bool
	totalFinal, paid                                                      float64
	updatedAt                                                             time.Time
}

func lockEmergencyTx(tx *sql.Tx, id string) (*lockedEmergency, error) {
	e := &lockedEmergency{}
	err := tx.QueryRow(`
		SELECT status, emergency, ratification_status, emergency_proof_url, COALESCE(outlet_id, ''),
		       emergency_category, emergency_reason, emergency_recipient,
		       COALESCE(total_final, 0), COALESCE(paid_amount, 0), updated_at
		FROM purchase_requests WHERE id = $1 FOR UPDATE`, id).
		Scan(&e.status, &e.emergency, &e.ratification, &e.proofURL, &e.outletID,
			&e.category, &e.reason, &e.recipient, &e.totalFinal, &e.paid, &e.updatedAt)
	if err == sql.ErrNoRows {
		return nil, Invalid("pengajuan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if !e.emergency {
		return nil, Invalid("pengajuan ini bukan Pembelian Darurat")
	}
	e.outletID = strings.TrimSpace(e.outletID)
	return e, nil
}

func (e *lockedEmergency) checkVersion(version string) error {
	if v := strings.TrimSpace(version); v != "" && !versionMatches(v, e.updatedAt) {
		return Conflict(purchaseChangedMsg)
	}
	return nil
}

// cashHanded: kas sudah diserahkan kasir (pembayaran tercatat).
func (e *lockedEmergency) cashHanded() bool { return cents(e.paid) > 0 }

func ratificationLabel(s string) string {
	switch s {
	case "ratified":
		return "disahkan"
	case "rejected":
		return "ditolak pengesahannya"
	}
	return "menunggu pengesahan"
}

// SubmitEmergencyProof: pengaju mengunggah foto struk & total belanja setelah
// kas diserahkan. Bisa diganti selama belum diputus penyetuju.
func SubmitEmergencyProof(id string, in models.EmergencyProofInput) (*models.PurchaseRequest, error) {
	url := strings.TrimSpace(in.ProofURL)
	if url == "" {
		return nil, Invalid("foto struk wajib diunggah")
	}
	if err := validateUploadField("foto struk", url); err != nil {
		return nil, err
	}
	if in.ProofTotal <= 0 || math.IsNaN(in.ProofTotal) || math.IsInf(in.ProofTotal, 0) {
		return nil, Invalid("isi total yang tertera di struk")
	}
	notes, err := limitText(in.Notes, 1000, "catatan")
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	e, err := lockEmergencyTx(tx, id)
	if err != nil {
		return nil, err
	}
	if err := e.checkVersion(in.Version); err != nil {
		return nil, err
	}
	if e.status == "cancelled" || e.status == "rejected" {
		return nil, Invalid("Pembelian Darurat ini sudah dibatalkan")
	}
	if !e.cashHanded() {
		return nil, Invalid("kas belum diserahkan kasir — unggah struk setelah belanja")
	}
	if e.ratification != "pending" {
		return nil, Invalid("Pembelian Darurat ini sudah %s — buktinya tidak bisa diubah lagi", ratificationLabel(e.ratification))
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE purchase_requests SET emergency_proof_url = $2, emergency_proof_total = $3,
		emergency_proof_notes = $4, emergency_proof_by = $5, emergency_proof_at = $6, updated_at = $6
		WHERE id = $1`, id, url, math.Round(in.ProofTotal*100)/100, notes, truncateRunes(in.ActorName, 100), now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetPurchaseRequest(id)
}

// DecideEmergencyPurchase: penyetuju mengesahkan (ratify) atau menolak
// (reject) Pembelian Darurat yang kasnya sudah diserahkan. Penolakan tidak
// membalik pembukuan — uangnya sudah keluar — tetapi tercatat sebagai temuan
// yang ditindaklanjuti (mis. menjadi tanggungan manajer outlet).
func DecideEmergencyPurchase(id string, in models.EmergencyDecisionInput) (*models.PurchaseRequest, error) {
	decision := strings.TrimSpace(in.Decision)
	if decision != "ratify" && decision != "reject" {
		return nil, Invalid("keputusan harus ratify atau reject")
	}
	notes, err := limitText(in.Notes, 1000, "catatan")
	if err != nil {
		return nil, err
	}
	if decision == "reject" && utf8.RuneCountInString(notes) < VerifyReasonMinLen {
		return nil, Invalid("alasan menolak pengesahan wajib diisi (minimal %d huruf)", VerifyReasonMinLen)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	e, err := lockEmergencyTx(tx, id)
	if err != nil {
		return nil, err
	}
	if err := e.checkVersion(in.Version); err != nil {
		return nil, err
	}
	if e.ratification != "pending" {
		return nil, Conflict("Pembelian Darurat ini sudah %s", ratificationLabel(e.ratification))
	}
	if e.status == "cancelled" || e.status == "rejected" {
		return nil, Invalid("Pembelian Darurat ini sudah dibatalkan")
	}
	if !e.cashHanded() {
		return nil, Invalid("kas belum diserahkan kasir — belum ada belanja yang perlu disahkan")
	}
	if decision == "ratify" && strings.TrimSpace(e.proofURL) == "" {
		return nil, Invalid("foto struk belum diunggah pengaju — pengesahan butuh bukti belanja")
	}
	status := "ratified"
	if decision == "reject" {
		status = "rejected"
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE purchase_requests SET ratification_status = $2, ratified_by = $3, ratified_by_id = $4,
		ratified_at = $5, ratification_notes = $6, updated_at = $5 WHERE id = $1`,
		id, status, truncateRunes(in.ActorName, 100), nilIfEmpty(validActorID(in.ActorID)), now, notes); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	pr, err := GetPurchaseRequest(id)
	if err == nil {
		go NotifyPurchaseEvent(pr, "emergency_reviewed", in.ActorName, notes, 0)
	}
	return pr, err
}

// CancelEmergencyPurchase: dibatalkan sebelum kas diserahkan (tidak jadi
// dibeli, tablet belum diperbarui, dsb.). Tugas kas yang masih menunggu ikut
// ditarik; yang sedang diproses kasir tidak bisa.
func CancelEmergencyPurchase(id string, in models.EmergencyDecisionInput) (*models.PurchaseRequest, error) {
	reason, err := limitText(in.Notes, 1000, "alasan")
	if err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(reason) < VerifyReasonMinLen {
		return nil, Invalid("alasan pembatalan wajib diisi (minimal %d huruf)", VerifyReasonMinLen)
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	e, err := lockEmergencyTx(tx, id)
	if err != nil {
		return nil, err
	}
	if err := e.checkVersion(in.Version); err != nil {
		return nil, err
	}
	if e.status != "payment_requested" || e.cashHanded() {
		return nil, Invalid("kas sudah diserahkan atau pengajuan sudah ditutup — tidak bisa dibatalkan. Bila uangnya tidak jadi dipakai, kembalikan ke laci dan minta penyetuju menolak pengesahannya")
	}
	var payoutID, payoutStatus string
	switch err := tx.QueryRow(`SELECT id, status FROM procurement_cash_payouts
		WHERE purchase_request_id = $1 AND status IN ('pending', 'claimed') FOR UPDATE`, id).Scan(&payoutID, &payoutStatus); {
	case err == sql.ErrNoRows:
	case err != nil:
		return nil, err
	case payoutStatus == "claimed":
		return nil, Conflict("kasir sedang memproses penyerahan kasnya — tunggu selesai, atau minta kasir mengembalikan tugasnya")
	}
	now := time.Now().UTC()
	actor := truncateRunes(in.ActorName, 100)
	if payoutID != "" {
		if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status = 'cancelled', resolved_by = $2,
			resolved_at = $3, resolution_reason = $4, updated_at = $3 WHERE id = $1`,
			payoutID, actor, now, "Pembelian Darurat dibatalkan: "+reason); err != nil {
			return nil, err
		}
	}
	res, err := tx.Exec(`UPDATE purchase_requests SET status = 'cancelled', ratification_status = '',
		rejected_reason = $2, rejected_by = $3, rejected_by_id = $4, rejected_at = $5, updated_at = $5
		WHERE id = $1 AND status = 'payment_requested'`, id, reason, actor, nilIfEmpty(validActorID(in.ActorID)), now)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, Conflict(purchaseChangedMsg)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetPurchaseRequest(id)
}

// insertEmergencyPayoutTx: tugas kas baru (pending) untuk Pembelian Darurat
// yang belum dibayar — dipakai kirim ulang setelah tugas lama dikembalikan
// kasir atau ditarik.
func insertEmergencyPayoutTx(tx *sql.Tx, prID string, e *lockedEmergency, actorName, actorID string) error {
	var open int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM procurement_cash_payouts
		WHERE purchase_request_id = $1 AND status IN ('pending', 'claimed')`, prID).Scan(&open); err != nil {
		return err
	}
	if open > 0 {
		return Conflict("tugas kas Pembelian Darurat ini masih terbuka di tablet kasir")
	}
	remaining := cents(e.totalFinal) - cents(e.paid)
	if e.status != "payment_requested" || remaining <= 0 {
		return Invalid("Pembelian Darurat ini tidak lagi menunggu kas")
	}
	var active bool
	if err := tx.QueryRow(`SELECT is_active FROM outlets WHERE id = $1`, e.outletID).Scan(&active); err != nil || !active {
		return Invalid("outlet pengajuan ini tidak aktif")
	}
	now := time.Now().UTC()
	_, err := tx.Exec(`INSERT INTO procurement_cash_payouts
		(id, purchase_request_id, outlet_id, amount, recipient_name, notes, status, assigned_by, assigned_by_id, assigned_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9, $9)`,
		NewULID(), prID, e.outletID, float64(remaining)/100, e.recipient,
		emergencyPayoutNotes(EmergencyCategoryLabel(e.category), e.reason),
		truncateRunes(actorName, 100), nilIfEmpty(validActorID(actorID)), now)
	return err
}

// ResendEmergencyPayout: kirim ulang tugas kas ke tablet (tugas sebelumnya
// dikembalikan kasir atau ditarik Keuangan).
func ResendEmergencyPayout(id string, in models.EmergencyDecisionInput) (*models.PurchaseRequest, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	e, err := lockEmergencyTx(tx, id)
	if err != nil {
		return nil, err
	}
	if err := e.checkVersion(in.Version); err != nil {
		return nil, err
	}
	if err := insertEmergencyPayoutTx(tx, id, e, in.ActorName, in.ActorID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE purchase_requests SET updated_at = $2 WHERE id = $1`, id, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetPurchaseRequest(id)
}

// IsEmergencyPurchase: dokumen ini Pembelian Darurat (untuk aturan izin di
// handler).
func IsEmergencyPurchase(id string) bool {
	var emergency bool
	database.DB.QueryRow(`SELECT emergency FROM purchase_requests WHERE id = $1`, id).Scan(&emergency)
	return emergency
}
