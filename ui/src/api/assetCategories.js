import { apiClient } from './client.js'

export const assetCategoriesApi = {
  list:   (includeInactive = false) =>
    apiClient.get('/admin/asset-categories', { params: includeInactive ? { include_inactive: '1' } : {} }),
  create: (data)     => apiClient.post('/admin/asset-categories', data),
  update: (id, data) => apiClient.put(`/admin/asset-categories/${id}`, data),
  remove: (id)       => apiClient.delete(`/admin/asset-categories/${id}`),
}
