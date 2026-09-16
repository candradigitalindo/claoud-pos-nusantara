package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud-pos/database"
)

// Pencadangan bukti foto ke Google Drive.
//
// Memakai rclone dengan remote "gdrive:" yang SUDAH dikonfigurasi di host dan
// dipakai cron backup database harian (scripts/backup-db.sh). Berkas konfigurasi
// dipasang read-only ke dalam container lewat docker-compose, sehingga tidak ada
// kredensial baru yang perlu disimpan di database.
//
// Foto dijadikan publik ("siapa saja yang punya link") supaya bisa dipanggil
// langsung dari layar tanpa login — keputusan pemilik sistem, 16 Sep 2026.
// Konsekuensinya: siapa pun yang memegang tautannya bisa membuka fotonya.

const driveRemoteDefault = "gdrive:cloud-pos-photos"

// Lokasi konfigurasi rclone: berkas milik host dipasang read-only, lalu disalin
// ke lokasi yang bisa ditulis.
//
// rclone menyegarkan token OAuth Google dan menulisnya kembali ke config. Bila
// berkasnya read-only, tiap pemanggilan memuntahkan "Failed to save config ...
// device or resource busy" ke stderr — yang dulu ikut tersimpan sebagai tautan.
// Salinan lokal membuat rclone bisa menyimpan tokennya sendiri tanpa menyentuh
// berkas host yang juga dipakai cron backup database.
const (
	rcloneSeedConfig = "/etc/rclone/rclone.conf"
	rcloneLiveConfig = "/root/.config/rclone/rclone.conf"
)

// prepareRcloneConfig menyalin konfigurasi sekali saat aplikasi menyala.
func prepareRcloneConfig() {
	seed, err := os.ReadFile(rcloneSeedConfig)
	if err != nil {
		return // tidak dipasang: pencadangan Drive memang tidak dipakai
	}
	if cur, err := os.ReadFile(rcloneLiveConfig); err == nil && len(cur) > 0 {
		return // sudah ada salinan yang bisa ditulis
	}
	if err := os.MkdirAll(filepath.Dir(rcloneLiveConfig), 0o700); err != nil {
		log.Printf("[Foto] Gagal menyiapkan folder konfigurasi rclone: %v", err)
		return
	}
	if err := os.WriteFile(rcloneLiveConfig, seed, 0o600); err != nil {
		log.Printf("[Foto] Gagal menyalin konfigurasi rclone: %v", err)
		return
	}
	log.Printf("[Foto] Konfigurasi rclone disalin ke lokasi yang bisa ditulis")
}

// driveIDPattern mengambil id berkas dari keluaran `rclone link`, yang berbentuk
// https://drive.google.com/open?id=<ID> atau .../file/d/<ID>/view
var driveIDPattern = regexp.MustCompile(`(?:[?&]id=|/d/)([A-Za-z0-9_-]{10,})`)

func driveRemote() string {
	if v, err := GetSetting("photo_drive_remote"); err == nil && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return driveRemoteDefault
}

// driveEnabled: pencadangan hidup bila rclone ada DAN remote-nya terdaftar.
func driveEnabled() (bool, string) {
	bin, err := exec.LookPath("rclone")
	if err != nil {
		return false, "rclone tidak terpasang di server aplikasi"
	}
	remote := driveRemote()
	name := strings.SplitN(remote, ":", 2)[0] + ":"
	out, err := exec.Command(bin, "listremotes").Output()
	if err != nil {
		return false, "gagal membaca daftar remote rclone: " + err.Error()
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true, ""
		}
	}
	return false, fmt.Sprintf("remote %q belum dikonfigurasi (jalankan: rclone config)", name)
}

// uploadPhotoToDrive menyalin satu berkas dan mengembalikan id + tautan publiknya.
func uploadPhotoToDrive(localURL string) (string, string, error) {
	path := strings.TrimPrefix(localURL, "/")
	if _, err := os.Stat(path); err != nil {
		return "", "", fmt.Errorf("berkas tidak ditemukan: %w", err)
	}
	// Susunan folder mengikuti bulan, sama seperti di server: gampang ditelusuri.
	sub := filepath.Base(filepath.Dir(path))
	dest := fmt.Sprintf("%s/%s", driveRemote(), sub)

	ctxCopy := exec.Command("rclone", "copy", path, dest, "--timeout", "3m")
	if out, err := ctxCopy.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("rclone copy gagal: %s", strings.TrimSpace(string(out)))
	}

	// `rclone link` sekaligus memasang izin "anyone with link" pada Drive.
	//
	// stdout dan stderr DIPISAH: rclone kadang menulis peringatan ke stderr,
	// dan menggabungkannya dulu membuat pesan peringatan ikut tersimpan
	// sebagai "tautan" di database.
	remoteFile := fmt.Sprintf("%s/%s", dest, filepath.Base(path))
	cmd := exec.Command("rclone", "link", remoteFile, "--timeout", "2m")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", "", fmt.Errorf("rclone link gagal: %s", msg)
	}
	// Ambil baris terakhir yang berisi URL — bukan seluruh keluaran.
	link := ""
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			link = line
		}
	}
	m := driveIDPattern.FindStringSubmatch(link)
	if len(m) < 2 {
		return "", link, fmt.Errorf("tautan Drive tidak dikenali: %q", link)
	}
	return m[1], link, nil
}

// BackupPhotosToDrive mengirim antrean foto ke Drive. Idempoten: hanya baris
// 'pending' yang diproses, dan tiap hasil ditandai per baris.
func BackupPhotosToDrive() {
	ok, reason := driveEnabled()
	if !ok {
		var n int
		database.DB.QueryRow(`SELECT COUNT(*) FROM handover_photos WHERE backup_status = 'pending'`).Scan(&n)
		if n > 0 {
			log.Printf("[Foto] %d bukti foto menunggu dicadangkan, tapi %s", n, reason)
		}
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, photo_url FROM handover_photos
		WHERE backup_status = 'pending' ORDER BY created_at LIMIT 20`)
	if err != nil {
		log.Printf("[Foto] Gagal membaca antrean: %v", err)
		return
	}
	type item struct{ id, url string }
	queue := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.url); err == nil {
			queue = append(queue, it)
		}
	}
	rows.Close()
	if len(queue) == 0 {
		return
	}

	sent, failed := 0, 0
	for _, it := range queue {
		fileID, link, err := uploadPhotoToDrive(it.url)
		if err != nil {
			markPhotoBackup(it.id, "failed", err.Error())
			failed++
			continue
		}
		database.DB.Exec(`UPDATE handover_photos SET backup_status='sent', backup_error='',
			backup_at=(now() AT TIME ZONE 'UTC'), drive_file_id=$1, drive_url=$2 WHERE id=$3`,
			fileID, link, it.id)
		sent++
	}
	if sent > 0 || failed > 0 {
		log.Printf("[Foto] Cadangan Drive: %d tersalin, %d gagal", sent, failed)
	}
}

// StartPhotoBackupScheduler menyalin antrean foto ke Drive tiap 15 menit.
func StartPhotoBackupScheduler() {
	prepareRcloneConfig()
	go func() {
		time.Sleep(30 * time.Second)
		BackupPhotosToDrive()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			BackupPhotosToDrive()
		}
	}()
}

// RetryFailedPhotoBackups mengembalikan yang gagal ke antrean.
func RetryFailedPhotoBackups() (int, error) {
	res, err := database.DB.Exec(`UPDATE handover_photos SET backup_status='pending', backup_error=''
		WHERE backup_status = 'failed'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	go BackupPhotosToDrive()
	return int(n), nil
}

// PhotoBackupStatus merangkum keadaan pencadangan untuk halaman Pengaturan.
type PhotoBackupStatus struct {
	Configured bool   `json:"configured"`
	Remote     string `json:"remote"`
	Reason     string `json:"reason"`
	Pending    int    `json:"pending"`
	Sent       int    `json:"sent"`
	Failed     int    `json:"failed"`
	LastError  string `json:"last_error"`
	LastSentAt string `json:"last_sent_at"`
}

func GetPhotoBackupStatus() (*PhotoBackupStatus, error) {
	ok, reason := driveEnabled()
	s := &PhotoBackupStatus{Configured: ok, Remote: driveRemote(), Reason: reason}
	database.DB.QueryRow(`
		SELECT COUNT(*) FILTER (WHERE backup_status='pending'),
		       COUNT(*) FILTER (WHERE backup_status='sent'),
		       COUNT(*) FILTER (WHERE backup_status='failed'),
		       COALESCE(MAX(backup_error) FILTER (WHERE backup_status='failed'), ''),
		       COALESCE(TO_CHAR(MAX(backup_at) FILTER (WHERE backup_status='sent'), 'YYYY-MM-DD HH24:MI'), '')
		FROM handover_photos`).Scan(&s.Pending, &s.Sent, &s.Failed, &s.LastError, &s.LastSentAt)
	return s, nil
}

func markPhotoBackup(id, status, errMsg string) {
	database.DB.Exec(`UPDATE handover_photos SET backup_status=$1, backup_error=$2,
		backup_at=(now() AT TIME ZONE 'UTC') WHERE id=$3`, status, errMsg, id)
}
