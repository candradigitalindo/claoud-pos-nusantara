<!--
  EmergencySettingsDialog.vue — batas Pembelian Darurat (penyetuju barang/jasa):
  batas per pembelian (bawaan Rp500.000, keputusan owner 19 Nov 2026) dan
  plafon bulanan per outlet (0 = tanpa plafon).
-->
<template>
  <AppModal :model-value="modelValue" title="Batas Pembelian Darurat" size="md" @update:model-value="close">
    <div class="space-y-3">
      <p class="text-xs text-gray-600">
        Pembelian Darurat dibeli dulu dari kas laci outlet dan disahkan sesudahnya. Batas ini dicek server setiap kali dicatat.
      </p>
      <AppAlert type="error" :message="error" />
      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700">Batas per Pembelian</label>
        <RupiahInput v-model="form.max_amount" placeholder="500.000" />
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700">Plafon Bulanan per Outlet</label>
        <RupiahInput v-model="form.monthly_cap" placeholder="0" />
        <p class="mt-1 text-[11px] text-gray-500">0 = tanpa plafon. Bila diisi, minimal sebesar batas per pembelian; Pembelian Darurat yang dibatalkan tidak dihitung.</p>
      </div>
    </div>
    <template #footer>
      <AppButton variant="secondary" :disabled="saving" @click="close">Batal</AppButton>
      <AppButton :loading="saving" :disabled="loading" @click="save">Simpan</AppButton>
    </template>
  </AppModal>
</template>

<script setup>
import { ref, watch } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { useToastStore } from '@/stores/toast.js'
import AppModal from '@/components/ui/AppModal.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'

const props = defineProps({ modelValue: { type: Boolean, required: true } })
const emit = defineEmits(['update:modelValue', 'saved'])
const toast = useToastStore()
const form = ref({ max_amount: 500000, monthly_cap: 0 })
const error = ref('')
const saving = ref(false)
const loading = ref(false)

watch(() => props.modelValue, async open => {
  if (!open) return
  error.value = ''
  loading.value = true
  try {
    const s = await purchaseApi.getEmergencySettings()
    form.value = { max_amount: Number(s?.max_amount) || 500000, monthly_cap: Number(s?.monthly_cap) || 0 }
  } catch (err) {
    error.value = err?.message || 'Gagal memuat batas.'
  } finally {
    loading.value = false
  }
}, { immediate: true })

function close() {
  if (!saving.value) emit('update:modelValue', false)
}

async function save() {
  error.value = ''
  const max = Math.round(Number(form.value.max_amount) || 0)
  const cap = Math.round(Number(form.value.monthly_cap) || 0)
  if (max < 1) { error.value = 'Batas per pembelian wajib diisi.'; return }
  if (cap > 0 && cap < max) { error.value = 'Plafon bulanan minimal sebesar batas per pembelian (atau 0 = tanpa plafon).'; return }
  saving.value = true
  try {
    const s = await purchaseApi.updateEmergencySettings({ max_amount: max, monthly_cap: cap })
    toast.success('Batas Pembelian Darurat tersimpan.')
    emit('saved', s)
    emit('update:modelValue', false)
  } catch (err) {
    error.value = err?.message || 'Gagal menyimpan.'
  } finally {
    saving.value = false
  }
}
</script>
