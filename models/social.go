package models

import "time"

// ── Kinerja Markom ───────────────────────────────────────────────────────────
//
// Instagram dan TikTok dipakai sebagai tolok ukur kedua di Analisa Bisnis.
// Halaman itu memisahkan dua sebab penurunan penjualan: eksekusi outlet, atau
// pasar/Markom. Medsos adalah bukti yang selama ini hilang untuk sebab kedua —
// tanpa angka jangkauan, "pasarnya memang sepi" dan "kita berhenti memposting"
// terlihat persis sama di grafik penjualan.
//
// Sumber angkanya halaman profil publik, bukan API resmi. Konsekuensinya harus
// dipikul di struktur data, bukan disembunyikan: tiap angka membawa keterangan
// dari mana ia datang, dan minggu yang tidak berhasil diambil dibiarkan kosong
// (nil) alih-alih diisi nol. Nol berarti "tidak ada pengikut"; kosong berarti
// "tidak tahu". Dua hal itu berbeda, dan menyamakannya membuat grafik berbohong
// justru pada minggu ketika scraper-nya sedang diblokir.

// Platform yang didukung.
const (
	SocialInstagram = "instagram"
	SocialTiktok    = "tiktok"
)

// Asal satu angka. Dipakai di snapshot maupun di ringkasan mingguan.
const (
	SocialSourceScrape = "scrape" // diambil scraper dari halaman publik
	SocialSourceManual = "manual" // diketik orang, menambal yang tak bisa di-scrape
)

// SocialAccount = satu akun medsos milik satu outlet.
//
// LastError sengaja disimpan dan ditampilkan apa adanya: scraping profil publik
// PASTI gagal sesekali (rate limit, halaman berubah, akun dikunci). Kegagalan
// yang diam-diam jauh lebih berbahaya daripada kegagalan yang kelihatan —
// grafiknya tetap tergambar, hanya berhenti bertambah, dan tidak ada yang sadar
// selama berminggu-minggu.
type SocialAccount struct {
	ID         string `json:"id"`
	OutletID   string `json:"outlet_id"`
	OutletCode string `json:"outlet_code,omitempty"`
	OutletName string `json:"outlet_name,omitempty"`
	Platform   string `json:"platform"`
	Username   string `json:"username"`
	ProfileURL string `json:"profile_url"`
	IsActive   bool   `json:"is_active"`

	// AutoFetch = angkanya dijemput penjadwal, bukan diantar orang.
	//
	// Instagram lahir dengan nilai false: halaman profil publiknya tidak
	// menyerahkan satu angka pun ke server (HTTP 429, lalu HTML tanpa angka),
	// dan jangkauan IG memang tidak pernah publik walau penarikannya tembus.
	// Menjadwalkan penarikan yang pasti gagal bukan cuma sia-sia — kegagalannya
	// menandai outletnya mandek dan mengeluarkannya dari grafik.
	//
	// Bisa dinyalakan per akun kalau suatu saat dipasang proxy yang tembus.
	AutoFetch bool `json:"auto_fetch"`

	LastScrapedAt *time.Time `json:"last_scraped_at"` // percobaan terakhir, berhasil atau tidak
	LastOkAt      *time.Time `json:"last_ok_at"`      // keberhasilan terakhir
	LastError     string     `json:"last_error"`

	// Angka terakhir yang berhasil dibaca — untuk daftar akun, bukan untuk grafik.
	Followers   *int64 `json:"followers"`
	PostsCount  *int64 `json:"posts_count"`
	LastCapture string `json:"last_capture"` // YYYY-MM-DD, kosong bila belum pernah

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SocialAccountRequest struct {
	OutletID string `json:"outlet_id"`
	Platform string `json:"platform"`
	// Boleh diisi username polos (@nusantara.kopi) atau URL profil penuh —
	// keduanya diuraikan jadi username + URL kanonik di service.
	Username  string `json:"username"`
	IsActive  *bool  `json:"is_active"`
	AutoFetch *bool  `json:"auto_fetch"`
}

// SocialManualWeek = tambalan manual untuk satu akun pada satu minggu.
//
// Kolomnya nullable dan hanya yang terisi yang menimpa hasil scrape. Ini bukan
// kenyamanan: jangkauan Instagram TIDAK ADA di halaman publik — angka itu hanya
// hidup di layar Insights milik pemegang akun. Satu-satunya cara jujur
// mendapatkannya tanpa API resmi adalah diketik orang yang membuka Insights-nya.
type SocialManualWeek struct {
	AccountID  string `json:"account_id"`
	WeekStart  string `json:"week_start"` // Senin, YYYY-MM-DD
	Followers  *int64 `json:"followers"`
	Posts      *int64 `json:"posts"`
	Views      *int64 `json:"views"`
	Engagement *int64 `json:"engagement"`
	Note       string `json:"note"`
	UpdatedBy  string `json:"updated_by"`
	UpdatedAt  string `json:"updated_at"`
}

// SocialWeeklyRow = satu akun pada satu minggu, sudah digabung antara hasil
// tarikan dan tambalan manualnya. Dipakai halaman Kinerja Markom untuk
// menampilkan apa adanya isi tiap minggu — termasuk minggu yang kosong, karena
// minggu kosong justru yang perlu diisi orang.
type SocialWeeklyRow struct {
	AccountID  string `json:"account_id"`
	OutletCode string `json:"outlet_code"`
	OutletName string `json:"outlet_name"`
	Platform   string `json:"platform"`
	Username   string `json:"username"`
	WeekStart  string `json:"week_start"`
	Followers  *int64 `json:"followers"`
	Posts      int64  `json:"posts"`
	Views      int64  `json:"views"`
	Engagement int64  `json:"engagement"`
	Manual     bool   `json:"manual"`
	// Approx = angka pengikut minggu itu berasal dari pratinjau yang sudah
	// dibulatkan platform, jadi selisihnya tidak bisa dipercaya sampai satuan.
	Approx bool `json:"approx"`
}

// SocialScrapeResult = laporan satu putaran penarikan, untuk ditampilkan
// setelah tombol "Tarik sekarang" ditekan.
type SocialScrapeResult struct {
	Attempted int      `json:"attempted"`
	OK        int      `json:"ok"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
	TookMs    int64    `json:"took_ms"`
}

// ── Bahan grafik di Analisa Bisnis ───────────────────────────────────────────

// BizSocialWeek = satu minggu medsos, untuk satu outlet atau untuk seluruh grup.
//
// Followers nil = minggu itu tidak ada pembacaan sama sekali. Gain nil = tidak
// ada dua pembacaan berurutan yang bisa dikurangkan. Keduanya digambar sebagai
// garis putus, bukan sebagai nol.
type BizSocialWeek struct {
	WeekStart string `json:"week_start"`
	Label     string `json:"label"`
	Followers *int64 `json:"followers"`
	Gain      *int64 `json:"gain"`
	Posts     int64  `json:"posts"`
	Views     int64  `json:"views"`
	// Engagement = suka + komentar + bagikan pada konten yang TERBIT minggu itu.
	Engagement int64 `json:"engagement"`
	// HasManual menandai minggu yang sebagian angkanya diketik orang. Ditandai di
	// grafik supaya pembaca tahu mana yang mesin dan mana yang tangan.
	HasManual bool `json:"has_manual"`
	// FollowersApprox = jumlah pengikut minggu ini disusun dari angka yang sudah
	// dibulatkan platform ("39K"). Selisih antar minggu pada angka seperti itu
	// bisa seluruhnya lahir dari pembulatan, bukan dari pengikut yang benar-benar
	// bertambah.
	FollowersApprox bool `json:"followers_approx"`
}

// BizSocialAccountRef = akun yang menyusun angka satu outlet, beserta keadaan
// pengambilan terakhirnya.
type BizSocialAccountRef struct {
	Platform  string `json:"platform"`
	Username  string `json:"username"`
	URL       string `json:"url"`
	Followers *int64 `json:"followers"`
	// AutoFetch false = akun ini memang diisi tangan, bukan sedang rusak.
	// Dibedakan supaya akun manual tidak pernah digambar sebagai kegagalan.
	AutoFetch bool `json:"auto_fetch"`
	// Stale = akun ini sudah lebih dari sepekan tidak berhasil dibaca. Hanya
	// berlaku untuk akun yang memang dijadwalkan menarik: akun yang angkanya
	// diantar orang tidak bisa "mandek", ia cuma belum diisi. Angkanya masih
	// ikut ditampilkan, tetapi outlet-nya diberi tanda supaya perbandingan
	// antar-outlet tidak dibaca sebagai setara padahal salah satu datanya mandek.
	Stale     bool   `json:"stale"`
	LastError string `json:"last_error,omitempty"`
}

// Nilai Quadrant pada BizSocialOutlet — pembacaan silang medsos × penjualan.
//
// Inilah alasan grafiknya ditempel di Analisa Bisnis dan bukan berdiri sebagai
// laporan sendiri: satu outlet yang penjualannya turun punya dua cerita yang
// sangat berbeda tergantung medsosnya ikut sepi atau tidak, dan tindakannya
// jatuh ke orang yang berbeda pula.
const (
	BizSocSejalan   = "SEJALAN"      // medsos naik, penjualan naik
	BizSocSepiDua   = "SEPI_DUANYA"  // medsos turun, penjualan turun → mesin promosi berhenti
	BizSocRamaiSepi = "RAMAI_SEPI"   // medsos naik, penjualan turun → berhenti di outlet
	BizSocTanpaMed  = "TANPA_MEDSOS" // medsos turun, penjualan naik → penjualan tidak bergantung medsos
	BizSocKurang    = "DATA_KURANG"
)

type BizSocialOutlet struct {
	OutletID string                `json:"outlet_id"`
	Code     string                `json:"code"`
	Name     string                `json:"name"`
	Accounts []BizSocialAccountRef `json:"accounts"`
	Weeks    []BizSocialWeek       `json:"weeks"`

	FollowersNow *int64 `json:"followers_now"`

	// Blok terakhir vs blok sebelumnya — panjang bloknya sama persis dengan yang
	// dipakai penjualan (BusinessAnalysis.BlockWeeks), supaya dua angka yang
	// disandingkan benar-benar menjelaskan rentang waktu yang sama.
	GainRecent  int64 `json:"gain_recent"`
	GainPrev    int64 `json:"gain_prev"`
	PostsRecent int64 `json:"posts_recent"`
	PostsPrev   int64 `json:"posts_prev"`
	ViewsRecent int64 `json:"views_recent"`
	ViewsPrev   int64 `json:"views_prev"`
	EngRecent   int64 `json:"eng_recent"`
	EngPrev     int64 `json:"eng_prev"`

	// ReachGrowth = gerak JANGKAUAN blok terakhir vs sebelumnya, persen. Ini yang
	// dipakai sebagai sumbu medsos — bukan pertumbuhan pengikut. Pengikut adalah
	// warisan: akun yang setahun tidak diurus tetap punya puluhan ribu pengikut,
	// jadi angkanya nyaris tidak bergerak dan tidak bisa menjelaskan naik-turun
	// penjualan minggu ini.
	ReachGrowth    *float64 `json:"reach_growth"`
	FollowerGrowth *float64 `json:"follower_growth"`

	// FollowerApprox = pertumbuhan pengikut di atas dihitung dari angka yang
	// sudah dibulatkan platform, jadi tidak boleh dibaca sampai satu desimal.
	//
	// Instagram menyerahkan "39K Followers" kepada perayap pratinjau — satu
	// langkah pembulatannya 500 pengikut, sementara pertambahan mingguan akun
	// outlet yang sehat biasanya ratusan. Tanpa penanda ini, angka yang
	// seluruhnya lahir dari pembulatan akan tampil sebagai "naik 2,6%" dan
	// dipakai menilai kerja orang.
	FollowerApprox bool `json:"follower_approx"`

	// ReachBasis menyebut angka mana yang dipakai sebagai sumbu medsos:
	// "tayangan" bila jumlah tonton terbaca, "interaksi" bila tidak. Instagram
	// tidak pernah menayangkan jumlah tonton untuk foto, jadi outlet yang hanya
	// ber-Instagram sering hanya punya suka + komentar. Menyamakan keduanya
	// diam-diam akan membandingkan outlet TikTok (satuan puluhan ribu) dengan
	// outlet Instagram (satuan ratusan) seolah sebanding — karena itu yang
	// dibandingkan selalu PERSENTASE geraknya, dan dasarnya disebutkan.
	ReachBasis string `json:"reach_basis"`

	// Stale = ada akun outlet ini yang sudah lebih sepekan gagal dibaca. Outlet
	// seperti ini dikeluarkan dari grafik sebar dan dari korelasi: jangkauan yang
	// jatuh ke nol karena scraper diblokir tidak boleh terbaca sebagai medsos
	// yang sepi.
	Stale bool `json:"stale"`

	// SalesGrowth disalin dari BizOutlet.Growth4 supaya grafik sebar tidak perlu
	// menggabungkan dua sumber angka di sisi UI.
	SalesGrowth *float64 `json:"sales_growth"`

	Quadrant      string `json:"quadrant"`
	QuadrantLabel string `json:"quadrant_label"`
	Reading       string `json:"reading"`  // kalimat pembacaan, dirakit dari angka
	Coverage      string `json:"coverage"` // dari mana angka outlet ini datang
}

// BizSocial = seluruh blok medsos yang ditempelkan ke laporan Analisa Bisnis.
type BizSocial struct {
	// Enabled false = belum ada satu pun akun terdaftar. Bagian medsos di halaman
	// disembunyikan seluruhnya, bukan digambar kosong.
	Enabled    bool `json:"enabled"`
	BlockWeeks int  `json:"block_weeks"`

	Group   []BizSocialWeek   `json:"group"`
	Outlets []BizSocialOutlet `json:"outlets"`

	GroupReachGrowth    *float64 `json:"group_reach_growth"`
	GroupFollowerGrowth *float64 `json:"group_follower_growth"`
	GroupPostsRecent    int64    `json:"group_posts_recent"`
	GroupPostsPrev      int64    `json:"group_posts_prev"`

	// Correlation = keeratan gerak medsos dengan gerak penjualan antar-outlet
	// pada blok ini (Pearson, −1..1). nil bila outlet berdata lengkap < 3:
	// korelasi dari dua titik selalu ±1 dan tidak berarti apa-apa.
	Correlation *float64 `json:"correlation"`
	CorrCount   int      `json:"corr_count"`

	// OutletsNoAccount = outlet yang ikut dinilai penjualannya tetapi belum
	// punya akun terdaftar. Disebut terang-terangan supaya kolom medsos yang
	// kosong tidak terbaca sebagai "outlet ini tidak bermedsos".
	OutletsNoAccount []string `json:"outlets_no_account"`
	StaleAccounts    []string `json:"stale_accounts"`

	Notes []string `json:"notes"`
}
