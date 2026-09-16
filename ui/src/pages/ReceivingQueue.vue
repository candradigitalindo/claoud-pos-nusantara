<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="min-w-0">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">{{ title }}</h1>
      <p class="mt-0.5 text-sm text-gray-500">{{ subtitle }}</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat antrean…</div>
      <div v-else-if="!rows.length" class="p-8 text-center text-sm text-gray-400">{{ emptyText }}</div>

      <template v-else>
        <!-- Mobile -->
        <ul class="divide-y divide-gray-100 sm:hidden">
          <li v-for="r in rows" :key="r.purchase_request_id" class="space-y-2 p-4">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-mono text-sm font-semibold text-gray-900">{{ r.request_number }}</p>
                <p class="text-xs text-gray-500">{{ r.outlet_name || '—' }}<span v-if="r.vendor_name"> · {{ r.vendor_name }}</span></p>
              </div>
              <span :class="payCls(r)">{{ payLabel(r) }}</span>
            </div>
            <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <dt class="text-gray-400">Belum diterima</dt><dd class="text-gray-700">{{ r.lines }} baris · {{ r.units }} unit</dd>
              <dt class="text-gray-400">Nilai</dt><dd class="text-gray-700">{{ formatRupiah(r.value) }}</dd>
              <dt class="text-gray-400">Dibayar</dt><dd class="text-gray-700">{{ r.paid_at || '—' }}</dd>
            </dl>
            <button class="act-btn bg-emerald-600 text-white" @click="open(r)">Terima Barang</button>
          </li>
        </ul>

        <!-- Desktop -->
        <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :emptyText="emptyText">
          <template #cell-number="{ row }">
            <p class="font-mono text-xs font-semibold text-gray-900">{{ row.request_number }}</p>
            <p class="text-[11px] text-gray-400">{{ row.paid_at || '—' }}</p>
          </template>
          <template #cell-outlet="{ row }">{{ row.outlet_name || '—' }}</template>
          <template #cell-vendor="{ row }">{{ row.vendor_name || '—' }}</template>
          <template #cell-lines="{ row }">{{ row.lines }} baris · {{ row.units }} unit</template>
          <template #cell-pay="{ row }"><span :class="payCls(row)">{{ payLabel(row) }}</span></template>
          <template #cell-value="{ row }">{{ formatRupiah(row.value) }}</template>
          <template #cell-actions="{ row }">
            <button class="rounded px-2 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50" @click="open(row)">
              Terima Barang
            </button>
          </template>
        </AppTable>
      </template>
    </AppCard>

    <ReceivingDialog v-model="showDialog" :purchase-request-id="activeId" :kind="kind" @done="onDone" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { receivingApi } from '@/api/purchase.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import ReceivingDialog from '@/components/ReceivingDialog.vue'

const route = useRoute()
const toast = useToastStore()

// Satu halaman, dua meja. Yang membedakan hanya `kind` dari meta rute:
// bagian Aset menerima perlengkapan, Gudang Induk menerima barang dapur.
const kind = computed(() => route.meta?.receivingKind || 'perlengkapan')
const isAset = computed(() => kind.value === 'perlengkapan')

const title = computed(() => isAset.value ? 'Penerimaan Peralatan' : 'Penerimaan Barang Dapur')
const subtitle = computed(() => isAset.value
  ? 'Serah terima belanja peralatan di bagian Aset — setiap barang dicatat sebagai aset bernomor.'
  : 'Serah terima barang dapur di Gudang Induk — setiap barang masuk buku stok.')
const emptyText = computed(() => isAset.value
  ? 'Tidak ada peralatan yang menunggu diterima. Pengadaan yang sudah dibayar akan muncul di sini.'
  : 'Tidak ada barang dapur yang menunggu diterima di gudang.')

const COLUMNS = [
  { key: 'number', label: 'Pengadaan' },
  { key: 'outlet', label: 'Outlet' },
  { key: 'vendor', label: 'Vendor' },
  { key: 'lines', label: 'Belum Diterima' },
  { key: 'pay', label: 'Pembayaran' },
  { key: 'value', label: 'Nilai' },
  { key: 'actions', label: '' },
]

const rows = ref([])
const loading = ref(false)
const errorMsg = ref('')
const showDialog = ref(false)
const activeId = ref('')

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }

// Barang boleh diterima sebelum dibayar (tempo). Petugas tetap perlu tahu
// keadaannya: menerima barang yang belum lunas itu sah, tapi bukan hal yang
// sama dengan menerima barang yang sudah dibayar.
function payLabel(r) {
  if (r.paid_amount >= r.total_final && r.total_final > 0) return 'Sudah dibayar'
  if (r.paid_amount > 0) return 'Dibayar sebagian'
  return 'Belum dibayar (tempo)'
}
function payCls(r) {
  if (r.paid_amount >= r.total_final && r.total_final > 0) return 'badge-ok'
  if (r.paid_amount > 0) return 'badge-warn'
  return 'badge-info'
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await receivingApi.queue(kind.value))
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat antrean' } finally { loading.value = false }
}

function open(r) {
  activeId.value = r.purchase_request_id
  showDialog.value = true
}

async function onDone(res) {
  if (res?.outstanding_lines) {
    toast.success(`Tercatat. ${res.outstanding_lines} baris masih menunggu meja lain.`)
  }
  await load()
}

onMounted(load)
</script>

<style scoped>
.act-btn { width: 100%; min-height: 40px; border-radius: .6rem; padding: .4rem .5rem; font-size: .8rem; font-weight: 600; text-align: center; }
</style>
