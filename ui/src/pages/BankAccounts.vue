<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-xl font-bold text-gray-900">Rekening Perusahaan</h1>
        <p class="text-sm text-gray-500 mt-0.5">Daftar rekening asal untuk pembayaran pengadaan</p>
      </div>
      <AppButton @click="openCreate">+ Tambah Rekening</AppButton>
    </div>

    <AppAlert type="error" :message="errorMsg" />

    <AppCard :padding="false">
      <AppTable :columns="COLUMNS" :rows="accounts" :loading="loading" emptyText="Belum ada rekening.">
        <template #cell-bank_name="{ row }">
          <span class="font-medium text-gray-900">{{ row.bank_name }}</span>
        </template>
        <template #cell-account_number="{ row }">
          <span class="font-mono text-gray-800">{{ row.account_number }}</span>
        </template>
        <template #cell-account_holder="{ row }">
          {{ row.account_holder }}
        </template>
        <template #cell-is_active="{ row }">
          <span v-if="row.is_active" class="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded-full">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span> Aktif
          </span>
          <span v-else class="inline-flex items-center gap-1 text-xs font-medium text-gray-500 bg-gray-100 px-2 py-0.5 rounded-full">
            <span class="w-1.5 h-1.5 rounded-full bg-gray-400"></span> Nonaktif
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1 justify-end">
            <AppActionButton icon="edit" label="Edit" @click="openEdit(row)" />
            <AppActionButton icon="delete" label="Hapus" @click="confirmDelete(row)" />
          </div>
        </template>
      </AppTable>
    </AppCard>

    <!-- Create / Edit Modal -->
    <AppModal v-model="showModal" :title="editTarget ? 'Edit Rekening' : 'Tambah Rekening'" size="sm">
      <div class="space-y-3">
        <AppInput v-model="form.bank_name" label="Nama Bank" placeholder="Contoh: BCA, BRI, Mandiri" />
        <AppInput v-model="form.account_number" label="Nomor Rekening" placeholder="Contoh: 1234567890" />
        <AppInput v-model="form.account_holder" label="Atas Nama" placeholder="Contoh: PT Perusahaan" />
        <div v-if="editTarget" class="flex items-center gap-2">
          <label class="relative inline-flex items-center cursor-pointer">
            <input type="checkbox" v-model="form.is_active" class="sr-only peer" />
            <div class="w-9 h-5 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-emerald-500"></div>
          </label>
          <span class="text-sm text-gray-700">Aktif</span>
        </div>
      </div>
      <template #footer>
        <AppButton variant="secondary" @click="showModal = false">Batal</AppButton>
        <AppButton :loading="saving" @click="submitForm">Simpan</AppButton>
      </template>
    </AppModal>
  </div>
</template>

<script setup>
import AppActionButton from '@/components/ui/AppActionButton.vue'
import { ref, onMounted } from 'vue'
import { bankAccountsApi } from '@/api/bankAccounts.js'
import { useToastStore } from '@/stores/toast.js'
import AppButton from '@/components/ui/AppButton.vue'
import AppCard   from '@/components/ui/AppCard.vue'
import AppTable  from '@/components/ui/AppTable.vue'
import AppModal  from '@/components/ui/AppModal.vue'
import AppAlert  from '@/components/ui/AppAlert.vue'
import AppInput  from '@/components/ui/AppInput.vue'

const toast = useToastStore()

const loading = ref(false)
const errorMsg = ref('')
const accounts = ref([])
const saving = ref(false)

const showModal = ref(false)
const editTarget = ref(null)
const form = ref({ bank_name: '', account_number: '', account_holder: '', is_active: true })

const COLUMNS = [
  { key: 'bank_name',       label: 'Bank' },
  { key: 'account_number',  label: 'Nomor Rekening' },
  { key: 'account_holder',  label: 'Atas Nama' },
  { key: 'is_active',       label: 'Status' },
  { key: 'actions',         label: '', align: 'right' },
]

onMounted(() => fetchList())

async function fetchList() {
  loading.value = true; errorMsg.value = ''
  try {
    const data = await bankAccountsApi.list()
    accounts.value = Array.isArray(data) ? data : []
  } catch (err) { errorMsg.value = err?.message ?? 'Gagal memuat data.' }
  finally { loading.value = false }
}

function openCreate() {
  editTarget.value = null
  form.value = { bank_name: '', account_number: '', account_holder: '', is_active: true }
  showModal.value = true
}

function openEdit(row) {
  editTarget.value = row
  form.value = {
    bank_name: row.bank_name,
    account_number: row.account_number,
    account_holder: row.account_holder,
    is_active: row.is_active,
  }
  showModal.value = true
}

async function submitForm() {
  if (!form.value.bank_name.trim() || !form.value.account_number.trim() || !form.value.account_holder.trim()) {
    toast.error('Semua field wajib diisi.')
    return
  }
  saving.value = true
  try {
    if (editTarget.value) {
      await bankAccountsApi.update(editTarget.value.id, form.value)
      toast.success('Rekening berhasil diperbarui.')
    } else {
      await bankAccountsApi.create(form.value)
      toast.success('Rekening berhasil ditambahkan.')
    }
    showModal.value = false
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal menyimpan.') }
  finally { saving.value = false }
}

async function confirmDelete(row) {
  if (!confirm(`Hapus rekening ${row.bank_name} ${row.account_number}?`)) return
  try {
    await bankAccountsApi.delete(row.id)
    toast.success('Rekening berhasil dihapus.')
    fetchList()
  } catch (err) { toast.error(err?.message ?? 'Gagal menghapus.') }
}
</script>
