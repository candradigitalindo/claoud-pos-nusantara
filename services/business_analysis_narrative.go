package services

import (
	"cloud-pos/models"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ── Perakit narasi Analisa Bisnis ────────────────────────────────────────────
//
// Seluruh kalimat penjelas halaman dirakit di sini dari angka yang sudah
// dihitung, bukan ditulis tetap di UI. Kalimat tetap akan terus berbunyi sama
// ketika keadaannya sudah berubah, sehingga halaman menjadi salah tanpa ada
// yang menyadarinya. Kalimat yang dirakit dari angka selalu ikut berubah
// bersama angkanya, atau tidak muncul sama sekali.
//
// Fungsi di berkas ini hanya membaca *models.BusinessAnalysis yang sudah jadi —
// tidak menyentuh basis data dan tidak menghitung ulang apa pun.

// bizPlural menyusun frasa berjumlah tanpa kalimat bercabang di pemanggil.
func bizPlural(n int, satuan string) string {
	return fmt.Sprintf("%d %s", n, satuan)
}

// bizDaftar menyambung kode outlet menjadi frasa yang enak dibaca:
// "SB", "SB dan NF", "SB, NF, dan GD".
func bizDaftar(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	case 2:
		return xs[0] + " dan " + xs[1]
	default:
		return strings.Join(xs[:len(xs)-1], ", ") + ", dan " + xs[len(xs)-1]
	}
}

// bizMult menulis pengali dengan dua desimal berkoma: "5,20×".
func bizMult(v float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1) + "×"
}

// bizFindOutlet mencari outlet berdasarkan kode.
func bizFindOutlet(rep *models.BusinessAnalysis, code string) *models.BizOutlet {
	for i := range rep.Outlets {
		if rep.Outlets[i].Code == code {
			return &rep.Outlets[i]
		}
	}
	return nil
}

// BuildBusinessNarrative mengisi Headline, Insights, Sections, dan Glossary
// pada laporan yang sudah dihitung.
func BuildBusinessNarrative(rep *models.BusinessAnalysis) {
	rep.Insights = []models.BizInsight{}
	rep.Sections = []models.BizSection{}
	rep.Glossary = []models.BizTerm{}

	bizBuildHeadline(rep)
	bizBuildInsights(rep)
	bizBuildSections(rep)
	bizBuildGlossary(rep)
}

// ── Judul utama ──────────────────────────────────────────────────────────────

func bizBuildHeadline(rep *models.BusinessAnalysis) {
	gerak := bizNaikTurun(deref(rep.GroupGrowth4))

	switch rep.Verdict {
	case models.BizVerdictNoData:
		rep.Headline = "Belum bisa disimpulkan"

	case models.BizVerdictBoth:
		rep.Headline = fmt.Sprintf("Pasar %s setelah libur dikoreksi, dan %s tertinggal", gerak, bizDaftar(rep.Lagging))

	case models.BizVerdictMarket:
		rep.Headline = fmt.Sprintf("Pasar %s setelah libur dikoreksi; %s", gerak, bizArahOutlet(rep.Outlets))

	case models.BizVerdictOutlet:
		rep.Headline = fmt.Sprintf("%s tertinggal, pasarnya sendiri baik-baik saja", bizDaftar(rep.Lagging))

	default:
		if rep.CalendarEffect != nil && math.Abs(*rep.CalendarEffect) >= 3 && deref(rep.GroupGrowthRaw4) < 0 {
			rep.Headline = fmt.Sprintf("Keadaan normal: penurunan mentah %s sebagian besar kalender", bizNum(-deref(rep.GroupGrowthRaw4))+"%")
		} else {
			rep.Headline = fmt.Sprintf("Keadaan normal, pasar %s setelah libur dikoreksi", gerak)
		}
	}
}

// ── Temuan ───────────────────────────────────────────────────────────────────

func bizBuildInsights(rep *models.BusinessAnalysis) {
	add := func(kind, title, body string, outlets []string, amount *float64) {
		rep.Insights = append(rep.Insights, models.BizInsight{
			Kind: kind, Title: title, Body: body, Outlets: outlets, Amount: amount,
		})
	}

	// 1. Nilai rupiah yang hilang karena outlet tertinggal. Diletakkan paling
	//    depan: satu-satunya temuan yang langsung bisa diprioritaskan.
	var hilang float64
	var kodeHilang []string
	for _, o := range rep.Outlets {
		if o.Diagnosis == models.BizDiagLagging && o.GapNet != nil && *o.GapNet < 0 {
			hilang += -*o.GapNet
			kodeHilang = append(kodeHilang, o.Code)
		}
	}
	if hilang > 0 {
		amt := hilang
		add(models.BizInsightProblem,
			fmt.Sprintf("Selisih %s dalam %s terakhir", bizRupiah(hilang), bizPlural(rep.BlockWeeks, "minggu")),
			fmt.Sprintf("Kalau %s bergerak seperti outlet pembanding (dengan kalender yang sama), penjualan grup %s lebih tinggi. Angka ini yang paling pantas dikejar lebih dulu.",
				bizDaftar(kodeHilang), bizRupiah(hilang)),
			kodeHilang, &amt)
	}

	// 2. Kalender: berapa poin gerak pasar mentah yang hanyalah libur.
	if rep.CalendarEffect != nil && rep.GroupGrowthRaw4 != nil && rep.GroupGrowth4 != nil && math.Abs(*rep.CalendarEffect) >= 3 {
		sebab := ""
		if rep.CalendarReason != "" {
			sebab = " Rinciannya: " + rep.CalendarReason + "."
		}
		kind := models.BizInsightNeutral
		if rep.Verdict == models.BizVerdictNormal && deref(rep.GroupGrowthRaw4) < 0 {
			kind = models.BizInsightWarning
		}
		add(kind,
			fmt.Sprintf("Kalender menjelaskan %s poin gerak pasar", bizNum(math.Abs(*rep.CalendarEffect))),
			fmt.Sprintf("Angka mentah pasar %s; setara kalender %s. Selisihnya lahir dari libur dan pola hari, bukan dari pasar.%s Membaca angka mentah saja akan menagih Markom atas kalender.",
				bizNaikTurun(*rep.GroupGrowthRaw4), bizNaikTurun(*rep.GroupGrowth4), sebab),
			nil, nil)
	}

	// 3. Outlet unggul — dijadikan bahan tiru, bukan sekadar pujian.
	var kodeUnggul []string
	var lebih float64
	for _, o := range rep.Outlets {
		if o.Diagnosis == models.BizDiagLeading {
			kodeUnggul = append(kodeUnggul, o.Code)
			if o.GapNet != nil && *o.GapNet > 0 {
				lebih += *o.GapNet
			}
		}
	}
	if len(kodeUnggul) > 0 {
		var amt *float64
		body := fmt.Sprintf("%s tumbuh jauh di atas outlet lain pada pasar dan kalender yang sama.", bizDaftar(kodeUnggul))
		if lebih > 0 {
			v := lebih
			amt = &v
			body = fmt.Sprintf("%s menghasilkan %s lebih banyak daripada seandainya ia bergerak seperti outlet pembanding.",
				bizDaftar(kodeUnggul), bizRupiah(lebih))
		}
		add(models.BizInsightChance, "Ada cara kerja yang layak ditiru",
			body+" Gali apa yang mereka ubah belakangan, lalu terapkan di outlet lain.", kodeUnggul, amt)
	}

	// 4. Outlet yang perlu dipantau — selisih mulai terlihat, belum meyakinkan.
	if len(rep.Watch) > 0 {
		var bawah, atas []string
		for _, o := range rep.Outlets {
			if o.Diagnosis != models.BizDiagWatch {
				continue
			}
			if deref(o.RGI4) < 0 {
				bawah = append(bawah, o.Code)
			} else {
				atas = append(atas, o.Code)
			}
		}
		var bagian []string
		if len(bawah) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s mulai tertinggal", bizDaftar(bawah)))
		}
		if len(atas) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s mulai unggul", bizDaftar(atas)))
		}
		add(models.BizInsightWarning, "Perlu dipantau, belum divonis",
			fmt.Sprintf("%s. Selisihnya belum melewati batas wajar, atau baru terjadi satu-dua minggu. Cek cepat di outletnya, dan lihat lagi dua minggu ke depan sebelum menyimpulkan.",
				bizKapital(strings.Join(bagian, "; "))),
			rep.Watch, nil)
	}

	// 5. Penggerak yang sama di banyak outlet.
	var kurangTamu, kecilBelanja []string
	for _, o := range rep.Outlets {
		if o.PrevTrx <= 0 || o.RecentTrx <= 0 {
			continue
		}
		switch bizDriver(o.PrevNet, o.RecentNet, o.PrevTrx, o.RecentTrx) {
		case "PENGUNJUNG":
			if o.RecentTrx < o.PrevTrx {
				kurangTamu = append(kurangTamu, o.Code)
			}
		case "BELANJA":
			if o.RecentNet*float64(o.PrevTrx) < o.PrevNet*float64(o.RecentTrx) {
				kecilBelanja = append(kecilBelanja, o.Code)
			}
		}
	}
	if n := len(kurangTamu); n >= 2 {
		kal := ""
		if rep.CalendarEffect != nil && *rep.CalendarEffect <= -3 {
			kal = " Sebagian dari berkurangnya pengunjung ini adalah kalender — blok pembanding punya libur yang blok terakhir tidak punya — jadi bandingkan dengan kolom setara kalender sebelum menyimpulkan."
		}
		add(models.BizInsightWarning,
			fmt.Sprintf("%s kehilangan pengunjung, bukan nilai belanja", bizPlural(n, "outlet")),
			fmt.Sprintf("Di %s jumlah struknya yang turun, sementara rata-rata belanja tiap struk relatif bertahan.%s Kalau polanya sama di banyak outlet setelah kalender dikoreksi, yang kurang biasanya program penarik kunjungan — bukan cara melayani di kasir.",
				bizDaftar(kurangTamu), kal), kurangTamu, nil)
	}
	if n := len(kecilBelanja); n >= 2 {
		add(models.BizInsightWarning,
			fmt.Sprintf("%s: orang tetap datang, belanjanya mengecil", bizPlural(n, "outlet")),
			fmt.Sprintf("Di %s jumlah struk bertahan tetapi nilai tiap struk turun. Periksa ketersediaan menu andalan, penawaran tambahan oleh kasir, dan paket hemat.",
				bizDaftar(kecilBelanja)), kecilBelanja, nil)
	}

	// 6. Outlet yang belum bisa dinilai — supaya tidak terbaca sebagai "aman".
	var lemah []string
	for _, o := range rep.Outlets {
		if o.Diagnosis == models.BizDiagWeak {
			lemah = append(lemah, o.Code)
		}
	}
	if len(lemah) > 0 {
		add(models.BizInsightWarning, "Ada outlet yang belum bisa dinilai mingguan",
			fmt.Sprintf("Penjualan %s terlalu naik-turun antar minggu untuk dibandingkan dengan adil, biasanya karena jumlah struknya sedikit. Jangan dibaca sebagai aman — nilai outlet ini per bulan.",
				bizDaftar(lemah)), lemah, nil)
	}

	// 7. Outlet yang belum ikut jadi pembanding.
	var luar []string
	for _, o := range rep.Outlets {
		if !o.InPanel {
			luar = append(luar, o.Code)
		}
	}
	if len(luar) > 0 {
		add(models.BizInsightNeutral, "Ada outlet yang belum ikut jadi pembanding",
			fmt.Sprintf("%s belum punya penjualan lengkap di seluruh minggu yang dibandingkan — biasanya karena baru buka atau sempat tutup. Angkanya tetap ditampilkan, tetapi tidak ikut menentukan patokan bagi outlet lain.",
				bizDaftar(luar)), luar, nil)
	}

	// 8. Ketergantungan pada satu outlet.
	var total float64
	for _, o := range rep.Outlets {
		total += o.Net
	}
	if total > 0 && len(rep.Outlets) >= 3 {
		top := rep.Outlets[0]
		for _, o := range rep.Outlets {
			if o.Net > top.Net {
				top = o
			}
		}
		if share := top.Net / total; share >= 0.35 {
			add(models.BizInsightWarning, "Penjualan grup bertumpu pada satu outlet",
				fmt.Sprintf("%s menyumbang %s%% dari seluruh penjualan grup. Ketika satu outlet sebesar ini goyang, angka grup ikut goyang meskipun outlet lain baik-baik saja.",
					top.Code, bizNum(math.Round(share*1000)/10)), []string{top.Code}, nil)
		}
	}

	// 9. Minggu tidak biasa.
	if n := len(rep.GroupEvents); n > 0 {
		add(models.BizInsightNeutral, fmt.Sprintf("%s bergerak di luar kebiasaan", bizPlural(n, "minggu")),
			fmt.Sprintf("Pada %s semua outlet bergerak bersamaan jauh dari kebiasaan meski libur sudah dikoreksi — biasanya cuaca, acara besar, atau gangguan. Angka minggu itu jangan dipakai menilai kerja manajer.",
				bizDaftar(rep.GroupEvents)), nil, nil)
	}

	// 10. Kalender yang belum menjangkau periode.
	if rep.CalendarModel != nil && rep.CalendarModel.CoverageUntil != "" && rep.CalendarModel.CoverageUntil < rep.PeriodTo {
		add(models.BizInsightWarning, "Kalender libur belum menjangkau seluruh periode",
			fmt.Sprintf("Kalender terisi sampai %s, sementara periode ini berakhir %s. Minggu sesudah tanggal itu dibandingkan tanpa koreksi libur. Lengkapi kalender di bagian bawah halaman.",
				rep.CalendarModel.CoverageUntil, rep.PeriodTo), nil, nil)
	}

	// 11. Riwayat masih pendek.
	if rep.WeeksCount < rep.WeeksRequested {
		add(models.BizInsightNeutral, "Riwayat masih lebih pendek daripada yang diminta",
			fmt.Sprintf("Anda meminta %s, data yang ada baru %s penuh. Yang dibandingkan tetap %s terakhir lawan %s sebelumnya, dan batas wajarnya dihitung dari seluruh %s yang tersedia — rentang hanya mengubah panjang gambar.",
				bizPlural(rep.WeeksRequested, "minggu"), bizPlural(rep.WeeksCount, "minggu"),
				bizPlural(rep.BlockWeeks, "minggu"), bizPlural(rep.BlockWeeks, "minggu"), bizPlural(rep.WindowWeeks, "minggu")), nil, nil)
	}

	bizSortInsights(rep)
}

// bizSortInsights mengurutkan temuan: yang bisa ditindak lebih dulu, lalu
// nilai rupiahnya. Dipanggil lagi setelah blok medsos dan model menambah
// temuannya, supaya urutan akhirnya satu untuk semua sumber.
func bizSortInsights(rep *models.BusinessAnalysis) {
	prioritas := map[string]int{
		models.BizInsightProblem: 0,
		models.BizInsightChance:  1,
		models.BizInsightWarning: 2,
		models.BizInsightNeutral: 3,
	}
	sort.SliceStable(rep.Insights, func(i, j int) bool {
		a, b := rep.Insights[i], rep.Insights[j]
		if prioritas[a.Kind] != prioritas[b.Kind] {
			return prioritas[a.Kind] < prioritas[b.Kind]
		}
		return deref(a.Amount) > deref(b.Amount)
	})
}

func bizKapital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ── Judul & penjelasan tiap bagian ──────────────────────────────────────────

func bizBuildSections(rep *models.BusinessAnalysis) {
	add := func(key, title, lead, hint string) {
		rep.Sections = append(rep.Sections, models.BizSection{
			Key: key, Title: title, Lead: lead, Hint: hint,
		})
	}
	band := bizNum(deref(rep.MarketBand))

	// Rentang gerak pasar sepanjang periode — dipakai menjelaskan kenapa
	// batas wajarnya selebar itu.
	var min, max *float64
	for _, g := range rep.Group {
		if g.Growth == nil {
			continue
		}
		if min == nil || *g.Growth < *min {
			v := *g.Growth
			min = &v
		}
		if max == nil || *g.Growth > *max {
			v := *g.Growth
			max = &v
		}
	}
	leadPasar := "Gabungan penjualan seluruh outlet pembanding, minggu demi minggu, setelah libur dikoreksi."
	if min != nil && max != nil {
		leadPasar = fmt.Sprintf("Sepanjang %s ini gerak pasar setara kalender berayun antara %s sampai %s dari satu minggu ke minggu berikutnya. Karena ayunan mingguan sebesar itu biasa, gerak %d minggu lawan %d minggu sebelumnya baru disebut turun kalau lebih dalam dari %s%%.",
			bizPlural(rep.WeeksCount, "minggu"), bizNaikTurun(*min), bizNaikTurun(*max), rep.BlockWeeks, rep.BlockWeeks, band)
	}
	hintPasar := "Batang = gerak pasar setara kalender: minggu berlibur sudah diskalakan ke minggu biasa, jadi lonjakan libur dan kejatuhan sesudahnya tidak muncul di sini. Kalau semua outlet tetap turun bersamaan, penyebabnya dari luar — bukan salah satu manajer. Angka mentahnya ada di tooltip."
	if len(rep.GroupEvents) > 0 {
		hintPasar += fmt.Sprintf(" Batang oranye menandai %s yang bergerak jauh dari kebiasaan.",
			bizPlural(len(rep.GroupEvents), "minggu"))
	}
	add("pasar", "Gerak Pasar per Minggu", leadPasar, hintPasar)

	// Selisih tiap outlet.
	nDinilai, nKanan, nKiri := 0, len(rep.Leading), len(rep.Lagging)
	for _, o := range rep.Outlets {
		if o.RGI4 != nil {
			nDinilai++
		}
	}
	bagian := []string{}
	if nKanan > 0 {
		bagian = append(bagian, fmt.Sprintf("%s keluar ke kanan (%s)", bizPlural(nKanan, "outlet"), bizDaftar(rep.Leading)))
	}
	if nKiri > 0 {
		bagian = append(bagian, fmt.Sprintf("%s keluar ke kiri (%s)", bizPlural(nKiri, "outlet"), bizDaftar(rep.Lagging)))
	}
	if len(rep.Watch) > 0 {
		bagian = append(bagian, fmt.Sprintf("%s perlu dipantau (%s)", bizPlural(len(rep.Watch), "outlet"), bizDaftar(rep.Watch)))
	}
	leadSelisih := fmt.Sprintf("Dari %s yang bisa dinilai, %s. Sisanya masih di dalam batas wajarnya masing-masing.",
		bizPlural(nDinilai, "outlet"), strings.Join(bagian, "; "))
	if len(bagian) == 0 {
		leadSelisih = fmt.Sprintf("Dari %s yang bisa dinilai, tidak ada satu pun yang keluar dari batas wajarnya sendiri secara konsisten.",
			bizPlural(nDinilai, "outlet"))
	}
	add("selisih", "Selisih Tiap Outlet dengan Outlet Lain", leadSelisih,
		"Batang ke kanan berarti tumbuh lebih cepat daripada outlet pembanding, ke kiri lebih lambat — keduanya pada angka setara kalender. Patokannya nilai tengah seluruh outlet pembanding, satu untuk semua. Dua garis abu-abu adalah batas wajar outlet tersebut; yang masih di dalam garis belum berarti apa-apa, dan yang melewati garis pun baru divonis kalau searah hampir tiap minggu.")

	// Perjalanan indeks.
	var awal string
	var pasarAkhir *float64
	for _, g := range rep.Group {
		if g.Index == nil {
			continue
		}
		if awal == "" {
			awal = g.Label
		}
		pasarAkhir = g.Index
	}
	var lo, hi *models.BizOutlet
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		if len(o.Weeks) == 0 || o.Weeks[len(o.Weeks)-1].Index == nil {
			continue
		}
		if lo == nil || *o.Weeks[len(o.Weeks)-1].Index < *lo.Weeks[len(lo.Weeks)-1].Index {
			lo = o
		}
		if hi == nil || *o.Weeks[len(o.Weeks)-1].Index > *hi.Weeks[len(hi.Weeks)-1].Index {
			hi = o
		}
	}
	leadTren := "Semua outlet disandingkan pada satu titik awal, pada angka setara kalender, supaya besar-kecilnya dan libur tidak mengaburkan arah geraknya."
	if awal != "" && pasarAkhir != nil && lo != nil && hi != nil {
		leadTren = fmt.Sprintf("Titik awalnya minggu %s = 100. Sampai minggu terakhir pasar berada di %s, sementara outlet terentang dari %s (%s) sampai %s (%s) — semuanya setara kalender.",
			awal, bizNum(*pasarAkhir),
			bizNum(*lo.Weeks[len(lo.Weeks)-1].Index), lo.Code,
			bizNum(*hi.Weeks[len(hi.Weeks)-1].Index), hi.Code)
	}
	add("perjalanan", "Perjalanan Penjualan Tiap Outlet", leadTren,
		"Angka 120 berarti naik 20% sejak titik awal, 80 berarti turun 20%. Garis hitam putus-putus adalah pasar; yang menjauh ke bawah dari garis itu tertinggal. Outlet yang baru ikut terhitung di tengah periode digambar mulai dari level pasar saat itu, bukan dari 100, supaya perjalanannya tetap sebanding.")

	// Peta posisi.
	add("peta", "Peta Posisi Outlet",
		fmt.Sprintf("Tiap titik satu outlet. Sumbu mendatar selisih dengan outlet pembanding, sumbu tegak naik-turun penjualan setara kalendernya sendiri selama %s terakhir.",
			bizPlural(rep.BlockWeeks, "minggu")),
		"Yang perlu diperhatikan pojok kiri bawah: turun, dan turunnya sendirian. Kalau semua titik berkumpul di dekat garis tegak, artinya semua outlet senasib — itu urusan pasar, bukan manajer.")

	// Tabel rincian.
	add("rincian", "Rincian per Outlet",
		fmt.Sprintf("Seluruh %s berjajar untuk dibandingkan sekaligus, diurutkan dari yang paling unggul.",
			bizPlural(len(rep.Outlets), "outlet")),
		"Kolom \"Naik / Turun\" adalah angka setara kalender, dengan angka mentahnya di bawahnya. Kolom \"Outlet Lain\" adalah nilai tengah seluruh outlet pembanding — satu patokan untuk semua baris.")

	// Laporan detail.
	add("detail", "Laporan Detail per Outlet",
		"Angka satu outlet dibuka satu per satu, lengkap dengan penjelasan dan langkah yang disarankan.",
		"Bagian ini dibuat untuk dibaca manajer outlet yang bersangkutan — semua selisih diterjemahkan ke rupiah dan jumlah struk. Kolom \"Setara Kalender\" menunjukkan berapa penjualan minggu itu kalau tidak ada libur.")

	// Kalender.
	leadKal := "Hari libur, cuti bersama, libur sekolah, dan kejadian lokal yang dipakai menyetarakan minggu."
	if m := rep.CalendarModel; m != nil {
		var bagianKal []string
		if m.HolidayEstimated {
			bagianKal = append(bagianKal, fmt.Sprintf("hari libur di hari kerja dihitung %s hari kerja biasa (dipelajari dari %d hari libur yang teramati)", bizMult(m.HolidayMult), m.HolidayObs))
		} else {
			bagianKal = append(bagianKal, "hari libur di hari kerja dianggap seperti hari Minggu (belum cukup hari libur yang teramati)")
		}
		if m.SchoolEstimated {
			bagianKal = append(bagianKal, fmt.Sprintf("libur sekolah dihitung %s di hari kerja dan %s di akhir pekan", bizMult(m.SchoolWeekdayMult), bizMult(m.SchoolWeekendMult)))
		} else {
			bagianKal = append(bagianKal, "libur sekolah belum dikoreksi karena pengamatannya belum cukup")
		}
		leadKal = fmt.Sprintf("Dalam periode ini ada %s bertanda. Pengali yang dipakai: %s.", bizPlural(len(rep.Calendar), "hari"), strings.Join(bagianKal, "; "))
		if m.CoverageUntil != "" {
			leadKal += fmt.Sprintf(" Kalender terisi sampai %s.", m.CoverageUntil)
		}
	}
	add("kalender", "Kalender Libur & Hari Khusus", leadKal,
		"Kalender ini ikut menentukan vonis: minggu yang memuat libur diskalakan ke minggu biasa sebelum dibandingkan. Tambahkan libur daerah, libur sekolah setempat, atau kejadian lokal (jalan ditutup, festival, cuaca ekstrem) yang belum ada — dan hapus yang ternyata tidak terjadi. Isi bawaan bersumber SKB 3 Menteri dan perkiraan libur sekolah, jadi perlu diperiksa.")

	add("istilah", "Arti Istilah di Halaman Ini",
		"Dibaca sekali saja sudah cukup. Contoh pada tiap istilah diambil dari angka periode ini.", "")

	add("metode", "Cara Angka Ini Dihitung",
		"Catatan untuk yang ingin menelusuri asal angkanya.", "")
}

// ── Kamus istilah, dengan contoh dari data periode ini ──────────────────────

func bizBuildGlossary(rep *models.BusinessAnalysis) {
	add := func(term, meaning, example string) {
		rep.Glossary = append(rep.Glossary, models.BizTerm{
			Term: term, Meaning: meaning, Example: example,
		})
	}

	// Contoh "setara kalender": minggu dengan faktor paling jauh dari 1.
	cKal := ""
	var contohW *models.BizWeek
	var contohO *models.BizOutlet
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		for j := range o.Weeks {
			w := &o.Weeks[j]
			if w.CalFactor <= 0 || w.Net <= 0 {
				continue
			}
			if contohW == nil || math.Abs(w.CalFactor-1) > math.Abs(contohW.CalFactor-1) {
				contohW, contohO = w, o
			}
		}
	}
	if contohW != nil && math.Abs(contohW.CalFactor-1) >= 0.02 {
		cKal = fmt.Sprintf("Minggu %s di %s (%s) menjual %s; sebagai minggu biasa nilainya setara %s.",
			contohW.Label, contohO.Code, contohW.Calendar, bizRupiah(contohW.Net), bizRupiah(contohW.NetAdj))
	}
	add("Setara kalender",
		"Penjualan satu minggu setelah libur dan pola hari dikoreksi: minggu yang memuat hari libur atau libur sekolah diskalakan ke minggu biasa memakai bobot hari outlet itu sendiri. Semua perbandingan di halaman ini memakai angka ini; angka sebenarnya tetap ditampilkan di sampingnya.",
		cKal)

	// Contoh "selisih" diambil dari outlet dengan selisih terbesar mutlak.
	var contoh *models.BizOutlet
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		if o.RGI4 == nil {
			continue
		}
		if contoh == nil || math.Abs(*o.RGI4) > math.Abs(*contoh.RGI4) {
			contoh = o
		}
	}
	cSelisih, cBatas, cKonsisten := "", "", ""
	if contoh != nil {
		cSelisih = fmt.Sprintf("Di periode ini nilai tengah %s %s sementara %s %s, jadi selisihnya %s poin.",
			bizPlural(contoh.PeerCount, "outlet pembanding"), bizNaikTurun(deref(contoh.PeerGrowth4)),
			contoh.Code, bizNaikTurun(deref(contoh.Growth4)), bizNum(math.Abs(*contoh.RGI4)))
		if contoh.Threshold != nil {
			lewat := "melewati"
			if math.Abs(*contoh.RGI4) < *contoh.Threshold {
				lewat = "masih di dalam"
			}
			cBatas = fmt.Sprintf("Batas wajar %s adalah %s poin. Selisih %s %s batas itu.",
				contoh.Code, bizNum(*contoh.Threshold), bizNum(math.Abs(*contoh.RGI4)), lewat)
		}
		if contoh.ConsistencyNeed > 0 {
			cKonsisten = fmt.Sprintf("%s searah pada %d dari %d minggu terakhir; syaratnya %d.",
				contoh.Code, contoh.Consistency, rep.BlockWeeks, contoh.ConsistencyNeed)
		}
	}
	add("Selisih",
		"Jarak antara gerak outlet ini dengan nilai tengah gerak seluruh outlet pembanding, pada angka setara kalender. Angka 0 berarti geraknya sama persis. Ini bukan rupiah dan bukan persen penjualan, melainkan jarak antara dua angka persen.",
		cSelisih)
	add("Batas wajar",
		"Penjualan tiap outlet memang naik-turun sendiri setiap minggu tanpa sebab khusus. Batas wajar adalah dua kali galat baku selisih outlet itu — ditaksir dari naik-turun mingguannya sendiri sebelum blok terakhir, digabung model derau grup yang mengecil seiring banyaknya struk. Selisih di dalam batas ini belum bisa disebut bagus atau jelek.",
		cBatas)
	add("Konsisten",
		"Selisih baru divonis kalau arahnya sama pada hampir semua minggu di blok terakhir. Satu minggu buruk yang menyeret satu blok bukan kemerosotan; empat minggu yang sama-sama di bawah barulah pola.",
		cKonsisten)

	cPasar := ""
	if rep.GroupGrowth4 != nil {
		cPasar = fmt.Sprintf("Periode ini pasar %s setara kalender (mentah %s), dengan batas wajar %s%%.",
			bizNaikTurun(*rep.GroupGrowth4), bizNaikTurun(deref(rep.GroupGrowthRaw4)), bizNum(deref(rep.MarketBand)))
	}
	add("Pasar",
		"Penjualan seluruh outlet pembanding digabung menjadi satu angka, setara kalender. Dipakai untuk melihat apakah yang turun cuma satu outlet, atau semuanya sekaligus — dan apakah turunnya nyata atau hanya karena bulan lalu ada libur.",
		cPasar)

	cPembanding := ""
	if n := len(rep.PanelCodes); n > 0 {
		cPembanding = fmt.Sprintf("Periode ini %s berdata lengkap (%s); nilai tengah pertumbuhan mereka menjadi satu patokan untuk semua.",
			bizPlural(n, "outlet"), strings.Join(rep.PanelCodes, ", "))
	}
	add("Outlet pembanding",
		"Outlet yang penjualannya lengkap di seluruh minggu yang dibandingkan, sehingga layak dijadikan patokan. Patokannya nilai tengah pertumbuhan mereka semua — tiap outlet satu suara, termasuk outlet yang sedang dinilai. Outlet yang baru buka atau sempat tutup tetap ditampilkan, tetapi tidak ikut menentukan patokan.",
		cPembanding)

	cStruk := ""
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		if o.PrevTrx > 0 && o.RecentTrx > 0 &&
			bizDriver(o.PrevNet, o.RecentNet, o.PrevTrx, o.RecentTrx) == "PENGUNJUNG" {
			cStruk = fmt.Sprintf("Contohnya %s: struk %d menjadi %d, sementara rata-rata belanja tiap struk hampir tidak berubah — jadi yang berkurang orangnya, bukan belanjanya.",
				o.Code, o.PrevTrx, o.RecentTrx)
			break
		}
	}
	add("Struk",
		"Jumlah transaksi yang tercatat di kasir — kira-kira sama dengan jumlah pembeli. Kalau penjualan turun, lihat dulu apakah struknya yang berkurang atau rata-rata belanjanya yang mengecil; keduanya ditangani dengan cara berbeda.",
		cStruk)

	if len(rep.GroupEvents) > 0 {
		add("Minggu tidak biasa",
			"Minggu ketika semua outlet bergerak jauh dari kebiasaan meski libur sudah dikoreksi — biasanya cuaca buruk, acara besar, atau gangguan. Angka pada minggu itu jangan dipakai menilai kerja manajer.",
			fmt.Sprintf("Periode ini ada %s: %s.", bizPlural(len(rep.GroupEvents), "minggu"), bizDaftar(rep.GroupEvents)))
	}

	cKesimpulan := ""
	{
		bagian := []string{}
		if len(rep.Leading) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s Lebih Baik", bizDaftar(rep.Leading)))
		}
		if len(rep.Lagging) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s Tertinggal", bizDaftar(rep.Lagging)))
		}
		if len(rep.Watch) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s Perlu Dipantau", bizDaftar(rep.Watch)))
		}
		if len(bagian) > 0 {
			cKesimpulan = "Periode ini: " + strings.Join(bagian, ", ") + ", sisanya Sama Saja."
		}
	}
	add("Lebih Baik / Tertinggal / Perlu Dipantau / Sama Saja",
		"Kesimpulan setelah selisih dibandingkan dengan batas wajar dan konsistensinya. \"Lebih Baik\" dan \"Tertinggal\" berarti melewati batas wajar dan searah hampir tiap minggu. \"Perlu Dipantau\" berarti selisihnya mulai terlihat tetapi belum memenuhi salah satu syarat itu. \"Sama Saja\" berarti bedanya masih di dalam batas wajar.",
		cKesimpulan)

	for _, o := range rep.Outlets {
		if o.Diagnosis == models.BizDiagWeak {
			add("Belum Bisa Dinilai",
				"Penjualan outlet itu terlalu naik-turun dari minggu ke minggu untuk dinilai secara mingguan, biasanya karena jumlah struknya sedikit. Nilai outlet seperti ini per bulan saja.",
				fmt.Sprintf("Periode ini %s masuk kelompok ini, dengan batas wajar %s poin.", o.Code, bizNum(deref(o.Threshold))))
			break
		}
	}
}
