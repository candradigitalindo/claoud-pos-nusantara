<template>
  <AppModal :model-value="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    title="Impor Aset dari Excel" size="2xl">
    <div class="space-y-4">
      <!-- Langkah 1: template -->
      <div class="rounded-xl border border-gray-200 p-3">
        <div class="flex flex-wrap items-start justify-between gap-2">
          <div class="min-w-0">
            <p class="text-sm font-semibold text-gray-900">1. Unduh template</p>
            <p class="mt-0.5 text-xs text-gray-500">
              Berisi contoh pengisian, panduan tiap kolom, serta daftar kategori dan kode outlet
              yang berlaku — jadi isian Anda tidak akan ditolak karena salah nama.
            </p>
          </div>
          <button class="btn-soft shrink-0" :disabled="downloading" @click="downloadTemplate">
            {{ downloading ? 'Menyiapkan…' : 'Unduh Template' }}
          </button>
        </div>
      </div>

      <!-- Langkah 2: unggah -->
      <div class="rounded-xl border border-gray-200 p-3">
        <p class="text-sm font-semibold text-gray-900">2. Unggah berkas terisi</p>
        <p class="mt-0.5 text-xs text-gray-500">
          Berkas diperiksa dulu. Tidak ada yang tersimpan sebelum Anda menekan Simpan.
        </p>
        <input ref="fileInput" type="file" accept=".xlsx" class="hidden" @change="onPick" />
        <div class="mt-2 flex flex-wrap items-center gap-2">
          <button class="btn-soft" :disabled="checking" @click="$refs.fileInput.click()">
            {{ checking ? 'Memeriksa…' : (fileName || 'Pilih Berkas .xlsx') }}
          </button>
          <button v-if="result" class="text-xs text-gray-500 underline" @click="reset">ganti berkas</button>
        </div>
      </div>

      <AppAlert type="error" :message="errorMsg" />

      <!-- Langkah 3: hasil pemeriksaan -->
      <template v-if="result">
        <div class="grid grid-cols-3 gap-2">
          <div class="rounded-xl border border-gray-200 p-3">
            <p class="stat-lbl">Baris Terbaca</p>
            <p class="stat-val">{{ result.total_rows }}</p>
          </div>
          <div class="rounded-xl border p-3" :class="result.valid_rows ? 'border-emerald-200 bg-emerald-50' : 'border-gray-200'">
            <p class="stat-lbl">Siap Disimpan</p>
            <p class="stat-val text-emerald-700">{{ result.valid_rows }}</p>
            <p class="stat-sub">{{ result.assets_made }} aset akan terbentuk</p>
          </div>
          <div class="rounded-xl border p-3" :class="result.error_rows ? 'border-red-200 bg-red-50' : 'border-gray-200'">
            <p class="stat-lbl">Perlu Diperbaiki</p>
            <p class="stat-val" :class="result.error_rows ? 'text-red-600' : ''">{{ result.error_rows }}</p>
          </div>
        </div>

        <div v-if="errorRows.length" class="rounded-xl border border-red-200">
          <p class="border-b border-red-100 bg-red-50 p-2 text-xs font-bold uppercase tracking-wide text-red-700">
            Baris yang perlu diperbaiki di Excel
          </p>
          <ul class="max-h-60 divide-y divide-gray-100 overflow-y-auto">
            <li v-for="r in errorRows" :key="r.row" class="p-2.5">
              <p class="text-sm font-medium text-gray-900">
                Baris {{ r.row }}<span v-if="r.name"> — {{ r.name }}</span>
              </p>
              <ul class="mt-0.5 list-inside list-disc text-xs text-red-600">
                <li v-for="(e, i) in r.errors" :key="i">{{ e }}</li>
              </ul>
            </li>
          </ul>
        </div>

        <div v-if="validRows.length" class="rounded-xl border border-gray-200">
          <p class="border-b border-gray-100 bg-gray-50 p-2 text-xs font-bold uppercase tracking-wide text-gray-500">
            Siap disimpan
          </p>
          <div class="max-h-48 overflow-y-auto">
            <table class="w-full text-sm">
              <tbody class="divide-y divide-gray-100">
                <tr v-for="r in validRows" :key="r.row">
                  <td class="p-2 text-xs text-gray-400">{{ r.row }}</td>
                  <td class="p-2 text-gray-800">{{ r.name }}</td>
                  <td class="p-2 text-xs text-gray-500">{{ r.category || '—' }}</td>
                  <td class="p-2 text-right text-xs text-gray-600">{{ r.qty }} · {{ r.mode }}</td>
                  <td class="p-2 text-right text-xs text-gray-700">{{ formatRupiah(r.price) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <p v-if="saved" class="rounded-lg bg-emerald-50 p-3 text-sm text-emerald-800">{{ result.message }}</p>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button type="button" class="btn-ghost" @click="close">{{ saved ? 'Tutup' : 'Batal' }}</button>
        <AppButton v-if="result && result.valid_rows && !saved" :loading="saving" @click="save">
          Simpan {{ result.valid_rows }} Baris
        </AppButton>
      </div>
    </template>
  </AppModal>
</template>

<script setup>
import { ref, computed } from 'vue'
import { assetImportApi } from '@/api/assetImport.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'

defineProps({ modelValue: { type: Boolean, default: false } })
const emit = defineEmits(['update:modelValue', 'done'])

const toast = useToastStore()
const fileInput = ref(null)
const file = ref(null)
const fileName = ref('')
const result = ref(null)
const errorMsg = ref('')
const downloading = ref(false)
const checking = ref(false)
const saving = ref(false)
const saved = ref(false)

function asObject(d) { return d?.data ?? d ?? null }
const validRows = computed(() => (result.value?.rows || []).filter(r => r.valid))
const errorRows = computed(() => (result.value?.rows || []).filter(r => !r.valid))

async function downloadTemplate() {
  downloading.value = true
  try {
    const blob = await assetImportApi.template()
    const url = URL.createObjectURL(blob instanceof Blob ? blob : new Blob([blob]))
    const a = document.createElement('a')
    a.href = url
    a.download = 'Template-Pendataan-Aset.xlsx'
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch (e) { toast.error(e?.message || 'Gagal mengunduh template') } finally { downloading.value = false }
}

async function onPick(e) {
  const f = e.target.files?.[0]
  if (!f) return
  file.value = f
  fileName.value = f.name
  saved.value = false
  await check()
  if (e.target) e.target.value = ''
}

async function check() {
  checking.value = true; errorMsg.value = ''; result.value = null
  try {
    result.value = asObject(await assetImportApi.upload(file.value, 'preview'))
  } catch (e) {
    errorMsg.value = e?.message || 'Berkas tidak bisa diproses'
  } finally { checking.value = false }
}

async function save() {
  saving.value = true
  try {
    result.value = asObject(await assetImportApi.upload(file.value, 'commit'))
    saved.value = true
    toast.success(result.value?.message || 'Aset tersimpan')
    emit('done')
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

function reset() { file.value = null; fileName.value = ''; result.value = null; saved.value = false; errorMsg.value = '' }
function close() { reset(); emit('update:modelValue', false) }
</script>

<style scoped>
.btn-soft {
  display: inline-flex; align-items: center; justify-content: center; gap: .4rem;
  padding: .5rem .9rem; border-radius: .6rem; font-size: .82rem; font-weight: 600;
  color: #374151; background: #fff; border: 1px solid rgba(0,0,0,.12); min-height: 40px;
}
.btn-soft:hover:not(:disabled) { background: #f3f4f6; border-color: rgba(5,150,105,.4); color: #047857; }
.btn-soft:disabled { opacity: .6; }
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; min-height: 40px; }
.btn-ghost:hover { background: #e5e7eb; }
.stat-lbl { font-size: .65rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .1rem; font-size: 1.1rem; font-weight: 700; color: #111827; }
.stat-sub { margin-top: .1rem; font-size: .68rem; color: #6b7280; }
</style>
