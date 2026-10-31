package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

// resolveTxnCreatedAt memilih created_at terbaik untuk transaksi: created_at dari
// payload bila valid; jika kosong/tak valid (umum pada transaksi sync tertunda),
// pakai created_at order tertaut agar tidak ter-stempel waktu sync; fallback ke waktu kini.
func resolveTxnCreatedAt(tx *sql.Tx, outletID string, req models.PushTransactionRequest) interface{} {
	// Prioritas: transaction_date (waktu transaksi asli, penting utk transaksi offline/tertunda).
	if t, ok := parseTimeStrict(req.TransactionDate); ok {
		return t
	}
	if t, ok := parseTimeStrict(req.CreatedAt); ok {
		return t
	}
	if req.OrderID != "" {
		var oc time.Time
		if err := tx.QueryRow(
			`SELECT created_at FROM cloud_orders WHERE id = $1 AND outlet_id = $2`,
			req.OrderID, outletID).Scan(&oc); err == nil {
			return oc
		}
	}
	return time.Now().UTC()
}

func SaveOrder(outletID string, req models.PushOrderRequest) (string, error) {
	if strings.TrimSpace(req.PaymentInfo.VoidedAt) != "" && strings.TrimSpace(req.LocalID) == "" {
		return "", fmt.Errorf("ID order void wajib diisi")
	}
	itemsJSON, err := json.Marshal(req.Items)
	if err != nil {
		return "", fmt.Errorf("item order tidak valid: %w", err)
	}
	paymentJSON, err := json.Marshal(req.PaymentInfo)
	if err != nil {
		return "", fmt.Errorf("rincian pembayaran order tidak valid: %w", err)
	}

	req.LocalID = strings.TrimSpace(req.LocalID)
	if req.LocalID == "" {
		req.LocalID = NewULID()
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = lockPOSStock(tx, outletID); err != nil {
		return "", err
	}
	cloudID, err := reservePOSID(tx, "order", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	var previousVoid, previousPayment string
	var previousVersion sql.NullInt64
	var previousUpdatedAt sql.NullTime
	err = tx.QueryRow(`SELECT COALESCE(payment_info->>'voided_at',''),
		COALESCE(payment_info->>'payment_status',''),version,updated_at
		FROM cloud_orders WHERE outlet_id=$1 AND local_id=$2`, outletID, req.LocalID).
		Scan(&previousVoid, &previousPayment, &previousVersion, &previousUpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	// A delayed unpaid/paid snapshot must never resurrect a completed void.
	if previousVoid != "" && strings.TrimSpace(req.PaymentInfo.VoidedAt) == "" {
		return cloudID, tx.Commit()
	}
	if previousVoid != "" {
		var completed bool
		if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pos_void_stock_events WHERE outlet_id=$1 AND order_id=$2 AND source_type='order')`, outletID, cloudID).Scan(&completed); err != nil {
			return "", err
		}
		if completed {
			return cloudID, tx.Commit()
		}
	}
	// Terminal payment state cannot be reopened by a delayed order snapshot.
	// A real void remains allowed and follows the existing atomic stock path.
	if strings.TrimSpace(req.PaymentInfo.VoidedAt) == "" {
		if strings.EqualFold(strings.TrimSpace(previousPayment), "paid") &&
			!strings.EqualFold(strings.TrimSpace(req.PaymentInfo.PaymentStatus), "paid") {
			return cloudID, tx.Commit()
		}
		// POS versions count enqueue seconds, so two mutations can share one
		// version. Prefer their source timestamps; version alone is a fallback.
		// Version-less legacy clients remain accepted rather than guessing age.
		if req.Version > 0 && previousVersion.Valid && previousVersion.Int64 > 0 {
			stale := int64(req.Version) < previousVersion.Int64
			if updated, valid := parseTimeStrict(req.UpdatedAt); valid && previousUpdatedAt.Valid {
				if updated.Before(previousUpdatedAt.Time) {
					stale = true
				} else if updated.After(previousUpdatedAt.Time) {
					stale = false
				}
			}
			if stale {
				return cloudID, tx.Commit()
			}
		}
	}
	err = tx.QueryRow(
		`INSERT INTO cloud_orders (id, local_id, outlet_id, outlet_code, table_number,
			customer_name, customer_phone, orderer_name, created_by, pax, total_amount, status, items, payment_info, version,
			created_at, updated_at, is_holding, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
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
		cloudID, req.LocalID, outletID, req.OutletCode, req.TableNumber,
		req.CustomerName, req.CustomerPhone, req.OrdererName, req.CreatedBy, req.Pax, req.TotalAmount, req.Status,
		string(itemsJSON), string(paymentJSON), req.Version,
		parseTime(req.CreatedAt), parseTime(req.UpdatedAt), req.IsHolding,
	).Scan(&cloudID)

	if err != nil {
		return "", err
	}

	if strings.TrimSpace(req.PaymentInfo.VoidedAt) != "" {
		if err = applyOrderVoidStock(tx, outletID, strings.TrimSpace(cloudID), req); err != nil {
			return "", fmt.Errorf("waste pembatalan: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return "", err
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
			COALESCE(customer_name,''), COALESCE(customer_phone,''),COALESCE(created_by,''),COALESCE(orderer_name,''),COALESCE(is_holding,false),pax, total_amount, status,
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
			&o.TableNumber, &o.CustomerName, &o.CustomerPhone, &o.CreatedBy, &o.OrdererName, &o.IsHolding, &o.Pax, &o.TotalAmount, &o.Status,
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
	req.LocalID = strings.TrimSpace(req.LocalID)
	if req.LocalID == "" {
		req.LocalID = NewULID()
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = lockPOSStock(tx, outletID); err != nil {
		return "", err
	}
	cloudID, err := reservePOSID(tx, "transaction", outletID, req.LocalID)
	if err != nil {
		return "", err
	}
	if req.OrderID != "" {
		req.OrderID, err = reservePOSID(tx, "order", outletID, req.OrderID)
		if err != nil {
			return "", err
		}
		var voided bool
		if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cloud_orders WHERE outlet_id=$1 AND id=$2 AND NULLIF(payment_info->>'voided_at','') IS NOT NULL)`, outletID, req.OrderID).Scan(&voided); err != nil {
			return "", err
		}
		if voided {
			return cloudID, nil
		}
	}

	// Waktu transaksi: pakai created_at payload bila valid; bila kosong/tak valid
	// (umum pada transaksi sync tertunda) pakai created_at order tertaut agar
	// tidak salah tanggal (ter-stempel waktu sync). Fallback terakhir: waktu sekarang.
	createdAt := resolveTxnCreatedAt(tx, outletID, req)
	// Old APKs omitted product_id. Resolve only an exact unique product in this
	// outlet; an ambiguous sale must remain retryable rather than skip stock.
	req.Items = append([]models.TransactionItem(nil), req.Items...)
	for i := range req.Items {
		item := &req.Items[i]
		if strings.TrimSpace(item.ProductID) == "" && item.Quantity > 0 {
			product, resolveErr := resolvePOSProduct(tx, outletID, posStockItem{Name: item.ProductName})
			if resolveErr != nil {
				return "", fmt.Errorf("identitas produk transaksi: %w", resolveErr)
			}
			item.ProductID = product.id
		}
	}

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

	var persistedCreatedAt time.Time
	err = tx.QueryRow(
		`INSERT INTO cloud_transactions (id, local_id, outlet_id, outlet_code, order_id,
			subtotal, total_amount, tax_amount, other_charges_total, charges,
			payment_method, cash_amount, change_amount, cashier_name, orderer_name,
			items, version, created_at, synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW())
		ON CONFLICT (outlet_id, local_id) DO UPDATE SET
			order_id = EXCLUDED.order_id,
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
			created_at = COALESCE(cloud_transactions.created_at, EXCLUDED.created_at),
			synced_at = NOW()
		RETURNING id, created_at`,
		cloudID, req.LocalID, outletID, req.OutletCode, req.OrderID,
		req.Subtotal, req.TotalAmount, req.TaxAmount, req.OtherChargesTotal, chargesJSON,
		req.PaymentMethod, req.CashAmount, req.ChangeAmount, cashierName, req.OrdererName,
		itemsJSON, req.Version, createdAt,
	).Scan(&cloudID, &persistedCreatedAt)

	if err != nil {
		return "", err
	}
	// A retry without source timestamps must reuse the persisted transaction
	// time, not move old payments to the time this retry reached the cloud.
	createdAt = persistedCreatedAt

	// Rincian pembayaran multi-metode (Gabung Bayar / Split Bill). Tulis ulang
	// (delete + insert) agar re-sync transaksi yang sama tidak menggandakan baris.
	// Preserve each payment's real time, including partial payments in earlier
	// shifts. Older payloads without that time use the transaction time.
	// Gagal menulis rincian pembayaran harus mengembalikan error (bukan sekadar log):
	// bila dilapor sukses, device tidak akan retry dan rekap per-metode kurang selamanya.
	// Upsert header + delete/insert di sini idempotent, jadi retry aman.
	if _, derr := tx.Exec(`DELETE FROM transaction_payments WHERE transaction_id = $1 AND outlet_id = $2`, cloudID, outletID); derr != nil {
		log.Printf("clear transaction_payments (tx=%s): %v", cloudID, derr)
		return "", fmt.Errorf("gagal menulis rincian pembayaran: %w", derr)
	}
	lines := append([]models.PaymentLine(nil), req.Payments...)
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
		paidAt := createdAt
		if parsed, valid := parseTimeStrict(p.CreatedAt); valid {
			paidAt = parsed.UTC()
		}
		method := p.PaymentMethod
		if method == "" {
			method = "other"
		}
		var note interface{}
		if p.PaymentNote != nil {
			note = *p.PaymentNote
		}
		if _, derr := tx.Exec(
			`INSERT INTO transaction_payments (transaction_id, outlet_id, payment_method, amount, payment_note, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			cloudID, outletID, method, p.Amount, note, paidAt,
		); derr != nil {
			log.Printf("insert transaction_payment (tx=%s method=%s): %v", cloudID, method, derr)
			return "", fmt.Errorf("gagal menulis rincian pembayaran: %w", derr)
		}
	}

	// Each product is atomic and has its own receipt. One failed ingredient
	// rolls back that product, while the payment and other products can sync.
	quantities := map[string]float64{}
	for _, item := range req.Items {
		if item.ProductID != "" && item.Quantity > 0 {
			quantities[item.ProductID] += float64(item.Quantity)
		}
	}
	products := make([]string, 0, len(quantities))
	for product := range quantities {
		products = append(products, product)
	}
	sort.Strings(products)
	for _, product := range products {
		if _, err = tx.Exec(`SAVEPOINT pos_product_stock`); err != nil {
			return "", err
		}
		if derr := deductPOSProduct(tx, outletID, product, quantities[product], cloudID, req.LocalID); derr != nil {
			if _, err = tx.Exec(`ROLLBACK TO SAVEPOINT pos_product_stock`); err != nil {
				return "", err
			}
			if _, err = tx.Exec(`INSERT INTO stock_deduction_failures(id,outlet_id,transaction_id,product_local_id,qty,last_error)
				VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(transaction_id,product_local_id)
				DO UPDATE SET last_error=EXCLUDED.last_error,qty=EXCLUDED.qty,status='pending'`, NewULID(), outletID, cloudID, product, quantities[product], derr.Error()); err != nil {
				return "", err
			}
		} else {
			if _, err = tx.Exec(`UPDATE stock_deduction_failures SET status='resolved' WHERE outlet_id=$1 AND transaction_id=$2 AND product_local_id=$3 AND status='pending'`, outletID, cloudID, product); err != nil {
				return "", err
			}
		}
		if _, err = tx.Exec(`RELEASE SAVEPOINT pos_product_stock`); err != nil {
			return "", err
		}
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	go logSync(outletID, "push_transaction", "transaction", 1, "success", "")
	BroadcastSync("transaction", outletID)
	if strings.TrimSpace(req.ReservationID) != "" {
		if serr := SettleReservation(outletID, req.ReservationID, cloudID, cashierName); serr != nil {
			// The financial row is committed and idempotent; keep the outbox
			// pending so settlement is retried after validation/network recovery.
			return "", fmt.Errorf("transaksi tersimpan; penyelesaian reservasi perlu dicoba ulang: %w", serr)
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
