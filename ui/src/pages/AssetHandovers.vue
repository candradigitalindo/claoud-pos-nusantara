<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Distribusi Aset ke PIC</h1>
        <p class="mt-0.5 text-sm text-gray-500">
          Barang yang sudah diterima bagian aset diteruskan ke PIC yang mengajukan pembeliannya.
        </p>
      </div>
      <AppButton v-if="canHandover && awaiting.length" class="w-full sm:w-auto" @click="openCreate">
        + Serahkan Aset
      </AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Menunggu diserahkan -->
    <AppCard>
      <div class="flex flex-wrap items-center gap-2">
        <h2 class="text-sm font-bold text-gray-900">Menunggu Diserahkan</h2>
        <span v-if="awaiting.length" class="badge-warn">{{ awaiting.length }} aset</span>
      </div>
      <p v-if="loadingAwaiting" class="mt-3 text-sm text-gray-400">Memuat…</p>
      <p v-else-if="!awaiting.length" class="mt-3 rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500">
        Tidak ada aset yang menunggu. Setiap aset yang diterima sudah punya penanggung jawab.
      </p>
      <ul v-else class="mt-3 divide-y divide-gray-100">
        <li v-for="a in awaiting" :key="a.id" class="flex flex-wrap items-center gap-2 py-2">
          <div class="min-w-0 flex-1">
            <p class="break-words text-sm font-medium text-gray-900">{{ a.name }}</p>
            <p class="font-mono text-[11px] text-gray-400">{{ a.asset_no }} · {{ a.outlet_name }} · {{ a.quantity }} {{ a.unit }}</p>
          </div>
          <span class="text-xs text-gray-500">{{ formatRupiah(a.purchase_price) }}</span>
        </li>
      </ul>
    </AppCard>

    <!-- Riwayat serah terima -->
    <AppCard :padding="false">
      <p class="border-b border-gray-100 p-3 text-xs font-bold uppercase tracking-wide text-gray-500">
        Riwayat Serah Terima
      </p>
      <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">
        Belum ada serah terima aset ke PIC.
      </div>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="h in rows" :key="h.id" class="p-4">
          <button class="w-full text-left" @click="openDetail(h)">
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-mono text-sm font-semibold text-gray-900">{{ h.handover_number }}</p>
                <p class="text-xs text-gray-500">
                  diserahkan ke <strong class="text-gray-700">{{ h.pic_name }}</strong>
                  <span v-if="h.pic_position"> ({{ h.pic_position }})</span>
                  <span v-if="h.outlet_name"> · {{ h.outlet_name }}</span>
                </p>
              </div>
              <span class="text-xs text-gray-400">{{ h.created_at }}</span>
            </div>
          </button>
        </li>
      </ul>
    </AppCard>

    <!-- Form serah terima -->
    <AppModal v-model="createModal" title="Serahkan Aset ke PIC" size="2xl">
      <form class="space-y-3" @submit.prevent="save">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Nama PIC Penerima <span class="text-red-500">*</span></label>
            <input v-model="form.pic_name" class="form-input" placeholder="Nama orang yang menerima" required />
          </div>
          <div>
            <label class="lbl">Jabatan / Bagian</label>
            <input v-model="form.pic_position" class="form-input" placeholder="mis. Kepala Dapur" />
          </div>
        </div>
        <div>
          <label class="lbl">Lokasi Penempatan</label>
          <input v-model="form.location" class="form-input" placeholder="mis. Dapur belakang" />
        </div>

        <div>
          <label class="lbl">Aset yang Diserahkan <span class="text-red-500">*</span></label>
          <ul class="max-h-64 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-2">
            <li v-for="a in awaiting" :key="a.id" class="rounded-lg p-2" :class="picked[a.id] ? 'bg-emerald-50' : 'hover:bg-gray-50'">
              <label class="flex items-start gap-2">
                <input type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-emerald-600"
                  :checked="!!picked[a.id]" @change="toggle(a, $event.target.checked)" />
                <span class="min-w-0 flex-1">
                  <span class="block break-words text-sm font-medium text-gray-900">{{ a.name }}</span>
                  <span class="block font-mono text-[11px] text-gray-500">{{ a.asset_no }} · {{ a.quantity }} {{ a.unit }} · {{ a.outlet_name }}</span>
                </span>
              </label>
            </li>
          </ul>
        </div>

        <div class="rounded-xl border border-emerald-200 bg-emerald-50/50 p-3">
          <PhotoCapture v-model="form.photo_url" label="Foto Serah Terima"
            hint="Foto saat barang diserahkan ke PIC. Salinannya dikirim ke email cadangan." />
        </div>

        <div>
          <label class="lbl">Catatan</label>
          <input v-model="form.notes" class="form-input" placeholder="Opsional" />
        </div>

        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="createModal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Serahkan</AppButton>
        </div>
      </form>
    </AppModal>

    <!-- Detail -->
    <AppModal v-model="detailModal" :title="detail ? `Serah Terima ${detail.handover_number}` : ''" size="lg">
      <div v-if="detail" class="space-y-3">
        <dl class="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
          <div class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">PIC penerima</dt><dd class="font-medium text-gray-800">{{ detail.pic_name }}</dd>
          </div>
          <div class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">Jabatan</dt><dd class="text-gray-800">{{ detail.pic_position || '—' }}</dd>
          </div>
          <div class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">Lokasi</dt><dd class="text-gray-800">{{ detail.location || '—' }}</dd>
          </div>
          <div class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">Diserahkan oleh</dt><dd class="text-gray-800">{{ detail.handed_by || '—' }} · {{ detail.created_at }}</dd>
          </div>
        </dl>

        <div>
          <p class="text-xs font-bold uppercase tracking-wide text-gray-500">Aset</p>
          <ul class="mt-1 divide-y divide-gray-100">
            <li v-for="it in detail.items" :key="it.id" class="flex items-center justify-between gap-2 py-1.5">
              <span class="min-w-0 break-words text-sm text-gray-800">{{ it.asset_name }}</span>
              <span class="font-mono text-[11px] text-gray-400">{{ it.asset_no }} · {{ it.qty }} {{ it.unit }}</span>
            </li>
          </ul>
        </div>

        <div v-if="detail.photos?.length">
          <p class="text-xs font-bold uppercase tracking-wide text-gray-500">Bukti Foto</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <a v-for="p in detail.photos" :key="p.id" class="w-28" :href="p.drive_url || p.photo_url" target="_blank" rel="noopener">
              <img :src="photoSrc(p)" alt="Bukti" class="h-24 w-28 rounded-lg border border-gray-200 object-cover"
                @error="$event.target.src = photoFallback(p)" />
              <p class="mt-1 text-[10px]" :class="backupCls(p.backup_status)">{{ backupLabel(p.backup_status) }}</p>
            </a>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="detailModal = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { assetHandoversApi } from '@/api/handover.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'
import { photoSrc, photoFallback, backupLabel, backupCls } from '@/utils/assets.js'

const toast = useToastStore()
const auth = useAuthStore()
const canHandover = auth.hasPermission('assets.update')

const rows = ref([])
const awaiting = ref([])
const loading = ref(false)
const loadingAwaiting = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const createModal = ref(false)
const detailModal = ref(false)
const detail = ref(null)
const picked = ref({})
const form = ref(blankForm())

function blankForm() { return { pic_name: '', pic_position: '', location: '', notes: '', photo_url: '' } }
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d ?? null }

async function load() {
  loading.value = true; errorMsg.value = ''
  try { rows.value = asArray(await assetHandoversApi.list()) }
  catch (e) { errorMsg.value = e?.message || 'Gagal memuat serah terima' }
  finally { loading.value = false }
}
async function loadAwaiting() {
  loadingAwaiting.value = true
  try { awaiting.value = asArray(await assetHandoversApi.awaiting()) }
  catch { awaiting.value = [] } finally { loadingAwaiting.value = false }
}

function openCreate() { form.value = blankForm(); picked.value = {}; createModal.value = true }
function toggle(a, checked) {
  const next = { ...picked.value }
  if (checked) next[a.id] = { qty: a.quantity }
  else delete next[a.id]
  picked.value = next
}

async function save() {
  const items = Object.entries(picked.value).map(([asset_id, v]) => ({ asset_id, qty: v.qty }))
  if (!form.value.pic_name?.trim()) { toast.error('Nama PIC wajib diisi'); return }
  if (!items.length) { toast.error('Pilih minimal satu aset'); return }
  if (!form.value.photo_url) { toast.error('Foto serah terima wajib diunggah'); return }
  saving.value = true
  try {
    await assetHandoversApi.create({ ...form.value, items })
    toast.success('Aset diserahkan ke PIC')
    createModal.value = false
    await Promise.all([load(), loadAwaiting()])
  } catch (e) { toast.error(e?.message || 'Gagal menyerahkan') } finally { saving.value = false }
}

async function openDetail(h) {
  detailModal.value = true
  detail.value = null
  try { detail.value = asObject(await assetHandoversApi.get(h.id)) }
  catch (e) { toast.error(e?.message || 'Gagal memuat dokumen'); detailModal.value = false }
}

onMounted(async () => { await Promise.all([load(), loadAwaiting()]) })
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
</style>
