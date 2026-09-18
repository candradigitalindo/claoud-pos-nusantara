import { apiClient as api } from './client.js'

// ── Stock Items ───────────────────────────────────────────────
export const stockItemsApi = {
  list: (params = {}) => api.get('/admin/stock-items', { params }),
  get: (id) => api.get(`/admin/stock-items/${id}`),
  create: (data) => api.post('/admin/stock-items', data),
  update: (id, data) => api.put(`/admin/stock-items/${id}`, data),
  toggle: (id) => api.post(`/admin/stock-items/${id}/toggle`),
  delete: (id) => api.delete(`/admin/stock-items/${id}`),
}

// ── Stock Item Categories ─────────────────────────────────────
export const stockItemCategoriesApi = {
  list: () => api.get('/admin/stock-item-categories'),
  create: (data) => api.post('/admin/stock-item-categories', data),
  update: (id, data) => api.put(`/admin/stock-item-categories/${id}`, data),
  delete: (id) => api.delete(`/admin/stock-item-categories/${id}`),
}

// ── Warehouses ────────────────────────────────────────────────
export const warehousesApi = {
  list: (params = {}) => api.get('/admin/warehouses', { params }),
  get: (id) => api.get(`/admin/warehouses/${id}`),
  create: (data) => api.post('/admin/warehouses', data),
  update: (id, data) => api.put(`/admin/warehouses/${id}`, data),
  delete: (id) => api.delete(`/admin/warehouses/${id}`),
}

// ── Stock Ledger & Movements ──────────────────────────────────
export const stockLedgerApi = {
  list: (params = {}) => api.get('/admin/stock-ledger', { params }),
  movements: (params = {}) => api.get('/admin/stock-movements', { params }),
  adjust: (data) => api.post('/admin/stock-adjustments', data),
  produce: (data) => api.post(`/admin/stock-items/${data.item_id}/produce`, data),
}

// ── Penerimaan Barang (Goods Receipt / GRN) ───────────────────
export const goodsReceiptsApi = {
  list: (params = {}) => api.get('/admin/goods-receipts', { params }),
  get: (id) => api.get(`/admin/goods-receipts/${id}`),
  create: (data) => api.post('/admin/goods-receipts', data),
}

// ── Master Recipe System (F&B) ───────────────────────────
export const recipeMastersApi = {
  list: (params = {}) => api.get('/admin/recipe-masters', { params }),
  get: (id) => api.get(`/admin/recipe-masters/${id}`),
  save: (id, data) => id ? api.put(`/admin/recipe-masters/${id}`, data) : api.post('/admin/recipe-masters', data),
  delete: (id) => api.delete(`/admin/recipe-masters/${id}`),
}

// ── Semi-Finished Good (SFG) Recipes ──────────────────────────
export const stockItemRecipesApi = {
  get: (id) => api.get(`/admin/stock-items/${id}/recipes`),
  save: (id, items, visibility) => api.put(`/admin/stock-items/${id}/recipes`, { parent_item_id: id, items, visibility }),
}


// ── Stock Transfers ───────────────────────────────────────────
export const stockTransfersApi = {
  list: (params = {}) => api.get('/admin/stock-transfers', { params }),
  get: (id) => api.get(`/admin/stock-transfers/${id}`),
  create: (data) => api.post('/admin/stock-transfers', data),
  // photoUrl wajib saat status 'sent' (bukti barang berangkat dari gudang asal).
  updateStatus: (id, status, photoUrl = '') => api.put(`/admin/stock-transfers/${id}/status`, { status, photo_url: photoUrl }),
  updateReceivedQty: (id, itemId, qty) =>
    api.put(`/admin/stock-transfers/${id}/items/${itemId}/received`, { received_qty_base: qty }),
}

// ── Product Recipes ───────────────────────────────────────────
export const productRecipesApi = {
  get: (productId) => api.get(`/admin/products/${productId}/recipes`),
  save: (productId, items) => api.put(`/admin/products/${productId}/recipes`, { product_id: productId, items }),
}
