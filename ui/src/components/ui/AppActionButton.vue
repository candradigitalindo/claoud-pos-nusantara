<!-- Actions on one record. Form submit/cancel buttons continue to use AppButton. -->
<template>
  <component :is="element" v-bind="destination"
    :type="isLink ? undefined : type" :disabled="isLink ? undefined : blocked"
    class="action-button" :class="`action-button--${tone || ACTION_TONES[icon] || 'neutral'}`"
    :title="label" :aria-label="label" :aria-disabled="blocked || undefined"
    :aria-busy="loading || undefined" :role="isLink && blocked ? 'link' : undefined" :tabindex="isLink && blocked ? -1 : undefined"
    @click="activate">
    <svg v-if="loading" class="action-button-icon animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true" focusable="false"><path d="M21 12a9 9 0 1 1-9-9" stroke-linecap="round" /></svg>
    <svg v-else class="action-button-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
      <path v-for="(path, index) in ACTION_ICONS[icon]" :key="index" :d="path" />
    </svg>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ACTION_ICONS, ACTION_TONES } from '@/utils/actionIcons'

const props = defineProps({
  icon: { type: String, required: true, validator: icon => Object.hasOwn(ACTION_ICONS, icon) },
  label: { type: String, required: true },
  tone: { type: String, default: '' },
  loading: Boolean,
  disabled: Boolean,
  type: { type: String, default: 'button' },
  href: String,
  to: [String, Object],
})
const emit = defineEmits(['click'])
const blocked = computed(() => props.disabled || props.loading)
const isLink = computed(() => props.href !== undefined || props.to !== undefined)
const element = computed(() => props.to !== undefined && !blocked.value ? RouterLink : isLink.value ? 'a' : 'button')
const destination = computed(() => {
  if (blocked.value) return {}
  if (props.to !== undefined) return { to: props.to }
  return props.href !== undefined ? { href: props.href } : {}
})
function activate(event) {
  if (blocked.value) { event.preventDefault(); event.stopImmediatePropagation(); return }
  emit('click', event)
}
</script>

<style scoped>
.action-button { --action-color: #476556; --action-border: #d4dfd8; display: inline-flex; align-items: center; justify-content: center; flex: 0 0 auto; vertical-align: middle; width: 2.25rem; height: 2.25rem; margin-inline: .125rem; padding: 0; border: 1px solid var(--action-border); border-radius: .55rem; background: #fff; color: var(--action-color); cursor: pointer; text-decoration: none; transition: background-color .15s, border-color .15s, box-shadow .15s; }
.action-button:hover:not(:disabled):not([aria-disabled="true"]) { background: color-mix(in srgb, var(--action-color) 7%, white); border-color: var(--action-color); }
.action-button:focus-visible { outline: 2px solid var(--action-color); outline-offset: 3px; }
.action-button:disabled, .action-button[aria-disabled="true"] { opacity: .4; cursor: not-allowed; }
.action-button--danger { --action-color: #c2414a; --action-border: #f0cbd0; }
.action-button--success { --action-color: #28754d; --action-border: #bcdcc8; }
.action-button--warning { --action-color: #a56918; --action-border: #e7d3ad; }
.action-button--info { --action-color: #38689c; --action-border: #c5d7e9; }
.action-button-icon { width: 1.05rem; height: 1.05rem; flex: none; }
@media (pointer: coarse) { .action-button { width: 2.75rem; height: 2.75rem; } }
</style>
