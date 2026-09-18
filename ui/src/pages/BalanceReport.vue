<template>
  <div class="space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Tanggal</label>
          <DateRangePicker v-model="range" />
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
                <p class="text-xs text-gray-400">Pendapatan + Pemasukan − Pengeluaran</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.cash_and_equivalents) }}</span>
            </div>
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Piutang Usaha</p>
                <p class="text-xs text-gray-400">Pesanan belum dibayar</p>
              </div>
              <span class="text-sm font-semibold" :class="report.receivables > 0 ? 'text-amber-600' : 'text-gray-800'">{{ formatRupiah(report.receivables) }}</span>
            </div>
            <!-- Tiga baris di bawah datang dari modul Gudang dan Perlengkapan:
                 sebelumnya nilai stok dan aset tidak pernah sampai ke Neraca. -->
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Persediaan</p>
                <p class="text-xs text-gray-400">Nilai buku stok gudang saat ini (qty × rata-rata biaya)</p>
              </div>
              <span class="text-sm font-semibold text-gray-800">{{ formatRupiah(report.inventory) }}</span>
            </div>
            <div class="flex justify-between items-center py-2 border-b border-gray-100">
              <div>
                <p class="text-sm font-medium text-gray-700">Aset Tetap</p>
                <p class="text-xs text-gray-400">Nilai buku peralatan per akhir periode (perolehan − penyusutan)</p>
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
                  <p class="text-xs text-gray-400">Pengadaan disetujui belum dibayar</p>
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
                  <p class="text-xs text-gray-400">PB1 10% inklusif dari pendapatan</p>
                </div>
                <span class="text-sm font-semibold text-red-600">{{ formatRupiah(report.tax_payable) }}</span>
              </div>
            </div>
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
              <span class="text-sm font-medium text-gray-700">Modal Pemilik (Aset − Kewajiban)</span>
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
        <h3 class="text-sm font-semibold text-gray-700 mb-3">Detail Komponen</h3>
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 text-sm">
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Pendapatan Transaksi</p>
            <p class="font-semibold text-gray-800">{{ formatRupiah(report.total_revenue) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Pemasukan Kas</p>
            <p class="font-semibold text-gray-800">{{ formatRupiah(report.total_cash_in) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Pengeluaran Kas</p>
            <p class="font-semibold text-red-600">{{ formatRupiah(report.total_expense) }}</p>
          </div>
          <div class="p-3 bg-gray-50 rounded-lg">
            <p class="text-xs text-gray-500 mb-1">Piutang Usaha</p>
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
import { ref, computed, onMounted, watch } from 'vue'
import { salesApi }    from '@/api/sales.js'
import { formatRupiah, todayDateString } from '@/utils/format.js'
import AppCard      from '@/components/ui/AppCard.vue'
import AppTable     from '@/components/ui/AppTable.vue'
import AppAlert     from '@/components/ui/AppAlert.vue'
import AppSpinner   from '@/components/ui/AppSpinner.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const dateFrom = ref(today)
const dateTo   = ref(today)
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; fetchReport() })
const loading  = ref(false)
const errorMsg = ref('')
const report   = ref(null)

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
  loading.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    report.value = await salesApi.getBalanceReport(params)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan neraca.'
  } finally {
    loading.value = false
  }
}

// Download Excel mengikuti filter aktif (rentang tanggal); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const blob = await salesApi.exportBalanceReport({ date_from: dateFrom.value, date_to: dateTo.value })
    const filename = `Laporan-Neraca_${dateFrom.value}_sd_${dateTo.value}.xlsx`

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
