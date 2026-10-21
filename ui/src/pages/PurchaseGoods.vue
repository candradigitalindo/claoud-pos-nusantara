<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Pengadaan {{ typeLabel }}</h1>
      <AppButton v-if="authStore.hasPermission(perm('submit'))" @click="openCreate">+ Buat Pengajuan</AppButton>
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
          <SearchableSelect v-model="filterStatus"
            class="w-full sm:w-auto rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
            <option value="">Semua Status</option>
            <option v-for="s in STATUS_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
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
          <span class="text-gray-800">{{ purchaseDisplayNames(row) }}</span>
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
          <span :class="statusBadge(row.status)">{{ statusLabel(row.status) }}</span>
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
      <form class="space-y-4" @submit.prevent="submitCreate">
        <AppAlert type="error" :message="createError" />
        <!-- Unit kerja pengaju: dari /my-work-units (cakupan role). Satu unit
             dipilih otomatis; lebih dari satu wajib dipilih. Outlet ikut unit. -->
        <AppAlert v-if="!workUnits.length" type="warning"
          :message="workUnitsFailed ? 'Daftar unit kerja gagal dimuat — tutup form ini lalu muat ulang halaman.' : 'Anda belum memiliki unit kerja dalam cakupan, jadi pengajuan belum bisa dikirim. Minta admin mengatur unit kerja (Batasan Unit Kerja) role Anda.'" />
        <div v-else class="space-y-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm">
          <p class="text-emerald-800">
            <strong>Pengaju:</strong> {{ form.requested_by || '-' }}
            <template v-if="workUnits.length === 1"> — <strong>Unit Kerja:</strong> {{ unitOptions[0].name }}</template>
          </p>
          <div v-if="workUnits.length > 1" class="flex flex-col gap-1">
            <label class="text-xs font-semibold text-emerald-900">Unit Kerja Pengaju <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form.work_unit_id" :options="unitOptions"
              placeholder="Pilih unit kerja yang mengajukan…" searchPlaceholder="Cari unit kerja…" />
            <p class="text-[11px] text-emerald-700">Cakupan Anda meliputi beberapa unit kerja — pilih unit yang mengajukan pengadaan ini.</p>
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
        <div v-if="!isBahan && rabNotSet" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          RAB projek <b>{{ projectRab.project_number }}</b> belum ditetapkan — pengajuan untuk projek ini ditolak server.
          Susun dan tetapkan RAB di halaman Projek dulu.
        </div>
        <div v-else-if="!isBahan && form.project_id && projectRab" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-800">
          RAB <b>{{ projectRab.project_number }}</b> v{{ projectRab.rab_version }} · {{ rabLines.length }} baris bisa diserap pengajuan ini.
          Pilih baris RAB pada tiap item agar serapannya tercatat per pos; memilih baris mengisi nama, satuan, HPS, dan sumber HPS.
        </div>

        <div v-if="isBahan" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          Pengajuan bahan baku langsung <strong>menunggu persetujuan</strong> (tanpa verifikasi) dan masih bisa diubah sampai disetujui.
          Seluruh bahan baku diterima di <strong>Gudang Induk</strong> dan dicatat sebagai bahan (HPP).
          <template v-if="!stockCatalogFailed">Ketik nama item lalu pilih saran dari katalog stok agar namanya sama dengan katalog dan satuannya terisi otomatis.</template>
          <template v-else>Katalog stok tidak dapat dimuat — tulis nama bahan persis seperti di katalog Gudang agar mudah dipetakan saat serah terima.</template>
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
                    <AppActionButton icon="delete" label="Hapus baris" type="button" @click="removeItem(i)" :disabled="form.items.length <= 1" class="ml-auto" />
                  </div>
                  <input v-model="item.name" placeholder="Nama Pengadaan" class="input-sm w-full" />
                  <!-- Sub-items table -->
                  <div class="pl-2 sm:ml-2 sm:pl-3 border-l-2 border-emerald-200 space-y-2">
                    <p class="text-xs font-semibold text-emerald-600 uppercase tracking-wide">Detail Item</p>
                    <div v-for="(sub, j) in item.items" :key="j"
                      class="grid grid-cols-2 gap-1.5 rounded-lg border border-gray-200 bg-white p-2 sm:grid-cols-12 sm:items-center sm:rounded-none sm:border-0 sm:bg-transparent sm:p-0">
                      <div v-if="!isBahan && form.project_id && rabLines.length" class="col-span-2 sm:col-span-12">
                        <div class="flex items-center gap-1.5">
                          <span class="shrink-0 rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-bold text-emerald-700">RAB</span>
                          <SearchableSelect v-model="sub.rab_item_id" class="input-sm w-full" @change="applyRabLine(sub)">
                            <option value="">— Di luar RAB (tidak menyerap baris mana pun) —</option>
                            <option v-for="l in rabLines" :key="l.id" :value="l.id">{{ rabOptionLabel(l) }}</option>
                          </SearchableSelect>
                        </div>
                        <p v-if="rabWarn(sub)" class="mt-0.5 text-[11px] font-semibold text-red-600">{{ rabWarn(sub) }}</p>
                      </div>
                      <input v-model="sub.name" :placeholder="isBahan ? 'Nama bahan (ketik untuk saran katalog)' : 'Nama item'"
                        :list="isBahan ? 'pg-stock-item-names' : null" autocomplete="off"
                        class="input-sm col-span-2 sm:col-span-4" @input="isBahan && applyCatalogItem(sub)" />
                      <input v-model.number="sub.qty" type="number" min="1" placeholder="Qty" class="input-sm sm:col-span-2" />
                      <input v-model="sub.unit" placeholder="Satuan" class="input-sm sm:col-span-1" />
                      <RupiahInput v-model="sub.hps_price" placeholder="HPS satuan" class="col-span-2" />
                      <p class="col-span-1 self-center text-right text-xs text-gray-500 sm:col-span-2">{{ formatRupiah((sub.qty || 0) * (sub.hps_price || 0)) }}</p>
                      <AppActionButton icon="delete" label="Hapus baris" type="button" @click="removeSubItem(i, j)" :disabled="item.items.length <= 1" class="self-center" />
                      <!-- Sumber HPS diberi barisnya sendiri: tautan marketplace
                           panjang dan tidak muat di deret kolom angka. -->
                      <div class="col-span-2 sm:col-span-12">
                        <div class="flex items-center gap-1.5 rounded-lg bg-gray-50 px-2 py-1.5">
                          <span class="shrink-0 text-gray-400">
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 007.5.5l3-3a5 5 0 00-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 00-7.5-.5l-3 3a5 5 0 007 7l1.7-1.7"/></svg>
                          </span>
                          <input v-model="sub.hps_source" class="input-sm w-full border-0 bg-transparent px-0"
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
          <div class="mt-3">
            <button type="button" @click="addItem" class="text-sm text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Pengadaan</button>
          </div>
          <datalist v-if="isBahan" id="pg-stock-item-names">
            <option v-for="it in stockItemSuggestions" :key="it.id || it.key" :value="it.name" :label="it.unit ? `${it.name} (${it.unit})` : it.name" />
          </datalist>
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

        <AppInput v-model="form.notes" label="Catatan / Alasan Pembelian" placeholder="Keterangan tambahan (opsional)" />
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
                <dt class="text-gray-400">Diajukan</dt>
                <dd class="text-gray-600">{{ formatDateTime(detail.created_at) }}</dd>
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

          <!-- Verifikasi -->
          <div v-if="detail.verified_by || detail.verified_at" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-indigo-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-indigo-600 mb-1.5">Verifikasi</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
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

          <!-- Penolakan (oleh verifikator atau penyetuju). Data lama menulis
               penolak ke approved_by — dipakai sebagai cadangan. -->
          <div v-if="detail.status === 'rejected'" class="rounded-lg border border-gray-200 bg-white shadow-sm overflow-hidden">
            <div class="border-l-4 border-red-500 px-3 py-2.5">
              <p class="text-[11px] font-semibold uppercase tracking-wider text-red-600 mb-1.5">Penolakan</p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
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
                  <span v-if="showHps" class="text-gray-500">HPS: <strong>{{ formatRupiah(detail.status === 'pending' ? calcDetailItemHps(item) : item.hps_total) }}</strong></span>
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
                  <dl class="mt-1.5 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[11px]">
                    <dt class="text-gray-400">Qty</dt>
                    <dd class="text-gray-800">
                      <span v-if="canEditItems" class="inline-flex items-center gap-1">
                        <input v-model.number="sub.qty" type="number" min="1" class="w-20 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span>{{ sub.unit }}</span>
                      </span>
                      <span v-else>{{ sub.qty }} {{ sub.unit }}</span>
                    </dd>
                    <template v-if="showHps && sub.hps_source">
                      <dt class="text-gray-400">Sumber HPS</dt>
                      <dd class="min-w-0">
                        <a v-if="isLink(sub.hps_source)" :href="sub.hps_source" target="_blank" rel="noopener"
                          class="block break-all text-emerald-700 underline">{{ shortSource(sub.hps_source) }}</a>
                        <span v-else class="block break-words text-gray-700">{{ sub.hps_source }}</span>
                      </dd>
                    </template>
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
                      <th v-if="canEditItems" class="py-1.5 w-8"></th>
                      <th class="py-1.5 pr-2">#</th>
                      <th class="py-1.5 pr-2">Nama Item</th>
                      <th class="py-1.5 pr-2 text-right">Qty</th>
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
                        <input v-if="canEditItems" v-model.number="sub.qty" type="number" min="1" class="w-24 text-right input-sm py-0.5 px-1" @change="saveDetailQty()" />
                        <span v-else>{{ sub.qty }}</span>
                      </td>
                      <td class="py-1.5 pr-2">{{ sub.unit }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(sub.hps_price) }}</td>
                      <td v-if="showHps" class="py-1.5 pr-2 text-right">{{ formatRupiah(detail.status === 'pending' ? (sub.qty || 0) * (sub.hps_price || 0) : sub.hps_subtotal) }}</td>
                      <td v-if="showHps" class="max-w-[14rem] py-1.5 pr-2">
                        <input v-if="canEditItems" v-model="sub.hps_source" class="input-sm w-full py-0.5 px-1"
                          placeholder="toko / link" @change="saveDetailQty()" />
                        <a v-else-if="isLink(sub.hps_source)" :href="sub.hps_source" target="_blank" rel="noopener"
                          class="block truncate text-emerald-700 underline" :title="sub.hps_source">{{ shortSource(sub.hps_source) }}</a>
                        <span v-else class="block truncate text-gray-600" :title="sub.hps_source">{{ sub.hps_source || '—' }}</span>
                      </td>
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
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-600">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
                </div>
                <div>
                  <p class="text-sm font-bold text-blue-900">Data Hasil Split</p>
                  <p class="text-xs text-blue-700">Silakan lengkapi harga final untuk vendor ini<template v-if="detail.parent_id">, lalu ajukan pembayarannya</template>.</p>
                </div>
              </div>
              <div class="flex w-full shrink-0 flex-col gap-2 sm:w-auto sm:flex-row">
                <AppActionButton icon="cost" label="Isi Harga Sekarang" @click="openEditFinal" />
                <!-- Pecahan diajukan bayar sendiri: pecahan yang belum berharga saat
                     induknya diajukan tidak ikut cascade dan tertahan di 'approved'.
                     Nonaktif selama masih ada item tanpa harga final (server menolak). -->
                <AppActionButton icon="pay" :label="paymentReadiness.ok ? '' : 'Isi harga final semua item dulu'" v-if="detail.parent_id" :disabled="!paymentReadiness.ok" @click="confirmDetailAction('request_payment')" />
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

        <!-- Struk pembelian & pengambilan barang (sejak disetujui): struk tiap
             transaksi di vendor dibayar Keuangan per struk; pengambilan oleh
             petugas logistik berstatus "dalam pengantaran" sampai diterima. -->
        <PurchaseStrukSection v-if="showStrukSection" :doc="detail" :can-manage="canManageStruk" @changed="reloadDetail" />
        <PurchasePickupSection v-if="showPickupSection" :doc="detail" :can-pickup="canPickup" @changed="reloadDetail" />

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
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
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

        <!-- Persetujuan: hanya untuk pengajuan yang sudah diverifikasi. -->
        <div v-if="detail.status === 'verified' && canApprove" class="rounded-xl border border-blue-200 bg-blue-50/40 p-3 sm:p-4">
          <p class="mb-3 text-xs text-gray-600">
            <template v-if="isBahan">Diajukan oleh <strong>{{ detail.requested_by || '-' }}</strong> · {{ formatDateTime(detail.created_at) }} — bahan baku tanpa tahap verifikasi.</template>
            <template v-else>Diverifikasi oleh <strong>{{ detail.verified_by || '-' }}</strong><template v-if="detail.verified_at"> · {{ formatDateTime(detail.verified_at) }}</template><template v-if="detail.rejected_items?.length"> · {{ detail.rejected_items.length }} item ditolak saat verifikasi</template>.</template>
          </p>
          <!-- Anggaran projek baru terpakai saat disetujui: peringatkan bila melampaui sisa baris RAB. -->
          <div v-if="approvalRabWarnings.length" class="mb-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
            <p class="font-semibold">Menyetujui pengajuan ini melampaui sisa anggaran RAB:</p>
            <ul class="mt-1 list-disc space-y-0.5 pl-4">
              <li v-for="w in approvalRabWarnings" :key="w.id">{{ w.name }} — sisa {{ formatRupiah(w.remaining) }}, pengajuan ini {{ formatRupiah(w.amount) }}</li>
            </ul>
            <p class="mt-1 text-amber-700">Persetujuan tetap bisa dilakukan. Bila perlu, revisi RAB di halaman Projek lebih dulu.</p>
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
            <AppActionButton icon="approve" label="Setujui" @click="submitDetailApprove" :disabled="actionLoading" />
          </div>
        </div>
      </div>
      <template #footer>
        <div v-if="!parentDetailId" class="flex flex-wrap items-center gap-2 mr-auto">
          <!-- Nonaktif selama ada item tanpa harga final; alasannya tampil di atas. -->
          <AppActionButton icon="pay" :label="paymentReadiness.ok ? '' : 'Belum bisa diajukan — lihat keterangan di atas'" v-if="detail && detail.status === 'approved' && authStore.hasPermission(perm('purchasing')) && !detail.parent_id" :disabled="!paymentReadiness.ok"
            @click="confirmDetailAction('request_payment')" />
          <!-- Serah terima sejak disetujui: barang tempo boleh datang sebelum dibayar (T-79). -->
          <AppActionButton icon="receive" :label="(detail.receipt_status === 'partial' ? 'Lanjutkan Serah Terima' : 'Serah Terima')" v-if="canOpenReceiving && detail.status !== 'received'" @click="openReceiving()" />
          <AppActionButton icon="receive" label="Lengkapi Penerimaan" v-else-if="canOpenReceiving" @click="openReceiving()" />
          <AppActionButton icon="cancel" label="Batalkan" v-if="canCancel" @click="confirmDetailAction('cancel')" />
        </div>
        <AppButton variant="secondary" @click="parentDetailId ? backToParent() : (showDetail = false)">{{ parentDetailId ? '← Kembali' : 'Tutup' }}</AppButton>
      </template>
    </AppModal>

    <!-- Edit Final Price Modal -->
    <AppModal v-model="showEditFinal" title="Isi Harga (Purchasing)" size="2xl">
      <AppAlert type="error" :message="editFinalError" />
      <p class="mb-3 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600">
        Pengajuan sudah disetujui: nama, qty, satuan, dan HPS terkunci, dan baris tidak bisa ditambah atau dihapus.
        Yang bisa diubah hanya harga final per item, vendor, dan nomor invoice.
      </p>
      <div class="mb-4 bg-emerald-50 p-3 sm:p-4 rounded-xl border border-emerald-100">
        <label class="text-sm font-bold text-emerald-900 mb-1.5 block">Vendor / Supplier Utama</label>
        <SearchableSelect v-model="editFinalVendorId"
          class="w-full rounded-lg border border-emerald-200 px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500">
          <option value="">— Pilih Vendor —</option>
          <option v-for="v in vendorList" :key="v.id" :value="v.id">{{ v.name }}</option>
        </SearchableSelect>
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
    <AppModal v-model="showActionConfirm" :title="actionConfirmTitle" size="sm">
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
      <template #footer>
        <AppButton variant="secondary" @click="showActionConfirm = false">{{ pendingAction === 'cancel' ? 'Kembali' : 'Batal' }}</AppButton>
        <AppButton :variant="pendingAction === 'cancel' ? 'danger' : 'primary'" :loading="actionLoading"
          :disabled="!!requestPaymentCheck && !requestPaymentCheck.ok" @click="submitAction">{{ actionConfirmBtn }}</AppButton>
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
import { ref, computed, onMounted, watch } from 'vue'
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
import ReceivingDialog from '@/components/ReceivingDialog.vue'
import PurchaseStrukSection from '@/components/PurchaseStrukSection.vue'
import PurchasePickupSection from '@/components/PurchasePickupSection.vue'
import { stockItemsApi } from '@/api/warehouse.js'
import {
  purchaseStatusLabel, purchaseStatusBadge, purchaseStatusOptions,
  VERIFY_REASON_MIN, reasonLength, isConflictError,
  requestPaymentReadiness,
  RECEIVABLE_PURCHASE_STATUSES, isPhysicalPurchaseType, purchaseDisplayNames,
} from '@/utils/procurement.js'

const toast = useToastStore()
const authStore = useAuthStore()

// Struk pembelian: dicatat bagian pembelian atau petugas logistik, sejak
// pengajuan disetujui, pada dokumen yang masih memegang item sendiri (induk
// yang itemnya sudah habis dipecah dicatat per pecahan). Pengambilan barang:
// petugas logistik (izin Pengambilan Barang).
const canManageStruk = computed(() => authStore.hasPermission(perm('purchasing')) || authStore.hasPermission(perm('pickup')))
const canPickup = computed(() => authStore.hasPermission(perm('pickup')))
const showStrukSection = computed(() => {
  const d = detail.value
  return !!d && RECEIVABLE_PURCHASE_STATUSES.includes(d.status) && ((d.items || []).length > 0 || (d.struks || []).length > 0)
})
const showPickupSection = computed(() => {
  const d = detail.value
  return !!d && RECEIVABLE_PURCHASE_STATUSES.includes(d.status) && (canPickup.value || (d.pickups || []).length > 0)
})
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
// Serah terima: pengaju (submit jenisnya), bagian Aset (assets.create — baris
// peralatan), atau gudang (stockledger.adjust — baris stok). Bahan baku
// seluruhnya stok, jadi assets.create saja tidak cukup untuknya.
const canReceive = computed(() => authStore.hasPermission(perm('submit'))
  || authStore.hasPermission('stockledger.adjust')
  || (!isBahan.value && authStore.hasPermission('assets.create')))
// Tombol Serah Terima: dokumen berkomitmen — disetujui, menunggu bayar (tempo,
// T-79), dibayar sebagian/lunas, atau diterima (melengkapi). Sebelum status
// 'received', tombol hilang bila seluruh barang sudah tercatat. Jasa tidak
// punya serah terima barang (draf jasa dibalas 400), jadi tidak ditawarkan.
const canOpenReceiving = computed(() => {
  const d = detail.value
  if (!d || !canReceive.value || !isPhysicalPurchaseType(d.request_type || requestType.value)) return false
  if (!RECEIVABLE_PURCHASE_STATUSES.includes(d.status)) return false
  return d.status === 'received' || d.receipt_status !== 'received'
})

// Nama item di katalog stok = barang dapur (diterima di Gudang Induk). Dipakai
// untuk memperingatkan pengaju SEBELUM disimpan (pengadaan barang), dan sebagai
// saran nama + satuan di form pengadaan bahan baku; server tetap yang memutuskan.
const stockItems = ref([])
// Katalog gagal dimuat (mis. tanpa izin): banner tidak menyuruh memilih saran.
const stockCatalogFailed = ref(false)
const stockItemNames = computed(() => new Set(stockItems.value.map(r => r.key)))
const stockItemByName = computed(() => Object.fromEntries(stockItems.value.map(r => [r.key, r])))
async function loadStockItemNames() {
  try {
    const d = await stockItemsApi.names()
    const rows = Array.isArray(d) ? d : (d?.data || d?.items || [])
    stockCatalogFailed.value = false
    stockItems.value = rows
      .filter(r => String(r.name || '').trim())
      .map(r => ({
        id: r.id,
        name: String(r.name).trim(),
        key: String(r.name).trim().toLowerCase(),
        active: r.is_active !== false,
        // Satuan beli mengikuti draft MRP: label satuan distribusi → satuan distribusi → satuan dasar.
        unit: r.unit || '',
      }))
  } catch { stockItems.value = []; stockCatalogFailed.value = true }
}
// Saran datalist hanya item aktif; pencocokan nama tetap memakai seluruh katalog.
const stockItemSuggestions = computed(() => stockItems.value.filter(r => r.active))
// Memilih/mengetik nama yang persis ada di katalog mengisi satuannya.
function applyCatalogItem(sub) {
  const it = stockItemByName.value[String(sub.name || '').trim().toLowerCase()]
  if (it?.unit) sub.unit = it.unit
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
const form = ref(emptyForm())

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
function isLink(v) { return /^https?:\/\//i.test(String(v || '').trim()) }
function shortSource(v) {
  const s = String(v || '').trim()
  if (!isLink(s)) return s
  try {
    const u = new URL(s)
    return u.hostname.replace(/^www\./, '') + (u.pathname.length > 1 ? '…' : '')
  } catch { return s }
}

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
  return (item.items || []).reduce((s, sub) => s + (sub.qty || 0) * (sub.final_price || 0), 0)
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
  }
}
function addItem() {
  form.value.items.push({ name: '', items: [{ name: '', qty: 1, unit: 'pcs', hps_price: 0, hps_source: '', rab_item_id: '' }] })
}
function removeItem(i) { if (form.value.items.length > 1) form.value.items.splice(i, 1) }
function addSubItem(i) { form.value.items[i].items.push({ name: '', qty: 1, unit: 'pcs', hps_price: 0, hps_source: '', rab_item_id: '' }) }
function removeSubItem(i, j) { if (form.value.items[i].items.length > 1) form.value.items[i].items.splice(j, 1) }
function adminName() { return authStore.admin?.name || 'Admin' }

const projects = ref([])

// Hanya projek yang masih bisa dibelanjai yang boleh dipilih; yang sudah
// selesai/batal tetap memegang pengajuan lamanya tapi tidak menerima yang baru.
const projectOptions = computed(() => [
  { id: '', name: 'Tanpa projek' },
  ...projects.value
    .filter(p => p.status === 'berjalan' || p.status === 'draft')
    .map(p => ({ id: p.id, name: p.project_number ? `${p.name} — ${p.project_number}` : p.name })),
])

// ── Baris RAB projek yang dipilih di form ──
// Pengajuan barang hanya boleh menyerap baris barang/umum; jasa → jasa/umum.
const projectRab = ref(null)
const rabLines = computed(() => (projectRab.value?.items || []).filter(l => l.kind === 'umum' || l.kind === requestType.value))
const rabById = computed(() => Object.fromEntries((projectRab.value?.items || []).map(l => [l.id, l])))
const rabNotSet = computed(() => !!form.value.project_id && !!projectRab.value && projectRab.value.rab_status !== 'ditetapkan')
async function loadProjectRab(id) {
  projectRab.value = null
  if (!id) return
  try { projectRab.value = await projectsApi.rab(id) } catch { projectRab.value = null }
}
watch(() => form.value.project_id, (id) => {
  loadProjectRab(id)
  if (!id) for (const it of form.value.items || []) for (const sub of it.items || []) sub.rab_item_id = ''
})
function rabOptionLabel(l) {
  return `${l.section ? l.section + ' / ' : ''}${l.name} · sisa ${formatRupiah(l.remaining)}${l.unit ? ` (${l.qty} ${l.unit} @ ${formatRupiah(l.unit_price)})` : ''}`
}
// Memilih baris RAB mengisi nama (bila kosong), satuan, HPS satuan, dan sumber HPS.
function applyRabLine(sub) {
  const l = rabById.value[sub.rab_item_id]
  if (!l) return
  if (!String(sub.name || '').trim()) sub.name = l.name
  if (l.unit) sub.unit = l.unit
  sub.hps_price = l.unit_price
  sub.hps_source = `RAB ${projectRab.value.project_number} v${projectRab.value.rab_version}: ${l.section ? l.section + ' / ' : ''}${l.name}`
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
  if (!d?.project_id || d.status !== 'verified') return []
  const byLine = {}
  for (const g of d.items || []) {
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
  await Promise.all([fetchList(), fetchWorkUnits(), fetchMyWorkUnit(), fetchVendors(), isBahan.value ? Promise.resolve() : fetchProjects(), loadStockItemNames()])
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
async function fetchList() {
  const seq = ++listSeq
  loading.value = true; errorMsg.value = ''
  try {
    const params = { page: page.value, limit: 20, type: requestType.value }
    if (filterWorkUnit.value) params.work_unit_id = filterWorkUnit.value
    if (filterStatus.value) params.status = filterStatus.value
    const data = await purchaseApi.list(params)
    if (seq !== listSeq) return
    requests.value = data.requests || []
    totalPages.value = data.total_pages || 1
  } catch (err) { if (seq === listSeq) errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { if (seq === listSeq) loading.value = false }
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
  filterWorkUnit.value = ''
  filterStatus.value = ''
  page.value = 1
  totalPages.value = 1
  requests.value = []
  fetchList()
})

function goPage(p) { page.value = p; fetchList() }

function openCreate() {
  form.value = emptyForm(); createError.value = ''
  // Pengaju = akun yang login (server mengisinya dari sesi, bukan dari form).
  form.value.requested_by = authStore.admin?.name || authStore.admin?.username || myWorkUnit.value?.admin_name || ''
  // Satu unit → otomatis. Lebih dari satu → wajib dipilih; bawaannya unit yang
  // ditugaskan langsung ke akun ini bila ada di cakupannya.
  const units = workUnits.value
  if (units.length === 1) form.value.work_unit_id = units[0].id
  else if (myWorkUnit.value && units.some(u => u.id === myWorkUnit.value.id)) form.value.work_unit_id = myWorkUnit.value.id
  showCreate.value = true
}

async function submitCreate() {
  createError.value = ''
  if (!workUnits.value.length) { createError.value = 'Anda belum memiliki unit kerja dalam cakupan — pengajuan tidak bisa dikirim.'; return }
  const unit = workUnits.value.find(u => u.id === form.value.work_unit_id)
  if (!unit) { createError.value = 'Pilih unit kerja pengaju terlebih dahulu.'; return }
  const validItems = form.value.items.filter(it => it.name.trim())
  if (!validItems.length) { createError.value = 'Minimal 1 barang harus diisi.'; return }

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
      items: validItems,
      notes: form.value.notes.trim(),
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

// Simpan perubahan item (qty / hapus / sumber HPS) selama pending. Berhasil
// atau gagal, layar dimuat ulang dari server — bila server menolak, angka
// yang tidak pernah tersimpan tidak boleh tetap tampil seolah berlaku.
async function saveDetailQty(successMsg = 'Perubahan item tersimpan.') {
  if (!detail.value?.id) return
  savingDetailQty.value = true
  try {
    // version: bila dokumen sudah berubah (mis. baru diverifikasi), server 409.
    await purchaseApi.updateItems(detail.value.id, { items: detailItems.value, version: detail.value.version })
    toast.success(successMsg)
    fetchList()
  } catch (err) {
    toast.error(isConflictError(err)
      ? (err?.message || 'Pengajuan berubah') + ' — data terbaru dimuat ulang, ulangi perubahan bila masih perlu.'
      : (err?.message ?? 'Gagal menyimpan perubahan item.'))
  }
  finally {
    await reloadDetail()
    savingDetailQty.value = false
  }
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
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(detail.value.id, { action: 'approve', actor_name: adminName(), version: detail.value.version })
    toast.success('Pengajuan berhasil disetujui.')
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
  showActionConfirm.value = true
}

async function submitAction() {
  if (!actionTarget.value || !pendingAction.value) return
  if (requestPaymentCheck.value && !requestPaymentCheck.value.ok) return
  actionLoading.value = true
  try {
    await purchaseApi.updateStatus(actionTarget.value.id, { action: pendingAction.value, actor_name: adminName() })
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
    // Status approved: server hanya menerima perubahan harga final, vendor, dan
    // invoice — baris dikirim persis seperti tersimpan, hanya final_price yang baru.
    await purchaseApi.updateItems(detail.value.id, {
      items: editFinalItems.value.map(g => ({
        name: g.name,
        items: (g.items || []).map(sub => ({ ...sub, final_price: Number(sub.final_price) || 0 })),
      })),
      vendor_id: editFinalVendorId.value,
      vendor_name: vendor?.name || '',
      invoice_number: editFinalInvoice.value.trim(),
      version: detail.value.version,
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
.rab-chip { display: inline-block; margin-top: .15rem; padding: .05rem .4rem; border-radius: .3rem; background: #ecfdf5; color: #047857; font-size: .62rem; font-weight: 700; }
</style>
