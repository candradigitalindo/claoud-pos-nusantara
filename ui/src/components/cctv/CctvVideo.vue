<template>
  <div class="space-y-2">
    <div class="relative overflow-hidden rounded-lg bg-gray-950">
      <video ref="video" class="aspect-video w-full" controls autoplay muted playsinline :aria-label="`Video ${camera.name}`" @playing="onPlaying" />
      <div v-if="loading" class="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/30 text-sm text-white" role="status">Menghubungkan kamera…</div>
    </div>
    <div v-if="error" class="flex flex-wrap items-center justify-between gap-2 text-sm text-red-700" role="alert">
      <span>{{ error }}</span>
      <button type="button" class="shrink-0 font-semibold underline" @click="connect(true)">Coba lagi</button>
    </div>
    <p v-if="retrying" class="text-xs text-gray-500" role="status">Mencoba menyambung kembali…</p>
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Hls from 'hls.js'
import { camerasApi } from '@/api/cameras.js'
import { useAuthStore } from '@/stores/auth.js'

const props = defineProps({
  camera: { type: Object, required: true },
  mode: { type: String, default: 'live' },
  range: { type: Object, default: () => ({}) },
})
const auth = useAuthStore()
const video = ref(null), loading = ref(true), error = ref(''), retrying = ref(false)
let hls, stream, generation = 0, attempts = 0, disposed = false, suspended = false, retryTimer, loadTimer

function release(result, unloading = false) {
  if (!result?.name) return
  if (unloading) {
    const base = import.meta.env.VITE_API_BASE_URL || '/api/v1'
    fetch(`${base}/admin/cameras/${result.cameraId}/stop?name=${encodeURIComponent(result.name)}`, {
      method: 'POST', keepalive: true, headers: { Authorization: `Bearer ${auth.token}` },
    }).catch(() => {})
  } else camerasApi.stop(result.cameraId, result.name).catch(() => {})
}

function disconnect(unloading = false) {
  ++generation
  window.clearTimeout(retryTimer)
  window.clearTimeout(loadTimer)
  retrying.value = false
  hls?.destroy()
  hls = null
  if (video.value) {
    video.value.onerror = null
    video.value.pause()
    video.value.removeAttribute('src')
    video.value.load()
  }
  release(stream, unloading)
  stream = null
}

function fail(message, status) {
  if (disposed || suspended) return
  disconnect()
  loading.value = false
  error.value = message
  if (status === 401 || status === 403 || attempts >= 2) return
  retrying.value = true
  retryTimer = window.setTimeout(() => connect(false), 2000 * ++attempts)
}

async function connect(reset = false) {
  disconnect()
  if (disposed || suspended) return
  if (reset) attempts = 0
  const request = generation
  const cameraId = props.camera.id
  loading.value = true
  error.value = ''
  try {
    const result = props.mode === 'playback'
      ? await camerasApi.playback(cameraId, { ...props.range })
      : await camerasApi.live(cameraId)
    const opened = { ...result, cameraId }
    if (disposed || suspended || generation !== request) {
      release(opened, disposed || suspended)
      return
    }
    stream = opened
    await nextTick()
    if (!video.value || generation !== request) return
    loadTimer = window.setTimeout(() => {
      if (request === generation) fail('Kamera belum mengirim video. Periksa koneksi dan format video H.264.')
    }, 20000)
    video.value.muted = true
    if (Hls.isSupported()) {
      hls = new Hls({ maxBufferLength: 10, backBufferLength: 5 })
      hls.on(Hls.Events.ERROR, (_, data) => {
        if (!data.fatal || request !== generation) return
        const unsupported = ['bufferAddCodecError', 'manifestIncompatibleCodecsError'].includes(data.details)
        fail(unsupported ? 'Format video kamera tidak didukung browser ini. Pilih stream H.264 yang didukung atau gunakan browser lain.' : 'Tayangan terputus. Periksa koneksi kamera dan format video H.264.', unsupported ? 403 : data.response?.code)
      })
      hls.loadSource(result.hls_url)
      hls.attachMedia(video.value)
    } else if (video.value.canPlayType('application/vnd.apple.mpegurl')) {
      video.value.src = result.hls_url
      video.value.onerror = () => fail('Tayangan terputus. Periksa koneksi kamera dan format video H.264.')
    } else {
      fail('Browser ini belum mendukung tayangan CCTV. Gunakan browser yang mendukung video HLS.', 403)
      return
    }
    video.value.play().catch(() => {})
  } catch (e) {
    if (!disposed && !suspended && generation === request) fail(e.message, e.status)
  }
}

function onPlaying() {
  window.clearTimeout(loadTimer)
  loading.value = false
  error.value = ''
}

const onHide = () => { suspended = true; disconnect(true) }
const onShow = event => { if (event.persisted && suspended) { suspended = false; connect(true) } }
watch(() => [props.camera.id, props.mode, props.range.start, props.range.end], () => connect(true))
onMounted(() => {
  window.addEventListener('pagehide', onHide)
  window.addEventListener('pageshow', onShow)
  connect(true)
})
onBeforeUnmount(() => {
  disposed = true
  window.removeEventListener('pagehide', onHide)
  window.removeEventListener('pageshow', onShow)
  disconnect(true)
})
</script>
