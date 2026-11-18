<!-- Same wiring as ProjectDetail: the parent keeps the draft in ref(), so the
     editor receives a reactive proxy of the array it emitted. -->
<template>
  <main class="mx-auto max-w-5xl space-y-3 p-4">
    <ProjectRabEditor :model-value="draft" @update:model-value="receive" />
    <button type="button" class="rounded border px-3 py-1 text-sm" @click="replaceFromParent">Ganti dari induk</button>
    <pre id="state" class="text-xs">{{ JSON.stringify({ updates, draft }) }}</pre>
  </main>
</template>

<script setup>
import { ref } from 'vue'
import ProjectRabEditor from '../../src/components/ProjectRabEditor.vue'

const draft = ref([
  { id: 'r1', section: 'Pekerjaan Persiapan', name: 'Pembersihan lokasi', kind: 'jasa', unit: 'ls', qty: 1, unit_price: 500000, notes: '' },
  { id: 'r2', section: 'Pekerjaan Sipil', name: 'Semen', kind: 'barang', unit: 'sak', qty: 20, unit_price: 75000, notes: '' },
])
const updates = ref(0)

function receive(rows) {
  updates.value++
  draft.value = rows
}

function replaceFromParent() {
  draft.value = [{ id: 'r9', section: 'Pekerjaan Atap', name: 'Rangka baja ringan', kind: 'barang', unit: 'm²', qty: 40, unit_price: 185000, notes: '' }]
}
</script>
