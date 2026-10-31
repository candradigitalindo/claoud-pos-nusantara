<template>
  <Teleport to="body">
    <div class="classification-overlay" @click.self="close" @keydown.esc.stop="close" @keydown.tab="trapFocus">
      <section ref="dialog" class="classification-dialog" role="dialog" aria-modal="true" aria-labelledby="classification-title" tabindex="-1">
        <header>
          <div><h2 id="classification-title">Analisis jenis produk</h2><p>{{ scopeLabel }} · Seluruh katalog aktif dalam cakupan ini</p></div>
          <button type="button" class="secondary" :disabled="applying" aria-label="Tutup analisis jenis" @click="close">Tutup</button>
        </header>
        <div class="classification-body" :aria-busy="loading">
          <p v-if="loading" role="status">Menganalisis seluruh katalog…</p>
          <div v-if="error" class="notice error" role="alert">{{ error }} <button v-if="!applying" type="button" class="text-button" @click="load">Muat ulang analisis</button></div>
          <p v-if="success" class="notice success" role="status">{{ success }}</p>
          <template v-if="report && !loading">
            <div class="classification-counts" aria-label="Ringkasan seluruh katalog">
              <div><strong>{{ report.totals.total }}</strong><span>Total produk</span></div>
              <div><strong>{{ report.totals.existing }}</strong><span>Sudah ditentukan</span></div>
              <div><strong>{{ report.totals.suggested }}</strong><span>Saran siap diterapkan</span></div>
              <div class="review-count"><strong>{{ report.totals.needs_review }}</strong><span>Perlu ditinjau</span></div>
            </div>
            <p class="kind-counts"><span v-for="kind in PRODUCT_KINDS" :key="kind.value">{{ kind.label }}: <strong>{{ report.totals[kind.value] || 0 }}</strong></span></p>
            <p class="classification-help">Saran mengikuti bukti katalog dan ketentuan outlet. Produk yang belum masuk Makanan, Minuman, atau Aktivitas dikelompokkan sebagai Lainnya. Jenis yang sudah ditentukan tetap dipertahankan; data jenis lama yang tidak valid perlu ditinjau. Jumlah jenis mencakup jenis saat ini dan saran.</p>
            <p v-if="canUpdate && eligible.length > APPLY_LIMIT" class="notice selection-limit" role="status">Maksimal 1.000 saran per penerapan. Pilihan awal berisi 1.000 saran pertama sesuai urutan laporan; {{ eligible.length - APPLY_LIMIT }} saran sisanya tidak ikut dipilih. Anda dapat mengganti pilihan. Setelah diterapkan, analisis dimuat ulang untuk meninjau saran berikutnya.</p>
            <div class="classification-filters">
              <input v-model="search" type="search" aria-label="Cari produk dalam analisis" placeholder="Cari nama, kategori, atau outlet…" />
              <select v-model="kindFilter" aria-label="Filter jenis dalam analisis">
                <option value="">Semua jenis</option>
                <option v-for="kind in PRODUCT_KINDS" :key="kind.value" :value="kind.value">{{ kind.label }}</option>
                <option value="unclassified">Belum ditentukan</option>
              </select>
              <select v-model="statusFilter" aria-label="Filter status analisis">
                <option value="">Semua status</option><option value="suggested">Saran siap diterapkan</option><option value="needs_review">Perlu ditinjau</option><option value="existing">Sudah ditentukan</option>
              </select>
            </div>
            <div class="classification-selection">
              <span>{{ filtered.length }} hasil · {{ selectedRows.length }} dipilih dari seluruh laporan</span>
              <button v-if="canUpdate" class="text-button" type="button" :disabled="applying || stale" @click="selectEligible">{{ eligible.length > APPLY_LIMIT ? 'Pilih 1.000 saran pertama' : `Pilih semua saran layak (${eligible.length})` }}</button>
              <button v-if="canUpdate" class="text-button" type="button" :disabled="applying || stale" @click="selected = new Set()">Kosongkan pilihan</button>
            </div>
            <div class="classification-rows" role="list" aria-label="Hasil analisis produk">
              <article v-for="row in visibleRows" :key="row.product_id" class="classification-row" :class="{ uncertain: row.status === 'needs_review' }" role="listitem" :data-product-id="row.product_id">
                <div class="row-selection">
                  <input v-if="canUpdate && canApplyClassification(row)" type="checkbox" :checked="selected.has(row.product_id)" :disabled="applying || stale || (selectedRows.length >= APPLY_LIMIT && !selected.has(row.product_id))" :aria-label="`Pilih ${row.name}`" @change="toggleRow(row.product_id, $event.target.checked)" />
                </div>
                <div class="row-product"><strong>{{ row.name }}</strong><span>{{ row.category_name || 'Tanpa kategori' }} · {{ row.outlet_name || row.outlet_id }}</span></div>
                <div class="row-result"><ProductKindBadge :kind="row.current_kind || row.suggested_kind || ''" /><span :class="{ 'review-label': row.status === 'needs_review' }">{{ statusLabel(row) }}</span></div>
                <p class="row-reason">{{ row.reason || 'Belum ada bukti yang cukup.' }}</p>
              </article>
              <p v-if="!filtered.length" class="empty">Tidak ada produk yang sesuai filter.</p>
            </div>
            <AppPagination v-model="page" :total="filtered.length" :per-page="PAGE_SIZE" />
          </template>
        </div>
        <footer>
          <div v-if="confirming" class="classification-confirm" role="group" aria-label="Konfirmasi penerapan jenis">
            <p>Terapkan jenis untuk <strong>{{ selectedRows.length }} produk</strong> di {{ scopeLabel }}? Pilihan mencakup seluruh laporan, termasuk baris di halaman lain.</p>
            <div><button type="button" class="secondary" :disabled="applying" @click="confirming = false">Batal</button><button type="button" class="primary" :disabled="applying || stale || !selectedRows.length || !canUpdate" @click="apply">{{ applying ? 'Menerapkan…' : 'Ya, terapkan jenis' }}</button></div>
          </div>
          <template v-else>
            <p>{{ canUpdate ? 'Saran hanya diterapkan untuk produk yang belum memiliki jenis. Lainnya mencakup produk di luar tiga kelompok utama.' : 'Akses lihat saja. Izin ubah produk diperlukan untuk menerapkan saran.' }}</p>
            <button v-if="canUpdate" type="button" class="primary" :disabled="loading || applying || stale || !selectedRows.length" @click="confirming = true">Terapkan {{ selectedRows.length }} saran</button>
          </template>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { productsApi } from '@/api/products.js'
import { useAuthStore } from '@/stores/auth.js'
import { PRODUCT_KINDS, canApplyClassification } from '@/utils/productKinds.js'
import AppPagination from '@/components/ui/AppPagination.vue'
import ProductKindBadge from '@/components/ProductKindBadge.vue'

const props = defineProps({ outletId: { type: String, default: '' }, scopeLabel: { type: String, default: 'Semua outlet yang dapat diakses' } })
const emit = defineEmits(['close', 'applied'])
const auth = useAuthStore()
const canUpdate = computed(() => auth.hasPermission('products.update'))
const report = ref(null), loading = ref(false), applying = ref(false), error = ref(''), success = ref(''), stale = ref(false)
const search = ref(''), kindFilter = ref(''), statusFilter = ref(''), page = ref(1), selected = ref(new Set()), confirming = ref(false), dialog = ref(null)
const PAGE_SIZE = 40
const APPLY_LIMIT = 1000
let request = 0, previousFocus, previousOverflow
const eligible = computed(() => (report.value?.rows || []).filter(canApplyClassification))
const selectedRows = computed(() => eligible.value.filter(row => selected.value.has(row.product_id)))
const filtered = computed(() => (report.value?.rows || []).filter(row => {
  const kind = row.current_kind || row.suggested_kind || 'unclassified'
  const text = [row.name, row.category_name, row.outlet_name].join(' ').toLocaleLowerCase('id')
  return (!kindFilter.value || kind === kindFilter.value) && (!statusFilter.value || row.status === statusFilter.value) && text.includes(search.value.trim().toLocaleLowerCase('id'))
}))
const visibleRows = computed(() => filtered.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))
watch([search, kindFilter, statusFilter], () => { page.value = 1 })
watch(() => props.outletId, () => { success.value = ''; load() })
function statusLabel(row) {
  if (row.status === 'existing') return 'Jenis saat ini · dipertahankan'
  if (row.status === 'suggested' && row.suggested_kind === 'other') return 'Saran · kelompok Lainnya'
  if (row.status === 'suggested' && row.confidence === 'high') return 'Saran · keyakinan tinggi'
  return 'Perlu ditinjau · bukti belum cukup'
}
function selectEligible() { selected.value = new Set(canUpdate.value ? eligible.value.slice(0, APPLY_LIMIT).map(row => row.product_id) : []) }
function toggleRow(id, checked) {
  if (checked && selectedRows.value.length >= APPLY_LIMIT) return
  const next = new Set(selected.value); checked ? next.add(id) : next.delete(id); selected.value = next
}
async function load() {
  const current = ++request
  loading.value = true; report.value = null; selected.value = new Set(); confirming.value = false; error.value = ''; stale.value = false; page.value = 1
  try {
    const result = await productsApi.classification({ outlet_id: props.outletId || undefined })
    if (current !== request) return
    report.value = result
    selectEligible()
  } catch (err) { if (current === request) error.value = err?.message || 'Gagal menganalisis produk.' }
  finally { if (current === request) loading.value = false }
}
async function apply() {
  if (applying.value || !confirming.value || !canUpdate.value || stale.value || !selectedRows.value.length || selectedRows.value.length > APPLY_LIMIT) return
  applying.value = true; error.value = ''; success.value = ''
  const scope = props.outletId
  const items = selectedRows.value.map(({ product_id, fingerprint, suggested_kind }) => ({ product_id, fingerprint, suggested_kind }))
  try {
    const result = await productsApi.applyClassification({ items }, { outlet_id: scope || undefined })
    success.value = `${result.applied} produk berhasil ditentukan jenisnya.`
    emit('applied')
    await load()
  } catch (err) {
    confirming.value = false
    stale.value = err?.status === 409
    error.value = stale.value ? 'Katalog berubah sejak analisis dibuat. Tidak ada perubahan diterapkan. Muat ulang analisis lalu tinjau kembali pilihan.' : (err?.message || 'Gagal menerapkan saran jenis.')
  } finally { applying.value = false }
}
function close() { if (!applying.value) emit('close') }
function trapFocus(event) {
  const elements = [...dialog.value.querySelectorAll('button, input, select, [tabindex="0"]')].filter(el => !el.disabled && el.getClientRects().length)
  const first = elements[0], last = elements.at(-1)
  if (!first) { event.preventDefault(); dialog.value.focus(); return }
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
onMounted(async () => {
  previousFocus = document.activeElement; previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'
  load(); await nextTick(); dialog.value?.focus()
})
onBeforeUnmount(() => { request++; document.body.style.overflow = previousOverflow ?? ''; previousFocus?.focus?.({ preventScroll: true }) })
</script>

<style scoped>
.classification-overlay { position: fixed; inset: 0; z-index: 100; display: flex; align-items: center; justify-content: center; padding: 16px; background: #10271c88; }
.classification-dialog { width: 1080px; max-width: 100%; max-height: calc(100dvh - 32px); min-width: 0; display: flex; flex-direction: column; overflow: hidden; border-radius: 16px; background: white; box-shadow: 0 20px 80px #0004; color: #243b2f; }
header, footer { padding: 16px 20px; display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-shrink: 0; }
header { border-bottom: 1px solid #e5ebe7; } h2 { font-size: 1.1rem; font-weight: 750; } header p, footer p { font-size: .78rem; color: #617568; margin-top: 4px; }
.classification-body { overflow-y: auto; min-height: 0; padding: 18px 20px; overscroll-behavior: contain; }
footer { border-top: 1px solid #e5ebe7; background: #f8faf9; } footer p { margin: 0; max-width: 610px; }
button { cursor: pointer; } button:disabled { opacity: .45; cursor: not-allowed; } button:focus-visible, input:focus-visible, select:focus-visible { outline: 2px solid #34845b; outline-offset: 2px; }
.primary, .secondary { border: 1px solid #ccd9d1; border-radius: 8px; padding: 9px 12px; font-size: .8rem; font-weight: 650; white-space: nowrap; }
.primary { background: #185538; color: white; border-color: #185538; } .secondary { background: white; color: #345742; }
.classification-counts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.classification-counts > div { border-radius: 10px; background: #f0f5f2; padding: 12px; display: flex; flex-direction: column; gap: 4px; }
.classification-counts strong { font-size: 1.5rem; } .classification-counts span { font-size: .75rem; } .classification-counts .review-count { background: #fff4dc; color: #825b10; }
.kind-counts { display: flex; flex-wrap: wrap; gap: 16px; font-size: .8rem; margin: 14px 0 8px; }
.classification-help { font-size: .78rem; color: #64766a; line-height: 1.6; }
.classification-filters { display: flex; flex-wrap: wrap; gap: 8px; margin: 16px 0 10px; }
.classification-filters input, .classification-filters select { min-width: 0; padding: 9px 10px; border: 1px solid #ccd9d1; border-radius: 8px; background: white; font-size: .8rem; }
.classification-filters input { flex: 1 1 250px; } .classification-filters select { flex: 1 1 150px; }
.classification-selection { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin-bottom: 12px; font-size: .75rem; }
.classification-selection > span { margin-right: auto; } .text-button { font-size: .75rem; color: #246142; text-decoration: underline; }
.classification-rows { border: 1px solid #e1e8e3; border-radius: 10px; overflow: hidden; }
.classification-row { display: grid; grid-template-columns: 20px minmax(0, 1fr) 180px minmax(0, 1fr); gap: 12px; padding: 13px 12px; border-bottom: 1px solid #e6ece8; align-items: start; }
.classification-row:last-child { border-bottom: 0; } .classification-row.uncertain { background: #fffaf0; }
.row-selection { padding-top: 3px; } .row-selection input { accent-color: #185538; width: 16px; height: 16px; }
.row-product, .row-result { display: flex; flex-direction: column; align-items: flex-start; gap: 5px; min-width: 0; } .row-product strong { font-size: .82rem; overflow-wrap: anywhere; }
.row-product span, .row-result > span:last-child, .row-reason { font-size: .73rem; line-height: 1.5; color: #657469; overflow-wrap: anywhere; } .row-result .review-label { color: #8c6010; font-weight: 650; }
.empty { text-align: center; padding: 24px; color: #64766a; font-size: .85rem; }
.notice { border-radius: 8px; padding: 12px; margin-bottom: 14px; font-size: .8rem; line-height: 1.6; } .error { background: #fff0ed; color: #a33221; } .success { background: #eaf6ed; color: #235b35; }
.selection-limit { margin-top: 12px; background: #edf3fc; color: #315078; }
.classification-confirm { width: 100%; display: flex; align-items: center; gap: 16px; justify-content: space-between; } .classification-confirm > div { display: flex; gap: 8px; flex-shrink: 0; }
@media (max-width: 640px) {
  .classification-overlay { padding: 8px; } .classification-dialog { max-height: calc(100dvh - 16px); border-radius: 12px; }
  header, footer, .classification-body { padding: 12px; } header { align-items: start; } h2 { font-size: 1rem; }
  .classification-counts { grid-template-columns: repeat(2, minmax(0, 1fr)); } .classification-counts strong { font-size: 1.25rem; }
  .classification-row { grid-template-columns: 20px minmax(0, 1fr); gap: 8px; } .row-result, .row-reason { grid-column: 2; }
  .row-result { flex-direction: row; flex-wrap: wrap; align-items: center; } footer, .classification-confirm { align-items: stretch; flex-direction: column; gap: 10px; }
  .classification-confirm > div { flex-wrap: wrap; } .classification-confirm button { flex: 1; } footer .primary { width: 100%; }
}
</style>
