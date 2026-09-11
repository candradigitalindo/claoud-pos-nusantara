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
// dihitung, bukan ditulis tetap di UI. Alasannya bukan kerapian: kalimat tetap
// akan terus berbunyi sama ketika keadaannya sudah berubah, sehingga halaman
// menjadi salah tanpa ada yang menyadarinya. Kalimat yang dirakit dari angka
// selalu ikut berubah bersama angkanya, atau tidak muncul sama sekali.
//
// Fungsi di berkas ini hanya membaca *models.BusinessAnalysis yang sudah jadi —
// tidak menyentuh basis data dan tidak menghitung ulang apa pun. Dengan begitu
// angka di kalimat dijamin sama dengan angka di tabel dan grafik.

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

// bizFindOutlet mencari outlet berdasarkan kode.
func bizFindOutlet(rep *models.BusinessAnalysis, code string) *models.BizOutlet {
	for i := range rep.Outlets {
		if rep.Outlets[i].Code == code {
			return &rep.Outlets[i]
		}
	}
	return nil
}

// bizMarketDown menurunkan ulang kesimpulan "pasar sedang turun" dari angka
// yang sudah ada di laporan, supaya narasi tidak pernah berbeda dengan vonis.
func bizMarketDown(rep *models.BusinessAnalysis) bool {
	return rep.GroupGrowth4 != nil && *rep.GroupGrowth4 < -deref(rep.MarketBand)
}

// BuildBusinessNarrative mengisi Headline, Subhead, Insights, Sections, dan
// Glossary pada laporan yang sudah dihitung.
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
		rep.Headline = fmt.Sprintf("Pasar %s, dan %s tertinggal", gerak, bizDaftar(rep.Lagging))

	case models.BizVerdictMarket:
		rep.Headline = fmt.Sprintf("Semua outlet %s bersama-sama", gerak)

	case models.BizVerdictOutlet:
		rep.Headline = fmt.Sprintf("%s tertinggal, pasarnya sendiri baik-baik saja", bizDaftar(rep.Lagging))

	default:
		rep.Headline = fmt.Sprintf("Keadaan normal, pasar %s", gerak)
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
			fmt.Sprintf("Kalau %s bergerak seperti outlet lain, penjualan grup %s lebih tinggi. Angka ini yang paling pantas dikejar lebih dulu.",
				bizDaftar(kodeHilang), bizRupiah(hilang)),
			kodeHilang, &amt)
	}

	// 2. Outlet unggul — dijadikan bahan tiru, bukan sekadar pujian.
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
		body := fmt.Sprintf("%s tumbuh jauh di atas outlet lain pada pasar yang sama.", bizDaftar(kodeUnggul))
		if lebih > 0 {
			v := lebih
			amt = &v
			body = fmt.Sprintf("%s menghasilkan %s lebih banyak daripada seandainya ia bergerak seperti outlet lain.",
				bizDaftar(kodeUnggul), bizRupiah(lebih))
		}
		add(models.BizInsightChance, "Ada cara kerja yang layak ditiru",
			body+" Gali apa yang mereka ubah belakangan, lalu terapkan di outlet lain.", kodeUnggul, amt)
	}

	// 3. Penggerak yang sama di banyak outlet. Kalau sebagian besar outlet
	//    kehilangan pengunjung, itu pertanda program penarik kunjungan yang
	//    kurang — bukan pekerjaan satu manajer.
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
		add(models.BizInsightWarning,
			fmt.Sprintf("%s kehilangan pengunjung, bukan nilai belanja", bizPlural(n, "outlet")),
			fmt.Sprintf("Di %s jumlah struknya yang turun, sementara rata-rata belanja tiap struk relatif bertahan. Kalau polanya sama di banyak outlet, yang kurang biasanya program penarik kunjungan — bukan cara melayani di kasir.",
				bizDaftar(kurangTamu)), kurangTamu, nil)
	}
	if n := len(kecilBelanja); n >= 2 {
		add(models.BizInsightWarning,
			fmt.Sprintf("%s: orang tetap datang, belanjanya mengecil", bizPlural(n, "outlet")),
			fmt.Sprintf("Di %s jumlah struk bertahan tetapi nilai tiap struk turun. Periksa ketersediaan menu andalan, penawaran tambahan oleh kasir, dan paket hemat.",
				bizDaftar(kecilBelanja)), kecilBelanja, nil)
	}

	// 4. Outlet yang belum bisa dinilai — supaya tidak terbaca sebagai "aman".
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

	// 5. Outlet yang belum ikut jadi pembanding.
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

	// 6. Ketergantungan pada satu outlet.
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

	// 7. Minggu tidak biasa.
	if n := len(rep.GroupEvents); n > 0 {
		add(models.BizInsightNeutral, fmt.Sprintf("%s bergerak di luar kebiasaan", bizPlural(n, "minggu")),
			fmt.Sprintf("Pada %s semua outlet bergerak bersamaan jauh dari kebiasaan — biasanya libur panjang, cuaca, atau acara besar. Angka minggu itu jangan dipakai menilai kerja manajer.",
				bizDaftar(rep.GroupEvents)), nil, nil)
	}

	// 8. Riwayat masih pendek.
	if rep.WeeksCount < rep.WeeksRequested {
		add(models.BizInsightNeutral, "Riwayat masih lebih pendek daripada yang diminta",
			fmt.Sprintf("Anda meminta %s, data yang ada baru %s penuh, dan yang dibandingkan %s terakhir lawan %s sebelumnya. Makin panjang riwayatnya, makin sempit batas wajarnya dan makin tajam kesimpulannya.",
				bizPlural(rep.WeeksRequested, "minggu"), bizPlural(rep.WeeksCount, "minggu"),
				bizPlural(rep.BlockWeeks, "minggu"), bizPlural(rep.BlockWeeks, "minggu")), nil, nil)
	}

	// Urutan tayang: yang bisa ditindak lebih dulu, lalu nilai rupiahnya.
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
	leadPasar := "Gabungan penjualan seluruh outlet, minggu demi minggu."
	if min != nil && max != nil {
		leadPasar = fmt.Sprintf("Sepanjang %s ini gerak pasar berayun antara %s sampai %s per minggu. Ayunan selebar itu memang biasa, karena itu pasar baru disebut benar-benar turun kalau lebih dari %s%%.",
			bizPlural(rep.WeeksCount, "minggu"), bizNaikTurun(*min), bizNaikTurun(*max), band)
	}
	hintPasar := "Kalau semua outlet turun bersamaan, penyebabnya dari luar — bukan salah satu manajer."
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
	leadSelisih := fmt.Sprintf("Dari %s yang bisa dinilai, %s. Sisanya masih di dalam batas wajarnya masing-masing.",
		bizPlural(nDinilai, "outlet"), strings.Join(bagian, " dan "))
	if nKanan == 0 && nKiri == 0 {
		leadSelisih = fmt.Sprintf("Dari %s yang bisa dinilai, tidak ada satu pun yang keluar dari batas wajarnya sendiri.",
			bizPlural(nDinilai, "outlet"))
	}
	add("selisih", "Selisih Tiap Outlet dengan Outlet Lain", leadSelisih,
		"Batang ke kanan berarti tumbuh lebih cepat daripada outlet lain, ke kiri lebih lambat. Patokannya nilai tengah outlet lain — outlet yang dinilai tidak ikut dihitung di dalamnya. Dua garis abu-abu adalah batas wajar outlet tersebut; yang masih di dalam garis belum berarti apa-apa.")

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
	leadTren := "Semua outlet disandingkan pada satu titik awal supaya besar-kecilnya tidak mengaburkan arah geraknya."
	if awal != "" && pasarAkhir != nil && lo != nil && hi != nil {
		leadTren = fmt.Sprintf("Titik awalnya minggu %s = 100. Sampai minggu terakhir pasar berada di %s, sementara outlet terentang dari %s (%s) sampai %s (%s).",
			awal, bizNum(*pasarAkhir),
			bizNum(*lo.Weeks[len(lo.Weeks)-1].Index), lo.Code,
			bizNum(*hi.Weeks[len(hi.Weeks)-1].Index), hi.Code)
	}
	add("perjalanan", "Perjalanan Penjualan Tiap Outlet", leadTren,
		"Angka 120 berarti naik 20% sejak titik awal, 80 berarti turun 20%. Garis hitam putus-putus adalah pasar; yang menjauh ke bawah dari garis itu tertinggal. Outlet yang baru ikut terhitung di tengah periode digambar mulai dari level pasar saat itu, bukan dari 100, supaya perjalanannya tetap sebanding.")

	// Peta posisi.
	add("peta", "Peta Posisi Outlet",
		fmt.Sprintf("Tiap titik satu outlet. Sumbu mendatar selisih dengan outlet lain, sumbu tegak naik-turun penjualannya sendiri selama %s terakhir.",
			bizPlural(rep.BlockWeeks, "minggu")),
		"Yang perlu diperhatikan pojok kiri bawah: turun, dan turunnya sendirian. Kalau semua titik berkumpul di dekat garis tegak, artinya semua outlet senasib — itu urusan pasar dan promosi.")

	// Tabel rincian.
	add("rincian", "Rincian per Outlet",
		fmt.Sprintf("Seluruh %s berjajar untuk dibandingkan sekaligus, diurutkan dari yang paling unggul.",
			bizPlural(len(rep.Outlets), "outlet")),
		"Kolom \"Outlet Lain\" adalah patokan yang dipakai untuk baris tersebut. Angkanya berbeda sedikit tiap baris karena outlet yang sedang dinilai selalu dikeluarkan dari patokannya sendiri.")

	// Laporan detail.
	add("detail", "Laporan Detail per Outlet",
		"Angka satu outlet dibuka satu per satu, lengkap dengan penjelasan dan langkah yang disarankan.",
		"Bagian ini dibuat untuk dibaca manajer outlet yang bersangkutan — semua selisih diterjemahkan ke rupiah dan jumlah struk.")

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

	// Contoh "selisih" diambil dari outlet dengan selisih terbesar mutlak,
	// supaya pembaca bisa mencocokkannya dengan baris yang ada di tabel.
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
	cSelisih, cBatas := "", ""
	if contoh != nil {
		cSelisih = fmt.Sprintf("Di periode ini %s %s sementara %s %s, jadi selisihnya %s.",
			bizPlural(contoh.PeerCount, "outlet lain"), bizNaikTurun(deref(contoh.PeerGrowth4)),
			contoh.Code, bizNaikTurun(deref(contoh.Growth4)), bizNum(math.Abs(*contoh.RGI4)))
		if contoh.Threshold != nil {
			cBatas = fmt.Sprintf("Batas wajar %s adalah %s. Selisih %s melewati batas itu, jadi bukan kebetulan.",
				contoh.Code, bizNum(*contoh.Threshold), bizNum(math.Abs(*contoh.RGI4)))
		}
	}
	add("Selisih",
		"Jarak antara gerak outlet ini dengan gerak outlet lain. Angka 0 berarti geraknya sama persis. Ini bukan rupiah dan bukan persen penjualan, melainkan jarak antara dua angka persen.",
		cSelisih)
	add("Batas wajar",
		"Penjualan tiap outlet memang naik-turun sendiri setiap minggu tanpa sebab khusus. Batas wajar adalah seberapa besar naik-turun itu biasanya terjadi di outlet tersebut. Selisih yang masih di dalam batas ini belum bisa disebut bagus atau jelek.",
		cBatas)

	cPasar := ""
	if rep.GroupGrowth4 != nil {
		cPasar = fmt.Sprintf("Periode ini pasar %s, dengan batas wajar %s%%.",
			bizNaikTurun(*rep.GroupGrowth4), bizNum(deref(rep.MarketBand)))
	}
	add("Pasar",
		"Penjualan semua outlet digabung menjadi satu angka. Dipakai untuk melihat apakah yang turun cuma satu outlet, atau semuanya sekaligus.",
		cPasar)

	cPembanding := ""
	if n := len(rep.PanelCodes); n > 0 {
		cPembanding = fmt.Sprintf("Periode ini %s berdata lengkap (%s), jadi tiap outlet dinilai melawan %s sisanya.",
			bizPlural(n, "outlet"), strings.Join(rep.PanelCodes, ", "), bizPlural(n-1, "outlet"))
	}
	add("Outlet pembanding",
		"Outlet yang penjualannya lengkap di seluruh minggu yang dibandingkan, sehingga layak dijadikan patokan. Outlet yang baru buka atau sempat tutup tetap ditampilkan, tetapi tidak ikut menentukan patokan.",
		cPembanding)

	// Contoh struk diambil dari outlet yang perubahannya paling jelas
	// digerakkan jumlah pengunjung — istilah ini paling sering disalahpahami.
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
			"Minggu ketika semua outlet bergerak jauh dari kebiasaan — biasanya libur panjang, cuaca buruk, atau ada acara besar. Angka pada minggu itu jangan dipakai menilai kerja manajer.",
			fmt.Sprintf("Periode ini ada %s: %s.", bizPlural(len(rep.GroupEvents), "minggu"), bizDaftar(rep.GroupEvents)))
	}

	cKesimpulan := ""
	if len(rep.Leading) > 0 || len(rep.Lagging) > 0 {
		bagian := []string{}
		if len(rep.Leading) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s Lebih Baik", bizDaftar(rep.Leading)))
		}
		if len(rep.Lagging) > 0 {
			bagian = append(bagian, fmt.Sprintf("%s Tertinggal", bizDaftar(rep.Lagging)))
		}
		cKesimpulan = "Periode ini: " + strings.Join(bagian, ", ") + ", sisanya Sama Saja."
	}
	add("Lebih Baik / Tertinggal / Sama Saja",
		"Kesimpulan setelah selisih dibandingkan dengan batas wajar outlet tersebut. \"Sama Saja\" berarti bedanya masih di dalam batas wajar, jadi tidak ada yang perlu dipermasalahkan.",
		cKesimpulan)

	for _, o := range rep.Outlets {
		if o.Diagnosis == models.BizDiagWeak {
			add("Belum Bisa Dinilai",
				"Penjualan outlet itu terlalu naik-turun dari minggu ke minggu untuk dinilai secara mingguan, biasanya karena jumlah struknya sedikit. Nilai outlet seperti ini per bulan saja.",
				fmt.Sprintf("Periode ini %s masuk kelompok ini, dengan batas wajar %s.", o.Code, bizNum(deref(o.Threshold))))
			break
		}
	}
}
