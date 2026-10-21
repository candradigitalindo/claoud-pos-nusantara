<!--
  OutletInfo.vue — Outlet info & API key management
  Route: /outlets/:id/info  (child of OutletDetail)
  Props: outlet (from parent <RouterView :outlet="..." />)
-->
<template>
  <div class="info-root">

    <!-- Outlet info card -->
    <div class="glass-card">
      <div class="card-head">
        <div class="ch-icon">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
          </svg>
        </div>
        <h2 class="ch-title">Informasi Outlet</h2>
        <AppActionButton icon="edit" label="Edit" @click="startEdit" />
      </div>

      <div v-if="loading" class="loading-rows">
        <div v-for="n in 4" :key="n" class="skel-row">
          <div class="skel skel-lbl" /><div class="skel skel-val" />
        </div>
      </div>

      <div v-else class="info-grid">
        <div class="info-item">
          <span class="info-lbl">Nama Outlet</span>
          <span class="info-val">{{ outlet?.name ?? '—' }}</span>
        </div>
        <div class="info-item">
          <span class="info-lbl">Kode</span>
          <code class="info-code">{{ outlet?.code ?? '—' }}</code>
        </div>
        <div class="info-item">
          <span class="info-lbl">Alamat</span>
          <span class="info-val">{{ outlet?.address || '—' }}</span>
        </div>
        <div class="info-item">
          <span class="info-lbl">No. Telepon</span>
          <span class="info-val">{{ outlet?.phone || '—' }}</span>
        </div>
        <div class="info-item">
          <span class="info-lbl">Webhook URL</span>
          <span class="info-val info-val--mono">{{ outlet?.webhook_url || '—' }}</span>
        </div>
        <div class="info-item">
          <span class="info-lbl">Status</span>
          <span :class="['status-badge', outlet?.is_active ? 'status-active' : 'status-inactive']">
            <span class="status-dot" />{{ outlet?.is_active ? 'Aktif' : 'Nonaktif' }}
          </span>
        </div>
        <div class="info-item">
          <span class="info-lbl">Dibuat</span>
          <span class="info-val">{{ fmtDate(outlet?.created_at) }}</span>
        </div>
        <div class="info-item">
          <span class="info-lbl">Diperbarui</span>
          <span class="info-val">{{ fmtDate(outlet?.updated_at) }}</span>
        </div>
      </div>
    </div>

    <!-- API Key card -->
    <div class="glass-card">
      <div class="card-head">
        <div class="ch-icon ch-icon--amber">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
        </div>
        <div class="ch-text">
          <h2 class="ch-title">API Key</h2>
          <p class="ch-sub">Gunakan key ini di aplikasi POS untuk autentikasi ke cloud</p>
        </div>
      </div>

      <div v-if="loading" class="skel skel-key-bar" />

      <div v-else class="apikey-section">
        <div class="apikey-bar">
          <div class="apikey-display">
            <svg class="apikey-ico" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/>
            </svg>
            <span class="apikey-text">{{ showKey ? localKey : maskedKey }}</span>
          </div>
          <div class="apikey-actions">
            <AppActionButton :icon="showKey ? 'hide' : 'view'" :label="showKey ? 'Sembunyikan' : 'Tampilkan'" @click="showKey = !showKey" />
            <AppActionButton :icon="copied ? 'approve' : 'copy'" :label="copied ? 'Tersalin!' : 'Salin'" @click="copyKey" />
          </div>
        </div>

        <div class="regen-section">
          <div class="regen-warn">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
            Setelah di-regenerate, API key lama tidak bisa digunakan lagi. Perbarui di aplikasi POS.
          </div>
          <AppActionButton icon="password" label="Regenerate API Key" :disabled="regenerating" @click="confirmRegen = true" />
        </div>
      </div>
    </div>

    <!-- Edit Outlet Modal -->
    <Teleport to="body">
      <Transition name="mfade">
        <div v-if="showEdit" class="modal-backdrop" @click.self="showEdit = false">
          <div class="modal-panel">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </div>
              <h2 class="modal-title">Edit Outlet</h2>
              <button class="modal-close" @click="showEdit = false">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div class="modal-body">
              <AppAlert type="error" :message="editError" />
              <div class="form-grid">
                <div class="field field-span2">
                  <label class="field-label">Nama Outlet <span class="req">*</span></label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
                    <input v-model="editForm.name" class="field-input" placeholder="Nama outlet" />
                  </div>
                </div>
                <div class="field field-span2">
                  <label class="field-label">Alamat</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/><circle cx="12" cy="10" r="3"/></svg>
                    <input v-model="editForm.address" class="field-input" placeholder="Alamat lengkap" />
                  </div>
                </div>
                <div class="field">
                  <label class="field-label">No. Telepon</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07A19.5 19.5 0 0 1 4.69 12 19.79 19.79 0 0 1 1.61 3.35 2 2 0 0 1 3.6 1h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L7.91 8.6a16 16 0 0 0 8 8l.96-.9a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 17.92v-.001z"/></svg>
                    <input v-model="editForm.phone" class="field-input" placeholder="08xxxxxxxxxx" type="tel" />
                  </div>
                </div>
                <div class="field">
                  <label class="field-label">Webhook URL</label>
                  <div class="field-input-wrap">
                    <svg class="field-ico" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>
                    <input v-model="editForm.webhook_url" class="field-input field-input--mono" placeholder="https://..." type="url" />
                  </div>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="showEdit = false">Batal</button>
              <button class="modal-btn-save" :disabled="saving" @click="saveEdit">
                <span v-if="saving" class="btn-spinner btn-spinner--light" />
                <template v-else>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg>
                  Simpan
                </template>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Regenerate confirm modal -->
    <Teleport to="body">
      <Transition name="mfade">
        <div v-if="confirmRegen" class="modal-backdrop" @click.self="confirmRegen = false">
          <div class="modal-panel modal-panel--sm">
            <div class="modal-glare" aria-hidden="true" />
            <div class="modal-header">
              <div class="modal-icon modal-icon--amber">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>
              </div>
              <h2 class="modal-title">Regenerate API Key?</h2>
              <button class="modal-close" @click="confirmRegen = false">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
              </button>
            </div>
            <div class="modal-body">
              <p class="confirm-text">API key lama akan <strong>tidak valid</strong> dan aplikasi POS yang menggunakannya harus diperbarui. Lanjutkan?</p>
            </div>
            <div class="modal-footer">
              <button class="modal-btn-cancel" @click="confirmRegen = false">Batal</button>
              <button class="modal-btn-danger" @click="doRegen">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="1 4 1 10 7 10"/><polyline points="23 20 23 14 17 14"/>
                  <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4-4.64 4.36A9 9 0 0 1 3.51 15"/>
                </svg>
                Ya, Regenerate
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, reactive, computed, watch } from 'vue'
import { outletsApi }    from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { formatDateTime } from '@/utils/format.js'
import AppAlert from '@/components/ui/AppAlert.vue'

const props = defineProps({
  outlet:   { type: Object, default: null },
  outletId: { type: String, required: true },
})
const emit = defineEmits(['refresh'])

const toast = useToastStore()

const loading   = computed(() => !props.outlet)
const localKey  = ref(props.outlet?.api_key ?? '')
const showKey   = ref(false)
const copied    = ref(false)

watch(() => props.outlet?.api_key, (v) => { if (v) localKey.value = v })

const maskedKey = computed(() => {
  if (!localKey.value) return '••••••••••••••••••••••••••••••••'
  return localKey.value.slice(0, 8) + '••••••••••••••••' + localKey.value.slice(-4)
})

const fmtDate = formatDateTime

async function copyKey() {
  const key = localKey.value || props.outlet?.api_key
  if (!key) return
  await navigator.clipboard.writeText(key).catch(() => {})
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

// ── Edit ──────────────────────────────────────────────────────
const showEdit  = ref(false)
const saving    = ref(false)
const editError = ref('')
const editForm  = reactive({ name: '', address: '', phone: '', webhook_url: '' })

function startEdit() {
  editForm.name        = props.outlet?.name        ?? ''
  editForm.address     = props.outlet?.address     ?? ''
  editForm.phone       = props.outlet?.phone       ?? ''
  editForm.webhook_url = props.outlet?.webhook_url ?? ''
  editError.value = ''
  showEdit.value = true
}

async function saveEdit() {
  if (!editForm.name.trim()) { editError.value = 'Nama outlet wajib diisi.'; return }
  saving.value = true
  editError.value = ''
  try {
    await outletsApi.update(props.outletId, editForm)
    toast.success('Outlet berhasil diperbarui!')
    showEdit.value = false
    emit('refresh')
  } catch (err) {
    editError.value = err?.response?.data?.message ?? 'Gagal menyimpan perubahan.'
  } finally {
    saving.value = false
  }
}

// ── Regenerate API Key ────────────────────────────────────────
const regenerating = ref(false)
const confirmRegen = ref(false)

async function doRegen() {
  regenerating.value = true
  confirmRegen.value = false
  try {
    const data = await outletsApi.regenerateKey(props.outletId)
    localKey.value = data?.api_key ?? data
    showKey.value  = true
    toast.success('API key berhasil di-regenerate!')
    emit('refresh')
  } catch (err) {
    toast.error(err?.response?.data?.message ?? 'Gagal regenerate API key.')
  } finally {
    regenerating.value = false
  }
}
</script>

<style scoped>
.info-root { display: flex; flex-direction: column; gap: 1rem; }

.glass-card {
  border-radius: 1.1rem; overflow: hidden;
  background: rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.7);
  box-shadow: 0 2px 16px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
  padding: 1.25rem 1.35rem;
}

.card-head {
  display: flex; align-items: center; gap: .65rem; margin-bottom: 1.1rem;
  padding-bottom: .85rem; border-bottom: 1px solid rgba(0,0,0,.06);
}
.ch-icon {
  width: 32px; height: 32px; border-radius: .6rem; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  display: flex; align-items: center; justify-content: center;
  box-shadow: 0 2px 8px rgba(15,66,38,.25);
}
.ch-icon--amber { background: linear-gradient(135deg, #d97706, #b45309); box-shadow: 0 2px 8px rgba(180,83,9,.25); }
.ch-text { flex: 1; min-width: 0; }
.ch-title { font-size: .9rem; font-weight: 800; color: #0f2d1d; }
.ch-sub   { font-size: .73rem; color: #7a9e8a; margin-top: .1rem; }

.edit-btn {
  display: inline-flex; align-items: center; gap: .3rem;
  padding: .3rem .75rem; border-radius: .6rem;
  background: rgba(45,143,86,.1); color: #2d8f56; border: none;
  font-size: .73rem; font-weight: 600; cursor: pointer; transition: background .14s;
}
.edit-btn:hover { background: rgba(45,143,86,.18); }

.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: .65rem 1.5rem; }
.info-item { display: flex; flex-direction: column; gap: .18rem; }
.info-lbl  { font-size: .65rem; font-weight: 700; color: #8aaa96; text-transform: uppercase; letter-spacing: .07em; }
.info-val  { font-size: .83rem; color: #1a3d2a; word-break: break-word; }
.info-val--mono { font-family: monospace; font-size: .78rem; color: #4a6555; }
.info-code { font-size: .78rem; font-family: monospace; font-weight: 700; color: #2d8f56; background: rgba(45,143,86,.1); padding: .18rem .5rem; border-radius: .4rem; }

.status-badge { display: inline-flex; align-items: center; gap: .28rem; padding: .18rem .55rem; border-radius: 999px; font-size: .65rem; font-weight: 700; width: fit-content; }
.status-active   { background: rgba(22,163,74,.13); color: #16a34a; }
.status-inactive { background: rgba(107,114,128,.11); color: #6b7280; }
.status-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; flex-shrink: 0; }
.status-active .status-dot { animation: pulse-dot 2s ease-in-out infinite; }
@keyframes pulse-dot { 0%,100%{opacity:1} 50%{opacity:.4} }

/* ── API Key ── */
.apikey-section { display: flex; flex-direction: column; gap: .85rem; }
.apikey-bar {
  display: flex; align-items: center; gap: .5rem; padding: .5rem .75rem;
  border-radius: .75rem; background: rgba(240,248,243,.9);
  border: 1px solid rgba(45,143,86,.15); overflow: hidden;
}
.apikey-display { display: flex; align-items: center; gap: .5rem; flex: 1; min-width: 0; }
.apikey-ico  { color: #8aaa96; flex-shrink: 0; }
.apikey-text { font-family: monospace; font-size: .78rem; color: #1a3d2a; flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; letter-spacing: .04em; }
.apikey-actions { display: flex; gap: .3rem; flex-shrink: 0; }
.key-btn {
  width: 26px; height: 26px; border-radius: .45rem; border: none; cursor: pointer;
  background: rgba(0,0,0,.05); color: #5a7866; display: flex; align-items: center; justify-content: center; transition: background .13s;
}
.key-btn:hover { background: rgba(45,143,86,.15); color: #1a5c38; }

.regen-section { display: flex; align-items: flex-start; gap: 1rem; flex-wrap: wrap; }
.regen-warn {
  flex: 1; min-width: 200px; display: flex; align-items: flex-start; gap: .35rem;
  font-size: .73rem; color: #92400e; background: rgba(251,191,36,.1);
  padding: .5rem .75rem; border-radius: .65rem; border: 1px solid rgba(251,191,36,.25);
}
.regen-btn {
  display: inline-flex; align-items: center; gap: .35rem;
  padding: .43rem 1rem; border-radius: .65rem; cursor: pointer;
  background: rgba(217,119,6,.1); color: #b45309; border: 1px solid rgba(217,119,6,.2);
  font-size: .77rem; font-weight: 700; transition: background .14s; white-space: nowrap;
}
.regen-btn:not(:disabled):hover { background: rgba(217,119,6,.18); }
.regen-btn:disabled { opacity: .5; cursor: not-allowed; }

/* Loading skeletons */
.loading-rows { display: flex; flex-direction: column; gap: .65rem; }
.skel-row   { display: flex; gap: 1rem; }
.skel-key-bar { height: 40px; border-radius: .75rem; }
.skel { background: linear-gradient(90deg,#e5eae7 25%,#f0f4f1 50%,#e5eae7 75%); background-size: 200% 100%; animation: shimmer 1.4s infinite; border-radius: 5px; }
.skel-lbl { height: 8px; width: 70px; }
.skel-val { height: 10px; flex: 1; }
@keyframes shimmer { 0%{background-position:200% 0} 100%{background-position:-200% 0} }

/* ── Modal ── */
.modal-backdrop {
  position: fixed; inset: 0; z-index: 60; display: flex; align-items: center; justify-content: center;
  padding: 1rem; background: rgba(7,26,13,.55); backdrop-filter: blur(6px);
}
.modal-panel {
  position: relative; overflow: hidden; width: 100%; max-width: 500px; border-radius: 1.35rem;
  background: rgba(255,255,255,.9); backdrop-filter: blur(32px) saturate(180%);
  border: 1px solid rgba(255,255,255,.75);
  box-shadow: 0 24px 64px rgba(0,0,0,.18), 0 1px 0 rgba(255,255,255,.9) inset;
}
.modal-panel--sm { max-width: 380px; }
.modal-glare { position: absolute; inset: 0; border-radius: inherit; pointer-events: none; background: linear-gradient(135deg, rgba(255,255,255,.6) 0%, rgba(255,255,255,0) 50%); }
.modal-header {
  display: flex; align-items: center; gap: .65rem; padding: 1.1rem 1.3rem;
  border-bottom: 1px solid rgba(0,0,0,.07);
  background: linear-gradient(135deg, rgba(255,255,255,.55) 0%, rgba(240,249,244,.35) 100%);
}
.modal-icon {
  width: 32px; height: 32px; border-radius: .65rem; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  display: flex; align-items: center; justify-content: center; box-shadow: 0 2px 8px rgba(15,66,38,.28);
}
.modal-icon--amber { background: linear-gradient(135deg, #d97706, #b45309); box-shadow: 0 2px 8px rgba(180,83,9,.28); }
.modal-title { flex: 1; font-size: .95rem; font-weight: 800; color: #0f2d1d; }
.modal-close {
  width: 28px; height: 28px; border-radius: .5rem; border: none; cursor: pointer;
  background: rgba(0,0,0,.05); color: #5a7866; display: flex; align-items: center; justify-content: center; transition: background .14s;
}
.modal-close:hover { background: rgba(0,0,0,.1); }
.modal-body { padding: 1.15rem 1.3rem; }
.confirm-text { font-size: .82rem; color: #4a6555; line-height: 1.55; }
.confirm-text strong { color: #0f2d1d; }

.form-grid  { display: grid; grid-template-columns: 1fr 1fr; gap: .8rem; }
.field-span2 { grid-column: span 2; }
.field { display: flex; flex-direction: column; gap: .28rem; }
.field-label { font-size: .67rem; font-weight: 700; color: #4a6555; text-transform: uppercase; letter-spacing: .06em; }
.req { color: #e11d48; margin-left:.1rem; }
.field-input-wrap { position: relative; }
.field-ico { position: absolute; left: .6rem; top: 50%; transform: translateY(-50%); color: #8aaa96; pointer-events: none; }
.field-input {
  width: 100%; padding: .43rem .6rem .43rem 1.9rem; border-radius: .6rem;
  background: rgba(240,248,243,.85); border: 1px solid rgba(0,0,0,.09);
  font-size: .81rem; color: #1a3d2a; outline: none;
  transition: border-color .15s, box-shadow .15s; box-sizing: border-box;
}
.field-input:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1); background: #fff; }
.field-input--mono { font-family: monospace; }

.modal-footer {
  display: flex; justify-content: flex-end; gap: .6rem;
  padding: .9rem 1.3rem; border-top: 1px solid rgba(0,0,0,.07); background: rgba(248,252,249,.5);
}
.modal-btn-cancel {
  padding: .42rem 1rem; border-radius: .6rem; cursor: pointer;
  background: rgba(0,0,0,.06); border: none; font-size: .78rem; font-weight: 600; color: #5a7866; transition: background .14s;
}
.modal-btn-cancel:hover { background: rgba(0,0,0,.1); }
.modal-btn-save {
  display: inline-flex; align-items: center; gap: .35rem; padding: .42rem 1.1rem; border-radius: .6rem;
  cursor: pointer; background: linear-gradient(135deg, #1a5c38, #0f4226); border: none;
  color: #fff; font-size: .78rem; font-weight: 700; box-shadow: 0 3px 10px rgba(15,66,38,.28);
  transition: opacity .13s, transform .13s; min-width: 90px; justify-content: center;
}
.modal-btn-save:disabled { opacity: .5; cursor: not-allowed; }
.modal-btn-save:not(:disabled):hover { opacity: .88; transform: translateY(-1px); }
.modal-btn-danger {
  display: inline-flex; align-items: center; gap: .35rem; padding: .42rem 1.1rem; border-radius: .6rem;
  cursor: pointer; background: linear-gradient(135deg, #dc2626, #b91c1c); border: none;
  color: #fff; font-size: .78rem; font-weight: 700; box-shadow: 0 3px 10px rgba(220,38,38,.28);
  transition: opacity .13s;
}
.modal-btn-danger:hover { opacity: .88; }

.btn-spinner { display: inline-block; width: 13px; height: 13px; border-radius: 50%; border: 2px solid rgba(0,0,0,.15); border-top-color: #059669; animation: spin .7s linear infinite; }
.btn-spinner--light { border-color: rgba(255,255,255,.25); border-top-color: #fff; }
@keyframes spin { to { transform: rotate(360deg); } }

.mfade-enter-active { transition: all .2s ease-out; }
.mfade-leave-active { transition: all .16s ease-in; }
.mfade-enter-from, .mfade-leave-to { opacity: 0; }
.mfade-enter-from .modal-panel { transform: scale(.96) translateY(10px); }
</style>
