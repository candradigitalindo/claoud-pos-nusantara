<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="min-w-0">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Broadcast WhatsApp</h1>
      <p class="mt-0.5 text-sm text-gray-500">
        Kirim satu pesan ke banyak nomor: pelanggan dari master pelanggan, penerima internal, atau daftar manual.
        Pesan dikirim satu per satu dengan jeda dan menghormati jam tenang serta batas harian.
      </p>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <div class="grid gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
      <AppCard>
        <h2 class="sec-title">Buat broadcast</h2>
        <form class="mt-3 space-y-3" @submit.prevent="send">
          <div>
            <label class="lbl">Judul (internal)</label>
            <input v-model="form.title" class="form-input" placeholder="Promo akhir pekan" required />
          </div>
          <div>
            <label class="lbl">Audiens</label>
            <select v-model="form.audience" class="form-input">
              <option value="customers">Pelanggan (dari master pelanggan)</option>
              <option value="recipients">Penerima notifikasi internal</option>
              <option value="manual">Daftar nomor manual</option>
            </select>
          </div>

          <div v-if="form.audience === 'customers'" class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="lbl">Outlet</label>
              <select v-model="form.outlet_id" class="form-input">
                <option value="">Semua outlet</option>
                <option v-for="o in outlets" :key="o.id" :value="o.id">{{ o.name }}</option>
              </select>
            </div>
            <div>
              <label class="lbl">Minimal kunjungan</label>
              <input v-model.number="form.min_visits" type="number" min="1" class="form-input" />
            </div>
            <div>
              <label class="lbl">Datang dalam … hari terakhir (0 = abaikan)</label>
              <input v-model.number="form.last_visit_days" type="number" min="0" class="form-input" />
            </div>
            <div>
              <label class="lbl">Tidak datang ≥ … hari (0 = abaikan)</label>
              <input v-model.number="form.not_visited_days" type="number" min="0" class="form-input" />
              <p class="hint">Untuk kampanye "ajak kembali".</p>
            </div>
          </div>

          <div v-if="form.audience === 'manual'">
            <label class="lbl">Daftar nomor (satu per baris, boleh "nomor, nama")</label>
            <textarea v-model="form.manual" rows="5" class="form-input font-mono text-xs" placeholder="0812xxxx, Budi&#10;0813xxxx" />
          </div>

          <div>
            <label class="lbl">Pesan</label>
            <textarea v-model="form.body" rows="7" class="form-input" placeholder="Halo {name}, …" required />
            <p class="hint">Gunakan <code>{name}</code> untuk nama penerima. Format WhatsApp: *tebal*, _miring_.</p>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label class="lbl">URL gambar (opsional)</label>
              <input v-model="form.image_url" class="form-input" placeholder="https://…/promo.jpg" />
              <p class="hint">JPEG/PNG maks. 6 MB; pesan menjadi keterangan gambar.</p>
            </div>
            <div>
              <label class="lbl">Jadwal kirim (opsional)</label>
              <input v-model="sendAtLocal" type="datetime-local" class="form-input" />
              <p class="hint">Kosong = mulai sekarang (zona aplikasi).</p>
            </div>
          </div>

          <div v-if="previewData" class="rounded-lg bg-gray-50 p-3 text-xs text-gray-700">
            <p class="font-semibold text-gray-900">{{ previewData.total }} penerima cocok.</p>
            <p v-if="previewData.sample?.length" class="mt-1">
              Contoh: {{ previewData.sample.map(s => (s.name ? s.name + ' ' : '') + '+' + s.target).join(', ') }}
            </p>
            <p class="mt-1 text-gray-500">
              Perkiraan durasi: ~{{ estimateMinutes(previewData.total) }} menit dengan jeda saat ini.
            </p>
          </div>

          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <AppButton type="button" variant="secondary" :loading="busy.preview" @click="preview">Hitung penerima</AppButton>
            <AppButton type="submit" :loading="busy.send">Kirim broadcast</AppButton>
          </div>
        </form>
      </AppCard>

      <AppCard>
        <div class="flex items-center justify-between gap-2">
          <h2 class="sec-title">Riwayat broadcast</h2>
          <button class="text-xs font-semibold text-emerald-700 hover:underline" @click="load">Segarkan</button>
        </div>
        <div class="mt-3 space-y-2">
          <p v-if="!list.length" class="py-8 text-center text-sm text-gray-400">Belum ada broadcast.</p>
          <div v-for="b in list" :key="b.id" class="rounded-xl border border-gray-100 p-3">
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="text-sm font-semibold text-gray-800">{{ b.title }}</p>
                <p class="text-xs text-gray-500">{{ AUD[b.audience] || b.audience }} · dibuat {{ b.created_at }} oleh {{ b.created_by || '-' }}</p>
              </div>
              <span class="chip" :class="STATUS_CLS[b.status]">{{ STATUS_LBL[b.status] || b.status }}</span>
            </div>
            <div class="mt-2">
              <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-100">
                <div class="h-full bg-emerald-500" :style="{ width: pct(b) + '%' }" />
              </div>
              <p class="mt-1 text-xs text-gray-600">
                {{ b.sent }} terkirim · {{ b.pending }} menunggu · <span :class="b.failed ? 'text-red-600' : ''">{{ b.failed }} gagal</span> · dari {{ b.total }}
              </p>
            </div>
            <p class="mt-1 line-clamp-2 whitespace-pre-wrap text-xs text-gray-500">{{ b.body }}</p>
            <div class="mt-2 flex flex-wrap gap-3 text-xs">
              <RouterLink :to="{ path: '/whatsapp/messages', query: { broadcast_id: b.id } }" class="font-semibold text-emerald-700 hover:underline">Lihat pesan</RouterLink>
              <button v-if="b.status === 'queued' && b.pending > 0" class="font-semibold text-red-600 hover:underline" @click="cancel(b)">Batalkan sisa</button>
            </div>
          </div>
          <AppPagination v-if="total > limit" v-model="page" :total="total" :perPage="limit" @change="load" />
        </div>
      </AppCard>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import { whatsappApi } from '@/api/whatsapp.js'
import { outletsApi } from '@/api/outlets.js'
import { useToastStore } from '@/stores/toast.js'

const toast = useToastStore()
const AUD = { customers: 'Pelanggan', recipients: 'Penerima internal', manual: 'Daftar manual' }
const STATUS_LBL = { queued: 'Berjalan', done: 'Selesai', cancelled: 'Dibatalkan' }
const STATUS_CLS = { queued: 'chip--amber', done: 'chip--ok', cancelled: 'chip--off' }

const form = ref({ title: '', audience: 'customers', outlet_id: '', min_visits: 1, last_visit_days: 0, not_visited_days: 0, manual: '', body: '', image_url: '' })
const sendAtLocal = ref('')
const previewData = ref(null)
const busy = ref({ preview: false, send: false })
const errorMsg = ref('')
const outlets = ref([])
const list = ref([])
const page = ref(1)
const limit = 10
const total = ref(0)
const gap = ref({ min: 4, max: 9 })
let timer = null

function payload() {
  const sendAt = sendAtLocal.value ? sendAtLocal.value.replace('T', ' ').slice(0, 16) : ''
  return { ...form.value, send_at: sendAt }
}
function estimateMinutes(n) {
  const avg = (gap.value.min + gap.value.max) / 2 + 2
  return Math.max(1, Math.round((n * avg) / 60))
}
function pct(b) { return b.total ? Math.round(((b.sent + b.failed) / b.total) * 100) : 0 }

async function preview() {
  busy.value.preview = true
  previewData.value = null
  try { previewData.value = await whatsappApi.previewBroadcast(payload()) }
  catch (e) { toast.error(e?.message || 'Gagal menghitung penerima') } finally { busy.value.preview = false }
}

async function send() {
  if (!previewData.value) await preview()
  const n = previewData.value?.total || 0
  if (!n) { toast.error('Tidak ada penerima yang cocok'); return }
  if (!confirm(`Kirim broadcast "${form.value.title}" ke ${n} penerima?`)) return
  busy.value.send = true
  try {
    await whatsappApi.createBroadcast(payload())
    toast.success(`Broadcast diantrekan ke ${n} penerima`)
    form.value.title = ''; form.value.body = ''; form.value.image_url = ''; form.value.manual = ''
    previewData.value = null
    page.value = 1
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal membuat broadcast') } finally { busy.value.send = false }
}

async function cancel(b) {
  if (!confirm(`Batalkan ${b.pending} pesan yang belum terkirim dari "${b.title}"?`)) return
  try { await whatsappApi.cancelBroadcast(b.id); toast.success('Sisa broadcast dibatalkan'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal membatalkan') }
}

async function load() {
  try {
    const res = await whatsappApi.listBroadcasts({ page: page.value, limit })
    list.value = res.data || []
    total.value = res.total || 0
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat riwayat' }
}
watch(page, load)

onMounted(async () => {
  load()
  try {
    const res = await outletsApi.list()
    outlets.value = (Array.isArray(res) ? res : (res?.data || [])).map(o => ({ id: (o.id || '').trim(), name: o.name }))
  } catch { outlets.value = [] }
  try {
    const s = await whatsappApi.getSettings()
    gap.value = { min: s.settings.gap_min_sec, max: s.settings.gap_max_sec }
  } catch { /* pakai perkiraan bawaan */ }
  timer = setInterval(() => { if (list.value.some(b => b.status === 'queued')) load() }, 10000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.hint { margin-top: .25rem; font-size: .68rem; color: #6b7280; }
.sec-title { font-size: .95rem; font-weight: 700; color: #111827; }
.chip { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 600; }
.chip--ok { background: #ecfdf5; color: #047857; }
.chip--amber { background: #fffbeb; color: #b45309; }
.chip--off { background: #f3f4f6; color: #6b7280; }
code { background: rgba(0,0,0,.05); padding: .05rem .25rem; border-radius: .25rem; }
</style>
