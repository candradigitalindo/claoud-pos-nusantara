<template>
  <Teleport to="body">
    <div class="recipe-picker-overlay" @click.self="emit('close')">
      <div ref="dialog" class="recipe-picker" role="dialog" aria-modal="true" aria-labelledby="recipe-picker-title" aria-describedby="recipe-picker-subtitle" tabindex="-1">
        <header class="picker-header">
          <div>
            <h2 id="recipe-picker-title">Pilih resep produk</h2>
            <p id="recipe-picker-subtitle">{{ productName || 'Produk baru' }} <span aria-hidden="true">·</span> {{ outletName }}</p>
          </div>
          <AppActionButton icon="close" label="Tutup pilihan resep" @click="emit('close')" />
        </header>

        <div class="picker-panels" :class="{ 'show-detail': mobileDetail }">
          <section class="recipe-catalog" aria-label="Daftar resep">
            <div class="catalog-tools">
              <label class="recipe-search">
                <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/></svg>
                <input ref="searchInput" v-model="query" type="search" placeholder="Cari nama resep…" aria-label="Cari nama resep" autocomplete="off" />
              </label>
              <div class="recipe-filters" role="group" aria-label="Kategori resep">
                <button v-for="filter in filters" :key="filter.id" type="button" :aria-pressed="kind === filter.id" @click="kind = filter.id">{{ filter.name }}</button>
              </div>
              <p class="catalog-count" aria-live="polite">{{ loading ? 'Memuat resep…' : `${filteredRecipes.length} resep tersedia` }}</p>
            </div>
            <div v-if="loading" class="picker-state" role="status">Memuat daftar resep…</div>
            <div v-else-if="error" class="picker-state" role="alert"><strong>Daftar resep belum dimuat</strong><p>{{ error }}</p><button type="button" class="picker-secondary" @click="emit('retry')">Coba lagi</button></div>
            <div v-else-if="!filteredRecipes.length" class="picker-state">
              <strong>{{ recipes.length ? 'Resep tidak ditemukan' : 'Belum ada resep tersedia' }}</strong>
              <p>{{ recipes.length ? 'Coba nama atau kategori lainnya.' : 'Resep perlu memiliki bahan dan akses untuk outlet ini. Produk tetap bisa disimpan tanpa resep.' }}</p>
              <button v-if="query || kind" type="button" class="picker-secondary" @click="query = ''; kind = ''">Reset pencarian</button>
            </div>
            <ul v-else class="recipe-list">
              <li v-for="recipe in filteredRecipes" :key="recipe.id">
                <button type="button" class="recipe-choice" :aria-pressed="activeId === recipe.id" aria-controls="recipe-picker-detail" @click="preview(recipe.id)">
                  <span class="choice-copy"><strong>{{ recipe.name }}</strong><span>{{ kindLabel(recipe.menu_kind) }} <span aria-hidden="true">·</span> {{ recipe.item_count }} bahan</span><small v-if="recipe.description">{{ recipe.description }}</small></span>
                  <svg v-if="draft.some(l => l.recipe_master_id === recipe.id)" class="choice-check" aria-label="Pilihan produk saat ini" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m5 12 4 4L19 6"/></svg>
                  <svg v-else class="choice-arrow" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="m9 5 7 7-7 7"/></svg>
                </button>
              </li>
            </ul>
          </section>

          <section id="recipe-picker-detail" ref="detailPanel" class="recipe-detail" aria-label="Detail resep" :aria-busy="detailLoading">
            <button type="button" class="back-to-list" @click="backToList"><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m14 5-7 7 7 7"/></svg>Daftar resep</button>
            <div v-if="detailLoading" class="picker-state" role="status">Memuat isi resep…</div>
            <div v-else-if="detailError" class="picker-state" role="alert"><strong>Detail resep belum dimuat</strong><p>{{ detailError }}</p><button type="button" class="picker-secondary" @click="loadDetail">Coba lagi</button></div>
            <template v-else-if="detail">
              <div class="detail-intro">
                <span class="recipe-kind">{{ kindLabel(detail.menu_kind) }}</span>
                <h3>{{ detail.name }}</h3>
                <p v-if="detail.description" class="recipe-description">{{ detail.description }}</p>
                <div class="recipe-facts"><span>{{ detail.items.length }} bahan</span><span>{{ detail.total_time ? `${detail.total_time} menit` : 'Durasi belum diatur' }}</span><span>Komposisi untuk 1× resep</span></div>
              </div>
              <div class="recipe-use-row"><span>{{ activeLink ? 'Sudah ditambahkan ke menu' : 'Gabungkan dengan resep lain untuk 1 menu' }}</span><AppActionButton :icon="activeLink ? 'close' : 'add'" :label="activeLink ? 'Lepas resep' : 'Tambahkan resep'" :disabled="!canUse" @click="useRecipe" /></div>
              <div class="detail-tabs" role="group" aria-label="Isi resep">
                <button type="button" :aria-pressed="detailTab === 'items'" @click="detailTab = 'items'">Bahan &amp; takaran</button>
                <button type="button" :aria-pressed="detailTab === 'cost'" @click="detailTab = 'cost'">HPP &amp; acuan</button>
                <button type="button" :aria-pressed="detailTab === 'steps'" @click="detailTab = 'steps'">Langkah SOP <span>{{ steps.length }}</span></button>
              </div>
              <div v-if="detailTab === 'items'" class="detail-content">
                <table class="ingredients">
                  <caption class="sr-only">Bahan dan takaran untuk satu kali resep</caption>
                  <thead><tr><th scope="col">Bahan</th><th scope="col">Takaran</th><th scope="col">Biaya</th></tr></thead>
                  <tbody><tr v-for="item in detail.items" :key="item.id || item.item_id"><td><strong>{{ item.item_name }}</strong><small v-if="item.notes">{{ item.notes }}</small></td><td>{{ quantity(item.qty_base) }} <span>{{ item.unit }}</span></td><td>{{ ingredientCost(item.item_id) }}</td></tr></tbody>
                </table>
              </div>
              <RecipeCostSummary v-else-if="detailTab === 'cost'" :cost="detail" />
              <div v-else class="detail-content">
                <ol v-if="steps.length" class="recipe-steps"><li v-for="(step, index) in steps" :key="index"><span class="step-number">{{ index + 1 }}</span><div><p>{{ step.text }}</p><small v-if="step.duration > 0">{{ quantity(step.duration) }} menit</small></div></li></ol>
                <p v-else class="empty-steps">Langkah SOP belum ditambahkan pada resep ini.</p>
              </div>
            </template>
            <div v-else class="picker-state detail-placeholder"><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 3h13a3 3 0 0 1 3 3v15H7a3 3 0 0 1-3-3V3Zm0 14h16M8 7h8M8 11h6"/></svg><strong>Periksa resep sebelum menautkan</strong><p>Pilih resep untuk melihat bahan, takaran, dan langkah pembuatannya.</p></div>
          </section>
        </div>

        <div v-if="draft.length" class="selection-tray" aria-label="Resep yang dipilih">
          <div v-for="link in draft" :key="link.recipe_master_id" class="selection-item">
            <button type="button" class="selection-name" :title="link.name" @click="preview(link.recipe_master_id)">{{ link.name || 'Resep tertaut' }}</button>
            <label><input v-model.number="link.multiplier" type="number" min="0.0001" max="99999999999" step="0.0001" :aria-label="`Pengali ${link.name}`" /> ×</label>
            <AppActionButton icon="close" :label="`Lepas ${link.name}`" @click="remove(link.recipe_master_id)" />
          </div>
        </div>
        <footer class="picker-footer">
          <div class="footer-note"><strong>{{ draft.length }} resep untuk 1 menu</strong><span>Pengali 0,5 = setengah resep. Tautan disimpan bersama produk.</span></div>
          <div class="picker-actions">
            <button type="button" class="picker-secondary" @click="emit('select', [])">Tanpa resep</button>
            <button type="button" class="picker-secondary picker-cancel" @click="emit('close')">Batal</button>
            <button type="button" class="picker-primary" :disabled="!validDraft" @click="applyRecipes">Terapkan pilihan</button>
          </div>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { productsApi } from '@/api/products'
import RecipeCostSummary from '@/components/RecipeCostSummary.vue'
import { formatRupiah as money } from '@/utils/format'
import { kindLabel } from '@/utils/fnb'

const props = defineProps({
  recipes: { type: Array, default: () => [] },
  loading: Boolean,
  error: { type: String, default: '' },
  outletId: { type: String, required: true },
  outletName: { type: String, default: '' },
  productName: { type: String, default: '' },
  selected: { type: Array, default: () => [] },
})
const emit = defineEmits(['close', 'select', 'retry'])
const draft = ref(props.selected.map(link => ({ ...link })))
const activeLink = computed(() => draft.value.find(link => link.recipe_master_id === activeId.value))
const validDraft = computed(() => draft.value.every(link => Number.isFinite(Number(link.multiplier)) && Number(link.multiplier) >= .0001 && Number(link.multiplier) <= 99999999999))
function remove(id) { draft.value = draft.value.filter(link => link.recipe_master_id !== id) }
function applyRecipes() { if (validDraft.value) emit('select', draft.value.map(link => ({ ...link, multiplier: Number(link.multiplier) }))) }
function ingredientCost(id) { const item = detail.value?.cost_ingredients?.find(i => i.item_id === id); return item?.cost_per_base > 0 ? money(item.cost) : '—' }
const dialog = ref(null), searchInput = ref(null), detailPanel = ref(null)
const query = ref(''), kind = ref(''), activeId = ref(''), detailTab = ref('items'), mobileDetail = ref(false)
const detail = ref(null), detailLoading = ref(false), detailError = ref('')
const filters = [{ id: '', name: 'Semua' }, { id: 'food', name: 'Makanan' }, { id: 'beverage', name: 'Minuman' }]
const normalized = value => String(value || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLocaleLowerCase().trim()
const filteredRecipes = computed(() => props.recipes.filter(r => (!kind.value || r.menu_kind === kind.value) && normalized(r.name).includes(normalized(query.value))))
const canUse = computed(() => !props.loading && !props.error && !detailLoading.value && !detailError.value && detail.value?.id === activeId.value && detail.value?.items?.length > 0)
const quantity = value => new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(value)

function plainText(value) {
  const doc = new DOMParser().parseFromString(String(value || ''), 'text/html')
  doc.querySelectorAll('script, style, iframe, object').forEach(el => el.remove())
  doc.querySelectorAll('br').forEach(el => el.replaceWith('\n'))
  doc.querySelectorAll('p, div, li').forEach(el => el.append('\n'))
  return doc.body.textContent.trim()
}
const steps = computed(() => {
  const source = detail.value?.instructions
  if (!source) return []
  try {
    const parsed = typeof source === 'string' ? JSON.parse(source) : source
    if (Array.isArray(parsed)) return parsed.map(step => ({ text: plainText(step?.text), duration: Number(step?.duration) || 0 })).filter(step => step.text)
  } catch { /* Historical instructions may be plain text. */ }
  const text = plainText(source)
  return text ? [{ text, duration: 0 }] : []
})

let request = 0
async function loadDetail() {
  const current = ++request, id = activeId.value, outlet = props.outletId
  detail.value = null
  detailError.value = ''
  detailLoading.value = !!id
  if (!id) return
  try {
    const result = await productsApi.recipeDetail(outlet, id)
    if (current !== request) return
    if (result?.id !== id || !Array.isArray(result.items) || !result.items.length) throw new Error('Komposisi resep belum tersedia. Pilih resep lainnya.')
    detail.value = result
  } catch (error) {
    if (current === request) detailError.value = error.message || 'Gagal memuat isi resep.'
  } finally {
    if (current === request) detailLoading.value = false
  }
}
watch(activeId, () => { detailTab.value = 'items'; loadDetail(); if (detailPanel.value) detailPanel.value.scrollTop = 0 })
watch(filteredRecipes, recipes => {
  if (!recipes.some(r => r.id === activeId.value)) activeId.value = recipes.find(r => draft.value.some(l => l.recipe_master_id === r.id))?.id || recipes[0]?.id || ''
}, { immediate: true })
watch(() => props.outletId, () => { request++; detail.value = null; emit('close') })

async function preview(id) {
  activeId.value = id
  mobileDetail.value = true
  await nextTick()
  if (window.matchMedia('(max-width: 767px)').matches) dialog.value?.focus({ preventScroll: true })
}
async function backToList() {
  mobileDetail.value = false
  await nextTick()
  searchInput.value?.focus({ preventScroll: true })
}
function useRecipe() {
  if (!canUse.value) return
  if (activeLink.value) remove(activeId.value)
  else draft.value.push({ recipe_master_id: detail.value.id, name: detail.value.name, multiplier: 1 })
}
function onKeydown(event) {
  if (!dialog.value) return
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); emit('close'); return }
  if (event.key !== 'Tab') return
  const focusable = [...dialog.value.querySelectorAll('button:not(:disabled), input:not(:disabled), [tabindex="0"]')].filter(el => el.getClientRects().length)
  const first = focusable[0], last = focusable.at(-1)
  if (!dialog.value.contains(document.activeElement)) { event.preventDefault(); (event.shiftKey ? last : first)?.focus() }
  else if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last?.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
}
let previousFocus
onMounted(async () => {
  previousFocus = document.activeElement
  document.addEventListener('keydown', onKeydown, true)
  await nextTick()
  searchInput.value?.focus({ preventScroll: true })
})
onBeforeUnmount(() => {
  request++
  document.removeEventListener('keydown', onKeydown, true)
  nextTick(() => { if (previousFocus?.isConnected && previousFocus.getClientRects().length && !previousFocus.closest('[inert]')) previousFocus.focus({ preventScroll: true }) })
})
</script>

<style scoped>
.recipe-use-row { display:flex;align-items:center;justify-content:space-between;gap:12px;font-size:.7rem;color:#728278;margin:12px 0 18px; }
.selection-tray { display:flex;gap:8px;padding:10px 18px;border-top:1px solid #e5ece7;overflow-x:auto;flex-shrink:0; }
.selection-item { display:flex;align-items:center;gap:8px;border:1px solid #dae6de;border-radius:8px;padding:5px 7px;background:#f7faf8;flex-shrink:0;font-size:.73rem; }
.selection-name { max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap; }
.selection-item label { white-space:nowrap; }
.selection-item input { border:1px solid #dce5df;border-radius:5px;padding:5px;width:66px;font-size:.8rem; }
.recipe-picker-overlay { position: fixed; inset: 0; z-index: 1010; padding: 20px; display: flex; align-items: center; justify-content: center; background: rgb(16 34 24 / .38); backdrop-filter: blur(4px); }
.recipe-picker { width: 100%; max-width: 1020px; height: min(680px, calc(100dvh - 40px)); display: flex; flex-direction: column; overflow: hidden; border-radius: 18px; background: #fff; box-shadow: 0 24px 80px rgb(0 0 0 / .22); color: #244332; outline: none; }
.picker-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 24px; border-bottom: 1px solid #e5ece7; flex-shrink: 0; }
.picker-header > div { min-width: 0; }
.picker-header h2 { font-size: 1.12rem; font-weight: 700; }
.picker-header p { margin-top: 3px; color: #74857a; font-size: .76rem; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.picker-header p span { padding: 0 4px; }
.picker-panels { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(0, 1.3fr); min-height: 0; flex: 1; }
.recipe-catalog { min-height: 0; display: flex; flex-direction: column; border-right: 1px solid #e5ece7; background: #fbfcfb; }
.catalog-tools { padding: 18px 18px 10px; flex-shrink: 0; }
.recipe-search { display: flex; align-items: center; gap: 9px; border: 1px solid #dce5df; padding: 9px 11px; border-radius: 9px; background: white; }
.recipe-search:focus-within { border-color: #63a37c; box-shadow: 0 0 0 2px #e7f3eb; }
.recipe-search svg { width: 17px; height: 17px; color: #7b9182; flex: none; }
.recipe-search input { min-width: 0; width: 100%; outline: none; font-size: .82rem; background: transparent; }
.recipe-filters { display: flex; gap: 4px; margin-top: 12px; }
.recipe-filters button { flex: 1; padding: 6px 8px; border: 1px solid transparent; border-radius: 7px; color: #728277; font-size: .72rem; }
.recipe-filters button[aria-pressed='true'] { background: #edf5ef; border-color: #d8e7dc; color: #27623c; font-weight: 600; }
.catalog-count { margin-top: 12px; font-size: .68rem; color: #849187; }
.recipe-list { list-style: none; margin: 0; padding: 0 10px 10px; overflow-y: auto; overscroll-behavior: contain; min-height: 0; }
.recipe-choice { display: flex; align-items: center; gap: 12px; width: 100%; padding: 13px 12px; margin-bottom: 3px; border: 1px solid transparent; border-radius: 10px; text-align: left; }
.recipe-choice:hover { background: #f1f5f2; }
.recipe-choice[aria-pressed='true'] { border-color: #cce0d2; background: #edf5ef; }
.choice-copy { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 4px; }
.choice-copy strong { font-size: .84rem; line-height: 1.45; font-weight: 600; overflow-wrap: anywhere; }
.choice-copy > span { font-size: .7rem; color: #708477; }
.choice-copy small { font-size: .69rem; color: #89958d; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.choice-arrow, .choice-check { width: 17px; height: 17px; flex: none; color: #91a497; }
.choice-check { color: #31744a; }
.recipe-detail { min-width: 0; overflow-y: auto; overscroll-behavior: contain; padding: 24px; }
.detail-intro h3 { margin: 10px 0 7px; font-size: 1.32rem; line-height: 1.35; font-weight: 650; overflow-wrap: anywhere; outline: none; }
.recipe-kind { display: inline-block; padding: 4px 8px; background: #f0f5f1; border-radius: 6px; color: #63806b; font-size: .68rem; }
.recipe-description { font-size: .8rem; line-height: 1.6; color: #718276; white-space: pre-line; overflow-wrap: anywhere; }
.recipe-facts { display: flex; flex-wrap: wrap; gap: 6px 16px; margin: 14px 0 20px; font-size: .7rem; color: #728278; }
.detail-tabs { display: flex; gap: 16px; border-bottom: 1px solid #e7ece9; }
.detail-tabs button { padding: 0 0 10px; border-bottom: 2px solid transparent; color: #7a8a7f; font-size: .8rem; }
.detail-tabs button[aria-pressed='true'] { border-color: #34744c; color: #245a38; font-weight: 600; }
.detail-tabs button span { padding-left: 4px; font-size: .68rem; color: #8a9a8e; }
.ingredients { width: 100%; border-collapse: collapse; table-layout: fixed; }
.ingredients th { padding: 12px 0 8px; font-size: .65rem; font-weight: 500; color: #8a998f; text-align: left; }
.ingredients td { padding: 12px 0; border-bottom: 1px solid #edf1ee; font-size: .8rem; vertical-align: top; overflow-wrap: anywhere; }
.ingredients th:last-child, .ingredients td:last-child { text-align: right; width: 32%; padding-left: 12px; }
.ingredients strong { font-weight: 500; }
.ingredients small { display: block; font-size: .7rem; color: #809084; margin-top: 4px; white-space: pre-line; }
.ingredients td > span { color: #7b8c80; }
.recipe-steps { list-style: none; margin: 0; padding: 18px 0 0; }
.recipe-steps li { display: flex; gap: 12px; margin-bottom: 20px; }
.recipe-steps li > div { min-width: 0; }
.step-number { flex: 0 0 25px; height: 25px; background: #f0f5f1; border-radius: 50%; text-align: center; line-height: 25px; color: #598369; font-size: .7rem; }
.recipe-steps p { font-size: .8rem; line-height: 1.7; white-space: pre-line; overflow-wrap: anywhere; }
.recipe-steps small { display: block; color: #86978b; font-size: .68rem; margin-top: 6px; }
.empty-steps { padding-top: 20px; font-size: .8rem; color: #7f9085; }
.picker-state { padding: 32px 20px; display: flex; flex-direction: column; gap: 10px; align-items: center; justify-content: center; text-align: center; color: #7c8d81; font-size: .8rem; line-height: 1.6; overflow-y: auto; }
.picker-state strong { color: #496451; font-weight: 600; }
.picker-state p { max-width: 300px; overflow-wrap: anywhere; }
.detail-placeholder { height: 100%; }
.detail-placeholder svg { width: 34px; height: 34px; color: #9bb4a2; margin-bottom: 6px; }
.picker-footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 16px 24px; border-top: 1px solid #e5ece7; flex-shrink: 0; }
.footer-note { display: flex; flex-direction: column; gap: 3px; font-size: .67rem; color: #829188; }
.footer-note strong { font-size: .71rem; font-weight: 500; color: #637a6b; }
.picker-actions { display: flex; gap: 8px; flex-shrink: 0; }
.picker-secondary, .picker-primary { border-radius: 8px; min-height: 38px; padding: 8px 12px; font-size: .76rem; cursor: pointer; }
.picker-secondary { background: white; border: 1px solid #dce4df; color: #61786a; }
.picker-primary { border: 1px solid #215b37; background: #215b37; color: white; }
.picker-primary:disabled { opacity: .4; cursor: not-allowed; }
button:focus-visible { outline: 2px solid #508e66; outline-offset: 2px; }
.back-to-list { display: none; }
@media (max-width: 767px) {
  .recipe-picker-overlay { padding: 12px; }
  .recipe-picker { height: calc(100dvh - 24px); border-radius: 14px; }
  .picker-header { padding: 14px 16px; gap: 10px; }
  .picker-header h2 { font-size: 1rem; }
  .picker-header p { font-size: .68rem; }
  .picker-panels { display: flex; }
  .recipe-catalog { border-right: 0; flex: 1; min-width: 0; }
  .catalog-tools { padding: 14px 16px 10px; }
  .recipe-search input { font-size: 16px; }
  .recipe-filters button { min-height: 38px; }
  .recipe-detail { display: none; padding: 16px; width: 100%; }
  .show-detail .recipe-catalog { display: none; }
  .show-detail .recipe-detail { display: block; }
  .back-to-list { display: flex; align-items: center; gap: 4px; font-size: .74rem; color: #64806d; margin-bottom: 18px; min-height: 28px; }
  .back-to-list svg { width: 16px; height: 16px; }
  .detail-intro h3 { font-size: 1.18rem; }
  .recipe-facts { gap: 7px 12px; margin-bottom: 18px; }
  .picker-footer { display: block; padding: 12px 16px; }
  .footer-note { margin-bottom: 9px; }
  .footer-note > span, .picker-cancel { display: none; }
  .picker-actions > button { flex: 1; min-height: 42px; }

}
@media (max-height: 540px) and (min-width: 768px) {
  .picker-header { padding-top: 10px; padding-bottom: 10px; }
  .picker-footer { padding-top: 10px; padding-bottom: 10px; }
  .recipe-detail { padding-top: 16px; }
}
</style>
