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

// ── Analisa Bisnis: memisahkan isu outlet dari isu pasar/Markom ──────────────
//
// Halaman ini menjawab satu keputusan: outlet yang dibenahi, atau Markom yang
// digencarkan? Karena itu ada DUA pengukuran yang sengaja dipisah:
//
//	1. Garis pasar — pertumbuhan same-store seluruh panel, ditimbang omzet.
//	   Menjawab "apakah uang di pasar memang sedang bergerak?" Ini dasar
//	   keputusan Markom.
//	2. Pembanding sejawat per outlet — median pertumbuhan outlet LAIN, tiap
//	   outlet satu suara. Menjawab "outlet ini tertinggal dari rekan-rekannya
//	   atau tidak?" Ini dasar keputusan manajer outlet.
//
// Keduanya tidak boleh dicampur. Memakai garis pasar sebagai pembanding outlet
// membuat outlet ikut menyusun angka pembandingnya sendiri: outlet beromzet
// besar praktis dibandingkan dengan dirinya sendiri, dan selisihnya menyusut
// sebesar pangsa omzetnya (outlet 70% omzet yang turun 20% hanya terbaca −6).
//
// Sumber penjualan sengaja dibuat sama dengan Laporan Pendapatan
// (cloud_transactions, mengecualikan order yang di-void) supaya angkanya
// bertemu; laporan yang tidak bertemu memindahkan perdebatan dari performa
// ke pipeline data.
//
// Minggu bisnis = Senin–Minggu, mengikuti zona waktu aplikasi lewat tz_date().
// Minggu berjalan selalu dikecualikan karena belum genap.
//
// Seluruh perbandingan bersifat SAME-STORE: pertumbuhan pada satu pasang
// minggu hanya dihitung dari outlet yang punya penjualan di KEDUA minggu itu.
// Tanpa aturan ini, outlet yang baru online di tengah periode akan terbaca
// sebagai "pasar sedang tumbuh", padahal yang bertambah hanyalah jumlah gerai.

type bizOutletRaw struct {
	id, code, name string
	weekNet        map[string]float64
	weekTrx        map[string]int
	net            float64
	trx            int
	// Rentang beroperasi, dipakai membuang minggu yang tidak dijalani outlet
	// secara utuh. Lihat bizDropPartialEdgeWeeks.
	firstDay, lastDay string // YYYY-MM-DD, kosong bila tak diketahui
	droppedWeeks      int
}

// bizWeekEnd mengembalikan tanggal Minggu (akhir pekan bisnis) dari sebuah
// tanggal Senin, keduanya dalam format YYYY-MM-DD.
func bizWeekEnd(weekStart string) string {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return weekStart
	}
	return t.AddDate(0, 0, 6).Format("2006-01-02")
}

// bizDropPartialEdgeWeeks membuang minggu yang TIDAK dijalani outlet selama
// tujuh hari penuh — yaitu minggu pembuka (outlet baru mulai di tengah minggu)
// dan minggu penutup (outlet berhenti di tengah minggu).
//
// Aturan same-store lama hanya memeriksa "outlet punya penjualan di kedua
// minggu", bukan "outlet berjualan sepanjang minggu". Minggu pembuka yang cuma
// 3 dari 7 hari karena itu ikut terhitung sebagai minggu penuh, membuat basis
// pembandingnya terlalu rendah dan pertumbuhannya tersanjung. Pada data nyata
// selisihnya sampai 13 poin RGI untuk satu outlet dan cukup untuk membalik
// vonis halaman — jadi minggu seperti ini dibuang, bukan diperkirakan isinya.
// Memprorata akan menebak hari yang tidak pernah terjadi; minggu pembuka juga
// biasanya tidak mewakili (ada lonjakan pembukaan atau justru masih sepi).
func bizDropPartialEdgeWeeks(o *bizOutletRaw) {
	if o.firstDay == "" || o.lastDay == "" {
		return
	}
	for wk := range o.weekNet {
		if wk < o.firstDay || bizWeekEnd(wk) > o.lastDay {
			delete(o.weekNet, wk)
			delete(o.weekTrx, wk)
			o.droppedWeeks++
		}
	}
}

const (
	// bizMinPanel = jumlah outlet minimum agar garis pasar berarti. Dengan dua
	// outlet, "pasar" praktis adalah satu outlet lawan satu outlet —
	// menyesatkan, bukan sekadar kurang tepat.
	bizMinPanel = 3

	// bizMinPeers = pembanding minimum DI LUAR outlet yang sedang dinilai.
	// Satu pembanding hanya menghasilkan duel, bukan posisi relatif.
	bizMinPeers = 2

	// bizConfidence = berapa galat baku selisih harus melampaui ambang sebelum
	// boleh disebut nyata. 1,5 dipilih agar cukup peka untuk laporan mingguan
	// tanpa menuduh outlet atas gerakan yang masih dalam kebiasaannya.
	bizConfidence = 1.5

	// Plafon "belum bisa dinilai": outlet dianggap terlalu berderau kalau batas
	// wajarnya jauh lebih lebar daripada batas wajar outlet pada umumnya di
	// grup ini (bizCeilFactor), dengan lantai absolut supaya grup yang sangat
	// stabil tidak menandai outlet yang sebenarnya masih layak dinilai.
	bizCeilFactor = 3.0
	bizCeilFloor  = 12.0
)

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// bizNum memformat angka satu desimal dengan koma, sesuai penulisan angka
// bahasa Indonesia — teks di halaman ini dibaca manajer, bukan mesin.
func bizNum(v float64) string {
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
}

func bizPct(cur, prev float64) *float64 {
	if prev <= 0 {
		return nil
	}
	v := round1((cur - prev) / prev * 100)
	return &v
}

// bizMedian mengembalikan median dari salinan xs (xs tidak diubah).
func bizMedian(xs []float64) float64 {
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	n := len(c)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// bizStdDev = simpangan baku sampel; butuh minimal 3 titik agar berarti.
func bizStdDev(xs []float64) *float64 {
	if len(xs) < 3 {
		return nil
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	v := round1(math.Sqrt(ss / float64(len(xs)-1)))
	return &v
}

// bizRobustSD = simpangan baku yang tahan pencilan (MAD × 1,4826).
//
// Dipakai memakai median, bukan rata-rata: satu minggu ekstrem (libur panjang)
// menggelembungkan simpangan baku yang dipakai untuk mengujinya sendiri,
// sehingga justru lolos tak terdeteksi. Jatuh kembali ke simpangan baku biasa
// kalau sebarannya terlalu pendek atau MAD-nya nol.
func bizRobustSD(xs []float64) *float64 {
	if len(xs) < 4 {
		return bizStdDev(xs)
	}
	med := bizMedian(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - med)
	}
	if mad := bizMedian(dev) * 1.4826; mad > 0 {
		v := round1(mad)
		return &v
	}
	return bizStdDev(xs)
}

// bizPeerMedian = median pertumbuhan outlet LAIN, beserta jumlah pembandingnya.
//
// Dua keputusan yang membuat angka ini adil terhadap besar-kecilnya outlet:
//
//   - Leave-one-out: outlet tidak ikut menyusun angka pembandingnya sendiri.
//     Kalau ikut, selisihnya menyusut sebesar pangsa omzetnya sendiri, jadi
//     outlet besar sistematis lolos dari deteksi dan outlet kecil terlalu
//     mudah tertuduh.
//   - Median, bukan rata-rata tertimbang omzet: tiap outlet satu suara. Dengan
//     bobot omzet, satu outlet raksasa yang menentukan "rata-rata" bagi semua
//     manajer lain.
//
// Hasilnya tidak bergantung pada berapa jumlah outlet maupun sebaran omzetnya.
func bizPeerMedian(growth map[string]float64, self string) (*float64, int) {
	vals := make([]float64, 0, len(growth))
	for code, g := range growth {
		if code != self {
			vals = append(vals, g)
		}
	}
	if len(vals) < bizMinPeers {
		return nil, len(vals)
	}
	v := round1(bizMedian(vals))
	return &v, len(vals)
}

// bizRupiah menulis rupiah bulat dengan pemisah titik — "Rp 12.450.000".
// Laporan detail dibaca manajer outlet, dan rupiah jauh lebih mudah dipegang
// daripada persen, apalagi poin persen.
func bizRupiah(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	d := fmt.Sprintf("%.0f", math.Round(v))
	var b strings.Builder
	for i, c := range d {
		if i > 0 && (len(d)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp " + b.String()
	}
	return "Rp " + b.String()
}

// bizNaikTurun menulis arah gerak dengan kata, bukan tanda plus/minus.
func bizNaikTurun(pct float64) string {
	switch {
	case pct > 0:
		return "naik " + bizNum(pct) + "%"
	case pct < 0:
		return "turun " + bizNum(-pct) + "%"
	default:
		return "tidak berubah"
	}
}

// bizDriver menyimpulkan penggerak perubahan: jumlah pengunjung atau besar
// belanja tiap pengunjung. Dua sebab dengan penanganan yang sama sekali
// berbeda — kalau tidak dipisah, manajer menebak-nebak harus membenahi apa.
func bizDriver(prevNet, recentNet float64, prevTrx, recentTrx int) string {
	if prevTrx <= 0 || recentTrx <= 0 || prevNet <= 0 || recentNet <= 0 {
		return ""
	}
	trxPct := float64(recentTrx-prevTrx) / float64(prevTrx) * 100
	prevATV := prevNet / float64(prevTrx)
	atvPct := (recentNet/float64(recentTrx) - prevATV) / prevATV * 100
	switch {
	case math.Abs(trxPct) > math.Abs(atvPct)*1.5:
		return "PENGUNJUNG"
	case math.Abs(atvPct) > math.Abs(trxPct)*1.5:
		return "BELANJA"
	default:
		return "SEIMBANG"
	}
}

// bizBreakdown menguraikan perubahan penjualan menjadi dua sebab yang bisa
// ditindak: berapa orang yang datang, dan berapa besar belanja tiap orang.
func bizBreakdown(prevNet, recentNet float64, prevTrx, recentTrx int) string {
	if prevTrx <= 0 || recentTrx <= 0 || prevNet <= 0 || recentNet <= 0 {
		return ""
	}
	trxPct := float64(recentTrx-prevTrx) / float64(prevTrx) * 100
	prevATV := prevNet / float64(prevTrx)
	recATV := recentNet / float64(recentTrx)
	atvPct := (recATV - prevATV) / prevATV * 100

	dasar := fmt.Sprintf("Jumlah struk %d menjadi %d (%s). Rata-rata belanja tiap struk %s menjadi %s (%s).",
		prevTrx, recentTrx, bizNaikTurun(round1(trxPct)),
		bizRupiah(prevATV), bizRupiah(recATV), bizNaikTurun(round1(atvPct)))

	switch bizDriver(prevNet, recentNet, prevTrx, recentTrx) {
	case "PENGUNJUNG":
		return dasar + " Yang paling menentukan di sini adalah jumlah orang yang datang, bukan besar belanjanya."
	case "BELANJA":
		return dasar + " Orang yang datang jumlahnya hampir sama; yang berubah adalah besar belanja tiap orang."
	default:
		return dasar + " Keduanya bergerak seimbang."
	}
}

// bizDiagLabel = teks yang dibaca pengguna untuk tiap nilai diagnosis.
func bizDiagLabel(d string) string {
	switch d {
	case models.BizDiagLeading:
		return "Lebih Baik"
	case models.BizDiagLagging:
		return "Tertinggal"
	case models.BizDiagOnPace:
		return "Sama Saja"
	case models.BizDiagWeak:
		return "Belum Bisa Dinilai"
	default:
		return "Data Kurang"
	}
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// bizAdvice menyusun langkah yang disarankan dalam kalimat perintah biasa.
// Sengaja menyebut hal yang bisa dilihat manajer sendiri di outletnya hari itu
// juga, bukan istilah analisa.
func bizAdvice(diagnosis, driver string, marketDown bool) string {
	switch diagnosis {
	case models.BizDiagLagging:
		langkah := "Periksa dulu tiga hal yang paling sering jadi sebab: jam buka dan tutup benar-benar sesuai jadwal, kesiapan stok menu andalan, dan kecepatan melayani saat ramai."
		switch driver {
		case "PENGUNJUNG":
			langkah = "Yang berkurang orang yang datang, jadi periksa hal yang membuat orang mampir: jam buka sesuai jadwal atau tidak, tampilan depan dan papan nama, kebersihan, serta apakah promo yang sedang berjalan benar-benar dipasang di outlet ini."
		case "BELANJA":
			langkah = "Orangnya tetap datang, tapi belanjanya mengecil. Periksa stok menu andalan sering habis atau tidak, apakah kasir menawarkan tambahan, dan apakah paket hemat masih tersedia."
		}
		return langkah + " Bandingkan juga cara kerja outlet yang sedang paling baik, lalu terapkan yang cocok."

	case models.BizDiagLeading:
		return "Outlet ini sedang paling baik. Tanyakan ke manajernya apa yang mereka ubah belakangan — jam sibuk, cara menawarkan, atau susunan menu — lalu terapkan di outlet lain."

	case models.BizDiagWeak:
		return "Penjualan mingguannya terlalu naik-turun untuk dinilai per minggu. Nilai outlet ini per bulan saja, dan pastikan semua transaksi benar-benar tercatat di kasir."

	case models.BizDiagNoData:
		return "Belum ada langkah yang bisa disarankan sampai riwayat penjualannya lebih panjang."

	default: // seirama
		if marketDown {
			return "Tidak ada yang perlu dibenahi di outlet ini — geraknya sama dengan outlet lain. Yang sedang turun pasarnya, jadi tunggu dan dukung program dari Markom."
		}
		return "Outlet ini berjalan normal, sejalan dengan outlet lain. Tidak ada tindakan khusus; teruskan yang sudah berjalan."
	}
}

// GetBusinessAnalysis menghitung tren penjualan mingguan per outlet, garis
// pasar, dan RGI tiap outlet terhadap sejawatnya — lalu menyimpulkan siapa
// yang harus bertindak.
// weeks = jumlah minggu penuh ke belakang yang diminta pengguna.
func GetBusinessAnalysis(weeks int) (*models.BusinessAnalysis, error) {
	if weeks < 6 {
		weeks = 6
	}
	if weeks > 104 {
		weeks = 104
	}

	rows, err := database.DB.Query(`
		WITH b AS (SELECT date_trunc('week', tz_today()::timestamp)::date AS cur)
		SELECT TRIM(o.id), TRIM(o.code), o.name,
		       to_char(date_trunc('week', tz_date(t.created_at)::timestamp), 'YYYY-MM-DD') AS wk,
		       to_char(date_trunc('week', tz_date(t.created_at)::timestamp), 'IYYY-"W"IW') AS label,
		       COALESCE(SUM(t.total_amount), 0), COUNT(*)
		FROM cloud_transactions t
		JOIN outlets o ON o.id = t.outlet_id
		CROSS JOIN b
		WHERE t.created_at >= tz_day_start(b.cur - ($1::int * 7))
		  AND t.created_at <  tz_day_start(b.cur)
		  AND o.is_active = true`+txNotVoided("t")+`
		GROUP BY 1, 2, 3, 4, 5
		ORDER BY 2, 4`, weeks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	outletsByCode := map[string]*bizOutletRaw{}
	var order []string
	weekLabel := map[string]string{}

	for rows.Next() {
		var id, code, name, wk, label string
		var net float64
		var trx int
		if err := rows.Scan(&id, &code, &name, &wk, &label, &net, &trx); err != nil {
			return nil, err
		}
		o, ok := outletsByCode[code]
		if !ok {
			o = &bizOutletRaw{id: id, code: code, name: name,
				weekNet: map[string]float64{}, weekTrx: map[string]int{}}
			outletsByCode[code] = o
			order = append(order, code)
		}
		o.weekNet[wk] = net
		o.weekTrx[wk] = trx
		o.net += net
		o.trx += trx
		weekLabel[wk] = label
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Rentang beroperasi tiap outlet. Sengaja TIDAK dibatasi minggu berjalan:
	// outlet yang masih berjualan hari ini punya lastDay di minggu berjalan,
	// sehingga seluruh minggu yang dianalisa terbaca utuh. Tanpa itu, outlet
	// yang normal tutup hari Minggu akan salah dikira berhenti beroperasi.
	spanRows, err := database.DB.Query(`
		SELECT TRIM(o.code),
		       to_char(MIN(tz_date(t.created_at)), 'YYYY-MM-DD'),
		       to_char(MAX(tz_date(t.created_at)), 'YYYY-MM-DD')
		FROM cloud_transactions t
		JOIN outlets o ON o.id = t.outlet_id
		WHERE o.is_active = true` + txNotVoided("t") + `
		GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer spanRows.Close()
	for spanRows.Next() {
		var code, first, last string
		if err := spanRows.Scan(&code, &first, &last); err != nil {
			return nil, err
		}
		if o, ok := outletsByCode[code]; ok {
			o.firstDay, o.lastDay = first, last
		}
	}
	if err := spanRows.Err(); err != nil {
		return nil, err
	}

	// Buang minggu tepi yang tidak dijalani penuh dari perhitungan.
	//
	// o.net dan o.trx sengaja TIDAK dihitung ulang: itu total penjualan
	// sebenarnya sepanjang periode dan tetap ditampilkan apa adanya di tabel.
	// Outlet yang baru buka pekan ini jadi tetap terlihat beserta uangnya,
	// hanya berdiagnosis "Data Kurang" — menghapusnya dari halaman akan
	// membuat penjualan yang nyata seolah tidak pernah ada.
	var edgeDropped []string
	for _, code := range order {
		o := outletsByCode[code]
		bizDropPartialEdgeWeeks(o)
		if o.droppedWeeks > 0 {
			edgeDropped = append(edgeDropped, o.code)
		}
	}
	sort.Strings(order)

	// Label minggu disaring ke minggu yang masih tersisa; labelnya sendiri tetap
	// dari Postgres (IYYY-IW) agar penomoran ISO tidak dihitung ulang di Go.
	labelAll := weekLabel
	weekLabel = map[string]string{}
	for _, o := range outletsByCode {
		for wk := range o.weekNet {
			weekLabel[wk] = labelAll[wk]
		}
	}

	weekList := make([]string, 0, len(weekLabel))
	for wk := range weekLabel {
		weekList = append(weekList, wk)
	}
	sort.Strings(weekList)

	out := &models.BusinessAnalysis{
		WeeksRequested: weeks,
		WeeksCount:     len(weekList),
		Outlets:        []models.BizOutlet{},
		Group:          []models.BizGroupWeek{},
		Lagging:        []string{},
		Leading:        []string{},
		PanelCodes:     []string{},
		Notes:          []string{},
	}
	if len(weekList) == 0 {
		out.Verdict = models.BizVerdictNoData
		out.VerdictText = "Belum ada transaksi pada rentang minggu yang dipilih."
		return out, nil
	}
	out.PeriodFrom = weekList[0]
	out.PeriodTo = weekList[len(weekList)-1]
	if t, err := time.Parse("2006-01-02", out.PeriodTo); err == nil {
		out.PeriodTo = t.AddDate(0, 0, 6).Format("2006-01-02")
	}
	n := len(weekList)

	// ── Garis pasar: pertumbuhan same-store seluruh panel ───────────────────
	// Ditimbang omzet karena yang diukur di sini adalah pergerakan UANG di
	// pasar, bukan penilaian orang. Untuk menilai outlet dipakai pembanding
	// sejawat di bawah, yang tiap outletnya satu suara.
	groupNet := make([]float64, n)
	groupCount := make([]int, n)
	groupGrowth := make([]*float64, n)
	var growthVals []float64

	for i, wk := range weekList {
		for _, code := range order {
			if v, ok := outletsByCode[code].weekNet[wk]; ok && v > 0 {
				groupNet[i] += v
				groupCount[i]++
			}
		}
	}
	for i := 1; i < n; i++ {
		var cur, prev float64
		members := 0
		for _, code := range order {
			o := outletsByCode[code]
			a, okA := o.weekNet[weekList[i]]
			b, okB := o.weekNet[weekList[i-1]]
			if okA && okB && a > 0 && b > 0 {
				cur += a
				prev += b
				members++
			}
		}
		if members >= bizMinPanel {
			if g := bizPct(cur, prev); g != nil {
				groupGrowth[i] = g
				growthVals = append(growthVals, *g)
			}
		}
	}

	// Minggu peristiwa pasar: pertumbuhan pasar menyimpang jauh dari
	// kebiasaannya sendiri. Minggu seperti ini bukan cermin kerja manajer.
	eventWeek := make([]bool, n)
	if len(growthVals) >= 4 {
		med := bizMedian(growthVals)
		if sd := bizRobustSD(growthVals); sd != nil && *sd > 0 {
			for i := 0; i < n; i++ {
				if groupGrowth[i] != nil && math.Abs(*groupGrowth[i]-med) > 2*(*sd) {
					eventWeek[i] = true
					out.GroupEvents = append(out.GroupEvents, weekLabel[weekList[i]])
				}
			}
		}
	}

	// Indeks pasar dirantai dari pertumbuhan same-store, bukan dari total
	// penjualan. Total melompat saat ada outlet baru bergabung; indeks berantai
	// tidak.
	groupIndex := make([]*float64, n)
	started := false
	var cursor float64 = 100
	for i := 0; i < n; i++ {
		if !started {
			if i+1 < n && groupGrowth[i+1] != nil {
				started = true
				v := cursor
				groupIndex[i] = &v
			}
			continue
		}
		if groupGrowth[i] == nil {
			continue
		}
		cursor = cursor * (1 + *groupGrowth[i]/100)
		v := round1(cursor)
		groupIndex[i] = &v
	}

	for i, wk := range weekList {
		gw := models.BizGroupWeek{
			WeekStart: wk, Label: weekLabel[wk],
			Net: groupNet[i], OutletCount: groupCount[i], Index: groupIndex[i],
			Growth: groupGrowth[i], IsGroupEvent: eventWeek[i],
		}
		out.Group = append(out.Group, gw)
	}

	// ── Pertumbuhan mingguan tiap outlet ────────────────────────────────────
	// Disimpan per minggu supaya pembanding sejawat bisa disusun ulang untuk
	// tiap outlet tanpa memasukkan outlet itu sendiri.
	wkGrowth := make([]map[string]float64, n)
	for i := range wkGrowth {
		wkGrowth[i] = map[string]float64{}
	}
	for i := 1; i < n; i++ {
		for _, code := range order {
			o := outletsByCode[code]
			cur, okA := o.weekNet[weekList[i]]
			prev, okB := o.weekNet[weekList[i-1]]
			if okA && okB && prev > 0 {
				wkGrowth[i][code] = round1((cur - prev) / prev * 100)
			}
		}
	}

	// ── Blok pembanding ─────────────────────────────────────────────────────
	// Pertumbuhan satu minggu terlalu mudah berubah untuk menilai orang, jadi
	// yang dibandingkan adalah blok minggu. Panjang blok mengikuti rentang yang
	// dipilih (setengahnya), lalu diperpendek otomatis kalau minggu-minggu
	// terlama belum diisi cukup banyak outlet.
	blockSum := func(weekNet map[string]float64, from, to int) (float64, bool) {
		var sum float64
		for i := from; i < to; i++ {
			v, ok := weekNet[weekList[i]]
			if !ok || v <= 0 {
				return 0, false
			}
			sum += v
		}
		return sum, true
	}
	blockTrx := func(weekTrx map[string]int, from, to int) int {
		var sum int
		for i := from; i < to; i++ {
			sum += weekTrx[weekList[i]]
		}
		return sum
	}
	blockPanel := func(bl int) []string {
		var codes []string
		for _, code := range order {
			o := outletsByCode[code]
			if _, okA := blockSum(o.weekNet, n-bl, n); !okA {
				continue
			}
			if _, okB := blockSum(o.weekNet, n-2*bl, n-bl); !okB {
				continue
			}
			codes = append(codes, code)
		}
		return codes
	}

	// Panjang blok dipilih yang MEMBERI PANEL TERBANYAK, bukan yang pertama
	// kebetulan mencukupi. Cara lama (mulai dari n/2 lalu turun sampai panel ≥ 3)
	// berhenti pada blok terpanjang yang lolos ambang minimum — dan blok
	// terpanjang justru menjangkau minggu-minggu terlama yang paling sedikit
	// outletnya. Akibatnya menambah riwayat malah MENYUSUTKAN panel: pada data
	// nyata, geser 9 → 10 minggu menjatuhkan panel 8 → 6 outlet dan membalik
	// gerak pasar dari +24,3% menjadi −17,1%. Dengan memilih panel terbesar,
	// riwayat yang lebih panjang tidak pernah memperburuk cakupan.
	maxBlock := n / 2
	if maxBlock > 13 {
		maxBlock = 13
	}
	blockLen := 0
	var panelCodes []string
	for bl := maxBlock; bl >= 3; bl-- {
		codes := blockPanel(bl)
		// Seri terpanjang menang saat cakupannya sama, karena blok yang lebih
		// panjang lebih tenang terhadap guncangan satu minggu.
		if len(codes) > len(panelCodes) {
			blockLen, panelCodes = bl, codes
		}
	}
	hasBlocks := blockLen >= 3 && len(panelCodes) >= bizMinPanel
	if !hasBlocks {
		blockLen = 0
		panelCodes = nil
	}
	out.BlockWeeks = blockLen
	out.PanelCodes = append(out.PanelCodes, panelCodes...)
	inPanel := map[string]bool{}
	for _, c := range panelCodes {
		inPanel[c] = true
	}

	// Pertumbuhan blok tiap outlet panel — bahan pembanding sejawat.
	blockGrowth := map[string]float64{}
	for _, code := range panelCodes {
		o := outletsByCode[code]
		a, _ := blockSum(o.weekNet, n-blockLen, n)
		b, _ := blockSum(o.weekNet, n-2*blockLen, n-blockLen)
		if g := bizPct(a, b); g != nil {
			blockGrowth[code] = *g
		}
	}

	// Garis pasar untuk blok yang sama — dasar keputusan Markom.
	var marketGrowth4 *float64
	if hasBlocks {
		var newSum, oldSum float64
		for _, code := range panelCodes {
			o := outletsByCode[code]
			a, _ := blockSum(o.weekNet, n-blockLen, n)
			b, _ := blockSum(o.weekNet, n-2*blockLen, n-blockLen)
			newSum += a
			oldSum += b
		}
		marketGrowth4 = bizPct(newSum, oldSum)

		// Cakupan panel terhadap omzet seluruh outlet pada periode yang sama.
		var panelNet, allNet float64
		for _, code := range order {
			if inPanel[code] {
				panelNet += outletsByCode[code].net
			}
			allNet += outletsByCode[code].net
		}
		if allNet > 0 {
			sh := round1(panelNet / allNet * 100)
			out.PanelShare = &sh
		}
	}
	out.GroupGrowth4 = marketGrowth4

	// Batas wajar pasar: derau mingguan pasar itu sendiri, diperkecil oleh
	// panjang blok. Ambang ini ikut bergerak kalau bisnisnya memang bergejolak,
	// jadi tidak ada angka mati yang harus dikalibrasi ulang tiap tahun — dan
	// tidak ada pula statistik yang membesar hanya karena outlet bertambah.
	var marketBand *float64
	if hasBlocks {
		if sd := bizRobustSD(growthVals); sd != nil {
			b := round1(bizConfidence * (*sd) / math.Sqrt(float64(blockLen)))
			marketBand = &b
		}
	}
	out.MarketBand = marketBand
	// Batas wajar yang tidak bisa dihitung berarti "belum bisa dinilai", BUKAN
	// batas nol. Dulu marketBand nil di-deref menjadi 0, sehingga penurunan
	// 0,1% pun langsung divonis pasar turun dan tagihan berpindah ke Markom.
	marketDown := marketGrowth4 != nil && marketBand != nil && *marketGrowth4 < -*marketBand

	// ── Metrik per outlet ───────────────────────────────────────────────────
	var thresholds []float64

	for _, code := range order {
		o := outletsByCode[code]
		bo := models.BizOutlet{
			OutletID: o.id, Code: o.code, Name: o.name,
			Net: o.net, Trx: o.trx, InPanel: inPanel[code],
			Weeks: []models.BizWeek{},
		}
		if o.trx > 0 {
			bo.ATV = math.Round(o.net / float64(o.trx))
		}

		// Titik sandar indeks = minggu pertama outlet ini yang SUDAH punya garis
		// pasar, dan nilainya disamakan dengan level pasar pada minggu itu.
		//
		// Memberi 100 pada minggu pertama tiap outlet terlihat masuk akal tetapi
		// menyesatkan: outlet yang baru terhitung pada minggu puncak memakai
		// puncak itu sebagai 100, sehingga seterusnya tampak anjlok dibanding
		// outlet yang titik nolnya sebelum puncak. Grafiknya lalu bisa
		// bertentangan dengan tabelnya sendiri — outlet berperingkat terbaik
		// tergambar di bawah garis pasar.
		var baseNet, anchor float64
		var rgiVals []float64
		for i, wk := range weekList {
			v, ok := o.weekNet[wk]
			if !ok {
				continue
			}
			if baseNet == 0 && v > 0 && groupIndex[i] != nil {
				baseNet, anchor = v, *groupIndex[i]
			}
			bw := models.BizWeek{WeekStart: wk, Label: weekLabel[wk], Net: v, Trx: o.weekTrx[wk]}
			if baseNet > 0 {
				iv := round1(anchor * v / baseNet)
				bw.Index = &iv
			}
			if g, okG := wkGrowth[i][code]; okG {
				gv := g
				bw.Growth = &gv
				if peer, _ := bizPeerMedian(wkGrowth[i], code); peer != nil {
					r := round1(g - *peer)
					bw.RGI = &r
					rgiVals = append(rgiVals, r)
				}
			}
			bo.Weeks = append(bo.Weeks, bw)
		}

		if hasBlocks {
			if a, okA := blockSum(o.weekNet, n-blockLen, n); okA {
				if b, okB := blockSum(o.weekNet, n-2*blockLen, n-blockLen); okB {
					bo.Growth4 = bizPct(a, b)
					bo.RecentNet, bo.PrevNet = a, b
					bo.RecentTrx = blockTrx(o.weekTrx, n-blockLen, n)
					bo.PrevTrx = blockTrx(o.weekTrx, n-2*blockLen, n-blockLen)
				}
			}
		}
		if bo.Growth4 != nil {
			peer, cnt := bizPeerMedian(blockGrowth, code)
			bo.PeerCount = cnt
			if peer != nil {
				bo.PeerGrowth4 = peer
				r := round1(*bo.Growth4 - *peer)
				bo.RGI4 = &r

				// Terjemahan RGI ke rupiah: berapa penjualan outlet ini
				// seandainya ia bergerak seperti outlet lain, dan berapa
				// selisihnya dengan kenyataan. Manajer membaca angka ini
				// tanpa perlu tahu apa itu poin persen.
				exp := math.Round(bo.PrevNet * (1 + *peer/100))
				gap := math.Round(bo.RecentNet - exp)
				bo.ExpectedNet, bo.GapNet = &exp, &gap
			}
		}
		bo.Breakdown = bizBreakdown(bo.PrevNet, bo.RecentNet, bo.PrevTrx, bo.RecentTrx)

		// Batas wajar adaptif: bizConfidence × galat baku RGI blok, diturunkan
		// dari sebaran RGI mingguan outlet itu sendiri. Outlet berderau tinggi
		// (basis transaksi kecil) butuh selisih lebih besar sebelum boleh
		// disebut tertinggal — kalau tidak, yang terdeteksi hanyalah ukurannya.
		if sd := bizStdDev(rgiVals); sd != nil && blockLen > 0 {
			th := round1(bizConfidence * (*sd) / math.Sqrt(float64(blockLen)))
			bo.Threshold = &th
			if th > 0 {
				thresholds = append(thresholds, th)
			}
		}

		out.Outlets = append(out.Outlets, bo)
	}

	// Plafon "belum bisa dinilai" mengikuti kebiasaan grup ini, bukan angka
	// mati: outlet disebut terlalu berderau kalau batas wajarnya jauh lebih
	// lebar daripada batas wajar outlet pada umumnya di sini. Di grup yang
	// memang bergejolak, plafon ikut naik sehingga halaman tetap terpakai.
	ceiling := bizCeilFloor
	if len(thresholds) > 0 {
		if c := round1(bizCeilFactor * bizMedian(thresholds)); c > ceiling {
			ceiling = c
		}
	}

	// ── Diagnosis & pemilik tindakan ────────────────────────────────────────
	countedForVerdict := 0
	for i := range out.Outlets {
		bo := &out.Outlets[i]

		switch {
		case bo.RGI4 == nil || bo.Threshold == nil || *bo.Threshold <= 0:
			bo.Diagnosis = models.BizDiagNoData
			bo.Owner = models.BizOwnerNone
			if bo.PeerCount > 0 && bo.PeerCount < bizMinPeers {
				bo.Note = fmt.Sprintf("Hanya ada %d outlet lain yang datanya lengkap pada periode ini — belum cukup untuk dijadikan pembanding.", bo.PeerCount)
			} else {
				bo.Note = "Datanya belum cukup panjang untuk dibandingkan dengan outlet lain."
			}

		case *bo.Threshold > ceiling:
			bo.Diagnosis = models.BizDiagWeak
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("Penjualan outlet ini terlalu naik-turun dari minggu ke minggu untuk dibandingkan dengan adil (bisa meleset %s%%, sementara outlet lain cukup %s%%). Biasanya karena jumlah struknya sedikit, jadi satu-dua hari ramai sudah mengubah angka satu minggu.",
				bizNum(*bo.Threshold), bizNum(ceiling/bizCeilFactor))

		case *bo.RGI4 >= *bo.Threshold:
			bo.Diagnosis = models.BizDiagLeading
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("%d outlet lain %s, outlet ini %s. Selisih sebesar ini di luar naik-turun biasanya outlet ini, jadi bukan kebetulan.",
				bo.PeerCount, bizNaikTurun(deref(bo.PeerGrowth4)), bizNaikTurun(deref(bo.Growth4)))

		case *bo.RGI4 <= -*bo.Threshold:
			bo.Diagnosis = models.BizDiagLagging
			bo.Owner = models.BizOwnerOutlet
			bo.Note = fmt.Sprintf("%d outlet lain %s, outlet ini %s. Outlet lain berjualan di pasar yang sama, jadi sebabnya ada di dalam outlet ini.",
				bo.PeerCount, bizNaikTurun(deref(bo.PeerGrowth4)), bizNaikTurun(deref(bo.Growth4)))

		case marketDown:
			// Seirama dengan sejawatnya, tetapi pasarnya sendiri sedang turun.
			// Outlet ini tidak salah apa-apa — yang perlu digerakkan Markom.
			bo.Diagnosis = models.BizDiagOnPace
			bo.Owner = models.BizOwnerMarkom
			bo.Note = fmt.Sprintf("Gerak outlet ini hampir sama dengan %d outlet lain, jadi cara kerjanya tidak bermasalah. Yang sedang turun pasarnya: semua outlet bersama-sama %s. Perbaikannya lewat program Markom, bukan di outlet ini.",
				bo.PeerCount, bizNaikTurun(deref(marketGrowth4)))

		default:
			bo.Diagnosis = models.BizDiagOnPace
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("Outlet ini %s, %d outlet lain %s. Bedanya masih di dalam naik-turun biasanya, jadi belum bisa disebut lebih baik maupun lebih buruk.",
				bizNaikTurun(deref(bo.Growth4)), bo.PeerCount, bizNaikTurun(deref(bo.PeerGrowth4)))
		}

		bo.DiagnosisLabel = bizDiagLabel(bo.Diagnosis)
		bo.Advice = bizAdvice(bo.Diagnosis,
			bizDriver(bo.PrevNet, bo.RecentNet, bo.PrevTrx, bo.RecentTrx), marketDown)

		switch bo.Diagnosis {
		case models.BizDiagLagging:
			out.Lagging = append(out.Lagging, bo.Code)
		case models.BizDiagLeading:
			out.Leading = append(out.Leading, bo.Code)
		}
		if bo.RGI4 != nil {
			countedForVerdict++
		}
	}

	sort.SliceStable(out.Outlets, func(i, j int) bool {
		a, b := out.Outlets[i].RGI4, out.Outlets[j].RGI4
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a > *b
	})

	// ── Vonis: siapa yang harus bertindak ───────────────────────────────────
	// Dua pertanyaan yang saling bebas, masing-masing diuji terhadap deraunya
	// sendiri sehingga hasilnya tidak berubah hanya karena jumlah outlet
	// bertambah:
	//
	//	pasar turun?      → marketGrowth4 di bawah batas wajar pasar
	//	ada yang tertinggal? → ada outlet yang RGI-nya melewati batas wajarnya
	//
	// Karena bebas, keduanya bisa benar bersamaan — dan itu justru situasi
	// paling lazim. Vonis lama memaksa memilih salah satu, sehingga penurunan
	// pasar yang dibarengi satu outlet bermasalah selalu terbaca sebagai
	// "salah outlet" saja.
	// Setiap kalimat yang menyebut angka pasar ikut menyebut cakupannya. Angka
	// itu same-store: hanya outlet panel. Menyebutnya "semua outlet" membuat
	// pembaca memutuskan anggaran Markom di atas cakupan yang salah — pada data
	// nyata dua outlet terbesar (40% omzet grup) berada di luar panel.
	scope := "outlet yang datanya lengkap"
	if hasBlocks {
		if out.PanelShare != nil {
			scope = fmt.Sprintf("%d dari %d outlet yang datanya lengkap (%s%% omzet grup)",
				len(panelCodes), len(out.Outlets), bizNum(*out.PanelShare))
		} else {
			scope = fmt.Sprintf("%d dari %d outlet yang datanya lengkap",
				len(panelCodes), len(out.Outlets))
		}
	}
	switch {
	case !hasBlocks || countedForVerdict == 0:
		out.Verdict = models.BizVerdictNoData
		out.VerdictText = fmt.Sprintf("Data yang ada baru %d minggu penuh, dan belum sampai %d outlet yang datanya lengkap untuk saling dibandingkan. Tunggu beberapa minggu lagi supaya kesimpulannya bisa dipercaya.", n, bizMinPanel)

	case marketDown && len(out.Lagging) > 0:
		out.Verdict = models.BizVerdictBoth
		out.VerdictText = fmt.Sprintf("Ada dua masalah sekaligus, dan keduanya harus dikerjakan bersamaan. Pertama, pasar memang sedang turun: %s digabung %s, lebih dalam daripada naik-turun biasanya. Ini tugas Markom — promosi perlu digencarkan untuk semua outlet. Kedua, %s tetap tertinggal dari outlet lain yang berjualan di pasar yang sama, jadi outlet itu tetap harus dibenahi sendiri. Menggencarkan promosi saja tidak akan menutup ketertinggalan %s.",
			scope, bizNaikTurun(deref(marketGrowth4)), strings.Join(out.Lagging, ", "), strings.Join(out.Lagging, ", "))

	case marketDown:
		out.Verdict = models.BizVerdictMarket
		out.VerdictText = fmt.Sprintf("Outlet turun bersama-sama: %s digabung %s, lebih dalam daripada naik-turun biasanya. Tidak ada satu outlet pun yang jelas lebih buruk daripada yang lain, jadi ini bukan soal cara kerja manajer — semuanya menghadapi keadaan yang sama. Yang perlu bergerak Markom: gencarkan promosi dan program yang menarik orang datang.",
			scope, bizNaikTurun(deref(marketGrowth4)))

	case len(out.Lagging) > 0:
		out.Verdict = models.BizVerdictOutlet
		out.VerdictText = fmt.Sprintf("Pasar tidak sedang turun — %s digabung %s, masih wajar. Tetapi %s tertinggal jauh di bawah outlet lain. Karena outlet yang lain baik-baik saja di pasar yang sama, yang perlu dibenahi outlet itu sendiri, bukan promosinya.",
			scope, bizNaikTurun(deref(marketGrowth4)), strings.Join(out.Lagging, ", "))

	default:
		out.Verdict = models.BizVerdictNormal
		out.VerdictText = fmt.Sprintf("Keadaan normal. %s digabung %s, masih di dalam naik-turun yang biasa, dan tidak ada outlet yang jelas lebih baik atau lebih buruk daripada yang lain. Belum ada yang perlu ditindaklanjuti secara khusus.",
			scope, bizNaikTurun(deref(marketGrowth4)))
	}

	// ── Catatan kaki ────────────────────────────────────────────────────────
	if n < weeks {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Anda memilih %d minggu, tetapi data yang tersedia baru %d minggu penuh. Yang ditampilkan adalah seluruh riwayat yang ada.", weeks, n))
	}
	if hasBlocks {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Yang dibandingkan adalah %d minggu terakhir dengan %d minggu sebelumnya — bukan satu minggu dengan satu minggu. Angka satu minggu terlalu gampang berubah hanya karena hujan, libur, atau satu pesanan besar.",
			blockLen, blockLen))
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Tiap outlet dinilai dengan membandingkannya ke outlet LAIN, dan outlet itu sendiri tidak ikut dihitung sebagai pembandingnya. Dari %d outlet yang datanya lengkap (%s), pembanding tiap outlet adalah %d outlet sisanya.",
			len(panelCodes), strings.Join(panelCodes, ", "), len(panelCodes)-1))
		out.Notes = append(out.Notes,
			"Patokannya diambil dari outlet yang berada di tengah-tengah, bukan dari penjumlahan seluruh penjualan. Artinya outlet besar dan outlet kecil sama-sama dihitung satu suara, sehingga outlet yang penjualannya paling besar tidak bisa sendirian menentukan patokan bagi yang lain.")
		if len(panelCodes) == bizMinPanel {
			out.Notes = append(out.Notes,
				"Outlet yang datanya lengkap hanya tiga, jadi tiap outlet cuma punya dua pembanding. Dengan pembanding sesedikit itu, satu outlet yang anjlok bisa membuat dua outlet lain terlihat lebih bagus daripada sebenarnya. Pakai hasilnya sebagai petunjuk awal saja, jangan sebagai penilaian akhir.")
		}
		if marketBand != nil {
			out.Notes = append(out.Notes, fmt.Sprintf(
				"Kata \"pasar\" di halaman ini berarti penjualan %s digabung jadi satu — bukan seluruh outlet. Outlet yang belum punya riwayat penuh di kedua blok sengaja ditinggalkan supaya angkanya bisa dibandingkan setara. Pasar baru disebut turun kalau turunnya lebih dari %s%%, karena naik-turun sebesar itu memang biasa terjadi dari minggu ke minggu tanpa sebab khusus.",
				scope, bizNum(*marketBand)))
		} else {
			out.Notes = append(out.Notes, fmt.Sprintf(
				"Kata \"pasar\" di halaman ini berarti penjualan %s digabung jadi satu — bukan seluruh outlet. Batas naik-turun wajarnya belum bisa dihitung karena riwayat mingguannya masih terlalu pendek, jadi halaman ini belum menyimpulkan pasar sedang turun atau tidak.",
				scope))
		}
	}
	out.Notes = append(out.Notes,
		"Yang dibandingkan hanya outlet yang berjualan di kedua minggu tersebut. Outlet yang baru buka tidak dihitung sebagai pasar yang sedang tumbuh — kalau dihitung, penjualan grup terlihat naik padahal yang bertambah cuma jumlah gerainya.")
	if len(edgeDropped) > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Minggu pembuka (dan penutup) yang tidak dijalani tujuh hari penuh tidak ikut dibandingkan, untuk %s. Minggu setengah jalan yang dihitung sebagai minggu penuh membuat basis pembandingnya terlalu rendah, sehingga outlet itu terlihat tumbuh padahal hanya jumlah harinya yang bertambah. Total penjualan di tabel tetap memuat minggu tersebut.",
			strings.Join(edgeDropped, ", ")))
	}
	out.Notes = append(out.Notes,
		"Tiap outlet punya batas naik-turun sendiri, diambil dari kebiasaan outlet itu selama ini. Outlet yang struknya sedikit memang angkanya lebih goyang, jadi batasnya dibuat lebih longgar. Tujuannya supaya yang ketahuan itu kinerjanya, bukan sekadar besar-kecilnya outlet.")
	if len(out.GroupEvents) > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Minggu yang tidak biasa: %s. Pada minggu itu semua outlet bergerak jauh dari kebiasaan — biasanya karena libur panjang, cuaca, atau acara besar — jadi angkanya jangan dipakai menilai kerja manajer.",
			strings.Join(out.GroupEvents, ", ")))

		// Penilaian antar-outlet (RGI) kebal terhadap guncangan yang dialami
		// semua outlet — guncangan itu hilang saat growth sejawat dikurangkan.
		// Angka PASAR tidak kebal: ia menjumlah semuanya. Jadi minggu tak normal
		// yang jatuh di dalam blok yang dibandingkan perlu disebutkan, karena
		// dialah yang bisa menggerakkan keputusan Markom.
		var inBlocks []string
		if hasBlocks {
			for i := n - 2*blockLen; i < n; i++ {
				if i >= 0 && eventWeek[i] {
					inBlocks = append(inBlocks, weekLabel[weekList[i]])
				}
			}
		}
		if len(inBlocks) > 0 {
			out.Notes = append(out.Notes, fmt.Sprintf(
				"Perhatikan: %s termasuk di dalam minggu yang sedang dibandingkan, jadi angka pasar di atas ikut terbawa peristiwa itu. Penilaian antar-outlet tidak terpengaruh — guncangan yang dialami semua outlet hilang dengan sendirinya saat outlet dibandingkan satu sama lain.",
				strings.Join(inBlocks, ", ")))
		}
	}
	out.Notes = append(out.Notes,
		"Angka penjualan di sini sama persis dengan Laporan Pendapatan: hanya transaksi yang sudah dibayar, dan transaksi yang dibatalkan tidak ikut dihitung. Minggu yang sedang berjalan belum dimasukkan karena belum genap tujuh hari.")

	// Narasi dirakit paling akhir: ia hanya membaca laporan yang sudah jadi,
	// sehingga kalimatnya dijamin memakai angka yang sama dengan tabel.
	BuildBusinessNarrative(out)

	// Medsos ditempel SESUDAH narasi, bukan sebelum: BuildBusinessNarrative
	// mengosongkan Sections dan Insights saat mulai merakit, jadi bagian yang
	// ditambahkan lebih dulu akan terhapus tanpa jejak. Kegagalannya tidak
	// menggagalkan laporan — angka penjualan tetap terpakai meski scraper medsos
	// sedang diblokir.
	AttachBusinessSocial(out, weeks)

	return out, nil
}
