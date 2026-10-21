<template>
  <div class="space-y-5">
    <div class="flex flex-wrap justify-between items-center gap-3"><div><h1 class="text-2xl font-bold text-gray-900">Pengembangan Produk</h1><p class="text-sm text-gray-500 mt-1">Dari konsep makanan/minuman sampai menu siap diluncurkan.</p></div><AppButton v-if="canCreate" @click="open()">+ Ide Produk</AppButton></div>
    <AppCard><div class="flex flex-wrap items-end gap-3"><FnbOutletFilter v-model="outlet" /><div><label for="dev-kind" class="label">Kategori</label><select id="dev-kind" v-model="kind" class="field"><option value="">Makanan & minuman</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option></select></div><div><label for="dev-stage" class="label">Tahap</label><select id="dev-stage" v-model="stage" class="field"><option value="">Semua tahap</option><option v-for="s in DEV_STAGES" :key="s.value" :value="s.value">{{ s.label }}</option></select></div><input v-model="search" class="field flex-1" placeholder="Cari ide / nama produk" aria-label="Cari pengembangan" /></div></AppCard>
    <AppAlert type="error" :message="error" />
    <div class="flex gap-2 overflow-x-auto pb-1"><button v-for="s in DEV_STAGES" :key="s.value" @click="stage = stage === s.value ? '' : s.value" class="rounded-xl border px-4 py-3 text-left min-w-28" :class="stage === s.value ? 'border-emerald-500 bg-emerald-50' : 'border-gray-200 bg-white'"><span class="text-xs text-gray-500 block">{{ s.label }}</span><b class="text-xl">{{ rows.filter(r => r.status === s.value && (!kind || r.menu_kind === kind)).length }}</b></button></div>
    <p v-if="loading" class="text-sm text-gray-500 py-8">Memuat pengembangan…</p>
    <AppCard v-else-if="!filtered.length"><p class="text-sm text-gray-500">Belum ada pengembangan dalam filter ini. Catat ide pertama beserta target harga dan rencana uji coba.</p></AppCard>
    <div v-else class="grid md:grid-cols-2 xl:grid-cols-3 gap-4">
      <AppCard v-for="row in filtered" :key="row.id">
        <div class="flex justify-between items-start gap-2"><span class="text-xs font-semibold text-emerald-700">{{ kindLabel(row.menu_kind) }}</span><span class="text-xs bg-gray-100 rounded-full px-2 py-1">{{ stageLabel(row.status) }}</span></div>
        <button @click="open(row)" class="block text-left text-lg font-semibold text-gray-900 mt-3 hover:text-emerald-700">{{ row.name }}</button><p class="text-xs text-gray-400">{{ row.outlet_name }}</p>
        <p class="text-sm text-gray-600 mt-3 line-clamp-3 whitespace-pre-line">{{ row.brief || 'Belum ada ringkasan konsep.' }}</p>
        <dl class="mt-4 text-xs space-y-2"><div class="flex justify-between"><dt class="text-gray-500">Harga target</dt><dd>{{ money(row.target_price) }}</dd></div><div class="flex justify-between"><dt class="text-gray-500">Target HPP</dt><dd>{{ row.target_cost_pct }}%</dd></div><div><dt class="text-gray-500">Resep</dt><dd>{{ row.recipe_name || 'Tanpa resep / belum ditautkan' }}</dd></div><div><dt class="text-gray-500">Produk POS</dt><dd>{{ row.product_name || 'Belum ditautkan' }}</dd></div></dl>
        <p v-if="row.approved_by" class="text-xs text-emerald-700 mt-3">Disetujui: {{ row.approved_by }}</p>
        <div class="flex flex-wrap gap-2 mt-4 pt-3 border-t border-gray-100"><button @click="open(row)" class="text-xs text-emerald-700 font-semibold">Detail</button><button v-for="next in actions(row)" :key="next" :disabled="acting === row.id" @click="transition(row, next)" class="text-xs border border-gray-200 rounded-lg px-2 py-1 hover:bg-gray-50 disabled:opacity-50">{{ actionLabel(next) }}</button></div>
      </AppCard>
    </div>
    <AppModal v-model="showForm" :title="editing ? 'Detail Pengembangan' : 'Ide Produk Baru'" size="lg">
      <fieldset :disabled="readOnly || saving" class="space-y-4 min-w-0">
        <div class="grid sm:grid-cols-2 gap-3"><AppInput v-model="form.name" label="Nama produk / ide" placeholder="Contoh: Es Kopi Pandan" /><div><label for="dev-form-kind" class="label">Kategori *</label><select id="dev-form-kind" v-model="form.menu_kind" class="field w-full"><option value="">Pilih kategori</option><option v-for="k in MENU_KINDS" :key="k.value" :value="k.value">{{ k.label }}</option></select></div></div>
        <div><label class="label">Outlet pengujian / peluncuran *</label><SearchSelect v-model="form.outlet_id" :options="outlets" :disabled="!!editing || readOnly" @change="changeOutlet" placeholder="Pilih outlet" /></div>
        <div><label for="dev-brief" class="label">Konsep & sasaran produk</label><textarea id="dev-brief" v-model="form.brief" rows="3" class="field w-full" placeholder="Konsep, target pelanggan, porsi, penyajian, dan pembeda produk"></textarea></div>
        <div class="grid sm:grid-cols-2 gap-3"><AppInput v-model.number="form.target_price" label="Harga jual target (Rp)" type="number" min="0" step="0.01" /><AppInput v-model.number="form.target_cost_pct" label="Target HPP (%)" type="number" min="0.01" max="100" step="0.01" /></div>
        <AppAlert type="error" :message="optionsError" />
        <div class="grid sm:grid-cols-2 gap-3"><div><label class="label">Master resep (opsional)</label><SearchSelect v-model="form.recipe_master_id" :options="recipeOptions" :disabled="readOnly || optionsLoading || !form.outlet_id" placeholder="Tanpa resep" /></div><div><label class="label">Produk POS (opsional)</label><SearchSelect v-model="form.product_id" :options="productOptions" :disabled="readOnly || optionsLoading || !form.outlet_id" placeholder="Belum ditautkan" /></div></div>
        <p class="text-xs text-gray-500">Resep boleh kosong untuk produk siap jual seperti minuman kemasan. Tautkan produk POS sebelum review bila akan diluncurkan; penautan di sini mencatat referensi pengembangan dan tidak mengubah resep atau harga produk POS.</p>
        <div><label for="dev-notes" class="label">Hasil uji coba & evaluasi</label><textarea id="dev-notes" v-model="form.trial_notes" rows="5" class="field w-full" placeholder="Tanggal/penguji, rasa, porsi/yield, tampilan, konsistensi, biaya, temuan, dan tindakan perbaikan"></textarea></div>
        <p v-if="editing?.status === 'review'" class="text-xs text-amber-700">Mengubah isi saat review akan mengembalikan pengembangan ke tahap uji coba.</p>
      </fieldset>
      <template #footer><AppButton variant="secondary" @click="showForm = false">Tutup</AppButton><AppButton v-if="!readOnly" :loading="saving" :disabled="optionsLoading || !!optionsError" @click="save">Simpan Pengembangan</AppButton></template>
    </AppModal>
  </div>
</template>
<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { fnbApi } from '@/api/fnb'
import { outletsApi } from '@/api/outlets'
import { MENU_KINDS, DEV_STAGES, kindLabel, stageLabel } from '@/utils/fnb'
import { formatRupiah as money } from '@/utils/format'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import FnbOutletFilter from '@/components/FnbOutletFilter.vue'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppButton from '@/components/ui/AppButton.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'
const auth = useAuthStore(), toast = useToastStore()
const canCreate = computed(() => auth.hasPermission('fnb.development.create'))
const canUpdate = computed(() => auth.hasPermission('fnb.development.update'))
const canApprove = computed(() => auth.hasPermission('fnb.development.approve'))
const outlet = ref(''), kind = ref(''), stage = ref(''), search = ref(''), rows = ref([]), loading = ref(false), error = ref('')
const showForm = ref(false), editing = ref(null), form = ref({}), saving = ref(false), acting = ref(''), outlets = ref([])
const recipes = ref([]), products = ref([]), optionsLoading = ref(false), optionsError = ref('')
const recipeOptions = computed(() => [{ id: '', name: 'Tanpa resep' }, ...recipes.value])
const productOptions = computed(() => [{ id: '', name: 'Belum ditautkan' }, ...products.value])
const readOnly = computed(() => editing.value ? !canUpdate.value || ['approved', 'launched', 'archived'].includes(editing.value.status) : !canCreate.value)
const filtered = computed(() => rows.value.filter(r => (!kind.value || r.menu_kind === kind.value) && (!stage.value || r.status === stage.value) && r.name.toLowerCase().includes(search.value.toLowerCase())))
let seq = 0, optionsSeq = 0
async function load() {
  const request = ++seq; loading.value = true; error.value = ''; rows.value = []
  try { const data = await fnbApi.developments({ outlet_id: outlet.value }); if (request === seq) rows.value = data || [] }
  catch (e) { if (request === seq) error.value = e.message }
  finally { if (request === seq) loading.value = false }
}
watch(outlet, load, { immediate: true })
onMounted(async () => { try { const data = await outletsApi.myOutlets(); outlets.value = data.outlets || data || [] } catch (e) { toast.error(e.message) } })
async function loadOptions() {
  const request = ++optionsSeq; optionsError.value = ''; recipes.value = []; products.value = []
  if (!form.value.outlet_id) { optionsLoading.value = false; return }
  optionsLoading.value = true
  try { const data = await fnbApi.options(form.value.outlet_id); if (request === optionsSeq) { recipes.value = data.recipes || []; products.value = data.products || [] } }
  catch (e) { if (request === optionsSeq) optionsError.value = e.message }
  finally { if (request === optionsSeq) optionsLoading.value = false }
}
function changeOutlet() { form.value.recipe_master_id = ''; form.value.product_id = ''; loadOptions() }
function open(row = null) {
  editing.value = row
  form.value = row ? { ...row } : { name: '', outlet_id: outlet.value, menu_kind: kind.value || '', brief: '', trial_notes: '', target_price: 0, target_cost_pct: 35, recipe_master_id: '', product_id: '', version: 0 }
  showForm.value = true; loadOptions()
}
async function save() {
  if (!form.value.name?.trim() || !form.value.outlet_id || !form.value.menu_kind) return toast.error('Nama, outlet, dan kategori wajib diisi')
  saving.value = true
  try { await fnbApi.saveDevelopment(editing.value?.id, form.value); showForm.value = false; toast.success('Pengembangan tersimpan'); await load() }
  catch (e) { toast.error(e.message) } finally { saving.value = false }
}
function actions(row) {
  const flow = { idea: ['trial', 'archived'], trial: ['review', 'archived'], review: ['trial', 'approved', 'archived'], approved: ['trial', 'launched', 'archived'], launched: ['archived'] }
  return (flow[row.status] || []).filter(next => next === 'approved' ? canApprove.value : canUpdate.value)
}
const actionLabel = status => ({ trial: 'Uji coba', review: 'Ajukan review', approved: 'Setujui', launched: 'Tandai diluncurkan', archived: 'Arsipkan' }[status])
async function transition(row, status) {
  acting.value = row.id
  try { await fnbApi.transition(row.id, status, row.version); toast.success(`Tahap: ${stageLabel(status)}`); await load() }
  catch (e) { toast.error(e.message); if (e.status === 409) await load() } finally { acting.value = '' }
}
</script>
<style scoped>
.field{border:1px solid #d1d5db;border-radius:.5rem;padding:.5rem .75rem;font-size:.875rem;background:white;min-width:0}.label{display:block;font-size:.75rem;color:#6b7280;margin-bottom:.35rem}fieldset:disabled .field{background:#f9fafb;color:#6b7280}
</style>
