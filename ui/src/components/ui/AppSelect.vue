<!--
  AppSelect.vue — Searchable dropdown with label and error support
  ───────────────────────────────────────────────────────────────
  Props:
    modelValue  : string | number       — v-model binding
    label       : string                — visible label above the select
    options     : Array<{ value, label }>
    error       : string                — red error message below
    placeholder : string                — empty/default option text
    disabled    : boolean

  Usage:
    <AppSelect
      v-model="form.printerType"
      label="Tipe Printer"
      :options="[{ value: 'kitchen', label: 'Kitchen' }, ...]"
      placeholder="Pilih tipe..."
    />
-->
<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="uid" class="text-sm font-medium text-gray-700">
      {{ label }}
    </label>

    <SearchableSelect
      :id="uid"
      :model-value="modelValue"
      :disabled="disabled"
      :class="[
        'w-full rounded-lg border px-3 py-2 text-sm shadow-sm bg-white transition-colors',
        'focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500',
        error
          ? 'border-red-400 bg-red-50'
          : 'border-gray-300 hover:border-gray-400',
        disabled ? 'opacity-60 cursor-not-allowed bg-gray-100' : '',
      ]"
      v-bind="$attrs"
      @update:model-value="$emit('update:modelValue', $event)"
    >
      <option v-if="placeholder" value="" disabled>
        {{ placeholder }}
      </option>
      <option
        v-for="opt in options"
        :key="opt.value"
        :value="opt.value"
        :disabled="opt.disabled"
      >
        {{ opt.label }}
      </option>
    </SearchableSelect>

    <p v-if="error" class="text-xs text-red-600">{{ error }}</p>
  </div>
</template>

<script setup>
import SearchableSelect from '@/components/ui/SearchableSelect.vue'
import { useId } from 'vue'

defineOptions({ inheritAttrs: false })

const props = defineProps({
  modelValue:  { type: [String, Number], default: '' },
  label:       { type: String,  default: '' },
  options:     { type: Array,   default: () => [] },
  error:       { type: String,  default: '' },
  placeholder: { type: String,  default: '' },
  disabled:    { type: Boolean, default: false },
})

defineEmits(['update:modelValue'])

const uid = `select-${useId()}`
</script>
