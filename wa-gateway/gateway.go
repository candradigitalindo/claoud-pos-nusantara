package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// State koneksi yang dilaporkan ke API utama / layar admin.
const (
	stateLoggedOut  = "logged_out" // belum ada perangkat terpasang; perlu pindai QR
	statePairing    = "pairing"    // QR sedang ditampilkan, menunggu dipindai
	stateQRTimeout  = "qr_timeout" // QR habis masa berlakunya tanpa dipindai
	stateConnecting = "connecting" // perangkat ada, sedang menyambung ke server WA
	stateConnected  = "connected"  // siap kirim
	stateOffline    = "offline"    // perangkat ada, koneksi putus (reconnect otomatis)
	stateBanned     = "banned"     // WhatsApp memblokir sementara nomor ini
	stateError      = "error"
)

// permanentError: kegagalan yang tidak akan sembuh dengan diulang (nomor tidak
// terdaftar, JID salah). API utama langsung menandai pesan gagal tanpa retry.
type permanentError struct{ msg string }

func (e permanentError) Error() string { return e.msg }

type onWACache struct {
	jid       types.JID
	isIn      bool
	checkedAt time.Time
}

type gateway struct {
	mu        sync.Mutex
	container *sqlstore.Container
	client    *whatsmeow.Client

	state     string
	note      string
	qrCode    string
	qrExpires time.Time
	pairing   context.CancelFunc

	sendMu    sync.Mutex // satu pengiriman pada satu waktu
	sentCount int64
	lastSent  time.Time
	startedAt time.Time

	onWAMu sync.Mutex
	onWA   map[string]onWACache

	http *http.Client
}

type statusSnapshot struct {
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

func newGateway(dsn string) (*gateway, error) {
	ctx := context.Background()
	container, err := sqlstore.New(ctx, "postgres", dsn, waLog.Stdout("DB", "WARN", false))
	if err != nil {
		return nil, fmt.Errorf("buka penyimpanan sesi: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca perangkat: %w", err)
	}
	g := &gateway{
		container: container,
		state:     stateLoggedOut,
		note:      "Belum ada nomor terpasang. Klik Tampilkan QR lalu pindai dari aplikasi WhatsApp.",
		onWA:      map[string]onWACache{},
		startedAt: time.Now(),
		http:      &http.Client{Timeout: 30 * time.Second},
	}
	g.setClient(device)
	if device.ID != nil {
		g.state = stateConnecting
		g.note = "Menyambung ke WhatsApp..."
		go g.connectExisting()
	}
	return g, nil
}

// setClient membuat *whatsmeow.Client baru untuk sebuah perangkat. Dipanggil
// saat boot dan setiap kali perangkat lama dibuang (logout / dikeluarkan dari
// HP), karena Client lama memegang store yang sudah dihapus.
func (g *gateway) setClient(device *store.Device) {
	cli := whatsmeow.NewClient(device, waLog.Stdout("Client", "INFO", false))
	cli.EnableAutoReconnect = true
	cli.AutoTrustIdentity = true
	cli.AddEventHandler(g.handleEvent)
	g.client = cli
}

func (g *gateway) connectExisting() {
	if err := g.client.Connect(); err != nil {
		g.mu.Lock()
		g.state = stateOffline
		g.note = "Gagal menyambung: " + err.Error()
		g.mu.Unlock()
		log.Printf("Connect gagal: %v", err)
	}
}

func (g *gateway) handleEvent(evt interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch v := evt.(type) {
	case *events.Connected:
		g.state = stateConnected
		g.qrCode = ""
		g.note = "Tersambung"
		log.Printf("Tersambung sebagai %s", g.jidString())
	case *events.PairSuccess:
		g.note = "Perangkat dipasangkan: " + v.ID.String()
		log.Printf("Pairing sukses: %s (%s)", v.ID, v.Platform)
	case *events.Disconnected:
		if g.state == stateConnected {
			g.state = stateOffline
			g.note = "Koneksi putus, menyambung ulang otomatis..."
		}
	case *events.StreamReplaced:
		g.state = stateOffline
		g.note = "Sesi diambil alih koneksi lain (gateway dijalankan dua kali?)"
	case *events.LoggedOut:
		// whatsmeow sudah menghapus perangkat dari store; siapkan perangkat
		// kosong agar pemindaian QR berikutnya bisa langsung dilakukan.
		g.state = stateLoggedOut
		g.qrCode = ""
		g.note = "Nomor dikeluarkan dari perangkat tertaut (logout dari HP). Pindai QR lagi."
		log.Printf("Logged out (onConnect=%v, reason=%v)", v.OnConnect, v.Reason)
		go g.replaceDevice()
	case *events.TemporaryBan:
		g.state = stateBanned
		g.note = "WhatsApp memblokir sementara nomor ini: " + v.String()
		log.Printf("BANNED: %s", v.String())
	case *events.ClientOutdated:
		g.state = stateError
		g.note = "Versi pustaka whatsmeow sudah kedaluwarsa — gateway perlu di-build ulang dengan versi terbaru."
		log.Printf("Client outdated")
	case *events.ConnectFailure:
		g.state = stateError
		g.note = fmt.Sprintf("Koneksi ditolak (%v): %s", v.Reason, v.Message)
		log.Printf("Connect failure: %v %s", v.Reason, v.Message)
	case *events.KeepAliveTimeout:
		g.note = "Server tidak merespons keep-alive, mencoba lagi..."
	case *events.Message:
		// Pesan masuk tidak diproses (gateway satu arah). Dicatat singkat
		// supaya terlihat di log bahwa koneksi hidup.
		if v.Info.Chat.Server == types.DefaultUserServer {
			log.Printf("Pesan masuk dari %s (diabaikan)", v.Info.Chat.User)
		}
	}
}

func (g *gateway) replaceDevice() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.client.Disconnect()
	g.setClient(g.container.NewDevice())
}

func (g *gateway) jidString() string {
	if g.client == nil || g.client.Store == nil || g.client.Store.ID == nil {
		return ""
	}
	return g.client.Store.ID.User
}

func (g *gateway) snapshot() statusSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := statusSnapshot{
		State:     g.state,
		Note:      g.note,
		SentCount: g.sentCount,
		UptimeSec: int64(time.Since(g.startedAt).Seconds()),
	}
	if g.client != nil {
		s.Connected = g.client.IsConnected()
		s.LoggedIn = g.client.IsLoggedIn()
		s.JID = g.jidString()
		if g.client.Store != nil {
			s.PushName = g.client.Store.PushName
		}
	}
	if s.LoggedIn && s.Connected && g.state != stateBanned {
		s.State = stateConnected
	}
	if g.qrCode != "" && time.Now().Before(g.qrExpires) {
		s.QR = g.qrCode
		s.QRExpiresAt = g.qrExpires.UTC().Format(time.RFC3339)
	}
	if !g.lastSent.IsZero() {
		s.LastSentAt = g.lastSent.UTC().Format(time.RFC3339)
	}
	return s
}

// startLogin memulai pemindaian QR. Bila QR yang masih berlaku sedang
// ditampilkan, tidak memulai ulang (klien tinggal ambil kode dari /status).
func (g *gateway) startLogin() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.client.IsLoggedIn() {
		return nil
	}
	if g.state == statePairing && g.qrCode != "" && time.Now().Before(g.qrExpires) {
		return nil
	}
	if g.pairing != nil {
		g.pairing()
		g.pairing = nil
	}
	if g.client.IsConnected() {
		g.client.Disconnect()
	}
	if g.client.Store.ID != nil {
		// Store berisi ID tapi tidak login (sisa sesi rusak): mulai dari perangkat baru.
		g.setClient(g.container.NewDevice())
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := g.client.GetQRChannel(ctx)
	if err != nil {
		cancel()
		return err
	}
	if err := g.client.Connect(); err != nil {
		cancel()
		return err
	}
	g.pairing = cancel
	g.state = statePairing
	g.qrCode = ""
	g.note = "Menunggu kode QR dari server..."
	go g.readQR(ch)
	return nil
}

func (g *gateway) readQR(ch <-chan whatsmeow.QRChannelItem) {
	for item := range ch {
		g.mu.Lock()
		switch item.Event {
		case whatsmeow.QRChannelEventCode:
			g.state = statePairing
			g.qrCode = item.Code
			g.qrExpires = time.Now().Add(item.Timeout)
			g.note = "Pindai QR dari WhatsApp di HP: Perangkat Tertaut → Tautkan perangkat."
		case whatsmeow.QRChannelSuccess.Event:
			g.qrCode = ""
			g.state = stateConnecting
			g.note = "QR dipindai, menyelesaikan pemasangan..."
		case whatsmeow.QRChannelTimeout.Event:
			g.qrCode = ""
			g.state = stateQRTimeout
			g.note = "QR kedaluwarsa sebelum dipindai. Klik Tampilkan QR lagi."
		default:
			g.qrCode = ""
			g.state = stateError
			g.note = "Pemasangan gagal: " + item.Event
			if item.Error != nil {
				g.note += " — " + item.Error.Error()
			}
		}
		g.mu.Unlock()
	}
	g.mu.Lock()
	g.pairing = nil
	g.mu.Unlock()
}

// logout melepas nomor dari gateway (dan dari daftar perangkat tertaut di HP).
func (g *gateway) logout() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pairing != nil {
		g.pairing()
		g.pairing = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if g.client.IsLoggedIn() {
		if err := g.client.Logout(ctx); err != nil {
			// Paksa: putus koneksi dan buang data lokal supaya QR baru bisa dipindai.
			g.client.Disconnect()
			_ = g.client.Store.Delete(ctx)
			log.Printf("Logout tidak bersih (%v); data lokal dibuang paksa", err)
		}
	} else {
		g.client.Disconnect()
		if g.client.Store.ID != nil {
			_ = g.client.Store.Delete(ctx)
		}
	}
	g.setClient(g.container.NewDevice())
	g.state = stateLoggedOut
	g.qrCode = ""
	g.note = "Nomor dilepas. Pindai QR untuk memasang nomor lagi."
	g.onWAMu.Lock()
	g.onWA = map[string]onWACache{}
	g.onWAMu.Unlock()
	return nil
}

func (g *gateway) shutdown() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pairing != nil {
		g.pairing()
	}
	if g.client != nil {
		g.client.Disconnect()
	}
	if g.container != nil {
		g.container.Close()
	}
}

// normalizePhone: "0812-3456" → "628123456"; "+62 812" → "62812"; "812…" → "62812…".
func normalizePhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	}
	if len(d) < 9 || len(d) > 16 {
		return ""
	}
	return d
}

// resolveJID menerima nomor HP atau JID grup (…@g.us). Nomor dicek ke server
// apakah terdaftar di WhatsApp; hasilnya di-cache 24 jam.
func (g *gateway) resolveJID(ctx context.Context, to string) (types.JID, error) {
	to = strings.TrimSpace(to)
	if strings.Contains(to, "@") {
		jid, err := types.ParseJID(to)
		if err != nil {
			return types.JID{}, permanentError{"JID tidak valid: " + err.Error()}
		}
		return jid, nil
	}
	phone := normalizePhone(to)
	if phone == "" {
		return types.JID{}, permanentError{"nomor tidak valid: " + to}
	}
	g.onWAMu.Lock()
	c, ok := g.onWA[phone]
	g.onWAMu.Unlock()
	if ok && time.Since(c.checkedAt) < 24*time.Hour {
		if !c.isIn {
			return types.JID{}, permanentError{"nomor +" + phone + " tidak terdaftar di WhatsApp"}
		}
		return c.jid, nil
	}
	resp, err := g.client.IsOnWhatsApp(ctx, []string{"+" + phone})
	if err != nil {
		return types.JID{}, fmt.Errorf("cek nomor gagal: %w", err)
	}
	res := onWACache{checkedAt: time.Now()}
	if len(resp) > 0 {
		res.isIn = resp[0].IsIn
		res.jid = resp[0].JID
	}
	if res.jid.IsEmpty() {
		res.jid = types.NewJID(phone, types.DefaultUserServer)
	}
	g.onWAMu.Lock()
	g.onWA[phone] = res
	g.onWAMu.Unlock()
	if !res.isIn {
		return types.JID{}, permanentError{"nomor +" + phone + " tidak terdaftar di WhatsApp"}
	}
	return res.jid, nil
}

func (g *gateway) fetchImage(ctx context.Context, url string) ([]byte, string, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, "", permanentError{"URL gambar harus http(s)"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", permanentError{"URL gambar tidak valid"}
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("unduh gambar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("unduh gambar: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 6*1024*1024))
	if err != nil {
		return nil, "", err
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		return nil, "", permanentError{"gambar harus JPEG/PNG/WebP (terdeteksi " + mime + ")"}
	}
	return data, mime, nil
}

// send mengirim satu pesan teks (atau gambar + keterangan). Serial: satu
// pengiriman pada satu waktu, supaya pola lalu lintas menyerupai manusia.
func (g *gateway) send(ctx context.Context, to, text, imageURL string) (string, error) {
	if !g.client.IsLoggedIn() {
		return "", errors.New("gateway belum login (pindai QR dulu)")
	}
	if !g.client.IsConnected() {
		return "", errors.New("gateway sedang tidak tersambung ke WhatsApp")
	}
	g.sendMu.Lock()
	defer g.sendMu.Unlock()

	jid, err := g.resolveJID(ctx, to)
	if err != nil {
		return "", err
	}
	var msg *waE2E.Message
	if imageURL != "" {
		data, mime, err := g.fetchImage(ctx, imageURL)
		if err != nil {
			return "", err
		}
		up, err := g.client.Upload(ctx, data, whatsmeow.MediaImage)
		if err != nil {
			return "", fmt.Errorf("unggah gambar ke WhatsApp: %w", err)
		}
		msg = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			Caption:       proto.String(text),
		}}
	} else {
		msg = &waE2E.Message{Conversation: proto.String(text)}
	}

	// Tanda "mengetik" sejenak sebelum kirim: jejak yang lebih wajar dari
	// pengirim manusia; pesan pendek ~1 detik, panjang ~3 detik.
	if jid.Server == types.DefaultUserServer {
		_ = g.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		d := time.Duration(800+len(text)*8) * time.Millisecond
		if d > 3*time.Second {
			d = 3 * time.Second
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return "", ctx.Err()
		}
		_ = g.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	resp, err := g.client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	g.sentCount++
	g.lastSent = time.Now()
	g.mu.Unlock()
	return string(resp.ID), nil
}

type groupInfo struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	Participants int    `json:"participants"`
}

func (g *gateway) groups(ctx context.Context) ([]groupInfo, error) {
	if !g.client.IsLoggedIn() {
		return nil, errors.New("gateway belum login")
	}
	list, err := g.client.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]groupInfo, 0, len(list))
	for _, gi := range list {
		out = append(out, groupInfo{JID: gi.JID.String(), Name: gi.Name, Participants: len(gi.Participants)})
	}
	return out, nil
}

func (g *gateway) check(ctx context.Context, phone string) (bool, string, error) {
	if !g.client.IsLoggedIn() {
		return false, "", errors.New("gateway belum login")
	}
	jid, err := g.resolveJID(ctx, phone)
	var pe permanentError
	if errors.As(err, &pe) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, jid.String(), nil
}
