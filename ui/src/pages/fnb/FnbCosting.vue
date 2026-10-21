<template>
  <div class="space-y-5">
    <div><h1 class="text-2xl font-bold text-gray-900">HPP Menu</h1><p class="text-sm text-gray-500 mt-1">Biaya standar per porsi, harga jual, dan target makanan / minuman.</p></div>
    <AppCard><div class="flex flex-wrap items-end gap-3">
      <FnbOutletFilter v-model="outlet" />
      <div><label for="fnb-cost-kind" class="block text-xs text-gray-500 mb-1">Kategori F&B</label><select id="fnb-cost-kind" v-model="kind" class="field"><option value="">Semua kategori</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option><option value="unclassified">Belum dikategorikan</option></select></div>
      <div class="flex-1 min-w-40"><label for="fnb-cost-search" class="block text-xs text-gray-500 mb-1">Cari menu</label><input id="fnb-cost-search" v-model="search" class="field w-full" placeholder="Nama menu atau kategori POS" /></div>
    </div></AppCard>
    <AppAlert type="error" :message="error" />
    <div class="rounded-xl bg-blue-50 border border-blue-100 p-4 text-xs text-blue-800 leading-relaxed">
      HPP = bahan resep/stok langsung + kemasan + overhead per porsi. Harga bahan memakai HPP gudang outlet, lalu gudang induk bila belum tersedia.
      Estimasi margin belum merupakan laba bersih. Produk tanpa resep tetap boleh dijual; sumber biaya yang belum tersedia ditandai, bukan dianggap gratis.
    </div>
    <AppCard :padding="false">
      <div class="overflow-x-auto"><table class="min-w-full text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500"><tr><th class="cell text-left">Menu / Outlet</th><th class="cell text-left">Kategori</th><th class="cell text-right">HPP / porsi</th><th class="cell text-right">Harga jual</th><th class="cell text-right">HPP % / target</th><th class="cell text-right">Margin kontribusi</th><th class="cell"></th></tr></thead>
        <tbody class="divide-y divide-gray-100">
          <tr v-if="loading"><td colspan="7" class="cell text-center text-gray-500">Memuat HPP…</td></tr>
          <tr v-else-if="!rows.length"><td colspan="7" class="cell text-center text-gray-500">Belum ada menu dalam filter ini.</td></tr>
          <template v-for="row in rows" v-else :key="row.product_id">
            <tr :class="row.is_over ? 'bg-amber-50/50' : ''">
              <td class="cell"><b>{{ row.product_name }}</b><p class="text-xs text-gray-400">{{ row.outlet_name }} · {{ row.category || 'Tanpa kategori POS' }}</p><p v-if="row.cost_status !== 'ready'" class="text-xs text-amber-700 mt-1">{{ costStatusLabel(row.cost_status) }}</p></td>
              <td class="cell text-xs">{{ kindLabel(row.menu_kind) }}</td>
              <td class="cell text-right font-semibold whitespace-nowrap">{{ row.cost_status === 'ready' ? money(row.total_cost) : '—' }}</td>
              <td class="cell text-right whitespace-nowrap">{{ money(row.price) }}</td>
              <td class="cell text-right whitespace-nowrap" :class="row.is_over ? 'text-red-600 font-semibold' : ''">{{ row.cost_status === 'ready' && row.price > 0 ? row.cost_pct.toFixed(1) + '%' : '—' }}<span class="block text-xs text-gray-400">target {{ row.target_cost_pct }}%</span></td>
              <td class="cell text-right whitespace-nowrap">{{ row.cost_status === 'ready' && row.price > 0 ? money(row.margin) : '—' }}</td>
              <td class="cell whitespace-nowrap"><button class="text-emerald-700 text-xs font-semibold mr-3" @click="expanded = expanded === row.product_id ? '' : row.product_id">Rincian</button><button v-if="canEdit" class="text-gray-600 text-xs" @click="edit(row)">Atur HPP</button></td>
            </tr>
            <tr v-if="expanded === row.product_id"><td colspan="7" class="cell bg-gray-50">
              <div class="grid md:grid-cols-2 gap-5">
                <div><p class="font-semibold mb-2">Komponen bahan</p><p v-if="!row.ingredients?.length" class="text-xs text-gray-500">Komposisi tidak tersedia atau Anda tidak memiliki izin melihat resep.</p><div v-for="i in row.ingredients" :key="i.item_id" class="flex justify-between gap-2 text-xs py-1"><span>{{ i.item_name }} · {{ i.qty_base }} {{ i.unit }}</span><span>{{ i.cost_per_base > 0 ? money(i.cost) : 'Harga belum tersedia' }}</span></div></div>
                <div class="space-y-2 text-xs"><p class="flex justify-between"><span>Kemasan / porsi</span><b>{{ money(row.packaging_cost) }}</b></p><p class="flex justify-between"><span>Overhead / porsi</span><b>{{ money(row.overhead_cost) }}</b></p><p class="flex justify-between"><span>Harga minimum sesuai target HPP</span><b>{{ row.cost_status === 'ready' ? money(row.suggested_price) : '—' }}</b></p><p class="text-gray-400">Harga minimum adalah simulasi biaya ÷ target HPP. Belum termasuk pajak atau diskon dan tidak mengubah harga jual produk.</p></div>
              </div>
            </td></tr>
          </template>
        </tbody>
      </table></div>
      <div class="p-3 flex flex-wrap justify-between items-center gap-2"><span class="text-xs text-gray-500">{{ total }} menu</span><AppPagination :page="page" :total-pages="Math.max(1, Math.ceil(total / limit))" @update:page="page = $event" @change="page = $event" /></div>
    </AppCard>
    <AppModal v-model="showEdit" title="Kategori & Target HPP" size="md">
      <div class="space-y-4"><p class="text-sm font-semibold">{{ target?.product_name }} · {{ target?.outlet_name }}</p>
        <div><label for="fnb-profile-kind" class="block text-sm mb-1">Kategori F&B</label><select id="fnb-profile-kind" v-model="form.menu_kind" class="field w-full"><option value="">Belum dikategorikan</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option></select></div>
        <AppInput v-model.number="form.target_cost_pct" label="Target HPP (%)" type="number" min="0.01" max="100" step="0.01" />
        <AppInput v-model.number="form.packaging_cost" label="Biaya kemasan per porsi (Rp)" type="number" min="0" step="0.01" />
        <AppInput v-model.number="form.overhead_cost" label="Overhead per porsi (Rp)" type="number" min="0" step="0.01" />
        <p class="text-xs text-gray-500">Isi alokasi tenaga kerja, energi, dan biaya lain yang ingin dihitung per porsi. Target 35% adalah nilai awal yang bisa diubah sesuai kebijakan divisi.</p>
      </div>
      <template #footer><AppButton variant="secondary" @click="showEdit = false">Batal</AppButton><AppButton :loading="saving" @click="save">Simpan</AppButton></template>
    </AppModal>
  </div>
</template>
<script setup>
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
const showEdit = ref(false), saving = ref(false), target = ref(null), form = ref({})
let seq = 0, timer
async function load() {
  const request = ++seq; loading.value = true; error.value = ''; rows.value = []
  try { const data = await fnbApi.costing({ outlet_id: outlet.value, menu_kind: kind.value, search: search.value, page: page.value, limit }); if (request === seq) { rows.value = data.rows || []; total.value = data.total || 0 } }
  catch (e) { if (request === seq) error.value = e.message }
  finally { if (request === seq) loading.value = false }
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
