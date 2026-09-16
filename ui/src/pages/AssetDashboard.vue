<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Dashboard Aset</h1>
        <p class="mt-0.5 text-sm text-gray-500">Nilai, kondisi, dan pekerjaan yang menunggu — dalam satu layar.</p>
      </div>
      <button class="btn-soft w-full sm:w-auto" :disabled="loading" @click="load">
        <span v-html="IC.refresh" class="inline-block h-4 w-4 align-[-2px]"></span>
        {{ loading ? 'Memuat…' : 'Muat Ulang' }}
      </button>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Nilai -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard v-for="c in valueCards" :key="c.label" class="min-w-0">
        <div class="flex items-start gap-2">
          <span v-html="c.icon" class="mt-0.5 h-5 w-5 shrink-0" :class="c.tone"></span>
          <div class="min-w-0">
            <p class="stat-lbl">{{ c.label }}</p>
            <p class="stat-val">{{ c.value }}</p>
            <p class="stat-sub">{{ c.sub }}</p>
          </div>
        </div>
      </AppCard>
    </div>

    <!-- Pekerjaan menunggu -->
    <AppCard>
      <div class="flex items-center gap-2">
        <span v-html="IC.bell" class="h-4 w-4 text-gray-400"></span>
        <h2 class="text-sm font-bold text-gray-900">Perlu Ditindak</h2>
        <span v-if="!totalActions" class="badge-ok ml-auto">Semua beres</span>
      </div>
      <div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        <RouterLink v-for="a in actionCards" :key="a.label" :to="a.to"
          class="rounded-xl border p-3 transition"
          :class="a.count ? 'border-amber-200 bg-amber-50 hover:border-amber-400' : 'border-gray-100 bg-white hover:border-gray-300'">
          <div class="flex items-start gap-2">
            <span v-html="a.icon" class="mt-0.5 h-4 w-4 shrink-0" :class="a.count ? 'text-amber-600' : 'text-gray-300'"></span>
            <div class="min-w-0">
              <p class="text-lg font-bold leading-none" :class="a.count ? 'text-amber-700' : 'text-gray-400'">{{ a.count }}</p>
              <p class="mt-1 text-[11px] leading-tight text-gray-600">{{ a.label }}</p>
            </div>
          </div>
        </RouterLink>
      </div>
    </AppCard>

    <!-- Grafik -->
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <AppCard>
        <div class="flex items-center gap-2">
          <span v-html="IC.layers" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Nilai Buku per Kategori</h2>
        </div>
        <p v-if="!data.by_category?.length" class="mt-4 text-center text-sm text-gray-400">Belum ada data aset.</p>
        <VueApexCharts v-else type="bar" :height="280" :options="categoryOptions" :series="categorySeries" />
      </AppCard>

      <AppCard>
        <div class="flex items-center gap-2">
          <span v-html="IC.wrench" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Biaya Perawatan 12 Bulan</h2>
        </div>
        <p v-if="!hasMaintTrend" class="mt-4 text-center text-sm text-gray-400">Belum ada perawatan selesai.</p>
        <VueApexCharts v-else type="area" :height="280" :options="trendOptions" :series="trendSeries" />
      </AppCard>

      <AppCard>
        <div class="flex items-center gap-2">
          <span v-html="IC.clock" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Umur Aset</h2>
        </div>
        <p v-if="!data.age_buckets?.length" class="mt-4 text-center text-sm text-gray-400">Belum ada data.</p>
        <VueApexCharts v-else type="donut" :height="280" :options="ageOptions" :series="ageSeries" />
      </AppCard>

      <AppCard>
        <div class="flex items-center gap-2">
          <span v-html="IC.shield" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Kondisi & Status</h2>
        </div>
        <div v-if="!data.by_condition?.length" class="mt-4 text-center text-sm text-gray-400">Belum ada data.</div>
        <div v-else class="mt-3 space-y-3">
          <div v-for="grp in [{ t: 'Kondisi fisik', rows: data.by_condition }, { t: 'Status', rows: data.by_status }]" :key="grp.t">
            <p class="text-[11px] font-bold uppercase tracking-wide text-gray-400">{{ grp.t }}</p>
            <div class="mt-1 space-y-1.5">
              <div v-for="b in grp.rows" :key="b.label" class="flex items-center gap-2">
                <span class="w-28 shrink-0 text-xs text-gray-600">{{ labelOf(b.label) }}</span>
                <div class="h-2 flex-1 overflow-hidden rounded-full bg-gray-100">
                  <div class="h-full rounded-full" :class="barTone(b.label)"
                    :style="{ width: barWidth(b.count, grp.rows) + '%' }"></div>
                </div>
                <span class="w-8 shrink-0 text-right text-xs font-semibold text-gray-700">{{ b.count }}</span>
              </div>
            </div>
          </div>
        </div>
      </AppCard>
    </div>

    <!-- Sebaran outlet + kandidat ganti -->
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <AppCard :padding="false">
        <div class="flex items-center gap-2 border-b border-gray-100 p-3">
          <span v-html="IC.store" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Sebaran per Outlet</h2>
        </div>
        <p v-if="!data.by_outlet?.length" class="p-6 text-center text-sm text-gray-400">Belum ada data.</p>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500">
              <tr>
                <th class="p-2 text-left">Outlet</th>
                <th class="p-2 text-right">Aset</th>
                <th class="p-2 text-right">Perolehan</th>
                <th class="p-2 text-right">Nilai Buku</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="o in data.by_outlet" :key="o.label">
                <td class="p-2 text-gray-800">{{ o.label }}</td>
                <td class="p-2 text-right text-gray-600">{{ o.count }} · {{ o.units }} unit</td>
                <td class="p-2 text-right text-gray-700">{{ formatRupiah(o.acquisition) }}</td>
                <td class="p-2 text-right font-medium text-gray-900">{{ formatRupiah(o.book_value) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <AppCard :padding="false">
        <div class="flex items-center gap-2 border-b border-gray-100 p-3">
          <span v-html="IC.alert" class="h-4 w-4 text-gray-400"></span>
          <h2 class="text-sm font-bold text-gray-900">Biaya Rawat Tertinggi</h2>
          <span class="ml-auto text-[11px] text-gray-400">&gt; 50% = pertimbangkan ganti</span>
        </div>
        <p v-if="!data.top_maintenance?.length" class="p-6 text-center text-sm text-gray-400">
          Belum ada aset yang menyerap biaya perawatan.
        </p>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="t in data.top_maintenance" :key="t.asset_id" class="p-3">
            <RouterLink :to="`/perlengkapan/${t.asset_id}`" class="block">
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="break-words text-sm font-medium text-gray-900">{{ t.name }}</p>
                  <p class="font-mono text-[11px] text-gray-400">{{ t.asset_no }} · {{ t.outlet_name }}</p>
                </div>
                <span :class="t.ratio >= 50 ? 'badge-bad' : 'badge-warn'">{{ Math.round(t.ratio) }}%</span>
              </div>
              <p class="mt-1 text-xs text-gray-600">
                rawat {{ formatRupiah(t.maint_cost) }} dari harga {{ formatRupiah(t.acquisition) }}
              </p>
            </RouterLink>
          </li>
        </ul>
      </AppCard>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import VueApexCharts from 'vue3-apexcharts'
import { assetReportsApi } from '@/api/assetOps.js'
import { formatRupiah } from '@/utils/format.js'
import { condLabel, statusLabel } from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'

// Ikon inline SVG bergaya lucide — emoji tampil berbeda di tiap sistem operasi
// dan tidak bisa ikut berubah warna mengikuti keadaan kartunya.
const svg = (inner) =>
  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" class="h-full w-full">${inner}</svg>`
const IC = {
  box: svg('<path d="M21 8v8a2 2 0 01-1 1.73l-7 4a2 2 0 01-2 0l-7-4A2 2 0 013 16V8a2 2 0 011-1.73l7-4a2 2 0 012 0l7 4A2 2 0 0121 8z"/><path d="M3.3 7L12 12l8.7-5M12 22V12"/>'),
  wallet: svg('<path d="M21 12V7H5a2 2 0 010-4h14v4"/><path d="M3 5v14a2 2 0 002 2h16v-5"/><path d="M18 12a2 2 0 000 4h4v-4z"/>'),
  trend: svg('<polyline points="22 7 13.5 15.5 8.5 10.5 2 17"/><polyline points="16 7 22 7 22 13"/>'),
  wrench: svg('<path d="M14.7 6.3a4 4 0 01-5.4 5.4L4 17v3h3l5.3-5.3a4 4 0 015.4-5.4l-2.3 2.3-2-2 2.3-2.3z"/>'),
  bell: svg('<path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.7 21a2 2 0 01-3.4 0"/>'),
  transfer: svg('<polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 014-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 01-4 4H3"/>'),
  trash: svg('<polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/><path d="M10 11v6M14 11v6"/>'),
  clipboard: svg('<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2"/><path d="M9 12h6M9 16h4"/>'),
  handshake: svg('<path d="M11 17l-2 2a1.5 1.5 0 01-2-2l2-2"/><path d="M3 12l4-4 4 3 3-3 7 5-4 4-3-2-3 3z"/>'),
  camera: svg('<path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/><circle cx="12" cy="13" r="4"/>'),
  inbox: svg('<polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.5 5.1L2 12v6a2 2 0 002 2h16a2 2 0 002-2v-6l-3.5-6.9A2 2 0 0016.8 4H7.2a2 2 0 00-1.7 1.1z"/>'),
  clock: svg('<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/>'),
  layers: svg('<polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/>'),
  shield: svg('<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/>'),
  store: svg('<path d="M3 9l1.5-5h15L21 9"/><path d="M4 9v10a1 1 0 001 1h14a1 1 0 001-1V9"/><path d="M3 9h18"/><path d="M9 20v-6h6v6"/>'),
  alert: svg('<path d="M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z"/><path d="M12 9v4M12 17h.01"/>'),
  refresh: svg('<polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.5 9a9 9 0 0114.9-3.4L23 10M1 14l4.6 4.4A9 9 0 0020.5 15"/>'),
}

const data = ref({})
const loading = ref(false)
const errorMsg = ref('')

function asObject(d) { return d?.data ?? d ?? null }
const sum = computed(() => data.value.summary || {})

// Kolom rupiah memakai pembantu format yang sudah dipakai seluruh aplikasi.
const valueCards = computed(() => [
  { label: 'Nilai Perolehan', value: formatRupiah(sum.value.acquisition_cost || 0),
    sub: `${sum.value.asset_count || 0} aset · ${sum.value.unit_count || 0} unit`,
    icon: IC.box, tone: 'text-gray-400' },
  { label: 'Nilai Buku', value: formatRupiah(sum.value.book_value || 0),
    sub: `susut ${formatRupiah(sum.value.depreciation || 0)}`,
    icon: IC.wallet, tone: 'text-emerald-500' },
  { label: 'Biaya Rawat Tahun Ini', value: formatRupiah(sum.value.maintenance_cost_year || 0),
    sub: 'perawatan selesai', icon: IC.wrench, tone: 'text-amber-500' },
  { label: 'Nilai Dihapus', value: formatRupiah(sum.value.disposed_value || 0),
    sub: 'lewat berita acara', icon: IC.trash, tone: 'text-gray-400' },
])

const act = computed(() => data.value.actions || {})
const actionCards = computed(() => [
  { label: 'Perawatan terlambat', count: act.value.maintenance_overdue || 0, to: '/perlengkapan/perawatan', icon: IC.wrench },
  { label: 'Jatuh tempo 7 hari', count: act.value.maintenance_due_week || 0, to: '/perlengkapan/perawatan', icon: IC.clock },
  { label: 'Mutasi menunggu diterima', count: act.value.transfers_to_receive || 0, to: '/perlengkapan/mutasi', icon: IC.transfer },
  { label: 'Mutasi menunggu persetujuan', count: act.value.transfers_to_approve || 0, to: '/perlengkapan/mutasi', icon: IC.transfer },
  { label: 'Penghapusan menunggu', count: act.value.disposals_pending || 0, to: '/perlengkapan/penghapusan', icon: IC.trash },
  { label: 'Opname berjalan', count: act.value.opname_running || 0, to: '/perlengkapan/opname', icon: IC.clipboard },
  { label: 'Belum diserahkan ke PIC', count: act.value.awaiting_handover || 0, to: '/perlengkapan/distribusi', icon: IC.handshake },
  { label: 'Pengadaan belum lengkap', count: act.value.incomplete_receipts || 0, to: '/perlengkapan/laporan', icon: IC.inbox },
  { label: 'Foto belum tercadangkan', count: act.value.photos_pending_backup || 0, to: '/settings/photo-backup', icon: IC.camera },
])
const totalActions = computed(() => actionCards.value.reduce((s, a) => s + a.count, 0))

// ── Grafik ──
const BRAND = ['#4a7c62', '#7eb89a', '#b5d9c5', '#d97706', '#dc2626', '#1d4ed8', '#6b7280']
const rupiahShort = (v) => {
  const n = Number(v) || 0
  if (n >= 1e9) return 'Rp ' + (n / 1e9).toFixed(1) + ' M'
  if (n >= 1e6) return 'Rp ' + (n / 1e6).toFixed(1) + ' jt'
  if (n >= 1e3) return 'Rp ' + Math.round(n / 1e3) + ' rb'
  return 'Rp ' + n
}

const categorySeries = computed(() => [{
  name: 'Nilai buku',
  data: (data.value.by_category || []).map(c => Math.round(c.book_value)),
}])
const categoryOptions = computed(() => ({
  chart: { toolbar: { show: false }, fontFamily: 'inherit' },
  colors: [BRAND[0]],
  plotOptions: { bar: { horizontal: true, borderRadius: 4, barHeight: '65%' } },
  dataLabels: { enabled: false },
  xaxis: { categories: (data.value.by_category || []).map(c => c.label),
           labels: { formatter: rupiahShort, style: { fontSize: '11px' } } },
  yaxis: { labels: { style: { fontSize: '11px' } } },
  grid: { borderColor: '#f1f5f2' },
  tooltip: { y: { formatter: (v) => formatRupiah(v) } },
}))

const hasMaintTrend = computed(() => (data.value.maintenance_trend || []).some(m => m.cost > 0))
const trendSeries = computed(() => [{
  name: 'Biaya perawatan',
  data: (data.value.maintenance_trend || []).map(m => Math.round(m.cost)),
}])
const trendOptions = computed(() => ({
  chart: { toolbar: { show: false }, fontFamily: 'inherit' },
  colors: [BRAND[3]],
  stroke: { curve: 'smooth', width: 2 },
  fill: { type: 'gradient', gradient: { opacityFrom: 0.35, opacityTo: 0.05 } },
  dataLabels: { enabled: false },
  xaxis: { categories: (data.value.maintenance_trend || []).map(m => m.month.slice(2)),
           labels: { style: { fontSize: '11px' } } },
  yaxis: { labels: { formatter: rupiahShort, style: { fontSize: '11px' } } },
  grid: { borderColor: '#f1f5f2' },
  tooltip: { y: { formatter: (v) => formatRupiah(v) } },
}))

const ageSeries = computed(() => (data.value.age_buckets || []).map(b => b.count))
const ageOptions = computed(() => ({
  chart: { fontFamily: 'inherit' },
  labels: (data.value.age_buckets || []).map(b => b.label),
  colors: BRAND,
  legend: { position: 'bottom', fontSize: '11px' },
  dataLabels: { enabled: true, formatter: (_, o) => o.w.config.series[o.seriesIndex] },
  plotOptions: { pie: { donut: { size: '62%' } } },
}))

function labelOf(v) { return condLabel(v) !== v ? condLabel(v) : statusLabel(v) }
function barWidth(count, rows) {
  const max = Math.max(...rows.map(r => r.count), 1)
  return Math.max(4, Math.round((count / max) * 100))
}
function barTone(label) {
  if (['baik', 'aktif'].includes(label)) return 'bg-emerald-500'
  if (['rusak_berat', 'dihapus'].includes(label)) return 'bg-red-500'
  if (['rusak_ringan', 'perbaikan', 'dipinjam'].includes(label)) return 'bg-amber-500'
  if (label === 'transit') return 'bg-blue-500'
  return 'bg-gray-400'
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try { data.value = asObject(await assetReportsApi.dashboard()) || {} }
  catch (e) { errorMsg.value = e?.message || 'Gagal memuat dashboard' }
  finally { loading.value = false }
}

onMounted(load)
</script>

<style scoped>
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .45rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.08); min-height: 40px;
}
.btn-soft:hover:not(:disabled) { background: #f3f4f6; }
.btn-soft:disabled { opacity: .6; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; overflow-wrap: anywhere; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
</style>
