package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// Bayar pengadaan dari Kas Kasir Outlet.
//
// Keuangan memilih sumber dana per pembayaran: Kantor Pusat (alur bayar biasa)
// atau Kas Kasir Outlet. Pilihan kedua membuat TUGAS — belum ada uang keluar.
// Tablet kasir outlet itu mengambil tugas langsung dari cloud (online, tidak
// menunggu sinkron), mengklaimnya (terkunci dari penarikan), mencatat kas
// keluar di shift berjalan, lalu menyelesaikannya: pembayaran pengajuan
// tercatat sekali lewat jalur bayar yang sama dengan Kantor Pusat, tertaut ke
// kas keluar itu. Pembukuan melewati kas keluar yang sudah tertaut sehingga
// uang yang sama tidak tercatat dua kali.

var movementIDRe = regexp.MustCompile(`^[0-9A-Za-z_-]{8,50}$`)

const cashPayoutSelect = `SELECT p.id, p.purchase_request_id, COALESCE(pr.request_number,''), COALESCE(pr.request_type,''),
	COALESCE(pr.items,'[]'::jsonb), COALESCE(pr.vendor_name,''), p.outlet_id, COALESCE(o.name,''), p.amount,
	p.recipient_name, p.notes, p.status, p.assigned_by, p.assigned_at, COALESCE(p.movement_local_id,''),
	p.cashier_name, p.authorized_by, p.claimed_at, p.paid_at, p.resolved_by, p.resolved_at, p.resolution_reason
	FROM procurement_cash_payouts p
	JOIN purchase_requests pr ON pr.id = p.purchase_request_id
	LEFT JOIN outlets o ON o.id = p.outlet_id`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func payoutTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

func scanCashPayout(row rowScanner) (*models.ProcurementCashPayout, error) {
	var p models.ProcurementCashPayout
	var items []byte
	var assignedAt time.Time
	var claimedAt, paidAt, resolvedAt sql.NullTime
	if err := row.Scan(&p.ID, &p.PurchaseRequestID, &p.RequestNumber, &p.RequestType, &items, &p.VendorName,
		&p.OutletID, &p.OutletName, &p.Amount, &p.RecipientName, &p.Notes, &p.Status, &p.AssignedBy, &assignedAt,
		&p.MovementLocalID, &p.CashierName, &p.AuthorizedBy, &claimedAt, &paidAt, &p.ResolvedBy, &resolvedAt,
		&p.ResolutionReason); err != nil {
		return nil, err
	}
	p.ID = strings.TrimSpace(p.ID)
	p.PurchaseRequestID = strings.TrimSpace(p.PurchaseRequestID)
	p.OutletID = strings.TrimSpace(p.OutletID)
	p.AssignedAt = assignedAt.UTC().Format(time.RFC3339)
	p.ClaimedAt = payoutTime(claimedAt)
	p.PaidAt = payoutTime(paidAt)
	p.ResolvedAt = payoutTime(resolvedAt)
	p.ItemsSummary = purchaseItemsSummary(items)
	return &p, nil
}

// purchaseItemsSummary: "Bahan dapur, Gas LPG (+2)" — ringkas untuk tablet.
func purchaseItemsSummary(raw []byte) string {
	var groups []models.PurchaseRequestItem
	if json.Unmarshal(raw, &groups) != nil {
		return ""
	}
	names := []string{}
	for _, g := range groups {
		if n := strings.TrimSpace(g.Name); n != "" {
			names = append(names, n)
		}
	}
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + fmt.Sprintf(" (+%d)", len(names)-3)
	}
	return strings.Join(names, ", ")
}

// GetCashPayout membaca satu tugas.
func GetCashPayout(id string) (*models.ProcurementCashPayout, error) {
	p, err := scanCashPayout(database.DB.QueryRow(cashPayoutSelect+` WHERE p.id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	return p, err
}

func cents(v float64) int64 { return int64(math.Round(v * 100)) }

func limitText(s string, max int, field string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		return "", Invalid("%s maksimal %d karakter", field, max)
	}
	return s, nil
}

// AssignCashPayout — keuangan memilih sumber dana Kas Kasir Outlet.
func AssignCashPayout(in models.AssignCashPayoutInput) (*models.ProcurementCashPayout, error) {
	prID := strings.TrimSpace(in.PurchaseRequestID)
	if prID == "" {
		return nil, Invalid("pengajuan wajib dipilih")
	}
	if in.Amount < 0 {
		return nil, Invalid("nominal tidak boleh negatif")
	}
	recipient, err := limitText(in.RecipientName, 200, "nama penerima")
	if err != nil {
		return nil, err
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

	// Urutan kunci sama dengan alur bayar: induk dulu, lalu dokumennya.
	var parentID sql.NullString
	if err := tx.QueryRow(`SELECT parent_id FROM purchase_requests WHERE id = $1`, prID).Scan(&parentID); err != nil {
		return nil, Invalid("pengajuan tidak ditemukan")
	}
	if p := strings.TrimSpace(parentID.String); p != "" {
		if _, err := tx.Exec(`SELECT 1 FROM purchase_requests WHERE id = $1 FOR UPDATE`, p); err != nil {
			return nil, err
		}
	}
	cur, err := lockPurchaseTx(tx, prID)
	if err != nil {
		return nil, Invalid("pengajuan tidak ditemukan")
	}
	if cur.SplitStatus == "master" {
		return nil, Invalid("pengajuan induk dibayar per vendor — pilih pecahannya")
	}
	if _, ok := validTransitions["pay"][cur.Status]; !ok {
		return nil, Invalid("pengajuan belum dalam tahap pembayaran")
	}
	var outletID, vendorID, vendorName, emergencyRecipient string
	var totalFinal, paid float64
	var emergency bool
	if err := tx.QueryRow(`
		SELECT COALESCE(outlet_id,''), COALESCE(vendor_id,''), COALESCE(vendor_name,''),
		       COALESCE(total_final,0), COALESCE(paid_amount,0), emergency, emergency_recipient
		FROM purchase_requests WHERE id = $1`, prID).Scan(&outletID, &vendorID, &vendorName, &totalFinal, &paid,
		&emergency, &emergencyRecipient); err != nil {
		return nil, err
	}
	outletID = strings.TrimSpace(outletID)
	if outletID == "" {
		return nil, Invalid("pengajuan tanpa outlet hanya bisa dibayar Kantor Pusat")
	}
	if emergency {
		// Pembelian Darurat: tunai untuk staf yang berbelanja — tidak memakai
		// vendor terdaftar maupun detail pembayaran dari Pembelian.
		if recipient == "" {
			recipient = strings.TrimSpace(emergencyRecipient)
		}
	} else {
		// Tujuan pembayaran milik Pembelian (atau hasil arahan langsung
		// penyetuju ke Kas Kasir Outlet): tugas kas tidak boleh berjalan di
		// atas detail yang sedang dikembalikan / belum diisi, dan VA
		// marketplace dibayar lewat transfer Kantor Pusat.
		payTo, err := payToForPRTx(tx, prID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(vendorID) == "" && !(payTo.Effective == PayToManual && payTo.Ready) {
			return nil, Invalid("pengajuan ini belum memilih vendor terdaftar")
		}
		switch {
		case payTo.ReturnedAt != nil:
			return nil, Invalid("detail pembayaran pengajuan ini sedang dikembalikan ke Pembelian — tunggu perbaikannya")
		case payTo.Effective == PayToMarketplaceVA:
			return nil, Invalid("Virtual Account Marketplace dibayar lewat transfer Kantor Pusat, bukan Kas Kasir Outlet")
		case !payTo.Ready:
			return nil, Invalid("belum bisa dibayar: %s", payTo.Problem)
		}
	}
	var active bool
	if err := tx.QueryRow(`SELECT is_active FROM outlets WHERE id = $1`, outletID).Scan(&active); err != nil || !active {
		return nil, Invalid("outlet pengajuan ini tidak aktif — bayar dari Kantor Pusat")
	}
	remaining := cents(totalFinal) - cents(paid)
	if remaining <= 0 {
		return nil, Invalid("tagihan sudah lunas")
	}
	amount := cents(in.Amount)
	if amount == 0 {
		amount = remaining
	}
	if amount > remaining {
		return nil, Invalid("nominal (%s) melebihi sisa tagihan (%s)", formatRupiahPlain(amount), formatRupiahPlain(remaining))
	}
	if recipient == "" {
		recipient = strings.TrimSpace(vendorName)
	}
	if recipient == "" {
		return nil, Invalid("nama penerima kas wajib diisi")
	}
	var open int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM procurement_cash_payouts
		WHERE purchase_request_id = $1 AND status IN ('pending','claimed')`, prID).Scan(&open); err != nil {
		return nil, err
	}
	if open > 0 {
		return nil, Conflict("pengajuan ini sudah punya tugas Kas Kasir Outlet yang terbuka")
	}
	id := NewULID()
	now := time.Now().UTC()
	if _, err := tx.Exec(`INSERT INTO procurement_cash_payouts
		(id, purchase_request_id, outlet_id, amount, recipient_name, notes, status, assigned_by, assigned_by_id, assigned_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8,$9,$9)`,
		id, prID, outletID, float64(amount)/100, recipient, notes, in.ActorName, nilIfEmpty(validActorID(in.ActorID)), now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCashPayout(id)
}

func formatRupiahPlain(c int64) string {
	whole := c / 100
	s := fmt.Sprintf("%d", whole)
	var out []byte
	for i, ch := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, ch)
	}
	return "Rp" + string(out)
}

type lockedPayout struct {
	ID, PRID, OutletID, Status, MovementID, Recipient, Notes, AssignedBy, Cashier, AuthorizedBy string
	Amount                                                                                     float64
}

func lockCashPayoutTx(tx *sql.Tx, id string) (*lockedPayout, error) {
	p := &lockedPayout{}
	err := tx.QueryRow(`SELECT id, purchase_request_id, outlet_id, status, COALESCE(movement_local_id,''),
		recipient_name, notes, assigned_by, cashier_name, authorized_by, amount
		FROM procurement_cash_payouts WHERE id = $1 FOR UPDATE`, id).
		Scan(&p.ID, &p.PRID, &p.OutletID, &p.Status, &p.MovementID, &p.Recipient, &p.Notes, &p.AssignedBy,
			&p.Cashier, &p.AuthorizedBy, &p.Amount)
	if err == sql.ErrNoRows {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	p.ID = strings.TrimSpace(p.ID)
	p.PRID = strings.TrimSpace(p.PRID)
	p.OutletID = strings.TrimSpace(p.OutletID)
	return p, nil
}

// CancelCashPayout — keuangan menarik tugas yang BELUM diproses kasir.
func CancelCashPayout(id string, in models.CashPayoutResolveInput) (*models.ProcurementCashPayout, error) {
	reason, err := limitText(in.Reason, 1000, "alasan")
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := lockCashPayoutTx(tx, id)
	if err != nil {
		return nil, err
	}
	switch p.Status {
	case "pending":
	case "claimed":
		return nil, Conflict("tugas sedang diproses kasir outlet — tidak bisa ditarik")
	case "paid":
		return nil, Conflict("tugas sudah dibayar dari kas outlet")
	default:
		return nil, Conflict("tugas sudah tidak aktif")
	}
	if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status='cancelled', resolved_by=$2,
		resolved_at=$3, resolution_reason=$4, updated_at=$3 WHERE id=$1`,
		p.ID, in.ActorName, time.Now().UTC(), reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCashPayout(p.ID)
}

// ListOutletCashPayouts — daftar untuk tablet: tugas terbuka + yang selesai
// 3 hari terakhir (cetak ulang bukti / riwayat singkat).
func ListOutletCashPayouts(outletID string) ([]models.ProcurementCashPayout, error) {
	rows, err := database.DB.Query(cashPayoutSelect+`
		WHERE p.outlet_id = $1 AND (p.status IN ('pending','claimed')
		   OR (p.status IN ('paid','returned','cancelled') AND p.updated_at > (NOW() AT TIME ZONE 'UTC') - INTERVAL '3 days'))
		ORDER BY CASE p.status WHEN 'claimed' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END, p.assigned_at`, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProcurementCashPayout{}
	for rows.Next() {
		p, err := scanCashPayout(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func validMovementID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !movementIDRe.MatchString(id) {
		return "", Invalid("id kas keluar tidak valid")
	}
	return id, nil
}

// ClaimCashPayout — tablet mengunci tugas SEBELUM uang keluar dari laci.
// Idempoten untuk id kas keluar yang sama (balasan hilang lalu dikirim ulang).
func ClaimCashPayout(outletID, id string, in models.CashPayoutDeviceInput) (*models.ProcurementCashPayout, error) {
	movement, err := validMovementID(in.MovementLocalID)
	if err != nil {
		return nil, err
	}
	cashier, err := limitText(in.CashierName, 100, "nama kasir")
	if err != nil {
		return nil, err
	}
	authorizer, err := limitText(in.AuthorizedBy, 100, "nama pemberi izin")
	if err != nil {
		return nil, err
	}
	if cashier == "" || authorizer == "" {
		return nil, Invalid("kasir dan pemberi izin (PIN manajer) wajib tercatat")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := lockCashPayoutTx(tx, id)
	if err != nil {
		return nil, err
	}
	if p.OutletID != outletID {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	switch p.Status {
	case "claimed", "paid":
		if p.MovementID == movement {
			tx.Rollback()
			return GetCashPayout(p.ID)
		}
		return nil, Conflict("tugas ini sudah diproses perangkat lain")
	case "returned", "cancelled":
		return nil, Conflict("tugas ini sudah ditarik atau dikembalikan — muat ulang daftar")
	}
	// Sisa tagihan masih cukup? (keuangan bisa saja mengubah harga final).
	var totalFinal, paid float64
	if err := tx.QueryRow(`SELECT COALESCE(total_final,0), COALESCE(paid_amount,0) FROM purchase_requests WHERE id=$1`, p.PRID).
		Scan(&totalFinal, &paid); err != nil {
		return nil, err
	}
	if cents(totalFinal)-cents(paid) < cents(p.Amount) {
		return nil, Conflict("sisa tagihan pengajuan sudah berubah — kembalikan tugas ini ke keuangan")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status='claimed', movement_local_id=$2,
		cashier_name=$3, authorized_by=$4, claimed_at=$5, updated_at=$5 WHERE id=$1`,
		p.ID, movement, cashier, authorizer, now); err != nil {
		if strings.Contains(err.Error(), "uq_cash_payout_movement") {
			return nil, Conflict("id kas keluar sudah dipakai tugas lain")
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCashPayout(p.ID)
}

// CompleteCashPayout — kas keluar sudah tercatat di tablet: catat pembayaran
// pengajuan (sekali) lewat jalur bayar yang sama dengan Kantor Pusat.
// Dipanggil tablet segera setelah penyerahan, dan juga dari sinkron kas keluar
// (jalur tahan-putus). Idempoten untuk id kas keluar yang sama.
func CompleteCashPayout(outletID, id, movementLocalID string) (*models.ProcurementCashPayout, error) {
	movement, err := validMovementID(movementLocalID)
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := lockCashPayoutTx(tx, id)
	if err != nil {
		return nil, err
	}
	if p.OutletID != outletID {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	if p.MovementID != movement {
		return nil, Conflict("kas keluar ini bukan milik tugas tersebut")
	}
	if p.Status == "paid" {
		tx.Rollback()
		return GetCashPayout(p.ID)
	}
	if p.Status != "claimed" {
		return nil, Conflict("tugas belum diklaim tablet ini")
	}
	var outletName string
	tx.QueryRow(`SELECT COALESCE(name,'') FROM outlets WHERE id=$1`, p.OutletID).Scan(&outletName)

	notice, err := payPurchaseFromCashPayoutTx(tx, p, outletName)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status='paid', paid_at=$2, updated_at=$2 WHERE id=$1`, p.ID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE cloud_cash_movements SET cash_payout_id=$1 WHERE outlet_id=$2 AND local_id=$3`,
		p.ID, p.OutletID, movement); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if pr, err := GetPurchaseRequest(p.PRID); err == nil {
		go NotifyPurchaseEvent(pr, notice.action, notice.actor, "", notice.amount)
	}
	return GetCashPayout(p.ID)
}

type payoutNotice struct {
	action, actor string
	amount        float64
}

func payPurchaseFromCashPayoutTx(tx *sql.Tx, p *lockedPayout, outletName string) (payoutNotice, error) {
	var parentID sql.NullString
	if err := tx.QueryRow(`SELECT parent_id FROM purchase_requests WHERE id = $1`, p.PRID).Scan(&parentID); err != nil {
		return payoutNotice{}, Invalid("pengajuan tidak ditemukan")
	}
	if pid := strings.TrimSpace(parentID.String); pid != "" {
		if _, err := tx.Exec(`SELECT 1 FROM purchase_requests WHERE id = $1 FOR UPDATE`, pid); err != nil {
			return payoutNotice{}, err
		}
	}
	cur, err := lockPurchaseTx(tx, p.PRID)
	if err != nil {
		return payoutNotice{}, Invalid("pengajuan tidak ditemukan")
	}
	newStatus, ok := validTransitions["pay"][cur.Status]
	if !ok || cur.SplitStatus == "master" {
		return payoutNotice{}, Conflict("status pengajuan berubah (%s) — hubungi keuangan", cur.Status)
	}
	label := strings.TrimSpace(outletName)
	if label == "" {
		label = "outlet"
	}
	actor := truncateRunes(p.Cashier+" · Kas Kasir "+label, 100)
	notes := fmt.Sprintf("Diserahkan kasir %s (izin %s) kepada %s. Ditetapkan %s.", p.Cashier, p.AuthorizedBy, p.Recipient, p.AssignedBy)
	if strings.TrimSpace(p.Notes) != "" {
		notes += " " + strings.TrimSpace(p.Notes)
	}
	input := models.UpdatePurchaseStatusInput{
		Action:               "pay",
		ActorName:            actor,
		PaymentAmount:        p.Amount,
		PaymentMethod:        "outlet_cash",
		PaymentAccountDest:   truncateRunes("Tunai dari Kas Kasir "+label+" — diterima "+p.Recipient, 300),
		PaymentAccountSource: truncateRunes("Kas Kasir "+label, 300),
		PaymentNotes:         notes,
		CashPayoutID:         p.ID,
	}
	n, err := applyStatusUpdateTx(tx, p.PRID, cur.Status, newStatus, input)
	if err != nil {
		return payoutNotice{}, err
	}
	if cur.ParentID != "" {
		if err := syncFamilyStatusTx(tx, cur.ParentID, Actor{Name: actor}); err != nil {
			return payoutNotice{}, err
		}
	}
	return payoutNotice{action: n.action, actor: actor, amount: n.amount}, nil
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// ReturnCashPayout — kasir mengembalikan tugas ke keuangan (mis. harga di
// lapangan berbeda, kas laci tidak cukup). Hanya tugas yang belum diklaim.
func ReturnCashPayout(outletID, id string, in models.CashPayoutDeviceInput) (*models.ProcurementCashPayout, error) {
	reason, err := limitText(in.Reason, 1000, "alasan")
	if err != nil {
		return nil, err
	}
	if reason == "" {
		return nil, Invalid("alasan pengembalian wajib diisi")
	}
	cashier, err := limitText(in.CashierName, 100, "nama kasir")
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := lockCashPayoutTx(tx, id)
	if err != nil {
		return nil, err
	}
	if p.OutletID != outletID {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	if p.Status != "pending" {
		return nil, Conflict("tugas ini tidak lagi menunggu — muat ulang daftar")
	}
	who := strings.TrimSpace(cashier + " (kasir)")
	if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status='returned', resolved_by=$2,
		resolved_at=$3, resolution_reason=$4, updated_at=$3 WHERE id=$1`,
		p.ID, who, time.Now().UTC(), reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCashPayout(p.ID)
}

// ReleaseCashPayout — klaim dilepas karena kas keluar GAGAL tercatat di
// tablet (uang belum diserahkan). Ditolak bila kas keluarnya sudah sampai.
func ReleaseCashPayout(outletID, id, movementLocalID string) (*models.ProcurementCashPayout, error) {
	movement, err := validMovementID(movementLocalID)
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := lockCashPayoutTx(tx, id)
	if err != nil {
		return nil, err
	}
	if p.OutletID != outletID {
		return nil, Invalid("tugas penyerahan kas tidak ditemukan")
	}
	if p.Status == "pending" {
		tx.Rollback()
		return GetCashPayout(p.ID)
	}
	if p.Status != "claimed" || p.MovementID != movement {
		return nil, Conflict("tugas ini tidak sedang diklaim oleh penyerahan tersebut")
	}
	var recorded bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cloud_cash_movements WHERE outlet_id=$1 AND local_id=$2)`,
		p.OutletID, movement).Scan(&recorded); err != nil {
		return nil, err
	}
	if recorded {
		return nil, Conflict("kas keluar penyerahan ini sudah tercatat — selesaikan tugasnya, jangan dilepas")
	}
	if _, err := tx.Exec(`UPDATE procurement_cash_payouts SET status='pending', movement_local_id=NULL,
		cashier_name='', authorized_by='', claimed_at=NULL, updated_at=$2 WHERE id=$1`, p.ID, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCashPayout(p.ID)
}

// linkableCashPayout: id tugas yang boleh ditautkan ke kas keluar sinkron —
// hanya tugas outlet yang sama yang diklaim/dibayar dengan id kas keluar itu.
func linkableCashPayout(outletID, payoutID, movementLocalID string) string {
	payoutID = strings.TrimSpace(payoutID)
	if payoutID == "" {
		return ""
	}
	var status, movement string
	err := database.DB.QueryRow(`SELECT status, COALESCE(movement_local_id,'') FROM procurement_cash_payouts
		WHERE id=$1 AND outlet_id=$2`, payoutID, outletID).Scan(&status, &movement)
	if err != nil || movement != strings.TrimSpace(movementLocalID) || (status != "claimed" && status != "paid") {
		log.Printf("cash payout link ditolak: outlet=%s payout=%s movement=%s status=%s err=%v",
			outletID, payoutID, movementLocalID, status, err)
		return ""
	}
	return payoutID
}

// settleCashPayoutFromSync — kas keluar bertaut tugas sampai lewat sinkron:
// selesaikan tugasnya bila tablet belum sempat melakukannya secara online.
func settleCashPayoutFromSync(outletID, payoutID, movementLocalID string) {
	if payoutID == "" {
		return
	}
	if _, err := CompleteCashPayout(outletID, payoutID, movementLocalID); err != nil {
		log.Printf("cash payout %s belum bisa diselesaikan dari sinkron: %v", payoutID, err)
	}
}

// attachCashPayouts menempelkan tugas Kas Kasir Outlet yang masih relevan ke
// daftar pengajuan (halaman Pembayaran): terbuka (menunggu/diproses) atau
// dikembalikan kasir selama tagihannya belum lunas.
func attachCashPayouts(list []models.PurchaseRequest) {
	if len(list) == 0 {
		return
	}
	ids := make([]string, 0, len(list))
	for _, pr := range list {
		ids = append(ids, pr.ID)
	}
	rows, err := database.DB.Query(`SELECT DISTINCT ON (p.purchase_request_id) `+strings.TrimPrefix(cashPayoutSelect, "SELECT ")+`
		WHERE p.purchase_request_id = ANY($1::text[]) AND p.status <> 'cancelled'
		ORDER BY p.purchase_request_id, p.assigned_at DESC`, pq.Array(ids))
	if err != nil {
		log.Printf("attach cash payouts: %v", err)
		return
	}
	defer rows.Close()
	byPR := map[string]*models.ProcurementCashPayout{}
	for rows.Next() {
		p, err := scanCashPayout(rows)
		if err != nil {
			log.Printf("attach cash payouts scan: %v", err)
			return
		}
		byPR[p.PurchaseRequestID] = p
	}
	for i := range list {
		p := byPR[strings.TrimSpace(list[i].ID)]
		if p == nil {
			continue
		}
		open := p.Status == "pending" || p.Status == "claimed"
		returnedStillDue := p.Status == "returned" && cents(list[i].TotalFinal)-cents(list[i].PaidAmount) > 0
		if open || returnedStillDue {
			list[i].CashPayout = p
		}
	}
}
