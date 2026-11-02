package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

type financePOSPayment struct {
	Method string  `json:"payment_method"`
	Amount float64 `json:"amount"`
	At     string  `json:"created_at"`
}

type financePOSRestore struct {
	Payments      []financePOSPayment `json:"payments,omitempty"`
	ReservationID string              `json:"reservation_id,omitempty"`
	Incomplete    bool                `json:"incomplete,omitempty"`
}

// Future recovery payloads remain valid POS writes even when their optional
// payment representation is not yet understood by this reporting version.
func (r *financePOSRestore) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		r.Incomplete = true
		return nil
	}
	if value, ok := fields["reservation_id"]; ok {
		if err := json.Unmarshal(value, &r.ReservationID); err != nil {
			r.Incomplete = true
		}
	}
	if value, ok := fields["payments"]; ok {
		if err := json.Unmarshal(value, &r.Payments); err != nil {
			r.Payments, r.Incomplete = nil, true
		}
	}
	return nil
}

type financePOSState struct {
	ID          string  `json:"id"`
	LocalID     string  `json:"local_id"`
	OutletID    string  `json:"outlet_id"`
	OrderID     string  `json:"order_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	Amount      float64 `json:"total_amount"`
	Tax         float64 `json:"tax_amount"`
	Method      string  `json:"payment_method"`
	Cashier     string  `json:"cashier_name"`
	Customer    string  `json:"customer_name"`
	Holding     bool    `json:"is_holding"`
	Costed      bool    `json:"costed"`
	PaymentInfo struct {
		Method     string            `json:"method"`
		Status     string            `json:"payment_status"`
		Paid       float64           `json:"paid_amount"`
		PaidAt     string            `json:"paid_at"`
		VoidedAt   string            `json:"voided_at"`
		Compliment string            `json:"complimented_at"`
		Restore    financePOSRestore `json:"restore"`
	} `json:"payment_info"`
	Payments []financePOSPayment `json:"payments"`
}

// Keep only accounting inputs. Sync times, enqueue versions and generated
// transaction_payments IDs are not financial revisions.
const financePOSOrderJSON = `jsonb_build_object(
	'id',TRIM(o.id),'local_id',o.local_id,'outlet_id',TRIM(o.outlet_id),
	'created_at',o.created_at,'updated_at',o.updated_at,'total_amount',o.total_amount,
	'customer_name',o.customer_name,'is_holding',COALESCE(o.is_holding,false),
	'payment_info',COALESCE(o.payment_info,'{}'::jsonb))`

const financePOSTransactionJSON = `jsonb_build_object(
	'id',TRIM(t.id),'local_id',t.local_id,'outlet_id',TRIM(t.outlet_id),'order_id',TRIM(COALESCE(t.order_id,'')),
	'created_at',t.created_at,'total_amount',t.total_amount,'tax_amount',COALESCE(t.tax_amount,0),
	'payment_method',COALESCE(t.payment_method,''),'cashier_name',COALESCE(t.cashier_name,''),
	'costed',EXISTS(SELECT 1 FROM stock_movements sm WHERE sm.ref_type='transaction' AND sm.ref_id=t.id::text AND sm.movement_type='sale'),
	'payments',COALESCE((SELECT jsonb_agg(jsonb_build_object('payment_method',tp.payment_method,
		'amount',tp.amount,'created_at',tp.created_at) ORDER BY tp.created_at,tp.payment_method,tp.amount)
		FROM transaction_payments tp WHERE tp.transaction_id=t.id AND tp.outlet_id=t.outlet_id),'[]'::jsonb))`

func financePOSTime(value string) time.Time {
	at, _ := financeSourceTime(value)
	return at.UTC()
}

func financePOSHash(state financePOSState, voided time.Time) string {
	state.UpdatedAt = ""
	state.Costed = false
	state.Customer = ""
	state.Cashier = ""
	encoded, _ := json.Marshal(struct {
		State  financePOSState
		Voided time.Time
	}{state, voided})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func captureFinancePOSOrder(tx *sql.Tx, outletID, orderID string) error {
	return captureFinancePOS(tx, "order", outletID, orderID, time.Time{})
}

func captureFinancePOSTransaction(tx *sql.Tx, outletID, txnID string) error {
	return captureFinancePOS(tx, "transaction", outletID, txnID, time.Time{})
}

func captureFinancePOSVoid(tx *sql.Tx, outletID, txnID string, voidedAt time.Time) error {
	return captureFinancePOS(tx, "transaction", outletID, txnID, voidedAt.UTC())
}

// Callers hold the POS outlet lock. Deduplicate only the immediately preceding
// state, so an A -> B -> A monetary correction retains both changes.
func captureFinancePOS(tx *sql.Tx, kind, outletID, sourceID string, voided time.Time) error {
	query := `SELECT ` + financePOSOrderJSON + ` FROM cloud_orders o WHERE o.outlet_id=$1 AND o.id=$2`
	if kind == "transaction" {
		query = `SELECT ` + financePOSTransactionJSON + ` FROM cloud_transactions t WHERE t.outlet_id=$1 AND t.id=$2`
	}
	var raw []byte
	if err := tx.QueryRow(query, outletID, sourceID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	var state financePOSState
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if kind == "transaction" && strings.EqualFold(strings.TrimSpace(state.Method), "adjustment") {
		return nil
	}
	created := financePOSTime(state.CreatedAt)
	if created.IsZero() {
		return fmt.Errorf("waktu sumber POS tidak valid")
	}
	if kind == "order" {
		voided = financePOSTime(state.PaymentInfo.VoidedAt)
	}
	hash := financePOSHash(state, voided)
	var previous string
	err := tx.QueryRow(`SELECT financial_hash FROM finance_pos_snapshots
		WHERE outlet_id=$1 AND entity_kind=$2 AND source_id=$3 ORDER BY id DESC LIMIT 1`, outletID, kind, sourceID).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && previous == hash {
		return nil
	}
	var updatedAt, voidedAt interface{}
	if at := financePOSTime(state.UpdatedAt); !at.IsZero() {
		updatedAt = at
	}
	if !voided.IsZero() {
		voidedAt = voided
	}
	_, err = tx.Exec(`INSERT INTO finance_pos_snapshots
		(outlet_id,entity_kind,source_id,source_created_at,source_updated_at,voided_at,financial_hash,payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, outletID, kind, sourceID, created, updatedAt, voidedAt, hash, string(raw))
	return err
}

type financePOSRevision struct {
	State    financePOSState
	Observed time.Time
	Voided   time.Time
}

type financePOSEntity struct {
	Kind      string
	Revisions []financePOSRevision
}

func loadFinancePOSEntities(outlet string, scope []string) (map[string]*financePOSEntity, error) {
	if outlet != "" {
		if !financeInScope(outlet, scope) {
			return nil, ErrFinanceAccess
		}
		scope = []string{outlet}
	}
	tx, err := database.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	entities := map[string]*financePOSEntity{}
	appendRevision := func(kind string, r financePOSRevision) {
		if kind == "transaction" && strings.EqualFold(strings.TrimSpace(r.State.Method), "adjustment") {
			return
		}
		key := kind + ":" + r.State.OutletID + ":" + r.State.ID
		entity := entities[key]
		if entity == nil {
			entity = &financePOSEntity{Kind: kind}
			entities[key] = entity
		}
		if r.State.Costed {
			// A later successful stock retry supplies proof for this same sale;
			// it is metadata, not another receipt or accounting correction.
			for i := range entity.Revisions {
				entity.Revisions[i].State.Costed = true
			}
		} else {
			for _, previous := range entity.Revisions {
				if previous.State.Costed {
					r.State.Costed = true
					break
				}
			}
		}
		if len(entity.Revisions) > 0 {
			last := entity.Revisions[len(entity.Revisions)-1]
			if financePOSHash(last.State, last.Voided) == financePOSHash(r.State, r.Voided) {
				return
			}
		}
		entity.Revisions = append(entity.Revisions, r)
	}
	args := []interface{}{}
	filter := ""
	if scope != nil {
		filter = ` WHERE outlet_id=ANY($1::text[])`
		args = append(args, pq.Array(scope))
	}
	rows, err := tx.Query(`SELECT entity_kind,payload,observed_at,voided_at FROM finance_pos_snapshots`+filter+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind string
		var raw []byte
		var r financePOSRevision
		var voided sql.NullTime
		if err := rows.Scan(&kind, &raw, &r.Observed, &voided); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &r.State); err != nil {
			rows.Close()
			return nil, err
		}
		if voided.Valid {
			r.Voided = voided.Time.UTC()
		}
		appendRevision(kind, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Current rows are read-only compatibility sources for history predating
	// capture. They are never inserted into the snapshot table by reporting.
	for _, source := range []struct{ kind, alias, query string }{
		{"order", "o", `SELECT ` + financePOSOrderJSON + ` FROM cloud_orders o`},
		{"transaction", "t", `SELECT ` + financePOSTransactionJSON + ` FROM cloud_transactions t`},
	} {
		q := source.query
		if scope != nil {
			q += ` WHERE ` + source.alias + `.outlet_id=ANY($1::text[])`
		}
		rows, err := tx.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var raw []byte
			var r financePOSRevision
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return nil, err
			}
			if err := json.Unmarshal(raw, &r.State); err != nil {
				rows.Close()
				return nil, err
			}
			r.Observed = time.Now().UTC()
			if source.kind == "order" {
				r.Voided = financePOSTime(r.State.PaymentInfo.VoidedAt)
			}
			appendRevision(source.kind, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entities, nil
}

func financePOSPayments(state financePOSState) []financePOSPayment {
	if strings.EqualFold(strings.TrimSpace(state.Method), "compliment") {
		return nil
	}
	payments := append([]financePOSPayment(nil), state.Payments...)
	if len(payments) == 0 && state.Amount != 0 {
		payments = []financePOSPayment{{Method: state.Method, Amount: state.Amount, At: state.CreatedAt}}
	}
	var sum float64
	for _, payment := range payments {
		sum += payment.Amount
	}
	// Historical cash lines may contain tendered cash, including change.
	// Apply the same normalization as POS transaction persistence; never invent
	// an additional payment when a header lacks complete payment evidence.
	if excess := round2(sum - state.Amount); state.Amount > 0 && excess > 0 {
		for i := range payments {
			if strings.EqualFold(strings.TrimSpace(payments[i].Method), "cash") {
				cut := excess
				if cut > payments[i].Amount {
					cut = payments[i].Amount
				}
				payments[i].Amount = round2(payments[i].Amount - cut)
				excess = round2(excess - cut)
			}
			if excess <= 0 {
				break
			}
		}
	}
	return payments
}

func financePOSOrderEligibleAt(entity *financePOSEntity, at time.Time) bool {
	state := entity.Revisions[0].State
	for _, revision := range entity.Revisions[1:] {
		effective := financePOSTime(revision.State.UpdatedAt)
		if effective.IsZero() {
			effective = revision.Observed
		}
		if !effective.After(at) {
			state = revision.State
		}
	}
	return !state.Holding
}

func financePOSOrderInvoiceAt(entity *financePOSEntity, at time.Time) float64 {
	state := entity.Revisions[0].State
	previous := financePOSTime(state.CreatedAt)
	for _, revision := range entity.Revisions[1:] {
		effective := financePOSTime(revision.State.UpdatedAt)
		if effective.IsZero() || !effective.After(previous) {
			effective = revision.Observed
		}
		if !effective.After(at) {
			state = revision.State
		}
		previous = effective
	}
	if state.Holding {
		return 0
	}
	if compliment := financePOSTime(state.PaymentInfo.Compliment); !compliment.IsZero() && !compliment.After(at) {
		return 0
	}
	return state.Amount
}

// A legacy normalized line set can be incomplete even though the final receipt
// header and its linked paid order agree. Preserve that corroborated receipt at
// completion, without assigning mixed/DP/unknown gaps to cash or changing old
// dated lines. Reporting does not rewrite transaction_payments.
func financePOSReceiptEvidence(state financePOSState, order *financePOSEntity) ([]financePOSPayment, bool, string) {
	payments := financePOSPayments(state)
	var sum float64
	for _, payment := range payments {
		sum += payment.Amount
	}
	gap := round2(state.Amount - sum)
	if gap < 0 {
		return payments, len(state.Payments) > 0, "Rincian pembayaran melebihi header transaksi; bukti pembayaran dipertahankan dan perlu diperiksa."
	}
	method := strings.ToLower(strings.TrimSpace(state.Method))
	if gap <= 0 || len(state.Payments) == 0 {
		return payments, len(state.Payments) > 0, ""
	}
	if order == nil || !statusIn(method, []string{"cash", "qris", "card", "transfer"}) {
		return payments, true, "Rincian pembayaran lebih kecil dari header tetapi bukti pelunasan/metode tunggal belum cukup; selisih kas tidak ditebak."
	}
	for _, payment := range state.Payments {
		if !strings.EqualFold(strings.TrimSpace(payment.Method), method) {
			return payments, true, "Metode rincian berbeda dari header; selisih pembayaran perlu diperiksa dan tidak ditambahkan sebagai kas."
		}
	}
	completed := financePOSTime(state.CreatedAt)
	verified := false
	for index, revision := range order.Revisions {
		candidate := revision.State
		if candidate.PaymentInfo.Status != "paid" || round2(candidate.PaymentInfo.Paid) < round2(state.Amount) {
			continue
		}
		paidAt := financePOSTime(candidate.PaymentInfo.PaidAt)
		if !paidAt.IsZero() && paidAt.After(completed) {
			continue
		}
		if index > 0 && paidAt.IsZero() {
			updated := financePOSTime(candidate.UpdatedAt)
			if updated.IsZero() || updated.After(completed) {
				continue
			}
		}
		verified = true
		break
	}
	if !verified {
		return payments, true, "Header penerimaan belum dapat dicocokkan dengan bukti pelunasan pada waktu transaksi; selisih tidak ditambahkan sebagai kas."
	}
	payments = append(payments, financePOSPayment{Method: method, Amount: gap, At: state.CreatedAt})
	return payments, true, "Rincian pembayaran lama belum lengkap; selisih memakai header penerimaan satu metode yang cocok dengan saldo lunas order pada waktu penyelesaian."
}

func financePOSVoidAt(entity *financePOSEntity) time.Time {
	var at time.Time
	for _, r := range entity.Revisions {
		if !r.Voided.IsZero() && (at.IsZero() || r.Voided.Before(at)) {
			at = r.Voided
		}
	}
	return at
}

type financePOSEmitter func(time.Time, string, string, financePOSState, string, string, float64, string, string)
type financePOSEvidence func(financePOSState) ([]financePOSPayment, bool)

func financePOSDescription(source string) string {
	labels := map[string]string{
		"sale": "Pembayaran POS", "receivable": "Pengakuan piutang POS",
		"sale_void": "Pembatalan penjualan POS", "sale_refund": "Pengembalian pembayaran POS",
		"sale_tax": "Pajak penjualan POS", "sale_tax_void": "Pembalikan pajak penjualan POS",
		"sale_correction": "Koreksi pembayaran POS", "sale_tax_correction": "Koreksi pajak penjualan POS",
		"sale_link_correction": "Koreksi tautan order POS", "pos_review": "Pemeriksaan riwayat POS",
		"sale_compliment": "Komplimen penjualan POS",
	}
	if label, ok := labels[source]; ok {
		return label
	}
	return "Transaksi POS"
}

func financePOSOrderPayments(state financePOSState) ([]financePOSPayment, bool) {
	if len(state.PaymentInfo.Restore.Payments) > 0 && !state.PaymentInfo.Restore.Incomplete {
		return state.PaymentInfo.Restore.Payments, true
	}
	method := strings.ToLower(strings.TrimSpace(state.PaymentInfo.Method))
	if state.PaymentInfo.Paid > 0 && !financePOSTime(state.PaymentInfo.PaidAt).IsZero() &&
		statusIn(method, []string{"cash", "qris", "card", "transfer", ReservationDpMethod}) {
		return []financePOSPayment{{Method: method, Amount: state.PaymentInfo.Paid, At: state.PaymentInfo.PaidAt}}, false
	}
	return nil, false
}

func financePOSPaymentMap(payments []financePOSPayment) (map[string]float64, map[string]float64) {
	keys, accounts := map[string]float64{}, map[string]float64{}
	for _, payment := range payments {
		method := strings.ToLower(strings.TrimSpace(payment.Method))
		if method == "compliment" || method == "adjustment" {
			continue
		}
		key := financePOSTime(payment.At).Format(time.RFC3339Nano) + ":" + method
		keys[key] = round2(keys[key] + payment.Amount)
		account := glAccCash
		if method == ReservationDpMethod {
			account = glAccCustomerDep
		}
		accounts[account] = round2(accounts[account] + payment.Amount)
	}
	return keys, accounts
}

// Preserve established payment evidence. Appending a genuinely new payment
// retains its source time; changing an existing date/method/amount is a dated
// accounting correction at observation time, since POS has no revision clock.
func financePOSEmitPayments(entity *financePOSEntity, voided time.Time, evidence financePOSEvidence,
	creditFor func(financePOSState, time.Time) string, emit financePOSEmitter) bool {
	firstIndex, baseIndex := -1, -1
	var payments []financePOSPayment
	var actual bool
	for i, revision := range entity.Revisions {
		if candidate, complete := evidence(revision.State); len(candidate) > 0 {
			firstIndex, baseIndex, payments, actual = i, i, candidate, complete
			break
		}
	}
	if firstIndex < 0 {
		return false
	}
	initial := entity.Revisions[firstIndex].State
	enriched := false
	if !actual {
		_, oldTotals := financePOSPaymentMap(payments)
		oldTotal := oldTotals[glAccCash] + oldTotals[glAccCustomerDep]
		for i := firstIndex + 1; i < len(entity.Revisions); i++ {
			candidate, complete := evidence(entity.Revisions[i].State)
			_, totals := financePOSPaymentMap(candidate)
			if complete && len(candidate) > 0 && round2(totals[glAccCash]+totals[glAccCustomerDep]) == round2(oldTotal) {
				baseIndex, payments, enriched = i, candidate, true
				break
			}
		}
	}
	validPayments := make([]financePOSPayment, 0, len(payments))
	for i, payment := range payments {
		at := financePOSTime(payment.At)
		if at.IsZero() || (!voided.IsZero() && at.After(voided)) {
			emit(financePOSTime(initial.CreatedAt), "pos_review", initial.ID+fmt.Sprintf(":invalid_payment:%d", i), initial,
				"", "", 0, "", "Waktu pembayaran kosong atau setelah tanggal void; penerimaan/refund tidak dibuat tanpa bukti waktu yang konsisten.")
			continue
		}
		validPayments = append(validPayments, payment)
	}
	payments = validPayments
	credit := creditFor(initial, financePOSTime(initial.CreatedAt))
	emitPayment := func(state financePOSState, payment financePOSPayment, id, review string) {
		method := strings.ToLower(strings.TrimSpace(payment.Method))
		if method == "compliment" || method == "adjustment" {
			return
		}
		at := financePOSTime(payment.At)
		if at.IsZero() {
			emit(financePOSTime(state.CreatedAt), "pos_review", id, state, "", "", 0, "", "Waktu pembayaran tidak tersedia; penerimaan kas tidak direkonstruksi tanpa bukti waktu.")
			return
		}
		debit := glAccCash
		if method == ReservationDpMethod {
			debit = glAccCustomerDep
		}
		emit(at, "sale", id, state, debit, credit, payment.Amount, "sales", review)
	}
	for i, payment := range payments {
		review := ""
		if i == 0 && entity.Kind == "transaction" && !initial.Costed {
			review = "Belum ada pemakaian stok tertaut pada penjualan ini; periksa kelengkapan HPP produk."
		}
		if enriched {
			review = strings.TrimSpace(review + " Rincian waktu pembayaran melengkapi fallback header lama; periksa bukti sumber.")
		}
		emitPayment(initial, payment, initial.ID+fmt.Sprintf(":payment:%d", i), review)
	}
	keys, totals := financePOSPaymentMap(payments)
	for i := firstIndex + 1; i < len(entity.Revisions); i++ {
		revision := entity.Revisions[i]
		state := revision.State
		newCredit := creditFor(state, financePOSTime(state.CreatedAt))
		if newCredit != credit {
			if voided.IsZero() || !revision.Observed.After(voided) {
				emit(revision.Observed, "sale_link_correction", initial.ID+fmt.Sprintf(":%d", i), state,
					credit, newCredit, round2(totals[glAccCash]+totals[glAccCustomerDep]), "", "Tautan order transaksi diperbaiki pada waktu pencatatan cloud; kas tidak diterima lagi.")
				credit = newCredit
			} else {
				emit(revision.Observed, "pos_review", initial.ID+fmt.Sprintf(":link:%d", i), state,
					"", "", 0, "", "Tautan order berubah setelah tanggal void; periksa koreksi tanpa mengubah penerimaan historis.")
			}
		}
		if i <= baseIndex {
			continue
		}
		candidate, _ := evidence(state)
		if len(candidate) == 0 {
			emit(revision.Observed, "pos_review", initial.ID+fmt.Sprintf(":missing:%d", i), state,
				"", "", 0, "", "Payload tidak menyertakan rincian pembayaran; bukti pembayaran sebelumnya dipertahankan.")
			continue
		}
		newKeys, newTotals := financePOSPaymentMap(candidate)
		same, appendOnly := len(keys) == len(newKeys), true
		for key, amount := range keys {
			if amount != newKeys[key] {
				same, appendOnly = false, false
			}
		}
		if same {
			continue
		}
		invalidAfterVoid := false
		if !voided.IsZero() {
			for _, payment := range candidate {
				if paid := financePOSTime(payment.At); paid.IsZero() || paid.After(voided) {
					invalidAfterVoid = true
				}
			}
		}
		if invalidAfterVoid {
			emit(revision.Observed, "pos_review", initial.ID+fmt.Sprintf(":late_payment:%d", i), state,
				"", "", 0, "", "Pembayaran bertanggal setelah void; pemeriksaan diperlukan tanpa menghidupkan kembali penerimaan.")
			continue
		}
		if appendOnly {
			for key, amount := range newKeys {
				if _, exists := keys[key]; exists {
					continue
				}
				for _, payment := range candidate {
					method := strings.ToLower(strings.TrimSpace(payment.Method))
					if financePOSTime(payment.At).Format(time.RFC3339Nano)+":"+method == key {
						payment.Amount = amount
						emitPayment(state, payment, initial.ID+fmt.Sprintf(":append:%d:%s", i, key), "")
						break
					}
				}
			}
		} else if voided.IsZero() || !revision.Observed.After(voided) {
			for _, account := range []string{glAccCash, glAccCustomerDep} {
				emit(revision.Observed, "sale_correction", initial.ID+fmt.Sprintf(":%d:%s", i, account), state,
					account, credit, round2(newTotals[account]-totals[account]), "sales", "Alokasi/waktu pembayaran dikoreksi pada waktu pencatatan cloud; riwayat yang sudah diketahui dipertahankan.")
			}
		} else {
			emit(revision.Observed, "pos_review", initial.ID+fmt.Sprintf(":after_void:%d", i), state,
				"", "", 0, "", "Pembayaran berubah setelah tanggal void; diperlukan pemeriksaan, tanpa membuat penerimaan/refund terbalik.")
			continue
		}
		keys, totals = newKeys, newTotals
	}
	if !voided.IsZero() {
		for _, account := range []string{glAccCash, glAccCustomerDep} {
			emit(voided, "sale_refund", initial.ID+":"+account, initial, credit, account, totals[account], "sales", "")
		}
	}
	return len(payments) > 0
}

// All operations have fixed source dates; no posting depends on the report's
// closing date. This keeps adjacent P&L periods additive.
func loadFinancePOSJournal(from, to, outlet string, scope []string) ([]models.FinanceOperation, error) {
	entities, err := loadFinancePOSEntities(outlet, scope)
	if err != nil {
		return nil, err
	}
	location := GetTimezoneLocation()
	start, err := time.ParseInLocation("2006-01-02", from, location)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02", to, location)
	if err != nil || end.Before(start) {
		return nil, Invalid("rentang tanggal tidak valid")
	}
	end = end.AddDate(0, 0, 1)
	ops := []models.FinanceOperation{}
	emit := func(at time.Time, source, id string, state financePOSState, debit, credit string, amount float64, category, review string) {
		if at.IsZero() || at.Before(start) || !at.Before(end) || (round2(amount) == 0 && review == "") {
			return
		}
		o := models.FinanceOperation{ID: "pos_" + source + ":" + state.OutletID + ":" + id,
			Source: source, SourceID: state.ID, Reference: state.LocalID,
			At: at, Date: at.In(location).Format("2006-01-02"), OutletID: state.OutletID,
			Description: financePOSDescription(source) + ": " + state.LocalID, Counterparty: state.Customer,
			Actor: state.Cashier, Method: state.Method, Status: "posted", CashCategory: category,
			ReviewNote: review, Postings: []models.FinancePosting{}}
		financePair(&o, debit, credit, amount)
		for _, p := range o.Postings {
			if p.AccountCode == glAccCash {
				o.CashIn += p.Debit
				o.CashOut += p.Credit
			}
		}
		ops = append(ops, o)
	}
	orders := map[string]*financePOSEntity{}
	aliases := map[string]*financePOSEntity{}
	for _, entity := range entities {
		if entity.Kind == "order" {
			s := entity.Revisions[0].State
			orders[s.OutletID+":"+s.ID] = entity
			aliases[s.OutletID+":"+s.LocalID] = entity
		}
	}
	linked := map[*financePOSEntity][]*financePOSEntity{}
	for _, entity := range entities {
		if entity.Kind != "transaction" {
			continue
		}
		s := entity.Revisions[len(entity.Revisions)-1].State
		order := orders[s.OutletID+":"+s.OrderID]
		if order == nil {
			order = aliases[s.OutletID+":"+s.OrderID]
		}
		if order != nil {
			linked[order] = append(linked[order], entity)
		}
	}
	for _, entity := range orders {
		first := entity.Revisions[0]
		state := first.State
		created := financePOSTime(state.CreatedAt)
		if voided := financePOSVoidAt(entity); !voided.IsZero() && voided.Before(created) {
			emit(created, "pos_review", state.ID, state, "", "", 0, "", "Waktu void mendahului waktu sumber order; riwayat penjualan belum dapat dipastikan.")
			continue
		}
		var compliment time.Time
		knownPayment := false
		for _, revision := range entity.Revisions {
			candidate := financePOSTime(revision.State.PaymentInfo.Compliment)
			if !candidate.IsZero() && (compliment.IsZero() || candidate.Before(compliment)) {
				compliment = candidate
			}
			payments, _ := financePOSOrderPayments(revision.State)
			for _, payment := range payments {
				if !financePOSTime(payment.At).IsZero() && payment.Amount > 0 && payment.Method != "compliment" && payment.Method != "adjustment" {
					knownPayment = true
				}
			}
		}
		for _, transaction := range linked[entity] {
			for index, revision := range transaction.Revisions {
				if strings.EqualFold(strings.TrimSpace(revision.State.Method), "compliment") {
					at := financePOSTime(revision.State.CreatedAt)
					if index > 0 {
						at = revision.Observed
					}
					if compliment.IsZero() || at.Before(compliment) {
						compliment = at
					}
					break
				}
			}
		}
		if state.PaymentInfo.Status == "paid" && len(linked[entity]) == 0 && !knownPayment && compliment.IsZero() {
			emit(created, "pos_review", state.ID+":legacy_paid", state, "", "", 0, "", "Order lama pertama kali tercatat lunas tanpa bukti waktu pembayaran; tidak dibuat piutang atau penerimaan kas yang belum terbukti.")
			continue
		}
		amount := 0.0
		recognized := 0.0
		lastAt := created
		voided := financePOSVoidAt(entity)
		for index, revision := range entity.Revisions {
			s := revision.State
			value := s.Amount
			if s.Holding {
				value = 0
			}
			at := created
			review := ""
			if index > 0 {
				at = financePOSTime(s.UpdatedAt)
				if at.IsZero() || !at.After(lastAt) {
					at = revision.Observed
					if value != amount {
						review = "Waktu perubahan nilai sumber tidak tersedia; memakai waktu pencatatan cloud."
					}
				}
			} else if updated := financePOSTime(s.UpdatedAt); updated.After(created) {
				review = "Rincian perubahan sebelum snapshot pertama belum tersedia; nilai memakai sumber POS yang tersimpan."
			}
			if s.PaymentInfo.Restore.Incomplete {
				review = strings.TrimSpace(review + " Format rincian pembayaran POS belum dapat dibaca; periksa sumber pembayaran.")
			}
			if s.PaymentInfo.Compliment != "" && financePOSTime(s.PaymentInfo.Compliment).IsZero() {
				review = strings.TrimSpace(review + " Waktu komplimen tidak valid; tidak meniadakan piutang historis tanpa bukti tanggal.")
			}
			if index > 0 && !voided.IsZero() && at.After(voided) && value != amount {
				emit(revision.Observed, "pos_review", s.ID+fmt.Sprintf(":after_void_invoice:%d", index), s,
					"", "", 0, "", "Nilai order berubah setelah tanggal void; nilai historis dipertahankan untuk diperiksa.")
				continue
			}
			if compliment.IsZero() || at.Before(compliment) {
				delta := round2(value - amount)
				emit(at, "receivable", s.ID+fmt.Sprintf(":%d", index), s, glAccReceivable, glAccRevenue, delta, "", review)
				recognized = round2(recognized + delta)
			} else if review != "" {
				emit(at, "pos_review", s.ID+fmt.Sprintf(":compliment_history:%d", index), s, "", "", 0, "", review)
			}
			amount, lastAt = value, at
		}
		if !compliment.IsZero() && (voided.IsZero() || !compliment.After(voided)) {
			emit(compliment, "sale_compliment", state.ID+":compliment", state, glAccRevenue, glAccReceivable, recognized, "", "")
			recognized = 0
		}
		if !voided.IsZero() {
			emit(voided, "sale_void", state.ID, state, glAccRevenue, glAccReceivable, recognized, "", "")
		}
		if len(linked[entity]) == 0 {
			known := financePOSEmitPayments(entity, voided, financePOSOrderPayments,
				func(financePOSState, time.Time) string { return glAccReceivable }, emit)
			latest := entity.Revisions[len(entity.Revisions)-1].State
			if !known && latest.PaymentInfo.Status == "paid" && latest.Amount > 0 {
				emit(created, "pos_review", state.ID+":missing_payment", state, "", "", 0, "", "Order tercatat lunas tetapi waktu/rincian pembayaran tidak tersedia; kas historis belum dapat dipastikan.")
			}
			if !known && !voided.IsZero() {
				emit(created, "pos_review", state.ID+":missing_void", state, "", "", 0, "", "Void sebelum arsip keuangan tersedia; riwayat pembayaran dan pajak tidak dapat direkonstruksi dengan pasti.")
			}
		}
	}
	for _, entity := range entities {
		if entity.Kind != "transaction" {
			continue
		}
		first := entity.Revisions[0].State
		latest := entity.Revisions[len(entity.Revisions)-1].State
		order := orders[latest.OutletID+":"+latest.OrderID]
		if order == nil && latest.OrderID != "" {
			order = aliases[latest.OutletID+":"+latest.OrderID]
		}
		voided := financePOSVoidAt(entity)
		if voided.IsZero() && order != nil {
			voided = financePOSVoidAt(order)
		}
		if !voided.IsZero() && voided.Before(financePOSTime(first.CreatedAt)) {
			emit(financePOSTime(first.CreatedAt), "pos_review", first.ID, first, "", "", 0, "", "Void mendahului sumber transaksi yang tersimpan; pembayaran tidak direkonstruksi tanpa bukti waktu yang valid.")
			continue
		}
		creditFor := func(s financePOSState, at time.Time) string {
			order := orders[s.OutletID+":"+s.OrderID]
			if order == nil && s.OrderID != "" {
				order = aliases[s.OutletID+":"+s.OrderID]
			}
			if order != nil && financePOSOrderEligibleAt(order, at) && financePOSOrderInvoiceAt(order, at) > 0 {
				return glAccReceivable
			}
			return glAccRevenue
		}
		financePOSEmitPayments(entity, voided, func(s financePOSState) ([]financePOSPayment, bool) {
			matched := orders[s.OutletID+":"+s.OrderID]
			if matched == nil && s.OrderID != "" {
				matched = aliases[s.OutletID+":"+s.OrderID]
			}
			payments, actual, _ := financePOSReceiptEvidence(s, matched)
			return payments, actual
		}, creditFor, emit)
		for index, revision := range entity.Revisions {
			s := revision.State
			matched := orders[s.OutletID+":"+s.OrderID]
			if matched == nil && s.OrderID != "" {
				matched = aliases[s.OutletID+":"+s.OrderID]
			}
			_, _, review := financePOSReceiptEvidence(s, matched)
			if matched != nil && s.Amount > 0 && financePOSOrderInvoiceAt(matched, financePOSTime(s.CreatedAt)) <= 0 {
				review = strings.TrimSpace(review + " Order tertaut tidak memiliki nilai tagihan positif yang terbukti; penerimaan memakai sumber transaksi sebagai penjualan lama, bukan piutang negatif.")
			}
			at := financePOSTime(s.CreatedAt)
			if index > 0 {
				at = revision.Observed
			}
			emit(at, "pos_review", s.ID+fmt.Sprintf(":receipt_evidence:%d", index), s, "", "", 0, "", review)
		}
		tax := first.Tax
		emit(financePOSTime(first.CreatedAt), "sale_tax", first.ID, first, glAccTaxExpense, glAccTaxPayable, tax, "", "")
		for index, revision := range entity.Revisions[1:] {
			delta := round2(revision.State.Tax - tax)
			if delta == 0 {
				continue
			}
			if !voided.IsZero() && revision.Observed.After(voided) {
				emit(revision.Observed, "pos_review", first.ID+fmt.Sprintf(":tax:%d", index), first, "", "", 0, "", "Pajak sumber berubah setelah tanggal void; diperlukan pemeriksaan riwayat.")
				continue
			}
			emit(revision.Observed, "sale_tax_correction", first.ID+fmt.Sprintf(":%d", index), revision.State,
				glAccTaxExpense, glAccTaxPayable, delta, "", "Koreksi pajak memakai waktu pencatatan cloud karena waktu koreksi sumber tidak tersedia.")
			tax = revision.State.Tax
		}
		if !voided.IsZero() {
			emit(voided, "sale_tax_void", first.ID, first, glAccTaxPayable, glAccTaxExpense, tax, "", "")
		}
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].At.Equal(ops[j].At) {
			return ops[i].ID < ops[j].ID
		}
		return ops[i].At.Before(ops[j].At)
	})
	return ops, nil
}

// Legacy POS refunds identify an order only by its first eight local-ID
// characters. Require the same outlet, exact void time and a unique order;
// never suppress another cash movement based only on a similar description.
func financePOSRefundCovered(outletID string, at time.Time, note string, amount float64) (bool, error) {
	if amount <= 0 || !strings.HasPrefix(note, "Void order #") {
		return false, nil
	}
	rows, err := database.DB.Query(`SELECT payload,COALESCE((SELECT o.local_id FROM cloud_orders o
		WHERE o.outlet_id=s.outlet_id AND (o.id::text=s.payload->>'order_id' OR o.local_id=s.payload->>'order_id')
		ORDER BY (o.id::text=s.payload->>'order_id') DESC LIMIT 1),payload->>'order_id')
		FROM (SELECT DISTINCT ON(source_id) * FROM finance_pos_snapshots
		WHERE outlet_id=$1 AND entity_kind='transaction' AND voided_at=$2
		ORDER BY source_id,id DESC) s`, outletID, at.UTC())
	if err != nil {
		return false, err
	}
	defer rows.Close()
	byOrder := map[string]map[string]bool{}
	for rows.Next() {
		var raw []byte
		var localID string
		var state financePOSState
		if err := rows.Scan(&raw, &localID); err != nil {
			return false, err
		}
		if err := json.Unmarshal(raw, &state); err != nil {
			return false, err
		}
		if len(localID) >= 8 && strings.HasPrefix(note, "Void order #"+strings.ToUpper(localID[:8])+" —") {
			if byOrder[localID] == nil {
				byOrder[localID] = map[string]bool{}
			}
			byOrder[localID][state.ID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	rows.Close()
	if len(byOrder) != 1 {
		return false, nil
	}
	var cutoff *time.Time
	// Where the originating shift is available, match the exact prior-shift
	// cash portion rather than the complete sale (which may contain QRIS/DP).
	cutoffs, err := database.DB.Query(`SELECT COALESCE(s.opened_at,s.created_at)
		FROM cloud_cash_movements m JOIN cloud_cashier_shifts s ON s.outlet_id=m.outlet_id
		AND (s.id::text=m.shift_id OR s.local_id=m.shift_id)
		WHERE m.outlet_id=$1 AND m.created_at=$2 AND m.note=$3 AND m.amount=$4`, outletID, at.UTC(), note, amount)
	if err != nil {
		return false, err
	}
	for cutoffs.Next() {
		var value time.Time
		if err := cutoffs.Scan(&value); err != nil {
			cutoffs.Close()
			return false, err
		}
		if cutoff != nil && !cutoff.Equal(value) {
			cutoffs.Close()
			return false, nil
		}
		copy := value
		cutoff = &copy
	}
	err = cutoffs.Err()
	cutoffs.Close()
	if err != nil {
		return false, err
	}
	// The report may use corroborated legacy header gaps or observed payment
	// corrections. Test coverage against the same temporal facts rather than
	// summing raw lines from the last archived payload again.
	entities, err := loadFinancePOSEntities(outletID, nil)
	if err != nil {
		return false, err
	}
	orders, aliases := map[string]*financePOSEntity{}, map[string]*financePOSEntity{}
	for _, entity := range entities {
		if entity.Kind == "order" {
			state := entity.Revisions[0].State
			orders[state.ID], aliases[state.LocalID] = entity, entity
		}
	}
	var cash, refunded float64
	for _, transactions := range byOrder {
		for _, entity := range entities {
			if entity.Kind != "transaction" || !transactions[entity.Revisions[0].State.ID] {
				continue
			}
			financePOSEmitPayments(entity, at.UTC(), func(state financePOSState) ([]financePOSPayment, bool) {
				order := orders[state.OrderID]
				if order == nil && state.OrderID != "" {
					order = aliases[state.OrderID]
				}
				payments, actual, _ := financePOSReceiptEvidence(state, order)
				return payments, actual
			}, func(financePOSState, time.Time) string { return glAccReceivable },
				func(effective time.Time, source, id string, state financePOSState, debit, credit string, amount float64, category, review string) {
					if source == "sale_refund" && credit == glAccCash && effective.Equal(at.UTC()) {
						refunded += amount
					}
					if (source == "sale" || source == "sale_correction") && debit == glAccCash && !effective.After(at) && (cutoff == nil || effective.Before(*cutoff)) {
						cash += amount
					}
				})
		}
	}
	if round2(refunded) < round2(amount) {
		return false, nil
	}
	if cutoff != nil {
		return round2(cash) == round2(amount), nil
	}
	return round2(cash) >= round2(amount), nil
}
