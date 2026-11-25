<template>
  <div class="space-y-4">
    <!-- Panduan & Kontrol -->
    <AppCard>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 max-w-3xl">
          <h2 class="text-base font-bold text-gray-900">Atur Notifikasi per Role</h2>
          <p class="mt-1 text-xs leading-relaxed text-gray-600">
            Aktifkan atau matikan pesan WhatsApp untuk tiap posisi tanpa mengubah hak akses aplikasi mereka.
            Klik langsung tombol pesan (mode <strong>Per Role</strong>) atau chip role (mode <strong>Per Event</strong>), lalu klik <strong>Simpan</strong> di bawah.
          </p>
        </div>
        <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 text-xs font-semibold">
          <button type="button" class="rounded-md px-3 py-1.5 transition-colors"
            :class="viewMode === 'role' ? 'bg-emerald-600 text-white shadow-sm' : 'text-gray-600 hover:text-gray-900'"
            @click="viewMode = 'role'">Per Role</button>
          <button type="button" class="rounded-md px-3 py-1.5 transition-colors"
            :class="viewMode === 'event' ? 'bg-emerald-600 text-white shadow-sm' : 'text-gray-600 hover:text-gray-900'"
            @click="viewMode = 'event'">Per Event (Siapa Terima Apa)</button>
        </div>
      </div>

      <div v-if="!settings.enabled" class="mt-3 rounded-lg bg-amber-50 p-2.5 text-xs text-amber-900">
        Pengiriman WhatsApp umum sedang nonaktif. Perubahan per role tetap bisa disimpan, tetapi pesan belum dikirim sampai saklar umum dinyalakan.
      </div>

      <!-- Filter Bar -->
      <div class="mt-4 grid gap-2 sm:grid-cols-12">
        <div class="sm:col-span-5">
          <input v-model="search" type="search" aria-label="Cari role, pengguna, atau jenis pesan"
            placeholder="Cari role, nama pengguna (mis. Ari, Marco), atau pesan…"
            class="wa-m-input" />
        </div>
        <div class="sm:col-span-3">
          <select v-model="groupFilter" aria-label="Filter modul notifikasi" class="wa-m-input">
            <option value="">Semua modul</option>
            <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
          </select>
        </div>
        <div class="flex items-center sm:col-span-4">
          <label class="flex cursor-pointer items-center gap-2 text-xs font-medium text-gray-700">
            <input v-model="onlyActiveRoles" type="checkbox" class="h-4 w-4 rounded accent-emerald-600" />
            <span>Hanya role berpengguna aktif <span class="text-gray-400">({{ activeRoleCount }}/{{ roles.length }})</span></span>
          </label>
        </div>
      </div>

      <!-- Legenda -->
      <div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 border-t border-gray-100 pt-3 text-[11px] text-gray-600">
        <span class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span><strong>Aktif</strong> (menerima WA)</span>
        <span class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-blue-500"></span><strong>Pengajuan sendiri</strong> (hanya dokumen sendiri)</span>
        <span class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-gray-300"></span><strong>Nonaktif</strong> (dimatikan khusus)</span>
        <span class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full bg-amber-400"></span><strong>Belum disimpan</strong></span>
      </div>
    </AppCard>

    <!-- ═══ MODE 1: PER ROLE ═══ -->
    <template v-if="viewMode === 'role'">
      <AppCard v-if="!filteredRoles.length" class="py-8 text-center text-sm text-gray-500">
        Tidak ada role yang cocok dengan filter.
        <button v-if="onlyActiveRoles || search || groupFilter" type="button"
          class="ml-1 font-semibold text-emerald-700 underline" @click="clearFilters">Tampilkan semua</button>
      </AppCard>

      <div v-else class="space-y-3">
        <AppCard v-for="role in filteredRoles" :key="role.name" :data-matrix-role="role.name"
          :class="isRoleDirty(role.name) ? 'ring-2 ring-amber-300' : ''">
          <!-- Role Header -->
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 pb-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-sm font-bold text-gray-900">{{ role.name }}</h3>
                <span class="wa-m-badge" :class="role.scope_type === 'specific' ? 'bg-gray-100 text-gray-600' : 'bg-indigo-50 text-indigo-700'">
                  {{ roleScopeSummary(role) }}
                </span>
                <span v-if="role.scope_type !== 'specific' && draftOf(role)?.outlet_mode === 'role'"
                  class="wa-m-badge" :class="draftOf(role)?.include_global ? 'bg-violet-50 text-violet-700' : 'bg-gray-100 text-gray-500'">
                  {{ draftOf(role)?.include_global ? 'Global aktif' : 'Tanpa global' }}
                </span>
                <span v-if="isRoleDirty(role.name)" class="wa-m-badge bg-amber-100 text-amber-800">Belum disimpan</span>
                <span v-else-if="role.has_policy" class="wa-m-badge bg-emerald-50 text-emerald-700">Diatur khusus</span>
              </div>

              <!-- Users & WA Readiness -->
              <div class="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-gray-600">
                <template v-if="activeCount(role) > 0">
                  <span class="font-medium text-gray-800">{{ userNamesSummary(role) }}</span>
                  <span class="wa-m-badge" :class="role.users >= activeCount(role) ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-800'">
                    {{ role.users || 0 }}/{{ activeCount(role) }} WA siap
                  </span>
                  <RouterLink v-if="(role.users || 0) < activeCount(role) && canViewUsers" to="/admins"
                    class="text-[11px] font-semibold text-amber-700 underline hover:text-amber-900">Isi nomor WA →</RouterLink>
                </template>
                <span v-else class="italic text-gray-400">Belum ada pengguna aktif di role ini</span>
              </div>
            </div>

            <!-- Header Controls -->
            <div class="flex flex-wrap items-center gap-2">
              <label class="flex cursor-pointer items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-semibold"
                :class="draftOf(role)?.enabled ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-gray-200 bg-gray-50 text-gray-500'">
                <input type="checkbox" class="h-3.5 w-3.5 accent-emerald-600" :checked="draftOf(role)?.enabled"
                  :disabled="!canManage || saving" @change="toggleRoleEnabled(role, $event.target.checked)" />
                {{ draftOf(role)?.enabled ? 'Aktif' : 'Nonaktif' }}
              </label>
              <button type="button" class="wa-m-btn" @click="openDetailModal(role.name)">Outlet & Detail</button>
              <button v-if="canManage && (role.has_policy || isRoleDirty(role.name))" type="button"
                class="wa-m-btn text-gray-500 hover:text-red-700" :disabled="saving"
                @click="resetRole(role)">Reset</button>
            </div>
          </div>

          <!-- Role Events Body -->
          <div v-if="!draftOf(role)?.enabled" class="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-900">
            Seluruh notifikasi WhatsApp untuk role ini dimatikan (termasuk kabar pengajuan milik sendiri). Hak akses halaman web tidak berubah.
          </div>
          <div v-else-if="!roleEventGroups(role).length" class="mt-3 text-xs text-gray-400">
            {{ groupFilter || search ? 'Tidak ada jenis pesan yang cocok dengan filter pada role ini.' : 'Belum ada jenis notifikasi internal yang terkait hak akses role ini.' }}
          </div>
          <div v-else class="mt-3 space-y-3">
            <div v-for="grp in roleEventGroups(role)" :key="grp.name">
              <p class="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-gray-400">{{ grp.name }}</p>
              <div class="flex flex-wrap gap-2">
                <button v-for="ev in grp.events" :key="ev.key" type="button"
                  :data-matrix-cell="`${role.name}:${ev.key}`"
                  :disabled="!canManage || saving || (!canEnableEvent(role, ev) && !isEventConfiguredOn(role, ev))"
                  :title="eventTooltip(role, ev)"
                  class="wa-m-pill"
                  :class="eventPillClass(role, ev)"
                  @click="toggleRoleEvent(role, ev)">
                  <span class="wa-m-dot" :class="eventDotClass(role, ev)"></span>
                  <span class="truncate">{{ ev.label }}</span>
                  <span v-if="isPersonalOnly(role, ev) && isEventConfiguredOn(role, ev)" class="wa-m-tag bg-blue-100 text-blue-800">sendiri</span>
                  <span v-else-if="isCustomEvent(role, ev)" class="wa-m-tag bg-black/5 text-current">khusus</span>
                  <span v-if="settings.events?.[ev.key] === false" class="wa-m-tag bg-amber-100 text-amber-800">umum mati</span>
                </button>
              </div>
            </div>
          </div>
        </AppCard>
      </div>
    </template>

    <!-- ═══ MODE 2: PER EVENT (SIAPA TERIMA APA) ═══ -->
    <template v-else>
      <AppCard v-if="!filteredEventGroups.length" class="py-8 text-center text-sm text-gray-500">
        Tidak ada jenis pesan yang cocok dengan filter.
        <button v-if="search || groupFilter" type="button"
          class="ml-1 font-semibold text-emerald-700 underline" @click="clearFilters">Reset filter</button>
      </AppCard>

      <div v-else class="space-y-4">
        <AppCard v-for="grp in filteredEventGroups" :key="grp.name">
          <h3 class="border-b border-gray-100 pb-2 text-xs font-bold uppercase tracking-wider text-gray-500">{{ grp.name }}</h3>
          <div class="divide-y divide-gray-100">
            <div v-for="ev in grp.events" :key="ev.key" :data-matrix-event="ev.key" class="py-3.5">
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <p class="text-sm font-semibold text-gray-900">{{ ev.label }}</p>
                    <span class="wa-m-badge" :class="ev.scope_kind === 'global' ? 'bg-violet-50 text-violet-700' : 'bg-gray-100 text-gray-600'">{{ scopeLabel(ev.scope_kind) }}</span>
                    <span v-if="settings.events?.[ev.key] === false" class="wa-m-badge bg-amber-100 text-amber-800">Nonaktif secara umum</span>
                  </div>
                  <p v-if="ev.description" class="mt-0.5 text-xs text-gray-500">{{ ev.description }}</p>
                </div>
                <span class="wa-m-badge shrink-0" :class="activeRolesCountForEvent(ev) > 0 ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'">
                  {{ activeRolesCountForEvent(ev) }} role aktif
                </span>
              </div>

              <!-- Role chips for this event -->
              <div v-if="rolesForEvent(ev).length" class="mt-2.5 flex flex-wrap gap-2">
                <button v-for="role in rolesForEvent(ev)" :key="role.name" type="button"
                  :data-matrix-cell="`${role.name}:${ev.key}`"
                  :disabled="!canManage || saving || !draftOf(role)?.enabled || (!canEnableEvent(role, ev) && !isEventConfiguredOn(role, ev))"
                  :title="eventTooltip(role, ev)"
                  class="wa-m-pill"
                  :class="eventPillClass(role, ev)"
                  @click="toggleRoleEvent(role, ev)">
                  <span class="wa-m-dot" :class="eventDotClass(role, ev)"></span>
                  <span class="font-semibold">{{ role.name }}</span>
                  <span v-if="shortUsersLabel(role)" class="text-[11px] opacity-75">({{ shortUsersLabel(role) }})</span>
                  <span v-if="!draftOf(role)?.enabled" class="wa-m-tag bg-amber-100 text-amber-800">role mati</span>
                  <span v-else-if="isPersonalOnly(role, ev) && isEventConfiguredOn(role, ev)" class="wa-m-tag bg-blue-100 text-blue-800">sendiri</span>
                </button>
              </div>
              <p v-else class="mt-2 text-xs italic text-gray-400">Tidak ada role yang sesuai filter ini untuk menerima pesan ini.</p>
            </div>
          </div>
        </AppCard>
      </div>
    </template>

    <!-- Sticky Save Bar -->
    <div v-if="dirtyRoleNames.length && canManage"
      class="sticky bottom-3 z-20 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-300 bg-white/95 p-3 shadow-lg backdrop-blur">
      <div class="min-w-0 text-xs text-gray-700">
        <p class="font-semibold text-amber-900">Perubahan belum disimpan pada {{ dirtyRoleNames.length }} role:</p>
        <p class="mt-0.5 truncate text-gray-600">{{ dirtyRoleNames.join(', ') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <AppButton variant="secondary" size="sm" :disabled="saving" @click="discardAll">Batalkan</AppButton>
        <AppButton size="sm" :loading="saving" @click="saveAll">Simpan Semua ({{ dirtyRoleNames.length }})</AppButton>
      </div>
    </div>

    <!-- Detail Modal (Outlet & Global Settings per Role) -->
    <AppModal v-model="showDetailModal" :title="`Pengaturan Detail — ${detailRoleName}`" size="2xl">
      <WhatsAppRoleNotifications v-if="detailRole" embedded :settings="settings" :events="events"
        :roles="[detailRole]" :role-name="detailRole.name" :can-manage="canManage" :can-view-logs="canViewLogs"
        :saving="saving" @save="onDetailSave" @reset="onDetailReset" />
    </AppModal>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import WhatsAppRoleNotifications from '@/components/WhatsAppRoleNotifications.vue'
import {
  normalizeRoleNotificationPolicy,
  roleNotificationEventState,
  selectedNotificationOutlets,
} from '@/utils/whatsappRoleNotifications.js'

const props = defineProps({
  settings: { type: Object, required: true },
  events: { type: Array, default: () => [] },
  roles: { type: Array, default: () => [] },
  canManage: Boolean,
  canViewUsers: Boolean,
  canViewLogs: Boolean,
  saving: Boolean,
  initialMode: { type: String, default: 'role' },
  initialSearch: { type: String, default: '' },
})
const emit = defineEmits(['save-many', 'save-one', 'reset-one'])

const viewMode = ref(props.initialMode === 'event' ? 'event' : 'role')
const search = ref(props.initialSearch || '')
const groupFilter = ref('')
const onlyActiveRoles = ref(true)
const drafts = ref({})
const baselines = ref({})
const showDetailModal = ref(false)
const detailRoleName = ref('')

const serialize = v => JSON.stringify(v)

watch(() => props.initialMode, m => { if (m === 'event' || m === 'role') viewMode.value = m })
watch(() => props.initialSearch, s => { if (typeof s === 'string') search.value = s })

watch(() => [props.roles, props.settings?.role_notifications], () => {
  for (const role of props.roles) {
    const saved = normalizeRoleNotificationPolicy(props.settings?.role_notifications?.[role.name] || role.policy, role)
    if (!drafts.value[role.name] || serialize(drafts.value[role.name]) === serialize(baselines.value[role.name])) {
      drafts.value[role.name] = structuredClone(saved)
    }
    baselines.value[role.name] = saved
  }
}, { immediate: true, deep: true })

const internalEvents = computed(() => props.events.filter(e => e.audience === 'internal'))
const allGroups = computed(() => [...new Set(internalEvents.value.map(e => e.group))])

function activeCount(role) {
  if (typeof role?.active_users === 'number') return role.active_users
  return role?.users || 0
}
const activeRoleCount = computed(() => props.roles.filter(r => activeCount(r) > 0).length)

function draftOf(role) {
  return drafts.value[role.name] || normalizeRoleNotificationPolicy(role.policy, role)
}
function isRoleDirty(roleName) {
  return drafts.value[roleName] && baselines.value[roleName] && serialize(drafts.value[roleName]) !== serialize(baselines.value[roleName])
}
const dirtyRoleNames = computed(() => props.roles.filter(r => isRoleDirty(r.name)).map(r => r.name))

function isEligible(role, ev) { return (role?.eligible_events || []).includes(ev.key) }
function isPersonal(role, ev) { return (role?.personal_events || []).includes(ev.key) }
function isAutomatic(role, ev) { return (role?.automatic_events || []).includes(ev.key) }
function isPersonalOnly(role, ev) { return isPersonal(role, ev) && !isEligible(role, ev) }
function canEnableEvent(role, ev) { return isEligible(role, ev) || isPersonal(role, ev) }

function isEventRelevant(role, ev) {
  const p = draftOf(role)
  return isEligible(role, ev) || isAutomatic(role, ev) || isPersonal(role, ev) || Boolean(p?.events?.[ev.key])
}
function isEventConfiguredOn(role, ev) {
  const p = draftOf(role)
  const val = p?.events?.[ev.key] || 'auto'
  if (val === 'off') return false
  if (val === 'on') return true
  return isAutomatic(role, ev) || isPersonal(role, ev)
}
function isCustomEvent(role, ev) {
  const val = draftOf(role)?.events?.[ev.key]
  return val === 'on' || val === 'off'
}
function isEventDirty(role, ev) {
  const cur = drafts.value[role.name]?.events?.[ev.key] || 'auto'
  const base = baselines.value[role.name]?.events?.[ev.key] || 'auto'
  return cur !== base
}

function toggleRoleEnabled(role, enabled) {
  if (!drafts.value[role.name]) return
  drafts.value[role.name].enabled = enabled
}

function toggleRoleEvent(role, ev) {
  if (!props.canManage || props.saving) return
  const p = drafts.value[role.name]
  if (!p) return
  const nextOn = !isEventConfiguredOn(role, ev)
  if (nextOn && !canEnableEvent(role, ev)) return
  if (nextOn) {
    if (isAutomatic(role, ev) || !isEligible(role, ev)) delete p.events[ev.key]
    else p.events[ev.key] = 'on'
  } else {
    if (!isAutomatic(role, ev) && !isPersonal(role, ev)) delete p.events[ev.key]
    else p.events[ev.key] = 'off'
  }
}

function eventStateOf(role, ev) {
  return roleNotificationEventState(ev, role, draftOf(role), props.settings)
}
function eventTooltip(role, ev) {
  const st = eventStateOf(role, ev)
  return `${role.name} · ${ev.label} — ${st.label}: ${st.reason}`
}
function eventPillClass(role, ev) {
  const p = draftOf(role)
  const dirty = isEventDirty(role, ev)
  const on = isEventConfiguredOn(role, ev)
  const ring = dirty ? ' ring-2 ring-amber-400' : ''
  if (!p?.enabled) return 'border-gray-200 bg-gray-50 text-gray-400' + ring
  if (!on) return 'border-gray-200 bg-white text-gray-500 hover:border-gray-300 hover:bg-gray-50' + ring
  if (isPersonalOnly(role, ev)) return 'border-blue-200 bg-blue-50 text-blue-800 hover:bg-blue-100' + ring
  return 'border-emerald-200 bg-emerald-50 text-emerald-800 hover:bg-emerald-100' + ring
}
function eventDotClass(role, ev) {
  const p = draftOf(role)
  if (!p?.enabled || !isEventConfiguredOn(role, ev)) return 'bg-gray-300'
  if (isPersonalOnly(role, ev)) return 'bg-blue-500'
  return 'bg-emerald-500'
}

function userNamesSummary(role) {
  const names = role?.active_user_names || []
  if (!names.length) return `${activeCount(role)} akun aktif`
  if (names.length <= 4) return names.join(', ')
  return `${names.slice(0, 4).join(', ')} +${names.length - 4} lainnya`
}
function shortUsersLabel(role) {
  const names = role?.active_user_names || []
  if (!names.length) return ''
  if (names.length === 1) return names[0]
  return `${names[0]} +${names.length - 1}`
}
function roleScopeSummary(role) {
  const p = draftOf(role)
  if (role.scope_type !== 'specific' && p?.outlet_mode === 'role') return 'Semua outlet'
  const outlets = selectedNotificationOutlets(role, p).map(o => o.name)
  const central = p?.outlet_mode === 'role' ? (role.central_work_unit_names || []) : []
  const all = [...outlets, ...central]
  if (!all.length) return 'Tanpa outlet'
  if (all.length === 1) return all[0]
  return `${all[0]} +${all.length - 1}`
}
function scopeLabel(kind) {
  return { outlet: 'Per outlet', global: 'Global / lintas outlet', mixed: 'Outlet & global' }[kind] || 'Sesuai kejadian'
}

const baseVisibleRoles = computed(() => {
  const list = onlyActiveRoles.value && activeRoleCount.value > 0
    ? props.roles.filter(r => activeCount(r) > 0 || isRoleDirty(r.name))
    : props.roles
  return [...list].sort((a, b) => {
    const aHas = activeCount(a) > 0 ? 0 : 1
    const bHas = activeCount(b) > 0 ? 0 : 1
    if (aHas !== bHas) return aHas - bHas
    if (a.name === 'superadmin') return 1
    if (b.name === 'superadmin') return -1
    return a.name.localeCompare(b.name)
  })
})

const filteredRoles = computed(() => {
  const q = search.value.trim().toLocaleLowerCase()
  return baseVisibleRoles.value.filter(role => {
    if (!q) return true
    const roleText = `${role.name} ${role.description || ''} ${(role.active_user_names || []).join(' ')} ${(role.outlet_names || []).join(' ')}`.toLocaleLowerCase()
    if (roleText.includes(q)) return true
    return internalEvents.value.some(ev => isEventRelevant(role, ev) && `${ev.label} ${ev.group}`.toLocaleLowerCase().includes(q))
  })
})

function roleEventGroups(role) {
  const q = search.value.trim().toLocaleLowerCase()
  const roleMatches = q && `${role.name} ${role.description || ''} ${(role.active_user_names || []).join(' ')} ${(role.outlet_names || []).join(' ')}`.toLocaleLowerCase().includes(q)
  const map = new Map()
  for (const ev of internalEvents.value) {
    if (!isEventRelevant(role, ev)) continue
    if (groupFilter.value && ev.group !== groupFilter.value) continue
    if (q && !roleMatches && !`${ev.label} ${ev.group} ${ev.description || ''}`.toLocaleLowerCase().includes(q)) continue
    if (!map.has(ev.group)) map.set(ev.group, [])
    map.get(ev.group).push(ev)
  }
  return [...map.entries()].map(([name, events]) => ({ name, events }))
}

const filteredEventGroups = computed(() => {
  const q = search.value.trim().toLocaleLowerCase()
  const map = new Map()
  for (const ev of internalEvents.value) {
    if (groupFilter.value && ev.group !== groupFilter.value) continue
    if (q) {
      const evText = `${ev.label} ${ev.group} ${ev.description || ''} ${ev.key}`.toLocaleLowerCase()
      const anyRoleMatch = baseVisibleRoles.value.some(r => isEventRelevant(r, ev) && `${r.name} ${(r.active_user_names || []).join(' ')}`.toLocaleLowerCase().includes(q))
      if (!evText.includes(q) && !anyRoleMatch) continue
    }
    if (!map.has(ev.group)) map.set(ev.group, [])
    map.get(ev.group).push(ev)
  }
  return [...map.entries()].map(([name, events]) => ({ name, events }))
})

function rolesForEvent(ev) {
  const q = search.value.trim().toLocaleLowerCase()
  const evMatches = !q || `${ev.label} ${ev.group} ${ev.description || ''} ${ev.key}`.toLocaleLowerCase().includes(q)
  return baseVisibleRoles.value.filter(role => {
    if (!isEventRelevant(role, ev)) return false
    if (!evMatches) {
      return `${role.name} ${(role.active_user_names || []).join(' ')}`.toLocaleLowerCase().includes(q)
    }
    return true
  })
}
function activeRolesCountForEvent(ev) {
  return rolesForEvent(ev).filter(role => draftOf(role)?.enabled && isEventConfiguredOn(role, ev)).length
}

function clearFilters() {
  search.value = ''
  groupFilter.value = ''
  onlyActiveRoles.value = false
}

function discardAll() {
  for (const role of props.roles) {
    if (baselines.value[role.name]) drafts.value[role.name] = structuredClone(baselines.value[role.name])
  }
}

function saveAll() {
  const items = dirtyRoleNames.value.map(name => ({
    role: name,
    policy: JSON.parse(JSON.stringify(drafts.value[name])),
  }))
  if (items.length) emit('save-many', items)
}

function resetRole(role) {
  if (role.has_policy) {
    if (!confirm(`Kembalikan notifikasi role "${role.name}" ke aturan bawaan otomatis?`)) return
    emit('reset-one', role.name)
  } else if (baselines.value[role.name]) {
    drafts.value[role.name] = structuredClone(baselines.value[role.name])
  }
}

const detailRole = computed(() => props.roles.find(r => r.name === detailRoleName.value) || null)
function openDetailModal(name) {
  detailRoleName.value = name
  showDetailModal.value = true
}
function onDetailSave(payload) {
  emit('save-one', payload)
}
function onDetailReset(name) {
  emit('reset-one', name)
}

function acceptSaved(roleNames) {
  for (const name of roleNames) {
    const role = props.roles.find(r => r.name === name)
    if (!role) continue
    const saved = normalizeRoleNotificationPolicy(props.settings?.role_notifications?.[name] || role.policy, role)
    baselines.value[name] = saved
    drafts.value[name] = structuredClone(saved)
  }
}
function focusEvent(eventLabel) {
  viewMode.value = 'event'
  groupFilter.value = ''
  search.value = eventLabel || ''
}
defineExpose({ acceptSaved, focusEvent })
</script>

<style scoped>
.wa-m-input {
  width: 100%; min-width: 0; padding: .48rem .7rem; border-radius: .6rem; font-size: .8rem;
  border: 1px solid #d1d5db; background: #fff; color: #1f2937; outline: none;
}
.wa-m-input:focus { border-color: #10b981; box-shadow: 0 0 0 3px rgba(16,185,129,.12); }
.wa-m-badge {
  display: inline-flex; align-items: center; border-radius: 999px;
  padding: .12rem .5rem; font-size: .66rem; font-weight: 600;
}
.wa-m-btn {
  border: 1px solid #e5e7eb; background: #fff; color: #374151; border-radius: .5rem;
  padding: .3rem .65rem; font-size: .72rem; font-weight: 600; transition: all .15s; cursor: pointer;
}
.wa-m-btn:hover:not(:disabled) { border-color: #10b981; color: #047857; background: #ecfdf5; }
.wa-m-btn:disabled { opacity: .5; cursor: default; }
.wa-m-pill {
  display: inline-flex; align-items: center; gap: .4rem; max-width: 100%;
  border-width: 1px; border-radius: .6rem; padding: .35rem .65rem;
  font-size: .74rem; line-height: 1.2; transition: all .15s; cursor: pointer; text-align: left;
}
.wa-m-pill:disabled { opacity: .55; cursor: default; }
.wa-m-dot { width: .45rem; height: .45rem; border-radius: 999px; flex-shrink: 0; }
.wa-m-tag {
  border-radius: 999px; padding: .05rem .38rem; font-size: .6rem; font-weight: 700; flex-shrink: 0;
}
</style>
