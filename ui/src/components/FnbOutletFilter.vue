<template>
  <div class="min-w-0 sm:min-w-64">
    <label class="block text-xs font-medium text-gray-500 mb-1">Outlet</label>
    <SearchSelect :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" :options="options" value-key="value" label-key="label" placeholder="Semua outlet dalam akses" />
    <p v-if="error" class="text-xs text-red-600 mt-1">{{ error }}</p>
  </div>
</template>
<script setup>
import { ref, onMounted } from 'vue'
import { outletsApi } from '@/api/outlets'
import SearchSelect from '@/components/ui/SearchSelect.vue'
defineProps({ modelValue: { type: String, default: '' } })
defineEmits(['update:modelValue'])
const options = ref([{ value: '', label: 'Semua outlet dalam akses' }])
const error = ref('')
onMounted(async () => {
  try {
    const data = await outletsApi.myOutlets()
    options.value.push(...(data.outlets || data || []).map(o => ({ value: o.id, label: o.name })))
  } catch (e) { error.value = e.message || 'Gagal memuat outlet' }
})
</script>
