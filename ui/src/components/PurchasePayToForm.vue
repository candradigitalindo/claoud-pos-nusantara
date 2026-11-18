<!--
  PurchasePayToForm — isian tujuan pembayaran oleh bagian Pembelian
  (keputusan owner 19 Nov 2026). Cermin aturan server di
  services/purchase_payto.go / utils/procurement.js → payToFormError:
    - Rekening Vendor Terdaftar: dari data Vendor, tidak bisa diketik;
    - Virtual Account Marketplace: VA dari checkout satu pesanan;
    - Tunai / Cara Bayar Lain: hanya untuk vendor tanpa rekening terdaftar.
  v-model = objek form dari newPayToForm(); payTo = pay_to dokumen (rekening
  terdaftar vendor saat ini ada di vendor_*).
-->
<template>
  <div class="space-y-3">
    <fieldset>
      <legend class="mb-1.5 block text-sm font-medium text-gray-700">Cara Bayar ke Vendor</legend>
      <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
        <label v-for="opt in options" :key="opt.value" class="payto-opt"
          :class="{ 'payto-opt--on': model.method === opt.value, 'payto-opt--off': opt.disabled }">
          <input v-model="model.method" type="radio" :value="opt.value" :disabled="opt.disabled" class="sr-only" />
          <span class="text-sm font-semibold text-gray-800">{{ opt.label }}</span>
          <span class="text-[11px] leading-snug text-gray-500">{{ opt.hint }}</span>
        </label>
      </div>
    </fieldset>

    <div v-if="model.method === 'vendor_account'" class="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs text-blue-900">
      <p class="font-semibold">{{ payTo?.vendor_bank_name || 'Bank' }} · <span class="font-mono">{{ payTo?.vendor_account_number }}</span></p>
      <p>a.n. {{ payTo?.vendor_account_holder || '-' }}</p>
      <p class="mt-1 text-[11px] leading-snug text-blue-700">Diambil dari data Vendor. Bila rekeningnya berubah, perbarui di menu Vendor — nomor rekening tidak bisa diketik di sini.</p>
    </div>

    <div v-else-if="model.method === 'marketplace_va'" class="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2">
      <p class="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs text-blue-800 sm:col-span-2">
        Salin dari halaman checkout pesanan ini. Nomor VA hanya berlaku untuk satu transaksi — bila kedaluwarsa, ubah detailnya dengan VA baru.
      </p>
      <AppInput v-model="model.marketplace" label="Marketplace *" placeholder="Shopee, Tokopedia, dll." maxlength="100" />
      <AppInput v-model="model.order_number" label="Nomor Pesanan *" placeholder="Nomor pesanan marketplace" maxlength="100" />
      <AppInput v-model="model.bank_name" label="Bank Virtual Account *" placeholder="Contoh: BCA, Mandiri, BRI" maxlength="100" />
      <AppInput v-model="model.account_number" label="Nomor Virtual Account *" placeholder="Nomor VA dari checkout" inputmode="numeric" maxlength="50" />
      <div class="sm:col-span-2">
        <AppInput v-model="model.account_holder" label="Atas Nama VA" placeholder="Nama penerima di checkout" maxlength="200" />
      </div>
    </div>

    <div v-else-if="model.method === 'manual'">
      <AppInput v-model="model.text" label="Cara Bayar *" placeholder="mis. Tunai saat barang diantar" maxlength="300" />
      <p class="mt-1 text-[11px] leading-snug text-amber-700">
        Vendor ini belum punya rekening terdaftar. Keuangan memilih sumber dananya (mis. Kas Kasir Outlet).
      </p>
    </div>

    <div>
      <label :for="noteId" class="mb-1 block text-sm font-medium text-gray-700">
        Catatan untuk Keuangan <span class="font-normal text-gray-400">(opsional)</span>
      </label>
      <textarea :id="noteId" v-model="model.note" rows="2" maxlength="1000"
        class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500"
        placeholder="mis. VA kedaluwarsa pukul 15.00 — mohon dibayar hari ini"></textarea>
    </div>

    <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">{{ error }}</p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import AppInput from '@/components/ui/AppInput.vue'

// Objek form milik induk (ref reaktif) — field-nya diubah langsung oleh
// v-model di bawah, tanpa memantulkan salinan (lihat catatan loop gema
// v-model di editor RAB).
const model = defineModel({ type: Object, required: true })
const props = defineProps({
  payTo: { type: Object, default: () => ({}) },
  error: { type: String, default: '' },
})

const noteId = `payto-note-${Math.random().toString(36).slice(2, 9)}`
const hasAccount = computed(() => !!String(props.payTo?.vendor_account_number || '').trim())

const options = computed(() => [
  {
    value: 'vendor_account', label: 'Rekening Vendor',
    hint: hasAccount.value ? 'Transfer ke rekening terdaftar vendor' : 'Vendor belum punya rekening terdaftar',
    disabled: !hasAccount.value,
  },
  { value: 'marketplace_va', label: 'VA Marketplace', hint: 'Virtual account dari checkout pesanan', disabled: false },
  {
    value: 'manual', label: 'Tunai / Cara Lain',
    hint: hasAccount.value ? 'Vendor punya rekening — pakai rekeningnya' : 'Untuk vendor tanpa rekening',
    disabled: hasAccount.value,
  },
])
</script>

<style scoped>
.payto-opt {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  cursor: pointer;
  border-radius: 0.5rem;
  border: 1px solid rgb(209 213 219);
  background: white;
  padding: 0.5rem 0.75rem;
  transition: border-color 0.15s, background-color 0.15s;
}
.payto-opt:hover { border-color: rgb(52 211 153); }
.payto-opt--on {
  border-color: rgb(16 185 129);
  background: rgb(236 253 245);
  box-shadow: 0 0 0 1px rgb(16 185 129);
}
.payto-opt--off {
  cursor: not-allowed;
  opacity: 0.55;
}
.payto-opt--off:hover { border-color: rgb(209 213 219); }
.payto-opt:focus-within { outline: 2px solid rgb(16 185 129); outline-offset: 2px; }
</style>
