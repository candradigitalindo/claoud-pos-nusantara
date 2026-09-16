<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Mutasi Aset Antar Outlet</h1>
        <p class="mt-0.5 text-sm text-gray-500">Perpindahan aset dengan persetujuan, bukti kirim, dan bukti terima.</p>
      </div>
      <AppButton v-if="canCreate" class="w-full sm:w-auto" @click="openCreate">+ Buat Mutasi</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Tab status -->
    <div class="-mx-1 overflow-x-auto">
      <div class="flex gap-1 px-1 pb-1">
        <button v-for="t in STATUS_TABS" :key="t.key" @click="filterStatus = t.key; load()"
          class="shrink-0 rounded-lg px-3 py-2 text-xs font-semibold transition"
          :class="filterStatus === t.key ? 'bg-emerald-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'">
          {{ t.label }}
        </button>
      </div>
    </div>

    <AppCard :padding="false">
      <!-- Mobile -->
      <div class="sm:hidden">
        <div v-if="loading" class="space-y-3 p-4">
          <div v-for="i in 3" :key="i" class="animate-pulse space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200"></div><div class="h-3 w-1/2 rounded bg-gray-100"></div>
          </div>
        </div>
        <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">{{ emptyText }}</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="t in rows" :key="t.id" class="p-4">
            <button class="w-full text-left" @click="openDetail(t)">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <p class="font-mono text-[11px] text-gray-500">{{ t.transfer_number }}</p>
                  <p class="mt-0.5 break-words text-sm font-semibold text-gray-900">
                    {{ t.from_outlet_name }} → {{ t.to_outlet_name }}
                  </p>
                </div>
                <div class="flex shrink-0 flex-col items-end gap-1">
                  <span :class="trStatusCls(t.status)">{{ trStatusLabel(t.status) }}</span>
                  <span v-if="t.has_shortfall" class="badge-warn">Selisih</span>
                </div>
              </div>
              <dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
                <dt class="text-gray-400">Aset</dt><dd class="text-gray-700">{{ t.item_count }} jenis · {{ t.total_qty }} unit</dd>
                <dt class="text-gray-400">Alasan</dt><dd class="text-gray-700">{{ TRANSFER_REASONS[t.reason] || t.reason || '—' }}</dd>
                <dt class="text-gray-400">Dibuat</dt><dd class="text-gray-700">{{ t.created_at }}<span v-if="t.created_by"> · {{ t.created_by }}</span></dd>
              </dl>
            </button>
          </li>
        </ul>
      </div>

      <!-- Desktop -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="rows" :loading="loading" :emptyText="emptyText">
        <template #cell-transfer_number="{ row }">
          <button class="text-left" @click="openDetail(row)">
            <p class="font-mono text-xs font-semibold text-gray-900 hover:text-emerald-700">{{ row.transfer_number }}</p>
            <p class="text-[11px] text-gray-400">{{ row.created_at }}</p>
          </button>
        </template>
        <template #cell-route="{ row }">
          <span class="text-sm text-gray-800">{{ row.from_outlet_name }}</span>
          <span class="mx-1 text-gray-400">→</span>
          <span class="text-sm text-gray-800">{{ row.to_outlet_name }}</span>
        </template>
        <template #cell-items="{ row }">{{ row.item_count }} jenis · {{ row.total_qty }} unit</template>
        <template #cell-reason="{ row }">{{ TRANSFER_REASONS[row.reason] || row.reason || '—' }}</template>
        <template #cell-status="{ row }">
          <div class="flex flex-col items-start gap-1">
            <span :class="trStatusCls(row.status)">{{ trStatusLabel(row.status) }}</span>
            <span v-if="row.has_shortfall" class="badge-warn">Selisih</span>
          </div>
        </template>
        <template #cell-actions="{ row }">
          <button class="rounded px-2 py-1 text-xs font-medium text-emerald-600 hover:bg-emerald-50" @click="openDetail(row)">Buka</button>
        </template>
      </AppTable>
    </AppCard>

    <!-- ── Modal buat mutasi ── -->
    <AppModal v-model="createModal" title="Buat Mutasi Aset" size="2xl">
      <form class="space-y-3" @submit.prevent="saveDraft">
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Outlet Asal <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form.from_outlet_id" :options="outlets" placeholder="Pilih outlet asal…" @change="loadAvailable" />
          </div>
          <div>
            <label class="lbl">Outlet Tujuan <span class="text-red-500">*</span></label>
            <SearchSelect v-model="form.to_outlet_id" :options="destinationOptions" placeholder="Pilih outlet tujuan…" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Alasan</label>
            <select v-model="form.reason" class="form-input" @change="loadAvailable">
              <option v-for="(lbl, key) in TRANSFER_REASONS" :key="key" :value="key">{{ lbl }}</option>
            </select>
          </div>
          <div v-if="form.reason === 'pinjam' || form.reason === 'perbaikan'">
            <label class="lbl">Rencana Kembali</label>
            <input v-model="form.expected_return" type="date" class="form-input" />
          </div>
        </div>

        <div>
          <label class="lbl">Aset yang Dimutasi <span class="text-red-500">*</span></label>
          <p v-if="!form.from_outlet_id" class="rounded-lg bg-gray-50 p-3 text-xs text-gray-500">
            Pilih outlet asal dulu untuk melihat aset yang bisa dimutasi.
          </p>
          <p v-else-if="loadingAvail" class="p-3 text-xs text-gray-400">Memuat aset…</p>
          <p v-else-if="!available.length" class="rounded-lg bg-amber-50 p-3 text-xs text-amber-700">
            Tidak ada aset yang bisa dimutasi dari outlet ini. Aset yang sedang dalam perjalanan atau sudah
            tercantum di dokumen mutasi lain tidak ditawarkan.
          </p>
          <ul v-else class="max-h-72 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-2">
            <li v-for="a in available" :key="a.id" class="rounded-lg p-2" :class="picked[a.id] ? 'bg-emerald-50' : 'hover:bg-gray-50'">
              <div class="flex items-start gap-2">
                <input type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-emerald-600"
                  :checked="!!picked[a.id]" @change="togglePick(a, $event.target.checked)" :aria-label="`Pilih ${a.name}`" />
                <div class="min-w-0 flex-1">
                  <p class="break-words text-sm font-medium text-gray-900">{{ a.name }}</p>
                  <p class="font-mono text-[11px] text-gray-500">{{ a.asset_no }} · tersedia {{ a.quantity }} {{ a.unit }}</p>
                </div>
                <div v-if="picked[a.id]" class="shrink-0">
                  <input v-model.number="picked[a.id].qty" type="number" inputmode="numeric" min="1" :max="a.quantity"
                    class="form-input w-20 text-right" :disabled="a.tracking_mode === 'tunggal'" />
                </div>
              </div>
            </li>
          </ul>
        </div>

        <div>
          <label class="lbl">Catatan</label>
          <textarea v-model="form.notes" rows="2" class="form-input" placeholder="Opsional"></textarea>
        </div>

        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="createModal = false">Batal</button>
          <AppButton type="submit" :loading="saving">Simpan Draft</AppButton>
        </div>
      </form>
    </AppModal>

    <!-- ── Modal detail ── -->
    <AppModal v-model="detailModal" :title="detail ? `Mutasi ${detail.transfer_number}` : 'Mutasi'" size="2xl">
      <div v-if="detail" class="space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <span :class="trStatusCls(detail.status)">{{ trStatusLabel(detail.status) }}</span>
          <span v-if="detail.has_shortfall" class="badge-warn">Ada selisih penerimaan</span>
          <span class="text-sm text-gray-700">{{ detail.from_outlet_name }} → {{ detail.to_outlet_name }}</span>
        </div>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
          <div class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">Alasan</dt><dd class="text-gray-800">{{ TRANSFER_REASONS[detail.reason] || detail.reason || '—' }}</dd>
          </div>
          <div v-if="detail.expected_return" class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">Rencana kembali</dt><dd class="text-gray-800">{{ detail.expected_return }}</dd>
          </div>
          <div v-for="j in jejak" :key="j.label" class="flex justify-between gap-2 border-b border-gray-50 py-1">
            <dt class="text-gray-400">{{ j.label }}</dt><dd class="min-w-0 break-words text-right text-gray-800">{{ j.value }}</dd>
          </div>
        </dl>

        <div v-if="detail.rejected_reason" class="rounded-lg bg-red-50 p-3 text-sm text-red-700">
          Ditolak: {{ detail.rejected_reason }}
        </div>
        <div v-if="detail.notes" class="rounded-lg bg-gray-50 p-3 text-sm text-gray-700">{{ detail.notes }}</div>

        <!-- Daftar aset -->
        <div class="overflow-hidden rounded-lg border border-gray-200">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500">
              <tr>
                <th class="p-2 text-left">Aset</th>
                <th class="p-2 text-right">Dikirim</th>
                <th class="p-2 text-right">{{ isReceiving ? 'Diterima' : 'Diterima' }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr v-for="it in detail.items" :key="it.id">
                <td class="p-2">
                  <p class="break-words font-medium text-gray-900">{{ it.asset_name }}</p>
                  <p class="font-mono text-[11px] text-gray-400">{{ it.asset_no }}</p>
                </td>
                <td class="p-2 text-right text-gray-700">{{ it.qty }} {{ it.unit }}</td>
                <td class="p-2 text-right">
                  <input v-if="isReceiving" v-model.number="receipt[it.id]" type="number" inputmode="numeric"
                    min="0" :max="it.qty" class="form-input w-20 text-right" />
                  <span v-else class="text-gray-700">{{ it.received_qty ?? '—' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="isReceiving" class="text-xs text-gray-500">
          Isi apa adanya. Unit yang tidak sampai otomatis dikembalikan ke {{ detail.from_outlet_name }} dan
          dokumen ditandai selisih.
        </p>
      </div>

      <template #footer>
        <div v-if="detail" class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center">
          <div class="flex flex-wrap gap-2 sm:mr-auto">
            <button v-if="detail.status === 'received'" class="btn-soft" @click="doPrint">Cetak Berita Acara</button>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn-ghost" @click="detailModal = false">Tutup</button>
            <template v-for="a in actionsFor(detail)" :key="a.act">
              <AppButton :variant="a.variant || 'primary'" :loading="acting === a.act" @click="runAction(a.act)">{{ a.label }}</AppButton>
            </template>
          </div>
        </div>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { assetTransfersApi } from '@/api/assetTransfers.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import { TRANSFER_REASONS, trStatusCls, trStatusLabel, printTransferNote } from '@/utils/assets.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canCreate = auth.hasPermission('assets.transfer.create')
const canApprove = auth.hasPermission('assets.transfer.approve')
const canReceive = auth.hasPermission('assets.transfer.receive')

const STATUS_TABS = [
  { key: '', label: 'Semua' },
  { key: 'draft', label: 'Draft' },
  { key: 'pending', label: 'Menunggu Persetujuan' },
  { key: 'approved', label: 'Disetujui' },
  { key: 'sent', label: 'Dalam Perjalanan' },
  { key: 'received', label: 'Selesai' },
]

const COLUMNS = [
  { key: 'transfer_number', label: 'Nomor' },
  { key: 'route', label: 'Perpindahan' },
  { key: 'items', label: 'Aset' },
  { key: 'reason', label: 'Alasan' },
  { key: 'status', label: 'Status' },
  { key: 'actions', label: '' },
]

const rows = ref([])
const outlets = ref([])
const loading = ref(false)
const errorMsg = ref('')
const filterStatus = ref('')

const emptyText = computed(() => filterStatus.value
  ? `Tidak ada mutasi berstatus "${trStatusLabel(filterStatus.value)}".`
  : (canCreate ? 'Belum ada mutasi aset. Buat lewat tombol di kanan atas.' : 'Belum ada mutasi aset.'))

function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function asObject(d) { return d?.data ?? d ?? null }

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    rows.value = asArray(await assetTransfersApi.list({ status: filterStatus.value || undefined }))
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat mutasi'
  } finally { loading.value = false }
}

async function loadOutlets() {
  try {
    const d = await outletsApi.myOutlets()
    outlets.value = d?.outlets ?? d ?? []
  } catch { outlets.value = [] }
}

// ── Buat draft ──
const createModal = ref(false)
const saving = ref(false)
const available = ref([])
const loadingAvail = ref(false)
const picked = ref({})
const form = ref(blankForm())

function blankForm() {
  return { from_outlet_id: '', to_outlet_id: '', reason: 'relokasi', expected_return: '', notes: '' }
}
const destinationOptions = computed(() => outlets.value.filter(o => o.id !== form.value.from_outlet_id))

function openCreate() {
  form.value = blankForm()
  picked.value = {}
  available.value = []
  if (outlets.value.length === 1) {
    form.value.from_outlet_id = outlets.value[0].id
    loadAvailable()
  }
  createModal.value = true
}

async function loadAvailable() {
  picked.value = {}
  available.value = []
  if (!form.value.from_outlet_id) return
  if (form.value.to_outlet_id === form.value.from_outlet_id) form.value.to_outlet_id = ''
  loadingAvail.value = true
  try {
    available.value = asArray(await assetTransfersApi.available(form.value.from_outlet_id, form.value.reason))
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat aset')
  } finally { loadingAvail.value = false }
}

function togglePick(a, checked) {
  const next = { ...picked.value }
  if (checked) next[a.id] = { qty: a.tracking_mode === 'tunggal' ? 1 : a.quantity, max: a.quantity }
  else delete next[a.id]
  picked.value = next
}

async function saveDraft() {
  const items = Object.entries(picked.value).map(([asset_id, v]) => ({ asset_id, qty: Number(v.qty) || 0 }))
  if (!form.value.from_outlet_id || !form.value.to_outlet_id) { toast.error('Outlet asal dan tujuan wajib dipilih'); return }
  if (!items.length) { toast.error('Pilih minimal satu aset'); return }
  const over = items.find(i => i.qty > (picked.value[i.asset_id]?.max ?? 0) || i.qty <= 0)
  if (over) { toast.error('Jumlah yang dimutasi melebihi yang tersedia'); return }
  saving.value = true
  try {
    await assetTransfersApi.create({ ...form.value, items })
    toast.success('Draft mutasi dibuat')
    createModal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

// ── Detail & aksi ──
const detailModal = ref(false)
const detail = ref(null)
const acting = ref('')
const receipt = ref({})
const isReceiving = computed(() => detail.value?.status === 'sent' && canReceive)

const jejak = computed(() => {
  const d = detail.value
  if (!d) return []
  const out = [{ label: 'Dibuat', value: [d.created_by, d.created_at].filter(Boolean).join(' · ') || '—' }]
  if (d.approved_by || d.approved_at) out.push({ label: 'Disetujui', value: [d.approved_by, d.approved_at].filter(Boolean).join(' · ') })
  if (d.sent_by || d.sent_at) out.push({ label: 'Dikirim', value: [d.sent_by, d.sent_at].filter(Boolean).join(' · ') })
  if (d.received_by || d.received_at) out.push({ label: 'Diterima', value: [d.received_by, d.received_at].filter(Boolean).join(' · ') })
  return out
})

async function openDetail(t) {
  detailModal.value = true
  detail.value = null
  try {
    detail.value = asObject(await assetTransfersApi.get(t.id))
    receipt.value = Object.fromEntries((detail.value.items || []).map(it => [it.id, it.received_qty ?? it.qty]))
  } catch (e) {
    toast.error(e?.message || 'Gagal memuat dokumen')
    detailModal.value = false
  }
}

// Tombol hanya muncul bila status DAN izin memungkinkan — bukan tampil lalu menolak.
function actionsFor(d) {
  const acts = []
  if (d.status === 'draft' && canCreate) acts.push({ act: 'submit', label: 'Ajukan' })
  if (d.status === 'pending' && canApprove) {
    acts.push({ act: 'reject', label: 'Tolak', variant: 'danger' })
    acts.push({ act: 'approve', label: 'Setujui' })
  }
  if (d.status === 'approved' && canCreate) acts.push({ act: 'send', label: 'Kirim Barang' })
  if (d.status === 'sent' && canReceive) acts.push({ act: 'receive', label: 'Terima Barang' })
  if (['draft', 'pending', 'approved'].includes(d.status) && canCreate) {
    acts.unshift({ act: 'cancel', label: 'Batalkan', variant: 'secondary' })
  }
  return acts
}

const CONFIRM = {
  send: 'Kirim barang ini sekarang? Jumlahnya akan berkurang dari outlet asal.',
  receive: 'Catat penerimaan ini? Aset akan tercatat di outlet tujuan.',
  cancel: 'Batalkan dokumen mutasi ini?',
}

async function runAction(act) {
  if (CONFIRM[act] && !window.confirm(CONFIRM[act])) return
  let reason = ''
  if (act === 'reject') {
    reason = window.prompt('Alasan penolakan:') || ''
    if (!reason.trim()) return
  }
  acting.value = act
  try {
    if (act === 'receive') {
      const items = (detail.value.items || []).map(it => ({ item_id: it.id, received_qty: Number(receipt.value[it.id] ?? it.qty) }))
      detail.value = asObject(await assetTransfersApi.receive(detail.value.id, items))
    } else if (act === 'reject') {
      detail.value = asObject(await assetTransfersApi.reject(detail.value.id, reason))
    } else {
      detail.value = asObject(await assetTransfersApi[act](detail.value.id))
    }
    toast.success('Dokumen diperbarui')
    await load()
  } catch (e) { toast.error(e?.message || 'Aksi gagal') } finally { acting.value = '' }
}

function doPrint() {
  const r = printTransferNote(detail.value)
  if (r?.blocked) toast.error('Jendela cetak diblokir browser. Izinkan pop-up untuk situs ini.')
}

onMounted(async () => { await loadOutlets(); await load() })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.form-input:disabled { background: #f9fafb; color: #6b7280; }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.btn-ghost {
  padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600;
  color: #374151; background: #f3f4f6; min-height: 40px;
}
.btn-ghost:hover { background: #e5e7eb; }
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.08); min-height: 40px;
}
.btn-soft:hover { background: #f3f4f6; }
</style>
