<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div><p class="text-xs font-semibold uppercase tracking-widest text-emerald-700">Food & Beverage</p><h1 class="text-2xl font-bold text-gray-900 mt-1">Dashboard F & B</h1><p class="text-sm text-gray-500 mt-1">Kesiapan menu, pengembangan produk, dan pengendalian biaya.</p></div>
      <FnbOutletFilter v-model="outlet" />
    </div>
    <AppAlert type="error" :message="error" />
    <p v-if="loading" class="text-sm text-gray-500 py-8">Memuat dashboard F&B…</p>
    <template v-else-if="data">
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <AppCard v-for="stat in stats" :key="stat.label"><p class="text-xs text-gray-500">{{ stat.label }}</p><p class="text-3xl font-bold mt-2" :class="stat.alert ? 'text-amber-700' : 'text-gray-900'">{{ stat.value }}</p><p class="text-xs text-gray-400 mt-1">{{ stat.note }}</p></AppCard>
      </div>
      <div class="grid lg:grid-cols-2 gap-4">
        <AppCard>
          <h2 class="font-semibold text-gray-900">Komposisi menu</h2><p class="text-xs text-gray-500 mt-1">Kategori F&B berlaku terpisah dari kategori POS.</p>
          <div v-for="kind in categories" :key="kind.label" class="mt-5">
            <div class="flex justify-between text-sm mb-2"><span>{{ kind.label }}</span><b>{{ kind.value }} menu</b></div>
            <div class="h-2 bg-gray-100 rounded-full overflow-hidden"><div :class="kind.color" class="h-full rounded-full" :style="{ width: `${data.product_count ? kind.value / data.product_count * 100 : 0}%` }"></div></div>
          </div>
          <RouterLink v-if="auth.hasPermission('fnb.costing.view')" :to="{ path: '/fnb/costing', query: { outlet_id: outlet } }" class="inline-block mt-5 text-sm text-emerald-700 font-semibold">Buka HPP & kategori menu →</RouterLink>
        </AppCard>
        <AppCard>
          <h2 class="font-semibold text-gray-900">Tahap pengembangan</h2><p class="text-xs text-gray-500 mt-1">Ide diuji, direview, lalu disetujui sebelum diluncurkan.</p>
          <div class="grid grid-cols-3 gap-3 mt-5">
            <div v-for="stage in DEV_STAGES" :key="stage.value" class="rounded-xl bg-gray-50 p-3"><p class="text-xs text-gray-500">{{ stage.label }}</p><b class="block text-xl mt-1">{{ data.development_stages[stage.value] || 0 }}</b></div>
          </div>
          <RouterLink v-if="auth.hasPermission('fnb.development.view')" to="/fnb/development" class="inline-block mt-5 text-sm text-emerald-700 font-semibold">Kelola pengembangan produk →</RouterLink>
        </AppCard>
      </div>
      <AppCard>
        <div class="flex flex-wrap justify-between gap-2"><h2 class="font-semibold text-gray-900">Perlu ditindaklanjuti</h2><span class="text-xs text-gray-500">Maksimal 12 menu dalam cakupan outlet</span></div>
        <p v-if="!data.attention.length" class="text-sm text-gray-500 py-5">{{ data.product_count ? 'Tidak ada menu yang memerlukan tindak lanjut.' : 'Belum ada produk aktif. Mulai dengan pengembangan produk atau katalog produk.' }}</p>
        <div v-for="row in data.attention" :key="row.product_id" class="py-3 border-b border-gray-100 last:border-0 flex flex-wrap justify-between gap-2">
          <div><b class="text-sm">{{ row.product_name }}</b><p class="text-xs text-gray-400">{{ row.outlet_name }} · {{ kindLabel(row.menu_kind) }}</p></div>
          <div class="text-xs text-amber-700 text-right"><p v-if="row.cost_status !== 'ready'">{{ costStatusLabel(row.cost_status) }}</p><p v-if="row.is_over">HPP {{ row.cost_pct.toFixed(1) }}% mencapai batas {{ row.target_cost_pct }}%</p><p v-if="row.projected_is_over">Proyeksi {{ row.projected_cost_pct.toFixed(1) }}% · tinjau biaya / harga</p><p v-if="!row.menu_kind">Tentukan makanan atau minuman</p><p v-if="row.price <= 0">Harga jual belum diisi</p></div>
        </div>
      </AppCard>
      <AppCard>
        <h2 class="font-semibold text-gray-900">Fokus kerja divisi</h2>
        <div class="grid sm:grid-cols-3 gap-4 mt-3 text-sm text-gray-600"><p><b class="block text-gray-800 mb-1">Pengembangan menu</b>Catat konsep, target harga, hasil uji rasa/porsi, dan keputusan review.</p><p><b class="block text-gray-800 mb-1">Standar resep & SOP</b>Jaga konsistensi bahan, takaran, langkah pengolahan, dan akses resep antar outlet.</p><p><b class="block text-gray-800 mb-1">Pengendalian HPP</b>Tinjau biaya bahan, kemasan, overhead, serta kontribusi menu terhadap margin.</p></div>
        <p class="text-xs text-gray-400 mt-4">Ringkasan memakai biaya standar terkini. Produk tanpa resep tetap boleh dijual; HPP memerlukan sumber biaya. Biaya tenaga kerja dan operasional hanya tercakup bila dimasukkan dalam overhead per porsi.</p>
      </AppCard>
    </template>
  </div>
</template>
<script setup>
import { ref, computed, watch } from 'vue'
import { fnbApi } from '@/api/fnb'
import { useAuthStore } from '@/stores/auth'
import { DEV_STAGES, kindLabel, costStatusLabel } from '@/utils/fnb'
import FnbOutletFilter from '@/components/FnbOutletFilter.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
const auth = useAuthStore()
const outlet = ref(''), data = ref(null), error = ref(''), loading = ref(false)
const stats = computed(() => data.value ? [
  { label: 'Menu aktif', value: data.value.product_count, note: 'Produk per outlet' },
  { label: 'HPP lengkap', value: data.value.cost_ready, note: `${data.value.no_recipe} menu tanpa sumber biaya` },
  { label: 'Mencapai batas HPP', value: data.value.over_target, note: `${data.value.projected_over_target || 0} menu berisiko dari proyeksi`, alert: true },
  { label: 'Master resep', value: data.value.recipe_count, note: `${data.value.missing_cost} menu dengan biaya belum tersedia` },
] : [])
const categories = computed(() => data.value ? [{ label: 'Makanan', value: data.value.food_count, color: 'bg-emerald-500' }, { label: 'Minuman', value: data.value.beverage_count, color: 'bg-sky-500' }, { label: 'Belum dikategorikan', value: data.value.unclassified, color: 'bg-amber-400' }] : [])
let seq = 0
watch(outlet, async () => {
  const request = ++seq; loading.value = true; error.value = ''; data.value = null
  try { const result = await fnbApi.dashboard({ outlet_id: outlet.value }); if (request === seq) data.value = result }
  catch (e) { if (request === seq) error.value = e.message }
  finally { if (request === seq) loading.value = false }
}, { immediate: true })
</script>
