<template>
  <div class="space-y-5">

    <!-- ══ DETAIL ══ -->
    <template v-if="detail">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button class="text-xs text-emerald-700 font-semibold hover:underline" @click="detail = null; load()">← Kembali ke daftar</button>
          <h1 class="text-xl font-bold text-gray-900 mt-1">
            {{ detail.plan_number }}
            <span class="pill ml-2" :class="statusPill(detail.status)">{{ statusLabel(detail.status) }}</span>
          </h1>
          <p class="text-xs text-gray-500 mt-0.5">
            {{ detail.warehouse_name }} · rencana tanggal {{ fmtDate(detail.plan_date) }} · dibuat {{ detail.created_by || '—' }}
            <template v-if="detail.approved_by"> · disetujui {{ detail.approved_by }}</template>
          </p>
        </div>
        <div class="flex gap-2 flex-wrap">
          <template v-if="detail.status === 'draft'">
            <AppActionButton icon="cancel" label="Batalkan" v-if="canCreate" :loading="acting" @click="doCancel" />
            <AppActionButton icon="approve" label="Approve" v-if="canApprove" :loading="acting" @click="doApprove" />
          </template>
          <template v-else-if="detail.status === 'approved'">
            <AppActionButton icon="cancel" label="Batalkan" v-if="canCreate" :loading="acting" @click="doCancel" />
            <AppActionButton icon="send" label="Release → Buat Work Order" v-if="canApprove" :loading="acting" @click="doRelease" />
          </template>
          <router-link v-else-if="detail.status === 'released' || detail.status === 'closed'" to="/ppic/work-orders" class="text-xs font-semibold text-emerald-700 hover:underline self-center">
            Lihat Work Order →
          </router-link>
        </div>
      </div>

      <AppCard :padding="false">
        <table class="pl-table">
          <thead>
            <tr><th>Item Setengah Jadi</th><th class="th-r">Qty Rencana</th><th class="th-r">Stok Saat Ini</th><th class="th-r">Par Level</th><th>Catatan</th><th>Work Order</th></tr>
          </thead>
          <tbody>
            <tr v-for="it in detail.items" :key="it.id">
              <td>
                <div class="font-medium text-gray-900 text-sm">{{ it.item_name }}</div>
                <div class="text-[11px] text-gray-400 font-mono">{{ it.item_code }}</div>
              </td>
              <td class="td-num font-bold">{{ fmtQty(it.qty_planned_base) }} {{ it.base_unit }}</td>
              <td class="td-num">{{ fmtQty(it.on_hand) }}</td>
              <td class="td-num text-gray-500">{{ it.par_level ? fmtQty(it.par_level) : '—' }}</td>
              <td class="text-xs text-gray-500">{{ it.notes || '—' }}</td>
              <td>
                <span v-if="it.wo_number" class="pill" :class="woPill(it.wo_status)">{{ it.wo_number }} · {{ woLabel(it.wo_status) }}</span>
                <span v-else class="text-gray-300 text-xs">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </AppCard>
    </template>

    <!-- ══ LIST ══ -->
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-bold text-gray-900">Rencana Produksi (MPS)</h1>
          <p class="text-xs text-gray-500 mt-0.5">Rencana harian produksi barang setengah jadi: draft → approve → release menjadi Work Order.</p>
        </div>
        <AppButton v-if="canCreate" variant="primary" @click="openCreate">+ Rencana Baru</AppButton>
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
          <table class="pl-table">
            <thead><tr><th>Nomor</th><th>Gudang</th><th>Tanggal</th><th>Status</th><th class="th-r">Item</th><th class="th-r">WO Selesai</th><th>Dibuat</th><th></th></tr></thead>
            <tbody>
              <tr v-if="loading"><td colspan="8" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
              <tr v-else-if="!rows.length"><td colspan="8" class="p-8 text-center text-sm text-gray-400">Belum ada rencana produksi.</td></tr>
              <tr v-for="r in rows" :key="r.id" class="cursor-pointer" @click="openDetail(r.id)">
                <td class="font-mono text-xs font-bold text-gray-700">{{ r.plan_number }}</td>
                <td class="text-sm text-gray-800">{{ r.warehouse_name }}</td>
                <td class="text-xs text-gray-600">{{ fmtDate(r.plan_date) }}</td>
                <td><span class="pill" :class="statusPill(r.status)">{{ statusLabel(r.status) }}</span></td>
                <td class="td-num">{{ r.item_count }}</td>
                <td class="td-num">{{ r.wo_done }}/{{ r.wo_total }}</td>
                <td class="text-xs text-gray-500">{{ r.created_by }}</td>
                <td class="text-right pr-3 text-emerald-700 text-xs font-semibold">Buka →</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
          <span class="text-xs text-gray-400">{{ total }} rencana</span>
          <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
        </div>
      </AppCard>

      <!-- Create modal -->
      <AppModal v-model="showCreate" title="Rencana Produksi Baru" size="xl">
        <div class="space-y-3">
          <div class="modal-form-grid">
            <div class="min-w-0">
              <label class="text-xs font-semibold text-gray-600">Gudang Produksi</label>
              <SearchSelect v-model="createForm.warehouse_id" :options="warehouseOptions" placeholder="Pilih Gudang" @change="loadProducible" />
            </div>
            <div>
              <label class="text-xs font-semibold text-gray-600">Tanggal Rencana</label>
              <input type="date" v-model="createForm.plan_date"
                class="block w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
            </div>
          </div>

          <div v-if="createForm.warehouse_id" class="overflow-x-auto">
            <label class="text-xs font-semibold text-gray-600">Item Setengah Jadi (ber-resep internal)</label>
            <div v-if="!producible.length" class="text-xs text-gray-400 p-3 bg-gray-50 rounded-lg mt-1">
              Tidak ada item ber-resep internal. Definisikan resep dulu di halaman Item Stok.
            </div>
            <table v-else class="pl-table mt-1">
              <thead><tr><th></th><th>Item</th><th class="th-r">Stok</th><th class="th-r">Par</th><th class="th-r">Qty Rencana</th></tr></thead>
              <tbody>
                <tr v-for="p in producible" :key="p.item_id">
                  <td class="w-8 text-center"><input type="checkbox" v-model="p._checked" /></td>
                  <td>
                    <div class="text-sm font-medium text-gray-800">{{ p.item_name }}</div>
                    <div class="text-[11px] text-gray-400 font-mono">{{ p.item_code }}</div>
                  </td>
                  <td class="td-num">{{ fmtQty(p.qty_base) }} {{ p.base_unit }}</td>
                  <td class="td-num text-gray-500">{{ p.par_level ? fmtQty(p.par_level) : '—' }}</td>
                  <td class="td-num">
                    <input type="number" min="0" step="0.5" v-model.number="p._qty" :disabled="!p._checked"
                      class="w-24 text-right text-sm font-mono border border-gray-200 rounded-lg px-2 py-1 focus:outline-none focus:ring-2 focus:ring-emerald-400 disabled:bg-gray-50" />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <textarea v-model="createForm.notes" rows="2" placeholder="Catatan..."
            class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400"></textarea>
        </div>
        <template #footer>
          <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
          <AppButton variant="primary" :loading="creating" @click="doCreate">Simpan Draft</AppButton>
        </template>
      </AppModal>
    </template>
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
const canCreate = computed(() => auth.hasPermission('ppic.production.create'))
const canApprove = computed(() => auth.hasPermission('ppic.production.approve'))

const STATUS_OPTIONS = [
  { id: '', name: 'Semua Status' },
  { id: 'draft', name: 'Draft' }, { id: 'approved', name: 'Disetujui' },
  { id: 'released', name: 'Release' }, { id: 'closed', name: 'Selesai' }, { id: 'cancelled', name: 'Dibatalkan' },
]

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)
const filters = reactive({ warehouse_id: '', status: '' })
const warehouseOptions = ref([])
const warehouseFilterOptions = ref([{ id: '', name: 'Semua Gudang' }])

const detail = ref(null)
const acting = ref(false)

const showCreate = ref(false)
const creating = ref(false)
const createForm = reactive({ warehouse_id: '', plan_date: new Date().toISOString().slice(0, 10), notes: '' })
const producible = ref([])

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
    const res = await ppicApi.listProductionPlans({ ...filters, page: page.value, limit: 20 })
    rows.value = res.data || []
    total.value = res.total || 0
    totalPages.value = res.total_pages || 1
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat rencana produksi')
  } finally {
    loading.value = false
  }
}
function reload() { page.value = 1; load() }
function changePage(p) { page.value = p; load() }

async function openDetail(id) {
  try { detail.value = await ppicApi.getProductionPlan(id) }
  catch (e) { toast.error(e?.message ?? 'Gagal membuka rencana') }
}

function openCreate() {
  createForm.warehouse_id = filters.warehouse_id || warehouseOptions.value[0]?.id || ''
  createForm.plan_date = new Date().toISOString().slice(0, 10)
  createForm.notes = ''
  producible.value = []
  showCreate.value = true
  if (createForm.warehouse_id) loadProducible()
}

async function loadProducible() {
  if (!createForm.warehouse_id) return
  try {
    const list = await ppicApi.listProducibleItems(createForm.warehouse_id)
    producible.value = (list || []).map(p => ({ ...p, _checked: false, _qty: 0 }))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat item produksi')
  }
}

async function doCreate() {
  const items = producible.value.filter(p => p._checked && p._qty > 0)
    .map(p => ({ item_id: p.item_id, qty_planned_base: p._qty, notes: '' }))
  if (!items.length) { toast.error('Centang minimal satu item dan isi qty rencana'); return }
  creating.value = true
  try {
    const plan = await ppicApi.createProductionPlan({ ...createForm, items })
    toast.success(`Rencana ${plan.plan_number} tersimpan (draft)`)
    showCreate.value = false
    detail.value = plan
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menyimpan rencana')
  } finally {
    creating.value = false
  }
}

async function doApprove() {
  acting.value = true
  try {
    await ppicApi.approveProductionPlan(detail.value.id)
    toast.success('Rencana disetujui')
    detail.value = await ppicApi.getProductionPlan(detail.value.id)
  } catch (e) { toast.error(e?.message ?? 'Gagal approve') } finally { acting.value = false }
}

async function doRelease() {
  if (!confirm('Release rencana ini? Setiap baris item menjadi Work Order siap dieksekusi.')) return
  acting.value = true
  try {
    detail.value = await ppicApi.releaseProductionPlan(detail.value.id)
    toast.success('Work order dibuat — lihat halaman Work Order')
  } catch (e) { toast.error(e?.message ?? 'Gagal release') } finally { acting.value = false }
}

async function doCancel() {
  if (!confirm('Batalkan rencana ini?')) return
  acting.value = true
  try {
    await ppicApi.cancelProductionPlan(detail.value.id)
    toast.success('Rencana dibatalkan')
    detail.value = null
    load()
  } catch (e) { toast.error(e?.message ?? 'Gagal membatalkan') } finally { acting.value = false }
}

const STATUS_LABELS = { draft: 'Draft', approved: 'Disetujui', released: 'Release', closed: 'Selesai', cancelled: 'Dibatalkan' }
function statusLabel(s) { return STATUS_LABELS[s] ?? s }
function statusPill(s) {
  return { draft: 'pill-gray', approved: 'pill-blue', released: 'pill-amber', closed: 'pill-green', cancelled: 'pill-red' }[s] ?? 'pill-gray'
}
const WO_LABELS = { planned: 'Rencana', in_progress: 'Proses', done: 'Selesai', cancelled: 'Batal' }
function woLabel(s) { return WO_LABELS[s] ?? s }
function woPill(s) { return { planned: 'pill-gray', in_progress: 'pill-amber', done: 'pill-green', cancelled: 'pill-red' }[s] ?? 'pill-gray' }

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }
</script>

<style scoped>
.pl-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.pl-table thead tr { background: #f9fafb; }
.pl-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.pl-table .th-r { text-align: right; }
.pl-table tbody tr { border-bottom: 1px solid #f9fafb; }
.pl-table tbody tr:hover { background: #fafafa; }
.pl-table td { padding: .5rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; }

.pill { display: inline-block; font-size: .62rem; font-weight: 700; padding: .15rem .5rem; border-radius: 999px; white-space: nowrap; }
.pill-gray { background: #f3f4f6; color: #6b7280; }
.pill-blue { background: #dbeafe; color: #1d4ed8; }
.pill-amber { background: #fef3c7; color: #d97706; }
.pill-green { background: #dcfce7; color: #15803d; }
.pill-red { background: #fee2e2; color: #dc2626; }
</style>
