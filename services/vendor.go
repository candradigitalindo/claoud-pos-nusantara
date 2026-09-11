package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
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
		       COALESCE((
		           SELECT SUM(pr.total_final)
		           FROM purchase_requests pr
		           WHERE pr.vendor_id = v.id
		             AND pr.status IN ('approved', 'payment_requested')
		             AND (pr.split_status IS NULL OR pr.split_status != 'master')%s
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
	return &v, nil
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

func UpdateVendor(id string, req models.UpdateVendorRequest) (*models.Vendor, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("nama vendor wajib diisi")
	}
	result, err := database.DB.Exec(`
		UPDATE vendors SET name=$1, phone=$2, email=$3, address=$4, notes=$5, bank_name=$6, account_number=$7, account_holder=$8, updated_at=NOW()
		WHERE id=$9
	`, req.Name, req.Phone, req.Email, req.Address, req.Notes, req.BankName, req.AccountNumber, req.AccountHolder, id)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("vendor tidak ditemukan")
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

func DeleteVendor(id string) error {
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
			COALESCE(SUM(CASE WHEN status NOT IN ('cancelled','rejected') THEN total_final ELSE 0 END), 0),
			-- Dibayar & hutang dihitung dari nominal, bukan nama status.
			-- Sebelumnya 'partial' tidak masuk keduanya, sehingga pengajuan yang
			-- baru dilunasi sebagian lenyap dari kedua angka sekaligus.
			COALESCE(SUM(CASE WHEN status NOT IN ('cancelled','rejected') THEN paid_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN `+outstandingCond+` THEN total_final - paid_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN total_final ELSE 0 END), 0)
		FROM purchase_requests WHERE vendor_id = $1`+aggCond,
		append([]interface{}{id}, aggArgs...)...).
		Scan(&resp.PurchaseCount, &resp.TotalSpent, &resp.TotalPaid, &resp.TotalDebt, &resp.TotalPending)
	if err != nil {
		return nil, err
	}

	// Monthly spending (last 12 months, scoped)
	monCond, monArgs := prScopeCond("", outletIDs, wuIDs, 2)
	rows, err := database.DB.Query(`
		SELECT TO_CHAR(created_at, 'YYYY-MM') AS month,
		       COUNT(*),
		       COALESCE(SUM(total_final), 0)
		FROM purchase_requests
		WHERE vendor_id = $1 AND status NOT IN ('cancelled','rejected')
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

	totalPages := int(float64(total+limit-1) / float64(limit))
	offset := (page - 1) * limit

	args := append([]interface{}{vendorID}, scopeArgs...)
	limIdx := len(args) + 1
	offIdx := len(args) + 2
	args = append(args, limit, offset)
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT pr.id, pr.outlet_id, COALESCE(o.name,''), pr.work_unit_id, COALESCE(wu.name,''),
		       pr.request_type, pr.requested_by, pr.vendor_id, pr.vendor_name, pr.status,
		       pr.items, pr.total_amount, pr.total_hps, pr.total_final, pr.notes,
		       pr.approved_by, pr.approved_at,
		       pr.rejected_reason,
		       pr.paid_by, pr.paid_at, pr.payment_proof,
		       pr.payment_account_dest, pr.payment_account_source, pr.payment_notes,
		       pr.received_by, pr.received_at,
		       pr.parent_id, pr.split_status,
		       pr.created_at, pr.updated_at
		FROM purchase_requests pr
		LEFT JOIN outlets o ON o.id = pr.outlet_id
		LEFT JOIN work_units wu ON wu.id = pr.work_unit_id
		WHERE pr.vendor_id = $1%s
		ORDER BY pr.created_at DESC
		LIMIT $%d OFFSET $%d
	`, scopeCond, limIdx, offIdx), args...)
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
			&r.ID, &r.OutletID, &r.OutletName, &r.WorkUnitID, &r.WorkUnitName,
			&r.RequestType, &r.RequestedBy, &r.VendorID, &r.VendorName, &r.Status,
			&itemsJSON, &r.TotalAmount, &r.TotalHps, &r.TotalFinal, &r.Notes,
			&r.ApprovedBy, &approvedAt,
			&r.RejectedReason,
			&r.PaidBy, &paidAt, &r.PaymentProof,
			&r.PaymentAccountDest, &r.PaymentAccountSource, &r.PaymentNotes,
			&r.ReceivedBy, &receivedAt,
			&r.ParentID, &r.SplitStatus,
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

	return &models.PurchaseRequestListResponse{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		Requests:   requests,
	}, nil
}
