import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import Chatboard from './views/Chatboard.vue'
import Accounts from './views/Accounts.vue'
import Playground from './views/Playground.vue'
import KnowledgeBase from './views/KnowledgeBase.vue'
import EvalRuns from './views/EvalRuns.vue'
import EvalLaunchDetail from './views/EvalLaunchDetail.vue'
import EvalCatalog from './views/EvalCatalog.vue'
import Blog from './views/Blog.vue'
import { useAuth } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: Login },
    { path: '/', name: 'home', component: Home },
    { path: '/chatboard', name: 'chatboard', component: Chatboard, meta: { requiresAuth: true } },
    { path: '/accounts', name: 'accounts', component: Accounts, meta: { requiresAuth: true } },
    { path: '/playground', name: 'playground', component: Playground, meta: { requiresAuth: true } },
    { path: '/knowledge-base', name: 'knowledge-base', component: KnowledgeBase, meta: { requiresAuth: true } },
    { path: '/evals', name: 'evals', component: EvalRuns, meta: { requiresAuth: true } },
    // MUST come before /evals/:launchId — that dynamic segment would otherwise
    // swallow this path with launchId === "catalog".
    { path: '/evals/catalog', name: 'eval-catalog', component: EvalCatalog, meta: { requiresAuth: true } },
    { path: '/evals/:launchId', name: 'eval-launch', component: EvalLaunchDetail, meta: { requiresAuth: true } },
    { path: '/blog', name: 'blog', component: Blog },
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
