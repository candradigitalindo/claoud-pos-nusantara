<template>
  <div class="space-y-6">

    <!-- Filter Bar -->
    <AppCard>
      <div class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700">Rentang Tanggal</label>
          <DateRangePicker v-model="range" />
        </div>
        <div class="flex flex-col gap-1 min-w-45">
          <label class="text-sm font-medium text-gray-700">Outlet</label>
          <SearchSelect
            v-model="selectedOutlet"
            :options="outletOptions"
            placeholder="Semua Outlet"
            searchPlaceholder="Cari outlet..."
            valueKey="value"
            labelKey="label"
          />
        </div>
        <button @click="page = 1; refetch()"
          class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">
          Tampilkan
        </button>
        <button v-if="canVoid" @click="downloadExcel" :disabled="exporting"
          class="inline-flex items-center gap-2 px-4 py-2 bg-white border border-emerald-600 text-emerald-700 text-sm font-medium rounded-lg hover:bg-emerald-50 transition-colors shadow-sm disabled:opacity-60 disabled:cursor-not-allowed">
          <svg v-if="!exporting" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
          </svg>
          <AppSpinner v-else size="sm" />
          {{ exporting ? 'Menyiapkan…' : 'Download Excel' }}
        </button>
      </div>
    </AppCard>

    <AppAlert type="error" :message="errorMsg" />

    <div v-if="loading" class="flex justify-center py-12">
      <AppSpinner size="lg" />
    </div>

    <template v-if="!loading && (report || titipan)">

      <!-- Summary Cards (khusus data void) -->
      <div v-if="canVoid && report" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-red-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Void Transaksi</p>
              <p class="text-2xl font-bold text-gray-900">{{ report.summary.total_voided }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-orange-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Nilai Void Transaksi</p>
              <p class="text-2xl font-bold text-gray-900">{{ formatRupiah(report.summary.total_amount) }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-rose-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-rose-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Void Item</p>
              <p class="text-2xl font-bold text-gray-900">{{ report.summary.item_voided }}</p>
            </div>
          </div>
        </AppCard>
        <AppCard>
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-xl bg-amber-100 flex items-center justify-center">
              <svg class="w-5 h-5 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
            </div>
            <div>
              <p class="text-xs text-gray-500 font-medium uppercase tracking-wide">Nilai Void Item</p>
              <p class="text-2xl font-bold text-gray-900">{{ formatRupiah(report.summary.item_amount) }}</p>
            </div>
          </div>
        </AppCard>
      </div>

      <!-- Tab: Void Transaksi vs Void Item -->
      <div class="flex gap-2">
        <button v-if="canVoid"
          class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
          :class="activeTab === 'order' ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
          @click="switchTab('order')">
          Void Transaksi ({{ report?.total ?? 0 }})
        </button>
        <button v-if="canVoid"
          class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
          :class="activeTab === 'item' ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
          @click="switchTab('item')">
          Void Item ({{ report?.items_total ?? 0 }})
        </button>
        <button v-if="canTitipan"
          class="px-4 py-2 text-sm font-medium rounded-lg transition-colors"
          :class="activeTab === 'titipan' ? 'bg-emerald-600 text-white shadow-sm' : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'"
          @click="switchTab('titipan')">
          Titipan ({{ titipan?.total ?? 0 }})
        </button>
      </div>

      <!-- Table: Void Transaksi -->
      <AppCard v-if="activeTab === 'order' && report">
        <div v-if="report.data.length === 0" class="text-center py-12 text-gray-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>
          <p class="font-medium">Tidak ada order void pada periode ini</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50/70">
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Waktu Void</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Outlet</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Order</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Item</th>
                <th class="text-right py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Total</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Petugas & Alasan</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="row in report.data" :key="row.id" class="hover:bg-red-50/40 transition-colors">
                <td class="py-3 px-3 whitespace-nowrap align-top">
                  <p class="font-semibold text-gray-900 tabular-nums">{{ fmtTime(row.voided_at) }}</p>
                  <p class="text-xs text-gray-400">{{ fmtDateShort(row.voided_at) }}</p>
                  <p v-if="lifeSpan(row.created_at, row.voided_at)" class="text-[11px] text-amber-600 mt-0.5" title="Selang waktu order dibuat sampai di-void">⏱ {{ lifeSpan(row.created_at, row.voided_at) }}</p>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="vr-pill"><span class="vr-dot" />{{ row.outlet_name || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <p class="font-medium text-gray-800">{{ row.customer_name || 'Tanpa nama' }}</p>
                  <span v-if="row.table_number" class="vr-meja">Meja {{ row.table_number }}</span>
                </td>
                <td class="py-3 px-3 align-top max-w-[16rem]">
                  <div class="flex flex-wrap gap-1">
                    <span v-for="item in parseItems(row.items).slice(0, 3)" :key="item.product_name"
                      class="vr-chip">{{ item.qty }}× {{ item.product_name }}</span>
                    <span v-if="parseItems(row.items).length > 3" class="vr-chip vr-chip-more"
                      :title="parseItems(row.items).map(i => `${i.qty}× ${i.product_name}`).join('\n')">
                      +{{ parseItems(row.items).length - 3 }} lagi
                    </span>
                  </div>
                  <p class="text-[11px] text-gray-400 mt-1">{{ itemsSummary(row.items) }}</p>
                </td>
                <td class="py-3 px-3 text-right align-top">
                  <span class="font-bold text-red-600 tabular-nums">{{ formatRupiah(row.total_amount) }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <div class="flex items-start gap-2">
                    <span class="vr-avatar">{{ initials(row.voided_by) }}</span>
                    <div class="min-w-0">
                      <p class="font-medium text-gray-800 leading-tight">{{ row.voided_by || '—' }}</p>
                      <p class="text-xs text-gray-500 italic break-words">"{{ row.void_reason || 'Dibatalkan kasir' }}"</p>
                    </div>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="report.total > report.limit" class="flex items-center justify-between pt-4 border-t border-gray-100 mt-4">
          <p class="text-sm text-gray-500">
            Menampilkan {{ (page-1)*report.limit + 1 }}–{{ Math.min(page*report.limit, report.total) }} dari {{ report.total }} data
          </p>
          <div class="flex gap-2">
            <button :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page--; fetchReport()">← Prev</button>
            <button :disabled="page * report.limit >= report.total"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page++; fetchReport()">Next →</button>
          </div>
        </div>
      </AppCard>

      <!-- Table: Void Item (hapus item dari order belum bayar) -->
      <AppCard v-if="activeTab === 'item' && report">
        <div v-if="report.items.length === 0" class="text-center py-12 text-gray-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
          <p class="font-medium">Tidak ada void item pada periode ini</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50/70">
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Waktu Void</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Outlet</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Item</th>
                <th class="text-right py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Subtotal</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Order</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Petugas & Alasan</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="row in report.items" :key="row.id" class="hover:bg-red-50/40 transition-colors">
                <td class="py-3 px-3 whitespace-nowrap align-top">
                  <p class="font-semibold text-gray-900 tabular-nums">{{ fmtTime(row.voided_at) }}</p>
                  <p class="text-xs text-gray-400">{{ fmtDateShort(row.voided_at) }}</p>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="vr-pill"><span class="vr-dot" />{{ row.outlet_name || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <p class="font-medium text-gray-800">{{ row.product_name }}</p>
                  <p class="text-xs text-gray-400 tabular-nums">{{ Number(row.qty) }} × {{ formatRupiah(row.price) }}</p>
                </td>
                <td class="py-3 px-3 text-right align-top">
                  <span class="font-bold text-red-600 tabular-nums">{{ formatRupiah(row.subtotal) }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <span v-if="row.table_number" class="vr-meja">Meja {{ row.table_number }}</span>
                  <p v-if="row.waiter_name" class="text-xs text-gray-500 mt-0.5">Pemesan: <span class="font-medium text-gray-700">{{ row.waiter_name }}</span></p>
                  <span v-if="!row.table_number && !row.waiter_name" class="text-gray-400">—</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <div class="flex items-start gap-2">
                    <span class="vr-avatar">{{ initials(row.voided_by) }}</span>
                    <div class="min-w-0">
                      <p class="font-medium text-gray-800 leading-tight">{{ row.voided_by || '—' }}</p>
                      <p class="text-xs text-gray-500 italic break-words">"{{ row.void_reason || 'Dibatalkan kasir' }}"</p>
                    </div>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="report.items_total > report.limit" class="flex items-center justify-between pt-4 border-t border-gray-100 mt-4">
          <p class="text-sm text-gray-500">
            Menampilkan {{ (page-1)*report.limit + 1 }}–{{ Math.min(page*report.limit, report.items_total) }} dari {{ report.items_total }} data
          </p>
          <div class="flex gap-2">
            <button :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page--; fetchReport()">← Prev</button>
            <button :disabled="page * report.limit >= report.items_total"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page++; fetchReport()">Next →</button>
          </div>
        </div>
      </AppCard>

      <!-- Table: Meja Titipan (parked check — titip/tarik) -->
      <AppCard v-if="activeTab === 'titipan'">
        <!-- Ringkasan titipan -->
        <div v-if="titipan" class="grid grid-cols-2 lg:grid-cols-3 gap-3 mb-4">
          <div class="tp-stat tp-park"><span class="tp-l">Dititip (park)</span><span class="tp-v">{{ titipan.summary.park_count }}</span><span class="tp-s">{{ formatRupiah(titipan.summary.park_amount) }}</span></div>
          <div class="tp-stat tp-pull"><span class="tp-l">Ditarik (pull)</span><span class="tp-v">{{ titipan.summary.pull_count }}</span><span class="tp-s">{{ formatRupiah(titipan.summary.pull_amount) }}</span></div>
          <div class="tp-stat tp-hold"><span class="tp-l">Masih Menggantung</span><span class="tp-v">{{ titipan.summary.holding_count }}</span><span class="tp-s">{{ formatRupiah(titipan.summary.holding_value) }}</span></div>
        </div>

        <div v-if="!titipan || titipan.data.length === 0" class="text-center py-12 text-gray-400">
          <svg class="w-12 h-12 mx-auto mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10"/></svg>
          <p class="font-medium">Tidak ada aktivitas titipan pada periode ini</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 bg-gray-50/70">
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Waktu</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Outlet</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Aksi</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Item</th>
                <th class="text-right py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Nilai</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Perpindahan</th>
                <th class="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">Oleh</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="row in titipan.data" :key="row.id" class="hover:bg-emerald-50/30 transition-colors">
                <td class="py-3 px-3 whitespace-nowrap align-top">
                  <p class="font-semibold text-gray-900 tabular-nums">{{ (row.performed_at || '—').slice(11) }}</p>
                  <p class="text-xs text-gray-400">{{ (row.performed_at || '').slice(0,10) }}</p>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="vr-pill"><span class="vr-dot" />{{ row.outlet_name || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <span class="tp-badge" :class="row.action === 'park' ? 'tp-b-park' : 'tp-b-pull'">
                    {{ row.action === 'park' ? '↓ Titip' : '↑ Tarik' }}
                  </span>
                </td>
                <td class="py-3 px-3 align-top">
                  <p class="font-medium text-gray-800">{{ row.product_name }}</p>
                  <p class="text-xs text-gray-400 tabular-nums">{{ Number(row.qty) }} × {{ formatRupiah(row.price) }}</p>
                </td>
                <td class="py-3 px-3 text-right align-top">
                  <span class="font-bold text-gray-900 tabular-nums">{{ formatRupiah(row.subtotal) }}</span>
                </td>
                <td class="py-3 px-3 align-top text-xs text-gray-500 whitespace-nowrap">
                  <span class="vr-meja">{{ row.source_table || '—' }}</span> → <span class="vr-meja">{{ row.target_table || '—' }}</span>
                </td>
                <td class="py-3 px-3 align-top">
                  <div class="flex items-start gap-2">
                    <span class="vr-avatar">{{ initials(row.performed_by) }}</span>
                    <p class="font-medium text-gray-800 leading-tight">{{ row.performed_by || '—' }}</p>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div v-if="titipan && titipan.total > titipan.limit" class="flex items-center justify-between pt-4 border-t border-gray-100 mt-4">
          <p class="text-sm text-gray-500">
            Menampilkan {{ (page-1)*titipan.limit + 1 }}–{{ Math.min(page*titipan.limit, titipan.total) }} dari {{ titipan.total }} data
          </p>
          <div class="flex gap-2">
            <button :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page--; fetchTitipan()">← Prev</button>
            <button :disabled="page * titipan.limit >= titipan.total"
              class="px-3 py-1.5 text-sm border rounded-lg disabled:opacity-40 hover:bg-gray-50"
              @click="page++; fetchTitipan()">Next →</button>
          </div>
        </div>
      </AppCard>
    </template>

  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { apiClient } from '@/api/client.js'
import { outletsApi } from '@/api/outlets.js'
import { useAuthStore } from '@/stores/auth.js'
import { useRealtime } from '@/utils/realtime.js'
import AppCard    from '@/components/ui/AppCard.vue'
import AppAlert   from '@/components/ui/AppAlert.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const dateFrom      = ref(new Date(new Date().setDate(1)).toISOString().slice(0,10))
const dateTo        = ref(new Date().toISOString().slice(0,10))
const range          = ref({ from: dateFrom.value, to: dateTo.value, label: 'Bulan Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; page.value = 1; refetch() })
function refetch() { activeTab.value === 'titipan' ? fetchTitipan() : fetchReport() }
const selectedOutlet = ref('')
const outletOptions  = ref([{ value: '', label: 'Semua Outlet' }])
const report        = ref(null)
const titipan       = ref(null)
const loading       = ref(false)
const errorMsg      = ref('')
const page          = ref(1)

// Izin per tab: Void (transaksi & item) dan Titipan dipisah — role bisa punya
// salah satu saja. Halaman bisa dibuka bila punya minimal satu izin (router).
const auth = useAuthStore()
const canVoid    = auth.hasPermission('reports.void.view')
const canTitipan = auth.hasPermission('reports.titipan.view')

const activeTab     = ref(canVoid ? 'order' : 'titipan') // 'order' = void transaksi, 'item' = void item, 'titipan' = meja titipan

function switchTab(tab) {
  if (activeTab.value === tab) return
  activeTab.value = tab
  page.value = 1
  if (tab === 'titipan') fetchTitipan()
  else fetchReport()
}

onMounted(async () => {
  try {
    // Use the scoped, permission-free endpoint so the dropdown only ever lists
    // outlets this role may access (consistent with the other report pages).
    const res = await outletsApi.myOutlets()
    const list = res.data ?? res ?? []
    outletOptions.value = [
      { value: '', label: 'Semua Outlet' },
      ...list.map(o => ({ value: o.id, label: o.name })),
    ]
  } catch {}
  canVoid ? fetchReport() : fetchTitipan()
})

async function fetchReport({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const params = new URLSearchParams({
      date_from: dateFrom.value,
      date_to: dateTo.value,
      page: page.value,
      limit: 50,
    })
    if (selectedOutlet.value) params.set('outlet_id', selectedOutlet.value)
    report.value = await apiClient.get(`/admin/void-report?${params}`)
  } catch (err) {
    if (!silent) errorMsg.value = err?.message ?? 'Gagal memuat laporan void'
  } finally {
    if (!silent) loading.value = false
  }
}

async function fetchTitipan({ silent = false } = {}) {
  if (!silent) { loading.value = true; errorMsg.value = '' }
  try {
    const params = new URLSearchParams({
      date_from: dateFrom.value,
      date_to: dateTo.value,
      page: page.value,
      limit: 50,
    })
    if (selectedOutlet.value) params.set('outlet_id', selectedOutlet.value)
    titipan.value = await apiClient.get(`/admin/titipan-report?${params}`)
  } catch (err) {
    if (!silent) errorMsg.value = err?.message ?? 'Gagal memuat laporan titipan'
  } finally {
    if (!silent) loading.value = false
  }
}

useRealtime(['order_item_void', 'order_item_titipan'], () => {
  if (activeTab.value === 'titipan') fetchTitipan({ silent: true })
  else fetchReport({ silent: true })
})

function parseItems(raw) {
  try { return JSON.parse(raw) ?? [] } catch { return [] }
}

// Download Excel void transaksi + void item, mengikuti filter aktif (tanggal +
// outlet). Titipan tidak ikut (permission terpisah). Scope role tetap
// dipaksakan di server.
const exporting = ref(false)
async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const params = { date_from: dateFrom.value, date_to: dateTo.value }
    if (selectedOutlet.value) params.outlet_id = selectedOutlet.value
    const blob = await apiClient.get('/admin/void-report/export', {
      params,
      responseType: 'blob',
      timeout: 120000,
    })

    const outletLabel = selectedOutlet.value
      ? (outletOptions.value.find(o => o.value === selectedOutlet.value)?.label || 'Outlet')
      : 'Semua-Outlet'
    const slug = outletLabel.replace(/[^A-Za-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'Outlet'
    const filename = `Laporan-Void_${slug}_${dateFrom.value}_sd_${dateTo.value}.xlsx`

    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    errorMsg.value = 'Gagal mengunduh Excel. Coba lagi, atau persempit rentang tanggal.'
  } finally {
    exporting.value = false
  }
}

function formatRupiah(val) {
  return 'Rp ' + Number(val ?? 0).toLocaleString('id-ID')
}

function fmtDateTime(str) {
  if (!str) return '—'
  return new Date(str).toLocaleString('id-ID', {
    day: '2-digit', month: 'short', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function fmtTime(str) {
  if (!str) return '—'
  const d = new Date(str)
  return isNaN(d) ? '—' : d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}

function fmtDateShort(str) {
  if (!str) return ''
  const d = new Date(str)
  return isNaN(d) ? '' : d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}

// Selang order dibuat → di-void; void yang terlalu cepat/lama bisa jadi sinyal
// pola yang perlu dicek (mis. void rutin sesaat setelah input).
function lifeSpan(createdStr, voidedStr) {
  if (!createdStr || !voidedStr) return ''
  const a = new Date(createdStr), b = new Date(voidedStr)
  if (isNaN(a) || isNaN(b)) return ''
  let mins = Math.round((b - a) / 60000)
  if (mins < 0 || mins > 60 * 24 * 30) return ''
  if (mins < 1) return 'di-void < 1 mnt'
  if (mins < 60) return `di-void setelah ${mins} mnt`
  const h = Math.floor(mins / 60)
  return `di-void setelah ${h} jam ${mins % 60} mnt`
}

function itemsSummary(raw) {
  const items = parseItems(raw)
  if (!items.length) return ''
  const qty = items.reduce((s, i) => s + (Number(i.qty) || 0), 0)
  return `${items.length} item · ${qty} qty`
}

function initials(name) {
  if (!name) return '?'
  return name.trim().split(/\s+/).slice(0, 2).map(w => w[0]).join('').toUpperCase()
}
</script>

<style scoped>
.vr-pill { display: inline-flex; align-items: center; gap: .4rem; padding: .2rem .6rem; border-radius: 999px; background: rgba(16,185,129,.08); border: 1px solid rgba(16,185,129,.2); font-size: .75rem; font-weight: 600; color: #065f46; white-space: nowrap; }
.vr-dot { width: 6px; height: 6px; border-radius: 50%; background: #10b981; flex-shrink: 0; }
.vr-meja { display: inline-block; padding: .1rem .5rem; border-radius: .4rem; background: rgba(59,130,246,.09); color: #1d4ed8; font-size: .7rem; font-weight: 700; }
.vr-chip { display: inline-flex; align-items: center; padding: .12rem .5rem; border-radius: .45rem; background: #f3f4f6; color: #374151; font-size: .72rem; white-space: nowrap; max-width: 11rem; overflow: hidden; text-overflow: ellipsis; }
.vr-chip-more { background: rgba(239,68,68,.08); color: #b91c1c; font-weight: 700; cursor: help; }
.vr-avatar { display: inline-flex; align-items: center; justify-content: center; width: 1.85rem; height: 1.85rem; border-radius: 50%; background: rgba(239,68,68,.1); color: #b91c1c; font-size: .68rem; font-weight: 800; flex-shrink: 0; }

/* Titipan */
.tp-stat { display: flex; flex-direction: column; gap: .1rem; padding: .75rem .9rem; border-radius: .9rem; border: 1px solid; }
.tp-l { font-size: .68rem; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; }
.tp-v { font-size: 1.5rem; font-weight: 800; line-height: 1.1; }
.tp-s { font-size: .72rem; opacity: .8; }
.tp-park { background: rgba(59,130,246,.06); border-color: rgba(59,130,246,.25); color: #1d4ed8; }
.tp-pull { background: rgba(16,185,129,.06); border-color: rgba(16,185,129,.25); color: #047857; }
.tp-hold { background: rgba(245,158,11,.07); border-color: rgba(245,158,11,.3); color: #b45309; }
.tp-badge { display: inline-block; padding: .15rem .55rem; border-radius: 999px; font-size: .7rem; font-weight: 700; white-space: nowrap; }
.tp-b-park { background: rgba(59,130,246,.12); color: #1d4ed8; }
.tp-b-pull { background: rgba(16,185,129,.14); color: #047857; }
</style>
