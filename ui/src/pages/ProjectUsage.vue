<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div><h1 class="text-xl font-bold text-gray-900">Penggunaan Barang Projek</h1><p class="mt-1 text-sm text-gray-500">Pertanggungjawaban material, bukti pekerjaan, dan penempatan peralatan.</p></div>
      <div class="flex gap-2"><AppButton variant="secondary" :loading="exporting" @click="exportReports">Unduh Excel</AppButton><AppButton v-if="canCreate" :disabled="!projectId || closed" @click="newReport">Buat laporan</AppButton></div>
    </div>
    <AppAlert type="error" :message="error" />
    <AppCard>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <div class="lg:col-span-2"><label for="usage-project" class="lbl">Projek</label><select id="usage-project" v-model="projectId" class="field"><option value="">Semua projek</option><option v-for="p in projects" :key="p.id" :value="p.id">{{ p.project_number }} · {{ p.name }}</option></select></div>
        <div><label for="usage-status" class="lbl">Status laporan</label><select id="usage-status" v-model="status" class="field"><option value="">Semua status</option><option v-for="(label, key) in statuses" :key="key" :value="key">{{ label }}</option></select></div>
        <div><label for="usage-from" class="lbl">Dari tanggal</label><input id="usage-from" v-model="from" type="date" class="field" /></div>
        <div><label for="usage-to" class="lbl">Sampai tanggal</label><input id="usage-to" v-model="to" type="date" class="field" /></div>
      </div>
      <p class="mt-3 text-xs text-gray-500">{{ projectId ? (closed ? 'Projek sudah ditutup. Laporan tersedia untuk dibaca.' : 'Pilih Buat laporan untuk mencatat pemakaian atau susut material.') : 'Pilih projek untuk melihat saldo barang dan membuat laporan.' }}</p>
    </AppCard>
    <div v-if="overview" class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <AppCard><p class="text-xs text-gray-500">Material diterima</p><p class="mt-1 font-semibold">{{ money(overview.summary.value_received) }}</p></AppCard>
      <AppCard><p class="text-xs text-gray-500">Material terpakai</p><p class="mt-1 font-semibold">{{ money(overview.summary.value_used) }}</p></AppCard>
      <AppCard><p class="text-xs text-gray-500">Sisa material</p><p class="mt-1 font-semibold">{{ money(overview.summary.value_remaining) }}</p></AppCard>
      <AppCard><p class="text-xs text-gray-500">Laporan belum diverifikasi</p><p class="mt-1 font-semibold">{{ overview.pending_reports }}</p></AppCard>
    </div>
    <div class="flex flex-wrap gap-2" aria-label="Tampilan penggunaan barang">
      <button v-for="t in tabs" :key="t.key" type="button" class="rounded-lg px-3 py-2 text-sm font-medium" :class="tab === t.key ? 'bg-emerald-700 text-white' : 'bg-white text-gray-600 border'" :disabled="t.key !== 'reports' && !overview" @click="tab = t.key">{{ t.label }}</button>
    </div>
    <AppCard v-if="tab === 'reports'">
      <p v-if="loading" role="status" class="py-8 text-center text-gray-500">Memuat laporan…</p>
      <p v-else-if="!reports.length" class="py-8 text-center text-gray-500">Belum ada laporan penggunaan barang.</p>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="r in reports" :key="r.id" class="flex flex-wrap items-center justify-between gap-3 py-4">
          <div class="min-w-0 flex-1"><button class="text-left font-semibold text-emerald-800" @click="openReport(r.id)">{{ r.report_number }}</button><p class="text-sm text-gray-700 break-words">{{ r.project_name }} · {{ r.kind === 'susut' ? 'Susut / rusak' : 'Pemakaian' }}</p><p class="text-xs text-gray-500">{{ r.usage_date }} · {{ r.created_by }} · {{ r.items.length }} barang</p></div>
          <span class="rounded-full px-2 py-1 text-xs font-semibold" :class="badge(r.status)">{{ statuses[r.status] }}</span>
          <AppButton size="sm" variant="secondary" @click="openReport(r.id)">Lihat laporan</AppButton>
        </li>
      </ul>
      <AppPagination :page="page" :total="total" :per-page="20" @change="page = $event; loadReports()" />
    </AppCard>
    <AppCard v-else-if="tab === 'materials' && overview">
      <ProjectMaterialPanel :key="projectId" :project-id="projectId" :closed="closed" report-mode @usage="newReport" @changed="loadOverview" />
    </AppCard>
    <AppCard v-else-if="tab === 'assets' && overview">
      <p class="mb-4 text-sm text-gray-500">Peralatan tetap menjadi aset. Lokasi, PIC, kondisi, dan bukti distribusinya ditampilkan di sini.</p>
      <p v-if="!overview.assets.length" class="py-6 text-center text-gray-500">Belum ada peralatan dari projek ini.</p>
      <ul class="divide-y"><li v-for="a in overview.assets" :key="a.id" class="py-4 space-y-2"><p class="font-semibold">{{ a.number }} · {{ a.name }}</p><p class="text-sm">{{ a.qty }} {{ a.unit }} · {{ a.condition }} · {{ a.location || 'Lokasi belum diisi' }}</p><p class="text-sm">PIC: {{ a.pic || 'Belum diserahkan' }}</p><div class="flex flex-wrap gap-3"><a v-for="(url, i) in a.evidence" :key="url" :href="safeUrl(url)" target="_blank" rel="noopener noreferrer" class="text-sm text-emerald-700 underline">Bukti serah terima {{ i + 1 }}</a></div></li></ul>
    </AppCard>

    <AppModal :model-value="showEditor" title="Laporan penggunaan barang" size="2xl" @update:model-value="closeEditor">
      <form id="usage-form" class="space-y-4" @submit.prevent="save">
        <AppAlert type="error" :message="formError" />
        <div class="grid gap-3 sm:grid-cols-2">
          <div><label for="report-kind" class="lbl">Jenis laporan</label><select id="report-kind" v-model="form.kind" :disabled="!!editingId || saving" class="field"><option value="pemakaian">Pemakaian material</option><option value="susut">Susut / rusak / terbuang</option></select></div>
          <div><label for="report-date" class="lbl">Tanggal penggunaan</label><input id="report-date" v-model="form.usage_date" type="date" required :max="today" class="field" /></div>
          <div><label for="report-location" class="lbl">Lokasi pekerjaan</label><input id="report-location" v-model="form.location" maxlength="200" class="field" placeholder="Contoh: dapur lantai 1" /></div>
          <div><label for="report-performer" class="lbl">Pelaksana</label><input id="report-performer" v-model="form.performer" maxlength="150" class="field" /></div>
          <div class="sm:col-span-2"><label for="report-pic" class="lbl">PIC yang mengonfirmasi penggunaan</label><input id="report-pic" v-model="form.pic_name" maxlength="150" class="field" /></div>
        </div>
        <div><label for="report-work" class="lbl">Pekerjaan / alasan susut</label><textarea id="report-work" v-model="form.work_description" rows="3" maxlength="3000" class="field" placeholder="Jelaskan penggunaan barang dan hasil pekerjaan atau penyebab kerusakan." /></div>
        <fieldset class="space-y-3"><legend class="text-sm font-semibold mb-2">Barang yang dilaporkan</legend>
          <div v-for="(item, i) in form.items" :key="i" class="rounded-lg bg-gray-50 p-3 space-y-2">
            <label :for="`item-${i}`" class="lbl">Barang {{ i + 1 }}</label><select :id="`item-${i}`" v-model="item.material_id" required class="field"><option value="">Pilih material</option><option v-for="m in editorMaterials" :key="m.id" :value="m.id">{{ m.name }} · {{ m.request_number || 'Pindahan' }} · sisa {{ qty(m.remaining) }} {{ m.unit }}</option></select>
            <div class="flex gap-3 items-end"><div class="flex-1"><label :for="`qty-${i}`" class="lbl">Jumlah {{ i + 1 }}</label><input :id="`qty-${i}`" v-model.number="item.qty" type="number" min="0.0001" step="0.0001" required class="field" /></div><button v-if="form.items.length > 1" type="button" class="text-sm text-red-600 pb-2" @click="form.items.splice(i, 1)">Hapus</button></div>
          </div>
          <AppButton type="button" size="sm" variant="secondary" @click="form.items.push({ material_id: '', qty: 1 })">Tambah barang</AppButton>
        </fieldset>
        <MaterialEvidence v-model="form.evidence" :disabled="saving" @busy="uploading = $event" />
        <p class="text-xs text-gray-500">Draft belum mengurangi saldo. Saat diajukan, jumlah dicatat sebagai pemakaian/susut. Foto, pekerjaan, lokasi, pelaksana, dan PIC wajib lengkap saat diajukan.</p>
      </form>
      <template #footer><AppButton variant="secondary" :disabled="saving || uploading" @click="closeEditor(false)">Batal</AppButton><AppButton type="submit" form="usage-form" :loading="saving" :disabled="uploading">Simpan draft / perbaikan</AppButton></template>
    </AppModal>

    <AppModal v-model="showDetail" :title="detail?.report_number || 'Laporan'" size="2xl">
      <div v-if="detail" class="space-y-4">
        <AppAlert type="error" :message="detailError" />
        <div class="flex flex-wrap justify-between gap-2"><h2 class="font-semibold">{{ detail.project_name }}</h2><span class="rounded-full px-3 py-1 text-xs" :class="badge(detail.status)">{{ statuses[detail.status] }}</span></div>
        <dl class="grid grid-cols-2 gap-3 text-sm"><div><dt class="text-gray-500">Tanggal penggunaan</dt><dd>{{ detail.usage_date }}</dd></div><div><dt class="text-gray-500">Lokasi</dt><dd>{{ detail.location || '—' }}</dd></div><div><dt class="text-gray-500">Pelaksana</dt><dd>{{ detail.performer || '—' }}</dd></div><div><dt class="text-gray-500">PIC</dt><dd>{{ detail.pic_name || '—' }}</dd></div><div><dt class="text-gray-500">Dicatat oleh</dt><dd>{{ detail.created_by }} · {{ date(detail.created_at) }}</dd></div><div><dt class="text-gray-500">Diperiksa oleh</dt><dd>{{ detail.reviewed_by || 'Belum diperiksa' }}<template v-if="detail.reviewed_at"> · {{ date(detail.reviewed_at) }}</template></dd></div></dl>
        <p class="whitespace-pre-line break-words rounded-lg bg-gray-50 p-3 text-sm">{{ detail.work_description || 'Pekerjaan belum diisi' }}</p>
        <ul class="divide-y text-sm"><li v-for="item in detail.items" :key="item.material_id" class="py-2"><strong>{{ item.name }}</strong> · {{ qty(item.qty) }} {{ item.unit }}<p class="text-xs text-gray-500">{{ item.request_number }}</p></li></ul>
        <div class="flex flex-wrap gap-3"><a v-for="(url, i) in detail.evidence" :key="url" :href="safeUrl(url)" target="_blank" rel="noopener noreferrer" class="text-sm text-emerald-700 underline"><span v-if="url.toLowerCase().endsWith('.pdf')">PDF {{ i + 1 }}</span><img v-else :src="safeUrl(url)" :alt="`Bukti penggunaan ${i + 1}`" class="h-28 w-36 rounded-lg border object-cover" /></a></div>
        <p v-if="detail.review_note" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">Catatan pemeriksa / pembatalan: {{ detail.review_note }}</p>
        <p v-if="detail.status === 'revision'" class="text-xs text-gray-600">Saldo masih mengikuti pengajuan terakhir. Perubahan jumlah diterapkan saat diajukan ulang.</p>
        <div v-if="mayReview || mayEdit"><label for="review-note" class="lbl">Catatan pemeriksaan / alasan pembatalan</label><textarea id="review-note" v-model="actionNote" rows="2" maxlength="2000" class="field" /></div>
        <details><summary class="cursor-pointer text-sm font-semibold">Riwayat perubahan ({{ detail.events?.length || 0 }})</summary><ul class="mt-2 space-y-3 text-xs"><li v-for="(event, i) in detail.events" :key="i" class="border-l-2 pl-3"><p class="font-medium">{{ eventLabels[event.action] }} · {{ event.actor }} · {{ date(event.created_at) }}</p><p v-if="event.note">{{ event.note }}</p><details class="mt-1"><summary class="cursor-pointer text-gray-500">Isi laporan saat itu</summary><p class="mt-1">{{ event.snapshot.work_description }}</p><p v-for="item in event.snapshot.items" :key="item.material_id">{{ item.name }}: {{ qty(item.qty) }} {{ item.unit }}</p><a v-for="(url, j) in event.snapshot.evidence" :key="url" :href="safeUrl(url)" target="_blank" rel="noopener noreferrer" class="mr-3 text-emerald-700 underline">Bukti {{ j + 1 }}</a></details></li></ul></details>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-2">
          <template v-if="mayEdit"><AppButton variant="secondary" :disabled="acting" @click="editReport">Perbaiki laporan</AppButton><AppButton variant="secondary" :loading="acting" @click="act('cancel')">Batalkan laporan</AppButton><AppButton :loading="acting" @click="act('submit')">Ajukan laporan</AppButton></template>
          <template v-if="mayReview"><AppButton variant="secondary" :loading="acting" @click="act('revise')">Minta perbaikan</AppButton><AppButton :loading="acting" @click="act('verify')">Verifikasi laporan</AppButton></template>
          <AppButton variant="secondary" :disabled="acting" @click="showDetail = false">Tutup</AppButton>
        </div>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { projectUsageApi } from '@/api/projectUsage.js'
import { useAuthStore } from '@/stores/auth.js'
import { useToastStore } from '@/stores/toast.js'
import { formatDateTime, formatRupiah } from '@/utils/format.js'
import { safeUrl } from '@/utils/safeUrl.js'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
import MaterialEvidence from '@/components/MaterialEvidence.vue'
import ProjectMaterialPanel from '@/components/ProjectMaterialPanel.vue'

const route = useRoute(), auth = useAuthStore(), toast = useToastStore()
const canCreate = computed(() => auth.hasPermission('assets.usage.create'))
const actorID = computed(() => !auth.currentAdmin?.id || auth.currentAdmin.id === 'static' ? 'static-admin' : auth.currentAdmin.id)
const statuses = { draft: 'Draft', submitted: 'Diajukan', revision: 'Perlu perbaikan', verified: 'Diverifikasi', cancelled: 'Dibatalkan' }
const eventLabels = { save: 'Disimpan', submit: 'Diajukan', revise: 'Diminta perbaikan', verify: 'Diverifikasi', cancel: 'Dibatalkan' }
const tabs = [{ key: 'reports', label: 'Laporan & bukti' }, { key: 'materials', label: 'Saldo material' }, { key: 'assets', label: 'Peralatan & PIC' }]
const money = formatRupiah, date = formatDateTime, qty = n => Number(n || 0).toLocaleString('id-ID', { maximumFractionDigits: 4 })
const badge = s => s === 'verified' ? 'bg-emerald-100 text-emerald-800' : s === 'revision' ? 'bg-red-100 text-red-800' : 'bg-gray-100 text-gray-700'
const projects = ref([]), projectId = ref(String(route.query.project_id || '')), overview = ref(null), tab = ref('reports')
const status = ref(''), from = ref(''), to = ref(''), page = ref(1), total = ref(0), reports = ref([]), loading = ref(false), error = ref(''), exporting = ref(false)
const closed = computed(() => ['selesai', 'batal'].includes(overview.value?.project.status))
const showEditor = ref(false), showDetail = ref(false), editingId = ref(''), editorMaterials = ref([]), formError = ref(''), detailError = ref(''), saving = ref(false), uploading = ref(false), acting = ref(false), detail = ref(null), actionNote = ref('')
const today = new Intl.DateTimeFormat('en-CA', { timeZone: localStorage.getItem('cloud_pos_timezone') || 'Asia/Jakarta', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date())
const form = reactive({ project_id: '', kind: 'pemakaian', usage_date: today, location: '', work_description: '', performer: '', pic_name: '', items: [], evidence: [], version: 0 })
const mayEdit = computed(() => canCreate.value && detail.value?.created_by_id === actorID.value && ['draft', 'revision'].includes(detail.value?.status))
const mayReview = computed(() => auth.hasPermission('assets.usage.verify') && detail.value?.status === 'submitted' && detail.value?.created_by_id !== actorID.value && detail.value?.submitted_by_id !== actorID.value)
const params = () => ({ project_id: projectId.value, status: status.value, from: from.value, to: to.value, page: page.value, limit: 20 })
let listSeq = 0, overviewSeq = 0
async function loadReports() {
  const seq = ++listSeq; loading.value = true; error.value = ''
  try { const data = await projectUsageApi.list(params()); if (seq === listSeq) { reports.value = data.items; total.value = data.total } }
  catch (e) { if (seq === listSeq) error.value = e.message }
  finally { if (seq === listSeq) loading.value = false }
}
async function loadOverview() {
  const seq = ++overviewSeq; overview.value = null
  if (!projectId.value) { tab.value = 'reports'; return }
  try { const data = await projectUsageApi.overview(projectId.value); if (seq === overviewSeq) overview.value = data }
  catch (e) { if (seq === overviewSeq) error.value = e.message }
}
async function newReport() {
  if (!overview.value || closed.value || !canCreate.value) return
  editingId.value = ''; editorMaterials.value = overview.value.materials
  Object.assign(form, { project_id: projectId.value, kind: 'pemakaian', usage_date: today, location: '', work_description: '', performer: '', pic_name: '', items: [{ material_id: '', qty: 1 }], evidence: [], version: 0 })
  formError.value = ''; showEditor.value = true
}
function closeEditor(value) { if (!saving.value && !uploading.value) showEditor.value = value }
async function openReport(id) {
  try { detail.value = await projectUsageApi.get(id); detailError.value = ''; actionNote.value = ''; showDetail.value = true }
  catch (e) { error.value = e.message }
}
async function editReport() {
  try {
    const data = await projectUsageApi.overview(detail.value.project_id)
    editorMaterials.value = data.materials; editingId.value = detail.value.id
    Object.assign(form, JSON.parse(JSON.stringify(detail.value)))
    showDetail.value = false; formError.value = ''; showEditor.value = true
  } catch (e) { detailError.value = e.message }
}
async function save() {
  if (saving.value || uploading.value) return
  saving.value = true; formError.value = ''
  try { const data = await projectUsageApi.save(editingId.value, form); showEditor.value = false; await openReport(data.id); await loadReports(); toast.success('Laporan tersimpan') }
  catch (e) { formError.value = e.message }
  finally { saving.value = false }
}
async function act(action) {
  if (acting.value) return
  if (['revise', 'cancel'].includes(action) && !actionNote.value.trim()) { detailError.value = 'Isi catatan / alasan terlebih dahulu.'; return }
  acting.value = true; detailError.value = ''
  try { detail.value = await projectUsageApi.action(detail.value.id, { action, version: detail.value.version, note: actionNote.value }); actionNote.value = ''; await Promise.all([loadReports(), loadOverview()]); toast.success('Laporan diperbarui') }
  catch (e) { detailError.value = e.message }
  finally { acting.value = false }
}
async function exportReports() {
  exporting.value = true
  try { const blob = await projectUsageApi.export(params()); const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = 'penggunaan-barang-projek.xlsx'; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000) }
  catch (e) { error.value = e.message }
  finally { exporting.value = false }
}
watch(projectId, () => { page.value = 1; loadReports(); loadOverview() })
watch([status, from, to], () => { page.value = 1; loadReports() })
onMounted(async () => { try { projects.value = await projectUsageApi.projects() } catch (e) { error.value = e.message }; await Promise.all([loadReports(), loadOverview()]) })
</script>
<style scoped>
.lbl { display:block;font-size:.8rem;font-weight:500;color:#374151;margin-bottom:.35rem; }
.field { display:block;width:100%;min-width:0;border:1px solid #d1d5db;border-radius:.5rem;background:white;padding:.55rem .7rem;font-size:.875rem; }
.field:focus { outline:2px solid #059669;outline-offset:1px; }
</style>
