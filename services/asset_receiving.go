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

	"github.com/lib/pq"
)

func pqStringArray(v []string) interface{} { return pq.Array(v) }

// Ambang kapitalisasi: barang di atas nilai ini diusulkan dicatat sebagai aset.
// Angka final ditetapkan manajemen lewat app_settings.
const defaultCapitalizationMin = 500000.0

// ExpenseReasonMinLen: baris yang ditandai "habis pakai" SELALU wajib beralasan
// minimal sekian huruf (dihitung per karakter setelah trim — sama dengan
// layar). Dulu alasan hanya wajib di atas ambang nilai dan "." pun diterima,
// sehingga laptop Rp10 jt bisa ditutup sebagai habis pakai tanpa jejak (T-17).
const ExpenseReasonMinLen = 10

// maxReceivedAssetNameLen: panjang kolom assets.name.
const maxReceivedAssetNameLen = 150

func settingFloat(key string, def float64) float64 {
	raw, err := GetSetting(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return def
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 {
		return def
	}
	return v
}

// receivingQuerier dipenuhi *sql.DB maupun *sql.Tx. Draf serah terima dibaca
// di luar transaksi untuk layar, dan DI DALAM transaksi yang mengunci keluarga
// dokumen saat penerimaan dicatat (T-28) — rumusnya satu.
type receivingQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// normItemName menormalkan nama untuk kunci baris dan pencocokan katalog:
// huruf kecil, spasi berlebih dibuang.
func normItemName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// prItemKeyBase: bagian kunci baris sebelum nomor kemunculan.
func prItemKeyBase(entryName, itemName string) string {
	return normItemName(entryName) + "|" + normItemName(itemName)
}

// prItemKey membangun kunci idempotensi satu sub-item: nama entri + nama
// sub-item (dinormalkan) + urutan kemunculan. Dipakai sebagai kunci unik
// bersama purchase_request_id, sehingga klik ganda tidak melahirkan aset kembar.
func prItemKey(entryName, itemName string, occurrence int) string {
	return fmt.Sprintf("%s#%d", prItemKeyBase(entryName, itemName), occurrence)
}

// forEachPRLine memanggil fn untuk tiap sub-item pengajuan beserta kunci
// tersimpannya — SATU-SATUNYA rumus kunci baris (draf, antrean, laporan,
// status penerimaan, sinkron harga). Nomor kemunculan dihitung per kunci yang
// sudah DINORMALKAN: dulu dihitung per nama mentah, sehingga "Kursi" dan
// "kursi " dalam satu grup memperoleh kunci yang sama dan sama-sama lolos
// batas sisa (T-29).
func forEachPRLine(items []models.PurchaseRequestItem, fn func(entry models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error) error {
	seen := map[string]int{}
	for _, entry := range items {
		for _, sub := range entry.Items {
			base := prItemKeyBase(entry.Name, sub.Name)
			key := fmt.Sprintf("%s#%d", base, seen[base])
			seen[base]++
			if err := fn(entry, sub, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// receivingLineID: identitas baris di draf serah terima. Baris milik dokumen
// yang dibuka memakai kuncinya apa adanya; baris pecahan yang dibuka lewat
// induknya diberi awalan id pecahan. Kunci tersimpan selalu huruf kecil
// sedangkan id ULID huruf besar, jadi keduanya tidak pernah tertukar.
func receivingLineID(draftID, sourceID, key string) string {
	if sourceID == draftID {
		return key
	}
	return sourceID + ":" + key
}

// effectiveUnitPrice: harga final bila sudah diisi, selain itu HPS.
func effectiveUnitPrice(sub models.PurchaseSubItem) float64 {
	if sub.FinalPrice > 0 {
		return sub.FinalPrice
	}
	return sub.HpsPrice
}

// parsePRItems membaca kolom items (JSONB) sebuah pengajuan.
func parsePRItems(raw []byte) ([]models.PurchaseRequestItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []models.PurchaseRequestItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("gagal membaca item pengajuan: %w", err)
	}
	return items, nil
}

// receivingSource: satu dokumen yang MEMEGANG baris yang akan diterima.
type receivingSource struct {
	ID, Number, Status, GoodsKind string
	VendorName, InvoiceNumber     string
	Items                         []models.PurchaseRequestItem
}

const receivingSourceCols = `id, request_number, status, COALESCE(goods_kind, ''),
	COALESCE(vendor_name, ''), COALESCE(invoice_number, ''), items`

func scanReceivingSource(sc interface{ Scan(...interface{}) error }) (receivingSource, error) {
	var s receivingSource
	var raw []byte
	if err := sc.Scan(&s.ID, &s.Number, &s.Status, &s.GoodsKind, &s.VendorName, &s.InvoiceNumber, &raw); err != nil {
		return s, err
	}
	s.ID = strings.TrimSpace(s.ID)
	items, err := parsePRItems(raw)
	if err != nil {
		return s, err
	}
	s.Items = items
	return s, nil
}

// receivingSources menentukan dokumen mana yang MEMEGANG item.
//
// Pengajuan yang dipecah per vendor menjadi induk + pecahan: baris pecahan
// diambil dari pecahannya (induk tinggal cangkang untuk baris itu), sisa milik
// induk sendiri dari induknya. Pecahan yang dibatalkan/ditolak tidak ikut —
// barang yang batal dibeli tidak diserahterimakan.
func receivingSources(q receivingQuerier, prID string) ([]receivingSource, error) {
	var splitStatus string
	if err := q.QueryRow(`SELECT COALESCE(split_status, '') FROM purchase_requests WHERE id = $1`, prID).Scan(&splitStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pengajuan tidak ditemukan")
		}
		return nil, err
	}
	self, err := scanReceivingSource(q.QueryRow(`SELECT `+receivingSourceCols+` FROM purchase_requests WHERE id = $1`, prID))
	if err != nil {
		return nil, err
	}
	if splitStatus != "master" {
		return []receivingSource{self}, nil
	}
	rows, err := q.Query(`SELECT `+receivingSourceCols+` FROM purchase_requests
		WHERE parent_id = $1 AND `+prCommittedCond("")+` ORDER BY created_at, id`, prID)
	if err != nil {
		return nil, err
	}
	out := []receivingSource{}
	for rows.Next() {
		s, err := scanReceivingSource(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(self.Items) > 0 {
		out = append(out, self) // induk masih memegang sisa itemnya sendiri
	}
	return out, nil
}

// assetKeyMatchSQL: aset yang lahir dari satu baris pengajuan ($2 = kunci
// baris): kuncinya sama persis (data lama) atau "<kunci>/<nomor>" — tepat
// nomor, tanpa LIKE. Dulu LIKE 'kunci#1%' ikut menghitung aset baris #10–#19,
// dan nama berisi '%' atau '_' merusak pencocokan (T-80).
const assetKeyMatchSQL = `(pr_item_key = $2::text OR (left(pr_item_key, length($2::text) + 1) = $2::text || '/'
	AND substr(pr_item_key, length($2::text) + 2) ~ '^[0-9]+$'))`

// recordedQty menghitung berapa unit dari satu baris pengajuan (dokumen sumber
// srcID, kunci key) yang SUDAH berwujud data: aset, baris GRN, habis pakai,
// atau material projek. Galat database dikembalikan — tidak pernah dianggap
// "belum ada yang dicatat", karena itu membuka pencatatan ganda.
func recordedQty(q receivingQuerier, srcID, key string) (int, error) {
	var v sql.NullFloat64
	err := q.QueryRow(`
		SELECT
		  (SELECT COALESCE(SUM(quantity), 0) FROM assets
		    WHERE purchase_request_id = $1 AND is_deleted = false AND `+assetKeyMatchSQL+`)
		+ (SELECT COALESCE(SUM(gi.qty_dist), 0) FROM goods_receipt_items gi
		    JOIN goods_receipts g ON g.id = gi.receipt_id
		    WHERE g.purchase_request_id = $1 AND gi.pr_item_key = $2::text)
		+ (SELECT COALESCE(SUM(qty), 0) FROM pr_receiving_decisions
		    WHERE purchase_request_id = $1 AND pr_item_key = $2::text AND destination = 'habis')
		+ (SELECT COALESCE(SUM(qty_received), 0) FROM project_materials
		    WHERE purchase_request_id = $1 AND pr_item_key = $2::text)`, srcID, key).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("gagal menghitung barang yang sudah dicatat: %w", err)
	}
	return int(math.Floor(v.Float64 + 1e-6)), nil
}

// docReceiptState: keadaan penerimaan baris milik SATU dokumen, dihitung dari
// catatan yang benar-benar ada — "" (belum ada barang), "partial", atau
// "received" (semua baris lengkap). Keputusan penerimaan apa pun (termasuk
// "stok, tunda ke gudang") sudah menandakan barang datang secara fisik.
func docReceiptState(q receivingQuerier, id string, items []models.PurchaseRequestItem) (string, error) {
	total, complete := 0, 0
	started := false
	err := forEachPRLine(items, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
		total++
		rec, err := recordedQty(q, id, key)
		if err != nil {
			return err
		}
		if rec > 0 {
			started = true
		}
		if rec >= sub.Qty {
			complete++
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if total == 0 {
		return "", nil
	}
	if complete == total {
		return "received", nil
	}
	if !started {
		if err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM pr_receiving_decisions WHERE purchase_request_id = $1)`, id).Scan(&started); err != nil {
			return "", err
		}
	}
	if started {
		return "partial", nil
	}
	return "", nil
}

// BuildReceivingDraft menyusun isi dialog Serah Terima.
func BuildReceivingDraft(prID string, outletScope []string) (*models.ReceivingDraft, error) {
	d, _, err := buildReceivingDraft(database.DB, prID, outletScope)
	if err != nil {
		return nil, err
	}
	// Pengambilan oleh petugas logistik yang masih dalam pengantaran: dialog
	// menawarkannya sebagai pengantar (selain "diantar vendor").
	if d.Pickups, err = openFamilyPickups(prID); err != nil {
		return nil, err
	}
	return d, nil
}

func buildReceivingDraft(q receivingQuerier, prID string, outletScope []string) (*models.ReceivingDraft, []receivingSource, error) {
	var d models.ReceivingDraft
	var outletID, projectID sql.NullString
	var requestType string
	err := q.QueryRow(`
		SELECT pr.id, pr.request_number, pr.status, pr.outlet_id, COALESCE(o.name, ''),
		       pr.project_id, COALESCE(pr.vendor_name, ''), COALESCE(pr.invoice_number, ''),
		       COALESCE(pr.paid_amount, 0), COALESCE(pr.total_final, 0), pr.request_type
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.id = $1`, prID).
		Scan(&d.PurchaseRequestID, &d.RequestNumber, &d.Status, &outletID, &d.OutletName,
			&projectID, &d.VendorName, &d.InvoiceNumber, &d.PaidAmount, &d.TotalFinal, &requestType)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	if err != nil {
		return nil, nil, err
	}
	d.PurchaseRequestID = strings.TrimSpace(d.PurchaseRequestID)
	d.OutletID = strings.TrimSpace(outletID.String)
	d.ProjectID = strings.TrimSpace(projectID.String)
	d.RequestType = requestType
	// Pengajuan pusat (tanpa outlet, mis. MRP Gudang Induk) hanya untuk role
	// bercakupan semua outlet — dulu role per outlet yang tahu id-nya bisa
	// membuka dan menerimanya, padahal detailnya sendiri ditolak (T-43).
	if outletScope != nil && (d.OutletID == "" || !outletInScope(d.OutletID, outletScope)) {
		return nil, nil, fmt.Errorf("pengajuan di luar akses Anda")
	}
	d.CapitalizationMin = settingFloat("asset_capitalization_min", defaultCapitalizationMin)
	d.AlreadyReceived = d.Status == "received"
	// Gudang yang MEMBUTUHKAN barang (untuk ditampilkan) vs gudang yang
	// MENERIMA (bawaan pilihan): barang dapur selalu masuk Gudang Induk dulu,
	// lalu diteruskan lewat Transfer Stok yang berfoto.
	d.TargetWarehouseID, d.TargetWarehouseName, d.TargetWarehouseType = targetWarehouseFor(q, prID)
	d.ReceivingWarehouseID = defaultReceivingWarehouse(q)
	// Barang boleh datang sebelum dibayar (tempo) maupun sesudah (bayar di
	// muka). Yang menghalangi penerimaan hanyalah dokumen yang belum disetujui
	// atau sudah mati — bukan urusan pembayarannya. Jasa tidak punya serah
	// terima barang (diselesaikan lewat tombol Selesai).
	d.CanReceive = IsCommittedPurchaseStatus(d.Status) && requestType != RequestTypeJasa

	sources, err := receivingSources(q, prID)
	if err != nil {
		return nil, nil, err
	}
	for _, src := range sources {
		err := forEachPRLine(src.Items, func(entry models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			price := effectiveUnitPrice(sub)
			recorded, err := recordedQty(q, src.ID, key)
			if err != nil {
				return err
			}
			kind, err := lineKindOf(q, src.ID, requestType, src.GoodsKind, key, sub)
			if err != nil {
				return err
			}
			line := models.ReceivingLine{
				PRItemKey: receivingLineID(d.PurchaseRequestID, src.ID, key), ItemKey: key,
				SourcePRID: src.ID, EntryName: entry.Name, Name: sub.Name,
				Qty: sub.Qty, Unit: sub.Unit, UnitPrice: price,
				Subtotal: price * float64(sub.Qty), Recorded: recorded,
			}
			line.Remaining = line.Qty - line.Recorded
			if line.Remaining < 0 {
				line.Remaining = 0
			}
			// Usulan tujuan mengikuti jenis baris yang DIBEKUKAN saat pengajuan
			// dibuat (lineKindOf), bukan katalog yang berlaku hari ini (T-19).
			//
			// Tim aset hanya mengurus barang perlengkapan, bukan barang dapur:
			// bahan dapur mahal (daging premium, keju impor, saffron) tetap
			// barang gudang berapa pun harganya, jadi jenis baris diperiksa
			// LEBIH DULU daripada ambang harga.
			switch {
			case kind == "dapur":
				line.Destination, line.Kind = "stok", "dapur"
				id, name, ok, err := catalogMatch(q, sub.Name)
				if err != nil {
					return err
				}
				if ok {
					line.SuggestedStockItemID, line.SuggestedStockItemName = id, name
				}
			case price >= d.CapitalizationMin:
				// Barang mahal tetap menjadi aset meski dibeli untuk projek:
				// AC dan kitchen set hasil renovasi adalah aset outlet, bukan
				// material yang habis dikonsumsi.
				line.Destination, line.Kind = "aset", "perlengkapan"
			case d.ProjectID != "":
				// Semen, cat, keramik: habis dikonsumsi projek, tapi tetap
				// harus berwujud data sampai projeknya ditutup (§8.8).
				line.Destination, line.Kind = "material", "projek"
			default:
				line.Destination, line.Kind = "habis", "habis"
			}
			d.Lines = append(d.Lines, line)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	if d.Lines == nil {
		d.Lines = []models.ReceivingLine{}
	}
	return &d, sources, nil
}

// ── Eksekusi penerimaan ─────────────────────────────────────────────────────

// ReceiveAccess: wewenang penerima atas baris serah terima (T-10, T-17).
// Diisi handler dari izin role; keputusan diambil PER BARIS di ReceiveGoods.
type ReceiveAccess struct {
	Submit bool // izin buat pengajuan untuk jenis ini → semua baris
	Asset  bool // assets.create → baris peralatan (aset / habis pakai / material)
	Stock  bool // stockledger.adjust → baris dapur/stok, langsung ke buku stok
}

// Any: memegang salah satu wewenang penerimaan.
func (a ReceiveAccess) Any() bool { return a.Submit || a.Asset || a.Stock }

// canHandle: boleh mencatat baris berjenis kind ("dapur" = meja Gudang Induk).
func (a ReceiveAccess) canHandle(kind string) bool {
	if a.Submit {
		return true
	}
	if kind == "dapur" {
		return a.Stock
	}
	return a.Asset
}

func (a ReceiveAccess) deny(line models.ReceivingLine) error {
	if line.Kind == "dapur" {
		return Invalid("%s: baris dapur/stok dicatat petugas gudang (izin penyesuaian stok) atau pengaju pengadaan jenis ini", line.Name)
	}
	return Invalid("%s: baris peralatan dicatat bagian Aset (izin tambah aset) atau pengaju pengadaan jenis ini", line.Name)
}

// validateReceivingDestination: tujuan yang sah untuk satu baris (T-17, T-80).
//   - pengajuan bahan baku dan baris dapur (katalog stok) → hanya "stok";
//     tanpa hak gudang barisnya ditunda ke antrean Gudang Induk;
//   - baris peralatan → "aset" atau "habis" (+ "material" bila pengajuannya
//     terikat projek). Peralatan tidak pernah masuk buku stok — dulu mesin
//     kopi yang ditandai stok berpindah ke antrean dapur.
func validateReceivingDestination(line models.ReceivingLine, requestType, projectID, dest string) error {
	switch {
	case requestType == RequestTypeBahan:
		// Bahan baku seluruhnya masuk buku stok Gudang Induk (kelas HPP):
		// menjadikannya aset menghitung biayanya dua kali, dan habis pakai
		// melewatkan buku stok sama sekali.
		if dest != "stok" {
			return Invalid("%s: pengajuan bahan baku hanya bisa diterima ke stok gudang", line.Name)
		}
	case line.Kind == "dapur":
		if dest != "stok" {
			return Invalid("%s: barang dapur (katalog stok) hanya bisa diterima ke stok gudang", line.Name)
		}
	default:
		switch dest {
		case "aset", "habis":
		case "material":
			if projectID == "" {
				return Invalid("%s: pengajuan ini tidak terikat projek", line.Name)
			}
		case "stok":
			return Invalid("%s: peralatan tidak dicatat sebagai stok gudang — catat sebagai aset atau habis pakai", line.Name)
		default:
			return Invalid("%s: tujuan '%s' tidak dikenal", line.Name, dest)
		}
	}
	return nil
}

// lockPurchaseFamilyTx mengunci satu keluarga dokumen: induk lebih dulu, lalu
// pecahannya menurut (created_at, id) — urutan yang sama dengan alur status,
// cascade, dan hapus, supaya tidak saling tunggu. Mengembalikan id induk
// keluarga (atau id dokumen itu sendiri bila tidak dipecah).
func lockPurchaseFamilyTx(tx *sql.Tx, id string) (string, error) {
	var parent sql.NullString
	if err := tx.QueryRow(`SELECT parent_id FROM purchase_requests WHERE id = $1`, id).Scan(&parent); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("pengajuan tidak ditemukan")
		}
		return "", err
	}
	root := id
	if p := strings.TrimSpace(parent.String); p != "" {
		root = p
	}
	if _, err := tx.Exec(`SELECT 1 FROM purchase_requests WHERE id = $1 FOR UPDATE`, root); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`SELECT 1 FROM purchase_requests WHERE parent_id = $1 ORDER BY created_at, id FOR UPDATE`, root); err != nil {
		return "", err
	}
	return root, nil
}

func insertReceivingDecision(tx *sql.Tx, srcID, key, dest string, qty int, reason, actor string) error {
	_, err := tx.Exec(`
		INSERT INTO pr_receiving_decisions (id, purchase_request_id, pr_item_key, destination, qty, reason, actor, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,(now() AT TIME ZONE 'UTC'))`,
		NewULID(), srcID, key, dest, qty, reason, actor)
	return err
}

// allocAssetKeys menyiapkan n kunci aset baru untuk satu baris pengajuan:
// "<kunci baris>/<nomor>", melanjutkan nomor terbesar yang pernah dipakai baris
// itu (termasuk aset yang sudah dihapus). Setiap aset hasil penerimaan SELALU
// bersufiks: dulu aset massal dan aset tunggal qty 1 memakai kunci baris apa
// adanya, sehingga penerimaan bertahap kedua bertabrakan dengan indeks unik
// idx_assets_pr_item ("sudah pernah dicatat", T-18).
func allocAssetKeys(tx *sql.Tx, srcID, base string, n int) ([]string, error) {
	rows, err := tx.Query(`
		SELECT pr_item_key FROM assets
		WHERE purchase_request_id = $1 AND left(pr_item_key, length($2::text) + 1) = $2::text || '/'`, srcID, base)
	if err != nil {
		return nil, err
	}
	maxN := 0
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return nil, err
		}
		if len(k) > len(base)+1 {
			if v, err := strconv.Atoi(k[len(base)+1:]); err == nil && v > maxN {
				maxN = v
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s/%d", base, maxN+i+1)
	}
	return keys, nil
}

// refreshFamilyReceiptTx menghitung ulang status penerimaan SETIAP dokumen di
// keluarga (induk + pecahan) dari catatan yang benar-benar ada, lalu status
// dokumennya. Dulu status penerimaan induk disalin mentah ke semua pecahannya,
// sehingga pecahan yang barangnya belum datang ikut tertutup, dan induk tetap
// 'paid' walau semua pecahannya sudah diterima (T-36).
func refreshFamilyReceiptTx(tx *sql.Tx, rootID string, actor Actor) error {
	rows, err := tx.Query(`
		SELECT id, status, COALESCE(split_status, ''), COALESCE(receipt_status, ''), items
		FROM purchase_requests WHERE id = $1 OR parent_id = $1
		ORDER BY created_at, id`, rootID)
	if err != nil {
		return err
	}
	type doc struct {
		id, status, split, receipt string
		items                      []models.PurchaseRequestItem
	}
	docs := []doc{}
	for rows.Next() {
		var d doc
		var raw []byte
		if err := rows.Scan(&d.id, &d.status, &d.split, &d.receipt, &raw); err != nil {
			rows.Close()
			return err
		}
		d.id = strings.TrimSpace(d.id)
		if d.items, err = parsePRItems(raw); err != nil {
			rows.Close()
			return err
		}
		docs = append(docs, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()
	masterID := ""
	for _, d := range docs {
		if d.split == "master" {
			masterID = d.id // induk dihitung dari keluarganya (syncFamilyStatusTx)
			continue
		}
		if !IsCommittedPurchaseStatus(d.status) {
			continue
		}
		state, err := docReceiptState(tx, d.id, d.items)
		if err != nil {
			return err
		}
		if state != d.receipt {
			if state == "received" {
				_, err = tx.Exec(`UPDATE purchase_requests SET receipt_status = 'received', received_by = $2,
					received_at = $3, updated_at = $3 WHERE id = $1`, d.id, actor.Name, now)
			} else {
				_, err = tx.Exec(`UPDATE purchase_requests SET receipt_status = $2, updated_at = $3 WHERE id = $1`,
					d.id, state, now)
			}
			if err != nil {
				return err
			}
		}
		// Status DOKUMEN baru menjadi 'received' bila barangnya lengkap DAN
		// uangnya sudah mulai keluar. Barang yang datang sebelum dibayar tetap
		// berstatus tagihan — kalau tidak, bagian pembayaran akan mengira
		// urusannya sudah selesai. Sisa tagihan 'partial' tetap bisa dilunasi
		// dari 'received' (T-12).
		if state == "received" && (d.status == "paid" || d.status == "partial") {
			if _, err := tx.Exec(`UPDATE purchase_requests SET status = 'received', received_by = $2,
				received_at = $3, updated_at = $3 WHERE id = $1 AND status = $4`, d.id, actor.Name, now, d.status); err != nil {
				return err
			}
		}
	}
	if masterID != "" {
		return syncFamilyStatusTx(tx, masterID, actor)
	}
	return nil
}

// ReceiveGoods menjalankan dialog Serah Terima: membuat aset, memasukkan stok
// ke gudang, dan menutup status pengajuan — semuanya dalam SATU transaksi yang
// mengunci keluarga dokumen lebih dulu.
//
// access menentukan baris mana yang boleh dicatat pemanggil (lihat
// ReceiveAccess). Tanpa hak gudang (access.Stock), baris dapur tidak dibuang
// melainkan ditunda ke antrean gudang: barangnya memang sudah diterima secara
// fisik, jadi status pengajuan tetap boleh maju.
func ReceiveGoods(prID string, req models.ReceiveGoodsRequest, actor Actor, access ReceiveAccess, outletScope []string) (*models.ReceiveGoodsResult, error) {
	if !access.Any() {
		return nil, Invalid("Anda tidak berwenang mencatat penerimaan barang")
	}
	if strings.TrimSpace(req.PhotoURL) == "" {
		return nil, Invalid("foto barang saat diterima wajib diunggah — itu satu-satunya bukti bahwa barangnya benar-benar sampai")
	}
	if err := validateUploadField("foto barang saat diterima", req.PhotoURL); err != nil {
		return nil, err
	}
	// Kiriman tanpa barang ditolak sebelum menyentuh apa pun: dulu tetap
	// menulis receipt_status 'partial' dan mengunci dokumen tanpa satu barang
	// pun diterima (T-45).
	hasQty := false
	for _, in := range req.Lines {
		if in.Qty < 0 {
			return nil, Invalid("jumlah barang diterima tidak boleh negatif")
		}
		if in.Qty > 0 {
			hasQty = true
		}
	}
	if !hasQty {
		return nil, Invalid("isi jumlah barang yang diterima, minimal pada satu baris")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Seluruh keluarga dokumen (induk + semua pecahan) dikunci lebih dulu, baru
	// sisa per baris dihitung — di dalam transaksi yang sama. Dulu sisa dibaca
	// di luar transaksi tanpa kunci: dua penerimaan bersamaan sama-sama
	// membaca sisa 3 dan stok tercatat dua kali (T-28).
	rootID, err := lockPurchaseFamilyTx(tx, prID)
	if err != nil {
		return nil, err
	}
	draft, sources, err := buildReceivingDraft(tx, prID, outletScope)
	if err != nil {
		return nil, err
	}
	if !draft.CanReceive {
		if draft.RequestType == RequestTypeJasa {
			return nil, Invalid("pengajuan jasa diselesaikan lewat tombol Selesai, bukan Serah Terima barang")
		}
		return nil, Invalid("pengajuan berstatus '%s' belum bisa diserahterimakan — setujui dulu pengajuannya", purchaseStatusLabel(draft.Status))
	}

	// Vendor & invoice dicatat per dokumen sumber: baris pecahan membawa
	// vendornya sendiri, bukan vendor induk.
	type sourceInfo struct{ number, vendor, invoice string }
	srcInfo := map[string]sourceInfo{}
	for _, s := range sources {
		info := sourceInfo{number: s.Number, vendor: s.VendorName, invoice: s.InvoiceNumber}
		if info.vendor == "" {
			info.vendor = draft.VendorName
		}
		if info.invoice == "" {
			info.invoice = draft.InvoiceNumber
		}
		srcInfo[s.ID] = info
	}
	byID := make(map[string]int, len(draft.Lines))
	byPair := make(map[[2]string]int, len(draft.Lines))
	for i, l := range draft.Lines {
		byID[l.PRItemKey] = i
		byPair[[2]string{l.SourcePRID, l.ItemKey}] = i
	}
	outletID := draft.OutletID
	if outletID == "" {
		outletID = strings.TrimSpace(req.OutletID)
	}

	res := &models.ReceiveGoodsResult{PurchaseRequestID: prID, AssetIDs: []string{}}

	// Tahap 1 — periksa SEMUA baris sebelum menulis apa pun: identitas baris,
	// wewenang, sisa, tujuan, dan kelengkapan isian.
	type receivePlan struct {
		idx  int
		in   models.ReceiveGoodsLine
		dest string
	}
	plans := make([]receivePlan, 0, len(req.Lines))
	used := map[int]bool{}
	for _, in := range req.Lines {
		if in.Qty == 0 {
			res.SkippedLines++
			continue
		}
		idx, ok := byID[in.PRItemKey]
		if sid := strings.TrimSpace(in.SourcePRID); sid != "" {
			if i, found := byPair[[2]string{sid, in.PRItemKey}]; found {
				idx, ok = i, true
			} else if ok && draft.Lines[idx].SourcePRID != sid {
				ok = false
			}
		}
		if !ok {
			return nil, Invalid("baris '%s' tidak ada di pengajuan ini — muat ulang lalu coba lagi", in.PRItemKey)
		}
		line := draft.Lines[idx]
		// Satu baris satu kali per kiriman: dulu dua baris berkunci sama
		// sama-sama lolos batas sisa (4 dari 2, T-29).
		if used[idx] {
			return nil, Invalid("%s: baris yang sama dikirim lebih dari sekali", line.Name)
		}
		used[idx] = true
		if !access.canHandle(line.Kind) {
			return nil, access.deny(line)
		}
		if in.Qty > line.Remaining {
			return nil, Invalid("%s: jumlah (%d) melebihi sisa yang belum dicatat (%d)", line.Name, in.Qty, line.Remaining)
		}
		dest := strings.ToLower(strings.TrimSpace(in.Destination))
		if err := validateReceivingDestination(line, draft.RequestType, draft.ProjectID, dest); err != nil {
			return nil, err
		}
		in.Reason = strings.TrimSpace(in.Reason)
		switch dest {
		case "habis":
			if utf8.RuneCountInString(in.Reason) < ExpenseReasonMinLen {
				return nil, Invalid("%s: alasan habis pakai wajib diisi, minimal %d huruf", line.Name, ExpenseReasonMinLen)
			}
		case "aset":
			if outletID == "" {
				return nil, Invalid("%s: outlet tujuan wajib dipilih", line.Name)
			}
			if !outletInScope(outletID, outletScope) {
				return nil, Invalid("outlet tujuan di luar akses Anda")
			}
			if utf8.RuneCountInString(strings.TrimSpace(in.AssetName)) > maxReceivedAssetNameLen {
				return nil, Invalid("%s: nama aset maksimal %d huruf", line.Name, maxReceivedAssetNameLen)
			}
		case "stok":
			if access.Stock {
				if in.StockItemID == "" || in.WarehouseID == "" {
					return nil, Invalid("%s: item stok dan gudang wajib dipilih", line.Name)
				}
				// Scope dokumen diperiksa lewat outlet pengaju; gudang tujuannya
				// dipilih bebas di dialog, jadi harus diperiksa sendiri.
				if !WarehouseMutableInScope(in.WarehouseID, outletScope) {
					return nil, Invalid("%s: gudang tujuan di luar akses Anda", line.Name)
				}
			}
		}
		plans = append(plans, receivePlan{idx: idx, in: in, dest: dest})
	}

	// Tahap 2 — tulis. Semua catatan ditautkan ke DOKUMEN SUMBER baris itu
	// (pecahan untuk baris pecahan), termasuk GRN: dulu GRN yang dibuat lewat
	// induk ditautkan ke induk sehingga tidak terhitung untuk baris pecahan —
	// sisanya tetap penuh dan stok bisa dicatat berulang kali (T-5).
	applied := map[int]int{}
	type grnKey struct{ warehouse, source string }
	grnLines := map[grnKey][]models.GoodsReceiptItemReq{}
	grnOrder := []grnKey{}
	for _, p := range plans {
		line := draft.Lines[p.idx]
		in := p.in
		qty := in.Qty
		info := srcInfo[line.SourcePRID]
		// Harga selalu dari dokumen (harga final, atau HPS bila belum ada) —
		// nilai kiriman klien diabaikan (T-38).
		price := line.UnitPrice

		switch p.dest {
		case "aset":
			name := strings.TrimSpace(in.AssetName)
			if name == "" {
				name = line.Name
			}
			category, cerr := resolveAssetCategory(in.Category)
			if cerr != nil {
				return nil, cerr
			}
			if in.UsefulLifeMonths == 0 && category != "" {
				if life, _ := categoryDefaults(category); life > 0 {
					in.UsefulLifeMonths = life
				}
			}
			mode := in.TrackingMode
			if mode != "tunggal" && mode != "massal" {
				mode = "massal"
				if qty == 1 || price >= 5000000 {
					mode = "tunggal"
				}
			}
			// Mode tunggal dengan qty > 1 menghasilkan N baris bernomor sendiri.
			rows, perRow := 1, qty
			if mode == "tunggal" {
				rows, perRow = qty, 1
			}
			keys, err := allocAssetKeys(tx, line.SourcePRID, line.ItemKey, rows)
			if err != nil {
				return nil, err
			}
			for i := 0; i < rows; i++ {
				assetID, err := insertReceivedAsset(tx, receivedAsset{
					OutletID: outletID, SourcePRID: line.SourcePRID, PRItemKey: keys[i],
					Name: name, Category: category, Qty: perRow, Unit: line.Unit,
					TrackingMode: mode, Serial: in.SerialNumber, Brand: in.Brand, Model: in.Model,
					Location: in.Location, Price: price,
					UsefulLife: in.UsefulLifeMonths, Residual: in.ResidualValue,
					Vendor: info.vendor, Invoice: info.invoice,
					ProjectID: draft.ProjectID, PRNumber: info.number,
				}, actor.Name)
				if err != nil {
					return nil, err
				}
				res.AssetIDs = append(res.AssetIDs, assetID)
				res.AssetsCreated++
			}

		case "stok":
			if !access.Stock {
				// Ditunda ke antrean gudang. Keputusan "ini stok" dicatat supaya
				// barisnya menunggu di meja Gudang Induk, tapi TIDAK dihitung
				// selesai: barangnya belum masuk buku stok siapa pun, jadi
				// dokumen harus tetap terbuka.
				res.StockLinesQueued++
				if err := insertReceivingDecision(tx, line.SourcePRID, line.ItemKey, "stok_tunda", qty, in.Reason, actor.Name); err != nil {
					return nil, err
				}
				continue
			}
			// Harga di pengajuan adalah harga per SATUAN BELI (mis. per kg),
			// sedangkan GRN menyimpan harga per SATUAN DASAR (mis. per gram).
			// Tanpa pembagian dist_ratio di sini, gula 5 kg seharga Rp 70.000
			// tercatat sebagai Rp 70.000.000 di buku stok.
			var distRatio float64
			if err := tx.QueryRow(`SELECT COALESCE(NULLIF(dist_ratio, 0), 1) FROM stock_items WHERE id = $1`,
				in.StockItemID).Scan(&distRatio); err != nil {
				return nil, Invalid("%s: item stok tidak ditemukan", line.Name)
			}
			k := grnKey{warehouse: in.WarehouseID, source: line.SourcePRID}
			if _, seen := grnLines[k]; !seen {
				grnOrder = append(grnOrder, k)
			}
			grnLines[k] = append(grnLines[k], models.GoodsReceiptItemReq{
				ItemID: in.StockItemID, QtyDist: float64(qty),
				CostPerBase: price / distRatio, ExpiryDate: in.ExpiryDate,
				PRItemKey: line.ItemKey,
			})

		case "material":
			if _, err := upsertProjectMaterialTx(tx, draft.ProjectID, line.SourcePRID, line.ItemKey,
				line.Name, line.Unit, float64(qty), price, in.Location, actor.Name); err != nil {
				return nil, err
			}
			res.MaterialLines++

		case "habis":
			// Cukup jejak keputusannya (di bawah) — alasan sudah diperiksa.
			res.ExpenseLines++
		}
		// Hanya baris yang benar-benar berwujud data yang dihitung selesai.
		applied[p.idx] += qty

		// Jejak keputusan: siapa memutuskan apa atas baris belanja mana.
		if err := insertReceivingDecision(tx, line.SourcePRID, line.ItemKey, p.dest, qty, in.Reason, actor.Name); err != nil {
			return nil, err
		}
	}

	// Satu GRN per (gudang, dokumen sumber).
	for _, k := range grnOrder {
		info := srcInfo[k.source]
		grnID, err := CreateGoodsReceiptTx(tx, models.GoodsReceiptRequest{
			WarehouseID: k.warehouse, VendorName: info.vendor, PORef: info.invoice,
			PurchaseRequestID: k.source, Notes: req.Notes, Items: grnLines[k],
		}, actor.Name)
		if err != nil {
			return nil, err
		}
		var num string
		if err := tx.QueryRow(`SELECT grn_number FROM goods_receipts WHERE id=$1`, grnID).Scan(&num); err != nil {
			return nil, err
		}
		res.GRNNumbers = append(res.GRNNumbers, num)
	}
	res.GRNNumber = strings.Join(res.GRNNumbers, ", ")

	// Pengadaan barang punya DUA meja serah terima: barang dapur diterima di
	// Gudang Induk, peralatan di bagian Aset. Satu dokumen bisa menyentuh
	// keduanya, jadi statusnya hanya boleh maju ke 'received' setelah SELURUH
	// barisnya selesai — dihitung ulang per dokumen dari catatan yang ada.
	if err := refreshFamilyReceiptTx(tx, rootID, actor); err != nil {
		return nil, err
	}
	outstanding := 0
	for i, l := range draft.Lines {
		if l.Remaining-applied[i] > 0 {
			outstanding++
		}
	}
	res.OutstandingLines = outstanding
	if err := tx.QueryRow(`SELECT status, COALESCE(receipt_status, '') FROM purchase_requests WHERE id = $1`, prID).
		Scan(&res.Status, &res.ReceiptStatus); err != nil {
		return nil, err
	}

	desk := req.Desk
	if desk != "dapur" && desk != "perlengkapan" {
		desk = "perlengkapan"
		for _, p := range plans {
			if p.dest == "stok" {
				desk = "dapur"
			}
		}
	}
	// Pengantar barang: vendor langsung, atau petugas logistik yang
	// pengambilannya dipilih — pengambilan itu ditutup (tidak lagi "dalam
	// pengantaran").
	handoverNote := "Barang diterima — diantar vendor"
	if pid := strings.TrimSpace(req.PickupID); pid != "" {
		pickedBy, err := closePickupTx(tx, prID, pid, actor, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		handoverNote = "Barang diterima — diantar petugas logistik " + pickedBy
	}
	// Dokumen diterima lengkap: pengambilan lain yang masih tercatat dalam
	// pengantaran ikut ditutup (setelah pengambilan yang dipilih di atas).
	if res.ReceiptStatus == "received" {
		if _, err := closeRemainingPickupsTx(tx, prID, actor, time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	SaveHandoverPhoto(tx, "terima", desk, "purchase_request", prID, draft.RequestNumber,
		req.PhotoURL, handoverNote, actor.Name)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	parts := []string{}
	if res.AssetsCreated > 0 {
		parts = append(parts, fmt.Sprintf("%d aset dicatat", res.AssetsCreated))
	}
	if res.GRNNumber != "" {
		parts = append(parts, "stok masuk lewat "+res.GRNNumber)
	}
	if res.MaterialLines > 0 {
		parts = append(parts, fmt.Sprintf("%d material projek dicatat", res.MaterialLines))
	}
	if res.StockLinesQueued > 0 {
		parts = append(parts, fmt.Sprintf("%d baris menunggu penerimaan gudang", res.StockLinesQueued))
	}
	// Dulu baris habis pakai hanya disebut bila tidak ada hal lain yang
	// tercatat — serah terima campuran (aset + habis pakai) cukup berbunyi
	// "1 aset dicatat" seolah baris habis pakainya tidak diproses.
	if res.ExpenseLines > 0 && len(parts) > 0 {
		parts = append(parts, fmt.Sprintf("%d baris dicatat sebagai habis pakai", res.ExpenseLines))
	}
	if len(parts) == 0 {
		parts = append(parts, "barang dicatat sebagai habis pakai")
	}
	if outstanding > 0 {
		parts = append(parts, fmt.Sprintf("%d baris belum diterima — dokumen tetap terbuka", outstanding))
	} else if res.Status != "received" {
		parts = append(parts, "barang lengkap diterima, menunggu pembayaran")
	}
	res.Message = strings.Join(parts, " · ")
	// Pengaju dan pembelian tahu barangnya sudah sampai (sebagian/lengkap).
	if pr, err := GetPurchaseRequest(prID); err == nil {
		go NotifyPurchaseEvent(pr, "received", actor.Name, res.Message, 0)
	}
	return res, nil
}

type receivedAsset struct {
	OutletID, SourcePRID, PRItemKey      string
	Name, Category, Unit, TrackingMode   string
	Serial, Brand, Model, Location       string
	Vendor, Invoice, ProjectID, PRNumber string
	Qty, UsefulLife                      int
	Price, Residual                      float64
}

func insertReceivedAsset(tx *sql.Tx, a receivedAsset, actor string) (string, error) {
	assetNo, err := generateAssetNo(tx, a.OutletID)
	if err != nil {
		return "", err
	}
	notes := []string{}
	if a.Vendor != "" {
		notes = append(notes, "Vendor: "+a.Vendor)
	}
	if a.Invoice != "" {
		notes = append(notes, "Invoice: "+a.Invoice)
	}
	notes = append(notes, "Pengadaan: "+a.PRNumber)
	id := NewULID()
	// Tanggal perolehan = tanggal barang tiba (hari ini, zona aplikasi), bukan
	// tanggal pengajuan dibuat: dari situlah penyusutan mulai berjalan.
	_, err = tx.Exec(`
		INSERT INTO assets (id, asset_no, outlet_id, code, name, category, quantity, unit,
			tracking_mode, serial_number, brand, model, condition, status, location,
			acquisition_src, purchase_request_id, pr_item_key, purchase_date, purchase_price,
			useful_life_months, residual_value, notes, is_deleted, created_at, updated_at)
		VALUES ($1,$2,$3,'',$4,$5,$6,$7,$8,$9,$10,$11,'baik','aktif',$12,'pengadaan',$13,$14,
			(now() AT TIME ZONE '`+GetTimezoneLocation().String()+`')::date,
			$15,$16,$17,$18,false,(now() AT TIME ZONE 'UTC'),(now() AT TIME ZONE 'UTC'))`,
		id, assetNo, a.OutletID, a.Name, a.Category, a.Qty, a.Unit,
		a.TrackingMode, a.Serial, a.Brand, a.Model, a.Location,
		a.SourcePRID, a.PRItemKey, a.Price, a.UsefulLife, a.Residual, strings.Join(notes, " · "))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return "", fmt.Errorf("%s sudah pernah dicatat sebagai aset dari pengajuan ini", a.Name)
		}
		return "", err
	}
	writeAssetMovement(tx, models.AssetMovement{
		AssetID: id, Type: "penerimaan", Qty: a.Qty, ToOutletID: a.OutletID, ToLocation: a.Location,
		ConditionAfter: "baik", RefType: "purchase_request", RefID: a.SourcePRID, RefNumber: a.PRNumber,
		Amount: a.Price * float64(a.Qty),
		Notes:  "Diterima dari pengadaan " + a.PRNumber, Actor: actor,
	})
	return id, nil
}

// syncReceivedLinePricesTx: pembelian tempo — barang sudah diterima dengan
// nilai HPS (atau harga final lama) sebelum harga final diisi. Begitu harga
// final per baris diubah, nilai aset dan material projek yang lahir dari baris
// itu ikut disesuaikan, selama nilainya masih sama dengan harga yang dipakai
// saat diterima (koreksi manual di register aset tidak ditimpa). Buku stok
// (GRN, biaya rata-rata) sengaja TIDAK dinilai ulang: barangnya mungkin sudah
// terpakai dengan biaya lama (T-13).
func syncReceivedLinePricesTx(tx *sql.Tx, docID string, before, after []models.PurchaseRequestItem) error {
	old := map[string]float64{}
	forEachPRLine(before, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
		old[key] = effectiveUnitPrice(sub)
		return nil
	})
	return forEachPRLine(after, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
		prev, ok := old[key]
		next := effectiveUnitPrice(sub)
		if !ok || math.Round(prev*100) == math.Round(next*100) {
			return nil
		}
		if _, err := tx.Exec(`
			UPDATE assets SET purchase_price = round($3::numeric, 2), updated_at = (now() AT TIME ZONE 'UTC')
			WHERE purchase_request_id = $1 AND is_deleted = false AND acquisition_src = 'pengadaan'
			  AND `+assetKeyMatchSQL+` AND purchase_price = round($4::numeric, 2)`,
			docID, key, next, prev); err != nil {
			return fmt.Errorf("gagal menyesuaikan nilai aset: %w", err)
		}
		if _, err := tx.Exec(`
			UPDATE project_materials SET unit_cost = round($3::numeric, 2), updated_at = (now() AT TIME ZONE 'UTC')
			WHERE purchase_request_id = $1 AND pr_item_key = $2::text AND unit_cost = round($4::numeric, 2)`,
			docID, key, next, prev); err != nil {
			return fmt.Errorf("gagal menyesuaikan nilai material projek: %w", err)
		}
		return nil
	})
}

// ── Rekonsiliasi ────────────────────────────────────────────────────────────

// IncompleteReceipt adalah satu baris pengadaan yang sudah diterima tapi belum
// seluruhnya berwujud data.
type IncompleteReceipt struct {
	PurchaseRequestID string  `json:"purchase_request_id"`
	RequestNumber     string  `json:"request_number"`
	OutletName        string  `json:"outlet_name"`
	VendorName        string  `json:"vendor_name"`
	ReceivedAt        string  `json:"received_at"`
	EntryName         string  `json:"entry_name"`
	Name              string  `json:"name"`
	Unit              string  `json:"unit"`
	Qty               int     `json:"qty"`
	Recorded          int     `json:"recorded"`
	Missing           int     `json:"missing"`
	UnitPrice         float64 `json:"unit_price"`
	MissingValue      float64 `json:"missing_value"`
	// Kind: "perlengkapan" (urusan tim aset) atau "dapur" (barang yang ada di
	// katalog stok — urusan gudang).
	Kind string `json:"kind"`
}

// prLineRow: satu pengajuan beserta data yang dibutuhkan rumus baris.
type prLineRow struct {
	id, number, outlet, vendor, extra, requestType, goodsKind string
	items                                                     []models.PurchaseRequestItem
}

// ListIncompleteReceipts mencari baris pengajuan berstatus 'received' yang
// jumlahnya masih kurang dari yang dibeli.
//
// Selama ada baris di sini, ada barang yang sudah dibayar tapi belum jelas
// keberadaannya — inilah alat rekonsiliasi utama modul.
//
// kind menyaring tanggung jawab: "perlengkapan" (bawaan modul aset) menyingkirkan
// barang dapur, karena bahan dapur bukan urusan tim aset. Kosongkan untuk
// melihat semuanya.
func ListIncompleteReceipts(outletScope []string, kind string) ([]IncompleteReceipt, error) {
	q := `
		SELECT pr.id, pr.request_number, COALESCE(o.name, ''), COALESCE(pr.vendor_name, ''),
		       COALESCE(TO_CHAR(pr.received_at, 'YYYY-MM-DD'), ''), pr.items,
		       pr.request_type, COALESCE(pr.goods_kind, '')
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.status = 'received' AND pr.request_type IN ('barang','bahan')
		  AND COALESCE(jsonb_array_length(pr.items), 0) > 0`
	args := []interface{}{}
	if outletScope != nil {
		q += ` AND pr.outlet_id = ANY($1::text[])`
		args = append(args, pqStringArray(outletScope))
	}
	q += ` ORDER BY pr.received_at DESC NULLS LAST LIMIT 300`

	docs, err := loadPRLineRows(q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]IncompleteReceipt, 0)
	for _, d := range docs {
		err := forEachPRLine(d.items, func(entry models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			recorded, err := recordedQty(database.DB, d.id, key)
			if err != nil {
				return err
			}
			missing := sub.Qty - recorded
			if missing <= 0 {
				return nil
			}
			lineKind, err := lineKindOf(database.DB, d.id, d.requestType, d.goodsKind, key, sub)
			if err != nil {
				return err
			}
			if kind != "" && lineKind != kind {
				return nil
			}
			price := effectiveUnitPrice(sub)
			out = append(out, IncompleteReceipt{
				PurchaseRequestID: d.id, RequestNumber: d.number, OutletName: d.outlet,
				VendorName: d.vendor, ReceivedAt: d.extra, EntryName: entry.Name,
				Name: sub.Name, Unit: sub.Unit, Qty: sub.Qty, Recorded: recorded,
				Missing: missing, UnitPrice: price, MissingValue: price * float64(missing),
				Kind: lineKind,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// loadPRLineRows membaca pengajuan (kolom: id, nomor, outlet, vendor, extra,
// items, request_type, goods_kind) seluruhnya dulu, baru dihitung per baris —
// supaya koneksi tidak tertahan oleh kursor yang masih terbuka.
func loadPRLineRows(q string, args ...interface{}) ([]prLineRow, error) {
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []prLineRow{}
	for rows.Next() {
		var d prLineRow
		var raw []byte
		if err := rows.Scan(&d.id, &d.number, &d.outlet, &d.vendor, &d.extra, &raw, &d.requestType, &d.goodsKind); err != nil {
			return nil, err
		}
		d.id = strings.TrimSpace(d.id)
		items, err := parsePRItems(raw)
		if err != nil {
			continue
		}
		d.items = items
		out = append(out, d)
	}
	return out, rows.Err()
}

// ── Antrean serah terima per meja ───────────────────────────────────────────

// ReceivingQueueRow adalah satu pengajuan yang masih menunggu diterima di meja
// tertentu: Gudang Induk untuk barang dapur, bagian Aset untuk peralatan.
type ReceivingQueueRow struct {
	PurchaseRequestID string  `json:"purchase_request_id"`
	RequestNumber     string  `json:"request_number"`
	Status            string  `json:"status"`
	OutletName        string  `json:"outlet_name"`
	VendorName        string  `json:"vendor_name"`
	PaidAt            string  `json:"paid_at"`
	Lines             int     `json:"lines"`
	Units             int     `json:"units"`
	Value             float64 `json:"value"`
	// Keadaan pembayaran ikut ditampilkan: menerima barang yang belum dibayar
	// itu sah, tapi petugas perlu tahu bahwa ia sedang menerima barang tempo.
	PaidAmount    float64 `json:"paid_amount"`
	TotalFinal    float64 `json:"total_final"`
	ReceiptStatus string  `json:"receipt_status"`
	// Gudang yang membutuhkan barangnya (gudang run MRP, atau gudang outlet
	// pengaju), supaya penerima tahu ke mana barang ini diteruskan.
	TargetWarehouseName string `json:"target_warehouse_name,omitempty"`
	TargetWarehouseType string `json:"target_warehouse_type,omitempty"`
}

// ListReceivingQueue mencari pengajuan yang sudah disetujui tapi barisnya belum
// seluruhnya diterima di meja `kind`.
//
// Dokumen yang sudah 'received' ikut ditampilkan bila masih menyisakan baris:
// itu terjadi ketika satu meja sudah selesai dan meja lain belum.
//
// allowedTypes membatasi jenis pengajuan (izin per jenis, lihat
// ViewableRequestTypes). nil = tanpa batas — meja Aset/Gudang melihat semua.
func ListReceivingQueue(kind string, outletScope []string, allowedTypes []string) ([]ReceivingQueueRow, error) {
	q := `
		SELECT pr.id, pr.request_number, pr.status, COALESCE(o.name, ''), COALESCE(pr.vendor_name, ''),
		       COALESCE(TO_CHAR(pr.paid_at, 'YYYY-MM-DD'), ''), COALESCE(pr.paid_amount, 0),
		       COALESCE(pr.total_final, 0), COALESCE(pr.receipt_status, ''), pr.items,
		       pr.request_type, COALESCE(pr.goods_kind, '')
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.request_type IN ('barang','bahan')
		  AND ` + prCommittedCond("pr") + `
		  AND COALESCE(jsonb_array_length(pr.items), 0) > 0`
	args := []interface{}{}
	// Pemilahan meja HANYA per baris (lineKindOf, yang membaca jenis baris
	// yang dibekukan), bukan saringan SQL terpisah — supaya antrean, dialog,
	// dan laporan tidak pernah saling bertolak belakang.
	if outletScope != nil {
		args = append(args, pqStringArray(outletScope))
		q += fmt.Sprintf(` AND pr.outlet_id = ANY($%d::text[])`, len(args))
	}
	if allowedTypes != nil {
		args = append(args, pqStringArray(allowedTypes))
		q += fmt.Sprintf(` AND pr.request_type = ANY($%d::text[])`, len(args))
	}
	q += ` ORDER BY pr.paid_at DESC NULLS LAST LIMIT 200`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	type queued struct {
		row                    ReceivingQueueRow
		requestType, goodsKind string
		items                  []models.PurchaseRequestItem
	}
	list := []queued{}
	for rows.Next() {
		var r queued
		var raw []byte
		if err := rows.Scan(&r.row.PurchaseRequestID, &r.row.RequestNumber, &r.row.Status, &r.row.OutletName,
			&r.row.VendorName, &r.row.PaidAt, &r.row.PaidAmount, &r.row.TotalFinal, &r.row.ReceiptStatus, &raw,
			&r.requestType, &r.goodsKind); err != nil {
			rows.Close()
			return nil, err
		}
		r.row.PurchaseRequestID = strings.TrimSpace(r.row.PurchaseRequestID)
		items, err := parsePRItems(raw)
		if err != nil {
			continue
		}
		r.items = items
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ReceivingQueueRow, 0)
	for _, r := range list {
		row := r.row
		err := forEachPRLine(r.items, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			recorded, err := recordedQty(database.DB, row.PurchaseRequestID, key)
			if err != nil {
				return err
			}
			remaining := sub.Qty - recorded
			if remaining <= 0 {
				return nil
			}
			if kind != "" {
				lk, err := lineKindOf(database.DB, row.PurchaseRequestID, r.requestType, r.goodsKind, key, sub)
				if err != nil {
					return err
				}
				if lk != kind {
					return nil
				}
			}
			row.Lines++
			row.Units += remaining
			row.Value += effectiveUnitPrice(sub) * float64(remaining)
			return nil
		})
		if err != nil {
			return nil, err
		}
		if row.Lines > 0 {
			_, row.TargetWarehouseName, row.TargetWarehouseType = targetWarehouseFor(database.DB, row.PurchaseRequestID)
			out = append(out, row)
		}
	}
	return out, nil
}

// stockQueuedFor: seorang petugas tanpa hak gudang sudah memutuskan baris ini
// adalah STOK dan menundanya ke antrean gudang (destination 'stok_tunda').
// Keputusan itu memindahkan baris ke meja Gudang Induk apa pun nama barangnya.
func stockQueuedFor(q receivingQuerier, prID, key string) (bool, error) {
	var exists bool
	err := q.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM pr_receiving_decisions
			WHERE purchase_request_id = $1 AND pr_item_key = $2 AND destination = 'stok_tunda')`,
		prID, key).Scan(&exists)
	return exists, err
}

// catalogMatch mencari item katalog stok AKTIF yang namanya sama dengan name —
// huruf besar/kecil dan spasi berlebih diabaikan (dulu " Gula" tidak cocok
// dengan "Gula", T-19).
func catalogMatch(q receivingQuerier, name string) (id, matched string, ok bool, err error) {
	n := normItemName(name)
	if n == "" {
		return "", "", false, nil
	}
	err = q.QueryRow(`
		SELECT id, name FROM stock_items
		WHERE is_active = true AND btrim(regexp_replace(lower(name), '\s+', ' ', 'g')) = $1
		ORDER BY created_at, id LIMIT 1`, n).Scan(&id, &matched)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return strings.TrimSpace(id), matched, true, nil
}

// catalogKind: "dapur" bila nama ada di katalog stok aktif (Gudang Induk),
// selain itu "perlengkapan" (bagian Aset). Galat database dikembalikan — tidak
// pernah dianggap perlengkapan.
func catalogKind(q receivingQuerier, name string) (string, error) {
	_, _, ok, err := catalogMatch(q, name)
	if err != nil {
		return "", fmt.Errorf("gagal mencocokkan katalog stok: %w", err)
	}
	if ok {
		return "dapur", nil
	}
	return "perlengkapan", nil
}

// lineKindOf: meja yang menerima satu baris — "dapur" (Gudang Induk → stok)
// atau "perlengkapan" (bagian Aset → aset / habis pakai / material projek).
// Satu-satunya rumus meja yang dipakai draf, antrean, laporan, serta
// pemeriksaan wewenang dan tujuan baris. Urutan sumber kebenaran (T-19):
//  1. pengajuan bahan baku → selalu dapur, apa pun nama barisnya;
//  2. keputusan "stok, tunda ke gudang" yang sudah tercatat → dapur;
//  3. jenis baris yang dibekukan saat pengajuan dibuat/diubah (goods_kind
//     per baris);
//  4. jenis dokumen yang dibekukan saat dibuat (purchase_requests.goods_kind)
//     — pengajuan lama sebelum jenis per baris ada;
//  5. cadangan terakhir untuk data lama tanpa nilai: cocokkan ke katalog.
func lineKindOf(q receivingQuerier, prID, requestType, docKind, key string, sub models.PurchaseSubItem) (string, error) {
	if requestType == RequestTypeBahan {
		return "dapur", nil
	}
	queued, err := stockQueuedFor(q, prID, key)
	if err != nil {
		return "", err
	}
	if queued {
		return "dapur", nil
	}
	for _, k := range []string{sub.GoodsKind, docKind} {
		if k == "dapur" || k == "perlengkapan" {
			return k, nil
		}
	}
	return catalogKind(q, sub.Name)
}

// targetWarehouseFor menebak gudang yang MEMBUTUHKAN barang pengajuan ini:
// gudang pada run MRP bila pengajuan lahir dari MRP, atau gudang outlet
// pengaju. Tidak disimpan sebagai kolom — cukup diturunkan dari tautan yang
// sudah ada (mrp_run_id, outlet_id).
func targetWarehouseFor(q receivingQuerier, prID string) (id, name, typ string) {
	var wid, wname, wtype sql.NullString
	q.QueryRow(`
		SELECT w.id, w.name, w.type
		FROM purchase_requests pr
		LEFT JOIN mrp_runs mr ON mr.id = pr.mrp_run_id
		LEFT JOIN warehouses w ON w.id = COALESCE(mr.warehouse_id,
			(SELECT ow.id FROM warehouses ow WHERE ow.outlet_id = pr.outlet_id AND ow.type = 'outlet' AND ow.is_active = true LIMIT 1))
		WHERE pr.id = $1`, prID).Scan(&wid, &wname, &wtype)
	return strings.TrimSpace(wid.String), wname.String, wtype.String
}

// defaultReceivingWarehouse: gudang induk aktif pertama — tempat bawaan
// barang dapur diterima sebelum diteruskan ke outlet.
func defaultReceivingWarehouse(q receivingQuerier) string {
	var id sql.NullString
	q.QueryRow(`SELECT id FROM warehouses WHERE type = 'central' AND is_active = true ORDER BY created_at LIMIT 1`).Scan(&id)
	return strings.TrimSpace(id.String)
}

// ── Pemisahan jenis belanja sejak pengajuan ─────────────────────────────────

// stampPurchaseItemKinds MEMBEKUKAN jenis tiap baris (goods_kind per baris) dan
// mengembalikan jenis dokumennya. Dipanggil saat pengajuan dibuat dan setiap
// kali isinya diubah selama pending; sesudah itu jenis baris tidak pernah
// dihitung ulang dari katalog (T-19):
//   - bahan  → semua baris "dapur";
//   - barang → tiap baris dicocokkan ke katalog stok saat ini ("dapur" bila
//     ada, selain itu "perlengkapan"); dokumen campuran ditolak (lihat
//     ClassifyPurchaseItems);
//   - jasa   → tanpa jenis (tidak ada serah terima barang).
//
// Nilai kiriman klien selalu ditimpa. Galat database dikembalikan, tidak
// pernah dianggap perlengkapan.
func stampPurchaseItemKinds(items []models.PurchaseRequestItem, requestType string) (string, error) {
	switch requestType {
	case RequestTypeBahan:
		for i := range items {
			for j := range items[i].Items {
				items[i].Items[j].GoodsKind = "dapur"
			}
		}
		return "dapur", nil
	case RequestTypeBarang:
		dapur := []string{}
		perlengkapan := []string{}
		for i := range items {
			for j := range items[i].Items {
				sub := &items[i].Items[j]
				sub.GoodsKind = ""
				if strings.TrimSpace(sub.Name) == "" {
					continue
				}
				kind, err := catalogKind(database.DB, sub.Name)
				if err != nil {
					return "", err
				}
				sub.GoodsKind = kind
				if kind == "dapur" {
					dapur = append(dapur, sub.Name)
				} else {
					perlengkapan = append(perlengkapan, sub.Name)
				}
			}
		}
		switch {
		case len(dapur) > 0 && len(perlengkapan) > 0:
			return "", Invalid(
				"satu pengajuan tidak boleh mencampur barang dapur dan peralatan — "+
					"serah terimanya di tempat berbeda. Barang dapur: %s. Peralatan: %s. "+
					"Pisahkan menjadi dua pengajuan",
				strings.Join(trimList(dapur, 3), ", "), strings.Join(trimList(perlengkapan, 3), ", "))
		case len(dapur) > 0:
			return "dapur", nil
		case len(perlengkapan) > 0:
			return "perlengkapan", nil
		}
		return "", nil
	default:
		for i := range items {
			for j := range items[i].Items {
				items[i].Items[j].GoodsKind = ""
			}
		}
		return "", nil
	}
}

// ClassifyPurchaseItems menentukan jenis belanja sebuah pengajuan barang, dan
// menolak yang mencampur. items tidak diubah.
//
// Satu pengajuan tidak boleh memuat barang dapur dan peralatan sekaligus, karena
// serah terimanya berada di dua tempat berbeda: barang dapur diterima di Gudang
// Induk, peralatan di bagian Aset. Dokumen campuran memaksa satu berkas
// diantarkan ke dua meja, dan itulah yang selama ini membuat barang tercecer.
//
// Mengembalikan "dapur" atau "perlengkapan"; error bila tercampur.
func ClassifyPurchaseItems(items []models.PurchaseRequestItem) (string, error) {
	return stampPurchaseItemKinds(clonePurchaseItems(items), RequestTypeBarang)
}

// clonePurchaseItems menyalin items sampai ke sub-itemnya.
func clonePurchaseItems(items []models.PurchaseRequestItem) []models.PurchaseRequestItem {
	out := make([]models.PurchaseRequestItem, len(items))
	for i, g := range items {
		out[i] = g
		out[i].Items = append([]models.PurchaseSubItem(nil), g.Items...)
	}
	return out
}

func trimList(v []string, max int) []string {
	if len(v) <= max {
		return v
	}
	return append(v[:max:max], fmt.Sprintf("dan %d lainnya", len(v)-max))
}
