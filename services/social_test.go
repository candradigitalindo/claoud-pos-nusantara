package services

// Menjaga janji-janji yang paling gampang diam-diam dilanggar oleh modul ini:
// angka yang dibulatkan platform tidak boleh terbaca sebagai angka pasti,
// tautan profil apa pun bentuknya harus jatuh ke username yang sama, dan
// minggu yang tidak terbaca tidak boleh menjelma jadi nol.
import (
	"cloud-pos/models"
	"math"
	"strings"
	"testing"
)

func TestParseCount(t *testing.T) {
	cases := []struct {
		in     string
		want   int64
		approx bool
		ok     bool
	}{
		{"1234", 1234, false, true},
		{"1,234", 1234, false, true}, // pemisah ribuan gaya Inggris
		{"1.234", 1234, false, true}, // pemisah ribuan gaya Indonesia
		{"12,345,678", 12345678, false, true},
		{"12.3K", 12300, true, true},
		{"1.2M", 1200000, true, true},
		{"4,5 rb", 4500, true, true},
		{"2 jt", 2000000, true, true},
		{"", 0, false, false},
		{"beberapa", 0, false, false},
	}
	for _, c := range cases {
		got, approx, ok := parseCount(c.in)
		if ok != c.ok {
			t.Errorf("parseCount(%q): ok=%v, mau %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if got != c.want {
			t.Errorf("parseCount(%q) = %d, mau %d", c.in, got, c.want)
		}
		// Ini yang paling penting: "12,3 rb" bisa berarti apa pun antara 12.250
		// dan 12.349. Kalau tandanya hilang, selisih pembulatan antar minggu
		// akan dilaporkan sebagai pertumbuhan pengikut yang tidak pernah terjadi.
		if approx != c.approx {
			t.Errorf("parseCount(%q): approx=%v, mau %v", c.in, approx, c.approx)
		}
	}
}

func TestParseSocialHandle(t *testing.T) {
	ig := []string{
		"nusantara.kopi",
		"@nusantara.kopi",
		"https://www.instagram.com/nusantara.kopi/",
		"https://www.instagram.com/nusantara.kopi/?hl=id&igsh=MXY0",
		"instagram.com/nusantara.kopi",
	}
	for _, in := range ig {
		u, url, err := parseSocialHandle(models.SocialInstagram, in)
		if err != nil {
			t.Errorf("instagram %q: %v", in, err)
			continue
		}
		if u != "nusantara.kopi" {
			t.Errorf("instagram %q -> %q, mau nusantara.kopi", in, u)
		}
		if url != "https://www.instagram.com/nusantara.kopi/" {
			t.Errorf("instagram %q -> url %q", in, url)
		}
	}

	tt := []string{
		"@nusantara.kopi",
		"https://www.tiktok.com/@nusantara.kopi",
		"https://www.tiktok.com/@nusantara.kopi?lang=id-ID",
	}
	for _, in := range tt {
		u, url, err := parseSocialHandle(models.SocialTiktok, in)
		if err != nil {
			t.Errorf("tiktok %q: %v", in, err)
			continue
		}
		if u != "nusantara.kopi" || url != "https://www.tiktok.com/@nusantara.kopi" {
			t.Errorf("tiktok %q -> %q / %q", in, u, url)
		}
	}

	for _, bad := range []string{"", "  ", "nama dengan spasi", "nama<script>"} {
		if _, _, err := parseSocialHandle(models.SocialInstagram, bad); err == nil {
			t.Errorf("parseSocialHandle(%q) seharusnya ditolak", bad)
		}
	}
	if _, _, err := parseSocialHandle("facebook", "halo"); err == nil {
		t.Error("platform tak dikenal seharusnya ditolak")
	}
}

func TestIgParseOgDescription(t *testing.T) {
	html := `<html><head><meta property="og:description" ` +
		`content="12.3K Followers, 431 Following, 268 Posts - See Instagram photos and videos" /></head></html>`
	p := igParseOgDescription(html)
	if p == nil {
		t.Fatal("og:description Instagram gagal dibaca")
	}
	if p.Followers == nil || *p.Followers != 12300 {
		t.Errorf("followers = %v, mau 12300", p.Followers)
	}
	if p.PostsCount == nil || *p.PostsCount != 268 {
		t.Errorf("posts = %v, mau 268", p.PostsCount)
	}
	if !p.Approx {
		t.Error("angka dari og:description harus ditandai approx — 12.3K bukan angka pasti")
	}
}

func TestIgParseProfileInfo(t *testing.T) {
	body := `{"data":{"user":{
		"edge_followed_by":{"count":5231},
		"edge_follow":{"count":312},
		"edge_owner_to_timeline_media":{"count":148,"edges":[
			{"node":{"shortcode":"AbC123","taken_at_timestamp":1757462400,
			 "edge_liked_by":{"count":214},"edge_media_to_comment":{"count":17},
			 "video_view_count":8200,
			 "edge_media_to_caption":{"edges":[{"node":{"text":"Promo akhir pekan"}}]}}}
		]}}}}`
	p := igParseProfileInfo(body)
	if p == nil {
		t.Fatal("web_profile_info gagal dibaca")
	}
	if *p.Followers != 5231 || *p.PostsCount != 148 {
		t.Errorf("stats salah: %v / %v", *p.Followers, *p.PostsCount)
	}
	if p.Approx {
		t.Error("angka dari JSON penuh tidak boleh ditandai approx")
	}
	if len(p.Posts) != 1 {
		t.Fatalf("konten terbaca %d, mau 1", len(p.Posts))
	}
	got := p.Posts[0]
	if got.Ref != "AbC123" || got.Likes != 214 || got.Comments != 17 || got.Views != 8200 {
		t.Errorf("konten salah: %+v", got)
	}
	if got.Caption != "Promo akhir pekan" {
		t.Errorf("caption = %q", got.Caption)
	}
}

func TestTtParseUniversal(t *testing.T) {
	raw := `{"__DEFAULT_SCOPE__":{"webapp.user-detail":{
		"userInfo":{"stats":{"followerCount":18420,"followingCount":21,"heartCount":250100,"videoCount":96}},
		"itemList":[{"id":"7412","createTime":1757462400,"desc":"Menu baru",
			"author":{"uniqueId":"nusantara.kopi"},
			"stats":{"playCount":54200,"diggCount":3100,"commentCount":88,"shareCount":140}}]
	}}}`
	p := ttParseUniversal(raw)
	if p == nil {
		t.Fatal("universal-data TikTok gagal dibaca")
	}
	if *p.Followers != 18420 || *p.PostsCount != 96 || *p.LikesTotal != 250100 {
		t.Errorf("stats salah: %+v", p)
	}
	if len(p.Posts) != 1 {
		t.Fatalf("konten terbaca %d, mau 1", len(p.Posts))
	}
	v := p.Posts[0]
	if v.Views != 54200 || v.Likes != 3100 || v.Comments != 88 || v.Shares != 140 {
		t.Errorf("angka konten salah: %+v", v)
	}
	if v.Permalink != "https://www.tiktok.com/@nusantara.kopi/video/7412" {
		t.Errorf("permalink = %q", v.Permalink)
	}
}

func TestTtParseOgDescription(t *testing.T) {
	html := `<meta property="og:description" content="Nusantara Kopi (@nusantara.kopi) on TikTok | 250.1K Likes. 18.4K Followers. Kopi susu terbaik.">`
	p := ttParseOgDescription(html)
	if p == nil {
		t.Fatal("og:description TikTok gagal dibaca")
	}
	if *p.Followers != 18400 || !p.Approx {
		t.Errorf("followers=%v approx=%v", *p.Followers, p.Approx)
	}
}

// socFollowersAt boleh mundur mencari pembacaan di dalam blok yang sama, tetapi
// TIDAK boleh melompat keluar blok — kalau ia melompat, pertumbuhan satu blok
// akan diukur terhadap titik yang berada di blok sebelumnya dan angkanya
// membengkak tanpa ada yang menyadari.
func TestSocFollowersAtTidakMelompatKeluarBlok(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	weeks := []models.BizSocialWeek{
		{Followers: n(1000)}, // 0
		{Followers: nil},     // 1
		{Followers: nil},     // 2
		{Followers: nil},     // 3
	}
	if got, _ := socFollowersAt(weeks, 1, 3); got != nil {
		t.Errorf("blok 1..3 seluruhnya kosong, tetapi terbaca %d", *got)
	}
	weeks[2].Followers = n(1200)
	if got, _ := socFollowersAt(weeks, 1, 3); got == nil || *got != 1200 {
		t.Errorf("mau 1200, dapat %v", got)
	}

	// Sifat "angka sudah dibulatkan platform" harus ikut terbawa keluar, bukan
	// hilang di perjalanan: itulah yang menahan pertumbuhan mingguan Instagram
	// dilaporkan sampai satu desimal padahal langkah pembulatannya 500.
	weeks[2].FollowersApprox = true
	if _, bulat := socFollowersAt(weeks, 1, 3); !bulat {
		t.Error("penanda angka bulat tidak ikut terbawa")
	}
	weeks[3].Followers, weeks[3].FollowersApprox = n(1300), false
	if _, bulat := socFollowersAt(weeks, 1, 3); bulat {
		t.Error("pembacaan terbaru eksak, tetapi masih ditandai bulat")
	}
}

func TestSocCorrelation(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	mk := func(code string, reach, sales float64, stale bool) models.BizSocialOutlet {
		return models.BizSocialOutlet{Code: code, ReachGrowth: f(reach), SalesGrowth: f(sales), Stale: stale}
	}

	// Dua outlet: selalu bisa dilewati satu garis, jadi korelasinya tidak boleh
	// dilaporkan sama sekali.
	if r, n := socCorrelation([]models.BizSocialOutlet{
		mk("A", 10, 5, false), mk("B", -10, -5, false),
	}); r != nil || n != 2 {
		t.Errorf("dua outlet: r=%v n=%d, mau nil/2", r, n)
	}

	// Tiga outlet searah sempurna.
	r, n := socCorrelation([]models.BizSocialOutlet{
		mk("A", 10, 5, false), mk("B", 0, 0, false), mk("C", -10, -5, false),
	})
	if r == nil || n != 3 || math.Abs(*r-1) > 0.001 {
		t.Errorf("tiga outlet searah: r=%v n=%d, mau 1/3", r, n)
	}

	// Outlet yang datanya mandek harus dikeluarkan, bukan ikut menyeret angka.
	_, n = socCorrelation([]models.BizSocialOutlet{
		mk("A", 10, 5, false), mk("B", 0, 0, false), mk("C", -10, -5, true),
	})
	if n != 2 {
		t.Errorf("outlet mandek ikut terhitung: n=%d, mau 2", n)
	}
}

func TestSocJudgeKuadran(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		reach, sales float64
		want         string
	}{
		{12, 8, models.BizSocSejalan},
		{-20, -15, models.BizSocSepiDua},
		{30, -12, models.BizSocRamaiSepi},
		{-25, 9, models.BizSocTanpaMed},
	}
	for _, c := range cases {
		o := models.BizSocialOutlet{
			Code: "X", ReachBasis: "tayangan",
			ReachGrowth: f(c.reach), SalesGrowth: f(c.sales),
		}
		socJudge(&o, 4)
		if o.Quadrant != c.want {
			t.Errorf("reach=%.0f sales=%.0f -> %s, mau %s", c.reach, c.sales, o.Quadrant, c.want)
		}
		if o.Reading == "" || o.QuadrantLabel == "" {
			t.Errorf("reach=%.0f sales=%.0f: pembacaannya kosong", c.reach, c.sales)
		}
	}

	// Outlet yang angkanya mandek tidak boleh diberi kuadran apa pun: jangkauan
	// yang jatuh karena scraper diblokir bukan medsos yang sepi.
	o := models.BizSocialOutlet{Code: "X", Stale: true, ReachGrowth: f(-90), SalesGrowth: f(-5)}
	socJudge(&o, 4)
	if o.Quadrant != models.BizSocKurang {
		t.Errorf("outlet mandek -> %s, mau DATA_KURANG", o.Quadrant)
	}
}

// Akun yang angkanya diantar orang tidak boleh pernah disebut mandek. Sebelum
// pemisahan ini ada, satu akun Instagram yang tidak pernah berhasil ditarik
// cukup untuk mengeluarkan seluruh outlet dari grafik — termasuk outlet yang
// TikTok-nya terbaca penuh tiap minggu.
func TestAkunManualTidakPernahMandek(t *testing.T) {
	manual := models.BizSocialAccountRef{Platform: models.SocialInstagram, Username: "a", AutoFetch: false}
	auto := models.BizSocialAccountRef{Platform: models.SocialTiktok, Username: "b", AutoFetch: true}

	if !socSemuaManual([]models.BizSocialAccountRef{manual}) {
		t.Error("outlet berisi akun manual saja seharusnya terbaca semua-manual")
	}
	if socSemuaManual([]models.BizSocialAccountRef{manual, auto}) {
		t.Error("outlet yang punya satu akun otomatis bukan semua-manual")
	}
	if socSemuaManual(nil) {
		t.Error("outlet tanpa akun bukan semua-manual")
	}

	// Keterangan sumbernya harus menyebut 'diisi manual', bukan bahasa kegagalan.
	bo := models.BizSocialOutlet{Accounts: []models.BizSocialAccountRef{manual}, ReachBasis: "tayangan"}
	cov := socCoverage(bo)
	if !strings.Contains(cov, "diisi manual") {
		t.Errorf("keterangan sumber tidak menyebut isian manual: %q", cov)
	}
	if strings.Contains(cov, "gagal") {
		t.Errorf("akun manual tidak boleh digambarkan sebagai gagal: %q", cov)
	}
}

// Halaman profil TikTok tidak menyerahkan daftar videonya, hanya penghitung
// kumulatif akun — jadi angka mingguannya lahir dari selisih. Dua keadaan yang
// wajib jadi nol: ujung yang tidak terbaca, dan penghitung yang mundur karena
// konten dihapus.
func TestSelisihKumulatif(t *testing.T) {
	n := func(v int64) *int64 { return &v }
	cases := []struct {
		nama      string
		cur, prev *int64
		want      int64
	}{
		{"pertambahan biasa", n(474), n(471), 3},
		{"tidak bertambah", n(474), n(474), 0},
		{"minggu ini tak terbaca", nil, n(471), 0},
		{"minggu lalu tak terbaca", n(474), nil, 0},
		{"dua-duanya tak terbaca", nil, nil, 0},
		{"konten dihapus (mundur)", n(468), n(474), 0},
		{"lompatan suka", n(1150451), n(1145639), 4812},
	}
	for _, c := range cases {
		if got := socialSelisihKumulatif(c.cur, c.prev); got != c.want {
			t.Errorf("%s: dapat %d, mau %d", c.nama, got, c.want)
		}
	}
}

func TestSocRibu(t *testing.T) {
	cases := map[int64]string{0: "0", 999: "999", 1000: "1.000", 12345: "12.345", 1234567: "1.234.567"}
	for in, want := range cases {
		if got := socRibu(in); got != want {
			t.Errorf("socRibu(%d) = %q, mau %q", in, got, want)
		}
	}
}
