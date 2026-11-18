package services

import (
	"cloud-pos/models"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// BuildPurchasingExcel applies the same type and location scopes as the request
// list. Split children are individual purchases; a master contributes only the
// items it still owns, rather than the totals rolled up from its children.
func BuildPurchasingExcel(outletID, workUnitID, status, requestType, projectID, parentID, search string, scopeIDs, wuScopeIDs, allowedTypes []string) ([]byte, string, error) {
	if parentID == "" {
		parentID = "all"
	}
	result, err := ListPurchaseRequests(outletID, workUnitID, status, requestType, projectID, parentID, true, search,
		scopeIDs, wuScopeIDs, allowedTypes, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}
	if result.Total > exportTxLimit {
		return nil, "", Invalid("data purchasing melebihi %d pengajuan — persempit filter sebelum mengunduh Excel", exportTxLimit)
	}
	now := time.Now().In(GetTimezoneLocation())
	data, err := buildPurchasingWorkbook(result.Requests, now)
	return data, "Purchasing-per-Vendor-" + now.Format("20060102-150405") + ".xlsx", err
}

type purchasingVendorSheet struct {
	key, name, sheet string
	requests         []models.PurchaseRequest
}

// purchasingSheetName also handles collisions caused by truncation or cleanup,
// and compares names without case as Excel does.
func purchasingSheetName(name string, used map[string]bool) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`[]:*?/\`, r) {
			return ' '
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), "'")
	if strings.TrimSpace(name) == "" {
		name = "Tanpa Vendor"
	}
	runes := []rune(name)
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = fmt.Sprintf(" (%d)", n)
		}
		length := 31 - len([]rune(suffix))
		// Excel counts UTF-16 units: an emoji uses two of the 31 units.
		base := make([]rune, 0, length)
		units := 0
		for _, r := range runes {
			width := 1
			if r > 0xFFFF {
				width = 2
			}
			if units+width > length {
				break
			}
			base = append(base, r)
			units += width
		}
		candidate := strings.TrimRight(string(base), " '") + suffix
		if !used[strings.ToLower(candidate)] {
			used[strings.ToLower(candidate)] = true
			return candidate
		}
	}
}

func buildPurchasingWorkbook(requests []models.PurchaseRequest, printedAt time.Time) ([]byte, error) {
	groups := map[string]*purchasingVendorSheet{}
	for _, pr := range requests {
		hasItems := false
		for _, g := range pr.Items {
			hasItems = hasItems || len(g.Items) > 0
		}
		if !hasItems {
			continue // fully split master: its items already belong to children
		}
		name := strings.TrimSpace(pr.VendorName)
		if name == "" {
			name = "Tanpa Vendor"
		}
		key := "name:" + name
		if pr.VendorID != nil && strings.TrimSpace(*pr.VendorID) != "" {
			key = "id:" + strings.TrimSpace(*pr.VendorID)
		}
		if groups[key] == nil {
			groups[key] = &purchasingVendorSheet{key: key, name: name}
		}
		groups[key].requests = append(groups[key].requests, pr)
	}
	vendors := make([]*purchasingVendorSheet, 0, len(groups))
	for _, group := range groups {
		vendors = append(vendors, group)
	}
	sort.Slice(vendors, func(i, j int) bool {
		a, b := strings.ToLower(vendors[i].name), strings.ToLower(vendors[j].name)
		if a == b {
			return vendors[i].key < vendors[j].key
		}
		return a < b
	})

	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Ringkasan"); err != nil {
		return nil, err
	}
	st, err := buildExportStyles(f)
	if err != nil {
		return nil, err
	}
	f.SetCellValue("Ringkasan", "A1", "LIST PURCHASING PER VENDOR")
	f.SetCellStyle("Ringkasan", "A1", "F1", st.title)
	f.SetCellValue("Ringkasan", "A2", "Dicetak: "+printedAt.Format("02 Jan 2006 15:04 MST"))
	f.SetCellValue("Ringkasan", "A3", "Setiap vendor memiliki sheet tersendiri. Qty Pengajuan tetap tersimpan; Qty Beli mengikuti penyesuaian purchasing.")
	f.SetCellValue("Ringkasan", "A4", "Qty Beli yang belum disesuaikan mengikuti Qty Pengajuan. Harga final yang belum diisi bernilai 0.")
	ppicHeaderRowAt(f, "Ringkasan", 6, []string{"Vendor", "Sheet", "Pengajuan", "Baris Item", "Total HPS", "Total Pembelian"}, st.header)
	f.SetColWidth("Ringkasan", "A", "B", 34)
	f.SetColWidth("Ringkasan", "C", "D", 14)
	f.SetColWidth("Ringkasan", "E", "F", 22)
	used := map[string]bool{"ringkasan": true}
	var totalHps, totalFinal float64
	for i, vendor := range vendors {
		vendor.sheet = purchasingSheetName(vendor.name, used)
		if _, err := f.NewSheet(vendor.sheet); err != nil {
			return nil, err
		}
		lineCount, hps, final, err := writePurchasingVendorSheet(f, vendor, st, printedAt.Location())
		if err != nil {
			return nil, err
		}
		row := i + 7
		f.SetSheetRow("Ringkasan", fmt.Sprintf("A%d", row), &[]interface{}{vendor.name, vendor.sheet, len(vendor.requests), lineCount, hps, final})
		f.SetCellStyle("Ringkasan", fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), st.text)
		f.SetCellStyle("Ringkasan", fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), st.money)
		totalHps += hps
		totalFinal += final
	}
	row := len(vendors) + 7
	if len(vendors) == 0 {
		f.SetCellValue("Ringkasan", "A7", "Tidak ada item purchasing yang sesuai filter.")
	} else {
		f.SetCellValue("Ringkasan", fmt.Sprintf("A%d", row), "TOTAL")
		f.SetCellValue("Ringkasan", fmt.Sprintf("E%d", row), totalHps)
		f.SetCellValue("Ringkasan", fmt.Sprintf("F%d", row), totalFinal)
		f.SetCellStyle("Ringkasan", fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), st.numTot)
		f.SetCellStyle("Ringkasan", fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), st.moneyTot)
		ppicAutoFilter(f, "Ringkasan", 6, row-1, 6)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writePurchasingVendorSheet(f *excelize.File, vendor *purchasingVendorSheet, st exportStyles, loc *time.Location) (int, float64, float64, error) {
	sheet := vendor.sheet
	f.SetCellValue(sheet, "A1", "VENDOR: "+vendor.name)
	f.SetCellStyle(sheet, "A1", "R1", st.title)
	headers := []string{"No. Pengajuan", "No. Induk", "Tanggal", "Jenis", "Unit Kerja", "Outlet", "Pengadaan", "Nama Item", "Qty Pengajuan", "Qty Beli", "Satuan", "HPS / Unit", "Harga Final / Unit", "Total HPS", "Total Pembelian", "Status", "No. Invoice", "Catatan"}
	ppicHeaderRowAt(f, sheet, 3, headers, st.header)
	f.SetColWidth(sheet, "A", "B", 20)
	f.SetColWidth(sheet, "C", "F", 22)
	f.SetColWidth(sheet, "G", "H", 32)
	f.SetColWidth(sheet, "I", "K", 16)
	f.SetColWidth(sheet, "L", "O", 22)
	f.SetColWidth(sheet, "P", "R", 28)
	row := 4
	var totalHps, totalFinal float64
	for _, pr := range vendor.requests {
		outlet := ""
		if pr.OutletName != nil {
			outlet = *pr.OutletName
		}
		date := pr.CreatedAt
		if created, err := time.Parse(time.RFC3339, date); err == nil {
			date = created.In(loc).Format("02 Jan 2006")
		}
		for _, group := range pr.Items {
			for _, sub := range group.Items {
				qty := sub.PurchaseQuantity()
				hps, final := float64(sub.Qty)*sub.HpsPrice, float64(qty)*sub.FinalPrice
				values := []interface{}{pr.RequestNumber, pr.ParentNumber, date, RequestTypeLabel(pr.RequestType), pr.WorkUnitName, outlet,
					group.Name, sub.Name, sub.Qty, qty, sub.Unit, sub.HpsPrice, sub.FinalPrice, hps, final, purchaseStatusLabel(pr.Status), pr.InvoiceNumber, pr.Notes}
				if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values); err != nil {
					return 0, 0, 0, err
				}
				f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("R%d", row), st.text)
				f.SetCellStyle(sheet, fmt.Sprintf("I%d", row), fmt.Sprintf("J%d", row), st.num)
				f.SetCellStyle(sheet, fmt.Sprintf("L%d", row), fmt.Sprintf("O%d", row), st.money)
				totalHps += hps
				totalFinal += final
				row++
			}
		}
	}
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "TOTAL VENDOR")
	f.SetCellValue(sheet, fmt.Sprintf("N%d", row), totalHps)
	f.SetCellValue(sheet, fmt.Sprintf("O%d", row), totalFinal)
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("M%d", row), st.numTot)
	f.SetCellStyle(sheet, fmt.Sprintf("N%d", row), fmt.Sprintf("O%d", row), st.moneyTot)
	ppicAutoFilter(f, sheet, 3, row-1, len(headers))
	return row - 4, totalHps, totalFinal, nil
}
