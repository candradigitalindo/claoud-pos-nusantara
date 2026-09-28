package models

// ── Analisa Bisnis (RGI / anti-alibi) ────────────────────────────────────────
// Memisahkan pergerakan yang dialami SELURUH grup (pasar & program Markom) dari
// pergerakan yang khas satu outlet (eksekusi manajer outlet).
//
// Tiga lapis pembersihan sebelum ada angka yang boleh menilai orang:
//
//  1. Kalender. Bisnis ini hidup dari akhir pekan dan hari libur: 58% omzet
//     mingguan jatuh di Sabtu–Minggu, dan satu hari libur nasional di hari
//     kerja bernilai 4–6 hari kerja biasa. Minggu yang memuat libur karena itu
//     diskalakan ke "minggu biasa" lebih dulu (NetAdj = penjualan setara
//     kalender). Tanpa ini, September yang tanpa libur selalu terbaca "pasar
//     turun" dibanding Agustus yang punya 17 Agustus dan Maulid — dan Markom
//     ditagih atas kalender.
//  2. Pembanding sejawat. RGI (Relative Growth Index) = growth outlet − nilai
//     tengah growth seluruh outlet pembanding, bersatuan poin persen. Guncangan
//     yang dialami semua outlet bersamaan (cuaca, daya beli) hilang di sini.
//  3. Derau. Selisih baru dianggap nyata bila melewati batas wajar outlet itu
//     (2 galat baku, dari gabungan derau outlet sendiri dan model derau grup
//     yang mengecil seiring banyaknya struk) DAN konsisten arahnya pada
//     minggu-minggu terakhir.
//
// Pembandingnya memakai nilai tengah SEMUA outlet pembanding (tiap outlet satu
// suara, termasuk dirinya). Median hampir tidak terpengaruh oleh satu anggota,
// jadi outlet tidak bisa "menyeret" patokannya sendiri; dan berbeda dari
// pembanding tanpa-diri, patokannya sama untuk semua outlet sehingga tidak
// ada lompatan buatan di tengah urutan.

// BizWeek = satu minggu untuk satu outlet.
type BizWeek struct {
	WeekStart string  `json:"week_start"` // Senin, tanggal lokal (zona waktu aplikasi)
	Label     string  `json:"label"`      // 2026-W35
	Net       float64 `json:"net"`        // penjualan sebenarnya
	Trx       int     `json:"trx"`
	// NetAdj = penjualan setara kalender: minggu yang memuat libur atau libur
	// sekolah diskalakan ke minggu biasa memakai bobot hari outlet ini.
	// CalFactor = NetAdj / Net. Calendar = keterangan singkat kalender minggu
	// ini ("Libur: HUT RI (Sen)"), kosong bila minggu biasa.
	NetAdj    float64 `json:"net_adj"`
	CalFactor float64 `json:"cal_factor"`
	Calendar  string  `json:"calendar,omitempty"`
	// Index = perjalanan penjualan setara kalender, disandingkan pada level
	// pasar saat outlet ini mulai ikut terhitung. nil selama outlet belum punya
	// garis pasar sebagai titik sandar — digambar sebagai putus, bukan nol.
	Index  *float64 `json:"index"`
	Growth *float64 `json:"growth"` // % setara kalender terhadap minggu sebelumnya
	RGI    *float64 `json:"rgi"`    // growth − median growth outlet pembanding, poin persen
}

// BizGroupWeek = satu minggu untuk grup pembanding (panel same-store).
type BizGroupWeek struct {
	WeekStart string  `json:"week_start"`
	Label     string  `json:"label"`
	Net       float64 `json:"net"`     // total sebenarnya
	NetAdj    float64 `json:"net_adj"` // total setara kalender
	// OutletCount = berapa outlet berjualan pada minggu itu. Dipakai untuk
	// menjelaskan kenapa total penjualan bisa melompat tanpa growth ikut naik.
	OutletCount int      `json:"outlet_count"`
	Index       *float64 `json:"index"` // nil sebelum panel cukup untuk dirantai
	Growth      *float64 `json:"growth"`     // same-store, setara kalender
	GrowthRaw   *float64 `json:"growth_raw"` // same-store, angka mentah
	Calendar    string   `json:"calendar,omitempty"`
	// IsGroupEvent: pertumbuhan grup (setelah kalender dikoreksi) menyimpang
	// jauh dari kebiasaannya sendiri — cuaca, acara besar, gangguan. Minggu
	// seperti ini peristiwa pasar, bukan hasil kerja manajer.
	IsGroupEvent bool `json:"is_group_event"`
}

// Nilai Diagnosis pada BizOutlet.
const (
	BizDiagLeading = "UNGGUL"       // tumbuh meyakinkan di atas grup, konsisten
	BizDiagLagging = "TERTINGGAL"   // tertinggal meyakinkan dari grup, konsisten → isu eksekusi outlet
	BizDiagWatch   = "PANTAU"       // selisih mulai terlihat tetapi belum meyakinkan / belum konsisten
	BizDiagOnPace  = "SEIRAMA"      // tidak berbeda dari grup melebihi deraunya sendiri
	BizDiagWeak    = "SINYAL_LEMAH" // basis transaksi terlalu kecil/berayun untuk disimpulkan
	BizDiagNoData  = "DATA_KURANG"  // riwayat belum cukup untuk menyimpulkan
)

// Nilai Owner — siapa yang memegang masalahnya.
//
// BizOwnerMarkom dipakai untuk outlet yang eksekusinya seirama dengan
// sejawatnya tetapi pasarnya sendiri sedang turun SETELAH kalender dikoreksi:
// tidak ada yang perlu dibenahi di outlet itu. Sebab pasarnya sendiri masih
// harus dicari (cuaca, daya beli, pesaing, atau promosi yang mengendur) — blok
// medsos di halaman yang sama dipakai untuk mengujinya, bukan diasumsikan.
const (
	BizOwnerOutlet = "Manajer Outlet"
	BizOwnerMarkom = "Pasar / Markom"
	BizOwnerNone   = "—"
)

type BizOutlet struct {
	OutletID string    `json:"outlet_id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Net      float64   `json:"net"` // total sebenarnya sepanjang periode
	Trx      int       `json:"trx"`
	ATV      float64   `json:"atv"`
	Weeks    []BizWeek `json:"weeks"`

	Growth4    *float64 `json:"growth4"`     // growth blok akhir vs blok sebelumnya, setara kalender, %
	GrowthRaw4 *float64 `json:"growth_raw4"` // growth blok yang sama, angka mentah, %

	// PeerGrowth4 = nilai tengah growth blok seluruh outlet pembanding —
	// patokan yang dipakai menilai outlet ini. PeerCount = berapa outlet
	// menyusunnya (termasuk outlet ini bila ia ikut jadi pembanding).
	PeerGrowth4 *float64 `json:"peer_growth4"`
	PeerCount   int      `json:"peer_count"`

	RGI4 *float64 `json:"rgi4"` // growth4 − peer_growth4, poin persen

	// Threshold = batas wajar: 2 galat baku RGI blok, dari gabungan derau
	// outlet ini sendiri dan model derau grup (mengecil seiring banyaknya
	// struk). Outlet berderau tinggi butuh selisih lebih besar sebelum boleh
	// disebut tertinggal atau unggul.
	Threshold *float64 `json:"threshold"`

	// Consistency = berapa dari minggu-minggu blok terakhir yang RGI
	// mingguannya searah dengan RGI blok; ConsistencyNeed = minimum yang
	// disyaratkan sebelum selisih boleh divonis. Satu minggu buruk yang
	// menyeret satu blok tidak boleh terbaca sebagai kemerosotan.
	Consistency     int `json:"consistency"`
	ConsistencyNeed int `json:"consistency_need"`

	// ── Bahan laporan detail per outlet ─────────────────────────────────────
	// Manajer outlet membaca rupiah dan jumlah struk, bukan poin persen. Angka
	// di bawah ini menerjemahkan RGI ke satuan yang bisa langsung ditindak.

	RecentNet float64 `json:"recent_net"` // penjualan blok terakhir, sebenarnya
	PrevNet   float64 `json:"prev_net"`   // penjualan blok sebelumnya, sebenarnya
	RecentAdj float64 `json:"recent_adj"` // setara kalender
	PrevAdj   float64 `json:"prev_adj"`
	RecentTrx int     `json:"recent_trx"`
	PrevTrx   int     `json:"prev_trx"`

	// ExpectedNet = penjualan blok terakhir (rupiah sebenarnya) SEANDAINYA
	// outlet ini bergerak seperti outlet pembanding, dengan kalender blok
	// terakhir apa adanya. GapNet = kenyataan − seharusnya, dalam rupiah.
	ExpectedNet *float64 `json:"expected_net"`
	GapNet      *float64 `json:"gap_net"`

	// Breakdown = penurunan/kenaikan ini datang dari jumlah pengunjung atau
	// dari besar belanja per struk. Dua sebab yang penanganannya berbeda.
	Breakdown string `json:"breakdown"`

	// Advice = langkah yang disarankan, ditulis sebagai kalimat perintah biasa.
	Advice string `json:"advice"`

	InPanel bool `json:"in_panel"` // ikut jadi pembanding grup

	// Diagnosis dipakai untuk memilih warna; DiagnosisLabel adalah teks yang
	// ditampilkan. Labelnya ikut dari sini supaya tidak ada peta kata-kata yang
	// hidup terpisah di UI dan bisa menyimpang dari makna aslinya.
	Diagnosis      string `json:"diagnosis"`
	DiagnosisLabel string `json:"diagnosis_label"`
	Owner          string `json:"owner"`
	Note           string `json:"note"`

	// ModelNotes = kalimat pengamatan model statistik untuk outlet ini (tren,
	// perkiraan, minggu aneh, pergeseran level, pola hari), dirakit dari angka
	// layanan analitik. Kosong bila layanannya tidak aktif atau datanya pendek.
	ModelNotes []string     `json:"model_notes,omitempty"`
	Trend      *BizTrend    `json:"trend,omitempty"`
	Forecast   *BizForecast `json:"forecast,omitempty"`
}

// ── Model statistik (layanan analytics, Python) ──────────────────────────────
//
// Go mengirim deret setara kalender, Python mengembalikan angka, Go merakit
// kalimatnya. Semua metode tahan pencilan (Theil–Sen, MAD) karena riwayat toko
// wisata pendek dan berayun.

type BizTrend struct {
	SlopePct    float64 `json:"slope_pct"` // % per minggu, seluruh jendela
	Lo          float64 `json:"lo"`
	Hi          float64 `json:"hi"`
	Significant bool    `json:"significant"`
	Weeks       int     `json:"weeks"`

	RecentSlopePct    float64 `json:"recent_slope_pct"` // 8 minggu terakhir
	RecentLo          float64 `json:"recent_lo"`
	RecentHi          float64 `json:"recent_hi"`
	RecentSignificant bool    `json:"recent_significant"`
	RecentWeeks       int     `json:"recent_weeks"`
}

type BizForecastWeek struct {
	WeekStart string  `json:"week_start"`
	Label     string  `json:"label"`
	Adj       float64 `json:"adj"` // setara kalender
	Raw       float64 `json:"raw"` // sebenarnya (kalender minggu itu sudah dihitung)
	Lo        float64 `json:"lo"`
	Hi        float64 `json:"hi"`
	Factor    float64 `json:"factor"`
	Calendar  string  `json:"calendar,omitempty"`
}

type BizForecast struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	TotalRaw float64           `json:"total_raw"`
	TotalLo  float64           `json:"total_lo"`
	TotalHi  float64           `json:"total_hi"`
	Basis    string            `json:"basis"` // tren | datar
	Weeks    []BizForecastWeek `json:"weeks"`
}

type BizAnomaly struct {
	WeekStart string  `json:"week_start"`
	Label     string  `json:"label"`
	ActualAdj float64 `json:"actual_adj"`
	Lo        float64 `json:"lo"`
	Hi        float64 `json:"hi"`
	Z         float64 `json:"z"`
}

type BizChangepoint struct {
	WeekStart   string  `json:"week_start"`
	Label       string  `json:"label"`
	ShiftPct    float64 `json:"shift_pct"`
	BeforeMean  float64 `json:"before_mean"`
	AfterMean   float64 `json:"after_mean"`
	Score       float64 `json:"score"`
	Significant bool    `json:"significant"`
	AfterWeeks  int     `json:"after_weeks"`
}

type BizDowShift struct {
	WeekendPct *float64        `json:"weekend_pct"`
	WeekdayPct *float64        `json:"weekday_pct"`
	PerDow     map[string]float64 `json:"per_dow"` // "0" = Senin … "6" = Minggu
	WorstDow   int             `json:"worst_dow"`
	WorstPct   float64         `json:"worst_pct"`
	BestDow    int             `json:"best_dow"`
	BestPct    float64         `json:"best_pct"`
	NRecent    int             `json:"n_recent"`
	NPrior     int             `json:"n_prior"`
}

type BizModelResult struct {
	Trend       *BizTrend       `json:"trend,omitempty"`
	Forecast    *BizForecast    `json:"forecast,omitempty"`
	Anomalies   []BizAnomaly    `json:"anomalies"`
	Changepoint *BizChangepoint `json:"changepoint,omitempty"`
	Dow         *BizDowShift    `json:"dow,omitempty"`
	Notes       []string        `json:"notes"`
}

type BizModel struct {
	Enabled bool                       `json:"enabled"`
	Engine  string                     `json:"engine"`
	Error   string                     `json:"error,omitempty"`
	Group   *BizModelResult            `json:"group,omitempty"`
	Outlets map[string]*BizModelResult `json:"outlets"`
}

// ── Kalender bisnis ──────────────────────────────────────────────────────────

// BizCalendarDay = satu hari bertanda di kalender bisnis. Satu tanggal boleh
// punya beberapa tanda (libur nasional yang jatuh di masa libur sekolah).
type BizCalendarDay struct {
	Day       string `json:"day"` // YYYY-MM-DD
	Kind      string `json:"kind"`
	KindLabel string `json:"kind_label"`
	Name      string `json:"name"`
	Source    string `json:"source"` // seed | manual
	UpdatedBy string `json:"updated_by,omitempty"`
}

// BizCalendarModel = pengali kalender yang dipelajari dari data penjualan
// sendiri, ditampilkan supaya koreksinya bisa ditelusuri, bukan dipercaya buta.
type BizCalendarModel struct {
	// HolidayMult: hari libur nasional/cuti bersama yang jatuh di hari kerja
	// bernilai berapa kali hari kerja biasa yang sama. Estimated = dipelajari
	// dari HolidayObs hari libur yang teramati; false = memakai patokan awal
	// (hari libur dianggap seperti hari Minggu) karena pengamatannya belum ada.
	HolidayMult      float64 `json:"holiday_mult"`
	HolidayObs       int     `json:"holiday_obs"`
	HolidayEstimated bool    `json:"holiday_estimated"`

	SchoolWeekdayMult float64 `json:"school_weekday_mult"`
	SchoolWeekendMult float64 `json:"school_weekend_mult"`
	SchoolWeekdayObs  int     `json:"school_weekday_obs"`
	SchoolWeekendObs  int     `json:"school_weekend_obs"`
	SchoolEstimated   bool    `json:"school_estimated"`

	// DowShare = pangsa omzet tiap hari Senin..Minggu pada minggu biasa, persen,
	// seluruh grup. Menjelaskan kenapa satu hari libur bisa bernilai lima hari.
	DowShare [7]float64 `json:"dow_share"`

	// CoverageUntil = tanggal terakhir yang tercatat di kalender. Analisa yang
	// melewati tanggal ini berjalan tanpa koreksi libur — dan itu harus terlihat.
	CoverageUntil string `json:"coverage_until"`
}

// ── Narasi: teks halaman dirakit dari angka, bukan ditulis tetap ─────────────
//
// Semua kalimat penjelas di halaman ini disusun di sini, dari angka periode
// yang sedang dilihat. Kalimat tetap yang ditanam di UI cepat berbohong: ia
// tetap berbunyi sama ketika keadaannya sudah berubah, dan pembaca kehilangan
// kepercayaan pada seluruh halaman begitu menemukan satu kalimat yang tidak
// cocok dengan angkanya.

// Jenis temuan — menentukan penekanan visual, bukan isinya.
const (
	BizInsightProblem = "MASALAH"
	BizInsightChance  = "PELUANG"
	BizInsightWarning = "PERHATIAN"
	BizInsightNeutral = "NETRAL"
)

// BizInsight = satu temuan yang dirakit dari angka periode ini.
type BizInsight struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Outlets []string `json:"outlets,omitempty"`
	// Amount = nilai rupiah yang melekat pada temuan, bila ada. Temuan yang
	// bisa dinyatakan dalam rupiah jauh lebih mudah diprioritaskan.
	Amount *float64 `json:"amount,omitempty"`
}

// BizSection = judul dan penjelasan satu bagian halaman. Lead memuat angka
// periode ini; Hint menerangkan cara membaca gambarnya.
type BizSection struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Lead  string `json:"lead"`
	Hint  string `json:"hint"`
}

// BizTerm = istilah beserta contoh yang diambil dari data periode ini, supaya
// pembaca mencocokkannya dengan angka yang sedang ia lihat.
type BizTerm struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
	Example string `json:"example"`
}

// Nilai Verdict pada BusinessAnalysis.
// Pasar-turun dan outlet-tertinggal adalah dua uji yang saling bebas, jadi
// keduanya bisa benar bersamaan — BizVerdictBoth. Memaksa memilih salah satu
// membuat penurunan pasar yang dibarengi satu outlet bermasalah selalu terbaca
// sebagai "salah outlet" saja, dan sisi pasar tidak pernah ikut diperiksa.
const (
	BizVerdictOutlet = "OUTLET"           // outlet tertinggal, pasarnya sendiri tidak turun
	BizVerdictMarket = "PASAR_MARKOM"     // pasar turun (setelah kalender), tak ada outlet yang menyimpang
	BizVerdictBoth   = "OUTLET_DAN_PASAR" // pasar turun DAN ada outlet yang tertinggal
	BizVerdictNormal = "NORMAL"
	BizVerdictNoData = "DATA_KURANG"
)

type BusinessAnalysis struct {
	WeeksRequested int `json:"weeks_requested"` // minggu yang diminta pengguna
	WeeksCount     int `json:"weeks_count"`     // minggu penuh yang DITAMPILKAN
	// WindowWeeks = minggu penuh yang dipakai MENGHITUNG (blok, batas wajar,
	// pengali kalender): 26 minggu terakhir yang tersedia, berapa pun rentang
	// yang ditampilkan. Rentang hanya mengubah panjang gambar, bukan vonis.
	WindowWeeks int `json:"window_weeks"`
	BlockWeeks  int `json:"block_weeks"` // panjang blok pembanding (4; 3 bila riwayat pendek)
	PeriodFrom     string `json:"period_from"`
	PeriodTo       string `json:"period_to"`

	Group           []BizGroupWeek `json:"group"`
	GroupGrowth4    *float64       `json:"group_growth4"`     // gerak pasar setara kalender, ditimbang omzet
	GroupGrowthRaw4 *float64       `json:"group_growth_raw4"` // gerak pasar mentah pada blok yang sama
	// CalendarEffect = GroupGrowthRaw4 − GroupGrowth4: berapa poin dari gerak
	// pasar mentah yang dijelaskan libur dan pola hari, bukan oleh pasar.
	CalendarEffect *float64 `json:"calendar_effect"`
	// CalendarReason = ringkasan libur di kedua blok untuk kalimat vonis:
	// "blok pembanding HUT RI, Maulid; blok terakhir tanpa libur".
	CalendarReason string   `json:"calendar_reason"`
	PanelCodes     []string `json:"panel_codes"`

	// PanelShare = berapa persen omzet grup yang diwakili outlet panel pada
	// periode ini. GroupGrowth4 hanya menghitung outlet panel (same-store), jadi
	// angka itu tidak boleh disebut "semua outlet" tanpa menyebutkan cakupannya.
	PanelShare *float64 `json:"panel_share"`

	// MarketBand = batas naik-turun wajar PASAR, diturunkan dari sebaran gerak
	// pasar itu sendiri (setara kalender). Pasar disebut turun hanya bila
	// melewati batas ini.
	MarketBand *float64 `json:"market_band"`

	Outlets []BizOutlet `json:"outlets"`

	Verdict     string `json:"verdict"`
	VerdictText string `json:"verdict_text"`

	// Headline = inti halaman dalam satu baris; penjelasan panjangnya sudah ada
	// di VerdictText, jadi tidak ada lapis ketiga yang mengulang keduanya.
	// Insights = temuan terurut kepentingan. Sections = judul + penjelasan tiap
	// bagian. Glossary = istilah dengan contoh dari periode ini.
	Headline string       `json:"headline"`
	Insights []BizInsight `json:"insights"`
	Sections []BizSection `json:"sections"`
	Glossary []BizTerm    `json:"glossary"`

	// Calendar = hari bertanda yang jatuh di dalam periode; CalendarModel =
	// pengali yang dipakai mengoreksinya. Keduanya ditampilkan karena koreksi
	// kalender ikut menentukan vonis, dan yang ikut menentukan vonis harus bisa
	// diperiksa (dan diperbaiki) pembacanya.
	Calendar      []BizCalendarDay  `json:"calendar"`
	CalendarModel *BizCalendarModel `json:"calendar_model,omitempty"`

	// Narrative = asal teks halaman: "ai" (ditulis ulang Claude dari angka
	// periode ini), "generating" (sedang disusun di latar; UI memuat ulang),
	// "template" (kalimat templat karena kunci API belum diisi), "error".
	Narrative *BizNarrativeStatus `json:"narrative,omitempty"`

	// Model = hasil layanan analitik Python: tren robust, perkiraan 4 minggu,
	// minggu aneh, pergeseran level, dan pergeseran pola hari — semuanya pada
	// angka setara kalender. nil bila layanannya tidak dikonfigurasi.
	Model *BizModel `json:"model,omitempty"`

	// Social = blok kinerja medsos (IG/TikTok) yang disandingkan dengan gerak
	// penjualan. nil bila belum ada akun yang didaftarkan — halaman menyembunyikan
	// bagiannya alih-alih menggambar grafik kosong.
	Social *BizSocial `json:"social,omitempty"`

	Lagging     []string `json:"lagging"`      // kode outlet tertinggal
	Leading     []string `json:"leading"`      // kode outlet unggul
	Watch       []string `json:"watch"`        // kode outlet yang perlu dipantau
	GroupEvents []string `json:"group_events"` // label minggu peristiwa grup
	Notes       []string `json:"notes"`
}

// BizNarrativeStatus = keterangan asal teks halaman untuk UI.
type BizNarrativeStatus struct {
	Status      string `json:"status"`
	Model       string `json:"model,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
	Note        string `json:"note,omitempty"`
}
