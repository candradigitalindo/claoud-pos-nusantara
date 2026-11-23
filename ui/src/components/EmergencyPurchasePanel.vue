<!--
  EmergencyPurchasePanel.vue — bagian detail Pembelian Darurat:
  1. kas: menunggu kasir → diserahkan (atau dikembalikan kasir → kirim ulang,
     atau dibatalkan sebelum kas keluar);
  2. bukti belanja: foto struk + total, diunggah pengaju sesudah belanja;
  3. pengesahan: penyetuju mengesahkan (butuh struk) atau menolak (alasan).
  Penolakan tidak membalik pembukuan — uangnya sudah keluar — dan menjadi
  temuan yang ditindaklanjuti Keuangan.
-->
<template>
  <div class="overflow-hidden rounded-xl border border-rose-200 bg-white">
    <div class="flex flex-wrap items-center justify-between gap-2 border-b border-rose-100 bg-rose-50 px-3 py-2">
      <p class="flex items-center gap-1.5 text-sm font-semibold text-rose-900">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        Pembelian Darurat
      </p>
      <span v-if="ratification" :class="['rounded-full px-2 py-0.5 text-[11px] font-semibold', ratification.cls]">{{ ratification.label }}</span>
    </div>

    <div class="space-y-3 p-3 text-xs">
      <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
        <dt class="text-gray-400">Kategori</dt>
        <dd class="font-medium text-gray-900">{{ em.category_label || em.category || '-' }}</dd>
        <dt class="text-gray-400">Alasan</dt>
        <dd class="break-words text-gray-800">{{ em.reason || '-' }}</dd>
        <dt class="text-gray-400">Penerima kas</dt>
        <dd class="text-gray-800">{{ em.recipient || '-' }}</dd>
        <template v-if="doc.vendor_name">
          <dt class="text-gray-400">Toko/teknisi</dt>
          <dd class="text-gray-800">{{ doc.vendor_name }}</dd>
        </template>
        <dt class="text-gray-400">Nilai kas</dt>
        <dd class="font-semibold text-gray-900">{{ formatRupiah(doc.total_final) }}</dd>
      </dl>

      <!-- 1. Kas -->
      <div class="rounded-lg border p-2.5" :class="cashBox.cls">
        <p class="font-semibold">{{ cashBox.title }}</p>
        <p v-if="cashBox.text" class="mt-0.5">{{ cashBox.text }}</p>
        <div v-if="!cancelled && !cashHanded" class="mt-2 flex flex-wrap gap-2">
          <AppButton v-if="canSubmitSide && canResend" size="sm" :loading="busy === 'resend'" @click="resend">Kirim Ulang ke Kasir</AppButton>
          <AppButton v-if="(canSubmitSide || canApprove) && payout?.status !== 'claimed'" size="sm" variant="secondary"
            :disabled="!!busy" @click="cancelOpen = !cancelOpen">Batalkan</AppButton>
        </div>
        <div v-if="cancelOpen && !cashHanded && !cancelled" class="mt-2 space-y-2">
          <textarea v-model="cancelReason" rows="2" class="fld" placeholder="Alasan pembatalan (mis. genset sudah hidup lagi)" />
          <div class="flex justify-end gap-2">
            <AppButton size="sm" variant="secondary" @click="cancelOpen = false">Tutup</AppButton>
            <AppButton size="sm" variant="danger" :loading="busy === 'cancel'" :disabled="reasonLength(cancelReason) < VERIFY_REASON_MIN" @click="cancelEmergency">Batalkan Pembelian</AppButton>
          </div>
        </div>
      </div>

      <!-- 2. Bukti belanja -->
      <div v-if="cashHanded && !cancelled" class="rounded-lg border border-gray-200 p-2.5">
        <p class="font-semibold text-gray-800">Bukti Belanja</p>
        <template v-if="em.proof_url">
          <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-gray-700">
            <a v-if="safeUrl(em.proof_url)" :href="safeUrl(em.proof_url)" target="_blank" rel="noopener noreferrer"
              class="inline-flex items-center gap-1 font-medium text-blue-600 hover:underline">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>
              Foto struk
            </a>
            <span>Total struk <b>{{ formatRupiah(em.proof_total) }}</b></span>
            <span class="text-gray-400">{{ em.proof_by || '-' }}<template v-if="em.proof_at"> · {{ formatDateTime(em.proof_at) }}</template></span>
          </div>
          <p v-if="em.proof_notes" class="mt-0.5 break-words text-gray-500">{{ em.proof_notes }}</p>
          <p v-if="diff > 0" class="mt-1 rounded bg-amber-50 px-2 py-1 text-amber-800">
            Struk lebih kecil {{ formatRupiah(diff) }} dari kas yang diserahkan — kembalikan sisanya ke laci kasir dan catat sebagai Kas Masuk.
          </p>
          <p v-else-if="diff < 0" class="mt-1 rounded bg-amber-50 px-2 py-1 text-amber-800">
            Struk lebih besar {{ formatRupiah(-diff) }} dari kas yang diserahkan — selisihnya belum diganti; ajukan terpisah bila perlu.
          </p>
        </template>
        <p v-else class="mt-1 text-gray-500">Belum ada. Unggah foto struk dan total belanjanya — pengesahan menunggu bukti ini.</p>

        <div v-if="canEditProof">
          <button v-if="!proofOpen" type="button" class="mt-2 text-xs font-semibold text-rose-700 hover:text-rose-900" @click="openProof">
            {{ em.proof_url ? 'Ganti bukti' : '+ Unggah struk' }}
          </button>
          <div v-else class="mt-2 space-y-2 rounded-lg bg-gray-50 p-2.5">
            <PhotoCapture v-model="proof.url" label="Foto Struk" hint="Foto struk/nota dari toko atau teknisi." />
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div>
                <label class="mb-1 block text-sm font-medium text-gray-700">Total di Struk</label>
                <RupiahInput v-model="proof.total" placeholder="0" />
              </div>
              <AppInput v-model="proof.notes" label="Catatan (opsional)" placeholder="mis. kembalian sudah ke laci" />
            </div>
            <div class="flex justify-end gap-2">
              <AppButton size="sm" variant="secondary" :disabled="busy === 'proof'" @click="proofOpen = false">Batal</AppButton>
              <AppButton size="sm" :loading="busy === 'proof'" @click="saveProof">Simpan Bukti</AppButton>
            </div>
          </div>
        </div>
      </div>

      <!-- 3. Pengesahan -->
      <div v-if="cashHanded && !cancelled" class="rounded-lg border border-gray-200 p-2.5">
        <p class="font-semibold text-gray-800">Pengesahan</p>
        <template v-if="em.ratification === 'pending'">
          <template v-if="canApprove">
            <p class="mt-1 text-gray-500">Periksa alasan dan struknya. Menolak tidak membatalkan pembayaran — uangnya sudah keluar — tetapi dicatat sebagai temuan untuk ditindaklanjuti Keuangan.</p>
            <textarea v-model="decisionNotes" rows="2" class="fld mt-2" placeholder="Catatan (wajib bila menolak, minimal 5 huruf)" />
            <div class="mt-2 flex flex-wrap justify-end gap-2">
              <AppButton size="sm" variant="danger" :loading="busy === 'reject'" :disabled="!!busy || reasonLength(decisionNotes) < VERIFY_REASON_MIN" @click="decide('reject')">Tolak Pengesahan</AppButton>
              <AppButton size="sm" :loading="busy === 'ratify'" :disabled="!!busy || !em.proof_url" @click="decide('ratify')">Sahkan</AppButton>
            </div>
            <p v-if="!em.proof_url" class="mt-1 text-right text-[11px] text-gray-400">Sahkan aktif setelah pengaju mengunggah struk.</p>
          </template>
          <p v-else class="mt-1 text-gray-500">Menunggu penyetuju mengesahkan<template v-if="!em.proof_url"> — struk belum diunggah pengaju</template>.</p>
        </template>
        <template v-else>
          <p class="mt-1" :class="em.ratification === 'ratified' ? 'text-emerald-700' : 'text-red-700'">
            {{ em.ratification === 'ratified' ? 'Disahkan' : 'Pengesahan ditolak' }} oleh <b>{{ em.ratified_by || '-' }}</b><template v-if="em.ratified_at"> · {{ formatDateTime(em.ratified_at) }}</template>
          </p>
          <p v-if="em.ratification_notes" class="mt-0.5 break-words text-gray-600">Catatan: {{ em.ratification_notes }}</p>
          <p v-if="em.ratification === 'rejected'" class="mt-1 rounded bg-red-50 px-2 py-1 text-red-800">
            Temuan: pembayaran tidak dibatalkan karena uangnya sudah keluar — tindak lanjuti, mis. menjadi tanggungan manajer outlet.
          </p>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { safeUrl } from '@/utils/safeUrl.js'
import { RATIFICATION, VERIFY_REASON_MIN, reasonLength, isConflictError, emergencyCashHanded } from '@/utils/procurement.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppInput from '@/components/ui/AppInput.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const props = defineProps({
  doc: { type: Object, required: true },
  // Pengaju/outlet: Pembelian Darurat atau Buat pengajuan barang/jasa.
  canSubmitSide: { type: Boolean, default: false },
  // Penyetuju barang/jasa.
  canApprove: { type: Boolean, default: false },
})
const emit = defineEmits(['changed'])
const toast = useToastStore()

const busy = ref('')
const cancelOpen = ref(false)
const cancelReason = ref('')
const proofOpen = ref(false)
const proof = ref({ url: '', total: 0, notes: '' })
const decisionNotes = ref('')

watch(() => props.doc?.id, () => {
  cancelOpen.value = false
  cancelReason.value = ''
  proofOpen.value = false
  decisionNotes.value = ''
})

const em = computed(() => props.doc?.emergency || {})
const ratification = computed(() => RATIFICATION[em.value.ratification] || null)
const cancelled = computed(() => ['cancelled', 'rejected'].includes(props.doc?.status))
const cashHanded = computed(() => emergencyCashHanded(props.doc))
const payout = computed(() => props.doc?.cash_payout || null)
// Tidak ada tugas kas terbuka (dikembalikan kasir / ditarik) dan belum dibayar.
const canResend = computed(() => props.doc?.status === 'payment_requested' && !cashHanded.value &&
  !(payout.value && ['pending', 'claimed'].includes(payout.value.status)))
const canEditProof = computed(() => props.canSubmitSide && em.value.ratification === 'pending')
const diff = computed(() => {
  if (em.value.proof_total == null) return 0
  return Math.round((Number(props.doc?.paid_amount) || 0) - Number(em.value.proof_total))
})

const cashBox = computed(() => {
  const d = props.doc || {}
  if (cancelled.value) {
    return { cls: 'border-gray-200 bg-gray-50 text-gray-600', title: 'Dibatalkan sebelum kas diserahkan', text: d.rejected_reason || '' }
  }
  if (cashHanded.value) {
    return {
      cls: 'border-emerald-200 bg-emerald-50 text-emerald-800',
      title: `Kas ${formatRupiah(d.paid_amount)} sudah diserahkan kasir`,
      text: d.paid_at ? `Tercatat ${formatDateTime(d.paid_at)} lewat tablet kasir (PIN Manager/SVP).` : '',
    }
  }
  const p = payout.value
  if (p?.status === 'claimed') {
    return { cls: 'border-blue-200 bg-blue-50 text-blue-800', title: 'Kasir sedang menyerahkan kas', text: `Diproses ${p.cashier_name || 'kasir'} dengan izin ${p.authorized_by || '-'}.` }
  }
  if (p?.status === 'pending') {
    return { cls: 'border-amber-200 bg-amber-50 text-amber-800', title: 'Menunggu kasir menyerahkan kas',
      text: 'Tugas sudah ada di tablet kasir outlet: menu Bayar Pengadaan → Serahkan Kas (PIN Manager/SVP). Tablet harus online dan memakai App POS v10.20.' }
  }
  if (p?.status === 'returned') {
    return { cls: 'border-red-200 bg-red-50 text-red-800', title: 'Dikembalikan kasir', text: `${p.resolution_reason || '-'} — kirim ulang ke kasir atau batalkan.` }
  }
  return { cls: 'border-red-200 bg-red-50 text-red-800', title: 'Tidak ada tugas kas aktif', text: 'Tugas kas ditarik atau dikembalikan — kirim ulang ke kasir atau batalkan.' }
})

function fail(err, fallback) {
  if (isConflictError(err)) {
    toast.error('Pengajuan berubah — dimuat ulang.')
    emit('changed')
    return
  }
  toast.error(err?.message || fallback)
}

async function resend() {
  busy.value = 'resend'
  try {
    await purchaseApi.resendEmergency(props.doc.id, { version: props.doc.version })
    toast.success('Tugas kas dikirim ulang ke tablet kasir.')
    emit('changed')
  } catch (err) { fail(err, 'Gagal mengirim ulang tugas kas.') } finally { busy.value = '' }
}

async function cancelEmergency() {
  busy.value = 'cancel'
  try {
    await purchaseApi.cancelEmergency(props.doc.id, { notes: cancelReason.value.trim(), version: props.doc.version })
    toast.success('Pembelian Darurat dibatalkan.')
    cancelOpen.value = false
    emit('changed')
  } catch (err) { fail(err, 'Gagal membatalkan.') } finally { busy.value = '' }
}

function openProof() {
  proof.value = { url: em.value.proof_url || '', total: Number(em.value.proof_total) || Number(props.doc?.paid_amount) || 0, notes: em.value.proof_notes || '' }
  proofOpen.value = true
}

async function saveProof() {
  if (!proof.value.url) { toast.error('Foto struk wajib diunggah.'); return }
  if (!(Number(proof.value.total) > 0)) { toast.error('Isi total yang tertera di struk.'); return }
  busy.value = 'proof'
  try {
    await purchaseApi.submitEmergencyProof(props.doc.id, {
      proof_url: proof.value.url, proof_total: Number(proof.value.total), notes: proof.value.notes.trim(), version: props.doc.version,
    })
    toast.success('Bukti belanja tersimpan.')
    proofOpen.value = false
    emit('changed')
  } catch (err) { fail(err, 'Gagal menyimpan bukti.') } finally { busy.value = '' }
}

async function decide(decision) {
  busy.value = decision
  try {
    await purchaseApi.decideEmergency(props.doc.id, { decision, notes: decisionNotes.value.trim(), version: props.doc.version })
    toast.success(decision === 'ratify' ? 'Pembelian Darurat disahkan.' : 'Pengesahan ditolak — Keuangan diberi tahu.')
    emit('changed')
  } catch (err) { fail(err, 'Gagal menyimpan keputusan.') } finally { busy.value = '' }
}
</script>

<style scoped>
.fld {
  width: 100%;
  border-radius: 0.5rem;
  border: 1px solid rgb(209 213 219);
  background: #fff;
  padding: 0.4rem 0.6rem;
  font-size: 0.8125rem;
}
.fld:focus {
  outline: none;
  border-color: rgb(16 185 129);
  box-shadow: 0 0 0 2px rgb(16 185 129 / 0.25);
}
</style>
