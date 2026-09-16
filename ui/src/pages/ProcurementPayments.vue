<template>
  <div class="space-y-5">
    <!-- Hero Header -->
    <div class="hero-header">
      <div class="hero-bg" />
      <div class="hero-content">
        <div>
          <h1 class="hero-title">Pembayaran</h1>
          <p class="hero-sub">Kelola pembayaran pengadaan per vendor</p>
        </div>
      </div>
    </div>

    <!-- Stat Cards -->
    <div class="grid grid-cols-2 md:grid-cols-5 gap-4">
      <div class="stat-card stat-waiting">
        <div class="stat-icon">
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        </div>
        <div>
          <p class="stat-val">{{ stats.waiting }}</p>
          <p class="stat-lbl">Menunggu Bayar</p>
        </div>
      </div>
      <div class="stat-card stat-total">
        <div class="stat-icon">
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><rect x="2" y="6" width="20" height="12" rx="2"/><circle cx="12" cy="12" r="2"/><path d="M6 12h.01M18 12h.01"/></svg>
        </div>
        <div>
          <p class="stat-val">{{ formatRupiah(stats.totalWaiting) }}</p>
          <p class="stat-lbl">Total Menunggu</p>
        </div>
      </div>
      <div class="stat-card stat-paid">
        <div class="stat-icon">
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        </div>
        <div>
          <p class="stat-val">{{ stats.paid }}</p>
          <p class="stat-lbl">Sudah Dibayar</p>
        </div>
      </div>
      <div class="stat-card stat-received">
        <div class="stat-icon">
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
        </div>
        <div>
          <p class="stat-val">{{ stats.received }}</p>
          <p class="stat-lbl">Diterima</p>
        </div>
      </div>
      <div class="stat-card stat-hutang">
        <div class="stat-icon">
          <svg width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24"><path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2"/><rect x="9" y="3" width="6" height="4" rx="1"/><path d="M9 14l2 2 4-4"/></svg>
        </div>
        <div>
          <p class="stat-val">{{ formatRupiah(stats.accountsPayable) }}</p>
          <p class="stat-lbl">Hutang Usaha</p>
        </div>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1 min-w-[200px] flex-1">
          <label class="text-sm font-medium text-gray-700">Cari</label>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Nomor, nama pengadaan, atau vendor…"
            class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg bg-white/85 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-100 outline-none transition-all"
            @keydown.enter="page = 1; clearSelection(); fetchList()"
          />
        </div>
        <div class="flex flex-col gap-1 min-w-[180px]">
          <label class="text-sm font-medium text-gray-700">Status</label>
          <SearchSelect
            v-model="filterStatus"
            :options="statusFilterOptions"
            placeholder="Semua Status"
            searchPlaceholder="Cari status…"
            valueKey="value"
            labelKey="label"
          />
        </div>
        <div class="flex flex-col gap-1 min-w-[200px]">
          <label class="text-sm font-medium text-gray-700">Projek</label>
          <SearchSelect
            v-model="filterProject"
            :options="projectFilterOptions"
            placeholder="Semua Projek"
            searchPlaceholder="Cari projek…"
          />
        </div>
        <div class="flex flex-col gap-1 min-w-[130px]">
          <label class="text-sm font-medium text-gray-700">Tipe</label>
          <select v-model="filterType" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg bg-white/85 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-100 outline-none transition-all">
            <option v-for="opt in typeFilterOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <button @click="page = 1; clearSelection(); fetchList()"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
        <button @click="downloadExcel" :disabled="exporting"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <svg v-else class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
    </AppCard>

    <!-- Floating Selection Bar -->
    <Transition name="slide-bar">
      <div v-if="selectedIds.size > 0" class="selection-bar">
        <div class="flex items-center gap-3">
          <span class="text-sm font-semibold text-white">{{ selectedIds.size }} pengajuan dipilih</span>
          <span class="text-xs text-emerald-200">Vendor: {{ selectedVendorName || '-' }}</span>
          <span class="text-xs text-emerald-200">Total: {{ formatRupiah(selectedTotal) }}</span>
        </div>
        <div class="flex items-center gap-2">
          <button class="bar-btn bar-cancel" @click="clearSelection">Batal</button>
          <button v-if="canPay" class="bar-btn bar-pay" @click="openBatchPay">Bayar {{ selectedIds.size }} Pengajuan</button>
        </div>
      </div>
    </Transition>

    <!-- List -->
    <AppCard :padding="false">
      <div class="overflow-x-auto rounded-lg border border-gray-200">
        <table class="min-w-full text-sm divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th v-if="filterStatus === 'payment_requested' || filterStatus === 'partial'" class="px-3 py-3 w-10">
                <input type="checkbox" :checked="allSelectableChecked" @change="toggleSelectAll" class="tbl-check" />
              </th>
              <th v-for="col in COLUMNS" :key="col.key" :class="['px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide', col.class ?? '']">
                {{ col.label }}
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-100">
            <template v-if="loading">
              <tr v-for="i in 5" :key="`sk-${i}`">
                <td v-if="filterStatus === 'payment_requested' || filterStatus === 'partial'" class="px-3 py-3"><div class="h-4 w-4 bg-gray-200 rounded animate-pulse" /></td>
                <td v-for="col in COLUMNS" :key="col.key" class="px-4 py-3"><div class="h-4 bg-gray-200 rounded animate-pulse" /></td>
              </tr>
            </template>
            <tr v-else-if="!requests.length">
              <td :colspan="COLUMNS.length + ((filterStatus === 'payment_requested' || filterStatus === 'partial') ? 1 : 0)" class="px-4 py-12 text-center text-gray-400">
                Tidak ada data pembayaran.
              </td>
            </tr>
            <template v-else>
              <tr v-for="row in requests" :key="row.id" class="hover:bg-gray-50 transition-colors" :class="{ 'bg-emerald-50/40': selectedIds.has(row.id) }">
                <td v-if="filterStatus === 'payment_requested' || filterStatus === 'partial'" class="px-3 py-3">
                  <input v-if="row.status === 'payment_requested' || row.status === 'partial'" type="checkbox" :checked="selectedIds.has(row.id)" @change="toggleSelect(row)" class="tbl-check" />
                </td>
                <td class="px-4 py-3 align-middle">
                  <span class="font-mono text-xs text-gray-700">{{ row.request_number || '-' }}</span>
                  <div v-if="row.parent_number" class="text-[10px] text-gray-400 mt-0.5">Induk: {{ row.parent_number }}</div>
                </td>
                <td class="px-4 py-3 align-middle">
                  <span :class="row.request_type === 'barang' ? 'type-badge type-barang' : 'type-badge type-jasa'">
                    {{ row.request_type === 'barang' ? 'Barang' : 'Jasa' }}
                  </span>
                </td>
                <td class="px-4 py-3 align-middle text-gray-800">
                  {{ (row.items || []).map(i => i.name).join(', ') || '-' }}
                  <div v-if="row.project_name" class="mt-0.5">
                    <span class="project-badge">{{ row.project_name }}</span>
                  </div>
                </td>
                <td class="px-4 py-3 align-middle">
                  <div class="font-medium text-gray-900">{{ row.vendor_name || '-' }}</div>
                  <div v-if="row.parent_id" class="mt-0.5">
                    <span class="split-badge">Split</span>
                  </div>
                </td>
                <td class="px-4 py-3 align-middle">{{ row.work_unit_name || '-' }}</td>
                <td class="px-4 py-3 align-middle text-right font-semibold text-emerald-700">
                  {{ formatRupiah(row.total_final) }}
                  <div v-if="row.status === 'partial' && row.paid_amount > 0" class="text-[10px] font-normal text-amber-600 mt-0.5">
                    Sisa: {{ formatRupiah((row.total_final || 0) - (row.paid_amount || 0)) }}
                  </div>
                </td>
                <td class="px-4 py-3 align-middle">
                  <div class="flex flex-col items-start gap-1">
                    <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
                    <span v-if="row.request_type === 'barang'" :class="receiptBadgeCls(row)" :title="receiptHint(row)">
                      {{ receiptLabel(row) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3 align-middle text-gray-700">{{ formatDateTime(row.created_at) }}</td>
                <td class="px-4 py-3 align-middle text-right">
                  <div class="action-btns">
                    <button class="act-view" @click="viewDetail(row)" title="Lihat Detail">
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                    </button>
                    <button v-if="canPay && (row.status === 'payment_requested' || row.status === 'partial')" class="act-pay" @click="openPayModal(row)" title="Bayar">
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="1" y="4" width="22" height="16" rx="2" ry="2"/><line x1="1" y1="10" x2="23" y2="10"/></svg>
                    </button>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
      <div v-if="totalPages > 1" class="px-4 py-3 border-t border-gray-100">
        <AppPagination :page="page" :totalPages="totalPages" @update:page="goPage" />
      </div>
    </AppCard>

    <!-- Detail Modal -->
    <AppModal v-model="showDetail" title="Detail Pengajuan" size="2xl">
      <div v-if="detail" class="space-y-4">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span :class="statusBadge(detail.status)" class="text-sm">{{ statusLabel(detail.status) }}</span>
          <span v-if="detail.request_type === 'barang'" :class="receiptBadgeCls(detail)" class="text-sm">
            {{ receiptLabel(detail) }}
          </span>
          <span v-if="detail.request_number" class="font-mono text-sm text-gray-600">No. {{ detail.request_number }}</span>
          <span :class="detail.request_type === 'barang' ? 'type-badge type-barang' : 'type-badge type-jasa'" class="text-xs">
            {{ detail.request_type === 'barang' ? 'Barang' : 'Jasa' }}
          </span>
          <span class="text-gray-400">•</span>
          <span class="text-gray-500">{{ detail.work_unit_name || '-' }}</span>
          <span class="text-gray-400">•</span>
          <span class="text-gray-500">{{ formatDateTime(detail.created_at) }}</span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <!-- Informasi -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-emerald-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-emerald-600 mb-1.5">Informasi</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Pengaju</dt>
                <dd class="font-medium text-gray-900">{{ detail.requested_by }}</dd>
                <template v-if="detail.work_unit_name">
                  <dt class="text-gray-400">Unit</dt>
                  <dd class="font-medium text-gray-900">{{ detail.work_unit_name }}</dd>
                </template>
                <template v-if="detail.vendor_name">
                  <dt class="text-gray-400">Vendor</dt>
                  <dd class="font-medium text-gray-900">{{ detail.vendor_name }}</dd>
                </template>
                <template v-if="detail.parent_id">
                  <dt class="text-gray-400">Tipe</dt>
                  <dd class="font-medium text-orange-600">Split dari pengadaan induk</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Nilai -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-blue-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-blue-600 mb-1.5">Nilai</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">HPS</dt>
                <dd class="font-medium text-gray-900">{{ formatRupiah(detail.total_hps) }}</dd>
                <dt class="text-gray-400">Harga Final</dt>
                <dd class="font-semibold text-emerald-700">{{ formatRupiah(detail.total_final) }}</dd>
                <template v-if="detail.paid_amount > 0">
                  <dt class="text-gray-400">Terbayar</dt>
                  <dd class="font-semibold text-blue-700">{{ formatRupiah(detail.paid_amount) }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Persetujuan -->
          <div v-if="detail.approved_by" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-amber-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-amber-600 mb-1.5">Persetujuan</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Oleh</dt>
                <dd class="font-medium text-gray-900">{{ detail.approved_by }}</dd>
                <template v-if="detail.approved_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.approved_at) }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Pembayaran -->
          <div v-if="detail.paid_by" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden sm:col-span-2">
            <div class="border-l-4 border-green-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-green-600 mb-1.5">Pembayaran</p>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                  <dt class="text-gray-400">Oleh</dt>
                  <dd class="font-medium text-gray-900">{{ detail.paid_by }}</dd>
                  <template v-if="detail.payment_account_dest">
                    <dt class="text-gray-400">Rek Tujuan</dt>
                    <dd class="font-medium font-mono text-gray-900">{{ detail.payment_account_dest }}</dd>
                  </template>
                  <template v-if="detail.payment_account_source">
                    <dt class="text-gray-400">Rek Asal</dt>
                    <dd class="font-medium font-mono text-gray-900">{{ detail.payment_account_source }}</dd>
                  </template>
                  <template v-if="parsedPaymentProof(detail.payment_proof).refNumber">
                    <dt class="text-gray-400">No. Referensi</dt>
                    <dd class="font-medium font-mono text-gray-900">{{ parsedPaymentProof(detail.payment_proof).refNumber }}</dd>
                  </template>
                  <template v-if="detail.payment_notes">
                    <dt class="text-gray-400">Catatan</dt>
                    <dd class="text-gray-700">{{ detail.payment_notes }}</dd>
                  </template>
                  <template v-if="detail.paid_at">
                    <dt class="text-gray-400">Waktu</dt>
                    <dd class="text-gray-600">{{ formatDateTime(detail.paid_at) }}</dd>
                  </template>
                </dl>
                <div v-if="parsedPaymentProof(detail.payment_proof).fileUrl" class="flex items-start justify-center">
                  <a :href="parsedPaymentProof(detail.payment_proof).fileUrl" target="_blank" class="block group">
                    <img v-if="parsedPaymentProof(detail.payment_proof).isImage" :src="parsedPaymentProof(detail.payment_proof).fileUrl" class="max-h-40 rounded-lg border border-gray-200 shadow-sm group-hover:shadow-md transition-shadow cursor-pointer" alt="Bukti transfer" />
                    <div v-else class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 group-hover:bg-gray-100 transition-colors">
                      <svg class="w-6 h-6 text-red-400 shrink-0" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/></svg>
                      <span class="text-xs font-medium text-blue-600 underline">Lihat Bukti Transfer (PDF)</span>
                    </div>
                  </a>
                </div>
              </div>
            </div>
          </div>

          <!-- No. Invoice & Catatan -->
          <div v-if="detail.invoice_number || detail.notes" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden col-span-1 sm:col-span-2">
            <div class="border-l-4 border-gray-400 px-3 py-2.5">
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="detail.invoice_number">
                  <dt class="text-gray-400">No. Invoice</dt>
                  <dd class="font-medium font-mono text-gray-900">{{ detail.invoice_number }}</dd>
                </template>
                <template v-if="detail.notes">
                  <dt class="text-gray-400">Catatan</dt>
                  <dd class="text-gray-700">{{ detail.notes }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Progress Pembayaran -->
          <div v-if="detail.paid_amount > 0 || detail.status === 'partial'" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden col-span-1 sm:col-span-2">
            <div class="border-l-4 border-amber-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-amber-600 mb-2">Progress Pembayaran</p>
              <div class="flex items-center gap-3 mb-1.5">
                <div class="flex-1 h-2 rounded-full bg-gray-200 overflow-hidden">
                  <div class="h-full rounded-full transition-all" :class="detail.paid_amount >= detail.total_final ? 'bg-emerald-500' : 'bg-amber-500'" :style="{ width: Math.min(100, ((detail.paid_amount || 0) / (detail.total_final || 1)) * 100) + '%' }" />
                </div>
                <span class="text-xs font-semibold" :class="detail.paid_amount >= detail.total_final ? 'text-emerald-600' : 'text-amber-600'">{{ Math.round(((detail.paid_amount || 0) / (detail.total_final || 1)) * 100) }}%</span>
              </div>
              <div class="flex justify-between text-xs text-gray-500">
                <span>Dibayar: <span class="font-semibold text-gray-700">{{ formatRupiah(detail.paid_amount || 0) }}</span></span>
                <span>Sisa: <span class="font-semibold" :class="detail.paid_amount >= detail.total_final ? 'text-emerald-600' : 'text-amber-600'">{{ formatRupiah((detail.total_final || 0) - (detail.paid_amount || 0)) }}</span></span>
              </div>
            </div>
          </div>
        </div>

        <!-- Histori Pembayaran -->
        <div v-if="paymentHistories.length > 0" class="mt-2">
          <p class="text-sm font-semibold text-gray-700 mb-2">Histori Pembayaran ({{ paymentHistories.length }})</p>
          <div class="space-y-2">
            <div v-for="(ph, idx) in paymentHistories" :key="ph.id" class="rounded-lg border border-gray-200 bg-gray-50 p-3">
              <div class="flex items-center justify-between mb-1">
                <span class="text-xs font-semibold text-gray-700">Pembayaran #{{ idx + 1 }}</span>
                <span class="text-xs text-gray-500">{{ formatDateTime(ph.created_at) }}</span>
              </div>
              <div class="grid grid-cols-2 gap-x-4 gap-y-0.5 text-xs">
                <div>
                  <span class="text-gray-400">Jumlah:</span>
                  <span class="font-semibold text-emerald-700 ml-1">{{ formatRupiah(ph.amount) }}</span>
                </div>
                <div v-if="ph.paid_by">
                  <span class="text-gray-400">Oleh:</span>
                  <span class="font-medium text-gray-700 ml-1">{{ ph.paid_by }}</span>
                </div>
                <div v-if="ph.payment_account_dest">
                  <span class="text-gray-400">Rek Tujuan:</span>
                  <span class="font-mono text-gray-700 ml-1">{{ ph.payment_account_dest }}</span>
                </div>
                <div v-if="ph.payment_account_source">
                  <span class="text-gray-400">Rek Asal:</span>
                  <span class="font-mono text-gray-700 ml-1">{{ ph.payment_account_source }}</span>
                </div>
                <div v-if="parsedPaymentProof(ph.payment_proof).refNumber">
                  <span class="text-gray-400">Ref:</span>
                  <span class="font-mono text-gray-700 ml-1">{{ parsedPaymentProof(ph.payment_proof).refNumber }}</span>
                </div>
                <div v-if="ph.payment_notes" class="col-span-2">
                  <span class="text-gray-400">Catatan:</span>
                  <span class="text-gray-700 ml-1">{{ ph.payment_notes }}</span>
                </div>
              </div>
              <div v-if="parsedPaymentProof(ph.payment_proof).fileUrl" class="mt-2">
                <a :href="parsedPaymentProof(ph.payment_proof).fileUrl" target="_blank" class="inline-flex items-center gap-1.5 text-xs text-blue-600 hover:text-blue-800 font-medium">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>
                  Lihat Bukti Transfer
                </a>
              </div>
            </div>
          </div>
        </div>

        <!-- Items -->
        <div>
          <p class="text-sm font-semibold text-gray-700 mb-2">Daftar Item ({{ detailItems.length }})</p>
          <div class="space-y-3">
            <div v-for="(item, i) in detailItems" :key="i" class="bg-gray-50 rounded-lg p-3 border border-gray-200">
              <div class="flex items-center justify-between mb-2">
                <p class="text-sm font-semibold text-gray-900">{{ i + 1 }}. {{ item.name }}</p>
                <span class="text-xs text-emerald-700 font-medium">{{ formatRupiah(item.final_total || 0) }}</span>
              </div>
              <table class="w-full text-xs">
                <thead>
                  <tr class="border-b border-gray-200 text-left text-gray-500 uppercase">
                    <th class="py-1.5 pr-2">#</th>
                    <th class="py-1.5 pr-2">Item</th>
                    <th class="py-1.5 pr-2 text-right">Qty</th>
                    <th class="py-1.5 pr-2">Satuan</th>
                    <th class="py-1.5 pr-2 text-right">Harga</th>
                    <th class="py-1.5 text-right">Total</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(sub, j) in item.items" :key="j" class="border-b border-gray-100">
                    <td class="py-1.5 pr-2 text-gray-400">{{ j + 1 }}</td>
                    <td class="py-1.5 pr-2 text-gray-800">{{ sub.name }}</td>
                    <td class="py-1.5 pr-2 text-right">{{ sub.qty }}</td>
                    <td class="py-1.5 pr-2">{{ sub.unit }}</td>
                    <td class="py-1.5 pr-2 text-right">{{ formatRupiah(sub.final_price || 0) }}</td>
                    <td class="py-1.5 text-right font-medium">{{ formatRupiah((sub.qty || 0) * (sub.final_price || 0)) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div class="flex justify-end mt-3 text-sm">
            <span class="text-emerald-700 font-semibold">Total: {{ formatRupiah(detail.total_final) }}</span>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton v-if="canPay && detail && (detail.status === 'payment_requested' || detail.status === 'partial')" variant="primary" @click="showDetail = false; openPayModal(detail)">Bayar</AppButton>
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>

    <!-- Payment Modal -->
    <AppModal v-model="showPayModal" :title="payTargets.length > 1 ? `Pembayaran ${payTargets.length} Pengajuan` : 'Pembayaran Pengajuan'" size="xl">
      <div class="space-y-4">
        <!-- Two cards side by side -->
        <div class="grid grid-cols-2 gap-3">
          <!-- Card 1: Nominal & Vendor -->
          <div class="rounded-lg border border-gray-200 bg-white overflow-hidden">
            <div class="bg-emerald-50 px-4 py-2.5 border-b border-emerald-100">
              <p class="text-[10px] text-gray-500 uppercase tracking-wider font-medium">Total Pembayaran</p>
              <p class="text-xl font-bold text-emerald-700 mt-0.5">{{ formatRupiah(payTotalAmount) }}</p>
              <template v-if="payTargets.length === 1 && payTargets[0]?.paid_amount > 0">
                <div class="flex items-center gap-2 mt-1.5">
                  <div class="flex-1 h-1.5 rounded-full bg-gray-200 overflow-hidden">
                    <div class="h-full rounded-full bg-emerald-500 transition-all" :style="{ width: Math.min(100, (payTargets[0].paid_amount / payTargets[0].total_final) * 100) + '%' }" />
                  </div>
                  <span class="text-[10px] text-gray-500">{{ Math.round((payTargets[0].paid_amount / payTargets[0].total_final) * 100) }}%</span>
                </div>
                <p class="text-[10px] text-gray-500 mt-0.5">Sudah dibayar: {{ formatRupiah(payTargets[0].paid_amount) }} · Sisa: <span class="font-semibold text-amber-600">{{ formatRupiah(payRemainingAmount) }}</span></p>
              </template>
            </div>
            <div class="px-4 py-2.5">
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
                <dt class="text-gray-400">Vendor</dt>
                <dd class="font-medium text-gray-800 truncate">{{ payTargets[0]?.vendor_name || '-' }}</dd>
                <template v-if="payTargets.length === 1">
                  <dt class="text-gray-400">Unit Kerja</dt>
                  <dd class="font-medium text-gray-800 truncate">{{ payTargets[0]?.work_unit_name || '-' }}</dd>
                </template>
                <template v-if="payTargets.length === 1 && payTargets[0]?.invoice_number">
                  <dt class="text-gray-400">Invoice</dt>
                  <dd class="font-medium font-mono text-gray-800">{{ payTargets[0].invoice_number }}</dd>
                </template>
                <template v-if="payTargets.length > 1">
                  <dt class="text-gray-400">Pengajuan</dt>
                  <dd class="font-medium text-gray-800">{{ payTargets.length }} item</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Card 2: Bank Info -->
          <div class="rounded-lg border border-gray-200 bg-white overflow-hidden">
            <div class="bg-blue-50 px-4 py-2.5 border-b border-blue-100">
              <p class="text-[10px] text-gray-500 uppercase tracking-wider font-medium">Rekening Vendor</p>
            </div>
            <div v-if="payVendor" class="px-4 py-2.5">
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
                <dt class="text-gray-400">Bank</dt>
                <dd class="font-medium text-gray-800">{{ payVendor.bank_name || '-' }}</dd>
                <dt class="text-gray-400">No. Rek</dt>
                <dd class="font-medium font-mono text-gray-800 flex items-center gap-1">
                  {{ payVendor.account_number || '-' }}
                  <button v-if="payVendor.account_number" @click="copyText(payVendor.account_number, 'No. Rekening')"
                    class="inline-flex items-center justify-center w-5 h-5 rounded hover:bg-blue-100 text-blue-400 hover:text-blue-600 transition-colors" title="Salin">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
                  </button>
                </dd>
                <dt class="text-gray-400">Atas Nama</dt>
                <dd class="font-medium text-gray-800 flex items-center gap-1">
                  {{ payVendor.account_holder || '-' }}
                  <button v-if="payVendor.account_holder" @click="copyText(payVendor.account_holder, 'Atas Nama')"
                    class="inline-flex items-center justify-center w-5 h-5 rounded hover:bg-blue-100 text-blue-400 hover:text-blue-600 transition-colors" title="Salin">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
                  </button>
                </dd>
              </dl>
            </div>
            <div v-else-if="payVendorLoading" class="px-4 py-4 text-xs text-gray-400 text-center">Memuat...</div>
            <div v-else class="px-4 py-3">
              <p class="text-xs text-amber-600">Belum tersedia. Isi rekening tujuan manual.</p>
            </div>
          </div>
        </div>

        <!-- Batch detail (only for multi) -->
        <div v-if="payTargets.length > 1" class="rounded-lg bg-gray-50 border border-gray-100 px-4 py-2.5 space-y-1">
          <p class="text-[10px] font-semibold uppercase tracking-wider text-gray-500 mb-1">Rincian {{ payTargets.length }} pengajuan</p>
          <div v-for="t in payTargets" :key="t.id" class="flex justify-between text-xs">
            <span class="text-gray-500 truncate mr-3">{{ t.request_number }} — {{ (t.items || []).map(i => i.name).join(', ') || t.work_unit_name || '-' }}</span>
            <span class="font-medium text-emerald-700 whitespace-nowrap">{{ formatRupiah(t.total_final) }}</span>
          </div>
        </div>

        <!-- Divider -->
        <hr class="border-gray-100" />

        <!-- Form 2-col -->
        <div class="grid grid-cols-2 gap-x-4 gap-y-3">
          <AppInput v-model="payForm.payment_account_dest" label="Rekening Tujuan" placeholder="Otomatis dari vendor, atau isi manual" />
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Rekening Asal (Perusahaan)</label>
            <SearchSelect
              v-model="payForm.payment_account_source"
              :options="bankAccountOptions"
              placeholder="Pilih rekening…"
              searchPlaceholder="Cari rekening…"
              valueKey="id"
              labelKey="name"
            />
          </div>
          <div v-if="payTargets.length === 1">
            <label class="block text-sm font-medium text-gray-700 mb-1">Jumlah Bayar</label>
            <RupiahInput v-model="payForm.payment_amount" :placeholder="`Kosongkan = bayar penuh (${formatRupiah(payRemainingAmount)})`" />
            <p class="text-[10px] text-gray-400 mt-0.5">Sisa tagihan: {{ formatRupiah(payRemainingAmount) }}</p>
          </div>
          <AppInput v-model="payForm.payment_proof" label="No. Referensi Transfer" placeholder="Contoh: TRF-20260410-001" />
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Keterangan</label>
            <textarea v-model="payForm.payment_notes" rows="1" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" placeholder="Opsional"></textarea>
          </div>
        </div>

        <!-- Upload -->
        <div v-if="uploadLoading" class="flex items-center gap-2 text-xs text-gray-500 py-1">
          <svg class="w-4 h-4 animate-spin text-emerald-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
          Mengunggah...
        </div>
        <div v-else-if="!uploadedFileUrl" class="relative">
          <label class="flex items-center gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50/50 hover:bg-gray-100 hover:border-emerald-400 cursor-pointer transition-colors px-4 py-2.5">
            <svg class="w-5 h-5 text-gray-400 shrink-0" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5"/></svg>
            <span class="text-xs text-gray-500">Upload bukti transfer <span class="text-gray-400">(opsional) — JPG, PNG, WEBP, PDF, maks 5MB</span></span>
            <input type="file" class="absolute inset-0 w-full h-full opacity-0 cursor-pointer" accept=".jpg,.jpeg,.png,.webp,.pdf" @change="handleFileUpload" />
          </label>
        </div>
        <div v-else class="flex items-center gap-3 rounded-lg bg-emerald-50 border border-emerald-200 px-4 py-2">
          <div v-if="uploadedFileIsImage" class="shrink-0">
            <img :src="uploadedFileUrl" class="w-9 h-9 rounded object-cover border border-gray-200" />
          </div>
          <div v-else class="shrink-0 w-9 h-9 rounded bg-red-50 border border-red-200 flex items-center justify-center">
            <svg class="w-4.5 h-4.5 text-red-400" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/></svg>
          </div>
          <p class="text-xs font-medium text-gray-700 truncate flex-1 min-w-0">{{ uploadedFileName }}</p>
          <button @click="removeUploadedFile" class="shrink-0 p-1 rounded hover:bg-red-100 text-gray-400 hover:text-red-500 transition-colors" title="Hapus">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
          </button>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showPayModal = false">Batal</AppButton>
        <AppButton :loading="payLoading" :disabled="uploadLoading" @click="submitPay">Konfirmasi Pembayaran</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { vendorsApi } from '@/api/vendors.js'
import { apiClient } from '@/api/client.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
// AppTable not used — custom table for checkbox support
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import RupiahInput   from '@/components/ui/RupiahInput.vue'
import { bankAccountsApi } from '@/api/bankAccounts.js'
import { projectsApi } from '@/api/projects.js'

const toast = useToastStore()
const authStore = useAuthStore()
// Melihat daftar tagihan dan mencairkannya kini izin terpisah.
const canPay = computed(() => authStore.hasPermission('finance.payments.pay'))

const loading = ref(false)
const errorMsg = ref('')
const requests = ref([])
const page = ref(1)
const totalPages = ref(1)
const filterStatus = ref('')
const filterType = ref('')
const filterProject = ref('')
const projects = ref([])

// Keuangan perlu tahu sebuah tagihan bagian dari projek mana, dan perlu bisa
// menyaring "tampilkan semua tagihan projek X" saat menyiapkan pembayaran.
const projectFilterOptions = computed(() => [
  { id: '', name: 'Semua Projek' },
  ...projects.value.map(p => ({ id: p.id, name: p.project_number ? `${p.name} — ${p.project_number}` : p.name })),
])

async function fetchProjects() {
  try {
    const data = await projectsApi.list()
    projects.value = Array.isArray(data) ? data : (data?.data || [])
  } catch { projects.value = [] }
}
const searchQuery = ref('')

const statusFilterOptions = [
  { value: '', label: 'Semua' },
  { value: 'payment_requested', label: 'Menunggu Pembayaran' },
  { value: 'partial', label: 'Dibayar Sebagian' },
  { value: 'paid', label: 'Sudah Dibayar' },
  { value: 'received', label: 'Diterima / Selesai' },
]
const typeFilterOptions = [
  { value: '', label: 'Semua' },
  { value: 'barang', label: 'Barang' },
  { value: 'jasa', label: 'Jasa' },
]

const bankAccounts = ref([])
const bankAccountOptions = computed(() =>
  bankAccounts.value.filter(a => a.is_active).map(a => ({
    id: a.id,
    name: [a.bank_name, a.account_number, a.account_holder ? `a.n. ${a.account_holder}` : ''].filter(Boolean).join(' '),
  }))
)

const stats = ref({ waiting: 0, totalWaiting: 0, paid: 0, received: 0, accountsPayable: 0 })

const showDetail = ref(false)
const detail = ref(null)
const detailItems = ref([])

const selectedIds = ref(new Set())

const showPayModal = ref(false)
const payTargets = ref([])
const payLoading = ref(false)
const payVendor = ref(null)
const payVendorLoading = ref(false)
const payForm = ref({ payment_account_dest: '', payment_account_source: '', payment_proof: '', payment_notes: '', payment_amount: 0 })

const paymentHistories = ref([])
const paymentHistoriesLoading = ref(false)

const uploadLoading = ref(false)
const uploadedFileUrl = ref('')
const uploadedFileName = ref('')
const uploadedFileIsImage = computed(() => /\.(jpg|jpeg|png|webp)$/i.test(uploadedFileName.value))

function parsedPaymentProof(proof) {
  if (!proof) return { refNumber: '', fileUrl: '', isImage: false }
  const parts = proof.split(' | ').map(s => s.trim()).filter(Boolean)
  const fileUrl = parts.find(p => p.startsWith('/uploads/')) || ''
  const refNumber = parts.find(p => !p.startsWith('/uploads/')) || ''
  const isImage = /\.(jpg|jpeg|png|webp)$/i.test(fileUrl)
  return { refNumber, fileUrl, isImage }
}

const statusMap = {
  payment_requested: { label: 'Menunggu Pembayaran', cls: 'bg-orange-100 text-orange-700' },
  partial:           { label: 'Dibayar Sebagian',    cls: 'bg-amber-100 text-amber-700' },
  paid:              { label: 'Dibayar',             cls: 'bg-emerald-100 text-emerald-700' },
  received:          { label: 'Diterima',            cls: 'bg-purple-100 text-purple-700' },
}

const COLUMNS = [
  { key: 'request_number', label: 'Nomor' },
  { key: 'request_type',    label: 'Tipe' },
  { key: 'pengadaan_names', label: 'Pengadaan' },
  { key: 'vendor_name',     label: 'Vendor' },
  { key: 'work_unit_name',  label: 'Unit Kerja' },
  { key: 'total_final',     label: 'Nominal', align: 'right' },
  { key: 'status',          label: 'Status' },
  { key: 'created_at',      label: 'Tanggal' },
  { key: 'actions',         label: '', align: 'right' },
]

const selectableRows = computed(() => requests.value.filter(r => r.status === 'payment_requested' || r.status === 'partial'))
const allSelectableChecked = computed(() => selectableRows.value.length > 0 && selectableRows.value.every(r => selectedIds.value.has(r.id)))
const selectedVendorName = computed(() => {
  const first = requests.value.find(r => selectedIds.value.has(r.id))
  return first?.vendor_name || ''
})
const selectedTotal = computed(() => requests.value.filter(r => selectedIds.value.has(r.id)).reduce((s, r) => s + (r.total_final || 0), 0))
const payTotalAmount = computed(() => payTargets.value.reduce((s, r) => s + (r.total_final || 0), 0))
const payRemainingAmount = computed(() => payTargets.value.reduce((s, r) => s + ((r.total_final || 0) - (r.paid_amount || 0)), 0))

function toggleSelect(row) {
  const next = new Set(selectedIds.value)
  if (next.has(row.id)) {
    next.delete(row.id)
  } else {
    // Check vendor consistency
    if (next.size > 0) {
      const existingRow = requests.value.find(r => next.has(r.id))
      if (existingRow && existingRow.vendor_name !== row.vendor_name) {
        toast.error('Pembayaran harus diproses per vendor. Pilih pengajuan dari vendor yang sama.')
        return
      }
    }
    next.add(row.id)
  }
  selectedIds.value = next
}

function toggleSelectAll(e) {
  if (e.target.checked) {
    // Only select if all selectable are same vendor
    const vendors = [...new Set(selectableRows.value.map(r => r.vendor_name))]
    if (vendors.length > 1) {
      toast.error('Tidak bisa memilih semua — terdapat lebih dari 1 vendor. Pilih per vendor.')
      e.target.checked = false
      return
    }
    selectedIds.value = new Set(selectableRows.value.map(r => r.id))
  } else {
    selectedIds.value = new Set()
  }
}

function clearSelection() { selectedIds.value = new Set() }

function statusBadge(s) {
  const m = statusMap[s] || { cls: 'bg-gray-100 text-gray-600' }
  return `inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${m.cls}`
}
function statusLabel(s) { return (statusMap[s] || { label: s }).label }

// Penerimaan barang berdiri sendiri dari pembayaran: ada vendor yang mengirim
// dulu baru ditagih, ada yang minta dibayar di muka. Bagian pembayaran perlu
// tahu bedanya — membayar barang yang belum datang adalah risiko yang berbeda.
function receiptLabel(row) {
  if (row.receipt_status === 'received') return 'Barang sudah diterima'
  if (row.receipt_status === 'partial') return 'Barang diterima sebagian'
  return 'Barang belum diterima'
}
function receiptBadgeCls(row) {
  if (row.receipt_status === 'received') return 'receipt-ok'
  if (row.receipt_status === 'partial') return 'receipt-partial'
  return 'receipt-none'
}
function receiptHint(row) {
  return row.receipt_status === 'received'
    ? 'Barang sudah diterima — pembayaran menyusul (tempo).'
    : 'Barang belum diterima seluruhnya — membayar sekarang berarti membayar di muka.'
}
function adminName() { return authStore.admin?.name || 'Admin' }

onMounted(() => { fetchList(); fetchStats(); fetchBankAccounts(); fetchProjects() })

async function fetchBankAccounts() {
  try {
    const data = await bankAccountsApi.list()
    bankAccounts.value = Array.isArray(data) ? data : []
  } catch { bankAccounts.value = [] }
}

async function fetchStats() {
  try {
    const data = await purchaseApi.getPaymentStats()
    stats.value = {
      waiting: data.waiting || 0,
      totalWaiting: data.total_waiting || 0,
      paid: data.paid || 0,
      received: data.received || 0,
      accountsPayable: data.accounts_payable?.total_amount || 0,
    }
  } catch { /* ignore */ }
}

async function fetchList() {
  loading.value = true; errorMsg.value = ''
  try {
    const params = { page: page.value, limit: 20, parent_id: 'all', exclude_masters: 'true' }
    if (filterStatus.value) params.status = filterStatus.value
    if (filterType.value) params.type = filterType.value
    if (filterProject.value) params.project_id = filterProject.value
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()
    const data = await purchaseApi.list(params)
    requests.value = data.requests || []
    totalPages.value = data.total_pages || 1
  } catch (err) { errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { loading.value = false }
}

function goPage(p) { page.value = p; clearSelection(); fetchList() }

// Download Excel mengikuti filter aktif (status + tipe + kata kunci); scope
// role tetap dipaksakan di server, jadi user hanya menerima data miliknya.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const params = {}
    if (filterStatus.value) params.status = filterStatus.value
    if (filterType.value) params.type = filterType.value
    if (filterProject.value) params.project_id = filterProject.value
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()
    const blob = await purchaseApi.exportPayments(params)

    const statusLabel = statusFilterOptions.find(o => o.value === filterStatus.value)?.label || 'Semua'
    const slug = (filterStatus.value ? statusLabel : 'Semua-Status').replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
    const today = new Date().toLocaleDateString('en-CA')
    const filename = `Laporan-Pembayaran-Pengadaan_${slug}_${today}.xlsx`

    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    errorMsg.value = 'Gagal mengunduh Excel. Coba lagi, atau persempit filter.'
  } finally {
    exporting.value = false
  }
}

async function viewDetail(row) {
  try {
    detail.value = await purchaseApi.get(row.id)
    detailItems.value = detail.value.items || []
    showDetail.value = true
    fetchPaymentHistories(row.id)
  } catch { toast.error('Gagal memuat detail.') }
}

async function fetchPaymentHistories(id) {
  paymentHistories.value = []
  paymentHistoriesLoading.value = true
  try {
    const data = await purchaseApi.getPaymentHistories(id)
    paymentHistories.value = Array.isArray(data) ? data : []
  } catch { paymentHistories.value = [] }
  finally { paymentHistoriesLoading.value = false }
}

function openPayModal(row) {
  payTargets.value = [row]
  payForm.value = { payment_account_dest: '', payment_account_source: '', payment_proof: '', payment_notes: '', payment_amount: 0 }
  payVendor.value = null
  uploadedFileUrl.value = ''
  uploadedFileName.value = ''
  showPayModal.value = true
  fetchPayVendor(row.vendor_id)
}

function openBatchPay() {
  const selected = requests.value.filter(r => selectedIds.value.has(r.id))
  if (selected.length === 0) return
  // Final vendor consistency check
  const vendors = [...new Set(selected.map(r => r.vendor_name))]
  if (vendors.length > 1) {
    toast.error('Pembayaran harus diproses per vendor. Pilih pengajuan dari vendor yang sama.')
    return
  }
  payTargets.value = selected
  payForm.value = { payment_account_dest: '', payment_account_source: '', payment_proof: '', payment_notes: '', payment_amount: 0 }
  payVendor.value = null
  uploadedFileUrl.value = ''
  uploadedFileName.value = ''
  showPayModal.value = true
  fetchPayVendor(selected[0]?.vendor_id)
}

async function fetchPayVendor(vendorId) {
  if (!vendorId) { payVendor.value = null; return }
  payVendorLoading.value = true
  try {
    const v = await vendorsApi.get(vendorId)
    payVendor.value = v
    // Auto-fill rekening tujuan from vendor data
    if (v.account_number) {
      const parts = [v.bank_name, v.account_number, v.account_holder ? `a.n. ${v.account_holder}` : ''].filter(Boolean)
      payForm.value.payment_account_dest = parts.join(' ')
    }
  } catch { payVendor.value = null }
  finally { payVendorLoading.value = false }
}

function copyText(text, label) {
  navigator.clipboard.writeText(text).then(() => {
    toast.success(`${label} berhasil disalin: ${text}`)
  }).catch(() => {
    toast.error('Gagal menyalin ke clipboard.')
  })
}

async function handleFileUpload(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) { toast.error('Ukuran file maksimal 5MB.'); e.target.value = ''; return }
  uploadLoading.value = true
  try {
    const formData = new FormData()
    formData.append('file', file)
    const res = await apiClient.post('/admin/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    uploadedFileUrl.value = res.url
    uploadedFileName.value = res.filename
  } catch (err) {
    toast.error(err?.message ?? 'Gagal mengunggah file.')
  } finally {
    uploadLoading.value = false
    e.target.value = ''
  }
}

function removeUploadedFile() {
  uploadedFileUrl.value = ''
  uploadedFileName.value = ''
}

async function submitPay() {
  if (!payForm.value.payment_account_dest.trim()) { toast.error('Rekening tujuan wajib diisi.'); return }
  if (!payForm.value.payment_account_source) { toast.error('Rekening asal wajib diisi.'); return }
  const paymentAmount = payForm.value.payment_amount || 0
  if (payTargets.value.length === 1 && paymentAmount > 0 && paymentAmount > payRemainingAmount.value) {
    toast.error(`Jumlah bayar (${formatRupiah(paymentAmount)}) melebihi sisa tagihan (${formatRupiah(payRemainingAmount.value)}).`)
    return
  }
  const selectedAccount = bankAccountOptions.value.find(a => a.id === payForm.value.payment_account_source)
  const accountSourceText = selectedAccount?.name || payForm.value.payment_account_source
  payLoading.value = true
  let successCount = 0
  let failCount = 0
  try {
    for (const target of payTargets.value) {
      try {
        await purchaseApi.updateStatus(target.id, {
          action: 'pay',
          actor_name: adminName(),
          payment_proof: [payForm.value.payment_proof.trim(), uploadedFileUrl.value].filter(Boolean).join(' | '),
          payment_account_dest: payForm.value.payment_account_dest.trim(),
          payment_account_source: accountSourceText,
          payment_notes: payForm.value.payment_notes.trim(),
          payment_amount: payTargets.value.length === 1 ? paymentAmount : 0,
        })
        successCount++
      } catch { failCount++ }
    }
    if (failCount === 0) {
      toast.success(payTargets.value.length > 1 ? `${successCount} pembayaran berhasil dicatat.` : 'Pembayaran berhasil dicatat.')
    } else {
      toast.error(`${successCount} berhasil, ${failCount} gagal.`)
    }
    showPayModal.value = false
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) { toast.error(err?.message ?? 'Gagal melakukan pembayaran.') }
  finally { payLoading.value = false }
}
</script>

<style scoped>
@reference "tailwindcss";

.hero-header { position: relative; border-radius: 1rem; overflow: hidden; padding: 1.5rem 2rem; }
.hero-bg {
  position: absolute; inset: 0;
  background: linear-gradient(135deg, #065f46 0%, #0f766e 50%, #0d9488 100%);
}
.hero-content { position: relative; z-index: 1; }
.hero-title { font-size: 1.35rem; font-weight: 800; color: #fff; letter-spacing: -.02em; }
.hero-sub { font-size: .82rem; color: rgba(255,255,255,.7); margin-top: .2rem; }

.stat-card {
  display: flex; align-items: center; gap: .75rem;
  padding: 1rem 1.1rem; border-radius: .85rem; border: 1px solid; transition: box-shadow .15s;
}
.stat-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,.06); }
.stat-icon {
  width: 2.5rem; height: 2.5rem; border-radius: .6rem;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.stat-val { font-size: 1.1rem; font-weight: 800; line-height: 1.2; }
.stat-lbl { font-size: .7rem; font-weight: 500; margin-top: .1rem; }

.stat-waiting { border-color: rgba(249,115,22,.2); background: rgba(249,115,22,.04); }
.stat-waiting .stat-icon { background: rgba(249,115,22,.12); color: #ea580c; }
.stat-waiting .stat-val { color: #c2410c; }
.stat-waiting .stat-lbl { color: #9a3412; }

.stat-total { border-color: rgba(16,185,129,.2); background: rgba(16,185,129,.04); }
.stat-total .stat-icon { background: rgba(16,185,129,.12); color: #059669; }
.stat-total .stat-val { color: #047857; }
.stat-total .stat-lbl { color: #065f46; }

.stat-paid { border-color: rgba(59,130,246,.2); background: rgba(59,130,246,.04); }
.stat-paid .stat-icon { background: rgba(59,130,246,.12); color: #2563eb; }
.stat-paid .stat-val { color: #1d4ed8; }
.stat-paid .stat-lbl { color: #1e3a5f; }

.stat-received { border-color: rgba(139,92,246,.2); background: rgba(139,92,246,.04); }
.stat-received .stat-icon { background: rgba(139,92,246,.12); color: #7c3aed; }
.stat-received .stat-val { color: #6d28d9; }
.stat-received .stat-lbl { color: #4c1d95; }

.stat-hutang { border-color: rgba(249,115,22,.2); background: rgba(249,115,22,.04); }
.stat-hutang .stat-icon { background: rgba(249,115,22,.12); color: #ea580c; }
.stat-hutang .stat-val { color: #c2410c; }
.stat-hutang .stat-lbl { color: #9a3412; }

.type-badge { display: inline-flex; padding: .1rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 700; }
.type-barang { background: rgba(59,130,246,.1); color: #1d4ed8; }
.type-jasa   { background: rgba(168,85,247,.1); color: #7c3aed; }
.project-badge {
  display: inline-block; font-size: .62rem; font-weight: 600; line-height: 1.2;
  padding: .1rem .4rem; border-radius: .3rem;
  background: rgba(99,102,241,.1); color: #4f46e5;
}
.split-badge {
  display: inline-flex; padding: .05rem .4rem; border-radius: 999px;
  font-size: .6rem; font-weight: 700; letter-spacing: .02em;
  background: rgba(249,115,22,.1); color: #c2410c;
}

.action-btns { display: flex; gap: .35rem; }
.act-view, .act-pay {
  width: 28px; height: 28px; border-radius: .45rem; border: none; cursor: pointer;
  display: flex; align-items: center; justify-content: center; transition: all .12s;
}
.act-view { background: rgba(59,130,246,.1); color: #3b82f6; }
.act-view:hover { background: rgba(59,130,246,.2); }
.act-pay { background: rgba(16,185,129,.1); color: #059669; }
.act-pay:hover { background: rgba(16,185,129,.2); }

.tbl-check {
  width: 1rem; height: 1rem; border-radius: .25rem;
  accent-color: #059669; cursor: pointer;
}

/* Selection bar */
.selection-bar {
  position: sticky; top: 0; z-index: 20;
  display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .75rem;
  padding: .75rem 1.25rem; border-radius: .75rem;
  background: linear-gradient(135deg, #065f46 0%, #0f766e 100%);
  box-shadow: 0 4px 16px rgba(0,0,0,.15);
}
.bar-btn {
  padding: .4rem 1rem; border-radius: .5rem; font-size: .8rem; font-weight: 600;
  border: none; cursor: pointer; transition: all .12s;
}
.bar-cancel { background: rgba(255,255,255,.15); color: #fff; }
.bar-cancel:hover { background: rgba(255,255,255,.25); }
.bar-pay { background: #fff; color: #065f46; }
.bar-pay:hover { background: #ecfdf5; }

.slide-bar-enter-active, .slide-bar-leave-active { transition: all .25s ease; }
.slide-bar-enter-from, .slide-bar-leave-to { opacity: 0; transform: translateY(-8px); }

/* Penanda penerimaan barang di halaman pembayaran */
.receipt-ok, .receipt-partial, .receipt-none {
  display: inline-block; padding: .1rem .45rem; border-radius: 999px;
  font-size: .65rem; font-weight: 700; white-space: nowrap;
}
.receipt-ok      { background: rgba(16,185,129,.13); color: #047857; }
.receipt-partial { background: rgba(245,158,11,.15); color: #b45309; }
.receipt-none    { background: rgba(107,114,128,.12); color: #4b5563; }
</style>
