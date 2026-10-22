import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DashboardLayout from '../../src/layouts/DashboardLayout.vue'
import Documents from '../../src/pages/Documents.vue'
import '../../src/style.css'
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: DashboardLayout, children: [{ path: 'documents', component: Documents, meta: { title: 'Dokumen — Cloud POS' } }] }] })
await router.push('/documents')
createApp(RouterView).use(createPinia()).use(router).mount('#app')
