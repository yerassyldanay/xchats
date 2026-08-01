import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePlayground } from './playground'
import { api, ApiError } from '@/api/client'
import { emptyChangeSet } from '@/test/fixtures'

// Partial mock: keep the REAL ApiError class (the store's own catch blocks
// do `e instanceof ApiError`, so a fully auto-mocked class would break that
// check) and only replace api.* with controllable mocks.
vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      del: vi.fn(),
      upload: vi.fn(),
      mediaURL: vi.fn(),
      realtimeURL: vi.fn(),
    },
  }
})

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('usePlayground', () => {
  it('ifMatch() sends base_version from the change set', () => {
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 42 })
    expect(pg.ifMatch()).toEqual({ 'If-Match': '42' })
  })

  it('ifMatch() is empty with no change set loaded yet', () => {
    const pg = usePlayground()
    expect(pg.ifMatch()).toEqual({})
  })

  it('a DRAFT_STALE write sets error and draftStale, and reloads the change set', async () => {
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 1 })
    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('DRAFT_STALE', 409, 'draft changed since you loaded it'))
    vi.mocked(api.get).mockResolvedValueOnce(emptyChangeSet({ base_version: 99 }))

    const result = await pg.upsertTopic({ slug: 't', title: 'T', body_md: 'B' })

    expect(result).toBeUndefined()
    expect(pg.draftStale).toBe(true)
    expect(pg.error).not.toBe('')
    expect(pg.changes?.base_version).toBe(99)
  })

  it('a non-stale write error sets error but does NOT set draftStale', async () => {
    const pg = usePlayground()
    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('VALIDATION_ERROR', 400, 'slug required'))

    await pg.upsertTopic({ slug: '', title: '', body_md: '' })

    expect(pg.draftStale).toBe(false)
    expect(pg.error).toBe('slug required')
  })

  it('cancelChange DELETEs the right path with If-Match and returns res.changed', async () => {
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 5 })
    vi.mocked(api.del).mockResolvedValueOnce({ changed: true, changes: emptyChangeSet({ base_version: 6 }) })

    const changed = await pg.cancelChange('topics', 'slug-1')

    expect(changed).toBe(true)
    expect(api.del).toHaveBeenCalledWith('/playground/draft/changes/topics/slug-1', { 'If-Match': '5' })
    expect(pg.changes?.base_version).toBe(6)
  })

  it('a repeated cancelChange returns changed:false and leaves base_version unchanged', async () => {
    const pg = usePlayground()
    pg.changes = emptyChangeSet({ base_version: 5 })
    vi.mocked(api.del).mockResolvedValueOnce({ changed: false, changes: emptyChangeSet({ base_version: 5 }) })

    const changed = await pg.cancelChange('topics', 'already-gone')

    expect(changed).toBe(false)
    expect(pg.changes?.base_version).toBe(5)
  })

  it('cancelChange returns false (without throwing) when the request itself fails', async () => {
    const pg = usePlayground()
    vi.mocked(api.del).mockRejectedValueOnce(new ApiError('VALIDATION_ERROR', 400, 'kind must be …'))

    const changed = await pg.cancelChange('topics', 'x')

    expect(changed).toBe(false)
    expect(pg.error).not.toBe('')
  })

  it('a 422 on approveEntity sets page-level gateReasons and clears publishingKey', async () => {
    const pg = usePlayground()
    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('VALIDATION_ERROR', 422, 'publish gate failed: topic body must be pure prose'))

    const ok = await pg.approveEntity('topics', 'slug-1')

    expect(ok).toBe(false)
    expect(pg.gateReasons).toContain('publish gate failed')
    expect(pg.publishingKey).toBe('')
    expect(pg.approving).toBe(false)
  })

  it('approveEntity sets publishingKey to `${kind}:${key}` while the call is in flight', async () => {
    const pg = usePlayground()
    let capturedDuringCall = ''
    vi.mocked(api.post).mockImplementationOnce(async () => {
      capturedDuringCall = pg.publishingKey
      return emptyChangeSet()
    })

    await pg.approveEntity('tariffs', 'ref-1')

    expect(capturedDuringCall).toBe('tariffs:ref-1')
    expect(pg.publishingKey).toBe('')
  })

  it('stageChange dispatches to the right upsert action per kind', async () => {
    const pg = usePlayground()
    vi.mocked(api.post).mockResolvedValue(emptyChangeSet())

    await pg.stageChange('tariffs', { ref: 'r1', name: 'N', price: '100', limit_text: '', fee: '', summary: '', pricing_type: 'fixed', advantages: '', disadvantages: '', sales_status: 'active' })

    expect(api.post).toHaveBeenCalledWith('/playground/draft/tariffs', expect.objectContaining({ ref: 'r1' }), {})
  })

  it('stageChange bumps lastStagedKind/lastStagedAt only on success', async () => {
    const pg = usePlayground()
    vi.mocked(api.post).mockResolvedValueOnce(emptyChangeSet())
    const before = pg.lastStagedAt

    const ok = await pg.stageChange('topics', { slug: 't', title: 'T', body_md: 'B' })

    expect(ok).toBe(true)
    expect(pg.lastStagedKind).toBe('topics')
    expect(pg.lastStagedAt).toBe(before + 1)
  })

  it('stageChange does not bump lastStaged on failure', async () => {
    const pg = usePlayground()
    vi.mocked(api.post).mockRejectedValueOnce(new ApiError('VALIDATION_ERROR', 400, 'bad'))
    const before = pg.lastStagedAt

    const ok = await pg.stageChange('topics', { slug: '', title: '', body_md: '' })

    expect(ok).toBe(false)
    expect(pg.lastStagedAt).toBe(before)
  })

  it('stageDelete dispatches to the right delete action per kind', async () => {
    const pg = usePlayground()
    vi.mocked(api.del).mockResolvedValue(emptyChangeSet())

    await pg.stageDelete('products', 'ref-1')

    expect(api.del).toHaveBeenCalledWith('/playground/draft/products/ref-1', {})
  })

  it('pendingTotal counts rows, deletes, and non-null config fields', () => {
    const pg = usePlayground()
    pg.changes = emptyChangeSet({
      topics: [{ id: 't' } as any],
      deletes: [{ kind: 'tariffs', key: 'r1' }],
      config: { persona: 'P', mission: null },
    })
    expect(pg.pendingTotal).toBe(1 /* topic */ + 1 /* delete */ + 1 /* persona */)
  })

  it('pendingTotal is 0 with no change set loaded', () => {
    const pg = usePlayground()
    expect(pg.pendingTotal).toBe(0)
  })
})
