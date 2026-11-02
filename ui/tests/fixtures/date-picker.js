import { createApp, h, ref } from 'vue'
import DateRangePicker from '../../src/components/ui/DateRangePicker.vue'
import '../../src/style.css'

createApp({
  setup() {
    const range = ref({ from: '2026-11-02', to: '2026-11-02', label: 'Hari Ini' })
    return () => h('main', [
      h(DateRangePicker, {
        modelValue: range.value,
        'onUpdate:modelValue': value => { range.value = value },
        clearable: true,
        label: 'Rentang tanggal laporan',
      }),
      h('output', { id: 'selected-range' }, JSON.stringify(range.value)),
    ])
  },
}).mount('#app')
