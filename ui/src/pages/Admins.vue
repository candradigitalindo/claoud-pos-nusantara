<template>
  <div class="admins-root">

    <!-- Header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">Pengguna</h1>
        <p class="page-sub">Kelola akun admin dan hak akses sistem</p>
      </div>
      <button class="add-btn" @click="showCreate = true">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
        </svg>
        Tambah Pengguna
      </button>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Table -->
    <div class="table-panel">
      <div class="table-scroll">
        <table class="atable">
          <thead>
            <tr>
              <th>Pengguna</th>
              <th>Role</th>
              <th>Status</th>
              <th>Dibuat</th>
              <th class="th-actions"></th>
            </tr>
          </thead>
          <tbody>
            <template v-if="loading">
              <tr v-for="n in 5" :key="n" class="tr-skeleton">
                <td><div class="skel skel-w60" /></td>
                <td><div class="skel skel-w44" /></td>
                <td><div class="skel skel-pill" /></td>
                <td><div class="skel skel-w60" /></td>
                <td></td>
              </tr>
            </template>
            <template v-else-if="admins.length">
              <tr v-for="row in admins" :key="row.id" class="tr-data">
                <td>
                  <div class="user-cell">
                    <div class="user-avatar">{{ row.name?.charAt(0)?.toUpperCase() ?? row.username?.charAt(0)?.toUpperCase() ?? 'U' }}</div>
                    <div>
                      <p class="user-name">{{ row.name }}</p>
                      <p class="user-username">username : {{ row.username }}</p>
                      <p v-if="row.wa_phone" class="user-username">WA : +{{ row.wa_phone }}<span v-if="row.wa_notify === false"> (notifikasi mati)</span></p>
                    </div>
                  </div>
                </td>
                <td>
                  <span class="role-badge">{{ row.role }}</span>
                </td>
                <td>
                  <span :class="['status-badge', row.is_active ? 'status-active' : 'status-inactive']">
                    <span class="status-dot" />{{ row.is_active ? 'Aktif' : 'Nonaktif' }}
                  </span>
                </td>
                <td class="td-date">{{ formatDateTime(row.created_at) }}</td>
                <td class="td-actions">
                  <button class="btn-icon btn-icon--edit"   title="Edit"           @click="openEdit(row)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                  </button>
                  <button class="btn-icon btn-icon--key"    title="Reset Password" @click="openResetPw(row)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
                  </button>
                  <button class="btn-icon" :class="row.is_active ? 'btn-icon--off' : 'btn-icon--on'" :title="row.is_active ? 'Nonaktifkan' : 'Aktifkan'" @click="toggleActive(row)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" y1="2" x2="12" y2="12"/></svg>
                  </button>
                  <button class="btn-icon btn-icon--delete" title="Hapus"          @click="confirmDelete(row)">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                  </button>
                </td>
              </tr>
            </template>
            <tr v-else>
              <td colspan="5">
                <div class="empty-state">
                  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                  <p>Belum ada pengguna.</p>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ── Create Modal ── -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showCreate" class="modal-backdrop" @click.self="closeCreate">
          <div class="modal-panel">
            <div class="modal-glare" aria-hidden="true" />

            <!-- Header -->
            <div class="modal-header">
              <div class="modal-icon">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
              </div>
              <div>
                <h2 class="modal-title">Tambah Pengguna</h2>
                <p class="modal-sub">Isi data akun dan pilih role yang sesuai</p>
              </div>
              <button class="modal-close" @click="closeCreate" aria-label="Tutup">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>

            <div class="modal-body">
              <AppAlert type="error" :message="createError" />

              <!-- Credential section -->
              <p class="section-label">Kredensial Login</p>
              <div class="form-grid">
                <div class="field" :class="{ 'field--error': formErrors.username }">
                  <label class="field-label">Username <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                    <input v-model="form.username" class="field-input field-input--mono" placeholder="contoh: budi.santoso" autocomplete="off" @input="formErrors.username=''" />
                  </div>
                  <p v-if="formErrors.username" class="field-err">{{ formErrors.username }}</p>
                </div>
                <div class="field" :class="{ 'field--error': formErrors.name }">
                  <label class="field-label">Nama Lengkap <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                    <input v-model="form.name" class="field-input" placeholder="contoh: Budi Santoso" @input="formErrors.name=''" />
                  </div>
                  <p v-if="formErrors.name" class="field-err">{{ formErrors.name }}</p>
                </div>
              </div>

              <div class="field field-span2 mt-3">
                <label class="field-label">Nomor WhatsApp <span style="font-weight:400;color:#9ca3af">(opsional)</span></label>
                <div class="field-input-wrap">
                  <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 11.5a8.38 8.38 0 01-.9 3.8 8.5 8.5 0 01-7.6 4.7 8.38 8.38 0 01-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 01-.9-3.8 8.5 8.5 0 014.7-7.6 8.38 8.38 0 013.8-.9h.5a8.48 8.48 0 018 8v.5z"/></svg>
                  <input v-model="form.wa_phone" class="field-input field-input--mono" placeholder="08xx… — untuk notifikasi WhatsApp sesuai posisi" autocomplete="off" />
                </div>
              </div>

              <div class="field field-span2 mt-3" :class="{ 'field--error': formErrors.password }">
                <label class="field-label">Password <span class="req">*</span></label>
                <div class="field-input-wrap">
                  <svg class="field-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
                  <input v-model="form.password" :type="showPw ? 'text' : 'password'" class="field-input field-input--mono" placeholder="Minimal 8 karakter" autocomplete="new-password" @input="formErrors.password=''" />
                  <button type="button" class="pw-toggle" @click="showPw = !showPw" tabindex="-1">
                    <svg v-if="!showPw" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
                    <svg v-else width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  </button>
                </div>
                <!-- Password strength bar -->
                <div class="pw-strength">
                  <div class="pw-bars">
                    <span :class="['pw-bar', pwStrength >= 1 ? 'pw-bar--' + pwLabel.key : '']" />
                    <span :class="['pw-bar', pwStrength >= 2 ? 'pw-bar--' + pwLabel.key : '']" />
                    <span :class="['pw-bar', pwStrength >= 3 ? 'pw-bar--' + pwLabel.key : '']" />
                    <span :class="['pw-bar', pwStrength >= 4 ? 'pw-bar--' + pwLabel.key : '']" />
                  </div>
                  <span v-if="form.password" :class="['pw-label', 'pw-label--' + pwLabel.key]">{{ pwLabel.text }}</span>
                </div>
                <p v-if="formErrors.password" class="field-err">{{ formErrors.password }}</p>
              </div>

              <!-- Role section -->
              <div class="field mt-4" :class="{ 'field--error': formErrors.role }">
                <label class="field-label">Role <span class="req">*</span></label>
                <SearchSelect
                  v-model="form.role"
                  :options="availableRoles"
                  valueKey="name"
                  labelKey="name"
                  placeholder="Pilih role…"
                  searchPlaceholder="Cari role…"
                  @change="formErrors.role = ''"
                />
                <p v-if="formErrors.role" class="field-err">{{ formErrors.role }}</p>
                <p v-if="selectedRoleDesc" class="role-hint">{{ selectedRoleDesc }}</p>
              </div>
            </div>

            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="closeCreate">Batal</button>
              <button class="modal-btn-save" :disabled="creating" @click="createAdmin">
                <span v-if="creating" class="btn-spinner btn-spinner--light" />
                <template v-else>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="15" y2="14"/></svg>
                  Buat Akun
                </template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── Edit Modal ── -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showEdit" class="modal-backdrop" @click.self="showEdit = false">
          <div class="modal-panel" style="max-width:440px">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon" style="background:linear-gradient(135deg,#1d4ed8,#1e40af)">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
              </div>
              <div>
                <h2 class="modal-title">Edit Pengguna</h2>
                <p class="modal-sub">username : {{ editRow?.username }}</p>
              </div>
              <button class="modal-close" @click="showEdit = false"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
            </div>
            <div class="modal-body">
              <AppAlert type="error" :message="editError" />
              <div class="field">
                <label class="field-label">Nama Lengkap</label>
                <div class="field-input-wrap">
                  <input v-model="editForm.name" class="field-input" placeholder="Nama lengkap" />
                </div>
              </div>
              <div class="field mt-4">
                <label class="field-label">Nomor WhatsApp</label>
                <div class="field-input-wrap">
                  <input v-model="editForm.wa_phone" class="field-input field-input--mono" placeholder="08xx… (kosongkan bila tidak dikirimi notifikasi)" />
                </div>
                <label class="role-hint" style="display:flex;align-items:center;gap:.4rem;cursor:pointer">
                  <input v-model="editForm.wa_notify" type="checkbox" class="accent-emerald-600" />
                  Terima notifikasi WhatsApp sesuai posisi (role &amp; outlet)
                </label>
              </div>
              <div class="field mt-4">
                <label class="field-label">Role</label>
                <SearchSelect
                  v-model="editForm.role"
                  :options="availableRoles"
                  valueKey="name"
                  labelKey="name"
                  placeholder="Pilih role…"
                  searchPlaceholder="Cari role…"
                />
                <p v-if="editRoleDesc" class="role-hint">{{ editRoleDesc }}</p>
              </div>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="showEdit = false">Batal</button>
              <button class="modal-btn-save" style="background:linear-gradient(135deg,#1d4ed8,#1e40af)" :disabled="editLoading" @click="saveEdit">
                <span v-if="editLoading" class="btn-spinner btn-spinner--light" />
                <template v-else>Simpan</template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── Reset Password Modal ── -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showResetPw" class="modal-backdrop" @click.self="showResetPw = false">
          <div class="modal-panel" style="max-width:400px">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon" style="background:rgba(217,119,6,.15);color:#b45309">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
              </div>
              <div>
                <h2 class="modal-title">Reset Password</h2>
                <p class="modal-sub">username : {{ resetPwRow?.username }}</p>
              </div>
              <button class="modal-close" @click="showResetPw = false"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
            </div>
            <div class="modal-body">
              <AppAlert type="error" :message="resetPwError" />
              <div class="field">
                <label class="field-label">Password Baru</label>
                <div class="field-input-wrap">
                  <input v-model="resetPwForm.newPassword" type="password" class="field-input field-input--mono" placeholder="Minimal 6 karakter" />
                </div>
              </div>
              <div class="field mt-3">
                <label class="field-label">Konfirmasi Password</label>
                <div class="field-input-wrap">
                  <input v-model="resetPwForm.confirmPassword" type="password" class="field-input field-input--mono" placeholder="Ulangi password baru" />
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="showResetPw = false">Batal</button>
              <button class="modal-btn-save" style="background:linear-gradient(135deg,#d97706,#b45309)" :disabled="resetPwLoading" @click="saveResetPw">
                <span v-if="resetPwLoading" class="btn-spinner btn-spinner--light" />
                <template v-else>Reset Password</template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ── Delete Confirm Modal ── -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="showDelete" class="modal-backdrop" @click.self="showDelete = false">
          <div class="modal-panel" style="max-width:400px">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header" style="border-bottom:none">
              <div class="modal-icon" style="background:rgba(239,68,68,.12);color:#dc2626">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
              </div>
              <h2 class="modal-title" style="color:#991b1b">Hapus Pengguna</h2>
              <button class="modal-close" @click="showDelete = false"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
            </div>
            <div class="modal-body" style="padding-top:0">
              <p style="font-size:.85rem;color:#64748b;line-height:1.6;margin:0">
                Yakin hapus akun <strong style="color:#0f172a">{{ deleteRow?.username }}</strong>? Tindakan ini tidak dapat dibatalkan.
              </p>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="showDelete = false">Batal</button>
              <button class="modal-btn-save" style="background:linear-gradient(135deg,#dc2626,#991b1b)" :disabled="deleteLoading" @click="doDelete">
                <span v-if="deleteLoading" class="btn-spinner btn-spinner--light" />
                <template v-else>Ya, Hapus</template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { adminsApi }      from '@/api/admins.js'
import { formatDateTime } from '@/utils/format.js'
import { useToastStore }  from '@/stores/toast.js'
import AppAlert      from '@/components/ui/AppAlert.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()

// ── Data ────────────────────────────────────────────────────
const admins        = ref([])
const loading       = ref(false)
const errorMsg      = ref('')
const availableRoles = ref([])

loadAdmins()
loadRoles()

async function loadAdmins() {
  loading.value = true; errorMsg.value = ''
  try {
    const res = await adminsApi.list()
    admins.value = res.admins ?? res ?? []
  } catch (err) {
    errorMsg.value = err?.response?.data?.message ?? 'Gagal memuat pengguna.'
  } finally { loading.value = false }
}

async function loadRoles() {
  try {
    const res = await adminsApi.listRoles()
    availableRoles.value = res.roles ?? []
    if (availableRoles.value.length && !availableRoles.value.find(r => r.name === form.role)) {
      form.role = availableRoles.value[0].name
    }
  } catch { /* ignore */ }
}

function defaultRoleDesc(name) {
  if (name === 'superadmin') return 'Akses penuh ke seluruh sistem termasuk pengaturan kritis'
  if (name === 'admin')      return 'Kelola outlet, produk, laporan, dan pengguna'
  if (name === 'manager')    return 'Dashboard, outlet, produk, dan laporan tanpa manajemen user'
  if (name === 'viewer')     return 'Hanya bisa melihat dashboard dan laporan'
  return 'Role kustom'
}

const selectedRoleDesc = computed(() => {
  const r = availableRoles.value.find(x => x.name === form.role)
  return r ? (r.description || defaultRoleDesc(r.name)) : ''
})

const editRoleDesc = computed(() => {
  const r = availableRoles.value.find(x => x.name === editForm.role)
  return r ? (r.description || defaultRoleDesc(r.name)) : ''
})

// ── Create ──────────────────────────────────────────────────
const showCreate  = ref(false)
const creating    = ref(false)
const createError = ref('')
const showPw      = ref(false)
const form        = reactive({ username: '', password: '', name: '', role: 'admin', wa_phone: '' })
const formErrors  = reactive({ username: '', password: '', name: '', role: '' })

const pwStrength = computed(() => {
  const p = form.password
  if (!p) return 0
  let s = 0
  if (p.length >= 8)              s++
  if (/[A-Z]/.test(p))           s++
  if (/[0-9]/.test(p))           s++
  if (/[^A-Za-z0-9]/.test(p))   s++
  return s
})
const pwLabel = computed(() => {
  const map = [
    { key: 'weak',   text: 'Lemah'    },
    { key: 'weak',   text: 'Lemah'    },
    { key: 'fair',   text: 'Sedang'   },
    { key: 'good',   text: 'Kuat'     },
    { key: 'strong', text: 'Sangat Kuat' },
  ]
  return map[pwStrength.value] ?? map[0]
})

function closeCreate() {
  showCreate.value = false
}

watch(showCreate, v => {
  if (!v) {
    form.username = ''; form.password = ''; form.name = ''; form.role = 'admin'; form.wa_phone = ''
    formErrors.username = ''; formErrors.password = ''; formErrors.name = ''; formErrors.role = ''
    createError.value = ''; showPw.value = false
  }
})

async function createAdmin() {
  formErrors.username = form.username.trim()          ? '' : 'Username wajib diisi'
  formErrors.password = form.password.length >= 8     ? '' : 'Password minimal 8 karakter'
  formErrors.name     = form.name.trim()              ? '' : 'Nama wajib diisi'
  formErrors.role     = form.role                     ? '' : 'Pilih role terlebih dahulu'
  if (formErrors.username || formErrors.password || formErrors.name || formErrors.role) return
  creating.value = true; createError.value = ''
  try {
    await adminsApi.create(form)
    toast.success('Pengguna berhasil ditambahkan!')
    showCreate.value = false
    await loadAdmins()
  } catch (err) {
    createError.value = err?.response?.data?.message ?? 'Gagal membuat pengguna.'
  } finally { creating.value = false }
}

// ── Edit ────────────────────────────────────────────────────
const showEdit    = ref(false)
const editRow     = ref(null)
const editForm    = reactive({ name: '', role: '', wa_phone: '', wa_notify: true })
const editLoading = ref(false)
const editError   = ref('')

function openEdit(row) {
  editRow.value = row; editForm.name = row.name; editForm.role = row.role
  editForm.wa_phone = row.wa_phone || ''; editForm.wa_notify = row.wa_notify !== false
  editError.value = ''; showEdit.value = true
}

async function saveEdit() {
  if (!editForm.name.trim()) { editError.value = 'Nama wajib diisi'; return }
  editLoading.value = true; editError.value = ''
  try {
    await adminsApi.updateAdmin(editRow.value.id, { name: editForm.name.trim(), role: editForm.role, wa_phone: editForm.wa_phone.trim(), wa_notify: editForm.wa_notify })
    toast.success('Pengguna berhasil diperbarui')
    showEdit.value = false; await loadAdmins()
  } catch (err) {
    editError.value = err?.response?.data?.error ?? 'Gagal memperbarui pengguna'
  } finally { editLoading.value = false }
}

// ── Reset Password ───────────────────────────────────────────
const showResetPw    = ref(false)
const resetPwRow     = ref(null)
const resetPwForm    = reactive({ newPassword: '', confirmPassword: '' })
const resetPwLoading = ref(false)
const resetPwError   = ref('')

function openResetPw(row) {
  resetPwRow.value = row; resetPwForm.newPassword = ''; resetPwForm.confirmPassword = ''
  resetPwError.value = ''; showResetPw.value = true
}

async function saveResetPw() {
  if (!resetPwForm.newPassword || resetPwForm.newPassword.length < 6) {
    resetPwError.value = 'Password minimal 6 karakter'; return
  }
  if (resetPwForm.newPassword !== resetPwForm.confirmPassword) {
    resetPwError.value = 'Konfirmasi password tidak cocok'; return
  }
  resetPwLoading.value = true; resetPwError.value = ''
  try {
    await adminsApi.resetPassword(resetPwRow.value.id, resetPwForm.newPassword)
    toast.success(`Password ${resetPwRow.value.username} berhasil direset`)
    showResetPw.value = false
  } catch (err) {
    resetPwError.value = err?.response?.data?.error ?? 'Gagal mereset password'
  } finally { resetPwLoading.value = false }
}

// ── Toggle Active ────────────────────────────────────────────
async function toggleActive(row) {
  try {
    const updated = await adminsApi.toggleActive(row.id)
    const idx = admins.value.findIndex(a => a.id === row.id)
    if (idx >= 0) admins.value[idx] = updated
    toast.success(updated.is_active ? `${row.username} diaktifkan` : `${row.username} dinonaktifkan`)
  } catch (err) {
    toast.error(err?.response?.data?.error ?? 'Gagal mengubah status')
  }
}

// ── Delete ───────────────────────────────────────────────────
const showDelete    = ref(false)
const deleteRow     = ref(null)
const deleteLoading = ref(false)

function confirmDelete(row) { deleteRow.value = row; showDelete.value = true }

async function doDelete() {
  deleteLoading.value = true
  try {
    await adminsApi.deleteAdmin(deleteRow.value.id)
    toast.success(`${deleteRow.value.username} berhasil dihapus`)
    showDelete.value = false; await loadAdmins()
  } catch (err) {
    toast.error(err?.response?.data?.error ?? 'Gagal menghapus pengguna')
  } finally { deleteLoading.value = false }
}
</script>

<style scoped>
.admins-root { display: flex; flex-direction: column; gap: 1.25rem; }

.page-header { display: flex; align-items: flex-end; justify-content: space-between; flex-wrap: wrap; gap: .75rem; }
.page-title  { font-size: 1.5rem; font-weight: 800; color: #0f4226; letter-spacing: -.03em; line-height: 1.15; }
.page-sub    { margin-top: .2rem; font-size: .82rem; color: #5a7866; }

.add-btn {
  display: inline-flex; align-items: center; gap: .4rem;
  padding: .48rem 1.1rem; border-radius: .75rem;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  font-size: .8rem; font-weight: 700; cursor: pointer; border: none;
  box-shadow: 0 3px 12px rgba(15,66,38,.32); transition: opacity .15s, transform .15s;
}
.add-btn:hover { opacity: .88; transform: translateY(-2px); }

/* Table */
.table-panel { background: rgba(255,255,255,.72); backdrop-filter: blur(20px); border-radius: 1.1rem; border: 1px solid rgba(255,255,255,.65); box-shadow: 0 2px 20px rgba(0,0,0,.07); overflow: hidden; }
.table-scroll { overflow-x: auto; }
.atable { width: 100%; border-collapse: collapse; font-size: .82rem; }
.atable thead tr { background: rgba(15,66,38,.04); border-bottom: 1px solid rgba(0,0,0,.06); }
.atable th { padding: .7rem 1rem; text-align: left; font-size: .7rem; font-weight: 700; color: #5a7866; text-transform: uppercase; letter-spacing: .05em; white-space: nowrap; }
.atable td { padding: .75rem 1rem; border-bottom: 1px solid rgba(0,0,0,.04); vertical-align: middle; }
.tr-data:last-child td { border-bottom: none; }
.tr-data:hover td { background: rgba(15,66,38,.02); }
.th-actions { width: 120px; }
.td-actions { display: flex; align-items: center; justify-content: flex-end; gap: .35rem; }
.td-date { font-size: .75rem; color: #7a9e8a; white-space: nowrap; }

.user-cell { display: flex; align-items: center; gap: .65rem; }
.user-avatar { width: 32px; height: 32px; border-radius: .55rem; background: linear-gradient(135deg,#1a5c38,#0f4226); color: #fff; display: flex; align-items: center; justify-content: center; font-size: .75rem; font-weight: 700; flex-shrink: 0; }
.user-name { font-size: .82rem; font-weight: 600; color: #1a2e23; line-height: 1.2; }
.user-username { font-size: .72rem; color: #7a9e8a; }

.role-badge { display: inline-block; padding: .2rem .6rem; border-radius: 999px; background: rgba(59,130,246,.1); color: #1d4ed8; font-size: .72rem; font-weight: 600; }

.status-badge { display: inline-flex; align-items: center; gap: .3rem; padding: .2rem .6rem; border-radius: 999px; font-size: .72rem; font-weight: 600; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.status-active   { background: rgba(16,185,129,.1); color: #059669; }
.status-active .status-dot { background: #10b981; }
.status-inactive { background: rgba(239,68,68,.08); color: #dc2626; }
.status-inactive .status-dot { background: #ef4444; }

.btn-icon { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: .5rem; border: 1px solid transparent; cursor: pointer; transition: background .14s, border-color .14s; flex-shrink: 0; }
.btn-icon:disabled { opacity: .4; cursor: not-allowed; }
.btn-icon--edit   { background: rgba(45,143,86,.1);  color: #2d8f56; border-color: rgba(45,143,86,.18); }
.btn-icon--edit:hover   { background: rgba(45,143,86,.18); }
.btn-icon--key    { background: rgba(217,119,6,.1);  color: #b45309; border-color: rgba(217,119,6,.18); }
.btn-icon--key:hover    { background: rgba(217,119,6,.18); }
.btn-icon--on     { background: rgba(22,163,74,.08); color: #16a34a; border-color: rgba(22,163,74,.18); }
.btn-icon--on:hover     { background: rgba(22,163,74,.16); }
.btn-icon--off    { background: rgba(220,38,38,.07); color: #dc2626; border-color: rgba(220,38,38,.18); }
.btn-icon--off:hover    { background: rgba(220,38,38,.15); }
.btn-icon--delete { background: rgba(239,68,68,.07); color: #dc2626; border-color: rgba(239,68,68,.18); }
.btn-icon--delete:hover { background: rgba(239,68,68,.15); }

.empty-state { display: flex; flex-direction: column; align-items: center; gap: .5rem; padding: 2.5rem 1rem; color: #7a9e8a; }
.empty-state p { font-size: .82rem; }

/* Skeleton */
.tr-skeleton td { padding: .75rem 1rem; }
.skel { border-radius: 5px; background: linear-gradient(90deg,#e5eae7 25%,#f0f4f1 50%,#e5eae7 75%); background-size: 200% 100%; animation: shimmer 1.4s infinite; height: 10px; }
.skel-w44 { width: 44%; } .skel-w60 { width: 60%; } .skel-pill { height: 20px; width: 56px; border-radius: 999px; }
@keyframes shimmer { from { background-position: 200% 0 } to { background-position: -200% 0 } }

/* Modal shared */
.modal-backdrop { position: fixed; inset: 0; z-index: 60; display: flex; align-items: center; justify-content: center; padding: 1rem; background: rgba(0,0,0,.45); backdrop-filter: blur(6px); }
.modal-panel { position: relative; width: 100%; max-width: 520px; background: rgba(255,255,255,.96); backdrop-filter: blur(40px) saturate(180%); border-radius: 1.25rem; border: 1px solid rgba(255,255,255,.8); box-shadow: 0 24px 80px rgba(0,0,0,.18), 0 4px 24px rgba(0,0,0,.1); overflow: hidden; }
.modal-glare { position: absolute; top: 0; left: 10%; right: 10%; height: 1px; background: linear-gradient(90deg,transparent,rgba(255,255,255,.9),transparent); pointer-events: none; }
.modal-header { display: flex; align-items: center; gap: .85rem; padding: 1.25rem 1.4rem 1rem; border-bottom: 1px solid rgba(0,0,0,.06); }
.modal-icon { width: 36px; height: 36px; border-radius: .75rem; background: linear-gradient(135deg,#1a5c38,#0f4226); color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.modal-title { font-size: 1rem; font-weight: 700; color: #0f2d1d; line-height: 1.2; }
.modal-sub { font-size: .75rem; color: #7a9e8a; margin-top: .1rem; }
.modal-close { margin-left: auto; display: flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: .55rem; border: none; background: rgba(0,0,0,.05); color: #64748b; cursor: pointer; transition: background .14s; flex-shrink: 0; }
.modal-close:hover { background: rgba(0,0,0,.1); }
.modal-body { padding: 1.2rem 1.4rem; display: flex; flex-direction: column; gap: .25rem; max-height: 70vh; overflow-y: auto; }
.modal-footer { display: flex; justify-content: flex-end; gap: .6rem; padding: .9rem 1.4rem; border-top: 1px solid rgba(0,0,0,.06); background: rgba(0,0,0,.015); }
.modal-btn-cancel { padding: .48rem 1rem; border-radius: .65rem; font-size: .82rem; font-weight: 600; background: transparent; border: 1px solid rgba(0,0,0,.12); color: #475569; cursor: pointer; transition: background .14s; }
.modal-btn-cancel:hover { background: rgba(0,0,0,.05); }
.modal-btn-save { display: inline-flex; align-items: center; gap: .4rem; padding: .48rem 1.1rem; border-radius: .65rem; font-size: .82rem; font-weight: 700; background: linear-gradient(135deg,#1a5c38,#0f4226); color: #fff; border: none; cursor: pointer; transition: opacity .15s; }
.modal-btn-save:disabled { opacity: .6; cursor: not-allowed; }
.modal-btn-save:not(:disabled):hover { opacity: .88; }

/* Fields */
.section-label { font-size: .68rem; font-weight: 700; color: #7a9e8a; text-transform: uppercase; letter-spacing: .07em; margin-bottom: .5rem; }
.mt-3 { margin-top: .75rem; } .mt-4 { margin-top: 1rem; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: .75rem; }
.field { display: flex; flex-direction: column; gap: .3rem; }
.field--error .field-input { border-color: #ef4444 !important; }
.field-label { font-size: .72rem; font-weight: 600; color: #334155; }
.req { color: #ef4444; }
.field-input-wrap { position: relative; display: flex; align-items: center; }
.field-ico { position: absolute; left: .7rem; color: #94a3b8; pointer-events: none; flex-shrink: 0; }
.field-input { width: 100%; padding: .5rem .75rem .5rem 2.1rem; font-size: .82rem; color: #1e293b; background: rgba(255,255,255,.8); border: 1px solid rgba(0,0,0,.13); border-radius: .6rem; outline: none; transition: border-color .15s, box-shadow .15s; }
.field-input:focus { border-color: #1a5c38; box-shadow: 0 0 0 3px rgba(26,92,56,.1); }
.field-input--mono { font-family: ui-monospace, monospace; letter-spacing: .02em; }
.field-err { font-size: .7rem; color: #ef4444; }
.pw-toggle { position: absolute; right: .65rem; background: none; border: none; color: #94a3b8; cursor: pointer; padding: .15rem; display: flex; }
.pw-toggle:hover { color: #475569; }

/* Password strength */
.pw-strength { display: flex; align-items: center; gap: .5rem; margin-top: .35rem; }
.pw-bars { display: flex; gap: .2rem; }
.pw-bar { display: block; height: 3px; width: 32px; border-radius: 2px; background: rgba(0,0,0,.1); transition: background .2s; }
.pw-bar--weak   { background: #ef4444; }
.pw-bar--fair   { background: #f59e0b; }
.pw-bar--good   { background: #10b981; }
.pw-bar--strong { background: #059669; }
.pw-label { font-size: .68rem; font-weight: 600; }
.pw-label--weak   { color: #ef4444; }
.pw-label--fair   { color: #f59e0b; }
.pw-label--good   { color: #10b981; }
.pw-label--strong { color: #059669; }

.role-hint { font-size: .71rem; color: #7a9e8a; margin-top: .3rem; line-height: 1.4; }

/* Spinner */
.btn-spinner { display: inline-block; width: 13px; height: 13px; border: 2px solid rgba(255,255,255,.35); border-top-color: #fff; border-radius: 50%; animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Modal transition */
.modal-fade-enter-active, .modal-fade-leave-active { transition: opacity .2s, transform .2s; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; transform: scale(.96) translateY(8px); }
</style>
