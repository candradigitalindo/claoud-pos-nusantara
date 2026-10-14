<!--
  PurchasePickupSection.vue — pengambilan barang oleh petugas logistik.

  Barang pengadaan bisa diantar vendor langsung atau diambil petugas logistik.
  Pengambilan dicatat di sini (foto + waktu) dan berstatus "dalam pengantaran"
  sampai tempat penerimaan (Gudang Induk / bagian aset) menerimanya lewat Serah
  Terima dan memilih pengambilan itu sebagai pengantarnya.
-->
<template>
  <div class="rounded-xl border border-gray-200 bg-white p-3 sm:p-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="flex items-center gap-1.5 text-sm font-semibold text-gray-800">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><path d="M15 18H9"/><path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.624l-3.48-4.35A1 1 0 0 0 17.52 8H14"/><circle cx="17" cy="18" r="2"/><circle cx="7" cy="18" r="2"/></svg>
        Pengambilan Barang <span class="font-normal text-gray-400">({{ pickups.length }})</span>
      </p>
      <button v-if="canPickup && allowed && !showForm" type="button" @click="openForm"
        class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-blue-700">
        Catat Pengambilan
      </button>
    </div>

    <ul v-if="pickups.length" class="mt-2 divide-y divide-gray-100">
      <li v-for="p in pickups" :key="p.id" class="flex flex-wrap items-start justify-between gap-2 py-2 text-xs">
        <div class="min-w-0">
          <p class="font-medium text-gray-900">Diambil {{ p.picked_by || '-' }} · {{ formatDateTime(p.picked_at) }}</p>
          <p v-if="p.notes" class="break-words text-gray-500">{{ p.notes }}</p>
          <p v-if="p.received_at" class="text-gray-400">Diterima {{ p.received_by || '-' }} · {{ formatDateTime(p.received_at) }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <span v-if="p.received_at" class="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700">Sudah diterima</span>
          <span v-else class="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700">Dalam pengantaran</span>
          <a v-if="safeUrl(p.photo_url)" :href="safeUrl(p.photo_url)" target="_blank" rel="noopener noreferrer" class="text-blue-600 hover:underline">Foto</a>
        </div>
      </li>
    </ul>
    <p v-else-if="!showForm" class="mt-1 text-xs text-gray-400">
      Belum ada pengambilan. Barang yang diantar vendor langsung diterima tanpa langkah ini.
    </p>

    <div v-if="showForm" class="mt-3 space-y-3 rounded-lg border border-blue-100 bg-blue-50/40 p-3">
      <PhotoCapture v-model="form.photo_url" label="Foto Barang Saat Diambil" hint="Ambil foto barang di tempat vendor sebelum diangkut." />
      <AppInput v-model="form.notes" label="Catatan (opsional)" placeholder="mis. 25 kg dalam 3 box, struk dibawa" />
      <div class="flex justify-end gap-2">
        <AppButton variant="secondary" :disabled="busy" @click="showForm = false">Batal</AppButton>
        <AppButton :loading="busy" @click="save">Simpan Pengambilan</AppButton>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { formatDateTime } from '@/utils/format.js'
import { safeUrl } from '@/utils/safeUrl.js'
import { useToastStore } from '@/stores/toast.js'
import { RECEIVABLE_PURCHASE_STATUSES } from '@/utils/procurement.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppInput from '@/components/ui/AppInput.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const props = defineProps({
  doc: { type: Object, required: true },
  // Petugas logistik (izin Pengambilan Barang) jenis pengajuan ini.
  canPickup: { type: Boolean, default: false },
})
const emit = defineEmits(['changed'])

const toast = useToastStore()
const busy = ref(false)
const showForm = ref(false)
const form = ref({ photo_url: '', notes: '' })

const pickups = computed(() => props.doc?.pickups || [])
// Sama dengan server: sudah disetujui, bukan jasa, belum diterima seluruhnya.
const allowed = computed(() => {
  const d = props.doc || {}
  return RECEIVABLE_PURCHASE_STATUSES.includes(d.status) && d.request_type !== 'jasa' && d.receipt_status !== 'received'
})

function openForm() {
  form.value = { photo_url: '', notes: '' }
  showForm.value = true
}

async function save() {
  if (!form.value.photo_url) { toast.error('Foto barang saat diambil wajib diunggah.'); return }
  busy.value = true
  try {
    await purchaseApi.addPickup(props.doc.id, { photo_url: form.value.photo_url, notes: form.value.notes.trim() })
    toast.success('Pengambilan dicatat — barang dalam pengantaran.')
    showForm.value = false
    emit('changed')
  } catch (err) {
    toast.error(err?.message || 'Gagal mencatat pengambilan.')
  } finally {
    busy.value = false
  }
}
</script>
