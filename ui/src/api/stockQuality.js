import { apiClient as api } from './client'
export const stockQualityApi = {
  list: params => api.get('/admin/stock-quality', { params }),
  get: id => api.get(`/admin/stock-quality/${id}`),
  create: data => api.post('/admin/stock-quality', data),
  act: (id, data) => api.post(`/admin/stock-quality/${id}/actions`, data),
  lookups: () => api.get('/admin/stock-quality/lookups'),
  batches: (warehouse_id, item_id) => api.get('/admin/stock-quality/batches', { params: { warehouse_id, item_id } }),
}
