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
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Urutkan</label>
          <SearchableSelect v-model="sortBy" @change="applyFilter"
            class="rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="revenue">Pendapatan</option>
            <option value="qty">Qty terjual</option>
          </SearchableSelect>
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Arah</label>
          <SearchableSelect v-model="sortDir" @change="applyFilter"
            class="rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="desc">Tertinggi</option>
            <option value="asc">Terendah</option>
          </SearchableSelect>
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

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <!-- Summary -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <SummaryCard label="Total Produk" :value="report.total" icon="receipt" />
        <SummaryCard label="Total Qty Terjual" :value="totalQty" icon="avg" />
        <SummaryCard label="Total Pendapatan" :value="formatRupiah(totalRevenue)" icon="revenue" />
      </div>

      <!-- Table -->
      <AppCard :padding="false">
        <div class="px-4 pt-4 pb-2 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-700">Penjualan per Produk</h3>
          <span class="text-xs text-gray-400">{{ formatDateStr(report.date_from) }} s/d {{ formatDateStr(report.date_to) }}</span>
        </div>
        <AppTable :columns="COLUMNS" :rows="report.items" :loading="false" emptyText="Tidak ada data penjualan produk.">
          <template #cell-total_revenue="{ row }">
            {{ formatRupiah(row.total_revenue) }}
          </template>
          <template #cell-pct="{ row }">
            <div class="flex items-center gap-2">
              <div class="flex-1 bg-gray-100 rounded-full h-1.5">
                <div class="bg-emerald-500 h-1.5 rounded-full" :style="{ width: pct(row.total_revenue) + '%' }" />
              </div>
              <span class="text-xs text-gray-500 w-10 text-right">{{ pct(row.total_revenue) }}%</span>
            </div>
          </template>
        </AppTable>
        <div v-if="report.total > PER_PAGE" class="px-4 py-3 border-t border-gray-100">
          <AppPagination v-model="page" :total="report.total" :per-page="PER_PAGE" />
        </div>
      </AppCard>
    </template>
  </div>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, onMounted, watch } from 'vue'
import { salesApi }    from '@/api/sales.js'
import { outletsApi }  from '@/api/outlets.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard      from '@/components/ui/AppCard.vue'
import AppTable     from '@/components/ui/AppTable.vue'
import AppAlert     from '@/components/ui/AppAlert.vue'
import AppSpinner   from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import SummaryCard  from '@/components/SummaryCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const today = todayDateString()
const dateFrom       = ref(today)
const dateTo         = ref(today)
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
const PER_PAGE       = 25
const page           = ref(1)
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; applyFilter() })
watch(page, fetchReport)
const selectedOutlet = ref('')
const sortBy         = ref('revenue') // revenue | qty
const sortDir        = ref('desc')    // desc | asc
const outletOptions  = ref([])
const loading        = ref(false)
const errorMsg       = ref('')
const report         = ref(null)

const COLUMNS = [
  { key: 'product_name',  label: 'Nama Produk' },
  { key: 'outlet_name',   label: 'Outlet' },
  { key: 'category_name', label: 'Kategori' },
  { key: 'total_qty',     label: 'Qty', align: 'right' },
  { key: 'total_revenue', label: 'Pendapatan', align: 'right' },
  { key: 'pct',           label: '% Kontribusi' },
]

// Grand total dari server (mencakup semua halaman, bukan cuma halaman aktif).
const totalQty     = computed(() => report.value?.total_qty ?? 0)
const totalRevenue = computed(() => report.value?.total_revenue ?? 0)

function pct(val) {
  if (!totalRevenue.value) return 0
  return Math.round((val / totalRevenue.value) * 1000) / 10
}

function applyFilter() {
  if (page.value === 1) fetchReport()
  else page.value = 1
}

onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    const list = data.outlets ?? data ?? []
    outletOptions.value = list.map(o => ({ value: o.id, label: o.name }))
  } catch { /* ignore */ }
  fetchReport()
})

// Download Excel mengikuti filter aktif (tanggal + outlet + urutan); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value, sort: sortBy.value, dir: sortDir.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    const blob = await salesApi.exportProductSalesReport(params)

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Penjualan-Produk_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`

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

async function fetchReport() {
  loading.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value, page: page.value, limit: PER_PAGE, sort: sortBy.value, dir: sortDir.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    report.value = await salesApi.getProductSalesReport(params)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat laporan penjualan produk.'
  } finally {
    loading.value = false
  }
}
</script>
