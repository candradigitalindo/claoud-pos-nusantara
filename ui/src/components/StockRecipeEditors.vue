<template>
    <!-- Pola Pemotongan: satu bahan → beberapa hasil; satu bahan boleh punya beberapa pola -->
    <AppModal v-model="showBreakdownModal" title="Atur Pemotongan" size="2xl">
      <p v-if="readOnly" class="mb-3 text-sm text-amber-800">Hanya baca. Perubahan pola dikelola F &amp; B dengan akses ubah resep untuk semua outlet.</p>
      <fieldset :disabled="readOnly || bdLoading || !!bdLoadError" class="space-y-4 min-w-0">
        <div class="p-3 bg-pink-50 rounded-lg border border-pink-100">
          <div class="text-[10px] uppercase font-bold text-pink-400">Bahan yang Dipotong / Diolah</div>
          <div class="text-lg font-bold text-pink-900 break-words">{{ bdTarget?.name }}</div>
          <div class="text-xs text-pink-700">
            Hasil standar diisi per 1 {{ bdDistLabel }}<span v-if="bdTarget && bdTarget.dist_unit !== bdTarget.base_unit"> (= {{ bdTarget.dist_ratio }} {{ bdTarget.base_unit }})</span>, dalam satuan dasar tiap hasil.
          </div>
        </div>
        <p class="text-xs text-gray-500">
          Satu bahan bisa punya beberapa pola, mis. ayam utuh → paha, dada, sayap ("Per Bagian") atau 8 potong ("Potong 8");
          daging sapi → has dalam, has luar, tetelan; ikan utuh → fillet, kepala; beras → nasi (hasil boleh lebih banyak dari bahan).
          Saat pemotongan dicatat, stok dan HPP mengikuti hasil aktual; biaya bahan dibagi ke hasil pola yang dipakai sebanding
          jumlah × bobot ÷ hasil standar. Hasil sebuah pola boleh dipotong lagi lewat polanya sendiri.
        </p>

        <div v-if="bdLoading" class="py-6 text-center text-sm text-gray-400">Memuat…</div>
        <AppAlert v-else-if="bdLoadError" type="error" :message="bdLoadError" />
        <template v-else>
          <div v-for="(p, pi) in bdPatterns" :key="p.key"
            :class="['rounded-xl border p-3 space-y-3', p.is_active ? 'border-gray-200' : 'border-dashed border-gray-300 bg-gray-50']">
            <div class="flex items-end gap-2">
              <div class="flex-1 min-w-0">
                <label class="block text-xs text-gray-500 mb-1">Nama pola</label>
                <input aria-label="Nama pola" v-model="p.name" type="text" maxlength="100" placeholder="mis. Per Bagian"
                  class="w-full min-w-0 text-sm font-semibold border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
              </div>
              <AppActionButton v-if="!readOnly" icon="delete" label="Hapus pola" @click="bdPatterns.splice(pi, 1)" />
            </div>
            <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
              <label class="flex items-center gap-2 text-sm text-gray-600 cursor-pointer select-none">
                <input type="checkbox" v-model="p.is_active" class="rounded text-emerald-600" />
                Aktif
              </label>
              <input aria-label="Catatan pola" v-model="p.notes" type="text" placeholder="Catatan pola (opsional)"
                class="flex-1 min-w-0 text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
            </div>

            <div class="modal-form-grid">
              <div v-for="(r, idx) in p.outputs" :key="idx" class="rounded-lg border border-gray-100 bg-white p-2.5 space-y-2">
                <div class="flex items-end gap-2">
                  <div class="flex-1 min-w-0">
                    <label class="block text-xs text-gray-500 mb-1">Item hasil</label>
                    <SearchSelect :disabled="readOnly || bdLoading || !!bdLoadError"
                      aria-label="Item hasil"
                      v-model="r.output_item_id"
                      :options="bdOutputOptions(p, idx)"
                      placeholder="Pilih item hasil…"
                      searchPlaceholder="Cari…"
                      valueKey="value"
                      labelKey="label"
                    />
                  </div>
                  <AppActionButton v-if="!readOnly" icon="delete" label="Hapus baris" @click="p.outputs.splice(idx, 1)" />
                </div>
                <div class="grid grid-cols-2 gap-2">
                  <div class="min-w-0">
                    <label class="block text-xs text-gray-500 mb-1">Hasil standar per 1 {{ bdDistLabel }}</label>
                    <div class="flex items-center gap-1.5">
                      <input aria-label="Hasil standar" v-model.number="r.qty_per_input" type="number" step="any" min="0" placeholder="0"
                        class="w-full min-w-0 text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
                      <span class="text-xs text-gray-500 shrink-0">{{ bdUnitOf(r) }}</span>
                    </div>
                  </div>
                  <div class="min-w-0">
                    <label class="block text-xs text-gray-500 mb-1">Bobot biaya</label>
                    <input aria-label="Bobot biaya" v-model.number="r.cost_weight" type="number" step="any" min="0" placeholder="1"
                      class="w-full min-w-0 text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-emerald-400" />
                  </div>
                </div>
                <p class="text-[11px] text-gray-400">Porsi biaya pada hasil standar: <b class="text-gray-600">{{ bdSharePct(p, r) }}</b></p>
              </div>
              <div v-if="p.outputs.length === 0" class="modal-span-full text-sm text-gray-400 py-3 text-center border border-dashed border-gray-200 rounded-lg">
                Pola ini belum punya hasil.
              </div>
              <button v-if="!readOnly" @click="addBdLine(p)" class="modal-span-full justify-self-start text-xs font-bold text-pink-600 hover:text-pink-800">+ Tambah Hasil</button>
            </div>
          </div>

          <div v-if="bdPatterns.length === 0" class="text-sm text-gray-400 py-4 text-center border border-dashed border-gray-200 rounded-lg">
            Belum ada pola.<span v-if="!readOnly"> Klik "+ Tambah Pola" — simpan tanpa pola untuk menghapus semua pola bahan ini.</span>
          </div>
          <button v-if="!readOnly" @click="addBdPattern"
            class="w-full rounded-lg border border-dashed border-pink-300 py-2 text-sm font-semibold text-pink-600 hover:bg-pink-50">
            + Tambah Pola
          </button>
          <p class="text-[11px] text-gray-400">
            Pola nonaktif tidak ditawarkan di form Pemotongan. Riwayat pemotongan tetap menyimpan nama pola walau pola diganti namanya atau dihapus.
          </p>
        </template>
      </fieldset>
      <template #footer>
        <AppButton variant="secondary" @click="showBreakdownModal = false">Batal</AppButton>
        <AppButton v-if="!readOnly" :loading="bdSaving" :disabled="bdLoading || !!bdLoadError" @click="submitBreakdownTemplate">Simpan Pola</AppButton>
      </template>
    </AppModal>

    <!-- SFG Recipe Modal -->
    <AppModal v-model="showRecipeModal" title="Resep Bahan Setengah Jadi" size="xl">
      <p v-if="readOnly" class="mb-3 text-sm text-amber-800">Hanya baca. Perubahan komposisi dikelola F &amp; B dengan akses ubah resep untuk semua outlet.</p>
      <p v-if="recipeLoading" class="mb-3 text-sm text-gray-500">Memuat resep…</p>
      <AppAlert type="error" :message="recipeLoadError" />
      <fieldset :disabled="readOnly || recipeLoading || !!recipeLoadError" class="space-y-4 min-w-0">
        <div class="p-3 bg-indigo-50 rounded-lg border border-indigo-100 flex items-center justify-between">
          <div>
            <div class="text-[10px] uppercase font-bold text-indigo-400">Bahan yang Dibuat</div>
            <div class="text-lg font-bold text-indigo-900">{{ recipeTarget?.name }}</div>
          </div>
          <div class="text-right">
            <div class="text-[10px] uppercase font-bold text-indigo-400">Satuan Produksi</div>
            <div class="text-lg font-bold text-indigo-900">1 {{ recipeTarget?.base_unit }}</div>
          </div>
        </div>

        <div class="p-3 bg-gray-50 rounded-xl border border-gray-100 flex flex-wrap gap-3 items-center justify-between">
          <label class="text-sm font-bold text-gray-700">Visibilitas Resep:</label>
          <div class="flex gap-2">
            <button
              v-for="v in [{v:'public', l:'Public (Semua Outlet)'}, {v:'secret', l:'Rahasia (Hanya Saya)'}]" :key="v.v"
              @click="recipeVisibility = v.v"
              :class="['px-3 py-1.5 rounded-lg text-xs font-bold transition-all border',
                recipeVisibility === v.v ? 'bg-indigo-600 text-white border-indigo-600' : 'bg-white text-gray-500 border-gray-200']"
            >
              {{ v.l }}
            </button>
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-xs font-bold text-gray-500 uppercase">Komponen Bahan Baku</label>
          <div class="border rounded-xl overflow-x-auto">
            <table class="min-w-full text-sm">
              <thead class="bg-gray-50 text-gray-500 uppercase text-[10px]">
                <tr>
                  <th class="px-3 py-2 text-left">Bahan Baku</th>
                  <th class="px-3 py-2 text-left w-32">Jumlah</th>
                  <th class="px-3 py-2 text-left w-20">Satuan</th>
                  <th class="px-3 py-2 w-10"></th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100">
                <tr v-for="(it, idx) in recipeItems" :key="idx">
                  <td class="px-3 py-2">
                    <SearchSelect :disabled="readOnly || recipeLoading || !!recipeLoadError"
                      aria-label="Bahan komponen"
                      v-model="it.item_id"
                      :options="stockItemOptions"
                      placeholder="Pilih bahan..."
                      searchPlaceholder="Cari..."
                      valueKey="value"
                      labelKey="label"
                      @change="onRecipeItemChange(idx)"
                    />
                  </td>
                  <td class="px-3 py-2">
                    <input aria-label="Jumlah komponen" v-model.number="it.qty_base" type="number" step="0.001" class="w-full text-sm border-gray-200 rounded-lg" placeholder="0" />
                  </td>
                  <td class="px-3 py-2 text-xs text-gray-500 font-medium">
                    {{ it.unit || '—' }}
                  </td>
                  <td class="px-3 py-2 text-center">
                    <AppActionButton v-if="!readOnly" icon="delete" label="Hapus komponen" @click="recipeItems.splice(idx, 1)" />
                  </td>
                </tr>
                <tr v-if="recipeItems.length === 0">
                  <td colspan="4" class="px-3 py-6 text-center text-gray-400 italic">Belum ada komponen resep.</td>
                </tr>
              </tbody>
            </table>
          </div>
          <button v-if="!readOnly" @click="addRecipeItem" class="text-xs font-bold text-indigo-600 hover:text-indigo-800">+ Tambah Komponen</button>
        </div>
      </fieldset>
      <template #footer>
        <AppButton variant="secondary" @click="showRecipeModal = false">Batal</AppButton>
        <AppButton v-if="!readOnly" :disabled="recipeLoading || !!recipeLoadError" :loading="savingRecipe" @click="submitRecipe">Simpan Resep</AppButton>
      </template>
    </AppModal>
</template>

<script setup>
import { computed, ref } from 'vue'
import { stockItemRecipesApi, stockBreakdownsApi } from '@/api/warehouse.js'
import { useToastStore } from '@/stores/toast.js'
import AppActionButton from '@/components/ui/AppActionButton.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import SearchSelect from '@/components/ui/SearchSelect.vue'

const props = defineProps({ items: { type: Array, default: () => [] }, canEdit: Boolean })
const readOnly = computed(() => !props.canEdit)
const toast = useToastStore()

// SFG Recipe State
const showRecipeModal = ref(false)
const recipeTarget = ref(null)
const recipeItems = ref([])
const recipeVisibility = ref('secret')
const savingRecipe = ref(false)
const allStockItems = computed(() => props.items.filter(i => i.is_active !== false))
const recipeLoading = ref(false)
const recipeLoadError = ref('')

const stockItemOptions = computed(() => {
  const options = allStockItems.value.filter(i => i.id !== recipeTarget.value?.id)
    .map(i => ({ value: i.id, label: `${i.name} (${i.code})` }))
  for (const item of recipeItems.value) {
    if (item.item_id && !options.some(o => o.value === item.item_id)) {
      options.push({ value: item.item_id, label: item.name || item.item_id })
    }
  }
  return options
})

async function openRecipe(row) {
  recipeTarget.value = row
  recipeItems.value = []
  recipeVisibility.value = 'secret'
  showRecipeModal.value = true

  recipeLoading.value = true
  recipeLoadError.value = ''

  try {
    // apiClient sudah meng-unwrap envelope {success,data} → responsnya array langsung.
    const data = await stockItemRecipesApi.get(row.id)
    const list = Array.isArray(data) ? data : (data?.data || [])
    recipeItems.value = list.map(r => ({
      item_id: r.child_item_id,
      name: `${r.child_name || r.child_item_id}${r.child_code ? ` (${r.child_code})` : ''}`,
      qty_base: r.qty_base,
      unit: r.unit,
      notes: r.notes || '',
    }))
    if (list.length > 0) {
      recipeVisibility.value = list[0].visibility || 'secret'
    }
  } catch (e) {
    recipeLoadError.value = e?.message || 'Gagal memuat resep'
  } finally {
    recipeLoading.value = false
  }
}

function addRecipeItem() {
  recipeItems.value.push({ item_id: '', qty_base: 1, unit: '', notes: '' })
}

function onRecipeItemChange(idx) {
  const it = recipeItems.value[idx]
  const found = allStockItems.value.find(i => i.id === it.item_id)
  if (found) {
    it.unit = found.base_unit
  }
}

async function submitRecipe() {
  if (readOnly.value || recipeLoading.value || recipeLoadError.value || savingRecipe.value) return
  if (recipeItems.value.some(it => !it.item_id || !Number.isFinite(Number(it.qty_base)) || !(Number(it.qty_base) > 0))) {
    toast.error('Pilih bahan dan isi jumlah positif pada setiap komponen')
    return
  }
  if (recipeItems.value.some(it => it.item_id === recipeTarget.value.id)) {
    toast.error('Bahan hasil tidak boleh menjadi komponen resepnya sendiri')
    return
  }
  if (new Set(recipeItems.value.map(it => it.item_id)).size !== recipeItems.value.length) {
    toast.error('Bahan yang sama cukup ditulis satu kali')
    return
  }
  const items = recipeItems.value.map(({ item_id, qty_base, unit, notes }) => ({
    item_id, qty_base: Number(qty_base), unit, notes: notes || '',
  }))
  savingRecipe.value = true
  try {
    await stockItemRecipesApi.save(recipeTarget.value.id, items, recipeVisibility.value)
    toast.success('Resep bahan baku disimpan')
    showRecipeModal.value = false
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan resep')
  } finally {
    savingRecipe.value = false
  }
}

// ── Pola Pemotongan (satu bahan → beberapa hasil) ─────────────
// Satu bahan boleh punya beberapa pola bernama (mis. ayam "Per Bagian" dan
// "Potong 8"). Definisi katalog global seperti item (T-20): hanya role
// bercakupan semua outlet dengan izin ubah resep — selain itu server 403.
const showBreakdownModal = ref(false)
const bdTarget = ref(null)
const bdPatterns = ref([])
const bdLoading = ref(false)
const bdLoadError = ref('')
const bdSaving = ref(false)
const bdDistLabel = computed(() => {
  const t = bdTarget.value
  return t ? (t.dist_unit_label || t.dist_unit || t.base_unit) : ''
})
const bdItemMap = computed(() => Object.fromEntries(allStockItems.value.map(i => [i.id, i])))

// key lokal untuk v-for: pola baru belum punya id.
let bdKeySeq = 0
function bdPatternFrom(p = {}) {
  return {
    key: p.id || `baru-${++bdKeySeq}`,
    id: p.id || '',
    name: p.name || '',
    notes: p.notes || '',
    is_active: p.is_active !== false,
    outputs: (p.outputs || []).map(o => ({
      output_item_id: o.output_item_id, name: `${o.output_item_name} (${o.output_item_code})`,
      qty_per_input: o.qty_per_input, cost_weight: o.cost_weight, unit: o.output_base_unit, notes: o.notes || '',
    })),
  }
}

// Pilihan item hasil: item aktif selain bahan & hasil di baris lain pola yang
// sama; item baris ini tetap tampil walau sudah nonaktif. Item yang sama boleh
// dipakai di pola lain.
function bdOutputOptions(p, idx) {
  const line = p.outputs[idx]
  const taken = new Set(p.outputs.filter((_, i) => i !== idx).map(l => l.output_item_id))
  const opts = allStockItems.value
    .filter(i => i.id !== bdTarget.value?.id && !taken.has(i.id))
    .map(i => ({ value: i.id, label: `${i.name} (${i.code})` }))
  if (line?.output_item_id && !opts.some(o => o.value === line.output_item_id)) {
    opts.unshift({ value: line.output_item_id, label: line.name || line.output_item_id })
  }
  return opts
}
function bdUnitOf(r) {
  return bdItemMap.value[r.output_item_id]?.base_unit || r.unit || ''
}
// Pada hasil standar porsi biaya baris = bobot ÷ Σ bobot pola itu.
function bdSharePct(p, r) {
  const total = p.outputs.reduce((s, l) => s + (l.output_item_id && Number(l.cost_weight) > 0 ? Number(l.cost_weight) : 0), 0)
  const w = Number(r.cost_weight)
  if (!r.output_item_id || !(w > 0) || total <= 0) return '—'
  return `${(w / total * 100).toLocaleString('id-ID', { maximumFractionDigits: 1 })}%`
}

async function openBreakdownTemplate(row) {
  bdTarget.value = row
  bdPatterns.value = []
  showBreakdownModal.value = true
  bdLoading.value = true
  bdLoadError.value = ''
  try {
    // Non-paginated: apiClient mengembalikan isinya langsung (bukan { data }).
    const t = await stockBreakdownsApi.getTemplate(row.id)
    bdPatterns.value = (t?.patterns || []).map(bdPatternFrom)
  } catch (e) {
    bdLoadError.value = e?.message || 'Gagal memuat pola pemotongan'
  } finally {
    bdLoading.value = false
  }
}

function addBdPattern() {
  const p = bdPatternFrom({ name: bdPatterns.value.length ? '' : 'Standar' })
  addBdLine(p)
  bdPatterns.value.push(p)
}

function addBdLine(p) {
  p.outputs.push({ output_item_id: '', name: '', qty_per_input: 1, cost_weight: 1, unit: '', notes: '' })
}

async function submitBreakdownTemplate() {
  if (readOnly.value || bdLoading.value || bdLoadError.value || bdSaving.value) return
  const patterns = bdPatterns.value.map(p => ({ ...p, name: p.name.trim(), outputs: p.outputs.filter(l => l.output_item_id) }))
  const names = new Set()
  for (const p of patterns) {
    if (!p.name) {
      toast.error('Setiap pola wajib diberi nama')
      return
    }
    if (names.has(p.name.toLowerCase())) {
      toast.error(`Nama pola "${p.name}" dipakai lebih dari sekali`)
      return
    }
    names.add(p.name.toLowerCase())
    if (p.outputs.length === 0) {
      toast.error(`Pola "${p.name}" belum punya hasil`)
      return
    }
    if (p.outputs.some(l => !(Number(l.qty_per_input) > 0))) {
      toast.error(`Pola "${p.name}": hasil standar tiap baris harus lebih dari 0`)
      return
    }
    if (p.outputs.some(l => !(Number(l.cost_weight) > 0))) {
      toast.error(`Pola "${p.name}": bobot biaya tiap baris harus lebih dari 0`)
      return
    }
  }
  bdSaving.value = true
  try {
    await stockBreakdownsApi.saveTemplate(bdTarget.value.id, patterns.map(p => ({
      id: p.id,
      name: p.name,
      notes: p.notes.trim(),
      is_active: p.is_active,
      outputs: p.outputs.map(l => ({
        output_item_id: l.output_item_id,
        qty_per_input: Number(l.qty_per_input),
        cost_weight: Number(l.cost_weight),
        notes: l.notes || '',
      })),
    })))
    toast.success(patterns.length ? 'Pola pemotongan disimpan' : 'Semua pola pemotongan dihapus')
    showBreakdownModal.value = false
  } catch (e) {
    toast.error(e?.message || 'Gagal menyimpan pola pemotongan')
  } finally {
    bdSaving.value = false
  }
}

defineExpose({ openRecipe, openBreakdownTemplate })
</script>
