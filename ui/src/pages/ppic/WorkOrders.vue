<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Work Order Produksi</h1>
        <p class="text-xs text-gray-500 mt-0.5">
          Eksekusi produksi: bahan keluar FIFO, hasil masuk sebagai batch ber-HPP aktual + tanggal kedaluwarsa (shelf life item). Yield tercatat per WO.
        </p>
      </div>
      <AppButton v-if="canCreate" variant="primary" @click="openCreate">+ WO Mandiri</AppButton>
    </div>

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[220px]">
          <SearchSelect v-model="filters.warehouse_id" :options="warehouseFilterOptions" placeholder="Semua Gudang" @change="reload" />
        </div>
        <div class="min-w-[170px]">
          <SearchSelect v-model="filters.status" :options="STATUS_OPTIONS" placeholder="Semua Status" @change="reload" />
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="wo-table">
          <thead>
            <tr><th>Nomor</th><th>Item</th><th>Gudang</th><th class="th-r">Rencana</th><th class="th-r">Aktual</th><th class="th-r">Yield</th><th class="th-r">HPP/Unit</th><th>Status</th><th>Waktu</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td colspan="10" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
            <tr v-else-if="!rows.length"><td colspan="10" class="p-8 text-center text-sm text-gray-400">Belum ada work order.</td></tr>
            <tr v-for="r in rows" :key="r.id">
              <td>
                <div class="font-mono text-xs font-bold text-gray-700">{{ r.wo_number }}</div>
                <div v-if="r.plan_number" class="text-[10px] text-gray-400">{{ r.plan_number }}</div>
              </td>
              <td class="text-sm font-medium text-gray-900">{{ r.item_name }}</td>
              <td class="text-xs text-gray-600">{{ r.warehouse_name }}</td>
              <td class="td-num">{{ fmtQty(r.qty_planned_base) }} {{ r.base_unit }}</td>
              <td class="td-num">{{ r.qty_actual_base == null ? '—' : fmtQty(r.qty_actual_base) }}</td>
              <td class="td-num" :class="yieldClass(r)">{{ r.status === 'done' ? r.yield_pct.toFixed(1) + '%' : '—' }}</td>
              <td class="td-num">{{ r.status === 'done' ? fmtRp(r.cost_per_unit) : '—' }}</td>
              <td><span class="pill" :class="statusPill(r.status)">{{ statusLabel(r.status) }}</span></td>
              <td class="text-[11px] text-gray-500">
                {{ fmtDateTime(r.created_at) }}
                <div v-if="r.expiry_date" class="text-[10px] text-amber-600">exp {{ fmtDate(r.expiry_date) }}</div>
              </td>
              <td class="text-right pr-3 whitespace-nowrap">
                <AppActionButton icon="play" label="Mulai" v-if="canExecute && r.status === 'planned'" @click="doStart(r)" />
                <AppActionButton icon="approve" label="Selesaikan" v-if="canExecute && (r.status === 'planned' || r.status === 'in_progress')" @click="openFinish(r)" />
                <AppActionButton icon="cancel" label="Batal" v-if="canCreate && (r.status === 'planned' || r.status === 'in_progress')" @click="doCancel(r)" />
                <AppActionButton icon="view" label="Detail" v-if="r.status === 'done'" @click="openFinished(r)" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
        <span class="text-xs text-gray-400">{{ total }} work order</span>
        <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
      </div>
    </AppCard>

    <!-- Create modal -->
    <AppModal v-model="showCreate" title="Work Order Mandiri" size="lg">
      <div class="modal-form-grid">
        <div>
          <label class="text-xs font-semibold text-gray-600">Gudang</label>
          <SearchSelect v-model="createForm.warehouse_id" :options="warehouseOptions" placeholder="Pilih Gudang" @change="loadProducible" />
        </div>
        <div>
          <label class="text-xs font-semibold text-gray-600">Item Setengah Jadi</label>
          <SearchSelect v-model="createForm.item_id" :options="producibleOptions" placeholder="Pilih Item (ber-resep internal)" />
        </div>
        <div>
          <label class="text-xs font-semibold text-gray-600">Qty Rencana (satuan dasar)</label>
          <input type="number" min="0" step="0.5" v-model.number="createForm.qty_planned_base"
            class="block w-full text-sm font-mono border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
        </div>
        <textarea v-model="createForm.notes" rows="2" placeholder="Catatan..."
          class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400"></textarea>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton variant="primary" :loading="creating" @click="doCreate">Buat WO</AppButton>
      </template>
    </AppModal>

    <!-- Finish modal -->
    <AppModal v-model="showFinish" :title="finishTarget ? `Selesaikan ${finishTarget.wo_number}` : ''" size="xl">
      <div v-if="finishTarget" class="space-y-3">
        <div class="flex flex-wrap items-end gap-4">
          <div>
            <label class="text-xs font-semibold text-gray-600">Qty Hasil Aktual ({{ finishTarget.base_unit }})</label>
            <input type="number" min="0" step="0.5" v-model.number="finishForm.qty_actual_base" @input="prefillMaterials"
              class="block w-40 text-sm font-mono border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
          </div>
          <div class="text-xs text-gray-500 pb-2">
            Rencana: <b>{{ fmtQty(finishTarget.qty_planned_base) }} {{ finishTarget.base_unit }}</b>
            · Yield: <b :class="previewYield >= 90 ? 'text-emerald-700' : 'text-amber-600'">{{ previewYield.toFixed(1) }}%</b>
            <template v-if="finishTarget.shelf_life_days > 0"> · Expiry hasil: <b>+{{ finishTarget.shelf_life_days }} hari</b></template>
            <template v-else> · <span class="text-amber-600">shelf life item belum diisi — batch tanpa expiry</span></template>
          </div>
        </div>

        <div class="overflow-x-auto">
          <label class="text-xs font-semibold text-gray-600">Bahan Terpakai (prefill = resep × qty aktual, koreksi bila beda)</label>
          <table class="wo-table mt-1">
            <thead><tr><th>Bahan</th><th class="th-r">Resep (plan)</th><th class="th-r">Stok</th><th class="th-r">Aktual Terpakai</th></tr></thead>
            <tbody>
              <tr v-for="m in finishForm.materials" :key="m.item_id">
                <td class="text-sm text-gray-800">{{ m.item_name }}</td>
                <td class="td-num text-gray-500">{{ fmtQty(m.qty_plan_base) }} {{ m.base_unit }}</td>
                <td class="td-num" :class="m.on_hand < m.qty_actual_base ? 'text-red-600 font-bold' : ''">{{ fmtQty(m.on_hand) }}</td>
                <td class="td-num">
                  <input type="number" min="0" step="0.01" v-model.number="m.qty_actual_base"
                    class="w-28 text-right text-sm font-mono border border-gray-200 rounded-lg px-2 py-1 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
                </td>
              </tr>
            </tbody>
          </table>
          <p v-if="insufficient" class="text-[11px] text-red-600 mt-1"><svg class="inline-block align-[-2px] mr-0.5" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg> Ada bahan yang stoknya kurang — posting akan gagal bila tidak dikoreksi.</p>
        </div>

        <textarea v-model="finishForm.notes" rows="2" placeholder="Catatan produksi (opsional)..."
          class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400"></textarea>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showFinish = false">Batal</AppButton>
        <AppButton v-if="finishTarget" variant="primary" :loading="finishing" @click="doFinish">Posting Produksi</AppButton>
      </template>
    </AppModal>

    <!-- Detail selesai -->
    <AppModal v-model="showDetail" :title="detailWo ? detailWo.wo_number + ' — Detail Produksi' : ''" size="xl">
      <div v-if="detailWo" class="space-y-3">
        <div class="det-grid">
          <div class="det"><div class="det-l">Hasil</div><div class="det-v">{{ fmtQty(detailWo.qty_actual_base) }} / {{ fmtQty(detailWo.qty_planned_base) }} {{ detailWo.base_unit }}</div></div>
          <div class="det"><div class="det-l">Yield</div><div class="det-v" :class="detailWo.yield_pct >= 90 ? 'text-emerald-700' : 'text-amber-600'">{{ detailWo.yield_pct.toFixed(1) }}%</div></div>
          <div class="det"><div class="det-l">HPP</div><div class="det-v">{{ fmtRp(detailWo.cost_per_unit) }}/{{ detailWo.base_unit }} · total {{ fmtRp(detailWo.cost_total) }}</div></div>
          <div class="det"><div class="det-l">Expiry Batch</div><div class="det-v">{{ detailWo.expiry_date ? fmtDate(detailWo.expiry_date) : '—' }}</div></div>
        </div>
        <div class="overflow-x-auto">
        <table class="wo-table">
          <thead><tr><th>Bahan</th><th class="th-r">Plan</th><th class="th-r">Aktual</th><th class="th-r">Variance</th><th class="th-r">Biaya</th></tr></thead>
          <tbody>
            <tr v-for="m in detailWo.materials" :key="m.id">
              <td class="text-sm text-gray-800">{{ m.item_name }}</td>
              <td class="td-num text-gray-500">{{ fmtQty(m.qty_plan_base) }} {{ m.base_unit }}</td>
              <td class="td-num">{{ m.qty_actual_base == null ? '—' : fmtQty(m.qty_actual_base) }}</td>
              <td class="td-num" :class="m.variance_pct > 5 ? 'text-red-600' : (m.variance_pct < -5 ? 'text-blue-600' : 'text-gray-500')">
                {{ m.qty_actual_base == null ? '—' : (m.variance_pct > 0 ? '+' : '') + m.variance_pct.toFixed(1) + '%' }}
              </td>
              <td class="td-num">{{ fmtRp(m.cost_per_base * (m.qty_actual_base ?? 0)) }}</td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
import { warehousesApi } from '@/api/warehouse'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canCreate = computed(() => auth.hasPermission('ppic.workorders.create'))
const canExecute = computed(() => auth.hasPermission('ppic.workorders.execute'))

const STATUS_OPTIONS = [
  { id: '', name: 'Semua Status' },
  { id: 'planned', name: 'Rencana' }, { id: 'in_progress', name: 'Proses' },
  { id: 'done', name: 'Selesai' }, { id: 'cancelled', name: 'Dibatalkan' },
]

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)
const filters = reactive({ warehouse_id: '', status: '' })
const warehouseOptions = ref([])
const warehouseFilterOptions = ref([{ id: '', name: 'Semua Gudang' }])

const showCreate = ref(false)
const creating = ref(false)
const createForm = reactive({ warehouse_id: '', item_id: '', qty_planned_base: 0, notes: '' })
const producibleOptions = ref([])

const showFinish = ref(false)
const finishing = ref(false)
const finishTarget = ref(null)
const finishForm = reactive({ qty_actual_base: 0, materials: [], notes: '' })

const showDetail = ref(false)
const detailWo = ref(null)

onMounted(async () => {
  load()
  try {
    const whRes = await warehousesApi.list({ limit: 100 })
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})` }))
    warehouseFilterOptions.value = [{ id: '', name: 'Semua Gudang' }, ...warehouseOptions.value]
  } catch { /* dropdown kosong */ }
})

async function load() {
  loading.value = true
  try {
    const res = await ppicApi.listWorkOrders({ ...filters, page: page.value, limit: 20 })
    rows.value = res.data || []
    total.value = res.total || 0
    totalPages.value = res.total_pages || 1
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat work order')
  } finally {
    loading.value = false
  }
}
function reload() { page.value = 1; load() }
function changePage(p) { page.value = p; load() }

function openCreate() {
  Object.assign(createForm, { warehouse_id: filters.warehouse_id || warehouseOptions.value[0]?.id || '', item_id: '', qty_planned_base: 0, notes: '' })
  producibleOptions.value = []
  showCreate.value = true
  if (createForm.warehouse_id) loadProducible()
}
async function loadProducible() {
  createForm.item_id = ''
  try {
    const list = await ppicApi.listProducibleItems(createForm.warehouse_id)
    producibleOptions.value = (list || []).map(p => ({ id: p.item_id, name: `${p.item_name} (stok ${fmtQty(p.qty_base)} ${p.base_unit})` }))
  } catch { producibleOptions.value = [] }
}
async function doCreate() {
  if (!createForm.item_id || createForm.qty_planned_base <= 0) { toast.error('Pilih item dan isi qty rencana'); return }
  creating.value = true
  try {
    const wo = await ppicApi.createWorkOrder({ ...createForm })
    toast.success(`${wo.wo_number} dibuat`)
    showCreate.value = false
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuat WO')
  } finally {
    creating.value = false
  }
}

async function doStart(r) {
  try {
    await ppicApi.startWorkOrder(r.id)
    toast.success(`${r.wo_number} dimulai`)
    load()
  } catch (e) { toast.error(e?.message ?? 'Gagal memulai WO') }
}

async function openFinish(r) {
  try {
    const wo = await ppicApi.getWorkOrder(r.id)
    finishTarget.value = wo
    finishForm.qty_actual_base = wo.qty_planned_base
    finishForm.notes = ''
    finishForm.materials = (wo.materials || []).map(m => ({ ...m, qty_actual_base: m.qty_plan_base }))
    showFinish.value = true
  } catch (e) { toast.error(e?.message ?? 'Gagal membuka WO') }
}

function prefillMaterials() {
  const t = finishTarget.value
  if (!t || t.qty_planned_base <= 0) return
  const ratio = (finishForm.qty_actual_base || 0) / t.qty_planned_base
  for (const m of finishForm.materials) {
    m.qty_actual_base = Math.round(m.qty_plan_base * ratio * 100) / 100
  }
}

const previewYield = computed(() => {
  const t = finishTarget.value
  if (!t || t.qty_planned_base <= 0) return 0
  return (finishForm.qty_actual_base || 0) / t.qty_planned_base * 100
})
const insufficient = computed(() => finishForm.materials.some(m => m.qty_actual_base > m.on_hand))

async function doFinish() {
  if (finishForm.qty_actual_base <= 0) { toast.error('Qty hasil aktual harus > 0'); return }
  if (!confirm(`Posting produksi ${finishTarget.value.wo_number}? Bahan dipotong FIFO dan hasil masuk sebagai batch baru.`)) return
  finishing.value = true
  try {
    const wo = await ppicApi.finishWorkOrder(finishTarget.value.id, {
      qty_actual_base: finishForm.qty_actual_base,
      materials: finishForm.materials.map(m => ({ item_id: m.item_id, qty_actual_base: Number(m.qty_actual_base) || 0 })),
      notes: finishForm.notes,
    })
    toast.success(`${wo.wo_number} selesai — yield ${wo.yield_pct.toFixed(1)}%, HPP ${fmtRp(wo.cost_per_unit)}/${wo.base_unit}`)
    showFinish.value = false
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal posting produksi')
  } finally {
    finishing.value = false
  }
}

async function doCancel(r) {
  if (!confirm(`Batalkan ${r.wo_number}?`)) return
  try {
    await ppicApi.cancelWorkOrder(r.id)
    toast.success('WO dibatalkan')
    load()
  } catch (e) { toast.error(e?.message ?? 'Gagal membatalkan WO') }
}

async function openFinished(r) {
  try {
    detailWo.value = await ppicApi.getWorkOrder(r.id)
    showDetail.value = true
  } catch (e) { toast.error(e?.message ?? 'Gagal membuka detail') }
}

function yieldClass(r) {
  if (r.status !== 'done') return 'text-gray-300'
  return r.yield_pct >= 90 ? 'text-emerald-700 font-bold' : 'text-amber-600 font-bold'
}
const STATUS_LABELS = { planned: 'Rencana', in_progress: 'Proses', done: 'Selesai', cancelled: 'Dibatalkan' }
function statusLabel(s) { return STATUS_LABELS[s] ?? s }
function statusPill(s) {
  return { planned: 'pill-gray', in_progress: 'pill-amber', done: 'pill-green', cancelled: 'pill-red' }[s] ?? 'pill-gray'
}

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtRp(v) { return 'Rp ' + Math.round(Number(v ?? 0)).toLocaleString('id-ID') }
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }
function fmtDateTime(s) { return s ? new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '—' }
</script>

<style scoped>
.wo-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.wo-table thead tr { background: #f9fafb; }
.wo-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.wo-table .th-r { text-align: right; }
.wo-table tbody tr { border-bottom: 1px solid #f9fafb; }
.wo-table tbody tr:hover { background: #fafafa; }
.wo-table td { padding: .5rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; }

.pill { display: inline-block; font-size: .62rem; font-weight: 700; padding: .15rem .5rem; border-radius: 999px; white-space: nowrap; }
.pill-gray { background: #f3f4f6; color: #6b7280; }
.pill-amber { background: #fef3c7; color: #d97706; }
.pill-green { background: #dcfce7; color: #15803d; }
.pill-red { background: #fee2e2; color: #dc2626; }

.act { display: inline-block; font-size: .66rem; font-weight: 700; padding: .26rem .55rem; border-radius: .45rem; margin-left: .25rem; cursor: pointer; border: 1px solid; background: #fff; }
.act-blue  { color: #1d4ed8; border-color: #bfdbfe; background: #eff6ff; }
.act-green { color: #047857; border-color: #a7f3d0; background: #ecfdf5; }
.act-red   { color: #dc2626; border-color: #fecaca; background: #fef2f2; }
.act-gray  { color: #374151; border-color: #e5e7eb; background: #f9fafb; }

.det-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; }
@media (max-width: 700px) { .det-grid { grid-template-columns: repeat(2, 1fr); } }
.det { background: #f9fafb; border: 1px solid #f3f4f6; border-radius: .6rem; padding: .5rem .7rem; }
.det-l { font-size: .6rem; font-weight: 700; text-transform: uppercase; color: #9ca3af; }
.det-v { font-size: .8rem; font-weight: 700; color: #111827; }
</style>
