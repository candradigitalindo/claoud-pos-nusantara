package models

// ── WhatsApp ────────────────────────────────────────────────────────────────
//
// Gateway (wa-gateway) hanya tahu "kirim teks/gambar ke nomor/grup". Semua
// kebijakan — siapa penerima notifikasi apa, jeda antar pesan, jam tenang,
// batas harian, template — hidup di API utama dan tabel wa_*.

// WAGatewayStatus = snapshot /status dari wa-gateway + info keterjangkauan.
type WAGatewayStatus struct {
	Configured  bool   `json:"configured"`
	Reachable   bool   `json:"reachable"`
	Error       string `json:"error,omitempty"`
	State       string `json:"state"`
	Connected   bool   `json:"connected"`
	LoggedIn    bool   `json:"logged_in"`
	JID         string `json:"jid"`
	PushName    string `json:"push_name"`
	QR          string `json:"qr,omitempty"`
	QRExpiresAt string `json:"qr_expires_at,omitempty"`
	Note        string `json:"note"`
	SentCount   int64  `json:"sent_count"`
	LastSentAt  string `json:"last_sent_at,omitempty"`
	UptimeSec   int64  `json:"uptime_sec"`
}

type WAGroup struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	Participants int    `json:"participants"`
}

type WAQueueStats struct {
	Pending     int    `json:"pending"`
	SentToday   int    `json:"sent_today"`
	FailedToday int    `json:"failed_today"`
	Failed      int    `json:"failed"`
	LastSentAt  string `json:"last_sent_at"`
	DailyLimit  int    `json:"daily_limit"`
}

type WAStatus struct {
	Enabled bool            `json:"enabled"`
	Gateway WAGatewayStatus `json:"gateway"`
	Queue   WAQueueStats    `json:"queue"`
}

// WASettings — kebijakan pengiriman (app_settings, awalan wa_).
type WASettings struct {
	Enabled          bool              `json:"enabled"`
	GapMinSec        int               `json:"gap_min_sec"`
	GapMaxSec        int               `json:"gap_max_sec"`
	DailyLimit       int               `json:"daily_limit"`
	QuietStart       string            `json:"quiet_start"` // HH:MM, jam tenang untuk pesan ke pelanggan
	QuietEnd         string            `json:"quiet_end"`
	ReportHour       int               `json:"report_hour"`   // jam rekap penjualan harian
	ReminderHour     int               `json:"reminder_hour"` // jam pengingat reservasi H-1
	DeviceOfflineMin int               `json:"device_offline_min"`
	PublicBaseURL    string            `json:"public_base_url"` // untuk tautan status reservasi
	Events           map[string]bool   `json:"events"`          // event → aktif
	Templates        map[string]string `json:"templates"`       // event → template khusus (kosong = bawaan)
	// EventRoles: daftar role penerima per event yang MENIMPA aturan otomatis
	// (otomatis = role yang memegang izin terkait event). Kosong = otomatis.
	EventRoles map[string][]string `json:"event_roles"`
}

// WAEventDef — satu jenis notifikasi untuk layar pengaturan.
type WAEventDef struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Group        string   `json:"group"`
	Audience     string   `json:"audience"` // internal | customer
	Description  string   `json:"description"`
	Placeholders []string `json:"placeholders"`
	Default      string   `json:"default_template"`
	Template     string   `json:"template"`
	Enabled      bool     `json:"enabled"`
	Scheduled    bool     `json:"scheduled"`
	// Permissions: izin yang membuat sebuah role otomatis menjadi penerima.
	Permissions []string `json:"permissions"`
	// Roles: daftar role penimpa (dari pengaturan); kosong = otomatis.
	Roles []string `json:"roles"`
	// RouteDesc: penjelasan singkat "dikirim ke siapa" untuk layar.
	RouteDesc string `json:"route_desc"`
}

// WARecipient — penerima notifikasi internal (nomor pribadi atau grup WA).
type WARecipient struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Target    string   `json:"target"` // 628xxx atau 1203…@g.us
	Kind      string   `json:"kind"`   // phone | group
	Events    []string `json:"events"` // kosong/["*"] = semua event internal
	OutletIDs []string `json:"outlet_ids"`
	IsActive  bool     `json:"is_active"`
	Notes     string   `json:"notes"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// WAMessage — satu baris antrean/log pesan.
type WAMessage struct {
	ID          string `json:"id"`
	Target      string `json:"target"`
	TargetName  string `json:"target_name"`
	Audience    string `json:"audience"` // internal | customer
	Kind        string `json:"kind"`     // notify | broadcast | test
	Event       string `json:"event"`
	RefID       string `json:"ref_id"`
	BroadcastID string `json:"broadcast_id"`
	Body        string `json:"body"`
	ImageURL    string `json:"image_url"`
	Status      string `json:"status"` // pending | sent | failed | cancelled
	Attempts    int    `json:"attempts"`
	LastError   string `json:"last_error"`
	WAMessageID string `json:"wa_message_id"`
	ScheduledAt string `json:"scheduled_at"`
	SentAt      string `json:"sent_at"`
	CreatedAt   string `json:"created_at"`
}

// WABroadcast — kampanye kirim massal.
type WABroadcast struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Body       string                 `json:"body"`
	ImageURL   string                 `json:"image_url"`
	Audience   string                 `json:"audience"` // customers | recipients | manual
	Filter     map[string]interface{} `json:"filter"`
	Total      int                    `json:"total"`
	Sent       int                    `json:"sent"`
	Failed     int                    `json:"failed"`
	Pending    int                    `json:"pending"`
	Status     string                 `json:"status"` // queued | done | cancelled
	CreatedBy  string                 `json:"created_by"`
	CreatedAt  string                 `json:"created_at"`
	FinishedAt string                 `json:"finished_at"`
}

type WABroadcastRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	ImageURL string `json:"image_url"`
	Audience string `json:"audience"`
	// Filter pelanggan
	OutletID       string `json:"outlet_id"`
	MinVisits      int    `json:"min_visits"`
	LastVisitDays  int    `json:"last_visit_days"`  // hanya yang datang dalam N hari terakhir (0 = abaikan)
	NotVisitedDays int    `json:"not_visited_days"` // hanya yang TIDAK datang ≥ N hari (0 = abaikan)
	// Daftar manual: satu nomor per baris, boleh "nomor, nama"
	Manual string `json:"manual"`
	SendAt string `json:"send_at"` // YYYY-MM-DD HH:MM (zona aplikasi), kosong = sekarang
}

type WAAudiencePreview struct {
	Total  int        `json:"total"`
	Sample []WATarget `json:"sample"`
}

type WATarget struct {
	Target string `json:"target"`
	Name   string `json:"name"`
}

// WARecipientUser — akun pengguna sebagai penerima notifikasi berdasarkan posisi.
type WARecipientUser struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	WAPhone     string   `json:"wa_phone"`
	WANotify    bool     `json:"wa_notify"`
	IsActive    bool     `json:"is_active"`
	ScopeType   string   `json:"scope_type"` // all | specific
	OutletNames []string `json:"outlet_names"`
	Events      []string `json:"events"` // event internal yang akan diterimanya
}

// WARoleOption — pilihan role untuk penimpa posisi per event.
type WARoleOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ScopeType   string `json:"scope_type"`
	Users       int    `json:"users"` // pengguna aktif ber-nomor WA pada role ini
}
