package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ── Katalog notifikasi WhatsApp ─────────────────────────────────────────────
//
// Setiap kejadian bisnis yang layak diberitahukan punya satu entri di sini:
// kunci, siapa penerimanya (internal = daftar penerima; customer = nomor
// pelanggan pada dokumen), placeholder yang tersedia, dan template bawaan
// yang bisa ditimpa admin dari layar WhatsApp → Notifikasi.
//
// Format WhatsApp: *tebal*, _miring_, ~coret~, ```monospace```.

type waEventSpec struct {
	Key          string
	Label        string
	Group        string
	Audience     string // internal | customer
	Description  string
	Placeholders []string
	Template     string
	Scheduled    bool
	// Permissions: izin yang membuat role otomatis menjadi penerima (event internal).
	Permissions []string
	// Extra: keterangan penerima tambahan di luar aturan izin (mis. pengaju).
	Extra string
}

var waEventSpecs = []waEventSpec{
	// ── Reservasi (internal) ──
	{
		Key: "reservation.created", Label: "Reservasi baru", Group: "Reservasi", Audience: "internal",
		Permissions:  []string{"reservations.view"},
		Description:  "Setiap reservasi baru dari halaman publik maupun yang dicatat admin.",
		Placeholders: []string{"company", "outlet", "customer_name", "customer_phone", "date", "time", "pax", "items", "total", "down_payment", "source", "notes"},
		Template: `🍽️ *Reservasi Baru — {outlet}*
Nama: {customer_name} ({customer_phone})
Jadwal: {date} pukul {time} · {pax} orang
Total: {total} · DP diminta: {down_payment}
Sumber: {source}

Pesanan:
{items}

Catatan: {notes}`,
	},
	{
		Key: "reservation.payment_submitted", Label: "Bukti pembayaran masuk (perlu validasi)", Group: "Reservasi", Audience: "internal",
		Permissions:  []string{"reservations.view"},
		Description:  "Pelanggan mengunggah bukti transfer dari halaman status reservasi.",
		Placeholders: []string{"company", "outlet", "customer_name", "customer_phone", "date", "time", "payment_type", "amount", "method", "remaining", "proof_url"},
		Template: `💳 *Bukti Pembayaran Masuk — {outlet}*
{customer_name} ({customer_phone}) mengirim bukti {payment_type} sebesar *{amount}* via {method}.
Reservasi: {date} pukul {time}
Sisa tagihan: {remaining}

Mohon divalidasi di menu Reservasi.
{proof_url}`,
	},
	{
		Key: "reservation.confirmed", Label: "Reservasi terkonfirmasi (DP tervalidasi)", Group: "Reservasi", Audience: "internal",
		Permissions:  []string{"reservations.view"},
		Description:  "DP yang diminta sudah terpenuhi oleh pembayaran tervalidasi.",
		Placeholders: []string{"company", "outlet", "customer_name", "customer_phone", "date", "time", "pax", "items", "total", "paid", "remaining"},
		Template: `✅ *Reservasi Terkonfirmasi — {outlet}*
{customer_name} ({customer_phone})
{date} pukul {time} · {pax} orang
Sudah dibayar: {paid} · Sisa: {remaining}

Pesanan:
{items}`,
	},
	{
		Key: "reservation.cancelled", Label: "Reservasi dibatalkan", Group: "Reservasi", Audience: "internal",
		Permissions:  []string{"reservations.view"},
		Description:  "Reservasi dibatalkan admin, termasuk keputusan uang muka (refund/hangus).",
		Placeholders: []string{"company", "outlet", "customer_name", "customer_phone", "date", "time", "paid", "disposition"},
		Template: `❌ *Reservasi Dibatalkan — {outlet}*
{customer_name} ({customer_phone}) · {date} pukul {time}
Uang masuk: {paid} — {disposition}`,
	},
	// ── Reservasi (ke pelanggan) ──
	{
		Key: "customer.reservation_created", Label: "Ke pelanggan: reservasi diterima + cara bayar DP", Group: "Pelanggan", Audience: "customer",
		Description:  "Dikirim ke nomor pelanggan begitu reservasi tercatat. Berisi DP, rekening, dan tautan status.",
		Placeholders: []string{"company", "outlet", "customer_name", "date", "time", "pax", "items", "total", "down_payment", "bank_accounts", "status_url"},
		Template: `Halo {customer_name}, terima kasih sudah melakukan reservasi di *{outlet}* 🙏

📅 {date} pukul {time} · {pax} orang
Pesanan:
{items}
Total: *{total}*

Untuk mengunci reservasi, mohon transfer uang muka *{down_payment}* ke:
{bank_accounts}

Lalu unggah bukti transfer dan cek status di:
{status_url}

_Pesan otomatis dari {company}._`,
	},
	{
		Key: "customer.reservation_confirmed", Label: "Ke pelanggan: DP diterima, reservasi terkonfirmasi", Group: "Pelanggan", Audience: "customer",
		Description:  "Dikirim saat pembayaran tervalidasi dan reservasi berubah menjadi Dikonfirmasi.",
		Placeholders: []string{"company", "outlet", "customer_name", "date", "time", "pax", "paid", "remaining", "status_url"},
		Template: `Halo {customer_name}, pembayaran Anda sebesar *{paid}* sudah kami terima ✅

Reservasi di *{outlet}* pada {date} pukul {time} untuk {pax} orang *sudah terkonfirmasi*.
Sisa yang dibayar saat kedatangan: {remaining}

Sampai jumpa! 🙏
_{company}_`,
	},
	{
		Key: "customer.payment_rejected", Label: "Ke pelanggan: bukti pembayaran ditolak", Group: "Pelanggan", Audience: "customer",
		Description:  "Admin menolak bukti transfer; pelanggan diberi tahu alasannya.",
		Placeholders: []string{"company", "outlet", "customer_name", "amount", "reason", "status_url"},
		Template: `Halo {customer_name}, mohon maaf bukti pembayaran sebesar {amount} untuk reservasi di *{outlet}* belum bisa kami terima.

Alasan: {reason}

Silakan kirim ulang bukti yang benar melalui:
{status_url}

_{company}_`,
	},
	{
		Key: "customer.reservation_reminder", Label: "Ke pelanggan: pengingat H-1", Group: "Pelanggan", Audience: "customer", Scheduled: true,
		Description:  "Dikirim otomatis sehari sebelum kedatangan (jam pengingat di pengaturan) untuk reservasi terkonfirmasi.",
		Placeholders: []string{"company", "outlet", "customer_name", "date", "time", "pax", "items", "remaining", "outlet_address", "outlet_phone"},
		Template: `Halo {customer_name}, pengingat reservasi Anda besok 😊

📍 *{outlet}*
📅 {date} pukul {time} · {pax} orang
Pesanan:
{items}
Sisa pembayaran saat datang: {remaining}

{outlet_address}
Ada perubahan? Hubungi kami di {outlet_phone}.
_{company}_`,
	},
	{
		Key: "customer.reservation_cancelled", Label: "Ke pelanggan: reservasi dibatalkan", Group: "Pelanggan", Audience: "customer",
		Description:  "Dikirim saat admin membatalkan reservasi.",
		Placeholders: []string{"company", "outlet", "customer_name", "date", "time", "paid", "disposition", "outlet_phone"},
		Template: `Halo {customer_name}, reservasi Anda di *{outlet}* pada {date} pukul {time} telah *dibatalkan*.
{disposition}

Bila ada pertanyaan, hubungi kami di {outlet_phone}.
_{company}_`,
	},
	// ── Kasir & penjualan ──
	{
		Key: "cashier_shift.closed", Label: "Laporan tutup kasir", Group: "Penjualan", Audience: "internal",
		Permissions:  []string{"cashier_shifts.view"},
		Description:  "Setiap shift kasir ditutup di app POS dan tersinkron ke cloud.",
		Placeholders: []string{"company", "outlet", "cashier", "closed_at", "sales_total", "sales_count", "methods", "opening_cash", "cash_in", "cash_out", "expected_cash", "closing_cash", "difference", "notes"},
		Template: `🧾 *Tutup Kasir — {outlet}*
Kasir: {cashier} · {closed_at}

Penjualan: *{sales_total}* ({sales_count} transaksi)
{methods}

Kas awal: {opening_cash}
Kas masuk: {cash_in} · Kas keluar: {cash_out}
Kas seharusnya: {expected_cash}
Kas fisik: {closing_cash}
Selisih: *{difference}*

{notes}`,
	},
	{
		Key: "sales.daily", Label: "Rekap penjualan harian", Group: "Penjualan", Audience: "internal", Scheduled: true,
		Permissions:  []string{"reports.sales.view"},
		Description:  "Dikirim otomatis setiap hari pada jam rekap. Penerima dengan filter outlet hanya menerima outletnya.",
		Placeholders: []string{"company", "date", "outlets", "total_revenue", "total_count", "avg", "unpaid_count", "unpaid_amount", "void_count"},
		Template: `📊 *Rekap Penjualan {date}*
{outlets}

Total: *{total_revenue}* · {total_count} transaksi · rata-rata {avg}
Belum dibayar: {unpaid_count} order ({unpaid_amount})
Void: {void_count} transaksi

_{company}_`,
	},
	// ── Pengadaan ──
	{
		Key: "purchase.submitted", Label: "Pengajuan pengadaan baru", Group: "Pengadaan", Audience: "internal",
		Permissions:  []string{"procurement.requests.approve"},
		Description:  "Pengajuan baru menunggu persetujuan.",
		Placeholders: []string{"company", "number", "type", "requested_by", "outlet", "work_unit", "project", "vendor", "total", "items", "notes"},
		Template: `📝 *Pengajuan Baru {number}*
Jenis: {type} · Oleh: {requested_by}
Unit: {outlet} {work_unit} {project}
Perkiraan: *{total}*

{items}

Catatan: {notes}
Menunggu persetujuan.`,
	},
	{
		Key: "purchase.approved", Label: "Pengajuan disetujui", Group: "Pengadaan", Audience: "internal",
		Permissions: []string{"procurement.requests.purchasing"}, Extra: "pengaju",
		Placeholders: []string{"company", "number", "type", "requested_by", "outlet", "work_unit", "project", "vendor", "total", "actor"},
		Template: `✅ *Pengajuan {number} disetujui* oleh {actor}
{type} · {requested_by} · {outlet} {work_unit}
Nilai: {total}. Lanjut ke pembelian.`,
	},
	{
		Key: "purchase.rejected", Label: "Pengajuan ditolak", Group: "Pengadaan", Audience: "internal",
		Permissions: []string{}, Extra: "pengaju",
		Placeholders: []string{"company", "number", "type", "requested_by", "outlet", "work_unit", "total", "actor", "reason"},
		Template: `⛔ *Pengajuan {number} ditolak* oleh {actor}
{type} · {requested_by} · {outlet} {work_unit}
Alasan: {reason}`,
	},
	{
		Key: "purchase.payment_requested", Label: "Pengajuan pembayaran ke keuangan", Group: "Pengadaan", Audience: "internal",
		Permissions:  []string{"finance.payments.pay"},
		Placeholders: []string{"company", "number", "type", "requested_by", "outlet", "work_unit", "vendor", "total", "actor"},
		Template: `💰 *Permintaan Pembayaran {number}*
Vendor: {vendor}
Tagihan: *{total}* · {outlet} {work_unit}
Diajukan oleh {actor}. Mohon diproses di menu Pembayaran.`,
	},
	{
		Key: "purchase.paid", Label: "Pembayaran pengadaan dicairkan", Group: "Pengadaan", Audience: "internal",
		Permissions: []string{"procurement.requests.purchasing"}, Extra: "pengaju",
		Placeholders: []string{"company", "number", "vendor", "outlet", "work_unit", "amount", "total", "paid_total", "actor"},
		Template: `🏦 *Pembayaran {number}*
Vendor: {vendor} · {outlet} {work_unit}
Dibayar: *{amount}* oleh {actor}
Total dibayar: {paid_total} dari {total}`,
	},
	// ── Gudang / PPIC ──
	{
		Key: "ppic.alert", Label: "Peringatan stok (kedaluwarsa & di bawah ROP)", Group: "Gudang", Audience: "internal", Scheduled: true,
		Permissions:  []string{"ppic.dashboard.view", "ppic.expiry.view", "warehouse_dashboard.view"},
		Description:  "Evaluasi harian PPIC (sekitar pukul 02:00) dan saat aplikasi dinyalakan.",
		Placeholders: []string{"company", "date", "expired_count", "expiring_count", "rop_count", "expiring_list", "rop_list"},
		Template: `⚠️ *Peringatan Stok {date}*
Batch kedaluwarsa: *{expired_count}* · ≤3 hari: *{expiring_count}* · Di bawah ROP: *{rop_count}*

Segera kedaluwarsa:
{expiring_list}

Di bawah titik pesan ulang:
{rop_list}`,
	},
	// ── Aset ──
	{
		Key: "asset.maintenance_due", Label: "Work order perawatan aset terbit", Group: "Aset", Audience: "internal", Scheduled: true,
		Permissions:  []string{"assets.maintenance.view"},
		Description:  "Penjadwal harian menerbitkan WO untuk perawatan yang jatuh tempo ≤7 hari.",
		Placeholders: []string{"company", "date", "count", "list"},
		Template: `🔧 *{count} Work Order Perawatan Terbit* ({date})
{list}

Cek menu Aset → Perawatan.`,
	},
	// ── Perangkat ──
	{
		Key: "device.offline", Label: "Perangkat kasir tidak melapor", Group: "Perangkat", Audience: "internal", Scheduled: true,
		Permissions:  []string{"devices.view"},
		Description:  "Tablet kasir tidak mengirim heartbeat melebihi ambang menit di pengaturan (satu peringatan per outlet per hari).",
		Placeholders: []string{"company", "outlet", "last_seen", "minutes", "pending_sync", "battery"},
		Template: `📴 *Perangkat kasir {outlet} tidak melapor*
Terakhir terlihat: {last_seen} ({minutes} menit lalu)
Antrean sync tertahan: {pending_sync} · Baterai terakhir: {battery}

Periksa koneksi internet / aplikasi POS di outlet.`,
	},
}

func waSpec(key string) *waEventSpec {
	for i := range waEventSpecs {
		if waEventSpecs[i].Key == key {
			return &waEventSpecs[i]
		}
	}
	return nil
}

// WAEventCatalog: katalog untuk layar pengaturan, dengan template & status aktif terkini.
func WAEventCatalog(s models.WASettings) []models.WAEventDef {
	out := make([]models.WAEventDef, 0, len(waEventSpecs))
	for _, spec := range waEventSpecs {
		tpl := s.Templates[spec.Key]
		if strings.TrimSpace(tpl) == "" {
			tpl = spec.Template
		}
		out = append(out, models.WAEventDef{
			Key: spec.Key, Label: spec.Label, Group: spec.Group, Audience: spec.Audience,
			Description: spec.Description, Placeholders: spec.Placeholders,
			Default: spec.Template, Template: tpl, Enabled: waEventEnabled(s, spec.Key), Scheduled: spec.Scheduled,
			Permissions: spec.Permissions, Roles: s.EventRoles[spec.Key], RouteDesc: waRouteDesc(&spec),
		})
	}
	return out
}

// ── Rendering ───────────────────────────────────────────────────────────────

var waPlaceholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

func waRender(tpl string, vars map[string]string) string {
	out := waPlaceholderRe.ReplaceAllStringFunc(tpl, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return v
		}
		return ""
	})
	// Rapikan baris kosong beruntun yang muncul dari placeholder kosong.
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}

func waGlobals() map[string]string {
	name, _ := GetSetting("company_name")
	if strings.TrimSpace(name) == "" {
		name = "Cloud POS"
	}
	now := time.Now().In(GetTimezoneLocation())
	return map[string]string{
		"company": strings.TrimSpace(name),
		"now":     now.Format("02-01-2006 15:04"),
	}
}

func waMerge(base map[string]string, vars map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range vars {
		out[k] = v
	}
	return out
}

// WARenderPreview merender template dengan data contoh (untuk tombol Pratinjau).
func WARenderPreview(event, tpl string) string {
	spec := waSpec(event)
	if spec == nil {
		return ""
	}
	if strings.TrimSpace(tpl) == "" {
		tpl = spec.Template
	}
	return waRender(tpl, waMerge(waGlobals(), waSampleVars(spec)))
}

func waSampleVars(spec *waEventSpec) map[string]string {
	sample := map[string]string{
		"outlet": "Outlet Contoh", "outlet_address": "Jl. Contoh No. 1", "outlet_phone": "0812-0000-0000",
		"customer_name": "Budi", "customer_phone": "0812-3456-7890",
		"date": "Sab, 26 Sep 2026", "time": "19:00", "pax": "4",
		"items": "• 2× Nasi Goreng Spesial\n• 4× Es Teh Manis", "total": "Rp 250.000",
		"down_payment": "Rp 125.000", "paid": "Rp 125.000", "remaining": "Rp 125.000", "amount": "Rp 125.000",
		"payment_type": "DP", "method": "transfer", "proof_url": "https://…/bukti.jpg",
		"bank_accounts": "• BCA 1234567890 a.n. PT Contoh", "status_url": "https://pos.contoh.id/r/outlet?id=…",
		"source": "halaman publik", "notes": "Meja dekat jendela", "reason": "Nominal tidak sesuai",
		"disposition": "Uang muka dikembalikan (refund).",
		"cashier":     "Siti", "closed_at": "21-09-2026 22:05", "sales_total": "Rp 4.250.000", "sales_count": "38",
		"methods":      "• Tunai: Rp 1.500.000 (15 trx)\n• QRIS: Rp 2.750.000 (23 trx)",
		"opening_cash": "Rp 500.000", "cash_in": "Rp 0", "cash_out": "Rp 120.000", "expected_cash": "Rp 1.880.000",
		"closing_cash": "Rp 1.880.000", "difference": "Rp 0",
		"outlets":       "• Outlet A: Rp 4.250.000 (38 trx)\n• Outlet B: Rp 2.100.000 (20 trx)",
		"total_revenue": "Rp 6.350.000", "total_count": "58", "avg": "Rp 109.483",
		"unpaid_count": "1", "unpaid_amount": "Rp 85.000", "void_count": "0",
		"number": "PR-2026-0912", "type": "Barang", "requested_by": "Andi", "work_unit": "· Dapur", "project": "",
		"vendor": "CV Sumber Rejeki", "actor": "manager", "paid_total": "Rp 1.000.000",
		"expired_count": "2", "expiring_count": "3", "rop_count": "5",
		"expiring_list": "• Ayam fillet — Gudang Induk — exp 23-09 (12 kg)", "rop_list": "• Minyak goreng — Outlet A: 4 / min 10 L",
		"count": "2", "list": "• WO-0021 Genset — servis rutin — 25-09-2026",
		"last_seen": "21-09-2026 09:12", "minutes": "45", "pending_sync": "3", "battery": "42%",
	}
	return sample
}

// ── Pemformat ───────────────────────────────────────────────────────────────

func waRp(v float64) string {
	neg := v < 0
	n := int64(math.Round(math.Abs(v)))
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp " + b.String()
	}
	return "Rp " + b.String()
}

var waDays = []string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}
var waMonths = []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// waDate: "2026-09-26" → "Sab, 26 Sep 2026".
func waDate(s string) string {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return s
	}
	return fmt.Sprintf("%s, %d %s %d", waDays[t.Weekday()], t.Day(), waMonths[t.Month()], t.Year())
}

func waItems(items []models.ReservationItem) string {
	if len(items) == 0 {
		return "(tidak ada item)"
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, fmt.Sprintf("• %d× %s", it.Qty, it.ProductName))
	}
	return strings.Join(lines, "\n")
}

func waOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func waMethodLabel(m string) string {
	switch strings.ToLower(m) {
	case "cash":
		return "Tunai"
	case "qris":
		return "QRIS"
	case "card":
		return "Kartu"
	case "transfer":
		return "Transfer"
	case "":
		return "Lainnya"
	}
	return waTitle(m)
}

// waTitle: huruf pertama kapital (pengganti strings.Title yang usang).
func waTitle(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ── Inti pengiriman notifikasi ──────────────────────────────────────────────

// notifyWA merender template event dan memasukkan pesan ke antrean: ke nomor
// pelanggan (audience customer) atau ke semua penerima internal yang
// berlangganan event ini (dan cocok filter outlet-nya).
// extraNames: nama akun yang ikut dikirimi di luar aturan posisi (mis. pengaju).
func notifyWA(event, refID, outletID string, vars map[string]string, customerPhone string, extraNames ...string) {
	if waGatewayURL == "" {
		return
	}
	spec := waSpec(event)
	if spec == nil {
		return
	}
	s := GetWASettings()
	if !s.Enabled || !waEventEnabled(s, event) {
		return
	}
	tpl := s.Templates[event]
	if strings.TrimSpace(tpl) == "" {
		tpl = spec.Template
	}
	body := waRender(tpl, waMerge(waGlobals(), vars))
	if body == "" {
		return
	}
	if spec.Audience == "customer" {
		target := WANormalizeTarget(customerPhone)
		if target == "" {
			return
		}
		if _, err := EnqueueWAMessage(models.WAMessage{
			Target: target, TargetName: vars["customer_name"], Audience: "customer", Kind: "notify",
			Event: event, RefID: refID, Body: body,
		}); err != nil {
			log.Printf("[WA] antre %s: %v", event, err)
		}
		return
	}
	for _, r := range waRecipientsFor(event, outletID, extraNames...) {
		if _, err := EnqueueWAMessage(models.WAMessage{
			Target: r.Target, TargetName: r.Name, Audience: "internal", Kind: "notify",
			Event: event, RefID: refID, Body: body,
		}); err != nil {
			log.Printf("[WA] antre %s → %s: %v", event, r.Name, err)
		}
	}
}

// ── Reservasi ───────────────────────────────────────────────────────────────

type waOutletInfo struct{ Name, Slug, Address, Phone string }

func waOutlet(id string) waOutletInfo {
	var o waOutletInfo
	database.DB.QueryRow(`SELECT COALESCE(name,''), COALESCE(slug,''), COALESCE(address,''), COALESCE(phone,'')
		FROM outlets WHERE id = $1`, id).Scan(&o.Name, &o.Slug, &o.Address, &o.Phone)
	return o
}

func waStatusURL(s models.WASettings, o waOutletInfo, reservationID string) string {
	if o.Slug == "" {
		return ""
	}
	return fmt.Sprintf("%s/r/%s?id=%s", s.PublicBaseURL, o.Slug, strings.TrimSpace(reservationID))
}

func waBankAccounts() string {
	list := activeBankAccounts()
	if len(list) == 0 {
		return "(rekening belum diatur — hubungi kami)"
	}
	lines := make([]string, 0, len(list))
	for _, b := range list {
		lines = append(lines, fmt.Sprintf("• %s %s a.n. %s", b.BankName, b.AccountNumber, b.AccountHolder))
	}
	return strings.Join(lines, "\n")
}

func waReservationVars(r *models.Reservation, o waOutletInfo, s models.WASettings) map[string]string {
	source := "admin"
	if r.Source == "public" {
		source = "halaman publik"
	}
	return map[string]string{
		"outlet": waOr(o.Name, r.OutletName), "outlet_address": o.Address, "outlet_phone": waOr(o.Phone, "-"),
		"customer_name": waOr(r.CustomerName, "Pelanggan"), "customer_phone": waOr(r.CustomerPhone, "-"),
		"date": waDate(r.ReservationDate), "time": waOr(r.ReservationTime, "-"), "pax": fmt.Sprintf("%d", r.Pax),
		"items": waItems(r.Items), "total": waRp(r.Total), "down_payment": waRp(r.DownPayment),
		"paid": waRp(r.PaidAmount), "remaining": waRp(r.Remaining), "source": source,
		"notes": waOr(r.Notes, "-"), "status_url": waStatusURL(s, o, r.ID), "bank_accounts": waBankAccounts(),
	}
}

func NotifyReservationCreated(r *models.Reservation) {
	if r == nil {
		return
	}
	s := GetWASettings()
	o := waOutlet(r.OutletID)
	vars := waReservationVars(r, o, s)
	notifyWA("reservation.created", r.ID, r.OutletID, vars, "")
	if r.DownPayment > 0 {
		notifyWA("customer.reservation_created", r.ID, r.OutletID, vars, r.CustomerPhone)
	} else {
		// Tanpa DP: pesan "diterima" tetap berguna, tapi tanpa instruksi transfer.
		vars["down_payment"] = "Rp 0 (tidak ada uang muka)"
		notifyWA("customer.reservation_created", r.ID, r.OutletID, vars, r.CustomerPhone)
	}
}

func NotifyReservationPaymentSubmitted(r *models.Reservation, p *models.ReservationPayment) {
	if r == nil || p == nil {
		return
	}
	s := GetWASettings()
	o := waOutlet(r.OutletID)
	vars := waReservationVars(r, o, s)
	vars["payment_type"] = strings.ToUpper(p.Type)
	vars["amount"] = waRp(p.Amount)
	vars["method"] = p.Method
	proof := ""
	if p.ProofURL != "" {
		proof = "Bukti: " + s.PublicBaseURL + "/" + strings.TrimPrefix(p.ProofURL, "/")
	}
	vars["proof_url"] = proof
	notifyWA("reservation.payment_submitted", p.ID, r.OutletID, vars, "")
}

func NotifyReservationConfirmed(r *models.Reservation) {
	if r == nil {
		return
	}
	s := GetWASettings()
	o := waOutlet(r.OutletID)
	vars := waReservationVars(r, o, s)
	notifyWA("reservation.confirmed", r.ID, r.OutletID, vars, "")
	notifyWA("customer.reservation_confirmed", r.ID, r.OutletID, vars, r.CustomerPhone)
}

func NotifyReservationCancelled(r *models.Reservation) {
	if r == nil {
		return
	}
	s := GetWASettings()
	o := waOutlet(r.OutletID)
	vars := waReservationVars(r, o, s)
	switch {
	case r.PaidAmount > 0 && r.CancelDisposition == "refund":
		vars["disposition"] = "Uang muka " + waRp(r.PaidAmount) + " akan dikembalikan (refund)."
	case r.PaidAmount > 0 && r.CancelDisposition == "hangus":
		vars["disposition"] = "Uang muka " + waRp(r.PaidAmount) + " hangus sesuai ketentuan."
	default:
		vars["disposition"] = "Tidak ada uang muka yang perlu dikembalikan."
	}
	notifyWA("reservation.cancelled", r.ID, r.OutletID, vars, "")
	notifyWA("customer.reservation_cancelled", r.ID, r.OutletID, vars, r.CustomerPhone)
}

func NotifyReservationPaymentRejected(r *models.Reservation, p *models.ReservationPayment) {
	if r == nil || p == nil {
		return
	}
	s := GetWASettings()
	o := waOutlet(r.OutletID)
	vars := waReservationVars(r, o, s)
	vars["amount"] = waRp(p.Amount)
	vars["reason"] = waOr(p.RejectedReason, "-")
	notifyWA("customer.payment_rejected", p.ID, r.OutletID, vars, r.CustomerPhone)
}

// SendReservationReminders: pengingat H-1 untuk reservasi terkonfirmasi.
func SendReservationReminders(now time.Time) {
	tomorrow := now.Add(24 * time.Hour).Format("2006-01-02")
	rows, err := database.DB.Query(`SELECT id FROM reservations
		WHERE status = 'confirmed' AND reservation_date = $1::date AND COALESCE(customer_phone,'') <> ''`, tomorrow)
	if err != nil {
		log.Printf("[WA] pengingat reservasi: %v", err)
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	rows.Close()
	s := GetWASettings()
	for _, id := range ids {
		r, err := GetReservation(id, nil)
		if err != nil {
			continue
		}
		o := waOutlet(r.OutletID)
		vars := waReservationVars(r, o, s)
		notifyWA("customer.reservation_reminder", r.ID, r.OutletID, vars, r.CustomerPhone)
	}
	if len(ids) > 0 {
		log.Printf("[WA] %d pengingat reservasi H-1 diantrekan untuk %s", len(ids), tomorrow)
	}
}

// ── Tutup kasir ─────────────────────────────────────────────────────────────

func NotifyCashierShiftClosed(outletID, shiftID string, req models.PushCashierShiftRequest) {
	o := waOutlet(outletID)
	var rj shiftReportJSON
	if req.Report != nil {
		if b, err := json.Marshal(req.Report); err == nil {
			json.Unmarshal(b, &rj)
		}
	}
	methods := []string{}
	keys := make([]string, 0, len(rj.ByMethod))
	for k := range rj.ByMethod {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := rj.ByMethod[k]
		methods = append(methods, fmt.Sprintf("• %s: %s (%d trx)", waMethodLabel(k), waRp(v.Total), v.Count))
	}
	methodsText := strings.Join(methods, "\n")
	if methodsText == "" {
		methodsText = "(rincian per metode belum tersinkron)"
	}
	diff := req.ClosingCash - rj.ExpectedCash
	diffText := waRp(diff)
	if rj.ExpectedCash == 0 && rj.SalesTotal == 0 {
		diffText = "(laporan rinci belum tersinkron)"
	} else if diff > 0 {
		diffText += " (lebih)"
	} else if diff < 0 {
		diffText += " (kurang)"
	}
	cashier := waOr(req.ClosedBy, req.OpenedBy)
	vars := map[string]string{
		"outlet": waOr(o.Name, outletID), "cashier": waOr(cashier, "-"),
		"closed_at":   time.Now().In(GetTimezoneLocation()).Format("02-01-2006 15:04"),
		"sales_total": waRp(rj.SalesTotal), "sales_count": fmt.Sprintf("%d", rj.SalesCount),
		"methods": methodsText, "opening_cash": waRp(req.OpeningCash),
		"cash_in": waRp(rj.CashInTotal), "cash_out": waRp(rj.CashOutTotal),
		"expected_cash": waRp(rj.ExpectedCash), "closing_cash": waRp(req.ClosingCash),
		"difference": diffText, "notes": waOr(req.Notes, ""),
	}
	notifyWA("cashier_shift.closed", shiftID, outletID, vars, "")
}

// ── Rekap penjualan harian ──────────────────────────────────────────────────

type waOutletSales struct {
	ID, Name     string
	Count        int
	Revenue      float64
	UnpaidCount  int
	UnpaidAmount float64
	VoidCount    int
}

func waDailySales(day string) []waOutletSales {
	rows, err := database.DB.Query(`
		SELECT o.id, o.name,
			(SELECT COUNT(*) FROM cloud_transactions t WHERE t.outlet_id = o.id
				AND t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($1::date + 1)`+txNotVoided("t")+`),
			(SELECT COALESCE(SUM(t.total_amount),0) FROM cloud_transactions t WHERE t.outlet_id = o.id
				AND t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($1::date + 1)`+txNotVoided("t")+`),
			(SELECT COUNT(*) FROM cloud_orders u WHERE u.outlet_id = o.id
				AND COALESCE(u.payment_info->>'payment_status','unpaid') NOT IN ('paid')
				AND NULLIF(u.payment_info->>'voided_at','') IS NULL AND COALESCE(u.is_holding,false) = false
				AND u.created_at >= tz_day_start($1::date) AND u.created_at < tz_day_start($1::date + 1)),
			(SELECT COALESCE(SUM(u.total_amount),0) FROM cloud_orders u WHERE u.outlet_id = o.id
				AND COALESCE(u.payment_info->>'payment_status','unpaid') NOT IN ('paid')
				AND NULLIF(u.payment_info->>'voided_at','') IS NULL AND COALESCE(u.is_holding,false) = false
				AND u.created_at >= tz_day_start($1::date) AND u.created_at < tz_day_start($1::date + 1)),
			(SELECT COUNT(*) FROM cloud_transactions t JOIN cloud_orders vo ON vo.id = t.order_id
				WHERE t.outlet_id = o.id AND NULLIF(vo.payment_info->>'voided_at','') IS NOT NULL
				AND t.created_at >= tz_day_start($1::date) AND t.created_at < tz_day_start($1::date + 1))
		FROM outlets o WHERE o.is_active = true ORDER BY o.name`, day)
	if err != nil {
		log.Printf("[WA] rekap harian: %v", err)
		return nil
	}
	defer rows.Close()
	out := []waOutletSales{}
	for rows.Next() {
		var s waOutletSales
		if err := rows.Scan(&s.ID, &s.Name, &s.Count, &s.Revenue, &s.UnpaidCount, &s.UnpaidAmount, &s.VoidCount); err != nil {
			continue
		}
		s.ID = strings.TrimSpace(s.ID)
		out = append(out, s)
	}
	return out
}

// SendDailySalesSummary menyusun rekap per penerima: yang punya filter outlet
// hanya melihat outletnya, yang tidak melihat semua.
func SendDailySalesSummary(day string) {
	if waGatewayURL == "" {
		return
	}
	s := GetWASettings()
	if !s.Enabled || !waEventEnabled(s, "sales.daily") {
		return
	}
	all := waDailySales(day)
	if len(all) == 0 {
		return
	}
	tpl := s.Templates["sales.daily"]
	if strings.TrimSpace(tpl) == "" {
		tpl = waSpec("sales.daily").Template
	}
	globals := waGlobals()
	for _, r := range waRecipientsFor("sales.daily", "") {
		rows := all
		if len(r.OutletIDs) > 0 {
			rows = []waOutletSales{}
			for _, o := range all {
				for _, id := range r.OutletIDs {
					if strings.TrimSpace(id) == o.ID {
						rows = append(rows, o)
					}
				}
			}
		}
		if len(rows) == 0 {
			continue
		}
		var rev, unpaidAmt float64
		var cnt, unpaidCnt, voidCnt int
		lines := make([]string, 0, len(rows))
		for _, o := range rows {
			rev += o.Revenue
			cnt += o.Count
			unpaidCnt += o.UnpaidCount
			unpaidAmt += o.UnpaidAmount
			voidCnt += o.VoidCount
			lines = append(lines, fmt.Sprintf("• %s: %s (%d trx)", o.Name, waRp(o.Revenue), o.Count))
		}
		avg := 0.0
		if cnt > 0 {
			avg = rev / float64(cnt)
		}
		vars := waMerge(globals, map[string]string{
			"date": waDate(day), "outlets": strings.Join(lines, "\n"),
			"total_revenue": waRp(rev), "total_count": fmt.Sprintf("%d", cnt), "avg": waRp(avg),
			"unpaid_count": fmt.Sprintf("%d", unpaidCnt), "unpaid_amount": waRp(unpaidAmt),
			"void_count": fmt.Sprintf("%d", voidCnt),
		})
		EnqueueWAMessage(models.WAMessage{
			Target: r.Target, TargetName: r.Name, Audience: "internal", Kind: "notify",
			Event: "sales.daily", RefID: day, Body: waRender(tpl, vars),
		})
	}
}

// ── Pengadaan ───────────────────────────────────────────────────────────────

func waPurchaseVars(pr *models.PurchaseRequest) map[string]string {
	outlet := ""
	if pr.OutletName != nil {
		outlet = *pr.OutletName
	}
	wu := ""
	if pr.WorkUnitName != "" {
		wu = "· " + pr.WorkUnitName
	}
	project := ""
	if pr.ProjectName != "" {
		project = "· Projek " + pr.ProjectName
	}
	total := pr.TotalFinal
	if total <= 0 {
		total = pr.TotalHps
	}
	if total <= 0 {
		total = pr.TotalAmount
	}
	lines := []string{}
	for _, grp := range pr.Items {
		for _, it := range grp.Items {
			if len(lines) >= 12 {
				break
			}
			lines = append(lines, fmt.Sprintf("• %s ×%d %s", it.Name, it.Qty, it.Unit))
		}
	}
	if len(lines) == 0 {
		for _, grp := range pr.Items {
			lines = append(lines, "• "+grp.Name)
		}
	}
	return map[string]string{
		"number": pr.RequestNumber, "type": waTitle(waOr(pr.RequestType, "-")),
		"requested_by": waOr(pr.RequestedBy, "-"), "outlet": waOr(outlet, "Pusat"), "work_unit": wu, "project": project,
		"vendor": waOr(pr.VendorName, "-"), "total": waRp(total), "items": strings.Join(lines, "\n"),
		"notes": waOr(pr.Notes, "-"), "paid_total": waRp(pr.PaidAmount),
	}
}

// NotifyPurchaseEvent: action = submitted | approve | reject | request_payment | pay.
func NotifyPurchaseEvent(pr *models.PurchaseRequest, action, actor, reason string, amount float64) {
	if pr == nil {
		return
	}
	event := map[string]string{
		"submitted": "purchase.submitted", "approve": "purchase.approved", "reject": "purchase.rejected",
		"request_payment": "purchase.payment_requested", "pay": "purchase.paid",
	}[action]
	if event == "" {
		return
	}
	vars := waPurchaseVars(pr)
	vars["actor"] = waOr(actor, "-")
	vars["reason"] = waOr(reason, "-")
	vars["amount"] = waRp(amount)
	outletID := ""
	if pr.OutletID != nil {
		outletID = strings.TrimSpace(*pr.OutletID)
	}
	ref := pr.ID + ":" + action
	if action == "pay" {
		ref += ":" + fmt.Sprintf("%.0f", pr.PaidAmount)
	}
	// Pengaju ikut diberi tahu nasib pengajuannya (disetujui/ditolak/dibayar),
	// dicocokkan dengan nama akun; tidak untuk "submitted" (ia yang membuatnya).
	extra := ""
	if action != "submitted" {
		extra = pr.RequestedBy
	}
	notifyWA(event, ref, outletID, vars, "", extra)
}

// ── PPIC ────────────────────────────────────────────────────────────────────

func NotifyPpicAlert(expired, expiring, belowRop int) {
	if waGatewayURL == "" {
		return
	}
	day := waToday()
	expLines := []string{}
	rows, err := database.DB.Query(`
		SELECT si.name, w.name, TO_CHAR(sb.expiry_date,'DD-MM'), sb.qty_base, si.base_unit
		FROM stock_batches sb JOIN stock_items si ON si.id = sb.item_id JOIN warehouses w ON w.id = sb.warehouse_id
		WHERE sb.qty_base > 0 AND sb.expiry_date IS NOT NULL AND sb.expiry_date < CURRENT_DATE + 3
		  AND sb.ppic_ack_at IS NULL AND w.is_active = true
		ORDER BY sb.expiry_date LIMIT 10`)
	if err == nil {
		for rows.Next() {
			var item, wh, exp, unit string
			var qty float64
			if rows.Scan(&item, &wh, &exp, &qty, &unit) == nil {
				expLines = append(expLines, fmt.Sprintf("• %s — %s — exp %s (%.0f %s)", item, wh, exp, qty, unit))
			}
		}
		rows.Close()
	}
	ropLines := []string{}
	rows, err = database.DB.Query(`
		SELECT si.name, w.name, sl.qty_base, COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock), si.base_unit
		FROM stock_ledger sl
		JOIN stock_items si ON si.id = sl.item_id AND si.is_active = true
		JOIN warehouses w ON w.id = sl.warehouse_id
		LEFT JOIN item_planning_params pp ON pp.item_id = sl.item_id AND pp.warehouse_id = sl.warehouse_id
		WHERE COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock) > 0
		  AND sl.qty_base <= COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock) AND w.is_active = true
		ORDER BY (sl.qty_base / NULLIF(COALESCE(NULLIF(pp.reorder_point, 0), sl.min_stock),0)) ASC LIMIT 10`)
	if err == nil {
		for rows.Next() {
			var item, wh, unit string
			var qty, rop float64
			if rows.Scan(&item, &wh, &qty, &rop, &unit) == nil {
				ropLines = append(ropLines, fmt.Sprintf("• %s — %s: %.0f / min %.0f %s", item, wh, qty, rop, unit))
			}
		}
		rows.Close()
	}
	if belowRop > 10 {
		ropLines = append(ropLines, fmt.Sprintf("… dan %d item lain", belowRop-10))
	}
	vars := map[string]string{
		"date": waDate(day), "expired_count": fmt.Sprintf("%d", expired), "expiring_count": fmt.Sprintf("%d", expiring),
		"rop_count":     fmt.Sprintf("%d", belowRop),
		"expiring_list": waOr(strings.Join(expLines, "\n"), "(tidak ada)"),
		"rop_list":      waOr(strings.Join(ropLines, "\n"), "(tidak ada)"),
	}
	notifyWA("ppic.alert", day, "", vars, "")
}

// ── Aset ────────────────────────────────────────────────────────────────────

type WAMaintenanceDue struct{ WONumber, AssetName, Type, DueDate string }

func NotifyMaintenanceDue(list []WAMaintenanceDue) {
	if len(list) == 0 {
		return
	}
	lines := make([]string, 0, len(list))
	for _, d := range list {
		lines = append(lines, fmt.Sprintf("• %s %s — %s — %s", d.WONumber, d.AssetName, d.Type, waDate(d.DueDate)))
	}
	day := waToday()
	vars := map[string]string{
		"date": waDate(day), "count": fmt.Sprintf("%d", len(list)), "list": strings.Join(lines, "\n"),
	}
	notifyWA("asset.maintenance_due", day, "", vars, "")
}

// ── Perangkat ───────────────────────────────────────────────────────────────

// CheckDeviceOffline: outlet aktif yang tablet kasirnya berhenti melapor
// lebih lama dari ambang. Satu peringatan per outlet per hari (ref = outlet:tanggal).
func CheckDeviceOffline(s models.WASettings) {
	if waGatewayURL == "" || !waEventEnabled(s, "device.offline") {
		return
	}
	rows, err := database.DB.Query(fmt.Sprintf(`
		SELECT o.id, o.name,
			COALESCE(to_char((h.received_at AT TIME ZONE 'UTC') AT TIME ZONE %s, 'DD-MM-YYYY HH24:MI'), ''),
			COALESCE(EXTRACT(EPOCH FROM ((now() AT TIME ZONE 'UTC') - h.received_at))/60, 0)::int,
			COALESCE(h.pending_sync, 0), COALESCE(h.battery, -1)
		FROM outlets o JOIN device_heartbeats h ON h.outlet_id = o.id
		WHERE o.is_active = true AND h.received_at < (now() AT TIME ZONE 'UTC') - ($1 * interval '1 minute')`, tzExpr),
		s.DeviceOfflineMin)
	if err != nil {
		return
	}
	defer rows.Close()
	day := waToday()
	for rows.Next() {
		var id, name, lastSeen string
		var minutes, pending, battery int
		if rows.Scan(&id, &name, &lastSeen, &minutes, &pending, &battery) != nil {
			continue
		}
		id = strings.TrimSpace(id)
		bat := "-"
		if battery >= 0 {
			bat = fmt.Sprintf("%d%%", battery)
		}
		vars := map[string]string{
			"outlet": name, "last_seen": lastSeen, "minutes": fmt.Sprintf("%d", minutes),
			"pending_sync": fmt.Sprintf("%d", pending), "battery": bat,
		}
		notifyWA("device.offline", id+":"+day, id, vars, "")
	}
}
