import { apiClient } from './client.js'

export const documentsApi = {
  list: (params) => apiClient.get('/admin/documents', { params }),
  upload: (number, about, file, onProgress) => {
    const data = new FormData()
    data.append('document_number', number)
    data.append('about', about)
    data.append('file', file)
    return apiClient.post('/admin/documents', data, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 0,
      onUploadProgress: (event) => {
        if (event.total) onProgress?.(Math.round(event.loaded / event.total * 100))
      },
    })
  },
  download: (id) => apiClient.post(`/admin/documents/${encodeURIComponent(id)}/download`, null, {
    responseType: 'blob', timeout: 0,
  }),
}
