<template>
  <div class="space-y-4 sm:space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="gl-filters flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-4"
        @keydown.enter="fetchReport">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Tanggal</label>
          <DateRangePicker v-model="range" />
        </div>
        <div class="flex flex-col gap-1 sm:min-w-50">
          <label class="text-sm font-medium text-gray-700">Akun</label>
          <select v-model="selectedAccount"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="">Semua Akun</option>
            <option v-for="a in ACCOUNT_OPTIONS" :key="a.value" :value="a.value">{{ a.label }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-1 sm:min-w-45">
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
        <!-- Aksi: 2 kolom di ponsel, sebaris di layar lebar -->
        <div class="grid grid-cols-2 gap-2 sm:flex sm:items-center sm:gap-3">
          <button @click="fetchReport"
            class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
            Tampilkan
          </button>
          <button @click="resetFilters"
            class="px-4 py-2 bg-gray-100 text-gray-600 text-sm font-medium rounded-lg hover:bg-gray-200 transition-colors shadow-sm">
            Reset
          </button>
          <button @click="downloadExcel" :disabled="exporting"
            class="col-span-2 inline-flex items-center justify-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
            <svg v-if="!exporting" class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
            </svg>
            <AppSpinner v-else size="sm" />
            {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
          </button>
        </div>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && report">
      <!-- Ringkasan hanya valid saat semua akun dihitung; kalau difilter ke satu
           akun, angka kas/pendapatan/beban lain memang tidak diambil server. -->
      <div v-if="report.account_all" class="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4">
        <SummaryCard label="Saldo Kas" :value="formatRupiah(report.summary.cash_balance)" icon="revenue" />
        <SummaryCard label="Total Pendapatan" :value="formatRupiah(report.summary.total_revenue)" icon="revenue" />
        <SummaryCard label="Total Beban" :value="formatRupiah(report.summary.total_expense)" icon="payment" />
      </div>

      <!-- Account Cards -->
      <div class="space-y-3 sm:space-y-4">
        <div v-if="report.accounts.length > 1" class="flex items-center justify-between gap-3">
          <p class="text-sm font-medium text-gray-600">{{ report.accounts.length }} akun ditemukan</p>
          <button @click="toggleAll"
            class="shrink-0 text-sm text-emerald-600 hover:text-emerald-700 font-medium flex items-center gap-1">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="allExpanded ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'" />
            </svg>
            {{ allExpanded ? 'Tutup Semua' : 'Buka Semua' }}
          </button>
        </div>

        <AppCard v-for="account in report.accounts" :key="account.code" :padding="false">
          <!-- Account Header -->
          <button
            class="w-full text-left px-4 py-3 sm:px-5 sm:py-4 hover:bg-gray-50 transition-colors rounded-2xl"
            :aria-expanded="!!expandedAccounts[account.code]"
            @click="toggleAccount(account.code)"
          >
            <div class="flex items-start gap-3">
              <!-- Badge kelompok: kolom sendiri di layar lebar, pindah ke baris
                   meta di ponsel supaya nama akun dapat lebar penuh. -->
              <span :class="['hidden md:inline-flex mt-0.5 shrink-0', BADGE_BASE, groupCls(account.group)]">{{ groupLabel(account.group) }}</span>
              <div class="min-w-0 flex-1">
                <p class="text-sm font-semibold text-gray-900 break-words">{{ account.code }} — {{ account.name }}</p>
                <p class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500">
                  <span :class="['md:hidden inline-flex', BADGE_BASE, groupCls(account.group)]">{{ groupLabel(account.group) }}</span>
                  <span>{{ account.entries.length }} entri</span>
                </p>
                <!-- Ponsel: cukup saldo di header; debit/kredit ada di baris Total -->
                <p class="md:hidden mt-1 flex items-baseline gap-2 text-xs text-gray-500">
                  Saldo
                  <span class="text-sm font-bold" :class="balanceClass(account.balance)">{{ formatRupiah(account.balance) }}</span>
                </p>
              </div>
              <!-- min-w tetap supaya ketiga kolom sejajar antar kartu akun -->
              <div class="hidden md:flex items-center gap-4 lg:gap-6 shrink-0">
                <div class="text-right min-w-[116px]">
                  <p class="text-xs text-gray-500">Debit</p>
                  <p class="text-sm font-medium text-gray-800">{{ formatRupiah(account.total_debit) }}</p>
                </div>
                <div class="text-right min-w-[116px]">
                  <p class="text-xs text-gray-500">Kredit</p>
                  <p class="text-sm font-medium text-gray-800">{{ formatRupiah(account.total_credit) }}</p>
                </div>
                <div class="text-right min-w-[124px]">
                  <p class="text-xs text-gray-500">Saldo</p>
                  <p class="text-sm font-bold" :class="balanceClass(account.balance)">{{ formatRupiah(account.balance) }}</p>
                </div>
              </div>
              <svg :class="['w-5 h-5 shrink-0 text-gray-400 transition-transform', expandedAccounts[account.code] ? 'rotate-180' : '']" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </div>
          </button>

          <!-- Entri akun -->
          <div v-if="expandedAccounts[account.code]" class="border-t border-gray-100">
            <template v-if="account.entries.length">
              <!-- Ponsel: daftar kartu -->
              <ul class="sm:hidden divide-y divide-gray-100">
                <li v-for="(entry, i) in paginatedEntries(account)" :key="pageStart(account) + i" class="px-4 py-3">
                  <div class="flex items-baseline justify-between gap-2">
                    <span class="text-[11px] font-medium text-gray-500 shrink-0">{{ formatDateStr(entry.date) }}</span>
                    <span class="text-sm font-semibold text-right" :class="entry.debit > 0 ? 'text-blue-700' : 'text-rose-600'">
                      {{ entry.debit > 0 ? 'D' : 'K' }} {{ formatRupiah(entry.debit > 0 ? entry.debit : entry.credit) }}
                    </span>
                  </div>
                  <p class="mt-0.5 text-sm text-gray-800 break-words">{{ entry.description }}</p>
                  <div class="mt-1 flex items-baseline justify-between gap-2 text-xs">
                    <span class="text-gray-400">Saldo</span>
                    <span class="font-semibold" :class="balanceClass(entry.balance)">{{ formatRupiah(entry.balance) }}</span>
                  </div>
                </li>
              </ul>
              <!-- Ponsel: total akun -->
              <div class="sm:hidden bg-gray-50 border-t border-gray-100 px-4 py-3 space-y-1 text-xs">
                <div class="flex items-baseline justify-between gap-2">
                  <span class="text-gray-500">Total Debit</span>
                  <span class="font-semibold text-blue-700">{{ formatRupiah(account.total_debit) }}</span>
                </div>
                <div class="flex items-baseline justify-between gap-2">
                  <span class="text-gray-500">Total Kredit</span>
                  <span class="font-semibold text-rose-600">{{ formatRupiah(account.total_credit) }}</span>
                </div>
                <div class="flex items-baseline justify-between gap-2">
                  <span class="text-gray-500">Saldo</span>
                  <span class="font-bold" :class="balanceClass(account.balance)">{{ formatRupiah(account.balance) }}</span>
                </div>
              </div>

              <!-- Tablet ke atas: tabel -->
              <div class="hidden sm:block overflow-x-auto">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="bg-gray-50 text-left text-gray-500 text-xs uppercase tracking-wide">
                      <th class="py-2.5 px-4 font-medium">Tanggal</th>
                      <th class="py-2.5 px-4 font-medium">Keterangan</th>
                      <th class="py-2.5 px-4 font-medium text-right">Debit</th>
                      <th class="py-2.5 px-4 font-medium text-right">Kredit</th>
                      <th class="py-2.5 px-4 font-medium text-right">Saldo</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(entry, i) in paginatedEntries(account)" :key="pageStart(account) + i" class="border-b border-gray-50 hover:bg-gray-50/50">
                      <td class="py-2 px-4 text-gray-600 whitespace-nowrap">{{ formatDateStr(entry.date) }}</td>
                      <td class="py-2 px-4 text-gray-800 min-w-[14rem]">{{ entry.description }}</td>
                      <td class="py-2 px-4 text-right font-medium whitespace-nowrap" :class="entry.debit > 0 ? 'text-blue-700' : 'text-gray-300'">{{ entry.debit > 0 ? formatRupiah(entry.debit) : '-' }}</td>
                      <td class="py-2 px-4 text-right font-medium whitespace-nowrap" :class="entry.credit > 0 ? 'text-rose-600' : 'text-gray-300'">{{ entry.credit > 0 ? formatRupiah(entry.credit) : '-' }}</td>
                      <td class="py-2 px-4 text-right font-semibold whitespace-nowrap" :class="balanceClass(entry.balance)">{{ formatRupiah(entry.balance) }}</td>
                    </tr>
                  </tbody>
                  <tfoot>
                    <tr class="bg-gray-50 font-semibold text-sm">
                      <td class="py-2.5 px-4 text-gray-700" colspan="2">Total</td>
                      <td class="py-2.5 px-4 text-right text-blue-700 whitespace-nowrap">{{ formatRupiah(account.total_debit) }}</td>
                      <td class="py-2.5 px-4 text-right text-rose-600 whitespace-nowrap">{{ formatRupiah(account.total_credit) }}</td>
                      <td class="py-2.5 px-4 text-right whitespace-nowrap" :class="balanceClass(account.balance)">{{ formatRupiah(account.balance) }}</td>
                    </tr>
                  </tfoot>
                </table>
              </div>

              <!-- Pagination per akun -->
              <div v-if="pageCount(account) > 1"
                class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between px-4 py-3 border-t border-gray-100">
                <p class="text-xs text-gray-500">
                  Menampilkan {{ pageStart(account) + 1 }}–{{ Math.min(pageStart(account) + ENTRIES_PER_PAGE, account.entries.length) }} dari {{ account.entries.length }} entri
                </p>
                <div class="flex items-center justify-between gap-1 sm:justify-end">
                  <button
                    @click="setAccountPage(account, currentPage(account) - 1)"
                    :disabled="currentPage(account) <= 1"
                    class="px-3 py-1.5 text-xs rounded border border-gray-200 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed"
                  >&laquo; Prev</button>
                  <span class="text-xs text-gray-600 px-2 whitespace-nowrap">{{ currentPage(account) }} / {{ pageCount(account) }}</span>
                  <button
                    @click="setAccountPage(account, currentPage(account) + 1)"
                    :disabled="currentPage(account) >= pageCount(account)"
                    class="px-3 py-1.5 text-xs rounded border border-gray-200 hover:bg-gray-100 disabled:opacity-40 disabled:cursor-not-allowed"
                  >Next &raquo;</button>
                </div>
              </div>
            </template>
            <p v-else class="py-6 px-4 text-center text-sm text-gray-400">Tidak ada entri pada periode ini.</p>
          </div>
        </AppCard>
      </div>

      <!-- Empty state -->
      <AppCard v-if="report.accounts.length === 0">
        <p class="text-sm text-gray-500 text-center py-8">Tidak ada data buku besar pada periode ini.</p>
      </AppCard>
    </template>
  </div>
</template>

<script setup>
import { ref, shallowRef, reactive, onMounted, computed, watch } from 'vue'
import { salesApi } from '@/api/sales.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateStr, todayDateString } from '@/utils/format.js'
import AppCard       from '@/components/ui/AppCard.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppSpinner    from '@/components/ui/AppSpinner.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import SummaryCard   from '@/components/SummaryCard.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const ENTRIES_PER_PAGE = 20

const today        = todayDateString()
const startOfMonth = today.slice(0, 8) + '01'
const defaultRange = () => ({ from: startOfMonth, to: today, label: 'Bulan Ini' })

const range    = ref(defaultRange())
const dateFrom = ref(range.value.from)
const dateTo   = ref(range.value.to)
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; fetchReport() })

const selectedOutlet  = ref('')
const selectedAccount = ref('')
const outletOptions   = ref([])
const loading  = ref(false)
const errorMsg = ref('')
// shallowRef: laporan sebulan bisa puluhan ribu entri. Objeknya selalu
// diganti utuh (tidak pernah dimutasi per-entri), jadi reaktivitas dalam
// tidak diperlukan — dan menghindarinya menghemat proxy untuk tiap entri.
const report = shallowRef(null)

const expandedAccounts = reactive({})
const accountPages     = reactive({})

const ACCOUNT_OPTIONS = [
  { value: '1-100', label: '1-100 Kas & Setara Kas' },
  { value: '1-200', label: '1-200 Piutang Usaha' },
  { value: '1-400', label: '1-400 Aset Tetap (Peralatan)' },
  { value: '1-450', label: '1-450 Projek Berjalan' },
  { value: '2-100', label: '2-100 Hutang Usaha' },
  { value: '2-200', label: '2-200 Hutang Pajak Restoran' },
  { value: '2-300', label: '2-300 Uang Muka Pelanggan' },
  { value: '4-100', label: '4-100 Pendapatan Penjualan' },
  { value: '4-200', label: '4-200 Pendapatan Lainnya' },
  { value: '5-100', label: '5-100 HPP - Bahan Baku' },
  { value: '5-200', label: '5-200 Beban Jasa & Layanan' },
  { value: '5-300', label: '5-300 Beban Operasional' },
  { value: '5-400', label: '5-400 Beban Penyusutan Aset' },
  { value: '6-100', label: '6-100 Beban Pajak Restoran' },
]

const GROUP_MAP = {
  aset:       { label: 'Aset',       cls: 'bg-blue-100 text-blue-700' },
  pendapatan: { label: 'Pendapatan', cls: 'bg-emerald-100 text-emerald-700' },
  beban:      { label: 'Beban',      cls: 'bg-amber-100 text-amber-700' },
  kewajiban:  { label: 'Kewajiban',  cls: 'bg-red-100 text-red-700' },
  ekuitas:    { label: 'Ekuitas',    cls: 'bg-purple-100 text-purple-700' },
}

const BADGE_BASE = 'items-center px-2 py-0.5 rounded text-[11px] font-semibold whitespace-nowrap'

function groupLabel(g) { return GROUP_MAP[g]?.label ?? g }
function groupCls(g) { return GROUP_MAP[g]?.cls ?? 'bg-gray-100 text-gray-700' }
function balanceClass(v) { return v >= 0 ? 'text-emerald-700' : 'text-red-600' }

function toggleAccount(code) {
  expandedAccounts[code] = !expandedAccounts[code]
}

const allExpanded = computed(() => {
  const list = report.value?.accounts
  return !!list?.length && list.every(a => expandedAccounts[a.code])
})

function toggleAll() {
  const expand = !allExpanded.value
  report.value?.accounts?.forEach(a => { expandedAccounts[a.code] = expand })
}

function resetFilters() {
  selectedOutlet.value = ''
  selectedAccount.value = ''
  const d = defaultRange()
  // Mengganti range memicu watcher yang sudah memanggil fetchReport; kalau
  // rentangnya memang sudah default, muat ulang manual agar Reset tetap terasa.
  if (range.value.from !== d.from || range.value.to !== d.to) {
    range.value = d
  } else {
    fetchReport()
  }
}

// ── Pagination per akun ──
function currentPage(account) { return accountPages[account.code] || 1 }
function pageCount(account) { return Math.max(1, Math.ceil(account.entries.length / ENTRIES_PER_PAGE)) }
function pageStart(account) { return (currentPage(account) - 1) * ENTRIES_PER_PAGE }

function setAccountPage(account, page) {
  accountPages[account.code] = Math.min(Math.max(page, 1), pageCount(account))
}

function paginatedEntries(account) {
  const start = pageStart(account)
  return account.entries.slice(start, start + ENTRIES_PER_PAGE)
}

onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    const list = data?.outlets ?? data ?? []
    outletOptions.value = list.map(o => ({ value: o.id, label: o.name }))
  } catch { /* daftar outlet opsional — laporan tetap dimuat */ }
  fetchReport()
})

// Download Excel mengikuti filter aktif (tanggal + outlet + akun); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  let url
  try {
    const blob = await salesApi.exportGeneralLedger(buildParams())

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    let slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    if (selectedAccount.value) slug += `_Akun-${selectedAccount.value}`

    url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `Buku-Besar_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`
    document.body.appendChild(a)
    a.click()
    a.remove()
  } catch {
    errorMsg.value = 'Gagal mengunduh Excel. Coba lagi, atau persempit rentang tanggal.'
  } finally {
    // Safari butuh blob-nya masih hidup saat klik diproses; lepas setelahnya.
    if (url) setTimeout(() => URL.revokeObjectURL(url), 1000)
    exporting.value = false
  }
}

function buildParams() {
  const params = { date_from: dateFrom.value, date_to: dateTo.value }
  if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
  if (selectedAccount.value) params.account = selectedAccount.value
  return params
}

function fetchReport() {
  loading.value = true
  errorMsg.value = ''
  return salesApi.getGeneralLedger(buildParams())
    .then((data) => {
      report.value = data

      // Bangun ulang state buka-tutup & halaman: kunci akun lama dibuang,
      // bukan sekadar di-reset, agar tidak menumpuk antar filter.
      for (const k of Object.keys(expandedAccounts)) delete expandedAccounts[k]
      for (const k of Object.keys(accountPages)) delete accountPages[k]

      // Kalau hasilnya cuma satu akun (mis. sedang difilter), langsung buka.
      const accounts = data?.accounts ?? []
      if (accounts.length === 1) expandedAccounts[accounts[0].code] = true
    })
    .catch((err) => {
      report.value = null
      errorMsg.value = err?.message ?? 'Gagal memuat buku besar.'
    })
    .finally(() => { loading.value = false })
}
</script>

<style scoped>
/* Trigger DateRangePicker default-nya inline-flex; di ponsel dibuat selebar
   kolom filter lain supaya barisnya rata. */
@media (max-width: 639px) {
  .gl-filters :deep(.drp),
  .gl-filters :deep(.drp-trigger) { width: 100%; }
  .gl-filters :deep(.drp-trigger) { justify-content: flex-start; }
  .gl-filters :deep(.drp-chev) { margin-left: auto; }
}
</style>
