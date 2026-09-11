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
}
