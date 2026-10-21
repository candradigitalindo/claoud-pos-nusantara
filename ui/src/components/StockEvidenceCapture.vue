<template>
  <div class="space-y-2">
    <label class="block text-sm font-semibold text-gray-700">{{ label }} *</label>
    <div v-if="modelValue?.id" class="flex flex-wrap items-start gap-3">
      <a :href="safeUrl(modelValue.url)" target="_blank" rel="noopener noreferrer"><img :src="safeUrl(modelValue.url)" alt="Bukti foto dengan timestamp server" class="w-36 rounded-lg border" /></a>
      <div class="text-xs text-gray-600"><p>{{ date(modelValue.recorded_at) }}</p><p>Oleh {{ modelValue.actor_name }}</p><button type="button" class="text-red-600 mt-2" @click="emit('update:modelValue', null)">Ganti foto</button></div>
    </div>
    <template v-else>
      <input ref="input" type="file" accept="image/jpeg,image/png,image/webp" capture="environment" class="hidden" @change="upload" />
      <button type="button" :disabled="busy || !warehouseId" @click="input.click()" class="min-h-11 border border-dashed border-emerald-400 text-emerald-800 px-4 py-2 rounded-lg disabled:opacity-50">{{ busy ? 'Mengunggah & memberi timestamp…' : 'Ambil / pilih foto' }}</button>
    </template>
    <p class="text-xs text-gray-500">Foto diberi cap waktu pencatatan server dan identitas pengunggah. JPG, PNG, atau WebP; maksimal 5 MB. Gunakan foto kondisi barang saat proses ini.</p>
  </div>
</template>
<script setup>
import { ref, watch } from 'vue'
import { apiClient } from '@/api/client'
import { safeUrl } from '@/utils/safeUrl'
import { useToastStore } from '@/stores/toast'
const props = defineProps({ modelValue: Object, warehouseId: { type: String, default: '' }, label: { type: String, default: 'Foto bukti' } })
const emit = defineEmits(['update:modelValue'])
const input = ref(null), busy = ref(false), toast = useToastStore()
let seq = 0
const date = value => value ? new Date(value).toLocaleString('id-ID') : ''
watch(() => props.warehouseId, () => { seq++; busy.value = false; emit('update:modelValue', null) })
async function upload(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) { toast.error('Ukuran foto maksimal 5 MB'); e.target.value = ''; return }
  const request = ++seq
  busy.value = true
  try {
    const data = new FormData(); data.append('file', file); data.append('warehouse_id', props.warehouseId)
    const photo = await apiClient.post('/admin/stock-evidence', data, { headers: { 'Content-Type': 'multipart/form-data' } })
    if (request === seq) emit('update:modelValue', photo)
  } catch (err) { if (request === seq) toast.error(err.message) }
  finally { if (request === seq) busy.value = false; e.target.value = '' }
}
</script>
