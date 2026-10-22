<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Laporan PPIC</h1>
        <p class="text-xs text-gray-500 mt-0.5">Distribusi induk → outlet, variance pemakaian & food cost, realisasi produksi, akurasi forecast, dan kinerja vendor.</p>
      </div>
      <AppButton v-if="canExport" variant="secondary" :loading="exporting" @click="doExport">
        <svg class="inline-block align-[-2px] mr-1" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        Export Excel
      </AppButton>
    </div>

    <!-- Tabs -->
    <div class="tabs">
      <button v-for="t in TABS" :key="t.key" class="tab" :class="{ 'tab--active': tab === t.key }" @click="switchTab(t.key)">
        {{ t.label }}
      </button>
    </div>

    <!-- Filters -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-3">
        <template v-if="tab !== 'hpp'">
          <div>
            <label class="text-xs font-semibold text-gray-600">Dari</label>
            <input type="date" v-model="dateFrom" class="block text-sm border border-gray-200 rounded-lg px-3 py-2" />
          </div>
          <div>
            <label class="text-xs font-semibold text-gray-600">Sampai</label>
            <input type="date" v-model="dateTo" class="block text-sm border border-gray-200 rounded-lg px-3 py-2" />
          </div>
        </template>
        <div v-if="tab === 'variance' || tab === 'production'" class="min-w-[220px]">
          <label class="text-xs font-semibold text-gray-600">Gudang</label>
          <SearchSelect v-model="warehouseId" :options="warehouseOptions" placeholder="Semua Gudang" />
        </div>
        <div v-if="tab === 'forecast' || tab === 'hpp' || tab === 'sold' || tab === 'distribution'" class="min-w-[220px]">
          <label class="text-xs font-semibold text-gray-600">Outlet</label>
          <SearchSelect v-model="outletId" :options="outletOptions" placeholder="Semua Outlet" />
        </div>
        <div v-if="tab === 'distribution'" class="flex-1 min-w-[180px]">
          <label class="text-xs font-semibold text-gray-600">Cari Item / Kode</label>
          <input v-model="distSearch" placeholder="Nama item atau kode bahan..."
            class="block w-full text-sm border border-gray-200 rounded-lg px-3 py-2" />
        </div>
        <div v-if="tab === 'sold'" class="flex-1 min-w-[180px]">
          <label class="text-xs font-semibold text-gray-600">Cari Produk / Kategori</label>
          <input v-model="soldSearch" placeholder="Nama produk atau kategori..."
            class="block w-full text-sm border border-gray-200 rounded-lg px-3 py-2" />
        </div>
        <div v-if="tab === 'hpp'" class="flex-1 min-w-[180px]">
          <label class="text-xs font-semibold text-gray-600">Cari Menu</label>
          <input v-model="hppSearch" placeholder="Nama menu..."
            class="block w-full text-sm border border-gray-200 rounded-lg px-3 py-2" />
        </div>
        <AppButton variant="primary" :loading="loading" @click="applyFilters">Tampilkan</AppButton>
      </div>
    </AppCard>

    <div v-if="loading" class="p-10 text-center"><AppSpinner size="lg" class="text-emerald-600 mx-auto" /></div>

    <!-- ══ PRODUK TERJUAL (qty-only) ══ -->
    <template v-else-if="tab === 'sold' && rep">
      <div class="stat-grid">
        <div class="stat"><div class="stat-l">Total Terjual</div><div class="stat-v">{{ fmtQty(rep.total_qty) }}</div><div class="stat-s">porsi dalam {{ rep.days }} hari</div></div>
        <div class="stat"><div class="stat-l">Produk</div><div class="stat-v">{{ rep.product_count }}</div><div class="stat-s">produk berbeda terjual</div></div>
        <div class="stat"><div class="stat-l">Rata-rata / Hari</div><div class="stat-v">{{ fmtQty(rep.days ? rep.total_qty / rep.days : 0) }}</div><div class="stat-s">porsi per hari (semua produk)</div></div>
        <div class="stat" :class="rep.no_recipe_qty > 0 ? 'stat--red' : 'stat--ok'">
          <div class="stat-l">Terjual Tanpa Resep</div><div class="stat-v">{{ fmtQty(rep.no_recipe_qty) }}</div><div class="stat-s">porsi — tidak terhitung di MRP/variance</div>
        </div>
      </div>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th>#</th><th>Produk</th><th>Kategori</th><th>Outlet</th><th class="th-r">Qty Terjual</th><th class="th-r">Rata²/Hari</th><th class="th-r">Kontribusi</th><th>Resep</th></tr></thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="8" class="p-8 text-center text-sm text-gray-400">Tidak ada penjualan pada rentang ini.</td></tr>
              <tr v-for="(r, i) in rep.rows" :key="i">
                <td class="text-xs text-gray-400">{{ (reportPage - 1) * PAGE_SIZE + i + 1 }}</td>
                <td class="text-sm font-medium text-gray-900">{{ r.product_name }}</td>
                <td><span class="cat-chip">{{ r.category || 'Tanpa Kategori' }}</span></td>
                <td><span class="outlet-chip">{{ r.outlet_name }}</span></td>
                <td class="td-num font-bold">{{ fmtQty(r.qty) }}</td>
                <td class="td-num text-gray-500">{{ fmtQty(r.avg_per_day) }}</td>
                <td class="td-num">
                  <div class="share-wrap">
                    <span class="text-xs">{{ r.share_pct.toFixed(1) }}%</span>
                    <div class="share-bar"><div class="share-fill" :style="{ width: Math.min(100, r.share_pct * 4) + '%' }"></div></div>
                  </div>
                </td>
                <td>
                  <span v-if="r.has_recipe" class="pill-ok"><svg class="inline-block align-[-2px]" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg></span>
                  <router-link v-else to="/fnb/recipes" class="pill-no" title="Produk belum punya resep — klik untuk melengkapi">belum</router-link>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ HPP MENU ══ -->
    <template v-else-if="tab === 'hpp' && rep">
      <div class="stat-grid">
        <div class="stat"><div class="stat-l">Menu</div><div class="stat-v">{{ rep.product_count }}</div><div class="stat-s">{{ rep.with_recipe }} ber-resep</div></div>
        <div class="stat">
          <div class="stat-l">Rata-rata HPP</div><div class="stat-v">{{ rep.avg_hpp_pct.toFixed(1) }}%</div><div class="stat-s">batas mengikuti outlet / menu</div>
        </div>
        <div class="stat" :class="rep.over_count ? 'stat--red' : 'stat--ok'">
          <div class="stat-l">Mencapai Batas</div><div class="stat-v">{{ rep.over_count }}</div><div class="stat-s">menu perlu ditinjau (harga/porsi/resep)</div>
        </div>
        <div class="stat"><div class="stat-l">Basis Biaya</div><div class="stat-s" style="margin-top:.35rem">Sama dengan F&B: biaya stok outlet + kemasan + overhead. Proyeksi induk untuk perencanaan; keuangan memakai pemakaian batch aktual.</div></div>
      </div>

      <div v-if="rep.no_recipe_sample" class="warn-note">
        <svg class="inline-block align-[-2px] mr-0.5" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>
        Menu tanpa resep (HPP tidak bisa dihitung): {{ rep.no_recipe_sample }}<span v-if="rep.product_count - rep.with_recipe > 10">, …</span>
        — total {{ rep.product_count - rep.with_recipe }} menu. Lengkapi di halaman <router-link to="/fnb/recipes" class="underline font-semibold">Resep</router-link>.
      </div>

      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th></th><th>Menu</th><th>Outlet</th><th>Kategori</th><th class="th-r">HPP</th><th class="th-r">Harga Jual</th><th class="th-r">HPP %</th><th class="th-r">Margin</th><th class="th-r">Batas biaya / menu</th></tr></thead>
            <tbody>
              <tr v-if="!hppRows.length"><td colspan="9" class="p-8 text-center text-sm text-gray-400">Tidak ada menu.</td></tr>
              <template v-for="(r, i) in hppRows" :key="r.product_id">
                <tr class="cursor-pointer" :class="{ 'row-over': r.is_over }" @click="hppOpen[r.product_id] = !hppOpen[r.product_id]">
                  <td class="w-8 text-center text-gray-400">{{ r.has_recipe ? (hppOpen[r.product_id] ? '▾' : '▸') : '' }}</td>
                  <td class="text-sm font-medium text-gray-900">{{ r.product_name }}</td>
                  <td class="text-xs text-gray-500">{{ r.outlet_name }}</td>
                  <td class="text-xs text-gray-500">{{ r.category || '—' }}</td>
                  <td class="td-num">{{ r.cost_status === 'ready' ? fmtRpFull(r.hpp) : '—' }}<span class="block text-xs text-gray-400">{{ r.cost_status === 'ready' ? (r.cost_basis === 'estimate' ? 'Acuan / estimasi' : 'Stok outlet') : 'Biaya belum lengkap / dibatasi' }}</span><span v-if="r.projected_hpp != null" class="block text-xs" :class="r.projected_is_over ? 'text-amber-700' : 'text-gray-400'">Proyeksi {{ fmtRpFull(r.projected_hpp) }}</span></td>
                  <td class="td-num">{{ fmtRpFull(r.price) }}</td>
                  <td class="td-num font-bold" :class="!r.has_recipe ? 'text-gray-300' : (r.is_over ? 'text-red-600' : 'text-emerald-700')">
                    {{ r.cost_status === 'ready' && r.price > 0 ? r.hpp_pct.toFixed(1) + '%' : '—' }}
                  </td>
                  <td class="td-num">{{ r.cost_status === 'ready' && r.price > 0 ? fmtRpFull(r.margin) : '—' }}</td>
                  <td class="td-num text-gray-500">{{ r.price > 0 ? fmtRpFull(r.ideal_cost) : '—' }}<span class="block text-xs">{{ r.target_cost_pct }}%</span></td>
                </tr>
                <tr v-if="hppOpen[r.product_id] && r.ingredients?.length">
                  <td></td>
                  <td colspan="8" class="!p-0">
                    <div class="p-3 text-xs text-gray-500">Kemasan {{ fmtRpFull(r.packaging_cost) }} + overhead {{ fmtRpFull(r.overhead_cost) }} / menu.<span v-if="r.projected_cost_pct != null"> Proyeksi {{ r.projected_cost_pct.toFixed(1) }}% dari harga jual.</span></div>
                    <table class="ing-table">
                      <thead><tr><th>Bahan / Komponen</th><th class="th-r">Qty</th><th>Satuan</th><th class="th-r">Harga/Satuan</th><th class="th-r">Cost</th></tr></thead>
                      <tbody>
                        <tr v-for="ing in r.ingredients" :key="ing.item_id">
                          <td>
                            <span v-if="ing.is_wip" class="wip-tag">WIP</span>
                            {{ ing.item_name || ing.item_id }}
                          </td>
                          <td class="td-num">{{ fmtQty(ing.qty_base) }}</td>
                          <td class="text-xs text-gray-500">{{ ing.unit }}</td>
                          <td class="td-num">{{ fmtRpFull(ing.cost_per_base) }}</td>
                          <td class="td-num font-semibold">{{ fmtRpFull(ing.cost) }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ VARIANCE ══ -->
    <template v-else-if="tab === 'variance' && rep">
      <p class="text-xs text-gray-500 mb-3">Pemakaian aktual memakai mutasi pada periode laporan. Kebutuhan teoretis adalah simulasi berdasarkan resep yang berlaku saat laporan dibuka; perubahan resep dapat mengubah simulasi, tanpa mengubah pemakaian aktual.</p>
      <div v-if="!rep.representative && rep.qty_sold > 0" class="warn-note">
        <svg class="inline-block align-[-2px] mr-0.5" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>
        Coverage resep baru <b>{{ rep.recipe_coverage_pct.toFixed(1) }}%</b> dari qty terjual — angka variance <b>belum representatif</b>.
        Lengkapi resep produk agar analisis akurat (target ≥ 80%).
      </div>
      <div class="stat-grid">
        <div class="stat"><div class="stat-l">Pemakaian Teoretis</div><div class="stat-v">{{ fmtRp(rep.theo_value) }}</div><div class="stat-s">penjualan × resep + produksi × resep</div></div>
        <div class="stat"><div class="stat-l">Pemakaian Aktual</div><div class="stat-v">{{ fmtRp(rep.act_value) }}</div><div class="stat-s">buku stok (tanpa transfer)</div></div>
        <div class="stat" :class="Math.abs(rep.variance_pct) > 5 ? 'stat--red' : 'stat--ok'">
          <div class="stat-l">Variance</div><div class="stat-v">{{ rep.variance_pct > 0 ? '+' : '' }}{{ rep.variance_pct.toFixed(1) }}%</div><div class="stat-s">target &lt; ±5%</div>
        </div>
        <div class="stat">
          <div class="stat-l">Food Cost Aktual</div><div class="stat-v">{{ rep.food_cost_pct.toFixed(1) }}%</div><div class="stat-s">dari omzet {{ fmtRp(rep.revenue) }} · biaya aktual periode laporan</div>
        </div>
      </div>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th>Item</th><th>Kategori</th><th class="th-r">Teoretis</th><th class="th-r">Aktual</th><th class="th-r">Selisih Qty</th><th class="th-r">Selisih Nilai</th><th class="th-r">Variance</th></tr></thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="7" class="p-8 text-center text-sm text-gray-400">Tidak ada data pada rentang ini.</td></tr>
              <tr v-for="r in rep.rows" :key="r.item_id" :class="{ 'row-over': r.is_over }">
                <td><div class="font-medium text-gray-900 text-sm">{{ r.item_name }}</div><div class="text-[11px] text-gray-400 font-mono">{{ r.item_code }}</div></td>
                <td class="text-xs text-gray-500">{{ r.category || '—' }}</td>
                <td class="td-num">{{ fmtQty(r.theo_qty) }} {{ r.base_unit }}<div class="text-[10px] text-gray-400">{{ fmtRp(r.theo_value) }}</div></td>
                <td class="td-num">{{ fmtQty(r.act_qty) }}<div class="text-[10px] text-gray-400">{{ fmtRp(r.act_value) }}</div></td>
                <td class="td-num" :class="r.diff_qty > 0 ? 'text-red-600' : 'text-emerald-700'">{{ r.diff_qty > 0 ? '+' : '' }}{{ fmtQty(r.diff_qty) }}</td>
                <td class="td-num" :class="r.diff_value > 0 ? 'text-red-600' : 'text-emerald-700'">{{ fmtRp(r.diff_value) }}</td>
                <td class="td-num font-bold" :class="r.is_over ? 'text-red-600' : 'text-gray-600'">{{ r.variance_pct > 0 ? '+' : '' }}{{ r.variance_pct.toFixed(1) }}%</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ PRODUKSI ══ -->
    <template v-else-if="tab === 'production' && rep">
      <div class="stat-grid">
        <div class="stat"><div class="stat-l">WO Selesai</div><div class="stat-v">{{ rep.wo_done }}</div><div class="stat-s">pada rentang</div></div>
        <div class="stat" :class="rep.avg_yield_pct >= 90 ? 'stat--ok' : 'stat--red'"><div class="stat-l">Rata-rata Yield</div><div class="stat-v">{{ rep.avg_yield_pct.toFixed(1) }}%</div><div class="stat-s">target ≥ 90%</div></div>
        <div class="stat" :class="rep.plan_adherence_pct >= 90 ? 'stat--ok' : 'stat--red'"><div class="stat-l">Plan Adherence</div><div class="stat-v">{{ rep.plan_adherence_pct.toFixed(1) }}%</div><div class="stat-s">Σ aktual / Σ rencana</div></div>
        <div class="stat"><div class="stat-l">Total HPP Produksi</div><div class="stat-v">{{ fmtRp(rep.total_cost) }}</div><div class="stat-s">variance bahan {{ fmtRp(rep.total_mat_variance) }}</div></div>
      </div>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th>WO</th><th>Item</th><th>Gudang</th><th class="th-r">Rencana</th><th class="th-r">Aktual</th><th class="th-r">Yield</th><th class="th-r">HPP/Unit</th><th class="th-r">Var. Bahan</th><th>Selesai</th></tr></thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="9" class="p-8 text-center text-sm text-gray-400">Belum ada WO selesai pada rentang ini.</td></tr>
              <tr v-for="r in rep.rows" :key="r.wo_number">
                <td><div class="font-mono text-xs font-bold text-gray-700">{{ r.wo_number }}</div><div class="text-[10px] text-gray-400">{{ r.plan_number }}</div></td>
                <td class="text-sm font-medium text-gray-900">{{ r.item_name }}</td>
                <td class="text-xs text-gray-600">{{ r.warehouse_name }}</td>
                <td class="td-num">{{ fmtQty(r.qty_planned) }} {{ r.base_unit }}</td>
                <td class="td-num">{{ fmtQty(r.qty_actual) }}</td>
                <td class="td-num font-bold" :class="r.yield_pct >= 90 ? 'text-emerald-700' : 'text-amber-600'">{{ r.yield_pct.toFixed(1) }}%</td>
                <td class="td-num">{{ fmtRp(r.cost_per_unit) }}</td>
                <td class="td-num" :class="r.mat_variance_value > 0 ? 'text-red-600' : 'text-gray-500'">{{ fmtRp(r.mat_variance_value) }}</td>
                <td class="text-xs text-gray-500">{{ fmtDate(r.finished_at) }}<div class="text-[10px]">{{ r.executed_by }}</div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ FORECAST ══ -->
    <template v-else-if="tab === 'forecast' && rep">
      <div class="stat-grid stat-grid--2">
        <div class="stat" :class="rep.accuracy_pct >= 80 ? 'stat--ok' : 'stat--red'"><div class="stat-l">Akurasi Agregat</div><div class="stat-v">{{ rep.eval_rows ? rep.accuracy_pct.toFixed(1) + '%' : '—' }}</div><div class="stat-s">100 − MAPE · target ≥ 80%</div></div>
        <div class="stat"><div class="stat-l">Titik Evaluasi</div><div class="stat-v">{{ rep.eval_rows }}</div><div class="stat-s">hari-produk dengan penjualan aktual</div></div>
      </div>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th>Outlet</th><th>Produk</th><th class="th-r">Titik Evaluasi</th><th class="th-r">Akurasi</th><th class="th-r">Bias</th></tr></thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="5" class="p-8 text-center text-sm text-gray-400">Belum ada data evaluasi — forecast perlu berjalan beberapa hari dulu.</td></tr>
              <tr v-for="(r, i) in rep.rows" :key="i">
                <td class="text-xs text-gray-600">{{ r.outlet_name }}</td>
                <td class="text-sm font-medium text-gray-900">{{ r.product_name }}</td>
                <td class="td-num">{{ r.eval_rows }}</td>
                <td class="td-num font-bold" :class="r.accuracy_pct >= 80 ? 'text-emerald-700' : 'text-amber-600'">{{ r.accuracy_pct.toFixed(1) }}%</td>
                <td class="td-num" :class="r.bias_pct > 0 ? 'text-blue-700' : 'text-red-600'">
                  {{ r.bias_pct > 0 ? '+' : '' }}{{ r.bias_pct.toFixed(1) }}%
                  <span class="text-[10px] text-gray-400">{{ r.bias_pct > 0 ? 'over' : 'under' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ OTIF ══ -->
    <template v-else-if="tab === 'otif' && rep">
      <div class="stat-grid stat-grid--2">
        <div class="stat" :class="rep.otif_pct >= 90 ? 'stat--ok' : 'stat--red'"><div class="stat-l">OTIF Agregat</div><div class="stat-v">{{ rep.total_pr ? rep.otif_pct.toFixed(1) + '%' : '—' }}</div><div class="stat-s">tepat waktu / diterima · target ≥ 90%</div></div>
        <div class="stat"><div class="stat-l">Total PR Ber-tanggal-butuh</div><div class="stat-v">{{ rep.total_pr }}</div><div class="stat-s">{{ rep.note }}</div></div>
      </div>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead><tr><th>Vendor</th><th class="th-r">Total PR</th><th class="th-r">Diterima</th><th class="th-r">Tepat Waktu</th><th class="th-r">Terlambat</th><th class="th-r">Overdue Belum Datang</th><th class="th-r">OTIF</th><th class="th-r">Lead Time Rata²</th></tr></thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="8" class="p-8 text-center text-sm text-gray-400">
                Belum ada PR ber-tanggal-butuh pada rentang ini — PR hasil MRP otomatis membawanya.
              </td></tr>
              <tr v-for="r in rep.rows" :key="r.vendor_name">
                <td class="text-sm font-medium text-gray-900">{{ r.vendor_name }}</td>
                <td class="td-num">{{ r.total_pr }}</td>
                <td class="td-num">{{ r.received }}</td>
                <td class="td-num text-emerald-700">{{ r.on_time }}</td>
                <td class="td-num" :class="r.late ? 'text-red-600' : ''">{{ r.late }}</td>
                <td class="td-num" :class="r.outstanding_overdue ? 'text-amber-600 font-bold' : ''">{{ r.outstanding_overdue }}</td>
                <td class="td-num font-bold" :class="r.otif_pct >= 90 ? 'text-emerald-700' : 'text-red-600'">{{ r.received ? r.otif_pct.toFixed(0) + '%' : '—' }}</td>
                <td class="td-num">{{ r.avg_lead_days.toFixed(1) }} hari</td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- ══ DISTRIBUSI INDUK → OUTLET ══ -->
    <template v-else-if="tab === 'distribution' && rep">
      <div class="stat-grid">
        <div class="stat"><div class="stat-l">Nilai Dikirim</div><div class="stat-v">{{ fmtRp(rep.total_delivered_value) }}</div><div class="stat-s">dari gudang induk pada rentang</div></div>
        <div class="stat"><div class="stat-l">Dokumen Kiriman</div><div class="stat-v">{{ rep.delivery_docs }}</div><div class="stat-s">transfer diterima outlet</div></div>
        <div class="stat"><div class="stat-l">Item Bergerak</div><div class="stat-v">{{ rep.items_delivered }}</div><div class="stat-s">item dikirim dari induk</div></div>
        <div class="stat"><div class="stat-l">Outlet Tujuan</div><div class="stat-v">{{ rep.outlets.length }}</div><div class="stat-s">gudang outlet aktif dalam scope</div></div>
      </div>
      <p class="text-[11px] text-gray-400 -mt-2">
        Angka = agregat semua outlet dalam filter (pilih outlet di atas untuk fokus satu outlet).
        Klik baris untuk rincian per outlet — bandingkan kiriman &amp; pemakaian dengan hasil hitung fisik saat stock opname.
      </p>
      <AppCard :padding="false">
        <div class="overflow-x-auto">
          <table class="rp-table">
            <thead>
              <tr>
                <th></th>
                <th>Item</th>
                <th class="th-r">Stok Induk</th>
                <th class="th-r">Dikirim</th>
                <th class="th-r">Terjual</th>
                <th class="th-r">Waste</th>
                <th class="th-r">Keluar Lain</th>
                <th class="th-r">Masuk Lain</th>
                <th class="th-r">Stok Outlet</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!rep.rows.length"><td colspan="9" class="p-8 text-center text-sm text-gray-400">Tidak ada kiriman atau stok outlet pada rentang &amp; filter ini.</td></tr>
              <template v-for="r in rep.rows" :key="r.item_id">
                <tr class="cursor-pointer" @click="distOpen[r.item_id] = !distOpen[r.item_id]">
                  <td class="w-8 text-center text-gray-400">{{ distOpen[r.item_id] ? '▾' : '▸' }}</td>
                  <td><div class="font-medium text-gray-900 text-sm">{{ r.item_name }}</div><div class="text-[11px] text-gray-400 font-mono">{{ r.item_code }} · {{ r.base_unit }}</div></td>
                  <td class="td-num text-gray-600">{{ fmtQty(r.central_qty) }}</td>
                  <td class="td-num">
                    <span :class="r.total_delivered > 0 ? 'font-bold text-blue-700' : 'text-gray-300'">{{ fmtQty(r.total_delivered) }}</span>
                    <div v-if="distTot(r).outlets > 0" class="text-[10px] text-gray-400">→ {{ distTot(r).outlets }} outlet</div>
                  </td>
                  <td class="td-num">{{ fmtQty(distTot(r).sale) }}</td>
                  <td class="td-num" :class="distTot(r).waste > 0 ? 'text-red-600 font-semibold' : ''">{{ fmtQty(distTot(r).waste) }}</td>
                  <td class="td-num text-gray-500">{{ fmtQty(distTot(r).otherOut) }}</td>
                  <td class="td-num text-gray-500">{{ fmtQty(distTot(r).otherIn) }}</td>
                  <td class="td-num font-bold">{{ fmtQty(r.total_outlet_qty) }}</td>
                </tr>
                <tr v-if="distOpen[r.item_id]">
                  <td></td>
                  <td colspan="8" class="!p-0">
                    <table class="ing-table">
                      <thead><tr><th>Outlet</th><th class="th-r">Dikirim</th><th class="th-r">Kiriman</th><th>Terakhir</th><th class="th-r">Terjual</th><th class="th-r">Waste</th><th class="th-r">Keluar Lain</th><th class="th-r">Masuk Lain</th><th class="th-r">Stok Kini</th></tr></thead>
                      <tbody>
                        <tr v-for="o in rep.outlets.filter(x => r.cells[x.outlet_id])" :key="o.outlet_id">
                          <td>{{ o.outlet_name }}</td>
                          <td class="td-num font-semibold text-blue-700">{{ fmtQty(r.cells[o.outlet_id].delivered) }}</td>
                          <td class="td-num">{{ r.cells[o.outlet_id].delivery_count }}×</td>
                          <td class="text-xs text-gray-500">{{ fmtDate(r.cells[o.outlet_id].last_delivery) }}</td>
                          <td class="td-num">{{ fmtQty(r.cells[o.outlet_id].sale_out) }}</td>
                          <td class="td-num" :class="r.cells[o.outlet_id].waste_out > 0 ? 'text-red-600 font-semibold' : ''">{{ fmtQty(r.cells[o.outlet_id].waste_out) }}</td>
                          <td class="td-num">{{ fmtQty(r.cells[o.outlet_id].other_out) }}</td>
                          <td class="td-num">{{ fmtQty(r.cells[o.outlet_id].other_in) }}</td>
                          <td class="td-num font-semibold">{{ fmtQty(r.cells[o.outlet_id].current_qty) }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </AppCard>
    </template>

    <!-- Paginasi bersama semua tab -->
    <div v-if="!loading && totalRows > PAGE_SIZE" class="flex items-center justify-between bg-white border border-gray-200 rounded-xl px-4 py-3">
      <span class="text-xs text-gray-400">
        Menampilkan {{ (reportPage - 1) * PAGE_SIZE + 1 }}–{{ Math.min(reportPage * PAGE_SIZE, totalRows) }} dari {{ totalRows }} baris
      </span>
      <AppPagination :page="reportPage" :total-pages="totalPages" @change="changePage" />
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ppicApi } from '@/api/ppic'
import { warehousesApi } from '@/api/warehouse'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import AppButton  from '@/components/ui/AppButton.vue'
import AppCard    from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canExport = computed(() => auth.hasPermission('ppic.reports.export'))

const TABS = [
  { key: 'sold', label: 'Produk Terjual' },
  { key: 'distribution', label: 'Distribusi Induk → Outlet' },
  { key: 'hpp', label: 'HPP Menu' },
  { key: 'variance', label: 'Variance Pemakaian' },
  { key: 'production', label: 'Produksi & Yield' },
  { key: 'forecast', label: 'Akurasi Forecast' },
  { key: 'otif', label: 'Kinerja Vendor (OTIF)' },
]

const tab = ref('sold')
const rep = ref(null)
const loading = ref(false)
const exporting = ref(false)
const warehouseId = ref('')
const outletId = ref('')
const hppSearch = ref('')
const hppOpen = reactive({})
const soldSearch = ref('')
const distSearch = ref('')
const distOpen = reactive({})

// Agregat sel distribusi per baris item (jumlah semua outlet dalam filter).
function distTot(r) {
  const t = { sale: 0, waste: 0, otherOut: 0, otherIn: 0, outlets: 0 }
  for (const c of Object.values(r.cells || {})) {
    t.sale += c.sale_out; t.waste += c.waste_out
    t.otherOut += c.other_out; t.otherIn += c.other_in
    if (c.delivered > 0) t.outlets++
  }
  return t
}
const PAGE_SIZE = 25
const reportPage = ref(1)
const totalRows = computed(() => rep.value?.total_rows ?? 0)
const totalPages = computed(() => Math.max(1, Math.ceil(totalRows.value / PAGE_SIZE)))
const warehouseOptions = ref([{ id: '', name: 'Semua Gudang' }])
const outletOptions = ref([{ id: '', name: 'Semua Outlet' }])

const today = new Date()
const monthAgo = new Date(); monthAgo.setDate(monthAgo.getDate() - 29)
const dateFrom = ref(monthAgo.toISOString().slice(0, 10))
const dateTo = ref(today.toISOString().slice(0, 10))

onMounted(async () => {
  load()
  try {
    const [whRes, oRes] = await Promise.all([warehousesApi.list({ limit: 100 }), apiClient.get('/admin/my-outlets')])
    const whList = Array.isArray(whRes) ? whRes : (whRes?.data || [])
    warehouseOptions.value = [{ id: '', name: 'Semua Gudang' }, ...whList.map(w => ({ id: w.id, name: `${w.name} (${w.code})` }))]
    const oList = Array.isArray(oRes) ? oRes : (oRes?.data || [])
    outletOptions.value = [{ id: '', name: 'Semua Outlet' }, ...oList.map(o => ({ id: o.id, name: o.name }))]
  } catch { /* filter default */ }
})

function params() {
  return {
    date_from: dateFrom.value, date_to: dateTo.value,
    warehouse_id: warehouseId.value, outlet_id: outletId.value,
    search: tab.value === 'hpp' ? hppSearch.value
      : tab.value === 'distribution' ? distSearch.value
      : soldSearch.value,
    page: reportPage.value, limit: PAGE_SIZE,
  }
}

// Pencarian HPP kini di server (ikut terpaginasi) — cukup teruskan baris apa adanya.
const hppRows = computed(() => rep.value?.rows ?? [])

function switchTab(t) {
  tab.value = t
  rep.value = null
  reportPage.value = 1
  load()
}

function applyFilters() {
  reportPage.value = 1
  load()
}

function changePage(p) {
  reportPage.value = p
  load()
}

async function load() {
  loading.value = true
  try {
    rep.value = await ppicApi.getReport(tab.value, params())
  } catch (e) {
    toast.error(e?.message ?? 'Gagal memuat laporan')
  } finally {
    loading.value = false
  }
}

async function doExport() {
  exporting.value = true
  try {
    const blob = await ppicApi.exportReport(tab.value, params())
    const url = URL.createObjectURL(new Blob([blob], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `Laporan-PPIC_${tab.value}_${dateFrom.value}_sd_${dateTo.value}.xlsx`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    toast.error(e?.message ?? 'Gagal export Excel')
  } finally {
    exporting.value = false
  }
}

function fmtQty(v) { return Number(v ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 4 }) }
function fmtRpFull(v) { return 'Rp ' + Math.round(Number(v ?? 0)).toLocaleString('id-ID') }
function fmtRp(v) {
  const n = Number(v ?? 0)
  const abs = Math.abs(n)
  let s
  if (abs >= 1_000_000_000) s = (abs / 1_000_000_000).toFixed(2) + ' M'
  else if (abs >= 1_000_000) s = (abs / 1_000_000).toFixed(1) + ' jt'
  else s = Math.round(abs).toLocaleString('id-ID')
  return (n < 0 ? '-Rp ' : 'Rp ') + s
}
function fmtDate(s) { return s ? new Date(s).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '—' }
</script>

<style scoped>
.tabs { display: flex; gap: .4rem; flex-wrap: wrap; }
.tab { font-size: .76rem; font-weight: 600; padding: .45rem .9rem; border-radius: 999px; border: 1.5px solid #e5e7eb; background: #fff; color: #6b7280; cursor: pointer; transition: all .12s; }
.tab:hover { border-color: #a7f3d0; color: #047857; }
.tab--active { background: #059669; border-color: #059669; color: #fff; }

.warn-note { font-size: .75rem; color: #92400e; background: #fffbeb; border: 1px solid #fde68a; border-radius: .6rem; padding: .6rem .8rem; }

.stat-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: .6rem; }
.stat-grid--2 { grid-template-columns: repeat(2, 1fr); }
@media (max-width: 800px) { .stat-grid { grid-template-columns: repeat(2, 1fr); } }
.stat { padding: .7rem .9rem; border-radius: .75rem; border: 1.5px solid #e5e7eb; background: #fff; }
.stat--ok  { background: #ecfdf5; border-color: #a7f3d0; }
.stat--red { background: #fef2f2; border-color: #fecaca; }
.stat-l { font-size: .62rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.stat-v { font-size: 1.2rem; font-weight: 800; color: #111827; }
.stat-s { font-size: .64rem; color: #9ca3af; }

.rp-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.rp-table thead tr { background: #f9fafb; }
.rp-table th { padding: .5rem .7rem; text-align: left; font-size: .64rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #9ca3af; border-bottom: 1px solid #f3f4f6; white-space: nowrap; }
.rp-table .th-r { text-align: right; }
.rp-table tbody tr { border-bottom: 1px solid #f9fafb; }
.rp-table tbody tr:hover { background: #fafafa; }
.rp-table td { padding: .5rem .7rem; }
.td-num { text-align: right; font-family: monospace; white-space: nowrap; }
.row-over { background: #fff7f7; }

/* Rincian bahan (HPP Menu) */
.ing-table { width: 100%; border-collapse: collapse; font-size: .74rem; background: #fafcff; }
.ing-table th { padding: .3rem .7rem; text-align: left; font-size: .58rem; font-weight: 700; text-transform: uppercase; color: #b6bdc9; border-bottom: 1px solid #eef2f7; }
.ing-table .th-r { text-align: right; }
.ing-table td { padding: .3rem .7rem; border-bottom: 1px solid #f2f6fb; color: #4b5563; }
.wip-tag { font-size: .56rem; font-weight: 800; padding: .08rem .35rem; border-radius: .3rem; background: #ccfbf1; color: #0f766e; margin-right: .3rem; }

/* Produk Terjual */
.cat-chip { font-size: .62rem; font-weight: 700; padding: .1rem .45rem; border-radius: 999px; background: #f3e8ff; color: #7c3aed; white-space: nowrap; }
.outlet-chip { font-size: .62rem; font-weight: 700; padding: .1rem .45rem; border-radius: 999px; background: #d1fae5; color: #065f46; white-space: nowrap; }
.share-wrap { display: flex; align-items: center; gap: .4rem; justify-content: flex-end; }
.share-bar { width: 60px; height: 7px; border-radius: 999px; background: #f1f5f9; overflow: hidden; }
.share-fill { height: 100%; background: #34d399; border-radius: 999px; }
.pill-ok { color: #059669; font-weight: 800; }
.pill-no { font-size: .62rem; font-weight: 700; padding: .1rem .45rem; border-radius: 999px; background: #fee2e2; color: #dc2626; text-decoration: none; }
</style>
