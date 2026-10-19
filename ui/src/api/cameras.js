import { apiClient } from './client.js'

export const camerasApi = {
  list: (params = {}) => apiClient.get('/admin/cameras', { params }),
  options: () => apiClient.get('/admin/cameras/options'),
  gateways: () => apiClient.get('/admin/cctv/gateways'),
  createGateway: (id, rotate = false) => apiClient.post(`/admin/outlets/${id}/cctv-gateway${rotate ? '/rotate' : ''}`),
  deleteGateway: (id) => apiClient.delete(`/admin/outlets/${id}/cctv-gateway`),
  probe: (outletId, payload) => apiClient.post(`/admin/outlets/${outletId}/cameras/probe`, payload, { timeout: 45000 }),
  onvifProfiles: (outletId, payload) => apiClient.post(`/admin/outlets/${outletId}/cameras/onvif/profiles`, payload, { timeout: 45000 }),
  discoverTuya: (outletId, payload) => apiClient.post(`/admin/outlets/${outletId}/cameras/tuya/discover`, payload, { timeout: 45000 }),
  test: (id) => apiClient.post(`/admin/cameras/${id}/test`, null, { timeout: 45000 }),
  create: (outletId, payload) => apiClient.post(`/admin/outlets/${outletId}/cameras`, payload),
  update: (id, payload) => apiClient.put(`/admin/cameras/${id}`, payload),
  toggle: (id) => apiClient.post(`/admin/cameras/${id}/toggle`),
  remove: (id) => apiClient.delete(`/admin/cameras/${id}`),
  live: (id) => apiClient.post(`/admin/cameras/${id}/stream`, null, { timeout: 45000 }),
  playback: (id, payload) => apiClient.post(`/admin/cameras/${id}/playback`, payload, { timeout: 45000 }),
  stop: (id, name) => apiClient.post(`/admin/cameras/${id}/stop`, null, { params: { name } }),
  refreshSession: () => apiClient.post('/admin/cameras/session'),
}
