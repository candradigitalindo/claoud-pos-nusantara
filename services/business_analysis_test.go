package services

// Menjaga janji-janji Analisa Bisnis: angka selisih tidak bergerak hanya
// karena jumlah outlet berubah atau omzet antar outlet timpang; minggu berlibur
// disetarakan sebelum dibandingkan; dan selisih baru divonis kalau konsisten.
import (
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
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
	peer, _ := bizPanelMedian(g)
	if peer == nil {
		return math.NaN()
	}
	return round1(g[self] - *peer)
}

func TestFairness(t *testing.T) {
	// A memegang 70% omzet dan turun 20%, sisanya flat. Rata-rata tertimbang
	// membaca −6,0 karena A menyusun patokannya sendiri; nilai tengah tidak.
	codes := []string{"A", "B", "C"}
	got := rgi([]float64{700, 150, 150}, []float64{560, 150, 150}, codes, "A")
	if got != -20 {
		t.Errorf("outlet dominan: mau -20, dapat %.1f", got)
	}

	// Outlet kecil yang turun 20% harus dapat angka yang SAMA besarnya.
	got = rgi([]float64{700, 150, 150}, []float64{700, 150, 120}, codes, "C")
	if got != -20 {
		t.Errorf("outlet kecil: mau -20, dapat %.1f", got)
	}

	// Outlet yang tidak melakukan apa-apa harus terbaca 0 — bukan +10 seperti
	// pada pembanding tanpa-diri, yang mengangkat outlet flat hanya karena
	// tetangganya jatuh.
	got = rgi([]float64{700, 150, 150}, []float64{560, 150, 150}, codes, "B")
	if got != 0 {
		t.Errorf("outlet flat: mau 0, dapat %.1f", got)
	}
}

// Nilai tengah meredam satu pembanding yang runtuh: outlet yang tidak berubah
// tetap terbaca 0, tidak ikut terangkat.
func TestMedianMeredamPencilan(t *testing.T) {
	for _, n := range []int{3, 4, 6, 10, 20} {
		codes := make([]string, n)
		prev := make([]float64, n)
		cur := make([]float64, n)
		for i := range codes {
			codes[i] = fmt.Sprintf("O%02d", i)
			prev[i], cur[i] = 100, 100
		}
		cur[0] = 20 // satu outlet runtuh 80%
		if got := rgi(prev, cur, codes, codes[n-1]); got != 0 {
			t.Errorf("n=%d: mau 0, dapat %.1f", n, got)
		}
	}
}

// Patokan yang sama untuk semua: dua outlet yang geraknya berbeda 2,5 poin
// harus berbeda 2,5 poin RGI-nya. Pembanding tanpa-diri membuat NHC (−42,0)
// dan SB (−44,5) terbaca +2,5 dan −2,5 — beda 5 poin dari beda 2,5.
func TestPatokanTunggalTanpaLompatan(t *testing.T) {
	g := map[string]float64{"TS": -9.6, "NF": -29.2, "GD": -35.5, "NHC": -42.0,
		"SB": -44.5, "NS": -46.2, "SA": -47.3, "KL": -53.6}
	peer, n := bizPanelMedian(g)
	if peer == nil || n != 8 {
		t.Fatalf("patokan tidak terbentuk: %v %d", peer, n)
	}
	d := round1((g["NHC"] - *peer) - (g["SB"] - *peer))
	if d != 2.5 {
		t.Errorf("beda RGI NHC−SB mau 2,5, dapat %.1f", d)
	}
}

func TestInvarianJumlahOutlet(t *testing.T) {
	// Satu outlet turun 20%, sisanya flat. Angkanya harus sama berapa pun
	// jumlah outletnya.
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
		if got := rgi(prev, cur, codes, codes[n-1]); got != -20 {
			t.Errorf("n=%d: mau -20, dapat %.1f", n, got)
		}
	}
}

func TestInvarianSebaranOmzet(t *testing.T) {
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
		if got := rgi(prev, cur, codes, "A"); got != -20 {
			t.Errorf("sebaran %v: mau -20, dapat %.1f", prev, got)
		}
	}
}

func TestPembandingKurang(t *testing.T) {
	// Dengan dua outlet, hasilnya harus ditahan, bukan dipaksakan.
	g := map[string]float64{"A": -20, "B": 0}
	if peer, cnt := bizPanelMedian(g); peer != nil || cnt != 2 {
		t.Errorf("2 outlet harus ditolak, dapat peer=%v cnt=%d", peer, cnt)
	}
	g["C"] = 0
	if peer, cnt := bizPanelMedian(g); peer == nil || cnt != 3 {
		t.Errorf("3 outlet harus diterima, dapat peer=%v cnt=%d", peer, cnt)
	}
}

// ── Konsistensi & derau ──────────────────────────────────────────────────────

func f(v float64) *float64 { return &v }

func TestKonsistensi(t *testing.T) {
	rgis := []*float64{f(-5), f(-3), f(2), f(-8)}
	if got := bizConsistency(rgis, -1); got != 3 {
		t.Errorf("3 dari 4 searah negatif, dapat %d", got)
	}
	if got := bizConsistency(rgis, 1); got != 1 {
		t.Errorf("1 dari 4 searah positif, dapat %d", got)
	}
	// nil dan nol tidak dihitung searah
	if got := bizConsistency([]*float64{nil, f(0), f(-1)}, -1); got != 1 {
		t.Errorf("nil/nol tidak boleh dihitung, dapat %d", got)
	}
}

func TestDerauGabungan(t *testing.T) {
	// Outlet kecil (80 struk/minggu) mendapat derau lebih besar daripada
	// outlet besar (400 struk/minggu) dengan derau sendiri yang sama.
	c := 330.0
	kecil := bizBlendedSD(20, c, 80)
	besar := bizBlendedSD(20, c, 400)
	if !(kecil > besar) {
		t.Errorf("outlet kecil harus lebih berderau: kecil=%.1f besar=%.1f", kecil, besar)
	}
	// Tanpa model grup, derau sendiri dipakai apa adanya.
	if got := bizBlendedSD(20, 0, 80); got != 20 {
		t.Errorf("tanpa model grup mau 20, dapat %.1f", got)
	}
	// Tanpa derau sendiri, model grup dipakai apa adanya.
	if got := bizBlendedSD(0, c, 100); math.Abs(got-33) > 0.01 {
		t.Errorf("tanpa derau sendiri mau 33,0, dapat %.2f", got)
	}
}

func TestPanjangBlok(t *testing.T) {
	kasus := map[int]int{5: 0, 6: 3, 7: 3, 8: 4, 13: 4, 52: 4}
	for n, want := range kasus {
		if got := bizBlockLen(n, 4); got != want {
			t.Errorf("n=%d: blok mau %d, dapat %d", n, want, got)
		}
	}
	// Blok 2 minggu: cukup 4 minggu riwayat, tidak pernah jatuh ke 3.
	for n, want := range map[int]int{3: 0, 4: 2, 13: 2} {
		if got := bizBlockLen(n, 2); got != want {
			t.Errorf("blok 2, n=%d: mau %d, dapat %d", n, want, got)
		}
	}
	// Syarat konsistensi: 3 dari 4, 2 dari 3, dan kedua minggu pada blok 2.
	for k, want := range map[int]int{4: 3, 3: 2, 2: 2} {
		if got := bizConsistencyNeed(k); got != want {
			t.Errorf("blok %d: syarat mau %d, dapat %d", k, want, got)
		}
	}
}

// ── Kalender ─────────────────────────────────────────────────────────────────

// Tiga outlet, enam minggu biasa berpola sama (akhir pekan ramai), lalu satu
// minggu dengan Senin libur yang menjual seperti hari Minggu.
func kalenderContoh(holidayObs int) (map[string]map[string]bizDaily, *bizCalendar, []string) {
	cal := &bizCalendar{days: map[string][]models.BizCalendarDay{}}
	start, _ := time.Parse("2006-01-02", "2026-06-01") // Senin
	weeks := []string{}
	for w := 0; w < 8; w++ {
		weeks = append(weeks, start.AddDate(0, 0, 7*w).Format("2006-01-02"))
	}
	// Libur di minggu ke-7 (Senin), dan bila diminta, di minggu-minggu lain juga.
	liburSenin := []string{weeks[6]}
	for i := 0; i < holidayObs-1 && i < 6; i++ {
		liburSenin = append(liburSenin, weeks[i])
	}
	for _, d := range liburSenin {
		cal.days[d] = append(cal.days[d], models.BizCalendarDay{Day: d, Kind: BizCalNational, Name: "Libur Uji"})
	}
	pola := [7]float64{20, 20, 20, 20, 30, 60, 120} // Sen..Min

	daily := map[string]map[string]bizDaily{}
	for _, code := range []string{"A", "B", "C"} {
		skala := map[string]float64{"A": 1, "B": 2, "C": 0.5}[code]
		m := map[string]bizDaily{}
		for _, ws := range weeks {
			t, _ := time.Parse("2006-01-02", ws)
			for i := 0; i < 7; i++ {
				day := t.AddDate(0, 0, i).Format("2006-01-02")
				v := pola[i] * skala
				if kind, _, _ := cal.classify(day); kind == bizDayHoliday {
					v = pola[6] * skala // libur menjual seperti hari Minggu
				}
				m[day] = bizDaily{net: v, trx: int(v)}
			}
		}
		daily[code] = m
	}
	return daily, cal, weeks
}

func TestFaktorKalenderMingguLibur(t *testing.T) {
	daily, cal, weeks := kalenderContoh(1)
	// Hanya dua outlet → dua pengamatan hari libur, belum cukup untuk dipelajari
	// → patokan awal: hari libur dianggap seperti hari Minggu.
	delete(daily, "C")
	m := bizBuildCalendarModel(daily, cal)
	if m.holidayEst {
		t.Errorf("dengan %d pengamatan pengali belum boleh dipelajari", m.holidayObs)
	}
	fBiasa, _ := m.weekFactor("A", weeks[2])
	if fBiasa != 1 {
		t.Errorf("minggu biasa: faktor mau 1, dapat %.3f", fBiasa)
	}
	fLibur, note := m.weekFactor("A", weeks[6])
	// bobot biasa 290; minggu libur 290 − 20 + 120 = 390 → 0,744
	if math.Abs(fLibur-0.744) > 0.002 {
		t.Errorf("minggu libur: faktor mau 0,744, dapat %.3f", fLibur)
	}
	if note == "" {
		t.Error("minggu libur harus punya keterangan kalender")
	}
	// Faktor tidak bergantung pada besar outlet.
	fB, _ := m.weekFactor("B", weeks[6])
	if math.Abs(fB-fLibur) > 0.001 {
		t.Errorf("faktor harus sama untuk outlet besar/kecil: %.3f vs %.3f", fB, fLibur)
	}
	// Pangsa akhir pekan dari pola uji: (60+120)/290 = 62,1%.
	if math.Abs(m.dowShare[5]+m.dowShare[6]-62.1) > 0.2 {
		t.Errorf("pangsa akhir pekan mau 62,1, dapat %.1f", m.dowShare[5]+m.dowShare[6])
	}
}

func TestPengaliLiburDipelajari(t *testing.T) {
	daily, cal, weeks := kalenderContoh(3) // 3 Senin libur × 3 outlet = 9 pengamatan
	m := bizBuildCalendarModel(daily, cal)
	if !m.holidayEst {
		t.Fatalf("pengali libur harus dipelajari dari %d pengamatan", m.holidayObs)
	}
	// Libur menjual 120 di hari yang biasanya 20 → pengali 6.
	if math.Abs(m.holidayMult-6) > 0.01 {
		t.Errorf("pengali libur mau 6, dapat %.2f", m.holidayMult)
	}
	fLibur, _ := m.weekFactor("A", weeks[6])
	if math.Abs(fLibur-0.744) > 0.002 {
		t.Errorf("faktor minggu libur mau 0,744, dapat %.3f", fLibur)
	}
	// Penjualan setara kalender minggu libur = minggu biasa.
	var netLibur float64
	t0, _ := time.Parse("2006-01-02", weeks[6])
	for i := 0; i < 7; i++ {
		netLibur += daily["A"][t0.AddDate(0, 0, i).Format("2006-01-02")].net
	}
	if math.Abs(netLibur*fLibur-290) > 1 {
		t.Errorf("setara kalender mau 290, dapat %.1f", netLibur*fLibur)
	}
}

func TestPengaliPerOutletDitarikKeGrup(t *testing.T) {
	// Tanpa pengamatan sendiri → nilai grup apa adanya.
	if got := bizBlendMult(nil, 4); got != 4 {
		t.Errorf("tanpa pengamatan mau 4, dapat %.2f", got)
	}
	// Lima pengamatan sendiri = bobot 50/50 dalam skala log: √(2×8) = 4.
	if got := bizBlendMult([]float64{2, 2, 2, 2, 2}, 8); math.Abs(got-4) > 0.01 {
		t.Errorf("blend 50/50 mau 4, dapat %.2f", got)
	}
	// Banyak pengamatan → mendekati nilai sendiri.
	own := make([]float64, 50)
	for i := range own {
		own[i] = 2
	}
	if got := bizBlendMult(own, 8); got > 2.3 {
		t.Errorf("50 pengamatan harus mendekati nilai sendiri (2), dapat %.2f", got)
	}
}

func TestKlasifikasiHari(t *testing.T) {
	cal := &bizCalendar{days: map[string][]models.BizCalendarDay{
		"2026-08-17": {{Kind: BizCalNational, Name: "HUT RI"}},   // Senin
		"2026-03-21": {{Kind: BizCalNational, Name: "Idul Fitri"}}, // Sabtu → bukan libur hari kerja
		"2026-07-01": {{Kind: BizCalSchool, Name: "Libur sekolah"}},
		"2026-06-01": {{Kind: BizCalNational, Name: "Pancasila"}, {Kind: BizCalSchool, Name: "Libur sekolah"}},
	}}
	if k, _, _ := cal.classify("2026-08-17"); k != bizDayHoliday {
		t.Error("Senin libur nasional harus bizDayHoliday")
	}
	if k, _, _ := cal.classify("2026-03-21"); k != bizDayNormal {
		t.Error("libur yang jatuh Sabtu tidak dikoreksi sebagai libur hari kerja")
	}
	if k, _, _ := cal.classify("2026-07-01"); k != bizDaySchool {
		t.Error("hari libur sekolah harus bizDaySchool")
	}
	if k, _, _ := cal.classify("2026-06-01"); k != bizDayHoliday {
		t.Error("libur nasional menang atas libur sekolah")
	}
	if k, _, _ := cal.classify("2026-09-28"); k != bizDayNormal {
		t.Error("hari tanpa tanda harus biasa")
	}
}

// ── Minggu tepi yang tidak dijalani penuh ────────────────────────────────────
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

	o := mk("2026-07-01", "2026-07-20")
	bizDropPartialEdgeWeeks(o)
	if _, ada := o.weekNet["2026-06-29"]; ada {
		t.Error("minggu pembuka yang parsial masih ikut dihitung")
	}
	if len(o.weekNet) != 2 {
		t.Errorf("sisa minggu = %d, mau 2", len(o.weekNet))
	}

	o = mk("2026-06-29", "2026-07-20")
	bizDropPartialEdgeWeeks(o)
	if len(o.weekNet) != 3 {
		t.Errorf("buka tepat Senin: sisa %d minggu, mau 3", len(o.weekNet))
	}

	o = mk("2026-06-29", "2026-07-15")
	bizDropPartialEdgeWeeks(o)
	if _, ada := o.weekNet["2026-07-13"]; ada {
		t.Error("minggu penutup yang parsial masih ikut dihitung")
	}

	o = mk("2026-06-29", "2026-09-10")
	bizDropPartialEdgeWeeks(o)
	if len(o.weekNet) != 3 {
		t.Errorf("outlet aktif: sisa %d minggu, mau 3", len(o.weekNet))
	}
}

func TestTrimTampilanSandarUlang(t *testing.T) {
	idx := func(v float64) *float64 { return &v }
	out := &models.BusinessAnalysis{
		Group: []models.BizGroupWeek{
			{WeekStart: "2026-07-06", Label: "2026-W28", Index: idx(100)},
			{WeekStart: "2026-07-13", Label: "2026-W29", Index: idx(80)},
			{WeekStart: "2026-07-20", Label: "2026-W30", Index: idx(120)},
			{WeekStart: "2026-07-27", Label: "2026-W31", Index: idx(90)},
		},
		Outlets: []models.BizOutlet{{Code: "A", Weeks: []models.BizWeek{
			{WeekStart: "2026-07-06", Index: idx(100)},
			{WeekStart: "2026-07-13", Index: idx(40)},
			{WeekStart: "2026-07-20", Index: idx(60)},
			{WeekStart: "2026-07-27", Index: idx(45)},
		}}},
		Calendar:    []models.BizCalendarDay{{Day: "2026-07-08"}, {Day: "2026-07-22"}},
		GroupEvents: []string{"2026-W29", "2026-W31"},
		WeeksCount:  4,
	}
	bizTrimDisplay(out, 2)
	if len(out.Group) != 2 || out.Group[0].WeekStart != "2026-07-20" || out.WeeksCount != 2 || out.PeriodFrom != "2026-07-20" {
		t.Fatalf("potongan salah: %+v", out.Group)
	}
	if *out.Group[0].Index != 100 || *out.Group[1].Index != 75 {
		t.Errorf("indeks pasar harus disandarkan ulang ke 100: %v %v", *out.Group[0].Index, *out.Group[1].Index)
	}
	// Outlet ikut diskalakan dengan faktor yang sama (120 → 100), jadi 60 → 50.
	if len(out.Outlets[0].Weeks) != 2 || *out.Outlets[0].Weeks[0].Index != 50 {
		t.Errorf("indeks outlet: %+v", out.Outlets[0].Weeks)
	}
	if len(out.Calendar) != 1 || out.Calendar[0].Day != "2026-07-22" {
		t.Errorf("kalender harus dipotong ke periode tampil: %+v", out.Calendar)
	}
	if len(out.GroupEvents) != 1 || out.GroupEvents[0] != "2026-W31" {
		t.Errorf("minggu peristiwa harus dipotong ke periode tampil: %v", out.GroupEvents)
	}
}

// Kalimat model harus mengikuti angkanya: naik/turun, nyata/tidak, rupiah
// perkiraan, dan libur ke depan ikut disebut.
func TestKalimatModelDinamis(t *testing.T) {
	pf := func(v float64) *float64 { return &v }
	r := &models.BizModelResult{
		Trend: &models.BizTrend{SlopePct: -2.4, Lo: -4.1, Hi: -0.6, Significant: true, Weeks: 13,
			RecentSlopePct: 1.2, RecentSignificant: true, RecentWeeks: 8},
		Forecast: &models.BizForecast{From: "2026-09-28", To: "2026-10-25", TotalRaw: 200e6, TotalLo: 150e6, TotalHi: 260e6, Basis: "tren",
			Weeks: []models.BizForecastWeek{{WeekStart: "2026-09-28"}, {WeekStart: "2026-10-05", Calendar: "Libur: Uji (Sen 5)"}}},
		Dow: &models.BizDowShift{WeekendPct: pf(-30), WeekdayPct: pf(-8), WorstDow: 6, WorstPct: -38, BestDow: 1, BestPct: 5},
		Changepoint: &models.BizChangepoint{WeekStart: "2026-08-31", ShiftPct: -25, BeforeMean: 80e6, AfterMean: 60e6, Significant: true, AfterWeeks: 4},
	}
	got := strings.Join(bizModelStatements("Kala (KL)", r, false, 4), " || ")
	for _, want := range []string{
		"turun 2,4% per minggu", "tren yang nyata", "arahnya berbalik: naik 1,2% per minggu",
		"bergeser turun 25,0%", "Rp 80.000.000", "akhir pekan turun 30,0%", "hari kerja turun 8,0%",
		"paling melemah Minggu", "Rp 150.000.000 sampai Rp 260.000.000", "Libur: Uji",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("kalimat tidak memuat %q:\n%s", want, got)
		}
	}

	// Tanpa tren nyata dan tanpa libur: kalimatnya berbeda.
	r2 := &models.BizModelResult{
		Trend:    &models.BizTrend{SlopePct: 0.3, Lo: -2, Hi: 2.5, Weeks: 13},
		Forecast: &models.BizForecast{From: "2026-09-28", To: "2026-10-25", TotalRaw: 100e6, TotalLo: 90e6, TotalHi: 110e6, Basis: "datar", Weeks: []models.BizForecastWeek{{WeekStart: "2026-09-28"}}},
	}
	got = strings.Join(bizModelStatements("pasar", r2, true, 4), " || ")
	for _, want := range []string{"masih memuat nol", "-2,0% sampai 2,5%", "tidak ada libur", "level empat minggu terakhir"} {
		if !strings.Contains(got, want) {
			t.Errorf("kalimat datar tidak memuat %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "bergeser") || strings.Contains(got, "akhir pekan") {
		t.Errorf("kalimat tidak boleh menyebut hal yang datanya tidak ada:\n%s", got)
	}
}

// Narasi AI hanya boleh mengganti TEKS; angka, vonis, dan bentuk laporan tidak
// tersentuh, dan bentuk yang tidak cocok ditolak supaya templat tetap dipakai.
func TestNarasiAIHanyaMenggantiTeks(t *testing.T) {
	pf := func(v float64) *float64 { return &v }
	rep := &models.BusinessAnalysis{
		Headline: "lama", VerdictText: "lama",
		Insights: []models.BizInsight{{Kind: "MASALAH", Title: "a", Body: "b"}, {Kind: "NETRAL", Title: "c", Body: "d"}},
		Sections: []models.BizSection{{Key: "pasar", Lead: "l", Hint: "h"}, {Key: "istilah", Lead: "l2", Hint: ""}},
		Outlets: []models.BizOutlet{
			{Code: "KL", Diagnosis: models.BizDiagLagging, RGI4: pf(-12.2), Note: "n", Advice: "a", Breakdown: "b", ModelNotes: []string{"m"}},
			{Code: "TS", Diagnosis: models.BizDiagWeak, Note: "n2", Advice: "a2"},
		},
	}
	n := &aiNarrative{Headline: "Baru", VerdictText: "Vonis baru"}
	n.Insights = append(n.Insights, struct {
		Index int    `json:"index"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Index: 1, Title: "C baru", Body: "D baru"})
	n.Sections = append(n.Sections, struct {
		Key  string `json:"key"`
		Lead string `json:"lead"`
		Hint string `json:"hint"`
	}{Key: "istilah", Lead: "Lead baru", Hint: "Hint yang seharusnya diabaikan"})
	n.Outlets = append(n.Outlets, struct {
		Code       string   `json:"code"`
		Note       string   `json:"note"`
		Advice     string   `json:"advice"`
		Breakdown  string   `json:"breakdown"`
		ModelNotes []string `json:"model_notes"`
	}{Code: "KL", Note: "Note baru", Advice: "Advice baru", Breakdown: "", ModelNotes: []string{"M baru"}},
		struct {
			Code       string   `json:"code"`
			Note       string   `json:"note"`
			Advice     string   `json:"advice"`
			Breakdown  string   `json:"breakdown"`
			ModelNotes []string `json:"model_notes"`
		}{Code: "ZZ", Note: "outlet asing"})

	if !bizAIApply(rep, n) {
		t.Fatal("narasi yang cocok harus diterapkan")
	}
	if rep.Headline != "Baru" || rep.VerdictText != "Vonis baru" {
		t.Errorf("judul/vonis tidak terganti: %q %q", rep.Headline, rep.VerdictText)
	}
	if rep.Insights[0].Title != "a" || rep.Insights[1].Title != "C baru" {
		t.Errorf("temuan: %+v", rep.Insights)
	}
	if rep.Sections[1].Lead != "Lead baru" || rep.Sections[1].Hint != "" {
		t.Errorf("bagian tanpa hint tidak boleh diberi hint: %+v", rep.Sections[1])
	}
	kl := rep.Outlets[0]
	if kl.Note != "Note baru" || kl.Advice != "Advice baru" || kl.Breakdown != "b" || kl.ModelNotes[0] != "M baru" {
		t.Errorf("outlet KL: %+v", kl)
	}
	if kl.Diagnosis != models.BizDiagLagging || *kl.RGI4 != -12.2 {
		t.Error("angka/vonis tidak boleh berubah")
	}
	if rep.Outlets[1].Note != "n2" {
		t.Error("outlet yang tidak disebut narasi harus tetap memakai templat")
	}
	if bizAIApply(rep, &aiNarrative{}) {
		t.Error("narasi kosong tidak boleh dianggap diterapkan")
	}
	// Kolom yang dikosongkan pemeriksa membiarkan templatnya.
	rep.Headline = "templat"
	if !bizAIApply(rep, &aiNarrative{VerdictText: "vonis AI"}) || rep.Headline != "templat" || rep.VerdictText != "vonis AI" {
		t.Errorf("headline kosong harus mempertahankan templat: %q / %q", rep.Headline, rep.VerdictText)
	}
	// Skema keluaran harus bisa diserialkan (dikirim ke API sebagai JSON).
	if b, err := json.Marshal(aiNarrativeSchema()); err != nil || !strings.Contains(string(b), "additionalProperties") {
		t.Errorf("skema tidak sah: %v", err)
	}
}

// Pemeriksa fakta: kalimat keliru yang benar-benar dihasilkan model pada
// percobaan pertama (28 Sep 2026) harus ditolak; kalimat benar harus lolos.
func TestPemeriksaFaktaNarasi(t *testing.T) {
	cek := func(text string, facts []string, label string) string {
		raw, _ := json.Marshal(map[string]any{"FAKTA": facts})
		return aiCheck(text, string(raw), aiAllowedNumbers(raw), label)
	}
	tsFacts := []string{
		"Outlet Teman Setia (TS). Kesimpulan halaman: Belum Bisa Dinilai.",
		"Penjualan 4 minggu terakhir lawan 4 minggu sebelumnya, setara kalender: naik 8,9% (angka mentah: turun 9,6%).",
		"Patokan (nilai tengah 8 outlet pembanding) pada periode yang sama: turun 31,1%.",
		"Selisih outlet ini dengan patokan: 40,0 poin (lebih cepat daripada patokan). Batas wajar selisih outlet ini: ±47,0 poin. Selisih itu MASIH DI DALAM batas wajar.",
		"Arah selisih mingguan searah pada 2 dari 4 minggu terakhir; syarat vonis 3 minggu; jadi BELUM konsisten.",
	}
	if w := cek("Pertumbuhan setara kalender 8,9% lebih tinggi daripada rata-rata pasar 23,3%.", tsFacts, "Belum Bisa Dinilai"); w == "" {
		t.Error("angka 23,3 (batas pasar) tidak ada di fakta TS — harus ditolak")
	}
	if w := cek("Selisih 40 poin di bawah batas wajar 47 poin, sehingga selisihnya 7 poin.", tsFacts, "Belum Bisa Dinilai"); w == "" {
		t.Error("'7 poin' hasil hitungan sendiri harus ditolak")
	}
	if w := cek("Selisihnya 40 poin, masih di dalam batas wajar ±47 poin, dan baru searah 2 dari 4 minggu — belum konsisten.", tsFacts, "Belum Bisa Dinilai"); w != "" {
		t.Errorf("kalimat benar ditolak: %s", w)
	}
	if w := cek("Selisihnya konsisten di atas outlet lain.", tsFacts, "Belum Bisa Dinilai"); w == "" {
		t.Error("menyebut konsisten padahal belum — harus ditolak")
	}

	sbFacts := []string{"Selisih outlet ini dengan patokan: 1,0 poin. Batas wajar selisih outlet ini: ±13,0 poin. Selisih itu MASIH DI DALAM batas wajar."}
	if w := cek("Selisih rgi hanya 1 poin di atas batas wajar 13.", sbFacts, "Sama Saja"); w == "" {
		t.Error("'di atas batas' padahal di dalam — harus ditolak")
	}
	if w := cek("Geraknya sama saja, tetapi tetap Perlu Dipantau.", sbFacts, "Sama Saja"); w == "" {
		t.Error("label kesimpulan lain harus ditolak")
	}

	nsDraft := []string{"NS pada minggu 14–20 Sep berada di bawah rentang wajarnya (setara kalender)."}
	if w := cek("Minggu terakhir NS jauh di atas garis kebiasaan.", nsDraft, ""); w == "" {
		t.Error("arah minggu aneh terbalik harus ditolak")
	}

	pasar := []string{"Level bergeser dari Rp 314.915.646 ke Rp 219.770.472; perkiraan Rp 615.338.260 sampai Rp 1.269.934.390, paling mungkin Rp 883.373.464.", "turun 43,3%"}
	for _, ok := range []string{"sekitar Rp 315 juta menjadi Rp 220 juta", "antara Rp 615 juta dan Rp 1,27 miliar", "paling mungkin Rp 883,4 juta", "turun 43%", "Rp 314.900.000"} {
		if w := cek(ok, pasar, ""); w != "" {
			t.Errorf("pembulatan sah %q ditolak: %s", ok, w)
		}
	}
	if w := cek("sampai Rp 1,3 miliar", pasar, ""); w != "" {
		t.Errorf("1,3 miliar adalah pembulatan satu desimal yang sah dari 1,27 miliar: %s", w)
	}
	if w := cek("sampai Rp 1,4 miliar", pasar, ""); w == "" {
		t.Error("angka yang tidak ada di fakta (1,4 miliar) harus ditolak")
	}
	if got := aiFixMonths("28 Sep–25 Oct turun 7,6 %"); got != "28 Sep–25 Okt turun 7,6%" {
		t.Errorf("rapikan: %q", got)
	}

	// Kekeliruan percobaan kedua (penalaran medium, 28 Sep 2026).
	pasarFakta := []string{"Outlet pembanding yang TURUN setara kalender: GD, KL. Yang NAIK setara kalender: TS. Ringkasnya: 2 dari 3 outlet pembanding turun (TS naik).", "turun 32,8%"}
	if w := cek("Semua 8 outlet dengan data lengkap ikut turun.", pasarFakta, ""); w == "" {
		t.Error("'semua outlet turun' padahal TS naik harus ditolak")
	}
	sbFakta := []string{"Patokan (nilai tengah 8 outlet pembanding): turun 31,1%. Searah pada 2 dari 4 minggu terakhir; syarat 3; jadi BELUM konsisten."}
	if w := cek("Patokan penurunan rata-rata 8 outlet pembanding 31,1%.", sbFakta, "Sama Saja"); w == "" {
		t.Error("patokan disebut rata-rata harus ditolak")
	}
	if w := cek("Belum konsisten karena belum mencapai 3 minggu berturut-turut.", sbFakta, "Sama Saja"); w == "" {
		t.Error("'berturut-turut' harus ditolak")
	}
	if w := cek("Kinerjanya sejalan dengan rata-rata outlet lain.", sbFakta, "Sama Saja"); w == "" {
		t.Error("'rata-rata outlet lain' harus ditolak")
	}
	pf := func(v float64) *float64 { return &v }
	ts := &models.BizOutlet{Diagnosis: models.BizDiagWeak, RGI4: pf(40), Threshold: pf(47)}
	if a := bizAlasanVonis(ts, 4); !strings.Contains(a, "BUKAN soal konsistensi") || !strings.Contains(a, "±47,0") {
		t.Errorf("alasan Belum Bisa Dinilai harus menyebut derau, bukan konsistensi: %q", a)
	}
	if w := cek("Selisihnya masih dalam batas wajar.", []string{"Selisih itu MASIH DI DALAM batas wajar."}, ""); w != "" {
		t.Errorf("'masih dalam batas' setara 'masih di dalam batas', tidak boleh ditolak: %s", w)
	}
}

func TestArahOutletPembanding(t *testing.T) {
	pf := func(v float64) *float64 { return &v }
	os := []models.BizOutlet{{Code: "A", InPanel: true, Growth4: pf(-5)}, {Code: "B", InPanel: true, Growth4: pf(-3)}, {Code: "C", InPanel: true, Growth4: pf(8.9)}, {Code: "D", InPanel: false, Growth4: pf(-1)}}
	if got := bizArahOutlet(os); got != "2 dari 3 outlet pembanding turun (C naik)" {
		t.Errorf("dapat %q", got)
	}
	os[2].Growth4 = pf(-1)
	if got := bizArahOutlet(os); got != "semua 3 outlet pembanding turun" {
		t.Errorf("dapat %q", got)
	}
}

// Narasi outlet disimpan per bagian dan tidak bergantung pada panjang gambar:
// rentang 8 dan 12 minggu harus memakai ulang narasi outlet yang sama,
// sementara ringkasan pasar (yang menyebut panjang periode) berbeda.
func TestBagianNarasiDipakaiUlangLintasRentang(t *testing.T) {
	pf := func(v float64) *float64 { return &v }
	mkRep := func(weeks int, from string) *models.BusinessAnalysis {
		return &models.BusinessAnalysis{
			WeeksCount: weeks, PeriodFrom: from, PeriodTo: "2026-09-27", BlockWeeks: 4, Verdict: "NORMAL",
			Headline: "h", VerdictText: "v", GroupGrowth4: pf(-3),
			Sections: []models.BizSection{{Key: "pasar", Lead: fmt.Sprintf("Sepanjang %d minggu", weeks)}},
			Outlets: []models.BizOutlet{{Code: "KL", Name: "Kala", DiagnosisLabel: "Sama Saja", Diagnosis: models.BizDiagOnPace,
				Growth4: pf(-5), RGI4: pf(-2), Threshold: pf(10), Note: "n", Advice: "a"}},
		}
	}
	p12, p8 := bizAIParts(mkRep(12, "2026-07-06")), bizAIParts(mkRep(8, "2026-08-03"))
	byName := func(ps []*aiPart) map[string]string {
		m := map[string]string{}
		for _, p := range ps {
			m[p.name] = p.hash
		}
		return m
	}
	a, b := byName(p12), byName(p8)
	if a["outlet KL"] == "" || a["outlet KL"] != b["outlet KL"] {
		t.Error("narasi outlet harus sama untuk rentang 8 dan 12 minggu")
	}
	if a["ringkasan"] == b["ringkasan"] {
		t.Error("ringkasan pasar menyebut periode, jadi harus berbeda antar rentang")
	}
}

func TestMingguISO(t *testing.T) {
	if got := bizWeekStartOf("2026-09-27"); got != "2026-09-21" { // Minggu → Senin sebelumnya
		t.Errorf("week start mau 2026-09-21, dapat %s", got)
	}
	if got := bizWeekStartOf("2026-09-21"); got != "2026-09-21" {
		t.Errorf("Senin tetap Senin, dapat %s", got)
	}
	if got := bizWeekLabel("2026-09-21"); got != "2026-W39" {
		t.Errorf("label mau 2026-W39, dapat %s", got)
	}
}

// ── Kesimpulan tidak boleh berubah hanya karena rentang minggu digeser ───────
// Bloknya tetap dan menempel di ujung riwayat, jadi angka pasar, panel, dan
// vonis harus IDENTIK untuk semua rentang ≥ 8 minggu.
func TestKesimpulanStabilTerhadapRentangMinggu(t *testing.T) {
	requireDB(t)

	type snap struct {
		panel   int
		verdict string
		market  float64
		lagging string
		leading string
		watch   string
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
			fmt.Sprint(rep.Lagging), fmt.Sprint(rep.Leading), fmt.Sprint(rep.Watch)}
	}

	base := got[12]
	for _, w := range []int{8, 9, 10, 16, 26, 52} {
		if got[w].verdict == "" {
			continue
		}
		s := got[w]
		if s.panel != base.panel {
			t.Errorf("weeks=%d: panel %d outlet, weeks=12 dapat %d", w, s.panel, base.panel)
		}
		if s.market != base.market {
			t.Errorf("weeks=%d: pasar %.1f%%, weeks=12 dapat %.1f%% — blok tetap harus memberi angka yang sama", w, s.market, base.market)
		}
		if s.verdict != base.verdict {
			t.Errorf("weeks=%d: vonis %q, weeks=12 dapat %q", w, s.verdict, base.verdict)
		}
		if s.lagging != base.lagging || s.leading != base.leading || s.watch != base.watch {
			t.Errorf("weeks=%d: tertinggal %s / unggul %s / pantau %s, weeks=12 dapat %s / %s / %s",
				w, s.lagging, s.leading, s.watch, base.lagging, base.leading, base.watch)
		}
	}
	t.Logf("stabil: panel=%d vonis=%s pasar=%.1f%% tertinggal=%s unggul=%s pantau=%s",
		base.panel, base.verdict, base.market, base.lagging, base.leading, base.watch)
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
