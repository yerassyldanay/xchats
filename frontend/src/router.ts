import { createRouter, createWebHistory } from 'vue-router'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import ChangePassword from './views/ChangePassword.vue'
import Chatboard from './views/Chatboard.vue'
import Accounts from './views/Accounts.vue'
import Customers from './views/Customers.vue'
import Followups from './views/Followups.vue'
import Playground from './views/Playground.vue'
import KnowledgeBase from './views/KnowledgeBase.vue'
import Simulator from './views/Simulator.vue'
import Campaigns from './views/Campaigns.vue'
import CampaignWizard from './views/CampaignWizard.vue'
import CampaignDetail from './views/CampaignDetail.vue'
import EvalRuns from './views/EvalRuns.vue'
import EvalLaunchDetail from './views/EvalLaunchDetail.vue'
import EvalCatalog from './views/EvalCatalog.vue'
import Blog from './views/Blog.vue'
import Settings from './views/Settings.vue'
import { useAuth } from './stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: Login },
    { path: '/change-password', name: 'change-password', component: ChangePassword, meta: { requiresAuth: true } },
    { path: '/', name: 'home', component: Home },
    { path: '/chatboard', name: 'chatboard', component: Chatboard, meta: { requiresAuth: true } },
    { path: '/accounts', name: 'accounts', component: Accounts, meta: { requiresAuth: true } },
    { path: '/customers', name: 'customers', component: Customers, meta: { requiresAuth: true } },
    { path: '/followups', name: 'followups', component: Followups, meta: { requiresAuth: true } },
    { path: '/draft', name: 'draft', component: Playground, meta: { requiresAuth: true } },
    // KB-01: /playground read "test AI prompts" (that's actually /simulator)
    // when the page is really the staged-changes review/ingest surface —
    // renamed to /draft to match the nav label and page header. Old links/
    // bookmarks still land somewhere real instead of a bare 404.
    { path: '/playground', redirect: '/draft' },
    { path: '/knowledge-base', name: 'knowledge-base', component: KnowledgeBase, meta: { requiresAuth: true } },
    { path: '/simulator', name: 'simulator', component: Simulator, meta: { requiresAuth: true } },
    { path: '/campaigns', name: 'campaigns', component: Campaigns, meta: { requiresAuth: true } },
    // MUST come before /campaigns/:campaignId — that dynamic segment would
    // otherwise swallow this path with campaignId === "new".
    { path: '/campaigns/new', name: 'campaign-new', component: CampaignWizard, meta: { requiresAuth: true } },
    { path: '/campaigns/:campaignId', name: 'campaign-detail', component: CampaignDetail, meta: { requiresAuth: true } },
    { path: '/evals', name: 'evals', component: EvalRuns, meta: { requiresAuth: true } },
    // MUST come before /evals/:launchId — that dynamic segment would otherwise
    // swallow this path with launchId === "catalog".
    { path: '/evals/catalog', name: 'eval-catalog', component: EvalCatalog, meta: { requiresAuth: true } },
    { path: '/evals/:launchId', name: 'eval-launch', component: EvalLaunchDetail, meta: { requiresAuth: true } },
    { path: '/blog', name: 'blog', component: Blog },
    { path: '/settings', name: 'settings', component: Settings, meta: { requiresAuth: true, requiresAdmin: true } },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuth()
  if (!auth.ready) await auth.fetchMe()
  if (to.meta.requiresAuth && !auth.isAuthed) return { name: 'login' }
  if (to.name === 'login' && auth.isAuthed) return { name: 'chatboard' }
  // A1: force through the change-password screen before any other
  // authenticated page — mirrors the backend's requirePasswordChanged gate
  // (server.go), which blocks the underlying API calls regardless; this
  // just steers the user there directly instead of a page loading into a
  // wall of 403s. Scoped to requiresAuth routes only — the public marketing/
  // blog pages need no such redirect, since they never call a gated API.
  if (to.meta.requiresAuth && auth.mustChangePassword && to.name !== 'change-password') {
    return { name: 'change-password' }
  }
  // And the reverse: nothing to do on /change-password once it's resolved
  // (or if navigated to directly without ever needing it).
  if (to.name === 'change-password' && !auth.mustChangePassword) {
    return { name: 'chatboard' }
  }
  if (to.meta.requiresAdmin && !auth.isAdmin) return { name: 'chatboard' }
  return true
})

export default router
