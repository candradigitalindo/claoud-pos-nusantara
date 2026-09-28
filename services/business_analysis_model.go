package services

import (
	"bytes"
	"cloud-pos/config"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ── Model statistik: tren, perkiraan, minggu aneh, pola hari ─────────────────
//
// Blok pembanding menjawab "empat minggu ini dibanding empat minggu lalu".
// Ia tidak menjawab "ke mana arahnya", "berapa bulan depan kalau begini
// terus", "minggu mana yang benar-benar aneh", dan "kapan levelnya bergeser".
// Pertanyaan-pertanyaan itu dijawab layanan analitik Python (analytics/) yang
// menerima deret setara kalender dari sini dan mengembalikan angka. Kalimatnya
// tetap dirakit di Go, dari angka itu, mengikuti aturan halaman: tidak ada
// kalimat tetap; kalimat muncul kalau angkanya ada dan berbunyi sesuai angkanya.
//
// Kegagalan layanan tidak menggagalkan laporan — bagian model hanya hilang,
// dan catatan kakinya menyebutkan itu.

var (
	analyticsURL  string
	analyticsHTTP = &http.Client{Timeout: 8 * time.Second}
)

const (
	bizModelHorizon  = 4  // minggu perkiraan ke depan
	bizModelDailyDay = 56 // hari terakhir untuk pola hari (4 mgg vs 4 mgg)
	bizModelMinWeeks = 6
	bizModelRecent   = 8 // "belakangan ini" = 8 minggu
)

func InitAnalytics(cfg *config.Config) {
	analyticsURL = strings.TrimRight(strings.TrimSpace(cfg.AnalyticsURL), "/")
	if analyticsURL == "" {
		log.Printf("[Analitik] ANALYTICS_URL kosong — bagian tren & perkiraan Analisa Bisnis nonaktif")
	}
}

// ── Kontrak dengan layanan Python ────────────────────────────────────────────

type anaWeek struct {
	WeekStart string   `json:"week_start"`
	Adj       *float64 `json:"adj"`
	Raw       *float64 `json:"raw"`
	Factor    float64  `json:"factor"`
}
type anaDay struct {
	D    string  `json:"d"`
	Net  float64 `json:"net"`
	Kind int     `json:"kind"`
}
type anaFuture struct {
	WeekStart string  `json:"week_start"`
	Factor    float64 `json:"factor"`
}
type anaSeries struct {
	Weeks  []anaWeek   `json:"weeks"`
	Daily  []anaDay    `json:"daily"`
	Future []anaFuture `json:"future"`
	// DowDays = panjang jendela pola hari (hari), mengikuti panjang blok.
	DowDays int `json:"dow_days"`
}
type anaRequest struct {
	Group   *anaSeries           `json:"group,omitempty"`
	Outlets map[string]anaSeries `json:"outlets"`
}

type anaTrend struct {
	SlopePct          float64 `json:"slope_pct"`
	Lo                float64 `json:"lo"`
	Hi                float64 `json:"hi"`
	Significant       bool    `json:"significant"`
	Weeks             int     `json:"weeks"`
	RecentSlopePct    float64 `json:"recent_slope_pct"`
	RecentLo          float64 `json:"recent_lo"`
	RecentHi          float64 `json:"recent_hi"`
	RecentSignificant bool    `json:"recent_significant"`
	RecentWeeks       int     `json:"recent_weeks"`
}
type anaForecastWeek struct {
	H         int     `json:"h"`
	Adj       float64 `json:"adj"`
	Raw       float64 `json:"raw"`
	Lo        float64 `json:"lo"`
	Hi        float64 `json:"hi"`
	Factor    float64 `json:"factor"`
	WeekStart string  `json:"week_start"`
}
type anaForecast struct {
	Weeks    []anaForecastWeek `json:"weeks"`
	TotalRaw float64           `json:"total_raw"`
	TotalLo  float64           `json:"total_lo"`
	TotalHi  float64           `json:"total_hi"`
	Basis    string            `json:"basis"`
}
type anaAnomaly struct {
	Index     int     `json:"index"`
	ActualAdj float64 `json:"actual_adj"`
	Lo        float64 `json:"lo"`
	Hi        float64 `json:"hi"`
	Z         float64 `json:"z"`
	WeekStart string  `json:"week_start"`
}
type anaChange struct {
	Index       int     `json:"index"`
	ShiftPct    float64 `json:"shift_pct"`
	BeforeMean  float64 `json:"before_mean"`
	AfterMean   float64 `json:"after_mean"`
	Score       float64 `json:"score"`
	Significant bool    `json:"significant"`
	AfterWeeks  int     `json:"after_weeks"`
	WeekStart   string  `json:"week_start"`
}
type anaDow struct {
	WeekendPct *float64           `json:"weekend_pct"`
	WeekdayPct *float64           `json:"weekday_pct"`
	PerDow     map[string]float64 `json:"per_dow"`
	WorstDow   int                `json:"worst_dow"`
	WorstPct   float64            `json:"worst_pct"`
	BestDow    int                `json:"best_dow"`
	BestPct    float64            `json:"best_pct"`
	NRecent    int                `json:"n_recent"`
	NPrior     int                `json:"n_prior"`
}
type anaResult struct {
	Trend       *anaTrend    `json:"trend"`
	Forecast    *anaForecast `json:"forecast"`
	Anomalies   []anaAnomaly `json:"anomalies"`
	Changepoint *anaChange   `json:"changepoint"`
	Dow         *anaDow      `json:"dow"`
}
type anaResponse struct {
	Engine  string               `json:"engine"`
	Group   *anaResult           `json:"group"`
	Outlets map[string]anaResult `json:"outlets"`
}

// bizModelInput = bahan yang dibutuhkan untuk menyusun permintaan; semuanya
// sudah ada di GetBusinessAnalysis.
type bizModelInput struct {
	outlets   map[string]*bizOutletRaw
	order     []string
	weekList  []string // jendela penuh, urut
	panel     []string
	calModel  *bizCalModel
	cal       *bizCalendar
	cur       time.Time // Senin minggu berjalan
	blockLen  int
	rangeEnd  string
}

// bizAttachModel memanggil layanan analitik dan menempelkan hasilnya.
// Dipanggil paling akhir: sesudah narasi dan medsos, karena ia menambah
// temuan dan bagian; urutan temuan dirapikan sesudahnya oleh pemanggil.
func bizAttachModel(rep *models.BusinessAnalysis, in bizModelInput) {
	if analyticsURL == "" || len(in.weekList) < bizModelMinWeeks {
		return
	}
	req := bizBuildModelRequest(in)
	if len(req.Outlets) == 0 && req.Group == nil {
		return
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, analyticsURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := analyticsHTTP.Do(httpReq)
	if err != nil {
		log.Printf("[Analitik] gagal memanggil layanan: %v", err)
		rep.Model = &models.BizModel{Enabled: false, Error: "layanan analitik tidak terjangkau", Outlets: map[string]*models.BizModelResult{}}
		rep.Notes = append(rep.Notes, "Layanan analitik (tren, perkiraan, pola hari) sedang tidak terjangkau, jadi bagian itu dilewati. Angka blok dan vonis di atas tidak bergantung padanya.")
		return
	}
	defer resp.Body.Close()
	var out anaResponse
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&out) != nil {
		log.Printf("[Analitik] jawaban tidak terbaca (HTTP %d)", resp.StatusCode)
		rep.Model = &models.BizModel{Enabled: false, Error: fmt.Sprintf("jawaban layanan analitik tidak terbaca (HTTP %d)", resp.StatusCode), Outlets: map[string]*models.BizModelResult{}}
		rep.Notes = append(rep.Notes, "Jawaban layanan analitik tidak terbaca, jadi bagian tren dan perkiraan dilewati.")
		return
	}

	labelOf := func(ws string) string { return bizWeekLabel(ws) }
	m := &models.BizModel{Enabled: true, Engine: out.Engine, Outlets: map[string]*models.BizModelResult{}}
	if out.Group != nil {
		m.Group = bizConvertModel(out.Group, labelOf, in.cal)
		m.Group.Notes = bizModelStatements("pasar", m.Group, true, in.blockLen)
	}
	for code, r := range out.Outlets {
		rr := r
		res := bizConvertModel(&rr, labelOf, in.cal)
		name := code
		if o, ok := in.outlets[code]; ok {
			name = o.name
		}
		res.Notes = bizModelStatements(name+" ("+code+")", res, false, in.blockLen)
		m.Outlets[code] = res
	}
	rep.Model = m

	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		if res, ok := m.Outlets[o.Code]; ok {
			o.ModelNotes = res.Notes
			o.Trend = res.Trend
			o.Forecast = res.Forecast
			o.Advice = bizEnrichAdvice(o, res, in.blockLen)
		}
	}

	bizModelSections(rep, m)
	bizModelInsights(rep, m)
	rep.Notes = append(rep.Notes,
		fmt.Sprintf("Tren, perkiraan, minggu aneh, pergeseran level, dan pola hari dihitung layanan analitik pada angka setara kalender: garis tren robust (Theil–Sen) dengan selang 95%%, rentang perkiraan 80%% dari derau robust (MAD) yang melebar seiring jarak minggu, minggu aneh = keluar %s simpangan robust dari garis, pergeseran level = skor beda dua segmen ≥ 3, pola hari = %d minggu terakhir lawan %d minggu sebelumnya pada hari biasa saja. Perkiraan bukan target: ia hanya berkata \"kalau polanya bertahan\".",
			strings.Replace(fmt.Sprintf("%.1f", 2.5), ".", ",", 1), rep.BlockWeeks, rep.BlockWeeks))
	rep.Glossary = append(rep.Glossary,
		models.BizTerm{Term: "Tren (per minggu)",
			Meaning: "Kemiringan garis yang paling pas melewati penjualan mingguan setara kalender, dalam persen per minggu. Garisnya dipilih dengan cara yang tahan terhadap satu-dua minggu ekstrem. \"Nyata\" berarti selang keyakinannya tidak memuat nol.",
			Example: bizModelTrendExample(rep)},
		models.BizTerm{Term: "Perkiraan 4 minggu",
			Meaning: "Penjualan empat minggu ke depan seandainya pola sekarang bertahan, sudah memperhitungkan libur pada minggu-minggu itu. Rentangnya (80%) menunjukkan seberapa goyah polanya — bukan janji, bukan target.",
			Example: ""},
	)
}

// bizBuildModelRequest menyusun deret untuk tiap outlet dan untuk pasar.
func bizBuildModelRequest(in bizModelInput) anaRequest {
	req := anaRequest{Outlets: map[string]anaSeries{}}
	dailyFrom := in.cur.AddDate(0, 0, -bizModelDailyDay).Format("2006-01-02")

	future := make([]string, bizModelHorizon)
	for h := 0; h < bizModelHorizon; h++ {
		future[h] = in.cur.AddDate(0, 0, 7*h).Format("2006-01-02")
	}

	inPanel := map[string]bool{}
	for _, c := range in.panel {
		inPanel[c] = true
	}

	// Per outlet.
	type futW struct {
		f float64
		w float64
	}
	groupFut := make([]futW, bizModelHorizon)
	for _, code := range in.order {
		o := in.outlets[code]
		if len(o.weekAdj) < bizModelMinWeeks {
			continue
		}
		s := anaSeries{DowDays: bizDowDays(in.blockLen)}
		for _, wk := range in.weekList {
			w := anaWeek{WeekStart: wk, Factor: 1}
			if v, ok := o.weekAdj[wk]; ok && v > 0 {
				adj, raw := v, o.weekNet[wk]
				w.Adj, w.Raw, w.Factor = &adj, &raw, o.weekFactor[wk]
			}
			s.Weeks = append(s.Weeks, w)
		}
		for d, v := range o.daily {
			if d < dailyFrom || d > in.rangeEnd {
				continue
			}
			kind, _, _ := in.cal.classify(d)
			s.Daily = append(s.Daily, anaDay{D: d, Net: v.net, Kind: kind})
		}
		sort.Slice(s.Daily, func(i, j int) bool { return s.Daily[i].D < s.Daily[j].D })
		// Bobot outlet untuk faktor pasar ke depan: penjualan setara 4 minggu terakhir.
		var recent float64
		for i := len(in.weekList) - bizModelHorizon; i < len(in.weekList); i++ {
			if i >= 0 {
				recent += o.weekAdj[in.weekList[i]]
			}
		}
		for h, ws := range future {
			f, _ := in.calModel.weekFactor(code, ws)
			s.Future = append(s.Future, anaFuture{WeekStart: ws, Factor: f})
			if inPanel[code] && recent > 0 {
				groupFut[h].f += f * recent
				groupFut[h].w += recent
			}
		}
		req.Outlets[code] = s
	}

	// Pasar: jumlah outlet panel pada minggu yang SEMUA anggota panel punya
	// datanya — kalau tidak, deretnya melompat saat outlet bergabung.
	if len(in.panel) >= bizMinPanel {
		g := anaSeries{DowDays: bizDowDays(in.blockLen)}
		for _, wk := range in.weekList {
			w := anaWeek{WeekStart: wk, Factor: 1}
			var adj, raw float64
			complete := true
			for _, code := range in.panel {
				v, ok := in.outlets[code].weekAdj[wk]
				if !ok || v <= 0 {
					complete = false
					break
				}
				adj += v
				raw += in.outlets[code].weekNet[wk]
			}
			if complete {
				a, r := adj, raw
				w.Adj, w.Raw = &a, &r
				if raw > 0 {
					w.Factor = adj / raw
				}
			}
			g.Weeks = append(g.Weeks, w)
		}
		daySum := map[string]float64{}
		for _, code := range in.panel {
			for d, v := range in.outlets[code].daily {
				if d >= dailyFrom && d <= in.rangeEnd {
					daySum[d] += v.net
				}
			}
		}
		for d, v := range daySum {
			kind, _, _ := in.cal.classify(d)
			g.Daily = append(g.Daily, anaDay{D: d, Net: v, Kind: kind})
		}
		sort.Slice(g.Daily, func(i, j int) bool { return g.Daily[i].D < g.Daily[j].D })
		for h, ws := range future {
			f := 1.0
			if groupFut[h].w > 0 {
				f = groupFut[h].f / groupFut[h].w
			}
			g.Future = append(g.Future, anaFuture{WeekStart: ws, Factor: f})
		}
		req.Group = &g
	}
	return req
}

// bizDowDays = jendela pola hari: panjang blok dalam hari, minimal 14.
func bizDowDays(blockLen int) int {
	if blockLen < 2 {
		return 28
	}
	return blockLen * 7
}

func bizConvertModel(r *anaResult, labelOf func(string) string, cal *bizCalendar) *models.BizModelResult {
	res := &models.BizModelResult{Anomalies: []models.BizAnomaly{}, Notes: []string{}}
	if t := r.Trend; t != nil {
		res.Trend = &models.BizTrend{SlopePct: t.SlopePct, Lo: t.Lo, Hi: t.Hi, Significant: t.Significant, Weeks: t.Weeks,
			RecentSlopePct: t.RecentSlopePct, RecentLo: t.RecentLo, RecentHi: t.RecentHi,
			RecentSignificant: t.RecentSignificant, RecentWeeks: t.RecentWeeks}
	}
	if f := r.Forecast; f != nil && len(f.Weeks) > 0 {
		fc := &models.BizForecast{TotalRaw: f.TotalRaw, TotalLo: f.TotalLo, TotalHi: f.TotalHi, Basis: f.Basis}
		for _, w := range f.Weeks {
			fc.Weeks = append(fc.Weeks, models.BizForecastWeek{WeekStart: w.WeekStart, Label: labelOf(w.WeekStart),
				Adj: w.Adj, Raw: w.Raw, Lo: w.Lo, Hi: w.Hi, Factor: w.Factor, Calendar: bizWeekCalendarNote(cal, w.WeekStart)})
		}
		fc.From = fc.Weeks[0].WeekStart
		fc.To = bizWeekEnd(fc.Weeks[len(fc.Weeks)-1].WeekStart)
		res.Forecast = fc
	}
	for _, a := range r.Anomalies {
		res.Anomalies = append(res.Anomalies, models.BizAnomaly{WeekStart: a.WeekStart, Label: labelOf(a.WeekStart),
			ActualAdj: a.ActualAdj, Lo: a.Lo, Hi: a.Hi, Z: a.Z})
	}
	if c := r.Changepoint; c != nil {
		res.Changepoint = &models.BizChangepoint{WeekStart: c.WeekStart, Label: labelOf(c.WeekStart), ShiftPct: c.ShiftPct,
			BeforeMean: c.BeforeMean, AfterMean: c.AfterMean, Score: c.Score, Significant: c.Significant, AfterWeeks: c.AfterWeeks}
	}
	if d := r.Dow; d != nil {
		res.Dow = &models.BizDowShift{WeekendPct: d.WeekendPct, WeekdayPct: d.WeekdayPct, PerDow: d.PerDow,
			WorstDow: d.WorstDow, WorstPct: d.WorstPct, BestDow: d.BestDow, BestPct: d.BestPct, NRecent: d.NRecent, NPrior: d.NPrior}
	}
	return res
}

// ── Kalimat dinamis ──────────────────────────────────────────────────────────

var bizDowName = [7]string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}

// bizRupiahBulat menulis rupiah yang dibulatkan agar enak dibaca dalam kalimat:
// ratusan juta ke ratusan ribu terdekat, puluhan juta ke puluhan ribu.
func bizRupiahBulat(v float64) string {
	a := math.Abs(v)
	var unit float64 = 1
	switch {
	case a >= 1e9:
		unit = 1e6
	case a >= 1e8:
		unit = 1e5
	case a >= 1e7:
		unit = 1e4
	case a >= 1e6:
		unit = 1e3
	}
	return bizRupiah(math.Round(v/unit) * unit)
}

// bizHariEkstrem menulis "paling melemah Senin (turun 52,6%)" dengan kata yang
// mengikuti tandanya: hari "terkuat" yang sebenarnya masih turun disebut
// "paling bertahan", bukan "paling menguat".
func bizHariEkstrem(worstDow int, worstPct float64, bestDow int, bestPct float64) string {
	w := "paling melemah"
	if worstPct >= 0 {
		w = "paling kecil naiknya"
	}
	b := "paling menguat"
	if bestPct <= 0 {
		b = "paling bertahan"
	}
	return fmt.Sprintf("Hari yang %s %s (%s), %s %s (%s).",
		w, bizDowName[worstDow%7], bizNaikTurun(worstPct), b, bizDowName[bestDow%7], bizNaikTurun(bestPct))
}

// bizPerMinggu: "naik 2,4% per minggu" / "turun 1,1% per minggu".
func bizPerMinggu(pct float64) string {
	return bizNaikTurun(pct) + " per minggu"
}

// bizRentangTgl menulis "28 Sep–25 Okt" dari dua tanggal.
func bizRentangTgl(from, to string) string {
	a, errA := time.Parse("2006-01-02", from)
	b, errB := time.Parse("2006-01-02", to)
	if errA != nil || errB != nil {
		return from + " s/d " + to
	}
	if a.Month() == b.Month() {
		return fmt.Sprintf("%d–%d %s", a.Day(), b.Day(), monthShortID[b.Month()-1])
	}
	return fmt.Sprintf("%d %s–%d %s", a.Day(), monthShortID[a.Month()-1], b.Day(), monthShortID[b.Month()-1])
}

// bizModelStatements merakit kalimat pengamatan dari hasil model. Tiap kalimat
// hanya muncul kalau angkanya ada, dan bunyinya mengikuti angkanya.
func bizModelStatements(subjek string, r *models.BizModelResult, isGroup bool, blockLen int) []string {
	var out []string
	subj := "penjualan " + subjek
	if isGroup {
		subj = "pasar"
	}

	// 1. Tren.
	if t := r.Trend; t != nil {
		switch {
		case t.Significant:
			s := fmt.Sprintf("Di luar libur, %s %s selama %d minggu terakhir (rentang keyakinan %s sampai %s per minggu) — tren yang nyata, bukan derau.",
				subj, bizPerMinggu(t.SlopePct), t.Weeks, bizNum(t.Lo)+"%", bizNum(t.Hi)+"%")
			if t.RecentSignificant && (t.RecentSlopePct > 0) != (t.SlopePct > 0) {
				s += fmt.Sprintf(" Tetapi %d minggu terakhir arahnya berbalik: %s.", t.RecentWeeks, bizPerMinggu(t.RecentSlopePct))
			} else if t.RecentSignificant && math.Abs(t.RecentSlopePct) >= 1.5*math.Abs(t.SlopePct) {
				s += fmt.Sprintf(" %d minggu terakhir makin cepat: %s.", t.RecentWeeks, bizPerMinggu(t.RecentSlopePct))
			}
			out = append(out, s)
		case t.RecentSignificant:
			out = append(out, fmt.Sprintf("Sepanjang %d minggu belum ada tren yang nyata, tetapi %d minggu terakhir %s %s — arah yang baru mulai, dan layak diperhatikan.",
				t.Weeks, t.RecentWeeks, subj, bizPerMinggu(t.RecentSlopePct)))
		default:
			out = append(out, fmt.Sprintf("Di luar libur, garis %s selama %d minggu terakhir miring %s, tetapi selang keyakinannya (%s sampai %s per minggu) masih memuat nol — belum bisa disebut tren, naik-turunnya masih di dalam derau.",
				subj, t.Weeks, bizPerMinggu(t.SlopePct), bizNum(t.Lo)+"%", bizNum(t.Hi)+"%"))
		}
	}

	// 2. Pergeseran level yang masih baru (di dalam 8 minggu terakhir).
	if c := r.Changepoint; c != nil && c.Significant && c.AfterWeeks <= bizModelRecent {
		out = append(out, fmt.Sprintf("Level %s bergeser %s sejak minggu %s: sebelumnya rata-rata %s per minggu, sesudahnya %s (setara kalender). Cari apa yang berubah di sekitar tanggal itu — harga, jam buka, orang, atau pesaing.",
			subj, bizNaikTurun(c.ShiftPct), bizRentangTgl(c.WeekStart, bizWeekEnd(c.WeekStart)),
			bizRupiahBulat(c.BeforeMean), bizRupiahBulat(c.AfterMean)))
	}

	// 3. Minggu aneh yang masih baru.
	for _, a := range r.Anomalies {
		if !bizWithinRecentWeeks(a.WeekStart, bizModelRecent) {
			continue
		}
		arah := "di atas"
		if a.ActualAdj < a.Lo {
			arah = "di bawah"
		}
		out = append(out, fmt.Sprintf("Minggu %s keluar dari rentang wajar model: %s setara kalender, padahal wajarnya %s sampai %s (%s). Minggu seperti ini pantas ditanyakan sebabnya, bukan langsung dijadikan tren.",
			bizRentangTgl(a.WeekStart, bizWeekEnd(a.WeekStart)), bizRupiahBulat(a.ActualAdj), bizRupiahBulat(a.Lo), bizRupiahBulat(a.Hi), arah))
	}

	// 4. Pola hari.
	if d := r.Dow; d != nil && d.WeekendPct != nil && d.WeekdayPct != nil {
		we, wd := *d.WeekendPct, *d.WeekdayPct
		var s string
		switch {
		case math.Abs(we-wd) >= 10 && we < wd:
			s = fmt.Sprintf("Dibanding %d minggu sebelumnya (hari biasa saja), akhir pekan %s sedangkan hari kerja %s: yang berkurang terutama pengunjung akhir pekan.", blockLen, bizNaikTurun(we), bizNaikTurun(wd))
		case math.Abs(we-wd) >= 10 && we > wd:
			s = fmt.Sprintf("Dibanding %d minggu sebelumnya (hari biasa saja), hari kerja %s sedangkan akhir pekan %s: yang berubah terutama hari kerja.", blockLen, bizNaikTurun(wd), bizNaikTurun(we))
		default:
			s = fmt.Sprintf("Dibanding %d minggu sebelumnya (hari biasa saja), akhir pekan %s dan hari kerja %s — bergerak serupa.", blockLen, bizNaikTurun(we), bizNaikTurun(wd))
		}
		if math.Abs(d.WorstPct) >= 20 || math.Abs(d.BestPct) >= 20 {
			s += " " + bizHariEkstrem(d.WorstDow, d.WorstPct, d.BestDow, d.BestPct)
		}
		out = append(out, s)
	}

	// 5. Perkiraan.
	if f := r.Forecast; f != nil && f.TotalRaw > 0 {
		var kal []string
		for _, w := range f.Weeks {
			if w.Calendar != "" {
				kal = append(kal, fmt.Sprintf("%s: %s", bizRentangTgl(w.WeekStart, bizWeekEnd(w.WeekStart)), w.Calendar))
			}
		}
		s := fmt.Sprintf("Kalau polanya bertahan, %s %s diperkirakan %s sampai %s, paling mungkin %s",
			subj, bizRentangTgl(f.From, f.To), bizRupiahBulat(f.TotalLo), bizRupiahBulat(f.TotalHi), bizRupiahBulat(f.TotalRaw))
		if len(kal) > 0 {
			s += " — sudah memperhitungkan kalender ke depan (" + strings.Join(kal, "; ") + ")."
		} else {
			s += "; tidak ada libur pada minggu-minggu itu."
		}
		if f.Basis == "datar" {
			s += " Dasarnya level empat minggu terakhir, karena trennya tidak nyata."
		}
		out = append(out, s)
	}
	return out
}

// bizEnrichAdvice menambahkan langkah yang khas outlet itu dari temuan model:
// hari mana yang melemah, sejak minggu mana levelnya bergeser, dan ke mana
// trennya. Saran umum per vonis tetap dipertahankan sebagai kalimat pembuka.
func bizEnrichAdvice(o *models.BizOutlet, r *models.BizModelResult, blockLen int) string {
	if r == nil || o.Diagnosis == models.BizDiagNoData {
		return o.Advice
	}
	var tambah []string
	if d := r.Dow; d != nil && d.WeekendPct != nil && d.WeekdayPct != nil {
		we, wd := *d.WeekendPct, *d.WeekdayPct
		switch {
		case we-wd <= -10 && we < -5:
			tambah = append(tambah, fmt.Sprintf("Yang melemah di %s adalah akhir pekan (%s, hari kerja %s): utamakan Sabtu–Minggu — jam buka lebih awal, stok menu andalan penuh, dan promo yang memang dipasang di outlet pada akhir pekan.",
				o.Code, bizNaikTurun(we), bizNaikTurun(wd)))
		case wd-we <= -10 && wd < -5:
			tambah = append(tambah, fmt.Sprintf("Yang melemah di %s adalah hari kerja (%s, akhir pekan %s): periksa jam buka Senin–Jumat, menu makan siang, dan paket untuk rombongan kantor atau sekolah.",
				o.Code, bizNaikTurun(wd), bizNaikTurun(we)))
		}
		if d.WorstPct <= -30 {
			tambah = append(tambah, fmt.Sprintf("Hari %s adalah yang paling turun (%s dibanding %d minggu sebelumnya); cek apa yang berbeda pada hari itu — jadwal orang, jam buka, atau acara rutin yang berhenti.",
				bizDowName[d.WorstDow%7], bizNaikTurun(d.WorstPct), blockLen))
		}
	}
	if c := r.Changepoint; c != nil && c.Significant && c.AfterWeeks <= bizModelRecent {
		tambah = append(tambah, fmt.Sprintf("Mulai penelusuran dari minggu %s, saat level penjualan bergeser %s: apa yang berubah tepat di sekitar tanggal itu di outlet ini.",
			bizRentangTgl(c.WeekStart, bizWeekEnd(c.WeekStart)), bizNaikTurun(c.ShiftPct)))
	}
	if t := r.Trend; t != nil && t.RecentSignificant && t.RecentSlopePct <= -1.5 && o.Diagnosis != models.BizDiagLagging {
		tambah = append(tambah, fmt.Sprintf("Angka bloknya masih wajar, tetapi %d minggu terakhir trennya %s — kalau dibiarkan, dalam sebulan selisihnya akan melewati batas wajar.",
			t.RecentWeeks, bizPerMinggu(t.RecentSlopePct)))
	}
	if len(tambah) == 0 {
		return o.Advice
	}
	return strings.TrimSpace(o.Advice + " " + strings.Join(tambah, " "))
}

func bizWithinRecentWeeks(weekStart string, weeks int) bool {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return false
	}
	return time.Since(t) <= time.Duration(weeks*7+7)*24*time.Hour
}

func bizModelTrendExample(rep *models.BusinessAnalysis) string {
	if rep.Model == nil || rep.Model.Group == nil || rep.Model.Group.Trend == nil {
		return ""
	}
	t := rep.Model.Group.Trend
	nyata := "belum nyata"
	if t.Significant {
		nyata = "nyata"
	}
	return fmt.Sprintf("Periode ini pasar %s (%s).", bizPerMinggu(t.SlopePct), nyata)
}

// ── Bagian & temuan dari model ───────────────────────────────────────────────

func bizModelSections(rep *models.BusinessAnalysis, m *models.BizModel) {
	lead := "Arah penjualan di luar libur, perkiraan empat minggu ke depan, dan pergeseran pola hari — dihitung dari angka setara kalender."
	if m.Group != nil && len(m.Group.Notes) > 0 {
		lead = strings.Join(m.Group.Notes, " ")
	}
	rep.Sections = append(rep.Sections, models.BizSection{
		Key:   "model",
		Title: "Tren, Perkiraan, dan Pola Hari",
		Lead:  lead,
		Hint: fmt.Sprintf("Tren = kemiringan garis robust pada penjualan setara kalender; \"nyata\" kalau selang keyakinannya tidak memuat nol. Perkiraan berkata \"kalau polanya bertahan\", bukan target, dan rentangnya 80%%. Pola hari membandingkan %d minggu terakhir dengan %d minggu sebelumnya pada hari biasa saja — akhir pekan yang melemah dan hari kerja yang melemah adalah dua masalah dengan obat yang berbeda.", rep.BlockWeeks, rep.BlockWeeks),
	})
}

func bizModelInsights(rep *models.BusinessAnalysis, m *models.BizModel) {
	add := func(kind, title, body string, outlets []string) {
		rep.Insights = append(rep.Insights, models.BizInsight{Kind: kind, Title: title, Body: body, Outlets: outlets})
	}
	g := m.Group
	if g != nil {
		if t := g.Trend; t != nil {
			slope, sig, weeks := t.SlopePct, t.Significant, t.Weeks
			if !sig && t.RecentSignificant {
				slope, sig, weeks = t.RecentSlopePct, true, t.RecentWeeks
			}
			if sig {
				kind, judul := models.BizInsightWarning, fmt.Sprintf("Pasar bergerak turun %s%% per minggu di luar libur", bizNum(-slope))
				if slope > 0 {
					kind, judul = models.BizInsightChance, fmt.Sprintf("Pasar tumbuh %s%% per minggu di luar libur", bizNum(slope))
				}
				body := fmt.Sprintf("Dihitung dari %d minggu penjualan setara kalender, jadi ini bukan libur dan bukan satu minggu ramai. ", weeks)
				if f := g.Forecast; f != nil && f.TotalRaw > 0 {
					body += fmt.Sprintf("Kalau berlanjut, %s pasar diperkirakan %s sampai %s (paling mungkin %s).",
						bizRentangTgl(f.From, f.To), bizRupiahBulat(f.TotalLo), bizRupiahBulat(f.TotalHi), bizRupiahBulat(f.TotalRaw))
				}
				add(kind, judul, body, nil)
			}
		}
		if c := g.Changepoint; c != nil && c.Significant && c.AfterWeeks <= bizModelRecent {
			add(models.BizInsightNeutral, fmt.Sprintf("Level pasar bergeser %s sejak %s", bizNaikTurun(c.ShiftPct), bizRentangTgl(c.WeekStart, bizWeekEnd(c.WeekStart))),
				fmt.Sprintf("Sebelum minggu itu pasar rata-rata %s per minggu (setara kalender), sesudahnya %s. Pergeseran serentak begini biasanya punya satu sebab yang bisa ditunjuk — cuaca, harga, akses jalan, atau promosi yang berhenti.",
					bizRupiahBulat(c.BeforeMean), bizRupiahBulat(c.AfterMean)), nil)
		}
		if d := g.Dow; d != nil && d.WeekendPct != nil && d.WeekdayPct != nil {
			we, wd := *d.WeekendPct, *d.WeekdayPct
			switch {
			case we-wd <= -10 && we < 0:
				add(models.BizInsightWarning, "Akhir pekan yang melemah, bukan hari kerja",
					fmt.Sprintf("Pada hari biasa, akhir pekan %s dibanding %d minggu sebelumnya, sementara hari kerja %s. Yang berkurang adalah kunjungan akhir pekan — sasaran program Markom yang paling jelas: acara, promo, dan konten yang mengajak datang Sabtu–Minggu.",
						bizNaikTurun(we), rep.BlockWeeks, bizNaikTurun(wd)), nil)
			case wd-we <= -10 && wd < 0:
				add(models.BizInsightWarning, "Hari kerja yang melemah, akhir pekan bertahan",
					fmt.Sprintf("Pada hari biasa, hari kerja %s dibanding %d minggu sebelumnya, sementara akhir pekan %s. Pengunjung hari kerja biasanya orang sekitar dan rombongan kecil — periksa jam buka, menu makan siang, dan paket untuk kantor atau sekolah.",
						bizNaikTurun(wd), rep.BlockWeeks, bizNaikTurun(we)), nil)
			}
		}
	}

	// Outlet yang minggu-minggu terakhirnya keluar dari rentang wajar model —
	// disebut per outlet, dengan minggu dan ARAHNYA, supaya tidak ada pembaca
	// (atau penulis ulang) yang harus menebak apakah itu lonjakan atau jatuh.
	type anehRow struct {
		code, week, arah string
	}
	var anehRows []anehRow
	var aneh []string
	for code, r := range m.Outlets {
		var last *models.BizAnomaly
		for i := range r.Anomalies {
			a := &r.Anomalies[i]
			if bizWithinRecentWeeks(a.WeekStart, 2) && (last == nil || a.WeekStart > last.WeekStart) {
				last = a
			}
		}
		if last != nil {
			arah := "di atas"
			if last.ActualAdj < last.Lo {
				arah = "di bawah"
			}
			anehRows = append(anehRows, anehRow{code, bizRentangTgl(last.WeekStart, bizWeekEnd(last.WeekStart)), arah})
			aneh = append(aneh, code)
		}
	}
	if len(anehRows) > 0 {
		sort.Slice(anehRows, func(i, j int) bool { return anehRows[i].code < anehRows[j].code })
		sort.Strings(aneh)
		var bagian []string
		for _, a := range anehRows {
			bagian = append(bagian, fmt.Sprintf("%s pada minggu %s berada %s rentang wajarnya", a.code, a.week, a.arah))
		}
		add(models.BizInsightWarning, "Ada outlet yang keluar dari rentang wajar model pekan-pekan ini",
			fmt.Sprintf("%s (setara kalender). Bisa sebab sesaat, bisa awal perubahan — tanyakan sebabnya sekarang selagi masih segar; rinciannya di laporan detail outlet.",
				bizKapital(strings.Join(bagian, "; "))), aneh)
	}

	// Outlet dengan tren turun nyata yang belum tertangkap blok (Sama Saja).
	var turun []string
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		r, ok := m.Outlets[o.Code]
		if !ok || r.Trend == nil {
			continue
		}
		t := r.Trend
		if (t.Significant && t.SlopePct <= -1.5 || t.RecentSignificant && t.RecentSlopePct <= -1.5) &&
			o.Diagnosis != models.BizDiagLagging && o.Diagnosis != models.BizDiagWatch {
			turun = append(turun, o.Code)
		}
	}
	if len(turun) > 0 {
		add(models.BizInsightWarning, "Tren turun yang belum terlihat di angka blok",
			fmt.Sprintf("%s punya tren turun yang nyata di luar libur meski selisih bloknya masih di dalam batas wajar. Tren yang pelan tidak menggerakkan vonis, tetapi menumpuk — lihat kalimat trennya di laporan detail.",
				bizDaftar(turun)), turun)
	}
}
