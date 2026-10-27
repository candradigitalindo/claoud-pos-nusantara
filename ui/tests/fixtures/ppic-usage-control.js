import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import UsageControl from '../../src/pages/ppic/UsageControl.vue'
import StockOpname from '../../src/pages/ppic/StockOpname.vue'
import ExpiryMonitor from '../../src/pages/ppic/ExpiryMonitor.vue'
import '../../src/style.css'

const params = new URLSearchParams(location.search)
const shell = params.has('shell')
const app = createApp(RouterView).use(createPinia())
const router = shell ? (await import('../../src/router/index.js')).default : createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/ppic/usage-control', component: UsageControl },
    { path: '/ppic/opname', component: StockOpname },
    { path: '/ppic/expiry', component: ExpiryMonitor },
    { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
  ],
})
app.use(router)
await router.replace(params.get('route') || '/ppic/usage-control')
await router.isReady()
if (shell) document.body.classList.remove('p-4')
app.mount('#app')
