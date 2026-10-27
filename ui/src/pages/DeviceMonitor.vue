<!--
  DeviceMonitor.vue — Pengaturan > Perangkat
  Monitoring kondisi tablet kasir tiap outlet (heartbeat App POS):
  status online, baterai, penyimpanan, printer, jaringan & antrian sync.
  Informatif, detail, mobile-first. Klik kartu → detail + tren histori.
-->
<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex items-start justify-between gap-3 flex-wrap">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Perangkat</h1>
        <p class="text-sm text-gray-500 mt-0.5">Kondisi tablet kasir tiap outlet — baterai, penyimpanan, printer & jaringan.</p>
      </div>
      <div class="flex items-center gap-2">
        <label class="flex items-center gap-1.5 text-xs text-gray-500 select-none cursor-pointer">
          <input type="checkbox" v-model="autoRefresh" class="accent-emerald-600" /> Auto
        </label>
        <button class="refresh-btn" :class="{ spinning: loading }" @click="load" title="Muat ulang">
          <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
        </button>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Notif masalah -->
    <div v-if="report && problemCount > 0" class="notif notif-warn">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      <div>
        <p class="font-bold">{{ problemCount }} perangkat perlu perhatian</p>
        <p class="text-sm opacity-90">
          {{ report.summary.offline_count }} offline · {{ report.summary.no_data_count }} tanpa data ·
          {{ report.summary.low_battery }} baterai lemah · {{ report.summary.printer_issues }} printer bermasalah<span v-if="report.summary.resource_issues"> · {{ report.summary.resource_issues }} RAM/CPU kritis</span>.
        </p>
      </div>
    </div>
    <div v-else-if="report && report.summary.total_outlets > 0" class="notif notif-ok">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      <p class="font-bold">Semua perangkat aktif & sehat. 🎉</p>
    </div>

    <!-- Filter -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <SearchSelect v-model="filterOutlet" :options="outletFilterOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" @change="load" />
        <SearchableSelect v-model="filterStatus" class="form-input">
          <option value="">Semua status</option>
          <option value="online">Online</option>
          <option value="idle">Idle</option>
          <option value="offline">Offline</option>
          <option value="no_data">Tanpa data</option>
        </SearchableSelect>
        <div class="text-xs text-gray-400 flex items-center sm:justify-end">
          <span v-if="lastUpdated">Diperbarui {{ lastUpdated }}</span>
        </div>
      </div>
    </AppCard>

    <!-- Summary -->
    <div v-if="report" class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div class="stat"><span class="stat-l">Total Perangkat</span><span class="stat-v">{{ report.summary.total_outlets }}</span><span class="stat-s">outlet aktif</span></div>
      <div class="stat stat-ok"><span class="stat-l">Online</span><span class="stat-v">{{ report.summary.online_count }}</span><span class="stat-s">{{ report.summary.idle_count }} idle</span></div>
      <div class="stat" :class="(report.summary.offline_count + report.summary.no_data_count) ? 'stat-bad' : ''"><span class="stat-l">Offline / No Data</span><span class="stat-v">{{ report.summary.offline_count + report.summary.no_data_count }}</span><span class="stat-s">{{ report.summary.no_data_count }} belum lapor</span></div>
      <div class="stat" :class="report.summary.pending_sync_total ? 'stat-warn' : ''"><span class="stat-l">Antrian Sync</span><span class="stat-v">{{ report.summary.pending_sync_total }}</span><span class="stat-s">{{ report.summary.printer_issues }} printer isu</span></div>
    </div>

    <!-- Grid kartu perangkat -->
    <div v-if="loading && !report" class="p-10 text-center text-sm text-gray-400">Memuat…</div>
    <div v-else-if="!filteredDevices.length" class="p-10 text-center text-sm text-gray-400">Tidak ada perangkat untuk filter ini.</div>
    <div v-else class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      <div
        v-for="d in filteredDevices" :key="d.outlet_id"
        class="dev-card" :class="`dev--${d.status}`"
        @click="openDetail(d)"
      >
        <!-- Header kartu -->
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <p class="font-bold text-gray-900 truncate">{{ d.outlet_name }}</p>
            <p class="text-xs text-gray-400 truncate">{{ d.model || '—' }}<span v-if="d.app_version"> · v{{ d.app_version }}</span></p>
          </div>
          <span class="st-badge" :class="`st--${d.status}`">
            <span class="st-dot" /> {{ statusLabel(d.status) }}
          </span>
        </div>

        <template v-if="d.has_data">
          <!-- Baterai + Penyimpanan -->
          <div class="grid grid-cols-2 gap-3 mt-3">
            <div>
              <div class="metric-top">
                <span class="metric-lbl">Baterai</span>
                <span class="metric-val" :class="batteryTextCls(d)">{{ d.battery >= 0 ? d.battery + '%' : '—' }}</span>
              </div>
              <div class="bar"><div class="bar-fill" :class="batteryBarCls(d)" :style="{ width: Math.max(0, d.battery) + '%' }" /></div>
              <p class="metric-sub">{{ batteryStateLabel(d.battery_state) }}</p>
            </div>
            <div>
              <div class="metric-top">
                <span class="metric-lbl">Penyimpanan</span>
                <span class="metric-val" :class="d.storage_used_pct >= 90 ? 'tx-bad' : ''">{{ d.storage_total_mb ? d.storage_used_pct + '%' : '—' }}</span>
              </div>
              <div class="bar"><div class="bar-fill" :class="d.storage_used_pct >= 90 ? 'bf-bad' : (d.storage_used_pct >= 75 ? 'bf-warn' : 'bf-ok')" :style="{ width: d.storage_used_pct + '%' }" /></div>
              <p class="metric-sub">{{ fmtGB(d.storage_free_mb) }} bebas dari {{ fmtGB(d.storage_total_mb) }}</p>
            </div>
          </div>

          <!-- RAM + CPU (Android; nullable) -->
          <div v-if="d.ram_used_percent != null || d.cpu_cores != null" class="grid grid-cols-2 gap-3 mt-3">
            <div v-if="d.ram_used_percent != null">
              <div class="metric-top">
                <span class="metric-lbl">RAM</span>
                <span class="metric-val" :class="ramTextCls(d)">{{ d.ram_used_percent }}%<span v-if="d.ram_low" class="metric-flag">kritis</span></span>
              </div>
              <div class="bar"><div class="bar-fill" :class="ramBarCls(d)" :style="{ width: d.ram_used_percent + '%' }" /></div>
              <p class="metric-sub">{{ fmtGB(d.ram_free_mb) }} bebas dari {{ fmtGB(d.ram_total_mb) }}</p>
            </div>
            <div v-if="d.cpu_cores != null">
              <div class="metric-top">
                <span class="metric-lbl">CPU<span class="metric-cores">{{ d.cpu_cores }} core</span><span v-if="d.cpu_source === 'app'" class="metric-flag" title="Angka CPU hanya beban aplikasi POS (bukan seluruh perangkat)">app</span></span>
                <span class="metric-val" :class="d.cpu_used_percent != null && d.cpu_source !== 'app' ? cpuTextCls(d) : ''">{{ d.cpu_used_percent != null ? Math.round(d.cpu_used_percent) + '%' : '—' }}</span>
              </div>
              <div class="bar"><div class="bar-fill" :class="d.cpu_source === 'app' ? 'bf-ok' : cpuBarCls(d)" :style="{ width: (d.cpu_used_percent || 0) + '%' }" /></div>
              <p class="metric-sub">{{ d.cpu_load_1m != null ? `load ${d.cpu_load_1m.toFixed(2)} / ${(d.cpu_load_5m ?? 0).toFixed(2)} / ${(d.cpu_load_15m ?? 0).toFixed(2)}` : (d.cpu_source === 'app' ? 'beban aplikasi' : 'load tak tersedia') }}</p>
            </div>
          </div>

          <!-- Jaringan + Printer -->
          <div class="flex items-center gap-2 mt-3 flex-wrap">
            <span class="chip" :class="d.network_online ? 'chip-ok' : 'chip-bad'">
              <span class="st-dot" /> {{ d.network_online ? 'Jaringan OK' : 'Jaringan putus' }}
            </span>
            <span class="chip" :class="d.pending_sync > 0 ? 'chip-warn' : 'chip-muted'">
              {{ d.pending_sync }} antri sync
            </span>
            <span class="chip" :class="printerChipCls(d)">
              🖨 {{ d.printers_online }}/{{ d.printers_total }} printer
            </span>
          </div>

          <!-- Footer waktu -->
          <div class="dev-foot">
            <span>{{ d.status === 'online' ? '🟢' : (d.status === 'idle' ? '🟡' : '🔴') }} {{ relTime(d.stale_minutes) }}</span>
            <span class="text-gray-400">Sync {{ d.last_sync_at ? d.last_sync_at.slice(5) : '—' }}</span>
          </div>
        </template>

        <template v-else>
          <div class="nodata">
            <svg class="w-8 h-8 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9.172 16.172a4 4 0 015.656 0M12 20h.01m-7.071-7.071a10 10 0 0114.142 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0"/></svg>
            <p class="text-sm font-medium text-gray-400 mt-1">Belum ada laporan</p>
            <p class="text-xs text-gray-400">Perangkat outlet ini belum pernah mengirim heartbeat.</p>
          </div>
        </template>
      </div>
    </div>

    <!-- Detail modal -->
    <AppModal v-model="detailOpen" :title="active?.outlet_name || 'Detail Perangkat'" size="xl">
      <div v-if="active" class="modal-form-grid">
        <div class="modal-span-full flex items-center justify-between flex-wrap gap-2">
          <span class="st-badge" :class="`st--${active.status}`"><span class="st-dot" /> {{ statusLabel(active.status) }}</span>
          <span class="text-xs text-gray-500">Terakhir lapor {{ relTime(active.stale_minutes) }}</span>
        </div>

        <template v-if="active.has_data">
          <!-- Spesifikasi -->
          <div class="modal-span-full grid grid-cols-1 sm:grid-cols-3 gap-3 text-sm">
            <div class="kv"><span>Model</span><b>{{ active.model || '—' }}</b></div>
            <div class="kv"><span>Sistem</span><b>{{ active.os || '—' }}</b></div>
            <div class="kv"><span>Versi App</span><b>{{ active.app_version || '—' }}</b></div>
            <div class="kv"><span>Baterai</span><b :class="batteryTextCls(active)">{{ active.battery >= 0 ? active.battery + '%' : '—' }} · {{ batteryStateLabel(active.battery_state) }}</b></div>
            <div class="kv"><span>Penyimpanan</span><b>{{ fmtGB(active.storage_free_mb) }} / {{ fmtGB(active.storage_total_mb) }} bebas</b></div>
            <div v-if="active.ram_used_percent != null" class="kv"><span>RAM</span><b :class="ramTextCls(active)">{{ active.ram_used_percent }}% terpakai{{ active.ram_low ? ' · kritis' : '' }}<br><span class="text-xs font-normal text-gray-400">{{ fmtGB(active.ram_free_mb) }} / {{ fmtGB(active.ram_total_mb) }} bebas</span></b></div>
            <div v-if="active.cpu_cores != null" class="kv"><span>CPU ({{ active.cpu_cores }} core)<template v-if="active.cpu_source === 'app'"> · app</template></span><b :class="active.cpu_used_percent != null && active.cpu_source !== 'app' ? cpuTextCls(active) : ''">{{ active.cpu_used_percent != null ? Math.round(active.cpu_used_percent) + (active.cpu_source === 'app' ? '% (beban app)' : '% terpakai') : '—' }}<br><span class="text-xs font-normal text-gray-400">{{ active.cpu_load_1m != null ? `load ${active.cpu_load_1m.toFixed(2)} / ${(active.cpu_load_5m ?? 0).toFixed(2)} / ${(active.cpu_load_15m ?? 0).toFixed(2)}` : 'load tak tersedia' }}</span></b></div>
            <div class="kv"><span>Antrian Sync</span><b :class="active.pending_sync ? 'tx-warn' : ''">{{ active.pending_sync }} item</b></div>
            <div class="kv"><span>Sync terakhir</span><b>{{ active.last_sync_at || '—' }}</b></div>
            <div class="kv"><span>Heartbeat</span><b>{{ active.reported_at || '—' }}</b></div>
            <div class="kv"><span>Diterima cloud</span><b>{{ active.received_at || '—' }}</b></div>
          </div>

          <!-- Printer -->
          <div class="rounded-xl border border-gray-200 overflow-hidden">
            <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600">Printer ({{ active.printers_online }}/{{ active.printers_total }} terhubung)</div>
            <div v-if="!active.printers || !active.printers.length" class="p-3 text-sm text-gray-400">Tidak ada printer terdaftar.</div>
            <ul v-else class="divide-y divide-gray-100">
              <li v-for="(p, i) in active.printers" :key="i" class="px-3 py-2 flex items-center justify-between gap-2 text-sm">
                <div class="min-w-0">
                  <p class="font-medium text-gray-800 truncate">{{ p.name || '—' }} <span v-if="p.roles" class="text-xs text-gray-400 font-normal">· {{ p.roles }}</span></p>
                  <p class="text-xs text-gray-400 truncate">{{ p.type === 'bluetooth' ? 'Bluetooth' : 'LAN' }} · {{ p.address || p.ip || '—' }}</p>
                </div>
                <span class="chip shrink-0" :class="(p.connected || p.online) ? 'chip-ok' : 'chip-bad'">
                  <span class="st-dot" /> {{ (p.connected || p.online) ? 'Terhubung' : 'Terputus' }}
                </span>
              </li>
            </ul>
          </div>

          <!-- Histori / tren -->
          <div class="rounded-xl border border-gray-200 overflow-hidden">
            <div class="px-3 py-2 bg-gray-50 text-xs font-semibold text-gray-600">Tren Baterai & Jaringan ({{ history.length }} laporan terakhir)</div>
            <div class="p-3">
              <div v-if="histLoading" class="text-sm text-gray-400">Memuat tren…</div>
              <div v-else-if="!history.length" class="text-sm text-gray-400">Belum ada histori.</div>
              <div v-else>
                <!-- Sparkline baterai -->
                <svg :viewBox="`0 0 ${sparkW} ${sparkH}`" class="w-full" :style="{ height: sparkH + 'px' }" preserveAspectRatio="none">
                  <polyline :points="batterySpark" fill="none" stroke="#10b981" stroke-width="2" vector-effect="non-scaling-stroke" />
                  <line x1="0" :y1="sparkH*0.8" :x2="sparkW" :y2="sparkH*0.8" stroke="#fca5a5" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke" />
                </svg>
                <!-- Strip jaringan + sync -->
                <div class="net-strip">
                  <span v-for="(pt, i) in history" :key="i" class="net-cell" :class="pt.network_online ? (pt.pending_sync > 0 ? 'net-warn' : 'net-ok') : 'net-bad'" :title="`${pt.reported_at} · bat ${pt.battery}% · antri ${pt.pending_sync}`" />
                </div>
                <div class="flex items-center justify-between text-[10px] text-gray-400 mt-1">
                  <span>{{ history[0]?.reported_at }}</span>
                  <span>baterai 0–100% · garis merah = 20%</span>
                  <span>{{ history[history.length-1]?.reported_at }}</span>
                </div>
              </div>
            </div>
          </div>
        </template>

        <div v-else class="modal-span-full nodata py-6">
          <p class="text-sm font-medium text-gray-400">Perangkat belum mengirim heartbeat.</p>
          <p class="text-xs text-gray-400">Pastikan App POS outlet ini aktif & terhubung internet.</p>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { devicesApi } from '@/api/devices.js'
import { outletsApi } from '@/api/outlets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import { useRealtime } from '@/utils/realtime.js'

const report = ref(null)
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const lastUpdated = ref('')
const filterOutlet = ref('')
const filterStatus = ref('')
const autoRefresh = ref(true)

const detailOpen = ref(false)
const active = ref(null)
const history = ref([])
const histLoading = ref(false)

let timer = null

const outletFilterOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])

const filteredDevices = computed(() => {
  const list = report.value?.devices ?? []
  if (!filterStatus.value) return list
  return list.filter(d => d.status === filterStatus.value)
})

const problemCount = computed(() => {
  const s = report.value?.summary
  if (!s) return 0
  return s.offline_count + s.no_data_count + s.low_battery + s.printer_issues + (s.resource_issues || 0)
})

function statusLabel(s) {
  return { online: 'Online', idle: 'Idle', offline: 'Offline', no_data: 'Tanpa data' }[s] || s
}
function batteryStateLabel(s) {
  return { charging: 'Mengisi', discharging: 'Memakai', full: 'Penuh', unknown: '—' }[s] || (s || '—')
}
function batteryTextCls(d) {
  if (d.battery < 0) return ''
  if (d.battery_state === 'charging' || d.battery_state === 'full') return 'tx-ok'
  if (d.battery < 20) return 'tx-bad'
  if (d.battery < 40) return 'tx-warn'
  return ''
}
function batteryBarCls(d) {
  if (d.battery_state === 'charging') return 'bf-ok'
  if (d.battery < 20) return 'bf-bad'
  if (d.battery < 40) return 'bf-warn'
  return 'bf-ok'
}
function printerChipCls(d) {
  if (d.printers_total === 0) return 'chip-muted'
  return d.printers_online >= d.printers_total ? 'chip-ok' : 'chip-bad'
}
// Ambang RAM: ≥90% atau lowMemory = merah, ≥80% = kuning.
function ramTextCls(d) {
  if ((d.ram_used_percent ?? 0) >= 90 || d.ram_low) return 'tx-bad'
  if ((d.ram_used_percent ?? 0) >= 80) return 'tx-warn'
  return ''
}
function ramBarCls(d) {
  if ((d.ram_used_percent ?? 0) >= 90 || d.ram_low) return 'bf-bad'
  if ((d.ram_used_percent ?? 0) >= 80) return 'bf-warn'
  return 'bf-ok'
}
// Ambang CPU: ≥90% = merah, ≥75% = kuning.
function cpuTextCls(d) {
  if ((d.cpu_used_percent ?? 0) >= 90) return 'tx-bad'
  if ((d.cpu_used_percent ?? 0) >= 75) return 'tx-warn'
  return ''
}
function cpuBarCls(d) {
  if ((d.cpu_used_percent ?? 0) >= 90) return 'bf-bad'
  if ((d.cpu_used_percent ?? 0) >= 75) return 'bf-warn'
  return 'bf-ok'
}
function fmtGB(mb) {
  if (!mb || mb <= 0) return '0 GB'
  if (mb < 1024) return Math.round(mb) + ' MB'
  return (mb / 1024).toFixed(1) + ' GB'
}
function relTime(min) {
  if (min == null || min < 0) return 'belum ada data'
  if (min < 1) return 'baru saja'
  if (min < 60) return `${min} menit lalu`
  const h = Math.floor(min / 60)
  if (h < 24) return `${h} jam lalu`
  return `${Math.floor(h / 24)} hari lalu`
}

// ── Sparkline baterai ──
const sparkW = 300
const sparkH = 48
const batterySpark = computed(() => {
  const h = history.value
  if (!h.length) return ''
  const n = h.length
  return h.map((pt, i) => {
    const x = n === 1 ? sparkW / 2 : (i / (n - 1)) * sparkW
    const bat = Math.max(0, Math.min(100, pt.battery < 0 ? 0 : pt.battery))
    const y = sparkH - (bat / 100) * sparkH
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
})

function openDetail(d) {
  active.value = d
  detailOpen.value = true
  history.value = []
  if (d.has_data) loadHistory(d.outlet_id)
}

async function loadHistory(outletId) {
  histLoading.value = true
  try {
    const d = await devicesApi.history(outletId)
    history.value = d?.data ?? d ?? []
  } catch { history.value = [] } finally { histLoading.value = false }
}

async function load(silent = false) {
  if (silent !== true) { loading.value = true }
  errorMsg.value = ''
  try {
    const d = await devicesApi.monitor({ outlet_id: filterOutlet.value || undefined })
    report.value = d?.data ?? d
    lastUpdated.value = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
  } catch (e) { if (silent !== true) errorMsg.value = e?.message || 'Gagal memuat status perangkat' } finally { loading.value = false }
}
async function loadOutlets() {
  try { const d = await outletsApi.myOutlets(); outlets.value = d?.outlets ?? d ?? [] } catch { outlets.value = [] }
}

watch(autoRefresh, (on) => { on ? startTimer() : stopTimer() })
function startTimer() { stopTimer(); timer = setInterval(() => load(true), 30000) }
function stopTimer() { if (timer) { clearInterval(timer); timer = null } }

onMounted(async () => { await loadOutlets(); await load(); if (autoRefresh.value) startTimer() })
onUnmounted(stopTimer)
// SSE: update seketika saat heartbeat device masuk (polling 30 dtk tetap
// berjalan sebagai fallback bila koneksi stream terputus). silent — tanpa kedip.
useRealtime(['device'], () => load(true))
</script>

<style scoped>
.form-input { width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem; border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none; }
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }

.refresh-btn { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: .6rem; border: 1px solid rgba(0,0,0,.1); background: #fff; color: #475569; cursor: pointer; transition: background .15s; }
.refresh-btn:hover { background: #f1f5f9; }
.refresh-btn svg { width: 17px; height: 17px; }
.refresh-btn.spinning svg { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.notif { display: flex; gap: .7rem; align-items: flex-start; padding: .85rem 1rem; border-radius: .9rem; }
.notif-warn { background: rgba(245,158,11,.12); color: #92400e; border: 1px solid rgba(245,158,11,.3); }
.notif-ok { background: rgba(16,185,129,.1); color: #065f46; border: 1px solid rgba(16,185,129,.25); }

.stat { background: #fff; border: 1px solid rgba(0,0,0,.07); border-radius: 1rem; padding: .85rem 1rem; display: flex; flex-direction: column; gap: .1rem; box-shadow: 0 1px 2px rgba(0,0,0,.04); }
.stat-l { font-size: .68rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.stat-v { font-size: 1.5rem; font-weight: 800; color: #111827; line-height: 1.1; }
.stat-s { font-size: .7rem; color: #9ca3af; }
.stat-ok { background: rgba(16,185,129,.06); border-color: rgba(16,185,129,.25); }
.stat-ok .stat-v { color: #047857; }
.stat-bad { background: rgba(239,68,68,.05); border-color: rgba(239,68,68,.25); }
.stat-bad .stat-v { color: #b91c1c; }
.stat-warn { background: rgba(245,158,11,.06); border-color: rgba(245,158,11,.3); }
.stat-warn .stat-v { color: #b45309; }

/* Kartu perangkat */
.dev-card { background: #fff; border: 1px solid rgba(0,0,0,.08); border-radius: 1rem; padding: 1rem; cursor: pointer; box-shadow: 0 1px 2px rgba(0,0,0,.04); transition: box-shadow .15s, transform .1s, border-color .15s; border-left-width: 4px; }
.dev-card:hover { box-shadow: 0 6px 22px rgba(0,0,0,.09); transform: translateY(-1px); }
.dev--online  { border-left-color: #10b981; }
.dev--idle    { border-left-color: #f59e0b; }
.dev--offline { border-left-color: #ef4444; }
.dev--no_data { border-left-color: #cbd5e1; }

.st-badge { display: inline-flex; align-items: center; gap: .35rem; padding: .2rem .6rem; border-radius: 999px; font-size: .68rem; font-weight: 700; white-space: nowrap; }
.st-dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; flex-shrink: 0; }
.st--online  { background: rgba(16,185,129,.14); color: #047857; }
.st--idle    { background: rgba(245,158,11,.16); color: #b45309; }
.st--offline { background: rgba(239,68,68,.13); color: #b91c1c; }
.st--no_data { background: rgba(148,163,184,.18); color: #64748b; }

.metric-top { display: flex; align-items: baseline; justify-content: space-between; }
.metric-lbl { font-size: .66rem; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; color: #94a3b8; }
.metric-val { font-size: .95rem; font-weight: 800; color: #111827; }
.metric-sub { font-size: .68rem; color: #9ca3af; margin-top: .25rem; }
.metric-cores { font-size: .6rem; font-weight: 600; color: #cbd5e1; margin-left: .35rem; text-transform: none; letter-spacing: 0; }
.metric-flag { font-size: .58rem; font-weight: 800; color: #b91c1c; background: rgba(239,68,68,.12); border-radius: .35rem; padding: .05rem .3rem; margin-left: .35rem; vertical-align: middle; }
.bar { height: 7px; border-radius: 999px; background: #eef2f5; overflow: hidden; margin-top: .3rem; }
.bar-fill { height: 100%; border-radius: 999px; transition: width .3s; }
.bf-ok { background: linear-gradient(90deg, #34d399, #10b981); }
.bf-warn { background: linear-gradient(90deg, #fbbf24, #f59e0b); }
.bf-bad { background: linear-gradient(90deg, #f87171, #ef4444); }

.tx-ok { color: #047857; } .tx-warn { color: #b45309; } .tx-bad { color: #b91c1c; }

.chip { display: inline-flex; align-items: center; gap: .3rem; padding: .18rem .55rem; border-radius: 999px; font-size: .68rem; font-weight: 600; white-space: nowrap; }
.chip-ok { background: rgba(16,185,129,.12); color: #047857; }
.chip-bad { background: rgba(239,68,68,.12); color: #b91c1c; }
.chip-warn { background: rgba(245,158,11,.14); color: #b45309; }
.chip-muted { background: #f1f5f9; color: #64748b; }

.dev-foot { display: flex; align-items: center; justify-content: space-between; margin-top: .75rem; padding-top: .6rem; border-top: 1px solid #f1f5f9; font-size: .7rem; color: #475569; font-weight: 600; }

.nodata { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; padding: 1.5rem 0 .5rem; }

.kv { display: flex; flex-direction: column; }
.kv span { font-size: .66rem; color: #9ca3af; text-transform: uppercase; letter-spacing: .03em; }
.kv b { color: #111827; font-size: .85rem; }

.net-strip { display: flex; gap: 2px; margin-top: .6rem; }
.net-cell { flex: 1; height: 14px; border-radius: 2px; min-width: 2px; }
.net-ok { background: #34d399; }
.net-warn { background: #fbbf24; }
.net-bad { background: #f87171; }
</style>
