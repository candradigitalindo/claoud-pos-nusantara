import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import Fixture from './ActionButtonsFixture.vue'
import '../../src/style.css'

const router = createRouter({ history: createWebHistory(), routes: [{ path: '/:pathMatch(.*)*', component: Fixture }] })
createApp(Fixture).use(router).mount('#app')
