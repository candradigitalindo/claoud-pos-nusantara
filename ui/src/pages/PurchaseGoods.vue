<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Pengadaan Barang</h1>
      <AppButton v-if="authStore.hasPermission('procurement.requests.submit')" @click="openCreate">+ Buat Pengajuan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Filters -->
    <AppCard>
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-4">
        <div class="flex w-full flex-col gap-1 sm:w-auto sm:min-w-[200px]">
          <label class="text-sm font-medium text-gray-700">Unit Kerja</label>
          <SearchSelect
            v-model="filterWorkUnit"
            :options="workUnitFilterOptions"
            placeholder="Semua Unit Kerja"
            searchPlaceholder="Cari unit kerja…"
          />
        </div>
        <div class="flex w-full flex-col gap-1 sm:w-auto">
          <label class="text-sm font-medium text-gray-700">Status</label>
          <select v-model="filterStatus"
            class="w-full sm:w-auto rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="">Semua Status</option>
            <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>
        <button @click="page = 1; fetchList()"
          class="w-full sm:w-auto px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
      </div>
    </AppCard>

    <!-- List -->
    <AppCard :padding="false">
      <!-- Mobile: daftar kartu (tabel disembunyikan di bawah sm) -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!requests.length" class="p-6 text-center text-sm text-gray-400">Belum ada pengajuan barang.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in requests" :key="row.id" class="p-4">
            <div class="flex items-start justify-between gap-2">
              <button @click="viewDetail(row)" class="min-w-0 flex-1 text-left">
                <p class="font-mono text-[11px] text-gray-500">{{ row.request_number || '-' }}</p>
                <p class="mt-0.5 text-sm font-medium text-gray-900 break-words">{{ (row.items || []).map(i => i.name).join(', ') || '-' }}</p>
              </button>
              <span :class="statusBadge(row.status)" class="shrink-0">{{ statusLabel(row.status) }}</span>
            </div>
            <dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Unit</dt>
              <dd class="font-medium text-gray-800">{{ row.work_unit_name || '-' }}</dd>
              <dt class="text-gray-400">Pengaju</dt>
              <dd class="text-gray-700">{{ row.requested_by || '-' }}</dd>
              <dt class="text-gray-400">Tanggal</dt>
              <dd class="text-gray-600">{{ formatDateTime(row.created_at) }}</dd>
            </dl>
            <div class="mt-2 flex items-end justify-between gap-2 border-t border-gray-100 pt-2">
              <div class="min-w-0">
                <p class="text-sm font-semibold text-gray-900">{{ formatRupiah(row.total_amount) }}</p>
                <p v-if="row.status === 'partial' && row.paid_amount > 0" class="text-[10px] text-amber-600">
                  Sisa: {{ formatRupiah((row.total_final || row.total_amount || 0) - (row.paid_amount || 0)) }}
                </p>
              </div>
              <div class="action-btns shrink-0">
                <button class="act-view" @click="viewDetail(row)" title="Lihat">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                </button>
                <button v-if="row.status === 'pending' && authStore.hasPermission('procurement.requests.submit')" class="act-edit" @click="viewDetail(row)" title="Edit">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                </button>
                <button v-if="canDelete(row.status) && authStore.hasPermission('procurement.requests.submit')" class="act-del" @click="confirmDelete(row)" title="Hapus">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
                </button>
              </div>
            </div>
          </li>
        </ul>
      </div>
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="requests" :loading="loading" emptyText="Belum ada pengajuan barang.">
        <template #cell-request_number="{ row }">
          <span class="font-mono text-xs text-gray-700">{{ row.request_number || '-' }}</span>
        </template>
        <template #cell-pengadaan_names="{ row }">
          <span class="text-gray-800">{{ (row.items || []).map(i => i.name).join(', ') || '-' }}</span>
        </template>
        <template #cell-work_unit_name="{ row }">
          <span class="font-medium text-gray-900">{{ row.work_unit_name || '-' }}</span>
        </template>
        <template #cell-total_amount="{ row }">
          {{ formatRupiah(row.total_amount) }}
          <div v-if="row.status === 'partial' && row.paid_amount > 0" class="text-[10px] font-normal text-amber-600 mt-0.5">
            Sisa: {{ formatRupiah((row.total_final || row.total_amount || 0) - (row.paid_amount || 0)) }}
          </div>
        </template>
        <template #cell-status="{ row }">
          <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
        </template>
        <template #cell-created_at="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
        <template #cell-actions="{ row }">
          <div class="action-btns">
            <button class="act-view" @click="viewDetail(row)" title="Lihat">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            </button>
            <button v-if="row.status === 'pending' && authStore.hasPermission('procurement.requests.submit')" class="act-edit" @click="viewDetail(row)" title="Edit">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
            </button>
            <button v-if="canDelete(row.status) && authStore.hasPermission('procurement.requests.submit')" class="act-del" @click="confirmDelete(row)" title="Hapus">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
            </button>
          </div>
        </template>
      </AppTable>
      <div v-if="totalPages > 1" class="px-4 py-3 border-t border-gray-100">
        <AppPagination :page="page" :totalPages="totalPages" @update:page="goPage" />
      </div>
    </AppCard>

    <!-- Create Modal -->
    <AppModal v-model="showCreate" title="Pengajuan Pembelian Barang" size="2xl">
      <form class="space-y-4" @submit.prevent="submitCreate">
        <AppAlert type="error" :message="createError" />
        <div v-if="myWorkUnit" class="bg-emerald-50 border border-emerald-200 rounded-lg p-3 text-sm">
          <p class="text-emerald-800"><strong>Unit Kerja:</strong> {{ myWorkUnit.name }} — <strong>Pengaju:</strong> {{ form.requested_by }}</p>
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Projek <span class="font-normal text-gray-400">(opsional)</span></label>
          <SearchSelect v-model="form.project_id" :options="projectOptions"
            placeholder="Tanpa projek" searchPlaceholder="Cari projek…" />
          <p class="text-xs text-gray-400">Isi bila belanja ini bagian dari projek pembangunan/renovasi — nilainya ikut terhitung ke RAB projek.</p>
        </div>

        <!-- Items List -->
        <div>
          <label class="text-sm font-medium text-gray-700 mb-2 block">Daftar Pengadaan</label>
          <div class="space-y-3">
            <div v-for="(item, i) in form.items" :key="i" class="bg-gray-50 rounded-lg p-3 sm:p-4 border border-gray-200">
              <div class="flex items-start gap-2">
                <div class="flex-1 space-y-3">
                  <div class="flex items-center gap-2">
                    <span class="bg-emerald-100 text-emerald-700 px-2 py-0.5 rounded text-xs font-semibold">Pengadaan #{{ i + 1 }}</span>
                    <button type="button" @click="removeItem(i)" class="text-red-400 hover:text-red-600 ml-auto" :disabled="form.items.length <= 1">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
                    </button>
                  </div>
                  <input v-model="item.name" placeholder="Nama Pengadaan" class="input-sm w-full" />
                  <!-- Sub-items table -->
                  <div class="pl-2 sm:ml-2 sm:pl-3 border-l-2 border-emerald-200 space-y-2">
                    <p class="text-xs font-semibold text-emerald-600 uppercase tracking-wide">Detail Item</p>
                    <div v-for="(sub, j) in item.items" :key="j"
                      class="grid grid-cols-2 gap-1.5 rounded-lg border border-gray-200 bg-white p-2 sm:grid-cols-12 sm:items-center sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0">
                      <input v-model="sub.name" placeholder="Nama item" class="input-sm col-span-2 sm:col-span-4" />
                      <input v-model.number="sub.qty" type="number" min="1" placeholder="Qty" class="input-sm sm:col-span-2" />
                      <input v-model="sub.unit" placeholder="Satuan" class="input-sm sm:col-span-1" />
                      <RupiahInput v-model="sub.hps_price" placeholder="HPS satuan" class="col-span-2" />
                      <p class="col-span-1 self-center text-right text-xs text-gray-500 sm:col-span-2">{{ formatRupiah((sub.qty || 0) * (sub.hps_price || 0)) }}</p>
                      <button type="button" @click="removeSubItem(i, j)" class="col-span-1 justify-self-end self-center text-red-400 hover:text-red-600 sm:col-span-1 sm:justify-self-center" :disabled="item.items.length <= 1">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
                      </button>
                    </div>
                    <button type="button" @click="addSubItem(i)" class="text-xs text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Item</button>
                  </div>
                  <p class="text-xs text-gray-500 text-right">HPS Pengadaan: <strong>{{ formatRupiah(calcItemHps(item)) }}</strong></p>
                </div>
              </div>
            </div>
          </div>
          <div class="mt-3">
            <button type="button" @click="addItem" class="text-sm text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Pengadaan</button>
          </div>
          <p v-if="form.items.length" class="mt-2 text-sm font-semibold text-gray-700">
            Total HPS: {{ formatRupiah(estimatedTotal) }}
          </p>
        </div>

        <AppInput v-model="form.notes" label="Catatan / Alasan Pembelian" placeholder="Keterangan tambahan (opsional)" />
      </form>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton :loading="creating" @click="submitCreate">Kirim Pengajuan</AppButton>
      </template>
    </AppModal>

    <!-- Detail Modal -->
    <AppModal v-model="showDetail" title="Detail Pengajuan Barang" size="2xl">
      <div v-if="detail" class="space-y-4">
        <button v-if="parentDetailId" @click="backToParent" class="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800 font-medium mb-1">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><polyline points="12 19 5 12 12 5"/></svg>
          Kembali ke Pengajuan Induk
        </button>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span :class="statusBadge(detail.status)" class="text-sm">{{ statusLabel(detail.status) }}</span>
          <span v-if="detail.request_number" class="font-mono text-sm text-gray-600">No. {{ detail.request_number }}</span>
          <span class="text-gray-400">•</span>
          <span class="text-gray-500">{{ detail.work_unit_name || '-' }}</span>
          <span class="text-gray-400">•</span>
          <span class="text-gray-500">{{ formatDateTime(detail.created_at) }}</span>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <!-- Permohonan -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-emerald-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-emerald-600 mb-1.5">Permohonan</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Pengaju</dt>
                <dd class="font-medium text-gray-900">{{ detail.requested_by }}</dd>
                <template v-if="detail.work_unit_name">
                  <dt class="text-gray-400">Unit</dt>
                  <dd class="font-medium text-gray-900">{{ detail.work_unit_name }}</dd>
                </template>
                <template v-if="detail.project_name">
                  <dt class="text-gray-400">Projek</dt>
                  <dd class="font-medium text-gray-900">{{ detail.project_name }}</dd>
                </template>
                <template v-if="detail.vendor_name && !detail.children?.length">
                  <dt class="text-gray-400">Supplier</dt>
                  <dd class="font-medium text-gray-900">{{ detail.vendor_name }}</dd>
                </template>
                <template v-if="detail.children?.length">
                  <dt class="text-gray-400">Vendor</dt>
                  <dd class="font-medium text-gray-900">{{ detail.children.length }} vendor <span class="text-gray-400 font-normal">(lihat pecahan)</span></dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Nilai -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-blue-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-blue-600 mb-1.5">Nilai</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="showHps">
                  <dt class="text-gray-400">HPS</dt>
                  <dd class="font-medium text-gray-900">{{ formatRupiah(detail.status === 'pending' ? detailHpsTotal : detail.total_hps) }}</dd>
                </template>
                <dt class="text-gray-400">{{ detail.children?.length ? 'Total' : 'Harga' }}</dt>
                <dd class="font-semibold text-emerald-700">{{ detail.total_final ? formatRupiah(detail.total_final) : 'Belum ditentukan' }}</dd>
              </dl>
            </div>
          </div>

          <!-- Persetujuan -->
          <div v-if="detail.approved_by || detail.rejected_reason" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-amber-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-amber-600 mb-1.5">{{ detail.status === 'rejected' ? 'Penolakan' : 'Persetujuan' }}</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="detail.approved_by">
                  <dt class="text-gray-400">Oleh</dt>
                  <dd class="font-medium text-gray-900">{{ detail.approved_by }}</dd>
                </template>
                <template v-if="detail.approved_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.approved_at) }}</dd>
                </template>
                <template v-if="detail.rejected_reason">
                  <dt class="text-gray-400">Alasan</dt>
                  <dd class="text-red-600">{{ detail.rejected_reason }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Pembayaran -->
          <div v-if="detail.paid_by || detail.payment_account_dest" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-green-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-green-600 mb-1.5">Pembayaran</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="detail.paid_by">
                  <dt class="text-gray-400">Oleh</dt>
                  <dd class="font-medium text-gray-900">{{ detail.paid_by }}</dd>
                </template>
                <template v-if="detail.payment_account_dest">
                  <dt class="text-gray-400">Rekening</dt>
                  <dd class="font-medium font-mono text-gray-900">{{ detail.payment_account_dest }}</dd>
                </template>
                <template v-if="detail.paid_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.paid_at) }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Penerimaan -->
          <div v-if="detail.received_by" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-purple-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-purple-600 mb-1.5">Penerimaan</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Oleh</dt>
                <dd class="font-medium text-gray-900">{{ detail.received_by }}</dd>
                <template v-if="detail.received_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.received_at) }}</dd>
                </template>
              </dl>
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
        </div>

        <div v-if="detail.children?.length" class="bg-emerald-50 rounded-xl p-4 border border-emerald-200">
          <p class="text-sm font-bold text-emerald-900 mb-3 flex items-center gap-2">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M12 2v8m0 0l-4-4m4 4l4-4M12 22v-8m0 0l-4 4m4-4l4 4"/></svg>
            Pecahan Vendor ({{ detail.children.length }})
          </p>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div v-for="child in detail.children" :key="child.id" class="bg-white border border-emerald-100 rounded-lg p-3 shadow-sm flex flex-col gap-2">
              <div class="flex justify-between items-start">
                <div>
                  <p class="text-xs font-bold text-gray-900 line-clamp-1">{{ child.vendor_name || 'Vendor Lain' }}</p>
                  <p v-if="child.request_number" class="text-[10px] text-gray-500 font-mono">No. {{ child.request_number }}</p>
                </div>
                <span :class="statusBadge(child.status)" class="text-[10px]">{{ statusLabel(child.status) }}</span>
              </div>
              <div class="flex justify-between items-end mt-auto">
                <p class="text-xs font-bold text-emerald-700">{{ formatRupiah(child.total_final || child.total_amount) }}</p>
                <button @click="viewChildDetail(child)" class="text-[10px] text-blue-600 hover:underline font-medium">Lihat Detail →</button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="!detail.children?.length || detailItems.length > 0">
          <p class="text-sm font-semibold text-gray-700 mb-2">
            {{ detail.children?.length ? 'Item Belum Dipecah' : 'Daftar Pengadaan' }} ({{ detailItems.length }} pengadaan)
          </p>
          <div class="space-y-4">
            <div v-for="(item, i) in detailItems" :key="i" class="bg-gray-50 rounded-lg p-3 border border-gray-200">
              <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 mb-2">
                <p class="min-w-0 break-words text-sm font-semibold text-gray-900">{{ i + 1 }}. {{ item.name }}</p>
                <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
                  <span v-if="showHps" class="text-gray-500">HPS: <strong>{{ formatRupiah(detail.status === 'pending' ? calcDetailItemHps(item) : item.hps_total) }}</strong></span>
                  <span class="text-emerald-700">Harga: <strong>{{ item.final_total != null && item.final_total > 0 ? formatRupiah(item.final_total) : '-' }}</strong></span>
                </div>
              </div>
              <!-- Mobile: satu kartu per item -->
              <div class="space-y-2 sm:hidden">
                <div v-for="(sub, j) in item.items" :key="j" class="rounded-lg border border-gray-200 bg-white p-2.5">
                  <div class="flex items-start justify-between gap-2">
                    <p class="min-w-0 break-words text-xs font-medium text-gray-800">{{ j + 1 }}. {{ sub.name }}</p>
                    <button v-if="detail.status === 'pending'" class="act-del shrink-0" @click="removeDetailSubItem(i, j)" title="Hapus item">
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
                    </button>
                  </div>
                  <dl class="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[11px]">
                    <dt class="text-gray-400">Qty</dt>
                    <dd class="text-gray-800">
                      <span v-if="detail.status === 'pending'" class="inline-flex items-center gap-1">
                        <input v-model.number="sub.qty" type="number" min="1" class="w-20 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span>{{ sub.unit }}</span>
                      </span>
                      <span v-else>{{ sub.qty }} {{ sub.unit }}</span>
                    </dd>
                    <template v-if="showHps">
                      <dt class="text-gray-400">HPS</dt>
                      <dd class="text-gray-700">
                        {{ formatRupiah(sub.hps_price) }}<span class="text-gray-400"> × {{ sub.qty }} = </span>
                        <strong>{{ formatRupiah(detail.status === 'pending' ? (sub.qty || 0) * (sub.hps_price || 0) : sub.hps_subtotal) }}</strong>
                      </dd>
                    </template>
                    <dt class="text-gray-400">Harga</dt>
                    <dd class="text-gray-700">
                      <template v-if="sub.final_price">
                        {{ formatRupiah(sub.final_price) }}<span class="text-gray-400"> × {{ sub.qty }} = </span>
                        <strong class="text-emerald-700">{{ formatRupiah((sub.qty || 0) * sub.final_price) }}</strong>
                      </template>
                      <template v-else>-</template>
                    </dd>
                  </dl>
                </div>
              </div>
              <div class="hidden sm:block overflow-x-auto">
                <table class="w-full text-xs">
                  <thead>
                    <tr class="border-b border-gray-200 text-left text-gray-500 uppercase">
                      <th v-if="detail.status === 'pending'" class="py-1.5 w-8"></th>
                      <th class="py-1.5 pr-2">#</th>
                      <th class="py-1.5 pr-2">Nama Item</th>
                      <th class="py-1.5 pr-2 text-right">Qty</th>
                      <th class="py-1.5 pr-2">Satuan</th>
                      <th v-if="showHps" class="py-1.5 pr-2 text-right">HPS Satuan</th>
                      <th v-if="showHps" class="py-1.5 pr-2 text-right">HPS Total</th>
                      <th class="py-1.5 pr-2 text-right">Harga Satuan</th>
                      <th class="py-1.5 text-right">Harga Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(sub, j) in item.items" :key="j" class="border-b border-gray-100">
                      <td v-if="detail.status === 'pending'" class="py-1.5 text-center">
                        <button class="act-del" @click="removeDetailSubItem(i, j)" title="Hapus item">
                          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
                        </button>
                      </td>
                      <td class="py-1.5 pr-2 text-gray-400">{{ j + 1 }}</td>
                      <td class="py-1.5 pr-2 text-gray-800">{{ sub.name }}</td>
                      <td class="py-1.5 pr-2 text-right">
                        <input v-if="detail.status === 'pending'" v-model.number="sub.qty" type="number" min="1" class="w-24 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span v-else>{{ sub.qty }}</span>
                      </td>
                      <td class="py-1.5 pr-2">{{ sub.unit }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(sub.hps_price) }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(detail.status === 'pending' ? (sub.qty || 0) * (sub.hps_price || 0) : sub.hps_subtotal) }}</td>
                      <td class="py-1.5 pr-2 text-right">{{ sub.final_price ? formatRupiah(sub.final_price) : '-' }}</td>
                      <td class="py-1.5 text-right font-medium">{{ sub.final_price ? formatRupiah((sub.qty || 0) * sub.final_price) : '-' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- Totals -->
          <div class="flex flex-wrap justify-end gap-x-6 gap-y-1 mt-3 text-sm">
            <span v-if="showHps" class="text-gray-700">Total HPS: <strong>{{ formatRupiah(detail.status === 'pending' ? detailHpsTotal : detail.total_hps) }}</strong></span>
            <span class="text-emerald-700">Total Harga: <strong>{{ detail.total_final ? formatRupiah(detail.total_final) : '-' }}</strong></span>
          </div>

          <!-- Purchasing Workflow Selection (Only for Main/Master requests) -->
          <div v-if="detail.status === 'approved' && authStore.hasPermission('procurement.requests.purchasing')" class="mt-6">
            <div v-if="!detail.parent_id && (!detail.children?.length || detailItems.length > 0)" class="bg-emerald-50 border border-emerald-200 rounded-2xl p-4 sm:p-5 shadow-sm">
              <p class="text-sm font-bold text-emerald-900 mb-4 flex items-center gap-2 uppercase tracking-tight">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M16 11V7a4 4 0 00-8 0v4M5 9h12a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V11a2 2 0 012-2z"/></svg>
                Proses Pembelian (Purchasing)
              </p>
              <div :class="detail.children?.length ? '' : 'grid grid-cols-1 sm:grid-cols-2 gap-4'">
                <!-- Option 1: Single Vendor (only if no split yet) -->
                <button v-if="!detail.children?.length" @click="openEditFinal" class="flex flex-col items-center gap-3 p-4 sm:p-5 bg-white border-2 border-emerald-100 hover:border-emerald-500 rounded-2xl transition-all group text-left shadow-sm">
                  <div class="w-12 h-12 rounded-xl bg-emerald-100 flex items-center justify-center text-emerald-600 group-hover:scale-110 transition-transform">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"/></svg>
                  </div>
                  <div class="text-center">
                    <p class="text-sm font-bold text-gray-900">Beli Semua di 1 Vendor</p>
                    <p class="text-[11px] text-gray-500 leading-tight mt-1">Gunakan satu supplier untuk seluruh daftar barang di atas.</p>
                  </div>
                </button>

                <!-- Option 2: Split Vendor -->
                <button @click="openSplitVendor" class="flex flex-col items-center gap-3 p-4 sm:p-5 bg-white border-2 border-blue-50 hover:border-blue-500 rounded-2xl transition-all group text-left shadow-sm">
                  <div class="w-12 h-12 rounded-xl bg-blue-100 flex items-center justify-center text-blue-600 group-hover:scale-110 transition-transform">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>
                  </div>
                  <div class="text-center">
                    <p class="text-sm font-bold text-gray-900">Pecah ke Vendor Berbeda</p>
                    <p class="text-[11px] text-gray-500 leading-tight mt-1">Pisahkan beberapa barang ke supplier lain (Split Invoice).</p>
                  </div>
                </button>
              </div>
            </div>
            
            <!-- For Child requests (already split) -->
            <div v-else class="bg-blue-50 border border-blue-200 rounded-xl p-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between shadow-sm">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-600">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
                </div>
                <div>
                  <p class="text-sm font-bold text-blue-900">Data Hasil Split</p>
                  <p class="text-xs text-blue-700">Silakan lengkapi harga final untuk vendor ini.</p>
                </div>
              </div>
              <button @click="openEditFinal" class="w-full sm:w-auto shrink-0 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-bold hover:bg-blue-700 transition-colors shadow-sm">
                Isi Harga Sekarang
              </button>
            </div>
          </div>

          <!-- Action bar: inline under items when pending (approval permission required) -->
          <div v-if="detail.status === 'pending' && authStore.hasPermission('procurement.requests.approve')" class="mt-4 border-t border-gray-200 pt-4">
            <!-- Reject reason input -->
            <div v-if="detailRejectMode" class="flex flex-col gap-2 sm:flex-row sm:items-end">
              <div class="min-w-0 flex-1">
                <label class="text-xs font-medium text-red-600 mb-1 block">Alasan Penolakan</label>
                <input v-model="detailRejectReason" placeholder="Masukkan alasan penolakan..." class="input-sm w-full border-red-300 focus:ring-red-500 focus:border-red-500" />
              </div>
              <div class="flex items-center justify-end gap-2">
                <button @click="detailRejectMode = false" class="px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700 rounded-lg hover:bg-gray-100">Batal</button>
                <button @click="submitDetailReject" :disabled="actionLoading" class="px-3 py-1.5 text-sm text-white bg-red-600 hover:bg-red-700 rounded-lg font-medium disabled:opacity-50">Konfirmasi Tolak</button>
              </div>
            </div>
            <!-- Action buttons -->
            <div v-else class="flex flex-wrap items-center justify-end gap-2">
              <button @click="detailRejectMode = true" class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-red-600 bg-red-50 hover:bg-red-100 rounded-lg transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                Tolak
              </button>
              <button @click="submitDetailApprove" :disabled="actionLoading" class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                Setujui
              </button>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <div v-if="!parentDetailId" class="flex flex-wrap items-center gap-2 mr-auto">
          <AppButton v-if="detail && detail.status === 'approved' && authStore.hasPermission('procurement.requests.purchasing') && detail.total_final > 0" variant="primary" @click="confirmDetailAction('request_payment')">Ajukan Pembayaran</AppButton>
          <AppButton v-if="detail && detail.status === 'paid' && authStore.hasPermission('procurement.requests.submit')" variant="primary" @click="confirmDetailAction('receive')">Serah Terima</AppButton>
          <AppButton v-if="detail && ['pending','approved'].includes(detail.status) && authStore.hasPermission('procurement.requests.submit')" variant="danger" @click="confirmDetailAction('cancel')">Batalkan</AppButton>
        </div>
        <AppButton variant="secondary" @click="parentDetailId ? backToParent() : (showDetail = false)">{{ parentDetailId ? '← Kembali' : 'Tutup' }}</AppButton>
      </template>
    </AppModal>

    <!-- Edit Final Price Modal -->
    <AppModal v-model="showEditFinal" title="Isi Harga (Purchasing)" size="2xl">
      <AppAlert type="error" :message="editFinalError" />
      <div class="mb-4 bg-emerald-50 p-3 sm:p-4 rounded-xl border border-emerald-100">
        <label class="text-sm font-bold text-emerald-900 mb-1.5 block">Vendor / Supplier Utama</label>
        <select v-model="editFinalVendorId"
          class="w-full rounded-lg border border-emerald-200 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
          <option value="">— Pilih Vendor —</option>
          <option v-for="v in vendorList" :key="v.id" :value="v.id">{{ v.name }}</option>
        </select>
      </div>
      <div class="mb-4">
        <label class="text-sm font-medium text-gray-700 mb-1 block">No. Invoice Vendor</label>
        <input v-model="editFinalInvoice" type="text" placeholder="Masukkan nomor invoice dari vendor"
          class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500" />
      </div>
      <p class="text-sm text-gray-500 mb-3">Masukkan harga per satuan untuk setiap item.</p>
      <div class="space-y-4">
        <div v-for="(item, i) in editFinalItems" :key="i" class="bg-gray-50 rounded-lg p-3 border border-gray-200">
          <p class="text-sm font-semibold text-gray-900 mb-2">{{ item.name }}</p>
          <div class="space-y-2 sm:space-y-1.5">
            <div v-for="(sub, j) in item.items" :key="j"
              class="grid grid-cols-2 gap-2 rounded-lg border border-gray-200 bg-white p-2 sm:grid-cols-12 sm:items-center sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0">
              <p class="col-span-2 break-words text-sm text-gray-700 sm:col-span-4">{{ sub.name }}</p>
              <p class="self-center text-xs text-gray-500 sm:col-span-2 sm:text-right">{{ sub.qty }} {{ sub.unit }}</p>
              <div class="hidden sm:block sm:col-span-1"></div>
              <div class="col-span-2 order-last sm:order-none sm:col-span-3">
                <RupiahInput v-model="sub.final_price" placeholder="Harga/satuan" />
              </div>
              <p class="self-center text-xs text-emerald-700 text-right font-medium sm:col-span-2">{{ formatRupiah((sub.qty || 0) * (sub.final_price || 0)) }}</p>
            </div>
          </div>
          <p class="text-xs text-right mt-2 text-gray-500">Harga pengadaan: <strong class="text-emerald-700">{{ formatRupiah(calcEditItemFinal(item)) }}</strong></p>
        </div>
      </div>
      <p class="mt-3 text-sm font-semibold text-gray-700">
        Total Harga: <span class="text-emerald-700">{{ formatRupiah(editFinalTotal) }}</span>
      </p>
      <template #footer>
        <AppButton variant="secondary" @click="showEditFinal = false">Batal</AppButton>
        <AppButton :loading="savingFinal" @click="submitEditFinal">Simpan Harga</AppButton>
      </template>
    </AppModal>

    <!-- Split Vendor Modal -->
    <AppModal v-model="showSplitModal" title="Split Barang ke Vendor Lain" size="2xl">
      <div class="space-y-4">
        <AppAlert type="error" :message="splitError" />
        <p class="text-sm text-gray-500">Pilih item barang yang ingin Anda pindahkan ke vendor/supplier berbeda.</p>
        
        <div class="space-y-2 max-h-[40vh] overflow-y-auto px-1">
          <div v-for="(sub, i) in splitItemsFlattened" :key="i" 
            class="flex items-center gap-3 p-3 rounded-lg border transition-all cursor-pointer"
            :class="sub.selected ? 'bg-emerald-50 border-emerald-300 ring-1 ring-emerald-300' : 'bg-gray-50 border-gray-200 hover:border-emerald-200'"
            @click="sub.selected = !sub.selected">
            <input type="checkbox" v-model="sub.selected" @click.stop class="w-4 h-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500" />
            <div class="flex-1 min-w-0">
              <div class="flex justify-between items-start gap-2">
                <div class="min-w-0">
                  <p class="text-sm font-bold text-gray-900 break-words">{{ sub.name }}</p>
                  <p class="text-[10px] text-gray-400 uppercase font-semibold break-words">{{ sub.groupName }}</p>
                </div>
                <div class="text-right shrink-0">
                  <p class="text-xs font-bold text-emerald-700">{{ formatRupiah((sub.qty || 0) * (sub.final_price || 0)) }}</p>
                  <p class="text-[10px] text-gray-500">{{ sub.qty }} {{ sub.unit }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-gray-50 p-3 sm:p-4 rounded-xl border border-gray-200 space-y-3">
          <div>
            <label class="text-sm font-bold text-gray-700 mb-1.5 block">Vendor Tujuan</label>
            <select v-model="splitVendorId"
              class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
              <option value="">— Pilih Vendor Baru —</option>
              <option v-for="v in vendorList" :key="v.id" :value="v.id">{{ v.name }}</option>
            </select>
          </div>
          <div class="flex flex-wrap justify-between items-center gap-x-3 gap-y-1 pt-2 border-t border-gray-200">
            <p class="text-sm text-gray-600 font-medium">Total Harga Dipindah:</p>
            <p class="text-lg font-bold text-emerald-700">{{ formatRupiah(splitTotal) }}</p>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showSplitModal = false">Batal</AppButton>
        <AppButton :loading="splitLoading" :disabled="!canSplit" @click="submitSplit">Eksekusi Split Vendor</AppButton>
      </template>
    </AppModal>

    <!-- Confirm Action Modal (Approve / Pay / Receive) -->
    <AppModal v-model="showActionConfirm" :title="actionConfirmTitle" size="sm">
      <div class="flex items-start gap-3">
        <div :class="actionConfirmIconBg" class="shrink-0 w-10 h-10 rounded-full flex items-center justify-center">
          <svg v-if="pendingAction === 'approve'" class="w-5 h-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          <svg v-else-if="pendingAction === 'request_payment'" class="w-5 h-5 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          <svg v-else class="w-5 h-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
        </div>
        <div>
          <p class="text-sm text-gray-600">{{ actionConfirmMessage }}</p>
          <p class="text-xs text-gray-400 mt-1">Pengaju: <strong>{{ actionTarget?.requested_by || '-' }}</strong></p>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showActionConfirm = false">Batal</AppButton>
        <AppButton :loading="actionLoading" @click="submitAction">{{ actionConfirmBtn }}</AppButton>
      </template>
    </AppModal>

    <!-- Reject Modal -->
    <AppModal v-model="showReject" title="Tolak Pengajuan" size="sm">
      <div class="space-y-3">
        <p class="text-sm text-gray-600">Berikan alasan penolakan pengajuan dari <strong>{{ rejectTarget?.work_unit_name || '-' }}</strong>.</p>
        <AppInput v-model="rejectReason" label="Alasan Penolakan" placeholder="Masukkan alasan..." />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showReject = false">Batal</AppButton>
        <AppButton variant="danger" :loading="actionLoading" @click="submitReject">Tolak</AppButton>
      </template>
    </AppModal>

    <!-- Delete Confirm -->
    <AppModal v-model="showDeleteConfirm" title="Hapus Pengajuan" size="sm">
      <p class="text-sm text-gray-600">Yakin ingin menghapus pengajuan dari <strong>{{ deleteTarget?.work_unit_name || '-' }}</strong>?</p>
      <template #footer>
        <AppButton variant="secondary" @click="showDeleteConfirm = false">Batal</AppButton>
        <AppButton variant="danger" :loading="actionLoading" @click="submitDelete">Hapus</AppButton>
      </template>
    </AppModal>

    <!-- Delete Detail Item Confirm -->
    <AppModal v-model="showDeleteItemConfirm" title="Hapus Item" size="sm">
      <div class="flex items-start gap-3">
        <div class="bg-red-100 shrink-0 w-10 h-10 rounded-full flex items-center justify-center">
          <svg class="w-5 h-5 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
        </div>
        <p class="text-sm text-gray-600">Hapus <strong>"{{ deleteItemTarget?.name }}"</strong> dari daftar pengadaan?</p>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showDeleteItemConfirm = false">Batal</AppButton>
        <AppButton variant="danger" @click="confirmRemoveDetailSubItem">Hapus</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { purchaseApi } from '@/api/purchase.js'
import { workUnitsApi } from '@/api/workUnits.js'
import { vendorsApi } from '@/api/vendors.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import RupiahInput   from '@/components/ui/RupiahInput.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import { projectsApi } from '@/api/projects.js'

const toast = useToastStore()
const authStore = useAuthStore()

const REQUEST_TYPE = 'barang'

// HPS visible to pengaju & approval; hidden from purchasing team
const showHps = computed(() => authStore.hasPermission('procurement.requests.submit') || authStore.hasPermission('procurement.requests.approve'))

const loading = ref(false)
const errorMsg = ref('')
const requests = ref([])
const workUnits = ref([])
const vendorList = ref([])
const page = ref(1)
const totalPages = ref(1)
const filterWorkUnit = ref('')
const filterStatus = ref('')
const myWorkUnit = ref(null)

const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const form = ref(emptyForm())

const showDetail = ref(false)
const detail = ref(null)
const detailItems = ref([])
const parentDetailId = ref(null)
const savingDetailQty = ref(false)
const detailRejectMode = ref(false)
const detailRejectReason = ref('')

// Item yang masih dipegang pengajuan ini. Sejak split benar-benar memindahkan
// item ke pecahan, `items` dari server sudah berisi sisa yang tepat — dulu di
// sini item pecahan disaring lagi berdasarkan nama, sehingga dua baris bernama
// sama dalam satu grup ikut hilang semuanya.
function getUnsplitItems(d) {
  return (d.items || []).map(it => ({ ...it, selected: false }))
}

const detailHpsTotal = computed(() =>
  detailItems.value.reduce((sum, it) => sum + calcDetailItemHps(it), 0)
)
function calcDetailItemHps(item) {
  return (item.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.hps_price || 0), 0)
}

const showReject = ref(false)
const rejectTarget = ref(null)
const rejectReason = ref('')

const showDeleteConfirm = ref(false)
const deleteTarget = ref(null)

const actionLoading = ref(false)

const showActionConfirm = ref(false)
const actionTarget = ref(null)
const pendingAction = ref('')

const ACTION_META = {
  approve:         { title: 'Setujui Pengajuan',     msg: 'Yakin ingin menyetujui pengajuan ini?',                     btn: 'Setujui',           bg: 'bg-blue-100',    done: 'disetujui' },
  request_payment: { title: 'Ajukan Pembayaran',     msg: 'Ajukan pembayaran ke tim keuangan untuk pengajuan ini?',   btn: 'Ajukan Pembayaran', bg: 'bg-orange-100',  done: 'diajukan untuk pembayaran' },
  pay:             { title: 'Bayar Pengajuan',        msg: 'Konfirmasi pembayaran untuk pengajuan ini?',               btn: 'Bayar',             bg: 'bg-emerald-100', done: 'dibayar' },
  receive:         { title: 'Terima Barang',          msg: 'Konfirmasi barang sudah diterima?',                        btn: 'Terima',            bg: 'bg-purple-100',  done: 'diterima' },
}
const actionConfirmTitle   = computed(() => ACTION_META[pendingAction.value]?.title ?? '')
const actionConfirmMessage = computed(() => ACTION_META[pendingAction.value]?.msg ?? '')
const actionConfirmBtn     = computed(() => ACTION_META[pendingAction.value]?.btn ?? 'OK')
const actionConfirmIconBg  = computed(() => ACTION_META[pendingAction.value]?.bg ?? 'bg-gray-100')

const showEditFinal = ref(false)
const editFinalVendorId = ref('')
const editFinalInvoice = ref('')
const editFinalItems = ref([])
const editFinalError = ref('')
const savingFinal = ref(false)
const editFinalTotal = computed(() =>
  editFinalItems.value.reduce((sum, it) => sum + (it.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.final_price || 0), 0), 0)
)

// Split Vendor State
const showSplitModal = ref(false)
const splitVendorId = ref('')
const splitItemsFlattened = ref([]) // Individual sub-items for selection
const splitLoading = ref(false)
const splitError = ref('')

function openSplitVendor() {
  const flattened = []
  detailItems.value.forEach(group => {
    group.items.forEach(sub => {
      flattened.push({
        groupName: group.name,
        ...sub,
        selected: false
      })
    })
  })
  splitItemsFlattened.value = flattened
  splitVendorId.value = ''
  splitError.value = ''
  showSplitModal.value = true
}

const canSplit = computed(() => splitItemsFlattened.value.some(it => it.selected) && splitVendorId.value)
const splitTotal = computed(() =>
  splitItemsFlattened.value.filter(it => it.selected).reduce((sum, it) => sum + (it.qty || 0) * (it.final_price || 0), 0)
)

async function submitSplit() {
  if (!canSplit.value) return
  splitLoading.value = true
  splitError.value = ''
  try {
    // Reconstruct group structure for API
    const selectedSubItems = splitItemsFlattened.value.filter(it => it.selected)
    const groupsMap = {}
    selectedSubItems.forEach(sub => {
      if (!groupsMap[sub.groupName]) groupsMap[sub.groupName] = { name: sub.groupName, items: [] }
      const { groupName, selected, ...rest } = sub
      groupsMap[sub.groupName].items.push(rest)
    })
    
    const itemsPayload = Object.values(groupsMap)
    const vendor = vendorList.value.find(v => v.id === splitVendorId.value)
    
    await purchaseApi.splitVendor(detail.value.id, {
      vendor_id: splitVendorId.value,
      vendor_name: vendor?.name || '',
      items: itemsPayload
    })
    
    toast.success('Beberapa barang berhasil di-split ke vendor baru.')
    showSplitModal.value = false
    // Refresh detail
    detail.value = await purchaseApi.get(detail.value.id)
    detailItems.value = getUnsplitItems(detail.value)
    fetchList()
  } catch (err) {
    splitError.value = err?.message ?? 'Gagal melakukan split vendor.'
  } finally {
    splitLoading.value = false
  }
}

const STATUS_OPTIONS = [
  { value: 'pending',           label: 'Menunggu' },
  { value: 'approved',          label: 'Disetujui' },
  { value: 'payment_requested', label: 'Menunggu Pembayaran' },
  { value: 'rejected',          label: 'Ditolak' },
  { value: 'partial',           label: 'Dibayar Sebagian' },
  { value: 'paid',              label: 'Dibayar' },
  { value: 'received',          label: 'Diterima' },
  { value: 'cancelled',         label: 'Dibatalkan' },
]

const COLUMNS = [
  { key: 'request_number', label: 'Nomor' },
  { key: 'pengadaan_names', label: 'Nama Pengadaan' },
  { key: 'work_unit_name', label: 'Unit Kerja' },
  { key: 'requested_by', label: 'Pengaju' },
  { key: 'total_amount', label: 'Total', align: 'right' },
  { key: 'status',       label: 'Status' },
  { key: 'created_at',   label: 'Tanggal' },
  { key: 'actions',      label: '', align: 'right' },
]

const statusMap = {
  pending:           { label: 'Menunggu',            cls: 'bg-amber-100 text-amber-700' },
  approved:          { label: 'Disetujui',           cls: 'bg-blue-100 text-blue-700' },
  payment_requested: { label: 'Menunggu Pembayaran', cls: 'bg-orange-100 text-orange-700' },
  rejected:          { label: 'Ditolak',             cls: 'bg-red-100 text-red-700' },
  partial:           { label: 'Dibayar Sebagian',    cls: 'bg-amber-100 text-amber-700' },
  paid:              { label: 'Dibayar',             cls: 'bg-emerald-100 text-emerald-700' },
  received:          { label: 'Diterima',            cls: 'bg-purple-100 text-purple-700' },
  cancelled:         { label: 'Dibatalkan',          cls: 'bg-gray-100 text-gray-600' },
}

const estimatedTotal = computed(() =>
  form.value.items.reduce((sum, it) => sum + (it.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.hps_price || 0), 0), 0)
)
function calcItemHps(item) {
  return (item.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.hps_price || 0), 0)
}
function calcEditItemFinal(item) {
  return (item.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.final_price || 0), 0)
}

function statusBadge(s) {
  const m = statusMap[s] || statusMap.pending
  return `inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${m.cls}`
}
function statusLabel(s) { return (statusMap[s] || statusMap.pending).label }
function canDelete(s) {
  if (authStore.currentAdmin?.role === 'admin') return s !== 'paid'
  return ['pending', 'rejected', 'cancelled'].includes(s)
}
function emptyForm() {
  return {
    outlet_id: '', work_unit_id: '', requested_by: '', vendor_id: '', vendor_name: '', project_id: '',
    items: [{ name: '', items: [{ name: '', qty: 1, unit: 'pcs', hps_price: 0 }] }],
    notes: '',
  }
}
function addItem() {
  form.value.items.push({ name: '', items: [{ name: '', qty: 1, unit: 'pcs', hps_price: 0 }] })
}
function removeItem(i) { if (form.value.items.length > 1) form.value.items.splice(i, 1) }
function addSubItem(i) { form.value.items[i].items.push({ name: '', qty: 1, unit: 'pcs', hps_price: 0 }) }
function removeSubItem(i, j) { if (form.value.items[i].items.length > 1) form.value.items[i].items.splice(j, 1) }
function adminName() { return authStore.admin?.name || 'Admin' }

const route = useRoute()
const projects = ref([])

// Hanya projek yang masih bisa dibelanjai yang boleh dipilih; yang sudah
// selesai/batal tetap memegang pengajuan lamanya tapi tidak menerima yang baru.
const projectOptions = computed(() => [
  { id: '', name: 'Tanpa projek' },
  ...projects.value
    .filter(p => p.status === 'berjalan' || p.status === 'draft')
    .map(p => ({ id: p.id, name: p.project_number ? `${p.name} — ${p.project_number}` : p.name })),
])

async function fetchProjects() {
  try {
    const data = await projectsApi.list()
    projects.value = Array.isArray(data) ? data : (data?.data || [])
  } catch { projects.value = [] }
}

onMounted(async () => {
  await Promise.all([fetchList(), fetchWorkUnits(), fetchMyWorkUnit(), fetchVendors(), fetchProjects()])
  // Datang lewat tombol "Belanja Tahap Baru" di halaman Projek: buka form
  // pengajuan dengan projeknya sudah terpilih.
  const pid = route.query.project_id
  if (pid) {
    openCreate()
    form.value.project_id = String(pid)
  }
})

async function fetchVendors() {
  try {
    const data = await vendorsApi.list({ active: 'true' })
    vendorList.value = Array.isArray(data) ? data : (data?.data || [])
  } catch { vendorList.value = [] }
}

async function fetchMyWorkUnit() {
  try {
    myWorkUnit.value = await workUnitsApi.getMyWorkUnit()
  } catch { myWorkUnit.value = null }
}

async function fetchWorkUnits() {
  try {
    const data = await workUnitsApi.myWorkUnits()
    workUnits.value = data?.data ?? data ?? []
  } catch { workUnits.value = [] }
}

const workUnitFilterOptions = computed(() => [{ id: '', name: 'Semua Unit Kerja' }, ...workUnits.value])

async function fetchList() {
  loading.value = true; errorMsg.value = ''
  try {
    const params = { page: page.value, limit: 20, type: REQUEST_TYPE }
    if (filterWorkUnit.value) params.work_unit_id = filterWorkUnit.value
    if (filterStatus.value) params.status = filterStatus.value
    const data = await purchaseApi.list(params)
    requests.value = data.requests || []
    totalPages.value = data.total_pages || 1
  } catch (err) { errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { loading.value = false }
}

function goPage(p) { page.value = p; fetchList() }

function openCreate() {
  form.value = emptyForm(); createError.value = ''
  if (myWorkUnit.value) {
    form.value.outlet_id = myWorkUnit.value.outlet_id
    form.value.requested_by = myWorkUnit.value.admin_name || authStore.admin?.name || ''
    form.value.work_unit_id = myWorkUnit.value.id
  }
  showCreate.value = true
}

async function submitCreate() {
  createError.value = ''
  if (!myWorkUnit.value) { createError.value = 'Anda belum ditugaskan ke unit kerja.'; return }
  const validItems = form.value.items.filter(it => it.name.trim())
  if (!validItems.length) { createError.value = 'Minimal 1 barang harus diisi.'; return }

  creating.value = true
  try {
    await purchaseApi.create({
      outlet_id: form.value.outlet_id || '',
      work_unit_id: form.value.work_unit_id || '',
      project_id: form.value.project_id || '',
      request_type: REQUEST_TYPE,
      requested_by: form.value.requested_by.trim(),
      items: validItems,
      notes: form.value.notes.trim(),
    })
    toast.success('Pengajuan barang berhasil dikirim.')
    showCreate.value = false; fetchList()
  } catch (err) { createError.value = err?.message ?? 'Gagal membuat pengajuan.' }
  finally { creating.value = false }
}

async function viewDetail(row) {
  try {
    parentDetailId.value = null
    detail.value = await purchaseApi.get(row.id)
    detailItems.value = getUnsplitItems(detail.value)
    detailRejectMode.value = false
    detailRejectReason.value = ''
    showDetail.value = true
  } catch { toast.error('Gagal memuat detail.') }
}

async function viewChildDetail(child) {
  try {
    const d = await purchaseApi.get(child.id)
    // Set state setelah fetch sukses — bila gagal, modal tidak menampilkan
    // tombol "Kembali ke Pengajuan Induk" untuk detail yang tak termuat.
    parentDetailId.value = detail.value?.id || null
    detail.value = d
    detailItems.value = getUnsplitItems(d)
    detailRejectMode.value = false
    detailRejectReason.value = ''
  } catch { toast.error('Gagal memuat detail.') }
}

async function backToParent() {
  if (!parentDetailId.value) return
  try {
    detail.value = await purchaseApi.get(parentDetailId.value)
    detailItems.value = getUnsplitItems(detail.value)
    parentDetailId.value = null
  } catch { toast.error('Gagal memuat detail induk.') }
}

const showDeleteItemConfirm = ref(false)
const deleteItemTarget = ref(null)
let _deleteItemIdx = null
let _deleteSubIdx = null

function removeDetailSubItem(itemIdx, subIdx) {
  const item = detailItems.value[itemIdx]
  if (!item?.items) return
  _deleteItemIdx = itemIdx
  _deleteSubIdx = subIdx
  deleteItemTarget.value = { name: item.items[subIdx]?.name || 'item' }
  showDeleteItemConfirm.value = true
}

function confirmRemoveDetailSubItem() {
  const item = detailItems.value[_deleteItemIdx]
  if (!item?.items) return
  const name = item.items[_deleteSubIdx]?.name || 'item'
  item.items.splice(_deleteSubIdx, 1)
  if (item.items.length === 0) detailItems.value.splice(_deleteItemIdx, 1)
  showDeleteItemConfirm.value = false
  toast.success(`"${name}" berhasil dihapus.`)
  saveDetailQty()
}

async function saveDetailQty() {
  savingDetailQty.value = true
  try {
    await purchaseApi.updateItems(detail.value.id, { items: detailItems.value })
    toast.success('Qty berhasil diperbarui.')
    detail.value = await purchaseApi.get(detail.value.id)
    detailItems.value = getUnsplitItems(detail.value)
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal menyimpan qty.') }
  finally { savingDetailQty.value = false }
}

async function submitDetailApprove() {
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(detail.value.id, { action: 'approve', actor_name: adminName() })
    toast.success('Pengajuan berhasil disetujui.')
    detail.value = await purchaseApi.get(detail.value.id)
    detailItems.value = getUnsplitItems(detail.value)
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal menyetujui.') }
  finally { actionLoading.value = false }
}

async function submitDetailReject() {
  if (!detailRejectReason.value.trim()) { toast.error('Alasan penolakan wajib diisi.'); return }
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(detail.value.id, { action: 'reject', actor_name: adminName(), rejected_reason: detailRejectReason.value.trim() })
    toast.success('Pengajuan ditolak.')
    detail.value = await purchaseApi.get(detail.value.id)
    detailItems.value = getUnsplitItems(detail.value)
    detailRejectMode.value = false
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal menolak.') }
  finally { actionLoading.value = false }
}

function confirmDetailAction(action) {
  actionTarget.value = detail.value
  pendingAction.value = action
  showActionConfirm.value = true
}

async function submitAction() {
  if (!actionTarget.value || !pendingAction.value) return
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(actionTarget.value.id, { action: pendingAction.value, actor_name: adminName() })
    toast.success(`Pengajuan berhasil ${ACTION_META[pendingAction.value]?.done}.`)
    showActionConfirm.value = false
    // Refresh detail modal if open
    if (showDetail.value && detail.value?.id === actionTarget.value.id) {
      detail.value = await purchaseApi.get(detail.value.id)
      detailItems.value = getUnsplitItems(detail.value)
    }
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal mengubah status.') }
  finally { actionLoading.value = false }
}

async function submitReject() {
  if (!rejectReason.value.trim()) { toast.error('Alasan penolakan wajib diisi.'); return }
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(rejectTarget.value.id, { action: 'reject', actor_name: adminName(), rejected_reason: rejectReason.value.trim() })
    toast.success('Pengajuan ditolak.'); showReject.value = false; fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal.') }
  finally { actionLoading.value = false }
}

function confirmDelete(row) { deleteTarget.value = row; showDeleteConfirm.value = true }

async function submitDelete() {
  actionLoading.value = true
  try {
    await purchaseApi.delete(deleteTarget.value.id)
    toast.success('Pengajuan dihapus.'); showDeleteConfirm.value = false; fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal.') }
  finally { actionLoading.value = false }
}

function openEditFinal() {
  editFinalVendorId.value = detail.value?.vendor_id || ''
  editFinalInvoice.value = detail.value?.invoice_number || ''
  // For masters, only show unsplit items for editing
  const items = getUnsplitItems(detail.value || {})
  editFinalItems.value = items.map(it => ({
    ...it,
    items: (it.items || []).map(sub => ({ ...sub }))
  }))
  editFinalError.value = ''
  showEditFinal.value = true
}

async function submitEditFinal() {
  if (!editFinalVendorId.value) { editFinalError.value = 'Silakan pilih vendor terlebih dahulu.'; return }
  savingFinal.value = true; editFinalError.value = ''
  try {
    const vendor = vendorList.value.find(v => v.id === editFinalVendorId.value)
    await purchaseApi.updateItems(detail.value.id, { 
      items: editFinalItems.value,
      vendor_id: editFinalVendorId.value,
      vendor_name: vendor?.name || '',
      invoice_number: editFinalInvoice.value.trim()
    })
    toast.success('Harga dan Vendor berhasil disimpan.')
    showEditFinal.value = false
    if (parentDetailId.value) {
      await backToParent()
    } else {
      detail.value = await purchaseApi.get(detail.value.id)
      detailItems.value = getUnsplitItems(detail.value)
    }
    fetchList()
  } catch (err) { editFinalError.value = err?.message ?? 'Gagal menyimpan.' }
  finally { savingFinal.value = false }
}
</script>

<style scoped>
@reference "tailwindcss";
.input-sm {
  @apply rounded-lg border border-gray-300 px-2.5 py-1.5 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500;
}
.action-btns { display: flex; gap: .35rem; }
/* Nama item / nomor invoice tanpa spasi tidak boleh memaksa halaman
   melebar di layar sempit — pecah di mana pun saat perlu. */
dl dd, dl dt { min-width: 0; overflow-wrap: anywhere; }
.act-view, .act-edit, .act-del {
  width: 28px; height: 28px; border-radius: .45rem; border: none; cursor: pointer;
  display: flex; align-items: center; justify-content: center; transition: all .12s;
}
.act-view { background: rgba(59,130,246,.1); color: #3b82f6; }
.act-view:hover { background: rgba(59,130,246,.2); }
.act-edit { background: rgba(45,143,86,.1); color: #2d8f56; }
.act-edit:hover { background: rgba(45,143,86,.2); }
.act-del  { background: rgba(220,38,38,.07); color: #dc2626; }
.act-del:hover { background: rgba(220,38,38,.15); }
</style>
