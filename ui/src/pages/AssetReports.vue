<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="min-w-0">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Laporan Aset</h1>
      <p class="mt-0.5 text-sm text-gray-500">Nilai, penyusutan, perawatan, mutasi, dan rekonsiliasi pengadaan.</p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Ringkasan -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard class="min-w-0">
        <p class="stat-lbl">Nilai Perolehan</p>
        <p class="stat-val">{{ formatRupiah(summary.acquisition_cost) }}</p>
        <p class="stat-sub">{{ summary.asset_count }} aset · {{ summary.unit_count }} unit</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Nilai Buku</p>
        <p class="stat-val">{{ formatRupiah(summary.book_value) }}</p>
        <p class="stat-sub">susut {{ formatRupiah(summary.depreciation) }}</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Perlu Perhatian</p>
        <p class="stat-val" :class="(summary.damaged || summary.under_repair) ? 'text-amber-600' : ''">
          {{ summary.damaged }}
        </p>
        <p class="stat-sub">{{ summary.under_repair }} diperbaiki · {{ summary.in_transit }} di jalan</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Belum Lengkap</p>
        <p class="stat-val" :class="summary.incomplete_lines ? 'text-red-600' : ''">{{ summary.incomplete_lines }}</p>
        <p class="stat-sub">baris pengadaan tanpa wujud data</p>
      </AppCard>
    </div>

    <!-- Pilihan laporan -->
    <div class="-mx-1 overflow-x-auto">
      <div class="flex gap-1 px-1 pb-1">
        <button v-for="t in TYPES" :key="t.key" @click="type = t.key; load()"
          class="shrink-0 rounded-lg px-3 py-2 text-xs font-semibold transition"
          :class="type === t.key ? 'bg-emerald-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'">
          {{ t.label }}
        </button>
      </div>
    </div>

    <AppCard v-if="type === 'perawatan'">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div>
          <label class="lbl">Dari</label>
          <input v-model="from" type="date" class="form-input" @change="load" />
        </div>
        <div>
          <label class="lbl">Sampai</label>
          <input v-model="to" type="date" class="form-input" @change="load" />
        </div>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 p-3">
        <p class="text-sm font-bold text-gray-900">{{ report?.title || '—' }}</p>
        <span class="text-xs text-gray-400">{{ report?.rows?.length || 0 }} baris</span>
        <button class="btn-soft ml-auto" :disabled="exporting || !report?.rows?.length" @click="doExport">
          {{ exporting ? 'Menyiapkan…' : 'Export Excel' }}
        </button>
      </div>

      <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!report?.rows?.length" class="p-8 text-center text-sm text-gray-400">
        {{ emptyText }}
      </div>
      <template v-else>
        <!-- Mobile: kartu per baris -->
        <ul class="divide-y divide-gray-100 sm:hidden">
          <li v-for="(r, i) in report.rows" :key="i" class="p-4">
            <p class="break-words font-semibold text-gray-900">{{ r.cells[headlineIdx] }}</p>
            <dl class="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
              <template v-for="(h, c) in report.headers" :key="c">
                <template v-if="c !== headlineIdx && r.cells[c]">
                  <dt class="text-gray-400">{{ h }}</dt>
                  <dd class="break-words text-gray-700">{{ fmtCell(h, r.cells[c]) }}</dd>
                </template>
              </template>
            </dl>
          </li>
        </ul>
        <!-- Desktop: tabel -->
        <div class="hidden overflow-x-auto sm:block">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500">
              <tr><th v-for="h in report.headers" :key="h" class="whitespace-nowrap p-2 text-left">{{ h }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="(r, i) in report.rows" :key="i" class="hover:bg-gray-50">
                <td v-for="(v, c) in r.cells" :key="c" class="p-2 align-top text-gray-700">
                  {{ fmtCell(report.headers[c], v) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="totalsList.length" class="flex flex-wrap gap-4 border-t border-gray-100 p-3 text-sm">
          <span v-for="t in totalsList" :key="t.key" class="text-gray-600">
            {{ t.label }}: <strong class="text-gray-900">{{ formatRupiah(t.value) }}</strong>
          </span>
        </div>
      </template>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetReportsApi } from '@/api/assetOps.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'

const toast = useToastStore()

const TYPES = [
  { key: 'daftar', label: 'Daftar Aset' },
  { key: 'penyusutan', label: 'Penyusutan' },
  { key: 'perawatan', label: 'Biaya Perawatan' },
  { key: 'mutasi', label: 'Mutasi' },
  { key: 'belum-lengkap', label: 'Pengadaan Belum Lengkap' },
]

const type = ref('daftar')
const report = ref(null)
const summary = ref({ asset_count: 0, unit_count: 0, acquisition_cost: 0, book_value: 0, depreciation: 0, damaged: 0, under_repair: 0, in_transit: 0, incomplete_lines: 0 })
const loading = ref(false)
const exporting = ref(false)
const errorMsg = ref('')
const from = ref('')
const to = ref('')

function asObject(d) { return d?.data ?? d ?? null }

// Kolom pertama yang layak jadi judul kartu di ponsel.
const headlineIdx = computed(() => (type.value === 'daftar' || type.value === 'penyusutan' ? 1 : 0))

const emptyText = computed(() => {
  if (type.value === 'belum-lengkap') return 'Tidak ada barang yang tertinggal — setiap baris pengadaan yang diterima sudah berwujud aset, stok, atau keputusan tertulis.'
  if (type.value === 'mutasi') return 'Belum ada mutasi aset yang dikirim.'
  if (type.value === 'perawatan') return 'Belum ada perawatan selesai pada rentang ini.'
  return 'Belum ada data aset.'
})

const TOTAL_LABELS = {
  nilai_perolehan: 'Nilai perolehan', perolehan: 'Nilai perolehan',
  penyusutan: 'Akumulasi penyusutan', nilai_buku: 'Nilai buku',
  biaya: 'Total biaya', downtime: 'Total downtime (jam)', nilai_kurang: 'Nilai belum tercatat',
}
const totalsList = computed(() => Object.entries(report.value?.totals || {})
  .map(([key, value]) => ({ key, label: TOTAL_LABELS[key] || key, value })))

// Kolom bernilai uang diformat rupiah; sisanya apa adanya.
const MONEY_HEADS = ['Harga Perolehan', 'Akumulasi Penyusutan', 'Nilai Buku', 'Biaya', 'Nilai Kurang']
function fmtCell(header, v) {
  if (MONEY_HEADS.includes(header) && v !== '' && !isNaN(Number(v))) return formatRupiah(Number(v))
  return v
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    report.value = asObject(await assetReportsApi.report(type.value, {
      from: from.value || undefined, to: to.value || undefined,
    }))
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat laporan' } finally { loading.value = false }
}

async function loadSummary() {
  try { summary.value = asObject(await assetReportsApi.summary()) || summary.value } catch { /* kartu tetap 0 */ }
}

async function doExport() {
  exporting.value = true
  try {
    const blob = await assetReportsApi.export(type.value, { from: from.value || undefined, to: to.value || undefined })
    const url = URL.createObjectURL(blob instanceof Blob ? blob : new Blob([blob]))
    const a = document.createElement('a')
    a.href = url
    a.download = `Laporan-Aset_${type.value}.xlsx`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch (e) { toast.error(e?.message || 'Gagal mengunduh laporan') } finally { exporting.value = false }
}

onMounted(async () => { await Promise.all([loadSummary(), load()]) })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.08); min-height: 40px;
}
.btn-soft:hover:not(:disabled) { background: #f3f4f6; }
.btn-soft:disabled { opacity: .5; cursor: not-allowed; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; overflow-wrap: anywhere; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
</style>
