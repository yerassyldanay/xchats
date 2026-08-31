import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { useCancelConfirm } from '@/composables/useCancelConfirm'
import { useDraftSelection } from '@/composables/useDraftSelection'
import { mountKb, testPinia } from '@/test/mount'
import DraftKnowledgeBase from './DraftKnowledgeBase.vue'
import KbIngestPanel from './KbIngestPanel.vue'
import type { DraftChangeSet, DraftView, ProductRow, TopicRow } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: {
      ...actual.api,
      get: vi.fn(),
      post: vi.fn(),
      patch: vi.fn(),
      del: vi.fn(),
      // KbIngestPanel (now always mounted here) pulls in KbImportCard,
      // which calls these directly — none of the four above are the same
      // underlying call, so each needs its own default here too.
      getKbImportProviders: vi.fn().mockResolvedValue([]),
      listKbImportRuns: vi.fn().mockResolvedValue({ runs: [], total: 0 }),
    },
  }
})
// KbIngestPanel (KB-14) reads useRoute/useRouter for its history dialog's
// URL state — no test in this file exercises that dialog, so a static,
// always-empty mock is enough (contrast KbIngestPanel.dom.test.ts's own
// mutable routeQuery, which actually drives history-dialog behavior).
vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return { ...actual, useRouter: () => ({ replace: vi.fn() }), useRoute: () => ({ query: {} }) }
})

function topic(over: Partial<TopicRow> = {}): TopicRow {
  return {
    id: 'pricing', slug: 'pricing', title: 'Тарифы', body_md: 'Текст.',
    featured_image: null, illustration_images: [], explainer_videos: [], reference_documents: [],
    draft: false, updated_at: '',
    ...over,
  }
}

function product(over: Partial<ProductRow> = {}): ProductRow {
  return {
    id: 'p1', ref: 'p1', name: 'Товар', price: '', description: '', category: '',
    in_stock: true, sales_status: 'active', featured_image: null, gallery_images: [],
    demo_videos: [], certificate_documents: [], guarantee_documents: [], draft: false, updated_at: '',
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
    // McpConnectCard (always rendered above the draft content, see
    // DraftKnowledgeBase.vue) fetches this on mount — a realistic default
    // response keeps every existing assertion below about the DRAFT
    // content, not an incidental "failed to load MCP info" banner.
    if (path === '/mcp-connection') {
      return { mcp_url: 'http://localhost:8080/mcp', auth_enabled: true, tunnel_available: false, tunnel_running: false, scopes: ['kb:read'] } as any
    }
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const wrapper = mountKb(DraftKnowledgeBase, { pinia })
  await flushPromises()
  return { wrapper, pg: usePlayground() }
}

// outsideIngestPanel filters out anything inside KbIngestPanel's subtree —
// that panel (Ссылки/Файлы staging + the ChatGPT/Claude connect card) is
// legitimately interactive now (its own "Добавить" button, url/guidance
// inputs), unlike the read-only review region below it these tests
// actually guard.
function outsideIngestPanel<T extends { element: Element }>(wrapper: ReturnType<typeof mountKb>, items: T[]): T[] {
  const panel = wrapper.findComponent(KbIngestPanel)
  return items.filter((el) => !(panel.exists() && panel.element.contains(el.element)))
}

// reka-ui's Dialog renders through a Teleport into document.body, outside
// @vue/test-utils' wrapper subtree — and previous mounts in this file are
// never unmounted, so their teleported nodes pile up there. Always take the
// LAST match, which belongs to the dialog this test just opened.
function openDialogAccept(): HTMLElement | null {
  const all = document.body.querySelectorAll('[data-testid="confirm-accept"]')
  return (all[all.length - 1] as HTMLElement | undefined) ?? null
}

// useCancelConfirm's request and useDraftSelection's set are module-level
// singletons (one dialog, one selection, per page) — without this reset they
// leak across tests in this file.
beforeEach(() => {
  testPinia()
  useCancelConfirm().close()
  useDraftSelection().clear()
  // Deliberately NOT clearing document.body: earlier mounts in this file are
  // still live components whose teleported nodes live there, and ripping
  // those out from under Vue makes its next patch throw on a null parentNode.
  // openDialogAccept()'s last-match rule is what disambiguates instead.
})

describe('DraftKnowledgeBase — empty state', () => {
  it('shows the empty state linking to /knowledge-base, and no Add button anywhere', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))

    expect(wrapper.text()).toContain('Нет неопубликованных изменений')
    expect(wrapper.find('a').exists()).toBe(true) // the stubbed RouterLink renders as <a>
    const addButtons = outsideIngestPanel(wrapper, wrapper.findAll('button')).filter((b) => /добавить/i.test(b.text()))
    expect(addButtons).toHaveLength(0)
  })

  // AC4: the review region (heading + StatTiles) must stay visible even
  // with nothing pending — DraftEmptyState now stands in for the entity
  // tabs/change lists only, never for the whole region (see
  // DraftKnowledgeBase.vue's own template comment on this v-if chain).
  it('AC4: still shows the review heading and all-zero StatTiles alongside the empty state', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))

    expect(wrapper.text()).toContain('Обзор черновика')
    expect(wrapper.text()).toContain('Добавлено')
    expect(wrapper.text()).toContain('Изменено')
    expect(wrapper.text()).toContain('Удалено')
    expect(wrapper.text()).toContain('Всего')
    const counts = wrapper.findAll('.text-3xl').map((el) => el.text())
    expect(counts).toEqual(['0', '0', '0', '0'])
  })

  it('AC1/AC4: the ingestion panel stays reachable while the draft is still loading', async () => {
    const { api } = await import('@/api/client')
    // Never resolves within this test — loading stays true throughout.
    vi.mocked(api.get).mockImplementation(() => new Promise(() => {}))
    const pinia = testPinia()
    const wrapper = mountKb(DraftKnowledgeBase, { pinia })
    await flushPromises()

    expect(wrapper.findComponent(KbIngestPanel).exists()).toBe(true)
    expect(wrapper.text()).toContain('Загрузка черновика')
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
  // The multi-select checkbox is excluded on purpose: this asserts a card
  // exposes no editor for its FIELDS (Черновик is review-only — editing goes
  // through the modal), which a selection tickbox doesn't violate.
  it('a card renders no input for its fields (read-only)', async () => {
    const { wrapper } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const fieldInputs = outsideIngestPanel(wrapper, wrapper.findAll('input')).filter(
      (i) => i.attributes('data-testid') !== 'kb-record-select'
    )
    expect(fieldInputs).toHaveLength(0)
    expect(outsideIngestPanel(wrapper, wrapper.findAll('textarea'))).toHaveLength(0)
  })

  it('Publish calls approveEntity(kind, key)', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const approveSpy = vi.spyOn(pg, 'approveEntity').mockResolvedValue(true)

    const publishBtn = wrapper.findAll('button').find((b) => b.text() === 'Опубликовать')
    expect(publishBtn).toBeTruthy()
    await publishBtn!.trigger('click')
    expect(approveSpy).toHaveBeenCalledWith('topics', 'new')
  })

  // A NEW card's cancel button says «Удалить из черновика», not «Отменить
  // изменение»: there is no published value to revert to, the record is
  // destroyed outright (records/actions.ts's REMOVE_FROM_DRAFT).
  it('a Новый card labels its cancel action as a removal', async () => {
    const { wrapper } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const labels = wrapper.findAll('button').map((b) => b.text())
    expect(labels).toContain('Удалить из черновика')
    expect(labels).not.toContain('Отменить изменение')
  })

  // Cancel is confirmed now, never immediate — one misclick used to drop a
  // freshly-imported record with no prompt and no undo.
  it('Cancel opens a confirmation and only calls cancelChange once confirmed', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const cancelSpy = vi.spyOn(pg, 'cancelChange').mockResolvedValue(true)

    const cancelBtn = wrapper.findAll('button').find((b) => b.text() === 'Удалить из черновика')
    expect(cancelBtn).toBeTruthy()
    await cancelBtn!.trigger('click')
    expect(cancelSpy, 'the store must not be touched before the operator confirms').not.toHaveBeenCalled()

    // reka-ui's Dialog renders through a Teleport, so its content lives
    // outside @vue/test-utils' wrapper subtree — query the real DOM.
    const accept = openDialogAccept()
    expect(accept, 'confirmation dialog did not open').toBeTruthy()
    accept!.click()
    await flushPromises()
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

// KB-09: the flat page-level gate message tells you SOMETHING is blocked but
// not WHERE — these structured reasons (kind/key/message from the backend's
// GateError.Reasons) let the banner point straight at the offending tab.
describe('DraftKnowledgeBase — KB-09: structured gate reasons link to their tab', () => {
  it('renders each reason\'s message, with a Fix-in-Tab link only for reasons that carry a kind', async () => {
    const { wrapper, pg } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })], products: [product()] }),
      emptyLive()
    )
    pg.gateReasons = 'publish gate failed: policy main is incomplete'
    pg.gateReasonDetails = [
      { kind: 'products', key: 'p1', message: 'available requires cost and days to be set' },
      { kind: '', key: '', message: '3 unresolved request(s) remain' },
    ]
    await wrapper.vm.$nextTick()

    const banner = wrapper.find('[data-testid="gate-error-banner"]')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('available requires cost and days to be set')
    expect(banner.text()).toContain('3 unresolved request(s) remain')
    // The flat message is the v-else fallback — once structured reasons are
    // present, it must not also render (no duplicate/contradictory text).
    expect(banner.text()).not.toContain('publish gate failed: policy main is incomplete')

    const fixLinks = wrapper.findAll('[data-testid="gate-reason-fix-link"]')
    expect(fixLinks).toHaveLength(1)
    expect(fixLinks[0].text()).toContain('Товары') // products' plural label
  })

  it('clicking a Fix-in-Tab link switches the active tab to that reason\'s kind', async () => {
    const { wrapper, pg } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })], products: [product()] }),
      emptyLive()
    )
    pg.gateReasons = 'publish gate failed'
    pg.gateReasonDetails = [{ kind: 'products', key: 'p1', message: 'available requires cost and days to be set' }]
    await wrapper.vm.$nextTick()

    await wrapper.find('[data-testid="gate-reason-fix-link"]').trigger('click')

    expect(wrapper.find('[data-testid="draft-tab-products"]').isVisible()).toBe(true)
    expect(wrapper.find('[data-testid="draft-tab-topics"]').isVisible()).toBe(false)
  })

  it('falls back to the flat message with no list or links when gateReasonDetails is empty', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.gateReasons = 'publish gate failed: policy main is incomplete'
    pg.gateReasonDetails = []
    await wrapper.vm.$nextTick()

    const banner = wrapper.find('[data-testid="gate-error-banner"]')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('publish gate failed: policy main is incomplete')
    expect(wrapper.find('[data-testid="gate-reason-fix-link"]').exists()).toBe(false)
    expect(banner.find('ul').exists()).toBe(false)
  })
})

// Multi-select: tick several pending cards and cancel them in one confirmed
// action, instead of one click (and one dialog) per card.
describe('DraftKnowledgeBase — bulk selection', () => {
  const threeTopics = () =>
    emptyChanges({
      topics: [
        topic({ id: 'a', slug: 'a', title: 'A' }),
        topic({ id: 'b', slug: 'b', title: 'B' }),
        topic({ id: 'c', slug: 'c', title: 'C' }),
      ],
    })

  it('shows no bulk bar until something is ticked', async () => {
    const { wrapper } = await mountWith(threeTopics(), emptyLive())
    expect(wrapper.find('[data-testid="draft-bulk-bar"]').exists()).toBe(false)

    await wrapper.findAll('[data-testid="kb-record-select"]')[0].setValue(true)
    expect(wrapper.find('[data-testid="draft-bulk-bar"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="draft-bulk-bar"]').text()).toContain('1')
  })

  it('cancels every selected entry after one confirmation', async () => {
    const { wrapper, pg } = await mountWith(threeTopics(), emptyLive())
    const cancelSpy = vi.spyOn(pg, 'cancelChange').mockResolvedValue(true)

    const boxes = wrapper.findAll('[data-testid="kb-record-select"]')
    await boxes[0].setValue(true)
    await boxes[2].setValue(true)

    await wrapper.find('[data-testid="bulk-cancel"]').trigger('click')
    expect(cancelSpy, 'nothing may happen before the operator confirms').not.toHaveBeenCalled()

    const accept = openDialogAccept()
    expect(accept, 'confirmation dialog did not open').toBeTruthy()
    accept!.click()
    await flushPromises()

    expect(cancelSpy).toHaveBeenCalledTimes(2)
    expect(cancelSpy).toHaveBeenCalledWith('topics', 'a')
    expect(cancelSpy).toHaveBeenCalledWith('topics', 'c')
    expect(cancelSpy, 'the unticked card must be untouched').not.toHaveBeenCalledWith('topics', 'b')
  })

  it('select-all ticks every card on the active tab, and toggles back off', async () => {
    const { wrapper } = await mountWith(threeTopics(), emptyLive())
    await wrapper.find('[data-testid="bulk-select-all"]').trigger('click')
    expect(wrapper.find('[data-testid="draft-bulk-bar"]').text()).toContain('3')

    await wrapper.find('[data-testid="bulk-select-all"]').trigger('click')
    expect(wrapper.find('[data-testid="draft-bulk-bar"]').exists()).toBe(false)
  })
})

// KB-08: both whole-draft actions are irreversible (Publish all pushes
// straight to live channels; Discard all destroys every pending change for
// good) and previously fired with zero confirmation (Publish all) or a bare
// window.confirm (Discard all) inconsistent with the rest of the app's
// styled dialogs.
describe('DraftKnowledgeBase — Publish all / Discard all require confirmation', () => {
  it('Publish all opens a styled confirmation summarizing counts, and only approves on confirm', async () => {
    const { wrapper, pg } = await mountWith(
      emptyChanges({ topics: [topic({ id: 'new', slug: 'new' }), topic({ title: 'edited' })] }),
      emptyLive({ topics: [topic()] })
    )
    const approveSpy = vi.spyOn(pg, 'approve').mockResolvedValue(true)

    await wrapper.find('[data-testid="publish-all"]').trigger('click')
    expect(approveSpy, 'no live write before the operator confirms').not.toHaveBeenCalled()

    const accept = openDialogAccept()
    expect(accept, 'confirmation dialog did not open').toBeTruthy()
    accept!.click()
    await flushPromises()

    expect(approveSpy).toHaveBeenCalledTimes(1)
  })

  it('Discard all opens a styled confirmation (no window.confirm), and only discards on confirm', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    const discardSpy = vi.spyOn(pg, 'discard').mockResolvedValue(undefined)
    const nativeConfirm = vi.spyOn(window, 'confirm')

    await wrapper.find('[data-testid="discard-all"]').trigger('click')
    expect(nativeConfirm, 'must not fall back to the native browser dialog').not.toHaveBeenCalled()
    expect(discardSpy).not.toHaveBeenCalled()

    const accept = openDialogAccept()
    expect(accept, 'confirmation dialog did not open').toBeTruthy()
    accept!.click()
    await flushPromises()

    expect(discardSpy).toHaveBeenCalledTimes(1)
    nativeConfirm.mockRestore()
  })
})

// KB-03: approveWith (both approve() and every approveEntity() call funnel
// through it) is the one choke point every publish path shares — these
// tests drive its own `approving` flag directly, the same signal
// DraftKnowledgeBase.vue's own watcher reacts to, rather than mocking a
// specific publish button's click handler.
describe('DraftKnowledgeBase — KB-03: post-publish bridge to Simulator', () => {
  it('shows a dismissible success banner linking to Simulator/Knowledge Base once a publish empties the draft', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.approving = true
    await wrapper.vm.$nextTick()
    pg.changes = emptyChanges() // the publish landed and emptied the draft
    pg.approving = false
    await wrapper.vm.$nextTick()

    const banner = wrapper.find('[data-testid="publish-success-banner"]')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('База знаний обновлена')
    expect(banner.text()).toContain('Симулятор')
    expect(banner.text()).toContain('Базу знаний')
  })

  it('does not show the banner just because the draft happened to load already empty', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive())
    expect(wrapper.find('[data-testid="publish-success-banner"]').exists()).toBe(false)
  })

  it('does not show the banner when the draft empties WITHOUT a publish (e.g. discard)', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.changes = emptyChanges() // discard's own effect — pg.approving is never touched
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="publish-success-banner"]').exists()).toBe(false)
  })

  it('does not show the banner after a FAILED publish attempt', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.approving = true
    await wrapper.vm.$nextTick()
    pg.error = 'publish failed' // the draft stays non-empty on a real failure, but guard on the error too
    pg.approving = false
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="publish-success-banner"]').exists()).toBe(false)
  })

  it('the dismiss button hides the banner', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges({ topics: [topic({ id: 'new', slug: 'new' })] }), emptyLive())
    pg.approving = true
    await wrapper.vm.$nextTick()
    pg.changes = emptyChanges()
    pg.approving = false
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="publish-success-banner"]').exists()).toBe(true)

    await wrapper.find('[data-testid="publish-success-dismiss"]').trigger('click')
    expect(wrapper.find('[data-testid="publish-success-banner"]').exists()).toBe(false)
  })
})
