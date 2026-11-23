package services

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"cloud-pos/database"

	"github.com/xuri/excelize/v2"
)

// assetListFixture membuat outlet uji berisi n aset. 150 aset pertama sengaja
// ber-created_at kembar — seperti hasil impor Excel dalam satu transaksi —
// supaya pemutus seri urutan ikut teruji. Aset terakhir berstatus dihapus dan
// tidak boleh ikut daftar harian.
func assetListFixture(t *testing.T, n int) string {
	t.Helper()
	outlet := recvOutlet(t)
	t.Cleanup(func() { execScratch(`DELETE FROM assets WHERE outlet_id = $1`, outlet) })
	for i := 0; i < n; i++ {
		created := "2026-01-01 08:00:00"
		if i >= 150 {
			created = fmt.Sprintf("2026-02-01 08:%02d:%02d", i/60, i%60)
		}
		cond, status := "baik", "aktif"
		if i%50 == 0 {
			cond = "rusak_ringan"
		}
		if i%70 == 1 {
			status = "perbaikan"
		}
		if i == n-1 {
			status = "dihapus"
		}
		if _, err := database.DB.Exec(`
			INSERT INTO assets (id, outlet_id, name, quantity, purchase_price, condition, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			NewULID(), outlet, fmt.Sprintf("Kursi Uji %03d", i), 1+i%3, 1000*(i+1), cond, status, created); err != nil {
			t.Fatalf("aset uji %d: %v", i, err)
		}
	}
	return outlet
}

// Daftar Perlengkapan dipecah 100 baris per halaman. Gabungan semua halaman
// harus sama persis dengan daftar utuh — tanpa baris ganda atau terlewat,
// termasuk untuk aset yang created_at-nya kembar — dan ringkasannya dihitung
// atas seluruh hasil saringan, bukan hanya halaman yang tampil.
func TestAssetListPaginationAndSummary(t *testing.T) {
	requireDB(t)
	outlet := assetListFixture(t, 206) // 205 tampil + 1 dihapus
	f := AssetListFilter{OutletID: outlet}

	all, err := ListAssets(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 205 {
		t.Fatalf("daftar utuh: dapat %d aset, harusnya 205 (yang dihapus tidak ikut)", len(all))
	}

	var paged []string
	seen := map[string]bool{}
	for p, want := range []int{100, 100, 5} {
		rows, s, got, err := ListAssetsPage(f, p+1, 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != p+1 || len(rows) != want {
			t.Fatalf("halaman %d: dapat halaman %d berisi %d baris, harusnya %d baris", p+1, got, len(rows), want)
		}
		if s.Count != 205 {
			t.Fatalf("halaman %d: ringkasan menghitung %d aset, harusnya 205", p+1, s.Count)
		}
		for _, a := range rows {
			if seen[a.ID] {
				t.Fatalf("aset %s (%s) muncul di dua halaman", a.ID, a.Name)
			}
			seen[a.ID] = true
			paged = append(paged, a.ID)
		}
	}
	for i, a := range all {
		if paged[i] != a.ID {
			t.Fatalf("urutan ke-%d berbeda: halaman %s, daftar utuh %s", i, paged[i], a.ID)
		}
	}

	var units, acq int
	attention := 0
	for i := 0; i < 205; i++ {
		units += 1 + i%3
		acq += 1000 * (i + 1) * (1 + i%3)
		if i%50 == 0 || i%70 == 1 {
			attention++
		}
	}
	_, s, _, err := ListAssetsPage(f, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if s.Units != units || s.Acquisition != float64(acq) || s.Book != float64(acq) || s.Attention != attention {
		t.Fatalf("ringkasan = %+v, harusnya unit %d, perolehan & nilai buku %d, perlu perhatian %d", s, units, acq, attention)
	}

	// Halaman di luar jangkauan digeser ke halaman terakhir.
	rows, _, got, err := ListAssetsPage(f, 99, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 || len(rows) != 5 {
		t.Fatalf("halaman 99: dapat halaman %d berisi %d baris, harusnya halaman 3 berisi 5", got, len(rows))
	}

	// Saringan ikut ke ringkasan: "Kursi Uji 01" = 010–019.
	_, s, _, err = ListAssetsPage(AssetListFilter{OutletID: outlet, Search: "Kursi Uji 01"}, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if s.Count != 10 {
		t.Fatalf("pencarian: ringkasan %d aset, harusnya 10", s.Count)
	}

	// Di luar cakupan outlet: kosong, tetap halaman 1.
	rows, s, got, err = ListAssetsPage(AssetListFilter{OutletID: outlet, OutletScope: []string{"__none__"}}, 2, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 || s.Count != 0 || got != 1 {
		t.Fatalf("di luar cakupan: %d baris, ringkasan %d, halaman %d — harusnya kosong di halaman 1", len(rows), s.Count, got)
	}
}

// Unduh Excel tidak ikut paginasi: berkasnya memuat SEMUA aset yang lolos
// saringan, dan baris TOTAL-nya sama dengan kartu ringkasan di layar.
func TestExportAssetListIgnoresPagination(t *testing.T) {
	requireDB(t)
	outlet := assetListFixture(t, 206)
	f := AssetListFilter{OutletID: outlet}
	_, s, _, err := ListAssetsPage(f, 1, 100)
	if err != nil {
		t.Fatal(err)
	}

	buf, filename, err := ExportAssetList(f, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if filename == "" {
		t.Fatal("nama berkas kosong")
	}
	x, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	rows, err := x.GetRows("Perlengkapan", excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}

	const headRow = 5 // baris ke-5 = indeks 4
	header := rows[headRow-1]
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	data := rows[headRow : headRow+s.Count]
	if len(rows) != headRow+s.Count+1 {
		t.Fatalf("berkas berisi %d baris data+total, harusnya %d aset + 1 baris TOTAL", len(rows)-headRow, s.Count)
	}
	if data[0][col["No"]] != "1" || data[len(data)-1][col["No"]] != strconv.Itoa(s.Count) {
		t.Fatalf("penomoran salah: baris pertama %q, terakhir %q", data[0][col["No"]], data[len(data)-1][col["No"]])
	}
	if data[0][col["Status"]] == "aktif" {
		t.Fatal("status masih kode internal, harusnya label (Aktif)")
	}

	total := rows[headRow+s.Count]
	if total[col["Nomor Aset"]] != fmt.Sprintf("TOTAL — %d aset", s.Count) {
		t.Fatalf("label total = %q", total[col["Nomor Aset"]])
	}
	num := func(name string) float64 {
		v, err := strconv.ParseFloat(total[col[name]], 64)
		if err != nil {
			t.Fatalf("total %s = %q: %v", name, total[col[name]], err)
		}
		return v
	}
	if num("Jumlah") != float64(s.Units) ||
		math.Abs(num("Total Nilai Perolehan")-s.Acquisition) > 0.5 ||
		math.Abs(num("Total Nilai Buku")-s.Book) > 0.5 {
		t.Fatalf("baris TOTAL (unit %v, perolehan %v, buku %v) tidak sama dengan ringkasan %+v",
			num("Jumlah"), num("Total Nilai Perolehan"), num("Total Nilai Buku"), s)
	}
}
