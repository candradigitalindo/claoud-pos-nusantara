/**
 * src/utils/safeUrl.js — Penyaring URL bukti & foto dari server (T-75)
 *
 * Bukti dan foto (serah terima aset, penerimaan pengadaan, bukti bayar
 * pengadaan, bukti DP reservasi, foto kirim mutasi stok/aset) disimpan server
 * sebagai path unggahan relatif "/uploads/YYYY-MM/<ULID>.<ext>". Nilai itu
 * tetap data dari server — dulu bisa diisi apa saja lewat API — jadi sebelum
 * dipasang sebagai href/src disaring di sini, aturannya sama dengan server
 * (services/upload_url.go):
 *   - path relatif berawalan "/uploads/", hanya huruf/angka ASCII, '-', '_',
 *     '.', '/', tanpa segmen kosong, '.', atau '..';
 *   - atau https ke domain tepercaya, HANYA bila pemanggil memintanya
 *     (mis. salinan Google Drive foto serah terima).
 * Selain itu hasilnya '' — pemanggil TIDAK boleh merendernya sebagai tautan
 * atau gambar (javascript:, data:, http://, domain lain, path traversal).
 */

// Domain tepercaya per keperluan. Drive = cadangan foto serah terima
// (rclone link → https://drive.google.com/open?id=…, gambar mini
// https://drive.google.com/thumbnail?id=…).
export const TRUSTED_HOSTS = Object.freeze({
  drive: Object.freeze(['drive.google.com']),
})

const UPLOAD_PREFIX = '/uploads/'
const MAX_UPLOAD_LEN = 255 // sama dengan maxUploadURLLen di server
const MAX_URL_LEN = 2048
const UPLOAD_CHARS = /^[A-Za-z0-9._\/-]+$/

function safeUploadPath(s) {
  if (s.length > MAX_UPLOAD_LEN || !s.startsWith(UPLOAD_PREFIX)) return ''
  if (!UPLOAD_CHARS.test(s)) return ''
  const segs = s.slice(UPLOAD_PREFIX.length).split('/')
  if (segs.some(seg => seg === '' || seg === '.' || seg === '..')) return ''
  return s
}

/**
 * safeUrl — URL yang aman dipasang sebagai href/src, atau '' bila tidak sah.
 * @param {unknown} raw   nilai dari server
 * @param {{ hosts?: readonly string[] }} [opts] domain https yang diizinkan
 *        (kosong = hanya path /uploads/…)
 */
export function safeUrl(raw, { hosts = [] } = {}) {
  if (typeof raw !== 'string') return ''
  const s = raw.trim()
  if (!s) return ''
  if (s.startsWith('/')) return safeUploadPath(s)
  if (!hosts.length || s.length > MAX_URL_LEN || !/^https:\/\//i.test(s)) return ''
  let u
  try { u = new URL(s) } catch { return '' }
  if (u.protocol !== 'https:' || u.username || u.password || u.port) return ''
  if (!hosts.includes(u.hostname.toLowerCase())) return ''
  return u.href
}

/** Berkas gambar (bukan PDF) — untuk memilih <img> atau tautan berkas. */
export function isImageUrl(url) {
  return /\.(jpe?g|png|webp|gif)$/i.test(String(url || '').split(/[?#]/)[0])
}

/**
 * Atribut <a> untuk URL yang sudah lolos safeUrl; objek kosong bila tidak
 * ada URL sah. Dipakai bersama <component :is="url ? 'a' : 'div'"> supaya
 * nilai yang ditolak tampil sebagai teks biasa, bukan tautan.
 */
export function linkAttrs(url) {
  return url ? { href: url, target: '_blank', rel: 'noopener noreferrer' } : {}
}
