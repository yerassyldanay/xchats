import { afterEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { usePlayground } from '@/stores/playground'
import { mountKb, testPinia } from '@/test/mount'
import KnowledgeBase from './KnowledgeBase.vue'
import ProductRecord from '@/components/kb/records/ProductRecord.vue'
import type { DraftChangeSet, DraftView, KbMaterial, ProductRow, TopicRow } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

// useKbModal's session/successCount are MODULE-level singletons shared by
// every mounted instance (see its own doc comment) — an unmounted-but-not-
// disposed wrapper from an earlier test stays reactive to them, so it would
// re-open its own Teleported dialog the instant a LATER test targets the
// same entity kind, and body()'s queries below would match the stale one.
// Track and unmount after every test so only the current test's dialog can
// ever be live in document.body.
let mounted: VueWrapper | undefined
afterEach(() => {
  mounted?.unmount()
  mounted = undefined
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
    id: 'coffee-machine', ref: 'coffee-machine', name: 'Кофемашина', price: '100000', description: '', category: '',
    in_stock: true, sales_status: 'active',
    featured_image: null, gallery_images: [], demo_videos: [], certificate_documents: [], guarantee_documents: [],
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
function material(over: Partial<KbMaterial> = {}): KbMaterial {
  return {
    id: 'mat-1', source_type: 'file', source_ref: '', blob_id: '', extracted_text: '',
    media_kind: '', status: 'ready', extraction: '{}', created_at: '2026-01-01', updated_at: '2026-01-02',
    filename: 'product-photo.png', mime_type: 'image/png', size_bytes: 204800,
    processing_status: 'parsed', customer_visibility: 'visible', visual_summary: '', transcript_text: '',
    operator_note: '', has_content: true,
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
  const wrapper = mountKb(KnowledgeBase, { pinia })
  mounted = wrapper
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
    // Every tab's body is mounted at once (v-show, not v-if — see switchTab's
    // own doc comment), so this really does scan the whole page, not just
    // the active one. /knowledge-base has no interactive/ingest surface at
    // all any more (that moved to Черновик's KbIngestPanel) — every tab
    // here (record lists, Промпт, Файлы) is read-only display.
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

// Regression test for "empty media slots are visible" (requirement 1):
// before MediaStrip dropped its v-if="list.length" root, a product with
// every media column empty rendered NO media rows at all, label included.
describe('KnowledgeBase — a product with no media still shows every media slot', () => {
  it('renders every media label (e.g. «Галерея») and the empty indicator, not nothing', async () => {
    const { wrapper } = await mountWith(emptyChanges(), emptyLive({ products: [product()] }))
    await switchTab(wrapper, 'Товары')
    expect(wrapper.text()).toContain('Галерея')
    expect(wrapper.text()).toContain('Не добавлено')
  })
})

// End-to-end wiring check for Stage 5: the picker's own behavior is unit
// tested in MediaFieldPicker.dom.test.ts — this proves the field NAMES each
// *Form.vue binds MediaFieldPicker to are actually correct end to end (a
// typo'd field prop would silently no-op rather than fail to compile,
// since MediaStrip/MediaFieldPicker only ever see a plain string).
describe('KnowledgeBase — editing a product\'s media through the picker reaches the API', () => {
  it('detaching an existing gallery image and saving sends gallery_images: []', async () => {
    const { wrapper, api } = await mountWith(
      emptyChanges(),
      emptyLive({
        products: [product({ gallery_images: ['img-1'] })],
        materials: [material({ id: 'img-1', filename: 'existing.png', mime_type: 'image/png' })],
      })
    )
    vi.mocked(api.post).mockResolvedValueOnce(emptyChanges())
    await switchTab(wrapper, 'Товары')

    // KnowledgeBase.vue keeps every tab's content mounted at once (v-show,
    // not v-if — only the toolbar action itself is conditional), so ALL
    // «Изменить» buttons coexist in the DOM regardless of the active tab,
    // including Обзор's config field cards. Scope to ProductRecord
    // specifically rather than a bare text match on the whole page.
    const productCard = wrapper.findComponent(ProductRecord)
    expect(productCard.exists()).toBe(true)
    const editBtn = productCard.findAll('button').find((b) => b.text() === 'Изменить')
    expect(editBtn).toBeTruthy()
    await editBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    // The picker shows the seeded gallery image as a real thumbnail, not
    // the empty state.
    const img = body().find('img[src*="/kb/materials/img-1/content"]')
    expect(img.exists()).toBe(true)

    const detachBtn = body().findAll('button').find((b) => b.attributes('aria-label') === 'Открепить')
    expect(detachBtn).toBeTruthy()
    await detachBtn!.trigger('click')

    const saveBtn = body().findAll('button').find((b) => b.text() === 'Сохранить')
    await saveBtn!.trigger('click')
    await flushPromises()

    // /knowledge-base writes commit straight to the live KB (KB-13) — no
    // /playground/draft detour, no If-Match (a live write has no staleness
    // concept to guard against).
    expect(api.post).toHaveBeenCalledWith('/kb/products', expect.objectContaining({ ref: 'coffee-machine', gallery_images: [] }))
    // This file's api mocks are module-level vi.fn()s with no shared
    // beforeEach reset — clear this call now so it can't bleed into a
    // LATER test's `expect(api.post).not.toHaveBeenCalled()` (file order,
    // not describe-block order, is what matters for vi.fn() call history).
    vi.mocked(api.post).mockClear()
  })
})

describe('KnowledgeBase — «Добавить …» opens the modal, writes only on Save', () => {
  it('«Добавить тему» opens the create modal (no write on click alone)', async () => {
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

    // useKbModal's session is a module-level singleton shared by every
    // mounted page (see its own doc comment), and this file never unmounts
    // a wrapper between tests — leaving this dialog open would leak a
    // draft-target 'topics' session into whichever test runs next. Close it
    // the same way a user would (Cancel), matching how every other test in
    // this file that opens a dialog also resolves it before finishing.
    const cancelBtn = body().findAll('button').find((b) => b.text() === 'Отмена')
    await cancelBtn!.trigger('click')
  })
})

describe('KnowledgeBase — delete asks for confirmation and writes live only on confirm', () => {
  it('clicking Удалить opens a confirmation, and only deletes on confirm', async () => {
    const { wrapper, pg } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))
    const deleteLiveSpy = vi.spyOn(pg, 'deleteLiveEntity').mockResolvedValue(true)
    await switchTab(wrapper, 'Темы')

    const deleteBtn = wrapper.findAll('button').find((b) => b.text() === 'Удалить')
    expect(deleteBtn).toBeTruthy()
    await deleteBtn!.trigger('click')
    await wrapper.vm.$nextTick()

    expect(deleteLiveSpy).not.toHaveBeenCalled() // confirmation first, no write yet
    expect(body().text()).toContain('Удалить запись?')

    const confirmBtn = body().findAll('button').find((b) => b.text() === 'Удалить' && b.classes().some((c) => c.includes('destructive')))
    expect(confirmBtn).toBeTruthy()
    await confirmBtn!.trigger('click')
    await flushPromises()

    expect(deleteLiveSpy).toHaveBeenCalledWith('topics', 'pricing')
  })
})

// KB-13: /knowledge-base is the sole MANUAL authoring surface — a Save here
// commits straight to ai_* (no draft/publish detour), so pg.live is
// reassigned from the write's own response and the record is visible
// immediately, with a transient "Saved" confirmation instead of the old
// "staged, go publish" banner.
describe('KnowledgeBase — a manual Save writes straight to the live KB', () => {
  it('pg.live is reassigned from POST /kb/topics\' response and a Saved confirmation shows', async () => {
    const { wrapper, pg, api } = await mountWith(emptyChanges(), emptyLive({ topics: [topic()] }))
    const liveBefore = pg.live
    const newLive = emptyLive({ topics: [topic(), topic({ id: 'new-topic', slug: 'new-topic', title: 'Новая' })] })
    vi.mocked(api.post).mockResolvedValueOnce(newLive as any)
    await switchTab(wrapper, 'Темы')

    const addBtn = wrapper.findAll('button').find((b) => b.text().includes('Добавить тему'))
    await addBtn!.trigger('click')
    await wrapper.vm.$nextTick()
    const slugInput = body().find('input')
    await slugInput.setValue('new-topic')
    const saveBtn = body().findAll('button').find((b) => b.text() === 'Сохранить')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(api.post).toHaveBeenCalledWith('/kb/topics', expect.objectContaining({ slug: 'new-topic' }))
    expect(pg.live).not.toBe(liveBefore) // reassigned from the write's response
    // Pinia wraps a newly assigned object in its own reactive proxy, so this
    // is never the SAME reference as newLive — deep-equal is the right check.
    expect(pg.live).toEqual(newLive)
    expect(wrapper.text()).toContain('Сохранено в базе знаний')
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

// Файлы (материалы): GET /kb already returns the whole materials table
// (pg.live.materials) — no separate GET /kb/materials call, no lazy
// fetch-on-tab-open. See KnowledgeBase.vue's own doc comment on `materials`.
describe('KnowledgeBase — Файлы (материалы) reads pg.live.materials with no extra fetch', () => {
  it('renders filename, size, and a ready image as a thumbnail with no GET /kb/materials call', async () => {
    const { wrapper, api } = await mountWith(emptyChanges(), emptyLive({ materials: [material()] }))
    await switchTab(wrapper, 'Файлы (материалы)')

    expect(wrapper.text()).toContain('product-photo.png')
    expect(wrapper.text()).toContain('204.8 KB')
    const img = wrapper.find('img[src*="/kb/materials/mat-1/content"]')
    expect(img.exists()).toBe(true)
    expect(api.get).not.toHaveBeenCalledWith('/kb/materials')
  })

  it('shows a processing placeholder (no thumbnail, no download link) while has_content is false', async () => {
    const { wrapper } = await mountWith(
      emptyChanges(),
      emptyLive({ materials: [material({ id: 'mat-2', has_content: false, processing_status: 'uploaded' })] })
    )
    await switchTab(wrapper, 'Файлы (материалы)')

    expect(wrapper.find('img[src*="/kb/materials/mat-2/content"]').exists()).toBe(false)
    expect(wrapper.find('a[href*="/kb/materials/mat-2/content"]').exists()).toBe(false)
  })

  it('a document material shows a download link instead of a thumbnail', async () => {
    const { wrapper } = await mountWith(
      emptyChanges(),
      emptyLive({ materials: [material({ id: 'mat-3', filename: 'terms.pdf', mime_type: 'application/pdf' })] })
    )
    await switchTab(wrapper, 'Файлы (материалы)')

    expect(wrapper.text()).toContain('terms.pdf')
    const link = wrapper.find('a[href*="/kb/materials/mat-3/content"]')
    expect(link.exists()).toBe(true)
    expect(link.text()).toContain('Скачать')
  })
})
