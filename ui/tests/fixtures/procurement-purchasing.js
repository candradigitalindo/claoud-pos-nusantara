import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useAuthStore } from '../../src/stores/auth.js'
import PurchaseGoods from '../../src/pages/PurchaseGoods.vue'
import '../../src/style.css'
const pinia = createPinia()
const auth = useAuthStore(pinia)
auth.admin = { id: 'admin', name: 'Kepala Gudang Uji' }
auth.setPermissions(['procurement.rawmaterials.view', 'procurement.rawmaterials.submit', 'procurement.rawmaterials.purchasing'])
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: PurchaseGoods, meta: { requestType: 'bahan' } }] })
await router.push('/')
createApp(PurchaseGoods).use(pinia).use(router).mount('#app')
