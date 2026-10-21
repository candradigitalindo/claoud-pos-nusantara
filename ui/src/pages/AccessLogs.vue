<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Log Akses</h1>
        <p class="text-sm text-gray-500 mt-0.5">Riwayat login &amp; akses sistem — IP, browser, perangkat, waktu.</p>
      </div>
      <span class="inline-flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-tight px-2.5 py-1 rounded-full bg-violet-100 text-violet-700">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
        Khusus Superadmin
      </span>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div class="flex items-center gap-2 px-4 py-3 border-b border-gray-100 flex-wrap">
        <input v-model="search" @input="debouncedLoad" placeholder="Cari username / IP / role..."
          class="flex-1 min-w-[180px] text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-violet-400" />
        <DateRangePicker v-model="range" clearable />
        <SearchableSelect v-model="status" @change="reload" class="text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-violet-400">
          <option value="">Semua status</option>
          <option value="success">Berhasil</option>
          <option value="failed">Gagal</option>
        </SearchableSelect>
        <button @click="setToday" class="text-sm px-3 py-2 rounded-lg bg-violet-50 hover:bg-violet-100 text-violet-700 font-medium">Hari Ini</button>
        <button @click="reload" class="text-sm px-3 py-2 rounded-lg bg-gray-100 hover:bg-gray-200 text-gray-700">Muat ulang</button>
      </div>

      <AppTable :columns="COLUMNS" :rows="rows" :loading="loading" emptyText="Belum ada log akses.">
        <template #cell-created_at="{ row }">
          <span class="text-sm text-gray-800">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-username="{ row }">
          <span class="font-medium text-gray-900">{{ row.username || '—' }}</span>
        </template>
        <template #cell-role="{ row }">
          <span class="text-xs text-gray-500">{{ row.role || '—' }}</span>
        </template>
        <template #cell-status="{ row }">
          <span :class="row.status === 'success' ? 'bg-emerald-100 text-emerald-700' : 'bg-red-100 text-red-700'"
            class="text-xs font-semibold px-2 py-0.5 rounded-full">
            {{ row.status === 'success' ? 'Berhasil' : 'Gagal' }}
          </span>
        </template>
        <template #cell-ip="{ row }">
          <span class="font-mono text-xs text-gray-700">{{ row.ip || '—' }}</span>
        </template>
        <template #cell-device="{ row }">
          <div class="text-xs leading-tight">
            <span class="font-medium text-gray-800">{{ row.browser }}</span>
            <span class="text-gray-400"> · {{ row.os }}</span>
            <div class="text-gray-400">{{ row.device }}</div>
          </div>
        </template>
        <template #cell-user_agent="{ row }">
          <span class="text-[11px] text-gray-400 line-clamp-2 max-w-[280px] block" :title="row.user_agent">{{ row.user_agent || '—' }}</span>
        </template>
      </AppTable>
      <AppPagination v-model="page" :total="total" :perPage="limit" class="px-4 py-3 border-t border-gray-100" />
    </AppCard>
  </div>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { ref, watch, onMounted } from 'vue'
import { accessLogsApi } from '@/api/accessLogs.js'
import { formatDateTime } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const errorMsg = ref('')
const search = ref('')
const status = ref('')
const page = ref(1)
const limit = 25

// Today's date in the app timezone (YYYY-MM-DD); default the range to today only.
function todayStr() {
  const tz = localStorage.getItem('cloud_pos_timezone') || 'Asia/Jakarta'
  try { return new Intl.DateTimeFormat('en-CA', { timeZone: tz }).format(new Date()) }
  catch { return new Date().toISOString().slice(0, 10) }
}
const dateFrom = ref(todayStr())
const dateTo   = ref(todayStr())
const range = ref({ from: dateFrom.value, to: dateTo.value, label: 'Hari Ini' })
watch(range, (r) => { dateFrom.value = r.from; dateTo.value = r.to; reload() })

function setToday() {
  // Lewat range agar label picker ikut ter-update; watcher-nya yang me-reload.
  range.value = { from: todayStr(), to: todayStr(), label: 'Hari Ini' }
}

const COLUMNS = [
  { key: 'created_at', label: 'Waktu',     sortable: false },
  { key: 'username',   label: 'Username',  sortable: false },
  { key: 'role',       label: 'Role',      sortable: false },
  { key: 'status',     label: 'Status',    sortable: false },
  { key: 'ip',         label: 'IP',        sortable: false },
  { key: 'device',     label: 'Browser / OS', sortable: false },
  { key: 'user_agent', label: 'User Agent', sortable: false },
]

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await accessLogsApi.list({ page: page.value, limit, search: search.value, status: status.value, date_from: dateFrom.value, date_to: dateTo.value })
    rows.value = data.data || []
    total.value = data.total || 0
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat log akses'
  } finally {
    loading.value = false
  }
}

function reload() { page.value = 1; load() }

let t = null
function debouncedLoad() {
  clearTimeout(t)
  t = setTimeout(() => { page.value = 1; load() }, 400)
}

watch(page, load)
onMounted(load)
</script>
