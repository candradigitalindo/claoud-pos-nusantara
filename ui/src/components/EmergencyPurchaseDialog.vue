<!--
  EmergencyPurchaseDialog.vue — form Pembelian Darurat (keputusan owner
  19 Nov 2026). Barang/jasa urgent ≤ batas (bawaan Rp500.000) dibeli dulu
  dari kas laci outlet: setelah dicatat, tugas kas langsung muncul di menu
  Bayar Pengadaan tablet kasir outlet (PIN Manager/SVP). Struk diunggah
  sesudah belanja, lalu penyetuju mengesahkan.
-->
<template>
  <AppModal :model-value="modelValue" :title="`Pembelian Darurat — ${typeLabel}`" size="2xl" @update:model-value="close">
    <div class="space-y-4">
      <div class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-900">
        <p class="font-semibold">Untuk kebutuhan yang tidak bisa menunggu persetujuan.</p>
        <p class="mt-1">
          Maksimal <b>{{ formatRupiah(maxAmount) }}</b> per pembelian. Setelah dicatat, kasir outlet menyerahkan kas lewat
          menu <b>Bayar Pengadaan</b> di tablet dengan PIN Manager/SVP. Unggah foto struk sesudah belanja, lalu penyetuju mengesahkan.
          Di atas batas, buat pengajuan biasa bertingkat <b>Urgent</b> supaya disetujui dulu.
        </p>
      </div>

      <AppAlert type="error" :message="error" />

      <div v-if="!workUnits.length" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
        Anda belum memiliki unit kerja outlet dalam cakupan, jadi Pembelian Darurat belum bisa dicatat.
      </div>
      <div v-else-if="workUnits.length > 1" class="flex flex-col gap-1">
        <label class="text-sm font-medium text-gray-700">Unit Kerja Outlet <span class="text-red-500">*</span></label>
        <SearchSelect v-model="form.work_unit_id" :options="unitOptions" placeholder="Pilih unit kerja outlet…" searchPlaceholder="Cari unit kerja…" />
      </div>

      <!-- Konteks outlet: pemakaian bulan ini & kesiapan tablet kasir. -->
      <div v-if="ctxLoading" class="text-xs text-gray-400">Memeriksa outlet…</div>
      <div v-else-if="ctx" class="space-y-2">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-700">
          <span><b>{{ ctx.outlet_name }}</b></span>
          <span>Bulan ini: {{ ctx.count_this_month }}× · {{ formatRupiah(ctx.used_this_month) }}</span>
          <span v-if="ctx.remaining != null">Sisa plafon: <b :class="ctx.remaining < total ? 'text-red-700' : ''">{{ formatRupiah(ctx.remaining) }}</b></span>
        </div>
        <div v-if="ctx.tablet_note" :class="['flex items-start gap-1.5 rounded-lg border p-2.5 text-xs',
          ctx.tablet_ready ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-red-200 bg-red-50 text-red-800']">
          <svg class="mt-px shrink-0" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="4" y="2" width="16" height="20" rx="2"/><line x1="12" y1="18" x2="12.01" y2="18"/></svg>
          <span>{{ ctx.tablet_note }}<template v-if="ctx.tablet_version"> (terpasang {{ ctx.tablet_version }})</template></span>
        </div>
      </div>
      <p v-else-if="ctxError" class="text-xs text-red-600">{{ ctxError }}</p>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div class="flex flex-col gap-1 sm:col-span-2">
          <label class="text-sm font-medium text-gray-700">Kategori Darurat <span class="text-red-500">*</span></label>
          <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
            <label v-for="c in EMERGENCY_CATEGORIES" :key="c.value"
              :class="['flex cursor-pointer items-start gap-2 rounded-lg border px-2.5 py-2 text-xs',
                form.category === c.value ? 'border-rose-400 bg-rose-50' : 'border-gray-200 hover:border-gray-300']">
              <input v-model="form.category" type="radio" name="emergency-category" :value="c.value" class="mt-0.5 accent-rose-600" />
              <span><span class="font-semibold text-gray-900">{{ c.label }}</span><br><span class="text-gray-500">{{ c.hint }}</span></span>
            </label>
          </div>
        </div>
        <div class="flex flex-col gap-1 sm:col-span-2">
          <label class="text-sm font-medium text-gray-700">Alasan Darurat <span class="text-red-500">*</span></label>
          <textarea v-model="form.reason" rows="2" maxlength="1000" class="fld"
            placeholder="Apa yang berhenti bila tidak dibeli sekarang? mis. genset mati, dapur tidak bisa memasak" />
          <p class="text-[11px]" :class="reasonShort ? 'text-amber-700' : 'text-gray-400'">Minimal {{ EMERGENCY_REASON_MIN }} huruf.</p>
        </div>
      </div>

      <div>
        <p class="mb-1.5 text-sm font-medium text-gray-700">{{ requestType === 'jasa' ? 'Jasa / Barang yang Dibeli' : 'Barang yang Dibeli' }} <span class="text-red-500">*</span></p>
        <div class="space-y-2">
          <div v-for="(it, i) in form.items" :key="i"
            class="grid grid-cols-2 gap-1.5 rounded-lg border border-gray-200 bg-white p-2 sm:grid-cols-12 sm:items-center">
            <input v-model="it.name" class="fld col-span-2 sm:col-span-5" :placeholder="requestType === 'jasa' ? 'mis. Servis genset + ganti relay' : 'mis. Steam valve mesin kopi'" />
            <input v-model.number="it.qty" type="number" min="1" class="fld sm:col-span-1" placeholder="Qty" aria-label="Jumlah" />
            <input v-model="it.unit" class="fld sm:col-span-2" placeholder="Satuan" aria-label="Satuan" />
            <RupiahInput v-model="it.price" class="col-span-2 sm:col-span-3" placeholder="Harga satuan" />
            <button type="button" class="col-span-2 justify-self-end rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-30 sm:col-span-1"
              :disabled="form.items.length <= 1" aria-label="Hapus baris" @click="form.items.splice(i, 1)">
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/></svg>
            </button>
          </div>
        </div>
        <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
          <button type="button" class="text-xs font-medium text-rose-700 hover:text-rose-900" @click="addLine">+ Tambah baris</button>
          <p class="text-sm font-semibold" :class="overLimit ? 'text-red-700' : 'text-gray-800'">
            Total kas: {{ formatRupiah(total) }}
            <span class="font-normal text-gray-500">/ batas {{ formatRupiah(maxAmount) }}</span>
          </p>
        </div>
        <p v-if="overLimit" class="mt-1 text-xs text-red-700">
          Melebihi batas Pembelian Darurat — buat pengajuan biasa bertingkat Urgent supaya disetujui dulu.
        </p>
        <p class="mt-1 text-[11px] text-gray-500">Isi harga yang sudah dikonfirmasi ke toko/teknisi: jumlah inilah yang diserahkan kasir.</p>
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <AppInput v-model="form.recipient_name" label="Staf Penerima Kas *" placeholder="Nama staf yang berbelanja" />
        <AppInput v-model="form.shop_name" label="Toko / Teknisi (opsional)" placeholder="mis. Toko Teknik Jaya" />
        <AppInput class="sm:col-span-2" v-model="form.notes" label="Catatan (opsional)" placeholder="Keterangan tambahan" />
      </div>
    </div>
    <template #footer>
      <AppButton variant="secondary" :disabled="saving" @click="close">Batal</AppButton>
      <AppButton variant="danger" :loading="saving" :disabled="!canSubmit" @click="submit">Catat &amp; Kirim ke Kasir</AppButton>
    </template>
  </AppModal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { useToastStore } from '@/stores/toast.js'
import { formatRupiah } from '@/utils/format.js'
import { EMERGENCY_CATEGORIES, EMERGENCY_REASON_MIN, reasonLength } from '@/utils/procurement.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppInput from '@/components/ui/AppInput.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const props = defineProps({
  modelValue: { type: Boolean, required: true },
  requestType: { type: String, default: 'barang' }, // barang | jasa
  // Unit kerja dalam cakupan (dari /my-work-units); hanya unit ber-outlet.
  workUnits: { type: Array, default: () => [] },
  defaultWorkUnitId: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue', 'created'])

const toast = useToastStore()
const typeLabel = computed(() => props.requestType === 'jasa' ? 'Jasa' : 'Barang')
const blankLine = () => ({ name: '', qty: 1, unit: props.requestType === 'jasa' ? 'kali' : 'pcs', price: 0 })
const blankForm = () => ({ work_unit_id: '', category: '', reason: '', shop_name: '', recipient_name: '', notes: '', items: [blankLine()] })
const form = ref(blankForm())
const error = ref('')
const saving = ref(false)
const ctx = ref(null)
const ctxLoading = ref(false)
const ctxError = ref('')
let ctxSeq = 0

const unitOptions = computed(() => props.workUnits.map(u => ({
  id: u.id,
  name: u.outlet_name && u.outlet_name !== u.name ? `${u.name} — ${u.outlet_name}` : u.name,
})))
const maxAmount = computed(() => Number(ctx.value?.max_amount) || 500000)
const total = computed(() => form.value.items.reduce((s, it) => s + (Number(it.qty) || 0) * Math.round(Number(it.price) || 0), 0))
const overLimit = computed(() => total.value > maxAmount.value)
const overCap = computed(() => ctx.value?.remaining != null && total.value > Number(ctx.value.remaining))
const reasonShort = computed(() => form.value.reason.trim() !== '' && reasonLength(form.value.reason) < EMERGENCY_REASON_MIN)
const canSubmit = computed(() => !saving.value && !!form.value.work_unit_id && !!ctx.value && !overLimit.value && total.value > 0)

watch(() => props.modelValue, open => {
  if (!open) return
  form.value = blankForm()
  error.value = ''
  ctx.value = null
  ctxError.value = ''
  const units = props.workUnits
  if (units.length === 1) form.value.work_unit_id = units[0].id
  else if (props.defaultWorkUnitId && units.some(u => u.id === props.defaultWorkUnitId)) form.value.work_unit_id = props.defaultWorkUnitId
  if (form.value.work_unit_id) loadContext()
}, { immediate: true })

watch(() => form.value.work_unit_id, (v, old) => { if (v && v !== old && props.modelValue) loadContext() })

async function loadContext() {
  const seq = ++ctxSeq
  ctxLoading.value = true
  ctxError.value = ''
  ctx.value = null
  try {
    const res = await purchaseApi.emergencyContext(form.value.work_unit_id)
    if (seq !== ctxSeq) return
    // apiClient membuka amplop {success,data} respons non-paginated.
    ctx.value = res?.outlet_id !== undefined ? res : (res?.data ?? null)
  } catch (err) {
    if (seq === ctxSeq) ctxError.value = err?.message || 'Gagal memeriksa outlet.'
  } finally {
    if (seq === ctxSeq) ctxLoading.value = false
  }
}

function addLine() { form.value.items.push(blankLine()) }

function close() {
  if (saving.value) return
  emit('update:modelValue', false)
}

async function submit() {
  error.value = ''
  const f = form.value
  const items = f.items.filter(it => String(it.name || '').trim())
  if (!f.work_unit_id) { error.value = 'Pilih unit kerja outlet.'; return }
  if (!f.category) { error.value = 'Pilih kategori darurat.'; return }
  if (reasonLength(f.reason) < EMERGENCY_REASON_MIN) { error.value = `Jelaskan alasan daruratnya (minimal ${EMERGENCY_REASON_MIN} huruf).`; return }
  if (!items.length) { error.value = 'Isi minimal satu barang/jasa.'; return }
  if (items.some(it => !(Number(it.qty) > 0) || !(Number(it.price) > 0))) { error.value = 'Setiap baris wajib berjumlah dan berharga lebih dari 0.'; return }
  if (!f.recipient_name.trim()) { error.value = 'Isi nama staf yang menerima kas.'; return }
  if (overLimit.value) { error.value = `Total melebihi batas ${formatRupiah(maxAmount.value)}.`; return }
  if (overCap.value) { error.value = `Total melebihi sisa plafon bulan ini (${formatRupiah(ctx.value.remaining)}).`; return }
  saving.value = true
  try {
    const pr = await purchaseApi.createEmergency({
      work_unit_id: f.work_unit_id,
      request_type: props.requestType,
      category: f.category,
      reason: f.reason.trim(),
      shop_name: f.shop_name.trim(),
      recipient_name: f.recipient_name.trim(),
      notes: f.notes.trim(),
      items: items.map(it => ({ name: it.name.trim(), qty: Number(it.qty), unit: String(it.unit || '').trim(), price: Math.round(Number(it.price)) })),
    })
    toast.success(`Pembelian Darurat ${pr?.request_number || ''} dicatat — tugas kas dikirim ke tablet kasir.`)
    emit('created', pr)
    emit('update:modelValue', false)
  } catch (err) {
    error.value = err?.message || 'Gagal mencatat Pembelian Darurat.'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.fld {
  width: 100%;
  min-width: 0;
  border-radius: 0.5rem;
  border: 1px solid rgb(209 213 219);
  background: #fff;
  padding: 0.4rem 0.6rem;
  font-size: 0.8125rem;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.04);
}
.fld:focus {
  outline: none;
  border-color: rgb(16 185 129);
  box-shadow: 0 0 0 2px rgb(16 185 129 / 0.25);
}
</style>
