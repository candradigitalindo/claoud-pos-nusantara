package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Batas wajar satu pesanan tamu — mencegah satu permintaan iseng membuat
// ratusan baris item yang harus ditarik dan dicetak POS.
const (
	maxOnlineOrderItems = 50
	maxOnlineOrderQty   = 99
)

// CreatePublicOrder mencatat pesanan dari halaman QR tamu.
//
// Nama menu dan harga TIDAK diambil dari kiriman tamu: hanya product_local_id
// dan qty yang dipercaya, sisanya dibaca ulang dari master menu outlet. Tanpa
// ini, badan permintaan bisa dipalsukan untuk memesan menu dengan harga sendiri.
func CreatePublicOrder(slug string, req models.PublicOrderRequest) (*models.OnlineOrder, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}

	if strings.TrimSpace(req.TableNumber) == "" {
		return nil, fmt.Errorf("nomor meja wajib diisi")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("pesanan masih kosong")
	}
	if len(req.Items) > maxOnlineOrderItems {
		return nil, fmt.Errorf("terlalu banyak jenis menu dalam satu pesanan")
	}

	// Master menu & add-on dibaca sekali untuk memvalidasi seluruh baris.
	validProducts, productPrices, err := productCatalog(outletID)
	if err != nil {
		return nil, err
	}
	addonsByProduct, err := GetProductAddons(outletID)
	if err != nil {
		return nil, err
	}

	items := make([]models.OnlineOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		name, ok := validProducts[it.ProductLocalID]
		if !ok {
			return nil, fmt.Errorf("menu tidak tersedia")
		}
		if it.Qty < 1 || it.Qty > maxOnlineOrderQty {
			return nil, fmt.Errorf("jumlah untuk %s tidak valid", name)
		}

		// Hanya add-on yang benar-benar terdaftar pada menu itu yang diterima.
		allowed := map[string]models.CloudProductAddon{}
		for _, a := range addonsByProduct[it.ProductLocalID] {
			allowed[a.LocalID] = a
		}
		picked := make([]models.OnlineOrderAddon, 0, len(it.Addons))
		for _, a := range it.Addons {
			master, ok := allowed[a.ID]
			if !ok {
				return nil, fmt.Errorf("tambahan tidak tersedia untuk %s", name)
			}
			picked = append(picked, models.OnlineOrderAddon{
				ID: master.LocalID, Name: master.Name, Price: master.Price,
			})
		}

		items = append(items, models.OnlineOrderItem{
			ProductLocalID: it.ProductLocalID,
			ProductName:    name,
			Qty:            it.Qty,
			Notes:          trimTo(it.Notes, 120),
			Addons:         picked,
		})
	}

	// Restoran yang tidak sedang buka shift TIDAK boleh menerima uang: tidak
	// ada kasir yang mencatat pembayarannya dan tidak ada dapur yang memasak.
	// Status shift di cloud bisa tertinggal satu siklus sinkronisasi, jadi
	// paling buruk tamu diminta menunggu sebentar — jauh lebih baik daripada
	// membayar makanan yang tak akan dibuat.
	open, err := hasOpenShift(outletID)
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, fmt.Errorf("kasir belum buka — silakan pesan lewat pramusaji")
	}

	// Total dihitung DI SINI, memakai daftar biaya yang dikirim POS, supaya
	// nominal QRIS sama persis dengan tagihan yang nanti dihitung kasir.
	charges, err := ActiveCharges(outletID)
	if err != nil {
		return nil, err
	}
	subtotal := 0.0
	for _, it := range items {
		unit := productPrices[it.ProductLocalID]
		for _, a := range it.Addons {
			unit += a.Price
		}
		subtotal += unit * float64(it.Qty)
	}
	total, chargeLines := CalculateOrderTotal(subtotal, charges)
	if total <= 0 {
		return nil, fmt.Errorf("total pesanan tidak valid")
	}

	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	id := newChargeID()
	_, err = database.DB.Exec(
		`INSERT INTO online_orders (id, outlet_id, table_number, customer_name,
			customer_phone, notes, items, status, total_amount, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'awaiting_payment', $8, (now() AT TIME ZONE 'UTC'), (now() AT TIME ZONE 'UTC'))`,
		id, outletID, trimTo(req.TableNumber, 20), trimTo(req.CustomerName, 100),
		trimTo(req.CustomerPhone, 30), trimTo(req.Notes, 200), payload, total,
	)
	if err != nil {
		return nil, err
	}

	// QRIS dinamis: nominalnya persis sebesar tagihan pesanan ini.
	charge, err := CreateQRISCharge(outletID, id, total,
		fmt.Sprintf("Pesanan Meja %s", req.TableNumber))
	if err != nil {
		// Pesanan tanpa QR tidak ada gunanya — buang supaya tidak menggantung
		// sebagai baris yang tak akan pernah dibayar maupun ditarik POS.
		database.DB.Exec(`DELETE FROM online_orders WHERE id = $1`, id)
		return nil, err
	}
	if _, err := database.DB.Exec(
		`UPDATE online_orders SET qris_charge_id = $2, updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $1`,
		id, charge.ChargeID,
	); err != nil {
		return nil, err
	}

	go logSync(outletID, "online_order_created", "online_order", 1, "success", "")

	return &models.OnlineOrder{
		ID: id, OutletID: outletID, TableNumber: req.TableNumber,
		CustomerName: req.CustomerName, Items: items,
		Status:      models.OnlineOrderAwaitingPayment,
		Subtotal:    subtotal,
		ChargeLines: chargeLines,
		TotalAmount: total,
		Payment:     charge,
	}, nil
}

// hasOpenShift menjawab apakah outlet sedang membuka shift kasir. Sumbernya
// adalah shift yang disinkronkan POS, jadi jawabannya bisa tertinggal paling
// lama satu siklus sinkronisasi.
func hasOpenShift(outletID string) (bool, error) {
	var n int
	err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM cloud_cashier_shifts
		WHERE outlet_id = $1 AND closed_at IS NULL`, outletID,
	).Scan(&n)
	return n > 0, err
}

// SimulateOnlineOrderPaid melunasi tagihan pesanan TANPA penyedia asli.
//
// Hanya berjalan saat penyedia aktif adalah "mock", jadi tidak ada jalur di
// produksi yang bisa menandai pesanan lunas tanpa uang benar-benar masuk.
func SimulateOnlineOrderPaid(slug, orderID string) error {
	if !MockSimulationEnabled() {
		return fmt.Errorf("simulasi hanya tersedia saat penyedia pembayaran mock aktif")
	}
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return fmt.Errorf("outlet tidak ditemukan")
	}
	var chargeID sql.NullString
	if err := database.DB.QueryRow(
		`SELECT qris_charge_id FROM online_orders WHERE outlet_id = $1 AND id = $2`,
		outletID, orderID,
	).Scan(&chargeID); err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}
	if !chargeID.Valid || chargeID.String == "" {
		return fmt.Errorf("pesanan ini tidak punya tagihan QRIS")
	}
	return SimulateQRISChargePaid(outletID, chargeID.String)
}

// productCatalog mengembalikan nama dan harga menu outlet. Harga dibaca dari
// master — HARGA KIRIMAN TAMU TIDAK PERNAH DIPAKAI, karena badan permintaan
// bisa dipalsukan untuk memesan dengan harga sendiri.
func productCatalog(outletID string) (map[string]string, map[string]float64, error) {
	rows, err := database.DB.Query(
		`SELECT local_id, name, price FROM cloud_products
		WHERE outlet_id = $1 AND is_deleted = false`, outletID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	names := map[string]string{}
	prices := map[string]float64{}
	for rows.Next() {
		var id, name string
		var price float64
		if err := rows.Scan(&id, &name, &price); err != nil {
			return nil, nil, err
		}
		names[id] = name
		prices[id] = price
	}
	return names, prices, rows.Err()
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// PendingOnlineOrders mengembalikan pesanan yang menunggu ditarik POS.
//
// Termasuk pesanan yang klaimnya basi: perangkat yang mengambilnya tidak pernah
// mengonfirmasi (mati/kehilangan jaringan), jadi pesanan itu harus bisa diambil
// perangkat lain daripada menggantung selamanya.
func PendingOnlineOrders(outletID string) ([]models.OnlineOrder, error) {
	rows, err := database.DB.Query(
		`SELECT id, table_number, COALESCE(customer_name,''), COALESCE(customer_phone,''),
			COALESCE(notes,''), items, status, created_at, total_amount, paid_amount
		FROM online_orders
		WHERE outlet_id = $1
			AND (status = 'new'
				OR (status = 'claimed' AND claimed_at < (now() AT TIME ZONE 'UTC') - INTERVAL '5 minutes'))
		ORDER BY created_at ASC LIMIT 50`,
		outletID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.OnlineOrder, 0)
	for rows.Next() {
		var o models.OnlineOrder
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&o.ID, &o.TableNumber, &o.CustomerName, &o.CustomerPhone,
			&o.Notes, &raw, &o.Status, &createdAt, &o.TotalAmount, &o.PaidAmount); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &o.Items); err != nil {
			// Baris rusak dilewati, bukan menggagalkan seluruh penarikan —
			// satu pesanan cacat tidak boleh memblokir sisanya.
			continue
		}
		o.OutletID = outletID
		o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ClaimOnlineOrder menandai pesanan sedang diambil satu perangkat.
//
// Atomik: hanya SATU perangkat yang bisa menang, sehingga dua kasir yang
// menarik bersamaan tidak menghasilkan order meja dobel. Perangkat yang sama
// boleh mengklaim ulang (idempoten) agar percobaan ulang setelah gagal jaringan
// tidak buntu.
func ClaimOnlineOrder(outletID, orderID, deviceID string) (bool, error) {
	res, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'claimed', claimed_by = $3, claimed_at = (now() AT TIME ZONE 'UTC'), updated_at = (now() AT TIME ZONE 'UTC')
		WHERE outlet_id = $1 AND id = $2
			AND (status = 'new'
				OR (status = 'claimed'
					AND (claimed_by = $3 OR claimed_at < (now() AT TIME ZONE 'UTC') - INTERVAL '5 minutes')))`,
		outletID, orderID, deviceID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ConfirmOnlineOrder menutup siklus: POS sudah berhasil membuat order meja.
// Setelah ini pesanan tidak pernah ditarik lagi, walau klaimnya lewat waktu.
func ConfirmOnlineOrder(outletID, orderID, localOrderID string) error {
	_, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'confirmed', local_order_id = $3, updated_at = (now() AT TIME ZONE 'UTC')
		WHERE outlet_id = $1 AND id = $2 AND status <> 'rejected'`,
		outletID, orderID, nullStr(localOrderID),
	)
	return err
}

// RejectOnlineOrder dipakai saat pesanan tak bisa diproses (mis. mejanya sudah
// dipakai tamu lain). Disimpan, bukan dihapus, agar tetap terlihat di riwayat.
func RejectOnlineOrder(outletID, orderID, reason string) error {
	_, err := database.DB.Exec(
		`UPDATE online_orders
		SET status = 'rejected', reject_reason = $3, updated_at = (now() AT TIME ZONE 'UTC')
		WHERE outlet_id = $1 AND id = $2 AND status <> 'confirmed'`,
		outletID, orderID, nullStr(trimTo(reason, 200)),
	)
	return err
}

// OnlineOrderStatus dipakai halaman tamu untuk menampilkan perkembangan
// pesanannya tanpa perlu login.
//
// Status pembayaran ikut dikembalikan supaya halaman tamu bisa membedakan
// "belum dibayar" dari "QR kedaluwarsa" — keduanya membuat pesanan tertahan di
// awaiting_payment, tetapi yang kedua menuntut tamu memesan ulang.
func OnlineOrderStatus(slug, orderID string) (map[string]interface{}, error) {
	outletID, _, err := GetOutletBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("outlet tidak ditemukan")
	}
	var status string
	var paymentStatus sql.NullString
	var total, paid float64
	err = database.DB.QueryRow(
		`SELECT o.status, o.total_amount, o.paid_amount, q.status
		FROM online_orders o
		LEFT JOIN qris_charges q ON q.id = o.qris_charge_id
		WHERE o.outlet_id = $1 AND o.id = $2`,
		outletID, orderID,
	).Scan(&status, &total, &paid, &paymentStatus)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pesanan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":         status,
		"payment_status": paymentStatus.String,
		"total_amount":   total,
		"paid_amount":    paid,
	}, nil
}
