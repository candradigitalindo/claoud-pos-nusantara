<!--
  PurchaseStrukSection.vue — struk pembelian sebuah pengajuan.

  Satu pengajuan bisa dibeli dalam beberapa transaksi di vendornya, tiap
  transaksi berstruk sendiri (nomor, tanggal, total, foto), dan Keuangan
  membayar per struk — boleh dicicil. Bagian pembelian atau petugas logistik
  mencatat struknya di sini; struk yang belum dibayar boleh dihapus.
-->
<template>
  <div class="rounded-xl border border-gray-200 bg-white p-3 sm:p-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="flex items-center gap-1.5 text-sm font-semibold text-gray-800">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 2v20l2-1 2 1 2-1 2 1 2-1 2 1 2-1 2 1V2l-2 1-2-1-2 1-2-1-2 1-2-1-2 1Z"/><path d="M16 8h-6"/><path d="M16 12h-6"/><path d="M13 16h-3"/></svg>
        Struk Pembelian <span class="font-normal text-gray-400">({{ struks.length }})</span>
      </p>
      <button v-if="canManage && !showForm && !fullyPaid" type="button" @click="openForm"
        class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-blue-700">
        + Tambah Struk
      </button>
    </div>

    <p v-if="struks.length" class="mt-1 text-[11px]" :class="mismatch ? 'text-amber-700' : 'text-gray-500'">
      Total struk {{ formatRupiah(totalStruk) }} · harga final {{ formatRupiah(ownFinal) }}<template v-if="mismatch"> — belum sama; periksa harga final atau struknya.</template>
    </p>

    <ul v-if="struks.length" class="mt-2 divide-y divide-gray-100">
      <li v-for="s in struks" :key="s.id" class="flex flex-wrap items-start justify-between gap-2 py-2 text-xs">
        <div class="min-w-0">
          <p class="break-words font-mono font-medium text-gray-900">{{ s.struk_number }}</p>
          <p class="text-gray-500">{{ formatDate(s.struk_date) }} · {{ formatRupiah(s.total) }}<template v-if="s.created_by"> · dicatat {{ s.created_by }}</template></p>
          <p v-if="s.notes" class="break-words text-gray-400">{{ s.notes }}</p>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <span v-if="s.remaining <= 0" class="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700">Lunas</span>
          <span v-else class="text-[11px] text-amber-700">Sisa {{ formatRupiah(s.remaining) }}<template v-if="s.paid_amount > 0"> · dibayar {{ formatRupiah(s.paid_amount) }}</template></span>
          <AppActionButton icon="photo" label="Foto" v-if="safeUrl(s.photo_url)" :href="safeUrl(s.photo_url)" target="_blank" rel="noopener noreferrer" />
          <AppActionButton icon="delete" label="Hapus" v-if="canManage && !(s.paid_amount > 0)" type="button" :disabled="busy" @click="remove(s)" />
        </div>
      </li>
    </ul>
    <p v-else-if="!showForm" class="mt-1 text-xs text-gray-400">
      Belum ada struk. Catat struk tiap transaksi di vendor supaya Keuangan bisa membayar per struk.
    </p>

    <div v-if="showForm" class="mt-3 space-y-3 rounded-lg border border-blue-100 bg-blue-50/40 p-3">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <AppInput v-model="form.struk_number" label="Nomor Struk" placeholder="mis. 000123" />
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700">Tanggal Struk</label>
          <!-- Kelas eksplisit seperti AppInput: .input-sm milik style scoped
               halaman induk tidak berlaku di komponen ini (tampil tanpa bingkai). -->
          <input v-model="form.struk_date" type="date" :max="today"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-emerald-500/30 focus:border-emerald-500" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700">Total Struk</label>
          <RupiahInput v-model="form.total" placeholder="0" />
        </div>
      </div>
      <PhotoCapture v-model="form.photo_url" label="Foto Struk" hint="Foto struk/nota dari vendor — bukti transaksi yang dibayar Keuangan." />
      <AppInput v-model="form.notes" label="Catatan (opsional)" placeholder="mis. pembelian pagi, 2 kali angkut" />
      <div class="flex justify-end gap-2">
        <AppButton variant="secondary" :disabled="busy" @click="showForm = false">Batal</AppButton>
        <AppButton :loading="busy" @click="save">Simpan Struk</AppButton>
      </div>
    </div>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, computed } from 'vue'
import { purchaseApi } from '@/api/purchase.js'
import { formatRupiah, formatDate, todayDateString } from '@/utils/format.js'
import { safeUrl } from '@/utils/safeUrl.js'
import { useToastStore } from '@/stores/toast.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppInput from '@/components/ui/AppInput.vue'
import RupiahInput from '@/components/ui/RupiahInput.vue'
import PhotoCapture from '@/components/PhotoCapture.vue'

const props = defineProps({
  doc: { type: Object, required: true },
  // Bagian pembelian / petugas logistik jenis pengajuan ini.
  canManage: { type: Boolean, default: false },
})
const emit = defineEmits(['changed'])

const toast = useToastStore()
const busy = ref(false)
const showForm = ref(false)
// "Hari ini" menurut zona waktu aplikasi, dihitung ulang tiap form dibuka.
// Dulu toISOString() (UTC): pukul 00.00–07.00 WIB tanggal bawaan & batas
// maksimalnya masih kemarin, jadi struk hari itu tidak bisa dipilih.
const today = ref(todayDateString())
const blankForm = () => ({ struk_number: '', struk_date: today.value, total: 0, photo_url: '', notes: '' })
const form = ref(blankForm())

const struks = computed(() => props.doc?.struks || [])
const totalStruk = computed(() => struks.value.reduce((s, x) => s + (x.total || 0), 0))
// Struk milik dokumen ini sendiri; total induk ikut menjumlah pecahannya.
const ownFinal = computed(() => {
  const d = props.doc || {}
  const children = (d.children || []).reduce((s, c) => s + (c.total_final || 0), 0)
  return Math.max(0, (d.total_final || 0) - children)
})
const mismatch = computed(() => ownFinal.value > 0 && Math.abs(totalStruk.value - ownFinal.value) >= 1)
// Tagihan lunas: server menolak struk baru (tidak akan pernah bisa dibayar).
const fullyPaid = computed(() => ownFinal.value > 0 && (props.doc?.paid_amount || 0) >= ownFinal.value - 0.005)

function openForm() {
  today.value = todayDateString()
  form.value = blankForm()
  showForm.value = true
}

async function save() {
  const f = form.value
  if (!f.struk_number.trim()) { toast.error('Nomor struk wajib diisi.'); return }
  if (!f.struk_date) { toast.error('Tanggal struk wajib diisi.'); return }
  // Batas max di kalender bisa dilewati dengan mengetik tanggal; struk
  // tidak mungkin bertanggal sesudah hari ini.
  if (f.struk_date > todayDateString()) { toast.error('Tanggal struk tidak boleh melewati hari ini.'); return }
  if (!(Number(f.total) > 0)) { toast.error('Total struk harus lebih dari 0.'); return }
  if (!f.photo_url) { toast.error('Foto struk wajib diunggah.'); return }
  busy.value = true
  try {
    await purchaseApi.addStruk(props.doc.id, { ...f, struk_number: f.struk_number.trim(), total: Number(f.total) })
    toast.success('Struk dicatat.')
    showForm.value = false
    emit('changed')
  } catch (err) {
    toast.error(err?.message || 'Gagal mencatat struk.')
  } finally {
    busy.value = false
  }
}

async function remove(s) {
  if (!window.confirm(`Hapus struk ${s.struk_number}?`)) return
  busy.value = true
  try {
    await purchaseApi.deleteStruk(props.doc.id, s.id)
    toast.success('Struk dihapus.')
    emit('changed')
  } catch (err) {
    toast.error(err?.message || 'Gagal menghapus struk.')
  } finally {
    busy.value = false
  }
}
</script>
