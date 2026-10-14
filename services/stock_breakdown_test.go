package services

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Pemotongan: satu bahan → beberapa hasil, lewat POLA bernama per bahan (mis.
// ayam "Per Bagian" vs "Potong 8"). Stok & HPP mengikuti hasil AKTUAL; biaya
// bahan = biaya FIFO yang benar-benar terpotong, dibagi ke hasil pola terpilih
// sebanding qty × (bobot ÷ hasil standar).

func bdNear(a, b float64) bool { return math.Abs(a-b) < 0.005 }

// ── Pembagian biaya (tanpa database) ─────────────────────────

func TestAllocateBreakdownCostByWeight(t *testing.T) {
	// Pola per 1 ekor: paha 2 (bobot 2), dada 2 (bobot 3), sayap 2 (bobot 1);
	// 4 ekor, hasil = standar 8/8/8 → porsi biaya 2:3:1.
	per, tot := allocateBreakdownCost(168000, []breakdownShare{{8, 2, 2}, {8, 2, 3}, {8, 2, 1}})
	wantPer, wantTot := []float64{7000, 10500, 3500}, []float64{56000, 84000, 28000}
	for i := range wantPer {
		if !bdNear(per[i], wantPer[i]) || !bdNear(tot[i], wantTot[i]) {
			t.Errorf("baris %d: per %.4f total %.2f, mau %.2f / %.2f", i, per[i], tot[i], wantPer[i], wantTot[i])
		}
	}
}

func TestAllocateBreakdownCostKeepsTotal(t *testing.T) {
	// Tiga baris setara atas Rp100.000: pembulatan sen tidak boleh membuat
	// rupiah hilang/bertambah — Σ total hasil = biaya bahan.
	_, tot := allocateBreakdownCost(100000, []breakdownShare{{3, 1, 1}, {3, 1, 1}, {3, 1, 1}})
	if sum := tot[0] + tot[1] + tot[2]; !bdNear(sum, 100000) {
		t.Errorf("Σ total %.2f, mau 100000 (%v)", sum, tot)
	}
	// Hasil 0 tidak menanggung biaya.
	per, tot := allocateBreakdownCost(50000, []breakdownShare{{0, 2, 1}, {4, 2, 1}})
	if tot[0] != 0 || per[0] != 0 || !bdNear(tot[1], 50000) || !bdNear(per[1], 12500) {
		t.Errorf("hasil 0: per %v total %v, mau [0 12500] [0 50000]", per, tot)
	}
	// Bahan tanpa biaya (batch HPP 0) → semua hasil 0, tanpa pembagian nol.
	if per, tot := allocateBreakdownCost(0, []breakdownShare{{2, 1, 1}}); per[0] != 0 || tot[0] != 0 {
		t.Errorf("biaya 0: per %v total %v", per, tot)
	}
}

// ── Fixture ber-database ──────────────────────────────────────

type bdFixture struct {
	sfx, outlet, wh string
	items           []string
}

// newBreakdownFixture: outlet + gudang outletnya; item dibuat lewat f.item.
func newBreakdownFixture(t *testing.T) *bdFixture {
	t.Helper()
	f := &bdFixture{sfx: strings.ToLower(NewULID()[16:]), outlet: NewULID(), wh: NewULID()}
	t.Cleanup(func() {
		execScratch(`DELETE FROM stock_breakdown_outputs WHERE breakdown_id IN (SELECT id FROM stock_breakdowns WHERE warehouse_id = $1)`, f.wh)
		execScratch(`DELETE FROM stock_breakdowns WHERE warehouse_id = $1`, f.wh)
		for _, id := range f.items {
			execScratch(`DELETE FROM stock_breakdown_patterns WHERE input_item_id = $1`, id)
			execScratch(`DELETE FROM stock_item_breakdowns WHERE input_item_id = $1 OR output_item_id = $1`, id)
			execScratch(`DELETE FROM stock_batches WHERE item_id = $1`, id)
			execScratch(`DELETE FROM stock_movements WHERE item_id = $1`, id)
			execScratch(`DELETE FROM stock_ledger WHERE item_id = $1`, id)
		}
		for _, id := range f.items {
			execScratch(`DELETE FROM stock_items WHERE id = $1`, id)
		}
		execScratch(`DELETE FROM warehouses WHERE id = $1`, f.wh)
		execScratch(`DELETE FROM outlets WHERE id = $1`, f.outlet)
	})
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		f.outlet, "BD"+f.sfx[:8], "Outlet Uji Potong "+f.sfx, "key-potong-"+f.outlet); err != nil {
		t.Fatalf("outlet uji: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id) VALUES ($1, $2, $3, 'outlet', $4)`,
		f.wh, "WBD"+strings.ToUpper(f.sfx), "Gudang Uji Potong "+f.sfx, f.outlet); err != nil {
		t.Fatalf("gudang uji: %v", err)
	}
	return f
}

// item membuat item katalog uji: satuan dasar base, distribusi dist (= ratio base).
func (f *bdFixture) item(t *testing.T, name, base, dist string, ratio float64) string {
	t.Helper()
	id := NewULID()
	f.items = append(f.items, id)
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio) VALUES ($1, $2, $3, $4, $5, $6)`,
		id, "BD"+strings.ToUpper(NewULID()[18:]), name+" Uji "+f.sfx, base, dist, ratio); err != nil {
		t.Fatalf("item uji %s: %v", name, err)
	}
	return id
}

func bdLine(item string, qty, weight float64) models.StockItemBreakdownLineReq {
	return models.StockItemBreakdownLineReq{OutputItemID: item, QtyPerInput: qty, CostWeight: &weight}
}

func bdPattern(name string, lines ...models.StockItemBreakdownLineReq) models.StockBreakdownPatternReq {
	return models.StockBreakdownPatternReq{Name: name, Outputs: lines}
}

func (f *bdFixture) savePatterns(t *testing.T, input string, patterns ...models.StockBreakdownPatternReq) *models.StockItemBreakdownTemplate {
	t.Helper()
	tmpl, err := SaveStockItemBreakdown(input, models.StockItemBreakdownRequest{Patterns: patterns}, "gudang_induk")
	if err != nil {
		t.Fatalf("simpan pola: %v", err)
	}
	return tmpl
}

func bdPatternID(t *testing.T, tmpl *models.StockItemBreakdownTemplate, name string) string {
	t.Helper()
	for _, p := range tmpl.Patterns {
		if p.Name == name {
			return p.ID
		}
	}
	t.Fatalf("pola %q tidak ada", name)
	return ""
}

func (f *bdFixture) stockIn(t *testing.T, itemID string, qty, cost float64, expiry string) {
	t.Helper()
	if err := CreateAdjustment(models.AdjustmentRequest{ItemID: itemID, WarehouseID: f.wh, QtyBase: qty, CostPerBase: cost,
		MovType: "purchase_in", ExpiryDate: expiry}, "uji"); err != nil {
		t.Fatalf("stok masuk: %v", err)
	}
}

func (f *bdFixture) ledger(t *testing.T, itemID string) (qty, avg float64) {
	t.Helper()
	database.DB.QueryRow(`SELECT qty_base, avg_cost FROM stock_ledger WHERE item_id = $1 AND warehouse_id = $2`, itemID, f.wh).Scan(&qty, &avg)
	return qty, avg
}

func (f *bdFixture) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE warehouse_id = $1`, f.wh).Scan(&n)
	return n
}

type bdOut struct {
	item string
	qty  float64
}

func (f *bdFixture) breakdown(input, pattern string, qtyDist float64, outs ...bdOut) (*models.StockBreakdown, error) {
	req := models.StockBreakdownRequest{WarehouseID: f.wh, InputItemID: input, PatternID: pattern, InputQtyDist: qtyDist}
	for _, o := range outs {
		req.Outputs = append(req.Outputs, models.StockBreakdownOutputReq{ItemID: o.item, QtyBase: o.qty})
	}
	return CreateStockBreakdown(req, "dapur_outlet")
}

func bdOutputs(doc *models.StockBreakdown) map[string]models.StockBreakdownOutput {
	m := map[string]models.StockBreakdownOutput{}
	for _, o := range doc.Outputs {
		m[o.ItemID] = o
	}
	return m
}

// chicken: ayam utuh (gr, distribusi ekor = 1000 gr), paha/dada/sayap (pcs)
// dengan pola "Per Bagian" per 1 ekor: paha 2 (bobot 2), dada 2 (3), sayap 2 (1).
type bdChicken struct{ whole, paha, dada, sayap string }

func (f *bdFixture) chicken(t *testing.T) bdChicken {
	t.Helper()
	c := bdChicken{
		whole: f.item(t, "Ayam Utuh", "gr", "ekor", 1000),
		paha:  f.item(t, "Paha", "pcs", "pcs", 1), dada: f.item(t, "Dada", "pcs", "pcs", 1), sayap: f.item(t, "Sayap", "pcs", "pcs", 1),
	}
	f.savePatterns(t, c.whole, bdPattern("Per Bagian", bdLine(c.paha, 2, 2), bdLine(c.dada, 2, 3), bdLine(c.sayap, 2, 1)))
	return c
}

// ── Uji ber-database ──────────────────────────────────────────

// Biaya bahan = FIFO yang benar-benar terpotong (dua batch berbeda harga),
// lalu dibagi ke hasil menurut bobot — angka eksak. Satu-satunya pola aktif
// dipakai tanpa perlu dipilih.
func TestStockBreakdownFifoCostAllocatedByWeight(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	f.stockIn(t, c.whole, 3000, 40, "") // 3 ekor @ Rp40/gr
	f.stockIn(t, c.whole, 5000, 48, "") // 5 ekor @ Rp48/gr

	doc, err := f.breakdown(c.whole, "", 4, bdOut{c.paha, 8}, bdOut{c.dada, 8}, bdOut{c.sayap, 8})
	if err != nil {
		t.Fatalf("pemotongan: %v", err)
	}
	if !regexp.MustCompile(`^POT-\d{8}-\d{3}$`).MatchString(doc.BreakdownNumber) {
		t.Errorf("nomor dokumen %q, mau POT-YYYYMMDD-NNN", doc.BreakdownNumber)
	}
	if doc.PatternName != "Per Bagian" || doc.PatternID == "" {
		t.Errorf("pola dokumen %q (%q), mau Per Bagian", doc.PatternName, doc.PatternID)
	}
	// 4 ekor = 4000 gr: 3000 gr @40 + 1000 gr @48 = 168.000 (Rp42/gr).
	if doc.InputQtyBase != 4000 || doc.InputUnit != "ekor" || !bdNear(doc.InputCostTotal, 168000) || !bdNear(doc.InputCostPerBase, 42) {
		t.Errorf("bahan: qty %.2f %s biaya %.2f (%.4f/gr), mau 4000 ekor 168000 (42/gr)",
			doc.InputQtyBase, doc.InputUnit, doc.InputCostTotal, doc.InputCostPerBase)
	}
	want := map[string][2]float64{c.paha: {7000, 56000}, c.dada: {10500, 84000}, c.sayap: {3500, 28000}}
	outs := bdOutputs(doc)
	if len(outs) != 3 {
		t.Fatalf("baris hasil %d, mau 3", len(outs))
	}
	for id, w := range want {
		o := outs[id]
		if o.QtyBase != 8 || o.StdQtyBase != 8 || o.YieldPct != 100 || !bdNear(o.CostPerBase, w[0]) || !bdNear(o.TotalCost, w[1]) {
			t.Errorf("%s: aktual %.2f standar %.2f yield %.2f HPP %.4f total %.2f, mau 8/8/100/%.0f/%.0f",
				o.ItemName, o.QtyBase, o.StdQtyBase, o.YieldPct, o.CostPerBase, o.TotalCost, w[0], w[1])
		}
		if qty, avg := f.ledger(t, id); qty != 8 || !bdNear(avg, w[0]) {
			t.Errorf("%s di gudang: qty %.2f avg %.2f, mau 8 @ %.0f", o.ItemName, qty, avg, w[0])
		}
	}
	if doc.YieldPct != 100 || doc.ShrinkCost != 0 || doc.ExpiryDate != "" {
		t.Errorf("yield dokumen %.2f susut %.2f expiry %q, mau 100 / 0 / kosong", doc.YieldPct, doc.ShrinkCost, doc.ExpiryDate)
	}

	// Stok bahan berkurang 4000 gr; batch @40 habis, sisa satu batch 4000 @48.
	if qty, _ := f.ledger(t, c.whole); qty != 4000 {
		t.Errorf("sisa ayam utuh %.2f gr, mau 4000", qty)
	}
	rows, err := database.DB.Query(`SELECT qty_base, cost_per_base FROM stock_batches WHERE item_id = $1 AND warehouse_id = $2`, c.whole, f.wh)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	var batches [][2]float64
	for rows.Next() {
		var q, cost float64
		rows.Scan(&q, &cost)
		batches = append(batches, [2]float64{q, cost})
	}
	rows.Close()
	if len(batches) != 1 || batches[0] != [2]float64{4000, 48} {
		t.Errorf("batch tersisa %v, mau [[4000 48]]", batches)
	}

	// Movement: bahan keluar breakdown_out (−4000 @42), hasil masuk breakdown_in.
	var outQty, outCost float64
	var refType, refNumber string
	if err := database.DB.QueryRow(`SELECT qty_base, cost_per_base, ref_type, ref_number FROM stock_movements
		WHERE item_id = $1 AND warehouse_id = $2 AND movement_type = 'breakdown_out'`, c.whole, f.wh).Scan(&outQty, &outCost, &refType, &refNumber); err != nil {
		t.Fatalf("movement bahan: %v", err)
	}
	if outQty != -4000 || !bdNear(outCost, 42) || refType != "stock_breakdown" || refNumber != doc.BreakdownNumber {
		t.Errorf("movement bahan: %.2f @%.2f %s %s", outQty, outCost, refType, refNumber)
	}
	var inCnt int
	database.DB.QueryRow(`SELECT COUNT(*) FROM stock_movements WHERE warehouse_id = $1 AND movement_type = 'breakdown_in' AND ref_id = $2`, f.wh, doc.ID).Scan(&inCnt)
	if inCnt != 3 {
		t.Errorf("movement hasil %d, mau 3", inCnt)
	}

	// Riwayat: dokumen muncul dengan barisnya; filter item cocok untuk bahan & hasil.
	list, total, err := ListStockBreakdowns(f.wh, c.paha, nil, 1, 20)
	if err != nil || total != 1 || len(list) != 1 || len(list[0].Outputs) != 3 || list[0].PatternName != "Per Bagian" {
		t.Errorf("riwayat: total %d len %d err %v", total, len(list), err)
	}
}

// Hasil di bawah standar → biaya bahan yang sama terbagi ke qty lebih sedikit:
// HPP per satuan naik, stok mengikuti hasil aktual, susut terlihat di yield.
func TestStockBreakdownBelowStandardRaisesUnitCost(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	f.stockIn(t, c.whole, 2000, 50, "") // 2 ekor @ Rp50/gr = 100.000

	// Standar 2 ekor: 4/4/4. Aktual: paha 3, dada 4, sayap 2.
	// Porsi: 3×1 + 4×1,5 + 2×0,5 = 10 → Rp10.000 per porsi.
	doc, err := f.breakdown(c.whole, "", 2, bdOut{c.paha, 3}, bdOut{c.dada, 4}, bdOut{c.sayap, 2})
	if err != nil {
		t.Fatalf("pemotongan: %v", err)
	}
	outs := bdOutputs(doc)
	for id, w := range map[string][4]float64{ // aktual, standar, yield, HPP
		c.paha: {3, 4, 75, 10000}, c.dada: {4, 4, 100, 15000}, c.sayap: {2, 4, 50, 5000},
	} {
		o := outs[id]
		if o.QtyBase != w[0] || o.StdQtyBase != w[1] || o.YieldPct != w[2] || !bdNear(o.CostPerBase, w[3]) || !bdNear(o.TotalCost, w[0]*w[3]) {
			t.Errorf("%s: aktual %.2f standar %.2f yield %.2f HPP %.4f total %.2f, mau %v",
				o.ItemName, o.QtyBase, o.StdQtyBase, o.YieldPct, o.CostPerBase, o.TotalCost, w)
		}
		if qty, avg := f.ledger(t, id); qty != w[0] || !bdNear(avg, w[3]) {
			t.Errorf("%s di gudang: %.2f @ %.2f, mau %.0f @ %.0f", o.ItemName, qty, avg, w[0], w[3])
		}
	}
	// Pada hasil standar (porsi 12) paha akan ber-HPP 100.000/12 ≈ 8.333,33.
	if std := 100000.0 / 12; outs[c.paha].CostPerBase <= std {
		t.Errorf("HPP paha %.2f tidak naik dari HPP standar %.2f", outs[c.paha].CostPerBase, std)
	}
	if sum := outs[c.paha].TotalCost + outs[c.dada].TotalCost + outs[c.sayap].TotalCost; !bdNear(sum, 100000) {
		t.Errorf("Σ nilai hasil %.2f, mau 100000 (biaya bahan)", sum)
	}
	// Yield dokumen (ditimbang bobot) 10/12 = 83,33%; susut = 100.000 × 2/12.
	if doc.YieldPct != 83.33 || !bdNear(doc.ShrinkCost, 16666.67) {
		t.Errorf("yield dokumen %.2f susut %.2f, mau 83.33 / 16666.67", doc.YieldPct, doc.ShrinkCost)
	}
	if qty, _ := f.ledger(t, c.whole); qty != 0 {
		t.Errorf("sisa ayam utuh %.2f, mau 0", qty)
	}
}

// Stok bahan kurang → galat, tanpa movement, tanpa dokumen.
func TestStockBreakdownInsufficientStock(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	f.stockIn(t, c.whole, 1000, 45, "") // hanya 1 ekor
	before := f.count(t, "stock_movements")

	_, err := f.breakdown(c.whole, "", 2, bdOut{c.paha, 4})
	if err == nil || !strings.Contains(err.Error(), "tidak mencukupi") {
		t.Fatalf("stok kurang: err %v, mau 'tidak mencukupi'", err)
	}
	if after := f.count(t, "stock_movements"); after != before {
		t.Errorf("movement bertambah %d → %d padahal pemotongan gagal", before, after)
	}
	if n := f.count(t, "stock_breakdowns"); n != 0 {
		t.Errorf("dokumen tercatat %d, mau 0", n)
	}
	if qty, _ := f.ledger(t, c.whole); qty != 1000 {
		t.Errorf("stok ayam utuh %.2f, mau tetap 1000", qty)
	}
	if qty, _ := f.ledger(t, c.paha); qty != 0 {
		t.Errorf("stok paha %.2f, mau 0", qty)
	}
}

// Hasil di luar pola, semua hasil 0, hasil negatif, qty bahan 0, dan bahan
// tanpa pola ditolak tanpa menyentuh stok.
func TestStockBreakdownRejectsInvalidOutputs(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	ati := f.item(t, "Ati", "gr", "gr", 1)
	f.stockIn(t, c.whole, 3000, 40, "")
	before := f.count(t, "stock_movements")

	cases := []struct {
		name  string
		input string
		qty   float64
		outs  []bdOut
		want  string
	}{
		{"hasil di luar pola", c.whole, 1, []bdOut{{c.paha, 2}, {ati, 150}}, "tidak ada di pola"},
		{"semua hasil 0", c.whole, 1, []bdOut{{c.paha, 0}}, "minimal satu hasil"},
		{"hasil negatif", c.whole, 1, []bdOut{{c.paha, 2}, {c.dada, -1}}, "negatif"},
		{"qty bahan 0", c.whole, 0, []bdOut{{c.paha, 2}}, "lebih dari 0"},
		{"bahan tanpa pola", c.paha, 1, []bdOut{{c.dada, 1}}, "belum punya pola"},
	}
	for _, tc := range cases {
		if _, err := f.breakdown(tc.input, "", tc.qty, tc.outs...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, mau memuat %q", tc.name, err, tc.want)
		}
	}
	if after := f.count(t, "stock_movements"); after != before {
		t.Errorf("movement bertambah %d → %d dari permintaan yang ditolak", before, after)
	}
	if n := f.count(t, "stock_breakdowns"); n != 0 {
		t.Errorf("dokumen tercatat %d, mau 0", n)
	}
}

// Satu bahan, dua pola: pola wajib dipilih, hanya hasil pola terpilih yang
// dihitung (yield & susut), hasil pola lain ditolak, pola nonaktif tidak
// ditawarkan dan tidak bisa dipakai.
func TestStockBreakdownTwoPatternsOnOneItem(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	potong := f.item(t, "Ayam Potong 8", "pcs", "pcs", 1)
	other := f.item(t, "Bebek Utuh", "gr", "ekor", 1500)
	f.savePatterns(t, other, bdPattern("Per Bagian", bdLine(c.paha, 2, 1)))
	tmpl := f.savePatterns(t, c.whole,
		bdPattern("Per Bagian", bdLine(c.paha, 2, 2), bdLine(c.dada, 2, 3), bdLine(c.sayap, 2, 1)),
		bdPattern("Potong 8", bdLine(potong, 8, 1)))
	if len(tmpl.Patterns) != 2 || tmpl.Patterns[0].Name != "Per Bagian" || tmpl.Patterns[1].Name != "Potong 8" ||
		len(tmpl.Patterns[0].Outputs) != 3 || len(tmpl.Patterns[1].Outputs) != 1 {
		t.Fatalf("pola tersimpan: %+v", tmpl.Patterns)
	}
	perBagian, potong8 := bdPatternID(t, tmpl, "Per Bagian"), bdPatternID(t, tmpl, "Potong 8")
	f.stockIn(t, c.whole, 4000, 40, "")

	if _, err := f.breakdown(c.whole, "", 1, bdOut{potong, 8}); err == nil || !strings.Contains(err.Error(), "pilih pola") {
		t.Errorf("dua pola aktif tanpa pattern_id: err %v, mau 'pilih pola'", err)
	}
	if _, err := f.breakdown(c.whole, potong8, 1, bdOut{c.paha, 2}); err == nil || !strings.Contains(err.Error(), "tidak ada di pola") {
		t.Errorf("hasil pola lain: err %v, mau 'tidak ada di pola'", err)
	}
	otherTmpl, _ := GetStockItemBreakdown(other)
	if _, err := f.breakdown(c.whole, otherTmpl.Patterns[0].ID, 1, bdOut{c.paha, 2}); err == nil || !strings.Contains(err.Error(), "tidak ditemukan") {
		t.Errorf("pola milik bahan lain: err %v, mau 'tidak ditemukan'", err)
	}

	// Pola "Potong 8": 2 ekor (2000 gr @40 = 80.000), standar 16, aktual 14.
	doc, err := f.breakdown(c.whole, potong8, 2, bdOut{potong, 14})
	if err != nil {
		t.Fatalf("pemotongan Potong 8: %v", err)
	}
	if doc.PatternName != "Potong 8" || doc.PatternID != potong8 || len(doc.Outputs) != 1 {
		t.Fatalf("dokumen Potong 8: pola %q, %d baris hasil (pola lain tidak boleh ikut)", doc.PatternName, len(doc.Outputs))
	}
	o := doc.Outputs[0]
	if o.ItemID != potong || o.StdQtyBase != 16 || o.YieldPct != 87.5 || !bdNear(o.TotalCost, 80000) || !bdNear(o.CostPerBase, 80000.0/14) {
		t.Errorf("hasil Potong 8: %+v", o)
	}
	if doc.YieldPct != 87.5 || !bdNear(doc.ShrinkCost, 10000) {
		t.Errorf("yield/susut dokumen %.2f / %.2f, mau 87.5 / 10000", doc.YieldPct, doc.ShrinkCost)
	}

	// Nonaktifkan "Potong 8": tidak ditawarkan, tidak bisa dipakai, dan
	// "Per Bagian" menjadi pilihan tunggal tanpa perlu dipilih.
	off := false
	p2 := bdPattern("Potong 8", bdLine(potong, 8, 1))
	p2.ID, p2.IsActive = potong8, &off
	p1 := bdPattern("Per Bagian", bdLine(c.paha, 2, 2), bdLine(c.dada, 2, 3), bdLine(c.sayap, 2, 1))
	p1.ID = perBagian
	tmpl = f.savePatterns(t, c.whole, p1, p2)
	if bdPatternID(t, tmpl, "Potong 8") != potong8 || tmpl.Patterns[1].IsActive {
		t.Errorf("pola dinonaktifkan harus tetap ber-id sama & nonaktif: %+v", tmpl.Patterns[1])
	}
	all, _ := ListStockItemBreakdowns()
	for _, it := range all {
		if it.InputItemID == c.whole && (len(it.Patterns) != 1 || it.Patterns[0].ID != perBagian) {
			t.Errorf("daftar form memuat pola nonaktif: %+v", it.Patterns)
		}
	}
	if _, err := f.breakdown(c.whole, potong8, 1, bdOut{potong, 8}); err == nil || !strings.Contains(err.Error(), "nonaktif") {
		t.Errorf("pola nonaktif dipakai: err %v, mau 'nonaktif'", err)
	}
	doc, err = f.breakdown(c.whole, "", 1, bdOut{c.paha, 2}, bdOut{c.dada, 2}, bdOut{c.sayap, 2})
	if err != nil || doc.PatternName != "Per Bagian" || len(doc.Outputs) != 3 {
		t.Errorf("satu pola aktif tersisa tanpa pattern_id: err %v doc %+v", err, doc)
	}
}

// Bahan berbasis berat (kg → gram) dengan angka eksak, lalu hasilnya dipotong
// lagi lewat pola sendiri (bertingkat: daging sapi → has dalam → steak).
func TestStockBreakdownWeightBasedChain(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	sapi := f.item(t, "Daging Sapi", "gr", "kg", 1000)
	hasDalam := f.item(t, "Has Dalam", "gr", "kg", 1000)
	hasLuar := f.item(t, "Has Luar", "gr", "kg", 1000)
	tetelan := f.item(t, "Tetelan", "gr", "gr", 1)
	steak := f.item(t, "Steak 200 g", "pcs", "pcs", 1)
	// Per 1 kg: has dalam 300 g (bobot 3), has luar 400 g (1,5), tetelan 200 g
	// (0,5) — 100 g tulang/lemak tidak menjadi stok.
	f.savePatterns(t, sapi, bdPattern("Bagian", bdLine(hasDalam, 300, 3), bdLine(hasLuar, 400, 1.5), bdLine(tetelan, 200, 0.5)))
	// Hasil pola pertama menjadi bahan pola kedua: per 1 kg has dalam → 5 steak.
	f.savePatterns(t, hasDalam, bdPattern("Steak", bdLine(steak, 5, 1)))
	f.stockIn(t, sapi, 5000, 120, "") // Rp120/gr

	// 2 kg = 2000 gr = Rp240.000. Porsi: 600×3/300 + 800×1,5/400 + 400×0,5/200
	// = 6 + 3 + 1 = 10 → Rp24.000 per porsi.
	doc, err := f.breakdown(sapi, "", 2, bdOut{hasDalam, 600}, bdOut{hasLuar, 800}, bdOut{tetelan, 400})
	if err != nil {
		t.Fatalf("pemotongan daging: %v", err)
	}
	if doc.InputQtyBase != 2000 || doc.InputUnit != "kg" || !bdNear(doc.InputCostTotal, 240000) {
		t.Errorf("bahan daging: %.2f %s Rp%.2f, mau 2000 gr (2 kg) Rp240000", doc.InputQtyBase, doc.InputUnit, doc.InputCostTotal)
	}
	outs := bdOutputs(doc)
	for id, w := range map[string][3]float64{ // standar, HPP per gr, total
		hasDalam: {600, 240, 144000}, hasLuar: {800, 90, 72000}, tetelan: {400, 60, 24000},
	} {
		o := outs[id]
		if o.StdQtyBase != w[0] || o.YieldPct != 100 || !bdNear(o.CostPerBase, w[1]) || !bdNear(o.TotalCost, w[2]) {
			t.Errorf("%s: standar %.2f yield %.2f HPP %.4f total %.2f, mau %v", o.ItemName, o.StdQtyBase, o.YieldPct, o.CostPerBase, o.TotalCost, w)
		}
	}
	if qty, avg := f.ledger(t, hasDalam); qty != 600 || !bdNear(avg, 240) {
		t.Errorf("has dalam di gudang %.2f @ %.2f, mau 600 @ 240", qty, avg)
	}

	// Tingkat kedua: 0,6 kg has dalam (600 gr × Rp240 = 144.000) → 3 steak.
	doc2, err := f.breakdown(hasDalam, "", 0.6, bdOut{steak, 3})
	if err != nil {
		t.Fatalf("pemotongan has dalam: %v", err)
	}
	s := doc2.Outputs[0]
	if !bdNear(doc2.InputCostTotal, 144000) || s.StdQtyBase != 3 || s.YieldPct != 100 || !bdNear(s.CostPerBase, 48000) {
		t.Errorf("steak: biaya bahan %.2f, standar %.2f yield %.2f HPP %.2f, mau 144000 / 3 / 100 / 48000",
			doc2.InputCostTotal, s.StdQtyBase, s.YieldPct, s.CostPerBase)
	}
	if qty, avg := f.ledger(t, steak); qty != 3 || !bdNear(avg, 48000) {
		t.Errorf("steak di gudang %.2f @ %.2f, mau 3 @ 48000", qty, avg)
	}
	if qty, _ := f.ledger(t, hasDalam); qty != 0 {
		t.Errorf("sisa has dalam %.2f, mau 0", qty)
	}
}

// Hasil boleh melebihi standar (beras mengembang jadi nasi): yield > 100%,
// HPP per satuan turun, susut negatif.
func TestStockBreakdownYieldAboveStandard(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	beras := f.item(t, "Beras", "gr", "kg", 1000)
	nasi := f.item(t, "Nasi", "gr", "gr", 1)
	f.savePatterns(t, beras, bdPattern("Masak", bdLine(nasi, 2200, 1))) // 1 kg → 2,2 kg nasi
	f.stockIn(t, beras, 2000, 15, "")                                   // Rp30.000

	doc, err := f.breakdown(beras, "", 2, bdOut{nasi, 4800}) // standar 4400 gr
	if err != nil {
		t.Fatalf("masak nasi: %v", err)
	}
	o := doc.Outputs[0]
	if o.StdQtyBase != 4400 || o.YieldPct != 109.09 || !bdNear(o.CostPerBase, 6.25) || !bdNear(o.TotalCost, 30000) {
		t.Errorf("nasi: standar %.2f yield %.2f HPP %.4f total %.2f, mau 4400 / 109.09 / 6.25 / 30000", o.StdQtyBase, o.YieldPct, o.CostPerBase, o.TotalCost)
	}
	if doc.YieldPct != 109.09 || !bdNear(doc.ShrinkCost, -2727.27) {
		t.Errorf("yield/susut dokumen %.2f / %.2f, mau 109.09 / -2727.27", doc.YieldPct, doc.ShrinkCost)
	}
	if qty, avg := f.ledger(t, nasi); qty != 4800 || !bdNear(avg, 6.25) {
		t.Errorf("nasi di gudang %.2f @ %.4f, mau 4800 @ 6.25", qty, avg)
	}
}

// Riwayat menyimpan nama pola saat dicatat: tidak ikut berubah saat pola
// diganti namanya, dan tetap ada setelah polanya dihapus.
func TestStockBreakdownHistoryKeepsPatternName(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	f.stockIn(t, c.whole, 3000, 40, "")
	tmpl, _ := GetStockItemBreakdown(c.whole)
	pid := bdPatternID(t, tmpl, "Per Bagian")
	doc, err := f.breakdown(c.whole, "", 1, bdOut{c.paha, 2}, bdOut{c.dada, 2}, bdOut{c.sayap, 2})
	if err != nil {
		t.Fatalf("pemotongan: %v", err)
	}

	renamed := bdPattern("Bagian Lengkap", bdLine(c.paha, 2, 2), bdLine(c.dada, 2, 3), bdLine(c.sayap, 2, 1))
	renamed.ID = pid
	f.savePatterns(t, c.whole, renamed)
	got, _ := GetStockBreakdown(doc.ID)
	if got.PatternName != "Per Bagian" || got.PatternID != pid {
		t.Errorf("setelah ganti nama: pola %q (%q), mau tetap Per Bagian (%s)", got.PatternName, got.PatternID, pid)
	}
	doc2, err := f.breakdown(c.whole, "", 1, bdOut{c.paha, 2})
	if err != nil || doc2.PatternName != "Bagian Lengkap" {
		t.Fatalf("pemotongan sesudah ganti nama: %v (%q)", err, doc2.PatternName)
	}

	// Pola dihapus (simpan tanpa pola itu): tautan putus, nama tetap.
	f.savePatterns(t, c.whole, bdPattern("Potong 4", bdLine(c.paha, 4, 1)))
	list, _, err := ListStockBreakdowns(f.wh, "", nil, 1, 20)
	if err != nil || len(list) != 2 {
		t.Fatalf("riwayat: %v (%d dokumen)", err, len(list))
	}
	names := map[string]string{}
	for _, d := range list {
		names[d.ID] = d.PatternName
		if d.PatternID != "" {
			t.Errorf("dokumen %s masih bertaut ke pola terhapus %q", d.BreakdownNumber, d.PatternID)
		}
	}
	if names[doc.ID] != "Per Bagian" || names[doc2.ID] != "Bagian Lengkap" {
		t.Errorf("nama pola di riwayat setelah pola dihapus: %v", names)
	}
}

// Batch hasil mewarisi kedaluwarsa TERDEKAT dari batch bahan yang benar-benar
// terpotong (FEFO); bahan tanpa kedaluwarsa → hasil tanpa kedaluwarsa.
func TestStockBreakdownInheritsEarliestExpiry(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	c := f.chicken(t)
	f.stockIn(t, c.whole, 1000, 40, "2030-01-25")
	f.stockIn(t, c.whole, 1000, 40, "2030-01-20")
	f.stockIn(t, c.whole, 1000, 40, "")
	expiryOf := func(doc *models.StockBreakdown) []string {
		rows, err := database.DB.Query(`SELECT COALESCE(TO_CHAR(expiry_date, 'YYYY-MM-DD'), '') FROM stock_batches
			WHERE warehouse_id = $1 AND ref_id = $2 ORDER BY item_id`, f.wh, doc.ID)
		if err != nil {
			t.Fatalf("batch hasil: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var e string
			rows.Scan(&e)
			out = append(out, e)
		}
		return out
	}
	allEqual := func(list []string, want string) bool {
		for _, e := range list {
			if e != want {
				return false
			}
		}
		return len(list) > 0
	}

	// 1,5 ekor: batch 20 Jan (1000) + 500 dari batch 25 Jan → terdekat 20 Jan.
	doc, err := f.breakdown(c.whole, "", 1.5, bdOut{c.paha, 3}, bdOut{c.dada, 3})
	if err != nil {
		t.Fatalf("pemotongan 1: %v", err)
	}
	if got := expiryOf(doc); doc.ExpiryDate != "2030-01-20" || !allEqual(got, "2030-01-20") {
		t.Errorf("pemotongan 1: dokumen %q, batch hasil %v, mau 2030-01-20", doc.ExpiryDate, got)
	}
	var movExp string
	database.DB.QueryRow(`SELECT COALESCE(TO_CHAR(expiry_date, 'YYYY-MM-DD'), '') FROM stock_movements
		WHERE ref_id = $1 AND movement_type = 'breakdown_in' LIMIT 1`, doc.ID).Scan(&movExp)
	if movExp != "2030-01-20" {
		t.Errorf("movement hasil ber-expiry %q, mau 2030-01-20", movExp)
	}
	// 1 ekor: sisa 500 batch 25 Jan + 500 batch tanpa expiry → 25 Jan.
	doc, err = f.breakdown(c.whole, "", 1, bdOut{c.sayap, 2})
	if err != nil {
		t.Fatalf("pemotongan 2: %v", err)
	}
	if got := expiryOf(doc); doc.ExpiryDate != "2030-01-25" || !allEqual(got, "2030-01-25") {
		t.Errorf("pemotongan 2: dokumen %q, batch hasil %v, mau 2030-01-25", doc.ExpiryDate, got)
	}
	// 0,5 ekor: hanya batch tanpa expiry → hasil tanpa expiry.
	doc, err = f.breakdown(c.whole, "", 0.5, bdOut{c.paha, 1})
	if err != nil {
		t.Fatalf("pemotongan 3: %v", err)
	}
	if got := expiryOf(doc); doc.ExpiryDate != "" || !allEqual(got, "") {
		t.Errorf("pemotongan 3: dokumen %q, batch hasil %v, mau tanpa kedaluwarsa", doc.ExpiryDate, got)
	}
}

func TestSaveStockItemBreakdownValidation(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	whole := f.item(t, "Ayam Utuh", "gr", "ekor", 1000)
	paha := f.item(t, "Paha", "pcs", "pcs", 1)
	dada := f.item(t, "Dada", "pcs", "pcs", 1)
	weight := func(v float64) *float64 { return &v }
	bad := []struct {
		name     string
		patterns []models.StockBreakdownPatternReq
		want     string
	}{
		{"nama kosong", []models.StockBreakdownPatternReq{bdPattern("  ", bdLine(paha, 2, 1))}, "wajib diisi"},
		{"nama ganda", []models.StockBreakdownPatternReq{bdPattern("Per Bagian", bdLine(paha, 2, 1)), bdPattern("per bagian", bdLine(dada, 2, 1))}, "lebih dari sekali"},
		{"pola tanpa hasil", []models.StockBreakdownPatternReq{bdPattern("Kosong")}, "belum punya hasil"},
		{"bahan = hasil", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(whole, 1, 1))}, "hasil pemotongannya sendiri"},
		{"hasil standar 0", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(paha, 0, 1))}, "hasil standar"},
		{"bobot 0", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(paha, 2, 0))}, "bobot biaya"},
		{"bobot negatif", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(paha, 2, -1))}, "bobot biaya"},
		{"hasil ganda dalam pola", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(paha, 2, 1), bdLine(paha, 1, 1))}, "lebih dari sekali"},
		{"hasil tak dikenal", []models.StockBreakdownPatternReq{bdPattern("A", bdLine(NewULID(), 2, 1))}, "tidak ditemukan"},
		{"id pola asing", []models.StockBreakdownPatternReq{{ID: NewULID(), Name: "A", Outputs: []models.StockItemBreakdownLineReq{bdLine(paha, 2, 1)}}}, "tidak ditemukan"},
	}
	for _, tc := range bad {
		if _, err := SaveStockItemBreakdown(whole, models.StockItemBreakdownRequest{Patterns: tc.patterns}, "uji"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, mau memuat %q", tc.name, err, tc.want)
		}
	}
	// Bobot tidak dikirim = 1; hasil yang sama boleh muncul di dua pola; pola
	// tanpa is_active = aktif.
	tmpl := f.savePatterns(t, whole,
		models.StockBreakdownPatternReq{Name: "Per Bagian", Notes: "untuk menu bakar", Outputs: []models.StockItemBreakdownLineReq{
			{OutputItemID: paha, QtyPerInput: 2}, {OutputItemID: dada, QtyPerInput: 2, CostWeight: weight(1.5)}}},
		bdPattern("Paha Saja", bdLine(paha, 2, 1)))
	if len(tmpl.Patterns) != 2 || !tmpl.Patterns[0].IsActive || tmpl.Patterns[0].Notes != "untuk menu bakar" || tmpl.Patterns[0].UpdatedBy != "gudang_induk" {
		t.Fatalf("pola tersimpan: %+v", tmpl.Patterns)
	}
	for _, o := range tmpl.Patterns[0].Outputs {
		if (o.OutputItemID == paha && o.CostWeight != 1) || (o.OutputItemID == dada && o.CostWeight != 1.5) {
			t.Errorf("bobot %s = %.2f", o.OutputItemName, o.CostWeight)
		}
	}
	if tmpl.InputDistUnit != "ekor" || tmpl.InputDistRatio != 1000 {
		t.Errorf("info bahan: %+v", tmpl)
	}
	// Tukar nama dua pola dalam sekali simpan (indeks unik nama tidak bentrok).
	a, b := tmpl.Patterns[0], tmpl.Patterns[1]
	swap := []models.StockBreakdownPatternReq{
		{ID: a.ID, Name: b.Name, Outputs: []models.StockItemBreakdownLineReq{bdLine(paha, 2, 1)}},
		{ID: b.ID, Name: a.Name, Outputs: []models.StockItemBreakdownLineReq{bdLine(dada, 2, 1)}},
	}
	if tmpl, err := SaveStockItemBreakdown(whole, models.StockItemBreakdownRequest{Patterns: swap}, "uji"); err != nil ||
		bdPatternID(t, tmpl, "Paha Saja") != a.ID || bdPatternID(t, tmpl, "Per Bagian") != b.ID {
		t.Errorf("tukar nama pola: %v", err)
	}
	// Daftar kosong menghapus semua pola.
	if tmpl, err := SaveStockItemBreakdown(whole, models.StockItemBreakdownRequest{}, "gudang_induk"); err != nil || len(tmpl.Patterns) != 0 {
		t.Errorf("hapus semua pola: %v (%d pola)", err, len(tmpl.Patterns))
	}
}
