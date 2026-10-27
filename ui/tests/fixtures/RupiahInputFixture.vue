<template>
  <main>
    <h1>Pengujian input rupiah</h1>
    <form id="money-form" @submit.prevent="save">
      <RupiahInput id="amount" v-model="amount" label="Nominal" name="amount" required min="0.01" max="10000000" step="0.01"
        @change="changes++; seenOnChange = amount" />
      <RupiahInput v-model="price" label="Harga satuan" name="price" min="0" step="0.01" />
      <RupiahInput v-model="disabledAmount" label="Nominal nonaktif" name="disabled_amount" disabled />
      <RupiahInput v-model="readonlyAmount" label="Nominal terkunci" name="readonly_amount" readonly />
      <fieldset disabled><RupiahInput v-model="disabledAmount" label="Fieldset nonaktif" /></fieldset>
      <button type="submit">Simpan nominal</button>
    </form>
    <button type="button" @click="amount = 1234567">Muat nominal</button>
    <button type="button" @click="price = 1234.5678">Muat HPP presisi</button>
    <button type="button" @click="amount = 0; price = 1234.56">Reset model</button>
    <form id="step-form" @submit.prevent="stepSubmits++">
      <RupiahInput v-model="stepped" label="Kelipatan pembayaran" min="0" max="1000" step="100" />
      <button type="submit">Simpan kelipatan</button>
    </form>
    <output id="state">{{ JSON.stringify({ amount, amountType: typeof amount, price, disabledAmount, readonlyAmount, changes, seenOnChange, submits, payload, stepped, stepSubmits }) }}</output>
  </main>
</template>
<script setup>
import { ref } from 'vue'
import RupiahInput from '../../src/components/ui/RupiahInput.vue'
const amount = ref(0), price = ref(1234.56), disabledAmount = ref(45000), readonlyAmount = ref(125000)
const changes = ref(0), seenOnChange = ref(null), submits = ref(0), payload = ref(null)
const stepped = ref(0), stepSubmits = ref(0)
function save() {
  submits.value++
  payload.value = { amount: amount.value, price: price.value }
}
</script>
<style scoped>
main { max-width: 640px; margin: 1rem auto; padding: 1rem; }
form { display: grid; gap: 1rem; margin: 1rem 0; }
button { padding: .5rem .75rem; border: 1px solid #b5d9c5; border-radius: .5rem; }
output { display: block; overflow-wrap: anywhere; margin-top: 1rem; }
</style>
