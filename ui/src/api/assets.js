import { apiClient } from './client.js'

export const assetsApi = {
  // Dengan params.page → respons berhalaman { data, page, total, total_pages,
  // summary } (dikembalikan utuh oleh client.js). Tanpa page → array utuh,
  // dipakai pemilih aset di halaman lain.
  list:              (params = {}) => apiClient.get('/admin/assets', { params }),
  // Excel semua aset yang lolos saringan, tanpa paginasi. responseType blob
  // melewati unwrap envelope di client.js.
  exportExcel:       (params = {}) => apiClient.get('/admin/asset-export', {
    params, responseType: 'blob', timeout: 120000,
  }),
  get:               (id)          => apiClient.get(`/admin/assets/${id}`),
  create:            (data)        => apiClient.post('/admin/assets', data),
  update:            (id, data)    => apiClient.put(`/admin/assets/${id}`, data),
  remove:            (id)          => apiClient.delete(`/admin/assets/${id}`),
  movements:         (id)          => apiClient.get(`/admin/assets/${id}/movements`),
  maintenances:      (id)          => apiClient.get(`/admin/assets/${id}/maintenances`),
  addMaintenance:    (id, data)    => apiClient.post(`/admin/assets/${id}/maintenances`, data),
  removeMaintenance: (id, mid)     => apiClient.delete(`/admin/assets/${id}/maintenances/${mid}`),
}
