import { apiClient } from './client.js'

export const projectMaterialsApi = {
  list:   (projectId)             => apiClient.get(`/admin/projects/${projectId}/materials`),
  logs:   (projectId, materialId) => apiClient.get(`/admin/projects/${projectId}/materials/${materialId}/logs`),
  usage:  (projectId, materialId, data) => apiClient.post(`/admin/projects/${projectId}/materials/${materialId}/usage`, data),
  settle: (projectId, materialId, data) => apiClient.post(`/admin/projects/${projectId}/materials/${materialId}/settle`, data),
  unsettled: () => apiClient.get('/admin/project-materials/unsettled'),
}
