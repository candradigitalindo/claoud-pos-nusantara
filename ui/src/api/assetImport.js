import { apiClient } from './client.js'

export const assetImportApi = {
  template: () => apiClient.get('/admin/asset-import/template', { responseType: 'blob', timeout: 120000 }),
  upload: (file, mode = 'preview') => {
    const fd = new FormData()
    fd.append('file', file)
    return apiClient.post(`/admin/asset-import?mode=${mode}`, fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 180000,
    })
  },
}
