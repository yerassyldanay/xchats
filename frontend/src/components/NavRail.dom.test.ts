import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { mountKb, testPinia } from '@/test/mount'
import { useAuth } from '@/stores/auth'
import { useSettings } from '@/stores/settings'
import { useCrm } from '@/stores/crm'
import NavRail from './NavRail.vue'

vi.mock('@/api/evals', () => ({ evalsApi: { probeAvailable: vi.fn(async () => false) } }))

beforeEach(() => localStorage.clear())

function testRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'chatboard', component: { template: '<div/>' } },
      { path: '/settings', name: 'settings', component: { template: '<div/>' } },
      { path: '/draft', name: 'draft', component: { template: '<div/>' } },
      { path: '/knowledge-base', name: 'knowledge-base', component: { template: '<div/>' } },
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
    expect(wrapper.find('a[aria-label="Настройки"] [data-testid="nav-health-dot"]').exists()).toBe(false)
  })

  it('shows a badge dot when a provider is unhealthy', async () => {
    const { wrapper, settingsStore } = mountNavRail()
    settingsStore.unhealthyProviders = new Set(['openrouter'])
    await wrapper.vm.$nextTick()
    expect(wrapper.find('a[aria-label="Настройки"] [data-testid="nav-health-dot"]').exists()).toBe(true)
  })

  it('the Settings link is not rendered at all for a non-admin', () => {
    const pinia = testPinia()
    const auth = useAuth()
    auth.user = { id: '1', email: 'm@x.test', name: 'Member', role: 'member', must_change_password: false }
    const wrapper = mountKb(NavRail, { pinia, global: { plugins: [testRouter()] } })
    expect(wrapper.find('a[aria-label="Настройки"]').exists()).toBe(false)
  })
})

describe('NavRail — collapsible sidebar', () => {
  it('starts open with text labels', () => {
    const { wrapper } = mountNavRail()
    expect(wrapper.classes()).toContain('w-[260px]')
    expect(wrapper.find('a[aria-label="Инбокс"]').text()).toBe('Инбокс')
  })

  it('toggle collapses to an icon-only rail and remembers the choice', async () => {
    const { wrapper } = mountNavRail()
    await wrapper.get('[data-testid="nav-toggle"]').trigger('click')
    expect(wrapper.classes()).toContain('w-[77px]')
    expect(wrapper.find('a[aria-label="Инбокс"]').text()).toBe('')
    expect(localStorage.getItem('xchats-nav-open')).toBe('0')

    await wrapper.get('[data-testid="nav-toggle"]').trigger('click')
    expect(wrapper.classes()).toContain('w-[260px]')
    expect(localStorage.getItem('xchats-nav-open')).toBe('1')
  })

  it('restores a previously collapsed state on mount', () => {
    localStorage.setItem('xchats-nav-open', '0')
    const { wrapper } = mountNavRail()
    expect(wrapper.classes()).toContain('w-[77px]')
  })

  it('shows the overdue count on the Задачи link, capped at 99+', async () => {
    const { wrapper } = mountNavRail()
    const crm = useCrm()
    expect(wrapper.find('[data-testid="nav-overdue-badge"]').exists()).toBe(false)
    crm.buckets.overdue = 3
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-testid="nav-overdue-badge"]').text()).toBe('3')
    crm.buckets.overdue = 250
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-testid="nav-overdue-badge"]').text()).toBe('99+')
  })

  it('keeps the Knowledge Base link active while on the Draft page', async () => {
    const router = testRouter()
    const pinia = testPinia()
    const auth = useAuth()
    auth.user = { id: '1', email: 'admin@x.test', name: 'Admin', role: 'admin', must_change_password: false }
    const wrapper = mountKb(NavRail, { pinia, global: { plugins: [router] } })
    await router.push('/draft')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('a[aria-label="База знаний"]').classes()).toContain('bg-secondary')
    expect(wrapper.get('a[aria-label="Инбокс"]').classes()).not.toContain('bg-secondary')
  })
})
