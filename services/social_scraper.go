package services

import (
	"cloud-pos/models"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ── Pengambil angka medsos dari halaman profil publik ────────────────────────
//
// Tidak memakai API resmi: Instagram Graph menuntut akun Professional yang
// tertaut Facebook Page plus app yang lolos review, TikTok menuntut developer
// app yang disetujui. Keduanya berminggu-minggu di sisi persetujuan, dan itu
// keputusan yang sudah diambil — berkas ini menanggung akibatnya.
//
// Akibat yang harus ditanggung, dan cara menanggungnya:
//
//  1. Halaman publik berubah bentuk tanpa pemberitahuan. Karena itu tiap
//     platform punya BEBERAPA jalur baca yang dicoba berurutan, dari yang
//     paling kaya ke yang paling miskin, dan yang paling miskin (meta
//     og:description) hampir selalu masih ada karena dipakai pratinjau tautan
//     WhatsApp. Lebih baik dapat jumlah pengikut saja daripada tidak sama
//     sekali.
//
//  2. Permintaan anonim diblokir berkala. Kegagalan TIDAK ditelan: ia disimpan
//     di social_accounts.last_error dan ditampilkan di halaman. Scraper yang
//     diam saat gagal menghasilkan grafik yang tetap tergambar tetapi berhenti
//     bertambah — dan tidak ada yang sadar selama berminggu-minggu.
//
//  3. Angka pratinjau sudah dibulatkan platform ("12,3 rb"). Angka seperti itu
//     ditandai approx, dan selisih mingguan yang lahir dari pembulatan tidak
//     pernah dilaporkan sebagai pertumbuhan.
//
// Jeda antar-akun disengaja dan acak. Menembak sepuluh profil berturut-turut
// dalam satu detik adalah cara tercepat membuat alamat IP server ini diblokir,
// dan yang hilang bukan hanya angka minggu ini melainkan seluruh riwayat ke
// depan.

const (
	// App ID publik yang dipakai web Instagram sendiri. Tanpa header ini
	// endpoint web_profile_info menolak permintaan.
	igWebAppID = "936619743392459"

	socialHTTPTimeout = 25 * time.Second
	socialMaxBody     = 8 << 20 // 8 MB; halaman profil TikTok bisa ~2-3 MB
)

// socialPostRaw = satu konten beserta angka TERKININYA (bukan angka saat
// terbit). Konten lama yang masih ditonton akan naik angkanya tiap kali dibaca
// ulang — itu memang yang terjadi, dan rollup mingguan memperlakukannya sebagai
// kinerja konten yang terbit pada minggu itu.
type socialPostRaw struct {
	Ref       string
	PostedAt  time.Time
	Permalink string
	Caption   string
	Views     int64
	Likes     int64
	Comments  int64
	Shares    int64
}

// socialProfile = hasil satu kali baca halaman profil.
type socialProfile struct {
	Followers  *int64
	Following  *int64
	PostsCount *int64
	LikesTotal *int64 // TikTok: total hati; Instagram tidak punya padanannya
	// Approx menerangkan JUMLAH PENGIKUT secara khusus, bukan halamannya secara
	// umum. Dialah satu-satunya medan yang dipakai menghitung pertumbuhan, jadi
	// dialah yang sifat ketelitiannya perlu ikut tersimpan sampai ke layar.
	Approx bool
	Posts  []socialPostRaw
	Via    string // jalur baca yang berhasil, untuk pesan galat yang bisa ditelusuri
}

// ── Klien HTTP ───────────────────────────────────────────────────────────────

var socialClient = func() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 60 * time.Second,
	}
	// Kuki disimpan dan dikirim kembali, sama seperti peramban sungguhan.
	// Tanpa ini tiap permintaan datang sebagai pengunjung yang belum pernah
	// ada — sembilan belas pengunjung baru dari satu alamat IP tiap pagi,
	// selamanya, tidak pernah menyimpan apa pun. Pola itu jauh lebih mencolok
	// daripada satu pengunjung yang kembali tiap hari.
	jar, _ := cookiejar.New(nil)
	// SOCIAL_PROXY_URL: server produksi biasanya duduk di alamat datacenter, dan
	// alamat datacenter adalah yang paling dulu diblokir kedua platform. Proxy
	// dibuat opsional lewat env supaya pemasangannya tidak menuntut deploy ulang
	// kode saat blokirnya mulai terasa.
	if p := strings.TrimSpace(os.Getenv("SOCIAL_PROXY_URL")); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: socialHTTPTimeout, Transport: tr, Jar: jar}
}()

// ── Penyamaran ──────────────────────────────────────────────────────────────
//
// Sebuah permintaan dikenali bukan dari User-Agent-nya saja, melainkan dari
// apakah seluruh rombongan header-nya MASUK AKAL bersama-sama. Safari di iPhone
// tidak pernah mengirim Sec-CH-UA; perayap pratinjau tautan tidak pernah
// mengirim Sec-Fetch-Dest. Menempelkan header Chrome pada User-Agent Safari
// justru menandai permintaan itu sebagai palsu — lebih mencolok daripada tidak
// menyamar sama sekali.
//
// Karena itu penyamaran disimpan sebagai satu paket utuh, bukan sebagai daftar
// User-Agent yang headernya ditambal belakangan.
//
// Diukur langsung terhadap halaman profil sungguhan pada 11 Sep 2026:
//
//	ponsel    → TikTok menyajikan halaman SSR penuh berisi JSON statsV2 dengan
//	            angka EKSAK; Instagram menyajikan blok JSON dan/atau og:description.
//	pratinjau → halaman pratinjau tautan berisi og:description saja. Kedua
//	            platform melayaninya karena tautan mereka harus bisa dipratinjau
//	            di WhatsApp — di situlah celahnya.
//	meja      → ditolak keduanya. Disimpan sebagai percobaan terakhir kalau
//	            keadaannya berbalik.
//
// Jangan menyusun ulang urutannya tanpa mengukur lagi.
type socialSamaran struct {
	nama   string
	ua     string
	header map[string]string
}

// Header Accept-Encoding SENGAJA tidak diisi di mana pun. Go menambahkannya
// sendiri dan membuka kompresinya secara transparan — begitu kita mengisinya
// manual, Go berhenti membuka dan yang sampai ke parser adalah byte terkompresi.
var (
	samaranPonsel = socialSamaran{
		nama: "ponsel",
		ua:   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
		header: map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language":           "en-US,en;q=0.9",
			"Upgrade-Insecure-Requests": "1",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
		},
	}
	// Perayap pratinjau mengirim header seadanya. Menambahinya Sec-Fetch-*
	// akan membuatnya berbunyi seperti peramban yang mengaku-ngaku perayap.
	samaranPratinjau = socialSamaran{
		nama: "pratinjau",
		ua:   "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
		header: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
		},
	}
	samaranMeja = socialSamaran{
		nama: "meja",
		ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		header: map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			"Accept-Language":           "en-US,en;q=0.9",
			"Upgrade-Insecure-Requests": "1",
			"Sec-Ch-Ua":                 `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
		},
	}
)

// Ragam versi iOS untuk penyamaran ponsel. Dipilih berdasarkan NAMA AKUN, bukan
// diacak tiap permintaan: satu akun yang tiap hari dikunjungi "peramban" yang
// versinya berubah-ubah jauh lebih aneh daripada satu akun yang selalu
// dikunjungi peramban yang sama. Yang ingin ditiru adalah beberapa orang
// berbeda yang masing-masing setia pada ponselnya, bukan satu orang yang
// berganti ponsel tiap pagi.
var ragamIOS = []string{
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 16_7_8 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
}

func samaranPonselUntuk(kunci string) socialSamaran {
	h := fnv.New32a()
	_, _ = h.Write([]byte(kunci))
	sam := samaranPonsel
	sam.ua = ragamIOS[int(h.Sum32())%len(ragamIOS)]
	return sam
}

// socialFetch mengambil satu URL sebagai teks.
//
// Accept-Language dipaksa en-US karena jalur cadangan membaca kalimat
// og:description ("1,234 Followers, 56 Following") — dalam bahasa Indonesia
// kalimat itu berbunyi lain ("1.234 pengikut") dan pola pembacanya jadi harus
// bercabang mengikuti bahasa yang kebetulan dipilihkan server.
func socialFetch(rawURL string, sam socialSamaran, extraHeaders map[string]string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", sam.ua)
	for k, v := range sam.header {
		req.Header.Set(k, v)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := socialClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, socialMaxBody))
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		// Retry-After adalah permintaan eksplisit platform untuk berhenti.
		// Mengabaikannya adalah cara tercepat berpindah dari "dibatasi
		// sementara" ke "diblokir permanen".
		tunggu := 6 * time.Hour
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, err := strconv.Atoi(ra); err == nil && d > 0 {
				tunggu = time.Duration(d) * time.Second
			}
		}
		return "", &socialDitolak{
			pesan:  fmt.Sprintf("dibatasi platform (HTTP 429), diminta menunggu %s", tunggu.Round(time.Minute)),
			tunggu: tunggu,
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("akun tidak ditemukan (HTTP 404) — periksa ejaan username")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d dari %s", resp.StatusCode, hostOf(rawURL))
	}
	return string(body), nil
}

// socialJalanKeluar menempelkan dua langkah yang benar-benar bisa diambil orang
// yang membaca pesan ini di halaman Kinerja Markom. Pesan galat yang hanya
// menyebutkan apa yang rusak membuat orang mencoba lagi berkali-kali pada
// keadaan yang tidak akan berubah dengan dicoba lagi.
func socialJalanKeluar(sebab string) string {
	jalan := "isi angkanya manual di halaman Kinerja Markom"
	if strings.TrimSpace(os.Getenv("SOCIAL_PROXY_URL")) == "" {
		jalan = "pasang SOCIAL_PROXY_URL supaya permintaan keluar lewat alamat lain, atau " + jalan
	}
	return sebab + ". Langkah berikutnya: " + jalan + "."
}

func socialUkuran(n int) string {
	if n >= 1024 {
		return fmt.Sprintf("%d KB halaman", n/1024)
	}
	return fmt.Sprintf("%d byte", n)
}

func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Host
	}
	return rawURL
}

// ── Penolakan laju, dibedakan dari kegagalan biasa ──────────────────────────
//
// Dua sebab kegagalan menuntut tanggapan yang berlawanan. Akun yang salah ketik
// atau halaman yang berubah bentuk sebaiknya dicoba lagi besok — mencoba lagi
// tidak merugikan siapa pun. Permintaan yang DITOLAK karena lajunya sebaliknya:
// mencoba lagi justru mempercepat perjalanan dari "dibatasi sementara" menuju
// "diblokir permanen". Karena itu keduanya dibedakan di tingkat tipe, bukan
// dicocokkan dari kalimat galatnya.
type socialDitolak struct {
	pesan  string
	tunggu time.Duration
}

func (e *socialDitolak) Error() string { return e.pesan }

// socialIstirahat mengembalikan lama istirahat yang diminta platform, dan
// apakah galat ini memang penolakan laju.
func socialIstirahat(err error) (time.Duration, bool) {
	var d *socialDitolak
	if errors.As(err, &d) {
		return d.tunggu, true
	}
	return 0, false
}

// socialJeda menunggu di antara dua akun.
//
// Panjangnya ikut siapa yang menunggu, dan itu disengaja. Putaran terjadwal
// dini hari boleh berlama-lama: tidak ada yang menatap layar, dan merentangkan
// belasan permintaan ke dalam setengah jam membuatnya menyerupai orang yang
// sesekali membuka profil, bukan mesin yang menyapu daftar. Putaran yang
// diminta lewat tombol harus selesai selagi orangnya masih menunggu — karena
// itu jedanya dipendekkan, dan sebagai gantinya jumlah akunnya dibatasi.
func socialJeda(santai bool) {
	min, span := 5000, 8000 // diminta orang: 5–13 detik
	if santai {
		min, span = 45000, 75000 // terjadwal dini hari: 45–120 detik
	}
	time.Sleep(time.Duration(min+rand.Intn(span)) * time.Millisecond)
}

// ── Penerjemah angka ─────────────────────────────────────────────────────────

var reAbbrev = regexp.MustCompile(`(?i)^([\d.,]+)\s*([kmb]|rb|jt|ribu|juta)?$`)

// parseCount menerjemahkan angka yang ditulis manusia jadi bilangan bulat.
// Mengembalikan approx=true bila angkanya disingkat, karena "12,3 rb" bisa
// berarti apa pun antara 12.250 dan 12.349 — selisih 99 pengikut yang akan
// terbaca sebagai pertumbuhan kalau diperlakukan sebagai angka pasti.
func parseCount(s string) (n int64, approx bool, ok bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", " "))
	m := reAbbrev.FindStringSubmatch(s)
	if m == nil {
		return 0, false, false
	}
	num, suffix := m[1], strings.ToLower(m[2])

	mult := int64(1)
	switch suffix {
	case "k", "rb", "ribu":
		mult = 1_000
	case "m", "jt", "juta":
		mult = 1_000_000
	case "b":
		mult = 1_000_000_000
	}

	if mult == 1 {
		// Angka penuh: titik dan koma keduanya pemisah ribuan, bukan desimal.
		clean := strings.NewReplacer(".", "", ",", "", " ", "").Replace(num)
		v, err := strconv.ParseInt(clean, 10, 64)
		if err != nil {
			return 0, false, false
		}
		return v, false, true
	}

	// Angka disingkat: pemisah terakhir adalah koma desimal (1.2K / 1,2 rb).
	norm := strings.ReplaceAll(num, ",", ".")
	if i := strings.LastIndex(norm, "."); i >= 0 {
		norm = strings.ReplaceAll(norm[:i], ".", "") + "." + norm[i+1:]
	}
	f, err := strconv.ParseFloat(norm, 64)
	if err != nil {
		return 0, false, false
	}
	return int64(f * float64(mult)), true, true
}

// ── Pembantu penelusuran JSON ────────────────────────────────────────────────
//
// Struktur JSON kedua platform berubah bentuk beberapa kali setahun. Menelusuri
// map[string]any lewat jalur yang boleh gagal jauh lebih tahan lama daripada
// struct bertag: yang hilang cuma satu kolom, bukan seluruh pembacaan.

func jat(v any, path ...string) any {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func jint(v any, path ...string) *int64 {
	switch x := jat(v, path...).(type) {
	case float64:
		n := int64(x)
		return &n
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
			return &n
		}
	}
	return nil
}

func jintOr0(v any, path ...string) int64 {
	if p := jint(v, path...); p != nil {
		return *p
	}
	return 0
}

func jstr(v any, path ...string) string {
	if s, ok := jat(v, path...).(string); ok {
		return s
	}
	return ""
}

// ── Instagram ────────────────────────────────────────────────────────────────

// scrapeInstagram mencoba tiga jalur, dari yang paling kaya:
//
//  1. web_profile_info — JSON yang dipakai web Instagram sendiri. Satu-satunya
//     jalur publik yang memberi angka per-konten (suka, komentar, tayangan
//     reel), jadi interaksi mingguan hanya hidup selama jalur ini jalan.
//  2. Blok JSON yang tertanam di HTML profil.
//  3. meta og:description — angka disingkat, hanya pengikut & jumlah post.
//
// Jangkauan/impresi akun TIDAK ADA di jalur mana pun. Itu angka Insights milik
// pemegang akun, bukan angka publik; yang bisa diambil di sini hanya tayangan
// per-reel. Kekurangan itu ditambal lewat isian manual, bukan diperkirakan.
func scrapeInstagram(username string) (*socialProfile, error) {
	profil := "https://www.instagram.com/" + url.PathEscape(username) + "/"
	var sebab []string
	diblokir := false
	catat := func(lapis, pesan string) { sebab = append(sebab, lapis+": "+pesan) }

	// Lapis 1 & 2 — penyamaran yang terbukti dilayani. Instagram menyembunyikan
	// seluruh angka dari peramban meja biasa, tetapi tetap menyerahkan
	// og:description kepada perayap pratinjau tautan dan kepada peramban ponsel,
	// karena tautannya harus bisa dipratinjau di WhatsApp.
	for _, sam := range []socialSamaran{samaranPratinjau, samaranPonselUntuk(username)} {
		html, err := socialFetch(profil, sam, nil)
		if err != nil {
			if _, ya := socialIstirahat(err); ya {
				diblokir = true
			}
			catat(sam.nama, err.Error())
			continue
		}
		if p := igGabung(html); p != nil {
			return p, nil
		}
		catat(sam.nama, socialUkuran(len(html))+" tanpa angka")
	}

	// Lapis 3 — endpoint web_profile_info. Inilah SATU-SATUNYA jalur yang
	// memberi angka per-konten (suka, komentar, tayangan reel), jadi tetap
	// dicoba meski paling sering ditolak: kalau ia lolos sekali saja, jangkauan
	// mingguan Instagram jadi angka sungguhan dan bukan tambalan tangan.
	if body, err := socialFetch(
		"https://www.instagram.com/api/v1/users/web_profile_info/?username="+url.QueryEscape(username),
		samaranMeja,
		map[string]string{
			"X-IG-App-ID": igWebAppID,
			"Referer":     profil,
			"Accept":      "application/json",
		}); err != nil {
		if _, ya := socialIstirahat(err); ya {
			diblokir = true
		}
		catat("web_profile_info", err.Error())
	} else if p := igParseProfileInfo(body); p != nil {
		return p, nil
	} else {
		catat("web_profile_info", "balasan tanpa data akun")
	}

	pesan := fmt.Sprintf("tidak satu pun jalur baca Instagram berhasil (%s) — %s",
		strings.Join(sebab, "; "),
		socialJalanKeluar("kemungkinan alamat ini sedang diblokir, atau nama akunnya salah"))
	if diblokir {
		return nil, &socialDitolak{pesan: pesan, tunggu: 6 * time.Hour}
	}
	return nil, fmt.Errorf("%s", pesan)
}

// igGabung menyatukan dua sumber angka yang hidup di halaman yang SAMA:
//
//	blok JSON tertanam → jumlah pengikut EKSAK (38.518), tetapi sering hanya itu;
//	meta og:description → pengikut yang sudah DIBULATKAN ("39K"), tetapi lengkap
//	                      dengan jumlah postingan dan jumlah yang diikuti, keduanya eksak.
//
// Keduanya tidak selalu muncul bersamaan — Instagram menyajikan cangkang yang
// berbeda-beda antar permintaan untuk alamat yang sama. Karena itu keduanya
// dibaca sekaligus lalu diambil yang terbaik per medan, bukan dipilih salah
// satu: memilih salah satu berarti kehilangan jumlah postingan (satu-satunya
// ukuran mingguan Instagram yang berguna) setiap kali blok JSON-nya muncul.
func igGabung(html string) *socialProfile {
	tertanam := igParseEmbeddedJSON(html)
	og := igParseOgDescription(html)
	if tertanam == nil && og == nil {
		return nil
	}
	if tertanam == nil {
		return og
	}
	if og == nil {
		return tertanam
	}

	// Pengikut dari blok JSON diutamakan karena eksak; Approx ikut angka yang
	// BENAR-BENAR dipakai, bukan sifat halamannya secara umum.
	p := &socialProfile{Followers: tertanam.Followers, Approx: tertanam.Approx, Via: "html-json+og"}
	if p.Followers == nil {
		p.Followers, p.Approx = og.Followers, og.Approx
	}
	p.PostsCount = tertanam.PostsCount
	if p.PostsCount == nil {
		p.PostsCount = og.PostsCount
	}
	p.Following = tertanam.Following
	if p.Following == nil {
		p.Following = og.Following
	}
	p.Posts = tertanam.Posts
	if len(p.Posts) == 0 {
		p.Posts = og.Posts
	}
	return p
}

func igParseProfileInfo(body string) *socialProfile {
	var root any
	if json.Unmarshal([]byte(body), &root) != nil {
		return nil
	}
	user := jat(root, "data", "user")
	if user == nil {
		return nil
	}
	fol := jint(user, "edge_followed_by", "count")
	if fol == nil {
		return nil
	}

	p := &socialProfile{
		Followers:  fol,
		Following:  jint(user, "edge_follow", "count"),
		PostsCount: jint(user, "edge_owner_to_timeline_media", "count"),
		Via:        "web_profile_info",
	}

	edges, _ := jat(user, "edge_owner_to_timeline_media", "edges").([]any)
	for _, e := range edges {
		node := jat(e, "node")
		if node == nil {
			continue
		}
		code := jstr(node, "shortcode")
		ts := jintOr0(node, "taken_at_timestamp")
		if code == "" || ts == 0 {
			continue
		}
		// Nama medan jumlah suka berganti-ganti antar versi; yang mana pun yang
		// ada dipakai, dan post yang sukanya disembunyikan tetap terhitung 0.
		likes := jintOr0(node, "edge_liked_by", "count")
		if likes == 0 {
			likes = jintOr0(node, "edge_media_preview_like", "count")
		}
		caption := ""
		if cap0, ok := jat(node, "edge_media_to_caption", "edges").([]any); ok && len(cap0) > 0 {
			caption = jstr(cap0[0], "node", "text")
		}
		views := jintOr0(node, "video_view_count")
		if views == 0 {
			views = jintOr0(node, "video_play_count")
		}
		p.Posts = append(p.Posts, socialPostRaw{
			Ref:       code,
			PostedAt:  time.Unix(ts, 0).UTC(),
			Permalink: "https://www.instagram.com/p/" + code + "/",
			Caption:   trimCaption(caption),
			Views:     views,
			Likes:     likes,
			Comments:  jintOr0(node, "edge_media_to_comment", "count"),
		})
	}
	return p
}

var (
	reIgFollowers = regexp.MustCompile(`"edge_followed_by":\{"count":(\d+)\}`)
	reIgFollowing = regexp.MustCompile(`"edge_follow":\{"count":(\d+)\}`)
	reIgPosts     = regexp.MustCompile(`"edge_owner_to_timeline_media":\{"count":(\d+)`)
	reIgFollower2 = regexp.MustCompile(`"follower_count":(\d+)`)
	reIgMedia2    = regexp.MustCompile(`"media_count":(\d+)`)
)

func igParseEmbeddedJSON(html string) *socialProfile {
	pick := func(re *regexp.Regexp) *int64 {
		if m := re.FindStringSubmatch(html); m != nil {
			if v, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				return &v
			}
		}
		return nil
	}
	fol := pick(reIgFollowers)
	if fol == nil {
		fol = pick(reIgFollower2)
	}
	if fol == nil {
		return nil
	}
	posts := pick(reIgPosts)
	if posts == nil {
		posts = pick(reIgMedia2)
	}
	return &socialProfile{
		Followers:  fol,
		Following:  pick(reIgFollowing),
		PostsCount: posts,
		Via:        "html-json",
	}
}

var (
	reOgDesc = regexp.MustCompile(`(?i)<meta[^>]+property="og:description"[^>]+content="([^"]*)"`)
	// "1,234 Followers, 567 Following, 89 Posts" — dengan angka boleh disingkat.
	reIgOgTriple = regexp.MustCompile(`(?i)([\d.,]+\s*[kmb]?)\s+Followers?,\s*([\d.,]+\s*[kmb]?)\s+Following,\s*([\d.,]+\s*[kmb]?)\s+Posts?`)
)

func igParseOgDescription(html string) *socialProfile {
	m := reOgDesc.FindStringSubmatch(html)
	if m == nil {
		return nil
	}
	desc := htmlUnescapeLite(m[1])
	t := reIgOgTriple.FindStringSubmatch(desc)
	if t == nil {
		return nil
	}
	fol, approx, ok := parseCount(t[1])
	if !ok {
		return nil
	}
	p := &socialProfile{Followers: &fol, Approx: approx, Via: "og:description"}
	if v, a, ok := parseCount(t[2]); ok {
		p.Following = &v
		p.Approx = p.Approx || a
	}
	if v, a, ok := parseCount(t[3]); ok {
		p.PostsCount = &v
		p.Approx = p.Approx || a
	}
	return p
}

// ── TikTok ───────────────────────────────────────────────────────────────────

var (
	reTtUniversal = regexp.MustCompile(`(?s)<script id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>(.*?)</script>`)
	reTtSigi      = regexp.MustCompile(`(?s)<script id="SIGI_STATE"[^>]*>(.*?)</script>`)
	// "Nusantara Kopi (@nusantara.kopi) on TikTok | 12.3K Likes. 4567 Followers."
	reTtOgFollow    = regexp.MustCompile(`(?i)([\d.,]+\s*[kmb]?)\s+Followers?`)
	reTtOgFollowing = regexp.MustCompile(`(?i)([\d.,]+\s*[kmb]?)\s+Following`)
	reTtOgLikes     = regexp.MustCompile(`(?i)([\d.,]+\s*[kmb]?)\s+Likes?`)
)

// scrapeTikTok membaca halaman profil TikTok.
//
// Berbeda dari Instagram, TikTok menayangkan jumlah tonton tiap video secara
// publik — jadi jangkauan mingguan di sini angka sungguhan, bukan tambalan.
// Yang tidak selalu ada justru daftar videonya: sebagian versi halaman hanya
// menyertakan statistik akun dan memuat daftar video lewat permintaan
// terpisah yang bertanda tangan. Ketika itu terjadi, jumlah pengikut tetap
// terbaca dan bagian kontennya dibiarkan kosong.
func scrapeTikTok(username string) (*socialProfile, error) {
	profil := "https://www.tiktok.com/@" + url.PathEscape(username)
	var sebab []string
	diblokir := false

	catat := func(lapis, pesan string) {
		sebab = append(sebab, lapis+": "+pesan)
	}

	// Lapis 1 — penyamaran peramban ponsel. Satu-satunya yang dilayani halaman
	// SSR penuh, dan karena itu satu-satunya yang memberi angka eksak.
	if html, err := socialFetch(profil, samaranPonselUntuk(username), nil); err != nil {
		if _, ya := socialIstirahat(err); ya {
			diblokir = true
		}
		catat("ponsel", err.Error())
	} else if p := ttParseHTML(html, username); p != nil {
		return p, nil
	} else if strings.Contains(html, "_wafchallengeid") || strings.Contains(html, "wafchallenge") {
		// Tantangan keamanan adalah penolakan, bukan halaman yang berubah:
		// mencoba lagi berulang kali hanya memperdalam blokirnya.
		diblokir = true
		catat("ponsel", "dibalas tantangan keamanan")
	} else {
		catat("ponsel", socialUkuran(len(html))+" tanpa data akun")
	}

	// Lapis 2 — penyamaran crawler pratinjau tautan. Angkanya sudah dibulatkan
	// platform, jadi ditandai approx dan tidak dipakai menghitung pertumbuhan
	// sampai satuan.
	if html, err := socialFetch(profil, samaranPratinjau, nil); err != nil {
		if _, ya := socialIstirahat(err); ya {
			diblokir = true
		}
		catat("pratinjau", err.Error())
	} else if p := ttParseOgDescription(html); p != nil {
		return p, nil
	} else {
		catat("pratinjau", socialUkuran(len(html))+" tanpa og:description")
	}

	// Lapis 3 — penyamaran peramban meja. Diketahui ditolak; dicoba terakhir
	// kalau-kalau keadaannya berbalik.
	if html, err := socialFetch(profil, samaranMeja, nil); err == nil {
		if p := ttParseHTML(html, username); p != nil {
			return p, nil
		}
		if p := ttParseOgDescription(html); p != nil {
			return p, nil
		}
	}

	pesan := fmt.Sprintf("tidak satu pun jalur baca TikTok berhasil (%s) — %s",
		strings.Join(sebab, "; "),
		socialJalanKeluar("kemungkinan alamat ini sedang diblokir, atau bentuk halamannya berubah"))
	if diblokir {
		return nil, &socialDitolak{pesan: pesan, tunggu: 6 * time.Hour}
	}
	return nil, fmt.Errorf("%s", pesan)
}

// ttParseHTML memungut blok data yang tertanam di halaman profil, apa pun
// bungkusnya. Dua bungkus yang pernah dipakai TikTok sama-sama dicoba.
func ttParseHTML(html, username string) *socialProfile {
	if m := reTtUniversal.FindStringSubmatch(html); m != nil {
		if p := ttParseUniversal(m[1]); p != nil {
			return p
		}
	}
	if m := reTtSigi.FindStringSubmatch(html); m != nil {
		if p := ttParseSigi(m[1], username); p != nil {
			return p
		}
	}
	return nil
}

func ttParseUniversal(raw string) *socialProfile {
	var root any
	if json.Unmarshal([]byte(raw), &root) != nil {
		return nil
	}
	detail := jat(root, "__DEFAULT_SCOPE__", "webapp.user-detail")

	// statsV2 SELALU didahulukan. Kedua blok memuat medan yang sama, tetapi
	// "stats" sudah dibulatkan platform untuk tampilan (54.500, 1.200.000)
	// sementara statsV2 menyimpan angka sebenarnya sebagai teks (54.453,
	// 1.150.451). Memakai yang dibulatkan membuat pertambahan mingguan di bawah
	// seratus pengikut lenyap sama sekali — dan justru sebesar itulah gerak
	// mingguan akun outlet yang sehat.
	stats := jat(detail, "userInfo", "statsV2")
	fol := jint(stats, "followerCount")
	if fol == nil {
		stats = jat(detail, "userInfo", "stats")
		fol = jint(stats, "followerCount")
	}
	if fol == nil {
		return nil
	}
	hearts := jint(stats, "heartCount")
	if hearts == nil {
		hearts = jint(stats, "heart")
	}
	p := &socialProfile{
		Followers:  fol,
		Following:  jint(stats, "followingCount"),
		PostsCount: jint(stats, "videoCount"),
		LikesTotal: hearts,
		Via:        "universal-data",
	}
	p.Posts = ttCollectItems(jat(root, "__DEFAULT_SCOPE__", "webapp.user-detail", "itemList"))
	return p
}

func ttParseSigi(raw, username string) *socialProfile {
	var root any
	if json.Unmarshal([]byte(raw), &root) != nil {
		return nil
	}
	// UserModule.stats dikunci nama pengguna apa adanya; kalau ejaannya berbeda
	// kapitalisasi, entri pertama yang ada dipakai — halaman ini hanya pernah
	// memuat satu akun.
	statsMap, _ := jat(root, "UserModule", "statsV2").(map[string]any)
	if statsMap == nil {
		statsMap, _ = jat(root, "UserModule", "stats").(map[string]any)
	}
	var stats any
	if statsMap != nil {
		if v, ok := statsMap[username]; ok {
			stats = v
		} else {
			for _, v := range statsMap {
				stats = v
				break
			}
		}
	}
	fol := jint(stats, "followerCount")
	if fol == nil {
		return nil
	}
	hearts := jint(stats, "heartCount")
	if hearts == nil {
		hearts = jint(stats, "heart")
	}
	p := &socialProfile{
		Followers:  fol,
		Following:  jint(stats, "followingCount"),
		PostsCount: jint(stats, "videoCount"),
		LikesTotal: hearts,
		Via:        "sigi-state",
	}
	if items, ok := jat(root, "ItemModule").(map[string]any); ok {
		list := make([]any, 0, len(items))
		for _, v := range items {
			list = append(list, v)
		}
		p.Posts = ttCollectItems(list)
	}
	return p
}

// ttCollectItems menerima daftar video apa pun bentuknya (array dari itemList
// atau nilai-nilai ItemModule) dan memungut yang punya id + waktu terbit.
func ttCollectItems(v any) []socialPostRaw {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []socialPostRaw
	for _, it := range list {
		id := jstr(it, "id")
		created := jintOr0(it, "createTime")
		if id == "" || created == 0 {
			continue
		}
		author := jstr(it, "author", "uniqueId")
		if author == "" {
			author = jstr(it, "author")
		}
		st := jat(it, "stats")
		if st == nil {
			st = jat(it, "statsV2")
		}
		out = append(out, socialPostRaw{
			Ref:       id,
			PostedAt:  time.Unix(created, 0).UTC(),
			Permalink: fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", author, id),
			Caption:   trimCaption(jstr(it, "desc")),
			Views:     jintOr0(st, "playCount"),
			Likes:     jintOr0(st, "diggCount"),
			Comments:  jintOr0(st, "commentCount"),
			Shares:    jintOr0(st, "shareCount"),
		})
	}
	return out
}

func ttParseOgDescription(html string) *socialProfile {
	m := reOgDesc.FindStringSubmatch(html)
	if m == nil {
		return nil
	}
	desc := htmlUnescapeLite(m[1])
	f := reTtOgFollow.FindStringSubmatch(desc)
	if f == nil {
		return nil
	}
	fol, approx, ok := parseCount(f[1])
	if !ok {
		return nil
	}
	p := &socialProfile{Followers: &fol, Approx: approx, Via: "og:description"}
	if f := reTtOgFollowing.FindStringSubmatch(desc); f != nil {
		if v, a, ok := parseCount(f[1]); ok {
			p.Following = &v
			p.Approx = p.Approx || a
		}
	}
	if l := reTtOgLikes.FindStringSubmatch(desc); l != nil {
		if v, a, ok := parseCount(l[1]); ok {
			p.LikesTotal = &v
			p.Approx = p.Approx || a
		}
	}
	return p
}

// ── Perkakas kecil ───────────────────────────────────────────────────────────

// htmlUnescapeLite membuka entitas yang benar-benar muncul di atribut meta.
// Memakai html.UnescapeString utuh juga benar, tetapi lima entitas ini yang
// nyata terjadi dan cakupan sekecil ini lebih mudah dipastikan perilakunya.
func htmlUnescapeLite(s string) string {
	return strings.NewReplacer(
		"&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">",
		"&nbsp;", " ",
	).Replace(s)
}

// trimCaption memotong keterangan konten. Isinya tidak dipakai menghitung
// apa pun — hanya untuk mengenali konten saat menelusuri angka yang aneh —
// jadi tidak ada gunanya menyimpan keterangan sepanjang layar.
func trimCaption(s string) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

// scrapeProfile mengarahkan ke pengambil sesuai platform.
func scrapeProfile(platform, username string) (*socialProfile, error) {
	switch platform {
	case models.SocialInstagram:
		return scrapeInstagram(username)
	case models.SocialTiktok:
		return scrapeTikTok(username)
	}
	return nil, fmt.Errorf("platform tidak dikenal: %s", platform)
}
