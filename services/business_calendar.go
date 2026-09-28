package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ── Kalender bisnis & penjualan setara kalender ──────────────────────────────
//
// Bisnis ini hidup dari akhir pekan dan hari libur. Pada data nyata 58% omzet
// mingguan jatuh di Sabtu–Minggu; Senin 17 Agustus (HUT RI) menghasilkan 174
// juta melawan Senin biasa 27 juta; Selasa 25 Agustus (Maulid) 119 juta
// melawan Selasa biasa 24 juta; dan minggu terakhir libur sekolah dua kali
// lipat minggu biasa. Membandingkan September yang kosong libur dengan Agustus
// yang berlibur dua kali karena itu SELALU berbunyi "pasar turun" — dan
// tagihannya jatuh ke Markom atas sesuatu yang dibuat kalender.
//
// Cara koreksinya sengaja sederhana supaya bisa dijelaskan ke manajer:
//
//	bobot hari      = penjualan wajar outlet itu pada hari tersebut
//	                  (nilai tengah hari-hari biasa dengan nama hari sama)
//	                  × pengali libur / libur sekolah bila hari itu bertanda
//	bobot minggu    = jumlah bobot tujuh harinya
//	faktor kalender = bobot minggu biasa ÷ bobot minggu ini
//	setara kalender = penjualan sebenarnya × faktor kalender
//
// Pengalinya DIPELAJARI dari data penjualan sendiri (nilai tengah rasio hari
// bertanda terhadap hari biasanya, digabung seluruh outlet), bukan angka mati.
// Kalau pengamatannya belum ada, dipakai patokan awal yang disebutkan
// terang-terangan: hari libur dianggap seperti hari Minggu, libur sekolah
// tidak dikoreksi. Semua pengali ditampilkan di halaman.
//
// Kalendernya sendiri hidup di tabel business_calendar dan bisa diubah dari
// halaman Analisa Bisnis — karena ia ikut menentukan vonis, ia harus bisa
// diperiksa dan diperbaiki oleh orang yang membaca vonisnya.

const (
	BizCalNational = "libur_nasional"
	BizCalJoint    = "cuti_bersama"
	BizCalSchool   = "libur_sekolah"
	BizCalEvent    = "kejadian"
)

var bizCalKindLabel = map[string]string{
	BizCalNational: "Libur nasional",
	BizCalJoint:    "Cuti bersama",
	BizCalSchool:   "Libur sekolah",
	BizCalEvent:    "Kejadian lokal",
}

// BizCalendarKindLabel = nama jenis untuk dibaca orang.
func BizCalendarKindLabel(kind string) string {
	if l, ok := bizCalKindLabel[kind]; ok {
		return l
	}
	return kind
}

// ── CRUD ────────────────────────────────────────────────────────────────────

// ListBusinessCalendar mengembalikan hari bertanda pada rentang [from, to].
func ListBusinessCalendar(from, to string) ([]models.BizCalendarDay, error) {
	rows, err := database.DB.Query(`
		SELECT to_char(day, 'YYYY-MM-DD'), kind, name, source, updated_by
		FROM business_calendar
		WHERE day >= $1::date AND day <= $2::date
		ORDER BY day, kind`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.BizCalendarDay{}
	for rows.Next() {
		var d models.BizCalendarDay
		if err := rows.Scan(&d.Day, &d.Kind, &d.Name, &d.Source, &d.UpdatedBy); err != nil {
			return nil, err
		}
		d.KindLabel = BizCalendarKindLabel(d.Kind)
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpsertBusinessCalendar menambah atau mengubah satu tanda. Tanda yang disentuh
// tangan selalu bersumber 'manual', supaya terlihat mana yang bawaan dan mana
// yang sudah diperiksa orang.
func UpsertBusinessCalendar(in models.BizCalendarDay, actor string) (*models.BizCalendarDay, error) {
	day := strings.TrimSpace(in.Day)
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return nil, fmt.Errorf("tanggal harus berformat YYYY-MM-DD")
	}
	kind := strings.TrimSpace(in.Kind)
	if _, ok := bizCalKindLabel[kind]; !ok {
		return nil, fmt.Errorf("jenis tidak dikenal: pilih libur_nasional, cuti_bersama, libur_sekolah, atau kejadian")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = BizCalendarKindLabel(kind)
	}
	if len(name) > 150 {
		name = name[:150]
	}
	_, err := database.DB.Exec(`
		INSERT INTO business_calendar (day, kind, name, source, updated_by, updated_at)
		VALUES ($1::date, $2, $3, 'manual', $4, now() AT TIME ZONE 'UTC')
		ON CONFLICT (day, kind) DO UPDATE SET
			name = EXCLUDED.name, source = 'manual', updated_by = EXCLUDED.updated_by,
			updated_at = now() AT TIME ZONE 'UTC'`, day, kind, name, actor)
	if err != nil {
		return nil, err
	}
	return &models.BizCalendarDay{Day: day, Kind: kind, KindLabel: BizCalendarKindLabel(kind),
		Name: name, Source: "manual", UpdatedBy: actor}, nil
}

// DeleteBusinessCalendar menghapus satu tanda pada satu tanggal.
func DeleteBusinessCalendar(day, kind string) error {
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return fmt.Errorf("tanggal harus berformat YYYY-MM-DD")
	}
	res, err := database.DB.Exec(`DELETE FROM business_calendar WHERE day = $1::date AND kind = $2`, day, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("tanda tidak ditemukan")
	}
	return nil
}

// ── Pemuatan untuk analisa ──────────────────────────────────────────────────

type bizCalendar struct {
	days          map[string][]models.BizCalendarDay
	coverageUntil string
}

func bizLoadCalendar(from, to string) (*bizCalendar, error) {
	list, err := ListBusinessCalendar(from, to)
	if err != nil {
		return nil, err
	}
	c := &bizCalendar{days: map[string][]models.BizCalendarDay{}}
	for _, d := range list {
		c.days[d.Day] = append(c.days[d.Day], d)
	}
	database.DB.QueryRow(`SELECT COALESCE(to_char(MAX(day), 'YYYY-MM-DD'), '') FROM business_calendar`).Scan(&c.coverageUntil)
	return c, nil
}

// bizDow: 0 = Senin … 6 = Minggu.
func bizDow(day string) int {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return 0
	}
	return (int(t.Weekday()) + 6) % 7
}

var bizDowShort = [7]string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}

// Jenis hari untuk pembobotan.
const (
	bizDayNormal  = 0
	bizDayHoliday = 1 // libur nasional / cuti bersama yang jatuh Senin–Jumat
	bizDaySchool  = 2 // libur sekolah (tanpa libur nasional)
)

// classify: jenis hari, nama-nama libur nasional/cuti bersama, dan kejadian.
func (c *bizCalendar) classify(day string) (kind int, holidays []string, events []string) {
	if c == nil {
		return bizDayNormal, nil, nil
	}
	dow := bizDow(day)
	school := false
	for _, d := range c.days[day] {
		switch d.Kind {
		case BizCalNational, BizCalJoint:
			holidays = append(holidays, d.Name)
		case BizCalSchool:
			school = true
		case BizCalEvent:
			events = append(events, d.Name)
		}
	}
	switch {
	case len(holidays) > 0 && dow <= 4:
		return bizDayHoliday, holidays, events
	case school:
		return bizDaySchool, holidays, events
	}
	return bizDayNormal, holidays, events
}

// ── Model bobot hari ────────────────────────────────────────────────────────

type bizDaily struct {
	net float64
	trx int
}

type bizCalModel struct {
	cal  *bizCalendar
	base map[string][7]float64 // per outlet: penjualan wajar tiap nama hari

	// Pengali grup (nilai tengah rasio seluruh outlet) …
	holidayMult float64
	holidayObs  int
	holidayEst  bool

	schoolMult [2]float64 // 0 = hari kerja, 1 = akhir pekan
	schoolObs  [2]int
	schoolEst  bool

	// … dan pengali per outlet: rasio outlet itu sendiri ditarik ke nilai grup
	// sebanding jumlah pengamatannya. Kafe keluarga dan wahana ATV tidak
	// merasakan libur sekolah dengan besaran yang sama; satu pengali untuk
	// semua akan menekan minggu libur outlet yang tidak terlalu ramai terlalu
	// dalam, lalu melahirkan lonjakan palsu di minggu berikutnya.
	holidayByCode map[string]float64
	schoolByCode  map[string][2]float64

	dowShare [7]float64
}

const (
	bizCalMinBaseObs    = 2 // hari biasa minimum per nama hari sebelum nilai tengahnya dipakai
	bizCalMinHolidayObs = 3 // hari libur teramati minimum sebelum pengalinya dipelajari
	bizCalMinSchoolObs  = 5
	// bizCalShrinkK = berapa pengamatan outlet sendiri yang setara dengan nilai
	// grup saat keduanya digabung. Lima hari libur sekolah outlet itu → 50/50.
	bizCalShrinkK = 5.0
)

// bizBlendMult menggabungkan rasio outlet sendiri dengan pengali grup dalam
// skala log, berbobot jumlah pengamatan outlet itu.
func bizBlendMult(own []float64, pooled float64) float64 {
	if len(own) == 0 || pooled <= 0 {
		return pooled
	}
	m := math.Min(15, math.Max(0.5, bizMedian(own)))
	w := float64(len(own)) / (float64(len(own)) + bizCalShrinkK)
	return math.Round(math.Exp(w*math.Log(m)+(1-w)*math.Log(pooled))*100) / 100
}

// bizBuildCalendarModel mempelajari bobot hari dari penjualan harian tiap
// outlet (daily: kode → tanggal → angka; hari tutup di dalam masa beroperasi
// sudah diisi nol oleh pemanggil).
func bizBuildCalendarModel(daily map[string]map[string]bizDaily, cal *bizCalendar) *bizCalModel {
	m := &bizCalModel{cal: cal, base: map[string][7]float64{}, schoolMult: [2]float64{1, 1},
		holidayByCode: map[string]float64{}, schoolByCode: map[string][2]float64{}}

	codes := make([]string, 0, len(daily))
	for c := range daily {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	// 1. Penjualan wajar tiap nama hari, per outlet, dari hari biasa saja.
	type acc struct {
		normal [7][]float64
		all    []float64
	}
	accs := map[string]*acc{}
	var groupNormal [7][]float64
	for _, code := range codes {
		a := &acc{}
		for day, v := range daily[code] {
			kind, _, _ := cal.classify(day)
			a.all = append(a.all, v.net)
			if kind == bizDayNormal {
				a.normal[bizDow(day)] = append(a.normal[bizDow(day)], v.net)
			}
		}
		accs[code] = a
	}
	// Pola grup (pangsa tiap nama hari) untuk menambal outlet yang hari
	// biasanya belum cukup: dari nilai tengah harian seluruh outlet.
	for _, code := range codes {
		for d := 0; d < 7; d++ {
			if len(accs[code].normal[d]) >= bizCalMinBaseObs {
				groupNormal[d] = append(groupNormal[d], bizMedian(accs[code].normal[d]))
			}
		}
	}
	var groupBase [7]float64
	var groupTotal float64
	for d := 0; d < 7; d++ {
		if len(groupNormal[d]) > 0 {
			groupBase[d] = 0
			for _, v := range groupNormal[d] {
				groupBase[d] += v
			}
			groupTotal += groupBase[d]
		}
	}
	if groupTotal > 0 {
		for d := 0; d < 7; d++ {
			m.dowShare[d] = math.Round(groupBase[d]/groupTotal*1000) / 10
		}
	}

	for _, code := range codes {
		a := accs[code]
		var base [7]float64
		var known [7]bool
		var sumKnown, shareKnown float64
		for d := 0; d < 7; d++ {
			if len(a.normal[d]) >= bizCalMinBaseObs {
				base[d] = bizMedian(a.normal[d])
				known[d] = true
				sumKnown += base[d]
				if groupTotal > 0 {
					shareKnown += groupBase[d] / groupTotal
				}
			}
		}
		// Nama hari yang belum punya cukup hari biasa ditambal dari pola grup,
		// diskalakan ke besar outlet ini.
		var scale float64
		switch {
		case shareKnown > 0:
			scale = sumKnown / shareKnown
		case len(a.all) > 0 && groupTotal > 0:
			var s float64
			for _, v := range a.all {
				s += v
			}
			scale = s / float64(len(a.all)) * 7
		}
		for d := 0; d < 7; d++ {
			if !known[d] && groupTotal > 0 {
				base[d] = scale * groupBase[d] / groupTotal
			}
		}
		m.base[code] = base
	}

	// 2. Pengali libur & libur sekolah: nilai grup dari seluruh outlet, lalu
	//    nilai per outlet yang ditarik ke nilai grup.
	var holRatio []float64
	var schRatio [2][]float64
	holOwn := map[string][]float64{}
	schOwn := map[string][2][]float64{}
	for _, code := range codes {
		base := m.base[code]
		var own [2][]float64
		for day, v := range daily[code] {
			kind, _, _ := cal.classify(day)
			if kind == bizDayNormal {
				continue
			}
			d := bizDow(day)
			if base[d] <= 0 {
				continue
			}
			r := v.net / base[d]
			switch kind {
			case bizDayHoliday:
				holRatio = append(holRatio, r)
				holOwn[code] = append(holOwn[code], r)
			case bizDaySchool:
				wk := 0
				if d >= 5 {
					wk = 1
				}
				schRatio[wk] = append(schRatio[wk], r)
				own[wk] = append(own[wk], r)
			}
		}
		schOwn[code] = own
	}
	m.holidayObs = len(holRatio)
	if m.holidayObs >= bizCalMinHolidayObs {
		m.holidayMult = math.Round(math.Min(12, math.Max(1, bizMedian(holRatio)))*100) / 100
		m.holidayEst = true
	}
	for wk := 0; wk < 2; wk++ {
		m.schoolObs[wk] = len(schRatio[wk])
		if m.schoolObs[wk] >= bizCalMinSchoolObs {
			m.schoolMult[wk] = math.Round(math.Min(6, math.Max(0.7, bizMedian(schRatio[wk])))*100) / 100
			m.schoolEst = true
		}
	}
	// Akhir pekan berlibur sekolah yang pengamatannya kurang memakai pengali
	// hari kerjanya, dan sebaliknya — lebih baik daripada tidak dikoreksi.
	if m.schoolEst {
		if m.schoolObs[0] < bizCalMinSchoolObs {
			m.schoolMult[0] = m.schoolMult[1]
		}
		if m.schoolObs[1] < bizCalMinSchoolObs {
			m.schoolMult[1] = m.schoolMult[0]
		}
	}
	for _, code := range codes {
		if m.holidayEst {
			m.holidayByCode[code] = bizBlendMult(holOwn[code], m.holidayMult)
		}
		if m.schoolEst {
			own := schOwn[code]
			m.schoolByCode[code] = [2]float64{
				bizBlendMult(own[0], m.schoolMult[0]),
				bizBlendMult(own[1], m.schoolMult[1]),
			}
		}
	}
	return m
}

// holidayMultFor / schoolMultFor = pengali yang berlaku untuk satu outlet.
func (m *bizCalModel) holidayMultFor(code string) float64 {
	if v, ok := m.holidayByCode[code]; ok && v > 0 {
		return v
	}
	return m.holidayMult
}

func (m *bizCalModel) schoolMultFor(code string, weekend bool) float64 {
	i := 0
	if weekend {
		i = 1
	}
	if v, ok := m.schoolByCode[code]; ok && v[i] > 0 {
		return v[i]
	}
	return m.schoolMult[i]
}

// dayWeight = penjualan yang wajar diharapkan outlet pada tanggal itu.
func (m *bizCalModel) dayWeight(code, day string) float64 {
	base := m.base[code]
	d := bizDow(day)
	kind, _, _ := m.cal.classify(day)
	switch kind {
	case bizDayHoliday:
		if m.holidayEst {
			return base[d] * m.holidayMultFor(code)
		}
		return base[6] // patokan awal: hari libur seperti hari Minggu
	case bizDaySchool:
		return base[d] * m.schoolMultFor(code, d >= 5)
	}
	return base[d]
}

// weekFactor = bobot minggu biasa ÷ bobot minggu ini, beserta keterangan
// singkat kalendernya ("Libur: HUT RI (Sen 17) · libur sekolah 7 hr").
func (m *bizCalModel) weekFactor(code, weekStart string) (float64, string) {
	base, ok := m.base[code]
	if !ok {
		return 1, m.weekNote(weekStart)
	}
	var normal, actual float64
	for d := 0; d < 7; d++ {
		normal += base[d]
	}
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil || normal <= 0 {
		return 1, ""
	}
	for i := 0; i < 7; i++ {
		actual += m.dayWeight(code, t.AddDate(0, 0, i).Format("2006-01-02"))
	}
	if actual <= 0 {
		return 1, m.weekNote(weekStart)
	}
	return math.Round(normal/actual*1000) / 1000, m.weekNote(weekStart)
}

// weekNote merangkum tanda kalender satu minggu untuk pembaca.
func (m *bizCalModel) weekNote(weekStart string) string {
	return bizWeekCalendarNote(m.cal, weekStart)
}

func bizWeekCalendarNote(cal *bizCalendar, weekStart string) string {
	if cal == nil {
		return ""
	}
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return ""
	}
	var libur, kejadian []string
	school := 0
	for i := 0; i < 7; i++ {
		day := t.AddDate(0, 0, i).Format("2006-01-02")
		kind, hol, ev := cal.classify(day)
		for _, h := range hol {
			libur = append(libur, fmt.Sprintf("%s (%s %d)", h, bizDowShort[bizDow(day)], t.AddDate(0, 0, i).Day()))
		}
		if kind == bizDaySchool {
			school++
		}
		kejadian = append(kejadian, ev...)
	}
	var parts []string
	if len(libur) > 0 {
		parts = append(parts, "Libur: "+strings.Join(libur, ", "))
	}
	if school > 0 {
		parts = append(parts, fmt.Sprintf("libur sekolah %d hr", school))
	}
	if len(kejadian) > 0 {
		parts = append(parts, "Kejadian: "+strings.Join(kejadian, ", "))
	}
	return strings.Join(parts, " · ")
}

// holidayNames = nama libur nasional/cuti bersama pada rentang minggu
// [from, to) dari daftar weekList, untuk kalimat vonis.
func bizHolidayNamesInWeeks(cal *bizCalendar, weeks []string) []string {
	if cal == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, ws := range weeks {
		t, err := time.Parse("2006-01-02", ws)
		if err != nil {
			continue
		}
		for i := 0; i < 7; i++ {
			day := t.AddDate(0, 0, i).Format("2006-01-02")
			kind, hol, _ := cal.classify(day)
			if kind == bizDayHoliday {
				for _, h := range hol {
					if !seen[h] {
						seen[h] = true
						out = append(out, h)
					}
				}
			}
		}
	}
	return out
}

// schoolDaysInWeeks = jumlah hari libur sekolah pada minggu-minggu tersebut.
func bizSchoolDaysInWeeks(cal *bizCalendar, weeks []string) int {
	if cal == nil {
		return 0
	}
	n := 0
	for _, ws := range weeks {
		t, err := time.Parse("2006-01-02", ws)
		if err != nil {
			continue
		}
		for i := 0; i < 7; i++ {
			if kind, _, _ := cal.classify(t.AddDate(0, 0, i).Format("2006-01-02")); kind == bizDaySchool {
				n++
			}
		}
	}
	return n
}

func (m *bizCalModel) export() *models.BizCalendarModel {
	out := &models.BizCalendarModel{
		HolidayMult: m.holidayMult, HolidayObs: m.holidayObs, HolidayEstimated: m.holidayEst,
		SchoolWeekdayMult: m.schoolMult[0], SchoolWeekendMult: m.schoolMult[1],
		SchoolWeekdayObs: m.schoolObs[0], SchoolWeekendObs: m.schoolObs[1], SchoolEstimated: m.schoolEst,
		DowShare: m.dowShare,
	}
	if m.cal != nil {
		out.CoverageUntil = m.cal.coverageUntil
	}
	return out
}
