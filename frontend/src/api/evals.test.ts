import { afterEach, describe, expect, it, vi } from 'vitest'
import { evalsApi } from './evals'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('evalsApi cache policy', () => {
  it('bypasses the browser cache for mutable JSON exports', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ generated_at: '', runs: [] }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await evalsApi.fetchRuns()

    expect(fetchMock).toHaveBeenCalledWith('/evals-data/runs.json', { cache: 'no-store' })
  })

  it('bypasses the browser cache for availability probes and prompt text', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 200 }))
      .mockResolvedValueOnce(new Response('rendered prompt', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(evalsApi.probeFile('/evals-data/report.md')).resolves.toBe(true)
    await expect(evalsApi.fetchPromptText('run-1', 'scenario-1')).resolves.toBe('rendered prompt')

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/evals-data/report.md', {
      method: 'HEAD',
      cache: 'no-store',
    })
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/evals-data/run-1/snapshots/scenario-1/prompt.txt',
      { cache: 'no-store' },
    )
  })
})
