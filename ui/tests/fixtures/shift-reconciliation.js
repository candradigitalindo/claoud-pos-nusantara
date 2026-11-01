import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from '../../src/stores/auth.js'
import ShiftReconciliation from '../../src/pages/ShiftReconciliation.vue'
import '../../src/style.css'

const pinia = createPinia()
const auth = useAuthStore(pinia)
auth.admin = { role: new URLSearchParams(location.search).has('readonly') ? 'viewer' : 'superadmin' }
auth.setPermissions(['shift_reconciliation.view'])
createApp(ShiftReconciliation).use(pinia).mount('#app')
