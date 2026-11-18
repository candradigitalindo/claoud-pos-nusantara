<!--
  PurchasePayToCard — tujuan pembayaran sebuah pengajuan, BACA SAJA, dengan
  tombol salin (keputusan owner 19 Nov 2026: Pembelian yang mengisi, Keuangan
  hanya melihat dan menyalin). payTo = pay_to dari server (services/
  purchase_payto.go → buildPayTo); amount = nominal yang akan dibayar (opsional).
  Slot default: peringatan / tombol tambahan di bawah kartu.
-->
<template>
  <div class="overflow-hidden rounded-lg border bg-white" :class="ready ? 'border-gray-200' : 'border-red-200'">
    <div class="flex items-center justify-between gap-2 border-b px-4 py-2.5" :class="ready ? 'border-blue-100 bg-blue-50' : 'border-red-100 bg-red-50'">
      <p class="text-[10px] font-medium uppercase tracking-wider text-gray-500">Tujuan Pembayaran</p>
      <span v-if="methodLabel" class="rounded bg-white/80 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700">{{ methodLabel }}</span>
    </div>

    <div v-if="!ready" class="border-b border-red-100 bg-red-50/60 px-4 py-2 text-xs text-red-700">
      <p class="font-semibold">{{ returned ? 'Dikembalikan ke Pembelian' : 'Belum bisa dibayar' }}</p>
      <p class="break-words">{{ returned ? payTo.return_reason : payTo?.problem }}</p>
      <p v-if="returned" class="mt-0.5 text-[11px] text-red-500">
        oleh {{ payTo.returned_by || '-' }}<template v-if="payTo.returned_at"> · {{ formatDateTime(payTo.returned_at) }}</template>
      </p>
    </div>

    <dl v-if="effective" class="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-1 px-4 py-2.5 text-xs">
      <template v-if="effective === 'marketplace_va'">
        <dt class="text-gray-400">Marketplace</dt>
        <dd class="font-medium text-gray-800">{{ payTo.marketplace || '-' }}</dd>
        <dt class="text-gray-400">No. Pesanan</dt>
        <dd class="flex min-w-0 items-center gap-1 font-mono font-medium text-gray-800">
          <span class="break-all">{{ payTo.order_number || '-' }}</span>
          <AppActionButton v-if="payTo.order_number" icon="copy" label="Salin No. Pesanan" @click="copy(payTo.order_number, 'No. Pesanan')" />
        </dd>
      </template>
      <template v-if="effective !== 'manual'">
        <dt class="text-gray-400">Bank</dt>
        <dd class="font-medium text-gray-800">{{ payTo.bank_name || '-' }}</dd>
        <dt class="text-gray-400">{{ effective === 'marketplace_va' ? 'No. VA' : 'No. Rekening' }}</dt>
        <dd class="flex min-w-0 items-center gap-1 font-mono font-medium text-gray-800">
          <span class="break-all">{{ payTo.account_number || '-' }}</span>
          <AppActionButton v-if="payTo.account_number" icon="copy" :label="`Salin ${effective === 'marketplace_va' ? 'No. VA' : 'No. Rekening'}`"
            @click="copy(payTo.account_number, effective === 'marketplace_va' ? 'No. VA' : 'No. Rekening')" />
        </dd>
        <dt class="text-gray-400">Atas Nama</dt>
        <dd class="flex min-w-0 items-center gap-1 font-medium text-gray-800">
          <span class="break-words">{{ payTo.account_holder || '-' }}</span>
          <AppActionButton v-if="payTo.account_holder" icon="copy" label="Salin Atas Nama" @click="copy(payTo.account_holder, 'Atas Nama')" />
        </dd>
      </template>
      <template v-else>
        <dt class="text-gray-400">Cara Bayar</dt>
        <dd class="flex min-w-0 items-center gap-1 font-medium text-gray-800">
          <span class="break-words">{{ payTo.text || '-' }}</span>
          <AppActionButton v-if="payTo.text" icon="copy" label="Salin Cara Bayar" @click="copy(payTo.text, 'Cara bayar')" />
        </dd>
      </template>
      <template v-if="amount > 0">
        <dt class="text-gray-400">Nominal</dt>
        <dd class="flex items-center gap-1 font-semibold text-emerald-700">
          {{ formatRupiah(amount) }}
          <AppActionButton icon="copy" label="Salin Nominal" @click="copy(String(Math.round(amount)), 'Nominal')" />
        </dd>
      </template>
    </dl>

    <p v-if="payTo?.note" class="break-words border-t border-gray-100 px-4 py-2 text-xs text-gray-700">
      <span class="text-gray-400">Catatan Pembelian:</span> {{ payTo.note }}
    </p>
    <p v-if="payTo?.set_by" class="border-t border-gray-100 px-4 py-1.5 text-[11px] text-gray-400">
      Diisi {{ payTo.set_by }}<template v-if="payTo.set_at"> · {{ formatDateTime(payTo.set_at) }}</template>
    </p>
    <p v-else-if="effective === 'vendor_account'" class="border-t border-gray-100 px-4 py-1.5 text-[11px] text-gray-400">
      Rekening terdaftar vendor (diajukan sebelum Pembelian mengisi detail pembayaran).
    </p>
    <slot />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { formatRupiah, formatDateTime } from '@/utils/format.js'
import { PAY_TO_LABELS } from '@/utils/procurement.js'
import { useToastStore } from '@/stores/toast.js'

const props = defineProps({
  payTo: { type: Object, default: () => ({}) },
  amount: { type: Number, default: 0 },
})

const toast = useToastStore()
const ready = computed(() => !!props.payTo?.ready)
const returned = computed(() => !!props.payTo?.returned_at)
const effective = computed(() => props.payTo?.effective || '')
const methodLabel = computed(() => PAY_TO_LABELS[effective.value] || '')

function copy(text, label) {
  const value = String(text || '').trim()
  if (!value) return
  const done = () => toast.success(`${label} disalin.`)
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(value).then(done).catch(() => fallbackCopy(value) ? done() : toast.error('Gagal menyalin ke clipboard.'))
  } else if (fallbackCopy(value)) {
    done()
  } else {
    toast.error('Gagal menyalin ke clipboard.')
  }
}

// Clipboard API butuh konteks aman (HTTPS); cadangan untuk peramban lama.
function fallbackCopy(value) {
  try {
    const ta = document.createElement('textarea')
    ta.value = value
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
</script>
