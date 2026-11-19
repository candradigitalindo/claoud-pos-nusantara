import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useAuthStore } from '../../src/stores/auth.js'
import PurchaseGoods from '../../src/pages/PurchaseGoods.vue'
import ToastContainer from '../../src/components/ToastContainer.vue'
import '../../src/style.css'
const pinia = createPinia()
const auth = useAuthStore(pinia)
auth.admin = { id: 'admin', name: 'Manajer Uji' }
auth.setPermissions(['procurement.requests.view', 'procurement.requests.submit', 'procurement.requests.emergency', 'procurement.requests.approve'])
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: PurchaseGoods, meta: { requestType: 'barang' } }] })
await router.push('/')
createApp({ render: () => [h(PurchaseGoods), h(ToastContainer)] }).use(pinia).use(router).mount('#app')
