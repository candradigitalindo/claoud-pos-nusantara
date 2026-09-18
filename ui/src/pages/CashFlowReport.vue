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
        <button @click="fetchReport"
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

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <!-- Cash Flow Statement -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- Penerimaan -->
        <AppCard>
          <h4 class="text-xs font-bold text-gray-500 uppercase tracking-wide mb-3">Penerimaan Operasi</h4>
          <div class="space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-gray-600">Penjualan</span>
              <span class="font-medium text-gray-800">{{ formatRupiah(report.summary.sales_receipts) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Pemasukan Kas Lainnya</span>
              <span class="font-medium text-gray-800">{{ formatRupiah(report.summary.other_receipts) }}</span>
            </div>
            <!-- Uang muka reservasi: kas masuk saat DP tervalidasi (kewajiban, bukan
                 pendapatan). Penjualan di atas sudah dikurangi bagian yang dibayar
                 dari uang muka supaya tidak dihitung dua kali. -->
            <div class="flex justify-between">
              <span class="text-gray-600">Uang Muka Reservasi <span class="text-[11px] text-gray-400">(DP/pelunasan)</span></span>
              <span class="font-medium text-gray-800">{{ formatRupiah(report.summary.deposit_receipts) }}</span>
            </div>
            <div class="flex justify-between pt-2 border-t border-emerald-200 font-semibold">
              <span class="text-emerald-700">Total Penerimaan</span>
              <span class="text-emerald-700">{{ formatRupiah(report.summary.total_receipts) }}</span>
            </div>
          </div>
        </AppCard>

        <!-- Pengeluaran -->
        <AppCard>
          <h4 class="text-xs font-bold text-gray-500 uppercase tracking-wide mb-3">Pengeluaran Operasi</h4>
          <div class="space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-gray-600">Pembelian Bahan Baku</span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.cogs_payments) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Pembayaran Jasa</span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.service_payments) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Pengeluaran Operasional</span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.opex_payments) }}</span>
            </div>
            <!-- Investasi: mengurangi kas, tapi bukan beban di Laba/Rugi — menjadi
                 Aset Tetap / Projek Berjalan di Neraca. -->
            <div class="flex justify-between pt-2 border-t border-dashed border-gray-200">
              <span class="text-gray-600">Belanja Modal <span class="text-[11px] text-gray-400">(peralatan → aset)</span></span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.capex_payments) }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Belanja Projek</span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.project_payments) }}</span>
            </div>
            <div v-if="report.summary.deposit_refunds" class="flex justify-between">
              <span class="text-gray-600">Refund Uang Muka Reservasi</span>
              <span class="font-medium text-red-600">{{ formatRupiah(report.summary.deposit_refunds) }}</span>
            </div>
            <div class="flex justify-between pt-2 border-t border-red-200 font-semibold">
              <span class="text-red-600">Total Pengeluaran</span>
              <span class="text-red-600">{{ formatRupiah(report.summary.total_payments) }}</span>
            </div>
          </div>
        </AppCard>

        <!-- Arus Kas Bersih -->
        <AppCard>
          <h4 class="text-xs font-bold text-gray-500 uppercase tracking-wide mb-3">Arus Kas Bersih</h4>
          <div class="flex items-center justify-center h-20">
            <span class="text-2xl font-bold" :class="report.summary.net_cash_flow >= 0 ? 'text-emerald-700' : 'text-red-600'">
              {{ formatRupiah(report.summary.net_cash_flow) }}
            </span>
          </div>
        </AppCard>
      </div>

      <!-- Daily -->
      <AppCard v-if="report.daily?.length > 0" :padding="false">
        <div class="px-4 pt-4 pb-2">
          <h3 class="text-sm font-semibold text-gray-700">Arus Kas Harian</h3>
        </div>
        <AppTable :columns="DAILY_COLS" :rows="report.daily" :loading="false" emptyText="Tidak ada data.">
          <template #cell-date="{ row }">{{ formatDateStr(row.date) }}</template>
          <template #cell-sales_receipts="{ row }">{{ formatRupiah(row.sales_receipts) }}</template>
          <template #cell-other_receipts="{ row }">{{ formatRupiah(row.other_receipts) }}</template>
          <template #cell-deposit_receipts="{ row }">{{ formatRupiah(row.deposit_receipts) }}</template>
          <template #cell-cogs_payments="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.cogs_payments) }}</span>
          </template>
          <template #cell-service_payments="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.service_payments) }}</span>
          </template>
          <template #cell-opex_payments="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.opex_payments) }}</span>
          </template>
          <template #cell-capex_payments="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.capex_payments) }}</span>
          </template>
          <template #cell-project_payments="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.project_payments) }}</span>
          </template>
          <template #cell-net_cash_flow="{ row }">
            <span :class="row.net_cash_flow >= 0 ? 'text-emerald-700 font-semibold' : 'text-red-600 font-semibold'">
              {{ formatRupiah(row.net_cash_flow) }}
            </span>
          </template>
        </AppTable>
      </AppCard>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { salesApi }    from '@/api/sales.js'
import { outletsApi }  from '@/api/outlets.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard      from '@/components/ui/AppCard.vue'
import AppTable     from '@/components/ui/AppTable.vue'
import AppAlert     from '@/components/ui/AppAlert.vue'
import AppSpinner   from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const dateFrom       = ref(today)
const dateTo         = ref(today)
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; fetchReport() })
const selectedOutlet = ref('')
const outletOptions  = ref([])
const loading        = ref(false)
const errorMsg       = ref('')
const report         = ref(null)

const DAILY_COLS = [
  { key: 'date',             label: 'Tanggal' },
  { key: 'sales_receipts',   label: 'Penjualan', align: 'right' },
  { key: 'other_receipts',   label: 'Kas Masuk', align: 'right' },
  { key: 'deposit_receipts', label: 'Uang Muka', align: 'right' },
  { key: 'cogs_payments',    label: 'Bahan Baku', align: 'right' },
  { key: 'service_payments', label: 'Jasa', align: 'right' },
  { key: 'opex_payments',    label: 'Opex', align: 'right' },
  { key: 'capex_payments',   label: 'Modal', align: 'right' },
  { key: 'project_payments', label: 'Projek', align: 'right' },
  { key: 'net_cash_flow',    label: 'Arus Bersih', align: 'right' },
]

onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    const list = data.outlets ?? data ?? []
    outletOptions.value = list.map(o => ({ value: o.id, label: o.name }))
  } catch { /* ignore */ }
  fetchReport()
})

async function fetchReport() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    report.value = await salesApi.getCashFlowReport(params)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan arus kas.'
  } finally {
    loading.value = false
  }
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
    const blob = await salesApi.exportCashFlowReport(params)

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Arus-Kas_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`

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
</script>
