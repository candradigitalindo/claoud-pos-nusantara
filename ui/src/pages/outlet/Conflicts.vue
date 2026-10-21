<!--
  Conflicts.vue — Sync conflicts with resolve button
  ───────────────────────────────────────────────────────────────
  Route: /outlets/:id/conflicts   (child of OutletDetail)

  AI NOTE: CONFLICT_STRATEGY values are in utils/constants.js.
  Resolution calls PUT /outlets/:id/conflicts/:conflictId/resolve.
-->
<template>
  <div class="space-y-4">
    <AppAlert type="error" :message="errorMsg" />
    <AppCard :padding="false">
      <AppTable
        :columns="COLUMNS"
        :rows="items"
        :loading="loading"
        emptyText="Tidak ada konflik data. 🎉"
      >
        <template #cell-status="{ row }"><AppBadge :status="row.status" /></template>
        <template #cell-created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
        <template #cell-actions="{ row }">
          <div v-if="row.status !== 'resolved'" class="flex items-center gap-2 flex-wrap">
            <AppActionButton :icon="({ cloud_wins: 'cloud', local_wins: 'device', newest_wins: 'history' })[strategy.value]" :label="(strategy.label)"
              v-for="strategy in STRATEGIES"
              :key="strategy.value"
              :loading="resolvingId === `${row.id}-${strategy.value}`"
              @click="resolve(row, strategy.value)" />
          </div>
          <span v-else class="text-xs text-gray-400">Selesai</span>
        </template>
      </AppTable>
      <div class="px-4 py-3 border-t border-gray-100">
        <AppPagination v-model="page" :total="total" :perPage="PAGE_SIZE" />
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { outletsApi } from '@/api/outlets.js'
import { formatDateTime } from '@/utils/format.js'
import { PAGE_SIZE, CONFLICT_STRATEGY } from '@/utils/constants.js'
import { useToastStore } from '@/stores/toast.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppBadge from '@/components/ui/AppBadge.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppPagination from '@/components/ui/AppPagination.vue'

const route    = useRoute()
const outletId = computed(() => route.params.id)
const toast    = useToastStore()

const COLUMNS = [
  { key: 'entity_type', label: 'Entitas'     },
  { key: 'entity_id',   label: 'ID', class: 'font-mono text-xs' },
  { key: 'status',      label: 'Status'      },
  { key: 'created_at',  label: 'Waktu'       },
  { key: 'actions',     label: 'Selesaikan', class: 'w-52' },
]

/** Strategy buttons shown per row */
const STRATEGIES = [
  { value: CONFLICT_STRATEGY.CLOUD_WINS,  label: 'Pakai Cloud'  },
  { value: CONFLICT_STRATEGY.LOCAL_WINS,  label: 'Pakai Lokal'  },
  { value: CONFLICT_STRATEGY.NEWEST_WINS, label: 'Terbaru'       },
]

const items       = ref([])
const total       = ref(0)
const page        = ref(1)
const loading     = ref(false)
const errorMsg    = ref('')
const resolvingId = ref(null)

onMounted(fetch); watch(page, fetch)

async function fetch() {
  loading.value = true; errorMsg.value = ''
  try {
    const res  = await outletsApi.conflicts(outletId.value, { page: page.value, limit: PAGE_SIZE })
    const data = res.data?.data ?? res.data
    items.value = data.conflicts ?? data ?? []
    total.value = data.total ?? items.value.length
  } catch (err) {
    errorMsg.value = err?.response?.data?.message ?? 'Gagal memuat konflik.'
  } finally { loading.value = false }
}

async function resolve(row, strategy) {
  resolvingId.value = `${row.id}-${strategy}`
  try {
    await outletsApi.resolveConflict(outletId.value, row.id, strategy)
    row.status = 'resolved'
    toast.success('Konflik berhasil diselesaikan.')
  } catch (err) {
    toast.error(err?.response?.data?.message ?? 'Gagal menyelesaikan konflik.')
  } finally {
    resolvingId.value = null
  }
}
</script>
