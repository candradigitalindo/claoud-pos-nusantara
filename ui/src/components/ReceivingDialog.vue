<template>
  <AppModal :model-value="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    :title="draft ? `Serah Terima ${draft.request_number}` : 'Serah Terima Pengadaan'" size="2xl">
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="animate-pulse space-y-2">
        <div class="h-4 w-2/3 rounded bg-gray-200"></div><div class="h-3 w-1/2 rounded bg-gray-100"></div>
      </div>
    </div>

    <AppAlert v-else-if="errorMsg" type="error" :message="errorMsg" />

    <!-- Draf terbaca tetapi dokumennya tidak bisa diterima: jasa (diselesaikan
         lewat tombol Selesai) atau belum disetujui / sudah batal. -->
    <AppAlert v-else-if="draft && !receivable" type="warning" :message="notReceivableMsg" />

    <div v-else-if="draft" class="space-y-4">
      <p v-if="isBahan" class="rounded-lg bg-emerald-50 p-3 text-xs text-emerald-800">
        Ini pengajuan <strong>bahan baku</strong>: seluruh barisnya diterima ke <strong>stok gudang</strong>
        (Gudang Induk), berapa pun harganya, dan tidak menjadi aset.
      </p>
      <p v-else class="rounded-lg bg-emerald-50 p-3 text-xs text-emerald-800">
        Tentukan tujuan tiap barang yang datang. Barang yang sudah ada di <strong>katalog stok</strong>
        (bahan dapur) langsung diarahkan ke gudang berapa pun harganya; sisanya yang bernilai
        <strong>{{ formatRupiah(draft.capitalization_min) }}</strong> ke atas diusulkan menjadi
        <strong>aset</strong>.
      </p>

      <div v-if="!draft.outlet_id && !isBahan" class="rounded-lg border border-amber-200 bg-amber-50 p-3">
        <label class="lbl">Outlet tujuan aset <span class="text-red-500">*</span></label>
        <SearchSelect v-model="outletId" :options="outlets" placeholder="Pilih outlet…" />
        <p class="mt-1 text-[11px] text-amber-700">Pengajuan ini tidak terikat outlet, jadi tujuan asetnya dipilih di sini.</p>
      </div>

      <p v-if="!openLines.length" class="rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500">
        Semua barang pada pengajuan ini sudah dicatat. Tidak ada yang tersisa untuk diterima.
      </p>

      <!-- Pengantar barang: vendor langsung, atau petugas logistik yang
           pengambilannya masih dalam pengantaran (ditutup saat diterima). -->
      <div v-if="pickups.length" class="rounded-xl border border-blue-200 bg-blue-50/50 p-3 text-xs">
        <p class="mb-1.5 text-sm font-semibold text-gray-800">Diantar oleh</p>
        <label class="flex items-start gap-2 py-1">
          <input v-model="pickupId" type="radio" value="" class="mt-0.5" />
          <span>Vendor langsung</span>
        </label>
        <label v-for="p in pickups" :key="p.id" class="flex items-start gap-2 py-1">
          <input v-model="pickupId" type="radio" :value="p.id" class="mt-0.5" />
          <span class="min-w-0 break-words">Petugas logistik <strong>{{ p.picked_by || '-' }}</strong> · diambil {{ formatDateTime(p.picked_at) }}<template v-if="p.notes"> — {{ p.notes }}</template></span>
        </label>
      </div>

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

      <!-- Baris yang bukan wewenang pengguna ini (mis. baris dapur bagi bagian
           Aset) hanya ditampilkan, tidak dikirim. -->
      <template v-for="line in group.lines" :key="line.pr_item_key">
      <div v-if="!canHandle(line)" class="rounded-xl border border-dashed border-gray-200 bg-gray-50 p-3">
        <p class="break-words font-semibold text-gray-500">{{ line.name }}</p>
        <p class="text-xs text-gray-400">
          {{ line.remaining }} {{ line.unit || 'unit' }} belum diterima · dicatat oleh
          {{ line.kind === 'dapur' ? 'petugas gudang (izin penyesuaian stok)' : 'bagian Aset (izin tambah aset)' }}
        </p>
      </div>

      <div v-else class="rounded-xl border border-gray-200 p-3">
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
          <button v-for="d in lineDests(line)" :key="d.key" type="button" @click="form[line.pr_item_key].destination = d.key"
            class="rounded-lg px-2.5 py-1.5 text-xs font-semibold transition"
            :class="form[line.pr_item_key].destination === d.key ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'">
            {{ d.label }}
          </button>
        </div>

        <!-- Tujuan: aset -->
        <div v-if="form[line.pr_item_key].destination === 'aset'" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div>
            <label class="lbl">Kategori</label>
            <SearchSelect v-model="form[line.pr_item_key].category" :options="categoryOptions" placeholder="Pilih kategori…" />
          </div>
          <div>
            <label class="lbl">Mode</label>
            <SearchableSelect v-model="form[line.pr_item_key].tracking_mode" class="form-input">
              <option value="tunggal">Tunggal (bernomor per unit)</option>
              <option value="massal">Massal (satu baris)</option>
            </SearchableSelect>
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
          <!-- Gudang yang MEMBUTUHKAN vs gudang yang MENERIMA: barang dapur masuk
               Gudang Induk dulu, lalu diteruskan lewat Transfer Stok yang berfoto. -->
          <p v-if="draft.target_warehouse_name" class="text-[11px] text-gray-500 sm:col-span-2">
            Kebutuhan: <strong>{{ draft.target_warehouse_name }}</strong><template v-if="draft.target_warehouse_type === 'outlet'">.
            Terima di Gudang Induk, lalu teruskan lewat <strong>Transfer Stok</strong> (berfoto) — jangan langsung ke gudang outlet.</template>
          </p>
          <p v-if="!canStock" class="text-[11px] text-amber-700 sm:col-span-2">
            Anda tidak punya izin menambah stok. Baris ini berpindah ke antrean Gudang Induk dan
            dokumen tetap terbuka sampai gudang mencatatnya.
          </p>
        </div>

        <!-- Tujuan: habis pakai — selalu beralasan (server: minimal 10 huruf),
             supaya barang tidak hilang dari catatan tanpa sebab. -->
        <div v-else class="mt-3">
          <label class="lbl">Alasan habis pakai (wajib, minimal {{ EXPENSE_REASON_MIN }} huruf)</label>
          <input v-model="form[line.pr_item_key].reason" class="form-input"
            placeholder="mis. langsung dipakai habis di dapur hari itu juga" />
          <p v-if="(form[line.pr_item_key].qty || 0) > 0 && reasonLength(form[line.pr_item_key].reason) < EXPENSE_REASON_MIN"
            class="mt-1 text-[11px] text-red-600">Alasan masih kurang dari {{ EXPENSE_REASON_MIN }} huruf.</p>
          <p v-if="needsReason(line)" class="mt-1 text-[11px] text-amber-700">
            Nilainya {{ formatRupiah(line.unit_price * (form[line.pr_item_key].qty || 0)) }} — barang sebesar ini
            sebaiknya dicatat sebagai aset bila masih dipakai berulang.
          </p>
        </div>
      </div>
      </template>
      </div>

      <p v-if="openLines.length && !handledLines.length" class="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">
        Tidak ada baris yang bisa Anda catat di sini — baris peralatan dicatat bagian Aset (izin tambah aset),
        baris dapur/stok dicatat petugas gudang (izin penyesuaian stok).
      </p>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button type="button" class="btn-ghost" @click="$emit('update:modelValue', false)">Batal</button>
        <AppButton v-if="draft && receivable && handledLines.length" :loading="saving" @click="submit">Catat Penerimaan</AppButton>
      </div>
    </template>
  </AppModal>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, watch } from 'vue'
import { receivingApi } from '@/api/purchase.js'
import { outletsApi } from '@/api/outlets.js'
import { warehousesApi, stockItemsApi } from '@/api/warehouse.js'
import { assetCategoriesApi } from '@/api/assetCategories.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { procurementPerm, EXPENSE_REASON_MIN, reasonLength, grnNumbersOf, isPhysicalPurchaseType } from '@/utils/procurement.js'
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
const canAsset = auth.hasPermission('assets.create')

// Tujuan "Material projek" hanya muncul bila pengajuannya terikat projek —
// menawarkannya di belanja biasa hanya membingungkan.
const ALL_DESTS = [
  { key: 'aset', label: 'Aset' },
  { key: 'material', label: 'Material Projek', projectOnly: true },
  { key: 'stok', label: 'Stok Gudang' },
  { key: 'habis', label: 'Habis Pakai' },
]
// Bahan baku hanya boleh ke stok gudang (server juga menolak tujuan lain).
const isBahan = computed(() => draft.value?.request_type === 'bahan')

const draft = ref(null)

// Server menandai draf yang boleh diterima (can_receive): dokumen berkomitmen
// dan bukan jasa. Klien lama tanpa field itu tetap dianggap boleh.
const receivable = computed(() => !!draft.value
  && isPhysicalPurchaseType(draft.value.request_type || 'barang')
  && draft.value.can_receive !== false)
const notReceivableMsg = computed(() => {
  if (draft.value && !isPhysicalPurchaseType(draft.value.request_type || 'barang')) {
    return 'Pengajuan jasa diselesaikan lewat tombol Selesai di halaman Pengadaan Jasa, bukan Serah Terima barang.'
  }
  return 'Pengajuan ini belum bisa diterima — barang baru bisa diserahterimakan setelah pengajuan disetujui (dan selama tidak dibatalkan/ditolak).'
})

// Tujuan yang sah per jenis baris — server memvalidasi hal yang sama:
// baris dapur/stok → stok gudang; baris peralatan → aset atau habis pakai
// (+ material projek bila pengajuannya terikat projek); bahan baku → stok saja.
function lineDests(line) {
  if (isBahan.value || line.kind === 'dapur') return ALL_DESTS.filter(d => d.key === 'stok')
  return ALL_DESTS.filter(d => d.key !== 'stok' && (!d.projectOnly || draft.value?.project_id))
}

// Siapa boleh mencatat baris apa (server memeriksa hal yang sama): pemegang
// izin buat pengajuan untuk jenis ini → semua baris; tanpa itu, bagian Aset
// (assets.create) → baris peralatan, petugas gudang (stockledger.adjust) →
// baris dapur/stok.
const canSubmitType = computed(() => auth.hasPermission(procurementPerm(draft.value?.request_type, 'submit')))
function canHandle(line) {
  if (canSubmitType.value) return true
  return line.kind === 'dapur' ? canStock : canAsset
}
const form = ref({})
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const outletId = ref('')
const photoURL = ref('')
// Pengambilan oleh petugas logistik yang barangnya diterima sekarang
// ('' = diantar vendor langsung). Bawaan: pengambilan terbaru bila ada.
const pickupId = ref('')
const pickups = computed(() => draft.value?.pickups || [])
const outlets = ref([])
const warehouses = ref([])
const stockItems = ref([])
const categories = ref([])
const categoryOptions = computed(() => categories.value.map(c => ({ id: c.name, name: c.name })))

function asArray(d) { return Array.isArray(d) ? d : (d?.data || d?.items || []) }
function asObject(d) { return d?.data ?? d ?? null }

const openLines = computed(() => (draft.value?.lines || [])
  .filter(l => l.remaining > 0)
  .filter(l => !props.kind || (props.kind === 'dapur' ? l.kind === 'dapur' : l.kind !== 'dapur')))
// Baris terbuka yang boleh dicatat pengguna ini — hanya ini yang dikirim.
const handledLines = computed(() => openLines.value.filter(canHandle))

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

// Habis pakai selalu wajib beralasan; ambang nilai ini hanya memunculkan
// pengingat tambahan untuk barang bernilai besar (angkanya dari server).
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
    pickupId.value = draft.value?.pickups?.[0]?.id || ''
    form.value = Object.fromEntries((draft.value?.lines || []).map(l => {
      // Usulan tujuan server dipakai bila sah untuk jenis barisnya.
      const allowed = lineDests(l).map(d => d.key)
      return [l.pr_item_key, {
        qty: l.remaining,
        destination: allowed.includes(l.destination) ? l.destination : (allowed[0] || l.destination),
        category: '',
        tracking_mode: l.qty === 1 || l.unit_price >= 5000000 ? 'tunggal' : 'massal',
        location: '',
        stock_item_id: l.suggested_stock_item_id || '',
        // Bawaan: Gudang Induk (dari server), bukan sekadar gudang pertama di daftar.
        warehouse_id: draft.value?.receiving_warehouse_id || warehouses.value[0]?.id || '',
        reason: '',
      }]
    }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat rincian pengajuan'
  } finally { loading.value = false }
}

async function loadRefs() {
  try { outlets.value = asArray((await outletsApi.myOutlets())?.outlets ?? await outletsApi.myOutlets()) } catch { outlets.value = [] }
  try { warehouses.value = asArray(await warehousesApi.list()) } catch { warehouses.value = [] }
  try { stockItems.value = asArray(await stockItemsApi.list({ limit: 500 })) } catch { stockItems.value = [] }
  // Kategori aset hanya bisa dibaca pemegang assets.view (izin rutenya);
  // tanpa izin itu panggilannya selalu 403.
  if (auth.hasPermission('assets.view')) {
    try { categories.value = asArray(await assetCategoriesApi.list()) } catch { categories.value = [] }
  }
}

watch(() => props.modelValue, async (open) => {
  if (!open) return
  if (!warehouses.value.length) await loadRefs()
  await load()
})

async function submit() {
  // Hanya baris wewenang pengguna yang diisi jumlahnya. Kiriman tanpa barang
  // ditolak di sini: kalau sampai server, dokumen ikut terkunci tanpa ada
  // barang yang benar-benar diterima.
  const byKey = Object.fromEntries(openLines.value.map(l => [l.pr_item_key, l]))
  const lines = handledLines.value
    .map(l => ({ pr_item_key: l.pr_item_key, ...form.value[l.pr_item_key] }))
    .filter(l => (Number(l.qty) || 0) > 0)
  if (!lines.length) { toast.error('Isi jumlah barang yang diterima, minimal pada satu baris'); return }
  const over = lines.find(l => l.qty > (byKey[l.pr_item_key]?.remaining ?? 0))
  if (over) { toast.error(`Jumlah "${byKey[over.pr_item_key]?.name}" melebihi sisa yang belum diterima`); return }
  const bad = lines.find(l => l.destination === 'stok' && canStock && (!l.stock_item_id || !l.warehouse_id))
  if (bad) { toast.error('Baris bertujuan gudang perlu item stok dan gudang'); return }
  const noReason = lines.find(l => l.destination === 'habis' && reasonLength(l.reason) < EXPENSE_REASON_MIN)
  if (noReason) {
    toast.error(`Alasan habis pakai untuk "${byKey[noReason.pr_item_key]?.name}" wajib diisi, minimal ${EXPENSE_REASON_MIN} huruf`)
    return
  }
  if (!draft.value.outlet_id && lines.some(l => l.destination === 'aset') && !outletId.value) {
    toast.error('Pilih outlet tujuan aset'); return
  }
  if (!photoURL.value) { toast.error('Foto barang saat diterima wajib diunggah'); return }
  saving.value = true
  try {
    const res = asObject(await receivingApi.receive(props.purchaseRequestId, {
      outlet_id: outletId.value, photo_url: photoURL.value, desk: props.kind || '', lines,
      pickup_id: pickupId.value || undefined,
    }))
    // Satu penerimaan bisa menerbitkan beberapa GRN (per gudang & dokumen
    // sumber): grn_numbers berupa array, grn_number gabungan dipisah koma.
    const grns = grnNumbersOf(res)
    const msg = res?.message || 'Penerimaan dicatat'
    const grnText = grns.length && !grns.every(g => msg.includes(g))
      ? ` · ${grns.length > 1 ? `${grns.length} GRN` : 'GRN'}: ${grns.join(', ')}`
      : ''
    toast.success(msg + grnText, grns.length ? 8000 : undefined)
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
