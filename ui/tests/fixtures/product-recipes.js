import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from '../../src/stores/auth.js'
import Products from '../../src/pages/Products.vue'
import '../../src/style.css'
const pinia = createPinia()
useAuthStore(pinia).setPermissions(['products.view', 'products.create', 'products.update', 'products.delete'])
createApp(Products).use(pinia).mount('#app')
