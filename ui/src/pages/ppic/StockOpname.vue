<template>
  <div class="space-y-5">

    <!-- ══ DETAIL VIEW ══ -->
    <template v-if="detail">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button class="text-xs text-emerald-700 font-semibold hover:underline" @click="closeDetail">← Kembali ke daftar</button>
          <h1 class="text-xl font-bold text-gray-900 mt-1">
            {{ detail.opname_number }}
            <span class="pill ml-2" :class="statusPill(detail.status)">{{ statusLabel(detail.status) }}</span>
          </h1>
          <p class="text-xs text-gray-500 mt-0.5">
            {{ detail.warehouse_name }} <template v-if="detail.category">· kategori {{ detail.category }}</template>
            · dibuat {{ fmtDateTime(detail.created_at) }} oleh {{ detail.created_by || '—' }}
            <template v-if="detail.status === 'approved'"> · disetujui {{ fmtDateTime(detail.approved_at) }} oleh {{ detail.approved_by }}</template>
          </p>
        </div>
        <div class="flex gap-2 flex-wrap">
          <template v-if="detail.status === 'counting'">
            <AppActionButton icon="cancel" label="Batalkan" v-if="canCreate" :loading="acting" @click="cancelSession" />
            <AppButton v-if="canCreate" variant="secondary" :disabled="!dirtyCount" :loading="savingCounts" @click="saveCounts">Simpan Hitungan ({{ dirtyCount }})</AppButton>
            <AppActionButton icon="review" label="Ajukan Review" v-if="canCreate" :loading="acting" @click="submitSession" />
          </template>
          <template v-else-if="detail.status === 'review'">
            <AppActionButton icon="cancel" label="Batalkan" v-if="canCreate" :loading="acting" @click="cancelSession" />
            <AppButton v-if="canCreate" variant="secondary" :disabled="!dirtyCount" :loading="savingCounts" @click="saveCounts">Simpan Koreksi ({{ dirtyCount }})</AppButton>
            <AppActionButton icon="approve" label="Approve &amp; Posting Penyesuaian" v-if="canApprove" :loading="acting" @click="approveSession" />
          </template>
        </div>
      </div>

      <!-- Progress cards -->
      <div class="stat-grid">
        <div class="stat"><div class="stat-label">Item</div><div class="stat-val">{{ detail.items_counted }}/{{ detail.items_total }}</div><div class="stat-sub">sudah dihitung</div></div>
        <div class="stat" :class="detail.items_diff > 0 ? 'stat--amber' : 'stat--ok'"><div class="stat-label">Selisih</div><div class="stat-val">{{ detail.items_diff }}</div><div class="stat-sub">item berbeda</div></div>
        <div class="stat" :class="detail.diff_value < 0 ? 'stat--red' : (detail.diff_value > 0 ? 'stat--amber' : 'stat--ok')">
          <div class="stat-label">Nilai Selisih</div><div class="stat-val">{{ fmtRp(detail.diff_value) }}</div><div class="stat-sub">fisik − sistem</div>
        </div>
        <div class="stat" :class="detail.accuracy_pct >= 97 ? 'stat--ok' : 'stat--amber'">
          <div class="stat-label">Akurasi (IRA)</div><div class="stat-val">{{ detail.items_counted ? detail.accuracy_pct.toFixed(1) + '%' : '—' }}</div><div class="stat-sub">target ≥ 97%</div>
        </div>
      </div>

      <div v-if="detail.status === 'counting'" class="blind-note">
        <svg class="inline-block align-[-2px] mr-0.5" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.88 9.88a3 3 0 1 0 4.24 4.24"/><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68"/><path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61"/><line x1="2" x2="22" y1="2" y2="22"/></svg>
        <b>Blind count:</b> qty sistem disembunyikan selama penghitungan (standar audit). Selisih tampil setelah sesi diajukan review.
      </div>

      <AppCard :padding="false">
        <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100">
          <input v-model="itemSearch" placeholder="Cari item..."
            class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
          <label v-if="showDiffCols" class="flex items-center gap-1.5 text-xs font-medium text-gray-600 cursor-pointer">
            <input type="checkbox" v-model="diffOnly" class="rounded text-emerald-600" /> Hanya selisih
          </label>
        </div>
        <div class="overflow-x-auto">
          <table class="op-table">
            <thead>
              <tr>
                <th>Item</th>
                <th v-if="showDiffCols" class="th-r">Qty Sistem</th>
                <th class="th-r">Qty Fisik</th>
                <th v-if="showDiffCols" class="th-r">Selisih</th>
                <th v-if="showDiffCols" class="th-r">Nilai Selisih</th>
                <th>Alasan</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="it in visibleItems" :key="it.item_id" :class="{ 'row-dirty': isDirty(it) }">
                <td>
                  <div class="font-medium text-gray-900 text-sm">{{ it.item_name }}</div>
                  <div class="text-[11px] text-gray-400 font-mono">{{ it.item_code }} · {{ it.category || '—' }}</div>
                </td>
                <td v-if="showDiffCols" class="td-num text-gray-500">{{ fmtQty(it.qty_system_base) }} {{ it.base_unit }}</td>
                <td class="td-num">
                  <input v-if="editable" type="number" min="0" step="0.01" class="num-in"
                    :value="it.qty_counted_base" @input="setCount(it, $event.target.value)" />
                  <span v-else>{{ it.qty_counted_base == null ? '—' : fmtQty(it.qty_counted_base) }}</span>
                  <span class="text-[11px] text-gray-400 ml-1">{{ it.base_unit }}</span>
                </td>
                <td v-if="showDiffCols" class="td-num" :class="diffClass(it)">{{ diffLabel(it) }}</td>
                <td v-if="showDiffCols" class="td-num" :class="diffClass(it)">{{ it.qty_counted_base == null ? '—' : fmtRp((it.qty_counted_base - it.qty_system_base) * it.cost_per_base) }}</td>
                <td>
                  <input v-if="editable" v-model="it.reason" placeholder="wajib bila selisih..." class="reason-in" />
                  <span v-else class="text-xs text-gray-500">{{ it.reason || '—' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ LIST VIEW ══ -->
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-bold text-gray-900">Stock Opname (Cycle Count)</h1>
          <p class="text-xs text-gray-500 mt-0.5">Hitung fisik → review selisih → approve → penyesuaian diposting ke buku stok.</p>
        </div>
        <AppButton v-if="canCreate" variant="primary" @click="openCreate">+ Sesi Opname Baru</AppButton>
      </div>

      <AppCard :padding="false">
        <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
          <div class="min-w-[220px]">
            <SearchSelect v-model="filters.warehouse_id" :options="warehouseOptions" placeholder="Semua Gudang" @change="reload" />
          </div>
          <div class="min-w-[170px]">
            <SearchSelect v-model="filters.status" :options="STATUS_OPTIONS" placeholder="Semua Status" @change="reload" />
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="op-table">
            <thead>
              <tr><th>Nomor</th><th>Gudang</th><th>Status</th><th class="th-r">Progress</th><th class="th-r">Selisih</th><th class="th-r">Akurasi</th><th>Dibuat</th><th></th></tr>
            </thead>
            <tbody>
              <tr v-if="loading"><td colspan="8" class="p-8 text-center"><AppSpinner class="text-emerald-600 mx-auto" /></td></tr>
              <tr v-else-if="!rows.length"><td colspan="8" class="p-8 text-center text-sm text-gray-400">Belum ada sesi opname.</td></tr>
              <tr v-for="r in rows" :key="r.id" class="cursor-pointer" @click="openDetail(r.id)">
                <td class="font-mono text-xs font-bold text-gray-700">{{ r.opname_number }}</td>
                <td>
                  <div class="text-sm text-gray-800">{{ r.warehouse_name }}</div>
                  <div class="text-[11px] text-gray-400">{{ r.category ? 'Kategori: ' + r.category : 'Semua item' }}</div>
                </td>
                <td><span class="pill" :class="statusPill(r.status)">{{ statusLabel(r.status) }}</span></td>
                <td class="td-num">{{ r.items_counted }}/{{ r.items_total }}</td>
                <td class="td-num" :class="r.items_diff ? 'text-amber-600' : ''">{{ r.items_diff }} item · {{ fmtRp(r.diff_value) }}</td>
                <td class="td-num">{{ r.items_counted ? r.accuracy_pct.toFixed(1) + '%' : '—' }}</td>
                <td class="text-xs text-gray-500">{{ fmtDateTime(r.created_at) }}<br/><span class="text-[10px]">{{ r.created_by }}</span></td>
                <td class="text-right pr-3 text-emerald-700 text-xs font-semibold">Buka →</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="px-4 py-3 border-t border-gray-100 flex items-center justify-between">
          <span class="text-xs text-gray-400">{{ total }} sesi</span>
          <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
        </div>
      </AppCard>

      <!-- Create modal -->
      <AppModal v-model="showCreate" title="Sesi Opname Baru" size="lg">
        <div class="modal-form-grid">
          <div>
            <label class="text-xs font-semibold text-gray-600">Gudang</label>
            <SearchSelect v-model="createForm.warehouse_id" :options="warehouseOnlyOptions" placeholder="Pilih Gudang" />
          </div>
          <div>
            <label class="text-xs font-semibold text-gray-600">Batasi Kategori (opsional — cycle count)</label>
            <SearchSelect v-model="createForm.category" :options="categoryOptions" placeholder="Semua Kategori" />
          </div>
          <div class="modal-span-full">
            <label class="text-xs font-semibold text-gray-600">Catatan</label>
            <textarea v-model="createForm.notes" rows="2" class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400"></textarea>
          </div>
          <p class="modal-span-full text-[11px] text-gray-400">Qty sistem di-snapshot saat sesi dibuat. Satu gudang hanya boleh punya satu sesi aktif.</p>
        </div>
        <template #footer>
          <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
          <AppButton variant="primary" :loading="creating" @click="createSession">Mulai Opname</AppButton>
        </template>
      </AppModal>
    </template>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
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
const canCreate = computed(() => auth.hasPermission('ppic.opname.create'))
const canApprove = computed(() => auth.hasPermission('ppic.opname.approve'))

const STATUS_OPTIONS = [
  { id: '', name: 'Semua Status' },
  { id: 'counting', name: 'Menghitung' },
  { id: 'review', name: 'Review' },
  { id: 'approved', name: 'Disetujui' },
  { id: 'cancelled', name: 'Dibatalkan' },
]

// ── List state ──
const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)
const filters = reactive({ warehouse_id: '', status: '' })
const warehouseOptions = ref([{ id: '', name: 'Semua Gudang' }])
const warehouseOnlyOptions = ref([])
const categoryOptions = ref([{ id: '', name: 'Semua Kategori' }])

// ── Create state ──
const showCreate = ref(false)
const creating = ref(false)
const createForm = reactive({ warehouse_id: '', category: '', notes: '' })

// ── Detail state ──
const detail = ref(null)
const originalCounts = ref({})
const itemSearch = ref('')
const diffOnly = ref(false)
const savingCounts = ref(false)
const acting = ref(false)

onMounted(async () => {
  load()
  try {
    const lookup = await ppicApi.getLookups()
    warehouseOnlyOptions.value = lookup.warehouses.map(w => ({ id: w.id, name: w.name }))
    warehouseOptions.value = [{ id: '', name: 'Semua Gudang' }, ...warehouseOnlyOptions.value]
    const cats = [...new Set(lookup.items.map(item => item.category).filter(Boolean))].sort()
    categoryOptions.value = [{ id: '', name: 'Semua Kategori' }, ...cats.map(name => ({ id: name, name }))]
  } catch (e) { toast.error(e?.message || 'Gagal memuat pilihan gudang dan kategori opname') }
})

async function load() {
  loading.value = true
  try {
    const res = await ppicApi.listOpnames({ ...filters, page: page.value, limit: 20 })
    rows.value = res.data || []
    total.value = res.total || 0
    totalPages.value = res.total_pages || 1
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat sesi opname')
  } finally {
    loading.value = false
  }
}
function reload() { page.value = 1; load() }
function changePage(p) { page.value = p; load() }

function openCreate() {
  Object.assign(createForm, { warehouse_id: filters.warehouse_id || '', category: '', notes: '' })
  showCreate.value = true
}
async function createSession() {
  if (!createForm.warehouse_id) { toast.error('Pilih gudang dulu'); return }
  creating.value = true
  try {
    const so = await ppicApi.createOpname({ ...createForm })
    showCreate.value = false
    toast.success(`Sesi ${so.opname_number} dibuat — ${so.items_total} item siap dihitung`)
    setDetail(so)
    load()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuat sesi opname')
  } finally {
    creating.value = false
  }
}

async function openDetail(id) {
  try {
    setDetail(await ppicApi.getOpname(id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membuka sesi')
  }
}
function setDetail(so) {
  detail.value = so
  itemSearch.value = ''
  diffOnly.value = false
  originalCounts.value = Object.fromEntries((so.items || []).map(i => [i.item_id, countSnap(i)]))
}
function closeDetail() { detail.value = null; load() }

const editable = computed(() => canCreate.value && detail.value && ['counting', 'review'].includes(detail.value.status))
const showDiffCols = computed(() => detail.value && detail.value.status !== 'counting')

const visibleItems = computed(() => {
  let items = detail.value?.items || []
  if (itemSearch.value) {
    const q = itemSearch.value.toLowerCase()
    items = items.filter(i => i.item_name.toLowerCase().includes(q) || i.item_code.toLowerCase().includes(q))
  }
  if (diffOnly.value && showDiffCols.value) {
    items = items.filter(i => i.qty_counted_base != null && i.qty_counted_base !== i.qty_system_base)
  }
  return items
})

function countSnap(i) { return JSON.stringify([i.qty_counted_base, i.reason]) }
function isDirty(i) { return originalCounts.value[i.item_id] !== countSnap(i) }
const dirtyCount = computed(() => (detail.value?.items || []).filter(isDirty).length)

function setCount(it, val) {
  it.qty_counted_base = val === '' ? null : Number(val)
}

async function saveCounts() {
  const dirty = (detail.value.items || []).filter(isDirty).map(i => ({
    item_id: i.item_id,
    qty_counted_base: i.qty_counted_base,
    reason: i.reason || '',
  }))
  if (!dirty.length) return
  savingCounts.value = true
  try {
    await ppicApi.saveOpnameCounts(detail.value.id, dirty)
    toast.success(`${dirty.length} hitungan tersimpan`)
    setDetail(await ppicApi.getOpname(detail.value.id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal menyimpan hitungan')
  } finally {
    savingCounts.value = false
  }
}

async function submitSession() {
  if (dirtyCount.value && !confirm('Ada hitungan belum disimpan — tetap ajukan tanpa menyimpannya?')) return
  acting.value = true
  try {
    await ppicApi.submitOpname(detail.value.id)
    toast.success('Sesi diajukan untuk review')
    setDetail(await ppicApi.getOpname(detail.value.id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal mengajukan review')
  } finally {
    acting.value = false
  }
}

async function approveSession() {
  const uncounted = detail.value.items_total - detail.value.items_counted
  let msg = `Approve ${detail.value.opname_number}? Stok ${detail.value.items_counted} item akan DISETEL ke qty fisik (penyesuaian diposting ke buku stok).`
  if (uncounted > 0) msg += ` ${uncounted} item yang tidak dihitung akan dilewati.`
  if (!confirm(msg)) return
  acting.value = true
  try {
    await ppicApi.approveOpname(detail.value.id)
    toast.success('Opname disetujui — penyesuaian stok diposting')
    setDetail(await ppicApi.getOpname(detail.value.id))
  } catch (e) {
    toast.error(e?.message ?? 'Gagal approve opname')
  } finally {
    acting.value = false
  }
}

async function cancelSession() {
  if (!confirm('Batalkan sesi opname ini? Hitungan yang sudah diisi ikut hangus.')) return
  acting.value = true
  try {
    await ppicApi.cancelOpname(detail.value.id)
    toast.success('Sesi dibatalkan')
    closeDetail()
  } catch (e) {
    toast.error(e?.message ?? 'Gagal membatalkan sesi')
  } finally {
    acting.value = false
  }
}

function diffClass(it) {
  if (it.qty_counted_base == null) return 'text-gray-300'
  const d = it.qty_counted_base - it.qty_system_base
  if (d < 0) return 'text-red-600'
  if (d > 0) return 'text-amber-600'
  return 'text-emerald-600'
}
function diffLabel(it) {
  if (it.qty_counted_base == null) return '—'
  const d = it.qty_counted_base - it.qty_system_base
  return (d > 0 ? '+' : '') + fmtQty(d)
}

const STATUS_LABELS = { counting: 'Menghitung', review: 'Review', approved: 'Disetujui', cancelled: 'Dibatalkan' }
function statusLabel(s) { return STATUS_LABELS[s] ?? s }
function statusPill(s) {
  return { counting: 'pill-blue', review: 'pill-amber', approved: 'pill-green', cancelled: 'pill-gray' }[s] ?? 'pill-gray'
}

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtRp(v) {
  const n = Number(v ?? 0)
  const abs = Math.abs(n)
  const s = abs >= 1_000_000 ? (abs / 1_000_000).toFixed(1) + ' jt' : Math.round(abs).toLocaleString('id-ID')
  return (n < 0 ? '-Rp ' : 'Rp ') + s
}
function fmtDateTime(s) { return s ? new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '—' }
</script>

<style scoped>
.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: .6rem; }
@media (max-width: 800px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
.stat { padding: .7rem .9rem; border-radius: .75rem; border: 1.5px solid #e5e7eb; background: #fff; }
.stat--ok    { background: #ecfdf5; border-color: #a7f3d0; }
.stat--amber { background: #fffbeb; border-color: #fde68a; }
.stat--red   { background: #fef2f2; border-color: #fecaca; }
.stat-label { font-size: .62rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.stat-val { font-size: 1.2rem; font-weight: 800; color: #111827; }
.stat-sub { font-size: .64rem; color: #9ca3af; }

.blind-note { font-size: .75rem; color: #92400e; background: #fffbeb; border: 1px solid #fde68a; border-radius: .6rem; padding: .6rem .8rem; }

.op-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.op-table thead tr { background: #f9fafb; }
.op-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.op-table .th-r { text-align: right; }
.op-table tbody tr { border-bottom: 1px solid #f9fafb; }
.op-table tbody tr:hover { background: #fafafa; }
.op-table td { padding: .5rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; font-weight: 600; }
.row-dirty { background: #fefce8 !important; }

.num-in { width: 92px; text-align: right; font-size: .8rem; font-family: monospace; border: 1px solid #e5e7eb; border-radius: .4rem; padding: .3rem .45rem; }
.num-in:focus { outline: none; border-color: #34d399; box-shadow: 0 0 0 2px rgba(52,211,153,.2); }
.reason-in { width: 100%; min-width: 130px; font-size: .74rem; border: 1px solid #e5e7eb; border-radius: .4rem; padding: .3rem .45rem; }
.reason-in:focus { outline: none; border-color: #34d399; }

.pill { display: inline-block; font-size: .62rem; font-weight: 700; padding: .15rem .5rem; border-radius: 999px; white-space: nowrap; }
.pill-blue { background: #dbeafe; color: #1d4ed8; }
.pill-amber { background: #fef3c7; color: #d97706; }
.pill-green { background: #dcfce7; color: #15803d; }
.pill-gray { background: #f3f4f6; color: #9ca3af; }
</style>
