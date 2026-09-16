import { apiClient } from './client.js'

export const assetsApi = {
  list:              (params = {}) => apiClient.get('/admin/assets', { params }),
  get:               (id)          => apiClient.get(`/admin/assets/${id}`),
  create:            (data)        => apiClient.post('/admin/assets', data),
  update:            (id, data)    => apiClient.put(`/admin/assets/${id}`, data),
  remove:            (id)          => apiClient.delete(`/admin/assets/${id}`),
  movements:         (id)          => apiClient.get(`/admin/assets/${id}/movements`),
  maintenances:      (id)          => apiClient.get(`/admin/assets/${id}/maintenances`),
  addMaintenance:    (id, data)    => apiClient.post(`/admin/assets/${id}/maintenances`, data),
  removeMaintenance: (id, mid)     => apiClient.delete(`/admin/assets/${id}/maintenances/${mid}`),
}
