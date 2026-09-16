import { apiClient } from './client.js'

export const assetHandoversApi = {
  list:     ()     => apiClient.get('/admin/asset-handovers'),
  awaiting: ()     => apiClient.get('/admin/asset-handovers/awaiting'),
  get:      (id)   => apiClient.get(`/admin/asset-handovers/${id}`),
  create:   (data) => apiClient.post('/admin/asset-handovers', data),
}

export const photoBackupApi = {
  status:         () => apiClient.get('/admin/photo-backup/status'),
  retry:          () => apiClient.post('/admin/photo-backup/retry'),
  getSettings:    () => apiClient.get('/admin/photo-backup/settings'),
  updateSettings: (data) => apiClient.put('/admin/photo-backup/settings', data),
}
