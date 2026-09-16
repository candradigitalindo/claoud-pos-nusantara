<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Penghapusan Aset</h1>
        <p class="mt-0.5 text-sm text-gray-500">Barang yang dijual, dimusnahkan, atau hilang — beserta nilai bukunya saat dihapus.</p>
      </div>
      <AppButton v-if="canCreate" class="w-full sm:w-auto" @click="openCreate">+ Ajukan Penghapusan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <div class="-mx-1 overflow-x-auto">
      <div class="flex gap-1 px-1 pb-1">
        <button v-for="t in TABS" :key="t.key" @click="filterStatus = t.key; load()"
          class="shrink-0 rounded-lg px-3 py-2 text-xs font-semibold transition"
          :class="filterStatus === t.key ? 'bg-emerald-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'">
          {{ t.label }}
        </button>
      </div>
    </div>

    <AppCard :padding="false">
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">{{ emptyText }}</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="d in rows" :key="d.id" class="space-y-2 p-4">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="break-words font-semibold text-gray-900">{{ d.asset_name }}</p>
                <p class="font-mono text-[11px] text-gray-500">{{ d.disposal_number }} · {{ d.asset_no }}</p>
              </div>
              <span :class="disposalStatusCls(d.status)">{{ DISPOSAL_STATUSES[d.status] || d.status }}</span>
            </div>
            <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Cara</dt><dd class="text-gray-700">{{ DISPOSAL_METHODS[d.method] || d.method }} · {{ d.qty }} unit</dd>
              <dt class="text-gray-400">Nilai buku</dt><dd class="text-gray-700">{{ formatRupiah(d.book_value) }}</dd>
              <dt v-if="d.proceeds" class="text-gray-400">Hasil jual</dt><dd v-if="d.proceeds" class="text-gray-700">{{ formatRupiah(d.proceeds) }}</dd>
              <dt class="text-gray-400">Alasan</dt><dd class="break-words text-gray-700">{{ d.reason }}</dd>
            </dl>
            <div v-if="canApprove && d.status === 'pending'" class="flex gap-2 pt-1">
              <button class="act-btn bg-red-50 text-red-600" @click="decide(d, false)">Tolak</button>
              <button class="act-btn bg-emerald-600 text-white" @click="decide(d, true)">Setujui</button>
            </div>
          </li>
        </ul>
      </div>

      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" :emptyText="emptyText">
        <template #cell-asset="{ row }">
          <p class="font-medium text-gray-900">{{ row.asset_name }}</p>
          <p class="font-mono text-[11px] text-gray-400">{{ row.disposal_number }} · {{ row.asset_no }}</p>
        </template>
        <template #cell-method="{ row }">{{ DISPOSAL_METHODS[row.method] || row.method }} · {{ row.qty }} unit</template>
        <template #cell-value="{ row }">
          <span class="text-sm">{{ formatRupiah(row.book_value) }}</span>
          <span v-if="row.proceeds" class="block text-xs text-gray-400">hasil {{ formatRupiah(row.proceeds) }}</span>
        </template>
        <template #cell-reason="{ row }">
          <p class="max-w-xs break-words text-sm text-gray-700">{{ row.reason }}</p>
          <p v-if="row.rejected_reason" class="text-xs text-red-600">Ditolak: {{ row.rejected_reason }}</p>
        </template>
        <template #cell-status="{ row }"><span :class="disposalStatusCls(row.status)">{{ DISPOSAL_STATUSES[row.status] || row.status }}</span></template>
        <template #cell-actions="{ row }">
          <div v-if="canApprove && row.status === 'pending'" class="flex justify-end gap-1">
            <button class="rounded px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50" @click="decide(row, false)">Tolak</button>
            <button class="rounded px-2 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50" @click="decide(row, true)">Setujui</button>
          </div>
        </template>
      </AppTable>
    </AppCard>

    <AppModal v-model="createModal" title="Ajukan Penghapusan Aset" size="lg">
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Aset <span class="text-red-500">*</span></label>
          <SearchSelect v-model="form.asset_id" :options="assetOptions" placeholder="Pilih aset…" searchPlaceholder="Cari nama / nomor…" />
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Jumlah</label>
            <input v-model.number="form.qty" type="number" inputmode="numeric" min="1" :max="selectedAsset?.quantity || 1" class="form-input" />
            <p v-if="selectedAsset" class="mt-1 text-[11px] text-gray-400">Tersedia {{ selectedAsset.quantity }} {{ selectedAsset.unit }}</p>
          </div>
          <div>
            <label class="lbl">Cara</label>
            <select v-model="form.method" class="form-input">
              <option v-for="(lbl, key) in DISPOSAL_METHODS" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
        </div>
        <div v-if="form.method === 'dijual' || form.method === 'tukar_tambah'">
          <label class="lbl">Hasil Penjualan</label>
          <input v-model.number="form.proceeds" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
        </div>
        <div>
          <label class="lbl">Alasan <span class="text-red-500">*</span></label>
          <textarea v-model="form.reason" rows="2" class="form-input" placeholder="mis. rusak berat, biaya perbaikan melebihi harga barang" required></textarea>
        </div>
        <p v-if="selectedAsset" class="rounded-lg bg-gray-50 p-3 text-xs text-gray-600">
          Nilai buku saat ini {{ formatRupiah(selectedAsset.book_value) }} per unit — angka ini dibekukan pada dokumen
          agar berita acara tetap cocok meski penyusutan terus berjalan.
        </p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="createModal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Ajukan</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetDisposalsApi } from '@/api/assetOps.js'
import { assetsApi } from '@/api/assets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah } from '@/utils/format.js'
import { DISPOSAL_METHODS, DISPOSAL_STATUSES, disposalStatusCls } from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.disposal.create')
const canApprove = auth.hasPermission('assets.disposal.approve')

const TABS = [
  { key: 'pending', label: 'Menunggu Persetujuan' },
  { key: 'approved', label: 'Disetujui' },
  { key: 'rejected', label: 'Ditolak' },
  { key: '', label: 'Semua' },
]
const COLUMNS = [
  { key: 'asset', label: 'Aset' },
  { key: 'method', label: 'Cara' },
  { key: 'value', label: 'Nilai Buku' },
  { key: 'reason', label: 'Alasan' },
  { key: 'status', label: 'Status' },
  { key: 'actions', label: '' },
]

const rows = ref([])
const assets = ref([])
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const filterStatus = ref('pending')
const createModal = ref(false)
const form = ref(blankForm())

function blankForm() { return { asset_id: '', qty: 1, method: 'dimusnahkan', reason: '', proceeds: 0 } }
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

const assetOptions = computed(() => assets.value.map(a => ({ id: a.id, name: `${a.name} — ${a.asset_no} (${a.quantity} ${a.unit})` })))
const selectedAsset = computed(() => assets.value.find(a => a.id === form.value.asset_id) || null)
const emptyText = computed(() => filterStatus.value === 'pending'
  ? 'Tidak ada pengajuan yang menunggu persetujuan.'
  : 'Belum ada dokumen penghapusan.')

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetDisposalsApi.list({ status: filterStatus.value || undefined }))
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat penghapusan' } finally { loading.value = false }
}
async function loadAssets() {
  try { assets.value = asArray(await assetsApi.list()) } catch { assets.value = [] }
}

function openCreate() { form.value = blankForm(); createModal.value = true }

async function save() {
  if (!form.value.asset_id) { toast.error('Pilih aset'); return }
  if (!form.value.reason?.trim()) { toast.error('Alasan wajib diisi'); return }
  saving.value = true
  try {
    await assetDisposalsApi.create(form.value)
    toast.success('Pengajuan penghapusan dibuat')
    createModal.value = false
    filterStatus.value = 'pending'
    await Promise.all([load(), loadAssets()])
  } catch (e) { toast.error(e?.message || 'Gagal mengajukan') } finally { saving.value = false }
}

async function decide(d, approve) {
  let reason = ''
  if (!approve) {
    reason = window.prompt('Alasan penolakan:') || ''
    if (!reason.trim()) return
  } else if (!window.confirm(`Setujui penghapusan ${d.qty} unit ${d.asset_name}? Aset akan keluar dari daftar aktif.`)) {
    return
  }
  try {
    await assetDisposalsApi.approve(d.id, { approve, reason })
    toast.success(approve ? 'Penghapusan disetujui' : 'Pengajuan ditolak')
    await Promise.all([load(), loadAssets()])
  } catch (e) { toast.error(e?.message || 'Aksi gagal') }
}

onMounted(async () => { await Promise.all([load(), loadAssets()]) })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; min-height: 40px; }
.btn-ghost:hover { background: #e5e7eb; }
.act-btn { flex: 1; min-height: 40px; border-radius: .6rem; padding: .4rem .5rem; font-size: .75rem; font-weight: 600; text-align: center; }
</style>
