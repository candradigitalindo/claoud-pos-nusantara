<!--
  BudgetBar.vue — bar serapan RAB sebuah projek.

  Dua lapis dalam satu track: komitmen (nilai yang sudah dikunci lewat
  pengajuan yang masih hidup) sebagai dasar, terbayar (uang yang benar-benar
  keluar) sebagai lapis gelap di atasnya. Track memerah begitu komitmen
  melewati RAB.
-->
<template>
  <div>
    <div class="h-1.5 w-full overflow-hidden rounded-full" :class="over ? 'bg-red-100' : 'bg-gray-100'">
      <div class="relative h-full" :style="{ width: pct(committed) }">
        <div class="absolute inset-0 rounded-full" :class="over ? 'bg-red-400' : 'bg-blue-400'" />
        <div class="absolute inset-y-0 left-0 rounded-full" :class="over ? 'bg-red-600' : 'bg-emerald-500'"
          :style="{ width: paidWithinCommitted }" />
      </div>
    </div>
    <p class="mt-1 flex flex-wrap gap-x-2 gap-y-0.5 text-[10px] leading-tight text-gray-500">
      <span :class="over ? 'font-semibold text-red-600' : ''">Komitmen {{ formatRupiah(committed) }}</span>
      <span class="text-gray-300">·</span>
      <span>Terbayar {{ formatRupiah(paid) }}</span>
      <span v-if="budget > 0" class="text-gray-400">({{ absorbed }}% RAB)</span>
      <span v-if="estimated > 0" class="text-amber-600">· {{ formatRupiah(estimated) }} masih estimasi HPS</span>
      <span v-if="over" class="font-semibold text-red-600">melebihi RAB</span>
    </p>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatRupiah } from '@/utils/format.js'

const props = defineProps({
  budget:    { type: Number, default: 0 },
  committed: { type: Number, default: 0 },
  paid:      { type: Number, default: 0 },
  // Bagian dari committed yang harganya belum diisi purchasing (masih HPS).
  estimated: { type: Number, default: 0 },
})

const over = computed(() => props.budget > 0 && props.committed > props.budget)
const absorbed = computed(() => props.budget > 0 ? Math.round(props.committed / props.budget * 100) : 0)

// Lebar bar relatif terhadap RAB; kalau RAB belum diisi, komitmen jadi acuan
// supaya bar tetap bermakna alih-alih kosong melompong.
const base = computed(() => props.budget > 0 ? props.budget : props.committed)

function pct(v) {
  if (base.value <= 0) return '0%'
  return Math.min(100, Math.max(0, v / base.value * 100)) + '%'
}
// Lapis "terbayar" digambar di dalam lapis komitmen, jadi porsinya dihitung
// terhadap komitmen — bukan terhadap RAB.
const paidWithinCommitted = computed(() => {
  if (props.committed <= 0) return '0%'
  return Math.min(100, props.paid / props.committed * 100) + '%'
})
</script>
