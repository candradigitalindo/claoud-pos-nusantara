<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Log Pesan WhatsApp</h1>
        <p class="mt-0.5 text-sm text-gray-500">Semua pesan yang diantrekan: notifikasi, broadcast, dan pesan uji — beserta status pengirimannya.</p>
      </div>
      <AppButton v-if="canManage" size="sm" variant="secondary" :loading="busy.retryAll" @click="retryAll">Ulangi semua yang gagal</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppAlert v-if="broadcastId" type="info" :message="`Menampilkan pesan dari satu broadcast (${broadcastId}).`">
    </AppAlert>

    <AppCard>
      <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
        <input v-model="q" type="search" class="form-input" placeholder="Cari nomor, nama, isi pesan, event…" />
        <select v-model="statusF" class="form-input">
          <option value="">Semua status</option>
          <option value="pending">Menunggu</option>
          <option value="sent">Terkirim</option>
          <option value="failed">Gagal</option>
          <option value="cancelled">Dibatalkan</option>
        </select>
        <select v-model="kindF" class="form-input">
          <option value="">Semua jenis</option>
          <option value="notify">Notifikasi</option>
          <option value="broadcast">Broadcast</option>
          <option value="test">Uji</option>
        </select>
      </div>
    </AppCard>

    <AppCard :padding="false">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
            <tr>
              <th class="px-3 py-2">Waktu</th>
              <th class="px-3 py-2">Tujuan</th>
              <th class="px-3 py-2">Jenis</th>
              <th class="px-3 py-2">Pesan</th>
              <th class="px-3 py-2">Status</th>
              <th class="px-3 py-2 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-if="loading && !rows.length"><td colspan="6" class="px-3 py-8 text-center text-gray-400">Memuat…</td></tr>
            <tr v-else-if="!rows.length"><td colspan="6" class="px-3 py-8 text-center text-gray-400">Belum ada pesan.</td></tr>
            <tr v-for="m in rows" :key="m.id" class="align-top">
              <td class="px-3 py-2 whitespace-nowrap text-xs text-gray-600">
                <p>{{ m.created_at }}</p>
                <p v-if="m.sent_at" class="text-emerald-700">kirim {{ m.sent_at }}</p>
                <p v-else-if="m.status === 'pending' && m.scheduled_at !== m.created_at" class="text-amber-600">jadwal {{ m.scheduled_at }}</p>
              </td>
              <td class="px-3 py-2 text-xs">
                <p class="font-medium text-gray-800">{{ m.target_name || '—' }}</p>
                <p class="font-mono text-gray-500">{{ m.target.includes('@') ? 'grup ' + m.target.replace('@g.us', '') : '+' + m.target }}</p>
              </td>
              <td class="px-3 py-2 text-xs text-gray-600">
                <span class="chip" :class="KIND_CLS[m.kind]">{{ KIND_LBL[m.kind] || m.kind }}</span>
                <p v-if="m.event" class="mt-0.5 font-mono text-[10px] text-gray-400">{{ m.event }}</p>
                <p class="text-[10px] text-gray-400">{{ m.audience === 'customer' ? 'pelanggan' : 'internal' }}</p>
              </td>
              <td class="px-3 py-2 text-xs text-gray-700">
                <button class="text-left" @click="toggle(m.id)">
                  <span class="whitespace-pre-wrap" :class="expanded[m.id] ? '' : 'line-clamp-2'">{{ m.body }}</span>
                </button>
                <a v-if="m.image_url" :href="m.image_url" target="_blank" class="mt-0.5 block text-emerald-700 hover:underline">gambar ↗</a>
              </td>
              <td class="px-3 py-2 text-xs">
                <span class="chip" :class="STATUS_CLS[m.status]">{{ STATUS_LBL[m.status] || m.status }}</span>
                <p v-if="m.last_error" class="mt-0.5 max-w-[16rem] text-[11px] text-red-600">{{ m.last_error }}</p>
                <p v-if="m.attempts > 1" class="text-[10px] text-gray-400">{{ m.attempts }}× percobaan</p>
              </td>
              <td class="px-3 py-2 text-right whitespace-nowrap text-xs">
                <template v-if="canManage">
                  <button v-if="m.status === 'failed' || m.status === 'cancelled'" class="font-semibold text-emerald-700 hover:underline" @click="retry(m)">Ulangi</button>
                  <button v-if="m.status === 'pending'" class="font-semibold text-red-600 hover:underline" @click="cancel(m)">Batalkan</button>
                </template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="border-t border-gray-100 p-3">
        <AppPagination v-model="page" :total="total" :perPage="limit" @change="load" />
      </div>
    </AppCard>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import { whatsappApi } from '@/api/whatsapp.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'

const route = useRoute()
const auth = useAuthStore()
const toast = useToastStore()
// Cek eksplisit: aturan lama hasPermission menganggap '.manage' terpenuhi oleh
// izin modul apa pun (mis. whatsapp.broadcast), padahal server menuntut kuncinya persis.
const canManage = computed(() => auth.isSuperadmin || (auth.permissions || []).includes('whatsapp.manage'))

const STATUS_LBL = { pending: 'Menunggu', sent: 'Terkirim', failed: 'Gagal', cancelled: 'Dibatalkan' }
const STATUS_CLS = { pending: 'chip--amber', sent: 'chip--ok', failed: 'chip--red', cancelled: 'chip--off' }
const KIND_LBL = { notify: 'Notifikasi', broadcast: 'Broadcast', test: 'Uji' }
const KIND_CLS = { notify: 'chip--int', broadcast: 'chip--purple', test: 'chip--off' }

const rows = ref([])
const loading = ref(false)
const errorMsg = ref('')
const q = ref('')
const statusF = ref('')
const kindF = ref('')
const broadcastId = ref(route.query.broadcast_id || '')
const page = ref(1)
const limit = 25
const total = ref(0)
const expanded = ref({})
const busy = ref({ retryAll: false })
let timer = null
let searchTimer = null

async function load() {
  loading.value = true
  try {
    const res = await whatsappApi.listMessages({
      page: page.value, limit, q: q.value, status: statusF.value, kind: kindF.value, broadcast_id: broadcastId.value,
    })
    rows.value = res.data || []
    total.value = res.total || 0
    errorMsg.value = ''
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat log' } finally { loading.value = false }
}
watch([statusF, kindF], () => { page.value = 1; load() })
watch(q, () => { clearTimeout(searchTimer); searchTimer = setTimeout(() => { page.value = 1; load() }, 350) })
watch(page, load)
watch(() => route.query.broadcast_id, (v) => { broadcastId.value = v || ''; page.value = 1; load() })

function toggle(id) { expanded.value[id] = !expanded.value[id] }
async function retry(m) {
  try { await whatsappApi.retryMessage(m.id); toast.success('Pesan diantrekan lagi'); load() }
  catch (e) { toast.error(e?.message || 'Gagal') }
}
async function cancel(m) {
  try { await whatsappApi.cancelMessage(m.id); toast.success('Pesan dibatalkan'); load() }
  catch (e) { toast.error(e?.message || 'Gagal') }
}
async function retryAll() {
  busy.value.retryAll = true
  try { const r = await whatsappApi.retryFailed(); toast.success(`${r.requeued} pesan diantrekan lagi`); load() }
  catch (e) { toast.error(e?.message || 'Gagal') } finally { busy.value.retryAll = false }
}

onMounted(() => {
  load()
  timer = setInterval(() => { if (rows.value.some(m => m.status === 'pending')) load() }, 15000)
})
onUnmounted(() => { clearInterval(timer); clearTimeout(searchTimer) })
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.chip { display: inline-block; padding: .05rem .45rem; border-radius: 999px; font-size: .65rem; font-weight: 600; white-space: nowrap; }
.chip--ok { background: #ecfdf5; color: #047857; }
.chip--amber { background: #fffbeb; color: #b45309; }
.chip--red { background: #fef2f2; color: #b91c1c; }
.chip--off { background: #f3f4f6; color: #6b7280; }
.chip--int { background: #ecfdf5; color: #047857; }
.chip--purple { background: #f5f3ff; color: #6d28d9; }
</style>
