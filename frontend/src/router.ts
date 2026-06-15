import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import Chatboard from './views/Chatboard.vue'
import Accounts from './views/Accounts.vue'
import InstancesMaintenance from './views/InstancesMaintenance.vue'
import { useAuth } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: Login },
    { path: '/', name: 'chatboard', component: Chatboard, meta: { requiresAuth: true } },
    { path: '/accounts', name: 'accounts', component: Accounts, meta: { requiresAuth: true } },
    { path: '/instances', name: 'instances', component: InstancesMaintenance, meta: { requiresAuth: true } },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuth()
  if (!auth.ready) await auth.fetchMe()
  if (to.meta.requiresAuth && !auth.isAuthed) return { name: 'login' }
  if (to.name === 'login' && auth.isAuthed) return { name: 'chatboard' }
  return true
})

export default router
