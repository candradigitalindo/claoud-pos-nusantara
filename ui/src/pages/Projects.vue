<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Projek</h1>
        <p class="text-xs text-gray-500 mt-0.5">Pembangunan &amp; renovasi — RAB disusun per baris pekerjaan, ditetapkan, lalu dibelanjakan bertahap lewat pengadaan.</p>
      </div>
      <AppButton v-if="canManage" @click="openCreate">+ Buat Projek</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters -->
    <AppCard>
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-4">
        <div class="flex w-full flex-col gap-1 sm:w-auto sm:min-w-[220px]">
          <label class="text-sm font-medium text-gray-700">Cari</label>
          <input v-model="search" @keyup.enter="fetchList" placeholder="Nama projek / nomor / PIC…"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500" />
        </div>
        <div class="flex w-full flex-col gap-1 sm:w-auto">
          <label class="text-sm font-medium text-gray-700">Status</label>
          <select v-model="filterStatus"
            class="w-full sm:w-auto rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="">Semua Status</option>
            <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
        <button @click="fetchList"
          class="w-full sm:w-auto px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
      </div>
    </AppCard>

    <!-- Ringkasan seluruh projek yang tampil -->
    <div v-if="projects.length" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div v-for="t in totals" :key="t.label" class="rounded-xl bg-white px-4 py-3 shadow-sm ring-1 ring-gray-100">
        <p class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">{{ t.label }}</p>
        <p class="mt-1 text-base sm:text-lg font-bold" :class="t.cls">{{ formatRupiah(t.value) }}</p>
      </div>
    </div>

    <AppCard :padding="false">
      <!-- Mobile: kartu -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!projects.length" class="p-6 text-center text-sm text-gray-400">Belum ada projek.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="p in projects" :key="p.id" class="p-4">
            <div class="flex items-start justify-between gap-2">
              <button @click="goDetail(p)" class="min-w-0 flex-1 text-left">
                <p class="font-mono text-[11px] text-gray-500">{{ p.project_number || '-' }}</p>
                <p class="mt-0.5 text-sm font-medium text-gray-900 break-words">{{ p.name }}</p>
              </button>
              <span :class="statusBadge(p.status)" class="shrink-0">{{ statusLabel(p.status) }}</span>
            </div>
            <dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Unit</dt>
              <dd class="font-medium text-gray-800">{{ p.work_unit_name || '-' }}</dd>
              <dt class="text-gray-400">PIC</dt>
              <dd class="text-gray-700">{{ p.pic || '-' }}</dd>
              <dt class="text-gray-400">Belanja</dt>
              <dd class="text-gray-700">{{ p.request_count }} pengajuan</dd>
              <dt class="text-gray-400">RAB</dt>
              <dd><span :class="rabBadge(p)">{{ rabLabel(p) }}</span> <span class="text-gray-700">{{ formatRupiah(p.budget) }}</span></dd>
            </dl>
            <BudgetBar class="mt-2" :budget="p.budget" :committed="p.committed" :paid="p.paid" :estimated="p.estimated" />
            <div class="mt-2 flex items-end justify-between gap-2 border-t border-gray-100 pt-2">
              <div class="min-w-0 text-xs">
                <p class="text-gray-400">Sisa RAB</p>
                <p class="text-sm font-semibold" :class="p.remaining_budget < 0 ? 'text-red-600' : 'text-gray-900'">
                  {{ formatRupiah(p.remaining_budget) }}
                </p>
              </div>
              <div class="action-btns shrink-0">
                <button class="act-view" @click="goDetail(p)" title="Lihat"><IconEye /></button>
                <button v-if="canManage" class="act-edit" @click="openEdit(p)" title="Ubah"><IconPencil /></button>
                <button v-if="canManage" class="act-del" @click="confirmDelete(p)" title="Hapus"><IconTrash /></button>
              </div>
            </div>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="projects" :loading="loading" emptyText="Belum ada projek.">
        <template #cell-project_number="{ row }">
          <span class="font-mono text-xs text-gray-700">{{ row.project_number || '-' }}</span>
        </template>
        <template #cell-name="{ row }">
          <button @click="goDetail(row)" class="text-left font-medium text-gray-900 hover:text-emerald-600 hover:underline">
            {{ row.name }}
          </button>
          <p v-if="row.pic" class="text-[11px] text-gray-400">PIC: {{ row.pic }}</p>
        </template>
        <template #cell-work_unit_name="{ row }">
          <span class="text-gray-700">{{ row.work_unit_name || '-' }}</span>
        </template>
        <template #cell-budget="{ row }">
          <div class="min-w-[160px]">
            <p class="flex items-center gap-1.5 text-sm font-medium text-gray-900">{{ formatRupiah(row.budget) }} <span :class="rabBadge(row)">{{ rabLabel(row) }}</span></p>
            <BudgetBar class="mt-1" :budget="row.budget" :committed="row.committed" :paid="row.paid" :estimated="row.estimated" />
          </div>
        </template>
        <template #cell-remaining_budget="{ row }">
          <span :class="row.remaining_budget < 0 ? 'font-semibold text-red-600' : 'text-gray-800'">
            {{ formatRupiah(row.remaining_budget) }}
          </span>
        </template>
        <template #cell-request_count="{ row }">
          <span class="text-gray-600">{{ row.request_count }}</span>
        </template>
        <template #cell-status="{ row }">
          <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="action-btns justify-end">
            <button class="act-view" @click="goDetail(row)" title="Lihat"><IconEye /></button>
            <button v-if="canManage" class="act-edit" @click="openEdit(row)" title="Ubah"><IconPencil /></button>
            <button v-if="canManage" class="act-del" @click="confirmDelete(row)" title="Hapus"><IconTrash /></button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- Create / Edit -->
    <AppModal v-model="showForm" :title="editingId ? 'Ubah Projek' : 'Buat Projek'" size="lg">
      <form class="space-y-4" @submit.prevent="submitForm">
        <AppAlert type="error" :message="formError" />
        <AppInput v-model="form.name" label="Nama Projek" placeholder="Contoh: Pembangunan Dapur Outlet Pusat" />

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium text-gray-700">Unit Kerja</label>
            <SearchSelect v-model="form.work_unit_id" :options="workUnitOptions"
              placeholder="Pilih unit kerja…" searchPlaceholder="Cari unit kerja…" />
          </div>
          <AppInput v-model="form.pic" label="PIC / Penanggung Jawab" placeholder="Nama penanggung jawab" />
        </div>

        <div class="rounded-lg border border-emerald-100 bg-emerald-50 px-3 py-2 text-xs text-emerald-800">
          <template v-if="editingId">
            RAB saat ini <b>{{ formatRupiah(editingBudget) }}</b>. Susunan barisnya diubah di halaman detail projek (“Ubah RAB” / “Buka Revisi”).
          </template>
          <template v-else>
            RAB disusun <b>per baris</b> (bagian pekerjaan → uraian, volume × harga satuan) di halaman detail setelah projek dibuat, lalu <b>ditetapkan</b>.
            Belanja tahap baru bisa dibuat setelah RAB ditetapkan.
          </template>
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium text-gray-700">Mulai</label>
            <input v-model="form.start_date" type="date" class="input-form" />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium text-gray-700">Target Selesai</label>
            <input v-model="form.target_date" type="date" class="input-form" />
          </div>
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium text-gray-700">Status</label>
            <select v-model="form.status" class="input-form">
              <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
            </select>
          </div>
        </div>

        <AppInput v-model="form.notes" label="Catatan" placeholder="Keterangan tambahan (opsional)" />
      </form>
      <template #footer>
        <AppButton variant="secondary" @click="showForm = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitForm">{{ editingId ? 'Simpan' : 'Buat Projek' }}</AppButton>
      </template>
    </AppModal>

    <!-- Delete -->
    <AppModal v-model="showDelete" title="Hapus Projek" size="sm">
      <p class="text-sm text-gray-600">
        Yakin ingin menghapus projek <strong>{{ deleteTarget?.name }}</strong>?
      </p>
      <p class="mt-2 text-xs text-gray-400">
        Projek yang masih memayungi pengajuan pengadaan tidak bisa dihapus.
      </p>
      <template #footer>
        <AppButton variant="secondary" @click="showDelete = false">Batal</AppButton>
        <AppButton variant="danger" :loading="saving" @click="submitDelete">Hapus</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { projectsApi } from '@/api/projects.js'
import { workUnitsApi } from '@/api/workUnits.js'
import { formatRupiah } from '@/utils/format.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import BudgetBar     from '@/components/BudgetBar.vue'

const router = useRouter()
const toast = useToastStore()
const authStore = useAuthStore()

const canManage = computed(() => authStore.hasPermission('procurement.projects.manage'))

// Ikon inline (gaya lucide) sebagai komponen render-function: halaman ini
// memakai tiga ikon yang sama di dua tempat (kartu mobile + tabel), jadi
// menyalin SVG-nya enam kali justru bikin template sulit dibaca.
const icon = (...children) => () => h(
  'svg',
  { width: 13, height: 13, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
    'stroke-width': 2, 'stroke-linecap': 'round', 'stroke-linejoin': 'round' },
  children.map(([tag, attrs]) => h(tag, attrs)),
)
const IconEye = icon(
  ['path', { d: 'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z' }],
  ['circle', { cx: 12, cy: 12, r: 3 }],
)
const IconPencil = icon(
  ['path', { d: 'M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7' }],
  ['path', { d: 'M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z' }],
)
const IconTrash = icon(
  ['polyline', { points: '3 6 5 6 21 6' }],
  ['path', { d: 'M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2' }],
)

const STATUS_OPTIONS = [
  { value: 'draft',    label: 'Draft' },
  { value: 'berjalan', label: 'Berjalan' },
  { value: 'selesai',  label: 'Selesai' },
  { value: 'batal',    label: 'Batal' },
]
const statusMap = {
  draft:    { label: 'Draft',    cls: 'bg-gray-100 text-gray-600' },
  berjalan: { label: 'Berjalan', cls: 'bg-blue-100 text-blue-700' },
  selesai:  { label: 'Selesai',  cls: 'bg-emerald-100 text-emerald-700' },
  batal:    { label: 'Batal',    cls: 'bg-red-100 text-red-700' },
}
function statusBadge(s) {
  const m = statusMap[s] || statusMap.berjalan
  return `inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${m.cls}`
}
function statusLabel(s) { return (statusMap[s] || statusMap.berjalan).label }
function rabLabel(p) { return p.rab_status === 'ditetapkan' ? `RAB v${p.rab_version}` : (p.rab_version > 0 ? 'RAB revisi' : 'RAB draft') }
function rabBadge(p) {
  const on = p.rab_status === 'ditetapkan'
  return `inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${on ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'}`
}

const COLUMNS = [
  { key: 'project_number',   label: 'Nomor' },
  { key: 'name',             label: 'Nama Projek' },
  { key: 'work_unit_name',   label: 'Unit Kerja' },
  { key: 'budget',           label: 'RAB & Serapan' },
  { key: 'remaining_budget', label: 'Sisa RAB' },
  { key: 'request_count',    label: 'Belanja' },
  { key: 'status',           label: 'Status' },
  { key: 'actions',          label: '' },
]

const loading = ref(false)
const errorMsg = ref('')
const projects = ref([])
const workUnits = ref([])
const search = ref('')
const filterStatus = ref('')

const workUnitOptions = computed(() => [{ id: '', name: '— Tanpa Unit Kerja —' }, ...workUnits.value])

const totals = computed(() => {
  const sum = (k) => projects.value.reduce((a, p) => a + (p[k] || 0), 0)
  const budget = sum('budget'), committed = sum('committed')
  return [
    { label: 'Total RAB',    value: budget,              cls: 'text-gray-900' },
    { label: 'Komitmen',     value: committed,           cls: 'text-blue-700' },
    { label: 'Terbayar',     value: sum('paid'),         cls: 'text-emerald-700' },
    { label: 'Sisa Hutang',  value: sum('outstanding'),  cls: 'text-amber-600' },
  ]
})

// RAB tidak diisi di sini: disusun per baris di halaman detail, dan status
// awal 'draft' sampai RAB ditetapkan (server menaikkannya ke 'berjalan').
function emptyForm() {
  return { name: '', work_unit_id: '', pic: '', start_date: '', target_date: '', status: 'draft', notes: '' }
}
const editingBudget = ref(0)

const showForm = ref(false)
const editingId = ref(null)
const form = ref(emptyForm())
const formError = ref('')
const saving = ref(false)

const showDelete = ref(false)
const deleteTarget = ref(null)

async function fetchList() {
  loading.value = true; errorMsg.value = ''
  try {
    const res = await projectsApi.list({ status: filterStatus.value, search: search.value })
    projects.value = Array.isArray(res) ? res : (res?.data || [])
  } catch (e) {
    errorMsg.value = e.message || 'Gagal memuat projek'
  } finally {
    loading.value = false
  }
}

async function fetchWorkUnits() {
  try {
    const res = await workUnitsApi.myWorkUnits()
    workUnits.value = Array.isArray(res) ? res : (res?.data || [])
  } catch { /* dropdown opsional — daftar projek tetap tampil */ }
}

function goDetail(p) { router.push(`/projects/${p.id}`) }

function openCreate() {
  editingId.value = null
  form.value = emptyForm()
  formError.value = ''
  showForm.value = true
}

function openEdit(p) {
  editingId.value = p.id
  editingBudget.value = p.budget || 0
  form.value = {
    name: p.name, work_unit_id: p.work_unit_id || '', pic: p.pic || '',
    start_date: p.start_date || '', target_date: p.target_date || '',
    status: p.status, notes: p.notes || '',
  }
  formError.value = ''
  showForm.value = true
}

async function submitForm() {
  if (!form.value.name.trim()) { formError.value = 'Nama projek wajib diisi'; return }
  saving.value = true; formError.value = ''
  try {
    if (editingId.value) {
      await projectsApi.update(editingId.value, form.value)
      toast.success('Projek diperbarui')
    } else {
      const created = await projectsApi.create(form.value)
      toast.success('Projek dibuat — susun RAB-nya sekarang')
      showForm.value = false
      // Langsung ke detail: langkah berikutnya adalah menyusun RAB.
      if (created?.id) { router.push(`/projects/${created.id}`); return }
    }
    showForm.value = false
    await fetchList()
  } catch (e) {
    formError.value = e.message || 'Gagal menyimpan projek'
  } finally {
    saving.value = false
  }
}

function confirmDelete(p) { deleteTarget.value = p; showDelete.value = true }

async function submitDelete() {
  saving.value = true
  try {
    await projectsApi.remove(deleteTarget.value.id)
    toast.success('Projek dihapus')
    showDelete.value = false
    await fetchList()
  } catch (e) {
    toast.error(e.message || 'Gagal menghapus projek')
  } finally {
    saving.value = false
  }
}

onMounted(() => { fetchList(); fetchWorkUnits() })
</script>

<style scoped>
@reference "tailwindcss";
.input-form {
  @apply w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500;
}
.action-btns { display: flex; gap: .35rem; }
dl dd, dl dt { min-width: 0; overflow-wrap: anywhere; }
.act-view, .act-edit, .act-del {
  width: 28px; height: 28px; border-radius: .45rem; border: none; cursor: pointer;
  display: flex; align-items: center; justify-content: center; transition: all .12s;
}
.act-view { background: rgba(59,130,246,.1); color: #3b82f6; }
.act-view:hover { background: rgba(59,130,246,.2); }
.act-edit { background: rgba(45,143,86,.1); color: #2d8f56; }
.act-edit:hover { background: rgba(45,143,86,.2); }
.act-del  { background: rgba(220,38,38,.07); color: #dc2626; }
.act-del:hover { background: rgba(220,38,38,.15); }
</style>
