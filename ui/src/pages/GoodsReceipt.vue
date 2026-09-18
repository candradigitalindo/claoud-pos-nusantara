<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Penerimaan Barang</h1>
        <p class="text-sm text-gray-500 mt-0.5">Catat stok masuk ke gudang (pembelian, stok awal, kiriman supplier).</p>
      </div>
      <!-- Backend mensyaratkan stockledger.adjust untuk membuat GRN; user view-only
           tidak perlu melihat tombol yang pasti gagal saat submit. -->
      <AppButton v-if="canCreate" @click="openForm">+ Terima Barang</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filter gudang -->
    <AppCard :padding="false">
      <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100 flex-wrap">
        <SearchSelect v-model="filterWarehouse" :options="warehouseFilterOptions" placeholder="Semua gudang"
          searchPlaceholder="Cari gudang…" valueKey="value" labelKey="label" @change="onFilterChange" />
      </div>

      <div v-if="loading" class="p-8 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!receipts.length" class="p-8 text-center text-sm text-gray-400">Belum ada penerimaan barang.</div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="border-b border-gray-200 bg-gray-50/70 text-left text-gray-500">
              <th class="py-2.5 px-3 font-medium">No. GRN</th>
              <th class="py-2.5 px-3 font-medium">Waktu</th>
              <th class="py-2.5 px-3 font-medium">Gudang</th>
              <th class="py-2.5 px-3 font-medium">Vendor / PO</th>
              <th class="py-2.5 px-3 font-medium text-right">Item</th>
              <th class="py-2.5 px-3 font-medium text-right">Nilai</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in receipts" :key="r.id" class="border-b border-gray-50 hover:bg-emerald-50/40 cursor-pointer" @click="openDetail(r)">
              <td class="py-2.5 px-3 font-semibold text-gray-900">{{ r.grn_number }}</td>
              <td class="py-2.5 px-3 text-gray-500 text-xs">{{ fmtDateTime(r.received_at) }}</td>
              <td class="py-2.5 px-3 text-gray-700">{{ r.warehouse_name }}</td>
              <td class="py-2.5 px-3 text-gray-600">{{ r.vendor_name || '—' }}<span v-if="r.po_ref" class="text-xs text-gray-400"> · {{ r.po_ref }}</span></td>
              <td class="py-2.5 px-3 text-right text-gray-600">{{ r.item_count }}</td>
              <td class="py-2.5 px-3 text-right font-semibold text-emerald-700">{{ formatRupiah(r.total_cost) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="total > PER_PAGE" class="px-4 py-3 border-t border-gray-100">
        <AppPagination v-model="page" :total="total" :per-page="PER_PAGE" />
      </div>
    </AppCard>

    <!-- Form Terima Barang -->
    <AppModal v-model="formOpen" title="Terima Barang" size="lg">
      <div class="space-y-4">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="lbl">Gudang tujuan *</label>
            <SearchSelect v-model="form.warehouse_id" :options="warehouseOptions" placeholder="Pilih gudang" searchPlaceholder="Cari gudang…" valueKey="value" labelKey="label" />
          </div>
          <div>
            <label class="lbl">Vendor / Supplier</label>
            <input v-model="form.vendor_name" class="form-input" placeholder="mis. CV Sumber Pangan" />
          </div>
          <div>
            <label class="lbl">No. PO vendor (opsional)</label>
            <input v-model="form.po_ref" class="form-input" placeholder="mis. PO/2026/091" />
            <!-- GRN manual tidak menulis pr_item_key, jadi pengajuan pengadaan tidak
                 pernah tertutup lewat sini; server menolak nomor pengajuan yang masih hidup. -->
            <p class="mt-1 text-[11px] text-amber-700">
              Barang dari <strong>pengajuan pengadaan</strong> dicatat lewat
              <RouterLink to="/gudang/penerimaan-pengadaan" class="font-semibold underline">Penerimaan dari Pengadaan</RouterLink>,
              bukan di sini — supaya pengajuannya ikut tertutup.
            </p>
          </div>
          <div>
            <label class="lbl">Catatan</label>
            <input v-model="form.notes" class="form-input" placeholder="opsional" />
          </div>
        </div>

        <div class="rounded-xl border border-gray-200 overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600 flex items-center justify-between">
            <span>Item Diterima</span>
            <button class="text-emerald-600 font-semibold" @click="addLine">+ Tambah item</button>
          </div>
          <div v-if="!form.items.length" class="p-4 text-sm text-gray-400 text-center">Belum ada item. Klik "Tambah item".</div>
          <div v-for="(ln, i) in form.items" :key="i" class="p-3 border-t border-gray-100 grid grid-cols-12 gap-2 items-end">
            <div class="col-span-12 sm:col-span-4">
              <label class="lbl">Item stok</label>
              <SearchSelect v-model="ln.item_id" :options="itemOptions" placeholder="Pilih item" searchPlaceholder="Cari item…" valueKey="value" labelKey="label" @change="onItemPick(ln)" />
            </div>
            <div class="col-span-4 sm:col-span-2">
              <label class="lbl">Qty ({{ unitOf(ln.item_id) }})</label>
              <input v-model.number="ln.qty_dist" type="number" min="0" step="0.01" class="form-input" />
            </div>
            <div class="col-span-4 sm:col-span-3">
              <label class="lbl">Harga / {{ baseUnitOf(ln.item_id) }}</label>
              <input v-model.number="ln.cost_per_base" type="number" min="0" step="1" class="form-input" />
            </div>
            <div class="col-span-4 sm:col-span-2">
              <label class="lbl">Kadaluarsa</label>
              <input v-model="ln.expiry_date" type="date" class="form-input" />
            </div>
            <div class="col-span-12 sm:col-span-1 flex justify-between items-center">
              <span class="text-xs text-gray-400 sm:hidden">Subtotal {{ formatRupiah(lineSubtotal(ln)) }}</span>
              <button class="text-red-500 text-lg leading-none px-1" title="Hapus baris" @click="form.items.splice(i, 1)">×</button>
            </div>
            <div class="col-span-12 hidden sm:block text-right text-xs text-gray-400 -mt-1">
              = {{ baseQty(ln) }} {{ baseUnitOf(ln.item_id) }} · Subtotal {{ formatRupiah(lineSubtotal(ln)) }}
            </div>
          </div>
          <div class="px-3 py-2 border-t border-gray-100 flex justify-between text-sm font-semibold">
            <span>Total</span><span class="text-emerald-700">{{ formatRupiah(formTotal) }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="formOpen = false">Batal</AppButton>
        <AppButton :loading="saving" :disabled="!canSubmit" @click="submit">Simpan Penerimaan</AppButton>
      </template>
    </AppModal>

    <!-- Detail GRN -->
    <AppModal v-model="detailOpen" :title="`Detail — ${active?.grn_number || ''}`">
      <div v-if="active" class="space-y-3 text-sm">
        <div class="grid grid-cols-2 gap-2">
          <div class="kv"><span>Gudang</span><b>{{ active.warehouse_name }}</b></div>
          <div class="kv"><span>Waktu</span><b>{{ fmtDateTime(active.received_at) }}</b></div>
          <div class="kv"><span>Vendor</span><b>{{ active.vendor_name || '—' }}</b></div>
          <div class="kv"><span>No. PO</span><b>{{ active.po_ref || '—' }}</b></div>
          <div v-if="active.purchase_request_number" class="kv"><span>No. Pengajuan</span><b>{{ active.purchase_request_number }}</b></div>
          <div class="kv"><span>Diterima oleh</span><b>{{ active.received_by || '—' }}</b></div>
          <div class="kv"><span>Total</span><b class="text-emerald-700">{{ formatRupiah(active.total_cost) }}</b></div>
        </div>
        <div v-if="active.notes" class="text-xs text-gray-500">Catatan: {{ active.notes }}</div>
        <div class="rounded-xl border border-gray-200 overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600">Item</div>
          <ul class="divide-y divide-gray-100">
            <li v-for="it in active.items" :key="it.id" class="px-3 py-2 flex items-center justify-between gap-2">
              <div class="min-w-0">
                <p class="font-medium text-gray-800">{{ it.item_name }}</p>
                <p class="text-xs text-gray-400">{{ Number(it.qty_dist) }} {{ it.unit_used }} = {{ Number(it.qty_base) }} {{ it.base_unit }} @ {{ formatRupiah(it.cost_per_base) }}<span v-if="it.expiry_date"> · exp {{ it.expiry_date }}</span></p>
              </div>
              <span class="font-semibold text-gray-800 shrink-0">{{ formatRupiah(it.subtotal) }}</span>
            </li>
          </ul>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { goodsReceiptsApi, warehousesApi, stockItemsApi } from '@/api/warehouse.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('stockledger.adjust')
const receipts = ref([])
const warehouses = ref([])
const items = ref([])
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const filterWarehouse = ref('')
const PER_PAGE = 25
const page = ref(1)
const total = ref(0)

const formOpen = ref(false)
const detailOpen = ref(false)
const active = ref(null)
const form = ref({ warehouse_id: '', vendor_name: '', po_ref: '', notes: '', items: [] })

const warehouseOptions = computed(() => warehouses.value.map(w => ({ value: w.id, label: w.name })))
const warehouseFilterOptions = computed(() => [{ value: '', label: 'Semua gudang' }, ...warehouseOptions.value])
const itemOptions = computed(() => items.value.map(it => ({ value: it.id, label: `${it.name} (${it.base_unit})` })))

function itemById(id) { return items.value.find(x => x.id === id) }
function unitOf(id) { const it = itemById(id); return it ? (it.dist_unit || it.base_unit) : 'unit' }
function baseUnitOf(id) { const it = itemById(id); return it ? it.base_unit : 'unit' }
function ratioOf(id) { const it = itemById(id); return it && it.dist_ratio > 0 ? it.dist_ratio : 1 }
function baseQty(ln) { return (Number(ln.qty_dist) || 0) * ratioOf(ln.item_id) }
function lineSubtotal(ln) { return baseQty(ln) * (Number(ln.cost_per_base) || 0) }
const formTotal = computed(() => form.value.items.reduce((s, ln) => s + lineSubtotal(ln), 0))
const canSubmit = computed(() => form.value.warehouse_id && form.value.items.length > 0 &&
  form.value.items.every(ln => ln.item_id && Number(ln.qty_dist) > 0))

function onItemPick(ln) {
  // Prefill harga dari avg_cost item bila ada & belum diisi.
  const it = itemById(ln.item_id)
  if (it && !ln.cost_per_base && it.avg_cost > 0) ln.cost_per_base = it.avg_cost
}
function addLine() { form.value.items.push({ item_id: '', qty_dist: null, cost_per_base: null, expiry_date: '' }) }
function openForm() {
  form.value = { warehouse_id: filterWarehouse.value || '', vendor_name: '', po_ref: '', notes: '', items: [] }
  addLine()
  formOpen.value = true
}
async function openDetail(r) {
  try { const d = await goodsReceiptsApi.get(r.id); active.value = d?.data ?? d; detailOpen.value = true }
  catch (e) { toast.error(e?.message || 'Gagal memuat detail') }
}

async function submit() {
  saving.value = true
  try {
    const payload = {
      warehouse_id: form.value.warehouse_id,
      vendor_name: form.value.vendor_name,
      po_ref: form.value.po_ref,
      notes: form.value.notes,
      items: form.value.items.map(ln => ({
        item_id: ln.item_id,
        qty_dist: Number(ln.qty_dist) || 0,
        cost_per_base: Number(ln.cost_per_base) || 0,
        expiry_date: ln.expiry_date || '',
      })),
    }
    const d = await goodsReceiptsApi.create(payload)
    toast.success(`Penerimaan ${d?.grn_number || d?.data?.grn_number || ''} tersimpan`)
    formOpen.value = false
    page.value = 1
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan penerimaan') }
  finally { saving.value = false }
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const d = await goodsReceiptsApi.list({ warehouse_id: filterWarehouse.value || undefined, page: page.value, limit: PER_PAGE })
    receipts.value = d?.data ?? []
    total.value = d?.total ?? receipts.value.length
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat penerimaan' }
  finally { loading.value = false }
}
watch(page, load)

// Ganti filter selalu mulai dari halaman 1; watch(page) yang memanggil load.
function onFilterChange() {
  if (page.value === 1) load()
  else page.value = 1
}

function fmtDateTime(s) {
  if (!s) return '—'
  const d = new Date(s)
  return isNaN(d) ? '—' : d.toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

onMounted(async () => {
  try {
    const [w, it] = await Promise.all([
      warehousesApi.list({ limit: 200 }),
      stockItemsApi.list({ limit: 500, active_only: true }),
    ])
    warehouses.value = w?.data ?? []
    items.value = it?.data ?? []
  } catch { /* ignore */ }
  await load()
})
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .7rem; font-weight: 600; color: #6b7280; margin-bottom: .2rem; }
.kv { display: flex; flex-direction: column; }
.kv span { font-size: .66rem; color: #9ca3af; text-transform: uppercase; letter-spacing: .03em; }
.kv b { color: #111827; }
</style>
