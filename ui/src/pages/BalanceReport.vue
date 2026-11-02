<template>
  <div class="min-w-0 space-y-6">
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex min-w-0 flex-col gap-1">
          <label for="balance-range" class="text-sm font-medium text-gray-700">Periode laporan</label>
          <DateRangePicker v-model="range" app-timezone input-id="balance-range" label="Periode laporan" />
        </div>
        <button @click="fetchReport" :disabled="loading || !range.from || !range.to"
          class="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60">
          Tampilkan
        </button>
        <button @click="downloadExcel" :disabled="!canExport"
          class="inline-flex items-center gap-2 rounded-lg border border-emerald-600 bg-white px-4 py-2 text-sm font-medium text-emerald-700 shadow-sm transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60">
          <svg v-if="!exporting" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
      <p class="mt-3 text-xs text-gray-500">Angka utama menunjukkan perubahan selama periode yang dipilih. Saldo awal dan saldo akhir ditampilkan terpisah.</p>
    </AppCard>

    <AppAlert type="error" :message="errorMsg || exportError" />
    <div v-if="loading" class="flex justify-center py-12"><AppSpinner size="lg" /></div>

    <template v-if="!loading && report && period && opening">
      <p class="text-sm text-gray-600" role="status">Perubahan periode <b>{{ formatDateStr(report.date_from) }} – {{ formatDateStr(report.date_to) }}</b>.</p>
      <AppAlert v-if="period.review_count > 0" type="warning">
        {{ period.review_count }} catatan pada periode ini perlu ditinjau. Periksa bukti transaksi dan harga pokok penjualan (HPP) yang belum lengkap.
      </AppAlert>

      <div class="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-3" aria-label="Ringkasan kas periode">
        <AppCard v-for="card in CASH_CARDS" :key="card.key" :data-cash-summary="card.key">
          <h3 class="text-sm font-semibold text-gray-700">{{ card.label }}</h3>
          <p class="mt-2 text-2xl font-bold" :class="card.key === 'cash_payments' || period[card.key] < 0 ? 'text-red-600' : 'text-emerald-700'" data-cash-value>{{ formatRupiah(period[card.key]) }}</p>
          <p class="mt-1 text-xs text-gray-500">{{ card.description }}</p>
        </AppCard>
      </div>
      <p class="text-xs text-gray-500">Kas dan setara kas mencakup tunai, QRIS, kartu, dan transfer sesuai pembukuan.</p>

      <div class="grid min-w-0 grid-cols-1 gap-6 lg:grid-cols-2">
        <AppCard data-section="period-assets">
          <h3 class="mb-4 text-sm font-bold uppercase tracking-wide text-emerald-700">Perubahan Aset</h3>
          <div v-for="row in ASSET_ROWS" :key="row.key" class="flex items-center justify-between gap-4 border-b border-gray-100 py-3">
            <div class="min-w-0"><p class="text-sm font-medium text-gray-700">{{ row.label }}</p><p class="text-xs text-gray-400">{{ row.description }}</p></div>
            <span class="shrink-0 text-sm font-semibold text-gray-800">{{ formatRupiah(period[row.key]) }}</span>
          </div>
          <div class="mt-2 flex items-center justify-between gap-4 border-t-2 border-emerald-200 pt-4">
            <span class="text-sm font-bold text-emerald-700">Total Perubahan Aset</span>
            <span class="text-lg font-bold text-emerald-700">{{ formatRupiah(period.total_assets) }}</span>
          </div>
        </AppCard>

        <div class="min-w-0 space-y-6">
          <AppCard data-section="period-liabilities">
            <h3 class="mb-4 text-sm font-bold uppercase tracking-wide text-red-600">Perubahan Kewajiban</h3>
            <div v-for="row in LIABILITY_ROWS" :key="row.key" class="flex items-center justify-between gap-4 border-b border-gray-100 py-3">
              <div class="min-w-0"><p class="text-sm font-medium text-gray-700">{{ row.label }}</p><p class="text-xs text-gray-400">{{ row.description }}</p></div>
              <span class="shrink-0 text-sm font-semibold text-gray-800">{{ formatRupiah(period[row.key]) }}</span>
            </div>
            <div class="mt-2 flex items-center justify-between gap-4 border-t-2 border-red-200 pt-4">
              <span class="text-sm font-bold text-red-600">Total Perubahan Kewajiban</span>
              <span class="text-base font-bold text-red-600">{{ formatRupiah(period.total_liabilities) }}</span>
            </div>
          </AppCard>
          <AppCard data-section="period-equity">
            <h3 class="mb-4 text-sm font-bold uppercase tracking-wide text-blue-700">Perubahan Ekuitas</h3>
            <p class="text-xs text-gray-500">Perubahan ekuitas dan hasil usaha selama periode.</p>
            <div class="mt-3 flex items-center justify-between gap-4 border-t-2 border-blue-200 pt-4">
              <span class="text-sm font-bold text-blue-700">Total Perubahan Ekuitas</span>
              <span class="text-base font-bold" :class="period.total_equity >= 0 ? 'text-blue-700' : 'text-red-600'">{{ formatRupiah(period.total_equity) }}</span>
            </div>
          </AppCard>
        </div>
      </div>

      <AppCard data-section="period-components">
        <h3 class="mb-3 text-sm font-semibold text-gray-700">Komponen Periode</h3>
        <div class="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
          <div v-for="row in PERIOD_COMPONENTS" :key="row.key" class="rounded-lg bg-gray-50 p-3">
            <p class="mb-1 text-xs text-gray-500">{{ row.label }}</p>
            <p class="font-semibold text-gray-800">{{ formatRupiah(period[row.key]) }}</p>
          </div>
        </div>
      </AppCard>

      <FinanceAccountTotals :accounts="period.accounts" />
      <AppCard v-if="period.outlets?.length > 0" :padding="false" data-section="period-outlets">
        <div class="px-4 pb-2 pt-4"><h3 class="text-sm font-semibold text-gray-700">Perubahan Per Outlet</h3></div>
        <AppTable :columns="OUTLET_COLS" :rows="period.outlets" :loading="false" emptyText="Tidak ada data.">
          <template v-for="column in OUTLET_COLS.slice(1)" :key="column.key" #[`cell-${column.key}`]="{ row }">{{ formatRupiah(row[column.key]) }}</template>
        </AppTable>
      </AppCard>

      <AppCard class="min-w-0" data-section="balance-comparison">
        <h3 class="text-sm font-semibold text-gray-800">Saldo Awal dan Saldo Akhir</h3>
        <p class="mt-1 text-xs text-gray-500">Saldo awal sebelum {{ formatDateStr(report.date_from) }} + perubahan periode = saldo akhir hingga {{ formatDateStr(report.date_to) }}. Saldo awal dan akhir mencakup riwayat sebelumnya.</p>
        <AppAlert v-if="report.review_count > 0" type="warning" class="mt-3">
          {{ report.review_count }} catatan kumulatif pada saldo akhir perlu ditinjau. Jumlah ini mencakup riwayat hingga {{ formatDateStr(report.date_to) }}, bukan hanya periode yang dipilih.
        </AppAlert>
        <div class="mt-4 overflow-x-auto">
          <table class="w-full text-sm" aria-label="Perbandingan saldo awal, perubahan, dan saldo akhir">
            <thead><tr class="text-left text-gray-500"><th class="py-2 pr-4">Komponen</th><th class="whitespace-nowrap px-3 text-right">Saldo Awal</th><th class="whitespace-nowrap px-3 text-right">Perubahan Periode</th><th class="whitespace-nowrap pl-3 text-right">Saldo Akhir</th></tr></thead>
            <tbody>
              <tr v-for="row in COMPARISON_ROWS" :key="row.key" :data-balance-field="row.key" class="border-t border-gray-100">
                <th scope="row" class="py-3 pr-4 text-left font-medium text-gray-700">{{ row.label }}</th>
                <td data-value="opening" class="whitespace-nowrap px-3 text-right">{{ formatRupiah(opening[row.key]) }}</td>
                <td data-value="period" class="whitespace-nowrap px-3 text-right font-semibold text-emerald-700">{{ formatRupiah(period[row.key]) }}</td>
                <td data-value="closing" class="whitespace-nowrap pl-3 text-right">{{ formatRupiah(report[row.key]) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

    </template>
  </div>
</template>

<script setup>
import FinanceAccountTotals from '@/components/FinanceAccountTotals.vue'
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { salesApi } from '@/api/sales.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const range = ref({ from: today, to: today, label: 'Hari Ini' })
watch(range, fetchReport)
const loading = ref(false)
const errorMsg = ref('')
const exportError = ref('')
const report = ref(null)
const period = computed(() => report.value?.period)
const opening = computed(() => report.value?.opening)
let requestSequence = 0
let disposed = false

const CASH_CARDS = [
  { key: 'cash_receipts', label: 'Kas Masuk', description: 'Penerimaan kas selama periode' },
  { key: 'cash_payments', label: 'Kas Keluar', description: 'Pengeluaran kas selama periode' },
  { key: 'cash_and_equivalents', label: 'Perubahan Kas', description: 'Kas masuk dikurangi kas keluar selama periode' },
]
const ASSET_ROWS = [
  { key: 'cash_and_equivalents', label: 'Kas & Setara Kas', description: 'Kas masuk dikurangi kas keluar selama periode' },
  { key: 'receivables', label: 'Piutang Usaha', description: 'Penambahan dan pelunasan piutang selama periode' },
  { key: 'inventory', label: 'Persediaan', description: 'Perubahan nilai buku persediaan selama periode' },
  { key: 'fixed_assets', label: 'Aset Tetap', description: 'Penambahan dan penyusutan aset selama periode' },
  { key: 'projects_in_progress', label: 'Projek Berjalan', description: 'Perubahan belanja projek yang belum berwujud aset' },
]
const LIABILITY_ROWS = [
  { key: 'accounts_payable', label: 'Hutang Usaha', description: 'Penambahan dan pelunasan hutang selama periode' },
  { key: 'customer_deposits', label: 'Uang Muka Pelanggan', description: 'Perubahan DP reservasi selama periode' },
  { key: 'tax_payable', label: 'Hutang Pajak Restoran', description: 'Perubahan pajak yang belum disetorkan' },
  { key: 'financing_liabilities', label: 'Pinjaman', description: 'Penerimaan dan pelunasan pinjaman selama periode' },
]
const PERIOD_COMPONENTS = [
  { key: 'total_revenue', label: 'Pendapatan Transaksi' },
  { key: 'total_cash_in', label: 'Pendapatan Lainnya' },
  { key: 'total_expense', label: 'Beban' },
  { key: 'unpaid_amount', label: 'Perubahan Piutang Usaha' },
]
const COMPARISON_ROWS = [
  ...ASSET_ROWS,
  { key: 'total_assets', label: 'Total Aset' },
  ...LIABILITY_ROWS,
  { key: 'total_liabilities', label: 'Total Kewajiban' },
  { key: 'total_equity', label: 'Total Ekuitas' },
  { key: 'balance_difference', label: 'Selisih Aset − Kewajiban − Ekuitas' },
]
const OUTLET_COLS = [
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'cash_and_equivalents', label: 'Perubahan Kas', align: 'right' },
  { key: 'receivables', label: 'Perubahan Piutang', align: 'right' },
  { key: 'inventory', label: 'Perubahan Persediaan', align: 'right' },
  { key: 'fixed_assets', label: 'Perubahan Aset Tetap', align: 'right' },
  { key: 'total_assets', label: 'Perubahan Aset', align: 'right' },
  { key: 'accounts_payable', label: 'Perubahan Hutang', align: 'right' },
  { key: 'customer_deposits', label: 'Perubahan Uang Muka', align: 'right' },
  { key: 'tax_payable', label: 'Perubahan Pajak', align: 'right' },
  { key: 'total_equity', label: 'Perubahan Ekuitas', align: 'right' },
]

onMounted(fetchReport)
async function fetchReport() {
  if (disposed) return
  const sequence = ++requestSequence
  const { from, to } = range.value
  loading.value = true
  errorMsg.value = ''
  exportError.value = ''
  report.value = null
  if (!from || !to || from > to) {
    errorMsg.value = 'Pilih tanggal awal dan akhir periode yang valid.'
    loading.value = false
    return
  }
  try {
    const data = await salesApi.getBalanceReport({ date_from: from, date_to: to })
    if (!data?.opening || !data?.period) throw new Error('Rincian perubahan periode belum tersedia. Muat ulang laporan.')
    if (data.date_from !== from || data.date_to !== to) throw new Error('Periode hasil tidak sesuai dengan filter. Muat ulang laporan.')
    if (!disposed && sequence === requestSequence) report.value = data
  } catch (err) {
    if (!disposed && sequence === requestSequence) errorMsg.value = err?.message ?? 'Gagal memuat laporan neraca.'
  } finally {
    if (!disposed && sequence === requestSequence) loading.value = false
  }
}
onBeforeUnmount(() => { disposed = true; requestSequence++ })

// An in-progress export retains both boundaries of its loaded report.
const exporting = ref(false)
const canExport = computed(() => !loading.value && !exporting.value && !errorMsg.value &&
  !!report.value && report.value.date_from === range.value.from && report.value.date_to === range.value.to)
async function downloadExcel() {
  if (disposed || !canExport.value) return
  const from = report.value.date_from
  const to = report.value.date_to
  const sequence = requestSequence
  const params = { date_from: from, date_to: to }
  const filename = `Laporan-Neraca_${from}_${to}.xlsx`
  exporting.value = true
  exportError.value = ''
  try {
    const blob = await salesApi.exportBalanceReport(params)
    if (disposed) return
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    if (!disposed && sequence === requestSequence) exportError.value = 'Gagal mengunduh Excel. Coba lagi.'
  } finally {
    if (!disposed) exporting.value = false
  }
}
</script>
