package services

import "github.com/lib/pq"

// ppicWarehouseScope mempertahankan cakupan monitor kedaluwarsa dan opname:
// gudang pusat serta gudang outlet yang ditugaskan. Alias tabel adalah "w".
// Kontrol pemakaian memakai pemeriksaan cakupannya sendiri yang lebih ketat.
func ppicWarehouseScope(outletIDs []string) (string, []interface{}) {
	if outletIDs == nil {
		return "", nil
	}
	return " AND (w.outlet_id = ANY($1) OR w.type = 'central')", []interface{}{pq.Array(outletIDs)}
}
