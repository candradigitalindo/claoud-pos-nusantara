<!--
  ShiftReconciliation.vue — Laporan > Rekonsiliasi Shift
  Bandingkan penjualan versi kasir (tutup shift di tablet) dengan yang masuk cloud.
  Superadmin bisa "ikuti versi kasir" (tambah penyesuaian) — tercatat & bisa dibatalkan.
-->
<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-xl font-bold text-gray-900">Rekonsiliasi Shift</h1>
      <p class="text-sm text-gray-500 mt-0.5">Cocokkan tutup kasir (tablet) dengan pendapatan yang masuk cloud. Selisih = transaksi yang belum tersinkron.</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Notif -->
    <div v-if="report && report.summary.short_count > 0" class="notif notif-warn">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      <div>
        <p class="font-bold">{{ report.summary.short_count }} shift kurang di cloud (total {{ formatRupiah(report.summary.short_total) }})</p>
        <p class="text-sm opacity-90">Ada transaksi yang dicatat kasir tapi belum masuk cloud. Superadmin bisa "Ikuti versi kasir" untuk menyamakan (bisa dibatalkan).</p>
      </div>
    </div>
    <div v-else-if="report && report.summary.total_shifts > 0" class="notif notif-ok">
      <svg class="w-5 h-5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      <p class="font-bold">Semua {{ report.summary.total_shifts }} shift cocok dengan cloud. 🎉</p>
    </div>

    <!-- Filter -->
    <AppCard>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <DateRangePicker v-model="range" />
        <SearchSelect v-model="filterOutlet" :options="outletOptions" placeholder="Semua outlet" searchPlaceholder="Cari outlet…" valueKey="value" labelKey="label" @change="load" />
        <button @click="load" class="px-4 py-2 bg-emerald-600 text-white text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-sm">Tampilkan</button>
      </div>
    </AppCard>

    <!-- Summary -->
    <div v-if="report" class="grid grid-cols-2 lg:grid-cols-4 gap-3">
      <div class="stat"><span class="stat-l">Total Shift</span><span class="stat-v">{{ report.summary.total_shifts }}</span><span class="stat-s">{{ report.summary.balanced_count }} cocok</span></div>
      <div class="stat" :class="report.summary.short_count ? 'stat-bad' : ''"><span class="stat-l">Kurang di Cloud</span><span class="stat-v">{{ report.summary.short_count }}</span><span class="stat-s stat-v-sm">{{ formatRupiah(report.summary.short_total) }}</span></div>
      <div class="stat" :class="report.summary.over_count ? 'stat-warn' : ''"><span class="stat-l">Lebih di Cloud</span><span class="stat-v">{{ report.summary.over_count }}</span><span class="stat-s stat-v-sm">{{ formatRupiah(report.summary.over_total) }}</span></div>
      <div class="stat stat-ok"><span class="stat-l">Sudah Disesuaikan</span><span class="stat-v">{{ report.summary.adjusted_count }}</span><span class="stat-s stat-v-sm">{{ formatRupiah(report.summary.adjusted_total) }}</span></div>
    </div>

    <!-- List -->
    <AppCard :padding="false">
      <div v-if="loading" class="p-8 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!report || !report.shifts.length" class="p-8 text-center text-sm text-gray-400">Belum ada shift tertutup pada periode ini.</div>
      <template v-else>
        <!-- Mobile -->
        <ul class="sm:hidden divide-y divide-gray-100">
          <li v-for="s in report.shifts" :key="s.shift_id" class="p-4 space-y-2">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="font-semibold text-gray-900">{{ s.outlet_name }}</p>
                <p class="text-xs text-gray-500">{{ s.cashiers || '—' }}</p>
              </div>
              <span class="rbadge" :class="stCls(s)">{{ stLabel(s) }}</span>
            </div>
            <p class="text-xs text-gray-500">🟢 {{ s.opened_at }} → 🔴 {{ s.closed_at }}</p>
            <div class="grid grid-cols-3 gap-1 text-xs">
              <div><span class="text-gray-400 block">Kasir</span>{{ formatRupiah(s.cashier_sales) }}</div>
              <div><span class="text-gray-400 block">Cloud</span>{{ formatRupiah(s.cloud_sales) }}</div>
              <div><span class="text-gray-400 block">Selisih</span><b :class="s.diff>0?'text-red-600':(s.diff<0?'text-amber-600':'text-gray-500')">{{ formatRupiah(s.diff) }}</b></div>
            </div>
            <div v-if="isSuper" class="pt-1">
              <AppActionButton icon="cancel" label="Batalkan penyesuaian" v-if="s.adjusted" @click="askRevert(s)" />
              <AppActionButton icon="approve" :label="&quot;Ikuti versi kasir (+ &quot; + (formatRupiah(s.diff)) + &quot; )&quot;" v-else-if="s.status==='short'" @click="askApply(s)" />
            </div>
          </li>
        </ul>
        <!-- Desktop -->
        <div class="hidden sm:block overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-left text-gray-500">
                <th class="py-2.5 px-3 font-medium">Outlet / Kasir</th>
                <th class="py-2.5 px-3 font-medium">Buka → Tutup</th>
                <th class="py-2.5 px-3 font-medium text-right">Kasir</th>
                <th class="py-2.5 px-3 font-medium text-right">Cloud</th>
                <th class="py-2.5 px-3 font-medium text-right">Selisih</th>
                <th class="py-2.5 px-3 font-medium text-center">Status</th>
                <th v-if="isSuper" class="py-2.5 px-3 font-medium text-right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in report.shifts" :key="s.shift_id" class="border-b border-gray-50 hover:bg-gray-50">
                <td class="py-2.5 px-3">
                  <p class="font-medium text-gray-900">{{ s.outlet_name }}</p>
                  <p class="text-xs text-gray-500">{{ s.cashiers || '—' }} · {{ s.cashier_count }} vs {{ s.cloud_count }} trx</p>
                </td>
                <td class="py-2.5 px-3 text-xs text-gray-500">{{ s.opened_at }}<br>{{ s.closed_at }}</td>
                <td class="py-2.5 px-3 text-right font-medium">{{ formatRupiah(s.cashier_sales) }}</td>
                <td class="py-2.5 px-3 text-right">{{ formatRupiah(s.cloud_sales) }}</td>
                <td class="py-2.5 px-3 text-right font-semibold" :class="s.diff>0?'text-red-600':(s.diff<0?'text-amber-600':'text-gray-400')">{{ formatRupiah(s.diff) }}</td>
                <td class="py-2.5 px-3 text-center"><span class="rbadge" :class="stCls(s)">{{ stLabel(s) }}</span></td>
                <td v-if="isSuper" class="py-2.5 px-3 text-right">
                  <AppActionButton icon="cancel" label="Batalkan" v-if="s.adjusted" @click="askRevert(s)" />
                  <AppActionButton icon="approve" label="Ikuti kasir" v-else-if="s.status==='short'" @click="askApply(s)" />
                  <span v-else class="text-xs text-gray-300">—</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </AppCard>

    <!-- Konfirmasi -->
    <AppModal v-model="confirmOpen" :title="confirmMode==='apply' ? 'Ikuti versi kasir?' : 'Batalkan penyesuaian?'" size="md">
      <div v-if="active" class="space-y-3 text-sm">
        <p v-if="confirmMode==='apply'" class="text-gray-600">
          Menambahkan penyesuaian <b class="text-emerald-700">+{{ formatRupiah(active.diff) }}</b> ke pendapatan
          <b>{{ active.outlet_name }}</b> agar sama dengan versi kasir ({{ formatRupiah(active.cashier_sales) }}).
        </p>
        <p v-else class="text-gray-600">
          Membatalkan penyesuaian <b>{{ active.outlet_name }}</b> — pendapatan kembali ke data cloud asli ({{ formatRupiah(active.cloud_sales) }}).
        </p>
        <div class="p-3 rounded-lg bg-blue-50 border border-blue-200 text-xs text-blue-700">
          <b>Tercatat & bisa dibatalkan.</b> Penyesuaian disimpan sebagai transaksi bertanda "adjustment" + catatan audit tersendiri, jadi bila keliru nilainya bisa dikembalikan kapan saja.
        </div>
        <div class="flex justify-end gap-2 pt-2">
          <button class="px-4 py-2 text-sm rounded-lg border border-gray-200 hover:bg-gray-50" @click="confirmOpen=false">Batal</button>
          <button class="px-4 py-2 text-sm rounded-lg text-white shadow-sm" :class="confirmMode==='apply'?'bg-emerald-600 hover:bg-emerald-700':'bg-amber-600 hover:bg-amber-700'" :disabled="busy" @click="doConfirm">
            {{ busy ? 'Memproses…' : (confirmMode==='apply' ? 'Ya, sesuaikan' : 'Ya, batalkan') }}
          </button>
        </div>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, computed, onMounted, watch } from 'vue'
import { reconciliationApi } from '@/api/reconciliation.js'
import { outletsApi } from '@/api/outlets.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'

const auth = useAuthStore()
const toast = useToastStore()
const isSuper = computed(() => auth.isSuperadmin)

const _today = new Date().toISOString().slice(0, 10)
const range = ref({ from: _today, to: _today, label: 'Hari Ini' })
const filterOutlet = ref('')
const outletOptions = ref([{ value: '', label: 'Semua outlet' }])
const report = ref(null)
const loading = ref(false)
const errorMsg = ref('')

const confirmOpen = ref(false)
const confirmMode = ref('apply')
const active = ref(null)
const busy = ref(false)

watch(range, load)

function stCls(s) {
  if (s.adjusted) return 'r-adj'
  return { balanced: 'r-ok', short: 'r-bad', over: 'r-warn' }[s.status] || 'r-ok'
}
function stLabel(s) {
  if (s.adjusted) return 'Disesuaikan'
  return { balanced: 'Cocok', short: 'Kurang', over: 'Lebih' }[s.status] || s.status
}

function askApply(s) { active.value = s; confirmMode.value = 'apply'; confirmOpen.value = true }
function askRevert(s) { active.value = s; confirmMode.value = 'revert'; confirmOpen.value = true }

async function doConfirm() {
  if (!active.value) return
  busy.value = true
  try {
    if (confirmMode.value === 'apply') {
      await reconciliationApi.apply(active.value.shift_id)
      toast.success('Pendapatan disesuaikan ke versi kasir')
    } else {
      await reconciliationApi.revert(active.value.adjustment_id)
      toast.success('Penyesuaian dibatalkan, nilai dikembalikan')
    }
    confirmOpen.value = false
    await load()
  } catch (e) {
    toast.error(e?.response?.data?.error || e?.message || 'Gagal memproses')
  } finally {
    busy.value = false
  }
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    const d = await reconciliationApi.report({
      date_from: range.value.from,
      date_to: range.value.to,
      outlet_id: filterOutlet.value || undefined,
    })
    report.value = d?.data ?? d
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat rekonsiliasi' } finally { loading.value = false }
}

onMounted(async () => {
  try {
    const res = await outletsApi.myOutlets()
    const listData = res.data ?? res ?? []
    outletOptions.value = [{ value: '', label: 'Semua outlet' }, ...listData.map(o => ({ value: o.id, label: o.name }))]
  } catch {}
  await load()
})
</script>

<style scoped>
.notif { display: flex; gap: .7rem; align-items: flex-start; padding: .85rem 1rem; border-radius: .9rem; }
.notif-warn { background: rgba(245,158,11,.12); color: #92400e; border: 1px solid rgba(245,158,11,.3); }
.notif-ok { background: rgba(16,185,129,.1); color: #065f46; border: 1px solid rgba(16,185,129,.25); }

.stat { background: #fff; border: 1px solid rgba(0,0,0,.07); border-radius: 1rem; padding: .85rem 1rem; display: flex; flex-direction: column; gap: .1rem; box-shadow: 0 1px 2px rgba(0,0,0,.04); }
.stat-l { font-size: .68rem; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: #6b7280; }
.stat-v { font-size: 1.5rem; font-weight: 800; color: #111827; line-height: 1.1; }
.stat-v-sm { font-size: .82rem; font-weight: 600; }
.stat-s { font-size: .7rem; color: #9ca3af; }
.stat-ok { background: rgba(16,185,129,.06); border-color: rgba(16,185,129,.25); }
.stat-ok .stat-v { color: #047857; }
.stat-bad { background: rgba(239,68,68,.05); border-color: rgba(239,68,68,.25); }
.stat-bad .stat-v { color: #b91c1c; }
.stat-warn { background: rgba(245,158,11,.06); border-color: rgba(245,158,11,.3); }
.stat-warn .stat-v { color: #b45309; }

.rbadge { display: inline-block; padding: .15rem .55rem; border-radius: 999px; font-size: .7rem; font-weight: 700; white-space: nowrap; }
.r-ok { background: rgba(16,185,129,.14); color: #047857; }
.r-bad { background: rgba(239,68,68,.13); color: #b91c1c; }
.r-warn { background: rgba(245,158,11,.16); color: #b45309; }
.r-adj { background: rgba(59,130,246,.13); color: #1d4ed8; }

.rbtn { font-size: .72rem; font-weight: 600; padding: .3rem .7rem; border-radius: .5rem; cursor: pointer; border: 1px solid transparent; white-space: nowrap; }
.rbtn-apply { background: rgba(16,185,129,.12); color: #047857; border-color: rgba(16,185,129,.3); }
.rbtn-apply:hover { background: rgba(16,185,129,.2); }
.rbtn-revert { background: rgba(245,158,11,.12); color: #b45309; border-color: rgba(245,158,11,.3); }
.rbtn-revert:hover { background: rgba(245,158,11,.2); }
</style>
