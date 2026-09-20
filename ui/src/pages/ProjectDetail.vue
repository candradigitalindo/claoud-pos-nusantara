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
        <div class="flex flex-wrap gap-2">
          <AppButton v-if="canManage && !rabSet && projectOpen" variant="secondary" @click="openRabEditor">{{ rabItems.length ? 'Ubah RAB' : 'Susun RAB' }}</AppButton>
          <AppButton v-if="canSubmit" :disabled="!rabSet || !projectOpen" :title="rabSet ? '' : 'Tetapkan RAB dulu sebelum membuat belanja tahap'" @click="showNewPurchase = true">+ Belanja Tahap Baru</AppButton>
        </div>
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

      <!-- RAB per baris: pos anggaran dan serapannya -->
      <AppCard :padding="false">
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-4 py-3">
          <div class="min-w-0">
            <h2 class="flex flex-wrap items-center gap-2 text-sm font-semibold text-gray-800">
              RAB — Rencana Anggaran Biaya
              <span :class="rabBadgeCls">{{ rabBadgeText }}</span>
            </h2>
            <p class="text-[11px] text-gray-400">
              <template v-if="rabSet">Ditetapkan<span v-if="project.rab_set_by"> oleh {{ project.rab_set_by }}</span><span v-if="project.rab_set_at"> · {{ project.rab_set_at }}</span>. Belanja tahap menyerap baris di bawah; ubah lewat “Buka Revisi”.</template>
              <template v-else-if="project.rab_version > 0">Sedang direvisi (terakhir v{{ project.rab_version }}). Belanja tahap baru ditahan sampai ditetapkan lagi.</template>
              <template v-else>Belum ditetapkan. Susun baris pekerjaan (volume × harga satuan), lalu tetapkan agar belanja tahap bisa dibuat.</template>
            </p>
          </div>
          <div v-if="canManage && projectOpen" class="flex flex-wrap gap-2">
            <template v-if="!rabSet">
              <AppButton size="sm" variant="secondary" @click="openRabEditor">{{ rabItems.length ? 'Ubah RAB' : 'Susun RAB' }}</AppButton>
              <AppButton size="sm" :disabled="!rabItems.length" :loading="rabBusy" @click="setRab">Tetapkan RAB</AppButton>
            </template>
            <AppButton v-else size="sm" variant="secondary" :loading="rabBusy" @click="reopenRab">Buka Revisi RAB</AppButton>
          </div>
        </div>

        <div v-if="!rabItems.length" class="p-6 text-center text-sm text-gray-400">
          Belum ada baris RAB.<span v-if="canManage && projectOpen"> Klik <b>Susun RAB</b> untuk memulai.</span>
        </div>
        <template v-else>
          <!-- Ponsel -->
          <div class="sm:hidden">
            <div v-for="(g, gi) in rabGroups" :key="gi">
              <div class="flex items-center justify-between gap-2 bg-gray-50 px-4 py-1.5 text-[11px] font-bold uppercase tracking-wide text-gray-500">
                <span class="min-w-0 truncate">{{ romawi(gi) }}. {{ g.section || 'Tanpa bagian' }}</span>
                <span class="shrink-0">{{ formatRupiah(g.total) }}</span>
              </div>
              <ul class="divide-y divide-gray-100">
                <li v-for="r in g.rows" :key="r.id" class="px-4 py-2.5">
                  <div class="flex items-start justify-between gap-2">
                    <p class="min-w-0 break-words text-sm font-medium text-gray-900">{{ r.name }} <span class="text-[10px] font-semibold uppercase text-gray-400">{{ r.kind }}</span></p>
                    <p class="shrink-0 text-sm font-semibold text-gray-900">{{ formatRupiah(r.subtotal) }}</p>
                  </div>
                  <p class="text-[11px] text-gray-500">{{ r.qty }} {{ r.unit }} × {{ formatRupiah(r.unit_price) }}<span v-if="r.notes"> · {{ r.notes }}</span></p>
                  <BudgetBar class="mt-1.5" :budget="r.subtotal" :committed="r.committed" :paid="r.paid" :estimated="r.estimated" />
                  <p class="mt-1 text-[11px]" :class="r.remaining < 0 ? 'font-semibold text-red-600' : 'text-gray-500'">Sisa {{ formatRupiah(r.remaining) }}<span v-if="r.request_count"> · {{ r.request_count }} pengajuan</span></p>
                </li>
              </ul>
            </div>
          </div>

          <!-- Desktop -->
          <div class="hidden overflow-x-auto sm:block">
            <table class="w-full text-xs">
              <thead>
                <tr class="border-b border-gray-200 text-left text-[11px] uppercase tracking-wide text-gray-500">
                  <th class="px-4 py-2">Uraian</th>
                  <th class="px-2 py-2">Jenis</th>
                  <th class="px-2 py-2 text-right">Vol</th>
                  <th class="px-2 py-2">Sat</th>
                  <th class="px-2 py-2 text-right">Harga Sat.</th>
                  <th class="px-2 py-2 text-right">RAB</th>
                  <th class="px-2 py-2 text-right">Komitmen</th>
                  <th class="px-2 py-2 text-right">Terbayar</th>
                  <th class="px-4 py-2 text-right">Sisa</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="(g, gi) in rabGroups" :key="gi">
                  <tr class="bg-gray-50 text-[11px] font-bold uppercase tracking-wide text-gray-600">
                    <td colspan="5" class="px-4 py-1.5">{{ romawi(gi) }}. {{ g.section || 'Tanpa bagian' }}</td>
                    <td class="px-2 py-1.5 text-right">{{ formatRupiah(g.total) }}</td>
                    <td class="px-2 py-1.5 text-right font-semibold text-gray-500">{{ formatRupiah(g.committed) }}</td>
                    <td class="px-2 py-1.5 text-right font-semibold text-gray-500">{{ formatRupiah(g.paid) }}</td>
                    <td class="px-4 py-1.5 text-right" :class="g.total - g.committed < 0 ? 'text-red-600' : ''">{{ formatRupiah(g.total - g.committed) }}</td>
                  </tr>
                  <tr v-for="r in g.rows" :key="r.id" class="border-b border-gray-100">
                    <td class="px-4 py-2">
                      <p class="font-medium text-gray-900">{{ r.name }}</p>
                      <p v-if="r.notes" class="text-[11px] text-gray-400">{{ r.notes }}</p>
                      <div class="mt-1 h-1 w-28 overflow-hidden rounded-full bg-gray-100" :title="`${Math.round(r.absorbed_pct)}% terserap`">
                        <div class="h-full rounded-full" :class="r.over_budget ? 'bg-red-500' : 'bg-blue-400'" :style="{ width: Math.min(100, r.absorbed_pct || 0) + '%' }"></div>
                      </div>
                    </td>
                    <td class="px-2 py-2 capitalize text-gray-600">{{ r.kind }}</td>
                    <td class="px-2 py-2 text-right text-gray-700">{{ r.qty }}</td>
                    <td class="px-2 py-2 text-gray-600">{{ r.unit }}</td>
                    <td class="px-2 py-2 text-right text-gray-700">{{ formatRupiah(r.unit_price) }}</td>
                    <td class="px-2 py-2 text-right font-medium text-gray-900">{{ formatRupiah(r.subtotal) }}</td>
                    <td class="px-2 py-2 text-right" :class="r.over_budget ? 'font-semibold text-red-600' : 'text-blue-700'">
                      {{ formatRupiah(r.committed) }}
                      <span v-if="r.estimated > 0" class="block text-[10px] text-amber-600">{{ formatRupiah(r.estimated) }} HPS</span>
                    </td>
                    <td class="px-2 py-2 text-right text-emerald-700">{{ formatRupiah(r.paid) }}</td>
                    <td class="px-4 py-2 text-right font-semibold" :class="r.remaining < 0 ? 'text-red-600' : 'text-gray-800'">{{ formatRupiah(r.remaining) }}</td>
                  </tr>
                </template>
              </tbody>
              <tfoot>
                <tr class="border-t-2 border-gray-200 bg-gray-50 font-semibold text-gray-900">
                  <td colspan="5" class="px-4 py-2">Total RAB</td>
                  <td class="px-2 py-2 text-right">{{ formatRupiah(project.budget) }}</td>
                  <td class="px-2 py-2 text-right text-blue-700">{{ formatRupiah(rabCommitted) }}</td>
                  <td class="px-2 py-2 text-right text-emerald-700">{{ formatRupiah(rabPaid) }}</td>
                  <td class="px-4 py-2 text-right" :class="project.budget - rabCommitted < 0 ? 'text-red-600' : ''">{{ formatRupiah(project.budget - rabCommitted) }}</td>
                </tr>
              </tfoot>
            </table>
          </div>
          <p v-if="project.off_rab_committed > 0" class="border-t border-amber-100 bg-amber-50 px-4 py-2 text-xs text-amber-800">
            Belanja di luar RAB: <b>{{ formatRupiah(project.off_rab_committed) }}</b> (terbayar {{ formatRupiah(project.off_rab_paid) }}) —
            item pengajuan yang tidak menunjuk baris RAB mana pun. Tetap terhitung dalam komitmen projek.
          </p>
        </template>
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

      <!-- Wujud barang dari belanja projek: yang habis dikonsumsi (material)
           dan yang bertahan (aset). Sebelumnya projek hanya memegang angka. -->
      <AppCard>
        <ProjectMaterialPanel :project-id="project.id" />
      </AppCard>

      <AppCard>
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-sm font-bold text-gray-900">Aset yang Dihasilkan Projek Ini</h2>
          <span v-if="projectAssets.length" class="text-xs text-gray-500">
            {{ projectAssets.length }} aset · {{ formatRupiah(projectAssetValue) }}
          </span>
        </div>
        <p v-if="loadingAssets" class="mt-3 text-sm text-gray-400">Memuat…</p>
        <p v-else-if="!projectAssets.length" class="mt-3 rounded-lg bg-gray-50 p-4 text-center text-sm text-gray-500">
          Belum ada aset dari projek ini. Aset lahir saat belanja projek diserahterimakan
          dengan tujuan “Aset”.
        </p>
        <ul v-else class="mt-3 divide-y divide-gray-100">
          <li v-for="a in projectAssets" :key="a.id" class="flex flex-wrap items-center gap-2 py-2">
            <button class="min-w-0 flex-1 text-left" @click="$router.push(`/perlengkapan/${a.id}`)">
              <p class="break-words text-sm font-medium text-gray-900 hover:text-emerald-700">{{ a.name }}</p>
              <p class="font-mono text-[11px] text-gray-400">{{ a.asset_no }} · {{ a.outlet_name }}</p>
            </button>
            <span class="text-sm text-gray-700">{{ formatRupiah(a.purchase_price) }}</span>
          </li>
        </ul>
      </AppCard>
    </template>

    <!-- Editor susunan RAB -->
    <AppModal v-model="showRabEditor" :title="`Susun RAB · ${project?.name || ''}`" size="2xl">
      <AppAlert type="error" :message="rabError" />
      <p class="mb-3 text-xs text-gray-500">
        Susun per bagian pekerjaan; volume × harga satuan menjadi jumlah tiap baris, dan total RAB = jumlah seluruh baris.
        Jenis baris menentukan pengajuan mana yang boleh menyerapnya (barang / jasa / umum untuk keduanya).
        Baris yang sudah dipakai pengajuan tidak bisa dihapus, hanya diubah.
      </p>
      <ProjectRabEditor v-model="rabDraft" />
      <template #footer>
        <AppButton variant="secondary" @click="showRabEditor = false">Batal</AppButton>
        <AppButton :loading="rabBusy" @click="saveRab">Simpan RAB</AppButton>
      </template>
    </AppModal>

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
import { useToastStore } from '@/stores/toast.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard   from '@/components/ui/AppCard.vue'
import ProjectMaterialPanel from '@/components/ProjectMaterialPanel.vue'
import ProjectRabEditor from '@/components/ProjectRabEditor.vue'
import { assetsApi } from '@/api/assets.js'
import AppTable  from '@/components/ui/AppTable.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import BudgetBar from '@/components/BudgetBar.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const toast = useToastStore()

const canSubmit = computed(() => authStore.hasPermission('procurement.requests.submit'))
const canManage = computed(() => authStore.hasPermission('procurement.projects.manage'))

// ── RAB per baris ──
const rabItems = ref([])
const rabSet = computed(() => project.value?.rab_status === 'ditetapkan')
const projectOpen = computed(() => project.value && project.value.status !== 'selesai' && project.value.status !== 'batal')
const rabBadgeText = computed(() => {
  const p = project.value
  if (!p) return ''
  if (rabSet.value) return `Ditetapkan v${p.rab_version}`
  return p.rab_version > 0 ? `Revisi (v${p.rab_version})` : 'Draft'
})
const rabBadgeCls = computed(() =>
  `inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold ${rabSet.value ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'}`)

// Kelompokkan baris per bagian pekerjaan, urutan sesuai seq.
const rabGroups = computed(() => {
  const out = []
  const idx = new Map()
  for (const r of rabItems.value) {
    const sec = r.section || ''
    if (!idx.has(sec)) { idx.set(sec, out.length); out.push({ section: sec, rows: [], total: 0, committed: 0, paid: 0 }) }
    const g = out[idx.get(sec)]
    g.rows.push(r); g.total += r.subtotal || 0; g.committed += r.committed || 0; g.paid += r.paid || 0
  }
  return out
})
const rabCommitted = computed(() => rabItems.value.reduce((s, r) => s + (r.committed || 0), 0))
const rabPaid = computed(() => rabItems.value.reduce((s, r) => s + (r.paid || 0), 0))
const ROMAWI = ['I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX', 'X', 'XI', 'XII', 'XIII', 'XIV', 'XV']
const romawi = (i) => ROMAWI[i] || String(i + 1)

const showRabEditor = ref(false)
const rabDraft = ref([])
const rabError = ref('')
const rabBusy = ref(false)

function openRabEditor() {
  rabDraft.value = rabItems.value.map(r => ({ id: r.id, section: r.section, name: r.name, kind: r.kind, unit: r.unit, qty: r.qty, unit_price: r.unit_price, notes: r.notes }))
  rabError.value = ''
  showRabEditor.value = true
}
async function saveRab() {
  const rows = rabDraft.value.filter(r => (r.name || '').trim())
  if (!rows.length) { rabError.value = 'Isi minimal satu baris RAB.'; return }
  rabBusy.value = true; rabError.value = ''
  try {
    await projectsApi.saveRab(project.value.id, rows)
    toast.success('RAB disimpan')
    showRabEditor.value = false
    await fetchDetail()
  } catch (e) { rabError.value = e.message || 'Gagal menyimpan RAB' }
  finally { rabBusy.value = false }
}
async function setRab() {
  if (!window.confirm(`Tetapkan RAB ${formatRupiah(project.value.budget)}? Setelah ini baris dikunci dan belanja tahap bisa dibuat.`)) return
  rabBusy.value = true
  try { await projectsApi.setRab(project.value.id); toast.success('RAB ditetapkan'); await fetchDetail() }
  catch (e) { toast.error(e.message || 'Gagal menetapkan RAB') }
  finally { rabBusy.value = false }
}
async function reopenRab() {
  if (!window.confirm('Buka RAB untuk direvisi? Belanja tahap baru ditahan sampai RAB ditetapkan lagi.')) return
  rabBusy.value = true
  try { await projectsApi.reopenRab(project.value.id); toast.success('RAB dibuka untuk revisi'); await fetchDetail() }
  catch (e) { toast.error(e.message || 'Gagal membuka RAB') }
  finally { rabBusy.value = false }
}

const loading = ref(false)
const errorMsg = ref('')
const project = ref(null)

// Aset hasil projek: dicocokkan lewat catatan pengadaan yang menautkannya.
const projectAssets = ref([])
const loadingAssets = ref(false)
const projectAssetValue = computed(() =>
  projectAssets.value.reduce((s, a) => s + (a.purchase_price || 0) * (a.quantity || 1), 0))

async function loadProjectAssets(prNumbers) {
  if (!prNumbers.length) { projectAssets.value = []; return }
  loadingAssets.value = true
  try {
    const d = await assetsApi.list()
    const rows = Array.isArray(d) ? d : (d?.data || [])
    projectAssets.value = rows.filter(a => prNumbers.some(n => (a.notes || '').includes(n)))
  } catch { projectAssets.value = [] } finally { loadingAssets.value = false }
}
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
    { label: 'RAB',         value: p.budget,      cls: 'text-gray-900',
      hint: rabSet.value ? `v${p.rab_version} · ${p.rab_item_count} baris` : (p.rab_item_count ? `${p.rab_item_count} baris, belum ditetapkan` : 'belum disusun') },
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
    rabItems.value = res?.rab_items ?? []
    requests.value = res?.requests ?? []
    // Aset hasil projek ditelusuri lewat nomor pengajuan yang tercatat di
    // catatan aset ("Pengadaan: <nomor>") saat serah terima.
    await loadProjectAssets(requests.value.map(r => r.request_number).filter(Boolean))
  } catch (e) {
    errorMsg.value = e.message || 'Gagal memuat projek'
  } finally {
    loading.value = false
  }
}

onMounted(fetchDetail)
</script>
