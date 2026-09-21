package services

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

func resvScopeCond(alias string, outletIDs []string, idx int) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	col := "outlet_id"
	if alias != "" {
		col = alias + ".outlet_id"
	}
	return fmt.Sprintf(" AND %s = ANY($%d::text[])", col, idx), []interface{}{pq.Array(outletIDs)}
}

// resolveReservationItems looks up each item's authoritative name & price from
// cloud_products (within the outlet) to prevent tampering, returning items + subtotal.
func resolveReservationItems(outletID string, items []models.ReservationItem) ([]models.ReservationItem, float64) {
	out := make([]models.ReservationItem, 0, len(items))
	var subtotal float64
	for _, it := range items {
		if it.Qty <= 0 {
			continue
		}
		name, price := it.ProductName, it.Price
		if it.ProductID != "" {
			var n string
			var p float64
			if err := database.DB.QueryRow(
				`SELECT name, price FROM cloud_products WHERE id = $1 AND outlet_id = $2 AND is_deleted = false`,
				it.ProductID, outletID).Scan(&n, &p); err == nil {
				name, price = n, p
			}
		}
		line := models.ReservationItem{ProductID: it.ProductID, ProductName: name, Qty: it.Qty, Price: price, Subtotal: price * float64(it.Qty)}
		subtotal += line.Subtotal
		out = append(out, line)
	}
	return out, subtotal
}

func scanReservation(scan func(dest ...interface{}) error) (*models.Reservation, error) {
	var r models.Reservation
	var itemsJSON string
	if err := scan(&r.ID, &r.OutletID, &r.OutletName, &r.CustomerName, &r.CustomerPhone, &r.Pax,
		&itemsJSON, &r.Subtotal, &r.DownPayment, &r.Total, &r.ReservationDate, &r.ReservationTime,
		&r.Status, &r.Notes, &r.Source, &r.CreatedAt, &r.UpdatedAt,
		&r.PaidAmount, &r.PendingAmount, &r.CancelDisposition, &r.ConfirmedAt, &r.CancelledAt,
		&r.SettledAt, &r.PosTransactionID); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(itemsJSON), &r.Items)
	if r.Items == nil {
		r.Items = []models.ReservationItem{}
	}
	// Sisa dihitung dari uang yang TERVALIDASI, bukan dari DP yang diminta.
	r.Remaining = math.Max(r.Total-r.PaidAmount, 0)
	r.DpPaid = r.DownPayment <= 0 || r.PaidAmount+0.009 >= r.DownPayment
	return &r, nil
}

// reservationCols disusun sebagai fungsi karena stempel waktu ditampilkan
// dalam zona aplikasi, yang bisa diubah lewat pengaturan tanpa rilis ulang.
func reservationCols() string {
	return `r.id, r.outlet_id, COALESCE(o.name,''), r.customer_name, r.customer_phone, r.pax,
	COALESCE(r.items::text,'[]'), r.subtotal, r.down_payment, r.total,
	COALESCE(TO_CHAR(r.reservation_date,'YYYY-MM-DD'),''), r.reservation_time, r.status, r.notes, r.source,
	r.created_at, r.updated_at,
	COALESCE((SELECT SUM(CASE WHEN p.type = 'refund' THEN -p.amount ELSE p.amount END)
	          FROM reservation_payments p WHERE p.reservation_id = r.id AND p.status = 'validated'), 0),
	COALESCE((SELECT SUM(p.amount) FROM reservation_payments p
	          WHERE p.reservation_id = r.id AND p.status = 'pending' AND p.type <> 'refund'), 0),
	COALESCE(r.cancel_disposition, ''), ` + tzStamp("r.confirmed_at") + `, ` + tzStamp("r.cancelled_at") + `,
	` + tzStamp("r.settled_at") + `, TRIM(COALESCE(r.pos_transaction_id, ''))`
}

func ListReservations(outletID, status, dateFrom, dateTo string, outletScope []string) ([]models.Reservation, error) {
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1
	if outletID != "" {
		conds = append(conds, fmt.Sprintf("r.outlet_id = $%d", idx))
		args = append(args, outletID)
		idx++
	}
	if status != "" {
		conds = append(conds, fmt.Sprintf("r.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if dateFrom != "" {
		conds = append(conds, fmt.Sprintf("r.reservation_date >= $%d::date", idx))
		args = append(args, dateFrom)
		idx++
	}
	if dateTo != "" {
		conds = append(conds, fmt.Sprintf("r.reservation_date <= $%d::date", idx))
		args = append(args, dateTo)
		idx++
	}
	if sc, sa := resvScopeCond("r", outletScope, idx); sc != "" {
		conds = append(conds, strings.TrimPrefix(sc, " AND "))
		args = append(args, sa...)
	}
	q := fmt.Sprintf(`SELECT %s FROM reservations r LEFT JOIN outlets o ON o.id = r.outlet_id
		WHERE %s ORDER BY r.reservation_date DESC NULLS LAST, r.reservation_time DESC, r.created_at DESC`,
		reservationCols(), strings.Join(conds, " AND "))
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Reservation, 0)
	for rows.Next() {
		r, err := scanReservation(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func GetReservation(id string, outletScope []string) (*models.Reservation, error) {
	sc, sa := resvScopeCond("r", outletScope, 2)
	args := append([]interface{}{id}, sa...)
	q := fmt.Sprintf(`SELECT %s FROM reservations r LEFT JOIN outlets o ON o.id = r.outlet_id WHERE r.id = $1%s`, reservationCols(), sc)
	r, err := scanReservation(database.DB.QueryRow(q, args...).Scan)
	if err != nil {
		return nil, err
	}
	// Detail selalu membawa riwayat pembayarannya: itulah jejak uang mukanya.
	r.Payments, _ = ListReservationPayments(r.ID)
	return r, nil
}

func saveReservation(id string, req models.ReservationRequest, source string) (*models.Reservation, error) {
	if strings.TrimSpace(req.CustomerName) == "" {
		return nil, fmt.Errorf("nama pemesan wajib diisi")
	}
	if req.OutletID == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	// Endpoint publik tanpa auth: batasi jumlah item agar satu request tidak bisa
	// memicu ribuan lookup produk (resolveReservationItems query per item).
	if len(req.Items) > 100 {
		return nil, fmt.Errorf("jumlah item reservasi maksimal 100")
	}
	if req.Pax <= 0 {
		req.Pax = 1
	}
	items, subtotal := resolveReservationItems(req.OutletID, req.Items)
	itemsJSON, _ := json.Marshal(items)
	total := subtotal
	// DP yang diminta. Dari halaman publik mengikuti kebijakan persen DP;
	// admin boleh menentukan sendiri, tapi tidak melebihi total.
	if source == "public" && id == "" {
		req.DownPayment = math.Round(total * ReservationDpPercent() / 100)
	}
	if req.DownPayment < 0 || req.DownPayment > total+0.009 {
		return nil, fmt.Errorf("DP yang diminta tidak boleh negatif atau melebihi total")
	}
	// Status hanya boleh ditentukan saat DIBUAT, dan 'confirmed' langsung hanya
	// sah bila tidak ada DP yang diminta. Setelah itu status bergerak lewat
	// UpdateReservationStatus, mengikuti pembayaran — bukan dropdown bebas.
	status := "pending"
	if id == "" && req.Status == "confirmed" {
		if req.DownPayment > 0 {
			return nil, Invalid("reservasi dengan DP hanya bisa dikonfirmasi setelah DP-nya tervalidasi")
		}
		status = "confirmed"
	}
	if id == "" {
		id = NewULID()
		var confirmedAt interface{}
		if status == "confirmed" {
			confirmedAt = time.Now().UTC()
		}
		_, err := database.DB.Exec(`
			INSERT INTO reservations (id, outlet_id, customer_name, customer_phone, pax, items,
				subtotal, down_payment, total, reservation_date, reservation_time, status, notes, source, confirmed_at, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NULLIF($10,'')::date, $11,$12,$13,$14,$15, NOW(), NOW())`,
			id, req.OutletID, req.CustomerName, req.CustomerPhone, req.Pax, string(itemsJSON),
			subtotal, req.DownPayment, total, req.ReservationDate, req.ReservationTime, status, req.Notes, source, confirmedAt)
		if err != nil {
			return nil, err
		}
	} else {
		// Status TIDAK disentuh di sini (lihat UpdateReservationStatus).
		_, err := database.DB.Exec(`
			UPDATE reservations SET customer_name=$1, customer_phone=$2, pax=$3, items=$4,
				subtotal=$5, down_payment=$6, total=$7, reservation_date=NULLIF($8,'')::date,
				reservation_time=$9, notes=$10, updated_at=NOW()
			WHERE id=$11`,
			req.CustomerName, req.CustomerPhone, req.Pax, string(itemsJSON),
			subtotal, req.DownPayment, total, req.ReservationDate, req.ReservationTime, req.Notes, id)
		if err != nil {
			return nil, err
		}
		// Menurunkan DP yang diminta setelah uang masuk bisa membuat syarat
		// konfirmasi mendadak terpenuhi — biarkan, tapi konfirmasi tetap eksplisit.
	}
	return GetReservation(id, nil)
}

func CreateReservation(req models.ReservationRequest, outletScope []string) (*models.Reservation, error) {
	if outletScope != nil {
		allowed := false
		for _, id := range outletScope {
			if id == req.OutletID {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("outlet di luar akses Anda")
		}
	}
	r, err := saveReservation("", req, "admin")
	if err == nil {
		go NotifyReservationCreated(r)
	}
	return r, err
}

func UpdateReservation(id string, req models.ReservationRequest, outletScope []string) (*models.Reservation, error) {
	existing, err := GetReservation(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	if existing.Status == "done" || existing.Status == "cancelled" {
		return nil, Invalid("reservasi yang sudah selesai/dibatalkan tidak bisa diubah")
	}
	req.OutletID = existing.OutletID // outlet immutable on edit
	return saveReservation(id, req, existing.Source)
}

// UpdateReservationStatus menggerakkan status MENGIKUTI pembayaran:
//
//	pending ──DP tervalidasi──▶ confirmed ──dilayani (POS/admin)──▶ done
//	   │                            │
//	   └──────── cancelled ─────────┘  (wajib pilih: refund | hangus bila sudah ada uang masuk)
//
// done dan cancelled bersifat final. Konfirmasi tanpa DP tervalidasi ditolak,
// supaya "Dikonfirmasi" benar-benar berarti uangnya sudah ada.
func UpdateReservationStatus(id, status, disposition, actor string, outletScope []string) (*models.Reservation, error) {
	r, err := GetReservation(id, outletScope)
	if err != nil {
		return nil, fmt.Errorf("reservasi tidak ditemukan")
	}
	if r.Status == "done" || r.Status == "cancelled" {
		return nil, Invalid("reservasi sudah %s dan tidak bisa diubah lagi", map[string]string{"done": "selesai", "cancelled": "dibatalkan"}[r.Status])
	}
	if r.PendingAmount > 0 && status != "cancelled" {
		return nil, Invalid("masih ada bukti pembayaran yang menunggu validasi — validasi atau tolak dulu")
	}
	switch status {
	case "confirmed":
		if !r.DpPaid {
			return nil, Invalid("DP yang diminta %.0f belum tervalidasi (baru %.0f) — catat/validasi pembayarannya dulu", r.DownPayment, r.PaidAmount)
		}
		_, err = database.DB.Exec(`UPDATE reservations SET status='confirmed', confirmed_at=COALESCE(confirmed_at, (now() AT TIME ZONE 'UTC')), updated_at=NOW() WHERE id=$1`, id)
	case "done":
		_, err = database.DB.Exec(`UPDATE reservations SET status='done', settled_at=COALESCE(settled_at, (now() AT TIME ZONE 'UTC')), updated_at=NOW() WHERE id=$1`, id)
	case "cancelled":
		if r.PaidAmount > 0 {
			if disposition != "refund" && disposition != "hangus" {
				return nil, Invalid("sudah ada uang masuk %.0f — tentukan nasibnya: 'refund' (dikembalikan, catat refundnya) atau 'hangus' (menjadi pendapatan lain)", r.PaidAmount)
			}
		} else {
			disposition = ""
		}
		_, err = database.DB.Exec(`UPDATE reservations SET status='cancelled', cancel_disposition=$2, cancelled_at=(now() AT TIME ZONE 'UTC'), updated_at=NOW() WHERE id=$1`, id, disposition)
	case "pending":
		return nil, Invalid("status tidak bisa dikembalikan ke menunggu")
	default:
		return nil, Invalid("status '%s' tidak dikenal", status)
	}
	if err != nil {
		return nil, err
	}
	out, err := GetReservation(id, outletScope)
	if err == nil {
		switch status {
		case "confirmed":
			go NotifyReservationConfirmed(out)
		case "cancelled":
			go NotifyReservationCancelled(out)
		}
	}
	return out, err
}

func DeleteReservation(id string, outletScope []string) error {
	r, err := GetReservation(id, outletScope)
	if err != nil {
		return fmt.Errorf("reservasi tidak ditemukan")
	}
	// Jejak uang tidak boleh ikut terhapus: reservasi yang sudah menerima
	// pembayaran tervalidasi dibatalkan lewat status, bukan dihapus.
	for _, p := range r.Payments {
		if p.Status == "validated" {
			return Invalid("reservasi ini sudah punya pembayaran tervalidasi — batalkan lewat status, jangan dihapus")
		}
	}
	_, err = database.DB.Exec(`DELETE FROM reservations WHERE id=$1`, id)
	return err
}

// ── Public (no auth) ────────────────────────────────────────

// GetOutletBySlug resolves an active outlet id/name by its public slug.
func GetOutletBySlug(slug string) (string, string, error) {
	var id, name string
	err := database.DB.QueryRow(`SELECT id, name FROM outlets WHERE slug = $1 AND is_active = true`, slug).Scan(&id, &name)
	return id, name, err
}

// GetPublicMenu returns the outlet's products grouped by category (with photos),
// for the public reservation page.
func GetPublicMenu(slug string) (*models.PublicMenu, error) {
	outletID, outletName, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	rows, err := database.DB.Query(`
		SELECT COALESCE(NULLIF(category_name,''),'Lainnya') AS cat, id, name, price, COALESCE(photo_url,'')
		FROM cloud_products
		WHERE outlet_id = $1 AND is_deleted = false
		ORDER BY cat ASC, name ASC`, outletID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menu := &models.PublicMenu{OutletID: outletID, OutletName: outletName, Slug: slug, Categories: []models.PublicCategory{}}
	idxByCat := map[string]int{}
	for rows.Next() {
		var cat string
		var p models.PublicProduct
		if err := rows.Scan(&cat, &p.ID, &p.Name, &p.Price, &p.PhotoURL); err != nil {
			return nil, err
		}
		i, ok := idxByCat[cat]
		if !ok {
			i = len(menu.Categories)
			idxByCat[cat] = i
			menu.Categories = append(menu.Categories, models.PublicCategory{Name: cat, Products: []models.PublicProduct{}})
		}
		menu.Categories[i].Products = append(menu.Categories[i].Products, p)
	}
	return menu, rows.Err()
}

// CreatePublicReservation creates a pending reservation from the public page.
func CreatePublicReservation(slug string, req models.ReservationRequest) (*models.Reservation, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	req.OutletID = outletID
	req.Status = "pending"
	r, err := saveReservation("", req, "public")
	if err == nil {
		go NotifyReservationCreated(r)
	}
	return r, err
}
