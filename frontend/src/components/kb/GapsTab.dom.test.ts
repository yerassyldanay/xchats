import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import KnowledgeBase from '@/views/KnowledgeBase.vue'
import type { DraftChangeSet, DraftView, KbGapReport } from '@/types'

// Same mocking shape as KnowledgeBase.dom.test.ts (this tab only ever mounts
// inside that page's tab strip).
vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

let mounted: VueWrapper | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
})

function emptyLive(): DraftView {
  return {
    config: {
      organization_id: 'org-1', persona: '', mission: '', guardrails: '', language_policy: '',
      reply_max_words: 120, draft: false, base_version: 0, updated_at: '',
    },
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], materials: [], requests: [],
  }
}
function emptyChanges(): DraftChangeSet {
  return {
    base_version: 1, updated_at: '', config: null,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
  }
}

function zeroReport(): KbGapReport {
  return {
    counts: [
      { reason_code: 'missing_entity', count: 0 },
      { reason_code: 'missing_field', count: 0 },
      { reason_code: 'ambiguous_entity', count: 0 },
      { reason_code: 'conflicting_kb_data', count: 0 },
    ],
    operational_counts: [
      { reason_code: 'unsupported_request', count: 0 },
      { reason_code: 'human_requested', count: 0 },
      { reason_code: 'engine_error', count: 0 },
      { reason_code: 'other', count: 0 },
    ],
    top_target_entities: [],
    top_missing_fields: [],
    recent: [],
  }
}

function seededReport(): KbGapReport {
  return {
    counts: [
      { reason_code: 'missing_entity', count: 0 },
      { reason_code: 'missing_field', count: 2 },
      { reason_code: 'ambiguous_entity', count: 0 },
      { reason_code: 'conflicting_kb_data', count: 0 },
    ],
    operational_counts: [
      { reason_code: 'unsupported_request', count: 1 },
      { reason_code: 'human_requested', count: 0 },
      { reason_code: 'engine_error', count: 0 },
      { reason_code: 'other', count: 0 },
    ],
    top_target_entities: [{ target_entity_type: 'product', target_entity_ref: 'coffee-machine', count: 2 }],
    top_missing_fields: [{ target_entity_type: 'product', field_name: 'price', count: 2 }],
    recent: [
      {
        id: 'evt-1', channel: 'whatsapp', chat_id: 'chat-1', draft_id: 'draft-1',
        reason_code: 'missing_field', target_entity_type: 'product', target_entity_ref: 'coffee-machine',
        missing_fields: ['price'], escalation_reason: 'нет цены', source: 'model', created_at: '2026-01-01T10:00:00Z',
      },
    ],
  }
}

async function mountWithGaps(report: KbGapReport) {
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/playground/draft') return emptyChanges() as any
    if (path === '/kb') return emptyLive() as any
    if (path.startsWith('/kb/gaps')) return report as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(KnowledgeBase, { pinia })
  mounted = wrapper
  await flushPromises()
  return { wrapper, pg: usePlayground(), api }
}

async function switchTab(wrapper: VueWrapper, label: string) {
  const tabBtn = wrapper.findAll('button').find((b) => b.text() === label)
  if (!tabBtn) throw new Error(`no tab button labelled ${label}`)
  await tabBtn.trigger('click')
  await wrapper.vm.$nextTick()
}

describe('GapsTab', () => {
  it('loads the report on first open and shows zero-filled content-gap counts', async () => {
    const { wrapper, api } = await mountWithGaps(zeroReport())
    await switchTab(wrapper, 'Пробелы в базе')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith('/kb/gaps')
    const tiles = wrapper.findAll('[data-testid="gaps-count-tile"]')
    expect(tiles).toHaveLength(4)
    for (const tile of tiles) expect(tile.text()).toContain('0')

    // No rollup rows to rank yet — the section stays hidden rather than
    // showing two empty lists.
    expect(wrapper.find('[data-testid="gaps-top-entity-row"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="gaps-top-field-row"]').exists()).toBe(false)
  })

  it('ranks the top gap-causing entity and field, answering "which one" a count alone cannot', async () => {
    const { wrapper } = await mountWithGaps(seededReport())
    await switchTab(wrapper, 'Пробелы в базе')
    await flushPromises()

    const entityRows = wrapper.findAll('[data-testid="gaps-top-entity-row"]')
    expect(entityRows).toHaveLength(1)
    expect(entityRows[0]?.text()).toContain('coffee-machine')
    expect(entityRows[0]?.text()).toContain('2')

    const fieldRows = wrapper.findAll('[data-testid="gaps-top-field-row"]')
    expect(fieldRows).toHaveLength(1)
    expect(fieldRows[0]?.text()).toContain('price')
    expect(fieldRows[0]?.text()).toContain('2')
  })

  it('renders a seeded event with its diagnostic fields, never a draft/message-text column', async () => {
    const { wrapper } = await mountWithGaps(seededReport())
    await switchTab(wrapper, 'Пробелы в базе')
    await flushPromises()

    expect(wrapper.text()).toContain('Не хватает поля') // missing_field label
    const rows = wrapper.findAll('[data-testid="gaps-event-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]?.text()).toContain('coffee-machine')
    expect(rows[0]?.text()).toContain('price')
    expect(rows[0]?.text()).toContain('нет цены')

    // Operational codes stay out of the default (content-gap) tiles...
    const tileTexts = wrapper.findAll('[data-testid="gaps-count-tile"]').map((t) => t.text())
    expect(tileTexts.some((t) => t.includes('Запрос не поддерживается'))).toBe(false)
    // ...but remain visible, distinguishable, elsewhere on the page.
    expect(wrapper.text()).toContain('Запрос не поддерживается')

    // The report never carries a draft_text/message-body field at all — the
    // row's own text is built ONLY from known diagnostic columns.
    expect(wrapper.html()).not.toContain('draft_text')
  })

  it('Apply sends the chosen filters as GET /kb/gaps query params', async () => {
    const { wrapper, api } = await mountWithGaps(zeroReport())
    await switchTab(wrapper, 'Пробелы в базе')
    await flushPromises()
    vi.mocked(api.get).mockClear()

    await wrapper.get('[data-testid="gaps-filter-entity-ref"]').setValue('coffee-machine')
    await wrapper.get('[data-testid="gaps-filter-apply"]').trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledTimes(1)
    const calledPath = vi.mocked(api.get).mock.calls[0]?.[0] as string
    expect(calledPath).toContain('entity_ref=coffee-machine')
  })

  // P2 finding: pg.gapsReport stays non-null after a first success, so a
  // LATER failed reload must not just silently keep showing the old report
  // with no indication anything went wrong.
  it('flags a failed reload after an initial success instead of silently keeping stale data', async () => {
    const { wrapper, api } = await mountWithGaps(seededReport())
    await switchTab(wrapper, 'Пробелы в базе')
    await flushPromises()
    expect(wrapper.find('[data-testid="gaps-retry-after-stale"]').exists()).toBe(false)

    vi.mocked(api.get).mockRejectedValueOnce(new Error('boom'))
    await wrapper.get('[data-testid="gaps-filter-apply"]').trigger('click')
    await flushPromises()

    // The error is now visible, with a way to retry...
    const retry = wrapper.find('[data-testid="gaps-retry-after-stale"]')
    expect(retry.exists()).toBe(true)
    expect(wrapper.text()).toContain('Не удалось загрузить пробелы')
    // ...but the PREVIOUS (now stale) report stays on screen rather than
    // vanishing into a blank/error-only state.
    const rows = wrapper.findAll('[data-testid="gaps-event-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]?.text()).toContain('coffee-machine')
  })
})
