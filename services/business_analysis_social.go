package services

import (
	"cloud-pos/models"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

// ── Medsos di dalam Analisa Bisnis ───────────────────────────────────────────
//
// Halaman ini sejak awal menjawab satu pertanyaan: penurunan penjualan ini
// masalah outlet atau masalah pasar/Markom? Sampai sekarang sisi Markom-nya
// dijawab lewat pengurangan — "pasar turun, dan tidak ada outlet yang
// menyimpang, jadi pasti pasar". Kesimpulan lewat pengurangan itu tidak pernah
// bisa membedakan dua hal yang sangat berbeda:
//
//	a. pasarnya memang sedang sepi;
//	b. kita berhenti mengetuk pintu pasar.
//
// Di grafik penjualan keduanya berbentuk persis sama. Angka medsos memisahkan
// keduanya, karena (b) meninggalkan jejak yang bisa dihitung: konten yang
// diterbitkan berkurang, dan jangkauannya ikut turun.
//
// Sumbu medsos yang dipakai adalah JANGKAUAN, bukan jumlah pengikut. Pengikut
// itu warisan — akun yang setahun tidak diurus tetap punya puluhan ribu, jadi
// angkanya nyaris tak bergerak dan tidak bisa menjelaskan naik-turun penjualan
// bulan ini. Yang bergerak seminggu-seminggu adalah berapa orang yang benar-
// benar melihat konten baru. Pengikut tetap digambar, tetapi sebagai perjalanan
// jangka panjang, bukan sebagai penjelas minggu ini.

const socialStaleDays = 8 // lebih dari sepekan tanpa pembacaan berhasil

// AttachBusinessSocial menempelkan blok medsos ke laporan yang sudah jadi.
//
// Dipanggil SESUDAH BuildBusinessNarrative, bukan sebelum: narasi mengosongkan
// Sections dan Insights saat mulai merakit, jadi bagian yang ditambahkan lebih
// dulu akan terhapus tanpa jejak.
func AttachBusinessSocial(rep *models.BusinessAnalysis, weeks int) {
	if SocialAccountCount() == 0 {
		return
	}
	rows, err := socialWeeklyRows(weeks)
	if err != nil {
		log.Printf("AttachBusinessSocial: %v", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	blockLen := rep.BlockWeeks
	if blockLen <= 0 {
		blockLen = 4
	}

	weekList, labelOf := socWeekAxis(rep, rows)
	if len(weekList) == 0 {
		return
	}

	soc := &models.BizSocial{
		Enabled:    true,
		BlockWeeks: blockLen,
		Outlets:    []models.BizSocialOutlet{},
		Group:      []models.BizSocialWeek{},
		Notes:      []string{},
	}

	// ── Kumpulkan per outlet ────────────────────────────────────────────────
	type akun struct {
		ref models.BizSocialAccountRef
		// pernahFollower = akun ini pernah menyerahkan angka pengikut sepanjang
		// periode. Akun yang tidak pernah — akun manual yang belum diisi — tidak
		// boleh ikut menentukan lengkap-tidaknya jumlah pengikut outlet, kalau
		// tidak satu akun IG kosong akan mengosongkan outlet yang TikTok-nya
		// terbaca penuh setiap minggu.
		pernahFollower bool
		weeks          map[string]socialWeekRow
	}
	type gerai struct {
		id, code, name string
		akun           []*akun
		byID           map[string]*akun
	}
	geraiByID := map[string]*gerai{}
	var urutan []string

	for _, r := range rows {
		g, ok := geraiByID[r.OutletID]
		if !ok {
			g = &gerai{id: r.OutletID, code: r.Code, name: r.Name, byID: map[string]*akun{}}
			geraiByID[r.OutletID] = g
			urutan = append(urutan, r.OutletID)
		}
		a, ok := g.byID[r.AccountID]
		if !ok {
			// Hanya akun yang memang dijadwalkan menarik yang bisa "mandek".
			// Akun yang angkanya diantar orang tidak sedang rusak ketika kosong
			// — ia cuma belum diisi, dan menyebutnya mandek akan mengeluarkan
			// outletnya dari grafik karena sebab yang salah.
			mandek := r.AutoFetch &&
				(r.LastOkAt == nil || time.Since(*r.LastOkAt) > socialStaleDays*24*time.Hour)
			a = &akun{
				ref: models.BizSocialAccountRef{
					Platform:  r.Platform,
					Username:  r.Username,
					URL:       r.ProfileURL,
					AutoFetch: r.AutoFetch,
					Stale:     mandek,
					LastError: r.LastError,
				},
				weeks: map[string]socialWeekRow{},
			}
			g.byID[r.AccountID] = a
			g.akun = append(g.akun, a)
		}
		a.weeks[r.WeekStart] = r
		if r.Followers != nil {
			a.ref.Followers = r.Followers // baris terurut menaik: yang terakhir menang
			a.pernahFollower = true
		}
	}

	recentFrom := len(weekList) - blockLen
	prevFrom := len(weekList) - 2*blockLen
	adaBlok := prevFrom >= 0

	// ── Per outlet ──────────────────────────────────────────────────────────
	salesGrowthByCode := map[string]*float64{}
	diagByCode := map[string]string{}
	for _, o := range rep.Outlets {
		salesGrowthByCode[o.Code] = o.Growth4
		diagByCode[o.Code] = o.Diagnosis
	}

	for _, oid := range urutan {
		g := geraiByID[oid]
		bo := models.BizSocialOutlet{
			OutletID: g.id, Code: g.code, Name: g.name,
			Accounts: []models.BizSocialAccountRef{},
			Weeks:    []models.BizSocialWeek{},
		}

		adaManualBaru := false
		for _, a := range g.akun {
			bo.Accounts = append(bo.Accounts, a.ref)
		}

		var prevFol *int64
		for i, wk := range weekList {
			mw := models.BizSocialWeek{WeekStart: wk, Label: labelOf[wk]}

			// Pengikut outlet = jumlah seluruh akunnya, dan hanya sah bila SEMUA
			// akun punya pembacaan minggu itu. Menjumlah sebagian akan terbaca
			// sebagai ribuan pengikut yang hilang pada minggu ketika satu akun
			// kebetulan gagal ditarik.
			// Konten, tayangan, dan interaksi dijumlah apa adanya: akun yang
			// diam minggu itu memang menyumbang nol, dan itu fakta yang benar.
			for _, a := range g.akun {
				r := a.weeks[wk]
				mw.Posts += r.Posts
				mw.Views += r.Views
				mw.Engagement += r.Engage
				if r.Manual {
					mw.HasManual = true
					if i >= recentFrom {
						adaManualBaru = true
					}
				}
			}

			// Pengikut berbeda: ia potret, bukan aliran. Jumlahnya hanya sah
			// kalau SETIAP akun yang pernah menyerahkan angka menyerahkannya
			// juga minggu ini. Menjumlah sebagian akan terbaca sebagai ribuan
			// pengikut yang lenyap pada minggu ketika satu akun gagal ditarik.
			//
			// Akun yang belum pernah menyerahkan angka sama sekali — akun manual
			// yang belum diisi — dilewati, bukan dianggap kosong. Kalau tidak,
			// satu akun IG yang menganggur akan mengosongkan outlet yang
			// TikTok-nya terbaca penuh tiap minggu.
			lengkap, adaPemasok := true, false
			var total int64
			for _, a := range g.akun {
				if !a.pernahFollower {
					continue
				}
				adaPemasok = true
				r := a.weeks[wk]
				if r.Followers == nil {
					lengkap = false
				} else {
					total += *r.Followers
					// Satu akun berangka bulat cukup untuk membuat JUMLAHNYA
					// bulat: 39K + 54.453 tetap membawa ketidakpastian ±500.
					if r.Approx {
						mw.FollowersApprox = true
					}
				}
			}
			if lengkap && adaPemasok {
				v := total
				mw.Followers = &v
				if prevFol != nil {
					d := v - *prevFol
					mw.Gain = &d
				}
				prevFol = &v
			} else {
				prevFol = nil // rantai terputus: selisih berikutnya tidak sah
			}

			bo.Weeks = append(bo.Weeks, mw)
		}

		// Mandek = ada akun yang lama tidak terbaca DAN tidak ditambal manual
		// pada blok terakhir. Outlet yang angkanya diketik orang tiap minggu
		// tetap sah meski scraper-nya sudah lama diblokir.
		for _, a := range g.akun {
			if a.ref.Stale && !adaManualBaru {
				bo.Stale = true
			}
		}

		for i := len(bo.Weeks) - 1; i >= 0; i-- {
			if bo.Weeks[i].Followers != nil {
				bo.FollowersNow = bo.Weeks[i].Followers
				break
			}
		}

		if adaBlok {
			for i := recentFrom; i < len(bo.Weeks); i++ {
				bo.PostsRecent += bo.Weeks[i].Posts
				bo.ViewsRecent += bo.Weeks[i].Views
				bo.EngRecent += bo.Weeks[i].Engagement
			}
			for i := prevFrom; i < recentFrom; i++ {
				bo.PostsPrev += bo.Weeks[i].Posts
				bo.ViewsPrev += bo.Weeks[i].Views
				bo.EngPrev += bo.Weeks[i].Engagement
			}

			// Pertumbuhan pengikut diukur ujung-ke-ujung, bukan menjumlah selisih
			// mingguan: menjumlah akan melewatkan minggu yang tak terbaca dan
			// melaporkan pertambahan yang lebih kecil dari kenyataan.
			awal, awalBulat := socFollowersAt(bo.Weeks, prevFrom, recentFrom-1)
			tengah, tengahBulat := socFollowersAt(bo.Weeks, recentFrom, len(bo.Weeks)-1)
			if awal != nil && tengah != nil {
				bo.GainRecent = *tengah - *awal
				bo.FollowerApprox = awalBulat || tengahBulat
				if *awal > 0 {
					v := round1(float64(bo.GainRecent) / float64(*awal) * 100)
					bo.FollowerGrowth = &v
				}
			}

			// Dasar sumbu medsos: tayangan bila terbaca, kalau tidak interaksi.
			// Instagram tidak menayangkan jumlah tonton untuk foto, jadi outlet
			// yang hanya ber-Instagram sering cuma punya suka + komentar —
			// dan itu tetap ukuran yang sah selama yang dibandingkan persentase
			// geraknya sendiri, bukan besarnya terhadap outlet lain.
			switch {
			case bo.ViewsPrev > 0:
				bo.ReachBasis = "tayangan"
				bo.ReachGrowth = bizPct(float64(bo.ViewsRecent), float64(bo.ViewsPrev))
			case bo.EngPrev > 0:
				bo.ReachBasis = "interaksi"
				bo.ReachGrowth = bizPct(float64(bo.EngRecent), float64(bo.EngPrev))
			}
		}

		bo.SalesGrowth = salesGrowthByCode[bo.Code]
		bo.Coverage = socCoverage(bo)
		socJudge(&bo, blockLen)
		soc.Outlets = append(soc.Outlets, bo)
	}

	sort.Slice(soc.Outlets, func(i, j int) bool { return soc.Outlets[i].Code < soc.Outlets[j].Code })

	// ── Grup ────────────────────────────────────────────────────────────────
	for i, wk := range weekList {
		gw := models.BizSocialWeek{WeekStart: wk, Label: labelOf[wk]}
		for _, o := range soc.Outlets {
			gw.Posts += o.Weeks[i].Posts
			gw.Views += o.Weeks[i].Views
			gw.Engagement += o.Weeks[i].Engagement
			if o.Weeks[i].HasManual {
				gw.HasManual = true
			}
		}
		soc.Group = append(soc.Group, gw)
	}
	if adaBlok {
		var vR, vP, eR, eP, folR, folP int64
		for i := recentFrom; i < len(soc.Group); i++ {
			soc.GroupPostsRecent += soc.Group[i].Posts
			vR += soc.Group[i].Views
			eR += soc.Group[i].Engagement
		}
		for i := prevFrom; i < recentFrom; i++ {
			soc.GroupPostsPrev += soc.Group[i].Posts
			vP += soc.Group[i].Views
			eP += soc.Group[i].Engagement
		}
		if vP > 0 {
			soc.GroupReachGrowth = bizPct(float64(vR), float64(vP))
		} else if eP > 0 {
			soc.GroupReachGrowth = bizPct(float64(eR), float64(eP))
		}
		// Pertumbuhan pengikut grup hanya menjumlah outlet yang punya DUA ujung
		// terbaca — outlet yang salah satu ujungnya kosong dikeluarkan dari
		// kedua sisi pembagian, bukan dihitung nol di salah satunya.
		for _, o := range soc.Outlets {
			a, _ := socFollowersAt(o.Weeks, prevFrom, recentFrom-1)
			b, _ := socFollowersAt(o.Weeks, recentFrom, len(o.Weeks)-1)
			if a != nil && b != nil {
				folP += *a
				folR += *b
			}
		}
		if folP > 0 {
			soc.GroupFollowerGrowth = bizPct(float64(folR), float64(folP))
		}
	}

	// ── Outlet tanpa akun & akun mandek ─────────────────────────────────────
	punyaAkun := map[string]bool{}
	for _, o := range soc.Outlets {
		punyaAkun[o.Code] = true
	}
	for _, o := range rep.Outlets {
		if !punyaAkun[o.Code] {
			soc.OutletsNoAccount = append(soc.OutletsNoAccount, o.Code)
		}
	}
	for _, o := range soc.Outlets {
		for _, a := range o.Accounts {
			if a.Stale {
				soc.StaleAccounts = append(soc.StaleAccounts,
					fmt.Sprintf("%s · %s @%s", o.Code, socPlatformName(a.Platform), a.Username))
			}
		}
	}

	soc.Correlation, soc.CorrCount = socCorrelation(soc.Outlets)

	socBuildSections(rep, soc)
	socBuildInsights(rep, soc, diagByCode)
	socBuildNotes(soc)

	rep.Social = soc
}

// socWeekAxis memakai daftar minggu yang sama persis dengan penjualan, supaya
// dua grafik yang ditumpuk pembacanya benar-benar sejajar. Baru kalau laporan
// penjualannya kosong, sumbu disusun dari data medsos sendiri.
func socWeekAxis(rep *models.BusinessAnalysis, rows []socialWeekRow) ([]string, map[string]string) {
	label := map[string]string{}
	if len(rep.Group) > 0 {
		list := make([]string, 0, len(rep.Group))
		for _, g := range rep.Group {
			list = append(list, g.WeekStart)
			label[g.WeekStart] = g.Label
		}
		return list, label
	}

	seen := map[string]bool{}
	var list []string
	for _, r := range rows {
		if !seen[r.WeekStart] {
			seen[r.WeekStart] = true
			list = append(list, r.WeekStart)
			label[r.WeekStart] = socWeekLabel(r.WeekStart)
		}
	}
	sort.Strings(list)
	return list, label
}

func socWeekLabel(weekStart string) string {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return weekStart
	}
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// socFollowersAt mengambil pembacaan pengikut TERAKHIR yang ada di rentang
// [from..to]. Dipakai sebagai ujung blok: kalau minggu penutup blok kebetulan
// tidak terbaca, minggu terdekat sebelumnya di dalam blok yang sama masih
// mewakili — yang tidak boleh adalah melompat keluar blok.
func socFollowersAt(weeks []models.BizSocialWeek, from, to int) (*int64, bool) {
	if from < 0 {
		from = 0
	}
	if to >= len(weeks) {
		to = len(weeks) - 1
	}
	for i := to; i >= from; i-- {
		if weeks[i].Followers != nil {
			return weeks[i].Followers, weeks[i].FollowersApprox
		}
	}
	return nil, false
}

func socPlatformName(p string) string {
	switch p {
	case models.SocialInstagram:
		return "Instagram"
	case models.SocialTiktok:
		return "TikTok"
	}
	return p
}

// socRibu menulis bilangan bulat dengan titik pemisah ribuan.
func socRibu(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "−" + b.String()
	}
	return b.String()
}

// socCoverage menerangkan dari mana angka satu outlet datang. Ditulis per
// outlet dan bukan sebagai satu kalimat umum di kaki halaman, karena cakupannya
// memang berbeda-beda: outlet yang cuma ber-Instagram tidak punya angka
// tayangan sama sekali, dan pembaca yang membandingkannya dengan outlet TikTok
// perlu tahu itu di baris outletnya sendiri.
func socCoverage(bo models.BizSocialOutlet) string {
	var bagian []string
	for _, a := range bo.Accounts {
		nama := socPlatformName(a.Platform) + " @" + a.Username
		switch {
		case !a.AutoFetch:
			// Bukan kegagalan, melainkan cara kerja: halaman publik Instagram
			// tidak menyerahkan angka apa pun, jadi akun ini memang diisi tangan.
			nama += " (angkanya diisi manual)"
		case a.Stale && a.LastError != "":
			nama += " (gagal dibaca: " + socPendekkan(a.LastError, 90) + ")"
		case a.Stale:
			nama += " (belum pernah berhasil dibaca)"
		}
		bagian = append(bagian, nama)
	}
	s := strings.Join(bagian, ", ")
	if bo.FollowerApprox {
		s += " — jumlah pengikutnya diserahkan platform sudah dibulatkan (\"39K\"), jadi pertambahan mingguan di bawah seratusan tidak terlihat di sini"
	}
	switch bo.ReachBasis {
	case "tayangan":
		s += " — jangkauan diukur dari jumlah tonton konten yang terbit."
	case "interaksi":
		s += " — jumlah tonton tidak diserahkan halaman publik, jadi jangkauan diukur dari pertambahan interaksi yang didapat."
	default:
		s += " — belum ada dua blok berisi yang bisa dibandingkan."
	}
	return s
}

func socPendekkan(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// socJudge menetapkan kuadran dan menulis kalimat pembacaannya.
//
// Kalimatnya dirakit dari angka outlet itu sendiri, mengikuti aturan halaman:
// tidak ada kalimat tetap yang ditanam, karena kalimat tetap terus berbunyi
// sama setelah keadaannya berubah.
func socJudge(bo *models.BizSocialOutlet, blockLen int) {
	dasar := bo.ReachBasis
	if dasar == "" {
		dasar = "jangkauan"
	}

	if bo.Stale {
		bo.Quadrant = models.BizSocKurang
		bo.QuadrantLabel = "Data Mandek"
		bo.Reading = "Angka medsos outlet ini sudah lebih dari sepekan tidak berhasil diambil, jadi ia tidak ikut dibandingkan. Naik-turun yang terlihat di grafiknya berasal dari pengambilan yang gagal, bukan dari medsosnya."
		return
	}
	if bo.ReachGrowth == nil || bo.SalesGrowth == nil {
		bo.Quadrant = models.BizSocKurang
		bo.QuadrantLabel = "Data Kurang"
		switch {
		case bo.ReachGrowth == nil && bo.SalesGrowth == nil:
			bo.Reading = fmt.Sprintf("Belum ada %d minggu berisi di kedua sisi — baik medsos maupun penjualan — untuk dibandingkan.", blockLen*2)
		case bo.ReachGrowth == nil:
			bo.Reading = fmt.Sprintf("Penjualannya %s, tetapi %s medsos pada %d minggu sebelumnya masih kosong sehingga belum ada pembanding.",
				bizNaikTurun(deref(bo.SalesGrowth)), dasar, blockLen)
			if socSemuaManual(bo.Accounts) {
				bo.Reading += " Seluruh akun outlet ini angkanya diisi tangan — isi minggu-minggunya di Laporan → Kinerja Markom supaya ia ikut masuk perbandingan."
			}
		default:
			bo.Reading = fmt.Sprintf("%s medsos %s, tetapi outlet ini belum punya riwayat penjualan yang cukup untuk disandingkan.",
				strings.ToUpper(dasar[:1])+dasar[1:], bizNaikTurun(deref(bo.ReachGrowth)))
		}
		return
	}

	r, s := *bo.ReachGrowth, *bo.SalesGrowth
	switch {
	case r >= 0 && s >= 0:
		bo.Quadrant = models.BizSocSejalan
		bo.QuadrantLabel = "Sejalan"
		bo.Reading = fmt.Sprintf("%s medsos %s dan penjualan %s. Keduanya bergerak searah — apa yang dikerjakan Markom dan outlet pada %d minggu terakhir layak diteruskan apa adanya.",
			socKapital(dasar), bizNaikTurun(r), bizNaikTurun(s), blockLen)
	case r < 0 && s < 0:
		bo.Quadrant = models.BizSocSepiDua
		bo.QuadrantLabel = "Sepi Dua-duanya"
		bo.Reading = fmt.Sprintf("%s medsos %s dan penjualan %s. Penurunan penjualan di sini punya penjelasan yang bisa dilihat: konten yang sampai ke orang ikut berkurang (%s konten pada %d minggu terakhir, sebelumnya %s). Yang perlu digerakkan lebih dulu programnya, bukan manajer outletnya.",
			socKapital(dasar), bizNaikTurun(r), bizNaikTurun(s),
			socRibu(bo.PostsRecent), blockLen, socRibu(bo.PostsPrev))
	case r >= 0 && s < 0:
		bo.Quadrant = models.BizSocRamaiSepi
		bo.QuadrantLabel = "Ramai tapi Sepi"
		bo.Reading = fmt.Sprintf("%s medsos %s, tetapi penjualan justru %s. Orang tetap melihat — yang tidak terjadi adalah mereka datang atau jadi membeli. Perhentiannya ada di outlet, bukan di promosinya.",
			socKapital(dasar), bizNaikTurun(r), bizNaikTurun(s))
	default:
		bo.Quadrant = models.BizSocTanpaMed
		bo.QuadrantLabel = "Jalan Tanpa Medsos"
		bo.Reading = fmt.Sprintf("Penjualan %s padahal %s medsos %s. Penjualan outlet ini sedang tidak bergantung pada medsos — pantas ditelusuri apa yang sebenarnya mendatangkan pembeli ke sini.",
			bizNaikTurun(s), dasar, bizNaikTurun(r))
	}
}

// socSemuaManual: tidak satu pun akun outlet ini dijadwalkan menarik. Dibedakan
// karena kolom medsos yang kosong pada outlet seperti ini bukan gejala sistem
// rusak, melainkan pekerjaan yang memang belum dilakukan orang.
func socSemuaManual(accs []models.BizSocialAccountRef) bool {
	if len(accs) == 0 {
		return false
	}
	for _, a := range accs {
		if a.AutoFetch {
			return false
		}
	}
	return true
}

func socKapital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// socCorrelation menghitung keeratan gerak medsos dengan gerak penjualan
// antar-outlet (Pearson).
//
// Sengaja menolak menghitung di bawah tiga outlet: korelasi dari dua titik
// selalu tepat +1 atau −1, apa pun angkanya. Menampilkannya akan memberi kesan
// hubungan yang sempurna padahal yang terjadi cuma "dua titik selalu bisa
// dilewati satu garis".
func socCorrelation(outlets []models.BizSocialOutlet) (*float64, int) {
	var xs, ys []float64
	for _, o := range outlets {
		if o.Stale || o.ReachGrowth == nil || o.SalesGrowth == nil {
			continue
		}
		xs = append(xs, *o.ReachGrowth)
		ys = append(ys, *o.SalesGrowth)
	}
	n := len(xs)
	if n < 3 {
		return nil, n
	}

	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(n)
	my /= float64(n)

	var num, dx, dy float64
	for i := range xs {
		a, b := xs[i]-mx, ys[i]-my
		num += a * b
		dx += a * a
		dy += b * b
	}
	if dx == 0 || dy == 0 {
		return nil, n
	}
	v := math.Round(num/math.Sqrt(dx*dy)*100) / 100
	return &v, n
}

// ── Narasi bagian medsos ─────────────────────────────────────────────────────
//
// Sama seperti bagian lain halaman ini, tidak ada satu pun kalimat penjelas
// yang ditanam di sisi UI. Semuanya dirakit di sini dari angka periode yang
// sedang dilihat, supaya kalimatnya ikut berubah ketika keadaannya berubah.

func socBuildSections(rep *models.BusinessAnalysis, soc *models.BizSocial) {
	add := func(key, title, lead, hint string) {
		rep.Sections = append(rep.Sections, models.BizSection{
			Key: key, Title: title, Lead: lead, Hint: hint,
		})
	}

	// ── Seberapa keras medsos bekerja ───────────────────────────────────────
	lead := fmt.Sprintf("Pada %d minggu terakhir seluruh outlet menerbitkan %s konten; %d minggu sebelumnya %s.",
		soc.BlockWeeks, socRibu(soc.GroupPostsRecent), soc.BlockWeeks, socRibu(soc.GroupPostsPrev))
	if soc.GroupReachGrowth != nil {
		lead += fmt.Sprintf(" Jangkauannya %s.", bizNaikTurun(*soc.GroupReachGrowth))
	} else {
		lead += " Jangkauannya belum bisa dibandingkan karena blok sebelumnya masih kosong."
	}
	add("medsos_jangkauan", "Seberapa Keras Medsos Bekerja", lead,
		"Batang = berapa kali konten yang terbit minggu itu ditonton. Garis = berapa konten yang terbit. Minggu dengan batang pendek DAN garis rendah berarti medsosnya memang sedang diam — itu sebab yang berbeda dari pasar yang sepi, dan orang yang menanganinya juga berbeda.")

	// ── Perjalanan pengikut ─────────────────────────────────────────────────
	var totalFol int64
	adaFol := false
	for _, o := range soc.Outlets {
		if o.FollowersNow != nil {
			totalFol += *o.FollowersNow
			adaFol = true
		}
	}
	lead = fmt.Sprintf("%d outlet punya akun terdaftar.", len(soc.Outlets))
	if adaFol {
		lead += fmt.Sprintf(" Pembacaan terakhir menghitung %s pengikut seluruhnya.", socRibu(totalFol))
	}
	if soc.GroupFollowerGrowth != nil {
		// "sekitar" bukan basa-basi: kalau salah satu akun menyerahkan angka
		// bulat, satu desimal di sini menjanjikan ketelitian yang tidak ada.
		kira := ""
		for _, o := range soc.Outlets {
			if o.FollowerApprox {
				kira = "sekitar "
				break
			}
		}
		lead += fmt.Sprintf(" Dalam %d minggu terakhir jumlahnya %s%s.",
			soc.BlockWeeks, kira, bizNaikTurun(*soc.GroupFollowerGrowth))
	}
	hint := "Pengikut bergerak lambat dan tidak menjelaskan penjualan minggu ini — ia dipakai untuk melihat arah jangka panjang. Garis yang putus berarti minggu itu tidak ada pembacaan yang berhasil; itu bukan berarti pengikutnya nol."
	for _, o := range soc.Outlets {
		if o.FollowerApprox {
			hint += " Sebagian akun hanya menyerahkan angka yang sudah dibulatkan platform, sehingga garisnya naik bertangga — tangga itu langkah pembulatan, bukan lonjakan pengikut."
			break
		}
	}
	add("medsos_pengikut", "Perjalanan Pengikut", lead, hint)

	// ── Medsos vs penjualan ─────────────────────────────────────────────────
	jml := map[string]int{}
	for _, o := range soc.Outlets {
		jml[o.Quadrant]++
	}
	var bagian []string
	if n := jml[models.BizSocRamaiSepi]; n > 0 {
		bagian = append(bagian, fmt.Sprintf("%d outlet medsosnya ramai tetapi penjualannya turun", n))
	}
	if n := jml[models.BizSocSepiDua]; n > 0 {
		bagian = append(bagian, fmt.Sprintf("%d outlet sepi di dua-duanya", n))
	}
	if n := jml[models.BizSocSejalan]; n > 0 {
		bagian = append(bagian, fmt.Sprintf("%d outlet naik di dua-duanya", n))
	}
	if n := jml[models.BizSocTanpaMed]; n > 0 {
		bagian = append(bagian, fmt.Sprintf("%d outlet penjualannya naik tanpa dukungan medsos", n))
	}

	lead = "Belum ada outlet yang kedua sisinya cukup untuk disandingkan."
	if len(bagian) > 0 {
		lead = socKapital(strings.Join(bagian, ", ")) + "."
	}
	if soc.Correlation != nil {
		lead += fmt.Sprintf(" Dari %d outlet yang datanya lengkap, keeratan gerak medsos dengan gerak penjualan %s (%s).",
			soc.CorrCount, bizNum(*soc.Correlation), socKeeratan(*soc.Correlation))
	} else if soc.CorrCount > 0 {
		lead += fmt.Sprintf(" Outlet yang kedua sisinya lengkap baru %d — keeratannya belum dihitung karena di bawah tiga outlet angkanya selalu keluar sempurna tanpa berarti apa-apa.", soc.CorrCount)
	}
	add("medsos_silang", "Medsos dan Penjualan Disandingkan", lead,
		"Sumbu mendatar = gerak medsos, sumbu tegak = gerak penjualan, keduanya membandingkan blok yang sama persis. Kanan-bawah: orang melihat tetapi tidak jadi datang — perhentiannya di outlet. Kiri-bawah: dua-duanya sepi — mesin promosinya yang berhenti. Titik yang menempel di garis tengah bisa berpindah kuadran minggu depan; yang layak ditindak titik yang jelas jauh dari garis.")
}

// socKeeratan menerjemahkan koefisien korelasi jadi kata. Angka −1..1 tidak
// berarti apa-apa bagi pembaca yang harus memutuskan anggaran; katanya yang
// dipakai memutuskan, angkanya untuk yang mau menelusuri.
func socKeeratan(r float64) string {
	a := math.Abs(r)
	arah := "searah"
	if r < 0 {
		arah = "berlawanan arah"
	}
	switch {
	case a >= 0.7:
		return "erat, " + arah
	case a >= 0.4:
		return "sedang, " + arah
	case a >= 0.2:
		return "lemah, " + arah
	default:
		return "nyaris tidak ada"
	}
}

func socBuildInsights(rep *models.BusinessAnalysis, soc *models.BizSocial, diagByCode map[string]string) {
	add := func(kind, title, body string, outlets []string) {
		rep.Insights = append(rep.Insights, models.BizInsight{
			Kind: kind, Title: title, Body: body, Outlets: outlets,
		})
	}

	var ramaiSepiTertinggal, ramaiSepi, sepiDua, berhentiPosting []string
	for _, o := range soc.Outlets {
		switch o.Quadrant {
		case models.BizSocRamaiSepi:
			if diagByCode[o.Code] == models.BizDiagLagging {
				ramaiSepiTertinggal = append(ramaiSepiTertinggal, o.Code)
			} else {
				ramaiSepi = append(ramaiSepi, o.Code)
			}
		case models.BizSocSepiDua:
			sepiDua = append(sepiDua, o.Code)
		}
		if o.PostsRecent == 0 && o.PostsPrev > 0 && !o.Stale {
			berhentiPosting = append(berhentiPosting, o.Code)
		}
	}

	// Temuan terkuat: outlet yang sudah divonis tertinggal oleh angka penjualan,
	// DAN medsosnya terbukti tidak ikut sepi. Ini menutup satu-satunya pembelaan
	// yang selama ini tidak bisa diuji halaman ini — "promosinya kurang".
	if len(ramaiSepiTertinggal) > 0 {
		add(models.BizInsightProblem, "Dilihat orang, tetapi tidak jadi dibeli",
			fmt.Sprintf("%s tertinggal dari outlet lain pada angka penjualan, sementara jangkauan medsosnya justru naik di blok yang sama. Orang tetap melihat tempat ini — yang tidak terjadi adalah mereka datang atau jadi membeli. Alasan \"promosinya kurang\" tidak berlaku di sini; yang perlu diperiksa apa yang dialami pembeli setelah mereka tertarik.",
				strings.Join(ramaiSepiTertinggal, ", ")),
			ramaiSepiTertinggal)
	}
	if len(sepiDua) > 0 {
		add(models.BizInsightWarning, "Medsos ikut sepi, bukan cuma penjualannya",
			fmt.Sprintf("Di %s penjualan dan jangkauan medsos turun bersamaan. Penurunan seperti ini punya sebab yang bisa dilihat sebelum menyalahkan manajer outlet: konten yang sampai ke orang memang berkurang. Bereskan dulu sisi programnya, lalu nilai ulang outletnya pada periode berikutnya.",
				strings.Join(sepiDua, ", ")),
			sepiDua)
	}
	if len(berhentiPosting) > 0 {
		add(models.BizInsightWarning, "Berhenti menerbitkan konten",
			fmt.Sprintf("%s tidak menerbitkan satu konten pun selama %d minggu terakhir, padahal pada blok sebelumnya masih ada. Selama medsosnya diam, penurunan penjualan di outlet ini tidak bisa dipakai menilai manajernya — belum ada yang mengetuk pintu pasarnya.",
				strings.Join(berhentiPosting, ", "), soc.BlockWeeks),
			berhentiPosting)
	}
	if len(ramaiSepi) > 0 {
		add(models.BizInsightNeutral, "Jangkauan naik, penjualan belum ikut",
			fmt.Sprintf("Di %s jangkauan medsos naik tetapi penjualannya belum ikut. Outlet ini belum divonis tertinggal dari sejawatnya, jadi belum tentu ada yang salah — tetapi jarak antara \"dilihat\" dan \"dibeli\" di sini pantas ditelusuri sebelum menambah anggaran konten.",
				strings.Join(ramaiSepi, ", ")),
			ramaiSepi)
	}
	if len(soc.StaleAccounts) > 0 {
		add(models.BizInsightWarning, "Angka medsos mandek",
			fmt.Sprintf("%s sudah lebih dari sepekan tidak berhasil dibaca dari halaman publik. Outlet yang terkena dikeluarkan dari perbandingan medsos supaya jangkauan yang jatuh karena gagal ambil tidak terbaca sebagai medsos yang sepi. Perbaiki lewat Laporan → Kinerja Markom, atau isi angkanya manual di sana.",
				strings.Join(soc.StaleAccounts, "; ")),
			nil)
	}
	if len(soc.OutletsNoAccount) > 0 {
		add(models.BizInsightNeutral, "Outlet tanpa akun terdaftar",
			fmt.Sprintf("%s belum punya akun Instagram/TikTok yang didaftarkan, jadi kolom medsosnya kosong. Kosong di sini berarti belum didaftarkan — bukan berarti outletnya tidak bermedsos.",
				strings.Join(soc.OutletsNoAccount, ", ")),
			soc.OutletsNoAccount)
	}
}

func socBuildNotes(soc *models.BizSocial) {
	soc.Notes = append(soc.Notes,
		"Angkanya diambil dari halaman profil publik Instagram dan TikTok, bukan dari API resmi. Kedua platform memblokir pembacaan anonim secara berkala, jadi minggu yang gagal dibaca dibiarkan kosong — tidak diisi nol dan tidak ditebak dari minggu sebelumnya.",
		"Jangkauan (impresi) akun Instagram tidak pernah ada di halaman publik: angka itu hanya hidup di layar Insights pemegang akun. Untuk outlet yang belum punya angka tonton, jangkauannya diukur dari interaksi yang didapat — dan dasar yang dipakai selalu disebutkan di baris outletnya.",
		"Halaman profil TikTok menyerahkan penghitung akun (pengikut, jumlah konten, total suka) tetapi bukan daftar videonya. Karena itu konten dan interaksi mingguan diturunkan dari SELISIH penghitung antar minggu — tambahan 3 konten dan 4.812 suka sepanjang minggu itu. Ukuran ini sah, hanya tidak memuat tayangan.",
		"Minggu yang tidak berhasil dibaca membuat selisihnya tidak bisa dihitung, dan minggu itu tercatat nol — bukan ditumpuk ke minggu berikutnya. Menumpuknya akan melahirkan satu batang raksasa yang tidak pernah terjadi.",
		"Satu konten dihitung pada minggu ia TERBIT, memakai angka tontonnya hari ini. Konten lama yang masih ditonton karena itu terus menambah angka minggu terbitnya — yang diukur di sini kinerja konten minggu tersebut, bukan lalu lintas yang terjadi di minggu itu.",
		"Minggu medsos memakai batas yang sama persis dengan minggu penjualan di halaman ini (Senin–Minggu, zona waktu aplikasi, minggu berjalan dikecualikan). Tanpa batas yang sama, dua grafik yang ditumpuk tidak benar-benar sejajar.",
		"Angka yang diketik manual di halaman Kinerja Markom menimpa hasil tarikan pada minggu dan kolom yang diisi saja; sisanya tetap dari tarikan. Minggu yang mengandung angka ketikan diberi tanda di grafik.",
	)
	if len(soc.StaleAccounts) > 0 {
		soc.Notes = append(soc.Notes,
			"Outlet yang salah satu akunnya lebih dari sepekan gagal dibaca dikeluarkan dari grafik sebar dan dari perhitungan keeratan, tetapi garis pengikut dan batang jangkauannya tetap digambar apa adanya.")
	}
}
