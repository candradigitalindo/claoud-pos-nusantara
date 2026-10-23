import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DashboardLayout from '../../src/layouts/DashboardLayout.vue'
import Customers from '../../src/pages/Customers.vue'
import '../../src/style.css'

const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: DashboardLayout, children: [{ path: 'customers', component: Customers, meta: { title: 'Pelanggan — Cloud POS' } }] }] })
await router.push('/customers')
createApp(RouterView).use(createPinia()).use(router).mount('#app')
