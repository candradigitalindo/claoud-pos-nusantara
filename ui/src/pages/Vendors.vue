<template>
  <div class="vendors-page space-y-6">
    <section class="vendors-hero relative overflow-hidden rounded-3xl p-5 md:p-7">
      <div class="vendors-hero__pattern" aria-hidden="true" />
      <div class="relative z-10 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-teal-100/90">Pengadaan</p>
          <h1 class="mt-1 text-2xl font-black text-white md:text-3xl">Vendor Directory</h1>
          <p class="mt-2 max-w-2xl text-sm text-teal-100/95">
            Pantau supplier aktif, kelengkapan data pembayaran, dan percepat proses belanja unit kerja dari satu halaman.
          </p>
        </div>
        <AppButton @click="openCreate" class="!bg-white !text-teal-700 hover:!bg-teal-50 !shadow-sm">+ Tambah Vendor</AppButton>
      </div>
    </section>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      <div class="metric-card metric-card--slate">
        <p class="metric-card__label">Total Vendor</p>
        <p class="metric-card__value">{{ summary.total }}</p>
        <p class="metric-card__hint">Semua supplier terdaftar</p>
      </div>
      <div class="metric-card metric-card--emerald">
        <p class="metric-card__label">Vendor Aktif</p>
        <p class="metric-card__value">{{ summary.active }}</p>
        <p class="metric-card__hint">{{ summary.inactive }} nonaktif</p>
      </div>
      <div class="metric-card metric-card--sky">
        <p class="metric-card__label">Rekening Lengkap</p>
        <p class="metric-card__value">{{ summary.withBank }}</p>
        <p class="metric-card__hint">Siap untuk proses pembayaran</p>
      </div>
      <div class="metric-card metric-card--amber">
        <p class="metric-card__label">Kontak Tersedia</p>
        <p class="metric-card__value">{{ summary.withContact }}</p>
        <p class="metric-card__hint">Telepon atau email tersedia</p>
      </div>
      <div class="metric-card metric-card--rose">
        <p class="metric-card__label">Tagihan Belum Bayar</p>
        <p class="metric-card__value">{{ formatRupiah(summary.totalUnpaid) }}</p>
        <p class="metric-card__hint">{{ summary.vendorsWithUnpaid }} vendor memiliki tagihan</p>
      </div>
    </div>

    <AppAlert type="info" message="Kelola daftar vendor/supplier untuk pengadaan barang dan jasa." />
    <AppAlert type="error" :message="errorMsg" />

    <AppCard>
      <div class="flex flex-col gap-4">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
          <div class="relative">
            <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" />
            </svg>
            <input
              v-model="search"
              type="search"
              placeholder="Cari vendor, email, telepon, atau bank..."
              class="w-full rounded-xl border border-slate-200 bg-white/90 py-2.5 pl-9 pr-3 text-sm text-slate-700 outline-none transition focus:border-teal-500 focus:ring-2 focus:ring-teal-100"
            />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              v-for="item in STATUS_FILTERS"
              :key="item.value"
              @click="statusFilter = item.value"
              class="rounded-full border px-3 py-1.5 text-xs font-semibold transition"
              :class="statusFilter === item.value
                ? 'border-teal-600 bg-teal-600 text-white shadow-sm'
                : 'border-slate-200 bg-white text-slate-600 hover:border-teal-200 hover:text-teal-700'"
            >
              {{ item.label }}
            </button>
            <button
              v-if="search || statusFilter !== 'all'"
              @click="resetFilter"
              class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-500 transition hover:text-slate-800"
            >
              Reset
            </button>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
          <p>
            Menampilkan <span class="font-semibold text-slate-700">{{ filteredVendors.length }}</span>
            dari <span class="font-semibold text-slate-700">{{ vendors.length }}</span> vendor
          </p>
          <p>
            {{ summary.withBankMissing }} vendor belum melengkapi rekening pembayaran
          </p>
        </div>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <AppTable :columns="COLUMNS" :rows="filteredVendors" :loading="loading" :emptyText="emptyStateText">
        <template #cell-name="{ row }">
          <div class="flex items-center gap-3">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-teal-500 to-cyan-500 text-xs font-bold text-white shadow-sm">
              {{ getInitial(row.name) }}
            </div>
            <div>
              <router-link :to="`/vendors/${row.id}`" class="font-semibold text-emerald-700 hover:text-emerald-900 hover:underline">{{ row.name }}</router-link>
              <div v-if="row.email" class="text-xs text-gray-400">{{ row.email }}</div>
              <div v-else class="text-xs text-gray-300 italic">Email belum diisi</div>
            </div>
          </div>
        </template>
        <template #cell-phone="{ row }">
          <span v-if="row.phone" class="text-gray-700">{{ row.phone }}</span>
          <span v-else class="text-gray-400 italic">—</span>
        </template>
        <template #cell-bank="{ row }">
          <div class="space-y-1 text-sm">
            <div v-if="row.bank_name" class="text-gray-700">{{ row.bank_name }}</div>
            <div v-if="row.account_number" class="text-xs text-gray-400">{{ row.account_number }} — {{ row.account_holder || 'Tanpa nama' }}</div>
            <span
              class="inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold"
              :class="hasBankInfo(row)
                ? 'bg-emerald-100 text-emerald-700'
                : 'bg-amber-100 text-amber-700'"
            >
              {{ hasBankInfo(row) ? 'Siap bayar' : 'Data rekening kurang' }}
            </span>
          </div>
        </template>
        <template #cell-unpaid="{ row }">
          <div v-if="row.unpaid_amount > 0" class="text-right">
            <span class="font-semibold text-rose-600">{{ formatRupiah(row.unpaid_amount) }}</span>
            <p class="mt-0.5 text-[11px] text-rose-400">Menunggu pembayaran</p>
          </div>
          <div v-else class="text-right">
            <span class="text-sm text-slate-400">—</span>
            <p class="mt-0.5 text-[11px] text-emerald-500">Lunas</p>
          </div>
        </template>
        <template #cell-is_active="{ row }">
          <span v-if="row.is_active" class="inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700">
            <span class="h-2 w-2 rounded-full bg-emerald-500"></span> Aktif
          </span>
          <span v-else class="inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-500">
            <span class="h-2 w-2 rounded-full bg-slate-400"></span> Nonaktif
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1.5">
            <router-link
              :to="`/vendors/${row.id}`"
              class="action-icon text-blue-600 hover:bg-blue-50 hover:text-blue-800"
              title="Lihat detail vendor"
              aria-label="Lihat detail vendor"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/></svg>
            </router-link>
            <AppActionButton icon="edit" label="Edit vendor"
              @click="openEdit(row)" />
            <AppActionButton :icon="row.is_active ? 'disable' : 'enable'" :label="row.is_active ? 'Nonaktifkan vendor' : 'Aktifkan vendor'"
              @click="toggleActive(row)" />
            <AppActionButton icon="delete" label="Hapus vendor"
              @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- Create/Edit Modal -->
    <AppModal v-model="showForm" :title="editTarget ? 'Edit Vendor' : 'Tambah Vendor Baru'" size="2xl">
      <div class="modal-form-grid">
        <!-- Section: Informasi Umum -->
        <div>
          <div class="flex items-center gap-2 mb-3">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-emerald-100 text-emerald-600">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
            </span>
            <h3 class="text-sm font-semibold text-gray-800">Informasi Umum</h3>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="md:col-span-2">
              <AppInput v-model="form.name" label="Nama Vendor *" placeholder="PT. Sumber Jaya" />
            </div>
            <AppInput v-model="form.phone" label="Telepon" placeholder="08123456789" />
            <AppInput v-model="form.email" label="Email" placeholder="info@vendor.com" />
            <div class="md:col-span-2">
              <AppInput v-model="form.address" label="Alamat" placeholder="Jl. Contoh No. 1, Kota, Provinsi" />
            </div>
          </div>
        </div>

        <!-- Section: Detail Pembayaran -->
        <div class="rounded-xl border border-gray-200 p-4">
          <div class="flex items-center gap-2 mb-3">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-blue-100 text-blue-600">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"/></svg>
            </span>
            <h3 class="text-sm font-semibold text-gray-800">Detail Pembayaran</h3>
          </div>
          <div class="bg-blue-50/60 rounded-xl p-4 border border-blue-100">
            <div class="grid grid-cols-1 gap-3">
              <AppInput v-model="form.bank_name" label="Nama Bank" placeholder="BCA, BRI, Mandiri, dll" />
              <AppInput v-model="form.account_number" label="Nomor Rekening" placeholder="1234567890" />
              <AppInput v-model="form.account_holder" label="Atas Nama Rekening" placeholder="PT. Sumber Jaya" />
            </div>
            <p class="text-xs text-blue-500 mt-3">Informasi rekening digunakan untuk proses pembayaran pengadaan. Kosongkan bila vendor dibayar tunai.</p>
            <p v-if="editTarget" class="text-xs text-blue-500 mt-1">Perubahan rekening dicatat (siapa dan kapan) dan diperingatkan di form bayar. Pembayaran lama tetap tercatat dengan rekening lama.</p>
          </div>
        </div>

        <!-- Section: Catatan -->
        <div class="modal-span-full border-t border-gray-200 pt-4">
          <div class="flex items-center gap-2 mb-3">
            <span class="flex items-center justify-center w-7 h-7 rounded-full bg-amber-100 text-amber-600">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
            </span>
            <h3 class="text-sm font-semibold text-gray-800">Catatan</h3>
          </div>
          <textarea v-model="form.notes" rows="3" placeholder="Catatan tentang vendor, kontrak, kesepakatan harga, dll..."
            class="w-full rounded-lg border-gray-300 shadow-sm text-sm focus:border-emerald-500 focus:ring-emerald-500" />
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showForm = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitForm">{{ editTarget ? 'Simpan Perubahan' : 'Tambah Vendor' }}</AppButton>
      </template>
    </AppModal>

  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { computed, ref, onMounted } from 'vue'
import Swal from 'sweetalert2'
import { vendorsApi } from '@/api/vendors.js'
import { formatRupiah } from '@/utils/format.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard   from '@/components/ui/AppCard.vue'
import AppTable  from '@/components/ui/AppTable.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import AppInput  from '@/components/ui/AppInput.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'

const loading  = ref(false)
const errorMsg = ref('')
const vendors  = ref([])
const search = ref('')
const statusFilter = ref('all')

const showForm   = ref(false)
const editTarget = ref(null)
const saving     = ref(false)
const form       = ref(emptyForm())

const STATUS_FILTERS = [
  { label: 'Semua', value: 'all' },
  { label: 'Aktif', value: 'active' },
  { label: 'Nonaktif', value: 'inactive' },
]

const COLUMNS = [
  { key: 'name',      label: 'Vendor' },
  { key: 'phone',     label: 'Telepon' },
  { key: 'bank',      label: 'Rekening' },
  { key: 'unpaid',    label: 'Tagihan Belum Bayar', align: 'right' },
  { key: 'is_active', label: 'Status', align: 'center' },
  { key: 'actions',   label: '', align: 'right' },
]

function emptyForm() {
  return { name: '', phone: '', email: '', address: '', notes: '', bank_name: '', account_number: '', account_holder: '' }
}

const filteredVendors = computed(() => {
  const q = search.value.trim().toLowerCase()
  return vendors.value.filter((v) => {
    const statusOk = statusFilter.value === 'all'
      || (statusFilter.value === 'active' && !!v.is_active)
      || (statusFilter.value === 'inactive' && !v.is_active)
    if (!statusOk) return false

    if (!q) return true
    const haystack = [
      v.name,
      v.email,
      v.phone,
      v.bank_name,
      v.account_number,
      v.account_holder,
    ].join(' ').toLowerCase()
    return haystack.includes(q)
  })
})

const summary = computed(() => {
  const total = vendors.value.length
  const active = vendors.value.filter(v => !!v.is_active).length
  const withBank = vendors.value.filter(v => hasBankInfo(v)).length
  const withContact = vendors.value.filter(v => !!(v.phone || v.email)).length
  const totalUnpaid = vendors.value.reduce((sum, v) => sum + (v.unpaid_amount || 0), 0)
  const vendorsWithUnpaid = vendors.value.filter(v => (v.unpaid_amount || 0) > 0).length
  return {
    total,
    active,
    inactive: total - active,
    withBank,
    withContact,
    withBankMissing: total - withBank,
    totalUnpaid,
    vendorsWithUnpaid,
  }
})

const emptyStateText = computed(() => {
  if (loading.value) return 'Memuat data vendor...'
  if (!vendors.value.length) return 'Belum ada vendor.'
  return 'Tidak ada vendor yang sesuai dengan pencarian/filter.'
})

onMounted(fetchVendors)

async function fetchVendors() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await vendorsApi.list()
    vendors.value = Array.isArray(data) ? data : (data?.data || [])
  } catch (err) {
    errorMsg.value = err?.message ?? 'Gagal memuat data.'
  } finally {
    loading.value = false
  }
}

function hasBankInfo(row) {
  return !!(row.bank_name && row.account_number)
}

function getInitial(name) {
  const text = (name || '').trim()
  return text ? text.charAt(0).toUpperCase() : 'V'
}

function resetFilter() {
  search.value = ''
  statusFilter.value = 'all'
}

function swalBase(options = {}) {
  return Swal.fire({
    background: '#ffffff',
    color: '#0f172a',
    confirmButtonColor: '#0f766e',
    cancelButtonColor: '#64748b',
    reverseButtons: true,
    ...options,
  })
}

function openCreate() {
  editTarget.value = null
  form.value = emptyForm()
  showForm.value = true
}

function openEdit(row) {
  editTarget.value = row
  form.value = {
    name: row.name,
    phone: row.phone || '',
    email: row.email || '',
    address: row.address || '',
    notes: row.notes || '',
    bank_name: row.bank_name || '',
    account_number: row.account_number || '',
    account_holder: row.account_holder || '',
  }
  showForm.value = true
}

async function submitForm() {
  if (!form.value.name.trim()) {
    await swalBase({
      icon: 'warning',
      title: 'Nama vendor wajib diisi',
      text: 'Lengkapi nama vendor sebelum menyimpan data.',
      confirmButtonText: 'Siap',
    })
    return
  }
  saving.value = true
  try {
    if (editTarget.value) {
      await vendorsApi.update(editTarget.value.id, form.value)
      await swalBase({
        icon: 'success',
        title: 'Vendor berhasil diperbarui',
        text: `${form.value.name} sudah diperbarui.`,
        showConfirmButton: false,
        timer: 1700,
      })
    } else {
      await vendorsApi.create(form.value)
      await swalBase({
        icon: 'success',
        title: 'Vendor berhasil ditambahkan',
        text: `${form.value.name} sudah masuk ke daftar vendor.`,
        showConfirmButton: false,
        timer: 1700,
      })
    }
    showForm.value = false
    fetchVendors()
  } catch (err) {
    await swalBase({
      icon: 'error',
      title: 'Gagal menyimpan vendor',
      text: err?.message ?? 'Terjadi kendala saat menyimpan data vendor.',
      confirmButtonText: 'Tutup',
    })
  } finally {
    saving.value = false
  }
}

async function toggleActive(row) {
  const willDeactivate = !!row.is_active
  const actionLabel = willDeactivate ? 'menonaktifkan' : 'mengaktifkan'
  const confirm = await swalBase({
    icon: 'question',
    title: `Yakin ${actionLabel} vendor ini?`,
    text: `${row.name} akan ${willDeactivate ? 'menjadi nonaktif' : 'aktif kembali'}.`,
    showCancelButton: true,
    confirmButtonText: willDeactivate ? 'Ya, nonaktifkan' : 'Ya, aktifkan',
    cancelButtonText: 'Batal',
  })
  if (!confirm.isConfirmed) return

  try {
    await vendorsApi.toggle(row.id)
    await swalBase({
      icon: 'success',
      title: `Vendor berhasil ${willDeactivate ? 'dinonaktifkan' : 'diaktifkan'}`,
      text: `${row.name} sudah ${willDeactivate ? 'nonaktif' : 'aktif'}.`,
      showConfirmButton: false,
      timer: 1600,
    })
    fetchVendors()
  } catch (err) {
    await swalBase({
      icon: 'error',
      title: 'Gagal mengubah status vendor',
      text: err?.message ?? 'Terjadi kendala saat mengubah status vendor.',
      confirmButtonText: 'Tutup',
    })
  }
}

async function confirmDelete(row) {
  const confirm = await swalBase({
    icon: 'warning',
    title: 'Hapus vendor ini?',
    text: `${row.name} akan dihapus permanen dan tidak bisa dikembalikan.`,
    showCancelButton: true,
    confirmButtonColor: '#dc2626',
    confirmButtonText: 'Ya, hapus',
    cancelButtonText: 'Batal',
  })
  if (!confirm.isConfirmed) return

  try {
    await vendorsApi.delete(row.id)
    await swalBase({
      icon: 'success',
      title: 'Vendor berhasil dihapus',
      text: `${row.name} sudah dihapus dari daftar vendor.`,
      showConfirmButton: false,
      timer: 1600,
    })
    fetchVendors()
  } catch (err) {
    await swalBase({
      icon: 'error',
      title: 'Gagal menghapus vendor',
      text: err?.message ?? 'Terjadi kendala saat menghapus vendor.',
      confirmButtonText: 'Tutup',
    })
  }
}
</script>

<style scoped>
.vendors-hero {
  background:
    radial-gradient(circle at 15% 20%, rgba(255, 255, 255, 0.22), transparent 38%),
    radial-gradient(circle at 90% 10%, rgba(125, 211, 252, 0.28), transparent 42%),
    linear-gradient(135deg, #0f766e 0%, #0f766e 20%, #0e7490 62%, #0369a1 100%);
  box-shadow:
    0 14px 36px rgba(15, 118, 110, 0.25),
    inset 0 1px 0 rgba(255, 255, 255, 0.32);
}

.vendors-hero__pattern {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(to right, rgba(255, 255, 255, 0.06) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(255, 255, 255, 0.06) 1px, transparent 1px);
  background-size: 26px 26px;
  mask-image: radial-gradient(circle at 70% 40%, rgba(0, 0, 0, 1), transparent 75%);
}

.metric-card {
  border-radius: 1rem;
  border: 1px solid rgba(226, 232, 240, 0.9);
  background: rgba(255, 255, 255, 0.88);
  padding: 0.9rem 1rem;
  box-shadow: 0 6px 20px rgba(15, 23, 42, 0.06);
}

.metric-card__label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.metric-card__value {
  margin-top: 0.3rem;
  font-size: 1.55rem;
  line-height: 1;
  font-weight: 800;
}

.metric-card__hint {
  margin-top: 0.35rem;
  font-size: 0.75rem;
  color: rgb(100 116 139);
}

.metric-card--slate .metric-card__label,
.metric-card--slate .metric-card__value {
  color: rgb(30 41 59);
}

.metric-card--emerald .metric-card__label,
.metric-card--emerald .metric-card__value {
  color: rgb(5 150 105);
}

.metric-card--sky .metric-card__label,
.metric-card--sky .metric-card__value {
  color: rgb(2 132 199);
}

.metric-card--amber .metric-card__label,
.metric-card--amber .metric-card__value {
  color: rgb(217 119 6);
}

.metric-card--rose .metric-card__label,
.metric-card--rose .metric-card__value {
  color: rgb(225 29 72);
}

.action-icon {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.65rem;
  transition: all 0.18s ease;
}

.action-icon svg {
  height: 1rem;
  width: 1rem;
}

.action-icon:hover {
  transform: translateY(-1px);
}
</style>
