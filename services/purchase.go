package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

// Valid status transitions:
//
//	pending → verified → approved → payment_requested → paid/partial → received
//	pending/verified → rejected
//	pending/verified/approved → cancelled
//
// Tahap VERIFIKASI (pending → verified) memeriksa ketersediaan — mis. barang
// masih ada di outlet lain — sebelum pengajuan sampai ke penyetuju. Penyetuju
// hanya memutus pengajuan yang sudah diverifikasi (T-3). Siapa verifikator dan
// penyetujunya diatur lewat Role (procurement.requests.verify / .approve),
// tidak ada akun yang di-hardcode.
//
// Bahan baku TIDAK melalui verifikasi (keputusan owner 14 Okt 2026): Kepala
// Gudang Induk mengajukan, Manager Logistik (procurement.rawmaterials.approve)
// menyetujui. Pengajuannya lahir 'verified' (Menunggu Persetujuan) dan
// pengajunya masih boleh mengubah isinya sampai disetujui.
var validTransitions = map[string]map[string]string{
	"verify":  {"pending": "verified"},
	"approve": {"verified": "approved"},
	// Tolak bergantung tahap: saat pending = keputusan verifikator, saat
	// verified = keputusan penyetuju. Izinnya dipilih handler per tahap.
	"reject":          {"pending": "rejected", "verified": "rejected"},
	"request_payment": {"approved": "payment_requested"},
	// Pelunasan tetap boleh setelah barang diterima selama masih ada sisa
	// tagihan (T-12): dokumen yang dibayar sebagian lalu barangnya datang
	// lengkap menjadi 'received', dan sisanya dulu tidak bisa dibayar lagi.
	// Statusnya tetap 'received'; nilai bayar tidak boleh melebihi sisa.
	"pay": {"payment_requested": "paid", "partial": "paid", "received": "received"},
	// 'partial' ikut boleh diterima: barang yang baru dilunasi sebagian tetap
	// datang secara fisik. Sisa hutangnya tidak hilang karena perhitungan
	// hutang usaha memakai (total_final - paid_amount), bukan nama status.
	"receive": {"paid": "received", "partial": "received"},
	"cancel":  {"pending": "cancelled", "verified": "cancelled", "approved": "cancelled"},
}

// purchaseChangedMsg: balasan 409 bila dokumen berubah sejak dibuka.
const purchaseChangedMsg = "Pengajuan berubah, muat ulang"

// allItemsRejectedReason: alasan otomatis bila verifikator menolak semua item.
const allItemsRejectedReason = "Semua item ditolak saat verifikasi"

// VerifyReasonMinLen: alasan menolak item saat verifikasi minimal 5 huruf
// (dihitung per karakter setelah trim — sama dengan layar).
const VerifyReasonMinLen = 5

// purchaseVersion: penanda versi dokumen = updated_at dalam UTC, RFC3339Nano.
func purchaseVersion(updatedAt time.Time) string {
	return updatedAt.UTC().Format(time.RFC3339Nano)
}

// versionMatches membandingkan version kiriman dengan updated_at tersimpan
// pada presisi mikrodetik (presisi kolom TIMESTAMP Postgres).
func versionMatches(version string, updatedAt time.Time) bool {
	v, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(version))
	if err != nil {
		return false
	}
	return v.UTC().Truncate(time.Microsecond).Equal(updatedAt.UTC().Truncate(time.Microsecond))
}

// ── Pelaku ──────────────────────────────────────────────────────────────────

// Actor: pelaku sebuah aksi pengadaan. SELALU diturunkan dari sesi login
// (middleware), tidak pernah dari isi permintaan — dulu actor_name dan
// requested_by diambil dari body sehingga siapa pun bisa tercatat sebagai
// penyetuju owner (T-7).
type Actor struct {
	ID   string // ULID akun; "" bila bukan akun (token statis)
	Name string // nama tampilan akun (cloud_admins.name)
}

// ResolveActor menurunkan pelaku dari admin_id & admin_username sesi: nama =
// cloud_admins.name (cadangan: username); token statis = "admin" tanpa id.
func ResolveActor(adminID, username string) Actor {
	adminID, username = strings.TrimSpace(adminID), strings.TrimSpace(username)
	if adminID == "static" {
		return Actor{Name: "admin"}
	}
	a := Actor{ID: validActorID(adminID), Name: username}
	if a.ID != "" {
		var name string
		if err := database.DB.QueryRow(`SELECT name FROM cloud_admins WHERE id = $1`, a.ID).Scan(&name); err == nil {
			if n := strings.TrimSpace(name); n != "" {
				a.Name = n
			}
		}
	}
	if a.Name == "" {
		a.Name = "admin"
	}
	return a
}

// validActorID: id yang boleh disimpan di kolom *_by_id — id akun
// cloud_admins: huruf besar/angka, maksimal 26 karakter (ULID, atau id akun
// bawaan "01ADMIN000SUPERADMIN00001" yang 25 karakter). Selain itu ("static"
// dari token statis, kosong, rusak) → "" sehingga tersimpan NULL. Kolomnya
// sengaja tanpa FK ke cloud_admins supaya token statis tetap bisa dipakai.
func validActorID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 26 {
		return ""
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z') {
			return ""
		}
	}
	return id
}

// trimIDPtr membuang spasi pengisi CHAR(26) (id akun bawaan hanya 25 karakter).
func trimIDPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	return &s
}

// outstandingCond memilih baris yang masih menyisakan kewajiban bayar (tanpa
// alias tabel). Definisinya tinggal di procurement_finance.go bersama versi
// ber-alias, supaya halaman Pembayaran dan seluruh laporan keuangan tidak
// pernah lagi memakai tiga rumus hutang yang berbeda.
var outstandingCond = outstandingCondFor("")

// fullySplitMasterCond cocok untuk baris master yang seluruh itemnya sudah
// dipindah ke pecahan. Dokumen semacam itu tinggal cangkang: tidak boleh ikut
// dijumlahkan (dobel hitung dengan pecahannya) dan tidak boleh dibayar.
//
// split_status di-COALESCE karena kondisi ini hampir selalu dipakai sebagai
// "NOT (...)": dokumen biasa (split_status NULL) yang semua itemnya ditolak
// verifikator juga ber-items kosong, dan tanpa COALESCE hasilnya NULL — NOT
// NULL tetap NULL, sehingga dokumen ditolak itu hilang dari dashboard, daftar
// Pembayaran, dan daftar projek.
func fullySplitMasterCond(alias string) string {
	p := ""
	if alias != "" {
		p = alias + "."
	}
	return fmt.Sprintf("(COALESCE(%[1]ssplit_status, '') = 'master' AND COALESCE(jsonb_array_length(%[1]sitems), 0) = 0)", p)
}

// rowQuerier dipenuhi *sql.DB maupun *sql.Tx.
type rowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

// purchaseHasReceipts melaporkan apakah pengajuan ini (atau pecahannya) sudah
// punya wujud barang: receipt_status terisi, atau ada aset / GRN / material
// projek yang menunjuk ke sana.
//
// Begitu barang mulai diterima, dokumennya dikunci: tidak boleh dibatalkan,
// itemnya tidak boleh diubah, dan tidak boleh dihapus. Tanpa kunci ini,
// pembelian tempo yang barangnya sudah jadi aset bisa dibatalkan — hutang ke
// vendor lenyap dari laporan sementara barangnya ada di outlet; mengganti nama
// item mengubah pr_item_key sehingga baris yang sudah dicatat muncul lagi
// sebagai belum diterima dan bisa dicatat dua kali.
func purchaseHasReceipts(q rowQuerier, id string) bool {
	var has bool
	err := q.QueryRow(`
		WITH fam AS (SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)
		SELECT EXISTS(SELECT 1 FROM purchase_requests p WHERE p.id IN (SELECT id FROM fam) AND COALESCE(p.receipt_status,'') <> '')
		    OR EXISTS(SELECT 1 FROM assets a WHERE a.purchase_request_id IN (SELECT id FROM fam) AND a.is_deleted = false)
		    OR EXISTS(SELECT 1 FROM goods_receipts g WHERE g.purchase_request_id IN (SELECT id FROM fam))
		    OR EXISTS(SELECT 1 FROM project_materials m WHERE m.purchase_request_id IN (SELECT id FROM fam))`, id).Scan(&has)
	return err == nil && has
}

const receivedLockMsg = "barang pada pengajuan ini sudah mulai diterima dan tercatat (aset/stok/material), jadi dokumennya dikunci — %s"

// docHasOwnReceipts: baris milik dokumen ini SENDIRI (bukan pecahannya) sudah
// punya catatan penerimaan — aset, GRN, material projek, atau keputusan
// penerimaan apa pun (termasuk "stok, tunda ke gudang").
func docHasOwnReceipts(q rowQuerier, id string) (bool, error) {
	var has bool
	err := q.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM assets WHERE purchase_request_id = $1 AND is_deleted = false)
		    OR EXISTS(SELECT 1 FROM goods_receipts WHERE purchase_request_id = $1)
		    OR EXISTS(SELECT 1 FROM project_materials WHERE purchase_request_id = $1)
		    OR EXISTS(SELECT 1 FROM pr_receiving_decisions WHERE purchase_request_id = $1)`, id).Scan(&has)
	return has, err
}

// resolvePurchaseVendor: bila vendor_id dikirim, vendornya wajib ada dan aktif,
// dan namanya SELALU diambil dari tabel vendors — nama kiriman diabaikan
// (T-67: dulu nama "CV Sumber Resmi" bisa dipasangkan dengan rekening vendor
// lain). Tanpa vendor_id, nama bebas tetap diterima seperti sebelumnya.
func resolvePurchaseVendor(q rowQuerier, vendorID, vendorName string) (string, string, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return "", strings.TrimSpace(vendorName), nil
	}
	var name string
	var active sql.NullBool
	err := q.QueryRow(`SELECT name, is_active FROM vendors WHERE id = $1`, vendorID).Scan(&name, &active)
	if err == sql.ErrNoRows {
		return "", "", Invalid("vendor tidak ditemukan — pilih vendor dari daftar")
	}
	if err != nil {
		return "", "", err
	}
	if active.Valid && !active.Bool {
		return "", "", Invalid("vendor %s nonaktif — aktifkan dulu atau pilih vendor lain", strings.TrimSpace(name))
	}
	return vendorID, strings.TrimSpace(name), nil
}

// requireRegisteredVendorTx: pengajuan wajib menunjuk vendor terdaftar
// sebelum ditagihkan — rekening tujuan pembayarannya diambil dari data vendor.
func requireRegisteredVendorTx(tx *sql.Tx, id string) error {
	var vendorID, number string
	if err := tx.QueryRow(`SELECT COALESCE(vendor_id, ''), request_number FROM purchase_requests WHERE id = $1`, id).
		Scan(&vendorID, &number); err != nil {
		return fmt.Errorf("gagal memeriksa vendor: %w", err)
	}
	if strings.TrimSpace(vendorID) == "" {
		return Invalid("pengajuan %s belum memilih vendor dari daftar vendor — pilih vendornya lewat Isi Harga dulu; rekening tujuan pembayaran diambil dari data vendor", number)
	}
	return nil
}

// paymentDest: snapshot of the vendor and destination for one payment.
type paymentDest struct {
	vendorID, vendorName, bank, number, holder, text string
	method, marketplace, orderNumber                 string
	manual                                           bool
}

// vendorPaymentDestTx reads the registered vendor under a shared lock and
// snapshots its account, a manual destination when no account exists, or the
// explicit marketplace VA belonging to this payment/order.
func vendorPaymentDestTx(tx *sql.Tx, vendorID string, input models.UpdatePurchaseStatusInput) (paymentDest, error) {
	vendorID = strings.TrimSpace(vendorID)
	if vendorID == "" {
		return paymentDest{}, Invalid("pengajuan ini belum memilih vendor dari daftar vendor — rekening tujuan pembayaran diambil dari data vendor")
	}
	d := paymentDest{vendorID: vendorID}
	err := tx.QueryRow(`
		SELECT name, COALESCE(bank_name, ''), COALESCE(account_number, ''), COALESCE(account_holder, '')
		FROM vendors WHERE id = $1 FOR SHARE`, vendorID).Scan(&d.vendorName, &d.bank, &d.number, &d.holder)
	if err == sql.ErrNoRows {
		return paymentDest{}, Invalid("vendor pengajuan ini sudah tidak ada — pilih vendor dari daftar dulu")
	}
	if err != nil {
		return paymentDest{}, err
	}
	return resolvePaymentDestination(d, input)
}

// formatAccountDest: "BCA 1234567890 a.n. CV Sumber" — format yang sama dengan
// form bayar.
func formatAccountDest(bank, number, holder string) string {
	parts := []string{}
	if bank != "" {
		parts = append(parts, bank)
	}
	parts = append(parts, number)
	if holder != "" {
		parts = append(parts, "a.n. "+holder)
	}
	return strings.Join(parts, " ")
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// purchaseSelectCols: kolom SELECT pengajuan yang dipakai daftar, detail, dan
// pecahan — satu daftar supaya ketiganya tidak pernah berbeda isi. Argumen =
// ekspresi total_amount, total_hps, total_final (daftar biasa menjumlahkan
// pecahan ke master). Wajib dipasangkan dengan purchaseSelectJoins dan
// dibaca dengan scanPurchaseRequest.
func purchaseSelectCols(totalAmount, totalHps, totalFinal string) string {
	return fmt.Sprintf(`
		pr.id, pr.request_number, pr.outlet_id, o.name, pr.work_unit_id, COALESCE(wu.name,''),
		pr.request_type, pr.requested_by, pr.requested_by_id, pr.vendor_id, pr.vendor_name, pr.status,
		pr.items, %s, %s, %s, pr.notes, pr.invoice_number, COALESCE(pr.invoice_url, ''),
		COALESCE(pr.verified_by,''), pr.verified_by_id, pr.verified_at, COALESCE(pr.verification_notes,''),
		pr.approved_by, pr.approved_by_id, pr.approved_at,
		pr.rejected_reason, COALESCE(pr.rejected_by,''), pr.rejected_at, COALESCE(pr.rejected_items, '[]'::jsonb),
		pr.paid_by, pr.paid_at, pr.payment_proof,
		pr.payment_account_dest, pr.payment_account_source, pr.payment_notes,
		pr.paid_amount,
		pr.received_by, pr.received_at, COALESCE(pr.receipt_status, ''),
		pr.parent_id, COALESCE(ppr.request_number,''), pr.split_status,
		pr.project_id, COALESCE(prj.name,''),
		pr.created_at, pr.updated_at,
		pr.pay_to_method, pr.pay_to_marketplace, pr.pay_to_order_number, pr.pay_to_bank_name,
		pr.pay_to_account_number, pr.pay_to_account_holder, pr.pay_to_text, pr.pay_to_note,
		pr.pay_to_set_by, pr.pay_to_set_at, pr.pay_to_returned_at, pr.pay_to_returned_by, pr.pay_to_return_reason,
		COALESCE(pvd.bank_name,''), COALESCE(pvd.account_number,''), COALESCE(pvd.account_holder,''),
		pr.urgency, pr.need_by_date, pr.emergency, pr.emergency_category, pr.emergency_reason, pr.emergency_recipient,
		pr.emergency_proof_url, pr.emergency_proof_total, pr.emergency_proof_notes, pr.emergency_proof_by, pr.emergency_proof_at,
		pr.ratification_status, pr.ratified_by, pr.ratified_at, pr.ratification_notes,
		COALESCE(pr.is_manual_payment, false), COALESCE(pr.payment_approval_status, ''), COALESCE(pr.payment_batch_number, ''),
		COALESCE(pr.payment_submitted_by, ''), pr.payment_submitted_at, COALESCE(pr.payment_submission_notes, ''),
		COALESCE(pr.payment_approved_by, ''), pr.payment_approved_at,
		COALESCE(pr.payment_rejected_by, ''), pr.payment_rejected_at, COALESCE(pr.payment_rejection_reason, '')`,
		totalAmount, totalHps, totalFinal)
}

const purchaseSelectJoins = `
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		LEFT JOIN purchase_requests ppr ON ppr.id = pr.parent_id
		LEFT JOIN projects prj ON prj.id = pr.project_id
		LEFT JOIN vendors pvd ON pvd.id = pr.vendor_id`

func fmtTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// scanPurchaseRequest membaca satu baris purchaseSelectCols.
func scanPurchaseRequest(sc interface{ Scan(...interface{}) error }) (models.PurchaseRequest, error) {
	var r models.PurchaseRequest
	var itemsJSON, rejectedJSON []byte
	var verifiedAt, approvedAt, rejectedAt, paidAt, receivedAt *time.Time
	var createdAt, updatedAt time.Time
	var st payToStored
	var vendorAcct vendorAccount
	var needBy, proofAt, ratifiedAt *time.Time
	var paySubAt, payAppAt, payRejAt *time.Time
	var emergency bool
	var em models.PurchaseEmergency
	var proofTotal sql.NullFloat64

	if err := sc.Scan(
		&r.ID, &r.RequestNumber, &r.OutletID, &r.OutletName, &r.WorkUnitID, &r.WorkUnitName,
		&r.RequestType, &r.RequestedBy, &r.RequestedByID, &r.VendorID, &r.VendorName, &r.Status,
		&itemsJSON, &r.TotalAmount, &r.TotalHps, &r.TotalFinal, &r.Notes, &r.InvoiceNumber, &r.InvoiceURL,
		&r.VerifiedBy, &r.VerifiedByID, &verifiedAt, &r.VerificationNotes,
		&r.ApprovedBy, &r.ApprovedByID, &approvedAt,
		&r.RejectedReason, &r.RejectedBy, &rejectedAt, &rejectedJSON,
		&r.PaidBy, &paidAt, &r.PaymentProof,
		&r.PaymentAccountDest, &r.PaymentAccountSource, &r.PaymentNotes,
		&r.PaidAmount,
		&r.ReceivedBy, &receivedAt, &r.ReceiptStatus,
		&r.ParentID, &r.ParentNumber, &r.SplitStatus,
		&r.ProjectID, &r.ProjectName,
		&createdAt, &updatedAt,
		&st.method, &st.marketplace, &st.orderNumber, &st.bank,
		&st.number, &st.holder, &st.text, &st.note,
		&st.setBy, &st.setAt, &st.returnedAt, &st.returnedBy, &st.returnReason,
		&vendorAcct.bank, &vendorAcct.number, &vendorAcct.holder,
		&r.Urgency, &needBy, &emergency, &em.Category, &em.Reason, &em.Recipient,
		&em.ProofURL, &proofTotal, &em.ProofNotes, &em.ProofBy, &proofAt,
		&em.Ratification, &em.RatifiedBy, &ratifiedAt, &em.RatificationNotes,
		&r.IsManualPayment, &r.PaymentApprovalStatus, &r.PaymentBatchNumber,
		&r.PaymentSubmittedBy, &paySubAt, &r.PaymentSubmissionNotes,
		&r.PaymentApprovedBy, &payAppAt,
		&r.PaymentRejectedBy, &payRejAt, &r.PaymentRejectionReason,
	); err != nil {
		return r, err
	}
	r.PaymentSubmittedAt = fmtTimePtr(paySubAt)
	r.PaymentApprovedAt = fmtTimePtr(payAppAt)
	r.PaymentRejectedAt = fmtTimePtr(payRejAt)
	if needBy != nil {
		s := needBy.Format("2006-01-02")
		r.NeedByDate = &s
	}
	if emergency {
		em.CategoryLabel = EmergencyCategoryLabel(em.Category)
		if proofTotal.Valid {
			v := proofTotal.Float64
			em.ProofTotal = &v
		}
		em.ProofAt = fmtTimePtr(proofAt)
		em.RatifiedAt = fmtTimePtr(ratifiedAt)
		r.Emergency = &em
	}
	vendorID := ""
	if r.VendorID != nil {
		vendorID = *r.VendorID
	}
	r.PayTo = buildPayTo(st, vendorID, vendorAcct)
	if r.Emergency != nil {
		// Tidak pernah ditransfer Kantor Pusat: tunai lewat tablet kasir.
		r.PayTo.Ready = false
		r.PayTo.Problem = "Pembelian Darurat dibayar tunai dari Kas Kasir Outlet lewat tablet kasir"
	}

	json.Unmarshal(itemsJSON, &r.Items)
	if r.Items == nil {
		r.Items = []models.PurchaseRequestItem{}
	}
	json.Unmarshal(rejectedJSON, &r.RejectedItems)
	if r.RejectedItems == nil {
		r.RejectedItems = []models.RejectedPurchaseItem{}
	}

	r.RequestedByID = trimIDPtr(r.RequestedByID)
	r.VerifiedByID = trimIDPtr(r.VerifiedByID)
	r.ApprovedByID = trimIDPtr(r.ApprovedByID)

	r.CreatedAt = createdAt.Format(time.RFC3339)
	r.UpdatedAt = updatedAt.Format(time.RFC3339)
	r.Version = purchaseVersion(updatedAt)
	r.VerifiedAt = fmtTimePtr(verifiedAt)
	r.ApprovedAt = fmtTimePtr(approvedAt)
	r.RejectedAt = fmtTimePtr(rejectedAt)
	r.PaidAt = fmtTimePtr(paidAt)
	r.ReceivedAt = fmtTimePtr(receivedAt)
	return r, nil
}

// recalcItems menghitung ulang seluruh subtotal dan total grup di tempat, lalu
// mengembalikan total dokumen. Satu-satunya tempat rumus ini hidup — Create,
// Update, dan Split memakainya supaya hasilnya tidak mungkin berbeda.
func recalcItems(items []models.PurchaseRequestItem) (totalHps, totalFinal float64) {
	for i := range items {
		var groupHps, groupFinal float64
		for j := range items[i].Items {
			sub := &items[i].Items[j]
			sub.HpsSubtotal = float64(sub.Qty) * sub.HpsPrice
			sub.FinalSubtotal = float64(sub.PurchaseQuantity()) * sub.FinalPrice
			groupHps += sub.HpsSubtotal
			groupFinal += sub.FinalSubtotal
		}
		items[i].HpsTotal = groupHps
		items[i].FinalTotal = groupFinal
		totalHps += groupHps
		totalFinal += groupFinal
	}
	return totalHps, totalFinal
}

// clearFinalPrices mengosongkan jumlah beli aktual, harga final, subtotal final,
// dan total final grup. Data pembelian hanya diisi setelah disetujui (T-39):
// angka kiriman pengaju — saat membuat maupun mengubah pengajuan pending —
// selalu diabaikan, supaya hutang, akrual, dan tagihan tidak pernah memakai
// harga pilihan pengaju sendiri.
func clearFinalPrices(items []models.PurchaseRequestItem) {
	for i := range items {
		items[i].FinalTotal = 0
		for j := range items[i].Items {
			items[i].Items[j].PurchasedQty = nil
			items[i].Items[j].FinalPrice = 0
			items[i].Items[j].FinalSubtotal = 0
		}
	}
}

// amountOf memilih nilai untuk total_amount: harga final bila sudah diisi,
// selain itu jatuh kembali ke HPS.
func amountOf(totalHps, totalFinal float64) float64 {
	if totalFinal == 0 {
		return totalHps
	}
	return totalFinal
}

// validateItems menolak dokumen pengadaan yang tidak masuk akal. Dipakai saat
// membuat maupun mengubah item — sebelumnya hanya jalur Create yang memvalidasi,
// sehingga qty 0/negatif dan harga negatif bisa masuk lewat endpoint update.
func validateItems(items []models.PurchaseRequestItem) error {
	if len(items) == 0 {
		return fmt.Errorf("minimal 1 pengadaan harus diisi")
	}
	for _, g := range items {
		if strings.TrimSpace(g.Name) == "" || len(g.Items) == 0 {
			return fmt.Errorf("setiap pengadaan harus memiliki nama dan minimal 1 item")
		}
		for _, s := range g.Items {
			if strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("nama item pada pengadaan %q wajib diisi", g.Name)
			}
			if s.Qty <= 0 {
				return fmt.Errorf("qty item %q harus lebih dari 0", s.Name)
			}
			if s.HpsPrice < 0 || s.FinalPrice < 0 {
				return fmt.Errorf("harga item %q tidak boleh negatif", s.Name)
			}
		}
	}
	return nil
}

// Panjang teks baris barang dibatasi kolom tujuan serah terimanya: nama aset &
// material projek VARCHAR(150), satuan VARCHAR(20). Dulu nama panjang baru
// gagal saat serah terima — setelah dokumen disetujui dan dibayar — dan
// dokumennya tidak bisa diperbaiki lagi (T-80). Bahan baku hanya diterima ke
// stok (nama & satuan GRN diambil dari katalog), jadi batasnya cukup untuk
// kunci baris.
const (
	maxGoodsItemNameLen  = 150
	maxGoodsUnitLen      = 20
	maxGoodsGroupNameLen = 200
	maxBahanNameLen      = 200
)

func validateReceivableItemText(items []models.PurchaseRequestItem, requestType string) error {
	nameMax, groupMax, unitMax := 0, 0, 0
	switch requestType {
	case RequestTypeBarang:
		nameMax, groupMax, unitMax = maxGoodsItemNameLen, maxGoodsGroupNameLen, maxGoodsUnitLen
	case RequestTypeBahan:
		nameMax, groupMax = maxBahanNameLen, maxBahanNameLen
	default:
		return nil // jasa tidak diserahterimakan sebagai barang
	}
	for _, g := range items {
		if utf8.RuneCountInString(strings.TrimSpace(g.Name)) > groupMax {
			return Invalid("nama pengadaan %q terlalu panjang (maksimal %d huruf)", g.Name, groupMax)
		}
		for _, s := range g.Items {
			if utf8.RuneCountInString(strings.TrimSpace(s.Name)) > nameMax {
				return Invalid("nama item %q terlalu panjang (maksimal %d huruf)", s.Name, nameMax)
			}
			if unitMax > 0 && utf8.RuneCountInString(strings.TrimSpace(s.Unit)) > unitMax {
				return Invalid("satuan item %q terlalu panjang (maksimal %d huruf)", s.Name, unitMax)
			}
		}
	}
	return nil
}

// generateRequestNumber creates a sequential number in the format ddmmYYnnn.
//
// Urutan per hari disimpan di purchase_request_counters, bukan dihitung dari
// MAX() dokumen yang tersisa: dulu nomor dokumen terakhir hari itu yang
// dihapus dipakai ulang oleh pengajuan berikutnya, sehingga notifikasi WA dan
// catatan lama menunjuk dua dokumen berbeda. MAX() tetap dibaca supaya
// penghitung tidak pernah tertinggal dari nomor yang sudah ada (dokumen yang
// dibuat sebelum tabel ini ada). Upsert mengunci baris hari itu, jadi dua
// pengajuan bersamaan tidak lagi memperoleh nomor yang sama. Nomor yang sudah
// diambil tetap terpakai walau insert-nya gagal — lebih baik berlubang
// daripada kembar.
func generateRequestNumber(t time.Time) (string, error) {
	prefix := t.Format("020106") // ddmmYY
	var seq int
	err := database.DB.QueryRow(`
		WITH existing AS (
			SELECT COALESCE(MAX(CAST(SUBSTRING(request_number FROM 7) AS INTEGER)), 0) AS max_seq
			FROM purchase_requests WHERE request_number LIKE ($1::text || '%')
		)
		INSERT INTO purchase_request_counters AS c (prefix, last_seq)
		SELECT $1::text, max_seq + 1 FROM existing
		ON CONFLICT (prefix) DO UPDATE
			SET last_seq = GREATEST(c.last_seq, (SELECT max_seq FROM existing)) + 1
		RETURNING last_seq`,
		prefix,
	).Scan(&seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// insertWithRequestNumber menjalankan insert memakai nomor pengajuan baru dan
// mengulang bila unique index menolaknya. Nomor dihitung dari MAX() tanpa lock,
// jadi dua pengajuan bersamaan bisa memperoleh nomor yang sama.
//
// insert dipanggil berkali-kali, jadi pemanggil yang bekerja di dalam transaksi
// wajib membungkus statement-nya dengan SAVEPOINT — di Postgres satu statement
// gagal membuat seluruh transaksi abort.
func insertWithRequestNumber(now time.Time, insert func(reqNumber string) error) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		reqNumber, err := generateRequestNumber(now)
		if err != nil {
			return fmt.Errorf("gagal generate nomor pengajuan: %w", err)
		}
		lastErr = insert(reqNumber)
		if lastErr == nil {
			return nil
		}
		if !strings.Contains(lastErr.Error(), "uq_purchase_requests_number") {
			return lastErr
		}
	}
	return lastErr
}

// allowedTypes membatasi jenis pengajuan yang boleh muncul (izin per jenis,
// lihat ViewableRequestTypes). nil = tanpa batas; slice kosong = tidak ada
// jenis yang boleh dilihat sehingga hasilnya kosong.
func ListPurchaseRequests(outletID, workUnitID, status, requestType, projectID, parentID string, excludeMasters bool, search string, scopeIDs []string, wuScopeIDs []string, allowedTypes []string, page, limit int) (*models.PurchaseRequestListResponse, error) {
	return ListPurchaseRequestsExt(outletID, workUnitID, status, requestType, projectID, parentID, excludeMasters, search,
		scopeIDs, wuScopeIDs, allowedTypes, page, limit, PurchaseListExtra{})
}

// PurchaseListExtra: saringan & urutan tambahan daftar pengajuan (layar).
type PurchaseListExtra struct {
	Urgency         string // rutin | prioritas | urgent; "" = semua
	Ratification    string // pending | ratified | rejected; "" = semua
	Emergency       bool   // hanya Pembelian Darurat
	PaymentApproval string // unsubmitted | pending | approved | rejected; "" = semua
	ManualOnly      bool   // hanya Pembayaran Manual (di luar pengajuan)
	// UrgentFirst: pengajuan URGENT lalu PRIORITAS yang masih berjalan
	// (belum dibayar) tampil paling atas; sisanya terbaru dulu. Ekspor
	// tetap kronologis.
	UrgentFirst bool
}

// purchaseOpenUrgencyRank: peringkat antrean — 0 urgent, 1 prioritas
// (menurut tanggal butuh), 2 sisanya. Hanya dokumen yang masih menunggu
// tindakan sebelum dibayar.
const purchaseOpenUrgencyRank = `CASE WHEN pr.status IN ('pending','verified','approved','payment_requested')
	THEN CASE pr.urgency WHEN 'urgent' THEN 0 WHEN 'prioritas' THEN 1 ELSE 2 END ELSE 2 END`

func ListPurchaseRequestsExt(outletID, workUnitID, status, requestType, projectID, parentID string, excludeMasters bool, search string, scopeIDs []string, wuScopeIDs []string, allowedTypes []string, page, limit int, extra PurchaseListExtra) (*models.PurchaseRequestListResponse, error) {
	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if requestType != "" {
		where += fmt.Sprintf(" AND pr.request_type = $%d", idx)
		args = append(args, requestType)
		idx++
	}
	if allowedTypes != nil {
		where += fmt.Sprintf(" AND pr.request_type = ANY($%d::text[])", idx)
		args = append(args, pq.Array(allowedTypes))
		idx++
	}

	// Scope filtering: use outlet_id OR work_unit_id for scoped roles
	if filterIDs != nil && wuScopeIDs != nil && len(wuScopeIDs) > 0 {
		// Scoped: show purchase requests matching outlet_ids OR work_unit_ids
		if len(filterIDs) > 0 {
			where += fmt.Sprintf(" AND (pr.outlet_id = ANY($%d::text[]) OR pr.work_unit_id = ANY($%d::text[]))", idx, idx+1)
			args = append(args, pq.Array(filterIDs), pq.Array(wuScopeIDs))
			idx += 2
		} else {
			where += fmt.Sprintf(" AND pr.work_unit_id = ANY($%d::text[])", idx)
			args = append(args, pq.Array(wuScopeIDs))
			idx++
		}
	} else if filterIDs != nil && len(filterIDs) > 0 {
		where += fmt.Sprintf(" AND pr.outlet_id = ANY($%d::text[])", idx)
		args = append(args, pq.Array(filterIDs))
		idx++
	} else if filterIDs != nil && len(filterIDs) == 0 {
		// Scoped but no outlets and no work units — return empty
		where += " AND FALSE"
	}
	if workUnitID != "" {
		where += fmt.Sprintf(" AND pr.work_unit_id = $%d", idx)
		args = append(args, workUnitID)
		idx++
	}
	if status == PurchaseStatusCommittedFilter {
		where += " AND " + prCommittedCond("pr")
	} else if status != "" {
		where += fmt.Sprintf(" AND pr.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if projectID != "" {
		where += fmt.Sprintf(" AND pr.project_id = $%d", idx)
		args = append(args, projectID)
		idx++
	}
	if extra.Urgency != "" {
		where += fmt.Sprintf(" AND pr.urgency = $%d", idx)
		args = append(args, extra.Urgency)
		idx++
	}
	if extra.Ratification != "" {
		where += fmt.Sprintf(" AND pr.emergency AND pr.ratification_status = $%d", idx)
		args = append(args, extra.Ratification)
		idx++
	}
	if extra.Emergency {
		where += " AND pr.emergency"
	}
	if extra.ManualOnly {
		where += " AND COALESCE(pr.is_manual_payment, false)"
	}
	switch extra.PaymentApproval {
	case "unsubmitted":
		where += " AND COALESCE(pr.payment_approval_status, '') = ''"
	case "pending", "approved", "rejected":
		where += fmt.Sprintf(" AND COALESCE(pr.payment_approval_status, '') = $%d", idx)
		args = append(args, extra.PaymentApproval)
		idx++
	}

	// Default: only show main requests (not children) unless explicitly filtering for children or parents
	if parentID == "" {
		where += " AND pr.parent_id IS NULL"
	} else if parentID != "all" {
		where += fmt.Sprintf(" AND pr.parent_id = $%d", idx)
		args = append(args, parentID)
		idx++
	}

	if excludeMasters {
		// Halaman Pembayaran menampilkan tiap pecahan sendiri-sendiri, jadi
		// master tidak boleh ikut muncul dan menggandakan nominal. Master yang
		// masih memegang sisa item tetap ditampilkan — sisa itu tagihan nyata
		// yang kalau disembunyikan tak akan pernah bisa dibayar.
		where += " AND NOT " + fullySplitMasterCond("pr")
	} else {
		// Halaman pengajuan pengadaan (/purchase-goods, /purchase-materials, /purchase-services)
		// tidak menampilkan Pembayaran Manual yang dibuat langsung oleh Keuangan.
		where += " AND NOT COALESCE(pr.is_manual_payment, false)"
	}

	if search != "" {
		// Kata kunci dicocokkan sampai ke nama sub-item; sebelumnya hanya nama
		// grup pengadaan yang dicari sehingga mencari nama barang tidak ketemu.
		where += fmt.Sprintf(` AND (pr.request_number ILIKE $%d OR pr.vendor_name ILIKE $%d OR COALESCE(pr.payment_batch_number, '') ILIKE $%d
			OR EXISTS (
				SELECT 1 FROM jsonb_array_elements(pr.items) AS grp
				LEFT JOIN LATERAL jsonb_array_elements(COALESCE(grp->'items', '[]'::jsonb)) AS sub ON TRUE
				WHERE grp->>'name' ILIKE $%d OR sub->>'name' ILIKE $%d
			))`, idx, idx, idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	// Count
	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM purchase_requests pr %s", where)
	if err := database.DB.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	offset := (page - 1) * limit

	// Fetch
	// Halaman Pembayaran (excludeMasters) menagih baris per baris, jadi master
	// hanya boleh menunjukkan sisa miliknya sendiri. Di daftar pengajuan biasa
	// master mewakili seluruh dokumen: sisa sendiri + semua pecahan.
	totalExpr := func(col string) string {
		if excludeMasters {
			return "pr." + col
		}
		return fmt.Sprintf(
			"pr.%[1]s + CASE WHEN pr.split_status = 'master' THEN COALESCE((SELECT SUM(c.%[1]s) FROM purchase_requests c WHERE c.parent_id = pr.id), 0) ELSE 0 END",
			col)
	}
	orderBy := "pr.created_at DESC"
	if extra.UrgentFirst {
		orderBy = purchaseOpenUrgencyRank + `,
		CASE WHEN ` + purchaseOpenUrgencyRank + ` < 2 THEN pr.need_by_date END ASC NULLS LAST,
		pr.created_at DESC`
	}
	query := fmt.Sprintf(`
		SELECT %s
		%s
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, purchaseSelectCols(totalExpr("total_amount"), totalExpr("total_hps"), totalExpr("total_final")),
		purchaseSelectJoins, where, orderBy, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]models.PurchaseRequest, 0)
	for rows.Next() {
		r, err := scanPurchaseRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	attachCashPayouts(requests)

	return &models.PurchaseRequestListResponse{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		Requests:   requests,
	}, nil
}

func CreatePurchaseRequest(input models.CreatePurchaseRequestInput) (*models.PurchaseRequest, error) {
	// Harga final dari pengaju diabaikan (T-39) — disalin dulu supaya slice
	// milik pemanggil tidak ikut berubah.
	input.Items = clonePurchaseItems(input.Items)
	clearFinalPrices(input.Items)
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}
	// Bahan baku selalu barang dapur dan tidak pernah dibelanjakan untuk
	// projek: projek membeli material lewat pengadaan barang/jasa ber-RAB.
	if input.RequestType == RequestTypeBahan && strings.TrimSpace(input.ProjectID) != "" {
		return nil, Invalid("Pengadaan bahan baku tidak terkait projek")
	}
	// Pemilik pengajuan: outlet SELALU diturunkan dari unit kerja (T-9) —
	// kombinasi outlet A + unit B ditolak.
	outletID, workUnitID, err := resolvePurchaseOwner(input.OutletID, input.WorkUnitID)
	if err != nil {
		return nil, err
	}
	// Vendor yang dikirim saat pengajuan (hanya lewat API — form, MRP, dan
	// work order tidak mengirimnya) diperiksa seperti saat pecah vendor & isi
	// harga (T-67): wajib ada dan aktif, namanya dari tabel vendors. Dulu
	// vendor_id asal-asalan gagal di foreign key sebagai HTTP 500, dan nama
	// bebas tersimpan berdampingan dengan id vendor lain.
	if input.VendorID, input.VendorName, err = resolvePurchaseVendor(database.DB, input.VendorID, input.VendorName); err != nil {
		return nil, err
	}
	// Belanja projek hanya boleh lahir setelah RAB-nya ditetapkan, dan tiap
	// rujukan baris RAB harus milik projek itu dengan jenis yang cocok.
	if err := validateProjectRab(input.ProjectID, input.RequestType, input.Items, true); err != nil {
		return nil, err
	}
	// ...dan dari unit/outlet pemilik projek itu (T-24).
	if err := validateProjectOwner(input.ProjectID, outletID, workUnitID); err != nil {
		return nil, err
	}
	if err := validateReceivableItemText(input.Items, input.RequestType); err != nil {
		return nil, err
	}
	// Tingkat kebutuhan (keputusan owner 19 Nov 2026): prioritas & urgent
	// wajib bertanggal butuh; urgent wajib menjelaskan dampaknya. Jalur
	// otomatis (MRP, work order) tidak mengirimnya → rutin.
	urgency, needBy, err := normalizeUrgency(input.Urgency, input.NeedByDate, input.Notes, time.Now())
	if err != nil {
		return nil, err
	}
	// Pengadaan barang dipisah sejak awal: dapur ke Gudang Induk, peralatan ke
	// bagian Aset. Lihat ClassifyPurchaseItems untuk alasannya. Bahan baku
	// tidak diklasifikasi: seluruh barisnya urusan Gudang Induk, termasuk
	// bahan yang belum ada di katalog stok. Jenis tiap baris DIBEKUKAN di
	// item (goods_kind per baris) — meja penerimaan dan kelas keuangan
	// membaca nilai tersimpan ini, bukan katalog yang berlaku nanti (T-19).
	goodsKind, err := stampPurchaseItemKinds(input.Items, input.RequestType)
	if err != nil {
		return nil, err
	}

	id := NewULID()
	totalHps, totalFinal := recalcItems(input.Items)

	itemsJSON, err := json.Marshal(input.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// Pengajuan barang lahir 'pending' dan wajib melewati verifikasi (cek
	// ketersediaan di outlet lain) sebelum sampai ke penyetuju.
	// Bahan baku & jasa tanpa verifikasi: lahir 'verified' (Menunggu Persetujuan).
	initialStatus := "pending"
	if SkipsVerification(input.RequestType) {
		initialStatus = "verified"
	}
	now := time.Now().UTC()
	err = insertWithRequestNumber(now, func(reqNumber string) error {
		tx, err := database.DB.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if input.ProjectID != "" {
			if err = lockMaterialProject(tx, input.ProjectID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`
			INSERT INTO purchase_requests (id, request_number, outlet_id, work_unit_id, request_type, requested_by, requested_by_id, vendor_id, vendor_name, status, items, total_amount, total_hps, total_final, notes, project_id, goods_kind, created_at, updated_at, urgency, need_by_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $18, $10, $11, $12, $13, $14, $15, $16, $17, $17, $19, $20)
		`, id, reqNumber, nilIfEmpty(outletID), nilIfEmpty(workUnitID), input.RequestType, input.RequestedBy,
			nilIfEmpty(validActorID(input.RequestedByID)),
			nilIfEmpty(input.VendorID), input.VendorName, itemsJSON,
			amountOf(totalHps, totalFinal), totalHps, totalFinal, input.Notes, nilIfEmpty(input.ProjectID), goodsKind, now,
			initialStatus, urgency, needBy)
		if err != nil {
			return err
		}
		return tx.Commit()
	})
	if err != nil {
		return nil, err
	}

	pr, err := GetPurchaseRequest(id)
	if err == nil {
		go NotifyPurchaseEvent(pr, "submitted", input.RequestedBy, "", 0)
	}
	return pr, err
}

// resolvePurchaseOwner menurunkan pasangan (outlet, unit kerja) pengajuan.
//
//   - unit dikirim → outlet = outlet milik unit itu; outlet kiriman yang
//     berbeda ditolak (T-9: dulu outlet A + unit B diterima begitu saja);
//   - hanya outlet (jalur otomatis: MRP gudang outlet, work order aset) →
//     unitnya diturunkan dari outlet (satu outlet = satu unit kerja), supaya
//     pengajuan ikut terlihat oleh role yang dibatasi per unit;
//   - keduanya kosong → pengajuan pusat (mis. MRP Gudang Induk).
//
// Kewajiban memilih unit untuk pengajuan manual dijaga handler.
func resolvePurchaseOwner(outletID, workUnitID string) (string, string, error) {
	outletID, workUnitID = strings.TrimSpace(outletID), strings.TrimSpace(workUnitID)
	if workUnitID != "" {
		var unitOutlet sql.NullString
		if err := database.DB.QueryRow(`SELECT outlet_id FROM work_units WHERE id = $1`, workUnitID).Scan(&unitOutlet); err != nil {
			if err == sql.ErrNoRows {
				return "", "", Invalid("unit kerja tidak ditemukan")
			}
			return "", "", err
		}
		derived := strings.TrimSpace(unitOutlet.String)
		if outletID != "" && outletID != derived {
			return "", "", Invalid("outlet tidak sesuai dengan unit kerja yang dipilih — outlet pengajuan selalu mengikuti unit kerjanya")
		}
		return derived, workUnitID, nil
	}
	if outletID != "" {
		var wu sql.NullString
		database.DB.QueryRow(`SELECT id FROM work_units WHERE outlet_id = $1 ORDER BY id LIMIT 1`, outletID).Scan(&wu)
		return outletID, strings.TrimSpace(wu.String), nil
	}
	return "", "", nil
}

// validateProjectOwner: belanja projek wajib diajukan dari unit/outlet pemilik
// projek itu (T-24) — anggaran projek satu outlet tidak boleh dihabiskan
// outlet lain tanpa disadari penyetuju. Hanya berlaku bila pengajuan punya
// pemilik (jalur form selalu punya unit; jalur internal tanpa projek).
func validateProjectOwner(projectID, outletID, workUnitID string) error {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || (outletID == "" && workUnitID == "") {
		return nil
	}
	var pOutlet, pUnit sql.NullString
	var number string
	if err := database.DB.QueryRow(`SELECT outlet_id, work_unit_id, COALESCE(project_number, '') FROM projects WHERE id = $1`, projectID).
		Scan(&pOutlet, &pUnit, &number); err != nil {
		if err == sql.ErrNoRows {
			return Invalid("projek tidak ditemukan")
		}
		return err
	}
	if u := strings.TrimSpace(pUnit.String); u != "" && u != workUnitID {
		return Invalid("projek %s milik unit kerja lain — ajukan belanjanya dari unit kerja projek tersebut", number)
	}
	if o := strings.TrimSpace(pOutlet.String); o != "" && o != outletID {
		return Invalid("projek %s milik outlet lain — ajukan belanjanya dari unit kerja outlet projek tersebut", number)
	}
	return nil
}

func GetPurchaseRequest(id string) (*models.PurchaseRequest, error) {
	r, err := scanPurchaseRequest(database.DB.QueryRow(`
		SELECT `+purchaseSelectCols("pr.total_amount", "pr.total_hps", "pr.total_final")+
		purchaseSelectJoins+`
		WHERE pr.id = $1
	`, id))
	if err != nil {
		return nil, err
	}

	// Master menampilkan nilai seluruh dokumen: sisa item yang masih dipegang
	// sendiri ditambah semua pecahannya. Dulu total master ditimpa mentah oleh
	// SUM(pecahan), sehingga item yang belum di-split hilang dari angka.
	if r.SplitStatus != nil && *r.SplitStatus == "master" {
		children, err := getPurchaseChildren(id)
		if err == nil {
			r.Children = children
			for _, ch := range children {
				r.TotalAmount += ch.TotalAmount
				r.TotalHps += ch.TotalHps
				r.TotalFinal += ch.TotalFinal
			}
		}
	}

	// Struk pembelian & pengambilan barang milik dokumen ini sendiri
	// (pecahan punya struk & pengambilannya masing-masing).
	if r.Struks, err = ListPurchaseStruks(database.DB, id); err != nil {
		return nil, err
	}
	if r.Pickups, err = ListPurchasePickups(database.DB, id); err != nil {
		return nil, err
	}
	one := []models.PurchaseRequest{r}
	attachCashPayouts(one)
	r.CashPayout = one[0].CashPayout

	return &r, nil
}

// getPurchaseChildren fetches all split children for a master request.
func getPurchaseChildren(parentID string) ([]models.PurchaseRequest, error) {
	rows, err := database.DB.Query(`
		SELECT `+purchaseSelectCols("pr.total_amount", "pr.total_hps", "pr.total_final")+
		purchaseSelectJoins+`
		WHERE pr.parent_id = $1
		ORDER BY pr.created_at ASC
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	children := make([]models.PurchaseRequest, 0)
	for rows.Next() {
		r, err := scanPurchaseRequest(rows)
		if err != nil {
			return nil, err
		}
		children = append(children, r)
	}
	return children, rows.Err()
}

func SplitPurchaseRequest(parentID string, input models.SplitPurchaseRequestInput) (*models.PurchaseRequest, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// 1. Get parent PR details — dikunci supaya status yang diperiksa di bawah
	// masih berlaku saat pecahan ditulis (mis. tidak sedang dibatalkan).
	var p models.PurchaseRequest
	var itemsJSON []byte
	err = tx.QueryRow(`
		SELECT outlet_id, work_unit_id, request_type, requested_by, items, total_hps, total_final, status, request_number, project_id, parent_id
		FROM purchase_requests WHERE id = $1 FOR UPDATE
	`, parentID).Scan(&p.OutletID, &p.WorkUnitID, &p.RequestType, &p.RequestedBy, &itemsJSON, &p.TotalHps, &p.TotalFinal, &p.Status, &p.RequestNumber, &p.ProjectID, &p.ParentID)
	if err != nil {
		return nil, fmt.Errorf("pengajuan induk tidak ditemukan")
	}

	// Pecah vendor adalah pekerjaan pembelian, yang baru dimulai setelah
	// pengajuan diverifikasi dan disetujui. Dulu pending pun boleh dipecah,
	// sehingga pecahan lahir tanpa pernah melewati verifikasi maupun
	// persetujuan (T-33). Pecahan juga tidak dipecah lagi: cascade, sinkron
	// status, dan total keluarga hanya mengenal satu tingkat.
	if p.Status != "approved" {
		return nil, fmt.Errorf("hanya pengajuan yang sudah disetujui yang bisa di-split vendor")
	}
	if p.ParentID != nil && strings.TrimSpace(*p.ParentID) != "" {
		return nil, fmt.Errorf("pecahan vendor tidak bisa dipecah lagi — pecah dari dokumen induknya")
	}
	// Barang yang sudah mulai dicatat (aset, stok, material, keputusan
	// penerimaan) tidak boleh dipindah ke dokumen lain: catatannya melekat pada
	// pasangan (dokumen ini, kunci baris). Dulu pecahannya muncul di antrean
	// dengan sisa penuh dan diterima ulang — meja tercatat 4 padahal dibeli 2
	// (T-31).
	if has, err := docHasOwnReceipts(tx, parentID); err != nil {
		return nil, err
	} else if has {
		return nil, Invalid(receivedLockMsg, "itemnya tidak bisa dipecah ke vendor lain")
	}
	// Vendor pecahan diambil dari daftar vendor bila id-nya dikirim (T-67):
	// nama yang dibaca penyetuju & keuangan selalu nama entitas yang dibayar.
	vendorID, vendorName, err := resolvePurchaseVendor(tx, input.VendorID, input.VendorName)
	if err != nil {
		return nil, err
	}
	input.VendorID, input.VendorName = vendorID, vendorName

	var parentItems []models.PurchaseRequestItem
	json.Unmarshal(itemsJSON, &parentItems)

	// 2. Pisahkan sub-item yang pindah ke pecahan dari yang tetap di master.
	// Pencocokan memakai kuota per (nama grup, nama item) yang dikonsumsi sekali
	// pakai: dua baris item bernama sama dalam satu grup dulu ikut terbawa
	// semuanya walau pengguna hanya memilih salah satu.
	quota := map[string]int{}
	for _, g := range input.Items {
		for _, s := range g.Items {
			quota[g.Name+"\x00"+s.Name]++
		}
	}

	var groupsToMove []models.PurchaseRequestItem
	groupsRemaining := []models.PurchaseRequestItem{}

	for _, pGroup := range parentItems {
		var subItemsToMove, subItemsRemaining []models.PurchaseSubItem

		for _, pSub := range pGroup.Items {
			key := pGroup.Name + "\x00" + pSub.Name
			if quota[key] > 0 {
				quota[key]--
				subItemsToMove = append(subItemsToMove, pSub)
			} else {
				subItemsRemaining = append(subItemsRemaining, pSub)
			}
		}

		if len(subItemsToMove) > 0 {
			g := pGroup
			g.Items = subItemsToMove
			groupsToMove = append(groupsToMove, g)
		}
		if len(subItemsRemaining) > 0 {
			g := pGroup
			g.Items = subItemsRemaining
			groupsRemaining = append(groupsRemaining, g)
		}
	}

	if len(groupsToMove) == 0 {
		return nil, fmt.Errorf("tidak ada item valid yang dipilih untuk di-split")
	}

	// 3. Buat PR pecahan berisi item yang dipindah.
	childID := NewULID()
	childHps, childFinal := recalcItems(groupsToMove)
	childItemsJSON, err := json.Marshal(groupsToMove)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun item pecahan: %w", err)
	}
	err = insertWithRequestNumber(now, func(reqNumber string) error {
		// SAVEPOINT: nomor kembar membuat INSERT gagal dan — tanpa ini —
		// seluruh transaksi split ikut abort sehingga retry tidak ada gunanya.
		if _, err := tx.Exec("SAVEPOINT split_child"); err != nil {
			return err
		}
		// Pecahan mewarisi dari induknya: pemilik, pengaju, jenis belanja
		// (goods_kind — memecah ke beberapa vendor tidak mengubah di meja mana
		// barangnya diterima; dulu disalin lewat UPDATE di luar transaksi yang
		// tidak pernah melihat pecahan baru, T-30), serta jejak verifikasi dan
		// persetujuan — pecahan berstatus approved tanpa penyetuju membuat
		// jejak audit dan periode akrual GL kosong.
		_, err := tx.Exec(`
			INSERT INTO purchase_requests (id, request_number, parent_id, outlet_id, work_unit_id, request_type,
				requested_by, requested_by_id, vendor_id, vendor_name, status, items, total_amount, total_hps, total_final,
				notes, project_id, goods_kind,
				verified_by, verified_by_id, verified_at, verification_notes,
				approved_by, approved_by_id, approved_at, created_at, updated_at)
			SELECT $1, $2, p.id, p.outlet_id, p.work_unit_id, p.request_type,
				p.requested_by, p.requested_by_id, $3, $4, p.status, $5, $6, $7, $8,
				'', p.project_id, COALESCE(p.goods_kind, ''),
				p.verified_by, p.verified_by_id, p.verified_at, p.verification_notes,
				p.approved_by, p.approved_by_id, p.approved_at, $9, $9
			FROM purchase_requests p WHERE p.id = $10
		`, childID, reqNumber, nilIfEmpty(input.VendorID), input.VendorName, childItemsJSON,
			amountOf(childHps, childFinal), childHps, childFinal, now, parentID)
		if err != nil {
			tx.Exec("ROLLBACK TO SAVEPOINT split_child")
			return err
		}
		_, err = tx.Exec("RELEASE SAVEPOINT split_child")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("gagal membuat pengajuan pecahan: %w", err)
	}

	// 4. Item yang dipindah benar-benar keluar dari master — master hanya
	// menyisakan bagian yang belum diserahkan ke vendor mana pun, dan sisa itu
	// tetap bisa dibayar atas nama master sendiri. Master yang habis ter-split
	// menjadi dokumen kosong bertotal 0, lalu otomatis lenyap dari halaman
	// Pembayaran dan dari semua penjumlahan (lihat fullySplitMasterCond).
	remainingHps, remainingFinal := recalcItems(groupsRemaining)
	remainingJSON, err := json.Marshal(groupsRemaining)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun item induk: %w", err)
	}
	_, err = tx.Exec(`
		UPDATE purchase_requests
		SET split_status='master', items=$1, total_amount=$2, total_hps=$3, total_final=$4, updated_at=$5
		WHERE id=$6
	`, remainingJSON, amountOf(remainingHps, remainingFinal), remainingHps, remainingFinal, now, parentID)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui pengajuan induk: %w", err)
	}

	// Keluarga dokumen berubah: status induk diturunkan ulang saat itu juga.
	// Pecahan yang menghabiskan sisa item induk menjadikannya cangkang, dan
	// statusnya harus mengikuti pecahan lain yang mungkin sudah dibayar — dulu
	// tetap 'approved' sampai ada peristiwa keluarga berikutnya.
	if err := syncFamilyStatusTx(tx, parentID, Actor{}); err != nil {
		return nil, fmt.Errorf("gagal menyelaraskan status induk: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetPurchaseRequest(parentID)
}

func UpdatePurchaseStatus(id string, input models.UpdatePurchaseStatusInput) (*models.PurchaseRequest, error) {
	// Bukti bayar ("REF | /uploads/…") dipasang sebagai tautan di halaman
	// Pembayaran: bagian berkasnya wajib URL unggahan yang sah (T-2).
	if input.Action == "pay" {
		if err := ValidatePaymentProof(input.PaymentProof); err != nil {
			return nil, err
		}
	}

	transitions, ok := validTransitions[input.Action]
	if !ok {
		return nil, fmt.Errorf("aksi '%s' tidak valid", input.Action)
	}

	// Isi permintaan divalidasi di AWAL, sebelum cabang master/pecahan. Dulu
	// alasan tolak hanya diperiksa di jalur dokumen tunggal, sehingga master
	// hasil split bisa ditolak tanpa alasan lewat API (T-72).
	//
	// Alasan penolakan wajib, dan dijaga di SERVER — bukan hanya di layar.
	// Pengaju berhak tahu apa yang harus diperbaiki; penolakan tanpa alasan
	// memaksanya menebak dan mengajukan ulang hal yang sama.
	if input.Action == "reject" && strings.TrimSpace(input.RejectedReason) == "" {
		return nil, Invalid("alasan penolakan wajib diisi")
	}
	var rejections []verifyRejection
	if input.Action == "verify" {
		if strings.TrimSpace(input.Version) == "" {
			return nil, Invalid("version wajib dikirim saat verifikasi — muat ulang pengajuan lalu coba lagi")
		}
		var err error
		if rejections, err = normalizeVerifyRejections(input.RejectedItems); err != nil {
			return nil, err
		}
	}

	// Seluruh transisi — termasuk cascade ke pecahan — berjalan dalam SATU
	// transaksi, dan tiap UPDATE status adalah compare-and-set
	// (WHERE id = $1 AND status = $lama, RowsAffected wajib 1). Dulu setujui
	// dan batal yang bersamaan sama-sama "berhasil", dan cascade yang gagal di
	// tengah meninggalkan status campuran (T-26). Notifikasi baru dikirim
	// setelah commit — tidak ada WA untuk perubahan yang akhirnya batal.
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Kunci induk dulu bila ada, baru dokumennya: urutannya sama dengan
	// cascade (induk → pecahan) supaya dua aksi bersamaan tidak saling kunci.
	var parentID sql.NullString
	if err := tx.QueryRow("SELECT parent_id FROM purchase_requests WHERE id = $1", id).Scan(&parentID); err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	if p := strings.TrimSpace(parentID.String); p != "" {
		if _, err := tx.Exec("SELECT 1 FROM purchase_requests WHERE id = $1 FOR UPDATE", p); err != nil {
			return nil, err
		}
	}
	cur, err := lockPurchaseTx(tx, id)
	if err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	// Izin dipilih handler berdasarkan tahap yang ia baca; bila dokumen sudah
	// pindah tahap sebelum terkunci, keputusan itu tidak berlaku lagi.
	if input.ExpectedStatus != "" && cur.Status != input.ExpectedStatus {
		return nil, Conflict(purchaseChangedMsg)
	}
	if v := strings.TrimSpace(input.Version); v != "" && !versionMatches(v, cur.UpdatedAt) {
		return nil, Conflict(purchaseChangedMsg)
	}
	if input.Action == "verify" && SkipsVerification(cur.RequestType) {
		return nil, Invalid("pengadaan %s tidak melalui verifikasi — pengajuannya langsung menunggu persetujuan", strings.ToLower(RequestTypeLabel(cur.RequestType)))
	}
	if input.Action == "receive" && cur.RequestType == RequestTypeJasa && cur.Status == "received" {
		return GetPurchaseRequest(id)
	}

	newStatus, ok := transitions[cur.Status]
	if !ok && input.Action == "approve" && cur.Status == "pending" && cur.Urgency == UrgencyUrgent {
		newStatus, ok = "approved", true
	}
	if !ok {
		return nil, fmt.Errorf("tidak bisa %s dari status '%s'", input.Action, cur.Status)
	}

	isMaster := cur.SplitStatus == "master"
	if input.Action == "approve" && input.DirectCashPayout && isMaster {
		return nil, Invalid("pengajuan yang sudah dipecah per vendor tidak bisa langsung diarahkan ke Kas Kasir Outlet")
	}
	// Barang & bahan baku diterima lewat Serah Terima (foto + pencatatan
	// aset/stok per baris). Tombol status "receive" hanya untuk jasa — dulu
	// pengajuan barang bisa ditutup 'received' tanpa foto, aset, maupun GRN
	// (T-35).
	if input.Action == "receive" && cur.RequestType != RequestTypeJasa {
		return nil, Invalid("pengajuan %s diterima lewat Serah Terima barang (foto dan pencatatan aset/stok per baris), bukan lewat tombol status",
			strings.ToLower(RequestTypeLabel(cur.RequestType)))
	}
	// Harga final SEMUA item wajib terisi sebelum ditagihkan (T-37, T-15):
	// dulu cukup salah satu item berharga, lalu dokumen 'lunas' dan item tanpa
	// harga ikut tertutup tanpa pernah dibeli. Untuk induk hasil pecah vendor
	// yang diperiksa sisa item miliknya sendiri; pecahan diperiksa di cascade.
	if input.Action == "request_payment" {
		if names := unpricedItemNames(cur.ItemsJSON); len(names) > 0 {
			who := "pengajuan ini"
			if isMaster {
				who = "sisa item induk (yang belum dipecah)"
			}
			return nil, Invalid("harga final %s belum diisi: %s — isi harganya dulu. Item yang tidak jadi dibeli pecah ke dokumen tersendiri lalu batalkan",
				who, strings.Join(trimList(names, 5), ", "))
		}
	}

	actor := Actor{ID: validActorID(input.ActorID), Name: input.ActorName}
	var notices []purchaseNotice
	switch {
	case isMaster && input.Action == "verify":
		// Pecah vendor baru boleh setelah disetujui, jadi induk tidak pernah
		// menunggu verifikasi; data lama semacam ini tidak bisa diverifikasi.
		return nil, Invalid("pengajuan induk hasil pecah vendor tidak bisa diverifikasi")

	case isMaster && input.Action == "pay":
		// Master hanya boleh dibayar sebatas sisa item yang masih dipegang
		// sendiri. Cek jumlah item, bukan total_final: master yang itemnya
		// sudah habis pindah ke pecahan tidak punya tagihan apa pun, dan
		// membayarnya berarti membayar dua kali atas barang yang sama.
		var own []json.RawMessage
		json.Unmarshal(cur.ItemsJSON, &own)
		if len(own) == 0 {
			return nil, fmt.Errorf("seluruh item sudah dipecah ke vendor, bayar per vendor melalui halaman Pembayaran")
		}
		n, err := applyStatusUpdateTx(tx, id, cur.Status, newStatus, input)
		if err != nil {
			return nil, err
		}
		notices = append(notices, n)
		// Status induk = status keluarganya: melunasi sisa milik induk sendiri
		// tidak menjadikannya 'paid' selama pecahannya belum lunas (T-36).
		if err := syncFamilyStatusTx(tx, id, actor); err != nil {
			return nil, err
		}

	case isMaster:
		// request_payment, approve, cancel, reject, receive ikut ke pecahan.
		// reject & receive dulu tidak ikut cascade: master ditolak tapi
		// pecahannya tertinggal 'pending', atau master tercatat diterima
		// sementara pecahannya masih 'paid'.
		//
		// Kunci yang sama berlaku sebelum cascade: master yang salah satu
		// pecahannya sudah menerima barang tidak boleh dibatalkan.
		if input.Action == "cancel" && purchaseHasReceipts(tx, id) {
			return nil, Invalid(receivedLockMsg, "tidak bisa dibatalkan")
		}
		if input.Action == "approve" {
			if err := revalidateProjectRab(cur, "disetujui"); err != nil {
				return nil, err
			}
		}
		moved, err := cascadeStatusToChildrenTx(tx, id, input)
		if err != nil {
			return nil, err
		}
		if input.Action == "request_payment" && !hasOwnPurchaseItems(cur.ItemsJSON) {
			// Induk tanpa sisa item hanya cangkang: yang ditagihkan adalah
			// pecahannya, dan statusnya sendiri mengikuti keluarga (di bawah).
			if moved == 0 {
				return nil, Invalid("tidak ada pecahan yang bisa diajukan pembayarannya — pecahan yang belum berharga, belum memilih vendor terdaftar, atau vendornya belum punya rekening (butuh VA / tunai) diajukan dari detail pecahannya")
			}
		} else if _, err := applyStatusUpdateTx(tx, id, cur.Status, newStatus, input); err != nil {
			return nil, err
		}
		// SATU pesan untuk induk, berisi rincian per vendor — bukan satu pesan
		// per pecahan ditambah pesan induk bernilai gabungan yang terbaca
		// seperti tagihan tambahan (T-62).
		notices = append(notices, purchaseNotice{id: id, action: input.Action, reason: strings.TrimSpace(input.RejectedReason)})
		if err := syncFamilyStatusTx(tx, id, actor); err != nil {
			return nil, err
		}

	default:
		if input.Action == "cancel" && purchaseHasReceipts(tx, id) {
			return nil, Invalid(receivedLockMsg, "tidak bisa dibatalkan. Selesaikan pembayarannya, atau kembalikan barangnya lewat dokumen penghapusan/koreksi stok")
		}
		var n purchaseNotice
		if input.Action == "verify" {
			n, err = applyVerifyTx(tx, cur, input, rejections)
		} else {
			if input.Action == "approve" {
				if err := revalidateProjectRab(cur, "disetujui"); err != nil {
					return nil, err
				}
			}
			n, err = applyStatusUpdateTx(tx, id, cur.Status, newStatus, input)
		}
		if err != nil {
			return nil, err
		}
		notices = append(notices, n)
		// Pecahan berubah (mis. diajukan, dibayar, dibatalkan) → status
		// induknya ikut dihitung ulang dari keluarganya.
		if cur.ParentID != "" {
			if err := syncFamilyStatusTx(tx, cur.ParentID, actor); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for _, n := range notices {
		if pr, err := GetPurchaseRequest(n.id); err == nil {
			go NotifyPurchaseEvent(pr, n.action, input.ActorName, n.reason, n.amount)
		}
	}
	return GetPurchaseRequest(id)
}

// lockedPurchase: potret satu pengajuan yang sudah dikunci (FOR UPDATE).
type lockedPurchase struct {
	ID, Status, SplitStatus, ParentID, RequestType, ProjectID, Urgency string
	UpdatedAt                                                          time.Time
	ItemsJSON, RejectedJSON                                            []byte
}

func lockPurchaseTx(tx *sql.Tx, id string) (*lockedPurchase, error) {
	p := &lockedPurchase{ID: id}
	var updatedAt sql.NullTime
	err := tx.QueryRow(`
		SELECT status, COALESCE(split_status, ''), COALESCE(parent_id, ''), request_type, COALESCE(project_id, ''),
		       COALESCE(urgency, 'rutin'), updated_at, items, COALESCE(rejected_items, '[]'::jsonb)
		FROM purchase_requests WHERE id = $1 FOR UPDATE`, id).
		Scan(&p.Status, &p.SplitStatus, &p.ParentID, &p.RequestType, &p.ProjectID, &p.Urgency, &updatedAt, &p.ItemsJSON, &p.RejectedJSON)
	if err != nil {
		return nil, err
	}
	p.ParentID = strings.TrimSpace(p.ParentID)
	p.ProjectID = strings.TrimSpace(p.ProjectID)
	if updatedAt.Valid {
		p.UpdatedAt = updatedAt.Time
	}
	return p, nil
}

// purchaseNotice: notifikasi yang dikirim setelah transaksi commit.
type purchaseNotice struct {
	id, action, reason string
	amount             float64
}

// revalidateProjectRab memeriksa ulang rujukan RAB pengajuan projek saat
// verifikasi/persetujuan: RAB yang sedang dibuka untuk revisi, atau projek
// yang sudah selesai/batal, tidak boleh menerima belanja baru (T-51).
func revalidateProjectRab(cur *lockedPurchase, stage string) error {
	if cur.ProjectID == "" || cur.RequestType == RequestTypeBahan {
		return nil
	}
	var items []models.PurchaseRequestItem
	json.Unmarshal(cur.ItemsJSON, &items)
	return revalidateProjectRabItems(cur, items, stage)
}

func revalidateProjectRabItems(cur *lockedPurchase, items []models.PurchaseRequestItem, stage string) error {
	if cur.ProjectID == "" || cur.RequestType == RequestTypeBahan {
		return nil
	}
	if err := validateProjectRab(cur.ProjectID, cur.RequestType, items, true); err != nil {
		var ve ValidationError
		if errors.As(err, &ve) {
			return Invalid("pengajuan belum bisa %s: %s", stage, ve.Msg)
		}
		return err
	}
	return nil
}

// cascadeStatusToChildrenTx menjalankan satu aksi pada semua pecahan sebuah
// induk, di dalam transaksi pemanggil (transisi induknya sendiri dijalankan
// pemanggil). Mengembalikan jumlah pecahan yang berpindah status.
//
//   - Pecahan yang sudah dibatalkan/ditolak tidak ikut, dan pecahan yang sudah
//     berada di status tujuan dilewati.
//   - request_payment: hanya pecahan 'approved' yang SELURUH itemnya sudah
//     berharga final dan sudah memilih vendor terdaftar. Pecahan lain
//     diajukan sendiri dari detail pecahannya setelah dilengkapi — tidak
//     tertahan selamanya (T-16).
//   - Aksi lain (batal, tolak, selesai): bila ada pecahan hidup yang tidak bisa
//     ikut, SELURUH aksi ditolak dengan menyebut nomornya. Dulu pecahan itu
//     dilewati diam-diam: induk batal sementara pecahannya yang sudah di
//     keuangan tetap dibayar, lalu menghidupkan induk lagi (T-14).
func cascadeStatusToChildrenTx(tx *sql.Tx, masterID string, input models.UpdatePurchaseStatusInput) (int, error) {
	rows, err := tx.Query(`
		SELECT id, request_number, status, items, COALESCE(vendor_id, '') FROM purchase_requests
		WHERE parent_id = $1 ORDER BY created_at, id FOR UPDATE`, masterID)
	if err != nil {
		return 0, err
	}
	type child struct {
		id, number, status, vendorID string
		items                        []byte
	}
	var children []child
	for rows.Next() {
		var c child
		if err := rows.Scan(&c.id, &c.number, &c.status, &c.items, &c.vendorID); err != nil {
			rows.Close()
			return 0, err
		}
		c.id = strings.TrimSpace(c.id)
		children = append(children, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	transitions := validTransitions[input.Action]
	targets := map[string]bool{}
	for _, to := range transitions {
		targets[to] = true
	}
	type step struct{ id, from, to string }
	var steps []step
	var blockers []string
	for _, c := range children {
		if c.status == "cancelled" || c.status == "rejected" {
			continue // pecahan mati tidak ikut
		}
		to, valid := transitions[c.status]
		if input.Action == "request_payment" {
			// Pecahan yang belum berharga lengkap atau belum memilih vendor
			// terdaftar diajukan sendiri setelah dilengkapi.
			if !valid || len(unpricedItemNames(c.items)) > 0 || strings.TrimSpace(c.vendorID) == "" {
				continue
			}
			// Pecahan ikut memakai rekening terdaftar vendornya. Yang vendornya
			// belum punya rekening (butuh VA marketplace / tunai) diajukan
			// sendiri dari detail pecahannya beserta detail pembayarannya.
			if !vendorHasAccountTx(tx, c.vendorID) {
				continue
			}
		} else if !valid {
			if targets[c.status] {
				continue // sudah di status tujuan
			}
			blockers = append(blockers, fmt.Sprintf("%s (%s)", c.number, purchaseStatusLabel(c.status)))
			continue
		}
		steps = append(steps, step{id: c.id, from: c.status, to: to})
	}
	if len(blockers) > 0 {
		return 0, Invalid("pengajuan induk tidak bisa %s: pecahan %s tidak bisa ikut. Selesaikan atau batalkan pecahan itu lebih dulu",
			purchaseActionVerb(input.Action), strings.Join(blockers, ", "))
	}
	childInput := input
	if input.Action == "request_payment" {
		// Detail pembayaran di dialog milik dokumen induk (vendornya sendiri);
		// pecahan selalu ke rekening terdaftar vendornya masing-masing.
		childInput.PayTo = &models.PurchasePaymentDetailsInput{Method: PayToVendorAccount}
	}
	for _, s := range steps {
		if _, err := applyStatusUpdateTx(tx, s.id, s.from, s.to, childInput); err != nil {
			return 0, fmt.Errorf("gagal update pecahan %s: %w", s.id, err)
		}
	}
	return len(steps), nil
}

// vendorHasAccountTx: vendor punya rekening terdaftar.
func vendorHasAccountTx(tx *sql.Tx, vendorID string) bool {
	var number string
	tx.QueryRow(`SELECT COALESCE(account_number, '') FROM vendors WHERE id = $1`, strings.TrimSpace(vendorID)).Scan(&number)
	return strings.TrimSpace(number) != ""
}

// purchaseActionVerb: bentuk pasif aksi status untuk pesan galat.
func purchaseActionVerb(action string) string {
	switch action {
	case "cancel":
		return "dibatalkan"
	case "reject":
		return "ditolak"
	case "receive":
		return "diselesaikan"
	case "approve":
		return "disetujui"
	case "verify":
		return "diverifikasi"
	case "request_payment":
		return "diajukan pembayarannya"
	case "pay":
		return "dibayar"
	}
	return action
}

// unpricedItemNames: nama sub-item yang harga finalnya belum diisi.
func unpricedItemNames(raw []byte) []string {
	var items []models.PurchaseRequestItem
	json.Unmarshal(raw, &items)
	out := []string{}
	for _, g := range items {
		for _, s := range g.Items {
			if s.FinalPrice <= 0 {
				out = append(out, s.Name)
			}
		}
	}
	return out
}

// hasOwnPurchaseItems: dokumen masih memegang item sendiri (induk yang belum
// seluruhnya dipecah, atau dokumen biasa).
func hasOwnPurchaseItems(raw []byte) bool {
	var own []json.RawMessage
	json.Unmarshal(raw, &own)
	return len(own) > 0
}

// applyStatusUpdateTx menulis satu transisi status sebagai compare-and-set:
// UPDATE … WHERE id = $id AND status = $fromStatus. Aksi yang kalah balapan
// mendapat 0 baris → ConflictError, bukan "berhasil" diam-diam.
func applyStatusUpdateTx(tx *sql.Tx, id, fromStatus, newStatus string, input models.UpdatePurchaseStatusInput) (purchaseNotice, error) {
	now := time.Now().UTC()
	actorID := nilIfEmpty(validActorID(input.ActorID))
	notice := purchaseNotice{id: id, action: input.Action, reason: strings.TrimSpace(input.RejectedReason)}
	var res sql.Result
	var err error

	switch input.Action {
	case "approve":
		if input.DirectCashPayout {
			var outletID, requestedBy, rawItems string
			if scanErr := tx.QueryRow(`SELECT COALESCE(outlet_id,''), COALESCE(requested_by,''), COALESCE(items::text,'[]') FROM purchase_requests WHERE id=$1`, id).
				Scan(&outletID, &requestedBy, &rawItems); scanErr != nil {
				return notice, fmt.Errorf("gagal memeriksa pengajuan: %w", scanErr)
			}
			outletID = strings.TrimSpace(outletID)
			if outletID == "" {
				return notice, Invalid("pengajuan tanpa outlet hanya bisa diproses melalui umum (Purchasing & Kantor Pusat)")
			}
			var outletName string
			var active bool
			if scanErr := tx.QueryRow(`SELECT COALESCE(name,''), is_active FROM outlets WHERE id=$1`, outletID).Scan(&outletName, &active); scanErr != nil || !active {
				return notice, Invalid("outlet pengajuan ini tidak aktif — proses melalui umum")
			}
			var items []models.PurchaseRequestItem
			if uerr := json.Unmarshal([]byte(rawItems), &items); uerr != nil || len(items) == 0 {
				return notice, Invalid("item pengajuan tidak valid")
			}
			for gi := range items {
				for si := range items[gi].Items {
					if items[gi].Items[si].FinalPrice <= 0 {
						items[gi].Items[si].FinalPrice = items[gi].Items[si].HpsPrice
					}
				}
			}
			totalHps, totalFinal := recalcItems(items)
			if cents(totalFinal) <= 0 {
				return notice, Invalid("total HPS harus lebih dari 0 untuk diarahkan ke Kas Kasir Outlet")
			}
			recipient, rerr := limitText(input.PayoutRecipient, 200, "nama penerima kas")
			if rerr != nil {
				return notice, rerr
			}
			if recipient == "" {
				recipient = strings.TrimSpace(requestedBy)
			}
			if recipient == "" {
				return notice, Invalid("nama penerima kas wajib diisi")
			}
			payoutNotes, nerr := limitText(input.PayoutNotes, 1000, "catatan kasir")
			if nerr != nil {
				return notice, nerr
			}
			itemsJSON, merr := json.Marshal(items)
			if merr != nil {
				return notice, merr
			}
			payToText := truncateRunes("Tunai dari Kas Kasir "+outletName+" — diserahkan kepada "+recipient, 300)
			res, err = tx.Exec(`
				UPDATE purchase_requests SET status='payment_requested', items=$1, total_hps=$2, total_final=$3, total_amount=$3,
				       approved_by=$4, approved_by_id=$5, approved_at=$6,
				       pay_to_method='manual', pay_to_text=$7, pay_to_note=$8, pay_to_set_by=$4, pay_to_set_by_id=$5, pay_to_set_at=$6,
				       updated_at=$6
				WHERE id=$9 AND status=$10`,
				itemsJSON, totalHps, totalFinal, input.ActorName, actorID, now, payToText, payoutNotes, id, fromStatus)
			if err == nil {
				if n, _ := res.RowsAffected(); n == 1 {
					_, err = tx.Exec(`INSERT INTO procurement_cash_payouts
						(id, purchase_request_id, outlet_id, amount, recipient_name, notes, status, assigned_by, assigned_by_id, assigned_at, updated_at)
						VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9, $9)`,
						NewULID(), id, outletID, totalFinal, recipient, payoutNotes, input.ActorName, actorID, now)
				}
			}
		} else {
			res, err = tx.Exec(`
				UPDATE purchase_requests SET status=$1, approved_by=$2, approved_by_id=$3, approved_at=$4, updated_at=$4
				WHERE id=$5 AND status=$6`,
				newStatus, input.ActorName, actorID, now, id, fromStatus)
		}
	case "reject":
		// Penolak dicatat di kolomnya sendiri — dulu ditulis ke
		// approved_by/approved_at sehingga penolak terbaca sebagai penyetuju
		// oleh konsumen data yang tidak menyaring status (T-73).
		res, err = tx.Exec(`
			UPDATE purchase_requests SET status=$1, rejected_reason=$2, rejected_by=$3, rejected_by_id=$4, rejected_at=$5, updated_at=$5
			WHERE id=$6 AND status=$7`,
			newStatus, strings.TrimSpace(input.RejectedReason), input.ActorName, actorID, now, id, fromStatus)
	case "request_payment":
		// Hanya tagihan milik dokumen ini sendiri: pecahan diajukan sebagai
		// dokumennya masing-masing (cascade atau dari detail pecahan), dan
		// induk tanpa sisa item tidak pernah diajukan langsung — statusnya
		// mengikuti keluarga (syncFamilyStatusTx). Dulu total pecahan ikut
		// dijumlahkan, sehingga induk bersisa item tanpa harga lolos menjadi
		// 'payment_requested' bernilai Rp0 dan buntu (T-15).
		var totalFinal float64
		if scanErr := tx.QueryRow("SELECT COALESCE(total_final, 0) FROM purchase_requests WHERE id = $1", id).Scan(&totalFinal); scanErr != nil {
			return notice, fmt.Errorf("gagal memeriksa total harga")
		}
		if totalFinal <= 0 {
			return notice, Invalid("harga final belum diisi, tidak bisa mengajukan pembayaran")
		}
		// Pembayaran selalu ke vendor terdaftar: rekening tujuannya dipotret
		// dari data vendor saat dibayar (keputusan owner 14 Okt 2026). Form
		// sudah mewajibkan memilih vendor dari daftar; ini penjaga API.
		if err := requireRegisteredVendorTx(tx, id); err != nil {
			return notice, err
		}
		// Tujuan pembayaran ditetapkan Pembelian di sini (keputusan owner
		// 19 Nov 2026); Keuangan hanya membacanya saat membayar.
		st, perr := payToFromInputTx(tx, id, input.PayTo)
		if perr != nil {
			return notice, perr
		}
		res, err = tx.Exec(`
			UPDATE purchase_requests SET status=$1, updated_at=$2,
			       pay_to_method=$5, pay_to_marketplace=$6, pay_to_order_number=$7,
			       pay_to_bank_name=$8, pay_to_account_number=$9, pay_to_account_holder=$10, pay_to_text=$11, pay_to_note=$12,
			       pay_to_set_by=$13, pay_to_set_by_id=$14, pay_to_set_at=$2,
			       pay_to_returned_at=NULL, pay_to_returned_by='', pay_to_returned_by_id=NULL, pay_to_return_reason=''
			WHERE id=$3 AND status=$4`,
			newStatus, now, id, fromStatus,
			st.method, st.marketplace, st.orderNumber, st.bank, st.number, st.holder, st.text, st.note,
			input.ActorName, actorID)
	case "pay":
		// FOR UPDATE di transaksi yang sama: dua pembayaran bersamaan (double
		// click / retry) tidak boleh sama-sama membaca paid_amount lama — tanpa
		// lock ini histori pembayaran bisa dobel dan status jadi tidak konsisten.
		var totalFinal, currentPaid float64
		var receipt, splitStatus, vendorID, vendorName, payApprovalStatus string
		var emergency, isManualPayment bool
		if scanErr := tx.QueryRow(`
			SELECT COALESCE(total_final,0), COALESCE(paid_amount,0), COALESCE(receipt_status,''), COALESCE(split_status,''),
			       COALESCE(vendor_id, ''), COALESCE(vendor_name, ''), emergency,
			       COALESCE(is_manual_payment, false), COALESCE(payment_approval_status, '')
			FROM purchase_requests WHERE id=$1 FOR UPDATE`, id).Scan(
			&totalFinal, &currentPaid, &receipt, &splitStatus, &vendorID, &vendorName, &emergency,
			&isManualPayment, &payApprovalStatus); scanErr != nil {
			return notice, fmt.Errorf("gagal memeriksa tagihan: %w", scanErr)
		}
		if payApprovalStatus == "pending" {
			return notice, Invalid("pengajuan pembayaran ini masih menunggu persetujuan Manager Keuangan atau General Manager")
		}
		if payApprovalStatus == "rejected" {
			return notice, Invalid("pengajuan pembayaran ini ditolak oleh Manager Keuangan / General Manager — ajukan ulang terlebih dahulu")
		}
		if isManualPayment && payApprovalStatus != "approved" {
			return notice, Invalid("pembayaran manual wajib disetujui Manager Keuangan atau General Manager sebelum dicairkan")
		}
		// Sumber dana tunggal: selama tugas Kas Kasir Outlet masih terbuka,
		// Kantor Pusat tidak boleh membayar pengajuan yang sama — uang yang
		// sama bisa keluar dua kali (dari laci outlet dan dari rekening).
		var openPayoutID, openPayoutStatus string
		switch scanErr := tx.QueryRow(`
			SELECT id, status FROM procurement_cash_payouts
			WHERE purchase_request_id=$1 AND status IN ('pending','claimed')`, id).Scan(&openPayoutID, &openPayoutStatus); {
		case scanErr == sql.ErrNoRows:
			if input.CashPayoutID != "" {
				return notice, Conflict("tugas Kas Kasir Outlet tidak lagi terbuka")
			}
		case scanErr != nil:
			return notice, fmt.Errorf("gagal memeriksa tugas kas outlet: %w", scanErr)
		case input.CashPayoutID == "":
			return notice, Invalid("pengajuan ini sedang menunggu pembayaran dari Kas Kasir Outlet — tarik tugasnya dulu bila ingin dibayar Kantor Pusat")
		case strings.TrimSpace(openPayoutID) != input.CashPayoutID || openPayoutStatus != "claimed":
			return notice, Conflict("tugas Kas Kasir Outlet tidak cocok dengan pengajuan ini")
		}

		// Semua perbandingan dalam sen: selisih pecahan float tidak boleh
		// menolak pelunasan yang tepat sebesar sisa tagihan.
		remainingCents := math.Round((totalFinal - currentPaid) * 100)
		if remainingCents <= 0 {
			return notice, fmt.Errorf("tagihan sudah lunas")
		}
		remaining := remainingCents / 100

		// Struk pembelian pengajuan ini (bila ada): pembayaran menunjuk struk
		// yang dilunasi, atau dibagi ke struk yang belum lunas, terlama dulu.
		struks, strukErr := strukRemainingsTx(tx, id)
		if strukErr != nil {
			return notice, fmt.Errorf("gagal membaca struk: %w", strukErr)
		}

		payAmount := input.PaymentAmount
		if payAmount < 0 {
			// Dulu nilai negatif diperlakukan sebagai "bayar penuh" (T-81).
			return notice, Invalid("jumlah bayar tidak boleh negatif")
		}
		if payAmount == 0 {
			payAmount = remaining // kosong = lunasi sisa tagihan
			if sid := strings.TrimSpace(input.StrukID); sid != "" {
				// Kosong + struk dipilih = lunasi sisa struk itu.
				c, err := strukPayDefaultCents(struks, sid)
				if err != nil {
					return notice, err
				}
				payAmount = math.Min(c, remainingCents) / 100
			}
		}
		payCents := math.Round(payAmount * 100)
		if payCents > remainingCents {
			return notice, fmt.Errorf("jumlah bayar (%.0f) melebihi sisa tagihan (%.0f)", payAmount, remaining)
		}

		payAmount = payCents / 100
		notice.amount = payAmount
		newPaid := math.Round(currentPaid*100+payCents) / 100

		// Snapshot the registered account or this order's explicit marketplace
		// VA. Neither changes to the vendor nor future orders alter this history.
		//
		// Kantor Pusat membayar ke tujuan yang ditetapkan Pembelian
		// (payToDestTx) — isian tujuan di body Keuangan diabaikan. Kas Kasir
		// Outlet: tunainya sudah keluar dari laci saat sampai di sini (tujuan
		// diperiksa ketika tugas ditetapkan), jadi penyelesaiannya tidak boleh
		// gagal karena isian Pembelian berubah sesudahnya.
		var dest paymentDest
		var destErr error
		switch {
		case input.CashPayoutID != "" && strings.TrimSpace(vendorID) == "":
			// Kas Kasir Outlet tanpa vendor terdaftar (mis. diarahkan langsung
			// oleh penyetuju): tunai diserahkan ke staf penerima kas.
			dest, destErr = resolvePaymentDestination(paymentDest{vendorName: vendorName}, input)
		case input.CashPayoutID != "":
			dest, destErr = vendorPaymentDestTx(tx, vendorID, input)
		case emergency:
			destErr = Invalid("Pembelian Darurat hanya dibayar tunai dari Kas Kasir Outlet lewat tablet kasir — kirim ulang tugas kasnya bila tugas sebelumnya dikembalikan")
		default:
			dest, destErr = payToDestTx(tx, id)
		}
		if destErr != nil {
			return notice, destErr
		}

		allocs, allocErr := allocateStrukPayment(struks, input.StrukID, payCents)
		if allocErr != nil {
			return notice, allocErr
		}

		bankFee := math.Round(input.BankFee*100) / 100
		if bankFee < 0 {
			return notice, Invalid("biaya bank tidak boleh negatif")
		}
		if input.CashPayoutID != "" {
			bankFee = 0
		}

		// Insert payment history — pembayar dari sesi login, bukan body (T-7).
		// Satu baris per struk yang dilunasi pembayaran ini; biaya admin bank
		// (bila ada) ditempelkan sekali pada baris pertama transfer ini.
		for i, a := range allocs {
			rowBankFee := 0.0
			if i == 0 {
				rowBankFee = bankFee
			}
			if _, insErr := tx.Exec(
				`INSERT INTO payment_histories (id, purchase_request_id, amount, payment_proof, payment_account_dest, payment_account_source, payment_notes, paid_by, created_at,
					vendor_id, vendor_name, dest_bank_name, dest_account_number, dest_account_holder, dest_manual, struk_id,
					payment_method, payment_marketplace, payment_order_number, cash_payout_id, bank_fee)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
				NewULID(), id, a.cents/100, input.PaymentProof,
				dest.text, input.PaymentAccountSource, input.PaymentNotes,
				input.ActorName, now,
				nilIfEmpty(dest.vendorID), dest.vendorName, dest.bank, dest.number, dest.holder, dest.manual, nilIfEmpty(a.strukID),
				dest.method, dest.marketplace, dest.orderNumber, nilIfEmpty(input.CashPayoutID), rowBankFee,
			); insErr != nil {
				return notice, fmt.Errorf("gagal menyimpan histori pembayaran: %w", insErr)
			}
		}

		// Determine new status
		actualStatus := "partial"
		if math.Round(newPaid*100) >= math.Round(totalFinal*100) {
			actualStatus = "paid"
		}
		switch {
		case fromStatus == "received":
			// Pelunasan sisa setelah barang diterima: status tetap 'received'
			// (T-12).
			actualStatus = "received"
		case splitStatus == "master":
			// Induk: status akhirnya dihitung dari keluarganya
			// (syncFamilyStatusTx), bukan dari penerimaan gabungan.
		case actualStatus == "paid" && receipt == "received":
			// Barang yang sudah diterima lebih dulu (pembelian tempo): begitu
			// lunas, dokumennya langsung tuntas — tidak perlu menekan "Serah
			// Terima" untuk barang yang fisiknya sudah lama ada di outlet.
			actualStatus = "received"
		}

		res, err = tx.Exec(
			`UPDATE purchase_requests SET status=$1, paid_by=$2, paid_at=$3, payment_proof=$4,
			 payment_account_dest=$5, payment_account_source=$6, payment_notes=$7,
			 paid_amount=$8, updated_at=$3 WHERE id=$9 AND status=$10`,
			actualStatus, input.ActorName, now, input.PaymentProof,
			dest.text, input.PaymentAccountSource, input.PaymentNotes,
			newPaid, id, fromStatus,
		)
	case "receive":
		res, err = tx.Exec(
			"UPDATE purchase_requests SET status=$1, received_by=$2, received_at=$3, updated_at=$3 WHERE id=$4 AND status=$5",
			newStatus, input.ActorName, now, id, fromStatus,
		)
	case "cancel":
		res, err = tx.Exec(
			"UPDATE purchase_requests SET status=$1, rejected_reason=$2, updated_at=$3 WHERE id=$4 AND status=$5",
			newStatus, input.RejectedReason, now, id, fromStatus,
		)
	default:
		return notice, fmt.Errorf("aksi '%s' tidak valid", input.Action)
	}

	if err != nil {
		return notice, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return notice, Conflict(purchaseChangedMsg)
	}
	return notice, nil
}

// ── Verifikasi ─────────────────────────────────────────────────────────────

// verifyRejection: satu sub-item yang ditolak verifikator, menurut indeks isi
// tersimpan (Group = indeks di items, Item = indeks sub-item di group itu).
type verifyRejection struct {
	Group, Item int
	Reason      string
}

// normalizeVerifyRejections memeriksa bentuk payload rejected_items: indeks
// tidak negatif, tanpa duplikat, dan alasan per item wajib (≥ 5 huruf).
// Rentang indeks diperiksa terhadap isi tersimpan oleh splitRejectedItems.
func normalizeVerifyRejections(in []models.VerifyRejectedItemInput) ([]verifyRejection, error) {
	out := make([]verifyRejection, 0, len(in))
	seen := make(map[[2]int]bool, len(in))
	for _, r := range in {
		if r.Group < 0 || r.Item < 0 {
			return nil, Invalid("indeks item yang ditolak tidak sah (group %d, item %d)", r.Group, r.Item)
		}
		key := [2]int{r.Group, r.Item}
		if seen[key] {
			return nil, Invalid("item yang ditolak (group %d, item %d) dikirim lebih dari sekali", r.Group, r.Item)
		}
		seen[key] = true
		reason := strings.TrimSpace(r.Reason)
		if utf8.RuneCountInString(reason) < VerifyReasonMinLen {
			return nil, Invalid("alasan menolak item (group %d, item %d) wajib diisi, minimal %d huruf", r.Group, r.Item, VerifyReasonMinLen)
		}
		out = append(out, verifyRejection{Group: r.Group, Item: r.Item, Reason: reason})
	}
	return out, nil
}

// splitRejectedItems memisahkan sub-item yang ditolak dari isi tersimpan.
// Yang ditolak keluar dari items (tidak ikut total, persetujuan, maupun
// pembelian) dan dicatat lengkap dengan alasan & pelakunya; group yang habis
// dibuang. Urutan hasil mengikuti isi tersimpan.
func splitRejectedItems(items []models.PurchaseRequestItem, rej []verifyRejection, actor Actor, at time.Time) ([]models.PurchaseRequestItem, []models.RejectedPurchaseItem, error) {
	reasons := make(map[[2]int]string, len(rej))
	for _, r := range rej {
		if r.Group >= len(items) || r.Item >= len(items[r.Group].Items) {
			return nil, nil, Invalid("item yang ditolak (group %d, item %d) tidak ada pada pengajuan — muat ulang lalu coba lagi", r.Group, r.Item)
		}
		reasons[[2]int{r.Group, r.Item}] = r.Reason
	}
	remaining := make([]models.PurchaseRequestItem, 0, len(items))
	rejected := make([]models.RejectedPurchaseItem, 0, len(rej))
	stamp := at.UTC().Format(time.RFC3339)
	for g, grp := range items {
		kept := make([]models.PurchaseSubItem, 0, len(grp.Items))
		for i, sub := range grp.Items {
			reason, isRejected := reasons[[2]int{g, i}]
			if !isRejected {
				kept = append(kept, sub)
				continue
			}
			rejected = append(rejected, models.RejectedPurchaseItem{
				Group: grp.Name, Name: sub.Name, Qty: sub.Qty, Unit: sub.Unit,
				HpsPrice: sub.HpsPrice, HpsSource: sub.HpsSource, RabItemID: sub.RabItemID,
				Reason: reason, RejectedBy: actor.Name, RejectedByID: actor.ID, RejectedAt: stamp,
				Stage: "verifikasi",
			})
		}
		if len(kept) > 0 {
			g2 := grp
			g2.Items = kept
			remaining = append(remaining, g2)
		}
	}
	return remaining, rejected, nil
}

// applyVerifyTx: pending → verified (sisa item lanjut ke persetujuan), atau
// pending → rejected bila verifikator menolak semua item.
func applyVerifyTx(tx *sql.Tx, cur *lockedPurchase, input models.UpdatePurchaseStatusInput, rej []verifyRejection) (purchaseNotice, error) {
	var items []models.PurchaseRequestItem
	if err := json.Unmarshal(cur.ItemsJSON, &items); err != nil {
		return purchaseNotice{}, fmt.Errorf("gagal membaca item pengajuan: %w", err)
	}
	actor := Actor{ID: validActorID(input.ActorID), Name: input.ActorName}
	now := time.Now().UTC()
	remaining, rejected, err := splitRejectedItems(items, rej, actor, now)
	if err != nil {
		return purchaseNotice{}, err
	}

	var prev []models.RejectedPurchaseItem
	json.Unmarshal(cur.RejectedJSON, &prev)
	all := append(prev, rejected...)
	if all == nil {
		all = []models.RejectedPurchaseItem{}
	}
	rejectedJSON, err := json.Marshal(all)
	if err != nil {
		return purchaseNotice{}, err
	}
	remainingJSON, err := json.Marshal(remaining)
	if err != nil {
		return purchaseNotice{}, err
	}
	notes := strings.TrimSpace(input.Notes)

	var res sql.Result
	notice := purchaseNotice{id: cur.ID, action: "verify"}
	if len(remaining) == 0 {
		// Semua item ditolak (mis. semuanya masih ada di outlet lain): seluruh
		// pengajuan berakhir ditolak, oleh verifikator.
		notice.action, notice.reason = "reject", allItemsRejectedReason
		res, err = tx.Exec(`
			UPDATE purchase_requests SET status='rejected', items=$1, rejected_items=$2,
				total_amount=0, total_hps=0, total_final=0,
				rejected_reason=$3, rejected_by=$4, rejected_by_id=$5, rejected_at=$6,
				verification_notes=$7, updated_at=$6
			WHERE id=$8 AND status='pending'`,
			remainingJSON, rejectedJSON, allItemsRejectedReason, actor.Name, nilIfEmpty(actor.ID), now, notes, cur.ID)
	} else {
		if err := revalidateProjectRabItems(cur, remaining, "diverifikasi"); err != nil {
			return notice, err
		}
		totalHps, totalFinal := recalcItems(remaining)
		// recalcItems mengisi subtotal di tempat: serialisasi ulang.
		if remainingJSON, err = json.Marshal(remaining); err != nil {
			return notice, err
		}
		res, err = tx.Exec(`
			UPDATE purchase_requests SET status='verified', items=$1, rejected_items=$2,
				total_amount=$3, total_hps=$4, total_final=$5,
				verified_by=$6, verified_by_id=$7, verified_at=$8, verification_notes=$9, updated_at=$8
			WHERE id=$10 AND status='pending'`,
			remainingJSON, rejectedJSON, amountOf(totalHps, totalFinal), totalHps, totalFinal,
			actor.Name, nilIfEmpty(actor.ID), now, notes, cur.ID)
	}
	if err != nil {
		return notice, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return notice, Conflict(purchaseChangedMsg)
	}
	return notice, nil
}

// ── Status keluarga pecah vendor ───────────────────────────────────────────

// familyMember: potret satu anggota keluarga dokumen hasil pecah vendor (satu
// pecahan, atau sisa item milik induk sendiri).
type familyMember struct {
	Status        string
	TotalFinal    float64
	PaidAmount    float64
	ReceiptStatus string
}

// settled: tagihan anggota ini sudah lunas (dinilai dari nominal, dalam sen).
func (m familyMember) settled() bool {
	return m.TotalFinal > 0 && math.Round(m.PaidAmount*100) >= math.Round(m.TotalFinal*100)
}

// paymentStageStatuses: status yang sudah sampai ke keuangan.
var paymentStageStatuses = []string{"payment_requested", "partial", "paid", "received"}

// deriveMasterStatus menurunkan status induk dari keluarganya (T-36): sisa item
// milik induk sendiri (own; nil bila induk tinggal cangkang) ditambah semua
// pecahan yang masih hidup — pecahan dibatalkan/ditolak tidak dihitung. Dulu
// pecahan batal ikut dihitung sehingga induk macet 'partial', induk menjadi
// 'paid' setelah melunasi bagiannya sendiri padahal pecahannya belum dibayar,
// dan induk tertahan 'paid' setelah semua pecahan diterima.
//
//   - Induk ditolak/dibatalkan/belum disetujui tidak pernah diubah — pembayaran
//     pecahan tidak menghidupkan induk yang batal (T-14). 'received' juga
//     tetap.
//   - Induk yang masih memegang item dan bagian itu belum diajukan tetap
//     'approved': isinya masih dikerjakan pembelian (harga, pecah, ajukan).
//   - Selebihnya: semua diterima → received; semua lunas → paid; ada yang
//     sudah dibayar → partial; semua sudah di keuangan (atau bagian induk
//     sendiri sudah diajukan) → payment_requested; selain itu approved.
func deriveMasterStatus(cur string, own *familyMember, children []familyMember) string {
	if !statusIn(cur, prCommittedStatuses) || cur == "received" {
		return cur
	}
	live := make([]familyMember, 0, len(children))
	for _, c := range children {
		if c.Status != "cancelled" && c.Status != "rejected" {
			live = append(live, c)
		}
	}
	if own == nil && len(live) == 0 {
		return cur // cangkang tanpa pecahan hidup: biarkan (bisa dibatalkan atau dipulihkan)
	}
	if own != nil && cur == "approved" {
		return "approved"
	}
	allReceived, allSettled, anyPaid, allRequested := true, true, false, true
	for _, c := range live {
		if c.Status != "received" {
			allReceived = false
		}
		if !c.settled() {
			allSettled = false
		}
		if c.PaidAmount > 0 {
			anyPaid = true
		}
		if !statusIn(c.Status, paymentStageStatuses) {
			allRequested = false
		}
	}
	if own != nil {
		// Bagian induk sendiri sudah diajukan (cur bukan approved). "Diterima"
		// mengikuti aturan dokumen biasa: barang lengkap dan uang sudah mulai
		// keluar.
		if !(own.ReceiptStatus == "received" && own.PaidAmount > 0) {
			allReceived = false
		}
		if !own.settled() {
			allSettled = false
		}
		if own.PaidAmount > 0 {
			anyPaid = true
		}
	}
	switch {
	case allReceived:
		return "received"
	case allSettled:
		return "paid"
	case anyPaid:
		return "partial"
	case allRequested || own != nil:
		return "payment_requested"
	}
	return "approved"
}

// aggregateReceiptStatus: status penerimaan induk = gabungan sisa item miliknya
// dan semua pecahan yang masih hidup.
func aggregateReceiptStatus(own *familyMember, children []familyMember) string {
	states := []string{}
	if own != nil {
		states = append(states, own.ReceiptStatus)
	}
	for _, c := range children {
		if c.Status != "cancelled" && c.Status != "rejected" {
			states = append(states, c.ReceiptStatus)
		}
	}
	if len(states) == 0 {
		return ""
	}
	all, any := true, false
	for _, s := range states {
		if s != "received" {
			all = false
		}
		if s != "" {
			any = true
		}
	}
	switch {
	case all:
		return "received"
	case any:
		return "partial"
	}
	return ""
}

// syncFamilyStatusTx menghitung ulang status dan status penerimaan induk dari
// keluarganya (lihat deriveMasterStatus), di transaksi pemanggil yang sudah
// mengunci induknya. Dipanggil setelah pecahan berubah, setelah aksi pada
// induk, dan setelah serah terima.
func syncFamilyStatusTx(tx *sql.Tx, masterID string, actor Actor) error {
	var status, requestType, receipt string
	var itemsJSON []byte
	var total, paid float64
	err := tx.QueryRow(`
		SELECT status, request_type, items, COALESCE(total_final, 0), COALESCE(paid_amount, 0), COALESCE(receipt_status, '')
		FROM purchase_requests WHERE id = $1 AND split_status = 'master'`, masterID).
		Scan(&status, &requestType, &itemsJSON, &total, &paid, &receipt)
	if err == sql.ErrNoRows {
		return nil // bukan induk (lagi)
	}
	if err != nil {
		return err
	}
	if !statusIn(status, prCommittedStatuses) {
		return nil // ditolak / dibatalkan / belum disetujui: tidak disentuh (T-14)
	}
	rows, err := tx.Query(`
		SELECT status, COALESCE(total_final, 0), COALESCE(paid_amount, 0), COALESCE(receipt_status, '')
		FROM purchase_requests WHERE parent_id = $1`, masterID)
	if err != nil {
		return err
	}
	children := []familyMember{}
	for rows.Next() {
		var m familyMember
		if err := rows.Scan(&m.Status, &m.TotalFinal, &m.PaidAmount, &m.ReceiptStatus); err != nil {
			rows.Close()
			return err
		}
		children = append(children, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	physical := requestType != RequestTypeJasa
	var own *familyMember
	items, err := parsePRItems(itemsJSON)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		own = &familyMember{Status: status, TotalFinal: total, PaidAmount: paid}
		if physical {
			if own.ReceiptStatus, err = docReceiptState(tx, masterID, items); err != nil {
				return err
			}
		}
	}
	newStatus := deriveMasterStatus(status, own, children)
	newReceipt := receipt
	if physical {
		newReceipt = aggregateReceiptStatus(own, children)
	}
	if newStatus == status && newReceipt == receipt {
		return nil
	}
	now := time.Now().UTC()
	if newReceipt == "received" && receipt != "received" {
		_, err = tx.Exec(`UPDATE purchase_requests SET status = $2, receipt_status = 'received',
			received_by = $3, received_at = $4, updated_at = $4 WHERE id = $1 AND status = $5`,
			masterID, newStatus, actor.Name, now, status)
	} else {
		_, err = tx.Exec(`UPDATE purchase_requests SET status = $2, receipt_status = $3, updated_at = $4
			WHERE id = $1 AND status = $5`, masterID, newStatus, newReceipt, now, status)
	}
	return err
}

// approvedLockMsg: isi pengajuan yang sudah disetujui terkunci (T-6).
const approvedLockMsg = "pengajuan sudah disetujui — isinya terkunci; pembelian hanya boleh mengisi jumlah beli aktual, harga final per baris, vendor, dan nomor invoice"

// applyApprovedPriceEdit membandingkan kiriman dengan isi tersimpan pengajuan
// berstatus approved. Yang boleh berubah hanya final_price dan purchased_qty;
// pengadaan, nama item, qty, satuan, HPS, sumber HPS, rujukan baris RAB,
// serta jumlah baris (tambah/hapus) terkunci. Hasilnya = isi tersimpan dengan
// harga final baru, sehingga field lain dari klien tidak pernah ikut tersimpan.
func applyApprovedPriceEdit(stored, incoming []models.PurchaseRequestItem) ([]models.PurchaseRequestItem, error) {
	if len(incoming) != len(stored) {
		return nil, Invalid("%s (jumlah pengadaan berubah dari %d menjadi %d)", approvedLockMsg, len(stored), len(incoming))
	}
	out := make([]models.PurchaseRequestItem, len(stored))
	for g := range stored {
		sg, ig := stored[g], incoming[g]
		if strings.TrimSpace(ig.Name) != strings.TrimSpace(sg.Name) {
			return nil, Invalid("%s (nama pengadaan %q tidak boleh diubah)", approvedLockMsg, sg.Name)
		}
		if len(ig.Items) != len(sg.Items) {
			return nil, Invalid("%s (baris item pada %q tidak boleh ditambah atau dihapus)", approvedLockMsg, sg.Name)
		}
		og := sg
		og.Items = make([]models.PurchaseSubItem, len(sg.Items))
		for i := range sg.Items {
			s, in := sg.Items[i], ig.Items[i]
			field := ""
			switch {
			case strings.TrimSpace(in.Name) != strings.TrimSpace(s.Name):
				field = "nama item"
			case in.Qty != s.Qty:
				field = "qty"
			case strings.TrimSpace(in.Unit) != strings.TrimSpace(s.Unit):
				field = "satuan"
			case math.Abs(in.HpsPrice-s.HpsPrice) > 0.005:
				field = "HPS"
			case strings.TrimSpace(in.HpsSource) != strings.TrimSpace(s.HpsSource):
				field = "sumber HPS"
			case strings.TrimSpace(in.RabItemID) != strings.TrimSpace(s.RabItemID):
				field = "baris RAB"
			}
			if field != "" {
				return nil, Invalid("%s (%s pada item %q tidak boleh diubah)", approvedLockMsg, field, s.Name)
			}
			if in.PurchasedQty != nil {
				qty := *in.PurchasedQty
				if qty <= 0 || qty > s.Qty {
					return nil, Invalid("jumlah beli aktual item %q harus antara 1 dan jumlah pengajuan (%d)", s.Name, s.Qty)
				}
				s.PurchasedQty = &qty
			}
			s.FinalPrice = in.FinalPrice
			og.Items[i] = s
		}
		out[g] = og
	}
	return out, nil
}

func UpdatePurchaseItems(id string, input models.UpdatePurchaseItemsInput) (*models.PurchaseRequest, error) {
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}

	// Dikunci sampai selesai: status yang memilih aturan di bawah (dan izin di
	// handler) tidak boleh berubah di tengah jalan — dulu qty yang disimpan
	// tepat saat owner menyetujui ikut lolos setelah approve (T-26).
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cur, err := lockPurchaseTx(tx, id)
	if err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	if input.ExpectedStatus != "" && cur.Status != input.ExpectedStatus {
		return nil, Conflict(purchaseChangedMsg)
	}
	if v := strings.TrimSpace(input.Version); v != "" && !versionMatches(v, cur.UpdatedAt) {
		return nil, Conflict(purchaseChangedMsg)
	}
	switch cur.Status {
	case "pending", "approved":
	case "verified":
		// Bahan baku dan jasa tidak melalui verifikasi, jadi 'verified' baginya
		// berarti "baru diajukan, menunggu persetujuan": pengaju masih boleh mengubah.
		if !SkipsVerification(cur.RequestType) {
			return nil, Invalid("pengajuan sudah diverifikasi dan menunggu persetujuan — isinya terkunci. Batalkan lalu ajukan ulang bila perlu diubah")
		}
	default:
		return nil, fmt.Errorf("hanya bisa update item pada status pending/approved")
	}
	// Pembelian tempo (T-13): barang boleh sudah diterima sebelum harga final
	// ada. Selama pending penerimaan tidak mungkin terjadi; pada status
	// approved struktur dokumen (nama, qty, satuan, jumlah baris) tetap
	// terkunci lewat applyApprovedPriceEdit — kunci baris penerimaan tidak
	// berubah — tetapi harga final, vendor, dan invoice tetap bisa diisi
	// supaya tagihannya bisa diajukan. Dulu dokumen semacam ini buntu: harga
	// tidak bisa diisi dan ajukan bayar ditolak karena total_final 0.
	hasReceipts := purchaseHasReceipts(tx, id)
	if hasReceipts && cur.Status != "approved" {
		return nil, Invalid(receivedLockMsg, "itemnya tidak bisa diubah lagi")
	}
	// wasPriced: dokumen sudah berharga final sebelum perubahan ini (untuk
	// notifikasi "harga & vendor ditetapkan" yang hanya dikirim sekali).
	var wasPriced bool
	if err := tx.QueryRow(`SELECT COALESCE(total_final, 0) > 0 FROM purchase_requests WHERE id = $1`, id).Scan(&wasPriced); err != nil {
		return nil, err
	}

	items := input.Items
	var stored []models.PurchaseRequestItem
	goodsKind := "" // kosong = goods_kind tidak diubah
	if cur.Status == "approved" {
		// Isi yang sudah diverifikasi & disetujui owner terkunci (T-6). Jenis
		// belanja juga tidak diklasifikasi ulang: katalog yang berubah sesudah
		// persetujuan tidak boleh memindahkan meja penerimaannya (T-19).
		if err := json.Unmarshal(cur.ItemsJSON, &stored); err != nil {
			return nil, fmt.Errorf("gagal membaca item pengajuan: %w", err)
		}
		merged, err := applyApprovedPriceEdit(stored, input.Items)
		if err != nil {
			return nil, err
		}
		items = merged
		if purchaseQuantitiesChanged(stored, items) {
			if err := ensurePurchaseQuantitiesEditable(tx, id, hasReceipts); err != nil {
				return nil, err
			}
		}
	} else {
		// Pending (atau bahan baku yang menunggu persetujuan): pengaju bebas
		// mengubah isi, tetapi harga final tetap urusan pembelian setelah
		// disetujui — kiriman pengaju diabaikan (T-39).
		items = clonePurchaseItems(items)
		clearFinalPrices(items)
		if err := validateReceivableItemText(items, cur.RequestType); err != nil {
			return nil, err
		}
		// Bahan baku selalu tanpa projek: validasi tetap dijalankan dengan projek
		// kosong supaya rujukan baris RAB ditolak, sama seperti saat dibuat.
		rabProjectID := cur.ProjectID
		if cur.RequestType == RequestTypeBahan {
			rabProjectID = ""
		}
		if err := validateProjectRab(rabProjectID, cur.RequestType, items, false); err != nil {
			return nil, err
		}
		// Selama pending jenis tiap baris dibekukan ulang dari katalog
		// (bahan baku tetap dapur, jasa tanpa jenis) — T-19.
		k, kerr := stampPurchaseItemKinds(items, cur.RequestType)
		if kerr != nil {
			return nil, kerr
		}
		goodsKind = k
	}

	totalHps, totalFinal := recalcItems(items)

	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// COALESCE dengan nilai lama: kolom yang tidak dikirim pemanggil tidak
	// tersentuh. Vendor id dan nama diperlakukan satu paket supaya tidak pernah
	// tersisa id tanpa nama atau sebaliknya.
	var vendorID, vendorName, invoice, invoiceURL interface{}
	if input.VendorID != nil || input.VendorName != nil {
		if input.VendorID != nil {
			vendorID = nilIfEmpty(strings.TrimSpace(*input.VendorID))
		}
		if input.VendorName != nil {
			vendorName = *input.VendorName
		} else {
			vendorName = ""
		}
		// Vendor terpilih dari daftar: nama diambil dari tabel vendors dan
		// vendornya wajib aktif (T-67).
		if input.VendorID != nil && strings.TrimSpace(*input.VendorID) != "" {
			_, name, verr := resolvePurchaseVendor(tx, *input.VendorID, "")
			if verr != nil {
				return nil, verr
			}
			vendorName = name
		}
	}
	// Nomor & berkas invoice hanya diisi pembelian pada dokumen yang sudah disetujui;
	// kiriman pengaju saat pending diabaikan (T-39).
	if input.InvoiceNumber != nil && cur.Status == "approved" {
		invoice = *input.InvoiceNumber
	}
	if input.InvoiceURL != nil && cur.Status == "approved" {
		u := strings.TrimSpace(*input.InvoiceURL)
		if u != "" && ValidateUploadURL(u) != nil {
			return nil, Invalid("tautan berkas invoice tidak valid")
		}
		invoiceURL = u
	}

	res, err := tx.Exec(`
		UPDATE purchase_requests SET
			items=$1, total_amount=$2, total_hps=$3, total_final=$4,
			vendor_id   = CASE WHEN $9::bool THEN $5::char(26) ELSE vendor_id END,
			vendor_name = CASE WHEN $9::bool THEN COALESCE($6::varchar, '') ELSE vendor_name END,
			invoice_number = COALESCE($7::text, invoice_number),
			invoice_url    = COALESCE($13::text, invoice_url),
			goods_kind  = CASE WHEN $10::text <> '' THEN $10::text ELSE goods_kind END,
			updated_at=$11
		WHERE id=$8 AND status=$12
	`, itemsJSON, amountOf(totalHps, totalFinal), totalHps, totalFinal,
		vendorID, vendorName, invoice, id,
		input.VendorID != nil || input.VendorName != nil,
		goodsKind, time.Now().UTC(), cur.Status, invoiceURL)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, Conflict(purchaseChangedMsg)
	}
	// Barang tempo yang sudah diterima dengan nilai HPS: nilai aset/material
	// mengikuti harga final yang baru diisi (T-13).
	if cur.Status == "approved" && hasReceipts {
		if err := syncReceivedLinePricesTx(tx, id, stored, items); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	pr, err := GetPurchaseRequest(id)
	// Pengaju dikabari begitu harga final & vendornya ditetapkan pembelian
	// (sekali: saat dokumen yang tadinya tanpa harga menjadi berharga).
	if err == nil && cur.Status == "approved" && !wasPriced && totalFinal > 0 {
		go NotifyPurchaseEvent(pr, "priced", input.ActorName, "", 0)
	}
	return pr, err
}

func DeletePurchaseRequest(id string, isAdmin bool) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Induk dikunci lebih dulu, baru dokumennya — urutan yang sama dengan alur
	// status dan serah terima (induk → pecahan). Dulu pecahan dikunci dulu
	// lalu induknya, sehingga bisa saling tunggu dengan serah terima.
	var parentRef sql.NullString
	if err := tx.QueryRow("SELECT parent_id FROM purchase_requests WHERE id = $1", id).Scan(&parentRef); err != nil {
		return fmt.Errorf("pengajuan tidak ditemukan")
	}
	parentID := strings.TrimSpace(parentRef.String)
	parentStatus, parentNumber := "", ""
	if parentID != "" {
		perr := tx.QueryRow("SELECT status, request_number FROM purchase_requests WHERE id = $1 FOR UPDATE", parentID).
			Scan(&parentStatus, &parentNumber)
		if perr != nil && perr != sql.ErrNoRows {
			return perr
		}
	}

	var status string
	var splitStatus *string
	var paidAmount float64
	var itemsJSON []byte
	err = tx.QueryRow(
		"SELECT status, split_status, COALESCE(paid_amount,0), items FROM purchase_requests WHERE id = $1 FOR UPDATE", id,
	).Scan(&status, &splitStatus, &paidAmount, &itemsJSON)
	if err != nil {
		return fmt.Errorf("pengajuan tidak ditemukan")
	}
	// Berlaku untuk admin maupun bukan: dokumen yang sudah melahirkan aset,
	// GRN, atau material projek tidak boleh hilang — tidak ada foreign key yang
	// menahannya, jadi kuncinya ada di sini.
	if purchaseHasReceipts(tx, id) {
		return Invalid(receivedLockMsg, "tidak bisa dihapus")
	}
	isMaster := splitStatus != nil && *splitStatus == "master"

	if isAdmin {
		// Patokannya uang yang sudah keluar, bukan nama status. Cek lama hanya
		// menahan status 'paid', sehingga pengajuan 'partial' — yang sudah ada
		// pembayarannya — bisa terhapus berikut histori pembayarannya (FK
		// payment_histories ON DELETE CASCADE).
		if paidAmount > 0 || status == "paid" || status == "received" {
			return fmt.Errorf("tidak bisa menghapus pengajuan yang sudah ada pembayarannya")
		}
	} else {
		if status != "pending" && status != "rejected" && status != "cancelled" {
			return fmt.Errorf("hanya bisa menghapus pengajuan berstatus pending/rejected/cancelled")
		}
		if paidAmount > 0 {
			return fmt.Errorf("tidak bisa menghapus pengajuan yang sudah ada pembayarannya")
		}
	}

	ids := []string{id}
	if isMaster {
		// Induk dan pecahannya dihapus BERSAMA, dengan aturan yang sama untuk
		// semua pemanggil: pecahan yang tidak boleh dihapus pemanggil ini (sudah
		// dibayar, atau — bagi non-admin — masih berjalan) menahan penghapusan
		// induknya. Dulu jalur non-admin meninggalkan pecahan yatim yang masih
		// bisa disetujui dan dibayar (T-32).
		rows, err := tx.Query(`
			SELECT id, request_number, status, COALESCE(paid_amount, 0) FROM purchase_requests
			WHERE parent_id = $1 ORDER BY created_at, id FOR UPDATE`, id)
		if err != nil {
			return fmt.Errorf("gagal memeriksa status pecahan")
		}
		blockers := []string{}
		for rows.Next() {
			var cid, cnum, cstatus string
			var cpaid float64
			if err := rows.Scan(&cid, &cnum, &cstatus, &cpaid); err != nil {
				rows.Close()
				return err
			}
			blocked := cpaid > 0 || cstatus == "paid" || cstatus == "received"
			if !isAdmin && cstatus != "pending" && cstatus != "rejected" && cstatus != "cancelled" {
				blocked = true
			}
			if blocked {
				blockers = append(blockers, fmt.Sprintf("%s (%s)", cnum, purchaseStatusLabel(cstatus)))
			}
			ids = append(ids, strings.TrimSpace(cid))
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(blockers) > 0 {
			return Invalid("pengajuan induk tidak bisa dihapus karena pecahan %s masih berjalan atau sudah ada pembayarannya — batalkan atau selesaikan pecahan itu lebih dulu",
				strings.Join(blockers, ", "))
		}
	}

	// Menghapus pecahan mengembalikan itemnya ke induk. Sejak split benar-benar
	// memindahkan item, tanpa langkah ini item tersebut akan hilang selamanya.
	// Hanya selama induknya masih bisa memproses item itu lagi: dulu item
	// kembali ke induk yang sudah lunas, total induk naik, dan sisanya tidak
	// pernah bisa dibayar (T-34). Induk yang sudah mati boleh menerima item
	// kembali (tidak diproses lagi).
	if parentID != "" && parentStatus != "" {
		switch parentStatus {
		case "approved", "cancelled", "rejected":
			if err := returnItemsToParent(tx, parentID, itemsJSON); err != nil {
				return err
			}
		default:
			return Invalid("pecahan tidak bisa dihapus: induknya (%s) sudah %s — item yang dikembalikan tidak akan pernah bisa dibeli atau dibayar. Biarkan pecahan ini berstatus dibatalkan",
				parentNumber, strings.ToLower(purchaseStatusLabel(parentStatus)))
		}
	}

	// Tautan work order dan baris MRP ke pengajuan yang dihapus dilepas, supaya
	// sumbernya bisa diajukan ulang (T-57).
	if _, err := tx.Exec(`UPDATE asset_maintenances SET purchase_request_id = NULL
		WHERE TRIM(purchase_request_id) = ANY($1::text[])`, pq.Array(ids)); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE mrp_run_items SET action_ref_type = '', action_ref_id = NULL, action_ref_number = ''
		WHERE action_ref_type = 'purchase_request' AND TRIM(action_ref_id) = ANY($1::text[])`, pq.Array(ids)); err != nil {
		return err
	}

	if isMaster {
		if _, err := tx.Exec("DELETE FROM purchase_requests WHERE parent_id = $1", id); err != nil {
			return fmt.Errorf("gagal menghapus pecahan: %w", err)
		}
	}
	if _, err := tx.Exec("DELETE FROM purchase_requests WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit()
}

// returnItemsToParent menggabungkan kembali item milik pecahan ke induknya dan
// menghitung ulang total induk. Bila setelah ini induk tidak punya pecahan lagi,
// penanda 'master' dilepas supaya kembali menjadi pengajuan biasa.
func returnItemsToParent(tx *sql.Tx, parentID string, childItemsJSON []byte) error {
	var parentJSON []byte
	if err := tx.QueryRow("SELECT items FROM purchase_requests WHERE id = $1 FOR UPDATE", parentID).Scan(&parentJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil // induk sudah tidak ada, tidak ada yang perlu dikembalikan
		}
		return fmt.Errorf("gagal memuat pengajuan induk: %w", err)
	}

	var parentItems, childItems []models.PurchaseRequestItem
	json.Unmarshal(parentJSON, &parentItems)
	json.Unmarshal(childItemsJSON, &childItems)

	// Item dikembalikan ke grup bernama sama bila ada, agar dokumen induk pulih
	// seperti sebelum di-split alih-alih memunculkan grup kembar.
	for _, cg := range childItems {
		merged := false
		for i := range parentItems {
			if parentItems[i].Name == cg.Name {
				parentItems[i].Items = append(parentItems[i].Items, cg.Items...)
				merged = true
				break
			}
		}
		if !merged {
			parentItems = append(parentItems, cg)
		}
	}
	if parentItems == nil {
		parentItems = []models.PurchaseRequestItem{}
	}

	totalHps, totalFinal := recalcItems(parentItems)
	mergedJSON, err := json.Marshal(parentItems)
	if err != nil {
		return fmt.Errorf("gagal menyusun item induk: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE purchase_requests SET
			items=$1, total_amount=$2, total_hps=$3, total_final=$4,
			split_status = CASE
				WHEN (SELECT COUNT(*) FROM purchase_requests c WHERE c.parent_id = $5) <= 1
				THEN NULL ELSE split_status END,
			updated_at=$6
		WHERE id=$5
	`, mergedJSON, amountOf(totalHps, totalFinal), totalHps, totalFinal, parentID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("gagal mengembalikan item ke induk: %w", err)
	}
	return nil
}

// GetPaymentHistories returns all payment history entries for a purchase request.
func GetPaymentHistories(purchaseRequestID string) ([]models.PaymentHistory, error) {
	rows, err := database.DB.Query(`
		SELECT ph.id, ph.purchase_request_id, ph.amount, COALESCE(ph.bank_fee, 0), ph.payment_proof,
		       ph.payment_account_dest, ph.payment_account_source, ph.payment_notes,
		       ph.paid_by, ph.created_at,
		       COALESCE(ph.vendor_id, ''), ph.vendor_name, ph.dest_bank_name, ph.dest_account_number, ph.dest_account_holder, ph.dest_manual,
		       COALESCE(ph.struk_id, ''), COALESCE(s.struk_number, ''),
		       ph.payment_method, ph.payment_marketplace, ph.payment_order_number
		FROM payment_histories ph
		LEFT JOIN purchase_struks s ON s.id = ph.struk_id
		WHERE ph.purchase_request_id = $1
		ORDER BY ph.created_at ASC, ph.id ASC
	`, purchaseRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := make([]models.PaymentHistory, 0)
	for rows.Next() {
		var h models.PaymentHistory
		var createdAt time.Time
		if err := rows.Scan(&h.ID, &h.PurchaseRequestID, &h.Amount, &h.BankFee, &h.PaymentProof,
			&h.PaymentAccountDest, &h.PaymentAccountSource, &h.PaymentNotes,
			&h.PaidBy, &createdAt,
			&h.VendorID, &h.VendorName, &h.DestBankName, &h.DestAccountNumber, &h.DestAccountHolder, &h.DestManual,
			&h.StrukID, &h.StrukNumber, &h.PaymentMethod, &h.PaymentMarketplace, &h.PaymentOrderNumber); err != nil {
			return nil, err
		}
		h.VendorID = strings.TrimSpace(h.VendorID)
		h.StrukID = strings.TrimSpace(h.StrukID)
		h.CreatedAt = createdAt.Format(time.RFC3339)
		histories = append(histories, h)
	}
	return histories, nil
}

// ── Pembayaran Manual (Di Luar Pengajuan) & Pengajuan / Persetujuan Pembayaran ──

func generatePrefixedNumber(prefix string) (string, error) {
	var seq int
	err := database.DB.QueryRow(`
		INSERT INTO purchase_request_counters AS c (prefix, last_seq)
		VALUES ($1, 1)
		ON CONFLICT (prefix) DO UPDATE
			SET last_seq = c.last_seq + 1
		RETURNING last_seq`, prefix).Scan(&seq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

// CreateManualPayment membuat tagihan pembayaran manual langsung dari Keuangan
// (di luar pengajuan pengadaan normal) yang wajib disetujui Manager Keuangan
// atau General Manager sebelum bisa dicairkan.
func CreateManualPayment(input models.CreateManualPaymentInput) (*models.PurchaseRequest, error) {
	reqType := strings.ToLower(strings.TrimSpace(input.RequestType))
	if reqType == "" {
		reqType = RequestTypeJasa
	}
	if !IsValidRequestType(reqType) {
		return nil, Invalid("jenis pembayaran harus 'barang', 'jasa', atau 'bahan'")
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, Invalid("judul / nama pembayaran wajib diisi")
	}
	if utf8.RuneCountInString(title) > 200 {
		return nil, Invalid("judul pembayaran maksimal 200 karakter")
	}
	if len(input.Items) == 0 {
		return nil, Invalid("minimal 1 rincian item pembayaran wajib diisi")
	}

	goodsKind := strings.ToLower(strings.TrimSpace(input.GoodsKind))
	switch reqType {
	case RequestTypeBarang:
		if goodsKind != "dapur" && goodsKind != "perlengkapan" {
			goodsKind = "perlengkapan"
		}
	case RequestTypeBahan:
		goodsKind = "dapur"
	default:
		goodsKind = ""
	}

	subItems := make([]models.PurchaseSubItem, 0, len(input.Items))
	for _, it := range input.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			return nil, Invalid("nama rincian item tidak boleh kosong")
		}
		qty := it.Qty
		if qty <= 0 {
			qty = 1
		}
		unit := strings.TrimSpace(it.Unit)
		if unit == "" {
			unit = "paket"
		}
		if it.Price <= 0 {
			return nil, Invalid("nominal / harga satuan item %q harus lebih dari 0", name)
		}
		pQty := qty
		subItems = append(subItems, models.PurchaseSubItem{
			Name:          name,
			Qty:           qty,
			PurchasedQty:  &pQty,
			Unit:          unit,
			HpsPrice:      it.Price,
			HpsSource:     "Pembayaran Manual Keuangan",
			FinalPrice:    it.Price,
			GoodsKind:     goodsKind,
		})
	}
	groups := []models.PurchaseRequestItem{{
		Name:  title,
		Items: subItems,
	}}
	totalHps, totalFinal := recalcItems(groups)
	if cents(totalFinal) <= 0 {
		return nil, Invalid("total nominal pembayaran harus lebih dari 0")
	}
	itemsJSON, err := json.Marshal(groups)
	if err != nil {
		return nil, err
	}

	// Resolve work unit & outlet (optional for manual payment)
	var outletID, workUnitID interface{}
	if wuID := strings.TrimSpace(input.WorkUnitID); wuID != "" {
		var oID sql.NullString
		if err := database.DB.QueryRow(`SELECT outlet_id FROM work_units WHERE id = $1`, wuID).Scan(&oID); err != nil {
			return nil, Invalid("unit kerja tidak ditemukan")
		}
		workUnitID = wuID
		if oID.Valid && strings.TrimSpace(oID.String) != "" {
			outletID = strings.TrimSpace(oID.String)
		}
	}
	var projectID interface{}
	if pID := strings.TrimSpace(input.ProjectID); pID != "" && reqType != RequestTypeBahan {
		var exists bool
		if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1)`, pID).Scan(&exists); err != nil || !exists {
			return nil, Invalid("projek tidak ditemukan")
		}
		projectID = pID
	}

	// Resolve vendor & payment destination
	vID := strings.TrimSpace(input.VendorID)
	vName := strings.TrimSpace(input.VendorName)
	var vAcct vendorAccount
	if vID != "" {
		resolvedID, resolvedName, verr := resolvePurchaseVendor(database.DB, vID, vName)
		if verr != nil {
			return nil, verr
		}
		vID, vName = resolvedID, resolvedName
		_ = database.DB.QueryRow(`SELECT COALESCE(bank_name,''), COALESCE(account_number,''), COALESCE(account_holder,'') FROM vendors WHERE id = $1`, vID).
			Scan(&vAcct.bank, &vAcct.number, &vAcct.holder)
	}
	if vName == "" {
		return nil, Invalid("pilih vendor terdaftar atau isi nama penerima pembayaran")
	}

	payMethod := strings.TrimSpace(input.PayToMethod)
	if payMethod == "" {
		payMethod = PayToVendorAccount
	}
	var st payToStored
	st.note = strings.TrimSpace(input.PayToNote)
	switch payMethod {
	case PayToVendorAccount:
		st.method = PayToVendorAccount
		if vID != "" && vAcct.registered() && strings.TrimSpace(input.PayToAccountNumber) == "" {
			st.bank, st.number, st.holder = vAcct.bank, vAcct.number, vAcct.holder
		} else {
			st.bank = strings.TrimSpace(input.PayToBankName)
			st.number = strings.TrimSpace(input.PayToAccountNumber)
			st.holder = strings.TrimSpace(input.PayToAccountHolder)
			if st.holder == "" {
				st.holder = vName
			}
			if st.bank == "" || st.number == "" {
				return nil, Invalid("nama bank dan nomor rekening tujuan wajib diisi untuk metode transfer rekening")
			}
		}
	case PayToMarketplaceVA:
		st.method = PayToMarketplaceVA
		st.marketplace = strings.TrimSpace(input.PayToMarketplace)
		st.orderNumber = strings.TrimSpace(input.PayToOrderNumber)
		st.bank = strings.TrimSpace(input.PayToBankName)
		st.number = strings.TrimSpace(input.PayToAccountNumber)
		st.holder = strings.TrimSpace(input.PayToAccountHolder)
		if st.holder == "" {
			st.holder = st.marketplace
		}
		if st.marketplace == "" || st.bank == "" || st.number == "" {
			return nil, Invalid("marketplace, bank VA, dan nomor VA wajib diisi")
		}
	case PayToManual:
		st.method = PayToManual
		st.text = strings.TrimSpace(input.PayToText)
		if st.text == "" {
			st.text = "Tunai / Pembayaran Langsung kepada " + vName
		}
	default:
		return nil, Invalid("metode tujuan pembayaran tidak valid")
	}

	invoiceURL := strings.TrimSpace(input.InvoiceURL)
	if invoiceURL != "" && ValidateUploadURL(invoiceURL) != nil {
		return nil, Invalid("tautan berkas invoice tidak valid")
	}

	now := time.Now().UTC()
	dateCode := now.Format("020106")
	reqNumber, err := generatePrefixedNumber("MPY-" + dateCode + "-")
	if err != nil {
		return nil, fmt.Errorf("gagal membuat nomor pembayaran manual: %w", err)
	}
	batchNumber, err := generatePrefixedNumber("AJU-" + dateCode + "-")
	if err != nil {
		return nil, fmt.Errorf("gagal membuat nomor pengajuan bayar: %w", err)
	}

	id := NewULID()
	actorID := nilIfEmpty(validActorID(input.ActorID))
	notes := strings.TrimSpace(input.Notes)
	_, err = database.DB.Exec(`
		INSERT INTO purchase_requests (
			id, request_number, outlet_id, work_unit_id, project_id,
			request_type, goods_kind, requested_by, requested_by_id,
			vendor_id, vendor_name, status, items,
			total_amount, total_hps, total_final, notes,
			invoice_number, invoice_url,
			pay_to_method, pay_to_marketplace, pay_to_order_number,
			pay_to_bank_name, pay_to_account_number, pay_to_account_holder,
			pay_to_text, pay_to_note, pay_to_set_by, pay_to_set_by_id, pay_to_set_at,
			is_manual_payment, payment_approval_status, payment_batch_number,
			payment_submitted_by, payment_submitted_at, payment_submission_notes,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, 'payment_requested', $12,
			$13, $14, $15, $16,
			$17, $18,
			$19, $20, $21,
			$22, $23, $24,
			$25, $26, $8, $9, $27,
			TRUE, 'pending', $28,
			$8, $27, $16,
			$27, $27
		)`,
		id, reqNumber, outletID, workUnitID, projectID,
		reqType, goodsKind, input.ActorName, actorID,
		nilIfEmpty(vID), vName, itemsJSON,
		totalFinal, totalHps, totalFinal, notes,
		strings.TrimSpace(input.InvoiceNumber), invoiceURL,
		st.method, st.marketplace, st.orderNumber,
		st.bank, st.number, st.holder,
		st.text, st.note, now,
		batchNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan pembayaran manual: %w", err)
	}
	return GetPurchaseRequest(id)
}

// SubmitPaymentApprovalBatch mengumpulkan item menunggu pembayaran yang diceklis
// ke dalam satu nomor pengajuan pembayaran (AJU-ddmmYY-nnn) untuk disetujui
// Manager Keuangan atau General Manager.
func SubmitPaymentApprovalBatch(input models.SubmitPaymentApprovalInput) (string, int, error) {
	if len(input.IDs) == 0 {
		return "", 0, Invalid("pilih minimal 1 tagihan untuk diajukan pembayarannya")
	}
	cleanIDs := make([]string, 0, len(input.IDs))
	seen := map[string]bool{}
	for _, raw := range input.IDs {
		id := strings.TrimSpace(raw)
		if id != "" && !seen[id] {
			seen[id] = true
			cleanIDs = append(cleanIDs, id)
		}
	}
	if len(cleanIDs) == 0 {
		return "", 0, Invalid("pilih minimal 1 tagihan untuk diajukan pembayarannya")
	}
	notes := strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(notes) > 1000 {
		return "", 0, Invalid("catatan pengajuan pembayaran maksimal 1000 karakter")
	}

	now := time.Now().UTC()
	batchNumber, err := generatePrefixedNumber("AJU-" + now.Format("020106") + "-")
	if err != nil {
		return "", 0, fmt.Errorf("gagal membuat nomor pengajuan pembayaran: %w", err)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()

	for _, id := range cleanIDs {
		var reqNum, status, approvalStatus string
		var totalFinal, paidAmount float64
		err := tx.QueryRow(`
			SELECT request_number, status, COALESCE(payment_approval_status, ''), COALESCE(total_final, 0), COALESCE(paid_amount, 0)
			FROM purchase_requests WHERE id = $1 FOR UPDATE`, id).
			Scan(&reqNum, &status, &approvalStatus, &totalFinal, &paidAmount)
		if err == sql.ErrNoRows {
			return "", 0, Invalid("tagihan tidak ditemukan")
		}
		if err != nil {
			return "", 0, err
		}
		if status != "payment_requested" && status != "partial" && status != "approved" {
			return "", 0, Invalid("tagihan #%s berstatus %s — hanya tagihan yang menunggu pembayaran yang bisa diajukan", reqNum, status)
		}
		if math.Round((totalFinal-paidAmount)*100) <= 0 {
			return "", 0, Invalid("tagihan #%s sudah lunas", reqNum)
		}
		if approvalStatus == "approved" {
			return "", 0, Invalid("tagihan #%s sudah disetujui pembayarannya", reqNum)
		}
	}

	res, err := tx.Exec(`
		UPDATE purchase_requests
		SET payment_approval_status = 'pending',
		    payment_batch_number = $1,
		    payment_submitted_by = $2,
		    payment_submitted_at = $3,
		    payment_submission_notes = $4,
		    payment_rejected_by = '',
		    payment_rejected_at = NULL,
		    payment_rejection_reason = '',
		    updated_at = $3
		WHERE id = ANY($5::text[])`,
		batchNumber, input.ActorName, now, notes, pq.Array(cleanIDs))
	if err != nil {
		return "", 0, err
	}
	affected, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return batchNumber, int(affected), nil
}

// DecidePaymentApprovalBatch menyetujui atau menolak pengajuan pembayaran
// (baik hasil ceklis tagihan maupun Pembayaran Manual) oleh Manager Keuangan
// atau General Manager.
func DecidePaymentApprovalBatch(input models.DecidePaymentApprovalInput) (int, error) {
	if len(input.IDs) == 0 {
		return 0, Invalid("pilih minimal 1 pengajuan pembayaran")
	}
	decision := strings.ToLower(strings.TrimSpace(input.Decision))
	if decision != "approve" && decision != "reject" {
		return 0, Invalid("keputusan harus 'approve' atau 'reject'")
	}
	reason := strings.TrimSpace(input.Reason)
	if decision == "reject" && utf8.RuneCountInString(reason) < 3 {
		return 0, Invalid("alasan penolakan pengajuan pembayaran wajib diisi (minimal 3 karakter)")
	}

	cleanIDs := make([]string, 0, len(input.IDs))
	seen := map[string]bool{}
	for _, raw := range input.IDs {
		id := strings.TrimSpace(raw)
		if id != "" && !seen[id] {
			seen[id] = true
			cleanIDs = append(cleanIDs, id)
		}
	}
	if len(cleanIDs) == 0 {
		return 0, Invalid("pilih minimal 1 pengajuan pembayaran")
	}

	now := time.Now().UTC()
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, id := range cleanIDs {
		var reqNum, status string
		err := tx.QueryRow(`SELECT request_number, status FROM purchase_requests WHERE id = $1 FOR UPDATE`, id).
			Scan(&reqNum, &status)
		if err == sql.ErrNoRows {
			return 0, Invalid("tagihan tidak ditemukan")
		}
		if err != nil {
			return 0, err
		}
		if status == "paid" || status == "cancelled" || status == "rejected" {
			return 0, Invalid("tagihan #%s sudah berstatus %s", reqNum, status)
		}
	}

	var res sql.Result
	if decision == "approve" {
		actorID := nilIfEmpty(validActorID(input.ActorID))
		res, err = tx.Exec(`
			UPDATE purchase_requests
			SET payment_approval_status = 'approved',
			    payment_approved_by = $1,
			    payment_approved_at = $2,
			    payment_rejected_by = '',
			    payment_rejected_at = NULL,
			    payment_rejection_reason = '',
			    approved_by = CASE WHEN is_manual_payment AND COALESCE(approved_by, '') = '' THEN $1 ELSE approved_by END,
			    approved_by_id = CASE WHEN is_manual_payment AND approved_by_id IS NULL THEN $3::char(26) ELSE approved_by_id END,
			    approved_at = CASE WHEN is_manual_payment AND approved_at IS NULL THEN $2 ELSE approved_at END,
			    updated_at = $2
			WHERE id = ANY($4::text[])`,
			input.ActorName, now, actorID, pq.Array(cleanIDs))
	} else {
		res, err = tx.Exec(`
			UPDATE purchase_requests
			SET payment_approval_status = 'rejected',
			    payment_rejected_by = $1,
			    payment_rejected_at = $2,
			    payment_rejection_reason = $3,
			    updated_at = $2
			WHERE id = ANY($4::text[])`,
			input.ActorName, now, reason, pq.Array(cleanIDs))
	}
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(affected), nil
}
