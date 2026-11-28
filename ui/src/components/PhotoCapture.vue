<template>
  <div>
    <label class="lbl">
      {{ label }} <span v-if="required" class="text-red-500">*</span>
      <span v-if="multiple" class="ml-1 font-normal text-gray-400">({{ urls.length }}/{{ maxFiles }})</span>
    </label>

    <!-- Mode multi-foto -->
    <template v-if="multiple">
      <div v-if="urls.length" class="mb-2.5 flex flex-wrap gap-2.5">
        <div v-for="(u, idx) in urls" :key="`${u}-${idx}`" class="group relative">
          <a v-if="isImageUrl(u)" :href="u" target="_blank" rel="noopener noreferrer" class="block">
            <img :src="u" :alt="`Bukti foto ${idx + 1}`"
              class="h-24 w-24 rounded-lg border border-gray-200 object-cover transition hover:opacity-90" />
          </a>
          <a v-else :href="u" target="_blank" rel="noopener noreferrer"
            class="flex h-24 w-24 flex-col items-center justify-center rounded-lg border border-gray-200 bg-gray-50 p-2 text-center text-[11px] font-medium text-emerald-700 hover:bg-emerald-50">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
            <span class="mt-1 w-full truncate">Berkas {{ idx + 1 }}</span>
          </a>
          <span class="pointer-events-none absolute bottom-1 left-1 rounded bg-black/60 px-1.5 py-0.5 text-[10px] font-semibold text-white">
            {{ idx + 1 }}
          </span>
          <button type="button" @click="removeAt(idx)" :disabled="uploading" title="Hapus berkas ini"
            class="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-red-600 text-xs font-bold text-white shadow hover:bg-red-700 disabled:opacity-50">
            ×
          </button>
        </div>
      </div>

      <div v-if="urls.length < maxFiles" class="flex flex-wrap gap-2">
        <!-- Tanpa capture agar ponsel & desktop membuka pemilih multi-file/galeri -->
        <input ref="multiInput" type="file" :accept="accept" multiple class="hidden" @change="onPickMulti" />
        <button type="button" class="cap-btn" :disabled="uploading" @click="multiInput?.click()">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>
          </svg>
          {{ uploading ? progressLabel : (urls.length ? '+ Tambah Foto / Berkas' : 'Pilih Foto (Bisa Banyak)') }}
        </button>

        <template v-if="showCameraBtn">
          <input ref="camInput" type="file" accept="image/*" :capture="captureAttr" class="hidden" @change="onPickMulti" />
          <button type="button" class="cap-btn cap-btn--cam" :disabled="uploading" @click="camInput?.click()">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/><circle cx="12" cy="13" r="4"/>
            </svg>
            Ambil Kamera
          </button>
        </template>
      </div>
      <p v-if="urls.length" class="mt-1 text-[11px] text-emerald-700">
        {{ urls.length }} berkas terunggah. Klik pratinjau untuk memperbesar atau × untuk menghapus.
      </p>
      <p v-if="hint" class="mt-0.5 text-[11px] text-gray-500">{{ hint }}</p>
    </template>

    <!-- Mode foto tunggal (bawaan, kompatibel mundur) -->
    <template v-else>
      <div v-if="singleUrl" class="flex items-start gap-3">
        <a v-if="isImageUrl(singleUrl)" :href="singleUrl" target="_blank" rel="noopener noreferrer">
          <img :src="singleUrl" alt="Bukti foto" class="h-24 w-24 rounded-lg border border-gray-200 object-cover" />
        </a>
        <a v-else :href="singleUrl" target="_blank" rel="noopener noreferrer"
          class="flex h-24 w-24 flex-col items-center justify-center rounded-lg border border-gray-200 bg-gray-50 p-2 text-center text-xs font-medium text-emerald-700">
          Lihat Berkas ↗
        </a>
        <div class="min-w-0">
          <p class="text-xs text-emerald-700">Foto terunggah.</p>
          <p class="mt-0.5 text-[11px] text-gray-500">Salinannya dikirim ke email cadangan secara otomatis.</p>
          <button type="button" class="mt-1.5 text-xs font-medium text-red-600 hover:underline" @click="clear">Ganti foto</button>
        </div>
      </div>

      <div v-else>
        <!-- capture="environment" membuka kamera belakang langsung di ponsel,
             tanpa memilih berkas dulu — ini dipakai sambil memegang barangnya. -->
        <input ref="input" type="file" :accept="accept" :capture="captureAttr" class="hidden" @change="onPickSingle" />
        <button type="button" class="cap-btn" :disabled="uploading" @click="input?.click()">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <path d="M23 19a2 2 0 01-2 2H3a2 2 0 01-2-2V8a2 2 0 012-2h4l2-3h6l2 3h4a2 2 0 012 2z"/><circle cx="12" cy="13" r="4"/>
          </svg>
          {{ uploading ? 'Mengunggah…' : 'Ambil / Pilih Foto' }}
        </button>
        <p v-if="hint" class="mt-1 text-[11px] text-gray-500">{{ hint }}</p>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { apiClient } from '@/api/client.js'
import { useToastStore } from '@/stores/toast.js'
import { safeUrl, isImageUrl } from '@/utils/safeUrl.js'

const props = defineProps({
  modelValue: { type: [String, Array], default: '' },
  multiple: { type: Boolean, default: false },
  maxFiles: { type: Number, default: 10 },
  accept: { type: String, default: 'image/*' },
  label: { type: String, default: 'Foto Bukti' },
  hint: { type: String, default: '' },
  required: { type: Boolean, default: true },
  capture: { type: [String, Boolean], default: 'environment' },
})
const emit = defineEmits(['update:modelValue'])

const captureAttr = computed(() => {
  if (!props.capture) return undefined
  return props.capture === true ? 'environment' : props.capture
})
const showCameraBtn = computed(() => Boolean(captureAttr.value))

const toast = useToastStore()
const uploading = ref(false)
const uploadDone = ref(0)
const uploadTotal = ref(0)
const progressLabel = computed(() =>
  uploadTotal.value > 1 ? `Mengunggah ${uploadDone.value}/${uploadTotal.value}…` : 'Mengunggah…'
)

const input = ref(null)
const multiInput = ref(null)
const camInput = ref(null)

// Pratinjau hanya untuk alamat unggahan yang sah (T-75).
const singleUrl = computed(() => {
  const raw = Array.isArray(props.modelValue) ? (props.modelValue[0] || '') : props.modelValue
  return safeUrl(raw)
})
const urls = computed(() => {
  const list = Array.isArray(props.modelValue)
    ? props.modelValue
    : (props.modelValue ? [props.modelValue] : [])
  return list.map(u => safeUrl(u)).filter(Boolean)
})

async function uploadOne(file) {
  const fd = new FormData()
  fd.append('file', file)
  const res = await apiClient.post('/admin/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
  const url = safeUrl(res?.url ?? res?.data?.url)
  if (!url) throw new Error('Server tidak mengembalikan alamat berkas yang sah')
  return url
}

async function onPickSingle(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) {
    toast.error('Ukuran berkas maksimal 5MB')
    e.target.value = ''
    return
  }
  uploading.value = true
  try {
    const url = await uploadOne(file)
    emit('update:modelValue', url)
  } catch (err) {
    toast.error(err?.message || 'Gagal mengunggah foto')
  } finally {
    uploading.value = false
    if (e.target) e.target.value = ''
  }
}

async function onPickMulti(e) {
  const picked = Array.from(e.target.files || [])
  if (!picked.length) return
  const current = [...urls.value]
  const remaining = Math.max(0, props.maxFiles - current.length)
  if (remaining === 0) {
    toast.error(`Maksimal ${props.maxFiles} berkas`)
    if (e.target) e.target.value = ''
    return
  }
  let queue = picked
  if (queue.length > remaining) {
    toast.error(`Maksimal ${props.maxFiles} berkas — hanya ${remaining} berkas pertama yang diunggah`)
    queue = queue.slice(0, remaining)
  }
  const valid = []
  for (const f of queue) {
    if (f.size > 5 * 1024 * 1024) {
      toast.error(`"${f.name}" melebihi batas 5MB`)
    } else {
      valid.push(f)
    }
  }
  if (!valid.length) {
    if (e.target) e.target.value = ''
    return
  }
  uploading.value = true
  uploadDone.value = 0
  uploadTotal.value = valid.length
  const next = [...current]
  try {
    for (const file of valid) {
      uploadDone.value++
      try {
        const url = await uploadOne(file)
        if (!next.includes(url)) next.push(url)
      } catch (err) {
        toast.error(`${file.name}: ${err?.message || 'Gagal mengunggah'}`)
      }
    }
    emit('update:modelValue', next)
  } finally {
    uploading.value = false
    uploadDone.value = 0
    uploadTotal.value = 0
    if (e.target) e.target.value = ''
  }
}

function removeAt(idx) {
  const next = urls.value.filter((_, i) => i !== idx)
  emit('update:modelValue', next)
}

function clear() { emit('update:modelValue', props.multiple ? [] : '') }
</script>

<style scoped>
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.cap-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: .45rem;
  min-height: 42px; width: 100%; border-radius: .6rem; padding: .45rem .9rem;
  font-size: .82rem; font-weight: 600; color: #374151;
  background: #fff; border: 1px dashed rgba(0,0,0,.22); cursor: pointer;
}
.cap-btn:hover:not(:disabled) { background: #f9fafb; border-color: rgba(5,150,105,.55); color: #047857; }
.cap-btn--cam { border-style: solid; border-color: rgba(5,150,105,.3); color: #065f46; background: #ecfdf5; }
.cap-btn--cam:hover:not(:disabled) { background: #d1fae5; }
.cap-btn:disabled { opacity: .6; cursor: not-allowed; }
@media (min-width: 640px) { .cap-btn { width: auto; } }
</style>
