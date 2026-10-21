<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between flex-wrap gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">App POS</h1>
        <p class="text-sm text-gray-500 mt-0.5">Unggah file aplikasi (APK, dll) — URL download dibuat otomatis.</p>
      </div>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <!-- Upload card -->
    <AppCard v-if="canUpload">
      <div class="flex flex-col sm:flex-row sm:items-center gap-3">
        <label class="flex-1 min-w-0">
          <input ref="fileInput" type="file" accept=".apk,.aab,.ipa,.zip,.exe,.dmg"
            @change="onPick"
            class="block w-full text-sm text-gray-600 file:mr-3 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-emerald-50 file:text-emerald-700 hover:file:bg-emerald-100 cursor-pointer" />
        </label>
        <AppButton class="w-full sm:w-auto shrink-0" :loading="uploading" :disabled="!picked" @click="doUpload">
          {{ uploading ? `Mengunggah ${progress}%` : 'Unggah' }}
        </AppButton>
      </div>
      <div v-if="uploading" class="mt-3 h-2 bg-gray-100 rounded-full overflow-hidden">
        <div class="h-full bg-emerald-500 transition-all" :style="{ width: progress + '%' }" />
      </div>
      <p class="text-xs text-gray-400 mt-2">Maks 200MB. Format: APK, AAB, IPA, ZIP, EXE, DMG. Tiap upload diberi versi otomatis (mis. <span class="font-mono">-v1</span>, <span class="font-mono">-v2</span>) — build lama tidak tertimpa.</p>
    </AppCard>

    <!-- Files list -->
    <AppCard :padding="false">
      <!-- Mobile: stacked cards -->
      <div class="sm:hidden">
        <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
        <div v-else-if="!files.length" class="p-6 text-center text-sm text-gray-400">Belum ada file. Unggah APK di atas.</div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in files" :key="row.name" class="p-4 space-y-3">
            <div class="flex items-start gap-2">
              <svg class="w-5 h-5 text-emerald-600 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M12 3v12m0 0l-4-4m4 4l4-4M4 17v2a2 2 0 002 2h12a2 2 0 002-2v-2"/></svg>
              <div class="min-w-0 flex-1">
                <p class="font-medium text-gray-900 break-all text-sm leading-snug">{{ row.name }}</p>
                <p class="text-xs text-gray-500 mt-0.5">{{ fmtSize(row.size) }} · {{ fmtDate(row.uploaded_at) }}</p>
              </div>
            </div>
            <div class="flex items-center gap-1.5">
              <input :value="fullUrl(row.url)" readonly
                class="flex-1 min-w-0 text-xs font-mono text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1.5" />
              <AppActionButton :icon="copied === row.name ? 'approve' : 'copy'" :label="copied === row.name ? 'Tersalin!' : 'Salin URL'" @click="copyUrl(row)" />
            </div>
            <div class="flex gap-2">
              <AppActionButton icon="download" label="Download" :href="fullUrl(row.url)" target="_blank" rel="noopener" />
              <AppActionButton icon="delete" label="Hapus" v-if="canDelete" @click="confirmDelete(row)" />
            </div>
          </li>
        </ul>
      </div>

      <!-- Desktop: table -->
      <AppTable class="hidden sm:block" :columns="COLUMNS" :rows="files" :loading="loading" emptyText="Belum ada file. Unggah APK di atas.">
        <template #cell-name="{ row }">
          <div class="flex items-center gap-2">
            <svg class="w-5 h-5 text-emerald-600 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M12 3v12m0 0l-4-4m4 4l4-4M4 17v2a2 2 0 002 2h12a2 2 0 002-2v-2"/></svg>
            <span class="font-medium text-gray-900 break-all">{{ row.name }}</span>
          </div>
        </template>
        <template #cell-size="{ row }">
          <span class="text-sm text-gray-600">{{ fmtSize(row.size) }}</span>
        </template>
        <template #cell-uploaded_at="{ row }">
          <span class="text-sm text-gray-500">{{ fmtDate(row.uploaded_at) }}</span>
        </template>
        <template #cell-url="{ row }">
          <div class="flex items-center gap-1.5">
            <input :value="fullUrl(row.url)" readonly
              class="text-xs font-mono text-gray-600 bg-gray-50 border border-gray-200 rounded px-2 py-1 w-[260px] max-w-full" />
            <AppActionButton :icon="copied === row.name ? 'approve' : 'copy'" :label="copied === row.name ? 'Tersalin!' : 'Salin URL'" @click="copyUrl(row)" />
          </div>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <AppActionButton icon="download" label="Download" :href="fullUrl(row.url)" target="_blank" rel="noopener" />
            <AppActionButton icon="delete" label="Hapus" v-if="canDelete" @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
    </AppCard>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, onMounted } from 'vue'
import { appFilesApi } from '@/api/appFiles.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppTable from '@/components/ui/AppTable.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canUpload = auth.hasPermission('appfiles.create')
const canDelete = auth.hasPermission('appfiles.delete')

const files = ref([])
const loading = ref(false)
const errorMsg = ref('')
const picked = ref(null)
const uploading = ref(false)
const progress = ref(0)
const copied = ref('')
const fileInput = ref(null)

const COLUMNS = [
  { key: 'name',        label: 'File',     sortable: false },
  { key: 'size',        label: 'Ukuran',   sortable: false },
  { key: 'uploaded_at', label: 'Diunggah', sortable: false },
  { key: 'url',         label: 'URL Download', sortable: false },
  { key: 'actions',     label: '',         sortable: false },
]

function fullUrl(u) { return window.location.origin + u }
function fmtSize(b) {
  if (b == null) return '—'
  if (b >= 1024 * 1024) return (b / 1024 / 1024).toFixed(1) + ' MB'
  if (b >= 1024) return (b / 1024).toFixed(0) + ' KB'
  return b + ' B'
}
function fmtDate(s) {
  if (!s) return '—'
  return new Date(s).toLocaleString('id-ID', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

async function load() {
  loading.value = true; errorMsg.value = ''
  try {
    // Non-paginated list: the API client unwraps {success,data} → returns the array directly.
    const data = await appFilesApi.list()
    files.value = Array.isArray(data) ? data : (data?.data || [])
  } catch (e) {
    errorMsg.value = e?.message || 'Gagal memuat daftar file'
  } finally {
    loading.value = false
  }
}

function onPick(e) { picked.value = e.target.files?.[0] || null }

async function doUpload() {
  if (!picked.value) return
  uploading.value = true; progress.value = 0; errorMsg.value = ''
  try {
    await appFilesApi.upload(picked.value, (p) => { progress.value = p })
    toast.success('File berhasil diunggah')
    picked.value = null
    if (fileInput.value) fileInput.value.value = ''
    load()
  } catch (e) {
    toast.error(e?.response?.data?.error || e?.message || 'Gagal mengunggah')
  } finally {
    uploading.value = false
  }
}

async function copyUrl(row) {
  try {
    await navigator.clipboard.writeText(fullUrl(row.url))
    copied.value = row.name
    setTimeout(() => { if (copied.value === row.name) copied.value = '' }, 1500)
  } catch { toast.error('Gagal menyalin URL') }
}

async function confirmDelete(row) {
  if (!window.confirm(`Hapus file "${row.name}"? URL download-nya akan mati.`)) return
  try {
    await appFilesApi.remove(row.name)
    toast.success('File dihapus')
    load()
  } catch (e) { toast.error(e?.response?.data?.error || e?.message || 'Gagal menghapus') }
}

onMounted(load)
</script>
