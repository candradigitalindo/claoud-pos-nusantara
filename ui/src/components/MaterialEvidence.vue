<template>
  <div class="space-y-2">
    <label :for="inputId" class="block text-sm font-medium text-gray-700">{{ label }}</label>
    <p class="text-xs text-gray-500">Foto penggunaan, pemasangan, atau kondisi barang wajib saat diajukan. <template v-if="!photoOnly">PDF dapat dilampirkan sebagai tambahan.</template> Maksimal {{ maxFiles }} berkas, masing-masing 5 MB.</p>
    <input :id="inputId" type="file" multiple :accept="photoOnly ? 'image/jpeg,image/png,image/webp' : 'image/jpeg,image/png,image/webp,application/pdf'" :disabled="busy || disabled" class="block w-full text-sm" @change="upload" />
    <p v-if="busy" role="status" class="text-sm text-emerald-700">Mengunggah bukti…</p>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <ul class="flex flex-wrap gap-3">
      <li v-for="(url, i) in modelValue" :key="url" class="rounded-lg border p-2">
        <a :href="safeUrl(url)" target="_blank" rel="noopener noreferrer" class="text-sm text-emerald-700">
          <span v-if="url.toLowerCase().endsWith('.pdf')">Dokumen PDF {{ i + 1 }}</span>
          <img v-else :src="safeUrl(url)" :alt="`Bukti ${i + 1}`" class="h-20 w-28 rounded object-cover" />
        </a>
        <button type="button" :disabled="busy || disabled" class="mt-1 block text-xs text-red-600" @click="emit('update:modelValue', modelValue.filter((_, j) => j !== i))">Hapus bukti {{ i + 1 }}</button>
      </li>
    </ul>
  </div>
</template>
<script setup>
import { ref, useId } from 'vue'
import { apiClient } from '@/api/client.js'
import { safeUrl } from '@/utils/safeUrl.js'
const props = defineProps({ modelValue: { type: Array, default: () => [] }, label: { type: String, default: 'Bukti penggunaan' }, disabled: Boolean, maxFiles: { type: Number, default: 10 }, photoOnly: Boolean })
const emit = defineEmits(['update:modelValue', 'busy'])
const inputId = useId(), busy = ref(false), error = ref('')
async function upload(event) {
  const files = [...(event.target.files || [])]
  error.value = ''
  if (props.modelValue.length + files.length > props.maxFiles) { error.value = `Maksimal ${props.maxFiles} bukti.`; event.target.value = ''; return }
  busy.value = true; emit('busy', true)
  const urls = [...props.modelValue]
  try {
    for (const file of files) {
      if (file.size > 5 * 1024 * 1024) throw new Error(`${file.name}: maksimal 5 MB`)
      const data = new FormData(); data.append('file', file)
      const result = await apiClient.post('/admin/upload', data, { headers: { 'Content-Type': 'multipart/form-data' } })
      const url = safeUrl(result.url)
      if (!url) throw new Error('Alamat bukti tidak valid')
      urls.push(url); emit('update:modelValue', [...urls])
    }
  } catch (e) { error.value = e.message }
  finally { busy.value = false; emit('busy', false); event.target.value = '' }
}
</script>
