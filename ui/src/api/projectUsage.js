import { apiClient } from './client.js'

export const projectUsageApi = {
  projects: () => apiClient.get('/admin/project-usage/projects'),
  overview: id => apiClient.get(`/admin/project-usage/projects/${id}`),
  list: params => apiClient.get('/admin/project-usage', { params }),
  get: id => apiClient.get(`/admin/project-usage/${id}`),
  save: (id, data) => id ? apiClient.put(`/admin/project-usage/${id}`, data) : apiClient.post('/admin/project-usage', data),
  action: (id, data) => apiClient.post(`/admin/project-usage/${id}/actions`, data),
  export: params => apiClient.get('/admin/project-usage-export', { params, responseType: 'blob' }),
}
