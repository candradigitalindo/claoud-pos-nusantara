import { apiClient } from './client.js'

// Semua endpoint di bawah /admin/whatsapp. Respons non-paginated sudah
// di-unwrap apiClient (langsung payload); list pesan & broadcast paginated
// (punya total_pages) dikembalikan utuh → baca res.data + res.total.
export const whatsappApi = {
  status:         (force = false) => apiClient.get('/admin/whatsapp/status', { params: force ? { force: 1 } : {} }),
  login:          ()     => apiClient.post('/admin/whatsapp/login'),
  logout:         ()     => apiClient.post('/admin/whatsapp/logout'),
  groups:         ()     => apiClient.get('/admin/whatsapp/groups'),
  sendTest:       (data) => apiClient.post('/admin/whatsapp/test', data),

  getSettings:    ()     => apiClient.get('/admin/whatsapp/settings'),
  updateSettings: (data) => apiClient.put('/admin/whatsapp/settings', data),
  preview:        (data) => apiClient.post('/admin/whatsapp/preview', data),

  listRecipients:  ()         => apiClient.get('/admin/whatsapp/recipients'),
  recipientUsers:  ()         => apiClient.get('/admin/whatsapp/recipients/users'),
  roles:           ()         => apiClient.get('/admin/whatsapp/roles'),
  createRecipient: (data)     => apiClient.post('/admin/whatsapp/recipients', data),
  updateRecipient: (id, data) => apiClient.put(`/admin/whatsapp/recipients/${id}`, data),
  deleteRecipient: (id)       => apiClient.delete(`/admin/whatsapp/recipients/${id}`),

  listMessages:  (params = {}) => apiClient.get('/admin/whatsapp/messages', { params }),
  retryMessage:  (id)          => apiClient.post(`/admin/whatsapp/messages/${id}/retry`),
  cancelMessage: (id)          => apiClient.post(`/admin/whatsapp/messages/${id}/cancel`),
  retryFailed:   ()            => apiClient.post('/admin/whatsapp/messages/retry-failed'),

  listBroadcasts:   (params = {}) => apiClient.get('/admin/whatsapp/broadcasts', { params }),
  previewBroadcast: (data)        => apiClient.post('/admin/whatsapp/broadcasts/preview', data),
  createBroadcast:  (data)        => apiClient.post('/admin/whatsapp/broadcasts', data),
  getBroadcast:     (id)          => apiClient.get(`/admin/whatsapp/broadcasts/${id}`),
  cancelBroadcast:  (id)          => apiClient.post(`/admin/whatsapp/broadcasts/${id}/cancel`),
}
