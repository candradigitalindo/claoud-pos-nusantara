import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import FnbCosting from '../../src/pages/fnb/FnbCosting.vue'
import '../../src/style.css'
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: FnbCosting }] })
await router.push('/')
createApp(FnbCosting).use(createPinia()).use(router).mount('#app')
