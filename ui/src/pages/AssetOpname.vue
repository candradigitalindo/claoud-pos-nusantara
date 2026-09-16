<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Opname Aset</h1>
        <p class="mt-0.5 text-sm text-gray-500">Hitung fisik per outlet, lalu terapkan selisihnya ke catatan.</p>
      </div>
      <AppButton v-if="canCreate" class="w-full sm:w-auto" @click="createModal = true">+ Buka Sesi Opname</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Sesi berjalan -->
    <AppCard v-if="active" :padding="false">
      <div class="border-b border-gray-100 p-4">
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-mono text-sm font-semibold text-gray-900">{{ active.opname_number }}</span>
          <span :class="opnameStatusCls(active.status)">{{ OPNAME_STATUSES[active.status] || active.status }}</span>
          <span class="text-sm text-gray-600">{{ active.outlet_name }}</span>
          <span class="ml-auto text-xs text-gray-500">{{ active.counted_count }}/{{ active.item_count }} baris terhitung</span>
        </div>
        <div v-if="active.counted_count" class="mt-2 flex flex-wrap gap-3 text-xs">
          <span class="text-gray-500">Selisih: <strong :class="active.diff_count ? 'text-amber-600' : 'text-emerald-700'">{{ active.diff_count }}</strong> baris</span>
          <span class="text-gray-500">Akurasi: <strong>{{ (active.accuracy * 100).toFixed(0) }}%</strong></span>
        </div>
      </div>

      <div class="divide-y divide-gray-100">
        <div v-for="it in active.items" :key="it.id" class="p-3 sm:p-4">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="break-words font-medium text-gray-900">{{ it.asset_name || it.found_name }}</p>
              <p class="font-mono text-[11px] text-gray-500">
                {{ it.asset_no || 'temuan baru' }} · sistem {{ it.system_qty }} {{ it.unit }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <input v-if="canCount && active.status === 'berjalan'" v-model.number="counts[it.id]" type="number"
                inputmode="numeric" min="0" class="form-input w-24 text-right" placeholder="hitung" />
              <span v-else class="text-sm text-gray-700">{{ it.counted_qty ?? '—' }}</span>
              <span v-if="it.counted_qty !== null && it.counted_qty !== undefined"
                :class="it.diff === 0 ? 'badge-ok' : 'badge-warn'">
                {{ it.diff === 0 ? 'cocok' : (it.diff > 0 ? `+${it.diff}` : it.diff) }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div v-if="canCount && active.status === 'berjalan'" class="space-y-3 border-t border-gray-100 p-4">
        <details class="rounded-lg border border-gray-200">
          <summary class="cursor-pointer select-none rounded-lg bg-gray-50 px-3 py-2 text-sm font-medium text-gray-700">
            + Catat barang yang tidak ada di daftar
          </summary>
          <div class="grid grid-cols-1 gap-3 border-t border-gray-100 p-3 sm:grid-cols-3">
            <input v-model="found.found_name" class="form-input" placeholder="Nama barang" />
            <input v-model.number="found.counted_qty" type="number" inputmode="numeric" min="1" class="form-input" placeholder="Jumlah" />
            <input v-model="found.location" class="form-input" placeholder="Lokasi" />
          </div>
        </details>
        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <AppButton variant="secondary" :loading="savingCount" @click="saveCount">Simpan Hitungan</AppButton>
          <AppButton v-if="canApprove" :loading="approving" @click="approve">Terapkan Selisih</AppButton>
        </div>
        <p class="text-xs text-gray-500">
          Menerapkan selisih akan menyesuaikan jumlah aset, menjadikan temuan sebagai aset baru, dan
          mengajukan penghapusan untuk barang yang tidak ditemukan.
        </p>
      </div>
    </AppCard>

    <!-- Riwayat sesi -->
    <AppCard :padding="false">
      <p class="border-b border-gray-100 p-3 text-xs font-bold uppercase tracking-wide text-gray-500">Riwayat Sesi</p>
      <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">
        Belum ada sesi opname. Opname dianjurkan minimal dua kali setahun per outlet.
      </div>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="s in rows" :key="s.id" class="flex flex-wrap items-center gap-2 p-4">
          <button class="min-w-0 flex-1 text-left" @click="open(s)">
            <p class="font-mono text-sm font-semibold text-gray-900">{{ s.opname_number }}</p>
            <p class="text-xs text-gray-500">{{ s.outlet_name }} · {{ s.created_at }} · {{ s.created_by }}</p>
          </button>
          <span :class="opnameStatusCls(s.status)">{{ OPNAME_STATUSES[s.status] || s.status }}</span>
          <span v-if="s.counted_count" class="text-xs text-gray-500">akurasi {{ (s.accuracy * 100).toFixed(0) }}%</span>
        </li>
      </ul>
    </AppCard>

    <AppModal v-model="createModal" title="Buka Sesi Opname" size="lg">
      <form class="space-y-3" @submit.prevent="createSession">
        <div>
          <label class="lbl">Outlet <span class="text-red-500">*</span></label>
          <SearchSelect v-model="newSession.outlet_id" :options="outlets" placeholder="Pilih outlet…" />
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <input v-model="newSession.notes" class="form-input" placeholder="mis. opname semester 2" />
        </div>
        <p class="rounded-lg bg-gray-50 p-3 text-xs text-gray-600">
          Daftar aset outlet akan dibekukan sebagai baris hitungan. Aset yang sedang dalam perjalanan
          mutasi tidak diikutkan — barangnya memang tidak ada di tempat.
        </p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="createModal = false">Batal</button>
          <AppButton type="submit" :loading="creating">Buka Sesi</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { assetOpnamesApi } from '@/api/assetOps.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { OPNAME_STATUSES, opnameStatusCls } from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.opname.create')
const canCount = canCreate
const canApprove = auth.hasPermission('assets.opname.approve')

const rows = ref([])
const outlets = ref([])
const active = ref(null)
const counts = ref({})
const found = ref({ found_name: '', counted_qty: 1, location: '' })
const loading = ref(false)
const creating = ref(false)
const savingCount = ref(false)
const approving = ref(false)
const errorMsg = ref('')
const createModal = ref(false)
const newSession = ref({ outlet_id: '', notes: '' })

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d ?? null }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetOpnamesApi.list())
    const running = rows.value.find(s => s.status === 'berjalan')
    if (running) await open(running)
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat sesi opname' } finally { loading.value = false }
}

async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

async function open(s) {
  try {
    active.value = asObject(await assetOpnamesApi.get(s.id))
    counts.value = Object.fromEntries((active.value.items || []).map(i => [i.id, i.counted_qty ?? null]))
  } catch (e) { toast.error(e?.message || 'Gagal membuka sesi') }
}

async function createSession() {
  if (!newSession.value.outlet_id) { toast.error('Pilih outlet'); return }
  creating.value = true
  try {
    const s = asObject(await assetOpnamesApi.create(newSession.value))
    toast.success(`Sesi ${s.opname_number} dibuka`)
    createModal.value = false
    newSession.value = { outlet_id: '', notes: '' }
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal membuka sesi') } finally { creating.value = false }
}

async function saveCount() {
  const lines = Object.entries(counts.value)
    .filter(([, v]) => v !== null && v !== undefined && v !== '')
    .map(([item_id, v]) => ({ item_id, counted_qty: Number(v) }))
  if (found.value.found_name?.trim()) {
    lines.push({ found_name: found.value.found_name, counted_qty: Number(found.value.counted_qty) || 1, location: found.value.location })
  }
  if (!lines.length) { toast.error('Belum ada hitungan yang diisi'); return }
  savingCount.value = true
  try {
    active.value = asObject(await assetOpnamesApi.saveCount(active.value.id, lines))
    counts.value = Object.fromEntries((active.value.items || []).map(i => [i.id, i.counted_qty ?? null]))
    found.value = { found_name: '', counted_qty: 1, location: '' }
    toast.success('Hitungan tersimpan')
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan hitungan') } finally { savingCount.value = false }
}

async function approve() {
  if (!window.confirm('Terapkan selisih opname ini? Jumlah aset akan disesuaikan dan barang hilang diajukan penghapusannya.')) return
  approving.value = true
  try {
    await assetOpnamesApi.approve(active.value.id)
    toast.success('Selisih diterapkan')
    active.value = null
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menerapkan selisih') } finally { approving.value = false }
}

onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; min-height: 40px; }
.btn-ghost:hover { background: #e5e7eb; }
</style>
