import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import Setup from './Setup.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

async function mountSetup(query = '') {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/settings/integrations')
      return { credential_store_available: true, providers: [{ id: 'openrouter', configured: false, fields: [{ key: 'api_key' }] }] }
    if (path === '/settings') return { llm: { default_provider: 'openrouter', default_model: 'm' } }
    if (path === '/mcp-connection') return { mcp_url: 'http://localhost:8080/mcp', auth_enabled: true, tunnel_available: true, tunnel_running: false, scopes: [] }
    if (path === '/accounts') return { items: [], page: 1, page_size: 50, total: 0 }
    throw new Error(`unexpected GET ${path}`)
  })
  vi.mocked(api.put).mockResolvedValue({ id: 'openrouter', verified: true })
  vi.mocked(api.post).mockResolvedValue({ session_id: 's1', status: 'qr_required' })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/setup', name: 'setup', component: { template: '<div/>' } },
      { path: '/home', name: 'dashboard', component: { template: '<div/>' } },
      { path: '/simulator', name: 'simulator', component: { template: '<div/>' } },
    ],
  })
  await router.push('/setup' + query)
  const wrapper = mountKb(Setup, { pinia: testPinia(), global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('Setup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('opens on the welcome screen with a Start button, no Back, no Skip', async () => {
    const wrapper = await mountSetup()
    expect(wrapper.text()).toContain('Давайте настроим вашего ассистента')
    expect(wrapper.get('[data-testid="setup-next"]').text()).toBe('Начать')
    expect(wrapper.find('[data-testid="setup-back"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="setup-skip"]').exists()).toBe(false)
  })

  it('Start → AI step, with Back and Skip now available', async () => {
    const wrapper = await mountSetup()
    await wrapper.get('[data-testid="setup-next"]').trigger('click')
    expect(wrapper.text()).toContain('Подключите ИИ')
    expect(wrapper.text()).toContain('Шаг 1 из 3')
    expect(wrapper.get('[data-testid="setup-next"]').text()).toBe('Проверить ключ')
    expect(wrapper.find('[data-testid="setup-back"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="setup-skip"]').exists()).toBe(true)
  })

  it('verifies a pasted key on the first press and then offers Next', async () => {
    const wrapper = await mountSetup('?step=ai')
    await wrapper.get('input[type="password"]').setValue('sk-or-1')
    await wrapper.get('[data-testid="setup-next"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Подтверждено')
    expect(wrapper.get('[data-testid="setup-next"]').text()).toBe('Далее')
  })

  it('?step= deep-links straight into a step', async () => {
    const wrapper = await mountSetup('?step=kb')
    expect(wrapper.text()).toContain('Есть ли у вашего сервера свой домен')
  })

  it('the KB step keeps Next disabled until a radio answer is picked', async () => {
    const wrapper = await mountSetup('?step=kb')
    const next = wrapper.get('[data-testid="setup-next"]')
    expect(next.attributes('disabled')).toBeDefined()
    await wrapper.findAll('[role="radio"]')[1]!.trigger('click')
    expect(wrapper.findAll('[role="radio"]')[1]!.attributes('aria-checked')).toBe('true')
    expect(wrapper.get('[data-testid="setup-next"]').attributes('disabled')).toBeUndefined()
  })

  it('Skip from the AI step lands on the channel choice', async () => {
    const wrapper = await mountSetup('?step=ai')
    await wrapper.get('[data-testid="setup-skip"]').trigger('click')
    expect(wrapper.text()).toContain('Где вам пишут клиенты?')
  })

  it('the finish screen drops the footer and offers the next actions', async () => {
    const wrapper = await mountSetup('?step=done')
    expect(wrapper.text()).toContain('Всё готово')
    expect(wrapper.text()).toContain('Проверить ассистента')
    expect(wrapper.text()).toContain('На главную')
    expect(wrapper.find('[data-testid="setup-next"]').exists()).toBe(false)
  })

  it('always offers a way out to the dashboard', async () => {
    const wrapper = await mountSetup()
    expect(wrapper.get('[data-testid="setup-exit"]').text()).toBe('Сохранить и выйти')
  })
})
