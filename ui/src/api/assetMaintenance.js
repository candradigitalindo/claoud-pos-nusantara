import { apiClient } from './client.js'

export const assetMaintenanceApi = {
  list:     (params = {}) => apiClient.get('/admin/asset-maintenances', { params }),
  summary:  ()            => apiClient.get('/admin/asset-maintenances/summary'),
  get:      (id)          => apiClient.get(`/admin/asset-maintenances/${id}`),
  start:    (id)          => apiClient.post(`/admin/asset-maintenances/${id}/start`),
  complete: (id, data)    => apiClient.post(`/admin/asset-maintenances/${id}/complete`, data),
  cancel:   (id)          => apiClient.post(`/admin/asset-maintenances/${id}/cancel`),
  toPurchaseRequest: (id) => apiClient.post(`/admin/asset-maintenances/${id}/purchase-request`),
  // Pembuatan WO tetap lewat endpoint milik aset — satu pintu dengan form lama.
  create:   (assetId, data) => apiClient.post(`/admin/assets/${assetId}/maintenances`, data),
}
