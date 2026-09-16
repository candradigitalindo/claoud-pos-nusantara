<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="min-w-0">
      <h1 class="text-lg sm:text-xl font-bold text-gray-900">Cadangan Bukti Foto</h1>
      <p class="mt-0.5 text-sm text-gray-500">
        Foto serah terima disalin ke Google Drive lewat rclone — remote yang sama dengan
        cadangan database harian.
      </p>
    </div>

    <AppAlert type="error" :message="errorMsg" />
    <AppAlert v-if="!status.configured && status.reason" type="warning" :message="status.reason" />

    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard class="min-w-0">
        <p class="stat-lbl">Status</p>
        <p class="stat-val" :class="status.configured ? 'text-emerald-700' : 'text-amber-600'">
          {{ status.configured ? 'Aktif' : 'Belum aktif' }}
        </p>
        <p class="stat-sub">{{ status.remote }}</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Menunggu</p>
        <p class="stat-val" :class="status.pending ? 'text-amber-600' : ''">{{ status.pending }}</p>
        <p class="stat-sub">disalin tiap 15 menit</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Tersalin</p>
        <p class="stat-val">{{ status.sent }}</p>
        <p class="stat-sub">{{ status.last_sent_at || 'belum ada' }}</p>
      </AppCard>
      <AppCard class="min-w-0">
        <p class="stat-lbl">Gagal</p>
        <p class="stat-val" :class="status.failed ? 'text-red-600' : ''">{{ status.failed }}</p>
        <p class="stat-sub">
          <button v-if="status.failed" class="text-emerald-700 underline" :disabled="retrying" @click="retry">
            {{ retrying ? 'mengulang…' : 'coba lagi' }}
          </button>
          <span v-else>tidak ada</span>
        </p>
      </AppCard>
    </div>

    <AppAlert v-if="status.failed && status.last_error" type="warning"
      :message="`Kesalahan terakhir: ${status.last_error}`" />

    <AppCard>
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Folder Google Drive</label>
          <input v-model="form.photo_drive_remote" class="form-input" placeholder="gdrive:cloud-pos-photos" />
          <p class="mt-1 text-[11px] text-gray-500">
            Format rclone: <code>namaremote:folder</code>. Kosongkan untuk memakai
            <code>gdrive:cloud-pos-photos</code>.
          </p>
        </div>

        <div class="rounded-lg bg-gray-50 p-3 text-xs text-gray-600">
          <p class="font-semibold text-gray-800">Kredensial tidak disimpan di aplikasi.</p>
          <p class="mt-1">
            Akses Drive memakai konfigurasi rclone milik server (<code>rclone.conf</code>),
            yang dipasang read-only ke dalam container — remote yang sama dengan cron cadangan
            database harian. Mengganti akun Drive dilakukan di server dengan
            <code>rclone config</code>, bukan dari layar ini.
          </p>
        </div>

        <div class="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">
          <p class="font-semibold">Foto dijadikan publik.</p>
          <p class="mt-1">
            Setiap foto yang tersalin diberi izin “siapa saja yang punya link” agar bisa
            ditampilkan langsung di layar tanpa login. Konsekuensinya: siapa pun yang memegang
            tautannya bisa membukanya, termasuk di luar perusahaan.
          </p>
        </div>

        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <AppButton type="submit" :loading="saving">Simpan</AppButton>
        </div>
      </form>
    </AppCard>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { photoBackupApi } from '@/api/handover.js'
import { useToastStore } from '@/stores/toast.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'

const toast = useToastStore()
const status = ref({ configured: false, remote: '', reason: '', pending: 0, sent: 0, failed: 0, last_error: '', last_sent_at: '' })
const form = ref({ photo_drive_remote: '' })
const saving = ref(false)
const retrying = ref(false)
const errorMsg = ref('')

function asObject(d) { return d?.data ?? d ?? null }

async function load() {
  errorMsg.value = ''
  try {
    const s = asObject(await photoBackupApi.getSettings())
    form.value.photo_drive_remote = s?.photo_drive_remote || ''
  } catch (e) { errorMsg.value = e?.message || 'Gagal memuat setelan' }
  try { status.value = asObject(await photoBackupApi.status()) || status.value } catch { /* kartu tetap 0 */ }
}

async function save() {
  saving.value = true
  try {
    await photoBackupApi.updateSettings(form.value)
    toast.success('Lokasi cadangan disimpan')
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

async function retry() {
  retrying.value = true
  try {
    await photoBackupApi.retry()
    toast.success('Penyalinan diulang')
    setTimeout(load, 4000)
  } catch (e) { toast.error(e?.message || 'Gagal mengulang') } finally { retrying.value = false }
}

onMounted(load)
</script>

<style scoped>
.form-input {
  width: 100%; padding: .5rem .7rem; border-radius: .6rem; font-size: .85rem;
  border: 1px solid rgba(0,0,0,.14); background: #fff; color: #111827; outline: none;
}
.form-input:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.lbl { display: block; font-size: .72rem; font-weight: 700; color: #4b5563; margin-bottom: .25rem; }
.stat-lbl { font-size: .68rem; font-weight: 700; color: #9ca3af; text-transform: uppercase; letter-spacing: .02em; }
.stat-val { margin-top: .15rem; font-size: 1rem; font-weight: 700; color: #111827; }
.stat-sub { margin-top: .1rem; font-size: .7rem; color: #6b7280; overflow-wrap: anywhere; }
code { background: rgba(0,0,0,.05); padding: .05rem .25rem; border-radius: .25rem; font-size: .95em; }
</style>
