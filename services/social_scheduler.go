package services

import (
	"cloud-pos/database"
	"hash/fnv"
	"log"
	"time"
)

// ── Penjadwal penarikan medsos ───────────────────────────────────────────────
//
// Sekali sehari, dini hari waktu aplikasi. Harian dan bukan mingguan meskipun
// laporannya mingguan: jumlah pengikut adalah potret, dan potret yang hanya
// diambil sekali seminggu akan hilang seluruhnya begitu pengambilan minggu itu
// kebetulan terblokir. Dengan tujuh kesempatan per minggu, satu minggu baru
// benar-benar kosong kalau tujuh-tujuhnya gagal.

// Jendela jam penarikan, dalam menit sejak tengah malam waktu aplikasi:
// 03:00–05:39. Lalu lintas kedua platform paling sepi pada jam-jam ini, dan
// hasilnya sudah ada sebelum siapa pun membuka Analisa Bisnis pagi harinya.
const (
	socialJendelaMulai = 3 * 60
	socialJendelaLebar = 160
)

// socialMenitTarget memilih menit penarikan untuk satu hari.
//
// Diturunkan dari tanggalnya, bukan diacak saat dipanggil: dengan begitu ia
// tetap sama sepanjang hari itu (restart tidak menggeser jadwal), tetapi
// berpindah-pindah dari hari ke hari. Penarikan yang jatuh pada menit yang
// sama persis tiap pagi selama berbulan-bulan adalah tanda tangan yang terlalu
// rapi — justru keteraturan itu yang membedakan mesin dari orang.
func socialMenitTarget(hari string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(hari))
	return socialJendelaMulai + int(h.Sum32()%socialJendelaLebar)
}

func StartSocialScheduler() {
	go func() {
		// Jeda awal: saat boot, database dan migrasi baru selesai, dan menarik
		// belasan profil bersamaan dengan pekerjaan boot lain hanya memperlambat
		// keduanya. Menunggu sebentar juga mencegah deploy beruntun menembaki
		// kedua platform berkali-kali dalam hitungan menit.
		time.Sleep(3 * time.Minute)

		lastRunDay := ""
		now := time.Now().In(GetTimezoneLocation())
		if socialSudahDitarikHariIni() {
			lastRunDay = now.Format("2006-01-02")
		} else if now.Hour()*60+now.Minute() >= socialMenitTarget(now.Format("2006-01-02")) {
			// Server menyala setelah jam jadwal dan masih ada akun yang belum
			// punya potret hari ini — kejar sekarang, jangan biarkan satu hari
			// bolong hanya karena mesinnya kebetulan mati pada dini hari.
			// Penyaringan "yang belum punya potret" ada di ScrapeSocialAccounts,
			// jadi yang ditembak hanya yang benar-benar kurang.
			lastRunDay = runSocialScrape("kejar ketinggalan")
		}

		ticker := time.NewTicker(8 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().In(GetTimezoneLocation())
			day := now.Format("2006-01-02")
			if day == lastRunDay {
				continue
			}
			if now.Hour()*60+now.Minute() >= socialMenitTarget(day) {
				lastRunDay = runSocialScrape("jadwal harian")
			}
		}
	}()
}

func runSocialScrape(sebab string) string {
	hariIni := time.Now().In(GetTimezoneLocation()).Format("2006-01-02")
	res, err := ScrapeSocialAccounts(nil, true)
	if err != nil {
		log.Printf("[Medsos] penarikan (%s) gagal dijalankan: %v", sebab, err)
		return hariIni
	}
	log.Printf("[Medsos] penarikan (%s): %d akun, %d berhasil, %d gagal, %.1f detik",
		sebab, res.Attempted, res.OK, res.Failed, float64(res.TookMs)/1000)
	for _, e := range res.Errors {
		log.Printf("[Medsos]   %s", e)
	}
	return hariIni
}

// socialSudahDitarikHariIni benar hanya bila SETIAP akun yang dijadwalkan
// menarik sudah punya potret hari ini.
//
// Sengaja tidak sekadar "ada potret hari ini": akun yang baru didaftarkan siang
// hari akan menunggu sampai besok pagi kalau pemeriksaannya sekasar itu, dan
// orang yang baru saja menempel tautannya tidak punya cara tahu apakah ia sudah
// benar sampai satu hari berlalu.
func socialSudahDitarikHariIni() bool {
	var tertinggal int
	err := database.DB.QueryRow(`
		SELECT COUNT(*)
		FROM social_accounts a
		JOIN outlets o ON o.id = a.outlet_id
		WHERE a.is_active = true AND o.is_active = true AND a.auto_fetch = true
		  AND NOT EXISTS (
			SELECT 1 FROM social_snapshots s
			WHERE s.account_id = a.id AND s.captured_date = tz_today()
		  )`).Scan(&tertinggal)
	return err == nil && tertinggal == 0
}
