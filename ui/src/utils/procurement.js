/**
 * src/utils/procurement.js — Jenis pengajuan pengadaan (request_type)
 *
 * Satu sumber label & warna untuk jenis pengajuan supaya halaman yang memilah
 * jenis (Pembayaran, Projek, Vendor, Dashboard) tidak menebak "bukan barang
 * berarti jasa" — sejak ada Bahan Baku, tebakan itu salah.
 */

export const PURCHASE_TYPE_LABEL = {
  barang: 'Barang',
  jasa:   'Jasa',
  bahan:  'Bahan Baku',
}

// Data lama di histori vendor ada yang memakai 'goods'/'services'.
const ALIAS = { goods: 'barang', services: 'jasa' }

export function normalizePurchaseType(t) {
  const v = String(t || '').trim().toLowerCase()
  return ALIAS[v] || v
}

export function purchaseTypeLabel(t) {
  const v = normalizePurchaseType(t)
  return PURCHASE_TYPE_LABEL[v] || (v ? v : '-')
}

// Jenis yang barangnya diserahterimakan secara fisik (punya status penerimaan).
export function isPhysicalPurchaseType(t) {
  const v = normalizePurchaseType(t)
  return v === 'barang' || v === 'bahan'
}

// Rute halaman pengajuan untuk tiap jenis.
export function purchaseTypePath(t) {
  const v = normalizePurchaseType(t)
  if (v === 'jasa') return '/purchase-services'
  if (v === 'bahan') return '/purchase-materials'
  return '/purchase-goods'
}

// Kunci izin pengadaan untuk (jenis, aksi) — cermin services.ProcurementPermission:
// barang & jasa diatur procurement.requests.*, bahan baku procurement.rawmaterials.*.
// Aksi: view, submit, verify, approve, purchasing.
export function procurementPerm(t, action) {
  return (normalizePurchaseType(t) === 'bahan' ? 'procurement.rawmaterials.' : 'procurement.requests.') + action
}

// ── Status pengajuan ───────────────────────────────────────────────────────
// Alur: pending (menunggu verifikasi) → verified (menunggu persetujuan) →
// approved → payment_requested → partial/paid → received; atau rejected /
// cancelled. Satu sumber label supaya semua halaman pengadaan sepakat.
export const PURCHASE_STATUS = {
  pending:           { label: 'Menunggu Verifikasi',  cls: 'bg-amber-100 text-amber-700' },
  verified:          { label: 'Menunggu Persetujuan', cls: 'bg-indigo-100 text-indigo-700' },
  approved:          { label: 'Disetujui',            cls: 'bg-blue-100 text-blue-700' },
  payment_requested: { label: 'Menunggu Pembayaran',  cls: 'bg-orange-100 text-orange-700' },
  partial:           { label: 'Dibayar Sebagian',     cls: 'bg-amber-100 text-amber-700' },
  paid:              { label: 'Dibayar',              cls: 'bg-emerald-100 text-emerald-700' },
  received:          { label: 'Diterima',             cls: 'bg-purple-100 text-purple-700' },
  rejected:          { label: 'Ditolak',              cls: 'bg-red-100 text-red-700' },
  cancelled:         { label: 'Dibatalkan',           cls: 'bg-gray-100 text-gray-600' },
}

// Urutan pilihan filter = urutan alur.
const PURCHASE_STATUS_ORDER = [
  'pending', 'verified', 'approved', 'payment_requested', 'partial', 'paid', 'received', 'rejected', 'cancelled',
]

// Status yang belum dikenal layar tampil apa adanya dengan warna netral —
// tidak boleh jatuh ke label "Menunggu …" yang menyesatkan.
const UNKNOWN_STATUS_CLS = 'bg-slate-100 text-slate-600 ring-1 ring-inset ring-slate-300'

// overrides: label khusus per halaman, mis. jasa { received: 'Selesai' }.
export function purchaseStatusLabel(s, overrides = {}) {
  const v = String(s || '')
  return overrides[v] || PURCHASE_STATUS[v]?.label || v || '-'
}

export function purchaseStatusBadge(s) {
  const cls = PURCHASE_STATUS[s]?.cls || UNKNOWN_STATUS_CLS
  return `inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${cls}`
}

export function purchaseStatusOptions(overrides = {}) {
  return PURCHASE_STATUS_ORDER.map(value => ({ value, label: purchaseStatusLabel(value, overrides) }))
}

// Panjang minimum alasan (dihitung per huruf setelah trim), sama dengan server:
// tolak item saat verifikasi ≥ 5, serah terima "habis pakai" ≥ 10.
export const VERIFY_REASON_MIN = 5
export const EXPENSE_REASON_MIN = 10

export function reasonLength(s) {
  return [...String(s || '').trim()].length
}

// Respons 409 = dokumen sudah berubah sejak dimuat (version/updated_at beda).
export function isConflictError(err) {
  return err?.status === 409
}

// Nama pengadaan untuk daftar. Pengajuan yang SEMUA itemnya ditolak saat
// verifikasi tidak lagi memegang `items` (server memindahkannya ke
// rejected_items), jadi namanya diambil dari grup item yang ditolak — dulu
// barisnya tampil "-" tanpa nama. Induk yang seluruh itemnya sudah dipecah ke
// vendor juga tidak memegang item (nama grupnya ada di pecahan, yang tidak
// ikut di respons daftar), jadi diberi keterangan alih-alih "-".
export function purchaseDisplayNames(row) {
  const names = (row?.items || []).map(g => String(g?.name || '').trim()).filter(Boolean)
  if (!names.length) {
    for (const r of row?.rejected_items || []) {
      const g = String(r?.group || r?.name || '').trim()
      if (g && !names.includes(g)) names.push(g)
    }
  }
  if (!names.length && row?.split_status === 'master') return 'Seluruh item dipecah ke vendor — lihat detail'
  return names.join(', ') || '-'
}

// ── Kesiapan "Ajukan Pembayaran" ───────────────────────────────────────────
// Cermin aturan server (UpdatePurchaseStatus, aksi request_payment — T-15,
// T-16, T-37). Server MENOLAK pengajuan bayar bila ada item tanpa harga final:
//   - dokumen tunggal & pecahan: SEMUA item wajib berharga final (> 0);
//   - induk hasil pecah vendor: sisa item miliknya sendiri wajib berharga;
//     pecahan 'approved' yang SELURUH itemnya berharga ikut diajukan, pecahan
//     lain diajukan dari detail pecahannya sendiri setelah harganya diisi;
//   - induk yang tidak lagi memegang item hanya bisa diajukan bila ada
//     pecahan yang ikut.
// Hasil: { ok, blockers: [teks pemblokir], notes: [keterangan pecahan] }.

/** Nama sub-item yang harga finalnya belum diisi (cermin unpricedItemNames). */
export function unpricedItemNames(doc) {
  const out = []
  for (const g of doc?.items || []) {
    for (const sub of g?.items || []) {
      if (!(Number(sub?.final_price) > 0)) out.push(String(sub?.name || '').trim() || '(tanpa nama)')
    }
  }
  return out
}

function ownItemCount(doc) {
  return (doc?.items || []).reduce((n, g) => n + (g?.items || []).length, 0)
}

function nameList(names, max = 5) {
  return names.length > max
    ? `${names.slice(0, max).join(', ')}, dan ${names.length - max} lainnya`
    : names.join(', ')
}

function childLabel(c) {
  return `${c?.vendor_name || 'vendor lain'}${c?.request_number ? ` (${c.request_number})` : ''}`
}

// Pembayaran selalu ke vendor terdaftar: rekening tujuannya dipotret server
// dari data vendor saat dibayar (keputusan owner 14 Okt 2026).
function hasRegisteredVendor(doc) {
  return !!String(doc?.vendor_id || '').trim()
}

export function requestPaymentReadiness(doc) {
  const blockers = []
  const notes = []
  if (!doc) return { ok: false, blockers, notes }
  const isChild = !!doc.parent_id
  const isMaster = !isChild && (doc.split_status === 'master' || (doc.children || []).length > 0)
  const unpriced = unpricedItemNames(doc)
  if (!hasRegisteredVendor(doc) && (!isMaster || ownItemCount(doc) > 0)) {
    blockers.push('Vendor belum dipilih dari daftar vendor — pilih vendornya lewat "Isi Harga". Rekening tujuan pembayaran diambil dari data vendor.')
  }
  if (unpriced.length) {
    if (isMaster) {
      blockers.push(`Sisa item induk (yang belum dipecah) belum berharga final: ${nameList(unpriced)}. ` +
        'Pecah item itu ke vendor lalu isi harganya di pecahan, atau pecah ke dokumen tersendiri lalu batalkan bila tidak jadi dibeli.')
    } else if (isChild) {
      blockers.push(`${unpriced.length} item belum berharga final: ${nameList(unpriced)}. Isi harganya lewat "Isi Harga Sekarang".`)
    } else {
      blockers.push(`${unpriced.length} item belum berharga final: ${nameList(unpriced)}. ` +
        'Isi harganya lewat "Beli Semua di 1 Vendor"; item yang tidak jadi dibeli pecah ke dokumen tersendiri lalu batalkan.')
    }
  }
  if (isMaster) {
    const approved = (doc.children || []).filter(c => c?.status === 'approved')
    // Pecahan ikut ke rekening terdaftar vendornya; pecahan yang vendornya
    // belum punya rekening (butuh VA / tunai) diajukan sendiri (cermin
    // cascadeStatusToChildrenTx).
    const ready = approved.filter(c => !unpricedItemNames(c).length && hasRegisteredVendor(c) && vendorHasAccount(c))
    for (const c of approved) {
      const miss = unpricedItemNames(c)
      if (miss.length) {
        notes.push(`Pecahan ${childLabel(c)} belum berharga lengkap (${nameList(miss, 3)}) sehingga tidak ikut diajukan — isi harganya lalu ajukan dari detail pecahan itu.`)
      } else if (!hasRegisteredVendor(c)) {
        notes.push(`Pecahan ${childLabel(c)} belum memilih vendor dari daftar vendor sehingga tidak ikut diajukan — pilih vendornya lewat Isi Harga di detail pecahan itu.`)
      } else if (!vendorHasAccount(c)) {
        notes.push(`Pecahan ${childLabel(c)}: vendornya belum punya rekening terdaftar sehingga tidak ikut diajukan — ajukan dari detail pecahan itu beserta cara bayarnya (VA marketplace / tunai).`)
      }
    }
    if (ready.length) notes.push(`Pecahan yang ikut diajukan (ke rekening terdaftar vendornya): ${ready.map(childLabel).join(', ')}.`)
    if (!ownItemCount(doc) && !ready.length) {
      blockers.push('Tidak ada pecahan yang bisa diajukan pembayarannya — pecahan yang belum berharga lengkap, atau yang vendornya belum punya rekening, diajukan dari detail pecahannya.')
    }
  }
  return { ok: blockers.length === 0, blockers, notes }
}

// ── Tujuan pembayaran (diisi Pembelian, dibaca Keuangan) ───────────────────
// Cermin services/purchase_payto.go (keputusan owner 19 Nov 2026):
//   - vendor_account: rekening vendor TERDAFTAR (dari data Vendor, tidak bisa
//     diketik — aturan 14 Okt 2026);
//   - marketplace_va: VA dari checkout satu pesanan;
//   - manual: tunai / cara bayar lain — hanya untuk vendor tanpa rekening.
// Keuangan hanya membaca `pay_to`, memilih sumber dana, atau mengembalikannya.

export const PAY_TO_LABELS = {
  vendor_account: 'Rekening Vendor Terdaftar',
  marketplace_va: 'Virtual Account Marketplace',
  manual: 'Tunai / Cara Bayar Lain',
}

/** Status yang masih bisa dibayar (cermin validTransitions["pay"]). */
export const PAY_STAGE_STATUSES = ['payment_requested', 'partial', 'received']

/** Vendor dokumen punya rekening terdaftar (dari pay_to server). */
export function vendorHasAccount(doc) {
  return !!String(doc?.pay_to?.vendor_account_number || '').trim()
}

/** Form awal dari pay_to dokumen: isian sebelumnya, atau rekening terdaftar. */
export function newPayToForm(payTo) {
  const p = payTo || {}
  const hasAcct = !!String(p.vendor_account_number || '').trim()
  let method = p.method || (hasAcct ? 'vendor_account' : '')
  if (method === 'vendor_account' && !hasAcct) method = ''
  if (method === 'manual' && hasAcct) method = 'vendor_account'
  const va = p.method === 'marketplace_va'
  return {
    method,
    marketplace: va ? p.marketplace || '' : '',
    order_number: va ? p.order_number || '' : '',
    bank_name: va ? p.bank_name || '' : '',
    account_number: va ? p.account_number || '' : '',
    account_holder: va ? p.account_holder || '' : '',
    text: p.method === 'manual' ? p.text || '' : '',
    note: p.note || '',
  }
}

/** Pesan galat isian tujuan pembayaran (cermin resolvePayToInput); '' = sah. */
export function payToFormError(form, payTo) {
  const f = form || {}
  const hasAcct = !!String(payTo?.vendor_account_number || '').trim()
  const filled = v => String(v || '').trim() !== ''
  switch (f.method) {
    case 'vendor_account':
      return hasAcct ? '' : 'Vendor ini belum punya rekening terdaftar — pilih VA Marketplace atau Tunai / cara bayar lain.'
    case 'marketplace_va':
      if (![f.marketplace, f.order_number, f.bank_name, f.account_number].every(filled)) {
        return 'Marketplace, nomor pesanan, bank, dan nomor virtual account wajib diisi.'
      }
      if (!/^\d+$/.test(String(f.account_number).trim())) return 'Nomor virtual account harus berisi angka.'
      return ''
    case 'manual':
      if (hasAcct) return 'Vendor ini punya rekening terdaftar — pembayarannya memakai rekening itu.'
      return filled(f.text) ? '' : 'Tulis cara bayarnya, mis. "Tunai saat barang diantar".'
    default:
      return 'Pilih cara bayar ke vendor.'
  }
}

/** Isian untuk API: hanya field milik metode terpilih. */
export function payToPayload(form) {
  const f = form || {}
  const t = v => String(v || '').trim()
  const out = { method: f.method, note: t(f.note) }
  if (f.method === 'marketplace_va') {
    Object.assign(out, {
      marketplace: t(f.marketplace), order_number: t(f.order_number), bank_name: t(f.bank_name),
      account_number: t(f.account_number), account_holder: t(f.account_holder),
    })
  }
  if (f.method === 'manual') out.text = t(f.text)
  return out
}

/** Detail pembayaran dokumen ini dikembalikan Keuangan (dan masih ditagih). */
export function payToReturned(doc) {
  return !!doc?.pay_to?.returned_at && PAY_STAGE_STATUSES.includes(doc?.status)
}

/**
 * Bisakah Pembelian mengubah detail pembayaran dokumen ini? (cermin
 * payToEditableTx). Hasil: { show, editable, reason }.
 */
export function payToEditState(doc) {
  const none = { show: false, editable: false, reason: '' }
  if (!doc || !PAY_STAGE_STATUSES.includes(doc.status)) return none
  if (doc.split_status === 'master' && !ownItemCount(doc)) return none
  if (!(Number(doc.total_final || 0) - Number(doc.paid_amount || 0) > 0.004)) return none
  const cp = doc.cash_payout
  if (cp && ['pending', 'claimed'].includes(cp.status)) {
    return { show: true, editable: false, reason: 'Sedang dibayar dari Kas Kasir Outlet — minta Keuangan menarik tugasnya dulu.' }
  }
  return { show: true, editable: true, reason: '' }
}

// ── Penerimaan (Serah Terima) ──────────────────────────────────────────────
// Status yang boleh menerima barang = status berkomitmen (cermin
// prCommittedStatuses): barang tempo boleh datang sebelum dibayar (T-79).
export const RECEIVABLE_PURCHASE_STATUSES = ['approved', 'payment_requested', 'partial', 'paid', 'received']

/**
 * Nomor GRN dari respons receive-goods. Server mengirim `grn_numbers` (array,
 * satu GRN per gudang & dokumen sumber) dan `grn_number` (gabungan dipisah
 * koma, untuk klien lama) — keduanya dibaca, tanpa duplikat.
 */
export function grnNumbersOf(res) {
  const list = Array.isArray(res?.grn_numbers) && res.grn_numbers.length
    ? res.grn_numbers
    : String(res?.grn_number || '').split(',')
  return [...new Set(list.map(s => String(s || '').trim()).filter(Boolean))]
}

// ── Tingkat kebutuhan & Pembelian Darurat ──────────────────────────────────
// Keputusan owner 19 Nov 2026 (cermin services/procurement_emergency.go):
//   rutin      — kebutuhan terencana, alur penuh;
//   prioritas  — dibutuhkan dalam beberapa hari, tanggal butuh wajib;
//   urgent     — operasional berhenti: ≤ batas (bawaan Rp500.000) lewat
//                Pembelian Darurat (beli dulu dari kas laci outlet, disahkan
//                sesudahnya); di atas batas tetap disetujui dulu.
export const URGENCY_OPTIONS = [
  { value: 'rutin',     label: 'Rutin',     hint: 'Kebutuhan terencana — alur biasa.' },
  { value: 'prioritas', label: 'Prioritas', hint: 'Dibutuhkan dalam beberapa hari; tampil di atas antrean.' },
  { value: 'urgent',    label: 'Urgent',    hint: 'Operasional terhenti / keselamatan — paling lambat 7 hari lagi.' },
]

// Server: URGENT paling lambat 7 hari lagi dan wajib menjelaskan dampaknya.
export const URGENT_MAX_LEAD_DAYS = 7
export const URGENT_NOTES_MIN = 10

export function urgencyLabel(u) {
  return URGENCY_OPTIONS.find(o => o.value === u)?.label || 'Rutin'
}

// Kategori Pembelian Darurat (cermin services.EmergencyCategories).
export const EMERGENCY_CATEGORIES = [
  { value: 'peralatan_rusak', label: 'Peralatan operasional rusak/mati', hint: 'mis. genset, mesin kopi, kompor, pompa air' },
  { value: 'bangunan',        label: 'Kerusakan bangunan/instalasi',      hint: 'mis. atap bocor, pintu, listrik, saluran air' },
  { value: 'keselamatan',     label: 'Keselamatan & keamanan',            hint: 'mis. APAR, kunci, lampu darurat' },
  { value: 'keamanan_pangan', label: 'Keamanan pangan',                   hint: 'mis. kulkas/freezer, air bersih' },
  { value: 'lainnya',         label: 'Lainnya',                           hint: 'jelaskan di alasan' },
]
export const EMERGENCY_REASON_MIN = 10

// Status pengesahan Pembelian Darurat.
export const RATIFICATION = {
  pending:  { label: 'Menunggu Pengesahan', cls: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-200' },
  ratified: { label: 'Disahkan',            cls: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-200' },
  rejected: { label: 'Pengesahan Ditolak',  cls: 'bg-red-100 text-red-700 ring-1 ring-inset ring-red-300' },
}

// Pembelian Darurat: kas sudah diserahkan kasir (pembayaran tercatat).
export function emergencyCashHanded(doc) {
  return Math.round((Number(doc?.paid_amount) || 0) * 100) > 0
}
