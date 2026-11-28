import { apiClient } from './client.js'

export const purchaseApi = {
  list: (params) =>
    apiClient.get('/admin/purchase-requests', { params }),

  get: (id) =>
    apiClient.get(`/admin/purchase-requests/${id}`),

  create: (data) =>
    apiClient.post('/admin/purchase-requests', data),

  createManualPayment: (data) =>
    apiClient.post('/admin/purchase-requests/manual-payment', data),

  submitPaymentApprovals: (data) =>
    apiClient.post('/admin/purchase-requests/payment-submissions', data),

  decidePaymentApprovals: (data) =>
    apiClient.post('/admin/purchase-requests/payment-approvals', data),

  updateStatus: (id, data) =>
    apiClient.put(`/admin/purchase-requests/${id}/status`, data),

  // Sumber dana Kas Kasir Outlet: tugas penyerahan kas yang diproses tablet
  // kasir outlet (online). Belum ada uang keluar sampai kasir memprosesnya.
  assignCashPayout: (data) =>
    apiClient.post('/admin/procurement-payouts', data),

  cancelCashPayout: (id, data = {}) =>
    apiClient.post(`/admin/procurement-payouts/${id}/cancel`, data),

  updateItems: (id, data) =>
    apiClient.put(`/admin/purchase-requests/${id}/items`, data),

  // Tujuan pembayaran diisi bagian Pembelian; Keuangan hanya membaca dan
  // mengembalikannya bila salah. Keduanya wajib membawa `version`.
  updatePaymentDetails: (id, data) =>
    apiClient.put(`/admin/purchase-requests/${id}/payment-details`, data),

  returnPaymentDetails: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/payment-details/return`, data),

  splitVendor: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/split`, data),

  // Struk pembelian (dibayar per struk) & pengambilan barang oleh logistik.
  addPickup: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/pickups`, data),

  delete: (id) =>
    apiClient.delete(`/admin/purchase-requests/${id}`),

  // Pembelian Darurat (≤ batas, keputusan owner 19 Nov 2026): dibeli dulu
  // dari kas laci outlet lewat tugas Bayar Pengadaan di tablet kasir, struk
  // diunggah sesudahnya, lalu disahkan penyetuju.
  emergencyContext: (workUnitId) =>
    apiClient.get('/admin/purchase-requests/emergency/context', { params: { work_unit_id: workUnitId } }),

  createEmergency: (data) =>
    apiClient.post('/admin/purchase-requests/emergency', data),

  submitEmergencyProof: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/emergency/proof`, data),

  // decision: 'ratify' | 'reject'
  decideEmergency: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/emergency/decision`, data),

  cancelEmergency: (id, data) =>
    apiClient.post(`/admin/purchase-requests/${id}/emergency/cancel`, data),

  resendEmergency: (id, data = {}) =>
    apiClient.post(`/admin/purchase-requests/${id}/emergency/resend`, data),

  getEmergencySettings: () =>
    apiClient.get('/admin/procurement/emergency-settings'),

  updateEmergencySettings: (data) =>
    apiClient.put('/admin/procurement/emergency-settings', data),

  // Rekap bulanan Pembelian Darurat untuk audit Keuangan ({ month: 'YYYY-MM', outlet_id }).
  emergencyRecap: (params) =>
    apiClient.get('/admin/procurement/emergency-recap', { params }),

  exportEmergencyRecap: (params) =>
    apiClient.get('/admin/procurement/emergency-recap/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getDashboard: () =>
    apiClient.get('/admin/procurement-dashboard'),

  getPaymentStats: () =>
    apiClient.get('/admin/payment-stats'),

  getPaymentHistories: (id) =>
    apiClient.get(`/admin/purchase-requests/${id}/payment-histories`),

  // Satu workbook dengan sheet terpisah per vendor, termasuk pecahan vendor.
  exportPurchasing: (params) =>
    apiClient.get('/admin/purchasing/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

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
