<!--
  MultiSearchSelect.vue — Dropdown with inline search/filter and MULTI-SELECT
  ───────────────────────────────────────────────────────────────
  Props:
    modelValue   : Array<string>       — v-model binding (list of selected IDs)
    options      : Array<{ id, name }> — list of options
    placeholder  : string              — placeholder when nothing selected
    searchPlaceholder : string         — placeholder for search input
    disabled     : boolean
    labelKey     : string              — key for display text (default: 'name')
    valueKey     : string              — key for value (default: 'id')

  Emits: update:modelValue

  Usage:
    <MultiSearchSelect
      v-model="form.outlet_ids"
      :options="outlets"
      placeholder="Pilih outlet…"
      searchPlaceholder="Cari outlet…"
    />
-->
<template>
  <div class="ss" ref="rootEl" :class="{ 'ss--disabled': disabled }">
    <!-- Trigger button -->
    <button
      type="button"
      class="ss-trigger"
      :disabled="disabled"
      @click.stop="toggle"
    >
      <div class="ss-values-wrap">
        <template v-if="selectedOptions.length">
          <div v-for="opt in selectedOptions" :key="opt[valueKey]" class="ss-badge">
            {{ opt[labelKey] }}
            <span class="ss-badge-del" @click.stop="deselect(opt[valueKey])">&times;</span>
          </div>
        </template>
        <span v-else class="ss-placeholder">{{ placeholder }}</span>
      </div>
      <svg class="ss-chevron" :class="{ 'ss-chevron--open': open }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </button>

    <!-- Dropdown panel -->
    <Teleport to="body">
      <div v-if="open" class="ss-backdrop" @click="close"></div>
      <div v-if="open" ref="panelEl" class="ss-panel" :style="panelStyle">
        <div class="ss-search-box">
          <svg class="ss-search-ico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          <input
            ref="searchInputEl"
            v-model="query"
            class="ss-search-input"
            :placeholder="searchPlaceholder"
            @keydown.escape="close"
          />
        </div>
        <ul class="ss-options" v-if="filtered.length">
          <li
            v-for="opt in filtered"
            :key="opt[valueKey]"
            class="ss-option"
            :class="{ 'ss-option--active': isSelected(opt[valueKey]) }"
            @click="toggleSelect(opt)"
          >
            {{ opt[labelKey] }}
            <svg v-if="isSelected(opt[valueKey])" class="ss-check" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="20 6 9 17 4 12"/>
            </svg>
          </li>
        </ul>
        <div v-else class="ss-empty">Tidak ditemukan</div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  modelValue:        { type: Array,   default: () => [] },
  options:           { type: Array,   default: () => [] },
  placeholder:       { type: String,  default: 'Pilih…' },
  searchPlaceholder: { type: String,  default: 'Cari…' },
  disabled:          { type: Boolean, default: false },
  labelKey:          { type: String,  default: 'name' },
  valueKey:          { type: String,  default: 'id' },
})

const emit = defineEmits(['update:modelValue', 'change'])

const rootEl       = ref(null)
const panelEl      = ref(null)
const searchInputEl = ref(null)
const open         = ref(false)
const query        = ref('')
const panelStyle   = ref({})

const selectedOptions = computed(() => {
  return props.options.filter(o => props.modelValue.includes(o[props.valueKey]))
})

const filtered = computed(() => {
  const q = query.value.toLowerCase().trim()
  if (!q) return props.options
  return props.options.filter(o =>
    String(o[props.labelKey]).toLowerCase().includes(q)
  )
})

function isSelected(val) {
  return props.modelValue.includes(val)
}

function positionPanel() {
  if (!rootEl.value) return
  const rect = rootEl.value.getBoundingClientRect()
  const spaceBelow = window.innerHeight - rect.bottom
  const dropUp = spaceBelow < 220 && rect.top > 220

  panelStyle.value = {
    position: 'fixed',
    left: rect.left + 'px',
    width: rect.width + 'px',
    zIndex: 9999,
    ...(dropUp
      ? { bottom: (window.innerHeight - rect.top + 4) + 'px' }
      : { top: (rect.bottom + 4) + 'px' }
    ),
  }
}

function toggle() {
  if (props.disabled) return
  if (open.value) { close(); return }
  query.value = ''
  open.value = true
  positionPanel()
  nextTick(() => {
    positionPanel()
    searchInputEl.value?.focus()
  })
}

function close() {
  open.value = false
  query.value = ''
}

function toggleSelect(opt) {
  const val = opt[props.valueKey]
  let newValue = [...props.modelValue]
  if (newValue.includes(val)) {
    newValue = newValue.filter(v => v !== val)
  } else {
    newValue.push(val)
  }
  emit('update:modelValue', newValue)
  emit('change', newValue)
}

function deselect(val) {
  const newValue = props.modelValue.filter(v => v !== val)
  emit('update:modelValue', newValue)
  emit('change', newValue)
}

function onScrollOrResize(event) {
  if (event?.target && panelEl.value?.contains(event.target)) return
  if (open.value) close()
}

onMounted(() => {
  window.addEventListener('scroll', onScrollOrResize, true)
  window.addEventListener('resize', onScrollOrResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScrollOrResize, true)
  window.removeEventListener('resize', onScrollOrResize)
})
</script>

<style scoped>
.ss { position: relative; width: 100%; }
.ss--disabled { opacity: .55; pointer-events: none; }

.ss-trigger {
  display: flex; align-items: center; justify-content: space-between; gap: .5rem;
  width: 100%; min-height: 2.45rem; padding: .35rem .75rem; border-radius: .65rem;
  background: rgba(255,255,255,.85); border: 1px solid rgba(0,0,0,.12);
  font-size: .82rem; color: #0f2d1d; cursor: pointer; text-align: left;
  transition: border-color .15s, box-shadow .15s;
}
.ss-trigger:hover { border-color: rgba(0,0,0,.2); }
.ss-trigger:focus { border-color: rgba(45,143,86,.4); box-shadow: 0 0 0 3px rgba(45,143,86,.1); outline: none; }

.ss-values-wrap { flex: 1; display: flex; flex-wrap: wrap; gap: .35rem; }

.ss-badge {
  display: inline-flex; align-items: center; gap: .25rem;
  padding: .15rem .45rem; border-radius: .4rem;
  background: rgba(45,143,86,.1); color: #1b7a42;
  font-size: .75rem; font-weight: 600;
}
.ss-badge-del {
  display: flex; align-items: center; justify-content: center;
  width: 14px; height: 14px; border-radius: 50%;
  background: rgba(0,0,0,.1); color: #1a3d2a;
  font-size: 10px; cursor: pointer; transition: background .1s;
}
.ss-badge-del:hover { background: rgba(0,0,0,.2); }

.ss-placeholder { color: #8ca898; }
.ss-chevron { flex-shrink: 0; color: #6c8a7a; transition: transform .15s; }
.ss-chevron--open { transform: rotate(180deg); }

.ss-backdrop { position: fixed; inset: 0; z-index: 9998; }

.ss-panel {
  background: #fff; border-radius: .75rem;
  border: 1px solid rgba(0,0,0,.1);
  box-shadow: 0 8px 30px rgba(0,0,0,.12), 0 2px 8px rgba(0,0,0,.06);
  overflow: hidden;
  max-height: 260px; display: flex; flex-direction: column;
}

.ss-search-box {
  display: flex; align-items: center; gap: .4rem;
  padding: .5rem .6rem; border-bottom: 1px solid rgba(0,0,0,.08);
  background: #fafcfb;
}
.ss-search-ico { flex-shrink: 0; color: #8ca898; }
.ss-search-input {
  flex: 1; border: none; outline: none; background: transparent;
  font-size: .78rem; color: #0f2d1d;
}
.ss-search-input::placeholder { color: #b0c4b8; }

.ss-options {
  list-style: none; margin: 0; padding: .3rem 0;
  overflow-y: auto; flex: 1;
}
.ss-option {
  display: flex; align-items: center; justify-content: space-between;
  padding: .42rem .75rem; font-size: .8rem; color: #1a3d2a;
  cursor: pointer; transition: background .1s;
}
.ss-option:hover { background: rgba(45,143,86,.07); }
.ss-option--active { color: #1b7a42; font-weight: 600; background: rgba(45,143,86,.08); }
.ss-check { flex-shrink: 0; color: #2d8f56; }

.ss-empty {
  padding: .75rem; text-align: center;
  font-size: .78rem; color: #8ca898;
}
</style>
