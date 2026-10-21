<template>
  <div class="space-y-5">
    <div class="flex flex-wrap justify-between items-start gap-3">
      <div><h1 class="text-2xl font-bold text-gray-900">Reject, Retur & Kedaluwarsa</h1><p class="text-sm text-gray-500 mt-1">Karantina barang, keputusan supervisor, dan penyelesaian dengan bukti foto.</p></div>
      <AppButton v-if="canReport" @click="openReport">+ Laporkan Barang</AppButton>
    </div>
    <AppCard><div class="grid sm:grid-cols-3 gap-4 text-sm"><p><b class="block text-gray-900">1. Periksa & karantina</b><span class="text-gray-500">Pilih batch dan foto. Barang bermasalah dipisahkan dari stok siap pakai.</span></p><p><b class="block text-gray-900">2. Keputusan supervisor</b><span class="text-gray-500">Retur, pemusnahan, atau pelepasan karantina. Pelapor tidak menyetujui laporannya sendiri.</span></p><p><b class="block text-gray-900">3. Selesaikan & buktikan</b><span class="text-gray-500">Gudang induk menerima retur/pengganti; finance mencatat dana yang benar-benar kembali.</span></p></div></AppCard>
    <AppCard><div class="flex flex-wrap gap-3"><SearchableSelect v-model="warehouse" class="field" aria-label="Filter gudang"><option value="">Semua gudang dalam akses</option><option v-for="w in lookups.warehouses" :key="w.id" :value="w.id">{{ w.name }} · {{ w.type === 'central' ? 'Induk' : 'Outlet' }}</option></SearchableSelect><SearchableSelect v-model="status" class="field" aria-label="Filter status"><option value="">Semua status</option><option v-for="(label, key) in QUALITY_STATUSES" :key="key" :value="key">{{ label }}</option></SearchableSelect><input v-model="search" class="field flex-1 basis-full sm:basis-48" aria-label="Cari laporan" placeholder="Nomor, nama barang, atau supplier" /></div></AppCard>
    <AppAlert type="error" :message="error" />
    <p v-if="loading" class="text-gray-500 text-sm">Memuat laporan…</p>
    <AppCard v-else-if="!rows.length"><p class="text-gray-500 text-sm">Belum ada laporan pada filter ini.</p></AppCard>
    <div v-else class="grid md:grid-cols-2 xl:grid-cols-3 gap-4">
      <AppCard v-for="row in rows" :key="row.id">
        <div class="flex flex-wrap justify-between gap-2 text-xs"><b class="text-emerald-700">{{ row.number }}</b><span class="text-gray-500">{{ row.warehouse_type === 'central' ? 'Gudang Induk' : 'Gudang Outlet' }}</span></div>
        <h2 class="font-semibold text-lg mt-3">{{ row.item_name }}</h2><p class="text-sm text-gray-500">{{ row.warehouse_name }}</p>
        <p class="text-sm mt-2">{{ row.qty_input }} {{ row.unit_used }} <span class="text-gray-400">({{ row.qty_base }} {{ row.base_unit }})</span> · {{ QUALITY_REASONS[row.reason] }}</p>
        <p class="text-xs mt-3 rounded-lg bg-amber-50 text-amber-900 px-2 py-2">{{ QUALITY_STATUSES[row.status] }}</p>
        <p v-if="row.vendor_name" class="text-xs text-gray-500 mt-2">{{ row.vendor_name }} · {{ row.settlement === 'refund' ? 'Pengembalian dana' : 'Barang pengganti' }}</p>
        <p v-if="row.parent_case_id" class="text-xs text-gray-500 mt-2">Retur dari outlet · tetap dikarantina</p>
        <div class="flex flex-wrap gap-2 mt-4"><AppActionButton icon="view" label="Detail &amp; Bukti" @click="openDetail(row.id)" /><AppActionButton :icon="action === 'execute' ? (row.disposition === 'dispose' ? 'dispose' : 'return') : ({ approve: 'approve', release: 'unlock', receive_return: 'receive', replacement: 'receive', refund: 'refund' })[action]" :label="(actionLabel(row, action))" v-for="action in actions(row)" :key="action" @click="openAction(row, action)" /></div>
      </AppCard>
    </div>
    <AppPagination :page="page" :total-pages="Math.max(1, Math.ceil(total / 20))" @change="page = $event" />
    <AppModal v-model="showReport" title="Laporkan Barang & Karantina" size="lg">
      <div class="space-y-4">
        <label class="block text-sm">Gudang *<SearchableSelect v-model="report.warehouse_id" class="field w-full mt-1"><option value="">Pilih gudang</option><option v-for="w in reportWarehouses" :key="w.id" :value="w.id">{{ w.name }}</option></SearchableSelect></label>
        <div><label class="label">Barang *</label><SearchSelect v-model="report.item_id" :options="items" placeholder="Pilih barang" /></div>
        <label class="block text-sm">Batch stok *<SearchableSelect v-model="report.batch_id" class="field w-full mt-1" :disabled="batchLoading"><option value="">{{ batchLoading ? 'Memuat batch…' : 'Pilih batch sesuai barang fisik' }}</option><option v-for="b in batches" :key="b.id" :value="b.id">{{ b.id.slice(-8) }} · {{ b.qty_base }} {{ selectedItem?.base_unit }} · {{ b.expiry_date ? `ED ${b.expiry_date}` : 'Tanpa tanggal ED' }}</option></SearchableSelect></label>
        <AppAlert type="error" :message="batchError" />
        <div class="grid sm:grid-cols-2 gap-3"><label class="text-sm">Jumlah *<div class="flex gap-2 mt-1"><input v-model.number="report.qty" type="number" min="0.0001" step="0.0001" class="field w-full" /><SearchableSelect v-model="report.unit_used" class="field"><option v-for="unit in units" :key="unit" :value="unit">{{ unit }}</option></SearchableSelect></div></label><label class="text-sm">Alasan *<SearchableSelect v-model="report.reason" class="field w-full mt-1"><option v-for="(label, key) in reportReasons" :key="key" :value="key">{{ label }}</option></SearchableSelect></label></div>
        <p v-if="selectedItem" class="text-xs text-gray-500">Dikarantina: {{ (Number(report.qty || 0) * stockUnitRatio(selectedItem, report.unit_used)).toFixed(4) }} {{ selectedItem.base_unit }}. Pilih batch yang benar agar biaya dan kedaluwarsa tetap sesuai.</p>
        <label class="block text-sm">Temuan / alasan retur *<textarea v-model="report.notes" rows="3" class="field w-full mt-1" placeholder="Kondisi barang, penyebab, nomor kemasan/lot, dan detail kejadian"></textarea></label>
        <StockEvidenceCapture v-model="reportEvidence" :warehouse-id="report.warehouse_id" label="Foto kondisi barang / label kedaluwarsa" />
      </div>
      <template #footer><AppButton variant="secondary" @click="showReport = false">Batal</AppButton><AppButton :loading="saving" :disabled="!reportEvidence?.id || batchLoading || !!batchError" @click="saveReport">Simpan & Karantina</AppButton></template>
    </AppModal>
    <AppModal v-model="showDetail" title="Riwayat Laporan & Bukti" size="lg">
      <div v-if="detail" class="space-y-4">
        <div><h2 class="font-bold">{{ detail.number }} · {{ detail.item_name }}</h2><p class="text-sm text-gray-500">{{ detail.warehouse_name }} · {{ detail.qty_base }} {{ detail.base_unit }} · {{ QUALITY_STATUSES[detail.status] }}</p><p class="text-sm mt-2 whitespace-pre-line">{{ detail.notes }}</p></div>
        <dl class="grid sm:grid-cols-2 gap-3 text-sm"><div><dt class="text-gray-500">Nilai barang dikarantina</dt><dd>{{ money(detail.cost_per_base * detail.qty_base) }}</dd></div><div><dt class="text-gray-500">Kedaluwarsa batch</dt><dd>{{ detail.expiry_date || 'Tidak tercatat' }}</dd></div><div v-if="detail.disposition"><dt class="text-gray-500">Keputusan</dt><dd>{{ QUALITY_DISPOSITIONS[detail.disposition] }}</dd></div><div v-if="detail.return_warehouse_name"><dt class="text-gray-500">Tujuan retur</dt><dd>{{ detail.return_warehouse_name }}</dd></div><div v-if="detail.vendor_name"><dt class="text-gray-500">Supplier / invoice</dt><dd>{{ detail.vendor_name }} · {{ detail.supplier_reference }}</dd></div><div v-if="detail.settlement"><dt class="text-gray-500">Penyelesaian</dt><dd>{{ detail.settled_qty_base }} / {{ detail.qty_base }} {{ detail.base_unit }} · {{ detail.settlement === 'refund' ? money(detail.refund_amount) : 'Barang pengganti' }}</dd></div></dl>
        <div v-for="event in detail.events" :key="event.id" class="border-t pt-3"><b class="text-sm">{{ QUALITY_ACTIONS[event.action] || event.action }}</b><p class="text-xs text-gray-500">{{ date(event.created_at) }} · {{ event.actor_name }}</p><p class="text-sm whitespace-pre-line mt-1">{{ event.notes }}</p><p v-if="event.reference" class="text-xs mt-1">Referensi: {{ event.reference }}</p><p v-if="event.qty_base" class="text-xs">{{ event.qty_base }} {{ detail.base_unit }}<span v-if="event.amount"> · {{ money(event.amount) }}</span></p><a v-if="event.evidence" :href="safeUrl(event.evidence.url)" target="_blank" rel="noopener noreferrer" class="block mt-2"><img :src="safeUrl(event.evidence.url)" alt="Bukti bertimestamp" class="max-h-64 max-w-full rounded-lg border" /><span class="text-xs text-gray-500">Dicatat server {{ date(event.evidence.recorded_at) }} · {{ event.evidence.actor_name }}</span></a></div>
      </div><p v-else class="text-sm text-gray-500">Memuat detail…</p>
    </AppModal>
    <AppModal v-model="showAction" :title="actionCase ? actionLabel(actionCase, actionForm.action) : 'Tindak Lanjut'" size="lg">
      <div v-if="actionCase" class="space-y-4">
        <p class="text-sm font-semibold">{{ actionCase.number }} · {{ actionCase.item_name }} · {{ actionCase.qty_base }} {{ actionCase.base_unit }}</p>
        <template v-if="actionForm.action === 'approve'">
          <label class="block text-sm">Keputusan *<SearchableSelect v-model="actionForm.disposition" class="field w-full mt-1"><option value="dispose">Pemusnahan</option><option v-if="actionCase.warehouse_type === 'central'" value="return_supplier">Retur supplier</option><option v-else value="return_warehouse">Retur ke gudang induk</option></SearchableSelect></label>
          <template v-if="actionForm.disposition === 'return_supplier'"><div><label class="label">Supplier *</label><SearchSelect v-model="actionForm.vendor_id" :options="lookups.vendors" placeholder="Pilih supplier" /></div><label class="block text-sm">Invoice / surat jalan supplier *<input v-model="actionForm.reference" class="field w-full mt-1" /></label><label class="block text-sm">Penyelesaian *<SearchableSelect v-model="actionForm.settlement" class="field w-full mt-1"><option value="replacement">Barang pengganti</option><option value="refund">Pengembalian dana</option></SearchableSelect></label></template>
          <label v-if="actionForm.disposition === 'return_warehouse'" class="block text-sm">Gudang induk tujuan *<SearchableSelect v-model="actionForm.return_warehouse_id" class="field w-full mt-1"><option value="">Pilih gudang induk</option><option v-for="w in lookups.return_warehouses" :key="w.id" :value="w.id">{{ w.name }}</option></SearchableSelect></label>
        </template>
        <p v-if="actionForm.action === 'release'" class="text-sm text-amber-800 bg-amber-50 p-3 rounded-lg">Pastikan hasil pemeriksaan menyatakan barang layak digunakan. Tindakan ini mengembalikan barang ke stok siap pakai.</p>
        <p v-if="actionForm.action === 'receive_return'" class="text-sm text-amber-800 bg-amber-50 p-3 rounded-lg">Barang diterima di {{ actionCase.return_warehouse_name }} sebagai karantina baru untuk diperiksa Gudang Induk.</p>
        <template v-if="['refund', 'replacement'].includes(actionForm.action)">
          <p class="text-sm text-gray-500">Sisa penyelesaian: {{ actionCase.qty_base - actionCase.settled_qty_base }} {{ actionCase.base_unit }}. Bisa dicatat bertahap sesuai yang benar-benar diterima.</p>
          <label class="block text-sm">Jumlah barang yang diselesaikan ({{ actionCase.base_unit }}) *<input v-model.number="actionForm.qty_base" type="number" min="0.0001" step="0.0001" :max="actionCase.qty_base - actionCase.settled_qty_base" class="field w-full mt-1" /></label>
          <label v-if="actionForm.action === 'refund'" class="block text-sm">Nominal dana diterima (Rp) *<input v-model.number="actionForm.amount" type="number" min="0.01" step="0.01" class="field w-full mt-1" /></label>
          <label v-else class="block text-sm">Tanggal kedaluwarsa barang pengganti<input v-model="actionForm.expiry_date" type="date" class="field w-full mt-1" /></label>
          <label class="block text-sm">Nomor bukti transfer dana / surat jalan pengganti *<input v-model="actionForm.reference" class="field w-full mt-1" /></label>
        </template>
        <label class="block text-sm">Catatan tindakan *<textarea v-model="actionForm.notes" class="field w-full mt-1" rows="3"></textarea></label>
        <StockEvidenceCapture v-if="actionForm.action !== 'approve'" v-model="actionEvidence" :warehouse-id="actionForm.action === 'receive_return' ? actionCase.return_warehouse_id : actionCase.warehouse_id" :label="actionForm.action === 'refund' ? 'Foto bukti dana diterima' : 'Foto bukti pelaksanaan'" />
      </div>
      <template #footer><AppButton variant="secondary" @click="showAction = false">Batal</AppButton><AppButton :loading="saving" :disabled="actionForm.action !== 'approve' && !actionEvidence?.id" @click="saveAction">Konfirmasi {{ actionCase ? actionLabel(actionCase, actionForm.action) : '' }}</AppButton></template>
    </AppModal>
  </div>
</template>
<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { stockQualityApi } from '@/api/stockQuality'
import { stockItemsApi } from '@/api/warehouse'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { safeUrl } from '@/utils/safeUrl'
import { formatRupiah as money } from '@/utils/format'
import { stockUnitRatio, preferredStockUnit } from '@/utils/stockUnits'
import { QUALITY_REASONS, QUALITY_STATUSES, QUALITY_ACTIONS, QUALITY_DISPOSITIONS } from '@/utils/stockQuality'
import StockEvidenceCapture from '@/components/StockEvidenceCapture.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
const auth = useAuthStore(), toast = useToastStore()
const has = p => auth.hasPermission(p)
const can = (kind, stage) => has(`stockquality.${kind}.${stage}`) && (kind !== 'central' || auth.scopeType !== 'specific')
const canReport = computed(() => can('central', 'report') || can('outlet', 'report'))
const lookups = ref({ warehouses: [], return_warehouses: [], vendors: [] }), items = ref([])
const warehouse = ref(''), status = ref(''), search = ref(''), page = ref(1), total = ref(0), rows = ref([]), loading = ref(false), error = ref(''), saving = ref(false)
const showReport = ref(false), report = ref({}), reportEvidence = ref(null), batches = ref([]), batchLoading = ref(false), batchError = ref('')
const showDetail = ref(false), detail = ref(null), showAction = ref(false), actionCase = ref(null), actionForm = ref({}), actionEvidence = ref(null)
const reportWarehouses = computed(() => lookups.value.warehouses.filter(w => can(w.type, 'report')))
const selectedItem = computed(() => items.value.find(i => i.id === report.value.item_id))
const units = computed(() => selectedItem.value ? [...new Set([selectedItem.value.base_unit, selectedItem.value.dist_unit].filter(Boolean))] : [])
const reportReasons = computed(() => Object.fromEntries(Object.entries(QUALITY_REASONS).filter(([key]) => key !== 'return' || lookups.value.warehouses.find(w => w.id === report.value.warehouse_id)?.type === 'central')))
const date = v => v ? new Date(v).toLocaleString('id-ID') : '—'
let seq = 0, batchSeq = 0, timer
async function load() {
  const request = ++seq; loading.value = true; error.value = ''
  try { const data = await stockQualityApi.list({ warehouse_id: warehouse.value, status: status.value, search: search.value, page: page.value, limit: 20 }); if (request === seq) { rows.value = data.rows || []; total.value = data.total || 0 } }
  catch (e) { if (request === seq) { rows.value = []; error.value = e.message } }
  finally { if (request === seq) loading.value = false }
}
watch([warehouse, status], () => { if (page.value !== 1) page.value = 1; else load() })
watch(page, load)
watch(search, () => { clearTimeout(timer); timer = setTimeout(() => { if (page.value !== 1) page.value = 1; else load() }, 300) })
onUnmounted(() => { clearTimeout(timer); seq++; batchSeq++ })
onMounted(async () => {
  load()
  try { lookups.value = await stockQualityApi.lookups(); if (canReport.value) { const data = await stockItemsApi.list({ active_only: true, limit: 1000 }); items.value = Array.isArray(data) ? data : data.data || [] } }
  catch (e) { error.value = e.message }
})
function openReport() { report.value = { warehouse_id: reportWarehouses.value.find(w => w.id === warehouse.value)?.id || '', item_id: '', batch_id: '', qty: 1, unit_used: '', reason: 'reject', notes: '' }; reportEvidence.value = null; showReport.value = true }
watch(() => [report.value.warehouse_id, report.value.item_id], async () => {
  const request = ++batchSeq; batches.value = []; report.value.batch_id = ''; batchError.value = ''; batchLoading.value = false
  const w = lookups.value.warehouses.find(w => w.id === report.value.warehouse_id)
  if (report.value.reason === 'return' && w?.type !== 'central') report.value.reason = 'reject'
  if (selectedItem.value) report.value.unit_used = preferredStockUnit(selectedItem.value, w?.type)
  if (!report.value.warehouse_id || !report.value.item_id) return
  batchLoading.value = true
  try { const data = await stockQualityApi.batches(report.value.warehouse_id, report.value.item_id); if (request === batchSeq) batches.value = data || [] }
  catch (e) { if (request === batchSeq) batchError.value = e.message }
  finally { if (request === batchSeq) batchLoading.value = false }
})
async function saveReport() {
  if (!report.value.warehouse_id || !report.value.item_id || !report.value.batch_id || !report.value.notes?.trim() || !reportEvidence.value?.id) return toast.error('Lengkapi gudang, barang, batch, catatan, dan foto')
  saving.value = true
  try { await stockQualityApi.create({ ...report.value, evidence_id: reportEvidence.value.id }); showReport.value = false; toast.success('Barang dikarantina; menunggu keputusan supervisor'); await load() }
  catch (e) { toast.error(e.message) } finally { saving.value = false }
}
async function openDetail(id) { detail.value = null; showDetail.value = true; try { detail.value = await stockQualityApi.get(id) } catch (e) { showDetail.value = false; toast.error(e.message) } }
function actions(row) {
  if (row.status === 'pending' && row.created_by === auth.currentAdmin?.id) return []
  if (row.status === 'pending' && can(row.warehouse_type, 'approve')) return ['approve', 'release']
  if (row.status === 'approved' && can(row.warehouse_type, 'execute')) return ['execute']
  if (row.status === 'in_transit' && row.disposition === 'return_warehouse' && can('central', 'receive')) return ['receive_return']
  if (row.status === 'in_transit' && row.disposition === 'return_supplier') {
    if (row.settlement === 'refund' && has('stockquality.refund') && auth.scopeType !== 'specific') return ['refund']
    if (row.settlement === 'replacement' && can('central', 'receive')) return ['replacement']
  }
  return []
}
const actionLabel = (row, action) => action === 'execute' ? (row.disposition === 'dispose' ? 'Catat pemusnahan' : 'Kirim retur') : QUALITY_ACTIONS[action]
function openAction(row, action) { actionCase.value = row; actionEvidence.value = null; actionForm.value = { action, version: row.version, notes: '', disposition: 'dispose', settlement: 'replacement', vendor_id: row.vendor_id || '', reference: '', return_warehouse_id: lookups.value.return_warehouses.some(w => w.id === row.return_warehouse_id) ? row.return_warehouse_id : '', qty_base: Number((row.qty_base - row.settled_qty_base).toFixed(4)), amount: 0, expiry_date: '' }; showAction.value = true }
async function saveAction() {
  if (!actionForm.value.notes?.trim()) return toast.error('Catatan tindakan wajib diisi')
  saving.value = true
  try { await stockQualityApi.act(actionCase.value.id, { ...actionForm.value, evidence_id: actionEvidence.value?.id || '' }); showAction.value = false; toast.success('Tindakan tersimpan beserta riwayatnya'); await load() }
  catch (e) { toast.error(e.message); if (e.status === 409) { showAction.value = false; await load() } } finally { saving.value = false }
}
</script>
<style scoped>
.field{border:1px solid #d1d5db;border-radius:.5rem;padding:.55rem .75rem;font-size:.875rem;background:white;min-width:0;max-width:100%}.label{display:block;font-size:.875rem;margin-bottom:.3rem}
</style>
