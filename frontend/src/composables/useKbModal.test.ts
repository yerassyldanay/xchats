import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { ApiError, api } from '@/api/client'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from './useKbModal'
import type { TopicRow } from '@/types'
import type { DraftChangeSet } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { ...actual.api, get: vi.fn(), post: vi.fn(), patch: vi.fn(), del: vi.fn() },
  }
})

function emptyChanges(over: Partial<DraftChangeSet> = {}): DraftChangeSet {
  return {
    base_version: 1, updated_at: '', config: null,
    topics: [], tariffs: [], products: [], contacts: [], policies: [], tariff_info: [], zones: [], deletes: [],
    ...over,
  }
}

const topic: TopicRow = {
  id: 'pricing', slug: 'pricing', title: 'Тарифы', body_md: 'Текст.',
  featured_image: null, illustration_images: ['img-1'], explainer_videos: [], reference_documents: [],
  draft: false, updated_at: '',
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.mocked(api.get).mockReset()
  vi.mocked(api.post).mockReset()
  vi.mocked(api.patch).mockReset()
  vi.mocked(api.del).mockReset()
})

describe('useKbModal — deep copy on openEdit', () => {
  it('snapshot is unaffected by later mutation of the original row (structuredClone, not a shallow reference)', () => {
    const modal = useKbModal()
    modal.openEdit('topics', topic)

    // Mutate the ORIGINAL row's array field after the snapshot was taken.
    topic.illustration_images.push('img-2')

    const snap = modal.session.value?.snapshot as TopicRow
    expect(snap.illustration_images).toEqual(['img-1'])
  })

  // Regression: a row read off real Pinia state (pg.live.topics[i], etc.) is
  // a Vue reactive Proxy, not a plain object — structuredClone() throws
  // "DataCloneError: could not be cloned" on a Proxy directly. This only
  // surfaced against a live backend (real Pinia-reactive rows); every other
  // test in this file uses a plain literal, which never exercised the bug.
  it('does not throw on a REACTIVE row (e.g. straight off Pinia state), unlike a bare structuredClone would', () => {
    const modal = useKbModal()
    const reactiveTopic = reactive({ ...topic })

    expect(() => modal.openEdit('topics', reactiveTopic)).not.toThrow()
    const snap = modal.session.value?.snapshot as TopicRow
    expect(snap.title).toBe(topic.title)
    expect(snap.illustration_images).toEqual(topic.illustration_images)
  })

  it('session id is stable and unrelated to the store row reference — a background reload never remounts it', () => {
    const pg = usePlayground()
    const modal = useKbModal()
    modal.openEdit('topics', topic)
    const sessionId = modal.session.value?.id

    // Simulate an SSE-driven reload replacing the store's object wholesale.
    pg.setChanges(emptyChanges({ topics: [{ ...topic, title: 'Изменено извне' }] }))

    expect(modal.session.value?.id).toBe(sessionId)
    expect((modal.session.value?.snapshot as TopicRow).title).toBe('Тарифы')
  })
})

describe('useKbModal — stale submit', () => {
  it('a DRAFT_STALE response keeps the session open, sets stale, and preserves the snapshot', async () => {
    const modal = useKbModal()
    modal.openEdit('topics', topic)

    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('DRAFT_STALE', 409, 'draft changed'))
    vi.mocked(api.get).mockResolvedValueOnce(emptyChanges({ base_version: 2 }))

    const ok = await modal.submit({ kind: 'topics', slug: 'pricing', title: 'Новое название', body_md: 'Текст.' })

    expect(ok).toBe(false)
    expect(modal.stale.value).toBe(true)
    expect(modal.isOpen.value).toBe(true)
    expect((modal.session.value?.snapshot as TopicRow).title).toBe('Тарифы')
  })

  it('reloadAndRetry resubmits and closes on success, without the caller re-typing anything', async () => {
    const modal = useKbModal()
    modal.openEdit('topics', topic)

    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('DRAFT_STALE', 409, 'draft changed'))
    vi.mocked(api.get).mockResolvedValueOnce(emptyChanges({ base_version: 2 }))
    const payload = { kind: 'topics' as const, slug: 'pricing', title: 'Новое название', body_md: 'Текст.' }
    await modal.submit(payload)
    expect(modal.stale.value).toBe(true)

    vi.mocked(api.post).mockResolvedValueOnce(emptyChanges({ base_version: 3, topics: [] }))
    const ok = await modal.reloadAndRetry(payload)

    expect(ok).toBe(true)
    expect(modal.isOpen.value).toBe(false)
    expect(modal.stale.value).toBe(false)
  })
})

describe('useKbModal — openCreate', () => {
  it('starts with no snapshot and mode create', () => {
    const modal = useKbModal()
    modal.openCreate('products')
    expect(modal.session.value?.mode).toBe('create')
    expect(modal.session.value?.snapshot).toBeNull()
  })

  it('defaults target to draft when unspecified (Черновик callers never pass one)', () => {
    const modal = useKbModal()
    modal.openCreate('products')
    expect(modal.session.value?.target).toBe('draft')
  })
})

// KB-13: /knowledge-base passes target:'live' explicitly, which routes
// submit() through the store's writeLiveChange (POST /kb/*) instead of
// stageChange (POST /playground/draft/*) — no If-Match, no draft staging.
describe('useKbModal — target: live (KB-13, /knowledge-base manual writes)', () => {
  it('openEdit/openCreate record target live on the session', () => {
    const modal = useKbModal()
    modal.openEdit('topics', topic, { target: 'live' })
    expect(modal.session.value?.target).toBe('live')

    modal.openCreate('products', { target: 'live' })
    expect(modal.session.value?.target).toBe('live')
  })

  it('submit() on a live session posts to /kb/* directly, with no If-Match header', async () => {
    const modal = useKbModal()
    modal.openEdit('topics', topic, { target: 'live' })

    vi.mocked(api.post).mockResolvedValueOnce({ topics: [topic] } as any)
    const ok = await modal.submit({ kind: 'topics', slug: 'pricing', title: 'Новое название', body_md: 'Текст.' })

    expect(ok).toBe(true)
    expect(api.post).toHaveBeenCalledWith('/kb/topics', expect.objectContaining({ slug: 'pricing' }))
    expect(api.post).not.toHaveBeenCalledWith('/playground/draft/topics', expect.anything(), expect.anything())
    expect(modal.isOpen.value).toBe(false)
  })
})
