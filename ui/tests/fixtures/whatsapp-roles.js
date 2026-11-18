import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import { useAuthStore } from '../../src/stores/auth.js'
import WhatsAppSettings from '../../src/pages/whatsapp/WhatsAppSettings.vue'
import Roles from '../../src/pages/Roles.vue'
import '../../src/style.css'
const params = new URLSearchParams(location.search)
const pinia = createPinia(), auth = useAuthStore(pinia)
auth.admin = { id: 'test-admin', name: 'Administrator Uji', role: 'admin' }
auth.setPermissions(params.has('readonly') ? ['whatsapp.view', 'roles.view'] : ['whatsapp.view', 'whatsapp.manage', 'roles.view'])
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/whatsapp', component: WhatsAppSettings }, { path: '/roles', component: Roles }] })
await router.push(params.has('roles') ? '/roles' : { path: '/whatsapp', query: { tab: 'role', role: params.get('role') || 'manager_outlet' } })
createApp(RouterView).use(pinia).use(router).mount('#app')
