<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center gap-2">
      <h2 class="text-sm font-bold text-gray-900">Material Projek</h2>
      <span v-if="summary.lines" class="text-xs text-gray-500">
        {{ summary.lines }} jenis · diterima {{ formatRupiah(summary.value_received) }} ·
        terpakai {{ formatRupiah(summary.value_used) }}
      </span>
      <span v-if="summary.unsettled_lines" class="badge-warn ml-auto">
        {{ summary.unsettled_lines }} baris masih bersisa
      </span>
    </div>

    <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
      <p>Material yang dipakai atau rusak wajib dilaporkan beserta bukti dan diverifikasi. Sisa ditentukan satu per satu sesuai kondisi fisiknya sebelum projek ditutup.</p>
      <button v-if="canManage && reportMode" class="mt-2 font-semibold underline" @click="$emit('usage')">Buat laporan pemakaian / susut</button>
      <RouterLink v-else-if="canViewUsage" :to="{ path: '/perlengkapan/penggunaan-projek', query: { project_id: projectId } }" class="mt-2 inline-block font-semibold underline">Buka laporan penggunaan &amp; bukti</RouterLink>
    </div>

    <p v-if="loading" class="rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-400">Memuat…</p>
    <p v-else-if="!materials.length" class="rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500">
      Belum ada material yang diterima untuk projek ini. Material tercatat saat belanja projek
      diserahterimakan dengan tujuan “Material projek”.
    </p>

    <ul v-else class="space-y-2">
      <li v-for="m in materials" :key="m.id" class="rounded-xl border border-gray-200 p-3">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div class="min-w-0">
            <p class="break-words font-medium text-gray-900">{{ m.name }}</p>
            <p class="text-xs text-gray-500">
              {{ formatRupiah(m.unit_cost) }}/{{ m.unit || 'unit' }}
              <span v-if="m.request_number"> · {{ m.request_number }}</span>
            </p>
          </div>
          <span :class="m.remaining > 0 ? 'badge-warn' : 'badge-ok'">
            sisa {{ fmtQty(m.remaining) }} {{ m.unit }}
          </span>
        </div>

        <!-- Identitas terlihat di layar, bukan hanya berlaku di database -->
        <p class="mt-2 text-xs text-gray-600">
          diterima <strong>{{ fmtQty(m.qty_received) }}</strong>
          = terpakai {{ fmtQty(m.qty_used) }}
          + dikembalikan {{ fmtQty(m.qty_returned) }}
          + susut {{ fmtQty(m.qty_wasted) }}
          + sisa {{ fmtQty(m.remaining) }}
        </p>
        <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-gray-100">
          <div class="h-full bg-emerald-500" :style="{ width: pct(m) + '%' }"></div>
        </div>

        <div v-if="canViewUsage" class="mt-2"><button class="text-xs text-emerald-700 underline" @click="showLogs(m)">Riwayat material</button></div>
        <div v-if="canManage && m.remaining > 0" class="mt-3 flex flex-wrap gap-2">
          <AppActionButton icon="usage" label="Tentukan Sisa" @click="openSettle(m)" />
        </div>
      </li>
    </ul>

    <AppModal v-model="logsModal" title="Riwayat material" size="lg">
      <p v-if="logsLoading" class="text-sm">Memuat riwayat…</p>
      <ul v-else class="space-y-3"><li v-for="log in logs" :key="log.id" class="border-b pb-3 text-sm"><p class="font-semibold">{{ log.type }} · {{ fmtQty(log.qty) }}</p><p>{{ log.notes }}</p><p class="text-xs text-gray-500">{{ log.created_at }} UTC · {{ log.actor }} · {{ log.ref_number }}</p><a v-if="log.evidence_url" :href="safeUrl(log.evidence_url)" target="_blank" rel="noopener noreferrer" class="text-emerald-700 underline">Lihat bukti</a></li></ul>
    </AppModal>

    <!-- Nasib sisa -->
    <AppModal v-model="settleModal" :title="`Tentukan Sisa — ${active?.name || ''}`" size="lg">
      <form class="space-y-3" @submit.prevent="saveSettle">
        <div class="flex flex-wrap gap-1.5">
          <button v-for="a in SETTLE_ACTIONS" :key="a.key" type="button" @click="settleForm.action = a.key"
            class="rounded-lg px-2.5 py-1.5 text-xs font-semibold transition"
            :class="settleForm.action === a.key ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'">
            {{ a.label }}
          </button>
        </div>
        <p class="text-xs text-gray-500">{{ actionHint }}</p>

        <div>
          <label class="lbl">Jumlah</label>
          <input v-model.number="settleForm.qty" type="number" inputmode="decimal" min="0.0001" step="0.0001"
            :max="active?.remaining" class="form-input" />
        </div>

        <div v-if="settleForm.action === 'gudang'" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Item Stok <span class="text-red-500">*</span></label>
            <SearchSelect v-model="settleForm.stock_item_id" :options="stockItems" placeholder="Pilih item stok…" />
          </div>
          <div>
            <label class="lbl">Gudang <span class="text-red-500">*</span></label>
            <SearchSelect v-model="settleForm.warehouse_id" :options="warehouses" placeholder="Pilih gudang…" />
          </div>
        </div>

        <div v-else-if="settleForm.action === 'aset'" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Outlet <span class="text-red-500">*</span></label>
            <SearchSelect v-model="settleForm.outlet_id" :options="outlets" placeholder="Pilih outlet…" />
          </div>
          <div>
            <label class="lbl">Kategori Aset</label>
            <SearchSelect v-model="settleForm.category" :options="categoryOptions" placeholder="Pilih kategori…" />
          </div>
        </div>

        <div v-else-if="settleForm.action === 'pindah'">
          <label class="lbl">Projek Tujuan <span class="text-red-500">*</span></label>
          <SearchSelect v-model="settleForm.target_project_id" :options="otherProjects" placeholder="Pilih projek…" />
        </div>

        <div>
          <label class="lbl">
            Keterangan <span v-if="settleForm.action === 'susut'" class="text-red-500">*</span>
          </label>
          <input v-model="settleForm.notes" class="form-input"
            :placeholder="settleForm.action === 'susut' ? 'mis. mengeras kena hujan' : 'opsional'" />
        </div>

        <MaterialEvidence v-model="settleEvidence" :max-files="1" photo-only label="Bukti penyelesaian sisa" @busy="evidenceBusy = $event" />
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="settleModal = false">Batal</button>
          <AppButton type="submit" :loading="saving" :disabled="evidenceBusy">Simpan</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, computed, watch, onMounted } from 'vue'
import { projectMaterialsApi } from '@/api/projectMaterials.js'
import { projectUsageApi } from '@/api/projectUsage.js'
import MaterialEvidence from '@/components/MaterialEvidence.vue'
import { safeUrl } from '@/utils/safeUrl.js'
import { outletsApi } from '@/api/outlets.js'
import { warehousesApi, stockItemsApi } from '@/api/warehouse.js'
import { assetCategoriesApi } from '@/api/assetCategories.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { formatRupiah } from '@/utils/format.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppButton from '@/components/ui/AppButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const props = defineProps({ projectId: { type: String, required: true }, reportMode: Boolean, closed: Boolean })
const emit = defineEmits(['changed', 'usage'])

const toast = useToastStore()
const auth = useAuthStore()
const canManage = computed(() => auth.hasPermission('assets.usage.create') && !props.closed)
const canViewUsage = auth.hasPermission('assets.usage.view')

// Jadi Aset adalah jalur baku: sisa projek harus didata, bukan lenyap dari
// catatan hanya karena projeknya selesai. Tiga sisanya pengecualian.
const SETTLE_ACTIONS = [
  { key: 'aset', label: 'Jadi Aset' },
  ...(auth.hasPermission('stockledger.adjust') ? [{ key: 'gudang', label: 'Kembali ke Gudang' }] : []),
  { key: 'pindah', label: 'Pindah Projek' },
]
const HINTS = {
  aset: 'Jalur baku. Sisa dicatat sebagai aset outlet projek ini, dengan nilai perolehan dari harga materialnya.',
  gudang: 'Pengecualian — hanya untuk barang yang memang ada di katalog stok. Perlu izin menambah stok.',
  susut: 'Pengecualian — hanya untuk yang benar-benar rusak atau habis. Wajib beralasan.',
  pindah: 'Pengecualian — dipakai projek lain yang sedang berjalan.',
}

const materials = ref([])
const summary = ref({ lines: 0, value_received: 0, value_used: 0, value_remaining: 0, unsettled_lines: 0 })
const loading = ref(false)
const saving = ref(false)
const outlets = ref([])
const warehouses = ref([])
const stockItems = ref([])
const projects = ref([])
const categories = ref([])
const categoryOptions = computed(() => categories.value.map(c => ({ id: c.name, name: c.name })))

const settleEvidence = ref([]), evidenceBusy = ref(false)
const logsModal = ref(false), logsLoading = ref(false), logs = ref([])
async function showLogs(m) {
  logs.value = []; logsModal.value = true; logsLoading.value = true
  try { logs.value = await projectMaterialsApi.logs(props.projectId, m.id) }
  catch(e) { toast.error(e.message) }
  finally { logsLoading.value = false }
}
const settleModal = ref(false)
const active = ref(null)
const settleForm = ref({ action: 'gudang', qty: 0, notes: '', stock_item_id: '', warehouse_id: '', outlet_id: '', category: '', target_project_id: '' })

function asArray(d) { return Array.isArray(d) ? d : (d?.data || d?.items || []) }
function asObject(d) { return d?.data ?? d ?? null }
function fmtQty(v) { return Number(v || 0).toLocaleString('id-ID', { maximumFractionDigits: 4 }) }
function pct(m) {
  if (!m.qty_received) return 0
  return Math.min(100, Math.round(((m.qty_used + m.qty_returned + m.qty_wasted) / m.qty_received) * 100))
}
const actionHint = computed(() => HINTS[settleForm.value.action] || '')
const otherProjects = computed(() => projects.value.filter(p => p.id !== props.projectId && !['selesai','batal'].includes(p.status)))

async function load() {
  loading.value = true
  try {
    const d = asObject(await projectMaterialsApi.list(props.projectId))
    materials.value = d?.materials || []
    summary.value = d?.summary || summary.value
  } catch (e) { toast.error(e?.message || 'Gagal memuat material') } finally { loading.value = false }
}

async function loadRefs() {
  if (!canManage.value) return
  try { outlets.value = asArray((await outletsApi.myOutlets())?.outlets ?? await outletsApi.myOutlets()) } catch { outlets.value = [] }
  if (auth.hasPermission('stockledger.adjust')) {
  try { warehouses.value = asArray(await warehousesApi.list()) } catch { warehouses.value = [] }
  try { stockItems.value = asArray(await stockItemsApi.list({ limit: 500 })) } catch { stockItems.value = [] }
  }
  try { projects.value = asArray(await projectUsageApi.projects()) } catch { projects.value = [] }
  try { categories.value = asArray(await assetCategoriesApi.list()) } catch { categories.value = [] }
}

function openSettle(m) {
  settleEvidence.value = []
  active.value = m
  settleForm.value = { action: 'aset', qty: m.remaining, notes: '', stock_item_id: '', warehouse_id: warehouses.value[0]?.id || '', outlet_id: '', category: '', target_project_id: '' }
  settleModal.value = true
}

async function saveSettle() {
  if (evidenceBusy.value) return
  if (!settleEvidence.value.length) { toast.error("Unggah bukti penyelesaian sisa"); return }
  if (!(settleForm.value.qty > 0)) { toast.error('Jumlah wajib diisi'); return }
  saving.value = true
  try {
    await projectMaterialsApi.settle(props.projectId, active.value.id, { ...settleForm.value, photo_url: settleEvidence.value[0] })
    toast.success('Sisa material ditentukan'); emit('changed')
    settleModal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

watch(() => props.projectId, load)
onMounted(async () => { await Promise.all([load(), loadRefs()]) })
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
.btn-mini { min-height: 36px; border-radius: .5rem; padding: .35rem .75rem; font-size: .75rem; font-weight: 600; }
</style>
