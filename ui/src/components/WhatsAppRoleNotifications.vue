<template>
  <div class="space-y-4">
    <AppCard>
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 max-w-3xl">
          <h2 class="text-base font-bold text-gray-900">Notifikasi per Role</h2>
          <p class="mt-1 text-sm text-gray-600">Atur pesan yang diterima setiap posisi dan outlet yang ingin dipantau. Pengaturan ini berlaku untuk semua pengguna pada role tersebut.</p>
          <p class="mt-2 text-xs text-gray-500">Nomor atau grup yang didaftarkan sebagai Penerima tambahan mengikuti pengaturannya sendiri. Nomor pengguna yang juga terdaftar di sana masih dapat menerima pesan melalui jalur tambahan itu.</p>
        </div>
        <RouterLink v-if="canViewRoles" to="/roles" class="text-xs font-semibold text-emerald-700 hover:underline">Lihat hak akses role →</RouterLink>
      </div>
      <div class="mt-4 max-w-xl">
        <label class="mb-1 block text-xs font-semibold text-gray-700">Pilih role yang akan diatur</label>
        <SearchSelect :model-value="selectedRoleName" :options="rolePickerOptions" value-key="name" label-key="label"
          aria-label="Pilih role notifikasi" placeholder="Pilih role…" search-placeholder="Cari nama atau deskripsi role…" @update:model-value="selectRole" />
      </div>
      <p v-if="unsavedRoleNames.length" class="mt-3 rounded-lg bg-amber-50 p-2 text-xs text-amber-900">Perubahan belum disimpan untuk: <strong>{{ unsavedRoleNames.join(', ') }}</strong>. Pilihan tetap tersimpan selama Anda berpindah role di halaman ini; buka kembali role tersebut lalu klik Simpan.</p>
      <p v-if="!roles.length" class="mt-3 text-sm text-gray-500">Daftar role belum tersedia. Muat ulang halaman untuk mencoba lagi.</p>
      <p v-else-if="selectedRoleName && !selectedRole" class="mt-3 text-sm text-amber-800">Role pada tautan ini sudah tidak tersedia. Pilih role lain dari daftar.</p>
    </AppCard>

    <template v-if="selectedRole && policy">
      <AppCard>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h3 class="break-words text-base font-bold text-gray-900">{{ selectedRole.name }}</h3>
            <p class="mt-1 text-sm text-gray-500">{{ selectedRole.description || 'Belum ada deskripsi role.' }}</p>
            <p class="mt-2 text-xs text-gray-600"><strong>{{ selectedRole.users || 0 }} akun aktif siap menerima</strong> · Nomor WA sudah diisi dan notifikasi pribadi pengguna aktif.</p>
            <p v-if="!selectedRole.users" class="mt-1 text-xs text-amber-800">Belum ada penerima yang siap. Isi nomor dan aktifkan notifikasi pada menu Pengguna.</p>
          </div>
          <label class="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-semibold"
            :class="policy.enabled ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-gray-200 bg-gray-50 text-gray-600'">
            <input v-model="policy.enabled" type="checkbox" aria-label="Aktifkan notifikasi role" class="h-4 w-4 accent-emerald-600" :disabled="!canManage || saving" />
            Notifikasi role {{ policy.enabled ? 'aktif' : 'nonaktif' }}
          </label>
        </div>
        <div v-if="!policy.enabled" class="mt-4 rounded-lg bg-amber-50 p-3 text-xs text-amber-900">Semua notifikasi role ini dihentikan, termasuk kabar pengajuan milik sendiri. Pengguna tetap memiliki hak akses aplikasi sesuai role-nya.</div>
        <div class="mt-4 grid gap-3 md:grid-cols-2">
          <div class="rounded-xl border border-gray-200 bg-gray-50 p-3">
            <p class="text-xs font-bold uppercase tracking-wide text-gray-500">Hak akses aplikasi</p>
            <p class="mt-1 text-sm font-semibold text-gray-800">{{ selectedRole.scope_type === 'specific' ? 'Unit kerja tertentu' : 'Semua outlet & unit kerja' }}</p>
            <p class="mt-1 text-xs text-gray-600">{{ accessScopeLabel }}</p>
            <p class="mt-2 text-xs text-gray-500">Hak akses menentukan data yang boleh dibuka. Mengubah notifikasi di halaman ini tidak menambah atau mengurangi hak akses tersebut.</p>
          </div>
          <div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3">
            <p class="text-xs font-bold uppercase tracking-wide text-emerald-700">Cakupan pesan WhatsApp</p>
            <p class="mt-1 text-sm font-semibold text-emerald-900">{{ notificationScopeLabel }}</p>
            <p class="mt-2 text-xs text-emerald-800">Pesan hanya memuat kejadian dari outlet atau unit kerja yang tercakup di sini. Pilihan outlet dapat mempersempit cakupan role, tanpa membuka data di luar hak aksesnya.</p>
          </div>
        </div>
        <fieldset class="mt-4" :disabled="!canManage || saving">
          <legend class="text-sm font-semibold text-gray-800">Outlet & unit kerja sumber notifikasi</legend>
          <div class="mt-2 grid gap-2 sm:grid-cols-2">
            <label class="flex items-start gap-2 rounded-lg border p-3 text-sm" :class="policy.outlet_mode === 'role' ? 'border-emerald-300 bg-emerald-50' : 'border-gray-200'">
              <input v-model="policy.outlet_mode" type="radio" value="role" :name="`wa-outlet-mode-${selectedRole.name}`" class="mt-0.5 accent-emerald-600" />
              <span><strong>Sesuai cakupan role</strong><small class="mt-1 block text-xs text-gray-500">Mengikuti outlet dan unit kerja yang diizinkan pada hak akses, termasuk bila cakupannya berubah.</small></span>
            </label>
            <label class="flex items-start gap-2 rounded-lg border p-3 text-sm" :class="policy.outlet_mode === 'selected' ? 'border-emerald-300 bg-emerald-50' : 'border-gray-200'">
              <input v-model="policy.outlet_mode" type="radio" value="selected" :name="`wa-outlet-mode-${selectedRole.name}`" class="mt-0.5 accent-emerald-600" />
              <span><strong>Pilih outlet tertentu</strong><small class="mt-1 block text-xs text-gray-500">Terima pesan hanya dari sebagian outlet yang boleh diakses role ini. Unit kerja pusat tanpa outlet tidak ikut dalam pilihan ini.</small></span>
            </label>
          </div>
          <div v-if="policy.outlet_mode === 'selected'" class="mt-3 rounded-lg border border-gray-200 p-3">
            <input v-model="outletSearch" type="search" aria-label="Cari outlet notifikasi" placeholder="Cari outlet…" class="wa-field" />
            <div class="mt-2 grid max-h-52 gap-2 overflow-y-auto sm:grid-cols-2">
              <label v-for="outlet in filteredOutlets" :key="outlet.id" class="flex items-center gap-2 text-sm text-gray-700">
                <input v-model="policy.outlet_ids" type="checkbox" :value="outlet.id" class="h-4 w-4 accent-emerald-600" />{{ outlet.name }}
              </label>
              <p v-if="!filteredOutlets.length" class="text-xs text-gray-500">Tidak ada outlet yang cocok dalam cakupan role.</p>
            </div>
            <p v-if="!effectiveOutlets.length" class="mt-2 text-xs font-semibold text-amber-800">Belum ada outlet dipilih. Pesan per outlet tidak dikirim sampai Anda memilih outlet.</p>
          </div>
          <div v-if="invalidSelectedOutlets" class="mt-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-900">
            Ada outlet tersimpan yang sudah berada di luar cakupan role. Hapus outlet tersebut sebelum menyimpan aturan baru.
            <button type="button" class="ml-1 font-semibold underline" @click="policy.outlet_ids = policy.outlet_ids.filter(id => allowedOutlets.some(outlet => outlet.id === id))">Hapus outlet di luar cakupan</button>
          </div>
          <label v-if="selectedRole.scope_type !== 'specific'" class="mt-4 flex items-start gap-2 rounded-lg border border-violet-200 bg-violet-50 p-3 text-sm">
            <input v-model="policy.include_global" type="checkbox" aria-label="Terima pesan global lintas outlet" :disabled="policy.outlet_mode === 'selected'" class="mt-0.5 h-4 w-4 accent-violet-600" />
            <span><strong class="text-violet-900">Terima pesan global / lintas outlet</strong><small class="mt-1 block text-xs text-violet-800">Misalnya rekap gabungan atau kondisi gudang pusat yang tidak mewakili satu outlet. Tersedia bila notifikasi mengikuti seluruh cakupan role.</small></span>
          </label>
          <p v-if="selectedRole.scope_type !== 'specific' && policy.outlet_mode === 'selected'" class="mt-2 text-xs text-violet-800">Pilihan outlet tertentu mempersempit seluruh isi pesan. Pilih Sesuai cakupan role untuk menerima rekap seluruh outlet atau informasi pusat.</p>
          <p v-if="selectedRole.scope_type === 'specific'" class="mt-4 rounded-lg bg-gray-50 p-3 text-xs text-gray-600"><strong>Pesan global tidak tersedia untuk role ini.</strong> Rekap yang memuat data lintas outlet hanya dapat diterima role dengan akses semua outlet. Pesan tentang outlet atau unit kerja milik role sendiri tetap mengikuti pilihan di atas.</p>
        </fieldset>
        <p v-if="selectedRole.personal_events?.length" class="mt-4 rounded-lg border border-blue-100 bg-blue-50 p-3 text-xs leading-relaxed text-blue-900"><strong>Kabar pengadaan pribadi:</strong> pengguna dengan posisi pengaju menerima kabar tentang pengajuannya sendiri. Memilih Otomatis tidak membuatnya menerima pengajuan pengguna lain. Pilihan Nonaktif pada jenis pesan atau tombol nonaktif role juga menghentikan kabar pribadi tersebut.</p>
      </AppCard>

      <AppCard>
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-bold text-gray-900">Jenis pesan untuk {{ selectedRole.name }}</h3>
            <p class="mt-1 max-w-3xl text-xs leading-relaxed text-gray-500"><strong>Otomatis</strong> mengikuti hak akses dan aturan penerima event. <strong>Aktif</strong> menetapkan penerimaan khusus role ini bila hak aksesnya sesuai. <strong>Nonaktif</strong> menghentikan pesan untuk role ini, termasuk pengajuan pribadi. Pengaturan umum yang nonaktif tetap berlaku untuk semua role.</p>
          </div>
          <button v-if="canManage" type="button" class="text-xs font-semibold text-emerald-700 hover:underline disabled:opacity-50" :disabled="saving" @click="restoreDefaults">Pulihkan isian bawaan</button>
        </div>
        <div v-if="!settings.enabled" class="mt-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-900" role="status">Pengiriman WhatsApp umum sedang nonaktif. Pilihan role dapat disimpan, tetapi pesan belum akan dikirim.</div>
        <div class="mt-4 grid gap-2 sm:grid-cols-[1fr_auto]">
          <input v-model="eventSearch" type="search" aria-label="Cari jenis pesan role" placeholder="Cari jenis pesan atau kelompok…" class="wa-field" />
          <SearchableSelect v-model="scopeFilter" aria-label="Filter cakupan jenis pesan" class="wa-field sm:min-w-44">
            <option value="">Semua cakupan</option><option value="outlet">Per outlet</option><option value="global">Global / lintas outlet</option><option value="mixed">Outlet & global</option>
          </SearchableSelect>
        </div>
        <div v-if="!eventGroups.length" class="py-8 text-center text-sm text-gray-500">Tidak ada jenis pesan yang cocok dengan pencarian.</div>
        <section v-for="group in eventGroups" :key="group.name" class="mt-5">
          <h4 class="border-b border-gray-100 pb-2 text-xs font-bold uppercase tracking-wide text-gray-500">{{ group.name }}</h4>
          <div class="divide-y divide-gray-100">
            <div v-for="event in group.events" :key="event.key" :data-event="event.key" class="wa-role-event grid gap-3 py-4 sm:grid-cols-[1fr_12rem]">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <p class="text-sm font-semibold text-gray-900">{{ event.label }}</p>
                  <span class="wa-badge" :class="event.scope_kind === 'global' ? 'bg-violet-50 text-violet-800' : 'bg-gray-100 text-gray-600'">{{ scopeLabel(event.scope_kind) }}</span>
                  <span v-if="settings.events?.[event.key] === false" class="wa-badge bg-amber-50 text-amber-800">Nonaktif secara umum</span>
                </div>
                <p v-if="event.description" class="mt-1 text-xs leading-relaxed text-gray-500">{{ event.description }}</p>
                <p v-if="event.scope_description" class="mt-1 text-xs leading-relaxed text-gray-600">{{ event.scope_description }}</p>
                <p class="mt-2 text-xs leading-relaxed" :class="eventState(event).active ? 'text-emerald-800' : 'text-gray-500'"><strong>{{ eventState(event).label }}:</strong> {{ eventState(event).reason }}</p>
                <p v-if="(settings.event_roles?.[event.key] || []).length" class="mt-1 text-[11px] text-gray-500">Aturan Otomatis mengikuti daftar penerima event: {{ settings.event_roles[event.key].join(', ') }}.</p>
              </div>
              <div class="self-start">
                <SearchableSelect :model-value="policy.events[event.key] || 'auto'" :aria-label="`Aturan pesan ${event.label}`" class="wa-field"
                  :disabled="!canManage || saving || (!isEligible(event) && !isPersonal(event))" @update:model-value="setEvent(event.key, $event)">
                  <option value="auto">Otomatis</option><option value="on" :disabled="!isEligible(event)">Aktif</option><option value="off">Nonaktif</option>
                </SearchableSelect>
                <p v-if="!isEligible(event) && isPersonal(event)" class="mt-1 text-[11px] text-gray-500">Otomatis / Nonaktif untuk pengajuan sendiri.</p>
              </div>
            </div>
          </div>
        </section>
      </AppCard>

      <p class="px-1 text-xs leading-relaxed text-gray-500">Berlaku untuk notifikasi baru setelah disimpan. Pesan yang sudah masuk antrean dapat dibatalkan di <RouterLink to="/whatsapp/messages" class="font-semibold text-emerald-700 hover:underline">Log Pesan</RouterLink>.</p>
      <div class="sticky bottom-3 rounded-xl border border-emerald-200 bg-white p-3 shadow-lg">
        <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3">
          <div class="min-w-0 flex-1 text-xs text-gray-600">
            <p class="font-semibold text-gray-900">Pratinjau penerimaan · {{ selectedRole.name }}</p>
            <p class="mt-1 break-words">{{ previewSummary }}</p>
            <p v-if="dirty" class="mt-1 font-semibold text-amber-700">Perubahan role ini belum disimpan.</p>
          </div>
          <div v-if="canManage" class="flex flex-wrap items-center justify-end gap-2">
            <button v-if="selectedRole.has_policy" type="button" :disabled="saving" class="px-2 py-2 text-xs font-semibold text-gray-600 hover:underline disabled:opacity-50" @click="emit('reset', selectedRole.name)">Hapus aturan khusus</button>
            <AppButton :loading="saving" :disabled="!dirty || invalidSelectedOutlets" @click="save">Simpan notifikasi role</AppButton>
          </div>
          <span v-else class="text-xs text-gray-500">Akses lihat saja</span>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppButton from '@/components/ui/AppButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { defaultRoleNotificationPolicy, normalizeRoleNotificationPolicy, roleNotificationOutlets, selectedNotificationOutlets, roleNotificationEventState } from '@/utils/whatsappRoleNotifications.js'
const props = defineProps({
  settings: { type: Object, required: true }, events: { type: Array, default: () => [] }, roles: { type: Array, default: () => [] },
  roleName: { type: String, default: '' }, canManage: Boolean, canViewRoles: Boolean, saving: Boolean,
})
const emit = defineEmits(['select-role', 'save', 'reset'])
const selectedRoleName = ref(props.roleName || '')
const drafts = ref({}), baselines = ref({})
const eventSearch = ref(''), scopeFilter = ref(''), outletSearch = ref('')
const rolePickerOptions = computed(() => props.roles.map(role => ({ ...role, label: `${role.name}${role.description ? ` — ${role.description}` : ''}` })))
const selectedRole = computed(() => props.roles.find(role => role.name === selectedRoleName.value))
const policy = computed(() => drafts.value[selectedRoleName.value])
const serialize = value => JSON.stringify(value)
watch(() => [props.roles, props.settings.role_notifications], () => {
  for (const role of props.roles) {
    const saved = normalizeRoleNotificationPolicy(props.settings.role_notifications?.[role.name] || role.policy, role)
    if (!drafts.value[role.name] || serialize(drafts.value[role.name]) === serialize(baselines.value[role.name])) drafts.value[role.name] = structuredClone(saved)
    baselines.value[role.name] = saved
  }
  if (!selectedRoleName.value && props.roles.length) selectRole(props.roles[0].name)
}, { immediate: true, deep: true })
watch(() => props.roleName, value => { if (value) selectedRoleName.value = value })
function selectRole(name) { selectedRoleName.value = name; outletSearch.value = ''; emit('select-role', name) }
const dirty = computed(() => serialize(policy.value) !== serialize(baselines.value[selectedRoleName.value]))
const unsavedRoleNames = computed(() => props.roles.filter(role => drafts.value[role.name] && serialize(drafts.value[role.name]) !== serialize(baselines.value[role.name])).map(role => role.name))
watch(() => policy.value?.outlet_mode, mode => { if (mode === 'selected' && policy.value) policy.value.include_global = false })
const allowedOutlets = computed(() => roleNotificationOutlets(selectedRole.value))
const effectiveOutlets = computed(() => selectedNotificationOutlets(selectedRole.value, policy.value))
const filteredOutlets = computed(() => allowedOutlets.value.filter(outlet => outlet.name.toLocaleLowerCase().includes(outletSearch.value.toLocaleLowerCase().trim())))
const centralWorkUnitNames = computed(() => selectedRole.value?.central_work_unit_names || [])
const accessScopeLabel = computed(() => {
  if (selectedRole.value?.scope_type !== 'specific') return 'Role ini boleh mengakses seluruh outlet dan unit kerja. Pesan global dapat diaktifkan secara terpisah.'
  const scopes = allowedOutlets.value.map(outlet => outlet.name)
  if (centralWorkUnitNames.value.length) scopes.push(`Unit kerja pusat: ${centralWorkUnitNames.value.join(', ')}`)
  return scopes.join(' · ') || 'Belum ada outlet atau unit kerja dalam cakupan role.'
})
const notificationScopeLabel = computed(() => {
  if (policy.value?.outlet_mode === 'role' && selectedRole.value?.scope_type !== 'specific') return 'Semua outlet dan unit kerja yang diizinkan role'
  const scopes = effectiveOutlets.value.map(outlet => outlet.name)
  if (policy.value?.outlet_mode === 'role' && centralWorkUnitNames.value.length) scopes.push(`Unit kerja pusat: ${centralWorkUnitNames.value.join(', ')}`)
  return scopes.join(' · ') || 'Belum ada outlet dipilih'
})
const invalidSelectedOutlets = computed(() => policy.value?.outlet_mode === 'selected' && (policy.value?.outlet_ids || []).some(id => !allowedOutlets.value.some(outlet => outlet.id === id)))
const eventGroups = computed(() => {
  const groups = new Map(), search = eventSearch.value.trim().toLocaleLowerCase()
  for (const event of props.events.filter(event => event.audience === 'internal')) {
    if (scopeFilter.value && event.scope_kind !== scopeFilter.value) continue
    if (search && !`${event.label} ${event.group} ${event.description || ''}`.toLocaleLowerCase().includes(search)) continue
    if (!groups.has(event.group)) groups.set(event.group, [])
    groups.get(event.group).push(event)
  }
  return [...groups].map(([name, events]) => ({ name, events }))
})
function scopeLabel(kind) { return { outlet: 'Per outlet', global: 'Global / lintas outlet', mixed: 'Outlet & global' }[kind] || 'Sesuai kejadian' }
function isEligible(event) { return (selectedRole.value?.eligible_events || []).includes(event.key) }
function isPersonal(event) { return (selectedRole.value?.personal_events || []).includes(event.key) }
function eventState(event) { return roleNotificationEventState(event, selectedRole.value, policy.value, props.settings) }
function setEvent(key, value) { if (value === 'auto') delete policy.value.events[key]; else policy.value.events[key] = value }
function restoreDefaults() { drafts.value[selectedRoleName.value] = defaultRoleNotificationPolicy(selectedRole.value) }
const previewSummary = computed(() => {
  if (!policy.value?.enabled) return 'Semua pesan untuk role ini nonaktif, termasuk pengajuan pribadi.'
  const count = props.events.filter(event => event.audience === 'internal' && eventState(event).active).length
  const global = selectedRole.value?.scope_type !== 'specific' && policy.value.outlet_mode === 'role' && policy.value.include_global ? 'Global / lintas outlet aktif' : 'Global / lintas outlet nonaktif'
  return `${notificationScopeLabel.value} · ${global} · ${count} jenis pesan aktif${!props.settings.enabled ? ' · Pengiriman umum sedang nonaktif' : ''}.`
})
function save() { if (selectedRole.value && policy.value) emit('save', { role: selectedRoleName.value, policy: JSON.parse(JSON.stringify(policy.value)) }) }
function acceptSaved(roleName) {
  const role = props.roles.find(role => role.name === roleName)
  if (!role) return
  const saved = normalizeRoleNotificationPolicy(props.settings.role_notifications?.[roleName] || role.policy, role)
  baselines.value[roleName] = saved; drafts.value[roleName] = structuredClone(saved)
}
defineExpose({ acceptSaved })
</script>

<style scoped>
.wa-field { width:100%; min-width:0; padding:.5rem .7rem; border:1px solid #d1d5db; border-radius:.6rem; background:#fff; font-size:.8rem; color:#374151; }
.wa-field:focus { outline:2px solid #a7f3d0; outline-offset:1px; }
.wa-badge { display:inline-block; border-radius:999px; padding:.15rem .5rem; font-size:.65rem; font-weight:600; }
</style>
