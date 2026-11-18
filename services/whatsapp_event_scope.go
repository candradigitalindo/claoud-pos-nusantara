package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// waEventScope describes the data in an event. A mixed event is either a
// document tied to an outlet/central office, or a digest rendered separately
// for each recipient's effective outlet scope.
func waEventScope(event string) (kind, description string) {
	switch {
	case event == "sales.daily":
		return "mixed", "Rekap dihitung khusus untuk outlet penerima. Rekap seluruh outlet hanya untuk penerima bercakupan semua dengan laporan global aktif."
	case event == "ppic.alert":
		return "mixed", "Jumlah dan daftar batch mengikuti outlet penerima. Stok gudang induk hanya masuk laporan global."
	case event == "asset.maintenance_due":
		return "mixed", "Jumlah dan daftar perawatan mengikuti outlet aset. Aset pusat hanya masuk laporan global."
	case strings.HasPrefix(event, "purchase."):
		return "mixed", "Pengajuan mengikuti cakupan outlet atau unit kerja penerima. Pengajuan tanpa outlet dan unit kerja hanya untuk penerima bercakupan semua dengan laporan global aktif. Pengaju tetap mengikuti batas cakupannya."
	case strings.HasPrefix(event, "customer."):
		return "outlet", "Pesan dikirim ke nomor pelanggan pada reservasi dan hanya memuat reservasinya sendiri."
	case strings.HasPrefix(event, "reservation."), event == "cashier_shift.closed", event == "device.offline":
		return "outlet", "Data satu outlet. Penerima hanya mendapat outlet yang ada dalam cakupannya."
	default:
		return "global", "Data pusat atau seluruh organisasi. Hanya untuk penerima bercakupan semua dengan laporan global aktif."
	}
}

// An empty recipient filter means all outlets. A nonempty filter never grants
// access to central data, whose outlet ID is empty.
func waDigestCoversOutlet(outletIDs []string, outletID string) bool {
	if len(outletIDs) == 0 {
		return true
	}
	outletID = strings.TrimSpace(outletID)
	if outletID == "" {
		return false
	}
	for _, id := range outletIDs {
		if strings.TrimSpace(id) == outletID {
			return true
		}
	}
	return false
}

// Each body is built after routing resolves the recipient's effective scope.
// The stable event/ref/target key keeps scheduler retries from enqueueing the
// same daily digest more than once.
func notifyWAScopedDigest(event, refID string, build func([]string) (map[string]string, bool)) {
	if waGatewayURL == "" {
		return
	}
	spec := waSpec(event)
	if spec == nil || spec.Audience != "internal" {
		return
	}
	s := GetWASettings()
	if !s.Enabled || !waEventEnabled(s, event) {
		return
	}
	tpl := s.Templates[event]
	if strings.TrimSpace(tpl) == "" {
		tpl = spec.Template
	}
	globals := waGlobals()
	for _, recipient := range waRecipientsFor(event, waDigestOutlet) {
		vars, ok := build(recipient.OutletIDs)
		if !ok {
			continue
		}
		body := waRender(tpl, waMerge(globals, vars))
		if strings.TrimSpace(body) == "" {
			continue
		}
		if _, err := EnqueueWAMessage(models.WAMessage{
			Target: recipient.Target, TargetName: recipient.Name, Audience: "internal", Kind: "notify",
			Event: event, RefID: refID, Body: body,
		}); err != nil {
			log.Printf("[WA] antre rekap %s: %v", event, err)
		}
	}
}

func waDailySalesVars(day string, all []waOutletSales, outletIDs []string) (map[string]string, bool) {
	var revenue, unpaidAmount float64
	var count, unpaidCount, voidCount int
	lines := []string{}
	for _, outlet := range all {
		if !waDigestCoversOutlet(outletIDs, outlet.ID) {
			continue
		}
		revenue += outlet.Revenue
		count += outlet.Count
		unpaidCount += outlet.UnpaidCount
		unpaidAmount += outlet.UnpaidAmount
		voidCount += outlet.VoidCount
		lines = append(lines, fmt.Sprintf("• %s: %s (%d trx)", outlet.Name, waRp(outlet.Revenue), outlet.Count))
	}
	if len(lines) == 0 {
		return nil, false
	}
	average := 0.0
	if count > 0 {
		average = revenue / float64(count)
	}
	return map[string]string{
		"date": waDate(day), "outlets": strings.Join(lines, "\n"),
		"total_revenue": waRp(revenue), "total_count": fmt.Sprintf("%d", count), "avg": waRp(average),
		"unpaid_count": fmt.Sprintf("%d", unpaidCount), "unpaid_amount": waRp(unpaidAmount), "void_count": fmt.Sprintf("%d", voidCount),
	}, true
}

type waExpiryAlertLine struct {
	item, warehouse, expiry, unit string
	qty                           float64
}

type waExpiryAlertGroup struct {
	outletID          string
	expired, expiring int
	lines             []waExpiryAlertLine
}

// Count the full set in SQL before limiting details to ten per outlet. This
// keeps both scoped and global counts accurate without loading every batch.
func waPpicAlertGroups() ([]waExpiryAlertGroup, error) {
	rows, err := database.DB.Query(`
		WITH alerts AS (
			SELECT CASE WHEN w.type = 'outlet' THEN COALESCE(w.outlet_id, '') ELSE '' END AS outlet_id,
				si.name AS item, w.name AS warehouse, sb.expiry_date::text AS expiry, sb.qty_base, si.base_unit,
				CASE WHEN sb.expiry_date < CURRENT_DATE THEN 1 ELSE 0 END AS expired
			FROM stock_batches sb
			JOIN stock_items si ON si.id = sb.item_id
			JOIN warehouses w ON w.id = sb.warehouse_id
			WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL AND sb.expiry_date < CURRENT_DATE + 3
			  AND sb.ppic_ack_at IS NULL AND w.is_active = true
		), ranked AS (
			SELECT *, SUM(expired) OVER (PARTITION BY outlet_id) AS expired_count,
				COUNT(*) OVER (PARTITION BY outlet_id) - SUM(expired) OVER (PARTITION BY outlet_id) AS expiring_count,
				ROW_NUMBER() OVER (PARTITION BY outlet_id ORDER BY expiry, item, warehouse) AS position
			FROM alerts
		)
		SELECT outlet_id, item, warehouse, expiry, qty_base, base_unit, expired_count, expiring_count
		FROM ranked WHERE position <= 10 ORDER BY expiry, item, warehouse`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []waExpiryAlertGroup{}
	positions := map[string]int{}
	for rows.Next() {
		var outletID string
		var line waExpiryAlertLine
		var expired, expiring int
		if err := rows.Scan(&outletID, &line.item, &line.warehouse, &line.expiry, &line.qty, &line.unit, &expired, &expiring); err != nil {
			return nil, err
		}
		outletID = strings.TrimSpace(outletID)
		position, found := positions[outletID]
		if !found {
			position = len(groups)
			positions[outletID] = position
			groups = append(groups, waExpiryAlertGroup{outletID: outletID, expired: expired, expiring: expiring})
		}
		groups[position].lines = append(groups[position].lines, line)
	}
	return groups, rows.Err()
}

func waPpicScopedVars(day string, groups []waExpiryAlertGroup, outletIDs []string) (map[string]string, bool) {
	var expired, expiring int
	lines := []waExpiryAlertLine{}
	for _, group := range groups {
		if !waDigestCoversOutlet(outletIDs, group.outletID) {
			continue
		}
		expired += group.expired
		expiring += group.expiring
		lines = append(lines, group.lines...)
	}
	if expired+expiring == 0 {
		return nil, false
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].expiry != lines[j].expiry {
			return lines[i].expiry < lines[j].expiry
		}
		if lines[i].item != lines[j].item {
			return lines[i].item < lines[j].item
		}
		return lines[i].warehouse < lines[j].warehouse
	})
	textLines := []string{}
	for i, line := range lines {
		if i >= 10 {
			break
		}
		expiry := line.expiry
		if parsed, err := time.Parse("2006-01-02", expiry); err == nil {
			expiry = parsed.Format("02-01")
		}
		textLines = append(textLines, fmt.Sprintf("• %s — %s — exp %s (%.0f %s)", line.item, line.warehouse, expiry, line.qty, line.unit))
	}
	return waPpicExpiryVars(day, expired, expiring, textLines), true
}

func waMaintenanceVars(day string, all []WAMaintenanceDue, outletIDs []string) (map[string]string, bool) {
	lines := []string{}
	for _, due := range all {
		if !waDigestCoversOutlet(outletIDs, due.OutletID) {
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s %s — %s — %s", due.WONumber, due.AssetName, due.Type, waDate(due.DueDate)))
	}
	if len(lines) == 0 {
		return nil, false
	}
	return map[string]string{"date": waDate(day), "count": fmt.Sprintf("%d", len(lines)), "list": strings.Join(lines, "\n")}, true
}
