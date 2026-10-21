<template>
  <main>
    <form id="fixture" @submit.prevent="submits++">
      <label for="required">Gudang wajib</label>
      <SearchableSelect id="required" v-model="warehouse" required name="warehouse" class="field" @change="changed">
        <option value="">Pilih gudang</option>
        <option value="central">Gudang Induk</option><option value="outlet">Outlet Candi</option>
        <option value="locked" disabled>Gudang nonaktif</option>
        <optgroup label="Tidak diizinkan" disabled><option value="forbidden">Gudang terlarang</option></optgroup>
      </SearchableSelect>
      <label>Jumlah baris
        <SearchableSelect v-model.number="count" class="field" name="count"><option value="0">Nol</option><option value="25">Dua puluh lima</option></SearchableSelect>
      </label>
      <label>Nilai boolean
        <SearchableSelect v-model="flag" class="field"><option :value="false">Tidak</option><option :value="true">Ya</option></SearchableSelect>
      </label>
      <label>Satuan
        <SearchableSelect :value="unit" class="field" @change="unit = $event.target.value; unitChanges++"><option value="dus">Dus</option><option value="botol">Botol</option></SearchableSelect>
      </label>
      <label>Item asinkron
        <SearchableSelect v-model="item" class="field"><option value="">Semua item</option><option v-for="o in items" :key="o.id" :value="o.id">{{ o.name }}</option></SearchableSelect>
      </label>
      <button type="button" @click="loadItems">Muat item</button>
      <button type="button" @click="items = []; item = ''">Kosongkan item</button>
      <label>Kontrol nonaktif <SearchableSelect v-model="unit" class="field" disabled><option value="dus">Dus</option></SearchableSelect></label>
      <fieldset disabled><label>Fieldset nonaktif <SearchableSelect v-model="unit" class="field"><option value="dus">Dus</option></SearchableSelect></label></fieldset>
      <AppSelect v-model="appValue" label="Jenis AppSelect" :options="[{value: 0, label: 'Nol'}, {value: 1, label: 'Satu'}]" />
      <SearchSelect v-model="searchValue" aria-label="Komponen lama" :options="many" />
      <MultiSearchSelect v-model="multi" :options="many" placeholder="Pilih beberapa" />
      <button type="submit">Simpan</button><button type="reset">Reset</button><button type="button" id="last">Selesai</button>
    </form>
    <output id="state">{{ JSON.stringify({ warehouse, count, countType: typeof count, flag, flagType: typeof flag, unit, unitChanges, item, changes, seenModel, appValue, appType: typeof appValue, searchValue, multi, submits }) }}</output>
  </main>
</template>
<script setup>
import { ref } from 'vue'
import SearchableSelect from '../../src/components/ui/SearchableSelect.vue'
import SearchSelect from '../../src/components/ui/SearchSelect.vue'
import AppSelect from '../../src/components/ui/AppSelect.vue'
import MultiSearchSelect from '../../src/components/ui/MultiSearchSelect.vue'
const warehouse = ref(''), count = ref(0), flag = ref(false), unit = ref('dus'), item = ref('')
const changes = ref(0), unitChanges = ref(0), seenModel = ref(''), submits = ref(0), appValue = ref(0)
const searchValue = ref(''), multi = ref([]), items = ref([])
const many = Array.from({ length: 100 }, (_, i) => ({ id: String(i), name: `Pilihan ${i}` }))
function changed() { changes.value++; seenModel.value = warehouse.value }
function loadItems() { setTimeout(() => { items.value = [{ id: 'tea', name: 'Téh Melati' }, { id: 'water', name: 'Air Mineral' }] }, 50) }
</script>
<style scoped>
main { max-width: 640px; margin: 1rem auto; padding: 1rem; }
form { display: grid; gap: .6rem; }
label { display: grid; gap: .2rem; }
.field { padding: .5rem .75rem; border: 1px solid #b5d9c5; border-radius: .5rem; width: 100%; background: white; }
output { display: block; overflow-wrap: anywhere; }
</style>
