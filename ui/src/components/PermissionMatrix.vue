<!--
  PermissionMatrix.vue — Sidebar-aligned, per-submenu granular permission editor

  Mirrors the app sidebar (kategori → submenu). Each submenu maps to the EXACT
  permission keys the backend enforces (services.AllPermissions):
    • CRUD submenu  → Lihat·Tambah·Ubah·Hapus table (ops can be partial, e.g.
                      Transfer Stok has no Hapus → shown as "–").
    • Workflow/single → toggle chips (e.g. Pengadaan: Lihat/Buat/Approval/Isi Harga,
                      Laporan: satu chip per jenis laporan).

  Parent owns the active list + view-dependency cascade; this only emits toggle(key).
-->
<template>
  <div class="pm">
    <!-- Ringkasan + pencarian -->
    <div class="pm-top">
      <div class="pm-sum">
        <div class="pm-sum-line">
          <span class="pm-sum-num">{{ activeTotal }}<small>/{{ allTotal }}</small></span>
          <span class="pm-sum-txt">hak akses aktif · {{ fullCats }}/{{ CATEGORIES.length }} kategori penuh</span>
        </div>
        <div class="pm-bar" role="progressbar" :aria-valuenow="activeTotal" :aria-valuemax="allTotal"><span :style="{ width: pct + '%' }" /></div>
      </div>
      <div class="pm-tools">
        <label class="pm-search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model.trim="q" type="search" placeholder="Cari submenu… (resep, neraca, vendor)" aria-label="Cari submenu" />
        </label>
        <button type="button" class="pm-fold" @click="toggleAll">{{ allClosed ? 'Buka semua' : 'Lipat semua' }}</button>
      </div>
    </div>

    <!-- Legenda kolom (tampil saat label kolom disembunyikan di layar sempit) -->
    <div class="pm-legend" aria-hidden="true">
      <span v-for="op in OPS" :key="op.key" class="th-pill" :class="`th-pill--${op.cls}`"><span class="op-svg" v-html="op.icon" />{{ op.label }}</span>
    </div>

    <p v-if="q && !visibleCats.length" class="pm-empty">Tidak ada submenu yang cocok dengan “{{ q }}”.</p>

    <div v-for="cat in visibleCats" :key="cat.label" class="pm-cat" :class="{ 'pm-cat--dim': disabled, 'pm-cat--closed': isClosed(cat) }">
      <button type="button" class="pm-cat-hd" :aria-expanded="!isClosed(cat)" @click="toggleCat(cat)">
        <span class="pm-cat-ic" v-html="cat.icon" />
        <span class="pm-cat-title">{{ cat.label }}</span>
        <span class="pm-cat-count" :class="{ 'pm-cat-count--full': catActive(cat.all) === catTotal(cat.all) && catTotal(cat.all) > 0, 'pm-cat-count--zero': catActive(cat.all) === 0 }">
          {{ catActive(cat.all) }}/{{ catTotal(cat.all) }}
        </span>
        <svg class="pm-cat-chev" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
      </button>

      <div v-show="!isClosed(cat)" class="pm-cat-body">
        <!-- CRUD table -->
        <div v-if="crudItems(cat).length" class="matrix-wrap">
          <table class="matrix">
            <thead>
              <tr>
                <th class="th-mod">Submenu</th>
                <th v-for="op in OPS" :key="op.key" :class="`th-${op.cls}`">
                  <span class="th-pill" :class="`th-pill--${op.cls}`" :title="op.label"><span class="op-svg" v-html="op.icon" /><span class="th-txt">{{ op.label }}</span></span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in crudItems(cat)" :key="m.module">
                <td class="td-mod"><span class="mod-ic" v-html="m.icon" /><span class="mod-name">{{ m.label }}</span></td>
                <td v-for="op in OPS" :key="op.key" class="td-op">
                  <button
                    v-if="(m.ops || ALL_OPS).includes(op.key)"
                    type="button"
                    class="op-pill" :class="[`op-pill--${op.cls}`, { 'op-pill--on': has(`${m.module}.${op.key}`) }]"
                    :aria-label="`${op.label} ${m.label}`" :aria-pressed="has(`${m.module}.${op.key}`)"
                    @click="emitToggle(`${m.module}.${op.key}`)"
                  >
                    <span class="op-svg" v-html="has(`${m.module}.${op.key}`) ? CHECK : CROSS" />
                  </button>
                  <span v-else class="op-na" title="Aksi ini tidak tersedia untuk submenu ini">–</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Single / toggle-chip rows -->
        <div v-if="optItems(cat).length" class="opt-rows">
          <div v-for="it in optItems(cat)" :key="it.label" class="opt-row">
            <span class="opt-label"><span class="opt-ic" v-html="it.icon" />{{ it.label }}</span>
            <div class="opt-pills">
              <template v-if="it.type === 'single'">
                <button type="button" class="opt-pill" :class="{ 'opt-pill--on': has(it.key) }" :aria-pressed="has(it.key)" @click="emitToggle(it.key)">
                  <span class="op-svg" v-if="has(it.key)" v-html="CHECK" />{{ has(it.key) ? 'Aktif' : 'Nonaktif' }}
                </button>
              </template>
              <template v-else>
                <button
                  v-for="t in it.toggles" :key="t.key"
                  type="button" class="opt-pill" :class="{ 'opt-pill--on': has(t.key) }" :aria-pressed="has(t.key)"
                  @click="emitToggle(t.key)"
                >
                  <span class="op-svg" v-if="has(t.key)" v-html="CHECK" />{{ t.label }}
                </button>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  permissions: { type: Array, default: () => [] },
  disabled:    { type: Boolean, default: false },
})
const emit = defineEmits(['toggle'])

function has(key) { return props.permissions.includes(key) }
function emitToggle(key) { if (!props.disabled) emit('toggle', key) }

const CHECK = '<svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>'
const CROSS = '<svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>'

const ALL_OPS = ['view', 'create', 'update', 'delete']
const OPS = [
  { key: 'view',   cls: 'v', label: 'Lihat',  icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M10 12a2 2 0 100-4 2 2 0 000 4z"/><path fill-rule="evenodd" d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clip-rule="evenodd"/></svg>' },
  { key: 'create', cls: 'c', label: 'Tambah', icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"/></svg>' },
  { key: 'update', cls: 'u', label: 'Ubah',   icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path d="M13.586 3.586a2 2 0 112.828 2.828l-.793.793-2.828-2.828.793-.793zM11.379 5.793L3 14.172V17h2.828l8.38-8.379-2.83-2.828z"/></svg>' },
  { key: 'delete', cls: 'd', label: 'Hapus',  icon: '<svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd"/></svg>' },
]

const stroke = (inner) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
const IC = {
  dashboard: stroke('<rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/>'),
  master:    stroke('<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.66 3.58 3 8 3s8-1.34 8-3V5"/><path d="M4 11v6c0 1.66 3.58 3 8 3s8-1.34 8-3v-6"/>'),
  outlet:    stroke('<path d="M3 9l1.5-5h15L21 9"/><path d="M4 9v10a1 1 0 001 1h14a1 1 0 001-1V9"/><path d="M3 9h18"/><path d="M9 20v-6h6v6"/>'),
  workunit:  stroke('<rect x="3" y="7" width="18" height="13" rx="2"/><path d="M8 7V5a2 2 0 012-2h4a2 2 0 012 2v2"/><line x1="3" y1="12" x2="21" y2="12"/>'),
  product:   stroke('<path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/>'),
  finance:   stroke('<line x1="6" y1="20" x2="6" y2="13"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="18" y1="20" x2="18" y2="9"/>'),
  report:    stroke('<path d="M14 3H6a2 2 0 00-2 2v14a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 3 14 8 20 8"/><line x1="8" y1="13" x2="12" y2="13"/><line x1="8" y1="17" x2="14" y2="17"/>'),
  wallet:    stroke('<path d="M21 12V7H5a2 2 0 010-4h14v4"/><path d="M3 5v14a2 2 0 002 2h16v-5"/><path d="M18 12a2 2 0 000 4h4v-4z"/>'),
  bank:      stroke('<line x1="3" y1="21" x2="21" y2="21"/><path d="M3 10l9-6 9 6"/><line x1="5" y1="10" x2="5" y2="21"/><line x1="19" y1="10" x2="19" y2="21"/><line x1="9" y1="10" x2="9" y2="21"/><line x1="15" y1="10" x2="15" y2="21"/>'),
  cart:      stroke('<circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 002 1.61h9.72a2 2 0 002-1.61L23 6H6"/>'),
  vendor:    stroke('<rect x="1" y="3" width="15" height="13" rx="1"/><path d="M16 8h4l3 3v5h-7z"/><circle cx="5.5" cy="18.5" r="2.5"/><circle cx="18.5" cy="18.5" r="2.5"/>'),
  users:     stroke('<path d="M17 21v-2a4 4 0 00-4-4H7a4 4 0 00-4 4v2"/><circle cx="10" cy="7" r="4"/><path d="M21 21v-2a4 4 0 00-3-3.87"/><path d="M16 3.13a4 4 0 010 7.75"/>'),
  shield:    stroke('<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M9 12l2 2 4-4"/>'),
  warehouse: stroke('<path d="M3 21V8l9-5 9 5v13"/><path d="M3 21h18"/><rect x="9" y="13" width="6" height="8"/>'),
  gauge:     stroke('<path d="M3 13a9 9 0 1118 0"/><path d="M12 13l4-3"/><line x1="3" y1="21" x2="21" y2="21"/>'),
  stockitem: stroke('<polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/>'),
  transfer:  stroke('<polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 014-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 01-4 4H3"/>'),
  waste:     stroke('<polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/><path d="M9 6V4a1 1 0 011-1h4a1 1 0 011 1v2"/>'),
  ledger:    stroke('<path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/>'),
  recipe:    stroke('<path d="M9 2h6a1 1 0 011 1v1h2a2 2 0 012 2v13a2 2 0 01-2 2H6a2 2 0 01-2-2V6a2 2 0 012-2h2V3a1 1 0 011-1z"/><line x1="8" y1="11" x2="16" y2="11"/><line x1="8" y1="15" x2="13" y2="15"/>'),
  whatsapp:  stroke('<path d="M21 11.5a8.38 8.38 0 01-.9 3.8 8.5 8.5 0 01-7.6 4.7 8.38 8.38 0 01-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 01-.9-3.8 8.5 8.5 0 014.7-7.6 8.38 8.38 0 013.8-.9h.5a8.48 8.48 0 018 8v.5z"/>'),
  settings:  stroke('<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 11-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 110-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 114 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06A1.65 1.65 0 0019.4 9v0a1.65 1.65 0 001.51 1H21a2 2 0 110 4h-.09a1.65 1.65 0 00-1.51 1z"/>'),
  company:   stroke('<line x1="3" y1="21" x2="21" y2="21"/><path d="M5 21V5a2 2 0 012-2h10a2 2 0 012 2v16"/><line x1="9" y1="7" x2="9.01" y2="7"/><line x1="15" y1="7" x2="15.01" y2="7"/><line x1="9" y1="11" x2="9.01" y2="11"/><line x1="15" y1="11" x2="15.01" y2="11"/>'),
  clock:     stroke('<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/>'),
  tax:       stroke('<line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/>'),
}

// ── Sidebar-aligned granular catalog (matches services.AllPermissions) ──────
const CATEGORIES = [
  {
    label: 'Dashboard', icon: IC.dashboard,
    items: [{ type: 'single', key: 'dashboard', label: 'Akses Dashboard', icon: IC.dashboard }],
  },
  {
    label: 'Master Data', icon: IC.master,
    items: [
      { type: 'crud', module: 'outlets', label: 'Outlet', icon: IC.outlet },
      { type: 'crud', module: 'workunits', label: 'Unit Kerja', icon: IC.workunit },
      { type: 'crud', module: 'warehouses', label: 'Gudang', icon: IC.warehouse },
      { type: 'crud', module: 'roles', label: 'Role & Hak Akses', icon: IC.shield },
      { type: 'crud', module: 'appfiles', label: 'App POS', icon: IC.product, ops: ['view', 'create', 'delete'] },
    ],
  },
  {
    label: 'Perlengkapan', icon: IC.warehouse,
    items: [
      { type: 'single', key: 'assets.dashboard.view', label: 'Dashboard Aset', icon: IC.dashboard },
      { type: 'crud', module: 'assets', label: 'Data Aset', icon: IC.warehouse },
      {
        type: 'toggles', label: 'Opname Aset', icon: IC.ledger,
        toggles: [
          { key: 'assets.opname.view',    label: 'Lihat' },
          { key: 'assets.opname.create',  label: 'Hitung' },
          { key: 'assets.opname.approve', label: 'Terapkan Selisih' },
        ],
      },
      {
        type: 'toggles', label: 'Penghapusan Aset', icon: IC.waste,
        toggles: [
          { key: 'assets.disposal.view',    label: 'Lihat' },
          { key: 'assets.disposal.create',  label: 'Ajukan' },
          { key: 'assets.disposal.approve', label: 'Setujui' },
        ],
      },
      { type: 'single', key: 'assets.report.view', label: 'Laporan Aset', icon: IC.report },
      {
        type: 'toggles', label: 'Perawatan', icon: IC.settings,
        toggles: [
          { key: 'assets.maintenance.view',   label: 'Lihat' },
          { key: 'assets.maintenance.create', label: 'Catat & Selesaikan' },
        ],
      },
      {
        type: 'toggles', label: 'Mutasi Antar Outlet', icon: IC.transfer,
        toggles: [
          { key: 'assets.transfer.view',    label: 'Lihat' },
          { key: 'assets.transfer.create',  label: 'Buat & Kirim' },
          { key: 'assets.transfer.approve', label: 'Setujui' },
          { key: 'assets.transfer.receive', label: 'Terima' },
        ],
      },
    ],
  },
  {
    label: 'Produk', icon: IC.product,
    items: [{ type: 'crud', module: 'products', label: 'Produk & Kategori', icon: IC.product }],
  },
  {
    label: 'Penjualan', icon: IC.cart,
    items: [
      { type: 'crud', module: 'reservations', label: 'Reservasi', icon: IC.cart },
      { type: 'single', key: 'customers.view', label: 'Pelanggan', icon: IC.users },
    ],
  },
  {
    label: 'Laporan', icon: IC.finance,
    items: [
      {
        type: 'toggles', label: 'Laporan (per jenis)', icon: IC.report,
        toggles: [
          { key: 'reports.sales.view',         label: 'Pendapatan' },
          { key: 'reports.product_sales.view', label: 'Penjualan Produk' },
          { key: 'reports.ledger.view',        label: 'Buku Besar' },
          { key: 'reports.cashflow.view',      label: 'Kas Masuk/Keluar' },
          { key: 'reports.pnl.view',           label: 'Profit & Loss' },
          { key: 'reports.balance.view',       label: 'Neraca' },
          { key: 'reports.tax.view',           label: 'Pajak' },
          { key: 'reports.void.view',          label: 'Void (Transaksi & Item)' },
          { key: 'reports.titipan.view',       label: 'Titipan' },
          { key: 'reports.discount.view',      label: 'Diskon & Komplimen' },
          { key: 'reports.business_analysis.view', label: 'Analisa Bisnis' },
        ],
      },
      {
        type: 'toggles', label: 'Kinerja Markom (IG/TikTok)', icon: IC.report,
        toggles: [
          { key: 'social.view',   label: 'Lihat' },
          { key: 'social.manage', label: 'Kelola akun & isi manual' },
        ],
      },
      { type: 'single', key: 'cashier_shifts.view', label: 'Laporan Shift Kasir', icon: IC.report },
      { type: 'single', key: 'shift_reconciliation.view', label: 'Rekonsiliasi Shift', icon: IC.report },
      {
        type: 'toggles', label: 'Pembayaran Pengadaan', icon: IC.wallet,
        toggles: [
          { key: 'finance.payments.view', label: 'Lihat' },
          { key: 'finance.payments.pay',  label: 'Bayar' },
        ],
      },
      { type: 'crud', module: 'finance.bank', label: 'Data Rekening', icon: IC.bank },
    ],
  },
  {
    label: 'Pengadaan', icon: IC.cart,
    items: [
      { type: 'single', key: 'procurement.dashboard.view', label: 'Dashboard Pengadaan', icon: IC.gauge },
      {
        type: 'toggles', label: 'Permintaan Pembelian (Barang & Jasa)', icon: IC.cart,
        toggles: [
          { key: 'procurement.requests.view',       label: 'Lihat' },
          { key: 'procurement.requests.submit',     label: 'Buat/Hapus' },
          { key: 'procurement.requests.approve',    label: 'Approval' },
          { key: 'procurement.requests.purchasing', label: 'Isi Harga' },
        ],
      },
      {
        type: 'toggles', label: 'Projek (Pembangunan/Renovasi)', icon: IC.workunit,
        toggles: [
          { key: 'procurement.projects.view',   label: 'Lihat' },
          { key: 'procurement.projects.manage', label: 'Kelola & Tetapkan RAB' },
        ],
      },
      { type: 'crud', module: 'vendors', label: 'Vendor', icon: IC.vendor },
    ],
  },
  {
    label: 'PPIC', icon: IC.gauge,
    items: [
      { type: 'single', key: 'ppic.dashboard.view', label: 'Dashboard PPIC', icon: IC.gauge },
      {
        type: 'toggles', label: 'Par Level & ROP', icon: IC.stockitem,
        toggles: [
          { key: 'ppic.planning.view',   label: 'Lihat' },
          { key: 'ppic.planning.update', label: 'Ubah' },
        ],
      },
      {
        type: 'toggles', label: 'Monitor Kedaluwarsa', icon: IC.clock,
        toggles: [
          { key: 'ppic.expiry.view', label: 'Lihat' },
          { key: 'ppic.expiry.ack',  label: 'Tandai Ditindak' },
        ],
      },
      {
        type: 'toggles', label: 'Stock Opname', icon: IC.ledger,
        toggles: [
          { key: 'ppic.opname.view',    label: 'Lihat' },
          { key: 'ppic.opname.create',  label: 'Buat/Hitung' },
          { key: 'ppic.opname.approve', label: 'Approve' },
        ],
      },
      {
        type: 'toggles', label: 'Demand Forecast', icon: IC.finance,
        toggles: [
          { key: 'ppic.forecast.view',   label: 'Lihat' },
          { key: 'ppic.forecast.manage', label: 'Generate/Ubah' },
        ],
      },
      {
        type: 'toggles', label: 'Kebutuhan Bahan (MRP)', icon: IC.cart,
        toggles: [
          { key: 'ppic.mrp.view',    label: 'Lihat' },
          { key: 'ppic.mrp.run',     label: 'Hitung' },
          { key: 'ppic.mrp.execute', label: 'Buat Draft PR/Transfer' },
        ],
      },
      {
        type: 'toggles', label: 'Rencana Produksi (MPS)', icon: IC.workunit,
        toggles: [
          { key: 'ppic.production.view',    label: 'Lihat' },
          { key: 'ppic.production.create',  label: 'Buat' },
          { key: 'ppic.production.approve', label: 'Approve/Release' },
        ],
      },
      {
        type: 'toggles', label: 'Work Order', icon: IC.recipe,
        toggles: [
          { key: 'ppic.workorders.view',    label: 'Lihat' },
          { key: 'ppic.workorders.create',  label: 'Buat/Batal' },
          { key: 'ppic.workorders.execute', label: 'Mulai/Selesaikan' },
        ],
      },
      {
        type: 'toggles', label: 'Laporan PPIC', icon: IC.report,
        toggles: [
          { key: 'ppic.reports.view',   label: 'Lihat' },
          { key: 'ppic.reports.export', label: 'Export Excel' },
        ],
      },
    ],
  },
  {
    label: 'Pengguna', icon: IC.users,
    items: [
      { type: 'crud', module: 'users', label: 'User (Admin)', icon: IC.users },
      { type: 'single', key: 'access_logs.view', label: 'Log Akses', icon: IC.shield },
    ],
  },
  {
    label: 'Gudang', icon: IC.warehouse,
    items: [
      { type: 'single', key: 'warehouse_dashboard.view', label: 'Dashboard Gudang', icon: IC.gauge },
      { type: 'crud', module: 'stockitems', label: 'Item Stok', icon: IC.stockitem },
      {
        // Setujui dan terima dipisah dari kirim/batalkan, sama seperti mutasi aset.
        type: 'toggles', label: 'Transfer Stok', icon: IC.transfer,
        toggles: [
          { key: 'stocktransfers.view',    label: 'Lihat' },
          { key: 'stocktransfers.create',  label: 'Buat' },
          { key: 'stocktransfers.update',  label: 'Kirim & Batalkan' },
          { key: 'stocktransfers.approve', label: 'Setujui' },
          { key: 'stocktransfers.receive', label: 'Terima' },
        ],
      },
      { type: 'crud', module: 'stockwastes', label: 'Stok Rusak/Hilang', icon: IC.waste, ops: ['view', 'create'] },
      {
        type: 'toggles', label: 'Buku Stok', icon: IC.ledger,
        toggles: [
          { key: 'stockledger.view',   label: 'Lihat' },
          { key: 'stockledger.adjust', label: 'Penyesuaian' },
        ],
      },
      { type: 'crud', module: 'recipes', label: 'Resep', icon: IC.recipe },
    ],
  },
  {
    label: 'WhatsApp', icon: IC.whatsapp,
    items: [
      {
        type: 'toggles', label: 'WhatsApp', icon: IC.whatsapp,
        toggles: [
          { key: 'whatsapp.view',      label: 'Lihat status & log' },
          { key: 'whatsapp.manage',    label: 'Kelola koneksi, penerima & notifikasi' },
          { key: 'whatsapp.broadcast', label: 'Kirim broadcast' },
        ],
      },
    ],
  },
  {
    label: 'Pengaturan', icon: IC.settings,
    items: [
      { type: 'toggles', label: 'Identitas Perusahaan', icon: IC.company, toggles: [{ key: 'settings.company.view', label: 'Lihat' }, { key: 'settings.company.update', label: 'Ubah' }] },
      { type: 'toggles', label: 'Zona Waktu', icon: IC.clock, toggles: [{ key: 'settings.timezone.view', label: 'Lihat' }, { key: 'settings.timezone.update', label: 'Ubah' }] },
      { type: 'toggles', label: 'Pajak', icon: IC.tax, toggles: [{ key: 'settings.tax.view', label: 'Lihat' }, { key: 'settings.tax.update', label: 'Ubah' }] },
      { type: 'single', key: 'devices.view', label: 'Perangkat (Monitoring)', icon: IC.gauge },
      { type: 'crud', module: 'cameras', label: 'Kamera CCTV (API)', icon: IC.gauge },
    ],
  },
]

function catKeys(cat) {
  const keys = []
  for (const it of cat.items) {
    if (it.type === 'crud') (it.ops || ALL_OPS).forEach(op => keys.push(`${it.module}.${op}`))
    else if (it.type === 'single') keys.push(it.key)
    else if (it.type === 'toggles') it.toggles.forEach(t => keys.push(t.key))
  }
  return keys
}
function catTotal(cat)  { return catKeys(cat).length }
function catActive(cat) { return catKeys(cat).filter(has).length }
function crudItems(cat) { return cat.items.filter(i => i.type === 'crud') }
function optItems(cat)  { return cat.items.filter(i => i.type !== 'crud') }

// ── Ringkasan ────────────────────────────────────────────────────────────
const ALL_KEYS = CATEGORIES.flatMap(catKeys)
const allTotal = ALL_KEYS.length
const activeTotal = computed(() => ALL_KEYS.filter(has).length)
const fullCats = computed(() => CATEGORIES.filter(c => catTotal(c) > 0 && catActive(c) === catTotal(c)).length)
const pct = computed(() => allTotal ? Math.round(activeTotal.value / allTotal * 100) : 0)

// ── Pencarian submenu (label submenu, kategori, atau label toggle) ────────
const q = ref('')
function itemMatch(it, needle) {
  if (it.label.toLowerCase().includes(needle)) return true
  return it.type === 'toggles' && it.toggles.some(t => t.label.toLowerCase().includes(needle))
}
// `all` menunjuk kategori asli supaya hitungan tidak ikut terfilter.
const visibleCats = computed(() => {
  const needle = q.value.toLowerCase()
  return CATEGORIES.map(cat => {
    const items = !needle || cat.label.toLowerCase().includes(needle) ? cat.items : cat.items.filter(it => itemMatch(it, needle))
    return { ...cat, items, all: cat }
  }).filter(c => c.items.length)
})

// ── Lipat kategori: di layar sempit mulai terlipat agar daftar tidak memanjang ──
const narrow = typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(max-width: 640px)').matches
const closed = ref(new Set(narrow ? CATEGORIES.map(c => c.label) : []))
function isClosed(cat) { return !q.value && closed.value.has(cat.label) }
function toggleCat(cat) {
  const next = new Set(closed.value)
  next.has(cat.label) ? next.delete(cat.label) : next.add(cat.label)
  closed.value = next
}
const allClosed = computed(() => !q.value && closed.value.size >= CATEGORIES.length)
function toggleAll() { closed.value = new Set(allClosed.value ? [] : CATEGORIES.map(c => c.label)) }
</script>

<style scoped>
.pm { display: flex; flex-direction: column; gap: .75rem; }

.pm-cat { border: 1.5px solid #eef0f3; border-radius: .85rem; background: #fff; padding: .7rem .8rem .8rem; transition: border-color .15s, box-shadow .15s; }
.pm-cat:hover { border-color: #e2e8f0; box-shadow: 0 2px 10px rgba(15,23,42,.04); }
.pm-cat--dim .pm-cat-body { opacity: .55; pointer-events: none; }

/* Ringkasan + alat */
.pm-top { display: flex; flex-wrap: wrap; align-items: center; gap: .6rem 1rem; padding: .7rem .85rem; border: 1.5px solid #e0e7ff; border-radius: .85rem; background: linear-gradient(135deg, #f5f7ff, #eef2ff); }
.pm-sum { flex: 1 1 220px; min-width: 0; }
.pm-sum-line { display: flex; align-items: baseline; gap: .45rem; flex-wrap: wrap; }
.pm-sum-num { font-size: 1.05rem; font-weight: 800; color: #3730a3; line-height: 1; }
.pm-sum-num small { font-size: .7rem; font-weight: 700; color: #818cf8; }
.pm-sum-txt { font-size: .7rem; font-weight: 600; color: #4b5563; }
.pm-bar { margin-top: .4rem; height: 5px; border-radius: 999px; background: #e0e7ff; overflow: hidden; }
.pm-bar span { display: block; height: 100%; border-radius: 999px; background: linear-gradient(90deg, #6366f1, #22c55e); transition: width .2s; }
.pm-tools { display: flex; align-items: center; gap: .5rem; flex: 1 1 240px; min-width: 0; }
.pm-search { display: flex; align-items: center; gap: .4rem; flex: 1; min-width: 0; padding: .35rem .6rem; border: 1.5px solid #c7d2fe; border-radius: .6rem; background: #fff; color: #6b7280; }
.pm-search:focus-within { border-color: #818cf8; box-shadow: 0 0 0 3px rgba(99,102,241,.12); }
.pm-search svg { width: 14px; height: 14px; flex-shrink: 0; }
.pm-search input { flex: 1; min-width: 0; border: none; outline: none; background: transparent; font-size: .76rem; color: #111827; }
.pm-fold { flex-shrink: 0; padding: .4rem .6rem; border: 1.5px solid #c7d2fe; border-radius: .6rem; background: #fff; color: #4338ca; font-size: .7rem; font-weight: 700; cursor: pointer; white-space: nowrap; }
.pm-fold:hover { background: #eef2ff; }
.pm-legend { display: none; flex-wrap: wrap; gap: .35rem; padding: 0 .2rem; }
.pm-empty { padding: 1rem; border: 1.5px dashed #e5e7eb; border-radius: .75rem; text-align: center; font-size: .76rem; color: #9ca3af; }

.pm-cat-hd { display: flex; width: 100%; align-items: center; gap: .55rem; margin-bottom: .65rem; padding: 0; border: none; background: none; text-align: left; cursor: pointer; }
.pm-cat--closed .pm-cat-hd { margin-bottom: 0; }
.pm-cat-chev { width: 16px; height: 16px; flex-shrink: 0; color: #94a3b8; transition: transform .18s; }
.pm-cat--closed .pm-cat-chev { transform: rotate(-90deg); }
.pm-cat-body { display: flex; flex-direction: column; gap: .5rem; }
.pm-cat-ic { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: .55rem; background: linear-gradient(135deg, #eef2ff, #e0e7ff); color: #4f46e5; flex-shrink: 0; }
.pm-cat-ic :deep(svg) { width: 16px; height: 16px; }
.pm-cat-title { font-size: .82rem; font-weight: 700; color: #1e293b; letter-spacing: -.01em; }
.pm-cat-count { margin-left: auto; flex-shrink: 0; font-size: .66rem; font-weight: 700; padding: .12rem .5rem; border-radius: 999px; background: #f1f5f9; color: #94a3b8; }
.pm-cat-count--full { background: #dcfce7; color: #15803d; }
.pm-cat-count--zero { background: #f1f5f9; color: #cbd5e1; }

.matrix-wrap { border: 1.5px solid #eef0f3; border-radius: .6rem; overflow: hidden; overflow-x: auto; }
.matrix { width: 100%; border-collapse: collapse; font-size: .78rem; }
.matrix thead tr { background: #f8fafc; }
.matrix th { padding: .45rem .7rem; text-align: center; font-weight: 700; border-bottom: 1.5px solid #eef0f3; }
.th-mod { text-align: left; width: 36%; font-size: .68rem; color: #64748b; text-transform: uppercase; letter-spacing: .04em; }
.th-pill { display: inline-flex; align-items: center; gap: .25rem; padding: .18rem .5rem; border-radius: .35rem; font-size: .66rem; font-weight: 700; }
.th-pill--v { background: #dbeafe; color: #1d4ed8; }
.th-pill--c { background: #dcfce7; color: #15803d; }
.th-pill--u { background: #fef3c7; color: #b45309; }
.th-pill--d { background: #fee2e2; color: #dc2626; }

.matrix tbody tr { border-bottom: 1px solid #f3f4f6; transition: background .1s; }
.matrix tbody tr:last-child { border-bottom: none; }
.matrix tbody tr:hover { background: #fafbff; }
.td-mod { padding: .55rem .7rem; display: flex; align-items: center; gap: .5rem; }
.mod-ic { display: inline-flex; color: #64748b; flex-shrink: 0; }
.mod-ic :deep(svg) { width: 16px; height: 16px; }
.mod-name { font-weight: 600; color: #374151; }
.td-op { text-align: center; padding: .4rem .45rem; }

.op-pill { display: inline-flex; align-items: center; justify-content: center; width: 1.7rem; height: 1.7rem; border-radius: .45rem; border: 1.5px solid #e5e7eb; background: #f9fafb; color: #cbd5e1; cursor: pointer; transition: all .12s; }
.op-pill:hover { transform: scale(1.12); }
.op-na { display: inline-block; color: #d1d5db; font-weight: 700; user-select: none; }
.op-svg { display: inline-flex; }
.op-svg :deep(svg) { width: 10px; height: 10px; }
.td-op .op-svg :deep(svg) { width: 9px; height: 9px; }

.op-pill--v.op-pill--on { background: #dbeafe; color: #1d4ed8; border-color: #93c5fd; }
.op-pill--c.op-pill--on { background: #dcfce7; color: #15803d; border-color: #86efac; }
.op-pill--u.op-pill--on { background: #fef3c7; color: #b45309; border-color: #fcd34d; }
.op-pill--d.op-pill--on { background: #fee2e2; color: #dc2626; border-color: #fca5a5; }
.op-pill--v:not(.op-pill--on):hover { background: #eff6ff; border-color: #bfdbfe; color: #60a5fa; }
.op-pill--c:not(.op-pill--on):hover { background: #f0fdf4; border-color: #bbf7d0; color: #4ade80; }
.op-pill--u:not(.op-pill--on):hover { background: #fffbeb; border-color: #fde68a; color: #f59e0b; }
.op-pill--d:not(.op-pill--on):hover { background: #fff1f2; border-color: #fecaca; color: #f87171; }

.opt-rows { display: flex; flex-direction: column; gap: .4rem; }
.opt-row { display: flex; align-items: center; gap: .6rem; flex-wrap: wrap; padding: .4rem .55rem; border-radius: .55rem; background: #f8fafc; border: 1px solid #f1f5f9; }
.opt-label { display: inline-flex; align-items: center; gap: .4rem; font-size: .74rem; font-weight: 600; color: #475569; min-width: 160px; }
.opt-ic { display: inline-flex; color: #94a3b8; }
.opt-ic :deep(svg) { width: 15px; height: 15px; }
.opt-pills { display: flex; flex-wrap: wrap; gap: .3rem; margin-left: auto; }

.opt-pill { display: inline-flex; align-items: center; gap: .25rem; font-size: .7rem; font-weight: 600; padding: .25rem .65rem; border-radius: 999px; border: 1.5px solid #e5e7eb; background: #f9fafb; color: #94a3b8; cursor: pointer; transition: all .12s; }
.opt-pill:hover { background: #eef2ff; border-color: #c7d2fe; color: #4f46e5; }
.opt-pill--on { background: #f0fdf4; color: #15803d; border-color: #86efac; }
.opt-pill--on:hover { background: #dcfce7; border-color: #4ade80; color: #166534; }
.opt-pill .op-svg :deep(svg) { width: 9px; height: 9px; }

@media (max-width: 640px) {
  .pm { gap: .6rem; }
  .pm-cat { padding: .6rem .6rem .7rem; }
  .pm-top { padding: .6rem .7rem; }
  .matrix th, .matrix td { padding: .35rem .25rem; }
  .td-mod { padding-left: .45rem; gap: .35rem; }
  .th-mod { width: auto; }
  .mod-name { font-size: .72rem; line-height: 1.25; }
  .opt-row { padding: .45rem .5rem; }
  .opt-label { min-width: 0; width: 100%; }
  .opt-pills { margin-left: 0; }
  .opt-pill { padding: .35rem .7rem; }
  .op-pill { width: 2rem; height: 2rem; }
}
@media (max-width: 420px) {
  /* Kolom aksi terlalu sempit untuk label; sisakan ikon, label ada di legenda. */
  .th-txt { display: none; }
  .th-pill { padding: .25rem .4rem; }
  .th-pill .op-svg :deep(svg) { width: 12px; height: 12px; }
  .pm-legend { display: flex; }
}
</style>
