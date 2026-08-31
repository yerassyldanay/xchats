import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mountKb, testPinia } from '@/test/mount'
import { useKbImport } from '@/stores/kbImport'
import { useAuth } from '@/stores/auth'
import { resetImportProvidersCache } from '@/composables/useImportProviders'
import KbIngestPanel from './KbIngestPanel.vue'
import type { KbImportRun } from '@/types'

vi.mock('@/lib/sse', () => ({ connectRealtime: vi.fn(() => vi.fn()) }))

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>()
  return {
    ...actual,
    api: {
      ...actual.api,
      get: vi.fn(),
      getKbImportProviders: vi.fn(),
      listKbImportRuns: vi.fn(),
      cancelKbImportRun: vi.fn(),
    },
  }
})

function run(over: Partial<KbImportRun> = {}): KbImportRun {
  return {
    run_id: 'run-1',
    status: 'extracting',
    started_by: 'user-1',
    started_at: '2026-08-31T10:00:00Z',
    cancelable: true,
    materials: [{ id: 'm-1', kind: 'url', label: 'https://vendor.example/zt40h', handle: 'evidence.1', processing_status: 'extracting' }],
    ...over,
  }
}

// KB-05: started_by only ever arrives as a raw user id — this panel is the
// one place that resolves it, against the SAME org user list already
// fetched app-wide for chat assignment (useInbox), not a fetch of its own.
async function mountPanel(currentRun: KbImportRun | null) {
  resetImportProvidersCache()
  const { api } = await import('@/api/client')
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/mcp-connection') return { mcp_url: 'https://example.com/mcp', auth_enabled: true, tunnel_available: false, tunnel_running: false, scopes: [] } as any
    if (path.startsWith('/users')) return { items: [{ id: 'user-1', email: 'aigul@example.com', name: 'Aigul', role: 'agent', must_change_password: false }] } as any
    throw new Error(`unexpected GET ${path}`)
  })
  vi.mocked(api.getKbImportProviders).mockResolvedValue([
    { id: 'native', display_name: 'native', families: ['url', 'docx', 'text', 'image'], requires_credential: false, configured: true },
  ])
  vi.mocked(api.listKbImportRuns).mockResolvedValue({ runs: currentRun ? [currentRun] : [] })

  const pinia = testPinia()
  const auth = useAuth()
  auth.user = { id: 'user-2', email: 'me@example.com', name: 'Me', role: 'admin', must_change_password: false }
  const kbi = useKbImport()
  const wrapper = mountKb(KbIngestPanel, { pinia })
  await flushPromises()
  return { wrapper, kbi }
}

describe('KbIngestPanel — ownership resolution (KB-05)', () => {
  it('resolves started_by to the matching user\'s name from the org user list', async () => {
    const { wrapper } = await mountPanel(run({ started_by: 'user-1' }))
    expect(wrapper.text()).toContain('Начал(а): Aigul')
  })

  it('shows "you" when started_by matches the current session\'s own user', async () => {
    const { wrapper } = await mountPanel(run({ started_by: 'user-2' }))
    expect(wrapper.text()).toContain('Начал(а): вы')
  })

  it('shows no ownership line when started_by matches nobody in the loaded user list', async () => {
    const { wrapper } = await mountPanel(run({ started_by: 'user-999' }))
    expect(wrapper.text()).not.toContain('Начал(а):')
  })
})

describe('KbIngestPanel — cancel wiring (KB-04)', () => {
  it('forwards KbImportRunStatus\'s cancel event to the store, which calls the API', async () => {
    const { wrapper } = await mountPanel(run())
    const { api } = await import('@/api/client')
    vi.mocked(api.cancelKbImportRun).mockResolvedValueOnce(run({ status: 'cancelled', cancelable: false }))

    await wrapper.find('[data-testid="kb-import-cancel"]').trigger('click')
    await flushPromises()

    expect(api.cancelKbImportRun).toHaveBeenCalledWith('run-1')
    expect(wrapper.text()).toContain('Отменено')
  })
})
