import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import PpicReports from '../../src/pages/ppic/PpicReports.vue'
import '../../src/style.css'
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: PpicReports }] })
await router.push('/')
createApp(PpicReports).use(createPinia()).use(router).mount('#app')
