import { createApp } from 'vue'
import { createPinia } from 'pinia'
import FinanceOperations from '../../src/pages/FinanceOperations.vue'
import '../../src/style.css'
createApp(FinanceOperations).use(createPinia()).mount('#app')
