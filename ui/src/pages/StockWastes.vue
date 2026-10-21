<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Pembuangan Stok (Waste)</h1>
      <AppButton @click="openCreateModal" variant="primary">
        <svg class="h-4 w-4 mr-1.5 -ml-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3m0 0v3m0-3h3m-3 0H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        Catat Pembuangan
      </AppButton>
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

    <!-- Create Modal -->
    <AppModal v-model="showCreateModal" title="Catat Pembuangan Stok (Waste)" size="lg">
      <form @submit.prevent="handleCreate" class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-700">Gudang</label>
            <SearchSelect
              v-model="form.warehouse_id"
              :options="warehouseOptions.filter(o => o.id !== '')"

              placeholder="Pilih Gudang"
              required
            />
          </div>
          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-700">Bahan Baku / Barang</label>
            <SearchSelect
              v-model="form.item_id"
              :options="itemOptions"
              placeholder="Pilih Barang"
              required
              @change="onItemChange"
            />
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-700">Jumlah Dibuang</label>
            <div class="flex gap-2">
              <input v-model.number="form.qty_dist" type="number" step="any" required min="0.001"
                class="flex-1 text-sm border border-gray-200 rounded-lg px-3 py-2 focus:ring-2 focus:ring-emerald-400 outline-none" />
              <select v-model="form.unit_used" class="w-32 text-sm border border-gray-200 rounded-lg px-2 outline-none bg-gray-50">
                <option v-for="u in unitOptions" :key="u" :value="u">{{ u }}</option>
              </select>
            </div>
            <p v-if="selectedItem" class="text-[11px] text-gray-500 mt-1">
              Konversi ke Satuan Dasar: 
              <span class="font-mono font-bold text-emerald-600">
                {{ (form.qty_dist * stockUnitRatio(selectedItem, form.unit_used)).toFixed(4) }} {{ selectedItem.base_unit }}
              </span>
            </p>
          </div>
          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-700">Alasan</label>
            <select v-model="form.reason" required
              class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:ring-2 focus:ring-emerald-400 outline-none bg-white">
              <option value="damaged">Rusak / Pecah</option>
              <option value="expired">Kadaluarsa (Expired)</option>
              <option value="spoiled">Basi / Busuk (Spoiled)</option>
              <option value="lost">Hilang / Selisih</option>
              <option value="other">Lainnya</option>
            </select>
          </div>
        </div>

        <div class="space-y-1">
          <label class="text-sm font-medium text-gray-700">Catatan</label>
          <textarea v-model="form.notes" rows="2"
            class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:ring-2 focus:ring-emerald-400 outline-none"
            placeholder="Opsional: detail kejadian..."></textarea>
        </div>

        <div class="flex justify-end gap-3 pt-2">
          <AppButton type="button" variant="secondary" @click="showCreateModal = false">Batal</AppButton>
          <AppButton type="submit" variant="primary" :loading="submitting">Simpan Catatan Waste</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive, computed, watch } from 'vue'
import { getWastes, createWaste } from '@/api/waste'
import { warehousesApi, stockItemsApi } from '@/api/warehouse'
import { useToastStore } from '@/stores/toast'
import { formatRupiah } from '@/utils/format'
import debounce from 'lodash/debounce'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppBadge      from '@/components/ui/AppBadge.vue'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import { preferredStockUnit, stockUnitRatio } from '@/utils/stockUnits'


const toast = useToastStore()

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
const submitting = ref(false)
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

const itemOptions = ref([])
const selectedItem = ref(null)

const showCreateModal = ref(false)
const form = reactive({
  warehouse_id: '',
  item_id: '',
  qty_dist: 1,
  unit_used: '',
  reason: 'damaged',
  notes: '',
})

const unitOptions = computed(() => {
  if (!selectedItem.value) return []
  const units = [selectedItem.value.base_unit]
  if (selectedItem.value.dist_unit && selectedItem.value.dist_unit !== selectedItem.value.base_unit) {
    units.push(selectedItem.value.dist_unit)
  }
  return units
})

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
    const [whRes, itemRes] = await Promise.all([
      warehousesApi.list({ limit: 500 }),
      stockItemsApi.list({ limit: 1000, active_only: true })
    ])
    
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = [
      { id: '', name: 'Semua Gudang' },
      ...whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})`, type: w.type }))
    ]

    const itemList = Array.isArray(itemRes) ? itemRes : (itemRes?.data || [])
    // ...i dulu supaya label gabungan "nama (kode)" tidak tertimpa nama polos —
    // pencarian per kode di picker bergantung pada label ini.
    itemOptions.value = itemList.map(i => ({
      ...i,
      id: i.id,
      name: `${i.name} (${i.code})`
    }))

  } catch (err) {
    console.error(err)
  }
}


const openCreateModal = () => {
  Object.assign(form, {
    warehouse_id: filters.warehouse_id || '',
    item_id: '',
    qty_dist: 1,
    unit_used: '',
    reason: 'damaged',
    notes: '',
  })
  selectedItem.value = null
  showCreateModal.value = true
}

const onItemChange = (val) => {
  const item = itemOptions.value.find(i => i.id === val)

  selectedItem.value = item
  if (item) {
    form.unit_used = preferredStockUnit(item, warehouseOptions.value.find(w => w.id === form.warehouse_id)?.type)
  }
}

const handleCreate = async () => {
  submitting.value = true
  try {
    await createWaste(form)
    toast.success('Catatan pembuangan berhasil disimpan')
    showCreateModal.value = false
    load()
  } catch (err) {
    toast.error(err.message || 'Gagal menyimpan data')

  } finally {
    submitting.value = false
  }
}

const getReasonLabel = (reason) => {
  const labels = {
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
