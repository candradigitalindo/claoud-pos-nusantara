import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from '../../src/stores/auth.js'
import ProcurementPayments from '../../src/pages/ProcurementPayments.vue'
import ToastContainer from '../../src/components/ToastContainer.vue'
import '../../src/style.css'

const pinia = createPinia()
const auth = useAuthStore(pinia)
auth.admin = { id: 'finance-test', name: 'Keuangan Uji' }
auth.setPermissions(['finance.payments.view', 'finance.payments.pay'])
createApp({ render: () => [h(ProcurementPayments), h(ToastContainer)] }).use(pinia).mount('#app')
