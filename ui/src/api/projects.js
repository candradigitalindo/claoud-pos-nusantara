import { apiClient } from './client.js'

// Projek pembangunan/renovasi — payung RAB di atas pengadaan.
// Catatan envelope: semua endpoint di sini non-paginated, jadi apiClient sudah
// meng-unwrap `{success,data}` → pemanggil menerima payload-nya langsung
// (JANGAN baca `res.data`).
export const projectsApi = {
  list: (params) =>
    apiClient.get('/admin/projects', { params }),

  /** → { project, requests } */
  get: (id) =>
    apiClient.get(`/admin/projects/${id}`),

  create: (data) =>
    apiClient.post('/admin/projects', data),

  update: (id, data) =>
    apiClient.put(`/admin/projects/${id}`, data),

  remove: (id) =>
    apiClient.delete(`/admin/projects/${id}`),

  // ── RAB per baris ──
  /** → { project_id, project_number, project_status, rab_status, rab_version, total, items[] } */
  rab: (id) =>
    apiClient.get(`/admin/projects/${id}/rab`),

  /** Ganti seluruh susunan RAB (hanya saat rab_status = 'draft'). → items[] */
  saveRab: (id, items) =>
    apiClient.put(`/admin/projects/${id}/rab`, { items }),

  /** Tetapkan RAB: baris dikunci, versi naik, belanja tahap boleh dibuat. → project */
  setRab: (id) =>
    apiClient.post(`/admin/projects/${id}/rab/set`),

  /** Buka RAB yang sudah ditetapkan untuk direvisi. → project */
  reopenRab: (id) =>
    apiClient.post(`/admin/projects/${id}/rab/reopen`),
}
