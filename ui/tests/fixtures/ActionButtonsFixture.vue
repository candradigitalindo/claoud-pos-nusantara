<template>
  <main>
    <h1>Tombol aksi per data</h1>
    <form @submit.prevent="submits++">
      <div @click="rowClicks++">
        <AppActionButton icon="edit" label="Edit produk" @click.stop="edits++" />
        <AppActionButton icon="delete" label="Hapus produk" disabled @click="deletes++" />
        <AppActionButton icon="approve" label="Setujui pengajuan" loading @click="approvals++" />
      </div>
      <AppButton type="submit">Simpan</AppButton>
      <AppButton variant="secondary">Batal</AppButton>
    </form>
    <section>
      <AppActionButton icon="download" label="Unduh bukti" href="data:text/plain,bukti" download="bukti.txt" />
      <AppActionButton icon="view" label="Buka detail" to="/tests/fixtures/action-buttons.html?detail=1" />
      <AppActionButton icon="link" label="Akses ditolak" to="/forbidden-target" disabled />
      <AppActionButton icon="download" label="Unduhan nonaktif" href="/forbidden-download" disabled />
    </section>
    <div class="gallery">
      <div v-for="icon in Object.keys(ACTION_ICONS)" :key="icon" class="sample">
        <AppActionButton :icon="icon" :label="`Aksi ${icon}`" /><small>{{ icon }}</small>
      </div>
    </div>
    <output id="state">{{ JSON.stringify({ edits, deletes, approvals, rowClicks, submits }) }}</output>
  </main>
</template>
<script setup>
import { ref } from 'vue'
import AppActionButton from '../../src/components/ui/AppActionButton.vue'
import AppButton from '../../src/components/ui/AppButton.vue'
import { ACTION_ICONS } from '../../src/utils/actionIcons'
const edits = ref(0), deletes = ref(0), approvals = ref(0), rowClicks = ref(0), submits = ref(0)
</script>
<style scoped>
main { max-width: 900px; padding: 1.5rem; margin: auto; }
h1 { font-weight: 600; font-size: 1.2rem; margin-bottom: 1rem; }
form, section { display: flex; gap: .75rem; flex-wrap: wrap; margin-bottom: 1rem; align-items: center; }
.gallery { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: .75rem; padding: 1rem; background: #fff; border-radius: 1rem; }
.sample { display: grid; justify-items: center; gap: .4rem; }
small { color: #6b7280; }
output { display: block; margin-top: 1rem; overflow-wrap: anywhere; }
</style>
