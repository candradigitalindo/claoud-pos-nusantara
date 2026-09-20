<!--
  ProjectRabEditor.vue — editor susunan RAB (Rencana Anggaran Biaya) projek.

  RAB disusun per bagian pekerjaan → baris uraian dengan volume × harga satuan.
  Komponen ini hanya mengedit; menyimpan/menetapkan dilakukan induknya lewat
  projectsApi.saveRab / setRab.

  v-model: array datar baris { id, section, name, kind, unit, qty, unit_price, notes }.
  Urutan array = urutan tampil (seq). Baris tanpa id = baris baru.
-->
<template>
  <div class="space-y-3">
    <div v-for="(g, gi) in groups" :key="g.key" class="rounded-xl border border-gray-200 bg-white">
      <!-- Judul bagian -->
      <div class="flex items-center gap-2 rounded-t-xl border-b border-gray-100 bg-gray-50 px-3 py-2">
        <span class="sec-no">{{ romawi(gi) }}</span>
        <input v-model.trim="g.section" :disabled="disabled" class="input-sm min-w-0 flex-1 font-semibold"
          placeholder="Nama bagian pekerjaan — mis. Pekerjaan Persiapan, Pekerjaan Sipil, Mekanikal & Elektrikal" />
        <span class="hidden shrink-0 text-xs font-semibold text-gray-700 sm:inline">{{ formatRupiah(groupTotal(g)) }}</span>
        <button v-if="!disabled" type="button" class="act-del shrink-0" :disabled="groups.length <= 1" title="Hapus bagian" @click="removeGroup(gi)">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a1 1 0 011-1h4a1 1 0 011 1v2"/></svg>
        </button>
      </div>

      <!-- Desktop: tabel -->
      <div class="hidden overflow-x-auto sm:block">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-left text-[11px] uppercase tracking-wide text-gray-500">
              <th class="px-3 py-2 w-8">#</th>
              <th class="px-2 py-2">Uraian pekerjaan / barang</th>
              <th class="px-2 py-2 w-24">Jenis</th>
              <th class="px-2 py-2 w-24 text-right">Volume</th>
              <th class="px-2 py-2 w-20">Satuan</th>
              <th class="px-2 py-2 w-36 text-right">Harga Satuan</th>
              <th class="px-2 py-2 w-32 text-right">Jumlah</th>
              <th class="px-2 py-2 w-8"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, ri) in g.rows" :key="r.key" class="border-t border-gray-100 align-top">
              <td class="px-3 py-1.5 text-gray-400">{{ ri + 1 }}</td>
              <td class="px-2 py-1.5">
                <input v-model.trim="r.name" :disabled="disabled" class="input-sm w-full" placeholder="mis. Pasang keramik lantai 40×40" />
                <input v-model.trim="r.notes" :disabled="disabled" class="input-sm mt-1 w-full border-dashed text-[11px]" placeholder="Spesifikasi / catatan (opsional)" />
              </td>
              <td class="px-2 py-1.5">
                <select v-model="r.kind" :disabled="disabled" class="input-sm w-full">
                  <option v-for="k in KINDS" :key="k.value" :value="k.value">{{ k.label }}</option>
                </select>
              </td>
              <td class="px-2 py-1.5"><input v-model.number="r.qty" :disabled="disabled" type="number" min="0" step="any" class="input-sm w-full text-right" /></td>
              <td class="px-2 py-1.5"><input v-model.trim="r.unit" :disabled="disabled" class="input-sm w-full" placeholder="m², sak, ls" /></td>
              <td class="px-2 py-1.5"><RupiahInput v-model="r.unit_price" :disabled="disabled" placeholder="Rp 0" /></td>
              <td class="px-2 py-1.5 text-right font-semibold text-gray-800">{{ formatRupiah(rowTotal(r)) }}</td>
              <td class="px-2 py-1.5">
                <button v-if="!disabled" type="button" class="act-del" title="Hapus baris" @click="removeRow(g, ri)">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Ponsel: kartu per baris -->
      <ul class="divide-y divide-gray-100 sm:hidden">
        <li v-for="(r, ri) in g.rows" :key="r.key" class="space-y-1.5 p-3">
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-400">{{ romawi(gi) }}.{{ ri + 1 }}</span>
            <input v-model.trim="r.name" :disabled="disabled" class="input-sm min-w-0 flex-1" placeholder="Uraian pekerjaan / barang" />
            <button v-if="!disabled" type="button" class="act-del shrink-0" title="Hapus baris" @click="removeRow(g, ri)">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>
            </button>
          </div>
          <div class="grid grid-cols-2 gap-1.5">
            <label class="fld"><span>Jenis</span>
              <select v-model="r.kind" :disabled="disabled" class="input-sm w-full">
                <option v-for="k in KINDS" :key="k.value" :value="k.value">{{ k.label }}</option>
              </select>
            </label>
            <label class="fld"><span>Satuan</span><input v-model.trim="r.unit" :disabled="disabled" class="input-sm w-full" placeholder="m², sak, ls" /></label>
            <label class="fld"><span>Volume</span><input v-model.number="r.qty" :disabled="disabled" type="number" min="0" step="any" class="input-sm w-full text-right" /></label>
            <label class="fld"><span>Harga satuan</span><RupiahInput v-model="r.unit_price" :disabled="disabled" placeholder="Rp 0" /></label>
          </div>
          <input v-model.trim="r.notes" :disabled="disabled" class="input-sm w-full border-dashed text-[11px]" placeholder="Spesifikasi / catatan (opsional)" />
          <p class="text-right text-xs">Jumlah: <b class="text-gray-800">{{ formatRupiah(rowTotal(r)) }}</b></p>
        </li>
      </ul>

      <div v-if="!disabled" class="flex items-center justify-between gap-2 border-t border-gray-100 px-3 py-2">
        <button type="button" class="text-xs font-semibold text-emerald-700 hover:text-emerald-900" @click="addRow(g)">+ Tambah baris</button>
        <span class="text-xs text-gray-500 sm:hidden">Subtotal <b class="text-gray-700">{{ formatRupiah(groupTotal(g)) }}</b></span>
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-2">
      <button v-if="!disabled" type="button" class="rounded-lg border border-dashed border-emerald-300 px-3 py-1.5 text-xs font-semibold text-emerald-700 hover:bg-emerald-50" @click="addGroup">+ Tambah bagian pekerjaan</button>
      <p class="ml-auto text-sm">Total RAB: <b class="text-gray-900">{{ formatRupiah(total) }}</b> <span class="text-xs text-gray-400">· {{ rowCount }} baris</span></p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { formatRupiah } from '@/utils/format.js'
import RupiahInput from '@/components/ui/RupiahInput.vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  disabled:   { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const KINDS = [
  { value: 'barang', label: 'Barang' },
  { value: 'jasa',   label: 'Jasa' },
  { value: 'umum',   label: 'Umum' },
]

let keySeq = 0
const nextKey = () => `k${++keySeq}`
const blankRow = (kind = 'barang') => ({ key: nextKey(), id: '', name: '', kind, unit: '', qty: 1, unit_price: 0, notes: '' })

const groups = ref([])

// Bangun kelompok dari array datar: urutan kemunculan bagian dipertahankan.
function fromFlat(rows) {
  const out = []
  const byName = new Map()
  for (const r of rows || []) {
    const sec = r.section || ''
    let g = byName.get(sec)
    if (!g) { g = { key: nextKey(), section: sec, rows: [] }; byName.set(sec, g); out.push(g) }
    g.rows.push({ key: nextKey(), id: r.id || '', name: r.name || '', kind: r.kind || 'barang', unit: r.unit || '', qty: Number(r.qty) || 0, unit_price: Number(r.unit_price) || 0, notes: r.notes || '' })
  }
  if (!out.length) out.push({ key: nextKey(), section: '', rows: [blankRow()] })
  return out
}
function toFlat() {
  const flat = []
  for (const g of groups.value) {
    for (const r of g.rows) {
      flat.push({ id: r.id, section: g.section, name: r.name, kind: r.kind, unit: r.unit, qty: r.qty, unit_price: r.unit_price, notes: r.notes })
    }
  }
  return flat
}

let lastEmitted = null
watch(() => props.modelValue, (v) => { if (v !== lastEmitted) groups.value = fromFlat(v) }, { immediate: true })
watch(groups, () => { lastEmitted = toFlat(); emit('update:modelValue', lastEmitted) }, { deep: true })

const rowTotal = (r) => (Number(r.qty) || 0) * (Number(r.unit_price) || 0)
const groupTotal = (g) => g.rows.reduce((s, r) => s + rowTotal(r), 0)
const total = computed(() => groups.value.reduce((s, g) => s + groupTotal(g), 0))
const rowCount = computed(() => groups.value.reduce((s, g) => s + g.rows.length, 0))

function addGroup() { groups.value.push({ key: nextKey(), section: '', rows: [blankRow()] }) }
function removeGroup(gi) { if (groups.value.length > 1) groups.value.splice(gi, 1) }
function addRow(g) { const last = g.rows[g.rows.length - 1]; g.rows.push(blankRow(last?.kind || 'barang')) }
function removeRow(g, ri) { g.rows.splice(ri, 1); if (!g.rows.length) g.rows.push(blankRow()) }

const ROMAWI = ['I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'VIII', 'IX', 'X', 'XI', 'XII', 'XIII', 'XIV', 'XV']
const romawi = (i) => ROMAWI[i] || String(i + 1)
</script>

<style scoped>
.input-sm { padding: .35rem .5rem; border-radius: .45rem; font-size: .78rem; border: 1px solid #e5e7eb; background: #fff; color: #111827; outline: none; }
.input-sm:focus { border-color: rgba(5,150,105,.5); box-shadow: 0 0 0 3px rgba(5,150,105,.12); }
.input-sm:disabled { background: #f9fafb; color: #6b7280; }
.sec-no { flex-shrink: 0; min-width: 1.6rem; padding: 0 .35rem; height: 1.5rem; border-radius: .4rem; background: #ecfdf5; color: #047857; font-size: .7rem; font-weight: 800; display: inline-flex; align-items: center; justify-content: center; }
.fld { display: flex; flex-direction: column; gap: .15rem; min-width: 0; }
.fld > span { font-size: .66rem; font-weight: 600; color: #6b7280; }
.act-del { width: 26px; height: 26px; border-radius: .4rem; border: none; cursor: pointer; display: inline-flex; align-items: center; justify-content: center; background: rgba(220,38,38,.07); color: #dc2626; }
.act-del:hover:not(:disabled) { background: rgba(220,38,38,.15); }
.act-del:disabled { opacity: .35; cursor: not-allowed; }
</style>
