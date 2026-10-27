<template>
  <div class="space-y-5">
    <div>
      <h1 class="text-2xl font-bold text-gray-900 font-display">Resep &amp; Formula F &amp; B</h1>
      <p class="mt-1 text-sm text-gray-500">Kelola resep menu, bahan setengah jadi, dan pola pemotongan untuk standar seluruh outlet.</p>
    </div>
    <div role="tablist" aria-label="Jenis resep F & B" class="flex flex-wrap gap-2 border-b border-gray-200 pb-3">
      <button v-for="tab in tabs" :key="tab.id" :id="`recipe-tab-${tab.id}`"
        role="tab" :data-testid="`recipe-tab-${tab.id}`" :aria-selected="activeTab === tab.id"
        :aria-controls="`recipe-panel-${tab.id}`" :tabindex="activeTab === tab.id ? 0 : -1"
        :class="['rounded-lg px-4 py-2 text-sm font-semibold transition-colors', activeTab === tab.id ? 'bg-emerald-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200']"
        @click="selectTab(tab.id)" @keydown="onTabKeydown($event, tab.id)">
        {{ tab.label }}
      </button>
    </div>
    <section :id="`recipe-panel-${activeTab}`" role="tabpanel" :aria-labelledby="`recipe-tab-${activeTab}`">
      <FnbMenuRecipes v-if="activeTab === 'menu'" />
      <FnbStockFormulas v-else :key="activeTab" :mode="activeTab" />
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import FnbMenuRecipes from '@/pages/fnb/FnbMenuRecipes.vue'
import FnbStockFormulas from '@/pages/fnb/FnbStockFormulas.vue'

const route = useRoute()
const router = useRouter()
const tabs = [
  { id: 'menu', label: 'Resep Menu & SOP' },
  { id: 'sfg', label: 'Resep Bahan Setengah Jadi' },
  { id: 'breakdown', label: 'Pola Pemotongan' },
]
const activeTab = computed(() => tabs.some(tab => tab.id === route.query.tab) ? route.query.tab : 'menu')
async function selectTab(tab) {
  await router.replace({ query: { ...route.query, tab } })
}
async function onTabKeydown(event, id) {
  const index = tabs.findIndex(tab => tab.id === id)
  let target
  if (event.key === 'ArrowRight') target = tabs[(index + 1) % tabs.length]
  else if (event.key === 'ArrowLeft') target = tabs[(index + tabs.length - 1) % tabs.length]
  else if (event.key === 'Home') target = tabs[0]
  else if (event.key === 'End') target = tabs[tabs.length - 1]
  if (!target) return
  event.preventDefault()
  await selectTab(target.id)
  await nextTick()
  document.getElementById(`recipe-tab-${target.id}`)?.focus()
}
</script>
