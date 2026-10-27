<template>
  <div class="space-y-5">
    <div><h1 class="text-2xl font-bold text-gray-900">HPP Menu</h1><p class="text-sm text-gray-500 mt-1">Biaya standar per porsi, harga jual, dan target makanan / minuman.</p></div>
    <AppCard><div class="flex flex-wrap items-end gap-3">
      <FnbOutletFilter v-model="outlet" />
      <AppButton v-if="canEdit" variant="secondary" :disabled="!outlet" title="Pilih satu outlet untuk mengatur batas HPP" @click="editPolicy">Atur batas outlet</AppButton>
      <div><label for="fnb-cost-kind" class="block text-xs text-gray-500 mb-1">Kategori F&B</label><SearchableSelect id="fnb-cost-kind" v-model="kind" class="field"><option value="">Semua kategori</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option><option value="unclassified">Belum dikategorikan</option></SearchableSelect></div>
      <div class="flex-1 min-w-40"><label for="fnb-cost-search" class="block text-xs text-gray-500 mb-1">Cari menu</label><input id="fnb-cost-search" v-model="search" class="field w-full" placeholder="Nama menu atau kategori POS" /></div>
    </div></AppCard>
    <AppAlert type="error" :message="error" />
    <div class="rounded-xl bg-blue-50 border border-blue-100 p-4 text-xs text-blue-800 leading-relaxed">
      HPP stok outlet mengikuti biaya bahan yang sudah diterima dan sisa batchnya. Pembelian di gudang induk memperbarui proyeksi; harga stok outlet berubah saat barang diterima atau batch terpakai.
      Proyeksi memakai harga penerimaan induk terbaru, atau rata-rata induk bila belum ada riwayat pembelian. HPP penjualan di keuangan mengikuti biaya batch yang terpakai.
      Batas HPP berlaku per outlet. Saat persentase mencapai batas, tinjau biaya resep atau harga jual melalui rekomendasi di rincian menu.
    </div>
    <AppCard :padding="false">
      <div class="overflow-x-auto"><table class="min-w-full text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500"><tr><th class="cell text-left">Menu / Outlet</th><th class="cell text-left">Kategori</th><th class="cell text-right">HPP outlet / proyeksi</th><th class="cell text-right">Harga jual</th><th class="cell text-right">HPP % / target</th><th class="cell text-right">Margin kontribusi</th><th class="cell"></th></tr></thead>
        <tbody class="divide-y divide-gray-100">
          <tr v-if="loading"><td colspan="7" class="cell text-center text-gray-500">Memuat HPP…</td></tr>
          <tr v-else-if="!rows.length"><td colspan="7" class="cell text-center text-gray-500">Belum ada menu dalam filter ini.</td></tr>
          <template v-for="row in rows" v-else :key="row.product_id">
            <tr :class="row.is_over || row.projected_is_over ? 'bg-amber-50/50' : ''">
              <td class="cell"><b>{{ row.product_name }}</b><p class="text-xs text-gray-400">{{ row.outlet_name }} · {{ row.category || 'Tanpa kategori POS' }}</p><p v-if="row.cost_status !== 'ready'" class="text-xs text-amber-700 mt-1">{{ costStatusLabel(row.cost_status) }}</p></td>
              <td class="cell text-xs">{{ kindLabel(row.menu_kind) }}</td>
              <td class="cell text-right font-semibold whitespace-nowrap">{{ row.cost_status === 'ready' ? money(row.total_cost) : '—' }}<span v-if="row.cost_basis === 'estimate'" class="block text-xs font-normal text-amber-700">Acuan / estimasi</span><span class="block text-xs font-normal mt-1" :class="row.projected_is_over ? 'text-amber-700' : 'text-gray-400'">Proyeksi {{ row.projected_total_cost != null ? money(row.projected_total_cost) : 'belum lengkap' }}</span><span v-if="row.recipes?.length" class="block text-xs text-gray-400 font-normal mt-1">{{ row.recipes.length }} resep</span></td>
              <td class="cell text-right whitespace-nowrap">{{ money(row.price) }}</td>
              <td class="cell text-right whitespace-nowrap" :class="row.is_over ? 'text-red-600 font-semibold' : ''">{{ row.cost_status === 'ready' && row.price > 0 ? row.cost_pct.toFixed(1) + '%' : '—' }}<span class="block text-xs text-gray-400">batas {{ row.target_cost_pct }}% · {{ row.target_source === 'outlet' ? 'outlet' : 'menu / awal' }}<span v-if="row.projected_cost_pct != null" class="block mt-1" :class="row.projected_is_over ? 'text-amber-700' : ''">Proyeksi {{ row.projected_cost_pct.toFixed(1) }}%</span></span></td>
              <td class="cell text-right whitespace-nowrap">{{ row.cost_status === 'ready' && row.price > 0 ? money(row.margin) : '—' }}</td>
              <td class="cell whitespace-nowrap"><AppActionButton icon="view" label="Rincian" @click="expanded = expanded === row.product_id ? '' : row.product_id" class="mr-3" :aria-expanded="expanded === row.product_id" /><AppActionButton icon="cost" label="Atur HPP" v-if="canEdit" @click="edit(row)" /></td>
            </tr>
            <tr v-if="expanded === row.product_id"><td colspan="7" class="cell bg-gray-50">
              <div v-if="row.is_over || row.projected_is_over" class="rounded-lg border border-amber-200 bg-amber-50 p-4 mb-5 text-xs">
                <p class="font-semibold text-amber-900">{{ row.is_over ? 'HPP outlet mencapai batas' : 'Peringatan dini: proyeksi mencapai batas' }} {{ row.target_cost_pct }}%</p>
                <p class="mt-1 text-amber-800">Pilih penyesuaian setelah meninjau kualitas, porsi, dan margin. Rekomendasi tidak mengubah produk atau resep.</p>
                <div class="grid sm:grid-cols-2 gap-4 mt-3">
                  <div><b>1. Tinjau harga jual</b><p class="mt-1">Harga minimum: {{ money(reviewPrice(row)) }} / menu</p><p class="text-gray-500">Biaya {{ useProjectionForReview(row) ? 'proyeksi' : 'outlet' }} ÷ batas HPP.</p></div>
                  <div><b>2. Tinjau biaya resep</b><p class="mt-1">Anggaran bahan: maksimal {{ money(row.max_material_cost) }} / menu</p><p class="text-gray-500">Kurangi biaya bahan sekitar {{ money(reviewReduction(row)) }} melalui evaluasi bahan atau takaran. Kemasan dan overhead tetap dihitung.</p><p v-if="row.packaging_cost + row.overhead_cost > row.price * row.target_cost_pct / 100" class="text-amber-800 mt-1">Biaya kemasan dan overhead sudah melebihi batas; pengurangan bahan saja belum cukup.</p></div>
                </div>
              </div>
              <div v-if="row.price_comparisons?.length" class="mb-5">
                <p class="font-semibold mb-2">Harga bahan: outlet dan proyeksi</p>
                <div class="overflow-x-auto"><table class="w-full text-xs"><thead><tr class="text-gray-500"><th class="text-left py-2">Bahan / satuan</th><th class="text-right">Acuan outlet</th><th class="text-right">Proyeksi</th><th class="text-right">Sumber proyeksi</th></tr></thead><tbody>
                  <tr v-for="item in row.price_comparisons" :key="item.item_id" class="border-t border-gray-100"><td class="py-2 pr-3">{{ item.item_name }} / {{ item.unit }}</td><td class="text-right pr-3">{{ item.current_unit_cost > 0 ? money(item.current_unit_cost) : 'Belum ada' }}</td><td class="text-right pr-3">{{ item.projected_unit_cost != null ? money(item.projected_unit_cost) : 'Belum ada' }}</td><td class="text-right text-gray-500">{{ priceSource(item.source) }}<span v-if="item.as_of" class="block">{{ new Date(item.as_of).toLocaleDateString('id-ID') }}</span></td></tr>
                </tbody></table></div>
                <p class="text-xs text-gray-400 mt-2">Proyeksi menunjukkan biaya jika bahan menggunakan harga acuan induk. Harga transfer sebenarnya mengikuti batch yang dikirim. Ini bukan penilaian ulang persediaan atau HPP transaksi lama.</p>
              </div>
              <div v-if="row.recipes?.length" class="mb-5">
                <p class="font-semibold mb-2">Resep untuk 1 menu</p>
                <div v-for="recipe in row.recipes" :key="recipe.recipe_master_id" class="mb-3">
                  <div class="flex justify-between gap-3 text-xs"><b>{{ recipe.name }} × {{ recipe.multiplier }}</b><span>{{ recipe.current_cost != null ? money(recipe.current_cost * recipe.multiplier) : 'Harga belum lengkap' }}</span></div>
                  <details class="text-xs mt-2"><summary class="cursor-pointer text-gray-500">Harga dan acuan resep</summary><RecipeCostSummary :cost="recipe" /></details>
                </div>
                <div class="grid sm:grid-cols-3 gap-3 border-t pt-3 text-xs">
                  <p>Bahan terkini <b class="block mt-1">{{ row.cost_status === 'ready' ? money(row.material_cost) : 'Belum lengkap' }}</b></p>
                  <p>Acuan bahan awal <b class="block mt-1">{{ row.reference_material_cost != null ? money(row.reference_material_cost) : 'Belum tersedia lengkap' }}</b></p>
                  <p>Selisih bahan <b class="block mt-1">{{ row.cost_status === 'ready' && row.reference_material_cost != null ? money(row.material_cost - row.reference_material_cost) : '—' }}</b></p>
                </div>
                <p class="text-xs text-gray-400 mt-2">Acuan = biaya awal tiap resep × pengali saat ini, belum termasuk kemasan dan overhead. Selisih dapat berasal dari harga stok, lokasi gudang, maupun perubahan komposisi resep.</p>
              </div>
              <div class="grid md:grid-cols-2 gap-5">
                <div><p class="font-semibold mb-2">Komponen bahan</p><p v-if="!row.ingredients?.length" class="text-xs text-gray-500">Komposisi tidak tersedia atau Anda tidak memiliki izin melihat resep.</p><div v-for="i in row.ingredients" :key="i.item_id" class="flex justify-between gap-2 text-xs py-1"><span>{{ i.item_name }} · {{ i.qty_base }} {{ i.unit }}<small v-if="i.cost_per_base > 0" class="block text-gray-400">{{ money(i.cost_per_base) }} / {{ i.unit }}</small></span><span>{{ i.cost_per_base > 0 ? money(i.cost) : 'Harga belum tersedia' }}</span></div></div>
                <div class="space-y-2 text-xs"><p class="flex justify-between"><span>Kemasan / porsi</span><b>{{ money(row.packaging_cost) }}</b></p><p class="flex justify-between"><span>Overhead / porsi</span><b>{{ money(row.overhead_cost) }}</b></p><p class="flex justify-between"><span>Harga minimum sesuai target HPP</span><b>{{ row.cost_status === 'ready' ? money(row.suggested_price) : '—' }}</b></p><p class="text-gray-400">Harga minimum adalah simulasi biaya ÷ target HPP. Belum termasuk pajak atau diskon dan tidak mengubah harga jual produk.</p></div>
              </div>
            </td></tr>
          </template>
        </tbody>
      </table></div>
      <div class="p-3 flex flex-wrap justify-between items-center gap-2"><span class="text-xs text-gray-500">{{ total }} menu</span><AppPagination :page="page" :total-pages="Math.max(1, Math.ceil(total / limit))" @update:page="page = $event" @change="page = $event" /></div>
    </AppCard>
    <AppModal v-model="showPolicy" title="Batas HPP outlet" size="md">
      <p v-if="policyLoading" class="text-sm text-gray-500">Memuat pengaturan outlet…</p>
      <div v-else class="space-y-3"><p class="font-semibold">{{ policy.outlet_name }}</p><AppAlert type="error" :message="policyError" /><AppInput v-model.number="policy.target_cost_pct" label="Batas HPP (%)" type="number" min="0.01" max="100" step="0.01" /><p class="text-xs text-gray-500">Berlaku untuk seluruh menu di outlet ini, termasuk menu yang dibuat kemudian. Peringatan muncul saat HPP outlet atau proyeksi mencapai batas. Misalnya 40% berarti biaya maksimal Rp4.000 untuk harga jual Rp10.000.</p></div>
      <template #footer><AppButton variant="secondary" :disabled="policySaving" @click="showPolicy = false">Batal</AppButton><AppButton :loading="policySaving" :disabled="policyLoading || !!policyError" @click="savePolicy">Simpan batas outlet</AppButton></template>
    </AppModal>
    <AppModal v-model="showEdit" title="Kategori & Target HPP" size="lg">
      <div class="modal-form-grid"><p class="modal-span-full text-sm font-semibold">{{ target?.product_name }} · {{ target?.outlet_name }}</p>
        <div><label for="fnb-profile-kind" class="block text-sm mb-1">Kategori F&B</label><SearchableSelect id="fnb-profile-kind" v-model="form.menu_kind" class="field w-full"><option value="">Belum dikategorikan</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option></SearchableSelect></div>
        <AppInput v-model.number="form.target_cost_pct" :disabled="target?.target_source === 'outlet'" :label="target?.target_source === 'outlet' ? 'Batas mengikuti outlet (%)' : 'Target HPP menu (%)'" type="number" min="0.01" max="100" step="0.01" />
        <RupiahInput v-model="form.packaging_cost" label="Biaya kemasan per porsi (Rp)" min="0" step="0.01" />
        <RupiahInput v-model="form.overhead_cost" label="Overhead per porsi (Rp)" min="0" step="0.01" />
        <p class="modal-span-full text-xs text-gray-500">Isi alokasi tenaga kerja, energi, dan biaya lain yang ingin dihitung per porsi. Gunakan “Atur batas outlet” untuk menetapkan persentase yang sama bagi seluruh menu outlet tersebut.</p>
      </div>
      <template #footer><AppButton variant="secondary" @click="showEdit = false">Batal</AppButton><AppButton :loading="saving" @click="save">Simpan</AppButton></template>
    </AppModal>
  </div>
</template>
<script setup>
import RupiahInput from '@/components/ui/RupiahInput.vue'
import RecipeCostSummary from '@/components/RecipeCostSummary.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { fnbApi } from '@/api/fnb'
import { MENU_KINDS, kindLabel, costStatusLabel } from '@/utils/fnb'
import { formatRupiah as money } from '@/utils/format'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import FnbOutletFilter from '@/components/FnbOutletFilter.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
const auth = useAuthStore(), toast = useToastStore(), route = useRoute()
const canEdit = computed(() => auth.hasPermission('fnb.costing.update'))
const outlet = ref(String(route.query.outlet_id || '')), kind = ref(''), search = ref(''), page = ref(1), limit = 25
const rows = ref([]), total = ref(0), loading = ref(false), error = ref(''), expanded = ref('')
const showPolicy = ref(false), policyLoading = ref(false), policySaving = ref(false), policyError = ref(''), policy = ref({})
let policySeq = 0
const showEdit = ref(false), saving = ref(false), target = ref(null), form = ref({})
let seq = 0, timer
async function load() {
  const request = ++seq; loading.value = true; error.value = ''; rows.value = []
  try { const data = await fnbApi.costing({ outlet_id: outlet.value, menu_kind: kind.value, search: search.value, page: page.value, limit }); if (request === seq) { rows.value = data.rows || []; total.value = data.total || 0 } }
  catch (e) { if (request === seq) error.value = e.message }
  finally { if (request === seq) loading.value = false }
}
const priceSource = source => ({ central_purchase: 'Penerimaan induk terbaru', central_average: 'Rata-rata induk', recipe_estimate: 'Komposisi WIP' }[source] || 'Belum tersedia')
const useProjectionForReview = row => row.projected_is_over && (row.cost_status !== 'ready' || row.projected_total_cost >= row.total_cost)
const reviewPrice = row => useProjectionForReview(row) ? row.projected_suggested_price : row.suggested_price
const reviewReduction = row => useProjectionForReview(row) ? row.projected_material_reduction : row.material_reduction
watch(outlet, () => { policySeq++; showPolicy.value = false })
async function editPolicy() {
  if (!outlet.value) return
  const current = ++policySeq, id = outlet.value
  showPolicy.value = true; policyLoading.value = true; policyError.value = ''; policy.value = {}
  try { const data = await fnbApi.outletPolicy(id); if (current === policySeq) policy.value = data }
  catch (e) { if (current === policySeq) policyError.value = e.message }
  finally { if (current === policySeq) policyLoading.value = false }
}
async function savePolicy() {
  if (policySaving.value || !policy.value.outlet_id) return
  const pct = Number(policy.value.target_cost_pct)
  if (!(pct > 0 && pct <= 100)) return toast.error('Batas HPP harus lebih dari 0 hingga 100%')
  policySaving.value = true
  try { await fnbApi.saveOutletPolicy(policy.value.outlet_id, { target_cost_pct: pct }); showPolicy.value = false; toast.success('Batas HPP outlet disimpan'); await load() }
  catch (e) { toast.error(e.message) } finally { policySaving.value = false }
}
function reset() { if (page.value !== 1) page.value = 1; else load() }
watch([outlet, kind], reset)
watch(search, () => { clearTimeout(timer); timer = setTimeout(reset, 300) })
watch(page, load, { immediate: true })
function edit(row) { target.value = row; form.value = { menu_kind: row.menu_kind, target_cost_pct: row.target_cost_pct, packaging_cost: row.packaging_cost, overhead_cost: row.overhead_cost }; showEdit.value = true }
async function save() {
  if (!(form.value.target_cost_pct > 0 && form.value.target_cost_pct <= 100) || form.value.packaging_cost < 0 || form.value.overhead_cost < 0) return toast.error('Target HPP dan biaya harus valid')
  saving.value = true
  try { await fnbApi.saveProfile(target.value.product_id, form.value); showEdit.value = false; toast.success('Pengaturan HPP disimpan'); await load() }
  catch (e) { toast.error(e.message) } finally { saving.value = false }
}
</script>
<style scoped>
.field{border:1px solid #d1d5db;border-radius:.5rem;padding:.5rem .75rem;font-size:.875rem;background:white;min-width:0}.cell{padding:.85rem 1rem}
</style>
