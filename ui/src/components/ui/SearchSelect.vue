<template>
  <div class="ss" :class="[{ 'ss--native': nativeTrigger }, !nativeTrigger && $attrs.class]" :style="!nativeTrigger && $attrs.style">
    <button ref="triggerEl" v-bind="triggerAttrs()" type="button" role="combobox"
      class="ss-trigger" :class="{ 'searchable-select-trigger': nativeTrigger }"
      :disabled="disabled" :aria-expanded="open" :aria-controls="`${uid}-options`"
      aria-haspopup="listbox" @click.stop="toggle" @keydown="onTriggerKeydown">
      <span :class="selectedLabel ? 'ss-value' : 'ss-placeholder'">{{ selectedLabel || placeholder }}</span>
      <svg class="ss-chevron" :class="{ 'ss-chevron--open': open }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" aria-hidden="true"><polyline points="6 9 12 15 18 9" /></svg>
    </button>
    <Teleport to="body">
      <div v-if="open" class="ss-backdrop" @click="close(false)"></div>
      <div v-if="open" ref="panelEl" class="ss-panel" :style="panelStyle" @keydown="onPanelKeydown">
        <div class="ss-search-box">
          <input ref="searchInputEl" v-model="query" type="search" class="ss-search-input"
            :placeholder="searchPlaceholder" aria-label="Cari pilihan" autocomplete="off"
            role="combobox" :aria-controls="`${uid}-options`" aria-expanded="true"
            aria-autocomplete="list" :aria-activedescendant="activeIndex >= 0 ? `${uid}-option-${activeIndex}` : undefined" />
        </div>
        <ul :id="`${uid}-options`" class="ss-options" role="listbox" :aria-label="placeholder">
          <li v-for="(opt, index) in filtered" :id="`${uid}-option-${index}`" :key="index"
            class="ss-option" role="option" :aria-selected="sameValue(opt[valueKey], modelValue)" :aria-disabled="!!opt.disabled"
            :class="{ 'ss-option--active': sameValue(opt[valueKey], modelValue), 'ss-option--focused': index === activeIndex, 'ss-option--disabled': opt.disabled }"
            @pointermove="!opt.disabled && (activeIndex = index)" @click="select(opt)">
            <span>{{ opt[labelKey] }}</span><span v-if="sameValue(opt[valueKey], modelValue)" aria-hidden="true">✓</span>
          </li>
        </ul>
        <div v-if="!filtered.length" class="ss-empty" role="status">Tidak ditemukan</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, useAttrs, useId } from 'vue'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: [String, Number, Boolean, Object], default: '' },
  options: { type: Array, default: () => [] },
  placeholder: { type: String, default: 'Pilih…' },
  searchPlaceholder: { type: String, default: 'Cari…' },
  disabled: { type: Boolean, default: false },
  labelKey: { type: String, default: 'name' },
  valueKey: { type: String, default: 'id' },
  nativeTrigger: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'change'])
const attrs = useAttrs()
const uid = `ss-${useId()}`
const triggerEl = ref(null)
const panelEl = ref(null)
const searchInputEl = ref(null)
const open = ref(false)
const query = ref('')
const activeIndex = ref(-1)
const panelStyle = ref({})
const triggerAttrs = () => Object.fromEntries(Object.entries(attrs).filter(([key]) => props.nativeTrigger || !['class', 'style'].includes(key)))
const sameValue = (a, b) => Object.is(a, b)
const selectedLabel = computed(() => props.options.find(o => sameValue(o[props.valueKey], props.modelValue))?.[props.labelKey] ?? '')
const normalized = value => String(value ?? '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLocaleLowerCase().trim()
const filtered = computed(() => props.options.filter(o => !o.hidden && normalized(o[props.labelKey]).includes(normalized(query.value))))

function positionPanel() {
  if (!open.value) return
  const rect = triggerEl.value?.getBoundingClientRect()
  if (!rect) return
  const viewport = window.visualViewport
  const viewportTop = viewport?.offsetTop || 0
  const viewportLeft = viewport?.offsetLeft || 0
  const height = viewport?.height || window.innerHeight
  const width = viewport?.width || window.innerWidth
  const below = viewportTop + height - rect.bottom - 8
  const above = rect.top - viewportTop - 8
  const dropUp = below < 220 && above > below
  const panelWidth = Math.min(Math.max(rect.width, 220), width - 16)
  panelStyle.value = {
    width: `${panelWidth}px`,
    left: `${Math.max(viewportLeft + 8, Math.min(rect.left, viewportLeft + width - panelWidth - 8))}px`,
    maxHeight: `${Math.max(80, Math.min(300, dropUp ? above : below))}px`,
    ...(dropUp ? { bottom: `${window.innerHeight - rect.top + 4}px` } : { top: `${rect.bottom + 4}px` }),
  }
}
function resetActive() {
  const selected = filtered.value.findIndex(o => !o.disabled && sameValue(o[props.valueKey], props.modelValue))
  activeIndex.value = selected >= 0 ? selected : filtered.value.findIndex(o => !o.disabled)
}
async function show() {
  if (props.disabled || triggerEl.value?.matches(':disabled')) return
  query.value = ''
  open.value = true
  resetActive()
  positionPanel()
  await nextTick()
  searchInputEl.value?.focus({ preventScroll: true })
  positionPanel()
}
function close(restoreFocus = false) {
  open.value = false
  query.value = ''
  if (restoreFocus) triggerEl.value?.focus({ preventScroll: true })
}
function toggle() { open.value ? close(true) : show() }
function select(opt) {
  if (!opt || opt.disabled || props.disabled || triggerEl.value?.matches(':disabled')) return
  emit('update:modelValue', opt[props.valueKey])
  emit('change', opt[props.valueKey])
  close(true)
}
function moveActive(direction) {
  const enabled = filtered.value.map((o, i) => o.disabled ? -1 : i).filter(i => i >= 0)
  if (!enabled.length) return
  activeIndex.value = enabled[(enabled.indexOf(activeIndex.value) + direction + enabled.length) % enabled.length]
  nextTick(() => document.getElementById(`${uid}-option-${activeIndex.value}`)?.scrollIntoView({ block: 'nearest' }))
}
function onTriggerKeydown(event) {
  if (['ArrowDown', 'ArrowUp'].includes(event.key)) { event.preventDefault(); show() }
}
function onPanelKeydown(event) {
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true) }
  else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); moveActive(event.key === 'ArrowDown' ? 1 : -1) }
  else if (event.key === 'Enter') { event.preventDefault(); select(filtered.value[activeIndex.value]) }
  else if (event.key === 'Tab') close(true)
}
function onScroll(event) {
  if (!open.value || panelEl.value?.contains(event.target)) return
  close(false)
}
watch(filtered, resetActive)
watch(() => props.disabled, value => { if (value) close(false) })
onMounted(() => {
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', positionPanel)
  window.visualViewport?.addEventListener('resize', positionPanel)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', positionPanel)
  window.visualViewport?.removeEventListener('resize', positionPanel)
})
defineExpose({ show, focus: () => triggerEl.value?.focus() })
</script>

<style scoped>
.ss { position: relative; width: 100%; min-width: 0; }
.ss--native { display: contents; }
.ss-trigger { display: flex; align-items: center; justify-content: space-between; gap: .5rem; min-width: 0; max-width: 100%; cursor: pointer; text-align: left; }
.ss-trigger:not(.searchable-select-trigger) { width: 100%; padding: .45rem .75rem; border-radius: .65rem; background: rgba(255,255,255,.85); border: 1px solid rgba(0,0,0,.12); font-size: .82rem; color: #0f2d1d; }
.searchable-select-trigger { width: 100%; padding: 0; border: 0; background: transparent; color: inherit; font: inherit; }
.ss-trigger:disabled { opacity: .55; cursor: not-allowed; }
.ss-trigger:focus-visible { outline: 2px solid #5a9170; outline-offset: 2px; }
.ss-value, .ss-placeholder { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ss-placeholder { color: #8ca898; }
.ss-chevron { flex-shrink: 0; color: currentColor; opacity: .65; transition: transform .15s; }
.ss-chevron--open { transform: rotate(180deg); }
.ss-backdrop { position: fixed; inset: 0; z-index: 9998; }
.ss-panel { position: fixed; z-index: 9999; background: #fff; border-radius: .75rem; border: 1px solid rgba(0,0,0,.1); box-shadow: 0 8px 30px rgba(0,0,0,.12); overflow: hidden; display: flex; flex-direction: column; }
.ss-search-box { flex-shrink: 0; padding: .5rem .6rem; border-bottom: 1px solid rgba(0,0,0,.08); background: #fafcfb; }
.ss-search-input { display: block; width: 100%; min-width: 0; border: none; outline: none; background: transparent; font-size: .85rem; color: #0f2d1d; }
.ss-options { list-style: none; margin: 0; padding: .3rem 0; overflow-y: auto; overscroll-behavior: contain; flex: 1; min-height: 0; }
.ss-option { display: flex; align-items: center; justify-content: space-between; gap: .5rem; padding: .5rem .75rem; font-size: .82rem; color: #1a3d2a; cursor: pointer; overflow-wrap: anywhere; }
.ss-option--active { color: #1b7a42; font-weight: 600; }
.ss-option--focused, .ss-option:not(.ss-option--disabled):hover { background: #edf6f0; }
.ss-option--disabled { opacity: .45; cursor: not-allowed; }
.ss-empty { padding: .75rem; text-align: center; font-size: .82rem; color: #6c8a7a; }
@media (max-width: 640px) { .ss-search-input { font-size: 16px; } .ss-option { min-height: 44px; } }
</style>
