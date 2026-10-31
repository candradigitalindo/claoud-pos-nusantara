<!-- Product list within one outlet; classification uses the same outlet scope. -->
<template>
  <div class="space-y-4">
    <AppAlert type="error" :message="errorMsg" />
    <p v-if="!canView" class="text-sm text-gray-500">Anda tidak memiliki izin melihat produk.</p>
    <template v-else>
      <div class="outlet-product-tools">
        <input v-model="search" type="search" aria-label="Cari produk outlet" placeholder="Cari nama, kode, atau kategori…" />
        <select v-model="menuKind" aria-label="Filter jenis produk outlet">
          <option value="">Semua jenis</option>
          <option v-for="kind in PRODUCT_KINDS" :key="kind.value" :value="kind.value">{{ kind.label }}</option>
          <option value="unclassified">Belum ditentukan</option>
        </select>
        <button type="button" @click="showClassification = true">Analisis jenis</button>
      </div>
      <AppCard :padding="false">
        <AppTable :columns="COLUMNS" :rows="items" :loading="loading" emptyText="Belum ada produk yang sesuai filter.">
          <template #cell-menu_kind="{ row }"><ProductKindBadge :kind="row.menu_kind || ''" /></template>
          <template #cell-price="{ row }">{{ formatRupiah(row.price) }}</template>
          <template #cell-is_available="{ row }">
            <AppBadge v-if="row.is_available != null" :status="row.is_available ? 'active' : 'inactive'" />
            <span v-else>—</span>
          </template>
        </AppTable>
        <div class="px-4 py-3 border-t border-gray-100">
          <AppPagination v-model="page" :total="total" :perPage="PAGE_SIZE" />
        </div>
      </AppCard>
      <ProductClassificationReview v-if="showClassification" :key="outletId" :outlet-id="outletId" :scope-label="outlet?.name || 'Outlet ini'" @close="showClassification = false" @applied="fetchProducts" />
    </template>
  </div>
</template>

<script setup>
import { ref, watch, onBeforeUnmount, computed } from 'vue'
import { useRoute } from 'vue-router'
import { productsApi } from '@/api/products.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah } from '@/utils/format.js'
import { PAGE_SIZE } from '@/utils/constants.js'
import { PRODUCT_KINDS } from '@/utils/productKinds.js'
import ProductKindBadge from '@/components/ProductKindBadge.vue'
import ProductClassificationReview from '@/components/ProductClassificationReview.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppBadge from '@/components/ui/AppBadge.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'

defineProps({ outlet: { type: Object, default: null } })
const route = useRoute(), auth = useAuthStore()
const outletId = computed(() => String(route.params.id || ''))
const canView = computed(() => auth.hasPermission('products.view'))
const COLUMNS = [
  { key: 'code', label: 'Kode', class: 'font-mono text-xs' },
  { key: 'name', label: 'Nama Produk' },
  { key: 'category_name', label: 'Kategori' },
  { key: 'menu_kind', label: 'Jenis' },
  { key: 'price', label: 'Harga' },
  { key: 'stock', label: 'Stok' },
  { key: 'is_available', label: 'Status' },
]
const items = ref([]), total = ref(0), page = ref(1), search = ref(''), menuKind = ref(''), showClassification = ref(false)
const loading = ref(false), errorMsg = ref('')
let request = 0, searchTimer
watch(page, fetchProducts)
watch([outletId, menuKind, canView], () => { if (page.value !== 1) page.value = 1; else fetchProducts() }, { immediate: true })
watch(outletId, () => { showClassification.value = false })
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { if (page.value !== 1) page.value = 1; else fetchProducts() }, 350)
})
onBeforeUnmount(() => { request++; clearTimeout(searchTimer) })
async function fetchProducts() {
  const current = ++request
  if (!canView.value || !outletId.value) { items.value = []; total.value = 0; return }
  loading.value = true; errorMsg.value = ''
  try {
    const result = await productsApi.listProducts({ outlet_id: outletId.value, page: page.value, limit: PAGE_SIZE, search: search.value.trim() || undefined, menu_kind: menuKind.value || undefined })
    if (current !== request) return
    items.value = result.items || []
    total.value = result.total || 0
  } catch (err) {
    if (current === request) { items.value = []; total.value = 0; errorMsg.value = err?.message || 'Gagal memuat produk.' }
  } finally { if (current === request) loading.value = false }
}
</script>

<style scoped>
.outlet-product-tools { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
.outlet-product-tools input, .outlet-product-tools select, .outlet-product-tools button { min-width: 0; max-width: 100%; border: 1px solid #d4e0d8; border-radius: 9px; padding: 9px 12px; font-size: .8rem; background: white; color: #345742; }
.outlet-product-tools input { flex: 1 1 220px; } .outlet-product-tools button { cursor: pointer; font-weight: 650; }
</style>
