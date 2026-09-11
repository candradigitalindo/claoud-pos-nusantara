package services

// Menjaga satu janji: angka RGI tidak boleh bergerak hanya karena jumlah
// outlet berubah atau karena omzet antar outlet timpang. Sebelum pembanding
// dibuat leave-one-out dan bermedian, outlet beromzet 70% yang turun 20% hanya
// terbaca −6,0 — sistematis lolos dari deteksi.
import (
	"fmt"
	"math"
	"testing"
)

// growth blok tiap outlet dari (prev, cur) omzet
func mk(prev, cur []float64, codes []string) map[string]float64 {
	g := map[string]float64{}
	for i, c := range codes {
		g[c] = round1((cur[i] - prev[i]) / prev[i] * 100)
	}
	return g
}

func rgi(prev, cur []float64, codes []string, self string) float64 {
	g := mk(prev, cur, codes)
	peer, _ := bizPeerMedian(g, self)
	if peer == nil {
		return math.NaN()
	}
	return round1(g[self] - *peer)
}

func TestFairness(t *testing.T) {
	// Kasus B lama: A memegang 70% omzet dan turun 20%, sisanya flat.
	// Rumus lama membaca −6,0 karena A ikut menyusun pembandingnya sendiri.
	codes := []string{"A", "B", "C"}
	got := rgi([]float64{700, 150, 150}, []float64{560, 150, 150}, codes, "A")
	fmt.Printf("outlet dominan (70%% omzet) turun 20%%  -> RGI %.1f (lama: -6,0)\n", got)
	if got != -20 {
		t.Errorf("outlet dominan: mau -20, dapat %.1f", got)
	}

	// Outlet kecil yang turun 20% harus dapat angka yang SAMA besarnya.
	got = rgi([]float64{700, 150, 150}, []float64{700, 150, 120}, codes, "C")
	fmt.Printf("outlet kecil (15%% omzet) turun 20%%    -> RGI %.1f (lama: -17,0)\n", got)
	if got != -20 {
		t.Errorf("outlet kecil: mau -20, dapat %.1f", got)
	}

	// Outlet yang tidak melakukan apa-apa: dengan hanya 2 pembanding, nilai
	// tengah = rata-rata keduanya, jadi keruntuhan satu pembanding memang
	// mengangkat yang lain. Batas panel kecil, bukan cacat rumus.
	got = rgi([]float64{700, 150, 150}, []float64{560, 150, 150}, codes, "B")
	fmt.Printf("outlet flat, panel 3 (2 pembanding)    -> RGI %.1f\n", got)
	if got != 10 {
		t.Errorf("panel 3: mau +10, dapat %.1f", got)
	}
}

// Mulai 4 outlet ke atas, nilai tengah meredam satu pembanding yang runtuh:
// outlet yang tidak berubah tetap terbaca 0, tidak ikut terangkat.
func TestMedianMeredamPencilan(t *testing.T) {
	for _, n := range []int{4, 6, 10, 20} {
		codes := make([]string, n)
		prev := make([]float64, n)
		cur := make([]float64, n)
		for i := range codes {
			codes[i] = fmt.Sprintf("O%02d", i)
			prev[i], cur[i] = 100, 100
		}
		cur[0] = 20 // satu outlet runtuh 80%
		got := rgi(prev, cur, codes, codes[n-1])
		fmt.Printf("n=%2d, satu pembanding runtuh -> RGI outlet flat %.1f\n", n, got)
		if got != 0 {
			t.Errorf("n=%d: mau 0, dapat %.1f", n, got)
		}
	}
}

func TestInvarianJumlahOutlet(t *testing.T) {
	// Satu outlet turun 20%, sisanya flat. Angkanya harus sama berapa pun
	// jumlah outletnya — inilah yang dulu bergeser (−13,3 → −18,3).
	for _, n := range []int{3, 5, 8, 12, 20, 40} {
		codes := make([]string, n)
		prev := make([]float64, n)
		cur := make([]float64, n)
		for i := range codes {
			codes[i] = fmt.Sprintf("O%02d", i)
			prev[i] = 100
			cur[i] = 100
		}
		cur[n-1] = 80
		got := rgi(prev, cur, codes, codes[n-1])
		fmt.Printf("n=%2d outlet -> RGI outlet yang turun %.1f\n", n, got)
		if got != -20 {
			t.Errorf("n=%d: mau -20, dapat %.1f", n, got)
		}
	}
}

func TestInvarianSebaranOmzet(t *testing.T) {
	// Outlet uji selalu turun 20%, sisanya flat, tapi sebaran omzetnya diubah
	// ekstrem. RGI tidak boleh bergerak sedikit pun.
	sebaran := [][]float64{
		{100, 100, 100, 100},
		{10, 100, 500, 5000},
		{5000, 100, 100, 100},
		{1, 1, 1, 100000},
	}
	codes := []string{"A", "B", "C", "D"}
	for _, prev := range sebaran {
		cur := append([]float64(nil), prev...)
		cur[0] = prev[0] * 0.8
		got := rgi(prev, cur, codes, "A")
		fmt.Printf("omzet %v -> RGI %.1f\n", prev, got)
		if got != -20 {
			t.Errorf("sebaran %v: mau -20, dapat %.1f", prev, got)
		}
	}
}

func TestPembandingKurang(t *testing.T) {
	// Dengan hanya satu pembanding, hasilnya harus ditahan, bukan dipaksakan.
	g := map[string]float64{"A": -20, "B": 0}
	if peer, cnt := bizPeerMedian(g, "A"); peer != nil || cnt != 1 {
		t.Errorf("1 pembanding harus ditolak, dapat peer=%v cnt=%d", peer, cnt)
	}
	g["C"] = 0
	if peer, cnt := bizPeerMedian(g, "A"); peer == nil || cnt != 2 {
		t.Errorf("2 pembanding harus diterima, dapat peer=%v cnt=%d", peer, cnt)
	}
}

// ── Minggu tepi yang tidak dijalani penuh ────────────────────────────────────
// Mengunci Masalah 2: minggu pembuka yang cuma sebagian hari dulu dihitung
// sebagai minggu penuh, membuat basis pembanding outlet terlalu rendah.
func TestBuangMingguTepiParsial(t *testing.T) {
	mk := func(first, last string) *bizOutletRaw {
		return &bizOutletRaw{
			code: "X", firstDay: first, lastDay: last,
			weekNet: map[string]float64{
				"2026-06-29": 10, // Sen 29 Jun – Min 5 Jul
				"2026-07-06": 20,
				"2026-07-13": 30,
			},
			weekTrx: map[string]int{"2026-06-29": 1, "2026-07-06": 2, "2026-07-13": 3},
		}
	}

	// Buka Rabu 1 Jul: minggu pembuka hanya 5 hari → dibuang.
	o := mk("2026-07-01", "2026-07-20")
	bizDropPartialEdgeWeeks(o)
	if _, ada := o.weekNet["2026-06-29"]; ada {
		t.Error("minggu pembuka yang parsial masih ikut dihitung")
	}
	if len(o.weekNet) != 2 {
		t.Errorf("sisa minggu = %d, mau 2", len(o.weekNet))
	}

	// Buka tepat hari Senin: minggu itu utuh → dipertahankan.
	o = mk("2026-06-29", "2026-07-20")
	bizDropPartialEdgeWeeks(o)
	if len(o.weekNet) != 3 {
		t.Errorf("buka tepat Senin: sisa %d minggu, mau 3 (tidak ada yang dibuang)", len(o.weekNet))
	}

	// Berhenti Rabu 15 Jul: minggu penutup tidak genap → dibuang.
	o = mk("2026-06-29", "2026-07-15")
	bizDropPartialEdgeWeeks(o)
	if _, ada := o.weekNet["2026-07-13"]; ada {
		t.Error("minggu penutup yang parsial masih ikut dihitung")
	}

	// Masih beroperasi (lastDay di minggu berjalan, di luar rentang analisa):
	// seluruh minggu analisa harus utuh — outlet yang biasa tutup hari Minggu
	// tidak boleh disalahartikan sebagai berhenti beroperasi.
	o = mk("2026-06-29", "2026-09-10")
	bizDropPartialEdgeWeeks(o)
	if len(o.weekNet) != 3 {
		t.Errorf("outlet aktif: sisa %d minggu, mau 3", len(o.weekNet))
	}
}

// ── Kesimpulan tidak boleh berubah hanya karena slider minggu digeser ────────
// Mengunci Masalah 1: dulu 9 → 10 minggu menjatuhkan panel 8 → 6 outlet dan
// membalik gerak pasar dari +24,3% menjadi −17,1%, karena blok terpanjang
// menjangkau minggu-minggu terlama yang paling sedikit outletnya.
func TestKesimpulanStabilTerhadapRentangMinggu(t *testing.T) {
	requireDB(t)

	type snap struct {
		panel   int
		verdict string
		market  float64
		lagging string
	}
	got := map[int]snap{}
	for _, w := range []int{8, 9, 10, 12, 16, 26, 52} {
		rep, err := GetBusinessAnalysis(w)
		if err != nil {
			t.Fatalf("weeks=%d: %v", w, err)
		}
		if rep.Verdict == "DATA_KURANG" {
			t.Skipf("data belum cukup untuk menguji kestabilan (weeks=%d)", w)
		}
		got[w] = snap{len(rep.PanelCodes), rep.Verdict, deref(rep.GroupGrowth4),
			fmt.Sprint(rep.Lagging)}
	}

	base := got[12]
	for _, w := range []int{8, 9, 10, 16, 26, 52} {
		s := got[w]
		if s.panel != base.panel {
			t.Errorf("weeks=%d: panel %d outlet, weeks=12 dapat %d — menambah riwayat tidak boleh menyusutkan panel",
				w, s.panel, base.panel)
		}
		if s.verdict != base.verdict {
			t.Errorf("weeks=%d: vonis %q, weeks=12 dapat %q", w, s.verdict, base.verdict)
		}
		if s.lagging != base.lagging {
			t.Errorf("weeks=%d: tertinggal %s, weeks=12 dapat %s", w, s.lagging, base.lagging)
		}
		// Rentang yang lebih panjang boleh menggeser angka pasar sedikit, tapi
		// tidak boleh berbalik arah.
		if (s.market < 0) != (base.market < 0) {
			t.Errorf("weeks=%d: pasar %.1f%%, weeks=12 dapat %.1f%% — arahnya berbalik",
				w, s.market, base.market)
		}
	}
	t.Logf("stabil: panel=%d vonis=%s pasar=%.1f%% tertinggal=%s",
		base.panel, base.verdict, base.market, base.lagging)
}

// Outlet yang belum punya satu pun minggu penuh tetap harus tampil di tabel —
// penjualannya nyata, hanya belum bisa dinilai.
func TestOutletTanpaMingguPenuhTetapTampil(t *testing.T) {
	requireDB(t)
	rep, err := GetBusinessAnalysis(12)
	if err != nil {
		t.Fatal(err)
	}
	var adaNet float64
	for _, o := range rep.Outlets {
		adaNet += o.Net
		if !o.InPanel && o.Net > 0 && o.Diagnosis != "DATA_KURANG" && o.Diagnosis != "SINYAL_LEMAH" {
			t.Errorf("%s di luar panel tapi divonis %q", o.Code, o.Diagnosis)
		}
	}
	if adaNet <= 0 {
		t.Error("tidak ada outlet yang menampilkan penjualan")
	}
	if rep.PanelShare == nil {
		t.Error("cakupan panel tidak dilaporkan — pembaca tidak tahu angka pasar mewakili berapa")
	}
}
