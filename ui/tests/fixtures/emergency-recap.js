import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from '../../src/stores/auth.js'
import EmergencyPurchaseRecap from '../../src/pages/EmergencyPurchaseRecap.vue'
import ToastContainer from '../../src/components/ToastContainer.vue'
import '../../src/style.css'
const pinia = createPinia()
const auth = useAuthStore(pinia)
auth.admin = { id: 'finance-test', name: 'Keuangan Uji' }
auth.setPermissions(['finance.payments.view'])
createApp({ render: () => [h(EmergencyPurchaseRecap), h(ToastContainer)] }).use(pinia).mount('#app')
