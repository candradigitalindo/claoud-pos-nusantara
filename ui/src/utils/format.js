/**
 * src/utils/format.js — Display formatting helpers
 *
 * All functions are pure (no side effects) and return strings.
 * Time formatting respects the app-wide timezone setting.
 */

/**
 * Get the current timezone from the settings store (cached in localStorage).
 * Falls back to Asia/Jakarta if not set.
 */
function getTimezone() {
  return localStorage.getItem('cloud_pos_timezone') || 'Asia/Jakarta'
}

/**
 * Format a number as Indonesian Rupiah currency.
 * @param {number|string} value
 * @returns {string}  e.g. "Rp 1.500.000"
 */
// Intl.NumberFormat mahal untuk dibuat; satu instance dipakai ulang karena
// opsinya konstan. Penting di halaman tabel besar (buku besar bisa ratusan
// pemanggilan per render).
const RUPIAH_FMT = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
})

export function formatRupiah(value) {
  return RUPIAH_FMT.format(Number(value) || 0)
}

/**
 * Format an ISO date string or Date to dd-MM-YYYY, HH:mm.
 * Uses the app-wide timezone setting.
 * @param {string|Date} value
 * @returns {string}  e.g. "04-03-2026, 14:30"
 */
export function formatDateTime(value) {
  if (!value) return '—'
  const d = new Date(value)
  const tz = getTimezone()
  const parts = new Intl.DateTimeFormat('en-GB', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: tz,
  }).formatToParts(d)
  const get = (type) => parts.find(p => p.type === type)?.value || ''
  return `${get('day')}-${get('month')}-${get('year')}, ${get('hour')}:${get('minute')}`
}

/**
 * Format an ISO date string or Date to dd-MM-YYYY (date only, no time).
 * Uses the app-wide timezone setting.
 * @param {string|Date} value
 * @returns {string}  e.g. "04-03-2026"
 */
export function formatDate(value) {
  if (!value) return '—'
  const d = new Date(value)
  const tz = getTimezone()
  const parts = new Intl.DateTimeFormat('en-GB', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: tz,
  }).formatToParts(d)
  const get = (type) => parts.find(p => p.type === type)?.value || ''
  return `${get('day')}-${get('month')}-${get('year')}`
}

/**
 * Get today's date as YYYY-MM-DD in the configured timezone.
 * Use this instead of new Date().toISOString().slice(0, 10).
 * @returns {string}  e.g. "2026-04-10"
 */
export function todayDateString() {
  const tz = getTimezone()
  const parts = new Intl.DateTimeFormat('en-CA', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: tz,
  }).formatToParts(new Date())
  const get = (type) => parts.find(p => p.type === type)?.value || ''
  return `${get('year')}-${get('month')}-${get('day')}`
}

/**
 * Convert a YYYY-MM-DD or ISO timestamp string to dd-MM-YYYY for display.
 * @param {string} value  e.g. "2026-04-11" or "2026-04-11T00:00:00Z"
 * @returns {string}      e.g. "11-04-2026"
 */
export function formatDateStr(value) {
  if (!value || typeof value !== 'string') return '—'
  const dateOnly = value.includes('T') ? value.split('T')[0] : value
  const parts = dateOnly.split('-')
  if (parts.length !== 3) return value
  return `${parts[2]}-${parts[1]}-${parts[0]}`
}

/**
 * Return a short relative time string.
 * @param {string|Date} value
 * @returns {string}  e.g. "3 menit lalu"
 */
export function timeAgo(value) {
  if (!value) return '—'
  const diff = (Date.now() - new Date(value).getTime()) / 1000
  if (diff < 60)   return 'baru saja'
  if (diff < 3600) return `${Math.floor(diff / 60)} menit lalu`
  if (diff < 86400) return `${Math.floor(diff / 3600)} jam lalu`
  return `${Math.floor(diff / 86400)} hari lalu`
}
