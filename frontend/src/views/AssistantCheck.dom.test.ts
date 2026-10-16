import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import AssistantCheck from './AssistantCheck.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

async function mountAt(path: '/simulator' | '/chat') {
  const { api } = await import('@/api/client')
  // Both modes load history on mount; an empty answer is enough for the shell.
  vi.mocked(api.get).mockImplementation(async (p: string) => {
    if (p.startsWith('/chat')) return { items: [], page: 1, page_size: 50, total: 0 }
    return { items: [] }
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/simulator', name: 'simulator', component: AssistantCheck },
      { path: '/chat', name: 'chat', component: AssistantCheck },
    ],
  })
  await router.push(path)
  const wrapper = mountKb(AssistantCheck, { pinia: testPinia(), global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('AssistantCheck', () => {
  beforeEach(() => vi.clearAllMocks())

  it('/simulator shows the title and the "Как клиент" mode with the simulator controls', async () => {
    const { wrapper } = await mountAt('/simulator')
    expect(wrapper.text()).toContain('Проверка ассистента')
    expect(wrapper.get('[data-testid="check-mode-simulator"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-testid="check-mode-chat"]').attributes('aria-pressed')).toBe('false')
    expect(wrapper.find('[data-testid="simulator-input"]').exists()).toBe(true)
  })

  it('/chat shows the knowledge-base assistant instead of the simulator', async () => {
    const { wrapper } = await mountAt('/chat')
    expect(wrapper.get('[data-testid="check-mode-chat"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('[data-testid="simulator-input"]').exists()).toBe(false)
  })

  it('switching mode navigates between the two URLs', async () => {
    const { wrapper, router } = await mountAt('/simulator')
    await wrapper.get('[data-testid="check-mode-chat"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('chat')
    expect(wrapper.find('[data-testid="simulator-input"]').exists()).toBe(false)

    await wrapper.get('[data-testid="check-mode-simulator"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('simulator')
    expect(wrapper.find('[data-testid="simulator-input"]').exists()).toBe(true)
  })
})
