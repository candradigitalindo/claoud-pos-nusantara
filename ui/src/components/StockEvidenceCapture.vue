<template>
  <div class="space-y-2">
    <label class="block text-sm font-semibold text-gray-700">
      {{ label }} <span class="text-red-500">*</span>
      <span v-if="multiple" class="ml-1 text-xs font-normal text-gray-500">({{ items.length }}/{{ maxFiles }})</span>
    </label>

    <!-- Mode multi-foto bertimestamp server -->
    <template v-if="multiple">
      <div v-if="items.length" class="flex flex-wrap gap-3">
        <div v-for="(p, idx) in items" :key="p.id" class="relative w-32 rounded-lg border border-gray-200 bg-white p-1.5 shadow-xs">
          <a :href="safeUrl(p.url)" target="_blank" rel="noopener noreferrer" class="block">
            <img :src="safeUrl(p.url)" :alt="`Bukti foto ${idx + 1} bertimestamp`" class="h-24 w-full rounded object-cover" />
          </a>
          <div class="mt-1 text-[10px] leading-tight text-gray-600">
            <p class="truncate">{{ date(p.recorded_at) }}</p>
            <p class="truncate text-gray-400">{{ p.actor_name }}</p>
          </div>
          <button type="button" :disabled="busy" @click="removeAt(idx)" title="Hapus foto ini"
            class="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-red-600 text-xs font-bold text-white shadow hover:bg-red-700 disabled:opacity-50">
            ×
          </button>
        </div>
      </div>

      <div v-if="items.length < maxFiles" class="flex flex-wrap gap-2">
        <input ref="multiInput" type="file" accept="image/jpeg,image/png,image/webp" multiple class="hidden" @change="uploadMulti" />
        <button type="button" :disabled="busy || !warehouseId" @click="multiInput?.click()"
          class="min-h-11 inline-flex items-center gap-1.5 border border-dashed border-emerald-400 bg-white text-emerald-800 px-3.5 py-2 rounded-lg text-sm font-medium hover:bg-emerald-50 disabled:opacity-50">
          {{ busy ? progressLabel : (items.length ? '+ Tambah Foto Pemeriksaan' : 'Pilih Foto (Bisa Banyak)') }}
        </button>

        <input ref="camInput" type="file" accept="image/jpeg,image/png,image/webp" capture="environment" class="hidden" @change="uploadMulti" />
        <button type="button" :disabled="busy || !warehouseId" @click="camInput?.click()"
          class="min-h-11 inline-flex items-center gap-1.5 border border-emerald-300 bg-emerald-50 text-emerald-800 px-3.5 py-2 rounded-lg text-sm font-medium hover:bg-emerald-100 disabled:opacity-50">
          Ambil Kamera
        </button>
      </div>
    </template>

    <!-- Mode tunggal (bawaan, kompatibel mundur untuk StockQuality) -->
    <template v-else>
      <div v-if="modelValue?.id" class="flex flex-wrap items-start gap-3">
        <a :href="safeUrl(modelValue.url)" target="_blank" rel="noopener noreferrer"><img :src="safeUrl(modelValue.url)" alt="Bukti foto dengan timestamp server" class="w-36 rounded-lg border" /></a>
        <div class="text-xs text-gray-600"><p>{{ date(modelValue.recorded_at) }}</p><p>Oleh {{ modelValue.actor_name }}</p><button type="button" class="text-red-600 mt-2" @click="emit('update:modelValue', null)">Ganti foto</button></div>
      </div>
      <template v-else>
        <input ref="input" type="file" accept="image/jpeg,image/png,image/webp" capture="environment" class="hidden" @change="uploadSingle" />
        <button type="button" :disabled="busy || !warehouseId" @click="input?.click()" class="min-h-11 border border-dashed border-emerald-400 text-emerald-800 px-4 py-2 rounded-lg disabled:opacity-50">{{ busy ? 'Mengunggah & memberi timestamp…' : 'Ambil / pilih foto' }}</button>
      </template>
    </template>

    <p class="text-xs text-gray-500">
      Setiap foto diberi cap waktu pencatatan server dan identitas pengunggah. JPG, PNG, atau WebP; maksimal 5 MB per foto<template v-if="multiple"> (maksimal {{ maxFiles }} foto)</template>.
    </p>
  </div>
</template>
<script setup>
import { ref, computed, watch } from 'vue'
import { apiClient } from '@/api/client'
import { safeUrl } from '@/utils/safeUrl'
import { useToastStore } from '@/stores/toast'

const props = defineProps({
  modelValue: { type: [Object, Array], default: null },
  multiple: { type: Boolean, default: false },
  maxFiles: { type: Number, default: 10 },
  warehouseId: { type: String, default: '' },
  label: { type: String, default: 'Foto bukti' },
})
const emit = defineEmits(['update:modelValue'])

const input = ref(null)
const multiInput = ref(null)
const camInput = ref(null)
const busy = ref(false)
const uploadDone = ref(0)
const uploadTotal = ref(0)
const toast = useToastStore()
let seq = 0

const items = computed(() => {
  if (Array.isArray(props.modelValue)) return props.modelValue.filter(p => p?.id)
  return props.modelValue?.id ? [props.modelValue] : []
})
const progressLabel = computed(() =>
  uploadTotal.value > 1
    ? `Mengunggah & timestamp ${uploadDone.value}/${uploadTotal.value}…`
    : 'Mengunggah & memberi timestamp…'
)
const date = value => value ? new Date(value).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta' }) : ''

watch(() => props.warehouseId, () => {
  seq++
  busy.value = false
  emit('update:modelValue', props.multiple ? [] : null)
})

async function uploadEvidence(file) {
  const data = new FormData()
  data.append('file', file)
  data.append('warehouse_id', props.warehouseId)
  return await apiClient.post('/admin/stock-evidence', data, { headers: { 'Content-Type': 'multipart/form-data' } })
}

async function uploadSingle(e) {
  const file = e.target.files?.[0]
  if (!file) return
  if (file.size > 5 * 1024 * 1024) { toast.error('Ukuran foto maksimal 5 MB'); e.target.value = ''; return }
  const request = ++seq
  busy.value = true
  try {
    const photo = await uploadEvidence(file)
    if (request === seq) emit('update:modelValue', photo)
  } catch (err) { if (request === seq) toast.error(err.message) }
  finally { if (request === seq) busy.value = false; if (e.target) e.target.value = '' }
}

async function uploadMulti(e) {
  const picked = Array.from(e.target.files || [])
  if (!picked.length) return
  const current = [...items.value]
  const remaining = Math.max(0, props.maxFiles - current.length)
  if (remaining === 0) {
    toast.error(`Maksimal ${props.maxFiles} foto`)
    if (e.target) e.target.value = ''
    return
  }
  let queue = picked
  if (queue.length > remaining) {
    toast.error(`Maksimal ${props.maxFiles} foto — hanya ${remaining} foto pertama yang diunggah`)
    queue = queue.slice(0, remaining)
  }
  const valid = []
  for (const f of queue) {
    if (f.size > 5 * 1024 * 1024) toast.error(`"${f.name}" melebihi batas 5 MB`)
    else valid.push(f)
  }
  if (!valid.length) { if (e.target) e.target.value = ''; return }
  const request = ++seq
  busy.value = true
  uploadDone.value = 0
  uploadTotal.value = valid.length
  const next = [...current]
  try {
    for (const file of valid) {
      if (request !== seq) return
      uploadDone.value++
      try {
        const photo = await uploadEvidence(file)
        if (request === seq && photo?.id) next.push(photo)
      } catch (err) {
        if (request === seq) toast.error(`${file.name}: ${err?.message || 'Gagal mengunggah'}`)
      }
    }
    if (request === seq) emit('update:modelValue', next)
  } finally {
    if (request === seq) {
      busy.value = false
      uploadDone.value = 0
      uploadTotal.value = 0
    }
    if (e.target) e.target.value = ''
  }
}

function removeAt(idx) {
  emit('update:modelValue', items.value.filter((_, i) => i !== idx))
}
</script>
