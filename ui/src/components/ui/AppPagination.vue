<!--
  AppPagination.vue — Numbered pagination bar
  ───────────────────────────────────────────────────────────────
  Mendukung DUA gaya pemakaian (banyak halaman memakai gaya kedua,
  yang dulu diam-diam tidak me-render apa pun):

  Gaya A (v-model + total item):
    <AppPagination v-model="page" :total="total" :perPage="20" />

  Gaya B (page + totalPages):
    <AppPagination :page="page" :total-pages="totalPages" @change="goPage" />
    <AppPagination :page="page" :totalPages="totalPages" @update:page="goPage" />
    <AppPagination v-model:page="page" :total-pages="totalPages" />

  Emits: update:modelValue, update:page, change — semuanya dipancarkan
  bersamaan saat pindah halaman, parent bebas dengarkan yang mana.

  AI NOTE: This component does NOT fetch data — the parent page is responsible
  for reacting to the emitted page number and calling the API.
-->
<template>
  <div v-if="pageCount > 1" class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between text-sm text-gray-600 mt-4">
    <!-- Info -->
    <span class="text-center sm:text-left">
      Halaman {{ current }} dari {{ pageCount }}
      <span v-if="total > 0" class="text-gray-400">({{ total }} data)</span>
    </span>

    <!-- Buttons -->
    <div class="flex flex-wrap items-center justify-center gap-1 sm:justify-end">
      <button
        :disabled="current <= 1"
        class="px-3 py-1.5 rounded-lg border border-gray-300 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        @click="go(current - 1)"
      >
        ‹ Prev
      </button>

      <button
        v-for="p in visiblePages"
        :key="p"
        :class="[
          'w-9 h-8 rounded-lg border transition-colors',
          p === current
            ? 'bg-brand-600 border-brand-600 text-white font-semibold'
            : 'border-gray-300 hover:bg-gray-50',
        ]"
        @click="go(p)"
      >
        {{ p }}
      </button>

      <button
        :disabled="current >= pageCount"
        class="px-3 py-1.5 rounded-lg border border-gray-300 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        @click="go(current + 1)"
      >
        Next ›
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  // Gaya A
  modelValue: { type: Number, default: 0 },
  total:      { type: Number, default: 0 },
  perPage:    { type: Number, default: 20 },
  // Gaya B
  page:       { type: Number, default: 0 },
  totalPages: { type: Number, default: 0 },
})

const emit = defineEmits(['update:modelValue', 'update:page', 'change'])

const current = computed(() => props.modelValue || props.page || 1)

const pageCount = computed(() => {
  if (props.totalPages > 0) return props.totalPages
  return Math.max(1, Math.ceil(props.total / props.perPage))
})

/** Show at most 7 page buttons centred around current page */
const visiblePages = computed(() => {
  const cur   = current.value
  const total = pageCount.value
  const window = 3
  const start = Math.max(1, cur - window)
  const end   = Math.min(total, cur + window)
  const pages = []
  for (let i = start; i <= end; i++) pages.push(i)
  return pages
})

function go(p) {
  if (p >= 1 && p <= pageCount.value && p !== current.value) {
    emit('update:modelValue', p)
    emit('update:page', p)
    emit('change', p)
  }
}
</script>
