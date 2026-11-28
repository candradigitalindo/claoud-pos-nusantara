<template>
  <div class="space-y-4 sm:space-y-5 pb-6">

    <!-- ── 1. Header & Filter Gudang ── -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-xl sm:text-2xl font-extrabold tracking-tight text-gray-900">Dashboard Gudang</h1>
          <span v-if="selectedWarehouseObj" class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-0.5 text-xs font-semibold text-emerald-800">
            {{ selectedWarehouseObj.warehouse_name }}
            <button type="button" @click="selectWarehouse('')" class="ml-0.5 text-emerald-600 hover:text-emerald-900 cursor-pointer" title="Reset ke semua gudang">×</button>
          </span>
        </div>
        <p class="mt-0.5 text-xs sm:text-sm text-gray-500">
          Ringkasan stok, kadaluwarsa ≤ 1 bulan, antrean transfer, dan pergerakan gudang secara real-time.
          <span v-if="lastUpdated" class="text-gray-400"> · {{ lastUpdated }}</span>
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <select
          v-if="warehouseOptions.length > 1"
          v-model="selectedWarehouseId"
          @change="load()"
          aria-label="Filter gudang"
          class="min-w-0 flex-1 sm:flex-none sm:w-56 rounded-lg border border-gray-300 bg-white px-3 py-2 text-xs sm:text-sm font-medium text-gray-700 shadow-2xs focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/20"
        >
          <option value="">Semua Gudang ({{ warehouseOptions.length }})</option>
          <option v-for="w in warehouseOptions" :key="w.warehouse_id" :value="w.warehouse_id">
            {{ w.warehouse_type === 'central' ? '[Induk] ' : '' }}{{ w.warehouse_name }}
          </option>
        </select>
        <button
          type="button"
          @click="load()"
          :disabled="loading"
          class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs sm:text-sm font-semibold text-emerald-800 hover:bg-emerald-100 disabled:opacity-50 transition cursor-pointer shrink-0"
        >
          <svg :class="{ 'animate-spin': loading }" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <AppAlert type="error" :message="err" />

    <div v-if="loading && !d" class="flex justify-center py-16">
      <AppSpinner size="lg" class="text-emerald-600" />
    </div>

    <template v-if="d">

      <!-- ── Banner Peringatan Mendesak (Expired / Kritis <= 7 Hari) ── -->
      <div
        v-if="d.expired_count > 0 || d.expiring_7d_count > 0"
        class="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between rounded-xl border border-rose-300 bg-gradient-to-r from-rose-50 to-amber-50 px-4 py-3 shadow-2xs"
      >
        <div class="flex items-start gap-2.5 min-w-0">
          <div class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-rose-600 text-white">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
              <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
            </svg>
          </div>
          <div class="min-w-0">
            <p class="text-xs sm:text-sm font-bold text-rose-950">
              Perhatian Kedaluwarsa!
              <span v-if="d.expired_count > 0">{{ d.expired_count }} batch sudah kadaluwarsa</span>
              <span v-if="d.expired_count > 0 && d.expiring_7d_count > 0"> · </span>
              <span v-if="d.expiring_7d_count > 0">{{ d.expiring_7d_count }} batch kritis (≤ 7 hari)</span>
            </p>
            <p class="text-[11px] sm:text-xs text-rose-800 mt-0.5">
              Total nilai risiko 1 bulan ke depan: <strong>{{ fmtRpFull(d.expiring_30d_value) }}</strong>. Prioritaskan pemakaian (FEFO) atau karantina segera.
            </p>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2 shrink-0">
          <button type="button" @click="scrollToExpiry"
            class="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700 transition cursor-pointer">
            Lihat Daftar ↓
          </button>
          <router-link v-if="canViewQuality" to="/stock-quality"
            class="rounded-lg border border-rose-300 bg-white px-3 py-1.5 text-xs font-semibold text-rose-700 hover:bg-rose-50 transition">
            Tindak Karantina
          </router-link>
        </div>
      </div>

      <!-- ── 2. 4 KPI Utama (Kondisi Stok Inti) ── -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-2.5 sm:gap-3.5">
        <!-- Nilai Aset Stok -->
        <div class="kpi-card bg-purple-50/80 border-purple-200">
          <div class="kpi-top">
            <span class="kpi-ico bg-purple-100 text-purple-700">
              <svg class="h-4 w-4 sm:h-5 sm:w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23" stroke-linecap="round"/><path stroke-linecap="round" stroke-linejoin="round" d="M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6"/></svg>
            </span>
            <span class="kpi-lbl">Nilai Aset Stok</span>
          </div>
          <div class="kpi-val text-purple-950" :title="fmtRpFull(d.total_stock_value)">{{ fmtRp(d.total_stock_value) }}</div>
          <div class="kpi-sub">{{ d.items_with_stock }}/{{ d.total_items }} item berstok · {{ d.total_warehouses }} gudang</div>
        </div>

        <!-- Expired <= 1 Bulan -->
        <button type="button" @click="scrollToExpiry"
          :class="d.expired_count > 0 ? 'bg-rose-50/90 border-rose-300 hover:border-rose-400' : (d.expiring_30d_count > 0 ? 'bg-amber-50/90 border-amber-300 hover:border-amber-400' : 'bg-emerald-50/75 border-emerald-200 hover:border-emerald-300')"
          class="kpi-card text-left transition cursor-pointer">
          <div class="kpi-top">
            <span :class="d.expired_count > 0 ? 'bg-rose-100 text-rose-700' : (d.expiring_30d_count > 0 ? 'bg-amber-100 text-amber-700' : 'bg-emerald-100 text-emerald-700')" class="kpi-ico">
              <svg class="h-4 w-4 sm:h-5 sm:w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </span>
            <span class="kpi-lbl">Expired ≤ 1 Bln</span>
          </div>
          <div class="kpi-val" :class="d.expired_count > 0 ? 'text-rose-700' : (d.expiring_30d_count > 0 ? 'text-amber-800' : 'text-emerald-700')">
            {{ d.expiring_30d_count }} <span class="text-xs font-semibold opacity-75">batch</span>
          </div>
          <div class="kpi-sub">
            <template v-if="d.expiring_30d_count > 0">
              {{ d.expired_count }} lewat · {{ d.expiring_7d_count }} ≤7 hr · {{ fmtRp(d.expiring_30d_value) }}
            </template>
            <template v-else>Aman (&gt; 30 hari)</template>
          </div>
        </button>

        <!-- Stok Rendah / Habis -->
        <component :is="canViewLedger ? 'router-link' : 'div'" :to="canViewLedger ? '/stock-ledger' : undefined"
          :class="[d.low_stock_count > 0 ? 'bg-amber-50/90 border-amber-300' : 'bg-gray-50 border-gray-200', canViewLedger ? (d.low_stock_count > 0 ? 'hover:border-amber-400' : 'hover:border-gray-300') : '']"
          class="kpi-card transition">
          <div class="kpi-top">
            <span :class="d.low_stock_count > 0 ? 'bg-amber-100 text-amber-700' : 'bg-gray-200 text-gray-500'" class="kpi-ico">
              <svg class="h-4 w-4 sm:h-5 sm:w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/></svg>
            </span>
            <span class="kpi-lbl">Stok Rendah</span>
          </div>
          <div class="kpi-val" :class="d.low_stock_count > 0 ? 'text-amber-800' : 'text-gray-900'">{{ d.low_stock_count }}</div>
          <div class="kpi-sub">{{ d.out_of_stock_count }} item habis (saldo ≤ 0)</div>
        </component>

        <!-- Aktivitas Hari Ini -->
        <div class="kpi-card bg-teal-50/80 border-teal-200">
          <div class="kpi-top">
            <span class="kpi-ico bg-teal-100 text-teal-700">
              <svg class="h-4 w-4 sm:h-5 sm:w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline stroke-linecap="round" stroke-linejoin="round" points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            </span>
            <span class="kpi-lbl">Aktivitas Hari Ini</span>
          </div>
          <div class="kpi-val text-teal-950">{{ d.movements_today }}</div>
          <div class="kpi-sub">{{ d.movements_7d }} transaksi dalam 7 hari</div>
        </div>
      </div>

      <!-- ── Strip Status Operasional Ringkas (1 baris rapi di desktop, 2x2 di HP) ── -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-2 sm:gap-3">
        <component :is="canViewTransfers ? 'router-link' : 'div'" :to="canViewTransfers ? '/stock-transfers' : undefined"
          class="op-pill" :class="[d.pending_transfers > 0 ? 'border-orange-200 bg-orange-50/70' : 'border-gray-200 bg-white', canViewTransfers ? (d.pending_transfers > 0 ? 'hover:bg-orange-50' : 'hover:bg-gray-50') : '']">
          <span class="text-[11px] font-semibold text-gray-600 truncate">Transfer Pending</span>
          <span class="text-sm font-extrabold shrink-0" :class="d.pending_transfers > 0 ? 'text-orange-700' : 'text-gray-800'">{{ d.pending_transfers }}</span>
        </component>
        <component :is="canViewLogistics ? 'router-link' : 'div'" :to="canViewLogistics ? '/logistics/deliveries' : undefined"
          class="op-pill" :class="[d.incoming_deliveries > 0 ? 'border-indigo-200 bg-indigo-50/70' : 'border-gray-200 bg-white', canViewLogistics ? (d.incoming_deliveries > 0 ? 'hover:bg-indigo-50' : 'hover:bg-gray-50') : '']">
          <span class="text-[11px] font-semibold text-gray-600 truncate">SJ Logistik Masuk</span>
          <span class="text-sm font-extrabold shrink-0" :class="d.incoming_deliveries > 0 ? 'text-indigo-700' : 'text-gray-800'">{{ d.incoming_deliveries }}</span>
        </component>
        <component :is="canViewQuality ? 'router-link' : 'div'" :to="canViewQuality ? '/stock-quality' : undefined"
          class="op-pill" :class="[d.open_quality_cases > 0 ? 'border-rose-200 bg-rose-50/70' : 'border-gray-200 bg-white', canViewQuality ? (d.open_quality_cases > 0 ? 'hover:bg-rose-50' : 'hover:bg-gray-50') : '']">
          <span class="text-[11px] font-semibold text-gray-600 truncate">Kasus Mutu Aktif</span>
          <span class="text-sm font-extrabold shrink-0" :class="d.open_quality_cases > 0 ? 'text-rose-700' : 'text-gray-800'">{{ d.open_quality_cases }}</span>
        </component>
        <component :is="canViewWastes ? 'router-link' : 'div'" :to="canViewWastes ? '/stock-wastes' : undefined"
          class="op-pill border-gray-200 bg-white" :class="canViewWastes ? 'hover:bg-gray-50' : ''">
          <span class="text-[11px] font-semibold text-gray-600 truncate">Waste 30 Hr ({{ d.waste_count_30d }})</span>
          <span class="text-xs sm:text-sm font-extrabold text-gray-800 shrink-0">{{ fmtRp(d.waste_value_30d) }}</span>
        </component>
      </div>

      <!-- ── 3. BAGIAN PRIORITAS: STOK EXPIRED & AKAN EXPIRED (<= 1 BULAN) ── -->
      <section ref="expirySectionRef" class="panel" :class="d.expiring_30d_count > 0 ? 'border-rose-200' : 'border-emerald-200/80'">
        <!-- Ringkas 1 baris saat tidak ada stok expired <= 30 hari -->
        <div v-if="!(d.expiring_items || []).length" class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between bg-emerald-50/50 px-4 py-3">
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
            </span>
            <div class="min-w-0">
              <p class="text-xs sm:text-sm font-bold text-emerald-900">Stok Expired ≤ 1 Bulan: Aman</p>
              <p class="text-[11px] text-emerald-700/90">Tidak ada batch aktif yang sudah kadaluwarsa atau mendekati kadaluwarsa dalam 30 hari ke depan.</p>
            </div>
          </div>
          <router-link v-if="canViewExpiry" to="/ppic/expiry" class="text-xs font-semibold text-emerald-700 hover:underline whitespace-nowrap shrink-0 self-end sm:self-center">
            Lihat Monitor PPIC →
          </router-link>
        </div>

        <!-- Lengkap saat ada batch expired / <= 30 hari -->
        <template v-else>
          <div class="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between border-b border-gray-100 px-4 py-3">
            <div class="flex items-start gap-2.5 min-w-0">
              <span class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-rose-100 text-rose-700">
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2">
                  <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
                </svg>
              </span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-sm sm:text-base font-bold text-gray-900">Stok Expired &amp; Akan Expired (≤ 1 Bulan)</h2>
                  <span class="rounded-full bg-rose-100 px-2 py-0.5 text-[11px] font-bold text-rose-700">
                    {{ d.expiring_30d_count }} batch · {{ fmtRpFull(d.expiring_30d_value) }}
                  </span>
                </div>
                <p class="text-[11px] sm:text-xs text-gray-500 mt-0.5">Urut paling mendesak (FEFO). Prioritaskan pemakaian atau tindak lanjut karantina.</p>
              </div>
            </div>
            <router-link v-if="canViewExpiry" to="/ppic/expiry" class="text-xs font-semibold text-emerald-700 hover:underline whitespace-nowrap shrink-0">
              Monitor PPIC →
            </router-link>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 bg-gray-50/70 px-4 py-2.5">
            <div class="flex flex-wrap items-center gap-1.5">
              <button v-for="tab in expiryTabs" :key="tab.key" type="button" @click="expiryFilter = tab.key"
                :class="[
                  'inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-semibold transition cursor-pointer',
                  expiryFilter === tab.key ? tab.activeCls : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-100',
                ]">
                <span>{{ tab.label }}</span>
                <span class="rounded-full px-1.5 text-[10px] font-bold" :class="expiryFilter === tab.key ? 'bg-white/25 text-white' : 'bg-gray-100 text-gray-700'">
                  {{ tab.count }}
                </span>
              </button>
            </div>
            <input
              v-model.trim="expirySearch"
              type="search"
              placeholder="Cari bahan atau gudang…"
              class="w-full sm:w-52 rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-xs focus:border-emerald-500 focus:outline-none"
            />
          </div>

          <div v-if="!filteredExpiringItems.length" class="px-4 py-8 text-center text-xs text-gray-400">
            Tidak ada batch yang cocok dengan filter tersebut.
          </div>
          <template v-else>
            <!-- Mobile Cards -->
            <ul class="sm:hidden divide-y divide-gray-100">
              <li v-for="b in filteredExpiringItems" :key="b.batch_id" class="p-3.5 space-y-1.5">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="text-sm font-bold text-gray-900 break-words">{{ b.item_name }}</p>
                    <p class="text-[11px] text-gray-500">{{ b.warehouse_name }}<span v-if="b.item_code"> · {{ b.item_code }}</span></p>
                  </div>
                  <span :class="expiryBadgeClass(b.status)" class="shrink-0 whitespace-nowrap rounded-full border px-2 py-0.5 text-[10px] font-bold">
                    {{ expiryDaysLabel(b.days_left) }}
                  </span>
                </div>
                <div class="flex flex-wrap items-center justify-between gap-2 pt-1 text-xs">
                  <div class="text-gray-600">
                    Exp: <strong class="font-mono text-gray-900">{{ fmtDate(b.expiry_date + 'T00:00:00+07:00') }}</strong>
                    · Sisa: <strong class="text-gray-900">{{ stockQtyText(b, b.qty_base, b.warehouse_type) }}</strong>
                  </div>
                  <div class="font-bold" :class="b.status === 'expired' ? 'text-rose-700' : 'text-amber-800'">
                    {{ fmtRpFull(b.stock_value) }}
                  </div>
                </div>
                <p v-if="b.ack_by" class="text-[10px] text-emerald-700">✓ Sudah ditindak PPIC ({{ b.ack_by }})</p>
              </li>
            </ul>

            <!-- Desktop Table -->
            <div class="hidden sm:block overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-100 text-xs">
                <thead class="bg-gray-50 text-[11px] uppercase tracking-wide text-gray-500">
                  <tr>
                    <th class="px-4 py-2.5 text-left font-bold">Item Bahan</th>
                    <th class="px-3 py-2.5 text-left font-bold">Gudang</th>
                    <th class="px-3 py-2.5 text-left font-bold">Tgl Kadaluwarsa</th>
                    <th class="px-3 py-2.5 text-left font-bold">Sisa Hari</th>
                    <th class="px-3 py-2.5 text-right font-bold">Sisa Stok</th>
                    <th class="px-3 py-2.5 text-right font-bold">Nilai Risiko</th>
                    <th class="px-4 py-2.5 text-left font-bold">Tindak Lanjut</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white">
                  <tr v-for="b in filteredExpiringItems" :key="b.batch_id"
                    :class="b.status === 'expired' ? 'bg-rose-50/40 hover:bg-rose-50/80' : 'hover:bg-gray-50'">
                    <td class="px-4 py-2.5">
                      <p class="font-semibold text-gray-900">{{ b.item_name }}</p>
                      <p v-if="b.item_code || b.category" class="text-[10px] text-gray-400">
                        {{ [b.item_code, b.category].filter(Boolean).join(' · ') }}
                      </p>
                    </td>
                    <td class="px-3 py-2.5 text-gray-600 whitespace-nowrap">{{ b.warehouse_name }}</td>
                    <td class="px-3 py-2.5 font-mono font-semibold text-gray-800 whitespace-nowrap">
                      {{ fmtDate(b.expiry_date + 'T00:00:00+07:00') }}
                    </td>
                    <td class="px-3 py-2.5 whitespace-nowrap">
                      <span :class="expiryBadgeClass(b.status)" class="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-bold">
                        {{ expiryDaysLabel(b.days_left) }}
                      </span>
                    </td>
                    <td class="px-3 py-2.5 text-right font-mono font-semibold text-gray-900 whitespace-nowrap">
                      {{ stockQtyText(b, b.qty_base, b.warehouse_type) }}
                    </td>
                    <td class="px-3 py-2.5 text-right font-semibold whitespace-nowrap" :class="b.status === 'expired' ? 'text-rose-700' : 'text-amber-800'">
                      {{ fmtRpFull(b.stock_value) }}
                    </td>
                    <td class="px-4 py-2.5 whitespace-nowrap">
                      <span v-if="b.ack_by" class="inline-flex items-center rounded-full bg-emerald-50 border border-emerald-200 px-2 py-0.5 text-[10px] font-semibold text-emerald-700">
                        ✓ Ditindak ({{ b.ack_by }})
                      </span>
                      <span v-else class="text-[11px] text-gray-400">Belum ditindak</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </template>
      </section>

      <!-- ── 4. PERLU TINDAK LANJUT: Stok Rendah/Habis & Transfer Pending (items-start, tidak saling tarik tinggi!) ── -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 items-start">

        <!-- Stok Rendah / Habis -->
        <section class="panel">
          <div class="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
            <div class="flex items-center gap-2 min-w-0">
              <h2 class="text-sm font-bold text-gray-900 truncate">Stok Rendah / Habis</h2>
              <span v-if="d.low_stock_count > 0" class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-bold text-amber-800">
                {{ d.low_stock_count }}
              </span>
            </div>
            <router-link v-if="canViewLedger" to="/stock-ledger" class="text-xs font-semibold text-emerald-700 hover:underline shrink-0">Lihat Semua →</router-link>
          </div>

          <div v-if="!d.low_stock_items.length" class="flex items-center justify-center gap-1.5 px-4 py-7 text-xs font-semibold text-emerald-700">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            Semua stok mencukupi batas minimum
          </div>
          <template v-else>
            <!-- Mobile Cards -->
            <ul class="sm:hidden divide-y divide-gray-100 max-h-[360px] overflow-y-auto">
              <li v-for="r in d.low_stock_items" :key="r.item_id + r.warehouse_id" class="p-3.5 space-y-1.5">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <p class="text-sm font-bold text-gray-900 break-words">{{ r.item_name }}</p>
                    <p class="text-[11px] text-gray-500">{{ r.warehouse_name }}</p>
                  </div>
                  <span :class="r.qty_base <= 0 ? 'bg-rose-100 text-rose-700' : 'bg-amber-100 text-amber-800'"
                    class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-bold">
                    {{ r.qty_base <= 0 ? 'Habis' : 'Rendah' }}
                  </span>
                </div>
                <div class="flex items-center justify-between text-xs">
                  <span :class="r.qty_base <= 0 ? 'text-rose-700' : 'text-amber-700'" class="font-mono font-bold">
                    Stok: {{ stockQtyText(r, r.qty_base, r.warehouse_type) }}
                  </span>
                  <span class="font-mono text-gray-500">Min: {{ stockQtyText(r, r.min_stock, r.warehouse_type) }}</span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-100">
                  <div :class="r.qty_base <= 0 ? 'bg-rose-500' : 'bg-amber-500'" class="h-full rounded-full" :style="{ width: lowStockPct(r) + '%' }" />
                </div>
              </li>
            </ul>

            <!-- Desktop Table -->
            <div class="hidden sm:block overflow-x-auto max-h-[360px] overflow-y-auto">
              <table class="min-w-full divide-y divide-gray-100 text-xs">
                <thead class="bg-gray-50 text-[11px] uppercase tracking-wide text-gray-500 sticky top-0">
                  <tr>
                    <th class="px-4 py-2.5 text-left font-bold">Item</th>
                    <th class="px-3 py-2.5 text-left font-bold">Gudang</th>
                    <th class="px-3 py-2.5 text-right font-bold">Stok</th>
                    <th class="px-3 py-2.5 text-right font-bold">Min</th>
                    <th class="px-4 py-2.5 text-left font-bold">Status</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 bg-white">
                  <tr v-for="r in d.low_stock_items" :key="r.item_id + r.warehouse_id" class="hover:bg-gray-50">
                    <td class="px-4 py-2.5 font-semibold text-gray-900 max-w-[180px] truncate" :title="r.item_name">{{ r.item_name }}</td>
                    <td class="px-3 py-2.5 text-gray-500 max-w-[130px] truncate" :title="r.warehouse_name">{{ r.warehouse_name }}</td>
                    <td class="px-3 py-2.5 text-right font-mono font-bold whitespace-nowrap" :class="r.qty_base <= 0 ? 'text-rose-600' : 'text-amber-700'">
                      {{ stockQtyText(r, r.qty_base, r.warehouse_type) }}
                    </td>
                    <td class="px-3 py-2.5 text-right font-mono text-gray-600 whitespace-nowrap">
                      {{ stockQtyText(r, r.min_stock, r.warehouse_type) }}
                    </td>
                    <td class="px-4 py-2.5 whitespace-nowrap">
                      <span :class="r.qty_base <= 0 ? 'bg-rose-100 text-rose-700' : 'bg-amber-100 text-amber-800'"
                        class="rounded-full px-2 py-0.5 text-[10px] font-bold">
                        {{ r.qty_base <= 0 ? 'Habis' : 'Rendah' }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </section>

        <!-- Transfer Pending -->
        <section class="panel">
          <div class="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
            <div class="flex items-center gap-2 min-w-0">
              <h2 class="text-sm font-bold text-gray-900 truncate">Transfer Stok Pending</h2>
              <span v-if="d.pending_transfers > 0" class="rounded-full bg-orange-100 px-2 py-0.5 text-[10px] font-bold text-orange-800">
                {{ d.pending_transfers }}
              </span>
            </div>
            <router-link v-if="canViewTransfers" to="/stock-transfers" class="text-xs font-semibold text-emerald-700 hover:underline shrink-0">Lihat Semua →</router-link>
          </div>
          <div v-if="!d.pending_transfer_list.length" class="flex items-center justify-center gap-1.5 px-4 py-7 text-xs font-semibold text-emerald-700">
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            Tidak ada transfer tertunda
          </div>
          <div v-else class="divide-y divide-gray-100 p-2 max-h-[360px] overflow-y-auto">
            <div v-for="t in d.pending_transfer_list" :key="t.id" class="rounded-lg px-3 py-2.5 hover:bg-gray-50 space-y-1">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="font-mono text-xs font-bold text-gray-900">{{ t.transfer_number }}</span>
                <span :class="statusClass(t.status)" class="rounded-full px-2 py-0.5 text-[10px] font-bold whitespace-nowrap">
                  {{ statusLabel(t.status) }}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-1.5 text-xs text-gray-700">
                <span class="font-semibold">{{ t.from_warehouse_name ?? t.from_warehouse }}</span>
                <span class="text-gray-400">→</span>
                <span class="font-semibold">{{ t.to_warehouse_name ?? t.to_warehouse }}</span>
                <span v-if="t.target_division" :class="['inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wide', transferDivisionClass(t.target_division)]">
                  {{ t.target_division }}
                </span>
              </div>
              <p class="text-[11px] text-gray-400">
                {{ t.transfer_date ? fmtDate(t.transfer_date + 'T00:00:00+07:00') : fmtDate(t.created_at) }}
                <span v-if="t.created_by"> · {{ t.created_by }}</span>
              </p>
            </div>
          </div>
        </section>
      </div>

      <!-- ── 5. GRAFIK PERGERAKAN STOK 14 HARI (Full-Width, Tinggi Proporsional Chart.js) ── -->
      <section class="panel">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-b border-gray-100 px-4 py-3">
          <div>
            <h2 class="text-sm font-bold text-gray-900">Pergerakan Stok 14 Hari Terakhir</h2>
            <p class="text-[11px] text-gray-400">Perbandingan frekuensi transaksi stok masuk dan keluar harian</p>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-xs font-semibold">
            <span class="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 border border-emerald-200 px-2.5 py-0.5 text-emerald-800">
              <span class="h-2 w-2 rounded-full bg-emerald-500" />
              Masuk: {{ trendTotalIn }}
            </span>
            <span class="inline-flex items-center gap-1.5 rounded-full bg-rose-50 border border-rose-200 px-2.5 py-0.5 text-rose-700">
              <span class="h-2 w-2 rounded-full bg-rose-500" />
              Keluar: {{ trendTotalOut }}
            </span>
          </div>
        </div>
        <div v-if="noTrend" class="px-4 py-8 text-center text-xs text-gray-400">
          Belum ada pergerakan stok dalam 14 hari terakhir.
        </div>
        <div v-else class="p-3 sm:p-4">
          <div class="h-[200px] sm:h-[230px] w-full">
            <Bar :data="trendChartData" :options="trendChartOptions" />
          </div>
        </div>
      </section>

      <!-- ── 6. SEBARAN STOK PER GUDANG (Grid 3 Kolom — Hemat Ruang & Mudah Dibandingkan!) ── -->
      <section class="panel">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
          <div class="min-w-0">
            <h2 class="text-sm font-bold text-gray-900">Sebaran Stok per Gudang ({{ d.warehouse_stocks.length }})</h2>
            <p class="text-[11px] text-gray-400">Ketuk kartu gudang untuk memfilter seluruh angka dashboard ke gudang tersebut</p>
          </div>
          <button v-if="selectedWarehouseId" type="button" @click="selectWarehouse('')"
            class="rounded-lg border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-800 hover:bg-emerald-100 cursor-pointer shrink-0">
            Tampilkan Semua Gudang
          </button>
        </div>
        <div v-if="!d.warehouse_stocks.length" class="p-8 text-center text-xs text-gray-400">Belum ada data stok</div>
        <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5 p-3 sm:p-4">
          <button
            v-for="w in d.warehouse_stocks"
            :key="w.warehouse_id"
            type="button"
            @click="selectWarehouse(selectedWarehouseId === w.warehouse_id ? '' : w.warehouse_id)"
            :class="[
              'text-left rounded-xl border p-3 transition cursor-pointer space-y-2',
              selectedWarehouseId === w.warehouse_id
                ? 'border-emerald-500 bg-emerald-50/70 ring-1 ring-emerald-400'
                : 'border-gray-200 bg-white hover:border-emerald-300 hover:bg-gray-50/70',
            ]"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <div class="flex items-center gap-1.5">
                  <span :class="w.warehouse_type === 'central' ? 'bg-purple-100 text-purple-700' : 'bg-blue-100 text-blue-700'"
                    class="rounded px-1.5 py-0.5 text-[10px] font-bold uppercase shrink-0">
                    {{ w.warehouse_type === 'central' ? 'Induk' : 'Outlet' }}
                  </span>
                  <p class="text-xs sm:text-sm font-bold text-gray-900 truncate">{{ w.warehouse_name }}</p>
                </div>
                <p v-if="w.outlet_name && w.outlet_name !== '—'" class="mt-0.5 text-[11px] text-gray-400 truncate">{{ w.outlet_name }}</p>
              </div>
              <div class="text-right shrink-0">
                <p class="text-xs sm:text-sm font-extrabold text-gray-900">{{ fmtRp(w.total_value) }}</p>
                <p class="text-[11px] text-gray-500">{{ w.total_items }} item</p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-100">
                <div class="h-full rounded-full bg-emerald-500 transition-all" :style="{ width: warehouseValuePct(w.total_value) + '%' }" />
              </div>
              <span v-if="w.expiring_30d_count > 0" class="rounded-full bg-rose-100 px-1.5 py-0.2 text-[10px] font-bold text-rose-700 whitespace-nowrap">
                Exp {{ w.expiring_30d_count }}
              </span>
              <span v-if="w.low_stock_count > 0" class="rounded-full bg-amber-100 px-1.5 py-0.2 text-[10px] font-bold text-amber-800 whitespace-nowrap">
                Rendah {{ w.low_stock_count }}
              </span>
            </div>
          </button>
        </div>
      </section>

      <!-- ── 7. PERGERAKAN STOK TERKINI (Full-Width, Mobile Cards + Desktop Table) ── -->
      <section class="panel">
        <div class="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between border-b border-gray-100 px-4 py-3">
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-bold text-gray-900">Pergerakan Stok Terkini</h2>
            <span class="text-xs text-gray-400">({{ filteredMovements.length }})</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 text-xs">
              <button v-for="opt in MOV_FILTERS" :key="opt.key" type="button" @click="movFilter = opt.key"
                :class="movFilter === opt.key ? 'bg-white text-gray-900 shadow-2xs font-semibold' : 'text-gray-500 hover:text-gray-700'"
                class="rounded-md px-2.5 py-1 transition cursor-pointer">
                {{ opt.label }}
              </button>
            </div>
            <router-link v-if="canViewLedger" to="/stock-ledger" class="text-xs font-semibold text-emerald-700 hover:underline whitespace-nowrap">Buku Stok →</router-link>
          </div>
        </div>

        <div v-if="!filteredMovements.length" class="p-8 text-center text-xs text-gray-400">
          Belum ada pergerakan stok yang cocok.
        </div>
        <template v-else>
          <!-- Mobile Card List -->
          <ul class="sm:hidden divide-y divide-gray-100">
            <li v-for="m in filteredMovements" :key="m.id" class="p-3.5 space-y-1.5">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="text-sm font-bold text-gray-900 break-words">{{ m.item_name }}</p>
                  <p class="text-[11px] text-gray-500">{{ m.warehouse_name }}</p>
                </div>
                <div class="text-right shrink-0">
                  <p class="font-mono text-sm font-bold" :class="m.qty_base >= 0 ? 'text-emerald-700' : 'text-rose-600'">
                    {{ movementQtyParts(m).primary }}
                  </p>
                  <p v-if="movementQtyParts(m).secondary" class="font-mono text-[10px] text-gray-400">
                    {{ movementQtyParts(m).secondary }}
                  </p>
                </div>
              </div>
              <div class="flex flex-wrap items-center justify-between gap-1.5 text-[11px]">
                <div class="flex flex-wrap items-center gap-1.5">
                  <span :class="movClass(m.movement_type, m.qty_base)" class="rounded px-1.5 py-0.5 text-[10px] font-bold">
                    {{ movLabel(m.movement_type) }}
                  </span>
                  <span v-if="m.ref_number || m.ref_type" class="font-mono text-gray-500">{{ m.ref_number || m.ref_type }}</span>
                </div>
                <span class="text-gray-400">{{ fmtDateTime(m.created_at) }}<template v-if="m.created_by"> · {{ m.created_by }}</template></span>
              </div>
            </li>
          </ul>

          <!-- Desktop Table -->
          <div class="hidden sm:block overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-100 text-xs">
              <thead class="bg-gray-50 text-[11px] uppercase tracking-wide text-gray-500">
                <tr>
                  <th class="px-4 py-2.5 text-left font-bold">Waktu</th>
                  <th class="px-3 py-2.5 text-left font-bold">Item</th>
                  <th class="px-3 py-2.5 text-left font-bold">Gudang</th>
                  <th class="px-3 py-2.5 text-left font-bold">Tipe</th>
                  <th class="px-3 py-2.5 text-right font-bold">Qty</th>
                  <th class="px-3 py-2.5 text-left font-bold">Referensi</th>
                  <th class="px-4 py-2.5 text-left font-bold">Oleh</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white">
                <tr v-for="m in filteredMovements" :key="m.id" class="hover:bg-gray-50">
                  <td class="px-4 py-2.5 text-gray-500 whitespace-nowrap">{{ fmtDateTime(m.created_at) }}</td>
                  <td class="px-3 py-2.5 font-semibold text-gray-900 max-w-[180px] truncate" :title="m.item_name">{{ m.item_name }}</td>
                  <td class="px-3 py-2.5 text-gray-600 max-w-[140px] truncate" :title="m.warehouse_name">{{ m.warehouse_name }}</td>
                  <td class="px-3 py-2.5 whitespace-nowrap">
                    <span :class="movClass(m.movement_type, m.qty_base)" class="rounded px-2 py-0.5 text-[10px] font-bold">
                      {{ movLabel(m.movement_type) }}
                    </span>
                  </td>
                  <td class="px-3 py-2.5 text-right font-mono font-bold whitespace-nowrap" :class="m.qty_base >= 0 ? 'text-emerald-700' : 'text-rose-600'">
                    <div>{{ movementQtyParts(m).primary }}</div>
                    <div v-if="movementQtyParts(m).secondary" class="text-[10px] font-medium text-gray-400">{{ movementQtyParts(m).secondary }}</div>
                  </td>
                  <td class="px-3 py-2.5 font-mono text-gray-500 max-w-[130px] truncate" :title="m.ref_number || m.ref_type">{{ m.ref_number || m.ref_type || '—' }}</td>
                  <td class="px-4 py-2.5 text-gray-500 whitespace-nowrap">{{ m.created_by || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </section>

    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Tooltip,
  Legend,
} from 'chart.js'
import { apiClient } from '@/api/client.js'
import { useAuthStore } from '@/stores/auth.js'
import { movementLabel, transferStatusLabel, transferDivisionClass } from '@/utils/warehouse.js'
import { stockQtyText } from '@/utils/stockUnits.js'
import { useRealtime } from '@/utils/realtime.js'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip, Legend)

const auth = useAuthStore()
const canViewQuality   = computed(() => auth.hasPermission('stockquality.view'))
const canViewLedger    = computed(() => auth.hasPermission('stockledger.view'))
const canViewTransfers = computed(() => auth.hasPermission('stocktransfers.view'))
const canViewLogistics = computed(() => auth.hasPermission([
  'logistics.deliveries.view', 'logistics.deliveries.manage',
  'logistics.rawmaterials.view', 'logistics.rawmaterials.manage',
  'logistics.goods.view', 'logistics.goods.manage',
  'stockledger.adjust', 'assets.create',
]))
const canViewWastes    = computed(() => auth.hasPermission('stockwastes.view'))
const canViewExpiry    = computed(() => auth.hasPermission('ppic.expiry.view'))

const d                   = ref(null)
const loading             = ref(false)
const err                 = ref('')
const selectedWarehouseId = ref('')
const lastUpdated         = ref('')
const expiryFilter        = ref('all')
const expirySearch        = ref('')
const movFilter           = ref('all')
const expirySectionRef    = ref(null)

const MOV_FILTERS = [
  { key: 'all', label: 'Semua' },
  { key: 'in',  label: 'Masuk' },
  { key: 'out', label: 'Keluar' },
]

onMounted(() => load())
useRealtime(['stock_transfer', 'stock_quality', 'goods_receipt', 'ppic_alert', 'logistics_delivery'], () => load({ silent: true }))

async function load({ silent = false } = {}) {
  if (!silent) { loading.value = true; err.value = '' }
  try {
    const params = selectedWarehouseId.value ? { warehouse_id: selectedWarehouseId.value } : {}
    d.value = await apiClient.get('/admin/warehouse-dashboard', { params })
    lastUpdated.value = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }) + ' WIB'
  } catch (e) {
    if (!silent) err.value = e?.message ?? 'Gagal memuat dashboard gudang'
  } finally {
    if (!silent) loading.value = false
  }
}

function selectWarehouse(id) {
  selectedWarehouseId.value = id
  load()
}

function scrollToExpiry() {
  expirySectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const warehouseOptions = computed(() => d.value?.warehouse_stocks || [])
const selectedWarehouseObj = computed(() =>
  selectedWarehouseId.value ? warehouseOptions.value.find(w => w.warehouse_id === selectedWarehouseId.value) || null : null
)
const totalAllWarehouseValue = computed(() =>
  Math.max(1, warehouseOptions.value.reduce((sum, w) => sum + Math.max(0, Number(w.total_value) || 0), 0))
)
function warehouseValuePct(val) {
  const pct = Math.round(((Number(val) || 0) / totalAllWarehouseValue.value) * 100)
  return Math.max(val > 0 ? 4 : 0, Math.min(100, pct))
}

// ── Expiry <= 1 Bulan ──
const expiryTabs = computed(() => {
  const list = d.value?.expiring_items || []
  const expired  = list.filter(b => b.status === 'expired').length
  const critical = list.filter(b => b.status === 'critical').length
  const warning  = list.filter(b => b.status === 'warning').length
  return [
    { key: 'all',      label: 'Semua ≤ 30 Hr', count: list.length, activeCls: 'border-gray-800 bg-gray-800 text-white' },
    { key: 'expired',  label: 'Sudah Expired', count: expired,     activeCls: 'border-rose-600 bg-rose-600 text-white' },
    { key: 'critical', label: 'Kritis ≤ 7 Hr', count: critical,    activeCls: 'border-orange-600 bg-orange-600 text-white' },
    { key: 'warning',  label: '8–30 Hari',     count: warning,     activeCls: 'border-amber-600 bg-amber-600 text-white' },
  ]
})

const filteredExpiringItems = computed(() => {
  const q = expirySearch.value.toLowerCase()
  return (d.value?.expiring_items || []).filter(b => {
    if (expiryFilter.value !== 'all' && b.status !== expiryFilter.value) return false
    if (q) {
      const hay = `${b.item_name} ${b.item_code || ''} ${b.warehouse_name} ${b.category || ''}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

function expiryBadgeClass(status) {
  if (status === 'expired')  return 'bg-rose-100 text-rose-800 border-rose-300'
  if (status === 'critical') return 'bg-orange-100 text-orange-800 border-orange-300'
  return 'bg-amber-100 text-amber-900 border-amber-300'
}
function expiryDaysLabel(daysLeft) {
  if (daysLeft < 0) return `Expired ${Math.abs(daysLeft)} hr lalu`
  if (daysLeft === 0) return 'Expired Hari Ini!'
  return `H-${daysLeft} (${daysLeft} hr lagi)`
}

// ── Chart.js Trend 14 Hari ──
const noTrend = computed(() => !d.value?.movement_trend?.some(p => p.in > 0 || p.out > 0))
const trendTotalIn  = computed(() => (d.value?.movement_trend || []).reduce((s, p) => s + (p.in || 0), 0))
const trendTotalOut = computed(() => (d.value?.movement_trend || []).reduce((s, p) => s + (p.out || 0), 0))

const trendChartData = computed(() => {
  const pts = d.value?.movement_trend || []
  return {
    labels: pts.map(p => fmtDay(p.date)),
    datasets: [
      {
        label: 'Stok Masuk',
        data: pts.map(p => p.in || 0),
        backgroundColor: 'rgba(16, 185, 129, 0.85)',
        hoverBackgroundColor: 'rgba(5, 150, 105, 1)',
        borderRadius: 4,
        maxBarThickness: 18,
      },
      {
        label: 'Stok Keluar',
        data: pts.map(p => p.out || 0),
        backgroundColor: 'rgba(244, 63, 94, 0.82)',
        hoverBackgroundColor: 'rgba(225, 29, 72, 1)',
        borderRadius: 4,
        maxBarThickness: 18,
      },
    ],
  }
})

const trendChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: '#111827',
      padding: 10,
      cornerRadius: 8,
      titleFont: { size: 12, weight: 'bold' },
      bodyFont: { size: 12 },
      callbacks: {
        label: (ctx) => ` ${ctx.dataset.label}: ${ctx.parsed.y} transaksi`,
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: '#6b7280', font: { size: 10 }, maxRotation: 0, autoSkip: true },
    },
    y: {
      beginAtZero: true,
      grid: { color: '#f3f4f6' },
      border: { display: false },
      ticks: { precision: 0, color: '#9ca3af', font: { size: 10 } },
    },
  },
}

// ── Low stock progress ──
function lowStockPct(r) {
  const min = Number(r?.min_stock) || 0
  const qty = Math.max(0, Number(r?.qty_base) || 0)
  if (min <= 0 || qty <= 0) return 0
  return Math.min(100, Math.round((qty / min) * 100))
}

// ── Movements filter ──
const filteredMovements = computed(() => {
  const list = d.value?.recent_movements || []
  if (movFilter.value === 'in')  return list.filter(m => Number(m.qty_base) > 0)
  if (movFilter.value === 'out') return list.filter(m => Number(m.qty_base) < 0)
  return list
})

function fmtRp(v) {
  if (!v || v === 0) return 'Rp 0'
  if (v >= 1_000_000_000) return 'Rp ' + (v / 1_000_000_000).toFixed(1) + ' M'
  if (v >= 1_000_000)     return 'Rp ' + (v / 1_000_000).toFixed(1) + ' jt'
  return 'Rp ' + Number(v).toLocaleString('id-ID')
}
function fmtRpFull(v) { return 'Rp ' + Math.round(Number(v) || 0).toLocaleString('id-ID') }
function fmtQty(v)    { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 2 }) }
function fmtDay(s)    { return new Date(s + 'T00:00:00+07:00').toLocaleDateString('id-ID', { day: '2-digit', month: 'short' }) }
function fmtDate(s) {
  if (!s) return '—'
  return new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' })
}
function fmtDateTime(s) {
  if (!s) return '—'
  return new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

function movementQtyParts(m) {
  const baseQty = Number(m?.qty_base ?? 0)
  const distQty = Number(m?.qty_dist ?? 0)
  const baseSign = baseQty >= 0 ? '+' : ''
  const baseUnit = (m?.base_unit || m?.unit_used || '').trim()
  const unitUsed = (m?.unit_used || baseUnit).trim()
  const sameUnit = !unitUsed || !baseUnit || unitUsed.toLowerCase() === baseUnit.toLowerCase()
  if (!sameUnit && distQty !== 0) {
    const distSign = distQty >= 0 ? '+' : ''
    return {
      primary: `${distSign}${fmtQty(distQty)} ${unitUsed}`,
      secondary: `${baseSign}${fmtQty(baseQty)} ${baseUnit}`,
    }
  }
  return {
    primary: `${baseSign}${fmtQty(baseQty)} ${baseUnit}`.trim(),
    secondary: '',
  }
}

const STATUS_CLASS = {
  draft: 'bg-gray-100 text-gray-600',
  approved: 'bg-blue-100 text-blue-700',
  sent: 'bg-amber-100 text-amber-800',
  partial_received: 'bg-amber-100 text-amber-800',
  received_rejected: 'bg-orange-100 text-orange-800',
  received: 'bg-emerald-100 text-emerald-700',
  cancelled: 'bg-rose-100 text-rose-700',
}
const statusLabel = transferStatusLabel
function statusClass(s) { return STATUS_CLASS[s] ?? 'bg-gray-100 text-gray-600' }

const MOV_IN  = new Set(['purchase_in','return_in','transfer_in','production_in','breakdown_in','quality_release','supplier_replacement'])
const MOV_OUT = new Set(['sale','waste','spoiled','expired','transfer_out','production_out','breakdown_out','quality_hold'])
const movLabel = movementLabel
function movClass(t, qtyBase = 0) {
  if (t === 'adjustment') return Number(qtyBase) >= 0 ? 'bg-emerald-100 text-emerald-800' : 'bg-rose-100 text-rose-700'
  if (MOV_IN.has(t))  return 'bg-emerald-100 text-emerald-800'
  if (MOV_OUT.has(t)) return 'bg-rose-100 text-rose-700'
  return 'bg-gray-100 text-gray-600'
}
</script>

<style scoped>
.kpi-card {
  display: flex;
  flex-direction: column;
  gap: .35rem;
  padding: .85rem .95rem;
  border-radius: .875rem;
  border-width: 1.5px;
  min-width: 0;
}
.kpi-top {
  display: flex;
  align-items: center;
  gap: .45rem;
  min-width: 0;
}
.kpi-ico {
  width: 1.9rem;
  height: 1.9rem;
  border-radius: .5rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.kpi-lbl {
  font-size: .68rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: .04em;
  color: #4b5563;
  line-height: 1.2;
}
.kpi-val {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1.15;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
@media (min-width: 640px) {
  .kpi-card { padding: 1rem 1.1rem; }
  .kpi-val { font-size: 1.45rem; }
  .kpi-ico { width: 2.2rem; height: 2.2rem; }
}
.kpi-sub {
  font-size: .68rem;
  color: #6b7280;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
}
.op-pill {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: .5rem;
  padding: .6rem .8rem;
  border-radius: .75rem;
  border-width: 1px;
  transition: background-color .15s, border-color .15s;
}
.panel {
  background: #fff;
  border-radius: .875rem;
  border: 1px solid #e5e7eb;
  box-shadow: 0 1px 2px rgba(0, 0, 0, .03);
  overflow: hidden;
}
</style>
