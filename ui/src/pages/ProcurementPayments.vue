<template>
  <div class="space-y-5">
    <!-- Hero Header -->
    <div class="hero-header">
      <div class="hero-bg" />
      <div class="hero-content flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="hero-title">Pembayaran</h1>
          <p class="hero-sub">Kelola pembayaran pengadaan, pengajuan bayar ke Manager Keuangan / GM, & pembayaran manual</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            @click="openManualModal"
            class="inline-flex items-center gap-2 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-emerald-800 shadow-sm hover:bg-emerald-50 transition-colors cursor-pointer"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2.2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
            </svg>
            Buat Pembayaran Manual
          </button>
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
            placeholder="Nomor, batch (AJU-…), nama pengadaan, atau vendor…"
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
          <SearchableSelect v-model="filterType" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg bg-white/85 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-100 outline-none transition-all">
            <option v-for="opt in typeFilterOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </SearchableSelect>
        </div>
        <!-- Pembelian Darurat & tingkat kebutuhan (ikut ke Download Excel). -->
        <div class="flex flex-col gap-1 min-w-[160px]">
          <label class="text-sm font-medium text-gray-700">Jenis</label>
          <SearchableSelect v-model="filterKind" aria-label="Filter jenis pembelian" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg bg-white/85 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-100 outline-none transition-all">
            <option v-for="opt in KIND_FILTERS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </SearchableSelect>
        </div>
        <div class="flex flex-col gap-1 min-w-[210px]">
          <label class="text-sm font-medium text-gray-700">Persetujuan Bayar</label>
          <SearchableSelect v-model="filterApproval" aria-label="Filter persetujuan pembayaran" class="w-full px-3 py-2 text-sm border border-gray-200 rounded-lg bg-white/85 focus:border-emerald-400 focus:ring-2 focus:ring-emerald-100 outline-none transition-all">
            <option v-for="opt in APPROVAL_FILTERS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </SearchableSelect>
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
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-sm font-semibold text-white">{{ selectedIds.size }} tagihan dipilih</span>
          <span class="text-xs text-emerald-200">Vendor: {{ selectedVendorSummary }}</span>
          <span class="text-xs font-semibold text-emerald-100">Total: {{ formatRupiah(selectedTotal) }}</span>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button class="bar-btn bar-cancel" @click="clearSelection">Batal</button>
          <button
            v-if="selectedSubmittableRows.length > 0"
            class="bar-btn bg-amber-400 text-amber-950 hover:bg-amber-300"
            @click="openSubmitApprovalModal(selectedSubmittableRows)"
          >
            📝 Buat Pengajuan Pembayaran ({{ selectedSubmittableRows.length }})
          </button>
          <button
            v-if="canApprovePayment && selectedApprovableRows.length > 0"
            class="bar-btn bg-emerald-400 text-emerald-950 hover:bg-emerald-300"
            :disabled="approvalDecisionLoading"
            @click="approvePaymentRows(selectedApprovableRows)"
          >
            ✅ Setujui Pembayaran ({{ selectedApprovableRows.length }})
          </button>
          <button
            v-if="canApprovePayment && selectedApprovableRows.length > 0"
            class="bar-btn bg-red-500 text-white hover:bg-red-600"
            :disabled="approvalDecisionLoading"
            @click="openRejectApprovalModal(selectedApprovableRows)"
          >
            ✕ Tolak ({{ selectedApprovableRows.length }})
          </button>
          <button
            v-if="canPay && selectedPayableRows.length > 0 && selectedPayableRows.length === selectedRows.length"
            class="bar-btn bar-pay"
            @click="openBatchPay"
          >
            💳 Bayar {{ selectedPayableRows.length }} Tagihan
          </button>
        </div>
      </div>
    </Transition>

    <!-- List -->
    <AppCard :padding="false">
      <div class="overflow-x-auto rounded-lg border border-gray-200">
        <table class="min-w-full text-sm divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th v-if="showSelect" class="px-3 py-3 w-10">
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
                <td v-if="showSelect" class="px-3 py-3"><div class="h-4 w-4 bg-gray-200 rounded animate-pulse" /></td>
                <td v-for="col in COLUMNS" :key="col.key" class="px-4 py-3"><div class="h-4 bg-gray-200 rounded animate-pulse" /></td>
              </tr>
            </template>
            <tr v-else-if="!requests.length">
              <td :colspan="COLUMNS.length + ((showSelect) ? 1 : 0)" class="px-4 py-12 text-center text-gray-400">
                Tidak ada data pembayaran.
              </td>
            </tr>
            <template v-else>
              <tr v-for="row in requests" :key="row.id" class="hover:bg-gray-50 transition-colors" :class="{ 'bg-emerald-50/40': selectedIds.has(row.id) }">
                <td v-if="showSelect" class="px-3 py-3">
                  <input v-if="isCheckable(row)" type="checkbox" :checked="selectedIds.has(row.id)" @change="toggleSelect(row)" class="tbl-check" />
                </td>
                <td class="px-4 py-3 align-middle">
                  <span class="font-mono text-xs text-gray-700">{{ row.request_number || '-' }}</span>
                  <div v-if="row.is_manual_payment" class="mt-0.5">
                    <span class="inline-flex items-center rounded-full bg-purple-100 border border-purple-200 px-2 py-0.5 text-[10px] font-semibold text-purple-800">
                      Pembayaran Manual
                    </span>
                  </div>
                  <div v-if="row.payment_batch_number" class="text-[10px] font-mono text-emerald-700 mt-0.5">
                    Batch: {{ row.payment_batch_number }}
                  </div>
                  <div v-if="row.parent_number" class="text-[10px] text-gray-400 mt-0.5">Induk: {{ row.parent_number }}</div>
                </td>
                <td class="px-4 py-3 align-middle">
                  <span :class="typeBadgeCls(row.request_type)">
                    {{ purchaseTypeLabel(row.request_type) }}
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
                  <div v-if="row.invoice_number" class="mt-0.5 font-mono text-[11px] text-gray-500">
                    Inv: {{ row.invoice_number }}
                  </div>
                  <div class="mt-0.5 flex flex-wrap items-center gap-1">
                    <span v-if="row.parent_id" class="split-badge">Split</span>
                    <a
                      v-if="safeUrl(row.invoice_url)"
                      :href="safeUrl(row.invoice_url)"
                      target="_blank"
                      rel="noopener noreferrer"
                      @click.stop
                      class="inline-flex items-center gap-1 rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 hover:bg-blue-100"
                      title="Buka berkas invoice yang diunggah Purchasing"
                    >
                      📄 Lihat Invoice
                    </a>
                  </div>
                </td>
                <td class="px-4 py-3 align-middle">{{ row.work_unit_name || '-' }}</td>
                <td class="px-4 py-3 align-middle text-right font-semibold text-emerald-700">
                  {{ formatRupiah(row.total_final) }}
                  <div v-if="row.paid_amount > 0 && remainingOf(row) > 0" class="text-[10px] font-normal text-amber-600 mt-0.5">
                    Sisa: {{ formatRupiah(remainingOf(row)) }}
                  </div>
                </td>
                <td class="px-4 py-3 align-middle">
                  <div class="flex flex-col items-start gap-1">
                    <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
                    <span v-if="showApprovalBadge(row)" :class="paymentApprovalBadgeCls(row)" :title="paymentApprovalHint(row)">
                      {{ paymentApprovalLabel(row) }}
                    </span>
                    <span v-if="row.payment_approval_status === 'rejected' && row.payment_rejection_reason" class="max-w-[220px] text-[10px] leading-snug text-red-600">
                      Alasan tolak: {{ row.payment_rejection_reason }}
                    </span>
                    <PurchaseUrgencyBadges :doc="row" />
                    <span v-if="!row.is_manual_payment && isPhysicalPurchaseType(row.request_type)" :class="receiptBadgeCls(row)" :title="receiptHint(row)">
                      {{ receiptLabel(row) }}
                    </span>
                    <span v-if="row.cash_payout" :class="payoutBadgeCls(row.cash_payout)" :title="payoutHint(row.cash_payout)">
                      {{ payoutLabel(row.cash_payout) }}
                    </span>
                    <span v-if="row.cash_payout?.status === 'returned'" class="max-w-[220px] text-[10px] leading-snug text-red-600">
                      Alasan kasir: {{ row.cash_payout.resolution_reason || '-' }}
                    </span>
                    <span v-if="payToBlocked(row)" :class="payToBlockCls(row)" :title="row.pay_to?.problem">{{ payToBlockLabel(row) }}</span>
                    <span v-if="payToBlocked(row) && row.pay_to?.returned_at" class="max-w-[220px] text-[10px] leading-snug text-red-600">
                      Alasan: {{ row.pay_to.return_reason || '-' }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-3 align-middle text-gray-700">{{ formatDateTime(row.created_at) }}</td>
                <td class="px-4 py-3 align-middle text-right">
                  <div class="action-btns justify-end">
                    <AppActionButton icon="view" label="Lihat Detail" @click="viewDetail(row)" />
                    <AppActionButton icon="send" tone="warning" label="Ajukan Pembayaran ke Mgr Keuangan / GM" v-if="canSubmitRowForApproval(row)" @click="openSubmitApprovalModal([row])" />
                    <AppActionButton icon="approve" label="Setujui Pembayaran (Mgr Keuangan / GM)" v-if="canApprovePayment && canApproveRowPayment(row)" :disabled="approvalDecisionLoading" @click="approvePaymentRows([row])" />
                    <AppActionButton icon="reject" label="Tolak Pengajuan Pembayaran" v-if="canApprovePayment && canApproveRowPayment(row)" :disabled="approvalDecisionLoading" @click="openRejectApprovalModal([row])" />
                    <AppActionButton icon="pay" label="Bayar" v-if="canPay && isPayable(row)" @click="openPayModal(row)" />
                    <AppActionButton icon="receive" label="Selesaikan Jasa" v-if="canPay && canFinishJasa(row)" @click="finishJasa(row)" />
                    <AppActionButton icon="cancel" label="Tarik tugas Kas Kasir Outlet" v-if="canPay && row.cash_payout?.status === 'pending'" @click="cancelPayout(row)" />
                    <AppActionButton icon="return" label="Kembalikan detail pembayaran ke Pembelian" v-if="canPay && canReturnPayTo(row)" @click="openReturnPayTo(row)" />
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
          <span v-if="detail.is_manual_payment" class="inline-flex items-center rounded-full bg-purple-100 border border-purple-200 px-2.5 py-0.5 text-xs font-semibold text-purple-800">
            Pembayaran Manual (Di Luar Pengajuan)
          </span>
          <span v-if="showApprovalBadge(detail)" :class="paymentApprovalBadgeCls(detail)">
            {{ paymentApprovalLabel(detail) }}
          </span>
          <span v-if="!detail.is_manual_payment && isPhysicalPurchaseType(detail.request_type)" :class="receiptBadgeCls(detail)" class="text-sm">
            {{ receiptLabel(detail) }}
          </span>
          <span v-if="detail.request_number" class="font-mono text-sm text-gray-600">No. {{ detail.request_number }}</span>
          <span :class="typeBadgeCls(detail.request_type)" class="text-xs">
            {{ purchaseTypeLabel(detail.request_type) }}
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
                <template v-if="detail.family_total_final">
                  <dt class="text-gray-400">Seluruh pecahan</dt>
                  <dd class="text-gray-600">{{ formatRupiah(detail.family_total_final) }}</dd>
                </template>
              </dl>
              <p v-if="detail.family_total_final" class="mt-1.5 text-[11px] leading-snug text-gray-500">
                Induk hasil pecah vendor: tagihan di sini hanya sisa item milik induk. Tiap pecahan dibayar di barisnya sendiri.
              </p>
            </div>
          </div>

          <!-- Persetujuan (beserta verifikasinya) -->
          <div v-if="detail.approved_by || detail.verified_by" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-amber-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-amber-600 mb-1.5">Persetujuan</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="detail.verified_by">
                  <dt class="text-gray-400">Diverifikasi</dt>
                  <dd class="font-medium text-gray-900">{{ detail.verified_by }}<span v-if="detail.verified_at" class="font-normal text-gray-500"> · {{ formatDateTime(detail.verified_at) }}</span></dd>
                </template>
                <template v-if="detail.approved_by">
                  <dt class="text-gray-400">Disetujui</dt>
                  <dd class="font-medium text-gray-900">{{ detail.approved_by }}<span v-if="detail.approved_at" class="font-normal text-gray-500"> · {{ formatDateTime(detail.approved_at) }}</span></dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Persetujuan Pengajuan Pembayaran (Manager Keuangan / General Manager) -->
          <div v-if="detail.payment_approval_status || detail.is_manual_payment || detail.payment_batch_number" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden sm:col-span-2">
            <div class="border-l-4 px-3 py-2.5" :class="detail.payment_approval_status === 'approved' ? 'border-emerald-500' : detail.payment_approval_status === 'rejected' ? 'border-red-500' : 'border-amber-500'">
              <p class="text-[11px] font-semibold uppercase tracking-wider mb-1.5" :class="detail.payment_approval_status === 'approved' ? 'text-emerald-700' : detail.payment_approval_status === 'rejected' ? 'text-red-700' : 'text-amber-700'">
                Persetujuan Pembayaran (Manager Keuangan / General Manager)
              </p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Status</dt>
                <dd class="font-semibold text-gray-900">{{ paymentApprovalLabel(detail) }}</dd>
                <template v-if="detail.payment_batch_number">
                  <dt class="text-gray-400">No. Pengajuan Bayar</dt>
                  <dd class="font-mono font-medium text-emerald-700">{{ detail.payment_batch_number }}</dd>
                </template>
                <template v-if="detail.payment_submitted_by">
                  <dt class="text-gray-400">Diajukan Oleh</dt>
                  <dd class="font-medium text-gray-900">{{ detail.payment_submitted_by }}<span v-if="detail.payment_submitted_at" class="font-normal text-gray-500"> · {{ formatDateTime(detail.payment_submitted_at) }}</span></dd>
                </template>
                <template v-if="detail.payment_submission_notes">
                  <dt class="text-gray-400">Catatan Pengajuan</dt>
                  <dd class="text-gray-700">{{ detail.payment_submission_notes }}</dd>
                </template>
                <template v-if="detail.payment_approved_by">
                  <dt class="text-gray-400">Disetujui Oleh</dt>
                  <dd class="font-semibold text-emerald-700">{{ detail.payment_approved_by }}<span v-if="detail.payment_approved_at" class="font-normal text-gray-500"> · {{ formatDateTime(detail.payment_approved_at) }}</span></dd>
                </template>
                <template v-if="detail.payment_rejected_by">
                  <dt class="text-gray-400">Ditolak Oleh</dt>
                  <dd class="font-semibold text-red-700">{{ detail.payment_rejected_by }}<span v-if="detail.payment_rejected_at" class="font-normal text-gray-500"> · {{ formatDateTime(detail.payment_rejected_at) }}</span></dd>
                </template>
                <template v-if="detail.payment_rejection_reason">
                  <dt class="text-gray-400">Alasan Tolak</dt>
                  <dd class="text-red-700">{{ detail.payment_rejection_reason }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Tujuan pembayaran: diisi Pembelian, Keuangan hanya membaca. -->
          <div v-if="PAYABLE_STATUSES.includes(detail.status) && remainingOf(detail) > 0" class="sm:col-span-2">
            <PurchasePayToCard :pay-to="detail.pay_to" :amount="remainingOf(detail)" />
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
                  <AppActionButton icon="photo" label="Lihat Bukti Transfer (PDF)" :href="parsedPaymentProof(detail.payment_proof).fileUrl" target="_blank" />
                </div>
              </div>
            </div>
          </div>

          <!-- No. Invoice & Catatan -->
          <div v-if="detail.invoice_number || detail.invoice_url || detail.notes" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden col-span-1 sm:col-span-2">
            <div class="border-l-4 border-gray-400 px-3 py-2.5 flex flex-wrap items-start justify-between gap-3">
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
              <div v-if="safeUrl(detail.invoice_url)" class="flex items-center gap-2">
                <a v-if="isImageUrl(safeUrl(detail.invoice_url))" :href="safeUrl(detail.invoice_url)" target="_blank" rel="noopener noreferrer" class="shrink-0">
                  <img :src="safeUrl(detail.invoice_url)" alt="Invoice Vendor" class="h-10 w-10 rounded border border-gray-200 object-cover" />
                </a>
                <AppActionButton icon="photo" label="Lihat Invoice Vendor" :href="safeUrl(detail.invoice_url)" target="_blank" />
              </div>
            </div>
          </div>

          <!-- Struk / Nota Pembelian dari Purchasing -->
          <div v-if="detail.struks?.length" class="rounded-lg border border-blue-200 bg-blue-50/40 shadow-sm overflow-hidden col-span-1 sm:col-span-2">
            <div class="border-l-4 border-blue-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-blue-700 mb-2">
                Struk / Invoice Pembelian dari Purchasing ({{ detail.struks.length }})
              </p>
              <ul class="divide-y divide-blue-100">
                <li v-for="s in detail.struks" :key="s.id" class="flex flex-wrap items-center justify-between gap-2 py-1.5 text-xs">
                  <div class="min-w-0">
                    <span class="font-mono font-semibold text-gray-900">{{ s.struk_number }}</span>
                    <span class="text-gray-500 ml-1.5">· {{ formatDate(s.struk_date) }} · {{ formatRupiah(s.total) }}</span>
                    <span v-if="s.created_by" class="text-gray-400 ml-1">({{ s.created_by }})</span>
                    <p v-if="s.notes" class="text-[11px] text-gray-500 mt-0.5">{{ s.notes }}</p>
                  </div>
                  <div class="flex items-center gap-2 shrink-0">
                    <span v-if="s.remaining <= 0" class="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700">Lunas</span>
                    <span v-else class="text-[11px] font-medium text-amber-700">Sisa {{ formatRupiah(s.remaining) }}</span>
                    <AppActionButton v-if="safeUrl(s.photo_url)" icon="photo" label="Lihat Foto Invoice/Struk" :href="safeUrl(s.photo_url)" target="_blank" />
                  </div>
                </li>
              </ul>
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
                <span>Sisa: <span class="font-semibold" :class="remainingOf(detail) > 0 ? 'text-amber-600' : 'text-emerald-600'">{{ formatRupiah(remainingOf(detail)) }}</span></span>
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
                <div v-if="ph.bank_fee > 0">
                  <span class="text-gray-400">Biaya Bank:</span>
                  <span class="font-semibold text-amber-700 ml-1">+{{ formatRupiah(ph.bank_fee) }}</span>
                  <span class="text-gray-500 ml-1">(Total: {{ formatRupiah((Number(ph.amount) || 0) + (Number(ph.bank_fee) || 0)) }})</span>
                </div>
                <div v-if="ph.paid_by">
                  <span class="text-gray-400">Oleh:</span>
                  <span class="font-medium text-gray-700 ml-1">{{ ph.paid_by }}</span>
                </div>
                <div v-if="ph.payment_account_dest" class="col-span-2">
                  <span class="text-gray-400">Rek Tujuan:</span>
                  <span class="font-mono text-gray-700 ml-1">{{ ph.payment_account_dest }}</span>
                  <span v-if="ph.vendor_name" class="ml-1 text-gray-500">({{ ph.vendor_name }})</span>
                  <span v-if="ph.payment_method === 'marketplace_va'" class="ml-1 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700">VA Marketplace</span>
                  <span v-else-if="ph.payment_method === 'outlet_cash'" class="ml-1 rounded bg-teal-100 px-1.5 py-0.5 text-[10px] font-semibold text-teal-700"
                    title="Diserahkan tunai oleh kasir outlet lewat tugas Bayar Pengadaan di tablet">Kas Kasir Outlet</span>
                  <span v-else-if="ph.dest_manual" class="ml-1 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700"
                    title="Vendor belum punya rekening terdaftar — rekening/cara bayar diketik pembayar">manual</span>
                </div>
                <div v-if="ph.payment_marketplace" class="col-span-2">
                  <span class="text-gray-400">Marketplace / Pesanan:</span>
                  <span class="text-gray-700 ml-1">{{ ph.payment_marketplace }} · {{ ph.payment_order_number }}</span>
                </div>
                <div v-if="ph.struk_number">
                  <span class="text-gray-400">Struk:</span>
                  <span class="font-mono text-gray-700 ml-1">{{ ph.struk_number }}</span>
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
                <AppActionButton icon="photo" label="Lihat Bukti Transfer" :href="parsedPaymentProof(ph.payment_proof).fileUrl" target="_blank" />
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
        <AppActionButton icon="return" label="Kembalikan detail pembayaran ke Pembelian" v-if="canPay && detail && canReturnPayTo(detail)" @click="openReturnPayTo(detail)" />
        <AppButton size="sm" variant="secondary" v-if="detail && canSubmitRowForApproval(detail)" @click="showDetail = false; openSubmitApprovalModal([detail])">Ajukan Pembayaran</AppButton>
        <AppButton size="sm" variant="danger" v-if="canApprovePayment && detail && canApproveRowPayment(detail)" :disabled="approvalDecisionLoading" @click="showDetail = false; openRejectApprovalModal([detail])">Tolak Pembayaran</AppButton>
        <AppButton size="sm" v-if="canApprovePayment && detail && canApproveRowPayment(detail)" :loading="approvalDecisionLoading" @click="approvePaymentRows([detail])">Setujui Pembayaran</AppButton>
        <AppActionButton icon="receive" label="Selesaikan Jasa" v-if="canPay && detail && canFinishJasa(detail)" @click="finishJasa(detail)" />
        <AppActionButton icon="pay" label="Bayar" v-if="canPay && detail && isPayable(detail)" @click="showDetail = false; openPayModal(detail)" />
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>

    <!-- Payment Modal -->
    <AppModal v-model="showPayModal" :title="payTargets.length > 1 ? `Pembayaran ${payTargets.length} Pengajuan` : 'Pembayaran Pengajuan'" size="2xl">
      <div class="modal-form-grid">
        <div class="space-y-4">
          <!-- Ringkasan dan rekening tetap berkelompok di samping isian pembayaran. -->
          <div class="grid grid-cols-1 gap-3">
            <!-- Card 1: Nominal & Vendor -->
            <div class="rounded-lg border border-gray-200 bg-white overflow-hidden">
              <div class="bg-emerald-50 px-4 py-2.5 border-b border-emerald-100">
                <p class="text-[10px] text-gray-500 uppercase tracking-wider font-medium">Total Pembayaran</p>
                <p class="text-xl font-bold text-emerald-700 mt-0.5">{{ formatRupiah(payTargets.length > 1 ? payRemainingAmount : payTotalAmount) }}</p>
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
                  <template v-if="payTargets.length === 1 && (payTargets[0]?.invoice_number || safeUrl(payTargets[0]?.invoice_url))">
                    <dt class="text-gray-400">Invoice</dt>
                    <dd class="flex flex-wrap items-center gap-2">
                      <span v-if="payTargets[0]?.invoice_number" class="font-medium font-mono text-gray-800">{{ payTargets[0].invoice_number }}</span>
                      <a
                        v-if="safeUrl(payTargets[0]?.invoice_url)"
                        :href="safeUrl(payTargets[0].invoice_url)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="inline-flex items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-700 hover:bg-blue-100"
                      >
                        📄 Lihat Invoice Vendor ↗
                      </a>
                    </dd>
                  </template>
                  <template v-if="payTargets.length === 1 && payStruks.some(s => safeUrl(s.photo_url))">
                    <dt class="text-gray-400">Foto Struk</dt>
                    <dd class="flex flex-wrap items-center gap-1.5">
                      <a
                        v-for="s in payStruks.filter(st => safeUrl(st.photo_url))"
                        :key="s.id"
                        :href="safeUrl(s.photo_url)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="inline-flex items-center gap-1 rounded border border-blue-200 bg-blue-50 px-2 py-0.5 text-[11px] font-semibold text-blue-700 hover:bg-blue-100"
                      >
                        🧾 Struk {{ s.struk_number }} ↗
                      </a>
                    </dd>
                  </template>
                  <template v-if="payTargets.length > 1">
                    <dt class="text-gray-400">Pengajuan</dt>
                    <dd class="font-medium text-gray-800">{{ payTargets.length }} item</dd>
                  </template>
                </dl>
              </div>
            </div>

            <!-- Card 2: Tujuan pembayaran — diisi Pembelian; Keuangan hanya
                 membaca & menyalin (keputusan owner 19 Nov 2026). -->
            <PurchasePayToCard v-if="paySource === 'office'" :pay-to="payTargets[0]?.pay_to" :amount="payCopyAmount">
              <!-- Rekening vendor baru diganti: pastikan sah sebelum transfer. -->
              <div v-if="recentBankChange && payTargets[0]?.pay_to?.effective === 'vendor_account'" class="border-t border-amber-200 bg-amber-50 px-4 py-2 text-[11px] leading-snug text-amber-800">
                <p class="font-semibold">Rekening vendor ini diubah {{ formatDateTime(recentBankChange.changed_at) }}<template v-if="recentBankChange.changed_by"> oleh {{ recentBankChange.changed_by }}</template>.</p>
                <p>Sebelumnya: <span class="font-mono">{{ bankText(recentBankChange.old_bank_name, recentBankChange.old_account_number, recentBankChange.old_account_holder) || '(kosong)' }}</span>. Konfirmasi ke vendor bahwa perubahan ini sah sebelum mentransfer.</p>
              </div>
              <div v-if="payTargets.length === 1 && canReturnPayTo(payTargets[0])" class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 px-4 py-2">
                <p class="text-[11px] text-gray-500">Detail salah atau kedaluwarsa?</p>
                <AppButton size="sm" variant="secondary" @click="openReturnPayTo(payTargets[0])">Kembalikan ke Pembelian</AppButton>
              </div>
            </PurchasePayToCard>
          </div>

          <!-- Batch detail (only for multi) -->
          <div v-if="payTargets.length > 1" class="rounded-lg bg-gray-50 border border-gray-100 px-4 py-2.5 space-y-1">
            <p class="text-[10px] font-semibold uppercase tracking-wider text-gray-500 mb-1">Rincian {{ payTargets.length }} pengajuan</p>
            <div v-for="t in payTargets" :key="t.id" class="flex items-center justify-between gap-2 text-xs">
              <div class="min-w-0 truncate mr-2">
                <span class="text-gray-500">{{ t.request_number }} — {{ (t.items || []).map(i => i.name).join(', ') || t.work_unit_name || '-' }}</span>
                <span v-if="t.invoice_number" class="ml-1.5 font-mono text-[11px] text-gray-400">(Inv: {{ t.invoice_number }})</span>
                <a
                  v-if="safeUrl(t.invoice_url)"
                  :href="safeUrl(t.invoice_url)"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="ml-1.5 inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 hover:bg-blue-100"
                >
                  📄 Invoice ↗
                </a>
              </div>
              <span class="font-medium text-emerald-700 whitespace-nowrap">{{ formatRupiah(remainingOf(t)) }}<template v-if="t.paid_amount > 0"> <span class="font-normal text-gray-400">(sisa)</span></template></span>
            </div>
          </div>

        </div>

        <div class="space-y-4">

          <!-- Sumber dana: satu pembayaran hanya punya SATU sumber uang. -->
          <fieldset v-if="payTargets.length === 1" class="space-y-2">
            <legend class="block text-sm font-medium text-gray-700 mb-1">Sumber Dana</legend>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <label class="source-opt" :class="{ 'source-opt--on': paySource === 'office' }">
                <input v-model="paySource" type="radio" value="office" class="sr-only" />
                <span class="text-sm font-semibold text-gray-800">Kantor Pusat</span>
                <span class="text-[11px] leading-snug text-gray-500">Dibayar Bagian Keuangan (transfer / VA)</span>
              </label>
              <label class="source-opt" :class="{ 'source-opt--on': paySource === 'outlet_cash', 'source-opt--off': !canUseOutletCash }">
                <input v-model="paySource" type="radio" value="outlet_cash" class="sr-only" :disabled="!canUseOutletCash" />
                <span class="text-sm font-semibold text-gray-800">Kas Kasir Outlet</span>
                <span class="text-[11px] leading-snug text-gray-500">{{ outletCashHint }}</span>
              </label>
            </div>
          </fieldset>

          <template v-if="paySource === 'outlet_cash'">
            <div class="rounded-lg border border-teal-200 bg-teal-50 px-4 py-3 text-xs leading-relaxed text-teal-900">
              Tugas langsung muncul di tablet kasir <strong>{{ payOutletName }}</strong> saat tablet online.
              Kasir menyerahkan kas dengan PIN manajer dan mencetak bukti penyerahan; status pengajuan
              berubah setelah kas diserahkan. Selama tugas terbuka, pengajuan ini tidak bisa dibayar dari Kantor Pusat.
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-3">
              <div class="sm:col-span-2">
                <label class="block text-sm font-medium text-gray-700 mb-1">Nominal dari Kas Outlet</label>
                <RupiahInput v-model="payoutForm.amount" :placeholder="`Kosongkan = sisa tagihan (${formatRupiah(payRemainingAmount)})`" />
              </div>
              <div class="sm:col-span-2">
                <AppInput v-model="payoutForm.recipient_name" label="Diserahkan Kepada *" placeholder="Nama vendor / staf yang belanja" maxlength="200" />
              </div>
              <div class="sm:col-span-2">
                <label class="block text-sm font-medium text-gray-700 mb-1">Catatan untuk Kasir</label>
                <textarea v-model="payoutForm.notes" rows="2" maxlength="1000" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" placeholder="Opsional, mis. bayar saat barang diantar"></textarea>
              </div>
            </div>
          </template>

          <template v-else>
          <!-- Form 2-col -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-3">
            <p v-if="!payTargetsReady" class="sm:col-span-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
              Tujuan pembayaran belum siap — tunggu Pembelian melengkapi atau memperbaikinya.
            </p>
            <div class="sm:col-span-2">
              <label for="payment-account-source" class="block text-sm font-medium text-gray-700 mb-1">Rekening Asal (Perusahaan)</label>
              <SearchSelect
                id="payment-account-source"
                v-model="payForm.payment_account_source"
                aria-label="Rekening Asal (Perusahaan)"
                :options="bankAccountOptions"
                placeholder="Pilih rekening…"
                searchPlaceholder="Cari rekening…"
                valueKey="id"
                labelKey="name"
              />
              <p v-if="bankAccountsError" class="mt-1 text-[11px] leading-snug text-red-600">{{ bankAccountsError }}</p>
              <p v-else-if="!bankAccountOptions.length" class="mt-1 text-[11px] leading-snug text-amber-700">
                Belum ada rekening perusahaan yang aktif — tambahkan di menu Data Rekening dulu.
              </p>
              <p v-else-if="isSameBank" class="mt-1 text-[11px] font-medium text-emerald-700">
                ✓ Sesama rekening {{ sourceBankName }} (bebas biaya admin antarbank)
              </p>
            </div>
            <!-- Biaya Admin Bank: muncul otomatis saat rekening sumber pembayaran berbeda dengan bank supplier/tujuan -->
            <div v-if="isDifferentBank" class="sm:col-span-2 rounded-lg border border-amber-200 bg-amber-50/70 p-3 space-y-2">
              <div class="flex flex-wrap items-center justify-between gap-1">
                <label class="text-xs font-semibold text-amber-900">
                  Biaya Admin Bank <span class="font-normal text-amber-700">(jika ada)</span>
                </label>
                <span class="inline-flex items-center rounded-full bg-amber-100 border border-amber-300 px-2 py-0.5 text-[10px] font-semibold text-amber-800">
                  Beda Bank: {{ sourceBankName }} → {{ destBankName || 'Rekening Tujuan' }}
                </span>
              </div>
              <RupiahInput v-model="payForm.bank_fee" placeholder="0 (Kosongkan jika bebas biaya admin)" />
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="text-[10px] text-amber-700 mr-0.5">Pilih cepat:</span>
                <button
                  v-for="preset in BANK_FEE_PRESETS"
                  :key="preset.value"
                  type="button"
                  class="rounded-full border px-2 py-0.5 text-[10px] font-semibold transition-colors cursor-pointer"
                  :class="(payForm.bank_fee || 0) === preset.value ? 'border-amber-600 bg-amber-600 text-white' : 'border-amber-300 bg-white text-amber-800 hover:bg-amber-100'"
                  @click="payForm.bank_fee = preset.value"
                >
                  {{ preset.label }}
                </button>
              </div>
              <p v-if="payForm.bank_fee > 0" class="text-[11px] font-medium text-amber-900 border-t border-amber-200/80 pt-1.5">
                Total keluar dari rekening: <strong>{{ formatRupiah(effectiveTransferAmount + payForm.bank_fee) }}</strong>
                <span class="text-amber-700 font-normal"> (Tagihan {{ formatRupiah(effectiveTransferAmount) }} + Biaya Bank {{ formatRupiah(payForm.bank_fee) }} · dicatat ke Beban Administrasi Bank)</span>
              </p>
            </div>
            <!-- Bayar per struk: pengajuan yang punya struk pembelian dibayar per
                 struk (boleh dicicil); tanpa memilih, server membagi ke struk
                 yang belum lunas, terlama dulu. -->
            <div v-if="payTargets.length === 1 && payStrukOptions.length" class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700 mb-1">Struk yang Dibayar</label>
              <!-- Kelas eksplisit: .input-sm hanya ada di style scoped halaman
                   Pengadaan, di sini pilihan struk tampil tanpa bingkai. -->
              <SearchableSelect v-model="payStrukId" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500">
                <option value="">Bagi otomatis ke struk yang belum lunas (terlama dulu)</option>
                <option v-for="st in payStrukOptions" :key="st.id" :value="st.id">
                  {{ st.struk_number }} · {{ formatDate(st.struk_date) }} · sisa {{ formatRupiah(st.remaining) }}
                </option>
              </SearchableSelect>
            </div>
            <div v-if="payTargets.length === 1">
              <label class="block text-sm font-medium text-gray-700 mb-1">Jumlah Bayar</label>
              <RupiahInput v-model="payForm.payment_amount" :placeholder="selectedPayStruk ? `Kosongkan = lunasi struk (${formatRupiah(selectedPayStruk.remaining)})` : `Kosongkan = bayar penuh (${formatRupiah(payRemainingAmount)})`" />
              <p class="text-[10px] text-gray-400 mt-0.5">Sisa tagihan: {{ formatRupiah(payRemainingAmount) }}<template v-if="selectedPayStruk"> · sisa struk {{ formatRupiah(selectedPayStruk.remaining) }}</template></p>
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
              <img v-if="safeUrl(uploadedFileUrl)" :src="safeUrl(uploadedFileUrl)" class="w-9 h-9 rounded object-cover border border-gray-200" />
            </div>
            <div v-else class="shrink-0 w-9 h-9 rounded bg-red-50 border border-red-200 flex items-center justify-center">
              <svg class="w-4.5 h-4.5 text-red-400" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/></svg>
            </div>
            <p class="text-xs font-medium text-gray-700 truncate flex-1 min-w-0">{{ uploadedFileName }}</p>
            <button @click="removeUploadedFile" class="shrink-0 p-1 rounded hover:bg-red-100 text-gray-400 hover:text-red-500 transition-colors" title="Hapus">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
            </button>
          </div>
          </template>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showPayModal = false">Batal</AppButton>
        <AppButton :loading="payLoading" :disabled="uploadLoading || (paySource === 'office' && !payTargetsReady)" @click="submitPay">{{ paySource === 'outlet_cash' ? 'Kirim ke Kas Kasir Outlet' : 'Konfirmasi Pembayaran' }}</AppButton>
      </template>
    </AppModal>

    <!-- Kembalikan detail pembayaran ke Pembelian (tujuan salah / kedaluwarsa). -->
    <AppModal v-model="showReturnPayTo" title="Kembalikan ke Pembelian" size="md">
      <div class="space-y-3">
        <p class="text-sm text-gray-600">
          Detail pembayaran <strong>{{ returnTarget?.request_number }}</strong> ({{ returnTarget?.vendor_name || '-' }})
          dikembalikan ke bagian Pembelian. Tagihan ini tidak bisa dibayar sampai detailnya diperbaiki; Pembelian diberi tahu.
        </p>
        <div>
          <label for="payto-return-reason" class="mb-1 block text-sm font-medium text-gray-700">Alasan *</label>
          <textarea id="payto-return-reason" v-model="returnReason" rows="3" maxlength="1000"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
            placeholder="mis. Nomor VA sudah kedaluwarsa / rekening tidak sesuai invoice"></textarea>
          <p v-if="returnReason.trim().length > 0 && returnReason.trim().length < 5" class="mt-1 text-[11px] text-red-600">Alasan minimal 5 huruf.</p>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showReturnPayTo = false">Batal</AppButton>
        <AppButton variant="danger" :loading="returnLoading" :disabled="returnReason.trim().length < 5" @click="submitReturnPayTo">Kembalikan</AppButton>
      </template>
    </AppModal>

    <!-- Modal 1: Buat Pembayaran Manual (Di Luar Pengajuan) -->
    <AppModal v-model="showManualModal" title="Buat Pembayaran Manual (Di Luar Pengajuan)" size="2xl">
      <div class="space-y-4">
        <div class="rounded-lg border border-purple-200 bg-purple-50 px-3.5 py-2.5 text-xs text-purple-900">
          Pembayaran manual digunakan untuk tagihan atau pengeluaran <strong>di luar alur pengajuan pengadaan</strong> (mis. listrik, sewa, jasa konsultan, reimburse, dsb.). Setiap pembayaran manual wajib mendapat <strong>approve dari Manager Keuangan atau General Manager</strong> sebelum dapat dicairkan.
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="sm:col-span-2">
            <AppInput v-model="manualForm.title" label="Judul / Keperluan Pembayaran *" placeholder="Contoh: Tagihan Listrik & Internet Kantor Bulan November" maxlength="200" />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Kategori / Jenis *</label>
            <SearchableSelect v-model="manualForm.request_type" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500">
              <option value="jasa">Jasa / Biaya Operasional</option>
              <option value="barang">Barang / Perlengkapan</option>
              <option value="bahan">Bahan Baku</option>
            </SearchableSelect>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Unit Kerja / Outlet <span class="text-xs text-gray-400">(opsional)</span></label>
            <SearchSelect
              v-model="manualForm.work_unit_id"
              :options="workUnitOptions"
              placeholder="Umum / Kantor Pusat"
              searchPlaceholder="Cari unit kerja…"
              valueKey="id"
              labelKey="name"
            />
          </div>
          <div v-if="manualForm.request_type !== 'bahan'">
            <label class="block text-sm font-medium text-gray-700 mb-1">Projek <span class="text-xs text-gray-400">(opsional)</span></label>
            <SearchSelect
              v-model="manualForm.project_id"
              :options="projectFilterOptions"
              placeholder="Tanpa Projek"
              searchPlaceholder="Cari projek…"
            />
          </div>
          <div>
            <AppInput v-model="manualForm.invoice_number" label="No. Invoice / Tagihan" placeholder="Opsional, mis. INV/2026/11/001" />
          </div>
        </div>

        <!-- Penerima / Vendor -->
        <div class="rounded-lg border border-gray-200 bg-gray-50/70 p-3.5 space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs font-semibold uppercase tracking-wider text-gray-600">Penerima & Tujuan Pembayaran</p>
            <div class="flex items-center gap-3 text-xs">
              <label class="inline-flex items-center gap-1.5 cursor-pointer">
                <input type="radio" v-model="manualVendorMode" value="registered" class="accent-emerald-600" />
                <span>Vendor Terdaftar</span>
              </label>
              <label class="inline-flex items-center gap-1.5 cursor-pointer">
                <input type="radio" v-model="manualVendorMode" value="manual" class="accent-emerald-600" />
                <span>Ketik Penerima Manual</span>
              </label>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-if="manualVendorMode === 'registered'" class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700 mb-1">Pilih Vendor *</label>
              <SearchSelect
                v-model="manualForm.vendor_id"
                :options="vendorOptions"
                placeholder="Pilih vendor…"
                searchPlaceholder="Cari vendor…"
                valueKey="id"
                labelKey="name"
                @update:modelValue="onManualVendorSelect"
              />
            </div>
            <div v-else class="sm:col-span-2">
              <AppInput v-model="manualForm.vendor_name" label="Nama Penerima / Supplier *" placeholder="Nama perusahaan / perorangan penerima dana" maxlength="200" />
            </div>

            <div class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700 mb-1">Metode Tujuan Pembayaran *</label>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
                <label class="source-opt" :class="{ 'source-opt--on': manualForm.pay_to_method === 'vendor_account' }">
                  <input v-model="manualForm.pay_to_method" type="radio" value="vendor_account" class="sr-only" />
                  <span class="text-xs font-semibold text-gray-800">Transfer Bank</span>
                  <span class="text-[10px] text-gray-500">Rekening bank penerima</span>
                </label>
                <label class="source-opt" :class="{ 'source-opt--on': manualForm.pay_to_method === 'marketplace_va' }">
                  <input v-model="manualForm.pay_to_method" type="radio" value="marketplace_va" class="sr-only" />
                  <span class="text-xs font-semibold text-gray-800">Virtual Account (VA)</span>
                  <span class="text-[10px] text-gray-500">VA tagihan / marketplace</span>
                </label>
                <label class="source-opt" :class="{ 'source-opt--on': manualForm.pay_to_method === 'manual' }">
                  <input v-model="manualForm.pay_to_method" type="radio" value="manual" class="sr-only" />
                  <span class="text-xs font-semibold text-gray-800">Tunai / Lainnya</span>
                  <span class="text-[10px] text-gray-500">Pembayaran langsung</span>
                </label>
              </div>
            </div>

            <template v-if="manualForm.pay_to_method === 'vendor_account'">
              <AppInput v-model="manualForm.pay_to_bank_name" label="Nama Bank *" placeholder="Contoh: BCA, Mandiri, BRI" />
              <AppInput v-model="manualForm.pay_to_account_number" label="Nomor Rekening *" placeholder="Nomor rekening tujuan" />
              <div class="sm:col-span-2">
                <AppInput v-model="manualForm.pay_to_account_holder" label="Atas Nama Rekening" placeholder="Nama pemilik rekening (otomatis mengikuti penerima bila kosong)" />
              </div>
            </template>
            <template v-else-if="manualForm.pay_to_method === 'marketplace_va'">
              <AppInput v-model="manualForm.pay_to_marketplace" label="Penyedia / Marketplace *" placeholder="Contoh: PLN, Tokopedia, Shopee" />
              <AppInput v-model="manualForm.pay_to_order_number" label="No. Pesanan / ID Pelanggan" placeholder="Opsional" />
              <AppInput v-model="manualForm.pay_to_bank_name" label="Bank VA *" placeholder="Contoh: BCA, Mandiri" />
              <AppInput v-model="manualForm.pay_to_account_number" label="Nomor Virtual Account *" placeholder="Nomor VA" />
            </template>
            <template v-else>
              <div class="sm:col-span-2">
                <AppInput v-model="manualForm.pay_to_text" label="Keterangan Cara Bayar *" placeholder="Contoh: Tunai diserahkan langsung kepada penerima" />
              </div>
            </template>
          </div>
        </div>

        <!-- Rincian Item & Nominal -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label class="text-sm font-semibold text-gray-700">Rincian Item & Nominal *</label>
            <button type="button" @click="addManualItem" class="text-xs font-semibold text-emerald-700 hover:text-emerald-800 cursor-pointer">+ Tambah Baris</button>
          </div>
          <div class="space-y-2">
            <div v-for="(item, idx) in manualForm.items" :key="idx" class="grid grid-cols-12 gap-2 items-end rounded-lg border border-gray-200 bg-white p-2.5">
              <div class="col-span-12 sm:col-span-5">
                <label v-if="idx === 0" class="block text-[11px] font-medium text-gray-500 mb-1">Uraian / Nama Item *</label>
                <input v-model="item.name" type="text" placeholder="Uraian biaya / barang" class="w-full rounded-md border border-gray-300 px-2.5 py-1.5 text-xs focus:border-emerald-500 focus:outline-none" />
              </div>
              <div class="col-span-4 sm:col-span-2">
                <label v-if="idx === 0" class="block text-[11px] font-medium text-gray-500 mb-1">Qty</label>
                <input v-model.number="item.qty" type="number" min="1" class="w-full rounded-md border border-gray-300 px-2.5 py-1.5 text-xs focus:border-emerald-500 focus:outline-none" />
              </div>
              <div class="col-span-4 sm:col-span-2">
                <label v-if="idx === 0" class="block text-[11px] font-medium text-gray-500 mb-1">Satuan</label>
                <input v-model="item.unit" type="text" placeholder="paket" class="w-full rounded-md border border-gray-300 px-2.5 py-1.5 text-xs focus:border-emerald-500 focus:outline-none" />
              </div>
              <div class="col-span-4 sm:col-span-3 flex items-end gap-1.5">
                <div class="flex-1 min-w-0">
                  <label v-if="idx === 0" class="block text-[11px] font-medium text-gray-500 mb-1">Harga Satuan *</label>
                  <RupiahInput v-model="item.price" placeholder="0" />
                </div>
                <button v-if="manualForm.items.length > 1" type="button" @click="removeManualItem(idx)" class="mb-1 rounded p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 cursor-pointer" title="Hapus baris">
                  ✕
                </button>
              </div>
            </div>
          </div>
          <div class="flex justify-end rounded-lg bg-emerald-50 border border-emerald-200 px-3.5 py-2 text-sm">
            <span class="text-emerald-900 font-semibold">Total Pembayaran Manual: {{ formatRupiah(manualTotalAmount) }}</span>
          </div>
        </div>

        <!-- Upload Invoice & Catatan -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">File / Foto Invoice <span class="text-xs text-gray-400">(opsional)</span></label>
            <div v-if="manualInvoiceUploading" class="text-xs text-gray-500 py-2">Mengunggah berkas invoice…</div>
            <div v-else-if="!manualForm.invoice_url" class="relative">
              <label class="flex items-center gap-2 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-2 text-xs text-gray-600 hover:border-emerald-400 cursor-pointer">
                <span>📄 Pilih file / foto invoice (JPG, PNG, WEBP, PDF · maks 5MB)</span>
                <input type="file" class="absolute inset-0 w-full h-full opacity-0 cursor-pointer" accept=".jpg,.jpeg,.png,.webp,.pdf" @change="handleManualInvoiceUpload" />
              </label>
            </div>
            <div v-else class="flex items-center justify-between gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs">
              <a :href="safeUrl(manualForm.invoice_url)" target="_blank" rel="noopener noreferrer" class="font-medium text-emerald-800 underline truncate">📄 {{ manualInvoiceName || 'Berkas Invoice' }}</a>
              <button type="button" @click="manualForm.invoice_url = ''; manualInvoiceName = ''" class="text-red-600 hover:underline shrink-0 cursor-pointer">Hapus</button>
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Catatan Pengajuan</label>
            <textarea v-model="manualForm.notes" rows="2" maxlength="1000" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-xs focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" placeholder="Keterangan tambahan untuk Manager Keuangan / GM"></textarea>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showManualModal = false">Batal</AppButton>
        <AppButton :loading="manualSubmitting" :disabled="manualInvoiceUploading" @click="submitManualPayment">Kirim untuk Approve Mgr Keuangan / GM</AppButton>
      </template>
    </AppModal>

    <!-- Modal 2: Buat Pengajuan Pembayaran (dari Ceklis Menunggu Pembayaran) -->
    <AppModal v-model="showSubmitApprovalModal" title="Buat Pengajuan Pembayaran" size="lg">
      <div class="space-y-4">
        <div class="rounded-lg border border-amber-200 bg-amber-50 px-3.5 py-2.5 text-xs text-amber-900">
          Daftar tagihan di bawah ini akan dikelompokkan dalam satu <strong>Nomor Pengajuan Pembayaran</strong> dan dikirim untuk persetujuan <strong>Manager Keuangan atau General Manager</strong>. Pembayaran baru dapat dicairkan setelah disetujui.
        </div>

        <div class="max-h-64 overflow-y-auto rounded-lg border border-gray-200 divide-y divide-gray-100">
          <div v-for="t in submitApprovalTargets" :key="t.id" class="flex items-center justify-between gap-3 px-3.5 py-2.5 text-xs bg-white">
            <div class="min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="font-mono font-semibold text-gray-800">{{ t.request_number }}</span>
                <span v-if="t.is_manual_payment" class="rounded-full bg-purple-100 px-1.5 py-0.5 text-[10px] font-semibold text-purple-800">Manual</span>
              </div>
              <p class="text-gray-700 truncate">{{ (t.items || []).map(i => i.name).join(', ') || '-' }}</p>
              <p class="text-[11px] text-gray-500">Vendor: {{ t.vendor_name || '-' }} <span v-if="t.work_unit_name">· {{ t.work_unit_name }}</span></p>
            </div>
            <div class="text-right shrink-0">
              <span class="font-semibold text-emerald-700">{{ formatRupiah(remainingOf(t)) }}</span>
            </div>
          </div>
        </div>

        <div class="flex items-center justify-between rounded-lg bg-emerald-50 border border-emerald-200 px-3.5 py-2.5 text-sm">
          <span class="text-emerald-900 font-medium">Total {{ submitApprovalTargets.length }} Tagihan Diajukan</span>
          <span class="text-emerald-800 font-bold">{{ formatRupiah(submitApprovalTotal) }}</span>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Catatan Pengajuan Pembayaran <span class="text-xs text-gray-400">(opsional)</span></label>
          <textarea v-model="submitApprovalNotes" rows="2" maxlength="1000" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500" placeholder="Mis. Pengajuan pembayaran batch minggu ke-4 November"></textarea>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showSubmitApprovalModal = false">Batal</AppButton>
        <AppButton :loading="submitApprovalLoading" @click="confirmSubmitApproval">Kirim Pengajuan Pembayaran</AppButton>
      </template>
    </AppModal>

    <!-- Modal 3: Tolak Pengajuan Pembayaran (Manager Keuangan / General Manager) -->
    <AppModal v-model="showRejectApprovalModal" title="Tolak Pengajuan Pembayaran" size="md">
      <div class="space-y-3">
        <p class="text-sm text-gray-600">
          Anda akan menolak <strong>{{ rejectApprovalTargets.length }} pengajuan pembayaran</strong> senilai <strong>{{ formatRupiah(rejectApprovalTotal) }}</strong>. Tagihan yang ditolak tidak dapat dicairkan sebelum diajukan ulang.
        </p>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700">Alasan Penolakan *</label>
          <textarea v-model="rejectApprovalReason" rows="3" maxlength="1000"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
            placeholder="Jelaskan alasan penolakan (minimal 3 karakter)…"></textarea>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showRejectApprovalModal = false">Batal</AppButton>
        <AppButton variant="danger" :loading="approvalDecisionLoading" :disabled="rejectApprovalReason.trim().length < 3" @click="confirmRejectApproval">Tolak Pengajuan</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, watch, onMounted } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { vendorsApi } from '@/api/vendors.js'
import { apiClient } from '@/api/client.js'
import { formatRupiah, formatDateTime, formatDate } from '@/utils/format.js'
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
import { workUnitsApi } from '@/api/workUnits.js'
import { purchaseTypeLabel, normalizePurchaseType, isPhysicalPurchaseType, purchaseStatusLabel, purchaseStatusBadge, isConflictError } from '@/utils/procurement.js'
import PurchasePayToCard from '@/components/PurchasePayToCard.vue'
import PurchaseUrgencyBadges from '@/components/PurchaseUrgencyBadges.vue'
import EmergencyPurchasePanel from '@/components/EmergencyPurchasePanel.vue'
import { safeUrl, isImageUrl } from '@/utils/safeUrl.js'

const toast = useToastStore()
const authStore = useAuthStore()
// Melihat daftar tagihan, menyetujui pengajuan bayar (Mgr Keuangan / GM), dan mencairkannya izin terpisah.
const canPay = computed(() => authStore.hasPermission('finance.payments.pay'))
const canApprovePayment = computed(() => {
  if (authStore.hasPermission('finance.payments.approve')) return true
  const role = String(authStore.admin?.role || '').trim().toLowerCase()
  return ['superadmin', 'admin', 'manager keuangan', 'general manager', 'direktur'].includes(role)
})

const loading = ref(false)
const errorMsg = ref('')
const requests = ref([])
const page = ref(1)
const totalPages = ref(1)
const filterStatus = ref('')
const filterType = ref('')
// Jenis: tingkat kebutuhan.
const filterKind = ref('')
const KIND_FILTERS = [
  { value: '', label: 'Semua Tingkat' },
  { value: 'urgent', label: 'Urgent' },
  { value: 'prioritas', label: 'Prioritas' },
  { value: 'rutin', label: 'Rutin' },
]
const filterApproval = ref('')
const APPROVAL_FILTERS = [
  { value: '', label: 'Semua Tahap Persetujuan' },
  { value: 'unsubmitted', label: 'Belum Diajukan Bayar' },
  { value: 'pending', label: 'Menunggu Approve Mgr Keuangan / GM' },
  { value: 'approved', label: 'Disetujui — Siap Dibayar' },
  { value: 'rejected', label: 'Ditolak Mgr Keuangan / GM' },
  { value: 'manual', label: 'Hanya Pembayaran Manual' },
]
// Parameter saringan jenis & persetujuan untuk daftar & ekspor (cermin PurchaseListExtra).
function kindParams() {
  const out = {}
  const k = filterKind.value
  if (k === 'urgent' || k === 'prioritas' || k === 'rutin') out.urgency = k
  const a = filterApproval.value
  if (a === 'manual') {
    out.manual_only = 'true'
  } else if (a) {
    out.payment_approval = a
  }
  return out
}
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
  { value: 'bahan', label: 'Bahan Baku' },
  { value: 'jasa', label: 'Jasa' },
]

const bankAccounts = ref([])
const bankAccountsError = ref('')
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
// Sumber dana pembayaran: 'office' (Kantor Pusat) | 'outlet_cash' (Kas Kasir Outlet).
const paySource = ref('office')
const payoutForm = ref({ amount: 0, recipient_name: '', notes: '' })
// Kas Kasir Outlet: satu pengajuan milik outlet yang tujuan pembayarannya
// siap; VA marketplace selalu lewat transfer Kantor Pusat (cermin server).
const canUseOutletCash = computed(() => {
  const t = payTargets.value[0]
  return payTargets.value.length === 1 && !!t?.outlet_id && !!t?.pay_to?.ready && t?.pay_to?.effective !== 'marketplace_va'
})
const outletCashHint = computed(() => {
  const t = payTargets.value[0]
  if (payTargets.value.length !== 1 || !t?.outlet_id) return 'Hanya untuk pengajuan milik outlet'
  if (t?.pay_to?.effective === 'marketplace_va') return 'VA Marketplace dibayar lewat transfer'
  if (!t?.pay_to?.ready) return 'Detail pembayaran belum siap'
  return `Diserahkan tunai dari laci ${payOutletName.value}`
})
// Tujuan pembayaran milik Pembelian: semua tagihan yang dibayar harus siap.
const payTargetsReady = computed(() => payTargets.value.length > 0 && payTargets.value.every(t => !!t?.pay_to?.ready))
const payOutletName = computed(() => payTargets.value[0]?.outlet_name || 'outlet')
const payLoading = ref(false)
const payVendor = ref(null)
const payVendorLoading = ref(false)
// Galat memuat vendor dibedakan dari "belum memilih vendor": dokumennya sudah
// menunjuk vendor terdaftar, jadi pesan tidak boleh menyuruh bagian pembelian
// memilih vendor lewat Isi Harga.
const payVendorError = ref('')
// Isian Keuangan saat membayar: rekening asal, nominal, biaya bank (bila beda bank), referensi, bukti.
// Tujuan pembayaran TIDAK diisi di sini — milik Pembelian (pay_to).
function newPayForm() {
  return { payment_account_source: '', payment_proof: '', payment_notes: '', payment_amount: 0, bank_fee: 0 }
}
const payForm = ref(newPayForm())

const BANK_FEE_PRESETS = [
  { value: 0,    label: 'Rp 0 (Gratis)' },
  { value: 2500, label: 'Rp 2.500 (BI-FAST)' },
  { value: 2900, label: 'Rp 2.900' },
  { value: 6500, label: 'Rp 6.500 (Online)' },
]

// Normalisasi nama bank agar "Bank BCA", "PT Bank Central Asia", dan "BCA"
// dikenali sebagai bank yang sama, sedangkan bank berbeda memunculkan input biaya bank.
function normalizeBankKey(raw) {
  if (!raw) return ''
  let s = String(raw).toLowerCase().replace(/[^a-z0-9\s]/g, ' ').replace(/\s+/g, ' ').trim()
  s = s.replace(/\b(pt|tbk|persero|bank)\b/g, ' ').replace(/\s+/g, ' ').trim()
  const aliases = {
    'central asia': 'bca',
    'bca': 'bca',
    'rakyat indonesia': 'bri',
    'bri': 'bri',
    'mandiri': 'mandiri',
    'negara indonesia': 'bni',
    'bni': 'bni',
    'syariah indonesia': 'bsi',
    'bsi': 'bsi',
    'cimb niaga': 'cimb',
    'cimb': 'cimb',
    'danamon indonesia': 'danamon',
    'danamon': 'danamon',
    'permata': 'permata',
    'tabungan negara': 'btn',
    'btn': 'btn',
    'ocbc nisp': 'ocbc',
    'ocbc': 'ocbc',
    'panin': 'panin',
    'mega': 'mega',
    'btpn': 'btpn',
    'jago': 'jago',
    'seabank': 'seabank',
    'Maybank': 'maybank',
    'maybank indonesia': 'maybank',
  }
  return aliases[s] || s
}

const selectedSourceAccount = computed(() =>
  bankAccounts.value.find(a => a.id === payForm.value.payment_account_source) || null
)
const sourceBankName = computed(() => (selectedSourceAccount.value?.bank_name || '').trim())
const destBankName = computed(() => {
  const pt = payTargets.value[0]?.pay_to
  if (!pt) return (payVendor.value?.bank_name || '').trim()
  if (pt.effective === 'vendor_account') {
    return (pt.bank_name || pt.vendor_bank_name || payVendor.value?.bank_name || '').trim()
  }
  return (pt.bank_name || '').trim()
})
const isSameBank = computed(() => {
  const src = normalizeBankKey(sourceBankName.value)
  const dst = normalizeBankKey(destBankName.value)
  return !!src && !!dst && src === dst
})
const isDifferentBank = computed(() => {
  const src = normalizeBankKey(sourceBankName.value)
  if (!src) return false
  const dst = normalizeBankKey(destBankName.value)
  return !dst || src !== dst
})

watch(isSameBank, (same) => {
  if (same && payForm.value.bank_fee > 0) {
    payForm.value.bank_fee = 0
  }
})

const paymentHistories = ref([])
const paymentHistoriesLoading = ref(false)

const uploadLoading = ref(false)
const uploadedFileUrl = ref('')
const uploadedFileName = ref('')
const uploadedFileIsImage = computed(() => /\.(jpg|jpeg|png|webp)$/i.test(uploadedFileName.value))

// payment_proof = "<no. referensi> | <URL bukti>". Bagian berupa path hanya
// dipasang sebagai tautan/gambar bila lolos safeUrl (T-75); path lain yang
// tidak sah dibuang — tidak ditampilkan sebagai tautan maupun sebagai no. ref.
function parsedPaymentProof(proof) {
  if (!proof) return { refNumber: '', fileUrl: '', isImage: false }
  const parts = String(proof).split(' | ').map(s => s.trim()).filter(Boolean)
  const isPath = (p) => p.startsWith('/') || p.startsWith('\\') || p.includes('://')
  const fileUrl = safeUrl(parts.find(isPath) || '')
  const refNumber = parts.find(p => !isPath(p)) || ''
  const isImage = isImageUrl(fileUrl)
  return { refNumber, fileUrl, isImage }
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

// Sisa tagihan MILIK dokumen ini, dihitung dalam sen seperti server — selisih
// pecahan float tidak boleh memunculkan "sisa Rp0,01".
function remainingOf(r) {
  const cents = Math.round(((Number(r?.total_final) || 0) - (Number(r?.paid_amount) || 0)) * 100)
  return cents > 0 ? cents / 100 : 0
}
// Bisa dibayar: status masih memiliki sisa tagihan, tujuan pembayaran siap,
// DAN sudah disetujui oleh Manager Keuangan atau General Manager.
const PAYABLE_STATUSES = ['payment_requested', 'partial', 'received']
function isCheckable(r) {
  return !!r && PAYABLE_STATUSES.includes(r.status) && remainingOf(r) > 0 && !payoutOpen(r)
}
function canSubmitRowForApproval(r) {
  return isCheckable(r) && !payToBlocked(r) && (!r.payment_approval_status || r.payment_approval_status === 'rejected')
}
function canApproveRowPayment(r) {
  return isCheckable(r) && r.payment_approval_status === 'pending'
}
function isPayable(r) {
  return isCheckable(r) && !!r.pay_to?.ready && r.payment_approval_status === 'approved'
}
function showApprovalBadge(r) {
  if (!r) return false
  return !!r.payment_approval_status || (PAYABLE_STATUSES.includes(r.status) && remainingOf(r) > 0)
}
function paymentApprovalLabel(r) {
  switch (r?.payment_approval_status) {
    case 'pending':
      return '⏳ Menunggu Approve Mgr Keuangan / GM'
    case 'approved':
      return r.payment_approved_by ? `✓ Disetujui Bayar (${r.payment_approved_by})` : '✓ Disetujui Bayar'
    case 'rejected':
      return r.payment_rejected_by ? `✕ Ditolak (${r.payment_rejected_by})` : '✕ Pengajuan Bayar Ditolak'
    default:
      return 'Belum Diajukan Bayar'
  }
}
function paymentApprovalBadgeCls(r) {
  const tone = {
    pending:  'bg-amber-50 text-amber-800 border-amber-300',
    approved: 'bg-emerald-50 text-emerald-800 border-emerald-300',
    rejected: 'bg-red-50 text-red-700 border-red-300',
  }[r?.payment_approval_status] || 'bg-slate-100 text-slate-600 border-slate-200'
  return `inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold ${tone}`
}
function paymentApprovalHint(r) {
  if (!r) return ''
  if (r.payment_approval_status === 'pending') {
    return `Diajukan oleh ${r.payment_submitted_by || '-'}${r.payment_batch_number ? ` (${r.payment_batch_number})` : ''}`
  }
  if (r.payment_approval_status === 'approved') {
    return `Disetujui oleh ${r.payment_approved_by || '-'}`
  }
  if (r.payment_approval_status === 'rejected') {
    return `Ditolak oleh ${r.payment_rejected_by || '-'}: ${r.payment_rejection_reason || '-'}`
  }
  return 'Ceklis tagihan ini lalu klik Buat Pengajuan Pembayaran untuk meminta persetujuan Mgr Keuangan / GM'
}
function canFinishJasa(r) {
  return !!r && normalizePurchaseType(r.request_type) === 'jasa' && r.status === 'paid'
}
async function finishJasa(r) {
  if (!r) return
  try {
    await purchaseApi.updateStatus(r.id, { action: 'receive', actor_name: adminName() })
    toast.success(`Pengajuan jasa ${r.request_number || ''} ditandai selesai.`)
    if (detail.value?.id === r.id) showDetail.value = false
    fetchList()
    fetchStats()
  } catch (err) { toast.error(err?.message || 'Gagal menyelesaikan pengajuan jasa.') }
}
// Tujuan pembayaran milik Pembelian: tagihan yang detailnya dikembalikan atau
// belum diisi tidak bisa dibayar sampai Pembelian melengkapinya.
function payToBlocked(r) {
  // Pembelian Darurat dibayar tunai lewat tablet kasir, tanpa detail Pembelian.
  return !!r && !r.emergency && PAYABLE_STATUSES.includes(r.status) && remainingOf(r) > 0 && !r.pay_to?.ready
}
function payToBlockLabel(r) { return r?.pay_to?.returned_at ? 'Dikembalikan ke Pembelian' : 'Menunggu detail Pembelian' }
function payToBlockCls(r) {
  const tone = r?.pay_to?.returned_at ? 'bg-red-50 text-red-700 border-red-200' : 'bg-amber-50 text-amber-800 border-amber-200'
  return `inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold ${tone}`
}
// Kembalikan ke Pembelian: selama masih ditagih, tidak sedang dibayar dari
// Kas Kasir Outlet, dan belum dikembalikan (cermin payToEditableTx).
function canReturnPayTo(r) {
  return !!r && !r.emergency && !r.is_manual_payment && PAYABLE_STATUSES.includes(r.status) && remainingOf(r) > 0 && !payoutOpen(r) && !r.pay_to?.returned_at
}
const showReturnPayTo = ref(false)
const returnTarget = ref(null)
const returnReason = ref('')
const returnLoading = ref(false)
function openReturnPayTo(row) {
  returnTarget.value = row
  returnReason.value = ''
  showReturnPayTo.value = true
}
async function submitReturnPayTo() {
  const t = returnTarget.value
  const reason = returnReason.value.trim()
  if (!t || returnLoading.value) return
  if (reason.length < 5) { toast.error('Alasan minimal 5 huruf.'); return }
  returnLoading.value = true
  try {
    await purchaseApi.returnPaymentDetails(t.id, { reason, version: t.version })
    toast.success(`Detail pembayaran ${t.request_number} dikembalikan ke Pembelian.`)
    showReturnPayTo.value = false
    showPayModal.value = false
    if (detail.value?.id === t.id) showDetail.value = false
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) {
    if (isConflictError(err)) {
      showReturnPayTo.value = false
      toast.error('Pengajuan berubah sejak dibuka — data terbaru sudah dimuat, periksa lagi.')
      fetchList()
    } else {
      toast.error(err?.message || 'Gagal mengembalikan detail pembayaran.')
    }
  } finally { returnLoading.value = false }
}
// Tugas Kas Kasir Outlet yang masih terbuka mengunci pengajuan dari
// pembayaran Kantor Pusat (satu pembayaran, satu sumber uang).
function payoutOpen(r) { return ['pending', 'claimed'].includes(r?.cash_payout?.status) }
function payoutLabel(p) {
  return { pending: 'Menunggu Kas Outlet', claimed: 'Diproses kasir outlet', returned: 'Dikembalikan kasir' }[p?.status] || ''
}
function payoutBadgeCls(p) {
  const tone = { pending: 'bg-amber-50 text-amber-800 border-amber-200', claimed: 'bg-sky-50 text-sky-800 border-sky-200', returned: 'bg-red-50 text-red-700 border-red-200' }[p?.status] || 'bg-gray-50 text-gray-600 border-gray-200'
  return `inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold ${tone}`
}
function payoutHint(p) {
  if (!p) return ''
  const base = `${formatRupiah(p.amount)} dari kas ${p.outlet_name || 'outlet'} untuk ${p.recipient_name} · ditetapkan ${p.assigned_by || '-'}`
  return p.status === 'claimed' ? `${base} · diproses ${p.cashier_name || 'kasir'}` : base
}
async function cancelPayout(row) {
  const p = row?.cash_payout
  if (!p || !window.confirm(`Tarik tugas Kas Kasir Outlet ${formatRupiah(p.amount)} untuk ${row.request_number}? Tablet kasir tidak lagi menampilkannya.`)) return
  try {
    await purchaseApi.cancelCashPayout(p.id, { reason: 'Ditarik Bagian Keuangan' })
    toast.success('Tugas Kas Kasir Outlet ditarik.')
    fetchList()
  } catch (err) { toast.error(err?.message || 'Gagal menarik tugas.') }
}

// Daftar halaman ini (exclude_masters) sudah memakai nilai milik dokumen
// sendiri, tetapi detail induk (GET /:id) menjumlahkan semua pecahannya.
// Untuk ditagih di sini yang dipakai hanya bagian induk sendiri — kalau tidak,
// sisa tagihan induk ikut membengkak sebesar nilai pecahan.
function ownBilling(d) {
  if (!d || d.split_status !== 'master' || !Array.isArray(d.children) || !d.children.length) return d
  const sum = (k) => d.children.reduce((acc, c) => acc + (Number(c?.[k]) || 0), 0)
  const own = (k) => Math.max(0, Math.round(((Number(d[k]) || 0) - sum(k)) * 100) / 100)
  return {
    ...d,
    total_final: own('total_final'),
    total_hps: own('total_hps'),
    total_amount: own('total_amount'),
    family_total_final: Number(d.total_final) || 0,
  }
}
// Kolom centang selalu tampil agar Keuangan bisa menceklis tagihan menunggu pembayaran
// untuk dibuatkan pengajuan pembayaran, dan Mgr Keuangan / GM bisa menyetujui sekaligus.
const showSelect = computed(() => true)
const selectableRows = computed(() => requests.value.filter(isCheckable))
const allSelectableChecked = computed(() => selectableRows.value.length > 0 && selectableRows.value.every(r => selectedIds.value.has(r.id)))
const selectedRows = computed(() => requests.value.filter(r => selectedIds.value.has(r.id)))
const selectedSubmittableRows = computed(() => selectedRows.value.filter(canSubmitRowForApproval))
const selectedApprovableRows = computed(() => selectedRows.value.filter(canApproveRowPayment))
const selectedPayableRows = computed(() => selectedRows.value.filter(isPayable))
const selectedVendorSummary = computed(() => {
  const names = [...new Set(selectedRows.value.map(r => r.vendor_name || '-'))]
  if (names.length === 0) return '-'
  if (names.length === 1) return names[0]
  return `${names.length} vendor`
})
// Yang dibayar server adalah SISA tiap tagihan, bukan harga penuh — total
// pilihan & batch mengikuti sisa supaya nominal transfer tidak kelebihan.
const selectedTotal = computed(() => selectedRows.value.reduce((s, r) => s + remainingOf(r), 0))
const payTotalAmount = computed(() => payTargets.value.reduce((s, r) => s + (r.total_final || 0), 0))
const payRemainingAmount = computed(() => payTargets.value.reduce((s, r) => s + remainingOf(r), 0))
// Nominal yang disalin dari kartu tujuan = yang akan dicatat server: isian
// Jumlah Bayar, atau sisa struk terpilih, atau sisa tagihan.
const payCopyAmount = computed(() => {
  if (payTargets.value.length !== 1) return payRemainingAmount.value
  if (payForm.value.payment_amount > 0) return payForm.value.payment_amount
  if (selectedPayStruk.value) return Math.min(selectedPayStruk.value.remaining, payRemainingAmount.value)
  return payRemainingAmount.value
})
const effectiveTransferAmount = computed(() => Number(payCopyAmount.value) || 0)

function toggleSelect(row) {
  const next = new Set(selectedIds.value)
  if (next.has(row.id)) {
    next.delete(row.id)
  } else {
    next.add(row.id)
  }
  selectedIds.value = next
}

function toggleSelectAll(e) {
  if (e.target.checked) {
    selectedIds.value = new Set(selectableRows.value.map(r => r.id))
  } else {
    selectedIds.value = new Set()
  }
}

function clearSelection() { selectedIds.value = new Set() }

// ── Pembayaran Manual (Di Luar Pengajuan) ─────────────────────────────────
const workUnits = ref([])
const vendorsList = ref([])
const workUnitOptions = computed(() => [
  { id: '', name: 'Umum / Kantor Pusat' },
  ...workUnits.value.map(w => ({ id: w.id, name: w.name })),
])
const vendorOptions = computed(() =>
  vendorsList.value.filter(v => v.is_active !== false).map(v => ({
    id: v.id,
    name: v.bank_name && v.account_number ? `${v.name} — ${v.bank_name} ${v.account_number}` : v.name,
  }))
)

async function fetchWorkUnits() {
  try {
    const data = await workUnitsApi.myWorkUnits()
    workUnits.value = Array.isArray(data) ? data : (data?.data || [])
  } catch {
    try {
      const data = await workUnitsApi.list()
      workUnits.value = Array.isArray(data) ? data : (data?.data || [])
    } catch { workUnits.value = [] }
  }
}

async function fetchVendors() {
  try {
    const data = await vendorsApi.list({ active_only: 'true', limit: 500 })
    vendorsList.value = Array.isArray(data) ? data : (data?.vendors || data?.data || [])
  } catch { vendorsList.value = [] }
}

const showManualModal = ref(false)
const manualVendorMode = ref('registered')
const manualSubmitting = ref(false)
const manualInvoiceUploading = ref(false)
const manualInvoiceName = ref('')
function newManualForm() {
  return {
    title: '',
    request_type: 'jasa',
    work_unit_id: '',
    project_id: '',
    vendor_id: '',
    vendor_name: '',
    pay_to_method: 'vendor_account',
    pay_to_bank_name: '',
    pay_to_account_number: '',
    pay_to_account_holder: '',
    pay_to_marketplace: '',
    pay_to_order_number: '',
    pay_to_text: '',
    invoice_number: '',
    invoice_url: '',
    notes: '',
    items: [{ name: '', qty: 1, unit: 'paket', price: 0 }],
  }
}
const manualForm = ref(newManualForm())
const manualTotalAmount = computed(() =>
  (manualForm.value.items || []).reduce((sum, it) => sum + ((Number(it.qty) || 0) * (Number(it.price) || 0)), 0)
)

function openManualModal() {
  manualForm.value = newManualForm()
  manualVendorMode.value = vendorsList.value.length > 0 ? 'registered' : 'manual'
  manualInvoiceName.value = ''
  showManualModal.value = true
}

function onManualVendorSelect(vId) {
  const v = vendorsList.value.find(item => item.id === vId)
  if (!v) return
  manualForm.value.vendor_name = v.name || ''
  if (v.bank_name || v.account_number) {
    manualForm.value.pay_to_method = 'vendor_account'
    manualForm.value.pay_to_bank_name = v.bank_name || ''
    manualForm.value.pay_to_account_number = v.account_number || ''
    manualForm.value.pay_to_account_holder = v.account_holder || v.name || ''
  }
}

function addManualItem() {
  manualForm.value.items.push({ name: '', qty: 1, unit: 'paket', price: 0 })
}
function removeManualItem(idx) {
  if (manualForm.value.items.length > 1) manualForm.value.items.splice(idx, 1)
}

async function handleManualInvoiceUpload(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) { toast.error('Ukuran file maksimal 5MB.'); e.target.value = ''; return }
  manualInvoiceUploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', file)
    const res = await apiClient.post('/admin/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = safeUrl(res?.url ?? res?.data?.url)
    if (!url) throw new Error('Server tidak mengembalikan alamat berkas yang sah.')
    manualForm.value.invoice_url = url
    manualInvoiceName.value = res?.filename ?? res?.data?.filename ?? file.name
  } catch (err) {
    toast.error(err?.message ?? 'Gagal mengunggah file invoice.')
  } finally {
    manualInvoiceUploading.value = false
    e.target.value = ''
  }
}

async function submitManualPayment() {
  const f = manualForm.value
  if (!f.title.trim()) { toast.error('Judul / keperluan pembayaran wajib diisi.'); return }
  const useRegistered = manualVendorMode.value === 'registered'
  if (useRegistered && !f.vendor_id) { toast.error('Pilih vendor terdaftar atau beralih ke Penerima Manual.'); return }
  if (!useRegistered && !f.vendor_name.trim()) { toast.error('Nama penerima / supplier wajib diisi.'); return }
  if (f.pay_to_method === 'vendor_account' && (!f.pay_to_bank_name.trim() || !f.pay_to_account_number.trim())) {
    toast.error('Nama bank dan nomor rekening tujuan wajib diisi.'); return
  }
  if (f.pay_to_method === 'marketplace_va' && (!f.pay_to_marketplace.trim() || !f.pay_to_bank_name.trim() || !f.pay_to_account_number.trim())) {
    toast.error('Penyedia/marketplace, bank VA, dan nomor VA wajib diisi.'); return
  }
  const validItems = f.items.filter(it => it.name.trim() && Number(it.price) > 0)
  if (!validItems.length) { toast.error('Isi minimal 1 rincian item dengan nominal lebih dari 0.'); return }

  manualSubmitting.value = true
  try {
    const payload = {
      title: f.title.trim(),
      request_type: f.request_type,
      work_unit_id: f.work_unit_id || '',
      project_id: f.request_type === 'bahan' ? '' : (f.project_id || ''),
      vendor_id: useRegistered ? f.vendor_id : '',
      vendor_name: f.vendor_name.trim(),
      pay_to_method: f.pay_to_method,
      pay_to_bank_name: f.pay_to_bank_name.trim(),
      pay_to_account_number: f.pay_to_account_number.trim(),
      pay_to_account_holder: f.pay_to_account_holder.trim(),
      pay_to_marketplace: f.pay_to_marketplace.trim(),
      pay_to_order_number: f.pay_to_order_number.trim(),
      pay_to_text: f.pay_to_text.trim(),
      invoice_number: f.invoice_number.trim(),
      invoice_url: f.invoice_url || '',
      notes: f.notes.trim(),
      items: validItems.map(it => ({
        name: it.name.trim(),
        qty: Math.max(1, Number(it.qty) || 1),
        unit: (it.unit || 'paket').trim(),
        price: Number(it.price) || 0,
      })),
    }
    const created = await purchaseApi.createManualPayment(payload)
    toast.success(`Pembayaran manual ${created?.request_number || ''} berhasil dibuat & menunggu approve Mgr Keuangan / GM.`)
    showManualModal.value = false
    fetchList()
    fetchStats()
  } catch (err) {
    toast.error(err?.message || 'Gagal membuat pembayaran manual.')
  } finally {
    manualSubmitting.value = false
  }
}

// ── Pengajuan & Persetujuan Pembayaran (Ceklis → Mgr Keuangan / GM) ────────
const showSubmitApprovalModal = ref(false)
const submitApprovalTargets = ref([])
const submitApprovalNotes = ref('')
const submitApprovalLoading = ref(false)
const submitApprovalTotal = computed(() => submitApprovalTargets.value.reduce((s, r) => s + remainingOf(r), 0))

function openSubmitApprovalModal(rows) {
  const targets = (rows || []).filter(canSubmitRowForApproval)
  if (!targets.length) {
    toast.error('Tidak ada tagihan yang bisa diajukan pembayarannya.')
    return
  }
  submitApprovalTargets.value = targets
  submitApprovalNotes.value = ''
  showSubmitApprovalModal.value = true
}

async function confirmSubmitApproval() {
  if (!submitApprovalTargets.value.length || submitApprovalLoading.value) return
  submitApprovalLoading.value = true
  try {
    const res = await purchaseApi.submitPaymentApprovals({
      ids: submitApprovalTargets.value.map(r => r.id),
      notes: submitApprovalNotes.value.trim(),
    })
    const batchNo = res?.batch_number || ''
    toast.success(`Pengajuan pembayaran ${batchNo ? `(${batchNo}) ` : ''}untuk ${submitApprovalTargets.value.length} tagihan berhasil dikirim ke Manager Keuangan / GM.`)
    showSubmitApprovalModal.value = false
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) {
    toast.error(err?.message || 'Gagal mengajukan pembayaran.')
  } finally {
    submitApprovalLoading.value = false
  }
}

const approvalDecisionLoading = ref(false)
const showRejectApprovalModal = ref(false)
const rejectApprovalTargets = ref([])
const rejectApprovalReason = ref('')
const rejectApprovalTotal = computed(() => rejectApprovalTargets.value.reduce((s, r) => s + remainingOf(r), 0))

async function approvePaymentRows(rows) {
  const targets = (rows || []).filter(canApproveRowPayment)
  if (!targets.length || approvalDecisionLoading.value) return
  const total = targets.reduce((s, r) => s + remainingOf(r), 0)
  const label = targets.length === 1
    ? `Setujui pembayaran untuk ${targets[0].request_number} (${targets[0].vendor_name || '-'}) senilai ${formatRupiah(total)}?`
    : `Setujui ${targets.length} pengajuan pembayaran senilai total ${formatRupiah(total)}?`
  if (!window.confirm(label)) return
  approvalDecisionLoading.value = true
  try {
    await purchaseApi.decidePaymentApprovals({
      ids: targets.map(r => r.id),
      decision: 'approve',
    })
    toast.success(`${targets.length} pengajuan pembayaran berhasil disetujui dan kini siap dibayar.`)
    if (detail.value && targets.some(t => t.id === detail.value.id)) {
      showDetail.value = false
    }
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) {
    toast.error(err?.message || 'Gagal menyetujui pengajuan pembayaran.')
  } finally {
    approvalDecisionLoading.value = false
  }
}

function openRejectApprovalModal(rows) {
  const targets = (rows || []).filter(canApproveRowPayment)
  if (!targets.length) return
  rejectApprovalTargets.value = targets
  rejectApprovalReason.value = ''
  showRejectApprovalModal.value = true
}

async function confirmRejectApproval() {
  const reason = rejectApprovalReason.value.trim()
  if (reason.length < 3 || !rejectApprovalTargets.value.length || approvalDecisionLoading.value) return
  approvalDecisionLoading.value = true
  try {
    await purchaseApi.decidePaymentApprovals({
      ids: rejectApprovalTargets.value.map(r => r.id),
      decision: 'reject',
      reason,
    })
    toast.success(`${rejectApprovalTargets.value.length} pengajuan pembayaran ditolak.`)
    showRejectApprovalModal.value = false
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) {
    toast.error(err?.message || 'Gagal menolak pengajuan pembayaran.')
  } finally {
    approvalDecisionLoading.value = false
  }
}

// Label & warna status dari satu sumber (pending = Menunggu Verifikasi,
// verified = Menunggu Persetujuan, …); status tak dikenal tampil netral.
function statusBadge(s) { return purchaseStatusBadge(s) }
function statusLabel(s) { return purchaseStatusLabel(s) }

// Penerimaan barang berdiri sendiri dari pembayaran: ada vendor yang mengirim
// dulu baru ditagih, ada yang minta dibayar di muka. Bagian pembayaran perlu
// tahu bedanya — membayar barang yang belum datang adalah risiko yang berbeda.
function receiptLabel(row) {
  if (row.receipt_status === 'received') return 'Barang sudah diterima'
  if (row.receipt_status === 'partial') return 'Barang diterima sebagian'
  return 'Barang belum diterima'
}
function typeBadgeCls(t) {
  const v = normalizePurchaseType(t)
  return `type-badge type-${v === 'bahan' || v === 'barang' ? v : 'jasa'}`
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

onMounted(() => { fetchList(); fetchStats(); fetchBankAccounts(); fetchProjects(); fetchWorkUnits(); fetchVendors() })

async function fetchBankAccounts() {
  try {
    const data = await bankAccountsApi.list()
    bankAccounts.value = Array.isArray(data) ? data : []
    bankAccountsError.value = ''
  } catch (err) {
    bankAccounts.value = []
    // Dulu galat ini ditelan: pilihan "Rekening Asal" kosong tanpa penjelasan
    // dan pembayaran selalu tertahan "Rekening asal wajib diisi".
    bankAccountsError.value = err?.status === 403
      ? 'Daftar rekening perusahaan tidak bisa dimuat: akun ini belum punya izin Rekening Bank (Lihat). Minta admin menambahkannya ke role Anda — tanpa rekening asal, pembayaran tidak bisa dicatat.'
      : (err?.message || 'Daftar rekening perusahaan gagal dimuat') + ' — tutup form ini lalu muat ulang halaman.'
  }
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
    const params = { page: page.value, limit: 20, parent_id: 'all', exclude_masters: 'true', ...kindParams() }
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
    const params = { ...kindParams() }
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

// Respons yang datang terlambat milik dokumen yang sudah ditutup tidak boleh
// menimpa detail/form dokumen yang sedang terbuka: tiap pemuatan bernomor urut
// dan hanya respons terbaru yang dipakai. Dulu histori pembayaran (rekening
// tujuan, vendor) dokumen lain bisa tampil di detail dokumen yang dibuka.
let detailSeq = 0
let historiesSeq = 0
let payVendorSeq = 0

async function viewDetail(row) {
  const seq = ++detailSeq
  try {
    const d = ownBilling(await purchaseApi.get(row.id))
    if (seq !== detailSeq) return
    detail.value = d
    detailItems.value = detail.value?.items || []
    showDetail.value = true
    fetchPaymentHistories(row.id)
  } catch { if (seq === detailSeq) toast.error('Gagal memuat detail.') }
}

async function fetchPaymentHistories(id) {
  const seq = ++historiesSeq
  paymentHistories.value = []
  paymentHistoriesLoading.value = true
  try {
    const data = await purchaseApi.getPaymentHistories(id)
    if (seq === historiesSeq) paymentHistories.value = Array.isArray(data) ? data : []
  } catch { if (seq === historiesSeq) paymentHistories.value = [] }
  finally { if (seq === historiesSeq) paymentHistoriesLoading.value = false }
}

// Struk pembelian tagihan yang sedang dibayar (hanya pembayaran satu
// pengajuan). Bernomor urut seperti pemuatan lain: respons milik form yang
// sudah ditutup tidak boleh mengisi form yang sedang terbuka.
const payStruks = ref([])
const payStrukId = ref('')
let payStruksSeq = 0
const payStrukOptions = computed(() => payStruks.value.filter(st => st.remaining > 0))
const selectedPayStruk = computed(() => payStrukOptions.value.find(st => st.id === payStrukId.value) || null)

async function fetchPayStruks(id) {
  const seq = ++payStruksSeq
  payStruks.value = []
  payStrukId.value = ''
  try {
    const d = await purchaseApi.get(id)
    if (seq !== payStruksSeq) return
    payStruks.value = Array.isArray(d?.struks) ? d.struks : []
    // Struk terlama yang belum lunas dipilih lebih dulu.
    payStrukId.value = payStruks.value.find(st => st.remaining > 0)?.id || ''
  } catch { if (seq === payStruksSeq) payStruks.value = [] }
}

function openPayModal(row) {
  payTargets.value = [row]
  paySource.value = 'office'
  payoutForm.value = { amount: 0, recipient_name: row.vendor_name || '', notes: '' }
  payForm.value = newPayForm()
  payVendor.value = null
  uploadedFileUrl.value = ''
  uploadedFileName.value = ''
  showPayModal.value = true
  fetchPayVendor(row.vendor_id)
  fetchPayStruks(row.id)
}

function openBatchPay() {
  const selected = requests.value.filter(r => selectedIds.value.has(r.id))
  if (selected.length === 0) return
  if (selected.some(r => !isPayable(r))) {
    toast.error('Semua tagihan yang akan dibayar harus sudah disetujui Manager Keuangan / GM.')
    return
  }
  if (selected.length === 1) {
    openPayModal(selected[0])
    return
  }
  // Final vendor consistency check
  const vendors = [...new Set(selected.map(r => r.vendor_id || ''))]
  if (vendors.length > 1 || vendors.includes('')) {
    toast.error('Pembayaran sekaligus hanya untuk 1 vendor terdaftar yang sama. Untuk vendor berbeda, bayar per baris.')
    return
  }
  if (selected.some(r => r.pay_to?.effective !== 'vendor_account')) {
    toast.error('VA marketplace / tunai dibayar satu per satu — tujuannya berbeda per pengajuan.')
    return
  }
  payTargets.value = selected
  paySource.value = 'office'
  ++payStruksSeq
  payStruks.value = []
  payStrukId.value = ''
  payForm.value = newPayForm()
  payVendor.value = null
  uploadedFileUrl.value = ''
  uploadedFileName.value = ''
  showPayModal.value = true
  fetchPayVendor(selected[0]?.vendor_id)
}

// Rekening tujuan selalu dari data vendor: server memotretnya saat pembayaran
// dicatat (ketikan diabaikan bila vendor punya rekening), jadi riwayat
// pembayaran lama tidak berubah walau vendor kemudian mengganti rekeningnya.
function bankText(bank, number, holder) {
  return [bank, number, holder ? `a.n. ${holder}` : ''].filter(Boolean).join(' ')
}

// Rekening vendor yang baru diganti (30 hari terakhir) — modus penipuan
// pembayaran yang umum: "vendor ganti rekening" tepat sebelum transfer.
const recentBankChange = computed(() => {
  const c = payVendor.value?.bank_changes?.[0]
  if (!c?.changed_at) return null
  const at = new Date(c.changed_at).getTime()
  return Number.isFinite(at) && Date.now() - at <= 30 * 24 * 3600 * 1000 ? c : null
})

// Bernomor urut: respons vendor form yang sudah ditutup (Batal lalu buka
// dokumen lain) dulu bisa datang belakangan dan menimpa form yang terbuka —
// rekening vendor lain tampil sebagai tujuan dan, bila vendor dokumen ini tak
// berekening, ikut tercatat sebagai tujuan "manual" pembayarannya.
async function fetchPayVendor(vendorId) {
  const seq = ++payVendorSeq
  payVendorError.value = ''
  if (!vendorId) { payVendor.value = null; payVendorLoading.value = false; return }
  payVendorLoading.value = true
  try {
    const v = await vendorsApi.get(vendorId)
    if (seq !== payVendorSeq) return
    payVendor.value = v
  } catch (err) {
    if (seq !== payVendorSeq) return
    payVendor.value = null
    payVendorError.value = err?.message || 'koneksi gagal'
  }
  finally { if (seq === payVendorSeq) payVendorLoading.value = false }
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
    // apiClient membuka amplop {success,data} → { url, filename }. Alamat yang
    // tidak sah (bukan /uploads/…) tidak dipakai sebagai bukti.
    const url = safeUrl(res?.url ?? res?.data?.url)
    if (!url) throw new Error('Server tidak mengembalikan alamat berkas yang sah.')
    uploadedFileUrl.value = url
    uploadedFileName.value = res?.filename ?? res?.data?.filename ?? ''
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

async function submitOutletCash() {
  const target = payTargets.value[0]
  if (payTargets.value.length !== 1 || !target?.outlet_id) { toast.error('Kas Kasir Outlet hanya untuk satu pengajuan milik outlet.'); return }
  const amount = payoutForm.value.amount || 0
  if (amount > payRemainingAmount.value) {
    toast.error(`Nominal (${formatRupiah(amount)}) melebihi sisa tagihan (${formatRupiah(payRemainingAmount.value)}).`)
    return
  }
  if (!payoutForm.value.recipient_name.trim()) { toast.error('Nama penerima kas wajib diisi.'); return }
  payLoading.value = true
  try {
    await purchaseApi.assignCashPayout({
      purchase_request_id: target.id,
      amount,
      recipient_name: payoutForm.value.recipient_name.trim(),
      notes: payoutForm.value.notes.trim(),
    })
    toast.success(`Tugas dikirim ke tablet kasir ${payOutletName.value || 'outlet'}.`)
    showPayModal.value = false
    clearSelection()
    fetchList()
    fetchStats()
  } catch (err) { toast.error(err?.message || 'Gagal menetapkan sumber dana.') }
  finally { payLoading.value = false }
}

async function submitPay() {
  if (paySource.value === 'outlet_cash') return submitOutletCash()
  // Tujuan pembayaran milik Pembelian (pay_to): server membayar ke tujuan itu
  // dan menolak 409 bila Pembelian mengubahnya sejak daftar ini dimuat.
  if (!payTargetsReady.value) {
    toast.error('Tujuan pembayaran belum siap — tunggu Pembelian melengkapi atau memperbaikinya.')
    return
  }
  if (!payForm.value.payment_account_source) { toast.error('Rekening asal wajib diisi.'); return }
  const paymentAmount = payForm.value.payment_amount || 0
  if (payTargets.value.length === 1 && paymentAmount > 0 && paymentAmount > payRemainingAmount.value) {
    toast.error(`Jumlah bayar (${formatRupiah(paymentAmount)}) melebihi sisa tagihan (${formatRupiah(payRemainingAmount.value)}).`)
    return
  }
  if (payTargets.value.length === 1 && selectedPayStruk.value && paymentAmount > selectedPayStruk.value.remaining) {
    toast.error(`Jumlah bayar (${formatRupiah(paymentAmount)}) melebihi sisa struk ${selectedPayStruk.value.struk_number} (${formatRupiah(selectedPayStruk.value.remaining)}).`)
    return
  }
  const selectedAccount = bankAccountOptions.value.find(a => a.id === payForm.value.payment_account_source)
  const accountSourceText = selectedAccount?.name || payForm.value.payment_account_source
  payLoading.value = true
  let successCount = 0
  let failCount = 0
  let conflict = false
  let failureMessage = ''
  try {
    const bankFeeAmount = isDifferentBank.value ? (Number(payForm.value.bank_fee) || 0) : 0
    for (let idx = 0; idx < payTargets.value.length; idx++) {
      const target = payTargets.value[idx]
      try {
        const payload = {
          action: 'pay',
          actor_name: adminName(),
          payment_proof: [payForm.value.payment_proof.trim(), uploadedFileUrl.value].filter(Boolean).join(' | '),
          payment_account_source: accountSourceText,
          version: target.version,
          payment_notes: payForm.value.payment_notes.trim(),
          payment_amount: payTargets.value.length === 1 ? paymentAmount : 0,
          struk_id: payTargets.value.length === 1 && payStrukId.value ? payStrukId.value : undefined,
        }
        // Biaya admin bank hanya dikirim sekali pada pengajuan pertama dalam 1 transfer
        if (idx === 0 && bankFeeAmount > 0) {
          payload.bank_fee = bankFeeAmount
        }
        const paidRes = await purchaseApi.updateStatus(target.id, payload)
        // Pengajuan Jasa yang sudah lunas otomatis ditandai Selesai (received)
        // agar tidak menggantung di status Dibayar menunggu pengaju.
        if (normalizePurchaseType(target.request_type) === 'jasa' && paidRes?.status === 'paid') {
          try {
            await purchaseApi.updateStatus(target.id, { action: 'receive', actor_name: adminName() })
          } catch { /* abaikan bila gagal; pembayaran sudah tercatat */ }
        }
        successCount++
      } catch (err) {
        failCount++
        if (isConflictError(err)) conflict = true
        failureMessage = err?.message || 'Gagal mencatat pembayaran.'
      }
    }
    if (conflict) {
      // Detail pengajuan berubah sejak dibuka (mis. Pembelian mengubah tujuan
      // pembayaran): tutup form, muat data terbaru, periksa lagi.
      toast.error(`${successCount ? `${successCount} berhasil; ` : ''}pengajuan berubah sejak form dibuka — data terbaru sudah dimuat, periksa tujuan pembayarannya lagi.`)
      showPayModal.value = false
      clearSelection()
      fetchList()
      fetchStats()
      return
    }
    if (failCount === 0) {
      toast.success(payTargets.value.length > 1 ? `${successCount} pembayaran berhasil dicatat.` : 'Pembayaran berhasil dicatat.')
    } else {
      toast.error(payTargets.value.length === 1 ? failureMessage : `${successCount} berhasil, ${failCount} gagal.`)
      if (successCount === 0) return
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
.source-opt {
  @apply flex cursor-pointer flex-col gap-0.5 rounded-lg border border-gray-200 bg-white px-3 py-2.5 transition-colors hover:border-emerald-300;
}
.source-opt--on { @apply border-emerald-500 bg-emerald-50 ring-1 ring-emerald-500; }
.source-opt--off { @apply cursor-not-allowed opacity-60 hover:border-gray-200; }
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
.type-bahan  { background: rgba(245,158,11,.12); color: #b45309; }
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
