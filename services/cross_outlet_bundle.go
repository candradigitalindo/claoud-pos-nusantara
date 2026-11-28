package services

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

func generateBundleClaimCodeTx(tx *sql.Tx) (string, error) {
	prefix := fmt.Sprintf("BDL%s", time.Now().In(GetTimezoneLocation()).Format("060102"))
	seq, err := nextDocNumber(tx, "cross_outlet_bundle_claims", "claim_code", prefix)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, seq), nil
}

func bundleProductHasRecipeSQL(alias string) string {
	return fmt.Sprintf(`(
		EXISTS(SELECT 1 FROM product_recipes pr WHERE pr.product_id = %s.id)
		OR EXISTS(SELECT 1 FROM product_recipe_links prl WHERE prl.product_id = %s.id)
		OR EXISTS(SELECT 1 FROM product_recipe_sources prs WHERE prs.product_id = %s.id)
		OR COALESCE(%s.recipe_master_id, '') <> ''
		OR COALESCE(%s.stock_type, '') = 'direct_stock'
	)`, alias, alias, alias, alias, alias)
}

func AttachProductBundleBadges(products []models.CloudProduct) {
	if len(products) == 0 {
		return
	}
	ids := make([]string, 0, len(products))
	idxByID := make(map[string]int, len(products))
	for i, p := range products {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		idxByID[id] = i
	}
	if len(ids) == 0 {
		return
	}
	rows, err := database.DB.Query(`
		SELECT TRIM(c.source_product_id), COALESCE(po.name, '')
		FROM product_bundle_components c
		JOIN outlets po ON po.id = c.partner_outlet_id
		WHERE c.is_active = true AND c.source_product_id = ANY($1::char(26)[])
		ORDER BY po.name ASC, c.id ASC`, pq.Array(ids))
	if err != nil {
		return
	}
	defer rows.Close()
	seen := map[string]map[string]bool{}
	for rows.Next() {
		var pid, pname string
		if rows.Scan(&pid, &pname) != nil {
			continue
		}
		idx, ok := idxByID[pid]
		if !ok {
			continue
		}
		products[idx].BundleCount++
		if pname != "" {
			if seen[pid] == nil {
				seen[pid] = map[string]bool{}
			}
			if !seen[pid][pname] {
				seen[pid][pname] = true
				products[idx].BundlePartners = append(products[idx].BundlePartners, pname)
			}
		}
	}
}

func ListBundlePartnerOutlets() ([]models.Outlet, error) {
	return GetOutletsScoped(nil)
}

func ListBundlePartnerProducts(outletID string) ([]models.BundlePartnerProduct, error) {
	oid := strings.TrimSpace(outletID)
	if oid == "" {
		return nil, Invalid("outlet_id wajib diisi")
	}
	rows, err := database.DB.Query(`
		SELECT TRIM(p.id), COALESCE(p.local_id, ''), TRIM(p.outlet_id), p.name,
		       COALESCE(p.category_name, ''), p.price, `+bundleProductHasRecipeSQL("p")+`
		FROM cloud_products p
		WHERE p.outlet_id = $1 AND p.is_deleted = false
		ORDER BY LOWER(p.name) ASC, p.name ASC, p.id ASC`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.BundlePartnerProduct, 0)
	for rows.Next() {
		var p models.BundlePartnerProduct
		if err := rows.Scan(&p.ID, &p.LocalID, &p.OutletID, &p.Name, &p.CategoryName, &p.Price, &p.HasRecipe); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanBundleComponent(scan func(dest ...interface{}) error) (models.ProductBundleComponent, error) {
	var c models.ProductBundleComponent
	err := scan(
		&c.ID, &c.SourceOutletID, &c.SourceOutletName, &c.SourceProductID, &c.SourceProductName, &c.SourceProductPrice,
		&c.PartnerOutletID, &c.PartnerOutletName, &c.PartnerProductID, &c.PartnerProductName, &c.PartnerProductPrice,
		&c.PartnerHasRecipe, &c.QtyPerBundle, &c.AllocationAmount, &c.ValidDays, &c.Notes, &c.IsActive,
		&c.CreatedAt, &c.UpdatedAt,
	)
	c.ID = strings.TrimSpace(c.ID)
	c.SourceOutletID = strings.TrimSpace(c.SourceOutletID)
	c.SourceProductID = strings.TrimSpace(c.SourceProductID)
	c.PartnerOutletID = strings.TrimSpace(c.PartnerOutletID)
	c.PartnerProductID = strings.TrimSpace(c.PartnerProductID)
	return c, err
}

func bundleComponentSelectSQL() string {
	return `SELECT c.id, c.source_outlet_id, COALESCE(so.name,''), c.source_product_id, COALESCE(sp.name,''), COALESCE(sp.price,0),
		c.partner_outlet_id, COALESCE(po.name,''), c.partner_product_id, COALESCE(pp.name,''), COALESCE(pp.price,0),
		` + bundleProductHasRecipeSQL("pp") + `, c.qty_per_bundle, c.allocation_amount, c.valid_days,
		COALESCE(c.notes,''), c.is_active, ` + tzStamp("c.created_at") + `, ` + tzStamp("c.updated_at") + `
		FROM product_bundle_components c
		JOIN outlets so ON so.id = c.source_outlet_id
		JOIN cloud_products sp ON sp.id = c.source_product_id
		JOIN outlets po ON po.id = c.partner_outlet_id
		JOIN cloud_products pp ON pp.id = c.partner_product_id`
}

func ListProductBundleComponents(sourceProductID string, outletScope []string) ([]models.ProductBundleComponent, error) {
	pid := strings.TrimSpace(sourceProductID)
	var srcOutlet string
	if err := database.DB.QueryRow(`SELECT TRIM(outlet_id) FROM cloud_products WHERE id = $1 AND is_deleted = false`, pid).Scan(&srcOutlet); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("produk tidak ditemukan")
		}
		return nil, err
	}
	if outletScope != nil && !outletInScope(srcOutlet, outletScope) {
		return nil, fmt.Errorf("produk di luar akses Anda")
	}
	rows, err := database.DB.Query(bundleComponentSelectSQL()+` WHERE c.source_product_id = $1 AND c.is_active = true ORDER BY po.name ASC, pp.name ASC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProductBundleComponent, 0)
	for rows.Next() {
		c, err := scanBundleComponent(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ListAllBundleComponents(outletID string, outletScope []string) ([]models.ProductBundleComponent, error) {
	conds := []string{"c.is_active = true", "sp.is_deleted = false", "pp.is_deleted = false"}
	args := []interface{}{}
	idx := 1
	if strings.TrimSpace(outletID) != "" {
		conds = append(conds, fmt.Sprintf("(c.source_outlet_id = $%d OR c.partner_outlet_id = $%d)", idx, idx))
		args = append(args, strings.TrimSpace(outletID))
		idx++
	}
	if outletScope != nil {
		conds = append(conds, fmt.Sprintf("(c.source_outlet_id = ANY($%d::text[]) OR c.partner_outlet_id = ANY($%d::text[]))", idx, idx))
		args = append(args, pq.Array(outletScope))
		idx++
	}
	q := bundleComponentSelectSQL() + " WHERE " + strings.Join(conds, " AND ") + " ORDER BY so.name ASC, sp.name ASC, po.name ASC, pp.name ASC"
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.ProductBundleComponent, 0)
	for rows.Next() {
		c, err := scanBundleComponent(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func SaveProductBundleComponents(sourceProductID string, items []models.SaveBundleComponentItem) ([]models.ProductBundleComponent, error) {
	pid := strings.TrimSpace(sourceProductID)
	var srcOutlet, srcName string
	var srcPrice float64
	if err := database.DB.QueryRow(`SELECT TRIM(outlet_id), name, price FROM cloud_products WHERE id = $1 AND is_deleted = false`, pid).Scan(&srcOutlet, &srcName, &srcPrice); err != nil {
		if err == sql.ErrNoRows {
			return nil, Invalid("produk sumber tidak ditemukan")
		}
		return nil, err
	}

	type cleanItem struct {
		partnerOutletID  string
		partnerProductID string
		qty              float64
		allocation       float64
		validDays        int
		notes            string
	}
	cleaned := make([]cleanItem, 0, len(items))
	seenPartnerProd := map[string]bool{}
	var totalAlloc float64
	for i, it := range items {
		po := strings.TrimSpace(it.PartnerOutletID)
		pp := strings.TrimSpace(it.PartnerProductID)
		if po == "" || pp == "" {
			return nil, Invalid("komponen #%d: outlet mitra dan produk mitra wajib dipilih", i+1)
		}
		if po == srcOutlet {
			return nil, Invalid("komponen #%d: outlet mitra harus berbeda dari outlet produk sumber", i+1)
		}
		if seenPartnerProd[pp] {
			return nil, Invalid("komponen #%d: produk mitra duplikat — gabungkan jumlahnya dalam satu baris", i+1)
		}
		seenPartnerProd[pp] = true
		if it.QtyPerBundle <= 0 || math.IsNaN(it.QtyPerBundle) || math.IsInf(it.QtyPerBundle, 0) {
			return nil, Invalid("komponen #%d: jumlah per paket harus lebih dari 0", i+1)
		}
		if it.AllocationAmount < 0 || math.IsNaN(it.AllocationAmount) || math.IsInf(it.AllocationAmount, 0) {
			return nil, Invalid("komponen #%d: nilai alokasi tidak boleh negatif", i+1)
		}
		if it.ValidDays < 0 || it.ValidDays > 365 {
			return nil, Invalid("komponen #%d: masa berlaku kupon harus 0–365 hari", i+1)
		}
		var dbPartnerOutlet string
		if err := database.DB.QueryRow(`SELECT TRIM(outlet_id) FROM cloud_products WHERE id = $1 AND is_deleted = false`, pp).Scan(&dbPartnerOutlet); err != nil {
			if err == sql.ErrNoRows {
				return nil, Invalid("komponen #%d: produk mitra tidak ditemukan", i+1)
			}
			return nil, err
		}
		if dbPartnerOutlet != po {
			return nil, Invalid("komponen #%d: produk mitra tidak sesuai dengan outlet mitra yang dipilih", i+1)
		}
		alloc := round2(it.AllocationAmount)
		totalAlloc = round2(totalAlloc + alloc)
		cleaned = append(cleaned, cleanItem{
			partnerOutletID: po, partnerProductID: pp,
			qty: round4(it.QtyPerBundle), allocation: alloc,
			validDays: it.ValidDays, notes: strings.TrimSpace(it.Notes),
		})
	}
	if srcPrice > 0 && totalAlloc > srcPrice+0.009 {
		return nil, Invalid("total alokasi omzet (%.0f) tidak boleh melebihi harga jual produk '%s' (%.0f)", totalAlloc, srcName, srcPrice)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Soft-deactivate old components not in new list, upsert active ones.
	if _, err := tx.Exec(`UPDATE product_bundle_components SET is_active = false, updated_at = NOW() WHERE source_product_id = $1`, pid); err != nil {
		return nil, err
	}
	for _, ci := range cleaned {
		_, err := tx.Exec(`
			INSERT INTO product_bundle_components (id, source_outlet_id, source_product_id, partner_outlet_id, partner_product_id,
				qty_per_bundle, allocation_amount, valid_days, notes, is_active, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true,NOW(),NOW())
			ON CONFLICT (source_product_id, partner_product_id) DO UPDATE SET
				partner_outlet_id = EXCLUDED.partner_outlet_id,
				qty_per_bundle = EXCLUDED.qty_per_bundle,
				allocation_amount = EXCLUDED.allocation_amount,
				valid_days = EXCLUDED.valid_days,
				notes = EXCLUDED.notes,
				is_active = true,
				updated_at = NOW()`,
			NewULID(), srcOutlet, pid, ci.partnerOutletID, ci.partnerProductID,
			ci.qty, ci.allocation, ci.validDays, ci.notes)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ListProductBundleComponents(pid, nil)
}

func computeBundleExpiry(saleAt time.Time, validDays int) interface{} {
	if validDays <= 0 {
		return nil
	}
	loc := GetTimezoneLocation()
	localSale := saleAt.In(loc)
	// validDays=1 → berlaku sampai akhir hari yang sama (23:59:59 zona aplikasi).
	endDay := time.Date(localSale.Year(), localSale.Month(), localSale.Day(), 23, 59, 59, 0, loc).AddDate(0, 0, validDays-1)
	return endDay.UTC()
}

// SyncCrossOutletBundleClaimsTx menerbitkan tiket klaim bundling lintas outlet
// secara otomatis dan idempoten saat transaksi POS disimpan.
func SyncCrossOutletBundleClaimsTx(tx *sql.Tx, outletID, txnID, txnLocalID, orderID string, items []models.TransactionItem, customerName, cashierName string, saleAt time.Time) (int, error) {
	if len(items) == 0 || strings.TrimSpace(txnID) == "" {
		return 0, nil
	}
	// Gabungkan qty per product_id/local_id dalam transaksi yang sama.
	type soldProd struct {
		cloudID string
		name    string
		qty     float64
	}
	byCloudID := map[string]*soldProd{}
	for _, it := range items {
		if it.Quantity <= 0 {
			continue
		}
		pid := strings.TrimSpace(it.ProductID)
		if pid == "" {
			continue
		}
		var cloudID, pname string
		err := tx.QueryRow(`
			SELECT TRIM(id), name FROM cloud_products
			WHERE outlet_id = $1 AND (id = $2 OR local_id = $2) AND is_deleted = false
			ORDER BY (id = $2) DESC LIMIT 1`, outletID, pid).Scan(&cloudID, &pname)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		if cur, ok := byCloudID[cloudID]; ok {
			cur.qty = round4(cur.qty + float64(it.Quantity))
		} else {
			byCloudID[cloudID] = &soldProd{cloudID: cloudID, name: pname, qty: float64(it.Quantity)}
		}
	}
	if len(byCloudID) == 0 {
		return 0, nil
	}

	created := 0
	for srcProdID, sp := range byCloudID {
		rows, err := tx.Query(`
			SELECT c.id, c.partner_outlet_id, c.partner_product_id, COALESCE(pp.local_id,''), pp.name,
			       c.qty_per_bundle, c.allocation_amount, c.valid_days, COALESCE(c.notes,'')
			FROM product_bundle_components c
			JOIN cloud_products pp ON pp.id = c.partner_product_id AND pp.is_deleted = false
			WHERE c.source_outlet_id = $1 AND c.source_product_id = $2 AND c.is_active = true
			ORDER BY c.id ASC`, outletID, srcProdID)
		if err != nil {
			return created, err
		}
		type compRow struct {
			id, partnerOutlet, partnerProd, partnerLocal, partnerName, notes string
			qtyPer, allocPer                                                 float64
			validDays                                                        int
		}
		comps := []compRow{}
		for rows.Next() {
			var r compRow
			if err := rows.Scan(&r.id, &r.partnerOutlet, &r.partnerProd, &r.partnerLocal, &r.partnerName, &r.qtyPer, &r.allocPer, &r.validDays, &r.notes); err != nil {
				rows.Close()
				return created, err
			}
			comps = append(comps, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return created, err
		}

		for _, c := range comps {
			// Idempoten bila transaksi POS di-sync ulang.
			var exists bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cross_outlet_bundle_claims WHERE source_transaction_id = $1 AND source_product_id = $2 AND partner_product_id = $3)`,
				txnID, srcProdID, c.partnerProd).Scan(&exists); err != nil {
				return created, err
			}
			if exists {
				continue
			}
			code, err := generateBundleClaimCodeTx(tx)
			if err != nil {
				return created, err
			}
			partnerQty := round4(sp.qty * c.qtyPer)
			allocTotal := round2(sp.qty * c.allocPer)
			exp := computeBundleExpiry(saleAt, c.validDays)
			res, err := tx.Exec(`
				INSERT INTO cross_outlet_bundle_claims (
					id, claim_code, component_id,
					source_outlet_id, source_transaction_id, source_transaction_local_id, source_order_id,
					source_product_id, source_product_name, source_qty,
					partner_outlet_id, partner_product_id, partner_product_local_id, partner_product_name, partner_qty,
					allocation_amount, status, customer_name, cashier_name, sale_at, expires_at, notes, created_at, updated_at
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'pending',$17,$18,$19,$20,$21,NOW(),NOW()
				) ON CONFLICT DO NOTHING`,
				NewULID(), code, c.id,
				outletID, txnID, strings.TrimSpace(txnLocalID), strings.TrimSpace(orderID),
				srcProdID, sp.name, sp.qty,
				c.partnerOutlet, c.partnerProd, c.partnerLocal, c.partnerName, partnerQty,
				allocTotal, strings.TrimSpace(customerName), strings.TrimSpace(cashierName), saleAt.UTC(), exp, c.notes)
			if err != nil {
				return created, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				created++
			}
		}
	}
	return created, nil
}

// VoidPendingBundleClaimsTx membatalkan kupon bundling yang belum diklaim bila order asal di-void.
func VoidPendingBundleClaimsTx(tx *sql.Tx, outletID, orderID string, txnIDs []string, reason string) error {
	if strings.TrimSpace(orderID) == "" && len(txnIDs) == 0 {
		return nil
	}
	note := strings.TrimSpace(reason)
	if note == "" {
		note = "Transaksi asal dibatalkan (void)"
	}
	_, err := tx.Exec(`
		UPDATE cross_outlet_bundle_claims
		SET status = 'voided', voided_at = (now() AT TIME ZONE 'UTC'), void_reason = $4, updated_at = NOW()
		WHERE source_outlet_id = $1 AND status = 'pending'
		  AND (($2 <> '' AND source_order_id = $2) OR (array_length($3::text[], 1) IS NOT NULL AND source_transaction_id = ANY($3::text[])))`,
		outletID, strings.TrimSpace(orderID), pq.Array(txnIDs), note)
	if err != nil {
		return err
	}
	// Bila kupon sudah terlanjur diklaim sebelum order di-void, catat peringatan review.
	_, err = tx.Exec(`
		UPDATE cross_outlet_bundle_claims
		SET notes = TRIM(BOTH E'
' FROM COALESCE(notes, '') || E'
[Perhatian] Transaksi asal di-void setelah barang diserahkan: ' || $4),
		    updated_at = NOW()
		WHERE source_outlet_id = $1 AND status = 'claimed'
		  AND (($2 <> '' AND source_order_id = $2) OR (array_length($3::text[], 1) IS NOT NULL AND source_transaction_id = ANY($3::text[])))`,
		outletID, strings.TrimSpace(orderID), pq.Array(txnIDs), note)
	return err
}

func CreateManualBundleClaim(req models.ManualCrossOutletBundleClaimRequest, actor string) (*models.CrossOutletBundleClaim, error) {
	cid := strings.TrimSpace(req.ComponentID)
	if cid == "" {
		return nil, Invalid("aturan bundling wajib dipilih")
	}
	qty := req.SourceQty
	if qty <= 0 {
		qty = 1
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var srcOutlet, srcProd, srcName, partnerOutlet, partnerProd, partnerLocal, partnerName string
	var qtyPer, allocPer float64
	var validDays int
	err = tx.QueryRow(`
		SELECT c.source_outlet_id, c.source_product_id, sp.name,
		       c.partner_outlet_id, c.partner_product_id, COALESCE(pp.local_id,''), pp.name,
		       c.qty_per_bundle, c.allocation_amount, c.valid_days
		FROM product_bundle_components c
		JOIN cloud_products sp ON sp.id = c.source_product_id
		JOIN cloud_products pp ON pp.id = c.partner_product_id
		WHERE c.id = $1 AND c.is_active = true`, cid).Scan(
		&srcOutlet, &srcProd, &srcName, &partnerOutlet, &partnerProd, &partnerLocal, &partnerName,
		&qtyPer, &allocPer, &validDays)
	if err == sql.ErrNoRows {
		return nil, Invalid("aturan bundling tidak ditemukan atau tidak aktif")
	}
	if err != nil {
		return nil, err
	}

	code, err := generateBundleClaimCodeTx(tx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	id := NewULID()
	exp := computeBundleExpiry(now, validDays)
	if _, err := tx.Exec(`
		INSERT INTO cross_outlet_bundle_claims (
			id, claim_code, component_id,
			source_outlet_id, source_transaction_id, source_transaction_local_id, source_order_id,
			source_product_id, source_product_name, source_qty,
			partner_outlet_id, partner_product_id, partner_product_local_id, partner_product_name, partner_qty,
			allocation_amount, status, customer_name, cashier_name, sale_at, expires_at, notes, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,'','','',$5,$6,$7,$8,$9,$10,$11,$12,$13,'pending',$14,$15,$16,$17,$18,NOW(),NOW()
		)`,
		id, code, cid, srcOutlet, srcProd, srcName, round4(qty),
		partnerOutlet, partnerProd, partnerLocal, partnerName, round4(qty*qtyPer),
		round2(qty*allocPer), strings.TrimSpace(req.CustomerName), strings.TrimSpace(actor), now, exp, strings.TrimSpace(req.Notes)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCrossOutletBundleClaim(id, nil)
}

func expireOverdueBundleClaims() {
	_, _ = database.DB.Exec(`
		UPDATE cross_outlet_bundle_claims
		SET status = 'expired', updated_at = NOW()
		WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at < (now() AT TIME ZONE 'UTC')`)
}

func bundleClaimSelectSQL() string {
	return `SELECT c.id, c.claim_code, COALESCE(c.component_id,''),
		c.source_outlet_id, COALESCE(so.name,''), c.source_transaction_id, c.source_transaction_local_id, c.source_order_id,
		c.source_product_id, c.source_product_name, c.source_qty,
		c.partner_outlet_id, COALESCE(po.name,''), c.partner_product_id, c.partner_product_local_id, c.partner_product_name,
		c.partner_qty, c.claimed_qty, c.allocation_amount, c.status, c.customer_name, c.cashier_name,
		` + tzStamp("c.sale_at") + `, ` + tzStamp("c.expires_at") + `, ` + tzStamp("c.claimed_at") + `,
		c.claimed_by, c.stock_deducted, c.stock_deduction_note, ` + tzStamp("c.voided_at") + `, c.void_reason, c.notes
		FROM cross_outlet_bundle_claims c
		JOIN outlets so ON so.id = c.source_outlet_id
		JOIN outlets po ON po.id = c.partner_outlet_id`
}

func scanBundleClaim(scan func(dest ...interface{}) error, outletScope []string) (models.CrossOutletBundleClaim, error) {
	var r models.CrossOutletBundleClaim
	err := scan(
		&r.ID, &r.ClaimCode, &r.ComponentID,
		&r.SourceOutletID, &r.SourceOutletName, &r.SourceTransactionID, &r.SourceTransactionLocalID, &r.SourceOrderID,
		&r.SourceProductID, &r.SourceProductName, &r.SourceQty,
		&r.PartnerOutletID, &r.PartnerOutletName, &r.PartnerProductID, &r.PartnerProductLocalID, &r.PartnerProductName,
		&r.PartnerQty, &r.ClaimedQty, &r.AllocationAmount, &r.Status, &r.CustomerName, &r.CashierName,
		&r.SaleAt, &r.ExpiresAt, &r.ClaimedAt, &r.ClaimedBy, &r.StockDeducted, &r.StockDeductionNote,
		&r.VoidedAt, &r.VoidReason, &r.Notes,
	)
	r.ID = strings.TrimSpace(r.ID)
	r.ComponentID = strings.TrimSpace(r.ComponentID)
	r.SourceOutletID = strings.TrimSpace(r.SourceOutletID)
	r.SourceProductID = strings.TrimSpace(r.SourceProductID)
	r.PartnerOutletID = strings.TrimSpace(r.PartnerOutletID)
	r.PartnerProductID = strings.TrimSpace(r.PartnerProductID)
	r.CanClaim = r.Status == "pending" && (outletScope == nil || outletInScope(r.PartnerOutletID, outletScope))
	return r, err
}

func GetCrossOutletBundleClaim(id string, outletScope []string) (*models.CrossOutletBundleClaim, error) {
	expireOverdueBundleClaims()
	conds := []string{"c.id = $1"}
	args := []interface{}{strings.TrimSpace(id)}
	if outletScope != nil {
		conds = append(conds, "(c.source_outlet_id = ANY($2::text[]) OR c.partner_outlet_id = ANY($2::text[]))")
		args = append(args, pq.Array(outletScope))
	}
	c, err := scanBundleClaim(database.DB.QueryRow(bundleClaimSelectSQL()+" WHERE "+strings.Join(conds, " AND "), args...).Scan, outletScope)
	if err != nil {
		return nil, fmt.Errorf("kupon bundling tidak ditemukan")
	}
	return &c, nil
}

func ListCrossOutletBundleClaims(filter models.CrossOutletBundleFilter, outletScope []string) (*models.CrossOutletBundleListResponse, error) {
	expireOverdueBundleClaims()
	conds := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	oid := strings.TrimSpace(filter.OutletID)
	dir := strings.ToLower(strings.TrimSpace(filter.Direction))
	if outletScope != nil {
		switch dir {
		case "inbound":
			conds = append(conds, fmt.Sprintf("c.partner_outlet_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(outletScope))
			idx++
		case "outbound":
			conds = append(conds, fmt.Sprintf("c.source_outlet_id = ANY($%d::text[])", idx))
			args = append(args, pq.Array(outletScope))
			idx++
		default:
			conds = append(conds, fmt.Sprintf("(c.source_outlet_id = ANY($%d::text[]) OR c.partner_outlet_id = ANY($%d::text[]))", idx, idx))
			args = append(args, pq.Array(outletScope))
			idx++
		}
	}
	if oid != "" {
		switch dir {
		case "inbound":
			conds = append(conds, fmt.Sprintf("c.partner_outlet_id = $%d", idx))
		case "outbound":
			conds = append(conds, fmt.Sprintf("c.source_outlet_id = $%d", idx))
		default:
			conds = append(conds, fmt.Sprintf("(c.source_outlet_id = $%d OR c.partner_outlet_id = $%d)", idx, idx))
		}
		args = append(args, oid)
		idx++
	}
	if st := strings.TrimSpace(filter.Status); st != "" {
		conds = append(conds, fmt.Sprintf("c.status = $%d", idx))
		args = append(args, st)
		idx++
	}
	if df := strings.TrimSpace(filter.DateFrom); df != "" {
		conds = append(conds, fmt.Sprintf("c.sale_at >= tz_day_start($%d::date)", idx))
		args = append(args, df)
		idx++
	}
	if dt := strings.TrimSpace(filter.DateTo); dt != "" {
		conds = append(conds, fmt.Sprintf("c.sale_at < tz_day_start($%d::date + 1)", idx))
		args = append(args, dt)
		idx++
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		conds = append(conds, fmt.Sprintf("(c.claim_code ILIKE $%d OR c.customer_name ILIKE $%d OR c.source_product_name ILIKE $%d OR c.partner_product_name ILIKE $%d)", idx, idx, idx, idx))
		args = append(args, "%"+q+"%")
		idx++
	}

	rows, err := database.DB.Query(
		bundleClaimSelectSQL()+" WHERE "+strings.Join(conds, " AND ")+" ORDER BY (c.status = 'pending') DESC, c.sale_at DESC, c.id DESC LIMIT 500",
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &models.CrossOutletBundleListResponse{
		Claims:         []models.CrossOutletBundleClaim{},
		Reconciliation: []models.CrossOutletBundleReconciliationRow{},
	}
	recMap := map[string]*models.CrossOutletBundleReconciliationRow{}
	recOrder := []string{}
	for rows.Next() {
		c, err := scanBundleClaim(rows.Scan, outletScope)
		if err != nil {
			return nil, err
		}
		resp.Claims = append(resp.Claims, c)
		resp.Summary.TotalClaims++
		switch c.Status {
		case "pending":
			resp.Summary.PendingCount++
			resp.Summary.PendingAllocation = round2(resp.Summary.PendingAllocation + c.AllocationAmount)
		case "claimed":
			resp.Summary.ClaimedCount++
			resp.Summary.ClaimedAllocation = round2(resp.Summary.ClaimedAllocation + c.AllocationAmount)
		case "expired":
			resp.Summary.ExpiredCount++
		case "voided":
			resp.Summary.VoidedCount++
		}

		rk := c.SourceOutletID + "|" + c.PartnerOutletID + "|" + c.SourceProductID + "|" + c.PartnerProductID
		rr := recMap[rk]
		if rr == nil {
			rr = &models.CrossOutletBundleReconciliationRow{
				SourceOutletID: c.SourceOutletID, SourceOutletName: c.SourceOutletName,
				PartnerOutletID: c.PartnerOutletID, PartnerOutletName: c.PartnerOutletName,
				SourceProductName: c.SourceProductName, PartnerProductName: c.PartnerProductName,
			}
			recMap[rk] = rr
			recOrder = append(recOrder, rk)
		}
		if c.Status != "voided" {
			rr.TotalBundlesSold = round4(rr.TotalBundlesSold + c.SourceQty)
			rr.TotalPartnerQty = round4(rr.TotalPartnerQty + c.PartnerQty)
		}
		switch c.Status {
		case "claimed":
			rr.ClaimedPartnerQty = round4(rr.ClaimedPartnerQty + c.ClaimedQty)
			rr.ClaimedAllocation = round2(rr.ClaimedAllocation + c.AllocationAmount)
		case "pending":
			rr.PendingPartnerQty = round4(rr.PendingPartnerQty + c.PartnerQty)
			rr.PendingAllocation = round2(rr.PendingAllocation + c.AllocationAmount)
		case "expired":
			rr.ExpiredPartnerQty = round4(rr.ExpiredPartnerQty + c.PartnerQty)
		case "voided":
			rr.VoidedPartnerQty = round4(rr.VoidedPartnerQty + c.PartnerQty)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, k := range recOrder {
		resp.Reconciliation = append(resp.Reconciliation, *recMap[k])
	}
	return resp, nil
}

// ClaimCrossOutletBundle mencatat penyerahan produk bundling di outlet mitra:
// memotong stok resep di gudang outlet mitra (tanpa transaksi kas/PB1 baru)
// dan mengaktifkan jurnal alokasi omzet internal antar outlet.
func ClaimCrossOutletBundle(claimID string, req models.ClaimCrossOutletBundleRequest, actor string, outletScope []string) (*models.CrossOutletBundleClaim, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var partnerOutlet, partnerProdID, partnerLocal, partnerName, sourceName, claimCode, status string
	var partnerQty float64
	var expiresAt sql.NullTime
	err = tx.QueryRow(`
		SELECT TRIM(partner_outlet_id), TRIM(partner_product_id), partner_product_local_id, partner_product_name,
		       source_product_name, claim_code, status, partner_qty, expires_at
		FROM cross_outlet_bundle_claims
		WHERE id = $1 FOR UPDATE`, strings.TrimSpace(claimID)).Scan(
		&partnerOutlet, &partnerProdID, &partnerLocal, &partnerName, &sourceName, &claimCode, &status, &partnerQty, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, Invalid("kupon bundling tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if outletScope != nil && !outletInScope(partnerOutlet, outletScope) {
		return nil, Invalid("hanya outlet mitra penyedia barang yang berhak menyerahkan/mengklaim kupon ini")
	}
	if status != "pending" {
		return nil, Invalid("kupon bundling berstatus '%s' tidak bisa diklaim lagi", status)
	}
	if expiresAt.Valid && expiresAt.Time.Before(time.Now().UTC()) {
		_, _ = tx.Exec(`UPDATE cross_outlet_bundle_claims SET status='expired', updated_at=NOW() WHERE id=$1`, strings.TrimSpace(claimID))
		_ = tx.Commit()
		return nil, Invalid("kupon bundling ini sudah melewati masa berlaku (kedaluwarsa)")
	}

	// Potong stok resep di gudang outlet mitra bila produk mitra sudah punya resep/stok langsung.
	stockDeducted := false
	stockNote := ""
	if err := lockPOSStock(tx, partnerOutlet); err != nil {
		return nil, err
	}
	lookupID := strings.TrimSpace(partnerLocal)
	if lookupID == "" {
		lookupID = partnerProdID
	}
	p, rerr := resolvePOSProduct(tx, partnerOutlet, posStockItem{ProductID: lookupID, Name: partnerName})
	if rerr != nil {
		// Produk tidak ditemukan atau resep belum lengkap — catat secara transparan
		// tanpa memblokir penyerahan fisik bila resep memang belum disusun.
		stockNote = "Stok tidak dipotong otomatis: " + rerr.Error()
	} else if len(p.materials) == 0 {
		stockNote = "Produk mitra belum memiliki resep/tautan stok; stok belum terpotong otomatis"
	} else {
		mids, cerr := consumePOSProduct(
			tx, partnerOutlet, p, partnerQty,
			strings.TrimSpace(claimID), "bundle_claim", claimCode,
			strings.TrimSpace(actor), "Penyerahan bundling "+claimCode+" ("+sourceName+")", "sale",
		)
		if cerr != nil {
			return nil, fmt.Errorf("gagal memotong stok gudang mitra: %w", cerr)
		}
		if len(mids) > 0 {
			stockDeducted = true
			stockNote = fmt.Sprintf("Stok resep terpotong otomatis (%d bahan)", len(mids))
		}
	}

	extraNote := strings.TrimSpace(req.Notes)
	if _, err := tx.Exec(`
		UPDATE cross_outlet_bundle_claims
		SET status = 'claimed', claimed_qty = partner_qty, claimed_at = (now() AT TIME ZONE 'UTC'),
		    claimed_by = $2, stock_deducted = $3, stock_deduction_note = $4,
		    notes = CASE WHEN $5 <> '' THEN TRIM(BOTH E'
' FROM COALESCE(notes,'') || E'
' || $5) ELSE notes END,
		    updated_at = NOW()
		WHERE id = $1`,
		strings.TrimSpace(claimID), strings.TrimSpace(actor), stockDeducted, stockNote, extraNote); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCrossOutletBundleClaim(claimID, outletScope)
}

func ExpireCrossOutletBundleClaim(claimID, reason, actor string, outletScope []string) (*models.CrossOutletBundleClaim, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var srcOutlet, partnerOutlet, status string
	err = tx.QueryRow(`SELECT TRIM(source_outlet_id), TRIM(partner_outlet_id), status FROM cross_outlet_bundle_claims WHERE id = $1 FOR UPDATE`, strings.TrimSpace(claimID)).
		Scan(&srcOutlet, &partnerOutlet, &status)
	if err == sql.ErrNoRows {
		return nil, Invalid("kupon bundling tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if outletScope != nil && !outletInScope(srcOutlet, outletScope) && !outletInScope(partnerOutlet, outletScope) {
		return nil, Invalid("kupon bundling di luar akses Anda")
	}
	if status != "pending" {
		return nil, Invalid("hanya kupon berstatus menunggu klaim yang bisa ditandai kedaluwarsa")
	}
	note := strings.TrimSpace(reason)
	if note == "" {
		note = "Ditandai kedaluwarsa oleh " + strings.TrimSpace(actor)
	}
	if _, err := tx.Exec(`UPDATE cross_outlet_bundle_claims SET status='expired', void_reason=$2, updated_at=NOW() WHERE id=$1`,
		strings.TrimSpace(claimID), note); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetCrossOutletBundleClaim(claimID, outletScope)
}
