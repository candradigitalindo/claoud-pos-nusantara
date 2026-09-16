<template>
  <div class="space-y-4 sm:space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-lg sm:text-xl font-bold text-gray-900">Kategori Aset</h1>
        <p class="mt-0.5 text-sm text-gray-500">
          Kelompok aset beserta umur ekonomis dan interval perawatan bawaannya — dipakai
          otomatis saat aset baru dicatat.
        </p>
      </div>
      <AppButton v-if="canManage" class="w-full sm:w-auto" @click="openCreate">+ Tambah Kategori</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <div class="flex items-center gap-2 border-b border-gray-100 p-3">
        <label class="flex items-center gap-1.5 text-xs text-gray-600">
          <input type="checkbox" v-model="showInactive" class="h-3.5 w-3.5 accent-emerald-600" @change="load" />
          Tampilkan yang nonaktif
        </label>
      </div>

      <div v-if="loading" class="p-6 text-center text-sm text-gray-400">Memuat…</div>
      <div v-else-if="!rows.length" class="p-6 text-center text-sm text-gray-400">Belum ada kategori.</div>

      <!-- Mobile -->
      <ul v-else class="divide-y divide-gray-100 sm:hidden">
        <li v-for="c in rows" :key="c.id" class="space-y-2 p-4">
          <div class="flex items-start justify-between gap-2">
            <p class="break-words font-semibold text-gray-900">{{ c.name }}</p>
            <span :class="c.is_active ? 'badge-ok' : 'badge-mute'">{{ c.is_active ? 'Aktif' : 'Nonaktif' }}</span>
          </div>
          <dl class="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs">
            <dt class="text-gray-400">Umur ekonomis</dt><dd class="text-gray-700">{{ lifeText(c) }}</dd>
            <dt class="text-gray-400">Interval rawat</dt><dd class="text-gray-700">{{ intervalText(c) }}</dd>
            <dt class="text-gray-400">Dipakai</dt><dd class="text-gray-700">{{ c.asset_count }} aset</dd>
          </dl>
          <div v-if="canManage" class="flex gap-2 pt-1">
            <button class="act-btn bg-gray-100 text-gray-700" @click="openEdit(c)">Edit</button>
            <button v-if="canDelete && !c.asset_count" class="act-btn bg-red-50 text-red-600" @click="remove(c)">Hapus</button>
          </div>
        </li>
      </ul>

      <!-- Desktop -->
      <div v-if="rows.length" class="hidden overflow-x-auto sm:block">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500">
            <tr>
              <th class="p-2 text-left">Kategori</th>
              <th class="p-2 text-right">Umur Ekonomis</th>
              <th class="p-2 text-right">Interval Perawatan</th>
              <th class="p-2 text-right">Dipakai</th>
              <th class="p-2 text-left">Status</th>
              <th class="p-2"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="c in rows" :key="c.id" class="hover:bg-gray-50">
              <td class="p-2">
                <p class="font-medium text-gray-900">{{ c.name }}</p>
                <p v-if="c.notes" class="text-[11px] text-gray-400">{{ c.notes }}</p>
              </td>
              <td class="p-2 text-right text-gray-700">{{ lifeText(c) }}</td>
              <td class="p-2 text-right text-gray-700">{{ intervalText(c) }}</td>
              <td class="p-2 text-right text-gray-600">{{ c.asset_count }}</td>
              <td class="p-2"><span :class="c.is_active ? 'badge-ok' : 'badge-mute'">{{ c.is_active ? 'Aktif' : 'Nonaktif' }}</span></td>
              <td class="p-2">
                <div v-if="canManage" class="flex justify-end gap-1">
                  <button class="rounded px-2 py-1 text-xs font-medium text-gray-600 hover:bg-gray-100" @click="openEdit(c)">Edit</button>
                  <button v-if="canDelete && !c.asset_count" class="rounded px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50" @click="remove(c)">Hapus</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </AppCard>

    <AppModal v-model="modal" :title="editing ? 'Edit Kategori' : 'Tambah Kategori'" size="lg">
      <form class="space-y-3" @submit.prevent="save">
        <div>
          <label class="lbl">Nama Kategori <span class="text-red-500">*</span></label>
          <input v-model="form.name" class="form-input" placeholder="mis. AC & Pendingin" required />
          <p v-if="editing && editing.asset_count" class="mt-1 text-[11px] text-amber-700">
            Mengganti nama juga memperbarui {{ editing.asset_count }} aset yang memakainya.
          </p>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="lbl">Umur Ekonomis (bulan)</label>
            <input v-model.number="form.useful_life_months" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
            <p class="mt-1 text-[11px] text-gray-400">Mengisi otomatis saat aset baru dicatat. 0 = tidak disusutkan.</p>
          </div>
          <div>
            <label class="lbl">Interval Perawatan (bulan)</label>
            <input v-model.number="form.maintenance_interval_months" type="number" inputmode="numeric" min="0" class="form-input" placeholder="0" />
            <p class="mt-1 text-[11px] text-gray-400">Menentukan jadwal berikutnya saat perawatan ditutup. 0 = tanpa jadwal.</p>
          </div>
        </div>
        <div>
          <label class="lbl">Catatan</label>
          <input v-model="form.notes" class="form-input" placeholder="Contoh barang yang termasuk kategori ini" />
        </div>
        <label v-if="editing" class="flex items-center gap-2 text-sm text-gray-700">
          <input type="checkbox" v-model="form.is_active" class="h-4 w-4 accent-emerald-600" />
          Aktif — kategori nonaktif tidak muncul saat mencatat aset baru
        </label>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="btn-ghost" @click="modal = false">Batal</button>
          <AppButton type="submit" :loading="saving">{{ editing ? 'Simpan' : 'Tambah' }}</AppButton>
        </div>
      </form>
    </AppModal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { assetCategoriesApi } from '@/api/assetCategories.js'
import { useToastStore } from '@/stores/toast.js'
import { useAuthStore } from '@/stores/auth.js'
import AppCard from '@/components/ui/AppCard.vue'
import AppAlert from '@/components/ui/AppAlert.vue'
import AppButton from '@/components/ui/AppButton.vue'
import AppModal from '@/components/ui/AppModal.vue'

const toast = useToastStore()
const auth = useAuthStore()
const canManage = auth.hasPermission('assets.update')
const canDelete = auth.hasPermission('assets.delete')

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const showInactive = ref(false)
const modal = ref(false)
const editing = ref(null)
const form = ref(blank())

function blank() { return { name: '', useful_life_months: 0, maintenance_interval_months: 0, notes: '', is_active: true } }
function asArray(d) { return Array.isArray(d) ? d : (d?.data || []) }
function lifeText(c) { return c.useful_life_months ? `${c.useful_life_months} bulan` : 'tidak disusutkan' }
function intervalText(c) { return c.maintenance_interval_months ? `tiap ${c.maintenance_interval_months} bulan` : 'tanpa jadwal' }

async function load() {
  loading.value = true; errorMsg.value = ''
  try { rows.value = asArray(await assetCategoriesApi.list(showInactive.value)) }
  catch (e) { errorMsg.value = e?.message || 'Gagal memuat kategori' }
  finally { loading.value = false }
}

function openCreate() { editing.value = null; form.value = blank(); modal.value = true }
function openEdit(c) {
  editing.value = c
  form.value = { name: c.name, useful_life_months: c.useful_life_months, maintenance_interval_months: c.maintenance_interval_months, notes: c.notes, is_active: c.is_active }
  modal.value = true
}

async function save() {
  if (!form.value.name?.trim()) { toast.error('Nama kategori wajib diisi'); return }
  saving.value = true
  try {
    if (editing.value) await assetCategoriesApi.update(editing.value.id, form.value)
    else await assetCategoriesApi.create(form.value)
    toast.success(editing.value ? 'Kategori diperbarui' : 'Kategori ditambahkan')
    modal.value = false
    await load()
  } catch (e) { toast.error(e?.message || 'Gagal menyimpan') } finally { saving.value = false }
}

async function remove(c) {
  if (!window.confirm(`Hapus kategori "${c.name}"?`)) return
  try { await assetCategoriesApi.remove(c.id); toast.success('Kategori dihapus'); await load() }
  catch (e) { toast.error(e?.message || 'Gagal menghapus') }
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
.btn-ghost { padding: .5rem 1rem; border-radius: .6rem; font-size: .85rem; font-weight: 600; color: #374151; background: #f3f4f6; min-height: 40px; }
.btn-ghost:hover { background: #e5e7eb; }
.act-btn { flex: 1; min-height: 40px; border-radius: .6rem; padding: .4rem .5rem; font-size: .75rem; font-weight: 600; text-align: center; }
</style>
