<template>
  <AppCard v-if="accounts?.length">
    <h3 class="mb-3 text-sm font-semibold text-gray-800">Rincian per Akun</h3>
    <div class="overflow-x-auto"><table class="w-full text-sm"><thead><tr class="text-left text-gray-500"><th class="py-2">Akun</th><th class="text-right">Nilai</th></tr></thead><tbody><tr v-for="a in accounts" :key="a.code" class="border-t border-gray-100"><td class="py-2 pr-3">{{ a.code }} · {{ a.name }}</td><td class="whitespace-nowrap text-right font-medium">{{ formatRupiah(a.amount) }}</td></tr></tbody></table></div>
  </AppCard>
</template>
<script setup>
import AppCard from '@/components/ui/AppCard.vue'
import { formatRupiah } from '@/utils/format'
defineProps({ accounts: Array })
</script>
