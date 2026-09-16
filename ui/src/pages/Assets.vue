<template>
  <div class="space-y-4 sm:space-y-5">
    <!-- Header -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Manajemen Perlengkapan</h1>
        <p class="mt-0.5 text-sm text-gray-500">Inventaris barang beserta nilai buku, riwayat lokasi, dan perawatannya.</p>
      </div>
      <AppButton v-if="canCreate" class="w-full sm:w-auto" @click="openCreate">+ Tambah Perlengkapan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan -->
    <div v-if="!loading && assets.length" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard class="min-w-0"><p class="stat-lbl">Jumlah Aset</p><p class="stat-val">{{ summary.count }}</p><p class="stat-sub">{{ summary.units }} unit</p></AppCard>
      <AppCard class="min-w-0"><p class="stat-lbl">Nilai Perolehan</p><p class="stat-val">{{ formatRupiah(summary.acquisition) }}</p><p class="stat-sub">total harga beli</p></AppCard>
      <AppCard class="min-w-0"><p class="stat-lbl">Nilai Buku</p><p class="stat-val">{{ formatRupiah(summary.book) }}</p><p class="stat-sub">setelah penyusutan</p></AppCard>
      <AppCard class="min-w-0"><p class="stat-lbl">Perlu Perhatian</p><p class="stat-val" :class="summary.attention ? 'text-amber-600' : ''">{{ summary.attention }}</p><p class="stat-sub">rusak / dalam perbaikan</p></AppCard>
    </div>

    <!-- Filter -->
    <AppCard>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <select v-model="filterStatus" @change="load" class="form-input">
          <option value="">Semua status</option>
          <option v-for="(lbl, key) in STATUSES" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <select v-model="filterCondition" @change="load" class="form-input">
          <option value="">Semua kondisi</option>
          <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
        </select>
        <input v-model="search" @input="debouncedLoad" type="search" placeholder="Cari nama / nomor / seri…" class="form-input" />
      </div>
    </AppCard>

    <!-- Bilah aksi massal -->
    <div v-if="selected.length"
      class="sticky bottom-3 z-20 flex flex-wrap items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 p-3 shadow-sm">
      <span class="text-sm font-semibold text-emerald-800">{{ selected.length }} aset dipilih</span>
      <div class="ml-auto flex flex-wrap gap-2">
        <button class="btn-soft" @click="selected = []">Batal</button>
        <button class="btn-soft" @click="printSelected('50x25')">Label 50×25</button>
        <AppButton @click="printSelected('70x40')">Cetak Label 70×40</AppButton>
      </div>
    </div>

    <!-- Daftar -->
    <AppCard :padding="false">
      <!-- Mobile: kartu -->
      <div class="sm:hidden">
        <div v-if="loading" class="space-y-3 p-4">
          <div v-for="i in 3" :key="i" class="animate-pulse space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200"></div>
            <div class="h-3 w-1/2 rounded bg-gray-100"></div>
          </div>
        </div>
        <div v-else-if="!assets.length" class="p-6 text-center text-sm text-gray-400">
          {{ emptyText }}
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="a in assets" :key="a.id" class="space-y-2 p-4">
            <div class="flex items-start gap-2">
              <input type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-emerald-600" :value="a.id" v-model="selected" :aria-label="`Pilih ${a.name}`" />
              <button class="min-w-0 flex-1 text-left" @click="goDetail(a)">
                <p class="break-words font-semibold text-gray-900">{{ a.name }}</p>
                <p class="mt-0.5 font-mono text-[11px] text-gray-500">{{ a.asset_no || a.code || '—' }}</p>
              </button>
              <div class="flex shrink-0 flex-col items-end gap-1">
                <span :class="statusCls(a.status)">{{ statusLabel(a.status) }}</span>
                <span :class="condCls(a.condition)">{{ condLabel(a.condition) }}</span>
              </div>
            </div>
            <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Lokasi</dt>
              <dd class="break-words text-gray-700">{{ a.outlet_name }}<span v-if="a.location"> · {{ a.location }}</span></dd>
              <dt class="text-gray-400">Jumlah</dt>
              <dd class="text-gray-700">{{ a.quantity }} {{ a.unit }}</dd>
              <dt class="text-gray-400">Nilai buku</dt>
              <dd class="text-gray-700">{{ formatRupiah(a.book_value) }}<span v-if="a.purchase_price" class="text-gray-400"> dari {{ formatRupiah(a.purchase_price) }}</span></dd>
              <dt class="text-gray-400">Perawatan</dt>
              <dd class="text-gray-700">{{ a.maintenance_count }}× · {{ a.last_maintenance ? formatDateStr(a.last_maintenance) : 'belum ada' }}</dd>
            </dl>
            <div class="flex gap-2 pt-1">
              <button @click="goDetail(a)" class="act-btn bg-emerald-50 text-emerald-700 hover:bg-emerald-100">Detail</button>
              <button v-if="canUpdate" @click="openEdit(a)" class="act-btn bg-gray-100 text-gray-700 hover:bg-gray-200">Edit</button>
              <button v-if="canDelete" @click="confirmDelete(a)" class="act-btn bg-red-50 text-red-600 hover:bg-red-100">Hapus</button>
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop: tabel -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="assets" :loading="loading" :emptyText="emptyText">
        <template #cell-pick="{ row }">
          <input type="checkbox" class="h-4 w-4 accent-emerald-600" :value="row.id" v-model="selected" :aria-label="`Pilih ${row.name}`" />
        </template>
        <template #cell-name="{ row }">
          <button class="text-left" @click="goDetail(row)">
            <p class="font-medium text-gray-900 hover:text-emerald-700">{{ row.name }}</p>
            <p class="font-mono text-xs text-gray-400">{{ row.asset_no || row.code || '—' }}</p>
          </button>
        </template>
        <template #cell-quantity="{ row }">{{ row.quantity }} {{ row.unit }}</template>
        <template #cell-status="{ row }">
          <div class="flex flex-col items-start gap-1">
            <span :class="statusCls(row.status)">{{ statusLabel(row.status) }}</span>
            <span :class="condCls(row.condition)">{{ condLabel(row.condition) }}</span>
          </div>
        </template>
        <template #cell-value="{ row }">
          <span class="text-sm">{{ formatRupiah(row.book_value) }}</span>
          <span v-if="row.purchase_price" class="block text-xs text-gray-400">dari {{ formatRupiah(row.purchase_price) }}</span>
        </template>
        <template #cell-maintenance="{ row }">
          <span class="text-sm">{{ row.maintenance_count }}×</span>
          <span class="block text-xs text-gray-400">{{ row.last_maintenance ? formatDateStr(row.last_maintenance) : 'belum ada' }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <button @click="goDetail(row)" class="rounded px-2 py-1 text-xs font-medium text-emerald-600 hover:bg-emerald-50 hover:text-emerald-800">Detail</button>
            <button v-if="canUpdate" @click="openEdit(row)" class="rounded px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900">Edit</button>
            <button v-if="canDelete" @click="confirmDelete(row)" class="rounded px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 hover:text-red-800">Hapus</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- ── Modal tambah/edit ── -->
    <AppModal v-model="assetModal" :title="editing ? 'Edit Perlengkapan' : 'Tambah Perlengkapan'" size="2xl">
      <form class="space-y-3" @submit.prevent="saveAsset">
        <div v-if="!editing">
          <label class="lbl">Outlet <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.outlet_id" :options="outlets" placeholder="Pilih outlet…" searchPlaceholder="Cari outlet…" />
        </div>
        <p v-else class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500">
          Outlet <strong class="text-gray-700">{{ editing.outlet_name }}</strong> tidak bisa diubah di sini —
          perpindahan antar outlet dilakukan lewat dokumen mutasi.
        </p>

        <div>
          <label class="lbl">Nama Perlengkapan <span class="text-red-500">*</span></label>
          <input v-model="form.name" class="form-input" placeholder="Contoh: AC Daikin 1PK" required />
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Kategori</label>
            <SearchSelect v-model="form.category" :options="categoryOptions" placeholder="Pilih kategori…"
              searchPlaceholder="Cari kategori…" @change="applyCategoryDefaults" />
            <p class="mt-1 text-[11px] text-gray-400">
              Dikelola di <RouterLink to="/perlengkapan/kategori" class="text-emerald-700 underline">Kategori Aset</RouterLink>.
            </p>
          </div>
          <div>
            <label class="lbl">Kode / Tag internal</label>
            <input v-model="form.code" class="form-input" placeholder="mis. stiker lama MJ-001" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="lbl">Mode Pencatatan</label>
            <select v-model="form.tracking_mode" class="form-input">
              <option v-for="(lbl, key) in TRACKING_MODES" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
          <div>
            <label class="lbl">Jumlah</label>
            <input v-model.number="form.quantity" type="number" inputmode="numeric" min="1" class="form-input"
              :disabled="form.tracking_mode === 'tunggal'" />
            <p v-if="form.tracking_mode === 'tunggal'" class="mt-1 text-[11px] text-gray-400">Mode tunggal selalu 1 unit.</p>
          </div>
          <div>
            <label class="lbl">Satuan</label>
            <input v-model="form.unit" class="form-input" placeholder="unit" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="lbl">Merk</label>
            <input v-model="form.brand" class="form-input" placeholder="mis. Daikin" />
          </div>
          <div>
            <label class="lbl">Tipe / Model</label>
            <input v-model="form.model" class="form-input" placeholder="mis. FTKC25" />
          </div>
          <div>
            <label class="lbl">Nomor Seri</label>
            <input v-model="form.serial_number" class="form-input" placeholder="Opsional" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Kondisi</label>
            <select v-model="form.condition" class="form-input">
              <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
          <div>
            <label class="lbl">Status</label>
            <select v-model="form.status" class="form-input" :disabled="systemStatusLocked">
              <option v-for="(lbl, key) in EDITABLE_STATUSES" :key="key" :value="key">{{ lbl }}</option>
            </select>
            <p v-if="systemStatusLocked" class="mt-1 text-[11px] text-amber-600">
              Status “{{ statusLabel(editing.status) }}” diatur oleh dokumen, bukan form ini.
            </p>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Lokasi / Ruang</label>
            <input v-model="form.location" class="form-input" placeholder="mis. Lantai 1 – Area Indoor" />
          </div>
          <div>
            <label class="lbl">Penanggung Jawab</label>
            <input v-model="form.pic_name" class="form-input" placeholder="Nama petugas" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="lbl">Tgl Perolehan</label>
            <input v-model="form.purchase_date" type="date" class="form-input" />
          </div>
          <div>
            <label class="lbl">Harga Perolehan</label>
            <input v-model.number="form.purchase_price" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
          </div>
          <div>
            <label class="lbl">Garansi s/d</label>
            <input v-model="form.warranty_until" type="date" class="form-input" />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Umur Ekonomis (bulan)</label>
            <input v-model.number="form.useful_life_months" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0 = tidak disusutkan" />
            <p class="mt-1 text-[11px] text-gray-400">Dipakai menghitung nilai buku (garis lurus).</p>
          </div>
          <div>
            <label class="lbl">Nilai Residu</label>
            <input v-model.number="form.residual_value" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
          </div>
        </div>

        <div>
          <label class="lbl">Catatan</label>
          <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
        </div>

        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="assetModal = false">Batal</button>
          <AppButton type="submit" :loading="saving">{{ editing ? 'Simpan' : 'Tambah' }}</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { assetCategoriesApi } from '@/api/assetCategories.js'
import { assetsApi } from '@/api/assets.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr } from '@/utils/format.js'
import {
  CONDITIONS, STATUSES, EDITABLE_STATUSES, SYSTEM_STATUSES, TRACKING_MODES,
  condCls, condLabel, statusCls, statusLabel, printAssetLabels,
} from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const route = useRoute()
const router = useRouter()
const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.create')
const canUpdate = auth.hasPermission('assets.update')
const canDelete = auth.hasPermission('assets.delete')

const COLUMNS = [
  { key: 'pick',        label: '' },
  { key: 'name',        label: 'Perlengkapan' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'quantity',    label: 'Jumlah' },
  { key: 'status',      label: 'Status / Kondisi' },
  { key: 'value',       label: 'Nilai Buku' },
  { key: 'maintenance', label: 'Perawatan' },
  { key: 'actions',     label: '' },
]

const assets = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterCondition = ref('')
const filterStatus = ref('')
const search = ref('')
const selected = ref([])

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
// Kategori berasal dari master, bukan ketikan bebas: "Elektronik",
// "elektronik", dan "Elektronic" dulu jadi tiga kelompok di dashboard & laporan.
const categories = ref([])
const categoryOptions = computed(() => categories.value.map(c => ({ id: c.name, name: c.name })))
async function loadCategories() {
  try {
    const d = await assetCategoriesApi.list()
    categories.value = Array.isArray(d) ? d : (d?.data || [])
  } catch { categories.value = [] }
}
const hasFilter = computed(() => !!(filterOutlet.value || filterCondition.value || filterStatus.value || search.value.trim()))
const emptyText = computed(() => hasFilter.value
  ? 'Tidak ada perlengkapan yang cocok dengan filter ini. Ubah atau kosongkan filter untuk melihat semuanya.'
  : (canCreate ? 'Belum ada perlengkapan. Tambahkan lewat tombol di kanan atas.' : 'Belum ada perlengkapan terdata.'))

const summary = computed(() => {
  const rows = assets.value
  return {
    count: rows.length,
    units: rows.reduce((s, a) => s + (a.quantity || 0), 0),
    acquisition: rows.reduce((s, a) => s + (a.purchase_price || 0) * (a.quantity || 1), 0),
    book: rows.reduce((s, a) => s + (a.book_value || 0) * (a.quantity || 1), 0),
    attention: rows.filter(a => a.condition !== 'baik' || a.status === 'perbaikan').length,
  }
})

// apiClient tidak membuka amplop untuk respons non-paginasi.
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await assetsApi.list({
      outlet_id: filterOutlet.value || undefined,
      condition: filterCondition.value || undefined,
      status: filterStatus.value || undefined,
      search: search.value.trim() || undefined,
    })
    assets.value = asArray(data)
    // Buang pilihan yang sudah tidak ada di hasil terbaru.
    const ids = new Set(assets.value.map(a => a.id))
    selected.value = selected.value.filter(id => ids.has(id))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat perlengkapan'
  } finally {
    loading.value = false
  }
}
let _t = null
function debouncedLoad() { clearTimeout(_t); _t = setTimeout(load, 350) }

async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

function goDetail(a) { router.push(`/perlengkapan/${a.id}`) }

async function printSelected(size) {
  const rows = assets.value.filter(a => selected.value.includes(a.id))
  const r = await printAssetLabels(rows, { size })
  if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
}

// ── CRUD ──
const assetModal = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref(blankForm())
const systemStatusLocked = computed(() => !!editing.value && SYSTEM_STATUSES.includes(editing.value.status))

function blankForm() {
  return {
    outlet_id: filterOutlet.value || '', code: '', name: '', category: '', quantity: 1, unit: 'unit',
    tracking_mode: 'massal', serial_number: '', brand: '', model: '', condition: 'baik', status: 'aktif',
    location: '', pic_name: '', purchase_date: '', purchase_price: 0, warranty_until: '',
    useful_life_months: 0, residual_value: 0, photo_url: '', notes: '',
  }
}
// Umur ekonomis mengikuti kategorinya — angka itu kini tinggal di master,
// bukan ditebak dari nama barang di sisi layar.
function applyCategoryDefaults() {
  const c = categories.value.find(x => x.name === form.value.category)
  if (c && !form.value.useful_life_months && c.useful_life_months) {
    form.value.useful_life_months = c.useful_life_months
  }
}
function openCreate() { editing.value = null; form.value = blankForm(); assetModal.value = true }
function openEdit(a) {
  editing.value = a
  form.value = {
    outlet_id: a.outlet_id, code: a.code, name: a.name, category: a.category, quantity: a.quantity, unit: a.unit,
    tracking_mode: a.tracking_mode || 'massal', serial_number: a.serial_number || '', brand: a.brand || '',
    model: a.model || '', condition: a.condition, status: a.status || 'aktif', location: a.location,
    pic_name: a.pic_name || '', purchase_date: a.purchase_date || '', purchase_price: a.purchase_price,
    warranty_until: a.warranty_until || '', useful_life_months: a.useful_life_months || 0,
    residual_value: a.residual_value || 0, photo_url: a.photo_url || '', notes: a.notes,
  }
  assetModal.value = true
}
async function saveAsset() {
  if (!form.value.name?.trim()) { toast.error('Nama perlengkapan wajib diisi'); return }
  if (!editing.value && !form.value.outlet_id) { toast.error('Pilih outlet'); return }
  if (form.value.tracking_mode === 'tunggal') form.value.quantity = 1
  saving.value = true
  try {
    if (editing.value) await assetsApi.update(editing.value.id, form.value)
    else await assetsApi.create(form.value)
    toast.success(editing.value ? 'Perlengkapan diperbarui' : 'Perlengkapan ditambahkan')
    assetModal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}
async function confirmDelete(a) {
  if (!window.confirm(`Hapus data "${a.name}" dari daftar? Gunakan ini hanya untuk salah input — barang yang rusak atau dijual dicatat lewat penghapusan aset.`)) return
  try { await assetsApi.remove(a.id); toast.success('Data perlengkapan dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(async () => {
  await Promise.all([loadOutlets(), loadCategories()])
  await load()
  // Datang dari halaman detail lewat tombol Edit.
  const id = route.query.edit
  if (id) {
    const found = assets.value.find(a => a.id === String(id))
    if (found && canUpdate) openEdit(found)
    router.replace({ path: '/perlengkapan' })
  }
})
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f9fafb; color: #6b7280; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost {
  padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600;
  color: #374151; background: #f3f4f6; min-height: 40px;
}
.btn-ghost:hover { background: #e5e7eb; }
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.08); min-height: 40px;
}
.btn-soft:hover { background: #f3f4f6; }
.act-btn {
  flex: 1; min-height: 40px; border-radius: .6rem; padding: .4rem .5rem;
  font-size: .75rem; font-weight: 600; text-align: center;
}
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; overflow-wrap: anywhere; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
</style>
