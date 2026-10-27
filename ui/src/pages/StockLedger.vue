<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h1 class="text-xl font-bold text-gray-900">Kartu Stok</h1>
      <div class="flex flex-wrap items-center gap-2">
        <AppButton @click="openBreakdownHistory" variant="secondary">Riwayat Pemotongan</AppButton>
        <AppButton v-if="canBreakdown" @click="openBreakdownModal" variant="secondary">
          <svg class="h-4 w-4 mr-1.5 -ml-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="6" cy="6" r="3"/><path d="M8.12 8.12 12 12"/><path d="M20 4 8.12 15.88"/><circle cx="6" cy="18" r="3"/><path d="M14.8 14.8 20 20"/>
          </svg>
          Pemotongan
        </AppButton>
        <AppButton v-if="canProduce" @click="openProduceModal" variant="primary">
          <svg class="h-4 w-4 mr-1.5 -ml-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
          Produksi (Masak)
        </AppButton>
      </div>
    </div>

    <p class="rounded-lg bg-indigo-50 px-4 py-3 text-sm text-indigo-900">
      Pantau saldo dan mutasi bahan baku di Gudang. Pencatatan pemakaian dan hasil pengolahan
      mengikuti formula yang sudah ditetapkan F &amp; B.
    </p>

    <!-- Summary Stats -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="bg-white p-4 rounded-xl border border-gray-100 shadow-sm flex items-center gap-4">
        <div class="w-12 h-12 rounded-full bg-blue-50 flex items-center justify-center text-blue-600">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
        </div>
        <div>
          <div class="text-xs text-gray-500 uppercase font-bold tracking-wider">Total Item</div>
          <div class="text-xl font-bold text-gray-900">{{ total }}</div>
        </div>
      </div>
      <div class="bg-white p-4 rounded-xl border border-gray-100 shadow-sm flex items-center gap-4">
        <div class="w-12 h-12 rounded-full bg-red-50 flex items-center justify-center text-red-600">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>
        </div>
        <div>
          <div class="text-xs text-gray-500 uppercase font-bold tracking-wider">Stok Rendah</div>
          <div class="text-xl font-bold text-red-600">{{ lowStockCount }}</div>
        </div>
      </div>
      <div class="bg-white p-4 rounded-xl border border-gray-100 shadow-sm flex items-center gap-4">
        <div class="w-12 h-12 rounded-full bg-emerald-50 flex items-center justify-center text-emerald-600">
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        </div>
        <div>
          <div class="text-xs text-gray-500 uppercase font-bold tracking-wider">Nilai Aset</div>
          <div class="text-xl font-bold text-emerald-700">{{ formatRupiah(totalAssetValue) }}</div>
        </div>
      </div>
    </div>

    <!-- Filters -->
    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-3 px-4 py-3 border-b border-gray-100">
        <div class="min-w-[260px] max-w-md">
          <SearchSelect
            v-model="filterWarehouse"
            :options="warehouseOptions"
            placeholder="Semua Gudang"
            searchPlaceholder="Cari gudang..."
            valueKey="value"
            labelKey="label"
            @change="applyFilters"
          />
        </div>
        <input v-model="search" @input="debouncedLoad" placeholder="Cari bahan baku..."
          class="flex-1 min-w-[160px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
        <label class="flex items-center gap-2 text-sm text-gray-600 cursor-pointer select-none">
          <input type="checkbox" v-model="lowStockOnly" @change="applyFilters" class="rounded text-red-500" />
          <span class="text-red-600 font-medium">Stok Rendah</span>
        </label>
      </div>

      <AppTable :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Tidak ada data stok.">
        <template #cell-item_name="{ row }">
          <div>
            <span class="font-medium text-gray-900">{{ row.item_name }}</span>
            <span class="ml-2 font-mono text-xs text-gray-400">{{ row.item_code }}</span>
          </div>
        </template>
        <template #cell-warehouse_name="{ row }">
          <div class="flex flex-col gap-0.5">
            <span class="text-sm font-medium text-gray-700">{{ getWarehouseLabel(row) }}</span>
            <span :class="row.warehouse_type === 'central' ? 'text-blue-700' : 'text-amber-700'"
              class="text-[11px] font-medium">
              {{ row.warehouse_type === 'central' ? 'Gudang Induk' : 'Gudang Outlet' }}
            </span>
          </div>
        </template>
        <template #cell-qty_base="{ row }">
          <div>
            <div :class="['font-semibold', row.is_low ? 'text-red-600' : 'text-gray-900']">
              {{ stockQtyText(row, row.qty_base, row.warehouse_type) }}
              <span v-if="row.is_low" class="ml-1 text-xs text-red-400">(Rendah!)</span>
            </div>
            <div v-if="row.dist_unit !== row.base_unit" class="text-xs text-gray-400">
              = {{ stockQtyText(row, row.qty_base, row.warehouse_type === 'central' ? 'outlet' : 'central') }}
            </div>
          </div>
        </template>
        <template #cell-avg_cost="{ row }">
          {{ formatRupiah(row.avg_cost * stockUnitRatio(row, preferredStockUnit(row, row.warehouse_type))) }}/{{ stockUnitLabel(row, preferredStockUnit(row, row.warehouse_type)) }}
        </template>
        <template #cell-stock_value="{ row }">
          <span class="font-medium">{{ formatRupiah(row.stock_value) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <AppActionButton icon="history" label="Histori" @click="openMovements(row)" />
          <AppActionButton icon="adjust" label="Penyesuaian" @click="openAdjust(row)" class="ml-1" />
        </template>
      </AppTable>
      <AppPagination v-model:page="page" :total-pages="totalPages" class="px-4 py-3 border-t border-gray-100" />
    </AppCard>

    <!-- Movements Modal -->
    <AppModal v-model="showMovements" title="Histori Pergerakan Stok" size="2xl">
      <div class="mb-3 text-sm font-medium text-gray-700">
        {{ movTarget?.item_name }} — {{ movTarget?.warehouse_name }}
      </div>
      <div class="flex gap-2 mb-3">
        <DateRangePicker v-model="movRange" clearable />
        <AppButton size="sm" @click="loadMovements">Tampilkan</AppButton>
      </div>
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500">
            <tr>
              <th class="px-3 py-2 text-left">Tanggal</th>
              <th class="px-3 py-2 text-left">Tipe</th>
              <th class="px-3 py-2 text-right">Qty</th>
              <th class="px-3 py-2 text-right">Saldo</th>
              <th class="px-3 py-2 text-left">Ref</th>
              <th class="px-3 py-2 text-left">Kadaluarsa</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-if="movLoading"><td colspan="6" class="px-3 py-4 text-center text-gray-400">Memuat...</td></tr>
            <tr v-else-if="movements.length === 0"><td colspan="6" class="px-3 py-4 text-center text-gray-400">Tidak ada histori.</td></tr>
            <tr v-for="m in movements" :key="m.id" class="hover:bg-gray-50">
              <td class="px-3 py-2 text-gray-500">{{ m.created_at.slice(0,16).replace('T',' ') }}</td>
              <td class="px-3 py-2">
                <span :class="movTypeCls(m.movement_type)" class="text-xs px-2 py-0.5 rounded-full font-medium">
                  {{ MOV_LABELS[m.movement_type] || m.movement_type }}
                </span>
              </td>
              <td :class="['px-3 py-2 text-right font-mono font-medium', m.qty_base >= 0 ? 'text-emerald-700' : 'text-red-600']">
                {{ m.qty_base >= 0 ? '+' : '' }}{{ m.qty_base.toFixed(3) }} {{ movTarget?.base_unit }}
                <div v-if="m.unit_used && m.unit_used !== movTarget?.base_unit" class="text-xs text-gray-400">
                  {{ m.qty_dist }} {{ m.unit_used }}
                </div>
              </td>
              <td class="px-3 py-2 text-right font-mono text-gray-700">{{ m.balance_after.toFixed(3) }}</td>
              <td class="px-3 py-2 text-gray-500 text-xs">{{ m.ref_number || '—' }}</td>
              <td class="px-3 py-2 text-gray-500 text-xs">{{ m.expiry_date || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showMovements = false">Tutup</AppButton>
      </template>
    </AppModal>

    <!-- Adjustment Modal -->
    <AppModal v-model="showAdjust" title="Penyesuaian Stok" size="lg">
      <div class="modal-form-grid">
        <p class="modal-span-full text-sm text-gray-600">
          Item: <strong>{{ adjustTarget?.item_name }}</strong><br/>
          Gudang: <strong>{{ adjustTarget?.warehouse_name }}</strong><br/>
          Stok saat ini: <strong>{{ adjustTarget?.qty_base?.toFixed(3) }} {{ adjustTarget?.base_unit }}</strong>
        </p>
        <div class="modal-span-full">
          <label class="block text-sm font-medium text-gray-700 mb-1">Tipe Gerakan</label>
          <div class="grid grid-cols-2 gap-2">
            <label v-for="t in ADJ_TYPES" :key="t.value"
              :class="['border rounded-lg p-2.5 cursor-pointer transition-all text-sm text-center',
                adjForm.movement_type === t.value ? 'border-emerald-400 bg-emerald-50 text-emerald-800 font-medium' : 'border-gray-200 hover:border-gray-300 text-gray-700']"
              @click="adjForm.movement_type = t.value">
              {{ t.label }}
            </label>
          </div>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Satuan Input</label>
          <SearchableSelect v-model="adjustUnit" class="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm">
            <option v-for="unit in stockUnitOptions(adjustTarget)" :key="unit.value" :value="unit.value">{{ unit.label }}</option>
          </SearchableSelect>
        </div>
        <AppInput v-model.number="adjustQty" :label="`Qty (${adjustUnit})`" type="number" step="0.0001" placeholder="0" />
        <RupiahInput v-if="adjForm.movement_type === 'purchase_in'" v-model="adjustCost"
          :label="`HPP per ${adjustUnit}`" step="0.01" placeholder="0" />
        <AppInput v-if="['purchase_in', 'adjustment', 'return_in'].includes(adjForm.movement_type)" 
          v-model="adjForm.expiry_date" label="Tanggal Kadaluarsa" type="date" />
        <AppInput class="modal-span-full" v-model="adjForm.notes" label="Keterangan" placeholder="Opsional" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showAdjust = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitAdjust">Simpan</AppButton>
      </template>
    </AppModal>

    <!-- Produce Modal -->
    <AppModal v-model="showProduce" title="Produksi / Memasak" size="lg">
      <div class="modal-form-grid">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Pilih Barang yang Dibuat</label>
          <SearchSelect
            v-model="produceForm.item_id"
            :options="produceItemOptions"
            placeholder="Cari sambel, saos, dll..."
            searchPlaceholder="Cari..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Gudang Tempat Produksi</label>
          <SearchSelect
            v-model="produceForm.warehouse_id"
            :options="warehouseOptions.filter(o => o.value !== '')"
            placeholder="Pilih gudang..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <AppInput v-model.number="produceForm.qty_produce" label="Kuantitas Hasil Produksi" type="number" step="0.001" placeholder="0" />
        <div v-if="produceForm.item_id" class="modal-span-full p-3 bg-amber-50 rounded-lg border border-amber-100 text-xs text-amber-700">
          <strong>Info:</strong> Stok bahan baku penyusun akan otomatis terpotong sesuai resep yang telah diatur, dan HPP akan dihitung secara otomatis.
        </div>
        <AppInput class="modal-span-full" v-model="produceForm.notes" label="Catatan Produksi" placeholder="Opsional" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showProduce = false">Batal</AppButton>
        <AppButton variant="primary" :loading="saving" @click="submitProduce">Proses Produksi</AppButton>
      </template>
    </AppModal>

    <!-- Pemotongan: satu bahan → beberapa hasil menurut pola yang dipilih -->
    <AppModal v-model="showBreakdown" title="Pemotongan Bahan" size="xl">
      <div class="space-y-4">
        <p class="text-xs text-gray-500">
          Satu bahan dipotong atau diolah menjadi beberapa hasil — mis. ayam utuh → paha, dada, sayap; daging sapi → has dalam,
          has luar, tetelan; ikan utuh → fillet, kepala; beras → nasi. Isi jumlah hasil yang benar-benar didapat: stok dan HPP
          mengikuti hasil aktual, dan susutnya terlihat dari hasil yang di bawah standar pola.
        </p>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="min-w-0">
            <label class="block text-sm font-medium text-gray-700 mb-1">Gudang</label>
            <SearchSelect
              v-model="bdForm.warehouse_id"
              :options="bdWarehouseOptions"
              placeholder="Pilih gudang..."
              searchPlaceholder="Cari gudang..."
              valueKey="value"
              labelKey="label"
            />
          </div>
          <div class="min-w-0">
            <label class="block text-sm font-medium text-gray-700 mb-1">Bahan yang Dipotong / Diolah</label>
            <SearchSelect
              v-model="bdForm.input_item_id"
              :options="bdInputOptions"
              :placeholder="bdTemplatesLoading ? 'Memuat…' : 'Pilih bahan...'"
              searchPlaceholder="Cari bahan..."
              valueKey="value"
              labelKey="label"
            />
          </div>
        </div>
        <div v-if="!bdTemplatesLoading && bdInputOptions.length === 0"
          class="flex items-start gap-2 rounded-lg bg-amber-50 ring-1 ring-amber-100 px-3 py-2 text-xs text-amber-800">
          <svg class="h-4 w-4 mt-0.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
          <span>Belum ada bahan dengan pola pemotongan aktif. Pola dikelola F &amp; B di <b>Resep &amp; Formula › Pola Pemotongan</b>.</span>
        </div>

        <!-- Pola: tampil hanya bila bahan punya lebih dari satu pola aktif. -->
        <div v-if="bdTemplate && bdPatterns.length > 1">
          <label class="block text-sm font-medium text-gray-700 mb-1">Pola Pemotongan</label>
          <div class="flex flex-wrap gap-2">
            <button v-for="p in bdPatterns" :key="p.id" type="button" @click="bdForm.pattern_id = p.id"
              :class="['px-3 py-1.5 rounded-lg border text-sm transition-colors',
                bdForm.pattern_id === p.id ? 'border-pink-400 bg-pink-50 text-pink-800 font-semibold' : 'border-gray-200 text-gray-700 hover:border-gray-300']">
              {{ p.name }}
            </button>
          </div>
          <p v-if="!bdPattern" class="mt-1 text-xs text-gray-400">Pilih pola untuk menampilkan hasilnya.</p>
        </div>
        <p v-if="bdPattern?.notes" class="-mt-2 text-xs text-gray-500">{{ bdPattern.notes }}</p>

        <template v-if="bdPattern">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:items-end">
            <AppInput v-model.number="bdForm.input_qty_dist" :label="`Jumlah dipakai (${bdDistLabel})`" type="number" step="any" min="0" placeholder="0" />
            <div class="text-xs text-gray-500 space-y-0.5 sm:pb-2">
              <div v-if="bdTemplate.input_dist_ratio !== 1 && bdQty > 0">= {{ fmtNum(bdInputBase) }} {{ bdTemplate.input_base_unit }}</div>
              <div>Stok di gudang: <b :class="bdStockShort ? 'text-red-600' : 'text-gray-800'">{{ bdStockText }}</b></div>
            </div>
          </div>

          <div>
            <div class="flex flex-wrap items-baseline justify-between gap-x-2 mb-2">
              <span class="text-sm font-medium text-gray-700">Hasil Aktual<span v-if="bdPatterns.length > 1" class="font-normal text-gray-500"> · {{ bdPattern.name }}</span></span>
              <span class="text-[11px] text-gray-400">terisi hasil standar — ubah sesuai hasil nyata</span>
            </div>
            <div class="modal-form-grid">
              <div v-for="r in bdRows" :key="r.item_id" class="rounded-lg border border-gray-200 p-3">
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="font-medium text-gray-900 break-words">{{ r.name }}</div>
                    <div class="text-xs text-gray-500">Standar {{ fmtNum(bdStd(r)) }} {{ r.unit }}</div>
                  </div>
                  <span :class="yieldBadgeCls(bdYield(r))" class="shrink-0 text-xs font-semibold px-2 py-0.5 rounded-full">{{ fmtPct(bdYield(r)) }}</span>
                </div>
                <div class="mt-2 grid grid-cols-2 gap-2 items-end">
                  <div class="min-w-0">
                    <label class="block text-xs text-gray-500 mb-1">Hasil aktual ({{ r.unit }})</label>
                    <input v-model.number="r.qty" @input="r.touched = true" type="number" step="any" min="0"
                      class="w-full min-w-0 text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
                  </div>
                  <div class="text-right text-xs text-gray-500 pb-1">
                    Perkiraan HPP
                    <div class="font-semibold text-gray-800">{{ bdEstUnitCostText(r) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 space-y-0.5">
            <div>
              Perkiraan biaya bahan: <b>{{ formatRupiah(bdEstCost) }}</b>
              <span class="text-gray-400">— dari HPP rata-rata gudang; biaya pasti dihitung FIFO saat disimpan.</span>
            </div>
            <div v-if="bdValueYield !== null">
              Hasil dibanding standar (ditimbang bobot biaya): <b :class="yieldTextCls(bdValueYield)">{{ fmtPct(bdValueYield) }}</b>
              <span v-if="bdValueYield < 100"> · susut ≈ {{ formatRupiah(bdEstCost * (1 - bdValueYield / 100)) }}</span>
            </div>
            <div class="text-gray-400">Hasil mewarisi tanggal kedaluwarsa terdekat dari bahan yang terpotong.</div>
          </div>
          <AppInput v-model="bdForm.notes" label="Catatan" placeholder="Opsional" />
        </template>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showBreakdown = false">Batal</AppButton>
        <AppButton variant="primary" :loading="saving" :disabled="!bdPattern" @click="submitBreakdown">Simpan Pemotongan</AppButton>
      </template>
    </AppModal>

    <!-- Riwayat Pemotongan -->
    <AppModal v-model="showBdHistory" title="Riwayat Pemotongan" size="2xl">
      <div class="mb-3 max-w-md">
        <SearchSelect
          v-model="bdHistWarehouse"
          :options="warehouseOptions"
          placeholder="Semua Gudang"
          searchPlaceholder="Cari gudang..."
          valueKey="value"
          labelKey="label"
          @change="reloadBdHistory"
        />
      </div>
      <div v-if="bdHistLoading" class="py-6 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="bdHistory.length === 0" class="py-6 text-center text-sm text-gray-400">Belum ada pemotongan.</div>
      <ul v-else class="modal-form-grid">
        <li v-for="d in bdHistory" :key="d.id" class="rounded-lg border border-gray-200 p-3">
          <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
            <span class="font-mono text-sm font-semibold text-gray-900">{{ d.breakdown_number }}</span>
            <span class="text-xs text-gray-500">{{ formatDateTime(d.created_at) }} · {{ d.created_by || '—' }}</span>
          </div>
          <div class="mt-1 text-sm text-gray-700 break-words">
            <b>{{ d.input_item_name }}</b>
            <span v-if="d.pattern_name" class="mx-0.5 inline-block rounded bg-pink-50 px-1.5 py-0.5 align-middle text-[11px] font-medium text-pink-700">{{ d.pattern_name }}</span>
            {{ fmtNum(d.input_qty_dist) }} {{ d.input_unit }}
            <span class="text-gray-400">({{ fmtNum(d.input_qty_base) }} {{ d.input_base_unit }})</span>
            · {{ formatRupiah(d.input_cost_total) }}
          </div>
          <div class="text-xs text-gray-500 break-words">
            {{ d.warehouse_name }}<span v-if="d.expiry_date"> · kedaluwarsa hasil {{ formatDateStr(d.expiry_date) }}</span>
          </div>
          <div class="mt-2 divide-y divide-gray-100 rounded-md bg-gray-50 px-2">
            <div v-for="o in d.outputs" :key="o.id" class="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5 py-1.5 text-xs">
              <span class="font-medium text-gray-800 break-words">{{ o.item_name }}</span>
              <span class="text-gray-600">
                {{ fmtNum(o.qty_base) }}/{{ fmtNum(o.std_qty_base) }} {{ o.base_unit }}
                <b :class="yieldTextCls(o.yield_pct)">({{ fmtPct(o.yield_pct) }})</b>
                · {{ formatRupiah(o.cost_per_base) }}/{{ o.base_unit }}
              </span>
            </div>
          </div>
          <div class="mt-1.5 text-xs text-gray-500">
            Hasil dibanding standar <b :class="yieldTextCls(d.yield_pct)">{{ fmtPct(d.yield_pct) }}</b>
            <span v-if="d.shrink_cost > 0"> · susut {{ formatRupiah(d.shrink_cost) }}</span>
            <span v-else-if="d.shrink_cost < 0"> · hasil melebihi standar</span>
          </div>
          <p v-if="d.notes" class="mt-1 text-xs italic text-gray-500 break-words">{{ d.notes }}</p>
        </li>
      </ul>
      <AppPagination v-model:page="bdHistPage" :total-pages="bdHistTotalPages" />
      <template #footer>
        <AppButton variant="secondary" @click="showBdHistory = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import RupiahInput from '@/components/ui/RupiahInput.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { stockQtyText, stockUnitOptions, stockUnitRatio, stockUnitLabel, preferredStockUnit } from '@/utils/stockUnits.js'
import { computed, ref, watch, onMounted } from 'vue'
import { warehousesApi, stockLedgerApi, stockItemsApi, stockBreakdownsApi } from '@/api/warehouse.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppInput      from '@/components/ui/AppInput.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchSelect  from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import { formatWarehouseOptionLabel, MOVEMENT_LABELS as MOV_LABELS, movementClass as movTypeCls } from '@/utils/warehouse.js'
import { formatRupiah, formatDateTime, formatDateStr } from '@/utils/format.js'
import debounce from 'lodash/debounce'

const toast = useToastStore()
const auth = useAuthStore()
// Pemotongan dicatat pemegang stockledger.adjust (sama dengan penyesuaian stok).
const canBreakdown = computed(() => auth.hasPermission('stockledger.adjust'))
const canProduce = computed(() => auth.hasPermission('stockledger.adjust'))
const loading = ref(false)
const saving = ref(false)
const rows = ref([])
const total = ref(0)
const lowStockCount = ref(0)
const totalAssetValue = ref(0)
const page = ref(1)
const limit = 30
const totalPages = ref(1)
const search = ref('')
const filterWarehouse = ref('')
const lowStockOnly = ref(false)
const warehouseList = ref([])

// Movements
const showMovements = ref(false)
const movTarget = ref(null)
const movements = ref([])
const movLoading = ref(false)
const movDateFrom = ref('')
const movRange = ref({ from: '', to: '', label: 'Semua Tanggal' })
watch(movRange, (r) => { movDateFrom.value = r.from; movDateTo.value = r.to; loadMovements() })
const movDateTo = ref('')

// Adjustment
const showAdjust = ref(false)
const adjustTarget = ref(null)
const adjForm = ref({ movement_type: 'purchase_in', qty_base: 0, cost_per_base: 0, notes: '' })
const adjustUnit = ref('')
const adjustRatio = computed(() => stockUnitRatio(adjustTarget.value, adjustUnit.value))
const adjustQty = computed({
  get: () => Number((adjForm.value.qty_base / adjustRatio.value).toFixed(4)),
  set: value => { adjForm.value.qty_base = Number(value) * adjustRatio.value },
})
const adjustCost = computed({
  get: () => Number((adjForm.value.cost_per_base * adjustRatio.value).toFixed(4)),
  set: value => { adjForm.value.cost_per_base = Number(value) / adjustRatio.value },
})

// Produce
const showProduce = ref(false)
const produceForm = ref({ item_id: '', warehouse_id: '', qty_produce: 0, notes: '' })
const allStockItems = ref([])

const produceItemOptions = computed(() =>
  allStockItems.value.map(i => ({ value: i.id, label: `${i.name} (${i.code})` }))
)

function openProduceModal() {
  if (!canProduce.value) return
  produceForm.value = { item_id: '', warehouse_id: filterWarehouse.value, qty_produce: 0, notes: '' }
  showProduce.value = true
  if (allStockItems.value.length === 0) loadAllItems()
}

async function loadAllItems() {
  try {
    const res = await stockItemsApi.list({ limit: 500, active_only: true })
    allStockItems.value = res.data || []
  } catch (e) {}
}

async function submitProduce() {
  if (!canProduce.value || saving.value) return
  if (!produceForm.value.item_id || !produceForm.value.warehouse_id || produceForm.value.qty_produce <= 0) {
    toast.error('Data produksi tidak lengkap')
    return
  }
  saving.value = true
  try {
    await stockLedgerApi.produce(produceForm.value)
    toast.success('Proses produksi berhasil')
    showProduce.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal memproses produksi')
  } finally {
    saving.value = false
  }
}

// ── Pemotongan (satu bahan → beberapa hasil) ──────────────────
// Pola pemotongan (hasil standar + bobot biaya; satu bahan bisa beberapa
// pola) dikelola F & B di pusat resep. Form ini mengisi hasil standar pola
// terpilih × qty sebagai awal; petugas mengubahnya ke hasil nyata. Server
// menghitung biaya pasti dari FIFO; angka di sini perkiraan dari HPP
// rata-rata gudang dengan rumus pembagian yang sama.
const showBreakdown = ref(false)
const bdTemplates = ref([])
const bdTemplatesLoading = ref(false)
const bdForm = ref({ warehouse_id: '', input_item_id: '', pattern_id: '', input_qty_dist: 1, notes: '' })
const bdRows = ref([])
const bdStock = ref(null) // { qty_base, avg_cost } bahan di gudang terpilih; null = belum dimuat

const bdTemplate = computed(() => bdTemplates.value.find(t => t.input_item_id === bdForm.value.input_item_id) || null)
// Pola aktif bahan terpilih; server hanya mengirim pola aktif yang berisi hasil.
const bdPatterns = computed(() => bdTemplate.value?.patterns || [])
const bdPattern = computed(() => bdPatterns.value.find(p => p.id === bdForm.value.pattern_id) || null)
const bdInputOptions = computed(() => bdTemplates.value
  .filter(t => t.input_is_active)
  .map(t => ({
    value: t.input_item_id,
    label: `${t.input_item_name} (${t.input_item_code})${t.patterns.length > 1 ? ` · ${t.patterns.length} pola` : ''}`,
  })))
// Role per outlet hanya di gudang outletnya (server menolak Gudang Induk dan
// outlet lain); daftar gudang dari server sudah terbatas cakupannya.
const bdWarehouseOptions = computed(() => warehouseList.value
  .filter(w => w.is_active && (auth.scopeType !== 'specific' || w.type === 'outlet'))
  .map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) })))
const bdDistLabel = computed(() => {
  const t = bdTemplate.value
  return t ? (t.input_dist_unit_label || t.input_dist_unit || t.input_base_unit) : ''
})
const bdQty = computed(() => Number(bdForm.value.input_qty_dist) || 0)
const bdInputBase = computed(() => bdQty.value * (bdTemplate.value?.input_dist_ratio || 1))
const bdEstCost = computed(() => bdInputBase.value * (bdStock.value?.avg_cost || 0))
const bdStockShort = computed(() => !!bdStock.value && bdInputBase.value > bdStock.value.qty_base + 1e-9)
const bdStockText = computed(() => {
  const t = bdTemplate.value
  if (!t || !bdForm.value.warehouse_id) return '—'
  if (!bdStock.value) return 'memuat…'
  const base = `${fmtNum(bdStock.value.qty_base)} ${t.input_base_unit}`
  const ratio = t.input_dist_ratio || 1
  return ratio !== 1 ? `${fmtNum(bdStock.value.qty_base / ratio)} ${bdDistLabel.value} (${base})` : base
})

function bdStd(r) { return bdQty.value * r.qty_per_input }
function bdYield(r) {
  const std = bdStd(r)
  return std > 0 ? (Number(r.qty) || 0) / std * 100 : null
}
// Rumus server (allocateBreakdownCost): porsi = qty × bobot ÷ hasil standar.
const bdShareSum = computed(() => bdRows.value.reduce((s, r) =>
  s + (Number(r.qty) > 0 ? Number(r.qty) * r.cost_weight / r.qty_per_input : 0), 0))
function bdEstUnitCost(r) {
  if (!(Number(r.qty) > 0) || bdShareSum.value <= 0) return 0
  return bdEstCost.value * (r.cost_weight / r.qty_per_input) / bdShareSum.value
}
function bdEstUnitCostText(r) {
  const c = bdEstUnitCost(r)
  return c > 0 ? `${formatRupiah(c)}/${r.unit}` : '—'
}
// Yield ditimbang bobot biaya (sama dengan yield dokumen di server).
const bdValueYield = computed(() => {
  const std = bdRows.value.reduce((s, r) => s + bdStd(r) * r.cost_weight / r.qty_per_input, 0)
  return std > 0 ? bdShareSum.value / std * 100 : null
})

function round3(v) { return Math.round(v * 1000) / 1000 }
function fmtNum(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 3 }) }
function fmtPct(v) {
  return v === null || v === undefined ? '—' : `${Number(v).toLocaleString('id-ID', { maximumFractionDigits: 1 })}%`
}
// Hasil ≥ standar hijau, ≥ 90% kuning, di bawahnya merah.
function yieldTextCls(p) {
  if (p === null || p === undefined) return 'text-gray-500'
  return p >= 99.95 ? 'text-emerald-700' : p >= 90 ? 'text-amber-600' : 'text-red-600'
}
function yieldBadgeCls(p) {
  if (p === null || p === undefined) return 'bg-gray-100 text-gray-500'
  return p >= 99.95 ? 'bg-emerald-100 text-emerald-700' : p >= 90 ? 'bg-amber-100 text-amber-700' : 'bg-red-100 text-red-600'
}

function buildBdRows() {
  const p = bdPattern.value
  bdRows.value = p ? p.outputs.map(o => ({
    item_id: o.output_item_id, name: o.output_item_name, unit: o.output_base_unit,
    qty_per_input: o.qty_per_input, cost_weight: o.cost_weight,
    qty: round3(bdQty.value * o.qty_per_input), touched: false,
  })) : []
}

let bdStockSeq = 0
async function loadBdStock() {
  const seq = ++bdStockSeq
  bdStock.value = null
  const { warehouse_id, input_item_id } = bdForm.value
  if (!warehouse_id || !input_item_id) return
  try {
    const data = await stockLedgerApi.list({ warehouse_id, item_id: input_item_id, limit: 1 })
    if (seq !== bdStockSeq) return
    const row = (data.data || [])[0]
    bdStock.value = { qty_base: row?.qty_base || 0, avg_cost: row?.avg_cost || 0 }
  } catch (e) {
    if (seq === bdStockSeq) bdStock.value = { qty_base: 0, avg_cost: 0 }
  }
}

// Bahan berganti → pola terpilih otomatis bila hanya ada satu; selain itu
// petugas memilih sendiri (server menolak tanpa pola bila lebih dari satu).
watch(() => bdForm.value.input_item_id, () => {
  const pats = bdPatterns.value
  bdForm.value.pattern_id = pats.length === 1 ? pats[0].id : ''
  buildBdRows()
  loadBdStock()
})
watch(() => bdForm.value.pattern_id, buildBdRows)
watch(() => bdForm.value.warehouse_id, loadBdStock)
// Qty bahan berubah → hasil yang belum diubah petugas ikut standar baru.
watch(bdQty, () => { bdRows.value.forEach(r => { if (!r.touched) r.qty = round3(bdStd(r)) }) })

async function openBreakdownModal() {
  const opts = bdWarehouseOptions.value
  const wh = opts.some(o => o.value === filterWarehouse.value) ? filterWarehouse.value
    : (opts.length === 1 ? opts[0].value : '')
  bdForm.value = { warehouse_id: wh, input_item_id: '', pattern_id: '', input_qty_dist: 1, notes: '' }
  bdRows.value = []
  bdStock.value = null
  showBreakdown.value = true
  bdTemplatesLoading.value = true
  try {
    // Non-paginated: apiClient mengembalikan array langsung.
    const data = await stockBreakdownsApi.templates()
    bdTemplates.value = Array.isArray(data) ? data : (data?.data || [])
  } catch (e) {
    bdTemplates.value = []
    toast.error(e?.message || 'Gagal memuat pola pemotongan')
  } finally {
    bdTemplatesLoading.value = false
  }
}

async function submitBreakdown() {
  const t = bdTemplate.value
  if (!bdForm.value.warehouse_id || !t) {
    toast.error('Pilih gudang dan bahan yang dipotong')
    return
  }
  if (!bdPattern.value) {
    toast.error('Pilih pola pemotongan')
    return
  }
  if (!(bdQty.value > 0)) {
    toast.error('Jumlah bahan yang dipakai harus lebih dari 0')
    return
  }
  if (bdRows.value.some(r => Number(r.qty) < 0)) {
    toast.error('Hasil aktual tidak boleh negatif')
    return
  }
  if (!bdRows.value.some(r => Number(r.qty) > 0)) {
    toast.error('Isi minimal satu hasil potongan')
    return
  }
  saving.value = true
  try {
    const doc = await stockBreakdownsApi.create({
      warehouse_id: bdForm.value.warehouse_id,
      input_item_id: t.input_item_id,
      pattern_id: bdPattern.value.id,
      input_qty_dist: bdQty.value,
      outputs: bdRows.value.map(r => ({ item_id: r.item_id, qty_base: Number(r.qty) || 0 })),
      notes: bdForm.value.notes,
    })
    toast.success(`Pemotongan ${doc?.breakdown_number || ''} dicatat`)
    showBreakdown.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal mencatat pemotongan')
  } finally {
    saving.value = false
  }
}

// Riwayat dokumen pemotongan (ikut cakupan outlet di server).
const showBdHistory = ref(false)
const bdHistory = ref([])
const bdHistLoading = ref(false)
const bdHistPage = ref(1)
const bdHistTotalPages = ref(1)
const bdHistWarehouse = ref('')

function openBreakdownHistory() {
  bdHistWarehouse.value = filterWarehouse.value
  bdHistory.value = []
  showBdHistory.value = true
  reloadBdHistory()
}

function reloadBdHistory() {
  // watch(bdHistPage) yang memuat ulang saat pindah ke hal. 1
  if (bdHistPage.value === 1) loadBdHistory()
  else bdHistPage.value = 1
}

let bdHistSeq = 0
async function loadBdHistory() {
  const seq = ++bdHistSeq
  bdHistLoading.value = true
  try {
    const data = await stockBreakdownsApi.list({ warehouse_id: bdHistWarehouse.value, page: bdHistPage.value, limit: 10 })
    if (seq !== bdHistSeq) return
    bdHistory.value = data.data || []
    bdHistTotalPages.value = data.total_pages || 1
  } catch (e) {
    if (seq === bdHistSeq) toast.error(e?.message || 'Gagal memuat riwayat pemotongan')
  } finally {
    if (seq === bdHistSeq) bdHistLoading.value = false
  }
}
watch(bdHistPage, loadBdHistory)

const COLUMNS = [
  { key: 'item_name',      label: 'Bahan Baku', sortable: false },
  { key: 'warehouse_name', label: 'Gudang',     sortable: false },
  { key: 'qty_base',       label: 'Stok',       sortable: false },
  { key: 'avg_cost',       label: 'HPP',        sortable: false },
  { key: 'stock_value',    label: 'Nilai Stok', sortable: false },
  { key: 'actions',        label: '',           sortable: false },
]

const ADJ_TYPES = [
  { value: 'purchase_in', label: 'Pembelian Masuk' },
  { value: 'adjustment',  label: 'Koreksi Stok' },
  { value: 'return_in',   label: 'Retur Masuk' },
]

const warehouseMap = computed(() => Object.fromEntries(warehouseList.value.map(w => [w.id, w])))
const warehouseOptions = computed(() =>
  [
    { value: '', label: 'Semua Gudang' },
    ...warehouseList.value.map(w => ({ value: w.id, label: formatWarehouseOptionLabel(w) })),
  ]
)

const debouncedLoad = debounce(applyFilters, 400)

let loadSeq = 0
async function load() {
  const seq = ++loadSeq
  loading.value = true
  try {
    const data = await stockLedgerApi.list({
      page: page.value, limit,
      warehouse_id: filterWarehouse.value,
      search: search.value,
      low_stock: lowStockOnly.value ? 'true' : '',
    })
    if (seq !== loadSeq) return // respons lama, sudah ada request lebih baru
    rows.value = data.data || []
    total.value = data.total || 0
    lowStockCount.value = data.low_stock_count || 0
    totalAssetValue.value = data.total_asset_value || 0
    totalPages.value = data.total_pages || 1
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat kartu stok')
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  // watch(page) yang memanggil load; hindari request ganda saat pindah ke hal. 1
  if (page.value === 1) load()
  else page.value = 1
}

function getWarehouseLabel(row) {
  return formatWarehouseOptionLabel(warehouseMap.value[row.warehouse_id] || row)
}

async function loadWarehouses() {
  try {
    const data = await warehousesApi.list({ limit: 200 })
    warehouseList.value = data.data || []
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat daftar gudang')
  }
}

watch(page, load)
onMounted(() => { load(); loadWarehouses() })

function openMovements(row) {
  movTarget.value = row
  movements.value = []
  showMovements.value = true
  // Reset filter tanggal dari item sebelumnya; watch(movRange) yang memanggil
  // loadMovements (objek baru selalu memicu watch).
  movRange.value = { from: '', to: '', label: 'Semua Tanggal' }
}

async function loadMovements() {
  movLoading.value = true
  try {
    const data = await stockLedgerApi.movements({
      item_id: movTarget.value.item_id,
      warehouse_id: movTarget.value.warehouse_id,
      date_from: movDateFrom.value,
      date_to: movDateTo.value,
      limit: 100,
    })
    movements.value = data.data || []
  } catch (e) {
    movements.value = []
    toast.error(e?.message || 'Gagal memuat histori stok')
  } finally {
    movLoading.value = false
  }
}

function openAdjust(row) {
  adjustTarget.value = row
  adjustUnit.value = preferredStockUnit(row, row.warehouse_type)
  adjForm.value = { movement_type: 'purchase_in', qty_base: 0, cost_per_base: 0, expiry_date: '', notes: '' }
  showAdjust.value = true
}

async function submitAdjust() {
  saving.value = true
  try {
    await stockLedgerApi.adjust({
      item_id: adjustTarget.value.item_id,
      warehouse_id: adjustTarget.value.warehouse_id,
      qty_base: adjForm.value.qty_base,
      cost_per_base: adjForm.value.cost_per_base,
      movement_type: adjForm.value.movement_type,
      expiry_date: adjForm.value.expiry_date,
      notes: adjForm.value.notes,
    })
    toast.success('Penyesuaian stok berhasil dicatat')
    showAdjust.value = false
    load()
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan')
  } finally {
    saving.value = false
  }
}
</script>
