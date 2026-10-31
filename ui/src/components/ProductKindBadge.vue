<template>
  <span class="product-kind" :class="`product-kind--${kind || 'unclassified'}`">{{ productKindLabel(kind) }}</span>
</template>

<script setup>
import { productKindLabel } from '@/utils/productKinds.js'
defineProps({ kind: { type: String, default: '' } })
</script>

<style scoped>
.product-kind { display: inline-flex; align-items: center; border-radius: 999px; padding: .18rem .55rem; font-size: .7rem; font-weight: 600; line-height: 1.5; background: #eef1f3; color: #55616e; white-space: nowrap; }
.product-kind--food { background: #eaf5e8; color: #38652c; }
.product-kind--beverage { background: #e6f2fa; color: #225e81; }
.product-kind--activity { background: #f1eafa; color: #704b94; }
</style>
