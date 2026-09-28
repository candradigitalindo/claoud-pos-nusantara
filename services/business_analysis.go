package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

// ── Analisa Bisnis: memisahkan isu outlet dari isu pasar/Markom ──────────────
//
// Halaman ini menjawab satu keputusan: outlet yang dibenahi, atau pasar/program
// yang digerakkan? Urutan pembersihannya:
//
//	1. Kalender. Tiap minggu diskalakan ke "minggu biasa" memakai bobot hari
//	   outlet itu sendiri (lihat business_calendar.go). Minggu berlibur tidak
//	   boleh terbaca sebagai pasar yang sedang ramai, dan minggu sesudahnya
//	   tidak boleh terbaca sebagai pasar yang jatuh.
//	2. Garis pasar — pertumbuhan same-store seluruh panel, ditimbang omzet,
//	   pada angka setara kalender. Menjawab "apakah uang di pasar memang
//	   sedang bergerak, di luar kalender?"
//	3. Pembanding sejawat per outlet — nilai tengah pertumbuhan SELURUH outlet
//	   pembanding, tiap outlet satu suara. Menjawab "outlet ini tertinggal dari
//	   rekan-rekannya atau tidak?"
//	4. Derau & konsistensi. Selisih baru divonis kalau melewati 2 galat baku
//	   outlet itu DAN searah pada hampir semua minggu di blok terakhir.
//
// Blok pembandingnya TETAP (4 minggu lawan 4 minggu sebelumnya) dan selalu
// menempel di ujung riwayat. Menggeser rentang analisa hanya menambah riwayat
// untuk menaksir derau, tidak mengubah apa yang dibandingkan — sehingga angka
// pasar dan vonisnya tidak berubah hanya karena pilihan di kotak rentang.
//
// Sumber penjualan sama dengan Laporan Pendapatan (cloud_transactions,
// mengecualikan order yang di-void). Minggu bisnis = Senin–Minggu, zona waktu
// aplikasi lewat tz_date(). Minggu berjalan selalu dikecualikan karena belum
// genap. Semua perbandingan SAME-STORE: hanya outlet yang berjualan di kedua
// sisi yang ikut menyusun angka.

type bizOutletRaw struct {
	id, code, name string
	daily          map[string]bizDaily
	weekNet        map[string]float64 // penjualan sebenarnya
	weekTrx        map[string]int
	weekAdj        map[string]float64 // setara kalender
	weekFactor     map[string]float64
	weekCal        map[string]string
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

// bizWeekStartOf = tanggal Senin dari minggu yang memuat hari tersebut.
func bizWeekStartOf(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7)).Format("2006-01-02")
}

// bizWeekLabel = label ISO "2026-W35" dari tanggal Senin.
func bizWeekLabel(weekStart string) string {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return weekStart
	}
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// bizDropPartialEdgeWeeks membuang minggu yang TIDAK dijalani outlet selama
// tujuh hari penuh — minggu pembuka (outlet mulai di tengah minggu) dan minggu
// penutup (outlet berhenti di tengah minggu). Minggu setengah jalan yang
// dihitung sebagai minggu penuh membuat basis pembandingnya terlalu rendah;
// pada data nyata selisihnya sampai 13 poin RGI dan cukup untuk membalik vonis.
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
	// bizMinPanel = jumlah outlet minimum agar garis pasar dan nilai tengah
	// pembanding berarti. Dengan dua outlet, "pasar" praktis satu outlet lawan
	// satu outlet — menyesatkan, bukan sekadar kurang tepat.
	bizMinPanel = 3

	// Panjang blok pembanding. Empat minggu = satu bulan bisnis, cukup panjang
	// untuk meredam satu akhir pekan hujan, cukup pendek untuk masih menjawab
	// "bulan ini". Tiga minggu dipakai hanya bila riwayatnya belum genap 8.
	bizBlockWeeks    = 4
	bizBlockWeeksMin = 3

	// bizConfidence = berapa galat baku selisih harus melampaui sebelum boleh
	// disebut nyata. Dua galat baku ≈ keyakinan 95% dua sisi — standar yang
	// wajar sebelum menuduh orang.
	bizConfidence = 2.0

	// bizThresholdFloor = batas wajar minimum, poin persen. Selisih di bawah
	// ini tidak pernah layak ditindak berapa pun tenangnya deraunya.
	bizThresholdFloor = 2.0

	// bizWatchFactor: selisih di atas 60% batas wajar yang konsisten arahnya
	// sudah pantas dipantau meski belum bisa divonis.
	bizWatchFactor = 0.6

	// bizLearnWeeks = panjang jendela hitung. Blok, batas wajar, dan pengali
	// kalender selalu dihitung dari 26 minggu terakhir yang tersedia, berapa pun
	// rentang yang diminta untuk digambar. Kalau tidak, menggeser kotak rentang
	// mengubah model derau dan pengali libur — dan ikut mengubah vonis.
	bizLearnWeeks = 26

	// bizPoolWeight = bobot derau outlet sendiri saat digabung dengan model
	// derau grup (yang mengecil seiring banyaknya struk). Setengah-setengah:
	// derau dari 8–12 titik terlalu goyah untuk dipercaya sendirian, model grup
	// terlalu kasar untuk dipercaya sendirian.
	bizPoolWeight = 0.5

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
// Memakai median, bukan rata-rata: satu minggu ekstrem menggelembungkan
// simpangan baku yang dipakai untuk mengujinya sendiri, sehingga justru lolos
// tak terdeteksi. Jatuh kembali ke simpangan baku biasa kalau sebarannya
// terlalu pendek atau MAD-nya nol.
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

// bizPanelMedian = nilai tengah pertumbuhan SELURUH outlet pembanding, beserta
// jumlah penyusunnya.
//
// Tiap outlet satu suara — bukan rata-rata tertimbang omzet, yang membiarkan
// satu outlet raksasa menentukan patokan bagi semua manajer lain. Outlet yang
// dinilai IKUT dihitung: pada median, satu anggota hanya bisa menggeser
// patokan paling jauh setengah jarak ke tetangganya, jadi ia tidak bisa
// menyeret patokannya sendiri; dan karena patokannya sama untuk semua outlet,
// dua outlet yang geraknya berbeda 2 poin juga berbeda 2 poin RGI-nya. Versi
// tanpa-diri membuat patokan berpindah tergantung outlet di atas atau di
// bawah tengah, sehingga tidak ada outlet yang pernah bisa mendapat RGI nol.
func bizPanelMedian(growth map[string]float64) (*float64, int) {
	vals := make([]float64, 0, len(growth))
	for _, g := range growth {
		vals = append(vals, g)
	}
	if len(vals) < bizMinPanel {
		return nil, len(vals)
	}
	v := round1(bizMedian(vals))
	return &v, len(vals)
}

// bizRupiah menulis rupiah bulat dengan pemisah titik — "Rp 12.450.000".
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

// bizBreakdown menguraikan perubahan penjualan (angka sebenarnya) menjadi dua
// sebab yang bisa ditindak: berapa orang yang datang, dan berapa besar belanja
// tiap orang. Dipakai angka sebenarnya, bukan setara kalender: pertanyaannya
// "apa yang terjadi", dan kalau jawabannya "pengunjung berkurang karena tidak
// ada libur", kalimat kalender di catatan outlet yang menjelaskannya.
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
	case models.BizDiagWatch:
		return "Perlu Dipantau"
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
		return "Outlet ini sedang paling baik, dan bukan karena libur — angkanya sudah disetarakan. Tanyakan ke manajernya apa yang mereka ubah belakangan — jam sibuk, cara menawarkan, atau susunan menu — lalu terapkan di outlet lain."

	case models.BizDiagWatch:
		return "Selisihnya mulai terlihat, tetapi belum melewati batas wajar atau belum konsisten dari minggu ke minggu. Belum perlu langkah besar: cek cepat jam buka, stok menu andalan, dan kecepatan layanan, lalu lihat lagi dua minggu ke depan."

	case models.BizDiagWeak:
		return "Penjualan mingguannya terlalu naik-turun untuk dinilai per minggu. Nilai outlet ini per bulan saja, dan pastikan semua transaksi benar-benar tercatat di kasir."

	case models.BizDiagNoData:
		return "Belum ada langkah yang bisa disarankan sampai riwayat penjualannya lebih panjang."

	default: // seirama
		if marketDown {
			return "Tidak ada yang perlu dibenahi di outlet ini — geraknya sama dengan outlet lain. Yang turun pasarnya, dan itu setelah libur dikoreksi; cari sebabnya bersama Markom: cuaca, daya beli, pesaing, atau promosi yang mengendur."
		}
		return "Outlet ini berjalan normal, sejalan dengan outlet lain. Tidak ada tindakan khusus; teruskan yang sudah berjalan."
	}
}

// bizBlockLen memilih panjang blok dari banyaknya minggu penuh yang ada dan
// panjang yang diminta pengguna (2, 3, atau 4). Blok 4 boleh jatuh ke 3 bila
// riwayatnya belum genap 8 minggu; blok 2 tidak punya cadangan.
func bizBlockLen(n, want int) int {
	switch want {
	case 2:
		if n >= 4 {
			return 2
		}
		return 0
	case 3:
		if n >= 6 {
			return 3
		}
		return 0
	}
	switch {
	case n >= 2*bizBlockWeeks:
		return bizBlockWeeks
	case n >= 2*bizBlockWeeksMin:
		return bizBlockWeeksMin
	}
	return 0
}

// bizConsistencyNeed = minggu searah minimum sebelum selisih boleh divonis:
// 3 dari 4, 2 dari 3, dan KEDUA minggu pada blok 2 — satu minggu searah tidak
// pernah cukup untuk menilai orang.
func bizConsistencyNeed(blockLen int) int {
	if blockLen-1 < 2 {
		return 2
	}
	return blockLen - 1
}

// bizConsistency menghitung berapa minggu dalam rgis yang searah dengan sign
// (positif/negatif). Nol dan minggu tanpa nilai tidak dihitung searah.
func bizConsistency(rgis []*float64, sign float64) int {
	n := 0
	for _, r := range rgis {
		if r == nil || sign == 0 {
			continue
		}
		if (*r > 0 && sign > 0) || (*r < 0 && sign < 0) {
			n++
		}
	}
	return n
}

// bizBlendedSD menggabungkan derau mingguan outlet sendiri (own, dari ownN
// titik) dengan model derau grup: pooledC / √(struk per minggu). Salah satu
// yang tidak tersedia ditandai nol, dan yang tersedia dipakai sendirian.
func bizBlendedSD(own float64, pooledC float64, trxPerWeek float64) float64 {
	var pooled float64
	if pooledC > 0 && trxPerWeek > 0 {
		pooled = pooledC / math.Sqrt(trxPerWeek)
	}
	switch {
	case own > 0 && pooled > 0:
		return math.Sqrt(bizPoolWeight*own*own + (1-bizPoolWeight)*pooled*pooled)
	case own > 0:
		return own
	default:
		return pooled
	}
}

// GetBusinessAnalysis menghitung tren penjualan mingguan per outlet (setara
// kalender), garis pasar, dan RGI tiap outlet terhadap sejawatnya — lalu
// menyimpulkan siapa yang harus bertindak.
// weeks = jumlah minggu penuh ke belakang yang diminta pengguna.
func GetBusinessAnalysis(weeks int) (*models.BusinessAnalysis, error) {
	return GetBusinessAnalysisWith(weeks, bizBlockWeeks)
}

// GetBusinessAnalysisWith = GetBusinessAnalysis dengan panjang blok pilihan
// pengguna: 4 (bawaan, satu bulan bisnis) atau 2 (lebih cepat menangkap
// perubahan, tetapi batas wajarnya ±41% lebih lebar dan vonis butuh kedua
// minggu searah).
func GetBusinessAnalysisWith(weeks, block int) (*models.BusinessAnalysis, error) {
	if block != 2 && block != 3 {
		block = bizBlockWeeks
	}
	if weeks < 6 {
		weeks = 6
	}
	if weeks > 104 {
		weeks = 104
	}
	learnWeeks := weeks
	if learnWeeks < bizLearnWeeks {
		learnWeeks = bizLearnWeeks
	}

	// Senin minggu berjalan menurut zona waktu aplikasi; jendela hitung
	// berakhir Minggu sebelumnya.
	var cur string
	if err := database.DB.QueryRow(
		`SELECT to_char(date_trunc('week', tz_today()::timestamp)::date, 'YYYY-MM-DD')`).Scan(&cur); err != nil {
		return nil, err
	}
	curT, err := time.Parse("2006-01-02", cur)
	if err != nil {
		return nil, err
	}
	rangeStart := curT.AddDate(0, 0, -learnWeeks*7).Format("2006-01-02")
	rangeEnd := curT.AddDate(0, 0, -1).Format("2006-01-02")

	// Penjualan HARIAN, bukan mingguan: bobot hari dan pengali libur dipelajari
	// dari hari, lalu minggu dirakit di sini.
	rows, err := database.DB.Query(`
		SELECT TRIM(o.id), TRIM(o.code), o.name,
		       to_char(tz_date(t.created_at), 'YYYY-MM-DD') AS d,
		       COALESCE(SUM(t.total_amount), 0), COUNT(*)
		FROM cloud_transactions t
		JOIN outlets o ON o.id = t.outlet_id
		WHERE t.created_at >= tz_day_start($1::date)
		  AND t.created_at <  tz_day_start($2::date)
		  AND o.is_active = true`+txNotVoided("t")+`
		GROUP BY 1, 2, 3, 4
		ORDER BY 2, 4`, rangeStart, cur)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	outletsByCode := map[string]*bizOutletRaw{}
	var order []string
	for rows.Next() {
		var id, code, name, day string
		var net float64
		var trx int
		if err := rows.Scan(&id, &code, &name, &day, &net, &trx); err != nil {
			return nil, err
		}
		o, ok := outletsByCode[code]
		if !ok {
			o = &bizOutletRaw{id: id, code: code, name: name,
				daily: map[string]bizDaily{}, weekNet: map[string]float64{}, weekTrx: map[string]int{},
				weekAdj: map[string]float64{}, weekFactor: map[string]float64{}, weekCal: map[string]string{}}
			outletsByCode[code] = o
			order = append(order, code)
		}
		o.daily[day] = bizDaily{net: net, trx: trx}
		wk := bizWeekStartOf(day)
		o.weekNet[wk] += net
		o.weekTrx[wk] += trx
		o.net += net
		o.trx += trx
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

	// Hari tutup di dalam masa beroperasi diisi nol supaya bobot harinya
	// jujur: outlet yang libur tiap Senin memang bernilai nol di hari Senin.
	for _, o := range outletsByCode {
		if o.firstDay == "" || o.lastDay == "" {
			continue
		}
		from, to := o.firstDay, o.lastDay
		if from < rangeStart {
			from = rangeStart
		}
		if to > rangeEnd {
			to = rangeEnd
		}
		a, errA := time.Parse("2006-01-02", from)
		b, errB := time.Parse("2006-01-02", to)
		if errA != nil || errB != nil {
			continue
		}
		for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
			key := d.Format("2006-01-02")
			if _, ok := o.daily[key]; !ok {
				o.daily[key] = bizDaily{}
			}
		}
	}

	// Buang minggu tepi yang tidak dijalani penuh dari perhitungan.
	// o.net dan o.trx sengaja TIDAK dihitung ulang: itu total penjualan
	// sebenarnya sepanjang periode dan tetap ditampilkan apa adanya.
	var edgeDropped []string
	for _, code := range order {
		o := outletsByCode[code]
		bizDropPartialEdgeWeeks(o)
		if o.droppedWeeks > 0 {
			edgeDropped = append(edgeDropped, o.code)
		}
	}
	sort.Strings(order)

	weekLabel := map[string]string{}
	for _, o := range outletsByCode {
		for wk := range o.weekNet {
			weekLabel[wk] = bizWeekLabel(wk)
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
		WindowWeeks:    len(weekList),
		Outlets:        []models.BizOutlet{},
		Group:          []models.BizGroupWeek{},
		Lagging:        []string{},
		Leading:        []string{},
		Watch:          []string{},
		PanelCodes:     []string{},
		Notes:          []string{},
		Calendar:       []models.BizCalendarDay{},
	}
	if len(weekList) == 0 {
		out.Verdict = models.BizVerdictNoData
		out.VerdictText = "Belum ada transaksi pada rentang minggu yang dipilih."
		return out, nil
	}
	out.PeriodFrom = weekList[0]
	out.PeriodTo = bizWeekEnd(weekList[len(weekList)-1])
	n := len(weekList)

	// ── Kalender & penjualan setara kalender ────────────────────────────────
	// Dimuat sampai empat minggu ke depan: perkiraan model butuh tahu libur
	// pada minggu-minggu yang belum terjadi.
	cal, err := bizLoadCalendar(rangeStart, curT.AddDate(0, 0, 7*bizModelHorizon).Format("2006-01-02"))
	if err != nil {
		// Tanpa kalender analisa tetap jalan — tetapi tanpa koreksi, dan itu
		// disebutkan di catatan kaki lewat CalendarModel yang kosong.
		log.Printf("GetBusinessAnalysis: kalender tidak terbaca: %v", err)
		cal = &bizCalendar{days: map[string][]models.BizCalendarDay{}}
	}
	for _, wk := range weekList {
		t, _ := time.Parse("2006-01-02", wk)
		for i := 0; i < 7; i++ {
			out.Calendar = append(out.Calendar, cal.days[t.AddDate(0, 0, i).Format("2006-01-02")]...)
		}
	}
	dailyByCode := map[string]map[string]bizDaily{}
	for code, o := range outletsByCode {
		dailyByCode[code] = o.daily
	}
	calModel := bizBuildCalendarModel(dailyByCode, cal)
	out.CalendarModel = calModel.export()

	for _, code := range order {
		o := outletsByCode[code]
		for wk, v := range o.weekNet {
			f, note := calModel.weekFactor(code, wk)
			o.weekFactor[wk] = f
			o.weekAdj[wk] = v * f
			o.weekCal[wk] = note
		}
	}

	// ── Garis pasar: pertumbuhan same-store seluruh panel ───────────────────
	// Ditimbang omzet karena yang diukur di sini adalah pergerakan UANG di
	// pasar, bukan penilaian orang. Untuk menilai outlet dipakai pembanding
	// sejawat di bawah, yang tiap outletnya satu suara.
	groupNet := make([]float64, n)
	groupAdj := make([]float64, n)
	groupCount := make([]int, n)
	groupGrowth := make([]*float64, n)
	groupGrowthRaw := make([]*float64, n)
	var growthVals []float64

	for i, wk := range weekList {
		for _, code := range order {
			o := outletsByCode[code]
			if v, ok := o.weekNet[wk]; ok && v > 0 {
				groupNet[i] += v
				groupAdj[i] += o.weekAdj[wk]
				groupCount[i]++
			}
		}
	}
	for i := 1; i < n; i++ {
		var curA, prevA, curR, prevR float64
		members := 0
		for _, code := range order {
			o := outletsByCode[code]
			a, okA := o.weekNet[weekList[i]]
			b, okB := o.weekNet[weekList[i-1]]
			if okA && okB && a > 0 && b > 0 {
				curR += a
				prevR += b
				curA += o.weekAdj[weekList[i]]
				prevA += o.weekAdj[weekList[i-1]]
				members++
			}
		}
		if members >= bizMinPanel {
			if g := bizPct(curA, prevA); g != nil {
				groupGrowth[i] = g
				growthVals = append(growthVals, *g)
			}
			groupGrowthRaw[i] = bizPct(curR, prevR)
		}
	}

	// Minggu peristiwa pasar: pertumbuhan pasar (setelah kalender) menyimpang
	// jauh dari kebiasaannya sendiri. Minggu seperti ini bukan cermin kerja
	// manajer — dan bukan pula kalender, karena kalender sudah dikoreksi.
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
		out.Group = append(out.Group, models.BizGroupWeek{
			WeekStart: wk, Label: weekLabel[wk],
			Net: groupNet[i], NetAdj: math.Round(groupAdj[i]), OutletCount: groupCount[i],
			Index: groupIndex[i], Growth: groupGrowth[i], GrowthRaw: groupGrowthRaw[i],
			Calendar: bizWeekCalendarNote(cal, wk), IsGroupEvent: eventWeek[i],
		})
	}

	// ── Pertumbuhan mingguan tiap outlet (setara kalender) ─────────────────
	wkGrowth := make([]map[string]float64, n)
	wkPeer := make([]*float64, n)
	for i := range wkGrowth {
		wkGrowth[i] = map[string]float64{}
	}
	for i := 1; i < n; i++ {
		for _, code := range order {
			o := outletsByCode[code]
			cur, okA := o.weekAdj[weekList[i]]
			prev, okB := o.weekAdj[weekList[i-1]]
			if okA && okB && prev > 0 {
				wkGrowth[i][code] = round1((cur - prev) / prev * 100)
			}
		}
		wkPeer[i], _ = bizPanelMedian(wkGrowth[i])
	}

	// ── Blok pembanding tetap ───────────────────────────────────────────────
	blockSum := func(m map[string]float64, from, to int) (float64, bool) {
		var sum float64
		for i := from; i < to; i++ {
			v, ok := m[weekList[i]]
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

	blockLen := bizBlockLen(n, block)
	var panelCodes []string
	if blockLen > 0 {
		panelCodes = blockPanel(blockLen)
		// Blok empat minggu yang panelnya belum cukup: coba tiga minggu sebelum
		// menyerah — riwayat pendek lebih baik dinilai kasar daripada tidak.
		if len(panelCodes) < bizMinPanel && block == bizBlockWeeks && blockLen > bizBlockWeeksMin {
			if alt := blockPanel(bizBlockWeeksMin); len(alt) >= bizMinPanel {
				blockLen, panelCodes = bizBlockWeeksMin, alt
			}
		}
	}
	hasBlocks := blockLen > 0 && len(panelCodes) >= bizMinPanel
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

	// Pertumbuhan blok tiap outlet panel — bahan nilai tengah pembanding.
	blockGrowth := map[string]float64{}
	for _, code := range panelCodes {
		o := outletsByCode[code]
		a, _ := blockSum(o.weekAdj, n-blockLen, n)
		b, _ := blockSum(o.weekAdj, n-2*blockLen, n-blockLen)
		if g := bizPct(a, b); g != nil {
			blockGrowth[code] = *g
		}
	}
	peerMedian, peerCount := bizPanelMedian(blockGrowth)

	// Garis pasar untuk blok yang sama — dasar keputusan sisi pasar/Markom.
	var marketGrowth4, marketGrowthRaw4, calendarEffect *float64
	if hasBlocks {
		var newAdj, oldAdj, newRaw, oldRaw float64
		for _, code := range panelCodes {
			o := outletsByCode[code]
			a, _ := blockSum(o.weekAdj, n-blockLen, n)
			b, _ := blockSum(o.weekAdj, n-2*blockLen, n-blockLen)
			newAdj += a
			oldAdj += b
			ar, _ := blockSum(o.weekNet, n-blockLen, n)
			br, _ := blockSum(o.weekNet, n-2*blockLen, n-blockLen)
			newRaw += ar
			oldRaw += br
		}
		marketGrowth4 = bizPct(newAdj, oldAdj)
		marketGrowthRaw4 = bizPct(newRaw, oldRaw)
		if marketGrowth4 != nil && marketGrowthRaw4 != nil {
			v := round1(*marketGrowthRaw4 - *marketGrowth4)
			calendarEffect = &v
		}

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
	out.GroupGrowthRaw4 = marketGrowthRaw4
	out.CalendarEffect = calendarEffect

	// Batas wajar pasar: derau mingguan pasar itu sendiri (setelah kalender),
	// diperkecil oleh panjang blok. Pertumbuhan blok ≈ selisih rata-rata dua
	// blok, jadi galat bakunya ≈ simpangan mingguan ÷ √panjang blok.
	var marketBand *float64
	if hasBlocks {
		if sd := bizRobustSD(growthVals); sd != nil {
			b := round1(bizConfidence * (*sd) / math.Sqrt(float64(blockLen)))
			marketBand = &b
		}
	}
	out.MarketBand = marketBand
	marketDown := marketGrowth4 != nil && marketBand != nil && *marketGrowth4 < -*marketBand

	// ── Metrik per outlet ───────────────────────────────────────────────────
	type noiseInfo struct {
		ownSD      float64 // simpangan RGI mingguan outlet sendiri (0 = tak ada)
		trxPerWeek float64
	}
	noise := map[string]noiseInfo{}
	var pooledCs []float64

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
		// pasar, dan nilainya disamakan dengan level pasar pada minggu itu —
		// supaya outlet yang baru terhitung pada minggu puncak tidak memakai
		// puncak itu sebagai 100 dan seterusnya tampak anjlok.
		var baseNet, anchor float64
		weekRGI := make([]*float64, n)
		for i, wk := range weekList {
			v, ok := o.weekNet[wk]
			if !ok {
				continue
			}
			adj := o.weekAdj[wk]
			if baseNet == 0 && adj > 0 && groupIndex[i] != nil {
				baseNet, anchor = adj, *groupIndex[i]
			}
			bw := models.BizWeek{WeekStart: wk, Label: weekLabel[wk], Net: v, Trx: o.weekTrx[wk],
				NetAdj: math.Round(adj), CalFactor: o.weekFactor[wk], Calendar: o.weekCal[wk]}
			if baseNet > 0 {
				iv := round1(anchor * adj / baseNet)
				bw.Index = &iv
			}
			if g, okG := wkGrowth[i][code]; okG {
				gv := g
				bw.Growth = &gv
				if wkPeer[i] != nil {
					r := round1(g - *wkPeer[i])
					bw.RGI = &r
					weekRGI[i] = &r
				}
			}
			bo.Weeks = append(bo.Weeks, bw)
		}

		if hasBlocks {
			if a, okA := blockSum(o.weekNet, n-blockLen, n); okA {
				if b, okB := blockSum(o.weekNet, n-2*blockLen, n-blockLen); okB {
					bo.RecentNet, bo.PrevNet = a, b
					aAdj, _ := blockSum(o.weekAdj, n-blockLen, n)
					bAdj, _ := blockSum(o.weekAdj, n-2*blockLen, n-blockLen)
					bo.RecentAdj, bo.PrevAdj = math.Round(aAdj), math.Round(bAdj)
					bo.Growth4 = bizPct(aAdj, bAdj)
					bo.GrowthRaw4 = bizPct(a, b)
					bo.RecentTrx = blockTrx(o.weekTrx, n-blockLen, n)
					bo.PrevTrx = blockTrx(o.weekTrx, n-2*blockLen, n-blockLen)
				}
			}
		}
		if bo.Growth4 != nil && peerMedian != nil {
			bo.PeerGrowth4 = peerMedian
			bo.PeerCount = peerCount
			r := round1(*bo.Growth4 - *peerMedian)
			bo.RGI4 = &r

			// Terjemahan RGI ke rupiah sebenarnya: penjualan blok terakhir
			// seandainya outlet ini bergerak seperti outlet pembanding, dengan
			// kalender blok terakhir apa adanya.
			fRecent := 1.0
			if bo.RecentNet > 0 && bo.RecentAdj > 0 {
				fRecent = bo.RecentAdj / bo.RecentNet
			}
			exp := math.Round(bo.PrevAdj * (1 + *peerMedian/100) / fRecent)
			gap := math.Round(bo.RecentNet - exp)
			bo.ExpectedNet, bo.GapNet = &exp, &gap

			// Konsistensi: berapa minggu di blok terakhir yang searah.
			bo.ConsistencyNeed = bizConsistencyNeed(blockLen)
			bo.Consistency = bizConsistency(weekRGI[n-blockLen:], r)
		}
		bo.Breakdown = bizBreakdown(bo.PrevNet, bo.RecentNet, bo.PrevTrx, bo.RecentTrx)

		// Derau outlet sendiri: sebaran RGI mingguan SEBELUM blok terakhir,
		// supaya kemerosotan yang sedang diuji tidak ikut melebarkan batas
		// yang mengujinya. Kalau riwayat sebelum blok belum cukup, seluruhnya
		// dipakai.
		if hasBlocks && bo.InPanel {
			var baseline, all []float64
			for i, r := range weekRGI {
				if r == nil {
					continue
				}
				all = append(all, *r)
				if i < n-blockLen {
					baseline = append(baseline, *r)
				}
			}
			vals := baseline
			if len(vals) < 4 {
				vals = all
			}
			ni := noiseInfo{trxPerWeek: float64(bo.RecentTrx+bo.PrevTrx) / float64(2*blockLen)}
			// Simpangan robust (MAD): satu minggu yang meledak — misalnya sisa
			// pengali libur yang tidak pas — tidak boleh melebarkan batas
			// wajar berbulan-bulan sesudahnya.
			if sd := bizRobustSD(vals); sd != nil && *sd > 0 {
				ni.ownSD = *sd
				if ni.trxPerWeek > 0 {
					pooledCs = append(pooledCs, ni.ownSD*math.Sqrt(ni.trxPerWeek))
				}
			}
			noise[code] = ni
		}

		out.Outlets = append(out.Outlets, bo)
	}

	// Model derau grup: derau relatif ∝ 1/√struk. Konstantanya nilai tengah
	// (derau outlet × √struk) seluruh outlet — cukup tahan terhadap satu outlet
	// yang kebetulan sangat tenang atau sangat liar.
	var pooledC float64
	if len(pooledCs) >= bizMinPanel {
		pooledC = bizMedian(pooledCs)
	}
	var thresholds []float64
	for i := range out.Outlets {
		bo := &out.Outlets[i]
		ni, ok := noise[bo.Code]
		if !ok || blockLen == 0 {
			continue
		}
		sd := bizBlendedSD(ni.ownSD, pooledC, ni.trxPerWeek)
		if sd <= 0 {
			continue
		}
		th := round1(math.Max(bizThresholdFloor, bizConfidence*sd/math.Sqrt(float64(blockLen))))
		bo.Threshold = &th
		thresholds = append(thresholds, th)
	}

	// Plafon "belum bisa dinilai" mengikuti kebiasaan grup ini, bukan angka
	// mati: outlet disebut terlalu berderau kalau batas wajarnya jauh lebih
	// lebar daripada batas wajar outlet pada umumnya di sini.
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
		abs := math.Abs(deref(bo.RGI4))
		konsisten := bo.RGI4 != nil && bo.Consistency >= bo.ConsistencyNeed
		lawan := fmt.Sprintf("nilai tengah %d outlet pembanding", bo.PeerCount)

		switch {
		case bo.RGI4 == nil || bo.Threshold == nil || *bo.Threshold <= 0:
			bo.Diagnosis = models.BizDiagNoData
			bo.Owner = models.BizOwnerNone
			switch {
			case hasBlocks && !bo.InPanel:
				bo.Note = fmt.Sprintf("Outlet ini belum punya penjualan di seluruh %d minggu yang dibandingkan — biasanya karena baru buka atau sempat tutup — jadi belum bisa disandingkan dengan outlet lain.", 2*blockLen)
			case hasBlocks && bo.RGI4 != nil:
				bo.Note = "Riwayat mingguannya belum cukup panjang untuk menaksir batas naik-turun wajarnya, jadi selisihnya belum bisa dinilai."
			default:
				bo.Note = "Datanya belum cukup panjang untuk dibandingkan dengan outlet lain."
			}

		case *bo.Threshold > ceiling:
			bo.Diagnosis = models.BizDiagWeak
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("Penjualan outlet ini terlalu naik-turun dari minggu ke minggu untuk dibandingkan dengan adil (bisa meleset %s poin, sementara outlet lain cukup %s). Biasanya karena jumlah struknya sedikit, jadi satu-dua hari ramai sudah mengubah angka satu minggu.",
				bizNum(*bo.Threshold), bizNum(ceiling/bizCeilFactor))

		case abs >= *bo.Threshold && konsisten && *bo.RGI4 > 0:
			bo.Diagnosis = models.BizDiagLeading
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("Setelah libur dikoreksi, %s %s, outlet ini %s. Selisih %s poin melewati batas wajar %s dan searah pada %d dari %d minggu terakhir — bukan kebetulan.",
				lawan, bizNaikTurun(deref(bo.PeerGrowth4)), bizNaikTurun(deref(bo.Growth4)),
				bizNum(abs), bizNum(*bo.Threshold), bo.Consistency, blockLen)

		case abs >= *bo.Threshold && konsisten:
			bo.Diagnosis = models.BizDiagLagging
			bo.Owner = models.BizOwnerOutlet
			bo.Note = fmt.Sprintf("Setelah libur dikoreksi, %s %s, outlet ini %s. Selisih %s poin melewati batas wajar %s dan searah pada %d dari %d minggu terakhir. Outlet lain berjualan di pasar dan kalender yang sama, jadi sebabnya ada di dalam outlet ini.",
				lawan, bizNaikTurun(deref(bo.PeerGrowth4)), bizNaikTurun(deref(bo.Growth4)),
				bizNum(abs), bizNum(*bo.Threshold), bo.Consistency, blockLen)

		case abs >= *bo.Threshold || (abs >= bizWatchFactor*(*bo.Threshold) && konsisten):
			bo.Diagnosis = models.BizDiagWatch
			bo.Owner = models.BizOwnerNone
			arah := "di atas"
			if *bo.RGI4 < 0 {
				arah = "di bawah"
			}
			if abs >= *bo.Threshold {
				bo.Note = fmt.Sprintf("Outlet ini %s, %s %s — selisih %s poin melewati batas wajar %s, tetapi baru searah pada %d dari %d minggu terakhir. Bisa satu minggu yang kebetulan; bisa awal perubahan. Dipantau dulu, belum divonis.",
					bizNaikTurun(deref(bo.Growth4)), lawan, bizNaikTurun(deref(bo.PeerGrowth4)),
					bizNum(abs), bizNum(*bo.Threshold), bo.Consistency, blockLen)
			} else {
				bo.Note = fmt.Sprintf("Outlet ini %s, %s %s. Selisih %s poin masih di dalam batas wajar %s, tetapi konsisten %s pada %d dari %d minggu terakhir. Belum bisa divonis; pantas dipantau.",
					bizNaikTurun(deref(bo.Growth4)), lawan, bizNaikTurun(deref(bo.PeerGrowth4)),
					bizNum(abs), bizNum(*bo.Threshold), arah, bo.Consistency, blockLen)
			}

		case marketDown:
			// Seirama dengan sejawatnya, tetapi pasarnya sendiri sedang turun
			// setelah kalender dikoreksi. Outlet ini tidak salah apa-apa.
			bo.Diagnosis = models.BizDiagOnPace
			bo.Owner = models.BizOwnerMarkom
			bo.Note = fmt.Sprintf("Gerak outlet ini hampir sama dengan %s, jadi cara kerjanya tidak bermasalah. Yang turun pasarnya: setelah libur dikoreksi pasar %s, dan %s. Sebabnya ada di luar outlet ini.",
				lawan, bizNaikTurun(deref(marketGrowth4)), bizArahOutlet(out.Outlets))

		default:
			bo.Diagnosis = models.BizDiagOnPace
			bo.Owner = models.BizOwnerNone
			bo.Note = fmt.Sprintf("Outlet ini %s, %s %s. Bedanya %s poin, masih di dalam batas wajar %s, jadi belum bisa disebut lebih baik maupun lebih buruk.",
				bizNaikTurun(deref(bo.Growth4)), lawan, bizNaikTurun(deref(bo.PeerGrowth4)),
				bizNum(abs), bizNum(*bo.Threshold))
		}

		bo.DiagnosisLabel = bizDiagLabel(bo.Diagnosis)
		bo.Advice = bizAdvice(bo.Diagnosis,
			bizDriver(bo.PrevNet, bo.RecentNet, bo.PrevTrx, bo.RecentTrx), marketDown)

		switch bo.Diagnosis {
		case models.BizDiagLagging:
			out.Lagging = append(out.Lagging, bo.Code)
		case models.BizDiagLeading:
			out.Leading = append(out.Leading, bo.Code)
		case models.BizDiagWatch:
			out.Watch = append(out.Watch, bo.Code)
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
	// Dua pertanyaan yang saling bebas:
	//	pasar turun?         → gerak pasar setara kalender di bawah batas wajarnya
	//	ada yang tertinggal? → ada outlet yang RGI-nya melewati batas wajarnya,
	//	                       konsisten
	// Karena bebas, keduanya bisa benar bersamaan. Setiap kalimat yang menyebut
	// angka pasar ikut menyebut cakupannya (same-store, outlet panel saja) dan
	// angka mentahnya, supaya pembaca melihat berapa yang dibuat kalender.
	scope := "outlet yang datanya lengkap"
	if hasBlocks {
		if out.PanelShare != nil {
			scope = fmt.Sprintf("%d dari %d outlet yang datanya lengkap (%s%% omzet grup)",
				len(panelCodes), len(out.Outlets), bizNum(*out.PanelShare))
		} else {
			scope = fmt.Sprintf("%d dari %d outlet yang datanya lengkap", len(panelCodes), len(out.Outlets))
		}
	}
	var recentWeeks, prevWeeks []string
	if hasBlocks {
		recentWeeks = weekList[n-blockLen:]
		prevWeeks = weekList[n-2*blockLen : n-blockLen]
	}
	kalender := bizCalendarReason(cal, prevWeeks, recentWeeks)
	out.CalendarReason = kalender
	mentah := ""
	if marketGrowthRaw4 != nil && calendarEffect != nil && math.Abs(*calendarEffect) >= 0.5 {
		mentah = fmt.Sprintf(" Angka mentahnya %s; %s poin di antaranya dibuat kalender (%s).",
			bizNaikTurun(*marketGrowthRaw4), bizNum(math.Abs(*calendarEffect)), kalender)
	}
	pantau := ""
	if len(out.Watch) > 0 {
		pantau = fmt.Sprintf(" %s perlu dipantau: selisihnya mulai terlihat tetapi belum meyakinkan.", strings.Join(out.Watch, ", "))
	}

	switch {
	case !hasBlocks || countedForVerdict == 0:
		out.Verdict = models.BizVerdictNoData
		out.VerdictText = fmt.Sprintf("Data yang ada baru %d minggu penuh, dan belum sampai %d outlet yang datanya lengkap untuk saling dibandingkan. Tunggu beberapa minggu lagi supaya kesimpulannya bisa dipercaya.", n, bizMinPanel)

	case marketDown && len(out.Lagging) > 0:
		out.Verdict = models.BizVerdictBoth
		out.VerdictText = fmt.Sprintf("Ada dua hal sekaligus, dan keduanya harus dikerjakan bersamaan. Pertama, pasar memang turun dan itu bukan sekadar kalender: setelah libur dan pola hari dikoreksi, %s digabung masih %s, lebih dalam daripada naik-turun biasanya (±%s%%).%s Ini gerak bersama semua outlet, jadi sebabnya di luar outlet — cuaca, daya beli, pesaing, atau promosi yang mengendur; bagian medsos di bawah menunjukkan apakah jangkauan ikut turun. Kedua, %s tetap tertinggal dari outlet lain yang berjualan di pasar dan kalender yang sama, konsisten beberapa minggu, jadi outlet itu tetap harus dibenahi sendiri.%s",
			scope, bizNaikTurun(deref(marketGrowth4)), bizNum(deref(marketBand)), mentah, strings.Join(out.Lagging, ", "), pantau)

	case marketDown:
		out.Verdict = models.BizVerdictMarket
		out.VerdictText = fmt.Sprintf("%s, dan penurunannya BUKAN sekadar kalender: setelah libur dan pola hari dikoreksi, %s digabung masih %s, lebih dalam daripada naik-turun biasanya (±%s%%).%s Tidak ada satu outlet pun yang jelas lebih buruk daripada yang lain, jadi ini bukan soal cara kerja manajer. Sebabnya perlu dicari di luar outlet — cuaca, daya beli, pesaing, atau promosi yang mengendur — dan bagian medsos di bawah menunjukkan apakah jangkauan Markom ikut turun.%s",
			bizKapital(bizArahOutlet(out.Outlets)), scope, bizNaikTurun(deref(marketGrowth4)), bizNum(deref(marketBand)), mentah, pantau)

	case len(out.Lagging) > 0:
		out.Verdict = models.BizVerdictOutlet
		out.VerdictText = fmt.Sprintf("Pasar tidak sedang turun setelah libur dan pola hari dikoreksi — %s digabung %s, masih di dalam naik-turun biasanya (±%s%%).%s Tetapi %s tertinggal dari outlet lain secara konsisten. Karena outlet yang lain baik-baik saja di pasar dan kalender yang sama, yang perlu dibenahi outlet itu sendiri, bukan promosinya.%s",
			scope, bizNaikTurun(deref(marketGrowth4)), bizNum(deref(marketBand)), mentah, strings.Join(out.Lagging, ", "), pantau)

	default:
		out.Verdict = models.BizVerdictNormal
		out.VerdictText = fmt.Sprintf("Keadaan normal. Setelah libur dan pola hari dikoreksi, %s digabung %s, masih di dalam naik-turun biasanya (±%s%%), dan tidak ada outlet yang jelas lebih baik atau lebih buruk daripada yang lain.%s Belum ada yang perlu ditindaklanjuti secara khusus.%s",
			scope, bizNaikTurun(deref(marketGrowth4)), bizNum(deref(marketBand)), mentah, pantau)
	}

	// ── Catatan kaki ────────────────────────────────────────────────────────
	if n < weeks {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Anda memilih %d minggu, tetapi data yang tersedia baru %d minggu penuh. Yang ditampilkan adalah seluruh riwayat yang ada.", weeks, n))
	}
	out.Notes = append(out.Notes, fmt.Sprintf(
		"Blok pembanding, batas wajar, dan pengali kalender selalu dihitung dari %d minggu terakhir yang tersedia (paling banyak %d), berapa pun rentang yang dipilih untuk digambar. Karena itu kesimpulan halaman ini tidak berubah ketika rentangnya digeser — rentang hanya mengubah panjang grafiknya.",
		n, bizLearnWeeks))
	out.Notes = append(out.Notes, bizCalendarNote(calModel))
	if hasBlocks {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Yang dibandingkan adalah %d minggu terakhir dengan %d minggu sebelumnya — bukan satu minggu dengan satu minggu. Blok selalu menempel di ujung riwayat; mengubah rentang analisa hanya menambah riwayat untuk menaksir batas wajar, tidak mengubah apa yang dibandingkan.",
			blockLen, blockLen))
		if blockLen == 2 {
			out.Notes = append(out.Notes,
				"Blok 2 minggu dipilih: perubahan terbaru terlihat lebih cepat, tetapi angkanya lebih goyah — batas wajar tiap outlet dan pasar sekitar 1,4 kali lebih lebar daripada blok 4 minggu, dan selisih baru divonis kalau KEDUA minggu searah. Untuk penilaian bulanan yang lebih tenang, pakai blok 4 minggu.")
		}
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Patokan tiap outlet adalah nilai tengah pertumbuhan seluruh %d outlet yang datanya lengkap (%s), termasuk outlet itu sendiri. Tiap outlet satu suara, sehingga outlet terbesar tidak bisa sendirian menentukan patokan; dan karena patokannya satu untuk semua, dua outlet yang geraknya berbeda 2 poin juga berbeda 2 poin selisihnya.",
			len(panelCodes), strings.Join(panelCodes, ", ")))
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Batas wajar tiap outlet = 2 kali galat baku selisihnya (keyakinan sekitar 95%%), dengan lantai %s poin. Deraunya ditaksir dari gabungan naik-turun mingguan outlet itu sendiri SEBELUM blok terakhir dan model derau grup yang mengecil seiring banyaknya struk — jadi outlet kecil otomatis diberi batas lebih longgar, dan kemerosotan yang sedang diuji tidak ikut melebarkan batas yang mengujinya.",
			bizNum(bizThresholdFloor)))
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Selisih baru divonis Tertinggal atau Lebih Baik kalau melewati batas wajar DAN searah pada sedikitnya %d dari %d minggu terakhir. Selisih besar yang belum searah cukup lama, atau selisih konsisten yang belum melewati batas, masuk Perlu Dipantau.",
			bizConsistencyNeed(blockLen), blockLen))
		if len(panelCodes) == bizMinPanel {
			out.Notes = append(out.Notes,
				"Outlet yang datanya lengkap hanya tiga. Dengan pembanding sesedikit itu, satu outlet yang anjlok bisa membuat dua outlet lain terlihat lebih bagus daripada sebenarnya. Pakai hasilnya sebagai petunjuk awal saja.")
		}
		if marketBand != nil {
			out.Notes = append(out.Notes, fmt.Sprintf(
				"Kata \"pasar\" di halaman ini berarti penjualan %s digabung jadi satu, pada angka setara kalender — bukan seluruh outlet, dan bukan angka mentah. Pasar baru disebut turun kalau turunnya lebih dari %s%%, karena naik-turun sebesar itu memang biasa terjadi dari minggu ke minggu tanpa sebab khusus.",
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
			"Minggu pembuka (dan penutup) yang tidak dijalani tujuh hari penuh tidak ikut dibandingkan, untuk %s. Total penjualan di tabel tetap memuat minggu tersebut.",
			strings.Join(edgeDropped, ", ")))
	}
	if len(out.GroupEvents) > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf(
			"Minggu yang tidak biasa: %s. Pada minggu itu semua outlet bergerak jauh dari kebiasaan meski libur sudah dikoreksi — biasanya cuaca, acara besar, atau gangguan — jadi angkanya jangan dipakai menilai kerja manajer.",
			strings.Join(out.GroupEvents, ", ")))
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

	// Gambar dipotong ke rentang yang diminta SEBELUM narasi dirakit, supaya
	// kalimat yang menyebut minggu pertama/terakhir memakai gambar yang sama
	// dengan yang dilihat pembaca. Blok, batas wajar, dan vonis sudah final.
	bizTrimDisplay(out, weeks)

	// Narasi dirakit paling akhir: ia hanya membaca laporan yang sudah jadi,
	// sehingga kalimatnya dijamin memakai angka yang sama dengan tabel.
	BuildBusinessNarrative(out)

	// Medsos ditempel SESUDAH narasi: BuildBusinessNarrative mengosongkan
	// Sections dan Insights saat mulai merakit.
	AttachBusinessSocial(out, weeks)

	// Model statistik (layanan Python) paling akhir; ia menambah temuan dan
	// bagian, lalu urutan temuan dirapikan sekali untuk semua.
	bizAttachModel(out, bizModelInput{
		outlets: outletsByCode, order: order, weekList: weekList, panel: panelCodes,
		calModel: calModel, cal: cal, cur: curT, blockLen: blockLen, rangeEnd: rangeEnd,
	})
	bizSortInsights(out)

	// Narasi AI paling akhir dari semuanya: ia hanya mengganti teks pada
	// laporan yang sudah final, dari cache bila ada, atau memulai pembuatannya
	// di latar tanpa menunda jawaban.
	bizApplyNarrativeAI(out)

	return out, nil
}

// bizTrimDisplay memotong deret mingguan ke `keep` minggu terakhir dan
// menyandarkan ulang indeks perjalanan supaya minggu pertama yang tergambar
// bernilai 100. Blok, batas wajar, dan vonis tidak disentuh — semuanya sudah
// dihitung dari jendela penuh.
func bizTrimDisplay(out *models.BusinessAnalysis, keep int) {
	if keep <= 0 || len(out.Group) <= keep {
		return
	}
	cut := len(out.Group) - keep
	keepFrom := out.Group[cut].WeekStart
	out.Group = out.Group[cut:]
	for i := range out.Outlets {
		o := &out.Outlets[i]
		var ws []models.BizWeek
		for _, w := range o.Weeks {
			if w.WeekStart >= keepFrom {
				ws = append(ws, w)
			}
		}
		if ws == nil {
			ws = []models.BizWeek{}
		}
		o.Weeks = ws
	}
	var kal []models.BizCalendarDay
	for _, d := range out.Calendar {
		if d.Day >= keepFrom {
			kal = append(kal, d)
		}
	}
	if kal == nil {
		kal = []models.BizCalendarDay{}
	}
	out.Calendar = kal
	shown := map[string]bool{}
	for _, g := range out.Group {
		shown[g.Label] = true
	}
	var ev []string
	for _, e := range out.GroupEvents {
		if shown[e] {
			ev = append(ev, e)
		}
	}
	out.GroupEvents = ev
	out.WeeksCount = len(out.Group)
	out.PeriodFrom = keepFrom

	// Sandar ulang indeks: minggu pertama yang punya indeks pasar = 100.
	var base float64
	for _, g := range out.Group {
		if g.Index != nil && *g.Index > 0 {
			base = *g.Index
			break
		}
	}
	if base <= 0 || base == 100 {
		return
	}
	scale := 100 / base
	for i := range out.Group {
		if out.Group[i].Index != nil {
			v := round1(*out.Group[i].Index * scale)
			out.Group[i].Index = &v
		}
	}
	for i := range out.Outlets {
		for j := range out.Outlets[i].Weeks {
			if w := &out.Outlets[i].Weeks[j]; w.Index != nil {
				v := round1(*w.Index * scale)
				w.Index = &v
			}
		}
	}
}

// bizArahOutlet menyebut berapa outlet pembanding yang turun dan yang naik
// (setara kalender): "7 dari 8 outlet pembanding turun (TS naik)". Kalimat
// "semua outlet turun" hanya boleh muncul kalau memang semuanya turun.
func bizArahOutlet(outlets []models.BizOutlet) string {
	var turun, naik []string
	for _, o := range outlets {
		if !o.InPanel || o.Growth4 == nil {
			continue
		}
		if *o.Growth4 < 0 {
			turun = append(turun, o.Code)
		} else {
			naik = append(naik, o.Code)
		}
	}
	total := len(turun) + len(naik)
	switch {
	case total == 0:
		return "pasar turun"
	case len(naik) == 0:
		return fmt.Sprintf("semua %d outlet pembanding turun", total)
	case len(turun) == 0:
		return fmt.Sprintf("semua %d outlet pembanding naik", total)
	default:
		return fmt.Sprintf("%d dari %d outlet pembanding turun (%s naik)", len(turun), total, bizDaftar(naik))
	}
}

// bizCalendarReason merangkum perbedaan kalender dua blok untuk kalimat vonis:
// "HUT RI dan Maulid Nabi di blok pembanding, tanpa libur di blok terakhir".
func bizCalendarReason(cal *bizCalendar, prevWeeks, recentWeeks []string) string {
	prevHol := bizHolidayNamesInWeeks(cal, prevWeeks)
	recHol := bizHolidayNamesInWeeks(cal, recentWeeks)
	prevSch := bizSchoolDaysInWeeks(cal, prevWeeks)
	recSch := bizSchoolDaysInWeeks(cal, recentWeeks)

	sisi := func(hol []string, sch int) string {
		var p []string
		if len(hol) > 0 {
			p = append(p, strings.Join(hol, ", "))
		}
		if sch > 0 {
			p = append(p, fmt.Sprintf("%d hari libur sekolah", sch))
		}
		if len(p) == 0 {
			return "tanpa libur"
		}
		return strings.Join(p, " dan ")
	}
	return fmt.Sprintf("blok pembanding %s; blok terakhir %s", sisi(prevHol, prevSch), sisi(recHol, recSch))
}

// bizCalendarNote menulis catatan kaki tentang koreksi kalender dari pengali
// yang benar-benar dipakai periode ini.
func bizCalendarNote(m *bizCalModel) string {
	e := m.export()
	var b strings.Builder
	b.WriteString("Sebelum dibandingkan, tiap minggu diskalakan ke \"minggu biasa\" memakai bobot hari outlet itu sendiri (penjualan wajar tiap nama hari, dari nilai tengah hari-hari biasanya). ")
	if e.DowShare[5]+e.DowShare[6] > 0 {
		b.WriteString(fmt.Sprintf("Sabtu dan Minggu menyumbang %s%% omzet minggu biasa di grup ini. ", bizNum(e.DowShare[5]+e.DowShare[6])))
	}
	if e.HolidayEstimated {
		b.WriteString(fmt.Sprintf("Hari libur nasional/cuti bersama yang jatuh di hari kerja dihitung %s kali hari kerja biasanya — dipelajari dari %d hari libur yang teramati. ",
			strings.Replace(fmt.Sprintf("%.2f", e.HolidayMult), ".", ",", 1), e.HolidayObs))
	} else {
		b.WriteString("Hari libur nasional/cuti bersama yang jatuh di hari kerja dianggap seperti hari Minggu (belum ada cukup hari libur yang teramati untuk mempelajarinya). ")
	}
	if e.SchoolEstimated {
		b.WriteString(fmt.Sprintf("Hari libur sekolah dihitung %s kali di hari kerja dan %s kali di akhir pekan (dari %d dan %d hari yang teramati). ",
			strings.Replace(fmt.Sprintf("%.2f", e.SchoolWeekdayMult), ".", ",", 1),
			strings.Replace(fmt.Sprintf("%.2f", e.SchoolWeekendMult), ".", ",", 1),
			e.SchoolWeekdayObs, e.SchoolWeekendObs))
	} else {
		b.WriteString("Libur sekolah belum dikoreksi karena hari yang teramati belum cukup. ")
	}
	if e.CoverageUntil != "" {
		b.WriteString(fmt.Sprintf("Kalender terisi sampai %s; minggu setelah itu berjalan tanpa koreksi libur sampai kalendernya dilengkapi.", e.CoverageUntil))
	} else {
		b.WriteString("Kalender libur masih kosong, jadi belum ada koreksi libur yang bisa dilakukan.")
	}
	return b.String()
}
