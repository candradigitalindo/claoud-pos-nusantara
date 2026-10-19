/**
 * src/stores/auth.js — Authentication state (Pinia store)
 *
 * Responsibilities:
 *   - Store JWT token + admin info in localStorage (persisted across tabs)
 *   - Expose login / logout actions
 *   - Provide computed `isAuthenticated` + `currentAdmin` getters
 *
 * Token lifecycle:
 *   login()  → calls POST /api/v1/admin/login → stores token
 *   logout() → clears token + redirects to /login
 *
 * AI NOTE: To add refresh-token logic, add a `refreshToken` action
 * that calls a /api/v1/admin/refresh endpoint and updates `token`.
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth.js'
import { adminsApi } from '@/api/admins.js'

// Storage key used in localStorage
const TOKEN_KEY  = 'cloud_pos_token'
const ADMIN_KEY  = 'cloud_pos_admin'
const PERMS_KEY  = 'cloud_pos_permissions'
const SCOPE_KEY  = 'cloud_pos_scope'
const SCOPE_IDS_KEY = 'cloud_pos_scope_outlet_ids'
const REDIRECT_KEY = 'cloud_pos_redirect_to'

export const useAuthStore = defineStore('auth', () => {
  // ── State ────────────────────────────────────────────────
  const token = ref(localStorage.getItem(TOKEN_KEY) || '')
  const admin = ref(JSON.parse(localStorage.getItem(ADMIN_KEY) || 'null'))
  const permissions = ref(JSON.parse(localStorage.getItem(PERMS_KEY) || '[]'))
  const scopeType = ref(localStorage.getItem(SCOPE_KEY) || 'all')
  const scopeOutletIDs = ref(JSON.parse(localStorage.getItem(SCOPE_IDS_KEY) || 'null'))
  const redirectTo = ref(localStorage.getItem(REDIRECT_KEY) || '/')
  const permsSynced = ref(false)

  // ── Getters ──────────────────────────────────────────────
  /** True when a valid token exists */
  const isAuthenticated = computed(() => !!token.value)

  /** The currently logged-in admin object */
  const currentAdmin = computed(() => admin.value)

  /** True when the logged-in admin is the superadmin (master account) */
  const isSuperadmin = computed(() => admin.value?.role === 'superadmin')

  function hasPermission(perm) {
    // Bentuk array = lolos bila punya SALAH SATU izin (halaman multi-tab
    // seperti Void & Titipan yang tiap tab-nya punya izin sendiri).
    if (Array.isArray(perm)) return perm.some(p => hasPermission(p))
    if (permissions.value.includes(perm)) return true
    // .manage (legacy) → true if user has any CRUD perm (.create/.update/.delete) for the module
    if (perm.endsWith('.manage')) {
      const module = perm.replace('.manage', '')
      return permissions.value.some(p => p.startsWith(module + '.') && p !== module + '.view')
    }
    // .view → true if user has any sub-permission for the module
    if (perm.endsWith('.view')) {
      const module = perm.replace('.view', '')
      return permissions.value.some(p => p.startsWith(module + '.'))
    }
    return false
  }

  /** Check if current user has access to a specific outlet ID.
   *  Returns true for "all" scope, or checks against the scoped outlet IDs. */
  function hasOutletAccess(outletID) {
    if (scopeType.value !== 'specific') return true
    if (!scopeOutletIDs.value) return true
    return scopeOutletIDs.value.includes(outletID)
  }

  // ── Actions ──────────────────────────────────────────────

  /**
   * login — Authenticate admin credentials against the API.
   * @param {string} username
   * @param {string} password
   * @throws {Error} when credentials are invalid
   */
  async function login(username, password) {
    const data = await authApi.login(username, password)
    token.value = data.token
    admin.value = data.admin
    permissions.value = data.permissions ?? []
    scopeType.value = data.scope_type ?? 'all'
    scopeOutletIDs.value = data.scope_outlet_ids ?? null
    redirectTo.value = data.redirect_to || '/'
    localStorage.setItem(TOKEN_KEY, data.token)
    localStorage.setItem(ADMIN_KEY, JSON.stringify(data.admin))
    localStorage.setItem(PERMS_KEY, JSON.stringify(data.permissions ?? []))
    localStorage.setItem(SCOPE_KEY, data.scope_type ?? 'all')
    localStorage.setItem(SCOPE_IDS_KEY, JSON.stringify(data.scope_outlet_ids ?? null))
    localStorage.setItem(REDIRECT_KEY, data.redirect_to || '/')
    // Login response already carries the full, fresh permission set — mark as
    // synced so the first post-login navigation does not depend on a second
    // /me/permissions call (which can be challenged by Cloudflare → 403/race).
    permsSynced.value = true
  }

  /**
   * setPermissions — Update permissions (e.g. after role change or refresh).
   */
  function setPermissions(perms) {
    permissions.value = perms ?? []
    localStorage.setItem(PERMS_KEY, JSON.stringify(perms ?? []))
  }

  /**
   * logout — Clear session and navigate to /login.
   * Called by the UI on manual logout or on 401 response from API.
   */
  function logout() {
    const apiBase = import.meta.env.VITE_API_BASE_URL || '/api/v1'
    fetch(`${apiBase}/cctv/logout`, { method: 'POST', credentials: 'include', keepalive: true }).catch(() => {})
    token.value = ''
    admin.value = null
    permissions.value = []
    scopeType.value = 'all'
    scopeOutletIDs.value = null
    redirectTo.value = '/'
    permsSynced.value = false
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(ADMIN_KEY)
    localStorage.removeItem(PERMS_KEY)
    localStorage.removeItem(SCOPE_KEY)
    localStorage.removeItem(SCOPE_IDS_KEY)
    localStorage.removeItem(REDIRECT_KEY)
  }

  /**
   * fetchPermissions — Load permissions from API for existing session.
   * Called on app init when user is authenticated but permissions are empty.
   */
  let _permPromise = null
  async function fetchPermissions() {
    if (_permPromise) return _permPromise
    _permPromise = adminsApi.getMyPermissions()
      .then(data => {
        const perms = Array.isArray(data) ? data : data.permissions ?? []
        setPermissions(perms)
        permsSynced.value = true
        if (data.scope_type) {
          scopeType.value = data.scope_type
          scopeOutletIDs.value = data.scope_outlet_ids ?? null
          localStorage.setItem(SCOPE_KEY, data.scope_type)
          localStorage.setItem(SCOPE_IDS_KEY, JSON.stringify(data.scope_outlet_ids ?? null))
        }
      })
      .catch(() => { /* ignore — user will see empty sidebar and can re-login */ })
      .finally(() => { _permPromise = null })
    return _permPromise
  }

  return {
    token,
    admin,
    permissions,
    scopeType,
    scopeOutletIDs,
    redirectTo,
    isAuthenticated,
    currentAdmin,
    isSuperadmin,
    hasPermission,
    hasOutletAccess,
    login,
    setPermissions,
    fetchPermissions,
    logout,
    permsSynced,
  }
})
