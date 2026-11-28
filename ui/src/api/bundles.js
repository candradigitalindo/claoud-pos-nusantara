import { apiClient } from './client.js'

export const bundlesApi = {
  getProductComponents:  (productId)              => apiClient.get(`/admin/products/${productId}/bundle-components`),
  saveProductComponents: (productId, components)  => apiClient.put(`/admin/products/${productId}/bundle-components`, { components }),
  listComponents:        (params = {})            => apiClient.get('/admin/cross-outlet-bundles/components', { params }),
  partnerOutlets:        ()                       => apiClient.get('/admin/cross-outlet-bundles/partner-outlets'),
  partnerProducts:       (outletId)               => apiClient.get('/admin/cross-outlet-bundles/partner-products', { params: { outlet_id: outletId } }),
  listClaims:            (params = {})            => apiClient.get('/admin/cross-outlet-bundles/claims', { params }),
  createManualClaim:     (data)                   => apiClient.post('/admin/cross-outlet-bundles/claims', data),
  claim:                 (id, data = {})          => apiClient.post(`/admin/cross-outlet-bundles/claims/${id}/claim`, data),
  expire:                (id, reason = '')        => apiClient.post(`/admin/cross-outlet-bundles/claims/${id}/expire`, { reason }),
}
