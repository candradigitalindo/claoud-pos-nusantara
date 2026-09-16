<template>
  <div>
    <label class="lbl">{{ label }} <span v-if="required" class="text-red-500">*</span></label>

    <div v-if="modelValue" class="flex items-start gap-3">
      <img :src="modelValue" alt="Bukti foto" class="h-24 w-24 rounded-lg border border-gray-200 object-cover" />
      <div class="min-w-0">
        <p class="text-xs text-emerald-700">Foto terunggah.</p>
        <p class="mt-0.5 text-[11px] text-gray-500">Salinannya dikirim ke email cadangan secara otomatis.</p>
        <button type="button" class="mt-1.5 text-xs font-medium text-red-600 hover:underline" @click="clear">Ganti foto</button>
      </div>
    </div>

    <div v-else>
      <!-- capture="environment" membuka kamera belakang langsung di ponsel,
           tanpa memilih berkas dulu — ini dipakai sambil memegang barangnya. -->
      <input ref="input" type="file" accept="image/*" capture="environment" class="hidden" @change="onPick" />
      <button type="button" class="cap-btn" :disabled="uploading" @click="$refs.input.click()">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
          <path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/><circle cx="12" cy="13" r="4"/>
        </svg>
        {{ uploading ? 'Mengunggah…' : 'Ambil / Pilih Foto' }}
      </button>
      <p v-if="hint" class="mt-1 text-[11px] text-gray-500">{{ hint }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { apiClient } from '@/api/client.js'
import { useToastStore } from '@/stores/toast.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  label: { type: String, default: 'Foto Bukti' },
  hint: { type: String, default: '' },
  required: { type: Boolean, default: true },
})
const emit = defineEmits(['update:modelValue'])

const toast = useToastStore()
const uploading = ref(false)
const input = ref(null)

async function onPick(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) {
    toast.error('Ukuran foto maksimal 5MB')
    e.target.value = ''
    return
  }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    const res = await apiClient.post('/admin/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = res?.url || res?.data?.url || res?.data
    if (!url) throw new Error('Server tidak mengembalikan alamat berkas')
    emit('update:modelValue', url)
  } catch (err) {
    toast.error(err?.message || 'Gagal mengunggah foto')
  } finally {
    uploading.value = false
    if (e.target) e.target.value = ''
  }
}

function clear() { emit('update:modelValue', '') }
</script>

<style scoped>
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.cap-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: .45rem;
  min-height: 44px; width: 100%; border-radius: .6rem; padding: .5rem 1rem;
  font-size: .85rem; font-weight: 600; color: #374151;
  background: #fff; border: 1px dashed rgba(0,0,0,.2);
}
.cap-btn:hover:not(:disabled) { background: #f9fafb; border-color: rgba(5,150,105,.5); color: #047857; }
.cap-btn:disabled { opacity: .6; }
@media (min-width: 640px) { .cap-btn { width: auto; } }
</style>
