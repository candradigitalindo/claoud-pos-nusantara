<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap items-center gap-2">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Pengadaan {{ typeLabel }}</h1>
        <span :class="isBahan ? 'bg-amber-100 text-amber-900 border-amber-300' : 'bg-blue-100 text-blue-900 border-blue-300'"
          class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold">
          <span :class="isBahan ? 'bg-amber-500' : 'bg-blue-500'" class="h-1.5 w-1.5 rounded-full" />
          {{ isBahan ? 'Kelompok 1 · Purchasing Bahan Baku' : 'Kelompok 2 · Purchasing Barang & Jasa' }}
        </span>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <AppButton variant="secondary" :loading="exportingPurchasing" @click="downloadPurchasingExcel">Excel per Vendor</AppButton>
        <AppButton v-if="authStore.hasPermission(perm('submit'))" @click="openCreate">+ Buat Pengajuan</AppButton>
      </div>
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
          <SearchableSelect v-model="filterStatus" aria-label="Filter status pengadaan"
            class="w-full sm:w-auto rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="">Semua Status</option>
            <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
          </SearchableSelect>
        </div>
        <div class="flex w-full flex-col gap-1 sm:w-auto">
          <label class="text-sm font-medium text-gray-700">Tingkat Kebutuhan</label>
          <SearchableSelect v-model="filterUrgency" aria-label="Filter tingkat kebutuhan"
            class="w-full sm:w-auto rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option v-for="o in URGENCY_FILTERS" :key="o.value" :value="o.value">{{ o.label }}</option>
          </SearchableSelect>
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
        <div v-else-if="!requests.length" class="p-6 text-center text-sm text-gray-400">{{ emptyListText }}</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in requests" :key="row.id" class="p-4">
            <div class="flex items-start justify-between gap-2">
              <button @click="viewDetail(row)" class="min-w-0 flex-1 text-left">
                <p class="font-mono text-[11px] text-gray-500">{{ row.request_number || '-' }}</p>
                <p class="mt-0.5 text-sm font-medium text-gray-900 break-words">{{ purchaseDisplayNames(row) }}</p>
                <div class="mt-1 flex flex-wrap items-center gap-1.5">
                  <span v-if="row.split_status === 'master'" class="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-semibold text-blue-700 ring-1 ring-blue-200">
                    Pecah Vendor<template v-if="row.split_count"> ({{ row.split_count }})</template>
                  </span>
                  <span v-else-if="row.parent_id" class="inline-flex items-center gap-1 rounded-full bg-orange-50 px-2 py-0.5 text-[10px] font-semibold text-orange-700 ring-1 ring-orange-200">
                    Pecah Vendor<template v-if="row.parent_number"> · {{ row.parent_number }}</template>
                  </span>
                  <PurchaseUrgencyBadges :doc="row" />
                  <component v-for="d in (row.deliveries || []).slice(0, 2)" :key="`m-sj-${d.id}`"
                    :is="canViewGroupLogistics ? 'router-link' : 'span'"
                    :to="canViewGroupLogistics ? { path: '/logistics/deliveries', query: { delivery_id: d.id } } : undefined" @click.stop
                    :class="['inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold ring-1 ring-inset', deliveryChipClass(d.status)]">
                    {{ d.delivery_number }} · {{ deliveryStatusLabel(d.status) }}
                  </component>
                  <span v-if="(row.deliveries || []).length > 2" class="text-[10px] text-indigo-700 font-semibold">+{{ row.deliveries.length - 2 }} SJ</span>
                </div>
              </button>
              <div class="flex shrink-0 flex-col items-end gap-1">
                <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
                <span v-if="row.receipt_status === 'received' && row.status !== 'received'" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700">Barang Diterima</span>
                <span v-else-if="row.receipt_status === 'partial'" class="rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-semibold text-sky-700">Diterima Sebagian</span>
                <span v-if="payToReturned(row)" class="payto-returned-badge" title="Keuangan mengembalikan detail pembayaran — perbaiki di detail pengajuan">Detail bayar dikembalikan</span>
              </div>
            </div>
            <dl class="mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
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
                <p v-if="remainingDebt(row) > 0" class="text-[10px] text-amber-600">
                  Sisa: {{ formatRupiah(remainingDebt(row)) }}
                </p>
              </div>
              <div class="action-btns shrink-0">
                <AppActionButton icon="view" label="Lihat" @click="viewDetail(row)" />
                <AppActionButton icon="edit" label="Edit" v-if="canEditStatus(row.status)" @click="viewDetail(row)" />
                <AppActionButton icon="delete" label="Hapus" v-if="canDelete(row) && authStore.hasPermission(perm('submit'))" @click="confirmDelete(row)" />
              </div>
            </div>
          </li>
        </ul>
      </div>
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="requests" :loading="loading" :emptyText="emptyListText">
        <template #cell-request_number="{ row }">
          <span class="font-mono text-xs text-gray-700">{{ row.request_number || '-' }}</span>
        </template>
        <template #cell-pengadaan_names="{ row }">
          <div class="flex flex-wrap items-center gap-1.5">
            <span class="text-gray-800">{{ purchaseDisplayNames(row) }}</span>
            <span v-if="row.split_status === 'master'" class="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-semibold text-blue-700 ring-1 ring-blue-200 whitespace-nowrap">
              Pecah Vendor<template v-if="row.split_count"> ({{ row.split_count }})</template>
            </span>
            <span v-else-if="row.parent_id" class="inline-flex items-center gap-1 rounded-full bg-orange-50 px-2 py-0.5 text-[10px] font-semibold text-orange-700 ring-1 ring-orange-200 whitespace-nowrap">
              Pecah Vendor<template v-if="row.parent_number"> · {{ row.parent_number }}</template>
            </span>
          </div>
        </template>
        <template #cell-work_unit_name="{ row }">
          <span class="font-medium text-gray-900">{{ row.work_unit_name || '-' }}</span>
        </template>
        <template #cell-total_amount="{ row }">
          {{ formatRupiah(row.total_amount) }}
          <div v-if="remainingDebt(row) > 0" class="text-[10px] font-normal text-amber-600 mt-0.5">
            Sisa: {{ formatRupiah(remainingDebt(row)) }}
          </div>
        </template>
        <template #cell-status="{ row }">
          <div class="flex flex-col items-start gap-1">
            <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
            <span v-if="row.receipt_status === 'received' && row.status !== 'received'" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 whitespace-nowrap">
              Barang Diterima
            </span>
            <span v-else-if="row.receipt_status === 'partial'" class="rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-semibold text-sky-700 whitespace-nowrap">
              Diterima Sebagian
            </span>
            <div v-if="(row.deliveries || []).length" class="flex flex-wrap gap-1">
              <component v-for="d in row.deliveries.slice(0, 2)" :key="`d-sj-${d.id}`"
                :is="canViewGroupLogistics ? 'router-link' : 'span'"
                :to="canViewGroupLogistics ? { path: '/logistics/deliveries', query: { delivery_id: d.id } } : undefined" @click.stop
                :class="['inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold ring-1 ring-inset whitespace-nowrap', canViewGroupLogistics ? 'hover:opacity-80' : '', deliveryChipClass(d.status)]"
                :title="`${d.delivery_number} (${d.total_received}/${d.total_planned} unit · ${d.destination_label})`">
                {{ d.delivery_number }} · {{ deliveryStatusLabel(d.status) }}
              </component>
              <span v-if="row.deliveries.length > 2" class="text-[10px] font-semibold text-indigo-700">+{{ row.deliveries.length - 2 }} SJ</span>
            </div>
            <PurchaseUrgencyBadges :doc="row" />
            <span v-if="payToReturned(row)" class="payto-returned-badge" title="Keuangan mengembalikan detail pembayaran — perbaiki di detail pengajuan">Detail bayar dikembalikan</span>
          </div>
        </template>
        <template #cell-created_at="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
        <template #cell-actions="{ row }">
          <div class="action-btns">
            <AppActionButton icon="view" label="Lihat" @click="viewDetail(row)" />
            <AppActionButton icon="edit" label="Edit" v-if="canEditStatus(row.status)" @click="viewDetail(row)" />
            <AppActionButton icon="delete" label="Hapus" v-if="canDelete(row) && authStore.hasPermission(perm('submit'))" @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
      <div v-if="totalPages > 1" class="px-4 py-3 border-t border-gray-100">
        <AppPagination :page="page" :totalPages="totalPages" @update:page="goPage" />
      </div>
    </AppCard>

    <!-- Create Modal -->
    <AppModal v-model="showCreate" :title="`Pengajuan Pembelian ${typeLabel}`" size="2xl">
      <form class="modal-form-grid" @submit.prevent="submitCreate">
        <AppAlert class="modal-span-full" type="error" :message="createError" />
        <!-- Unit kerja pengaju: dari /my-work-units (cakupan role). Satu unit
             dipilih otomatis; lebih dari satu wajib dipilih. Outlet ikut unit. -->
        <AppAlert class="modal-span-full" v-if="!workUnits.length" type="warning"
          :message="workUnitsFailed ? 'Daftar unit kerja gagal dimuat — tutup form ini lalu muat ulang halaman.' : 'Anda belum memiliki unit kerja dalam cakupan, jadi pengajuan belum bisa dikirim. Minta admin mengatur unit kerja (Batasan Unit Kerja) role Anda.'" />
        <div v-else class="space-y-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm">
          <p class="text-emerald-800">
            <strong>Pengaju:</strong> {{ form.requested_by || '-' }}
            <template v-if="workUnits.length === 1"> — <strong>Unit Kerja:</strong> {{ unitOptions[0].name }}</template>
          </p>
          <div v-if="workUnits.length > 1" class="flex flex-col gap-1" :data-form-error="formErrors.work_unit_id ? '1' : undefined">
            <label class="text-xs font-semibold text-emerald-900">Unit Kerja Pengaju <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form.work_unit_id" :options="unitOptions"
              placeholder="Pilih unit kerja yang mengajukan…" searchPlaceholder="Cari unit kerja…"
              @update:model-value="clearFieldError('work_unit_id')" />
            <p v-if="formErrors.work_unit_id" class="text-[11px] font-medium text-red-600">{{ formErrors.work_unit_id }}</p>
            <p v-else class="text-[11px] text-emerald-700">Cakupan Anda meliputi beberapa unit kerja — pilih unit yang mengajukan pengadaan ini.</p>
          </div>
        </div>
        <!-- Bahan baku tidak pernah tertaut projek (server menolaknya), jadi
             pemilih projek & baris RAB hanya untuk pengadaan barang. -->
        <div v-if="!isBahan" class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Projek <span class="font-normal text-gray-400">(opsional)</span></label>
          <SearchSelect v-model="form.project_id" :options="projectOptions"
            placeholder="Tanpa projek" searchPlaceholder="Cari projek…" />
          <p class="text-xs text-gray-400">Isi bila belanja ini bagian dari projek pembangunan/renovasi — tiap item lalu memilih baris RAB yang diserapnya.</p>
        </div>
        <div v-if="!isBahan && rabNotSet" :data-form-error="formErrors.project_id ? '1' : undefined" class="modal-span-full rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          RAB projek <b>{{ projectRab.project_number }}</b> belum ditetapkan — pengajuan untuk projek ini ditolak server.
          Susun dan tetapkan RAB di halaman Projek dulu.
        </div>
        <div v-else-if="!isBahan && form.project_id && projectRab" class="modal-span-full rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-800">
          RAB <b>{{ projectRab.project_number }}</b> v{{ projectRab.rab_version }} · {{ rabLines.length }} baris bisa diserap pengajuan ini.
          Pilih baris RAB pada tiap item agar serapannya tercatat per pos; memilih baris mengisi nama, satuan, HPS, dan sumber HPS.
        </div>

        <div v-if="isBahan" class="modal-span-full rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          Pengajuan bahan baku langsung <strong>menunggu persetujuan</strong> (tanpa verifikasi) dan masih bisa diubah sampai disetujui.
          Seluruh bahan baku diterima di <strong>Gudang Induk</strong> dan dicatat sebagai bahan (HPP).
          Pilih bahan melalui pencarian katalog; satuan pembelian dan HPS (harga beli terakhir, tetap bisa diubah) terisi otomatis.
        </div>
        <div v-if="isBahan" class="modal-span-full space-y-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-900">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <label class="font-semibold">Gudang Induk untuk cek stok terakhir</label>
            <button type="button" class="font-medium underline disabled:opacity-50" :disabled="stockCatalogLoading" @click="loadStockItemNames">{{ stockCatalogLoading ? 'Memuat…' : 'Perbarui stok' }}</button>
          </div>
          <SearchSelect v-if="stockWarehouses.length > 1" v-model="stockWarehouseId" :options="stockWarehouses"
            aria-label="Gudang Induk untuk cek stok" placeholder="Pilih Gudang Induk milik pengaju…" searchPlaceholder="Cari Gudang Induk…" />
          <p v-else-if="selectedStockWarehouse" class="font-semibold">{{ selectedStockWarehouse.name }}</p>
          <p v-if="stockCatalogFailed" class="text-red-700">Katalog dan stok gagal dimuat. Klik Perbarui stok sebelum memilih bahan.</p>
          <p v-else-if="!stockCatalogLoading && !stockWarehouses.length" class="text-amber-800">Belum ada Gudang Induk aktif dalam cakupan akun ini; stok tidak tersedia.</p>
          <p v-else-if="!selectedStockWarehouse" class="text-amber-800">Pilih satu Gudang Induk untuk melihat stoknya.</p>
          <p v-if="stockAsOf && selectedStockWarehouse" class="text-emerald-700">Diperbarui {{ formatDateTime(stockAsOf) }} · Stok dapat berubah selama pengajuan diproses.</p>
        </div>

        <!-- Items List -->
        <div class="modal-span-full">
          <label class="text-sm font-medium text-gray-700 mb-2 block">{{ isProjectMode ? 'Daftar Item Pengadaan Projek' : 'Daftar Pengadaan' }}</label>
          <div class="space-y-3">
            <div v-for="(item, i) in form.items" :key="i" class="bg-gray-50 rounded-lg p-3 sm:p-4 border border-gray-200 overflow-hidden">
              <div class="flex items-start gap-2">
                <div class="min-w-0 flex-1 space-y-3">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="bg-emerald-100 text-emerald-700 px-2 py-0.5 rounded text-xs font-semibold">
                      {{ isProjectMode && form.items.length === 1 ? 'Item Projek' : `Pengadaan #${i + 1}` }}
                    </span>
                    <span v-if="isProjectMode && autoGroupName(item)" class="min-w-0 truncate text-xs text-gray-500">
                      {{ autoGroupName(item) }}
                    </span>
                    <AppActionButton v-if="!isProjectMode || form.items.length > 1" icon="delete" label="Hapus baris" type="button" @click="removeItem(i)" :disabled="form.items.length <= 1" class="ml-auto" />
                  </div>
                  <div v-if="!isProjectMode" :data-form-error="formErrors[`group:${i}`] ? '1' : undefined">
                    <input v-model="item.name" placeholder="Nama Pengadaan *"
                      :class="['input-sm w-full min-w-0', formErrors[`group:${i}`] ? '!border-red-400 !bg-red-50/40' : '']"
                      @input="clearFieldError(`group:${i}`)" />
                    <p v-if="formErrors[`group:${i}`]" class="mt-0.5 text-[11px] font-medium text-red-600">{{ formErrors[`group:${i}`] }}</p>
                  </div>
                  <!-- Sub-items table -->
                  <div class="pl-2 sm:ml-2 sm:pl-3 border-l-2 border-emerald-200 space-y-2 min-w-0">
                    <p class="text-xs font-semibold text-emerald-600 uppercase tracking-wide">Detail Item</p>
                    <div v-for="(sub, j) in item.items" :key="j"
                      class="grid grid-cols-2 gap-1.5 rounded-lg border border-gray-200 bg-white p-2 sm:grid-cols-12 sm:items-start sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0">
                      <div v-if="!isBahan && form.project_id && rabLines.length" class="col-span-2 sm:col-span-12 min-w-0">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <span class="shrink-0 rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-bold text-emerald-700">RAB</span>
                          <SearchableSelect :model-value="sub.rab_item_id" class="input-sm min-w-0 flex-1 w-full" @update:model-value="(val) => onRabSelect(sub, val, i, j)">
                            <option value="">— Di luar RAB (tidak menyerap baris mana pun) —</option>
                            <option v-for="l in rabLines" :key="l.id" :value="l.id">{{ rabOptionLabel(l) }}</option>
                          </SearchableSelect>
                        </div>
                        <p v-if="rabWarn(sub)" class="mt-0.5 text-[11px] font-semibold text-red-600 break-words">{{ rabWarn(sub) }}</p>
                      </div>
                      <div v-if="isBahan" class="col-span-2 sm:col-span-4 min-w-0" :data-form-error="formErrors[`sub_name:${i}:${j}`] ? '1' : undefined">
                        <SearchSelect v-model="sub.name" :options="stockItemOptions" value-key="name" label-key="label"
                          :disabled="stockCatalogLoading || stockCatalogFailed" placeholder="Pilih bahan baku…" searchPlaceholder="Cari nama atau kode bahan baku…"
                          :aria-label="`Bahan baku pengadaan ${i + 1} item ${j + 1}`" @change="applyCatalogItem(sub); clearFieldError(`sub_name:${i}:${j}`)" />
                        <p v-if="formErrors[`sub_name:${i}:${j}`]" class="mt-0.5 text-[11px] font-medium text-red-600">{{ formErrors[`sub_name:${i}:${j}`] }}</p>
                        <p v-else-if="sub.name" class="mt-1 text-[11px] text-emerald-700 break-words">{{ stockBalanceLabel(sub) }}</p>
                      </div>
                      <div v-else class="col-span-2 sm:col-span-4 min-w-0" :data-form-error="formErrors[`sub_name:${i}:${j}`] ? '1' : undefined">
                        <input v-model="sub.name" placeholder="Nama item *" autocomplete="off"
                          :class="['input-sm min-w-0 w-full', formErrors[`sub_name:${i}:${j}`] ? '!border-red-400 !bg-red-50/40' : '']"
                          @input="sub._fromRab = false; clearFieldError(`sub_name:${i}:${j}`)" />
                        <p v-if="formErrors[`sub_name:${i}:${j}`]" class="mt-0.5 text-[11px] font-medium text-red-600">{{ formErrors[`sub_name:${i}:${j}`] }}</p>
                      </div>
                      <div class="min-w-0 sm:col-span-2" :data-form-error="formErrors[`sub_qty:${i}:${j}`] ? '1' : undefined">
                        <input v-model.number="sub.qty" type="number" min="1" placeholder="Qty *"
                          :class="['input-sm min-w-0 w-full', formErrors[`sub_qty:${i}:${j}`] ? '!border-red-400 !bg-red-50/40' : '']"
                          @input="clearFieldError(`sub_qty:${i}:${j}`)" />
                        <p v-if="formErrors[`sub_qty:${i}:${j}`]" class="mt-0.5 text-[11px] font-medium text-red-600">{{ formErrors[`sub_qty:${i}:${j}`] }}</p>
                      </div>
                      <input v-model="sub.unit" placeholder="Satuan" class="input-sm min-w-0 w-full sm:col-span-1" />
                      <div class="col-span-2 min-w-0" :data-form-error="formErrors[`sub_hps:${i}:${j}`] ? '1' : undefined">
                        <RupiahInput v-model="sub.hps_price" placeholder="HPS satuan" :error="formErrors[`sub_hps:${i}:${j}`]"
                          @update:model-value="clearFieldError(`sub_hps:${i}:${j}`)" />
                        <p v-if="isBahan && lastPriceHint(sub)" class="mt-0.5 text-[10px] leading-tight text-amber-700">{{ lastPriceHint(sub) }}</p>
                      </div>
                      <p class="col-span-1 min-w-0 break-words self-center text-right text-xs text-gray-500 sm:col-span-2 sm:pt-2">{{ formatRupiah((sub.qty || 0) * (sub.hps_price || 0)) }}</p>
                      <AppActionButton icon="delete" label="Hapus baris" type="button" @click="removeSubItem(i, j)" :disabled="item.items.length <= 1" class="self-center justify-self-end sm:mt-0.5" />
                      <!-- Sumber HPS diberi barisnya sendiri: tautan marketplace
                           panjang dan tidak muat di deret kolom angka. -->
                      <div class="col-span-2 sm:col-span-12">
                        <div class="flex min-w-0 items-center gap-1.5 rounded-lg bg-gray-50 px-2 py-1.5">
                          <span class="shrink-0 text-gray-400">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.5.5l3-3a5 5 0 00-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 00-7.5-.5l-3 3a5 5 0 007 7l1.7-1.7"/></svg>
                          </span>
                          <input v-model="sub.hps_source" class="input-sm min-w-0 flex-1 w-full border-0 bg-transparent px-0"
                            placeholder="Sumber HPS — mis. Toko Jaya (telp 0812…), atau tempel link Shopee/Tokopedia" />
                        </div>
                      </div>
                    </div>
                    <button type="button" @click="addSubItem(i)" class="text-xs text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Item</button>
                  </div>
                  <p class="text-xs text-gray-500 text-right">HPS Pengadaan: <strong>{{ formatRupiah(calcItemHps(item)) }}</strong></p>
                </div>
              </div>
            </div>
          </div>
          <div v-if="!isProjectMode" class="mt-3">
            <button type="button" @click="addItem" class="text-sm text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Pengadaan</button>
          </div>
          <div v-if="mixedKind" class="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
            <p class="font-semibold">Pengajuan ini mencampur barang dapur dan peralatan.</p>
            <p class="mt-1">
              Barang dapur ({{ mixedKind.dapur.join(', ') }}) diterima di <strong>Gudang Induk</strong>,
              sedangkan peralatan ({{ mixedKind.perlengkapan.join(', ') }}) diterima di <strong>bagian Aset</strong>.
              Pisahkan menjadi dua pengajuan — server akan menolak yang tercampur.
            </p>
          </div>
          <p v-if="form.items.length" class="mt-2 text-sm font-semibold text-gray-700">
            Total HPS: {{ formatRupiah(estimatedTotal) }}
          </p>
        </div>

        <div class="modal-span-full space-y-1" :data-form-error="formErrors.need_by_date ? '1' : undefined">
          <PurchaseUrgencyFields v-model:urgency="form.urgency" v-model:need-by="form.need_by_date"
            :can-emergency="false" :emergency-max="emergencyMax"
            @update:need-by="clearFieldError('need_by_date')" @update:urgency="clearFieldError('need_by_date'); clearFieldError('notes')" />
          <p v-if="formErrors.need_by_date" class="text-xs font-medium text-red-600">{{ formErrors.need_by_date }}</p>
        </div>
        <div class="modal-span-full" :data-form-error="formErrors.notes ? '1' : undefined">
          <AppInput v-model="form.notes" label="Catatan / Alasan Pembelian" :error="formErrors.notes"
            :placeholder="form.urgency === 'urgent' ? 'Wajib: dampaknya bila tidak segera dibeli' : 'Keterangan tambahan (opsional)'"
            @update:model-value="clearFieldError('notes')" />
        </div>
      </form>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton :loading="creating" :disabled="!workUnits.length" @click="submitCreate">Kirim Pengajuan</AppButton>
      </template>
    </AppModal>

    <!-- Detail Modal -->
    <AppModal v-model="showDetail" :title="`Detail Pengajuan ${typeLabel}`" size="2xl">
      <div v-if="detail" class="space-y-4">
        <button v-if="parentDetailId" @click="backToParent" class="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800 font-medium mb-1">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><polyline points="12 19 5 12 12 5"/></svg>
          Kembali ke Pengajuan Induk
        </button>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span :class="statusBadge(detail.status)" class="text-sm">{{ statusLabel(detail.status) }}</span>
          <PurchaseUrgencyBadges :doc="detail" />
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
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Pengaju</dt>
                <dd class="font-medium text-gray-900">{{ detail.requested_by }}</dd>
                <dt class="text-gray-400">Diajukan</dt>
                <dd class="text-gray-600">{{ formatDateTime(detail.created_at) }}</dd>
                <dt class="text-gray-400">Kebutuhan</dt>
                <dd class="text-gray-700">{{ urgencyLabel(detail.urgency) }}<template v-if="detail.need_by_date"> · butuh {{ formatDateStr(detail.need_by_date) }}</template></dd>
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
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="showHps">
                  <dt class="text-gray-400">HPS</dt>
                  <dd class="font-medium text-gray-900">{{ formatRupiah(canEditDetailQty ? detailHpsTotal : detail.total_hps) }}</dd>
                </template>
                <dt class="text-gray-400">{{ detail.children?.length ? 'Total' : 'Harga' }}</dt>
                <dd class="font-semibold text-emerald-700">{{ detail.total_final ? formatRupiah(detail.total_final) : 'Belum ditentukan' }}</dd>
              </dl>
            </div>
          </div>

          <!-- Verifikasi -->
          <div v-if="detail.verified_by || detail.verified_at" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-indigo-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-indigo-600 mb-1.5">Verifikasi</p>
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Oleh</dt>
                <dd class="font-medium text-gray-900">{{ detail.verified_by || '-' }}</dd>
                <template v-if="detail.verified_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.verified_at) }}</dd>
                </template>
                <template v-if="detail.rejected_items?.length">
                  <dt class="text-gray-400">Item ditolak</dt>
                  <dd class="font-medium text-red-600">{{ detail.rejected_items.length }} item (lihat di bawah)</dd>
                </template>
                <template v-if="detail.verification_notes">
                  <dt class="text-gray-400">Catatan</dt>
                  <dd class="text-gray-700">{{ detail.verification_notes }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Persetujuan -->
          <div v-if="detail.approved_by && detail.status !== 'rejected'" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-amber-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-amber-600 mb-1.5">Persetujuan</p>
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Oleh</dt>
                <dd class="font-medium text-gray-900">{{ detail.approved_by }}</dd>
                <template v-if="detail.approved_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.approved_at) }}</dd>
                </template>
              </dl>
            </div>
          </div>

          <!-- Penolakan (oleh verifikator atau penyetuju). Data lama menulis
               penolak ke approved_by — dipakai sebagai cadangan. -->
          <div v-if="detail.status === 'rejected'" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-red-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-red-600 mb-1.5">Penolakan</p>
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Oleh</dt>
                <dd class="font-medium text-gray-900">{{ detail.rejected_by || detail.approved_by || '-' }}</dd>
                <template v-if="detail.rejected_at || detail.approved_at">
                  <dt class="text-gray-400">Waktu</dt>
                  <dd class="text-gray-600">{{ formatDateTime(detail.rejected_at || detail.approved_at) }}</dd>
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
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
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

          <!-- Tujuan pembayaran: diisi Pembelian saat mengajukan; Keuangan hanya
               membaca. Bisa diubah selama tagihan belum lunas. -->
          <div v-if="payToEdit.show && !detail.emergency" class="col-span-1 sm:col-span-2">
            <PurchasePayToCard :pay-to="detail.pay_to" :amount="detailRemaining">
              <div v-if="canPurchasingPayTo" class="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 px-4 py-2">
                <p v-if="!payToEdit.editable" class="mr-auto text-[11px] text-amber-700">{{ payToEdit.reason }}</p>
                <AppButton size="sm" :variant="payToReturnedDetail ? 'primary' : 'secondary'" :disabled="!payToEdit.editable" @click="openPayToEdit">
                  {{ payToReturnedDetail ? 'Perbaiki Detail Pembayaran' : 'Ubah Detail Pembayaran' }}
                </AppButton>
              </div>
            </PurchasePayToCard>
          </div>

          <!-- Penerimaan -->
          <div v-if="detail.received_by" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-purple-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-purple-600 mb-1.5">Penerimaan</p>
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
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
          <div v-if="detail.invoice_number || detailInvoiceUrls.length || detail.notes" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden col-span-1 sm:col-span-2">
            <div class="border-l-4 border-gray-400 px-3 py-2.5 flex flex-wrap items-start justify-between gap-2">
              <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                <template v-if="detail.invoice_number">
                  <dt class="text-gray-400">No. Invoice / Struk</dt>
                  <dd class="font-medium font-mono text-gray-900">{{ detail.invoice_number }}</dd>
                </template>
                <template v-if="detail.notes">
                  <dt class="text-gray-400">Catatan</dt>
                  <dd class="text-gray-700">{{ detail.notes }}</dd>
                </template>
              </dl>
              <div v-if="detailInvoiceUrls.length" class="flex flex-wrap items-center gap-1.5">
                <AppActionButton v-for="(u, idx) in detailInvoiceUrls" :key="`inv-${idx}`" icon="photo"
                  :label="detailInvoiceUrls.length > 1 ? `Struk/Invoice ${idx + 1}` : 'Lihat Struk / Invoice'"
                  :href="u" target="_blank" />
              </div>
            </div>
          </div>
        </div>

        <!-- Sinkronisasi read-only dari modul Logistik → Pengantaran Logistik (posisi teratas agar langsung terlihat) -->
        <div v-if="logisticsDeliveriesOfDetail.length && !detail.emergency" class="rounded-xl border border-indigo-200 bg-indigo-50/40 p-3 sm:p-4 space-y-2">
          <p class="flex items-center gap-1.5 text-sm font-semibold text-indigo-950">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 3h15v13H1zM16 8h4l3 5v3h-7V8z"/><circle cx="5.5" cy="18.5" r="2"/><circle cx="18.5" cy="18.5" r="2"/></svg>
            Status Pengantaran Logistik <span class="font-normal text-indigo-700/70">({{ logisticsDeliveriesOfDetail.length }})</span>
          </p>
          <ul class="divide-y divide-indigo-100 rounded-lg bg-white border border-indigo-100 px-3">
            <li v-for="d in logisticsDeliveriesOfDetail" :key="d.id" class="flex flex-wrap items-center justify-between gap-2 py-2 text-xs">
              <div class="min-w-0 flex-1">
                <component :is="canViewGroupLogistics ? 'router-link' : 'span'"
                  :to="canViewGroupLogistics ? { path: '/logistics/deliveries', query: { delivery_id: d.id } } : undefined"
                  :class="['font-mono font-bold text-indigo-700', canViewGroupLogistics ? 'hover:underline' : '']">
                  {{ d.delivery_number }}
                </component>
                <span class="ml-1.5 rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-semibold text-gray-600">{{ dispatchModeLabel(d.dispatch_mode) }}</span>
                <p class="text-gray-500 mt-0.5">
                  {{ d.delivery_date }} · {{ d.courier_name || (d.dispatch_mode === 'vendor_direct' ? 'Vendor' : 'Logistik') }} → <strong>{{ d.destination_label }}</strong>
                  · <strong class="text-gray-800">{{ d.total_received }}/{{ d.total_planned }} unit diterima</strong>
                </p>
              </div>
              <div class="flex shrink-0 flex-wrap items-center gap-1.5">
                <span :class="deliveryStatusClass(d.status)" class="rounded-full px-2 py-0.5 text-[10px] font-semibold">
                  {{ deliveryStatusLabel(d.status) }}
                </span>
                <AppActionButton v-if="canViewGroupLogistics" icon="print" label="Cetak SJ" @click="quickPrintDelivery(d.id)" />
              </div>
            </li>
          </ul>
        </div>

        <div v-if="detail.children?.length" class="bg-emerald-50 rounded-xl p-4 border border-emerald-200">
          <p class="text-sm font-bold text-emerald-900 mb-3 flex items-center gap-2">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M12 2v8m0 0l-4-4m4 4l4-4M12 22v-8m0 0l-4 4m4-4l4 4"/></svg>
            Pecahan Vendor ({{ detail.children.length }})
          </p>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div v-for="child in detail.children" :key="child.id" class="bg-white border border-emerald-100 rounded-lg p-3 shadow-sm flex flex-col gap-2 min-w-0">
              <div class="flex flex-wrap justify-between items-start gap-2">
                <div class="min-w-0 flex-1">
                  <p class="text-xs font-bold text-gray-900 break-words">{{ child.vendor_name || 'Vendor Lain' }}</p>
                  <p v-if="child.request_number" class="text-[10px] text-gray-500 font-mono">No. {{ child.request_number }}</p>
                </div>
                <span :class="statusBadge(child.status)" class="text-[10px] shrink-0">{{ statusLabel(child.status) }}</span>
              </div>
              <!-- Status sinkron Logistik & Penerimaan per pecahan -->
              <div v-if="(child.deliveries || []).length || child.receipt_status" class="flex flex-wrap gap-1">
                <component v-for="cd in child.deliveries || []" :key="`ch-sj-${child.id}-${cd.id}`"
                  :is="canViewGroupLogistics ? 'router-link' : 'span'"
                  :to="canViewGroupLogistics ? { path: '/logistics/deliveries', query: { delivery_id: cd.id } } : undefined"
                  :class="['inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold ring-1 ring-inset', canViewGroupLogistics ? 'hover:opacity-80' : '', deliveryChipClass(cd.status)]">
                  {{ cd.delivery_number }} · {{ deliveryStatusLabel(cd.status) }} ({{ cd.total_received }}/{{ cd.total_planned }})
                </component>
                <span v-if="!(child.deliveries || []).length && child.receipt_status === 'received'" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700">
                  Barang Diterima
                </span>
                <span v-else-if="!(child.deliveries || []).length && child.receipt_status === 'partial'" class="rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-semibold text-sky-700">
                  Diterima Sebagian
                </span>
              </div>
              <div class="flex justify-between items-end gap-2 mt-auto">
                <p class="min-w-0 break-words text-xs font-bold text-emerald-700">{{ formatRupiah(child.total_final || child.total_amount) }}</p>
                <AppActionButton icon="view" label="Lihat Detail →" @click="viewChildDetail(child)" />
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
                  <span v-if="showHps" class="text-gray-500">HPS: <strong>{{ formatRupiah(canEditDetailQty ? calcDetailItemHps(item) : item.hps_total) }}</strong></span>
                  <span class="text-emerald-700">Harga: <strong>{{ item.final_total != null && item.final_total > 0 ? formatRupiah(item.final_total) : '-' }}</strong></span>
                </div>
              </div>
              <!-- Mobile: satu kartu per item -->
              <div class="space-y-2 sm:hidden">
                <div v-for="(sub, j) in item.items" :key="j" class="rounded-lg border border-gray-200 bg-white p-2.5">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p class="break-words text-xs font-medium text-gray-800">{{ j + 1 }}. {{ sub.name }}</p>
                      <span v-if="sub.rab_item_id" class="rab-chip">RAB · {{ rabName(sub.rab_item_id) }}</span>
                    </div>
                    <AppActionButton icon="delete" label="Hapus baris" v-if="canEditItems" @click="removeDetailSubItem(i, j)" />
                  </div>
                  <dl class="mt-1.5 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[11px]">
                    <dt class="text-gray-400">Qty diajukan</dt>
                    <dd class="text-gray-800">
                      <span v-if="canEditDetailQty" class="inline-flex items-center gap-1">
                        <input v-model.number="sub.qty" type="number" min="1" step="1" class="w-20 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span>{{ sub.unit }}</span>
                      </span>
                      <span v-else>{{ sub.qty }} {{ sub.unit }}</span>
                      <span v-if="sub.requested_qty != null && sub.requested_qty !== sub.qty" class="ml-1 text-[10px] text-amber-700">(diajukan {{ sub.requested_qty }})</span>
                    </dd>
                    <template v-if="sub.purchased_qty != null">
                      <dt class="text-gray-400">Qty dibeli</dt>
                      <dd class="font-semibold text-emerald-700">{{ purchaseQuantity(sub) }} {{ sub.unit }}</dd>
                    </template>
                    <template v-if="showHps && sub.hps_source">
                      <dt class="text-gray-400">Sumber HPS</dt>
                      <dd class="min-w-0 overflow-hidden">
                        <a v-if="extractSourceUrl(sub.hps_source)" :href="extractSourceUrl(sub.hps_source)" target="_blank" rel="noopener noreferrer"
                          class="block max-w-full truncate text-emerald-700 underline" :title="sub.hps_source">{{ formatSourceLabel(sub.hps_source) }}</a>
                        <span v-else class="block break-words [overflow-wrap:anywhere] text-gray-700">{{ sub.hps_source }}</span>
                      </dd>
                    </template>
                    <template v-if="showHps">
                      <dt class="text-gray-400">HPS</dt>
                      <dd class="text-gray-700">
                        {{ formatRupiah(sub.hps_price) }}<span class="text-gray-400"> × {{ sub.qty }} = </span>
                        <strong>{{ formatRupiah(canEditDetailQty ? (sub.qty || 0) * (sub.hps_price || 0) : sub.hps_subtotal) }}</strong>
                      </dd>
                    </template>
                    <dt class="text-gray-400">Harga</dt>
                    <dd class="text-gray-700">
                      <template v-if="sub.final_price">
                        {{ formatRupiah(sub.final_price) }}<span class="text-gray-400"> × {{ purchaseQuantity(sub) }} = </span>
                        <strong class="text-emerald-700">{{ formatRupiah(purchaseQuantity(sub) * sub.final_price) }}</strong>
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
                      <th v-if="canEditItems" class="py-1.5 w-8"></th>
                      <th class="py-1.5 pr-2">#</th>
                      <th class="py-1.5 pr-2">Nama Item</th>
                      <th class="py-1.5 pr-2 text-right">Qty Diajukan</th>
                      <th class="py-1.5 pr-2 text-right">Qty Dibeli</th>
                      <th class="py-1.5 pr-2">Satuan</th>
                      <th v-if="showHps" class="py-1.5 pr-2 text-right">HPS Satuan</th>
                      <th v-if="showHps" class="py-1.5 pr-2 text-right">HPS Total</th>
                      <th v-if="showHps" class="py-1.5 pr-2 text-left">Sumber HPS</th>
                      <th class="py-1.5 pr-2 text-right">Harga Satuan</th>
                      <th class="py-1.5 text-right">Harga Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(sub, j) in item.items" :key="j" class="border-b border-gray-100">
                      <td v-if="canEditItems" class="py-1.5 text-center">
                        <AppActionButton icon="delete" label="Hapus baris" @click="removeDetailSubItem(i, j)" />
                      </td>
                      <td class="py-1.5 pr-2 text-gray-400">{{ j + 1 }}</td>
                      <td class="py-1.5 pr-2 text-gray-800">
                        {{ sub.name }}
                        <span v-if="sub.rab_item_id" class="rab-chip block w-fit">RAB · {{ rabName(sub.rab_item_id) }}</span>
                      </td>
                      <td class="py-1.5 pr-2 text-right">
                        <input v-if="canEditDetailQty" v-model.number="sub.qty" type="number" min="1" step="1" class="w-24 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span v-else>{{ sub.qty }}</span>
                        <span v-if="sub.requested_qty != null && sub.requested_qty !== sub.qty" class="block text-[10px] text-amber-700">diajukan {{ sub.requested_qty }}</span>
                      </td>
                      <td class="py-1.5 pr-2 text-right font-medium text-emerald-700">{{ sub.purchased_qty != null ? purchaseQuantity(sub) : '—' }}</td>
                      <td class="py-1.5 pr-2">{{ sub.unit }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(sub.hps_price) }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(canEditDetailQty ? (sub.qty || 0) * (sub.hps_price || 0) : sub.hps_subtotal) }}</td>
                      <td v-if="showHps" class="w-[12rem] max-w-[12rem] py-1.5 pr-2">
                        <div class="max-w-[12rem] min-w-0">
                          <input v-if="canEditItems" v-model="sub.hps_source" class="input-sm w-full min-w-0 py-0.5 px-1"
                            placeholder="toko / link" @change="saveDetailQty()" />
                          <a v-else-if="extractSourceUrl(sub.hps_source)" :href="extractSourceUrl(sub.hps_source)" target="_blank" rel="noopener noreferrer"
                            class="block max-w-[12rem] truncate text-emerald-700 underline" :title="sub.hps_source">{{ formatSourceLabel(sub.hps_source) }}</a>
                          <span v-else class="block max-w-[12rem] truncate text-gray-600" :title="sub.hps_source">{{ sub.hps_source || '—' }}</span>
                        </div>
                      </td>
                      <td class="py-1.5 pr-2 text-right">{{ sub.final_price ? formatRupiah(sub.final_price) : '-' }}</td>
                      <td class="py-1.5 text-right font-medium">{{ sub.final_price ? formatRupiah(purchaseQuantity(sub) * sub.final_price) : '-' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- Totals -->
          <div class="flex flex-wrap justify-end gap-x-6 gap-y-1 mt-3 text-sm">
            <span v-if="showHps" class="text-gray-700">Total HPS: <strong>{{ formatRupiah(canEditDetailQty ? detailHpsTotal : detail.total_hps) }}</strong></span>
            <span class="text-emerald-700">Total Harga: <strong>{{ detail.total_final ? formatRupiah(detail.total_final) : '-' }}</strong></span>
          </div>

          <!-- Purchasing Workflow Selection (Only for Main/Master requests) -->
          <div v-if="detail.status === 'approved' && authStore.hasPermission(perm('purchasing'))" class="mt-6">
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
              <div class="flex items-center gap-3 min-w-0 flex-1">
                <div class="w-10 h-10 shrink-0 rounded-full bg-blue-100 flex items-center justify-center text-blue-600">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-bold text-blue-900">Data Hasil Split</p>
                  <p class="text-xs text-blue-700 break-words">Silakan lengkapi harga final untuk vendor ini<template v-if="detail.parent_id">, lalu ajukan pembayarannya</template>.</p>
                </div>
              </div>
              <div class="flex w-full shrink-0 flex-col gap-2 sm:w-auto sm:flex-row">
                <AppActionButton icon="cost" label="Isi Harga Sekarang" @click="openEditFinal" />
                <!-- Pecahan diajukan bayar sendiri: pecahan yang belum berharga saat
                     induknya diajukan tidak ikut cascade dan tertahan di 'approved'.
                     Nonaktif selama masih ada item tanpa harga final (server menolak). -->
                <AppActionButton icon="pay" :label="paymentReadiness.ok ? 'Ajukan Pembayaran' : 'Isi harga final semua item dulu'" v-if="detail.parent_id" :disabled="!paymentReadiness.ok" @click="confirmDetailAction('request_payment')" />
              </div>
            </div>

            <!-- Pemblokir "Ajukan Pembayaran": server menolak selama ada item tanpa
                 harga final (dokumen tunggal, pecahan, atau sisa item induk). -->
            <div v-if="!paymentReadiness.ok" class="mt-3 rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
              <p class="flex items-center gap-1.5 font-semibold">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                Pembayaran belum bisa diajukan
              </p>
              <ul class="mt-1 list-disc space-y-0.5 pl-5">
                <li v-for="(b, k) in paymentReadiness.blockers" :key="k" class="break-words">{{ b }}</li>
              </ul>
            </div>
            <ul v-if="paymentReadiness.notes.length" class="mt-2 space-y-1 rounded-lg border border-blue-100 bg-blue-50/60 p-2 text-[11px] text-blue-900">
              <li v-for="(n, k) in paymentReadiness.notes" :key="k" class="break-words">{{ n }}</li>
            </ul>
          </div>
        </div>

        <!-- Item yang ditolak verifikator: server memindahkannya dari `items`
             ke `rejected_items`, jadi tidak ikut total maupun pembelian. -->
        <div v-if="detail.rejected_items?.length" class="rounded-xl border border-red-200 bg-red-50/60 p-3 sm:p-4">
          <p class="mb-2 flex items-center gap-1.5 text-sm font-semibold text-red-800">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
            Item ditolak saat verifikasi ({{ detail.rejected_items.length }})
          </p>
          <ul class="space-y-2">
            <li v-for="(ri, k) in detail.rejected_items" :key="k" class="rounded-lg border border-red-100 bg-white p-2.5 text-xs">
              <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
                <p class="min-w-0 break-words font-medium text-gray-500 line-through">{{ ri.name }}</p>
                <p class="text-gray-400 line-through">{{ ri.qty }} {{ ri.unit }}<template v-if="showHps"> · HPS {{ formatRupiah((ri.qty || 0) * (ri.hps_price || 0)) }}</template></p>
              </div>
              <p v-if="ri.group" class="break-words text-[11px] text-gray-400">{{ ri.group }}</p>
              <p class="mt-1 break-words text-red-700"><span class="font-semibold">Alasan:</span> {{ ri.reason || '-' }}</p>
              <p class="mt-0.5 text-[11px] text-gray-400">Ditolak oleh {{ ri.rejected_by || '-' }}<template v-if="ri.rejected_at"> · {{ formatDateTime(ri.rejected_at) }}</template></p>
            </li>
          </ul>
        </div>

        <!-- Arsip read-only untuk data struk lama (jika ada pada transaksi terdahulu).
             Pembelian baru cukup memakai 1 No. Invoice/Struk + Multi-Foto di form Isi Harga. -->
        <div v-if="(detail.struks || []).length && !detail.emergency" class="rounded-xl border border-gray-200 bg-gray-50/70 p-3 space-y-1.5">
          <p class="text-xs font-semibold text-gray-600">Arsip Lampiran Struk Lama ({{ detail.struks.length }})</p>
          <ul class="divide-y divide-gray-200/80 text-xs">
            <li v-for="s in detail.struks" :key="s.id" class="flex flex-wrap items-center justify-between gap-2 py-1.5">
              <div class="min-w-0">
                <span class="font-mono font-semibold text-gray-800">{{ s.struk_number }}</span>
                <span class="text-gray-500 ml-1.5">· {{ s.struk_date }} · {{ formatRupiah(s.total) }}</span>
              </div>
              <div class="flex flex-wrap gap-1.5">
                <AppActionButton v-for="(u, idx) in safeUrls(s.photo_url, s.photo_urls)" :key="`${s.id}-${idx}`" icon="photo"
                  :label="safeUrls(s.photo_url, s.photo_urls).length > 1 ? `Foto ${idx + 1}` : 'Lihat Foto'" :href="u" target="_blank" />
              </div>
            </li>
          </ul>
        </div>


        <!-- Verifikasi (status pending, izin verify jenisnya): pastikan barang
             belum tersedia di outlet lain sebelum diteruskan ke penyetuju. -->
        <div v-if="detail.status === 'pending' && canVerify" class="space-y-3 rounded-xl border border-indigo-200 bg-indigo-50/50 p-3 sm:p-4">
          <div>
            <p class="flex items-center gap-1.5 text-sm font-bold text-indigo-900">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/></svg>
              Verifikasi Ketersediaan
            </p>
            <p class="mt-0.5 text-xs text-indigo-800">
              Periksa apakah tiap barang masih tersedia di outlet lain. Barang yang masih ada ditolak per item
              beserta alasannya; item lain tetap diteruskan ke persetujuan.
            </p>
          </div>
          <ul class="space-y-2">
            <li v-for="row in verifyRows" :key="row.key" class="rounded-lg border bg-white p-2.5" :class="row.reject ? 'border-red-200' : 'border-gray-200'">
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0 flex-1">
                  <p class="break-words text-sm font-medium" :class="row.reject ? 'text-gray-400 line-through' : 'text-gray-900'">{{ row.name }}</p>
                  <p class="break-words text-[11px] text-gray-500">{{ row.groupName }} · {{ row.qty }} {{ row.unit }}<template v-if="showHps"> · HPS {{ formatRupiah((row.qty || 0) * (row.hps_price || 0)) }}</template></p>
                </div>
                <label class="inline-flex shrink-0 cursor-pointer select-none items-center gap-1.5 text-xs font-semibold text-red-700">
                  <input v-model="row.reject" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-500" />
                  Tolak item ini
                </label>
              </div>
              <div v-if="row.reject" class="mt-2">
                <input v-model="row.reason" class="input-sm w-full" placeholder="Tersedia di outlet … — konfirmasi ke bagian perlengkapan" />
                <p v-if="reasonLength(row.reason) < VERIFY_REASON_MIN" class="mt-0.5 text-[11px] text-red-600">Alasan wajib diisi, minimal {{ VERIFY_REASON_MIN }} huruf.</p>
              </div>
            </li>
          </ul>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-700">Catatan verifikasi <span class="font-normal text-gray-400">(opsional)</span></label>
            <textarea v-model="verifyNotes" rows="2" class="input-sm w-full" placeholder="mis. sudah dicek ke outlet Kala & Semesta — tidak ada yang menganggur"></textarea>
          </div>
          <AppAlert v-if="verifyAllRejected" type="warning"
            message="Semua item ditandai ditolak — pengajuan ini akan berstatus Ditolak dan tidak diteruskan ke persetujuan." />
          <div v-if="detailRejectMode" class="flex flex-col gap-2 sm:flex-row sm:items-end">
            <div class="min-w-0 flex-1">
              <label class="text-xs font-medium text-red-600 mb-1 block">Alasan Menolak Seluruh Pengajuan</label>
              <input v-model="detailRejectReason" placeholder="Masukkan alasan penolakan..." class="input-sm w-full border-red-300 focus:ring-red-500 focus:border-red-500" />
            </div>
            <div class="flex items-center justify-end gap-2">
              <button @click="detailRejectMode = false" class="px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700 rounded-lg hover:bg-gray-100">Batal</button>
              <button @click="submitDetailReject" :disabled="actionLoading" class="px-3 py-1.5 text-sm text-white bg-red-600 hover:bg-red-700 rounded-lg font-medium disabled:opacity-50">Konfirmasi Tolak</button>
            </div>
          </div>
          <div v-else class="flex flex-wrap items-center justify-end gap-2">
            <AppActionButton icon="reject" label="Tolak Seluruh Pengajuan" @click="detailRejectMode = true" />
            <AppActionButton :icon="verifyAllRejected ? 'reject' : 'review'" :label="(verifyAllRejected ? 'Tolak Semua Item' : 'Verifikasi &amp; Teruskan ke Persetujuan')" @click="submitVerify" :disabled="actionLoading || !verifyRows.length" />
          </div>
        </div>

        <!-- Persetujuan: untuk pengajuan verified, atau pengajuan urgent yang masih pending. -->
        <div v-if="canShowApprovePanel" class="rounded-xl border border-blue-200 bg-blue-50/40 p-3 sm:p-4">
          <p class="mb-3 text-xs text-gray-600">
            <template v-if="isBahan">Diajukan oleh <strong>{{ detail.requested_by || '-' }}</strong> · {{ formatDateTime(detail.created_at) }} — bahan baku tanpa tahap verifikasi.</template>
            <template v-else-if="detail.status === 'pending'">Diajukan oleh <strong>{{ detail.requested_by || '-' }}</strong> · {{ formatDateTime(detail.created_at) }} — pengajuan <strong>Urgent</strong> dapat langsung diputuskan penyetuju.</template>
            <template v-else>Diverifikasi oleh <strong>{{ detail.verified_by || '-' }}</strong><template v-if="detail.verified_at"> · {{ formatDateTime(detail.verified_at) }}</template><template v-if="detail.rejected_items?.length"> · {{ detail.rejected_items.length }} item ditolak saat verifikasi</template>.</template>
          </p>
          <p v-if="canApproveEditQty" class="mb-3 rounded-lg border border-blue-200 bg-white/80 px-3 py-1.5 text-[11px] text-blue-900">
            Penyetuju dapat menyesuaikan <strong>Qty</strong> pada daftar item di atas sebelum menyetujui — total HPS dihitung ulang otomatis dan qty awal pengaju tetap tercatat.
          </p>
          <!-- Anggaran projek baru terpakai saat disetujui: peringatkan bila melampaui sisa baris RAB. -->
          <div v-if="approvalRabWarnings.length" class="mb-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
            <p class="font-semibold">Menyetujui pengajuan ini melampaui sisa anggaran RAB:</p>
            <ul class="mt-1 list-disc space-y-0.5 pl-4">
              <li v-for="w in approvalRabWarnings" :key="w.id">{{ w.name }} — sisa {{ formatRupiah(w.remaining) }}, pengajuan ini {{ formatRupiah(w.amount) }}</li>
            </ul>
            <p class="mt-1 text-amber-700">Persetujuan tetap bisa dilakukan. Bila perlu, revisi RAB di halaman Projek lebih dulu.</p>
          </div>
          <!-- Pilihan jalur proses setelah disetujui: Proses Umum atau Langsung ke Kas Kasir Outlet -->
          <div v-if="detail.outlet_id && !detail.children?.length && !detailRejectMode" class="mb-3 rounded-lg border border-blue-200 bg-white p-3 text-xs">
            <p class="mb-2 font-semibold text-gray-800">Arahkan Pembayaran / Proses</p>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border p-2.5 transition"
                :class="!approveDirectCash ? 'border-blue-500 bg-blue-50/60 ring-1 ring-blue-500' : 'border-gray-200 hover:border-blue-200'">
                <input v-model="approveDirectCash" type="radio" :value="false" class="mt-0.5 text-blue-600" />
                <div>
                  <p class="font-semibold text-gray-900">Proses Melalui Umum</p>
                  <p class="mt-0.5 text-[11px] text-gray-500">Dilanjutkan ke bagian Purchasing untuk pengisian harga final, vendor, dan pembayaran reguler.</p>
                </div>
              </label>
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border p-2.5 transition"
                :class="approveDirectCash ? 'border-emerald-500 bg-emerald-50/60 ring-1 ring-emerald-500' : 'border-gray-200 hover:border-emerald-200'">
                <input v-model="approveDirectCash" type="radio" :value="true" class="mt-0.5 text-emerald-600" />
                <div>
                  <p class="font-semibold text-gray-900">Langsung Pembayaran Kasir Outlet</p>
                  <p class="mt-0.5 text-[11px] text-gray-500">Kirim tugas penyerahan kas senilai HPS ({{ formatRupiah(canEditDetailQty ? detailHpsTotal : detail.total_hps) }}) langsung ke tablet kasir outlet.</p>
                </div>
              </label>
            </div>
            <div v-if="approveDirectCash" class="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div>
                <label class="mb-1 block text-[11px] font-medium text-gray-700">Diserahkan Kepada (Penerima Kas) *</label>
                <input v-model="approvePayoutRecipient" type="text" placeholder="Nama staf / penerima kas" class="input-sm w-full" />
              </div>
              <div>
                <label class="mb-1 block text-[11px] font-medium text-gray-700">Catatan untuk Kasir <span class="font-normal text-gray-400">(opsional)</span></label>
                <input v-model="approvePayoutNotes" type="text" placeholder="Mis. belanja tunai mendesak" class="input-sm w-full" />
              </div>
            </div>
          </div>
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
            <AppActionButton icon="reject" label="Tolak" @click="detailRejectMode = true" />
            <AppActionButton icon="approve" :label="approveDirectCash ? 'Setujui &amp; Kirim ke Kasir' : 'Setujui'" @click="submitDetailApprove" :disabled="actionLoading" />
          </div>
        </div>
      </div>
      <template #footer>
        <div v-if="!parentDetailId" class="flex flex-wrap items-center gap-2 mr-auto">
          <!-- Nonaktif selama ada item tanpa harga final; alasannya tampil di atas. -->
          <AppActionButton icon="pay" :label="paymentReadiness.ok ? 'Ajukan Pembayaran' : 'Belum bisa diajukan — lihat keterangan di atas'" v-if="detail && detail.status === 'approved' && authStore.hasPermission(perm('purchasing')) && !detail.parent_id" :disabled="!paymentReadiness.ok"
            @click="confirmDetailAction('request_payment')" />
          <!-- Serah terima hanya untuk pemegang wewenang Aset / Gudang yang barangnya belum diterima semua. -->
          <AppActionButton icon="receive" :label="(detail.receipt_status === 'partial' ? 'Lanjutkan Serah Terima' : 'Serah Terima')" v-if="canOpenReceiving" @click="openReceiving()" />
          <AppActionButton icon="cancel" label="Batalkan" v-if="canCancel" @click="confirmDetailAction('cancel')" />
        </div>
        <AppButton variant="secondary" @click="parentDetailId ? backToParent() : (showDetail = false)">{{ parentDetailId ? '← Kembali' : 'Tutup' }}</AppButton>
      </template>
    </AppModal>

    <!-- Edit Final Price Modal -->
    <AppModal v-model="showEditFinal" title="Jumlah & Harga Pembelian (Purchasing)" size="2xl">
      <AppAlert type="error" :message="editFinalError" />
      <p class="mb-3 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600">
        Isi jumlah aktual yang dibeli dan harga satuannya. Jumlah pengajuan tetap tersimpan sebagai pembanding.
        Nama, satuan, HPS, serta susunan item mengikuti pengajuan yang disetujui.
        <span v-if="!canAdjustPurchaseQuantity" class="mt-1 block text-amber-700">Jumlah pembelian terkunci setelah ada penerimaan atau pembayaran.</span>
      </p>
      <div class="mb-4 bg-emerald-50 p-3 sm:p-4 rounded-xl border border-emerald-100">
        <label class="text-sm font-bold text-emerald-900 mb-1.5 block">Vendor / Supplier Utama</label>
        <SearchableSelect v-model="editFinalVendorId"
          class="w-full rounded-lg border border-emerald-200 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
          <option value="">— Pilih Vendor —</option>
          <option v-for="v in vendorList" :key="v.id" :value="v.id">{{ v.name }}</option>
        </SearchableSelect>
      </div>
      <div class="mb-4 space-y-3">
        <div>
          <label class="text-sm font-medium text-gray-700 mb-1 block">No. Invoice / Struk Vendor</label>
          <input v-model="editFinalInvoice" type="text" placeholder="Masukkan nomor struk / invoice dari vendor"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500" />
        </div>
        <PhotoCapture multiple accept="image/*,.pdf" :required="false" v-model="editFinalInvoiceUrls"
          label="Foto / File Struk / Invoice / Kwitansi Vendor (opsional)"
          hint="1 struk/invoice per pembelian (bisa langsung pilih banyak foto/lembar atau PDF sekaligus, maks 10 berkas, 5MB/berkas)." />
      </div>
      <p class="text-sm text-gray-500 mb-3">Contoh: diajukan 10 kotak, tersedia 8 kotak — isi jumlah dibeli 8. Total harga dan penerimaan mengikuti jumlah dibeli.</p>
      <div class="space-y-4">
        <div v-for="(item, i) in editFinalItems" :key="i" class="bg-gray-50 rounded-lg p-3 border border-gray-200">
          <p class="text-sm font-semibold text-gray-900 mb-2">{{ item.name }}</p>
          <div class="space-y-2 sm:space-y-1.5">
            <div v-for="(sub, j) in item.items" :key="j"
              class="grid grid-cols-2 gap-2 rounded-lg border border-gray-200 bg-white p-2.5 sm:grid-cols-12 sm:items-center sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0">
              <div class="col-span-2 sm:col-span-4 min-w-0">
                <p class="break-words text-sm text-gray-700">{{ sub.name }}</p>
                <p class="text-xs text-gray-500">Diajukan: {{ sub.qty }} {{ sub.unit }}</p>
              </div>
              <label class="col-span-2 sm:col-span-3 min-w-0 text-xs text-gray-500">
                Jumlah dibeli ({{ sub.unit }})
                <input v-model.number="sub.purchased_qty" type="number" min="1" :max="sub.qty" step="1"
                  :disabled="!canAdjustPurchaseQuantity" :aria-label="`Jumlah dibeli ${sub.name}`"
                  class="input-sm mt-1 w-full disabled:bg-gray-100 disabled:text-gray-500" />
              </label>
              <div class="col-span-2 sm:col-span-3 min-w-0">
                <label class="mb-1 block text-xs text-gray-500">Harga/satuan</label>
                <RupiahInput v-model="sub.final_price" placeholder="Harga/satuan" />
              </div>
              <p class="col-span-2 self-center text-xs text-emerald-700 text-right font-semibold sm:col-span-2">{{ formatRupiah(purchaseQuantity(sub) * (sub.final_price || 0)) }}</p>
            </div>
          </div>
          <p class="text-xs text-right mt-2 text-gray-500">Harga pengadaan: <strong class="text-emerald-700">{{ formatRupiah(calcEditItemFinal(item)) }}</strong></p>
        </div>
      </div>
      <p class="mt-3 text-sm font-semibold text-gray-700">
        Total Harga: <span class="text-emerald-700">{{ formatRupiah(editFinalTotal) }}</span>
      </p>
      <div v-if="editFinalVendorId && !detail?.children?.length" class="mt-4 border-t border-gray-200 pt-4">
        <PurchasePayToForm v-model="editFinalPayToForm" :pay-to="editFinalPayToMeta" :error="editFinalPayToError" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showEditFinal = false">Batal</AppButton>
        <AppButton variant="secondary" :loading="savingFinal" @click="submitEditFinal(false)">Simpan Harga Saja</AppButton>
        <AppButton v-if="!detail?.children?.length" :loading="savingFinal" :disabled="!editFinalAllPriced" @click="submitEditFinal(true)">Simpan &amp; Ajukan Pembayaran</AppButton>
      </template>
    </AppModal>

    <!-- Split Vendor Modal -->
    <AppModal v-model="showSplitModal" :title="`Split ${typeLabel} ke Vendor Lain`" size="2xl">
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
              <div class="flex flex-wrap justify-between items-start gap-2">
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-bold text-gray-900 break-words">{{ sub.name }}</p>
                  <p class="text-[10px] text-gray-400 uppercase font-semibold break-words">{{ sub.groupName }}</p>
                </div>
                <div class="text-right shrink-0">
                  <p class="text-xs font-bold text-emerald-700">{{ formatRupiah(purchaseQuantity(sub) * (sub.final_price || 0)) }}</p>
                  <p class="text-[10px] text-gray-500">Dibeli {{ purchaseQuantity(sub) }} {{ sub.unit }} · Diajukan {{ sub.qty }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-gray-50 p-3 sm:p-4 rounded-xl border border-gray-200 space-y-3">
          <div>
            <label class="text-sm font-bold text-gray-700 mb-1.5 block">Vendor Tujuan</label>
            <SearchableSelect v-model="splitVendorId"
              class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
              <option value="">— Pilih Vendor Baru —</option>
              <option v-for="v in vendorList" :key="v.id" :value="v.id">{{ v.name }}</option>
            </SearchableSelect>
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
    <AppModal v-model="showActionConfirm" :title="actionConfirmTitle" :size="requestPayToNeeded ? 'lg' : 'sm'">
      <div class="flex items-start gap-3">
        <div :class="actionConfirmIconBg" class="shrink-0 w-10 h-10 rounded-full flex items-center justify-center">
          <svg v-if="pendingAction === 'approve'" class="w-5 h-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          <svg v-else-if="pendingAction === 'request_payment'" class="w-5 h-5 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          <svg v-else-if="pendingAction === 'cancel'" class="w-5 h-5 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
          <svg v-else class="w-5 h-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
        </div>
        <div class="min-w-0">
          <p class="text-sm text-gray-600">{{ actionConfirmMessage }}</p>
          <p class="text-xs text-gray-400 mt-1">Pengaju: <strong>{{ actionTarget?.requested_by || '-' }}</strong></p>
          <template v-if="requestPaymentCheck">
            <ul v-if="requestPaymentCheck.blockers.length" class="mt-2 space-y-1 rounded-lg border border-red-200 bg-red-50 p-2 text-[11px] text-red-800">
              <li v-for="(b, k) in requestPaymentCheck.blockers" :key="'b' + k" class="break-words">{{ b }}</li>
            </ul>
            <ul v-if="requestPaymentCheck.notes.length" class="mt-2 space-y-1 rounded-lg border border-amber-200 bg-amber-50 p-2 text-[11px] text-amber-800">
              <li v-for="(n, k) in requestPaymentCheck.notes" :key="'n' + k" class="break-words">{{ n }}</li>
            </ul>
          </template>
        </div>
      </div>
      <!-- Pembelian menetapkan tujuan pembayaran; Keuangan hanya membacanya. -->
      <div v-if="requestPayToNeeded && requestPaymentCheck?.ok" class="mt-4 border-t border-gray-100 pt-4">
        <PurchasePayToForm v-model="requestPayToForm" :pay-to="actionTarget?.pay_to" :error="requestPayToError" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showActionConfirm = false">{{ pendingAction === 'cancel' ? 'Kembali' : 'Batal' }}</AppButton>
        <AppButton :variant="pendingAction === 'cancel' ? 'danger' : 'primary'" :loading="actionLoading"
          :disabled="!!requestPaymentCheck && !requestPaymentCheck.ok" @click="submitAction">{{ actionConfirmBtn }}</AppButton>
      </template>
    </AppModal>

    <!-- Ubah / perbaiki tujuan pembayaran (Pembelian) selama tagihan belum lunas. -->
    <AppModal v-model="showPayToEdit" :title="payToReturnedDetail ? 'Perbaiki Detail Pembayaran' : 'Ubah Detail Pembayaran'" size="lg">
      <div class="space-y-3">
        <div v-if="payToReturnedDetail" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-800">
          <p class="font-semibold">Dikembalikan Keuangan: <span class="font-normal">{{ detail?.pay_to?.return_reason }}</span></p>
          <p class="mt-0.5 text-[11px] text-red-600">oleh {{ detail?.pay_to?.returned_by || '-' }}<template v-if="detail?.pay_to?.returned_at"> · {{ formatDateTime(detail.pay_to.returned_at) }}</template></p>
        </div>
        <p class="text-xs text-gray-500">Keuangan diberi tahu setelah disimpan. Form Bayar yang sedang dibuka Keuangan dengan detail lama akan diminta muat ulang.</p>
        <PurchasePayToForm v-model="payToEditForm" :pay-to="detail?.pay_to" :error="payToEditError" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showPayToEdit = false">Batal</AppButton>
        <AppButton :loading="payToSaving" @click="submitPayToEdit">Simpan &amp; Kirim ke Keuangan</AppButton>
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
    <!-- Serah Terima: membagi tiap baris belanja ke aset / stok gudang / habis pakai -->
    <ReceivingDialog v-model="showReceiving" :purchase-request-id="receivingPrId" @done="onReceived" />
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { purchaseApi } from '@/api/purchase.js'
import { apiClient } from '@/api/client.js'
import { safeUrl, safeUrls } from '@/utils/safeUrl.js'
import { workUnitsApi } from '@/api/workUnits.js'
import { vendorsApi } from '@/api/vendors.js'
import { formatRupiah, formatDateTime, formatDateStr } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
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
import ReceivingDialog from '@/components/ReceivingDialog.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'
import { logisticsApi } from '@/api/logistics.js'
import { deliveryStatusLabel, deliveryStatusClass, dispatchModeLabel, printDeliveryNote } from '@/utils/logisticsPrint.js'
import PurchasePayToForm from '@/components/PurchasePayToForm.vue'
import PurchasePayToCard from '@/components/PurchasePayToCard.vue'
import PurchaseUrgencyBadges from '@/components/PurchaseUrgencyBadges.vue'
import PurchaseUrgencyFields from '@/components/PurchaseUrgencyFields.vue'
import EmergencyPurchaseDialog from '@/components/EmergencyPurchaseDialog.vue'
import EmergencyPurchasePanel from '@/components/EmergencyPurchasePanel.vue'
import EmergencySettingsDialog from '@/components/EmergencySettingsDialog.vue'
import { stockItemsApi } from '@/api/warehouse.js'
import {
  purchaseStatusLabel, purchaseStatusBadge, purchaseStatusOptions,
  VERIFY_REASON_MIN, reasonLength, isConflictError,
  requestPaymentReadiness,
  RECEIVABLE_PURCHASE_STATUSES, isPhysicalPurchaseType, purchaseDisplayNames,
  newPayToForm, payToFormError, payToPayload, payToReturned, payToEditState,
  urgencyLabel, URGENT_NOTES_MIN,
  extractSourceUrl, formatSourceLabel,
} from '@/utils/procurement.js'

const toast = useToastStore()
const authStore = useAuthStore()


const logisticsDeliveriesOfDetail = computed(() => detail.value?.deliveries || [])
function deliveryChipClass(status) {
  const map = {
    draft: 'bg-gray-50 text-gray-700 ring-gray-200',
    in_transit: 'bg-amber-50 text-amber-800 ring-amber-200',
    partial_received: 'bg-blue-50 text-blue-700 ring-blue-200',
    received: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  }
  return map[status] || 'bg-gray-50 text-gray-600 ring-gray-200'
}
async function quickPrintDelivery(deliveryId) {
  try {
    const d = await logisticsApi.get(deliveryId)
    const r = printDeliveryNote(d)
    if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
  } catch (err) {
    toast.error(err?.message || 'Gagal mencetak Surat Jalan')
  }
}
const route = useRoute()

// Satu halaman, dua jenis: /purchase-goods (barang) dan /purchase-materials
// (bahan baku). Jenis dibaca dari meta rute; komponen yang sama dipakai ulang
// saat berpindah di antara keduanya, jadi semua yang bergantung pada jenis
// harus computed — lihat watch(requestType) di bawah.
const requestType = computed(() => route.meta?.requestType || 'barang')
const isBahan = computed(() => requestType.value === 'bahan')
const typeLabel = computed(() => isBahan.value ? 'Bahan Baku' : 'Barang')
const emptyListText = computed(() => isBahan.value ? 'Belum ada pengajuan bahan baku.' : 'Belum ada pengajuan barang.')

// Kunci izin mengikuti jenis: bahan baku diatur procurement.rawmaterials.*,
// barang (dan jasa) diatur procurement.requests.*. Aksi baru (mis. 'verify')
// cukup dipanggil perm('verify') tanpa mengubah helper ini.
function perm(action) {
  return (isBahan.value ? 'procurement.rawmaterials.' : 'procurement.requests.') + action
}
const canVerify  = computed(() => authStore.hasPermission(perm('verify')))
const canApprove = computed(() => authStore.hasPermission(perm('approve')))

// ── Tingkat kebutuhan & Pembelian Darurat (keputusan owner 19 Nov 2026) ──
// Pembelian Darurat hanya barang/jasa: dibeli dulu dari kas laci outlet lewat
// tugas Bayar Pengadaan di tablet kasir, disahkan penyetuju sesudahnya.
const canEmergency = computed(() => !isBahan.value && authStore.hasPermission('procurement.requests.emergency'))
// Sisi outlet (unggah struk, batalkan sebelum kas keluar, kirim ulang).
const canEmergencySide = computed(() => authStore.hasPermission('procurement.requests.emergency') || authStore.hasPermission('procurement.requests.submit'))
const showEmergency = ref(false)
const showEmergencySettings = ref(false)
const emergencyMax = ref(500000)
const filterUrgency = ref('')
const URGENCY_FILTERS = [
  { value: '', label: 'Semua Tingkat' },
  { value: 'urgent', label: 'Urgent' },
  { value: 'prioritas', label: 'Prioritas' },
  { value: 'rutin', label: 'Rutin' },
]
// Kas darurat diambil dari laci kasir outlet: hanya unit kerja ber-outlet.
const emergencyUnits = computed(() => workUnits.value.filter(u => u.outlet_id))
function openEmergency() { showEmergency.value = true }
function switchToEmergency() { showCreate.value = false; showEmergency.value = true }
function onEmergencyCreated(pr) {
  fetchList()
  if (pr?.id) viewDetail(pr)
}
async function afterEmergencyChange() {
  await reloadDetail()
  fetchList()
}
async function loadEmergencyMax() {
  if (isBahan.value) return
  try {
    const s = await purchaseApi.getEmergencySettings()
    emergencyMax.value = Number(s?.max_amount) || 500000
  } catch { /* bawaan Rp500.000 tetap dipakai untuk petunjuk */ }
}
// Serah terima satu pintu sesuai wewenang: bagian Aset (assets.create — baris
// peralatan) atau Gudang (stockledger.adjust — baris stok).
const canReceive = computed(() => authStore.hasPermission('stockledger.adjust')
  || (!isBahan.value && authStore.hasPermission('assets.create')))
const canViewGroupLogistics = computed(() => {
  if (authStore.isSuperadmin || authStore.hasPermission('logistics.deliveries.view') || authStore.hasPermission('logistics.deliveries.manage')) return true
  if (isBahan.value) {
    return authStore.hasPermission('logistics.rawmaterials.view') ||
      authStore.hasPermission('logistics.rawmaterials.manage') ||
      authStore.hasPermission('procurement.rawmaterials.purchasing') ||
      (authStore.scopeType !== 'specific' && authStore.hasPermission('stockledger.adjust'))
  }
  return authStore.hasPermission('logistics.goods.view') ||
    authStore.hasPermission('logistics.goods.manage') ||
    authStore.hasPermission('procurement.requests.purchasing') ||
    authStore.hasPermission('assets.create')
})
// Tombol Serah Terima hanya tampil bila masih ada barang yang belum diterima.
const canOpenReceiving = computed(() => {
  const d = detail.value
  if (!d || !canReceive.value || !isPhysicalPurchaseType(d.request_type || requestType.value)) return false
  if (!RECEIVABLE_PURCHASE_STATUSES.includes(d.status)) return false
  return d.receipt_status !== 'received'
})
const canShowApprovePanel = computed(() => {
  const d = detail.value
  if (!d || !canApprove.value) return false
  return d.status === 'verified' || (d.status === 'pending' && d.urgency === 'urgent')
})
const approveDirectCash = ref(false)
const approvePayoutRecipient = ref('')
const approvePayoutNotes = ref('')

// Nama item di katalog stok = barang dapur (diterima di Gudang Induk). Dipakai
// untuk memperingatkan pengaju SEBELUM disimpan (pengadaan barang), dan sebagai
// saran nama + satuan di form pengadaan bahan baku; server tetap yang memutuskan.
const stockItems = ref([])
const stockCatalogFailed = ref(false)
const stockCatalogLoading = ref(false)
const stockWarehouses = ref([])
const stockWarehouseId = ref('')
const stockAsOf = ref('')
const selectedStockWarehouse = computed(() => stockWarehouses.value.find(w => w.id === stockWarehouseId.value))
const stockItemNames = computed(() => new Set(stockItems.value.map(r => r.key)))
const stockItemByName = computed(() => Object.fromEntries(stockItems.value.map(r => [r.key, r])))
const stockItemOptions = computed(() => stockItems.value.filter(r => r.active).map(r => ({
  ...r, label: `${r.name}${r.code ? ` · ${r.code}` : ''}${r.unit ? ` (${r.unit})` : ''}`,
})))
let stockCatalogSeq = 0
async function loadStockItemNames() {
  const seq = ++stockCatalogSeq
  const withStock = isBahan.value
  stockCatalogLoading.value = true
  try {
    const d = await stockItemsApi.names(withStock ? { include_stock: 1 } : {})
    if (seq !== stockCatalogSeq) return
    const rows = Array.isArray(d) ? d : (d?.items || d?.data || [])
    stockCatalogFailed.value = false
    stockWarehouses.value = withStock ? (d?.warehouses || []) : []
    stockAsOf.value = d?.as_of || ''
    if (!stockWarehouses.value.some(w => w.id === stockWarehouseId.value)) {
      stockWarehouseId.value = stockWarehouses.value.length === 1 ? stockWarehouses.value[0].id : ''
    }
    stockItems.value = rows
      .filter(r => String(r.name || '').trim())
      .map(r => ({
        ...r, name: String(r.name).trim(), key: String(r.name).trim().toLowerCase(),
        active: r.is_active !== false, unit: r.unit || '',
      }))
  } catch {
    if (seq !== stockCatalogSeq) return
    stockItems.value = []; stockWarehouses.value = []; stockAsOf.value = ''
    stockCatalogFailed.value = true
  } finally { if (seq === stockCatalogSeq) stockCatalogLoading.value = false }
}
function applyCatalogItem(sub) {
  const it = stockItemByName.value[String(sub.name || '').trim().toLowerCase()]
  if (it?.unit) sub.unit = it.unit
  if (it && Number(it.last_purchase_price) > 0) {
    sub.hps_price = Math.round(Number(it.last_purchase_price))
  }
}
function lastPriceHint(sub) {
  const it = stockItemByName.value[String(sub.name || '').trim().toLowerCase()]
  if (!it || !(Number(it.last_purchase_price) > 0)) return ''
  const src = it.last_purchase_source
    ? ` (${it.last_purchase_source}${it.last_purchase_date ? ' ' + it.last_purchase_date : ''})`
    : ''
  return `Terakhir ${formatRupiah(it.last_purchase_price)}${src}`
}
function stockBalanceLabel(sub) {
  const it = stockItemByName.value[String(sub.name || '').trim().toLowerCase()]
  const warehouse = selectedStockWarehouse.value
  if (!it || !warehouse) return 'Pilih Gudang Induk untuk melihat stok terakhir.'
  const baseQty = Number(it.stock_by_warehouse?.[warehouse.id] || 0)
  const formatQty = value => new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(value)
  const baseLabel = `${formatQty(baseQty)} ${it.base_unit}`
  const ratio = Number(it.purchase_ratio)
  const qtyLabel = it.unit && it.unit !== it.base_unit && ratio > 0
    ? `${formatQty(baseQty / ratio)} ${it.unit} (${baseLabel})` : baseLabel
  return `Stok ${warehouse.name}: ${qtyLabel}`
}
function purchaseQuantity(sub) {
  return Number(sub.purchased_qty ?? sub.qty) || 0
}

const mixedKind = computed(() => {
  // Bahan baku seluruhnya diterima di gudang — tidak ada campuran dapur/peralatan.
  if (isBahan.value || !stockItemNames.value.size) return null
  const dapur = [], perlengkapan = []
  for (const entry of form.value.items || []) {
    for (const sub of entry.items || []) {
      const nm = String(sub.name || '').trim()
      if (!nm) continue
      if (stockItemNames.value.has(nm.toLowerCase())) dapur.push(nm)
      else perlengkapan.push(nm)
    }
  }
  return dapur.length && perlengkapan.length ? { dapur, perlengkapan } : null
})

const showReceiving = ref(false)
const receivingPrId = ref('')
function openReceiving() {
  receivingPrId.value = detail.value?.id || ''
  showReceiving.value = true
}
async function onReceived() {
  showDetail.value = false
  await fetchList()
}

// HPS terlihat oleh pengaju, verifikator, dan penyetuju; disembunyikan dari tim purchasing
const showHps = computed(() => authStore.hasPermission(perm('submit')) || canVerify.value || canApprove.value)

const loading = ref(false)
const errorMsg = ref('')
const requests = ref([])
// Unit kerja dalam cakupan role (/my-work-units) — filter daftar sekaligus
// pilihan unit pengaju di form.
const workUnits = ref([])
const workUnitsFailed = ref(false)
const vendorList = ref([])
const page = ref(1)
const totalPages = ref(1)
const filterWorkUnit = ref('')
const filterStatus = ref('')
// Unit yang ditugaskan langsung ke akun ini (work_units.admin_id) — hanya
// dipakai sebagai pilihan bawaan; banyak akun tidak punya, jadi bisa null.
const myWorkUnit = ref(null)

const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const formErrors = ref({})
const form = ref(emptyForm())
function clearFieldError(key) {
  if (!formErrors.value[key]) return
  const next = { ...formErrors.value }
  delete next[key]
  formErrors.value = next
}

const showDetail = ref(false)
const detail = ref(null)
const detailItems = ref([])
const parentDetailId = ref(null)
const savingDetailQty = ref(false)
const detailRejectMode = ref(false)
const detailRejectReason = ref('')

// Isi pengajuan hanya boleh diubah pengaju selama menunggu verifikasi; sejak
// diverifikasi isinya terkunci (server menolak dengan 400). Bahan baku tanpa
// verifikasi: masih boleh diubah selama menunggu persetujuan.
function canEditStatus(st) {
  return (st === 'pending' || (isBahan.value && st === 'verified')) && authStore.hasPermission(perm('submit'))
}
const canEditItems = computed(() => canEditStatus(detail.value?.status))
// Penyetuju dapat menyesuaikan qty item pada tahap persetujuan (verified, atau pending urgent).
const canApproveEditQty = computed(() => canShowApprovePanel.value && !canEditItems.value)
const canEditDetailQty = computed(() => canEditItems.value || canApproveEditQty.value)
const canAdjustPurchaseQuantity = computed(() => {
  const d = detail.value
  return d?.status === 'approved' && authStore.hasPermission(perm('purchasing'))
    && !d.receipt_status && !(d.paid_amount > 0) && !(d.struks || []).length
})
// Batal: sebelum ajukan bayar dan sebelum ada barang diterima.
const canCancel = computed(() => {
  const d = detail.value
  if (!d || !authStore.hasPermission(perm('submit'))) return false
  if (d.status === 'approved') return !d.receipt_status
  return d.status === 'pending' || d.status === 'verified'
})

// ── Verifikasi per item ──
// Baris disusun dari `items` tersimpan (bukan salinan yang sedang diedit),
// karena indeks group/item yang dikirim harus menunjuk isi di server.
const verifyRows = ref([])
const verifyNotes = ref('')
const verifyRejected = computed(() => verifyRows.value.filter(r => r.reject))
const verifyAllRejected = computed(() => verifyRows.value.length > 0 && verifyRejected.value.length === verifyRows.value.length)
function resetVerify() {
  const d = detail.value
  verifyNotes.value = ''
  // Mode "tolak seluruh pengajuan" dipakai panel verifikasi & persetujuan —
  // jangan terbawa ke dokumen/versi/status lain.
  detailRejectMode.value = false
  detailRejectReason.value = ''
  verifyRows.value = d?.status === 'pending'
    ? (d.items || []).flatMap((g, gi) => (g.items || []).map((sub, si) => ({
        key: `${gi}:${si}`, group: gi, item: si, groupName: g.name || '',
        name: sub.name, qty: sub.qty, unit: sub.unit, hps_price: sub.hps_price || 0,
        reject: false, reason: '',
      })))
    : []
}
// Disusun ulang setiap kali dokumen lain dibuka atau isinya berubah (versi baru).
watch(() => detail.value ? `${detail.value.id}|${detail.value.version || detail.value.updated_at || ''}|${detail.value.status}` : '',
  resetVerify, { immediate: true })

// Item yang masih dipegang pengajuan ini. Sejak split benar-benar memindahkan
// item ke pecahan, `items` dari server sudah berisi sisa yang tepat — dulu di
// sini item pecahan disaring lagi berdasarkan nama, sehingga dua baris bernama
// sama dalam satu grup ikut hilang semuanya.
// Sumber HPS boleh berupa tautan marketplace atau sekadar catatan "telepon
// toko". Tautan ditampilkan sebagai nama situsnya saja supaya kolom tidak
// melebar oleh URL panjang, tapi tetap bisa diklik dan di-hover untuk melihat
// alamat penuhnya.
// extractSourceUrl & formatSourceLabel diimpor dari @/utils/procurement.js

function getUnsplitItems(d) {
  return (d.items || []).map(it => ({
    ...it,
    selected: false,
    items: (it.items || []).map(sub => ({ ...sub })),
  }))
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
  cancel:          { title: 'Batalkan Pengajuan',     msg: 'Batalkan pengajuan ini? Pengajuan yang dibatalkan tidak bisa diproses lagi.', btn: 'Batalkan Pengajuan', bg: 'bg-red-100', done: 'dibatalkan' },
}
const actionConfirmTitle   = computed(() => ACTION_META[pendingAction.value]?.title ?? '')
const actionConfirmMessage = computed(() => {
  if (pendingAction.value === 'cancel' && actionTarget.value?.status === 'approved') {
    return 'Pengajuan ini sudah disetujui. Membatalkannya menghentikan proses pembelian — lanjutkan?'
  }
  return ACTION_META[pendingAction.value]?.msg ?? ''
})
const actionConfirmBtn     = computed(() => ACTION_META[pendingAction.value]?.btn ?? 'OK')
const actionConfirmIconBg  = computed(() => ACTION_META[pendingAction.value]?.bg ?? 'bg-gray-100')

// Ajukan Pembayaran DITOLAK server selama ada item tanpa harga final
// (dokumen tunggal, pecahan, atau sisa item induk) — dulu item tanpa harga
// "ikut tertutup" saat lunas. Tombolnya nonaktif dengan alasan yang jelas; saat
// induk diajukan hanya pecahan yang berharga lengkap ikut, pecahan lain
// diajukan dari detailnya (utils/procurement.js → requestPaymentReadiness).
const paymentReadiness = computed(() => requestPaymentReadiness(detail.value))
const requestPaymentCheck = computed(() =>
  pendingAction.value === 'request_payment' && actionTarget.value ? requestPaymentReadiness(actionTarget.value) : null)

// Tujuan pembayaran (keputusan owner 19 Nov 2026): Pembelian menetapkannya
// saat mengajukan pembayaran dan boleh mengubahnya selama tagihan belum lunas;
// Keuangan hanya membaca (utils/procurement.js → payTo*).
// Induk tanpa sisa item hanya cangkang — pecahannya ke rekening vendornya.
const requestPayToNeeded = computed(() => pendingAction.value === 'request_payment' &&
  (actionTarget.value?.items || []).some(g => (g?.items || []).length > 0))
const requestPayToForm = ref(newPayToForm())
const requestPayToError = ref('')
const payToEdit = computed(() => payToEditState(detail.value))
const canPurchasingPayTo = computed(() => payToEdit.value.show && authStore.hasPermission(perm('purchasing')))
const payToReturnedDetail = computed(() => payToReturned(detail.value))
const detailRemaining = computed(() =>
  Math.max(0, Number(detail.value?.total_final || 0) - Number(detail.value?.paid_amount || 0)))
const showPayToEdit = ref(false)
const payToEditForm = ref(newPayToForm())
const payToEditError = ref('')
const payToSaving = ref(false)

const showEditFinal = ref(false)
const editFinalVendorId = ref('')
const editFinalInvoice = ref('')
const editFinalItems = ref([])
const editFinalError = ref('')
const savingFinal = ref(false)
const editFinalTotal = computed(() =>
  editFinalItems.value.reduce((sum, it) => sum + (it.items || []).reduce((s, sub) => s + purchaseQuantity(sub) * (sub.final_price || 0), 0), 0)
)
const editFinalAllPriced = computed(() =>
  !!editFinalVendorId.value && editFinalItems.value.length > 0 &&
  editFinalItems.value.every(g => (g.items || []).length > 0 && (g.items || []).every(sub => Number(sub.final_price) > 0))
)
const editFinalPayToForm = ref(newPayToForm())
const editFinalPayToError = ref('')
const editFinalPayToMeta = computed(() => {
  const v = vendorList.value.find(x => x.id === editFinalVendorId.value)
  return {
    ...(detail.value?.pay_to || {}),
    vendor_bank_name: v?.bank_name || '',
    vendor_account_number: v?.account_number || '',
    vendor_account_holder: v?.account_holder || '',
  }
})
watch(editFinalVendorId, () => {
  editFinalPayToError.value = ''
  editFinalPayToForm.value = newPayToForm(editFinalPayToMeta.value)
})

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
  splitItemsFlattened.value.filter(it => it.selected).reduce((sum, it) => sum + purchaseQuantity(it) * (it.final_price || 0), 0)
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

// Bahan baku tidak pernah 'Menunggu Verifikasi'.
const STATUS_OPTIONS = computed(() => purchaseStatusOptions().filter(s => !(isBahan.value && s.value === 'pending')))

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

const estimatedTotal = computed(() =>
  form.value.items.reduce((sum, it) => sum + (it.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.hps_price || 0), 0), 0)
)
function calcItemHps(item) {
  return (item.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.hps_price || 0), 0)
}
function calcEditItemFinal(item) {
  return (item.items || []).reduce((s, sub) => s + purchaseQuantity(sub) * (sub.final_price || 0), 0)
}

// Status tak dikenal tampil apa adanya dengan warna netral (lihat utils/procurement.js).
function statusBadge(s) { return purchaseStatusBadge(s) }
function statusLabel(s) { return purchaseStatusLabel(s) }
// Sama dengan aturan server (DeletePurchaseRequest): dokumen yang sudah ada
// pembayaran atau penerimaannya tidak bisa dihapus; selain admin/superadmin
// hanya boleh pending/rejected/cancelled.
function canDelete(row) {
  if ((row.paid_amount || 0) > 0 || row.receipt_status) return false
  const role = authStore.currentAdmin?.role
  if (role === 'admin' || role === 'superadmin') return !['paid', 'received'].includes(row.status)
  return ['pending', 'rejected', 'cancelled'].includes(row.status)
}
// Sisa tagihan yang masih harus dibayar (dibayar sebagian, termasuk yang
// barangnya sudah diterima sebelum lunas). Dihitung dalam sen seperti halaman
// Pembayaran supaya selisih float tidak tampil sebagai "Sisa: Rp 0".
//
// Induk hasil pecah vendor TIDAK diberi label: di daftar ini totalnya =
// milik induk + semua pecahan, sedangkan paid_amount hanya milik induk
// sendiri — selisihnya kebesaran sebesar pembayaran pecahan (mis. tampil
// Rp 1.010.000 padahal sisa keluarganya Rp 450.000). Pembayaran pecahan tidak
// ikut di respons daftar; sisa per dokumen ada di halaman Pembayaran.
function remainingDebt(row) {
  if (row.split_status === 'master') return 0
  if (!['partial', 'received'].includes(row.status) || !(row.paid_amount > 0)) return 0
  const cents = Math.round(((Number(row.total_final || row.total_amount) || 0) - (Number(row.paid_amount) || 0)) * 100)
  return cents > 0 ? cents / 100 : 0
}
function emptyForm() {
  return {
    outlet_id: '', work_unit_id: '', requested_by: '', vendor_id: '', vendor_name: '', project_id: '',
    items: [{ name: '', items: [{ name: '', qty: 1, unit: 'pcs', hps_price: 0, hps_source: '', rab_item_id: '' }] }],
    notes: '',
    urgency: 'rutin', need_by_date: '',
  }
}
function addItem() {
  form.value.items.push({ name: '', items: [{ name: '', qty: 1, unit: 'pcs', hps_price: 0, hps_source: '', rab_item_id: '' }] })
}
function removeItem(i) { if (form.value.items.length > 1) { form.value.items.splice(i, 1); formErrors.value = {} } }
function addSubItem(i) { form.value.items[i].items.push({ name: '', qty: 1, unit: 'pcs', hps_price: 0, hps_source: '', rab_item_id: '' }) }
function removeSubItem(i, j) { if (form.value.items[i].items.length > 1) { form.value.items[i].items.splice(j, 1); formErrors.value = {} } }
function adminName() { return authStore.admin?.name || 'Admin' }

const projects = ref([])

// Hanya projek yang masih bisa dibelanjai yang boleh dipilih; yang sudah
// selesai/batal tetap memegang pengajuan lamanya tapi tidak menerima yang baru.
const projectOptions = computed(() => [
  { id: '', name: 'Tanpa projek' },
  ...projects.value
    .filter(p => p.status === 'berjalan' || p.status === 'draft')
    .map(p => {
      const base = p.project_number ? `${p.name} — ${p.project_number}` : p.name
      const tag = p.rab_status === 'ditetapkan' ? '' : ' (RAB belum ditetapkan)'
      return { id: p.id, name: base + tag }
    }),
])

// ── Baris RAB projek yang dipilih di form ──
// Pengajuan barang hanya boleh menyerap baris barang/umum; jasa → jasa/umum.
const isProjectMode = computed(() => !isBahan.value && !!form.value.project_id)
const selectedProject = computed(() => projects.value.find(p => p.id === form.value.project_id) || null)
const projectRab = ref(null)
const rabLines = computed(() => (projectRab.value?.items || []).filter(l => l.kind === 'umum' || l.kind === requestType.value))
const rabById = computed(() => Object.fromEntries((projectRab.value?.items || []).map(l => [l.id, l])))
const rabNotSet = computed(() => !!form.value.project_id && !!projectRab.value && projectRab.value.rab_status !== 'ditetapkan')
async function loadProjectRab(id) {
  projectRab.value = null
  if (!id) return
  try { projectRab.value = await projectsApi.rab(id) } catch { projectRab.value = null }
}
watch(() => form.value.project_id, (id, oldId) => {
  loadProjectRab(id)
  if (id !== oldId) {
    clearFieldError('project_id')
    for (const it of form.value.items || []) {
      for (const sub of it.items || []) {
        sub.rab_item_id = ''
        if (sub._fromRab) {
          sub.name = ''
          sub.unit = 'pcs'
          sub.hps_price = 0
          sub.hps_source = ''
          sub._fromRab = false
        } else if (String(sub.hps_source || '').startsWith('RAB ')) {
          sub.hps_source = ''
        }
      }
    }
  }
})
function autoGroupName(item) {
  const sections = []
  for (const sub of item?.items || []) {
    const l = sub.rab_item_id ? rabById.value[sub.rab_item_id] : null
    const sec = String(l?.section || '').trim()
    if (sec && !sections.includes(sec)) sections.push(sec)
  }
  if (sections.length) return sections.join(' · ').slice(0, 200)
  const projName = String(selectedProject.value?.name || projectRab.value?.project_name || '').trim()
  if (projName) return projName.slice(0, 200)
  const firstSub = String((item?.items || []).find(s => String(s.name || '').trim())?.name || '').trim()
  return firstSub.slice(0, 200)
}
function rabOptionLabel(l) {
  return `${l.section ? l.section + ' / ' : ''}${l.name} · sisa ${formatRupiah(l.remaining)}${l.unit ? ` (${l.qty} ${l.unit} @ ${formatRupiah(l.unit_price)})` : ''}`
}
// Memilih / mengganti baris RAB selalu memperbarui nama, satuan, HPS satuan, dan sumber HPS.
function onRabSelect(sub, val, groupIdx, subIdx) {
  sub.rab_item_id = val || ''
  const l = rabById.value[sub.rab_item_id]
  if (!l) {
    if (sub._fromRab) {
      sub.name = ''
      sub.unit = 'pcs'
      sub.hps_price = 0
      sub.hps_source = ''
      sub._fromRab = false
    } else if (String(sub.hps_source || '').startsWith('RAB ')) {
      sub.hps_source = ''
    }
    return
  }
  sub.name = l.name
  sub.unit = l.unit || ''
  sub.hps_price = Number(l.unit_price) || 0
  sub.hps_source = `RAB ${projectRab.value.project_number} v${projectRab.value.rab_version}: ${l.section ? l.section + ' / ' : ''}${l.name}`
  sub._fromRab = true
  if (groupIdx != null && subIdx != null) {
    clearFieldError(`sub_name:${groupIdx}:${subIdx}`)
    clearFieldError(`sub_hps:${groupIdx}:${subIdx}`)
  }
}
function rabWarn(sub) {
  const l = rabById.value[sub.rab_item_id]
  if (!l) return ''
  const v = (sub.qty || 0) * (sub.hps_price || 0)
  return v > l.remaining ? `Melebihi sisa baris RAB (${formatRupiah(l.remaining)}) — periksa volume/HPS atau revisi RAB.` : ''
}

// Nama & sisa baris RAB untuk detail pengajuan (projek detail bisa berbeda dari
// form). Dimuat ulang setiap kali detail dimuat — bukan hanya saat projeknya
// berganti: sisa baris berubah begitu pengajuan lain disetujui, dan peringatan
// persetujuan di bawah memakai sisa itu. Dulu penyetuju yang menyetujui
// beberapa pengajuan satu projek berturut-turut melihat sisa basi, sehingga
// pengajuan berikutnya yang melampaui RAB lolos tanpa peringatan.
const detailRabById = ref({})
let detailRabSeq = 0
watch(detail, async (d, old) => {
  const id = d?.project_id
  const seq = ++detailRabSeq
  if (id !== old?.project_id) detailRabById.value = {}
  if (!id) return
  try {
    const r = await projectsApi.rab(id)
    if (seq === detailRabSeq) detailRabById.value = Object.fromEntries((r?.items || []).map(l => [l.id, l]))
  } catch { if (seq === detailRabSeq) detailRabById.value = {} }
})
function rabName(id) {
  const l = detailRabById.value[id]
  return l ? `${l.section ? l.section + ' / ' : ''}${l.name}` : 'baris RAB'
}

// Anggaran projek baru terpakai saat pengajuan disetujui (keputusan owner
// 14 Okt 2026), jadi penyetuju diberi tahu bila persetujuan ini melampaui sisa
// baris RAB. Sisa dari server hanya menghitung pengajuan yang sudah disetujui;
// nilai item = harga final bila sudah ada, selain itu HPS (sama dengan server).
const approvalRabWarnings = computed(() => {
  const d = detail.value
  if (!d?.project_id || !canShowApprovePanel.value) return []
  const byLine = {}
  const srcGroups = canEditDetailQty.value ? detailItems.value : (d.items || [])
  for (const g of srcGroups) {
    for (const sub of g.items || []) {
      if (!sub.rab_item_id) continue
      const price = sub.final_price > 0 ? sub.final_price : (sub.hps_price || 0)
      byLine[sub.rab_item_id] = (byLine[sub.rab_item_id] || 0) + (sub.qty || 0) * price
    }
  }
  return Object.entries(byLine)
    .map(([id, amount]) => {
      const l = detailRabById.value[id]
      if (!l || amount - (l.remaining || 0) < 0.5) return null
      return { id, name: rabName(id), remaining: l.remaining || 0, amount }
    })
    .filter(Boolean)
})

async function fetchProjects() {
  try {
    const data = await projectsApi.list()
    projects.value = Array.isArray(data) ? data : (data?.data || [])
  } catch { projects.value = [] }
}

onMounted(async () => {
  // Bahan baku tidak pernah memakai projek: daftar projek tidak dimuat
  // (role khusus bahan baku memang tidak boleh membacanya).
  await Promise.all([fetchList(), fetchWorkUnits(), fetchMyWorkUnit(), fetchVendors(), isBahan.value ? Promise.resolve() : fetchProjects(), loadStockItemNames(), loadEmergencyMax()])
  // Datang lewat tombol "Belanja Tahap Baru" di halaman Projek: buka form
  // pengajuan dengan projeknya sudah terpilih.
  const pid = String(route.query.project_id || '')
  if (pid && !isBahan.value) {
    openCreate()
    // Hanya projek yang memang bisa dipilih di form (ada di daftar projek
    // pengguna dan masih draft/berjalan); id lain dari URL diabaikan.
    if (projectOptions.value.some(o => o.id === pid)) form.value.project_id = pid
  }
})

async function fetchVendors() {
  try {
    const data = await vendorsApi.list({ active: 'true' })
    vendorList.value = Array.isArray(data) ? data : (data?.data || [])
  } catch { vendorList.value = [] }
}

// apiClient membuka amplop {success,data}: bila server membalas data:null
// (field dibuang omitempty), yang sampai ke sini adalah {success:true} — objek
// truthy tanpa id. Karena itu hasilnya dinormalisasi: unit sah harus ber-id.
async function fetchMyWorkUnit() {
  try {
    const d = await workUnitsApi.getMyWorkUnit()
    myWorkUnit.value = d && d.id ? d : null
  } catch { myWorkUnit.value = null }
}

async function fetchWorkUnits() {
  try {
    const data = await workUnitsApi.myWorkUnits()
    workUnits.value = (Array.isArray(data) ? data : (Array.isArray(data?.data) ? data.data : [])).filter(u => u && u.id)
    workUnitsFailed.value = false
  } catch { workUnits.value = []; workUnitsFailed.value = true }
}

const workUnitFilterOptions = computed(() => [{ id: '', name: 'Semua Unit Kerja' }, ...workUnits.value])
// Pilihan unit pengaju di form; nama outlet ditambahkan bila berbeda dari nama unit.
const unitOptions = computed(() => workUnits.value.map(u => ({
  id: u.id,
  name: u.outlet_name && u.outlet_name !== u.name ? `${u.name} — ${u.outlet_name}` : u.name,
})))

// Nomor urut permintaan daftar: respons yang datang terlambat (mis. daftar
// barang yang masih dimuat saat pengguna pindah ke bahan baku) dibuang.
let listSeq = 0
// Nomor urut pemuatan detail, sama seperti listSeq: detail yang datang setelah
// pengguna pindah jenis (atau membuka detail lain) tidak boleh membuka modal.
let detailSeq = 0
async function fetchList({ silent = false } = {}) {
  const seq = ++listSeq
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const params = { page: page.value, limit: 20, type: requestType.value }
    if (filterWorkUnit.value) params.work_unit_id = filterWorkUnit.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterUrgency.value === 'emergency') params.emergency = 'true'
    else if (filterUrgency.value === 'emergency_pending') params.ratification = 'pending'
    else if (filterUrgency.value) params.urgency = filterUrgency.value
    const data = await purchaseApi.list(params)
    if (seq !== listSeq) return
    requests.value = data.requests || []
    totalPages.value = data.total_pages || 1
  } catch (err) { if (seq === listSeq && !silent) errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { if (seq === listSeq && !silent) loading.value = false }
}

useRealtime(['procurement', 'procurement_payment', 'procurement_receiving', 'logistics_delivery'], async () => {
  await fetchList({ silent: true })
  if (!isBahan.value) {
    fetchProjects()
    if (showCreate.value && form.value.project_id) {
      const pid = form.value.project_id
      try {
        const freshRab = await projectsApi.rab(pid)
        if (showCreate.value && form.value.project_id === pid) projectRab.value = freshRab
      } catch {}
    }
  }
  if (showDetail.value && detail.value?.id && !detailRejectMode.value && !showEditFinal.value && !showSplitModal.value && !showReceiving.value) {
    const id = detail.value.id
    try {
      const d = await purchaseApi.get(id)
      if (showDetail.value && detail.value?.id === id && (detail.value.status !== 'pending' || d.version !== detail.value.version)) {
        detail.value = d
        detailItems.value = getUnsplitItems(d)
      }
    } catch {}
  }
})

const exportingPurchasing = ref(false)
async function downloadPurchasingExcel() {
  if (exportingPurchasing.value) return
  exportingPurchasing.value = true
  errorMsg.value = ''
  try {
    const params = { type: requestType.value, status: filterStatus.value, parent_id: 'all' }
    if (filterWorkUnit.value) params.work_unit_id = filterWorkUnit.value
    const blob = await purchaseApi.exportPurchasing(params)
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `Purchasing-${typeLabel.value.replaceAll(' ', '-')}-per-Vendor-${new Date().toLocaleDateString('en-CA')}.xlsx`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    errorMsg.value = err?.message || 'Gagal mengunduh Excel purchasing per vendor.'
  } finally { exportingPurchasing.value = false }
}

// Pindah /purchase-goods ↔ /purchase-materials memakai ulang komponen ini
// (onMounted tidak terpanggil lagi). Tutup semua dialog, kosongkan filter,
// halaman, detail, dan form jenis sebelumnya, lalu muat daftar jenis baru.
watch(requestType, () => {
  detailSeq++ // detail yang masih dimuat milik jenis sebelumnya — buang
  showCreate.value = false
  showDetail.value = false
  showEditFinal.value = false
  showSplitModal.value = false
  showActionConfirm.value = false
  showReject.value = false
  showDeleteConfirm.value = false
  showDeleteItemConfirm.value = false
  showReceiving.value = false
  showEmergency.value = false
  showEmergencySettings.value = false
  filterUrgency.value = ''
  detail.value = null
  detailItems.value = []
  parentDetailId.value = null
  detailRejectMode.value = false
  detailRejectReason.value = ''
  actionTarget.value = null
  rejectTarget.value = null
  deleteTarget.value = null
  form.value = emptyForm()
  createError.value = ''
  formErrors.value = {}
  filterWorkUnit.value = ''
  filterStatus.value = ''
  page.value = 1
  totalPages.value = 1
  requests.value = []
  projectRab.value = null
  fetchList()
  loadStockItemNames()
  if (!isBahan.value) {
    fetchProjects()
    loadEmergencyMax()
  }
})

function goPage(p) { page.value = p; fetchList() }

function openCreate() {
  form.value = emptyForm(); createError.value = ''; formErrors.value = {}; projectRab.value = null
  // Pengaju = akun yang login (server mengisinya dari sesi, bukan dari form).
  form.value.requested_by = authStore.admin?.name || authStore.admin?.username || myWorkUnit.value?.admin_name || ''
  // Satu unit → otomatis. Lebih dari satu → wajib dipilih; bawaannya unit yang
  // ditugaskan langsung ke akun ini bila ada di cakupannya.
  const units = workUnits.value
  if (units.length === 1) form.value.work_unit_id = units[0].id
  else if (myWorkUnit.value && units.some(u => u.id === myWorkUnit.value.id)) form.value.work_unit_id = myWorkUnit.value.id
  showCreate.value = true
  loadStockItemNames()
  if (!isBahan.value) fetchProjects()
}

async function submitCreate() {
  createError.value = ''
  formErrors.value = {}
  if (!workUnits.value.length) {
    createError.value = 'Anda belum memiliki unit kerja dalam cakupan — pengajuan tidak bisa dikirim.'
    return
  }

  const errors = {}
  const unit = workUnits.value.find(u => u.id === form.value.work_unit_id)
  if (!unit) {
    errors.work_unit_id = 'Unit Kerja Pengaju wajib dipilih.'
  }
  if (!isBahan.value && rabNotSet.value) {
    errors.project_id = `RAB projek ${projectRab.value?.project_number || ''} belum ditetapkan — tetapkan RAB di halaman Projek dulu.`
  }
  if (isBahan.value && (stockCatalogFailed.value || stockCatalogLoading.value)) {
    errors.catalog = 'Tunggu katalog bahan baku selesai dimuat, atau klik Perbarui stok.'
  }

  const payloadItems = []
  const multiGroup = form.value.items.length > 1
  form.value.items.forEach((it, i) => {
    const groupName = isProjectMode.value ? autoGroupName(it) : String(it.name || '').trim()
    if (!isProjectMode.value && !groupName) {
      errors[`group:${i}`] = multiGroup ? `Nama Pengadaan #${i + 1} wajib diisi.` : 'Nama Pengadaan wajib diisi.'
    }
    const subs = it.items || []
    if (!subs.length) {
      errors[`group:${i}`] = `Pengadaan #${i + 1} minimal harus memiliki 1 item.`
    }
    const cleanSubs = []
    subs.forEach((sub, j) => {
      const loc = multiGroup ? `Pengadaan #${i + 1}, Item #${j + 1}` : `Item #${j + 1}`
      const subName = String(sub.name || '').trim()
      if (!subName) {
        errors[`sub_name:${i}:${j}`] = `${loc}: ${isBahan.value ? 'Bahan baku' : 'Nama item'} wajib diisi.`
      } else if (isBahan.value && !stockCatalogFailed.value && !stockCatalogLoading.value && !stockItemByName.value[subName.toLowerCase()]?.active) {
        errors[`sub_name:${i}:${j}`] = `${loc}: Pilih bahan baku aktif dari pencarian katalog.`
      }
      const qty = Number(sub.qty)
      if (!Number.isFinite(qty) || qty <= 0) {
        errors[`sub_qty:${i}:${j}`] = `${loc}: Qty harus lebih dari 0.`
      }
      const hps = Number(sub.hps_price) || 0
      if (hps < 0) {
        errors[`sub_hps:${i}:${j}`] = `${loc}: HPS satuan tidak boleh negatif.`
      }
      cleanSubs.push({
        name: subName,
        qty: Number.isFinite(qty) ? qty : 0,
        unit: String(sub.unit || '').trim(),
        hps_price: hps,
        hps_source: String(sub.hps_source || '').trim(),
        rab_item_id: isBahan.value ? '' : String(sub.rab_item_id || ''),
      })
    })
    payloadItems.push({
      name: groupName || (cleanSubs[0]?.name || 'Pengadaan Projek'),
      items: cleanSubs,
    })
  })

  // Tingkat kebutuhan: aturan yang sama dengan server (normalizeUrgency).
  if (form.value.urgency !== 'rutin' && !form.value.need_by_date) {
    errors.need_by_date = `Tanggal dibutuhkan wajib diisi untuk pengajuan ${urgencyLabel(form.value.urgency).toUpperCase()}.`
  }
  if (form.value.urgency === 'urgent' && [...form.value.notes.trim()].length < URGENT_NOTES_MIN) {
    errors.notes = `Catatan / alasan wajib diisi minimal ${URGENT_NOTES_MIN} huruf untuk pengajuan URGENT.`
  }

  if (Object.keys(errors).length) {
    formErrors.value = errors
    createError.value = 'Mohon lengkapi bagian berikut: ' + Object.values(errors).join(' • ')
    await nextTick()
    document.querySelector('[data-form-error]')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    return
  }

  creating.value = true
  try {
    await purchaseApi.create({
      // Outlet mengikuti unit kerja (server juga menurunkannya dari unit).
      outlet_id: unit.outlet_id || '',
      work_unit_id: unit.id,
      // Bahan baku tidak boleh tertaut projek — kirim kosong walau form sempat terisi.
      project_id: isBahan.value ? '' : (form.value.project_id || ''),
      request_type: requestType.value,
      requested_by: form.value.requested_by.trim(),
      items: payloadItems,
      notes: form.value.notes.trim(),
      urgency: form.value.urgency,
      need_by_date: form.value.urgency === 'rutin' ? '' : form.value.need_by_date,
    })
    toast.success(`Pengajuan ${typeLabel.value.toLowerCase()} berhasil dikirim.`)
    showCreate.value = false; fetchList()
  } catch (err) { createError.value = err?.message ?? 'Gagal membuat pengajuan.' }
  finally { creating.value = false }
}

async function viewDetail(row) {
  const seq = ++detailSeq
  try {
    const d = await purchaseApi.get(row.id)
    if (seq !== detailSeq) return
    parentDetailId.value = null
    detail.value = d
    detailItems.value = getUnsplitItems(detail.value)
    detailRejectMode.value = false
    detailRejectReason.value = ''
    approveDirectCash.value = false
    approvePayoutRecipient.value = d?.requested_by || ''
    approvePayoutNotes.value = ''
    showDetail.value = true
  } catch { if (seq === detailSeq) toast.error('Gagal memuat detail.') }
}

async function viewChildDetail(child) {
  const seq = ++detailSeq
  try {
    const d = await purchaseApi.get(child.id)
    if (seq !== detailSeq) return
    // Set state setelah fetch sukses — bila gagal, modal tidak menampilkan
    // tombol "Kembali ke Pengajuan Induk" untuk detail yang tak termuat.
    parentDetailId.value = detail.value?.id || null
    detail.value = d
    detailItems.value = getUnsplitItems(d)
    detailRejectMode.value = false
    detailRejectReason.value = ''
  } catch { if (seq === detailSeq) toast.error('Gagal memuat detail.') }
}

async function backToParent() {
  if (!parentDetailId.value) return
  const seq = ++detailSeq
  try {
    const d = await purchaseApi.get(parentDetailId.value)
    if (seq !== detailSeq) return
    detail.value = d
    detailItems.value = getUnsplitItems(detail.value)
    parentDetailId.value = null
  } catch { if (seq === detailSeq) toast.error('Gagal memuat detail induk.') }
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
  saveDetailQty(`"${name}" berhasil dihapus.`)
}

// Muat ulang detail yang sedang terbuka dari server.
async function reloadDetail() {
  if (!detail.value?.id) return
  const seq = ++detailSeq
  try {
    const d = await purchaseApi.get(detail.value.id)
    if (seq !== detailSeq) return
    detail.value = d
    detailItems.value = getUnsplitItems(d)
  } catch { if (seq === detailSeq) toast.error('Gagal memuat ulang detail.') }
}

let _saveDetailQtyPromise = null

function hasUnsavedDetailQtyChanges() {
  const saved = detail.value?.items || []
  const cur = detailItems.value || []
  if (saved.length !== cur.length) return false
  for (let i = 0; i < saved.length; i++) {
    const sSubs = saved[i]?.items || []
    const cSubs = cur[i]?.items || []
    if (sSubs.length !== cSubs.length) return false
    for (let j = 0; j < sSubs.length; j++) {
      if (Number(sSubs[j]?.qty) !== Number(cSubs[j]?.qty)) return true
    }
  }
  return false
}

// Simpan perubahan item (qty / hapus / sumber HPS) selama pending atau penyesuaian
// qty oleh penyetuju pada tahap persetujuan. Berhasil atau gagal, layar dimuat ulang
// dari server — bila server menolak, angka yang tidak tersimpan tidak tetap tampil.
function saveDetailQty(successMsg = 'Perubahan qty tersimpan.') {
  if (!detail.value?.id) return Promise.resolve(false)
  for (const g of detailItems.value || []) {
    for (const sub of g.items || []) {
      const q = Number(sub.qty)
      if (!Number.isInteger(q) || q < 1) {
        toast.error(`Qty item "${sub.name}" harus bilangan bulat minimal 1.`)
        reloadDetail()
        return Promise.resolve(false)
      }
    }
  }
  const run = (async () => {
    savingDetailQty.value = true
    try {
      await purchaseApi.updateItems(detail.value.id, { items: detailItems.value, version: detail.value.version })
      toast.success(successMsg)
      fetchList()
      return true
    } catch (err) {
      toast.error(isConflictError(err)
        ? (err?.message || 'Pengajuan berubah') + ' — data terbaru dimuat ulang, ulangi perubahan bila masih perlu.'
        : (err?.message ?? 'Gagal menyimpan perubahan item.'))
      return false
    } finally {
      await reloadDetail()
      savingDetailQty.value = false
      if (_saveDetailQtyPromise === run) _saveDetailQtyPromise = null
    }
  })()
  _saveDetailQtyPromise = run
  return run
}

// 409 = isi pengajuan berubah sejak dibuka (mis. pengaju mengubah qty):
// muat ulang supaya keputusan diambil atas isi terbaru.
async function handleConflict(err, fallbackMsg) {
  if (isConflictError(err)) {
    await reloadDetail()
    toast.error((err?.message || 'Pengajuan berubah') + ' — data terbaru sudah dimuat, periksa lagi sebelum melanjutkan.')
    fetchList()
    return
  }
  toast.error(err?.message ?? fallbackMsg)
}

async function submitVerify() {
  const d = detail.value
  if (!d || actionLoading.value) return
  const rejected = verifyRejected.value
  const bad = rejected.find(r => reasonLength(r.reason) < VERIFY_REASON_MIN)
  if (bad) { toast.error(`Alasan menolak "${bad.name}" wajib diisi, minimal ${VERIFY_REASON_MIN} huruf.`); return }
  const allRejected = verifyAllRejected.value
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(d.id, {
      action: 'verify',
      actor_name: adminName(),
      version: d.version,
      notes: verifyNotes.value.trim(),
      rejected_items: rejected.map(r => ({ group: r.group, item: r.item, reason: r.reason.trim() })),
    })
    toast.success(allRejected
      ? 'Semua item ditolak — pengajuan berstatus Ditolak.'
      : rejected.length
        ? `Pengajuan diverifikasi (${rejected.length} item ditolak) dan diteruskan ke persetujuan.`
        : 'Pengajuan diverifikasi dan diteruskan ke persetujuan.')
    await reloadDetail()
    fetchList()
  } catch (err) { await handleConflict(err, 'Gagal memverifikasi pengajuan.') }
  finally { actionLoading.value = false }
}

async function submitDetailApprove() {
  if (!detail.value || actionLoading.value) return
  if (approveDirectCash.value && !approvePayoutRecipient.value.trim()) {
    toast.error('Nama penerima kas wajib diisi untuk diarahkan ke Kas Kasir Outlet.')
    return
  }
  if (_saveDetailQtyPromise) {
    const ok = await _saveDetailQtyPromise
    if (!ok) return
  } else if (canEditDetailQty.value && hasUnsavedDetailQtyChanges()) {
    const ok = await saveDetailQty('Perubahan qty tersimpan.')
    if (!ok) return
  }
  actionLoading.value = true
  try {
    const body = { action: 'approve', actor_name: adminName(), version: detail.value.version }
    if (approveDirectCash.value) {
      body.direct_cash_payout = true
      body.payout_recipient = approvePayoutRecipient.value.trim()
      body.payout_notes = approvePayoutNotes.value.trim()
    }
    await purchaseApi.updateStatus(detail.value.id, body)
    toast.success(approveDirectCash.value
      ? 'Pengajuan disetujui & tugas kas dikirim ke tablet kasir outlet.'
      : 'Pengajuan berhasil disetujui.')
    await reloadDetail()
    fetchList()
  } catch (err) { await handleConflict(err, 'Gagal menyetujui.') }
  finally { actionLoading.value = false }
}

// Dipakai verifikator (tolak seluruh pengajuan saat pending) maupun penyetuju
// (tolak saat verified) — payload sama, izinnya diperiksa server per status.
async function submitDetailReject() {
  if (!detailRejectReason.value.trim()) { toast.error('Alasan penolakan wajib diisi.'); return }
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(detail.value.id, { action: 'reject', actor_name: adminName(), rejected_reason: detailRejectReason.value.trim() })
    toast.success('Pengajuan ditolak.')
    detailRejectMode.value = false
    detailRejectReason.value = ''
    await reloadDetail()
    fetchList()
  } catch (err) { await handleConflict(err, 'Gagal menolak.') }
  finally { actionLoading.value = false }
}

function confirmDetailAction(action) {
  actionTarget.value = detail.value
  pendingAction.value = action
  if (action === 'request_payment') {
    requestPayToForm.value = newPayToForm(detail.value?.pay_to)
    requestPayToError.value = ''
  }
  showActionConfirm.value = true
}

function openPayToEdit() {
  if (!detail.value) return
  payToEditForm.value = newPayToForm(detail.value.pay_to)
  payToEditError.value = ''
  showPayToEdit.value = true
}

async function submitPayToEdit() {
  const target = detail.value
  if (!target || payToSaving.value) return
  const err = payToFormError(payToEditForm.value, target.pay_to)
  payToEditError.value = err
  if (err) return
  payToSaving.value = true
  try {
    await purchaseApi.updatePaymentDetails(target.id, { ...payToPayload(payToEditForm.value), version: target.version })
    toast.success('Detail pembayaran disimpan — Keuangan diberi tahu.')
    showPayToEdit.value = false
    await reloadDetail()
    fetchList()
  } catch (e) {
    if (isConflictError(e)) showPayToEdit.value = false
    await handleConflict(e, 'Gagal menyimpan detail pembayaran.')
  } finally { payToSaving.value = false }
}

async function submitAction() {
  if (!actionTarget.value || !pendingAction.value) return
  if (requestPaymentCheck.value && !requestPaymentCheck.value.ok) return
  const body = { action: pendingAction.value, actor_name: adminName() }
  if (requestPayToNeeded.value) {
    requestPayToError.value = payToFormError(requestPayToForm.value, actionTarget.value.pay_to)
    if (requestPayToError.value) return
    body.pay_to = payToPayload(requestPayToForm.value)
    body.version = actionTarget.value.version
  }
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(actionTarget.value.id, body)
    toast.success(`Pengajuan berhasil ${ACTION_META[pendingAction.value]?.done ?? 'diperbarui'}.`)
    showActionConfirm.value = false
    // Refresh detail modal if open
    if (showDetail.value && detail.value?.id === actionTarget.value.id) await reloadDetail()
    fetchList()
  } catch (err) {
    if (isConflictError(err)) showActionConfirm.value = false
    if (showDetail.value && detail.value?.id === actionTarget.value?.id) await handleConflict(err, 'Gagal mengubah status.')
    else toast.error(err?.message ?? 'Gagal mengubah status.')
  }
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

const editFinalInvoiceUrls = ref([])
const detailInvoiceUrls = computed(() => safeUrls(detail.value?.invoice_url, detail.value?.invoice_urls))

function openEditFinal() {
  editFinalVendorId.value = detail.value?.vendor_id || ''
  editFinalInvoice.value = detail.value?.invoice_number || ''
  editFinalInvoiceUrls.value = safeUrls(detail.value?.invoice_url, detail.value?.invoice_urls)
  // For masters, only show unsplit items for editing
  const items = getUnsplitItems(detail.value || {})
  editFinalItems.value = items.map(it => ({
    ...it,
    items: (it.items || []).map(sub => ({ ...sub, purchased_qty: purchaseQuantity(sub) }))
  }))
  editFinalPayToForm.value = newPayToForm(editFinalPayToMeta.value)
  editFinalPayToError.value = ''
  editFinalError.value = ''
  showEditFinal.value = true
}

async function submitEditFinal(andRequestPayment = false) {
  if (!editFinalVendorId.value) { editFinalError.value = 'Silakan pilih vendor terlebih dahulu.'; return }
  if (editFinalItems.value.some(g => (g.items || []).some(sub =>
    !Number.isInteger(Number(sub.purchased_qty)) || Number(sub.purchased_qty) < 1 || Number(sub.purchased_qty) > Number(sub.qty)))) {
    editFinalError.value = 'Jumlah dibeli harus bilangan bulat antara 1 dan jumlah yang diajukan.'; return
  }
  if (andRequestPayment) {
    if (!editFinalAllPriced.value) {
      editFinalError.value = 'Isi harga final semua item terlebih dahulu sebelum mengajukan pembayaran.'; return
    }
    const pErr = payToFormError(editFinalPayToForm.value, editFinalPayToMeta.value)
    editFinalPayToError.value = pErr
    if (pErr) return
  }
  savingFinal.value = true; editFinalError.value = ''
  try {
    const vendor = vendorList.value.find(v => v.id === editFinalVendorId.value)
    // Original requested quantities remain unchanged; purchasing sets its own quantity.
    const updated = await purchaseApi.updateItems(detail.value.id, {
      items: editFinalItems.value.map(g => ({
        name: g.name,
        items: (g.items || []).map(sub => ({ ...sub, purchased_qty: Number(sub.purchased_qty), final_price: Number(sub.final_price) || 0 })),
      })),
      vendor_id: editFinalVendorId.value,
      vendor_name: vendor?.name || '',
      invoice_number: editFinalInvoice.value.trim(),
      invoice_url: editFinalInvoiceUrls.value[0] || '',
      invoice_urls: editFinalInvoiceUrls.value,
      version: detail.value.version,
    })
    if (andRequestPayment) {
      await purchaseApi.updateStatus(detail.value.id, {
        action: 'request_payment',
        actor_name: adminName(),
        pay_to: payToPayload(editFinalPayToForm.value),
        version: updated?.version || undefined,
      })
      toast.success('Harga disimpan & pembayaran berhasil diajukan ke Keuangan.')
    } else {
      toast.success('Jumlah pembelian, harga, dan vendor berhasil disimpan.')
    }
    showEditFinal.value = false
    if (parentDetailId.value) {
      await backToParent()
    } else {
      detail.value = await purchaseApi.get(detail.value.id)
      detailItems.value = getUnsplitItems(detail.value)
    }
    fetchList()
  } catch (err) {
    if (isConflictError(err)) {
      // Dokumen berubah sejak dibuka: tutup form, muat ulang, isi ulang dari data terbaru.
      showEditFinal.value = false
      await handleConflict(err, 'Gagal menyimpan.')
    } else editFinalError.value = err?.message ?? 'Gagal menyimpan.'
  }
  finally { savingFinal.value = false }
}
</script>

<style scoped>
@reference "tailwindcss";
.payto-returned-badge {
  @apply inline-flex max-w-full whitespace-normal break-words rounded-full bg-red-100 px-2 py-0.5 text-[10px] font-semibold text-red-700 leading-snug;
}
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
.rab-chip { display: inline-block; max-width: 100%; overflow-wrap: break-word; margin-top: .15rem; padding: .05rem .4rem; border-radius: .3rem; background: #ecfdf5; color: #047857; font-size: .62rem; font-weight: 700; }
</style>
