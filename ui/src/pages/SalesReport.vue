<template>
  <div class="space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Tanggal</label>
          <DateRangePicker v-model="range" />
        </div>
        <div class="flex flex-col gap-1 min-w-45">
          <label class="text-sm font-medium text-gray-700">Outlet</label>
          <SearchSelect
            v-model="selectedOutlet"
            :options="outletOptions"
            placeholder="Semua Outlet"
            searchPlaceholder="Cari outlet..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <button @click="applyFilter"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
        <button @click="downloadExcel" :disabled="exporting"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <!-- Summary Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <SummaryCard label="Total Transaksi" :value="report.summary.total_transactions" icon="receipt" />
        <SummaryCard label="Total Pendapatan" :value="formatRupiah(report.summary.total_revenue)" icon="revenue" />
        <SummaryCard label="Rata-rata / Transaksi" :value="formatRupiah(report.summary.avg_per_transaction)" icon="avg" />
        <SummaryCard label="Metode Pembayaran" icon="payment">
          <div class="text-xs space-y-1 mt-1">
            <div class="flex justify-between"><span class="text-gray-500">Cash</span><span class="font-medium">{{ formatRupiah(report.summary.cash_revenue) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">QRIS</span><span class="font-medium">{{ formatRupiah(report.summary.qris_revenue) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">Card</span><span class="font-medium">{{ formatRupiah(report.summary.card_revenue) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">Transfer</span><span class="font-medium">{{ formatRupiah(report.summary.transfer_revenue) }}</span></div>
          </div>
        </SummaryCard>
      </div>

      <!-- Unpaid Orders Summary (klik → detail) -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <SummaryCard label="Pesanan Belum Bayar" :value="report.summary.unpaid_orders ?? 0" icon="receipt">
          <button type="button" class="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-amber-700 bg-amber-50 hover:bg-amber-100 px-2.5 py-1 rounded-lg transition" @click="openUnpaid">Lihat Detail →</button>
        </SummaryCard>
        <SummaryCard label="Nominal Belum Bayar" :value="formatRupiah(report.summary.unpaid_amount ?? 0)" icon="revenue">
          <button type="button" class="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-amber-700 bg-amber-50 hover:bg-amber-100 px-2.5 py-1 rounded-lg transition" @click="openUnpaid">Lihat Detail →</button>
        </SummaryCard>
      </div>

      <!-- Per Outlet -->
      <AppCard v-if="report.by_outlet && report.by_outlet.length > 1">
        <h3 class="text-sm font-semibold text-gray-700 mb-3">Pendapatan per Outlet</h3>
        <div class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-gray-500">
                <th class="py-2 pr-4 font-medium">Outlet</th>
                <th class="py-2 pr-4 font-medium text-right">Transaksi</th>
                <th class="py-2 pr-4 font-medium text-right">Pendapatan</th>
                <th class="py-2 pr-4 font-medium text-right">Belum Bayar</th>
                <th class="py-2 font-medium text-right">Nominal Belum Bayar</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="o in report.by_outlet" :key="o.outlet_id" class="border-b border-gray-100">
                <td class="py-2 pr-4">{{ o.outlet_name }}</td>
                <td class="py-2 pr-4 text-right">{{ o.total_transactions }}</td>
                <td class="py-2 pr-4 text-right font-medium">{{ formatRupiah(o.total_revenue) }}</td>
                <td class="py-2 pr-4 text-right" :class="o.unpaid_orders > 0 ? 'text-amber-600 font-semibold' : 'text-gray-400'">{{ o.unpaid_orders || 0 }}</td>
                <td class="py-2 text-right" :class="o.unpaid_amount > 0 ? 'text-amber-600 font-semibold' : 'text-gray-400'">{{ formatRupiah(o.unpaid_amount || 0) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <!-- Daily Breakdown -->
      <AppCard v-if="report.daily && report.daily.length > 0">
        <h3 class="text-sm font-semibold text-gray-700 mb-3">Penjualan Harian</h3>
        <div class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-gray-500">
                <th class="py-2 pr-4 font-medium">Tanggal</th>
                <th class="py-2 pr-4 font-medium text-right">Transaksi</th>
                <th class="py-2 pr-4 font-medium text-right">Tamu</th>
                <th class="py-2 pr-4 font-medium text-right">Cash</th>
                <th class="py-2 pr-4 font-medium text-right">QRIS</th>
                <th class="py-2 pr-4 font-medium text-right">Card</th>
                <th class="py-2 pr-4 font-medium text-right">Transfer</th>
                <th class="py-2 font-medium text-right">Total</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in report.daily" :key="d.date" class="border-b border-gray-100">
                <td class="py-2 pr-4">{{ formatDateStr(d.date) }}</td>
                <td class="py-2 pr-4 text-right">{{ d.total_transactions }}</td>
                <td class="py-2 pr-4 text-right">{{ d.total_pax || 0 }}</td>
                <td class="py-2 pr-4 text-right">{{ formatRupiah(d.cash_revenue) }}</td>
                <td class="py-2 pr-4 text-right">{{ formatRupiah(d.qris_revenue) }}</td>
                <td class="py-2 pr-4 text-right">{{ formatRupiah(d.card_revenue) }}</td>
                <td class="py-2 pr-4 text-right">{{ formatRupiah(d.transfer_revenue) }}</td>
                <td class="py-2 text-right font-semibold">{{ formatRupiah(d.total_revenue) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <!-- Transaction Table -->
      <AppCard :padding="false">
        <div class="px-4 pt-4 pb-2">
          <h3 class="text-sm font-semibold text-gray-700">Detail Transaksi</h3>
        </div>

        <!-- Mobile cards -->
        <div class="sm:hidden">
          <div v-if="!report.transactions.length" class="p-6 text-center text-sm text-gray-400">Tidak ada transaksi pada periode ini.</div>
          <ul v-else class="divide-y divide-gray-100">
            <li v-for="(t, i) in report.transactions" :key="i" class="p-4">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-medium text-gray-900 break-words">{{ t.orderer_name || t.outlet_name }}</p>
                  <p class="text-xs text-gray-500">{{ t.outlet_name }} · {{ t.pax > 0 ? t.pax + ' tamu' : '—' }}</p>
                </div>
                <p class="font-bold text-gray-900 shrink-0">{{ formatRupiah(t.total_amount) }}</p>
              </div>
              <p class="text-xs text-gray-500 mt-1.5 flex flex-wrap gap-x-2 gap-y-0.5">
                <span class="capitalize">{{ t.payment_method }}</span>
                <span>· Kasir: {{ t.cashier_name || '—' }}</span>
                <span>· {{ formatDateTime(t.created_at) }}</span>
              </p>
            </li>
          </ul>
        </div>

        <AppTable
          class="hidden sm:block"
          :columns="TX_COLUMNS"
          :rows="report.transactions"
          :loading="false"
          emptyText="Tidak ada transaksi pada periode ini."
        >
          <template #cell-orderer_name="{ row }">
            {{ row.orderer_name || '—' }}
          </template>
          <template #cell-pax="{ row }">
            {{ row.pax > 0 ? row.pax : '—' }}
          </template>
          <template #cell-cashier_name="{ row }">
            {{ row.cashier_name || '—' }}
          </template>
          <template #cell-total_amount="{ row }">
            {{ formatRupiah(row.total_amount) }}
          </template>
          <template #cell-payment_method="{ row }">
            <span class="capitalize">{{ row.payment_method }}</span>
          </template>
          <template #cell-created_at="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </AppTable>
        <div class="px-4 py-3 border-t border-gray-100">
          <AppPagination v-model="page" :total="report.total" :perPage="PAGE_SIZE" />
        </div>
      </AppCard>
    </template>

    <!-- Unpaid detail modal -->
    <AppModal v-model="showUnpaid" title="Pesanan Belum Dibayar" size="xl">
      <div v-if="unpaidLoading" class="py-8 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!unpaidOrders.length" class="py-8 text-center text-sm text-gray-400">Tidak ada pesanan yang belum dibayar.</div>
      <div v-else class="modal-form-grid">
        <p class="modal-span-full text-xs text-gray-500">{{ unpaidOrders.length }} pesanan · total {{ formatRupiah(unpaidTotal) }}</p>
        <div v-for="o in unpaidOrders" :key="o.id" class="rounded-xl border border-gray-100 overflow-hidden">
          <div class="flex items-start justify-between gap-2 px-3 py-2 bg-gray-50">
            <div class="min-w-0">
              <p class="font-medium text-gray-900 text-sm truncate">{{ o.customer_name || 'Tanpa Nama' }}</p>
              <p class="text-xs text-gray-500">Meja {{ o.table_number || '—' }} · {{ o.pax > 0 ? o.pax + ' tamu' : '—' }}<span v-if="o.outlet_name"> · {{ o.outlet_name }}</span></p>
            </div>
            <div class="text-right shrink-0">
              <p class="font-bold text-gray-900 text-sm">{{ formatRupiah(o.total_amount) }}</p>
              <span class="inline-block text-[10px] uppercase font-bold tracking-wide text-amber-600 bg-amber-50 rounded px-1.5 py-0.5 mt-0.5">{{ o.status || 'belum bayar' }}</span>
            </div>
          </div>
          <ul class="px-3 py-1.5">
            <li v-for="(it, i) in parseItems(o.items)" :key="i" class="flex justify-between gap-2 py-1 text-xs border-b border-gray-50 last:border-0">
              <span class="text-gray-700 truncate">{{ it.product_name }} <span class="text-gray-400">×{{ it.qty }}</span></span>
              <span class="text-gray-600 shrink-0">{{ formatRupiah(it.subtotal || (it.price * it.qty)) }}</span>
            </li>
            <li v-if="!parseItems(o.items).length" class="py-1 text-xs text-gray-400">Tidak ada rincian item.</li>
          </ul>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { salesApi } from '@/api/sales.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateTime, formatDateStr, todayDateString } from '@/utils/format.js'
import { PAGE_SIZE } from '@/utils/constants.js'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import SummaryCard   from '@/components/SummaryCard.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const dateFrom       = ref(today)
const dateTo         = ref(today)
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; applyFilter() })
const selectedOutlet = ref('')
const outletOptions  = ref([])
const loading        = ref(false)
const errorMsg       = ref('')
const report         = ref(null)
const page           = ref(1)

// Unpaid orders detail modal
const showUnpaid    = ref(false)
const unpaidLoading = ref(false)
const unpaidOrders  = ref([])
const unpaidTotal   = computed(() => unpaidOrders.value.reduce((s, o) => s + (o.total_amount || 0), 0))
function parseItems(s) {
  if (Array.isArray(s)) return s
  try { const a = JSON.parse(s || '[]'); return Array.isArray(a) ? a : [] } catch { return [] }
}
async function openUnpaid() {
  showUnpaid.value = true
  unpaidLoading.value = true
  try {
    const res = await salesApi.getUnpaidOrders({
      outlet_id: selectedOutlet.value || undefined,
      date_from: dateFrom.value,
      date_to: dateTo.value,
      limit: 200,
    })
    unpaidOrders.value = res?.orders ?? res?.data?.orders ?? (Array.isArray(res) ? res : [])
  } catch { unpaidOrders.value = [] } finally { unpaidLoading.value = false }
}

// Download Excel mengikuti filter aktif (tanggal + outlet); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    const blob = await salesApi.exportReport(params)

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Penjualan_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`

    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    errorMsg.value = 'Gagal mengunduh Excel. Coba lagi, atau persempit rentang tanggal.'
  } finally {
    exporting.value = false
  }
}

const TX_COLUMNS = [
  { key: 'outlet_name',    label: 'Outlet' },
  { key: 'orderer_name',   label: 'Pemesan' },
  { key: 'pax',            label: 'Tamu' },
  { key: 'cashier_name',   label: 'Kasir' },
  { key: 'payment_method', label: 'Metode Bayar' },
  { key: 'total_amount',   label: 'Total' },
  { key: 'created_at',     label: 'Waktu' },
]

onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    const list = data.outlets ?? data ?? []
    outletOptions.value = list.map(o => ({ value: o.id, label: o.name }))
  } catch { /* ignore */ }
  fetchReport()
})

watch(page, fetchReport)

// Set page ke 1 memicu watcher di atas; fetch manual hanya bila sudah di page 1
// (kalau tidak, tombol Tampilkan dari page > 1 memicu dua request identik).
function applyFilter() {
  if (page.value === 1) fetchReport()
  else page.value = 1
}

async function fetchReport() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = {
      date_from: dateFrom.value,
      date_to: dateTo.value,
      page: page.value,
      limit: PAGE_SIZE,
    }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    report.value = await salesApi.getReport(params)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan penjualan.'
  } finally {
    loading.value = false
  }
}
</script>
