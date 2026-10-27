<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="text-xl font-bold text-gray-900">{{ isSfg ? 'Resep Bahan Setengah Jadi' : 'Pola Pemotongan' }}</h2>
        <p class="mt-1 text-sm text-gray-500">{{ isSfg
          ? 'Pilih bahan hasil, lalu atur komponen untuk menghasilkan satu satuan dasar. Pelaksanaan produksi dicatat Gudang.'
          : 'Pilih bahan sumber, lalu atur pola hasil standar dan bobot biaya. Hasil aktual dicatat saat pemotongan di Gudang.' }}</p>
      </div>
      <AppButton variant="secondary" :loading="loading" @click="loadCatalog">Muat Ulang</AppButton>
    </div>
    <p v-if="!canEdit" class="rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-800">
      Hanya baca. Pengelolaan formula memerlukan akses ubah resep dengan cakupan semua outlet.
    </p>
    <AppAlert type="error" :message="error" />
    <AppCard :padding="false">
      <div class="flex flex-wrap gap-3 items-center border-b border-gray-100 p-4">
        <input v-model="search" aria-label="Cari bahan formula" type="search" placeholder="Cari nama / kode bahan..."
          class="min-w-0 flex-1 rounded-lg border border-gray-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-400" />
        <label class="flex items-center gap-2 text-sm text-gray-600">
          <input v-model="activeOnly" type="checkbox" class="rounded text-emerald-600" /> Aktif saja
        </label>
      </div>
      <p v-if="loading" role="status" class="p-8 text-center text-sm text-gray-500">Memuat bahan…</p>
      <p v-else-if="!filteredItems.length" class="p-8 text-center text-sm text-gray-500">Tidak ada bahan yang cocok. Master bahan disiapkan melalui Bahan Baku di Gudang.</p>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="item in pagedItems" :key="item.id" :data-testid="`formula-item-${item.id}`" class="flex flex-wrap items-center justify-between gap-3 p-4">
          <div class="min-w-0">
            <p class="font-semibold text-gray-900 break-words">{{ item.name }}</p>
            <p class="mt-1 text-xs text-gray-500">{{ item.code }} <span v-if="item.category">· {{ item.category }}</span> · {{ item.is_active === false ? 'Nonaktif' : 'Aktif' }}</p>
            <p class="mt-1 text-xs text-gray-600">
              <template v-if="isSfg">Acuan resep: 1 {{ item.base_unit }} hasil</template>
              <template v-else>Acuan pola: 1 {{ item.dist_unit_label || item.dist_unit || item.base_unit }} bahan<span v-if="item.dist_unit && item.dist_unit !== item.base_unit"> = {{ item.dist_ratio }} {{ item.base_unit }}</span></template>
            </p>
          </div>
          <AppButton variant="secondary" size="sm" @click="openEditor(item)">
            {{ isSfg ? (canEdit ? 'Atur Resep' : 'Lihat Resep') : (canEdit ? 'Atur Pola' : 'Lihat Pola') }}
          </AppButton>
        </li>
      </ul>
      <AppPagination v-model="page" :total="filteredItems.length" :per-page="pageSize" class="border-t border-gray-100 px-4 py-3" />
    </AppCard>
    <StockRecipeEditors ref="editors" :items="items" :can-edit="canEdit" />
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { stockItemsApi } from '@/api/warehouse.js'
import { useAuthStore } from '@/stores/auth.js'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import StockRecipeEditors from '@/components/StockRecipeEditors.vue'

const props = defineProps({ mode: { type: String, required: true } })
const auth = useAuthStore()
const isSfg = computed(() => props.mode === 'sfg')
const canEdit = computed(() => auth.scopeType !== 'specific' && auth.hasPermission('recipes.update'))
const items = ref([])
const loading = ref(false)
const error = ref('')
const search = ref('')
const activeOnly = ref(false)
const page = ref(1)
const pageSize = 20
const editors = ref(null)
const filteredItems = computed(() => {
  const query = search.value.trim().toLocaleLowerCase('id-ID')
  return items.value.filter(item => (!activeOnly.value || item.is_active !== false) &&
    (!query || `${item.name} ${item.code}`.toLocaleLowerCase('id-ID').includes(query)))
})
const pagedItems = computed(() => filteredItems.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch([search, activeOnly], () => { page.value = 1 })
async function loadCatalog() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    // The catalog endpoint also permits recipes.view; F&B needs no stockitems.view.
    const first = await stockItemsApi.list({ page: 1, limit: 1000 })
    const catalog = [...(Array.isArray(first) ? first : first?.data || [])]
    for (let nextPage = 2; nextPage <= (first?.total_pages || 1); nextPage++) {
      const result = await stockItemsApi.list({ page: nextPage, limit: 1000 })
      catalog.push(...(result?.data || []))
    }
    items.value = catalog
    page.value = 1
  } catch (e) {
    error.value = e?.message || 'Gagal memuat bahan formula'
  } finally {
    loading.value = false
  }
}
function openEditor(item) {
  if (isSfg.value) editors.value?.openRecipe(item)
  else editors.value?.openBreakdownTemplate(item)
}
onMounted(loadCatalog)
</script>
