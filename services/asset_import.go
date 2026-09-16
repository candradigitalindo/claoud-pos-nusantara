package services

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

// Impor aset dari Excel.
//
// Mendata ratusan aset lama satu per satu lewat form adalah pekerjaan berhari-hari;
// staf lebih cepat mengetik di Excel sambil berkeliling. Yang dijaga di sini:
// berkas yang diunggah TIDAK boleh langsung masuk. Setiap baris divalidasi dan
// ditampilkan dulu hasilnya, baru disimpan — supaya salah ketik satu kolom tidak
// melahirkan ratusan aset yang salah dan harus dibersihkan satu per satu.

const importSheet = "Data Aset"

// Kolom template. Urutannya mengikat: pembaca mencocokkan header, bukan posisi,
// tapi urutan ini yang dipakai saat membuat template.
var importColumns = []struct {
	Header   string
	Width    float64
	Required bool
	Help     string
	Example  string
}{
	{"Kode Outlet*", 14, true, "Kode outlet tempat aset berada. Lihat sheet Referensi.", "NHC01"},
	{"Nama Aset*", 30, true, "Nama barang sejelas mungkin, termasuk ukuran/kapasitas bila ada.", "AC Daikin 1PK"},
	{"Kategori", 22, false, "Harus persis salah satu dari sheet Referensi. Kosongkan bila belum jelas.", "AC & Pendingin"},
	{"Jumlah*", 10, true, "Angka bulat lebih dari 0. Untuk mode Tunggal isi 1.", "1"},
	{"Satuan", 10, false, "unit, buah, set, pasang. Kosong dianggap 'unit'.", "unit"},
	{"Mode", 12, false, "Tunggal (bernomor per unit) atau Massal (satu baris banyak unit).", "Tunggal"},
	{"Merk", 16, false, "Merk pabrikan.", "Daikin"},
	{"Tipe", 16, false, "Tipe/model dari pabrikan.", "FTKC25"},
	{"Nomor Seri", 20, false, "Nomor seri pada badan barang. Sangat membantu saat opname.", "SN12345678"},
	{"Kondisi", 14, false, "Baik, Rusak Ringan, atau Rusak Berat. Kosong dianggap Baik.", "Baik"},
	{"Lokasi", 22, false, "Ruang atau area penempatan.", "Ruang Tamu Lt.1"},
	{"Penanggung Jawab", 20, false, "Nama orang yang memegang barang ini.", "Budi"},
	{"Tgl Perolehan", 15, false, "Format YYYY-MM-DD. Dipakai menghitung penyusutan.", "2025-01-15"},
	{"Harga Perolehan", 16, false, "Harga per satu unit, angka saja tanpa titik/Rp.", "8000000"},
	{"Umur Ekonomis (bln)", 18, false, "Kosongkan untuk memakai bawaan kategorinya.", ""},
	{"Nilai Residu", 14, false, "Nilai sisa di akhir umur ekonomis. Kosong dianggap 0.", "0"},
	{"Garansi s/d", 14, false, "Format YYYY-MM-DD. Kosongkan bila tanpa garansi.", ""},
	{"Catatan", 28, false, "Keterangan tambahan.", ""},
}

// BuildAssetImportTemplate menyusun berkas Excel berisi contoh, panduan per
// kolom, dan daftar referensi yang diambil dari data nyata — bukan contoh
// karangan yang nanti ditolak saat diunggah.
func BuildAssetImportTemplate(outletScope []string) (*bytes.Buffer, string, error) {
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", importSheet)

	// ── Sheet referensi dulu, karena dropdown menunjuk ke sini ──
	ref := "Referensi"
	f.NewSheet(ref)
	cats, _ := ListAssetCategories(false)
	outlets, _ := listOutletsForImport(outletScope)

	f.SetCellValue(ref, "A1", "KATEGORI ASET")
	f.SetCellValue(ref, "B1", "Umur ekonomis bawaan")
	for i, c := range cats {
		f.SetCellValue(ref, fmt.Sprintf("A%d", i+2), c.Name)
		if c.UsefulLifeMonths > 0 {
			f.SetCellValue(ref, fmt.Sprintf("B%d", i+2), fmt.Sprintf("%d bulan", c.UsefulLifeMonths))
		} else {
			f.SetCellValue(ref, fmt.Sprintf("B%d", i+2), "tidak disusutkan")
		}
	}
	f.SetCellValue(ref, "D1", "KODE OUTLET")
	f.SetCellValue(ref, "E1", "Nama outlet")
	for i, o := range outlets {
		f.SetCellValue(ref, fmt.Sprintf("D%d", i+2), o.Code)
		f.SetCellValue(ref, fmt.Sprintf("E%d", i+2), o.Name)
	}
	f.SetCellValue(ref, "G1", "KONDISI")
	for i, v := range []string{"Baik", "Rusak Ringan", "Rusak Berat"} {
		f.SetCellValue(ref, fmt.Sprintf("G%d", i+2), v)
	}
	f.SetCellValue(ref, "H1", "MODE")
	for i, v := range []string{"Tunggal", "Massal"} {
		f.SetCellValue(ref, fmt.Sprintf("H%d", i+2), v)
	}
	f.SetColWidth(ref, "A", "A", 26)
	f.SetColWidth(ref, "B", "B", 22)
	f.SetColWidth(ref, "D", "E", 20)
	f.SetColWidth(ref, "G", "H", 16)

	// ── Sheet panduan ──
	guide := "Panduan"
	f.NewSheet(guide)
	f.SetCellValue(guide, "A1", "CARA MENGISI")
	rows := [][]string{
		{"1.", "Isi satu baris untuk satu jenis barang, mulai dari baris ke-2 di sheet \"Data Aset\"."},
		{"2.", "Kolom bertanda * wajib diisi. Baris yang kolom wajibnya kosong akan ditolak."},
		{"3.", "Kolom Kategori, Kode Outlet, Kondisi, dan Mode punya pilihan — klik selnya, lalu pilih dari daftar."},
		{"4.", "Jangan mengubah judul kolom di baris 1, jangan menghapus atau menambah kolom."},
		{"5.", "Contoh pengisian ada di sheet \"Contoh Pengisian\" — sengaja dipisah agar tidak ikut terkirim."},
		{"6.", "Tanggal ditulis YYYY-MM-DD, misalnya 2025-01-15."},
		{"7.", "Harga ditulis angka saja: 8000000 — bukan Rp 8.000.000 dan bukan 8.000.000."},
		{"8.", "Mode Tunggal berarti satu baris = satu unit bernomor sendiri. Jumlah 3 dengan mode Tunggal akan menjadi 3 aset terpisah."},
		{"9.", "Mode Massal berarti satu baris berisi banyak unit seragam, misalnya 40 kursi yang sama."},
		{"10.", "Simpan sebagai .xlsx, lalu unggah lewat menu Daftar Aset → Impor Excel."},
		{"11.", "Setelah diunggah, sistem memeriksa dulu seluruh baris dan menampilkan hasilnya. Tidak ada yang tersimpan sebelum Anda menekan Simpan."},
		{"", ""},
		{"PENTING", "Nomor aset dibuat otomatis oleh sistem — tidak perlu diisi di sini."},
	}
	for i, r := range rows {
		f.SetCellValue(guide, fmt.Sprintf("A%d", i+3), r[0])
		f.SetCellValue(guide, fmt.Sprintf("B%d", i+3), r[1])
	}
	start := len(rows) + 5
	f.SetCellValue(guide, fmt.Sprintf("A%d", start), "ARTI TIAP KOLOM")
	f.SetCellValue(guide, fmt.Sprintf("A%d", start+1), "Kolom")
	f.SetCellValue(guide, fmt.Sprintf("B%d", start+1), "Wajib?")
	f.SetCellValue(guide, fmt.Sprintf("C%d", start+1), "Penjelasan")
	for i, c := range importColumns {
		r := start + 2 + i
		f.SetCellValue(guide, fmt.Sprintf("A%d", r), c.Header)
		wajib := "opsional"
		if c.Required {
			wajib = "WAJIB"
		}
		f.SetCellValue(guide, fmt.Sprintf("B%d", r), wajib)
		f.SetCellValue(guide, fmt.Sprintf("C%d", r), c.Help)
	}
	f.SetColWidth(guide, "A", "A", 22)
	f.SetColWidth(guide, "B", "B", 10)
	f.SetColWidth(guide, "C", "C", 82)

	// ── Sheet data ──
	headStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4A7C62"}},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	for i, c := range importColumns {
		col, _ := excelize.ColumnNumberToName(i + 1)
		cell := fmt.Sprintf("%s1", col)
		f.SetCellValue(importSheet, cell, c.Header)
		f.SetCellStyle(importSheet, cell, cell, headStyle)
		f.SetColWidth(importSheet, col, col, c.Width)
	}

	// Contoh pengisian sengaja TIDAK ditaruh di lembar data.
	//
	// Baris contoh di lembar yang akan diunggah adalah jebakan: staf yang lupa
	// menghapusnya akan melahirkan aset hantu, dan itu ketahuan belakangan saat
	// opname. Contohnya ditaruh di lembar sendiri agar tetap bisa dilihat
	// berdampingan tanpa ikut terkirim.
	ex := "Contoh Pengisian"
	f.NewSheet(ex)
	exStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF7E0"}},
		Font: &excelize.Font{Italic: true, Color: "8A6D1F"},
	})
	f.SetCellValue(ex, "A1", "Contoh pengisian — JANGAN disalin ke lembar Data Aset beserta barisnya;")
	f.SetCellValue(ex, "A2", "ketik ulang sesuai barang Anda sendiri.")
	for i, c := range importColumns {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetCellValue(ex, fmt.Sprintf("%s4", col), c.Header)
		f.SetCellStyle(ex, fmt.Sprintf("%s4", col), fmt.Sprintf("%s4", col), headStyle)
		f.SetColWidth(ex, col, col, c.Width)
		if c.Example != "" {
			cell := fmt.Sprintf("%s5", col)
			f.SetCellValue(ex, cell, c.Example)
			f.SetCellStyle(ex, cell, cell, exStyle)
		}
	}
	f.SetCellValue(ex, "A7", "Contoh kedua — barang seragam berjumlah banyak:")
	for i, v := range []string{"NHC01", "Kursi Tamu Kayu", "Mebel", "40", "unit", "Massal", "", "", "",
		"Baik", "Area Indoor", "Budi", "2025-03-10", "450000", "", "", "", "Pembelian renovasi 2025"} {
		if v == "" {
			continue
		}
		col, _ := excelize.ColumnNumberToName(i + 1)
		cell := fmt.Sprintf("%s8", col)
		f.SetCellValue(ex, cell, v)
		f.SetCellStyle(ex, cell, cell, exStyle)
	}
	f.SetRowHeight(importSheet, 1, 32)
	f.SetPanes(importSheet, &excelize.Panes{Freeze: true, Split: false, XSplit: 0, YSplit: 1,
		TopLeftCell: "A2", ActivePane: "bottomLeft"})

	// Dropdown untuk kolom berpilihan — menunjuk daftar di sheet Referensi
	addList := func(colLetter, formula string) {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = fmt.Sprintf("%s2:%s1000", colLetter, colLetter)
		dv.SetSqrefDropList(formula)
		dv.ShowErrorMessage = true
		dv.ErrorTitle = strPtr("Pilihan tidak dikenal")
		dv.Error = strPtr("Pilih salah satu dari daftar. Daftar lengkapnya ada di sheet Referensi.")
		f.AddDataValidation(importSheet, dv)
	}
	if len(outlets) > 0 {
		addList("A", fmt.Sprintf("Referensi!$D$2:$D$%d", len(outlets)+1))
	}
	if len(cats) > 0 {
		addList("C", fmt.Sprintf("Referensi!$A$2:$A$%d", len(cats)+1))
	}
	addList("F", "Referensi!$H$2:$H$3")
	addList("J", "Referensi!$G$2:$G$4")

	f.SetActiveSheet(0)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("Template-Pendataan-Aset_%s.xlsx",
		time.Now().In(GetTimezoneLocation()).Format("20060102"))
	return buf, name, nil
}

func strPtr(s string) *string { return &s }

type importOutlet struct{ ID, Code, Name string }

func listOutletsForImport(outletScope []string) ([]importOutlet, error) {
	q := `SELECT id, code, name FROM outlets WHERE is_active = true`
	args := []interface{}{}
	if outletScope != nil {
		q += ` AND id = ANY($1::text[])`
		args = append(args, pqStringArray(outletScope))
	}
	q += ` ORDER BY code`
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []importOutlet{}
	for rows.Next() {
		var o importOutlet
		if rows.Scan(&o.ID, &o.Code, &o.Name) == nil {
			out = append(out, o)
		}
	}
	return out, rows.Err()
}

// ── Membaca berkas unggahan ─────────────────────────────────────────────────

// AssetImportRow adalah hasil pemeriksaan satu baris Excel.
type AssetImportRow struct {
	Row      int      `json:"row"`      // nomor baris di Excel, supaya mudah dicari
	Name     string   `json:"name"`
	Outlet   string   `json:"outlet"`
	Category string   `json:"category"`
	Qty      int      `json:"qty"`
	Mode     string   `json:"mode"`
	Price    float64  `json:"price"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	// Assets = jumlah baris aset yang akan terbentuk (mode tunggal qty 3 → 3).
	Assets int `json:"assets"`

	req      models.AssetRequest
	outletID string
}

type AssetImportResult struct {
	Mode       string           `json:"mode"` // preview | commit
	TotalRows  int              `json:"total_rows"`
	ValidRows  int              `json:"valid_rows"`
	ErrorRows  int              `json:"error_rows"`
	AssetsMade int              `json:"assets_made"`
	Rows       []AssetImportRow `json:"rows"`
	Message    string           `json:"message"`
}

var importConditionMap = map[string]string{
	"baik": "baik", "rusak ringan": "rusak_ringan", "rusak_ringan": "rusak_ringan",
	"rusak berat": "rusak_berat", "rusak_berat": "rusak_berat",
}

// ImportAssetsFromExcel membaca berkas dan memeriksa tiap baris.
//
// commit=false hanya memeriksa (tidak menulis apa pun); commit=true menyimpan
// baris yang sah. Baris bermasalah TIDAK menggagalkan seluruh berkas — yang
// benar tetap masuk, yang salah dilaporkan nomor barisnya agar bisa diperbaiki
// di Excel lalu diunggah ulang.
func ImportAssetsFromExcel(content []byte, commit bool, actor string, outletScope []string) (*AssetImportResult, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, Invalid("berkas tidak bisa dibaca — pastikan berformat .xlsx")
	}
	defer f.Close()

	sheet := importSheet
	if idx, _ := f.GetSheetIndex(sheet); idx < 0 {
		sheet = f.GetSheetName(0)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, Invalid("gagal membaca sheet %q", sheet)
	}
	if len(rows) < 2 {
		return nil, Invalid("berkas belum berisi data — isi mulai baris ke-2 pada sheet %q", importSheet)
	}

	// Cocokkan kolom lewat JUDUL, bukan posisi: staf kerap menyisipkan kolom
	// bantu sendiri, dan itu tidak boleh membuat seluruh berkas salah baca.
	idxOf := map[string]int{}
	for i, h := range rows[0] {
		key := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(h), "*")))
		idxOf[key] = i
	}
	need := []string{"kode outlet", "nama aset", "jumlah"}
	for _, n := range need {
		if _, ok := idxOf[n]; !ok {
			return nil, Invalid("kolom %q tidak ditemukan — gunakan template yang diunduh dari aplikasi", n)
		}
	}
	cell := func(r []string, key string) string {
		i, ok := idxOf[key]
		if !ok || i >= len(r) {
			return ""
		}
		return strings.TrimSpace(r[i])
	}

	// Peta outlet & kategori dibaca sekali, bukan per baris.
	outlets, _ := listOutletsForImport(outletScope)
	outletByCode := map[string]importOutlet{}
	for _, o := range outlets {
		outletByCode[strings.ToLower(o.Code)] = o
	}
	cats, _ := ListAssetCategories(false)
	catByName := map[string]models.AssetCategory{}
	for _, c := range cats {
		catByName[strings.ToLower(c.Name)] = c
	}

	res := &AssetImportResult{Mode: "preview", Rows: []AssetImportRow{}}
	if commit {
		res.Mode = "commit"
	}
	seenSerial := map[string]int{}

	for i := 1; i < len(rows); i++ {
		raw := rows[i]
		line := AssetImportRow{Row: i + 1, Errors: []string{}}
		if strings.TrimSpace(strings.Join(raw, "")) == "" {
			continue // baris kosong dilewati diam-diam
		}
		res.TotalRows++

		line.Name = cell(raw, "nama aset")
		line.Outlet = cell(raw, "kode outlet")
		line.Category = cell(raw, "kategori")

		if line.Name == "" {
			line.Errors = append(line.Errors, "nama aset kosong")
		}
		o, ok := outletByCode[strings.ToLower(line.Outlet)]
		if !ok {
			if line.Outlet == "" {
				line.Errors = append(line.Errors, "kode outlet kosong")
			} else {
				line.Errors = append(line.Errors, fmt.Sprintf("kode outlet %q tidak dikenal atau di luar akses Anda", line.Outlet))
			}
		}
		line.outletID = o.ID

		qty, qerr := strconv.Atoi(strings.TrimSpace(cell(raw, "jumlah")))
		if qerr != nil || qty <= 0 {
			line.Errors = append(line.Errors, "jumlah harus angka bulat lebih dari 0")
			qty = 0
		}
		line.Qty = qty

		mode := strings.ToLower(cell(raw, "mode"))
		switch mode {
		case "", "massal":
			mode = "massal"
		case "tunggal":
			mode = "tunggal"
		default:
			line.Errors = append(line.Errors, fmt.Sprintf("mode %q tidak dikenal — isi Tunggal atau Massal", cell(raw, "mode")))
		}
		line.Mode = mode

		cond := "baik"
		if v := cell(raw, "kondisi"); v != "" {
			c, ok := importConditionMap[strings.ToLower(v)]
			if !ok {
				line.Errors = append(line.Errors, fmt.Sprintf("kondisi %q tidak dikenal", v))
			} else {
				cond = c
			}
		}

		category := ""
		if line.Category != "" {
			c, ok := catByName[strings.ToLower(line.Category)]
			if !ok {
				line.Errors = append(line.Errors, fmt.Sprintf("kategori %q tidak terdaftar — tambahkan dulu di menu Kategori Aset", line.Category))
			} else {
				category = c.Name
				line.Category = c.Name
			}
		}

		price := parseImportNumber(cell(raw, "harga perolehan"))
		if price < 0 {
			line.Errors = append(line.Errors, "harga perolehan tidak boleh negatif")
		}
		line.Price = price
		residu := parseImportNumber(cell(raw, "nilai residu"))
		life := int(parseImportNumber(cell(raw, "umur ekonomis (bln)")))

		buyDate, derr := parseImportDate(cell(raw, "tgl perolehan"))
		if derr != nil {
			line.Errors = append(line.Errors, "Tgl Perolehan: "+derr.Error())
		}
		warranty, werr := parseImportDate(cell(raw, "garansi s/d"))
		if werr != nil {
			line.Errors = append(line.Errors, "Garansi s/d: "+werr.Error())
		}

		serial := cell(raw, "nomor seri")
		if serial != "" {
			if prev, dup := seenSerial[strings.ToLower(serial)]; dup {
				line.Errors = append(line.Errors, fmt.Sprintf("nomor seri sama dengan baris %d", prev))
			} else {
				seenSerial[strings.ToLower(serial)] = line.Row
			}
		}

		line.req = models.AssetRequest{
			OutletID: o.ID, Name: line.Name, Category: category, Quantity: qty,
			Unit: firstNonEmpty(cell(raw, "satuan"), "unit"), TrackingMode: mode,
			SerialNumber: serial, Brand: cell(raw, "merk"), Model: cell(raw, "tipe"),
			Condition: cond, Status: "aktif", Location: cell(raw, "lokasi"),
			PicName: cell(raw, "penanggung jawab"), PurchaseDate: buyDate,
			PurchasePrice: price, WarrantyUntil: warranty, UsefulLifeMonths: life,
			ResidualValue: residu, Notes: cell(raw, "catatan"),
		}
		if residu > price && price > 0 {
			line.Errors = append(line.Errors, "nilai residu melebihi harga perolehan")
		}

		line.Valid = len(line.Errors) == 0
		if line.Valid {
			line.Assets = 1
			if mode == "tunggal" {
				line.Assets = qty
			}
			res.ValidRows++
			res.AssetsMade += line.Assets
		} else {
			res.ErrorRows++
		}
		res.Rows = append(res.Rows, line)
	}

	if res.TotalRows == 0 {
		return nil, Invalid("tidak ada baris berisi data yang bisa dibaca")
	}

	if !commit {
		res.Message = fmt.Sprintf("%d baris diperiksa: %d siap disimpan (%d aset), %d perlu diperbaiki",
			res.TotalRows, res.ValidRows, res.AssetsMade, res.ErrorRows)
		return res, nil
	}

	// Simpan. Tiap baris berdiri sendiri: satu baris gagal tidak membatalkan
	// yang sudah tersimpan, dan nomor barisnya dilaporkan.
	made := 0
	for i := range res.Rows {
		r := &res.Rows[i]
		if !r.Valid {
			continue
		}
		req := r.req
		n := 1
		if req.TrackingMode == "tunggal" {
			n = req.Quantity
			req.Quantity = 1
		}
		for k := 0; k < n; k++ {
			if _, err := CreateAsset(req, actor); err != nil {
				r.Valid = false
				r.Errors = append(r.Errors, err.Error())
				res.ValidRows--
				res.ErrorRows++
				break
			}
			made++
		}
	}
	res.AssetsMade = made
	res.Message = fmt.Sprintf("%d aset tersimpan dari %d baris; %d baris perlu diperbaiki",
		made, res.TotalRows, res.ErrorRows)
	return res, nil
}

func firstNonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

// parseImportNumber menerima "8000000", "8.000.000", "8,000,000", dan "Rp 8.000.000".
// Staf mengetik di Excel, bukan di formulir — pemisah ribuan pasti muncul.
func parseImportNumber(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	s = strings.NewReplacer("rp", "", " ", "", "_", "").Replace(s)
	// Bila ada koma DAN titik, yang terakhir muncul dianggap pemisah desimal.
	lastComma, lastDot := strings.LastIndex(s, ","), strings.LastIndex(s, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case lastComma >= 0:
		// "1,5" → desimal; "8,000,000" → pemisah ribuan
		if len(s)-lastComma-1 == 3 && strings.Count(s, ",") > 0 && len(s) > 4 {
			s = strings.ReplaceAll(s, ",", "")
		} else {
			s = strings.Replace(s, ",", ".", 1)
		}
	case lastDot >= 0:
		if len(s)-lastDot-1 == 3 {
			s = strings.ReplaceAll(s, ".", "")
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseImportDate menerima YYYY-MM-DD, DD/MM/YYYY, dan angka serial Excel.
func parseImportDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2/1/2006", "02-01-2006", "2006/01/02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	// Sel bertipe tanggal terbaca sebagai angka serial Excel.
	if n, err := strconv.ParseFloat(s, 64); err == nil && n > 20000 && n < 60000 {
		if t, err := excelize.ExcelDateToTime(n, false); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("format tanggal tidak dikenali (%q) — tulis YYYY-MM-DD, misalnya 2025-01-15", s)
}
