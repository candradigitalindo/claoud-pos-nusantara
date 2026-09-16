<template>
  <div class="space-y-4 sm:space-y-5">
    <!-- Header -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <button @click="goBack" class="inline-flex items-center gap-1 text-xs font-medium text-gray-500 hover:text-gray-800">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><path d="M12 19l-7-7 7-7"/></svg>
          Daftar Perlengkapan
        </button>
        <h1 v-if="asset" class="mt-1 text-lg sm:text-xl font-bold text-gray-900 break-words">{{ asset.name }}</h1>
        <div v-if="asset" class="mt-1 flex flex-wrap items-center gap-2">
          <span class="font-mono text-xs text-gray-500">{{ asset.asset_no || '—' }}</span>
          <span :class="statusCls(asset.status)">{{ statusLabel(asset.status) }}</span>
          <span :class="condCls(asset.condition)">{{ condLabel(asset.condition) }}</span>
          <span v-if="warrantyState" :class="warrantyState.cls">{{ warrantyState.text }}</span>
        </div>
      </div>
      <div v-if="asset" class="flex flex-wrap gap-2">
        <button class="btn-soft w-full sm:w-auto" @click="doPrint">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 6 2 18 2 18 9"/><path d="M6 18H4a2 2 0 01-2-2v-5a2 2 0 012-2h16a2 2 0 012 2v5a2 2 0 01-2 2h-2"/><rect x="6" y="14" width="12" height="8"/></svg>
          Cetak Label
        </button>
        <AppButton v-if="canUpdate" class="w-full sm:w-auto" @click="goEdit">Edit</AppButton>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Memuat -->
    <AppCard v-if="loading">
      <div class="space-y-3 animate-pulse">
        <div class="h-5 w-1/3 rounded bg-gray-200"></div>
        <div class="h-24 rounded bg-gray-100"></div>
        <div class="h-4 w-2/3 rounded bg-gray-100"></div>
      </div>
    </AppCard>

    <template v-else-if="asset">
      <!-- Ringkasan angka -->
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <AppCard class="min-w-0">
          <p class="stat-lbl">Nilai Perolehan</p>
          <p class="stat-val">{{ formatRupiah(asset.purchase_price) }}</p>
          <p class="stat-sub">{{ asset.quantity }} {{ asset.unit }}<span v-if="asset.purchase_date"> · {{ formatDateStr(asset.purchase_date) }}</span></p>
        </AppCard>
        <AppCard class="min-w-0">
          <p class="stat-lbl">Nilai Buku</p>
          <p class="stat-val">{{ formatRupiah(asset.book_value) }}</p>
          <p class="stat-sub">
            <template v-if="asset.useful_life_months > 0">
              susut {{ formatRupiah(asset.depreciation) }} · {{ asset.months_elapsed }}/{{ asset.useful_life_months }} bln
            </template>
            <template v-else>tidak disusutkan</template>
          </p>
        </AppCard>
        <AppCard class="min-w-0">
          <p class="stat-lbl">Biaya Perawatan</p>
          <p class="stat-val">{{ formatRupiah(asset.maintenance_cost) }}</p>
          <p class="stat-sub">{{ asset.maintenance_count }}× perawatan</p>
        </AppCard>
        <AppCard class="min-w-0">
          <p class="stat-lbl">Rasio Rawat : Beli</p>
          <p class="stat-val" :class="ratioPct >= 50 ? 'text-red-600' : ''">{{ ratioText }}</p>
          <p class="stat-sub">{{ ratioPct >= 50 ? 'Pertimbangkan ganti, bukan perbaiki' : 'Dalam batas wajar' }}</p>
        </AppCard>
      </div>

      <!-- Identitas -->
      <AppCard>
        <h2 class="mb-3 text-sm font-bold text-gray-900">Identitas & Penempatan</h2>
        <dl class="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="f in infoFields" :key="f.label" class="flex justify-between gap-3 border-b border-gray-50 py-1 text-sm">
            <dt class="shrink-0 text-gray-400">{{ f.label }}</dt>
            <dd class="min-w-0 break-words text-right font-medium text-gray-800">{{ f.value }}</dd>
          </div>
        </dl>
        <div v-if="asset.notes" class="mt-3 rounded-lg bg-gray-50 p-3 text-sm text-gray-700 break-words">{{ asset.notes }}</div>
        <div v-if="asset.photo_url" class="mt-3">
          <img :src="asset.photo_url" alt="Foto aset" class="max-h-56 rounded-lg border border-gray-200" />
        </div>
      </AppCard>

      <!-- Tab -->
      <AppCard :padding="false">
        <div class="flex border-b border-gray-100">
          <button v-for="t in TABS" :key="t.key" @click="tab = t.key"
            class="flex-1 px-3 py-2.5 text-xs sm:text-sm font-semibold transition"
            :class="tab === t.key ? 'border-b-2 border-emerald-500 text-emerald-700' : 'text-gray-500 hover:text-gray-800'">
            {{ t.label }}
            <span v-if="t.count !== null" class="ml-1 text-[10px] text-gray-400">{{ t.count }}</span>
          </button>
        </div>

        <!-- Perawatan -->
        <div v-if="tab === 'rawat'" class="p-3 sm:p-4 space-y-4">
          <details v-if="canUpdate" class="rounded-lg border border-gray-200" :open="!maintenances.length">
            <summary class="cursor-pointer select-none rounded-lg bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-700">+ Catat Perawatan</summary>
            <form class="space-y-3 border-t border-gray-100 p-3" @submit.prevent="saveMaintenance">
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="lbl">Tanggal</label>
                  <input v-model="mForm.maintenance_date" type="date" class="form-input" />
                </div>
                <div>
                  <label class="lbl">Jenis</label>
                  <select v-model="mForm.type" class="form-input">
                    <option v-for="(lbl, key) in MTYPES" :key="key" :value="key">{{ lbl }}</option>
                  </select>
                </div>
              </div>
              <div>
                <label class="lbl">Deskripsi <span class="text-red-500">*</span></label>
                <textarea v-model="mForm.description" rows="2" class="form-input" placeholder="Pekerjaan yang dilakukan" required></textarea>
              </div>
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="lbl">Biaya</label>
                  <input v-model.number="mForm.cost" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
                </div>
                <div>
                  <label class="lbl">Pelaksana / Teknisi</label>
                  <input v-model="mForm.performed_by" class="form-input" placeholder="Nama / vendor" />
                </div>
              </div>
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="lbl">Kondisi Setelah</label>
                  <select v-model="mForm.condition_after" class="form-input">
                    <option value="">— tidak diubah —</option>
                    <option v-for="(lbl, key) in CONDITIONS" :key="key" :value="key">{{ lbl }}</option>
                  </select>
                </div>
                <div>
                  <label class="lbl">Jadwal Berikutnya</label>
                  <input v-model="mForm.next_due_date" type="date" class="form-input" />
                </div>
              </div>
              <div class="flex justify-end">
                <AppButton type="submit" :loading="savingM" class="w-full sm:w-auto">Simpan Perawatan</AppButton>
              </div>
            </form>
          </details>

          <p v-if="loadingSub" class="py-4 text-center text-sm text-gray-400">Memuat…</p>
          <p v-else-if="!maintenances.length" class="py-6 text-center text-sm text-gray-400">
            Belum ada catatan perawatan.<span v-if="canUpdate"> Catat pekerjaan pertama lewat tombol di atas.</span>
          </p>
          <ol v-else class="space-y-3">
            <li v-for="m in maintenances" :key="m.id" class="relative border-l-2 border-emerald-100 pl-5">
              <span class="absolute -left-[7px] top-1 h-3 w-3 rounded-full bg-emerald-500 ring-2 ring-white"></span>
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-sm font-semibold text-gray-900">{{ formatDateStr(m.scheduled_date || m.maintenance_date) }}</span>
                    <span class="badge-mute">{{ MTYPES[m.type] || m.type }}</span>
                    <span v-if="m.status && m.status !== 'selesai'" :class="woStatusCls(m.status)">{{ woStatusLabel(m.status) }}</span>
                    <span v-if="m.wo_number" class="font-mono text-[10px] text-gray-400">{{ m.wo_number }}</span>
                  </div>
                  <p class="mt-0.5 break-words text-sm text-gray-700">{{ m.description }}</p>
                  <p class="mt-1 space-x-2 text-xs text-gray-500">
                    <span v-if="m.cost > 0">Biaya: {{ formatRupiah(m.cost) }}</span>
                    <span v-if="m.performed_by">Oleh: {{ m.performed_by }}</span>
                    <span v-if="m.condition_after">→ {{ condLabel(m.condition_after) }}</span>
                    <span v-if="m.next_due_date">Berikutnya: {{ formatDateStr(m.next_due_date) }}</span>
                  </p>
                </div>
                <button v-if="canUpdate" @click="deleteMaintenance(m)" title="Hapus catatan"
                  class="shrink-0 rounded p-1.5 text-gray-300 hover:bg-red-50 hover:text-red-500">
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
                </button>
              </div>
            </li>
          </ol>
        </div>

        <!-- Riwayat buku besar -->
        <div v-else class="p-3 sm:p-4">
          <p v-if="loadingSub" class="py-4 text-center text-sm text-gray-400">Memuat…</p>
          <p v-else-if="!movements.length" class="py-6 text-center text-sm text-gray-400">
            Belum ada riwayat. Setiap perubahan lokasi, kondisi, perawatan, dan mutasi akan tercatat di sini.
          </p>
          <ol v-else class="space-y-3">
            <li v-for="mv in movements" :key="mv.id" class="relative border-l-2 border-gray-100 pl-5">
              <span class="absolute -left-[7px] top-1 h-3 w-3 rounded-full ring-2 ring-white" :class="movDot(mv.type)"></span>
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge-mute">{{ MOVEMENT_TYPES[mv.type] || mv.type }}</span>
                <span class="text-xs text-gray-500">{{ formatDateTime(mv.created_at) }}</span>
                <span v-if="mv.actor" class="text-xs text-gray-400">· {{ mv.actor }}</span>
              </div>
              <p v-if="mv.notes" class="mt-1 break-words text-sm text-gray-700">{{ mv.notes }}</p>
              <p class="mt-0.5 space-x-2 text-xs text-gray-500">
                <span v-if="mv.qty">Qty: {{ mv.qty > 0 ? '+' : '' }}{{ mv.qty }}</span>
                <span v-if="mv.from_outlet_name || mv.to_outlet_name">
                  {{ mv.from_outlet_name || '—' }} → {{ mv.to_outlet_name || '—' }}
                </span>
                <span v-if="mv.amount">Nilai: {{ formatRupiah(mv.amount) }}</span>
                <span v-if="mv.ref_number">Ref: {{ mv.ref_number }}</span>
              </p>
            </li>
          </ol>
        </div>
      </AppCard>
    </template>

    <AppCard v-else-if="!loading">
      <p class="py-6 text-center text-sm text-gray-400">Aset tidak ditemukan atau di luar akses Anda.</p>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { assetsApi } from '@/api/assets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateStr, formatDateTime, todayDateString } from '@/utils/format.js'
import {
  CONDITIONS, MTYPES, MOVEMENT_TYPES, TRACKING_MODES,
  condCls, condLabel, statusCls, statusLabel, printAssetLabels, woStatusCls, woStatusLabel,
} from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'

const route = useRoute()
const router = useRouter()
const toast = useToastStore()
const auth = useAuthStore()
const canUpdate = auth.hasPermission('assets.update')

const asset = ref(null)
const maintenances = ref([])
const movements = ref([])
const loading = ref(true)
const loadingSub = ref(false)
const errorMsg = ref('')
const tab = ref('rawat')

// apiClient TIDAK membuka amplop untuk respons non-paginasi — lihat catatan di
// docs/perlengkapan-aset.md §11.3. Tanpa penjaga ini, res.data = undefined.
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d ?? null }

const TABS = computed(() => [
  { key: 'rawat', label: 'Perawatan', count: maintenances.value.length },
  { key: 'riwayat', label: 'Riwayat & Lokasi', count: movements.value.length },
])

const ratioPct = computed(() => {
  const beli = Number(asset.value?.purchase_price || 0) * Number(asset.value?.quantity || 1)
  if (!beli) return 0
  return Math.round((Number(asset.value?.maintenance_cost || 0) / beli) * 100)
})
const ratioText = computed(() => (asset.value?.purchase_price ? `${ratioPct.value}%` : '—'))

const warrantyState = computed(() => {
  const w = asset.value?.warranty_until
  if (!w) return null
  const habis = new Date(w + 'T23:59:59') < new Date()
  return habis
    ? { text: `Garansi habis ${formatDateStr(w)}`, cls: 'badge-mute' }
    : { text: `Garansi s/d ${formatDateStr(w)}`, cls: 'badge-info' }
})

const infoFields = computed(() => {
  const a = asset.value
  if (!a) return []
  return [
    { label: 'Outlet', value: a.outlet_name || '—' },
    { label: 'Lokasi', value: a.location || '—' },
    { label: 'Penanggung Jawab', value: a.pic_name || '—' },
    { label: 'Kategori', value: a.category || '—' },
    { label: 'Merk / Tipe', value: [a.brand, a.model].filter(Boolean).join(' ') || '—' },
    { label: 'Nomor Seri', value: a.serial_number || '—' },
    { label: 'Kode / Tag', value: a.code || '—' },
    { label: 'Mode Pencatatan', value: TRACKING_MODES[a.tracking_mode] || a.tracking_mode },
    { label: 'Sumber Perolehan', value: a.acquisition_src === 'pengadaan' ? 'Pengadaan' : (a.acquisition_src || 'manual') },
    { label: 'Umur Ekonomis', value: a.useful_life_months ? `${a.useful_life_months} bulan` : 'tidak disusutkan' },
    { label: 'Nilai Residu', value: formatRupiah(a.residual_value || 0) },
    { label: 'Terakhir Dirawat', value: a.last_maintenance ? formatDateStr(a.last_maintenance) : '—' },
  ]
})

function movDot(type) {
  if (type === 'perawatan') return 'bg-amber-500'
  if (type === 'penghapusan') return 'bg-red-500'
  if (type === 'mutasi_masuk' || type === 'mutasi_keluar') return 'bg-blue-500'
  if (type === 'pendataan' || type === 'penerimaan') return 'bg-emerald-500'
  return 'bg-gray-400'
}

function goBack() { router.push('/perlengkapan') }
function goEdit() { router.push({ path: '/perlengkapan', query: { edit: asset.value.id } }) }

async function doPrint() {
  const r = await printAssetLabels(asset.value)
  if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    asset.value = asObject(await assetsApi.get(route.params.id))
  } catch (e) {
    asset.value = null
    errorMsg.value = e?.message || 'Gagal memuat aset'
  } finally {
    loading.value = false
  }
  if (asset.value) await loadSub()
}

async function loadSub() {
  loadingSub.value = true
  try {
    const [mt, mv] = await Promise.all([
      assetsApi.maintenances(route.params.id),
      assetsApi.movements(route.params.id),
    ])
    maintenances.value = asArray(mt)
    movements.value = asArray(mv)
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat riwayat')
  } finally {
    loadingSub.value = false
  }
}

// ── Perawatan ──
const savingM = ref(false)
const mForm = ref(blankM())
function blankM() {
  return { maintenance_date: todayDateString(), type: 'rutin', description: '', cost: 0, performed_by: '', condition_after: '', next_due_date: '' }
}
async function saveMaintenance() {
  if (!mForm.value.description?.trim()) { toast.error('Deskripsi wajib diisi'); return }
  savingM.value = true
  try {
    await assetsApi.addMaintenance(route.params.id, mForm.value)
    toast.success('Perawatan dicatat')
    mForm.value = blankM()
    await load() // kondisi & biaya aset ikut berubah
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan perawatan') } finally { savingM.value = false }
}
async function deleteMaintenance(m) {
  if (!window.confirm('Hapus catatan perawatan ini?')) return
  try {
    await assetsApi.removeMaintenance(route.params.id, m.id)
    toast.success('Catatan dihapus')
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menghapus') }
}

onMounted(load)
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #f3f4f6; min-height: 40px;
}
.btn-soft:hover { background: #e5e7eb; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; overflow-wrap: anywhere; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
</style>
