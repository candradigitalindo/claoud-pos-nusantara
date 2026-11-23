package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
	"github.com/xuri/excelize/v2"
)

// ── Rekap Pembelian Darurat (audit Keuangan) ───────────────────────────────
//
// Satu bulan (zona aplikasi, menurut tanggal dicatat) per outlet: berapa yang
// dicatat, di tahap mana (menunggu kas / struk / pengesahan, disahkan,
// ditolak = temuan), selisih kas − struk yang harus kembali ke laci, dan —
// sebagai pembanding — Kas Keluar tablet yang BUKAN tugas Bayar Pengadaan.

// Tahap Pembelian Darurat.
const (
	EmergencyStageWaitingCash         = "waiting_cash"
	EmergencyStageWaitingProof        = "waiting_proof"
	EmergencyStageWaitingRatification = "waiting_ratification"
	EmergencyStageRatified            = "ratified"
	EmergencyStageRejected            = "rejected"
	EmergencyStageCancelled           = "cancelled"
)

// emergencyStageKey: tahap satu Pembelian Darurat dari status dokumennya.
func emergencyStageKey(status string, paid float64, ratification, proofURL string) string {
	switch {
	case status == "cancelled" || status == "rejected":
		return EmergencyStageCancelled
	case cents(paid) <= 0:
		return EmergencyStageWaitingCash
	case ratification == "ratified":
		return EmergencyStageRatified
	case ratification == "rejected":
		return EmergencyStageRejected
	case strings.TrimSpace(proofURL) == "":
		return EmergencyStageWaitingProof
	}
	return EmergencyStageWaitingRatification
}

// EmergencyStageLabel: label tahap untuk layar & Excel.
func EmergencyStageLabel(stage string) string {
	switch stage {
	case EmergencyStageWaitingCash:
		return "Menunggu kas diserahkan"
	case EmergencyStageWaitingProof:
		return "Menunggu struk"
	case EmergencyStageWaitingRatification:
		return "Menunggu pengesahan"
	case EmergencyStageRatified:
		return "Disahkan"
	case EmergencyStageRejected:
		return "Pengesahan ditolak (temuan)"
	case EmergencyStageCancelled:
		return "Dibatalkan"
	}
	return stage
}

// emergencyStageLabel: tahap dokumen pengadaan (kosong bila bukan darurat).
func emergencyStageLabel(r models.PurchaseRequest) string {
	e := r.Emergency
	if e == nil {
		return ""
	}
	return EmergencyStageLabel(emergencyStageKey(r.Status, r.PaidAmount, e.Ratification, e.ProofURL))
}

// parseRecapMonth: "YYYY-MM" (kosong = bulan berjalan) → kunci, label, dan
// jendela UTC [from, to).
func parseRecapMonth(month string, now time.Time) (string, string, time.Time, time.Time, error) {
	loc := GetTimezoneLocation()
	month = strings.TrimSpace(month)
	var start time.Time
	if month == "" {
		l := now.In(loc)
		start = time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc)
	} else {
		t, err := time.ParseInLocation("2006-01", month, loc)
		if err != nil {
			return "", "", time.Time{}, time.Time{}, Invalid("bulan tidak valid (format YYYY-MM)")
		}
		start = t
	}
	label := fmt.Sprintf("%s %d", monthLongID[start.Month()-1], start.Year())
	return start.Format("2006-01"), label, start.UTC(), start.AddDate(0, 1, 0).UTC(), nil
}

var monthLongID = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// recapOutletFilter: outlet yang boleh dibaca. all=true → tanpa batas. Role
// bercakupan unit kerja dibatasi ke outlet milik unit-unitnya.
func recapOutletFilter(outletID string, scopeIDs, wuScopeIDs []string) ([]string, bool, error) {
	if id := strings.TrimSpace(outletID); id != "" {
		return []string{id}, false, nil
	}
	if scopeIDs == nil && len(wuScopeIDs) == 0 {
		return nil, true, nil
	}
	ids := append([]string{}, scopeIDs...)
	if len(wuScopeIDs) > 0 {
		rows, err := database.DB.Query(`SELECT DISTINCT outlet_id FROM work_units WHERE id = ANY($1::text[]) AND outlet_id IS NOT NULL`, pq.Array(wuScopeIDs))
		if err != nil {
			return nil, false, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, strings.TrimSpace(id))
			}
		}
	}
	return ids, false, nil
}

func addBucket(b *models.EmergencyRecapBucket, amount float64) {
	b.Count++
	b.Amount += amount
}

// GetEmergencyRecap menyusun rekap bulanan Pembelian Darurat.
func GetEmergencyRecap(month, outletID string, scopeIDs, wuScopeIDs []string) (*models.EmergencyRecap, error) {
	key, label, from, to, err := parseRecapMonth(month, time.Now())
	if err != nil {
		return nil, err
	}
	allowed, all, err := recapOutletFilter(outletID, scopeIDs, wuScopeIDs)
	if err != nil {
		return nil, err
	}
	recap := &models.EmergencyRecap{Month: key, MonthLabel: label, Settings: GetEmergencySettings(),
		Outlets: []models.EmergencyRecapOutlet{}, Rows: []models.EmergencyRecapRow{}}
	recap.Totals.OutletName = "Semua outlet"
	if !all && len(allowed) == 0 {
		return recap, nil
	}

	args := []interface{}{from, to}
	outletCond := func(col string) string {
		if all {
			return ""
		}
		return fmt.Sprintf(" AND %s = ANY($3::text[])", col)
	}
	if !all {
		args = append(args, pq.Array(allowed))
	}

	rows, err := database.DB.Query(`
		SELECT pr.id, pr.request_number, pr.created_at, pr.outlet_id, COALESCE(o.name, ''), COALESCE(wu.name, ''),
		       pr.request_type, pr.items, pr.emergency_category, pr.emergency_reason, COALESCE(pr.requested_by, ''),
		       pr.emergency_recipient, COALESCE(pr.total_final, 0), pr.status, COALESCE(pr.paid_amount, 0), pr.paid_at,
		       COALESCE(cp.cashier_name, ''), COALESCE(cp.authorized_by, ''),
		       pr.emergency_proof_url, pr.emergency_proof_total, pr.ratification_status, pr.ratified_by, pr.ratified_at,
		       pr.ratification_notes, COALESCE(pr.rejected_reason, '')
		FROM purchase_requests pr
		JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		LEFT JOIN LATERAL (
			SELECT cashier_name, authorized_by FROM procurement_cash_payouts p
			WHERE p.purchase_request_id = pr.id AND p.status = 'paid'
			ORDER BY p.paid_at DESC NULLS LAST LIMIT 1
		) cp ON TRUE
		WHERE pr.emergency AND pr.created_at >= $1 AND pr.created_at < $2`+outletCond("pr.outlet_id")+`
		ORDER BY o.name, pr.created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byOutlet := map[string]*models.EmergencyRecapOutlet{}
	outletOf := func(id, name string) *models.EmergencyRecapOutlet {
		id = strings.TrimSpace(id)
		o, ok := byOutlet[id]
		if !ok {
			o = &models.EmergencyRecapOutlet{OutletID: id, OutletName: name}
			byOutlet[id] = o
		}
		return o
	}
	for rows.Next() {
		var r models.EmergencyRecapRow
		var created time.Time
		var itemsJSON []byte
		var category, ratification string
		var paidAt, ratifiedAt *time.Time
		var proofTotal sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.RequestNumber, &created, &r.OutletID, &r.OutletName, &r.WorkUnitName,
			&r.RequestType, &itemsJSON, &category, &r.Reason, &r.RequestedBy,
			&r.Recipient, &r.Amount, &r.Status, &r.PaidAmount, &paidAt,
			&r.Cashier, &r.AuthorizedBy,
			&r.ProofURL, &proofTotal, &ratification, &r.RatifiedBy, &ratifiedAt,
			&r.RatificationNotes, &r.CancelReason); err != nil {
			return nil, err
		}
		r.ID = strings.TrimSpace(r.ID)
		r.OutletID = strings.TrimSpace(r.OutletID)
		r.CreatedAt = created.UTC().Format(time.RFC3339)
		r.PaidAt = fmtTimePtr(paidAt)
		r.RatifiedAt = fmtTimePtr(ratifiedAt)
		r.CategoryLabel = EmergencyCategoryLabel(category)
		r.Items = emergencyRecapItems(itemsJSON)
		r.Stage = emergencyStageKey(r.Status, r.PaidAmount, ratification, r.ProofURL)
		if r.Stage != EmergencyStageCancelled {
			r.CancelReason = ""
		}
		if proofTotal.Valid {
			v := proofTotal.Float64
			r.ProofTotal = &v
			if cents(r.PaidAmount) > 0 {
				d := float64(cents(r.PaidAmount)-cents(v)) / 100
				r.Difference = &d
			}
		}

		for _, o := range []*models.EmergencyRecapOutlet{outletOf(r.OutletID, r.OutletName), &recap.Totals} {
			if r.Stage == EmergencyStageCancelled {
				addBucket(&o.Cancelled, r.Amount)
				continue
			}
			addBucket(&o.Recorded, r.Amount)
			switch r.Stage {
			case EmergencyStageWaitingCash:
				addBucket(&o.WaitingCash, r.Amount)
			case EmergencyStageWaitingProof:
				addBucket(&o.WaitingProof, r.Amount)
			case EmergencyStageWaitingRatification:
				addBucket(&o.WaitingRatification, r.Amount)
			case EmergencyStageRatified:
				addBucket(&o.Ratified, r.Amount)
			case EmergencyStageRejected:
				addBucket(&o.Rejected, r.Amount)
			}
			if r.Difference != nil {
				o.ProofDifference += *r.Difference
			}
		}
		recap.Rows = append(recap.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Pembanding: Kas Keluar tablet di luar tugas Bayar Pengadaan.
	cash, err := database.DB.Query(`
		SELECT m.outlet_id, COALESCE(o.name, ''), COUNT(*), COALESCE(SUM(m.amount), 0)
		FROM cloud_cash_movements m
		JOIN outlets o ON o.id = m.outlet_id
		WHERE m.cash_payout_id IS NULL
		  AND lower(btrim(m.movement_type)) NOT IN ('masuk', 'in', 'income', 'pemasukan')
		  AND m.created_at >= $1 AND m.created_at < $2`+outletCond("m.outlet_id")+`
		GROUP BY m.outlet_id, o.name`, args...)
	if err != nil {
		return nil, err
	}
	defer cash.Close()
	for cash.Next() {
		var id, name string
		var b models.EmergencyRecapBucket
		if err := cash.Scan(&id, &name, &b.Count, &b.Amount); err != nil {
			return nil, err
		}
		o := outletOf(id, name)
		o.OtherCashOut = b
		recap.Totals.OtherCashOut.Count += b.Count
		recap.Totals.OtherCashOut.Amount += b.Amount
	}
	if err := cash.Err(); err != nil {
		return nil, err
	}

	for _, o := range byOutlet {
		o.ProofDifference = roundCents(o.ProofDifference)
		recap.Outlets = append(recap.Outlets, *o)
	}
	recap.Totals.ProofDifference = roundCents(recap.Totals.ProofDifference)
	// Outlet dengan Pembelian Darurat terbanyak (nilai) dulu, lalu nama.
	sort.SliceStable(recap.Outlets, func(i, j int) bool {
		a, b := recap.Outlets[i], recap.Outlets[j]
		if cents(a.Recorded.Amount) != cents(b.Recorded.Amount) {
			return a.Recorded.Amount > b.Recorded.Amount
		}
		return a.OutletName < b.OutletName
	})
	return recap, nil
}

func roundCents(v float64) float64 { return float64(cents(v)) / 100 }

// emergencyRecapItems: "Steam valve ×1, Jasa pasang ×1".
func emergencyRecapItems(raw []byte) string {
	var groups []models.PurchaseRequestItem
	if json.Unmarshal(raw, &groups) != nil {
		return ""
	}
	parts := []string{}
	for _, g := range groups {
		for _, it := range g.Items {
			parts = append(parts, fmt.Sprintf("%s ×%d", it.Name, it.Qty))
		}
	}
	return strings.Join(parts, ", ")
}

// BuildEmergencyRecapExcel: rekap yang sama dalam bentuk Excel (Ringkasan per
// outlet, Rincian per Pembelian Darurat, Cara Membaca).
func BuildEmergencyRecapExcel(month, outletID string, scopeIDs, wuScopeIDs []string) ([]byte, string, error) {
	recap, err := GetEmergencyRecap(month, outletID, scopeIDs, wuScopeIDs)
	if err != nil {
		return nil, "", err
	}
	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()
	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}
	const (
		shSummary = "Ringkasan"
		shRows    = "Rincian"
		shHelp    = "Cara Membaca"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shRows, shHelp} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}
	fmtStamp := func(p *string) string {
		if p == nil || *p == "" {
			return ""
		}
		return fmtUTCStampID(*p, loc)
	}

	// ── Ringkasan ────────────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 30)
	f.SetColWidth(shSummary, "B", "M", 16)
	f.SetCellValue(shSummary, "A1", "REKAP PEMBELIAN DARURAT")
	f.MergeCell(shSummary, "A1", "D1")
	f.SetCellStyle(shSummary, "A1", "D1", st.title)
	row := 2
	if strings.TrimSpace(companyName) != "" {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
		row++
	}
	row++
	setKV := func(label string, value interface{}, style int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if style != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), style)
		}
		row++
	}
	t := recap.Totals
	setKV("Periode", recap.MonthLabel+" (menurut tanggal dicatat)", 0)
	capText := "Tanpa plafon"
	if recap.Settings.MonthlyCap > 0 {
		capText = fmt.Sprintf("%.0f per outlet per bulan", recap.Settings.MonthlyCap)
	}
	setKV("Batas per pembelian", recap.Settings.MaxAmount, st.money)
	setKV("Plafon bulanan", capText, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++
	setKV(fmt.Sprintf("Dicatat (tidak termasuk batal) — %d", t.Recorded.Count), t.Recorded.Amount, st.money)
	setKV(fmt.Sprintf("Disahkan — %d", t.Ratified.Count), t.Ratified.Amount, st.money)
	setKV(fmt.Sprintf("Menunggu (kas/struk/pengesahan) — %d", t.WaitingCash.Count+t.WaitingProof.Count+t.WaitingRatification.Count),
		t.WaitingCash.Amount+t.WaitingProof.Amount+t.WaitingRatification.Amount, st.money)
	setKV(fmt.Sprintf("Pengesahan ditolak (temuan) — %d", t.Rejected.Count), t.Rejected.Amount, st.money)
	setKV("Selisih kas − struk (harus kembali ke laci)", t.ProofDifference, st.money)
	setKV(fmt.Sprintf("Kas Keluar tablet lain — %d", t.OtherCashOut.Count), t.OtherCashOut.Amount, st.money)
	row++

	header := []string{"Outlet", "Jml Dicatat", "Nilai Dicatat", "Menunggu Kas", "Menunggu Struk", "Menunggu Pengesahan",
		"Disahkan", "Ditolak (Temuan)", "Jml Ditolak", "Jml Batal", "Selisih Kas−Struk", "Kas Keluar Lain (Jml)", "Kas Keluar Lain"}
	for i, h := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		f.SetCellValue(shSummary, cell, h)
	}
	last, _ := excelize.CoordinatesToCellName(len(header), row)
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), last, st.header)
	row++
	writeOutlet := func(o models.EmergencyRecapOutlet, numStyle, moneyStyle int) {
		vals := []interface{}{o.OutletName, o.Recorded.Count, o.Recorded.Amount, o.WaitingCash.Amount, o.WaitingProof.Amount,
			o.WaitingRatification.Amount, o.Ratified.Amount, o.Rejected.Amount, o.Rejected.Count, o.Cancelled.Count,
			o.ProofDifference, o.OtherCashOut.Count, o.OtherCashOut.Amount}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			f.SetCellValue(shSummary, cell, v)
			style := moneyStyle
			if i == 0 || i == 1 || i == 8 || i == 9 || i == 11 {
				style = numStyle
			}
			f.SetCellStyle(shSummary, cell, cell, style)
		}
		row++
	}
	for _, o := range recap.Outlets {
		writeOutlet(o, st.num, st.money)
	}
	if len(recap.Outlets) > 0 {
		writeOutlet(t, st.numTot, st.moneyTot)
	}

	// ── Rincian ──────────────────────────────────────────────────────────
	setHeaderRow(f, shRows, []string{
		"No", "Dicatat", "Outlet", "Nomor", "Jenis", "Barang/Jasa", "Kategori", "Alasan", "Pengaju", "Penerima Kas",
		"Nilai Kas", "Tahap", "Kas Diserahkan", "Kasir", "Izin (PIN)", "Total Struk", "Selisih Kas−Struk",
		"Diputus Oleh", "Waktu Keputusan", "Catatan Pengesahan", "Alasan Batal",
	}, st.header)
	f.SetColWidth(shRows, "A", "A", 5)
	f.SetColWidth(shRows, "B", "B", 17)
	f.SetColWidth(shRows, "C", "C", 22)
	f.SetColWidth(shRows, "D", "E", 11)
	f.SetColWidth(shRows, "F", "F", 32)
	f.SetColWidth(shRows, "G", "G", 26)
	f.SetColWidth(shRows, "H", "H", 36)
	f.SetColWidth(shRows, "I", "J", 18)
	f.SetColWidth(shRows, "K", "K", 14)
	f.SetColWidth(shRows, "L", "L", 24)
	f.SetColWidth(shRows, "M", "M", 17)
	f.SetColWidth(shRows, "N", "O", 18)
	f.SetColWidth(shRows, "P", "Q", 14)
	f.SetColWidth(shRows, "R", "S", 18)
	f.SetColWidth(shRows, "T", "U", 30)
	for i, r := range recap.Rows {
		rw := i + 2
		created := r.CreatedAt
		vals := []interface{}{i + 1, fmtStamp(&created), r.OutletName, r.RequestNumber, RequestTypeLabel(r.RequestType), r.Items,
			r.CategoryLabel, r.Reason, r.RequestedBy, r.Recipient, r.Amount, EmergencyStageLabel(r.Stage), fmtStamp(r.PaidAt),
			r.Cashier, r.AuthorizedBy, nil, nil, r.RatifiedBy, fmtStamp(r.RatifiedAt), r.RatificationNotes, r.CancelReason}
		if r.ProofTotal != nil {
			vals[15] = *r.ProofTotal
		}
		if r.Difference != nil {
			vals[16] = *r.Difference
		}
		for c, v := range vals {
			if v == nil {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(c+1, rw)
			f.SetCellValue(shRows, cell, v)
		}
		f.SetCellStyle(shRows, fmt.Sprintf("A%d", rw), fmt.Sprintf("E%d", rw), st.num)
		f.SetCellStyle(shRows, fmt.Sprintf("F%d", rw), fmt.Sprintf("H%d", rw), st.text)
		f.SetCellStyle(shRows, fmt.Sprintf("I%d", rw), fmt.Sprintf("J%d", rw), st.num)
		f.SetCellStyle(shRows, fmt.Sprintf("K%d", rw), fmt.Sprintf("K%d", rw), st.money)
		f.SetCellStyle(shRows, fmt.Sprintf("L%d", rw), fmt.Sprintf("O%d", rw), st.num)
		f.SetCellStyle(shRows, fmt.Sprintf("P%d", rw), fmt.Sprintf("Q%d", rw), st.money)
		f.SetCellStyle(shRows, fmt.Sprintf("R%d", rw), fmt.Sprintf("S%d", rw), st.num)
		f.SetCellStyle(shRows, fmt.Sprintf("T%d", rw), fmt.Sprintf("U%d", rw), st.text)
	}
	if len(recap.Rows) == 0 {
		f.SetCellValue(shRows, "A2", "Tidak ada Pembelian Darurat pada periode ini.")
		f.SetCellStyle(shRows, "A2", "A2", st.note)
	}

	// ── Cara Membaca ─────────────────────────────────────────────────────
	f.SetColWidth(shHelp, "A", "A", 30)
	f.SetColWidth(shHelp, "B", "B", 100)
	help := [][2]string{
		{"Pembelian Darurat", "Barang/jasa urgent ≤ batas yang dibeli dulu dengan kas laci outlet (tugas Bayar Pengadaan di tablet, PIN Manager/SVP), lalu disahkan penyetuju. Diakui sebagai hutang saat dicatat dan lunas saat kasir menyerahkan kas."},
		{"Menunggu kas diserahkan", "Sudah dicatat, kasir belum menyerahkan kas. Masih bisa dibatalkan atau dikirim ulang ke kasir."},
		{"Menunggu struk", "Kas sudah keluar dari laci, foto struk & total belum diunggah pengaju."},
		{"Menunggu pengesahan", "Struk sudah ada, penyetuju belum memutus."},
		{"Pengesahan ditolak (temuan)", "Pembayaran tidak dibatalkan (uang sudah keluar). Tindak lanjuti, mis. menjadi tanggungan manajer outlet."},
		{"Selisih kas − struk", "Positif: struk lebih kecil dari kas yang diserahkan → sisanya harus kembali ke laci dan tercatat sebagai Kas Masuk di shift outlet. Negatif: belanja lebih besar dari kas, selisihnya belum diganti."},
		{"Kas Keluar tablet lain", "Kas Keluar tablet yang BUKAN tugas Bayar Pengadaan (gaji harian, BBM, refund, belanja di luar sistem). Periksa & klasifikasikan di Keuangan → Operasional Keuangan."},
		{"Kroscek fisik", "Nilai Kas = Kas Keluar di laporan tutup kasir shift itu = Bukti Penyerahan Kas yang ditandatangani kasir & penerima."},
	}
	f.SetCellValue(shHelp, "A1", "Istilah")
	f.SetCellValue(shHelp, "B1", "Arti")
	f.SetCellStyle(shHelp, "A1", "B1", st.header)
	for i, h := range help {
		rw := i + 2
		f.SetCellValue(shHelp, fmt.Sprintf("A%d", rw), h[0])
		f.SetCellValue(shHelp, fmt.Sprintf("B%d", rw), h[1])
		f.SetCellStyle(shHelp, fmt.Sprintf("A%d", rw), fmt.Sprintf("A%d", rw), st.label)
		f.SetCellStyle(shHelp, fmt.Sprintf("B%d", rw), fmt.Sprintf("B%d", rw), st.text)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), fmt.Sprintf("Rekap-Pembelian-Darurat_%s.xlsx", recap.Month), nil
}
