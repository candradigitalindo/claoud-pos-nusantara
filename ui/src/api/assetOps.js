import { apiClient } from './client.js'

export const assetDisposalsApi = {
  list:    (params = {}) => apiClient.get('/admin/asset-disposals', { params }),
  create:  (data)        => apiClient.post('/admin/asset-disposals', data),
  approve: (id, data)    => apiClient.post(`/admin/asset-disposals/${id}/approve`, data),
}

export const assetOpnamesApi = {
  list:    (params = {}) => apiClient.get('/admin/asset-opnames', { params }),
  get:     (id)          => apiClient.get(`/admin/asset-opnames/${id}`),
  create:  (data)        => apiClient.post('/admin/asset-opnames', data),
  saveCount: (id, lines) => apiClient.put(`/admin/asset-opnames/${id}/items`, { lines }),
  approve: (id)          => apiClient.post(`/admin/asset-opnames/${id}/approve`),
}

export const assetReportsApi = {
  summary: ()                  => apiClient.get('/admin/asset-summary'),
  report:  (type, params = {}) => apiClient.get(`/admin/asset-reports/${type}`, { params }),
  export:  (type, params = {}) => apiClient.get(`/admin/asset-reports/${type}/export`, {
    params, responseType: 'blob', timeout: 120000,
  }),
}
