<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Kebutuhan Bahan (MRP)</h1>
      <p class="text-xs text-gray-500 mt-0.5">
        Forecast × resep → kebutuhan kotor − (stok + PR berjalan + transfer dalam perjalanan) + buffer → saran <b>beli / transfer / produksi</b>.
        Hasil tersimpan sebagai snapshot yang bisa diaudit; draft PR &amp; transfer masuk alur persetujuan biasa.
      </p>
    </div>

    <!-- Parameter run -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-[220px]">
          <label class="text-xs font-semibold text-gray-600">Gudang Tujuan</label>
          <SearchSelect v-model="runForm.warehouse_id" :options="warehouseOptions" placeholder="Pilih Gudang" />
        </div>
        <div>
          <label class="text-xs font-semibold text-gray-600">Horizon (hari)</label>
          <input type="number" min="1" max="28" v-model.number="runForm.horizon_days"
            class="block w-24 text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
        </div>
        <label class="flex items-center gap-1.5 text-xs font-medium text-gray-600 cursor-pointer pb-2.5">
          <input type="checkbox" v-model="runForm.include_par" class="rounded text-emerald-600" />
          Isi ulang sampai Par Level
        </label>
        <AppButton v-if="canRun" variant="primary" :loading="running" @click="doRun">
          <svg class="inline-block align-[-2px] mr-1" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="2" width="16" height="20" rx="2"/><line x1="8" x2="16" y1="6" y2="6"/><line x1="16" x2="16" y1="14" y2="18"/><path d="M16 10h.01"/><path d="M12 10h.01"/><path d="M8 10h.01"/><path d="M12 14h.01"/><path d="M8 14h.01"/><path d="M12 18h.01"/><path d="M8 18h.01"/></svg>
          Hitung Kebutuhan
        </AppButton>
        <div class="min-w-[240px] ml-auto">
          <label class="text-xs font-semibold text-gray-600">Riwayat Run</label>
          <SearchSelect v-model="selectedRunId" :options="runOptions" placeholder="Pilih run sebelumnya" @change="openRun" />
        </div>
      </div>
    </AppCard>

    <template v-if="run">
      <!-- Ringkasan run -->
      <div class="sum-grid">
        <div class="sum">
          <div class="sum-label">{{ run.run_number }}</div>
          <div class="sum-val">{{ run.warehouse_name }}</div>
          <div class="sum-sub">horizon {{ run.horizon_days }} hari · {{ fmtQty(run.forecast_qty) }} porsi forecast · oleh {{ run.created_by }} · {{ fmtDateTime(run.created_at) }}</div>
        </div>
        <div class="sum sum--blue"><div class="sum-label">Saran Beli</div><div class="sum-val">{{ countBy('purchase') }}</div><div class="sum-sub">item</div></div>
        <div class="sum sum--purple"><div class="sum-label">Saran Transfer</div><div class="sum-val">{{ countBy('transfer') }}</div><div class="sum-sub">dari gudang pusat</div></div>
        <div class="sum sum--teal"><div class="sum-label">Saran Produksi</div><div class="sum-val">{{ countBy('produce') }}</div><div class="sum-sub">barang setengah jadi (Fase 3)</div></div>
      </div>

      <div v-if="run.no_recipe_count > 0" class="warn-note">
        <svg class="inline-block align-[-2px] mr-0.5" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>
        <b>{{ run.no_recipe_count }} produk ter-forecast tidak punya resep/tautan bahan</b> sehingga tidak ikut dihitung:
        {{ run.no_recipe_products }}<span v-if="run.no_recipe_count > 12">, …</span>.
        Lengkapi resep di halaman <router-link to="/recipes" class="underline font-semibold">Resep</router-link> agar MRP representatif.
      </div>

      <AppCard :padding="false">
        <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
          <label class="flex items-center gap-1.5 text-xs font-medium text-gray-600 cursor-pointer">
            <input type="checkbox" v-model="needOnly" class="rounded text-emerald-600" /> Hanya yang perlu tindakan
          </label>
          <input v-model="itemSearch" placeholder="Cari item..."
            class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
          <div class="flex gap-2" v-if="canExecute">
            <AppButton variant="primary" :disabled="!selPurchase.length" :loading="executing" @click="createPR">
              Buat Draft PR ({{ selPurchase.length }})
            </AppButton>
            <AppButton variant="secondary" :disabled="!selTransfer.length" :loading="executing" @click="createTransfer">
              Buat Draft Transfer ({{ selTransfer.length }})
            </AppButton>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="mrp-table">
            <thead>
              <tr>
                <th class="th-c"><input type="checkbox" :checked="allChecked" @change="toggleAll($event.target.checked)" /></th>
                <th>Item</th>
                <th class="th-r" title="Kebutuhan kotor dari forecast × resep">Kotor</th>
                <th class="th-r">Stok</th>
                <th class="th-r" title="PR aktif (estimasi, dicocokkan per nama)">On-Order</th>
                <th class="th-r" title="Transfer berstatus terkirim menuju gudang ini">Transit</th>
                <th class="th-r" title="Par level (bila diaktifkan) atau safety stock">Buffer</th>
                <th class="th-r">Bersih</th>
                <th>Saran</th>
                <th class="th-r">Qty Saran</th>
                <th>Vendor / Harga Terakhir</th>
                <th>Tindak Lanjut</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="it in visibleItems" :key="it.item_id" :class="{ 'row-done': it.action_ref_id }">
                <td class="th-c">
                  <input v-if="selectable(it)" type="checkbox" v-model="checked[it.item_id]" />
                </td>
                <td>
                  <div class="font-medium text-gray-900 text-sm">{{ it.item_name }}</div>
                  <div class="text-[11px] text-gray-400 font-mono">{{ it.item_code }} · {{ it.category || '—' }}</div>
                </td>
                <td class="td-num">{{ fmtQty(it.gross_req) }} {{ it.base_unit }}</td>
                <td class="td-num">{{ fmtQty(it.on_hand) }}</td>
                <td class="td-num text-gray-500">{{ fmtQty(it.on_order) }}</td>
                <td class="td-num text-gray-500">{{ fmtQty(it.in_transit) }}</td>
                <td class="td-num text-gray-500">{{ fmtQty(run.include_par && it.par_level > 0 ? it.par_level : it.safety_stock) }}</td>
                <td class="td-num" :class="it.net_req > 0 ? 'text-red-600 font-bold' : 'text-emerald-600'">{{ fmtQty(it.net_req) }}</td>
                <td><span class="sug" :class="`sug--${it.suggestion}`">{{ sugLabel(it.suggestion) }}</span>
                  <div v-if="it.suggestion === 'transfer'" class="text-[10px] text-gray-400">dari {{ it.source_warehouse_name }}</div>
                </td>
                <td class="td-num font-bold">
                  <template v-if="it.qty_suggested_dist > 0">{{ fmtQty(it.qty_suggested_dist) }} {{ it.dist_unit_label || it.dist_unit }}</template>
                  <span v-else class="text-gray-300">—</span>
                </td>
                <td class="text-xs text-gray-500">
                  <template v-if="it.last_vendor || it.last_price_dist">
                    {{ it.last_vendor || '—' }}<br/><span class="text-[10px]">{{ fmtRp(it.last_price_dist) }}/{{ it.dist_unit_label || it.dist_unit }}</span>
                  </template>
                  <span v-else class="text-gray-300">belum ada histori</span>
                </td>
                <td>
                  <span v-if="it.action_ref_number" class="ref-pill">{{ refTypeLabel(it.action_ref_type) }} {{ it.action_ref_number }}</span>
                  <span v-else class="text-gray-300 text-xs">—</span>
                </td>
              </tr>
              <tr v-if="!visibleItems.length"><td colspan="12" class="p-8 text-center text-sm text-gray-400">Tidak ada baris.</td></tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <div v-else-if="!loadingRuns" class="p-10 text-center text-sm text-gray-400 bg-white rounded-xl border border-gray-200">
      Pilih gudang lalu klik <b>Hitung Kebutuhan</b> untuk membuat worksheet MRP,
      atau buka run sebelumnya dari Riwayat Run.
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
import { warehousesApi } from '@/api/warehouse'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import AppButton    from '@/components/ui/AppButton.vue'
import AppCard      from '@/components/ui/AppCard.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canRun = computed(() => auth.hasPermission('ppic.mrp.run'))
const canExecute = computed(() => auth.hasPermission('ppic.mrp.execute'))

const warehouseOptions = ref([])
const runForm = reactive({ warehouse_id: '', horizon_days: 7, include_par: true })
const running = ref(false)
const executing = ref(false)
const loadingRuns = ref(false)

const run = ref(null)
const runOptions = ref([])
const selectedRunId = ref('')
const checked = reactive({})
const needOnly = ref(true)
const itemSearch = ref('')

onMounted(async () => {
  loadRuns()
  try {
    const whRes = await warehousesApi.list({ limit: 100 })
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})` }))
    if (whList.length) runForm.warehouse_id = whList[0].id
  } catch { /* dropdown kosong, user tetap bisa buka riwayat */ }
})

async function loadRuns() {
  loadingRuns.value = true
  try {
    const res = await ppicApi.listMrpRuns({ limit: 20 })
    runOptions.value = (res.data || []).map(r => ({
      id: r.id,
      name: `${r.run_number} · ${r.warehouse_name} · ${fmtDateTime(r.created_at)}`,
    }))
  } catch { /* riwayat tidak wajib */ } finally {
    loadingRuns.value = false
  }
}

async function doRun() {
  if (!runForm.warehouse_id) { toast.error('Pilih gudang tujuan dulu'); return }
  running.value = true
  try {
    setRun(await ppicApi.runMrp({ ...runForm }))
    toast.success(`Run ${run.value.run_number}: ${run.value.items.length} item dihitung`)
    loadRuns()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menjalankan MRP')
  } finally {
    running.value = false
  }
}

async function openRun() {
  if (!selectedRunId.value) return
  try {
    setRun(await ppicApi.getMrpRun(selectedRunId.value))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuka run')
  }
}

function setRun(r) {
  run.value = r
  selectedRunId.value = r.id
  Object.keys(checked).forEach(k => delete checked[k])
  // Pre-select semua baris yang bisa dieksekusi.
  for (const it of r.items || []) {
    if (selectable(it)) checked[it.item_id] = true
  }
}

function selectable(it) {
  return !it.action_ref_id && (it.suggestion === 'purchase' || it.suggestion === 'transfer') && it.qty_suggested_dist > 0
}

const visibleItems = computed(() => {
  let items = run.value?.items || []
  if (needOnly.value) items = items.filter(i => i.suggestion !== 'none')
  if (itemSearch.value) {
    const q = itemSearch.value.toLowerCase()
    items = items.filter(i => i.item_name.toLowerCase().includes(q) || i.item_code.toLowerCase().includes(q))
  }
  return items
})

const selPurchase = computed(() => (run.value?.items || []).filter(i => checked[i.item_id] && i.suggestion === 'purchase' && selectable(i)).map(i => i.item_id))
const selTransfer = computed(() => (run.value?.items || []).filter(i => checked[i.item_id] && i.suggestion === 'transfer' && selectable(i)).map(i => i.item_id))
const allChecked = computed(() => {
  const sel = visibleItems.value.filter(selectable)
  return sel.length > 0 && sel.every(i => checked[i.item_id])
})
function toggleAll(on) {
  for (const it of visibleItems.value) {
    if (selectable(it)) checked[it.item_id] = on
  }
}

async function createPR() {
  if (!confirm(`Buat 1 draft PR berisi ${selPurchase.value.length} item? PR masuk Pengadaan → Bahan Baku dan langsung menunggu persetujuan Manager Logistik (vendor & harga diisi pembelian).`)) return
  executing.value = true
  try {
    const pr = await ppicApi.mrpCreatePR(run.value.id, selPurchase.value)
    toast.success(`Draft PR ${pr.request_number} dibuat`)
    setRun(await ppicApi.getMrpRun(run.value.id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuat draft PR')
  } finally {
    executing.value = false
  }
}

async function createTransfer() {
  if (!confirm(`Buat draft transfer untuk ${selTransfer.value.length} item dari gudang pusat?`)) return
  executing.value = true
  try {
    const list = await ppicApi.mrpCreateTransfer(run.value.id, selTransfer.value)
    toast.success(`${list.length} draft transfer dibuat: ${list.map(t => t.transfer_number).join(', ')}`)
    setRun(await ppicApi.getMrpRun(run.value.id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuat draft transfer')
  } finally {
    executing.value = false
  }
}

function countBy(s) { return (run.value?.items || []).filter(i => i.suggestion === s).length }
const SUG_LABELS = { purchase: 'Beli', transfer: 'Transfer', produce: 'Produksi', none: 'Cukup' }
function sugLabel(s) { return SUG_LABELS[s] ?? s }
function refTypeLabel(t) { return t === 'purchase_request' ? 'PR' : (t === 'stock_transfer' ? 'TRF' : t) }

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtRp(v) {
  if (!v) return 'Rp 0'
  return 'Rp ' + Math.round(v).toLocaleString('id-ID')
}
function fmtDateTime(s) { return s ? new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '' }
</script>

<style scoped>
.sum-grid { display: grid; grid-template-columns: 2fr 1fr 1fr 1fr; gap: .6rem; }
@media (max-width: 900px) { .sum-grid { grid-template-columns: 1fr 1fr; } }
.sum { padding: .7rem .9rem; border-radius: .75rem; border: 1.5px solid #e5e7eb; background: #fff; }
.sum--blue   { background: #eff6ff; border-color: #bfdbfe; }
.sum--purple { background: #faf5ff; border-color: #e9d5ff; }
.sum--teal   { background: #f0fdfa; border-color: #99f6e4; }
.sum-label { font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.sum-val { font-size: 1.1rem; font-weight: 800; color: #111827; }
.sum-sub { font-size: .66rem; color: #9ca3af; }

.warn-note { font-size: .75rem; color: #92400e; background: #fffbeb; border: 1px solid #fde68a; border-radius: .6rem; padding: .6rem .8rem; }

.mrp-table { width: 100%; border-collapse: collapse; font-size: .78rem; }
.mrp-table thead tr { background: #f9fafb; }
.mrp-table th { padding: .5rem .55rem; text-align: left; font-size: .62rem; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.mrp-table .th-r { text-align: right; }
.mrp-table .th-c { width: 30px; text-align: center; }
.mrp-table tbody tr { border-bottom: 1px solid #f9fafb; }
.mrp-table tbody tr:hover { background: #fafafa; }
.mrp-table td { padding: .45rem .55rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; }
.row-done { opacity: .55; }

.sug { font-size: .64rem; font-weight: 700; padding: .16rem .5rem; border-radius: 999px; white-space: nowrap; }
.sug--purchase { background: #dbeafe; color: #1d4ed8; }
.sug--transfer { background: #ede9fe; color: #6d28d9; }
.sug--produce  { background: #ccfbf1; color: #0f766e; }
.sug--none     { background: #dcfce7; color: #15803d; }

.ref-pill { font-size: .64rem; font-weight: 700; padding: .16rem .5rem; border-radius: .4rem; background: #f3f4f6; color: #374151; white-space: nowrap; }
</style>
