<template>
  <AppModal :model-value="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    :title="draft ? `Serah Terima ${draft.request_number}` : 'Serah Terima Pengadaan'" size="2xl">
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="animate-pulse space-y-2">
        <div class="h-4 w-2/3 rounded bg-gray-200"></div><div class="h-3 w-1/2 rounded bg-gray-100"></div>
      </div>
    </div>

    <AppAlert v-else-if="errorMsg" type="error" :message="errorMsg" />

    <div v-else-if="draft" class="space-y-4">
      <p class="rounded-lg bg-emerald-50 p-3 text-xs text-emerald-800">
        Tentukan tujuan tiap barang yang datang. Barang yang sudah ada di <strong>katalog stok</strong>
        (bahan dapur) langsung diarahkan ke gudang berapa pun harganya; sisanya yang bernilai
        <strong>{{ formatRupiah(draft.capitalization_min) }}</strong> ke atas diusulkan menjadi
        <strong>aset</strong>.
      </p>

      <div v-if="!draft.outlet_id" class="rounded-lg border border-amber-200 bg-amber-50 p-3">
        <label class="lbl">Outlet tujuan aset <span class="text-red-500">*</span></label>
        <SearchSelect v-model="outletId" :options="outlets" placeholder="Pilih outlet…" />
        <p class="mt-1 text-[11px] text-amber-700">Pengajuan ini tidak terikat outlet, jadi tujuan asetnya dipilih di sini.</p>
      </div>

      <p v-if="!openLines.length" class="rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500">
        Semua barang pada pengajuan ini sudah dicatat. Tidak ada yang tersisa untuk diterima.
      </p>

      <!-- Bukti barang benar-benar sampai dari tim purchasing. -->
      <div class="rounded-xl border border-emerald-200 bg-emerald-50/50 p-3">
        <PhotoCapture v-model="photoURL" label="Foto Barang Saat Diterima"
          hint="Ambil foto barangnya di tempat serah terima. Salinannya otomatis dikirim ke email cadangan." />
      </div>

      <div v-for="group in groups" :key="group.key" class="space-y-3">
        <div v-if="groups.length > 1" class="flex items-center gap-2">
          <span class="text-xs font-bold uppercase tracking-wide text-gray-500">{{ group.title }}</span>
          <span class="text-[11px] text-gray-400">{{ group.hint }}</span>
        </div>

      <div v-for="line in group.lines" :key="line.pr_item_key" class="rounded-xl border border-gray-200 p-3">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div class="min-w-0">
            <p class="break-words font-semibold text-gray-900">{{ line.name }}</p>
            <p class="text-xs text-gray-500">
              {{ line.entry_name }} · {{ formatRupiah(line.unit_price) }}/{{ line.unit || 'unit' }}
              <span v-if="line.recorded"> · sudah dicatat {{ line.recorded }}</span>
            </p>
          </div>
          <div class="flex items-center gap-2">
            <label class="text-xs text-gray-400">Jumlah</label>
            <input v-model.number="form[line.pr_item_key].qty" type="number" inputmode="numeric" min="0"
              :max="line.remaining" class="form-input w-20 text-right" />
            <span class="text-xs text-gray-500">/ {{ line.remaining }}</span>
          </div>
        </div>

        <div class="mt-3 flex flex-wrap gap-1.5">
          <button v-for="d in DESTS" :key="d.key" type="button" @click="form[line.pr_item_key].destination = d.key"
            class="rounded-lg px-2.5 py-1.5 text-xs font-semibold transition"
            :class="form[line.pr_item_key].destination === d.key ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'">
            {{ d.label }}
          </button>
        </div>

        <!-- Tujuan: aset -->
        <div v-if="form[line.pr_item_key].destination === 'aset'" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="lbl">Kategori</label>
            <input v-model="form[line.pr_item_key].category" class="form-input" placeholder="mis. Elektronik" />
          </div>
          <div>
            <label class="lbl">Mode</label>
            <select v-model="form[line.pr_item_key].tracking_mode" class="form-input">
              <option value="tunggal">Tunggal (bernomor per unit)</option>
              <option value="massal">Massal (satu baris)</option>
            </select>
          </div>
          <div>
            <label class="lbl">Lokasi</label>
            <input v-model="form[line.pr_item_key].location" class="form-input" placeholder="mis. Dapur" />
          </div>
        </div>

        <!-- Tujuan: material projek -->
        <div v-else-if="form[line.pr_item_key].destination === 'material'" class="mt-3">
          <label class="lbl">Titik simpan di lokasi</label>
          <input v-model="form[line.pr_item_key].location" class="form-input" placeholder="mis. gudang sementara lokasi" />
          <p class="mt-1 text-[11px] text-gray-500">
            Masuk buku material projek. Pemakaian dan sisanya dicatat di halaman Projek sampai projek ditutup.
          </p>
        </div>

        <!-- Tujuan: stok gudang -->
        <div v-else-if="form[line.pr_item_key].destination === 'stok'" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Item Stok <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form[line.pr_item_key].stock_item_id" :options="stockItems" placeholder="Pilih item stok…" />
          </div>
          <div>
            <label class="lbl">Gudang <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form[line.pr_item_key].warehouse_id" :options="warehouses" placeholder="Pilih gudang…" />
          </div>
          <p v-if="!canStock" class="text-[11px] text-amber-700 sm:col-span-2">
            Anda tidak punya izin menambah stok. Baris ini akan masuk antrean penerimaan gudang —
            serah terima tetap bisa diselesaikan.
          </p>
        </div>

        <!-- Tujuan: habis pakai -->
        <div v-else class="mt-3">
          <label class="lbl">Alasan {{ needsReason(line) ? '(wajib)' : '(opsional)' }}</label>
          <input v-model="form[line.pr_item_key].reason" class="form-input"
            placeholder="mis. langsung dipakai hari itu juga" />
          <p v-if="needsReason(line)" class="mt-1 text-[11px] text-amber-700">
            Nilainya {{ formatRupiah(line.unit_price * (form[line.pr_item_key].qty || 0)) }} — barang sebesar ini
            perlu keterangan agar tidak hilang dari catatan tanpa sebab.
          </p>
        </div>
      </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button type="button" class="btn-ghost" @click="$emit('update:modelValue', false)">Batal</button>
        <AppButton v-if="draft && openLines.length" :loading="saving" @click="submit">Catat Penerimaan</AppButton>
      </div>
    </template>
  </AppModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { receivingApi } from '@/api/purchase.js'
import { outletsApi } from '@/api/outlets.js'
import { warehousesApi, stockItemsApi } from '@/api/warehouse.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah } from '@/utils/format.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  purchaseRequestId: { type: String, default: '' },
  // Batasi baris ke satu meja: "perlengkapan" (bagian Aset) atau "dapur"
  // (Gudang Induk). Kosong = tampilkan semua, dipakai dari halaman Pengadaan.
  kind: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue', 'done'])

const toast = useToastStore()
const auth = useAuthStore()
const canStock = auth.hasPermission('stockledger.adjust')

// Tujuan "Material projek" hanya muncul bila pengajuannya terikat projek —
// menawarkannya di belanja biasa hanya membingungkan.
const ALL_DESTS = [
  { key: 'aset', label: 'Aset' },
  { key: 'material', label: 'Material Projek', projectOnly: true },
  { key: 'stok', label: 'Stok Gudang' },
  { key: 'habis', label: 'Habis Pakai' },
]
const DESTS = computed(() => ALL_DESTS.filter(d => !d.projectOnly || draft.value?.project_id))

const draft = ref(null)
const form = ref({})
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const outletId = ref('')
const photoURL = ref('')
const outlets = ref([])
const warehouses = ref([])
const stockItems = ref([])

function asArray(d) { return Array.isArray(d) ? d : (d?.data || d?.items || []) }
function asObject(d) { return d?.data ?? d ?? null }

const openLines = computed(() => (draft.value?.lines || [])
  .filter(l => l.remaining > 0)
  .filter(l => !props.kind || (props.kind === 'dapur' ? l.kind === 'dapur' : l.kind !== 'dapur')))

// Tanggung jawab dipisah di layar: tim aset mengurus perlengkapan, gudang
// mengurus bahan dapur. Keduanya tetap satu dokumen supaya tidak ada barang
// yang lolos tanpa tujuan, tapi siapa mengerjakan apa terlihat jelas.
const groups = computed(() => {
  const dapur = openLines.value.filter(l => l.kind === 'dapur')
  const projek = openLines.value.filter(l => l.kind === 'projek')
  const lainnya = openLines.value.filter(l => l.kind !== 'dapur' && l.kind !== 'projek')
  const out = []
  if (lainnya.length) {
    out.push({
      key: 'perlengkapan', lines: lainnya,
      title: 'Perlengkapan & aset',
      hint: 'dicatat oleh tim aset',
    })
  }
  if (projek.length) {
    out.push({
      key: 'projek', lines: projek,
      title: 'Material projek',
      hint: 'diterima tim aset · habis dikonsumsi projek, sisanya jadi aset',
    })
  }
  if (dapur.length) {
    out.push({
      key: 'dapur', lines: dapur,
      title: 'Barang dapur / gudang',
      hint: canStock ? 'masuk buku stok' : 'diteruskan ke antrean gudang',
    })
  }
  return out
})

// Ambang alasan mengikuti aturan server; angkanya dikonfirmasi ulang di sana.
function needsReason(line) {
  const qty = form.value[line.pr_item_key]?.qty || 0
  return line.unit_price * qty >= 1000000
}

async function load() {
  if (!props.purchaseRequestId) return
  loading.value = true; errorMsg.value = ''
  try {
    draft.value = asObject(await receivingApi.draft(props.purchaseRequestId))
    outletId.value = draft.value?.outlet_id || ''
    photoURL.value = ''
    form.value = Object.fromEntries((draft.value?.lines || []).map(l => [l.pr_item_key, {
      qty: l.remaining,
      destination: l.destination,
      category: '',
      tracking_mode: l.qty === 1 || l.unit_price >= 5000000 ? 'tunggal' : 'massal',
      location: '',
      stock_item_id: l.suggested_stock_item_id || '',
      warehouse_id: warehouses.value[0]?.id || '',
      reason: '',
    }]))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat rincian pengajuan'
  } finally { loading.value = false }
}

async function loadRefs() {
  try { outlets.value = asArray((await outletsApi.myOutlets())?.outlets ?? await outletsApi.myOutlets()) } catch { outlets.value = [] }
  try { warehouses.value = asArray(await warehousesApi.list()) } catch { warehouses.value = [] }
  try { stockItems.value = asArray(await stockItemsApi.list({ limit: 500 })) } catch { stockItems.value = [] }
}

watch(() => props.modelValue, async (open) => {
  if (!open) return
  if (!warehouses.value.length) await loadRefs()
  await load()
})

async function submit() {
  const lines = openLines.value.map(l => ({ pr_item_key: l.pr_item_key, ...form.value[l.pr_item_key] }))
  const bad = lines.find(l => l.destination === 'stok' && canStock && (!l.stock_item_id || !l.warehouse_id))
  if (bad) { toast.error('Baris bertujuan gudang perlu item stok dan gudang'); return }
  if (!draft.value.outlet_id && lines.some(l => l.destination === 'aset') && !outletId.value) {
    toast.error('Pilih outlet tujuan aset'); return
  }
  if (!photoURL.value) { toast.error('Foto barang saat diterima wajib diunggah'); return }
  saving.value = true
  try {
    const res = asObject(await receivingApi.receive(props.purchaseRequestId, {
      outlet_id: outletId.value, photo_url: photoURL.value, desk: props.kind || '', lines,
    }))
    toast.success(res?.message || 'Penerimaan dicatat')
    emit('update:modelValue', false)
    emit('done', res)
  } catch (e) { toast.error(e?.message || 'Gagal mencatat penerimaan') } finally { saving.value = false }
}
</script>

<style scoped>
.form-input {
  width: 100%; padding: .45rem .65rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost {
  padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600;
  color: #374151; background: #f3f4f6; min-height: 40px;
}
.btn-ghost:hover { background: #e5e7eb; }
</style>
