<template>
  <AppModal :model-value="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    :title="`Bundling Lintas Outlet · ${product?.name || ''}`" size="xl">
    <div v-if="product" class="space-y-4">
      <!-- Ringkasan alokasi -->
      <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3.5 space-y-2">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="min-w-0">
            <p class="text-sm font-bold text-gray-900">{{ product.name }}</p>
            <p class="text-xs text-gray-600">Outlet penjual: <b>{{ product.outlet_name || '—' }}</b></p>
          </div>
          <span class="rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-emerald-800 border border-emerald-200">
            Harga Jual: {{ formatRupiah(product.price) }}
          </span>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 pt-1 text-xs">
          <div class="rounded-lg bg-white/90 border border-emerald-100 px-3 py-2">
            <span class="block text-[11px] text-gray-500">Harga Produk (ex-tax)</span>
            <b class="text-sm text-gray-900">{{ formatRupiah(product.price) }}</b>
          </div>
          <div class="rounded-lg bg-white/90 border border-emerald-100 px-3 py-2">
            <span class="block text-[11px] text-gray-500">Total Alokasi ke Mitra</span>
            <b class="text-sm text-indigo-700">− {{ formatRupiah(totalAllocation) }}</b>
          </div>
          <div class="rounded-lg bg-white/90 border border-emerald-100 px-3 py-2">
            <span class="block text-[11px] text-gray-500">Net Omzet {{ product.outlet_name || 'Outlet Asal' }}</span>
            <b class="text-sm" :class="netSourceRevenue < 0 ? 'text-red-600' : 'text-emerald-700'">{{ formatRupiah(netSourceRevenue) }}</b>
          </div>
        </div>
        <p class="text-[11px] leading-relaxed text-gray-600">
          Saat produk ini terjual di kasir <b>{{ product.outlet_name }}</b>, sistem otomatis menerbitkan <b>Kupon Klaim Bundling</b> ke outlet mitra.
          Ketika mitra menyerahkan barang, stok resep gudang mitra langsung terpotong dan porsi alokasi omzet berpindah ke Laba Rugi mitra tanpa menggandakan omzet konsolidasi ataupun PB1.
        </p>
      </div>

      <AppAlert v-if="errorMsg" type="error" :message="errorMsg" />
      <p v-if="!canEdit" class="rounded-lg bg-amber-50 border border-amber-200 px-3 py-2 text-xs text-amber-800">
        Pengaturan komponen &amp; nilai alokasi lintas outlet hanya dapat diubah oleh role dengan cakupan semua outlet.
      </p>

      <!-- Daftar komponen -->
      <div class="flex items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-gray-800">Komponen dari Outlet Mitra</h3>
        <button v-if="canEdit" type="button" @click="addRow"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200 hover:bg-emerald-100">
          + Tambah Komponen Mitra
        </button>
      </div>

      <div v-if="loading" class="py-8 text-center text-sm text-gray-400">Memuat pengaturan bundling…</div>
      <div v-else-if="!rows.length" class="rounded-xl border border-dashed border-gray-200 bg-gray-50 p-5 text-center text-xs text-gray-500">
        Belum ada komponen lintas outlet untuk produk ini.
        <template v-if="canEdit"> Klik <b>+ Tambah Komponen Mitra</b> untuk menautkan produk dari outlet lain (mis. Donat dari Gadis Desa).</template>
      </div>
      <div v-else class="space-y-3">
        <div v-for="(row, idx) in rows" :key="idx" class="rounded-xl border border-gray-200 bg-white p-3.5 space-y-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-bold text-gray-700">Komponen #{{ idx + 1 }}</span>
            <AppActionButton v-if="canEdit" icon="delete" label="Hapus komponen" type="button" @click="rows.splice(idx, 1)" />
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label class="lbl">Outlet Mitra Penyedia <span class="text-red-500">*</span></label>
              <SearchSelect
                v-model="row.partner_outlet_id"
                :options="partnerOutletOptions"
                placeholder="Pilih outlet mitra…"
                searchPlaceholder="Cari outlet…"
                :disabled="!canEdit"
                @change="onPartnerOutletChange(row)"
              />
            </div>
            <div>
              <label class="lbl">Produk Mitra yang Dikeluarkan <span class="text-red-500">*</span></label>
              <SearchSelect
                v-model="row.partner_product_id"
                :options="partnerProductOptions(row.partner_outlet_id)"
                :placeholder="loadingProducts[row.partner_outlet_id] ? 'Memuat produk…' : (row.partner_outlet_id ? 'Pilih produk mitra…' : 'Pilih outlet mitra dulu')"
                searchPlaceholder="Cari produk mitra…"
                :disabled="!canEdit || !row.partner_outlet_id || !!loadingProducts[row.partner_outlet_id]"
              />
              <p v-if="selectedPartnerProduct(row)" class="mt-1 text-[11px]"
                :class="selectedPartnerProduct(row).has_recipe ? 'text-emerald-700' : 'text-amber-700'">
                {{ selectedPartnerProduct(row).has_recipe
                  ? '✓ Produk mitra sudah memiliki resep/tautan stok — stok otomatis terpotong saat diklaim.'
                  : '⚠ Produk mitra belum memiliki resep — klaim tetap tercatat, pasang resep agar stok ikut terpotong otomatis.' }}
              </p>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label class="lbl">Qty per 1 Paket Terjual <span class="text-red-500">*</span></label>
              <input v-model.number="row.qty_per_bundle" type="number" min="0.01" step="any" inputmode="decimal"
                class="form-input" :disabled="!canEdit" />
            </div>
            <div>
              <label class="lbl">Alokasi Omzet (ex-tax / paket)</label>
              <RupiahInput v-model="row.allocation_amount" min="0" step="1" :disabled="!canEdit" />
              <p class="mt-1 text-[11px] text-gray-500">Contoh: Rp 63.636 (dari Rp 70.000 inc. PB1 10%).</p>
            </div>
            <div>
              <label class="lbl">Masa Berlaku Kupon (hari)</label>
              <input v-model.number="row.valid_days" type="number" min="0" max="365" inputmode="numeric"
                class="form-input" :disabled="!canEdit" />
              <p class="mt-1 text-[11px] text-gray-500">1 = hari yang sama, 0 = tanpa batas waktu.</p>
            </div>
          </div>

          <div>
            <label class="lbl">Catatan Operasional</label>
            <input v-model="row.notes" class="form-input" :disabled="!canEdit" placeholder="Contoh: Tunjukkan kupon di kasir Gadis Desa untuk ambil 2 donat" />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse sm:flex-row sm:items-center gap-2">
        <button type="button" class="btn-ghost w-full sm:w-auto sm:mr-auto" @click="$emit('update:modelValue', false)">Tutup</button>
        <AppButton v-if="canEdit" :loading="saving" class="w-full sm:w-auto" @click="save">Simpan Aturan Bundling</AppButton>
      </div>
    </template>
  </AppModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { bundlesApi } from '@/api/bundles.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  product:    { type: Object,  default: null },
})
const emit = defineEmits(['update:modelValue', 'saved'])

const auth = useAuthStore()
const toast = useToastStore()

const canEdit = computed(() =>
  auth.scopeType !== 'specific' && (auth.hasPermission('products.update') || auth.hasPermission('bundles.manage'))
)

const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const outlets = ref([])
const productsByOutlet = ref({})
const loadingProducts = ref({})
const rows = ref([])

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

const partnerOutletOptions = computed(() =>
  outlets.value
    .filter(o => o.id !== props.product?.outlet_id)
    .map(o => ({ id: o.id, name: o.name }))
)

function partnerProductOptions(outletId) {
  const list = productsByOutlet.value[outletId] || []
  return list.map(p => ({
    id: p.id,
    name: `${p.name} — ${formatRupiah(p.price)}${p.has_recipe ? ' [Resep ✓]' : ''}`,
  }))
}

function selectedPartnerProduct(row) {
  const list = productsByOutlet.value[row.partner_outlet_id] || []
  return list.find(p => p.id === row.partner_product_id) || null
}

const totalAllocation = computed(() =>
  rows.value.reduce((s, r) => s + (Number(r.allocation_amount) || 0), 0)
)
const netSourceRevenue = computed(() =>
  (Number(props.product?.price) || 0) - totalAllocation.value
)

async function ensurePartnerProducts(outletId) {
  if (!outletId || productsByOutlet.value[outletId]) return
  loadingProducts.value = { ...loadingProducts.value, [outletId]: true }
  try {
    const res = await bundlesApi.partnerProducts(outletId)
    productsByOutlet.value = { ...productsByOutlet.value, [outletId]: asArray(res) }
  } catch {
    productsByOutlet.value = { ...productsByOutlet.value, [outletId]: [] }
  } finally {
    loadingProducts.value = { ...loadingProducts.value, [outletId]: false }
  }
}

async function onPartnerOutletChange(row) {
  row.partner_product_id = ''
  await ensurePartnerProducts(row.partner_outlet_id)
}

function addRow() {
  rows.value.push({
    partner_outlet_id: '',
    partner_product_id: '',
    qty_per_bundle: 1,
    allocation_amount: 0,
    valid_days: 1,
    notes: '',
  })
}

async function load() {
  if (!props.product?.id) return
  loading.value = true
  errorMsg.value = ''
  try {
    const [oRes, cRes] = await Promise.all([
      outlets.value.length ? Promise.resolve(outlets.value) : bundlesApi.partnerOutlets(),
      bundlesApi.getProductComponents(props.product.id),
    ])
    outlets.value = asArray(oRes)
    const comps = asArray(cRes)
    rows.value = comps.map(c => ({
      partner_outlet_id: c.partner_outlet_id,
      partner_product_id: c.partner_product_id,
      qty_per_bundle: Number(c.qty_per_bundle) || 1,
      allocation_amount: Number(c.allocation_amount) || 0,
      valid_days: Number.isFinite(Number(c.valid_days)) ? Number(c.valid_days) : 1,
      notes: c.notes || '',
    }))
    const uniqueOutlets = [...new Set(rows.value.map(r => r.partner_outlet_id).filter(Boolean))]
    await Promise.all(uniqueOutlets.map(ensurePartnerProducts))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat komponen bundling'
  } finally {
    loading.value = false
  }
}

watch(() => [props.modelValue, props.product?.id], ([open]) => {
  if (open) load()
})

async function save() {
  errorMsg.value = ''
  for (let i = 0; i < rows.value.length; i++) {
    const r = rows.value[i]
    if (!r.partner_outlet_id || !r.partner_product_id) {
      errorMsg.value = `Komponen #${i + 1}: outlet mitra dan produk mitra wajib dipilih.`
      return
    }
    if (!(Number(r.qty_per_bundle) > 0)) {
      errorMsg.value = `Komponen #${i + 1}: qty per paket harus lebih dari 0.`
      return
    }
  }
  if (netSourceRevenue.value < -0.009) {
    errorMsg.value = 'Total alokasi omzet ke mitra tidak boleh melebihi harga jual produk.'
    return
  }
  saving.value = true
  try {
    await bundlesApi.saveProductComponents(props.product.id, rows.value.map(r => ({
      partner_outlet_id: r.partner_outlet_id,
      partner_product_id: r.partner_product_id,
      qty_per_bundle: Number(r.qty_per_bundle),
      allocation_amount: Number(r.allocation_amount) || 0,
      valid_days: Number(r.valid_days) || 0,
      notes: r.notes || '',
    })))
    toast.success('Aturan bundling lintas outlet disimpan')
    emit('saved')
    emit('update:modelValue', false)
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal menyimpan aturan bundling'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f9fafb; color: #6b7280; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
@media (max-width: 640px) { .form-input { font-size: 16px; } }
</style>
