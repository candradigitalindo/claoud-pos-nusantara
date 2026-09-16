import { apiClient } from './client.js'

export const assetTransfersApi = {
  list:      (params = {}) => apiClient.get('/admin/asset-transfers', { params }),
  get:       (id)          => apiClient.get(`/admin/asset-transfers/${id}`),
  available: (outletId, reason) => apiClient.get('/admin/asset-transfers/available', { params: { outlet_id: outletId, reason } }),
  create:    (data)        => apiClient.post('/admin/asset-transfers', data),
  update:    (id, data)    => apiClient.put(`/admin/asset-transfers/${id}`, data),
  remove:    (id)          => apiClient.delete(`/admin/asset-transfers/${id}`),
  submit:    (id)          => apiClient.post(`/admin/asset-transfers/${id}/submit`),
  approve:   (id)          => apiClient.post(`/admin/asset-transfers/${id}/approve`),
  reject:    (id, reason)  => apiClient.post(`/admin/asset-transfers/${id}/reject`, { reason }),
  send:      (id)          => apiClient.post(`/admin/asset-transfers/${id}/send`),
  receive:   (id, items)   => apiClient.post(`/admin/asset-transfers/${id}/receive`, { items }),
  cancel:    (id)          => apiClient.post(`/admin/asset-transfers/${id}/cancel`),
}
