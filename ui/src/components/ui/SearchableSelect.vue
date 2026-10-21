<!-- Keep native option slots, typed values, change events and form validation;
     the visible control is the shared searchable picker. -->
<template>
  <div class="searchable-select" :class="$attrs.class" :style="$attrs.style">
  <SearchSelect ref="picker" v-bind="pickerAttrs()" native-trigger :model-value="selectedIndex"
    :options="options" value-key="index" label-key="label" :disabled="disabled"
    :aria-required="required || undefined" :aria-invalid="invalid || $attrs['aria-invalid']"
    @update:model-value="choose" />
  <select ref="native" v-model="selection" class="searchable-select-source" tabindex="-1" aria-hidden="true"
    :name="name" :form="form" :required="required" :disabled="disabled"
    @change="changed" @invalid.prevent="onInvalid">
    <slot />
  </select>
  </div>
</template>

<script setup>
import { computed, ref, useAttrs, onMounted, onUpdated, onBeforeUnmount, nextTick } from 'vue'
import SearchSelect from './SearchSelect.vue'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { default: undefined },
  value: { default: undefined },
  modelModifiers: { type: Object, default: () => ({}) },
  disabled: Boolean,
  required: Boolean,
  name: String,
  form: String,
})
const emit = defineEmits(['update:modelValue', 'change', 'input'])
const attrs = useAttrs()
const pickerAttrs = () => Object.fromEntries(Object.entries(attrs).filter(([key]) => !['class', 'style'].includes(key)))
const native = ref(null)
const picker = ref(null)
const selectedIndex = ref(-1)
const options = ref([])
const invalid = ref(false)
const selection = computed({
  get: () => props.modelValue !== undefined ? props.modelValue : props.value,
  set: value => {
    if (props.modelModifiers.number && typeof value === 'string') {
      const number = parseFloat(value)
      if (!Number.isNaN(number)) value = number
    }
    emit('update:modelValue', value)
  },
})
function sync() {
  if (!native.value) return
  const next = Array.from(native.value.options, (option, index) => ({
    index, label: option.label, disabled: option.disabled || !!option.closest('optgroup')?.disabled,
    hidden: option.hidden || !!option.closest('optgroup')?.hidden,
  }))
  if (JSON.stringify(next) !== JSON.stringify(options.value)) options.value = next
  selectedIndex.value = native.value.selectedIndex
  if (native.value.validity.valid) invalid.value = false
}
function choose(index) {
  const option = native.value?.options[index]
  if (!option || option.disabled || option.closest('optgroup')?.disabled || native.value.matches(':disabled')) return
  native.value.selectedIndex = index
  const input = new Event('input', { bubbles: true })
  native.value.dispatchEvent(input)
  emit('input', input)
  native.value.dispatchEvent(new Event('change', { bubbles: true }))
}
function changed(event) {
  sync()
  emit('change', event)
  nextTick(sync)
}
function onInvalid() {
  invalid.value = true
  picker.value?.show()
}
let resetTimer
function onReset() {
  resetTimer = setTimeout(() => {
    if (!native.value) return
    const option = native.value.selectedOptions[0]
    selection.value = option && ('_value' in option ? option._value : option.value)
    sync()
  }, 0)
}
let ownerForm
onMounted(() => { sync(); ownerForm = native.value?.form; ownerForm?.addEventListener('reset', onReset) })
onUpdated(sync)
onBeforeUnmount(() => { clearTimeout(resetTimer); ownerForm?.removeEventListener('reset', onReset) })
</script>

<style scoped>
.searchable-select { display: inline-flex; align-items: center; min-width: 0; max-width: 100%; vertical-align: middle; }
.searchable-select-source { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); opacity: 0; pointer-events: none; }
</style>
