<!--
  Transactions.vue — Paginated transaction list for one outlet
  ───────────────────────────────────────────────────────────────
  Route: /outlets/:id/transactions   (child of OutletDetail)
  Props: outletId (passed via RouterView)

  AI NOTE: Columns can be extended. The receipt_number column links to a
  future transaction detail page.
-->
<template>
  <div class="space-y-4">
    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <AppTable
        :columns="COLUMNS"
        :rows="items"
        :loading="loading"
        emptyText="Belum ada transaksi."
      >
        <template #cell-payment_method="{ row }">
          <span class="capitalize">{{ row.payment_method }}</span>
        </template>
        <template #cell-total_amount="{ row }">
          {{ formatRupiah(row.total_amount) }}
        </template>
        <template #cell-status="{ row }">
          <AppBadge :status="row.status" />
        </template>
        <template #cell-created_at="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </AppTable>
      <div class="px-4 py-3 border-t border-gray-100">
        <AppPagination v-model="page" :total="total" :perPage="PAGE_SIZE" />
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { PAGE_SIZE } from '@/utils/constants.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard        from '@/components/ui/AppCard.vue'
import AppTable       from '@/components/ui/AppTable.vue'
import AppBadge       from '@/components/ui/AppBadge.vue'
import AppAlert       from '@/components/ui/AppAlert.vue'
import AppPagination  from '@/components/ui/AppPagination.vue'

const route    = useRoute()
const outletId = computed(() => route.params.id)

const COLUMNS = [
  { key: 'receipt_number', label: 'No. Struk', class: 'font-mono text-xs' },
  { key: 'payment_method', label: 'Metode Bayar' },
  { key: 'total_amount',   label: 'Total' },
  { key: 'status',         label: 'Status' },
  { key: 'created_at',     label: 'Waktu' },
]

const items    = ref([])
const total    = ref(0)
const page     = ref(1)
const loading  = ref(false)
const errorMsg = ref('')

onMounted(fetch)
watch(page, fetch)
useRealtime(['transaction'], (ev) => {
  if (!ev?.outlet_id || ev.outlet_id === outletId.value) fetch({ silent: true })
})

async function fetch({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const res  = await outletsApi.transactions(outletId.value, { page: page.value, limit: PAGE_SIZE })
    const data = res.data?.data ?? res.data
    items.value = data.transactions ?? data ?? []
    total.value = data.total ?? items.value.length
  } catch (err) {
    if (!silent) errorMsg.value = err?.response?.data?.message ?? 'Gagal memuat transaksi.'
  } finally { if (!silent) loading.value = false }
}
</script>
