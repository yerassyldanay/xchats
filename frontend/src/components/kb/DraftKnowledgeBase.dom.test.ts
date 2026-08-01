import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { api, ApiError } from '@/api/client'
import { emptyChangeSet, emptyLiveView, tariffRow, topicRow } from '@/test/fixtures'
import { mountKb } from '@/test/mount'
import DraftKnowledgeBase from './DraftKnowledgeBase.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), del: vi.fn(), upload: vi.fn(), mediaURL: vi.fn(), realtimeURL: vi.fn() },
  }
})

function mockRoutes(changes = emptyChangeSet(), live = emptyLiveView()) {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/playground/draft') return changes
    if (path === '/kb') return live
    throw new Error('unexpected GET ' + path)
  })
}

async function mountDraft() {
  const wrapper = mountKb(DraftKnowledgeBase)
  await flushPromises()
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})
afterEach(() => {
  // useKbModal's session is module-scoped (shared across every mount) —
  // reset it so one test's open dialog can never leak into the next.
  useKbModal().close()
})

describe('DraftKnowledgeBase (Черновик)', () => {
  it('renders the empty state, linking back to База знаний, when nothing is pending', async () => {
    mockRoutes()
    const wrapper = await mountDraft()

    expect(wrapper.text()).toContain('Нет неопубликованных изменений')
    const link = wrapper.find('a[href="knowledge-base"]')
    expect(link.exists()).toBe(true)
  })

  it('shows a tab only for kinds with pending changes', async () => {
    mockRoutes(emptyChangeSet({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })] }))
    const wrapper = await mountDraft()

    const tabButtons = wrapper.findAll('button').map((b) => b.text())
    expect(tabButtons.some((t) => t.includes('Тарифы'))).toBe(true)
    expect(tabButtons.some((t) => t.includes('Темы'))).toBe(false)
    expect(tabButtons.some((t) => t.includes('Товары'))).toBe(false)
  })

  it('shows no Обзор tab or card when config has no pending change', async () => {
    mockRoutes(emptyChangeSet({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })], config: null }))
    const wrapper = await mountDraft()

    const tabButtons = wrapper.findAll('button').map((b) => b.text())
    expect(tabButtons.some((t) => t.includes('Обзор'))).toBe(false)
  })

  it('renders Добавлено/Изменено/Удалено/Всего stat tiles reflecting the change set', async () => {
    mockRoutes(
      emptyChangeSet({
        topics: [topicRow({ id: 'new', slug: 'new' })],
        tariffs: [tariffRow({ id: 'r1', ref: 'r1' })],
        deletes: [{ kind: 'products', key: 'gone' }],
      }),
      emptyLiveView({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })] })
    )
    const wrapper = await mountDraft()

    expect(wrapper.text()).toContain('Добавлено')
    expect(wrapper.text()).toContain('Изменено')
    expect(wrapper.text()).toContain('Удалено')
    expect(wrapper.text()).toContain('Всего')
  })

  it('a card renders no input/textarea — it is read-only', async () => {
    mockRoutes(emptyChangeSet({ topics: [topicRow({ id: 'new-topic', slug: 'new-topic', title: 'New' })] }))
    const wrapper = await mountDraft()

    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('no Add button or create form exists anywhere on the page', async () => {
    mockRoutes(emptyChangeSet({ topics: [topicRow({ id: 'new-topic', slug: 'new-topic' })] }))
    const wrapper = await mountDraft()

    const allText = wrapper.text()
    expect(allText).not.toMatch(/Добавить/)
    expect(document.body.textContent).not.toMatch(/Добавить/)
  })

  it("clicking a card's Publish button calls approveEntity(kind, key)", async () => {
    mockRoutes(emptyChangeSet({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })] }))
    const wrapper = await mountDraft()
    const pg = usePlayground()
    vi.mocked(api.post).mockResolvedValue(emptyChangeSet())
    const spy = vi.spyOn(pg, 'approveEntity')

    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('Опубликовать') && !b.text().includes('всё'))
    expect(publishBtn).toBeTruthy()
    await publishBtn!.trigger('click')
    await flushPromises()

    expect(spy).toHaveBeenCalledWith('tariffs', 'r1')
  })

  it("clicking a card's Отменить изменение button calls cancelChange(kind, key)", async () => {
    mockRoutes(emptyChangeSet({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })] }))
    const wrapper = await mountDraft()
    const pg = usePlayground()
    vi.mocked(api.del).mockResolvedValue({ changed: true, changes: emptyChangeSet() })
    const spy = vi.spyOn(pg, 'cancelChange')

    const cancelBtn = wrapper.findAll('button').find((b) => b.text().includes('Отменить изменение'))
    expect(cancelBtn).toBeTruthy()
    await cancelBtn!.trigger('click')
    await flushPromises()

    expect(spy).toHaveBeenCalledWith('tariffs', 'r1')
  })

  it('a 422 gate error renders page-level, with only a neutral note on the card — never claiming the record itself is invalid', async () => {
    mockRoutes(emptyChangeSet({ tariffs: [tariffRow({ id: 'r1', ref: 'r1' })] }))
    const wrapper = await mountDraft()
    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('VALIDATION_ERROR', 422, 'publish gate failed: unrelated policy is invalid'))

    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('Опубликовать') && !b.text().includes('всё'))
    await publishBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('нельзя опубликовать')
    expect(wrapper.text()).toContain('publish gate failed: unrelated policy is invalid')
    // The card itself must never claim IT is invalid.
    expect(wrapper.text()).not.toContain('этот тариф')
  })

  it('cancelling the last pending config field removes the whole Обзор section', async () => {
    mockRoutes(emptyChangeSet({ config: { persona: 'P' } }))
    const wrapper = await mountDraft()
    expect(wrapper.findAll('button').some((b) => b.text().includes('Обзор'))).toBe(true)

    vi.mocked(api.del).mockResolvedValueOnce({ changed: true, changes: emptyChangeSet({ config: null }) })
    const cancelBtn = wrapper.findAll('button').find((b) => b.text() === 'Отменить изменение')
    await cancelBtn!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('button').some((b) => b.text().includes('Обзор'))).toBe(false)
  })
})
