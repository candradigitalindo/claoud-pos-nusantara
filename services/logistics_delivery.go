package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/lib/pq"
)

func normalizeDispatchMode(raw string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(raw))
	if m == "" {
		return "logistics", nil
	}
	switch m {
	case "logistics", "vendor_direct":
		return m, nil
	default:
		return "", Invalid("metode pengantaran tidak valid (pilih logistics atau vendor_direct)")
	}
}

func normalizeDeliveryDestType(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if d == "" {
		return "warehouse", nil
	}
	switch d {
	case "warehouse", "outlet", "asset_desk":
		return d, nil
	default:
		return "", Invalid("tipe tujuan pengantaran tidak valid")
	}
}

func normalizeDeliveryDate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Now().In(GetTimezoneLocation()).Format("2006-01-02"), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", Invalid("format tanggal pengantaran tidak valid (gunakan YYYY-MM-DD)")
	}
	return t.Format("2006-01-02"), nil
}

// activeDeliveryReservedQty menghitung total qty baris PR yang saat ini sedang
// terikat di dokumen Pengantaran Logistik aktif (belum selesai diterima/batal).
func activeDeliveryReservedQty(q receivingQuerier, srcPRID, itemKey, excludeDeliveryID string) (int, error) {
	var v sql.NullInt64
	err := q.QueryRow(`
		SELECT COALESCE(SUM(GREATEST(ldi.qty_planned - ldi.qty_received, 0)), 0)
		FROM logistics_delivery_items ldi
		JOIN logistics_deliveries ld ON ld.id = ldi.delivery_id
		WHERE ldi.purchase_request_id = $1
		  AND ldi.pr_item_key = $2
		  AND ld.status IN ('draft', 'in_transit', 'partial_received')
		  AND ($3 = '' OR ld.id <> $3)`,
		srcPRID, itemKey, strings.TrimSpace(excludeDeliveryID)).Scan(&v)
	if err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// activeDeliveryRefsForLine daftar Surat Jalan aktif yang membawa baris PR tertentu.
func activeDeliveryRefsForLine(q receivingQuerier, srcPRID, itemKey string) ([]models.LineDeliveryRef, error) {
	rows, err := q.Query(`
		SELECT ld.id, ld.delivery_number, ld.dispatch_mode, ld.status,
		       COALESCE(ld.courier_name, ''), GREATEST(ldi.qty_planned - ldi.qty_received, 0)
		FROM logistics_delivery_items ldi
		JOIN logistics_deliveries ld ON ld.id = ldi.delivery_id
		WHERE ldi.purchase_request_id = $1
		  AND ldi.pr_item_key = $2
		  AND ld.status IN ('draft', 'in_transit', 'partial_received')
		  AND ldi.qty_planned > ldi.qty_received
		ORDER BY ld.delivery_date DESC, ld.created_at DESC`, srcPRID, itemKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.LineDeliveryRef, 0)
	for rows.Next() {
		var r models.LineDeliveryRef
		if err := rows.Scan(&r.DeliveryID, &r.DeliveryNumber, &r.DispatchMode, &r.Status, &r.CourierName, &r.QtyPending); err != nil {
			return nil, err
		}
		r.DeliveryID = strings.TrimSpace(r.DeliveryID)
		out = append(out, r)
	}
	return out, rows.Err()
}

const deliveryGroupExpr = `COALESCE(NULLIF(ld.purchasing_group, ''), CASE WHEN EXISTS (SELECT 1 FROM logistics_delivery_items ldi_g WHERE ldi_g.delivery_id = ld.id AND ldi_g.request_type = 'bahan') THEN 'bahan_baku' ELSE 'barang_jasa' END)`

// GetLogisticsDeliveryGroup mengembalikan kelompok purchasing ('bahan_baku' atau 'barang_jasa') dari Surat Jalan.
func GetLogisticsDeliveryGroup(id string) (string, error) {
	id = strings.TrimSpace(id)
	var grp string
	err := database.DB.QueryRow(`SELECT `+deliveryGroupExpr+` FROM logistics_deliveries ld WHERE ld.id = $1`, id).Scan(&grp)
	if err == sql.ErrNoRows {
		return "", Invalid("pengantaran logistik tidak ditemukan")
	}
	if err != nil {
		return "", err
	}
	return grp, nil
}

// CheckLogisticsDeliveryGroupAccess memastikan Surat Jalan termasuk kelompok Purchasing yang boleh diakses pengguna.
func CheckLogisticsDeliveryGroupAccess(id string, allowedGroups []string) error {
	if allowedGroups == nil {
		return nil
	}
	grp, err := GetLogisticsDeliveryGroup(id)
	if err != nil {
		return err
	}
	for _, g := range allowedGroups {
		if g == grp {
			return nil
		}
	}
	return Invalid("Surat Jalan ini milik kelompok %s — di luar akses kelompok Purchasing Anda", PurchasingGroupLabel(grp))
}

// ListPRDeliveries mengembalikan ringkasan pengantaran logistik yang memuat item
// dari PR ini beserta keluarga induk/pecahannya (non-cancelled).
func ListPRDeliveries(q receivingQuerier, prID string) ([]models.PRDeliverySummary, error) {
	rows, err := q.Query(`
		SELECT ld.id, ld.delivery_number, TO_CHAR(ld.delivery_date, 'YYYY-MM-DD'),
		       `+deliveryGroupExpr+`, ld.dispatch_mode, ld.status, COALESCE(ld.courier_name, ''), COALESCE(ld.vehicle_info, ''),
		       COALESCE(ld.destination_label, ''),
		       COUNT(ldi.id)::int,
		       COALESCE(SUM(ldi.qty_planned), 0)::int,
		       COALESCE(SUM(ldi.qty_received), 0)::int,
		       COALESCE(ld.pickup_photo_urls, '[]'::jsonb),
		       COALESCE(ld.receive_photo_urls, '[]'::jsonb)
		FROM logistics_deliveries ld
		JOIN logistics_delivery_items ldi ON ldi.delivery_id = ld.id
		JOIN purchase_requests pr ON pr.id = ldi.purchase_request_id
		WHERE ld.status <> 'cancelled'
		  AND (pr.id = $1 OR pr.parent_id = $1
		       OR pr.id = (SELECT parent_id FROM purchase_requests WHERE id = $1))
		GROUP BY ld.id
		ORDER BY ld.delivery_date DESC, ld.created_at DESC`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.PRDeliverySummary, 0)
	for rows.Next() {
		var d models.PRDeliverySummary
		var pickupRaw, recvRaw []byte
		if err := rows.Scan(&d.ID, &d.DeliveryNumber, &d.DeliveryDate, &d.PurchasingGroup, &d.DispatchMode, &d.Status,
			&d.CourierName, &d.VehicleInfo, &d.DestinationLabel, &d.ItemCount, &d.TotalPlanned, &d.TotalReceived,
			&pickupRaw, &recvRaw); err != nil {
			return nil, err
		}
		d.ID = strings.TrimSpace(d.ID)
		d.PickupPhotoURLs = DecodeUploadURLs("", pickupRaw)
		d.ReceivePhotoURLs = DecodeUploadURLs("", recvRaw)
		out = append(out, d)
	}
	return out, rows.Err()
}

// attachPRDeliveries melampirkan ringkasan Pengantaran Logistik (non-cancelled)
// ke daftar PR dalam 1 query batch — mencakup PR mandiri, pecahan, maupun induk
// (induk mengumpulkan seluruh SJ milik dirinya dan pecahan-pecahannya).
func attachPRDeliveries(requests []models.PurchaseRequest) {
	if len(requests) == 0 {
		return
	}
	ids := make([]string, 0, len(requests))
	idxByID := make(map[string]int, len(requests))
	for i := range requests {
		id := strings.TrimSpace(requests[i].ID)
		if id != "" {
			ids = append(ids, id)
			idxByID[id] = i
		}
	}
	if len(ids) == 0 {
		return
	}
	rows, err := database.DB.Query(`
		WITH targets AS (
			SELECT UNNEST($1::text[]) AS target_id
		),
		matched AS (
			SELECT t.target_id, ldi.delivery_id, ldi.id AS item_id, ldi.qty_planned, ldi.qty_received
			FROM targets t
			JOIN purchase_requests pr ON pr.id = t.target_id OR pr.parent_id = t.target_id
			JOIN logistics_delivery_items ldi ON ldi.purchase_request_id = pr.id
		)
		SELECT m.target_id,
		       ld.id, ld.delivery_number, TO_CHAR(ld.delivery_date, 'YYYY-MM-DD'),
		       `+deliveryGroupExpr+`, ld.dispatch_mode, ld.status, COALESCE(ld.courier_name, ''), COALESCE(ld.vehicle_info, ''),
		       COALESCE(ld.destination_label, ''),
		       COUNT(DISTINCT m.item_id)::int,
		       COALESCE(SUM(m.qty_planned), 0)::int,
		       COALESCE(SUM(m.qty_received), 0)::int,
		       COALESCE(ld.pickup_photo_urls, '[]'::jsonb),
		       COALESCE(ld.receive_photo_urls, '[]'::jsonb)
		FROM matched m
		JOIN logistics_deliveries ld ON ld.id = m.delivery_id
		WHERE ld.status <> 'cancelled'
		GROUP BY m.target_id, ld.id
		ORDER BY ld.delivery_date DESC, ld.created_at DESC`, pq.Array(ids))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var targetID string
		var d models.PRDeliverySummary
		var pickupRaw, recvRaw []byte
		if err := rows.Scan(
			&targetID, &d.ID, &d.DeliveryNumber, &d.DeliveryDate, &d.PurchasingGroup, &d.DispatchMode, &d.Status,
			&d.CourierName, &d.VehicleInfo, &d.DestinationLabel,
			&d.ItemCount, &d.TotalPlanned, &d.TotalReceived, &pickupRaw, &recvRaw,
		); err != nil {
			continue
		}
		targetID = strings.TrimSpace(targetID)
		d.ID = strings.TrimSpace(d.ID)
		d.PickupPhotoURLs = DecodeUploadURLs("", pickupRaw)
		d.ReceivePhotoURLs = DecodeUploadURLs("", recvRaw)
		if idx, ok := idxByID[targetID]; ok {
			requests[idx].Deliveries = append(requests[idx].Deliveries, d)
		}
	}
}

// ListDispatchablePRItems mengembalikan seluruh baris item pengadaan (Barang & Bahan Baku)
// yang sudah disetujui dan masih memiliki kuota bebas (qty_available > 0) untuk
// dimasukkan ke Pengantaran Logistik / Surat Jalan.
func ListDispatchablePRItems(outletScope []string, allowedTypes []string, filterPRID, filterReqType, filterKind, search string) ([]models.DispatchablePRItem, error) {
	q := `
		SELECT pr.id, pr.request_number, pr.request_type, pr.status,
		       COALESCE(pr.outlet_id, ''), COALESCE(o.name, ''),
		       COALESCE(pr.project_id, ''), COALESCE(pr.vendor_name, ''),
		       COALESCE(pr.goods_kind, ''), pr.items
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE pr.request_type IN ('barang', 'bahan')
		  AND ` + prCommittedCond("pr") + `
		  AND COALESCE(jsonb_array_length(pr.items), 0) > 0`
	args := []interface{}{}
	if outletScope != nil {
		args = append(args, pqStringArray(outletScope))
		q += fmt.Sprintf(` AND pr.outlet_id = ANY($%d::text[])`, len(args))
	}
	if allowedTypes != nil {
		args = append(args, pqStringArray(allowedTypes))
		q += fmt.Sprintf(` AND pr.request_type = ANY($%d::text[])`, len(args))
	}
	if pid := strings.TrimSpace(filterPRID); pid != "" {
		args = append(args, pid)
		q += fmt.Sprintf(` AND (pr.id = $%d OR pr.parent_id = $%d)`, len(args), len(args))
	}
	if rt := strings.TrimSpace(filterReqType); rt != "" {
		switch rt {
		case PurchasingGroupBahanBaku:
			rt = RequestTypeBahan
		case PurchasingGroupBarangJasa:
			rt = RequestTypeBarang
		}
		args = append(args, rt)
		q += fmt.Sprintf(` AND pr.request_type = $%d`, len(args))
	}
	q += ` ORDER BY pr.created_at DESC LIMIT 250`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	type prRow struct {
		id, num, reqType, status, outletID, outletName, projectID, vendorName, goodsKind string
		items                                                                            []models.PurchaseRequestItem
	}
	prs := []prRow{}
	for rows.Next() {
		var r prRow
		var raw []byte
		if err := rows.Scan(&r.id, &r.num, &r.reqType, &r.status, &r.outletID, &r.outletName,
			&r.projectID, &r.vendorName, &r.goodsKind, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		r.id = strings.TrimSpace(r.id)
		r.outletID = strings.TrimSpace(r.outletID)
		r.projectID = strings.TrimSpace(r.projectID)
		items, err := parsePRItems(raw)
		if err != nil {
			continue
		}
		r.items = items
		prs = append(prs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(search))
	kindFilter := strings.ToLower(strings.TrimSpace(filterKind))
	out := make([]models.DispatchablePRItem, 0)
	for _, pr := range prs {
		_, targetWhName, _ := targetWarehouseFor(database.DB, pr.id)
		err := forEachPRLine(pr.items, func(entry models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			purchased := sub.PurchaseQuantity()
			if purchased <= 0 {
				return nil
			}
			recorded, err := recordedQty(database.DB, pr.id, key)
			if err != nil {
				return err
			}
			inDel, err := activeDeliveryReservedQty(database.DB, pr.id, key, "")
			if err != nil {
				return err
			}
			avail := purchased - recorded - inDel
			if avail <= 0 {
				return nil
			}
			lk, err := lineKindOf(database.DB, pr.id, pr.reqType, pr.goodsKind, key, sub)
			if err != nil {
				return err
			}
			if kindFilter != "" && lk != kindFilter {
				return nil
			}
			if needle != "" {
				hay := strings.ToLower(pr.num + " " + pr.vendorName + " " + pr.outletName + " " + entry.Name + " " + sub.Name)
				if !strings.Contains(hay, needle) {
					return nil
				}
			}
			out = append(out, models.DispatchablePRItem{
				PurchaseRequestID:   pr.id,
				PRNumber:            pr.num,
				RequestType:         pr.reqType,
				PurchasingGroup:     PurchasingGroupOfRequestType(pr.reqType),
				Status:              pr.status,
				OutletID:            pr.outletID,
				OutletName:          pr.outletName,
				ProjectID:           pr.projectID,
				VendorName:          pr.vendorName,
				TargetWarehouseName: targetWhName,
				PRItemKey:           key,
				EntryName:           entry.Name,
				ItemName:            sub.Name,
				Unit:                sub.Unit,
				LineKind:            lk,
				UnitPrice:           effectiveUnitPrice(sub),
				QtyPurchased:        purchased,
				QtyReceived:         recorded,
				QtyInDelivery:       inDel,
				QtyAvailable:        avail,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func nextLogisticsDeliveryNumberTx(tx *sql.Tx, dateStr string) (string, error) {
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(774219801)`); err != nil {
		return "", err
	}
	compact := strings.ReplaceAll(dateStr, "-", "")
	prefix := "SJ-" + compact + "-"
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM logistics_deliveries WHERE delivery_number LIKE $1`, prefix+"%").Scan(&count); err != nil {
		return "", err
	}
	for seq := count + 1; ; seq++ {
		cand := fmt.Sprintf("%s%03d", prefix, seq)
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM logistics_deliveries WHERE delivery_number = $1)`, cand).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return cand, nil
		}
	}
}

func resolveDestinationLabelTx(tx *sql.Tx, destType, whID, outletID, explicit string) (string, error) {
	label := strings.TrimSpace(explicit)
	if label != "" {
		return label, nil
	}
	switch destType {
	case "warehouse":
		if whID != "" {
			var name string
			if err := tx.QueryRow(`SELECT name FROM warehouses WHERE id = $1`, whID).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
				return name, nil
			}
		}
		return "Gudang Induk", nil
	case "outlet":
		if outletID != "" {
			var name string
			if err := tx.QueryRow(`SELECT name FROM outlets WHERE id = $1`, outletID).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
				return "Outlet " + name, nil
			}
		}
		return "Outlet Tujuan", nil
	case "asset_desk":
		return "Bagian Perlengkapan / Aset", nil
	default:
		return "Gudang Induk", nil
	}
}

func deliveryInScope(id string, outletScope []string) bool {
	if outletScope == nil {
		return true
	}
	if len(outletScope) == 0 {
		return false
	}
	var ok bool
	err := database.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM logistics_deliveries ld
			LEFT JOIN warehouses w ON w.id = ld.destination_warehouse_id
			WHERE ld.id = $1
			  AND (
				ld.destination_outlet_id = ANY($2::text[])
				OR w.outlet_id = ANY($2::text[])
				OR EXISTS (
					SELECT 1 FROM logistics_delivery_items ldi
					JOIN purchase_requests pr ON pr.id = ldi.purchase_request_id
					WHERE ldi.delivery_id = ld.id AND pr.outlet_id = ANY($2::text[])
				)
			  )
		)`, id, pq.Array(outletScope)).Scan(&ok)
	return err == nil && ok
}

func ListLogisticsDeliveries(status, dispatchMode, dateFrom, dateTo, prID, search string, outletScope []string, page, limit int) ([]models.LogisticsDelivery, int, error) {
	return ListLogisticsDeliveriesFiltered(status, dispatchMode, "", dateFrom, dateTo, prID, search, outletScope, nil, page, limit)
}

func ListLogisticsDeliveriesFiltered(status, dispatchMode, purchasingGroup, dateFrom, dateTo, prID, search string, outletScope []string, allowedGroups []string, page, limit int) ([]models.LogisticsDelivery, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	where := []string{"1=1"}
	args := []interface{}{}
	addArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if allowedGroups != nil {
		if len(allowedGroups) == 0 {
			return []models.LogisticsDelivery{}, 0, nil
		}
		where = append(where, deliveryGroupExpr+" = ANY("+addArg(pq.Array(allowedGroups))+"::text[])")
	}
	if pg := strings.TrimSpace(purchasingGroup); pg != "" {
		where = append(where, deliveryGroupExpr+" = "+addArg(pg))
	}
	if outletScope != nil {
		if len(outletScope) == 0 {
			return []models.LogisticsDelivery{}, 0, nil
		}
		p := addArg(pq.Array(outletScope))
		where = append(where, fmt.Sprintf(`(
			ld.destination_outlet_id = ANY(%s::text[])
			OR w.outlet_id = ANY(%s::text[])
			OR EXISTS (
				SELECT 1 FROM logistics_delivery_items ldi2
				JOIN purchase_requests pr2 ON pr2.id = ldi2.purchase_request_id
				WHERE ldi2.delivery_id = ld.id AND pr2.outlet_id = ANY(%s::text[])
			)
		)`, p, p, p))
	}
	if s := strings.TrimSpace(status); s != "" {
		where = append(where, "ld.status = "+addArg(s))
	}
	if dm := strings.TrimSpace(dispatchMode); dm != "" {
		where = append(where, "ld.dispatch_mode = "+addArg(dm))
	}
	if df := strings.TrimSpace(dateFrom); df != "" {
		if _, err := time.Parse("2006-01-02", df); err == nil {
			where = append(where, "ld.delivery_date >= "+addArg(df)+"::date")
		}
	}
	if dt := strings.TrimSpace(dateTo); dt != "" {
		if _, err := time.Parse("2006-01-02", dt); err == nil {
			where = append(where, "ld.delivery_date <= "+addArg(dt)+"::date")
		}
	}
	if pid := strings.TrimSpace(prID); pid != "" {
		p := addArg(pid)
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM logistics_delivery_items ldi3 WHERE ldi3.delivery_id = ld.id AND ldi3.purchase_request_id = %s)`, p))
	}
	if q := strings.TrimSpace(search); q != "" {
		p := addArg("%" + strings.ToLower(q) + "%")
		where = append(where, fmt.Sprintf(`(
			LOWER(ld.delivery_number) LIKE %s
			OR LOWER(ld.courier_name) LIKE %s
			OR LOWER(ld.destination_label) LIKE %s
			OR LOWER(ld.origin_label) LIKE %s
			OR EXISTS (
				SELECT 1 FROM logistics_delivery_items ldi4
				WHERE ldi4.delivery_id = ld.id
				  AND (LOWER(ldi4.pr_number) LIKE %s OR LOWER(ldi4.item_name) LIKE %s OR LOWER(ldi4.vendor_name) LIKE %s)
			)
		)`, p, p, p, p, p, p, p))
	}

	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := database.DB.QueryRow(`
		SELECT COUNT(*) FROM logistics_deliveries ld
		LEFT JOIN warehouses w ON w.id = ld.destination_warehouse_id
		WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	limP := addArg(limit)
	offP := addArg(offset)
	rows, err := database.DB.Query(`
		SELECT ld.id, ld.delivery_number, TO_CHAR(ld.delivery_date, 'YYYY-MM-DD'),
		       `+deliveryGroupExpr+`, ld.dispatch_mode, ld.status,
		       COALESCE(ld.courier_name, ''), COALESCE(ld.vehicle_info, ''), COALESCE(ld.origin_label, ''),
		       ld.destination_type, COALESCE(ld.destination_warehouse_id, ''), COALESCE(ld.destination_outlet_id, ''),
		       COALESCE(ld.destination_label, ''),
		       COALESCE(ld.pickup_photo_urls, '[]'::jsonb), COALESCE(ld.receive_photo_urls, '[]'::jsonb),
		       COALESCE(ld.notes, ''), COALESCE(ld.created_by, ''),
		       COALESCE(ld.dispatched_by, ''), ld.dispatched_at,
		       COALESCE(ld.received_by, ''), ld.received_at,
		       COALESCE(ld.cancelled_by, ''), ld.cancelled_at, COALESCE(ld.cancel_reason, ''),
		       ld.created_at, ld.updated_at,
		       COALESCE((SELECT COUNT(*)::int FROM logistics_delivery_items ldi WHERE ldi.delivery_id = ld.id), 0),
		       COALESCE((SELECT SUM(ldi.qty_planned)::int FROM logistics_delivery_items ldi WHERE ldi.delivery_id = ld.id), 0),
		       COALESCE((SELECT SUM(ldi.qty_received)::int FROM logistics_delivery_items ldi WHERE ldi.delivery_id = ld.id), 0),
		       COALESCE((SELECT jsonb_agg(DISTINCT ldi.pr_number) FROM logistics_delivery_items ldi WHERE ldi.delivery_id = ld.id AND ldi.pr_number <> ''), '[]'::jsonb)
		FROM logistics_deliveries ld
		LEFT JOIN warehouses w ON w.id = ld.destination_warehouse_id
		WHERE `+whereSQL+`
		ORDER BY ld.delivery_date DESC, ld.created_at DESC
		LIMIT `+limP+` OFFSET `+offP, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.LogisticsDelivery, 0)
	for rows.Next() {
		var d models.LogisticsDelivery
		var pickupRaw, recvRaw, prNumsRaw []byte
		var dispAt, recAt, cancAt sql.NullTime
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&d.ID, &d.DeliveryNumber, &d.DeliveryDate, &d.PurchasingGroup, &d.DispatchMode, &d.Status,
			&d.CourierName, &d.VehicleInfo, &d.OriginLabel,
			&d.DestinationType, &d.DestinationWarehouseID, &d.DestinationOutletID, &d.DestinationLabel,
			&pickupRaw, &recvRaw, &d.Notes, &d.CreatedBy,
			&d.DispatchedBy, &dispAt, &d.ReceivedBy, &recAt, &d.CancelledBy, &cancAt, &d.CancelReason,
			&createdAt, &updatedAt,
			&d.ItemCount, &d.TotalPlanned, &d.TotalReceived, &prNumsRaw,
		); err != nil {
			return nil, 0, err
		}
		d.ID = strings.TrimSpace(d.ID)
		d.DestinationWarehouseID = strings.TrimSpace(d.DestinationWarehouseID)
		d.DestinationOutletID = strings.TrimSpace(d.DestinationOutletID)
		d.PickupPhotoURLs = DecodeUploadURLs("", pickupRaw)
		d.ReceivePhotoURLs = DecodeUploadURLs("", recvRaw)
		if dispAt.Valid {
			d.DispatchedAt = dispAt.Time.UTC().Format(time.RFC3339)
		}
		if recAt.Valid {
			d.ReceivedAt = recAt.Time.UTC().Format(time.RFC3339)
		}
		if cancAt.Valid {
			d.CancelledAt = cancAt.Time.UTC().Format(time.RFC3339)
		}
		d.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		d.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		_ = json.Unmarshal(prNumsRaw, &d.PRNumbers)
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func GetLogisticsDelivery(id string, outletScope []string) (*models.LogisticsDelivery, error) {
	id = strings.TrimSpace(id)
	if outletScope != nil && !deliveryInScope(id, outletScope) {
		return nil, Invalid("pengantaran di luar akses Anda")
	}
	var d models.LogisticsDelivery
	var pickupRaw, recvRaw []byte
	var dispAt, recAt, cancAt sql.NullTime
	var createdAt, updatedAt time.Time
	err := database.DB.QueryRow(`
		SELECT ld.id, ld.delivery_number, TO_CHAR(ld.delivery_date, 'YYYY-MM-DD'),
		       `+deliveryGroupExpr+`, ld.dispatch_mode, ld.status,
		       COALESCE(ld.courier_name, ''), COALESCE(ld.courier_id, ''), COALESCE(ld.vehicle_info, ''), COALESCE(ld.origin_label, ''),
		       ld.destination_type, COALESCE(ld.destination_warehouse_id, ''), COALESCE(ld.destination_outlet_id, ''),
		       COALESCE(ld.destination_label, ''),
		       COALESCE(ld.pickup_photo_urls, '[]'::jsonb), COALESCE(ld.receive_photo_urls, '[]'::jsonb),
		       COALESCE(ld.notes, ''), COALESCE(ld.created_by, ''),
		       COALESCE(ld.dispatched_by, ''), ld.dispatched_at,
		       COALESCE(ld.received_by, ''), ld.received_at,
		       COALESCE(ld.cancelled_by, ''), ld.cancelled_at, COALESCE(ld.cancel_reason, ''),
		       ld.created_at, ld.updated_at
		FROM logistics_deliveries ld WHERE ld.id = $1`, id).Scan(
		&d.ID, &d.DeliveryNumber, &d.DeliveryDate, &d.PurchasingGroup, &d.DispatchMode, &d.Status,
		&d.CourierName, &d.CourierID, &d.VehicleInfo, &d.OriginLabel,
		&d.DestinationType, &d.DestinationWarehouseID, &d.DestinationOutletID, &d.DestinationLabel,
		&pickupRaw, &recvRaw, &d.Notes, &d.CreatedBy,
		&d.DispatchedBy, &dispAt, &d.ReceivedBy, &recAt, &d.CancelledBy, &cancAt, &d.CancelReason,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, Invalid("pengantaran logistik tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	d.ID = strings.TrimSpace(d.ID)
	d.CourierID = strings.TrimSpace(d.CourierID)
	d.DestinationWarehouseID = strings.TrimSpace(d.DestinationWarehouseID)
	d.DestinationOutletID = strings.TrimSpace(d.DestinationOutletID)
	d.PickupPhotoURLs = DecodeUploadURLs("", pickupRaw)
	d.ReceivePhotoURLs = DecodeUploadURLs("", recvRaw)
	if dispAt.Valid {
		d.DispatchedAt = dispAt.Time.UTC().Format(time.RFC3339)
	}
	if recAt.Valid {
		d.ReceivedAt = recAt.Time.UTC().Format(time.RFC3339)
	}
	if cancAt.Valid {
		d.CancelledAt = cancAt.Time.UTC().Format(time.RFC3339)
	}
	d.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	d.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	d.ReceivingWarehouseID = defaultReceivingWarehouse(database.DB)
	d.CapitalizationMin = settingFloat("asset_capitalization_min", defaultCapitalizationMin)

	rows, err := database.DB.Query(`
		SELECT ldi.id, ldi.delivery_id, ldi.purchase_request_id, ldi.pr_number, ldi.request_type,
		       ldi.vendor_name, COALESCE(pr.outlet_id, ''), COALESCE(o.name, ''), COALESCE(pr.project_id, ''),
		       ldi.pr_item_key, ldi.entry_name, ldi.item_name, ldi.unit, ldi.line_kind,
		       ldi.qty_planned, ldi.qty_received, ldi.notes, pr.items
		FROM logistics_delivery_items ldi
		JOIN purchase_requests pr ON pr.id = ldi.purchase_request_id
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		WHERE ldi.delivery_id = $1
		ORDER BY ldi.pr_number, ldi.created_at, ldi.id`, d.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]models.LogisticsDeliveryItem, 0)
	prNumSet := map[string]bool{}
	for rows.Next() {
		var it models.LogisticsDeliveryItem
		var rawItems []byte
		if err := rows.Scan(
			&it.ID, &it.DeliveryID, &it.PurchaseRequestID, &it.PRNumber, &it.RequestType,
			&it.VendorName, &it.OutletID, &it.OutletName, &it.ProjectID,
			&it.PRItemKey, &it.EntryName, &it.ItemName, &it.Unit, &it.LineKind,
			&it.QtyPlanned, &it.QtyReceived, &it.Notes, &rawItems,
		); err != nil {
			return nil, err
		}
		it.ID = strings.TrimSpace(it.ID)
		it.DeliveryID = strings.TrimSpace(it.DeliveryID)
		it.PurchaseRequestID = strings.TrimSpace(it.PurchaseRequestID)
		it.OutletID = strings.TrimSpace(it.OutletID)
		it.ProjectID = strings.TrimSpace(it.ProjectID)
		it.Remaining = it.QtyPlanned - it.QtyReceived
		if it.Remaining < 0 {
			it.Remaining = 0
		}
		// Cari harga satuan terbaru dari PR sumber
		if parsed, perr := parsePRItems(rawItems); perr == nil {
			_ = forEachPRLine(parsed, func(_ models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
				if key == it.PRItemKey {
					it.UnitPrice = effectiveUnitPrice(sub)
				}
				return nil
			})
		}
		switch {
		case it.RequestType == RequestTypeBahan || it.LineKind == "dapur":
			it.Destination = "stok"
			if sid, sname, ok, cerr := catalogMatch(database.DB, it.ItemName); cerr == nil && ok {
				it.SuggestedStockItemID, it.SuggestedStockItemName = sid, sname
			}
		case it.UnitPrice >= d.CapitalizationMin:
			it.Destination = "aset"
		case it.ProjectID != "":
			it.Destination = "material"
		default:
			it.Destination = "habis"
		}
		d.Items = append(d.Items, it)
		d.ItemCount++
		d.TotalPlanned += it.QtyPlanned
		d.TotalReceived += it.QtyReceived
		if it.PRNumber != "" && !prNumSet[it.PRNumber] {
			prNumSet[it.PRNumber] = true
			d.PRNumbers = append(d.PRNumbers, it.PRNumber)
		}
	}
	return &d, rows.Err()
}

type resolvedDeliveryItem struct {
	prID, prNumber, reqType, vendorName, prItemKey, entryName, itemName, unit, lineKind, notes string
	qtyPlanned                                                                                 int
}

func resolveAndValidateDeliveryItemsTx(tx *sql.Tx, items []models.LogisticsDeliveryItemInput, excludeDeliveryID string, outletScope []string, allowedTypes []string) ([]resolvedDeliveryItem, string, error) {
	if len(items) == 0 {
		return nil, "", Invalid("tambahkan minimal 1 barang untuk pengantaran logistik")
	}

	// Kumpulkan & kunci seluruh keluarga PR sumber secara terurut agar bebas deadlock.
	prIDSet := map[string]bool{}
	seenPair := map[[2]string]bool{}
	for _, in := range items {
		pid := strings.TrimSpace(in.PurchaseRequestID)
		key := strings.TrimSpace(in.PRItemKey)
		if pid == "" || key == "" {
			return nil, "", Invalid("setiap baris wajib menunjuk pengadaan dan item yang sah")
		}
		if in.QtyPlanned <= 0 {
			return nil, "", Invalid("jumlah barang yang diantar harus lebih dari 0")
		}
		pair := [2]string{pid, key}
		if seenPair[pair] {
			return nil, "", Invalid("item yang sama dari pengadaan yang sama tidak boleh dimasukkan dua kali dalam satu pengantaran")
		}
		seenPair[pair] = true
		prIDSet[pid] = true
	}

	prIDs := make([]string, 0, len(prIDSet))
	for pid := range prIDSet {
		prIDs = append(prIDs, pid)
	}
	sort.Strings(prIDs)

	rootSet := map[string]bool{}
	for _, pid := range prIDs {
		var parent sql.NullString
		if err := tx.QueryRow(`SELECT parent_id FROM purchase_requests WHERE id = $1`, pid).Scan(&parent); err != nil {
			if err == sql.ErrNoRows {
				return nil, "", Invalid("pengadaan '%s' tidak ditemukan", pid)
			}
			return nil, "", err
		}
		root := pid
		if p := strings.TrimSpace(parent.String); p != "" {
			root = p
		}
		rootSet[root] = true
	}
	roots := make([]string, 0, len(rootSet))
	for r := range rootSet {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		if _, err := lockPurchaseFamilyTx(tx, r); err != nil {
			return nil, "", err
		}
	}

	type prInfo struct {
		number, reqType, status, outletID, vendorName, goodsKind string
		lines                                                    map[string]struct {
			entry, name, unit, kind string
			purchased               int
		}
	}
	prMap := map[string]*prInfo{}
	for _, pid := range prIDs {
		var info prInfo
		var raw []byte
		if err := tx.QueryRow(`
			SELECT request_number, request_type, status, COALESCE(outlet_id, ''),
			       COALESCE(vendor_name, ''), COALESCE(goods_kind, ''), items
			FROM purchase_requests WHERE id = $1`, pid).Scan(
			&info.number, &info.reqType, &info.status, &info.outletID, &info.vendorName, &info.goodsKind, &raw,
		); err != nil {
			return nil, "", err
		}
		info.outletID = strings.TrimSpace(info.outletID)
		if info.reqType == RequestTypeJasa {
			return nil, "", Invalid("%s: pengajuan jasa tidak memakai pengantaran logistik barang", info.number)
		}
		if !IsCommittedPurchaseStatus(info.status) {
			return nil, "", Invalid("%s: pengajuan belum disetujui atau sudah dibatalkan", info.number)
		}
		if outletScope != nil && (info.outletID == "" || !outletInScope(info.outletID, outletScope)) {
			return nil, "", Invalid("%s: pengajuan di luar akses outlet Anda", info.number)
		}
		if allowedTypes != nil {
			ok := false
			for _, t := range allowedTypes {
				if t == info.reqType {
					ok = true
					break
				}
			}
			if !ok {
				return nil, "", Invalid("%s: Anda tidak memiliki izin untuk jenis pengadaan ini", info.number)
			}
		}
		parsed, err := parsePRItems(raw)
		if err != nil {
			return nil, "", err
		}
		info.lines = map[string]struct {
			entry, name, unit, kind string
			purchased               int
		}{}
		if err := forEachPRLine(parsed, func(entry models.PurchaseRequestItem, sub models.PurchaseSubItem, key string) error {
			lk, err := lineKindOf(tx, pid, info.reqType, info.goodsKind, key, sub)
			if err != nil {
				return err
			}
			info.lines[key] = struct {
				entry, name, unit, kind string
				purchased               int
			}{
				entry: entry.Name, name: sub.Name, unit: sub.Unit, kind: lk, purchased: sub.PurchaseQuantity(),
			}
			return nil
		}); err != nil {
			return nil, "", err
		}
		prMap[pid] = &info
	}

	resolved := make([]resolvedDeliveryItem, 0, len(items))
	for _, in := range items {
		pid := strings.TrimSpace(in.PurchaseRequestID)
		key := strings.TrimSpace(in.PRItemKey)
		info := prMap[pid]
		line, ok := info.lines[key]
		if !ok {
			return nil, "", Invalid("%s: baris item tidak ditemukan di pengadaan", info.number)
		}
		recorded, err := recordedQty(tx, pid, key)
		if err != nil {
			return nil, "", err
		}
		inDel, err := activeDeliveryReservedQty(tx, pid, key, excludeDeliveryID)
		if err != nil {
			return nil, "", err
		}
		avail := line.purchased - recorded - inDel
		if in.QtyPlanned > avail {
			return nil, "", Invalid("%s (%s): jumlah antar (%d) melebihi sisa kuota bebas (%d — sudah diterima %d, dalam pengantaran lain %d)",
				line.name, info.number, in.QtyPlanned, maxInt(avail, 0), recorded, inDel)
		}
		resolved = append(resolved, resolvedDeliveryItem{
			prID:       pid,
			prNumber:   info.number,
			reqType:    info.reqType,
			vendorName: info.vendorName,
			prItemKey:  key,
			entryName:  line.entry,
			itemName:   line.name,
			unit:       line.unit,
			lineKind:   line.kind,
			qtyPlanned: in.QtyPlanned,
			notes:      strings.TrimSpace(in.Notes),
		})
	}
	group := ""
	for _, r := range resolved {
		g := PurchasingGroupOfRequestType(r.reqType)
		if group == "" {
			group = g
		} else if group != g {
			return nil, "", Invalid("Item Bahan Baku dan Barang/Perlengkapan tidak boleh dicampur dalam satu Surat Jalan — buat Surat Jalan terpisah sesuai kelompok Purchasing")
		}
	}
	return resolved, group, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func CreateLogisticsDelivery(in models.CreateLogisticsDeliveryInput, actor Actor, outletScope []string, allowedTypes []string) (*models.LogisticsDelivery, error) {
	mode, err := normalizeDispatchMode(in.DispatchMode)
	if err != nil {
		return nil, err
	}
	dateStr, err := normalizeDeliveryDate(in.DeliveryDate)
	if err != nil {
		return nil, err
	}
	destType, err := normalizeDeliveryDestType(in.DestinationType)
	if err != nil {
		return nil, err
	}
	pickupPhotos, err := NormalizeUploadURLs("foto pengantaran", "", in.PickupPhotoURLs)
	if err != nil {
		return nil, err
	}
	receivePhotos, err := NormalizeUploadURLs("foto penerimaan", "", in.ReceivePhotoURLs)
	if err != nil {
		return nil, err
	}
	courierName := strings.TrimSpace(in.CourierName)
	if mode == "logistics" && courierName == "" {
		courierName = actor.Name
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	resolved, purchasingGroup, err := resolveAndValidateDeliveryItemsTx(tx, in.Items, "", outletScope, allowedTypes)
	if err != nil {
		return nil, err
	}
	delNum, err := nextLogisticsDeliveryNumberTx(tx, dateStr)
	if err != nil {
		return nil, err
	}
	whID := strings.TrimSpace(in.DestinationWarehouseID)
	if destType == "warehouse" && whID == "" {
		whID = defaultReceivingWarehouse(tx)
	}
	outletID := strings.TrimSpace(in.DestinationOutletID)
	destLabel, err := resolveDestinationLabelTx(tx, destType, whID, outletID, in.DestinationLabel)
	if err != nil {
		return nil, err
	}

	id := NewULID()
	now := time.Now().UTC()
	status := "draft"
	var dispBy string
	var dispAt interface{}
	if in.DispatchNow {
		status = "in_transit"
		dispBy = actor.Name
		dispAt = now
	}
	if _, err := tx.Exec(`
		INSERT INTO logistics_deliveries (
			id, delivery_number, delivery_date, purchasing_group, dispatch_mode, status,
			courier_name, vehicle_info, origin_label,
			destination_type, destination_warehouse_id, destination_outlet_id, destination_label,
			pickup_photo_urls, receive_photo_urls, notes,
			created_by, created_by_id, dispatched_by, dispatched_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3::date, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13,
			$14::jsonb, $15::jsonb, $16,
			$17, $18, $19, $20,
			$21, $21
		)`,
		id, delNum, dateStr, purchasingGroup, mode, status,
		courierName, strings.TrimSpace(in.VehicleInfo), strings.TrimSpace(in.OriginLabel),
		destType, nilIfEmpty(whID), nilIfEmpty(outletID), destLabel,
		MarshalUploadURLs(pickupPhotos), MarshalUploadURLs(receivePhotos), strings.TrimSpace(in.Notes),
		actor.Name, nilIfEmpty(validActorID(actor.ID)), dispBy, dispAt,
		now,
	); err != nil {
		return nil, err
	}

	for _, it := range resolved {
		if _, err := tx.Exec(`
			INSERT INTO logistics_delivery_items (
				id, delivery_id, purchase_request_id, pr_number, request_type, vendor_name,
				pr_item_key, entry_name, item_name, unit, line_kind, qty_planned, qty_received, notes, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,0,$13,$14)`,
			NewULID(), id, it.prID, it.prNumber, it.reqType, it.vendorName,
			it.prItemKey, it.entryName, it.itemName, it.unit, it.lineKind, it.qtyPlanned, it.notes, now,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetLogisticsDelivery(id, nil)
}

func UpdateLogisticsDelivery(id string, in models.UpdateLogisticsDeliveryInput, actor Actor, outletScope []string, allowedTypes []string) (*models.LogisticsDelivery, error) {
	id = strings.TrimSpace(id)
	if outletScope != nil && !deliveryInScope(id, outletScope) {
		return nil, Invalid("pengantaran di luar akses Anda")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var curStatus, curMode, curDate, curDestType string
	if err := tx.QueryRow(`SELECT status, dispatch_mode, TO_CHAR(delivery_date,'YYYY-MM-DD'), destination_type FROM logistics_deliveries WHERE id = $1 FOR UPDATE`, id).
		Scan(&curStatus, &curMode, &curDate, &curDestType); err != nil {
		if err == sql.ErrNoRows {
			return nil, Invalid("pengantaran tidak ditemukan")
		}
		return nil, err
	}
	if curStatus == "received" || curStatus == "cancelled" {
		return nil, Invalid("pengantaran yang sudah diterima atau dibatalkan tidak dapat diubah")
	}
	mode := curMode
	if strings.TrimSpace(in.DispatchMode) != "" {
		if mode, err = normalizeDispatchMode(in.DispatchMode); err != nil {
			return nil, err
		}
	}
	dateStr := curDate
	if strings.TrimSpace(in.DeliveryDate) != "" {
		if dateStr, err = normalizeDeliveryDate(in.DeliveryDate); err != nil {
			return nil, err
		}
	}
	destType := curDestType
	if strings.TrimSpace(in.DestinationType) != "" {
		if destType, err = normalizeDeliveryDestType(in.DestinationType); err != nil {
			return nil, err
		}
	}
	whID := strings.TrimSpace(in.DestinationWarehouseID)
	outletID := strings.TrimSpace(in.DestinationOutletID)
	destLabel, err := resolveDestinationLabelTx(tx, destType, whID, outletID, in.DestinationLabel)
	if err != nil {
		return nil, err
	}

	var pickupJSON, recvJSON interface{}
	if in.PickupPhotoURLs != nil {
		urls, uerr := NormalizeUploadURLs("foto pengantaran", "", *in.PickupPhotoURLs)
		if uerr != nil {
			return nil, uerr
		}
		pickupJSON = MarshalUploadURLs(urls)
	}
	if in.ReceivePhotoURLs != nil {
		urls, uerr := NormalizeUploadURLs("foto penerimaan", "", *in.ReceivePhotoURLs)
		if uerr != nil {
			return nil, uerr
		}
		recvJSON = MarshalUploadURLs(urls)
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(`
		UPDATE logistics_deliveries SET
			delivery_date = $2::date,
			dispatch_mode = $3,
			courier_name = $4,
			vehicle_info = $5,
			origin_label = $6,
			destination_type = $7,
			destination_warehouse_id = $8,
			destination_outlet_id = $9,
			destination_label = $10,
			pickup_photo_urls = COALESCE($11::jsonb, pickup_photo_urls),
			receive_photo_urls = COALESCE($12::jsonb, receive_photo_urls),
			notes = $13,
			updated_at = $14
		WHERE id = $1`,
		id, dateStr, mode, strings.TrimSpace(in.CourierName), strings.TrimSpace(in.VehicleInfo), strings.TrimSpace(in.OriginLabel),
		destType, nilIfEmpty(whID), nilIfEmpty(outletID), destLabel,
		pickupJSON, recvJSON, strings.TrimSpace(in.Notes), now,
	); err != nil {
		return nil, err
	}

	// Perubahan daftar item hanya boleh saat masih draft (belum berangkat & belum ada penerimaan).
	if len(in.Items) > 0 {
		if curStatus != "draft" {
			return nil, Invalid("daftar item hanya bisa diubah saat pengantaran masih berstatus Draft")
		}
		resolved, purchasingGroup, err := resolveAndValidateDeliveryItemsTx(tx, in.Items, id, outletScope, allowedTypes)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE logistics_deliveries SET purchasing_group = $2 WHERE id = $1`, id, purchasingGroup); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`DELETE FROM logistics_delivery_items WHERE delivery_id = $1`, id); err != nil {
			return nil, err
		}
		for _, it := range resolved {
			if _, err := tx.Exec(`
				INSERT INTO logistics_delivery_items (
					id, delivery_id, purchase_request_id, pr_number, request_type, vendor_name,
					pr_item_key, entry_name, item_name, unit, line_kind, qty_planned, qty_received, notes, created_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,0,$13,$14)`,
				NewULID(), id, it.prID, it.prNumber, it.reqType, it.vendorName,
				it.prItemKey, it.entryName, it.itemName, it.unit, it.lineKind, it.qtyPlanned, it.notes, now,
			); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetLogisticsDelivery(id, nil)
}

func UpdateLogisticsDeliveryStatus(id string, in models.UpdateLogisticsStatusInput, actor Actor, outletScope []string) (*models.LogisticsDelivery, error) {
	id = strings.TrimSpace(id)
	if outletScope != nil && !deliveryInScope(id, outletScope) {
		return nil, Invalid("pengantaran di luar akses Anda")
	}
	target := strings.ToLower(strings.TrimSpace(in.Status))
	if target != "in_transit" && target != "cancelled" {
		return nil, Invalid("status tujuan hanya mendukung 'in_transit' atau 'cancelled'")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var curStatus string
	if err := tx.QueryRow(`SELECT status FROM logistics_deliveries WHERE id = $1 FOR UPDATE`, id).Scan(&curStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, Invalid("pengantaran tidak ditemukan")
		}
		return nil, err
	}

	now := time.Now().UTC()
	switch target {
	case "in_transit":
		if curStatus != "draft" {
			return nil, Invalid("hanya pengantaran berstatus Draft yang bisa diberangkatkan")
		}
		var pickupJSON interface{}
		if in.PickupPhotoURLs != nil {
			urls, uerr := NormalizeUploadURLs("foto pengantaran", "", *in.PickupPhotoURLs)
			if uerr != nil {
				return nil, uerr
			}
			pickupJSON = MarshalUploadURLs(urls)
		}
		if _, err := tx.Exec(`
			UPDATE logistics_deliveries
			SET status = 'in_transit',
			    dispatched_by = $2,
			    dispatched_at = $3,
			    pickup_photo_urls = COALESCE($4::jsonb, pickup_photo_urls),
			    updated_at = $3
			WHERE id = $1`, id, actor.Name, now, pickupJSON); err != nil {
			return nil, err
		}

	case "cancelled":
		if curStatus == "received" || curStatus == "cancelled" {
			return nil, Invalid("pengantaran sudah selesai diterima atau dibatalkan")
		}
		var totalRecv int
		tx.QueryRow(`SELECT COALESCE(SUM(qty_received), 0) FROM logistics_delivery_items WHERE delivery_id = $1`, id).Scan(&totalRecv)
		if totalRecv > 0 {
			return nil, Invalid("pengantaran tidak bisa dibatalkan karena sebagian barang sudah diterima")
		}
		if _, err := tx.Exec(`
			UPDATE logistics_deliveries
			SET status = 'cancelled',
			    cancelled_by = $2,
			    cancelled_at = $3,
			    cancel_reason = $4,
			    updated_at = $3
			WHERE id = $1`, id, actor.Name, now, strings.TrimSpace(in.CancelReason)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetLogisticsDelivery(id, nil)
}

// UpdateLogisticsDeliveryPhotos memungkinkan menambah, menghapus per foto, atau
// mengganti daftar foto pengantaran maupun daftar foto penerimaan SEBELUM
// pengantaran berstatus final 'received'.
func UpdateLogisticsDeliveryPhotos(id string, in models.UpdateLogisticsPhotosInput, canEditPickup, canEditReceive bool, outletScope []string) (*models.LogisticsDelivery, error) {
	id = strings.TrimSpace(id)
	if outletScope != nil && !deliveryInScope(id, outletScope) {
		return nil, Invalid("pengantaran di luar akses Anda")
	}
	if in.PickupPhotoURLs == nil && in.ReceivePhotoURLs == nil {
		return nil, Invalid("tidak ada perubahan foto yang dikirim")
	}
	if in.PickupPhotoURLs != nil && !canEditPickup {
		return nil, Invalid("Anda tidak berwenang mengubah foto pengantaran")
	}
	if in.ReceivePhotoURLs != nil && !canEditReceive {
		return nil, Invalid("Anda tidak berwenang mengubah foto penerimaan")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRow(`SELECT status FROM logistics_deliveries WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return nil, Invalid("pengantaran tidak ditemukan")
		}
		return nil, err
	}
	if status == "received" || status == "cancelled" {
		return nil, Invalid("pengantaran sudah selesai diterima atau dibatalkan — daftar foto sudah terkunci")
	}

	var pickupJSON, recvJSON interface{}
	if in.PickupPhotoURLs != nil {
		urls, err := NormalizeUploadURLs("foto pengantaran", "", *in.PickupPhotoURLs)
		if err != nil {
			return nil, err
		}
		pickupJSON = MarshalUploadURLs(urls)
	}
	if in.ReceivePhotoURLs != nil {
		urls, err := NormalizeUploadURLs("foto penerimaan", "", *in.ReceivePhotoURLs)
		if err != nil {
			return nil, err
		}
		recvJSON = MarshalUploadURLs(urls)
	}

	if _, err := tx.Exec(`
		UPDATE logistics_deliveries
		SET pickup_photo_urls = COALESCE($2::jsonb, pickup_photo_urls),
		    receive_photo_urls = COALESCE($3::jsonb, receive_photo_urls),
		    updated_at = $4
		WHERE id = $1`, id, pickupJSON, recvJSON, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetLogisticsDelivery(id, nil)
}

// ReceiveLogisticsDelivery memproses penerimaan final barang sesuai isi Surat Jalan
// (bisa lintas beberapa PR sekaligus, atau bertahap antar meja Gudang vs Aset).
func ReceiveLogisticsDelivery(id string, req models.ReceiveLogisticsDeliveryInput, actor Actor, access ReceiveAccess, outletScope []string) (*models.ReceiveLogisticsDeliveryResult, error) {
	if !access.Any() {
		return nil, Invalid("Anda tidak berwenang mencatat penerimaan barang")
	}
	id = strings.TrimSpace(id)
	if outletScope != nil && !deliveryInScope(id, outletScope) {
		return nil, Invalid("pengantaran di luar akses Anda")
	}

	hasQty := false
	for _, l := range req.Lines {
		if l.Qty < 0 {
			return nil, Invalid("jumlah barang diterima tidak boleh negatif")
		}
		if l.Qty > 0 {
			hasQty = true
		}
	}
	if !hasQty {
		return nil, Invalid("isi jumlah barang yang diterima, minimal pada satu baris Surat Jalan")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var delNumber, delStatus, delMode, courierName string
	var pickupRaw, savedRecvRaw []byte
	if err := tx.QueryRow(`
		SELECT delivery_number, status, dispatch_mode, COALESCE(courier_name, ''),
		       COALESCE(pickup_photo_urls, '[]'::jsonb), COALESCE(receive_photo_urls, '[]'::jsonb)
		FROM logistics_deliveries WHERE id = $1 FOR UPDATE`, id).
		Scan(&delNumber, &delStatus, &delMode, &courierName, &pickupRaw, &savedRecvRaw); err != nil {
		if err == sql.ErrNoRows {
			return nil, Invalid("pengantaran tidak ditemukan")
		}
		return nil, err
	}
	if delStatus == "received" {
		return nil, Invalid("seluruh barang pada Surat Jalan %s sudah diterima", delNumber)
	}
	if delStatus == "cancelled" {
		return nil, Invalid("Surat Jalan %s sudah dibatalkan", delNumber)
	}
	if delStatus == "draft" {
		return nil, Invalid("Surat Jalan %s masih Draft — tandai 'Dalam Pengantaran' terlebih dahulu sebelum penerimaan", delNumber)
	}

	// Gabungkan foto kiriman payload atau pakai draft foto penerimaan yang tersimpan.
	var photoURLs []string
	if len(req.PhotoURLs) > 0 || strings.TrimSpace(req.PhotoURL) != "" {
		photoURLs, err = NormalizeUploadURLs("foto barang saat diterima", req.PhotoURL, req.PhotoURLs)
		if err != nil {
			return nil, err
		}
	} else {
		photoURLs = DecodeUploadURLs("", savedRecvRaw)
	}
	if len(photoURLs) == 0 {
		return nil, Invalid("foto barang saat diterima wajib diunggah")
	}

	// Baca semua baris Surat Jalan (terkunci bersama header).
	type delItem struct {
		id, prID, prNumber, prItemKey, itemName, lineKind string
		qtyPlanned, qtyReceived                           int
	}
	rows, err := tx.Query(`
		SELECT id, purchase_request_id, pr_number, pr_item_key, item_name, line_kind, qty_planned, qty_received
		FROM logistics_delivery_items WHERE delivery_id = $1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	itemByID := map[string]*delItem{}
	allItems := []*delItem{}
	for rows.Next() {
		it := &delItem{}
		if err := rows.Scan(&it.id, &it.prID, &it.prNumber, &it.prItemKey, &it.itemName, &it.lineKind, &it.qtyPlanned, &it.qtyReceived); err != nil {
			rows.Close()
			return nil, err
		}
		it.id = strings.TrimSpace(it.id)
		it.prID = strings.TrimSpace(it.prID)
		itemByID[it.id] = it
		allItems = append(allItems, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Validasi tiap baris penerimaan terhadap sisa Surat Jalan & kelompokkan per PR sumber.
	seenItem := map[string]bool{}
	linesByPR := map[string][]models.ReceiveGoodsLine{}
	prOrder := []string{}
	prOutletOverride := map[string]string{}
	qtyByItemID := map[string]int{}

	for _, in := range req.Lines {
		if in.Qty == 0 {
			continue
		}
		diID := strings.TrimSpace(in.DeliveryItemID)
		di, ok := itemByID[diID]
		if !ok {
			return nil, Invalid("baris Surat Jalan tidak ditemukan — muat ulang lalu coba lagi")
		}
		if seenItem[diID] {
			return nil, Invalid("%s: baris yang sama dikirim lebih dari sekali", di.itemName)
		}
		seenItem[diID] = true
		rem := di.qtyPlanned - di.qtyReceived
		if in.Qty > rem {
			return nil, Invalid("%s: jumlah diterima (%d) melebihi sisa pada Surat Jalan %s (%d)", di.itemName, in.Qty, delNumber, rem)
		}
		if _, exists := linesByPR[di.prID]; !exists {
			prOrder = append(prOrder, di.prID)
		}
		if oid := strings.TrimSpace(in.OutletID); oid != "" {
			prOutletOverride[di.prID] = oid
		}
		linesByPR[di.prID] = append(linesByPR[di.prID], models.ReceiveGoodsLine{
			PRItemKey:        di.prItemKey,
			SourcePRID:       di.prID,
			Destination:      in.Destination,
			Qty:              in.Qty,
			Reason:           in.Reason,
			AssetName:        in.AssetName,
			Category:         in.Category,
			TrackingMode:     in.TrackingMode,
			SerialNumber:     in.SerialNumber,
			Brand:            in.Brand,
			Model:            in.Model,
			Location:         in.Location,
			UsefulLifeMonths: in.UsefulLifeMonths,
			ResidualValue:    in.ResidualValue,
			StockItemID:      in.StockItemID,
			WarehouseID:      in.WarehouseID,
			UnitUsed:         in.UnitUsed,
			ExpiryDate:       in.ExpiryDate,
		})
		qtyByItemID[diID] = in.Qty
	}

	// Kunci seluruh projek & keluarga PR yang tersentuh secara terurut (bebas deadlock).
	sort.Strings(prOrder)
	projSet := map[string]bool{}
	rootSet := map[string]bool{}
	for _, pid := range prOrder {
		var projID, parentID sql.NullString
		if err := tx.QueryRow(`SELECT project_id, parent_id FROM purchase_requests WHERE id = $1`, pid).Scan(&projID, &parentID); err != nil {
			return nil, err
		}
		if p := strings.TrimSpace(projID.String); p != "" {
			projSet[p] = true
		}
		root := pid
		if p := strings.TrimSpace(parentID.String); p != "" {
			root = p
		}
		rootSet[root] = true
	}
	projs := make([]string, 0, len(projSet))
	for p := range projSet {
		projs = append(projs, p)
	}
	sort.Strings(projs)
	for _, p := range projs {
		if err := lockMaterialProject(tx, p); err != nil {
			return nil, err
		}
	}
	roots := make([]string, 0, len(rootSet))
	for r := range rootSet {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		if _, err := lockPurchaseFamilyTx(tx, r); err != nil {
			return nil, err
		}
	}

	handoverNote := fmt.Sprintf("Diterima sesuai Surat Jalan %s", delNumber)
	if delMode == "vendor_direct" {
		handoverNote += " (vendor langsung)"
	} else if courierName != "" {
		handoverNote += " (logistik: " + courierName + ")"
	}

	out := &models.ReceiveLogisticsDeliveryResult{
		DeliveryID:     id,
		DeliveryNumber: delNumber,
	}
	notifPRs := []struct{ id, msg string }{}

	for _, pid := range prOrder {
		oid := req.OutletID
		if o, ok := prOutletOverride[pid]; ok && o != "" {
			oid = o
		}
		subReq := models.ReceiveGoodsRequest{
			OutletID:  oid,
			Notes:     strings.TrimSpace(req.Notes),
			PhotoURL:  photoURLs[0],
			PhotoURLs: photoURLs,
			Desk:      req.Desk,
			Lines:     linesByPR[pid],
		}
		subRes, err := receiveGoodsInTx(tx, pid, subReq, actor, access, outletScope, receiveTxOpts{
			alreadyLocked:        true,
			skipDirectQuotaCheck: true,
			skipLegacyPickup:     true,
			handoverNoteOverride: handoverNote,
		})
		if err != nil {
			return nil, err
		}
		out.AssetsCreated += subRes.AssetsCreated
		out.GRNNumbers = append(out.GRNNumbers, subRes.GRNNumbers...)
		out.MaterialLines += subRes.MaterialLines
		out.ExpenseLines += subRes.ExpenseLines
		out.StockLinesQueued += subRes.StockLinesQueued
		notifPRs = append(notifPRs, struct{ id, msg string }{pid, subRes.Message})
	}
	out.GRNNumber = strings.Join(out.GRNNumbers, ", ")

	// Perbarui qty_received pada tiap baris Surat Jalan.
	for diID, q := range qtyByItemID {
		if _, err := tx.Exec(`UPDATE logistics_delivery_items SET qty_received = qty_received + $2 WHERE id = $1`, diID, q); err != nil {
			return nil, err
		}
		itemByID[diID].qtyReceived += q
	}

	// Hitung ulang status Surat Jalan.
	remainingItems := 0
	for _, it := range allItems {
		if it.qtyReceived < it.qtyPlanned {
			remainingItems++
		}
	}
	out.RemainingItems = remainingItems

	now := time.Now().UTC()
	newStatus := "partial_received"
	if remainingItems == 0 {
		newStatus = "received"
	}
	out.DeliveryStatus = newStatus

	// Simpan daftar foto penerimaan final (digabungkan dengan foto sebelumnya bila partial).
	mergedRecv := DecodeUploadURLs("", savedRecvRaw)
	seenURL := map[string]bool{}
	for _, u := range mergedRecv {
		seenURL[u] = true
	}
	for _, u := range photoURLs {
		if !seenURL[u] && len(mergedRecv) < maxMultiUploadCount {
			seenURL[u] = true
			mergedRecv = append(mergedRecv, u)
		}
	}

	if newStatus == "received" {
		if _, err := tx.Exec(`
			UPDATE logistics_deliveries
			SET status = 'received',
			    receive_photo_urls = $2::jsonb,
			    received_by = $3,
			    received_at = $4,
			    updated_at = $4
			WHERE id = $1`, id, MarshalUploadURLs(mergedRecv), actor.Name, now); err != nil {
			return nil, err
		}
		// Arsipkan foto pengambilan/pengantaran ke handover_photos saat selesai diterima.
		for _, pu := range DecodeUploadURLs("", pickupRaw) {
			SaveHandoverPhoto(tx, "ambil", "dapur", "logistics_delivery", id, delNumber, pu,
				"Foto pengantaran Surat Jalan "+delNumber, actor.Name)
		}
	} else {
		if _, err := tx.Exec(`
			UPDATE logistics_deliveries
			SET status = 'partial_received',
			    receive_photo_urls = $2::jsonb,
			    updated_at = $3
			WHERE id = $1`, id, MarshalUploadURLs(mergedRecv), now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	for _, np := range notifPRs {
		if pr, err := GetPurchaseRequest(np.id); err == nil {
			go NotifyPurchaseEvent(pr, "received", actor.Name, handoverNote+" · "+np.msg, 0)
		}
	}

	parts := []string{"Surat Jalan " + delNumber}
	if out.AssetsCreated > 0 {
		parts = append(parts, fmt.Sprintf("%d aset dicatat", out.AssetsCreated))
	}
	if out.GRNNumber != "" {
		parts = append(parts, "stok masuk lewat "+out.GRNNumber)
	}
	if out.MaterialLines > 0 {
		parts = append(parts, fmt.Sprintf("%d material projek", out.MaterialLines))
	}
	if out.ExpenseLines > 0 {
		parts = append(parts, fmt.Sprintf("%d habis pakai", out.ExpenseLines))
	}
	if out.StockLinesQueued > 0 {
		parts = append(parts, fmt.Sprintf("%d baris menunggu gudang", out.StockLinesQueued))
	}
	if remainingItems > 0 {
		parts = append(parts, fmt.Sprintf("%d baris SJ belum diterima", remainingItems))
	} else {
		parts = append(parts, "seluruh barang SJ diterima lengkap")
	}
	out.Message = strings.Join(parts, " · ")
	return out, nil
}
