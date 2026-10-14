<!--
  ProcurementDashboard.vue — Outlet Procurement Dashboard
  Sub-page of OutletDetail.vue
-->
<template>
  <div class="pd-root">

    <!-- ── Header ── -->
    <div class="pd-header">
      <div>
        <h2 class="pd-title">Dashboard Pengadaan</h2>
        <p class="pd-sub">Ringkasan pengadaan unit bisnis ini</p>
      </div>
      <button class="pd-refresh" @click="fetchData" :disabled="loading">
        <svg :class="['pd-refresh-ico', loading ? 'spin' : '']" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
          <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
        </svg>
        <span v-if="!loading">Refresh</span>
        <span v-else>Memuat…</span>
      </button>
    </div>

    <!-- ── Loading skeleton ── -->
    <div v-if="loading && !data" class="pd-skeleton-grid">
      <div v-for="i in 4" :key="i" class="pd-skel-card"><div class="skel-bar w60" /><div class="skel-bar w40" /></div>
    </div>

    <!-- ── Error ── -->
    <div v-else-if="errorMsg && !data" class="pd-error">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
      <span>{{ errorMsg }}</span>
    </div>

    <template v-if="data">
      <!-- ── Stat cards ── -->
      <div class="stat-grid">
        <div v-for="card in statCards" :key="card.label" :class="['stat-card', `sc-${card.theme}`]">
          <div class="sc-glare" />
          <div class="sc-blob" />
          <div class="sc-top">
            <div class="sc-icon-wrap" v-html="card.svg" />
            <span class="sc-label">{{ card.label }}</span>
          </div>
          <div class="sc-value">{{ card.value }}</div>
          <div v-if="card.sub" class="sc-sub">{{ card.sub }}</div>
        </div>
      </div>

      <!-- ── Charts row ── -->
      <div class="chart-row">
        <!-- Doughnut: Status Breakdown -->
        <div class="chart-panel">
          <div class="cp-header">
            <div class="cp-icon">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg>
            </div>
            <h3 class="cp-title">Status Pengadaan</h3>
          </div>
          <div class="chart-wrap chart-doughnut-wrap">
            <Doughnut :data="statusChartData" :options="doughnutOptions" />
          </div>
          <!-- Legend -->
          <div class="chart-legend">
            <div v-for="(item, i) in statusLegend" :key="i" class="legend-item">
              <span class="legend-dot" :style="{ background: item.color }" />
              <span class="legend-label">{{ item.label }}</span>
              <span class="legend-val">{{ item.value }}</span>
            </div>
          </div>
        </div>

        <!-- Doughnut: Jenis Pengadaan -->
        <div class="chart-panel">
          <div class="cp-header">
            <div class="cp-icon cp-icon--amber">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="1" y="3" width="15" height="13"/><polygon points="16 8 20 8 23 11 23 16 16 16 16 8"/><circle cx="5.5" cy="18.5" r="2.5"/><circle cx="18.5" cy="18.5" r="2.5"/></svg>
            </div>
            <h3 class="cp-title">Jenis Pengadaan</h3>
          </div>
          <div class="chart-wrap chart-doughnut-wrap">
            <Doughnut :data="typeChartData" :options="doughnutOptions" />
          </div>
          <div class="chart-legend">
            <div class="legend-item">
              <span class="legend-dot" style="background: #3b82f6" />
              <span class="legend-label">Barang</span>
              <span class="legend-val">{{ data.type_split.barang }} ({{ formatRupiah(data.type_split.barang_total) }})</span>
            </div>
            <div class="legend-item">
              <span class="legend-dot" style="background: #f59e0b" />
              <span class="legend-label">Bahan Baku</span>
              <span class="legend-val">{{ data.type_split.bahan || 0 }} ({{ formatRupiah(data.type_split.bahan_total || 0) }})</span>
            </div>
            <div class="legend-item">
              <span class="legend-dot" style="background: #8b5cf6" />
              <span class="legend-label">Jasa</span>
              <span class="legend-val">{{ data.type_split.jasa }} ({{ formatRupiah(data.type_split.jasa_total) }})</span>
            </div>
          </div>
        </div>
      </div>

      <!-- ── Monthly Trend Bar Chart ── -->
      <div class="chart-panel chart-panel--wide">
        <div class="cp-header">
          <div class="cp-icon cp-icon--violet">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
          </div>
          <h3 class="cp-title">Tren Pengadaan (12 Bulan)</h3>
        </div>
        <div class="chart-wrap chart-line-wrap">
          <Bar v-if="trendHasData" :data="trendChartData" :options="trendOptions" />
          <span v-else class="chart-empty">Belum ada data tren</span>
        </div>
        <div v-if="trendHasData" class="trend-summary">
          <div class="ts-item">
            <span class="ts-dot ts-dot--blue" />
            <span class="ts-label">Total Pengadaan</span>
            <span class="ts-val">{{ trendTotalCount }} pengadaan</span>
          </div>
          <div class="ts-item">
            <span class="ts-dot ts-dot--green" />
            <span class="ts-label">Total Nominal</span>
            <span class="ts-val">{{ formatRupiah(trendTotalAmount) }}</span>
          </div>
          <div class="ts-item">
            <span class="ts-dot ts-dot--violet" />
            <span class="ts-label">Rata-rata / Bulan</span>
            <span class="ts-val">{{ trendAvgCount }} pengadaan · {{ formatRupiah(trendAvgAmount) }}</span>
          </div>
        </div>
      </div>

      <!-- ── Work Unit Breakdown Table ── -->
      <div class="wu-panel">
        <div class="cp-header">
          <div class="cp-icon cp-icon--emerald">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
          </div>
          <h3 class="cp-title">Pengadaan per Unit Kerja</h3>
          <span class="cp-count">{{ data.work_units.length }}</span>
        </div>

        <div v-if="data.work_units.length === 0" class="wu-empty">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity=".4"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
          <span>Belum ada data pengadaan.</span>
        </div>

        <!-- Cards for mobile, table for desktop -->
        <div v-else class="wu-content">
          <!-- Desktop table -->
          <div class="wu-table-wrap">
            <table class="wu-table">
              <thead>
                <tr>
                  <th>Unit Kerja</th>
                  <th class="text-right">Nominal</th>
                  <th class="text-center">Total</th>
                  <th class="text-center">Menunggu Verifikasi</th>
                  <th v-if="wuHasVerified" class="text-center">Menunggu Persetujuan</th>
                  <th class="text-center">Disetujui</th>
                  <th class="text-center">Dibayar</th>
                  <th class="text-center">Diterima</th>
                  <th class="text-center">Ditolak</th>
                  <th class="text-center">Dibatalkan</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="wu in data.work_units" :key="wu.work_unit_id">
                  <td class="wu-name-cell">
                    <div class="wu-avatar">{{ wu.work_unit_name.charAt(0).toUpperCase() }}</div>
                    <span>{{ wu.work_unit_name }}</span>
                  </td>
                  <td class="text-right font-semibold">{{ formatRupiah(wu.total_amount) }}</td>
                  <td class="text-center">{{ wu.total }}</td>
                  <td class="text-center"><span v-if="wu.pending" class="badge badge-pending">{{ wu.pending }}</span><span v-else class="text-gray-300">0</span></td>
                  <td v-if="wuHasVerified" class="text-center"><span v-if="wu.verified" class="badge badge-verified">{{ wu.verified }}</span><span v-else class="text-gray-300">0</span></td>
                  <td class="text-center"><span v-if="wu.approved" class="badge badge-approved">{{ wu.approved }}</span><span v-else class="text-gray-300">0</span></td>
                  <td class="text-center"><span v-if="wu.paid" class="badge badge-paid">{{ wu.paid }}</span><span v-else class="text-gray-300">0</span></td>
                  <td class="text-center"><span v-if="wu.received" class="badge badge-received">{{ wu.received }}</span><span v-else class="text-gray-300">0</span></td>
                  <td class="text-center"><span v-if="wu.rejected" class="badge badge-rejected">{{ wu.rejected }}</span><span v-else class="text-gray-300">0</span></td>
                  <td class="text-center"><span v-if="wu.cancelled" class="badge badge-cancelled">{{ wu.cancelled }}</span><span v-else class="text-gray-300">0</span></td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Mobile cards -->
          <div class="wu-mobile-cards">
            <div v-for="wu in data.work_units" :key="wu.work_unit_id" class="wu-mobile-card">
              <div class="wmc-header">
                <div class="wu-avatar">{{ wu.work_unit_name.charAt(0).toUpperCase() }}</div>
                <div class="wmc-info">
                  <span class="wmc-name">{{ wu.work_unit_name }}</span>
                  <span class="wmc-amount">{{ formatRupiah(wu.total_amount) }}</span>
                </div>
                <span class="wmc-total">{{ wu.total }} pengadaan</span>
              </div>
              <div class="wmc-badges">
                <span v-if="wu.pending" class="badge badge-pending">Menunggu Verifikasi {{ wu.pending }}</span>
                <span v-if="wu.verified" class="badge badge-verified">Menunggu Persetujuan {{ wu.verified }}</span>
                <span v-if="wu.approved" class="badge badge-approved">Disetujui {{ wu.approved }}</span>
                <span v-if="wu.paid" class="badge badge-paid">Dibayar {{ wu.paid }}</span>
                <span v-if="wu.received" class="badge badge-received">Diterima {{ wu.received }}</span>
                <span v-if="wu.rejected" class="badge badge-rejected">Ditolak {{ wu.rejected }}</span>
                <span v-if="wu.cancelled" class="badge badge-cancelled">Dibatalkan {{ wu.cancelled }}</span>
              </div>
              <!-- Mini bar -->
              <div class="wmc-bar">
                <div class="wmc-bar-seg wmc-seg-received" :style="{ width: pct(wu.received, wu.total) }" />
                <div class="wmc-bar-seg wmc-seg-paid" :style="{ width: pct(wu.paid, wu.total) }" />
                <div class="wmc-bar-seg wmc-seg-approved" :style="{ width: pct(wu.approved, wu.total) }" />
                <div class="wmc-bar-seg wmc-seg-verified" :style="{ width: pct(wu.verified || 0, wu.total) }" />
                <div class="wmc-bar-seg wmc-seg-pending" :style="{ width: pct(wu.pending, wu.total) }" />
                <div class="wmc-bar-seg wmc-seg-rejected" :style="{ width: pct(wu.rejected + wu.cancelled, wu.total) }" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Doughnut, Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  ArcElement, BarElement, LineElement, LineController, PointElement, CategoryScale, LinearScale,
  Tooltip, Legend, Filler
} from 'chart.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah } from '@/utils/format.js'

// Grafik tren = Bar dengan dataset bertipe 'line' (grafik campuran): komponen
// Bar hanya mendaftarkan BarController, jadi LineController wajib didaftarkan
// di sini — tanpanya Chart.js melempar "line" is not a registered controller
// dan grafik tren kosong (lihat juga VendorDetail.vue).
ChartJS.register(ArcElement, BarElement, LineElement, LineController, PointElement, CategoryScale, LinearScale, Tooltip, Legend, Filler)

const props = defineProps({
  outlet: { type: Object, default: null },
  outletId: { type: String, required: true },
})

const data     = ref(null)
const loading  = ref(false)
const errorMsg = ref('')

onMounted(fetchData)

async function fetchData() {
  loading.value  = true
  errorMsg.value = ''
  try {
    data.value = await outletsApi.getProcurementDashboard(props.outletId)
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat dashboard pengadaan.'
  } finally {
    loading.value = false
  }
}

// ── Stat cards ──
const statCards = computed(() => {
  if (!data.value) return []
  const d = data.value
  return [
    {
      label: 'Total Pengadaan', theme: 'blue',
      value: d.total_requests.toLocaleString('id-ID'),
      sub: `${d.status_breakdown.received + d.status_breakdown.paid} selesai/dibayar`,
      svg: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`,
    },
    {
      label: 'Nominal Pengadaan', theme: 'emerald',
      value: formatRupiah(d.total_amount),
      sub: null,
      svg: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2" y="7" width="20" height="14" rx="2"/><path d="M16 3H8"/><path d="M12 3v4"/><circle cx="12" cy="14" r="2.5"/></svg>`,
    },
    {
      label: 'Menunggu Verifikasi', theme: 'amber',
      value: (d.status_breakdown.pending || 0).toLocaleString('id-ID'),
      sub: 'Belum diperiksa verifikator',
      svg: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`,
    },
    {
      label: 'Menunggu Persetujuan', theme: 'indigo',
      value: (d.status_breakdown.verified || 0).toLocaleString('id-ID'),
      sub: 'Sudah diverifikasi, menunggu approval',
      svg: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/></svg>`,
    },
    {
      label: 'Dibatalkan / Ditolak', theme: 'red',
      value: (d.status_breakdown.rejected + d.status_breakdown.cancelled).toLocaleString('id-ID'),
      sub: `${d.status_breakdown.rejected} ditolak, ${d.status_breakdown.cancelled} dibatalkan`,
      svg: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>`,
    },
  ]
})

// ── Status chart ──
const STATUS_COLORS = {
  pending: '#f59e0b', verified: '#6366f1', approved: '#3b82f6', payment_requested: '#f97316', partial: '#d97706',
  paid: '#8b5cf6', received: '#10b981', rejected: '#ef4444', cancelled: '#6b7280',
}
const STATUS_LABELS = {
  pending: 'Menunggu Verifikasi', verified: 'Menunggu Persetujuan', approved: 'Disetujui', payment_requested: 'Menunggu Bayar', partial: 'Dibayar Sebagian',
  paid: 'Dibayar', received: 'Diterima', rejected: 'Ditolak', cancelled: 'Dibatalkan',
}
// Kolom "Menunggu Persetujuan" per unit kerja hanya bila server mengirim
// hitungannya (field `verified` di baris unit kerja).
const wuHasVerified = computed(() => (data.value?.work_units || []).some(wu => wu && 'verified' in wu))

const statusLegend = computed(() => {
  if (!data.value) return []
  const sb = data.value.status_breakdown
  return Object.keys(STATUS_LABELS).map(k => ({
    label: STATUS_LABELS[k], value: sb[k], color: STATUS_COLORS[k],
  })).filter(i => i.value > 0)
})

const statusChartData = computed(() => {
  if (!data.value) return { labels: [], datasets: [] }
  const sb = data.value.status_breakdown
  const keys = Object.keys(STATUS_LABELS).filter(k => sb[k] > 0)
  return {
    labels: keys.map(k => STATUS_LABELS[k]),
    datasets: [{
      data: keys.map(k => sb[k]),
      backgroundColor: keys.map(k => STATUS_COLORS[k]),
      borderWidth: 0,
      hoverOffset: 6,
    }],
  }
})

const typeChartData = computed(() => {
  if (!data.value) return { labels: [], datasets: [] }
  const ts = data.value.type_split
  return {
    labels: ['Barang', 'Bahan Baku', 'Jasa'],
    datasets: [{
      data: [ts.barang, ts.bahan || 0, ts.jasa],
      backgroundColor: ['#3b82f6', '#f59e0b', '#8b5cf6'],
      borderWidth: 0,
      hoverOffset: 6,
    }],
  }
})

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '62%',
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: 'rgba(15,45,29,.92)',
      titleFont: { size: 12, weight: 600 },
      bodyFont: { size: 11 },
      padding: 10,
      cornerRadius: 8,
    },
  },
}

// ── Monthly trend ──
const MONTH_NAMES = ['Jan','Feb','Mar','Apr','Mei','Jun','Jul','Agu','Sep','Okt','Nov','Des']

const trendHasData = computed(() => data.value?.monthly_trend?.length > 0)

const trendChartData = computed(() => {
  if (!data.value) return { labels: [], datasets: [] }
  const trend = data.value.monthly_trend

  // Build lookup and generate full 12 months
  const map = {}
  trend.forEach(t => { map[t.month] = t })
  const now = new Date()
  const months = []
  for (let i = 11; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    months.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`)
  }

  const labels = months.map(m => {
    const [y, mo] = m.split('-')
    return `${MONTH_NAMES[parseInt(mo, 10) - 1]} '${y.slice(2)}`
  })
  const counts  = months.map(m => map[m]?.count ?? 0)
  const amounts = months.map(m => map[m]?.total_amount ?? 0)

  return {
    labels,
    datasets: [
      {
        type: 'bar',
        label: 'Jumlah Pengadaan',
        data: counts,
        backgroundColor: 'rgba(59,130,246,.65)',
        hoverBackgroundColor: 'rgba(59,130,246,.85)',
        borderRadius: 6,
        borderSkipped: false,
        barPercentage: 0.55,
        categoryPercentage: 0.7,
        yAxisID: 'y',
        order: 2,
      },
      {
        type: 'line',
        label: 'Nominal (Rp)',
        data: amounts,
        borderColor: '#10b981',
        backgroundColor: 'rgba(16,185,129,.08)',
        borderWidth: 2.5,
        tension: 0.4,
        fill: true,
        pointRadius: 5,
        pointHoverRadius: 8,
        pointBackgroundColor: '#fff',
        pointBorderColor: '#10b981',
        pointBorderWidth: 2.5,
        pointHoverBorderWidth: 3,
        yAxisID: 'y1',
        order: 1,
      },
    ],
  }
})

// Trend summary computeds
const trendTotalCount = computed(() => {
  if (!data.value) return 0
  return data.value.monthly_trend.reduce((s, t) => s + t.count, 0)
})
const trendTotalAmount = computed(() => {
  if (!data.value) return 0
  return data.value.monthly_trend.reduce((s, t) => s + t.total_amount, 0)
})
const trendAvgCount = computed(() => {
  const n = data.value?.monthly_trend?.length || 1
  return Math.round((trendTotalCount.value / n) * 10) / 10
})
const trendAvgAmount = computed(() => {
  const n = data.value?.monthly_trend?.length || 1
  return Math.round(trendTotalAmount.value / n)
})

const trendOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: {
      position: 'top',
      align: 'end',
      labels: { usePointStyle: true, font: { size: 11, weight: 600 }, padding: 16, color: '#3a5c4a' },
    },
    tooltip: {
      backgroundColor: 'rgba(15,45,29,.94)',
      titleFont: { size: 12, weight: 700 },
      bodyFont: { size: 11.5 },
      padding: 12,
      cornerRadius: 10,
      boxPadding: 4,
      callbacks: {
        label: (ctx) => {
          if (ctx.datasetIndex === 1) return ` Nominal: ${new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(ctx.raw)}`
          return ` Jumlah: ${ctx.raw} pengadaan`
        },
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { font: { size: 10.5, weight: 600 }, color: '#6b7f74', padding: 6 },
    },
    y: {
      position: 'left',
      beginAtZero: true,
      grid: { color: 'rgba(0,0,0,.05)', drawTicks: false },
      ticks: { font: { size: 10 }, color: '#3b82f6', stepSize: 1, padding: 8 },
      title: { display: true, text: 'Jumlah', font: { size: 11, weight: 700 }, color: '#3b82f6', padding: 4 },
    },
    y1: {
      position: 'right',
      beginAtZero: true,
      grid: { drawOnChartArea: false },
      ticks: {
        font: { size: 10 }, color: '#10b981', padding: 8,
        callback: (v) => v >= 1_000_000_000 ? (v / 1_000_000_000).toFixed(1) + 'M'
          : v >= 1_000_000 ? (v / 1_000_000).toFixed(0) + 'jt'
          : v >= 1_000 ? (v / 1_000).toFixed(0) + 'rb' : v,
      },
      title: { display: true, text: 'Nominal (Rp)', font: { size: 11, weight: 700 }, color: '#10b981', padding: 4 },
    },
  },
}

function pct(val, total) {
  if (!total) return '0%'
  return Math.round((val / total) * 100) + '%'
}
</script>

<style scoped>
/* ── Root ── */
.pd-root { display: flex; flex-direction: column; gap: 1.25rem; }

/* ── Header ── */
.pd-header {
  display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .6rem;
}
.pd-title { font-size: 1.15rem; font-weight: 800; color: #0f4226; letter-spacing: -.02em; }
.pd-sub { font-size: .78rem; color: #5a7866; margin-top: .1rem; }
.pd-refresh {
  display: inline-flex; align-items: center; gap: .35rem;
  padding: .35rem .85rem; border-radius: .6rem; cursor: pointer;
  font-size: .75rem; font-weight: 600;
  background: rgba(255,255,255,.72); backdrop-filter: blur(12px);
  border: 1px solid rgba(0,0,0,.08); color: #2d5c40;
  transition: all .15s;
}
.pd-refresh:hover { background: rgba(255,255,255,.92); border-color: rgba(45,143,86,.2); }
.pd-refresh:disabled { opacity: .6; cursor: not-allowed; }
.pd-refresh-ico { transition: transform .3s; }
.spin { animation: spin-anim .8s linear infinite; }
@keyframes spin-anim { to { transform: rotate(360deg); } }

/* ── Skeleton ── */
.pd-skeleton-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: .875rem; }
.pd-skel-card {
  padding: 1.2rem; border-radius: 1.1rem;
  background: rgba(255,255,255,.6); border: 1px solid rgba(255,255,255,.5);
  display: flex; flex-direction: column; gap: .6rem;
}
.skel-bar {
  height: 14px; border-radius: 4px;
  background: linear-gradient(90deg, #e5eae7 25%, #f0f4f1 50%, #e5eae7 75%);
  background-size: 200% 100%; animation: shimmer 1.4s infinite;
}
.w60 { width: 60%; height: 12px; }
.w40 { width: 40%; height: 22px; }
@keyframes shimmer { 0%{background-position:200% 0} 100%{background-position:-200% 0} }

/* ── Error ── */
.pd-error {
  display: flex; align-items: center; gap: .5rem;
  padding: .8rem 1.1rem; border-radius: .8rem;
  background: rgba(239,68,68,.08); border: 1px solid rgba(239,68,68,.15);
  color: #dc2626; font-size: .82rem; font-weight: 500;
}

/* ── Stat cards ── */
.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: .875rem; }
.stat-card {
  position: relative; overflow: hidden; border-radius: 1.1rem;
  padding: 1.1rem 1.15rem .9rem;
  background: rgba(255,255,255,.75); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.68);
  box-shadow: 0 2px 14px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
  transition: transform .18s ease, box-shadow .18s ease;
}
.stat-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 28px rgba(0,0,0,.10), 0 1px 0 rgba(255,255,255,.95) inset;
}
.sc-glare {
  position: absolute; inset: 0; pointer-events: none; border-radius: inherit;
  background: linear-gradient(135deg, rgba(255,255,255,.52) 0%, rgba(255,255,255,0) 55%);
}
.sc-blob {
  position: absolute; top: -28px; right: -28px;
  width: 90px; height: 90px; border-radius: 50%;
  filter: blur(28px); opacity: .16; pointer-events: none;
}
.sc-blue    .sc-blob { background: #3b82f6; }
.sc-emerald .sc-blob { background: #10b981; }
.sc-amber   .sc-blob { background: #f59e0b; }
.sc-red     .sc-blob { background: #ef4444; }
.sc-indigo  .sc-blob { background: #6366f1; }

.sc-icon-wrap {
  display: flex; align-items: center; justify-content: center;
  width: 34px; height: 34px; border-radius: .65rem; flex-shrink: 0;
}
.sc-blue    .sc-icon-wrap { background: rgba(59,130,246,.12);  color: #2563eb; }
.sc-emerald .sc-icon-wrap { background: rgba(16,185,129,.13);  color: #059669; }
.sc-amber   .sc-icon-wrap { background: rgba(245,158,11,.13);  color: #d97706; }
.sc-red     .sc-icon-wrap { background: rgba(239,68,68,.12);   color: #dc2626; }
.sc-indigo  .sc-icon-wrap { background: rgba(99,102,241,.12);  color: #4f46e5; }

.sc-top { display: flex; align-items: center; gap: .65rem; margin-bottom: .65rem; }
.sc-label {
  font-size: .68rem; font-weight: 700; text-transform: uppercase;
  letter-spacing: .07em; color: #6b7f74; line-height: 1.3;
}
.sc-value {
  font-size: clamp(1.1rem, 2.8vw, 1.4rem);
  font-weight: 800; color: #0f2d1d;
  letter-spacing: -.035em; line-height: 1;
  word-break: break-word; margin-bottom: .35rem;
}
.sc-sub { font-size: .7rem; color: #6b7f74; font-weight: 500; }

/* ── Chart panels ── */
.chart-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: .875rem; }
.chart-panel {
  border-radius: 1.1rem; overflow: hidden;
  background: rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.68);
  box-shadow: 0 2px 16px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
}
.chart-panel--wide { grid-column: 1 / -1; }

.cp-header {
  display: flex; align-items: center; gap: .55rem;
  padding: .85rem 1.15rem;
  border-bottom: 1px solid rgba(0,0,0,.06);
  background: linear-gradient(135deg, rgba(255,255,255,.6) 0%, rgba(240,249,244,.4) 100%);
}
.cp-icon {
  display: flex; align-items: center; justify-content: center;
  width: 30px; height: 30px; border-radius: .55rem;
  background: rgba(59,130,246,.12); color: #2563eb;
}
.cp-icon--amber { background: rgba(245,158,11,.12); color: #d97706; }
.cp-icon--violet { background: rgba(139,92,246,.12); color: #7c3aed; }
.cp-icon--emerald { background: rgba(16,185,129,.12); color: #059669; }
.cp-title { font-size: .85rem; font-weight: 700; color: #0f3d23; }
.cp-count {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 .4rem;
  border-radius: 999px; background: rgba(16,185,129,.15);
  color: #059669; font-size: .67rem; font-weight: 700;
}

.chart-wrap { padding: 1rem 1.15rem; }
.chart-doughnut-wrap { height: 200px; display: flex; align-items: center; justify-content: center; }
.chart-line-wrap { height: 320px; }

/* ── Trend Summary ── */
.trend-summary {
  display: flex; flex-wrap: wrap; gap: .5rem 1.5rem;
  padding: .75rem 1.15rem 1rem;
  border-top: 1px solid rgba(0,0,0,.05);
}
.ts-item { display: flex; align-items: center; gap: .4rem; font-size: .75rem; }
.ts-dot {
  width: 10px; height: 10px; border-radius: 3px; flex-shrink: 0;
}
.ts-dot--blue   { background: #3b82f6; }
.ts-dot--green  { background: #10b981; }
.ts-dot--violet { background: #8b5cf6; }
.ts-label { color: #6b7f74; font-weight: 500; }
.ts-val { color: #0f3d23; font-weight: 700; }

/* ── Chart legend ── */
.chart-legend {
  display: flex; flex-wrap: wrap; gap: .4rem .85rem;
  padding: 0 1.15rem 1rem;
}
.legend-item { display: flex; align-items: center; gap: .3rem; font-size: .72rem; color: #4a6355; }
.legend-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.legend-label { font-weight: 600; }
.legend-val { color: #6b7f74; }

/* ── Work unit panel ── */
.wu-panel {
  border-radius: 1.1rem; overflow: hidden;
  background: rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid rgba(255,255,255,.68);
  box-shadow: 0 2px 16px rgba(0,0,0,.07), 0 1px 0 rgba(255,255,255,.95) inset;
}
.wu-empty {
  display: flex; flex-direction: column; align-items: center; gap: .5rem;
  padding: 2.5rem 1rem; color: #8a9e93; font-size: .82rem;
}

/* ── Desktop table ── */
.wu-table-wrap { overflow-x: auto; }
.wu-table { width: 100%; border-collapse: collapse; font-size: .78rem; }
.wu-table th {
  padding: .6rem .8rem; text-align: left;
  font-size: .68rem; font-weight: 700; text-transform: uppercase;
  letter-spacing: .05em; color: #6b7f74;
  border-bottom: 1px solid rgba(0,0,0,.08);
  background: rgba(248,252,249,.6);
  white-space: nowrap;
}
.wu-table td {
  padding: .65rem .8rem; border-bottom: 1px solid rgba(0,0,0,.04);
  color: #1a3d2a; white-space: nowrap;
}
.wu-table tbody tr:hover { background: rgba(45,143,86,.03); }
.wu-name-cell { display: flex; align-items: center; gap: .5rem; }
.wu-avatar {
  width: 28px; height: 28px; border-radius: .5rem; flex-shrink: 0;
  background: linear-gradient(135deg, #1a5c38, #0f4226); color: #fff;
  font-size: .72rem; font-weight: 800;
  display: flex; align-items: center; justify-content: center;
}

/* ── Badges ── */
.badge {
  display: inline-flex; align-items: center; padding: .12rem .45rem;
  border-radius: 999px; font-size: .65rem; font-weight: 700;
}
.badge-pending   { background: rgba(245,158,11,.13); color: #b45309; }
.badge-verified  { background: rgba(99,102,241,.13);  color: #4338ca; }
.badge-approved  { background: rgba(59,130,246,.13);  color: #1d4ed8; }
.badge-paid      { background: rgba(139,92,246,.13);  color: #6d28d9; }
.badge-received  { background: rgba(16,185,129,.13);  color: #047857; }
.badge-rejected  { background: rgba(239,68,68,.13);   color: #b91c1c; }
.badge-cancelled { background: rgba(107,114,128,.13); color: #4b5563; }

.text-right { text-align: right; }
.text-center { text-align: center; }
.font-semibold { font-weight: 600; }
.text-gray-300 { color: #d1d5db; }

/* ── Mobile cards (hidden on desktop) ── */
.wu-mobile-cards { display: none; }
.wu-mobile-card {
  padding: .9rem 1.1rem;
  border-bottom: 1px solid rgba(0,0,0,.05);
}
.wu-mobile-card:last-child { border-bottom: none; }
.wmc-header { display: flex; align-items: center; gap: .5rem; margin-bottom: .5rem; }
.wmc-info { flex: 1; min-width: 0; }
.wmc-name { display: block; font-size: .82rem; font-weight: 700; color: #0f3d23; }
.wmc-amount { display: block; font-size: .72rem; color: #059669; font-weight: 600; }
.wmc-total { font-size: .68rem; color: #6b7f74; font-weight: 600; white-space: nowrap; }
.wmc-badges { display: flex; flex-wrap: wrap; gap: .3rem; margin-bottom: .55rem; }

/* Mini progress bar */
.wmc-bar {
  display: flex; height: 6px; border-radius: 3px; overflow: hidden;
  background: rgba(0,0,0,.06);
}
.wmc-bar-seg { height: 100%; transition: width .3s ease; }
.wmc-seg-received { background: #10b981; }
.wmc-seg-paid { background: #8b5cf6; }
.wmc-seg-verified { background: #6366f1; }
.wmc-seg-approved { background: #3b82f6; }
.wmc-seg-pending { background: #f59e0b; }
.wmc-seg-rejected { background: #ef4444; }

/* ── Responsive ── */
@media (max-width: 768px) {
  .stat-grid { grid-template-columns: repeat(2, 1fr); }
  .chart-row { grid-template-columns: 1fr; }
  .chart-line-wrap { height: 240px; }
  .wu-table-wrap { display: none; }
  .wu-mobile-cards { display: block; }
}
@media (max-width: 480px) {
  .stat-grid { grid-template-columns: 1fr; }
  .pd-header { flex-direction: column; align-items: flex-start; }
  .chart-doughnut-wrap { height: 170px; }
  .chart-line-wrap { height: 200px; }
}
</style>
