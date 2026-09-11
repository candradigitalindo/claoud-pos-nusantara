package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── Kinerja Markom: pendaftaran akun, penarikan, dan ringkasan mingguan ──────
//
// Angka mingguan TIDAK disimpan sebagai tabel tersendiri. Ia dihitung ulang
// dari potret harian + daftar konten setiap kali diminta. Alasannya: konten
// lama terus bertambah tontonannya, jadi "jangkauan minggu ke-32" bukan angka
// yang membeku pada akhir minggu ke-32. Menyimpannya sekali akan membekukan
// pembacaan pertama dan membuat dua laporan dengan rentang berbeda menyebut
// angka berbeda untuk minggu yang sama.

var reSocialHandle = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// Pesan galat halaman ini dibaca staf Markom, jadi nama harinya ikut bahasa
// halamannya — "jatuh pada hari Thursday" memaksa pembaca menerjemahkan sendiri
// satu-satunya kata yang menjelaskan kenapa isiannya ditolak.
var namaHari = map[time.Weekday]string{
	time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa",
	time.Wednesday: "Rabu", time.Thursday: "Kamis", time.Friday: "Jumat",
	time.Saturday: "Sabtu",
}

// parseSocialHandle menerima apa pun yang biasa ditempel orang — URL profil
// lengkap, "@nama", atau nama polos — dan mengembalikan username bersih beserta
// URL kanoniknya.
//
// Menerima URL penuh bukan kemanjaan: yang dipegang Markom memang tautan, dan
// menuntut mereka memangkasnya sendiri jadi sumber salah ketik yang diam —
// akun dengan "?igsh=..." menempel akan gagal ditarik selamanya tanpa sebab
// yang terlihat di layar.
func parseSocialHandle(platform, raw string) (username, profileURL string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("username atau tautan profil wajib diisi")
	}

	if strings.Contains(s, "/") || strings.Contains(s, ".com") {
		if !strings.HasPrefix(s, "http") {
			s = "https://" + s
		}
		u, perr := url.Parse(s)
		if perr != nil {
			return "", "", fmt.Errorf("tautan profil tidak bisa dibaca")
		}
		// Segmen pertama path adalah nama akun pada kedua platform
		// (instagram.com/<nama>, tiktok.com/@<nama>).
		seg := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(seg) == 0 || seg[0] == "" {
			return "", "", fmt.Errorf("tautan profil tidak memuat nama akun")
		}
		s = seg[0]
	}

	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	if !reSocialHandle.MatchString(s) {
		return "", "", fmt.Errorf("nama akun \"%s\" tidak sah — hanya huruf, angka, titik, garis bawah", s)
	}

	switch platform {
	case models.SocialInstagram:
		return s, "https://www.instagram.com/" + s + "/", nil
	case models.SocialTiktok:
		return s, "https://www.tiktok.com/@" + s, nil
	}
	return "", "", fmt.Errorf("platform harus instagram atau tiktok")
}

// ── CRUD akun ────────────────────────────────────────────────────────────────

const socialAccountSelect = `
	SELECT a.id, a.outlet_id, TRIM(o.code), o.name, a.platform, a.username, a.profile_url,
	       a.is_active, a.auto_fetch, a.last_scraped_at, a.last_ok_at, a.last_error,
	       a.created_at, a.updated_at,
	       s.followers, s.posts_count, s.captured_date
	FROM social_accounts a
	JOIN outlets o ON o.id = a.outlet_id
	LEFT JOIN LATERAL (
		SELECT followers, posts_count, captured_date
		FROM social_snapshots
		WHERE account_id = a.id AND followers IS NOT NULL
		ORDER BY captured_date DESC LIMIT 1
	) s ON true`

func scanSocialAccounts(rows *sql.Rows) ([]models.SocialAccount, error) {
	defer rows.Close()
	out := []models.SocialAccount{}
	for rows.Next() {
		var a models.SocialAccount
		var captured sql.NullTime
		if err := rows.Scan(&a.ID, &a.OutletID, &a.OutletCode, &a.OutletName, &a.Platform,
			&a.Username, &a.ProfileURL, &a.IsActive, &a.AutoFetch, &a.LastScrapedAt, &a.LastOkAt,
			&a.LastError, &a.CreatedAt, &a.UpdatedAt, &a.Followers, &a.PostsCount, &captured); err != nil {
			return nil, err
		}
		if captured.Valid {
			a.LastCapture = captured.Time.Format("2006-01-02")
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func ListSocialAccounts() ([]models.SocialAccount, error) {
	rows, err := database.DB.Query(socialAccountSelect + ` ORDER BY TRIM(o.code), a.platform`)
	if err != nil {
		return nil, err
	}
	return scanSocialAccounts(rows)
}

func GetSocialAccount(id string) (*models.SocialAccount, error) {
	rows, err := database.DB.Query(socialAccountSelect+` WHERE a.id = $1`, id)
	if err != nil {
		return nil, err
	}
	list, err := scanSocialAccounts(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("akun medsos tidak ditemukan")
	}
	return &list[0], nil
}

func CreateSocialAccount(req models.SocialAccountRequest) (*models.SocialAccount, error) {
	if strings.TrimSpace(req.OutletID) == "" {
		return nil, fmt.Errorf("outlet wajib dipilih")
	}
	username, profileURL, err := parseSocialHandle(req.Platform, req.Username)
	if err != nil {
		return nil, err
	}

	// Kedua platform dijadwalkan menarik. Saklarnya tetap ada karena jalan yang
	// dipakai bukan jalan resmi: ia bisa tertutup sewaktu-waktu, dan ketika itu
	// terjadi akun yang bersangkutan dimatikan penarikannya supaya kegagalan
	// hariannya tidak menandai outletnya mandek dan mengeluarkannya dari grafik.
	autoFetch := true
	if req.AutoFetch != nil {
		autoFetch = *req.AutoFetch
	}

	id := NewULID()
	_, err = database.DB.Exec(`
		INSERT INTO social_accounts (id, outlet_id, platform, username, profile_url, is_active, auto_fetch)
		VALUES ($1, $2, $3, $4, $5, true, $6)`,
		id, req.OutletID, req.Platform, username, profileURL, autoFetch)
	if err != nil {
		if strings.Contains(err.Error(), "uq_social_accounts") {
			return nil, fmt.Errorf("akun %s @%s sudah terdaftar untuk outlet ini", req.Platform, username)
		}
		return nil, err
	}
	return GetSocialAccount(id)
}

func UpdateSocialAccount(id string, req models.SocialAccountRequest) (*models.SocialAccount, error) {
	cur, err := GetSocialAccount(id)
	if err != nil {
		return nil, err
	}

	username, profileURL := cur.Username, cur.ProfileURL
	if strings.TrimSpace(req.Username) != "" {
		platform := req.Platform
		if platform == "" {
			platform = cur.Platform
		}
		if username, profileURL, err = parseSocialHandle(platform, req.Username); err != nil {
			return nil, err
		}
	}
	active := cur.IsActive
	if req.IsActive != nil {
		active = *req.IsActive
	}
	autoFetch := cur.AutoFetch
	if req.AutoFetch != nil {
		autoFetch = *req.AutoFetch
	}

	// Ganti nama akun = ganti akun. Riwayat akun lama dibuang supaya grafik
	// tidak menyambung dua audiens berbeda jadi satu garis yang seolah melonjak.
	if !strings.EqualFold(username, cur.Username) {
		if _, err := database.DB.Exec(`DELETE FROM social_snapshots WHERE account_id = $1`, id); err != nil {
			return nil, err
		}
		if _, err := database.DB.Exec(`DELETE FROM social_posts WHERE account_id = $1`, id); err != nil {
			return nil, err
		}
	}

	_, err = database.DB.Exec(`
		UPDATE social_accounts
		SET username = $2, profile_url = $3, is_active = $4, auto_fetch = $5, last_error = '',
		    updated_at = (now() AT TIME ZONE 'UTC')
		WHERE id = $1`, id, username, profileURL, active, autoFetch)
	if err != nil {
		return nil, err
	}
	return GetSocialAccount(id)
}

func DeleteSocialAccount(id string) error {
	res, err := database.DB.Exec(`DELETE FROM social_accounts WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("akun medsos tidak ditemukan")
	}
	return nil
}

// ── Tambalan manual ──────────────────────────────────────────────────────────

// UpsertSocialManualWeek menyimpan angka yang diketik orang untuk satu akun
// pada satu minggu. Kolom yang dikirim kosong (nil) DIHAPUS dari tambalan,
// bukan disimpan sebagai nol — supaya orang bisa membatalkan satu isian tanpa
// memaksa angka scrape ikut terkubur di bawah nol.
func UpsertSocialManualWeek(m models.SocialManualWeek, actor string) error {
	week, err := time.Parse("2006-01-02", strings.TrimSpace(m.WeekStart))
	if err != nil {
		return fmt.Errorf("minggu tidak sah, format harus YYYY-MM-DD")
	}
	if week.Weekday() != time.Monday {
		return fmt.Errorf("minggu harus dimulai hari Senin (%s jatuh pada hari %s)",
			m.WeekStart, namaHari[week.Weekday()])
	}
	if _, err := GetSocialAccount(m.AccountID); err != nil {
		return err
	}

	// Semua kolom kosong = tambalan dicabut seluruhnya.
	if m.Followers == nil && m.Posts == nil && m.Views == nil && m.Engagement == nil &&
		strings.TrimSpace(m.Note) == "" {
		_, err := database.DB.Exec(
			`DELETE FROM social_manual_weeks WHERE account_id = $1 AND week_start = $2`,
			m.AccountID, m.WeekStart)
		return err
	}

	_, err = database.DB.Exec(`
		INSERT INTO social_manual_weeks
			(account_id, week_start, followers, posts, views, engagement, note, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (now() AT TIME ZONE 'UTC'))
		ON CONFLICT (account_id, week_start) DO UPDATE SET
			followers = EXCLUDED.followers, posts = EXCLUDED.posts,
			views = EXCLUDED.views, engagement = EXCLUDED.engagement,
			note = EXCLUDED.note, updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at`,
		m.AccountID, m.WeekStart, m.Followers, m.Posts, m.Views, m.Engagement,
		strings.TrimSpace(m.Note), actor)
	return err
}

func ListSocialManualWeeks(accountID string) ([]models.SocialManualWeek, error) {
	rows, err := database.DB.Query(`
		SELECT account_id, week_start::text, followers, posts, views, engagement,
		       note, updated_by, to_char(updated_at, 'YYYY-MM-DD HH24:MI')
		FROM social_manual_weeks
		WHERE account_id = $1
		ORDER BY week_start DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.SocialManualWeek{}
	for rows.Next() {
		var m models.SocialManualWeek
		if err := rows.Scan(&m.AccountID, &m.WeekStart, &m.Followers, &m.Posts, &m.Views,
			&m.Engagement, &m.Note, &m.UpdatedBy, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── Penarikan ────────────────────────────────────────────────────────────────

// ── Rem: menjaga alamat IP ini tetap boleh membaca minggu depan ─────────────
//
// Modul ini menarik belasan halaman sehari — jumlah yang sepele. Yang membuat
// sebuah alamat diblokir hampir tidak pernah jumlahnya, melainkan POLANYA:
// permintaan yang datang beruntun pada detik yang sama tiap hari, dan
// permintaan yang terus diulang justru setelah ditolak. Tiga rem di bawah ini
// menangani keduanya.
var (
	// satuPutaran memastikan hanya ada satu putaran penarikan pada satu waktu.
	// Tanpa ini, tombol "Tarik sekarang" yang diklik dua kali — atau diklik
	// bersamaan dengan putaran terjadwal — melipatgandakan lalu lintas keluar
	// pada saat yang justru paling tidak tepat.
	satuPutaran sync.Mutex
	sedangJalan atomic.Bool

	// istirahatSampai menahan akun yang BARU SAJA ditolak platform. Kuncinya
	// per akun, bukan global: satu akun bermasalah tidak perlu menghentikan
	// pembacaan akun lain yang sehat.
	istirahatMu   sync.Mutex
	istirahatAkun = map[string]time.Time{}
)

// socialBolehDitarik melaporkan apakah akun ini sedang tidak dalam masa
// istirahat, beserta sisa waktunya bila sedang.
func socialBolehDitarik(id string) (time.Duration, bool) {
	istirahatMu.Lock()
	defer istirahatMu.Unlock()
	if sampai, ada := istirahatAkun[id]; ada {
		if sisa := time.Until(sampai); sisa > 0 {
			return sisa, false
		}
		delete(istirahatAkun, id)
	}
	return 0, true
}

func socialIstirahatkan(id string, lama time.Duration) {
	istirahatMu.Lock()
	defer istirahatMu.Unlock()
	istirahatAkun[id] = time.Now().Add(lama)
}

// ScrapeSocialAccounts menarik angka untuk akun tertentu, atau seluruh akun
// terjadwal bila ids kosong.
//
// Berjalan berurutan dengan jeda, tidak paralel. Sepuluh permintaan serentak
// memang selesai sepuluh kali lebih cepat, dan itu persis pola yang dikenali
// kedua platform sebagai robot. Yang dipertaruhkan bukan kecepatan satu
// putaran, melainkan apakah alamat IP ini masih bisa membaca minggu depan.
// dariJadwal memisahkan dua peran yang tuntutannya berlawanan:
//
//	true  → penjadwal dini hari. Tidak ada yang menunggu, jadi jedanya panjang
//	        dan daftarnya tidak dibatasi. Inilah yang mengumpulkan angka.
//	false → orang menekan tombol. Ia sedang menatap layar, jadi jedanya pendek
//	        dan daftarnya dipotong — tombol itu untuk MEMASTIKAN tautannya benar,
//	        bukan untuk mengumpulkan. Sisanya diserahkan ke penjadwal.
const socialBatasManual = 5

func ScrapeSocialAccounts(ids []string, dariJadwal bool) (*models.SocialScrapeResult, error) {
	if sedangJalan.Load() {
		return nil, fmt.Errorf("penarikan lain sedang berjalan — tunggu sampai selesai sebelum menarik lagi")
	}
	satuPutaran.Lock()
	sedangJalan.Store(true)
	defer func() {
		sedangJalan.Store(false)
		satuPutaran.Unlock()
	}()

	mulai := time.Now()
	seluruhnya := len(ids) == 0

	q := `SELECT a.id, a.platform, a.username, TRIM(o.code)
	      FROM social_accounts a JOIN outlets o ON o.id = a.outlet_id
	      WHERE a.is_active = true AND o.is_active = true`
	args := []any{}
	if !seluruhnya {
		// Penarikan yang diminta per akun TIDAK menyaring auto_fetch: inilah cara
		// orang menguji apakah jalan keluarnya (proxy baru) sudah terbuka untuk
		// akun yang selama ini diisi tangan.
		q += ` AND a.id = ANY($1)`
		args = append(args, pgTextArray(ids))
	} else {
		// Putaran terjadwal melewati akun yang potret hari ini sudah ada.
		// Tanpa saringan ini, satu restart di sore hari membuat seluruh daftar
		// ditembak ulang padahal angkanya sudah lengkap sejak subuh — lalu
		// lintas yang tidak menambah satu pun angka baru.
		q += ` AND a.auto_fetch = true
		       AND NOT EXISTS (
		         SELECT 1 FROM social_snapshots s
		         WHERE s.account_id = a.id AND s.captured_date = tz_today()
		       )`
	}
	// Urutan diacak tiap putaran. Daftar yang selalu dibaca dengan urutan sama
	// pada jam yang sama adalah tanda tangan yang rapi sekali untuk dikenali.
	q += ` ORDER BY random()`

	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	type target struct{ id, platform, username, code string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.platform, &t.username, &t.code); err != nil {
			rows.Close()
			return nil, err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Putaran yang diminta orang dipotong supaya selesai selagi ia menunggu.
	// Sisanya bukan hilang — penjadwal dini hari akan menariknya, dan itu
	// disebutkan terang-terangan supaya tidak terbaca sebagai kegagalan.
	tersisa := 0
	if !dariJadwal && seluruhnya && len(targets) > socialBatasManual {
		tersisa = len(targets) - socialBatasManual
		targets = targets[:socialBatasManual]
	}

	res := &models.SocialScrapeResult{Attempted: len(targets), Errors: []string{}}
	today := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")

	// beruntun menghitung penolakan BERTURUT-TURUT. Ketika platform mulai
	// menolak, meneruskan sisa daftar tidak akan menghasilkan satu angka pun —
	// yang bertambah hanya catatan buruk pada alamat IP ini. Jadi putarannya
	// dihentikan dan sisanya ditinggalkan untuk besok.
	const batasBeruntun = 3
	beruntun := 0
	ditarik := 0

	for _, t := range targets {
		if sisa, boleh := socialBolehDitarik(t.id); !boleh {
			res.Errors = append(res.Errors, fmt.Sprintf(
				"%s · %s @%s: dilewati, sedang diistirahatkan %s lagi setelah ditolak platform",
				t.code, t.platform, t.username, sisa.Round(time.Minute)))
			continue
		}
		if ditarik > 0 {
			socialJeda(dariJadwal)
		}
		ditarik++

		prof, err := scrapeProfile(t.platform, t.username)
		if err != nil {
			res.Failed++
			pesan := fmt.Sprintf("%s · %s @%s: %v", t.code, t.platform, t.username, err)
			res.Errors = append(res.Errors, pesan)
			database.DB.Exec(`
				UPDATE social_accounts
				SET last_scraped_at = (now() AT TIME ZONE 'UTC'), last_error = $2,
				    updated_at = (now() AT TIME ZONE 'UTC')
				WHERE id = $1`, t.id, err.Error())
			log.Printf("[Medsos] gagal %s", pesan)

			if lama, ditolak := socialIstirahat(err); ditolak {
				socialIstirahatkan(t.id, lama)
				beruntun++
				if beruntun >= batasBeruntun {
					res.Errors = append(res.Errors, fmt.Sprintf(
						"Putaran dihentikan: %d akun berturut-turut ditolak platform. Sisanya tidak dicoba supaya alamat ini tidak makin dibatasi — akan dicoba lagi pada jadwal berikutnya.",
						beruntun))
					log.Printf("[Medsos] putaran dihentikan setelah %d penolakan beruntun", beruntun)
					break
				}
			}
			continue
		}
		beruntun = 0

		if err := storeSocialProfile(t.id, today, prof); err != nil {
			res.Failed++
			res.Errors = append(res.Errors,
				fmt.Sprintf("%s · %s @%s: gagal menyimpan — %v", t.code, t.platform, t.username, err))
			continue
		}
		res.OK++
	}

	if tersisa > 0 {
		res.Errors = append(res.Errors, fmt.Sprintf(
			"%d akun lain belum ditarik pada putaran ini supaya tidak terlalu lama menunggu — penjadwal dini hari akan mengambilnya, atau tekan tombolnya lagi.",
			tersisa))
	}

	res.TookMs = time.Since(mulai).Milliseconds()
	return res, nil
}

// storeSocialProfile menyimpan satu hasil baca: potret harian + daftar konten.
func storeSocialProfile(accountID, today string, p *socialProfile) error {
	_, err := database.DB.Exec(`
		INSERT INTO social_snapshots
			(id, account_id, captured_date, followers, following, posts_count, likes_total, source, approx)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'scrape', $8)
		ON CONFLICT (account_id, captured_date) DO UPDATE SET
			followers = EXCLUDED.followers, following = EXCLUDED.following,
			posts_count = EXCLUDED.posts_count, likes_total = EXCLUDED.likes_total,
			approx = EXCLUDED.approx, source = 'scrape',
			captured_at = (now() AT TIME ZONE 'UTC')`,
		NewULID(), accountID, today, p.Followers, p.Following, p.PostsCount, p.LikesTotal, p.Approx)
	if err != nil {
		return err
	}

	for _, post := range p.Posts {
		// Angka nol pada pembacaan ulang TIDAK menimpa angka yang sudah ada:
		// nol di sini hampir selalu berarti "kolomnya tidak ikut terkirim",
		// bukan "tidak ada yang menyukai". Penurunan yang sungguhan (orang
		// membatalkan suka) tetap tersimpan karena hanya nol yang ditolak.
		_, err := database.DB.Exec(`
			INSERT INTO social_posts
				(id, account_id, post_ref, posted_at, permalink, caption, views, likes, comments, shares)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (account_id, post_ref) DO UPDATE SET
				views    = CASE WHEN EXCLUDED.views    > 0 THEN EXCLUDED.views    ELSE social_posts.views    END,
				likes    = CASE WHEN EXCLUDED.likes    > 0 THEN EXCLUDED.likes    ELSE social_posts.likes    END,
				comments = CASE WHEN EXCLUDED.comments > 0 THEN EXCLUDED.comments ELSE social_posts.comments END,
				shares   = CASE WHEN EXCLUDED.shares   > 0 THEN EXCLUDED.shares   ELSE social_posts.shares   END,
				caption  = CASE WHEN EXCLUDED.caption <> '' THEN EXCLUDED.caption ELSE social_posts.caption  END,
				fetched_at = (now() AT TIME ZONE 'UTC')`,
			NewULID(), accountID, post.Ref, post.PostedAt, post.Permalink, post.Caption,
			post.Views, post.Likes, post.Comments, post.Shares)
		if err != nil {
			return err
		}
	}

	_, err = database.DB.Exec(`
		UPDATE social_accounts
		SET last_scraped_at = (now() AT TIME ZONE 'UTC'),
		    last_ok_at = (now() AT TIME ZONE 'UTC'),
		    last_error = '', updated_at = (now() AT TIME ZONE 'UTC')
		WHERE id = $1`, accountID)
	return err
}

// pgTextArray membungkus []string jadi literal array Postgres. Dipakai agar
// penarikan terpilih tetap satu query, tanpa merakit IN (...) dari string.
func pgTextArray(v []string) string {
	esc := make([]string, 0, len(v))
	for _, s := range v {
		esc = append(esc, `"`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)+`"`)
	}
	return "{" + strings.Join(esc, ",") + "}"
}

// ── Ringkasan mingguan ───────────────────────────────────────────────────────

// socialWeekRow = satu akun pada satu minggu, sudah digabung dengan tambalan
// manualnya.
type socialWeekRow struct {
	OutletID, Code, Name string
	AccountID            string
	Platform, Username   string
	ProfileURL           string
	AutoFetch            bool
	LastOkAt             *time.Time
	LastError            string
	WeekStart            string
	Followers            *int64
	Approx               bool
	Posts, Views, Engage int64
	Manual               bool
}

// socialWeeklyRows menyusun angka mingguan per akun untuk N minggu penuh
// terakhir — minggu bisnis Senin–Minggu di zona waktu aplikasi, persis sama
// dengan yang dipakai penjualan di Analisa Bisnis. Kalau dua laporan memakai
// batas minggu yang berbeda, seluruh penyandingannya jadi omong kosong.
//
// Pengikut sengaja TIDAK dibawa maju dari minggu sebelumnya ketika minggu itu
// tidak punya pembacaan. Membawa maju akan melahirkan pertumbuhan nol yang
// terlihat seperti fakta, padahal artinya "tidak ada yang membaca minggu itu".
//
// ── Dua cara menurunkan angka MINGGUAN ──────────────────────────────────────
//
// Halaman profil publik TikTok tidak menyerahkan daftar videonya, hanya
// penghitung kumulatif akun: total video dan total suka sepanjang masa. Jadi
// ada dua jalur, dipilih per akun (bukan per minggu, supaya satu garis tidak
// berpindah-pindah sumber di tengah jalan):
//
//	punya rincian konten → jumlahkan konten yang TERBIT minggu itu;
//	                       ini yang memberi angka tayangan.
//	hanya kumulatif      → SELISIH penghitung antar minggu. Tambahan 3 video
//	                       dan 4.812 suka sepanjang minggu itu sama sahihnya
//	                       sebagai ukuran aliran mingguan, hanya tanpa tayangan.
//
// Selisih negatif dijadikan nol: konten yang dihapus membuat penghitungnya
// mundur, dan "minggu ini menerbitkan −2 konten" bukan kalimat yang berarti.
func socialWeeklyRows(weeks int) ([]socialWeekRow, error) {
	rows, err := database.DB.Query(`
		WITH b AS (SELECT date_trunc('week', tz_today()::timestamp)::date AS cur),
		wk AS (
			SELECT gs::date AS week_start
			FROM b, generate_series(b.cur - ($1::int * 7), b.cur - 7, interval '7 days') gs
		),
		acc AS (
			SELECT a.id, a.outlet_id, TRIM(o.code) AS code, o.name, a.platform, a.username,
			       a.profile_url, a.auto_fetch, a.last_ok_at, a.last_error,
			       EXISTS (SELECT 1 FROM social_posts sp WHERE sp.account_id = a.id) AS has_posts
			FROM social_accounts a
			JOIN outlets o ON o.id = a.outlet_id
			WHERE a.is_active = true AND o.is_active = true
		),
		grid AS (SELECT acc.id AS account_id, wk.week_start FROM acc CROSS JOIN wk),
		snap AS (
			SELECT g.account_id, g.week_start, s.followers, s.posts_count, s.likes_total,
			       COALESCE(ap.approx, false) AS approx
			FROM grid g
			LEFT JOIN LATERAL (
				SELECT followers, posts_count, likes_total
				FROM social_snapshots s
				WHERE s.account_id = g.account_id AND s.followers IS NOT NULL
				  AND s.captured_date BETWEEN g.week_start AND g.week_start + 6
				ORDER BY s.captured_date DESC LIMIT 1
			) s ON true
			LEFT JOIN LATERAL (
				SELECT bool_or(s2.approx) AS approx
				FROM social_snapshots s2
				WHERE s2.account_id = g.account_id
				  AND s2.captured_date BETWEEN g.week_start AND g.week_start + 6
			) ap ON true
		),
		pst AS (
			SELECT g.account_id, g.week_start,
			       COUNT(p.id) AS posts,
			       COALESCE(SUM(p.views), 0) AS views,
			       COALESCE(SUM(p.likes + p.comments + p.shares), 0) AS engagement
			FROM grid g
			LEFT JOIN social_posts p
			  ON p.account_id = g.account_id
			 AND tz_date(p.posted_at) BETWEEN g.week_start AND g.week_start + 6
			GROUP BY 1, 2
		)
		SELECT a.outlet_id, a.code, a.name, a.id, a.platform, a.username, a.profile_url,
		       a.auto_fetch, a.last_ok_at, a.last_error, a.has_posts, g.week_start::text,
		       s.followers, s.posts_count, s.likes_total, s.approx,
		       pst.posts, pst.views, pst.engagement,
		       m.followers, m.posts, m.views, m.engagement
		FROM grid g
		JOIN acc a ON a.id = g.account_id
		LEFT JOIN snap s ON s.account_id = g.account_id AND s.week_start = g.week_start
		LEFT JOIN pst ON pst.account_id = g.account_id AND pst.week_start = g.week_start
		LEFT JOIN social_manual_weeks m ON m.account_id = g.account_id AND m.week_start = g.week_start
		ORDER BY a.code, a.platform, a.id, g.week_start`, weeks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// mentah = satu baris apa adanya dari database, sebelum diputuskan jalur
	// mana yang dipakai. Penyatuannya dikerjakan di Go, bukan di SQL, karena
	// keputusannya bergantung pada baris TETANGGA (minggu sebelumnya) — dan
	// aturan "pakai selisih, kecuali ada tambalan manual" jauh lebih mudah
	// diperiksa orang kalau ditulis sebagai kalimat biasa.
	type mentah struct {
		row                     socialWeekRow
		hasPosts                bool
		postsCum, likesCum      *int64
		dPosts, dViews, dEngage int64
		mFollowers, mPosts      *int64
		mViews, mEngage         *int64
	}
	var all []mentah
	for rows.Next() {
		var x mentah
		var followers, postsCum, likesCum sql.NullInt64
		var approx sql.NullBool
		var dPosts, dViews, dEngage sql.NullInt64
		if err := rows.Scan(&x.row.OutletID, &x.row.Code, &x.row.Name, &x.row.AccountID,
			&x.row.Platform, &x.row.Username, &x.row.ProfileURL, &x.row.AutoFetch,
			&x.row.LastOkAt, &x.row.LastError, &x.hasPosts, &x.row.WeekStart,
			&followers, &postsCum, &likesCum, &approx,
			&dPosts, &dViews, &dEngage,
			&x.mFollowers, &x.mPosts, &x.mViews, &x.mEngage); err != nil {
			return nil, err
		}
		if followers.Valid {
			v := followers.Int64
			x.row.Followers = &v
		}
		if postsCum.Valid {
			v := postsCum.Int64
			x.postsCum = &v
		}
		if likesCum.Valid {
			v := likesCum.Int64
			x.likesCum = &v
		}
		x.row.Approx = approx.Bool
		x.dPosts, x.dViews, x.dEngage = dPosts.Int64, dViews.Int64, dEngage.Int64
		all = append(all, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]socialWeekRow, 0, len(all))
	for i, x := range all {
		r := x.row

		if x.hasPosts {
			r.Posts, r.Views, r.Engage = x.dPosts, x.dViews, x.dEngage
		} else if i > 0 && all[i-1].row.AccountID == x.row.AccountID {
			p := all[i-1]
			r.Posts = socialSelisihKumulatif(x.postsCum, p.postsCum)
			r.Engage = socialSelisihKumulatif(x.likesCum, p.likesCum)
		}

		// Tambalan manual menimpa per kolom, bukan per baris: outlet Instagram
		// biasanya hanya perlu menambal jangkauan, dan pengikutnya tetap dari
		// tarikan.
		if x.mFollowers != nil {
			r.Followers = x.mFollowers
			r.Manual = true
		}
		if x.mPosts != nil {
			r.Posts = *x.mPosts
			r.Manual = true
		}
		if x.mViews != nil {
			r.Views = *x.mViews
			r.Manual = true
		}
		if x.mEngage != nil {
			r.Engage = *x.mEngage
			r.Manual = true
		}
		out = append(out, r)
	}
	return out, nil
}

// socialSelisihKumulatif mengubah dua pembacaan penghitung kumulatif jadi
// pertambahan satu minggu.
//
// Dua keadaan dikembalikan sebagai nol, dan keduanya disengaja:
//
//   - salah satu ujungnya tidak terbaca — tanpa dua ujung tidak ada selisih
//     yang bisa dihitung, dan menebaknya dari minggu yang lebih jauh akan
//     menumpuk pertambahan beberapa minggu ke satu batang;
//   - penghitungnya MUNDUR — itu terjadi ketika konten dihapus, dan "minggu ini
//     menerbitkan −2 konten" bukan kalimat yang berarti bagi pembacanya.
//
// Baris sudah terurut per akun lalu per minggu menaik, jadi minggu sebelumnya
// untuk akun yang sama selalu tepat satu langkah ke belakang.
func socialSelisihKumulatif(cur, prev *int64) int64 {
	if cur == nil || prev == nil || *cur < *prev {
		return 0
	}
	return *cur - *prev
}

// SocialAccountCount menghitung akun aktif — dipakai Analisa Bisnis untuk
// memutuskan apakah bagian medsos digambar sama sekali.
func SocialAccountCount() int {
	var n int
	database.DB.QueryRow(`
		SELECT COUNT(*) FROM social_accounts a
		JOIN outlets o ON o.id = a.outlet_id
		WHERE a.is_active = true AND o.is_active = true`).Scan(&n)
	return n
}

// GetSocialWeekly menyajikan ringkasan mingguan apa adanya untuk halaman
// Kinerja Markom — termasuk minggu yang kosong. Minggu kosong sengaja ikut
// dikirim: justru barisan kosong itulah yang perlu dilihat orang yang bertugas
// menambal, dan menyembunyikannya membuat halaman terlihat lengkap padahal
// separuh riwayatnya tidak pernah terbaca.
func GetSocialWeekly(weeks int) ([]models.SocialWeeklyRow, error) {
	if weeks < 1 {
		weeks = 12
	}
	if weeks > 104 {
		weeks = 104
	}
	rows, err := socialWeeklyRows(weeks)
	if err != nil {
		return nil, err
	}
	out := make([]models.SocialWeeklyRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.SocialWeeklyRow{
			AccountID:  r.AccountID,
			OutletCode: r.Code,
			OutletName: r.Name,
			Platform:   r.Platform,
			Username:   r.Username,
			WeekStart:  r.WeekStart,
			Followers:  r.Followers,
			Posts:      r.Posts,
			Views:      r.Views,
			Engagement: r.Engage,
			Manual:     r.Manual,
			Approx:     r.Approx,
		})
	}
	return out, nil
}
