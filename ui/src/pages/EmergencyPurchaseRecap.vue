<!--
  EmergencyPurchaseRecap.vue — audit Keuangan atas Pembelian Darurat (barang/
  jasa urgent ≤ batas yang dibeli dulu dari kas laci outlet). Per bulan
  (tanggal dicatat, zona aplikasi) per outlet: tahap pengesahan, selisih kas −
  struk, dan sebagai pembanding Kas Keluar tablet di luar tugas Bayar
  Pengadaan. Rincian tiap transaksi bisa dibuka (bukti struk & pengesahan,
  baca saja).
-->
<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="min-w-0">
        <h1 class="text-lg font-bold text-gray-900 sm:text-xl">Rekap Pembelian Darurat</h1>
        <p class="mt-0.5 text-xs text-gray-500">
          Pembelian ≤ {{ formatRupiah(recap?.settings?.max_amount || 500000) }} yang dibeli dulu dari kas laci outlet, lalu disahkan penyetuju.
          Dikelompokkan per bulan menurut tanggal dicatat.
        </p>
      </div>
      <AppButton variant="secondary" :loading="exporting" :disabled="loading" @click="downloadExcel">Download Excel</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard>
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-4">
        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium text-gray-700" for="recap-month">Bulan</label>
          <input id="recap-month" v-model="month" type="month" :max="currentMonth"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/30 sm:w-48" />
        </div>
        <div class="flex w-full flex-col gap-1 sm:w-auto sm:min-w-[220px]">
          <label class="text-sm font-medium text-gray-700">Outlet</label>
          <SearchSelect v-model="outletId" :options="outletOptions" value-key="value" label-key="label"
            placeholder="Semua outlet" search-placeholder="Cari outlet…" />
        </div>
        <button type="button" class="w-full rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-emerald-700 sm:w-auto"
          @click="load">Tampilkan</button>
      </div>
    </AppCard>

    <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
    <template v-else-if="recap">
      <!-- Ringkasan -->
      <div class="grid grid-cols-2 gap-2.5 sm:gap-3 lg:grid-cols-5">
        <div v-for="tile in tiles" :key="tile.key" :class="['min-w-0 overflow-hidden rounded-xl border bg-white p-3 shadow-sm', tile.key === 'diff' ? 'col-span-2 lg:col-span-1' : '', tile.cls]">
          <p class="text-[11px] font-semibold uppercase tracking-wide text-gray-500 break-words">{{ tile.label }}</p>
          <p class="mt-1 text-sm font-bold text-gray-900 sm:text-lg break-words leading-snug">{{ formatRupiah(tile.amount) }}</p>
          <p class="text-[11px] text-gray-500 break-words">{{ tile.note }}</p>
        </div>
      </div>

      <!-- Per outlet -->
      <AppCard :padding="false">
        <div class="border-b border-gray-100 px-4 py-3">
          <p class="text-sm font-semibold text-gray-800">Per Outlet — {{ recap.month_label }}</p>
          <p class="text-[11px] text-gray-500">
            Kas keluar lain = Kas Keluar tablet yang bukan tugas Bayar Pengadaan (gaji harian, BBM, refund, belanja di luar sistem) — periksa di Operasional Keuangan.
          </p>
        </div>
        <div v-if="!recap.outlets.length" class="p-6 text-center text-sm text-gray-400">Tidak ada data pada periode ini.</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-[960px] w-full text-xs">
            <thead class="bg-gray-50 text-[11px] uppercase tracking-wide text-gray-500">
              <tr>
                <th class="px-3 py-2 text-left">Outlet</th>
                <th class="px-3 py-2 text-right">Dicatat</th>
                <th class="px-3 py-2 text-right">Menunggu kas</th>
                <th class="px-3 py-2 text-right">Menunggu struk</th>
                <th class="px-3 py-2 text-right">Menunggu pengesahan</th>
                <th class="px-3 py-2 text-right">Disahkan</th>
                <th class="px-3 py-2 text-right">Ditolak (temuan)</th>
                <th class="px-3 py-2 text-right">Batal</th>
                <th class="px-3 py-2 text-right">Selisih kas − struk</th>
                <th class="px-3 py-2 text-right">Kas keluar lain</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="o in [...recap.outlets, recap.totals]" :key="o.outlet_id || 'total'"
                :class="o === recap.totals ? 'bg-gray-50 font-semibold' : ''">
                <td class="px-3 py-2 text-gray-900">{{ o.outlet_name }}</td>
                <td v-for="b in ['recorded', 'waiting_cash', 'waiting_proof', 'waiting_ratification', 'ratified', 'rejected']" :key="b"
                  :class="['px-3 py-2 text-right', b === 'rejected' && o[b].count ? 'text-red-700' : 'text-gray-800']">
                  <template v-if="o[b].count">{{ formatRupiah(o[b].amount) }}<span class="block text-[10px] font-normal text-gray-400">{{ o[b].count }}×</span></template>
                  <span v-else class="text-gray-300">—</span>
                </td>
                <td class="px-3 py-2 text-right text-gray-600">{{ o.cancelled.count || '—' }}</td>
                <td :class="['px-3 py-2 text-right', o.proof_difference > 0 ? 'text-amber-700' : 'text-gray-800']">
                  {{ o.proof_difference ? formatRupiah(o.proof_difference) : '—' }}
                </td>
                <td class="px-3 py-2 text-right text-gray-700">
                  <template v-if="o.other_cash_out.count">{{ formatRupiah(o.other_cash_out.amount) }}<span class="block text-[10px] font-normal text-gray-400">{{ o.other_cash_out.count }}×</span></template>
                  <span v-else class="text-gray-300">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </AppCard>

      <!-- Rincian -->
      <AppCard :padding="false">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
          <p class="text-sm font-semibold text-gray-800">Rincian ({{ filteredRows.length }})</p>
          <div class="flex flex-wrap gap-1" role="group" aria-label="Saring tahap">
            <button v-for="f in STAGE_FILTERS" :key="f.value" type="button"
              :class="['rounded-full px-2.5 py-1 text-[11px] font-semibold', stageFilter === f.value ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200']"
              @click="stageFilter = f.value">{{ f.label }}</button>
          </div>
        </div>
        <div v-if="!filteredRows.length" class="p-6 text-center text-sm text-gray-400">Tidak ada Pembelian Darurat untuk saringan ini.</div>
        <template v-else>
          <!-- Ponsel: kartu -->
          <ul class="divide-y divide-gray-100 sm:hidden">
            <li v-for="r in filteredRows" :key="r.id" class="p-4">
              <div class="flex items-start justify-between gap-2">
                <button type="button" class="min-w-0 flex-1 text-left" @click="openDetail(r)">
                  <p class="font-mono text-[11px] text-gray-500">{{ r.request_number }} · {{ formatDateTime(r.created_at) }}</p>
                  <p class="mt-0.5 break-words text-sm font-medium text-gray-900">{{ r.items || '-' }}</p>
                  <p class="text-xs text-gray-500">{{ r.outlet_name }} · {{ r.category_label }}</p>
                </button>
                <span :class="['shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold', STAGE[r.stage]?.cls]">{{ STAGE[r.stage]?.label || r.stage }}</span>
              </div>
              <div class="mt-2 flex flex-wrap items-end justify-between gap-2 text-xs">
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-semibold text-gray-900 break-words">{{ formatRupiah(r.amount) }}</p>
                  <p class="text-gray-500 break-words">ke {{ r.recipient || '-' }}<template v-if="r.cashier"> · kasir {{ r.cashier }}</template></p>
                  <p v-if="r.difference" :class="r.difference > 0 ? 'text-amber-700' : 'text-gray-600'">Selisih struk {{ formatRupiah(r.difference) }}</p>
                </div>
                <AppActionButton icon="view" label="Lihat" @click="openDetail(r)" />
              </div>
            </li>
          </ul>
          <!-- Desktop: tabel -->
          <div class="hidden overflow-x-auto sm:block">
            <table class="min-w-[1000px] w-full text-xs">
              <thead class="bg-gray-50 text-[11px] uppercase tracking-wide text-gray-500">
                <tr>
                  <th class="px-3 py-2 text-left">Dicatat</th>
                  <th class="px-3 py-2 text-left">Outlet</th>
                  <th class="px-3 py-2 text-left">Nomor</th>
                  <th class="px-3 py-2 text-left">Barang/Jasa · Alasan</th>
                  <th class="px-3 py-2 text-right">Nilai kas</th>
                  <th class="px-3 py-2 text-left">Penerima · Kasir (izin)</th>
                  <th class="px-3 py-2 text-right">Struk · Selisih</th>
                  <th class="px-3 py-2 text-left">Tahap</th>
                  <th class="px-3 py-2"></th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100">
                <tr v-for="r in filteredRows" :key="r.id" class="align-top">
                  <td class="whitespace-nowrap px-3 py-2 text-gray-600">{{ formatDateTime(r.created_at) }}</td>
                  <td class="px-3 py-2 text-gray-800">{{ r.outlet_name }}</td>
                  <td class="px-3 py-2 font-mono text-gray-700">{{ r.request_number }}</td>
                  <td class="max-w-[280px] px-3 py-2">
                    <p class="break-words text-gray-900">{{ r.items || '-' }}</p>
                    <p class="break-words text-gray-500">{{ r.category_label }}: {{ r.reason }}</p>
                  </td>
                  <td class="px-3 py-2 text-right font-semibold text-gray-900">{{ formatRupiah(r.amount) }}</td>
                  <td class="px-3 py-2 text-gray-700">
                    {{ r.recipient || '-' }}
                    <p v-if="r.cashier" class="text-gray-500">{{ r.cashier }} <span class="text-gray-400">({{ r.authorized_by || '-' }})</span></p>
                  </td>
                  <td class="px-3 py-2 text-right">
                    <template v-if="r.proof_total != null">
                      {{ formatRupiah(r.proof_total) }}
                      <p v-if="r.difference" :class="r.difference > 0 ? 'text-amber-700' : 'text-gray-600'">{{ r.difference > 0 ? '+' : '' }}{{ formatRupiah(r.difference) }}</p>
                    </template>
                    <span v-else class="text-gray-300">—</span>
                  </td>
                  <td class="px-3 py-2">
                    <span :class="['whitespace-nowrap rounded-full px-2 py-0.5 text-[10px] font-semibold', STAGE[r.stage]?.cls]">{{ STAGE[r.stage]?.label || r.stage }}</span>
                    <p v-if="r.ratified_by" class="mt-0.5 text-[10px] text-gray-500">{{ r.ratified_by }}</p>
                  </td>
                  <td class="px-3 py-2 text-right">
                    <AppActionButton icon="view" label="Lihat" @click="openDetail(r)" />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </AppCard>
    </template>

    <AppModal v-model="showDetail" :title="detail ? `Pembelian Darurat ${detail.request_number}` : 'Pembelian Darurat'" size="xl">
      <div v-if="detail" class="space-y-3">
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
          <dt class="text-gray-400">Outlet</dt><dd class="text-gray-900 break-words">{{ detail.outlet_name || '-' }} · {{ detail.work_unit_name || '-' }}</dd>
          <dt class="text-gray-400">Dicatat</dt><dd class="text-gray-700 break-words">{{ detail.requested_by }} · {{ formatDateTime(detail.created_at) }}</dd>
          <dt class="text-gray-400">Barang/Jasa</dt>
          <dd class="text-gray-700 break-words">
            <template v-for="g in detail.items || []" :key="g.name">
              <span v-for="(it, i) in g.items || []" :key="i" class="block">{{ it.name }} ×{{ it.qty }} {{ it.unit }} · {{ formatRupiah(it.final_price) }}</span>
            </template>
          </dd>
          <dt class="text-gray-400">Status</dt><dd class="text-gray-700 break-words">{{ purchaseStatusLabel(detail.status) }}</dd>
        </dl>
        <EmergencyPurchasePanel :doc="detail" :can-submit-side="false" :can-approve="false" />
      </div>
      <p v-else class="text-sm text-gray-400">Memuat…</p>
      <template #footer>
        <AppButton variant="secondary" @click="showDetail = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { outletsApi } from '@/api/outlets.js'
import { formatRupiah, formatDateTime, todayDateString } from '@/utils/format.js'
import { purchaseStatusLabel } from '@/utils/procurement.js'
import { useRealtime } from '@/utils/realtime.js'
import { useToastStore } from '@/stores/toast.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import EmergencyPurchasePanel from '@/components/EmergencyPurchasePanel.vue'

const toast = useToastStore()
const currentMonth = todayDateString().slice(0, 7)
const month = ref(currentMonth)
const outletId = ref('')
const outletOptions = ref([{ value: '', label: 'Semua outlet' }])
const recap = ref(null)
const loading = ref(false)
const errorMsg = ref('')
const exporting = ref(false)
const stageFilter = ref('')
const showDetail = ref(false)
const detail = ref(null)

// Tahap Pembelian Darurat (cermin services.EmergencyStage*).
const STAGE = {
  waiting_cash:         { label: 'Menunggu kas', cls: 'bg-amber-100 text-amber-800' },
  waiting_proof:        { label: 'Menunggu struk', cls: 'bg-amber-100 text-amber-800' },
  waiting_ratification: { label: 'Menunggu pengesahan', cls: 'bg-indigo-100 text-indigo-700' },
  ratified:             { label: 'Disahkan', cls: 'bg-emerald-100 text-emerald-700' },
  rejected:             { label: 'Ditolak (temuan)', cls: 'bg-red-100 text-red-700' },
  cancelled:            { label: 'Dibatalkan', cls: 'bg-gray-100 text-gray-600' },
}
const STAGE_FILTERS = [
  { value: '', label: 'Semua' },
  { value: 'open', label: 'Perlu tindakan' },
  { value: 'rejected', label: 'Ditolak (temuan)' },
  { value: 'ratified', label: 'Disahkan' },
  { value: 'cancelled', label: 'Dibatalkan' },
]
const OPEN_STAGES = ['waiting_cash', 'waiting_proof', 'waiting_ratification']

const filteredRows = computed(() => {
  const rows = recap.value?.rows || []
  if (!stageFilter.value) return rows
  if (stageFilter.value === 'open') return rows.filter(r => OPEN_STAGES.includes(r.stage))
  return rows.filter(r => r.stage === stageFilter.value)
})

const tiles = computed(() => {
  const t = recap.value?.totals
  if (!t) return []
  const waiting = ['waiting_cash', 'waiting_proof', 'waiting_ratification']
  const waitingAmount = waiting.reduce((s, k) => s + (t[k]?.amount || 0), 0)
  const waitingCount = waiting.reduce((s, k) => s + (t[k]?.count || 0), 0)
  return [
    { key: 'recorded', label: 'Dicatat', amount: t.recorded.amount, note: `${t.recorded.count} pembelian (tanpa yang batal)`, cls: 'border-gray-200' },
    { key: 'ratified', label: 'Disahkan', amount: t.ratified.amount, note: `${t.ratified.count} pembelian`, cls: 'border-emerald-200' },
    { key: 'waiting', label: 'Menunggu', amount: waitingAmount, note: `${waitingCount} · kas ${t.waiting_cash.count}, struk ${t.waiting_proof.count}, pengesahan ${t.waiting_ratification.count}`, cls: 'border-amber-200' },
    { key: 'rejected', label: 'Ditolak — temuan', amount: t.rejected.amount, note: `${t.rejected.count} pembelian, perlu ditindaklanjuti`, cls: t.rejected.count ? 'border-red-300 bg-red-50' : 'border-gray-200' },
    { key: 'diff', label: 'Selisih kas − struk', amount: t.proof_difference, note: t.proof_difference > 0 ? 'Harus kembali ke laci (Kas Masuk)' : 'Tidak ada sisa kas', cls: t.proof_difference > 0 ? 'border-amber-300 bg-amber-50' : 'border-gray-200' },
  ]
})

function params() {
  const p = { month: month.value || currentMonth }
  if (outletId.value) p.outlet_id = outletId.value
  return p
}

async function load({ silent = false } = {}) {
  if (!silent) {
    loading.value = true
    errorMsg.value = ''
  }
  try {
    const res = await purchaseApi.emergencyRecap(params())
    // apiClient membuka amplop {success,data} respons non-paginated.
    recap.value = Array.isArray(res?.rows) ? res : (res?.data ?? null)
  } catch (err) {
    if (!silent) {
      recap.value = null
      errorMsg.value = err?.message || 'Gagal memuat rekap.'
    }
  } finally {
    if (!silent) loading.value = false
  }
}

async function downloadExcel() {
  if (exporting.value) return
  exporting.value = true
  errorMsg.value = ''
  try {
    const blob = await purchaseApi.exportEmergencyRecap(params())
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `Rekap-Pembelian-Darurat_${month.value || currentMonth}.xlsx`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    errorMsg.value = err?.message || 'Gagal mengunduh Excel.'
  } finally {
    exporting.value = false
  }
}

let detailSeq = 0
async function openDetail(row) {
  const seq = ++detailSeq
  detail.value = null
  showDetail.value = true
  try {
    const d = await purchaseApi.get(row.id)
    if (seq === detailSeq) detail.value = d
  } catch (err) {
    if (seq === detailSeq) {
      showDetail.value = false
      toast.error(err?.message || 'Gagal memuat detail.')
    }
  }
}

onMounted(async () => {
  try {
    const res = await outletsApi.myOutlets()
    const list = Array.isArray(res) ? res : (res?.data ?? [])
    outletOptions.value = [{ value: '', label: 'Semua outlet' }, ...list.map(o => ({ value: o.id, label: o.name }))]
  } catch { /* filter outlet tetap "Semua outlet" */ }
  await load()
})

useRealtime(['procurement', 'procurement_payment'], async () => {
  await load({ silent: true })
  if (showDetail.value && detail.value?.id) {
    try { detail.value = await purchaseApi.get(detail.value.id) } catch {}
  }
})
</script>
