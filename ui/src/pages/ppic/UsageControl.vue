<template>
  <div class="space-y-5">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div class="max-w-3xl">
        <h1 class="text-xl font-bold text-gray-900">Kontrol Pemakaian Bahan</h1>
        <p class="mt-1 text-sm text-gray-500">Pantau bahan per gudang, tetapkan batas pemakaian dan waste, lalu periksa selisih dengan hasil opname.</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <AppButton v-if="canExport" variant="secondary" :loading="exporting" :disabled="!report || loading" @click="download">Export Excel</AppButton>
        <AppButton v-if="canUpdate" :disabled="!lookups.warehouses.length || loadingLookups" @click="openEditor()">Atur Batas</AppButton>
      </div>
    </header>

    <AppAlert type="error" :message="error" />
    <AppCard>
      <form class="grid items-end gap-3 sm:grid-cols-2 xl:grid-cols-6" @submit.prevent="applyFilters">
        <AppSelect v-model="filters.warehouse_id" label="Gudang" :options="warehouseOptions" :disabled="loadingLookups" />
        <AppInput v-model="filters.date_from" label="Dari tanggal" type="date" required />
        <AppInput v-model="filters.date_to" label="Sampai tanggal" type="date" required />
        <AppSelect v-model="filters.status" label="Status kontrol" :options="statusOptions" />
        <AppInput v-model.trim="filters.search" label="Cari bahan" placeholder="Nama atau kode bahan" />
        <AppButton type="submit" :loading="loading">Tampilkan</AppButton>
      </form>
      <p class="mt-3 text-xs text-gray-500">Periode mengikuti tanggal pencatatan stok. Batas yang ditampilkan adalah kebijakan PPIC yang aktif sekarang.</p>
    </AppCard>

    <div v-if="loading" role="status" class="py-8 text-center text-sm text-gray-500"><AppSpinner class="mx-auto mb-2" />Memuat kontrol bahan…</div>
    <template v-else-if="report">
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="metric"><p>Melebihi batas</p><strong class="text-red-700">{{ report.summary.over_limit_count }}</strong><small>bahan–gudang untuk diperiksa</small></div>
        <div class="metric"><p>Batas belum ditetapkan</p><strong>{{ report.summary.unset_count }}</strong><small>PPIC perlu menentukan toleransi</small></div>
        <div class="metric"><p>Perlu cek fisik</p><strong class="text-amber-700">{{ report.summary.needs_count }}</strong><small>lengkapi dengan stock opname</small></div>
        <div class="metric"><p>Waste tercatat</p><strong>{{ money(report.summary.waste_value) }}</strong><small>Koreksi stok negatif: {{ money(report.summary.shortage_value) }}</small></div>
      </div>
      <AppAlert v-if="report.summary.pending_deductions" type="warning" :message="`${report.summary.pending_deductions} pencatatan pemakaian POS belum berhasil. Lengkapi data sebelum menyimpulkan pemakaian bahan.`" />
      <section class="rounded-xl border border-teal-200 bg-teal-50 px-4 py-3 text-sm text-teal-900">
        <p class="font-semibold">Penjualan memberi acuan resep; opname memeriksa stok fisik.</p>
        <p class="mt-1 text-xs leading-relaxed">Pemakaian dari POS dipotong otomatis menurut resep. Angka tersebut belum membuktikan penggunaan fisik di dapur. Waste diperiksa terpisah, termasuk void yang sudah menjadi waste. Titipan pesanan tidak menjadi pemakaian.</p>
      </section>

      <AppCard :padding="false">
        <p v-if="!report.rows.length" class="p-8 text-center text-sm text-gray-500">Tidak ada bahan untuk filter ini.</p>
        <template v-else>
          <div class="hidden overflow-x-auto lg:block">
            <table class="control-table">
              <thead><tr><th>Bahan / gudang</th><th class="number">Stok saat ini</th><th class="number">Acuan pemakaian</th><th class="number">Pakai + koreksi negatif</th><th class="number">Waste</th><th>Batas PPIC</th><th>Status / tindak lanjut</th><th></th></tr></thead>
              <tbody><tr v-for="row in report.rows" :key="rowKey(row)">
                <td><p class="font-semibold text-gray-900">{{ row.item_name }}</p><p class="text-xs text-gray-500">{{ row.item_code }} · {{ row.warehouse_name }}</p><p class="text-xs text-gray-400">{{ row.base_unit }}</p></td>
                <td class="number">{{ qty(row.stock_qty) }}</td>
                <td class="number">{{ qty(row.reference_qty) }}</td>
                <td class="number"><span :class="row.excess_usage_qty > 0 ? 'font-semibold text-amber-700' : ''">{{ qty(row.evaluated_usage_qty) }}</span><small v-if="row.excess_usage_qty > 0">Tambahan {{ qty(row.excess_usage_qty) }}</small></td>
                <td class="number">{{ qty(row.waste_qty) }}<small>{{ percent(row.waste_pct) }} dari acuan</small><small v-if="row.void_waste_qty">Termasuk void {{ qty(row.void_waste_qty) }}</small></td>
                <td><template v-if="row.policy"><p>Pakai +{{ qty(row.policy.usage_tolerance_pct) }}%</p><p>Waste {{ qty(row.policy.waste_tolerance_pct) }}%</p></template><span v-else class="text-gray-400">Belum ditetapkan</span></td>
                <td><span class="status-badge" :class="usageStatus(row.status).tone">{{ usageStatus(row.status).label }}</span><p class="mt-1 max-w-60 text-xs text-gray-500">{{ row.has_stocktake ? 'Ada opname dalam periode' : 'Belum ada opname dalam periode' }}</p></td>
                <td><div class="flex flex-col items-end gap-2"><button type="button" class="text-link" @click="openDetail(row)">Rincian</button><button v-if="canUpdate" type="button" class="text-link" @click="openEditor(row)">{{ row.policy ? 'Ubah Batas' : 'Atur Batas' }}</button></div></td>
              </tr></tbody>
            </table>
          </div>
          <ul class="divide-y divide-gray-100 lg:hidden">
            <li v-for="row in report.rows" :key="rowKey(row)" class="space-y-3 p-4">
              <div class="flex flex-wrap items-start justify-between gap-2"><div><p class="font-semibold">{{ row.item_name }}</p><p class="text-xs text-gray-500">{{ row.warehouse_name }} · {{ row.base_unit }}</p></div><span class="status-badge" :class="usageStatus(row.status).tone">{{ usageStatus(row.status).label }}</span></div>
              <dl class="grid grid-cols-2 gap-3 text-sm"><div><dt>Acuan pemakaian</dt><dd>{{ qty(row.reference_qty) }}</dd></div><div><dt>Pakai + koreksi negatif</dt><dd>{{ qty(row.evaluated_usage_qty) }}</dd></div><div><dt>Waste</dt><dd>{{ qty(row.waste_qty) }} · {{ percent(row.waste_pct) }}</dd></div><div><dt>Stok saat ini</dt><dd>{{ qty(row.stock_qty) }}</dd></div></dl>
              <p class="text-xs text-gray-500">{{ row.policy ? `Toleransi pakai ${qty(row.policy.usage_tolerance_pct)}% · waste ${qty(row.policy.waste_tolerance_pct)}%` : 'Batas belum ditetapkan oleh PPIC.' }}</p>
              <div class="flex flex-wrap gap-4"><button type="button" class="text-link" @click="openDetail(row)">Rincian</button><button v-if="canUpdate" type="button" class="text-link" @click="openEditor(row)">{{ row.policy ? 'Ubah Batas' : 'Atur Batas' }}</button></div>
            </li>
          </ul>
        </template>
        <div class="px-4 pb-4"><AppPagination :page="page" :total="report.total_rows" :per-page="20" @change="changePage" /></div>
      </AppCard>
      <details class="rounded-xl border border-gray-200 bg-white p-4 text-xs text-gray-500"><summary class="cursor-pointer font-semibold text-gray-700">Cara membaca hasil kontrol</summary><ul class="mt-3 list-disc space-y-2 pl-5"><li v-for="note in report.notes" :key="note">{{ note }}</li><li>Indikator menunjukkan catatan yang perlu diperiksa. Cocokkan dengan resep F&amp;B, bukti waste, dan hitungan fisik sebelum mengambil tindakan.</li></ul></details>
    </template>

    <AppModal v-model="editorOpen" title="Atur Batas Pemakaian" size="lg">
      <form id="ppic-usage-policy-form" class="modal-form-grid" @submit.prevent="savePolicy">
        <AppAlert class="modal-span-full" type="error" :message="editorError" />
        <AppSelect v-model="form.warehouse_id" label="Gudang kebijakan" :options="policyWarehouseOptions" placeholder="Pilih gudang" required :disabled="editingFixed || saving" @change="loadEditorPolicy" />
        <AppSelect v-model="form.item_id" label="Bahan" :options="itemOptions" placeholder="Pilih bahan" required :disabled="editingFixed || saving" @change="loadEditorPolicy" />
        <p class="modal-span-full text-xs leading-relaxed text-gray-500">Batas dihitung dari acuan pemakaian penjualan dan produksi. Isi nol untuk tanpa toleransi. Jika belum ada acuan pemakaian, hasil perlu ditinjau dan tidak otomatis dinyatakan wajar.</p>
        <AppInput v-model.number="form.usage_tolerance_pct" label="Toleransi pemakaian (%)" type="number" min="0" max="100" step="0.01" placeholder="Belum ditetapkan" required :disabled="policyLoading || saving" />
        <AppInput v-model.number="form.waste_tolerance_pct" label="Toleransi waste (%)" type="number" min="0" max="100" step="0.01" placeholder="Belum ditetapkan" required :disabled="policyLoading || saving" />
        <label class="modal-span-full text-sm font-medium text-gray-700">Alasan penetapan / perubahan<textarea v-model.trim="form.reason" class="field mt-1" rows="2" maxlength="1000" required :disabled="saving" placeholder="Dasar evaluasi PPIC, hasil uji porsi, atau temuan opname" /></label>
        <p v-if="policyLoading" class="modal-span-full text-xs text-gray-500">Memuat batas yang berlaku…</p>
        <p v-if="form.version" class="modal-span-full text-xs text-gray-500">Mengubah revisi {{ form.version }}. Batas sebelumnya tetap tersimpan dalam riwayat.</p>
      </form>
      <template #footer><AppButton variant="secondary" :disabled="saving" @click="editorOpen = false">Batal</AppButton><AppButton type="submit" form="ppic-usage-policy-form" :loading="saving" :disabled="policyLoading || !canUpdate">Simpan Batas</AppButton></template>
    </AppModal>

    <AppModal v-model="detailOpen" :title="`Rincian Kontrol — ${active?.item_name || ''}`" size="2xl">
      <div v-if="active" class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-2"><p class="text-sm text-gray-500">{{ active.warehouse_name }} · semua kuantitas dalam {{ active.base_unit }}</p><span class="status-badge" :class="usageStatus(active.status).tone">{{ usageStatus(active.status).label }}</span></div>
        <ul v-if="active.flags?.length" class="space-y-1 rounded-lg bg-amber-50 p-3 text-sm text-amber-800"><li v-for="flag in active.flags" :key="flag">{{ usageFlagLabel(flag) }}</li></ul>
        <div class="modal-form-grid">
          <section class="rounded-xl border border-gray-200 p-4"><h3 class="mb-3 font-semibold">Pemakaian dan batas</h3><dl class="detail-list"><dt>Penjualan menurut resep transaksi</dt><dd>{{ qty(active.sale_qty) }}</dd><dt>Produksi tercatat</dt><dd>{{ qty(active.production_qty) }}</dd><dt>Acuan bahan produksi</dt><dd>{{ qty(active.production_reference_qty) }}</dd><dt>Pemotongan bahan</dt><dd>{{ qty(active.breakdown_qty) }}</dd><dt>Pemakaian lain</dt><dd>{{ qty(active.other_usage_qty) }}</dd><dt>Acuan pemakaian</dt><dd>{{ qty(active.reference_qty) }}</dd><dt>Pakai + koreksi negatif</dt><dd>{{ qty(active.evaluated_usage_qty) }}</dd><dt>Batas pemakaian PPIC</dt><dd>{{ qty(active.usage_limit_qty) }}</dd><dt>Selisih terhadap acuan</dt><dd>{{ percent(active.usage_variance_pct) }}</dd></dl></section>
          <section class="rounded-xl border border-gray-200 p-4"><h3 class="mb-3 font-semibold">Waste dan stok fisik</h3><dl class="detail-list"><dt>Waste total</dt><dd>{{ qty(active.waste_qty) }}</dd><dt>Termasuk waste dari void</dt><dd>{{ qty(active.void_waste_qty) }}</dd><dt>Batas waste PPIC</dt><dd>{{ qty(active.waste_limit_qty) }}</dd><dt>Nilai waste</dt><dd>{{ money(active.waste_value) }}</dd><dt>Kekurangan hasil opname</dt><dd>{{ qty(active.stocktake_shortage_qty) }}</dd><dt>Kelebihan hasil opname</dt><dd>{{ qty(active.stocktake_surplus_qty) }}</dd><dt>Koreksi negatif lain</dt><dd>{{ qty(active.adjustment_shortage_qty) }}</dd></dl><p class="mt-3 text-xs text-gray-500">{{ active.stocktake_at ? `Opname terakhir: ${active.stocktake_number} · ${dateTime(active.stocktake_at)}` : 'Belum ada opname yang disetujui untuk bahan ini.' }}</p><p class="mt-1 text-xs text-gray-500">Kelebihan stok tidak menutup kekurangan. Selisih opname dicatat saat persetujuan dan dapat berasal dari periode hitung sebelumnya.</p></section>
        </div>
        <p v-if="active.undocumented_waste_qty" class="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">Waste total sudah mencakup {{ qty(active.undocumented_waste_qty) }} {{ active.base_unit }} pengeluaran stok tanpa dokumen waste yang sesuai. Lengkapi bukti dan alasan pengeluarannya.</p>
        <p class="rounded-lg bg-gray-50 p-3 text-xs text-gray-500">Transfer keluar {{ qty(active.transfer_out_qty) }} dan karantina {{ qty(active.quality_hold_qty) }} ditampilkan sebagai informasi perpindahan; keduanya tidak dihitung sebagai pemakaian.</p>
        <section><h3 class="mb-2 font-semibold">Riwayat Batas PPIC</h3><AppAlert type="error" :message="historyError" /><p v-if="historyLoading" class="text-sm text-gray-500">Memuat riwayat…</p><p v-else-if="!history.length && !historyError" class="text-sm text-gray-500">Belum ada batas yang ditetapkan.</p><ol v-else class="grid gap-3 md:grid-cols-2"><li v-for="entry in history" :key="entry.version" class="rounded-lg border border-gray-200 p-3 text-sm"><p class="font-semibold">Revisi {{ entry.version }} · Pakai +{{ qty(entry.usage_tolerance_pct) }}% · Waste {{ qty(entry.waste_tolerance_pct) }}%</p><p class="mt-1 whitespace-pre-wrap break-words">{{ entry.reason }}</p><p class="mt-1 text-xs text-gray-500">{{ entry.updated_by }} · {{ dateTime(entry.updated_at) }}</p></li></ol></section>
      </div>
      <template #footer><RouterLink v-if="canCount" class="text-link mr-auto" to="/ppic/opname">Buka Stock Opname</RouterLink><AppButton variant="secondary" @click="detailOpen = false">Tutup</AppButton><AppButton v-if="canUpdate" @click="detailOpen = false; openEditor(active)">{{ active?.policy ? 'Ubah Batas' : 'Atur Batas' }}</AppButton></template>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ppicApi } from '@/api/ppic'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { formatDateTime, formatRupiah, todayDateString } from '@/utils/format'
import { usageStatuses, usageStatus, usageFlagLabel, usageQty as qty } from '@/utils/ppicUsageControl'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSelect from '@/components/ui/AppSelect.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'

const auth = useAuthStore(), toast = useToastStore()
const canUpdate = computed(() => auth.hasPermission('ppic.control.update'))
const canExport = computed(() => auth.hasPermission('ppic.control.export'))
const canCount = computed(() => auth.hasPermission('ppic.opname.view'))
const end = todayDateString()
const start = new Date(`${end}T12:00:00Z`); start.setUTCDate(start.getUTCDate() - 29)
const filters = reactive({ date_from: start.toISOString().slice(0, 10), date_to: end, warehouse_id: '', search: '', status: '' })
const applied = ref({ ...filters }), page = ref(1), report = ref(null)
const loading = ref(false), exporting = ref(false), error = ref(''), loadingLookups = ref(false)
const lookups = ref({ warehouses: [], items: [] })
const warehouseOptions = computed(() => [{ value: '', label: 'Semua gudang yang diizinkan' }, ...policyWarehouseOptions.value])
const policyWarehouseOptions = computed(() => lookups.value.warehouses.map(w => ({ value: w.id, label: `${w.name}${w.outlet_name ? ` · ${w.outlet_name}` : ''}` })))
const itemOptions = computed(() => lookups.value.items.map(i => ({ value: i.id, label: `${i.code} · ${i.name} (${i.base_unit})` })))
const statusOptions = [{ value: '', label: 'Semua status' }, ...Object.entries(usageStatuses).map(([value, info]) => ({ value, label: info.label }))]
const money = value => formatRupiah(value || 0)
const dateTime = value => value ? formatDateTime(value) : '—'
const percent = value => value == null ? '—' : `${qty(value)}%`
const rowKey = row => `${row.warehouse_id}:${row.item_id}`
let loadSequence = 0

async function load(params = { ...filters }, requestedPage = 1) {
  const sequence = ++loadSequence
  loading.value = true; error.value = ''; report.value = null
  try {
    if (!params.date_from || !params.date_to || params.date_from > params.date_to) throw new Error('Periksa rentang tanggal laporan.')
    const result = await ppicApi.getUsageControl({ ...params, page: requestedPage, limit: 20 })
    if (sequence !== loadSequence) return
    report.value = result; applied.value = { ...params }; page.value = requestedPage
  } catch (e) {
    if (sequence === loadSequence) error.value = e?.message || 'Gagal memuat kontrol bahan.'
  } finally { if (sequence === loadSequence) loading.value = false }
}
function applyFilters() { load({ ...filters }, 1) }
function changePage(value) { load(applied.value, value) }
onMounted(async () => {
  loadingLookups.value = true
  try { lookups.value = await ppicApi.getUsageLookups() }
  catch (e) { error.value = e?.message || 'Gagal memuat gudang dan bahan.'; return }
  finally { loadingLookups.value = false }
  await load()
})

const editorOpen = ref(false), editorError = ref(''), editingFixed = ref(false), policyLoading = ref(false), saving = ref(false)
const blankPolicy = () => ({ warehouse_id: '', item_id: '', usage_tolerance_pct: '', waste_tolerance_pct: '', version: 0, reason: '' })
const form = reactive(blankPolicy())
let policySequence = 0
function setPolicy(policy) {
  form.usage_tolerance_pct = policy?.usage_tolerance_pct ?? ''
  form.waste_tolerance_pct = policy?.waste_tolerance_pct ?? ''
  form.version = policy?.version || 0
  form.reason = ''
}
function openEditor(row) {
  if (!canUpdate.value) return
  ++policySequence; policyLoading.value = false; editorError.value = ''
  editingFixed.value = !!row
  Object.assign(form, blankPolicy(), { warehouse_id: row?.warehouse_id || filters.warehouse_id, item_id: row?.item_id || '' })
  setPolicy(row?.policy)
  editorOpen.value = true
}
async function loadEditorPolicy() {
  const sequence = ++policySequence
  setPolicy(null); editorError.value = ''; policyLoading.value = false
  if (!form.warehouse_id || !form.item_id) return
  policyLoading.value = true
  try {
    const entries = await ppicApi.getUsagePolicyHistory(form.warehouse_id, form.item_id)
    if (sequence === policySequence) setPolicy(entries[0])
  } catch (e) { if (sequence === policySequence) editorError.value = e?.message || 'Gagal memuat batas yang berlaku.' }
  finally { if (sequence === policySequence) policyLoading.value = false }
}
async function savePolicy() {
  if (!canUpdate.value || saving.value || policyLoading.value) return
  editorError.value = ''
  if (!form.warehouse_id || !form.item_id || !form.reason.trim() || [form.usage_tolerance_pct, form.waste_tolerance_pct].some(v => v === '' || !Number.isFinite(Number(v)) || Number(v) < 0 || Number(v) > 100)) {
    editorError.value = 'Lengkapi bahan, gudang, toleransi 0–100%, dan alasan.'; return
  }
  saving.value = true
  try {
    await ppicApi.saveUsagePolicy({ ...form, usage_tolerance_pct: Number(form.usage_tolerance_pct), waste_tolerance_pct: Number(form.waste_tolerance_pct) })
    editorOpen.value = false; toast.success('Batas PPIC tersimpan beserta riwayatnya.')
    await load(applied.value, page.value)
  } catch (e) { editorError.value = e?.message || 'Gagal menyimpan batas. Muat ulang jika kebijakan sudah berubah.' }
  finally { saving.value = false }
}

const active = ref(null), detailOpen = ref(false), history = ref([]), historyLoading = ref(false), historyError = ref('')
let historySequence = 0
async function openDetail(row) {
  active.value = row; detailOpen.value = true; history.value = []; historyError.value = ''
  historyLoading.value = true
  const sequence = ++historySequence
  try {
    const result = await ppicApi.getUsagePolicyHistory(row.warehouse_id, row.item_id)
    if (sequence === historySequence) history.value = result
  } catch (e) { if (sequence === historySequence) historyError.value = e?.message || 'Gagal memuat riwayat batas.' }
  finally { if (sequence === historySequence) historyLoading.value = false }
}
async function download() {
  if (!canExport.value || !report.value) return
  exporting.value = true
  try {
    const blob = await ppicApi.exportUsageControl(applied.value)
    const url = URL.createObjectURL(blob), link = document.createElement('a')
    link.href = url; link.download = `Kontrol-Bahan-${applied.value.date_from}-${applied.value.date_to}.xlsx`
    document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e) { toast.error(e?.message || 'Gagal mengekspor kontrol bahan.') }
  finally { exporting.value = false }
}
</script>

<style scoped>
.metric { border: 1px solid #e5e7eb; border-radius: .75rem; background: white; padding: 1rem; min-width: 0; }
.metric p { font-size: .75rem; color: #6b7280; }
.metric strong { display: block; margin: .35rem 0; font-size: 1.4rem; overflow-wrap: anywhere; }
.metric small { display: block; font-size: .7rem; color: #6b7280; }
.control-table { width: 100%; min-width: 1080px; text-align: left; font-size: .8rem; }
.control-table th { padding: .85rem .75rem; font-size: .7rem; color: #6b7280; background: #f9fafb; }
.control-table td { padding: 1rem .75rem; border-top: 1px solid #f3f4f6; vertical-align: top; }
.control-table .number { text-align: right; font-variant-numeric: tabular-nums; }
.control-table small { display: block; margin-top: .2rem; font-size: .65rem; color: #6b7280; }
.status-badge { display: inline-block; max-width: 100%; border-radius: .5rem; padding: .3rem .55rem; font-size: .7rem; font-weight: 600; }
.danger { color: #b91c1c; background: #fee2e2; }.warning { color: #92400e; background: #fef3c7; }.neutral { color: #475569; background: #f1f5f9; }.success { color: #047857; background: #d1fae5; }
.text-link { font-size: .8rem; font-weight: 600; color: #047857; }.text-link:hover { text-decoration: underline; }
dt { color: #6b7280; font-size: .75rem; } dd { margin-top: .15rem; color: #111827; font-weight: 600; font-variant-numeric: tabular-nums; }
.detail-list { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .65rem 1rem; align-items: baseline; }
.detail-list dd { text-align: right; font-size: .85rem; }
.field { width: 100%; border: 1px solid #d1d5db; border-radius: .5rem; padding: .5rem .75rem; font-size: .875rem; font-weight: 400; }
</style>
