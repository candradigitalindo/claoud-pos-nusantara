package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// resolveTxnCreatedAt memilih created_at terbaik untuk transaksi: created_at dari
// payload bila valid; jika kosong/tak valid (umum pada transaksi sync tertunda),
// pakai created_at order tertaut agar tidak ter-stempel waktu sync; fallback ke waktu kini.
func resolveTxnCreatedAt(outletID string, req models.PushTransactionRequest) interface{} {
	// Prioritas: transaction_date (waktu transaksi asli, penting utk transaksi offline/tertunda).
	if t, ok := parseTimeStrict(req.TransactionDate); ok {
		return t
	}
	if t, ok := parseTimeStrict(req.CreatedAt); ok {
		return t
	}
	if req.OrderID != "" {
		var oc time.Time
		if err := database.DB.QueryRow(
			`SELECT created_at FROM cloud_orders WHERE id = $1 AND outlet_id = $2`,
			req.OrderID, outletID).Scan(&oc); err == nil {
			return oc
		}
	}
	return parseTime(req.CreatedAt)
}

func SaveOrder(outletID string, req models.PushOrderRequest) (string, error) {
	itemsJSON, _ := json.Marshal(req.Items)
	paymentJSON, _ := json.Marshal(req.PaymentInfo)

	cloudID := req.LocalID
	if cloudID == "" {
		cloudID = NewULID()
	}
	err := database.DB.QueryRow(
		`INSERT INTO cloud_orders (id, local_id, outlet_id, outlet_code, table_number,
			customer_name, customer_phone, orderer_name, created_by, pax, total_amount, status, items, payment_info, version,
			created_at, updated_at, is_holding, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			id = EXCLUDED.id,
			table_number = EXCLUDED.table_number,
			customer_name = EXCLUDED.customer_name,
			customer_phone = EXCLUDED.customer_phone,
			orderer_name = EXCLUDED.orderer_name,
			created_by = EXCLUDED.created_by,
			pax = EXCLUDED.pax,
			total_amount = EXCLUDED.total_amount,
			status = EXCLUDED.status,
			items = EXCLUDED.items,
			payment_info = EXCLUDED.payment_info,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at,
			is_holding = EXCLUDED.is_holding,
			synced_at = NOW()
		RETURNING id`,
		cloudID, cloudID, outletID, req.OutletCode, req.TableNumber,
		req.CustomerName, req.CustomerPhone, req.OrdererName, req.CreatedBy, req.Pax, req.TotalAmount, req.Status,
		string(itemsJSON), string(paymentJSON), req.Version,
		parseTime(req.CreatedAt), parseTime(req.UpdatedAt), req.IsHolding,
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}

	// Bila order di-void, transaksinya (bila ada) bukan penjualan sah → hapus dari
	// cloud_transactions + transaction_payments agar tidak terhitung di laporan/dashboard.
	if strings.TrimSpace(req.PaymentInfo.VoidedAt) != "" {
		oid := strings.TrimSpace(cloudID) // RETURNING id bisa ter-padding (CHAR 26); order_id varchar tak padded
		database.DB.Exec(`DELETE FROM transaction_payments WHERE transaction_id IN (SELECT id FROM cloud_transactions WHERE TRIM(order_id) = $1)`, oid)
		database.DB.Exec(`DELETE FROM cloud_transactions WHERE TRIM(order_id) = $1`, oid)
	}

	// Master pelanggan: cocokkan/daftarkan pelanggan dari nama & no. HP order
	// (identitas utama = no. HP), lalu tautkan customer_id untuk histori kunjungan.
	UpsertCustomerForOrder(strings.TrimSpace(cloudID), req.CustomerName, req.CustomerPhone)

	go logSync(outletID, "push_order", "order", 1, "success", "")
	BroadcastSync("order", outletID)
	return cloudID, nil
}

func GetOrders(outletID string, page, limit int) ([]models.CloudOrder, int, error) {
	offset := (page - 1) * limit
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM cloud_orders WHERE outlet_id = $1", outletID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	rows, err := database.DB.Query(
		`SELECT id, local_id, outlet_id, outlet_code, COALESCE(table_number,''),
			COALESCE(customer_name,''), COALESCE(customer_phone,''), pax, total_amount, status,
			COALESCE(items::text,'[]'), COALESCE(payment_info::text,'{}'),
			version, created_at, updated_at, synced_at
		FROM cloud_orders WHERE outlet_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		outletID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]models.CloudOrder, 0)
	for rows.Next() {
		var o models.CloudOrder
		if err := rows.Scan(&o.ID, &o.LocalID, &o.OutletID, &o.OutletCode,
			&o.TableNumber, &o.CustomerName, &o.CustomerPhone, &o.Pax, &o.TotalAmount, &o.Status,
			&o.Items, &o.PaymentInfo, &o.Version, &o.CreatedAt, &o.UpdatedAt, &o.SyncedAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func SaveTransaction(outletID string, req models.PushTransactionRequest) (string, error) {
	cloudID := req.LocalID
	if cloudID == "" {
		cloudID = NewULID()
	}

	// Bila order tertaut sudah di-void, jangan catat transaksinya (bukan penjualan sah).
	if req.OrderID != "" {
		var voidedAt string
		database.DB.QueryRow(`SELECT COALESCE(payment_info->>'voided_at','') FROM cloud_orders WHERE id = $1`, req.OrderID).Scan(&voidedAt)
		if strings.TrimSpace(voidedAt) != "" {
			return cloudID, nil
		}
	}

	// Waktu transaksi: pakai created_at payload bila valid; bila kosong/tak valid
	// (umum pada transaksi sync tertunda) pakai created_at order tertaut agar
	// tidak salah tanggal (ter-stempel waktu sync). Fallback terakhir: waktu sekarang.
	createdAt := resolveTxnCreatedAt(outletID, req)

	itemsJSON := "[]"
	if len(req.Items) > 0 {
		b, _ := json.Marshal(req.Items)
		itemsJSON = string(b)
	}

	chargesJSON := "[]"
	if len(req.Charges) > 0 {
		b, _ := json.Marshal(req.Charges)
		chargesJSON = string(b)
	}

	// Nama kasir: app kini mengirimnya terisi; bila kosong pakai created_by
	// (= kasir menurut kontrak). Fallback lanjutan dari shift dilakukan saat baca.
	cashierName := req.CashierName
	if cashierName == "" {
		cashierName = req.CreatedBy
	}

	// Transaksi komplimen ("dibayar" gratis) BUKAN penjualan: tidak ada uang masuk.
	// App mengirim total_amount = nilai kotor item, padahal uang diterima Rp0.
	// Nolkan total & pajak agar tak menggelembungkan omzet/pajak; baris tetap
	// disimpan supaya kunjungan & pax tetap tercatat, dan nilai komplimen tetap
	// terlihat di laporan Diskon & Komplimen (dihitung dari item order).
	if strings.EqualFold(strings.TrimSpace(req.PaymentMethod), "compliment") {
		req.TotalAmount = 0
		req.TaxAmount = 0
	}

	err := database.DB.QueryRow(
		`INSERT INTO cloud_transactions (id, local_id, outlet_id, outlet_code, order_id,
			subtotal, total_amount, tax_amount, other_charges_total, charges,
			payment_method, cash_amount, change_amount, cashier_name, orderer_name,
			items, version, created_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			id = EXCLUDED.id,
			subtotal = EXCLUDED.subtotal,
			total_amount = EXCLUDED.total_amount,
			tax_amount = EXCLUDED.tax_amount,
			other_charges_total = EXCLUDED.other_charges_total,
			charges = EXCLUDED.charges,
			payment_method = EXCLUDED.payment_method,
			cash_amount = EXCLUDED.cash_amount,
			change_amount = EXCLUDED.change_amount,
			cashier_name = EXCLUDED.cashier_name,
			orderer_name = EXCLUDED.orderer_name,
			items = EXCLUDED.items,
			version = EXCLUDED.version,
			synced_at = NOW()
		RETURNING id`,
		cloudID, cloudID, outletID, req.OutletCode, req.OrderID,
		req.Subtotal, req.TotalAmount, req.TaxAmount, req.OtherChargesTotal, chargesJSON,
		req.PaymentMethod, req.CashAmount, req.ChangeAmount, cashierName, req.OrdererName,
		itemsJSON, req.Version, createdAt,
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}

	// Rincian pembayaran multi-metode (Gabung Bayar / Split Bill). Tulis ulang
	// (delete + insert) agar re-sync transaksi yang sama tidak menggandakan baris.
	// created_at memakai tanggal transaksi agar rekap per-metode konsisten dengan
	// laporan yang berbasis tanggal transaksi.
	// Gagal menulis rincian pembayaran harus mengembalikan error (bukan sekadar log):
	// bila dilapor sukses, device tidak akan retry dan rekap per-metode kurang selamanya.
	// Upsert header + delete/insert di sini idempotent, jadi retry aman.
	txCreatedAt := createdAt
	if _, derr := database.DB.Exec(`DELETE FROM transaction_payments WHERE transaction_id = $1`, cloudID); derr != nil {
		log.Printf("clear transaction_payments (tx=%s): %v", cloudID, derr)
		return "", fmt.Errorf("gagal menulis rincian pembayaran: %w", derr)
	}
	lines := req.Payments
	if len(lines) == 0 {
		// Fallback klien lama tanpa payments[]: satu baris dari metode header + total.
		method := req.PaymentMethod
		if method == "" {
			method = "other"
		}
		lines = []models.PaymentLine{{PaymentMethod: method, Amount: req.TotalAmount}}
	}
	// Normalisasi kembalian: app mengirim nominal tunai yang DISERAHKAN pelanggan
	// (mis. 50.000 untuk tagihan 30.800), sehingga total baris > total transaksi.
	// Kurangi kelebihannya dari baris tunai agar rekap per-metode = pendapatan riil.
	if req.TotalAmount > 0 {
		var sum float64
		for _, p := range lines {
			sum += p.Amount
		}
		if excess := sum - req.TotalAmount; excess > 0.009 {
			for i := range lines {
				if strings.EqualFold(lines[i].PaymentMethod, "cash") {
					cut := math.Min(excess, lines[i].Amount)
					lines[i].Amount = round2(lines[i].Amount - cut)
					excess -= cut
					if excess <= 0.009 {
						break
					}
				}
			}
		}
	}
	for _, p := range lines {
		method := p.PaymentMethod
		if method == "" {
			method = "other"
		}
		var note interface{}
		if p.PaymentNote != nil {
			note = *p.PaymentNote
		}
		if _, derr := database.DB.Exec(
			`INSERT INTO transaction_payments (transaction_id, outlet_id, payment_method, amount, payment_note, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			cloudID, outletID, method, p.Amount, note, txCreatedAt,
		); derr != nil {
			log.Printf("insert transaction_payment (tx=%s method=%s): %v", cloudID, method, derr)
			return "", fmt.Errorf("gagal menulis rincian pembayaran: %w", derr)
		}
	}

	go logSync(outletID, "push_transaction", "transaction", 1, "success", "")
	BroadcastSync("transaction", outletID)

	// Transaksi yang menutup reservasi: tandai selesai + tautkan. Non-fatal —
	// transaksinya sah walau tautannya gagal, dan admin masih bisa menutup manual.
	if strings.TrimSpace(req.ReservationID) != "" {
		if serr := SettleReservation(outletID, req.ReservationID, cloudID, cashierName); serr != nil {
			log.Printf("settle reservasi %s dari tx %s: %v", req.ReservationID, cloudID, serr)
		}
	}

	// Auto-deduct stok bahan baku via resep produk (lenient: gagal dicatat,
	// transaksi tetap commit — selisih stok lebih baik daripada transaksi gagal sync).
	// Catatan: saat ini akan no-op untuk item tanpa product_id (app outlet belum
	// mengirim local_id produk di payload transaksi — lihat memory project_cloud_pos_overview).
	//
	// Idempotensi: upsert di atas berarti push ulang (retry outbox / respons hilang)
	// tetap sampai ke sini untuk transaksi yang sama. Tanpa guard ini stok resep
	// terpotong dobel setiap re-sync.
	var stockAlreadyDeducted bool
	if err := database.DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM stock_movements WHERE ref_type='transaction' AND ref_id=$1)`,
		cloudID,
	).Scan(&stockAlreadyDeducted); err != nil {
		// Gagal cek = jangan deduct (lebih aman kurang potong sekali daripada dobel).
		log.Printf("cek idempotensi deduksi stok (tx=%s): %v", cloudID, err)
		stockAlreadyDeducted = true
	}
	if stockAlreadyDeducted {
		return cloudID, nil
	}
	for _, item := range req.Items {
		if item.ProductID == "" || item.Quantity <= 0 {
			continue
		}
		if derr := DeductStockByRecipe(outletID, item.ProductID, float64(item.Quantity), cloudID, req.LocalID); derr != nil {
			log.Printf("DeductStockByRecipe gagal (outlet=%s produk=%s transaksi=%s): %v", outletID, item.ProductID, cloudID, derr)
			go logSync(outletID, "deduct_stock", "stock_movement", 1, "failed", derr.Error())
			// Catat ke antrian retry agar selisih stok tidak menumpuk diam-diam;
			// scheduler mencoba ulang berkala (RetryFailedStockDeductions).
			if _, ierr := database.DB.Exec(`
				INSERT INTO stock_deduction_failures (id, outlet_id, transaction_id, product_local_id, qty, last_error)
				VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (transaction_id, product_local_id)
				DO UPDATE SET last_error = EXCLUDED.last_error, qty = EXCLUDED.qty`,
				NewULID(), outletID, cloudID, item.ProductID, float64(item.Quantity), derr.Error()); ierr != nil {
				log.Printf("catat stock_deduction_failure (tx=%s): %v", cloudID, ierr)
			}
		}
	}

	return cloudID, nil
}

func GetTransactions(outletID string, page, limit int) ([]models.CloudTransaction, int, error) {
	offset := (page - 1) * limit
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM cloud_transactions WHERE outlet_id = $1", outletID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	rows, err := database.DB.Query(
		`SELECT t.id, t.local_id, t.outlet_id, t.outlet_code, COALESCE(t.order_id,''),
			t.total_amount, COALESCE(t.payment_method,''), t.cash_amount, t.change_amount,
			COALESCE(NULLIF(t.cashier_name,''),
				(SELECT NULLIF(s.opened_by,'') FROM cloud_cashier_shifts s
				 WHERE s.outlet_id = t.outlet_id AND s.created_at <= t.created_at
				 ORDER BY s.created_at DESC LIMIT 1), ''),
			COALESCE(t.items::text,'[]'), t.version, t.created_at, t.synced_at
		FROM cloud_transactions t WHERE t.outlet_id = $1
		ORDER BY t.created_at DESC LIMIT $2 OFFSET $3`,
		outletID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	txns := make([]models.CloudTransaction, 0)
	for rows.Next() {
		var t models.CloudTransaction
		if err := rows.Scan(&t.ID, &t.LocalID, &t.OutletID, &t.OutletCode,
			&t.OrderID, &t.TotalAmount, &t.PaymentMethod, &t.CashAmount,
			&t.ChangeAmount, &t.CashierName, &t.Items, &t.Version, &t.CreatedAt, &t.SyncedAt); err != nil {
			return nil, 0, err
		}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return txns, total, nil
}
