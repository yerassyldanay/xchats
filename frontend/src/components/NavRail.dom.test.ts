import { describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import NavRail from './NavRail.vue'

vi.mock('@/api/evals', () => ({ evalsApi: { probeAvailable: vi.fn(async () => false) } }))

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'chatboard', component: { template: '<div/>' } },
      { path: '/settings', name: 'settings', component: { template: '<div/>' } },
    ],
  })
  router.push('/')
  return router
}

function mountNavRail() {
  const pinia = testPinia()
  const auth = useAuth()
  const settingsStore = useSettings()
  auth.user = { id: '1', email: 'admin@x.test', name: 'Admin', role: 'admin', must_change_password: false }
  const wrapper = mountKb(NavRail, { pinia, global: { plugins: [testRouter()] } })
  return { wrapper, settingsStore }
}

describe('NavRail — self-healing status badge', () => {
  it('shows no badge when every provider is healthy', () => {
    const { wrapper } = mountNavRail()
    expect(wrapper.find('a[aria-label="Настройки"] span').exists()).toBe(false)
  })

  it('shows a badge dot when a provider is unhealthy', async () => {
    const { wrapper, settingsStore } = mountNavRail()
    settingsStore.unhealthyProviders = new Set(['openrouter'])
    await wrapper.vm.$nextTick()
    expect(wrapper.find('a[aria-label="Настройки"] span').exists()).toBe(true)
  })

  it('the Settings link is not rendered at all for a non-admin', () => {
    const pinia = testPinia()
    const auth = useAuth()
    auth.user = { id: '1', email: 'm@x.test', name: 'Member', role: 'member', must_change_password: false }
    const wrapper = mountKb(NavRail, { pinia, global: { plugins: [testRouter()] } })
    expect(wrapper.find('a[aria-label="Настройки"]').exists()).toBe(false)
  })
})
