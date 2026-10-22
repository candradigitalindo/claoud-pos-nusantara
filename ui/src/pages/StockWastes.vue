<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Pembuangan Stok (Waste)</h1>
      <RouterLink v-if="auth.hasPermission('stockquality.view')" to="/stock-quality" class="text-sm font-semibold text-emerald-700">Laporkan Reject / Expired / Spoil →</RouterLink>
    </div>

    <!-- Filters -->
    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[200px]">
          <SearchSelect
            v-model="filters.warehouse_id"
            :options="warehouseOptions"
            placeholder="Semua Gudang"
            @change="resetAndLoad"
          />
        </div>
        <DateRangePicker v-model="range" clearable />
        <input v-model="filters.search" @input="debouncedLoad" placeholder="Cari item atau nomor..."
          class="flex-1 min-w-[200px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
      </div>

      <AppTable :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Tidak ada data pembuangan stok.">
        <template #cell-waste_number="{ row }">
          <span class="font-mono text-xs font-bold text-gray-600">{{ row.waste_number }}</span>
        </template>
        <template #cell-item_name="{ row }">
          <div>
            <div class="font-medium text-gray-900">{{ row.item_name }}</div>
            <div class="text-[11px] text-gray-400 font-mono">{{ row.item_code }}</div>
          </div>
        </template>
        <template #cell-qty_dist="{ row }">
          <div>
            <div class="font-semibold text-red-600">{{ row.qty_dist }} {{ row.unit_used }}</div>
            <div class="text-[10px] text-gray-400">({{ row.qty_base }} {{ row.base_unit }})</div>
          </div>
        </template>
        <template #cell-total_cost="{ row }">
          <div class="text-sm">
            <div class="font-medium text-gray-900">{{ formatRupiah(row.total_cost) }}</div>
            <div class="text-[10px] text-gray-400">{{ formatRupiah(row.cost_per_base) }}/{{ row.base_unit }}</div>
          </div>
        </template>
        <template #cell-reason="{ row }">
          <AppBadge :variant="getReasonVariant(row.reason)">
            {{ getReasonLabel(row.reason) }}
          </AppBadge>
        </template>
        <template #cell-created_at="{ row }">
          <div class="text-xs text-gray-500">
            <div>{{ formatDate(row.created_at) }}</div>
            <div class="text-[10px]">Oleh: {{ row.created_by }}</div>
          </div>
        </template>
        <template #cell-notes="{ row }">
          <span class="text-xs text-gray-500 italic">{{ row.notes || '-' }}</span>
        </template>
      </AppTable>

      <div class="px-4 py-3 border-t border-gray-100 flex justify-between items-center">
         <p class="text-xs text-gray-500">Menampilkan {{ rows?.length || 0 }} dari {{ total || 0 }} data</p>
        <AppPagination :page="page" :total-pages="totalPages" @change="changePage" />
      </div>
    </AppCard>


  </div>
</template>

<script setup>
import { ref, onMounted, reactive, computed, watch } from 'vue'
import { getWastes } from '@/api/waste'
import { warehousesApi } from '@/api/warehouse'
import { useToastStore } from '@/stores/toast'
import { formatRupiah } from '@/utils/format'
import debounce from 'lodash/debounce'
import AppBadge      from '@/components/ui/AppBadge.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import { useAuthStore } from '@/stores/auth'


const toast = useToastStore()
const auth = useAuthStore()

const COLUMNS = [
  { key: 'waste_number', label: 'Nomor' },
  { key: 'warehouse_name', label: 'Gudang' },
  { key: 'item_name',    label: 'Item' },
  { key: 'qty_dist',     label: 'Jumlah' },
  { key: 'total_cost',   label: 'Nilai Kerugian', align: 'right' },
  { key: 'reason',       label: 'Alasan' },
  { key: 'created_at',   label: 'Waktu & User' },
  { key: 'notes',        label: 'Catatan' },
]

// State
const loading = ref(false)
const rows = ref([])
const total = ref(0)
const page = ref(1)
const totalPages = ref(1)

const filters = reactive({
  warehouse_id: '',
  search: '',
  date_from: '',
  date_to: '',
})
const range = ref({ from: '', to: '', label: 'Semua Tanggal' })
watch(range, (r) => { filters.date_from = r.from; filters.date_to = r.to; resetAndLoad() })

const warehouseOptions = ref([{ id: '', name: 'Semua Gudang' }])


let loadSeq = 0
const load = async () => {
  const seq = ++loadSeq
  loading.value = true
  try {
    const res = await getWastes({
      ...filters,
      page: page.value,
      limit: 20
    })
    if (seq !== loadSeq) return // respons lama, sudah ada request lebih baru
    rows.value = res.data || []
    total.value = res.total || 0
    totalPages.value = res.totalPages || res.total_pages || 1

  } catch (err) {
    toast.error('Gagal mengambil data waste')
  } finally {
    loading.value = false
  }
}

// Ganti filter selalu mulai dari halaman 1 (halaman ini tanpa watch(page)).
const resetAndLoad = () => { page.value = 1; load() }

const debouncedLoad = debounce(resetAndLoad, 500)

const changePage = (p) => {
  page.value = p
  load()
}

const fetchMetadata = async () => {
  try {
    const data = await warehousesApi.list({ limit: 500 })
    const list = Array.isArray(data) ? data : data?.data || []
    warehouseOptions.value = [{ id: '', name: 'Semua Gudang' }, ...list.map(w => ({ id: w.id, name: w.name }))]
  } catch (e) { toast.error(e.message) }
}

const getReasonLabel = (reason) => {
  const labels = {
    void: 'Waste akibat pembatalan',
    reject: 'Reject',
    return: 'Retur',
    damaged: 'Rusak',
    expired: 'Kadaluarsa',
    spoiled: 'Basi/Busuk',
    lost: 'Hilang',
    other: 'Lainnya'
  }
  return labels[reason] || reason
}

const getReasonVariant = (reason) => {
  const variants = {
    void: 'warning',
    damaged: 'danger',
    expired: 'danger',
    spoiled: 'danger',
    lost: 'warning',
    other: 'secondary'
  }
  return variants[reason] || 'secondary'
}

const formatDate = (dateStr) => {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return d.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

onMounted(() => {
  load()
  fetchMetadata()
})
</script>
