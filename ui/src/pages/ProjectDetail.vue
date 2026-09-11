<template>
  <div class="space-y-5">
    <button @click="router.push('/projects')" class="flex items-center gap-1 text-xs font-medium text-blue-600 hover:text-blue-800">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><polyline points="12 19 5 12 12 5"/></svg>
      Kembali ke Daftar Projek
    </button>

    <AppAlert type="error" :message="errorMsg" />

    <div v-if="loading" class="rounded-xl bg-white p-8 text-center text-sm text-gray-400 shadow-sm">Memuat…</div>

    <template v-else-if="project">
      <!-- Header -->
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <p class="font-mono text-xs text-gray-500">{{ project.project_number || '-' }}</p>
          <h1 class="text-lg sm:text-xl font-bold text-gray-900 break-words">{{ project.name }}</h1>
          <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500">
            <span :class="statusBadge(project.status)">{{ statusLabel(project.status) }}</span>
            <span v-if="project.work_unit_name">· {{ project.work_unit_name }}</span>
            <span v-if="project.pic">· PIC {{ project.pic }}</span>
            <span v-if="periode">· {{ periode }}</span>
          </div>
        </div>
        <AppButton v-if="canSubmit" @click="showNewPurchase = true">+ Belanja Tahap Baru</AppButton>
      </div>

      <!-- Rekap RAB -->
      <AppCard>
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <div v-for="t in tiles" :key="t.label">
            <p class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">{{ t.label }}</p>
            <p class="mt-0.5 text-base sm:text-lg font-bold" :class="t.cls">{{ formatRupiah(t.value) }}</p>
            <p v-if="t.hint" class="text-[10px] text-gray-400">{{ t.hint }}</p>
          </div>
        </div>
        <BudgetBar class="mt-4" :budget="project.budget" :committed="project.committed" :paid="project.paid" :estimated="project.estimated" />
        <p v-if="project.notes" class="mt-3 border-t border-gray-100 pt-3 text-xs text-gray-600">
          <span class="text-gray-400">Catatan:</span> {{ project.notes }}
        </p>
      </AppCard>

      <!-- Daftar belanja tahap -->
      <AppCard :padding="false">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
          <h2 class="text-sm font-semibold text-gray-800">Belanja Tahap ({{ requests.length }})</h2>
          <p class="text-[11px] text-gray-400">Pecahan vendor ditampilkan sebagai dokumen tersendiri.</p>
        </div>

        <!-- Mobile -->
        <div class="sm:hidden">
          <div v-if="!requests.length" class="p-6 text-center text-sm text-gray-400">Belum ada belanja pada projek ini.</div>
          <ul v-else class="divide-y divide-gray-100">
            <li v-for="r in requests" :key="r.id" class="p-4">
              <div class="flex items-start justify-between gap-2">
                <button @click="goRequest(r)" class="min-w-0 flex-1 text-left">
                  <p class="font-mono text-[11px] text-gray-500">{{ r.request_number || '-' }}</p>
                  <p class="mt-0.5 text-sm font-medium text-gray-900 break-words">{{ itemNames(r) }}</p>
                </button>
                <span :class="prStatusBadge(r.status)" class="shrink-0">{{ prStatusLabel(r.status) }}</span>
              </div>
              <dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Jenis</dt>
                <dd class="text-gray-700">{{ r.request_type === 'jasa' ? 'Jasa' : 'Barang' }}</dd>
                <dt class="text-gray-400">Vendor</dt>
                <dd class="text-gray-700">{{ r.vendor_name || '-' }}</dd>
                <dt class="text-gray-400">Tanggal</dt>
                <dd class="text-gray-600">{{ formatDateTime(r.created_at) }}</dd>
              </dl>
              <div class="mt-2 flex items-end justify-between gap-2 border-t border-gray-100 pt-2 text-xs">
                <div>
                  <p class="text-gray-400">Nilai <span v-if="isEstimate(r)" class="text-amber-600">(HPS)</span></p>
                  <p class="text-sm font-semibold text-gray-900">{{ formatRupiah(r.total_amount) }}</p>
                </div>
                <div class="text-right">
                  <p class="text-gray-400">Terbayar</p>
                  <p class="font-medium text-emerald-700">{{ formatRupiah(r.paid_amount) }}</p>
                </div>
              </div>
            </li>
          </ul>
        </div>

        <AppTable class="hidden sm:block" :columns="PR_COLUMNS" :rows="requests" emptyText="Belum ada belanja pada projek ini.">
          <template #cell-request_number="{ row }">
            <button @click="goRequest(row)" class="font-mono text-xs text-blue-600 hover:underline">{{ row.request_number || '-' }}</button>
          </template>
          <template #cell-items="{ row }">
            <span class="text-gray-800">{{ itemNames(row) }}</span>
          </template>
          <template #cell-request_type="{ row }">
            <span class="text-gray-600">{{ row.request_type === 'jasa' ? 'Jasa' : 'Barang' }}</span>
          </template>
          <template #cell-vendor_name="{ row }">
            <span class="text-gray-700">{{ row.vendor_name || '-' }}</span>
          </template>
          <template #cell-total_final="{ row }">
            {{ formatRupiah(row.total_amount) }}
            <span v-if="isEstimate(row)" class="ml-1 text-[10px] font-medium text-amber-600">HPS</span>
          </template>
          <template #cell-paid_amount="{ row }">
            <span class="text-emerald-700">{{ formatRupiah(row.paid_amount) }}</span>
          </template>
          <template #cell-status="{ row }">
            <span :class="prStatusBadge(row.status)">{{ prStatusLabel(row.status) }}</span>
          </template>
          <template #cell-created_at="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </AppTable>
      </AppCard>
    </template>

    <!-- Arahkan ke halaman pengadaan untuk membuat belanja tahap baru.
         Form pengajuan tetap satu-satunya tempat membuat PR, jadi tidak ada
         duplikasi alur di sini. -->
    <AppModal v-model="showNewPurchase" title="Belanja Tahap Baru" size="sm">
      <p class="text-sm text-gray-600">Belanja tahap dibuat lewat form pengadaan biasa. Pilih jenisnya:</p>
      <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <button @click="goNewPurchase('barang')"
          class="rounded-xl border-2 border-emerald-100 bg-white p-4 text-left transition-colors hover:border-emerald-500">
          <p class="text-sm font-bold text-gray-900">Barang</p>
          <p class="mt-0.5 text-[11px] leading-tight text-gray-500">Material, perabot, peralatan.</p>
        </button>
        <button @click="goNewPurchase('jasa')"
          class="rounded-xl border-2 border-blue-50 bg-white p-4 text-left transition-colors hover:border-blue-500">
          <p class="text-sm font-bold text-gray-900">Jasa</p>
          <p class="mt-0.5 text-[11px] leading-tight text-gray-500">Upah tukang, sewa alat, termin kontraktor.</p>
        </button>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showNewPurchase = false">Tutup</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { projectsApi } from '@/api/projects.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { useAuthStore } from '@/stores/auth.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard   from '@/components/ui/AppCard.vue'
import AppTable  from '@/components/ui/AppTable.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import BudgetBar from '@/components/BudgetBar.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const canSubmit = computed(() => authStore.hasPermission('procurement.requests.submit'))

const loading = ref(false)
const errorMsg = ref('')
const project = ref(null)
const requests = ref([])
const showNewPurchase = ref(false)

const PR_COLUMNS = [
  { key: 'request_number', label: 'Nomor' },
  { key: 'items',          label: 'Pengadaan' },
  { key: 'request_type',   label: 'Jenis' },
  { key: 'vendor_name',    label: 'Vendor' },
  { key: 'total_final',    label: 'Nilai' },
  { key: 'paid_amount',    label: 'Terbayar' },
  { key: 'status',         label: 'Status' },
  { key: 'created_at',     label: 'Tanggal' },
]

const projectStatusMap = {
  draft:    { label: 'Draft',    cls: 'bg-gray-100 text-gray-600' },
  berjalan: { label: 'Berjalan', cls: 'bg-blue-100 text-blue-700' },
  selesai:  { label: 'Selesai',  cls: 'bg-emerald-100 text-emerald-700' },
  batal:    { label: 'Batal',    cls: 'bg-red-100 text-red-700' },
}
const prStatusMap = {
  pending:           { label: 'Menunggu',            cls: 'bg-amber-100 text-amber-700' },
  approved:          { label: 'Disetujui',           cls: 'bg-blue-100 text-blue-700' },
  payment_requested: { label: 'Menunggu Pembayaran', cls: 'bg-orange-100 text-orange-700' },
  rejected:          { label: 'Ditolak',             cls: 'bg-red-100 text-red-700' },
  partial:           { label: 'Dibayar Sebagian',    cls: 'bg-amber-100 text-amber-700' },
  paid:              { label: 'Dibayar',             cls: 'bg-emerald-100 text-emerald-700' },
  received:          { label: 'Diterima',            cls: 'bg-purple-100 text-purple-700' },
  cancelled:         { label: 'Dibatalkan',          cls: 'bg-gray-100 text-gray-600' },
}
const badge = (map, s, fallback) => {
  const m = map[s] || map[fallback]
  return `inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${m.cls}`
}
const statusBadge   = (s) => badge(projectStatusMap, s, 'berjalan')
const statusLabel   = (s) => (projectStatusMap[s] || projectStatusMap.berjalan).label
const prStatusBadge = (s) => badge(prStatusMap, s, 'pending')
const prStatusLabel = (s) => (prStatusMap[s] || prStatusMap.pending).label

function itemNames(r) {
  return (r.items || []).map(i => i.name).join(', ') || '-'
}

// Baris yang harganya belum diisi purchasing dihitung memakai HPS — ditandai
// supaya angka rekap tidak terbaca sebagai harga pasti semuanya.
function isEstimate(r) {
  return !r.total_final && r.total_amount > 0
}

function komitmenHint(p) {
  const parts = []
  if (p.budget > 0) parts.push(`${Math.round(p.absorbed_pct)}% dari RAB`)
  if (p.estimated > 0) parts.push(`${formatRupiah(p.estimated)} masih HPS`)
  return parts.join(' · ')
}

const periode = computed(() => {
  if (!project.value) return ''
  const { start_date: a, target_date: b } = project.value
  if (a && b) return `${a} → ${b}`
  if (a) return `mulai ${a}`
  if (b) return `target ${b}`
  return ''
})

const tiles = computed(() => {
  const p = project.value
  if (!p) return []
  return [
    { label: 'RAB',         value: p.budget,      cls: 'text-gray-900' },
    { label: 'Komitmen',    value: p.committed,   cls: p.over_budget ? 'text-red-600' : 'text-blue-700',
      hint: komitmenHint(p) },
    { label: 'Terbayar',    value: p.paid,        cls: 'text-emerald-700' },
    { label: 'Sisa RAB',    value: p.remaining_budget, cls: p.remaining_budget < 0 ? 'text-red-600' : 'text-gray-900',
      hint: p.outstanding > 0 ? `sisa hutang ${formatRupiah(p.outstanding)}` : '' },
  ]
})

function goRequest(r) {
  router.push(r.request_type === 'jasa' ? '/purchase-services' : '/purchase-goods')
}

function goNewPurchase(type) {
  const path = type === 'jasa' ? '/purchase-services' : '/purchase-goods'
  router.push({ path, query: { project_id: route.params.id } })
}

async function fetchDetail() {
  loading.value = true; errorMsg.value = ''
  try {
    const res = await projectsApi.get(route.params.id)
    project.value = res?.project ?? null
    requests.value = res?.requests ?? []
  } catch (e) {
    errorMsg.value = e.message || 'Gagal memuat projek'
  } finally {
    loading.value = false
  }
}

onMounted(fetchDetail)
</script>
