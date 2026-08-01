import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { useKbModal } from '@/composables/useKbModal'
import { api } from '@/api/client'
import { contactRow, emptyChangeSet, emptyLiveView, emptyPromptView, topicRow } from '@/test/fixtures'
import { mountKb, bodyWrapper } from '@/test/mount'
import KnowledgeBase from './KnowledgeBase.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), del: vi.fn(), upload: vi.fn(), mediaURL: vi.fn(), realtimeURL: vi.fn() },
  }
})

function mockRoutes(live = emptyLiveView(), changes = emptyChangeSet()) {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/kb') return live
    if (path === '/kb/prompt') return emptyPromptView()
    if (path === '/playground/draft') return changes
    if (path === '/kb/materials') return { materials: [] }
    throw new Error('unexpected GET ' + path)
  })
}

async function mountLive() {
  const wrapper = mountKb(KnowledgeBase)
  await flushPromises()
  await flushPromises()
  return wrapper
}

function clickTab(wrapper: Awaited<ReturnType<typeof mountLive>>, label: string) {
  const tab = wrapper.findAll('button').find((b) => b.text().includes(label))
  expect(tab, `tab "${label}" not found`).toBeTruthy()
  return tab!.trigger('click')
}

beforeEach(() => vi.clearAllMocks())
afterEach(() => useKbModal().close())

describe('KnowledgeBase (База знаний)', () => {
  it('renders every entity tab plus Промпт/Файлы, defaulting to Обзор, with no Черновик stat tiles', async () => {
    mockRoutes()
    const wrapper = await mountLive()

    const tabLabels = wrapper.findAll('button').map((b) => b.text())
    for (const label of ['Обзор', 'Темы', 'Товары', 'Тарифы', 'Зоны доставки', 'Контакты', 'Политики', 'Промпт', 'Файлы (материалы)']) {
      expect(tabLabels.some((t) => t.includes(label))).toBe(true)
    }
    expect(wrapper.text()).not.toContain('Добавлено')
    expect(wrapper.text()).not.toContain('Всего')
  })

  it('shows the published topic list on the Темы tab, unaffected by any pending draft change', async () => {
    mockRoutes(emptyLiveView({ topics: [topicRow({ id: 't1', slug: 't1', title: 'Published Title' })] }))
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Темы')

    expect(wrapper.text()).toContain('Published Title')
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it("a topic card's «Изменить» opens it prefilled, with the slug natural key locked", async () => {
    mockRoutes(emptyLiveView({ topics: [topicRow({ id: 't1', slug: 't1', title: 'Published Title' })] }))
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Темы')

    // v-show keeps every tab's cards in the DOM at once, including the five
    // Обзор section cards — each with its own "Изменить" button — so match
    // on the card that actually contains this topic, not just the label.
    const editBtn = wrapper
      .findAll('button')
      .find((b) => b.text() === 'Изменить' && b.element.closest('.rounded-lg')?.textContent?.includes('Published Title'))
    await editBtn!.trigger('click')
    await flushPromises()

    expect(bodyWrapper().text()).toContain('Тема') // editTitle, not "Новая тема"
    const inputs = bodyWrapper().findAll('input')
    const slugInput = inputs.find((i) => (i.element as HTMLInputElement).value === 't1')!
    const titleInput = inputs.find((i) => (i.element as HTMLInputElement).value === 'Published Title')!
    expect((slugInput.element as HTMLInputElement).disabled).toBe(true)
    expect((titleInput.element as HTMLInputElement).disabled).toBe(false)
  })

  it("Темы tab's «Добавить тему» toolbar button opens a blank create dialog", async () => {
    mockRoutes()
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Темы')

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить тему'))
    expect(addBtn).toBeTruthy()
    await addBtn!.trigger('click')
    await flushPromises()

    expect(bodyWrapper().text()).toContain('Новая тема')
    const slugInput = bodyWrapper().find('input.font-mono')
    expect((slugInput.element as HTMLInputElement).value).toBe('')
    expect((slugInput.element as HTMLInputElement).disabled).toBe(false)
  })

  it('no toolbar button renders on Обзор, Промпт, or Файлы — those tabs have no create affordance', async () => {
    mockRoutes()
    const wrapper = await mountLive()

    // Обзор is the default tab.
    expect(wrapper.findAll('button').some((b) => b.text().includes('Добавить'))).toBe(false)

    await clickTab(wrapper, 'Промпт')
    expect(wrapper.findAll('button').some((b) => b.text().includes('Добавить'))).toBe(false)

    await clickTab(wrapper, 'Файлы')
    expect(wrapper.findAll('button').some((b) => b.text().includes('Добавить'))).toBe(false)
  })

  it('staging a new topic leaves the published list unchanged and shows the DraftBanner naming the staged kind', async () => {
    mockRoutes(emptyLiveView({ topics: [topicRow({ id: 't1', slug: 't1', title: 'Old Title' })] }))
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Темы')
    expect(wrapper.text()).toContain('Old Title')

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить тему'))
    await addBtn!.trigger('click')
    await flushPromises()

    await bodyWrapper().find('input.font-mono').setValue('new-topic')
    vi.mocked(api.post).mockResolvedValueOnce(
      emptyChangeSet({ topics: [topicRow({ id: 'new-topic', slug: 'new-topic', title: 'New Title' })] })
    )
    const saveBtn = bodyWrapper()
      .findAll('button')
      .find((b) => b.text() === 'Сохранить')
    await saveBtn!.trigger('click')
    await flushPromises()

    // The staged row must never appear in the published list — only Old Title
    // is published, and pg.live was never touched by stageChange.
    expect(wrapper.text()).toContain('Old Title')
    expect(wrapper.text()).not.toContain('New Title')
    expect(wrapper.text()).toContain('Изменение добавлено в Черновик')
    const banner = wrapper.findAll('a').find((a) => a.text().includes('Перейти в Черновик'))
    expect(banner).toBeTruthy()
  })

  it("Контакты toolbar «Изменить контакты» opens the edit dialog prefilled from the published contact", async () => {
    mockRoutes(emptyLiveView({ contacts: [contactRow({ whatsapp: '+7 700 000 00 00' })] }))
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Контакты')

    const editBtn = wrapper.findAll('button').find((b) => b.text().includes('Изменить контакты'))
    await editBtn!.trigger('click')
    await flushPromises()

    expect(bodyWrapper().text()).toContain('Изменить контакты')
    const whatsapp = bodyWrapper().find('input[placeholder="WhatsApp"]')
    expect((whatsapp.element as HTMLInputElement).value).toBe('+7 700 000 00 00')
  })

  it('Обзор renders one card per assistant-config section from the published config, and Изменить opens just that field', async () => {
    mockRoutes(emptyLiveView({ config: { organization_id: 'org1', persona: 'Дружелюбный помощник', mission: '', guardrails: '', language_policy: '', reply_max_words: 120, draft: false, base_version: 1, updated_at: '' } }))
    const wrapper = await mountLive()

    expect(wrapper.text()).toContain('Персона')
    expect(wrapper.text()).toContain('Дружелюбный помощник')
    expect(wrapper.text()).toContain('Миссия')

    // v-show (not v-if) keeps every tab's content permanently in the DOM, so
    // scope to the Обзор section's own container — otherwise Контакты/
    // Политики's own always-mounted "Изменить" buttons get counted too.
    const overview = wrapper.find('.max-w-3xl')
    const editButtons = overview.findAll('button').filter((b) => b.text() === 'Изменить')
    expect(editButtons.length).toBe(5) // one per CONFIG_SECTIONS entry
    await editButtons[0].trigger('click')
    await flushPromises()

    expect(bodyWrapper().text()).toContain('Персона')
    const textarea = bodyWrapper().find('textarea')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('Дружелюбный помощник')
  })

  it('a published row with a pending draft change shows its badge but the card still renders the published value, never the draft one', async () => {
    mockRoutes(
      emptyLiveView({ topics: [topicRow({ id: 't1', slug: 't1', title: 'Published Title' })] }),
      emptyChangeSet({ topics: [topicRow({ id: 't1', slug: 't1', title: 'Pending Title' })] })
    )
    const wrapper = await mountLive()
    await clickTab(wrapper, 'Темы')

    expect(wrapper.text()).toContain('Изменён')
    expect(wrapper.text()).toContain('Published Title')
    expect(wrapper.text()).not.toContain('Pending Title')
  })

  it('the Файлы tab lazy-loads /kb/materials only once switched to', async () => {
    mockRoutes()
    const wrapper = await mountLive()
    expect(api.get).not.toHaveBeenCalledWith('/kb/materials')

    await clickTab(wrapper, 'Файлы')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/kb/materials')
  })
})
