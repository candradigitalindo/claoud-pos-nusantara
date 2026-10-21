package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ErrFinanceAccess = errors.New("akses transaksi keuangan tidak diizinkan")

type FinanceAccess struct {
	ActorID, Actor     string
	Outlets, WorkUnits []string
	Permissions        map[string]bool
}

func (a FinanceAccess) Has(p string) bool { return a.Permissions["*"] || a.Permissions[p] }
func financeInScope(id string, scope []string) bool {
	if scope == nil {
		return true
	}
	for _, s := range scope {
		if s == id && id != "" {
			return true
		}
	}
	return false
}

// Direction is the cash direction, not the accounting group. For example,
// receiving a loan credits a liability; it must never increase revenue.
var FinanceAccounts = []models.FinanceAccount{
	{Code: "1-100", Name: "Kas & Setara Kas", Group: "aset"},
	{Code: "1-200", Name: "Piutang Usaha", Group: "aset"},
	{Code: "1-250", Name: "Piutang Lainnya / Penjualan Aset", Group: "aset", Direction: "in", CashCategory: "other"},
	{Code: "1-300", Name: "Persediaan Bahan & Produk", Group: "aset"},
	{Code: "1-310", Name: "Persediaan dalam Pengiriman", Group: "aset"},
	{Code: "1-320", Name: "Persediaan Karantina / Retur", Group: "aset"},
	{Code: "1-330", Name: "Persediaan dalam Produksi", Group: "aset"},
	{Code: "1-350", Name: "Pengadaan Bahan Belum Diterima", Group: "aset"},
	{Code: "1-400", Name: "Aset Tetap (Peralatan)", Group: "aset"},
	{Code: "1-450", Name: "Projek Berjalan", Group: "aset"},
	{Code: "2-100", Name: "Hutang Usaha", Group: "kewajiban"},
	{Code: "2-150", Name: "Hutang Operasional Lainnya", Group: "kewajiban", Direction: "out", CashCategory: "opex"},
	{Code: "2-200", Name: "Hutang Pajak Restoran", Group: "kewajiban", Direction: "out", CashCategory: "opex"},
	{Code: "2-300", Name: "Uang Muka Pelanggan", Group: "kewajiban"},
	{Code: "2-350", Name: "Uang Muka Menunggu Sinkronisasi POS", Group: "kewajiban"},
	{Code: "2-400", Name: "Pinjaman", Group: "kewajiban", Direction: "both", CashCategory: "financing"},
	{Code: "3-100", Name: "Modal / Saldo Awal", Group: "ekuitas", Direction: "both", CashCategory: "financing"},
	{Code: "3-200", Name: "Alokasi Aset Antar Unit", Group: "ekuitas"},
	{Code: "4-100", Name: "Pendapatan Penjualan", Group: "pendapatan"},
	{Code: "4-200", Name: "Pendapatan Lainnya", Group: "pendapatan", Direction: "in", CashCategory: "other"},
	{Code: "4-210", Name: "Pendapatan Bunga", Group: "pendapatan", Direction: "in", CashCategory: "other"},
	{Code: "4-220", Name: "Laba Pelepasan Aset / Retur", Group: "pendapatan"},
	{Code: "4-230", Name: "Selisih Lebih Persediaan", Group: "pendapatan"},
	{Code: "5-100", Name: "HPP - Pemakaian Bahan / Produk Terjual", Group: "beban"},
	{Code: "5-200", Name: "Beban Jasa & Layanan", Group: "beban", Direction: "out", CashCategory: "service"},
	{Code: "5-300", Name: "Beban Operasional Lainnya", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-310", Name: "Beban Gaji & Upah", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-320", Name: "Beban Listrik, Air & Internet", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-330", Name: "Beban Sewa", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-340", Name: "Beban Transportasi", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-350", Name: "Beban Administrasi Bank", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-360", Name: "Beban Perawatan Aset", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-370", Name: "Beban Promosi", Group: "beban", Direction: "out", CashCategory: "opex"},
	{Code: "5-380", Name: "Beban Persediaan Rusak / Susut", Group: "beban"},
	{Code: "5-390", Name: "Rugi Pelepasan Aset / Retur", Group: "beban"},
	{Code: "5-400", Name: "Beban Penyusutan Aset", Group: "beban"},
	{Code: "6-100", Name: "Beban Pajak Restoran", Group: "beban"},
}

func financeAccount(code string) (models.FinanceAccount, bool) {
	for _, a := range FinanceAccounts {
		if a.Code == code {
			return a, true
		}
	}
	return models.FinanceAccount{}, false
}

func validateFinanceEntry(r *models.FinanceEntryInput, a FinanceAccess) (time.Time, string, error) {
	r.Description = strings.TrimSpace(r.Description)
	r.Counterparty = strings.TrimSpace(r.Counterparty)
	r.Notes = strings.TrimSpace(r.Notes)
	r.Reference = strings.TrimSpace(r.Reference)
	meta, ok := financeAccount(r.AccountCode)
	if !ok || (r.Direction != "in" && r.Direction != "out") || (meta.Direction != r.Direction && meta.Direction != "both") {
		return time.Time{}, "", Invalid("pilih akun sesuai jenis pemasukan/pengeluaran")
	}
	if math.IsNaN(r.Amount) || math.IsInf(r.Amount, 0) || r.Amount <= 0 || r.Amount > 1e12 || round2(r.Amount) != r.Amount {
		return time.Time{}, "", Invalid("nominal harus positif, maksimal 1 triliun dan dua angka desimal")
	}
	if r.Description == "" || r.Counterparty == "" || len(r.Description) > 300 || len(r.Counterparty) > 200 || len(r.Notes) > 10000 || len(r.Reference) > 200 {
		return time.Time{}, "", Invalid("uraian dan penerima/pemberi wajib diisi; periksa panjang catatan")
	}
	if r.OccurredAt == "" {
		return time.Time{}, "", Invalid("tanggal transaksi wajib diisi")
	}
	at, err := parsePaidAt(r.OccurredAt)
	if err != nil {
		return at, "", err
	}
	if r.Period != "" {
		if _, err = time.Parse("2006-01", r.Period); err != nil {
			return at, "", Invalid("periode harus YYYY-MM")
		}
	}
	if r.AccountCode == "5-310" && (r.Period == "" || r.WorkUnitID == "" && r.OutletID == "") {
		return at, "", Invalid("gaji wajib memiliki periode dan outlet/divisi")
	}
	if r.WorkUnitID != "" {
		var oid string
		if err = database.DB.QueryRow(`SELECT COALESCE(outlet_id,'') FROM work_units WHERE id=$1`, r.WorkUnitID).Scan(&oid); err != nil {
			return at, "", Invalid("unit kerja tidak ditemukan")
		}
		oid = strings.TrimSpace(oid)
		if !financeInScope(r.WorkUnitID, a.WorkUnits) {
			return at, "", ErrFinanceAccess
		}
		if r.OutletID != "" && r.OutletID != oid {
			return at, "", Invalid("outlet tidak sesuai unit kerja")
		}
		r.OutletID = oid
	}
	// A central division is only available to all-outlet finance roles; this
	// matches the report scope and prevents an invisible expense for that role.
	if !financeInScope(r.OutletID, a.Outlets) {
		return at, "", ErrFinanceAccess
	}
	if r.OutletID != "" {
		var active bool
		if err = database.DB.QueryRow(`SELECT is_active FROM outlets WHERE id=$1`, r.OutletID).Scan(&active); err != nil || !active {
			return at, "", Invalid("pilih outlet aktif")
		}
	}
	if r.Method != "cash" && r.Method != "transfer" && r.Method != "qris" {
		return at, "", Invalid("pilih metode kas, transfer, atau QRIS")
	}
	bank := ""
	if r.Method == "cash" {
		r.BankAccountID = ""
	} else {
		if err = database.DB.QueryRow(`SELECT bank_name||' '||account_number||' a.n. '||account_holder FROM bank_accounts WHERE id=$1 AND is_active`, r.BankAccountID).Scan(&bank); err != nil {
			return at, "", Invalid("pilih rekening aktif")
		}
	}
	if r.ProofURL != "" {
		if err = ValidateUploadURL(r.ProofURL); err != nil {
			return at, "", err
		}
	}
	return at, bank, nil
}

func financeAudit(tx *sql.Tx, id, action, actor, notes string) error {
	_, err := tx.Exec(`INSERT INTO finance_entry_events(id,entry_id,action,actor,notes,snapshot) SELECT $1,id,$3,$4,$5,to_jsonb(f) FROM finance_entries f WHERE id=$2`, NewULID(), id, action, actor, notes)
	return err
}

func CreateFinanceEntry(r models.FinanceEntryInput, a FinanceAccess) (*models.FinanceEntry, error) {
	if !a.Has("finance.entries.create") {
		return nil, ErrFinanceAccess
	}
	r.RequestKey = strings.TrimSpace(r.RequestKey)
	if len(r.RequestKey) < 12 || len(r.RequestKey) > 100 {
		return nil, Invalid("kunci permintaan tidak valid; buka ulang form")
	}
	at, bank, err := validateFinanceEntry(&r, a)
	if err != nil {
		return nil, err
	}
	canonical := r
	canonical.Version = 0
	canonical.OccurredAt = at.UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	id := NewULID()
	number := "FIN-" + time.Now().In(GetTimezoneLocation()).Format("20060102") + "-" + id[18:]
	res, err := tx.Exec(`INSERT INTO finance_entries(id,number,request_key,direction,account_code,outlet_id,work_unit_id,amount,occurred_at,period,counterparty,description,notes,reference,method,bank_account_id,bank_label,proof_url,created_by,created_actor_id,request_fingerprint)
	VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) ON CONFLICT(request_key) DO NOTHING`, id, number, r.RequestKey, r.Direction, r.AccountCode, nilIfEmpty(r.OutletID), nilIfEmpty(r.WorkUnitID), r.Amount, at, r.Period, r.Counterparty, r.Description, r.Notes, r.Reference, r.Method, nilIfEmpty(r.BankAccountID), bank, r.ProofURL, a.Actor, a.ActorID, fingerprint)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var owner, originalFingerprint string
		if err = tx.QueryRow(`SELECT id,created_actor_id,request_fingerprint FROM finance_entries WHERE request_key=$1`, r.RequestKey).Scan(&id, &owner, &originalFingerprint); err != nil {
			return nil, err
		}
		if owner != a.ActorID || originalFingerprint != fingerprint {
			return nil, Conflict("kunci permintaan sudah digunakan untuk transaksi berbeda; periksa draf sebelumnya")
		}
	} else if err = financeAudit(tx, id, "created", a.Actor, r.Notes); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return GetFinanceEntry(id, a)
}

const financeEntryColumns = `f.id,f.number,f.request_key,f.direction,f.account_code,COALESCE(f.outlet_id,''),COALESCE(f.work_unit_id,''),f.amount,
	f.occurred_at,f.period,f.counterparty,f.description,f.notes,f.reference,f.method,COALESCE(f.bank_account_id,''),f.bank_label,f.proof_url,
	f.status,f.version,f.created_by,f.created_at,f.posted_by,f.posted_at,f.voided_by,f.voided_at,f.void_reason,
	COALESCE(o.name,'Pusat'),COALESCE(w.name,'')`

func scanFinanceEntry(scan func(...interface{}) error) (models.FinanceEntry, error) {
	var f models.FinanceEntry
	var at time.Time
	err := scan(&f.ID, &f.Number, &f.RequestKey, &f.Direction, &f.AccountCode, &f.OutletID, &f.WorkUnitID, &f.Amount, &at, &f.Period, &f.Counterparty, &f.Description, &f.Notes, &f.Reference, &f.Method, &f.BankAccountID, &f.BankLabel, &f.ProofURL, &f.Status, &f.Version, &f.CreatedBy, &f.CreatedAt, &f.PostedBy, &f.PostedAt, &f.VoidedBy, &f.VoidedAt, &f.VoidReason, &f.OutletName, &f.WorkUnitName)
	f.OccurredAt = at.In(GetTimezoneLocation()).Format("2006-01-02T15:04")
	f.History = []models.FinanceEntryEvent{}
	return f, err
}
func GetFinanceEntry(id string, a FinanceAccess) (*models.FinanceEntry, error) {
	f, err := scanFinanceEntry(database.DB.QueryRow(`SELECT `+financeEntryColumns+` FROM finance_entries f LEFT JOIN outlets o ON o.id=f.outlet_id LEFT JOIN work_units w ON w.id=f.work_unit_id WHERE f.id=$1`, id).Scan)
	if err != nil {
		return nil, err
	}
	if !financeInScope(f.OutletID, a.Outlets) {
		return nil, ErrFinanceAccess
	}
	rows, err := database.DB.Query(`SELECT action,actor,notes,created_at FROM finance_entry_events WHERE entry_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e models.FinanceEntryEvent
		if err = rows.Scan(&e.Action, &e.Actor, &e.Notes, &e.At); err != nil {
			return nil, err
		}
		f.History = append(f.History, e)
	}
	return &f, rows.Err()
}

func UpdateFinanceEntry(id string, r models.FinanceEntryInput, a FinanceAccess) (*models.FinanceEntry, error) {
	if !a.Has("finance.entries.create") {
		return nil, ErrFinanceAccess
	}
	old, err := GetFinanceEntry(id, a)
	if err != nil {
		return nil, err
	}
	if old.Status != "draft" || old.Version != r.Version {
		return nil, Conflict("hanya draf terbaru yang bisa diubah")
	}
	at, bank, err := validateFinanceEntry(&r, a)
	if err != nil {
		return nil, err
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE finance_entries SET direction=$2,account_code=$3,outlet_id=$4,work_unit_id=$5,amount=$6,occurred_at=$7,period=$8,counterparty=$9,description=$10,notes=$11,reference=$12,method=$13,bank_account_id=$14,bank_label=$15,proof_url=$16,version=version+1 WHERE id=$1 AND version=$17 AND status='draft'`, id, r.Direction, r.AccountCode, nilIfEmpty(r.OutletID), nilIfEmpty(r.WorkUnitID), r.Amount, at, r.Period, r.Counterparty, r.Description, r.Notes, r.Reference, r.Method, nilIfEmpty(r.BankAccountID), bank, r.ProofURL, r.Version)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, Conflict("draf sudah berubah; muat ulang")
	}
	if err = financeAudit(tx, id, "updated", a.Actor, r.Notes); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return GetFinanceEntry(id, a)
}

func ActFinanceEntry(id, action, reason string, version int, a FinanceAccess) (*models.FinanceEntry, error) {
	if action != "post" && action != "void" {
		return nil, Invalid("aksi tidak dikenal")
	}
	if !a.Has("finance.entries." + action) {
		return nil, ErrFinanceAccess
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var state, oid, proof string
	var current int
	if err = tx.QueryRow(`SELECT status,COALESCE(outlet_id,''),proof_url,version FROM finance_entries WHERE id=$1 FOR UPDATE`, id).Scan(&state, &oid, &proof, &current); err != nil {
		return nil, err
	}
	if !financeInScope(strings.TrimSpace(oid), a.Outlets) {
		return nil, ErrFinanceAccess
	}
	if current != version {
		return nil, Conflict("dokumen berubah; muat ulang sebelum melanjutkan")
	}
	if action == "post" {
		if state != "draft" {
			return nil, Conflict("hanya draf yang dapat dibukukan")
		}
		if err = ValidateUploadURL(proof); err != nil {
			return nil, Invalid("bukti pembayaran/penerimaan wajib diunggah sebelum dibukukan")
		}
		_, err = tx.Exec(`UPDATE finance_entries SET status='posted',posted_by=$2,posted_at=(now() AT TIME ZONE 'UTC'),version=version+1 WHERE id=$1`, id, a.Actor)
	} else {
		reason = strings.TrimSpace(reason)
		if reason == "" || len(reason) > 10000 {
			return nil, Invalid("alasan pembatalan wajib diisi")
		}
		if state == "voided" {
			return nil, Conflict("dokumen sudah dibatalkan")
		}
		_, err = tx.Exec(`UPDATE finance_entries SET status='voided',voided_by=$2,voided_at=(now() AT TIME ZONE 'UTC'),void_reason=$3,version=version+1 WHERE id=$1`, id, a.Actor, reason)
	}
	if err != nil {
		return nil, err
	}
	if err = financeAudit(tx, id, action, a.Actor, reason); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return GetFinanceEntry(id, a)
}

func ListFinanceEntries(from, to, outlet string, a FinanceAccess) ([]models.FinanceEntry, error) {
	q := `SELECT ` + financeEntryColumns + ` FROM finance_entries f LEFT JOIN outlets o ON o.id=f.outlet_id LEFT JOIN work_units w ON w.id=f.work_unit_id WHERE f.occurred_at>=tz_day_start($1::date) AND f.occurred_at<tz_day_start($2::date+1)`
	args := []interface{}{from, to}
	if outlet != "" {
		if !financeInScope(outlet, a.Outlets) {
			return nil, ErrFinanceAccess
		}
		q += ` AND f.outlet_id=$3`
		args = append(args, outlet)
	} else if a.Outlets != nil {
		q += ` AND f.outlet_id=ANY($3::text[])`
		args = append(args, pq.Array(a.Outlets))
	}
	q += ` ORDER BY f.occurred_at DESC,f.id DESC`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []models.FinanceEntry{}
	for rows.Next() {
		f, e := scanFinanceEntry(rows.Scan)
		if e != nil {
			return nil, e
		}
		list = append(list, f)
	}
	return list, rows.Err()
}
