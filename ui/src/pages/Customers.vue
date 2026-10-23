<template>
  <div class="customer-page space-y-5">
    <header class="customer-hero">
      <div class="relative z-10 flex flex-wrap items-center justify-between gap-5">
        <div class="max-w-2xl">
          <p class="text-xs font-semibold uppercase tracking-[0.18em] text-teal-100">Penjualan / Direktori pelanggan</p>
          <h1 class="mt-2 text-3xl font-bold tracking-tight text-white">Kenali pelanggan Anda</h1>
          <p class="mt-2 text-sm leading-relaxed text-teal-50">Temukan kontak, lihat kunjungan, dan kenali pesanan pelanggan di outlet yang Anda kelola.</p>
        </div>
        <AppButton variant="secondary" :loading="loading" @click="load">Muat ulang</AppButton>
      </div>
    </header>

    <section class="grid gap-3 sm:grid-cols-3" aria-label="Ringkasan pelanggan">
      <div class="summary-card">
        <p class="summary-label">{{ search.trim() ? 'Pelanggan ditemukan' : 'Pelanggan terdaftar' }}</p>
        <p class="summary-value">{{ loading || errorMsg ? '—' : number(total) }}</p>
        <p class="summary-note">{{ search.trim() ? 'Sesuai pencarian dan akses outlet' : 'Dengan riwayat di outlet yang dapat Anda akses' }}</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Pernah kembali</p>
        <p class="summary-value">{{ loading || errorMsg ? '—' : number(returningCount) }} <span class="text-sm font-normal text-slate-500">pelanggan</span></p>
        <p class="summary-note">Lebih dari 1 kunjungan · halaman ini</p>
      </div>
      <div class="summary-card">
        <p class="summary-label">Nomor HP tersedia</p>
        <p class="summary-value">{{ loading || errorMsg ? '—' : number(contactCount) }} <span class="text-sm font-normal text-slate-500">pelanggan</span></p>
        <p class="summary-note">Dari {{ loading || errorMsg ? '—' : number(customers.length) }} pelanggan di halaman ini</p>
      </div>
    </section>

    <section class="customer-panel" aria-labelledby="customer-list-title" :aria-busy="loading">
      <div class="border-b border-slate-100 p-4 sm:p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 id="customer-list-title" class="text-base font-bold text-slate-900">Daftar pelanggan</h2>
            <p class="mt-1 text-xs leading-relaxed text-slate-500">Tercatat otomatis dari pesanan kasir. Pilih pelanggan untuk melihat rinciannya.</p>
          </div>
          <span class="rounded-full bg-slate-100 px-3 py-1.5 text-xs font-medium text-slate-600">Kunjungan terbaru lebih dulu</span>
        </div>
        <div class="mt-4 flex flex-wrap items-end gap-3">
          <div class="min-w-0 flex-1 basis-64">
            <label for="customer-search" class="mb-1.5 block text-xs font-semibold text-slate-600">Cari pelanggan</label>
            <div class="relative">
              <svg aria-hidden="true" class="pointer-events-none absolute left-3 top-3.5 h-4 w-4 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m16 16 4 4" /></svg>
              <input id="customer-search" v-model="search" type="search" maxlength="200" placeholder="Nama atau nomor HP, mis. 081234567890" class="customer-search" />
            </div>
          </div>
          <AppButton v-if="search" variant="ghost" @click="search = ''">Hapus pencarian</AppButton>
        </div>
      </div>

      <div v-if="loading" class="p-5" role="status">
        <p class="mb-4 text-sm text-slate-500">Memuat pelanggan…</p>
        <div v-for="i in 4" :key="i" class="flex items-center gap-4 border-t border-slate-100 py-4" aria-hidden="true">
          <div class="h-11 w-11 shrink-0 animate-pulse rounded-2xl bg-slate-100" />
          <div class="flex-1 space-y-2"><div class="h-3 w-2/5 animate-pulse rounded bg-slate-100" /><div class="h-3 w-3/5 animate-pulse rounded bg-slate-100" /></div>
        </div>
      </div>
      <div v-else-if="errorMsg" class="space-y-4 p-5">
        <AppAlert type="error" :message="errorMsg" />
        <AppButton variant="secondary" @click="load">Coba lagi</AppButton>
      </div>
      <div v-else-if="!customers.length" class="px-5 py-12 text-center" role="status">
        <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-teal-50 text-teal-700">
          <svg aria-hidden="true" class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="8" r="4" /><path d="M4 21v-2a8 8 0 0 1 16 0v2" /></svg>
        </div>
        <h3 class="font-semibold text-slate-800">{{ search.trim() ? 'Pelanggan tidak ditemukan' : 'Belum ada pelanggan tercatat' }}</h3>
        <p class="mx-auto mt-2 max-w-sm text-sm leading-relaxed text-slate-500">{{ search.trim() ? 'Coba nama lain atau nomor HP lengkap. Anda juga dapat menghapus pencarian.' : 'Pelanggan akan muncul setelah pesanan kasir mencatat nama atau nomor HP di outlet yang dapat Anda akses.' }}</p>
      </div>
      <template v-else>
        <div class="hidden overflow-x-auto xl:block">
          <table class="customer-table">
            <caption class="sr-only">Pelanggan beserta kontak, kunjungan, outlet, dan nilai pesanan</caption>
            <thead><tr><th scope="col">Pelanggan</th><th scope="col">Kunjungan</th><th scope="col">Outlet dikunjungi</th><th scope="col" class="text-right">Nilai pesanan</th><th scope="col">Kunjungan terakhir</th></tr></thead>
            <tbody>
              <tr v-for="row in customers" :key="row.id">
                <td>
                  <button type="button" class="customer-identity" :aria-label="`Lihat detail ${customerName(row)}`" @click="openDetail(row, $event)">
                    <span class="customer-avatar" aria-hidden="true">{{ initials(row.name) }}</span>
                    <span class="min-w-0"><span class="block font-semibold text-slate-800">{{ customerName(row) }}</span><span class="mt-1 block text-xs font-normal text-slate-500">{{ row.phone || 'Nomor HP belum tercatat' }}</span><span class="mt-1.5 block text-xs font-semibold text-teal-700">Lihat detail <span aria-hidden="true">↗</span></span></span>
                  </button>
                </td>
                <td><span class="visit-badge">{{ number(row.visit_count) }} kali</span></td>
                <td><span class="block max-w-44 break-words text-slate-600">{{ row.outlet_names || 'Outlet belum tercatat' }}</span></td>
                <td class="text-right font-semibold tabular-nums whitespace-nowrap">{{ formatRupiah(row.total_spent) }}</td>
                <td class="text-xs whitespace-nowrap text-slate-500">{{ formatDateTime(row.last_visit_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <ul class="grid gap-3 bg-slate-50/60 p-3 sm:grid-cols-2 sm:p-4 xl:hidden" aria-label="Kartu pelanggan">
          <li v-for="row in customers" :key="row.id" class="min-w-0 rounded-2xl border border-slate-200/80 bg-white p-4">
            <div class="flex items-start gap-3">
              <span class="customer-avatar" aria-hidden="true">{{ initials(row.name) }}</span>
              <div class="min-w-0 flex-1"><h3 class="break-words font-semibold text-slate-900">{{ customerName(row) }}</h3><p class="mt-1 break-all text-xs text-slate-500">{{ row.phone || 'Nomor HP belum tercatat' }}</p></div>
            </div>
            <dl class="my-4 grid grid-cols-2 gap-x-3 gap-y-4 border-y border-slate-100 py-4 text-sm">
              <div><dt class="text-xs text-slate-500">Kunjungan</dt><dd class="mt-1 font-semibold text-teal-800">{{ number(row.visit_count) }} kali</dd></div>
              <div><dt class="text-xs text-slate-500">Nilai pesanan</dt><dd class="mt-1 break-words font-semibold tabular-nums text-slate-800">{{ formatRupiah(row.total_spent) }}</dd></div>
              <div class="col-span-2"><dt class="text-xs text-slate-500">Outlet dikunjungi</dt><dd class="mt-1 break-words text-slate-700">{{ row.outlet_names || 'Outlet belum tercatat' }}</dd></div>
              <div class="col-span-2"><dt class="text-xs text-slate-500">Kunjungan terakhir</dt><dd class="mt-1 text-slate-700">{{ formatDateTime(row.last_visit_at) }}</dd></div>
            </dl>
            <button type="button" class="detail-button" :aria-label="`Lihat detail ${customerName(row)}`" @click="openDetail(row, $event)">Lihat detail pelanggan <span aria-hidden="true">→</span></button>
          </li>
        </ul>
        <div class="border-t border-slate-100 p-4 sm:px-5">
          <p class="text-xs text-slate-500" aria-live="polite">Menampilkan {{ number((page - 1) * limit + 1) }}–{{ number(Math.min(page * limit, total)) }} dari {{ number(total) }} pelanggan{{ search.trim() ? ' yang cocok' : '' }}.</p>
          <AppPagination :page="page" :total="total" :per-page="limit" @change="changePage" />
        </div>
      </template>
    </section>
    <p class="px-1 text-xs leading-relaxed text-slate-500">Satu kunjungan mewakili satu pesanan. Nilai pesanan mencakup seluruh status, termasuk pesanan belum selesai atau dibatalkan.</p>

    <AppModal v-model="showDetail" title="Detail pelanggan" size="2xl">
      <div ref="detailRoot" class="min-w-0">
        <div v-if="detailLoading" class="flex items-center justify-center gap-3 py-12 text-sm text-slate-500" role="status"><AppSpinner />Memuat detail pelanggan…</div>
        <div v-else-if="detailError" class="space-y-4"><AppAlert type="error" :message="detailError" /><AppButton variant="secondary" @click="loadDetail">Coba lagi</AppButton></div>
        <div v-else-if="detail" class="space-y-6">
          <div class="flex flex-wrap items-center gap-4 rounded-2xl bg-teal-50/70 p-4">
            <span class="customer-avatar customer-avatar--large" aria-hidden="true">{{ initials(detail.customer.name) }}</span>
            <div class="min-w-0 flex-1 basis-40"><h3 class="break-words text-xl font-bold text-slate-900">{{ customerName(detail.customer) }}</h3><p class="mt-1 break-all text-sm text-slate-600">{{ detail.customer.phone || 'Nomor HP belum tercatat' }}</p></div>
            <a v-if="phoneLink(detail.customer.phone)" :href="phoneLink(detail.customer.phone)" class="contact-button">Hubungi pelanggan</a>
          </div>
          <dl class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="detail-stat"><dt>Total kunjungan</dt><dd>{{ number(detail.customer.visit_count) }} kali</dd></div>
            <div class="detail-stat"><dt>Outlet dikunjungi</dt><dd>{{ number(detail.outlets.length) }} outlet</dd></div>
            <div class="detail-stat"><dt>Nilai pesanan</dt><dd>{{ formatRupiah(detail.customer.total_spent) }}</dd></div>
            <div class="detail-stat"><dt>Rata-rata per kunjungan</dt><dd>{{ detail.customer.visit_count ? formatRupiah(detail.customer.total_spent / detail.customer.visit_count) : '—' }}</dd></div>
          </dl>
          <p class="text-xs leading-relaxed text-slate-500">Terakhir berkunjung: <span class="font-medium text-slate-700">{{ formatDateTime(detail.customer.last_visit_at) }}</span>. Ringkasan mencakup seluruh pesanan di outlet yang dapat Anda akses, termasuk pesanan belum selesai atau dibatalkan.</p>
          <section aria-labelledby="customer-outlets-title">
            <h3 id="customer-outlets-title" class="mb-3 text-sm font-bold text-slate-900">Kunjungan per outlet</h3>
            <ul v-if="detail.outlets.length" class="grid gap-3 sm:grid-cols-2">
              <li v-for="outlet in detail.outlets" :key="outlet.outlet_id" class="min-w-0 rounded-xl border border-slate-200 p-3.5">
                <p class="break-words text-sm font-semibold text-slate-800">{{ outlet.outlet_name || 'Outlet belum tercatat' }}</p>
                <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-sm"><span class="text-teal-700">{{ number(outlet.visit_count) }} kunjungan</span><span class="font-semibold tabular-nums text-slate-700">{{ formatRupiah(outlet.total_spent) }}</span></div>
                <p class="mt-2 text-xs text-slate-500">Terakhir: {{ formatDateTime(outlet.last_visit_at) }}</p>
              </li>
            </ul>
            <p v-else class="text-sm text-slate-500">Belum ada kunjungan di outlet yang dapat Anda akses.</p>
          </section>
          <section aria-labelledby="customer-history-title">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><h3 id="customer-history-title" class="text-sm font-bold text-slate-900">Riwayat kunjungan</h3><span class="text-xs text-slate-500">Terbaru lebih dulu</span></div>
            <p v-if="!detail.visits.length" class="rounded-xl bg-slate-50 p-5 text-center text-sm text-slate-500">Belum ada riwayat kunjungan yang dapat ditampilkan.</p>
            <div v-else class="space-y-3">
              <p class="text-xs leading-relaxed text-slate-500">Buka kunjungan untuk melihat menu yang dipesan. Menampilkan {{ Math.min(visitLimit, detail.visits.length) }} dari {{ number(detail.visits.length) }} kunjungan yang dimuat.</p>
              <p v-if="detail.customer.visit_count > detail.visits.length" class="rounded-lg bg-amber-50 p-3 text-xs leading-relaxed text-amber-800">Riwayat dibatasi hingga 200 kunjungan terbaru. Ringkasan di atas tetap mencakup seluruh kunjungan.</p>
              <details v-for="(visit, index) in visibleVisits" :key="visit.order_id" class="visit-card" :open="index === 0">
                <summary class="visit-summary">
                  <span class="min-w-0 flex-1"><span class="block break-words text-sm font-semibold text-slate-800">{{ visit.outlet_name || visit.outlet_code || 'Outlet belum tercatat' }}</span><span class="mt-1 block text-xs text-slate-500">{{ formatDateTime(visit.created_at) }}</span></span>
                  <span class="flex flex-wrap items-center gap-2"><span class="status-badge" :class="statusInfo(visit.status).class">{{ statusInfo(visit.status).label }}</span><span class="text-sm font-semibold tabular-nums text-slate-800">{{ formatRupiah(visit.total_amount) }}</span></span>
                  <svg aria-hidden="true" class="visit-chevron h-4 w-4 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 9 6 6 6-6" /></svg>
                </summary>
                <div class="border-t border-slate-100 p-4">
                  <p v-if="visit.table_number || visit.pax" class="mb-3 text-xs text-slate-500"><span v-if="visit.table_number">Meja {{ visit.table_number }}</span><span v-if="visit.table_number && visit.pax"> · </span><span v-if="visit.pax">{{ visit.pax }} orang</span></p>
                  <ul v-if="parseItems(visit.items).length" class="divide-y divide-slate-100">
                    <li v-for="(item, i) in parseItems(visit.items)" :key="i" class="flex items-start justify-between gap-4 py-2 text-sm"><span class="min-w-0 break-words text-slate-600"><span class="font-semibold text-slate-800">{{ item.qty }}×</span> {{ item.product_name || 'Menu tanpa nama' }}<span v-if="item.is_complimentary" class="block text-xs text-teal-700">Komplimen</span></span><span class="shrink-0 tabular-nums text-slate-600">{{ formatRupiah(item.subtotal ?? (item.qty * item.price)) }}</span></li>
                  </ul>
                  <p v-else class="py-2 text-sm text-slate-500">Rincian menu belum tersedia.</p>
                  <div class="mt-2 flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3 text-sm font-semibold"><span class="text-slate-500">Total pesanan</span><span class="tabular-nums text-slate-900">{{ formatRupiah(visit.total_amount) }}</span></div>
                </div>
              </details>
              <AppButton v-if="visitLimit < detail.visits.length" variant="secondary" class="w-full" @click="visitLimit += 10">Tampilkan kunjungan berikutnya</AppButton>
            </div>
          </section>
        </div>
      </div>
      <template #footer><AppButton variant="secondary" @click="showDetail = false">Tutup detail</AppButton></template>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppSpinner from '@/components/ui/AppSpinner.vue'
import { customersApi } from '@/api/customers.js'
import { formatDateTime, formatRupiah } from '@/utils/format.js'

const customers = ref([])
const loading = ref(true)
const errorMsg = ref('')
const search = ref('')
const page = ref(1)
const limit = 20
const total = ref(0)
const returningCount = computed(() => customers.value.filter(c => c.visit_count > 1).length)
const contactCount = computed(() => customers.value.filter(c => c.phone?.trim()).length)
const numberFormat = new Intl.NumberFormat('id-ID')
const number = value => numberFormat.format(value || 0)
const customerName = customer => customer.name?.trim() || 'Pelanggan tanpa nama'
const showDetail = ref(false)
const selectedId = ref('')
const detail = ref(null)
const detailRoot = ref(null)
const detailLoading = ref(false)
const detailError = ref('')
const visitLimit = ref(10)
const visibleVisits = computed(() => detail.value?.visits.slice(0, visitLimit.value) || [])
let searchTimer, listSequence = 0, detailSequence = 0, lastTrigger, bodyOverflow

async function load() {
  clearTimeout(searchTimer)
  const sequence = ++listSequence
  loading.value = true
  errorMsg.value = ''
  try {
    const data = await customersApi.list({ search: search.value.trim(), page: page.value, limit })
    if (sequence !== listSequence) return
    customers.value = data.customers || []
    total.value = data.pagination?.total || 0
  } catch (error) {
    if (sequence !== listSequence) return
    customers.value = []
    total.value = 0
    errorMsg.value = error.message || 'Gagal memuat pelanggan. Silakan coba lagi.'
  } finally {
    if (sequence === listSequence) loading.value = false
  }
}
function changePage(value) { page.value = value; load() }
watch(search, () => {
  ++listSequence
  loading.value = true
  page.value = 1
  clearTimeout(searchTimer)
  searchTimer = setTimeout(load, 350)
}, { flush: 'sync' })

function openDetail(row, event) {
  lastTrigger = event?.currentTarget || document.activeElement
  selectedId.value = row.id
  showDetail.value = true
  loadDetail()
}
async function loadDetail() {
  const sequence = ++detailSequence
  detailLoading.value = true
  detailError.value = ''
  detail.value = null
  visitLimit.value = 10
  try {
    const data = await customersApi.get(selectedId.value)
    if (sequence !== detailSequence || !showDetail.value) return
    detail.value = { ...data, outlets: data.outlets || [], visits: data.visits || [] }
  } catch (error) {
    if (sequence === detailSequence && showDetail.value) detailError.value = error.message || 'Gagal memuat detail pelanggan.'
  } finally {
    if (sequence === detailSequence) detailLoading.value = false
  }
}
// Keep keyboard focus inside this dialog and restore the triggering control.
function dialogKeydown(event) {
  if (event.key === 'Escape') { event.preventDefault(); showDetail.value = false; return }
  if (event.key !== 'Tab') return
  const dialog = detailRoot.value?.closest('[role="dialog"]')
  const controls = [...(dialog?.querySelectorAll('button:not(:disabled), a[href], summary, [tabindex="0"]') || [])].filter(el => el.getClientRects().length)
  const first = controls[0], last = controls[controls.length - 1]
  if (!first) return
  if (!dialog.contains(document.activeElement)) { event.preventDefault(); (event.shiftKey ? last : first).focus() }
  else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
watch(showDetail, async open => {
  if (open) {
    bodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', dialogKeydown, true)
    await nextTick()
    if (showDetail.value) detailRoot.value?.closest('[role="dialog"]')?.querySelector('button')?.focus()
  } else {
    ++detailSequence
    document.body.style.overflow = bodyOverflow ?? ''
    document.removeEventListener('keydown', dialogKeydown, true)
    lastTrigger?.focus()
  }
})
function parseItems(items) {
  try { const value = typeof items === 'string' ? JSON.parse(items) : items; return Array.isArray(value) ? value.filter(item => item && typeof item === 'object') : [] }
  catch { return [] }
}
function initials(name) { return (name || '').trim().split(/\s+/).filter(Boolean).slice(0, 2).map(part => Array.from(part)[0]).join('').toUpperCase() || '?' }
function phoneLink(phone) { const digits = (phone || '').replace(/[^\d+]/g, ''); return /\d{6}/.test(digits) ? `tel:${digits}` : '' }
const STATUSES = {
  completed: ['Selesai', 'status-success'], paid: ['Dibayar', 'status-success'],
  pending: ['Menunggu', 'status-pending'], cooking: ['Dimasak', 'status-pending'],
  ready: ['Siap disajikan', 'status-info'], served: ['Disajikan', 'status-info'],
  open: ['Berjalan', 'status-info'], closed: ['Ditutup', 'status-neutral'],
  cancelled: ['Dibatalkan', 'status-danger'], void: ['Dibatalkan', 'status-danger'], voided: ['Dibatalkan', 'status-danger'], refunded: ['Dikembalikan', 'status-danger'],
}
function statusInfo(status) { const [label, style] = STATUSES[status] || [status || 'Belum tercatat', 'status-neutral']; return { label, class: style } }
onMounted(load)
onBeforeUnmount(() => {
  clearTimeout(searchTimer)
  ++listSequence; ++detailSequence
  document.removeEventListener('keydown', dialogKeydown, true)
  if (showDetail.value) document.body.style.overflow = bodyOverflow ?? ''
})
</script>

<style scoped>
.customer-page { min-width: 0; }
.customer-hero { position: relative; overflow: hidden; border-radius: 24px; padding: 28px; background: linear-gradient(115deg, #134e4a, #0f766e); }
.customer-hero h1 { color: #fff; }
.customer-hero::after { content: ''; position: absolute; width: 280px; height: 280px; border: 1px solid #ffffff18; border-radius: 50%; right: -55px; top: -155px; box-shadow: 0 0 0 45px #ffffff05, 0 0 0 90px #ffffff04; pointer-events: none; }
.summary-card { min-width: 0; padding: 20px; border: 1px solid #e2e8f0; border-radius: 18px; background: white; }
.summary-label { color: #475569; font-size: 12px; font-weight: 600; }
.summary-value { margin-top: 8px; font-size: 28px; font-weight: 700; color: #134e4a; font-variant-numeric: tabular-nums; }
.summary-note { margin-top: 5px; font-size: 11px; line-height: 1.6; color: #64748b; }
.customer-panel { overflow: hidden; border: 1px solid #e2e8f0; border-radius: 20px; background: white; box-shadow: 0 3px 18px #0f172a03; }
.customer-search { width: 100%; min-height: 44px; border: 1px solid #cbd5e1; border-radius: 12px; padding: 10px 12px 10px 36px; font-size: 14px; color: #334155; background: #fff; }
.customer-search:focus { outline: 2px solid #0d9488; outline-offset: 2px; }
.customer-table { width: 100%; text-align: left; font-size: 13px; }
.customer-table th { padding: 14px 16px; background: #f8fafc; color: #64748b; font-size: 11px; font-weight: 600; white-space: nowrap; }
.customer-table td { padding: 18px 16px; border-top: 1px solid #f1f5f9; }
.customer-table tbody tr:hover { background: #f0fdfa60; }
.customer-identity { display: flex; align-items: center; gap: 12px; min-width: 170px; max-width: 280px; text-align: left; overflow-wrap: anywhere; border-radius: 8px; }
.customer-avatar { display: inline-flex; flex-shrink: 0; align-items: center; justify-content: center; width: 42px; height: 42px; border-radius: 14px; color: #0f766e; background: #ccfbf150; border: 1px solid #ccfbf1; font-size: 13px; font-weight: 700; }
.customer-avatar--large { width: 54px; height: 54px; font-size: 18px; background: white; }
.visit-badge { display: inline-flex; border-radius: 7px; padding: 5px 9px; background: #f0fdfa; color: #0f766e; white-space: nowrap; font-size: 12px; font-weight: 600; }
.detail-button { display: flex; justify-content: space-between; align-items: center; gap: 12px; width: 100%; min-height: 44px; border-radius: 10px; background: #f0fdfa; padding: 10px 12px; font-size: 13px; font-weight: 600; color: #0f766e; }
.detail-button:hover { background: #ccfbf1; }
.contact-button { display: inline-flex; min-height: 44px; align-items: center; justify-content: center; border: 1px solid #99f6e4; background: white; border-radius: 10px; padding: 10px 14px; font-size: 12px; font-weight: 600; color: #0f766e; }
.detail-stat { min-width: 0; background: #f8fafc; border-radius: 12px; padding: 14px; }
.detail-stat dt { font-size: 11px; line-height: 1.5; color: #64748b; }
.detail-stat dd { margin-top: 6px; font-size: 15px; font-weight: 700; color: #1e293b; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.visit-card { overflow: hidden; border: 1px solid #e2e8f0; border-radius: 14px; }
.visit-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; padding: 16px; cursor: pointer; list-style: none; }
.visit-summary::-webkit-details-marker { display: none; }
.visit-summary:hover { background: #f8fafc; }
.visit-card[open] .visit-chevron { transform: rotate(180deg); }
.status-badge { display: inline-flex; padding: 4px 8px; border-radius: 6px; font-size: 11px; font-weight: 600; }
.status-success { background: #ecfdf5; color: #047857; }
.status-pending { background: #fffbeb; color: #92400e; }
.status-info { background: #eff6ff; color: #1d4ed8; }
.status-danger { background: #fff1f2; color: #be123c; }
.status-neutral { background: #f1f5f9; color: #475569; }
button:focus-visible, a:focus-visible, summary:focus-visible { outline: 2px solid #0d9488; outline-offset: 3px; }
@media (max-width: 639px) {
  .customer-hero { padding: 22px 20px; }
  .customer-hero h1 { font-size: 26px; }
  .summary-card { display: grid; grid-template-columns: 1fr auto; column-gap: 12px; padding: 16px 18px; }
  .summary-value { grid-column: 2; grid-row: 1 / 3; align-self: center; margin: 0; font-size: 24px; }
  .summary-value span { display: none; }
  .summary-note { grid-column: 1; }
  .visit-summary > span:first-child { flex-basis: calc(100% - 32px); }
  .visit-summary > span:nth-child(2) { order: 3; flex-basis: 100%; justify-content: space-between; }
}
@media (prefers-reduced-motion: reduce) { .animate-pulse { animation: none; } }
</style>
