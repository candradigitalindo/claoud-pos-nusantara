<template>
  <div class="space-y-6">
        <RecipeCostSummary v-if="editingId && savedCost" :cost="savedCost" current-source="Harga terkini: rata-rata stok gudang induk, untuk komposisi yang sudah disimpan" />
        <p v-else class="text-xs text-gray-500">Harga bahan gudang induk dan komposisi awal direkam saat resep dibuat. Tautkan satu atau beberapa resep melalui Produk untuk menghitung HPP menu.</p>
    <div class="flex flex-wrap gap-3 items-center justify-between">
      <div>
        <h2 class="text-2xl font-bold text-gray-900 font-display tracking-tight">Resep Menu &amp; SOP</h2>
        <p class="text-sm text-gray-500">Pusat standarisasi resep dan SOP Dapur untuk seluruh outlet</p>
      </div>
      <AppButton v-if="canCreate" @click="openCreate" class="shadow-lg shadow-emerald-100">+ Buat Resep Baru</AppButton>
    </div>

    <!-- Filters -->
    <AppCard class="border-none shadow-sm bg-gray-50/50">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-2">
          <button
            v-for="v in VISIBILITIES" :key="v.value"
            @click="filterVisibility = v.value"
            :class="['px-4 py-2 rounded-xl text-sm font-bold transition-all',
              filterVisibility === v.value ? 'bg-emerald-600 text-white shadow-md' : 'bg-white text-gray-500 hover:bg-gray-100 border border-gray-100']"
          >
            {{ v.label }}
          </button>
        </div>
        <div class="flex flex-wrap items-center gap-3 flex-1 min-w-0 max-w-md">
          <SearchableSelect v-model="filterKind" aria-label="Kategori resep" class="border border-gray-200 rounded-xl text-sm px-2 py-2 min-w-0"><option value="">Makanan & minuman</option><option value="food">Makanan</option><option value="beverage">Minuman</option><option value="unclassified">Belum dikategorikan</option></SearchableSelect>
          <div class="relative flex-1 min-w-40">
            <svg class="absolute left-3 top-2.5 w-4 h-4 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
            <input v-model="search" placeholder="Cari nama resep..."
              class="w-full text-sm border border-gray-200 rounded-xl pl-10 pr-4 py-2.5 focus:outline-none focus:ring-2 focus:ring-emerald-400 bg-white" />
          </div>
        </div>
      </div>
    </AppCard>

    <!-- Master Recipe Table -->
    <AppCard :padding="false" class="overflow-hidden border-none shadow-sm">
      <AppTable :columns="COLUMNS" :rows="filteredRecipes" :loading="loading" emptyText="Belum ada master resep.">
        <template #cell-name="{ row }">
          <div class="cursor-pointer group" @click="openEdit(row)">
            <div class="font-bold text-gray-900 group-hover:text-emerald-700 transition-colors">{{ row.name }}</div>
            <div class="text-xs text-gray-400 truncate max-w-xs">{{ row.description || 'SOP & Resep standar' }}</div>
          </div>
        </template>
        <template #cell-menu_kind="{ row }"><span class="text-xs">{{ kindLabel(row.menu_kind) }}</span></template>
        <template #cell-visibility="{ row }">
          <span :class="row.visibility === 'public' ? 'bg-blue-50 text-blue-600 ring-1 ring-blue-100' : 'bg-amber-50 text-amber-600 ring-1 ring-amber-100'"
            class="text-[10px] px-2.5 py-1 rounded-full font-bold uppercase tracking-wider flex items-center w-fit gap-1">
            <svg v-if="row.visibility === 'public'" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
            <svg v-else class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
            {{ row.visibility === 'public' ? 'Public' : 'Rahasia' }}
          </span>
        </template>
        <template #cell-total_time="{ row }">
          <div class="flex items-center gap-1.5 text-gray-600 font-medium">
            <svg class="w-3.5 h-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>
            <span class="text-xs">{{ row.total_time || 0 }} menit</span>
          </div>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-xs text-gray-500">{{ formatDate(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <AppActionButton :icon="canUpdate ? 'edit' : 'view'" :label="canUpdate ? 'Edit Resep &amp; SOP' : 'Lihat Resep &amp; SOP'" @click="openEdit(row)" />
            <AppActionButton icon="delete" label="Hapus Resep" v-if="canDelete" @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- Recipe Modal (SOP Dapur) -->
    <AppModal v-model="showModal" :title="readOnly ? 'Detail Resep & SOP' : editingId ? 'Edit SOP & Resep Master' : 'Buat SOP & Resep Baru'" size="2xl">
      <div class="space-y-4">

        <fieldset :disabled="readOnly" class="space-y-4 min-w-0">
        <!-- Header Info -->
        <div class="modal-form-grid modal-form-grid--three">
          <div class="contents">
            <AppInput v-model="form.name" label="Nama Menu / Resep" placeholder="Contoh: Nasi Goreng Spesial" required />
            <div><label for="recipe-menu-kind" class="block text-sm font-medium text-gray-700 mb-1">Kategori F&B</label><SearchableSelect id="recipe-menu-kind" v-model="form.menu_kind" class="w-full border border-gray-200 rounded-lg p-2 text-sm"><option value="">Belum dikategorikan</option><option value="food">Makanan</option><option value="beverage">Minuman</option></SearchableSelect></div>
            <AppInput v-model="form.description" label="Keterangan Singkat" placeholder="Catatan standar porsi, dll." />
          </div>
          <div class="contents">
            <div>
              <label class="block text-sm font-bold text-gray-700 mb-1.5">Visibilitas Akses</label>
              <div class="grid grid-cols-2 gap-2">
                <button type="button" @click="form.visibility = 'public'"
                  :class="['py-2.5 text-xs font-bold border rounded-xl transition-all flex items-center justify-center gap-2',
                    form.visibility === 'public' ? 'bg-emerald-50 border-emerald-500 text-emerald-700 shadow-sm' : 'bg-white border-gray-200 text-gray-400']">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
                  Public
                </button>
                <button type="button" @click="form.visibility = 'secret'"
                  :class="['py-2.5 text-xs font-bold border rounded-xl transition-all flex items-center justify-center gap-2',
                    form.visibility === 'secret' ? 'bg-emerald-50 border-emerald-500 text-emerald-700 shadow-sm' : 'bg-white border-gray-200 text-gray-400']">
                  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
                  Rahasia
                </button>
              </div>
            </div>
            <div class="bg-emerald-50 p-3 rounded-xl border border-emerald-100 flex items-center justify-between">
               <div>
                 <label class="block text-[10px] uppercase font-bold text-emerald-600 mb-1">Estimasi Total Waktu</label>
                 <div class="flex items-center gap-2">
                    <span class="text-xl font-bold text-emerald-700 font-mono">{{ calculatedTotalTime }}</span>
                    <span class="text-xs font-bold text-emerald-600 uppercase">menit</span>
                 </div>
               </div>
               <svg class="w-8 h-8 text-emerald-200" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6l4 2"/></svg>
            </div>
          </div>
        </div>

        <!-- Secret Access -->
        <div v-if="form.visibility === 'secret'" class="p-4 bg-amber-50 rounded-2xl border border-amber-100 space-y-3">
          <label class="text-xs font-bold text-amber-800 uppercase tracking-widest flex items-center gap-2">
             Akses Khusus Outlet
          </label>
          <MultiSearchSelect v-if="!readOnly" v-model="form.outlet_ids" :options="outlets" placeholder="Pilih outlet..." />
          <p v-else class="text-xs">Resep tersedia untuk outlet yang diberi akses.</p>
        </div>
        </fieldset>

        <!-- Tabs -->
        <div class="flex gap-4 border-b border-gray-100">
          <button @click="activeTab = 'items'" :class="['pb-2 text-sm font-bold border-b-2 px-1', activeTab === 'items' ? 'border-emerald-500 text-emerald-700' : 'border-transparent text-gray-400']">Bahan Baku</button>
          <button @click="activeTab = 'steps'" :class="['pb-2 text-sm font-bold border-b-2 px-1', activeTab === 'steps' ? 'border-emerald-500 text-emerald-700' : 'border-transparent text-gray-400']">Langkah (SOP)</button>
        </div>

        <!-- Tab Content -->
        <div v-if="activeTab === 'items'" class="space-y-4">
          <div class="border rounded-xl overflow-x-auto">
            <table class="min-w-full text-xs">
              <thead class="bg-gray-50 text-gray-500 uppercase font-bold text-[10px]">
                <tr>
                  <th class="px-4 py-3 text-left">Bahan Baku (Raw)</th>
                  <th class="px-4 py-3 text-left w-32">Qty Pakai</th>
                  <th class="px-4 py-3 text-left w-24">Unit</th>
                  <th class="px-4 py-3 w-10"></th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-50">
                <tr v-for="(it, idx) in form.items" :key="idx" class="hover:bg-emerald-50/20 transition-colors">
                  <td class="px-4 py-2">
                    <SearchSelect
                      v-model="it.item_id"
                      :options="stockItemOptions"
                      label-key="label"
                      value-key="value"
                      placeholder="Pilih bahan..."
                      :disabled="readOnly"
                      @change="onItemChange(idx)"
                    />
                  </td>
                  <td class="px-4 py-2">
                    <input v-model.number="it.qty_base" :disabled="readOnly" type="number" min="0.0001" step="0.0001" class="w-full text-sm border-gray-100 focus:ring-emerald-400 rounded-lg py-1.5" placeholder="0" />
                  </td>
                  <td class="px-4 py-2 text-xs text-gray-500 font-bold uppercase tracking-tight">{{ it.unit || '—' }}</td>
                  <td class="px-4 py-2 text-center">
                    <AppActionButton icon="delete" label="Hapus bahan" v-if="!readOnly" @click="form.items.splice(idx, 1)" />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <button v-if="!readOnly" @click="addItem" class="text-xs font-bold text-emerald-600">+ Tambah Bahan</button>
        </div>

        <div v-if="activeTab === 'steps'" class="modal-form-grid">
          <div v-for="(step, idx) in form.instruction_steps" :key="idx" class="p-4 bg-gray-50 rounded-xl border border-gray-100 space-y-3">
             <div class="flex items-start gap-4">
                <div class="w-6 h-6 bg-emerald-600 text-white rounded-full flex items-center justify-center text-[10px] font-bold flex-shrink-0">{{ idx + 1 }}</div>
                <p v-if="readOnly" class="flex-1 text-sm whitespace-pre-line">{{ stepPlainText(step.text) }}</p>
                <QuillEditor v-else v-model="step.text" class="min-w-0 flex-1 bg-white" placeholder="Instruksi..." />
                <AppActionButton icon="delete" label="Hapus langkah" v-if="!readOnly" @click="form.instruction_steps.splice(idx, 1)" />
             </div>
             <div class="flex items-center gap-3 pl-10">
                <span class="text-[10px] font-bold text-gray-400 uppercase tracking-wider">Durasi Langkah:</span>
                <div class="flex items-center gap-1.5 bg-white border border-gray-200 rounded-lg px-2 py-1 shadow-sm focus-within:ring-2 focus-within:ring-emerald-400 transition-all">
                  <input v-model.number="step.duration" :disabled="readOnly" type="number" min="0" class="w-12 text-sm font-bold text-emerald-700 bg-transparent border-none p-0 focus:ring-0 text-center" />
                  <span class="text-[10px] text-gray-400 font-bold uppercase">Menit</span>
                </div>
             </div>
          </div>
          <button v-if="!readOnly" @click="addStep" class="modal-span-full justify-self-start text-xs font-bold text-emerald-600">+ Tambah Langkah</button>
        </div>

      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showModal = false">Batal</AppButton>
        <AppButton v-if="!readOnly" :loading="saving" @click="save">Simpan Resep & SOP</AppButton>
      </template>
    </AppModal>

    <!-- Delete Confirm -->
    <AppModal v-model="showDelete" title="Hapus Resep" size="sm">
      <p class="text-sm text-gray-600 leading-relaxed">Hapus resep <strong>{{ deleteTarget?.name }}</strong>?</p>
      <template #footer>
        <AppButton variant="secondary" @click="showDelete = false">Batal</AppButton>
        <AppButton variant="danger" :loading="saving" @click="doDelete">Hapus</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import RecipeCostSummary from '@/components/RecipeCostSummary.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, onMounted, computed } from 'vue'
import { recipeMastersApi, stockItemsApi } from '@/api/warehouse.js'
import { outletsApi } from '@/api/outlets.js'
import { formatDateTime } from '@/utils/format.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth'
import { kindLabel } from '@/utils/fnb'
import AppCard    from '@/components/ui/AppCard.vue'
import AppButton  from '@/components/ui/AppButton.vue'
import AppTable   from '@/components/ui/AppTable.vue'
import AppModal   from '@/components/ui/AppModal.vue'
import AppInput   from '@/components/ui/AppInput.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import MultiSearchSelect from '@/components/ui/MultiSearchSelect.vue'
import QuillEditor from '@/components/ui/QuillEditor.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = computed(() => auth.scopeType !== 'specific' && auth.hasPermission('recipes.create'))
const canUpdate = computed(() => auth.scopeType !== 'specific' && auth.hasPermission('recipes.update'))
const canDelete = computed(() => auth.scopeType !== 'specific' && auth.hasPermission('recipes.delete'))
const readOnly = computed(() => editingId.value ? !canUpdate.value : !canCreate.value)
const filterKind = ref('')
function stepPlainText(html) { return new DOMParser().parseFromString(html || '', 'text/html').body.textContent || '' }
const VISIBILITIES = [
  { label: 'Semua Resep', value: '' },
  { label: 'Public', value: 'public' },
  { label: 'Rahasia', value: 'secret' },
]

const COLUMNS = [
  { key: 'name',       label: 'Resep & Menu' },
  { key: 'menu_kind', label: 'Kategori' },
  { key: 'total_time', label: 'Timeline' },
  { key: 'visibility', label: 'Akses' },
  { key: 'created_at', label: 'Dibuat' },
  { key: 'actions',    label: '' },
]

const loading = ref(false)
const saving = ref(false)
const search = ref('')
const filterVisibility = ref('')
const recipes = ref([])
const outlets = ref([])
const allStockItems = ref([])
const activeTab = ref('items')

const calculatedTotalTime = computed(() => {
  return form.value.instruction_steps.reduce((acc, s) => acc + (Number(s.duration) || 0), 0)
})

const filteredRecipes = computed(() => {
  return recipes.value.filter(r => {
    const mName = !search.value || r.name.toLowerCase().includes(search.value.toLowerCase())
    const mVis = !filterVisibility.value || r.visibility === filterVisibility.value
    const mKind = !filterKind.value || r.menu_kind === filterKind.value || (filterKind.value === 'unclassified' && !r.menu_kind)
    return mName && mVis && mKind
  })
})

const stockItemOptions = computed(() =>
  allStockItems.value.map(i => ({ value: i.id, label: `${i.name} (${i.code})` }))
)

async function fetchAll() {
  loading.value = true
  try {
    const [resR, resO, resS] = await Promise.all([
      recipeMastersApi.list(),
      outletsApi.myOutlets(),
      stockItemsApi.list({ limit: 1000, active_only: true })
    ])
    // apiClient meng-unwrap envelope {success,data} → responsnya array langsung.
    recipes.value = Array.isArray(resR) ? resR : (resR?.data || [])
    outlets.value = resO.outlets || resO || []
    allStockItems.value = resS.data || []
  } catch (e) {
    toast.error('Gagal memuat data')
  } finally {
    loading.value = false
  }
}

// Modal & Form
const showModal = ref(false)
const editingId = ref(null)
const savedCost = ref(null)
const form = ref({
  name: '', description: '', visibility: 'public',
  items: [], outlet_ids: [], total_time: 0, instruction_steps: []
})

function openCreate() {
  if (!canCreate.value) return
  editingId.value = null
  savedCost.value = null
  activeTab.value = 'items'
  form.value = {
    name: '', description: '', visibility: 'public', menu_kind: '',
    items: [{ item_id: '', qty_base: 1, unit: '' }],
    outlet_ids: [], total_time: 0, instruction_steps: [{ text: '', duration: 0 }]
  }
  showModal.value = true
}

async function openEdit(row) {
  activeTab.value = 'items'
  try {
    const res = await recipeMastersApi.get(row.id)
    // apiClient sudah meng-unwrap envelope → res adalah objek resepnya.
    const data = res?.data ?? res
    editingId.value = data.id
    savedCost.value = data

    let steps = []
    try { steps = JSON.parse(data.instructions) } catch { steps = [] }

    form.value = {
      name: data.name,
      menu_kind: data.menu_kind || '',
      description: data.description,
      visibility: data.visibility,
      total_time: data.total_time,
      items: (data.items || []).map(it => ({ item_id: it.item_id, qty_base: it.qty_base, unit: it.unit })),
      outlet_ids: data.outlet_ids || [],
      instruction_steps: steps.length ? steps : [{ text: '', duration: 0 }]
    }
    showModal.value = true
  } catch (e) {
    toast.error('Gagal memuat detail resep')
  }
}

function addItem() {
  form.value.items.push({ item_id: '', qty_base: 1, unit: '' })
}

function addStep() {
  form.value.instruction_steps.push({ text: '', duration: 0 })
}

function onItemChange(idx) {
  const it = form.value.items[idx]
  const found = allStockItems.value.find(i => i.id === it.item_id)
  if (found) it.unit = found.base_unit
}

async function save() {
  if (readOnly.value || saving.value) return
  if (!form.value.name) return toast.error('Nama resep wajib diisi')
  const validItems = form.value.items
  if (!validItems.length || validItems.some(it => !it.item_id || !Number.isFinite(Number(it.qty_base)) || !(it.qty_base > 0))) return toast.error('Isi bahan dan takaran positif pada setiap baris')
  if (new Set(validItems.map(it => it.item_id)).size !== validItems.length) return toast.error('Bahan yang sama cukup ditulis satu kali')

  saving.value = true
  try {
    await recipeMastersApi.save(editingId.value, {
      ...form.value,
      total_time: calculatedTotalTime.value,
      instructions: JSON.stringify(form.value.instruction_steps),
      items: validItems
    })
    toast.success('Resep & SOP berhasil disimpan')
    showModal.value = false
    fetchAll()
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan resep')
  } finally {
    saving.value = false
  }
}

// Delete
const showDelete = ref(false)
const deleteTarget = ref(null)
function confirmDelete(row) {
  if (!canDelete.value) return
  deleteTarget.value = row
  showDelete.value = true
}
async function doDelete() {
  if (!canDelete.value || saving.value) return
  saving.value = true
  try {
    await recipeMastersApi.delete(deleteTarget.value.id)
    toast.success('Resep dihapus')
    showDelete.value = false
    fetchAll()
  } catch (e) {
    toast.error('Gagal menghapus')
  } finally {
    saving.value = false
  }
}

const formatDate = (d) => formatDateTime(d).slice(0, 10)

onMounted(fetchAll)
</script>
