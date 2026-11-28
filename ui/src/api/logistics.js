import { apiClient as api } from './client.js'

export const logisticsApi = {
  dispatchableItems: (params = {}) => api.get('/admin/logistics/dispatchable-items', { params }),
  list: (params = {}) => api.get('/admin/logistics/deliveries', { params }),
  get: (id) => api.get(`/admin/logistics/deliveries/${id}`),
  create: (data) => api.post('/admin/logistics/deliveries', data),
  update: (id, data) => api.put(`/admin/logistics/deliveries/${id}`, data),
  updateStatus: (id, data) => api.put(`/admin/logistics/deliveries/${id}/status`, data),
  updatePhotos: (id, data) => api.put(`/admin/logistics/deliveries/${id}/photos`, data),
  receive: (id, data) => api.post(`/admin/logistics/deliveries/${id}/receive`, data),
}
