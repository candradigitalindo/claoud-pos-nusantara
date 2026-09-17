package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Valid status transitions:
//   pending → approved → payment_requested → paid/partial → received
//   pending → rejected
//   pending/approved → cancelled
var validTransitions = map[string]map[string]string{
	"approve":         {"pending": "approved"},
	"reject":          {"pending": "rejected"},
	"request_payment": {"approved": "payment_requested"},
	"pay":             {"payment_requested": "paid", "partial": "paid"},
	// 'partial' ikut boleh diterima: barang yang baru dilunasi sebagian tetap
	// datang secara fisik. Sisa hutangnya tidak hilang karena perhitungan
	// hutang usaha memakai (total_final - paid_amount), bukan nama status.
	"receive": {"paid": "received", "partial": "received"},
	"cancel":  {"pending": "cancelled", "approved": "cancelled"},
}

// outstandingCond memilih baris yang masih menyisakan kewajiban bayar (tanpa
// alias tabel). Definisinya tinggal di procurement_finance.go bersama versi
// ber-alias, supaya halaman Pembayaran dan seluruh laporan keuangan tidak
// pernah lagi memakai tiga rumus hutang yang berbeda.
var outstandingCond = outstandingCondFor("")

// fullySplitMasterCond cocok untuk baris master yang seluruh itemnya sudah
// dipindah ke pecahan. Dokumen semacam itu tinggal cangkang: tidak boleh ikut
// dijumlahkan (dobel hitung dengan pecahannya) dan tidak boleh dibayar.
func fullySplitMasterCond(alias string) string {
	p := ""
	if alias != "" {
		p = alias + "."
	}
	return fmt.Sprintf("(%[1]ssplit_status = 'master' AND COALESCE(jsonb_array_length(%[1]sitems), 0) = 0)", p)
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// recalcItems menghitung ulang seluruh subtotal dan total grup di tempat, lalu
// mengembalikan total dokumen. Satu-satunya tempat rumus ini hidup — Create,
// Update, dan Split memakainya supaya hasilnya tidak mungkin berbeda.
func recalcItems(items []models.PurchaseRequestItem) (totalHps, totalFinal float64) {
	for i := range items {
		var groupHps, groupFinal float64
		for j := range items[i].Items {
			sub := &items[i].Items[j]
			sub.HpsSubtotal = float64(sub.Qty) * sub.HpsPrice
			sub.FinalSubtotal = float64(sub.Qty) * sub.FinalPrice
			groupHps += sub.HpsSubtotal
			groupFinal += sub.FinalSubtotal
		}
		items[i].HpsTotal = groupHps
		items[i].FinalTotal = groupFinal
		totalHps += groupHps
		totalFinal += groupFinal
	}
	return totalHps, totalFinal
}

// amountOf memilih nilai untuk total_amount: harga final bila sudah diisi,
// selain itu jatuh kembali ke HPS.
func amountOf(totalHps, totalFinal float64) float64 {
	if totalFinal == 0 {
		return totalHps
	}
	return totalFinal
}

// validateItems menolak dokumen pengadaan yang tidak masuk akal. Dipakai saat
// membuat maupun mengubah item — sebelumnya hanya jalur Create yang memvalidasi,
// sehingga qty 0/negatif dan harga negatif bisa masuk lewat endpoint update.
func validateItems(items []models.PurchaseRequestItem) error {
	if len(items) == 0 {
		return fmt.Errorf("minimal 1 pengadaan harus diisi")
	}
	for _, g := range items {
		if strings.TrimSpace(g.Name) == "" || len(g.Items) == 0 {
			return fmt.Errorf("setiap pengadaan harus memiliki nama dan minimal 1 item")
		}
		for _, s := range g.Items {
			if strings.TrimSpace(s.Name) == "" {
				return fmt.Errorf("nama item pada pengadaan %q wajib diisi", g.Name)
			}
			if s.Qty <= 0 {
				return fmt.Errorf("qty item %q harus lebih dari 0", s.Name)
			}
			if s.HpsPrice < 0 || s.FinalPrice < 0 {
				return fmt.Errorf("harga item %q tidak boleh negatif", s.Name)
			}
		}
	}
	return nil
}

// generateRequestNumber creates a sequential number in the format ddmmYYnnn.
// It queries existing records for today and increments.
func generateRequestNumber(t time.Time) (string, error) {
	prefix := t.Format("020106") // ddmmYY
	var maxSeq int
	err := database.DB.QueryRow(
		"SELECT COALESCE(MAX(CAST(SUBSTRING(request_number FROM 7) AS INTEGER)), 0) FROM purchase_requests WHERE request_number LIKE $1",
		prefix+"%",
	).Scan(&maxSeq)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, maxSeq+1), nil
}

// insertWithRequestNumber menjalankan insert memakai nomor pengajuan baru dan
// mengulang bila unique index menolaknya. Nomor dihitung dari MAX() tanpa lock,
// jadi dua pengajuan bersamaan bisa memperoleh nomor yang sama.
//
// insert dipanggil berkali-kali, jadi pemanggil yang bekerja di dalam transaksi
// wajib membungkus statement-nya dengan SAVEPOINT — di Postgres satu statement
// gagal membuat seluruh transaksi abort.
func insertWithRequestNumber(now time.Time, insert func(reqNumber string) error) error {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		reqNumber, err := generateRequestNumber(now)
		if err != nil {
			return fmt.Errorf("gagal generate nomor pengajuan: %w", err)
		}
		lastErr = insert(reqNumber)
		if lastErr == nil {
			return nil
		}
		if !strings.Contains(lastErr.Error(), "uq_purchase_requests_number") {
			return lastErr
		}
	}
	return lastErr
}

func ListPurchaseRequests(outletID, workUnitID, status, requestType, projectID, parentID string, excludeMasters bool, search string, scopeIDs []string, wuScopeIDs []string, page, limit int) (*models.PurchaseRequestListResponse, error) {
	// Normalize outlet filter
	var filterIDs []string
	if outletID != "" {
		filterIDs = []string{outletID}
	} else if scopeIDs != nil {
		filterIDs = scopeIDs
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if requestType != "" {
		where += fmt.Sprintf(" AND pr.request_type = $%d", idx)
		args = append(args, requestType)
		idx++
	}

	// Scope filtering: use outlet_id OR work_unit_id for scoped roles
	if filterIDs != nil && wuScopeIDs != nil && len(wuScopeIDs) > 0 {
		// Scoped: show purchase requests matching outlet_ids OR work_unit_ids
		if len(filterIDs) > 0 {
			where += fmt.Sprintf(" AND (pr.outlet_id = ANY($%d::text[]) OR pr.work_unit_id = ANY($%d::text[]))", idx, idx+1)
			args = append(args, pq.Array(filterIDs), pq.Array(wuScopeIDs))
			idx += 2
		} else {
			where += fmt.Sprintf(" AND pr.work_unit_id = ANY($%d::text[])", idx)
			args = append(args, pq.Array(wuScopeIDs))
			idx++
		}
	} else if filterIDs != nil && len(filterIDs) > 0 {
		where += fmt.Sprintf(" AND pr.outlet_id = ANY($%d::text[])", idx)
		args = append(args, pq.Array(filterIDs))
		idx++
	} else if filterIDs != nil && len(filterIDs) == 0 {
		// Scoped but no outlets and no work units — return empty
		where += " AND FALSE"
	}
	if workUnitID != "" {
		where += fmt.Sprintf(" AND pr.work_unit_id = $%d", idx)
		args = append(args, workUnitID)
		idx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND pr.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if projectID != "" {
		where += fmt.Sprintf(" AND pr.project_id = $%d", idx)
		args = append(args, projectID)
		idx++
	}

	// Default: only show main requests (not children) unless explicitly filtering for children or parents
	if parentID == "" {
		where += " AND pr.parent_id IS NULL"
	} else if parentID != "all" {
		where += fmt.Sprintf(" AND pr.parent_id = $%d", idx)
		args = append(args, parentID)
		idx++
	}

	if excludeMasters {
		// Halaman Pembayaran menampilkan tiap pecahan sendiri-sendiri, jadi
		// master tidak boleh ikut muncul dan menggandakan nominal. Master yang
		// masih memegang sisa item tetap ditampilkan — sisa itu tagihan nyata
		// yang kalau disembunyikan tak akan pernah bisa dibayar.
		where += " AND NOT " + fullySplitMasterCond("pr")
	}

	if search != "" {
		// Kata kunci dicocokkan sampai ke nama sub-item; sebelumnya hanya nama
		// grup pengadaan yang dicari sehingga mencari nama barang tidak ketemu.
		where += fmt.Sprintf(` AND (pr.request_number ILIKE $%d OR pr.vendor_name ILIKE $%d
			OR EXISTS (
				SELECT 1 FROM jsonb_array_elements(pr.items) AS grp
				LEFT JOIN LATERAL jsonb_array_elements(COALESCE(grp->'items', '[]'::jsonb)) AS sub ON TRUE
				WHERE grp->>'name' ILIKE $%d OR sub->>'name' ILIKE $%d
			))`, idx, idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	// Count
	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM purchase_requests pr %s", where)
	if err := database.DB.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	offset := (page - 1) * limit

	// Fetch
	// Halaman Pembayaran (excludeMasters) menagih baris per baris, jadi master
	// hanya boleh menunjukkan sisa miliknya sendiri. Di daftar pengajuan biasa
	// master mewakili seluruh dokumen: sisa sendiri + semua pecahan.
	totalExpr := func(col string) string {
		if excludeMasters {
			return "pr." + col
		}
		return fmt.Sprintf(
			"pr.%[1]s + CASE WHEN pr.split_status = 'master' THEN COALESCE((SELECT SUM(c.%[1]s) FROM purchase_requests c WHERE c.parent_id = pr.id), 0) ELSE 0 END",
			col)
	}
	totalAmountExpr := totalExpr("total_amount")
	totalHpsExpr := totalExpr("total_hps")
	totalFinalExpr := totalExpr("total_final")
	query := fmt.Sprintf(`
		SELECT pr.id, pr.request_number, pr.outlet_id, o.name, pr.work_unit_id, COALESCE(wu.name,''),
		       pr.request_type, pr.requested_by, pr.vendor_id, pr.vendor_name, pr.status,
		       pr.items,
		       %s, %s, %s,
		       pr.notes, pr.invoice_number,`, totalAmountExpr, totalHpsExpr, totalFinalExpr)
	query += fmt.Sprintf(`
		       pr.approved_by, pr.approved_at,
		       pr.rejected_reason,
		       pr.paid_by, pr.paid_at, pr.payment_proof,
		       pr.payment_account_dest, pr.payment_account_source, pr.payment_notes,
		       pr.paid_amount,
		       pr.received_by, pr.received_at, COALESCE(pr.receipt_status, ''),
		       pr.parent_id, COALESCE(ppr.request_number,''), pr.split_status,
		       pr.project_id, COALESCE(prj.name,''),
		       pr.created_at, pr.updated_at
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		LEFT JOIN purchase_requests ppr ON ppr.id = pr.parent_id
		LEFT JOIN projects prj ON prj.id = pr.project_id
		%s
		ORDER BY pr.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]models.PurchaseRequest, 0)
	for rows.Next() {
		var r models.PurchaseRequest
		var itemsJSON []byte
		var approvedAt, paidAt, receivedAt *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&r.ID, &r.RequestNumber, &r.OutletID, &r.OutletName, &r.WorkUnitID, &r.WorkUnitName,
			&r.RequestType, &r.RequestedBy, &r.VendorID, &r.VendorName, &r.Status,
			&itemsJSON, &r.TotalAmount, &r.TotalHps, &r.TotalFinal, &r.Notes, &r.InvoiceNumber,
			&r.ApprovedBy, &approvedAt,
			&r.RejectedReason,
			&r.PaidBy, &paidAt, &r.PaymentProof,
			&r.PaymentAccountDest, &r.PaymentAccountSource, &r.PaymentNotes,
			&r.PaidAmount,
			&r.ReceivedBy, &receivedAt, &r.ReceiptStatus,
			&r.ParentID, &r.ParentNumber, &r.SplitStatus,
			&r.ProjectID, &r.ProjectName,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		json.Unmarshal(itemsJSON, &r.Items)
		if r.Items == nil {
			r.Items = []models.PurchaseRequestItem{}
		}

		r.CreatedAt = createdAt.Format(time.RFC3339)
		r.UpdatedAt = updatedAt.Format(time.RFC3339)
		if approvedAt != nil {
			s := approvedAt.Format(time.RFC3339)
			r.ApprovedAt = &s
		}
		if paidAt != nil {
			s := paidAt.Format(time.RFC3339)
			r.PaidAt = &s
		}
		if receivedAt != nil {
			s := receivedAt.Format(time.RFC3339)
			r.ReceivedAt = &s
		}

		requests = append(requests, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.PurchaseRequestListResponse{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		Requests:   requests,
	}, nil
}

func CreatePurchaseRequest(input models.CreatePurchaseRequestInput) (*models.PurchaseRequest, error) {
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}
	// Pengadaan barang dipisah sejak awal: dapur ke Gudang Induk, peralatan ke
	// bagian Aset. Lihat ClassifyPurchaseItems untuk alasannya.
	goodsKind := ""
	if input.RequestType == "barang" {
		k, err := ClassifyPurchaseItems(input.Items)
		if err != nil {
			return nil, err
		}
		goodsKind = k
	}

	id := NewULID()
	totalHps, totalFinal := recalcItems(input.Items)

	itemsJSON, err := json.Marshal(input.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	now := time.Now().UTC()
	err = insertWithRequestNumber(now, func(reqNumber string) error {
		_, err := database.DB.Exec(`
			INSERT INTO purchase_requests (id, request_number, outlet_id, work_unit_id, request_type, requested_by, vendor_id, vendor_name, status, items, total_amount, total_hps, total_final, notes, project_id, goods_kind, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $10, $11, $12, $13, $14, $15, $16, $16)
		`, id, reqNumber, nilIfEmpty(input.OutletID), nilIfEmpty(input.WorkUnitID), input.RequestType, input.RequestedBy,
			nilIfEmpty(input.VendorID), input.VendorName, itemsJSON,
			amountOf(totalHps, totalFinal), totalHps, totalFinal, input.Notes, nilIfEmpty(input.ProjectID), goodsKind, now)
		return err
	})
	if err != nil {
		return nil, err
	}

	return GetPurchaseRequest(id)
}

func GetPurchaseRequest(id string) (*models.PurchaseRequest, error) {
	var r models.PurchaseRequest
	var itemsJSON []byte
	var approvedAt, paidAt, receivedAt *time.Time
	var createdAt, updatedAt time.Time

	err := database.DB.QueryRow(`
		SELECT pr.id, pr.request_number, pr.outlet_id, o.name, pr.work_unit_id, COALESCE(wu.name,''),
		       pr.request_type, pr.requested_by, pr.vendor_id, pr.vendor_name, pr.status,
		       pr.items, pr.total_amount, pr.total_hps, pr.total_final, pr.notes, pr.invoice_number,
		       pr.approved_by, pr.approved_at,
		       pr.rejected_reason,
		       pr.paid_by, pr.paid_at, pr.payment_proof,
		       pr.payment_account_dest, pr.payment_account_source, pr.payment_notes,
		       pr.paid_amount,
		       pr.received_by, pr.received_at, COALESCE(pr.receipt_status, ''),
		       pr.parent_id, COALESCE(ppr.request_number,''), pr.split_status,
		       pr.project_id, COALESCE(prj.name,''),
		       pr.created_at, pr.updated_at
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		LEFT JOIN purchase_requests ppr ON ppr.id = pr.parent_id
		LEFT JOIN projects prj ON prj.id = pr.project_id
		WHERE pr.id = $1
	`, id).Scan(
		&r.ID, &r.RequestNumber, &r.OutletID, &r.OutletName, &r.WorkUnitID, &r.WorkUnitName,
		&r.RequestType, &r.RequestedBy, &r.VendorID, &r.VendorName, &r.Status,
		&itemsJSON, &r.TotalAmount, &r.TotalHps, &r.TotalFinal, &r.Notes, &r.InvoiceNumber,
		&r.ApprovedBy, &approvedAt,
		&r.RejectedReason,
		&r.PaidBy, &paidAt, &r.PaymentProof,
		&r.PaymentAccountDest, &r.PaymentAccountSource, &r.PaymentNotes,
		&r.PaidAmount,
		&r.ReceivedBy, &receivedAt, &r.ReceiptStatus,
		&r.ParentID, &r.ParentNumber, &r.SplitStatus,
		&r.ProjectID, &r.ProjectName,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(itemsJSON, &r.Items)
	if r.Items == nil {
		r.Items = []models.PurchaseRequestItem{}
	}

	// Master menampilkan nilai seluruh dokumen: sisa item yang masih dipegang
	// sendiri ditambah semua pecahannya. Dulu total master ditimpa mentah oleh
	// SUM(pecahan), sehingga item yang belum di-split hilang dari angka.
	if r.SplitStatus != nil && *r.SplitStatus == "master" {
		children, err := getPurchaseChildren(id)
		if err == nil {
			r.Children = children
			for _, ch := range children {
				r.TotalAmount += ch.TotalAmount
				r.TotalHps += ch.TotalHps
				r.TotalFinal += ch.TotalFinal
			}
		}
	}

	r.CreatedAt = createdAt.Format(time.RFC3339)
	r.UpdatedAt = updatedAt.Format(time.RFC3339)
	if approvedAt != nil {
		s := approvedAt.Format(time.RFC3339)
		r.ApprovedAt = &s
	}
	if paidAt != nil {
		s := paidAt.Format(time.RFC3339)
		r.PaidAt = &s
	}
	if receivedAt != nil {
		s := receivedAt.Format(time.RFC3339)
		r.ReceivedAt = &s
	}

	return &r, nil
}

// getPurchaseChildren fetches all split children for a master request.
func getPurchaseChildren(parentID string) ([]models.PurchaseRequest, error) {
	rows, err := database.DB.Query(`
		SELECT pr.id, pr.request_number, pr.outlet_id, o.name, pr.work_unit_id, COALESCE(wu.name,''),
		       pr.request_type, pr.requested_by, pr.vendor_id, pr.vendor_name, pr.status,
		pr.items, pr.total_amount, pr.total_hps, pr.total_final, pr.notes, pr.invoice_number,
		       pr.approved_by, pr.approved_at,
		       pr.rejected_reason,
		       pr.paid_by, pr.paid_at, pr.payment_proof,
		       pr.payment_account_dest, pr.payment_account_source, pr.payment_notes,
		       pr.paid_amount,
		       pr.received_by, pr.received_at, COALESCE(pr.receipt_status, ''),
		       pr.parent_id, COALESCE(ppr.request_number,''), pr.split_status,
		       pr.project_id, COALESCE(prj.name,''),
		       pr.created_at, pr.updated_at
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		LEFT JOIN purchase_requests ppr ON ppr.id = pr.parent_id
		LEFT JOIN projects prj ON prj.id = pr.project_id
		WHERE pr.parent_id = $1
		ORDER BY pr.created_at ASC
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	children := make([]models.PurchaseRequest, 0)
	for rows.Next() {
		var r models.PurchaseRequest
		var itemsJSON []byte
		var approvedAt, paidAt, receivedAt *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&r.ID, &r.RequestNumber, &r.OutletID, &r.OutletName, &r.WorkUnitID, &r.WorkUnitName,
			&r.RequestType, &r.RequestedBy, &r.VendorID, &r.VendorName, &r.Status,
			&itemsJSON, &r.TotalAmount, &r.TotalHps, &r.TotalFinal, &r.Notes, &r.InvoiceNumber,
			&r.ApprovedBy, &approvedAt,
			&r.RejectedReason,
			&r.PaidBy, &paidAt, &r.PaymentProof,
			&r.PaymentAccountDest, &r.PaymentAccountSource, &r.PaymentNotes,
			&r.PaidAmount,
			&r.ReceivedBy, &receivedAt, &r.ReceiptStatus,
			&r.ParentID, &r.ParentNumber, &r.SplitStatus,
			&r.ProjectID, &r.ProjectName,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		json.Unmarshal(itemsJSON, &r.Items)
		if r.Items == nil {
			r.Items = []models.PurchaseRequestItem{}
		}

		r.CreatedAt = createdAt.Format(time.RFC3339)
		r.UpdatedAt = updatedAt.Format(time.RFC3339)
		if approvedAt != nil {
			s := approvedAt.Format(time.RFC3339)
			r.ApprovedAt = &s
		}
		if paidAt != nil {
			s := paidAt.Format(time.RFC3339)
			r.PaidAt = &s
		}
		if receivedAt != nil {
			s := receivedAt.Format(time.RFC3339)
			r.ReceivedAt = &s
		}

		children = append(children, r)
	}
	return children, nil
}

func SplitPurchaseRequest(parentID string, input models.SplitPurchaseRequestInput) (*models.PurchaseRequest, error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// 1. Get parent PR details
	var p models.PurchaseRequest
	var itemsJSON []byte
	err = tx.QueryRow(`
		SELECT outlet_id, work_unit_id, request_type, requested_by, items, total_hps, total_final, status, request_number, project_id
		FROM purchase_requests WHERE id = $1
	`, parentID).Scan(&p.OutletID, &p.WorkUnitID, &p.RequestType, &p.RequestedBy, &itemsJSON, &p.TotalHps, &p.TotalFinal, &p.Status, &p.RequestNumber, &p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("pengajuan induk tidak ditemukan")
	}

	if p.Status != "approved" && p.Status != "pending" {
		return nil, fmt.Errorf("hanya pengajuan yang sudah disetujui atau pending yang bisa di-split vendor")
	}

	var parentItems []models.PurchaseRequestItem
	json.Unmarshal(itemsJSON, &parentItems)

	// 2. Pisahkan sub-item yang pindah ke pecahan dari yang tetap di master.
	// Pencocokan memakai kuota per (nama grup, nama item) yang dikonsumsi sekali
	// pakai: dua baris item bernama sama dalam satu grup dulu ikut terbawa
	// semuanya walau pengguna hanya memilih salah satu.
	quota := map[string]int{}
	for _, g := range input.Items {
		for _, s := range g.Items {
			quota[g.Name+"\x00"+s.Name]++
		}
	}

	var groupsToMove []models.PurchaseRequestItem
	groupsRemaining := []models.PurchaseRequestItem{}

	for _, pGroup := range parentItems {
		var subItemsToMove, subItemsRemaining []models.PurchaseSubItem

		for _, pSub := range pGroup.Items {
			key := pGroup.Name + "\x00" + pSub.Name
			if quota[key] > 0 {
				quota[key]--
				subItemsToMove = append(subItemsToMove, pSub)
			} else {
				subItemsRemaining = append(subItemsRemaining, pSub)
			}
		}

		if len(subItemsToMove) > 0 {
			g := pGroup
			g.Items = subItemsToMove
			groupsToMove = append(groupsToMove, g)
		}
		if len(subItemsRemaining) > 0 {
			g := pGroup
			g.Items = subItemsRemaining
			groupsRemaining = append(groupsRemaining, g)
		}
	}

	if len(groupsToMove) == 0 {
		return nil, fmt.Errorf("tidak ada item valid yang dipilih untuk di-split")
	}

	// 3. Buat PR pecahan berisi item yang dipindah.
	childID := NewULID()
	childHps, childFinal := recalcItems(groupsToMove)
	childItemsJSON, err := json.Marshal(groupsToMove)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun item pecahan: %w", err)
	}
	err = insertWithRequestNumber(now, func(reqNumber string) error {
		// SAVEPOINT: nomor kembar membuat INSERT gagal dan — tanpa ini —
		// seluruh transaksi split ikut abort sehingga retry tidak ada gunanya.
		if _, err := tx.Exec("SAVEPOINT split_child"); err != nil {
			return err
		}
		_, err := tx.Exec(`
			INSERT INTO purchase_requests (id, request_number, parent_id, outlet_id, work_unit_id, request_type, requested_by, vendor_id, vendor_name, status, items, total_amount, total_hps, total_final, notes, project_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $17)
		`, childID, reqNumber, parentID, p.OutletID, p.WorkUnitID, p.RequestType, p.RequestedBy,
			nilIfEmpty(input.VendorID), input.VendorName, p.Status, childItemsJSON,
			amountOf(childHps, childFinal), childHps, childFinal, "", p.ProjectID, now)
		if err != nil {
			tx.Exec("ROLLBACK TO SAVEPOINT split_child")
			return err
		}
		_, err = tx.Exec("RELEASE SAVEPOINT split_child")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("gagal membuat pengajuan pecahan: %w", err)
	}

	// Pecahan mewarisi jenis belanja induknya: memecah ke beberapa vendor tidak
	// mengubah di meja mana barangnya diterima.
	database.DB.Exec(`UPDATE purchase_requests c SET goods_kind = p.goods_kind
		FROM purchase_requests p WHERE c.parent_id = p.id AND COALESCE(c.goods_kind,'') = ''`)

	// 4. Item yang dipindah benar-benar keluar dari master — master hanya
	// menyisakan bagian yang belum diserahkan ke vendor mana pun, dan sisa itu
	// tetap bisa dibayar atas nama master sendiri. Master yang habis ter-split
	// menjadi dokumen kosong bertotal 0, lalu otomatis lenyap dari halaman
	// Pembayaran dan dari semua penjumlahan (lihat fullySplitMasterCond).
	remainingHps, remainingFinal := recalcItems(groupsRemaining)
	remainingJSON, err := json.Marshal(groupsRemaining)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun item induk: %w", err)
	}
	_, err = tx.Exec(`
		UPDATE purchase_requests
		SET split_status='master', items=$1, total_amount=$2, total_hps=$3, total_final=$4, updated_at=$5
		WHERE id=$6
	`, remainingJSON, amountOf(remainingHps, remainingFinal), remainingHps, remainingFinal, now, parentID)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui pengajuan induk: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetPurchaseRequest(parentID)
}

func UpdatePurchaseStatus(id string, input models.UpdatePurchaseStatusInput) (*models.PurchaseRequest, error) {
	// Get current status and split_status
	var currentStatus string
	var splitStatus *string
	err := database.DB.QueryRow("SELECT status, split_status FROM purchase_requests WHERE id = $1", id).Scan(&currentStatus, &splitStatus)
	if err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}

	transitions, ok := validTransitions[input.Action]
	if !ok {
		return nil, fmt.Errorf("aksi '%s' tidak valid", input.Action)
	}

	// If this is a master with children, cascade certain actions to children
	isMaster := splitStatus != nil && *splitStatus == "master"
	if isMaster {
		switch input.Action {
		case "pay":
			// Master hanya boleh dibayar sebatas sisa item yang masih dipegang
			// sendiri. Cek jumlah item, bukan total_final: master yang itemnya
			// sudah habis pindah ke pecahan tidak punya tagihan apa pun, dan
			// membayarnya berarti membayar dua kali atas barang yang sama.
			var ownItems int
			if scanErr := database.DB.QueryRow(
				"SELECT COALESCE(jsonb_array_length(items), 0) FROM purchase_requests WHERE id = $1", id,
			).Scan(&ownItems); scanErr != nil {
				return nil, fmt.Errorf("gagal memeriksa item pengajuan")
			}
			if ownItems == 0 {
				return nil, fmt.Errorf("seluruh item sudah dipecah ke vendor, bayar per vendor melalui halaman Pembayaran")
			}
			// Fall through to normal pay flow for this master's own items
		case "request_payment", "approve", "cancel", "reject", "receive":
			// reject & receive dulu tidak ikut cascade: master ditolak tapi
			// pecahannya tertinggal 'pending', atau master tercatat diterima
			// sementara pecahannya masih 'paid'.
			return cascadeStatusToChildren(id, currentStatus, input)
		}
	}

	newStatus, ok := transitions[currentStatus]
	if !ok {
		return nil, fmt.Errorf("tidak bisa %s dari status '%s'", input.Action, currentStatus)
	}
	// Alasan penolakan wajib, dan dijaga di SERVER — bukan hanya di layar.
	// Pengaju berhak tahu apa yang harus diperbaiki; penolakan tanpa alasan
	// memaksanya menebak dan mengajukan ulang hal yang sama.
	if input.Action == "reject" && strings.TrimSpace(input.RejectedReason) == "" {
		return nil, Invalid("alasan penolakan wajib diisi")
	}

	return applyStatusUpdate(id, newStatus, input)
}

// cascadeStatusToChildren applies a status action to all children of a master PR.
func cascadeStatusToChildren(masterID, masterStatus string, input models.UpdatePurchaseStatusInput) (*models.PurchaseRequest, error) {
	transitions, _ := validTransitions[input.Action]
	newStatus, ok := transitions[masterStatus]
	if !ok {
		return nil, fmt.Errorf("tidak bisa %s dari status '%s'", input.Action, masterStatus)
	}

	// Get all children
	rows, err := database.DB.Query("SELECT id, status FROM purchase_requests WHERE parent_id = $1", masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type child struct {
		id, status string
	}
	var children []child
	for rows.Next() {
		var c child
		rows.Scan(&c.id, &c.status)
		children = append(children, c)
	}

	// Apply to each eligible child
	for _, c := range children {
		if _, valid := transitions[c.status]; !valid {
			continue // skip children that can't transition
		}

		// For request_payment on children, validate total_final > 0
		if input.Action == "request_payment" {
			var totalFinal float64
			database.DB.QueryRow("SELECT COALESCE(total_final, 0) FROM purchase_requests WHERE id = $1", c.id).Scan(&totalFinal)
			if totalFinal <= 0 {
				continue // skip children without final price
			}
		}

		if _, err := applyStatusUpdate(c.id, newStatus, input); err != nil {
			return nil, fmt.Errorf("gagal update child %s: %w", c.id, err)
		}
	}

	// Also update master status
	if _, err := applyStatusUpdate(masterID, newStatus, input); err != nil {
		return nil, err
	}

	return GetPurchaseRequest(masterID)
}

// applyStatusUpdate performs the actual DB update for a single purchase request.
func applyStatusUpdate(id, newStatus string, input models.UpdatePurchaseStatusInput) (*models.PurchaseRequest, error) {
	now := time.Now().UTC()
	var err error

	switch input.Action {
	case "approve":
		_, err = database.DB.Exec(
			"UPDATE purchase_requests SET status=$1, approved_by=$2, approved_at=$3, updated_at=$3 WHERE id=$4",
			newStatus, input.ActorName, now, id,
		)
	case "reject":
		_, err = database.DB.Exec(
			"UPDATE purchase_requests SET status=$1, rejected_reason=$2, approved_by=$3, approved_at=$4, updated_at=$4 WHERE id=$5",
			newStatus, input.RejectedReason, input.ActorName, now, id,
		)
	case "request_payment":
		var totalFinal float64
		var splitStatus *string
		if scanErr := database.DB.QueryRow("SELECT COALESCE(total_final, 0), split_status FROM purchase_requests WHERE id = $1", id).Scan(&totalFinal, &splitStatus); scanErr != nil {
			return nil, fmt.Errorf("gagal memeriksa total harga")
		}
		// Master dinilai dari seluruh dokumen: sisa miliknya sendiri ditambah
		// semua pecahan. Dulu hanya pecahan yang dihitung, sehingga sisa item
		// milik master tidak pernah ikut diajukan.
		if splitStatus != nil && *splitStatus == "master" {
			var childFinal float64
			database.DB.QueryRow("SELECT COALESCE(SUM(total_final), 0) FROM purchase_requests WHERE parent_id = $1", id).Scan(&childFinal)
			totalFinal += childFinal
		}
		if totalFinal <= 0 {
			return nil, fmt.Errorf("harga final belum diisi, tidak bisa mengajukan pembayaran")
		}
		_, err = database.DB.Exec(
			"UPDATE purchase_requests SET status=$1, updated_at=$2 WHERE id=$3",
			newStatus, now, id,
		)
	case "pay":
		// Satu transaksi DB dengan FOR UPDATE: dua pembayaran bersamaan (double
		// click / retry) tidak boleh sama-sama membaca paid_amount lama — tanpa
		// lock ini histori pembayaran bisa dobel dan status jadi tidak konsisten.
		tx, txErr := database.DB.Begin()
		if txErr != nil {
			return nil, txErr
		}
		defer tx.Rollback()

		var totalFinal, currentPaid float64
		if scanErr := tx.QueryRow("SELECT COALESCE(total_final,0), COALESCE(paid_amount,0) FROM purchase_requests WHERE id=$1 FOR UPDATE", id).Scan(&totalFinal, &currentPaid); scanErr != nil {
			return nil, fmt.Errorf("gagal memeriksa tagihan: %w", scanErr)
		}
		remaining := totalFinal - currentPaid

		if remaining <= 0 {
			return nil, fmt.Errorf("tagihan sudah lunas")
		}

		payAmount := input.PaymentAmount
		if payAmount <= 0 {
			payAmount = remaining // default: pay full remaining
		}
		if payAmount > remaining {
			return nil, fmt.Errorf("jumlah bayar (%.0f) melebihi sisa tagihan (%.0f)", payAmount, remaining)
		}

		// Round to 2 decimals
		payAmount = math.Round(payAmount*100) / 100
		newPaid := math.Round((currentPaid+payAmount)*100) / 100

		// Insert payment history
		phID := NewULID()
		_, err = tx.Exec(
			`INSERT INTO payment_histories (id, purchase_request_id, amount, payment_proof, payment_account_dest, payment_account_source, payment_notes, paid_by, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			phID, id, payAmount, input.PaymentProof,
			input.PaymentAccountDest, input.PaymentAccountSource, input.PaymentNotes,
			input.ActorName, now,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal menyimpan histori pembayaran: %w", err)
		}

		// Determine new status
		actualStatus := "partial"
		if newPaid >= totalFinal {
			actualStatus = "paid"
		}
		// Barang yang sudah diterima lebih dulu (pembelian tempo): begitu
		// lunas, dokumennya langsung tuntas — tidak perlu menekan "Serah
		// Terima" untuk barang yang fisiknya sudah lama ada di outlet.
		if actualStatus == "paid" {
			var receipt string
			tx.QueryRow(`SELECT COALESCE(receipt_status, '') FROM purchase_requests WHERE id = $1`, id).Scan(&receipt)
			if receipt == "received" {
				actualStatus = "received"
			}
		}

		_, err = tx.Exec(
			`UPDATE purchase_requests SET status=$1, paid_by=$2, paid_at=$3, payment_proof=$4,
			 payment_account_dest=$5, payment_account_source=$6, payment_notes=$7,
			 paid_amount=$8, updated_at=$3 WHERE id=$9`,
			actualStatus, input.ActorName, now, input.PaymentProof,
			input.PaymentAccountDest, input.PaymentAccountSource, input.PaymentNotes,
			newPaid, id,
		)
		if err == nil {
			err = tx.Commit()
		}
	case "receive":
		_, err = database.DB.Exec(
			"UPDATE purchase_requests SET status=$1, received_by=$2, received_at=$3, updated_at=$3 WHERE id=$4",
			newStatus, input.ActorName, now, id,
		)
	case "cancel":
		_, err = database.DB.Exec(
			"UPDATE purchase_requests SET status=$1, rejected_reason=$2, updated_at=$3 WHERE id=$4",
			newStatus, input.RejectedReason, now, id,
		)
	}

	if err != nil {
		return nil, err
	}

	// After updating a child, sync the master's status based on all children
	syncMasterStatusFromChild(id)

	return GetPurchaseRequest(id)
}

// syncMasterStatusFromChild checks if a purchase request has a parent (master),
// and if all siblings are paid/received, updates the master status accordingly.
func syncMasterStatusFromChild(childID string) {
	var parentID *string
	database.DB.QueryRow("SELECT parent_id FROM purchase_requests WHERE id = $1", childID).Scan(&parentID)
	if parentID == nil || *parentID == "" {
		return
	}

	// Pelunasan dinilai dari nominal, bukan nama status: pecahan berstatus
	// 'received' yang masih kurang bayar dulu terhitung lunas dan ikut
	// menandai master lunas padahal uangnya belum keluar penuh.
	var total, settled int
	var sumPaid float64
	if err := database.DB.QueryRow(`
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE total_final > 0 AND paid_amount >= total_final),
		       COALESCE(SUM(paid_amount), 0)
		FROM purchase_requests WHERE parent_id = $1`, *parentID).Scan(&total, &settled, &sumPaid); err != nil || total == 0 {
		return
	}

	// Sisa item yang tidak ikut dipecah tetap tanggungan master sendiri, jadi
	// master belum lunas selama bagiannya sendiri masih kurang bayar.
	var ownOutstanding float64
	database.DB.QueryRow(
		"SELECT GREATEST(COALESCE(total_final,0) - COALESCE(paid_amount,0), 0) FROM purchase_requests WHERE id = $1",
		*parentID).Scan(&ownOutstanding)

	now := time.Now().UTC()
	switch {
	case settled == total && ownOutstanding == 0:
		database.DB.Exec("UPDATE purchase_requests SET status = 'paid', updated_at = $1 WHERE id = $2 AND status NOT IN ('paid','received')", now, *parentID)
	case sumPaid > 0:
		database.DB.Exec("UPDATE purchase_requests SET status = 'partial', updated_at = $1 WHERE id = $2 AND status NOT IN ('paid','received','partial')", now, *parentID)
	}
}

func UpdatePurchaseItems(id string, input models.UpdatePurchaseItemsInput) (*models.PurchaseRequest, error) {
	var currentStatus string
	err := database.DB.QueryRow("SELECT status FROM purchase_requests WHERE id = $1", id).Scan(&currentStatus)
	if err != nil {
		return nil, fmt.Errorf("pengajuan tidak ditemukan")
	}
	if currentStatus != "pending" && currentStatus != "approved" {
		return nil, fmt.Errorf("hanya bisa update item pada status pending/approved")
	}
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}
	var reqType string
	database.DB.QueryRow("SELECT request_type FROM purchase_requests WHERE id = $1", id).Scan(&reqType)
	if reqType == "barang" {
		k, kerr := ClassifyPurchaseItems(input.Items)
		if kerr != nil {
			return nil, kerr
		}
		database.DB.Exec("UPDATE purchase_requests SET goods_kind = $1 WHERE id = $2", k, id)
	}

	totalHps, totalFinal := recalcItems(input.Items)

	itemsJSON, err := json.Marshal(input.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	// COALESCE dengan nilai lama: kolom yang tidak dikirim pemanggil tidak
	// tersentuh. Vendor id dan nama diperlakukan satu paket supaya tidak pernah
	// tersisa id tanpa nama atau sebaliknya.
	var vendorID, vendorName, invoice interface{}
	if input.VendorID != nil || input.VendorName != nil {
		if input.VendorID != nil {
			vendorID = nilIfEmpty(*input.VendorID)
		}
		if input.VendorName != nil {
			vendorName = *input.VendorName
		} else {
			vendorName = ""
		}
	}
	if input.InvoiceNumber != nil {
		invoice = *input.InvoiceNumber
	}

	_, err = database.DB.Exec(`
		UPDATE purchase_requests SET
			items=$1, total_amount=$2, total_hps=$3, total_final=$4,
			vendor_id   = CASE WHEN $9::bool THEN $5::char(26) ELSE vendor_id END,
			vendor_name = CASE WHEN $9::bool THEN COALESCE($6::varchar, '') ELSE vendor_name END,
			invoice_number = COALESCE($7::text, invoice_number),
			updated_at=NOW()
		WHERE id=$8
	`, itemsJSON, amountOf(totalHps, totalFinal), totalHps, totalFinal,
		vendorID, vendorName, invoice, id,
		input.VendorID != nil || input.VendorName != nil)
	if err != nil {
		return nil, err
	}

	return GetPurchaseRequest(id)
}

func DeletePurchaseRequest(id string, isAdmin bool) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var splitStatus, parentID *string
	var paidAmount float64
	var itemsJSON []byte
	err = tx.QueryRow(
		"SELECT status, split_status, parent_id, COALESCE(paid_amount,0), items FROM purchase_requests WHERE id = $1 FOR UPDATE", id,
	).Scan(&status, &splitStatus, &parentID, &paidAmount, &itemsJSON)
	if err != nil {
		return fmt.Errorf("pengajuan tidak ditemukan")
	}

	if isAdmin {
		// Patokannya uang yang sudah keluar, bukan nama status. Cek lama hanya
		// menahan status 'paid', sehingga pengajuan 'partial' — yang sudah ada
		// pembayarannya — bisa terhapus berikut histori pembayarannya (FK
		// payment_histories ON DELETE CASCADE).
		if paidAmount > 0 || status == "paid" || status == "received" {
			return fmt.Errorf("tidak bisa menghapus pengajuan yang sudah ada pembayarannya")
		}
		if splitStatus != nil && *splitStatus == "master" {
			var blocked int
			if err := tx.QueryRow(
				`SELECT COUNT(*) FROM purchase_requests
				 WHERE parent_id = $1 AND (paid_amount > 0 OR status IN ('paid','received'))`, id,
			).Scan(&blocked); err != nil {
				return fmt.Errorf("gagal memeriksa status pecahan")
			}
			if blocked > 0 {
				return fmt.Errorf("tidak bisa menghapus pengajuan induk karena ada pecahan yang sudah ada pembayarannya")
			}
			if _, err := tx.Exec("DELETE FROM purchase_requests WHERE parent_id = $1", id); err != nil {
				return fmt.Errorf("gagal menghapus pecahan: %w", err)
			}
		}
	} else {
		if status != "pending" && status != "rejected" && status != "cancelled" {
			return fmt.Errorf("hanya bisa menghapus pengajuan berstatus pending/rejected/cancelled")
		}
		if paidAmount > 0 {
			return fmt.Errorf("tidak bisa menghapus pengajuan yang sudah ada pembayarannya")
		}
	}

	// Menghapus pecahan mengembalikan itemnya ke induk. Sejak split benar-benar
	// memindahkan item, tanpa langkah ini item tersebut akan hilang selamanya.
	if parentID != nil && *parentID != "" {
		if err := returnItemsToParent(tx, *parentID, itemsJSON); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("DELETE FROM purchase_requests WHERE id = $1", id); err != nil {
		return err
	}
	return tx.Commit()
}

// returnItemsToParent menggabungkan kembali item milik pecahan ke induknya dan
// menghitung ulang total induk. Bila setelah ini induk tidak punya pecahan lagi,
// penanda 'master' dilepas supaya kembali menjadi pengajuan biasa.
func returnItemsToParent(tx *sql.Tx, parentID string, childItemsJSON []byte) error {
	var parentJSON []byte
	if err := tx.QueryRow("SELECT items FROM purchase_requests WHERE id = $1 FOR UPDATE", parentID).Scan(&parentJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil // induk sudah tidak ada, tidak ada yang perlu dikembalikan
		}
		return fmt.Errorf("gagal memuat pengajuan induk: %w", err)
	}

	var parentItems, childItems []models.PurchaseRequestItem
	json.Unmarshal(parentJSON, &parentItems)
	json.Unmarshal(childItemsJSON, &childItems)

	// Item dikembalikan ke grup bernama sama bila ada, agar dokumen induk pulih
	// seperti sebelum di-split alih-alih memunculkan grup kembar.
	for _, cg := range childItems {
		merged := false
		for i := range parentItems {
			if parentItems[i].Name == cg.Name {
				parentItems[i].Items = append(parentItems[i].Items, cg.Items...)
				merged = true
				break
			}
		}
		if !merged {
			parentItems = append(parentItems, cg)
		}
	}
	if parentItems == nil {
		parentItems = []models.PurchaseRequestItem{}
	}

	totalHps, totalFinal := recalcItems(parentItems)
	mergedJSON, err := json.Marshal(parentItems)
	if err != nil {
		return fmt.Errorf("gagal menyusun item induk: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE purchase_requests SET
			items=$1, total_amount=$2, total_hps=$3, total_final=$4,
			split_status = CASE
				WHEN (SELECT COUNT(*) FROM purchase_requests c WHERE c.parent_id = $5) <= 1
				THEN NULL ELSE split_status END,
			updated_at=$6
		WHERE id=$5
	`, mergedJSON, amountOf(totalHps, totalFinal), totalHps, totalFinal, parentID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("gagal mengembalikan item ke induk: %w", err)
	}
	return nil
}

// GetPaymentHistories returns all payment history entries for a purchase request.
func GetPaymentHistories(purchaseRequestID string) ([]models.PaymentHistory, error) {
	rows, err := database.DB.Query(`
		SELECT id, purchase_request_id, amount, payment_proof,
		       payment_account_dest, payment_account_source, payment_notes,
		       paid_by, created_at
		FROM payment_histories
		WHERE purchase_request_id = $1
		ORDER BY created_at ASC
	`, purchaseRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	histories := make([]models.PaymentHistory, 0)
	for rows.Next() {
		var h models.PaymentHistory
		var createdAt time.Time
		if err := rows.Scan(&h.ID, &h.PurchaseRequestID, &h.Amount, &h.PaymentProof,
			&h.PaymentAccountDest, &h.PaymentAccountSource, &h.PaymentNotes,
			&h.PaidBy, &createdAt); err != nil {
			return nil, err
		}
		h.CreatedAt = createdAt.Format(time.RFC3339)
		histories = append(histories, h)
	}
	return histories, nil
}
