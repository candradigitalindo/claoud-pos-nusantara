/**
 * products.js — Admin API for products & categories across all outlets
 */
import { apiClient } from './client.js'

export const productsApi = {
  // ── Products ──────────────────────────────────────────────
  listProducts:   (params = {})      => apiClient.get('/admin/products', { params }),
  classification: (params = {})    => apiClient.get('/admin/products/classification', { params }),
  applyClassification: (data, params = {}) => apiClient.post('/admin/products/classification', data, { params }),
  recipeOptions: (outletId)         => apiClient.get('/admin/products/recipe-options', { params: { outlet_id: outletId } }),
  recipeDetail:  (outletId, id)     => apiClient.get(`/admin/products/recipe-options/${encodeURIComponent(id)}`, { params: { outlet_id: outletId } }),
  createProduct:  (data)             => apiClient.post('/admin/products', data),
  updateProduct:  (id, data)         => apiClient.put(`/admin/products/${id}`, data),
  deleteProduct:  (id)               => apiClient.delete(`/admin/products/${id}`),
  uploadPhoto:    (id, file)         => { const fd = new FormData(); fd.append('file', file); return apiClient.post(`/admin/products/${id}/photo`, fd, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 0 }) },
  removePhoto:    (id)               => apiClient.delete(`/admin/products/${id}/photo`),

  // ── Categories ────────────────────────────────────────────
  listCategories:   (params = {})    => apiClient.get('/admin/categories', { params }),
  createCategory:   (data)           => apiClient.post('/admin/categories', data),
  updateCategory:   (id, data)       => apiClient.put(`/admin/categories/${id}`, data),
  deleteCategory:   (id)             => apiClient.delete(`/admin/categories/${id}`),
}
