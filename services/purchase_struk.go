package services

// Struk pembelian & pengambilan barang (keputusan owner 14 Okt 2026).
//
// Struk: satu pengajuan bisa dibeli dalam beberapa transaksi di vendornya —
// mis. struk A tanggal 1 senilai 19 jt dan struk B senilai 20 jt — dan dibayar
// per struk (boleh dicicil, boleh mundur). Struk dicatat bagian pembelian atau
// petugas logistik yang membawanya; pembayaran menunjuk struk yang dilunasi.
//
// Pengambilan: barang bisa diantar vendor atau diambil petugas logistik.
// Pengambilan dicatat (foto + waktu); saat tempat penerimaan menerima barang
// itu, pengambilannya ditutup (received_at) sehingga yang masih "dalam
// pengantaran" selalu terlihat.

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// purchaseDocForStrukTx membaca keadaan dokumen yang akan diberi struk/
// pengambilan, terkunci (FOR UPDATE) supaya tidak balapan dengan pembayaran.
func purchaseDocForStrukTx(tx *sql.Tx, prID string) (status, reqType, receipt string, items []byte, err error) {
	err = tx.QueryRow(`
		SELECT status, request_type, COALESCE(receipt_status, ''), items
		FROM purchase_requests WHERE id = $1 FOR UPDATE`, prID).Scan(&status, &reqType, &receipt, &items)
	if err == sql.ErrNoRows {
		err = Invalid("pengajuan tidak ditemukan")
	}
	return
}

// ListPurchaseStruks: struk sebuah pengajuan beserta yang sudah dibayar,
// terlama dulu (urutan yang sama dengan pembagian pembayaran).
func ListPurchaseStruks(q rowsQuerier, prID string) ([]models.PurchaseStruk, error) {
	rows, err := q.Query(`
		SELECT s.id, s.struk_number, TO_CHAR(s.struk_date, 'YYYY-MM-DD'), s.total,
		       COALESCE((SELECT SUM(ph.amount) FROM payment_histories ph WHERE ph.struk_id = s.id), 0),
		       s.photo_url, s.notes, s.created_by, s.created_at
		FROM purchase_struks s WHERE s.purchase_request_id = $1
		ORDER BY s.struk_date, s.created_at, s.id`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.PurchaseStruk, 0)
	for rows.Next() {
		var s models.PurchaseStruk
		var created time.Time
		if err := rows.Scan(&s.ID, &s.StrukNumber, &s.StrukDate, &s.Total, &s.PaidAmount,
			&s.PhotoURL, &s.Notes, &s.CreatedBy, &created); err != nil {
			return nil, err
		}
		s.ID = strings.TrimSpace(s.ID)
		s.Remaining = math.Max(0, math.Round((s.Total-s.PaidAmount)*100)/100)
		s.CreatedAt = created.UTC().Format(time.RFC3339)
		out = append(out, s)
	}
	return out, rows.Err()
}

// rowsQuerier dipenuhi *sql.DB maupun *sql.Tx.
type rowsQuerier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// AddPurchaseStruk mencatat struk pembelian pada pengajuan yang sudah
// disetujui. Foto struk wajib — struk adalah bukti transaksi di vendor.
func AddPurchaseStruk(prID string, in models.AddPurchaseStrukInput, actor Actor) (*models.PurchaseStruk, error) {
	in.StrukNumber = strings.TrimSpace(in.StrukNumber)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.StrukNumber == "" {
		return nil, Invalid("nomor struk wajib diisi")
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(in.StrukDate))
	if err != nil {
		return nil, Invalid("tanggal struk tidak valid (format YYYY-MM-DD)")
	}
	// Dibandingkan sebagai tanggal kalender zona waktu aplikasi: struk
	// bertanggal besok dulu lolos karena batasnya "sekarang + 1 hari".
	if date.Format("2006-01-02") > time.Now().In(GetTimezoneLocation()).Format("2006-01-02") {
		return nil, Invalid("tanggal struk tidak boleh di masa depan")
	}
	if in.Total <= 0 {
		return nil, Invalid("total struk harus lebih dari 0")
	}
	if err := ValidateUploadURL(in.PhotoURL); err != nil {
		return nil, Invalid("foto struk wajib diunggah")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	status, _, _, items, err := purchaseDocForStrukTx(tx, prID)
	if err != nil {
		return nil, err
	}
	if !IsCommittedPurchaseStatus(status) {
		return nil, Invalid("struk baru bisa dicatat setelah pengajuan disetujui")
	}
	if !hasOwnPurchaseItems(items) {
		return nil, Invalid("seluruh item pengajuan ini sudah dipecah ke vendor — catat struknya pada pecahan vendor masing-masing")
	}
	// Struk adalah dasar pembayaran per struk: tagihan yang sudah lunas tidak
	// bisa lagi membayar struk baru, yang lalu tampil "belum lunas" selamanya.
	var totalFinal, paid float64
	if err := tx.QueryRow(`SELECT COALESCE(total_final, 0), COALESCE(paid_amount, 0) FROM purchase_requests WHERE id = $1`, prID).
		Scan(&totalFinal, &paid); err != nil {
		return nil, err
	}
	if totalFinal > 0 && math.Round(paid*100) >= math.Round(totalFinal*100) {
		return nil, Invalid("tagihan pengajuan ini sudah lunas — struk baru tidak bisa ditambahkan")
	}
	id := NewULID()
	if _, err := tx.Exec(`
		INSERT INTO purchase_struks (id, purchase_request_id, struk_number, struk_date, total, photo_url, notes,
			created_by, created_by_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id, prID, in.StrukNumber, date.Format("2006-01-02"), math.Round(in.Total*100)/100, in.PhotoURL, in.Notes,
		actor.Name, nilIfEmpty(validActorID(actor.ID)), time.Now().UTC()); err != nil {
		if strings.Contains(err.Error(), "uq_purchase_struks_number") {
			return nil, Invalid("struk nomor %s sudah dicatat pada pengajuan ini", in.StrukNumber)
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	list, err := ListPurchaseStruks(database.DB, prID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("struk baru tidak terbaca")
}

// DeletePurchaseStruk menghapus struk yang belum dibayar sama sekali —
// struk yang sudah menjadi dasar pembayaran adalah jejak keuangan.
func DeletePurchaseStruk(prID, strukID string) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, _, _, err := purchaseDocForStrukTx(tx, prID); err != nil {
		return err
	}
	var paid int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM payment_histories WHERE struk_id = $1`, strukID).Scan(&paid); err != nil {
		return err
	}
	if paid > 0 {
		return Invalid("struk ini sudah dibayar — tidak bisa dihapus")
	}
	res, err := tx.Exec(`DELETE FROM purchase_struks WHERE id = $1 AND purchase_request_id = $2`, strukID, prID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Invalid("struk tidak ditemukan pada pengajuan ini")
	}
	return tx.Commit()
}

// strukAllocation: bagian satu pembayaran untuk satu struk (strukID kosong =
// bagian tanpa struk), dalam sen.
type strukAllocation struct {
	strukID string
	cents   float64
}

type strukRemaining struct {
	id, number string
	cents      float64 // sisa belum dibayar, dalam sen
}

// strukRemainingsTx: sisa tiap struk pengajuan, terlama dulu. Dipanggil di
// dalam transaksi pembayaran yang sudah mengunci baris pengajuannya, sehingga
// pembayaran ke struk yang sama selalu berurutan.
func strukRemainingsTx(tx *sql.Tx, prID string) ([]strukRemaining, error) {
	rows, err := tx.Query(`
		SELECT s.id, s.struk_number,
		       ROUND(s.total * 100) - COALESCE((SELECT ROUND(SUM(ph.amount) * 100) FROM payment_histories ph WHERE ph.struk_id = s.id), 0)
		FROM purchase_struks s WHERE s.purchase_request_id = $1
		ORDER BY s.struk_date, s.created_at, s.id`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []strukRemaining
	for rows.Next() {
		var s strukRemaining
		if err := rows.Scan(&s.id, &s.number, &s.cents); err != nil {
			return nil, err
		}
		s.id = strings.TrimSpace(s.id)
		out = append(out, s)
	}
	return out, rows.Err()
}

// strukPayDefaultCents: nominal bawaan "lunasi" untuk pembayaran yang menunjuk
// struk tertentu = sisa struk itu (dibatasi sisa tagihan pemanggil).
func strukPayDefaultCents(list []strukRemaining, strukID string) (float64, error) {
	for _, s := range list {
		if s.id == strukID {
			if s.cents <= 0 {
				return 0, Invalid("struk %s sudah lunas", s.number)
			}
			return s.cents, nil
		}
	}
	return 0, Invalid("struk tidak ditemukan pada pengajuan ini")
}

// allocateStrukPayment membagi pembayaran (dalam sen) ke struk:
//   - strukID diisi → seluruhnya untuk struk itu, tidak boleh melebihi sisanya;
//   - strukID kosong, pengajuan punya struk → dibagi ke struk yang belum lunas,
//     terlama dulu; kelebihannya (bila total struk < harga final) tanpa struk;
//   - tanpa struk → satu bagian tanpa struk.
func allocateStrukPayment(list []strukRemaining, strukID string, payCents float64) ([]strukAllocation, error) {
	strukID = strings.TrimSpace(strukID)
	if strukID != "" {
		for _, s := range list {
			if s.id != strukID {
				continue
			}
			if s.cents <= 0 {
				return nil, Invalid("struk %s sudah lunas", s.number)
			}
			if payCents > s.cents {
				return nil, Invalid("jumlah bayar (%.0f) melebihi sisa struk %s (%.0f)", payCents/100, s.number, s.cents/100)
			}
			return []strukAllocation{{strukID: s.id, cents: payCents}}, nil
		}
		return nil, Invalid("struk tidak ditemukan pada pengajuan ini")
	}
	left := payCents
	var out []strukAllocation
	for _, s := range list {
		if left <= 0 {
			break
		}
		if s.cents <= 0 {
			continue
		}
		take := math.Min(left, s.cents)
		out = append(out, strukAllocation{strukID: s.id, cents: take})
		left -= take
	}
	if left > 0 {
		out = append(out, strukAllocation{cents: left})
	}
	return out, nil
}

// ListPurchasePickups: pengambilan sebuah pengajuan, terbaru dulu.
func ListPurchasePickups(q rowsQuerier, prID string) ([]models.PurchasePickup, error) {
	rows, err := q.Query(`
		SELECT id, picked_by, picked_at, photo_url, notes, received_by, received_at
		FROM purchase_pickups WHERE purchase_request_id = $1
		ORDER BY picked_at DESC, id DESC`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.PurchasePickup, 0)
	for rows.Next() {
		var p models.PurchasePickup
		var picked time.Time
		var received sql.NullTime
		if err := rows.Scan(&p.ID, &p.PickedBy, &picked, &p.PhotoURL, &p.Notes, &p.ReceivedBy, &received); err != nil {
			return nil, err
		}
		p.ID = strings.TrimSpace(p.ID)
		p.PickedAt = picked.UTC().Format(time.RFC3339)
		if received.Valid {
			p.ReceivedAt = received.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddPurchasePickup mencatat pengambilan barang oleh petugas logistik. Hanya
// untuk barang/bahan (jasa tidak diambil) yang sudah disetujui dan belum
// diterima seluruhnya. Fotonya ikut dicadangkan seperti foto serah terima.
func AddPurchasePickup(prID string, in models.AddPurchasePickupInput, actor Actor) (*models.PurchasePickup, error) {
	in.Notes = strings.TrimSpace(in.Notes)
	if err := ValidateUploadURL(in.PhotoURL); err != nil {
		return nil, Invalid("foto barang saat diambil wajib diunggah")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	status, reqType, receipt, _, err := purchaseDocForStrukTx(tx, prID)
	if err != nil {
		return nil, err
	}
	if reqType == RequestTypeJasa {
		return nil, Invalid("pengajuan jasa tidak punya barang untuk diambil")
	}
	if !IsCommittedPurchaseStatus(status) {
		return nil, Invalid("pengambilan baru bisa dicatat setelah pengajuan disetujui")
	}
	if receipt == "received" {
		return nil, Invalid("seluruh barang pengajuan ini sudah diterima")
	}
	var number string
	tx.QueryRow(`SELECT request_number FROM purchase_requests WHERE id = $1`, prID).Scan(&number)
	id := NewULID()
	now := time.Now().UTC()
	if _, err := tx.Exec(`
		INSERT INTO purchase_pickups (id, purchase_request_id, picked_by, picked_by_id, picked_at, photo_url, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $5)`,
		id, prID, actor.Name, nilIfEmpty(validActorID(actor.ID)), now, in.PhotoURL, in.Notes); err != nil {
		return nil, err
	}
	desk := "perlengkapan"
	if reqType == RequestTypeBahan {
		desk = "dapur"
	}
	SaveHandoverPhoto(tx, "ambil", desk, "purchase_request", prID, number, in.PhotoURL,
		"Barang diambil petugas logistik dari vendor", actor.Name)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	list, err := ListPurchasePickups(database.DB, prID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("pengambilan baru tidak terbaca")
}

// openFamilyPickups: pengambilan yang masih dalam pengantaran untuk pengajuan
// ini beserta induk/pecahannya — barang pecahan bisa diterima lewat induknya.
func openFamilyPickups(prID string) ([]models.PurchasePickup, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.picked_by, p.picked_at, p.photo_url, p.notes
		FROM purchase_pickups p
		JOIN purchase_requests pr ON pr.id = p.purchase_request_id
		WHERE p.received_at IS NULL
		  AND (pr.id = $1 OR pr.parent_id = $1
		       OR pr.id = (SELECT parent_id FROM purchase_requests WHERE id = $1))
		ORDER BY p.picked_at DESC, p.id DESC`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.PurchasePickup, 0)
	for rows.Next() {
		var p models.PurchasePickup
		var picked time.Time
		if err := rows.Scan(&p.ID, &p.PickedBy, &picked, &p.PhotoURL, &p.Notes); err != nil {
			return nil, err
		}
		p.ID = strings.TrimSpace(p.ID)
		p.PickedAt = picked.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	return out, rows.Err()
}

// closeRemainingPickupsTx menutup pengambilan yang masih "dalam pengantaran"
// begitu dokumen (beserta pecahannya) diterima lengkap — tidak ada lagi
// barang yang bisa sedang diantar. Dulu pengambilan yang tidak dipilih pada
// penerimaan terakhir (mis. diterima sebagai "diantar vendor") tertinggal
// "dalam pengantaran" selamanya tanpa jalur untuk menutupnya.
func closeRemainingPickupsTx(tx *sql.Tx, prID string, actor Actor, now time.Time) (int64, error) {
	res, err := tx.Exec(`
		UPDATE purchase_pickups SET received_by = $2, received_at = $3
		WHERE received_at IS NULL
		  AND purchase_request_id IN (SELECT id FROM purchase_requests WHERE id = $1 OR parent_id = $1)`,
		prID, actor.Name+" (penerimaan lengkap)", now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// closePickupTx menandai pengambilan sudah diterima tempat penerimaan.
// Pengambilan wajib milik pengajuan ini atau induk/pecahannya, dan belum
// pernah ditutup. Mengembalikan nama petugas yang mengambilnya.
func closePickupTx(tx *sql.Tx, prID, pickupID string, actor Actor, now time.Time) (string, error) {
	var pickedBy string
	err := tx.QueryRow(`
		UPDATE purchase_pickups p SET received_by = $3, received_at = $4
		FROM purchase_requests pr
		WHERE p.id = $2 AND p.received_at IS NULL AND pr.id = p.purchase_request_id
		  AND (pr.id = $1 OR pr.parent_id = $1
		       OR pr.id = (SELECT parent_id FROM purchase_requests WHERE id = $1))
		RETURNING p.picked_by`, prID, pickupID, actor.Name, now).Scan(&pickedBy)
	if err == sql.ErrNoRows {
		return "", Invalid("pengambilan yang dipilih tidak ditemukan atau sudah diterima")
	}
	return pickedBy, err
}
