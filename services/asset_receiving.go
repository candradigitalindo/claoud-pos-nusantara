package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

func pqStringArray(v []string) interface{} { return pq.Array(v) }

// Ambang kapitalisasi: barang di atas nilai ini diusulkan dicatat sebagai aset.
// Angka final ditetapkan manajemen lewat app_settings.
const defaultCapitalizationMin = 500000.0

// Baris "habis pakai" bernilai besar wajib beralasan tertulis — tanpa rem ini,
// seluruh alur bisa dilewati hanya dengan menandai semuanya habis pakai.
const defaultExpenseReasonMin = 1000000.0

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

// prItemKey membangun kunci idempotensi satu sub-item: nama entri + nama
// sub-item (dinormalkan) + urutan kemunculan. Dipakai sebagai kunci unik
// bersama purchase_request_id, sehingga klik ganda tidak melahirkan aset kembar.
func prItemKey(entryName, itemName string, occurrence int) string {
	norm := func(s string) string {
		return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
	}
	return fmt.Sprintf("%s|%s#%d", norm(entryName), norm(itemName), occurrence)
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

// receivingSourceIDs menentukan dokumen mana yang MEMEGANG item.
//
// Pengajuan yang dipecah per vendor menjadi master + anak: masternya tinggal
// cangkang. Mencatat aset dari master berarti mencatat barang yang sama dua
// kali, jadi item diambil dari anak-anaknya.
func receivingSourceIDs(prID string) ([]string, error) {
	var splitStatus sql.NullString
	var itemCount int
	if err := database.DB.QueryRow(`
		SELECT split_status, COALESCE(jsonb_array_length(items), 0)
		FROM purchase_requests WHERE id = $1`, prID).Scan(&splitStatus, &itemCount); err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	ids := []string{}
	if splitStatus.Valid && splitStatus.String == "master" {
		rows, err := database.DB.Query(`SELECT id FROM purchase_requests WHERE parent_id = $1 ORDER BY created_at`, prID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		if itemCount > 0 {
			ids = append(ids, prID) // master masih memegang sisa itemnya sendiri
		}
		return ids, nil
	}
	return []string{prID}, nil
}

// recordedQty menghitung berapa unit dari satu baris pengajuan yang SUDAH
// berwujud data — sebagai aset maupun sebagai baris GRN.
func recordedQty(prIDs []string, key string) int {
	total := 0
	for _, id := range prIDs {
		var asAsset, asGRN, asExpense sql.NullFloat64
		// Aset: kunci bisa berakhiran "/n" untuk aset tunggal yang dipecah per unit.
		database.DB.QueryRow(`
			SELECT COALESCE(SUM(quantity), 0) FROM assets
			WHERE purchase_request_id = $1 AND pr_item_key LIKE $2 AND is_deleted = false`,
			id, key+"%").Scan(&asAsset)
		// Stok gudang: dihitung dalam satuan beli (qty_dist), sama dengan qty pengajuan.
		database.DB.QueryRow(`
			SELECT COALESCE(SUM(gi.qty_dist), 0) FROM goods_receipt_items gi
			JOIN goods_receipts g ON g.id = gi.receipt_id
			WHERE g.purchase_request_id = $1 AND gi.pr_item_key = $2`, id, key).Scan(&asGRN)
		// Habis pakai: keputusan yang sengaja diambil petugas, bukan barang hilang.
		database.DB.QueryRow(`
			SELECT COALESCE(SUM(qty), 0) FROM pr_receiving_decisions
			WHERE purchase_request_id = $1 AND pr_item_key = $2 AND destination = 'habis'`,
			id, key).Scan(&asExpense)
		// Material projek: sudah berwujud data di buku material lokasi projek.
		var asMaterial sql.NullFloat64
		database.DB.QueryRow(`
			SELECT COALESCE(SUM(qty_received), 0) FROM project_materials
			WHERE purchase_request_id = $1 AND pr_item_key = $2`, id, key).Scan(&asMaterial)
		total += int(asAsset.Float64 + asGRN.Float64 + asExpense.Float64 + asMaterial.Float64)
	}
	return total
}

// BuildReceivingDraft menyusun isi dialog Serah Terima.
func BuildReceivingDraft(prID string, outletScope []string) (*models.ReceivingDraft, error) {
	var d models.ReceivingDraft
	var outletID, projectID sql.NullString
	var itemsRaw []byte
	err := database.DB.QueryRow(`
		SELECT pr.id, pr.request_number, pr.status, pr.outlet_id, COALESCE(o.name, ''),
		       pr.project_id, COALESCE(pr.vendor_name, ''), COALESCE(pr.invoice_number, ''),
		       COALESCE(pr.paid_amount, 0), COALESCE(pr.total_final, 0), pr.items
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.id = $1`, prID).
		Scan(&d.PurchaseRequestID, &d.RequestNumber, &d.Status, &outletID, &d.OutletName,
			&projectID, &d.VendorName, &d.InvoiceNumber, &d.PaidAmount, &d.TotalFinal, &itemsRaw)
	if err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	d.OutletID = outletID.String
	d.ProjectID = projectID.String
	if outletID.Valid && !outletInScope(outletID.String, outletScope) {
		return nil, fmt.Errorf("pengajuan di luar akses Anda")
	}
	d.CapitalizationMin = settingFloat("asset_capitalization_min", defaultCapitalizationMin)
	d.AlreadyReceived = d.Status == "received"
	// Gudang yang MEMBUTUHKAN barang (untuk ditampilkan) vs gudang yang
	// MENERIMA (bawaan pilihan): barang dapur selalu masuk Gudang Induk dulu,
	// lalu diteruskan lewat Transfer Stok yang berfoto.
	d.TargetWarehouseID, d.TargetWarehouseName, d.TargetWarehouseType = targetWarehouseFor(prID)
	d.ReceivingWarehouseID = defaultReceivingWarehouse()
	// Barang boleh datang sebelum dibayar (tempo) maupun sesudah (bayar di
	// muka). Yang menghalangi penerimaan hanyalah dokumen yang belum disetujui
	// atau sudah mati — bukan urusan pembayarannya.
	switch d.Status {
	case "approved", "payment_requested", "paid", "partial", "received":
		d.CanReceive = true
	}

	sourceIDs, err := receivingSourceIDs(prID)
	if err != nil {
		return nil, err
	}

	for _, srcID := range sourceIDs {
		var raw []byte
		if srcID == prID {
			raw = itemsRaw
		} else if err := database.DB.QueryRow(`SELECT items FROM purchase_requests WHERE id=$1`, srcID).Scan(&raw); err != nil {
			continue
		}
		entries, err := parsePRItems(raw)
		if err != nil {
			return nil, err
		}
		seen := map[string]int{}
		for _, entry := range entries {
			for _, sub := range entry.Items {
				base := prItemKey(entry.Name, sub.Name, seen[entry.Name+"|"+sub.Name])
				seen[entry.Name+"|"+sub.Name]++
				price := sub.FinalPrice
				if price == 0 {
					price = sub.HpsPrice
				}
				line := models.ReceivingLine{
					PRItemKey: base, SourcePRID: srcID, EntryName: entry.Name, Name: sub.Name,
					Qty: sub.Qty, Unit: sub.Unit, UnitPrice: price,
					Subtotal: price * float64(sub.Qty),
				}
				line.Recorded = recordedQty([]string{srcID}, base)
				line.Remaining = line.Qty - line.Recorded
				if line.Remaining < 0 {
					line.Remaining = 0
				}
				// Usulan tujuan. URUTANNYA PENTING: katalog stok diperiksa LEBIH
				// DULU daripada ambang harga.
				//
				// Tim aset hanya mengurus barang perlengkapan, bukan barang dapur.
				// Kalau ambang harga menang duluan, bahan dapur mahal (daging
				// premium, keju impor, saffron) akan diusulkan menjadi aset dan
				// mendarat di meja tim aset — padahal barang itu habis dimasak
				// minggu itu juga. Barang yang sudah ada di katalog stok adalah
				// urusan gudang/dapur, berapa pun harganya.
				var stockID, stockName string
				err := database.DB.QueryRow(`
					SELECT id, name FROM stock_items
					WHERE is_active = true AND lower(name) = lower($1) LIMIT 1`, sub.Name).Scan(&stockID, &stockName)
				switch {
				case err == nil:
					line.SuggestedStockItemID, line.SuggestedStockItemName = stockID, stockName
					line.Destination = "stok"
					line.Kind = "dapur"
				case price >= d.CapitalizationMin:
					// Barang mahal tetap menjadi aset meski dibeli untuk projek:
					// AC dan kitchen set hasil renovasi adalah aset outlet, bukan
					// material yang habis dikonsumsi.
					line.Destination = "aset"
					line.Kind = "perlengkapan"
				case d.ProjectID != "":
					// Semen, cat, keramik: habis dikonsumsi projek, tapi tetap
					// harus berwujud data sampai projeknya ditutup (§8.8).
					line.Destination = "material"
					line.Kind = "projek"
				default:
					line.Destination = "habis"
					line.Kind = "habis"
				}
				// Petugas tanpa hak gudang sudah memutuskan baris ini STOK dan
				// menundanya: sejak itu baris ini milik meja Gudang Induk, apa pun
				// namanya di katalog.
				if line.Remaining > 0 && stockQueuedFor(srcID, base) {
					line.Destination, line.Kind = "stok", "dapur"
				}
				d.Lines = append(d.Lines, line)
			}
		}
	}
	if d.Lines == nil {
		d.Lines = []models.ReceivingLine{}
	}
	return &d, nil
}

// ── Eksekusi penerimaan ─────────────────────────────────────────────────────

// ReceiveGoods menjalankan dialog Serah Terima: membuat aset, memasukkan stok
// ke gudang, dan menutup status pengajuan — semuanya dalam SATU transaksi.
//
// canStock menyatakan apakah pengguna berhak menambah stok (stockledger.adjust).
// Bila tidak, baris bertujuan gudang tidak dibuang melainkan ditunda: barangnya
// memang sudah diterima secara fisik, jadi status pengajuan tetap boleh maju.
func ReceiveGoods(prID string, req models.ReceiveGoodsRequest, actor string, canStock bool, outletScope []string) (*models.ReceiveGoodsResult, error) {
	draft, err := BuildReceivingDraft(prID, outletScope)
	if err != nil {
		return nil, err
	}
	if !draft.CanReceive {
		return nil, Invalid("pengajuan berstatus '%s' belum bisa diserahterimakan — setujui dulu pengajuannya", draft.Status)
	}
	byKey := map[string]models.ReceivingLine{}
	for _, l := range draft.Lines {
		byKey[l.PRItemKey] = l
	}

	outletID := draft.OutletID
	if outletID == "" {
		outletID = req.OutletID
	}
	expenseMin := settingFloat("asset_expense_reason_min", defaultExpenseReasonMin)

	if strings.TrimSpace(req.PhotoURL) == "" {
		return nil, Invalid("foto barang saat diterima wajib diunggah — itu satu-satunya bukti bahwa barangnya benar-benar sampai")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := &models.ReceiveGoodsResult{PurchaseRequestID: prID, AssetIDs: []string{}}
	grnLines := map[string][]models.GoodsReceiptItemReq{} // per gudang
	appliedQty := map[string]int{}                        // per baris, untuk hitung sisa

	for _, in := range req.Lines {
		src, ok := byKey[in.PRItemKey]
		if !ok {
			return nil, fmt.Errorf("baris '%s' tidak ada di pengajuan ini", in.PRItemKey)
		}
		qty := in.Qty
		if qty <= 0 {
			res.SkippedLines++
			continue
		}
		if qty > src.Remaining {
			return nil, fmt.Errorf("%s: jumlah (%d) melebihi sisa yang belum dicatat (%d)", src.Name, qty, src.Remaining)
		}
		price := in.UnitPrice
		if price <= 0 {
			price = src.UnitPrice
		}

		switch in.Destination {
		case "aset":
			if outletID == "" {
				return nil, fmt.Errorf("%s: outlet tujuan wajib dipilih", src.Name)
			}
			if !outletInScope(outletID, outletScope) {
				return nil, fmt.Errorf("outlet tujuan di luar akses Anda")
			}
			name := strings.TrimSpace(in.AssetName)
			if name == "" {
				name = src.Name
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
			// Mode tunggal dengan qty > 1 menghasilkan N baris bernomor sendiri;
			// kunci idempotensinya diberi akhiran agar tetap unik per unit.
			rows := 1
			perRow := qty
			if mode == "tunggal" {
				rows = qty
				perRow = 1
			}
			for i := 0; i < rows; i++ {
				key := src.PRItemKey
				if rows > 1 {
					key = fmt.Sprintf("%s/%d", src.PRItemKey, src.Recorded+i+1)
				}
				assetID, err := insertReceivedAsset(tx, receivedAsset{
					OutletID: outletID, SourcePRID: src.SourcePRID, PRItemKey: key,
					Name: name, Category: category, Qty: perRow, Unit: src.Unit,
					TrackingMode: mode, Serial: in.SerialNumber, Brand: in.Brand, Model: in.Model,
					Location: in.Location, Price: price,
					UsefulLife: in.UsefulLifeMonths, Residual: in.ResidualValue,
					Vendor: draft.VendorName, Invoice: draft.InvoiceNumber,
					ProjectID: draft.ProjectID, PRNumber: draft.RequestNumber,
				}, actor)
				if err != nil {
					return nil, err
				}
				res.AssetIDs = append(res.AssetIDs, assetID)
				res.AssetsCreated++
			}

		case "stok":
			if !canStock {
				// Ditunda ke antrean gudang. Keputusan "ini stok" dicatat supaya
				// barisnya berpindah ke meja Gudang Induk, tapi TIDAK dihitung
				// selesai (lihat appliedQty di bawah): barangnya belum masuk buku
				// stok siapa pun, jadi dokumen harus tetap terbuka.
				res.StockLinesQueued++
				if _, err := tx.Exec(`
					INSERT INTO pr_receiving_decisions (id, purchase_request_id, pr_item_key, destination, qty, reason, actor, created_at)
					VALUES ($1,$2,$3,'stok_tunda',$4,$5,$6,(now() AT TIME ZONE 'UTC'))`,
					NewULID(), src.SourcePRID, src.PRItemKey, qty, in.Reason, actor); err != nil {
					return nil, err
				}
				continue
			}
			if in.StockItemID == "" || in.WarehouseID == "" {
				return nil, fmt.Errorf("%s: item stok dan gudang wajib dipilih", src.Name)
			}
			// Scope dokumen diperiksa lewat outlet pengaju; gudang tujuannya
			// dipilih bebas di dialog, jadi harus diperiksa sendiri.
			if !WarehouseMutableInScope(in.WarehouseID, outletScope) {
				return nil, Invalid("%s: gudang tujuan di luar akses Anda", src.Name)
			}
			// Harga di pengajuan adalah harga per SATUAN BELI (mis. per kg),
			// sedangkan GRN menyimpan harga per SATUAN DASAR (mis. per gram).
			// Tanpa pembagian dist_ratio di sini, gula 5 kg seharga Rp 70.000
			// tercatat sebagai Rp 70.000.000 di buku stok.
			var distRatio float64
			if err := tx.QueryRow(`SELECT COALESCE(NULLIF(dist_ratio, 0), 1) FROM stock_items WHERE id = $1`,
				in.StockItemID).Scan(&distRatio); err != nil {
				return nil, fmt.Errorf("%s: item stok tidak ditemukan", src.Name)
			}
			grnLines[in.WarehouseID] = append(grnLines[in.WarehouseID], models.GoodsReceiptItemReq{
				ItemID: in.StockItemID, QtyDist: float64(qty),
				CostPerBase: price / distRatio, ExpiryDate: in.ExpiryDate,
				PRItemKey: src.PRItemKey,
			})

		case "material":
			if draft.ProjectID == "" {
				return nil, Invalid("%s: pengajuan ini tidak terikat projek", src.Name)
			}
			if _, err := upsertProjectMaterialTx(tx, draft.ProjectID, src.SourcePRID, src.PRItemKey,
				src.Name, src.Unit, float64(qty), price, in.Location, actor); err != nil {
				return nil, err
			}
			res.MaterialLines++

		case "habis":
			if src.UnitPrice*float64(qty) >= expenseMin && strings.TrimSpace(in.Reason) == "" {
				return nil, fmt.Errorf("%s: baris bernilai besar yang ditandai habis pakai wajib diberi alasan", src.Name)
			}

		default:
			return nil, fmt.Errorf("%s: tujuan '%s' tidak dikenal", src.Name, in.Destination)
		}
		// Hanya baris yang benar-benar berwujud data yang dihitung selesai.
		appliedQty[src.PRItemKey] += qty

		// Jejak keputusan: siapa memutuskan apa atas baris belanja mana.
		if _, err := tx.Exec(`
			INSERT INTO pr_receiving_decisions (id, purchase_request_id, pr_item_key, destination, qty, reason, actor, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,(now() AT TIME ZONE 'UTC'))`,
			NewULID(), src.SourcePRID, src.PRItemKey, in.Destination, qty, in.Reason, actor); err != nil {
			return nil, err
		}
	}

	for whID, items := range grnLines {
		grnID, err := CreateGoodsReceiptTx(tx, models.GoodsReceiptRequest{
			WarehouseID: whID, VendorName: draft.VendorName, PORef: draft.InvoiceNumber,
			PurchaseRequestID: prID, Notes: req.Notes, Items: items,
		}, actor)
		if err != nil {
			return nil, err
		}
		var num string
		tx.QueryRow(`SELECT grn_number FROM goods_receipts WHERE id=$1`, grnID).Scan(&num)
		res.GRNNumber = num
	}

	// Pengadaan barang punya DUA meja serah terima: barang dapur diterima di
	// Gudang Induk, peralatan diterima di bagian Aset. Satu dokumen bisa
	// menyentuh keduanya, jadi statusnya hanya boleh maju ke 'received' setelah
	// SELURUH barisnya selesai — bukan setelah salah satu meja selesai.
	// Tanpa aturan ini, tim aset yang mencatat mesin kopi akan menutup dokumen
	// sementara daging di gudang belum diterima siapa pun.
	outstanding := 0
	for _, l := range draft.Lines {
		if l.Remaining-appliedQty[l.PRItemKey] > 0 {
			outstanding++
		}
	}
	res.OutstandingLines = outstanding

	// Dimensi penerimaan dicatat lebih dulu, apa pun keadaan pembayarannya.
	receiptStatus := "partial"
	if outstanding == 0 {
		receiptStatus = "received"
	}
	if _, err := tx.Exec(`
		UPDATE purchase_requests SET receipt_status=$1,
			received_by = CASE WHEN $2 = 'received' THEN $3 ELSE received_by END,
			received_at = CASE WHEN $2 = 'received' THEN (now() AT TIME ZONE 'UTC') ELSE received_at END,
			updated_at = (now() AT TIME ZONE 'UTC')
		WHERE id=$4`, receiptStatus, receiptStatus, actor, prID); err != nil {
		return nil, err
	}
	tx.Exec(`UPDATE purchase_requests SET receipt_status=$1 WHERE parent_id=$2`, receiptStatus, prID)
	res.ReceiptStatus = receiptStatus

	// Status DOKUMEN baru menjadi 'received' bila barangnya lengkap DAN uangnya
	// sudah keluar. Barang yang datang sebelum dibayar tetap berstatus tagihan —
	// kalau tidak, bagian pembayaran akan mengira urusannya sudah selesai.
	if outstanding == 0 && (draft.Status == "paid" || draft.Status == "partial") {
		if _, err := tx.Exec(`
			UPDATE purchase_requests SET status='received', received_by=$1,
				received_at=(now() AT TIME ZONE 'UTC'), updated_at=(now() AT TIME ZONE 'UTC')
			WHERE id=$2`, actor, prID); err != nil {
			return nil, err
		}
		// Pengajuan yang dipecah: anak-anaknya ikut ditutup, supaya tidak ada
		// dokumen yang tertinggal 'paid' padahal barangnya sudah datang.
		tx.Exec(`
			UPDATE purchase_requests SET status='received', received_by=$1,
				received_at=(now() AT TIME ZONE 'UTC'), updated_at=(now() AT TIME ZONE 'UTC')
			WHERE parent_id=$2 AND status IN ('paid','partial')`, actor, prID)
		res.Status = "received"
	} else {
		res.Status = draft.Status
	}

	desk := req.Desk
	if desk != "dapur" && desk != "perlengkapan" {
		desk = "perlengkapan"
		for _, l := range req.Lines {
			if l.Destination == "stok" {
				desk = "dapur"
			}
		}
	}
	SaveHandoverPhoto(tx, "terima", desk, "purchase_request", prID, draft.RequestNumber,
		req.PhotoURL, "Barang diterima dari tim purchasing", actor)

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
	if len(parts) == 0 {
		parts = append(parts, "tidak ada barang yang perlu dicatat")
	}
	if outstanding > 0 {
		parts = append(parts, fmt.Sprintf("%d baris belum diterima — dokumen tetap terbuka", outstanding))
	} else if res.Status != "received" {
		parts = append(parts, "barang lengkap diterima, menunggu pembayaran")
	}
	res.Message = strings.Join(parts, " · ")
	return res, nil
}

type receivedAsset struct {
	OutletID, SourcePRID, PRItemKey        string
	Name, Category, Unit, TrackingMode     string
	Serial, Brand, Model, Location         string
	Vendor, Invoice, ProjectID, PRNumber   string
	Qty, UsefulLife                        int
	Price, Residual                        float64
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

// ListIncompleteReceipts mencari baris pengajuan berstatus 'received' yang
// jumlahnya masih kurang dari yang dibeli.
//
// Selama ada baris di sini, ada barang yang sudah dibayar tapi belum jelas
// keberadaannya — inilah alat rekonsiliasi utama modul.
//
// kind menyaring tanggung jawab: "perlengkapan" (bawaan modul aset) menyingkirkan
// barang yang ada di katalog stok, karena bahan dapur bukan urusan tim aset.
// Kosongkan untuk melihat semuanya.
func ListIncompleteReceipts(outletScope []string, kind string) ([]IncompleteReceipt, error) {
	q := `
		SELECT pr.id, pr.request_number, COALESCE(o.name, ''), COALESCE(pr.vendor_name, ''),
		       COALESCE(TO_CHAR(pr.received_at, 'YYYY-MM-DD'), ''), pr.items
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.status = 'received' AND pr.request_type = 'barang'
		  AND COALESCE(jsonb_array_length(pr.items), 0) > 0`
	args := []interface{}{}
	if outletScope != nil {
		q += ` AND pr.outlet_id = ANY($1::text[])`
		args = append(args, pqStringArray(outletScope))
	}
	q += ` ORDER BY pr.received_at DESC NULLS LAST LIMIT 300`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]IncompleteReceipt, 0)
	for rows.Next() {
		var prID, number, outlet, vendor, receivedAt string
		var raw []byte
		if err := rows.Scan(&prID, &number, &outlet, &vendor, &receivedAt, &raw); err != nil {
			return nil, err
		}
		entries, err := parsePRItems(raw)
		if err != nil {
			continue
		}
		seen := map[string]int{}
		for _, entry := range entries {
			for _, sub := range entry.Items {
				key := prItemKey(entry.Name, sub.Name, seen[entry.Name+"|"+sub.Name])
				seen[entry.Name+"|"+sub.Name]++
				recorded := recordedQty([]string{prID}, key)
				missing := sub.Qty - recorded
				if missing <= 0 {
					continue
				}
				price := sub.FinalPrice
				if price == 0 {
					price = sub.HpsPrice
				}
				lineKind := lineKindOf(prID, key, sub.Name)
				if kind != "" && lineKind != kind {
					continue
				}
				out = append(out, IncompleteReceipt{
					PurchaseRequestID: prID, RequestNumber: number, OutletName: outlet,
					VendorName: vendor, ReceivedAt: receivedAt, EntryName: entry.Name,
					Name: sub.Name, Unit: sub.Unit, Qty: sub.Qty, Recorded: recorded,
					Missing: missing, UnitPrice: price, MissingValue: price * float64(missing),
					Kind: lineKind,
				})
			}
		}
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

// ListReceivingQueue mencari pengajuan yang sudah dibayar tapi barisnya belum
// seluruhnya diterima di meja `kind`.
//
// Dokumen yang sudah 'received' ikut ditampilkan bila masih menyisakan baris:
// itu terjadi ketika satu meja sudah selesai dan meja lain belum.
func ListReceivingQueue(kind string, outletScope []string) ([]ReceivingQueueRow, error) {
	q := `
		SELECT pr.id, pr.request_number, pr.status, COALESCE(o.name, ''), COALESCE(pr.vendor_name, ''),
		       COALESCE(TO_CHAR(pr.paid_at, 'YYYY-MM-DD'), ''), COALESCE(pr.paid_amount, 0),
		       COALESCE(pr.total_final, 0), COALESCE(pr.receipt_status, ''), pr.items
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.request_type = 'barang'
		  AND pr.status IN ('approved', 'payment_requested', 'paid', 'partial', 'received')
		  AND COALESCE(jsonb_array_length(pr.items), 0) > 0`
	args := []interface{}{}
	// Pemilahan meja sengaja HANYA per baris (lineKindOf), bukan lewat kolom
	// goods_kind. Kolom itu dibekukan saat pengajuan dibuat; bila katalog stok
	// berubah sesudahnya, saringan SQL dan saringan per baris bisa saling
	// bertolak belakang dan dokumen lenyap dari KEDUA antrean.
	if outletScope != nil {
		args = append(args, pqStringArray(outletScope))
		q += fmt.Sprintf(` AND pr.outlet_id = ANY($%d::text[])`, len(args))
	}
	q += ` ORDER BY pr.paid_at DESC NULLS LAST LIMIT 200`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ReceivingQueueRow, 0)
	for rows.Next() {
		var r ReceivingQueueRow
		var raw []byte
		if err := rows.Scan(&r.PurchaseRequestID, &r.RequestNumber, &r.Status, &r.OutletName,
			&r.VendorName, &r.PaidAt, &r.PaidAmount, &r.TotalFinal, &r.ReceiptStatus, &raw); err != nil {
			return nil, err
		}
		entries, err := parsePRItems(raw)
		if err != nil {
			continue
		}
		seen := map[string]int{}
		for _, entry := range entries {
			for _, sub := range entry.Items {
				key := prItemKey(entry.Name, sub.Name, seen[entry.Name+"|"+sub.Name])
				seen[entry.Name+"|"+sub.Name]++
				remaining := sub.Qty - recordedQty([]string{r.PurchaseRequestID}, key)
				if remaining <= 0 {
					continue
				}
				if kind != "" && lineKindOf(r.PurchaseRequestID, key, sub.Name) != kind {
					continue
				}
				price := sub.FinalPrice
				if price == 0 {
					price = sub.HpsPrice
				}
				r.Lines++
				r.Units += remaining
				r.Value += price * float64(remaining)
			}
		}
		if r.Lines > 0 {
			_, r.TargetWarehouseName, r.TargetWarehouseType = targetWarehouseFor(r.PurchaseRequestID)
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// lineKindFor menentukan meja mana yang menerima barang: yang sudah ada di
// katalog stok adalah barang dapur (Gudang Induk), selebihnya perlengkapan
// (bagian Aset). Material projek tidak dipisah di sini — pengajuan projek
// diterima oleh PIC projeknya, dan pemilahan per barisnya terjadi di dialog.
func lineKindFor(name string) string {
	var exists bool
	if err := database.DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM stock_items WHERE is_active = true AND lower(name) = lower($1))`,
		name).Scan(&exists); err == nil && exists {
		return "dapur"
	}
	return "perlengkapan"
}

// stockQueuedFor: seorang petugas tanpa hak gudang sudah memutuskan baris ini
// adalah STOK dan menundanya ke antrean gudang (destination 'stok_tunda').
// Keputusan itu memindahkan baris ke meja Gudang Induk apa pun nama barangnya —
// kalau tidak, "Gula pasir 1 kg" yang tidak persis sama dengan katalog akan
// menunggu di meja yang salah selamanya.
func stockQueuedFor(prID, key string) bool {
	var exists bool
	database.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM pr_receiving_decisions
			WHERE purchase_request_id = $1 AND pr_item_key = $2 AND destination = 'stok_tunda')`,
		prID, key).Scan(&exists)
	return exists
}

// lineKindOf = lineKindFor + keputusan tunda. Satu-satunya rumus meja yang
// dipakai antrean, dialog, dan laporan, supaya ketiganya tidak pernah berbeda.
func lineKindOf(prID, key, name string) string {
	if stockQueuedFor(prID, key) {
		return "dapur"
	}
	return lineKindFor(name)
}

// targetWarehouseFor menebak gudang yang MEMBUTUHKAN barang pengajuan ini:
// gudang pada run MRP bila pengajuan lahir dari MRP, atau gudang outlet
// pengaju. Tidak disimpan sebagai kolom — cukup diturunkan dari tautan yang
// sudah ada (mrp_run_id, outlet_id).
func targetWarehouseFor(prID string) (id, name, typ string) {
	var wid, wname, wtype sql.NullString
	database.DB.QueryRow(`
		SELECT w.id, w.name, w.type
		FROM purchase_requests pr
		LEFT JOIN mrp_runs mr ON mr.id = pr.mrp_run_id
		LEFT JOIN warehouses w ON w.id = COALESCE(mr.warehouse_id,
			(SELECT ow.id FROM warehouses ow WHERE ow.outlet_id = pr.outlet_id AND ow.type = 'outlet' AND ow.is_active = true LIMIT 1))
		WHERE pr.id = $1`, prID).Scan(&wid, &wname, &wtype)
	return wid.String, wname.String, wtype.String
}

// defaultReceivingWarehouse: gudang induk aktif pertama — tempat bawaan
// barang dapur diterima sebelum diteruskan ke outlet.
func defaultReceivingWarehouse() string {
	var id sql.NullString
	database.DB.QueryRow(`SELECT id FROM warehouses WHERE type = 'central' AND is_active = true ORDER BY created_at LIMIT 1`).Scan(&id)
	return id.String
}

// ── Pemisahan jenis belanja sejak pengajuan ─────────────────────────────────

// ClassifyPurchaseItems menentukan jenis belanja sebuah pengajuan barang, dan
// menolak yang mencampur.
//
// Satu pengajuan tidak boleh memuat barang dapur dan peralatan sekaligus, karena
// serah terimanya berada di dua tempat berbeda: barang dapur diterima di Gudang
// Induk, peralatan di bagian Aset. Dokumen campuran memaksa satu berkas
// diantarkan ke dua meja, dan itulah yang selama ini membuat barang tercecer.
//
// Mengembalikan "dapur" atau "perlengkapan"; error bila tercampur.
func ClassifyPurchaseItems(items []models.PurchaseRequestItem) (string, error) {
	dapur := []string{}
	perlengkapan := []string{}
	for _, entry := range items {
		for _, sub := range entry.Items {
			if strings.TrimSpace(sub.Name) == "" {
				continue
			}
			if lineKindFor(sub.Name) == "dapur" {
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
}

func trimList(v []string, max int) []string {
	if len(v) <= max {
		return v
	}
	return append(v[:max:max], fmt.Sprintf("dan %d lainnya", len(v)-max))
}
