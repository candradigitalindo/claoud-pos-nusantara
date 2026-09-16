<template>
  <div class="rp">

    <!-- ── Header ── -->
    <div class="rp-hd">
      <div>
        <h1 class="rp-title">Role & Hak Akses</h1>
        <p class="rp-sub">Kelola role dan atur wewenang setiap pengguna di sistem.</p>
      </div>
      <button class="btn-new" @click="copyingFrom = ''; showCreate = true">
        <svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"/></svg>
        Tambah Role
      </button>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- ── Loading ── -->
    <div v-if="loading" class="rp-loading">
      <AppSpinner size="lg" class="text-indigo-500" />
    </div>

    <!-- ── Roles list ── -->
    <div v-else class="roles-list">
      <div
        v-for="role in roles"
        :key="role.name"
        class="role-card"
        :class="{ 'role-card--open': expandedRole === role.name }"
      >
        <!-- Card summary -->
        <div class="role-summary" @click="expandedRole = expandedRole === role.name ? '' : role.name">
          <div class="role-avatar" :class="role.is_system ? 'avatar--sys' : 'avatar--custom'">
            {{ role.name.charAt(0).toUpperCase() }}
          </div>

          <div class="role-meta">
            <div class="role-nameline">
              <span class="role-name">{{ role.name }}</span>
              <span v-if="role.is_system" class="badge-sys">Sistem</span>
            </div>
            <p class="role-desc">{{ role.description || 'Tidak ada deskripsi' }}</p>
            <div class="role-chips">
              <span class="chip chip--scope" :class="(roleScopes[role.name]?.scope_type ?? role.scope_type) !== 'all' ? 'chip--sky' : ''">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"/></svg>
                {{ (roleScopes[role.name]?.scope_type ?? role.scope_type) === 'all' ? 'Semua Unit Kerja' : 'Unit Kerja Tertentu' }}
              </span>
              <span class="chip chip--perm">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
                <template v-if="role.is_system">Akses Penuh</template>
                <template v-else>{{ countPerms(role.name) }} hak akses aktif</template>
              </span>
              <span class="chip chip--redirect">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1"/></svg>
                Login → {{ getRedirectLabel(role.redirect_to) }}
              </span>
            </div>
          </div>

          <div class="role-actions" @click.stop>
            <button class="act-btn act-btn--edit" title="Edit Role" @click.stop="openEditRole(role)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
            </button>
            <button class="act-btn act-btn--copy" title="Duplikat Role" @click.stop="openCopyRole(role)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="11" height="11" rx="2"/><path stroke-linecap="round" stroke-linejoin="round" d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
            </button>
            <button
              class="act-btn"
              :class="role.is_system ? 'act-btn--disabled' : 'act-btn--del'"
              :title="role.is_system ? 'Role sistem tidak dapat dihapus' : 'Hapus Role'"
              @click.stop="!role.is_system && confirmDelete(role)"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </button>
            <button class="act-expand" :class="{ 'act-expand--open': expandedRole === role.name }" @click="expandedRole = expandedRole === role.name ? '' : role.name">
              <svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
              {{ expandedRole === role.name ? 'Tutup' : 'Kelola Akses' }}
            </button>
          </div>
        </div>

        <!-- Expanded panel -->
        <div v-show="expandedRole === role.name" class="role-panel">

          <div v-if="role.is_system" class="sys-notice">
            <svg width="16" height="16" viewBox="0 0 20 20" fill="currentColor" class="sys-notice-icon"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"/></svg>
            <div>
              <strong>Role Sistem (Read-only)</strong> — Role <em>{{ role.name }}</em> memiliki akses penuh ke semua fitur secara bawaan dan tidak dapat diubah.
            </div>
          </div>

          <div class="panel-body" :class="{ 'panel-body--dim': role.is_system || savingRole === role.name }">

            <!-- Scope column -->
            <div class="scope-col">
              <div class="col-label">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"/></svg>
                Batasan Unit Kerja
              </div>
              <p class="col-hint">Tentukan unit kerja yang dapat diakses pengguna dengan role ini.</p>

              <div class="scope-opts">
                <label class="scope-opt" :class="{ 'scope-opt--on': (roleScopes[role.name]?.scope_type ?? role.scope_type) === 'all' }">
                  <input type="radio" :name="'sc_'+role.name" value="all"
                    :checked="(roleScopes[role.name]?.scope_type ?? role.scope_type) === 'all'"
                    @change="updateScope(role.name,'all',[])" />
                  <div>
                    <span>Semua Unit Kerja</span>
                    <small>Akses ke seluruh outlet &amp; unit kerja</small>
                  </div>
                </label>
                <label class="scope-opt" :class="{ 'scope-opt--on': (roleScopes[role.name]?.scope_type ?? role.scope_type) === 'specific' }">
                  <input type="radio" :name="'sc_'+role.name" value="specific"
                    :checked="(roleScopes[role.name]?.scope_type ?? role.scope_type) === 'specific'"
                    @change="showScopeModal(role.name)" />
                  <div>
                    <span>Unit Kerja Tertentu</span>
                    <small>Pilih unit kerja secara spesifik</small>
                  </div>
                </label>

                <div v-if="(roleScopes[role.name]?.scope_type ?? role.scope_type) === 'specific'" class="scope-tags">
                  <span v-for="wuId in (roleScopes[role.name]?.work_unit_ids ?? [])" :key="wuId" class="scope-tag">
                    {{ getWorkUnitName(wuId) }}
                  </span>
                  <span v-if="!(roleScopes[role.name]?.work_unit_ids?.length)" class="scope-tag-empty">Belum ada unit dipilih</span>
                  <button class="scope-edit-btn" @click="showScopeModal(role.name)">Ubah Pilihan</button>
                </div>
              </div>
            </div>

            <!-- Permissions column -->
            <div class="perm-col">
              <div class="col-label">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
                Hak Akses
                <span v-if="savingRole === role.name" class="saving-dot">● Menyimpan…</span>
              </div>
              <p class="col-hint">Klik pill untuk mengaktifkan atau menonaktifkan hak akses. Perubahan otomatis tersimpan.</p>

              <PermissionMatrix :permissions="rolePermissions[role.name] ?? []" :disabled="role.is_system" @toggle="(k) => togglePermission(role.name, k)" />
            </div>

          </div><!-- /panel-body -->
        </div><!-- /role-panel -->
      </div>

      <div v-if="roles.length === 0" class="empty-state">
        <div class="empty-icon">
          <svg width="44" height="44" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 00-4-4H7a4 4 0 00-4 4v2"/><circle cx="10" cy="7" r="4"/><path d="M21 21v-2a4 4 0 00-3-3.87"/><path d="M16 3.13a4 4 0 010 7.75"/></svg>
        </div>
        <h3>Belum ada role</h3>
        <p>Klik "Tambah Role" untuk membuat peran baru.</p>
      </div>
    </div>

    <!-- ══════════════════════════════════════
         MODAL — Tambah Role
    ══════════════════════════════════════ -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showCreate" class="modal-backdrop" @click.self="showCreate = false">
          <div class="modal modal--lg">
            <div class="modal-hd">
              <span class="modal-title">{{ copyingFrom ? 'Duplikat Role' : 'Tambah Role Baru' }}</span>
              <button class="modal-close" @click="showCreate = false">✕</button>
            </div>
            <div class="modal-bd">
              <div v-if="copyingFrom" class="copy-note">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="11" height="11" rx="2"/><path stroke-linecap="round" stroke-linejoin="round" d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
                Disalin dari <b>{{ copyingFrom }}</b> — hak akses sudah ikut tersalin. Cukup ubah nama &amp; unit kerja.
              </div>
              <AppAlert type="error" :message="createError" />

              <div class="form-row">
                <AppInput v-model="form.name" label="Nama Role" placeholder="contoh: kasir" :error="formErrors.name" />
                <AppInput v-model="form.description" label="Deskripsi (opsional)" placeholder="Deskripsi singkat role" />
              </div>

              <div class="form-group">
                <label class="form-label">Halaman Setelah Login</label>
                <SearchSelect v-model="form.redirect_to" :options="REDIRECT_SEARCH_OPTIONS" placeholder="Cari halaman tujuan..." />
                <p v-if="!redirectAllowed(form.permissions, form.redirect_to)" class="form-hint" style="color:#b45309">
                  ⚠ Role ini belum punya izin untuk halaman tersebut — centang izinnya di Hak Akses di bawah, atau user akan mendarat di halaman "Akses Ditolak".
                </p>
                <p v-else class="form-hint">Semua halaman sidebar tersedia — ketik untuk mencari.</p>
              </div>

              <!-- Scope -->
              <div class="form-section">
                <div class="form-section-title">🏢 Scope Unit Kerja</div>
                <div class="scope-row">
                  <label class="scope-opt" :class="{ 'scope-opt--on': form.scope_type === 'all' }">
                    <input type="radio" name="create_scope" value="all" v-model="form.scope_type" />
                    <div><span>Semua Unit Kerja</span><small>Akses semua outlet & unit kerja</small></div>
                  </label>
                  <label class="scope-opt" :class="{ 'scope-opt--on': form.scope_type === 'specific' }">
                    <input type="radio" name="create_scope" value="specific" v-model="form.scope_type" />
                    <div><span>Unit Kerja Tertentu</span><small>Pilih unit kerja spesifik</small></div>
                  </label>
                </div>
                <div v-if="form.scope_type === 'specific'" class="wu-list">
                  <label v-for="wu in workUnits" :key="wu.id" class="wu-item">
                    <input type="checkbox" :checked="form.work_unit_ids.includes(wu.id)" @change="toggleFormWorkUnit(wu.id)" />
                    <div><span>{{ wu.name }}</span><small v-if="wu.outlet_name">{{ wu.outlet_name }}</small></div>
                  </label>
                  <p v-if="workUnits.length === 0" class="wu-empty">Belum ada unit kerja.</p>
                </div>
              </div>

              <!-- Permissions matrix -->
              <div class="form-section">
                <div class="form-section-title"><svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="display:inline-block;vertical-align:-2px;margin-right:4px"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>Hak Akses</div>

                <PermissionMatrix :permissions="form.permissions" @toggle="toggleFormPermission" />
              </div>
            </div>
            <div class="modal-ft">
              <button class="btn-cancel" @click="showCreate = false">Batal</button>
              <button class="btn-submit" :disabled="creating" @click="createRole">
                <span v-if="creating" class="spinner-sm" />
                Buat Role
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ══════════════════════════════════════
         MODAL — Edit Role
    ══════════════════════════════════════ -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showEdit" class="modal-backdrop" @click.self="showEdit = false">
          <div class="modal modal--sm">
            <div class="modal-hd">
              <span class="modal-title">Edit Role</span>
              <button class="modal-close" @click="showEdit = false">✕</button>
            </div>
            <div class="modal-bd">
              <AppAlert type="error" :message="editError" />
              <div class="space-y-4">
                <AppInput v-model="editForm.name" label="Nama Role" placeholder="contoh: kasir" :error="editFormErrors.name" :disabled="editForm.isSystem" />
                <p v-if="editForm.isSystem" class="text-xs text-gray-400 -mt-2">Nama role sistem tidak dapat diubah.</p>
                <AppInput v-model="editForm.description" label="Deskripsi" placeholder="Deskripsi singkat role" />
                <div class="form-group">
                  <label class="form-label">Halaman Setelah Login</label>
                  <SearchSelect v-model="editForm.redirect_to" :options="REDIRECT_SEARCH_OPTIONS" placeholder="Cari halaman tujuan..." />
                  <p v-if="!editForm.isSystem && !redirectAllowed(rolePermissions[editForm.originalName] ?? [], editForm.redirect_to)" class="form-hint" style="color:#b45309">
                    ⚠ Role ini belum punya izin untuk halaman tersebut — user akan mendarat di halaman "Akses Ditolak".
                  </p>
                </div>
              </div>
            </div>
            <div class="modal-ft">
              <button class="btn-cancel" @click="showEdit = false">Batal</button>
              <button class="btn-submit" :disabled="editing" @click="submitEditRole">
                <span v-if="editing" class="spinner-sm" />
                Simpan
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ══════════════════════════════════════
         MODAL — Hapus Role
    ══════════════════════════════════════ -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showDeleteConfirm" class="modal-backdrop" @click.self="showDeleteConfirm = false">
          <div class="modal modal--sm">
            <div class="modal-hd modal-hd--danger">
              <span class="modal-title">Hapus Role</span>
              <button class="modal-close" @click="showDeleteConfirm = false">✕</button>
            </div>
            <div class="modal-bd" style="text-align:center;padding-top:1.5rem">
              <div class="del-icon-wrap">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
              </div>
              <p class="del-confirm-text">
                Yakin ingin menghapus role <strong class="del-target">{{ deleteTarget?.name }}</strong>?<br/>
                Semua hak akses untuk role ini akan hilang.
              </p>
            </div>
            <div class="modal-ft">
              <button class="btn-cancel" @click="showDeleteConfirm = false">Batal</button>
              <button class="btn-danger" :disabled="!!deletingRole" @click="doDelete">
                <span v-if="deletingRole" class="spinner-sm" />
                Hapus Permanen
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ══════════════════════════════════════
         MODAL — Scope Unit Kerja
    ══════════════════════════════════════ -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showScopeEdit" class="modal-backdrop" @click.self="showScopeEdit = false">
          <div class="modal modal--sm">
            <div class="modal-hd">
              <span class="modal-title">Pilih Unit Kerja</span>
              <button class="modal-close" @click="showScopeEdit = false">✕</button>
            </div>
            <div class="modal-bd">
              <p class="text-sm text-gray-600 mb-3">Pilih unit kerja yang dapat diakses oleh role <strong class="capitalize">{{ scopeEditRole }}</strong>:</p>
              <div class="wu-list">
                <label v-for="wu in workUnits" :key="wu.id" class="wu-item">
                  <input type="checkbox" :checked="scopeEditIDs.includes(wu.id)" @change="toggleScopeEditWU(wu.id)" />
                  <div>
                    <span>{{ wu.name }}</span>
                    <small v-if="wu.outlet_name">{{ wu.outlet_name }}</small>
                  </div>
                </label>
                <p v-if="workUnits.length === 0" class="wu-empty">Belum ada unit kerja.</p>
              </div>
            </div>
            <div class="modal-ft">
              <button class="btn-cancel" @click="showScopeEdit = false">Batal</button>
              <button class="btn-submit" :disabled="savingScope === scopeEditRole" @click="saveScopeEdit">
                <span v-if="savingScope === scopeEditRole" class="spinner-sm" />
                Simpan Perubahan
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { adminsApi }    from '@/api/admins.js'
import { workUnitsApi } from '@/api/workUnits.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore }  from '@/stores/auth.js'
import AppInput   from '@/components/ui/AppInput.vue'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import PermissionMatrix from '@/components/PermissionMatrix.vue'

const toast     = useToastStore()
const authStore = useAuthStore()

// ── Static config ────────────────────────────────────

// Post-login redirect targets, grouped to mirror the app sidebar. Each option
// carries the permission required to view that page so the dropdown only offers
// pages the role can actually land on.
// Susunan & urutan mengikuti sidebar (DashboardLayout NAV_ITEMS) — setiap
// kategori minimal memuat submenu PERTAMA-nya sebagai target landing.
const REDIRECT_GROUPS = [
  { category: 'Umum', options: [
    { value: '/', label: 'Dashboard', perm: 'dashboard' },
  ]},
  { category: 'Master Data', options: [
    { value: '/outlets',      label: 'Outlet',       perm: 'outlets.view' },
    { value: '/work-units',   label: 'Unit Kerja',   perm: 'workunits.view' },
    { value: '/warehouses',   label: 'Gudang',       perm: 'warehouses.view' },
    { value: '/roles',        label: 'Role & Hak Akses', perm: 'roles.view' },
    { value: '/app-pos',      label: 'App POS',      perm: 'appfiles.view' },
  ]},
  { category: 'Perlengkapan', options: [
    { value: '/perlengkapan',        label: 'Daftar Aset',         perm: 'assets.view' },
    { value: '/perlengkapan/penerimaan', label: 'Penerimaan Peralatan', perm: 'assets.create' },
    { value: '/perlengkapan/distribusi', label: 'Distribusi Aset ke PIC', perm: 'assets.view' },
    { value: '/perlengkapan/mutasi', label: 'Mutasi Antar Outlet', perm: 'assets.transfer.view' },
    { value: '/perlengkapan/perawatan', label: 'Perawatan Aset', perm: 'assets.maintenance.view' },
    { value: '/perlengkapan/opname', label: 'Opname Aset', perm: 'assets.opname.view' },
    { value: '/perlengkapan/penghapusan', label: 'Penghapusan Aset', perm: 'assets.disposal.view' },
    { value: '/perlengkapan/laporan', label: 'Laporan Aset', perm: 'assets.report.view' },
  ]},
  { category: 'Produk', options: [
    { value: '/products', label: 'Produk & Kategori', perm: 'products.view' },
  ]},
  { category: 'Penjualan', options: [
    { value: '/reservations', label: 'Reservasi', perm: 'reservations.view' },
    { value: '/customers',    label: 'Pelanggan', perm: 'customers.view' },
  ]},
  { category: 'Laporan', options: [
    { value: '/sales-report',          label: 'Laporan Pendapatan',       perm: 'reports.sales.view' },
    { value: '/cashier-shifts',        label: 'Shift Kasir',              perm: 'cashier_shifts.view' },
    { value: '/shift-reconciliation',  label: 'Rekonsiliasi Shift',       perm: 'shift_reconciliation.view' },
    { value: '/procurement-payments',  label: 'Pembayaran',               perm: 'finance.payments.view' },
    { value: '/product-sales-report',  label: 'Penjualan Produk',         perm: 'reports.product_sales.view' },
    { value: '/general-ledger',        label: 'Buku Besar',               perm: 'reports.ledger.view' },
    { value: '/cash-flow-report',      label: 'Pemasukan & Pengeluaran',  perm: 'reports.cashflow.view' },
    { value: '/profit-loss-report',    label: 'Profit & Loss',            perm: 'reports.pnl.view' },
    { value: '/balance-report',        label: 'Laporan Neraca',           perm: 'reports.balance.view' },
    { value: '/tax-report',            label: 'Laporan Pajak',            perm: 'reports.tax.view' },
    { value: '/void-report',           label: 'Void & Titipan',           perm: 'reports.void.view' },
    { value: '/discount-report',       label: 'Diskon & Komplimen',       perm: 'reports.discount.view' },
    { value: '/business-analysis',     label: 'Analisa Bisnis',           perm: 'reports.business_analysis.view' },
    { value: '/social-performance',    label: 'Kinerja Markom',           perm: 'social.view' },
    { value: '/bank-accounts',         label: 'Data Rekening',            perm: 'finance.bank.view' },
  ]},
  { category: 'Pengadaan', options: [
    { value: '/procurement-dashboard', label: 'Dashboard Pengadaan', perm: 'procurement.dashboard.view' },
    { value: '/purchase-goods',        label: 'Pengadaan Barang',    perm: 'procurement.requests.view' },
    { value: '/purchase-services',     label: 'Pengadaan Jasa',      perm: 'procurement.requests.view' },
    { value: '/projects',              label: 'Projek',              perm: 'procurement.projects.view' },
    { value: '/vendors',               label: 'Vendor',              perm: 'vendors.view' },
  ]},
  { category: 'PPIC', options: [
    { value: '/ppic/dashboard',        label: 'Dashboard PPIC',        perm: 'ppic.dashboard.view' },
    { value: '/ppic/planning-params',  label: 'Par Level & ROP',       perm: 'ppic.planning.view' },
    { value: '/ppic/expiry',           label: 'Monitor Kedaluwarsa',   perm: 'ppic.expiry.view' },
    { value: '/ppic/opname',           label: 'Stock Opname',          perm: 'ppic.opname.view' },
    { value: '/ppic/forecast',         label: 'Demand Forecast',       perm: 'ppic.forecast.view' },
    { value: '/ppic/mrp',              label: 'Kebutuhan Bahan (MRP)', perm: 'ppic.mrp.view' },
    { value: '/ppic/production-plans', label: 'Rencana Produksi',      perm: 'ppic.production.view' },
    { value: '/ppic/work-orders',      label: 'Work Order',            perm: 'ppic.workorders.view' },
    { value: '/ppic/reports',          label: 'Laporan PPIC',          perm: 'ppic.reports.view' },
  ]},
  { category: 'Pengguna', options: [
    { value: '/admins', label: 'User (Admin)', perm: 'users.view' },
  ]},
  { category: 'Gudang', options: [
    { value: '/warehouse-dashboard', label: 'Dashboard Gudang',     perm: 'warehouse_dashboard.view' },
    { value: '/stock-items',         label: 'Item Stok',            perm: 'stockitems.view' },
    { value: '/goods-receipts',      label: 'Penerimaan Barang',    perm: 'stockledger.view' },
    { value: '/stock-transfers',     label: 'Transfer Stok',        perm: 'stocktransfers.view' },
    { value: '/stock-wastes',        label: 'Stok Rusak/Hilang',    perm: 'stockwastes.view' },
    { value: '/stock-ledger',        label: 'Buku Stok',            perm: 'stockledger.view' },
    { value: '/recipes',             label: 'Resep',                perm: 'recipes.view' },
  ]},
  { category: 'Pengaturan', options: [
    { value: '/settings/company',  label: 'Identitas Perusahaan', perm: 'settings.company.view' },
    { value: '/settings/timezone', label: 'Zona Waktu',           perm: 'settings.timezone.view' },
    { value: '/settings/tax',      label: 'Pajak',                perm: 'settings.tax.view' },
    { value: '/settings/devices',  label: 'Perangkat',            perm: 'devices.view' },
  ]},
  { category: 'Log Akses', options: [
    { value: '/access-logs', label: 'Log Akses', perm: 'access_logs.view' },
  ]},
]
const REDIRECT_OPTIONS = REDIRECT_GROUPS.flatMap(g => g.options)

// Semua halaman sidebar sebagai opsi SearchSelect (bisa dicari), berlabel
// "Kategori › Halaman". Tidak difilter izin — bila role belum punya izin
// halaman terpilih, form menampilkan peringatan (bukan menyembunyikan opsi).
const REDIRECT_SEARCH_OPTIONS = REDIRECT_GROUPS.flatMap(g =>
  g.options.map(o => ({ id: o.value, name: `${g.category} › ${o.label}` }))
)

// Does `perms` grant access to a page needing `perm`? Mirrors backend RequirePermission:
// having any sub-permission of a module implies its `.view`. Dashboard is always allowed.
function pageAllowed(perms, perm) {
  if (!perm || perm === 'dashboard') return true
  if (!perms || !perms.length) return false
  if (perms.includes(perm)) return true
  if (perm.endsWith('.view')) {
    const mod = perm.slice(0, -'.view'.length)
    return perms.some(p => p.startsWith(mod + '.'))
  }
  return false
}

// Apakah daftar izin boleh membuka halaman redirect `path`?
function redirectAllowed(perms, path) {
  const opt = REDIRECT_OPTIONS.find(o => o.value === path)
  return pageAllowed(perms, opt?.perm)
}

// ── State ────────────────────────────────────────────

const roles           = ref([])
const rolePermissions = ref({})
const roleScopes      = ref({})
const allPermissions  = ref([])
const workUnits       = ref([])
const loading         = ref(false)
const errorMsg        = ref('')
const savingRole      = ref('')
const savingScope     = ref('')
const expandedRole    = ref('')

const showCreate  = ref(false)
const creating    = ref(false)
const createError = ref('')
const copyingFrom = ref('')   // source role name when duplicating
const form        = reactive({ name: '', description: '', permissions: [], scope_type: 'all', work_unit_ids: [], redirect_to: '/' })
const formErrors  = reactive({ name: '' })

const showEdit       = ref(false)
const editing        = ref(false)
const editError      = ref('')
const editForm       = reactive({ name: '', description: '', originalName: '', isSystem: false, redirect_to: '/' })
const editFormErrors = reactive({ name: '' })

const showDeleteConfirm = ref(false)
const deleteTarget      = ref(null)
const deletingRole      = ref('')

const showScopeEdit = ref(false)
const scopeEditRole = ref('')
const scopeEditIDs  = ref([])

// Dropdown redirect memuat SEMUA halaman sidebar (REDIRECT_SEARCH_OPTIONS);
// bila pilihan tidak sesuai izin role, form menampilkan peringatan inline —
// tidak lagi me-reset paksa ke Dashboard.

// ── Helpers ──────────────────────────────────────────

function getWorkUnitName(id) {
  return workUnits.value.find(w => w.id === id)?.name ?? id
}

function getRedirectLabel(path) {
  return REDIRECT_OPTIONS.find(o => o.value === path)?.label ?? (path || 'Dashboard')
}

function countPerms(roleName) {
  return (rolePermissions.value[roleName] ?? []).length
}

function roleHas(roleName, key) {
  return (rolePermissions.value[roleName] ?? []).includes(key)
}

function toggleScopeEditWU(id) {
  const idx = scopeEditIDs.value.indexOf(id)
  if (idx >= 0) scopeEditIDs.value.splice(idx, 1)
  else scopeEditIDs.value.push(id)
}

function toggleFormWorkUnit(id) {
  const idx = form.work_unit_ids.indexOf(id)
  if (idx >= 0) form.work_unit_ids.splice(idx, 1)
  else form.work_unit_ids.push(id)
}

function showScopeModal(roleName) {
  scopeEditRole.value = roleName
  scopeEditIDs.value  = [...(roleScopes.value[roleName]?.work_unit_ids ?? [])]
  showScopeEdit.value = true
}

// ── Actions ──────────────────────────────────────────

async function updateScope(roleName, type, wuIDs) {
  savingScope.value = roleName
  try {
    await adminsApi.updateRoleScope(roleName, { scope_type: type, work_unit_ids: wuIDs })
    roleScopes.value = { ...roleScopes.value, [roleName]: { scope_type: type, work_unit_ids: wuIDs } }
    toast.success(`Scope ${roleName} diperbarui`)
    if (roleName === authStore.admin?.role) await authStore.fetchPermissions()
  } catch (err) {
    toast.error(err?.message ?? 'Gagal memperbarui scope')
  } finally {
    savingScope.value = ''
  }
}

async function saveScopeEdit() {
  await updateScope(scopeEditRole.value, 'specific', scopeEditIDs.value)
  showScopeEdit.value = false
}

loadRoles()

async function loadRoles() {
  loading.value = true; errorMsg.value = ''
  try {
    const [rolesRes, wuRes] = await Promise.all([
      adminsApi.listRoles(),
      workUnitsApi.list().catch(() => []),
    ])
    roles.value           = rolesRes.roles ?? []
    rolePermissions.value = rolesRes.permissions ?? {}
    allPermissions.value  = rolesRes.all_permissions ?? []
    roleScopes.value      = rolesRes.scopes ?? {}
    workUnits.value       = Array.isArray(wuRes) ? wuRes : (wuRes?.data ?? wuRes ?? [])
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat data role.'
  } finally { loading.value = false }
}

// Antrean simpan per sesi: dua klik cepat menghasilkan dua PUT full-list yang bisa
// selesai tidak berurutan — response lama menimpa toggle yang lebih baru di DB.
let saveQueue = Promise.resolve()

async function togglePermission(role, perm) {
  if (roles.value.find(r => r.name === role)?.is_system) return

  const current = [...(rolePermissions.value[role] ?? [])]
  const has = current.includes(perm)
  let updated

  if (has) {
    updated = current.filter(p => p !== perm)
    if (perm.endsWith('.view')) {
      const mod = perm.replace('.view', '')
      updated = updated.filter(p => !p.startsWith(mod + '.'))
    }
    if (!perm.includes('.')) {
      updated = updated.filter(p => !p.startsWith(perm + '.'))
    }
  } else {
    updated = [...current, perm]
    if (!perm.endsWith('.view') && perm.includes('.')) {
      const mod = perm.substring(0, perm.lastIndexOf('.'))
      const viewKey = mod + '.view'
      if (allPermissions.value.includes(viewKey) && !updated.includes(viewKey)) updated.push(viewKey)
    }
  }

  rolePermissions.value = { ...rolePermissions.value, [role]: updated }
  savingRole.value = role
  saveQueue = saveQueue.then(async () => {
    try {
      // Kirim snapshot TERKINI (bukan `updated` saat klik) supaya toggle yang
      // menyusul selama request sebelumnya ikut tersimpan.
      await adminsApi.updatePermissions(role, { permissions: rolePermissions.value[role] ?? [] })
      toast.success(`Hak akses ${role} diperbarui`)
      if (role === authStore.admin?.role) await authStore.fetchPermissions()
    } catch (err) {
      rolePermissions.value = { ...rolePermissions.value, [role]: current }
      toast.error(err?.message ?? 'Gagal memperbarui hak akses')
    } finally {
      savingRole.value = ''
    }
  })
  await saveQueue
}

function toggleFormPermission(perm) {
  const idx = form.permissions.indexOf(perm)
  if (idx >= 0) {
    form.permissions.splice(idx, 1)
    if (perm.endsWith('.view')) {
      const mod = perm.replace('.view', '')
      for (let i = form.permissions.length - 1; i >= 0; i--) {
        if (form.permissions[i].startsWith(mod + '.')) form.permissions.splice(i, 1)
      }
    }
    if (!perm.includes('.')) {
      for (let i = form.permissions.length - 1; i >= 0; i--) {
        if (form.permissions[i].startsWith(perm + '.')) form.permissions.splice(i, 1)
      }
    }
  } else {
    form.permissions.push(perm)
    if (!perm.endsWith('.view') && perm.includes('.')) {
      const mod = perm.substring(0, perm.lastIndexOf('.'))
      const viewKey = mod + '.view'
      if (allPermissions.value.includes(viewKey) && !form.permissions.includes(viewKey)) form.permissions.push(viewKey)
    }
  }
}

async function createRole() {
  formErrors.name = form.name.trim() ? '' : 'Nama role wajib diisi'
  if (formErrors.name) return
  creating.value = true; createError.value = ''
  try {
    await adminsApi.createRole({
      name: form.name.trim(), description: form.description.trim(),
      permissions: form.permissions, scope_type: form.scope_type,
      work_unit_ids: form.scope_type === 'specific' ? form.work_unit_ids : [],
      redirect_to: form.redirect_to || '/',
    })
    toast.success('Role berhasil dibuat!')
    showCreate.value = false
    await loadRoles()
  } catch (err) {
    createError.value = err?.message ?? 'Gagal membuat role.'
  } finally { creating.value = false }
}

// Duplicate a role: prefill the create form with the source role's permissions and
// scope so the admin only needs to change the name & work units.
function openCopyRole(role) {
  copyingFrom.value = role.name
  form.name = `${role.name} copy`
  form.description = role.description || ''
  form.permissions = [...(rolePermissions.value[role.name] ?? [])]
  form.scope_type = roleScopes.value[role.name]?.scope_type ?? role.scope_type ?? 'all'
  form.work_unit_ids = [...(roleScopes.value[role.name]?.work_unit_ids ?? [])]
  form.redirect_to = role.redirect_to || '/'
  formErrors.name = ''; createError.value = ''
  showCreate.value = true
}

function openEditRole(role) {
  editForm.name = role.name; editForm.description = role.description || ''
  editForm.originalName = role.name; editForm.isSystem = !!role.is_system
  editForm.redirect_to = role.redirect_to || '/'
  editFormErrors.name = ''; editError.value = ''
  showEdit.value = true
}

async function submitEditRole() {
  editFormErrors.name = editForm.name.trim() ? '' : 'Nama role wajib diisi'
  if (editFormErrors.name) return
  editing.value = true; editError.value = ''
  try {
    await adminsApi.updateRole(editForm.originalName, {
      name: editForm.name.trim(), description: editForm.description.trim(),
      redirect_to: editForm.redirect_to || '/',
    })
    toast.success('Role berhasil diperbarui!')
    showEdit.value = false
    await loadRoles()
  } catch (err) {
    editError.value = err?.message ?? 'Gagal memperbarui role.'
  } finally { editing.value = false }
}

function confirmDelete(role) { deleteTarget.value = role; showDeleteConfirm.value = true }

async function doDelete() {
  if (!deleteTarget.value) return
  deletingRole.value = deleteTarget.value.name
  try {
    await adminsApi.deleteRole(deleteTarget.value.name)
    toast.success(`Role ${deleteTarget.value.name} dihapus`)
    showDeleteConfirm.value = false
    await loadRoles()
  } catch (err) {
    toast.error(err?.message ?? 'Gagal menghapus role')
  } finally { deletingRole.value = '' }
}

watch(showCreate, v => {
  if (!v) {
    form.name = ''; form.description = ''; form.permissions = []
    form.scope_type = 'all'; form.work_unit_ids = []; form.redirect_to = '/'
    formErrors.name = ''; createError.value = ''; copyingFrom.value = ''
  }
})
</script>

<style scoped>
/* ─── Page ─────────────────────────────────────── */
.rp { display: flex; flex-direction: column; gap: 1.25rem; }

.rp-hd {
  display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem;
}
.rp-title { font-size: 1.35rem; font-weight: 800; color: #111827; margin: 0; }
.rp-sub   { font-size: .8rem; color: #6b7280; margin: .2rem 0 0; }
.rp-loading { display: flex; justify-content: center; padding: 3rem; }

.btn-new {
  display: inline-flex; align-items: center; gap: .45rem;
  background: #4f46e5; color: #fff; border: none; border-radius: .6rem;
  padding: .55rem 1rem; font-size: .82rem; font-weight: 600; cursor: pointer;
  white-space: nowrap; transition: background .15s;
}
.btn-new:hover { background: #4338ca; }

/* ─── Roles list ────────────────────────────────── */
.roles-list { display: flex; flex-direction: column; gap: .75rem; }

.role-card {
  background: #fff; border-radius: .875rem;
  border: 1.5px solid #e5e7eb; overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,.04);
  transition: border-color .15s, box-shadow .15s;
}
.role-card--open {
  border-color: #a5b4fc;
  box-shadow: 0 0 0 3px rgba(99,102,241,.07);
}

/* Summary row */
.role-summary {
  display: flex; align-items: center; gap: 1rem; padding: 1rem 1.25rem;
  cursor: pointer; user-select: none;
  transition: background .12s;
}
.role-summary:hover { background: #fafafa; }

.role-avatar {
  width: 2.75rem; height: 2.75rem; border-radius: .75rem;
  display: flex; align-items: center; justify-content: center;
  font-size: 1.25rem; font-weight: 800; color: #fff; flex-shrink: 0;
  text-transform: uppercase;
}
.avatar--sys    { background: linear-gradient(135deg, #4b5563, #1f2937); }
.avatar--custom { background: linear-gradient(135deg, #6366f1, #4f46e5); }

.role-meta { flex: 1; min-width: 0; }
.role-nameline { display: flex; align-items: center; gap: .45rem; margin-bottom: .15rem; }
.role-name { font-size: 1rem; font-weight: 700; color: #111827; text-transform: capitalize; }

.badge-sys {
  font-size: .62rem; font-weight: 700; letter-spacing: .04em;
  text-transform: uppercase; background: #f3f4f6; color: #6b7280;
  border: 1px solid #e5e7eb; border-radius: .3rem; padding: .1rem .4rem;
}

.role-desc {
  font-size: .78rem; color: #9ca3af; margin: 0 0 .5rem;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 400px;
}

.role-chips { display: flex; flex-wrap: wrap; gap: .3rem; }
.chip {
  display: inline-flex; align-items: center; gap: .3rem;
  font-size: .7rem; font-weight: 500; padding: .2rem .55rem;
  border-radius: .35rem; border: 1px solid;
}
.chip--scope   { background: #eef2ff; color: #4338ca; border-color: #c7d2fe; }
.chip--sky     { background: #e0f2fe; color: #0369a1; border-color: #bae6fd; }
.chip--perm    { background: #ecfdf5; color: #065f46; border-color: #a7f3d0; }
.chip--redirect{ background: #fffbeb; color: #92400e; border-color: #fde68a; }

/* Action buttons */
.role-actions { display: flex; align-items: center; gap: .35rem; flex-shrink: 0; }

.act-btn {
  width: 2rem; height: 2rem; border-radius: .5rem; border: 1px solid;
  display: flex; align-items: center; justify-content: center; cursor: pointer;
  transition: all .12s;
}
.act-btn--edit     { background: #eff6ff; color: #3b82f6; border-color: #bfdbfe; }
.act-btn--edit:hover { background: #dbeafe; }
.act-btn--copy     { background: #f5f3ff; color: #7c3aed; border-color: #ddd6fe; }
.act-btn--copy:hover { background: #ede9fe; }
.copy-note { display: flex; align-items: center; gap: .4rem; font-size: .72rem; color: #6d28d9; background: #f5f3ff; border: 1px solid #e9d5ff; border-radius: .5rem; padding: .5rem .65rem; margin-bottom: .6rem; }
.copy-note svg { flex-shrink: 0; }
.act-btn--del      { background: #fff1f2; color: #ef4444; border-color: #fecaca; }
.act-btn--del:hover { background: #fee2e2; }
.act-btn--disabled { background: #f9fafb; color: #d1d5db; border-color: #e5e7eb; cursor: not-allowed; }

.act-expand {
  display: inline-flex; align-items: center; gap: .3rem;
  font-size: .75rem; font-weight: 600; padding: .4rem .75rem;
  border-radius: .5rem; border: 1px solid #e5e7eb;
  background: #f9fafb; color: #374151; cursor: pointer; transition: all .12s;
}
.act-expand svg { transition: transform .2s; }
.act-expand--open { background: #eef2ff; color: #4338ca; border-color: #c7d2fe; }
.act-expand--open svg { transform: rotate(180deg); }

/* ─── Panel ─────────────────────────────────────── */
.role-panel { border-top: 1.5px solid #f3f4f6; background: #fafafa; }

.sys-notice {
  display: flex; align-items: flex-start; gap: .6rem;
  margin: 1rem 1.25rem 0; padding: .75rem 1rem;
  background: #f9fafb; border: 1px solid #e5e7eb; border-radius: .6rem;
  font-size: .78rem; color: #6b7280;
}
.sys-notice strong { color: #374151; }
.sys-notice-icon { color: #9ca3af; flex-shrink: 0; margin-top: .1rem; }

.panel-body {
  display: grid; grid-template-columns: 260px 1fr; gap: 0;
  transition: opacity .15s;
}
.panel-body--dim { opacity: .45; pointer-events: none; }

/* ─── Scope column ──────────────────────────────── */
.scope-col {
  padding: 1.25rem; border-right: 1.5px solid #f3f4f6;
}

.col-label {
  display: flex; align-items: center; gap: .4rem;
  font-size: .78rem; font-weight: 700; color: #374151; margin-bottom: .35rem;
}
.col-hint { font-size: .72rem; color: #9ca3af; margin: 0 0 .9rem; line-height: 1.45; }

.scope-opts { display: flex; flex-direction: column; gap: .5rem; }

.scope-opt {
  display: flex; align-items: flex-start; gap: .6rem;
  padding: .6rem .75rem; border-radius: .55rem; border: 1.5px solid #e5e7eb;
  background: #fff; cursor: pointer; transition: all .12s;
}
.scope-opt:hover { border-color: #c7d2fe; }
.scope-opt--on   { border-color: #818cf8; background: #eef2ff; }
.scope-opt input[type=radio] { margin-top: .15rem; accent-color: #6366f1; }
.scope-opt span  { font-size: .8rem; font-weight: 600; color: #374151; display: block; }
.scope-opt small { font-size: .68rem; color: #9ca3af; display: block; margin-top: .1rem; }

.scope-tags {
  margin-top: .5rem; padding: .6rem; border-radius: .5rem;
  background: #f5f3ff; border: 1px solid #ddd6fe;
  display: flex; flex-wrap: wrap; gap: .35rem; align-items: center;
}
.scope-tag {
  font-size: .68rem; font-weight: 600; background: #fff; color: #5b21b6;
  border: 1px solid #ddd6fe; border-radius: .3rem; padding: .15rem .45rem;
}
.scope-tag-empty { font-size: .68rem; color: #a78bfa; font-style: italic; }
.scope-edit-btn {
  font-size: .68rem; font-weight: 700; color: #6366f1;
  background: none; border: none; cursor: pointer; margin-left: auto;
  text-decoration: underline;
}

/* ─── Permissions column ────────────────────────── */
.perm-col { padding: 1.25rem; }

.saving-dot {
  font-size: .68rem; font-weight: 600; color: #6366f1;
  margin-left: .5rem; animation: pulse 1.2s ease-in-out infinite;
}
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: .35 } }

/* ─── CRUD Matrix ───────────────────────────────── */
.matrix-wrap {
  border: 1.5px solid #e5e7eb; border-radius: .65rem; overflow: hidden; margin-bottom: .75rem;
}
.matrix { width: 100%; border-collapse: collapse; font-size: .78rem; }

.matrix thead tr { background: #f8fafc; }
.matrix th {
  padding: .5rem .75rem; text-align: center; font-weight: 700;
  border-bottom: 1.5px solid #e5e7eb; font-size: .7rem;
}
.th-mod { text-align: left; width: 35%; }
.th-v   { color: #1d4ed8; }
.th-c   { color: #15803d; }
.th-u   { color: #b45309; }
.th-d   { color: #dc2626; }

.th-pill {
  display: inline-flex; align-items: center; gap: .25rem;
  padding: .2rem .5rem; border-radius: .35rem; font-size: .68rem; font-weight: 700;
}
.th-pill--v { background: #dbeafe; color: #1d4ed8; }
.th-pill--c { background: #dcfce7; color: #15803d; }
.th-pill--u { background: #fef3c7; color: #b45309; }
.th-pill--d { background: #fee2e2; color: #dc2626; }

.matrix tbody tr { border-bottom: 1px solid #f3f4f6; transition: background .1s; }
.matrix tbody tr:last-child { border-bottom: none; }
.matrix tbody tr:hover { background: #fafafa; }

.td-mod {
  padding: .6rem .75rem; display: flex; align-items: center; gap: .5rem;
}
.mod-icon { font-size: .95rem; }
.mod-name { font-weight: 600; color: #374151; }
.td-op { text-align: center; padding: .45rem .5rem; }

/* Operation pills */
.op-pill {
  display: inline-flex; align-items: center; justify-content: center;
  width: 1.75rem; height: 1.75rem; border-radius: .4rem; border: 1.5px solid #e5e7eb;
  background: #f9fafb; color: #d1d5db; cursor: pointer; transition: all .12s;
}
.op-pill:hover { transform: scale(1.1); }

.op-pill--v.op-pill--on { background: #dbeafe; color: #1d4ed8; border-color: #93c5fd; }
.op-pill--c.op-pill--on { background: #dcfce7; color: #15803d; border-color: #86efac; }
.op-pill--u.op-pill--on { background: #fef3c7; color: #b45309; border-color: #fcd34d; }
.op-pill--d.op-pill--on { background: #fee2e2; color: #dc2626; border-color: #fca5a5; }

.op-pill--v:not(.op-pill--on):hover { background: #eff6ff; border-color: #bfdbfe; color: #60a5fa; }
.op-pill--c:not(.op-pill--on):hover { background: #f0fdf4; border-color: #bbf7d0; color: #4ade80; }
.op-pill--u:not(.op-pill--on):hover { background: #fffbeb; border-color: #fde68a; color: #f59e0b; }
.op-pill--d:not(.op-pill--on):hover { background: #fff1f2; border-color: #fecaca; color: #f87171; }

/* ─── Special perms ─────────────────────────────── */
.special-rows {
  display: flex; flex-direction: column; gap: .45rem;
}
.sp-row {
  display: flex; align-items: center; gap: .6rem; flex-wrap: wrap;
  padding: .4rem .5rem; border-radius: .5rem;
  background: #fafafa; border: 1px solid #f3f4f6;
}
.sp-label {
  font-size: .72rem; font-weight: 700; color: #6b7280;
  min-width: 100px; white-space: nowrap;
}
.sp-pills { display: flex; flex-wrap: wrap; gap: .3rem; }

.sp-pill {
  display: inline-flex; align-items: center; gap: .25rem;
  font-size: .7rem; font-weight: 600; padding: .25rem .6rem;
  border-radius: 999px; border: 1.5px solid #e5e7eb;
  background: #f9fafb; color: #9ca3af; cursor: pointer; transition: all .12s;
}
.sp-pill:hover { background: #f5f3ff; border-color: #c4b5fd; color: #7c3aed; }
.sp-pill--on {
  background: #f0fdf4; color: #15803d; border-color: #86efac;
}

/* ─── Empty state ───────────────────────────────── */
.empty-state {
  text-align: center; padding: 3rem;
  background: #f9fafb; border: 1.5px dashed #e5e7eb; border-radius: .875rem;
}
.empty-icon { font-size: 3rem; opacity: .5; margin-bottom: .75rem; }
.empty-state h3 { font-size: .95rem; font-weight: 700; color: #374151; margin: 0 0 .25rem; }
.empty-state p  { font-size: .8rem; color: #9ca3af; }

/* ─── Modals ────────────────────────────────────── */
.modal-backdrop {
  position: fixed; inset: 0; background: rgba(0,0,0,.45);
  display: flex; align-items: center; justify-content: center;
  z-index: 9999; padding: 1rem;
}
.modal {
  background: #fff; border-radius: .875rem; width: 100%;
  max-height: 90vh; overflow-y: auto; display: flex; flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,.22);
}
.modal--sm { max-width: 420px; }
.modal--lg { max-width: 820px; }

.modal-hd {
  display: flex; align-items: center; justify-content: space-between;
  padding: 1rem 1.25rem; border-bottom: 1.5px solid #f3f4f6;
  background: linear-gradient(135deg, #6366f1, #4f46e5); border-radius: .875rem .875rem 0 0;
}
.modal-hd--danger { background: linear-gradient(135deg, #ef4444, #dc2626); }
.modal-title { font-size: .9rem; font-weight: 700; color: #fff; }
.modal-close {
  background: rgba(255,255,255,.2); border: none; color: #fff;
  width: 1.75rem; height: 1.75rem; border-radius: .4rem; cursor: pointer;
  font-size: .85rem; display: flex; align-items: center; justify-content: center;
  transition: background .12s;
}
.modal-close:hover { background: rgba(255,255,255,.35); }

.modal-bd { padding: 1.25rem; flex: 1; overflow-y: auto; }
.modal-ft {
  padding: 1rem 1.25rem; border-top: 1.5px solid #f3f4f6;
  display: flex; justify-content: flex-end; gap: .5rem; background: #fafafa;
  border-radius: 0 0 .875rem .875rem;
}

/* Form elements in modal */
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: .75rem; margin-bottom: 1rem; }
.form-group { margin-bottom: 1rem; }
.form-label { display: block; font-size: .78rem; font-weight: 600; color: #374151; margin-bottom: .3rem; }
.form-hint { margin-top: .3rem; font-size: .68rem; color: #9ca3af; }
.form-select {
  width: 100%; font-size: .82rem; padding: .5rem .75rem;
  border: 1.5px solid #e5e7eb; border-radius: .5rem; background: #fff;
  color: #374151; outline: none; transition: border-color .12s;
}
.form-select:focus { border-color: #818cf8; }

.form-section { padding-top: 1rem; border-top: 1.5px solid #f3f4f6; margin-top: .75rem; }
.form-section-title { font-size: .78rem; font-weight: 700; color: #374151; margin-bottom: .75rem; }

.scope-row { display: grid; grid-template-columns: 1fr 1fr; gap: .5rem; margin-bottom: .75rem; }

.wu-list {
  max-height: 10rem; overflow-y: auto; display: flex; flex-direction: column; gap: .25rem;
  border: 1.5px solid #e5e7eb; border-radius: .5rem; padding: .5rem; background: #fafafa;
}
.wu-item {
  display: flex; align-items: flex-start; gap: .5rem; padding: .4rem .5rem;
  border-radius: .35rem; cursor: pointer; transition: background .1s;
}
.wu-item:hover { background: #fff; }
.wu-item input[type=checkbox] { margin-top: .15rem; accent-color: #6366f1; }
.wu-item span  { font-size: .8rem; font-weight: 600; color: #374151; display: block; }
.wu-item small { font-size: .68rem; color: #9ca3af; display: block; }
.wu-empty { font-size: .78rem; color: #9ca3af; text-align: center; padding: .75rem; }

/* Modal buttons */
.btn-cancel {
  padding: .5rem 1rem; border: 1.5px solid #e5e7eb; border-radius: .5rem;
  background: #fff; color: #6b7280; font-size: .82rem; font-weight: 600; cursor: pointer;
  transition: all .12s;
}
.btn-cancel:hover { background: #f9fafb; }
.btn-submit {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .5rem 1.25rem; border-radius: .5rem; border: none;
  background: #4f46e5; color: #fff; font-size: .82rem; font-weight: 700; cursor: pointer;
  transition: background .12s;
}
.btn-submit:hover:not(:disabled) { background: #4338ca; }
.btn-submit:disabled { opacity: .5; cursor: not-allowed; }
.btn-danger {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .5rem 1.25rem; border-radius: .5rem; border: none;
  background: #dc2626; color: #fff; font-size: .82rem; font-weight: 700; cursor: pointer;
  transition: background .12s;
}
.btn-danger:hover:not(:disabled) { background: #b91c1c; }
.btn-danger:disabled { opacity: .5; cursor: not-allowed; }

/* Delete modal */
.del-icon-wrap {
  width: 3rem; height: 3rem; border-radius: 50%;
  background: #fee2e2; color: #dc2626;
  display: flex; align-items: center; justify-content: center; margin: 0 auto 1rem;
}
.del-confirm-text { font-size: .85rem; color: #6b7280; line-height: 1.65; }
.del-target { color: #111827; text-decoration: underline dotted; }

/* Spinner */
.spinner-sm {
  width: 14px; height: 14px; border-radius: 50%;
  border: 2px solid rgba(255,255,255,.3); border-top-color: #fff;
  animation: spin .6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Modal transition */
.modal-fade-enter-active, .modal-fade-leave-active { transition: opacity .18s; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
.modal-fade-enter-active .modal, .modal-fade-leave-active .modal { transition: transform .18s; }
.modal-fade-enter-from .modal { transform: scale(.96) translateY(-8px); }
.modal-fade-leave-to .modal   { transform: scale(.96) translateY(-8px); }

/* ─── Responsive ────────────────────────────────── */
@media (max-width: 768px) {
  .panel-body { grid-template-columns: 1fr; }
  .scope-col  { border-right: none; border-bottom: 1.5px solid #f3f4f6; }
  .form-row   { grid-template-columns: 1fr; }
  .scope-row  { grid-template-columns: 1fr; }
  .matrix th, .matrix td { padding: .4rem .35rem; }
  .mod-name { font-size: .72rem; }
}
</style>
