// Kosakata bersama modul Perlengkapan/Aset.
//
// Kondisi dan status adalah dua sumbu berbeda (lihat docs/perlengkapan-aset.md §3.1):
// kondisi = bagaimana barangnya, status = di mana posisinya dalam siklus hidup.
// Sebelum Fase 1 keduanya bercampur, sehingga 'perbaikan' muncul sebagai kondisi.

export const CONDITIONS = {
  baik: 'Baik',
  rusak_ringan: 'Rusak Ringan',
  rusak_berat: 'Rusak Berat',
}

export const STATUSES = {
  aktif: 'Aktif',
  dipinjam: 'Dipinjam',
  perbaikan: 'Dalam Perbaikan',
  transit: 'Dalam Perjalanan',
  tidak_aktif: 'Tidak Dipakai',
  dihapus: 'Dihapus',
}

// Status yang hanya boleh berubah lewat dokumen — form edit tidak menawarkannya.
// Server menolaknya juga; ini sekadar agar pengguna tidak mencoba.
//
// 'perbaikan' termasuk sejak Fase 3: statusnya dinyalakan oleh work order
// perawatan, bukan oleh form ini.
export const SYSTEM_STATUSES = ['transit', 'perbaikan', 'dihapus']

export const EDITABLE_STATUSES = Object.fromEntries(
  Object.entries(STATUSES).filter(([k]) => !SYSTEM_STATUSES.includes(k)),
)

export const TRACKING_MODES = {
  massal: 'Massal (beberapa unit identik)',
  tunggal: 'Tunggal (satu unit bernomor)',
}

export const MTYPES = {
  rutin: 'Rutin',
  perbaikan: 'Perbaikan',
  penggantian: 'Penggantian Part',
  inspeksi: 'Inspeksi',
}

export const MOVEMENT_TYPES = {
  pendataan: 'Pendataan',
  penerimaan: 'Penerimaan',
  mutasi_keluar: 'Mutasi Keluar',
  mutasi_masuk: 'Mutasi Masuk',
  kondisi: 'Perubahan Data',
  perawatan: 'Perawatan',
  opname: 'Opname',
  penghapusan: 'Penghapusan',
  pemecahan: 'Pemecahan',
}

export function condLabel(c) { return CONDITIONS[c] || c || '—' }
export function statusLabel(s) { return STATUSES[s] || s || '—' }

export function condCls(c) {
  if (c === 'baik') return 'badge-ok'
  if (c === 'rusak_ringan') return 'badge-warn'
  if (c === 'rusak_berat') return 'badge-bad'
  return 'badge-mute'
}

export function statusCls(s) {
  if (s === 'aktif') return 'badge-ok'
  if (s === 'transit') return 'badge-info'
  if (s === 'perbaikan' || s === 'dipinjam') return 'badge-warn'
  if (s === 'dihapus') return 'badge-bad'
  return 'badge-mute'
}

/**
 * Cetak label aset (nomor + nama + outlet + QR) lewat jendela terpisah.
 *
 * Sengaja memakai jendela sendiri, bukan @media print pada halaman: aturan cetak
 * aplikasi tidak ikut campur, dan ukuran label bisa dipatok dalam milimeter.
 * QR dibuat lewat import dinamis supaya pustakanya tidak membebani bundel utama.
 */
export async function printAssetLabels(assets, { size = '50x25' } = {}) {
  const list = (Array.isArray(assets) ? assets : [assets]).filter(Boolean)
  if (!list.length) return

  let toDataURL = null
  try {
    const mod = await import('qrcode')
    toDataURL = (mod.default || mod).toDataURL
  } catch {
    toDataURL = null // label tetap tercetak tanpa QR daripada gagal sama sekali
  }

  const dims = size === '70x40' ? { w: 70, h: 40, qr: 26, name: 9 } : { w: 50, h: 25, qr: 17, name: 7.5 }
  const origin = window.location.origin

  const cards = []
  for (const a of list) {
    let qr = ''
    if (toDataURL) {
      try {
        qr = await toDataURL(`${origin}/perlengkapan/${a.id}`, { margin: 0, width: 240 })
      } catch { qr = '' }
    }
    const esc = (v) => String(v ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))
    cards.push(`
      <div class="label">
        ${qr ? `<img class="qr" src="${qr}" alt="" />` : '<div class="qr placeholder"></div>'}
        <div class="meta">
          <div class="no">${esc(a.asset_no || a.code || '—')}</div>
          <div class="name">${esc(a.name)}</div>
          <div class="sub">${esc(a.outlet_name || '')}${a.location ? ' · ' + esc(a.location) : ''}</div>
        </div>
      </div>`)
  }

  const html = `<!doctype html><html><head><meta charset="utf-8"><title>Label Aset</title>
    <style>
      @page { margin: 6mm; }
      * { box-sizing: border-box; }
      body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; }
      .sheet { display: flex; flex-wrap: wrap; gap: 3mm; }
      .label {
        width: ${dims.w}mm; height: ${dims.h}mm; border: 0.3mm solid #111; border-radius: 1.5mm;
        padding: 1.5mm; display: flex; align-items: center; gap: 1.5mm; overflow: hidden;
        page-break-inside: avoid;
      }
      .qr { width: ${dims.qr}mm; height: ${dims.qr}mm; flex: none; }
      .qr.placeholder { border: 0.2mm dashed #999; }
      .meta { min-width: 0; flex: 1; }
      .no { font-family: ui-monospace, monospace; font-size: ${dims.name - 1.5}pt; font-weight: 700; letter-spacing: -0.2pt; }
      .name { font-size: ${dims.name}pt; font-weight: 600; line-height: 1.15; margin-top: 0.6mm;
              overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
      .sub { font-size: ${dims.name - 2}pt; color: #555; margin-top: 0.5mm; overflow-wrap: anywhere; }
      @media screen { body { background: #f3f4f6; padding: 8mm; } .label { background: #fff; } }
    </style></head>
    <body><div class="sheet">${cards.join('')}</div>
    <script>window.onload = function () { window.focus(); window.print(); }<\/script>
    </body></html>`

  const win = window.open('', '_blank', 'width=900,height=650')
  if (!win) return { blocked: true }
  win.document.open()
  win.document.write(html)
  win.document.close()
  return { blocked: false }
}

// ── Mutasi antar outlet ─────────────────────────────────────────────────────

export const TRANSFER_STATUSES = {
  draft: 'Draft',
  pending: 'Menunggu Persetujuan',
  approved: 'Disetujui',
  sent: 'Dalam Perjalanan',
  received: 'Selesai',
  rejected: 'Ditolak',
  cancelled: 'Dibatalkan',
}

export const TRANSFER_REASONS = {
  relokasi: 'Relokasi permanen',
  pinjam: 'Pinjam sementara',
  perbaikan: 'Dikirim untuk diperbaiki',
  penyeimbangan: 'Penyeimbangan antar outlet',
  lainnya: 'Lainnya',
}

export function trStatusLabel(s) { return TRANSFER_STATUSES[s] || s || '—' }

export function trStatusCls(s) {
  if (s === 'received') return 'badge-ok'
  if (s === 'sent') return 'badge-info'
  if (s === 'pending' || s === 'approved') return 'badge-warn'
  if (s === 'rejected' || s === 'cancelled') return 'badge-bad'
  return 'badge-mute'
}

/**
 * Cetak Berita Acara Serah Terima Aset — dokumen yang selama ini dibuat manual
 * di luar sistem. Dicetak dari jendela terpisah dengan alasan yang sama seperti
 * label: aturan cetak aplikasi tidak ikut campur.
 */
export function printTransferNote(t) {
  if (!t) return { blocked: false }
  const esc = (v) => String(v ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]))
  const rows = (t.items || []).map((it, i) => `
    <tr>
      <td class="num">${i + 1}</td>
      <td><strong>${esc(it.asset_name)}</strong><div class="mono">${esc(it.asset_no || '-')}</div></td>
      <td class="num">${it.qty} ${esc(it.unit || '')}</td>
      <td class="num">${it.received_qty ?? '—'}</td>
      <td>${esc(CONDITIONS[it.condition_sent] || it.condition_sent || '—')}</td>
      <td>${esc(CONDITIONS[it.condition_recv] || it.condition_recv || '—')}</td>
    </tr>`).join('')

  const html = `<!doctype html><html><head><meta charset="utf-8"><title>BAST ${esc(t.transfer_number)}</title>
  <style>
    @page { size: A4; margin: 15mm; }
    body { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; font-size: 11pt; color: #111; margin: 0; }
    h1 { font-size: 14pt; text-align: center; margin: 0 0 2mm; text-transform: uppercase; letter-spacing: .5pt; }
    .sub { text-align: center; font-size: 10pt; color: #444; margin-bottom: 8mm; }
    .meta { width: 100%; border-collapse: collapse; margin-bottom: 6mm; font-size: 10pt; }
    .meta td { padding: 1.2mm 0; vertical-align: top; }
    .meta td:first-child { width: 38mm; color: #555; }
    table.items { width: 100%; border-collapse: collapse; font-size: 10pt; }
    table.items th, table.items td { border: .3mm solid #999; padding: 2mm; text-align: left; vertical-align: top; }
    table.items th { background: #f1f1f1; }
    .num { text-align: center; }
    .mono { font-family: ui-monospace, monospace; font-size: 8pt; color: #666; }
    .sign { display: flex; justify-content: space-between; margin-top: 14mm; font-size: 10pt; }
    .sign div { width: 45%; text-align: center; }
    .line { margin-top: 20mm; border-top: .3mm solid #333; padding-top: 1.5mm; }
    .note { margin-top: 6mm; font-size: 9.5pt; color: #444; }
    @media screen { body { background: #f3f4f6; padding: 10mm; } }
  </style></head><body>
  <h1>Berita Acara Serah Terima Aset</h1>
  <div class="sub">Nomor: ${esc(t.transfer_number)}</div>
  <table class="meta">
    <tr><td>Outlet Asal</td><td>: ${esc(t.from_outlet_name)}</td></tr>
    <tr><td>Outlet Tujuan</td><td>: ${esc(t.to_outlet_name)}</td></tr>
    <tr><td>Alasan</td><td>: ${esc(TRANSFER_REASONS[t.reason] || t.reason || '-')}</td></tr>
    ${t.expected_return ? `<tr><td>Rencana Kembali</td><td>: ${esc(t.expected_return)}</td></tr>` : ''}
    <tr><td>Dikirim</td><td>: ${esc(t.sent_by || '-')}${t.sent_at ? ' · ' + esc(t.sent_at) : ''}</td></tr>
    <tr><td>Diterima</td><td>: ${esc(t.received_by || '-')}${t.received_at ? ' · ' + esc(t.received_at) : ''}</td></tr>
  </table>
  <table class="items">
    <thead><tr><th class="num">No</th><th>Aset</th><th class="num">Dikirim</th><th class="num">Diterima</th><th>Kondisi Kirim</th><th>Kondisi Terima</th></tr></thead>
    <tbody>${rows || '<tr><td colspan="6">—</td></tr>'}</tbody>
  </table>
  ${t.notes ? `<div class="note"><strong>Catatan:</strong> ${esc(t.notes)}</div>` : ''}
  <div class="sign">
    <div><div>Yang Menyerahkan</div><div class="line">${esc(t.from_outlet_name)}</div></div>
    <div><div>Yang Menerima</div><div class="line">${esc(t.to_outlet_name)}</div></div>
  </div>
  <script>window.onload = function () { window.focus(); window.print(); }<\/script>
  </body></html>`

  const win = window.open('', '_blank', 'width=900,height=700')
  if (!win) return { blocked: true }
  win.document.open(); win.document.write(html); win.document.close()
  return { blocked: false }
}

// ── Work order perawatan ────────────────────────────────────────────────────

export const WO_STATUSES = {
  dijadwalkan: 'Dijadwalkan',
  berjalan: 'Sedang Dikerjakan',
  selesai: 'Selesai',
  batal: 'Dibatalkan',
}

export function woStatusLabel(s) { return WO_STATUSES[s] || s || '—' }

export function woStatusCls(s) {
  if (s === 'selesai') return 'badge-ok'
  if (s === 'berjalan') return 'badge-info'
  if (s === 'dijadwalkan') return 'badge-warn'
  return 'badge-mute'
}

/**
 * Label jatuh tempo yang menjawab "kapan", bukan sekadar menampilkan tanggal.
 * due_in_days negatif = terlambat.
 */
export function dueLabel(days) {
  if (days === null || days === undefined) return ''
  if (days < 0) return `Terlambat ${Math.abs(days)} hari`
  if (days === 0) return 'Jatuh tempo hari ini'
  if (days === 1) return 'Besok'
  return `${days} hari lagi`
}

export function dueCls(days) {
  if (days === null || days === undefined) return 'badge-mute'
  if (days < 0) return 'badge-bad'
  if (days <= 7) return 'badge-warn'
  return 'badge-mute'
}

// ── Penghapusan & opname ────────────────────────────────────────────────────

export const DISPOSAL_METHODS = {
  dijual: 'Dijual',
  dihibahkan: 'Dihibahkan',
  dimusnahkan: 'Dimusnahkan',
  hilang: 'Hilang',
  tukar_tambah: 'Tukar Tambah',
}

export const DISPOSAL_STATUSES = {
  pending: 'Menunggu Persetujuan',
  approved: 'Disetujui',
  rejected: 'Ditolak',
}

export function disposalStatusCls(s) {
  if (s === 'approved') return 'badge-ok'
  if (s === 'rejected') return 'badge-bad'
  return 'badge-warn'
}

export const OPNAME_STATUSES = {
  berjalan: 'Berjalan',
  selesai: 'Selesai',
  batal: 'Dibatalkan',
}

export function opnameStatusCls(s) {
  if (s === 'selesai') return 'badge-ok'
  if (s === 'batal') return 'badge-mute'
  return 'badge-info'
}

// ── Bukti foto ──────────────────────────────────────────────────────────────

/**
 * Alamat gambar untuk satu bukti foto.
 *
 * Foto yang sudah tersalin ke Google Drive dipanggil dari sana (berkasnya
 * dijadikan publik, jadi tidak perlu login). Yang dipakai adalah endpoint
 * `thumbnail` — `uc?export=view` sering dialihkan Google dan gagal dimuat di
 * tag <img>. Selama belum tersalin, berkas lokal yang dipakai.
 */
export function photoSrc(p, width = 1200) {
  if (p?.drive_file_id) {
    return `https://drive.google.com/thumbnail?id=${p.drive_file_id}&sz=w${width}`
  }
  return p?.photo_url || ''
}

/** Alamat cadangan bila tautan Drive gagal dimuat — dipakai di @error. */
export function photoFallback(p) { return p?.photo_url || '' }

export function backupLabel(s) {
  if (s === 'sent') return '✓ tersalin ke Drive'
  if (s === 'failed') return '! gagal disalin'
  return '… menunggu disalin'
}

export function backupCls(s) {
  if (s === 'sent') return 'text-emerald-600'
  if (s === 'failed') return 'text-red-600'
  return 'text-gray-400'
}
