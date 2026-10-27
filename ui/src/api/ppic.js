import { apiClient } from './client.js'

export const ppicApi = {
  // Kontrol pemakaian dan batas kewajaran per bahan/gudang.
  getUsageControl: (params) => apiClient.get('/admin/ppic/usage-control', { params }),
  getUsageLookups: () => apiClient.get('/admin/ppic/usage-control/lookups'),
  saveUsagePolicy: (data) => apiClient.put('/admin/ppic/usage-control/policy', data),
  getUsagePolicyHistory: (warehouseId, itemId) => apiClient.get('/admin/ppic/usage-control/policy/history', { params: { warehouse_id: warehouseId, item_id: itemId } }),
  exportUsageControl: (params) => apiClient.get('/admin/ppic/usage-control/export', { params, responseType: 'blob' }),
  // Pilihan gudang/bahan untuk tiga fungsi PPIC, tanpa izin Gudang tambahan.
  getLookups: () => apiClient.get('/admin/ppic/lookups'),

  // Monitor Kedaluwarsa (FEFO)
  getExpiry: (params) =>
    apiClient.get('/admin/ppic/expiry', { params }),
  ackExpiryBatch: (batchId, note) =>
    apiClient.post(`/admin/ppic/expiry/${batchId}/ack`, { note }),

  // Stock Opname
  listOpnames: (params) =>
    apiClient.get('/admin/ppic/opnames', { params }),
  getOpname: (id) =>
    apiClient.get(`/admin/ppic/opnames/${id}`),
  createOpname: (data) =>
    apiClient.post('/admin/ppic/opnames', data),
  saveOpnameCounts: (id, items) =>
    apiClient.put(`/admin/ppic/opnames/${id}/items`, { items }),
  submitOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/submit`),
  approveOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/approve`),
  cancelOpname: (id) =>
    apiClient.post(`/admin/ppic/opnames/${id}/cancel`),
}
