import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import DraftKnowledgeBase from './DraftKnowledgeBase.vue'
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
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(DraftKnowledgeBase, { pinia })
  await flushPromises()
  return { wrapper, pg: usePlayground() }
}

describe('DraftKnowledgeBase — empty state', () => {
  it('shows the empty state linking to /knowledge-base, and no Add button anywhere', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))

    expect(wrapper.text()).toContain('Нет неопубликованных изменений')
    expect(wrapper.find('a').exists()).toBe(true) // the stubbed RouterLink renders as <a>
    const addButtons = wrapper.findAll('button').filter((b) => /добавить/i.test(b.text()))
    expect(addButtons).toHaveLength(0)
  })
})

describe('DraftKnowledgeBase — never mixes published with pending', () => {
  it('an unchanged published topic never appears, even though live has rows', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ topics: [topic(), topic({ id: 'other', slug: 'other', title: 'Другое' })] }))
    expect(wrapper.text()).not.toContain('Другое')
    expect(wrapper.text()).toContain('Нет неопубликованных изменений')
  })
})

describe('DraftKnowledgeBase — classification badges', () => {
  it('a newly added item shows as Новый (added)', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new-topic', slug: 'new-topic', title: 'Новая тема' })] }),
      emptyLive()
    )
    expect(wrapper.text()).toContain('Новая тема')
    expect(wrapper.text()).toContain('Новый')
  })

  it('a modified item shows as Изменён and its before value via FieldDiffNote', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ topics: [topic({ title: 'Новое название' })] }),
      emptyLive({ topics: [topic({ title: 'Старое название' })] })
    )
    expect(wrapper.text()).toContain('Изменён')
    expect(wrapper.text()).toContain('Новое название')
    expect(wrapper.text()).toContain('Старое название') // FieldDiffNote's "Было: …"
  })

  it('a staged deletion shows as На удаление, rendering the PUBLISHED record', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ deletes: [{ kind: 'topics', key: 'pricing' }] }),
      emptyLive({ topics: [topic({ title: 'Опубликованная версия' })] })
    )
    expect(wrapper.text()).toContain('На удаление')
    expect(wrapper.text()).toContain('Опубликованная версия')
  })
})

describe('DraftKnowledgeBase — tabs', () => {
  it('shows a tab only for kinds with pending changes', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }),
      emptyLive()
    )
    const tabButtons = wrapper.findAll('button').map((b) => b.text())
    expect(tabButtons.some((t) => /тем/i.test(t))).toBe(true)
    expect(tabButtons.some((t) => /тариф/i.test(t))).toBe(false)
    expect(tabButtons.some((t) => /товар/i.test(t))).toBe(false)
  })

  it('shows no Обзор tab or card when config has no pending change', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })], config: null }),
      emptyLive()
    )
    const tabButtons = wrapper.findAll('button').map((b) => b.text())
    expect(tabButtons.some((t) => /обзор/i.test(t))).toBe(false)
  })
})

describe('DraftKnowledgeBase — stat tiles', () => {
  it('shows Добавлено/Изменено/Удалено/Всего counts', async () => {
    const { wrapper } = await mountWith(
      emptyChanges({
        topics: [topic({ id: 'new', slug: 'new' }), topic({ title: 'edited' })],
        deletes: [{ kind: 'products', key: 'gone' }],
      }),
      emptyLive({
        topics: [topic()],
        products: [{ id: 'gone', ref: 'gone', name: 'T', price: '', description: '', category: '', in_stock: true, sales_status: 'active', featured_image: null, gallery_images: [], demo_videos: [], certificate_documents: [], guarantee_documents: [], draft: false, updated_at: '' }],
      })
    )
    expect(wrapper.text()).toContain('Добавлено')
    expect(wrapper.text()).toContain('Изменено')
    expect(wrapper.text()).toContain('Удалено')
    expect(wrapper.text()).toContain('Всего')
  })
})

describe('DraftKnowledgeBase — card actions wire to the store', () => {
  it('a card renders no input for its fields (read-only)', async () => {
    const { wrapper } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    expect(wrapper.findAll('input')).toHaveLength(0)
    expect(wrapper.findAll('textarea')).toHaveLength(0)
  })

  it('Publish calls approveEntity(kind, key)', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const approveSpy = vi.spyOn(pg, 'approveEntity').mockResolvedValue(true)

    const publishBtn = wrapper.findAll('button').find((b) => b.text() === 'Опубликовать')
    expect(publishBtn).toBeTruthy()
    await publishBtn!.trigger('click')
    expect(approveSpy).toHaveBeenCalledWith('topics', 'new')
  })

  it('Cancel calls cancelChange(kind, key)', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const cancelSpy = vi.spyOn(pg, 'cancelChange').mockResolvedValue(true)

    const cancelBtn = wrapper.findAll('button').find((b) => b.text() === 'Отменить изменение')
    expect(cancelBtn).toBeTruthy()
    await cancelBtn!.trigger('click')
    expect(cancelSpy).toHaveBeenCalledWith('topics', 'new')
  })
})

describe('DraftKnowledgeBase — 422 gate failure renders page-level only', () => {
  it('shows a page-level message and a neutral note on the card that triggered it, never "invalid"', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.gateReasons = 'publish gate failed: policy main is incomplete'
    pg.gateBlockedKey = 'topics:new'
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('publish gate failed: policy main is incomplete')
    expect(wrapper.text()).toContain('Публикация заблокирована другим конфликтом в Черновике')
    expect(wrapper.text().toLowerCase()).not.toContain('невалид')
  })
})
