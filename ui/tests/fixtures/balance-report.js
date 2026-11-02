import { createApp, h, ref } from 'vue'
import BalanceReport from '../../src/pages/BalanceReport.vue'
import '../../src/style.css'

// View-only permissions match the report/export permission in the real router.
localStorage.setItem('cloud_pos_permissions', JSON.stringify(['reports.balance.view']))
createApp({
  setup() {
    const shown = ref(true)
    return () => h('div', [
      shown.value ? h(BalanceReport) : h('p', 'Laporan ditutup'),
      h('button', { onClick: () => { shown.value = false }, class: 'mt-8 text-sm' }, 'Tutup laporan uji'),
    ])
  },
}).mount('#app')
