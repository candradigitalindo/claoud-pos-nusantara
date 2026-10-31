<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">App POS</h1>
        <p class="mt-1 text-sm text-gray-500">Unduh aplikasi kasir, lihat pembaruan, dan kelola versi aplikasi.</p>
      </div>
      <AppButton variant="secondary" size="sm" :loading="loading" @click="load">Muat ulang</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <div v-if="loading && !files.length" class="rounded-2xl border border-emerald-100 bg-emerald-50/70 p-8 text-sm text-emerald-800" role="status">
      Memuat versi aplikasi terbaru…
    </div>

    <section v-else-if="latestApk" aria-label="APK terbaru" class="release-hero overflow-hidden rounded-2xl border border-emerald-200">
      <div class="grid gap-6 p-5 sm:p-7 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
        <div class="min-w-0">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <span class="inline-flex items-center gap-1.5 rounded-full bg-emerald-700 px-3 py-1 text-xs font-semibold text-white">
              <span class="h-1.5 w-1.5 rounded-full bg-emerald-200" aria-hidden="true" />APK Android terbaru
            </span>
            <span v-if="latestRelease" class="text-xs font-medium text-emerald-800">Paket rilis terverifikasi</span>
          </div>
          <h2 class="text-2xl font-bold tracking-tight text-emerald-950 sm:text-3xl">
            POS Nusantara <span v-if="releaseVersion(latestApk)" class="text-emerald-700">{{ releaseVersion(latestApk) }}</span>
          </h2>
          <p class="mt-2 max-w-2xl text-sm leading-relaxed text-emerald-900/80">
            {{ latestRelease?.summary || 'Unduh paket Android yang paling baru diunggah untuk memperbarui aplikasi kasir Anda.' }}
          </p>
          <dl class="mt-5 flex flex-wrap gap-x-7 gap-y-3 text-sm">
            <div>
              <dt class="text-xs text-emerald-800/75">Diunggah</dt>
              <dd class="mt-1 font-medium text-emerald-950">{{ fmtDate(latestApk.uploaded_at) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-emerald-800/75">Ukuran file</dt>
              <dd class="mt-1 font-medium text-emerald-950">{{ fmtSize(latestApk.size) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-emerald-800/75">Perangkat</dt>
              <dd class="mt-1 font-medium text-emerald-950">{{ latestRelease?.compatibility || 'Android · APK' }}</dd>
            </div>
          </dl>
        </div>
        <div class="flex min-w-0 flex-col gap-2 lg:w-56">
          <a :href="fullUrl(latestApk.url)" download class="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl bg-emerald-700 px-5 py-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-emerald-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-700">
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M12 3v12m0 0-4-4m4 4 4-4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" stroke-linecap="round" stroke-linejoin="round" /></svg>
            Unduh APK {{ releaseVersion(latestApk) }}
          </a>
          <button type="button" class="rounded-xl border border-emerald-200 bg-white/80 px-4 py-2.5 text-sm font-medium text-emerald-800 transition-colors hover:bg-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-700" @click="copyUrl(latestApk)">
            {{ copied === latestApk.name ? 'Tautan tersalin' : 'Salin tautan unduh' }}
          </button>
          <p class="mt-1 break-all text-center text-xs leading-relaxed text-emerald-800/70">{{ latestApk.name }}</p>
        </div>
      </div>
      <div v-if="latestRelease" class="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-emerald-200/70 bg-white/45 px-5 py-3 text-xs text-emerald-900 sm:px-7">
        <span>✓ Kompatibel sebagai pembaruan versi sebelumnya</span>
        <span>✓ Paket universal untuk tiga arsitektur Android</span>
      </div>
    </section>

    <AppCard v-else-if="!errorMsg" class="text-center">
      <h2 class="font-semibold text-gray-900">Belum ada APK Android</h2>
      <p class="mt-1 text-sm text-gray-500">{{ canUpload ? 'Unggah APK melalui bagian kelola aplikasi di bawah.' : 'APK akan tersedia setelah pengelola mengunggah versi aplikasi.' }}</p>
    </AppCard>

    <div v-if="latestApk" class="grid gap-5 lg:grid-cols-2">
      <AppCard>
        <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
          <h2 class="font-semibold text-gray-900">Yang baru {{ releaseVersion(latestApk) }}</h2>
          <span class="rounded-md bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700">Catatan rilis</span>
        </div>
        <ul v-if="latestRelease" class="space-y-4">
          <li v-for="note in latestRelease.highlights" :key="note.title" class="flex gap-3">
            <span class="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-sm text-emerald-700" aria-hidden="true">✓</span>
            <div>
              <h3 class="text-sm font-semibold text-gray-900">{{ note.title }}</h3>
              <p class="mt-1 text-sm leading-relaxed text-gray-500">{{ note.description }}</p>
            </div>
          </li>
        </ul>
        <p v-else class="text-sm leading-relaxed text-gray-500">Catatan perubahan untuk file ini belum tersedia. Periksa informasi rilis dari pengelola sebelum memperbarui perangkat.</p>
      </AppCard>

      <AppCard>
        <h2 class="mb-4 font-semibold text-gray-900">Cara memperbarui aplikasi</h2>
        <ol class="space-y-4">
          <li v-for="(step, index) in INSTALL_STEPS" :key="step.title" class="flex gap-3">
            <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600">{{ index + 1 }}</span>
            <div>
              <h3 class="text-sm font-semibold text-gray-900">{{ step.title }}</h3>
              <p class="mt-1 text-sm leading-relaxed text-gray-500">{{ step.description }}</p>
            </div>
          </li>
        </ol>
        <p v-if="latestRelease" class="mt-4 rounded-lg bg-emerald-50 px-3 py-2.5 text-xs leading-relaxed text-emerald-800">Sinkronisasi katalog Cloud diperlukan agar jenis Makanan, Minuman, Aktivitas, dan Lainnya tersedia di tablet.</p>
      </AppCard>
    </div>

    <section aria-label="Arsip aplikasi">
      <AppCard :padding="false">
        <div class="space-y-4 border-b border-gray-100 p-5">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 class="font-semibold text-gray-900">Arsip aplikasi</h2>
              <p class="mt-1 text-xs text-gray-500">Semua file yang tersedia, diurutkan dari unggahan terbaru.</p>
            </div>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">{{ files.length }} file</span>
          </div>
          <div class="flex flex-col gap-3 sm:flex-row">
            <label class="min-w-0 flex-1">
              <span class="sr-only">Cari file aplikasi</span>
              <input v-model="search" type="search" placeholder="Cari nama atau versi aplikasi…" class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-100" />
            </label>
            <SearchableSelect v-model="formatFilter" aria-label="Filter format" search-placeholder="Cari format…" class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 sm:w-44">
                <option value="">Semua format</option>
                <option v-for="format in FILE_FORMATS" :key="format" :value="format">{{ format.toUpperCase() }}</option>
            </SearchableSelect>
          </div>
        </div>
        <p v-if="loading" class="p-8 text-center text-sm text-gray-500" role="status">Memuat daftar aplikasi…</p>
        <div v-else-if="!filteredFiles.length" class="p-8 text-center">
          <p class="text-sm text-gray-600">{{ errorMsg ? 'Daftar aplikasi belum dapat dimuat.' : files.length ? 'Tidak ada file yang cocok dengan pencarian.' : 'Belum ada file aplikasi yang diunggah.' }}</p>
          <button v-if="files.length && (search || formatFilter)" type="button" class="mt-2 text-sm font-medium text-emerald-700 hover:underline" @click="search = ''; formatFilter = ''">Hapus filter</button>
        </div>
        <ul v-else class="divide-y divide-gray-100">
          <li v-for="row in filteredFiles" :key="row.name" class="grid min-w-0 gap-3 px-5 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div class="flex min-w-0 gap-3">
              <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-xs font-bold text-emerald-700">{{ fileExtension(row).toUpperCase() || 'FILE' }}</span>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <p class="text-sm font-semibold text-gray-900">{{ platformLabel(row) }}<span v-if="releaseVersion(row)"> · {{ releaseVersion(row) }}</span></p>
                  <span v-if="row.name === latestApk?.name" class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">APK terbaru</span>
                </div>
                <p class="mt-1 break-all text-xs text-gray-500">{{ row.name }}</p>
                <p class="mt-1 text-xs text-gray-500">{{ fmtSize(row.size) }} <span class="mx-1 text-gray-300">·</span> {{ fmtDate(row.uploaded_at) }}</p>
              </div>
            </div>
            <div class="flex items-center gap-1.5 md:justify-end">
              <a :href="fullUrl(row.url)" download :aria-label="`Unduh ${row.name}`" class="inline-flex min-h-9 items-center rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 hover:border-emerald-300 hover:text-emerald-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-600">Unduh</a>
              <AppActionButton :icon="copied === row.name ? 'approve' : 'copy'" :label="copied === row.name ? 'Tautan tersalin' : `Salin tautan ${row.name}`" @click="copyUrl(row)" />
              <AppActionButton v-if="canDelete" icon="delete" :label="`Hapus ${row.name}`" @click="confirmDelete(row)" />
            </div>
          </li>
        </ul>
        <p v-if="files.length && (search || formatFilter)" class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500" aria-live="polite">Menampilkan {{ filteredFiles.length }} dari {{ files.length }} file.</p>
      </AppCard>
    </section>

    <AppCard v-if="canUpload">
      <details>
        <summary class="cursor-pointer text-sm font-semibold text-gray-900">Kelola aplikasi · Unggah versi baru</summary>
        <p class="mb-4 mt-2 text-sm text-gray-500">File tersimpan sebagai versi baru agar unggahan sebelumnya tetap tersedia.</p>
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
          <label class="min-w-0 flex-1">
            <span class="sr-only">File aplikasi</span>
            <input ref="fileInput" type="file" accept=".apk,.aab,.ipa,.zip,.exe,.dmg" :disabled="uploading" @change="onPick" class="block w-full cursor-pointer text-sm text-gray-600 file:mr-3 file:rounded-lg file:border-0 file:bg-emerald-50 file:px-4 file:py-2 file:text-sm file:font-medium file:text-emerald-700 hover:file:bg-emerald-100" />
          </label>
          <AppButton class="w-full shrink-0 sm:w-auto" :loading="uploading" :disabled="!picked" @click="doUpload">{{ uploading ? `Mengunggah ${progress}%` : 'Unggah aplikasi' }}</AppButton>
        </div>
        <div v-if="uploading" class="mt-3 h-2 overflow-hidden rounded-full bg-gray-100" role="progressbar" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100" aria-label="Progres unggahan">
          <div class="h-full bg-emerald-500 transition-all" :style="{ width: progress + '%' }" />
        </div>
        <p class="mt-3 text-xs leading-relaxed text-gray-500">Maksimal 200 MB. Format APK, AAB, IPA, ZIP, EXE, dan DMG. Nomor versi ditambahkan otomatis pada nama file.</p>
      </details>
    </AppCard>
    <p class="sr-only" aria-live="polite">{{ copied ? 'Tautan unduh berhasil disalin.' : '' }}</p>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { appFilesApi } from '@/api/appFiles.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import SearchableSelect from '@/components/ui/SearchableSelect.vue'

// Release evidence: docs/pos-master-integration.md, "Rilis produksi — 31 Oktober 2026".
// Match the exact artifact so future uploads never inherit these release claims.
const RELEASES = {
  'app-universal-release-v10.18.apk': {
    size: 64178671,
    summary: 'Pembaruan tutup kasir dan rincian menu terjual, dengan jenis produk yang mengikuti katalog Cloud.',
    compatibility: 'Android 7.0+ · Universal',
    highlights: [
      { title: 'Penutupan kasir lebih konsisten', description: 'Shift yang sudah ditutup tetap tertutup saat sinkronisasi. Laporan membedakan rincian harian dan total seluruh shift.' },
      { title: 'Empat jenis produk', description: 'Menu dikelompokkan menjadi Makanan, Minuman, Aktivitas, dan Lainnya. Produk ATV Overland termasuk Aktivitas.' },
      { title: 'Rincian menu pada struk', description: 'Struk tutup kasir dan serah-terima memuat nama menu, jumlah per jenis, dan total unit terjual.' },
    ],
  },
}
const INSTALL_STEPS = [
  { title: 'Unduh APK di perangkat kasir', description: 'Buka halaman ini dari tablet atau kirim tautan unduh ke perangkat Android yang digunakan.' },
  { title: 'Pasang sebagai pembaruan', description: 'Buka file APK, lalu pilih Perbarui / Update pada aplikasi yang sudah terpasang. Tidak perlu menghapus aplikasi lama.' },
  { title: 'Buka POS dan sinkronkan Cloud', description: 'Jalankan sinkronisasi katalog Cloud setelah pembaruan, lalu periksa menu dan koneksi printer sebelum mulai digunakan.' },
]
const FILE_FORMATS = ['apk', 'aab', 'ipa', 'zip', 'exe', 'dmg']
const PLATFORMS = { apk: 'Android', aab: 'Android App Bundle', ipa: 'iOS', zip: 'Arsip ZIP', exe: 'Windows', dmg: 'macOS' }
const toast = useToastStore()
const auth = useAuthStore()
const canUpload = computed(() => auth.hasPermission('appfiles.create'))
const canDelete = computed(() => auth.hasPermission('appfiles.delete'))
const files = ref([])
const loading = ref(true)
const errorMsg = ref('')
const search = ref('')
const formatFilter = ref('')
const picked = ref(null)
const uploading = ref(false)
const progress = ref(0)
const copied = ref('')
const fileInput = ref(null)
let copyTimer

const sortedFiles = computed(() => [...files.value].sort((a, b) => {
  const byDate = timestamp(b.uploaded_at) - timestamp(a.uploaded_at)
  return byDate || b.name.localeCompare(a.name, undefined, { numeric: true })
}))
const latestApk = computed(() => sortedFiles.value.find(file => fileExtension(file) === 'apk'))
const latestRelease = computed(() => {
  const file = latestApk.value
  const release = file && RELEASES[file.name]
  return release && Number(file.size) === release.size ? release : null
})
const filteredFiles = computed(() => {
  const query = search.value.trim().toLocaleLowerCase('id-ID')
  return sortedFiles.value.filter(file => (
    (!formatFilter.value || fileExtension(file) === formatFilter.value) &&
    (!query || `${file.name} ${platformLabel(file)}`.toLocaleLowerCase('id-ID').includes(query))
  ))
})

function fileExtension(file) { return file.name.split('.').pop().toLowerCase() }
function platformLabel(file) { return PLATFORMS[fileExtension(file)] || 'File aplikasi' }
function releaseVersion(file) { return file.name.match(/(?:^|[-_])(v\d+(?:\.\d+)*)(?=\.[a-z]+$|[-_]|$)/i)?.[1] || '' }
function timestamp(value) { return Date.parse(value) || 0 }
function fullUrl(value) { return new URL(value, window.location.origin).href }
function fmtSize(bytes) {
  const size = Number(bytes)
  if (bytes == null || !Number.isFinite(size) || size < 0) return '—'
  if (size >= 1_000_000) return `${(size / 1_000_000).toLocaleString('id-ID', { maximumFractionDigits: 1 })} MB`
  if (size >= 1_000) return `${(size / 1_000).toLocaleString('id-ID', { maximumFractionDigits: 0 })} KB`
  return `${size} B`
}
function fmtDate(value) {
  if (!value || !Number.isFinite(Date.parse(value))) return 'Tanggal tidak tersedia'
  return `${new Date(value).toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })} WIB`
}

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const data = await appFilesApi.list()
    files.value = Array.isArray(data) ? data : (Array.isArray(data?.data) ? data.data : [])
  } catch (error) {
    errorMsg.value = error?.message || 'Gagal memuat daftar file. Coba muat ulang.'
  } finally {
    loading.value = false
  }
}

function onPick(event) {
  const file = event.target.files?.[0] || null
  picked.value = null
  if (!file) return
  if (!FILE_FORMATS.includes(fileExtension(file))) {
    toast.error('Format tidak didukung. Gunakan APK, AAB, IPA, ZIP, EXE, atau DMG.')
    event.target.value = ''
    return
  }
  if (file.size > 200 * 1024 * 1024) {
    toast.error('Ukuran file maksimal 200 MB.')
    event.target.value = ''
    return
  }
  picked.value = file
}

async function doUpload() {
  if (!canUpload.value || !picked.value || uploading.value) return
  uploading.value = true
  progress.value = 0
  errorMsg.value = ''
  try {
    await appFilesApi.upload(picked.value, value => { progress.value = value })
    toast.success('File berhasil diunggah')
    picked.value = null
    if (fileInput.value) fileInput.value.value = ''
    await load()
  } catch (error) {
    toast.error(error?.response?.data?.error || error?.message || 'Gagal mengunggah')
  } finally {
    uploading.value = false
  }
}

async function copyUrl(row) {
  try {
    await navigator.clipboard.writeText(fullUrl(row.url))
    copied.value = row.name
    clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copied.value = '' }, 2000)
  } catch { toast.error('Gagal menyalin tautan') }
}

async function confirmDelete(row) {
  if (!canDelete.value || !window.confirm(`Hapus file "${row.name}"? Tautan unduhnya tidak akan tersedia lagi.`)) return
  try {
    await appFilesApi.remove(row.name)
    toast.success('File dihapus')
    await load()
  } catch (error) { toast.error(error?.response?.data?.error || error?.message || 'Gagal menghapus') }
}

onMounted(load)
onUnmounted(() => clearTimeout(copyTimer))
</script>

<style scoped>
.release-hero {
  background: radial-gradient(ellipse at top right, #d1fae5 0%, transparent 55%), linear-gradient(120deg, #f0fdf4, #ecfdf5);
}
</style>
