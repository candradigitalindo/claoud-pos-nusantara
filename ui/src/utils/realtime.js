import { onMounted, onUnmounted } from 'vue'

/**
 * Langganan event realtime (SSE) dari server — perubahan data di backend /
 * device sync memicu event {type, outlet_id, at}, halaman me-refresh datanya
 * sendiri secara senyap.
 *
 * Memakai satu koneksi EventSource bersama per tab (ref-counted) supaya
 * berpindah halaman atau beberapa komponen sekaligus tidak membuka koneksi
 * ganda dan tidak mengenai batas koneksi paralel browser per domain.
 *
 * useRealtime(['transaction','order'], () => fetchData(), { debounceMs: 2000, pauseWhen: () => false })
 * - types: jenis event yang dipedulikan halaman ini (null/[] = semua)
 * - callback dipanggil ter-debounce (default 2 dtk) supaya burst mutasi
 *   hanya memicu satu refresh.
 * - pauseWhen: opsional, () => boolean; bila true saat jadwal tiba, refresh
 *   dilewati agar tidak mengganggu input yang sedang aktif diedit.
 * EventSource reconnect otomatis saat koneksi putus; halaman tetap berfungsi
 * normal tanpa SSE (fallback: refresh manual / polling yang sudah ada).
 */

const listeners = new Set()
let sharedEs = null
let sharedToken = null
let closeTimer = null

function ensureSharedSource(token) {
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
  if (sharedEs && sharedToken === token && sharedEs.readyState !== EventSource.CLOSED) {
    return
  }
  if (sharedEs) {
    try { sharedEs.close() } catch {}
    sharedEs = null
  }
  sharedToken = token
  const es = new EventSource(`/api/v1/admin/events?token=${encodeURIComponent(token)}`)
  es.onmessage = (e) => {
    let ev
    try {
      ev = JSON.parse(e.data)
    } catch {
      return // abaikan frame non-JSON / komentar heartbeat
    }
    for (const fn of listeners) {
      try { fn(ev) } catch {}
    }
  }
  // onerror: EventSource reconnect sendiri; tidak perlu penanganan khusus.
  sharedEs = es
}

function releaseSharedSource() {
  if (listeners.size > 0) return
  if (closeTimer) clearTimeout(closeTimer)
  // Tunda penutupan sebentar agar transisi rute antar-halaman realtime memakai
  // ulang koneksi SSE yang sama tanpa putus-sambung.
  closeTimer = setTimeout(() => {
    closeTimer = null
    if (listeners.size === 0 && sharedEs) {
      try { sharedEs.close() } catch {}
      sharedEs = null
      sharedToken = null
    }
  }, 150)
}

export function useRealtime(types, callback, { debounceMs = 2000, pauseWhen = null } = {}) {
  let timer = null
  let lastEv = null
  const wanted = types && types.length ? new Set(types) : null

  function schedule(ev) {
    lastEv = ev
    clearTimeout(timer)
    timer = setTimeout(() => {
      if (typeof pauseWhen === 'function') {
        try {
          if (pauseWhen()) return
        } catch {}
      }
      try { callback(lastEv) } catch {}
    }, debounceMs)
  }

  function onEvent(ev) {
    if (!wanted || (ev && wanted.has(ev.type))) {
      schedule(ev)
    }
  }

  onMounted(() => {
    const token = localStorage.getItem('cloud_pos_token')
    if (!token || typeof EventSource === 'undefined') return
    listeners.add(onEvent)
    ensureSharedSource(token)
  })

  onUnmounted(() => {
    clearTimeout(timer)
    timer = null
    listeners.delete(onEvent)
    releaseSharedSource()
  })
}
