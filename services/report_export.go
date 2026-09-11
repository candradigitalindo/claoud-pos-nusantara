package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Batas baris export demi menjaga memori server. Jika jumlah transaksi pada
// rentang tanggal melebihi batas, sheet Detail Transaksi dipotong dan diberi
// keterangan agar pembaca tahu datanya tidak lengkap.
const (
	exportTxLimit     = 20000
	exportUnpaidLimit = 5000
)

var monthShortID = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// fmtDateID mengubah "2026-07-27" menjadi "27 Jul 2026".
func fmtDateID(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthShortID[t.Month()-1], t.Year())
}

// fmtTimeID memformat time.Time menjadi "27 Jul 2026 14:30".
func fmtTimeID(t time.Time) string {
	return fmt.Sprintf("%02d %s %d %02d:%02d", t.Day(), monthShortID[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// fmtUTCStampID mengonversi stempel waktu UTC dari service report
// ("YYYY-MM-DDTHH:MM:SSZ") ke zona waktu aplikasi lalu memformatnya.
func fmtUTCStampID(iso string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return fmtTimeID(t.In(loc))
}

var slugRe = regexp.MustCompile(`[^A-Za-z0-9]+`)

// slugFilename membersihkan label agar aman dipakai di nama file.
func slugFilename(s string) string {
	out := strings.Trim(slugRe.ReplaceAllString(s, "-"), "-")
	if out == "" {
		return "Outlet"
	}
	return out
}

// unpaidItemRow adalah bentuk minimal item di kolom JSON cloud_orders.items
// yang dipakai untuk merangkum rincian pesanan belum dibayar.
type unpaidItemRow struct {
	ProductName string  `json:"product_name"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
}

// paymentMethodLabel merapikan nilai payment_method mentah ("cash", "qris",
// "mixed") menjadi label tampilan ("Cash", "QRIS", "Mixed").
func paymentMethodLabel(m string) string {
	l := strings.ToLower(strings.TrimSpace(m))
	if l == "" {
		return ""
	}
	if l == "qris" {
		return "QRIS"
	}
	r := []rune(l)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// summarizeItems mengubah JSON items menjadi teks ringkas "Nasi Goreng ×2; Es Teh ×1".
func summarizeItems(raw string) string {
	var items []unpaidItemRow
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.ProductName) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s ×%d", it.ProductName, int(it.Qty)))
	}
	return strings.Join(parts, "; ")
}

// exportStyles menampung style ID yang dipakai berulang di semua sheet.
type exportStyles struct {
	title    int
	section  int
	label    int
	text     int
	header   int
	money    int
	moneyTot int
	numTot   int
	num      int
	note     int
	pct      int
	pctTot   int
}

func buildExportStyles(f *excelize.File) (exportStyles, error) {
	var st exportStyles
	border := []excelize.Border{
		{Type: "left", Color: "D1D5DB", Style: 1},
		{Type: "right", Color: "D1D5DB", Style: 1},
		{Type: "top", Color: "D1D5DB", Style: 1},
		{Type: "bottom", Color: "D1D5DB", Style: 1},
	}
	moneyFmt := "#,##0"
	var err error
	if st.title, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}}); err != nil {
		return st, err
	}
	if st.section, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 11, Color: "047857"}}); err != nil {
		return st, err
	}
	if st.label, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "374151"}}); err != nil {
		return st, err
	}
	if st.text, err = f.NewStyle(&excelize.Style{Border: border, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}}); err != nil {
		return st, err
	}
	if st.header, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"059669"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	}); err != nil {
		return st, err
	}
	if st.money, err = f.NewStyle(&excelize.Style{CustomNumFmt: &moneyFmt, Border: border}); err != nil {
		return st, err
	}
	if st.moneyTot, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, CustomNumFmt: &moneyFmt, Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ECFDF5"}}}); err != nil {
		return st, err
	}
	if st.numTot, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ECFDF5"}}}); err != nil {
		return st, err
	}
	if st.num, err = f.NewStyle(&excelize.Style{Border: border}); err != nil {
		return st, err
	}
	if st.note, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Italic: true, Color: "92400E"}}); err != nil {
		return st, err
	}
	pctFmt := "0.0%"
	if st.pct, err = f.NewStyle(&excelize.Style{CustomNumFmt: &pctFmt, Border: border}); err != nil {
		return st, err
	}
	if st.pctTot, err = f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, CustomNumFmt: &pctFmt, Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ECFDF5"}}}); err != nil {
		return st, err
	}
	return st, nil
}

// setHeaderRow menulis baris judul kolom (baris 1) dengan style header.
func setHeaderRow(f *excelize.File, sheet string, headers []string, styleID int) {
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	last, _ := excelize.CoordinatesToCellName(len(headers), 1)
	f.SetCellStyle(sheet, "A1", last, styleID)
	f.SetRowHeight(sheet, 1, 22)
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

// BuildSalesReportExcel menyusun file Excel laporan penjualan untuk rentang
// tanggal & scope outlet yang sama persis dengan tampilan halaman /sales-report:
// scopeIDs berasal dari role admin (getOutletScope), outletID dari filter UI.
// Mengembalikan isi file .xlsx beserta nama file yang disarankan.
func BuildSalesReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetSalesReport(dateFrom, dateTo, outletID, scopeIDs, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}
	unpaid, err := GetUnpaidOrders(outletID, "", dateFrom, dateTo, scopeIDs, 1, exportUnpaidLimit)
	if err != nil {
		// Sheet Belum Dibayar bersifat pelengkap — jangan gagalkan seluruh export.
		log.Printf("BuildSalesReportExcel unpaid warning: %v", err)
		unpaid = &models.UnpaidOrdersResponse{}
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()

	// Label outlet untuk header laporan & nama file.
	outletLabel := resolveOutletLabel(outletID, scopeIDs)

	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shDaily   = "Harian"
		shOutlet  = "Per Outlet"
		shTx      = "Detail Transaksi"
		shUnpaid  = "Belum Dibayar"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shDaily, shOutlet, shTx, shUnpaid} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 26)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN PENJUALAN")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	totalPax := 0
	for _, d := range report.Daily {
		totalPax += d.TotalPax
	}

	setSection("RINGKASAN")
	setKV("Total Transaksi", report.Summary.TotalTransactions, st.num)
	setKV("Total Pendapatan", report.Summary.TotalRevenue, st.money)
	setKV("Rata-rata / Transaksi", report.Summary.AvgPerTransaction, st.money)
	setKV("Total Tamu (pax)", totalPax, st.num)
	row++

	setSection("METODE PEMBAYARAN")
	setKV("Cash", report.Summary.CashRevenue, st.money)
	setKV("QRIS", report.Summary.QrisRevenue, st.money)
	setKV("Card", report.Summary.CardRevenue, st.money)
	setKV("Transfer", report.Summary.TransferRevenue, st.money)
	setKV("Total per Metode", report.Summary.CashRevenue+report.Summary.QrisRevenue+report.Summary.CardRevenue+report.Summary.TransferRevenue, st.moneyTot)
	row++

	setSection("BELUM DIBAYAR")
	setKV("Jumlah Pesanan", report.Summary.UnpaidOrders, st.num)
	setKV("Nominal", report.Summary.UnpaidAmount, st.money)

	// ── Sheet Harian ─────────────────────────────────────────────────────
	setHeaderRow(f, shDaily, []string{"Tanggal", "Transaksi", "Tamu (pax)", "Cash", "QRIS", "Card", "Transfer", "Total"}, st.header)
	f.SetColWidth(shDaily, "A", "A", 14)
	f.SetColWidth(shDaily, "B", "C", 11)
	f.SetColWidth(shDaily, "D", "H", 14)

	// Service mengembalikan urutan DESC; untuk dibaca manusia urutkan naik.
	daily := make([]models.SalesReportRow, len(report.Daily))
	copy(daily, report.Daily)
	for i, j := 0, len(daily)-1; i < j; i, j = i+1, j-1 {
		daily[i], daily[j] = daily[j], daily[i]
	}
	var sumTx, sumPax int
	var sumCash, sumQris, sumCard, sumTransfer, sumTotal float64
	for i, d := range daily {
		r := i + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), fmtDateID(d.Date))
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), d.TotalTransactions)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), d.TotalPax)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), d.CashRevenue)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), d.QrisRevenue)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), d.CardRevenue)
		f.SetCellValue(shDaily, fmt.Sprintf("G%d", r), d.TransferRevenue)
		f.SetCellValue(shDaily, fmt.Sprintf("H%d", r), d.TotalRevenue)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), st.num)
		f.SetCellStyle(shDaily, fmt.Sprintf("D%d", r), fmt.Sprintf("H%d", r), st.money)
		sumTx += d.TotalTransactions
		sumPax += d.TotalPax
		sumCash += d.CashRevenue
		sumQris += d.QrisRevenue
		sumCard += d.CardRevenue
		sumTransfer += d.TransferRevenue
		sumTotal += d.TotalRevenue
	}
	if len(daily) > 0 {
		r := len(daily) + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), sumTx)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), sumPax)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), sumCash)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), sumQris)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), sumCard)
		f.SetCellValue(shDaily, fmt.Sprintf("G%d", r), sumTransfer)
		f.SetCellValue(shDaily, fmt.Sprintf("H%d", r), sumTotal)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), st.numTot)
		f.SetCellStyle(shDaily, fmt.Sprintf("D%d", r), fmt.Sprintf("H%d", r), st.moneyTot)
	}

	// ── Sheet Per Outlet ─────────────────────────────────────────────────
	setHeaderRow(f, shOutlet, []string{"Outlet", "Transaksi", "Pendapatan", "Pesanan Belum Bayar", "Nominal Belum Bayar"}, st.header)
	f.SetColWidth(shOutlet, "A", "A", 28)
	f.SetColWidth(shOutlet, "B", "B", 11)
	f.SetColWidth(shOutlet, "C", "E", 18)
	var oSumTx, oSumUnpaid int
	var oSumRev, oSumUnpaidAmt float64
	for i, o := range report.ByOutlet {
		r := i + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), o.OutletName)
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), o.TotalTransactions)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), o.TotalRevenue)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), o.UnpaidOrders)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), o.UnpaidAmount)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.num)
		f.SetCellStyle(shOutlet, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), st.money)
		f.SetCellStyle(shOutlet, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), st.num)
		f.SetCellStyle(shOutlet, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.money)
		oSumTx += o.TotalTransactions
		oSumRev += o.TotalRevenue
		oSumUnpaid += o.UnpaidOrders
		oSumUnpaidAmt += o.UnpaidAmount
	}
	if len(report.ByOutlet) > 0 {
		r := len(report.ByOutlet) + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), oSumTx)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), oSumRev)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), oSumUnpaid)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), oSumUnpaidAmt)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.numTot)
		f.SetCellStyle(shOutlet, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.moneyTot)
	}

	// ── Sheet Detail Transaksi ───────────────────────────────────────────
	setHeaderRow(f, shTx, []string{"No", "Waktu", "Outlet", "Pemesan", "Tamu (pax)", "Kasir", "Metode Bayar", "Total"}, st.header)
	f.SetColWidth(shTx, "A", "A", 6)
	f.SetColWidth(shTx, "B", "B", 18)
	f.SetColWidth(shTx, "C", "D", 22)
	f.SetColWidth(shTx, "E", "E", 10)
	f.SetColWidth(shTx, "F", "G", 15)
	f.SetColWidth(shTx, "H", "H", 14)

	// Service mengurutkan DESC (terbaru dulu); untuk laporan urutkan kronologis.
	txs := make([]models.SalesReportTransaction, len(report.Transactions))
	copy(txs, report.Transactions)
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}
	for i, t := range txs {
		r := i + 2
		f.SetCellValue(shTx, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shTx, fmt.Sprintf("B%d", r), fmtUTCStampID(t.CreatedAt, loc))
		f.SetCellValue(shTx, fmt.Sprintf("C%d", r), t.OutletName)
		f.SetCellValue(shTx, fmt.Sprintf("D%d", r), t.OrdererName)
		f.SetCellValue(shTx, fmt.Sprintf("E%d", r), t.Pax)
		f.SetCellValue(shTx, fmt.Sprintf("F%d", r), t.CashierName)
		f.SetCellValue(shTx, fmt.Sprintf("G%d", r), paymentMethodLabel(t.PaymentMethod))
		f.SetCellValue(shTx, fmt.Sprintf("H%d", r), t.TotalAmount)
		f.SetCellStyle(shTx, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.num)
		f.SetCellStyle(shTx, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), st.money)
	}
	if len(txs) > 0 {
		r := len(txs) + 2
		f.SetCellValue(shTx, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shTx, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r))
		f.SetCellValue(shTx, fmt.Sprintf("H%d", r), report.Summary.TotalRevenue)
		f.SetCellStyle(shTx, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.numTot)
		f.SetCellStyle(shTx, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), st.moneyTot)
	}
	if report.Total > len(report.Transactions) {
		r := len(txs) + 4
		f.SetCellValue(shTx, fmt.Sprintf("A%d", r),
			fmt.Sprintf("Catatan: menampilkan %d dari %d transaksi. Persempit rentang tanggal untuk data lengkap.",
				len(report.Transactions), report.Total))
		f.SetCellStyle(shTx, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.note)
	}

	// ── Sheet Belum Dibayar ──────────────────────────────────────────────
	setHeaderRow(f, shUnpaid, []string{"No", "Waktu", "Outlet", "Pelanggan", "Meja", "Tamu (pax)", "Status", "Total", "Rincian Item"}, st.header)
	f.SetColWidth(shUnpaid, "A", "A", 6)
	f.SetColWidth(shUnpaid, "B", "B", 18)
	f.SetColWidth(shUnpaid, "C", "D", 20)
	f.SetColWidth(shUnpaid, "E", "G", 10)
	f.SetColWidth(shUnpaid, "H", "H", 14)
	f.SetColWidth(shUnpaid, "I", "I", 50)
	for i, o := range unpaid.Orders {
		r := i + 2
		f.SetCellValue(shUnpaid, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shUnpaid, fmt.Sprintf("B%d", r), fmtUTCStampID(o.CreatedAt, loc))
		f.SetCellValue(shUnpaid, fmt.Sprintf("C%d", r), o.OutletName)
		f.SetCellValue(shUnpaid, fmt.Sprintf("D%d", r), o.CustomerName)
		f.SetCellValue(shUnpaid, fmt.Sprintf("E%d", r), o.TableNumber)
		f.SetCellValue(shUnpaid, fmt.Sprintf("F%d", r), o.Pax)
		f.SetCellValue(shUnpaid, fmt.Sprintf("G%d", r), o.Status)
		f.SetCellValue(shUnpaid, fmt.Sprintf("H%d", r), o.TotalAmount)
		f.SetCellValue(shUnpaid, fmt.Sprintf("I%d", r), summarizeItems(o.Items))
		f.SetCellStyle(shUnpaid, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.num)
		f.SetCellStyle(shUnpaid, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), st.money)
		f.SetCellStyle(shUnpaid, fmt.Sprintf("I%d", r), fmt.Sprintf("I%d", r), st.text)
	}
	if len(unpaid.Orders) > 0 {
		r := len(unpaid.Orders) + 2
		f.SetCellValue(shUnpaid, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shUnpaid, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r))
		f.SetCellValue(shUnpaid, fmt.Sprintf("H%d", r), unpaid.TotalAmount)
		f.SetCellStyle(shUnpaid, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.numTot)
		f.SetCellStyle(shUnpaid, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), st.moneyTot)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Penjualan_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// resolveOutletLabel menentukan label outlet untuk judul laporan & nama file:
// nama outlet jika difilter satu outlet, selain itu "Semua Outlet".
func resolveOutletLabel(outletID string, scopeIDs []string) string {
	if outletID != "" {
		var name string
		if err := database.DB.QueryRow(`SELECT name FROM outlets WHERE id = $1`, outletID).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
			return name
		}
		return outletID
	}
	if scopeIDs != nil {
		return "Semua Outlet (sesuai hak akses)"
	}
	return "Semua Outlet"
}

// BuildProductSalesReportExcel menyusun file Excel laporan penjualan per produk
// dengan filter yang sama persis dengan halaman /product-sales-report: rentang
// tanggal, outlet, dan urutan (revenue|qty × asc|desc). scopeIDs berasal dari
// role admin sehingga hasil export tak pernah melebihi hak akses user.
func BuildProductSalesReportExcel(dateFrom, dateTo, outletID, sortBy, sortDir string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetProductSalesReport(dateFrom, dateTo, outletID, sortBy, sortDir, scopeIDs, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	sortLabel := "Pendapatan"
	if sortBy == "qty" {
		sortLabel = "Qty terjual"
	}
	if strings.EqualFold(sortDir, "asc") {
		sortLabel += " (Terendah)"
	} else {
		sortLabel += " (Tertinggi)"
	}

	grandRevenue := report.TotalRevenue
	// pctOf menghasilkan fraksi kontribusi pendapatan (ditampilkan sebagai %).
	pctOf := func(rev float64) float64 {
		if grandRevenue == 0 {
			return 0
		}
		return rev / grandRevenue
	}

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary  = "Ringkasan"
		shProduct  = "Per Produk"
		shCategory = "Per Kategori"
		shOutlet   = "Per Outlet"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shProduct, shCategory, shOutlet} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 26)
	f.SetColWidth(shSummary, "B", "B", 34)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN PENJUALAN PER PRODUK")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Diurutkan berdasarkan", sortLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	setSection("RINGKASAN")
	setKV("Total Produk Terjual (jenis)", report.Total, st.num)
	setKV("Total Qty Terjual", report.TotalQty, st.num)
	setKV("Total Pendapatan", report.TotalRevenue, st.money)
	row++

	// 5 produk penyumbang pendapatan terbesar, agar pembaca langsung menangkap
	// gambaran tanpa membuka sheet detail.
	topByRevenue := make([]models.ProductSalesRow, len(report.Items))
	copy(topByRevenue, report.Items)
	sort.SliceStable(topByRevenue, func(i, j int) bool {
		return topByRevenue[i].TotalRevenue > topByRevenue[j].TotalRevenue
	})
	if len(topByRevenue) > 0 {
		setSection("5 PRODUK PENDAPATAN TERTINGGI")
		for i, p := range topByRevenue {
			if i >= 5 {
				break
			}
			setKV(fmt.Sprintf("%d. %s", i+1, p.ProductName), p.TotalRevenue, st.money)
		}
	}

	// ── Sheet Per Produk ─────────────────────────────────────────────────
	setHeaderRow(f, shProduct, []string{"No", "Nama Produk", "Outlet", "Kategori", "Qty", "Pendapatan", "% Kontribusi"}, st.header)
	f.SetColWidth(shProduct, "A", "A", 6)
	f.SetColWidth(shProduct, "B", "B", 34)
	f.SetColWidth(shProduct, "C", "D", 22)
	f.SetColWidth(shProduct, "E", "E", 9)
	f.SetColWidth(shProduct, "F", "F", 15)
	f.SetColWidth(shProduct, "G", "G", 13)

	var pSumQty int
	var pSumRev float64
	for i, p := range report.Items {
		r := i + 2
		f.SetCellValue(shProduct, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shProduct, fmt.Sprintf("B%d", r), p.ProductName)
		f.SetCellValue(shProduct, fmt.Sprintf("C%d", r), p.OutletName)
		f.SetCellValue(shProduct, fmt.Sprintf("D%d", r), p.CategoryName)
		f.SetCellValue(shProduct, fmt.Sprintf("E%d", r), p.TotalQty)
		f.SetCellValue(shProduct, fmt.Sprintf("F%d", r), p.TotalRevenue)
		f.SetCellValue(shProduct, fmt.Sprintf("G%d", r), pctOf(p.TotalRevenue))
		f.SetCellStyle(shProduct, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r), st.num)
		f.SetCellStyle(shProduct, fmt.Sprintf("F%d", r), fmt.Sprintf("F%d", r), st.money)
		f.SetCellStyle(shProduct, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), st.pct)
		pSumQty += p.TotalQty
		pSumRev += p.TotalRevenue
	}
	if len(report.Items) > 0 {
		r := len(report.Items) + 2
		f.SetCellValue(shProduct, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shProduct, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r))
		f.SetCellValue(shProduct, fmt.Sprintf("E%d", r), pSumQty)
		f.SetCellValue(shProduct, fmt.Sprintf("F%d", r), pSumRev)
		f.SetCellValue(shProduct, fmt.Sprintf("G%d", r), pctOf(pSumRev))
		f.SetCellStyle(shProduct, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r), st.numTot)
		f.SetCellStyle(shProduct, fmt.Sprintf("F%d", r), fmt.Sprintf("F%d", r), st.moneyTot)
		f.SetCellStyle(shProduct, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), st.pctTot)
	}
	if report.Total > len(report.Items) {
		r := len(report.Items) + 4
		f.SetCellValue(shProduct, fmt.Sprintf("A%d", r),
			fmt.Sprintf("Catatan: menampilkan %d dari %d baris produk. Persempit rentang tanggal untuk data lengkap.",
				len(report.Items), report.Total))
		f.SetCellStyle(shProduct, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.note)
	}

	// ── Rekap Per Kategori & Per Outlet ──────────────────────────────────
	type aggRow struct {
		name     string
		products int
		qty      int
		revenue  float64
	}
	aggregate := func(keyFn func(models.ProductSalesRow) string) []aggRow {
		m := map[string]*aggRow{}
		for _, p := range report.Items {
			k := keyFn(p)
			a, ok := m[k]
			if !ok {
				a = &aggRow{name: k}
				m[k] = a
			}
			a.products++
			a.qty += p.TotalQty
			a.revenue += p.TotalRevenue
		}
		out := make([]aggRow, 0, len(m))
		for _, a := range m {
			out = append(out, *a)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].revenue > out[j].revenue })
		return out
	}
	writeAgg := func(sheet, nameHeader string, rows []aggRow) {
		setHeaderRow(f, sheet, []string{nameHeader, "Jenis Produk", "Qty", "Pendapatan", "% Kontribusi"}, st.header)
		f.SetColWidth(sheet, "A", "A", 30)
		f.SetColWidth(sheet, "B", "C", 12)
		f.SetColWidth(sheet, "D", "D", 15)
		f.SetColWidth(sheet, "E", "E", 13)
		var sumProducts, sumQty int
		var sumRev float64
		for i, a := range rows {
			r := i + 2
			f.SetCellValue(sheet, fmt.Sprintf("A%d", r), a.name)
			f.SetCellValue(sheet, fmt.Sprintf("B%d", r), a.products)
			f.SetCellValue(sheet, fmt.Sprintf("C%d", r), a.qty)
			f.SetCellValue(sheet, fmt.Sprintf("D%d", r), a.revenue)
			f.SetCellValue(sheet, fmt.Sprintf("E%d", r), pctOf(a.revenue))
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), st.num)
			f.SetCellStyle(sheet, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), st.money)
			f.SetCellStyle(sheet, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.pct)
			sumProducts += a.products
			sumQty += a.qty
			sumRev += a.revenue
		}
		if len(rows) > 0 {
			r := len(rows) + 2
			f.SetCellValue(sheet, fmt.Sprintf("A%d", r), "TOTAL")
			f.SetCellValue(sheet, fmt.Sprintf("B%d", r), sumProducts)
			f.SetCellValue(sheet, fmt.Sprintf("C%d", r), sumQty)
			f.SetCellValue(sheet, fmt.Sprintf("D%d", r), sumRev)
			f.SetCellValue(sheet, fmt.Sprintf("E%d", r), pctOf(sumRev))
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), st.numTot)
			f.SetCellStyle(sheet, fmt.Sprintf("D%d", r), fmt.Sprintf("D%d", r), st.moneyTot)
			f.SetCellStyle(sheet, fmt.Sprintf("E%d", r), fmt.Sprintf("E%d", r), st.pctTot)
		}
	}
	writeAgg(shCategory, "Kategori", aggregate(func(p models.ProductSalesRow) string { return p.CategoryName }))
	writeAgg(shOutlet, "Outlet", aggregate(func(p models.ProductSalesRow) string { return p.OutletName }))

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Penjualan-Produk_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildCashierShiftReportExcel menyusun file Excel laporan shift kasir dengan
// filter yang sama persis dengan halaman /cashier-shifts: outlet, status
// (open|closed), dan rentang tanggal (boleh kosong = semua tanggal). scopeIDs
// berasal dari role admin sehingga hasil export tak melebihi hak akses user.
func BuildCashierShiftReportExcel(outletID, status, dateFrom, dateTo string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetCashierShiftReport(outletID, status, dateFrom, dateTo, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	periodLabel := "Semua Tanggal"
	if dateFrom != "" || dateTo != "" {
		from, to := dateFrom, dateTo
		if from == "" {
			from = "…"
		} else {
			from = fmtDateID(from)
		}
		if to == "" {
			to = "…"
		} else {
			to = fmtDateID(to)
		}
		periodLabel = from + " s/d " + to
	}
	statusLabel := "Semua Status"
	switch status {
	case "open":
		statusLabel = "Berjalan"
	case "closed":
		statusLabel = "Tutup"
	}

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}
	// Style khusus kolom Selisih/Keterangan agar shift bermasalah langsung terlihat.
	border := []excelize.Border{
		{Type: "left", Color: "D1D5DB", Style: 1},
		{Type: "right", Color: "D1D5DB", Style: 1},
		{Type: "top", Color: "D1D5DB", Style: 1},
		{Type: "bottom", Color: "D1D5DB", Style: 1},
	}
	moneyFmt := "#,##0"
	stMoneyBad, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "B91C1C"}, CustomNumFmt: &moneyFmt, Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FEF2F2"}}})
	if err != nil {
		return nil, "", err
	}
	stMoneyWarn, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "B45309"}, CustomNumFmt: &moneyFmt, Border: border, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFFBEB"}}})
	if err != nil {
		return nil, "", err
	}
	stTextOk, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "047857"}, Border: border})
	if err != nil {
		return nil, "", err
	}
	stTextBad, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "B91C1C"}, Border: border})
	if err != nil {
		return nil, "", err
	}
	stTextWarn, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "B45309"}, Border: border})
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary  = "Ringkasan"
		shShifts   = "Daftar Shift"
		shMethod   = "Per Metode"
		shMovement = "Kas Masuk-Keluar"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shShifts, shMethod, shMovement} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 28)
	f.SetColWidth(shSummary, "B", "B", 32)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN SHIFT KASIR")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", periodLabel, 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Status", statusLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("RINGKASAN SHIFT")
	setKV("Total Shift", sum.TotalShifts, st.num)
	setKV("Shift Berjalan", sum.OpenShifts, st.num)
	setKV("Shift Tutup", sum.ClosedShifts, st.num)
	setKV("Balance (tanpa selisih)", sum.BalancedCount, st.num)
	setKV("Tidak Balance (miss)", sum.MissCount, st.num)
	row++

	setSection("SELISIH KAS (shift tertutup)")
	setKV("Total Selisih", sum.TotalVariance, st.money)
	setKV("Total Kurang (shortage)", sum.ShortageTotal, st.money)
	setKV("Total Lebih (overage)", sum.OverageTotal, st.money)
	row++

	setSection("PENJUALAN & KAS")
	setKV("Total Penjualan", sum.TotalSales, st.money)
	setKV("Total Kas Masuk (non-transaksi)", sum.TotalCashIn, st.money)
	setKV("Total Kas Keluar (non-transaksi)", sum.TotalCashOut, st.money)
	row++
	f.SetCellValue(shSummary, fmt.Sprintf("A%d", row),
		"Keterangan: \"Kas Seharusnya\" = kas awal + penjualan tunai + kas masuk − kas keluar. Selisih = kas akhir − kas seharusnya (negatif berarti kurang).")
	f.MergeCell(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.note)

	// ── Sheet Daftar Shift ───────────────────────────────────────────────
	setHeaderRow(f, shShifts, []string{
		"No", "Outlet", "Kasir", "Serah Terima Ke", "Buka", "Tutup", "Status",
		"Kas Awal", "Penjualan Tunai", "Kas Masuk", "Kas Keluar", "Kas Seharusnya",
		"Kas Akhir", "Selisih", "Keterangan", "Penjualan Total", "Jml Trx", "Sumber Data",
	}, st.header)
	f.SetColWidth(shShifts, "A", "A", 5)
	f.SetColWidth(shShifts, "B", "B", 20)
	f.SetColWidth(shShifts, "C", "D", 16)
	f.SetColWidth(shShifts, "E", "F", 17)
	f.SetColWidth(shShifts, "G", "G", 9)
	f.SetColWidth(shShifts, "H", "N", 14)
	f.SetColWidth(shShifts, "O", "O", 16)
	f.SetColWidth(shShifts, "P", "P", 15)
	f.SetColWidth(shShifts, "Q", "Q", 8)
	f.SetColWidth(shShifts, "R", "R", 12)

	var tOpen, tCashSales, tIn, tOut, tExpected, tClosing, tVar, tSales float64
	var tTrx int
	for i, sh := range report.Shifts {
		r := i + 2
		closed := sh.Status == "closed"

		statusTxt := "Berjalan"
		if closed {
			statusTxt = "Tutup"
		}
		ket := "Berjalan"
		ketStyle := st.num
		if closed {
			switch {
			case sh.Balanced:
				ket = "Balance"
				ketStyle = stTextOk
			case sh.Variance < 0:
				ket = "Kurang"
				ketStyle = stTextBad
			default:
				ket = "Lebih"
				ketStyle = stTextWarn
			}
		}
		source := "Device"
		if sh.SalesSource == "cloud" {
			source = "Cloud (belum sinkron penuh)"
		}

		f.SetCellValue(shShifts, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shShifts, fmt.Sprintf("B%d", r), sh.OutletName)
		f.SetCellValue(shShifts, fmt.Sprintf("C%d", r), sh.OpenedBy)
		f.SetCellValue(shShifts, fmt.Sprintf("D%d", r), sh.HandoverTo)
		f.SetCellValue(shShifts, fmt.Sprintf("E%d", r), sh.OpenedAt)
		f.SetCellValue(shShifts, fmt.Sprintf("F%d", r), sh.ClosedAt)
		f.SetCellValue(shShifts, fmt.Sprintf("G%d", r), statusTxt)
		f.SetCellValue(shShifts, fmt.Sprintf("H%d", r), sh.OpeningCash)
		f.SetCellValue(shShifts, fmt.Sprintf("I%d", r), sh.CashSales)
		f.SetCellValue(shShifts, fmt.Sprintf("J%d", r), sh.CashIn)
		f.SetCellValue(shShifts, fmt.Sprintf("K%d", r), sh.CashOut)
		f.SetCellValue(shShifts, fmt.Sprintf("L%d", r), sh.ExpectedCash)
		if closed {
			f.SetCellValue(shShifts, fmt.Sprintf("M%d", r), sh.ClosingCash)
			f.SetCellValue(shShifts, fmt.Sprintf("N%d", r), sh.Variance)
		}
		f.SetCellValue(shShifts, fmt.Sprintf("O%d", r), ket)
		f.SetCellValue(shShifts, fmt.Sprintf("P%d", r), sh.SalesTotal)
		f.SetCellValue(shShifts, fmt.Sprintf("Q%d", r), sh.SalesCount)
		f.SetCellValue(shShifts, fmt.Sprintf("R%d", r), source)

		f.SetCellStyle(shShifts, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.num)
		f.SetCellStyle(shShifts, fmt.Sprintf("H%d", r), fmt.Sprintf("M%d", r), st.money)
		varStyle := st.money
		if closed && !sh.Balanced {
			if sh.Variance < 0 {
				varStyle = stMoneyBad
			} else {
				varStyle = stMoneyWarn
			}
		}
		f.SetCellStyle(shShifts, fmt.Sprintf("N%d", r), fmt.Sprintf("N%d", r), varStyle)
		f.SetCellStyle(shShifts, fmt.Sprintf("O%d", r), fmt.Sprintf("O%d", r), ketStyle)
		f.SetCellStyle(shShifts, fmt.Sprintf("P%d", r), fmt.Sprintf("P%d", r), st.money)
		f.SetCellStyle(shShifts, fmt.Sprintf("Q%d", r), fmt.Sprintf("R%d", r), st.num)

		tOpen += sh.OpeningCash
		tCashSales += sh.CashSales
		tIn += sh.CashIn
		tOut += sh.CashOut
		tExpected += sh.ExpectedCash
		if closed {
			tClosing += sh.ClosingCash
			tVar += sh.Variance
		}
		tSales += sh.SalesTotal
		tTrx += sh.SalesCount
	}
	if len(report.Shifts) > 0 {
		r := len(report.Shifts) + 2
		f.SetCellValue(shShifts, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shShifts, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r))
		f.SetCellValue(shShifts, fmt.Sprintf("H%d", r), tOpen)
		f.SetCellValue(shShifts, fmt.Sprintf("I%d", r), tCashSales)
		f.SetCellValue(shShifts, fmt.Sprintf("J%d", r), tIn)
		f.SetCellValue(shShifts, fmt.Sprintf("K%d", r), tOut)
		f.SetCellValue(shShifts, fmt.Sprintf("L%d", r), tExpected)
		f.SetCellValue(shShifts, fmt.Sprintf("M%d", r), tClosing)
		f.SetCellValue(shShifts, fmt.Sprintf("N%d", r), tVar)
		f.SetCellValue(shShifts, fmt.Sprintf("P%d", r), tSales)
		f.SetCellValue(shShifts, fmt.Sprintf("Q%d", r), tTrx)
		f.SetCellStyle(shShifts, fmt.Sprintf("A%d", r), fmt.Sprintf("G%d", r), st.numTot)
		f.SetCellStyle(shShifts, fmt.Sprintf("H%d", r), fmt.Sprintf("N%d", r), st.moneyTot)
		f.SetCellStyle(shShifts, fmt.Sprintf("O%d", r), fmt.Sprintf("O%d", r), st.numTot)
		f.SetCellStyle(shShifts, fmt.Sprintf("P%d", r), fmt.Sprintf("P%d", r), st.moneyTot)
		f.SetCellStyle(shShifts, fmt.Sprintf("Q%d", r), fmt.Sprintf("R%d", r), st.numTot)
	}

	// ── Sheet Per Metode ─────────────────────────────────────────────────
	setHeaderRow(f, shMethod, []string{"No", "Outlet", "Kasir", "Buka", "Cash", "QRIS", "Card", "Transfer", "Total", "Jml Trx"}, st.header)
	f.SetColWidth(shMethod, "A", "A", 5)
	f.SetColWidth(shMethod, "B", "B", 20)
	f.SetColWidth(shMethod, "C", "C", 16)
	f.SetColWidth(shMethod, "D", "D", 17)
	f.SetColWidth(shMethod, "E", "I", 14)
	f.SetColWidth(shMethod, "J", "J", 8)
	methodTotals := map[string]float64{}
	for i, sh := range report.Shifts {
		r := i + 2
		f.SetCellValue(shMethod, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shMethod, fmt.Sprintf("B%d", r), sh.OutletName)
		f.SetCellValue(shMethod, fmt.Sprintf("C%d", r), sh.OpenedBy)
		f.SetCellValue(shMethod, fmt.Sprintf("D%d", r), sh.OpenedAt)
		for j, m := range []string{"cash", "qris", "card", "transfer"} {
			var v float64
			if mt, ok := sh.ByMethod[m]; ok {
				v = mt.Total
			}
			cell, _ := excelize.CoordinatesToCellName(5+j, r)
			f.SetCellValue(shMethod, cell, v)
			methodTotals[m] += v
		}
		f.SetCellValue(shMethod, fmt.Sprintf("I%d", r), sh.SalesTotal)
		f.SetCellValue(shMethod, fmt.Sprintf("J%d", r), sh.SalesCount)
		f.SetCellStyle(shMethod, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.num)
		f.SetCellStyle(shMethod, fmt.Sprintf("E%d", r), fmt.Sprintf("I%d", r), st.money)
		f.SetCellStyle(shMethod, fmt.Sprintf("J%d", r), fmt.Sprintf("J%d", r), st.num)
	}
	if len(report.Shifts) > 0 {
		r := len(report.Shifts) + 2
		f.SetCellValue(shMethod, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shMethod, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r))
		f.SetCellValue(shMethod, fmt.Sprintf("E%d", r), methodTotals["cash"])
		f.SetCellValue(shMethod, fmt.Sprintf("F%d", r), methodTotals["qris"])
		f.SetCellValue(shMethod, fmt.Sprintf("G%d", r), methodTotals["card"])
		f.SetCellValue(shMethod, fmt.Sprintf("H%d", r), methodTotals["transfer"])
		f.SetCellValue(shMethod, fmt.Sprintf("I%d", r), tSales)
		f.SetCellValue(shMethod, fmt.Sprintf("J%d", r), tTrx)
		f.SetCellStyle(shMethod, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.numTot)
		f.SetCellStyle(shMethod, fmt.Sprintf("E%d", r), fmt.Sprintf("I%d", r), st.moneyTot)
		f.SetCellStyle(shMethod, fmt.Sprintf("J%d", r), fmt.Sprintf("J%d", r), st.numTot)
	}

	// ── Sheet Kas Masuk-Keluar (di luar transaksi) ───────────────────────
	setHeaderRow(f, shMovement, []string{"No", "Outlet", "Kasir", "Shift Buka", "Waktu", "Jenis", "Jumlah", "Pihak", "Catatan"}, st.header)
	f.SetColWidth(shMovement, "A", "A", 5)
	f.SetColWidth(shMovement, "B", "B", 20)
	f.SetColWidth(shMovement, "C", "C", 16)
	f.SetColWidth(shMovement, "D", "E", 17)
	f.SetColWidth(shMovement, "F", "F", 9)
	f.SetColWidth(shMovement, "G", "G", 14)
	f.SetColWidth(shMovement, "H", "H", 18)
	f.SetColWidth(shMovement, "I", "I", 40)
	mRow := 2
	no := 0
	var mIn, mOut float64
	for _, sh := range report.Shifts {
		for _, m := range sh.Movements {
			no++
			jenis := "Keluar"
			if m.Type == "in" {
				jenis = "Masuk"
				mIn += m.Amount
			} else {
				mOut += m.Amount
			}
			f.SetCellValue(shMovement, fmt.Sprintf("A%d", mRow), no)
			f.SetCellValue(shMovement, fmt.Sprintf("B%d", mRow), sh.OutletName)
			f.SetCellValue(shMovement, fmt.Sprintf("C%d", mRow), sh.OpenedBy)
			f.SetCellValue(shMovement, fmt.Sprintf("D%d", mRow), sh.OpenedAt)
			f.SetCellValue(shMovement, fmt.Sprintf("E%d", mRow), m.CreatedAt)
			f.SetCellValue(shMovement, fmt.Sprintf("F%d", mRow), jenis)
			f.SetCellValue(shMovement, fmt.Sprintf("G%d", mRow), m.Amount)
			f.SetCellValue(shMovement, fmt.Sprintf("H%d", mRow), m.CounterpartName)
			f.SetCellValue(shMovement, fmt.Sprintf("I%d", mRow), m.Note)
			f.SetCellStyle(shMovement, fmt.Sprintf("A%d", mRow), fmt.Sprintf("F%d", mRow), st.num)
			f.SetCellStyle(shMovement, fmt.Sprintf("G%d", mRow), fmt.Sprintf("G%d", mRow), st.money)
			f.SetCellStyle(shMovement, fmt.Sprintf("H%d", mRow), fmt.Sprintf("H%d", mRow), st.num)
			f.SetCellStyle(shMovement, fmt.Sprintf("I%d", mRow), fmt.Sprintf("I%d", mRow), st.text)
			mRow++
		}
	}
	if no > 0 {
		f.SetCellValue(shMovement, fmt.Sprintf("A%d", mRow), "TOTAL — Masuk")
		f.MergeCell(shMovement, fmt.Sprintf("A%d", mRow), fmt.Sprintf("F%d", mRow))
		f.SetCellValue(shMovement, fmt.Sprintf("G%d", mRow), mIn)
		f.SetCellStyle(shMovement, fmt.Sprintf("A%d", mRow), fmt.Sprintf("F%d", mRow), st.numTot)
		f.SetCellStyle(shMovement, fmt.Sprintf("G%d", mRow), fmt.Sprintf("G%d", mRow), st.moneyTot)
		mRow++
		f.SetCellValue(shMovement, fmt.Sprintf("A%d", mRow), "TOTAL — Keluar")
		f.MergeCell(shMovement, fmt.Sprintf("A%d", mRow), fmt.Sprintf("F%d", mRow))
		f.SetCellValue(shMovement, fmt.Sprintf("G%d", mRow), mOut)
		f.SetCellStyle(shMovement, fmt.Sprintf("A%d", mRow), fmt.Sprintf("F%d", mRow), st.numTot)
		f.SetCellStyle(shMovement, fmt.Sprintf("G%d", mRow), fmt.Sprintf("G%d", mRow), st.moneyTot)
	} else {
		f.SetCellValue(shMovement, "A2", "Tidak ada kas masuk/keluar di luar transaksi pada periode ini.")
		f.SetCellStyle(shMovement, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	fromPart, toPart := dateFrom, dateTo
	if fromPart == "" {
		fromPart = "awal"
	}
	if toPart == "" {
		toPart = "akhir"
	}
	filename := fmt.Sprintf("Laporan-Shift-Kasir_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), fromPart, toPart)
	return buf.Bytes(), filename, nil
}

// purchaseStatusLabel menerjemahkan status pengajuan pengadaan ke label
// tampilan yang sama dengan halaman /procurement-payments.
func purchaseStatusLabel(s string) string {
	switch s {
	case "pending":
		return "Menunggu Persetujuan"
	case "approved":
		return "Disetujui"
	case "payment_requested":
		return "Menunggu Pembayaran"
	case "partial":
		return "Dibayar Sebagian"
	case "paid":
		return "Dibayar"
	case "received":
		return "Diterima"
	case "rejected":
		return "Ditolak"
	case "cancelled":
		return "Dibatalkan"
	}
	return s
}

// splitPaymentProof memisahkan kolom payment_proof ("REF-123 | /uploads/x.jpg")
// menjadi nomor referensi transfer; path file bukti dilewati karena tidak
// bermakna di luar aplikasi.
func splitPaymentProof(proof string) string {
	for _, p := range strings.Split(proof, " | ") {
		p = strings.TrimSpace(p)
		if p != "" && !strings.HasPrefix(p, "/uploads/") {
			return p
		}
	}
	return ""
}

// BuildProcurementPaymentsExcel menyusun file Excel daftar pembayaran pengadaan
// dengan filter yang sama persis dengan halaman /procurement-payments: status,
// tipe (barang|jasa), dan kata kunci pencarian. Scope outlet/unit kerja dari
// role admin ikut dipaksakan lewat ListPurchaseRequests.
func BuildProcurementPaymentsExcel(status, requestType, search string, scopeIDs, wuScopeIDs []string) ([]byte, string, error) {
	// parent_id="all" + excludeMasters=true meniru halaman Pembayaran: baris
	// split ditampilkan satu-satu, baris master disembunyikan.
	result, err := ListPurchaseRequests("", "", status, requestType, "", "all", true, search, scopeIDs, wuScopeIDs, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	companyName, _ := GetSetting("company_name")

	statusLabel := "Semua Status"
	if status != "" {
		statusLabel = purchaseStatusLabel(status)
	}
	typeLabel := "Semua Tipe"
	switch requestType {
	case "barang":
		typeLabel = "Barang"
	case "jasa":
		typeLabel = "Jasa"
	}

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shList    = "Daftar Pembayaran"
		shVendor  = "Per Vendor"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shList, shVendor} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// deref aman untuk field pointer di model.
	strOf := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	// Sisa tagihan hanya bermakna untuk status yang masih menunggu pelunasan.
	remainingOf := func(r models.PurchaseRequest) float64 {
		if r.Status == "payment_requested" || r.Status == "partial" {
			rem := r.TotalFinal - r.PaidAmount
			if rem < 0 {
				return 0
			}
			return rem
		}
		return 0
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 30)
	f.SetColWidth(shSummary, "B", "B", 34)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN PEMBAYARAN PENGADAAN")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Status", statusLabel, 0)
	setKV("Tipe", typeLabel, 0)
	if strings.TrimSpace(search) != "" {
		setKV("Kata Kunci", search, 0)
	}
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	// Rekap per status + grand total dihitung dari baris yang diekspor.
	type statusAgg struct {
		count   int
		nominal float64
		paid    float64
	}
	statusOrder := []string{"pending", "approved", "payment_requested", "partial", "paid", "received", "rejected", "cancelled"}
	byStatus := map[string]*statusAgg{}
	var grandNominal, grandPaid, grandRemaining float64
	for _, r := range result.Requests {
		a, ok := byStatus[r.Status]
		if !ok {
			a = &statusAgg{}
			byStatus[r.Status] = a
		}
		a.count++
		a.nominal += r.TotalFinal
		a.paid += r.PaidAmount
		grandNominal += r.TotalFinal
		grandPaid += r.PaidAmount
		grandRemaining += remainingOf(r)
	}

	setSection("RINGKASAN")
	setKV("Jumlah Pengajuan", result.Total, st.num)
	setKV("Total Nominal", grandNominal, st.money)
	setKV("Total Terbayar", grandPaid, st.money)
	setKV("Sisa Hutang (menunggu + sebagian)", grandRemaining, st.money)
	row++

	setSection("PER STATUS")
	for _, s := range statusOrder {
		if a, ok := byStatus[s]; ok {
			setKV(fmt.Sprintf("%s — %d pengajuan", purchaseStatusLabel(s), a.count), a.nominal, st.money)
		}
	}

	// ── Sheet Daftar Pembayaran ──────────────────────────────────────────
	setHeaderRow(f, shList, []string{
		"No", "Nomor", "No. Induk", "Tipe", "Pengadaan", "Vendor", "Unit Kerja",
		"Status", "Diajukan", "Nominal", "Terbayar", "Sisa Tagihan",
		"Dibayar Oleh", "Dibayar Pada", "Rek Tujuan", "Rek Asal", "No. Referensi",
		"No. Invoice", "Catatan Pembayaran",
	}, st.header)
	f.SetColWidth(shList, "A", "A", 5)
	f.SetColWidth(shList, "B", "C", 12)
	f.SetColWidth(shList, "D", "D", 8)
	f.SetColWidth(shList, "E", "E", 36)
	f.SetColWidth(shList, "F", "G", 20)
	f.SetColWidth(shList, "H", "H", 19)
	f.SetColWidth(shList, "I", "I", 17)
	f.SetColWidth(shList, "J", "L", 14)
	f.SetColWidth(shList, "M", "M", 14)
	f.SetColWidth(shList, "N", "N", 17)
	f.SetColWidth(shList, "O", "P", 28)
	f.SetColWidth(shList, "Q", "R", 16)
	f.SetColWidth(shList, "S", "S", 30)

	fmtStamp := func(iso string) string {
		if iso == "" {
			return ""
		}
		return fmtUTCStampID(iso, loc)
	}
	for i, r := range result.Requests {
		rw := i + 2
		typeTxt := "Jasa"
		if r.RequestType == "barang" {
			typeTxt = "Barang"
		}
		names := make([]string, 0, len(r.Items))
		for _, it := range r.Items {
			if strings.TrimSpace(it.Name) != "" {
				names = append(names, it.Name)
			}
		}

		f.SetCellValue(shList, fmt.Sprintf("A%d", rw), i+1)
		f.SetCellValue(shList, fmt.Sprintf("B%d", rw), r.RequestNumber)
		f.SetCellValue(shList, fmt.Sprintf("C%d", rw), r.ParentNumber)
		f.SetCellValue(shList, fmt.Sprintf("D%d", rw), typeTxt)
		f.SetCellValue(shList, fmt.Sprintf("E%d", rw), strings.Join(names, "; "))
		f.SetCellValue(shList, fmt.Sprintf("F%d", rw), r.VendorName)
		f.SetCellValue(shList, fmt.Sprintf("G%d", rw), r.WorkUnitName)
		f.SetCellValue(shList, fmt.Sprintf("H%d", rw), purchaseStatusLabel(r.Status))
		f.SetCellValue(shList, fmt.Sprintf("I%d", rw), fmtStamp(r.CreatedAt))
		f.SetCellValue(shList, fmt.Sprintf("J%d", rw), r.TotalFinal)
		f.SetCellValue(shList, fmt.Sprintf("K%d", rw), r.PaidAmount)
		f.SetCellValue(shList, fmt.Sprintf("L%d", rw), remainingOf(r))
		f.SetCellValue(shList, fmt.Sprintf("M%d", rw), strOf(r.PaidBy))
		f.SetCellValue(shList, fmt.Sprintf("N%d", rw), fmtStamp(strOf(r.PaidAt)))
		f.SetCellValue(shList, fmt.Sprintf("O%d", rw), r.PaymentAccountDest)
		f.SetCellValue(shList, fmt.Sprintf("P%d", rw), r.PaymentAccountSource)
		f.SetCellValue(shList, fmt.Sprintf("Q%d", rw), splitPaymentProof(strOf(r.PaymentProof)))
		f.SetCellValue(shList, fmt.Sprintf("R%d", rw), r.InvoiceNumber)
		f.SetCellValue(shList, fmt.Sprintf("S%d", rw), r.PaymentNotes)

		f.SetCellStyle(shList, fmt.Sprintf("A%d", rw), fmt.Sprintf("D%d", rw), st.num)
		f.SetCellStyle(shList, fmt.Sprintf("E%d", rw), fmt.Sprintf("E%d", rw), st.text)
		f.SetCellStyle(shList, fmt.Sprintf("F%d", rw), fmt.Sprintf("I%d", rw), st.num)
		f.SetCellStyle(shList, fmt.Sprintf("J%d", rw), fmt.Sprintf("L%d", rw), st.money)
		f.SetCellStyle(shList, fmt.Sprintf("M%d", rw), fmt.Sprintf("R%d", rw), st.num)
		f.SetCellStyle(shList, fmt.Sprintf("S%d", rw), fmt.Sprintf("S%d", rw), st.text)
	}
	if len(result.Requests) > 0 {
		rw := len(result.Requests) + 2
		f.SetCellValue(shList, fmt.Sprintf("A%d", rw), "TOTAL")
		f.MergeCell(shList, fmt.Sprintf("A%d", rw), fmt.Sprintf("I%d", rw))
		f.SetCellValue(shList, fmt.Sprintf("J%d", rw), grandNominal)
		f.SetCellValue(shList, fmt.Sprintf("K%d", rw), grandPaid)
		f.SetCellValue(shList, fmt.Sprintf("L%d", rw), grandRemaining)
		f.SetCellStyle(shList, fmt.Sprintf("A%d", rw), fmt.Sprintf("I%d", rw), st.numTot)
		f.SetCellStyle(shList, fmt.Sprintf("J%d", rw), fmt.Sprintf("L%d", rw), st.moneyTot)
	}
	if result.Total > len(result.Requests) {
		rw := len(result.Requests) + 4
		f.SetCellValue(shList, fmt.Sprintf("A%d", rw),
			fmt.Sprintf("Catatan: menampilkan %d dari %d pengajuan. Persempit filter untuk data lengkap.",
				len(result.Requests), result.Total))
		f.SetCellStyle(shList, fmt.Sprintf("A%d", rw), fmt.Sprintf("A%d", rw), st.note)
	}

	// ── Sheet Per Vendor ─────────────────────────────────────────────────
	type vendorAgg struct {
		name    string
		count   int
		nominal float64
		paid    float64
		remain  float64
	}
	vm := map[string]*vendorAgg{}
	for _, r := range result.Requests {
		name := r.VendorName
		if strings.TrimSpace(name) == "" {
			name = "(Tanpa Vendor)"
		}
		a, ok := vm[name]
		if !ok {
			a = &vendorAgg{name: name}
			vm[name] = a
		}
		a.count++
		a.nominal += r.TotalFinal
		a.paid += r.PaidAmount
		a.remain += remainingOf(r)
	}
	vendors := make([]vendorAgg, 0, len(vm))
	for _, a := range vm {
		vendors = append(vendors, *a)
	}
	sort.SliceStable(vendors, func(i, j int) bool { return vendors[i].nominal > vendors[j].nominal })

	setHeaderRow(f, shVendor, []string{"Vendor", "Jumlah Pengajuan", "Nominal", "Terbayar", "Sisa Tagihan"}, st.header)
	f.SetColWidth(shVendor, "A", "A", 30)
	f.SetColWidth(shVendor, "B", "B", 16)
	f.SetColWidth(shVendor, "C", "E", 16)
	for i, v := range vendors {
		rw := i + 2
		f.SetCellValue(shVendor, fmt.Sprintf("A%d", rw), v.name)
		f.SetCellValue(shVendor, fmt.Sprintf("B%d", rw), v.count)
		f.SetCellValue(shVendor, fmt.Sprintf("C%d", rw), v.nominal)
		f.SetCellValue(shVendor, fmt.Sprintf("D%d", rw), v.paid)
		f.SetCellValue(shVendor, fmt.Sprintf("E%d", rw), v.remain)
		f.SetCellStyle(shVendor, fmt.Sprintf("A%d", rw), fmt.Sprintf("B%d", rw), st.num)
		f.SetCellStyle(shVendor, fmt.Sprintf("C%d", rw), fmt.Sprintf("E%d", rw), st.money)
	}
	if len(vendors) > 0 {
		rw := len(vendors) + 2
		f.SetCellValue(shVendor, fmt.Sprintf("A%d", rw), "TOTAL")
		var vc int
		for _, v := range vendors {
			vc += v.count
		}
		f.SetCellValue(shVendor, fmt.Sprintf("B%d", rw), vc)
		f.SetCellValue(shVendor, fmt.Sprintf("C%d", rw), grandNominal)
		f.SetCellValue(shVendor, fmt.Sprintf("D%d", rw), grandPaid)
		f.SetCellValue(shVendor, fmt.Sprintf("E%d", rw), grandRemaining)
		f.SetCellStyle(shVendor, fmt.Sprintf("A%d", rw), fmt.Sprintf("B%d", rw), st.numTot)
		f.SetCellStyle(shVendor, fmt.Sprintf("C%d", rw), fmt.Sprintf("E%d", rw), st.moneyTot)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Pembayaran-Pengadaan_%s_%s.xlsx",
		slugFilename(statusLabel), time.Now().In(loc).Format("2006-01-02"))
	return buf.Bytes(), filename, nil
}

// ledgerGroupLabel menerjemahkan kelompok akun buku besar ke label tampilan.
func ledgerGroupLabel(g string) string {
	switch g {
	case "aset":
		return "Aset"
	case "kewajiban":
		return "Kewajiban"
	case "ekuitas":
		return "Ekuitas"
	case "pendapatan":
		return "Pendapatan"
	case "beban":
		return "Beban"
	}
	return g
}

// BuildGeneralLedgerExcel menyusun file Excel buku besar dengan filter yang
// sama persis dengan halaman /general-ledger: rentang tanggal, outlet, dan
// akun. scopeIDs berasal dari role admin sehingga hasil export tak melebihi
// hak akses user.
func BuildGeneralLedgerExcel(dateFrom, dateTo, outletID, accountFilter string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetGeneralLedger(dateFrom, dateTo, outletID, accountFilter, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	accountLabel := "Semua Akun"
	if accountFilter != "" {
		accountLabel = accountFilter
		for _, a := range report.Accounts {
			if a.Code == accountFilter {
				accountLabel = a.Code + " " + a.Name
				break
			}
		}
	}

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shAccount = "Rekap Akun"
		shLedger  = "Buku Besar"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shAccount, shLedger} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 26)
	f.SetColWidth(shSummary, "B", "B", 34)

	row := 1
	f.SetCellValue(shSummary, "A1", "BUKU BESAR")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Akun", accountLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	setSection("RINGKASAN")
	setKV("Saldo Kas", report.Summary.CashBalance, st.money)
	setKV("Total Pendapatan", report.Summary.TotalRevenue, st.money)
	setKV("Total Beban", report.Summary.TotalExpense, st.money)
	setKV("Jumlah Akun", len(report.Accounts), st.num)

	// ── Sheet Rekap Akun ─────────────────────────────────────────────────
	setHeaderRow(f, shAccount, []string{"Kode", "Nama Akun", "Kelompok", "Jml Entri", "Total Debit", "Total Kredit", "Saldo"}, st.header)
	f.SetColWidth(shAccount, "A", "A", 9)
	f.SetColWidth(shAccount, "B", "B", 28)
	f.SetColWidth(shAccount, "C", "C", 12)
	f.SetColWidth(shAccount, "D", "D", 10)
	f.SetColWidth(shAccount, "E", "G", 16)
	var sumDebit, sumCredit float64
	for i, a := range report.Accounts {
		r := i + 2
		f.SetCellValue(shAccount, fmt.Sprintf("A%d", r), a.Code)
		f.SetCellValue(shAccount, fmt.Sprintf("B%d", r), a.Name)
		f.SetCellValue(shAccount, fmt.Sprintf("C%d", r), ledgerGroupLabel(a.Group))
		f.SetCellValue(shAccount, fmt.Sprintf("D%d", r), len(a.Entries))
		f.SetCellValue(shAccount, fmt.Sprintf("E%d", r), a.TotalDebit)
		f.SetCellValue(shAccount, fmt.Sprintf("F%d", r), a.TotalCredit)
		f.SetCellValue(shAccount, fmt.Sprintf("G%d", r), a.Balance)
		f.SetCellStyle(shAccount, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.num)
		f.SetCellStyle(shAccount, fmt.Sprintf("E%d", r), fmt.Sprintf("G%d", r), st.money)
		sumDebit += a.TotalDebit
		sumCredit += a.TotalCredit
	}
	if len(report.Accounts) > 0 {
		r := len(report.Accounts) + 2
		f.SetCellValue(shAccount, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shAccount, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r))
		f.SetCellValue(shAccount, fmt.Sprintf("E%d", r), sumDebit)
		f.SetCellValue(shAccount, fmt.Sprintf("F%d", r), sumCredit)
		f.SetCellStyle(shAccount, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.numTot)
		f.SetCellStyle(shAccount, fmt.Sprintf("E%d", r), fmt.Sprintf("G%d", r), st.moneyTot)
	}

	// ── Sheet Buku Besar ─────────────────────────────────────────────────
	// Meniru tampilan halaman: tiap akun jadi satu blok — baris judul akun,
	// entri kronologis dengan saldo berjalan, lalu subtotal akun.
	setHeaderRow(f, shLedger, []string{"Tanggal", "Keterangan", "Debit", "Kredit", "Saldo"}, st.header)
	f.SetColWidth(shLedger, "A", "A", 14)
	f.SetColWidth(shLedger, "B", "B", 52)
	f.SetColWidth(shLedger, "C", "E", 16)
	r := 2
	for _, a := range report.Accounts {
		f.SetCellValue(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("%s — %s (%s)", a.Code, a.Name, ledgerGroupLabel(a.Group)))
		f.MergeCell(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r))
		f.SetCellStyle(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r), st.section)
		r++
		for _, e := range a.Entries {
			f.SetCellValue(shLedger, fmt.Sprintf("A%d", r), fmtDateID(e.Date))
			f.SetCellValue(shLedger, fmt.Sprintf("B%d", r), e.Description)
			if e.Debit > 0 {
				f.SetCellValue(shLedger, fmt.Sprintf("C%d", r), e.Debit)
			}
			if e.Credit > 0 {
				f.SetCellValue(shLedger, fmt.Sprintf("D%d", r), e.Credit)
			}
			f.SetCellValue(shLedger, fmt.Sprintf("E%d", r), e.Balance)
			f.SetCellStyle(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.num)
			f.SetCellStyle(shLedger, fmt.Sprintf("B%d", r), fmt.Sprintf("B%d", r), st.text)
			f.SetCellStyle(shLedger, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.money)
			r++
		}
		f.SetCellValue(shLedger, fmt.Sprintf("A%d", r), "Subtotal "+a.Code)
		f.MergeCell(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r))
		f.SetCellValue(shLedger, fmt.Sprintf("C%d", r), a.TotalDebit)
		f.SetCellValue(shLedger, fmt.Sprintf("D%d", r), a.TotalCredit)
		f.SetCellValue(shLedger, fmt.Sprintf("E%d", r), a.Balance)
		f.SetCellStyle(shLedger, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.numTot)
		f.SetCellStyle(shLedger, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.moneyTot)
		r += 2 // baris kosong pemisah antar akun
	}
	if len(report.Accounts) == 0 {
		f.SetCellValue(shLedger, "A2", "Tidak ada data buku besar pada periode ini.")
		f.SetCellStyle(shLedger, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	namePart := slugFilename(outletLabel)
	if accountFilter != "" {
		namePart += "_Akun-" + slugFilename(accountFilter)
	}
	filename := fmt.Sprintf("Buku-Besar_%s_%s_sd_%s.xlsx", namePart, dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildCashFlowReportExcel menyusun file Excel laporan arus kas dengan filter
// yang sama persis dengan halaman /cash-flow-report: rentang tanggal & outlet.
// scopeIDs berasal dari role admin sehingga hasil export tak melebihi hak akses.
func BuildCashFlowReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetCashFlowReport(dateFrom, dateTo, outletID, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shDaily   = "Harian"
	)
	f.SetSheetName("Sheet1", shSummary)
	if _, err := f.NewSheet(shDaily); err != nil {
		return nil, "", err
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 28)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN ARUS KAS")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("PENERIMAAN OPERASI")
	setKV("Penjualan", sum.SalesReceipts, st.money)
	setKV("Pemasukan Kas Lainnya", sum.OtherReceipts, st.money)
	setKV("Total Penerimaan", sum.TotalReceipts, st.moneyTot)
	row++

	setSection("PENGELUARAN OPERASI")
	setKV("Pembelian Bahan Baku", sum.COGSPayments, st.money)
	setKV("Pembayaran Jasa", sum.ServicePayments, st.money)
	setKV("Pengeluaran Operasional", sum.OpexPayments, st.money)
	setKV("Total Pengeluaran", sum.TotalPayments, st.moneyTot)
	row++

	setSection("ARUS KAS BERSIH")
	setKV("Penerimaan − Pengeluaran", sum.NetCashFlow, st.moneyTot)

	// ── Sheet Harian ─────────────────────────────────────────────────────
	setHeaderRow(f, shDaily, []string{
		"Tanggal", "Penjualan", "Kas Masuk Lain", "Total Masuk",
		"Bahan Baku", "Jasa", "Operasional", "Total Keluar", "Arus Bersih",
	}, st.header)
	f.SetColWidth(shDaily, "A", "A", 14)
	f.SetColWidth(shDaily, "B", "I", 15)

	// Service mengembalikan urutan DESC; untuk dibaca manusia urutkan naik.
	daily := make([]models.CashFlowRow, len(report.Daily))
	copy(daily, report.Daily)
	for i, j := 0, len(daily)-1; i < j; i, j = i+1, j-1 {
		daily[i], daily[j] = daily[j], daily[i]
	}
	var tSales, tOther, tCogs, tService, tOpex, tNet float64
	for i, d := range daily {
		r := i + 2
		in := d.SalesReceipts + d.OtherReceipts
		out := d.COGSPayments + d.ServicePayments + d.OpexPayments
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), fmtDateID(d.Date))
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), d.SalesReceipts)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), d.OtherReceipts)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), in)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), d.COGSPayments)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), d.ServicePayments)
		f.SetCellValue(shDaily, fmt.Sprintf("G%d", r), d.OpexPayments)
		f.SetCellValue(shDaily, fmt.Sprintf("H%d", r), out)
		f.SetCellValue(shDaily, fmt.Sprintf("I%d", r), d.NetCashFlow)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.num)
		f.SetCellStyle(shDaily, fmt.Sprintf("B%d", r), fmt.Sprintf("I%d", r), st.money)
		tSales += d.SalesReceipts
		tOther += d.OtherReceipts
		tCogs += d.COGSPayments
		tService += d.ServicePayments
		tOpex += d.OpexPayments
		tNet += d.NetCashFlow
	}
	if len(daily) > 0 {
		r := len(daily) + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), tSales)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), tOther)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), tSales+tOther)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), tCogs)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), tService)
		f.SetCellValue(shDaily, fmt.Sprintf("G%d", r), tOpex)
		f.SetCellValue(shDaily, fmt.Sprintf("H%d", r), tCogs+tService+tOpex)
		f.SetCellValue(shDaily, fmt.Sprintf("I%d", r), tNet)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.numTot)
		f.SetCellStyle(shDaily, fmt.Sprintf("B%d", r), fmt.Sprintf("I%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shDaily, "A2", "Tidak ada data arus kas pada periode ini.")
		f.SetCellStyle(shDaily, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Arus-Kas_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildProfitLossReportExcel menyusun file Excel laporan laba rugi F&B dengan
// filter yang sama persis dengan halaman /profit-loss-report: rentang tanggal
// & outlet. scopeIDs berasal dari role admin sehingga hasil export tak
// melebihi hak akses user.
func BuildProfitLossReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetProfitLossReport(dateFrom, dateTo, outletID, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Laba Rugi"
		shDaily   = "Harian"
		shOutlet  = "Per Outlet"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shDaily, shOutlet} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Laba Rugi (ringkasan berformat laporan) ────────────────────
	f.SetColWidth(shSummary, "A", "A", 34)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN LABA RUGI F&B")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("PENDAPATAN")
	setKV("Penjualan Makanan & Minuman", sum.SalesRevenue, st.money)
	setKV("Pendapatan Lainnya", sum.OtherIncome, st.money)
	setKV("Total Pendapatan", sum.TotalRevenue, st.moneyTot)
	row++

	setSection("HARGA POKOK PENJUALAN")
	setKV("HPP (Bahan Baku)", sum.COGS, st.money)
	setKV("Laba Kotor", sum.GrossProfit, st.moneyTot)
	// Margin dari service dalam satuan persen; style pct memakai fraksi.
	setKV("Margin Kotor", sum.GrossMargin/100, st.pct)
	row++

	setSection("BEBAN OPERASIONAL")
	setKV("Beban Jasa & Layanan", sum.ServiceExpense, st.money)
	setKV("Beban Operasional Outlet", sum.OperatingExpense, st.money)
	setKV("Total Beban Operasional", sum.TotalOpex, st.moneyTot)
	setKV("Laba Operasional", sum.OperatingProfit, st.moneyTot)
	row++

	setSection("PAJAK & LABA BERSIH")
	setKV("Pajak Restoran (PB1)", sum.TaxExpense, st.money)
	setKV("Laba Bersih", sum.NetProfit, st.moneyTot)
	setKV("Margin Bersih", sum.NetMargin/100, st.pct)

	// ── Sheet Harian ─────────────────────────────────────────────────────
	setHeaderRow(f, shDaily, []string{"Tanggal", "Pendapatan", "HPP", "Laba Kotor", "Beban Opex", "Laba Bersih"}, st.header)
	f.SetColWidth(shDaily, "A", "A", 14)
	f.SetColWidth(shDaily, "B", "F", 15)

	// Service mengembalikan urutan DESC; untuk dibaca manusia urutkan naik.
	daily := make([]models.ProfitLossRow, len(report.Daily))
	copy(daily, report.Daily)
	for i, j := 0, len(daily)-1; i < j; i, j = i+1, j-1 {
		daily[i], daily[j] = daily[j], daily[i]
	}
	var tRev, tCogs, tGross, tOpex, tNet float64
	for i, d := range daily {
		r := i + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), fmtDateID(d.Date))
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), d.Revenue)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), d.COGS)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), d.GrossProfit)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), d.OperatingExpense)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), d.NetProfit)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.num)
		f.SetCellStyle(shDaily, fmt.Sprintf("B%d", r), fmt.Sprintf("F%d", r), st.money)
		tRev += d.Revenue
		tCogs += d.COGS
		tGross += d.GrossProfit
		tOpex += d.OperatingExpense
		tNet += d.NetProfit
	}
	if len(daily) > 0 {
		r := len(daily) + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), tRev)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), tCogs)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), tGross)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), tOpex)
		f.SetCellValue(shDaily, fmt.Sprintf("F%d", r), tNet)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.numTot)
		f.SetCellStyle(shDaily, fmt.Sprintf("B%d", r), fmt.Sprintf("F%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shDaily, "A2", "Tidak ada data pada periode ini.")
		f.SetCellStyle(shDaily, "A2", "A2", st.note)
	}

	// ── Sheet Per Outlet ─────────────────────────────────────────────────
	setHeaderRow(f, shOutlet, []string{"Outlet", "Pendapatan", "HPP", "Beban Opex", "Laba Bersih"}, st.header)
	f.SetColWidth(shOutlet, "A", "A", 28)
	f.SetColWidth(shOutlet, "B", "E", 15)
	var oRev, oCogs, oOpex, oNet float64
	for i, o := range report.ByOutlet {
		r := i + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), o.OutletName)
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), o.Revenue)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), o.COGS)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), o.OperatingExpense)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), o.NetProfit)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.num)
		f.SetCellStyle(shOutlet, fmt.Sprintf("B%d", r), fmt.Sprintf("E%d", r), st.money)
		oRev += o.Revenue
		oCogs += o.COGS
		oOpex += o.OperatingExpense
		oNet += o.NetProfit
	}
	if len(report.ByOutlet) > 0 {
		r := len(report.ByOutlet) + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), oRev)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), oCogs)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), oOpex)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), oNet)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.numTot)
		f.SetCellStyle(shOutlet, fmt.Sprintf("B%d", r), fmt.Sprintf("E%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shOutlet, "A2", "Tidak ada data pada periode ini.")
		f.SetCellStyle(shOutlet, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Laba-Rugi_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildBalanceReportExcel menyusun file Excel laporan neraca dengan filter yang
// sama persis dengan halaman /balance-report (rentang tanggal; outlet_id
// diterima demi paritas dengan endpoint layar). scopeIDs berasal dari role
// admin sehingga hasil export tak melebihi hak akses user.
func BuildBalanceReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetBalanceReport(dateFrom, dateTo, outletID, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Neraca"
		shOutlet  = "Per Outlet"
	)
	f.SetSheetName("Sheet1", shSummary)
	if _, err := f.NewSheet(shOutlet); err != nil {
		return nil, "", err
	}

	// ── Sheet Neraca ─────────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 36)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN NERACA")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	setSection("ASET")
	setKV("Kas & Setara Kas (pendapatan + pemasukan − pengeluaran)", report.CashAndEquivalents, st.money)
	setKV("Piutang Usaha (pesanan belum dibayar)", report.Receivables, st.money)
	setKV("Total Aset", report.TotalAssets, st.moneyTot)
	row++

	setSection("KEWAJIBAN")
	setKV("Hutang Usaha (pengadaan disetujui belum dibayar)", report.AccountsPayable, st.money)
	setKV("Hutang Pajak Restoran (PB1)", report.TaxPayable, st.money)
	setKV("Total Kewajiban", report.TotalLiabilities, st.moneyTot)
	row++

	setSection("EKUITAS")
	setKV("Modal Pemilik (Aset − Kewajiban)", report.TotalEquity, st.moneyTot)
	row++

	// Cek keseimbangan seperti banner di halaman.
	diff := report.TotalAssets - (report.TotalLiabilities + report.TotalEquity)
	balanceTxt := "Aset = Kewajiban + Ekuitas — Neraca Seimbang ✓"
	if diff > 0.02 || diff < -0.02 {
		balanceTxt = fmt.Sprintf("Aset ≠ Kewajiban + Ekuitas (selisih %.2f) — Neraca Tidak Seimbang", diff)
	}
	f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), balanceTxt)
	f.MergeCell(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.note)
	row += 2

	setSection("DETAIL KOMPONEN")
	setKV("Pendapatan Transaksi", report.TotalRevenue, st.money)
	setKV("Pemasukan Kas", report.TotalCashIn, st.money)
	setKV("Pengeluaran Kas", report.TotalExpense, st.money)
	setKV("Piutang Usaha", report.UnpaidAmount, st.money)

	// ── Sheet Per Outlet ─────────────────────────────────────────────────
	setHeaderRow(f, shOutlet, []string{
		"Outlet", "Kas & Setara Kas", "Piutang", "Total Aset",
		"Hutang Usaha", "Hutang Pajak", "Total Kewajiban", "Ekuitas",
	}, st.header)
	f.SetColWidth(shOutlet, "A", "A", 28)
	f.SetColWidth(shOutlet, "B", "H", 16)
	var tCash, tRecv, tAssets, tAP, tTax, tLiab, tEq float64
	for i, o := range report.Outlets {
		r := i + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), o.OutletName)
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), o.CashAndEquivalents)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), o.Receivables)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), o.TotalAssets)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), o.AccountsPayable)
		f.SetCellValue(shOutlet, fmt.Sprintf("F%d", r), o.TaxPayable)
		f.SetCellValue(shOutlet, fmt.Sprintf("G%d", r), o.TotalLiabilities)
		f.SetCellValue(shOutlet, fmt.Sprintf("H%d", r), o.TotalEquity)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.num)
		f.SetCellStyle(shOutlet, fmt.Sprintf("B%d", r), fmt.Sprintf("H%d", r), st.money)
		tCash += o.CashAndEquivalents
		tRecv += o.Receivables
		tAssets += o.TotalAssets
		tAP += o.AccountsPayable
		tTax += o.TaxPayable
		tLiab += o.TotalLiabilities
		tEq += o.TotalEquity
	}
	if len(report.Outlets) > 0 {
		r := len(report.Outlets) + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), tCash)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), tRecv)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), tAssets)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), tAP)
		f.SetCellValue(shOutlet, fmt.Sprintf("F%d", r), tTax)
		f.SetCellValue(shOutlet, fmt.Sprintf("G%d", r), tLiab)
		f.SetCellValue(shOutlet, fmt.Sprintf("H%d", r), tEq)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.numTot)
		f.SetCellStyle(shOutlet, fmt.Sprintf("B%d", r), fmt.Sprintf("H%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shOutlet, "A2", "Tidak ada data pada periode ini.")
		f.SetCellStyle(shOutlet, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Neraca_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildTaxReportExcel menyusun file Excel laporan pajak restoran (PB1) dengan
// filter yang sama persis dengan halaman /tax-report: rentang tanggal &
// outlet. scopeIDs berasal dari role admin sehingga hasil export tak melebihi
// hak akses user.
func BuildTaxReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetTaxReport(dateFrom, dateTo, outletID, scopeIDs)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shDaily   = "Harian"
		shOutlet  = "Per Outlet"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shDaily, shOutlet} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 30)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN PAJAK RESTORAN (PB1)")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("RINGKASAN")
	setKV("Total Transaksi", sum.TotalTransactions, st.num)
	setKV("Pendapatan Bruto", sum.GrossRevenue, st.money)
	setKV("Pajak Restoran", sum.TaxAmount, st.money)
	setKV("Pendapatan Neto", sum.NetRevenue, st.moneyTot)
	setKV("Tarif Efektif Gabungan", sum.TaxRate/100, st.pct)
	row++

	f.SetCellValue(shSummary, fmt.Sprintf("A%d", row),
		"Keterangan: pajak dihitung per outlet dari nilai pajak nyata tiap transaksi (inklusif). Rincian tarif tiap outlet ada di sheet \"Per Outlet\".")
	f.MergeCell(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.note)

	// ── Sheet Harian ─────────────────────────────────────────────────────
	setHeaderRow(f, shDaily, []string{"Tanggal", "Transaksi", "Pendapatan Bruto", "Pajak", "Pendapatan Neto"}, st.header)
	f.SetColWidth(shDaily, "A", "A", 14)
	f.SetColWidth(shDaily, "B", "B", 10)
	f.SetColWidth(shDaily, "C", "E", 17)

	// Service mengembalikan urutan DESC; untuk dibaca manusia urutkan naik.
	daily := make([]models.TaxReportRow, len(report.Daily))
	copy(daily, report.Daily)
	for i, j := 0, len(daily)-1; i < j; i, j = i+1, j-1 {
		daily[i], daily[j] = daily[j], daily[i]
	}
	var tTrx int
	var tGross, tTax, tNet float64
	for i, d := range daily {
		r := i + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), fmtDateID(d.Date))
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), d.TotalTransactions)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), d.GrossRevenue)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), d.TaxAmount)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), d.NetRevenue)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.num)
		f.SetCellStyle(shDaily, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.money)
		tTrx += d.TotalTransactions
		tGross += d.GrossRevenue
		tTax += d.TaxAmount
		tNet += d.NetRevenue
	}
	if len(daily) > 0 {
		r := len(daily) + 2
		f.SetCellValue(shDaily, fmt.Sprintf("A%d", r), "TOTAL")
		f.SetCellValue(shDaily, fmt.Sprintf("B%d", r), tTrx)
		f.SetCellValue(shDaily, fmt.Sprintf("C%d", r), tGross)
		f.SetCellValue(shDaily, fmt.Sprintf("D%d", r), tTax)
		f.SetCellValue(shDaily, fmt.Sprintf("E%d", r), tNet)
		f.SetCellStyle(shDaily, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.numTot)
		f.SetCellStyle(shDaily, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shDaily, "A2", "Tidak ada data pada periode ini.")
		f.SetCellStyle(shDaily, "A2", "A2", st.note)
	}

	// ── Sheet Per Outlet ─────────────────────────────────────────────────
	setHeaderRow(f, shOutlet, []string{"Outlet", "Tarif", "Pendapatan Bruto", "Pajak", "Pendapatan Neto"}, st.header)
	f.SetColWidth(shOutlet, "A", "A", 28)
	f.SetColWidth(shOutlet, "B", "B", 10)
	f.SetColWidth(shOutlet, "C", "E", 17)
	var oGross, oTax, oNet float64
	for i, o := range report.ByOutlet {
		r := i + 2
		tarif := "Nonaktif"
		if o.TaxEnabled {
			tarif = strconv.FormatFloat(o.TaxRate, 'f', -1, 64) + "%"
		}
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), o.OutletName)
		f.SetCellValue(shOutlet, fmt.Sprintf("B%d", r), tarif)
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), o.GrossRevenue)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), o.TaxAmount)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), o.NetRevenue)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.num)
		f.SetCellStyle(shOutlet, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.money)
		oGross += o.GrossRevenue
		oTax += o.TaxAmount
		oNet += o.NetRevenue
	}
	if len(report.ByOutlet) > 0 {
		r := len(report.ByOutlet) + 2
		f.SetCellValue(shOutlet, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r))
		f.SetCellValue(shOutlet, fmt.Sprintf("C%d", r), oGross)
		f.SetCellValue(shOutlet, fmt.Sprintf("D%d", r), oTax)
		f.SetCellValue(shOutlet, fmt.Sprintf("E%d", r), oNet)
		f.SetCellStyle(shOutlet, fmt.Sprintf("A%d", r), fmt.Sprintf("B%d", r), st.numTot)
		f.SetCellStyle(shOutlet, fmt.Sprintf("C%d", r), fmt.Sprintf("E%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shOutlet, "A2", "Tidak ada data pada periode ini.")
		f.SetCellStyle(shOutlet, "A2", "A2", st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Pajak_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// parseFlexTime mencoba beberapa format stempel waktu yang muncul di data void
// (ISO dari device maupun ::text dari Postgres). ok=false bila tak dikenal.
func parseFlexTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// fmtFlexTimeID memformat stempel waktu fleksibel ke zona aplikasi; bila tidak
// bisa diparse, kembalikan apa adanya agar data tidak hilang.
func fmtFlexTimeID(s string, loc *time.Location) string {
	if t, ok := parseFlexTime(s); ok {
		return fmtTimeID(t.In(loc))
	}
	return s
}

// voidLifespan menghitung selang order dibuat → di-void ("1 jam 5 mnt");
// kosong bila salah satu stempel tak bisa diparse atau hasilnya tak wajar.
func voidLifespan(createdAt, voidedAt string) string {
	a, okA := parseFlexTime(createdAt)
	b, okB := parseFlexTime(voidedAt)
	if !okA || !okB {
		return ""
	}
	mins := int(b.Sub(a).Minutes())
	if mins < 0 || mins > 60*24*30 {
		return ""
	}
	if mins < 1 {
		return "< 1 mnt"
	}
	if mins < 60 {
		return fmt.Sprintf("%d mnt", mins)
	}
	return fmt.Sprintf("%d jam %d mnt", mins/60, mins%60)
}

// BuildVoidReportExcel menyusun file Excel laporan void (void transaksi + void
// item) dengan filter yang sama persis dengan halaman /void-report: rentang
// tanggal & outlet. Tab Titipan TIDAK ikut karena memakai permission terpisah
// (reports.titipan.view). scopeIDs berasal dari role admin.
func BuildVoidReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetVoidReport(dateFrom, dateTo, outletID, scopeIDs, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shOrder   = "Void Transaksi"
		shItem    = "Void Item"
	)
	f.SetSheetName("Sheet1", shSummary)
	for _, name := range []string{shOrder, shItem} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, "", err
		}
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 26)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN VOID")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("VOID TRANSAKSI (order dibatalkan penuh)")
	setKV("Jumlah", sum.TotalVoided, st.num)
	setKV("Nilai", sum.TotalAmount, st.money)
	row++

	setSection("VOID ITEM (item dihapus, order jalan terus)")
	setKV("Jumlah", sum.ItemVoided, st.num)
	setKV("Nilai", sum.ItemAmount, st.money)
	row++

	f.SetCellValue(shSummary, fmt.Sprintf("A%d", row),
		"Keterangan: kolom \"Selang\" pada sheet Void Transaksi = waktu order dibuat sampai di-void; void yang terlalu cepat/rutin bisa jadi sinyal pola yang perlu dicek.")
	f.MergeCell(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.note)

	// ── Sheet Void Transaksi ─────────────────────────────────────────────
	setHeaderRow(f, shOrder, []string{
		"No", "Waktu Void", "Outlet", "Pelanggan", "Meja", "Rincian Item",
		"Total", "Di-void Oleh", "Alasan", "Order Dibuat", "Selang",
	}, st.header)
	f.SetColWidth(shOrder, "A", "A", 5)
	f.SetColWidth(shOrder, "B", "B", 18)
	f.SetColWidth(shOrder, "C", "D", 20)
	f.SetColWidth(shOrder, "E", "E", 8)
	f.SetColWidth(shOrder, "F", "F", 45)
	f.SetColWidth(shOrder, "G", "G", 14)
	f.SetColWidth(shOrder, "H", "H", 16)
	f.SetColWidth(shOrder, "I", "I", 28)
	f.SetColWidth(shOrder, "J", "J", 18)
	f.SetColWidth(shOrder, "K", "K", 13)

	// Service mengurutkan DESC (terbaru dulu); untuk laporan urutkan kronologis.
	orders := make([]models.VoidOrderRow, len(report.Data))
	copy(orders, report.Data)
	for i, j := 0, len(orders)-1; i < j; i, j = i+1, j-1 {
		orders[i], orders[j] = orders[j], orders[i]
	}
	var oSum float64
	for i, o := range orders {
		r := i + 2
		reason := o.VoidReason
		if strings.TrimSpace(reason) == "" {
			reason = "Dibatalkan kasir"
		}
		f.SetCellValue(shOrder, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shOrder, fmt.Sprintf("B%d", r), fmtFlexTimeID(o.VoidedAt, loc))
		f.SetCellValue(shOrder, fmt.Sprintf("C%d", r), o.OutletName)
		f.SetCellValue(shOrder, fmt.Sprintf("D%d", r), o.CustomerName)
		f.SetCellValue(shOrder, fmt.Sprintf("E%d", r), o.TableNumber)
		f.SetCellValue(shOrder, fmt.Sprintf("F%d", r), summarizeItems(o.Items))
		f.SetCellValue(shOrder, fmt.Sprintf("G%d", r), o.TotalAmount)
		f.SetCellValue(shOrder, fmt.Sprintf("H%d", r), o.VoidedBy)
		f.SetCellValue(shOrder, fmt.Sprintf("I%d", r), reason)
		f.SetCellValue(shOrder, fmt.Sprintf("J%d", r), fmtFlexTimeID(o.CreatedAt, loc))
		f.SetCellValue(shOrder, fmt.Sprintf("K%d", r), voidLifespan(o.CreatedAt, o.VoidedAt))
		f.SetCellStyle(shOrder, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r), st.num)
		f.SetCellStyle(shOrder, fmt.Sprintf("F%d", r), fmt.Sprintf("F%d", r), st.text)
		f.SetCellStyle(shOrder, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), st.money)
		f.SetCellStyle(shOrder, fmt.Sprintf("H%d", r), fmt.Sprintf("H%d", r), st.num)
		f.SetCellStyle(shOrder, fmt.Sprintf("I%d", r), fmt.Sprintf("I%d", r), st.text)
		f.SetCellStyle(shOrder, fmt.Sprintf("J%d", r), fmt.Sprintf("K%d", r), st.num)
		oSum += o.TotalAmount
	}
	if len(orders) > 0 {
		r := len(orders) + 2
		f.SetCellValue(shOrder, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shOrder, fmt.Sprintf("A%d", r), fmt.Sprintf("F%d", r))
		f.SetCellValue(shOrder, fmt.Sprintf("G%d", r), oSum)
		f.SetCellStyle(shOrder, fmt.Sprintf("A%d", r), fmt.Sprintf("F%d", r), st.numTot)
		f.SetCellStyle(shOrder, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), st.moneyTot)
		f.SetCellStyle(shOrder, fmt.Sprintf("H%d", r), fmt.Sprintf("K%d", r), st.numTot)
	} else {
		f.SetCellValue(shOrder, "A2", "Tidak ada order void pada periode ini.")
		f.SetCellStyle(shOrder, "A2", "A2", st.note)
	}
	if report.Total > len(report.Data) {
		r := len(orders) + 4
		f.SetCellValue(shOrder, fmt.Sprintf("A%d", r),
			fmt.Sprintf("Catatan: menampilkan %d dari %d void transaksi. Persempit rentang tanggal untuk data lengkap.",
				len(report.Data), report.Total))
		f.SetCellStyle(shOrder, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.note)
	}

	// ── Sheet Void Item ──────────────────────────────────────────────────
	setHeaderRow(f, shItem, []string{
		"No", "Waktu Void", "Outlet", "Produk", "Qty", "Harga", "Subtotal",
		"Meja", "Pemesan", "Di-void Oleh", "Alasan",
	}, st.header)
	f.SetColWidth(shItem, "A", "A", 5)
	f.SetColWidth(shItem, "B", "B", 18)
	f.SetColWidth(shItem, "C", "C", 20)
	f.SetColWidth(shItem, "D", "D", 28)
	f.SetColWidth(shItem, "E", "E", 7)
	f.SetColWidth(shItem, "F", "G", 13)
	f.SetColWidth(shItem, "H", "H", 8)
	f.SetColWidth(shItem, "I", "J", 16)
	f.SetColWidth(shItem, "K", "K", 28)

	items := make([]models.VoidItemRow, len(report.Items))
	copy(items, report.Items)
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	var iSum float64
	var iQty float64
	for i, it := range items {
		r := i + 2
		reason := it.VoidReason
		if strings.TrimSpace(reason) == "" {
			reason = "Dibatalkan kasir"
		}
		f.SetCellValue(shItem, fmt.Sprintf("A%d", r), i+1)
		f.SetCellValue(shItem, fmt.Sprintf("B%d", r), fmtFlexTimeID(it.VoidedAt, loc))
		f.SetCellValue(shItem, fmt.Sprintf("C%d", r), it.OutletName)
		f.SetCellValue(shItem, fmt.Sprintf("D%d", r), it.ProductName)
		f.SetCellValue(shItem, fmt.Sprintf("E%d", r), it.Qty)
		f.SetCellValue(shItem, fmt.Sprintf("F%d", r), it.Price)
		f.SetCellValue(shItem, fmt.Sprintf("G%d", r), it.Subtotal)
		f.SetCellValue(shItem, fmt.Sprintf("H%d", r), it.TableNumber)
		f.SetCellValue(shItem, fmt.Sprintf("I%d", r), it.WaiterName)
		f.SetCellValue(shItem, fmt.Sprintf("J%d", r), it.VoidedBy)
		f.SetCellValue(shItem, fmt.Sprintf("K%d", r), reason)
		f.SetCellStyle(shItem, fmt.Sprintf("A%d", r), fmt.Sprintf("E%d", r), st.num)
		f.SetCellStyle(shItem, fmt.Sprintf("F%d", r), fmt.Sprintf("G%d", r), st.money)
		f.SetCellStyle(shItem, fmt.Sprintf("H%d", r), fmt.Sprintf("J%d", r), st.num)
		f.SetCellStyle(shItem, fmt.Sprintf("K%d", r), fmt.Sprintf("K%d", r), st.text)
		iSum += it.Subtotal
		iQty += it.Qty
	}
	if len(items) > 0 {
		r := len(items) + 2
		f.SetCellValue(shItem, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shItem, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r))
		f.SetCellValue(shItem, fmt.Sprintf("E%d", r), iQty)
		f.SetCellValue(shItem, fmt.Sprintf("G%d", r), iSum)
		f.SetCellStyle(shItem, fmt.Sprintf("A%d", r), fmt.Sprintf("F%d", r), st.numTot)
		f.SetCellStyle(shItem, fmt.Sprintf("G%d", r), fmt.Sprintf("G%d", r), st.moneyTot)
		f.SetCellStyle(shItem, fmt.Sprintf("H%d", r), fmt.Sprintf("K%d", r), st.numTot)
	} else {
		f.SetCellValue(shItem, "A2", "Tidak ada void item pada periode ini.")
		f.SetCellStyle(shItem, "A2", "A2", st.note)
	}
	if report.ItemsTotal > len(report.Items) {
		r := len(items) + 4
		f.SetCellValue(shItem, fmt.Sprintf("A%d", r),
			fmt.Sprintf("Catatan: menampilkan %d dari %d void item. Persempit rentang tanggal untuk data lengkap.",
				len(report.Items), report.ItemsTotal))
		f.SetCellStyle(shItem, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Void_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}

// BuildDiscountReportExcel menyusun file Excel laporan diskon & komplimen
// dengan filter yang sama persis dengan halaman /discount-report: rentang
// tanggal & outlet. scopeIDs berasal dari role admin sehingga hasil export tak
// melebihi hak akses user.
func BuildDiscountReportExcel(dateFrom, dateTo, outletID string, scopeIDs []string) ([]byte, string, error) {
	report, err := GetDiscountReport(dateFrom, dateTo, outletID, scopeIDs, 1, exportTxLimit)
	if err != nil {
		return nil, "", err
	}

	loc := GetTimezoneLocation()
	tzName, _ := GetTimezone()
	outletLabel := resolveOutletLabel(outletID, scopeIDs)
	companyName, _ := GetSetting("company_name")

	f := excelize.NewFile()
	defer f.Close()

	st, err := buildExportStyles(f)
	if err != nil {
		return nil, "", err
	}

	const (
		shSummary = "Ringkasan"
		shList    = "Daftar Order"
	)
	f.SetSheetName("Sheet1", shSummary)
	if _, err := f.NewSheet(shList); err != nil {
		return nil, "", err
	}

	// ── Sheet Ringkasan ──────────────────────────────────────────────────
	f.SetColWidth(shSummary, "A", "A", 30)
	f.SetColWidth(shSummary, "B", "B", 30)

	row := 1
	f.SetCellValue(shSummary, "A1", "LAPORAN DISKON & KOMPLIMEN")
	f.MergeCell(shSummary, "A1", "B1")
	f.SetCellStyle(shSummary, "A1", "B1", st.title)
	if strings.TrimSpace(companyName) != "" {
		row++
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), companyName)
	}
	row += 2
	setKV := func(label string, value interface{}, valStyle int) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.label)
		f.SetCellValue(shSummary, fmt.Sprintf("B%d", row), value)
		if valStyle != 0 {
			f.SetCellStyle(shSummary, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valStyle)
		}
		row++
	}
	setSection := func(title string) {
		f.SetCellValue(shSummary, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.section)
		row++
	}

	setKV("Periode", fmtDateID(dateFrom)+" s/d "+fmtDateID(dateTo), 0)
	setKV("Outlet", outletLabel, 0)
	setKV("Dibuat pada", fmtTimeID(time.Now().In(loc))+" ("+tzName+")", 0)
	row++

	sum := report.Summary
	setSection("RINGKASAN")
	setKV("Jumlah Order", sum.TotalOrders, st.num)
	setKV("Bruto", sum.Gross, st.money)
	setKV("Total Diskon", sum.Discount, st.money)
	setKV("Nilai Komplimen", sum.Compliment, st.money)
	setKV("Net (Dibayar)", sum.Net, st.moneyTot)
	row++

	f.SetCellValue(shSummary, fmt.Sprintf("A%d", row),
		"Keterangan: Bruto = Net + Diskon + Komplimen. Diskon mencakup diskon bill maupun per item; Komplimen = nilai item yang digratiskan.")
	f.MergeCell(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
	f.SetCellStyle(shSummary, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.note)

	// ── Sheet Daftar Order ───────────────────────────────────────────────
	setHeaderRow(f, shList, []string{"No", "Waktu", "Outlet", "Pelanggan", "Bruto", "Diskon", "Komplimen", "Net (Dibayar)"}, st.header)
	f.SetColWidth(shList, "A", "A", 5)
	f.SetColWidth(shList, "B", "B", 18)
	f.SetColWidth(shList, "C", "D", 22)
	f.SetColWidth(shList, "E", "H", 15)

	// Service mengurutkan DESC (terbaru dulu); untuk laporan urutkan kronologis.
	rowsData := make([]models.DiscountReportRow, len(report.Data))
	copy(rowsData, report.Data)
	for i, j := 0, len(rowsData)-1; i < j; i, j = i+1, j-1 {
		rowsData[i], rowsData[j] = rowsData[j], rowsData[i]
	}
	var tGross, tDisc, tComp, tNet float64
	for i, d := range rowsData {
		r := i + 2
		f.SetCellValue(shList, fmt.Sprintf("A%d", r), i+1)
		// CreatedAt dari service sudah dalam zona aplikasi (tanpa penanda zona),
		// jadi cukup diformat ulang tanpa konversi.
		waktu := d.CreatedAt
		if t, err := time.Parse("2006-01-02T15:04:05", d.CreatedAt); err == nil {
			waktu = fmtTimeID(t)
		}
		f.SetCellValue(shList, fmt.Sprintf("B%d", r), waktu)
		f.SetCellValue(shList, fmt.Sprintf("C%d", r), d.OutletName)
		f.SetCellValue(shList, fmt.Sprintf("D%d", r), d.CustomerName)
		f.SetCellValue(shList, fmt.Sprintf("E%d", r), d.Gross)
		if d.Discount > 0 {
			f.SetCellValue(shList, fmt.Sprintf("F%d", r), d.Discount)
		}
		if d.Compliment > 0 {
			f.SetCellValue(shList, fmt.Sprintf("G%d", r), d.Compliment)
		}
		f.SetCellValue(shList, fmt.Sprintf("H%d", r), d.Net)
		f.SetCellStyle(shList, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.num)
		f.SetCellStyle(shList, fmt.Sprintf("E%d", r), fmt.Sprintf("H%d", r), st.money)
		tGross += d.Gross
		tDisc += d.Discount
		tComp += d.Compliment
		tNet += d.Net
	}
	if len(rowsData) > 0 {
		r := len(rowsData) + 2
		f.SetCellValue(shList, fmt.Sprintf("A%d", r), "TOTAL")
		f.MergeCell(shList, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r))
		f.SetCellValue(shList, fmt.Sprintf("E%d", r), tGross)
		f.SetCellValue(shList, fmt.Sprintf("F%d", r), tDisc)
		f.SetCellValue(shList, fmt.Sprintf("G%d", r), tComp)
		f.SetCellValue(shList, fmt.Sprintf("H%d", r), tNet)
		f.SetCellStyle(shList, fmt.Sprintf("A%d", r), fmt.Sprintf("D%d", r), st.numTot)
		f.SetCellStyle(shList, fmt.Sprintf("E%d", r), fmt.Sprintf("H%d", r), st.moneyTot)
	} else {
		f.SetCellValue(shList, "A2", "Tidak ada diskon / komplimen pada periode ini.")
		f.SetCellStyle(shList, "A2", "A2", st.note)
	}
	if report.Total > len(report.Data) {
		r := len(rowsData) + 4
		f.SetCellValue(shList, fmt.Sprintf("A%d", r),
			fmt.Sprintf("Catatan: menampilkan %d dari %d order. Persempit rentang tanggal untuk data lengkap.",
				len(report.Data), report.Total))
		f.SetCellStyle(shList, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), st.note)
	}

	if idx, err := f.GetSheetIndex(shSummary); err == nil {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Laporan-Diskon_%s_%s_sd_%s.xlsx", slugFilename(outletLabel), dateFrom, dateTo)
	return buf.Bytes(), filename, nil
}
