<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Dokumen</h1>
        <p class="mt-1 text-sm text-gray-500">Simpan dokumen, temukan berdasarkan nomor atau tentang, dan pantau jumlah unduhannya.</p>
      </div>
      <AppButton v-if="canUpload" @click="openUpload">Unggah dokumen</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppCard>
      <div class="flex flex-col sm:flex-row gap-3 sm:items-end">
        <div class="flex-1">
          <AppInput v-model="search" type="search" label="Cari dokumen" placeholder="Nomor dokumen, tentang, atau nama berkas" maxlength="200" />
        </div>
        <AppButton variant="secondary" :loading="loading" @click="load()">Muat ulang</AppButton>
      </div>
      <p class="mt-3 text-sm text-gray-500" aria-live="polite">{{ total }} dokumen{{ search.trim() ? ' ditemukan' : ' tersimpan' }}</p>
    </AppCard>

    <AppCard :padding="false">
      <AppTable class="hidden md:block" :columns="columns" :rows="rows" :loading="loading" :empty-text="emptyText">
        <template #cell-document_number="{ row }">
          <span class="font-semibold text-gray-900 break-all">{{ row.document_number }}</span>
        </template>
        <template #cell-about="{ row }">
          <p class="max-w-sm whitespace-pre-line break-words text-gray-800">{{ row.about }}</p>
          <p class="mt-1 text-xs text-gray-500 break-all">{{ row.file_name }} · {{ formatSize(row.file_size) }}</p>
        </template>
        <template #cell-uploaded_at="{ row }">
          <time :datetime="row.uploaded_at" class="whitespace-nowrap">{{ formatDateTime(row.uploaded_at) }}</time>
          <p class="mt-1 text-xs text-gray-500">{{ row.uploaded_by }}</p>
        </template>
        <template #cell-download_count="{ row }"><span class="tabular-nums">{{ row.download_count }} kali</span></template>
        <template #cell-actions="{ row }">
          <AppActionButton v-if="canDownload" icon="download" :label="`Unduh ${row.document_number}`" :loading="downloading.has(row.id)" @click="download(row)" />
          <span v-else class="text-xs text-gray-400">Akses lihat</span>
        </template>
      </AppTable>

      <div class="md:hidden">
        <p v-if="loading" class="p-6 text-center text-sm text-gray-500">Memuat dokumen…</p>
        <p v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-500">{{ emptyText }}</p>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in rows" :key="row.id" class="p-4 space-y-3">
            <p class="font-semibold text-gray-900 break-all">{{ row.document_number }}</p>
            <p class="text-sm text-gray-700 whitespace-pre-line break-words">{{ row.about }}</p>
            <p class="text-xs text-gray-500 break-all">{{ row.file_name }} · {{ formatSize(row.file_size) }}</p>
            <dl class="grid grid-cols-2 gap-2 text-xs">
              <div><dt class="text-gray-500">Waktu unggah</dt><dd class="mt-1">{{ formatDateTime(row.uploaded_at) }}</dd><dd class="text-gray-500">{{ row.uploaded_by }}</dd></div>
              <div><dt class="text-gray-500">Diunduh</dt><dd class="mt-1">{{ row.download_count }} kali</dd></div>
            </dl>
            <AppButton v-if="canDownload" variant="secondary" size="sm" :loading="downloading.has(row.id)" :aria-label="`Unduh ${row.document_number}`" @click="download(row)">Unduh dokumen</AppButton>
          </li>
        </ul>
      </div>
    </AppCard>
    <AppPagination :page="page" :total="total" :per-page="20" @change="changePage" />

    <AppModal :model-value="showUpload" title="Unggah dokumen" size="lg" @update:model-value="closeUpload">
      <form id="document-upload-form" class="space-y-4" @submit.prevent="upload">
        <AppAlert type="error" :message="uploadError" />
        <AppInput v-model="form.number" label="Nomor dokumen" placeholder="Contoh: SOP/OPS/001/2026" required maxlength="120" :disabled="uploading" />
        <div>
          <label for="document-about" class="block text-sm font-medium text-gray-700 mb-1">Tentang</label>
          <textarea id="document-about" v-model="form.about" rows="3" required maxlength="1000" :disabled="uploading" placeholder="Judul atau keterangan isi dokumen" class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:outline-none" />
        </div>
        <div>
          <label for="document-file" class="block text-sm font-medium text-gray-700 mb-1">Berkas dokumen</label>
          <input id="document-file" type="file" required :disabled="uploading" accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.csv,.jpg,.jpeg,.png" class="block w-full min-w-0 text-sm text-gray-600 file:mr-3 file:py-2 file:px-3 file:rounded-lg file:border-0 file:bg-brand-50 file:text-brand-700" @change="pickFile" />
          <p class="mt-2 text-xs text-gray-500">PDF, Word, Excel, PowerPoint, TXT, CSV, JPG, atau PNG. Maksimal 20 MB per berkas.</p>
        </div>
        <p class="text-xs text-gray-500">Nomor dokumen harus unik. Waktu unggah dicatat otomatis setelah berkas tersimpan.</p>
        <div v-if="uploading" role="status" class="text-sm text-brand-700">
          {{ progress < 100 ? `Mengunggah ${progress}%…` : 'Menyimpan dokumen…' }}
          <div class="mt-2 h-2 bg-gray-100 rounded-full overflow-hidden"><div class="h-full bg-brand-500 transition-all" :style="{ width: progress + '%' }" /></div>
        </div>
      </form>
      <template #footer>
        <AppButton variant="secondary" :disabled="uploading" @click="closeUpload(false)">Batal</AppButton>
        <AppButton type="submit" form="document-upload-form" :loading="uploading" :disabled="!picked">Simpan dokumen</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { documentsApi } from '@/api/documents.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatDateTime } from '@/utils/format.js'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import AppTable from '@/components/ui/AppTable.vue'

const auth = useAuthStore()
const toast = useToastStore()
const canUpload = computed(() => auth.hasPermission('documents.create'))
const canDownload = computed(() => auth.hasPermission('documents.download'))
const rows = ref([]), total = ref(0), page = ref(1), search = ref('')
const loading = ref(false), errorMsg = ref('')
const showUpload = ref(false), uploading = ref(false), uploadError = ref(''), progress = ref(0), picked = ref(null)
const form = reactive({ number: '', about: '' })
const downloading = reactive(new Set())
const emptyText = computed(() => search.value.trim() ? 'Tidak ada dokumen yang cocok dengan pencarian.' : 'Belum ada dokumen tersimpan.')
const columns = [
  { key: 'document_number', label: 'Nomor dokumen' },
  { key: 'about', label: 'Tentang' },
  { key: 'uploaded_at', label: 'Waktu unggah (Timestamp)' },
  { key: 'download_count', label: 'Diunduh' },
  { key: 'actions', label: 'Aksi' },
]
let searchTimer, loadSequence = 0
function formatSize(bytes) { return bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.ceil(bytes / 1024))} KB` }
async function load(showLoading = true) {
  const sequence = ++loadSequence
  if (showLoading) loading.value = true
  errorMsg.value = ''
  try {
    const data = await documentsApi.list({ page: page.value, limit: 20, search: search.value.trim() })
    if (sequence !== loadSequence) return
    rows.value = data.items || []
    total.value = data.total || 0
  } catch (error) {
    if (sequence === loadSequence) errorMsg.value = error.message || 'Gagal memuat dokumen.'
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}
function changePage(value) { page.value = value; load() }
watch(search, () => {
  clearTimeout(searchTimer)
  ++loadSequence
  searchTimer = setTimeout(() => { page.value = 1; load() }, 300)
})
function openUpload() {
  form.number = ''; form.about = ''; picked.value = null; uploadError.value = ''; progress.value = 0
  showUpload.value = true
}
function closeUpload(value) { if (!uploading.value) showUpload.value = value }
function pickFile(event) {
  picked.value = event.target.files?.[0] || null
  uploadError.value = ''
  if (picked.value && (picked.value.size === 0 || picked.value.size > 20 * 1024 * 1024)) {
    uploadError.value = 'Pilih berkas yang tidak kosong, maksimal 20 MB.'
    picked.value = null
    event.target.value = ''
  }
}
async function upload() {
  if (uploading.value || !picked.value) return
  if (!form.number.trim() || !form.about.trim()) { uploadError.value = 'Nomor dokumen dan tentang wajib diisi.'; return }
  uploading.value = true; uploadError.value = ''; progress.value = 0
  try {
    await documentsApi.upload(form.number.trim(), form.about.trim(), picked.value, value => { progress.value = value })
    showUpload.value = false
    toast.success('Dokumen berhasil diunggah.')
    search.value = ''; page.value = 1
    clearTimeout(searchTimer)
    await load()
  } catch (error) {
    uploadError.value = error.message || 'Gagal mengunggah dokumen.'
  } finally { uploading.value = false }
}
async function download(row) {
  if (downloading.has(row.id)) return
  downloading.add(row.id)
  try {
    const blob = await documentsApi.download(row.id)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url; link.download = row.file_name
    document.body.appendChild(link); link.click(); link.remove()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
    await load(false)
  } catch (error) {
    toast.error(error.status === 404 ? 'Dokumen atau berkas tidak ditemukan.' : error.status === 403 ? 'Anda tidak memiliki izin mengunduh dokumen.' : 'Gagal mengunduh dokumen. Silakan coba lagi.')
  } finally { downloading.delete(row.id) }
}
onMounted(() => load())
onBeforeUnmount(() => { clearTimeout(searchTimer); ++loadSequence })
</script>
