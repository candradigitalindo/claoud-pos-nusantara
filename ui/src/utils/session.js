/**
 * src/utils/session.js — Pesan sesi untuk halaman login
 *
 * Sejak T-55 server bisa mencabut sesi yang masih berjalan: password diganti
 * atau direset, role akun diganti, nama/cakupan role diubah, akun
 * dinonaktifkan. Permintaan berikutnya dibalas 401 dan UI kembali ke halaman
 * login. Supaya pengguna tahu sebabnya, pihak yang mengeluarkan (halaman atau
 * interceptor 401 di api/client.js) menitipkan pesan di sini dan Login.vue
 * menampilkannya sekali. Disimpan di sessionStorage: hanya untuk tab ini.
 */

const NOTICE_KEY = 'cloud_pos_login_notice'
const NOTICE_TYPES = ['info', 'success', 'warning']

/** Pesan galat sementara (503 dari server, 502/504 dari proxy). */
export const SERVICE_UNAVAILABLE_MSG = 'Layanan sementara terganggu, coba lagi'

export function setLoginNotice(message, type = 'info') {
  if (!message) return
  try {
    sessionStorage.setItem(NOTICE_KEY, JSON.stringify({ message: String(message), type }))
  } catch { /* penyimpanan tidak tersedia — pesan dilewati */ }
}

export function hasLoginNotice() {
  try { return !!sessionStorage.getItem(NOTICE_KEY) } catch { return false }
}

/** Ambil lalu hapus pesan (ditampilkan sekali). null bila tidak ada. */
export function takeLoginNotice() {
  try {
    const raw = sessionStorage.getItem(NOTICE_KEY)
    sessionStorage.removeItem(NOTICE_KEY)
    if (!raw) return null
    const n = JSON.parse(raw)
    if (!n || typeof n.message !== 'string' || !n.message) return null
    return { message: n.message, type: NOTICE_TYPES.includes(n.type) ? n.type : 'info' }
  } catch { return null }
}

/**
 * Teks untuk sesi yang ditolak server (401). Pesan server berasal dari
 * RejectAdminSession (middleware/auth.go).
 */
export function sessionEndedMessage(serverMsg) {
  const m = String(serverMsg || '').toLowerCase()
  if (m.includes('akun tidak aktif')) {
    return 'Akun Anda dinonaktifkan. Hubungi admin bila ini keliru.'
  }
  if (m.includes('sesi sudah tidak berlaku')) {
    return 'Sesi Anda sudah tidak berlaku — password, role, atau cakupan akun Anda baru saja diubah. Silakan login ulang.'
  }
  return 'Sesi berakhir. Silakan login ulang.'
}
