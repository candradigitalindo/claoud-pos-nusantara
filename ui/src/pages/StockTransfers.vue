<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-xl font-bold text-gray-900">Transfer Stok</h1>
      <AppButton @click="openCreate">{{ isOutletUser ? '+ Minta Stok / Transfer' : '+ Buat Transfer' }}</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="w-full sm:w-auto sm:min-w-[180px]">
          <SearchSelect
            v-model="filterStatus"
            :options="statusOptions"
            placeholder="Semua Status"
            searchPlaceholder="Cari status..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
        <div class="w-full sm:w-auto sm:min-w-[240px] sm:flex-1 sm:max-w-sm">
          <SearchSelect
            v-model="filterWarehouse"
            :options="warehouseFilterOptions"
            placeholder="Semua Gudang"
            searchPlaceholder="Cari gudang..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
        <div class="w-full sm:w-auto sm:min-w-[165px]">
          <SearchSelect
            v-model="filterDivision"
            :options="divisionFilterOptions"
            placeholder="Semua Divisi"
            searchPlaceholder="Cari divisi..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
        <div class="flex flex-wrap items-center gap-2 w-full sm:w-auto">
          <input v-model="filterDateFrom" type="date" aria-label="Dari tanggal transfer" title="Dari tanggal transfer"
            class="text-sm border border-gray-300 rounded-lg px-2.5 py-2 bg-white focus:outline-none focus:ring-2 focus:ring-emerald-400"
            @change="applyFilters" />
          <span class="text-xs text-gray-400">s/d</span>
          <input v-model="filterDateTo" type="date" aria-label="Sampai tanggal transfer" title="Sampai tanggal transfer"
            class="text-sm border border-gray-300 rounded-lg px-2.5 py-2 bg-white focus:outline-none focus:ring-2 focus:ring-emerald-400"
            @change="applyFilters" />
          <button v-if="filterDateFrom || filterDateTo" type="button" @click="clearDateFilter"
            class="text-xs font-medium text-gray-500 hover:text-red-600 px-1.5 py-1">
            Reset tanggal
          </button>
        </div>
      </div>

      <!-- Layar kecil: kartu, supaya status ("Menunggu Gudang Induk") tidak
           tersembunyi di balik gulir tabel. -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!transfers.length" class="p-6 text-center text-sm text-gray-400">Belum ada transfer.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in transfers" :key="row.id">
            <button @click="openDetail(row.id)" class="w-full px-4 py-3 text-left hover:bg-gray-50">
              <div class="flex items-start justify-between gap-2">
                <span class="font-mono text-sm text-emerald-600">{{ row.transfer_number }}</span>
                <span :class="statusCls(row.status)" class="shrink-0 whitespace-nowrap text-xs font-medium px-2 py-0.5 rounded-full">
                  {{ transferRowStatusLabel(row) }}
                </span>
              </div>
              <p class="mt-1 text-sm text-gray-800 break-words">
                {{ row.from_warehouse_name }} <span class="text-gray-400">→</span> {{ row.to_warehouse_name }}
                <span v-if="row.target_division" :class="['inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wide ml-1', transferDivisionClass(row.target_division)]">
                  {{ row.target_division }}
                </span>
              </p>
              <p class="mt-0.5 text-xs text-gray-500">
                {{ row.transfer_date || row.created_at.slice(0,10) }}
                <span v-if="row.transfer_date && row.transfer_date !== row.created_at?.slice(0,10)" class="text-gray-400">(input {{ row.created_at.slice(0,10) }})</span>
                · {{ row.created_by || '—' }}
              </p>
            </button>
          </li>
        </ul>
      </div>
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="transfers" :loading="loading" emptyText="Belum ada transfer.">
        <template #cell-transfer_number="{ row }">
          <button @click="openDetail(row.id)" class="font-mono text-sm text-emerald-600 hover:text-emerald-800 hover:underline">
            {{ row.transfer_number }}
          </button>
        </template>
        <template #cell-route="{ row }">
          <div class="text-sm flex flex-wrap items-center gap-1">
            <span class="font-medium">{{ row.from_warehouse_name }}</span>
            <span class="text-gray-400 mx-0.5">→</span>
            <span class="font-medium">{{ row.to_warehouse_name }}</span>
            <span v-if="row.target_division" :class="['inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wide ml-1', transferDivisionClass(row.target_division)]">
              {{ row.target_division }}
            </span>
          </div>
        </template>
        <template #cell-status="{ row }">
          <span :class="statusCls(row.status)" class="whitespace-nowrap text-xs font-medium px-2 py-0.5 rounded-full">
            {{ transferRowStatusLabel(row) }}
          </span>
        </template>
        <template #cell-created_at="{ row }">
          <div class="text-sm text-gray-700">
            <span>{{ row.transfer_date || row.created_at.slice(0,10) }}</span>
            <span v-if="row.transfer_date && row.transfer_date !== row.created_at?.slice(0,10)" class="block text-[11px] text-gray-400">
              input {{ row.created_at.slice(0,10) }}
            </span>
          </div>
        </template>
        <template #cell-actions="{ row }">
          <AppActionButton icon="view" label="Detail" @click="openDetail(row.id)" />
        </template>
      </AppTable>
      <AppPagination v-model:page="page" :total-pages="totalPages" class="px-4 py-3 border-t border-gray-100" />
    </AppCard>

    <!-- Create Transfer Modal -->
    <AppModal v-model="showCreate" :title="isRequestForm ? 'Minta Stok ke Gudang Induk' : 'Buat Transfer Stok'" size="xl">
      <div class="space-y-4">
        <div class="modal-form-grid">
          <div class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Gudang Asal</label>
            <SearchSelect
              v-model="form.from_warehouse_id"
              :options="fromWarehouseOptions"
              placeholder="Pilih gudang asal..."
              searchPlaceholder="Cari gudang asal..."
              valueKey="value"
              labelKey="label"
            />
          </div>
          <div class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Gudang Tujuan</label>
            <SearchSelect
              v-model="form.to_warehouse_id"
              :options="toWarehouseOptions"
              placeholder="Pilih gudang tujuan..."
              searchPlaceholder="Cari gudang tujuan..."
              valueKey="value"
              labelKey="label"
            />
          </div>
        </div>
        <div class="modal-form-grid">
          <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-400">Asal</div>
            <div class="mt-1 text-sm font-semibold text-gray-900">{{ selectedFromWarehouse?.name || 'Belum dipilih' }}</div>
            <div class="text-xs text-gray-500">{{ describeWarehouse(selectedFromWarehouse) }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
            <div class="text-[11px] font-semibold uppercase tracking-wide text-gray-400">Tujuan</div>
            <div class="mt-1 text-sm font-semibold text-gray-900">{{ selectedToWarehouse?.name || 'Belum dipilih' }}</div>
            <div class="text-xs text-gray-500">{{ describeWarehouse(selectedToWarehouse) }}</div>
          </div>
        </div>
        <!-- Permintaan outlet ke Gudang Induk: tahap setujui & kirim milik
             Gudang Induk (server menolaknya dari role per outlet). -->
        <div v-if="isRequestForm" class="flex items-start gap-2 rounded-lg bg-blue-50 ring-1 ring-blue-100 px-3 py-2 text-xs text-blue-800">
          <svg class="h-4 w-4 mt-0.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
          <span>
            Ini <b>permintaan stok ke Gudang Induk</b>. Gudang Induk akan menyetujui dan mengirim barangnya;
            Anda mengonfirmasi penerimaan di gudang outlet saat barang tiba.
          </span>
        </div>
        <p v-else-if="isOutletUser" class="text-xs text-gray-500 -mt-1">
          Pilih <b>Gudang Induk</b> sebagai gudang asal untuk meminta stok, atau gudang outlet Anda untuk memindahkan stok antar outlet dalam cakupan Anda.
        </p>
        <p v-else class="text-xs text-gray-500 -mt-1">
          Transfer stok mendukung antar gudang induk, gudang induk ke outlet, dan antar gudang outlet.
        </p>
        <div class="modal-form-grid">
          <div class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Tanggal Transfer</label>
            <input v-model="form.transfer_date" type="date"
              class="w-full text-sm border border-gray-300 rounded-lg px-3 py-2 bg-white focus:outline-none focus:ring-2 focus:ring-emerald-400" />
            <p class="text-[11px] text-gray-500">Default hari ini — bisa diubah untuk rekap tanggal manual.</p>
          </div>
          <div v-if="!selectedToWarehouse || selectedToWarehouse.type === 'outlet'" class="space-y-1">
            <label class="block text-sm font-medium text-gray-700">Divisi Outlet Tujuan</label>
            <div class="flex flex-wrap gap-1.5 pt-0.5">
              <button type="button" @click="form.target_division = ''"
                :class="['px-3 py-1.5 rounded-lg text-xs font-semibold border transition cursor-pointer',
                  !form.target_division ? 'bg-gray-800 text-white border-gray-800' : 'bg-white text-gray-600 border-gray-200 hover:bg-gray-50']">
                Umum
              </button>
              <button v-for="div in TRANSFER_DIVISIONS" :key="div.value" type="button"
                @click="form.target_division = div.value"
                :class="['px-3 py-1.5 rounded-lg text-xs font-bold tracking-wide border transition cursor-pointer',
                  form.target_division === div.value ? divisionActiveClass(div.value) : 'bg-white text-gray-600 border-gray-200 hover:bg-gray-50']">
                {{ div.label }}
              </button>
            </div>
            <p class="text-[11px] text-gray-500">Tracking pengantaran barang per divisi di outlet tujuan (BAR / KITCHEN / SERVER).</p>
          </div>
        </div>
        <AppInput v-model="form.notes" label="Keterangan" placeholder="Opsional" />

        <!-- Items -->
        <div>
          <div class="flex flex-wrap items-center justify-between gap-2 mb-2">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-gray-700">Daftar Item</span>
              <span v-if="loadingStockItems" class="text-xs text-gray-400">Memuat stok…</span>
              <span v-else-if="selectedFromWarehouse" class="text-xs text-gray-500">
                ({{ availableStockCount }} berstok / {{ stockItems.length }} bahan baku)
              </span>
            </div>
            <button @click="addItem" class="text-xs text-emerald-600 hover:text-emerald-800 font-medium">+ Tambah Item</button>
          </div>
          <div v-if="form.items.length === 0" class="text-sm text-gray-400 py-3 text-center border border-dashed border-gray-200 rounded-lg">
            Belum ada item. Klik "+ Tambah Item".
          </div>
          <div v-for="(it, i) in form.items" :key="i" class="mb-3 sm:mb-2 space-y-1">
            <div class="flex flex-wrap sm:flex-nowrap items-end gap-2">
              <div class="w-full min-w-0 sm:w-auto sm:flex-1">
                <label class="block text-xs text-gray-500 mb-1">Bahan Baku</label>
                <SearchSelect
                  v-model="it.item_id"
                  :options="stockItemOptions"
                  placeholder="Pilih bahan baku..."
                  searchPlaceholder="Cari bahan baku..."
                  valueKey="value"
                  labelKey="label"
                  @change="onItemSelect(it)"
                />
              </div>
              <div class="w-28">
                <label class="block text-xs text-gray-500 mb-1">Jumlah</label>
                <input v-model.number="it.qty_dist" type="number" step="any" min="0"
                  class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
              </div>
              <div class="w-28">
                <label class="block text-xs text-gray-500 mb-1">Satuan</label>
                <SearchableSelect v-model="it.unit_used" class="w-full text-sm border border-gray-200 rounded-lg px-2 py-2">
                  <option v-for="unit in stockUnitOptions(stockItemMap[it.item_id])" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
                </SearchableSelect>
              </div>
              <div class="flex-1 sm:flex-none sm:w-28 text-xs text-gray-400 pb-2">
                <span v-if="it.qty_dist && it.dist_ratio">
                  = {{ (it.qty_dist * stockUnitRatio(stockItemMap[it.item_id], it.unit_used)).toFixed(3) }} {{ it.base_unit }}
                </span>
              </div>
              <AppActionButton icon="delete" label="Hapus baris" @click="removeItem(i)" />
            </div>
            <div v-if="selectedFromWarehouse && it.item_id && stockItemMap[it.item_id]" class="text-[11px] pl-0.5">
              <span :class="itemExceedsSourceStock(it) ? 'text-amber-600 font-medium' : (Number(stockItemMap[it.item_id].warehouse_qty || 0) > 0 ? 'text-emerald-700' : 'text-gray-400')">
                Stok di {{ selectedFromWarehouse.name }}: {{ formatSourceStock(stockItemMap[it.item_id]) }}
                <template v-if="itemExceedsSourceStock(it)"> · Melebihi stok tersedia saat ini</template>
              </span>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitCreate">{{ isRequestForm ? 'Kirim Permintaan' : 'Buat Transfer' }}</AppButton>
      </template>
    </AppModal>

    <!-- Detail Modal -->
    <AppModal v-model="showDetail" :title="`Transfer ${detail?.transfer_number || ''}`" size="2xl">
      <div v-if="detail" class="space-y-4">
        <!-- Status flow -->
        <div class="flex items-center gap-2 overflow-x-auto pb-1">
          <div v-for="(s, i) in STATUSES" :key="s.value" class="flex items-center gap-2">
            <div :class="['px-3 py-1 rounded-full text-xs font-medium',
              detail.status === s.value ? statusCls(s.value) : 'bg-gray-100 text-gray-400']">
              {{ s.label }}
            </div>
            <span v-if="i < STATUSES.length - 1" class="text-gray-300">›</span>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-2 text-sm">
          <div><span class="text-gray-500">Dari:</span> <strong>{{ detail.from_warehouse_name }}</strong></div>
          <div>
            <span class="text-gray-500">Ke:</span> <strong>{{ detail.to_warehouse_name }}</strong>
            <span v-if="detail.target_division" :class="['inline-flex items-center px-2 py-0.5 rounded text-[11px] font-bold tracking-wide ml-1.5', transferDivisionClass(detail.target_division)]">
              Divisi {{ detail.target_division }}
            </span>
          </div>
          <div>
            <span class="text-gray-500">Tanggal Transfer:</span>
            <strong>{{ detail.transfer_date || detail.created_at?.slice(0,10) }}</strong>
            <span v-if="detail.transfer_date && detail.transfer_date !== detail.created_at?.slice(0,10)" class="ml-1 text-xs text-gray-400">
              (diinput {{ detail.created_at?.slice(0,10) }})
            </span>
          </div>
          <div v-if="detail.approved_by"><span class="text-gray-500">Disetujui:</span> {{ detail.approved_by }} · {{ detail.approved_at?.slice(0,10) }}</div>
          <div v-if="detail.sent_by"><span class="text-gray-500">Dikirim:</span> {{ detail.sent_by }} · {{ detail.sent_at?.slice(0,10) }}</div>
          <div v-if="detail.received_by"><span class="text-gray-500">Diterima:</span> {{ detail.received_by }} · {{ detail.received_at?.slice(0,10) }}</div>
        </div>

        <!-- Role per outlet: tahap Gudang Induk tidak bertombol — jelaskan kenapa. -->
        <div v-if="requestNotice" class="flex items-start gap-2 rounded-lg bg-blue-50 ring-1 ring-blue-100 px-3 py-2 text-xs text-blue-800">
          <svg class="h-4 w-4 mt-0.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
          <span>{{ requestNotice }}</span>
        </div>

        <!-- Items table -->
        <div class="overflow-x-auto">
        <table class="min-w-full text-sm border border-gray-100 rounded-lg overflow-hidden">
          <thead class="bg-gray-50 text-xs text-gray-500">
            <tr>
              <th class="px-3 py-2 text-left">Bahan Baku</th>
              <th class="px-3 py-2 text-right">Dikirim</th>
              <th class="px-3 py-2 text-right">Sudah diperiksa</th>
              <th v-if="isReceiving" class="px-3 py-2 text-right">Terima sekarang</th>
              <th v-if="isReceiving && canReject" class="px-3 py-2 text-right">Reject sekarang</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="it in detail.items" :key="it.id" class="hover:bg-gray-50">
              <td class="px-3 py-2 font-medium">{{ it.item_name }}</td>
              <td class="px-3 py-2 text-right">
                {{ it.qty_dist }} {{ it.unit_used }}
                <span class="text-gray-400">({{ it.qty_base.toFixed(3) }} {{ it.base_unit }})</span>
              </td>
              <td class="px-3 py-2 text-right text-xs"><p>{{ acceptedTotal(it) }} {{ it.base_unit }} diterima</p><p class="text-red-600">{{ it.rejected_qty_base || 0 }} {{ it.base_unit }} reject</p><p class="text-gray-500">{{ remaining(it) }} {{ it.base_unit }} belum diperiksa</p></td>
              <td v-if="isReceiving" class="px-3 py-2 text-right">
                <input v-model.number="it._received_qty" type="number" step="0.001"
                  min="0"
                  :max="remaining(it)"
                  :aria-label="`Terima ${it.item_name}`"
                  class="w-24 text-sm border border-gray-200 rounded px-2 py-1 text-right focus:outline-none focus:ring-1 focus:ring-emerald-400" />
                <span class="ml-1 text-xs text-gray-500">{{ it.base_unit }}</span>
              </td>
              <td v-if="isReceiving && canReject" class="px-3 py-2 text-right min-w-40">
                <input v-model.number="it._rejected_qty" type="number" min="0" step="0.0001" :max="remaining(it)" :aria-label="`Reject ${it.item_name}`" class="w-24 border rounded px-2 py-1 text-right" /><span class="ml-1 text-xs">{{ it.base_unit }}</span>
                <input v-if="it._rejected_qty > 0" v-model="it._reject_reason" :aria-label="`Alasan reject ${it.item_name}`" placeholder="Alasan reject wajib" class="block w-full mt-2 border rounded px-2 py-1 text-xs" />
              </td>
            </tr>
          </tbody>
        </table>
        </div>

        <p v-if="detail.notes" class="text-sm text-gray-500">{{ detail.notes }}</p>
        <div v-if="isReceiving" class="space-y-3 rounded-xl border border-amber-200 bg-amber-50/50 p-3">
          <p class="text-xs text-gray-600">Isi jumlah yang diterima dan ditolak pada pemeriksaan ini. Sisa yang belum datang tetap menunggu penerimaan berikutnya. Barang reject masuk karantina dan tidak menambah stok siap pakai.</p>
          <AppActionButton icon="reject" label="Reject seluruh sisa barang" v-if="canReject" type="button" @click="rejectRemaining" />
          <textarea v-model="receiveNotes" rows="2" aria-label="Catatan penerimaan" placeholder="Catatan pemeriksaan / kedatangan sebagian" class="block w-full border rounded-lg p-2 text-sm"></textarea>
          <StockEvidenceCapture multiple v-model="receiveEvidences" :warehouse-id="detail.to_warehouse_id" label="Foto barang saat diperiksa" />
        </div>
        <div v-for="receipt in detail.receipts || []" :key="receipt.id" class="border-t pt-3 text-sm">
          <p class="font-semibold">Pemeriksaan {{ formatDateTime(receipt.created_at) }} · {{ receipt.actor_name }}</p><p class="text-gray-500">{{ receipt.notes }}</p>
          <p v-for="item in receipt.items" :key="item.item_id" class="text-xs text-gray-600">{{ detail.items.find(i => i.item_id === item.item_id)?.item_name }}: terima {{ item.accepted_qty_base }}, reject {{ item.rejected_qty_base }} {{ detail.items.find(i => i.item_id === item.item_id)?.base_unit }} <span v-if="item.reason">· {{ item.reason }}</span></p>
          <div class="mt-2 flex flex-wrap gap-2.5">
            <div v-for="(ev, eidx) in receiptEvidencesOf(receipt)" :key="ev.id || eidx" class="w-36">
              <a v-if="safeUrl(ev.url)" :href="safeUrl(ev.url)" target="_blank" rel="noopener noreferrer" class="block">
                <img :src="safeUrl(ev.url)" :alt="`Bukti pemeriksaan ${eidx + 1} bertimestamp`" class="h-28 w-36 rounded-lg border object-cover" />
              </a>
              <p class="mt-1 text-[11px] text-gray-500">Server: {{ formatDateTime(ev.recorded_at) }}</p>
            </div>
          </div>
        </div>

        <!-- Bukti barang berangkat dari gudang asal. Server menolak status 'sent'
             tanpa foto — mendukung multi-foto sekaligus. -->
        <div v-if="isSending" class="rounded-xl border border-emerald-200 bg-emerald-50/50 p-3">
          <PhotoCapture multiple v-model="sendPhotos" label="Foto Barang Saat Dikirim"
            hint="Bisa pilih lebih dari satu foto saat barang dimuat/berangkat dari gudang asal." />
        </div>
        <!-- Alamat foto dari server disaring (T-75): hanya berkas /uploads/… yang ditampilkan. -->
        <div v-else-if="detailSendPhotos.length || canEditSentPhotos" class="space-y-2 rounded-xl border border-gray-200 bg-gray-50/50 p-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs font-medium text-gray-600">Foto saat dikirim dari {{ detail.from_warehouse_name }} ({{ detailSendPhotos.length }}):</p>
            <button v-if="canEditSentPhotos && !editingSendPhotos" type="button" @click="startEditSendPhotos"
              class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer">
              Edit / Hapus / Ganti Foto
            </button>
          </div>
          <div v-if="editingSendPhotos" class="space-y-2">
            <PhotoCapture multiple v-model="draftEditSendPhotos" label="Kelola Foto Pengiriman (Sebelum Diterima)"
              hint="Hapus foto dengan tombol × atau tambah/ganti foto baru, lalu simpan." />
            <div class="flex justify-end gap-2">
              <AppButton size="sm" variant="secondary" @click="editingSendPhotos = false">Batal</AppButton>
              <AppButton size="sm" :loading="savingSendPhotos" @click="saveEditedSendPhotos">Simpan Foto</AppButton>
            </div>
          </div>
          <div v-else-if="detailSendPhotos.length" class="flex flex-wrap gap-2">
            <a v-for="(u, idx) in detailSendPhotos" :key="`snd-${idx}`" :href="u" target="_blank" rel="noopener noreferrer">
              <img :src="u" :alt="`Bukti kirim ${idx + 1}`" class="h-20 w-20 rounded-lg border border-gray-200 object-cover hover:opacity-90" />
            </a>
          </div>
        </div>

        <!-- Actions: tombol hanya muncul bila status DAN izin memungkinkan. -->
        <div v-if="nextAction" class="flex justify-end">
          <AppActionButton v-if="nextAction.status !== 'received'" :icon="nextAction.status === 'approved' ? 'approve' : 'send'" :label="nextAction.label" :loading="saving" @click="doAction" />
          <AppButton v-else :loading="saving" @click="doAction">{{ nextAction.label }}</AppButton>
        </div>
        <div v-if="canCancelDetail">
          <AppActionButton icon="cancel" :label="(isOutletUser &amp;&amp; isCentralRequest(detail) ? 'Batalkan Permintaan' : 'Batalkan Transfer')" @click="doCancel" :disabled="saving" />
        </div>
      </div>
      <div v-else class="py-8 text-center text-gray-400">Memuat...</div>
      <template #footer>
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { stockUnitOptions, stockUnitRatio, preferredStockUnit, stockQtyText } from '@/utils/stockUnits.js'
import { ref, computed, watch, onMounted } from 'vue'
import { warehousesApi, stockItemsApi, stockTransfersApi } from '@/api/warehouse.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import PhotoCapture  from '@/components/PhotoCapture.vue'
import StockEvidenceCapture from '@/components/StockEvidenceCapture.vue'
import { safeUrl, safeUrls } from '@/utils/safeUrl.js'
import { todayDateString, formatDateTime } from '@/utils/format.js'
import { useRealtime } from '@/utils/realtime.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import { describeWarehouse, formatWarehouseOptionLabel, TRANSFER_STATUS_LABELS, transferStatusClass, isCentralRequest, transferRowStatusLabel, TRANSFER_DIVISIONS, transferDivisionClass } from '@/utils/warehouse.js'

const toast = useToastStore()
const auth = useAuthStore()
// Setujui, kirim/batalkan, dan terima adalah tiga izin berbeda — sama
// dengan mutasi aset. Tombol yang izinnya tidak ada tidak ditampilkan.
const canApprove = auth.hasPermission('stocktransfers.approve')
const canSend    = auth.hasPermission('stocktransfers.update')
const canReceive = auth.hasPermission('stocktransfers.receive') || auth.hasPermission('stockquality.central.receive') || auth.hasPermission('stockquality.outlet.receive')
const canReject = auth.hasPermission('stocktransfers.reject')
const receiveEvidences = ref([]), receiveNotes = ref('')
// Role per outlet boleh MEMINTA stok ke Gudang Induk (asal = Gudang Induk),
// tetapi menyetujui, mengirim, dan menerima di Gudang Induk adalah tahap milik
// role semua outlet — server menolaknya walau izinnya ada, jadi tombolnya
// tidak ditampilkan.
const isOutletUser = auth.scopeType === 'specific'
const sendPhotos = ref([])
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const transfers = ref([])
const page = ref(1)
const limit = 20
const totalPages = ref(1)
const filterStatus = ref('')
const filterWarehouse = ref('')
const filterDivision = ref('')
const filterDateFrom = ref('')
const filterDateTo = ref('')
const divisionFilterOptions = [
  { value: '', label: 'Semua Divisi' },
  ...TRANSFER_DIVISIONS,
]
function divisionActiveClass(div) {
  const map = {
    BAR: 'bg-purple-600 text-white border-purple-600',
    KITCHEN: 'bg-orange-600 text-white border-orange-600',
    SERVER: 'bg-cyan-600 text-white border-cyan-600',
  }
  return map[div] || 'bg-emerald-600 text-white border-emerald-600'
}
function clearDateFilter() {
  filterDateFrom.value = ''
  filterDateTo.value = ''
  applyFilters()
}
const detailSendPhotos = computed(() => safeUrls(detail.value?.photo_url, detail.value?.photo_urls))
const editingSendPhotos = ref(false)
const draftEditSendPhotos = ref([])
const savingSendPhotos = ref(false)
const canEditSentPhotos = computed(() =>
  detail.value?.status === 'sent' && canSend && !centralStageBlocked(detail.value, 'sent')
)
function startEditSendPhotos() {
  draftEditSendPhotos.value = [...detailSendPhotos.value]
  editingSendPhotos.value = true
}
async function saveEditedSendPhotos() {
  if (!draftEditSendPhotos.value.length) { toast.error('Minimal 1 foto pengiriman wajib dipertahankan'); return }
  savingSendPhotos.value = true
  try {
    detail.value = await stockTransfersApi.updatePhotos(detail.value.id, draftEditSendPhotos.value)
    editingSendPhotos.value = false
    toast.success('Foto pengiriman diperbarui')
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui foto pengiriman')
  } finally {
    savingSendPhotos.value = false
  }
}
function receiptEvidencesOf(receipt) {
  if (Array.isArray(receipt?.evidences) && receipt.evidences.length) return receipt.evidences
  return receipt?.evidence?.url ? [receipt.evidence] : []
}
const warehouseList = ref([])
// Pilihan gudang form buat transfer: untuk role per outlet ditambah Gudang
// Induk aktif (?include_central=1) — daftar filter tetap gudang outletnya.
const formWarehouseList = ref([])
const stockItems = ref([])
const loadingStockItems = ref(false)
const stockItemMap = computed(() => Object.fromEntries(stockItems.value.map(s => [s.id, s])))

const showCreate = ref(false)
const showDetail = ref(false)
const detail = ref(null)

const STATUSES = ['draft', 'approved', 'sent', 'partial_received', 'received', 'received_rejected', 'cancelled']
  .map(v => ({ value: v, label: TRANSFER_STATUS_LABELS[v] }))
const statusOptions = [
  { value: '', label: 'Semua Status' },
  ...STATUSES,
]

const NEXT_STATUS = { draft: 'approved', approved: 'sent', sent: 'received', partial_received: 'received' }
const NEXT_LABELS = { draft: 'Setujui Transfer', approved: 'Tandai Sudah Dikirim', sent: 'Simpan Pemeriksaan Penerimaan', partial_received: 'Simpan Pemeriksaan Berikutnya' }

const COLUMNS = [
  { key: 'transfer_number', label: 'No. Transfer', sortable: false },
  { key: 'route',           label: 'Rute',         sortable: false },
  { key: 'status',          label: 'Status',       sortable: false },
  { key: 'created_at',      label: 'Tanggal',      sortable: false },
  { key: 'created_by',      label: 'Dibuat oleh',  sortable: false },
  { key: 'actions',         label: '',             sortable: false },
]

const statusCls = transferStatusClass

const NEXT_PERM = { approved: canApprove, sent: canSend, received: canReceive }
// Tahap atas nama Gudang Induk (cermin centralTransferStageDenied di server).
function centralStageBlocked(t, stage) {
  if (stage === 'received' && t && !auth.hasPermission('stocktransfers.receive') && !auth.hasPermission(`stockquality.${t.to_warehouse_type}.receive`)) return true
  if (!isOutletUser || !t) return false
  if (stage === 'approved' || stage === 'sent') return isCentralRequest(t)
  if (stage === 'received') return t.to_warehouse_type === 'central'
  return false
}
const nextAction = computed(() => {
  if (!detail.value) return null
  const ns = NEXT_STATUS[detail.value.status]
  if (!ns || !NEXT_PERM[ns] || centralStageBlocked(detail.value, ns)) return null
  return { status: ns, label: NEXT_LABELS[detail.value.status] }
})
const isSending = computed(() => detail.value?.status === 'approved' && canSend && !centralStageBlocked(detail.value, 'sent'))
const isReceiving = computed(() => ['sent', 'partial_received'].includes(detail.value?.status) && canReceive && !centralStageBlocked(detail.value, 'received'))
// Batal: draft/disetujui. Permintaan ke Gudang Induk hanya bisa ditarik outlet
// selagi draft; setelah disetujui pembatalan menjadi wewenang Gudang Induk.
const canCancelDetail = computed(() => {
  const t = detail.value
  if (!t || !canSend || !['draft', 'approved'].includes(t.status)) return false
  return !(isOutletUser && isCentralRequest(t) && t.status !== 'draft')
})
const requestNotice = computed(() => {
  const t = detail.value
  if (!t || !isOutletUser) return ''
  if (isCentralRequest(t)) {
    if (t.status === 'draft') return 'Menunggu Gudang Induk menyetujui permintaan ini. Gudang Induk akan menyetujui dan mengirim barangnya.'
    if (t.status === 'approved') return 'Disetujui Gudang Induk — menunggu barang dikirim.'
    if (t.status === 'sent') return 'Barang sudah dikirim Gudang Induk. Periksa jumlah yang tiba, lalu konfirmasi penerimaan.'
  }
  if (t.to_warehouse_type === 'central' && t.status === 'sent') return 'Penerimaan di Gudang Induk dicatat oleh Gudang Induk.'
  return ''
})

const EMPTY_FORM = () => ({ from_warehouse_id: '', to_warehouse_id: '', target_division: '', transfer_date: todayDateString(), notes: '', items: [] })
const form = ref(EMPTY_FORM())

const warehouseFilterOptions = computed(() =>
  [
    { value: '', label: 'Semua Gudang' },
    ...warehouseList.value.map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) })),
  ]
)

// Gagal memuat daftar form → pakai daftar biasa (toast sudah dari loadWarehouses).
const formWarehouses = computed(() => formWarehouseList.value.length ? formWarehouseList.value : warehouseList.value)
const formWarehouseMap = computed(() => Object.fromEntries(formWarehouses.value.map(w => [w.id, w])))

const fromWarehouseOptions = computed(() =>
  formWarehouses.value
    .filter(w => w.id !== form.value.to_warehouse_id)
    .map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) }))
)

// Role per outlet: tujuan hanya gudang outlet dalam cakupannya — Gudang Induk
// → outlet lain dan antar Gudang Induk ditolak server.
const toWarehouseOptions = computed(() =>
  formWarehouses.value
    .filter(w => w.id !== form.value.from_warehouse_id)
    .filter(w => !isOutletUser || w.type === 'outlet')
    .map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) }))
)

const selectedFromWarehouse = computed(() => formWarehouseMap.value[form.value.from_warehouse_id] || null)
const selectedToWarehouse = computed(() => formWarehouseMap.value[form.value.to_warehouse_id] || null)
// Outlet memilih Gudang Induk sebagai asal = permintaan stok.
const isRequestForm = computed(() => isOutletUser && selectedFromWarehouse.value?.type === 'central')

let loadSeq = 0
async function load({ silent = false } = {}) {
  const seq = ++loadSeq
  if (!silent) {
    loading.value = true
    errorMsg.value = ''
  }
  try {
    const data = await stockTransfersApi.list({
      page: page.value, limit,
      status: filterStatus.value,
      warehouse_id: filterWarehouse.value,
      division: filterDivision.value,
      date_from: filterDateFrom.value,
      date_to: filterDateTo.value,
    })
    if (seq !== loadSeq) return // respons lama, sudah ada request lebih baru
    transfers.value = data.data || []
    totalPages.value = data.total_pages || 1
  } catch (e) {
    if (!silent) errorMsg.value = e?.message || 'Gagal memuat data'
  } finally {
    if (!silent) loading.value = false
  }
}

function applyFilters() {
  // watch(page) yang memanggil load; hindari request ganda saat pindah ke hal. 1
  if (page.value === 1) load()
  else page.value = 1
}

async function loadWarehouses() {
  try {
    const data = await warehousesApi.list({ limit: 200 })
    warehouseList.value = data.data || []
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat daftar gudang')
  }
}

async function loadFormWarehouses() {
  try {
    const data = await warehousesApi.list({ limit: 200, include_central: 1 })
    formWarehouseList.value = data.data || []
  } catch (e) {
    formWarehouseList.value = []
  }
}

const fmtQtyNum = (v) => new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(Number(v || 0))

function formatSourceStock(item) {
  if (!item) return '0'
  const qtyBase = Number(item.warehouse_qty || 0)
  const whType = selectedFromWarehouse.value?.type
  const primary = stockQtyText(item, qtyBase, whType)
  const unit = preferredStockUnit(item, whType)
  if (unit && unit !== item.base_unit && item.base_unit && qtyBase !== 0) {
    return `${primary} (${fmtQtyNum(qtyBase)} ${item.base_unit})`
  }
  return primary
}

function itemExceedsSourceStock(it) {
  const s = stockItemMap.value[it?.item_id]
  if (!selectedFromWarehouse.value || !s || !it?.qty_dist || it.qty_dist <= 0) return false
  const needBase = Number(it.qty_dist) * stockUnitRatio(s, it.unit_used)
  return needBase > Number(s.warehouse_qty || 0) + 1e-6
}

const availableStockCount = computed(() =>
  stockItems.value.filter(s => Number(s.warehouse_qty || 0) > 0).length
)

const stockItemOptions = computed(() => {
  const hasWh = Boolean(selectedFromWarehouse.value)
  const list = [...stockItems.value]
  if (hasWh) {
    list.sort((a, b) => {
      const aAvail = Number(a.warehouse_qty || 0) > 0 ? 1 : 0
      const bAvail = Number(b.warehouse_qty || 0) > 0 ? 1 : 0
      if (aAvail !== bAvail) return bAvail - aAvail
      return String(a.name || '').localeCompare(String(b.name || ''), 'id', { sensitivity: 'base' })
    })
  }
  return list.map(s => {
    const base = `${s.code} — ${s.name}`
    if (!hasWh) return { value: s.id, label: base }
    const qtyBase = Number(s.warehouse_qty || 0)
    return {
      value: s.id,
      label: qtyBase > 0 ? `${base} · Stok: ${formatSourceStock(s)}` : `${base} · Stok: 0 ${s.base_unit || ''}`.trim(),
    }
  })
})

let stockItemsSeq = 0
async function loadStockItems(warehouseId = form.value.from_warehouse_id) {
  const seq = ++stockItemsSeq
  loadingStockItems.value = true
  try {
    const params = { active_only: 'true' }
    if (warehouseId) params.warehouse_id = warehouseId
    const items = await stockItemsApi.listAll(params)
    if (seq !== stockItemsSeq) return
    stockItems.value = items
  } catch (e) {
    if (seq === stockItemsSeq) {
      toast.error(e?.message || 'Gagal memuat daftar bahan baku')
    }
  } finally {
    if (seq === stockItemsSeq) loadingStockItems.value = false
  }
}

watch(page, load)
watch(() => [showCreate.value, form.value.from_warehouse_id], ([open, nextValue]) => {
  if (nextValue && nextValue === form.value.to_warehouse_id) {
    form.value.to_warehouse_id = ''
  }
  if (open) {
    loadStockItems(nextValue)
  }
})
watch(() => form.value.to_warehouse_id, (nextValue) => {
  if (nextValue && nextValue === form.value.from_warehouse_id) {
    form.value.from_warehouse_id = ''
  }
  const wh = formWarehouseMap.value[nextValue]
  if (wh && wh.type !== 'outlet') {
    form.value.target_division = ''
  }
})
onMounted(() => { load(); loadWarehouses(); loadFormWarehouses(); loadStockItems() })
useRealtime(['stock_transfer', 'stock_quality'], async () => {
  await load({ silent: true })
  if (showDetail.value && detail.value?.id && !saving.value && !isReceiving.value && !isSending.value) {
    try { detail.value = await stockTransfersApi.get(detail.value.id) } catch {}
  }
})

function addItem() {
  form.value.items.push({ item_id: '', qty_dist: 0, unit_used: '', notes: '', dist_ratio: 1, base_unit: '', unit_label: '' })
}
function removeItem(i) {
  form.value.items.splice(i, 1)
}
function onItemSelect(it) {
  const s = stockItemMap.value[it.item_id]
  if (s) {
    it.dist_ratio = s.dist_ratio
    it.base_unit = s.base_unit
    it.unit_label = s.dist_unit_label || s.dist_unit
    it.unit_used = preferredStockUnit(s, selectedFromWarehouse.value?.type)
  }
}

function openCreate() {
  form.value = EMPTY_FORM()
  // Role per outlet: bentuk paling umum = minta dari Gudang Induk ke gudang
  // outletnya sendiri — terisi otomatis bila pilihannya tunggal.
  if (isOutletUser) {
    const centrals = formWarehouses.value.filter(w => w.type === 'central')
    const own = formWarehouses.value.filter(w => w.type === 'outlet')
    if (centrals.length === 1) form.value.from_warehouse_id = centrals[0].id
    if (own.length === 1) form.value.to_warehouse_id = own[0].id
  }
  showCreate.value = true
}

async function submitCreate() {
  if (!form.value.from_warehouse_id || !form.value.to_warehouse_id) {
    toast.error('Pilih gudang asal dan tujuan')
    return
  }
  if (form.value.from_warehouse_id === form.value.to_warehouse_id) {
    toast.error('Gudang asal dan tujuan tidak boleh sama')
    return
  }
  if (form.value.items.length === 0) {
    toast.error('Tambahkan minimal 1 item')
    return
  }
  if (form.value.items.some(it => !it.item_id || !it.qty_dist || it.qty_dist <= 0)) {
    toast.error('Setiap item transfer harus memiliki bahan baku dan qty lebih dari 0')
    return
  }
  saving.value = true
  const isRequest = isRequestForm.value
  try {
    await stockTransfersApi.create(form.value)
    toast.success(isRequest ? 'Permintaan stok dikirim ke Gudang Induk' : 'Transfer berhasil dibuat')
    showCreate.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan')
  } finally {
    saving.value = false
  }
}

async function openDetail(id) {
  detail.value = null
  sendPhotos.value = []
  editingSendPhotos.value = false
  showDetail.value = true
  try {
    const data = await stockTransfersApi.get(id)
    detail.value = data
    initReceiptInputs()
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat detail')
    showDetail.value = false
  }
}

function acceptedTotal(it) {
  if (detail.value?.receipt_version > 0) return it.accepted_qty_base || 0
  return detail.value?.status === 'received' ? (it.received_qty_base ?? it.qty_base) : 0
}
function remaining(it) { return Math.max(0, Number((it.qty_base - acceptedTotal(it) - (it.rejected_qty_base || 0)).toFixed(4))) }
function initReceiptInputs() {
  receiveEvidences.value = []; receiveNotes.value = ''
  detail.value.items.forEach(it => { it._received_qty = remaining(it); it._rejected_qty = 0; it._reject_reason = '' })
}
function rejectRemaining() { detail.value.items.forEach(it => { it._received_qty = 0; it._rejected_qty = remaining(it) }) }

async function doAction() {
  if (!nextAction.value) return
  if (nextAction.value.status === 'sent' && !sendPhotos.value.length) {
    toast.error('Foto barang saat dikirim wajib diunggah')
    return
  }
  if (nextAction.value.status === 'received' && !receiveEvidences.value.length) return toast.error('Foto pemeriksaan bertimestamp wajib diunggah')
  saving.value = true
  try {
    const data = nextAction.value.status === 'received'
      ? await stockTransfersApi.receive(detail.value.id, {
        version: detail.value.receipt_version || 0,
        evidence_id: receiveEvidences.value[0].id,
        evidence_ids: receiveEvidences.value.map(e => e.id),
        notes: receiveNotes.value,
        items: detail.value.items.map(it => ({ item_id: it.item_id, accepted_qty_base: Number(it._received_qty || 0), rejected_qty_base: Number(it._rejected_qty || 0), reason: it._reject_reason || '' })),
      })
      : await stockTransfersApi.updateStatus(detail.value.id, nextAction.value.status, sendPhotos.value)
    detail.value = data
    sendPhotos.value = []
    initReceiptInputs()
    toast.success('Status transfer diperbarui')
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui status')
  } finally {
    saving.value = false
  }
}

async function doCancel() {
  saving.value = true
  try {
    const data = await stockTransfersApi.updateStatus(detail.value.id, 'cancelled')
    detail.value = data
    toast.success('Transfer dibatalkan')
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal membatalkan')
  } finally {
    saving.value = false
  }
}
</script>
