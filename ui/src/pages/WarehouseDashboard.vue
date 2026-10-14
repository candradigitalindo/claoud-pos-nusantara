<template>
  <div class="wd">

    <!-- ── Header ── -->
    <div class="wd-hd">
      <div>
        <h1 class="wd-title">Dashboard Gudang</h1>
        <p class="wd-sub">Ringkasan stok, pergerakan, dan kondisi gudang secara real-time.</p>
      </div>
      <button class="wd-refresh" @click="load" :disabled="loading">
        <svg :class="{ spin: loading }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
        Refresh
      </button>
    </div>

    <AppAlert type="error" :message="err" />

    <div v-if="loading" class="wd-loading"><AppSpinner size="lg" class="text-emerald-600"/></div>

    <template v-if="!loading && d">

      <!-- ── KPI Cards ── -->
      <div class="kpi-grid">
        <div class="kpi kpi--blue">
          <div class="kpi-ico kpi-ico--blue">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline stroke-linecap="round" stroke-linejoin="round" points="9 22 9 12 15 12 15 22"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ d.total_warehouses }}</div>
            <div class="kpi-label">Total Gudang</div>
            <div class="kpi-sub">{{ d.central_warehouses }} Induk &middot; {{ d.outlet_warehouses }} Outlet</div>
          </div>
        </div>

        <div class="kpi kpi--emerald">
          <div class="kpi-ico kpi-ico--emerald">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ d.total_items }}</div>
            <div class="kpi-label">Total Item Bahan</div>
            <div class="kpi-sub">{{ d.items_with_stock }} item ada stok</div>
          </div>
        </div>

        <div class="kpi" :class="d.low_stock_count > 0 ? 'kpi--amber' : 'kpi--gray'">
          <div class="kpi-ico" :class="d.low_stock_count > 0 ? 'kpi-ico--amber' : 'kpi-ico--gray'">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ d.low_stock_count }}</div>
            <div class="kpi-label">Stok Rendah</div>
            <div class="kpi-sub">{{ d.out_of_stock_count }} item habis</div>
          </div>
        </div>

        <div class="kpi kpi--purple">
          <div class="kpi-ico kpi-ico--purple">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><line x1="12" y1="1" x2="12" y2="23" stroke-linecap="round"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ fmtRp(d.total_stock_value) }}</div>
            <div class="kpi-label">Nilai Aset Stok</div>
            <div class="kpi-sub">Berdasarkan rata-rata biaya</div>
          </div>
        </div>

        <div class="kpi" :class="d.pending_transfers > 0 ? 'kpi--orange' : 'kpi--gray'">
          <div class="kpi-ico" :class="d.pending_transfers > 0 ? 'kpi-ico--orange' : 'kpi-ico--gray'">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="1" y="3" width="15" height="13" rx="1" stroke-linecap="round" stroke-linejoin="round"/><path stroke-linecap="round" stroke-linejoin="round" d="M16 8h4l3 5v3h-7V8zM5.5 21a1.5 1.5 0 100-3 1.5 1.5 0 000 3zm13 0a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ d.pending_transfers }}</div>
            <div class="kpi-label">Transfer Pending</div>
            <div class="kpi-sub">Menunggu proses</div>
          </div>
        </div>

        <div class="kpi kpi--teal">
          <div class="kpi-ico kpi-ico--teal">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><polyline stroke-linecap="round" stroke-linejoin="round" points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
          </div>
          <div class="kpi-body">
            <div class="kpi-val">{{ d.movements_today }}</div>
            <div class="kpi-label">Pergerakan Hari Ini</div>
            <div class="kpi-sub">{{ d.movements_7d }} dalam 7 hari</div>
          </div>
        </div>
      </div>

      <!-- ── Row 2: Gudang + Trend ── -->
      <div class="row2">

        <!-- Stok per Gudang -->
        <div class="card">
          <div class="card-hd">
            <div class="card-title-wrap">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"/><path stroke-linecap="round" stroke-linejoin="round" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
              <span class="card-title">Stok per Gudang</span>
            </div>
          </div>
          <div class="wh-list">
            <div v-if="d.warehouse_stocks.length === 0" class="empty-sm">Belum ada data stok</div>
            <div v-for="w in d.warehouse_stocks" :key="w.warehouse_id" class="wh-row">
              <div class="wh-left">
                <span class="wh-badge" :class="w.warehouse_type === 'central' ? 'wh-badge--central' : 'wh-badge--outlet'">
                  {{ w.warehouse_type === 'central' ? 'Induk' : 'Outlet' }}
                </span>
                <div>
                  <div class="wh-name">{{ w.warehouse_name }}</div>
                  <div class="wh-outlet">{{ w.outlet_name }}</div>
                </div>
              </div>
              <div class="wh-right">
                <div class="wh-stat">
                  <span class="wh-items">{{ w.total_items }} item</span>
                  <span v-if="w.low_stock_count > 0" class="wh-low">
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/></svg>
                    {{ w.low_stock_count }}
                  </span>
                </div>
                <div class="wh-value">{{ fmtRp(w.total_value) }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- Trend Pergerakan 14 Hari -->
        <div class="card">
          <div class="card-hd">
            <div class="card-title-wrap">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline stroke-linecap="round" stroke-linejoin="round" points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline stroke-linecap="round" stroke-linejoin="round" points="17 6 23 6 23 12"/></svg>
              <span class="card-title">Pergerakan Stok 14 Hari</span>
            </div>
          </div>
          <div v-if="noTrend" class="empty-sm">Belum ada pergerakan stok</div>
          <div v-else class="trend-wrap">
            <div class="trend-legend">
              <span class="dot dot--in"></span><span>Masuk</span>
              <span class="dot dot--out" style="margin-left:.75rem"></span><span>Keluar</span>
            </div>
            <div class="trend-bars">
              <div v-for="p in d.movement_trend" :key="p.date" class="trend-col">
                <div class="bar-wrap">
                  <div class="bar bar--out" :style="{ height: barH(p.out) + 'px' }" :title="`Keluar: ${p.out}`"></div>
                  <div class="bar bar--in"  :style="{ height: barH(p.in)  + 'px' }" :title="`Masuk: ${p.in}`"></div>
                </div>
                <div class="bar-label">{{ fmtDay(p.date) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ── Row 3: Stok Rendah + Transfer Pending ── -->
      <div class="row2">

        <!-- Stok Rendah / Habis -->
        <div class="card">
          <div class="card-hd">
            <div class="card-title-wrap">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/></svg>
              <span class="card-title">Stok Rendah / Habis</span>
            </div>
            <router-link to="/stock-ledger" class="card-link">Lihat Semua →</router-link>
          </div>
          <div v-if="d.low_stock_items.length === 0" class="empty-sm ok">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            Semua stok mencukupi
          </div>
          <table v-else class="mini-table">
            <thead>
              <tr><th>Item</th><th>Gudang</th><th>Stok</th><th>Min</th><th>Status</th></tr>
            </thead>
            <tbody>
              <tr v-for="r in d.low_stock_items" :key="r.item_id + r.warehouse_id">
                <td class="td-name">{{ r.item_name }}</td>
                <td class="td-wh">{{ r.warehouse_name }}</td>
                <td class="td-num" :class="r.qty_base <= 0 ? 'txt-red' : 'txt-amber'">
                  {{ fmtQty(r.qty_base) }} {{ r.base_unit }}
                </td>
                <td class="td-num">{{ fmtQty(r.min_stock) }}</td>
                <td>
                  <span class="status-pill" :class="r.qty_base <= 0 ? 'pill-red' : 'pill-amber'">
                    {{ r.qty_base <= 0 ? 'Habis' : 'Rendah' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Transfer Pending -->
        <div class="card">
          <div class="card-hd">
            <div class="card-title-wrap">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="1" y="3" width="15" height="13" rx="1" stroke-linecap="round" stroke-linejoin="round"/><path stroke-linecap="round" stroke-linejoin="round" d="M16 8h4l3 5v3h-7V8zM5.5 21a1.5 1.5 0 100-3 1.5 1.5 0 000 3zm13 0a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"/></svg>
              <span class="card-title">Transfer Pending</span>
            </div>
            <router-link to="/stock-transfers" class="card-link">Lihat Semua →</router-link>
          </div>
          <div v-if="d.pending_transfer_list.length === 0" class="empty-sm ok">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            Tidak ada transfer tertunda
          </div>
          <div v-else class="transfer-list">
            <div v-for="t in d.pending_transfer_list" :key="t.id" class="tr-row">
              <div class="tr-top">
                <span class="tr-num">{{ t.transfer_number }}</span>
                <span class="tr-status" :class="statusClass(t.status)">{{ statusLabel(t.status) }}</span>
              </div>
              <div class="tr-route">
                <span class="tr-wh">{{ t.from_warehouse_name ?? t.from_warehouse }}</span>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
                <span class="tr-wh">{{ t.to_warehouse_name ?? t.to_warehouse }}</span>
              </div>
              <div class="tr-date">{{ fmtDate(t.created_at) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- ── Pergerakan Terkini ── -->
      <div class="card">
        <div class="card-hd">
          <div class="card-title-wrap">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline stroke-linecap="round" stroke-linejoin="round" points="17 1 21 5 17 9"/><path stroke-linecap="round" stroke-linejoin="round" d="M3 11V9a4 4 0 014-4h14M7 23l-4-4 4-4"/><path stroke-linecap="round" stroke-linejoin="round" d="M21 13v2a4 4 0 01-4 4H3"/></svg>
            <span class="card-title">Pergerakan Stok Terkini</span>
          </div>
          <router-link to="/stock-ledger" class="card-link">Buku Stok →</router-link>
        </div>
        <div v-if="d.recent_movements.length === 0" class="empty-sm">Belum ada pergerakan stok</div>
        <div v-else class="overflow-x-auto">
          <table class="mini-table">
            <thead>
              <tr>
                <th>Waktu</th><th>Item</th><th>Gudang</th><th>Tipe</th>
                <th class="th-r">Qty</th><th>Referensi</th><th>Oleh</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in d.recent_movements" :key="m.id">
                <td class="td-date">{{ fmtDateTime(m.created_at) }}</td>
                <td class="td-name">{{ m.item_name }}</td>
                <td class="td-wh">{{ m.warehouse_name }}</td>
                <td><span class="mov-badge" :class="movClass(m.movement_type)">{{ movLabel(m.movement_type) }}</span></td>
                <td class="td-num" :class="m.qty_base >= 0 ? 'txt-green' : 'txt-red'">
                  {{ m.qty_base >= 0 ? '+' : '' }}{{ fmtQty(m.qty_base) }} {{ m.unit_used }}
                </td>
                <td class="td-ref">{{ m.ref_number || m.ref_type || '—' }}</td>
                <td class="td-by">{{ m.created_by || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { apiClient } from '@/api/client.js'
import { movementLabel, transferStatusLabel } from '@/utils/warehouse.js'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'

const d       = ref(null)
const loading = ref(false)
const err     = ref('')

onMounted(load)

async function load() {
  loading.value = true; err.value = ''
  try {
    d.value = await apiClient.get('/admin/warehouse-dashboard')
  } catch (e) {
    err.value = e?.message ?? 'Gagal memuat dashboard gudang'
  } finally {
    loading.value = false
  }
}

const noTrend = computed(() => !d.value?.movement_trend?.some(p => p.in > 0 || p.out > 0))

const maxMov = computed(() => {
  if (!d.value?.movement_trend?.length) return 1
  return Math.max(1, ...d.value.movement_trend.map(p => Math.max(p.in, p.out)))
})
function barH(val) { return Math.max(2, Math.round((val / maxMov.value) * 80)) }

function fmtRp(v) {
  if (!v || v === 0) return 'Rp 0'
  if (v >= 1_000_000_000) return 'Rp ' + (v / 1_000_000_000).toFixed(1) + ' M'
  if (v >= 1_000_000)     return 'Rp ' + (v / 1_000_000).toFixed(1) + ' jt'
  return 'Rp ' + Number(v).toLocaleString('id-ID')
}
function fmtQty(v)  { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtDay(s)  { return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short' }) }
function fmtDate(s) {
  if (!s) return '—'
  return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' })
}
function fmtDateTime(s) {
  if (!s) return '—'
  return new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

// Label dari util bersama; kelas badge tetap lokal (pakai CSS scoped halaman ini).
const STATUS_CLASS = { draft: 'st-gray', approved: 'st-blue', sent: 'st-amber', received: 'st-green', cancelled: 'st-red' }
const statusLabel = transferStatusLabel
function statusClass(s) { return STATUS_CLASS[s] ?? 'st-gray' }

const MOV_IN  = new Set(['purchase_in','return_in','transfer_in','production_in','breakdown_in','adjustment'])
const MOV_OUT = new Set(['sale','waste','spoiled','expired','transfer_out','production_out','breakdown_out'])
const movLabel = movementLabel
function movClass(t) {
  if (MOV_IN.has(t))  return 'mov-in'
  if (MOV_OUT.has(t)) return 'mov-out'
  return 'mov-neutral'
}
</script>

<style scoped>
.wd { display: flex; flex-direction: column; gap: 1.25rem; }

/* Header */
.wd-hd  { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.wd-title { font-size: 1.35rem; font-weight: 800; color: #111827; margin: 0; }
.wd-sub   { font-size: .8rem; color: #6b7280; margin: .2rem 0 0; }
.wd-loading { display: flex; justify-content: center; padding: 4rem; }
.wd-refresh {
  display: inline-flex; align-items: center; gap: .4rem;
  font-size: .78rem; font-weight: 600; padding: .5rem .9rem;
  border: 1.5px solid #a7f3d0; border-radius: .55rem;
  background: #ecfdf5; color: #065f46; cursor: pointer; transition: all .15s;
}
.wd-refresh:hover   { background: #d1fae5; }
.wd-refresh:disabled{ opacity: .5; cursor: not-allowed; }
.spin { animation: spin .7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* KPI Grid */
.kpi-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: .75rem;
}
@media (max-width: 900px) { .kpi-grid { grid-template-columns: repeat(2,1fr); } }
@media (max-width: 520px) { .kpi-grid { grid-template-columns: 1fr; } }

.kpi {
  display: flex; align-items: center; gap: .85rem;
  padding: 1rem 1.1rem; border-radius: .875rem; border: 1.5px solid;
}
.kpi--blue   { background: #eff6ff; border-color: #bfdbfe; }
.kpi--emerald{ background: #ecfdf5; border-color: #a7f3d0; }
.kpi--amber  { background: #fffbeb; border-color: #fde68a; }
.kpi--purple { background: #faf5ff; border-color: #e9d5ff; }
.kpi--orange { background: #fff7ed; border-color: #fed7aa; }
.kpi--teal   { background: #f0fdfa; border-color: #99f6e4; }
.kpi--gray   { background: #f9fafb; border-color: #e5e7eb; }

.kpi-ico {
  width: 2.6rem; height: 2.6rem; border-radius: .65rem; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
}
.kpi-ico--blue   { background: #dbeafe; color: #1d4ed8; }
.kpi-ico--emerald{ background: #d1fae5; color: #059669; }
.kpi-ico--amber  { background: #fef3c7; color: #d97706; }
.kpi-ico--purple { background: #ede9fe; color: #7c3aed; }
.kpi-ico--orange { background: #ffedd5; color: #ea580c; }
.kpi-ico--teal   { background: #ccfbf1; color: #0f766e; }
.kpi-ico--gray   { background: #f3f4f6; color: #9ca3af; }

.kpi-val  { font-size: 1.45rem; font-weight: 800; color: #111827; line-height: 1.1; }
.kpi-label{ font-size: .7rem; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: #6b7280; margin-top: .2rem; }
.kpi-sub  { font-size: .68rem; color: #9ca3af; margin-top: .15rem; }

/* Layout */
.row2 { display: grid; grid-template-columns: 1fr 1fr; gap: .75rem; }
@media (max-width: 768px) { .row2 { grid-template-columns: 1fr; } }

/* Card */
.card {
  background: #fff; border-radius: .875rem; border: 1.5px solid #e5e7eb;
  padding: 1.1rem; box-shadow: 0 1px 3px rgba(0,0,0,.04);
}
.card-hd { display: flex; align-items: center; justify-content: space-between; margin-bottom: .9rem; }
.card-title-wrap { display: flex; align-items: center; gap: .4rem; color: #374151; }
.card-title { font-size: .85rem; font-weight: 700; color: #111827; }
.card-link  { font-size: .72rem; font-weight: 600; color: #059669; text-decoration: none; white-space: nowrap; }
.card-link:hover { text-decoration: underline; }

/* Empty */
.empty-sm { display: flex; align-items: center; justify-content: center; gap: .4rem; padding: 1.75rem; font-size: .8rem; color: #9ca3af; }
.empty-sm.ok { color: #059669; }

/* Warehouse list */
.wh-list { display: flex; flex-direction: column; gap: .45rem; }
.wh-row {
  display: flex; align-items: center; justify-content: space-between; gap: .75rem;
  padding: .55rem .65rem; border-radius: .5rem; background: #f9fafb; border: 1px solid #f3f4f6;
}
.wh-left { display: flex; align-items: center; gap: .5rem; min-width: 0; }
.wh-badge { font-size: .6rem; font-weight: 700; padding: .15rem .4rem; border-radius: .3rem; flex-shrink: 0; }
.wh-badge--central { background: #ede9fe; color: #6d28d9; }
.wh-badge--outlet  { background: #d1fae5; color: #065f46; }
.wh-name   { font-size: .78rem; font-weight: 600; color: #374151; }
.wh-outlet { font-size: .68rem; color: #9ca3af; }
.wh-right  { text-align: right; flex-shrink: 0; }
.wh-stat   { display: flex; gap: .4rem; justify-content: flex-end; align-items: center; }
.wh-items  { font-size: .7rem; color: #6b7280; }
.wh-low    { display: flex; align-items: center; gap: .2rem; font-size: .68rem; color: #d97706; font-weight: 600; }
.wh-value  { font-size: .78rem; font-weight: 700; color: #111827; margin-top: .1rem; }

/* Trend chart */
.trend-legend { display: flex; align-items: center; gap: .3rem; font-size: .7rem; color: #6b7280; margin-bottom: .7rem; }
.dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot--in  { background: #10b981; }
.dot--out { background: #f43f5e; }
.trend-bars { display: flex; align-items: flex-end; gap: 2px; height: 96px; overflow-x: auto; padding-bottom: 1.5rem; position: relative; }
.trend-col  { display: flex; flex-direction: column; align-items: center; gap: 2px; flex: 1; min-width: 20px; }
.bar-wrap   { display: flex; gap: 1px; align-items: flex-end; }
.bar        { width: 6px; border-radius: 2px 2px 0 0; min-height: 2px; transition: height .25s; }
.bar--in    { background: #10b981; }
.bar--out   { background: #f43f5e; }
.bar-label  { font-size: .55rem; color: #9ca3af; position: absolute; bottom: 0; white-space: nowrap; }

/* Mini table */
.mini-table { width: 100%; border-collapse: collapse; font-size: .75rem; }
.mini-table thead tr { background: #f9fafb; }
.mini-table th {
  padding: .4rem .6rem; text-align: left;
  font-size: .65rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em;
  color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap;
}
.mini-table .th-r { text-align: right; }
.mini-table tbody tr { border-bottom: 1px solid #f9fafb; transition: background .1s; }
.mini-table tbody tr:last-child { border-bottom: none; }
.mini-table tbody tr:hover { background: #f9fafb; }
.mini-table td { padding: .45rem .6rem; color: #374151; }

.td-name { font-weight: 600; max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.td-wh   { color: #6b7280; white-space: nowrap; max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
.td-num  { text-align: right; font-weight: 600; font-family: monospace; white-space: nowrap; }
.td-date { color: #9ca3af; white-space: nowrap; font-size: .7rem; }
.td-ref  { color: #9ca3af; font-size: .7rem; max-width: 100px; overflow: hidden; text-overflow: ellipsis; }
.td-by   { color: #9ca3af; font-size: .7rem; white-space: nowrap; }

.txt-red   { color: #dc2626; }
.txt-amber { color: #d97706; }
.txt-green { color: #059669; }

/* Status pills */
.status-pill { font-size: .62rem; font-weight: 700; padding: .15rem .45rem; border-radius: 999px; }
.pill-red    { background: #fee2e2; color: #dc2626; }
.pill-amber  { background: #fef3c7; color: #d97706; }

/* Transfer list */
.transfer-list { display: flex; flex-direction: column; gap: .45rem; }
.tr-row {
  padding: .6rem .75rem; border-radius: .5rem;
  background: #f9fafb; border: 1px solid #f3f4f6;
}
.tr-top  { display: flex; align-items: center; justify-content: space-between; margin-bottom: .25rem; }
.tr-num  { font-size: .78rem; font-weight: 700; color: #111827; }
.tr-status { font-size: .62rem; font-weight: 700; padding: .15rem .45rem; border-radius: 999px; }
.st-gray  { background: #f3f4f6; color: #6b7280; }
.st-blue  { background: #dbeafe; color: #1d4ed8; }
.st-amber { background: #fef3c7; color: #d97706; }
.st-green { background: #dcfce7; color: #15803d; }
.st-red   { background: #fee2e2; color: #dc2626; }
.tr-route { display: flex; align-items: center; gap: .35rem; font-size: .72rem; color: #374151; margin-bottom: .2rem; }
.tr-wh    { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 120px; }
.tr-date  { font-size: .68rem; color: #9ca3af; }

/* Movement badges */
.mov-badge { font-size: .65rem; font-weight: 700; padding: .15rem .45rem; border-radius: .3rem; white-space: nowrap; }
.mov-in      { background: #dcfce7; color: #15803d; }
.mov-out     { background: #fee2e2; color: #dc2626; }
.mov-neutral { background: #f3f4f6; color: #6b7280; }
</style>
