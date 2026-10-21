<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h1 class="text-xl font-bold text-gray-900">Operasional Keuangan</h1><p class="mt-1 text-sm text-gray-500">Pemasukan, pengeluaran, dan pencatatan otomatis dari kegiatan usaha.</p></div>
      <div v-if="can('create')" class="flex flex-wrap gap-2"><AppButton @click="openCreate('in')">+ Pemasukan</AppButton><AppButton variant="secondary" @click="openCreate('out')">+ Pengeluaran</AppButton></div>
    </div>
    <AppAlert type="error" :message="error" />
    <AppCard>
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div><label class="field-label">Periode transaksi</label><DateRangePicker v-model="range" /></div>
        <label><span class="field-label">Outlet</span><SearchableSelect v-model="filters.outlet_id" class="field"><option value="">Semua outlet yang diizinkan</option><option v-for="o in lookups.outlets" :key="o.id" :value="o.id">{{ o.name }}</option></SearchableSelect></label>
        <label><span class="field-label">Cari catatan / nomor / penerima</span><input v-model="filters.search" class="field" @keydown.enter="reload" /></label>
        <div class="flex items-end"><AppButton :loading="loading" @click="reload">Tampilkan</AppButton></div>
      </div>
    </AppCard>
    <div class="flex gap-2 border-b border-gray-200 pb-2"><AppButton :variant="tab === 'operations' ? 'primary' : 'secondary'" @click="tab = 'operations'">Semua Operasional</AppButton><AppButton :variant="tab === 'entries' ? 'primary' : 'secondary'" @click="tab = 'entries'">Transaksi Manual & Draf</AppButton></div>
    <template v-if="tab === 'operations'">
      <div class="grid gap-3 sm:grid-cols-3"><SummaryCard label="Kas masuk" :value="formatRupiah(operations.cash_in)" icon="revenue" /><SummaryCard label="Kas keluar" :value="formatRupiah(operations.cash_out)" icon="payment" /><SummaryCard label="Perlu ditinjau" :value="String(operations.review_count || 0)" icon="payment" /></div>
      <AppCard>
        <div class="mb-4 grid gap-3 sm:grid-cols-3">
          <label><span class="field-label">Sumber / status tinjauan</span><SearchableSelect v-model="filters.source" class="field"><option value="">Semua sumber</option><option value="review">Perlu ditinjau (semua sumber)</option><option v-for="(name, code) in sources" :key="code" :value="code">{{ name }}</option></SearchableSelect></label>
          <label><span class="field-label">Jenis pencatatan</span><SearchableSelect v-model="filters.direction" class="field"><option value="">Semua jenis</option><option value="in">Kas masuk</option><option value="out">Kas keluar</option><option value="noncash">Nonkas</option></SearchableSelect></label>
          <label><span class="field-label">Akun</span><SearchableSelect v-model="filters.account" class="field"><option value="">Semua akun</option><option v-for="a in lookups.accounts" :key="a.code" :value="a.code">{{ a.code }} · {{ a.name }}</option></SearchableSelect></label>
        </div>
        <p class="mb-3 text-xs text-gray-500">Transaksi operasional otomatis dibaca dari dokumen asal. Draf manual masuk laporan setelah dibukukan. Pencatatan nonkas tidak menambah pemasukan/pengeluaran kas.</p>
        <div v-if="loading" class="p-6 text-center"><AppSpinner /></div>
        <div v-else-if="!operations.rows.length" class="p-6 text-center text-sm text-gray-500">Belum ada transaksi untuk filter ini.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in operations.rows" :key="row.id" class="flex flex-wrap items-start gap-3 py-4">
            <div class="min-w-0 flex-1"><p class="text-xs text-gray-500">{{ row.date }} · {{ sources[row.source] || row.source }} · {{ row.reference || '—' }}</p><p class="mt-1 break-words font-semibold text-gray-800">{{ row.description }}</p><p class="mt-1 text-xs text-gray-500">{{ row.outlet_name }}<span v-if="row.work_unit_name"> / {{ row.work_unit_name }}</span></p><p v-if="row.notes" class="mt-1 break-words text-sm text-gray-600">{{ row.notes }}</p><p v-if="row.review_note" class="mt-1 text-xs text-amber-700">{{ row.review_note }}</p><p class="mt-1 text-xs text-gray-500">{{ [...new Set(row.postings.map(p => p.account_code + ' ' + p.account_name))].join(' · ') }}</p></div>
            <div class="text-right text-sm"><p v-if="row.cash_in" class="font-semibold text-emerald-700">+ {{ formatRupiah(row.cash_in) }}</p><p v-if="row.cash_out" class="font-semibold text-red-600">− {{ formatRupiah(row.cash_out) }}</p><p v-if="!row.cash_in && !row.cash_out" class="text-gray-500">Nonkas</p></div>
            <AppActionButton icon="view" label="Detail pencatatan" @click="openOperation(row)" />
          </li>
        </ul>
        <AppPagination :page="page" :total="operations.total" :per-page="20" @change="changePage" />
      </AppCard>
    </template>
    <AppCard v-else>
      <p class="mb-4 text-sm text-gray-500">Gaji dicatat total per outlet/divisi dan periode. Dokumen yang sudah dibukukan dikoreksi melalui pembalikan dengan alasan, sehingga riwayat tetap tersedia.</p>
      <div v-if="loading" class="p-6 text-center"><AppSpinner /></div>
      <p v-else-if="!visibleEntries.length" class="p-6 text-center text-sm text-gray-500">Belum ada transaksi manual.</p>
      <ul v-else class="divide-y divide-gray-100"><li v-for="row in visibleEntries" :key="row.id" class="flex flex-wrap items-center gap-3 py-4">
        <div class="min-w-0 flex-1"><p class="text-xs text-gray-500">{{ row.number }} · {{ row.occurred_at.replace('T', ' ') }}</p><p class="mt-1 break-words font-semibold">{{ row.description }}</p><p class="mt-1 text-xs text-gray-500">{{ row.outlet_name }} / {{ row.work_unit_name || '—' }} · {{ accountName(row.account_code) }}</p><p class="mt-1 text-xs" :class="row.status === 'posted' ? 'text-emerald-700' : 'text-gray-500'">{{ statusName(row.status) }}<span v-if="row.period"> · Periode {{ row.period }}</span></p></div>
        <p class="text-sm font-semibold" :class="row.direction === 'in' ? 'text-emerald-700' : 'text-red-600'">{{ row.direction === 'in' ? '+' : '−' }} {{ formatRupiah(row.amount) }}</p>
        <div class="flex flex-wrap gap-1"><AppActionButton icon="view" label="Detail transaksi" @click="openDetail(row)" /><AppActionButton v-if="can('create') && row.status === 'draft'" icon="edit" label="Edit draf" @click="openEdit(row)" /><AppActionButton v-if="can('post') && row.status === 'draft'" icon="approve" label="Bukukan transaksi" @click="openAction(row, 'post')" /><AppActionButton v-if="can('void') && row.status !== 'voided'" icon="cancel" :label="row.status === 'posted' ? 'Balik transaksi' : 'Batalkan draf'" @click="openAction(row, 'void')" /></div>
      </li></ul>
    </AppCard>

    <AppModal v-model="showForm" :title="editing ? 'Edit Draf Keuangan' : form.direction === 'in' ? 'Catat Pemasukan' : 'Catat Pengeluaran'" size="2xl">
      <form id="finance-form" class="space-y-4" @submit.prevent="save">
        <AppAlert type="error" :message="formError" />
        <div class="grid gap-4 sm:grid-cols-2">
          <label><span class="field-label">Akun *</span><SearchableSelect v-model="form.account_code" required class="field"><option value="" disabled>Pilih akun</option><option v-for="a in entryAccounts" :key="a.code" :value="a.code">{{ a.code }} · {{ a.name }}</option></SearchableSelect></label>
          <label><span class="field-label">Nominal (Rp) *</span><input v-model.number="form.amount" type="number" min="0.01" max="1000000000000" step="0.01" required class="field" /></label>
          <label><span class="field-label">Tanggal transaksi *</span><input v-model="form.occurred_at" type="date" :max="today" required class="field" /></label>
          <label><span class="field-label">Periode {{ form.account_code === '5-310' ? 'gaji *' : '(opsional)' }}</span><input v-model="form.period" type="month" :required="form.account_code === '5-310'" class="field" /></label>
          <label><span class="field-label">Outlet *</span><SearchableSelect v-model="form.outlet_id" class="field" @change="form.work_unit_id = ''"><option v-if="lookups.allow_central" value="">Pusat / Divisi</option><option v-else value="" disabled>Pilih outlet</option><option v-for="o in lookups.outlets" :key="o.id" :value="o.id">{{ o.name }}</option></SearchableSelect></label>
          <label><span class="field-label">Divisi / unit kerja</span><SearchableSelect v-model="form.work_unit_id" class="field"><option value="">Tanpa unit kerja</option><option v-for="w in entryUnits" :key="w.id" :value="w.id">{{ w.name }}</option></SearchableSelect></label>
          <label><span class="field-label">{{ form.direction === 'in' ? 'Diterima dari' : 'Dibayarkan kepada' }} *</span><input v-model="form.counterparty" maxlength="200" required class="field" placeholder="Contoh: Tim Outlet A" /></label>
          <label><span class="field-label">Referensi / nomor bukti</span><input v-model="form.reference" maxlength="200" class="field" placeholder="Nomor transfer / dokumen" /></label>
          <label><span class="field-label">Metode *</span><SearchableSelect v-model="form.method" class="field"><option value="cash">Kas tunai</option><option value="transfer">Transfer bank</option><option value="qris">QRIS</option></SearchableSelect></label>
          <label v-if="form.method !== 'cash'"><span class="field-label">Rekening perusahaan *</span><SearchableSelect v-model="form.bank_account_id" required class="field"><option value="" disabled>Pilih rekening</option><option v-for="b in activeBanks" :key="b.id" :value="b.id">{{ b.bank_name }} · {{ b.account_number }} · {{ b.account_holder }}</option></SearchableSelect></label>
        </div>
        <label class="block"><span class="field-label">Uraian *</span><input v-model="form.description" required maxlength="300" class="field" placeholder="Contoh: Gaji tim outlet bulan Oktober 2026" /></label>
        <label class="block"><span class="field-label">Catatan rinci</span><textarea v-model="form.notes" rows="3" maxlength="10000" class="field" placeholder="Rincian keperluan, jumlah staf, atau penjelasan pendukung" /></label>
        <label class="block"><span class="field-label">Bukti transaksi (wajib saat dibukukan)</span><input type="file" accept="image/jpeg,image/png,image/webp,application/pdf" :disabled="uploading" @change="upload" /><span v-if="uploading" class="text-sm text-gray-500">Mengunggah…</span></label>
        <a v-if="safeUrl(form.proof_url)" :href="safeUrl(form.proof_url)" target="_blank" rel="noopener" class="text-sm text-emerald-700 underline">Lihat bukti terunggah</a>
      </form>
      <template #footer><AppButton variant="secondary" @click="showForm = false">Batal</AppButton><AppButton type="submit" form="finance-form" :loading="saving" :disabled="uploading">Simpan Draf</AppButton></template>
    </AppModal>

    <AppModal v-model="showAction" :title="action === 'post' ? 'Bukukan Transaksi' : 'Batalkan / Balik Transaksi'">
      <AppAlert type="error" :message="actionError" />
      <p class="text-sm">{{ actionTarget?.number }} · {{ actionTarget?.description }} · <strong>{{ formatRupiah(actionTarget?.amount || 0) }}</strong></p>
      <p class="mt-3 text-sm text-gray-600">{{ action === 'post' ? 'Periksa akun, nominal, periode, dan bukti. Setelah dibukukan, transaksi masuk laporan keuangan.' : actionTarget?.status === 'posted' ? 'Jurnal pembalikan dicatat pada tanggal hari ini. Transaksi asal dan riwayatnya tetap tersimpan.' : 'Draf dibatalkan dan tidak masuk laporan keuangan.' }}</p>
      <label v-if="action === 'void'" class="mt-4 block"><span class="field-label">Alasan *</span><textarea v-model="reason" class="field" rows="3" /></label>
      <template #footer><AppButton variant="secondary" @click="showAction = false">Kembali</AppButton><AppButton :loading="saving" :disabled="action === 'void' && !reason.trim()" @click="act">{{ action === 'post' ? 'Konfirmasi Pembukuan' : 'Konfirmasi Pembatalan' }}</AppButton></template>
    </AppModal>
    <AppModal v-model="showDetail" title="Detail Transaksi Keuangan" size="xl">
      <div v-if="detail" class="space-y-3 text-sm"><p class="font-semibold">{{ detail.number }} · {{ statusName(detail.status) }}</p><p>{{ detail.description }}</p><p>{{ accountName(detail.account_code) }} · {{ formatRupiah(detail.amount) }}</p><p>{{ detail.outlet_name }} / {{ detail.work_unit_name || '—' }} · {{ detail.counterparty }}</p><p>Tanggal {{ detail.occurred_at.replace('T', ' ') }} · Periode {{ detail.period || '—' }}</p><p>{{ detail.method }} · {{ detail.bank_label || 'Kas tunai' }} · Referensi {{ detail.reference || '—' }}</p><p class="whitespace-pre-wrap">{{ detail.notes || 'Tidak ada catatan tambahan.' }}</p><a v-if="safeUrl(detail.proof_url)" :href="safeUrl(detail.proof_url)" target="_blank" rel="noopener" class="text-emerald-700 underline">Lihat bukti transaksi</a><h3 class="font-semibold pt-3">Riwayat</h3><ul class="space-y-2"><li v-for="(h, i) in detail.history" :key="i">{{ formatStamp(h.at) }} · {{ historyName(h.action) }} · {{ h.actor }}<p v-if="h.notes" class="text-gray-500 whitespace-pre-wrap">{{ h.notes }}</p></li></ul></div>
    </AppModal>
    <AppModal v-model="showOperation" title="Detail Pencatatan Operasional" size="xl">
      <div v-if="operation" class="space-y-3 text-sm"><p class="font-semibold">{{ operation.description }}</p><p>{{ sources[operation.source] }} · {{ operation.reference || '—' }} · {{ operation.outlet_name }}</p><p>{{ formatStamp(operation.at) }} · {{ operation.actor || 'Sistem' }} · {{ operation.counterparty || '—' }}</p><p class="whitespace-pre-wrap">{{ operation.notes || 'Tidak ada catatan tambahan dari sumber.' }}</p><p v-if="operation.review_note" class="text-amber-700">{{ operation.review_note }}</p><a v-if="safeUrl(operation.proof_url)" :href="safeUrl(operation.proof_url)" target="_blank" rel="noopener" class="text-emerald-700 underline">Lihat bukti</a><div class="overflow-x-auto"><table class="w-full text-sm"><thead><tr><th scope="col" class="text-left py-2">Akun</th><th scope="col" class="text-right">Debit</th><th scope="col" class="text-right">Kredit</th></tr></thead><tbody><tr v-for="(p, i) in operation.postings" :key="i" class="border-t border-gray-100"><td class="py-2">{{ p.account_code }} · {{ p.account_name }}</td><td class="text-right whitespace-nowrap">{{ formatRupiah(p.debit) }}</td><td class="text-right whitespace-nowrap">{{ formatRupiah(p.credit) }}</td></tr></tbody></table></div></div>
      <div v-if="operation?.source === 'pos_cash' && can('post')" class="mt-5 space-y-3 border-t border-gray-100 pt-4">
        <h3 class="text-sm font-semibold">Klasifikasi Akun Kas POS</h3><AppAlert type="error" :message="classificationError" />
        <label><span class="field-label">Akun sesuai keperluan</span><SearchableSelect v-model="classificationAccount" class="field"><option value="" disabled>Pilih akun</option><option v-for="a in classificationAccounts" :key="a.code" :value="a.code">{{ a.code }} · {{ a.name }}</option></SearchableSelect></label>
        <label class="block"><span class="field-label">Alasan / catatan klasifikasi</span><textarea v-model="classificationNotes" rows="2" class="field" /></label>
        <AppButton :loading="saving" :disabled="!classificationAccount || !classificationNotes.trim()" @click="classify">Simpan Klasifikasi</AppButton>
      </div>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { financeApi } from '@/api/finance'
import { apiClient } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { formatRupiah, todayDateString } from '@/utils/format'
import { safeUrl } from '@/utils/safeUrl'
import AppCard from '@/components/ui/AppCard.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import SummaryCard from '@/components/SummaryCard.vue'

const auth = useAuthStore(), toast = useToastStore(), can = action => auth.hasPermission('finance.entries.' + action)
const today = todayDateString(), range = ref({ from: today.slice(0, 8) + '01', to: today, label: 'Bulan Ini' })
const filters = reactive({ outlet_id: '', source: '', direction: '', account: '', search: '' })
const lookups = ref({ accounts: [], outlets: [], work_units: [], banks: [], allow_central: false })
const operations = ref({ rows: [], total: 0, cash_in: 0, cash_out: 0, review_count: 0 }), entries = ref([])
const tab = ref('operations'), page = ref(1), loading = ref(false), error = ref('')
const sources = { sale: 'Penjualan POS', pos_cash: 'Kas outlet POS', procurement_accrual: 'Pengakuan pengadaan', procurement_payment: 'Pembayaran pengadaan', procurement_consumed: 'Penerimaan barang habis pakai', receivable: 'Piutang pelanggan', reservation_payment: 'Pembayaran reservasi', reservation_forfeit: 'DP hangus', reservation_usage: 'Pemakaian DP', reservation_pending: 'DP menunggu sinkronisasi', stock: 'Mutasi persediaan', stock_quality: 'Mutu stok / refund supplier', stock_reject: 'Reject penerimaan', stock_transfer_settlement: 'Penyelesaian pengiriman', stock_return_dispatch: 'Kirim retur', stock_return_receipt: 'Terima retur', stock_return_settlement: 'Penyelesaian retur', maintenance: 'Perawatan aset', asset_opening: 'Aset awal', asset_disposal: 'Pelepasan aset', project_asset_release: 'Alokasi biaya projek ke aset', project_asset_receipt: 'Aset hasil projek', depreciation: 'Penyusutan aset', manual: 'Transaksi manual', manual_reversal: 'Pembalikan transaksi' }
const accountName = code => lookups.value.accounts.find(a => a.code === code)?.name || code
const statusName = code => ({ draft: 'Draf', posted: 'Dibukukan', voided: 'Dibatalkan' })[code] || code
const historyName = code => ({ created: 'Draf dibuat', updated: 'Draf diubah', post: 'Dibukukan', void: 'Dibatalkan / dibalik' })[code] || code
const formatStamp = stamp => stamp ? new Date(stamp).toLocaleString('id-ID') : '—'
const visibleEntries = computed(() => entries.value.filter(r => !filters.search || [r.number, r.description, r.counterparty, r.notes].join(' ').toLowerCase().includes(filters.search.toLowerCase())))
let request = 0
async function load() {
  const current = ++request; loading.value = true; error.value = ''
  try {
    const params = { date_from: range.value.from, date_to: range.value.to, ...filters, page: page.value, limit: 20 }
    const [ops, docs] = await Promise.all([financeApi.operations(params), financeApi.entries(params)])
    if (current === request) { operations.value = ops; entries.value = docs }
  } catch (e) { if (current === request) { error.value = e.message; operations.value = { rows: [], total: 0 }; entries.value = [] } }
  finally { if (current === request) loading.value = false }
}
function reload() { page.value = 1; load() }
function changePage(p) { page.value = p; load() }
watch([range, () => filters.outlet_id, () => filters.source, () => filters.direction, () => filters.account], reload)
onMounted(async () => { try { lookups.value = await financeApi.lookups(); await load() } catch (e) { error.value = e.message } })
const showForm = ref(false), editing = ref(''), form = reactive({}), formError = ref(''), saving = ref(false), uploading = ref(false)
const entryAccounts = computed(() => lookups.value.accounts.filter(a => a.direction === form.direction || a.direction === 'both'))
const entryUnits = computed(() => lookups.value.work_units.filter(w => (w.outlet_id || '') === (form.outlet_id || '')))
const activeBanks = computed(() => lookups.value.banks.filter(b => b.is_active))
function openCreate(direction) {
  editing.value = ''; formError.value = ''
  Object.assign(form, { request_key: crypto.randomUUID(), direction, account_code: '', outlet_id: filters.outlet_id || '', work_unit_id: '', amount: '', occurred_at: today, period: '', counterparty: '', description: '', notes: '', reference: '', method: 'cash', bank_account_id: '', proof_url: '', version: 1 })
  showForm.value = true
}
function openEdit(row) { editing.value = row.id; Object.assign(form, row, { occurred_at: row.occurred_at.slice(0, 10) }); formError.value = ''; showForm.value = true }
async function save() {
  if (saving.value || uploading.value) return
  saving.value = true; formError.value = ''
  try { if (editing.value) await financeApi.update(editing.value, { ...form }); else await financeApi.create({ ...form }); showForm.value = false; tab.value = 'entries'; toast.success('Draf transaksi tersimpan'); await load() }
  catch (e) { formError.value = e.message } finally { saving.value = false }
}
async function upload(e) {
  const file = e.target.files?.[0]; if (!file) return
  if (file.size > 5 * 1024 * 1024) { formError.value = 'Bukti maksimal 5 MB'; return }
  uploading.value = true; formError.value = ''
  try { const body = new FormData(); body.append('file', file); const data = await apiClient.post('/admin/upload', body, { headers: { 'Content-Type': 'multipart/form-data' } }); const url = safeUrl(data.url); if (!url) throw new Error('Alamat bukti tidak valid'); form.proof_url = url }
  catch (e) { formError.value = e.message } finally { uploading.value = false; e.target.value = '' }
}
const showDetail = ref(false), detail = ref(null), showOperation = ref(false), operation = ref(null)
async function openDetail(row) { try { detail.value = await financeApi.detail(row.id); showDetail.value = true } catch (e) { error.value = e.message } }
const classificationAccount = ref(''), classificationNotes = ref(''), classificationError = ref('')
const classificationAccounts = computed(() => lookups.value.accounts.filter(a => a.direction === (operation.value?.cash_in ? 'in' : 'out') || a.direction === 'both'))
function openOperation(row) { operation.value = row; classificationAccount.value = ''; classificationNotes.value = ''; classificationError.value = ''; showOperation.value = true }
async function classify() {
  if (saving.value) return
  saving.value = true; classificationError.value = ''
  try { await financeApi.classify(operation.value.source_id, { account: classificationAccount.value, notes: classificationNotes.value }); showOperation.value = false; toast.success('Klasifikasi akun tersimpan'); await load() }
  catch (e) { classificationError.value = e.message } finally { saving.value = false }
}
const showAction = ref(false), action = ref(''), actionTarget = ref(null), reason = ref(''), actionError = ref('')
function openAction(row, kind) { actionTarget.value = row; action.value = kind; reason.value = ''; actionError.value = ''; showAction.value = true }
async function act() {
  if (saving.value) return
  saving.value = true; actionError.value = ''
  try { await financeApi.action(actionTarget.value.id, { action: action.value, reason: reason.value, version: actionTarget.value.version }); showAction.value = false; toast.success(action.value === 'post' ? 'Transaksi dibukukan' : 'Pembatalan tersimpan'); await load() }
  catch (e) { actionError.value = e.message } finally { saving.value = false }
}
</script>
<style scoped>
.field-label { display: block; margin-bottom: .35rem; font-size: .8rem; font-weight: 600; color: #4b5563; }
.field { width: 100%; min-width: 0; padding: .55rem .7rem; border: 1px solid #d1d5db; border-radius: .5rem; background: white; font-size: .875rem; }
.field:focus { outline: 2px solid #059669; outline-offset: 1px; }
</style>
