<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Pengantaran Logistik</h1>
        <p class="mt-0.5 text-sm text-gray-500">
          Kelola pengantaran barang Purchasing per item (bisa lintas beberapa PR dalam 1 Surat Jalan), cetak Surat Jalan, dan sinkronisasi penerimaan Gudang / Aset.
        </p>
      </div>
      <AppButton v-if="canManage" @click="openCreate()">+ Buat Pengantaran / Surat Jalan</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Kelompok Purchasing Switcher / Locked Mode Banner -->
    <div v-if="canSeeBothGroups" class="flex flex-wrap items-center gap-2 rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 shadow-2xs">
      <span class="text-xs font-bold uppercase tracking-wide text-gray-500 mr-1">Kelompok Purchasing:</span>
      <button type="button" @click="selectGroup('')"
        :class="['rounded-full border px-3 py-1 text-xs font-semibold transition cursor-pointer',
          activeGroup === '' ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-gray-200 bg-gray-50 text-gray-700 hover:bg-gray-100']">
        Semua Kelompok
      </button>
      <button type="button" @click="selectGroup('bahan_baku')"
        :class="['inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold transition cursor-pointer',
          activeGroup === 'bahan_baku' ? 'border-amber-600 bg-amber-600 text-white' : 'border-amber-200 bg-amber-50 text-amber-900 hover:bg-amber-100']">
        <span class="h-1.5 w-1.5 rounded-full" :class="activeGroup === 'bahan_baku' ? 'bg-white' : 'bg-amber-500'" />
        1. Purchasing Bahan Baku
      </button>
      <button type="button" @click="selectGroup('barang_jasa')"
        :class="['inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold transition cursor-pointer',
          activeGroup === 'barang_jasa' ? 'border-blue-600 bg-blue-600 text-white' : 'border-blue-200 bg-blue-50 text-blue-900 hover:bg-blue-100']">
        <span class="h-1.5 w-1.5 rounded-full" :class="activeGroup === 'barang_jasa' ? 'bg-white' : 'bg-blue-500'" />
        2. Purchasing Barang &amp; Jasa
      </button>
    </div>
    <div v-else-if="lockedGroup"
      :class="lockedGroup === 'bahan_baku' ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-blue-200 bg-blue-50 text-blue-900'"
      class="flex flex-wrap items-center justify-between gap-2 rounded-xl border px-3.5 py-2.5 text-xs">
      <div class="flex items-center gap-2">
        <span :class="lockedGroup === 'bahan_baku' ? 'bg-amber-500' : 'bg-blue-500'" class="h-2 w-2 rounded-full shrink-0" />
        <span class="font-bold">Mode Kelompok Terpisah: {{ lockedGroup === 'bahan_baku' ? 'Kelompok 1 · Purchasing Bahan Baku' : 'Kelompok 2 · Purchasing Barang & Jasa' }}</span>
      </div>
      <span class="opacity-80">Surat Jalan dan barang siap antar otomatis disaring khusus kelompok ini agar tidak tercampur.</span>
    </div>

    <!-- Tabs -->
    <div class="flex flex-wrap items-center gap-2 border-b border-gray-200 pb-2">
      <button type="button" @click="activeTab = 'deliveries'"
        :class="['rounded-lg px-3.5 py-2 text-sm font-semibold transition cursor-pointer',
          activeTab === 'deliveries' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-white text-gray-600 hover:bg-gray-100 border border-gray-200']">
        Daftar Surat Jalan
        <span class="ml-1 rounded-full px-1.5 py-0.5 text-xs" :class="activeTab === 'deliveries' ? 'bg-white/20 text-white' : 'bg-gray-100 text-gray-600'">
          {{ totalDeliveries }}
        </span>
      </button>
      <button type="button" @click="switchToReadyTab"
        :class="['rounded-lg px-3.5 py-2 text-sm font-semibold transition cursor-pointer',
          activeTab === 'ready' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-white text-gray-600 hover:bg-gray-100 border border-gray-200']">
        Barang Purchasing Siap Antar
        <span class="ml-1 rounded-full px-1.5 py-0.5 text-xs" :class="activeTab === 'ready' ? 'bg-white/20 text-white' : 'bg-amber-100 text-amber-800'">
          {{ dispatchableItems.length }}
        </span>
      </button>
    </div>

    <!-- ═══ TAB 1: DAFTAR SURAT JALAN ═══ -->
    <AppCard v-if="activeTab === 'deliveries'" :padding="false">
      <div class="flex flex-wrap items-center gap-2.5 border-b border-gray-100 px-4 py-3">
        <input v-model="filterQ" type="search" placeholder="Cari No. SJ, No. PR, kurir, barang…"
          class="w-full sm:w-56 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-400/30"
          @input="debouncedLoad" />
        <select v-model="filterStatus" @change="applyFilters"
          class="w-full sm:w-auto rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none">
          <option value="">Semua Status</option>
          <option value="draft">Draft</option>
          <option value="in_transit">Dalam Pengantaran</option>
          <option value="partial_received">Diterima Sebagian</option>
          <option value="received">Diterima</option>
          <option value="cancelled">Dibatalkan</option>
        </select>
        <select v-model="filterMode" @change="applyFilters"
          class="w-full sm:w-auto rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none">
          <option value="">Semua Metode</option>
          <option value="logistics">Petugas Logistik</option>
          <option value="vendor_direct">Vendor Langsung</option>
        </select>
        <div class="flex flex-wrap items-center gap-1.5 w-full sm:w-auto">
          <input v-model="filterDateFrom" type="date" aria-label="Dari tanggal" @change="applyFilters"
            class="rounded-lg border border-gray-300 bg-white px-2.5 py-2 text-sm focus:border-emerald-500 focus:outline-none" />
          <span class="text-xs text-gray-400">s/d</span>
          <input v-model="filterDateTo" type="date" aria-label="Sampai tanggal" @change="applyFilters"
            class="rounded-lg border border-gray-300 bg-white px-2.5 py-2 text-sm focus:border-emerald-500 focus:outline-none" />
          <button v-if="filterDateFrom || filterDateTo" type="button" @click="filterDateFrom = ''; filterDateTo = ''; applyFilters()"
            class="text-xs font-medium text-gray-500 hover:text-red-600 px-1.5">Reset</button>
        </div>
      </div>

      <!-- Mobile list -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!deliveries.length" class="p-8 text-center text-sm text-gray-400">Belum ada pengantaran logistik.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="d in deliveries" :key="d.id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="space-y-0.5">
                <button type="button" @click="openDetail(d.id)" class="font-mono text-sm font-bold text-emerald-700 hover:underline text-left">
                  {{ d.delivery_number }}
                </button>
                <span v-if="d.purchasing_group" :class="purchasingGroupBadgeClass(d.purchasing_group)"
                  class="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold">
                  {{ purchasingGroupShort(d.purchasing_group) }}
                </span>
              </div>
              <span :class="deliveryStatusClass(d.status)" class="shrink-0 whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-semibold">
                {{ deliveryStatusLabel(d.status) }}
              </span>
            </div>
            <p class="text-xs text-gray-700">
              <span class="font-semibold">{{ dispatchModeLabel(d.dispatch_mode) }}</span>
              <template v-if="d.courier_name"> · {{ d.courier_name }}</template>
              <template v-if="d.vehicle_info"> ({{ d.vehicle_info }})</template>
            </p>
            <p class="text-xs text-gray-600">Tujuan: <strong>{{ d.destination_label || '-' }}</strong></p>
            <p class="text-xs text-gray-500">
              {{ d.delivery_date }} · {{ d.item_count }} item ({{ d.total_received }}/{{ d.total_planned }} unit diterima)
              <span v-if="d.pr_numbers?.length" class="block font-mono text-[11px] text-gray-400 mt-0.5">{{ d.pr_numbers.join(', ') }}</span>
            </p>
            <div class="flex flex-wrap gap-2 pt-1">
              <AppActionButton icon="view" label="Detail" @click="openDetail(d.id)" />
              <AppActionButton icon="print" label="Cetak SJ" @click="quickPrint(d.id)" />
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop table -->
      <AppTable class="hidden sm:block" :columns="DELIVERY_COLS" :rows="deliveries" :loading="loading" emptyText="Belum ada pengantaran logistik.">
        <template #cell-delivery_number="{ row }">
          <button type="button" @click="openDetail(row.id)" class="font-mono text-sm font-bold text-emerald-700 hover:underline">
            {{ row.delivery_number }}
          </button>
          <div class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <span v-if="row.purchasing_group" :class="purchasingGroupBadgeClass(row.purchasing_group)"
              class="inline-flex items-center rounded-full border px-1.5 py-0.5 text-[10px] font-semibold">
              {{ purchasingGroupShort(row.purchasing_group) }}
            </span>
            <span class="text-[11px] text-gray-400">{{ row.delivery_date }}</span>
          </div>
        </template>
        <template #cell-courier="{ row }">
          <span :class="row.dispatch_mode === 'logistics' ? 'bg-indigo-50 text-indigo-700' : 'bg-teal-50 text-teal-700'"
            class="inline-block rounded px-1.5 py-0.5 text-[10px] font-semibold">
            {{ dispatchModeLabel(row.dispatch_mode) }}
          </span>
          <p class="mt-0.5 text-sm font-medium text-gray-800">{{ row.courier_name || (row.dispatch_mode === 'vendor_direct' ? 'Pengantar Vendor' : '—') }}</p>
          <p v-if="row.vehicle_info" class="text-[11px] text-gray-400">{{ row.vehicle_info }}</p>
        </template>
        <template #cell-destination="{ row }">
          <p class="text-sm font-medium text-gray-800">{{ row.destination_label || '—' }}</p>
          <p v-if="row.origin_label" class="text-[11px] text-gray-400">Dari: {{ row.origin_label }}</p>
        </template>
        <template #cell-manifest="{ row }">
          <p class="text-sm text-gray-800">
            <strong>{{ row.item_count }}</strong> jenis · <strong>{{ row.total_received }}/{{ row.total_planned }}</strong> unit
          </p>
          <p v-if="row.pr_numbers?.length" class="font-mono text-[11px] text-gray-500 truncate max-w-[220px]" :title="row.pr_numbers.join(', ')">
            {{ row.pr_numbers.join(', ') }}
          </p>
        </template>
        <template #cell-status="{ row }">
          <span :class="deliveryStatusClass(row.status)" class="whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-semibold">
            {{ deliveryStatusLabel(row.status) }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1.5">
            <AppActionButton icon="print" label="Cetak Surat Jalan" @click="quickPrint(row.id)" />
            <AppActionButton icon="view" label="Detail" @click="openDetail(row.id)" />
          </div>
        </template>
      </AppTable>
      <AppPagination v-model:page="page" :total-pages="totalPages" class="border-t border-gray-100 px-4 py-3" />
    </AppCard>

    <!-- ═══ TAB 2: BARANG PURCHASING SIAP ANTAR ═══ -->
    <AppCard v-else :padding="false">
      <div class="flex flex-wrap items-center justify-between gap-2.5 border-b border-gray-100 px-4 py-3">
        <div class="flex flex-wrap items-center gap-2 flex-1">
          <input v-model="readyQ" type="search" placeholder="Cari No. PR, vendor, nama barang…"
            class="w-full sm:w-64 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none"
            @input="debouncedLoadReady" />
          <select v-if="canSeeBothGroups && !activeGroup" v-model="readyReqType" @change="loadDispatchable"
            class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm">
            <option value="">Semua Kelompok</option>
            <option value="bahan">Kel. 1 · Bahan Baku</option>
            <option value="barang">Kel. 2 · Barang &amp; Perlengkapan</option>
          </select>
          <select v-model="readyKind" @change="loadDispatchable"
            class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm">
            <option value="">Semua Kategori</option>
            <option value="dapur">Bahan / Dapur</option>
            <option value="perlengkapan">Perlengkapan / Aset</option>
            <option value="projek">Material Projek</option>
          </select>
        </div>
        <div v-if="canManage && selectedReadyKeys.length" class="flex items-center gap-2">
          <span class="text-xs font-semibold text-emerald-800">{{ selectedReadyKeys.length }} item dipilih</span>
          <AppButton size="sm" @click="createFromSelectedReady">+ Buat Surat Jalan</AppButton>
        </div>
      </div>

      <div v-if="loadingReady" class="p-6 text-center text-sm text-gray-400">Memuat barang siap antar…</div>
      <div v-else-if="!dispatchableItems.length" class="p-8 text-center text-sm text-gray-400">
        Tidak ada sisa barang Purchasing yang belum masuk pengantaran atau diterima.
      </div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full text-sm divide-y divide-gray-100">
          <thead class="bg-gray-50 text-xs text-gray-500">
            <tr>
              <th v-if="canManage" class="w-10 px-3 py-2.5 text-center">
                <input type="checkbox" :checked="allReadySelected" @change="toggleAllReady" class="h-4 w-4 rounded border-gray-300 text-emerald-600" />
              </th>
              <th class="px-3 py-2.5 text-left">Pengadaan &amp; Vendor</th>
              <th class="px-3 py-2.5 text-left">Barang</th>
              <th class="px-3 py-2.5 text-left">Kategori</th>
              <th class="px-3 py-2.5 text-right">Beli</th>
              <th class="px-3 py-2.5 text-right">Diterima</th>
              <th class="px-3 py-2.5 text-right">Dalam SJ</th>
              <th class="px-3 py-2.5 text-right">Siap Antar</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="it in dispatchableItems" :key="readyKey(it)" class="hover:bg-gray-50">
              <td v-if="canManage" class="px-3 py-2.5 text-center">
                <input type="checkbox" :value="readyKey(it)" v-model="selectedReadyKeys" class="h-4 w-4 rounded border-gray-300 text-emerald-600" />
              </td>
              <td class="px-3 py-2.5">
                <p class="font-mono text-xs font-bold text-gray-900">{{ it.pr_number }}</p>
                <p class="text-xs text-gray-500">{{ it.vendor_name || '—' }}<span v-if="it.outlet_name"> · {{ it.outlet_name }}</span></p>
              </td>
              <td class="px-3 py-2.5">
                <p class="font-medium text-gray-900">{{ it.item_name }}</p>
                <p v-if="it.target_warehouse_name" class="text-[11px] text-gray-400">Kebutuhan: {{ it.target_warehouse_name }}</p>
              </td>
              <td class="px-3 py-2.5">
                <span :class="it.line_kind === 'dapur' ? 'bg-amber-50 text-amber-800' : (it.line_kind === 'projek' ? 'bg-purple-50 text-purple-700' : 'bg-blue-50 text-blue-700')"
                  class="rounded px-2 py-0.5 text-[11px] font-semibold">
                  {{ LINE_KIND_LABELS[it.line_kind] || it.line_kind }}
                </span>
              </td>
              <td class="px-3 py-2.5 text-right text-xs text-gray-600">{{ it.qty_purchased }} {{ it.unit }}</td>
              <td class="px-3 py-2.5 text-right text-xs text-gray-500">{{ it.qty_received }}</td>
              <td class="px-3 py-2.5 text-right text-xs text-amber-700">{{ it.qty_in_delivery || 0 }}</td>
              <td class="px-3 py-2.5 text-right font-bold text-emerald-700">{{ it.qty_available }} {{ it.unit }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </AppCard>

    <!-- ═══ MODAL BUAT / EDIT PENGANTARAN & SURAT JALAN ═══ -->
    <AppModal v-model="showForm" :title="editingId ? `Edit Surat Jalan ${editingNumber}` : 'Buat Pengantaran Logistik / Surat Jalan'" size="2xl">
      <div class="space-y-4">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Tanggal Pengantaran</label>
            <input v-model="form.delivery_date" type="date" class="fld" />
          </div>
          <div>
            <label class="lbl">Metode Pengantaran</label>
            <div class="flex gap-2">
              <button type="button" @click="form.dispatch_mode = 'logistics'"
                :class="['flex-1 rounded-lg border px-3 py-2 text-xs font-semibold transition cursor-pointer',
                  form.dispatch_mode === 'logistics' ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50']">
                Petugas Logistik
              </button>
              <button type="button" @click="form.dispatch_mode = 'vendor_direct'"
                :class="['flex-1 rounded-lg border px-3 py-2 text-xs font-semibold transition cursor-pointer',
                  form.dispatch_mode === 'vendor_direct' ? 'border-teal-600 bg-teal-600 text-white' : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50']">
                Vendor Langsung
              </button>
            </div>
          </div>
          <div>
            <label class="lbl">{{ form.dispatch_mode === 'logistics' ? 'Nama Petugas Logistik / Kurir' : 'Nama Pengantar / Ekspedisi Vendor' }}</label>
            <input v-model="form.courier_name" class="fld" :placeholder="form.dispatch_mode === 'logistics' ? 'mis. Budi Logistik' : 'mis. Kurir PT Sumber Pangan'" />
          </div>
          <div>
            <label class="lbl">Kendaraan / Plat (opsional)</label>
            <input v-model="form.vehicle_info" class="fld" placeholder="mis. Granmax B 1234 XY / Motor" />
          </div>
          <div>
            <label class="lbl">Asal / Pengirim (opsional)</label>
            <input v-model="form.origin_label" class="fld" placeholder="mis. Purchasing / Vendor Pasar Induk" />
          </div>
          <div>
            <label class="lbl">Tipe Tujuan Pengantaran</label>
            <select v-model="form.destination_type" class="fld" @change="onDestTypeChange">
              <option value="warehouse">Gudang (Gudang Induk / Outlet)</option>
              <option value="outlet">Outlet</option>
              <option value="asset_desk">Bagian Perlengkapan / Aset</option>
            </select>
          </div>
          <div v-if="form.destination_type === 'warehouse'">
            <label class="lbl">Pilih Gudang Tujuan</label>
            <SearchSelect v-model="form.destination_warehouse_id" :options="warehouses" placeholder="Pilih gudang…" @change="syncDestLabel" />
          </div>
          <div v-else-if="form.destination_type === 'outlet'">
            <label class="lbl">Pilih Outlet Tujuan</label>
            <SearchSelect v-model="form.destination_outlet_id" :options="outlets" placeholder="Pilih outlet…" @change="syncDestLabel" />
          </div>
          <div>
            <label class="lbl">Nama / Keterangan Tujuan (tampil di Surat Jalan)</label>
            <input v-model="form.destination_label" class="fld" placeholder="mis. Gudang Induk Pusat / Bagian Aset" />
          </div>
        </div>

        <div>
          <label class="lbl">Catatan Pengantaran (opsional)</label>
          <input v-model="form.notes" class="fld" placeholder="mis. Barang dingin didahulukan, struk asli dibawa kurir" />
        </div>

        <!-- Daftar Item yang Dibawa (Editable saat Create atau Edit Draft) -->
        <div v-if="!editingId || editingStatus === 'draft'" class="space-y-2.5 rounded-xl border border-gray-200 bg-gray-50/70 p-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <p class="text-sm font-bold text-gray-800">Daftar Barang yang Dibawa ({{ form.items.length }} item)</p>
              <p class="text-[11px] text-gray-500">
                Pilih item dari daftar Purchasing di bawah — bisa gabungkan beberapa PR dalam 1 Surat Jalan <strong>dalam kelompok Purchasing yang sama</strong>.
              </p>
              <span v-if="formLockedGroup" :class="purchasingGroupBadgeClass(formLockedGroup)"
                class="mt-1 inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold">
                Kelompok SJ: {{ purchasingGroupLabel(formLockedGroup) }}
              </span>
            </div>
            <button type="button" @click="showPicker = !showPicker"
              class="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700 cursor-pointer">
              {{ showPicker ? 'Tutup Pencarian Item' : '+ Pilih Item Purchasing' }}
            </button>
          </div>

          <!-- Inline Picker dari Barang Purchasing Siap Antar -->
          <div v-if="showPicker" class="rounded-lg border border-emerald-200 bg-white p-2.5 space-y-2">
            <div class="flex flex-wrap gap-2">
              <input v-model="pickerQ" type="search" placeholder="Cari No. PR, vendor, atau nama barang…" class="fld flex-1 min-w-[200px]" />
              <select v-model="pickerKind" class="fld w-auto">
                <option value="">Semua Kategori</option>
                <option value="dapur">Bahan / Dapur</option>
                <option value="perlengkapan">Perlengkapan / Aset</option>
                <option value="projek">Material Projek</option>
              </select>
            </div>
            <div class="max-h-52 overflow-y-auto divide-y divide-gray-100">
              <div v-if="!filteredPickerItems.length" class="py-4 text-center text-xs text-gray-400">
                Tidak ada item tersedia yang cocok.
              </div>
              <div v-for="cand in filteredPickerItems" :key="readyKey(cand)"
                class="flex flex-wrap items-center justify-between gap-2 py-2 px-1 text-xs hover:bg-emerald-50/40">
                <div class="min-w-0 flex-1">
                  <p class="font-semibold text-gray-900">{{ cand.item_name }}
                    <span class="ml-1 font-normal text-gray-400">({{ LINE_KIND_LABELS[cand.line_kind] || cand.line_kind }})</span>
                  </p>
                  <p class="text-[11px] text-gray-500">
                    <span class="font-mono font-semibold">{{ cand.pr_number }}</span> · {{ cand.vendor_name || 'Tanpa vendor' }} ·
                    <strong class="text-emerald-700">Siap antar {{ cand.qty_available }} {{ cand.unit }}</strong>
                  </p>
                </div>
                <button type="button" :disabled="isInFormItems(cand)" @click="addItemToForm(cand)"
                  class="rounded-md px-2.5 py-1 text-xs font-semibold transition cursor-pointer"
                  :class="isInFormItems(cand) ? 'bg-gray-100 text-gray-400 cursor-not-allowed' : 'bg-emerald-600 text-white hover:bg-emerald-700'">
                  {{ isInFormItems(cand) ? 'Sudah dipilih' : '+ Tambah' }}
                </button>
              </div>
            </div>
          </div>

          <!-- Selected items table -->
          <div v-if="!form.items.length" class="rounded-lg border border-dashed border-gray-300 bg-white py-4 text-center text-xs text-gray-400">
            Belum ada barang dipilih. Klik "+ Pilih Item Purchasing".
          </div>
          <div v-else class="space-y-2">
            <div v-for="(it, idx) in form.items" :key="`${it.purchase_request_id}:${it.pr_item_key}`"
              class="flex flex-wrap items-center gap-2 rounded-lg border border-gray-200 bg-white p-2.5 text-xs">
              <div class="min-w-0 flex-1">
                <p class="font-semibold text-gray-900">{{ it.item_name }}</p>
                <p class="text-[11px] text-gray-500">
                  <span class="font-mono">{{ it.pr_number }}</span>
                  <template v-if="it.vendor_name"> · {{ it.vendor_name }}</template>
                  · Maks {{ it.max_qty }} {{ it.unit }}
                </p>
              </div>
              <div class="flex items-center gap-1.5">
                <label class="text-[11px] text-gray-400">Qty</label>
                <input v-model.number="it.qty_planned" type="number" min="1" :max="it.max_qty"
                  class="w-20 rounded border border-gray-300 px-2 py-1 text-right text-sm font-semibold" />
                <span class="text-gray-500">{{ it.unit }}</span>
              </div>
              <input v-model="it.notes" placeholder="Catatan item…" class="w-full sm:w-40 rounded border border-gray-200 px-2 py-1 text-xs" />
              <AppActionButton icon="delete" label="Hapus" @click="form.items.splice(idx, 1)" />
            </div>
          </div>
        </div>

        <!-- Multi-foto Pengantaran -->
        <div class="rounded-xl border border-emerald-200 bg-emerald-50/40 p-3">
          <PhotoCapture multiple :required="false" v-model="form.pickup_photo_urls"
            label="Foto Barang Saat Diambil / Disiapkan (opsional, bisa banyak)"
            hint="Foto masih dapat ditambah, dihapus per foto, atau diganti selama pengantaran belum berstatus Diterima." />
        </div>
      </div>

      <template #footer>
        <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <AppButton variant="secondary" @click="showForm = false">Batal</AppButton>
          <template v-if="!editingId">
            <AppButton variant="secondary" :loading="saving" @click="submitForm(false)">Simpan Draft</AppButton>
            <AppButton :loading="saving" @click="submitForm(true)">Simpan &amp; Berangkatkan</AppButton>
          </template>
          <AppButton v-else :loading="saving" @click="submitForm(false)">Simpan Perubahan</AppButton>
        </div>
      </template>
    </AppModal>

    <!-- ═══ MODAL DETAIL PENGANTARAN & SURAT JALAN ═══ -->
    <AppModal v-model="showDetail" :title="detail ? `Surat Jalan ${detail.delivery_number}` : 'Detail Pengantaran'" size="2xl">
      <div v-if="loadingDetail" class="py-8 text-center text-sm text-gray-400">Memuat rincian…</div>
      <div v-else-if="detail" class="space-y-4">
        <!-- Top Status Bar & Cetak Surat Jalan -->
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="flex flex-wrap items-center gap-1.5">
            <span :class="deliveryStatusClass(detail.status)" class="rounded-full px-3 py-1 text-xs font-bold">
              {{ deliveryStatusLabel(detail.status) }}
            </span>
            <span v-if="detail.purchasing_group" :class="purchasingGroupBadgeClass(detail.purchasing_group)"
              class="rounded-full border px-2.5 py-1 text-xs font-semibold">
              {{ purchasingGroupLabel(detail.purchasing_group) }}
            </span>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-700">
              {{ dispatchModeLabel(detail.dispatch_mode) }}
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <AppActionButton icon="print" label="Cetak Surat Jalan" @click="doPrintDetail" />
            <AppActionButton v-if="canManageDetail && detail.status === 'draft'" icon="edit" label="Edit Draft" @click="openEdit(detail)" />
          </div>
        </div>

        <!-- Metadata -->
        <div class="grid grid-cols-1 gap-x-4 gap-y-1.5 rounded-xl border border-gray-200 bg-gray-50 p-3 text-xs sm:grid-cols-2">
          <div><span class="text-gray-500">Tanggal Kirim:</span> <strong>{{ detail.delivery_date }}</strong></div>
          <div><span class="text-gray-500">Tujuan:</span> <strong>{{ detail.destination_label || '-' }}</strong></div>
          <div><span class="text-gray-500">Petugas / Kurir:</span> <strong>{{ detail.courier_name || (detail.dispatch_mode === 'vendor_direct' ? 'Pengantar Vendor' : '-') }}</strong><span v-if="detail.vehicle_info"> · {{ detail.vehicle_info }}</span></div>
          <div><span class="text-gray-500">Asal / Pengirim:</span> {{ detail.origin_label || 'Purchasing / Vendor' }}</div>
          <div><span class="text-gray-500">Dibuat:</span> {{ detail.created_by || '-' }} · {{ formatDateTime(detail.created_at) }}</div>
          <div v-if="detail.dispatched_at"><span class="text-gray-500">Berangkat:</span> {{ detail.dispatched_by || '-' }} · {{ formatDateTime(detail.dispatched_at) }}</div>
          <div v-if="detail.received_at"><span class="text-gray-500">Selesai Diterima:</span> {{ detail.received_by || '-' }} · {{ formatDateTime(detail.received_at) }}</div>
          <div v-if="detail.cancelled_at" class="sm:col-span-2 text-red-700">
            <span class="font-semibold">Dibatalkan:</span> {{ detail.cancelled_by || '-' }} · {{ formatDateTime(detail.cancelled_at) }}
            <template v-if="detail.cancel_reason"> — {{ detail.cancel_reason }}</template>
          </div>
          <div v-if="detail.notes" class="sm:col-span-2"><span class="text-gray-500">Catatan:</span> {{ detail.notes }}</div>
        </div>

        <!-- Manifest Barang -->
        <div>
          <p class="mb-2 text-xs font-bold uppercase tracking-wide text-gray-500">
            Manifest Barang dalam Pengantaran ({{ detail.items?.length || 0 }} jenis · {{ detail.total_received }}/{{ detail.total_planned }} unit diterima)
          </p>
          <div class="overflow-x-auto rounded-lg border border-gray-200">
            <table class="min-w-full text-xs divide-y divide-gray-100">
              <thead class="bg-gray-50 text-gray-500">
                <tr>
                  <th class="px-3 py-2 text-left">No. PR &amp; Vendor</th>
                  <th class="px-3 py-2 text-left">Barang</th>
                  <th class="px-3 py-2 text-left">Kategori</th>
                  <th class="px-3 py-2 text-right">Dibawa</th>
                  <th class="px-3 py-2 text-right">Diterima</th>
                  <th class="px-3 py-2 text-right">Sisa SJ</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 bg-white">
                <tr v-for="it in detail.items || []" :key="it.id">
                  <td class="px-3 py-2">
                    <p class="font-mono font-semibold text-gray-900">{{ it.pr_number }}</p>
                    <p class="text-[11px] text-gray-500">{{ it.vendor_name || '-' }}</p>
                  </td>
                  <td class="px-3 py-2">
                    <p class="font-medium text-gray-900">{{ it.item_name }}</p>
                    <p v-if="it.notes" class="text-[11px] text-gray-400">{{ it.notes }}</p>
                  </td>
                  <td class="px-3 py-2">
                    <span :class="it.line_kind === 'dapur' ? 'bg-amber-50 text-amber-800' : (it.line_kind === 'projek' ? 'bg-purple-50 text-purple-700' : 'bg-blue-50 text-blue-700')"
                      class="rounded px-1.5 py-0.5 text-[10px] font-semibold">
                      {{ LINE_KIND_LABELS[it.line_kind] || it.line_kind }}
                    </span>
                  </td>
                  <td class="px-3 py-2 text-right font-semibold">{{ it.qty_planned }} {{ it.unit }}</td>
                  <td class="px-3 py-2 text-right text-emerald-700 font-semibold">{{ it.qty_received }}</td>
                  <td class="px-3 py-2 text-right font-bold" :class="it.remaining > 0 ? 'text-amber-700' : 'text-gray-400'">{{ it.remaining }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Foto Pengantaran (bisa diedit/hapus/ganti sebelum status received/cancelled) -->
        <div class="rounded-xl border border-gray-200 bg-white p-3 space-y-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs font-bold text-gray-700">Foto Pengambilan / Pengantaran ({{ (detail.pickup_photo_urls || []).length }})</p>
            <button v-if="canManageDetail && isEditableBeforeReceived && !editingPickupPhotos" type="button"
              @click="startEditPickupPhotos" class="text-xs font-semibold text-emerald-700 hover:underline cursor-pointer">
              Edit / Hapus / Tambah Foto
            </button>
          </div>
          <div v-if="editingPickupPhotos" class="space-y-2">
            <PhotoCapture multiple :required="false" v-model="draftPickupPhotos"
              label="Kelola Foto Pengantaran" hint="Hapus dengan tombol × pada foto, atau tambah/ganti foto baru lalu simpan." />
            <div class="flex justify-end gap-2">
              <AppButton size="sm" variant="secondary" @click="editingPickupPhotos = false">Batal</AppButton>
              <AppButton size="sm" :loading="savingPhotos" @click="savePickupPhotos">Simpan Foto Pengantaran</AppButton>
            </div>
          </div>
          <div v-else-if="(detail.pickup_photo_urls || []).length" class="flex flex-wrap gap-2">
            <a v-for="(u, idx) in detail.pickup_photo_urls" :key="`pk-${idx}`" :href="safeUrl(u)" target="_blank" rel="noopener noreferrer">
              <img v-if="safeUrl(u)" :src="safeUrl(u)" :alt="`Foto pengantaran ${idx + 1}`" class="h-20 w-20 rounded-lg border border-gray-200 object-cover hover:opacity-90" />
            </a>
          </div>
          <p v-else class="text-xs text-gray-400">Belum ada foto pengantaran.</p>
        </div>

        <!-- List Foto Penerimaan (bisa ditambah, dihapus, atau diganti sebelum diterima final!) -->
        <div class="rounded-xl border border-emerald-200 bg-emerald-50/30 p-3 space-y-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <p class="text-xs font-bold text-emerald-900">
                Foto Penerimaan Barang ({{ (detail.receive_photo_urls || []).length }})
              </p>
              <p v-if="isEditableBeforeReceived" class="text-[11px] text-emerald-700">
                Daftar foto penerimaan masih bisa ditambah, dihapus, atau diganti selama status belum final Diterima.
              </p>
            </div>
            <button v-if="canEditReceivePhotos && isEditableBeforeReceived && !editingReceivePhotos" type="button"
              @click="startEditReceivePhotos" class="text-xs font-semibold text-emerald-800 hover:underline cursor-pointer">
              {{ (detail.receive_photo_urls || []).length ? 'Edit / Hapus / Ganti Foto' : '+ Unggah Draft Foto Penerimaan' }}
            </button>
          </div>
          <div v-if="editingReceivePhotos" class="space-y-2">
            <PhotoCapture multiple :required="false" v-model="draftReceivePhotos"
              label="Kelola Daftar Foto Penerimaan (Sebelum Final Diterima)"
              hint="Klik × untuk menghapus foto yang salah atau tambah/ganti dengan foto baru, lalu klik Simpan Daftar Foto." />
            <div class="flex justify-end gap-2">
              <AppButton size="sm" variant="secondary" @click="editingReceivePhotos = false">Batal</AppButton>
              <AppButton size="sm" :loading="savingPhotos" @click="saveReceivePhotos">Simpan Daftar Foto</AppButton>
            </div>
          </div>
          <div v-else-if="(detail.receive_photo_urls || []).length" class="flex flex-wrap gap-2">
            <a v-for="(u, idx) in detail.receive_photo_urls" :key="`rc-${idx}`" :href="safeUrl(u)" target="_blank" rel="noopener noreferrer">
              <img v-if="safeUrl(u)" :src="safeUrl(u)" :alt="`Foto penerimaan ${idx + 1}`" class="h-20 w-20 rounded-lg border border-emerald-200 object-cover hover:opacity-90" />
            </a>
          </div>
          <p v-else class="text-xs text-gray-500">Belum ada foto penerimaan.</p>
        </div>

        <!-- Info SOP 2-Tahap jika user Purchasing murni (bukan penerima akhir) -->
        <p v-if="['in_transit', 'partial_received'].includes(detail.status) && !canReceiveFinal"
          class="rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-800">
          Konfirmasi penerimaan final (masuk stok GRN / Aset) dilakukan oleh <strong>Gudang Induk</strong> (untuk bahan baku/dapur) atau <strong>Bagian Perlengkapan / Aset</strong> (untuk peralatan/projek).
        </p>

        <!-- Action Buttons -->
        <div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-3">
          <div class="flex flex-wrap gap-2">
            <AppButton v-if="canManageDetail && ['draft', 'in_transit'].includes(detail.status) && detail.total_received === 0"
              variant="danger" size="sm" :loading="actionBusy" @click="doCancelDelivery">
              Batalkan Pengantaran
            </AppButton>
          </div>
          <div class="flex flex-wrap gap-2">
            <AppButton v-if="canManageDetail && detail.status === 'draft'" :loading="actionBusy" @click="doDispatch">
              Tandai Dalam Pengantaran
            </AppButton>
            <AppButton v-if="canOpenReceiveModal" @click="openReceiveModal">
              Terima Barang Sesuai Surat Jalan
            </AppButton>
          </div>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>

    <!-- ═══ MODAL TERIMA BARANG SESUAI SURAT JALAN (GUDANG / ASET) ═══ -->
    <AppModal v-model="showReceive" :title="detail ? `Terima Barang — ${detail.delivery_number}` : 'Terima Barang'" size="2xl">
      <div v-if="detail" class="space-y-4">
        <div class="rounded-lg bg-emerald-50 border border-emerald-200 p-3 text-xs text-emerald-900">
          Penerimaan terikat pada manifest <strong>{{ detail.delivery_number }}</strong> ({{ dispatchModeLabel(detail.dispatch_mode) }}<template v-if="detail.courier_name"> · {{ detail.courier_name }}</template>).
          Hanya item yang menjadi wewenang meja Anda yang dapat dicatat; jika ada item meja lain, Surat Jalan akan berstatus <strong>Diterima Sebagian</strong> hingga seluruh itemnya diterima.
        </div>

        <!-- Foto Penerimaan (pre-filled dari draft SJ, bebas hapus / ganti / tambah sebelum final!) -->
        <div class="rounded-xl border border-emerald-200 bg-white p-3">
          <PhotoCapture multiple v-model="receivePhotos"
            label="Foto Barang Saat Diterima"
            hint="Bisa hapus foto sebelumnya dengan tombol ×, tambah, atau ganti foto sebelum menekan Konfirmasi Terima Barang." />
        </div>

        <div v-if="needsFallbackOutlet" class="rounded-lg border border-amber-200 bg-amber-50 p-3">
          <label class="lbl">Outlet Tujuan Aset (untuk PR tanpa outlet)</label>
          <SearchSelect v-model="receiveOutletId" :options="outlets" placeholder="Pilih outlet…" />
        </div>

        <div>
          <label class="lbl">Catatan Penerimaan (opsional)</label>
          <input v-model="receiveNotes" class="fld" placeholder="mis. Kondisi barang baik, lengkap sesuai Surat Jalan" />
        </div>

        <!-- Lines -->
        <div class="space-y-3">
          <template v-for="it in openDeliveryItems" :key="it.id">
            <div v-if="!canHandleDeliveryItem(it)" class="rounded-xl border border-dashed border-gray-200 bg-gray-50 p-3">
              <p class="font-semibold text-gray-500 text-sm">{{ it.item_name }} <span class="font-mono text-xs">({{ it.pr_number }})</span></p>
              <p class="text-xs text-gray-400">
                Sisa {{ it.remaining }} {{ it.unit }} · dicatat oleh {{ it.line_kind === 'dapur' ? 'Gudang Induk (izin penyesuaian stok)' : 'Bagian Aset (izin tambah aset)' }}
              </p>
            </div>
            <div v-else class="rounded-xl border border-gray-200 bg-white p-3 space-y-2.5">
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-semibold text-gray-900 text-sm">{{ it.item_name }}</p>
                  <p class="text-xs text-gray-500">
                    <span class="font-mono font-semibold">{{ it.pr_number }}</span>
                    <template v-if="it.vendor_name"> · {{ it.vendor_name }}</template>
                    · {{ formatRupiah(it.unit_price) }}/{{ it.unit || 'unit' }}
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <label class="text-xs text-gray-400">Terima</label>
                  <input v-model.number="receiveForm[it.id].qty" type="number" min="0" :max="it.remaining" class="fld w-20 text-right font-semibold" />
                  <span class="text-xs text-gray-500">/ {{ it.remaining }} {{ it.unit }}</span>
                </div>
              </div>

              <!-- Tujuan per baris -->
              <div class="flex flex-wrap gap-1.5">
                <button v-for="d in destOptionsFor(it)" :key="d.key" type="button"
                  @click="receiveForm[it.id].destination = d.key"
                  class="rounded-lg px-2.5 py-1 text-xs font-semibold transition cursor-pointer"
                  :class="receiveForm[it.id].destination === d.key ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'">
                  {{ d.label }}
                </button>
              </div>

              <!-- Stok Gudang -->
              <div v-if="receiveForm[it.id].destination === 'stok'" class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                <div>
                  <label class="lbl">Item Stok <span class="text-red-500">*</span></label>
                  <SearchSelect v-model="receiveForm[it.id].stock_item_id" :options="stockItems" placeholder="Pilih item stok…" @change="syncRecvStockUnit(it)" />
                </div>
                <div>
                  <label class="lbl">Gudang Penerima <span class="text-red-500">*</span></label>
                  <SearchSelect v-model="receiveForm[it.id].warehouse_id" :options="warehouses" placeholder="Pilih gudang…" />
                </div>
                <div v-if="stockItemOf(it.id)">
                  <label class="lbl">Satuan Terima</label>
                  <SearchableSelect v-model="receiveForm[it.id].unit_used" class="fld">
                    <option v-for="u in stockUnitOptions(stockItemOf(it.id))" :key="u.value" :value="u.value">{{ u.label }}</option>
                  </SearchableSelect>
                  <p class="mt-1 text-[11px] text-emerald-700">{{ conversionPreview(it) }}</p>
                </div>
                <div>
                  <label class="lbl">Kedaluwarsa (opsional)</label>
                  <input v-model="receiveForm[it.id].expiry_date" type="date" class="fld" />
                </div>
              </div>

              <!-- Aset -->
              <div v-else-if="receiveForm[it.id].destination === 'aset'" class="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
                <div>
                  <label class="lbl">Kategori Aset</label>
                  <SearchSelect v-model="receiveForm[it.id].category" :options="categoryOptions" placeholder="Pilih kategori…" />
                </div>
                <div>
                  <label class="lbl">Mode</label>
                  <select v-model="receiveForm[it.id].tracking_mode" class="fld">
                    <option value="tunggal">Tunggal (bernomor per unit)</option>
                    <option value="massal">Massal (satu baris)</option>
                  </select>
                </div>
                <div>
                  <label class="lbl">Lokasi</label>
                  <input v-model="receiveForm[it.id].location" class="fld" placeholder="mis. Dapur / Bar" />
                </div>
              </div>

              <!-- Material Projek -->
              <div v-else-if="receiveForm[it.id].destination === 'material'">
                <label class="lbl">Titik Simpan di Lokasi</label>
                <input v-model="receiveForm[it.id].location" class="fld" placeholder="mis. gudang sementara lokasi" />
              </div>

              <!-- Habis Pakai -->
              <div v-else>
                <label class="lbl">Alasan Habis Pakai (wajib, min {{ EXPENSE_REASON_MIN }} huruf)</label>
                <input v-model="receiveForm[it.id].reason" class="fld" placeholder="mis. langsung digunakan untuk operasional hari ini" />
              </div>
            </div>
          </template>
        </div>
      </div>
      <template #footer>
        <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <AppButton variant="secondary" @click="showReceive = false">Batal</AppButton>
          <AppButton :loading="receivingBusy" @click="submitReceiveDelivery">Konfirmasi Terima Barang</AppButton>
        </div>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { logisticsApi } from '@/api/logistics.js'
import { warehousesApi, stockItemsApi } from '@/api/warehouse.js'
import { outletsApi } from '@/api/outlets.js'
import { assetCategoriesApi } from '@/api/assetCategories.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah, formatDateTime, todayDateString } from '@/utils/format.js'
import { safeUrl } from '@/utils/safeUrl.js'
import { stockUnitOptions, stockUnitRatio } from '@/utils/stockUnits.js'
import { EXPENSE_REASON_MIN, reasonLength } from '@/utils/procurement.js'
import { useRealtime } from '@/utils/realtime.js'
import {
  deliveryStatusLabel, deliveryStatusClass, dispatchModeLabel, LINE_KIND_LABELS, printDeliveryNote,
  purchasingGroupLabel, purchasingGroupShort, purchasingGroupBadgeClass,
} from '@/utils/logisticsPrint.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const canManageAll = computed(() => auth.hasPermission('logistics.deliveries.manage'))
const canManageRaw = computed(() => canManageAll.value || auth.hasPermission('logistics.rawmaterials.manage'))
const canManageGoods = computed(() => canManageAll.value || auth.hasPermission('logistics.goods.manage'))
const canManage = computed(() => canManageRaw.value || canManageGoods.value)
const canViewAll = computed(() => canManageAll.value || auth.hasPermission('logistics.deliveries.view'))
const isOutletScoped = computed(() => auth.scopeType === 'specific')
const canViewRaw = computed(() =>
  canViewAll.value || canManageRaw.value ||
  auth.hasPermission('logistics.rawmaterials.view') ||
  auth.hasPermission('procurement.rawmaterials.purchasing') ||
  (!isOutletScoped.value && auth.hasPermission('stockledger.adjust'))
)
const canViewGoods = computed(() =>
  canViewAll.value || canManageGoods.value ||
  auth.hasPermission('logistics.goods.view') ||
  auth.hasPermission('assets.create') ||
  auth.hasPermission('procurement.requests.purchasing')
)
const canSeeBothGroups = computed(() => canViewRaw.value && canViewGoods.value)
const lockedGroup = computed(() => {
  if (canSeeBothGroups.value) return ''
  if (canViewRaw.value) return 'bahan_baku'
  if (canViewGoods.value) return 'barang_jasa'
  return ''
})
const activeGroup = ref('')
const effectiveGroup = computed(() => lockedGroup.value || activeGroup.value)

const canStock = computed(() => auth.hasPermission('stockledger.adjust'))
const canAsset = computed(() => auth.hasPermission('assets.create'))
const canSubmitRaw = computed(() => auth.hasPermission('procurement.rawmaterials.submit'))
const canSubmitGoods = computed(() => auth.hasPermission('procurement.requests.submit'))
const canSubmitAny = computed(() => canSubmitRaw.value || canSubmitGoods.value)
const canReceiveFinal = computed(() => canStock.value || canAsset.value || canSubmitAny.value)
const canManageDetail = computed(() => {
  if (!detail.value) return false
  if (canManageAll.value) return true
  const g = detail.value.purchasing_group
  if (g === 'bahan_baku') return canManageRaw.value
  if (g === 'barang_jasa') return canManageGoods.value
  return canManage.value
})
const canEditReceivePhotos = computed(() => {
  if (canManageDetail.value) return true
  const g = detail.value?.purchasing_group
  if (g === 'bahan_baku') return canStock.value || canSubmitRaw.value
  if (g === 'barang_jasa') return canAsset.value || canSubmitGoods.value
  return canReceiveFinal.value
})

function selectGroup(g) {
  activeGroup.value = g
  selectedReadyKeys.value = []
  readyReqType.value = ''
  applyFilters()
  loadDispatchable()
}

const activeTab = ref('deliveries')
const loading = ref(false)
const errorMsg = ref('')
const deliveries = ref([])
const totalDeliveries = ref(0)
const page = ref(1)
const limit = 20
const totalPages = ref(1)
const filterQ = ref('')
const filterStatus = ref('')
const filterMode = ref('')
const filterDateFrom = ref('')
const filterDateTo = ref('')

const DELIVERY_COLS = [
  { key: 'delivery_number', label: 'No. Surat Jalan' },
  { key: 'courier', label: 'Metode & Petugas' },
  { key: 'destination', label: 'Tujuan' },
  { key: 'manifest', label: 'Isi Manifest' },
  { key: 'status', label: 'Status' },
  { key: 'actions', label: '', align: 'right' },
]

// Tab 2: Barang Purchasing siap antar
const dispatchableItems = ref([])
const loadingReady = ref(false)
const readyQ = ref('')
const readyReqType = ref('')
const readyKind = ref('')
const selectedReadyKeys = ref([])

const readyKey = (it) => `${it.purchase_request_id}::${it.pr_item_key}`
const allReadySelected = computed(() =>
  dispatchableItems.value.length > 0 && selectedReadyKeys.value.length === dispatchableItems.value.length
)
function toggleAllReady(e) {
  selectedReadyKeys.value = e.target.checked ? dispatchableItems.value.map(readyKey) : []
}

// Lookups
const warehouses = ref([])
const outlets = ref([])
const stockItems = ref([])
const categories = ref([])
const categoryOptions = computed(() => categories.value.map(c => ({ id: c.name, name: c.name })))

// Create / Edit Form
const showForm = ref(false)
const saving = ref(false)
const editingId = ref('')
const editingNumber = ref('')
const editingStatus = ref('draft')
const showPicker = ref(true)
const pickerQ = ref('')
const pickerKind = ref('')
const form = ref(blankForm())

function blankForm() {
  return {
    delivery_date: todayDateString(),
    dispatch_mode: 'logistics',
    courier_name: auth.admin?.name || '',
    vehicle_info: '',
    origin_label: 'Purchasing',
    destination_type: 'warehouse',
    destination_warehouse_id: '',
    destination_outlet_id: '',
    destination_label: '',
    pickup_photo_urls: [],
    notes: '',
    items: [],
  }
}

const formLockedGroup = computed(() => {
  if (form.value.items.length > 0) {
    const first = form.value.items[0]
    if (first.purchasing_group) return first.purchasing_group
    if (first.request_type) return first.request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa'
  }
  if (!canManageRaw.value && canManageGoods.value) return 'barang_jasa'
  if (canManageRaw.value && !canManageGoods.value) return 'bahan_baku'
  return lockedGroup.value || activeGroup.value || ''
})

const filteredPickerItems = computed(() => {
  const q = pickerQ.value.trim().toLowerCase()
  const grp = formLockedGroup.value
  return dispatchableItems.value.filter(it => {
    const itGrp = it.purchasing_group || (it.request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa')
    if (grp && itGrp !== grp) return false
    if (itGrp === 'bahan_baku' && !canManageRaw.value) return false
    if (itGrp === 'barang_jasa' && !canManageGoods.value) return false
    if (pickerKind.value && it.line_kind !== pickerKind.value) return false
    if (!q) return true
    return `${it.pr_number} ${it.vendor_name} ${it.item_name} ${it.outlet_name}`.toLowerCase().includes(q)
  })
})

function isInFormItems(cand) {
  return form.value.items.some(x => x.purchase_request_id === cand.purchase_request_id && x.pr_item_key === cand.pr_item_key)
}
function addItemToForm(cand) {
  if (isInFormItems(cand)) return
  const candGrp = cand.purchasing_group || (cand.request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa')
  if (form.value.items.length > 0) {
    const curGrp = form.value.items[0].purchasing_group || (form.value.items[0].request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa')
    if (curGrp !== candGrp) {
      toast.error('Tidak boleh mencampur item Bahan Baku dan Barang/Perlengkapan dalam satu Surat Jalan.')
      return
    }
  }
  form.value.items.push({
    purchase_request_id: cand.purchase_request_id,
    pr_number: cand.pr_number,
    request_type: cand.request_type,
    purchasing_group: candGrp,
    vendor_name: cand.vendor_name,
    pr_item_key: cand.pr_item_key,
    item_name: cand.item_name,
    unit: cand.unit,
    line_kind: cand.line_kind,
    qty_planned: cand.qty_available,
    max_qty: cand.qty_available,
    notes: '',
  })
}
function onDestTypeChange() {
  form.value.destination_warehouse_id = ''
  form.value.destination_outlet_id = ''
  if (form.value.destination_type === 'asset_desk') {
    form.value.destination_label = 'Bagian Perlengkapan / Aset'
  } else {
    form.value.destination_label = ''
  }
}
function syncDestLabel() {
  if (form.value.destination_type === 'warehouse') {
    const w = warehouses.value.find(x => x.id === form.value.destination_warehouse_id)
    if (w) form.value.destination_label = w.name
  } else if (form.value.destination_type === 'outlet') {
    const o = outlets.value.find(x => x.id === form.value.destination_outlet_id)
    if (o) form.value.destination_label = `Outlet ${o.name}`
  }
}

// Detail Modal
const showDetail = ref(false)
const loadingDetail = ref(false)
const detail = ref(null)
const actionBusy = ref(false)
const editingPickupPhotos = ref(false)
const editingReceivePhotos = ref(false)
const draftPickupPhotos = ref([])
const draftReceivePhotos = ref([])
const savingPhotos = ref(false)

const isEditableBeforeReceived = computed(() =>
  !!detail.value && ['draft', 'in_transit', 'partial_received'].includes(detail.value.status)
)
const openDeliveryItems = computed(() => (detail.value?.items || []).filter(it => it.remaining > 0))
function canHandleDeliveryItem(it) {
  if (canSubmitAny.value) return true
  return it.line_kind === 'dapur' ? canStock.value : canAsset.value
}
const canOpenReceiveModal = computed(() => {
  if (!detail.value || !['in_transit', 'partial_received'].includes(detail.value.status)) return false
  return openDeliveryItems.value.some(canHandleDeliveryItem)
})

// Receive by Delivery Modal
const showReceive = ref(false)
const receivingBusy = ref(false)
const receivePhotos = ref([])
const receiveOutletId = ref('')
const receiveNotes = ref('')
const receiveForm = ref({})

const ALL_DESTS = [
  { key: 'aset', label: 'Aset' },
  { key: 'material', label: 'Material Projek' },
  { key: 'stok', label: 'Stok Gudang' },
  { key: 'habis', label: 'Habis Pakai' },
]
function destOptionsFor(it) {
  if (it.request_type === 'bahan' || it.line_kind === 'dapur') return ALL_DESTS.filter(d => d.key === 'stok')
  return ALL_DESTS.filter(d => d.key !== 'stok' && !(it.project_id && d.key === 'habis') && (d.key !== 'material' || it.project_id))
}
const needsFallbackOutlet = computed(() =>
  openDeliveryItems.value.some(it =>
    canHandleDeliveryItem(it) && !it.outlet_id && receiveForm.value[it.id]?.destination === 'aset' && (receiveForm.value[it.id]?.qty || 0) > 0
  )
)

function asArr(d) { return Array.isArray(d) ? d : (d?.data || d?.items || []) }

let searchTimer = null
function debouncedLoad() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => applyFilters(), 280)
}
let readyTimer = null
function debouncedLoadReady() {
  clearTimeout(readyTimer)
  readyTimer = setTimeout(() => loadDispatchable(), 280)
}
function applyFilters() {
  if (page.value === 1) loadDeliveries()
  else page.value = 1
}

async function loadDeliveries({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const res = await logisticsApi.list({
      page: page.value,
      limit,
      q: filterQ.value.trim(),
      status: filterStatus.value,
      dispatch_mode: filterMode.value,
      purchasing_group: effectiveGroup.value,
      date_from: filterDateFrom.value,
      date_to: filterDateTo.value,
    })
    deliveries.value = res?.data || []
    totalDeliveries.value = Number(res?.total || deliveries.value.length)
    totalPages.value = Number(res?.total_pages || 1)
  } catch (e) {
    if (!silent) errorMsg.value = e?.message || 'Gagal memuat daftar pengantaran'
  } finally {
    if (!silent) loading.value = false
  }
}

async function loadDispatchable(extraParams = {}) {
  loadingReady.value = true
  try {
    const reqTypeParam = effectiveGroup.value
      ? (effectiveGroup.value === 'bahan_baku' ? 'bahan' : 'barang')
      : readyReqType.value
    const res = await logisticsApi.dispatchableItems({
      q: readyQ.value.trim(),
      request_type: reqTypeParam,
      line_kind: readyKind.value,
      ...extraParams,
    })
    dispatchableItems.value = asArr(res)
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat barang siap antar')
  } finally {
    loadingReady.value = false
  }
}

async function loadLookups() {
  try { warehouses.value = asArr(await warehousesApi.list({ limit: 200 })) } catch { warehouses.value = [] }
  try {
    const oRes = await outletsApi.myOutlets()
    outlets.value = asArr(oRes?.outlets ?? oRes)
  } catch { outlets.value = [] }
  if (canStock.value) {
    try { stockItems.value = asArr(await stockItemsApi.listAll()) } catch { stockItems.value = [] }
  }
  if (auth.hasPermission('assets.view')) {
    try { categories.value = asArr(await assetCategoriesApi.list()) } catch { categories.value = [] }
  }
}

function switchToReadyTab() {
  activeTab.value = 'ready'
  loadDispatchable()
}

async function openCreate(preselected = []) {
  editingId.value = ''
  editingNumber.value = ''
  editingStatus.value = 'draft'
  form.value = blankForm()
  const central = warehouses.value.find(w => w.type === 'central')
  if (central) {
    form.value.destination_warehouse_id = central.id
    form.value.destination_label = central.name
  }
  await loadDispatchable()
  for (const it of preselected) addItemToForm(it)
  showPicker.value = preselected.length === 0
  pickerQ.value = ''
  pickerKind.value = ''
  showForm.value = true
}

function createFromSelectedReady() {
  const chosen = dispatchableItems.value.filter(it => selectedReadyKeys.value.includes(readyKey(it)))
  if (!chosen.length) return
  const groups = new Set(chosen.map(it => it.purchasing_group || (it.request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa')))
  if (groups.size > 1) {
    toast.error('Item Bahan Baku dan Barang/Perlengkapan tidak boleh dicampur dalam satu Surat Jalan. Pilih salah satu kelompok Purchasing.')
    return
  }
  openCreate(chosen)
}

async function openEdit(d) {
  editingId.value = d.id
  editingNumber.value = d.delivery_number
  editingStatus.value = d.status
  await loadDispatchable()
  form.value = {
    delivery_date: d.delivery_date || todayDateString(),
    dispatch_mode: d.dispatch_mode || 'logistics',
    courier_name: d.courier_name || '',
    vehicle_info: d.vehicle_info || '',
    origin_label: d.origin_label || '',
    destination_type: d.destination_type || 'warehouse',
    destination_warehouse_id: d.destination_warehouse_id || '',
    destination_outlet_id: d.destination_outlet_id || '',
    destination_label: d.destination_label || '',
    pickup_photo_urls: [...(d.pickup_photo_urls || [])],
    notes: d.notes || '',
    items: (d.items || []).map(it => {
      const extra = dispatchableItems.value.find(c => c.purchase_request_id === it.purchase_request_id && c.pr_item_key === it.pr_item_key)
      return {
        purchase_request_id: it.purchase_request_id,
        pr_number: it.pr_number,
        request_type: it.request_type,
        purchasing_group: it.request_type === 'bahan' ? 'bahan_baku' : 'barang_jasa',
        vendor_name: it.vendor_name,
        pr_item_key: it.pr_item_key,
        item_name: it.item_name,
        unit: it.unit,
        line_kind: it.line_kind,
        qty_planned: it.qty_planned,
        max_qty: it.qty_planned + (extra?.qty_available || 0),
        notes: it.notes || '',
      }
    }),
  }
  showPicker.value = false
  showDetail.value = false
  showForm.value = true
}

async function submitForm(dispatchNow) {
  if (!form.value.items.length) { toast.error('Pilih minimal 1 barang untuk pengantaran'); return }
  const badQty = form.value.items.find(it => !(Number(it.qty_planned) > 0) || Number(it.qty_planned) > Number(it.max_qty || 999999))
  if (badQty) { toast.error(`Qty "${badQty.item_name}" tidak valid (1 s/d ${badQty.max_qty})`); return }
  saving.value = true
  try {
    const payload = {
      delivery_date: form.value.delivery_date,
      dispatch_mode: form.value.dispatch_mode,
      dispatch_now: dispatchNow,
      courier_name: form.value.courier_name.trim(),
      vehicle_info: form.value.vehicle_info.trim(),
      origin_label: form.value.origin_label.trim(),
      destination_type: form.value.destination_type,
      destination_warehouse_id: form.value.destination_warehouse_id,
      destination_outlet_id: form.value.destination_outlet_id,
      destination_label: form.value.destination_label.trim(),
      pickup_photo_urls: form.value.pickup_photo_urls,
      notes: form.value.notes.trim(),
      items: form.value.items.map(it => ({
        purchase_request_id: it.purchase_request_id,
        pr_item_key: it.pr_item_key,
        qty_planned: Number(it.qty_planned),
        notes: (it.notes || '').trim(),
      })),
    }
    let saved
    if (editingId.value) {
      saved = await logisticsApi.update(editingId.value, payload)
      toast.success('Pengantaran diperbarui.')
    } else {
      saved = await logisticsApi.create(payload)
      toast.success(dispatchNow ? `Surat Jalan ${saved?.delivery_number || ''} diberangkatkan.` : `Draft Surat Jalan ${saved?.delivery_number || ''} tersimpan.`)
    }
    showForm.value = false
    selectedReadyKeys.value = []
    await Promise.all([loadDeliveries(), loadDispatchable()])
    if (saved?.id) openDetail(saved.id)
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan pengantaran')
  } finally {
    saving.value = false
  }
}

async function openDetail(id) {
  showDetail.value = true
  loadingDetail.value = true
  editingPickupPhotos.value = false
  editingReceivePhotos.value = false
  try {
    detail.value = await logisticsApi.get(id)
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat detail pengantaran')
    showDetail.value = false
  } finally {
    loadingDetail.value = false
  }
}

async function quickPrint(id) {
  try {
    const d = await logisticsApi.get(id)
    const r = printDeliveryNote(d)
    if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
  } catch (e) {
    toast.error(e?.message || 'Gagal mencetak Surat Jalan')
  }
}

function doPrintDetail() {
  const r = printDeliveryNote(detail.value)
  if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
}

function startEditPickupPhotos() {
  draftPickupPhotos.value = [...(detail.value?.pickup_photo_urls || [])]
  editingPickupPhotos.value = true
}
function startEditReceivePhotos() {
  draftReceivePhotos.value = [...(detail.value?.receive_photo_urls || [])]
  editingReceivePhotos.value = true
}

async function savePickupPhotos() {
  savingPhotos.value = true
  try {
    detail.value = await logisticsApi.updatePhotos(detail.value.id, { pickup_photo_urls: draftPickupPhotos.value })
    editingPickupPhotos.value = false
    toast.success('Foto pengantaran diperbarui.')
    loadDeliveries({ silent: true })
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui foto pengantaran')
  } finally {
    savingPhotos.value = false
  }
}

async function saveReceivePhotos() {
  savingPhotos.value = true
  try {
    detail.value = await logisticsApi.updatePhotos(detail.value.id, { receive_photo_urls: draftReceivePhotos.value })
    editingReceivePhotos.value = false
    toast.success('Daftar foto penerimaan diperbarui.')
    loadDeliveries({ silent: true })
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui foto penerimaan')
  } finally {
    savingPhotos.value = false
  }
}

async function doDispatch() {
  actionBusy.value = true
  try {
    detail.value = await logisticsApi.updateStatus(detail.value.id, { status: 'in_transit' })
    toast.success(`Surat Jalan ${detail.value.delivery_number} ditandai Dalam Pengantaran.`)
    loadDeliveries({ silent: true })
  } catch (e) {
    toast.error(e?.message || 'Gagal memperbarui status')
  } finally {
    actionBusy.value = false
  }
}

async function doCancelDelivery() {
  const reason = window.prompt('Alasan pembatalan pengantaran (opsional):', '')
  if (reason === null) return
  actionBusy.value = true
  try {
    detail.value = await logisticsApi.updateStatus(detail.value.id, { status: 'cancelled', cancel_reason: reason.trim() })
    toast.success('Pengantaran dibatalkan — kuota item dikembalikan ke daftar siap antar.')
    await Promise.all([loadDeliveries({ silent: true }), loadDispatchable()])
  } catch (e) {
    toast.error(e?.message || 'Gagal membatalkan pengantaran')
  } finally {
    actionBusy.value = false
  }
}

// Receive Modal helpers
function stockItemOf(diID) {
  const sid = receiveForm.value[diID]?.stock_item_id
  return sid ? stockItems.value.find(s => s.id === sid) || null : null
}
function defaultUnitFor(stockItemId, prUnit) {
  const it = stockItems.value.find(s => s.id === stockItemId)
  if (!it) return ''
  const b = String(it.base_unit || '').trim()
  const d = String(it.dist_unit || b).trim()
  const u = String(prUnit || '').trim()
  if (b && d && b.toLowerCase() === d.toLowerCase()) return b
  if (u && b && u.toLowerCase() === b.toLowerCase()) return b
  return d || b
}
function syncRecvStockUnit(it) {
  const row = receiveForm.value[it.id]
  if (row) row.unit_used = defaultUnitFor(row.stock_item_id, it.unit)
}
function conversionPreview(it) {
  const s = stockItemOf(it.id)
  const row = receiveForm.value[it.id]
  if (!s || !row) return ''
  const qty = Number(row.qty) || 0
  const unit = row.unit_used || defaultUnitFor(row.stock_item_id, it.unit)
  const ratio = stockUnitRatio(s, unit)
  const totalBase = qty * ratio
  const perBase = ratio > 0 ? (Number(it.unit_price) || 0) / ratio : Number(it.unit_price) || 0
  return `Masuk gudang: ${new Intl.NumberFormat('id-ID', { maximumFractionDigits: 4 }).format(totalBase)} ${s.base_unit} · HPP ${formatRupiah(perBase)}/${s.base_unit}`
}

async function openReceiveModal() {
  if (!stockItems.value.length && canStock.value) await loadLookups()
  receivePhotos.value = [...(detail.value?.receive_photo_urls || [])]
  receiveOutletId.value = detail.value?.destination_outlet_id || ''
  receiveNotes.value = ''
  const defaultWh = detail.value?.receiving_warehouse_id || detail.value?.destination_warehouse_id || warehouses.value[0]?.id || ''
  receiveForm.value = Object.fromEntries(openDeliveryItems.value.map(it => {
    const allowed = destOptionsFor(it).map(d => d.key)
    const dest = allowed.includes(it.destination) ? it.destination : (allowed[0] || 'habis')
    return [it.id, {
      qty: it.remaining,
      destination: dest,
      category: '',
      tracking_mode: it.remaining === 1 || it.unit_price >= 5000000 ? 'tunggal' : 'massal',
      location: '',
      stock_item_id: it.suggested_stock_item_id || '',
      warehouse_id: defaultWh,
      unit_used: defaultUnitFor(it.suggested_stock_item_id, it.unit),
      expiry_date: '',
      reason: '',
    }]
  }))
  showReceive.value = true
}

async function submitReceiveDelivery() {
  const handled = openDeliveryItems.value.filter(canHandleDeliveryItem)
  const lines = handled
    .map(it => ({ delivery_item_id: it.id, ...receiveForm.value[it.id], _item: it }))
    .filter(l => (Number(l.qty) || 0) > 0)
  if (!lines.length) { toast.error('Isi jumlah diterima minimal pada satu baris'); return }
  const over = lines.find(l => l.qty > l._item.remaining)
  if (over) { toast.error(`Jumlah "${over._item.item_name}" melebihi sisa pada Surat Jalan`); return }
  const badStock = lines.find(l => l.destination === 'stok' && canStock.value && (!l.stock_item_id || !l.warehouse_id))
  if (badStock) { toast.error('Baris stok gudang wajib memilih item stok dan gudang'); return }
  const noReason = lines.find(l => l.destination === 'habis' && reasonLength(l.reason) < EXPENSE_REASON_MIN)
  if (noReason) { toast.error(`Alasan habis pakai untuk "${noReason._item.item_name}" minimal ${EXPENSE_REASON_MIN} huruf`); return }
  if (needsFallbackOutlet.value && !receiveOutletId.value) { toast.error('Pilih outlet tujuan aset'); return }
  if (!receivePhotos.value.length) { toast.error('Minimal 1 foto barang saat diterima wajib diunggah'); return }

  receivingBusy.value = true
  try {
    const res = await logisticsApi.receive(detail.value.id, {
      outlet_id: receiveOutletId.value,
      photo_url: receivePhotos.value[0],
      photo_urls: receivePhotos.value,
      notes: receiveNotes.value.trim(),
      lines: lines.map(({ _item, ...rest }) => rest),
    })
    toast.success(res?.message || 'Penerimaan Surat Jalan dicatat.', 7000)
    showReceive.value = false
    detail.value = await logisticsApi.get(detail.value.id)
    await Promise.all([loadDeliveries({ silent: true }), loadDispatchable()])
  } catch (e) {
    toast.error(e?.message || 'Gagal mencatat penerimaan Surat Jalan')
  } finally {
    receivingBusy.value = false
  }
}

watch(page, () => loadDeliveries())

onMounted(async () => {
  await Promise.all([loadDeliveries(), loadDispatchable(), loadLookups()])
  const qDelId = String(route.query.delivery_id || '').trim()
  const qPrId = String(route.query.pr_id || '').trim()
  if (qDelId) {
    openDetail(qDelId)
    router.replace({ query: {} })
  } else if (qPrId && route.query.create === '1' && canManage.value) {
    const matched = dispatchableItems.value.filter(it => it.purchase_request_id === qPrId)
    await openCreate(matched)
    router.replace({ query: {} })
  }
})

useRealtime(['logistics_delivery', 'procurement_receiving', 'procurement'], async () => {
  loadDeliveries({ silent: true })
  if (activeTab.value === 'ready') loadDispatchable()
  if (showDetail.value && detail.value?.id && !editingPickupPhotos.value && !editingReceivePhotos.value && !showReceive.value) {
    try { detail.value = await logisticsApi.get(detail.value.id) } catch {}
  }
})
</script>

<style scoped>
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.fld {
  width: 100%; padding: .45rem .65rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.16); background: #fff; color: #111827; outline: none;
}
.fld:focus { border-color: rgba(5,150,105,.55); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
</style>
