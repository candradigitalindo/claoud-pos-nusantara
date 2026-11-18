import { apiClient } from './client.js'

export const appFilesApi = {
  list: () => apiClient.get('/admin/app-files'),
  // Versi aplikasi di tablet tiap outlet (dari heartbeat) vs APK POS terbaru.
  devices: () => apiClient.get('/admin/app-files/devices'),
  upload: (file, onProgress) => {
    const fd = new FormData()
    fd.append('file', file)
    return apiClient.post('/admin/app-files', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 0, // large uploads must not be cut off by the default 15s axios timeout
      onUploadProgress: (e) => { if (onProgress && e.total) onProgress(Math.round((e.loaded / e.total) * 100)) },
    })
  },
  remove: (name) => apiClient.delete(`/admin/app-files/${encodeURIComponent(name)}`),
}
