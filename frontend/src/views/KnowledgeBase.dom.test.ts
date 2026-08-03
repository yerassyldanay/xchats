import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import KnowledgeBase from './KnowledgeBase.vue'
import type { DraftChangeSet, DraftView, TopicRow } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

function topic(over: Partial<TopicRow> = {}): TopicRow {
  return {
    id: 'pricing', slug: 'pricing', title: 'Тарифы', body_md: 'Текст.',
    featured_image: null, illustration_images: [], explainer_videos: [], reference_documents: [],
    draft: false, updated_at: '',
    ...over,
  }
}
function emptyLive(over: Partial<DraftView> = {}): DraftView {
  return {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], materials: [], requests: [],
    ...over,
  }
}
function emptyChanges(over: Partial<DraftChangeSet> = {}): DraftChangeSet {
  return {
    base_version: 1, updated_at: '', config: null,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], zones: [], deletes: [],
    ...over,
  }
}

async function mountWith(changes: DraftChangeSet, live: DraftView) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/playground/draft') return changes as any
    if (path === '/kb') return live as any
    if (path === '/kb/materials') return { materials: [] } as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(KnowledgeBase, { pinia })
  await flushPromises()
  return { wrapper, pg: usePlayground(), api }
}

// switchTab clicks a tab by its exact label — the toolbar action (and each
// tab's own content) only renders for the ACTIVE tab, and the page starts
// on Обзор (config is first in KB_ENTITY_ORDER).
async function switchTab(wrapper: VueWrapper, label: string) {
  const tabBtn = wrapper.findAll('button').find((b) => b.text() === label)
  if (!tabBtn) throw new Error(`no tab button labelled ${label}`)
  await tabBtn.trigger('click')
  await wrapper.vm.$nextTick()
}

// body wraps document.body: reka-ui's Dialog renders through a Teleport, so
// its content lives outside @vue/test-utils' own wrapper subtree — queries
// for anything inside an open dialog must go through the real DOM instead
// of `wrapper`.
function body() {
  return new DOMWrapper(document.body)
}

describe('KnowledgeBase — published rows are read-only', () => {
  it('lists a published topic with no input fields', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))
    expect(wrapper.text()).toContain('Тарифы')
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('textarea')).toHaveLength(0)
  })

  it('a published row with a pending change is marked, without showing the draft value', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ topics: [topic({ title: 'Черновик: новое название' })] }),
      emptyLive({ topics: [topic({ title: 'Опубликованное название' })] })
    )
    expect(wrapper.text()).toContain('Опубликованное название')
    expect(wrapper.text()).not.toContain('Черновик: новое название')
    expect(wrapper.text()).toContain('Есть неопубликованное изменение')
  })
})

describe('KnowledgeBase — «Добавить …» stages into the draft, not /kb', () => {
  it('«Добавить тему» opens the create modal (no direct /kb write on click)', async () => {
    const { wrapper, api } = await mountWith(emptyChanges(), emptyLive())
    await switchTab(wrapper, 'Темы')
    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить тему'))
    expect(addBtn).toBeTruthy()
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // Opening the modal must not itself call any write endpoint.
    expect(api.post).not.toHaveBeenCalled()
    expect(api.patch).not.toHaveBeenCalled()
    // The dialog is now open (teleported to document.body) with the topic
    // form's slug field visible.
    expect(body().find('input').exists()).toBe(true)
  })
})

describe('KnowledgeBase — delete asks for confirmation and stages a removal', () => {
  it('clicking Удалить opens a confirmation, and only stages on confirm', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))
    const stageDeleteSpy = vi.spyOn(pg, 'stageDelete').mockResolvedValue(true)
    await switchTab(wrapper, 'Темы')

    const deleteBtn = wrapper.findAll('button').find((b) => b.text() === 'Удалить')
    expect(deleteBtn).toBeTruthy()
    await deleteBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(stageDeleteSpy).not.toHaveBeenCalled() // confirmation first, no write yet
    expect(body().text()).toContain('Удалить запись?')

    const confirmBtn = body().findAll('button').find((b) => b.text() === 'Удалить' && b.classes().some((c) => c.includes('destructive')))
    expect(confirmBtn).toBeTruthy()
    await confirmBtn!.trigger('click')
    await flushPromises()

    expect(stageDeleteSpy).toHaveBeenCalledWith('topics', 'pricing')
  })
})

describe('KnowledgeBase — published data unchanged after a staged write', () => {
  it('pg.live is untouched by a successful stage (no live refetch is triggered by the modal)', async () => {
    const { wrapper, pg, api } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))
    const liveBefore = pg.live
    vi.mocked(api.post).mockResolvedValueOnce(emptyChanges({ topics: [topic({ id: 'new-topic', slug: 'new-topic', title: 'Новая' })] }))
    await switchTab(wrapper, 'Темы')

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить тему'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const slugInput = body().find('input')
    await slugInput.setValue('new-topic')
    const saveBtn = body().findAll('button').find((b) => b.text() === 'Сохранить')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(pg.live).toBe(liveBefore) // same object reference — never reassigned
    expect(wrapper.text()).toContain('Изменение добавлено в Черновик')
  })
})

describe('KnowledgeBase — singleton toolbars', () => {
  it('Контакты reads «Изменить контакты», never «Добавить»', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive())
    const contactsTab = wrapper.findAll('button').find((b) => b.text() === 'Контакты')
    await contactsTab!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Изменить контакты')
    expect(wrapper.text()).not.toContain('Добавить контакты')
  })

  it('Политики reads «Изменить политики», never «Добавить»', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive())
    const policiesTab = wrapper.findAll('button').find((b) => b.text() === 'Политики')
    await policiesTab!.trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Изменить политики')
    expect(wrapper.text()).not.toContain('Добавить политики')
  })
})
