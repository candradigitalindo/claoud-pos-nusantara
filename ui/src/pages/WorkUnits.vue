<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-gray-900">Unit Kerja</h1>
      <AppButton @click="openCreate">+ Tambah Unit Kerja</AppButton>
    </div>

    <AppAlert type="info" message="Outlet otomatis terdaftar sebagai unit kerja. Anda juga dapat menambahkan unit kerja mandiri yang tidak terkait outlet." />
    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <AppTable :columns="COLUMNS" :rows="units" :loading="loading" emptyText="Belum ada unit kerja.">
        <template #cell-outlet_name="{ row }">
          <span v-if="row.outlet_name" class="font-medium text-gray-900">{{ row.outlet_name }}</span>
          <span v-else class="text-gray-400 italic">—</span>
        </template>
        <template #cell-name="{ row }">
          {{ row.name }}
        </template>
        <template #cell-admin_name="{ row }">
          <span v-if="row.admin_name" class="inline-flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
            <span class="text-gray-900 font-medium">{{ row.admin_name }}</span>
          </span>
          <span v-else class="text-gray-400 italic">Belum ditugaskan</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="inline-flex items-center gap-1 justify-end">
            <AppActionButton icon="assign" :label="(row.admin_id ? 'Ubah User' : 'Tugaskan User')" @click="openAssign(row)" />
            <AppActionButton icon="delete" label="Hapus unit kerja mandiri"
              v-if="!row.outlet_id && !row.outlet_name"
              @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- Assign User Modal -->
    <AppModal v-model="showAssign" title="Tugaskan User Pengadaan" size="sm">
      <div class="space-y-3">
        <p class="text-sm text-gray-600">
          Pilih user yang bertanggung jawab atas pengadaan untuk unit kerja
          <strong>{{ assignTarget?.name }}</strong>.
        </p>
        <AppSelect
          v-model="selectedAdminId"
          label="User Pengadaan"
          :options="adminOptions"
          placeholder="Pilih user..."
        />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showAssign = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitAssign">Simpan</AppButton>
      </template>
    </AppModal>

    <!-- Create Work Unit Modal -->
    <AppModal v-model="showCreate" title="Tambah Unit Kerja Baru" size="sm">
      <div class="space-y-3">
        <AppInput v-model="createName" label="Nama Unit Kerja" placeholder="Contoh: Bagian Keuangan" />
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showCreate = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitCreate">Simpan</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, computed, onMounted } from 'vue'
import { workUnitsApi } from '@/api/workUnits.js'
import { adminsApi } from '@/api/admins.js'
import { useToastStore } from '@/stores/toast.js'
import AppButton     from '@/components/ui/AppButton.vue'
import AppCard       from '@/components/ui/AppCard.vue'
import AppTable      from '@/components/ui/AppTable.vue'
import AppModal      from '@/components/ui/AppModal.vue'
import AppSelect     from '@/components/ui/AppSelect.vue'
import AppAlert      from '@/components/ui/AppAlert.vue'
import AppInput      from '@/components/ui/AppInput.vue'

const toast = useToastStore()

const loading = ref(false)
const errorMsg = ref('')
const units = ref([])
const admins = ref([])

const showAssign = ref(false)
const assignTarget = ref(null)
const selectedAdminId = ref('')
const saving = ref(false)

const showCreate = ref(false)
const createName = ref('')

const COLUMNS = [
  { key: 'name',        label: 'Unit Kerja' },
  { key: 'outlet_name', label: 'Outlet' },
  { key: 'admin_name',  label: 'User Pengadaan' },
  { key: 'actions',     label: '', align: 'right' },
]

const adminOptions = computed(() => [
  { value: '', label: '— Tidak ada —' },
  ...admins.value.map(a => ({ value: a.id, label: `${a.name} (${a.username})` })),
])

onMounted(async () => {
  await Promise.all([fetchUnits(), fetchAdmins()])
})

async function fetchUnits() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await workUnitsApi.list()
    units.value = Array.isArray(data) ? data : (data?.work_units || data?.data || [])
  } catch (err) { errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { loading.value = false }
}

async function fetchAdmins() {
  try {
    const data = await adminsApi.list()
    admins.value = Array.isArray(data) ? data : (data?.admins || data?.data || [])
  } catch { /* ignore */ }
}

function openAssign(row) {
  assignTarget.value = row
  selectedAdminId.value = row.admin_id || ''
  showAssign.value = true
}

async function submitAssign() {
  saving.value = true
  try {
    await workUnitsApi.update(assignTarget.value.id, { admin_id: selectedAdminId.value || null })
    toast.success('User pengadaan berhasil diperbarui.')
    showAssign.value = false
    fetchUnits()
  } catch (err) { toast.error(err?.message ?? 'Gagal menyimpan.') }
  finally { saving.value = false }
}

function openCreate() {
  createName.value = ''
  showCreate.value = true
}

async function confirmDelete(row) {
  if (!window.confirm(`Hapus unit kerja "${row.name}"? Tindakan ini tidak dapat dibatalkan.`)) return
  try {
    await workUnitsApi.remove(row.id)
    toast.success('Unit kerja dihapus.')
    fetchUnits()
  } catch (err) { toast.error(err?.message ?? 'Gagal menghapus unit kerja.') }
}

async function submitCreate() {
  if (!createName.value.trim()) {
    toast.error('Nama unit kerja wajib diisi.')
    return
  }
  saving.value = true
  try {
    await workUnitsApi.create({ name: createName.value.trim() })
    toast.success('Unit kerja berhasil ditambahkan.')
    showCreate.value = false
    fetchUnits()
  } catch (err) { toast.error(err?.message ?? 'Gagal menyimpan.') }
  finally { saving.value = false }
}
</script>
