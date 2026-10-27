<template>
  <div class="rupiah-input min-w-0 flex flex-col gap-1" :class="$attrs.class" :style="$attrs.style">
    <label v-if="label" :for="uid" class="text-sm font-medium text-gray-700">{{ label }}</label>
    <div class="relative min-w-0">
      <span aria-hidden="true" class="absolute left-3 top-1/2 -translate-y-1/2 text-xs text-gray-400 pointer-events-none select-none">Rp</span>
      <input
        ref="inputEl"
        v-bind="inputAttrs()"
        :id="uid"
        type="text"
        inputmode="decimal"
        :value="text"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        :required="required"
        :min="min"
        :max="max"
        :step="step"
        :aria-invalid="error ? true : $attrs['aria-invalid']"
        :aria-describedby="[$attrs['aria-describedby'], error ? `${uid}-error` : null].filter(Boolean).join(' ') || undefined"
        class="w-full min-w-0 rounded-lg border py-2 pr-3 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500 disabled:opacity-60 disabled:cursor-not-allowed"
        :class="[error ? 'border-red-400 bg-red-50' : 'border-gray-300 bg-white', inputClass]"
        style="padding-left: 2.25rem"
        @focus="focused = true"
        @blur="onBlur"
        @beforeinput="onBeforeInput"
        @paste="onPaste"
        @input="onInput"
        @compositionend="onInput"
      />
    </div>
    <p v-if="error" :id="`${uid}-error`" class="text-xs text-red-600">{{ error }}</p>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, useAttrs, useId, watch } from 'vue'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: [Number, String], default: 0 },
  label: { type: String, default: '' },
  error: { type: String, default: '' },
  id: { type: String, default: '' },
  placeholder: { type: String, default: '0' },
  inputClass: { type: [String, Array, Object], default: '' },
  disabled: Boolean,
  readonly: Boolean,
  required: Boolean,
  min: { type: [Number, String], default: 0 },
  max: { type: [Number, String], default: undefined },
  step: { type: [Number, String], default: 'any' },
})
const emit = defineEmits(['update:modelValue'])
const attrs = useAttrs()
const generatedId = `rupiah-${useId()}`
const uid = computed(() => props.id || generatedId)
const inputEl = ref(null)
const focused = ref(false)
const text = ref(formatValue(props.modelValue))
let numberValidator

// Layout classes belong to the wrapper; native attributes/events belong to the input.
function inputAttrs() {
  const { class: wrapperClass, style: wrapperStyle, ...input } = attrs
  return input
}

function formatValue(value, emptyZero = true) {
  const number = Number(value)
  if (value == null || value === '' || !Number.isFinite(number) || (emptyZero && number === 0)) return ''
  // HPP/unit conversions may already have more than two decimal places.
  return number.toLocaleString('id-ID', { maximumFractionDigits: 20 })
}

function formatText(value) {
  const clean = value.replace(/−/g, '-').replace(/[^\d,-]/g, '')
  const negative = clean.startsWith('-') ? '-' : ''
  const parts = clean.replace(/-/g, '').split(',')
  const integer = parts[0].replace(/^0+(?=\d)/, '') || (parts.length > 1 ? '0' : '')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return negative + grouped + (parts.length > 1 ? ',' + parts.slice(1).join('') : '')
}

function numericValue(value) {
  return Number(value.replace(/\./g, '').replace(',', '.'))
}

function pastedText(value) {
  const clean = value.replace(/−/g, '-').replace(/[^\d.,-]/g, '')
  // Keep Indonesian grouping (1.234), but also accept ungrouped decimal numbers
  // copied from older number inputs (1234.56) or spreadsheets (1,234.56).
  if (clean.includes(',') && clean.lastIndexOf('.') > clean.lastIndexOf(',')) {
    return clean.replace(/,/g, '').replace('.', ',')
  }
  if (!clean.includes(',') && /^-?\d*\.\d*$/.test(clean) && !/^-?[1-9]\d{0,2}\.\d{3}$/.test(clean)) {
    return clean.replace('.', ',')
  }
  return clean
}

function insertText(el, value) {
  el.setRangeText(value, el.selectionStart, el.selectionEnd, 'end')
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

function onPaste(event) {
  if (props.disabled || props.readonly) return
  const value = event.clipboardData?.getData('text')
  if (!value) return
  event.preventDefault()
  insertText(event.target, pastedText(value))
}

function caretAfterTokens(value, tokens) {
  if (!tokens) return 0
  let seen = 0
  for (let i = 0; i < value.length; i++) {
    if (/[\d,-]/.test(value[i]) && ++seen === tokens) return i + 1
  }
  return value.length
}

function onBeforeInput(event) {
  const el = event.target
  const start = el.selectionStart
  // Some decimal keyboards send a dot even with lang="id". Grouping is added
  // automatically; a typed decimal key is displayed as a comma.
  if (event.inputType === 'insertText' && event.data === '.') {
    event.preventDefault()
    const remaining = el.value.slice(0, start) + el.value.slice(el.selectionEnd)
    if (!remaining.includes(',')) insertText(el, ',')
    return
  }
  if (start !== el.selectionEnd) return
  // Deleting a grouping dot also deletes the adjacent digit, instead of restoring
  // the dot and leaving Backspace/Delete apparently unresponsive.
  if (event.inputType === 'deleteContentBackward' && el.value[start - 1] === '.') {
    el.setSelectionRange(start - 2, start)
  } else if (event.inputType === 'deleteContentForward' && el.value[start] === '.') {
    el.setSelectionRange(start, start + 2)
  }
}

function onInput(event) {
  if (event.isComposing) return
  const el = event.target
  if (event.inputType === 'insertReplacementText' || (event.data?.length > 1 && event.data === el.value)) {
    const normalized = pastedText(el.value)
    el.value = normalized
    el.setSelectionRange(normalized.length, normalized.length)
  }
  const prefix = el.value.slice(0, el.selectionStart).replace(/−/g, '-').replace(/[^\d,-]/g, '')
  // A leading comma adds a zero (",5" -> "0,5"); keep the caret after it.
  const tokens = prefix.length + (/^-?,/.test(prefix) ? 1 : 0)
  text.value = formatText(el.value)
  el.value = text.value
  const caret = caretAfterTokens(text.value, tokens)
  el.setSelectionRange(caret, caret)
  const value = numericValue(text.value)
  emit('update:modelValue', Number.isFinite(value) ? value : 0)
  validate()
}

function onBlur() {
  focused.value = false
  if (text.value) text.value = formatValue(numericValue(text.value), false)
  validate()
}

function validate() {
  const el = inputEl.value
  if (!el) return
  el.setCustomValidity('')
  if (!text.value || props.disabled || props.readonly) return
  const value = numericValue(text.value)
  if (!Number.isFinite(value)) {
    el.setCustomValidity('Masukkan nominal rupiah yang valid.')
    return
  }
  // Text inputs ignore native numeric constraints. Reuse the browser's number
  // validation without submitting a second input or losing locale formatting.
  numberValidator ||= el.ownerDocument.createElement('input')
  numberValidator.type = 'number'
  numberValidator.min = props.min == null ? '' : String(props.min)
  numberValidator.max = props.max == null ? '' : String(props.max)
  numberValidator.step = String(props.step)
  numberValidator.value = String(value)
  el.setCustomValidity(numberValidator.validationMessage)
}

watch(() => props.modelValue, value => {
  // Keep a trailing comma/zero and the caret while the parent accepts the edit.
  if (focused.value && Number(value) === numericValue(text.value)) return
  text.value = formatValue(value)
})
watch([text, () => props.min, () => props.max, () => props.step, () => props.disabled, () => props.readonly], validate, { flush: 'post' })
onMounted(validate)
defineExpose({ focus: () => inputEl.value?.focus() })
</script>
