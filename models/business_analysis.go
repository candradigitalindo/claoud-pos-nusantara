package models

// ── Analisa Bisnis (RGI / anti-alibi) ────────────────────────────────────────
// Memisahkan pergerakan yang dialami SELURUH grup (pasar & program Markom) dari
// pergerakan yang khas satu outlet (eksekusi manajer outlet).
//
// Inti metriknya RGI (Relative Growth Index) = growth outlet − growth outlet
// LAIN, bersatuan poin persen. Growth mentah per outlet terlalu berderau untuk
// dipakai menilai orang: guncangan akhir pekan besar/libur menggerakkan semua
// outlet bersamaan. Mengurangkan growth sejawat membuang guncangan bersama itu,
// menyisakan bagian yang benar-benar khas outlet tersebut.
//
// Pembandingnya sengaja TIDAK memuat outlet yang sedang dinilai, dan memakai
// nilai tengah (tiap outlet satu suara) alih-alih rata-rata tertimbang omzet.
// Keduanya membuat angka RGI tidak bergantung pada jumlah outlet maupun besar
// kecilnya omzet masing-masing.

// BizWeek = satu minggu untuk satu outlet.
type BizWeek struct {
	WeekStart string   `json:"week_start"` // Senin, tanggal lokal (zona waktu aplikasi)
	Label     string   `json:"label"`      // 2026-W35
	Net       float64  `json:"net"`
	Trx       int      `json:"trx"`
	// Index = perjalanan penjualan, disandingkan pada level pasar saat outlet
	// ini mulai ikut terhitung. nil selama outlet belum punya garis pasar
	// sebagai titik sandar — digambar sebagai putus, bukan sebagai nol.
	Index *float64 `json:"index"`
	Growth    *float64 `json:"growth"` // % terhadap minggu sebelumnya; nil bila tak ada pembanding
	RGI       *float64 `json:"rgi"`    // growth − median growth outlet lain, poin persen
}

// BizGroupWeek = satu minggu untuk grup pembanding (panel same-store).
type BizGroupWeek struct {
	WeekStart string  `json:"week_start"`
	Label     string  `json:"label"`
	Net       float64 `json:"net"`
	// OutletCount = berapa outlet berjualan pada minggu itu. Dipakai untuk
	// menjelaskan kenapa total penjualan bisa melompat tanpa growth ikut naik.
	OutletCount int      `json:"outlet_count"`
	Index       *float64 `json:"index"` // nil sebelum panel cukup untuk dirantai
	Growth      *float64 `json:"growth"`
	// IsGroupEvent: pertumbuhan grup menyimpang jauh dari kebiasaannya sendiri
	// — libur panjang, cuaca, acara besar. Minggu seperti ini peristiwa pasar,
	// bukan hasil kerja manajer.
	//
	// Penilaian antar-outlet (RGI) TIDAK perlu ditahan pada minggu ini: RGI
	// mengurangkan pertumbuhan sejawat, jadi guncangan yang dialami bersama
	// hilang dengan sendirinya. Yang ikut terbawa adalah angka PASAR, karena ia
	// menjumlah semua outlet — karena itu minggu semacam ini disebutkan di
	// catatan kaki ketika jatuh di dalam blok yang sedang dibandingkan.
	IsGroupEvent bool `json:"is_group_event"`
}

// Nilai Diagnosis pada BizOutlet.
const (
	BizDiagLeading = "UNGGUL"       // tumbuh meyakinkan di atas grup
	BizDiagLagging = "TERTINGGAL"   // tertinggal meyakinkan dari grup → isu eksekusi outlet
	BizDiagOnPace  = "SEIRAMA"      // tidak berbeda dari grup melebihi deraunya sendiri
	BizDiagWeak    = "SINYAL_LEMAH" // basis transaksi terlalu kecil/berayun untuk disimpulkan
	BizDiagNoData  = "DATA_KURANG"  // riwayat belum cukup untuk menyimpulkan
)

// Nilai Owner — siapa yang memegang masalahnya.
//
// BizOwnerMarkom dipakai untuk outlet yang eksekusinya seirama dengan
// sejawatnya tetapi pasarnya sendiri sedang turun: tidak ada yang perlu
// dibenahi di outlet itu, yang perlu digerakkan adalah programnya.
const (
	BizOwnerOutlet = "Manajer Outlet"
	BizOwnerMarkom = "Markom & Pasar"
	BizOwnerNone   = "—"
)

type BizOutlet struct {
	OutletID string    `json:"outlet_id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Net      float64   `json:"net"` // total sepanjang periode
	Trx      int       `json:"trx"`
	ATV      float64   `json:"atv"`
	Weeks    []BizWeek `json:"weeks"`

	Growth4 *float64 `json:"growth4"` // growth blok akhir vs blok sebelumnya, %

	// PeerGrowth4 = nilai tengah growth blok outlet LAIN — patokan yang dipakai
	// untuk menilai outlet ini. PeerCount = berapa outlet menyusunnya.
	// Keduanya ditampilkan supaya perbandingannya bisa ditelusuri manajer.
	PeerGrowth4 *float64 `json:"peer_growth4"`
	PeerCount   int      `json:"peer_count"`

	RGI4 *float64 `json:"rgi4"` // growth4 − peer_growth4, poin persen

	// Threshold = ambang keyakinan adaptif, 1,5 × galat baku RGI blok.
	// Dipakai sebagai ambang diagnosis: outlet berderau tinggi butuh selisih
	// lebih besar sebelum boleh disebut tertinggal atau unggul.
	Threshold *float64 `json:"threshold"`

	// ── Bahan laporan detail per outlet ─────────────────────────────────────
	// Manajer outlet membaca rupiah dan jumlah struk, bukan poin persen. Angka
	// di bawah ini menerjemahkan RGI ke satuan yang bisa langsung ditindak.

	RecentNet float64 `json:"recent_net"` // penjualan blok terakhir
	PrevNet   float64 `json:"prev_net"`   // penjualan blok sebelumnya
	RecentTrx int     `json:"recent_trx"`
	PrevTrx   int     `json:"prev_trx"`

	// ExpectedNet = penjualan blok terakhir SEANDAINYA outlet ini bergerak
	// seperti outlet lain. GapNet = kenyataan − seharusnya, dalam rupiah.
	// Inilah bentuk RGI yang bisa dipahami tanpa menjelaskan poin persen.
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
	Owner     string `json:"owner"`
	Note      string `json:"note"`
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
// sebagai "salah outlet" saja, dan Markom tidak pernah ikut ditagih.
const (
	BizVerdictOutlet = "OUTLET"           // outlet tertinggal, pasarnya sendiri tidak turun
	BizVerdictMarket = "PASAR_MARKOM"     // pasar turun, tak ada outlet yang menyimpang
	BizVerdictBoth   = "OUTLET_DAN_PASAR" // pasar turun DAN ada outlet yang tertinggal
	BizVerdictNormal = "NORMAL"
	BizVerdictNoData = "DATA_KURANG"
)

type BusinessAnalysis struct {
	WeeksRequested int    `json:"weeks_requested"` // minggu yang diminta pengguna
	WeeksCount     int    `json:"weeks_count"`     // minggu penuh yang benar-benar ada
	BlockWeeks     int    `json:"block_weeks"`     // panjang blok pembanding (biasanya 4)
	PeriodFrom     string `json:"period_from"`
	PeriodTo       string `json:"period_to"`

	Group        []BizGroupWeek `json:"group"`
	GroupGrowth4 *float64       `json:"group_growth4"` // gerak pasar, ditimbang omzet
	PanelCodes   []string       `json:"panel_codes"`

	// PanelShare = berapa persen omzet grup yang diwakili outlet panel pada
	// periode ini. GroupGrowth4 hanya menghitung outlet panel (same-store), jadi
	// angka itu tidak boleh disebut "semua outlet" tanpa menyebutkan cakupannya
	// — pembaca memakainya untuk memutuskan anggaran Markom.
	PanelShare *float64 `json:"panel_share"`

	// MarketBand = batas naik-turun wajar PASAR, diturunkan dari sebaran gerak
	// pasar itu sendiri. Pasar disebut turun hanya bila melewati batas ini —
	// ambangnya ikut bisnis, bukan angka mati, dan tidak melar hanya karena
	// jumlah outlet bertambah.
	MarketBand *float64 `json:"market_band"`

	Outlets []BizOutlet `json:"outlets"`

	Verdict     string   `json:"verdict"`
	VerdictText string   `json:"verdict_text"`

	// Headline = inti halaman dalam satu baris; penjelasan panjangnya sudah ada
	// di VerdictText, jadi tidak ada lapis ketiga yang mengulang keduanya.
	// Insights = temuan terurut kepentingan. Sections = judul + penjelasan tiap
	// bagian. Glossary = istilah dengan contoh dari periode ini.
	Headline string       `json:"headline"`
	Insights []BizInsight `json:"insights"`
	Sections []BizSection `json:"sections"`
	Glossary []BizTerm    `json:"glossary"`
	// Social = blok kinerja medsos (IG/TikTok) yang disandingkan dengan gerak
	// penjualan. nil bila belum ada akun yang didaftarkan — halaman menyembunyikan
	// bagiannya alih-alih menggambar grafik kosong.
	Social *BizSocial `json:"social,omitempty"`

	Lagging     []string `json:"lagging"`      // kode outlet tertinggal
	Leading     []string `json:"leading"`      // kode outlet unggul
	GroupEvents []string `json:"group_events"` // label minggu peristiwa grup
	Notes       []string `json:"notes"`
}
