<!--
  PurchaseUrgencyFields.vue — pilihan Tingkat Kebutuhan (Rutin / Prioritas /
  Urgent) dan Tanggal Dibutuhkan di form pengajuan. Aturan sama dengan server
  (normalizeUrgency): prioritas & urgent wajib bertanggal, urgent paling lambat
  7 hari lagi dan wajib menjelaskan dampaknya di Catatan.
-->
<template>
  <div class="space-y-2">
    <div>
      <p class="mb-1 text-sm font-medium text-gray-700">Tingkat Kebutuhan</p>
      <div class="grid grid-cols-3 gap-1 rounded-lg bg-gray-100 p-1" role="radiogroup" aria-label="Tingkat kebutuhan">
        <button v-for="o in URGENCY_OPTIONS" :key="o.value" type="button" role="radio"
          :aria-checked="urgency === o.value"
          :class="['rounded-md px-2 py-1.5 text-xs font-semibold transition-colors',
            urgency === o.value ? activeCls(o.value) : 'text-gray-600 hover:bg-white/70']"
          @click="setUrgency(o.value)">
          {{ o.label }}
        </button>
      </div>
      <p class="mt-1 text-[11px] text-gray-500">{{ currentHint }}</p>
    </div>

    <div v-if="urgency !== 'rutin'" class="flex flex-col gap-1">
      <label class="text-sm font-medium text-gray-700">Tanggal Dibutuhkan <span class="text-red-500">*</span></label>
      <input :value="needBy" type="date" :min="today" :max="urgency === 'urgent' ? urgentMax : undefined"
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm hover:border-gray-400 focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/30 sm:w-56"
        @input="$emit('update:needBy', $event.target.value)" />
    </div>

    <div v-if="urgency === 'urgent'" class="rounded-lg border border-rose-200 bg-rose-50 p-2.5 text-[11px] text-rose-800">
      <p>Jelaskan dampaknya di <b>Catatan</b> (mis. mesin mati, operasional terhenti). Pengajuan URGENT tetap diverifikasi &amp; disetujui dulu, tetapi tampil paling atas dan diingatkan lebih cepat.</p>
      <p v-if="canEmergency" class="mt-1.5">
        Nilainya ≤ {{ formatRupiah(emergencyMax) }} dan harus dibeli hari ini?
        <button type="button" class="font-semibold underline hover:text-rose-950" @click="$emit('emergency')">Pakai Pembelian Darurat</button>
        — kas diambil dari laci kasir outlet, disahkan sesudahnya.
      </p>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { URGENCY_OPTIONS, URGENT_MAX_LEAD_DAYS } from '@/utils/procurement.js'
import { formatRupiah, todayDateString } from '@/utils/format.js'

const props = defineProps({
  urgency: { type: String, default: 'rutin' },
  needBy: { type: String, default: '' },
  canEmergency: { type: Boolean, default: false },
  emergencyMax: { type: Number, default: 500000 },
})
const emit = defineEmits(['update:urgency', 'update:needBy', 'emergency'])

const today = computed(() => todayDateString())
const urgentMax = computed(() => {
  const [y, m, d] = today.value.split('-').map(Number)
  const t = new Date(Date.UTC(y, m - 1, d + URGENT_MAX_LEAD_DAYS))
  return t.toISOString().slice(0, 10)
})
const currentHint = computed(() => URGENCY_OPTIONS.find(o => o.value === props.urgency)?.hint || '')

function activeCls(v) {
  if (v === 'urgent') return 'bg-rose-600 text-white shadow-sm'
  if (v === 'prioritas') return 'bg-amber-500 text-white shadow-sm'
  return 'bg-white text-gray-900 shadow-sm'
}

function setUrgency(v) {
  emit('update:urgency', v)
  // Urgent bawaannya hari ini; tanggal yang sudah lewat batas dikosongkan.
  if (v === 'urgent' && (!props.needBy || props.needBy > urgentMax.value)) emit('update:needBy', today.value)
  if (v === 'rutin') emit('update:needBy', '')
}
</script>
