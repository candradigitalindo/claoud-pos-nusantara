import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { useAuthStore } from '../../src/stores/auth.js'
import AppPos from '../../src/pages/AppPos.vue'
import ToastContainer from '../../src/components/ToastContainer.vue'
import '../../src/style.css'

const pinia = createPinia()
useAuthStore(pinia).setPermissions(new URLSearchParams(location.search).has('readonly')
  ? ['appfiles.view']
  : ['appfiles.view', 'appfiles.create', 'appfiles.delete'])
createApp({ render: () => h('div', [h(AppPos), h(ToastContainer)]) }).use(pinia).mount('#app')
