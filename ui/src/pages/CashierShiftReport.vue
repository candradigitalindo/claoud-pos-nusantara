<template>
  <div class="space-y-5">
    <div class="flex items-start justify-between gap-3 flex-wrap">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Laporan Shift Kasir</h1>
        <p class="text-sm text-gray-500 mt-0.5">Buka, ganti shift, & tutup kasir — beserta analisa balance kas tiap sesi.</p>
      </div>
      <button @click="downloadExcel" :disabled="exporting"
        class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
        <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
        </svg>
        <AppSpinner v-else size="sm" />
        {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
      </button>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Notif miss / balance -->
    <div v-if="report && report.summary.miss_count > 0" class="notif notif-warn">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      <div>
        <p class="font-bold">{{ report.summary.miss_count }} shift tidak balance (ada selisih kas)</p>
        <p class="text-sm opacity-90">Total selisih <b>{{ formatRupiah(report.summary.total_variance) }}</b> — kurang {{ formatRupiah(Math.abs(report.summary.shortage_total)) }}, lebih {{ formatRupiah(report.summary.overage_total) }}. Periksa shift bertanda merah/kuning di bawah.</p>
      </div>
    </div>
    <div v-else-if="report && report.summary.closed_shifts > 0" class="notif notif-ok">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      <p class="font-bold">Semua {{ report.summary.closed_shifts }} shift tertutup <span class="font-extrabold">balance</span> — tidak ada selisih kas. 🎉</p>
    </div>

    <!-- Filters -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <SearchableSelect v-model="filterStatus" @change="load" class="form-input">
          <option value="">Semua status</option>
          <option value="open">Berjalan</option>
          <option value="closed">Tutup</option>
        </SearchableSelect>
        <DateRangePicker v-model="range" clearable />
      </div>
    </AppCard>

    <!-- Summary -->
    <div v-if="report" class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div class="stat"><span class="stat-l">Total Shift</span><span class="stat-v">{{ report.summary.total_shifts }}</span><span class="stat-s">{{ report.summary.open_shifts }} berjalan</span></div>
      <div class="stat stat-ok"><span class="stat-l">Balance</span><span class="stat-v">{{ report.summary.balanced_count }}</span><span class="stat-s">dari {{ report.summary.closed_shifts }} tutup</span></div>
      <div class="stat" :class="report.summary.miss_count ? 'stat-bad' : ''"><span class="stat-l">Selisih (Miss)</span><span class="stat-v">{{ report.summary.miss_count }}</span><span class="stat-s">shift bermasalah</span></div>
      <div class="stat" :class="report.summary.total_variance < 0 ? 'stat-bad' : (report.summary.total_variance > 0 ? 'stat-warn' : '')"><span class="stat-l">Total Selisih</span><span class="stat-v stat-v-sm">{{ formatRupiah(report.summary.total_variance) }}</span><span class="stat-s">omzet {{ formatRupiah(report.summary.total_sales) }}</span></div>
    </div>

    <!-- List -->
    <AppCard :padding="false">
      <div v-if="loading" class="p-8 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!report || !report.shifts.length" class="p-8 text-center text-sm text-gray-400">Belum ada data shift kasir.</div>
      <template v-else>
        <!-- Mobile cards -->
        <ul class="sm:hidden divide-y divide-gray-100">
          <li v-for="sh in report.shifts" :key="sh.id" class="p-4 space-y-2" @click="openDetail(sh)">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900">{{ sh.opened_by }}<span v-if="sh.handover_to" class="text-gray-400 font-normal"> → {{ sh.handover_to }}</span></p>
                <p class="text-xs text-gray-500">{{ sh.outlet_name }} · {{ sh.status === 'open' ? 'Berjalan' : 'Tutup' }}</p>
              </div>
              <span class="vbadge shrink-0" :class="varCls(sh)">{{ varLabel(sh) }}</span>
            </div>
            <p class="text-xs text-gray-500">🟢 Buka {{ sh.opened_at || '—' }} · Kas awal {{ formatRupiah(sh.opening_cash) }}</p>
            <p class="text-xs text-gray-500" v-if="sh.status==='closed'">🔴 Tutup {{ sh.closed_at || '—' }} · Kas akhir {{ formatRupiah(sh.closing_cash) }}</p>
            <p class="text-xs text-gray-600">Penjualan {{ formatRupiah(sh.sales_total) }} ({{ sh.sales_count }} trx)<span v-if="sh.sales_source === 'cloud'" class="text-sky-500"> ☁</span> · Kas seharusnya {{ formatRupiah(sh.expected_cash) }}</p>
            <p v-if="sh.cash_in || sh.cash_out" class="text-xs text-gray-500">↑ Masuk {{ formatRupiah(sh.cash_in) }} · ↓ Keluar {{ formatRupiah(sh.cash_out) }}</p>
          </li>
        </ul>
        <!-- Desktop table -->
        <div class="hidden sm:block overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-gray-500">
                <th class="py-2.5 px-3 font-medium">Kasir</th>
                <th class="py-2.5 px-3 font-medium">Outlet</th>
                <th class="py-2.5 px-3 font-medium">Buka</th>
                <th class="py-2.5 px-3 font-medium">Tutup</th>
                <th class="py-2.5 px-3 font-medium text-right">Kas Awal</th>
                <th class="py-2.5 px-3 font-medium text-right">Penjualan</th>
                <th class="py-2.5 px-3 font-medium text-right">Kas Seharusnya</th>
                <th class="py-2.5 px-3 font-medium text-right">Kas Akhir</th>
                <th class="py-2.5 px-3 font-medium text-right">Selisih</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="sh in report.shifts" :key="sh.id" class="border-b border-gray-50 hover:bg-gray-50 cursor-pointer" @click="openDetail(sh)">
                <td class="py-2.5 px-3 font-medium text-gray-900">{{ sh.opened_by }}<span v-if="sh.handover_to" class="text-gray-400 font-normal"> → {{ sh.handover_to }}</span></td>
                <td class="py-2.5 px-3 text-gray-600">{{ sh.outlet_name }}</td>
                <td class="py-2.5 px-3 text-gray-500 text-xs">{{ sh.opened_at || '—' }}</td>
                <td class="py-2.5 px-3 text-xs"><span v-if="sh.status==='open'" class="text-emerald-600 font-medium">Berjalan</span><span v-else class="text-gray-500">{{ sh.closed_at || '—' }}</span></td>
                <td class="py-2.5 px-3 text-right">{{ formatRupiah(sh.opening_cash) }}</td>
                <td class="py-2.5 px-3 text-right">
                  {{ formatRupiah(sh.sales_total) }}
                  <span v-if="sh.sales_source === 'cloud'" class="text-sky-500 cursor-help" title="Dihitung dari transaksi yang tersinkron ke cloud — laporan tutup kasir dari device belum diterima">☁</span>
                </td>
                <td class="py-2.5 px-3 text-right">{{ formatRupiah(sh.expected_cash) }}</td>
                <td class="py-2.5 px-3 text-right">{{ sh.status==='closed' ? formatRupiah(sh.closing_cash) : '—' }}</td>
                <td class="py-2.5 px-3 text-right"><span class="vbadge" :class="varCls(sh)">{{ varLabel(sh) }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </AppCard>

    <!-- Detail modal -->
    <AppModal v-model="detailOpen" :title="`Detail Shift — ${active?.opened_by || ''}`" size="xl">
      <div v-if="active" class="modal-form-grid">
        <div class="modal-span-full flex items-center justify-between flex-wrap gap-2 text-xs text-gray-500">
          <span>{{ active.outlet_name }} · {{ active.status === 'open' ? 'Berjalan' : 'Tutup' }}</span>
          <span class="vbadge" :class="varCls(active)">{{ varLabel(active) }}</span>
        </div>

        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="kv"><span>Buka</span><b>{{ active.opened_at || '—' }}</b></div>
          <div class="kv"><span>Tutup</span><b>{{ active.closed_at || '—' }}</b></div>
          <div class="kv"><span>Dibuka oleh</span><b>{{ active.opened_by }}</b></div>
          <div class="kv"><span>Ditutup oleh</span><b>{{ active.closed_by || '—' }}</b></div>
          <div v-if="active.handover_to" class="kv col-span-2"><span>Serah terima ke</span><b>{{ active.handover_to }} (bawa {{ formatRupiah(active.carry_over_cash) }})</b></div>
        </div>

        <!-- Cash reconciliation -->
        <div class="rounded-xl border border-gray-200 overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600">Rekonsiliasi Kas</div>
          <div class="p-3 space-y-1.5 text-sm">
            <div class="flex justify-between"><span class="text-gray-500">Kas awal</span><span>{{ formatRupiah(active.opening_cash) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">+ Penjualan tunai</span><span>{{ formatRupiah(active.cash_sales) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">+ Kas masuk</span><span>{{ formatRupiah(active.cash_in) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">− Kas keluar</span><span>{{ formatRupiah(active.cash_out) }}</span></div>
            <div class="flex justify-between border-t border-gray-200 pt-1.5"><span class="font-semibold">Kas seharusnya</span><span class="font-semibold">{{ formatRupiah(active.expected_cash) }}</span></div>
            <div class="flex justify-between"><span class="text-gray-500">Kas akhir (aktual)</span><span>{{ active.status==='closed' ? formatRupiah(active.closing_cash) : '—' }}</span></div>
            <div v-if="active.status==='closed'" class="flex justify-between border-t border-gray-200 pt-1.5">
              <span class="font-bold">Selisih</span>
              <span class="font-bold" :class="active.balanced ? 'text-emerald-600' : (active.variance<0 ? 'text-red-600' : 'text-amber-600')">{{ active.balanced ? 'Balance ✓' : formatRupiah(active.variance) }}</span>
            </div>
          </div>
        </div>

        <!-- Kas masuk/keluar di luar transaksi -->
        <div v-if="active.movements?.length" class="rounded-xl border border-gray-200 overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600">Kas Masuk & Keluar (di luar transaksi)</div>
          <ul class="divide-y divide-gray-50">
            <li v-for="(m, i) in active.movements" :key="i" class="flex items-center justify-between gap-2 px-3 py-2 text-sm">
              <div class="min-w-0">
                <p class="font-medium" :class="m.type === 'in' ? 'text-emerald-700' : 'text-red-600'">{{ m.type === 'in' ? 'Masuk' : 'Keluar' }}<span class="text-gray-400 font-normal"> · {{ m.counterpart_name || '—' }}</span></p>
                <p v-if="m.note" class="text-xs text-gray-400 break-words">{{ m.note }}</p>
              </div>
              <span class="font-semibold shrink-0" :class="m.type === 'in' ? 'text-emerald-700' : 'text-red-600'">{{ m.type === 'in' ? '+' : '−' }}{{ formatRupiah(m.amount) }}</span>
            </li>
          </ul>
        </div>

        <!-- Per method -->
        <div class="rounded-xl border border-gray-200 overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600 flex items-center justify-between gap-2">
            <span>Penjualan per Metode ({{ active.sales_count }} trx · {{ formatRupiah(active.sales_total) }})</span>
            <span v-if="active.sales_source === 'cloud'" class="font-normal normal-case text-sky-600">☁ dari transaksi cloud</span>
          </div>
          <div class="p-3 grid grid-cols-1 gap-2 text-sm">
            <div v-for="m in ['cash','qris','card','transfer']" :key="m" class="flex justify-between">
              <span class="text-gray-500 capitalize">{{ m }}</span>
              <span>{{ formatRupiah(active.by_method?.[m]?.total || 0) }} <span class="text-xs text-gray-400">({{ active.by_method?.[m]?.count || 0 }})</span></span>
            </div>
          </div>
        </div>
        <p v-if="active.sales_source === 'cloud'" class="modal-span-full text-xs text-sky-600">☁ Penjualan dihitung dari transaksi yang tersinkron ke cloud karena laporan tutup kasir dari perangkat belum/tidak diterima. Angka bisa lebih kecil dari struk bila ada transaksi yang belum tersinkron.</p>
        <p v-if="active.notes" class="modal-span-full text-xs text-gray-500">Catatan: {{ active.notes }}</p>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, onMounted, watch } from 'vue'
import { cashierShiftsApi } from '@/api/cashierShifts.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import { useRealtime } from '@/utils/realtime.js'

const report = ref(null)
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterOutlet = ref('')
const filterStatus = ref('')
// Default "Hari Ini" (timezone aplikasi) — konsisten dengan halaman laporan lain.
const APP_TZ = localStorage.getItem('cloud_pos_timezone') || 'Asia/Jakarta'
const _today = new Date().toLocaleDateString('en-CA', { timeZone: APP_TZ })
const dateFrom = ref(_today)
const dateTo = ref(_today)
const range = ref({ from: _today, to: _today, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; load() })
// Auto-update: shift buka/tutup, kas masuk/keluar, transaksi baru (omzet ☁ live).
// silent — angka berubah di tempat tanpa spinner.
useRealtime(['cashier_shift', 'cash_movement', 'transaction'], () => load(true))

const detailOpen = ref(false)
const active = ref(null)

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])

// Shift "open" lebih dari 18 jam hampir pasti sudah ditutup di device tapi
// sinkronisasi tutupnya belum sampai (device offline) — beri label berbeda
// supaya tidak terbaca seolah kasir masih berjalan.
function isStale(sh) {
  if (sh.status === 'closed' || !sh.opened_at) return false
  const opened = new Date(sh.opened_at.replace(' ', 'T'))
  return !isNaN(opened) && (Date.now() - opened.getTime()) > 18 * 3600 * 1000
}
function varCls(sh) {
  if (sh.status !== 'closed') return isStale(sh) ? 'v-warn' : 'v-open'
  if (sh.balanced) return 'v-ok'
  return sh.variance < 0 ? 'v-bad' : 'v-warn'
}
function varLabel(sh) {
  if (sh.status !== 'closed') return isStale(sh) ? 'Belum Sinkron' : 'Berjalan'
  if (sh.balanced) return 'Balance ✓'
  return (sh.variance < 0 ? 'Kurang ' : 'Lebih ') + formatRupiah(Math.abs(sh.variance))
}

function openDetail(sh) { active.value = sh; detailOpen.value = true }

async function load(silent = false) {
  if (silent !== true) { loading.value = true }
  errorMsg.value = ''
  try {
    const d = await cashierShiftsApi.getReport({
      outlet_id: filterOutlet.value || undefined,
      status: filterStatus.value || undefined,
      date_from: dateFrom.value || undefined,
      date_to: dateTo.value || undefined,
    })
    report.value = d?.data ?? d
  } catch (e) { if (silent !== true) errorMsg.value = e?.message || 'Gagal memuat laporan' } finally { loading.value = false }
}
// Download Excel mengikuti filter aktif (outlet + status + tanggal); scope role
// tetap dipaksakan di server, jadi user hanya menerima data outlet miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const blob = await cashierShiftsApi.exportReport({
      outlet_id: filterOutlet.value || undefined,
      status: filterStatus.value || undefined,
      date_from: dateFrom.value || undefined,
      date_to: dateTo.value || undefined,
    })

    const outletLabel = filterOutlet.value
      ? (outlets.value.find(o => o.id === filterOutlet.value)?.name || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Shift-Kasir_${slug}_${dateFrom.value || 'awal'}_sd_${dateTo.value || 'akhir'}.xlsx`

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

async function loadOutlets() {
  try { const d = await outletsApi.myOutlets(); outlets.value = d?.outlets ?? d ?? [] } catch { outlets.value = [] }
}

onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }

.notif { display: flex; gap: .7rem; align-items: flex-start; padding: .85rem 1rem; border-radius: .9rem; }
.notif-warn { background: rgba(245,158,11,.12); color: #92400e; border: 1px solid rgba(245,158,11,.3); }
.notif-ok { background: rgba(16,185,129,.1); color: #065f46; border: 1px solid rgba(16,185,129,.25); }

.stat { background: #fff; border: 1px solid rgba(0,0,0,.07); border-radius: 1rem; padding: .85rem 1rem; display: flex; flex-direction: column; gap: .1rem; box-shadow: 0 1px 2px rgba(0,0,0,.04); }
.stat-l { font-size: .68rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.stat-v { font-size: 1.5rem; font-weight: 800; color: #111827; line-height: 1.1; }
.stat-v-sm { font-size: 1.05rem; }
.stat-s { font-size: .7rem; color: #9ca3af; }
.stat-ok { background: rgba(16,185,129,.06); border-color: rgba(16,185,129,.25); }
.stat-ok .stat-v { color: #047857; }
.stat-bad { background: rgba(239,68,68,.05); border-color: rgba(239,68,68,.25); }
.stat-bad .stat-v { color: #b91c1c; }
.stat-warn { background: rgba(245,158,11,.06); border-color: rgba(245,158,11,.3); }
.stat-warn .stat-v { color: #b45309; }

.vbadge { display: inline-block; padding: .15rem .55rem; border-radius: 999px; font-size: .7rem; font-weight: 700; white-space: nowrap; }
.v-ok { background: rgba(16,185,129,.14); color: #047857; }
.v-bad { background: rgba(239,68,68,.13); color: #b91c1c; }
.v-warn { background: rgba(245,158,11,.16); color: #b45309; }
.v-open { background: rgba(107,114,128,.13); color: #4b5563; }

.kv { display: flex; flex-direction: column; }
.kv span { font-size: .68rem; color: #9ca3af; text-transform: uppercase; letter-spacing: .03em; }
.kv b { color: #111827; }
.col-span-2 { grid-column: span 2; }
</style>
