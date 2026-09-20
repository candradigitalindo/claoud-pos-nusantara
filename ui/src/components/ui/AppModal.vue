<!--
  AppModal.vue — Accessible dialog overlay
  ───────────────────────────────────────────────────────────────
  Props:
    modelValue  : boolean  — v-model controls open/close
    title       : string   — dialog title in the header
    size        : 'sm' | 'md' | 'lg' | 'xl'   default 'md'

  Slots:
    default   — modal body content
    footer    — action buttons (optional; hides footer row if empty)

  Usage:
    <AppModal v-model="showModal" title="Tambah Outlet">
      <AppInput ... />
      <template #footer>
        <AppButton variant="secondary" @click="showModal = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submit">Simpan</AppButton>
      </template>
    </AppModal>

  AI NOTE: Pressing Escape or clicking the backdrop calls close().
-->
<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-200"
      leave-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4"
      >
        <!-- Backdrop -->
        <div
          class="absolute inset-0 bg-black/50"
          @click="close"
          aria-hidden="true"
        />

        <!-- Dialog panel -->
        <div
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          :class="[
            'relative bg-white rounded-xl shadow-xl w-full min-w-0 flex flex-col max-h-[92vh] sm:max-h-[90vh]',
            SIZES[size],
          ]"
          @keydown.esc="close"
        >
          <!-- Header -->
          <div class="flex items-center justify-between gap-3 px-4 sm:px-6 py-3 sm:py-4 border-b border-gray-200 flex-shrink-0">
            <h2 :id="titleId" class="min-w-0 truncate text-base sm:text-lg font-semibold text-gray-900">
              {{ title }}
            </h2>
            <button
              class="shrink-0 text-gray-400 hover:text-gray-600 transition-colors"
              @click="close"
              aria-label="Tutup"
            >
              <svg class="w-5 h-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path d="M6.293 6.293a1 1 0 011.414 0L10 8.586l2.293-2.293a1 1 0 111.414 1.414L11.414 10l2.293 2.293a1 1 0 01-1.414 1.414L10 11.414l-2.293 2.293a1 1 0 01-1.414-1.414L8.586 10 6.293 7.707a1 1 0 010-1.414z" />
              </svg>
            </button>
          </div>

          <!-- Body -->
          <div class="overflow-y-auto px-4 sm:px-6 py-4 flex-1">
            <slot />
          </div>

          <!-- Footer -->
          <div
            v-if="$slots.footer"
            class="flex flex-wrap items-center justify-end gap-2 px-4 sm:px-6 py-3 sm:py-4 border-t border-gray-200 flex-shrink-0"
          >
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, required: true },
  title:      { type: String,  default: '' },
  size:       { type: String,  default: 'md' },
})

const emit = defineEmits(['update:modelValue'])

function close() {
  emit('update:modelValue', false)
}

const titleId = computed(() => `modal-title-${Math.random().toString(36).slice(2, 7)}`)

const SIZES = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-2xl',
  '2xl': 'max-w-4xl',
}
</script>
