<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Bundling Lintas Outlet</h1>
        <p class="text-sm text-gray-500 mt-0.5">
          Klaim kupon produk antar outlet, pemotongan stok gudang mitra secara real-time, dan rekonsiliasi alokasi omzet tanpa PB1 ganda.
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <router-link v-if="auth.hasPermission('products.view')" to="/products"
          class="px-3 py-2 rounded-lg text-sm font-semibold border border-gray-200 bg-white text-gray-700 hover:bg-gray-50">
          Atur di Produk
        </router-link>
        <AppButton v-if="canManage && components.length" @click="openManual">+ Buat Kupon Manual</AppButton>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- KPI Cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div class="rounded-xl border border-amber-200 bg-amber-50/60 p-3.5">
        <p class="text-xs font-medium text-amber-800">Menunggu Klaim</p>
        <p class="text-xl font-bold text-amber-900 mt-1">{{ summary.pending_count }} <span class="text-xs font-normal">kupon</span></p>
        <p class="text-xs text-amber-700 mt-0.5">Alokasi {{ formatRupiah(summary.pending_allocation) }}</p>
      </div>
      <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3.5">
        <p class="text-xs font-medium text-emerald-800">Sudah Diklaim</p>
        <p class="text-xl font-bold text-emerald-900 mt-1">{{ summary.claimed_count }} <span class="text-xs font-normal">kupon</span></p>
        <p class="text-xs text-emerald-700 mt-0.5">Alokasi diakui {{ formatRupiah(summary.claimed_allocation) }}</p>
      </div>
      <div class="rounded-xl border border-gray-200 bg-white p-3.5">
        <p class="text-xs font-medium text-gray-500">Kedaluwarsa / Void</p>
        <p class="text-xl font-bold text-gray-900 mt-1">{{ summary.expired_count + summary.voided_count }} <span class="text-xs font-normal text-gray-500">kupon</span></p>
        <p class="text-xs text-gray-500 mt-0.5">{{ summary.expired_count }} hangus · {{ summary.voided_count }} void</p>
      </div>
      <div class="rounded-xl border border-indigo-200 bg-indigo-50/50 p-3.5">
        <p class="text-xs font-medium text-indigo-800">Aturan Bundling Aktif</p>
        <p class="text-xl font-bold text-indigo-900 mt-1">{{ components.length }} <span class="text-xs font-normal">komponen</span></p>
        <p class="text-xs text-indigo-700 mt-0.5">Otomatis dari transaksi POS</p>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex border-b border-gray-200 gap-2 overflow-x-auto">
      <button type="button" @click="activeTab = 'claims'"
        :class="['px-4 py-2.5 text-sm font-semibold border-b-2 whitespace-nowrap transition-colors',
          activeTab === 'claims' ? 'border-emerald-600 text-emerald-700' : 'border-transparent text-gray-500 hover:text-gray-700']">
        Antrean &amp; Riwayat Klaim
        <span v-if="summary.pending_count" class="ml-1.5 rounded-full bg-amber-100 text-amber-800 px-2 py-0.5 text-xs">{{ summary.pending_count }}</span>
      </button>
      <button type="button" @click="activeTab = 'reconciliation'"
        :class="['px-4 py-2.5 text-sm font-semibold border-b-2 whitespace-nowrap transition-colors',
          activeTab === 'reconciliation' ? 'border-emerald-600 text-emerald-700' : 'border-transparent text-gray-500 hover:text-gray-700']">
        Aturan &amp; Rekonsiliasi Antar Outlet
      </button>
    </div>

    <!-- TAB 1: ANTREAN & RIWAYAT KLAIM -->
    <template v-if="activeTab === 'claims'">
      <AppCard>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-3">
          <SearchableSelect v-model="filterDirection" class="form-input" @change="load">
            <option value="">Semua arah</option>
            <option value="inbound">Masuk (Outlet penyedia)</option>
            <option value="outbound">Keluar (Outlet penjual)</option>
          </SearchableSelect>
          <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
          <SearchableSelect v-model="filterStatus" class="form-input" @change="load">
            <option value="">Semua status</option>
            <option value="pending">Menunggu Klaim</option>
            <option value="claimed">Sudah Diklaim</option>
            <option value="expired">Kedaluwarsa</option>
            <option value="voided">Dibatalkan (Void)</option>
          </SearchableSelect>
          <div class="bundle-drp w-full"><DateRangePicker v-model="range" clearable /></div>
          <input v-model="searchQuery" type="search" class="form-input" placeholder="Cari kode / tamu / menu…" @input="debouncedLoad" />
        </div>
      </AppCard>

      <AppCard :padding="false">
        <!-- Mobile & tablet cards -->
        <div class="lg:hidden">
          <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
          <div v-else-if="!claims.length" class="p-6 text-center text-sm text-gray-400">Belum ada kupon bundling pada filter ini.</div>
          <ul v-else class="divide-y divide-gray-100">
            <li v-for="c in claims" :key="c.id" class="p-4 space-y-2.5">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-mono text-sm font-bold text-emerald-700">{{ c.claim_code }}</p>
                  <p class="text-xs text-gray-500">{{ c.sale_at }}<span v-if="c.customer_name"> · {{ c.customer_name }}</span></p>
                </div>
                <span class="st-badge shrink-0" :class="statusCls(c.status)">{{ STATUS_LABELS[c.status] || c.status }}</span>
              </div>
              <div class="rounded-lg bg-gray-50 p-2.5 text-xs space-y-1">
                <p><span class="text-gray-500">Dijual di <b>{{ c.source_outlet_name }}</b>:</span> {{ fmtQty(c.source_qty) }}× {{ c.source_product_name }}</p>
                <p><span class="text-gray-500">Diambil di <b>{{ c.partner_outlet_name }}</b>:</span> <b class="text-gray-900">{{ fmtQty(c.partner_qty) }}× {{ c.partner_product_name }}</b></p>
                <p class="flex flex-wrap items-center justify-between gap-2 pt-1 border-t border-gray-200/70">
                  <span class="text-gray-500">Alokasi internal (ex-tax)</span>
                  <b class="text-indigo-700">{{ formatRupiah(c.allocation_amount) }}</b>
                </p>
              </div>
              <p v-if="c.status === 'pending' && c.expires_at" class="text-[11px] text-amber-700">Berlaku s/d {{ c.expires_at }}</p>
              <p v-if="c.status === 'claimed'" class="text-[11px] text-emerald-700">
                Diserahkan oleh <b>{{ c.claimed_by || '—' }}</b> ({{ c.claimed_at }})
                <span class="block text-gray-500">{{ c.stock_deduction_note || (c.stock_deducted ? 'Stok resep terpotong otomatis' : '') }}</span>
              </p>
              <p v-if="c.void_reason" class="text-[11px] text-red-600">{{ c.void_reason }}</p>
              <div v-if="c.status === 'pending'" class="flex flex-wrap gap-2 pt-1">
                <AppButton v-if="c.can_claim && canClaim" size="sm" @click="openClaimModal(c)">Serahkan / Klaim</AppButton>
                <button v-if="canManage || (c.can_claim && canClaim)" type="button"
                  class="px-3 py-1.5 rounded-lg text-xs font-semibold border border-gray-200 text-gray-600 hover:bg-gray-50"
                  @click="expireClaim(c)">Tandai Kedaluwarsa</button>
              </div>
            </li>
          </ul>
        </div>

        <!-- Desktop table -->
        <AppTable class="hidden lg:block" :columns="CLAIM_COLUMNS" :rows="claims" :loading="loading" emptyText="Belum ada kupon bundling pada filter ini.">
          <template #cell-claim_code="{ row }">
            <div>
              <span class="font-mono text-sm font-bold text-emerald-700 whitespace-nowrap">{{ row.claim_code }}</span>
              <span class="block text-xs text-gray-400 whitespace-nowrap">{{ row.sale_at }}</span>
              <span v-if="row.customer_name" class="block text-xs text-gray-600">{{ row.customer_name }}</span>
            </div>
          </template>
          <template #cell-source="{ row }">
            <div class="text-sm">
              <span class="font-semibold text-gray-900">{{ row.source_outlet_name }}</span>
              <span class="block text-xs text-gray-500">{{ fmtQty(row.source_qty) }}× {{ row.source_product_name }}</span>
            </div>
          </template>
          <template #cell-partner="{ row }">
            <div class="text-sm">
              <span class="font-semibold text-indigo-800">{{ row.partner_outlet_name }}</span>
              <span class="block text-xs text-gray-700 font-medium">{{ fmtQty(row.partner_qty) }}× {{ row.partner_product_name }}</span>
            </div>
          </template>
          <template #cell-allocation="{ row }">
            <span class="font-semibold text-gray-900 whitespace-nowrap">{{ formatRupiah(row.allocation_amount) }}</span>
          </template>
          <template #cell-status="{ row }">
            <div class="space-y-0.5">
              <span class="st-badge" :class="statusCls(row.status)">{{ STATUS_LABELS[row.status] || row.status }}</span>
              <span v-if="row.status === 'pending' && row.expires_at" class="block text-[11px] text-amber-700 whitespace-nowrap">s/d {{ row.expires_at }}</span>
              <span v-else-if="row.status === 'claimed'" class="block text-[11px] text-gray-500">
                {{ row.claimed_by }} · {{ row.claimed_at }}
                <span class="block" :class="row.stock_deducted ? 'text-emerald-700' : 'text-amber-700'">
                  {{ row.stock_deducted ? 'Stok terpotong ✓' : 'Tanpa resep stok' }}
                </span>
              </span>
              <span v-else-if="row.void_reason" class="block text-[11px] text-red-600">{{ row.void_reason }}</span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1.5 whitespace-nowrap">
              <AppButton v-if="row.status === 'pending' && row.can_claim && canClaim" size="sm" @click="openClaimModal(row)">Serahkan</AppButton>
              <button v-if="row.status === 'pending' && (canManage || (row.can_claim && canClaim))" type="button"
                class="px-2.5 py-1.5 rounded-lg text-xs font-medium border border-gray-200 text-gray-600 hover:bg-gray-50"
                @click="expireClaim(row)">Hangus</button>
            </div>
          </template>
        </AppTable>
      </AppCard>
    </template>

    <!-- TAB 2: ATURAN & REKONSILIASI ANTAR OUTLET -->
    <template v-else>
      <AppCard :padding="false">
        <div class="px-4 py-3 border-b border-gray-100">
          <h2 class="text-sm font-bold text-gray-900">Rekonsiliasi Bundling Antar Outlet</h2>
          <p class="text-xs text-gray-500 mt-0.5">
            Membandingkan paket terjual di outlet asal vs item yang benar-benar diserahkan di outlet mitra beserta alokasi omzet internalnya.
          </p>
        </div>
        <div v-if="!reconciliation.length" class="p-6 text-center text-sm text-gray-400">Belum ada data klaim pada rentang filter saat ini.</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full text-sm divide-y divide-gray-200">
            <thead class="bg-gray-50 text-xs text-gray-500 uppercase">
              <tr>
                <th class="px-4 py-3 text-left">Outlet Asal → Mitra</th>
                <th class="px-4 py-3 text-left">Produk Paket → Item Mitra</th>
                <th class="px-4 py-3 text-right">Paket Terjual</th>
                <th class="px-4 py-3 text-right">Diklaim</th>
                <th class="px-4 py-3 text-right">Menunggu</th>
                <th class="px-4 py-3 text-right">Hangus/Void</th>
                <th class="px-4 py-3 text-right">Alokasi Diakui Mitra</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white">
              <tr v-for="(r, idx) in reconciliation" :key="idx" class="hover:bg-gray-50">
                <td class="px-4 py-3 whitespace-nowrap font-medium text-gray-900">
                  {{ r.source_outlet_name }} <span class="text-gray-400 mx-1">→</span> <span class="text-indigo-700">{{ r.partner_outlet_name }}</span>
                </td>
                <td class="px-4 py-3 text-xs text-gray-700">
                  <div>{{ r.source_product_name }}</div>
                  <div class="text-gray-500">→ {{ r.partner_product_name }}</div>
                </td>
                <td class="px-4 py-3 text-right whitespace-nowrap">{{ fmtQty(r.total_bundles_sold) }} pkt ({{ fmtQty(r.total_partner_qty) }} item)</td>
                <td class="px-4 py-3 text-right whitespace-nowrap font-semibold text-emerald-700">{{ fmtQty(r.claimed_partner_qty) }}</td>
                <td class="px-4 py-3 text-right whitespace-nowrap text-amber-700">{{ fmtQty(r.pending_partner_qty) }}</td>
                <td class="px-4 py-3 text-right whitespace-nowrap text-gray-500">{{ fmtQty(r.expired_partner_qty + r.voided_partner_qty) }}</td>
                <td class="px-4 py-3 text-right whitespace-nowrap font-bold text-indigo-700">{{ formatRupiah(r.claimed_allocation) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <AppCard :padding="false">
        <div class="px-4 py-3 border-b border-gray-100 flex items-center justify-between gap-2 flex-wrap">
          <div>
            <h2 class="text-sm font-bold text-gray-900">Aturan Bundling Lintas Outlet Aktif</h2>
            <p class="text-xs text-gray-500 mt-0.5">Tambahkan aturan baru lewat tombol <b>Bundling</b> pada halaman <b>Produk</b>.</p>
          </div>
        </div>
        <div v-if="!components.length" class="p-6 text-center text-sm text-gray-400">Belum ada aturan bundling lintas outlet yang dikonfigurasi.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="c in components" :key="c.id" class="px-4 py-3 flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0 space-y-0.5">
              <p class="text-sm font-semibold text-gray-900">
                {{ c.source_outlet_name }}: {{ c.source_product_name }}
                <span class="text-gray-400 font-normal">({{ formatRupiah(c.source_product_price) }})</span>
              </p>
              <p class="text-xs text-gray-600">
                → Diambil di <b class="text-indigo-700">{{ c.partner_outlet_name }}</b>:
                <b>{{ fmtQty(c.qty_per_bundle) }}× {{ c.partner_product_name }}</b>
                · Alokasi internal <b>{{ formatRupiah(c.allocation_amount) }}</b>/paket
                · {{ c.valid_days > 0 ? `Berlaku ${c.valid_days} hari` : 'Tanpa batas waktu' }}
              </p>
              <p class="text-[11px]" :class="c.partner_has_recipe ? 'text-emerald-700' : 'text-amber-700'">
                {{ c.partner_has_recipe ? '✓ Produk mitra sudah memiliki resep (stok terpotong otomatis saat klaim)' : '⚠ Produk mitra belum memiliki resep (pasang resep di Produk agar stok terpotong otomatis)' }}
              </p>
            </div>
            <AppButton v-if="canManage" size="sm" variant="secondary" @click="openEditRule(c)">Atur</AppButton>
          </li>
        </ul>
      </AppCard>
    </template>

    <!-- Modal Konfirmasi Klaim / Penyerahan -->
    <AppModal v-model="showClaimModal" title="Konfirmasi Penyerahan Bundling" size="md">
      <div v-if="claimTarget" class="space-y-3 text-sm">
        <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3.5 space-y-1.5">
          <p class="font-mono text-base font-bold text-emerald-800">{{ claimTarget.claim_code }}</p>
          <p class="text-xs text-gray-600">Pembelian di <b>{{ claimTarget.source_outlet_name }}</b>: {{ fmtQty(claimTarget.source_qty) }}× {{ claimTarget.source_product_name }}<span v-if="claimTarget.customer_name"> ({{ claimTarget.customer_name }})</span></p>
          <p class="text-sm font-bold text-gray-900 pt-1">
            Serahkan: {{ fmtQty(claimTarget.partner_qty) }}× {{ claimTarget.partner_product_name }}
          </p>
          <p class="text-xs text-indigo-700">Alokasi omzet internal ke {{ claimTarget.partner_outlet_name }}: <b>{{ formatRupiah(claimTarget.allocation_amount) }}</b> (ex-tax)</p>
        </div>
        <p class="text-xs text-gray-500">
          Setelah dikonfirmasi, stok resep di gudang <b>{{ claimTarget.partner_outlet_name }}</b> otomatis terpotong (bila resep terpasang) dan kupon tidak bisa diklaim ulang.
        </p>
        <div>
          <label class="lbl">Catatan Penyerahan (opsional)</label>
          <input v-model="claimNotes" class="form-input" placeholder="Mis. Diserahkan langsung ke tamu di kasir" />
        </div>
      </div>
      <template #footer>
        <div class="flex w-full flex-col-reverse sm:flex-row sm:justify-end gap-2">
          <button type="button" class="btn-ghost w-full sm:w-auto" @click="showClaimModal = false">Batal</button>
          <AppButton :loading="acting" class="w-full sm:w-auto" @click="submitClaim">Konfirmasi Sudah Diserahkan</AppButton>
        </div>
      </template>
    </AppModal>

    <!-- Modal Buat Kupon Manual / Uji -->
    <AppModal v-model="showManualModal" title="Buat Kupon Bundling Manual" size="md">
      <div class="space-y-3">
        <p class="text-xs text-gray-500">
          Normalnya kupon bundling lahir otomatis saat transaksi POS disinkronkan dari outlet penjual. Form ini dipakai untuk simulasi pengujian di Demo atau penerbitan manual bila diperlukan.
        </p>
        <div>
          <label class="lbl">Aturan Bundling <span class="text-red-500">*</span></label>
          <SearchSelect v-model="manualForm.component_id" :options="componentOptions" placeholder="Pilih aturan bundling…" searchPlaceholder="Cari aturan…" />
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="lbl">Jumlah Paket Terjual</label>
            <input v-model.number="manualForm.source_qty" type="number" min="1" step="1" inputmode="numeric" class="form-input" />
          </div>
          <div>
            <label class="lbl">Nama Tamu / Referensi</label>
            <input v-model="manualForm.customer_name" class="form-input" placeholder="Mis. Tamu Budi" />
          </div>
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <input v-model="manualForm.notes" class="form-input" placeholder="Opsional" />
        </div>
      </div>
      <template #footer>
        <div class="flex w-full flex-col-reverse sm:flex-row sm:justify-end gap-2">
          <button type="button" class="btn-ghost w-full sm:w-auto" @click="showManualModal = false">Batal</button>
          <AppButton :loading="acting" class="w-full sm:w-auto" @click="submitManual">Terbitkan Kupon</AppButton>
        </div>
      </template>
    </AppModal>

    <ProductBundleModal v-model="showRuleModal" :product="ruleProduct" @saved="load" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { bundlesApi } from '@/api/bundles.js'
import { outletsApi } from '@/api/outlets.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import ProductBundleModal from '@/components/ProductBundleModal.vue'

const auth = useAuthStore()
const toast = useToastStore()

const canClaim = computed(() => auth.hasPermission('bundles.claim'))
const canManage = computed(() => auth.scopeType !== 'specific' && auth.hasPermission('bundles.manage'))

const activeTab = ref('claims')
const loading = ref(false)
const acting = ref(false)
const errorMsg = ref('')

const outlets = ref([])
const components = ref([])
const claims = ref([])
const reconciliation = ref([])
const summary = ref({
  total_claims: 0, pending_count: 0, pending_allocation: 0,
  claimed_count: 0, claimed_allocation: 0, expired_count: 0, voided_count: 0,
})

const filterDirection = ref('')
const filterOutlet = ref('')
const filterStatus = ref('')
const searchQuery = ref('')
const range = ref({ from: '', to: '', label: 'Semua Tanggal' })
watch(range, () => load())

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const componentOptions = computed(() => components.value.map(c => ({
  id: c.id,
  name: `${c.source_outlet_name}: ${c.source_product_name} → ${c.partner_outlet_name}: ${fmtQty(c.qty_per_bundle)}× ${c.partner_product_name}`,
})))

const STATUS_LABELS = {
  pending: 'Menunggu Klaim',
  claimed: 'Sudah Diklaim',
  expired: 'Kedaluwarsa',
  voided:  'Dibatalkan (Void)',
}
function statusCls(s) {
  return {
    'st-pending': s === 'pending',
    'st-claimed': s === 'claimed',
    'st-expired': s === 'expired',
    'st-voided':  s === 'voided',
  }
}
function fmtQty(v) {
  return new Intl.NumberFormat('id-ID', { maximumFractionDigits: 3 }).format(Number(v || 0))
}
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d }

const CLAIM_COLUMNS = [
  { key: 'claim_code', label: 'Kupon & Waktu' },
  { key: 'source',     label: 'Dijual di' },
  { key: 'partner',    label: 'Diambil di Mitra' },
  { key: 'allocation', label: 'Alokasi (ex-tax)' },
  { key: 'status',     label: 'Status & Stok' },
  { key: 'actions',    label: '' },
]

let timer
function debouncedLoad() {
  clearTimeout(timer)
  timer = setTimeout(() => load(), 250)
}

async function load({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const [claimsRes, compRes] = await Promise.all([
      bundlesApi.listClaims({
        direction: filterDirection.value || undefined,
        outlet_id: filterOutlet.value || undefined,
        status: filterStatus.value || undefined,
        date_from: range.value?.from || undefined,
        date_to: range.value?.to || undefined,
        search: searchQuery.value.trim() || undefined,
      }),
      bundlesApi.listComponents({ outlet_id: filterOutlet.value || undefined }),
    ])
    const data = asObject(claimsRes) || {}
    claims.value = data.claims || []
    summary.value = data.summary || summary.value
    reconciliation.value = data.reconciliation || []
    components.value = asArray(compRes)
  } catch (e) {
    if (!silent) errorMsg.value = e?.message || 'Gagal memuat data bundling lintas outlet'
  } finally {
    if (!silent) loading.value = false
  }
}

async function loadOutlets() {
  try { const d = await outletsApi.myOutlets(); outlets.value = d?.outlets ?? d ?? [] } catch { outlets.value = [] }
}

// Modal claim
const showClaimModal = ref(false)
const claimTarget = ref(null)
const claimNotes = ref('')
function openClaimModal(c) {
  claimTarget.value = c
  claimNotes.value = ''
  showClaimModal.value = true
}
async function submitClaim() {
  if (!claimTarget.value) return
  acting.value = true
  try {
    const res = asObject(await bundlesApi.claim(claimTarget.value.id, { notes: claimNotes.value }))
    showClaimModal.value = false
    toast.success(res?.stock_deducted ? 'Barang diserahkan & stok mitra terpotong otomatis' : 'Penyerahan bundling berhasil dicatat')
    await load()
  } catch (e) {
    toast.error(e?.message || 'Gagal mencatat penyerahan')
  } finally {
    acting.value = false
  }
}

async function expireClaim(c) {
  if (!window.confirm(`Tandai kupon ${c.claim_code} sebagai kedaluwarsa (hangus)?`)) return
  acting.value = true
  try {
    await bundlesApi.expire(c.id, 'Ditandai kedaluwarsa oleh petugas')
    toast.success('Kupon ditandai kedaluwarsa')
    await load()
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui status')
  } finally {
    acting.value = false
  }
}

// Modal manual claim
const showManualModal = ref(false)
const manualForm = ref({ component_id: '', source_qty: 1, customer_name: '', notes: '' })
function openManual() {
  manualForm.value = { component_id: components.value[0]?.id || '', source_qty: 1, customer_name: '', notes: '' }
  showManualModal.value = true
}
async function submitManual() {
  if (!manualForm.value.component_id) { toast.error('Pilih aturan bundling terlebih dahulu'); return }
  acting.value = true
  try {
    await bundlesApi.createManualClaim(manualForm.value)
    showManualModal.value = false
    toast.success('Kupon bundling berhasil diterbitkan')
    await load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menerbitkan kupon')
  } finally {
    acting.value = false
  }
}

// Edit bundling rule modal
const showRuleModal = ref(false)
const ruleProduct = ref(null)
function openEditRule(c) {
  ruleProduct.value = {
    id: c.source_product_id,
    outlet_id: c.source_outlet_id,
    outlet_name: c.source_outlet_name,
    name: c.source_product_name,
    price: c.source_product_price,
  }
  showRuleModal.value = true
}

onMounted(async () => {
  await loadOutlets()
  await load()
})
useRealtime(['bundle_claim', 'transaction'], () => load({ silent: true }))
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; }
.btn-ghost:hover { background: #e5e7eb; }
.bundle-drp :deep(.drp), .bundle-drp :deep(.drp-trigger) { width: 100%; justify-content: space-between; }
.st-badge { display: inline-block; padding: .15rem .55rem; border-radius: 999px; font-size: .7rem; font-weight: 700; white-space: nowrap; }
.st-pending { background: rgba(245,158,11,.15); color: #b45309; }
.st-claimed { background: rgba(16,185,129,.14); color: #047857; }
.st-expired { background: #f3f4f6; color: #4b5563; }
.st-voided  { background: rgba(239,68,68,.13); color: #b91c1c; }
@media (max-width: 640px) { .form-input { font-size: 16px; } }
</style>
