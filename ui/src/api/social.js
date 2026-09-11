/**
 * src/api/social.js — Kinerja Markom (Instagram / TikTok)
 *
 * Memasok angka yang dipakai grafik medsos di Laporan → Analisa Bisnis.
 *
 *   GET    /admin/social-accounts
 *   POST   /admin/social-accounts
 *   PUT    /admin/social-accounts/:id
 *   DELETE /admin/social-accounts/:id
 *   POST   /admin/social-accounts/scrape
 *   GET    /admin/social-weekly?weeks=n
 *   GET    /admin/social-accounts/:id/manual
 *   PUT    /admin/social-accounts/:id/manual
 *
 * CATATAN: apiClient sudah membuka amplop { success, data }, jadi tiap fungsi
 * mengembalikan payload-nya langsung — bukan objek respons Axios.
 */

import { apiClient } from './client.js'

export const socialApi = {
  list: () => apiClient.get('/admin/social-accounts'),

  create: (payload) => apiClient.post('/admin/social-accounts', payload),

  update: (id, payload) => apiClient.put(`/admin/social-accounts/${id}`, payload),

  remove: (id) => apiClient.delete(`/admin/social-accounts/${id}`),

  /**
   * Tarik angka dari halaman profil publik sekarang juga.
   * Sengaja bertimeout panjang: penarikan berjeda antar-akun supaya alamat IP
   * server tidak diblokir, jadi sepuluh akun memang memakan beberapa menit.
   */
  scrape: (ids = []) =>
    apiClient.post('/admin/social-accounts/scrape', { ids }, { timeout: 600000 }),

  weekly: (weeks = 12) =>
    apiClient.get('/admin/social-weekly', { params: { weeks } }),

  manualList: (id) => apiClient.get(`/admin/social-accounts/${id}/manual`),

  manualSave: (id, payload) =>
    apiClient.put(`/admin/social-accounts/${id}/manual`, payload),
}
