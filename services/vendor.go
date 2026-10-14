package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// prScopeCond builds an AND-condition restricting purchase_requests to the scoped
// outlets/work-units (so vendor stats never leak other outlets' purchases).
// prefix is the table alias ("pr" or "" for unaliased). startIdx = next $ placeholder.
// Returns ("", nil) for all-scope (both nil).
func prScopeCond(prefix string, outletIDs, wuIDs []string, startIdx int) (string, []interface{}) {
	if outletIDs == nil && wuIDs == nil {
		return "", nil
	}
	col := func(c string) string {
		if prefix == "" {
			return c
		}
		return prefix + "." + c
	}
	conds := []string{}
	args := []interface{}{}
	i := startIdx
	if outletIDs != nil {
		conds = append(conds, fmt.Sprintf("%s = ANY($%d::text[])", col("outlet_id"), i))
		args = append(args, pq.Array(outletIDs))
		i++
	}
	if wuIDs != nil {
		conds = append(conds, fmt.Sprintf("%s = ANY($%d::text[])", col("work_unit_id"), i))
		args = append(args, pq.Array(wuIDs))
		i++
	}
	return " AND (" + strings.Join(conds, " OR ") + ")", args
}

func ListVendors(activeOnly bool, outletIDs, wuIDs []string) ([]models.Vendor, error) {
	where := ""
	if activeOnly {
		where = "WHERE v.is_active = true"
	}
	scopeCond, scopeArgs := prScopeCond("pr", outletIDs, wuIDs, 1)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT v.id, v.name, v.phone, v.email, v.address, v.notes,
		       v.bank_name, v.account_number, v.account_holder, v.is_active,
		       -- Definisi tunggal hutang (procurement_finance.go). Dulu di sini
		       -- memakai SUM(total_final) atas status approved/payment_requested
		       -- saja: cicilan yang sudah dibayar tidak dikurangi, dan tagihan
		       -- berstatus 'partial' / 'received' yang belum lunas tidak muncul
		       -- sama sekali — angkanya tidak pernah cocok dengan halaman
		       -- Pembayaran maupun Neraca.
		       COALESCE((
		           SELECT SUM(`+payableExpr("pr")+`)
		           FROM purchase_requests pr
		           WHERE pr.vendor_id = v.id
		             AND `+outstandingCondFor("pr")+`
		             AND `+countableCond("pr")+`%s
		       ), 0) AS unpaid_amount,
		       v.created_at, v.updated_at
		FROM vendors v %s ORDER BY v.name ASC
	`, scopeCond, where), scopeArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vendors := make([]models.Vendor, 0)
	for rows.Next() {
		var v models.Vendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Phone, &v.Email, &v.Address, &v.Notes, &v.BankName, &v.AccountNumber, &v.AccountHolder, &v.IsActive, &v.UnpaidAmount, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		vendors = append(vendors, v)
	}
	return vendors, rows.Err()
}

func GetVendor(id string) (*models.Vendor, error) {
	var v models.Vendor
	err := database.DB.QueryRow(`
		SELECT id, name, phone, email, address, notes, bank_name, account_number, account_holder, is_active, created_at, updated_at
		FROM vendors WHERE id = $1
	`, id).Scan(&v.ID, &v.Name, &v.Phone, &v.Email, &v.Address, &v.Notes, &v.BankName, &v.AccountNumber, &v.AccountHolder, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	changes, err := vendorBankChanges(v.ID, vendorBankChangesShown)
	if err != nil {
		return nil, err
	}
	v.BankChanges = changes
	return &v, nil
}

// vendorBankChangesShown: jumlah perubahan rekening terakhir yang ikut GetVendor.
const vendorBankChangesShown = 10

// vendorBankChanges: perubahan rekening vendor terbaru dulu.
func vendorBankChanges(vendorID string, limit int) ([]models.VendorBankChange, error) {
	rows, err := database.DB.Query(`
		SELECT id, old_bank_name, old_account_number, old_account_holder,
		       new_bank_name, new_account_number, new_account_holder, changed_by, changed_at
		FROM vendor_bank_changes WHERE vendor_id = $1
		ORDER BY changed_at DESC, id DESC LIMIT $2`, vendorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.VendorBankChange, 0)
	for rows.Next() {
		var c models.VendorBankChange
		if err := rows.Scan(&c.ID, &c.OldBankName, &c.OldAccountNumber, &c.OldAccountHolder,
			&c.NewBankName, &c.NewAccountNumber, &c.NewAccountHolder, &c.ChangedBy, &c.ChangedAt); err != nil {
			return nil, err
		}
		c.ID = strings.TrimSpace(c.ID)
		out = append(out, c)
	}
	return out, rows.Err()
}

func CreateVendor(req models.CreateVendorRequest) (*models.Vendor, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("nama vendor wajib diisi")
	}
	id := NewULID()
	_, err := database.DB.Exec(`
		INSERT INTO vendors (id, name, phone, email, address, notes, bank_name, account_number, account_holder, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, NOW(), NOW())
	`, id, req.Name, req.Phone, req.Email, req.Address, req.Notes, req.BankName, req.AccountNumber, req.AccountHolder)
	if err != nil {
		return nil, err
	}
	return GetVendor(id)
}

// UpdateVendor menyimpan data vendor. Bila rekeningnya (bank, nomor, atas
// nama) berubah, perubahan itu dicatat di vendor_bank_changes beserta
// pelakunya. Pembayaran lama tidak ikut berubah — rekeningnya dipotret per
// pembayaran — dan form bayar memperingatkan perubahan rekening yang baru.
func UpdateVendor(id string, req models.UpdateVendorRequest, actor Actor) (*models.Vendor, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("nama vendor wajib diisi")
	}
	req.BankName = strings.TrimSpace(req.BankName)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.AccountHolder = strings.TrimSpace(req.AccountHolder)

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// FOR UPDATE: pembayaran yang sedang memotret rekening vendor ini
	// (FOR SHARE) selesai lebih dulu, atau menunggu perubahan ini selesai.
	var oldBank, oldNumber, oldHolder string
	err = tx.QueryRow(`
		SELECT COALESCE(bank_name, ''), COALESCE(account_number, ''), COALESCE(account_holder, '')
		FROM vendors WHERE id = $1 FOR UPDATE`, id).Scan(&oldBank, &oldNumber, &oldHolder)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("vendor tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`
		UPDATE vendors SET name=$1, phone=$2, email=$3, address=$4, notes=$5, bank_name=$6, account_number=$7, account_holder=$8, updated_at=NOW()
		WHERE id=$9
	`, req.Name, req.Phone, req.Email, req.Address, req.Notes, req.BankName, req.AccountNumber, req.AccountHolder, id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(oldBank) != req.BankName || strings.TrimSpace(oldNumber) != req.AccountNumber ||
		strings.TrimSpace(oldHolder) != req.AccountHolder {
		if _, err := tx.Exec(`
			INSERT INTO vendor_bank_changes (id, vendor_id, old_bank_name, old_account_number, old_account_holder,
				new_bank_name, new_account_number, new_account_holder, changed_by, changed_by_id, changed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			NewULID(), id, strings.TrimSpace(oldBank), strings.TrimSpace(oldNumber), strings.TrimSpace(oldHolder),
			req.BankName, req.AccountNumber, req.AccountHolder, actor.Name, nilIfEmpty(validActorID(actor.ID)), time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("gagal mencatat perubahan rekening vendor: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetVendor(id)
}

func ToggleVendorActive(id string) (*models.Vendor, error) {
	result, err := database.DB.Exec(`
		UPDATE vendors SET is_active = NOT is_active, updated_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("vendor tidak ditemukan")
	}
	return GetVendor(id)
}

// DeleteVendor menolak menghapus vendor yang masih memegang riwayat pengadaan.
// FK purchase_requests.vendor_id ON DELETE SET NULL, jadi menghapusnya akan
// memutus tautan seluruh pengajuan vendor itu tanpa jejak: halaman Detail
// Vendor, rekap hutang dan riwayat pembayarannya ikut kosong. Nonaktifkan
// vendornya saja (ToggleVendorActive) bila sudah tidak dipakai.
func DeleteVendor(id string) error {
	var used int
	if err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM purchase_requests WHERE vendor_id = $1", id,
	).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("vendor masih memiliki %d pengajuan pengadaan — nonaktifkan saja agar riwayatnya tetap utuh", used)
	}
	result, err := database.DB.Exec("DELETE FROM vendors WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("vendor tidak ditemukan")
	}
	return nil
}

// GetVendorDetail returns vendor info with aggregated purchase stats and monthly spending.
func GetVendorDetail(id string, outletIDs, wuIDs []string) (*models.VendorDetailResponse, error) {
	vendor, err := GetVendor(id)
	if err != nil {
		return nil, err
	}

	resp := &models.VendorDetailResponse{Vendor: *vendor}

	// Aggregate stats from purchase_requests (scoped to the user's outlets/work-units)
	aggCond, aggArgs := prScopeCond("", outletIDs, wuIDs, 2)
	err = database.DB.QueryRow(`
		SELECT
			COUNT(*),
			-- total_amount = harga final bila sudah diisi, selain itu HPS.
			-- Dulu memakai total_final: pengajuan yang belum diberi harga oleh
			-- purchasing bernilai 0, sehingga "Total Belanja" dan terutama
			-- "Pending" hampir selalu menampilkan Rp 0.
			--
			-- Total Belanja hanya dari pengajuan yang SUDAH DISETUJUI (daftar
			-- putih status berkomitmen); yang masih menunggu verifikasi atau
			-- persetujuan masuk kartu "Menunggu", bukan belanja (T-42).
			COALESCE(SUM(CASE WHEN `+prCommittedCond("")+` THEN total_amount ELSE 0 END), 0),
			-- Dibayar & hutang dihitung dari nominal, bukan nama status.
			-- Sebelumnya 'partial' tidak masuk keduanya, sehingga pengajuan yang
			-- baru dilunasi sebagian lenyap dari kedua angka sekaligus.
			COALESCE(SUM(CASE WHEN `+prCommittedCond("")+` THEN paid_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN `+outstandingCond+` THEN `+payableExpr("")+` ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN `+prPreApprovalCond("")+` THEN total_amount ELSE 0 END), 0)
		FROM purchase_requests WHERE vendor_id = $1 AND `+countableCond("")+aggCond,
		append([]interface{}{id}, aggArgs...)...).
		Scan(&resp.PurchaseCount, &resp.TotalSpent, &resp.TotalPaid, &resp.TotalDebt, &resp.TotalPending)
	if err != nil {
		return nil, err
	}

	// Monthly spending (last 12 months, scoped)
	monCond, monArgs := prScopeCond("", outletIDs, wuIDs, 2)
	rows, err := database.DB.Query(`
		SELECT TO_CHAR(tz_date(created_at), 'YYYY-MM') AS month,
		       COUNT(*),
		       COALESCE(SUM(total_amount), 0)
		FROM purchase_requests
		WHERE vendor_id = $1 AND `+prCommittedCond("")+`
		  AND `+countableCond("")+`
		  AND created_at >= NOW() - INTERVAL '12 months'`+monCond+`
		GROUP BY month ORDER BY month ASC
	`, append([]interface{}{id}, monArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	monthMap := make(map[string]models.VendorMonthlySpend)
	for rows.Next() {
		var ms models.VendorMonthlySpend
		if err := rows.Scan(&ms.Month, &ms.Count, &ms.TotalAmount); err != nil {
			return nil, err
		}
		monthMap[ms.Month] = ms
	}

	// Fill 12 months
	now := time.Now().UTC()
	resp.MonthlySpend = make([]models.VendorMonthlySpend, 0, 12)
	for i := 11; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		key := t.Format("2006-01")
		if ms, ok := monthMap[key]; ok {
			resp.MonthlySpend = append(resp.MonthlySpend, ms)
		} else {
			resp.MonthlySpend = append(resp.MonthlySpend, models.VendorMonthlySpend{Month: key})
		}
	}

	return resp, nil
}

// ListVendorPurchases returns paginated purchase requests for a specific vendor.
func ListVendorPurchases(vendorID string, outletIDs, wuIDs []string, page, limit int) (*models.PurchaseRequestListResponse, error) {
	scopeCond, scopeArgs := prScopeCond("pr", outletIDs, wuIDs, 2)

	var total int
	countArgs := append([]interface{}{vendorID}, scopeArgs...)
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM purchase_requests pr WHERE pr.vendor_id = $1"+scopeCond, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	totalPages := (total + limit - 1) / limit
	offset := (page - 1) * limit

	args := append([]interface{}{vendorID}, scopeArgs...)
	limIdx := len(args) + 1
	offIdx := len(args) + 2
	args = append(args, limit, offset)
	// Kolom & pembacaan sama dengan daftar pengajuan (purchaseSelectCols):
	// jejak verifikasi, rejected_items, dan version ikut terisi.
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT %s
		%s
		WHERE pr.vendor_id = $1%s
		ORDER BY pr.created_at DESC
		LIMIT $%d OFFSET $%d
	`, purchaseSelectCols("pr.total_amount", "pr.total_hps", "pr.total_final"), purchaseSelectJoins,
		scopeCond, limIdx, offIdx), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]models.PurchaseRequest, 0)
	for rows.Next() {
		r, err := scanPurchaseRequest(rows)
		if err != nil {
			return nil, err
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
