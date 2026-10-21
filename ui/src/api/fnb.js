import { apiClient as api } from './client'

export const fnbApi = {
  dashboard: params => api.get('/admin/fnb/dashboard', { params }),
  costing: params => api.get('/admin/fnb/costing', { params }),
  saveProfile: (id, data) => api.put(`/admin/fnb/products/${id}/profile`, data),
  developments: params => api.get('/admin/fnb/developments', { params }),
  options: outletID => api.get('/admin/fnb/development-options', { params: { outlet_id: outletID } }),
  saveDevelopment: (id, data) => id ? api.put(`/admin/fnb/developments/${id}`, data) : api.post('/admin/fnb/developments', data),
  transition: (id, status, version) => api.put(`/admin/fnb/developments/${id}/status`, { status, version }),
}
