import { apiClient } from './client.js'

export const reservationsApi = {
  list:      (params = {}) => apiClient.get('/admin/reservations', { params }),
  get:       (id)          => apiClient.get(`/admin/reservations/${id}`),
  create:    (data)        => apiClient.post('/admin/reservations', data),
  update:    (id, data)    => apiClient.put(`/admin/reservations/${id}`, data),
  // disposition: 'refund' | 'hangus' — wajib saat membatalkan reservasi yang sudah ada uang masuk.
  setStatus: (id, status, disposition = '') => apiClient.patch(`/admin/reservations/${id}/status`, { status, disposition }),
  remove:    (id)          => apiClient.delete(`/admin/reservations/${id}`),
  // Uang muka: admin mencatat langsung sah; bukti dari pelanggan divalidasi/ditolak.
  payments:        (id)            => apiClient.get(`/admin/reservations/${id}/payments`),
  addPayment:      (id, data)      => apiClient.post(`/admin/reservations/${id}/payments`, data),
  validatePayment: (id, pid)       => apiClient.post(`/admin/reservations/${id}/payments/${pid}/validate`),
  rejectPayment:   (id, pid, reason) => apiClient.post(`/admin/reservations/${id}/payments/${pid}/reject`, { reason }),
  settings:        ()              => apiClient.get('/admin/reservations/settings'),
  updateSettings:  (data)          => apiClient.put('/admin/reservations/settings', data),
}
