package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

// Siklus work order perawatan:
//
//	dijadwalkan ──mulai──▶ berjalan ──selesai──▶ selesai
//	      └───────────batal──────────┴──▶ batal
//
// Pencatatan mundur (pekerjaan yang sudah terjadi) tetap didukung: dokumen
// langsung lahir berstatus 'selesai', persis seperti form lama.
var maintenanceValidStatus = map[string]bool{
	"dijadwalkan": true, "berjalan": true, "selesai": true, "batal": true,
}

var maintenanceValidType = map[string]bool{
	"rutin": true, "perbaikan": true, "penggantian": true, "inspeksi": true,
}

// Interval perawatan preventif default per kategori (bulan).
//
// Disimpan sebagai DAFTAR BERURUT, bukan map: urutan iterasi map di Go acak,
// sehingga "AC" (3 bulan) dan "Elektronik" (12 bulan) — yang sama-sama cocok
// untuk satu unit AC — akan bergantian menang dan jadwal perawatan berbeda
// tiap kali dihitung. Yang paling spesifik harus diperiksa lebih dulu.
//
// Bisa ditimpa lewat app_settings 'asset_maintenance_intervals' berisi JSON
// {"<pola kategori>": <bulan>}; kunci yang lebih panjang dianggap lebih
// spesifik dan diperiksa lebih dulu.
type maintenanceInterval struct {
	Key    string
	Months int
}

var defaultMaintenanceIntervals = []maintenanceInterval{
	{"mesin kopi", 1},
	{"kopi", 1},
	{"chiller", 6},
	{"freezer", 6},
	{"kulkas", 6},
	{"genset", 6},
	{"kendaraan", 6},
	{"ac", 3},
	{"mesin", 6},
	{"elektronik", 12},
}

// intervalMatches mencocokkan pola dengan teks kategori+nama. Pola pendek
// (≤3 huruf, seperti "ac") dicocokkan sebagai KATA UTUH — tanpa itu "ac" ikut
// cocok pada "rack", "vacuum", dan "machine".
func intervalMatches(hay, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	if len(key) > 3 {
		return strings.Contains(hay, key)
	}
	for _, word := range strings.FieldsFunc(hay, func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	}) {
		if word == key {
			return true
		}
	}
	return false
}

func maintenanceIntervalMonths(category, name string) int {
	// Master kategori adalah sumber utama sejak kategori tidak lagi teks bebas.
	// Daftar bawaan di bawah tinggal jaring pengaman untuk kategori yang belum
	// mengisi intervalnya.
	if _, interval := categoryDefaults(category); interval > 0 {
		return interval
	}
	table := defaultMaintenanceIntervals
	if raw, err := GetSetting("asset_maintenance_intervals"); err == nil && strings.TrimSpace(raw) != "" {
		custom := map[string]int{}
		if json.Unmarshal([]byte(raw), &custom) == nil && len(custom) > 0 {
			table = make([]maintenanceInterval, 0, len(custom))
			for k, v := range custom {
				table = append(table, maintenanceInterval{k, v})
			}
			// Paling spesifik (kunci terpanjang) diperiksa lebih dulu, dan
			// urutannya tetap sama di setiap pemanggilan.
			sort.Slice(table, func(i, j int) bool {
				if len(table[i].Key) != len(table[j].Key) {
					return len(table[i].Key) > len(table[j].Key)
				}
				return table[i].Key < table[j].Key
			})
		}
	}
	hay := strings.ToLower(category + " " + name)
	for _, row := range table {
		if intervalMatches(hay, row.Key) {
			return row.Months
		}
	}
	return 0
}

// appTodayExpr menghasilkan ekspresi SQL untuk "hari ini menurut zona waktu
// APLIKASI", bukan menurut UTC-nya server database.
//
// CURRENT_DATE di Postgres memakai zona sesi (UTC di container ini). Dengan
// WIB = UTC+7, setiap pukul 17.00–24.00 WIB tanggal UTC masih kemarin —
// sehingga perawatan yang jatuh tempo hari ini tampil sebagai "besok" dan yang
// terlambat tampil belum terlambat. Ini kelas kesalahan yang sama dengan yang
// pernah menggeser tanggal laporan penjualan.
func appTodayExpr() string {
	tz := GetTimezoneLocation().String()
	// Nama zona berasal dari app_settings dan sudah lolos time.LoadLocation,
	// tapi ia tetap masuk ke string SQL — jadi disaring sekali lagi di sini.
	for _, r := range tz {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '/' || r == '_' || r == '-' || r == '+'
		if !ok {
			return "CURRENT_DATE"
		}
	}
	// Satu konversi, bukan dua: now() sudah bertipe timestamptz, sehingga
	// "AT TIME ZONE '<zona>'" langsung menghasilkan jam dinding di zona itu.
	// Konversi ganda lewat UTC justru menggeser ke arah sebaliknya.
	return fmt.Sprintf("(now() AT TIME ZONE '%s')::date", tz)
}

func generateWONumber(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("WOM%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "asset_maintenances", "wo_number", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

const maintenanceColsTpl = `
	m.id, COALESCE(m.wo_number, ''), m.asset_id, COALESCE(a.name, ''), COALESCE(a.asset_no, ''),
	COALESCE(a.outlet_id, ''), COALESCE(o.name, ''), COALESCE(m.status, 'selesai'),
	COALESCE(TO_CHAR(m.scheduled_date, 'YYYY-MM-DD'), ''), TO_CHAR(m.maintenance_date, 'YYYY-MM-DD'),
	m.type, m.description, m.cost, m.performed_by, COALESCE(m.vendor_id, ''), COALESCE(v.name, ''),
	COALESCE(m.purchase_request_id, ''), COALESCE(pr.request_number, ''),
	COALESCE(m.downtime_hours, 0), COALESCE(m.attachment_url, ''), m.condition_after,
	COALESCE(TO_CHAR(m.next_due_date, 'YYYY-MM-DD'), ''), COALESCE(m.created_by, ''),
	CASE WHEN COALESCE(m.status,'selesai') = 'dijadwalkan' AND m.scheduled_date IS NOT NULL
	     THEN (m.scheduled_date - %[1]s) ELSE NULL END,
	m.created_at`

const maintenanceJoins = `
	LEFT JOIN assets a  ON a.id = m.asset_id
	LEFT JOIN outlets o ON o.id = a.outlet_id
	LEFT JOIN vendors v ON v.id = m.vendor_id
	LEFT JOIN purchase_requests pr ON pr.id = m.purchase_request_id`

// maintenanceCols menyisipkan ekspresi "hari ini" zona aplikasi ke kolom due.
func maintenanceCols() string {
	return fmt.Sprintf(maintenanceColsTpl, appTodayExpr())
}

func scanMaintenance(sc interface{ Scan(...interface{}) error }) (models.AssetMaintenance, error) {
	var m models.AssetMaintenance
	var due sql.NullInt64
	err := sc.Scan(&m.ID, &m.WONumber, &m.AssetID, &m.AssetName, &m.AssetNo,
		&m.OutletID, &m.OutletName, &m.Status, &m.ScheduledDate, &m.MaintenanceDate,
		&m.Type, &m.Description, &m.Cost, &m.PerformedBy, &m.VendorID, &m.VendorName,
		&m.PurchaseRequestID, &m.PurchaseRequestNumber, &m.DowntimeHours, &m.AttachmentURL,
		&m.ConditionAfter, &m.NextDueDate, &m.CreatedBy, &due, &m.CreatedAt)
	if due.Valid {
		v := int(due.Int64)
		m.DueInDays = &v
	}
	return m, err
}

// MaintenanceFilter menyaring daftar work order lintas aset.
type MaintenanceFilter struct {
	AssetID  string
	OutletID string
	Status   string
	Type     string
	DueScope string // terlambat | minggu | bulan — hanya untuk WO dijadwalkan
	From     string
	To       string
	Search   string
}

func ListMaintenances(f MaintenanceFilter, outletScope []string) ([]models.AssetMaintenance, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	add := func(cond string, val interface{}) {
		conds = append(conds, fmt.Sprintf(cond, idx))
		args = append(args, val)
		idx++
	}
	if f.AssetID != "" {
		add("m.asset_id = $%d", f.AssetID)
	}
	if f.OutletID != "" {
		add("a.outlet_id = $%d", f.OutletID)
	}
	if f.Status != "" {
		add("COALESCE(m.status,'selesai') = $%d", f.Status)
	}
	if f.Type != "" {
		add("m.type = $%d", f.Type)
	}
	if f.From != "" {
		add("COALESCE(m.scheduled_date, m.maintenance_date) >= $%d::date", f.From)
	}
	if f.To != "" {
		add("COALESCE(m.scheduled_date, m.maintenance_date) <= $%d::date", f.To)
	}
	if f.Search != "" {
		conds = append(conds, fmt.Sprintf("(a.name ILIKE $%d OR m.description ILIKE $%d OR m.wo_number ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+f.Search+"%")
		idx++
	}
	switch f.DueScope {
	case "terlambat":
		conds = append(conds, fmt.Sprintf("COALESCE(m.status,'selesai') = 'dijadwalkan' AND m.scheduled_date < %s", appTodayExpr()))
	case "minggu":
		conds = append(conds, fmt.Sprintf("COALESCE(m.status,'selesai') = 'dijadwalkan' AND m.scheduled_date BETWEEN %[1]s AND %[1]s + 7", appTodayExpr()))
	case "bulan":
		conds = append(conds, fmt.Sprintf("COALESCE(m.status,'selesai') = 'dijadwalkan' AND m.scheduled_date BETWEEN %[1]s AND %[1]s + 30", appTodayExpr()))
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("a.outlet_id = ANY($%d::text[])", idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}

	q := fmt.Sprintf(`SELECT %s FROM asset_maintenances m %s WHERE %s
		ORDER BY COALESCE(m.scheduled_date, m.maintenance_date) DESC, m.created_at DESC LIMIT 500`,
		maintenanceCols(), maintenanceJoins, strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetMaintenance, 0)
	for rows.Next() {
		m, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListAssetMaintenances tetap ada untuk halaman detail aset.
func ListAssetMaintenances(assetID string) ([]models.AssetMaintenance, error) {
	return ListMaintenances(MaintenanceFilter{AssetID: assetID}, nil)
}

func GetMaintenance(id string, outletScope []string) (*models.AssetMaintenance, error) {
	conds := "m.id = $1"
	args := []interface{}{id}
	if outletScope != nil {
		conds += " AND a.outlet_id = ANY($2::text[])"
		args = append(args, pq.Array(outletScope))
	}
	q := fmt.Sprintf(`SELECT %s FROM asset_maintenances m %s WHERE %s`, maintenanceCols(), maintenanceJoins, conds)
	m, err := scanMaintenance(database.DB.QueryRow(q, args...))
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// MaintenanceSummary adalah angka untuk kartu ringkasan halaman Perawatan.
type MaintenanceSummary struct {
	Overdue     int     `json:"overdue"`
	DueWeek     int     `json:"due_week"`
	DueMonth    int     `json:"due_month"`
	InProgress  int     `json:"in_progress"`
	CostYear    float64 `json:"cost_year"`
	DoneYear    int     `json:"done_year"`
	Preventive  int     `json:"preventive_year"`
	Corrective  int     `json:"corrective_year"`
}

func GetMaintenanceSummary(outletScope []string) (*MaintenanceSummary, error) {
	scopeCond := ""
	args := []interface{}{}
	if outletScope != nil {
		scopeCond = " AND a.outlet_id = ANY($1::text[])"
		args = append(args, pq.Array(outletScope))
	}
	var s MaintenanceSummary
	today := appTodayExpr()
	err := database.DB.QueryRow(fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE m.status='dijadwalkan' AND m.scheduled_date < %[1]s),
			COUNT(*) FILTER (WHERE m.status='dijadwalkan' AND m.scheduled_date BETWEEN %[1]s AND %[1]s + 7),
			COUNT(*) FILTER (WHERE m.status='dijadwalkan' AND m.scheduled_date BETWEEN %[1]s AND %[1]s + 30),
			COUNT(*) FILTER (WHERE m.status='berjalan'),
			COALESCE(SUM(m.cost) FILTER (WHERE m.status='selesai' AND m.maintenance_date >= date_trunc('year', %[1]s)), 0),
			COUNT(*) FILTER (WHERE m.status='selesai' AND m.maintenance_date >= date_trunc('year', %[1]s)),
			COUNT(*) FILTER (WHERE m.status='selesai' AND m.type IN ('rutin','inspeksi') AND m.maintenance_date >= date_trunc('year', %[1]s)),
			COUNT(*) FILTER (WHERE m.status='selesai' AND m.type IN ('perbaikan','penggantian') AND m.maintenance_date >= date_trunc('year', %[1]s))
		FROM asset_maintenances m
		LEFT JOIN assets a ON a.id = m.asset_id
		WHERE COALESCE(a.is_deleted, false) = false%[2]s`, today, scopeCond), args...).
		Scan(&s.Overdue, &s.DueWeek, &s.DueMonth, &s.InProgress, &s.CostYear, &s.DoneYear, &s.Preventive, &s.Corrective)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ── Pembuatan & siklus work order ───────────────────────────────────────────

func normalizeMaintenanceInput(req *models.AssetMaintenanceRequest) error {
	if strings.TrimSpace(req.Description) == "" {
		return fmt.Errorf("deskripsi perawatan wajib diisi")
	}
	if req.Type == "" {
		req.Type = "rutin"
	}
	if !maintenanceValidType[req.Type] {
		return fmt.Errorf("jenis perawatan '%s' tidak dikenal", req.Type)
	}
	if req.Status == "" {
		req.Status = "selesai" // bentuk lama: mencatat pekerjaan yang sudah terjadi
	}
	if !maintenanceValidStatus[req.Status] {
		return fmt.Errorf("status '%s' tidak dikenal", req.Status)
	}
	if req.Status == "dijadwalkan" && strings.TrimSpace(req.ScheduledDate) == "" {
		return fmt.Errorf("tanggal rencana wajib diisi untuk perawatan terjadwal")
	}
	// 'perbaikan' bukan lagi kondisi fisik — ia status aset.
	if req.ConditionAfter == "perbaikan" {
		req.ConditionAfter = "rusak_ringan"
	}
	if req.ConditionAfter != "" && !assetValidCondition[req.ConditionAfter] {
		return fmt.Errorf("kondisi '%s' tidak dikenal", req.ConditionAfter)
	}
	if req.Cost < 0 || req.DowntimeHours < 0 {
		return fmt.Errorf("biaya dan lama berhenti tidak boleh negatif")
	}
	return nil
}

// CreateMaintenance membuat work order. Dua bentuk yang sah:
//   - status 'dijadwalkan' → rencana kerja, belum menyentuh kondisi aset
//   - status 'selesai'     → pencatatan mundur, langsung menerapkan hasilnya
func CreateMaintenance(assetID string, req models.AssetMaintenanceRequest, actor string, outletScope []string) (*models.AssetMaintenance, error) {
	before, err := GetAsset(assetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}
	if err := normalizeMaintenanceInput(&req); err != nil {
		return nil, err
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	wo, err := generateWONumber(tx)
	if err != nil {
		return nil, err
	}
	// Jadwal berikutnya diisi template kategori bila pengguna tidak mengisinya —
	// inilah yang membuat perawatan preventif tidak berhenti setelah satu siklus.
	nextDue := req.NextDueDate
	if req.Status == "selesai" && strings.TrimSpace(nextDue) == "" &&
		(req.Type == "rutin" || req.Type == "inspeksi") {
		if months := maintenanceIntervalMonths(before.Category, before.Name); months > 0 {
			base := time.Now().In(GetTimezoneLocation())
			if req.MaintenanceDate != "" {
				if d, perr := time.Parse("2006-01-02", req.MaintenanceDate); perr == nil {
					base = d
				}
			}
			nextDue = base.AddDate(0, months, 0).Format("2006-01-02")
		}
	}

	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO asset_maintenances (id, wo_number, asset_id, status, scheduled_date, maintenance_date,
			type, description, cost, performed_by, vendor_id, downtime_hours, attachment_url,
			condition_after, next_due_date, created_by, created_at)
		VALUES ($1,$2,$3,$4, NULLIF($5,'')::date,
			COALESCE(NULLIF($6,'')::date, NULLIF($5,'')::date, CURRENT_DATE),
			$7,$8,$9,$10, NULLIF($11,''), $12,$13,$14, NULLIF($15,'')::date, $16, (now() AT TIME ZONE 'UTC'))`,
		id, wo, assetID, req.Status, req.ScheduledDate, req.MaintenanceDate,
		req.Type, req.Description, req.Cost, req.PerformedBy, req.VendorID,
		req.DowntimeHours, req.AttachmentURL, req.ConditionAfter, nextDue, actor); err != nil {
		return nil, err
	}

	if req.Status == "selesai" {
		if err := applyMaintenanceResult(tx, before, id, wo, req.ConditionAfter, req.Cost, req.Description, actor); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetMaintenance(id, outletScope)
}

// applyMaintenanceResult menerapkan hasil pekerjaan ke asetnya: kondisi fisik
// diperbarui, status kembali normal bila tadinya sedang diperbaiki, dan satu
// baris buku besar ditulis dengan biayanya.
func applyMaintenanceResult(tx *sql.Tx, asset *models.Asset, woID, woNumber, conditionAfter string, cost float64, notes, actor string) error {
	if strings.TrimSpace(conditionAfter) != "" {
		if _, err := tx.Exec(`UPDATE assets SET condition=$1, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$2`,
			conditionAfter, asset.ID); err != nil {
			return err
		}
	}
	// Aset yang tadi dikunci oleh work order dilepas kembali.
	if _, err := tx.Exec(`UPDATE assets SET status='aktif', updated_at=(now() AT TIME ZONE 'UTC')
		WHERE id=$1 AND COALESCE(status,'aktif')='perbaikan'`, asset.ID); err != nil {
		return err
	}
	writeAssetMovement(tx, models.AssetMovement{
		AssetID: asset.ID, Type: "perawatan",
		ConditionBefore: asset.Condition, ConditionAfter: conditionAfter,
		RefType: "maintenance", RefID: woID, RefNumber: woNumber,
		Amount: cost, Notes: notes, Actor: actor,
	})
	return nil
}

// MaintenanceAction menjalankan satu langkah siklus: start | complete | cancel.
func MaintenanceAction(id, action string, req models.AssetMaintenanceCompleteRequest, actor string, outletScope []string) (*models.AssetMaintenance, error) {
	wo, err := GetMaintenance(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("work order tidak ditemukan")
	}
	asset, err := GetAsset(wo.AssetID, outletScope)
	if err != nil {
		return nil, fmt.Errorf("aset tidak ditemukan")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	switch action {
	case "start":
		if wo.Status != "dijadwalkan" {
			return nil, fmt.Errorf("hanya work order terjadwal yang bisa dimulai")
		}
		if asset.Status == "transit" {
			return nil, fmt.Errorf("aset sedang dalam perjalanan mutasi")
		}
		if _, err := tx.Exec(`UPDATE asset_maintenances SET status='berjalan' WHERE id=$1`, id); err != nil {
			return nil, err
		}
		// Aset dikunci selama dikerjakan: tidak bisa dimutasi, terlihat jelas
		// di daftar kenapa barangnya tidak ada di tempat.
		if _, err := tx.Exec(`UPDATE assets SET status='perbaikan', updated_at=(now() AT TIME ZONE 'UTC')
			WHERE id=$1 AND COALESCE(status,'aktif') IN ('aktif','tidak_aktif')`, wo.AssetID); err != nil {
			return nil, err
		}
		writeAssetMovement(tx, models.AssetMovement{
			AssetID: wo.AssetID, Type: "perawatan", ConditionBefore: asset.Condition,
			RefType: "maintenance", RefID: id, RefNumber: wo.WONumber,
			Notes: "Pekerjaan dimulai: " + wo.Description, Actor: actor,
		})

	case "complete":
		if wo.Status != "dijadwalkan" && wo.Status != "berjalan" {
			return nil, fmt.Errorf("work order ini sudah %s", wo.Status)
		}
		cond := req.ConditionAfter
		if cond == "perbaikan" {
			cond = "rusak_ringan"
		}
		if cond != "" && !assetValidCondition[cond] {
			return nil, fmt.Errorf("kondisi '%s' tidak dikenal", cond)
		}
		desc := wo.Description
		if strings.TrimSpace(req.Description) != "" {
			desc = req.Description
		}
		nextDue := req.NextDueDate
		if strings.TrimSpace(nextDue) == "" && (wo.Type == "rutin" || wo.Type == "inspeksi") {
			if months := maintenanceIntervalMonths(asset.Category, asset.Name); months > 0 {
				nextDue = time.Now().In(GetTimezoneLocation()).AddDate(0, months, 0).Format("2006-01-02")
			}
		}
		if _, err := tx.Exec(`
			UPDATE asset_maintenances SET status='selesai',
				maintenance_date = COALESCE(NULLIF($1,'')::date, CURRENT_DATE),
				description=$2, cost=$3, performed_by=COALESCE(NULLIF($4,''), performed_by),
				downtime_hours=$5, condition_after=$6, next_due_date=NULLIF($7,'')::date,
				attachment_url=COALESCE(NULLIF($8,''), attachment_url)
			WHERE id=$9`,
			req.MaintenanceDate, desc, req.Cost, req.PerformedBy, req.DowntimeHours,
			cond, nextDue, req.AttachmentURL, id); err != nil {
			return nil, err
		}
		if err := applyMaintenanceResult(tx, asset, id, wo.WONumber, cond, req.Cost, desc, actor); err != nil {
			return nil, err
		}
		// Siklus berikutnya langsung terjadwal, bukan menunggu diingat orang.
		if strings.TrimSpace(nextDue) != "" {
			nextWO, err := generateWONumber(tx)
			if err != nil {
				return nil, err
			}
			if _, err := tx.Exec(`
				INSERT INTO asset_maintenances (id, wo_number, asset_id, status, scheduled_date,
					maintenance_date, type, description, cost, performed_by, created_by, created_at)
				VALUES ($1,$2,$3,'dijadwalkan',$4::date,$4::date,$5,$6,0,'',$7,(now() AT TIME ZONE 'UTC'))`,
				NewULID(), nextWO, wo.AssetID, nextDue, wo.Type,
				fmt.Sprintf("Perawatan %s terjadwal (lanjutan %s)", wo.Type, wo.WONumber), actor); err != nil {
				return nil, err
			}
		}

	case "cancel":
		if wo.Status == "selesai" {
			return nil, fmt.Errorf("work order yang sudah selesai tidak bisa dibatalkan")
		}
		if _, err := tx.Exec(`UPDATE asset_maintenances SET status='batal' WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if wo.Status == "berjalan" {
			if _, err := tx.Exec(`UPDATE assets SET status='aktif', updated_at=(now() AT TIME ZONE 'UTC')
				WHERE id=$1 AND COALESCE(status,'aktif')='perbaikan'`, wo.AssetID); err != nil {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("aksi '%s' tidak dikenal", action)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetMaintenance(id, outletScope)
}

func DeleteAssetMaintenance(assetID, maintenanceID string, outletScope []string) error {
	if _, err := GetAsset(assetID, outletScope); err != nil {
		return fmt.Errorf("aset tidak ditemukan")
	}
	var status string
	if err := database.DB.QueryRow(`SELECT COALESCE(status,'selesai') FROM asset_maintenances
		WHERE id=$1 AND asset_id=$2`, maintenanceID, assetID).Scan(&status); err != nil {
		return fmt.Errorf("catatan perawatan tidak ditemukan")
	}
	if status == "berjalan" {
		return fmt.Errorf("work order yang sedang berjalan tidak bisa dihapus — batalkan dulu")
	}
	_, err := database.DB.Exec(`DELETE FROM asset_maintenances WHERE id=$1 AND asset_id=$2`, maintenanceID, assetID)
	return err
}

// CreateMaintenancePurchaseRequest menurunkan work order menjadi pengajuan
// Pengadaan Jasa. Biaya perawatan dan biaya pengadaan jadi berasal dari satu
// angka, bukan dua entri yang diketik terpisah di dua modul.
func CreateMaintenancePurchaseRequest(id, requestedBy string, outletScope []string) (*models.PurchaseRequest, error) {
	wo, err := GetMaintenance(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("work order tidak ditemukan")
	}
	if wo.PurchaseRequestID != "" {
		return nil, fmt.Errorf("work order ini sudah tertaut pengajuan %s", wo.PurchaseRequestNumber)
	}
	if wo.Status == "selesai" || wo.Status == "batal" {
		return nil, fmt.Errorf("work order sudah %s", wo.Status)
	}
	name := fmt.Sprintf("Perbaikan %s", wo.AssetName)
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		OutletID:    wo.OutletID,
		RequestType: "jasa",
		RequestedBy: requestedBy,
		Notes:       fmt.Sprintf("Dari work order %s (%s)", wo.WONumber, wo.AssetNo),
		Items: []models.PurchaseRequestItem{{
			Name: name,
			Items: []models.PurchaseSubItem{{
				Name: wo.Description, Qty: 1, Unit: "paket", HpsPrice: wo.Cost,
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	if _, err := database.DB.Exec(`UPDATE asset_maintenances SET purchase_request_id=$1 WHERE id=$2`, pr.ID, id); err != nil {
		return nil, err
	}
	return pr, nil
}

// ── Penjadwal harian ────────────────────────────────────────────────────────

// StartAssetMaintenanceScheduler menerbitkan work order dari jadwal yang sudah
// tersimpan. Sebelum Fase 3, kolom next_due_date terisi tapi tidak pernah
// dibaca — jadwal perawatan praktis mati di dalam database.
func StartAssetMaintenanceScheduler() {
	go func() {
		generateDueMaintenanceWorkOrders()
		lastRunDay := ""
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			loc := GetTimezoneLocation()
			now := time.Now().In(loc)
			day := now.Format("2006-01-02")
			if now.Hour() == 3 && day != lastRunDay {
				lastRunDay = day
				generateDueMaintenanceWorkOrders()
			}
		}
	}()
}

// generateDueMaintenanceWorkOrders membuat WO terjadwal untuk perawatan yang
// jatuh tempo dalam 7 hari ke depan (termasuk yang sudah lewat), selama aset
// itu belum punya WO terbuka. Idempoten: aman dijalankan berkali-kali.
func generateDueMaintenanceWorkOrders() {
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT m.id, m.asset_id, m.type, TO_CHAR(m.next_due_date, 'YYYY-MM-DD'),
		       COALESCE(m.wo_number, ''), COALESCE(a.name, '')
		FROM asset_maintenances m
		JOIN assets a ON a.id = m.asset_id AND a.is_deleted = false
		WHERE m.next_due_date IS NOT NULL
		  AND COALESCE(m.status, 'selesai') = 'selesai'
		  AND m.next_due_date <= %s + 7
		  AND NOT EXISTS (
			SELECT 1 FROM asset_maintenances o
			WHERE o.asset_id = m.asset_id AND COALESCE(o.status,'selesai') IN ('dijadwalkan','berjalan'))
		ORDER BY m.next_due_date`, appTodayExpr()))
	if err != nil {
		log.Printf("[Aset] Gagal membaca jadwal perawatan: %v", err)
		return
	}
	type due struct{ srcID, assetID, woType, dueDate, srcWO, assetName string }
	pending := []due{}
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.srcID, &d.assetID, &d.woType, &d.dueDate, &d.srcWO, &d.assetName); err != nil {
			continue
		}
		pending = append(pending, d)
	}
	rows.Close()
	if len(pending) == 0 {
		return
	}

	created := 0
	dueList := []WAMaintenanceDue{}
	for _, d := range pending {
		tx, err := database.DB.Begin()
		if err != nil {
			continue
		}
		wo, err := generateWONumber(tx)
		if err != nil {
			tx.Rollback()
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO asset_maintenances (id, wo_number, asset_id, status, scheduled_date,
				maintenance_date, type, description, cost, performed_by, created_by, created_at)
			VALUES ($1,$2,$3,'dijadwalkan',$4::date,$4::date,$5,$6,0,'','sistem',(now() AT TIME ZONE 'UTC'))`,
			NewULID(), wo, d.assetID, d.dueDate, d.woType,
			fmt.Sprintf("Perawatan %s jatuh tempo (jadwal dari %s)", d.woType, d.srcWO))
		if err != nil {
			tx.Rollback()
			continue
		}
		if err := tx.Commit(); err == nil {
			created++
			dueList = append(dueList, WAMaintenanceDue{WONumber: wo, AssetName: d.assetName, Type: d.woType, DueDate: d.dueDate})
		}
	}
	if created > 0 {
		log.Printf("[Aset] %d work order perawatan diterbitkan dari jadwal jatuh tempo", created)
		BroadcastSync("asset_maintenance_due", "")
		go NotifyMaintenanceDue(dueList)
	}
}
