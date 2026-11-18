/**
 * src/api/admins.js — Admin user management API calls
 *
 * Endpoints used:
 *   GET  /admin/admins
 *   POST /admin/admins
 *
 * AI NOTE: To add edit/delete admin endpoints, first implement them in
 * the Go handlers (cloud-pos/handlers/handlers.go) then add
 * the corresponding calls here.
 */

import { apiClient } from './client.js'

export const adminsApi = {
  /** List all admin users */
  list: () =>
    apiClient.get('/admin/admins'),

  /**
   * Create a new admin user.
   * @param {{ username: string, password: string, name: string, role?: string }} payload
   */
  create: (payload) =>
    apiClient.post('/admin/admins', payload),

  /**
   * Update admin role.
   * @param {string} id
   * @param {{ role: string }} payload
   */
  updateRole: (id, payload) =>
    apiClient.put(`/admin/admins/${id}/role`, payload),

  /**
   * Update permissions for a role.
   * @param {string} role
   * @param {{ permissions: string[] }} payload
   */
  updatePermissions: (role, payload) =>
    apiClient.put(`/admin/permissions/${role}`, payload),

  /** Get current user's permissions */
  getMyPermissions: () =>
    apiClient.get('/admin/me/permissions'),

  /** List all roles with permissions */
  listRoles: () =>
    apiClient.get('/admin/roles'),

  roleNotificationOptions: () => apiClient.get('/admin/roles/notification-options'),
  updateRoleNotifications: (role, policy) => apiClient.put(`/admin/roles/${encodeURIComponent(role)}/notifications`, policy),
  resetRoleNotifications: (role) => apiClient.delete(`/admin/roles/${encodeURIComponent(role)}/notifications`),

  /** Create a new custom role */
  createRole: (payload) =>
    apiClient.post('/admin/roles', payload),

  /** Delete a custom role */
  deleteRole: (name) =>
    apiClient.delete(`/admin/roles/${name}`),

  /** Update a role's name and description */
  updateRole: (name, payload) =>
    apiClient.put(`/admin/roles/${name}`, payload),

  /** Update role scope */
  updateRoleScope: (role, payload) =>
    apiClient.put(`/admin/roles/${role}/scope`, payload),

  /** Update admin (name + role) */
  updateAdmin: (id, payload) =>
    apiClient.put(`/admin/admins/${id}`, payload),

  /** Reset admin password */
  resetPassword: (id, newPassword) =>
    apiClient.put(`/admin/admins/${id}/reset-password`, { new_password: newPassword }),

  /** Toggle admin active status */
  toggleActive: (id) =>
    apiClient.post(`/admin/admins/${id}/toggle`),

  /** Delete admin */
  deleteAdmin: (id) =>
    apiClient.delete(`/admin/admins/${id}`),
}
