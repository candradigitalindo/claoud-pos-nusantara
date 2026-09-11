import { apiClient } from './client.js'

export const salesApi = {
  getReport: (params) =>
    apiClient.get('/admin/sales-report', { params }),

  getUnpaidOrders: (params) =>
    apiClient.get('/admin/unpaid-orders', { params }),

  // Download Excel — responseType blob melewati unwrap envelope di client.js;
  // timeout dinaikkan karena file dibuat on-the-fly di server.
  exportReport: (params) =>
    apiClient.get('/admin/sales-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getProductSalesReport: (params) =>
    apiClient.get('/admin/product-sales-report', { params }),

  exportProductSalesReport: (params) =>
    apiClient.get('/admin/product-sales-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getTaxReport: (params) =>
    apiClient.get('/admin/tax-report', { params }),

  exportTaxReport: (params) =>
    apiClient.get('/admin/tax-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getCashFlowReport: (params) =>
    apiClient.get('/admin/cash-flow-report', { params }),

  exportCashFlowReport: (params) =>
    apiClient.get('/admin/cash-flow-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getBalanceReport: (params) =>
    apiClient.get('/admin/balance-report', { params }),

  exportBalanceReport: (params) =>
    apiClient.get('/admin/balance-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  getProfitLossReport: (params) =>
    apiClient.get('/admin/profit-loss-report', { params }),

  exportProfitLossReport: (params) =>
    apiClient.get('/admin/profit-loss-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),

  // Buku besar mengembalikan satu entri per transaksi, jadi rentang sebulan
  // bisa puluhan ribu baris — timeout default 15s terlalu pendek.
  getGeneralLedger: (params) =>
    apiClient.get('/admin/general-ledger', { params, timeout: 60000 }),

  exportGeneralLedger: (params) =>
    apiClient.get('/admin/general-ledger/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    }),
}
