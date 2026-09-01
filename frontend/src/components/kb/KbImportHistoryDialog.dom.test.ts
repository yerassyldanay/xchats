import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useKbImport } from '@/stores/kbImport'
import { useAuth } from '@/stores/auth'
import KbImportHistoryDialog from './KbImportHistoryDialog.vue'
import type { KbImportRun } from '@/types'

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: {
      ...actual.api,
      get: vi.fn(),
      listKbImportRuns: vi.fn(),
      getKbImportRun: vi.fn(),
    },
  }
})

function run(over: Partial<KbImportRun> = {}): KbImportRun {
  return {
    run_id: 'run-1',
    status: 'built',
    started_by: 'user-1',
    started_at: '2026-08-30T10:00:00Z',
    finished_at: '2026-08-30T10:05:00Z',
    cancelable: false,
    materials: [{ id: 'm-1', kind: 'url', label: 'https://vendor.example/a', handle: 'evidence.1', processing_status: 'parsed' }],
    ...over,
  }
}

// The dialog teleports into document.body — unmount at the end of every
// test (real Vue teardown, unlike ripping out document.body by hand) so
// the next test's queries never see a previous test's leftover nodes.
let mounted: ReturnType<typeof mountKb> | null = null
afterEach(() => {
  mounted?.unmount()
  mounted = null
})

async function mountDialog(props: { open: boolean; initialRunId?: string }) {
  const { api } = await import('@/api/client')
  // Call history is NOT reset between tests by default in this project —
  // every mountDialog() call starts clean so a test's toHaveBeenCalledWith
  // (or not.toHaveBeenCalled) assertion only ever sees ITS OWN calls, not
  // an earlier test's leftover history on the same shared vi.fn().
  vi.mocked(api.listKbImportRuns).mockClear()
  vi.mocked(api.getKbImportRun).mockClear()
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.startsWith('/users')) return { items: [] } as any
    throw new Error(`unexpected GET ${path}`)
  })
  const pinia = testPinia()
  const auth = useAuth()
  auth.user = { id: 'user-2', email: 'me@example.com', name: 'Me', role: 'admin', must_change_password: false }
  const wrapper = mountKb(KbImportHistoryDialog, { pinia, props })
  mounted = wrapper
  await flushPromises()
  return { wrapper, kbi: useKbImport() }
}

describe('KbImportHistoryDialog — list view (KB-14)', () => {
  it('renders each run\'s status badge and a sources summary', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.listKbImportRuns).mockResolvedValueOnce({
      runs: [run({ run_id: 'r1', status: 'built' }), run({ run_id: 'r2', status: 'failed', materials: [{ id: 'm-2', kind: 'file', label: 'b.pdf', handle: 'upload.1', processing_status: 'failed' }] })],
      total: 2,
    })

    await mountDialog({ open: true })

    const rows = document.body.querySelectorAll('[data-testid="history-row"]')
    expect(rows).toHaveLength(2)
    expect(document.body.textContent).toContain('Готово') // built
    expect(document.body.textContent).toContain('Ошибка') // failed
    expect(document.body.textContent).toContain('https://vendor.example/a')
    expect(document.body.textContent).toContain('b.pdf')
  })

  it('shows an empty state when the org has no import runs yet', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.listKbImportRuns).mockResolvedValueOnce({ runs: [], total: 0 })

    await mountDialog({ open: true })

    expect(document.body.querySelector('[data-testid="history-empty"]')).toBeTruthy()
    expect(document.body.querySelector('[data-testid="history-row"]')).toBeFalsy()
  })

  it('Next page requests the next offset from the store', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.listKbImportRuns)
      .mockResolvedValueOnce({ runs: Array.from({ length: 10 }, (_, i) => run({ run_id: `page1-${i}` })), total: 15 })
      .mockResolvedValueOnce({ runs: Array.from({ length: 5 }, (_, i) => run({ run_id: `page2-${i}` })), total: 15 })

    await mountDialog({ open: true })
    expect(api.listKbImportRuns).toHaveBeenCalledWith(10, 0)

    const next = document.body.querySelector('[aria-label="Следующая страница"]') as HTMLElement
    expect(next).toBeTruthy()
    next.click()
    await flushPromises()

    expect(api.listKbImportRuns).toHaveBeenCalledWith(10, 10)
  })
})

describe('KbImportHistoryDialog — run detail (KB-14)', () => {
  it('clicking a row shows that run\'s detail and emits update:selectedRunId', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.listKbImportRuns).mockResolvedValueOnce({ runs: [run({ run_id: 'r1' })], total: 1 })

    const { wrapper } = await mountDialog({ open: true })
    const row = document.body.querySelector('[data-testid="history-row"]') as HTMLElement
    row.click()
    await flushPromises()

    expect(document.body.querySelector('[data-testid="kb-import-run-status"]')).toBeTruthy()
    expect(document.body.querySelector('[data-testid="history-row"]')).toBeFalsy()
    expect(wrapper.emitted('update:selectedRunId')).toEqual([['r1']])
  })

  it('Back returns to the list and emits update:selectedRunId with an empty id', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.listKbImportRuns).mockResolvedValue({ runs: [run({ run_id: 'r1' })], total: 1 })

    const { wrapper } = await mountDialog({ open: true })
    ;(document.body.querySelector('[data-testid="history-row"]') as HTMLElement).click()
    await flushPromises()

    const back = document.body.querySelector('[data-testid="history-back"]') as HTMLElement
    expect(back).toBeTruthy()
    back.click()
    await flushPromises()

    expect(document.body.querySelector('[data-testid="kb-import-run-status"]')).toBeFalsy()
    expect(document.body.querySelector('[data-testid="history-row"]')).toBeTruthy()
    // Row click emits 'r1' first (opening the detail), the Back click below
    // emits '' second (returning to the list) — both calls, in order.
    expect(wrapper.emitted('update:selectedRunId')).toEqual([['r1'], ['']])
  })

  it('initialRunId fetches and shows that run\'s detail directly, without listing page 1 first', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.getKbImportRun).mockResolvedValueOnce(run({ run_id: 'r9', status: 'needs_human' }))

    await mountDialog({ open: true, initialRunId: 'r9' })

    expect(api.getKbImportRun).toHaveBeenCalledWith('r9')
    expect(api.listKbImportRuns).not.toHaveBeenCalled()
    expect(document.body.querySelector('[data-testid="kb-import-run-status"]')).toBeTruthy()
    expect(document.body.textContent).toContain('Нужна проверка') // needs_human
  })

  it('falls back to the list with an inline error when initialRunId cannot be loaded', async () => {
    const { api } = await import('@/api/client')
    vi.mocked(api.getKbImportRun).mockRejectedValueOnce(new Error('boom'))
    vi.mocked(api.listKbImportRuns).mockResolvedValueOnce({ runs: [run({ run_id: 'r1' })], total: 1 })

    await mountDialog({ open: true, initialRunId: 'r-missing' })

    expect(document.body.querySelector('[data-testid="history-detail-error"]')).toBeTruthy()
    expect(document.body.querySelector('[data-testid="kb-import-run-status"]')).toBeFalsy()
    expect(document.body.querySelector('[data-testid="history-row"]')).toBeTruthy()
  })
})
