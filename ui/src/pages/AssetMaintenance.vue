<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="min-w-0">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Perawatan Aset</h1>
      <p class="mt-0.5 text-sm text-gray-500">Jadwal perawatan preventif dan riwayat perbaikan seluruh outlet.</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard class="min-w-0">
        <p class="stat-lbl">Terlambat</p>
        <p class="stat-val" :class="summary.overdue ? 'text-red-600' : ''">{{ summary.overdue }}</p>
        <p class="stat-sub">lewat tanggal rencana</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Jatuh Tempo 7 Hari</p>
        <p class="stat-val" :class="summary.due_week ? 'text-amber-600' : ''">{{ summary.due_week }}</p>
        <p class="stat-sub">{{ summary.due_month }} dalam 30 hari</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Sedang Dikerjakan</p>
        <p class="stat-val">{{ summary.in_progress }}</p>
        <p class="stat-sub">aset terkunci selama dikerjakan</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Biaya Tahun Ini</p>
        <p class="stat-val">{{ formatRupiah(summary.cost_year) }}</p>
        <p class="stat-sub">{{ summary.preventive_year }} preventif · {{ summary.corrective_year }} korektif</p>
      </AppCard>
    </div>

    <!-- Tab -->
    <div class="-mx-1 overflow-x-auto">
      <div class="flex gap-1 px-1 pb-1">
        <button v-for="t in TABS" :key="t.key" @click="setTab(t.key)"
          class="shrink-0 rounded-lg px-3 py-2 text-xs font-semibold transition"
          :class="tab === t.key ? 'bg-emerald-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'">
          {{ t.label }}
        </button>
      </div>
    </div>

    <AppCard>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <SearchSelect v-model="filterOutlet" :options="outletOptions" placeholder="Semua outlet" @change="load" />
        <select v-model="filterType" @change="load" class="form-input">
          <option value="">Semua jenis</option>
          <option v-for="(lbl, key) in MTYPES" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="search" @input="debouncedLoad" type="search" placeholder="Cari aset / nomor WO…" class="form-input" />
      </div>
    </AppCard>

    <AppCard :padding="false">
      <!-- Mobile -->
      <div class="sm:hidden">
        <div v-if="loading" class="space-y-3 p-4">
          <div v-for="i in 3" :key="i" class="animate-pulse space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200"></div><div class="h-3 w-1/2 rounded bg-gray-100"></div>
          </div>
        </div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">{{ emptyText }}</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="m in rows" :key="m.id" class="space-y-2 p-4">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="break-words font-semibold text-gray-900">{{ m.asset_name }}</p>
                <p class="font-mono text-[11px] text-gray-500">{{ m.wo_number }} · {{ m.asset_no }}</p>
              </div>
              <div class="flex shrink-0 flex-col items-end gap-1">
                <span :class="woStatusCls(m.status)">{{ woStatusLabel(m.status) }}</span>
                <span v-if="m.due_in_days !== undefined && m.due_in_days !== null" :class="dueCls(m.due_in_days)">{{ dueLabel(m.due_in_days) }}</span>
              </div>
            </div>
            <p class="break-words text-sm text-gray-700">{{ m.description }}</p>
            <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Outlet</dt><dd class="text-gray-700">{{ m.outlet_name || '—' }}</dd>
              <dt class="text-gray-400">Jenis</dt><dd class="text-gray-700">{{ MTYPES[m.type] || m.type }}</dd>
              <dt class="text-gray-400">Tanggal</dt><dd class="text-gray-700">{{ m.scheduled_date || m.maintenance_date }}</dd>
              <dt v-if="m.cost > 0" class="text-gray-400">Biaya</dt><dd v-if="m.cost > 0" class="text-gray-700">{{ formatRupiah(m.cost) }}</dd>
            </dl>
            <div class="flex flex-wrap gap-2 pt-1">
              <button class="act-btn bg-gray-100 text-gray-700 hover:bg-gray-200" @click="openDetail(m)">Buka</button>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" :emptyText="emptyText">
        <template #cell-asset="{ row }">
          <button class="text-left" @click="openDetail(row)">
            <p class="font-medium text-gray-900 hover:text-emerald-700">{{ row.asset_name }}</p>
            <p class="font-mono text-[11px] text-gray-400">{{ row.wo_number }} · {{ row.asset_no }}</p>
          </button>
        </template>
        <template #cell-description="{ row }">
          <p class="max-w-md break-words text-sm text-gray-700">{{ row.description }}</p>
          <p class="text-xs text-gray-400">{{ MTYPES[row.type] || row.type }}<span v-if="row.outlet_name"> · {{ row.outlet_name }}</span></p>
        </template>
        <template #cell-jadwal="{ row }">
          <span class="text-sm text-gray-700">{{ row.scheduled_date || row.maintenance_date }}</span>
          <span v-if="row.due_in_days !== undefined && row.due_in_days !== null" :class="dueCls(row.due_in_days)" class="mt-1 block w-fit">{{ dueLabel(row.due_in_days) }}</span>
        </template>
        <template #cell-status="{ row }"><span :class="woStatusCls(row.status)">{{ woStatusLabel(row.status) }}</span></template>
        <template #cell-cost="{ row }">{{ row.cost > 0 ? formatRupiah(row.cost) : '—' }}</template>
        <template #cell-actions="{ row }">
          <button class="rounded px-2 py-1 text-xs font-medium text-emerald-600 hover:bg-emerald-50" @click="openDetail(row)">Buka</button>
        </template>
      </AppTable>
    </AppCard>

    <!-- Detail WO -->
    <AppModal v-model="detailModal" :title="detail ? `Work Order ${detail.wo_number}` : 'Work Order'" size="xl">
      <div v-if="detail" class="space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <span :class="woStatusCls(detail.status)">{{ woStatusLabel(detail.status) }}</span>
          <span v-if="detail.due_in_days !== undefined && detail.due_in_days !== null" :class="dueCls(detail.due_in_days)">{{ dueLabel(detail.due_in_days) }}</span>
          <span class="text-sm text-gray-700">{{ detail.asset_name }}</span>
        </div>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
          <div v-for="f in detailFields" :key="f.label" class="flex justify-between gap-3 border-b border-gray-50 py-1">
            <dt class="shrink-0 text-gray-400">{{ f.label }}</dt>
            <dd class="min-w-0 break-words text-right font-medium text-gray-800">{{ f.value }}</dd>
          </div>
        </dl>

        <p class="rounded-lg bg-gray-50 p-3 text-sm text-gray-700">{{ detail.description }}</p>

        <!-- Form penutupan -->
        <div v-if="canWrite && ['dijadwalkan','berjalan'].includes(detail.status)" class="space-y-3 rounded-lg border border-gray-200 p-3">
          <p class="text-xs font-bold uppercase tracking-wide text-gray-500">Penutupan Pekerjaan</p>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="lbl">Tanggal Pengerjaan</label>
              <input v-model="cForm.maintenance_date" type="date" class="form-input" />
            </div>
            <div>
              <label class="lbl">Biaya</label>
              <input v-model.number="cForm.cost" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
            </div>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="lbl">Pelaksana / Teknisi</label>
              <input v-model="cForm.performed_by" class="form-input" placeholder="Nama / vendor" />
            </div>
            <div>
              <label class="lbl">Lama Berhenti (jam)</label>
              <input v-model.number="cForm.downtime_hours" type="number" inputmode="decimal" min="0" step="0.5" class="form-input" placeholder="0" />
            </div>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="lbl">Kondisi Setelah</label>
              <select v-model="cForm.condition_after" class="form-input">
                <option value="">— tidak diubah —</option>
                <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
              </select>
            </div>
            <div>
              <label class="lbl">Jadwal Berikutnya</label>
              <input v-model="cForm.next_due_date" type="date" class="form-input" />
              <p class="mt-1 text-[11px] text-gray-400">Kosongkan untuk memakai interval standar kategori.</p>
            </div>
          </div>
          <div>
            <label class="lbl">Catatan Pekerjaan</label>
            <textarea v-model="cForm.description" rows="2" class="form-input" :placeholder="detail.description"></textarea>
          </div>
        </div>
      </div>

      <template #footer>
        <div v-if="detail" class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center">
          <div class="flex flex-wrap gap-2 sm:mr-auto">
            <button v-if="canWrite && !detail.purchase_request_id && ['dijadwalkan','berjalan'].includes(detail.status)"
              class="btn-soft" @click="runAction('pr')">Ajukan Pengadaan Jasa</button>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn-ghost" @click="detailModal = false">Tutup</button>
            <template v-if="canWrite">
              <AppButton v-if="['dijadwalkan','berjalan'].includes(detail.status)" variant="secondary"
                :loading="acting === 'cancel'" @click="runAction('cancel')">Batalkan</AppButton>
              <AppButton v-if="detail.status === 'dijadwalkan'" :loading="acting === 'start'" @click="runAction('start')">Mulai Kerjakan</AppButton>
              <AppButton v-if="['dijadwalkan','berjalan'].includes(detail.status)" :loading="acting === 'complete'" @click="runAction('complete')">Selesai</AppButton>
            </template>
          </div>
        </div>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetMaintenanceApi } from '@/api/assetMaintenance.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, todayDateString } from '@/utils/format.js'
import { CONDITIONS, MTYPES, condLabel, woStatusCls, woStatusLabel, dueCls, dueLabel } from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canWrite = auth.hasPermission('assets.maintenance.create')

const TABS = [
  { key: 'terlambat', label: 'Terlambat' },
  { key: 'minggu', label: 'Jatuh Tempo 7 Hari' },
  { key: 'bulan', label: '30 Hari' },
  { key: 'berjalan', label: 'Sedang Dikerjakan' },
  { key: 'riwayat', label: 'Riwayat' },
]

const COLUMNS = [
  { key: 'asset', label: 'Aset / WO' },
  { key: 'description', label: 'Pekerjaan' },
  { key: 'jadwal', label: 'Jadwal' },
  { key: 'status', label: 'Status' },
  { key: 'cost', label: 'Biaya' },
  { key: 'actions', label: '' },
]

const rows = ref([])
const outlets = ref([])
const summary = ref({ overdue: 0, due_week: 0, due_month: 0, in_progress: 0, cost_year: 0, preventive_year: 0, corrective_year: 0 })
const loading = ref(false)
const errorMsg = ref('')
const tab = ref('terlambat')
const filterOutlet = ref('')
const filterType = ref('')
const search = ref('')

const outletOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])

const emptyText = computed(() => {
  if (tab.value === 'terlambat') return 'Tidak ada perawatan yang terlambat. Semua jadwal terkejar.'
  if (tab.value === 'minggu') return 'Tidak ada perawatan jatuh tempo dalam 7 hari ke depan.'
  if (tab.value === 'bulan') return 'Tidak ada perawatan jatuh tempo dalam 30 hari ke depan.'
  if (tab.value === 'berjalan') return 'Tidak ada pekerjaan yang sedang berjalan.'
  return 'Belum ada riwayat perawatan. Catat lewat halaman detail aset.'
})

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d ?? null }

function paramsForTab() {
  const base = {
    outlet_id: filterOutlet.value || undefined,
    type: filterType.value || undefined,
    search: search.value.trim() || undefined,
  }
  if (tab.value === 'riwayat') return { ...base, status: 'selesai' }
  if (tab.value === 'berjalan') return { ...base, status: 'berjalan' }
  return { ...base, due: tab.value }
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetMaintenanceApi.list(paramsForTab()))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat perawatan'
  } finally { loading.value = false }
}
let _t = null
function debouncedLoad() { clearTimeout(_t); _t = setTimeout(load, 350) }
function setTab(k) { tab.value = k; load() }

async function loadSummary() {
  try { summary.value = asObject(await assetMaintenanceApi.summary()) || summary.value } catch { /* kartu tetap 0 */ }
}
async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

// ── Detail & aksi ──
const detailModal = ref(false)
const detail = ref(null)
const acting = ref('')
const cForm = ref(blankComplete())
function blankComplete() {
  return { maintenance_date: todayDateString(), cost: 0, performed_by: '', downtime_hours: 0, condition_after: '', next_due_date: '', description: '' }
}

const detailFields = computed(() => {
  const d = detail.value
  if (!d) return []
  const out = [
    { label: 'Aset', value: `${d.asset_name} (${d.asset_no || '—'})` },
    { label: 'Outlet', value: d.outlet_name || '—' },
    { label: 'Jenis', value: MTYPES[d.type] || d.type },
    { label: 'Rencana', value: d.scheduled_date || '—' },
  ]
  if (d.status === 'selesai') {
    out.push({ label: 'Dikerjakan', value: d.maintenance_date || '—' })
    out.push({ label: 'Biaya', value: formatRupiah(d.cost) })
    out.push({ label: 'Pelaksana', value: d.performed_by || '—' })
    out.push({ label: 'Lama berhenti', value: d.downtime_hours ? `${d.downtime_hours} jam` : '—' })
    out.push({ label: 'Kondisi setelah', value: d.condition_after ? condLabel(d.condition_after) : '—' })
    out.push({ label: 'Jadwal berikutnya', value: d.next_due_date || '—' })
  }
  if (d.purchase_request_number) out.push({ label: 'Pengadaan Jasa', value: d.purchase_request_number })
  return out
})

async function openDetail(m) {
  detailModal.value = true
  detail.value = null
  cForm.value = blankComplete()
  try {
    detail.value = asObject(await assetMaintenanceApi.get(m.id))
    cForm.value.performed_by = detail.value.performed_by || ''
    cForm.value.cost = detail.value.cost || 0
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat work order')
    detailModal.value = false
  }
}

const CONFIRM = {
  start: 'Mulai kerjakan? Aset akan ditandai sedang diperbaiki dan tidak bisa dimutasi.',
  cancel: 'Batalkan work order ini?',
}

async function runAction(act) {
  if (CONFIRM[act] && !window.confirm(CONFIRM[act])) return
  acting.value = act
  try {
    if (act === 'complete') {
      detail.value = asObject(await assetMaintenanceApi.complete(detail.value.id, cForm.value))
      toast.success('Pekerjaan ditutup')
    } else if (act === 'pr') {
      const pr = asObject(await assetMaintenanceApi.toPurchaseRequest(detail.value.id))
      toast.success(`Pengajuan ${pr?.request_number || 'jasa'} dibuat`)
      detail.value = asObject(await assetMaintenanceApi.get(detail.value.id))
    } else {
      detail.value = asObject(await assetMaintenanceApi[act](detail.value.id))
      toast.success('Work order diperbarui')
    }
    await Promise.all([load(), loadSummary()])
  } catch (e) { toast.error(e?.message || 'Aksi gagal') } finally { acting.value = '' }
}

onMounted(async () => { await loadOutlets(); await Promise.all([load(), loadSummary()]) })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost {
  padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600;
  color: #374151; background: #f3f4f6; min-height: 40px;
}
.btn-ghost:hover { background: #e5e7eb; }
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.08); min-height: 40px;
}
.btn-soft:hover { background: #f3f4f6; }
.act-btn { flex: 1; min-height: 40px; border-radius: .6rem; padding: .4rem .5rem; font-size: .75rem; font-weight: 600; text-align: center; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; overflow-wrap: anywhere; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
</style>
