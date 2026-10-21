import { apiClient } from './client'
export const financeApi = {
  classify: (id, data) => apiClient.put(`/admin/finance/cash-movements/${id}/account`, data),
  accounts: () => apiClient.get('/admin/finance/accounts'),
  lookups: () => apiClient.get('/admin/finance/lookups'),
  operations: params => apiClient.get('/admin/finance/operations', { params }),
  entries: params => apiClient.get('/admin/finance/entries', { params }),
  detail: id => apiClient.get(`/admin/finance/entries/${id}`),
  create: data => apiClient.post('/admin/finance/entries', data),
  update: (id, data) => apiClient.put(`/admin/finance/entries/${id}`, data),
  action: (id, data) => apiClient.post(`/admin/finance/entries/${id}/actions`, data),
}
