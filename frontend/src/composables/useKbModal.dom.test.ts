import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { usePlayground } from '@/stores/playground'
import { useKbModal } from '@/composables/useKbModal'
import { api, ApiError } from '@/api/client'
import { emptyChangeSet, topicRow } from '@/test/fixtures'
import { mountKb, bodyWrapper } from '@/test/mount'
import TopicForm from '@/components/kb/forms/TopicForm.vue'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), del: vi.fn(), upload: vi.fn(), mediaURL: vi.fn(), realtimeURL: vi.fn() },
  }
})

beforeEach(() => vi.clearAllMocks())
afterEach(() => useKbModal().close())

// TopicForm's dialog content is Teleported to document.body (reka-ui's
// DialogPortal) — it lives outside the mounted wrapper's own subtree, so
// every query below goes through bodyWrapper(), not `wrapper` itself.
function titleInput() {
  return bodyWrapper()
    .findAll('input')
    .filter((i) => (i.element as HTMLInputElement).type !== 'checkbox')[1]
}

// Close the modal (collapsing reka-ui's Presence/transition state) and let
// its now-pending exit-transition teardown flush BEFORE @vue/test-utils'
// end-of-test auto-unmount tears down the component tree itself — doing
// both at once is what the "Cannot read properties of null" teardown
// errors above came from.
async function closeAndSettle(modal: ReturnType<typeof useKbModal>) {
  modal.close()
  await flushPromises()
}

describe('useKbModal — stable snapshots under SSE (§2.6)', () => {
  it('typed text and a deep-copied media array survive the underlying row object being mutated afterward', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 1 })
    const row = topicRow({ id: 'x', slug: 'x', title: 'Original', illustration_images: ['img-1'] })
    const modal = useKbModal()
    modal.openEdit('topics', row)

    mountKb(TopicForm, { pinia })
    await flushPromises()
    const input = titleInput()
    await input.setValue('Typed by the user')

    // Simulate a reload mutating the ORIGINAL row in place (the worst case a
    // shallow spread would fail on) — openEdit's structuredClone must have
    // already made the form's own copy immune to this.
    row.title = 'Reloaded from server'
    row.illustration_images.push('img-2')

    expect((input.element as HTMLInputElement).value).toBe('Typed by the user')
    const snapshot = modal.session.value?.snapshot as typeof row
    expect(snapshot.illustration_images).toEqual(['img-1'])
    expect(snapshot.title).toBe('Original')

    await closeAndSettle(modal)
  })

  it('submitting against a stale version keeps the dialog open with typed values intact, and sets stale', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 5 })
    const modal = useKbModal()
    modal.openEdit('topics', topicRow({ id: 'x', slug: 'x', title: 'Original' }))

    mountKb(TopicForm, { pinia })
    await flushPromises()
    const input = titleInput()
    await input.setValue('Edited text')

    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('DRAFT_STALE', 409, 'draft changed since you loaded it'))
    vi.mocked(api.get).mockResolvedValueOnce(emptyChangeSet({ base_version: 6 }))

    const saveBtn = bodyWrapper()
      .findAll('button')
      .find((b) => b.text() === 'Сохранить')
    await saveBtn!.trigger('click')
    await flushPromises()

    expect(modal.isOpen.value).toBe(true)
    expect((input.element as HTMLInputElement).value).toBe('Edited text')
    expect(modal.stale.value).toBe(true)
    expect(pg.changes?.base_version).toBe(6) // write()'s own auto-reload already picked up the fresh version

    await closeAndSettle(modal)
  })

  it('reloadAndRetry resubmits the SAME typed buffer against the fresh version and succeeds without re-typing', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 6 }) // already past the stale moment
    const modal = useKbModal()
    modal.openEdit('topics', topicRow({ id: 'x', slug: 'x', title: 'Original' }))

    mountKb(TopicForm, { pinia })
    await flushPromises()
    const input = titleInput()
    await input.setValue('Edited text')

    vi.mocked(api.get).mockResolvedValueOnce(emptyChangeSet({ base_version: 6 }))
    vi.mocked(api.post).mockResolvedValueOnce(emptyChangeSet({ base_version: 7, topics: [topicRow({ id: 'x', slug: 'x', title: 'Edited text' })] }))

    // reloadAndRetry is only reachable once `stale` is true — drive it
    // directly rather than re-deriving the stale trigger a second time.
    const ok = await modal.reloadAndRetry({ slug: 'x', title: 'Edited text', body_md: '' })

    expect(ok).toBe(true)
    expect(modal.isOpen.value).toBe(false)
    expect(api.post).toHaveBeenCalledWith('/playground/draft/topics', { slug: 'x', title: 'Edited text', body_md: '' }, { 'If-Match': '6' })

    await closeAndSettle(modal)
  })
})
