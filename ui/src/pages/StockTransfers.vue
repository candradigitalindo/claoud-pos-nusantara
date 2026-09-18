<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Transfer Stok</h1>
      <AppButton @click="openCreate">+ Buat Transfer</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[180px]">
          <SearchSelect
            v-model="filterStatus"
            :options="statusOptions"
            placeholder="Semua Status"
            searchPlaceholder="Cari status..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
        <div class="min-w-[260px] flex-1 max-w-md">
          <SearchSelect
            v-model="filterWarehouse"
            :options="warehouseFilterOptions"
            placeholder="Semua Gudang"
            searchPlaceholder="Cari gudang..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
      </div>

      <AppTable :columns="COLUMNS" :rows="transfers" :loading="loading" emptyText="Belum ada transfer.">
        <template #cell-transfer_number="{ row }">
          <button @click="openDetail(row.id)" class="font-mono text-sm text-emerald-600 hover:text-emerald-800 hover:underline">
            {{ row.transfer_number }}
          </button>
        </template>
        <template #cell-route="{ row }">
          <div class="text-sm">
            <span class="font-medium">{{ row.from_warehouse }}</span>
            <span class="text-gray-400 mx-1.5">→</span>
            <span class="font-medium">{{ row.to_warehouse }}</span>
          </div>
        </template>
        <template #cell-status="{ row }">
          <span :class="statusCls(row.status)" class="text-xs font-medium px-2 py-0.5 rounded-full">
            {{ STATUS_LABELS[row.status] || row.status }}
          </span>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-sm text-gray-500">{{ row.created_at.slice(0,10) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <button @click="openDetail(row.id)" class="text-xs text-emerald-600 hover:text-emerald-800 px-2 py-1 rounded hover:bg-emerald-50">Detail</button>
        </template>
      </AppTable>
      <AppPagination v-model:page="page" :total-pages="totalPages" class="px-4 py-3 border-t border-gray-100" />
    </AppCard>

    <!-- Create Transfer Modal -->
    <AppModal v-model="showCreate" title="Buat Transfer Stok" size="lg">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Gudang Asal</label>
            <SearchSelect
              v-model="form.from_warehouse_id"
              :options="fromWarehouseOptions"
              placeholder="Pilih gudang asal..."
              searchPlaceholder="Cari gudang asal..."
              valueKey="value"
              labelKey="label"
            />
          </div>
          <div class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Gudang Tujuan</label>
            <SearchSelect
              v-model="form.to_warehouse_id"
              :options="toWarehouseOptions"
              placeholder="Pilih gudang tujuan..."
              searchPlaceholder="Cari gudang tujuan..."
              valueKey="value"
              labelKey="label"
            />
          </div>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-400">Asal</div>
            <div class="mt-1 text-sm font-semibold text-gray-900">{{ selectedFromWarehouse?.name || 'Belum dipilih' }}</div>
            <div class="text-xs text-gray-500">{{ describeWarehouse(selectedFromWarehouse) }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-400">Tujuan</div>
            <div class="mt-1 text-sm font-semibold text-gray-900">{{ selectedToWarehouse?.name || 'Belum dipilih' }}</div>
            <div class="text-xs text-gray-500">{{ describeWarehouse(selectedToWarehouse) }}</div>
          </div>
        </div>
        <p class="text-xs text-gray-500 -mt-1">
          Transfer stok mendukung antar gudang induk, gudang induk ke outlet, dan antar gudang outlet.
        </p>
        <AppInput v-model="form.notes" label="Keterangan" placeholder="Opsional" />

        <!-- Items -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm font-medium text-gray-700">Daftar Item</span>
            <button @click="addItem" class="text-xs text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Item</button>
          </div>
          <div v-if="form.items.length === 0" class="text-sm text-gray-400 py-3 text-center border border-dashed border-gray-200 rounded-lg">
            Belum ada item. Klik "+ Tambah Item".
          </div>
          <div v-for="(it, i) in form.items" :key="i" class="flex items-end gap-2 mb-2">
            <div class="flex-1">
              <label class="block text-xs text-gray-500 mb-1">Bahan Baku</label>
              <SearchSelect
                v-model="it.item_id"
                :options="stockItemOptions"
                placeholder="Pilih bahan baku..."
                searchPlaceholder="Cari bahan baku..."
                valueKey="value"
                labelKey="label"
                @change="onItemSelect(it)"
              />
            </div>
            <div class="w-28">
              <label class="block text-xs text-gray-500 mb-1">Qty ({{ it.unit_label || 'unit' }})</label>
              <input v-model.number="it.qty_dist" type="number" step="0.01" min="0"
                class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
            </div>
            <div class="w-28 text-xs text-gray-400 pb-2">
              <span v-if="it.qty_dist && it.dist_ratio">
                = {{ (it.qty_dist * it.dist_ratio).toFixed(3) }} {{ it.base_unit }}
              </span>
            </div>
            <button @click="removeItem(i)" class="pb-2 text-red-400 hover:text-red-600 text-lg leading-none">×</button>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitCreate">Buat Transfer</AppButton>
      </template>
    </AppModal>

    <!-- Detail Modal -->
    <AppModal v-model="showDetail" :title="`Transfer ${detail?.transfer_number || ''}`" size="lg">
      <div v-if="detail" class="space-y-4">
        <!-- Status flow -->
        <div class="flex items-center gap-2 overflow-x-auto pb-1">
          <div v-for="(s, i) in STATUSES" :key="s.value" class="flex items-center gap-2">
            <div :class="['px-3 py-1 rounded-full text-xs font-medium',
              detail.status === s.value ? statusCls(s.value) : 'bg-gray-100 text-gray-400']">
              {{ s.label }}
            </div>
            <span v-if="i < STATUSES.length - 1" class="text-gray-300">›</span>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4 text-sm">
          <div><span class="text-gray-500">Dari:</span> <strong>{{ detail.from_warehouse }}</strong></div>
          <div><span class="text-gray-500">Ke:</span> <strong>{{ detail.to_warehouse }}</strong></div>
          <div v-if="detail.approved_by"><span class="text-gray-500">Disetujui:</span> {{ detail.approved_by }} · {{ detail.approved_at?.slice(0,10) }}</div>
          <div v-if="detail.sent_by"><span class="text-gray-500">Dikirim:</span> {{ detail.sent_by }} · {{ detail.sent_at?.slice(0,10) }}</div>
          <div v-if="detail.received_by"><span class="text-gray-500">Diterima:</span> {{ detail.received_by }} · {{ detail.received_at?.slice(0,10) }}</div>
        </div>

        <!-- Items table -->
        <table class="min-w-full text-sm border border-gray-100 rounded-lg overflow-hidden">
          <thead class="bg-gray-50 text-xs text-gray-500">
            <tr>
              <th class="px-3 py-2 text-left">Bahan Baku</th>
              <th class="px-3 py-2 text-right">Dikirim</th>
              <th v-if="isReceiving" class="px-3 py-2 text-right">Diterima (input)</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="it in detail.items" :key="it.id" class="hover:bg-gray-50">
              <td class="px-3 py-2 font-medium">{{ it.item_name }}</td>
              <td class="px-3 py-2 text-right">
                {{ it.qty_dist }} {{ it.dist_unit_label || it.dist_unit }}
                <span class="text-gray-400">({{ it.qty_base.toFixed(3) }} {{ it.base_unit }})</span>
              </td>
              <td v-if="isReceiving" class="px-3 py-2 text-right">
                <input v-model.number="it._received_qty" type="number" step="0.001"
                  min="0"
                  :max="it.qty_base"
                  :placeholder="it.qty_base.toFixed(3)"
                  @blur="saveReceivedQty(it)"
                  class="w-24 text-sm border border-gray-200 rounded px-2 py-1 text-right focus:outline-none focus:ring-1 focus:ring-emerald-400" />
              </td>
            </tr>
          </tbody>
        </table>

        <p v-if="detail.notes" class="text-sm text-gray-500">{{ detail.notes }}</p>

        <!-- Bukti barang berangkat dari gudang asal. Server menolak status 'sent'
             tanpa foto — sebelumnya tombol ini tidak pernah mengirimkannya. -->
        <div v-if="isSending" class="rounded-xl border border-emerald-200 bg-emerald-50/50 p-3">
          <PhotoCapture v-model="sendPhoto" label="Foto Barang Saat Dikirim"
            hint="Potret barangnya saat dimuat/berangkat dari gudang asal." />
        </div>
        <div v-else-if="detail.photo_url" class="flex items-center gap-3">
          <img :src="detail.photo_url" alt="Bukti kirim" class="h-20 w-20 rounded-lg border border-gray-200 object-cover" />
          <p class="text-xs text-gray-500">Foto saat dikirim dari {{ detail.from_warehouse_name }}.</p>
        </div>

        <!-- Actions: tombol hanya muncul bila status DAN izin memungkinkan. -->
        <div v-if="nextAction" class="flex justify-end">
          <AppButton :loading="saving" @click="doAction">{{ nextAction.label }}</AppButton>
        </div>
        <div v-if="(detail.status === 'draft' || detail.status === 'approved') && canSend">
          <button @click="doCancel" :disabled="saving" class="text-xs text-red-500 hover:text-red-700">Batalkan Transfer</button>
        </div>
      </div>
      <div v-else class="py-8 text-center text-gray-400">Memuat...</div>
      <template #footer>
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { warehousesApi, stockItemsApi, stockTransfersApi } from '@/api/warehouse.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import PhotoCapture  from '@/components/PhotoCapture.vue'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import { describeWarehouse, formatWarehouseOptionLabel, TRANSFER_STATUS_LABELS, transferStatusClass } from '@/utils/warehouse.js'

const toast = useToastStore()
const auth = useAuthStore()
// Setujui, kirim/batalkan, dan terima adalah tiga izin berbeda — sama
// dengan mutasi aset. Tombol yang izinnya tidak ada tidak ditampilkan.
const canApprove = auth.hasPermission('stocktransfers.approve')
const canSend    = auth.hasPermission('stocktransfers.update')
const canReceive = auth.hasPermission('stocktransfers.receive')
const sendPhoto = ref('')
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const transfers = ref([])
const page = ref(1)
const limit = 20
const totalPages = ref(1)
const filterStatus = ref('')
const filterWarehouse = ref('')
const warehouseList = ref([])
const stockItemOptions = ref([])
const stockItemMap = ref({})

const showCreate = ref(false)
const showDetail = ref(false)
const detail = ref(null)

const STATUSES = ['draft', 'approved', 'sent', 'received', 'cancelled']
  .map(v => ({ value: v, label: TRANSFER_STATUS_LABELS[v] }))
const statusOptions = [
  { value: '', label: 'Semua Status' },
  ...STATUSES,
]
const STATUS_LABELS = TRANSFER_STATUS_LABELS

const NEXT_STATUS = { draft: 'approved', approved: 'sent', sent: 'received' }
const NEXT_LABELS = { draft: 'Setujui Transfer', approved: 'Tandai Sudah Dikirim', sent: 'Konfirmasi Penerimaan' }

const COLUMNS = [
  { key: 'transfer_number', label: 'No. Transfer', sortable: false },
  { key: 'route',           label: 'Rute',         sortable: false },
  { key: 'status',          label: 'Status',       sortable: false },
  { key: 'created_at',      label: 'Tanggal',      sortable: false },
  { key: 'created_by',      label: 'Dibuat oleh',  sortable: false },
  { key: 'actions',         label: '',             sortable: false },
]

const statusCls = transferStatusClass

const NEXT_PERM = { approved: canApprove, sent: canSend, received: canReceive }
const nextAction = computed(() => {
  if (!detail.value) return null
  const ns = NEXT_STATUS[detail.value.status]
  if (!ns || !NEXT_PERM[ns]) return null
  return { status: ns, label: NEXT_LABELS[detail.value.status] }
})
const isSending = computed(() => detail.value?.status === 'approved' && canSend)
const isReceiving = computed(() => detail.value?.status === 'sent' && canReceive)

const EMPTY_FORM = () => ({ from_warehouse_id: '', to_warehouse_id: '', notes: '', items: [] })
const form = ref(EMPTY_FORM())

const warehouseMap = computed(() => Object.fromEntries(warehouseList.value.map(w => [w.id, w])))

const warehouseFilterOptions = computed(() =>
  [
    { value: '', label: 'Semua Gudang' },
    ...warehouseList.value.map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) })),
  ]
)

const fromWarehouseOptions = computed(() =>
  warehouseList.value
    .filter(w => w.id !== form.value.to_warehouse_id)
    .map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) }))
)

const toWarehouseOptions = computed(() =>
  warehouseList.value
    .filter(w => w.id !== form.value.from_warehouse_id)
    .map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) }))
)

const selectedFromWarehouse = computed(() => warehouseMap.value[form.value.from_warehouse_id] || null)
const selectedToWarehouse = computed(() => warehouseMap.value[form.value.to_warehouse_id] || null)

let loadSeq = 0
async function load() {
  const seq = ++loadSeq
  loading.value = true
  errorMsg.value = ''
  try {
    const data = await stockTransfersApi.list({ page: page.value, limit, status: filterStatus.value, warehouse_id: filterWarehouse.value })
    if (seq !== loadSeq) return // respons lama, sudah ada request lebih baru
    transfers.value = data.data || []
    totalPages.value = data.total_pages || 1
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat data'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  // watch(page) yang memanggil load; hindari request ganda saat pindah ke hal. 1
  if (page.value === 1) load()
  else page.value = 1
}

async function loadWarehouses() {
  try {
    const data = await warehousesApi.list({ limit: 200 })
    warehouseList.value = data.data || []
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat daftar gudang')
  }
}

async function loadStockItems() {
  try {
    const data = await stockItemsApi.list({ limit: 500, active_only: 'true' })
    const items = data.data || []
    stockItemMap.value = Object.fromEntries(items.map(s => [s.id, s]))
    stockItemOptions.value = items.map(s => ({
      value: s.id, label: `${s.code} — ${s.name}`
    }))
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat daftar bahan baku')
  }
}

watch(page, load)
watch(() => form.value.from_warehouse_id, (nextValue) => {
  if (nextValue && nextValue === form.value.to_warehouse_id) {
    form.value.to_warehouse_id = ''
  }
})
watch(() => form.value.to_warehouse_id, (nextValue) => {
  if (nextValue && nextValue === form.value.from_warehouse_id) {
    form.value.from_warehouse_id = ''
  }
})
onMounted(() => { load(); loadWarehouses(); loadStockItems() })

function addItem() {
  form.value.items.push({ item_id: '', qty_dist: 0, notes: '', dist_ratio: 1, base_unit: '', unit_label: '' })
}
function removeItem(i) {
  form.value.items.splice(i, 1)
}
function onItemSelect(it) {
  const s = stockItemMap.value[it.item_id]
  if (s) {
    it.dist_ratio = s.dist_ratio
    it.base_unit = s.base_unit
    it.unit_label = s.dist_unit_label || s.dist_unit
  }
}

function openCreate() {
  form.value = EMPTY_FORM()
  showCreate.value = true
}

async function submitCreate() {
  if (!form.value.from_warehouse_id || !form.value.to_warehouse_id) {
    toast.error('Pilih gudang asal dan tujuan')
    return
  }
  if (form.value.from_warehouse_id === form.value.to_warehouse_id) {
    toast.error('Gudang asal dan tujuan tidak boleh sama')
    return
  }
  if (form.value.items.length === 0) {
    toast.error('Tambahkan minimal 1 item')
    return
  }
  if (form.value.items.some(it => !it.item_id || !it.qty_dist || it.qty_dist <= 0)) {
    toast.error('Setiap item transfer harus memiliki bahan baku dan qty lebih dari 0')
    return
  }
  saving.value = true
  try {
    await stockTransfersApi.create(form.value)
    toast.success('Transfer berhasil dibuat')
    showCreate.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan')
  } finally {
    saving.value = false
  }
}

async function openDetail(id) {
  detail.value = null
  sendPhoto.value = ''
  showDetail.value = true
  try {
    const data = await stockTransfersApi.get(id)
    detail.value = data
    detail.value.items.forEach(it => {
      it._received_qty = (it.received_qty_base !== null && it.received_qty_base !== undefined)
        ? it.received_qty_base
        : it.qty_base
    })
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat detail')
    showDetail.value = false
  }
}

async function saveReceivedQty(it) {
  if (it._received_qty === undefined || it._received_qty === null || !detail.value || detail.value.status !== 'sent') return
  const previousQty = it.received_qty_base ?? it.qty_base
  const nextQty = Number(it._received_qty)
  if (Number.isNaN(nextQty)) {
    it._received_qty = previousQty
    return
  }
  if (nextQty < 0) {
    toast.error('Qty diterima tidak boleh negatif')
    it._received_qty = previousQty
    return
  }
  if (nextQty > it.qty_base) {
    toast.error('Qty diterima tidak boleh melebihi qty dikirim')
    it._received_qty = previousQty
    return
  }
  try {
    await stockTransfersApi.updateReceivedQty(detail.value.id, it.item_id, nextQty)
    it.received_qty_base = nextQty
  } catch (e) {
    it._received_qty = previousQty
    toast.error(e?.message || 'Gagal menyimpan qty diterima')
  }
}

async function doAction() {
  if (!nextAction.value) return
  if (nextAction.value.status === 'sent' && !sendPhoto.value) {
    toast.error('Foto barang saat dikirim wajib diunggah')
    return
  }
  saving.value = true
  try {
    const data = await stockTransfersApi.updateStatus(detail.value.id, nextAction.value.status, sendPhoto.value)
    detail.value = data
    sendPhoto.value = ''
    toast.success('Status transfer diperbarui')
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui status')
  } finally {
    saving.value = false
  }
}

async function doCancel() {
  saving.value = true
  try {
    const data = await stockTransfersApi.updateStatus(detail.value.id, 'cancelled')
    detail.value = data
    toast.success('Transfer dibatalkan')
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal membatalkan')
  } finally {
    saving.value = false
  }
}
</script>
