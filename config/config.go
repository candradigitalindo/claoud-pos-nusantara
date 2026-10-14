package config

import (
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	JWTSecret       string
	WebhookSecret   string
	RateLimitPerMin int
	AdminToken      string
	// CCTV relay (go2rtc). Go2rtcURL = alamat API internal media server.
	// CameraEncKey = kunci enkripsi kredensial RTSP (opsional; kosong → turunan JWT_SECRET).
	Go2rtcURL     string
	CameraEncKey  string
	// Gateway WhatsApp (layanan wa-gateway di compose). URL kosong = fitur WA mati.
	WAGatewayURL   string
	// AnalyticsURL = layanan Python (tren, perkiraan, pola hari) untuk Analisa Bisnis; kosong = nonaktif.
	AnalyticsURL   string
	// Narasi AI Analisa Bisnis: penyedia (openai | anthropic), alamat, model, kunci.
	NarrativeProvider string
	NarrativeBaseURL  string
	NarrativeModel    string
	NarrativeAPIKey   string
	WAGatewayToken string
}

func (c *Config) DSN() string {
	dsn := "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
	if c.DBPassword != "" {
		dsn += " password=" + c.DBPassword
	}
	return dsn
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	rateLimit := 100
	if v := os.Getenv("RATE_LIMIT_PER_MINUTE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rateLimit = n
		}
	}

	cfg := &Config{
		Port:            getEnv("PORT", "3000"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "postgres"),
		DBPassword:      getEnv("DB_PASSWORD", ""),
		DBName:          getEnv("DB_NAME", "cloud_pos"),
		DBSSLMode:       getEnv("DB_SSLMODE", "disable"),
		JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
		WebhookSecret:   getEnv("WEBHOOK_SECRET", "webhook-secret-key"),
		RateLimitPerMin: rateLimit,
		AdminToken:      getEnv("ADMIN_TOKEN", ""), // tanpa bawaan: kosong = jalur token statis nonaktif
		Go2rtcURL:       getEnv("GO2RTC_URL", "http://go2rtc:1984"),
		CameraEncKey:    getEnv("CAMERA_ENC_KEY", ""),
		WAGatewayURL:    getEnv("WA_GATEWAY_URL", ""),
		AnalyticsURL:    getEnv("ANALYTICS_URL", ""),
		NarrativeProvider: getEnv("NARRATIVE_PROVIDER", ""),
		NarrativeBaseURL:  getEnv("NARRATIVE_BASE_URL", ""),
		NarrativeModel:    getEnv("NARRATIVE_MODEL", ""),
		// ANTHROPIC_API_KEY sengaja TIDAK dijadikan cadangan di sini: kunci itu
		// hanya boleh dipakai bila penyedianya Anthropic (lihat InitNarrativeAI),
		// bukan dikirim sebagai Bearer ke titik akhir kompatibel OpenAI.
		NarrativeAPIKey:   getEnv("NARRATIVE_API_KEY", ""),
		WAGatewayToken:  getEnv("WA_GATEWAY_TOKEN", ""),
	}

	if p := cfg.StaticAdminTokenProblem(); p != "" {
		log.Printf("[Auth] PERINGATAN: jalur token statis ADMIN_TOKEN NONAKTIF — %s. "+
			"Akses admin hanya lewat login (JWT). Bila memang perlu, isi ADMIN_TOKEN "+
			"dengan nilai acak: openssl rand -hex 32", p)
	}
	// JWT_SECRET lemah = siapa pun bisa menandatangani sesi admin sendiri,
	// termasuk untuk akun superadmin bawaan yang id-nya tercantum di seeder.
	// Hanya diperingatkan, tidak diganti otomatis: bila CAMERA_ENC_KEY kosong,
	// kunci enkripsi kredensial kamera diturunkan dari JWT_SECRET — rahasia
	// acak per proses membuat kredensial tersimpan tak terbaca setelah restart.
	if p := cfg.JWTSecretProblem(); p != "" {
		log.Printf("[Auth] PERINGATAN KEAMANAN: %s — sesi admin bisa dipalsukan. "+
			"Isi JWT_SECRET di .env dengan nilai acak (openssl rand -hex 32). Bila "+
			"CAMERA_ENC_KEY kosong, isi dulu CAMERA_ENC_KEY=cctv:<JWT_SECRET lama> "+
			"supaya kredensial kamera yang sudah tersimpan tetap terbaca", p)
	}
	return cfg
}

// JWTSecretProblem menjelaskan kenapa JWT_SECRET tidak layak dipakai
// (kosong, nilai bawaan/contoh repo, atau terlalu pendek). String kosong
// berarti layak.
func (c *Config) JWTSecretProblem() string {
	norm := strings.ToLower(strings.TrimSpace(c.JWTSecret))
	if norm == "" {
		return "JWT_SECRET kosong"
	}
	for _, w := range weakAdminTokens {
		if strings.Contains(norm, w) {
			return "JWT_SECRET masih nilai bawaan/contoh yang tercantum di repo"
		}
	}
	if len(c.JWTSecret) < MinAdminTokenLen {
		return "JWT_SECRET kurang dari 32 karakter"
	}
	return ""
}

// ── Token admin statis ──────────────────────────────────────────────────────
//
// ADMIN_TOKEN adalah jalan pintas lama: pembawanya diperlakukan sebagai
// superadmin dengan scope semua outlet. Nilai bawaannya dulu tertulis di repo
// dan masih terpasang di produksi (temuan T-1, 14 Okt 2026). Sekarang jalurnya
// hanya hidup bila nilainya diisi dengan sengaja dan cukup kuat; selain itu
// token statis diabaikan. Middleware AdminAuth dan stream SSE memakai helper
// yang sama agar aturannya tidak pernah berbeda.

// MinAdminTokenLen: panjang minimum ADMIN_TOKEN (openssl rand -hex 32 = 64).
const MinAdminTokenLen = 32

// weakAdminTokens: nilai bawaan/contoh yang pernah beredar di repo dan
// dokumentasinya. Dicocokkan sebagai potongan, jadi nilai bawaan yang sekadar
// diulang atau ditambal tetap tertolak.
var weakAdminTokens = []string{
	"admin-secret-token",
	"change-me-in-production",
	"your-cloud-jwt-secret-change-in-production",
	"shared-secret-between-cloud-and-pos",
	"webhook-secret-key",
	"webhook-secret",
	"wa-gateway-secret",
	"ganti-dengan-token-acak",
}

// StaticAdminTokenProblem menjelaskan kenapa jalur token statis nonaktif.
// String kosong berarti aktif.
func (c *Config) StaticAdminTokenProblem() string {
	t := c.AdminToken
	norm := strings.ToLower(strings.TrimSpace(t))
	if norm == "" {
		return "ADMIN_TOKEN kosong"
	}
	for _, w := range weakAdminTokens {
		if strings.Contains(norm, w) {
			return "ADMIN_TOKEN masih nilai bawaan/contoh yang dikenal"
		}
	}
	if len(t) < MinAdminTokenLen {
		return "ADMIN_TOKEN kurang dari 32 karakter"
	}
	// Nilai seragam ("aaaa…", "1212…") panjang tetapi mudah ditebak.
	distinct := map[rune]bool{}
	for _, r := range t {
		distinct[r] = true
	}
	if len(distinct) < 8 {
		return "ADMIN_TOKEN terlalu seragam (kurang dari 8 karakter berbeda)"
	}
	return ""
}

// StaticAdminTokenMatches: true hanya bila jalur token statis aktif dan token
// sama persis dengan ADMIN_TOKEN. Perbandingan waktu-konstan atas SHA-256
// kedua nilai, sehingga isi maupun panjang token tidak bocor lewat waktu.
func (c *Config) StaticAdminTokenMatches(token string) bool {
	if token == "" || c.StaticAdminTokenProblem() != "" {
		return false
	}
	got := sha256.Sum256([]byte(token))
	want := sha256.Sum256([]byte(c.AdminToken))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
