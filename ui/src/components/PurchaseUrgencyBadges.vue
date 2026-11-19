<!--
  PurchaseUrgencyBadges.vue — penanda tingkat kebutuhan sebuah pengajuan:
  DARURAT (+ status pengesahan), URGENT, atau PRIORITAS beserta tanggal
  butuhnya. Pengajuan rutin tidak diberi penanda.
-->
<template>
  <span v-if="doc?.emergency" class="inline-flex flex-wrap items-center gap-1">
    <span class="ub ub-emergency" title="Pembelian Darurat — dibeli dulu dari kas laci outlet, disahkan sesudahnya">
      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
      Darurat
    </span>
    <span v-if="ratification" :class="['ub', ratification.cls]">{{ ratification.label }}</span>
  </span>
  <span v-else-if="isUrgent || isPriority" :class="['ub', isUrgent ? 'ub-urgent' : 'ub-priority']"
    :title="needByTitle">
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <template v-if="isUrgent"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></template>
      <template v-else><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></template>
    </svg>
    {{ isUrgent ? 'Urgent' : 'Prioritas' }}<template v-if="needByShort"> · {{ overdue ? 'lewat ' : 'butuh ' }}{{ needByShort }}</template>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { RATIFICATION } from '@/utils/procurement.js'
import { todayDateString } from '@/utils/format.js'

const props = defineProps({
  doc: { type: Object, default: null },
})

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const DONE = ['paid', 'partial', 'received', 'rejected', 'cancelled']

const isUrgent = computed(() => props.doc?.urgency === 'urgent')
const isPriority = computed(() => props.doc?.urgency === 'prioritas')
const ratification = computed(() => RATIFICATION[props.doc?.emergency?.ratification] || null)
const needBy = computed(() => String(props.doc?.need_by_date || '').slice(0, 10))
const needByShort = computed(() => {
  const [, m, d] = needBy.value.split('-')
  if (!m || !d) return ''
  return `${Number(d)} ${MONTHS[Number(m) - 1] || m}`
})
// Lewat tanggal butuh dan belum dibayar/diterima.
const overdue = computed(() => !!needBy.value && needBy.value < todayDateString() && !DONE.includes(props.doc?.status))
const needByTitle = computed(() => needBy.value
  ? `Dibutuhkan paling lambat ${needByShort.value}${overdue.value ? ' — sudah lewat' : ''}`
  : '')
</script>

<style scoped>
.ub {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border-radius: 9999px;
  padding: 1px 7px;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  letter-spacing: .02em;
  white-space: nowrap;
}
.ub-emergency { background: #e11d48; color: #fff; text-transform: uppercase; }
.ub-urgent    { background: #fee2e2; color: #b91c1c; box-shadow: inset 0 0 0 1px #fca5a5; }
.ub-priority  { background: #fef3c7; color: #b45309; box-shadow: inset 0 0 0 1px #fcd34d; }
</style>
