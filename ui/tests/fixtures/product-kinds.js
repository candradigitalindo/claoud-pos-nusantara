import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useAuthStore } from '../../src/stores/auth.js'
import Products from '../../src/pages/Products.vue'
import OutletProducts from '../../src/pages/outlet/Products.vue'
import '../../src/style.css'
const params = new URLSearchParams(location.search)
const pinia = createPinia()
useAuthStore(pinia).setPermissions(params.has('readonly') ? ['products.view'] : ['products.view', 'products.create', 'products.update', 'products.delete'])
const outlet = params.get('outlet')
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/outlets/:id/products', component: OutletProducts }, { path: '/', component: Products }] })
await router.push(outlet ? `/outlets/${outlet}/products` : '/')
createApp(outlet ? OutletProducts : Products).use(pinia).use(router).mount('#app')
