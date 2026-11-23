<!--
  SyncLogs.vue — Sync history for one outlet
  ───────────────────────────────────────────────────────────────
  Route: /outlets/:id/sync-logs   (child of OutletDetail)
-->
<template>
  <div class="space-y-4">
    <AppAlert type="error" :message="errorMsg" />
    <AppCard :padding="false">
      <AppTable :columns="COLUMNS" :rows="items" :loading="loading" emptyText="Belum ada log sync.">
        <template #cell-status="{ row }"><AppBadge :status="row.status" /></template>
        <template #cell-synced_at="{ row }">{{ formatDateTime(row.synced_at) }}</template>
        <template #cell-error_message="{ row }">
          <span class="text-xs text-red-600 line-clamp-2">{{ row.error_message || '—' }}</span>
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
import { formatDateTime } from '@/utils/format.js'
import { PAGE_SIZE } from '@/utils/constants.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppBadge from '@/components/ui/AppBadge.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'

const route    = useRoute()
const outletId = computed(() => route.params.id)

const COLUMNS = [
  { key: 'entity_type',   label: 'Entitas' },
  { key: 'records_count', label: 'Jumlah Record' },
  { key: 'status',        label: 'Status' },
  { key: 'error_message', label: 'Error', class: 'max-w-xs' },
  { key: 'synced_at',     label: 'Waktu' },
]

const items = ref([]); const total = ref(0); const page = ref(1)
const loading = ref(false); const errorMsg = ref('')

onMounted(fetch); watch(page, fetch)
useRealtime(['transaction', 'order', 'cashier_shift', 'device'], (ev) => {
  if (!ev?.outlet_id || ev.outlet_id === outletId.value) fetch({ silent: true })
})

async function fetch({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const res  = await outletsApi.syncLogs(outletId.value, { page: page.value, limit: PAGE_SIZE })
    const data = res.data?.data ?? res.data
    items.value = data.logs ?? data ?? []
    total.value = data.total ?? items.value.length
  } catch (err) {
    if (!silent) errorMsg.value = err?.response?.data?.message ?? 'Gagal memuat log sync.'
  } finally { if (!silent) loading.value = false }
}
</script>
