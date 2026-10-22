import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import DashboardLayout from '../../src/layouts/DashboardLayout.vue'
import ProjectUsage from '../../src/pages/ProjectUsage.vue'
import '../../src/style.css'
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: DashboardLayout, children: [{ path: 'perlengkapan/penggunaan-projek', component: ProjectUsage, meta: { title: 'Penggunaan Barang Projek' } }] }] })
await router.push('/perlengkapan/penggunaan-projek?project_id=p1')
createApp(RouterView).use(createPinia()).use(router).mount('#app')
