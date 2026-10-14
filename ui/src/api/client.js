/**
 * src/api/client.js — Axios HTTP client singleton
 *
 * Every API module imports `apiClient` from here — there is one shared
 * Axios instance for the whole app.
 *
 * Features configured here:
 *   - baseURL      : reads VITE_API_BASE_URL env var (falls back to /api/v1)
 *   - Request interceptor  : auto-attaches Bearer token from localStorage
 *   - Response interceptor : unwraps { success, data } envelope
 *                            auto-redirects to /login on 401
 *                            503 (galat sementara) TIDAK membuang sesi
 *
 * AI NOTE: To add a request timeout, set `timeout: 10000` in the config.
 * To add retry logic, integrate the `axios-retry` package here.
 */

import axios from 'axios'
import { setLoginNotice, hasLoginNotice, sessionEndedMessage, SERVICE_UNAVAILABLE_MSG } from '@/utils/session.js'

// Base URL — override with VITE_API_BASE_URL in .env for staging/prod
const BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'
const TOKEN_KEY = 'cloud_pos_token'

// ── Axios instance ─────────────────────────────────────────
export const apiClient = axios.create({
  baseURL: BASE_URL,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  },
})

// ── Request interceptor ────────────────────────────────────
// Attaches the JWT token to every outgoing request
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem(TOKEN_KEY)
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error),
)

// ── Response interceptor ───────────────────────────────────
// Unwraps the standard { success, data, error } envelope from the Go API.
// On 401, clears the token and redirects to /login.
// On 503 (server tidak bisa memeriksa sesi / DB sesaat terganggu) sesi tetap
// dipertahankan — permintaan boleh diulang begitu layanan pulih.
apiClient.interceptors.response.use(
  (response) => {
    // Our API always returns { success: bool, data: any }
    const body = response.data
    if (body && typeof body === 'object' && 'success' in body) {
      if (!body.success) {
        const msg = body.error || 'Unknown error from server'
        return Promise.reject(new Error(msg))
      }
      // Paginated responses ({ success, data, page, total, total_pages }) →
      // return the FULL body so callers can read both `data` (array) and the
      // pagination metadata (total_pages, total). Detected by `total_pages`.
      if ('total_pages' in body) return body
      // Standard envelope → return the `data` payload directly to callers
      return body.data ?? body
    }
    return response.data
  },
  (error) => {
    const isLoginRequest = error.config?.url?.includes('/admin/login')
    const status = error.response?.status

    if (status === 401 && !isLoginRequest) {
      // Token expired, invalid, or REVOKED by the server (password/role/scope
      // changed, account deactivated — T-55) — clear session and redirect to
      // login (skip redirect when the failed request IS the login call itself,
      // so Login.vue can display the "wrong credentials" error).
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem('cloud_pos_admin')
      localStorage.removeItem('cloud_pos_permissions')
      localStorage.removeItem('cloud_pos_scope')
      localStorage.removeItem('cloud_pos_scope_outlet_ids')
      localStorage.removeItem('cloud_pos_redirect_to')
      // Sudah di halaman login (mis. baru saja keluar sendiri, lalu ada
      // permintaan lama yang menyusul): cukup bersihkan, jangan muat ulang
      // dan jangan titip pesan yang akan basi.
      if (window.location.pathname !== '/login') {
        // Alasannya dititipkan ke halaman login; pesan yang sudah dititipkan
        // halaman lain (mis. "Password diganti") tidak ditimpa.
        if (!hasLoginNotice()) setLoginNotice(sessionEndedMessage(error.response?.data?.error), 'warning')
        window.location.href = '/login'
      }
    }
    // 503 = server menolak SEMENTARA (sesi tidak dapat diperiksa, DB sesaat
    // terganggu); 502/504 = proxy tidak mendapat jawaban dari server. Ketiganya
    // bukan alasan keluar: sesi dibiarkan, pengguna cukup mencoba lagi.
    const unavailable = status === 503 || status === 502 || status === 504
    const message = unavailable
      ? SERVICE_UNAVAILABLE_MSG
      : (error.response?.data?.error ||
        error.message ||
        'Terjadi kesalahan jaringan')
    // Kode HTTP ikut dibawa supaya halaman bisa membedakan, mis., 409
    // "pengajuan berubah, muat ulang" dari galat biasa; pemanggil lama yang
    // hanya membaca `message` tidak terpengaruh.
    const err = new Error(message)
    err.status = error.response?.status
    return Promise.reject(err)
  },
)
