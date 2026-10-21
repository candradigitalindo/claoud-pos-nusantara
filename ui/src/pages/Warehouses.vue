<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Gudang</h1>
      <AppButton @click="openCreate">+ Tambah Gudang</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[220px]">
          <SearchSelect
            v-model="filterType"
            :options="warehouseTypeOptions"
            placeholder="Semua Tipe"
            searchPlaceholder="Cari tipe gudang..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
      </div>
      <!-- Mobile cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!warehouses.length" class="p-6 text-center text-sm text-gray-400">Belum ada gudang.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in warehouses" :key="row.id" class="p-4 flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium text-gray-900 break-words">{{ row.name }}</p>
              <p class="mt-1 flex flex-wrap items-center gap-1.5">
                <span class="font-mono text-xs bg-gray-100 px-2 py-0.5 rounded">{{ row.code }}</span>
                <span :class="row.type === 'central' ? 'bg-blue-100 text-blue-700' : 'bg-amber-100 text-amber-700'" class="text-xs font-medium px-2 py-0.5 rounded-full">{{ row.type === 'central' ? 'Gudang Induk' : 'Gudang Outlet' }}</span>
                <span :class="row.is_active ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-500'" class="text-xs font-medium px-2 py-0.5 rounded-full">{{ row.is_active ? 'Aktif' : 'Nonaktif' }}</span>
              </p>
              <p v-if="row.outlet_name" class="text-xs text-gray-500 mt-1">{{ row.outlet_name }}</p>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <AppActionButton icon="edit" label="Edit" @click="openEdit(row)" />
              <AppActionButton icon="delete" label="Hapus" @click="confirmDelete(row)" />
            </div>
          </li>
        </ul>
      </div>
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="warehouses" :loading="loading" emptyText="Belum ada gudang.">
        <template #cell-code="{ row }">
          <span class="font-mono text-xs bg-gray-100 px-2 py-0.5 rounded">{{ row.code }}</span>
        </template>
        <template #cell-type="{ row }">
          <span :class="row.type === 'central' ? 'bg-blue-100 text-blue-700' : 'bg-amber-100 text-amber-700'"
            class="text-xs font-medium px-2 py-0.5 rounded-full">
            {{ row.type === 'central' ? 'Gudang Induk' : 'Gudang Outlet' }}
          </span>
        </template>
        <template #cell-outlet_name="{ row }">
          <span v-if="row.outlet_name" class="text-gray-900">{{ row.outlet_name }}</span>
          <span v-else class="text-gray-400 italic">—</span>
        </template>
        <template #cell-is_active="{ row }">
          <span :class="row.is_active ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-500'"
            class="text-xs font-medium px-2 py-0.5 rounded-full">
            {{ row.is_active ? 'Aktif' : 'Nonaktif' }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <AppActionButton icon="edit" label="Edit" @click="openEdit(row)" />
            <AppActionButton icon="delete" label="Hapus" @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
      <AppPagination v-model:page="page" :total-pages="totalPages" class="px-4 py-3 border-t border-gray-100" />
    </AppCard>

    <!-- Create / Edit Modal -->
    <AppModal v-model="showForm" :title="editTarget ? 'Edit Gudang' : 'Tambah Gudang'" size="sm">
      <div class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <AppInput v-model="form.code" label="Kode" placeholder="Contoh: GDG-INDUK" />
          <AppInput v-model="form.name" label="Nama Gudang" placeholder="Contoh: Gudang Pusat" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Tipe Gudang</label>
          <div class="grid grid-cols-2 gap-2">
            <label v-for="t in TYPES" :key="t.value"
              :class="['border rounded-lg p-3 cursor-pointer transition-all text-sm',
                form.type === t.value ? 'border-emerald-400 bg-emerald-50 text-emerald-800 font-medium' : 'border-gray-200 hover:border-gray-300']"
              @click="form.type = t.value; if(t.value==='central') form.outlet_id=''">
              <div class="font-medium">{{ t.label }}</div>
              <div class="text-xs text-gray-500 mt-0.5">{{ t.desc }}</div>
            </label>
          </div>
        </div>
        <div v-if="form.type === 'outlet'" class="space-y-1">
          <label class="block text-sm font-medium text-gray-700">Outlet Terkait</label>
          <SearchSelect
            v-model="form.outlet_id"
            :options="outletOptions"
            placeholder="Pilih outlet..."
            searchPlaceholder="Cari outlet..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <p v-if="form.type === 'outlet'" class="text-xs text-gray-500 -mt-1">
          Setiap outlet hanya boleh memiliki satu gudang outlet. Gudang induk A dan B tetap bisa sama-sama transfer ke gudang outlet yang sama, dan transfer antar gudang outlet juga bisa.
        </p>
        <AppInput v-model="form.notes" label="Keterangan" placeholder="Opsional" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showForm = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitForm">Simpan</AppButton>
      </template>
    </AppModal>

    <!-- Delete confirm -->
    <AppModal v-model="showDelete" title="Hapus Gudang" size="sm">
      <p class="text-sm text-gray-600">Hapus gudang <strong>{{ deleteTarget?.name }}</strong>?</p>
      <template #footer>
        <AppButton variant="secondary" @click="showDelete = false">Batal</AppButton>
        <AppButton variant="danger" :loading="saving" @click="submitDelete">Hapus</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, watch, onMounted } from 'vue'
import { warehousesApi } from '@/api/warehouse.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const warehouses = ref([])
const page = ref(1)
const limit = 20
const totalPages = ref(1)
const filterType = ref('')

const showForm = ref(false)
const showDelete = ref(false)
const editTarget = ref(null)
const deleteTarget = ref(null)
const outletOptions = ref([])

const TYPES = [
  { value: 'central', label: 'Gudang Induk', desc: 'Bisa lebih dari satu dan dapat memasok outlet mana pun' },
  { value: 'outlet',  label: 'Gudang Outlet', desc: 'Satu outlet satu gudang outlet, bisa terima pasokan dari banyak gudang dan transfer antar outlet' },
]
const warehouseTypeOptions = [
  { value: '', label: 'Semua Tipe' },
  ...TYPES.map(t => ({ value: t.value, label: t.label })),
]

const EMPTY_FORM = () => ({ code: '', name: '', type: 'central', outlet_id: '', notes: '' })
const form = ref(EMPTY_FORM())

const COLUMNS = [
  { key: 'code',        label: 'Kode',       sortable: false },
  { key: 'name',        label: 'Nama',       sortable: false },
  { key: 'type',        label: 'Tipe',       sortable: false },
  { key: 'outlet_name', label: 'Outlet',     sortable: false },
  { key: 'notes',       label: 'Keterangan', sortable: false },
  { key: 'is_active',   label: 'Status',     sortable: false },
  { key: 'actions',     label: '',           sortable: false },
]

let loadSeq = 0
async function load() {
  const seq = ++loadSeq
  loading.value = true
  errorMsg.value = ''
  try {
    const data = await warehousesApi.list({ page: page.value, limit, type: filterType.value })
    if (seq !== loadSeq) return // respons lama, sudah ada request lebih baru
    warehouses.value = data.data || []
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

async function loadOutlets() {
  try {
    const data = await outletsApi.list({ limit: 200 })
    const outlets = data.outlets || data.data || data || []
    outletOptions.value = outlets.map(o => ({ value: o.id, label: o.name }))
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat daftar outlet')
  }
}

watch(page, load)
onMounted(() => { load(); loadOutlets() })

function openCreate() {
  editTarget.value = null
  form.value = EMPTY_FORM()
  showForm.value = true
}
function openEdit(row) {
  editTarget.value = row
  form.value = { code: row.code, name: row.name, type: row.type, outlet_id: row.outlet_id || '', notes: row.notes }
  showForm.value = true
}
function confirmDelete(row) {
  deleteTarget.value = row
  showDelete.value = true
}

async function submitForm() {
  if (form.value.type === 'outlet' && !form.value.outlet_id) {
    toast.error('Gudang outlet wajib memilih outlet')
    return
  }
  saving.value = true
  try {
    if (editTarget.value) {
      await warehousesApi.update(editTarget.value.id, form.value)
      toast.success('Gudang diperbarui')
    } else {
      await warehousesApi.create(form.value)
      toast.success('Gudang ditambahkan')
    }
    showForm.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan')
  } finally {
    saving.value = false
  }
}

async function submitDelete() {
  saving.value = true
  try {
    await warehousesApi.delete(deleteTarget.value.id)
    toast.success('Gudang dihapus')
    showDelete.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menghapus')
  } finally {
    saving.value = false
  }
}
</script>
