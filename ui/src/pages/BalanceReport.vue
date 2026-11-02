<template>
  <div class="space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex min-w-0 flex-col gap-1">
          <label for="balance-as-of" class="text-sm font-medium text-gray-700">Tanggal posisi neraca</label>
          <input id="balance-as-of" v-model="asOfDate" type="date"
            class="w-full min-w-0 rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-800 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500" />
        </div>
        <button @click="fetchReport" :disabled="loading || !asOfDate"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          Tampilkan
        </button>
        <button @click="downloadExcel" :disabled="!canExport"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
      <p class="mt-3 text-xs text-gray-500">Menampilkan saldo kumulatif hingga tanggal yang dipilih.</p>
    </AppCard>

    <AppAlert type="error" :message="errorMsg || exportError" />

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <p class="text-sm text-gray-600" role="status">Posisi neraca per <b>{{ formatDateStr(report.date_to) }}</b>. Saldo kumulatif hingga tanggal ini, termasuk seluruh periode sebelumnya.</p>
      <AppAlert v-if="report.review_count > 0" type="warning">
        {{ report.review_count }} catatan sumber perlu ditinjau. Periksa kelengkapan riwayat dan harga pokok penjualan (HPP). Status seimbang belum menjamin semua catatan sudah lengkap.
      </AppAlert>
      <FinanceAccountTotals :accounts="report.accounts" />
      <!-- Neraca: Aset = Kewajiban + Ekuitas -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">

        <!-- ASET (Left side) -->
        <AppCard>
          <h3 class="text-sm font-bold text-gray-800 uppercase tracking-wide mb-4 flex items-center gap-2">
            <svg class="w-4 h-4 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"/>
            </svg>
            Aset
          </h3>
          <div class="space-y-3">
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Kas & Setara Kas</p>
                <p class="text-xs text-gray-400">Saldo buku penerimaan dan pengeluaran kas sampai tanggal posisi</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.cash_and_equivalents) }}</span>
            </div>
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Piutang Usaha</p>
                <p class="text-xs text-gray-400">Sisa tagihan per tanggal posisi, termasuk pembayaran sebagian</p>
              </div>
              <span class="text-sm font-semibold" :class="report.receivables > 0 ? 'text-amber-600' : 'text-gray-800'">{{ formatRupiah(report.receivables) }}</span>
            </div>
            <!-- Tiga baris di bawah datang dari modul Gudang dan Perlengkapan:
                 sebelumnya nilai stok dan aset tidak pernah sampai ke Neraca. -->
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Persediaan</p>
                <p class="text-xs text-gray-400">Nilai buku persediaan, karantina, dan pengiriman (qty × rata-rata biaya)</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.inventory) }}</span>
            </div>
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Aset Tetap</p>
                <p class="text-xs text-gray-400">Nilai buku peralatan per tanggal posisi (perolehan − penyusutan)</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.fixed_assets) }}</span>
            </div>
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Projek Berjalan</p>
                <p class="text-xs text-gray-400">Belanja projek yang dibayar dan belum berwujud aset</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.projects_in_progress) }}</span>
            </div>
          </div>
          <div class="flex justify-between items-center pt-4 mt-2 border-t-2 border-emerald-200">
            <span class="text-sm font-bold text-emerald-700">Total Aset</span>
            <span class="text-lg font-bold text-emerald-700">{{ formatRupiah(report.total_assets) }}</span>
          </div>
        </AppCard>

        <!-- KEWAJIBAN + EKUITAS (Right side) -->
        <div class="space-y-6">
          <!-- Kewajiban -->
          <AppCard>
            <h3 class="text-sm font-bold text-gray-800 uppercase tracking-wide mb-4 flex items-center gap-2">
              <svg class="w-4 h-4 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/>
              </svg>
              Kewajiban
            </h3>
            <div class="space-y-3">
              <div class="flex justify-between items-center py-2 border-b border-gray-100">
                <div>
                  <p class="text-sm font-medium text-gray-700">Hutang Usaha</p>
                  <p class="text-xs text-gray-400">Hutang pengadaan dan operasional yang belum dilunasi</p>
                </div>
                <span class="text-sm font-semibold text-red-600">{{ formatRupiah(report.accounts_payable) }}</span>
              </div>
              <div class="flex justify-between items-center py-2 border-b border-gray-100">
                <div>
                  <p class="text-sm font-medium text-gray-700">Uang Muka Pelanggan</p>
                  <p class="text-xs text-gray-400">DP reservasi tervalidasi yang belum ditutup di POS</p>
                </div>
                <span class="text-sm font-semibold text-red-600">{{ formatRupiah(report.customer_deposits) }}</span>
              </div>
              <div class="flex justify-between items-center py-2 border-b border-gray-100">
                <div>
                  <p class="text-sm font-medium text-gray-700">Hutang Pajak Restoran</p>
                  <p class="text-xs text-gray-400">Pajak transaksi yang belum disetorkan</p>
                </div>
                <span class="text-sm font-semibold text-red-600">{{ formatRupiah(report.tax_payable) }}</span>
              </div>
            </div>
            <div class="flex justify-between items-center py-2"><span class="text-sm text-gray-700">Pinjaman</span><span class="text-sm font-semibold text-red-600">{{ formatRupiah(report.financing_liabilities) }}</span></div>
            <div class="flex justify-between items-center pt-4 mt-2 border-t-2 border-red-200">
              <span class="text-sm font-bold text-red-600">Total Kewajiban</span>
              <span class="text-base font-bold text-red-600">{{ formatRupiah(report.total_liabilities) }}</span>
            </div>
          </AppCard>

          <!-- Ekuitas -->
          <AppCard>
            <h3 class="text-sm font-bold text-gray-800 uppercase tracking-wide mb-4 flex items-center gap-2">
              <svg class="w-4 h-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5"/>
              </svg>
              Ekuitas
            </h3>
            <div class="flex justify-between items-center pt-2">
              <span class="text-sm font-medium text-gray-700">Ekuitas dan akumulasi hasil usaha</span>
              <span class="text-sm font-semibold" :class="report.total_equity >= 0 ? 'text-blue-700' : 'text-red-600'">
                {{ formatRupiah(report.total_equity) }}
              </span>
            </div>
            <div class="flex justify-between items-center pt-4 mt-3 border-t-2 border-blue-200">
              <span class="text-sm font-bold text-blue-700">Total Ekuitas</span>
              <span class="text-base font-bold" :class="report.total_equity >= 0 ? 'text-blue-700' : 'text-red-600'">{{ formatRupiah(report.total_equity) }}</span>
            </div>
          </AppCard>
        </div>
      </div>

      <!-- Balance Check -->
      <div class="flex items-center justify-center gap-3 py-3 rounded-xl text-sm"
        :class="isBalanced ? 'bg-emerald-50 border border-emerald-200 text-emerald-700' : 'bg-red-50 border border-red-200 text-red-700'">
        <svg v-if="isBalanced" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <svg v-else class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
        </svg>
        <span class="font-medium">
          Aset ({{ formatRupiah(report.total_assets) }}) = Kewajiban ({{ formatRupiah(report.total_liabilities) }}) + Ekuitas ({{ formatRupiah(report.total_equity) }})
          <template v-if="isBalanced"> — Neraca Seimbang ✓</template>
          <template v-else> — Neraca Tidak Seimbang</template>
        </span>
      </div>

      <!-- Detail Komponen -->
      <AppCard>
        <h3 class="text-sm font-semibold text-gray-700 mb-3">Detail Komponen Kumulatif</h3>
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Pendapatan Transaksi Kumulatif</p>
            <p class="font-semibold text-gray-800">{{ formatRupiah(report.total_revenue) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Pendapatan Lainnya Kumulatif</p>
            <p class="font-semibold text-gray-800">{{ formatRupiah(report.total_cash_in) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Beban Kumulatif</p>
            <p class="font-semibold text-red-600">{{ formatRupiah(report.total_expense) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Piutang Usaha per Tanggal Posisi</p>
            <p class="font-semibold" :class="report.unpaid_amount > 0 ? 'text-amber-600' : 'text-gray-800'">{{ formatRupiah(report.unpaid_amount) }}</p>
          </div>
        </div>
      </AppCard>

      <!-- Per Outlet -->
      <AppCard v-if="report.outlets?.length > 0" :padding="false">
        <div class="px-4 pt-4 pb-2">
          <h3 class="text-sm font-semibold text-gray-700">Neraca Per Outlet</h3>
        </div>
        <AppTable :columns="OUTLET_COLS" :rows="report.outlets" :loading="false" emptyText="Tidak ada data.">
          <template #cell-cash_and_equivalents="{ row }">{{ formatRupiah(row.cash_and_equivalents) }}</template>
          <template #cell-receivables="{ row }">
            <span :class="row.receivables > 0 ? 'text-amber-600' : ''">{{ formatRupiah(row.receivables) }}</span>
          </template>
          <template #cell-inventory="{ row }">{{ formatRupiah(row.inventory) }}</template>
          <template #cell-fixed_assets="{ row }">{{ formatRupiah(row.fixed_assets) }}</template>
          <template #cell-total_assets="{ row }">
            <span class="font-semibold text-emerald-700">{{ formatRupiah(row.total_assets) }}</span>
          </template>
          <template #cell-accounts_payable="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.accounts_payable) }}</span>
          </template>
          <template #cell-customer_deposits="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.customer_deposits) }}</span>
          </template>
          <template #cell-tax_payable="{ row }">
            <span class="text-red-600">{{ formatRupiah(row.tax_payable) }}</span>
          </template>
          <template #cell-total_equity="{ row }">
            <span :class="row.total_equity >= 0 ? 'text-blue-700 font-semibold' : 'text-red-600 font-semibold'">
              {{ formatRupiah(row.total_equity) }}
            </span>
          </template>
        </AppTable>
      </AppCard>
    </template>
  </div>
</template>

<script setup>
import FinanceAccountTotals from '@/components/FinanceAccountTotals.vue'
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { salesApi }    from '@/api/sales.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard      from '@/components/ui/AppCard.vue'
import AppTable     from '@/components/ui/AppTable.vue'
import AppAlert     from '@/components/ui/AppAlert.vue'
import AppSpinner   from '@/components/ui/AppSpinner.vue'

const asOfDate = ref(todayDateString())
watch(asOfDate, fetchReport)
const loading  = ref(false)
const errorMsg = ref('')
const exportError = ref('')
const report   = ref(null)
let requestSequence = 0
let disposed = false

const isBalanced = computed(() => {
  if (!report.value) return true
  const diff = Math.abs(report.value.total_assets - (report.value.total_liabilities + report.value.total_equity))
  return diff < 0.02
})

const OUTLET_COLS = [
  { key: 'outlet_name',          label: 'Outlet' },
  { key: 'cash_and_equivalents', label: 'Kas', align: 'right' },
  { key: 'receivables',          label: 'Piutang', align: 'right' },
  { key: 'inventory',            label: 'Persediaan', align: 'right' },
  { key: 'fixed_assets',         label: 'Aset Tetap', align: 'right' },
  { key: 'total_assets',         label: 'Total Aset', align: 'right' },
  { key: 'accounts_payable',     label: 'Hutang Usaha', align: 'right' },
  { key: 'customer_deposits',    label: 'Uang Muka', align: 'right' },
  { key: 'tax_payable',          label: 'Hutang Pajak', align: 'right' },
  { key: 'total_equity',         label: 'Ekuitas', align: 'right' },
]

onMounted(fetchReport)

async function fetchReport() {
  if (disposed) return
  const sequence = ++requestSequence
  const date = asOfDate.value
  loading.value = true
  errorMsg.value = ''
  exportError.value = ''
  report.value = null
  if (!date) {
    errorMsg.value = 'Pilih tanggal posisi neraca.'
    loading.value = false
    return
  }
  try {
    const data = await salesApi.getBalanceReport({ date_from: date, date_to: date })
    if (!disposed && sequence === requestSequence) report.value = data
  } catch (err) {
    if (!disposed && sequence === requestSequence) errorMsg.value = err?.message ?? 'Gagal memuat laporan neraca.'
  } finally {
    if (!disposed && sequence === requestSequence) loading.value = false
  }
}

onBeforeUnmount(() => { disposed = true; requestSequence++ })

// Export uses the loaded position date. A download already in progress retains
// that date in both its request and filename if the user selects another date.
const exporting = ref(false)
const canExport = computed(() => !loading.value && !exporting.value && !errorMsg.value &&
  !!report.value && report.value.date_to === asOfDate.value)
async function downloadExcel() {
  if (disposed || !canExport.value) return
  const date = report.value.date_to
  const sequence = requestSequence
  const params = { date_from: date, date_to: date }
  const filename = `Laporan-Neraca_per_${date}.xlsx`
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
