<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Monitor Kedaluwarsa (FEFO)</h1>
      <p class="text-xs text-gray-500 mt-0.5">Batch stok dengan tanggal kedaluwarsa — keluarkan yang paling dekat kedaluwarsa lebih dulu (First-Expired-First-Out).</p>
    </div>

    <!-- Bucket cards -->
    <div class="bucket-grid">
      <button v-for="b in BUCKETS" :key="b.key" class="bucket" :class="[`bucket--${b.cls}`, { 'bucket--active': filters.bucket === b.key }]"
        @click="toggleBucket(b.key)">
        <div class="bucket-label">{{ b.label }}</div>
        <div class="bucket-count">{{ bucketData(b.key).count }}</div>
        <div class="bucket-val">{{ fmtRp(bucketData(b.key).value) }}</div>
      </button>
      <div class="bucket bucket--info">
        <div class="bucket-label">Tanpa Tanggal</div>
        <div class="bucket-count">{{ resp?.no_expiry_count ?? 0 }}</div>
        <div class="bucket-val">batch belum diisi expiry</div>
      </div>
    </div>

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[220px]">
          <SearchSelect v-model="filters.warehouse_id" :options="warehouseOptions" placeholder="Semua Gudang" @change="reload" />
        </div>
        <input v-model="filters.search" @input="debouncedReload" placeholder="Cari item..."
          class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
      </div>

      <div class="overflow-x-auto">
        <table class="ex-table">
          <thead>
            <tr>
              <th>Item</th><th>Gudang</th><th class="th-r">Qty Sisa</th><th class="th-r">Nilai</th>
              <th>Kedaluwarsa</th><th>Asal</th><th>Penanganan</th><th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td colspan="8" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
            <tr v-else-if="!rows.length"><td colspan="8" class="p-8 text-center text-sm text-gray-400">Tidak ada batch ber-tanggal kedaluwarsa.</td></tr>
            <tr v-for="r in rows" :key="r.batch_id" :class="{ 'row-acked': r.ack_at }">
              <td>
                <div class="font-medium text-gray-900 text-sm">{{ r.item_name }}</div>
                <div class="text-[11px] text-gray-400 font-mono">{{ r.item_code }} · {{ r.category || '—' }}</div>
              </td>
              <td class="text-xs text-gray-600">{{ r.warehouse_name }}</td>
              <td class="td-num">{{ fmtQty(r.qty_base) }} {{ r.base_unit }}</td>
              <td class="td-num">{{ fmtRp(r.value) }}</td>
              <td>
                <div class="text-xs font-medium" :class="expiryTextClass(r)">{{ fmtDate(r.expiry_date) }}</div>
                <span class="pill" :class="bucketPill(r.bucket)">{{ daysLeftLabel(r) }}</span>
              </td>
              <td class="text-[11px] text-gray-400">{{ refLabel(r.ref_type) }}</td>
              <td>
                <div v-if="r.ack_at" class="text-[11px] text-emerald-700">
                  <svg class="inline-block align-[-2px]" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg> Ditindak oleh {{ r.ack_by }}<br/>
                  <span class="text-gray-400">{{ r.ack_note || '—' }}</span>
                </div>
                <span v-else class="text-[11px] text-gray-300">belum</span>
              </td>
              <td class="text-right whitespace-nowrap">
                <router-link v-if="r.bucket === 'expired'" class="act act-red" to="/stock-wastes">Buang</router-link>
                <AppActionButton icon="approve" label="Tandai Ditindak" v-if="canAck && !r.ack_at" @click="openAck(r)" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
        <span class="text-xs text-gray-400">{{ resp?.total ?? 0 }} batch</span>
        <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
      </div>
    </AppCard>

    <!-- Ack modal -->
    <AppModal v-model="showAck" title="Tandai Batch Sudah Ditindak" size="md">
      <div v-if="ackTarget" class="space-y-3">
        <p class="text-sm text-gray-600">
          <b>{{ ackTarget.item_name }}</b> — {{ fmtQty(ackTarget.qty_base) }} {{ ackTarget.base_unit }} di {{ ackTarget.warehouse_name }},
          kedaluwarsa {{ fmtDate(ackTarget.expiry_date) }}. Alert dashboard untuk batch ini akan disenyapkan.
        </p>
        <textarea v-model="ackNote" rows="3" placeholder="Catatan penanganan (mis. sudah dibuang / dipakai duluan / ditransfer)..."
          class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400"></textarea>
        <div class="flex justify-end gap-2">
          <AppButton variant="secondary" @click="showAck = false">Batal</AppButton>
          <AppButton variant="primary" :loading="acking" @click="doAck">Simpan</AppButton>
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
import debounce from 'lodash/debounce'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canAck = computed(() => auth.hasPermission('ppic.expiry.ack'))

const BUCKETS = [
  { key: 'expired', label: 'Sudah Lewat', cls: 'red' },
  { key: 'd3',      label: '≤ 3 Hari',    cls: 'red' },
  { key: 'd7',      label: '≤ 7 Hari',    cls: 'amber' },
  { key: 'd30',     label: '≤ 30 Hari',   cls: 'yellow' },
  { key: 'safe',    label: 'Aman',        cls: 'green' },
]

const loading = ref(false)
const resp = ref(null)
const rows = ref([])
const page = ref(1)
const totalPages = ref(1)
const filters = reactive({ warehouse_id: '', bucket: '', search: '' })
const warehouseOptions = ref([{ id: '', name: 'Semua Gudang' }])

const showAck = ref(false)
const ackTarget = ref(null)
const ackNote = ref('')
const acking = ref(false)

onMounted(async () => {
  load()
  try {
    const whRes = await warehousesApi.list({ limit: 100 })
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = [{ id: '', name: 'Semua Gudang' }, ...whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})` }))]
  } catch { /* selector tetap kosong, halaman tetap jalan */ }
})

async function load() {
  loading.value = true
  try {
    resp.value = await ppicApi.getExpiry({ ...filters, page: page.value, limit: 25 })
    rows.value = resp.value?.data || []
    totalPages.value = Math.max(1, Math.ceil((resp.value?.total || 0) / 25))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat monitor kedaluwarsa')
  } finally {
    loading.value = false
  }
}

function bucketData(key) {
  const s = resp.value?.summary?.find(x => x.bucket === key)
  return { count: s?.count ?? 0, value: s?.value ?? 0 }
}
function toggleBucket(key) {
  filters.bucket = filters.bucket === key ? '' : key
  reload()
}

function openAck(r) { ackTarget.value = r; ackNote.value = ''; showAck.value = true }
async function doAck() {
  acking.value = true
  try {
    await ppicApi.ackExpiryBatch(ackTarget.value.batch_id, ackNote.value)
    toast.success('Batch ditandai sudah ditindak')
    showAck.value = false
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menandai batch')
  } finally {
    acking.value = false
  }
}

function reload() { page.value = 1; load() }
const debouncedReload = debounce(reload, 500)
function changePage(p) { page.value = p; load() }

function daysLeftLabel(r) {
  if (r.days_left < 0) return `lewat ${-r.days_left} hari`
  if (r.days_left === 0) return 'hari ini'
  return `${r.days_left} hari lagi`
}
function expiryTextClass(r) {
  if (r.bucket === 'expired') return 'text-red-600'
  if (r.bucket === 'd3') return 'text-red-500'
  if (r.bucket === 'd7') return 'text-amber-600'
  return 'text-gray-700'
}
function bucketPill(b) {
  return { expired: 'pill-red', d3: 'pill-red', d7: 'pill-amber', d30: 'pill-yellow', safe: 'pill-green' }[b] ?? 'pill-gray'
}
const REF_LABELS = { goods_receipt: 'Penerimaan', purchase_in: 'Pembelian', production_in: 'Produksi', transfer_in: 'Transfer', manual: 'Manual', adjustment: 'Penyesuaian', stock_breakdown: 'Pemotongan' }
function refLabel(t) { return REF_LABELS[t] ?? (t || '—') }

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtRp(v) {
  if (!v) return 'Rp 0'
  if (v >= 1_000_000) return 'Rp ' + (v / 1_000_000).toFixed(1) + ' jt'
  return 'Rp ' + Math.round(v).toLocaleString('id-ID')
}
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }
</script>

<style scoped>
.bucket-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: .6rem; }
@media (max-width: 1000px) { .bucket-grid { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 560px)  { .bucket-grid { grid-template-columns: repeat(2, 1fr); } }

.bucket { text-align: left; padding: .7rem .85rem; border-radius: .75rem; border: 1.5px solid; cursor: pointer; transition: transform .12s, box-shadow .12s; background: #fff; }
.bucket:hover { transform: translateY(-1px); }
.bucket--active { box-shadow: 0 0 0 2px rgba(16,185,129,.4); }
.bucket--red    { background: #fef2f2; border-color: #fecaca; }
.bucket--amber  { background: #fffbeb; border-color: #fde68a; }
.bucket--yellow { background: #fefce8; border-color: #fef08a; }
.bucket--green  { background: #ecfdf5; border-color: #a7f3d0; }
.bucket--info   { background: #f9fafb; border-color: #e5e7eb; cursor: default; }
.bucket-label { font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.bucket-count { font-size: 1.25rem; font-weight: 800; color: #111827; line-height: 1.2; }
.bucket-val { font-size: .66rem; color: #9ca3af; }

.ex-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.ex-table thead tr { background: #f9fafb; }
.ex-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.ex-table .th-r { text-align: right; }
.ex-table tbody tr { border-bottom: 1px solid #f9fafb; }
.ex-table tbody tr:hover { background: #fafafa; }
.ex-table td { padding: .5rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; font-weight: 600; }
.row-acked { opacity: .6; }

.pill { display: inline-block; font-size: .6rem; font-weight: 700; padding: .12rem .45rem; border-radius: 999px; margin-top: .15rem; }
.pill-red { background: #fee2e2; color: #dc2626; }
.pill-amber { background: #fef3c7; color: #d97706; }
.pill-yellow { background: #fef9c3; color: #a16207; }
.pill-green { background: #dcfce7; color: #15803d; }
.pill-gray { background: #f3f4f6; color: #9ca3af; }

.act { display: inline-block; font-size: .68rem; font-weight: 700; padding: .28rem .6rem; border-radius: .45rem; margin-left: .3rem; text-decoration: none; cursor: pointer; border: 1px solid; }
.act-red { background: #fef2f2; color: #dc2626; border-color: #fecaca; }
.act-red:hover { background: #fee2e2; }
.act-green { background: #ecfdf5; color: #047857; border-color: #a7f3d0; }
.act-green:hover { background: #d1fae5; }
</style>
