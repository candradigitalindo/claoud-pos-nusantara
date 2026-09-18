package services

// Uang muka reservasi.
//
// Uang yang masuk atas sebuah reservasi hidup di reservation_payments, bukan
// di kolom angka bebas. Bukti dari pelanggan (halaman publik) lahir 'pending'
// dan harus divalidasi admin; yang dicatat admin sendiri langsung 'validated'
// karena admin melihat uangnya. Secara keuangan uang muka adalah KEWAJIBAN
// (Uang Muka Pelanggan) sampai reservasi ditutup di POS — saat itulah baris
// pembayaran bermetode 'reservasi_dp' pada transaksi memakainya, dan
// pendapatan diakui penuh pada hari kunjungan.

import (
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// ReservationDpMethod adalah metode pembayaran pada transaksi POS yang berarti
// "dibayar dari uang muka reservasi". Laporan kas mengecualikannya dari
// penerimaan hari kunjungan karena uangnya sudah masuk saat DP.
const ReservationDpMethod = "reservasi_dp"

// tzStamp: TIMESTAMP (UTC) → teks 'YYYY-MM-DD HH24:MI' dalam zona aplikasi.
func tzStamp(expr string) string {
	return fmt.Sprintf(`COALESCE(TO_CHAR((%s AT TIME ZONE 'UTC') AT TIME ZONE '%s', 'YYYY-MM-DD HH24:MI'), '')`,
		expr, GetTimezoneLocation().String())
}

var reservationPaymentTypes = map[string]bool{"dp": true, "pelunasan": true, "refund": true}
var reservationPaymentMethods = map[string]bool{"transfer": true, "cash": true, "qris": true, "lainnya": true}

func reservationPaymentCols() string {
	return `p.id, p.reservation_id, p.type, p.amount, COALESCE(p.method, ''), COALESCE(p.bank_account_id, ''),
	COALESCE(b.bank_name || ' ' || b.account_number || ' a.n. ' || b.account_holder, ''),
	COALESCE(p.proof_url, ''), ` + tzStamp("p.paid_at") + `, p.status, COALESCE(p.submitted_by, ''),
	COALESCE(p.validated_by, ''), ` + tzStamp("p.validated_at") + `, COALESCE(p.rejected_reason, ''),
	COALESCE(p.notes, ''), ` + tzStamp("p.created_at")
}

func scanReservationPayment(scan func(dest ...interface{}) error) (models.ReservationPayment, error) {
	var p models.ReservationPayment
	err := scan(&p.ID, &p.ReservationID, &p.Type, &p.Amount, &p.Method, &p.BankAccountID, &p.BankLabel,
		&p.ProofURL, &p.PaidAt, &p.Status, &p.SubmittedBy, &p.ValidatedBy, &p.ValidatedAt,
		&p.RejectedReason, &p.Notes, &p.CreatedAt)
	p.BankAccountID = strings.TrimSpace(p.BankAccountID)
	return p, err
}

func ListReservationPayments(reservationID string) ([]models.ReservationPayment, error) {
	rows, err := database.DB.Query(`SELECT `+reservationPaymentCols()+`
		FROM reservation_payments p LEFT JOIN bank_accounts b ON b.id = p.bank_account_id
		WHERE p.reservation_id = $1 ORDER BY p.paid_at, p.created_at`, reservationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ReservationPayment, 0)
	for rows.Next() {
		p, err := scanReservationPayment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func getReservationPayment(reservationID, paymentID string) (*models.ReservationPayment, error) {
	p, err := scanReservationPayment(database.DB.QueryRow(`SELECT `+reservationPaymentCols()+`
		FROM reservation_payments p LEFT JOIN bank_accounts b ON b.id = p.bank_account_id
		WHERE p.id = $1 AND p.reservation_id = $2`, paymentID, reservationID).Scan)
	if err != nil {
		return nil, fmt.Errorf("pembayaran tidak ditemukan")
	}
	return &p, nil
}

// parsePaidAt membaca tanggal bayar dalam zona aplikasi dan mengembalikan UTC.
// Kosong = sekarang. Tanggal di masa depan ditolak: uang yang belum masuk
// tidak bisa divalidasi.
func parsePaidAt(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().UTC(), nil
	}
	loc := GetTimezoneLocation()
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			if t.After(time.Now().Add(time.Hour)) {
				return time.Time{}, Invalid("tanggal bayar tidak boleh di masa depan")
			}
			return t.UTC(), nil
		}
	}
	return time.Time{}, Invalid("format tanggal bayar tidak dikenali (YYYY-MM-DD atau YYYY-MM-DD HH:MM)")
}

// AddReservationPayment mencatat satu bukti uang masuk/keluar. validated=true
// untuk pencatatan oleh admin (langsung sah), false untuk kiriman pelanggan.
func AddReservationPayment(reservationID string, req models.ReservationPaymentRequest, actor string, validated bool, outletScope []string) (*models.ReservationPayment, error) {
	r, err := GetReservation(reservationID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if !reservationPaymentTypes[req.Type] {
		return nil, Invalid("jenis pembayaran harus dp, pelunasan, atau refund")
	}
	if req.Amount <= 0 {
		return nil, Invalid("nominal harus lebih dari 0")
	}
	req.Method = strings.ToLower(strings.TrimSpace(req.Method))
	if req.Method == "" {
		req.Method = "transfer"
	}
	if !reservationPaymentMethods[req.Method] {
		return nil, Invalid("metode '%s' tidak dikenal", req.Method)
	}
	// Bukti wajib untuk transfer/QRIS dan untuk semua kiriman pelanggan —
	// tanpa bukti, "sudah transfer" tidak bisa dibedakan dari "belum".
	if strings.TrimSpace(req.ProofURL) == "" && (!validated || req.Method == "transfer" || req.Method == "qris") {
		return nil, Invalid("bukti pembayaran wajib diunggah")
	}
	switch req.Type {
	case "refund":
		if r.Status != "cancelled" || r.CancelDisposition != "refund" {
			return nil, Invalid("refund hanya untuk reservasi yang dibatalkan dengan keputusan 'refund'")
		}
		if req.Amount > r.PaidAmount+0.009 {
			return nil, Invalid("refund %.0f melebihi uang muka yang tersisa %.0f", req.Amount, r.PaidAmount)
		}
	default:
		if r.Status != "pending" && r.Status != "confirmed" {
			return nil, Invalid("reservasi berstatus %s tidak menerima pembayaran lagi", r.Status)
		}
		if open := r.Remaining - r.PendingAmount; req.Amount > open+0.009 {
			return nil, Invalid("nominal %.0f melebihi sisa yang belum dibayar/menunggu validasi (%.0f)", req.Amount, math.Max(open, 0))
		}
	}
	if req.BankAccountID != "" {
		var exists bool
		database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM bank_accounts WHERE id = $1)`, req.BankAccountID).Scan(&exists)
		if !exists {
			return nil, Invalid("rekening tujuan tidak dikenal")
		}
	}
	paidAt, err := parsePaidAt(req.PaidAt)
	if err != nil {
		return nil, err
	}

	id := NewULID()
	status, validatedBy := "pending", ""
	var validatedAt interface{}
	submittedBy := "pelanggan"
	if validated {
		status, validatedBy, validatedAt, submittedBy = "validated", actor, time.Now().UTC(), actor
	}
	if _, err := database.DB.Exec(`
		INSERT INTO reservation_payments (id, reservation_id, outlet_id, type, amount, method, bank_account_id,
			proof_url, paid_at, status, submitted_by, validated_by, validated_at, notes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,(now() AT TIME ZONE 'UTC'))`,
		id, r.ID, r.OutletID, req.Type, req.Amount, req.Method, nilIfEmpty(req.BankAccountID),
		req.ProofURL, paidAt, status, submittedBy, validatedBy, validatedAt, req.Notes); err != nil {
		return nil, err
	}
	if validated {
		maybeAutoConfirm(r.ID)
	}
	return getReservationPayment(r.ID, id)
}

// ValidateReservationPayment mengesahkan bukti kiriman pelanggan. Nominal
// diperiksa ulang terhadap sisa SAAT INI, karena pembayaran lain bisa saja
// sudah divalidasi sejak bukti ini dikirim.
func ValidateReservationPayment(reservationID, paymentID, actor string, outletScope []string) (*models.ReservationPayment, error) {
	r, err := GetReservation(reservationID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	p, err := getReservationPayment(reservationID, paymentID)
	if err != nil {
		return nil, err
	}
	if p.Status != "pending" {
		return nil, Invalid("pembayaran ini sudah %s", p.Status)
	}
	if p.Type != "refund" && p.Amount > r.Remaining+0.009 {
		return nil, Invalid("nominal %.0f melebihi sisa tagihan %.0f — tolak dan minta pelanggan kirim ulang", p.Amount, r.Remaining)
	}
	if _, err := database.DB.Exec(`UPDATE reservation_payments SET status='validated', validated_by=$1,
		validated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`, actor, paymentID); err != nil {
		return nil, err
	}
	maybeAutoConfirm(reservationID)
	return getReservationPayment(reservationID, paymentID)
}

func RejectReservationPayment(reservationID, paymentID, reason, actor string, outletScope []string) (*models.ReservationPayment, error) {
	if _, err := GetReservation(reservationID, outletScope); err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	p, err := getReservationPayment(reservationID, paymentID)
	if err != nil {
		return nil, err
	}
	if p.Status != "pending" {
		return nil, Invalid("pembayaran ini sudah %s", p.Status)
	}
	if strings.TrimSpace(reason) == "" {
		return nil, Invalid("alasan penolakan wajib diisi — pelanggan perlu tahu apa yang harus diperbaiki")
	}
	if _, err := database.DB.Exec(`UPDATE reservation_payments SET status='rejected', rejected_reason=$1,
		validated_by=$2, validated_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`, reason, actor, paymentID); err != nil {
		return nil, err
	}
	return getReservationPayment(reservationID, paymentID)
}

// maybeAutoConfirm: begitu DP yang diminta terpenuhi oleh pembayaran
// tervalidasi, reservasi 'pending' otomatis menjadi 'confirmed'. Inilah arti
// "Dikonfirmasi": uangnya sudah ada, bukan sekadar admin menekan tombol.
func maybeAutoConfirm(reservationID string) {
	r, err := GetReservation(reservationID, nil)
	if err != nil || r.Status != "pending" || !r.DpPaid || r.PaidAmount <= 0 {
		return
	}
	database.DB.Exec(`UPDATE reservations SET status='confirmed', confirmed_at=(now() AT TIME ZONE 'UTC'), updated_at=NOW() WHERE id=$1 AND status='pending'`, reservationID)
}

// ── Kebijakan DP & rekening ─────────────────────────────────────────────────

func ReservationDpPercent() float64 {
	v := settingFloat("reservation_dp_percent", 50)
	if v > 100 {
		v = 100
	}
	return v
}

func SetReservationDpPercent(v float64) error {
	if v < 0 || v > 100 {
		return Invalid("persen DP harus 0–100")
	}
	return UpdateSettings(map[string]string{"reservation_dp_percent": fmt.Sprintf("%g", v)})
}

func activeBankAccounts() []models.BankAccount {
	all, err := ListBankAccounts()
	if err != nil {
		return []models.BankAccount{}
	}
	out := make([]models.BankAccount, 0, len(all))
	for _, b := range all {
		if b.IsActive {
			out = append(out, b)
		}
	}
	return out
}

func GetReservationSettings() *models.ReservationSettings {
	return &models.ReservationSettings{DpPercent: ReservationDpPercent(), BankAccounts: activeBankAccounts()}
}

// ── Publik (tanpa auth) ─────────────────────────────────────────────────────

// PublicReservationStatus: yang perlu diketahui pelanggan — berapa yang harus
// dibayar, ke rekening mana, dan sudah sampai mana. Bukan seluruh dokumen.
func PublicReservationStatus(slug, id string) (*models.PublicReservationStatus, error) {
	outletID, outletName, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	r, err := GetReservation(id, nil)
	if err != nil || r.OutletID != outletID {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	return &models.PublicReservationStatus{
		ID: r.ID, OutletName: outletName, CustomerName: r.CustomerName,
		ReservationDate: r.ReservationDate, ReservationTime: r.ReservationTime, Pax: r.Pax,
		Items: r.Items, Total: r.Total, DownPayment: r.DownPayment, PaidAmount: r.PaidAmount,
		PendingAmount: r.PendingAmount, Remaining: r.Remaining, Status: r.Status, DpPaid: r.DpPaid,
		Payments: r.Payments, BankAccounts: activeBankAccounts(),
	}, nil
}

// PublicSubmitReservationPayment: pelanggan mengirim bukti transfer. Lahir
// 'pending'; admin yang memvalidasi.
func PublicSubmitReservationPayment(slug, id string, req models.ReservationPaymentRequest) (*models.ReservationPayment, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	r, err := GetReservation(id, nil)
	if err != nil || r.OutletID != outletID {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	if req.Type != "dp" && req.Type != "pelunasan" {
		req.Type = "dp"
		if r.DpPaid {
			req.Type = "pelunasan"
		}
	}
	if req.Method == "" {
		req.Method = "transfer"
	}
	return AddReservationPayment(r.ID, req, "pelanggan", false, nil)
}

// ── App POS (auth API key outlet) ───────────────────────────────────────────

// ListOutletReservations: reservasi hari itu yang masih hidup, untuk dibuka
// kasir sebagai order. Bawaan = hari ini menurut zona aplikasi.
func ListOutletReservations(outletID, date string) ([]models.OutletReservation, error) {
	if strings.TrimSpace(date) == "" {
		date = time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	}
	list, err := ListReservations(outletID, "", date, date, nil)
	if err != nil {
		return nil, err
	}
	out := make([]models.OutletReservation, 0, len(list))
	for _, r := range list {
		if r.Status != "pending" && r.Status != "confirmed" {
			continue
		}
		out = append(out, models.OutletReservation{
			ID: r.ID, CustomerName: r.CustomerName, CustomerPhone: r.CustomerPhone, Pax: r.Pax,
			ReservationDate: r.ReservationDate, ReservationTime: r.ReservationTime, Items: r.Items,
			Total: r.Total, PaidAmount: r.PaidAmount, Remaining: r.Remaining, Status: r.Status, Notes: r.Notes,
		})
	}
	return out, nil
}

// SettleReservation menutup reservasi dari transaksi POS: status 'done',
// tertaut ke transaksinya. Idempoten untuk transaksi yang sama (re-sync).
func SettleReservation(outletID, reservationID, transactionID, actor string) error {
	r, err := GetReservation(reservationID, nil)
	if err != nil || r.OutletID != outletID {
		return fmt.Errorf("reservasi tidak ditemukan di outlet ini")
	}
	if r.Status == "done" {
		if r.PosTransactionID == "" || r.PosTransactionID == transactionID {
			return nil
		}
		return Invalid("reservasi sudah ditutup oleh transaksi lain (%s)", r.PosTransactionID)
	}
	if r.Status == "cancelled" {
		return Invalid("reservasi sudah dibatalkan")
	}
	if r.PendingAmount > 0 {
		return Invalid("masih ada bukti pembayaran yang menunggu validasi")
	}
	_, err = database.DB.Exec(`UPDATE reservations SET status='done', settled_at=(now() AT TIME ZONE 'UTC'),
		pos_transaction_id=$2, updated_at=NOW() WHERE id=$1`, reservationID, nilIfEmpty(transactionID))
	return err
}
