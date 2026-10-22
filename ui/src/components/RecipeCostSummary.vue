<template>
  <div class="recipe-cost-summary">
    <div class="cost-pair">
      <div><span>HPP bahan terkini</span><strong>{{ cost.current_cost != null ? money(cost.current_cost) : 'Harga belum lengkap' }}</strong></div>
      <div><span>Acuan saat dibuat</span><strong>{{ cost.initial_cost?.total != null ? money(cost.initial_cost.total) : cost.initial_cost ? 'Harga awal belum lengkap' : 'Belum direkam' }}</strong></div>
    </div>
    <p>{{ currentSource }}. Acuan awal memakai harga gudang induk saat resep dibuat dan tetap tersimpan saat resep diedit.</p>
    <details v-if="cost.initial_cost?.ingredients?.length">
      <summary>Acuan {{ date(cost.initial_cost.captured_at) }} · lihat komposisi awal</summary>
      <div v-for="(item, index) in cost.initial_cost.ingredients" :key="index" class="reference-line"><span>{{ item.item_name }} · {{ item.qty_base }} {{ item.unit }}</span><span>{{ item.cost_per_base > 0 ? money(item.cost) : 'Belum ada harga' }}</span></div>
    </details>
  </div>
</template>
<script setup>
import { formatRupiah as money } from '@/utils/format'
defineProps({ cost: { type: Object, required: true }, currentSource: { type: String, default: 'Harga terkini: rata-rata stok outlet, cadangan harga gudang induk' } })
const date = value => new Date(value).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
</script>
<style scoped>
.recipe-cost-summary{border:1px solid #dce8df;border-radius:10px;background:#f7faf8;padding:12px;margin:12px 0;color:#335e42}.cost-pair{display:grid;grid-template-columns:1fr 1fr;gap:12px}.cost-pair span{display:block;font-size:.68rem;color:#718578}.cost-pair strong{display:block;font-size:.86rem;margin-top:4px}.recipe-cost-summary p{font-size:.66rem;line-height:1.6;color:#7b8a80;margin-top:8px}.recipe-cost-summary details{font-size:.7rem;margin-top:8px}.recipe-cost-summary summary{cursor:pointer}.reference-line{display:flex;justify-content:space-between;gap:12px;padding-top:7px}.reference-line>span:last-child{flex-shrink:0}
</style>
