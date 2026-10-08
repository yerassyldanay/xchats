import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import GeneralTemplateTab from '@/components/kb/GeneralTemplateTab.vue'
import PromptTab from '@/components/kb/PromptTab.vue'
import KnowledgeBase from '@/views/KnowledgeBase.vue'
import { ApiError } from '@/api/client'
import type { PromptTemplatesView, PromptView } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), put: vi.fn(), del: vi.fn() } }
})

let mounted: VueWrapper | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
  vi.clearAllMocks()
})

function view(over: Partial<PromptTemplatesView> = {}): PromptTemplatesView {
  return {
    active_template_id: 'general',
    kb_configured: true,
    templates: [
      { id: 'general', instructions: 'ПРАВИЛА-GENERAL', is_default_text: false },
      { id: 'online-shop', instructions: 'ПРАВИЛА-SHOP', is_default_text: false },
      { id: 'service-business', instructions: 'ПРАВИЛА-SERVICE', is_default_text: false },
      { id: 'online-service', instructions: 'ПРАВИЛА-ONLINE', is_default_text: false },
    ],
    ...over,
  }
}

function promptView(over: Partial<PromptView> = {}): PromptView {
  return {
    prompt_ref: 'template:general@1', template_id: 'general', rendered_text: 'ИТОГОВЫЙ ТЕКСТ', frame_text: '%%ASSISTANT%%',
    char_count: 14, approx_tokens: 3, built_at: '2026-01-01T00:00:00Z', status: 'ok',
    section_counts: { topics: 1, products: 2, tariffs: 0, zones: 0, contacts: 1, policies: 1, specialists: 1, services: 3 },
    ...over,
  }
}

async function mountTab(initial: PromptTemplatesView = view()) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/kb/templates') return structuredClone(initial) as any
    if (path === '/kb/prompt') return promptView() as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(GeneralTemplateTab, { pinia })
  mounted = wrapper
  await flushPromises()
  return { wrapper, pg: usePlayground(), api }
}

const textarea = (w: VueWrapper) => w.get('[data-testid="template-instructions"]')
async function typeInto(w: VueWrapper, value: string) {
  await textarea(w).setValue(value)
  await w.vm.$nextTick()
}
async function pick(w: VueWrapper, id: string) {
  await w.get(`[data-testid="template-profile-${id}"]`).trigger('click')
  await w.vm.$nextTick()
}
const saveBtn = (w: VueWrapper) => w.get('[data-testid="template-save"]')

describe('GeneralTemplateTab', () => {
  it('opens on the active profile with its saved instructions, kept apart from the controls', async () => {
    const { wrapper, api } = await mountTab(view({ active_template_id: 'online-shop' }))
    expect(api.get).toHaveBeenCalledWith('/kb/templates')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВИЛА-SHOP')
    expect(wrapper.get('[data-testid="template-profile-online-shop"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.find('[data-testid="template-active-online-shop"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="template-active-general"]').exists()).toBe(false)
    // controls and instruction text are separate sections
    const controls = wrapper.get('[data-testid="template-controls"]')
    const text = wrapper.get('[data-testid="template-text"]')
    expect(controls.element.contains(text.element)).toBe(false)
    expect(controls.find('textarea').exists()).toBe(false)
    expect(text.find('[data-testid="template-save"]').exists()).toBe(false)
    expect(saveBtn(wrapper).attributes('disabled')).toBeDefined() // nothing to save yet
  })

  it('offers all four profiles, optional to choose', async () => {
    const { wrapper } = await mountTab()
    for (const id of ['general', 'online-shop', 'service-business', 'online-service']) {
      expect(wrapper.find(`[data-testid="template-profile-${id}"]`).exists()).toBe(true)
    }
  })

  it('keeps unsaved edits per profile when switching', async () => {
    const { wrapper } = await mountTab()
    await typeInto(wrapper, 'ПРАВКА-ОБЩЕГО')
    expect(wrapper.find('[data-testid="template-dirty-general"]').exists()).toBe(true)

    await pick(wrapper, 'online-shop')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВИЛА-SHOP') // its own saved text
    await typeInto(wrapper, 'ПРАВКА-МАГАЗИНА')

    await pick(wrapper, 'general')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВКА-ОБЩЕГО') // not lost
    await pick(wrapper, 'online-shop')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВКА-МАГАЗИНА')
    expect(wrapper.find('[data-testid="template-dirty-general"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="template-dirty-online-shop"]').exists()).toBe(true)
  })

  it('saves the selected profile only, activates it, and refreshes the Final Template', async () => {
    const { wrapper, pg, api } = await mountTab()
    const saved = view({ active_template_id: 'online-shop' })
    saved.templates[1]!.instructions = 'ПРАВКА-МАГАЗИНА'
    vi.mocked(api.put).mockResolvedValue(saved as any)

    await pick(wrapper, 'online-shop')
    await typeInto(wrapper, 'ПРАВКА-МАГАЗИНА')
    expect(saveBtn(wrapper).attributes('disabled')).toBeUndefined()
    await saveBtn(wrapper).trigger('click')
    await flushPromises()

    expect(api.put).toHaveBeenCalledTimes(1)
    expect(api.put).toHaveBeenCalledWith('/kb/templates/online-shop', { instructions: 'ПРАВКА-МАГАЗИНА', activate: true })
    expect(api.get).toHaveBeenCalledWith('/kb/prompt') // preview refreshed without waiting for SSE
    expect(pg.promptView?.rendered_text).toBe('ИТОГОВЫЙ ТЕКСТ')
    expect(wrapper.find('[data-testid="template-active-online-shop"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="template-dirty-online-shop"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="template-saved"]').exists()).toBe(true)
  })

  it('lets a profile be chosen and activated without editing its text', async () => {
    const { wrapper, api } = await mountTab()
    vi.mocked(api.put).mockResolvedValue(view({ active_template_id: 'service-business' }) as any)
    await pick(wrapper, 'service-business')
    expect(saveBtn(wrapper).attributes('disabled')).toBeUndefined() // selecting a different profile can be saved
    await saveBtn(wrapper).trigger('click')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith('/kb/templates/service-business', { instructions: 'ПРАВИЛА-SERVICE', activate: true })
  })

  it('blocks reserved syntax client-side and never sends it', async () => {
    const { wrapper, api } = await mountTab()
    await typeInto(wrapper, 'вставь %%FACTS%% сюда')
    expect(saveBtn(wrapper).attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="template-error"]').text()).toContain('двойн')
    await typeInto(wrapper, 'цена {{product.x.price}}')
    expect(saveBtn(wrapper).attributes('disabled')).toBeDefined()
    await typeInto(wrapper, '   ')
    expect(saveBtn(wrapper).attributes('disabled')).toBeDefined()
    await typeInto(wrapper, 'нормальные правила')
    expect(saveBtn(wrapper).attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-testid="template-error"]').exists()).toBe(false)
    expect(api.put).not.toHaveBeenCalled()
  })

  it('shows the server rejection and keeps the operator\'s text', async () => {
    const { wrapper, api } = await mountTab()
    vi.mocked(api.put).mockRejectedValue(new ApiError('ERR_VALIDATION', 422, 'template instructions must not contain links'))
    await typeInto(wrapper, 'см. сайт')
    await saveBtn(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="template-error"]').text()).toContain('must not contain links')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('см. сайт')
    expect(wrapper.find('[data-testid="template-dirty-general"]').exists()).toBe(true)
  })

  it('discards the selected profile\'s unsaved edit only', async () => {
    const { wrapper } = await mountTab()
    await typeInto(wrapper, 'ПРАВКА-A')
    await pick(wrapper, 'online-shop')
    await typeInto(wrapper, 'ПРАВКА-B')
    await wrapper.get('[data-testid="template-discard"]').trigger('click')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВИЛА-SHOP')
    expect(wrapper.find('[data-testid="template-dirty-online-shop"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="template-dirty-general"]').exists()).toBe(true) // the other edit stays
  })

  it('a server refresh never overwrites text still being edited', async () => {
    const { wrapper, pg } = await mountTab()
    await typeInto(wrapper, 'МОЯ-ПРАВКА')
    const refreshed = view()
    refreshed.templates[0]!.instructions = 'ИЗМЕНЕНО-В-ДРУГОЙ-ВКЛАДКЕ'
    pg.templates = refreshed
    await wrapper.vm.$nextTick()
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('МОЯ-ПРАВКА')
  })

  it('first launch: shows the defaults together with the not-configured notice', async () => {
    const { wrapper } = await mountTab(view({ kb_configured: false }))
    expect(wrapper.find('[data-testid="template-not-configured"]').exists()).toBe(true)
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('ПРАВИЛА-GENERAL')
    expect(saveBtn(wrapper).attributes('disabled')).toBeDefined()
  })

  it('surfaces a load failure with a retry', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockRejectedValue(new ApiError('ERR_INTERNAL', 500, 'boom'))
    const wrapper = mountKb(GeneralTemplateTab, { pinia: testPinia() })
    mounted = wrapper
    await flushPromises()
    expect(wrapper.text()).toContain('boom')
    expect(wrapper.find('[data-testid="template-instructions"]').exists()).toBe(false)
  })
})

describe('Final Template tab', () => {
  async function mountPrompt(pv: PromptView) {
    const { api } = await import('@/api/client')
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/kb/prompt') return structuredClone(pv) as any
      throw new Error(`unexpected GET ${path}`)
    })
    const wrapper = mountKb(PromptTab, { pinia: testPinia() })
    mounted = wrapper
    await flushPromises()
    return wrapper
  }

  it('shows the assembled prompt, the active profile and the data counts', async () => {
    const w = await mountPrompt(promptView({ template_id: 'service-business' }))
    expect(w.get('[data-testid="prompt-text"]').text()).toContain('ИТОГОВЫЙ ТЕКСТ')
    expect(w.get('[data-testid="prompt-profile"]').text()).toBe('Сервисный бизнес')
    expect(w.find('[data-testid="prompt-not-configured"]').exists()).toBe(false)
    expect(w.text()).toContain('3') // services count
  })

  it('first launch: shows the not-configured notice AND still shows the preview text', async () => {
    const w = await mountPrompt(promptView({ status: 'not_configured', error: 'knowledge base not configured' }))
    expect(w.get('[data-testid="prompt-not-configured"]').text()).toContain('не настроена')
    expect(w.get('[data-testid="prompt-text"]').text()).toContain('ИТОГОВЫЙ ТЕКСТ')
  })

  it('a real build error still hides the text and shows the reason', async () => {
    const w = await mountPrompt(promptView({ status: 'error', error: 'катастрофа сборки', rendered_text: '' }))
    expect(w.text()).toContain('катастрофа сборки')
    expect(w.find('[data-testid="prompt-text"]').exists()).toBe(false)
  })
})

describe('Knowledge base page tabs', () => {
  it('puts General Template immediately before Final Template, and refreshes the preview when it opens', async () => {
    const { api } = await import('@/api/client')
    const live = {
      config: { organization_id: 'o', persona: '', mission: '', guardrails: '', language_policy: '', reply_max_words: 120, draft: false, base_version: 0, updated_at: '' },
      topics: [], tariffs: [], products: [], specialists: [], services: [], contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
    }
    const changes = {
      base_version: 1, updated_at: '', config: null,
      topics: [], tariffs: [], products: [], specialists: [], services: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
    }
    vi.mocked(api.get).mockImplementation(async (path: string) => {
      if (path === '/playground/draft') return changes as any
      if (path === '/kb') return live as any
      if (path === '/kb/prompt') return promptView() as any
      if (path === '/kb/templates') return view() as any
      throw new Error(`unexpected GET ${path}`)
    })
    const wrapper = mountKb(KnowledgeBase, { pinia: testPinia() })
    mounted = wrapper
    await flushPromises()

    const labels = wrapper.findAll('button').map((b) => b.text())
    const i = labels.indexOf('Общий шаблон')
    expect(i).toBeGreaterThan(-1)
    expect(labels[i + 1]).toBe('Итоговый шаблон') // immediately before the renamed Prompt tab
    expect(labels).not.toContain('Промпт')

    const promptCalls = () => vi.mocked(api.get).mock.calls.filter((c) => c[0] === '/kb/prompt').length
    await wrapper.findAll('button').find((b) => b.text() === 'Общий шаблон')!.trigger('click')
    await flushPromises()
    expect(api.get).toHaveBeenCalledWith('/kb/templates')
    const before = promptCalls()
    await wrapper.findAll('button').find((b) => b.text() === 'Итоговый шаблон')!.trigger('click')
    await flushPromises()
    expect(promptCalls()).toBe(before + 1) // re-fetched every time it is opened
  })
})
