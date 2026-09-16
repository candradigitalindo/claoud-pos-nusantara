import { apiClient } from './client.js'

export const purchaseApi = {
  list: (params) =>
    apiClient.get('/admin/purchase-requests', { params }),

  get: (id) =>
    apiClient.get(`/admin/purchase-requests/${id}`),

  create: (data) =>
    apiClient.post('/admin/purchase-requests', data),

  updateStatus: (id, data) =>
    apiClient.put(`/admin/purchase-requests/${id}/status`, data),

  updateItems: (id, data) =>
    apiClient.put(`/admin/purchase-requests/${id}/items`, data),

  splitVendor: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/split`, data),

  delete: (id) =>
    apiClient.delete(`/admin/purchase-requests/${id}`),

  getDashboard: () =>
    apiClient.get('/admin/procurement-dashboard'),

  getPaymentStats: () =>
    apiClient.get('/admin/payment-stats'),

  getPaymentHistories: (id) =>
    apiClient.get(`/admin/purchase-requests/${id}/payment-histories`),

  // Download Excel — responseType blob melewati unwrap envelope di client.js;
  // timeout dinaikkan karena file dibuat on-the-fly di server.
  exportPayments: (params) =>
    apiClient.get('/admin/procurement-payments/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),
}

// Serah Terima pengadaan → aset / stok gudang (docs/perlengkapan-aset.md §8)
export const receivingApi = {
  draft:   (prId)       => apiClient.get(`/admin/purchase-requests/${prId}/receiving-draft`),
  receive: (prId, data) => apiClient.post(`/admin/purchase-requests/${prId}/receive-goods`, data),
  incomplete: ()        => apiClient.get('/admin/asset-incomplete-receipts'),
  queue:   (kind)       => apiClient.get('/admin/receiving-queue', { params: { kind } }),
}
