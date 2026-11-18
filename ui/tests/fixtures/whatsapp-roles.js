import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import { useAuthStore } from '../../src/stores/auth.js'
import WhatsAppSettings from '../../src/pages/whatsapp/WhatsAppSettings.vue'
import Roles from '../../src/pages/Roles.vue'
import '../../src/style.css'
const params = new URLSearchParams(location.search)
const pinia = createPinia(), auth = useAuthStore(pinia)
auth.admin = { id: 'test-admin', name: 'Pengelola Role Uji', role: 'pengelola_role' }
auth.setPermissions(params.has('readonly') ? ['roles.view'] : ['roles.view', 'roles.update', ...(params.has('whatsapp') ? ['whatsapp.view', 'whatsapp.manage'] : [])])
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/whatsapp', component: WhatsAppSettings }, { path: '/roles', component: Roles }] })
await router.push(params.has('legacy') ? { path: '/whatsapp', query: { tab: 'role', role: params.get('role') || 'kasir' } } : params.has('whatsapp') ? '/whatsapp' : { path: '/roles', query: { tab: 'notifications', role: params.get('role') || 'kasir' } })
window.notificationTestRoute = () => router.currentRoute.value.fullPath
createApp(RouterView).use(pinia).use(router).mount('#app')
