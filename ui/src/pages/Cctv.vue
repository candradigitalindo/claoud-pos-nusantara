<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-bold text-gray-900">CCTV</h1>
        <p class="mt-1 text-sm text-gray-500">Satukan kamera berbagai merek, dikelompokkan menurut outlet. Rekaman tetap di perangkat masing-masing.</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <AppButton v-if="can('cameras.create') || can('cameras.update') || can('cameras.delete')" variant="secondary" @click="openGateway">Koneksi outlet</AppButton>
        <AppButton variant="secondary" :loading="loading" @click="load">Muat ulang</AppButton>
        <AppButton v-if="can('cameras.create')" @click="editCamera()">Tambah kamera</AppButton>
      </div>
    </div>

    <AppAlert type="error" :message="error" />
    <AppAlert v-if="!gatewayOpen" type="error" :message="gatewayError" />
    <section v-if="monitored.length" ref="monitorSection" class="scroll-mt-4 space-y-3 rounded-xl border border-emerald-200 bg-emerald-50/40 p-4" aria-label="Monitor langsung">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="font-semibold text-gray-900">Monitor langsung ({{ monitored.length }}/4)</h2>
        <AppButton size="sm" variant="secondary" @click="monitorIds = []">Tutup semua monitor</AppButton>
      </div>
      <div class="grid gap-4 lg:grid-cols-2">
        <article v-for="camera in monitored" :key="`${camera.id}:${camera.updated_at || ''}`" class="min-w-0 rounded-lg bg-white p-3">
          <div class="mb-2 flex items-start justify-between gap-2">
            <div class="min-w-0"><h3 class="truncate font-semibold text-gray-900">{{ camera.name }}</h3><p class="truncate text-xs text-gray-500">{{ camera.outlet_name }}</p></div>
            <button class="text-sm text-gray-600 underline" :aria-label="`Tutup monitor ${camera.name}`" @click="toggleMonitor(camera)">Tutup</button>
          </div>
          <CctvVideo :camera="camera" />
        </article>
      </div>
      <p class="text-xs text-gray-500">Maksimal empat kamera bersamaan. Gunakan sub-stream untuk menghemat koneksi outlet.</p>
    </section>
    <div class="rounded-xl border border-gray-200 bg-white p-4">
      <label class="mb-2 block text-sm font-medium text-gray-700">Outlet</label>
      <SearchSelect v-model="outletFilter" :options="outletOptions" value-key="id" placeholder="Semua outlet" />
      <input v-model.trim="search" aria-label="Cari kamera" type="search" class="form-input mt-3" placeholder="Cari nama kamera, outlet, atau merek" />
      <p class="mt-3 text-xs text-gray-500">{{ filtered.length }} kamera · {{ filtered.filter(c => c.is_active).length }} aktif</p>
    </div>

    <div v-if="loading && !cameras.length" class="py-12 text-center text-sm text-gray-500">Memuat kamera…</div>
    <div v-else-if="!filtered.length" class="rounded-xl border border-dashed border-gray-300 bg-white px-5 py-14 text-center">
      <h2 class="font-semibold text-gray-800">{{ search ? 'Kamera tidak ditemukan' : `Belum ada kamera${outletFilter ? ' di outlet ini' : ''}` }}</h2>
      <p class="mt-2 text-sm text-gray-500">{{ search ? 'Coba kata pencarian lain atau pilih semua outlet.' : can('cameras.create') ? 'Tambahkan kamera, lalu pasang penghubung melalui Koneksi outlet untuk mengakses kamera lokal tanpa VPN.' : 'Hubungi admin untuk menambahkan kamera outlet.' }}</p>
    </div>
    <div v-else class="space-y-6">
      <section v-for="group in groups" :key="group.id" class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="font-semibold text-gray-900">{{ group.name }} <span class="ml-1 text-xs font-normal text-gray-500">{{ group.cameras.length }} kamera</span></h2>
          <div class="flex flex-wrap items-center gap-3"><span v-if="group.cameras.some(c => c.access_mode === 'gateway')" class="text-xs" :class="gatewayFor(group.id)?.online ? 'text-emerald-700' : 'text-amber-700'">Penghubung {{ gatewayFor(group.id)?.online ? 'online' : 'belum tersambung' }}</span><AppButton size="sm" variant="secondary" :disabled="!group.cameras.some(c => c.is_active)" @click="monitorOutlet(group)">Pantau outlet</AppButton></div>
        </div>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <article v-for="camera in group.cameras" :key="camera.id" class="rounded-xl border border-gray-200 bg-white p-5">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h2 class="truncate font-semibold text-gray-900">{{ camera.name }}</h2>
            <p class="mt-1 text-sm text-gray-500">{{ camera.outlet_name }}</p>
          </div>
          <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="camera.is_active ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'">{{ camera.is_active ? 'Aktif' : 'Nonaktif' }}</span>
        </div>
        <p class="mt-4 text-xs text-gray-500">{{ brandName(camera.brand) }} · {{ connectionName(camera.connection_type) }}<template v-if="camera.connection_type === 'onvif'"> · Profil perangkat</template><template v-else-if="!connectionUsesURL(camera.connection_type)"> · Channel {{ camera.channel }} · {{ camera.subtype === 0 ? 'Kualitas utama' : 'Hemat bandwidth' }}</template></p>
        <div class="mt-4 flex flex-wrap gap-2">
          <AppButton size="sm" :disabled="!camera.is_active" @click="startLive(camera)">Tonton live</AppButton>
          <AppButton size="sm" variant="secondary" :disabled="!camera.is_active || camera.supports_playback === false" :title="camera.supports_playback === false ? 'Tambahkan koneksi rekaman yang didukung perangkat' : 'Putar rekaman perangkat'" @click="choosePlayback(camera)">Putar ulang</AppButton>
          <AppButton size="sm" variant="ghost" :disabled="!camera.is_active || (!monitorIds.includes(camera.id) && monitored.length >= 4)" @click="toggleMonitor(camera)">{{ monitorIds.includes(camera.id) ? 'Lepas monitor' : 'Tambah ke monitor' }}</AppButton>
        </div>
        <div class="mt-3 flex items-center gap-2 text-xs">
          <button class="text-emerald-700 disabled:opacity-50" :disabled="!camera.is_active || checking === camera.id" @click="testConnection(camera)">{{ checking === camera.id ? 'Memeriksa…' : 'Tes koneksi' }}</button>
          <span v-if="checks[camera.id]" :class="checks[camera.id].ok ? 'text-emerald-700' : 'text-red-600'">{{ checks[camera.id].message }}</span>
        </div>
        <div v-if="can('cameras.update') || can('cameras.delete')" class="mt-4 flex flex-wrap gap-3 border-t border-gray-100 pt-3 text-xs">
          <button v-if="can('cameras.update')" class="font-medium text-emerald-700" @click="editCamera(camera)">Ubah</button>
          <button v-if="can('cameras.update')" :disabled="busy === camera.id" class="text-gray-600 disabled:opacity-50" @click="toggleCamera(camera)">{{ camera.is_active ? 'Nonaktifkan' : 'Aktifkan' }}</button>
          <button v-if="can('cameras.delete')" :disabled="busy === camera.id" class="text-red-600 disabled:opacity-50" @click="deleteCamera(camera)">Hapus</button>
        </div>
      </article>
        </div>
      </section>
    </div>

    <AppModal v-model="formOpen" :title="editing ? 'Ubah kamera' : 'Tambah kamera'" size="lg">
      <form id="camera-form" class="grid grid-cols-1 gap-4 sm:grid-cols-2" @submit.prevent="saveCamera">
        <AppAlert class="sm:col-span-2" type="error" :message="formError" />
        <AppAlert v-if="probeResult" class="sm:col-span-2" :type="probeResult.ok ? 'success' : 'error'" :message="probeResult.message" />
        <label class="field sm:col-span-2">Outlet
          <select aria-label="Outlet" v-model="form.outlet_id" class="form-input" required :disabled="!!editing">
            <option disabled value="">Pilih outlet</option>
            <option v-for="outlet in outlets" :key="outlet.id" :value="outlet.id">{{ outlet.name }}</option>
          </select>
        </label>
        <label class="field">Nama kamera<input v-model.trim="form.name" class="form-input" required maxlength="100" placeholder="Kasir depan" /></label>
        <label class="field">Merek<select aria-label="Merek" v-model="form.brand" class="form-input" @change="changeBrand"><option v-for="brand in options.brands" :key="brand" :value="brand.toLowerCase()">{{ brand }}</option></select></label>
        <label class="field sm:col-span-2">Jenis koneksi<select aria-label="Jenis koneksi" v-model="form.connection_type" class="form-input" @change="changeConnection"><option v-for="connection in options.connections" :key="connection.id" :value="connection.id">{{ connection.name }}</option></select><span class="text-xs font-normal text-gray-500">{{ selectedConnection?.help }}</span></label>
        <label v-if="form.connection_type !== 'tuya'" class="field sm:col-span-2">Jalur ke outlet<select aria-label="Jalur ke outlet" v-model="form.access_mode" class="form-input"><option value="gateway">Penghubung outlet — tanpa VPN</option><option value="direct">Langsung — alamat dapat dijangkau server</option></select><span class="text-xs font-normal text-gray-500">{{ form.access_mode === 'gateway' ? 'Satu penghubung berjalan di jaringan outlet dan mengirim tayangan melalui HTTPS.' : 'Untuk alamat perangkat atau layanan vendor yang dapat dijangkau server.' }}</span></label>
        <AppAlert v-if="form.access_mode === 'gateway' && !gatewayFor(form.outlet_id)?.online" class="sm:col-span-2" type="info" message="Konfigurasi kamera dapat disimpan sekarang. Hubungkan outlet melalui tombol Koneksi outlet sebelum menonton." />
        <p v-if="brandHelp" class="text-xs text-amber-700 sm:col-span-2">{{ brandHelp }}</p>
        <p v-if="['v380','yoosee','hk'].includes(form.brand)" class="text-xs text-amber-700 sm:col-span-2">Dukungan tergantung model. Gunakan RTSP/ONVIF bila tersedia; UID aplikasi bawaan belum tentu dapat digunakan pada sistem lain.</p>
        <div v-if="form.connection_type === 'tuya'" class="sm:col-span-2 space-y-3 rounded-lg border border-gray-200 p-3">
          <p class="text-sm font-medium">Hubungkan Tuya Smart tanpa VPN</p>
          <p class="text-xs text-gray-500">Gunakan akun Tuya Smart. Akun Smart Life tidak didukung konektor ini. Dukungan kamera tetap mengikuti layanan vendor.</p>
          <label class="field">Region<select aria-label="Region Tuya" v-model="cloud.region" class="form-input"><option v-for="region in options.tuya_regions" :key="region.id" :value="region.id">{{ region.name }}</option></select></label>
          <label class="field">Email / nomor akun<input v-model="cloud.email" class="form-input" autocomplete="off" /></label>
          <label class="field">Password Tuya Smart<input v-model="cloud.password" type="password" class="form-input" autocomplete="new-password" /></label>
          <AppButton size="sm" variant="secondary" :loading="discovering" :disabled="!form.outlet_id" @click="discoverCloud">Cari kamera akun</AppButton>
          <AppAlert type="error" :message="cloudError" />
          <label v-if="cloudDevices.length" class="field">Kamera cloud<select aria-label="Kamera cloud" v-model="form.source_ticket" class="form-input" @change="selectCloudDevice"><option value="">Pilih kamera</option><option v-for="device in cloudDevices" :key="device.id" :value="device.ticket">{{ device.name }}</option></select></label>
        </div>
        <label v-if="selectedConnection?.needs_url || form.connection_type === 'onvif'" class="field sm:col-span-2">{{ form.connection_type === 'onvif' ? 'URL ONVIF khusus (opsional)' : 'URL sumber' }}<input aria-label="URL sumber" v-model.trim="form.source_url" type="password" class="form-input" autocomplete="new-password" :disabled="clearSecrets.source" :required="selectedConnection?.needs_url && !form.source_ticket && !(editing && savedSecrets.source && editedConnection === form.connection_type)" :placeholder="editing ? 'Kosong = pertahankan sumber lama' : form.connection_type === 'rtsp' ? 'rtsp://user:password@alamat/path' : 'URL konektor sesuai dokumentasi perangkat'" /><span v-if="form.brand === 'ezviz' && form.connection_type === 'rtsp'" class="break-all text-xs font-normal text-gray-500">Contoh C6N/TY1/TY2: rtsp://admin:PASSWORD_LOKAL@IP_KAMERA:554/ch1/main. Gunakan password koneksi lokal; kode verifikasi pada label berlaku jika belum diganti. Model lain dapat memakai URL berbeda.</span><span v-if="form.source_ticket" class="text-xs text-emerald-700">{{ form.connection_type === 'onvif' ? 'Profil ONVIF telah dipilih.' : 'Kamera cloud telah dipilih.' }}</span></label>
        <label v-if="editing && savedSecrets.source && !selectedConnection?.needs_url" class="flex items-center gap-2 text-sm text-gray-600 sm:col-span-2"><input v-model="clearSecrets.source" type="checkbox" /> Hapus URL sumber khusus tersimpan dan gunakan alamat perangkat</label>
        <label v-if="!selectedConnection?.needs_url" class="field sm:col-span-2">Alamat perangkat<input aria-label="Alamat perangkat" v-model.trim="form.host" class="form-input" :required="!form.source_ticket && !form.source_url && !(editing && savedSecrets.source && !clearSecrets.source && editedConnection === form.connection_type)" maxlength="120" placeholder="IP atau hostname kamera/DVR" /><span class="text-xs font-normal text-gray-500">Untuk penghubung outlet, isi IP lokal kamera di outlet tersebut.</span></label>
        <label v-if="!selectedConnection?.needs_url" class="field">Port<input v-model.number="form.port" class="form-input" type="number" min="1" max="65535" required /></label>
        <label v-if="['hikvision','dahua','dvrip','reolink'].includes(form.connection_type)" class="field">Channel<input v-model.number="form.channel" class="form-input" type="number" min="1" max="256" required /></label>
        <label v-if="!selectedConnection?.needs_url" class="field">Username perangkat<input v-model.trim="form.rtsp_username" class="form-input" maxlength="80" autocomplete="off" /></label>
        <label v-if="!selectedConnection?.needs_url" class="field">Password perangkat<input v-model="form.rtsp_password" class="form-input" type="password" autocomplete="new-password" :disabled="clearSecrets.password" :placeholder="editing ? 'Kosong = pertahankan password lama' : 'Password DVR'" /></label>
        <label v-if="editing && savedSecrets.password && !selectedConnection?.needs_url" class="flex items-center gap-2 text-sm text-gray-600 sm:col-span-2"><input v-model="clearSecrets.password" type="checkbox" /> Hapus password perangkat tersimpan</label>
        <div v-if="form.connection_type === 'onvif'" class="space-y-3 rounded-lg border border-gray-200 p-3 sm:col-span-2">
          <p class="text-xs text-gray-600">Cari profil pada alamat perangkat untuk memilih kamera/channel dan kualitas yang benar. Jika hanya ada satu profil, sistem dapat memilihnya otomatis.</p>
          <AppButton size="sm" variant="secondary" :loading="findingProfiles" :disabled="!form.outlet_id || saving || probing" @click="findONVIFProfiles">Cari profil ONVIF</AppButton>
          <AppAlert type="error" :message="profileError" />
          <label v-if="onvifProfiles.length" class="field">Profil ONVIF<select aria-label="Profil ONVIF" v-model="form.source_ticket" class="form-input" :required="!savedSecrets.source || clearSecrets.source" @change="clearSecrets.source = false"><option value="">Pilih profil kamera/channel</option><option v-for="profile in onvifProfiles" :key="profile.id" :value="profile.ticket">{{ profile.name }}</option></select></label>
        </div>
        <label v-if="!selectedConnection?.needs_url && form.connection_type !== 'onvif'" class="field">Kualitas<select v-model.number="form.subtype" class="form-input"><option :value="1">Hemat bandwidth (sub-stream)</option><option :value="0">Kualitas utama (main-stream)</option></select></label>
        <p v-else class="text-xs text-gray-500 sm:col-span-2">Channel dan kualitas mengikuti profil, URL sumber, atau kamera cloud yang dipilih.</p>
        <label class="field">Urutan<input v-model.number="form.sort_order" class="form-input" type="number" min="0" max="100000" /></label>
        <label class="field">Zona waktu kamera<select v-model="form.timezone" class="form-input"><option value="Asia/Jakarta">WIB — Jakarta</option><option value="Asia/Makassar">WITA — Makassar</option><option value="Asia/Jayapura">WIT — Jayapura</option><option value="UTC">UTC</option><option v-if="!['Asia/Jakarta','Asia/Makassar','Asia/Jayapura','UTC'].includes(form.timezone)" :value="form.timezone">{{ form.timezone }}</option></select></label>
        <label class="field sm:col-span-2">URL rekaman RTSP (opsional)<input v-model.trim="form.playback_url" type="password" class="form-input" autocomplete="new-password" :disabled="clearSecrets.playback" :placeholder="editing ? 'Kosong = pertahankan URL rekaman lama' : 'URL playback dari dokumentasi perangkat'" /><span class="text-xs font-normal text-gray-500">Hikvision/Dahua memakai pola bawaan. Merek lain memerlukan URL rekaman; placeholder waktu: {start}, {end}, {start_iso}, {end_iso}, {start_unix}, {end_unix}.</span></label>
        <label v-if="editing && savedSecrets.playback" class="flex items-center gap-2 text-sm text-gray-600 sm:col-span-2"><input v-model="clearSecrets.playback" type="checkbox" /> Hapus URL rekaman tersimpan</label>
        <label class="flex items-center gap-2 text-sm text-gray-700"><input v-model="form.is_active" type="checkbox" class="accent-emerald-600" /> Kamera aktif</label>
      </form>
      <template #footer>
        <AppButton variant="secondary" @click="formOpen = false">Batal</AppButton>
        <AppButton variant="secondary" :loading="probing" :disabled="saving || !form.outlet_id" @click="probeConfiguration">Tes sebelum simpan</AppButton>
        <AppButton type="submit" form="camera-form" :loading="saving" :disabled="probing">Simpan</AppButton>
      </template>
    </AppModal>

    <AppModal v-model="gatewayOpen" title="Hubungkan outlet tanpa VPN" size="lg">
      <div class="space-y-4">
        <p class="text-sm text-gray-600">Pasang satu penghubung pada komputer yang selalu menyala di outlet. Penghubung mengakses kamera lokal dan membuat koneksi keluar ke Cloud POS melalui HTTPS.</p>
        <label class="field">Outlet penghubung<select aria-label="Outlet penghubung" v-model="gatewayOutlet" class="form-input" :disabled="creatingGateway || revokingGateway" @change="gatewayToken = ''"><option value="">Pilih outlet</option><option v-for="outlet in outlets" :key="outlet.id" :value="outlet.id">{{ outlet.name }}</option></select></label>
        <p v-if="gatewayFor(gatewayOutlet)" class="text-sm" :class="gatewayFor(gatewayOutlet).online ? 'text-emerald-700' : 'text-amber-700'">Status: {{ gatewayFor(gatewayOutlet).online ? 'Online' : 'Belum tersambung' }}</p>
        <p v-if="gatewayFor(gatewayOutlet)?.last_seen" class="text-xs text-gray-500">Terakhir terhubung: {{ formatSeen(gatewayFor(gatewayOutlet).last_seen) }}</p>
        <AppAlert type="error" :message="gatewayError" />
        <AppButton :loading="creatingGateway" :disabled="revokingGateway || !gatewayOutlet || (gatewayFor(gatewayOutlet) ? !can('cameras.update') : !can('cameras.create'))" @click="createGateway">{{ gatewayFor(gatewayOutlet) ? 'Ganti token penghubung' : 'Buat token penghubung' }}</AppButton>
        <AppButton v-if="gatewayFor(gatewayOutlet) && can('cameras.delete')" variant="danger" :loading="revokingGateway" :disabled="creatingGateway" @click="revokeGateway">Cabut akses penghubung</AppButton>
        <template v-if="gatewayToken">
          <p class="text-xs text-gray-500">Token hanya ditampilkan saat dibuat. Penggantian token memutus penghubung yang memakai token lama.</p>
          <label class="field">Alamat cloud HTTPS<input v-model.trim="gatewayCloudURL" class="form-input" type="url" placeholder="https://pos.example.com" /></label>
          <AppButton variant="secondary" size="sm" @click="downloadGatewayConfig">Unduh .env penghubung</AppButton>
          <p class="text-sm text-gray-600">Pada komputer outlet yang memiliki Docker dan salinan proyek ini, simpan berkas unduhan sebagai <code>cctv-gateway.env</code>, lalu jalankan:</p>
          <pre class="overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs text-white">docker compose --env-file cctv-gateway.env -f docker-compose.cctv-gateway.yml up -d --build</pre>
        </template>
      </div>
      <template #footer><AppButton variant="secondary" @click="refreshGateways">Periksa status</AppButton><AppButton @click="gatewayOpen = false">Selesai</AppButton></template>
    </AppModal>

    <AppModal v-model="playbackOpen" title="Putar ulang rekaman DVR">
      <form id="playback-form" class="space-y-4" @submit.prevent="startPlayback">
        <p class="text-sm text-gray-600">{{ selectedCamera?.name }} · {{ selectedCamera?.outlet_name }}</p>
        <p class="text-xs text-gray-500">Waktu mengikuti zona perangkat: {{ selectedCamera?.timezone || 'Asia/Jakarta' }}. Pastikan jam DVR sudah benar.</p>
        <AppAlert type="error" :message="playbackError" />
        <label class="field">Mulai<input v-model="playback.start" type="datetime-local" step="1" class="form-input" required /></label>
        <label class="field">Selesai<input v-model="playback.end" type="datetime-local" step="1" class="form-input" required /></label>
        <p class="text-xs text-gray-500">Pilih waktu sesuai jam lokal DVR. Rekaman harus tersedia pada DVR outlet.</p>
      </form>
      <template #footer>
        <AppButton variant="secondary" @click="playbackOpen = false">Batal</AppButton>
        <AppButton type="submit" form="playback-form">Putar rekaman</AppButton>
      </template>
    </AppModal>

    <AppModal v-model="playerOpen" :title="`${playerMode === 'playback' ? 'Putar ulang' : 'Live'} — ${selectedCamera?.name || 'CCTV'}`" size="2xl">
      <CctvVideo v-if="playerOpen && selectedCamera" :key="`${selectedCamera.id}:${playerMode}`" :camera="selectedCamera" :mode="playerMode" :range="playback" />
      <p class="mt-3 text-xs text-gray-500">{{ selectedCamera?.outlet_name }} · Rekaman tetap tersimpan di DVR outlet.</p>
      <template #footer><AppButton variant="secondary" @click="playerOpen = false">Tutup tayangan</AppButton></template>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import CctvVideo from '@/components/cctv/CctvVideo.vue'
import { useSettingsStore } from '@/stores/settings.js'
import { camerasApi } from '@/api/cameras.js'
import { outletsApi } from '@/api/outlets.js'
import { useAuthStore } from '@/stores/auth.js'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const auth = useAuthStore()
const settings = useSettingsStore()
const can = (permission) => auth.isSuperadmin || auth.hasPermission(permission)
const cameras = ref([]), outlets = ref([]), outletFilter = ref(''), search = ref('')
const monitorIds = ref([])
const monitorSection = ref(null)
const monitored = computed(() => monitorIds.value.map(id => cameras.value.find(c => c.id === id && c.is_active)).filter(Boolean))
const outletOptions = computed(() => [{ id: '', name: 'Semua outlet' }, ...outlets.value])
const groups = computed(() => {
  const grouped = new Map()
  for (const camera of filtered.value) {
    if (!grouped.has(camera.outlet_id)) grouped.set(camera.outlet_id, { id: camera.outlet_id, name: camera.outlet_name || 'Outlet', cameras: [] })
    grouped.get(camera.outlet_id).cameras.push(camera)
  }
  return [...grouped.values()]
})
const options = reactive({ brands: ['Hikvision', 'Dahua'], connections: [{ id: 'hikvision', name: 'Hikvision', port: 554 }, { id: 'dahua', name: 'Dahua', port: 554 }], tuya_regions: [] })
const gateways = ref([]), gatewayOpen = ref(false), gatewayOutlet = ref(''), gatewayToken = ref(''), gatewayError = ref(''), creatingGateway = ref(false)
const checking = ref(''), checks = reactive({})
const gatewayCloudURL = ref(window.location.origin), revokingGateway = ref(false)
const savedSecrets = reactive({ password: false, source: false, playback: false })
const clearSecrets = reactive({ password: false, source: false, playback: false })
const probing = ref(false), probeResult = ref(null)
const findingProfiles = ref(false), onvifProfiles = ref([]), profileError = ref('')
const cloud = reactive({ region: 'protect-eu.ismartlife.me', email: '', password: '' })
const cloudDevices = ref([]), discovering = ref(false), cloudError = ref('')
const gatewayFor = id => gateways.value.find(g => g.outlet_id === id)
const brandName = value => options.brands.find(b => b.toLowerCase() === value?.toLowerCase()) || value || 'Lainnya'
const connectionName = value => options.connections.find(c => c.id === value)?.name || 'RTSP'
const connectionUsesURL = value => options.connections.find(c => c.id === value)?.needs_url
const loading = ref(false), error = ref(''), busy = ref('')
const filtered = computed(() => cameras.value.filter(c => (!outletFilter.value || c.outlet_id === outletFilter.value) && (!search.value || `${c.name} ${c.outlet_name || ''} ${c.brand}`.toLowerCase().includes(search.value.toLowerCase()))))
const formOpen = ref(false), editing = ref(null), editedConnection = ref(''), saving = ref(false), formError = ref('')
const defaults = () => ({ outlet_id: outletFilter.value, name: '', brand: 'hikvision', connection_type: 'hikvision', access_mode: 'gateway', source_url: '', playback_url: '', source_ticket: '', host: '', port: 554, channel: 1, subtype: 1, rtsp_username: '', rtsp_password: '', is_active: true, sort_order: 0, timezone: settings.timezone || 'Asia/Jakarta' })
const form = reactive(defaults())
const selectedConnection = computed(() => options.connections.find(c => c.id === form.connection_type))
const brandHelp = computed(() => ({
  hikvision: 'Hik-Connect adalah aplikasi akses perangkat. Koneksi ini memakai IP dan akun lokal kamera/DVR/NVR melalui penghubung outlet; login cloud Hik-Connect belum tersedia di sini.',
  hilook: 'Gunakan akun lokal perangkat HiLook. Akun aplikasi HiLook/Hik-Connect berbeda dari akun perangkat.',
  dahua: 'Untuk kamera yang terhubung ke DVR/NVR Dahua, isi IP dan akun DVR/NVR lalu pilih channel kamera. Login cloud/P2P Dahua belum tersedia di sini.',
  hkm: 'Untuk HKM-Smartview, gunakan ONVIF atau RTSP jika perangkat mendukung. Jika kamera tersambung ke DVR/NVR, isi alamat dan akun DVR/NVR. UID Smartview belum dapat dipakai langsung; jenis koneksi perlu dicocokkan dengan model perangkat.',
  ezviz: 'Gunakan RTSP jika model EZVIZ mendukung dan layanan lokalnya aktif. Password koneksi lokal dapat berbeda dari password akun EZVIZ. Login cloud EZVIZ belum tersedia di sini.',
}[form.brand] || ''))
const selectedCamera = ref(null), playbackOpen = ref(false), playbackError = ref('')
const playback = reactive({ start: '', end: '' })
const playerOpen = ref(false), playerMode = ref('live')
let refreshTimer, gatewayTimer, disposed = false, cloudGeneration = 0, gatewayGeneration = 0

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, outletList, catalog, gatewayList] = await Promise.all([camerasApi.list(), outletsApi.myOutlets(), camerasApi.options(), camerasApi.gateways()])
    if (disposed) return
    cameras.value = list
    outlets.value = outletList
    Object.assign(options, catalog)
    gateways.value = gatewayList
    gatewayError.value = ''
    monitorIds.value = monitorIds.value.filter(id => list.some(c => c.id === id && c.is_active))
    if (selectedCamera.value && playerOpen.value) {
      const current = list.find(c => c.id === selectedCamera.value.id && c.is_active)
      if (!current || current.updated_at !== selectedCamera.value.updated_at) playerOpen.value = false
    }
  } catch (e) { if (!disposed) error.value = e.message }
  finally { loading.value = false }
}

function toggleMonitor(camera) {
  if (monitorIds.value.includes(camera.id)) monitorIds.value = monitorIds.value.filter(id => id !== camera.id)
  else if (monitored.value.length < 4 && camera.is_active) {
    monitorIds.value.push(camera.id)
    nextTick(() => monitorSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
  }
}

function monitorOutlet(group) {
  monitorIds.value = group.cameras.filter(c => c.is_active).slice(0, 4).map(c => c.id)
  nextTick(() => monitorSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
}

function editCamera(camera = null) {
  if (saving.value) return
  ++cloudGeneration
  probeResult.value = null
  onvifProfiles.value = []
  profileError.value = ''
  Object.assign(savedSecrets, { password: !!camera?.has_password, source: !!camera?.has_source_url, playback: !!camera?.has_playback_url })
  Object.assign(clearSecrets, { password: false, source: false, playback: false })
  editing.value = camera?.id || null
  editedConnection.value = camera?.connection_type || ''
  const initial = defaults()
  if (camera) for (const key of Object.keys(initial)) if (key in camera) initial[key] = camera[key]
  Object.assign(form, initial, { rtsp_password: '', source_url: '', playback_url: '', source_ticket: '' })
  cloudDevices.value = []
  cloud.password = ''
  cloudError.value = ''
  formError.value = ''
  formOpen.value = true
}

function cameraPayload() {
  const payload = { ...form }
  delete payload.outlet_id
  if (clearSecrets.password) payload.rtsp_password = ''
  else if (editing.value && !payload.rtsp_password) delete payload.rtsp_password
  if (clearSecrets.source) payload.source_url = ''
  else if (!payload.source_url && (payload.source_ticket || editing.value && editedConnection.value === form.connection_type)) delete payload.source_url
  if (clearSecrets.playback) payload.playback_url = ''
  else if (editing.value && !payload.playback_url) delete payload.playback_url
  return payload
}

async function probeConfiguration() {
  if (probing.value || saving.value) return
  probing.value = true
  probeResult.value = null
  const payload = { ...cameraPayload(), camera_id: editing.value || '' }
  const snapshot = JSON.stringify(payload), outlet = form.outlet_id
  try {
    await camerasApi.probe(outlet, payload)
    if (formOpen.value && outlet === form.outlet_id && snapshot === JSON.stringify({ ...cameraPayload(), camera_id: editing.value || '' })) probeResult.value = { ok: true, message: 'Koneksi video berhasil. Konfigurasi belum disimpan.' }
  } catch (e) {
    if (formOpen.value && outlet === form.outlet_id && snapshot === JSON.stringify({ ...cameraPayload(), camera_id: editing.value || '' })) probeResult.value = { ok: false, message: e.message }
  } finally { probing.value = false }
}

async function saveCamera() {
  if (saving.value || probing.value) return
  saving.value = true
  formError.value = ''
  try {
    const payload = cameraPayload()
    if (editing.value) await camerasApi.update(editing.value, payload)
    else await camerasApi.create(form.outlet_id, payload)
    form.rtsp_password = form.source_url = form.playback_url = form.source_ticket = ''
    formOpen.value = false
    await load()
  } catch (e) { formError.value = e.message }
  finally { saving.value = false }
}

async function toggleCamera(camera) {
  busy.value = camera.id
  error.value = ''
  try { await camerasApi.toggle(camera.id); await load() }
  catch (e) { error.value = e.message }
  finally { busy.value = '' }
}

async function deleteCamera(camera) {
  if (!window.confirm(`Hapus kamera "${camera.name}" dari ${camera.outlet_name}?`)) return
  busy.value = camera.id
  error.value = ''
  try { await camerasApi.remove(camera.id); await load() }
  catch (e) { error.value = e.message }
  finally { busy.value = '' }
}

function changeConnection() {
  ++cloudGeneration
  probeResult.value = null
  clearSecrets.source = false
  form.port = selectedConnection.value?.port || 0
  form.source_url = form.source_ticket = ''
  cloudDevices.value = []
  onvifProfiles.value = []
  profileError.value = ''
  if (form.connection_type === 'tuya') form.access_mode = 'direct'
}

function changeBrand() {
  const profiles = { hikvision: 'hikvision', hilook: 'hikvision', dahua: 'dahua', imou: 'dahua', ezviz: 'rtsp', 'tp-link tapo': 'tapo', reolink: 'reolink', tuya: 'tuya', icsee: 'dvrip', xmeye: 'dvrip' }
  form.connection_type = profiles[form.brand] || 'onvif'
  form.access_mode = form.connection_type === 'tuya' ? 'direct' : 'gateway'
  changeConnection()
}

async function refreshGateways() {
  try {
    const list = await camerasApi.gateways()
    if (!disposed) { gateways.value = list; gatewayError.value = '' }
  }
  catch (e) { gatewayError.value = e.message }
}

function openGateway() {
  ++gatewayGeneration
  gatewayOutlet.value = outletFilter.value
  gatewayToken.value = gatewayError.value = ''
  gatewayCloudURL.value = window.location.origin
  gatewayOpen.value = true
  refreshGateways()
}

async function createGateway() {
  if (creatingGateway.value || revokingGateway.value) return
  const rotate = !!gatewayFor(gatewayOutlet.value)
  if (rotate && !window.confirm('Ganti token? Penghubung dengan token lama perlu dikonfigurasi ulang.')) return
  creatingGateway.value = true
  gatewayError.value = ''
  const outlet = gatewayOutlet.value, request = ++gatewayGeneration
  try {
    const result = await camerasApi.createGateway(outlet, rotate)
    if (gatewayOpen.value && outlet === gatewayOutlet.value && request === gatewayGeneration) gatewayToken.value = result.token
    await refreshGateways()
  } catch (e) { gatewayError.value = e.message }
  finally { creatingGateway.value = false }
}

async function revokeGateway() {
  if (creatingGateway.value || revokingGateway.value) return
  if (!window.confirm('Cabut akses penghubung outlet? Kamera yang memakai penghubung ini tidak dapat ditonton sampai penghubung baru dibuat.')) return
  revokingGateway.value = true
  gatewayError.value = ''
  const outlet = gatewayOutlet.value
  try {
    await camerasApi.deleteGateway(outlet)
    gatewayToken.value = ''
    await refreshGateways()
    monitorIds.value = monitorIds.value.filter(id => !cameras.value.some(c => c.id === id && c.outlet_id === outlet && c.access_mode === 'gateway'))
    if (selectedCamera.value?.outlet_id === outlet && selectedCamera.value.access_mode === 'gateway') playerOpen.value = false
  } catch (e) { gatewayError.value = e.message }
  finally { revokingGateway.value = false }
}

function formatSeen(value) {
  return new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short', timeZone: settings.timezone || 'Asia/Jakarta' }).format(new Date(value))
}

function downloadGatewayConfig() {
  let cloudURL
  try {
    cloudURL = new URL(gatewayCloudURL.value)
    if (cloudURL.protocol !== 'https:' || cloudURL.username || cloudURL.password || cloudURL.search || cloudURL.hash) throw new Error()
  } catch {
    gatewayError.value = 'Isi alamat HTTPS cloud yang dapat dijangkau komputer outlet, tanpa username, query, atau fragment.'
    return
  }
  gatewayError.value = ''
  const blob = new Blob([`CCTV_CLOUD_URL=${cloudURL.href.replace(/\/$/, '')}\nCCTV_GATEWAY_TOKEN=${gatewayToken.value}\n`], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = 'cctv-gateway.env'; a.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function testConnection(camera) {
  checking.value = camera.id
  delete checks[camera.id]
  try { await camerasApi.test(camera.id); checks[camera.id] = { ok: true, message: 'Terhubung' } }
  catch (e) { checks[camera.id] = { ok: false, message: e.message } }
  finally { checking.value = '' }
}

async function findONVIFProfiles() {
  if (findingProfiles.value) return
  findingProfiles.value = true
  profileError.value = ''
  const outlet = form.outlet_id, payload = { ...cameraPayload(), camera_id: editing.value || '' }
  const snapshot = JSON.stringify(payload)
  try {
    const profiles = await camerasApi.onvifProfiles(outlet, payload)
    if (!formOpen.value || form.connection_type !== 'onvif' || form.outlet_id !== outlet || snapshot !== JSON.stringify({ ...cameraPayload(), camera_id: editing.value || '' })) return
    onvifProfiles.value = profiles
    if (profiles.length === 1) { clearSecrets.source = false; form.source_ticket = profiles[0].ticket }
  } catch (e) { if (formOpen.value && form.connection_type === 'onvif' && form.outlet_id === outlet) profileError.value = e.message }
  finally { findingProfiles.value = false }
}

async function discoverCloud() {
  if (discovering.value) return
  discovering.value = true
  cloudError.value = ''
  const request = ++cloudGeneration, outlet = form.outlet_id
  const credentials = { ...cloud }
  cloud.password = ''
  try {
    const devices = await camerasApi.discoverTuya(outlet, credentials)
    if (request !== cloudGeneration || !formOpen.value || form.connection_type !== 'tuya' || form.outlet_id !== outlet) return
    cloudDevices.value = devices
    if (!devices.length) cloudError.value = 'Tidak ada kamera yang didukung pada akun/region ini.'
  } catch (e) { if (request === cloudGeneration) cloudError.value = e.message }
  finally { discovering.value = false; credentials.password = '' }
}

function selectCloudDevice() {
  const device = cloudDevices.value.find(d => d.ticket === form.source_ticket)
  if (device && !form.name) form.name = device.name
  form.brand = 'tuya'
}

function choosePlayback(camera) {
  selectedCamera.value = camera
  playbackError.value = ''
  playbackOpen.value = true
}

function startLive(camera) {
  selectedCamera.value = camera
  playerMode.value = 'live'
  playerOpen.value = true
}

function startPlayback() {
  if (!playback.start || !playback.end || playback.end <= playback.start) {
    playbackError.value = 'Waktu selesai harus setelah waktu mulai.'
    return
  }
  playerMode.value = 'playback'
  playbackOpen.value = false
  playerOpen.value = true
}

watch([form, clearSecrets], () => { probeResult.value = null }, { deep: true })
watch(() => form.outlet_id, () => { ++cloudGeneration; cloudDevices.value = []; onvifProfiles.value = []; form.source_ticket = '' })
watch(() => [form.host, form.port, form.rtsp_username, form.rtsp_password, form.access_mode, form.source_url], () => {
  if (form.connection_type === 'onvif') { onvifProfiles.value = []; form.source_ticket = ''; profileError.value = '' }
})
watch(() => clearSecrets.source, clear => { if (clear && form.connection_type === 'onvif') { onvifProfiles.value = []; form.source_ticket = '' } })
watch(formOpen, open => {
  if (!open) {
    ++cloudGeneration
    form.rtsp_password = form.source_url = form.playback_url = form.source_ticket = ''
    cloud.password = ''
    cloudDevices.value = []
    onvifProfiles.value = []
  }
})
watch(gatewayOpen, open => { if (!open) { ++gatewayGeneration; gatewayToken.value = '' } })
watch(() => !!monitored.value.length || playerOpen.value, active => {
  window.clearInterval(refreshTimer)
  if (active) refreshTimer = window.setInterval(() => {
    camerasApi.refreshSession().catch(e => { error.value = e.message; monitorIds.value = []; playerOpen.value = false })
  }, 30 * 60 * 1000)
})
onMounted(async () => {
  await load()
  if (disposed) return
  gatewayTimer = window.setInterval(() => { if (!document.hidden) refreshGateways() }, 20000)
})
onBeforeUnmount(() => {
  disposed = true
  ++cloudGeneration
  window.clearInterval(refreshTimer)
  window.clearInterval(gatewayTimer)
})
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: .375rem; font-size: .875rem; font-weight: 500; color: #374151; }
.form-input { width: 100%; min-width: 0; padding: .5rem .7rem; border-radius: .6rem; border: 1px solid #d1d5db; background: #fff; color: #111827; font-weight: 400; }
.form-input:focus { outline: 2px solid #a7f3d0; border-color: #059669; }
</style>
